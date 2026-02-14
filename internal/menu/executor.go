package menu

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/robbiew/vision3/internal/ansi"
	"github.com/robbiew/vision3/internal/conference"
	"github.com/robbiew/vision3/internal/config"
	"github.com/robbiew/vision3/internal/editor"
	"github.com/robbiew/vision3/internal/file"
	"github.com/robbiew/vision3/internal/message"
	"github.com/robbiew/vision3/internal/terminalio" // <-- Added import
	"github.com/robbiew/vision3/internal/transfer"
	"github.com/robbiew/vision3/internal/types"
	"github.com/robbiew/vision3/internal/user"
	"golang.org/x/term"
)

// Mutex for protecting access to the oneliners file
var onelinerMutex sync.Mutex
var lastCallerATTokenRegex = regexp.MustCompile(`@([A-Za-z]{2,12})(?::(-?\d+))?@`)

// IPLockoutChecker defines the interface for IP-based authentication lockout.
// This allows the menu system to check and record failed login attempts without
// depending on the specific implementation in main.
type IPLockoutChecker interface {
	IsIPLockedOut(ip string) (bool, time.Time, int)
	RecordFailedLoginAttempt(ip string) bool
	ClearFailedLoginAttempts(ip string)
}

// RunnableFunc defines the signature for functions executable via RUN:
// Returns: authenticatedUser, nextAction (e.g., "GOTO:MENU"), err
type RunnableFunc func(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (authenticatedUser *user.User, nextAction string, err error)

// AutoRunTracker definition removed, using the one from types.go

// MenuExecutor handles the loading and execution of ViSiON/2 menus.
type MenuExecutor struct {
	ConfigPath     string                        // DEPRECATED: Use MenuSetPath + "/cfg" or RootConfigPath
	AssetsPath     string                        // DEPRECATED: Use MenuSetPath + "/ansi" or RootAssetsPath
	MenuSetPath    string                        // NEW: Path to the active menu set (e.g., "menus/v3")
	RootConfigPath string                        // NEW: Path to global configs (e.g., "configs")
	RootAssetsPath string                        // NEW: Path to global assets (e.g., "assets")
	RunRegistry    map[string]RunnableFunc       // Map RUN: targets to functions (Use local RunnableFunc)
	DoorRegistry   map[string]config.DoorConfig  // Map DOOR: targets to configurations
	OneLiners      []string                      // Loaded oneliners (Consider if these should be menu-set specific)
	LoadedStrings  config.StringsConfig          // Loaded global strings configuration
	Theme          config.ThemeConfig            // Loaded theme configuration
	ServerCfg      config.ServerConfig           // Server configuration (NEW)
	MessageMgr     *message.MessageManager       // <-- ADDED FIELD
	FileMgr        *file.FileManager             // <-- ADDED FIELD: File manager instance
	ConferenceMgr  *conference.ConferenceManager // Conference grouping manager
	IPLockoutCheck IPLockoutChecker              // IP-based authentication lockout checker
	LoginSequence  []config.LoginItem            // Configurable login sequence from login.json
	configMu       sync.RWMutex                  // Mutex for thread-safe config updates
}

// NewExecutor creates a new MenuExecutor.
// Added oneLiners, loadedStrings, theme, messageMgr, fileMgr, serverCfg, and ipLockoutCheck parameters
// Updated paths to use new structure
// << UPDATED Signature with msgMgr, fileMgr, serverCfg, and ipLockoutCheck
func NewExecutor(menuSetPath, rootConfigPath, rootAssetsPath string, oneLiners []string, doorRegistry map[string]config.DoorConfig, loadedStrings config.StringsConfig, theme config.ThemeConfig, serverCfg config.ServerConfig, msgMgr *message.MessageManager, fileMgr *file.FileManager, confMgr *conference.ConferenceManager, ipLockoutCheck IPLockoutChecker, loginSequence []config.LoginItem) *MenuExecutor {

	// Initialize the run registry
	runRegistry := make(map[string]RunnableFunc) // Use local RunnableFunc
	registerPlaceholderRunnables(runRegistry)    // Add placeholder registrations
	registerAppRunnables(runRegistry)            // Add application-specific runnables

	return &MenuExecutor{
		MenuSetPath:    menuSetPath,    // Store path to active menu set
		RootConfigPath: rootConfigPath, // Store path to global configs
		RootAssetsPath: rootAssetsPath, // Store path to global assets
		RunRegistry:    runRegistry,
		DoorRegistry:   doorRegistry,
		OneLiners:      oneLiners,      // Store loaded oneliners
		LoadedStrings:  loadedStrings,  // Store loaded strings
		Theme:          theme,          // Store loaded theme
		ServerCfg:      serverCfg,      // Store server configuration
		MessageMgr:     msgMgr,         // <-- ASSIGN FIELD
		FileMgr:        fileMgr,        // <-- ASSIGN FIELD
		ConferenceMgr:  confMgr,        // Conference grouping manager
		IPLockoutCheck: ipLockoutCheck, // IP-based lockout checker
		LoginSequence:  loginSequence,  // Configurable login sequence
	}
}

// --- Hot Reload Methods ---

// SetDoorRegistry atomically updates the door registry.
func (e *MenuExecutor) SetDoorRegistry(doors map[string]config.DoorConfig) {
	e.configMu.Lock()
	defer e.configMu.Unlock()
	e.DoorRegistry = doors
}

// GetDoorConfig atomically retrieves a door configuration.
func (e *MenuExecutor) GetDoorConfig(name string) (config.DoorConfig, bool) {
	e.configMu.RLock()
	defer e.configMu.RUnlock()
	cfg, ok := e.DoorRegistry[name]
	return cfg, ok
}

// SetLoginSequence atomically updates the login sequence.
func (e *MenuExecutor) SetLoginSequence(sequence []config.LoginItem) {
	e.configMu.Lock()
	defer e.configMu.Unlock()
	e.LoginSequence = sequence
}

// GetLoginSequence atomically retrieves the login sequence.
func (e *MenuExecutor) GetLoginSequence() []config.LoginItem {
	e.configMu.RLock()
	defer e.configMu.RUnlock()
	return e.LoginSequence
}

// SetStrings atomically updates the strings configuration.
func (e *MenuExecutor) SetStrings(strings config.StringsConfig) {
	e.configMu.Lock()
	defer e.configMu.Unlock()
	e.LoadedStrings = strings
}

// GetStrings atomically retrieves the strings configuration.
func (e *MenuExecutor) GetStrings() config.StringsConfig {
	e.configMu.RLock()
	defer e.configMu.RUnlock()
	return e.LoadedStrings
}

// SetTheme atomically updates the theme configuration.
func (e *MenuExecutor) SetTheme(theme config.ThemeConfig) {
	e.configMu.Lock()
	defer e.configMu.Unlock()
	e.Theme = theme
}

// GetTheme atomically retrieves the theme configuration.
func (e *MenuExecutor) GetTheme() config.ThemeConfig {
	e.configMu.RLock()
	defer e.configMu.RUnlock()
	return e.Theme
}

// SetServerConfig atomically updates the server configuration.
func (e *MenuExecutor) SetServerConfig(serverCfg config.ServerConfig) {
	e.configMu.Lock()
	defer e.configMu.Unlock()
	e.ServerCfg = serverCfg
}

// GetServerConfig atomically retrieves the server configuration.
func (e *MenuExecutor) GetServerConfig() config.ServerConfig {
	e.configMu.RLock()
	defer e.configMu.RUnlock()
	return e.ServerCfg
}

func (e *MenuExecutor) showUndefinedMenuInput(terminal *term.Terminal, outputMode ansi.OutputMode, nodeNumber int) {
	errMsg := "\r\n|01Unknown command!|07\r\n"
	processedErrMsg := ansi.ReplacePipeCodes([]byte(errMsg))
	if wErr := terminalio.WriteProcessedBytes(terminal, processedErrMsg, outputMode); wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing unknown command message: %v", nodeNumber, wErr)
	}
	time.Sleep(500 * time.Millisecond)
}

// setUserMsgConference updates the user's current message conference based on a conference ID.
func (e *MenuExecutor) setUserMsgConference(u *user.User, conferenceID int) {
	u.CurrentMsgConferenceID = conferenceID
	u.CurrentMsgConferenceTag = ""
	if conferenceID != 0 && e.ConferenceMgr != nil {
		if conf, ok := e.ConferenceMgr.GetByID(conferenceID); ok {
			u.CurrentMsgConferenceTag = conf.Tag
		}
	}
}

// setUserFileConference updates the user's current file conference based on a conference ID.
func (e *MenuExecutor) setUserFileConference(u *user.User, conferenceID int) {
	u.CurrentFileConferenceID = conferenceID
	u.CurrentFileConferenceTag = ""
	if conferenceID != 0 && e.ConferenceMgr != nil {
		if conf, ok := e.ConferenceMgr.GetByID(conferenceID); ok {
			u.CurrentFileConferenceTag = conf.Tag
		}
	}
}

// registerPlaceholderRunnables adds dummy functions for testing
func registerPlaceholderRunnables(registry map[string]RunnableFunc) { // Use local RunnableFunc
	// Keep READMAIL as a placeholder for now
	registry["READMAIL"] = func(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
		if currentUser == nil {
			log.Printf("WARN: Node %d: READMAIL called without logged in user.", nodeNumber)
			msg := "\r\n|01Error: You must be logged in to read mail.|07\r\n"
			wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			if wErr != nil {
				log.Printf("ERROR: Failed writing READMAIL error message: %v", wErr)
			}
			time.Sleep(1 * time.Second)
			return nil, "", nil // No user change, no next action, no error
		}
		msg := fmt.Sprintf("\r\n|15Executing |11READMAIL|15 for |14%s|15... (Not Implemented)|07\r\n", currentUser.Handle)
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil {
			log.Printf("ERROR: Failed writing READMAIL placeholder message: %v", wErr)
		}
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil // No user change, no next action, no error
	}

	// Register DOOR handler — delegates to door_handler.go
	registry["DOOR:"] = func(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, doorName string, outputMode ansi.OutputMode) (*user.User, string, error) {
		if currentUser == nil {
			log.Printf("WARN: Node %d: DOOR:%s called without logged in user.", nodeNumber, doorName)
			msg := "\r\n|01Error: You must be logged in to run doors.|07\r\n"
			wErr := terminalio.WriteProcessedBytes(s.Stderr(), ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			if wErr != nil {
				log.Printf("ERROR: Failed writing DOOR error message (not logged in): %v", wErr)
			}
			return nil, "", nil
		}
		log.Printf("INFO: Node %d: User %s attempting to run door: %s", nodeNumber, currentUser.Handle, doorName)

		// Look up door configuration
		doorConfig, exists := e.GetDoorConfig(strings.ToUpper(doorName))
		if !exists {
			log.Printf("WARN: Door configuration not found for '%s'", doorName)
			errMsg := fmt.Sprintf("\r\n|12Error: Door '%s' is not configured.\r\nPress Enter to continue...\r\n", doorName)
			wErr := terminalio.WriteProcessedBytes(s.Stderr(), ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
			if wErr != nil {
				log.Printf("ERROR: Failed writing DOOR error message (not configured) to stderr: %v", wErr)
			}
			return nil, "", nil
		}

		// Build door context and execute
		ctx := buildDoorCtx(e, s, terminal,
			currentUser.ID, currentUser.Handle, currentUser.RealName,
			currentUser.AccessLevel, currentUser.TimeLimit, currentUser.TimesCalled,
			currentUser.PhoneNumber, currentUser.GroupLocation,
			nodeNumber, sessionStartTime, outputMode,
			doorConfig, doorName)

		cmdErr := executeDoor(ctx)

		if cmdErr != nil {
			log.Printf("ERROR: Node %d: Door execution failed for user %s, door %s: %v", nodeNumber, currentUser.Handle, doorName, cmdErr)
			doorErrorMessage(ctx, fmt.Sprintf("Error running external program '%s': %v", doorName, cmdErr))
		} else {
			log.Printf("INFO: Node %d: Door completed for user %s, door %s", nodeNumber, currentUser.Handle, doorName)
		}

		return nil, "", nil
	}
}

// registerAppRunnables registers the actual application command functions.
func registerAppRunnables(registry map[string]RunnableFunc) { // Use local RunnableFunc
	registry["PLACEHOLDER"] = runPlaceholderCommand // Canonical handler for undefined/not-yet-implemented options
	registry["MAINLOGOFF"] = runMainLogoffCommand   // MAIN menu logoff with confirmation + GOODBYE.ANS
	registry["IMMEDIATELOGOFF"] = runImmediateLogoffCommand
	registry["SHOWSTATS"] = runShowStats
	registry["LASTCALLERS"] = runLastCallers
	registry["AUTHENTICATE"] = runAuthenticate
	registry["ONELINER"] = runOneliners                              // Register new placeholder
	registry["FULL_LOGIN_SEQUENCE"] = runFullLoginSequence           // Register the new sequence
	registry["SHOWVERSION"] = runShowVersion                         // Register the version display runnable
	registry["LISTUSERS"] = runListUsers                             // Register the user list runnable
	registry["PENDINGVALIDATIONNOTICE"] = runPendingValidationNotice // SysOp notice for new users awaiting validation
	registry["VALIDATEUSER"] = runValidateUser                       // Validate user accounts from admin menu
	registry["UNVALIDATEUSER"] = runUnvalidateUser                   // Remove validation from user accounts
	registry["BANUSER"] = runBanUser                                 // Quick-ban user accounts
	registry["DELETEUSER"] = runDeleteUser                           // Soft-delete user accounts (data preserved)
	registry["ADMINLISTUSERS"] = runAdminListUsers                   // Admin detailed user browser
	registry["LISTMSGAR"] = runListMessageAreas                      // <-- ADDED: Register message area list runnable
	registry["COMPOSEMSG"] = runComposeMessage                       // <-- ADDED: Register compose message runnable
	registry["PROMPTANDCOMPOSEMESSAGE"] = runPromptAndComposeMessage // <-- ADDED: Register prompt/compose runnable (Corrected key to uppercase)
	registry["READMSGS"] = runReadMsgs                               // <-- ADDED: Register message reading runnable
	registry["NEWSCAN"] = runNewscan                                 // <-- ADDED: Register newscan runnable
	registry["LISTFILES"] = runListFiles                             // <-- ADDED: Register file list runnable
	registry["LISTFILEAR"] = runListFileAreas                        // <-- ADDED: Register file area list runnable
	registry["SELECTFILEAREA"] = runSelectFileArea                   // <-- ADDED: Register file area selection runnable
	registry["SELECTMSGAREA"] = runSelectMessageArea                 // Register message area selection runnable
	registry["CHANGEMSGCONF"] = runChangeMsgConference               // Change message conference
	registry["NEXTMSGAREA"] = runNextMsgArea                         // Navigate to next message area
	registry["PREVMSGAREA"] = runPrevMsgArea                         // Navigate to previous message area
	registry["NEWUSER"] = runNewUser                                 // Register new user application runnable
	registry["GETHEADERTYPE"] = runGetHeaderType                     // Message header style selection
	registry["LISTMSGS"] = runListMsgs                               // List messages in current area
	registry["SENDPRIVMAIL"] = runSendPrivateMail                    // Send private mail to user
	registry["READPRIVMAIL"] = runReadPrivateMail                    // Read private mail
	registry["LISTPRIVMAIL"] = runListPrivateMail                    // List private mail
	registry["NEWSCANCONFIG"] = runNewscanConfig                     // Configure newscan tagged areas
	registry["NMAILSCAN"] = runNewMailScan                           // New mail scan
	registry["DISPLAYFILE"] = runLoginDisplayFile                    // Display ANSI file
	registry["RUNDOOR"] = runLoginDoor                               // Run external script/door
	registry["FASTLOGIN"] = runFastLogin                             // Inline fast login menu
	registry["LISTDOORS"] = runListDoors                             // List available doors
	registry["OPENDOOR"] = runOpenDoor                               // Prompt and open a door
	registry["DOORINFO"] = runDoorInfo                               // Show door information
}

func runPlaceholderCommand(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	e.showUndefinedMenuInput(terminal, outputMode, nodeNumber)
	return currentUser, "", nil
}

func runMainLogoffCommand(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	prompt := e.LoadedStrings.LogOffStr
	if prompt == "" {
		prompt = "\r\n|07Log off now? @"
	}

	confirm, err := e.promptYesNo(s, terminal, prompt, outputMode, nodeNumber)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return currentUser, "", err
	}

	if !confirm {
		return currentUser, "", nil
	}

	return runImmediateLogoffCommand(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode)
}

func runImmediateLogoffCommand(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if displayErr := e.displayFile(terminal, "GOODBYE.ANS", outputMode); displayErr != nil {
		log.Printf("WARN: Node %d: Failed to display GOODBYE.ANS before logoff: %v", nodeNumber, displayErr)
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07Goodbye!|07\r\n")), outputMode)
	}

	time.Sleep(1 * time.Second)
	return currentUser, "LOGOFF", nil
}

// runShowStats displays the user statistics screen (YOURSTAT.ANS).
func runShowStats(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		log.Printf("WARN: Node %d: SHOWSTATS called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to view stats.|07\r\n"
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil {
			log.Printf("ERROR: Failed writing SHOWSTATS error message: %v", wErr)
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Updated return
	}

	ansFilename := "YOURSTAT.ANS"
	// Use MenuSetPath for ANSI file
	fullAnsPath := filepath.Join(e.MenuSetPath, "ansi", ansFilename)
	rawAnsiContent, readErr := ansi.GetAnsiFileContent(fullAnsPath)
	if readErr != nil {
		log.Printf("ERROR: Node %d: Failed to read %s for SHOWSTATS: %v", nodeNumber, fullAnsPath, readErr)
		msg := fmt.Sprintf("\r\n|01Error displaying stats screen (%s).|07\r\n", ansFilename)
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil {
			log.Printf("ERROR: Failed writing SHOWSTATS file read error message: %v", wErr)
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed to read %s: %w", ansFilename, readErr) // Updated return
	}

	placeholders := map[string]string{
		"|UH": currentUser.Handle,
		"|UN": currentUser.PrivateNote,
		"|UL": strconv.Itoa(currentUser.AccessLevel),
		"|FL": strconv.Itoa(currentUser.AccessLevel),
		"|UK": strconv.Itoa(currentUser.NumUploads),
		"|NU": strconv.Itoa(currentUser.NumUploads),
		"|DK": "0", "|ND": "0", "|TP": "0", "|NM": "0", "|LC": "N/A",
	}
	if currentUser.TimeLimit <= 0 {
		placeholders["|TL"] = "Unlimited"
	} else {
		elapsedSeconds := time.Since(sessionStartTime).Seconds()
		totalSeconds := float64(currentUser.TimeLimit * 60)
		remainingSeconds := totalSeconds - elapsedSeconds
		if remainingSeconds < 0 {
			remainingSeconds = 0
		}
		placeholders["|TL"] = strconv.Itoa(int(remainingSeconds / 60))
	}

	substitutedContent := string(rawAnsiContent)
	for key, val := range placeholders {
		substitutedContent = strings.ReplaceAll(substitutedContent, key, val)
	}

	// Use WriteProcessedBytes for ClearScreen
	wErr := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	if wErr != nil {
		// Log error but continue if possible
		log.Printf("ERROR: Node %d: Failed clearing screen for SHOWSTATS: %v", nodeNumber, wErr)
	}

	// Log hex bytes before writing
	statsDisplayBytes := []byte(substitutedContent)
	// log.Printf("DEBUG: Node %d: Writing SHOWSTATS content bytes (hex): %x", nodeNumber, statsDisplayBytes)
	wErr = terminalio.WriteProcessedBytes(terminal, statsDisplayBytes, outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing processed YOURSTAT.ANS: %v", nodeNumber, wErr)
		return nil, "", wErr // Updated return
	}

	// 5. Wait for Enter key press
	pausePrompt := e.LoadedStrings.PauseString // Use configured pause string
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... " // Fallback
	}

	// Prepare bytes for the specific mode
	var pauseBytesToWrite []byte
	processedPausePrompt := ansi.ReplacePipeCodes([]byte(pausePrompt))
	if outputMode == ansi.OutputModeCP437 {
		var cp437Buf bytes.Buffer
		for _, r := range string(processedPausePrompt) {
			if r < 128 {
				cp437Buf.WriteByte(byte(r))
			} else if cp437Byte, ok := ansi.UnicodeToCP437[r]; ok {
				cp437Buf.WriteByte(cp437Byte)
			} else {
				cp437Buf.WriteByte('?')
			}
		}
		pauseBytesToWrite = cp437Buf.Bytes()
	} else {
		pauseBytesToWrite = processedPausePrompt
	}

	log.Printf("DEBUG: Node %d: Writing SHOWSTATS pause prompt. Mode: %d, Bytes: %q", nodeNumber, outputMode, string(pauseBytesToWrite))
	// Log hex bytes before writing
	log.Printf("DEBUG: Node %d: Writing SHOWSTATS pause bytes (hex): %x", nodeNumber, pauseBytesToWrite)
	wErr = terminalio.WriteProcessedBytes(terminal, pauseBytesToWrite, outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing SHOWSTATS pause prompt: %v", nodeNumber, wErr)
	}
	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during SHOWSTATS pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF // Updated return (Signal logoff)
			}
			log.Printf("ERROR: Failed reading input during SHOWSTATS pause: %v", err)
			return nil, "", err // Updated return
		}
		if r == '\r' || r == '\n' {
			break
		}
	}
	return nil, "", nil // Updated return (Success)
}

const (
	oneLinerMaxStored  = 20
	oneLinerMaxDisplay = 10
	oneLinerMaxLength  = 51
	oneLinerNameWidth  = 20
	oneLinerStartRow   = 12
	oneLinerStartCol   = 5
)

type onelinerRecord struct {
	Text             string `json:"text"`
	Anonymous        bool   `json:"anonymous,omitempty"`
	PostedByUsername string `json:"posted_by_username,omitempty"`
	PostedByHandle   string `json:"posted_by_handle,omitempty"`
	PostedAt         string `json:"posted_at,omitempty"`
}

type onelinerRecordCompat struct {
	DisplayName      string `json:"display_name,omitempty"`
	Username         string `json:"username,omitempty"`
	Text             string `json:"text"`
	Anonymous        bool   `json:"anonymous,omitempty"`
	PostedByUsername string `json:"posted_by_username,omitempty"`
	PostedByHandle   string `json:"posted_by_handle,omitempty"`
	PostedAt         string `json:"posted_at,omitempty"`
}

func truncateRunes(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || value == "" {
		return ""
	}
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func isPipeCodeStartChar(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func pipeCodeLenAt(value string, index int) int {
	if index < 0 || index >= len(value) || value[index] != '|' || index+1 >= len(value) {
		return 0
	}

	// 3-char forms: |00..|15, |CR, |DE, |CL, |PP, |23
	if index+2 < len(value) {
		two := value[index+1 : index+3]
		if len(two) == 2 {
			if (two[0] >= '0' && two[0] <= '9') && (two[1] >= '0' && two[1] <= '9') {
				return 3
			}
			u := strings.ToUpper(two)
			if u == "CR" || u == "DE" || u == "CL" || u == "PP" {
				return 3
			}
		}
	}

	// 4-char forms: |B0..|B9, |B10..|B15
	if index+2 < len(value) {
		if (value[index+1] == 'B' || value[index+1] == 'b') && (value[index+2] >= '0' && value[index+2] <= '9') {
			if index+3 < len(value) && (value[index+3] >= '0' && value[index+3] <= '9') {
				return 5 // |B10..|B15 (validated loosely)
			}
			return 4 // |B0..|B9
		}
	}

	// 2-char form: |P
	if index+1 < len(value) && (value[index+1] == 'P' || value[index+1] == 'p') {
		return 2
	}

	if index+1 < len(value) && isPipeCodeStartChar(value[index+1]) {
		return 0
	}

	return 0
}

func truncateOnelinerPreservePipeCodes(value string, maxVisible int) string {
	value = strings.TrimSpace(value)
	if value == "" || maxVisible <= 0 {
		return ""
	}

	var out strings.Builder
	visible := 0
	i := 0
	for i < len(value) {
		if value[i] == '|' {
			codeLen := pipeCodeLenAt(value, i)
			if codeLen > 0 && i+codeLen <= len(value) {
				out.WriteString(value[i : i+codeLen])
				i += codeLen
				continue
			}
		}

		r, size := utf8.DecodeRuneInString(value[i:])
		if r == utf8.RuneError && size == 1 {
			size = 1
		}
		if visible >= maxVisible {
			break
		}
		out.WriteString(value[i : i+size])
		visible++
		i += size
	}

	return strings.TrimSpace(out.String())
}

func visibleWidthPreservePipeCodes(value string) int {
	if value == "" {
		return 0
	}

	visible := 0
	i := 0
	for i < len(value) {
		if value[i] == '|' && i+1 < len(value) && value[i+1] == '|' {
			visible++
			i += 2
			continue
		}

		if value[i] == '|' {
			codeLen := pipeCodeLenAt(value, i)
			if codeLen > 0 && i+codeLen <= len(value) {
				i += codeLen
				continue
			}
		}

		_, size := utf8.DecodeRuneInString(value[i:])
		if size <= 0 {
			size = 1
		}
		visible++
		i += size
	}

	return visible
}

func truncatePipeCodedText(value string, maxVisible int) string {
	if value == "" || maxVisible <= 0 {
		return ""
	}

	var out strings.Builder
	visible := 0
	i := 0
	for i < len(value) {
		if value[i] == '|' && i+1 < len(value) && value[i+1] == '|' {
			if visible >= maxVisible {
				break
			}
			out.WriteString("||")
			visible++
			i += 2
			continue
		}

		if value[i] == '|' {
			codeLen := pipeCodeLenAt(value, i)
			if codeLen > 0 && i+codeLen <= len(value) {
				out.WriteString(value[i : i+codeLen])
				i += codeLen
				continue
			}
		}

		_, size := utf8.DecodeRuneInString(value[i:])
		if size <= 0 {
			size = 1
		}
		if visible >= maxVisible {
			break
		}
		out.WriteString(value[i : i+size])
		visible++
		i += size
	}

	return out.String()
}

func containsDisallowedOnelinerColorCode(value string) bool {
	i := 0
	for i < len(value) {
		if value[i] == '|' && i+1 < len(value) && value[i+1] == '|' {
			i += 2
			continue
		}

		if value[i] == '|' {
			codeLen := pipeCodeLenAt(value, i)
			if codeLen > 0 && i+codeLen <= len(value) {
				// Only standard foreground colors |01..|15 are allowed.
				if codeLen != 3 {
					return true
				}

				colorCode := value[i+1 : i+3]
				if colorCode < "01" || colorCode > "15" {
					return true
				}

				i += codeLen
				continue
			}
		}

		_, size := utf8.DecodeRuneInString(value[i:])
		if size <= 0 {
			size = 1
		}
		i += size
	}

	return false
}

func centerPipeCodedText(value string, width int) string {
	if width <= 0 {
		return value
	}

	visible := visibleWidthPreservePipeCodes(value)
	if visible >= width {
		return value
	}

	leftPad := (width - visible) / 2
	if leftPad <= 0 {
		return value
	}

	return strings.Repeat(" ", leftPad) + value
}

func formatOnelinerDisplayName(name string) string {
	formatted := truncateRunes(name, oneLinerNameWidth)
	if formatted == "" {
		formatted = "Unknown"
	}
	padding := oneLinerNameWidth - utf8.RuneCountInString(formatted)
	if padding > 0 {
		formatted = strings.Repeat(" ", padding) + formatted
	}
	return formatted
}

func onelinerVisibleName(record onelinerRecord, anonymousName string) string {
	if strings.TrimSpace(anonymousName) == "" {
		anonymousName = "Anonymous"
	}
	if record.Anonymous {
		return anonymousName
	}
	if strings.TrimSpace(record.PostedByHandle) != "" {
		return record.PostedByHandle
	}
	if strings.TrimSpace(record.PostedByUsername) != "" {
		return record.PostedByUsername
	}
	return "Unknown"
}

func loadOnelinerRecords(onelinerPath string) ([]onelinerRecord, error) {
	jsonData, readErr := os.ReadFile(onelinerPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return []onelinerRecord{}, nil
		}
		return nil, readErr
	}

	if strings.TrimSpace(string(jsonData)) == "" {
		return []onelinerRecord{}, nil
	}

	var rawEntries []json.RawMessage
	if err := json.Unmarshal(jsonData, &rawEntries); err != nil {
		return nil, err
	}

	records := make([]onelinerRecord, 0, len(rawEntries))
	for _, raw := range rawEntries {
		var legacyText string
		if err := json.Unmarshal(raw, &legacyText); err == nil {
			legacyText = truncateOnelinerPreservePipeCodes(legacyText, oneLinerMaxLength)
			if legacyText != "" {
				records = append(records, onelinerRecord{
					Text:             legacyText,
					PostedByUsername: "Unknown",
				})
			}
			continue
		}

		var compat onelinerRecordCompat
		if err := json.Unmarshal(raw, &compat); err != nil {
			continue
		}

		record := onelinerRecord{
			Text:             truncateOnelinerPreservePipeCodes(compat.Text, oneLinerMaxLength),
			Anonymous:        compat.Anonymous,
			PostedByUsername: strings.TrimSpace(compat.PostedByUsername),
			PostedByHandle:   strings.TrimSpace(compat.PostedByHandle),
			PostedAt:         compat.PostedAt,
		}

		if record.PostedByUsername == "" {
			record.PostedByUsername = strings.TrimSpace(compat.Username)
		}
		if record.PostedByHandle == "" {
			if strings.TrimSpace(compat.DisplayName) != "" && !record.Anonymous {
				record.PostedByHandle = strings.TrimSpace(compat.DisplayName)
			} else if strings.TrimSpace(compat.Username) != "" {
				record.PostedByHandle = strings.TrimSpace(compat.Username)
			}
		}

		if record.Text == "" {
			continue
		}

		records = append(records, record)
	}

	return records, nil
}

func saveOnelinerRecords(onelinerPath string, records []onelinerRecord) error {
	if len(records) > oneLinerMaxStored {
		records = records[len(records)-oneLinerMaxStored:]
	}

	updatedJSON, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(onelinerPath, updatedJSON, 0644)
}

// runOneliners displays the oneliners using templates.
func runOneliners(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running ONELINER", nodeNumber)

	onelinerPath := filepath.Join("data", "oneliners.json")

	var currentOneLiners []onelinerRecord
	onelinerMutex.Lock()
	loadedOneLiners, loadErr := loadOnelinerRecords(onelinerPath)
	onelinerMutex.Unlock()
	if loadErr != nil {
		log.Printf("ERROR: Failed loading oneliners from %s: %v", onelinerPath, loadErr)
		currentOneLiners = []onelinerRecord{}
	} else {
		currentOneLiners = loadedOneLiners
	}
	log.Printf("DEBUG: Loaded %d oneliners from %s", len(currentOneLiners), onelinerPath)

	numLiners := len(currentOneLiners)
	maxLinesToShow := oneLinerMaxDisplay
	startIdx := 0
	if numLiners > maxLinesToShow {
		startIdx = numLiners - maxLinesToShow
	}

	// 1. Load template files (same flow as LASTCALLERS)
	topTemplatePath := filepath.Join(e.MenuSetPath, "templates", "ONELINER.TOP")
	midTemplatePath := filepath.Join(e.MenuSetPath, "templates", "ONELINER.MID")
	botTemplatePath := filepath.Join(e.MenuSetPath, "templates", "ONELINER.BOT")

	topTemplateBytes, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplateBytes, errBot := os.ReadFile(botTemplatePath)
	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load one or more ONELINER template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading Oneliners screen templates.|07\r\n"
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil {
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading ONELINER templates")
	}

	// Strip SAUCE metadata and normalize broken bar delimiters, matching LASTCALLERS behavior.
	topTemplateBytes = stripSauceMetadata(topTemplateBytes)
	midTemplateBytes = stripSauceMetadata(midTemplateBytes)
	botTemplateBytes = stripSauceMetadata(botTemplateBytes)

	topTemplateBytes = normalizePipeCodeDelimiters(topTemplateBytes)
	midTemplateBytes = normalizePipeCodeDelimiters(midTemplateBytes)
	botTemplateBytes = normalizePipeCodeDelimiters(botTemplateBytes)

	processedTopTemplate := ansi.ReplacePipeCodes(topTemplateBytes)
	midTemplateRaw := string(midTemplateBytes)
	processedBotTemplate := ansi.ReplacePipeCodes(botTemplateBytes)

	wErr := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed clearing screen for ONELINER: %v", nodeNumber, wErr)
	}

	wErr = terminalio.WriteProcessedBytes(terminal, processedTopTemplate, outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing ONELINER top template: %v", nodeNumber, wErr)
		return nil, "", wErr
	}

	if numLiners == 0 {
		line := strings.ReplaceAll(midTemplateRaw, "^NU", formatOnelinerDisplayName("System"))
		line = strings.ReplaceAll(line, "^OL", "No one-liners yet. Be the first!")
		line = "    " + line
		lineBytes := ansi.ReplacePipeCodes([]byte(line))
		wErr = terminalio.WriteProcessedBytes(terminal, lineBytes, outputMode)
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing empty oneliner state: %v", nodeNumber, wErr)
			return nil, "", wErr
		}
	} else {
		anonymousName := strings.TrimSpace(e.LoadedStrings.AnonymousName)
		if anonymousName == "" {
			anonymousName = "Anonymous"
		}
		for i := startIdx; i < numLiners; i++ {
			record := currentOneLiners[i]
			displayName := onelinerVisibleName(record, anonymousName)
			displayName = formatOnelinerDisplayName(displayName)
			messageText := truncateOnelinerPreservePipeCodes(record.Text, oneLinerMaxLength)

			line := strings.ReplaceAll(midTemplateRaw, "^NU", displayName)
			line = strings.ReplaceAll(line, "^OL", messageText)
			line = "    " + line

			lineBytes := ansi.ReplacePipeCodes([]byte(line))
			wErr = terminalio.WriteProcessedBytes(terminal, lineBytes, outputMode)
			if wErr != nil {
				log.Printf("ERROR: Node %d: Failed writing oneliner line %d: %v", nodeNumber, i, wErr)
				return nil, "", wErr
			}
		}
	}

	wErr = terminalio.WriteProcessedBytes(terminal, processedBotTemplate, outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing ONELINER bottom template: %v", nodeNumber, wErr)
		return nil, "", wErr
	}
	// --- Ask to Add New One ---
	askPrompt := e.LoadedStrings.AskOneLiner
	if askPrompt == "" {
		log.Printf("ERROR: Required string 'AskOneLiner' is missing or empty in strings configuration.")
		return nil, "", fmt.Errorf("missing AskOneLiner string in configuration")
	}

	log.Printf("DEBUG: Node %d: Calling promptYesNo for ONELINER add prompt", nodeNumber)
	addYes, err := e.promptYesNo(s, terminal, askPrompt, outputMode, nodeNumber)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during ONELINER add prompt.", nodeNumber)
			return nil, "LOGOFF", io.EOF
		}
		log.Printf("ERROR: Failed getting Yes/No input for ONELINER add: %v", err)
		return nil, "", err
	}

	if addYes {
		allowAnon := currentUser != nil && currentUser.AccessLevel >= e.ServerCfg.AnonymousLevel
		isAnonymous := false
		if allowAnon {
			anonPrompt := e.LoadedStrings.OneLinerAnonymousPrompt
			if anonPrompt == "" {
				anonPrompt = "|09Post this one-liner as |08[|15A|08]nonymous|09? @"
			}
			// Start anonymous prompt at column 1 to avoid inherited indentation.
			wErr = terminalio.WriteProcessedBytes(terminal, []byte("\r\x1b[2K"), outputMode)
			if wErr != nil {
				log.Printf("WARN: Node %d: Failed to clear line before ONELINER anonymous prompt: %v", nodeNumber, wErr)
			}
			anonYes, anonErr := e.promptYesNo(s, terminal, anonPrompt, outputMode, nodeNumber)
			if anonErr != nil {
				if errors.Is(anonErr, io.EOF) {
					log.Printf("INFO: Node %d: User disconnected during ONELINER anonymous prompt.", nodeNumber)
					return nil, "LOGOFF", io.EOF
				}
				log.Printf("WARN: Node %d: Failed anonymous prompt for ONELINER: %v", nodeNumber, anonErr)
			} else {
				isAnonymous = anonYes
			}
		}

		enterPrompt := e.LoadedStrings.EnterOneLiner
		if enterPrompt == "" {
			log.Printf("ERROR: Required string 'EnterOneLiner' is missing or empty in strings configuration.")
			return nil, "", fmt.Errorf("missing EnterOneLiner string in configuration")
		}

		promptRow := 23
		promptColWidth := 80
		if ptyReq, _, isPty := s.Pty(); isPty && ptyReq.Window.Height > 0 {
			promptRow = ptyReq.Window.Height
			if ptyReq.Window.Width > 0 {
				promptColWidth = ptyReq.Window.Width
			}
		}

		// Prefer current cursor row so EnterOneLiner prompt reuses the same line
		// as the previous Yes/No prompt, avoiding stacked prompts.
		inputRow := promptRow
		if row, _, posErr := requestCursorPosition(s, terminal); posErr == nil && row > 0 {
			inputRow = row
		}

		legendText := strings.TrimSpace(e.LoadedStrings.OneLinerLegend)
		legendRow := inputRow - 1
		if legendRow < 1 {
			legendRow = 1
		}

		// Use WriteProcessedBytes for SaveCursor, positioning, and clear line
		wErr = terminalio.WriteProcessedBytes(terminal, []byte(ansi.SaveCursor()), outputMode)
		if wErr != nil { /* Log? */
		}
		// Clear legend row, detected input row, and prompt row fallback.
		posClearCmd := fmt.Sprintf("\x1b[%d;1H\x1b[2K\x1b[%d;1H\x1b[2K\x1b[%d;1H\x1b[2K\x1b[%d;1H", legendRow, inputRow, promptRow, inputRow)
		wErr = terminalio.WriteProcessedBytes(terminal, []byte(posClearCmd), outputMode)
		if wErr != nil { /* Log? */
		}

		if legendText != "" {
			legendText = truncatePipeCodedText(legendText, promptColWidth)
			legendPosCmd := fmt.Sprintf("\x1b[%d;1H", legendRow)
			wErr = terminalio.WriteProcessedBytes(terminal, []byte(legendPosCmd), outputMode)
			if wErr != nil {
				log.Printf("WARN: Node %d: Failed positioning ONELINER legend row: %v", nodeNumber, wErr)
			}

			legendBytes := ansi.ReplacePipeCodes([]byte(legendText))
			wErr = terminalio.WriteProcessedBytes(terminal, legendBytes, outputMode)
			if wErr != nil {
				log.Printf("WARN: Node %d: Failed writing ONELINER legend: %v", nodeNumber, wErr)
			}

			wErr = terminalio.WriteProcessedBytes(terminal, []byte(fmt.Sprintf("\x1b[%d;1H", inputRow)), outputMode)
			if wErr != nil {
				log.Printf("WARN: Node %d: Failed restoring ONELINER input row after legend: %v", nodeNumber, wErr)
			}
		}

		// Log hex bytes before writing
		enterPromptBytes := ansi.ReplacePipeCodes([]byte(enterPrompt))
		log.Printf("DEBUG: Node %d: Writing ONELINER enter prompt bytes (hex): %x", nodeNumber, enterPromptBytes)
		wErr = terminalio.WriteProcessedBytes(terminal, enterPromptBytes, outputMode)
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing EnterOneLiner prompt: %v", nodeNumber, wErr)
		}

		newOneliner, err := terminal.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected while entering oneliner.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Failed reading new oneliner input: %v", err)
			return nil, "", err
		}
		newOneliner = truncateOnelinerPreservePipeCodes(newOneliner, oneLinerMaxLength)
		if containsDisallowedOnelinerColorCode(newOneliner) {
			msg := "\r\n|01Only standard foreground colors |15||01|01-|15||15 |01are allowed in one-liners.|07\r\n"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(500 * time.Millisecond)
			return nil, "", nil
		}

		if newOneliner != "" {
			postedByUsername := ""
			postedByHandle := ""
			if currentUser != nil {
				postedByUsername = currentUser.Username
				postedByHandle = currentUser.Handle
			}
			if strings.TrimSpace(postedByUsername) == "" {
				postedByUsername = "Unknown"
			}
			if strings.TrimSpace(postedByHandle) == "" {
				postedByHandle = postedByUsername
			}

			entry := onelinerRecord{
				Text:             newOneliner,
				Anonymous:        isAnonymous,
				PostedByUsername: postedByUsername,
				PostedByHandle:   postedByHandle,
				PostedAt:         time.Now().UTC().Format(time.RFC3339),
			}

			onelinerMutex.Lock()
			latestOneLiners, latestErr := loadOnelinerRecords(onelinerPath)
			if latestErr != nil {
				log.Printf("WARN: Node %d: Failed reloading oneliners before save: %v", nodeNumber, latestErr)
				latestOneLiners = currentOneLiners
			}
			latestOneLiners = append(latestOneLiners, entry)
			saveErr := saveOnelinerRecords(onelinerPath, latestOneLiners)
			onelinerMutex.Unlock()

			if saveErr != nil {
				log.Printf("ERROR: Node %d: Failed to write updated oneliners JSON to %s: %v", nodeNumber, onelinerPath, saveErr)
				msg := "\r\n|01Error writing oneliner to disk.|07\r\n"
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			} else {
				log.Printf("INFO: Node %d: Successfully saved updated oneliners to %s", nodeNumber, onelinerPath)
				msg := "\r\n|10Oneliner added!|07\r\n"
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(500 * time.Millisecond)
			}
		} else {
			msg := "\r\n|01Empty oneliner not added.|07\r\n"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(500 * time.Millisecond)
		}
	} // end if addYes

	return nil, "", nil
}

// runAuthenticate handles the RUN:AUTHENTICATE command.
// Update signature to return three values
func runAuthenticate(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	// If already logged in, maybe show an error or just return?
	if currentUser != nil {
		log.Printf("WARN: Node %d: User %s tried to run AUTHENTICATE while already logged in.", nodeNumber, currentUser.Handle)
		msg := "\r\n|01You are already logged in.|07\r\n"
		// Use WriteProcessedBytes
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil {
			log.Printf("ERROR: Failed writing already logged in message: %v", wErr)
		}
		time.Sleep(1 * time.Second) // Pause after failed attempt
		return nil, "", nil         // No user change, no error
	}

	// Define approximate coordinates (MODIFY THESE based on LOGIN.ANS)
	userRow, userCol := 18, 20
	passRow, passCol := 19, 20
	errorRow := passRow + 2 // Row for error messages

	// Move to Username position, display prompt, and read input
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(userRow, userCol)), outputMode)
	usernamePrompt := "|07Username/Handle: |15" // Original prompt text was in ANSI
	// Use WriteProcessedBytes for prompt
	wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(usernamePrompt)), outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing username prompt: %v", nodeNumber, wErr)
		// Continue anyway?
	}
	usernameInput, err := terminal.ReadLine()
	if err != nil {
		if err == io.EOF {
			log.Printf("INFO: Node %d: User disconnected during username input.", nodeNumber)
			// Return an error that signals disconnection to the main loop
			return nil, "LOGOFF", io.EOF // Signal logoff
		}
		log.Printf("ERROR: Node %d: Failed to read username input: %v", nodeNumber, err)
		return nil, "", fmt.Errorf("failed reading username: %w", err) // Critical error
	}
	username := strings.TrimSpace(usernameInput)
	if username == "" {
		return nil, "", nil // Empty username, just redisplay login menu
	}

	// Check if user wants to apply as a new user
	if strings.EqualFold(username, "new") {
		log.Printf("INFO: Node %d: User typed 'new' in AUTHENTICATE - starting new user application", nodeNumber)
		newUserErr := e.handleNewUserApplication(s, terminal, userManager, nodeNumber, outputMode)
		if newUserErr != nil {
			if errors.Is(newUserErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: New user application error: %v", nodeNumber, newUserErr)
		}
		return nil, "", nil // Return to LOGIN screen after signup
	}

	// Move to Password position, display prompt, and read input securely
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(passRow, passCol)), outputMode)
	passwordPrompt := "|07Password: |15" // Original prompt text was in ANSI
	wErr = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(passwordPrompt)), outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing password prompt: %v", nodeNumber, wErr)
	}
	password, err := readPasswordSecurely(s, terminal, outputMode)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during password input.", nodeNumber)
			return nil, "LOGOFF", io.EOF // Signal logoff
		}
		if err.Error() == "password entry interrupted" { // Check for Ctrl+C
			log.Printf("INFO: Node %d: User interrupted password entry.", nodeNumber)
			// Treat interrupt like a failed attempt?
			terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(errorRow, 1)), outputMode) // Move cursor for message
			msg := "\r\n|01Login cancelled.|07\r\n"
			wErr = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			if wErr != nil {
				log.Printf("ERROR: Node %d: Failed writing login cancelled message: %v", nodeNumber, wErr)
			}
			time.Sleep(500 * time.Millisecond)
			return nil, "", nil // No user change, no critical error
		}
		log.Printf("ERROR: Node %d: Failed to read password securely: %v", nodeNumber, err)
		return nil, "", fmt.Errorf("failed reading password: %w", err) // Critical error
	}

	// Get remote IP address for lockout checking
	remoteAddr := s.RemoteAddr().String()
	// Extract just the IP (remove port if present)
	remoteIP := remoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		remoteIP = remoteAddr[:idx]
	}

	// Check if this IP is currently locked out
	if e.IPLockoutCheck != nil {
		isLocked, lockedUntil, attempts := e.IPLockoutCheck.IsIPLockedOut(remoteIP)
		if isLocked {
			log.Printf("SECURITY: Node %d: Login attempt from locked IP %s (locked until %s, %d attempts)",
				nodeNumber, remoteIP, lockedUntil.Format("2006-01-02 15:04:05"), attempts)
			terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(errorRow, 1)), outputMode)
			minutesLeft := int(time.Until(lockedUntil).Minutes()) + 1
			errMsg := fmt.Sprintf("\r\n|09Too many failed login attempts from your IP.\r\n|09Please try again in %d minutes.|07\r\n", minutesLeft)
			wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
			if wErr != nil {
				log.Printf("ERROR: Failed writing IP lockout message: %v", wErr)
			}
			time.Sleep(2 * time.Second)
			return nil, "", nil
		}
	}

	// Attempt Authentication via UserManager
	log.Printf("DEBUG: Node %d: Attempting authentication for user: %s from IP: %s", nodeNumber, username, remoteIP)
	authUser, authenticated := userManager.Authenticate(username, password)
	if !authenticated {
		log.Printf("WARN: Node %d: Failed authentication attempt for user: %s from IP: %s", nodeNumber, username, remoteIP)

		// Record failed login attempt for this IP
		if e.IPLockoutCheck != nil {
			wasLocked := e.IPLockoutCheck.RecordFailedLoginAttempt(remoteIP)
			if wasLocked {
				log.Printf("SECURITY: Node %d: IP %s has been locked out after too many failed attempts", nodeNumber, remoteIP)
			}
		}

		// Display error message to user
		terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(errorRow, 1)), outputMode) // Move cursor for message
		errMsg := "\r\n|01Login incorrect.|07\r\n"
		// Use WriteProcessedBytes
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
		if wErr != nil {
			log.Printf("ERROR: Failed writing login incorrect message: %v", wErr)
		}
		time.Sleep(1 * time.Second) // Pause after failed attempt
		return nil, "", nil         // Failed auth, but not a critical error. Let LOGIN menu handle retries.
	}

	// Check if user is validated
	if !authUser.Validated {
		log.Printf("INFO: Node %d: Login denied for user '%s' - account not validated", nodeNumber, username)
		// Display specific message for validation issue
		terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(errorRow, 1)), outputMode) // Move cursor for message
		errMsg := "\r\n|01Account requires validation by SysOp.|07\r\n"
		// Use WriteProcessedBytes
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
		if wErr != nil {
			log.Printf("ERROR: Failed writing validation required message: %v", wErr)
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Not validated, treat as failed login for this attempt
	}

	// Authentication Successful!
	log.Printf("INFO: Node %d: User '%s' (Handle: %s) authenticated successfully via RUN:AUTHENTICATE", nodeNumber, authUser.Username, authUser.Handle)

	// Clear failed login attempts for this IP
	if e.IPLockoutCheck != nil {
		e.IPLockoutCheck.ClearFailedLoginAttempts(remoteIP)
		log.Printf("DEBUG: Node %d: Cleared failed login attempts for IP %s", nodeNumber, remoteIP)
	}

	// Display success message (optional) - Move cursor first
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(errorRow, 1)), outputMode)
	// successMsg := "\r\n|10Login successful!|07\r\n"
	// terminal.Write(ansi.ReplacePipeCodes([]byte(successMsg)))
	// time.Sleep(500 * time.Millisecond)

	// Return the authenticated user object!
	return authUser, "", nil
}

// Run executes the menu logic for a given starting menu name.
// Reverted s parameter back to ssh.Session
// Added outputMode parameter
// Added currentAreaName parameter
func (e *MenuExecutor) Run(s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, startMenu string, nodeNumber int, sessionStartTime time.Time, autoRunLog types.AutoRunTracker, outputMode ansi.OutputMode, currentAreaName string, termWidth int, termHeight int) (string, *user.User, error) {
	currentMenuName := strings.ToUpper(startMenu)
	var previousMenuName string // Track the last menu visited
	// var authenticatedUserResult *user.User // Unused

	if currentUser != nil {
		log.Printf("DEBUG: Running menu for user %s (Level: %d)", currentUser.Handle, currentUser.AccessLevel)
	} else {
		log.Printf("DEBUG: Running menu for potentially unauthenticated user (login phase)")
	}

	for {
		log.Printf("INFO: Running menu: %s (Previous: %s) for Node %d", currentMenuName, previousMenuName, nodeNumber)

		var userInput string // Declare userInput here (Keep this one)
		// Removed authenticatedUserResult declaration from here
		// Numeric commands must be explicitly defined in KEYS tokens (no positional matching)

		// Determine ANSI filename using standard convention
		ansFilename := currentMenuName + ".ANS"
		// Use MenuSetPath for ANSI file
		fullAnsPath := filepath.Join(e.MenuSetPath, "ansi", ansFilename)

		// Process the associated ANSI file to get display bytes and coordinates
		rawAnsiContent, readErr := ansi.GetAnsiFileContent(fullAnsPath)
		if readErr == nil && currentMenuName == "ADMIN" {
			pendingCount := pendingValidationCount(userManager)
			rawAnsiContent = bytes.ReplaceAll(rawAnsiContent, []byte("{{PENDING_VALIDATIONS}}"), []byte(strconv.Itoa(pendingCount)))
		}
		var ansiProcessResult ansi.ProcessAnsiResult
		var processErr error
		if readErr != nil {
			log.Printf("ERROR: Failed to read ANSI file %s: %v", ansFilename, readErr)
			// Display error message to user (using new helper)
			errMsg := fmt.Sprintf("\r\n|01Error reading screen file: %s|07\r\n", ansFilename)
			wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
			if wErr != nil {
				log.Printf("ERROR: Failed writing screen read error: %v", wErr)
			}
			// Reading the screen file is critical, return error
			return "", nil, fmt.Errorf("failed to read screen file %s: %w", ansFilename, readErr)
		}

		// Successfully read, now process for coords and display bytes using the passed outputMode
		ansiProcessResult, processErr = ansi.ProcessAnsiAndExtractCoords(rawAnsiContent, outputMode)
		if processErr != nil {
			log.Printf("ERROR: Failed to process ANSI file %s: %v. Display may be incorrect.", ansFilename, processErr)
			// Processing error is also critical, return error
			return "", nil, fmt.Errorf("failed to process screen file %s: %w", ansFilename, processErr)
		}

		// --- SPECIAL HANDLING FOR LOGIN MENU INTERACTION ---
		if currentMenuName == "LOGIN" {
			if currentUser != nil {
				log.Printf("WARN: Attempting to run LOGIN menu for already authenticated user %s. Skipping login, going to MAIN.", currentUser.Handle)

				// Set default message area if not already set (e.g., SSH auto-login)
				if currentUser.CurrentMessageAreaID == 0 && e.MessageMgr != nil {
					for _, area := range e.MessageMgr.ListAreas() {
						if checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
							currentUser.CurrentMessageAreaID = area.ID
							currentUser.CurrentMessageAreaTag = area.Tag
							e.setUserMsgConference(currentUser, area.ConferenceID)
							break
						}
					}
				}

				// Set default file area if not already set
				if currentUser.CurrentFileAreaID == 0 && e.FileMgr != nil {
					for _, area := range e.FileMgr.ListAreas() {
						if checkACS(area.ACSList, currentUser, s, terminal, sessionStartTime) {
							currentUser.CurrentFileAreaID = area.ID
							currentUser.CurrentFileAreaTag = area.Tag
							e.setUserFileConference(currentUser, area.ConferenceID)
							break
						}
					}
				}

				// Persist defaults
				if userManager != nil {
					if saveErr := userManager.UpdateUser(currentUser); saveErr != nil {
						log.Printf("ERROR: Failed to save user default area selections: %v", saveErr)
					}
				}

				currentMenuName = "MAIN"
				previousMenuName = "LOGIN" // Set previous explicitly here
				continue
			}

			// Display the processed LOGIN screen, truncated to fit terminal height
			terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode) // Clear first
			displayBytes := ansiProcessResult.DisplayBytes
			if termHeight > 0 {
				// Truncate ANSI output to terminal height to prevent scrolling
				// which would shift all Y coordinates
				lines := bytes.Split(displayBytes, []byte("\n"))
				if len(lines) > termHeight {
					displayBytes = bytes.Join(lines[:termHeight], []byte("\n"))
					log.Printf("DEBUG: Truncated LOGIN.ANS from %d to %d lines for %d-row terminal",
						len(lines), termHeight, termHeight)
				}
			}
			wErr := terminalio.WriteProcessedBytes(terminal, displayBytes, outputMode)
			if wErr != nil {
				log.Printf("ERROR: Failed to write processed LOGIN.ANS bytes to terminal: %v", wErr)
				return "", nil, fmt.Errorf("failed to display LOGIN.ANS: %w", wErr)
			}

			// Handle the interactive login prompt using extracted coordinates
			authenticatedUserResult, loginErr := e.handleLoginPrompt(s, terminal, userManager, nodeNumber, ansiProcessResult.FieldCoords, outputMode, termWidth, termHeight)

			// Process result of login attempt
			if loginErr != nil {
				if errors.Is(loginErr, io.EOF) {
					log.Printf("INFO: User disconnected during login prompt.")
					return "LOGOFF", nil, nil // Signal logoff
				}
				log.Printf("ERROR: Error during login prompt handling: %v", loginErr)
				return "", nil, loginErr // Propagate critical error
			}

			if authenticatedUserResult != nil {
				log.Printf("INFO: Login successful for user %s. Proceeding based on LOGIN menu config.", authenticatedUserResult.Handle)
				currentUser = authenticatedUserResult // Update the user for this Run context

				// --- Update user's terminal dimensions from detected size ---
				if termWidth > 0 && termHeight > 0 {
					currentUser.ScreenWidth = termWidth
					currentUser.ScreenHeight = termHeight
					log.Printf("INFO: Updated user %s screen preferences to %dx%d", currentUser.Handle, termWidth, termHeight)
					if userManager != nil {
						if saveErr := userManager.UpdateUser(currentUser); saveErr != nil {
							log.Printf("ERROR: Failed to save user screen preferences: %v", saveErr)
						}
					}
				}

				// --- BEGIN Set Default Message Area ---
				if currentUser != nil && e.MessageMgr != nil {
					allAreas := e.MessageMgr.ListAreas() // Already sorted by ID
					log.Printf("DEBUG: Found %d message areas for user %s.", len(allAreas), currentUser.Handle)
					foundDefaultArea := false
					for _, area := range allAreas {
						// Check if user has read access to this area
						if checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
							log.Printf("INFO: Setting default message area for user %s to Area ID %d (%s)", currentUser.Handle, area.ID, area.Tag)
							currentUser.CurrentMessageAreaID = area.ID
							currentUser.CurrentMessageAreaTag = area.Tag
							e.setUserMsgConference(currentUser, area.ConferenceID)
							foundDefaultArea = true
							break // Found the first accessible area
						} else {
							log.Printf("TRACE: User %s denied read access to Area ID %d (%s) based on ACS '%s'", currentUser.Handle, area.ID, area.Tag, area.ACSRead)
						}
					}
					if !foundDefaultArea {
						log.Printf("WARN: User %s has no access to any message areas.", currentUser.Handle)
						currentUser.CurrentMessageAreaID = 0 // Set to 0 if no accessible areas found
						currentUser.CurrentMessageAreaTag = ""
					}
				} else {
					log.Printf("WARN: Cannot set default message area: currentUser (%v) or MessageMgr (%v) is nil.", currentUser == nil, e.MessageMgr == nil)
				}
				// --- END Set Default Message Area ---

				// --- BEGIN Set Default File Area ---
				if currentUser != nil && e.FileMgr != nil {
					allFileAreas := e.FileMgr.ListAreas() // Assumes ListAreas is sorted by ID
					log.Printf("DEBUG: Found %d file areas for user %s.", len(allFileAreas), currentUser.Handle)
					foundDefaultFileArea := false
					for _, area := range allFileAreas {
						// Check if user has list access to this area
						if checkACS(area.ACSList, currentUser, s, terminal, sessionStartTime) { // Use ACSList
							log.Printf("INFO: Setting default file area for user %s to Area ID %d (%s)", currentUser.Handle, area.ID, area.Tag)
							currentUser.CurrentFileAreaID = area.ID
							currentUser.CurrentFileAreaTag = area.Tag
							e.setUserFileConference(currentUser, area.ConferenceID)
							foundDefaultFileArea = true
							break // Found the first accessible area
						} else {
							log.Printf("TRACE: User %s denied list access to File Area ID %d (%s) based on ACS '%s'", currentUser.Handle, area.ID, area.Tag, area.ACSList)
						}
					}
					if !foundDefaultFileArea {
						log.Printf("WARN: User %s has no access to any file areas.", currentUser.Handle)
						currentUser.CurrentFileAreaID = 0 // Set to 0 if no accessible areas found
						currentUser.CurrentFileAreaTag = ""
					}
				} else {
					log.Printf("WARN: Cannot set default file area: currentUser (%v) or FileMgr (%v) is nil.", currentUser == nil, e.FileMgr == nil)
				}
				// --- END Set Default File Area ---

				// Persist default area/conference selections to disk
				if userManager != nil {
					if saveErr := userManager.UpdateUser(currentUser); saveErr != nil {
						log.Printf("ERROR: Failed to save user default area selections: %v", saveErr)
					}
				}

				// --- BEGIN POST-AUTHENTICATION TRANSITION ---
				// Load LOGIN.CFG to find the default action
				loginCfgPath := filepath.Join(e.MenuSetPath, "cfg") // Use correct path structure
				loginCommands, loadCmdErr := LoadCommands("LOGIN", loginCfgPath)
				if loadCmdErr != nil {
					log.Printf("CRITICAL: Failed to load LOGIN.CFG (%s) after successful authentication: %v", filepath.Join(loginCfgPath, "LOGIN.CFG"), loadCmdErr)
					// Return an error? Or try to default to MAIN?
					return "LOGOFF", currentUser, fmt.Errorf("failed loading LOGIN.CFG post-auth") // Logoff user on critical error
				}

				// Find the default command (Keys == "")
				nextAction := "" // Default action if not found?
				foundDefault := false
				for _, cmd := range loginCommands {
					if cmd.Keys == "" { // Check for empty string
						if cmd.Command == "RUN:AUTHENTICATE" {
							continue
						}
						if checkACS(cmd.ACS, currentUser, s, terminal, sessionStartTime) { // Use ssh.Session 's'
							nextAction = cmd.Command
							foundDefault = true
							log.Printf("DEBUG: Found default command in LOGIN.CFG after auth: %s", nextAction)
							break // Found the relevant default command (e.g., GOTO:MAIN)
						} else {
							log.Printf("WARN: User %s denied default command '%s' in LOGIN.CFG due to ACS '%s'", currentUser.Handle, cmd.Command, cmd.ACS)
						}
					}
				}

				if !foundDefault {
					log.Printf("CRITICAL: No accessible default command ('') found in LOGIN.CFG for user %s. Logging off.", currentUser.Handle)
					return "LOGOFF", currentUser, fmt.Errorf("no accessible default command found in LOGIN.CFG")
				}
				// -- Return the next action AND the authenticated user --
				return nextAction, currentUser, nil
			} else { // authenticatedUserResult == nil
				log.Printf("INFO: Login failed. Redisplaying LOGIN menu.")
				continue // Restart loop for LOGIN
			}
		} // --- END SPECIAL LOGIN INTERACTION BLOCK ---

		// --- REGULAR MENU PROCESSING (Common for ALL menus, including LOGIN after interaction) ---
		// 1. Load Menu Definition (.MNU)
		menuMnuPath := filepath.Join(e.MenuSetPath, "mnu") // Use correct path structure for MNU
		menuRec, err := LoadMenu(currentMenuName, menuMnuPath)
		if err != nil {
			errMsg := fmt.Sprintf("|01Error loading menu %s: %v|07", currentMenuName, err)
			processedErrMsg := ansi.ReplacePipeCodes([]byte(errMsg))
			// Use new helper for error message
			wErr := terminalio.WriteProcessedBytes(terminal, processedErrMsg, outputMode)
			if wErr != nil {
				log.Printf("ERROR: Failed writing menu load error message: %v", wErr)
			}
			log.Printf("ERROR: %s", errMsg)
			return "", nil, fmt.Errorf("failed to load menu %s: %w", currentMenuName, err)
		}

		// 2. Load Commands (.CFG) for the *current* menu (which might be LOGIN)
		menuCfgPath := filepath.Join(e.MenuSetPath, "cfg") // Use correct path structure for CFG
		commands, err := LoadCommands(currentMenuName, menuCfgPath)
		if err != nil {
			log.Printf("WARN: Failed to load commands for menu %s: %v", currentMenuName, err)
			commands = []CommandRecord{} // Use empty slice
		}

		// Check Menu Password if required
		menuPassword := menuRec.Password
		if menuPassword != "" {
			log.Printf("DEBUG: Menu '%s' requires password.", currentMenuName)
			passwordOk := false
			for i := 0; i < 3; i++ { // Allow 3 attempts
				prompt := fmt.Sprintf("\r\n|07Password for %s (|15Attempt %d/3|07): ", currentMenuName, i+1)
				processedPrompt := ansi.ReplacePipeCodes([]byte(prompt))
				wErr := terminalio.WriteProcessedBytes(terminal, processedPrompt, outputMode)
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing menu password prompt: %v", nodeNumber, wErr)
				}

				// Use our helper for secure input reading (using ssh.Session 's')
				inputPassword, err := readPasswordSecurely(s, terminal, outputMode)
				if err != nil {
					if errors.Is(err, io.EOF) {
						log.Printf("INFO: User disconnected during menu password entry for '%s'", currentMenuName)
						return "LOGOFF", nil, nil // Signal logoff
					}
					if err.Error() == "password entry interrupted" { // Check for specific error
						log.Printf("INFO: User interrupted password entry for menu '%s'", currentMenuName)
						return "LOGOFF", nil, nil // Signal logoff
					}
					log.Printf("ERROR: Failed to read password input securely: %v", err)
					return "", nil, fmt.Errorf("failed reading password: %w", err)
				}
				if inputPassword == menuPassword {
					passwordOk = true
					// Use new helper for feedback message
					wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07Password accepted.|07\r\n")), outputMode)
					if wErr != nil {
						log.Printf("ERROR: Failed writing password accepted message: %v", wErr)
					}
					break
				} else {
					// Use new helper for feedback message
					wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01Incorrect Password.|07\r\n")), outputMode)
					if wErr != nil {
						log.Printf("ERROR: Failed writing incorrect password message: %v", wErr)
					}
				}
			}
			if !passwordOk {
				log.Printf("WARN: User failed password entry for menu '%s' (User: %v)", currentMenuName, currentUser)
				// Use new helper for feedback message
				wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01Too many incorrect attempts.|07\r\n")), outputMode)
				if wErr != nil {
					log.Printf("ERROR: Failed writing too many attempts message: %v", wErr)
				}
				time.Sleep(1 * time.Second)
				return "LOGOFF", nil, nil // Signal logoff after too many failures
			}
		}

		// Check Menu ACS before proceeding
		menuACS := menuRec.ACS
		if !checkACS(menuACS, currentUser, s, terminal, sessionStartTime) { // Use ssh.Session 's'
			log.Printf("INFO: User denied access to menu '%s' due to ACS: %s (User: %v)", currentMenuName, menuACS, currentUser)
			errMsg := "\r\n|01Access Denied.|07\r\n"
			processedErrMsg := ansi.ReplacePipeCodes([]byte(errMsg))
			// Use new helper for error message
			wErr := terminalio.WriteProcessedBytes(terminal, processedErrMsg, outputMode)
			if wErr != nil {
				log.Printf("ERROR: Failed writing ACS denied message: %v", wErr)
			}
			time.Sleep(1 * time.Second) // Brief pause
			return "LOGOFF", nil, nil   // Signal logoff
		}

		// --- AutoRun Command Execution ---
		autoRunActionTaken := false
		for _, cmd := range commands {
			if cmd.Keys == "//" || cmd.Keys == "~~" {
				autoRunKey := fmt.Sprintf("%s:%s", currentMenuName, cmd.Command) // Unique key per menu/command

				if cmd.Keys == "//" && autoRunLog[autoRunKey] {
					log.Printf("DEBUG: Skipping already executed run-once command: %s", autoRunKey)
					continue // Skip if already run
				}
				if checkACS(cmd.ACS, currentUser, s, terminal, sessionStartTime) { // Use ssh.Session 's'
					log.Printf("INFO: Executing AutoRun command (%s): %s (ACS: %s)", cmd.Keys, cmd.Command, cmd.ACS)

					if cmd.Keys == "//" {
						autoRunLog[autoRunKey] = true
					}
					nextAction, nextMenu, userResult, err := e.executeCommandAction(cmd.Command, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, outputMode)
					if err != nil {
						return "", userResult, err
					}
					if nextAction == "GOTO" {
						previousMenuName = currentMenuName
						currentMenuName = nextMenu
						autoRunActionTaken = true
						break
					} else if nextAction == "LOGOFF" {
						return "LOGOFF", userResult, nil
					} else if nextAction == "CONTINUE" {
						if userResult != nil {
							currentUser = userResult
						}
					}
				} else {
					log.Printf("DEBUG: AutoRun command (%s) %s denied by ACS: %s", cmd.Keys, cmd.Command, cmd.ACS)
				}
			}
		}
		if autoRunActionTaken {
			continue
		}
		// --- End AutoRun Command Execution ---

		// 3. Display ANSI Screen (Processed Bytes) - Moved display logic here for ALL menus
		// (Avoid double-display for LOGIN which handles its own display before prompt)
		// We still need the raw content for potential lightbar background
		// Note: ansBackgroundBytes is currently unused but will be needed for full lightbar implementation
		// ansBackgroundBytes := ansiProcessResult.DisplayBytes
		if currentMenuName != "LOGIN" {
			// Clear screen before displaying menu if configured to do so
			if menuRec.GetClrScrBefore() {
				if wErr := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode); wErr != nil {
					log.Printf("ERROR: Node %d: Failed clearing screen for menu %s: %v", nodeNumber, currentMenuName, wErr)
				}
			}
			// Truncate ANSI output to terminal height to prevent scrolling
			displayBytes := ansiProcessResult.DisplayBytes
			if termHeight > 0 {
				lines := bytes.Split(displayBytes, []byte("\n"))
				if len(lines) > termHeight {
					displayBytes = bytes.Join(lines[:termHeight], []byte("\n"))
					log.Printf("DEBUG: Truncated %s.ANS from %d to %d lines for %d-row terminal",
						currentMenuName, len(lines), termHeight, termHeight)
				}
			}
			if wErr := terminalio.WriteProcessedBytes(terminal, displayBytes, outputMode); wErr != nil {
				log.Printf("ERROR: Failed writing ANSI screen for %s: %v", currentMenuName, wErr)
				return "", nil, fmt.Errorf("failed displaying screen: %w", wErr)
			}
		}

		// --- Check for Lightbar Menu (.BAR) ---
		// Check if a .BAR file exists for this menu in the MENU SET directory
		barFilename := currentMenuName + ".BAR"
		barPath := filepath.Join(e.MenuSetPath, "bar", barFilename)
		_, barErr := os.Stat(barPath)
		isLightbarMenu := barErr == nil // Treat as lightbar if .BAR exists and is accessible
		if barErr != nil && !os.IsNotExist(barErr) {
			log.Printf("WARN: Error checking for BAR file %s: %v. Assuming standard menu.", barPath, barErr)
		}

		// Variable declarations for command handling
		// var userInput string // REMOVE this redeclaration
		// var numericMatchAction string // Moved declaration up

		// 4. Determine Input Mode / Method
		if isLightbarMenu {
			log.Printf("DEBUG: Entering Lightbar input mode for %s", currentMenuName)

			// Load lightbar options from the config directory
			// Pass 'e' (MenuExecutor) to the updated function
			lightbarOptions, loadErr := loadLightbarOptions(currentMenuName, e)
			if loadErr != nil {
				log.Printf("ERROR: Failed to load lightbar options for %s: %v", currentMenuName, loadErr)
				isLightbarMenu = false
			} else if len(lightbarOptions) == 0 {
				log.Printf("WARN: No valid lightbar options loaded for %s", currentMenuName)
				isLightbarMenu = false
			}

			if isLightbarMenu { // Double check after loading options
				// Save background for redrawing during selection changes
				ansBackgroundBytes := ansiProcessResult.DisplayBytes // Use the already processed bytes

				// Initially draw with first option selected
				selectedIndex := 0
				drawErr := drawLightbarMenu(terminal, ansBackgroundBytes, lightbarOptions, selectedIndex, outputMode, false)
				if drawErr != nil {
					log.Printf("ERROR: Failed to draw lightbar menu for %s: %v", currentMenuName, drawErr)
					isLightbarMenu = false
				} else {
					// Process keyboard navigation for lightbar
					lightbarResult := "" // Use a local variable for the result
					inputLoop := true
					bufioReader := bufio.NewReader(s)
					for inputLoop {
						// Read keyboard input for lightbar navigation
						r, _, err := bufioReader.ReadRune()
						if err != nil {
							if err == io.EOF {
								log.Printf("INFO: User disconnected during lightbar input for %s", currentMenuName)
								return "LOGOFF", nil, nil // Signal logoff
							}
							log.Printf("ERROR: Failed to read lightbar input for menu %s: %v", currentMenuName, err)
							return "", nil, fmt.Errorf("failed reading lightbar input: %w", err)
						}
						log.Printf("DEBUG: Lightbar input rune: '%c' (%d)", r, r)

						if r < 32 && r != '\r' && r != '\n' && r != 27 {
							continue
						}

						// Map specific keys for navigation and selection
						switch r {
						case '1', '2', '3', '4', '5', '6', '7', '8', '9':
							// Direct selection by number
							numIndex := int(r - '1') // Convert 1-9 to 0-8
							if numIndex >= 0 && numIndex < len(lightbarOptions) {
								prevIndex := selectedIndex
								selectedIndex = numIndex
								if prevIndex != selectedIndex {
									_ = drawLightbarOption(terminal, lightbarOptions[prevIndex], false, outputMode)
									_ = drawLightbarOption(terminal, lightbarOptions[selectedIndex], true, outputMode)
								}
								lightbarResult = lightbarOptions[numIndex].HotKey
								inputLoop = false
							}
						case '\r', '\n': // Enter - select current item
							if selectedIndex >= 0 && selectedIndex < len(lightbarOptions) {
								lightbarResult = lightbarOptions[selectedIndex].HotKey
								inputLoop = false
							}
						case 27: // ESC key - check for arrow keys in ANSI sequence
							time.Sleep(20 * time.Millisecond)
							seq := make([]byte, 0, 8)
							for bufioReader.Buffered() > 0 && len(seq) < 8 {
								b, readErr := bufioReader.ReadByte()
								if readErr != nil {
									break
								}
								seq = append(seq, b)
							}

							// Check for arrow keys and handle navigation
							if len(seq) >= 2 && seq[0] == 91 { // '['
								switch seq[1] {
								case 65: // Up arrow
									if selectedIndex > 0 {
										prevIndex := selectedIndex
										selectedIndex--
										_ = drawLightbarOption(terminal, lightbarOptions[prevIndex], false, outputMode)
										_ = drawLightbarOption(terminal, lightbarOptions[selectedIndex], true, outputMode)
									}
								case 66: // Down arrow
									if selectedIndex < len(lightbarOptions)-1 {
										prevIndex := selectedIndex
										selectedIndex++
										_ = drawLightbarOption(terminal, lightbarOptions[prevIndex], false, outputMode)
										_ = drawLightbarOption(terminal, lightbarOptions[selectedIndex], true, outputMode)
									}
								}
							}
							continue // Continue waiting for more input after navigation
						default:
							// Check if key matches any hotkey directly
							keyStr := strings.ToUpper(string(r))
							for _, opt := range lightbarOptions {
								if keyStr == opt.HotKey {
									lightbarResult = opt.HotKey
									inputLoop = false
									break // Exit inner loop
								}
							}
							if !inputLoop {
								break // Exit switch if hotkey matched
							}
							continue // Otherwise keep waiting for valid input
						}
					}
					log.Printf("DEBUG: Processed Lightbar input as: '%s'", lightbarResult)
					// Set userInput to lightbar result if a selection was made
					if lightbarResult != "" {
						userInput = lightbarResult
					}
				}
			}

			if !isLightbarMenu || userInput == "" {
				// Fallback to standard input if lightbar loading failed or no valid selection made
				// Display Prompt (Skip if USEPROMPT is false)
				if menuRec.GetUsePrompt() { // Condition changed: Only check UsePrompt
					err = e.displayPrompt(terminal, menuRec, currentUser, userManager, nodeNumber, currentMenuName, sessionStartTime, outputMode, currentAreaName) // Pass currentAreaName
					if err != nil {
						return "", nil, err // Propagate the error
					}
				} else {
					// Log message remains the same, but the condition causing it is now just UsePrompt==false
					log.Printf("DEBUG: Skipping prompt display for %s (UsePrompt: %t, Prompt1 empty: %t)", currentMenuName, menuRec.GetUsePrompt(), menuRec.Prompt1 == "")
				}

				// Read User Input Line
				input, err := terminal.ReadLine()
				if err != nil {
					if err == io.EOF {
						log.Printf("INFO: User disconnected during menu input for %s", currentMenuName)
						return "LOGOFF", nil, nil // Signal logoff
					}
					log.Printf("ERROR: Failed to read input for menu %s: %v", currentMenuName, err)
					return "", nil, fmt.Errorf("failed reading input: %w", err)
				}
				userInput = strings.ToUpper(strings.TrimSpace(input))
				log.Printf("DEBUG: User input: '%s'", userInput)
			}
		} else {
			// --- Standard Menu Input Handling ---
			// Display Prompt (Skip if USEPROMPT is false)
			log.Printf("DEBUG: Checking prompt display for menu: %s. UsePrompt=%t", currentMenuName, menuRec.GetUsePrompt())
			if menuRec.GetUsePrompt() { // Condition changed: Only check UsePrompt
				log.Printf("DEBUG: Calling displayPrompt for menu: %s", currentMenuName)
				err = e.displayPrompt(terminal, menuRec, currentUser, userManager, nodeNumber, currentMenuName, sessionStartTime, outputMode, currentAreaName) // Pass currentAreaName
				log.Printf("DEBUG: Returned from displayPrompt for menu: %s. Error: %v", currentMenuName, err)
				if err != nil {
					return "", nil, err // Propagate the error
				}
			} else {
				// Log message remains the same, but the condition causing it is now just UsePrompt==false
				log.Printf("DEBUG: Skipping prompt display for %s (UsePrompt: %t, Prompt1 empty: %t)", currentMenuName, menuRec.GetUsePrompt(), menuRec.Prompt1 == "")
			}

			// Read User Input Line
			input, err := terminal.ReadLine()
			if err != nil {
				if err == io.EOF {
					log.Printf("INFO: User disconnected during menu input for %s", currentMenuName)
					return "LOGOFF", nil, nil // Signal logoff
				}
				log.Printf("ERROR: Failed to read input for menu %s: %v", currentMenuName, err)
				return "", nil, fmt.Errorf("failed reading input: %w", err)
			}
			userInput = strings.ToUpper(strings.TrimSpace(input))
			log.Printf("DEBUG: User input: '%s'", userInput)

			// --- Special Input Handling (^P, ##) ---
			if userInput == "\x10" || userInput == "^P" { // Ctrl+P is ASCII 16 (\x10)
				if previousMenuName != "" {
					log.Printf("DEBUG: User entered ^P, going back to previous menu: %s", previousMenuName)
					temp := currentMenuName
					currentMenuName = previousMenuName
					previousMenuName = temp // Update previous in case they go back again
					continue                // Go directly to the previous menu loop iteration
				} else {
					log.Printf("DEBUG: User entered ^P, but no previous menu recorded.")
					continue // Re-display current menu prompt
				}
			}

			// --- End Special Input Handling ---
		} // End if isLightbarMenu / else

		// 6. Process Input / Find Command Match (userInput determined by menu type)
		matched := false
		nextAction := "" // Store the action determined by the matched command

		// Global hangup shortcut: /G
		if userInput == "/G" {
			nextAction = "RUN:IMMEDIATELOGOFF"
			matched = true
		}

		if !matched { // Check keyword matches (relevant for both)
			for _, cmdRec := range commands {
				if cmdRec.GetHidden() {
					continue // Skip hidden commands
				}

				cmdACS := cmdRec.ACS
				if !checkACS(cmdACS, currentUser, s, terminal, sessionStartTime) { // Use ssh.Session 's'
					if currentUser != nil {
						log.Printf("DEBUG: User '%s' does not meet ACS '%s' for command key(s) '%s'", currentUser.Handle, cmdACS, cmdRec.Keys)
					} else {
						log.Printf("DEBUG: Unauthenticated user does not meet ACS '%s' for command key(s) '%s'", cmdACS, cmdRec.Keys)
					}
					continue // Skip this command if ACS check fails
				}

				keys := strings.Split(cmdRec.Keys, " ") // Use string directly
				for _, key := range keys {
					// Handle empty userInput from lightbar mode if non-mapped key was pressed
					if key != "" && userInput != "" && userInput == key {
						nextAction = cmdRec.Command // Store the action string
						log.Printf("DEBUG: Matched key '%s' to command action: '%s'", key, nextAction)
						matched = true
						break // Found match, break inner key loop
					}
				}
				if matched {
					break // Break outer command loop
				}
			}
		}

		// 7. Handle Action or No Match
		if matched {
			// Execute the determined action here
			nextActionType, nextMenuName, userResult, err := e.executeCommandAction(nextAction, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, outputMode)
			if err != nil {
				return "", userResult, err
			}
			if nextActionType == "GOTO" {
				previousMenuName = currentMenuName // Store current before going to next
				currentMenuName = nextMenuName
				continue // Continue main loop to the new menu
			} else if nextActionType == "LOGOFF" {
				return "LOGOFF", userResult, nil // Return specific logoff action
			} else if nextActionType == "CONTINUE" {
				if userResult != nil {
					currentUser = userResult
				}
				continue // Re-display current menu prompt
			}
			log.Printf("WARN: Unhandled action type '%s' after executing command '%s'", nextActionType, nextAction)
			continue
		} else {
			log.Printf("DEBUG: Input '%s' did not match any commands in menu %s", userInput, currentMenuName)

			// If it was a lightbar menu and input was ignored (userInput == ""), just loop again
			if isLightbarMenu {
				continue
			}

			fallbackMenu := menuRec.Fallback
			if fallbackMenu != "" {
				log.Printf("INFO: No command match, using fallback menu: %s", fallbackMenu)
				previousMenuName = currentMenuName // Store current before going to fallback
				currentMenuName = strings.ToUpper(fallbackMenu)
				continue
			}
			e.showUndefinedMenuInput(terminal, outputMode, nodeNumber)
			continue // Redisplay current menu
		}
	}
}

// handleLoginPrompt manages the interactive username/password entry using coordinates.
// Added outputMode parameter.
func (e *MenuExecutor) handleLoginPrompt(s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, nodeNumber int, coords map[string]struct{ X, Y int }, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, error) {
	// Get coordinates for username and password fields from the map
	userCoord, userOk := coords["P"] // Use 'P' for Handle/Name field based on LOGIN.ANS
	passCoord, passOk := coords["O"] // Use 'O' for Password field based on LOGIN.ANS

	log.Printf("DEBUG: LOGIN Coords Received - P: %+v (Ok: %t), O: %+v (Ok: %t)", userCoord, userOk, passCoord, passOk)

	if !userOk || !passOk {
		log.Printf("CRITICAL: LOGIN.ANS is missing required coordinate codes P or O.")
		if wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01CRITICAL ERROR: Login screen configuration invalid (Missing P/O).|07\r\n")), outputMode); wErr != nil {
			log.Printf("ERROR: Failed writing critical login configuration message: %v", wErr)
		}
		time.Sleep(2 * time.Second)
		return nil, fmt.Errorf("missing login coordinates P/O in LOGIN.ANS")
	}

	// No Y offset needed — ANSI display is truncated to termHeight rows,
	// preventing scrolling, so extracted coordinates are accurate as-is
	log.Printf("DEBUG: Node %d: Login prompt coords P=(%d,%d) O=(%d,%d) termHeight=%d", nodeNumber, userCoord.X, userCoord.Y, passCoord.X, passCoord.Y, termHeight)

	errorRow := passCoord.Y + 2 // Error message row below password
	if errorRow <= userCoord.Y || errorRow <= passCoord.Y {
		errorRow = userCoord.Y + 2 // Adjust if overlapping
	}

	// Move to Username position (coordinates are accurate since display is truncated to fit)
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(userCoord.Y, userCoord.X)), outputMode)
	usernameInput, err := terminal.ReadLine()
	if err != nil {
		if err == io.EOF {
			return nil, io.EOF // Signal disconnection
		}
		log.Printf("ERROR: Node %d: Failed to read username input: %v", nodeNumber, err)
		return nil, fmt.Errorf("failed reading username: %w", err)
	}
	username := strings.TrimSpace(usernameInput)
	if username == "" {
		log.Printf("DEBUG: Node %d: Empty username entered.", nodeNumber)
		return nil, nil // Return nil user, nil error to signal retry LOGIN
	}

	// Check if user wants to apply as a new user
	if strings.EqualFold(username, "new") {
		log.Printf("INFO: Node %d: User typed 'new' - starting new user application", nodeNumber)
		err := e.handleNewUserApplication(s, terminal, userManager, nodeNumber, outputMode)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, io.EOF
			}
			log.Printf("ERROR: Node %d: New user application error: %v", nodeNumber, err)
		}
		return nil, nil // Return to LOGIN screen after signup
	}

	// Move to Password position (coordinates are accurate since display is truncated to fit)
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(passCoord.Y, passCoord.X)), outputMode)
	password, err := readPasswordSecurely(s, terminal, outputMode) // Use ssh.Session 's'
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF // Signal disconnection
		}
		if err.Error() == "password entry interrupted" { // Check for Ctrl+C
			log.Printf("INFO: Node %d: User interrupted password entry.", nodeNumber)
			terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(errorRow, 1)), outputMode)
			if wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01Login cancelled.|07\r\n")), outputMode); wErr != nil {
				log.Printf("ERROR: Failed writing login cancelled message: %v", wErr)
			}
			time.Sleep(500 * time.Millisecond)
			return nil, nil // Signal retry LOGIN
		}
		log.Printf("ERROR: Node %d: Failed to read password securely: %v", nodeNumber, err)
		return nil, fmt.Errorf("failed reading password: %w", err)
	}

	// Get remote IP address for lockout checking
	remoteAddr := s.RemoteAddr().String()
	// Extract just the IP (remove port if present)
	remoteIP := remoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		remoteIP = remoteAddr[:idx]
	}

	// Check if this IP is currently locked out
	if e.IPLockoutCheck != nil {
		isLocked, lockedUntil, attempts := e.IPLockoutCheck.IsIPLockedOut(remoteIP)
		if isLocked {
			log.Printf("SECURITY: Node %d: Login attempt from locked IP %s (locked until %s, %d attempts)",
				nodeNumber, remoteIP, lockedUntil.Format("2006-01-02 15:04:05"), attempts)
			terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(errorRow, 1)), outputMode)
			minutesLeft := int(time.Until(lockedUntil).Minutes()) + 1
			errMsg := fmt.Sprintf("\r\n|09Too many failed login attempts from your IP.\r\n|09Please try again in %d minutes.|07\r\n", minutesLeft)
			wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
			if wErr != nil {
				log.Printf("ERROR: Failed writing IP lockout message: %v", wErr)
			}
			time.Sleep(2 * time.Second)
			return nil, nil
		}
	}

	// Attempt Authentication via UserManager
	log.Printf("DEBUG: Node %d: Attempting authentication for user: %s from IP: %s", nodeNumber, username, remoteIP)
	authUser, authenticated := userManager.Authenticate(username, password)
	if !authenticated {
		log.Printf("WARN: Node %d: Failed authentication attempt for user: %s from IP: %s", nodeNumber, username, remoteIP)

		// Record failed login attempt for this IP
		if e.IPLockoutCheck != nil {
			wasLocked := e.IPLockoutCheck.RecordFailedLoginAttempt(remoteIP)
			if wasLocked {
				log.Printf("SECURITY: Node %d: IP %s has been locked out after too many failed attempts", nodeNumber, remoteIP)
			}
		}

		terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(errorRow, 1)), outputMode) // Move cursor for message
		errMsg := "\r\n|01Login incorrect.|07\r\n"
		// Use WriteProcessedBytes with the passed outputMode
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
		if wErr != nil {
			log.Printf("ERROR: Failed writing login incorrect message: %v", wErr)
		}
		time.Sleep(1 * time.Second) // Pause after failed attempt
		return nil, nil             // Failed auth, but not a critical error. Let LOGIN menu handle retries.
	}

	if !authUser.Validated {
		log.Printf("INFO: Node %d: Login denied for user '%s' - account not validated", nodeNumber, username)
		terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(errorRow, 1)), outputMode) // Move cursor for message
		errMsg := "\r\n|01Account requires validation by SysOp.|07\r\n"
		// Use WriteProcessedBytes with the passed outputMode
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
		if wErr != nil {
			log.Printf("ERROR: Failed writing validation required message: %v", wErr)
		}
		time.Sleep(1 * time.Second)
		return nil, nil // Not validated, treat as failed login for this attempt
	}

	log.Printf("INFO: Node %d: User '%s' (Handle: %s) authenticated successfully via LOGIN prompt", nodeNumber, authUser.Username, authUser.Handle)

	// Clear failed login attempts for this IP
	if e.IPLockoutCheck != nil {
		e.IPLockoutCheck.ClearFailedLoginAttempts(remoteIP)
		log.Printf("DEBUG: Node %d: Cleared failed login attempts for IP %s", nodeNumber, remoteIP)
	}

	return authUser, nil // Success!
}

// readPasswordSecurely reads a password from the terminal without echoing characters,
// Reverted s parameter back to ssh.Session
func readPasswordSecurely(s ssh.Session, terminal *term.Terminal, outputMode ansi.OutputMode) (string, error) {
	var password []rune
	var byteBuf [1]byte               // Buffer for writing '*'
	bufioReader := bufio.NewReader(s) // Wrap ssh.Session

	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Println("DEBUG: EOF received during secure password read.")
			}
			return "", err // Propagate errors
		}

		switch r {
		case '\r': // Enter key (Carriage Return)
			terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
			return string(password), nil
		case '\n': // Newline - often follows \r, ignore it if so.
			continue
		case 127, 8: // Backspace (DEL or BS)
			if len(password) > 0 {
				password = password[:len(password)-1]
				err := terminalio.WriteProcessedBytes(terminal, []byte("\b \b"), outputMode)
				if err != nil {
					log.Printf("WARN: Failed to write backspace sequence: %v", err)
				}
			}
		case 3: // Ctrl+C (ETX)
			terminalio.WriteProcessedBytes(terminal, []byte("^C\r\n"), outputMode)
			return "", fmt.Errorf("password entry interrupted")
		default:
			if r >= 32 { // Basic check for printable ASCII
				password = append(password, r)
				byteBuf[0] = '*'
				err := terminalio.WriteProcessedBytes(terminal, byteBuf[:], outputMode)
				if err != nil {
					log.Printf("WARN: Failed to write asterisk: %v", err)
				}
			}
		}
	}
}

// executeCommandAction handles the logic for executing a command string (GOTO, RUN, DOOR, LOGOFF).
// Returns: actionType (GOTO, LOGOFF, CONTINUE), nextMenu, resultingUser, error
func (e *MenuExecutor) executeCommandAction(action string, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, outputMode ansi.OutputMode) (actionType string, nextMenu string, userResult *user.User, err error) {
	if strings.HasPrefix(action, "GOTO:") {
		nextMenu = strings.ToUpper(strings.TrimPrefix(action, "GOTO:"))
		return "GOTO", nextMenu, currentUser, nil
	} else if action == "LOGOFF" {
		return "LOGOFF", "", currentUser, nil
	} else if strings.HasPrefix(action, "RUN:") {
		parts := strings.SplitN(strings.TrimPrefix(action, "RUN:"), " ", 2)
		runTarget := strings.ToUpper(parts[0])
		var runArgs string
		if len(parts) > 1 {
			runArgs = parts[1]
		}
		log.Printf("INFO: Executing RUN action: Target='%s' Args='%s'", runTarget, runArgs)

		if runnableFunc, exists := e.RunRegistry[runTarget]; exists {
			log.Printf("DEBUG: Node %d: Calling registered function for RUN:%s", nodeNumber, runTarget)
			// RunnableFunc now returns user, nextActionString, error
			authUser, nextActionStr, runErr := runnableFunc(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, runArgs, outputMode)
			if runErr != nil {
				if errors.Is(runErr, io.EOF) {
					log.Printf("INFO: Node %d: User disconnected during RUN:%s execution.", nodeNumber, runTarget)
					return "LOGOFF", "", nil, nil
				}
				log.Printf("ERROR: RUN:%s function failed: %v", runTarget, runErr)
				errMsg := fmt.Sprintf("\r\n|01Error running command '%s': %v|07\r\n", runTarget, runErr)
				wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
				if wErr != nil {
					log.Printf("ERROR: Failed writing RUN command error message: %v", wErr)
				}
				time.Sleep(1 * time.Second)
				// Assign the potentially updated user before returning
				userResult = authUser                     // Capture potential user changes (like from AUTHENTICATE)
				return "CONTINUE", "", userResult, runErr // Continue but report error?
			}
			log.Printf("DEBUG: RUN:%s function completed.", runTarget)

			// Check if the runnable function returned a specific next action
			if strings.HasPrefix(nextActionStr, "GOTO:") {
				nextMenu = strings.ToUpper(strings.TrimPrefix(nextActionStr, "GOTO:"))
				log.Printf("DEBUG: RUN:%s requested GOTO:%s", runTarget, nextMenu)
				return "GOTO", nextMenu, authUser, nil
			} else if nextActionStr == "LOGOFF" {
				log.Printf("DEBUG: RUN:%s requested LOGOFF", runTarget)
				return "LOGOFF", "", authUser, nil
			}

			// Default action for RUN is CONTINUE
			return "CONTINUE", "", authUser, nil
		} else {
			log.Printf("WARN: No internal function registered for RUN:%s", runTarget)
			msg := fmt.Sprintf("\r\n|01Internal command '%s' not found.|07\r\n", runTarget)
			wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			if wErr != nil {
				log.Printf("ERROR: Failed writing missing RUN command message: %v", wErr)
			}
			time.Sleep(1 * time.Second)
			return "CONTINUE", "", currentUser, nil
		}
	} else if strings.HasPrefix(action, "DOOR:") {
		doorTarget := strings.TrimPrefix(action, "DOOR:")
		log.Printf("INFO: Executing DOOR action: '%s'", doorTarget)
		if doorFunc, exists := e.RunRegistry["DOOR:"]; exists {
			// DOOR runnable returns user, "", error
			userResultDoor, nextActionStrDoor, doorErr := doorFunc(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, doorTarget, outputMode)
			if doorErr != nil {
				if errors.Is(doorErr, io.EOF) {
					log.Printf("INFO: Node %d: User disconnected during DOOR:%s execution.", nodeNumber, doorTarget)
					return "LOGOFF", "", nil, nil
				}
				log.Printf("ERROR: DOOR:%s execution failed: %v", doorTarget, doorErr)
				errMsg := fmt.Sprintf("\r\n|01Error running door '%s': %v|07\r\n", doorTarget, doorErr)
				wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
				if wErr != nil {
					log.Printf("ERROR: Failed writing DOOR command error message: %v", wErr)
				}
				time.Sleep(1 * time.Second)
				// Assign potential user result before returning
				userResult = userResultDoor
				return "CONTINUE", "", userResult, doorErr // Continue after door error?
			}
			// Handle potential LOGOFF request from DOOR runnable (though currently returns "")
			if nextActionStrDoor == "LOGOFF" {
				log.Printf("DEBUG: DOOR:%s requested LOGOFF", doorTarget)
				return "LOGOFF", "", userResultDoor, nil
			}
			log.Printf("DEBUG: DOOR:%s completed.", doorTarget)
			return "CONTINUE", "", userResultDoor, nil // Default CONTINUE after door
		} else {
			log.Printf("CRITICAL: DOOR: function not registered!")
			return "CONTINUE", "", currentUser, nil
		}
	} else {
		log.Printf("WARN: Unhandled command action type in executeCommandAction: %s", action)
		return "CONTINUE", "", currentUser, nil
	}
}

// runLastCallers displays the last callers list using templates.
func runLastCallers(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running LASTCALLERS", nodeNumber)

	// Parse optional caller count argument (e.g., RUN:LASTCALLERS 10)
	callerLimit := 10
	if strings.TrimSpace(args) != "" {
		if parsedLimit, parseErr := strconv.Atoi(strings.TrimSpace(args)); parseErr == nil && parsedLimit > 0 {
			callerLimit = parsedLimit
		}
	}

	// 1. Load Template Files from MenuSetPath/templates
	topTemplatePath := filepath.Join(e.MenuSetPath, "templates", "LASTCALL.TOP")
	midTemplatePath := filepath.Join(e.MenuSetPath, "templates", "LASTCALL.MID")
	botTemplatePath := filepath.Join(e.MenuSetPath, "templates", "LASTCALL.BOT")

	topTemplateBytes, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplateBytes, errBot := os.ReadFile(botTemplatePath)

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load one or more LASTCALL template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading Last Callers screen templates.|07\r\n"
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading LASTCALL templates")
	}

	// Strip SAUCE metadata, normalize delimiters, and process pipe codes in templates first.
	topTemplateBytes = stripSauceMetadata(topTemplateBytes)
	midTemplateBytes = stripSauceMetadata(midTemplateBytes)
	botTemplateBytes = stripSauceMetadata(botTemplateBytes)

	// Normalize delimiters and process pipe codes in templates first.
	// Some ANSI/ASCII assets may use broken bar (¦) instead of literal pipe (|).
	topTemplateBytes = normalizePipeCodeDelimiters(topTemplateBytes)
	midTemplateBytes = normalizePipeCodeDelimiters(midTemplateBytes)
	botTemplateBytes = normalizePipeCodeDelimiters(botTemplateBytes)

	processedTopTemplate := string(ansi.ReplacePipeCodes(topTemplateBytes))
	processedMidTemplate := string(ansi.ReplacePipeCodes(midTemplateBytes)) // Process MID template
	processedBotTemplate := string(ansi.ReplacePipeCodes(botTemplateBytes))
	// --- END Template Processing ---

	// 2. Get last callers data from UserManager
	lastCallers := userManager.GetLastCallers()
	users := userManager.GetAllUsers()
	totalUsers := len(users)
	userNotesByID := make(map[int]string, len(users))
	for _, userRecord := range users {
		if userRecord == nil {
			continue
		}
		userNotesByID[userRecord.ID] = userRecord.PrivateNote
	}
	timeLoc := getLastCallerTimeLocation(strings.TrimSpace(e.ServerCfg.Timezone))
	if callerLimit > 0 && len(lastCallers) > callerLimit {
		lastCallers = lastCallers[len(lastCallers)-callerLimit:]
	}

	processedTopTemplate = renderLastCallerGlobalATTokens(processedTopTemplate, totalUsers)
	processedBotTemplate = renderLastCallerGlobalATTokens(processedBotTemplate, totalUsers)

	// 3. Build the output string using processed templates and processed data
	var outputBuffer bytes.Buffer
	outputBuffer.WriteString(processedTopTemplate) // Write processed top template
	if !strings.HasSuffix(processedTopTemplate, "\r\n") && !strings.HasSuffix(processedTopTemplate, "\n") {
		outputBuffer.WriteString("\r\n")
	}

	if len(lastCallers) == 0 {
		// Optional: Handle empty state. The template might handle this.
		log.Printf("DEBUG: Node %d: No last callers to display.", nodeNumber)
		// If templates don't handle empty, add a message here.
	} else {
		// Iterate through call records and format using processed LASTCALL.MID
		for _, record := range lastCallers {
			line := processedMidTemplate // Start with the pipe-code-processed mid template
			userNote := string(ansi.ReplacePipeCodes([]byte(userNotesByID[record.UserID])))

			// Format data for substitution with fixed-width padding for column alignment
			baud := record.BaudRate
			name := string(ansi.ReplacePipeCodes([]byte(record.Handle)))
			groupLoc := string(ansi.ReplacePipeCodes([]byte(record.GroupLocation)))
			onTime := formatLastCallerShortLocalTime(record.ConnectTime, timeLoc)
			actions := record.Actions
			hours := int(record.Duration.Hours())
			mins := int(record.Duration.Minutes()) % 60
			hmm := fmt.Sprintf("%d:%02d", hours, mins)
			upM := fmt.Sprintf("%.1f", record.UploadedMB)
			dnM := fmt.Sprintf("%.1f", record.DownloadedMB)
			nodeStr := strconv.Itoa(record.NodeID)
			callNumStr := strconv.FormatUint(record.CallNumber, 10)

			// Replace placeholders with padded data to match header column widths.
			// Header: " # |  Node |  Handle           | Baud         | Group/Affil"
			// Widths:   3     7      19                  14             rest
			// All spacing is in the padding — template has no extra spaces.
			line = strings.ReplaceAll(line, "^CN", fmt.Sprintf(" %-2s", callNumStr)) // 3 chars
			line = strings.ReplaceAll(line, "^ND", fmt.Sprintf("  %-5s", nodeStr))   // 7 chars
			line = strings.ReplaceAll(line, "^UN", fmt.Sprintf("  %-17s", name))     // 19 chars
			line = strings.ReplaceAll(line, "^BA", fmt.Sprintf(" %-13s", baud))      // 14 chars
			line = strings.ReplaceAll(line, "^GL", fmt.Sprintf(" %s", groupLoc))
			line = strings.ReplaceAll(line, "^OT", fmt.Sprintf("%-8s", onTime))
			line = strings.ReplaceAll(line, "^AC", actions)
			line = strings.ReplaceAll(line, "^HM", fmt.Sprintf("%-5s", hmm))
			line = strings.ReplaceAll(line, "^UM", fmt.Sprintf("%-6s", upM))
			line = strings.ReplaceAll(line, "^DM", fmt.Sprintf("%-6s", dnM))
			line = strings.ReplaceAll(line, "^NT", userNote)
			line = renderLastCallerATTokens(line, record, totalUsers, userNote, timeLoc)

			line = strings.TrimRight(line, "\r\n") + "\r\n"
			outputBuffer.WriteString(line) // Add the fully substituted and processed line
		}
	}

	outputBuffer.WriteString(processedBotTemplate) // Write processed bottom template

	// 4. Clear screen and display the assembled content
	writeErr := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	if writeErr != nil {
		log.Printf("ERROR: Node %d: Failed clearing screen for LASTCALLERS: %v", nodeNumber, writeErr)
		return nil, "", writeErr
	}

	// Use WriteProcessedBytes for the assembled template content
	processedContent := outputBuffer.Bytes() // Contains already-processed ANSI bytes
	wErr := terminalio.WriteProcessedBytes(terminal, processedContent, outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing LASTCALLERS output: %v", nodeNumber, wErr)
		return nil, "", wErr
	}

	// 5. Wait for Enter using configured PauseString (logic remains the same)
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... " // Fallback
	}
	if !strings.HasPrefix(pausePrompt, "\r\n") && !strings.HasPrefix(pausePrompt, "\n") {
		pausePrompt = "\r\n" + pausePrompt
	}

	var pauseBytesToWrite []byte
	processedPausePrompt := ansi.ReplacePipeCodes([]byte(pausePrompt))
	if outputMode == ansi.OutputModeCP437 {
		var cp437Buf bytes.Buffer
		for _, r := range string(processedPausePrompt) {
			if r < 128 {
				cp437Buf.WriteByte(byte(r))
			} else if cp437Byte, ok := ansi.UnicodeToCP437[r]; ok {
				cp437Buf.WriteByte(cp437Byte)
			} else {
				cp437Buf.WriteByte('?')
			}
		}
		pauseBytesToWrite = cp437Buf.Bytes()
	} else {
		pauseBytesToWrite = processedPausePrompt
	}

	log.Printf("DEBUG: Node %d: Writing LASTCALLERS pause prompt. Mode: %d, Bytes: %q", nodeNumber, outputMode, string(pauseBytesToWrite))
	// Log hex bytes before writing
	log.Printf("DEBUG: Node %d: Writing LASTCALLERS pause bytes (hex): %x", nodeNumber, pauseBytesToWrite)
	wErr = terminalio.WriteProcessedBytes(terminal, pauseBytesToWrite, outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing LASTCALLERS pause prompt: %v", nodeNumber, wErr)
	}

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during LASTCALLERS pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading input during LASTCALLERS pause: %v", nodeNumber, err)
			return nil, "", err
		}
		if r == '\r' || r == '\n' { // Check for CR or LF
			break
		}
	}

	return nil, "", nil // Success
}

func renderLastCallerATTokens(template string, record user.CallRecord, totalUsers int, userNote string, timeLoc *time.Location) string {
	return lastCallerATTokenRegex.ReplaceAllStringFunc(template, func(match string) string {
		parts := lastCallerATTokenRegex.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		code := strings.ToUpper(parts[1])
		value, ok := lastCallerATTokenValue(code, record, totalUsers, userNote, timeLoc)
		if !ok {
			return match
		}

		if len(parts) > 2 && parts[2] != "" {
			if width, err := strconv.Atoi(parts[2]); err == nil {
				if isLastCallerATCenterAligned(code) {
					value = formatLastCallerATWidthCentered(value, width)
				} else {
					value = formatLastCallerATWidth(value, width, isLastCallerATNumeric(code))
				}
			}
		}

		return value
	})
}

func renderLastCallerGlobalATTokens(template string, totalUsers int) string {
	return lastCallerATTokenRegex.ReplaceAllStringFunc(template, func(match string) string {
		parts := lastCallerATTokenRegex.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		code := strings.ToUpper(parts[1])
		if code != "USERCT" {
			return match
		}

		value := strconv.Itoa(totalUsers)
		if len(parts) > 2 && parts[2] != "" {
			if width, err := strconv.Atoi(parts[2]); err == nil {
				value = formatLastCallerATWidth(value, width, true)
			}
		}

		return value
	})
}

func normalizePipeCodeDelimiters(input []byte) []byte {
	if len(input) == 0 {
		return input
	}

	// Only normalize likely pipe-code delimiters (e.g. ¦CR, ¦08, │DE).
	// Do NOT blanket-convert ANSI line-art bytes (such as CP437 0xB3), which
	// can corrupt imported Retrograde art templates.
	normalized := make([]byte, 0, len(input))

	isPipeCodeLead := func(b byte) bool {
		return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
	}

	for i := 0; i < len(input); {
		replaced := false

		// UTF-8 broken bar (U+00A6 => 0xC2 0xA6)
		if i+1 < len(input) && input[i] == 0xC2 && input[i+1] == 0xA6 {
			if i+2 < len(input) && isPipeCodeLead(input[i+2]) {
				normalized = append(normalized, '|')
				i += 2
				replaced = true
			}
		}

		if !replaced {
			// UTF-8 box drawing light vertical (U+2502 => 0xE2 0x94 0x82)
			if i+2 < len(input) && input[i] == 0xE2 && input[i+1] == 0x94 && input[i+2] == 0x82 {
				if i+3 < len(input) && isPipeCodeLead(input[i+3]) {
					normalized = append(normalized, '|')
					i += 3
					replaced = true
				}
			}
		}

		if !replaced {
			// Raw single-byte broken bar (0xA6)
			if input[i] == 0xA6 {
				if i+1 < len(input) && isPipeCodeLead(input[i+1]) {
					normalized = append(normalized, '|')
					i++
					replaced = true
				}
			}
		}

		if !replaced {
			normalized = append(normalized, input[i])
			i++
		}
	}

	return normalized
}

func stripSauceMetadata(input []byte) []byte {
	if len(input) < 7 {
		return input
	}

	idx := bytes.LastIndex(input, []byte("SAUCE00"))
	if idx < 0 {
		return input
	}

	// SAUCE record should be near EOF; ignore stray in-body text matches.
	if idx < len(input)-512 {
		return input
	}

	cut := idx

	// If full SAUCE record is present, remove optional COMNT block too.
	if idx+128 <= len(input) {
		comments := int(input[idx+104])
		if comments > 0 {
			commentLen := 5 + (comments * 64)
			commentStart := idx - commentLen
			if commentStart >= 0 && bytes.Equal(input[commentStart:commentStart+5], []byte("COMNT")) {
				cut = commentStart
			}
		}
	}

	// Remove CP/M EOF marker if present before metadata.
	if cut > 0 && input[cut-1] == 0x1A {
		cut--
	}

	return input[:cut]
}

func lastCallerATTokenValue(code string, record user.CallRecord, totalUsers int, userNote string, timeLoc *time.Location) (string, bool) {
	switch code {
	case "USERCT":
		return strconv.Itoa(totalUsers), true
	case "NOTE", "NT":
		return userNote, true
	case "CA":
		return strconv.FormatUint(record.CallNumber, 10), true
	case "UN":
		return record.Handle, true
	case "LC":
		return record.GroupLocation, true
	case "ND":
		return strconv.Itoa(record.NodeID), true
	case "LO":
		if record.ConnectTime.IsZero() {
			return "", true
		}
		return formatLastCallerShortLocalTime(record.ConnectTime, timeLoc), true
	case "LT":
		if !record.DisconnectTime.IsZero() {
			return formatLastCallerShortLocalTime(record.DisconnectTime, timeLoc), true
		}
		if !record.ConnectTime.IsZero() {
			return formatLastCallerShortLocalTime(record.ConnectTime.Add(record.Duration), timeLoc), true
		}
		return "", true
	case "NU":
		if record.CallNumber <= 1 {
			return "*", true
		}
		return " ", true
	case "TO":
		return strconv.Itoa(int(record.Duration.Minutes())), true
	case "MP", "MR", "DL", "UL", "ES", "FS":
		return "0", true
	case "DK":
		return strconv.Itoa(int(record.DownloadedMB * 1024.0)), true
	case "UK":
		return strconv.Itoa(int(record.UploadedMB * 1024.0)), true
	default:
		return "", false
	}
}

func isLastCallerATNumeric(code string) bool {
	switch code {
	case "CA", "ND", "TO", "MP", "MR", "DL", "DK", "UL", "UK", "ES", "FS":
		return true
	default:
		return false
	}
}

func isLastCallerATCenterAligned(code string) bool {
	switch code {
	case "ND", "CA", "TO":
		return true
	default:
		return false
	}
}

func formatLastCallerATWidth(value string, width int, alignRight bool) string {
	if width == 0 {
		return value
	}

	if width < 0 {
		width = -width
		alignRight = true
	}

	runes := []rune(value)
	if len(runes) > width {
		runes = runes[:width]
	}
	value = string(runes)

	padding := width - utf8.RuneCountInString(value)
	if padding <= 0 {
		return value
	}

	pad := strings.Repeat(" ", padding)
	if alignRight {
		return pad + value
	}
	return value + pad
}

func formatLastCallerATWidthCentered(value string, width int) string {
	if width == 0 {
		return value
	}

	if width < 0 {
		width = -width
	}

	runes := []rune(value)
	if len(runes) > width {
		runes = runes[:width]
	}
	value = string(runes)

	padding := width - utf8.RuneCountInString(value)
	if padding <= 0 {
		return value
	}

	left := padding / 2
	right := padding - left
	return strings.Repeat(" ", left) + value + strings.Repeat(" ", right)
}

func formatLastCallerShortLocalTime(t time.Time, timeLoc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	if timeLoc == nil {
		timeLoc = time.Local
	}
	return t.In(timeLoc).Format("03:04pm")
}

func getLastCallerTimeLocation(configTZ string) *time.Location {
	tzName := strings.TrimSpace(configTZ)
	if tzName == "" {
		tzName = strings.TrimSpace(os.Getenv("VISION3_TIMEZONE"))
	}
	if tzName == "" {
		tzName = strings.TrimSpace(os.Getenv("TZ"))
	}

	if tzName != "" {
		if loc, err := time.LoadLocation(tzName); err == nil {
			return loc
		}
		log.Printf("WARN: Invalid LASTCALLERS timezone '%s'. Falling back to server local timezone.", tzName)
	}

	return time.Local
}

// displayFile reads and displays an ANSI file from the MENU SET's ansi directory.
func (e *MenuExecutor) displayFile(terminal *term.Terminal, filename string, outputMode ansi.OutputMode) error {
	// Construct full path using MenuSetPath
	filePath := filepath.Join(e.MenuSetPath, "ansi", filename)

	// Read ANSI content via helper (strips SAUCE metadata)
	data, err := ansi.GetAnsiFileContent(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to read ANSI file %s: %v", filePath, err)
		errMsg := fmt.Sprintf("\r\n|01Error loading file: %s|07\r\n", filename)
		// Use new helper, need outputMode... Pass it into displayFile?
		// Use the passed outputMode for the error message
		writeErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode) // Use passed outputMode
		if writeErr != nil {
			log.Printf("ERROR: Failed writing displayFile error message: %v", writeErr)
		}
		return writeErr
	}

	// Write the data using the new helper (this assumes displayFile is ONLY for ANSI files)
	// We should ideally process the file content using ProcessAnsiAndExtractCoords first,
	// but for a quick fix, let's assume CP437 output is desired here.
	// Use the passed outputMode for the file content
	err = terminalio.WriteProcessedBytes(terminal, data, outputMode) // Use passed outputMode
	if err != nil {
		log.Printf("ERROR: Failed to write ANSI file %s using WriteProcessedBytes: %v", filePath, err)
		return err
	}

	return nil
}

// displayPrompt handles rendering the menu prompt, including file includes and placeholder substitution.
// Added currentAreaName parameter
func (e *MenuExecutor) displayPrompt(terminal *term.Terminal, menu *MenuRecord, currentUser *user.User, userManager *user.UserMgr, nodeNumber int, currentMenuName string, sessionStartTime time.Time, outputMode ansi.OutputMode, currentAreaName string) error {
	promptParts := make([]string, 0, 2)
	if strings.TrimSpace(menu.Prompt1) != "" {
		promptParts = append(promptParts, menu.Prompt1)
	}
	if strings.TrimSpace(menu.Prompt2) != "" {
		promptParts = append(promptParts, menu.Prompt2)
	}

	if currentMenuName == "MAIN" {
		isAdmin := currentUser != nil && currentUser.AccessLevel >= 100
		pendingCount := pendingValidationCount(userManager)
		showValidationLine := isAdmin && pendingCount > 0
		if !showValidationLine {
			filtered := make([]string, 0, len(promptParts))
			for _, part := range promptParts {
				if strings.Contains(part, "|PV") {
					continue
				}
				filtered = append(filtered, part)
			}
			promptParts = filtered
		}
	}

	promptString := strings.Join(promptParts, "\r\n")

	// Special handling for MSGMENU prompt (Corrected menu name)
	if currentMenuName == "MSGMENU" && e.LoadedStrings.MessageMenuPrompt != "" {
		promptString = e.LoadedStrings.MessageMenuPrompt
		log.Printf("DEBUG: Using MessageMenuPrompt for MSGMENU")
	} else if promptString == "" {
		if e.LoadedStrings.DefPrompt != "" { // Use loaded strings
			promptString = e.LoadedStrings.DefPrompt
		} else {
			log.Printf("WARN: Default prompt (DefPrompt) is empty in config/strings.json and menu prompt fields are empty for menu %s. No prompt will be displayed.", currentMenuName)
			return nil // Explicitly return nil if no prompt string can be determined
		}
	}

	log.Printf("DEBUG: Displaying menu prompt for: %s", currentMenuName)

	placeholders := map[string]string{
		"|NODE":   strconv.Itoa(nodeNumber), // Node Number
		"|DATE":   time.Now().Format("01/02/06"),
		"|TIME":   time.Now().Format("15:04"),
		"|MN":     currentMenuName, // Menu Name
		"|PV":     "0",             // Pending validations
		"|ALIAS":  "Guest",         // Default
		"|HANDLE": "Guest",         // Default
		"|LEVEL":  "0",             // Default
		"|NAME":   "Guest User",    // Default
		"|PHONE":  "",              // Default
		"|UPLDS":  "0",             // Default
		"|DNLDS":  "0",             // Default
		"|POSTS":  "0",             // Default
		"|CALLS":  "0",             // Default
		"|LCALL":  "Never",         // Default
		"|TL":     "N/A",           // Default
		"|CA":     "None",          // Default
	}

	// Populate user-specific placeholders if logged in
	if currentUser != nil {
		placeholders["|ALIAS"] = currentUser.Handle
		placeholders["|HANDLE"] = currentUser.Handle
		placeholders["|LEVEL"] = strconv.Itoa(currentUser.AccessLevel)
		placeholders["|NAME"] = currentUser.RealName
		placeholders["|PHONE"] = currentUser.PhoneNumber
		placeholders["|UPLDS"] = strconv.Itoa(currentUser.NumUploads)
		placeholders["|CALLS"] = strconv.Itoa(currentUser.TimesCalled)
		if !currentUser.LastLogin.IsZero() {
			placeholders["|LCALL"] = currentUser.LastLogin.Format("01/02/06")
		}

		// Set |CA based on user's current area tag if available
		if currentUser.CurrentMessageAreaTag != "" {
			placeholders["|CA"] = currentUser.CurrentMessageAreaTag
			log.Printf("DEBUG: Using user's CurrentMessageAreaTag '%s' for |CA placeholder", currentUser.CurrentMessageAreaTag)
		} else {
			// Keep default "None" if user tag is empty
			log.Printf("DEBUG: User's CurrentMessageAreaTag is empty, using default 'None' for |CA placeholder")
		}

		// Calculate Time Left |TL
		if currentUser.TimeLimit <= 0 {
			placeholders["|TL"] = "Unlimited"
		} else {
			elapsedSeconds := time.Since(sessionStartTime).Seconds()
			totalSeconds := float64(currentUser.TimeLimit * 60)
			remainingSeconds := totalSeconds - elapsedSeconds
			if remainingSeconds < 0 {
				remainingSeconds = 0
			}
			remainingMinutes := int(remainingSeconds / 60)
			placeholders["|TL"] = strconv.Itoa(remainingMinutes)
		}

		if currentMenuName == "MAIN" && currentUser.AccessLevel >= 100 {
			placeholders["|PV"] = strconv.Itoa(pendingValidationCount(userManager))
		}
	} // End if currentUser != nil

	substitutedPrompt := promptString
	for key, val := range placeholders {
		substitutedPrompt = strings.ReplaceAll(substitutedPrompt, key, val) // Corrected keys from |KEY| to |KEY
		substitutedPrompt = strings.ReplaceAll(substitutedPrompt, key, val)
	}

	processedPrompt, err := e.processFileIncludes(substitutedPrompt, 0) // Pass 'e'
	if err != nil {
		log.Printf("ERROR: Failed processing file includes in prompt for menu %s: %v", currentMenuName, err)

		// Use RootAssetsPath for global assets if needed, or MenuSetPath for set-specific
		// pausePrompt := e.LoadedStrings.PauseString // This comes from global strings
		// ... (rest of pause logic) ...
		return err // Use original error if includes fail
	}

	// 3. Process pipe codes in the final string (includes/placeholders already processed)
	rawPromptBytes := ansi.ReplacePipeCodes([]byte(processedPrompt))

	// 4. Process character encoding based on outputMode (Reverted to manual loop)
	var finalBuf bytes.Buffer
	finalBuf.Write([]byte("\r\n")) // Add newline prefix

	for i := 0; i < len(rawPromptBytes); i++ {
		b := rawPromptBytes[i]
		if b < 128 || outputMode == ansi.OutputModeCP437 {
			// ASCII or CP437 mode, write raw byte
			finalBuf.WriteByte(b)
		} else {
			// UTF-8 mode, convert extended characters
			r := ansi.Cp437ToUnicode[b] // Use the exported map
			if r == 0 && b != 0 {
				finalBuf.WriteByte('?') // Fallback
			} else if r != 0 {
				finalBuf.WriteRune(r)
			}
		}
	}

	// 5. Write the final processed bytes using the terminal's standard Write (Reverted)
	err = terminalio.WriteProcessedBytes(terminal, finalBuf.Bytes(), outputMode)
	if err != nil {
		log.Printf("ERROR: Failed writing processed prompt for menu %s: %v", currentMenuName, err)
		return err
	}

	return nil
}

// processFileIncludes recursively replaces %%filename.ans tags with file content.
// It now looks for included files within the MENU SET's ansi directory.
func (e *MenuExecutor) processFileIncludes(prompt string, depth int) (string, error) {
	const maxDepth = 5 // Limit recursion depth
	if depth > maxDepth {
		log.Printf("WARN: Exceeded maximum file inclusion depth (%d). Stopping processing.", maxDepth)
		return prompt, nil
	}

	re := regexp.MustCompile(`%%([a-zA-Z0-9_\-]+\.[a-zA-Z0-9]+)%%`)
	processedAny := false
	result := re.ReplaceAllStringFunc(prompt, func(match string) string {
		processedAny = true
		fileName := re.FindStringSubmatch(match)[1]
		// Look for included file in MenuSetPath/ansi
		filePath := filepath.Join(e.MenuSetPath, "ansi", fileName)

		log.Printf("DEBUG: Including file in prompt: %s (Depth: %d)", filePath, depth)
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("WARN: Failed to read included file '%s': %v. Skipping inclusion.", filePath, err)
			return ""
		}
		return string(data)
	})

	if processedAny {
		return e.processFileIncludes(result, depth+1)
	}

	return result, nil
}

// runNewMailScan checks for new private mail and displays a count to the user.
func runNewMailScan(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running NMAILSCAN for user %s", nodeNumber, currentUser.Handle)

	if currentUser == nil {
		return nil, "", nil
	}

	if e.MessageMgr == nil {
		log.Printf("WARN: Node %d: MessageMgr not available for NMAILSCAN", nodeNumber)
		return currentUser, "", nil
	}

	// Get PRIVMAIL area
	privmailArea, exists := e.MessageMgr.GetAreaByTag("PRIVMAIL")
	if !exists {
		log.Printf("DEBUG: Node %d: PRIVMAIL area not configured, skipping mail scan", nodeNumber)
		return currentUser, "", nil
	}

	// Get JAM base for PRIVMAIL area
	base, err := e.MessageMgr.GetBase(privmailArea.ID)
	if err != nil {
		log.Printf("WARN: Node %d: JAM base not open for PRIVMAIL area: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	// Get total message count
	totalMessages, err := e.MessageMgr.GetMessageCountForArea(privmailArea.ID)
	if err != nil {
		log.Printf("WARN: Node %d: Failed to get message count for PRIVMAIL: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	if totalMessages == 0 {
		msg := "\r\n|07No new private mail.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		return currentUser, "", nil
	}

	// Get lastread pointer for this user
	lastRead, err := e.MessageMgr.GetLastRead(privmailArea.ID, currentUser.Handle)
	if err != nil {
		log.Printf("WARN: Node %d: Failed to get lastread for PRIVMAIL: %v", nodeNumber, err)
		lastRead = 0
	}

	// Count unread private messages addressed to this user
	newMailCount := 0
	for msgNum := lastRead + 1; msgNum <= totalMessages; msgNum++ {
		msg, readErr := base.ReadMessage(msgNum)
		if readErr != nil {
			continue
		}
		if msg.IsDeleted() {
			continue
		}
		if msg.IsPrivate() && strings.EqualFold(msg.To, currentUser.Handle) {
			newMailCount++
		}
	}

	if newMailCount > 0 {
		mailMsg := fmt.Sprintf("\r\n|14You have |15%d|14 new private mail message(s).|07\r\n", newMailCount)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(mailMsg)), outputMode)
	} else {
		msg := "\r\n|07No new private mail.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	}

	return currentUser, "", nil
}

// runLoginDisplayFile displays an ANSI file during the login sequence.
// The filename is passed via the args parameter (from LoginItem.Data).
func runLoginDisplayFile(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	filename := strings.TrimSpace(args)
	if filename == "" {
		log.Printf("WARN: Node %d: DISPLAYFILE called with no filename", nodeNumber)
		return currentUser, "", nil
	}

	log.Printf("DEBUG: Node %d: Running DISPLAYFILE for %s", nodeNumber, filename)

	err := e.displayFile(terminal, filename, outputMode)
	if err != nil {
		log.Printf("WARN: Node %d: Failed to display file %s: %v", nodeNumber, filename, err)
		// Non-fatal - continue login sequence even if file is missing
	}

	return currentUser, "", nil
}

// runLoginDoor executes a script/program during the login sequence.
// The script path is passed via the args parameter (from LoginItem.Data).
// The node number is passed as the first argument to the script.
func runLoginDoor(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	scriptPath := strings.TrimSpace(args)
	if scriptPath == "" {
		log.Printf("WARN: Node %d: RUNDOOR called with no script path", nodeNumber)
		return currentUser, "", nil
	}

	log.Printf("INFO: Node %d: Running login door script: %s", nodeNumber, scriptPath)

	// Verify script exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		log.Printf("WARN: Node %d: Login door script not found: %s", nodeNumber, scriptPath)
		return currentUser, "", nil
	}

	// Execute the script with node number as argument
	cmd := exec.Command(scriptPath, strconv.Itoa(nodeNumber))
	cmd.Stdin = s
	cmd.Stdout = s
	cmd.Stderr = s.Stderr()

	if err := cmd.Run(); err != nil {
		log.Printf("WARN: Node %d: Login door script %s exited with error: %v", nodeNumber, scriptPath, err)
		// Non-fatal - continue login sequence
	}

	return currentUser, "", nil
}

// runFastLogin presents the FASTLOGN menu inline during the login sequence.
// Returns a GOTO action if the user chooses to skip/jump, or empty string to continue.
func runFastLogin(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running FASTLOGIN inline for user %s", nodeNumber, currentUser.Handle)

	// Load FASTLOGN menu definition (.MNU) for CLR/CLS + prompt behavior
	var fastlognMenu *MenuRecord
	menuMnuPath := filepath.Join(e.MenuSetPath, "mnu")
	loadedMenu, menuErr := LoadMenu("FASTLOGN", menuMnuPath)
	if menuErr != nil {
		log.Printf("WARN: Node %d: Failed to load FASTLOGN.MNU: %v", nodeNumber, menuErr)
	} else {
		fastlognMenu = loadedMenu
	}

	renderFastLoginScreen := func() {
		if fastlognMenu != nil && fastlognMenu.GetClrScrBefore() {
			_ = terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
		}

		if displayErr := e.displayFile(terminal, "FASTLOGN.ANS", outputMode); displayErr != nil {
			log.Printf("WARN: Node %d: Failed to display FASTLOGN.ANS: %v", nodeNumber, displayErr)
		}

		if fastlognMenu != nil && fastlognMenu.GetUsePrompt() {
			promptParts := make([]string, 0, 2)
			if strings.TrimSpace(fastlognMenu.Prompt1) != "" {
				promptParts = append(promptParts, fastlognMenu.Prompt1)
			}
			if strings.TrimSpace(fastlognMenu.Prompt2) != "" {
				promptParts = append(promptParts, fastlognMenu.Prompt2)
			}
			if len(promptParts) > 0 {
				prompt := "\r\n" + strings.Join(promptParts, "\r\n")
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
			}
		}
	}

	// Load FASTLOGN commands
	cfgPath := filepath.Join(e.MenuSetPath, "cfg")
	commands, err := LoadCommands("FASTLOGN", cfgPath)
	if err != nil {
		log.Printf("WARN: Node %d: Failed to load FASTLOGN.CFG: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	renderFastLoginScreen()

	bufioReader := bufio.NewReader(s)
	for {
		r, _, readErr := bufioReader.ReadRune()
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return currentUser, "", readErr
		}

		if r == 27 {
			for bufioReader.Buffered() > 0 {
				_, _ = bufioReader.ReadByte()
			}
			continue
		}

		if r < 32 || r == 127 {
			continue
		}

		keyStr := strings.ToUpper(string(r))
		if r == '/' {
			time.Sleep(25 * time.Millisecond)
			if bufioReader.Buffered() > 0 {
				nextRune, _, nextErr := bufioReader.ReadRune()
				if nextErr == nil {
					keyStr = "/" + strings.ToUpper(string(nextRune))
				}
			}
		}

		// Match against configured commands
		for _, cmd := range commands {
			keys := strings.Fields(strings.ToUpper(cmd.Keys))
			matchedKey := false
			for _, key := range keys {
				if key == keyStr {
					matchedKey = true
					break
				}
			}

			if matchedKey {
				// Check ACS
				if cmd.ACS != "" && cmd.ACS != "*" {
					if !checkACS(cmd.ACS, currentUser, s, terminal, sessionStartTime) {
						continue
					}
				}

				// If this is the full login sequence command, return empty to continue
				if cmd.Command == "RUN:FULL_LOGIN_SEQUENCE" {
					log.Printf("DEBUG: Node %d: FASTLOGIN - user chose to continue full sequence", nodeNumber)
					terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
					return currentUser, "", nil
				}

				// For GOTO commands, return the action to break out of login sequence
				if strings.HasPrefix(cmd.Command, "GOTO:") {
					nextMenu := strings.ToUpper(strings.TrimPrefix(cmd.Command, "GOTO:"))
					log.Printf("DEBUG: Node %d: FASTLOGIN - user chose GOTO:%s", nodeNumber, nextMenu)
					terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
					return currentUser, "GOTO:" + nextMenu, nil
				}

				if cmd.Command == "LOGOFF" {
					return currentUser, "LOGOFF", nil
				}
			}
		}

		// If Enter was pressed with no match, continue sequence
		if r == '\r' || r == '\n' {
			return currentUser, "", nil
		}

		e.showUndefinedMenuInput(terminal, outputMode, nodeNumber)
		renderFastLoginScreen()
	}
}

// loginPausePrompt displays the configured pause prompt and waits for Enter.
func (e *MenuExecutor) loginPausePrompt(s ssh.Session, terminal *term.Terminal, nodeNumber int, outputMode ansi.OutputMode) error {
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... "
	}

	// Process pipe codes and convert to CP437 if needed
	var pauseBytesToWrite []byte
	processedPausePrompt := ansi.ReplacePipeCodes([]byte(pausePrompt))
	if outputMode == ansi.OutputModeCP437 {
		var cp437Buf bytes.Buffer
		for _, r := range string(processedPausePrompt) {
			if r < 128 {
				cp437Buf.WriteByte(byte(r))
			} else if cp437Byte, ok := ansi.UnicodeToCP437[r]; ok {
				cp437Buf.WriteByte(cp437Byte)
			} else {
				cp437Buf.WriteByte('?')
			}
		}
		pauseBytesToWrite = cp437Buf.Bytes()
	} else {
		pauseBytesToWrite = processedPausePrompt
	}

	wErr := terminalio.WriteProcessedBytes(terminal, pauseBytesToWrite, outputMode)
	if wErr != nil {
		return wErr
	}

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			return err
		}
		if r == '\r' || r == '\n' {
			break
		}
	}
	return nil
}

// RunLoginSequence is the exported entry point for running the login sequence from main.go.
// Returns the next menu name to enter (e.g., "MAIN") or "LOGOFF".
func (e *MenuExecutor) RunLoginSequence(s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, outputMode ansi.OutputMode) (string, error) {
	_, nextAction, err := runFullLoginSequence(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, "", outputMode)
	if err != nil {
		return "LOGOFF", err
	}
	// Parse "GOTO:MAIN" -> "MAIN", pass through "LOGOFF" as-is
	if strings.HasPrefix(nextAction, "GOTO:") {
		return strings.ToUpper(strings.TrimPrefix(nextAction, "GOTO:")), nil
	}
	if nextAction == "LOGOFF" {
		return "LOGOFF", nil
	}
	return "MAIN", nil
}

// runFullLoginSequence executes the configurable login sequence from login.json.
func runFullLoginSequence(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	loginSequence := e.GetLoginSequence()
	log.Printf("INFO: Node %d: Running FULL_LOGIN_SEQUENCE for user %s (%d items configured)", nodeNumber, currentUser.Handle, len(loginSequence))

	// Build dispatch map for login item commands
	type loginHandler func(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error)

	handlers := map[string]loginHandler{
		"LASTCALLS":   runLastCallers,
		"ONELINERS":   runOneliners,
		"USERSTATS":   runShowStats,
		"NMAILSCAN":   runNewMailScan,
		"DISPLAYFILE": runLoginDisplayFile,
		"RUNDOOR":     runLoginDoor,
		"FASTLOGIN":   runFastLogin,
	}

	for i, item := range loginSequence {
		// Check security level requirement
		if item.SecLevel > 0 && currentUser.AccessLevel < item.SecLevel {
			log.Printf("DEBUG: Node %d: Skipping login item %d (%s) - user level %d < required %d",
				nodeNumber, i+1, item.Command, currentUser.AccessLevel, item.SecLevel)
			continue
		}

		log.Printf("DEBUG: Node %d: Executing login item %d/%d: %s", nodeNumber, i+1, len(loginSequence), item.Command)

		// Clear screen if requested
		if item.ClearScreen {
			terminalio.WriteProcessedBytes(terminal, []byte("\x1b[2J\x1b[H"), outputMode)
		}

		// Check if this is a DOOR: command
		var nextAction string
		var err error
		if strings.HasPrefix(item.Command, "DOOR:") {
			// Extract door name and execute via DOOR: handler
			doorName := strings.TrimPrefix(item.Command, "DOOR:")
			log.Printf("INFO: Node %d: Executing door '%s' from login sequence", nodeNumber, doorName)

			// Call the DOOR: handler from RunRegistry
			if doorFunc, exists := e.RunRegistry["DOOR:"]; exists {
				_, nextAction, err = doorFunc(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, doorName, outputMode)
			} else {
				log.Printf("ERROR: Node %d: DOOR: handler not registered", nodeNumber)
				continue
			}
		} else {
			// Look up and execute the handler from the local handlers map
			handler, exists := handlers[item.Command]
			if !exists {
				log.Printf("WARN: Node %d: Unknown login sequence command: %s", nodeNumber, item.Command)
				continue
			}

			// Pass item.Data as the args parameter for commands that need it
			itemArgs := args
			if item.Data != "" {
				itemArgs = item.Data
			}

			_, nextAction, err = handler(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, itemArgs, outputMode)
		}
		if err != nil {
			log.Printf("ERROR: Node %d: Error during login item %s: %v", nodeNumber, item.Command, err)
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			// Non-fatal errors - continue with next item
		}

		// Check if the handler requested navigation (GOTO/LOGOFF)
		if nextAction == "LOGOFF" {
			return nil, "LOGOFF", nil
		}
		if strings.HasPrefix(nextAction, "GOTO:") {
			log.Printf("DEBUG: Node %d: Login sequence interrupted by %s -> %s", nodeNumber, item.Command, nextAction)
			return currentUser, nextAction, nil
		}

		// Pause after if requested
		if item.PauseAfter {
			if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
				if errors.Is(pauseErr, io.EOF) {
					return nil, "LOGOFF", io.EOF
				}
			}
		}
	}

	// Sequence completed - transition to MAIN menu
	log.Printf("DEBUG: Node %d: FULL_LOGIN_SEQUENCE completed. Transitioning to MAIN.", nodeNumber)
	return currentUser, "GOTO:MAIN", nil
}

// Define needed ANSI attributes
const (
	attrInverse = "\x1b[7m" // Inverse video - Keep for fallback?
	attrReset   = "\x1b[0m" // Reset attributes
)

// LightbarOption represents a single option in a lightbar menu
type LightbarOption struct {
	X, Y           int    // Screen coordinates
	Text           string // Display text
	HotKey         string // Command hotkey
	HighlightColor int    // Color code when highlighted
	RegularColor   int    // Color code when not highlighted
}

// ANSI foreground color codes (standard and bright)
var ansiFg = map[int]int{
	0: 30, 1: 34, 2: 32, 3: 36, 4: 31, 5: 35, 6: 33, 7: 37, // Standard
	8: 90, 9: 94, 10: 92, 11: 96, 12: 91, 13: 95, 14: 93, 15: 97, // Bright
}

// ANSI background color codes (standard)
var ansiBg = map[int]int{
	0: 40, 1: 44, 2: 42, 3: 46, 4: 41, 5: 45, 6: 43, 7: 47,
}

// colorCodeToAnsi converts a DOS-style color code (0-255) to ANSI escape sequence.
// Assumes Color = Background*16 + Foreground
func colorCodeToAnsi(code int) string {
	fgCode := code % 16
	bgCode := code / 16

	fgAnsi, okFg := ansiFg[fgCode]
	if !okFg {
		fgAnsi = 97 // Default to bright white if invalid fg code
	}

	// Use standard background colors (40-47). Bright backgrounds (100-107) have less support.
	bgAnsi, okBg := ansiBg[bgCode%8]
	if !okBg {
		bgAnsi = 40 // Default to black background if invalid bg code
	}

	return fmt.Sprintf("\x1b[%d;%dm", fgAnsi, bgAnsi)
}

// loadLightbarOptions loads and parses lightbar options from configuration files
func loadLightbarOptions(menuName string, e *MenuExecutor) ([]LightbarOption, error) {
	// Determine paths using MenuSetPath
	cfgFilename := menuName + ".CFG"
	barFilename := menuName + ".BAR"
	cfgPath := filepath.Join(e.MenuSetPath, "cfg", cfgFilename)
	barPath := filepath.Join(e.MenuSetPath, "bar", barFilename)

	log.Printf("DEBUG: Loading CFG: %s", cfgPath)
	log.Printf("DEBUG: Loading BAR: %s", barPath)

	// Load commands from CFG file using the proper JSON loader
	commandsByHotkey := make(map[string]string)
	configPath := filepath.Join(e.MenuSetPath, "cfg")
	commands, err := LoadCommands(menuName, configPath)
	if err != nil {
		log.Printf("WARN: Failed to load CFG file %s: %v", cfgPath, err)
	} else {
		// Build hotkey -> command mapping for validation
		for _, cmd := range commands {
			hotkey := strings.ToUpper(strings.TrimSpace(cmd.Keys))
			commandsByHotkey[hotkey] = cmd.Command
		}
	}

	// Parse BAR file
	barFile, err := os.Open(barPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open BAR file %s: %w", barPath, err)
	}
	defer barFile.Close()

	var options []LightbarOption
	scanner := bufio.NewScanner(barFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue // Skip empty lines and comments
		}

		// Parse record in format: X,Y,HotKey,DisplayText // OLD Format
		// Parse record in format: X,Y,HiLitedColor,RegularColor,HotKey,ReturnValue,HiLitedString // NEW Format
		parts := strings.SplitN(line, ",", 7) // Split into 7 parts
		if len(parts) != 7 {                  // Check for 7 parts
			log.Printf("WARN: Malformed BAR line (expected 7 fields): %s", line)
			continue
		}

		x, xerr := strconv.Atoi(strings.TrimSpace(parts[0]))
		y, yerr := strconv.Atoi(strings.TrimSpace(parts[1]))
		if xerr != nil || yerr != nil {
			log.Printf("WARN: Invalid coordinates in BAR line: %s", line)
			continue
		}

		// Parse color codes
		highlightColor, hcErr := strconv.Atoi(strings.TrimSpace(parts[2]))
		regularColor, rcErr := strconv.Atoi(strings.TrimSpace(parts[3]))
		if hcErr != nil || rcErr != nil {
			log.Printf("WARN: Invalid color codes in BAR line: %s", line)
			// Default colors? Or skip?
			highlightColor = 7 // Default: White on Black (inverse)
			regularColor = 15  // Default: Bright White on Black
		}

		hotkey := strings.ToUpper(strings.TrimSpace(parts[4])) // HotKey is the 5th field (index 4)
		// Field 5 is ReturnValue - ignore for now
		displayText := strings.TrimSpace(parts[6]) // DisplayText is the 7th field (index 6)

		// Verify the hotkey maps to a command
		if _, exists := commandsByHotkey[hotkey]; !exists {
			log.Printf("WARN: Hotkey '%s' in BAR file has no matching command in CFG", hotkey)
		}

		options = append(options, LightbarOption{
			X:              x,
			Y:              y,
			Text:           displayText,
			HotKey:         hotkey,
			HighlightColor: highlightColor,
			RegularColor:   regularColor,
		})
	}

	return options, nil
}

// drawLightbarMenu draws the lightbar menu with the specified option selected
func drawLightbarOption(terminal *term.Terminal, opt LightbarOption, selected bool, outputMode ansi.OutputMode) error {
	posCmd := fmt.Sprintf("\x1b[%d;%dH", opt.Y, opt.X)
	err := terminalio.WriteProcessedBytes(terminal, []byte(posCmd), outputMode)
	if err != nil {
		return fmt.Errorf("failed positioning cursor for lightbar option: %w", err)
	}

	colorCode := opt.RegularColor
	if selected {
		colorCode = opt.HighlightColor
	}
	ansiColorSequence := colorCodeToAnsi(colorCode)
	err = terminalio.WriteProcessedBytes(terminal, []byte(ansiColorSequence), outputMode)
	if err != nil {
		return fmt.Errorf("failed setting color for lightbar option: %w", err)
	}

	err = terminalio.WriteProcessedBytes(terminal, []byte(opt.Text), outputMode)
	if err != nil {
		return fmt.Errorf("failed writing lightbar option text: %w", err)
	}

	err = terminalio.WriteProcessedBytes(terminal, []byte(attrReset), outputMode)
	if err != nil {
		return fmt.Errorf("failed resetting attributes after lightbar option: %w", err)
	}

	return nil
}

func drawLightbarMenu(terminal *term.Terminal, backgroundBytes []byte, options []LightbarOption, selectedIndex int, outputMode ansi.OutputMode, drawBackground bool) error {
	if drawBackground {
		err := terminalio.WriteProcessedBytes(terminal, backgroundBytes, outputMode)
		if err != nil {
			return fmt.Errorf("failed writing lightbar background: %w", err)
		}
	}

	for i, opt := range options {
		if err := drawLightbarOption(terminal, opt, i == selectedIndex, outputMode); err != nil {
			return err
		}
	}

	return nil
}

// Helper function to request and parse cursor position
// Returns row, col, error
func requestCursorPosition(s ssh.Session, terminal *term.Terminal) (int, int, error) {
	// Ensure terminal is in a state to respond (raw mode might be needed temporarily,
	// but the main loop often handles raw mode via terminal.ReadLine() or pty)
	// If not in raw mode, the response might not be read correctly.

	err := terminalio.WriteProcessedBytes(terminal, []byte("\x1b[6n"), ansi.OutputModeAuto) // DSR - Device Status Report - Request cursor position
	if err != nil {
		return 0, 0, fmt.Errorf("failed to send cursor position request: %w", err)
	}

	// Read the response, typically \x1b[<row>;<col>R
	// This is tricky and needs robust parsing. A simple ReadRune loop might not suffice
	// if other data arrives or if the response format varies slightly.
	// We need to read until 'R', accumulating digits.
	var response []byte
	reader := bufio.NewReader(s)                  // Use the session reader
	timeout := time.After(500 * time.Millisecond) // Add a timeout

	log.Printf("DEBUG: Waiting for cursor position report...")

	for {
		select {
		case <-timeout:
			log.Printf("WARN: Timeout waiting for cursor position report.")
			return 0, 0, fmt.Errorf("timeout waiting for cursor position report")
		default:
			b, err := reader.ReadByte()
			if err != nil {
				// Check for EOF specifically
				if errors.Is(err, io.EOF) {
					log.Printf("WARN: EOF received while waiting for cursor position report.")
					return 0, 0, io.EOF
				}
				log.Printf("ERROR: Error reading byte for cursor position report: %v", err)
				return 0, 0, fmt.Errorf("error reading cursor position report: %w", err)
			}

			response = append(response, b)
			// log.Printf("DEBUG: Read byte: %d (%c)", b, b) // Verbose logging

			// Check if we have the expected end marker 'R'
			if b == 'R' {
				// Also check if the response starts with \x1b[
				if !bytes.HasPrefix(response, []byte("\x1b[")) {
					log.Printf("WARN: Invalid cursor position report format (missing ESC [): %q", string(response))
					return 0, 0, fmt.Errorf("invalid cursor position report format: %q", response)
				}
				// Extract the part between '[' and 'R'
				payload := response[2 : len(response)-1]
				parts := bytes.Split(payload, []byte(";"))
				if len(parts) != 2 {
					log.Printf("WARN: Invalid cursor position report format (expected row;col): %q", string(response))
					return 0, 0, fmt.Errorf("invalid cursor position report format: %q", response)
				}

				row, errRow := strconv.Atoi(string(parts[0]))
				col, errCol := strconv.Atoi(string(parts[1]))

				if errRow != nil || errCol != nil {
					log.Printf("WARN: Failed to parse row/col from cursor report %q: RowErr=%v, ColErr=%v", string(response), errRow, errCol)
					return 0, 0, fmt.Errorf("failed to parse cursor position report %q", response)
				}
				log.Printf("DEBUG: Received cursor position: Row=%d, Col=%d", row, col)
				return row, col, nil // Success!
			}

			// Prevent infinitely growing buffer if 'R' is never received
			if len(response) > 32 {
				log.Printf("WARN: Cursor position report buffer exceeded limit without finding 'R': %q", string(response))
				return 0, 0, fmt.Errorf("cursor position report too long or invalid")
			}
		}
	}
}

// promptYesNo is the canonical Yes/No prompt entrypoint for menu flows.
// Keep all call sites routed here so prompt behavior can be changed in one place.
func (e *MenuExecutor) promptYesNo(s ssh.Session, terminal *term.Terminal, promptText string, outputMode ansi.OutputMode, nodeNumber int) (bool, error) {
	return e.promptYesNoLightbar(s, terminal, promptText, outputMode, nodeNumber)
}

// promptYesNoLightbar displays a Yes/No prompt with lightbar selection.
// Returns true for Yes, false for No, and error on issues like disconnect.
func (e *MenuExecutor) promptYesNoLightbar(s ssh.Session, terminal *term.Terminal, promptText string, outputMode ansi.OutputMode, nodeNumber int) (bool, error) {
	// Strip trailing ' @' — ViSiON/2 convention for Yes/No prompt terminator.
	// The '@' signals WriteStr to render an interactive Yes/No lightbar.
	promptText = strings.TrimSuffix(promptText, " @")
	promptText = strings.TrimSuffix(promptText, "@")

	// Use nodeNumber in logging calls instead of e.nodeID
	ptyReq, _, isPty := s.Pty()
	hasPtyHeight := isPty && ptyReq.Window.Height > 0

	if hasPtyHeight {
		// --- Dynamic Lightbar Logic (if terminal height is known) ---
		log.Printf("DEBUG: Terminal height known (%d), using lightbar prompt.", ptyReq.Window.Height)
		wErr := terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
		if wErr != nil {
			log.Printf("WARN: Node %d: Failed hiding cursor for Yes/No prompt: %v", nodeNumber, wErr)
		}
		defer func() {
			restoreErr := terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode)
			if restoreErr != nil {
				log.Printf("WARN: Node %d: Failed restoring cursor for Yes/No prompt: %v", nodeNumber, restoreErr)
			}
		}()

		promptRow := ptyReq.Window.Height // Use last row
		promptCol := 3
		yesLabel := strings.TrimSpace(e.LoadedStrings.YesPromptText)
		if yesLabel == "" {
			yesLabel = "Yes"
		}
		noLabel := strings.TrimSpace(e.LoadedStrings.NoPromptText)
		if noLabel == "" {
			noLabel = "No"
		}

		yesOptionText := " " + yesLabel + " "
		noOptionText := " " + noLabel + " "
		yesNoSpacing := 2  // Spaces between prompt and first option (after cursor)
		optionSpacing := 2 // Spaces between Yes and No
		highlightColor := e.Theme.YesNoHighlightColor
		regularColor := e.Theme.YesNoRegularColor

		// Use WriteProcessedBytes for ANSI codes
		saveCursorBytes := []byte(ansi.SaveCursor())
		log.Printf("DEBUG: Node %d: Writing prompt save cursor bytes (hex): %x", nodeNumber, saveCursorBytes) // Use nodeNumber
		wErr = terminalio.WriteProcessedBytes(terminal, saveCursorBytes, outputMode)
		if wErr != nil {
			log.Printf("WARN: Failed saving cursor: %v", wErr)
		}
		defer func() {
			restoreCursorBytes := []byte(ansi.RestoreCursor())
			log.Printf("DEBUG: Node %d: Writing prompt restore cursor bytes (hex): %x", nodeNumber, restoreCursorBytes) // Use nodeNumber
			wErr := terminalio.WriteProcessedBytes(terminal, restoreCursorBytes, outputMode)
			if wErr != nil {
				log.Printf("WARN: Failed restoring cursor: %v", wErr)
			}
		}()

		// Clear the prompt line first
		clearCmdBytes := []byte(fmt.Sprintf("\x1b[%d;1H\x1b[2K", promptRow))                               // Move + Clear line
		log.Printf("DEBUG: Node %d: Writing prompt clear line bytes (hex): %x", nodeNumber, clearCmdBytes) // Use nodeNumber
		wErr = terminalio.WriteProcessedBytes(terminal, clearCmdBytes, outputMode)
		if wErr != nil {
			log.Printf("WARN: Failed clearing prompt line: %v", wErr)
		}

		// Move to prompt column and display prompt text
		promptPosCmdBytes := []byte(fmt.Sprintf("\x1b[%d;%dH", promptRow, promptCol))
		log.Printf("DEBUG: Node %d: Writing prompt position bytes (hex): %x", nodeNumber, promptPosCmdBytes) // Use nodeNumber
		wErr = terminalio.WriteProcessedBytes(terminal, promptPosCmdBytes, outputMode)
		if wErr != nil {
			log.Printf("WARN: Failed positioning for prompt: %v", wErr)
		}

		promptDisplayBytes := ansi.ReplacePipeCodes([]byte(promptText))
		log.Printf("DEBUG: Node %d: Writing prompt text bytes (hex): %x", nodeNumber, promptDisplayBytes) // Use nodeNumber
		err := terminalio.WriteStringCP437(terminal, promptDisplayBytes, outputMode)
		if err != nil {
			log.Printf("ERROR: Node %d: Failed writing Yes/No prompt text (lightbar mode): %v", nodeNumber, err) // Use nodeNumber
			return false, fmt.Errorf("failed writing prompt text: %w", err)
		}

		_, currentCursorCol, err := requestCursorPosition(s, terminal)
		if err != nil {
			log.Printf("ERROR: Failed getting cursor position for Yes/No prompt: %v", err)
			// Fallback to text prompt if cursor position fails?
			// For now, return error, as layout depends on it.
			return false, fmt.Errorf("failed getting cursor position: %w", err)
		}

		yesOptionCol := currentCursorCol + yesNoSpacing
		noOptionCol := yesOptionCol + len(yesOptionText) + optionSpacing

		yesOption := LightbarOption{
			X: yesOptionCol, Y: promptRow,
			Text: yesOptionText, HotKey: "Y",
			HighlightColor: highlightColor, RegularColor: regularColor,
		}
		noOption := LightbarOption{
			X: noOptionCol, Y: promptRow,
			Text: noOptionText, HotKey: "N",
			HighlightColor: highlightColor, RegularColor: regularColor,
		}
		options := []LightbarOption{noOption, yesOption} // No=0, Yes=1
		selectedIndex := 0                               // Default to 'No'

		drawOptions := func(currentSelection int) {
			// Use WriteProcessedBytes within drawOptions
			saveCursorBytes := []byte(ansi.SaveCursor())
			log.Printf("DEBUG: Node %d: Writing prompt drawOpt save cursor bytes (hex): %x", nodeNumber, saveCursorBytes) // Use nodeNumber
			wErr := terminalio.WriteProcessedBytes(terminal, saveCursorBytes, outputMode)
			if wErr != nil {
				log.Printf("WARN: Failed saving cursor in drawOptions: %v", wErr)
			}
			defer func() {
				restoreCursorBytes := []byte(ansi.RestoreCursor())
				log.Printf("DEBUG: Node %d: Writing prompt drawOpt restore cursor bytes (hex): %x", nodeNumber, restoreCursorBytes) // Use nodeNumber
				wErr := terminalio.WriteProcessedBytes(terminal, restoreCursorBytes, outputMode)
				if wErr != nil {
					log.Printf("WARN: Failed restoring cursor in drawOptions: %v", wErr)
				}
			}()

			for i, opt := range options {
				if opt.X <= 0 || opt.Y <= 0 {
					log.Printf("WARN: Invalid coordinates for Yes/No option %d: X=%d, Y=%d", i, opt.X, opt.Y)
					continue
				}
				posCmdBytes := []byte(fmt.Sprintf("\x1b[%d;%dH", opt.Y, opt.X))
				log.Printf("DEBUG: Node %d: Writing prompt option %d position bytes (hex): %x", nodeNumber, i, posCmdBytes) // Use nodeNumber
				wErr = terminalio.WriteProcessedBytes(terminal, posCmdBytes, outputMode)
				if wErr != nil {
					log.Printf("WARN: Failed positioning cursor for option %d: %v", i, wErr)
				}

				colorCode := opt.RegularColor
				if i == currentSelection {
					colorCode = opt.HighlightColor
				}
				ansiColorSequenceBytes := []byte(colorCodeToAnsi(colorCode))
				log.Printf("DEBUG: Node %d: Writing prompt option %d color bytes (hex): %x", nodeNumber, i, ansiColorSequenceBytes) // Use nodeNumber
				wErr = terminalio.WriteProcessedBytes(terminal, ansiColorSequenceBytes, outputMode)
				if wErr != nil {
					log.Printf("WARN: Failed setting color for option %d: %v", i, wErr)
				}

				optionTextBytes := []byte(opt.Text)
				log.Printf("DEBUG: Node %d: Writing prompt option %d text bytes (hex): %x", nodeNumber, i, optionTextBytes) // Use nodeNumber
				wErr = terminalio.WriteProcessedBytes(terminal, optionTextBytes, outputMode)
				if wErr != nil {
					log.Printf("WARN: Failed writing text for option %d: %v", i, wErr)
				}

				resetBytes := []byte("\x1b[0m")                                                                         // Reset attributes
				log.Printf("DEBUG: Node %d: Writing prompt option %d reset bytes (hex): %x", nodeNumber, i, resetBytes) // Use nodeNumber
				wErr = terminalio.WriteProcessedBytes(terminal, resetBytes, outputMode)
				if wErr != nil {
					log.Printf("WARN: Failed resetting attributes for option %d: %v", i, wErr)
				}
			}
		}

		drawOptions(selectedIndex)

		bufioReader := bufio.NewReader(s)
		for {
			// Move cursor back to where prompt ended for input visual
			posCmd := fmt.Sprintf("\x1b[%d;%dH", promptRow, currentCursorCol)
			log.Printf("DEBUG: Node %d: Repositioning cursor for input bytes (hex): %x", nodeNumber, []byte(posCmd)) // Use nodeNumber
			wErr := terminalio.WriteProcessedBytes(terminal, []byte(posCmd), outputMode)
			if wErr != nil {
				log.Printf("WARN: Failed positioning cursor for input: %v", wErr)
			}

			r, _, err := bufioReader.ReadRune()
			if err != nil {
				// Clear line on error using WriteProcessedBytes
				clearCmd := fmt.Sprintf("\x1b[%d;1H\x1b[2K", promptRow)
				log.Printf("DEBUG: Node %d: Writing prompt clear on read error bytes (hex): %x", nodeNumber, []byte(clearCmd)) // Use nodeNumber
				wErr := terminalio.WriteProcessedBytes(terminal, []byte(clearCmd), outputMode)
				if wErr != nil {
					log.Printf("WARN: Failed clearing line on read error: %v", wErr)
				}

				if errors.Is(err, io.EOF) {
					return false, io.EOF
				}
				return false, fmt.Errorf("failed reading yes/no input: %w", err)
			}

			newSelectedIndex := selectedIndex
			selectionMade := false
			result := false

			switch unicode.ToUpper(r) {
			case 'Y':
				selectionMade = true
				result = true
			case 'N':
				selectionMade = true
				result = false
			case ' ', '\r', '\n':
				selectionMade = true
				result = (selectedIndex == 1)
			case 27:
				escSeq := make([]byte, 2)
				n, readErr := bufioReader.Read(escSeq)
				if readErr != nil || n != 2 {
					log.Printf("DEBUG: Read %d bytes after ESC, err: %v. Ignoring ESC.", n, readErr)
					continue
				}
				log.Printf("DEBUG: ESC sequence read: [%x %x]", escSeq[0], escSeq[1])
				if escSeq[0] == 91 {
					switch escSeq[1] {
					case 67: // Right arrow
						newSelectedIndex = 1 - selectedIndex
					case 68: // Left arrow
						newSelectedIndex = 1 - selectedIndex
					}
				}
			default:
				// Ignore other chars
			}

			if selectionMade {
				// Clear line on selection using WriteProcessedBytes
				clearCmdBytes := []byte(fmt.Sprintf("\x1b[%d;1H\x1b[2K", promptRow))
				log.Printf("DEBUG: Node %d: Writing prompt final clear bytes (hex): %x", nodeNumber, clearCmdBytes) // Use nodeNumber
				wErr := terminalio.WriteProcessedBytes(terminal, clearCmdBytes, outputMode)
				if wErr != nil {
					log.Printf("WARN: Failed clearing line on selection: %v", wErr)
				}
				return result, nil
			}

			if newSelectedIndex != selectedIndex {
				selectedIndex = newSelectedIndex
				drawOptions(selectedIndex)
			}
		}
		// Lightbar logic ends here

	} else {
		// --- Text Input Fallback (if terminal height is unknown) ---
		log.Printf("DEBUG: Terminal height unknown, using text fallback for Yes/No prompt.")

		// Construct the simple text prompt
		fullPrompt := promptText + " [Y/N]? "

		// Write the prompt after one blank row: newline + blank line, then prompt.
		wErr := terminalio.WriteProcessedBytes(terminal, []byte("\r\n\r\n"), outputMode)
		if wErr != nil {
			log.Printf("WARN: Failed writing fallback pre-prompt spacing: %v", wErr)
		}

		processedPromptBytes := ansi.ReplacePipeCodes([]byte(fullPrompt))
		err := terminalio.WriteStringCP437(terminal, processedPromptBytes, outputMode)
		if err != nil {
			log.Printf("ERROR: Node %d: Failed writing Yes/No prompt text (fallback mode): %v", nodeNumber, err) // Use nodeNumber
			return false, fmt.Errorf("failed writing fallback prompt text: %w", err)
		}

		// Read user input
		input, err := terminal.ReadLine()
		if err != nil {
			// Clean up line on error using WriteProcessedBytes
			wErr := terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode) // Assuming CRLF is enough cleanup here
			if wErr != nil {
				log.Printf("WARN: Failed writing CRLF on read error: %v", wErr)
			}

			if errors.Is(err, io.EOF) {
				return false, io.EOF // Signal disconnect
			}
			return false, fmt.Errorf("failed reading yes/no fallback input: %w", err)
		}

		// Process input
		trimmedInput := strings.ToUpper(strings.TrimSpace(input))
		if len(trimmedInput) > 0 && trimmedInput[0] == 'Y' {
			return true, nil
		}
		return false, nil // Default to No if not 'Y'
	}
}

// runListUsers displays a list of users, sorted alphabetically.
func runListUsers(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running LISTUSERS", nodeNumber)

	// 1. Load Templates (Corrected filenames)
	topTemplatePath := filepath.Join(e.MenuSetPath, "templates", "USERLIST.TOP")
	midTemplatePath := filepath.Join(e.MenuSetPath, "templates", "USERLIST.MID")
	botTemplatePath := filepath.Join(e.MenuSetPath, "templates", "USERLIST.BOT")

	topTemplateBytes, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplateBytes, errBot := os.ReadFile(botTemplatePath)

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load one or more USERLIST template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading User List screen templates.|07\r\n"
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading USERLIST templates")
	}

	// --- Process Pipe Codes in Templates FIRST ---
	processedTopTemplate := ansi.ReplacePipeCodes(topTemplateBytes)
	processedMidTemplate := string(ansi.ReplacePipeCodes(midTemplateBytes)) // Process MID template
	processedBotTemplate := ansi.ReplacePipeCodes(botTemplateBytes)
	// --- END Template Processing ---

	// 2. Get user list data from UserManager
	users := userManager.GetAllUsers() // Corrected method call
	pendingCount := 0
	for _, u := range users {
		if u != nil && !u.Validated && !u.DeletedUser {
			pendingCount++
		}
	}

	// 3. Build the output string using processed templates and processed data
	var outputBuffer bytes.Buffer
	outputBuffer.Write(processedTopTemplate) // Write processed top template
	outputBuffer.WriteString("\r\n")
	outputBuffer.WriteString(string(ansi.ReplacePipeCodes([]byte(fmt.Sprintf("|11Pending validation: |15%d|07\r\n\r\n", pendingCount)))))

	if len(users) == 0 {
		// Optional: Handle empty state. The template might handle this.
		log.Printf("DEBUG: Node %d: No users to display.", nodeNumber)
		// If templates don't handle empty, add a message here.
	} else {
		// Iterate through user records and format using processed USERLIST.MID
		for _, user := range users {
			// Skip deleted users in public listing
			if user.DeletedUser {
				continue
			}
			line := processedMidTemplate // Start with the pipe-code-processed mid template

			// Format data for substitution
			handle := strings.TrimSpace(string(ansi.ReplacePipeCodes([]byte(user.Handle))))
			level := strings.TrimSpace(strconv.Itoa(user.AccessLevel))
			groupLocation := strings.TrimSpace(string(ansi.ReplacePipeCodes([]byte(user.GroupLocation))))
			if groupLocation == "" {
				groupLocation = "-"
			}
			if !user.Validated {
				handle = handle + " [NV]"
			}

			handle = formatLastCallerATWidth(handle, 30, false)
			groupLocation = formatLastCallerATWidth(groupLocation, 24, false)
			level = formatLastCallerATWidth(level, 3, true)

			// Replace placeholders with *already processed* data
			// Match placeholders found in USERLIST.MID: |UH, |GL, |LV, |AC
			line = strings.ReplaceAll(line, "|UH", handle)        // Use |UH for Handle (Alias)
			line = strings.ReplaceAll(line, "|GL", groupLocation) // Use |GL for Group/Location (Replaces |UN)
			line = strings.ReplaceAll(line, "|LV", level)         // Use |LV for Level

			log.Printf("DEBUG: About to write line for user %s: %q", handle, line)
			outputBuffer.WriteString(line) // Add the fully substituted and processed line
			log.Printf("DEBUG: Wrote line. Buffer size now: %d", outputBuffer.Len())
		}
	}

	log.Printf("DEBUG: Finished user loop. Total buffer size before BOT: %d", outputBuffer.Len())
	outputBuffer.Write(processedBotTemplate) // Write processed bottom template
	log.Printf("DEBUG: Added BOT template. Final buffer size: %d", outputBuffer.Len())

	// 4. Clear screen and display the assembled content
	writeErr := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	if writeErr != nil {
		log.Printf("ERROR: Node %d: Failed clearing screen for USERLIST: %v", nodeNumber, writeErr)
		return nil, "", writeErr
	}

	// Use WriteProcessedBytes for the assembled template content
	processedContent := outputBuffer.Bytes() // Contains already-processed ANSI bytes
	wErr := terminalio.WriteProcessedBytes(terminal, processedContent, outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing USERLIST output: %v", nodeNumber, wErr)
		return nil, "", wErr
	}

	// 5. Wait for Enter using configured PauseString (logic remains the same)
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... " // Fallback
	}

	var pauseBytesToWrite []byte
	processedPausePrompt := ansi.ReplacePipeCodes([]byte(pausePrompt))
	if outputMode == ansi.OutputModeCP437 {
		var cp437Buf bytes.Buffer
		for _, r := range string(processedPausePrompt) {
			if r < 128 {
				cp437Buf.WriteByte(byte(r))
			} else if cp437Byte, ok := ansi.UnicodeToCP437[r]; ok {
				cp437Buf.WriteByte(cp437Byte)
			} else {
				cp437Buf.WriteByte('?')
			}
		}
		pauseBytesToWrite = cp437Buf.Bytes()
	} else {
		pauseBytesToWrite = processedPausePrompt
	}

	log.Printf("DEBUG: Node %d: Writing USERLIST pause prompt. Mode: %d, Bytes: %q", nodeNumber, outputMode, string(pauseBytesToWrite))
	// Log hex bytes before writing
	log.Printf("DEBUG: Node %d: Writing USERLIST pause bytes (hex): %x", nodeNumber, pauseBytesToWrite)
	wErr = terminalio.WriteProcessedBytes(terminal, pauseBytesToWrite, outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing USERLIST pause prompt: %v", nodeNumber, wErr)
	}

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during USERLIST pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading input during USERLIST pause: %v", nodeNumber, err)
			return nil, "", err
		}
		if r == '\r' || r == '\n' { // Check for CR or LF
			break
		}
	}

	return nil, "", nil // Success
}

func pendingValidationCount(userManager *user.UserMgr) int {
	if userManager == nil {
		return 0
	}
	users := userManager.GetAllUsers()
	count := 0
	for _, u := range users {
		if u == nil {
			continue
		}
		if !u.Validated {
			count++
		}
	}
	return count
}

func sortedUsersByID(users []*user.User) []*user.User {
	filtered := make([]*user.User, 0, len(users))
	for _, u := range users {
		if u != nil {
			filtered = append(filtered, u)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ID < filtered[j].ID
	})
	return filtered
}

func adminTruncate(input string, max int) string {
	runes := []rune(strings.TrimSpace(input))
	if len(runes) <= max {
		return string(runes)
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

func adminTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.Format("2006-01-02 15:04")
}

func adminDate(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.Format("2006-01-02")
}

func adminUserLightbarBrowser(s ssh.Session, terminal *term.Terminal, users []*user.User, title string, instruction string, outputMode ansi.OutputMode, selectOnEnter bool) (*user.User, bool, error) {
	if len(users) == 0 {
		return nil, false, nil
	}

	selectedIndex := 0
	topIndex := 0
	pageSize := 10
	reader := bufio.NewReader(s)

	render := func() error {
		if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode); err != nil {
			return err
		}

		var b strings.Builder
		b.WriteString("\r\n|15" + title + "|07\r\n")
		b.WriteString("|07" + instruction + "\r\n")
		b.WriteString("|08--------------------------------------------------------------------------------|07\r\n")

		endIndex := topIndex + pageSize
		if endIndex > len(users) {
			endIndex = len(users)
		}

		for idx := topIndex; idx < endIndex; idx++ {
			u := users[idx]
			status := "OK"
			if u.DeletedUser {
				status = "DEL"
			} else if !u.Validated {
				status = "NV"
			}
			prefix := "  "
			if idx == selectedIndex {
				prefix = "» "
			}
			line := fmt.Sprintf("%s%-22s  @%-16s ID:%-4d L:%-3d %-2s", prefix, adminTruncate(u.Handle, 22), adminTruncate(u.Username, 16), u.ID, u.AccessLevel, status)
			if idx == selectedIndex {
				b.WriteString("\x1b[7m" + line + "\x1b[0m\r\n")
			} else {
				b.WriteString("|07" + line + "|07\r\n")
			}
		}

		for idx := endIndex; idx < topIndex+pageSize; idx++ {
			b.WriteString("\r\n")
		}

		sel := users[selectedIndex]
		b.WriteString("|08--------------------------------------------------------------------------------|07\r\n")
		b.WriteString(fmt.Sprintf("|15Handle        :|07 %-24s |15Username      :|07 %s\r\n", adminTruncate(sel.Handle, 24), adminTruncate(sel.Username, 30)))
		b.WriteString(fmt.Sprintf("|15Real Name     :|07 %-21s |15Phone         :|07 %s\r\n", adminTruncate(sel.RealName, 21), adminTruncate(sel.PhoneNumber, 29)))
		b.WriteString(fmt.Sprintf("|15Group/Location:|07 %-16s |15Flags         :|07 %-8s\r\n", adminTruncate(sel.GroupLocation, 16), adminTruncate(sel.Flags, 8)))
		b.WriteString(fmt.Sprintf("|15Validated     :|07 %-5t |15Level         :|07 %-3d |15TimeLimit     :|07 %-4d\r\n", sel.Validated, sel.AccessLevel, sel.TimeLimit))
		b.WriteString(fmt.Sprintf("|15Created       :|07 %-16s |15Last Login    :|07 %s\r\n", adminTime(sel.CreatedAt), adminTime(sel.LastLogin)))
		b.WriteString(fmt.Sprintf("|15Calls         :|07 %-5d |15Uploads       :|07 %-5d |15FilePoints    :|07 %-6d\r\n", sel.TimesCalled, sel.NumUploads, sel.FilePoints))
		b.WriteString(fmt.Sprintf("|15Msg Area      :|07 %-16s |15File Area     :|07 %s\r\n", adminTruncate(sel.CurrentMessageAreaTag, 16), adminTruncate(sel.CurrentFileAreaTag, 24)))
		b.WriteString(fmt.Sprintf("|15Note          :|07 %s\r\n", adminTruncate(sel.PrivateNote, 70)))

		return terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(b.String())), outputMode)
	}

	moveUp := func() {
		if selectedIndex > 0 {
			selectedIndex--
			if selectedIndex < topIndex {
				topIndex = selectedIndex
			}
		}
	}
	moveDown := func() {
		if selectedIndex < len(users)-1 {
			selectedIndex++
			if selectedIndex >= topIndex+pageSize {
				topIndex = selectedIndex - pageSize + 1
			}
		}
	}

	for {
		if err := render(); err != nil {
			return nil, false, err
		}

		r, _, err := reader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, false, io.EOF
			}
			return nil, false, err
		}

		switch r {
		case 'k', 'K', 'w', 'W':
			moveUp()
		case 'j', 'J', 's', 'S':
			moveDown()
		case 'q', 'Q':
			return nil, false, nil
		case '\r', '\n':
			if selectOnEnter {
				return users[selectedIndex], true, nil
			}
		case 27:
			time.Sleep(20 * time.Millisecond)
			seq := make([]byte, 0, 8)
			for reader.Buffered() > 0 && len(seq) < 8 {
				b, readErr := reader.ReadByte()
				if readErr != nil {
					break
				}
				seq = append(seq, b)
			}
			if len(seq) >= 2 && seq[0] == 91 {
				switch seq[1] {
				case 65:
					moveUp()
				case 66:
					moveDown()
				}
			} else {
				return nil, false, nil
			}
		}
	}
}

func runAdminListUsers(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil || userManager == nil {
		return nil, "", nil
	}
	sysOpACS := fmt.Sprintf("S%d", e.ServerCfg.SysOpLevel)
	if !checkACS(sysOpACS, currentUser, s, terminal, sessionStartTime) {
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01Access denied.|07\r\n")), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	users := sortedUsersByID(userManager.GetAllUsers())
	if len(users) == 0 {
		_ = terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|10No users found.|07")), outputMode)
		if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
			if errors.Is(pauseErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", pauseErr
		}
		return nil, "", nil
	}

	reader := bufio.NewReader(s)
	selectedIndex := 0
	topIndex := 0
	termHeight := 24
	termWidth := 80
	if ptyReq, _, ok := s.Pty(); ok && ptyReq.Window.Height > 0 {
		termHeight = ptyReq.Window.Height
		if ptyReq.Window.Width > 0 {
			termWidth = ptyReq.Window.Width
		}
	}
	pageSize := termHeight - 13
	if pageSize < 3 {
		pageSize = 3
	}
	if pageSize > 12 {
		pageSize = 12
	}

	titleRow := 1
	sepTopRow := 2
	listStartRow := 3
	sepMidRow := listStartRow + pageSize
	detailStartRow := sepMidRow + 1
	statusRow := termHeight - 1
	actionRow := termHeight

	writeAt := func(row, col int, text string) error {
		cmd := fmt.Sprintf("\x1b[%d;%dH\x1b[2K%s", row, col, text)
		return terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(cmd)), outputMode)
	}

	clearRow := func(row int) error {
		cmd := fmt.Sprintf("\x1b[%d;1H\x1b[2K", row)
		return terminalio.WriteProcessedBytes(terminal, []byte(cmd), outputMode)
	}

	renderHeader := func() error {
		if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode); err != nil {
			return err
		}
		if err := writeAt(titleRow, 1, "|15Admin User Browser|07"); err != nil {
			return err
		}
		if err := writeAt(sepTopRow, 1, "|08--------------------------------------------------------------------------------|07"); err != nil {
			return err
		}
		if err := writeAt(sepMidRow, 1, "|08--------------------------------------------------------------------------------|07"); err != nil {
			return err
		}
		for r := detailStartRow; r <= statusRow; r++ {
			if err := clearRow(r); err != nil {
				return err
			}
		}
		return nil
	}

	pendingChanges := make(map[string]interface{})

	renderActionBar := func() error {
		var barText string
		if len(pendingChanges) > 0 {
			barText = "[S] Save Changes  [X] Abort  [Q] Quit"
		} else {
			sel := users[selectedIndex]
			// Dynamic labels based on user state
			banLabel := "Ban"
			if sel.AccessLevel == 0 && !sel.Validated {
				banLabel = "Un-Ban"
			}
			deleteLabel := "Delete"
			if sel.DeletedUser {
				deleteLabel = "Un-Delete"
			}
			barText = fmt.Sprintf("[0] %s  [9] %s  [Q] Quit", banLabel, deleteLabel)
		}
		maxWidth := termWidth - 1 // never touch last column
		if maxWidth < 10 {
			maxWidth = 10
		}
		if len(barText) > maxWidth {
			barText = barText[:maxWidth]
		}
		startCol := (termWidth-len(barText))/2 + 1
		if startCol < 1 {
			startCol = 1
		}
		if startCol+len(barText)-1 >= termWidth {
			trimLen := termWidth - startCol
			if trimLen > 0 && trimLen < len(barText) {
				barText = barText[:trimLen]
			}
		}
		if err := clearRow(actionRow); err != nil {
			return err
		}
		cmd := fmt.Sprintf("\x1b[%d;%dH\x1b[7m%s\x1b[0m", actionRow, startCol, barText)
		return terminalio.WriteProcessedBytes(terminal, []byte(cmd), outputMode)
	}

	renderList := func() error {
		endIndex := topIndex + pageSize
		if endIndex > len(users) {
			endIndex = len(users)
		}
		row := listStartRow
		for idx := topIndex; idx < endIndex; idx++ {
			u := users[idx]
			status := "OK"
			if u.DeletedUser {
				status = "DEL"
			} else if !u.Validated {
				status = "NV"
			}
			prefix := "  "
			if idx == selectedIndex {
				prefix = "» "
			}
			line := fmt.Sprintf("%s%-22s   %-10s   ID:%-4d   L:%-3d  %-2s", prefix, adminTruncate(u.Handle, 22), adminDate(u.CreatedAt), u.ID, u.AccessLevel, status)
			if idx == selectedIndex {
				line = "\x1b[7m" + line + "\x1b[0m"
			}
			if err := writeAt(row, 1, line); err != nil {
				return err
			}
			row++
		}
		for ; row < listStartRow+pageSize; row++ {
			if err := clearRow(row); err != nil {
				return err
			}
		}
		return nil
	}

	renderDetails := func(message string) error {
		sel := users[selectedIndex]

		getFieldValue := func(fieldName string, originalValue string) string {
			if val, ok := pendingChanges[fieldName]; ok {
				return fmt.Sprintf("*%s", adminTruncate(val.(string), 23))
			}
			return adminTruncate(originalValue, 24)
		}

		getIntFieldValue := func(fieldName string, originalValue int) string {
			if val, ok := pendingChanges[fieldName]; ok {
				return fmt.Sprintf("*%d", val.(int))
			}
			return fmt.Sprintf("%d", originalValue)
		}

		getBoolFieldValue := func(fieldName string, originalValue bool) string {
			if val, ok := pendingChanges[fieldName]; ok {
				return fmt.Sprintf("*%t", val.(bool))
			}
			return fmt.Sprintf("%t", originalValue)
		}

		lineTwoCol := func(leftLabel, leftValue, rightLabel, rightValue string) string {
			return fmt.Sprintf("|15%-14s:|07 %-24s |15%-14s:|07 %-24s", leftLabel, leftValue, rightLabel, rightValue)
		}

		deletedStatus := "No"
		deletedAtStr := ""
		if sel.DeletedUser {
			deletedStatus = "Yes"
			if sel.DeletedAt != nil {
				deletedAtStr = adminTime(*sel.DeletedAt)
			}
		}

		lines := []string{
			lineTwoCol("[A] User Name", getFieldValue("username", sel.Username), "[B] Real Name", getFieldValue("realname", sel.RealName)),
			lineTwoCol("[C] Phone", getFieldValue("phone", sel.PhoneNumber), "[D] Group/Loc", getFieldValue("grouploc", sel.GroupLocation)),
			lineTwoCol("[E] Note", getFieldValue("note", sel.PrivateNote), "[F] Flags", getFieldValue("flags", sel.Flags)),
			lineTwoCol("[G] Level", getIntFieldValue("level", sel.AccessLevel), "[I] Validated", getBoolFieldValue("validated", sel.Validated)),
			lineTwoCol("Calls", fmt.Sprintf("%d", sel.TimesCalled), "Uploads", fmt.Sprintf("%d", sel.NumUploads)),
			lineTwoCol("FilePoints", fmt.Sprintf("%d", sel.FilePoints), "Posts", fmt.Sprintf("%d", sel.MessagesPosted)),
			lineTwoCol("Created", adminTime(sel.CreatedAt), "Last Login", adminTime(sel.LastLogin)),
			lineTwoCol("Deleted", deletedStatus, "Deleted At", deletedAtStr),
		}
		for i, line := range lines {
			if err := writeAt(detailStartRow+i, 1, line); err != nil {
				return err
			}
		}
		if message != "" {
			if err := writeAt(statusRow, 1, message); err != nil {
				return err
			}
		} else {
			if err := clearRow(statusRow); err != nil {
				return err
			}
		}
		return renderActionBar()
	}

	readFieldInput := func(fieldLabel string, currentValue string, maxLen int) (string, error) {
		prompt := fmt.Sprintf("|15%s:|07 ", fieldLabel)
		if err := writeAt(statusRow, 1, prompt); err != nil {
			return "", err
		}

		// Position cursor after prompt
		cursorPos := len(fieldLabel) + 3
		cmd := fmt.Sprintf("\x1b[%d;%dH", statusRow, cursorPos)
		if err := terminalio.WriteProcessedBytes(terminal, []byte(cmd), outputMode); err != nil {
			return "", err
		}

		// Show current value
		if err := terminalio.WriteProcessedBytes(terminal, []byte(currentValue), outputMode); err != nil {
			return "", err
		}

		input := []rune(currentValue)
		cursorIdx := len(input)

		for {
			r, _, readErr := reader.ReadRune()
			if readErr != nil {
				return "", readErr
			}

			switch r {
			case '\r', '\n':
				return string(input), nil
			case 27: // ESC
				return "", fmt.Errorf("cancelled")
			case 127, 8: // Backspace
				if cursorIdx > 0 {
					input = append(input[:cursorIdx-1], input[cursorIdx:]...)
					cursorIdx--
					if err := writeAt(statusRow, 1, prompt+string(input)+"  "); err != nil {
						return "", err
					}
					cmd := fmt.Sprintf("\x1b[%d;%dH", statusRow, cursorPos+cursorIdx)
					if err := terminalio.WriteProcessedBytes(terminal, []byte(cmd), outputMode); err != nil {
						return "", err
					}
				}
			default:
				if r >= 32 && r < 127 && len(input) < maxLen {
					input = append(input[:cursorIdx], append([]rune{r}, input[cursorIdx:]...)...)
					cursorIdx++
					if err := writeAt(statusRow, 1, prompt+string(input)); err != nil {
						return "", err
					}
					cmd := fmt.Sprintf("\x1b[%d;%dH", statusRow, cursorPos+cursorIdx)
					if err := terminalio.WriteProcessedBytes(terminal, []byte(cmd), outputMode); err != nil {
						return "", err
					}
				}
			}
		}
	}

	moveUp := func() {
		if selectedIndex > 0 {
			selectedIndex--
			if selectedIndex < topIndex {
				topIndex = selectedIndex
			}
		}
	}
	moveDown := func() {
		if selectedIndex < len(users)-1 {
			selectedIndex++
			if selectedIndex >= topIndex+pageSize {
				topIndex = selectedIndex - pageSize + 1
			}
		}
	}

	if err := renderHeader(); err != nil {
		return nil, "", err
	}
	if err := renderList(); err != nil {
		return nil, "", err
	}
	if err := renderActionBar(); err != nil {
		return nil, "", err
	}
	if err := renderDetails(""); err != nil {
		return nil, "", err
	}

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", err
		}

		refresh := false
		statusMessage := ""

		switch r {
		case 'k', 'K', 'w', 'W':
			if len(pendingChanges) == 0 {
				moveUp()
				refresh = true
			}
		case 'j', 'J':
			if len(pendingChanges) == 0 {
				moveDown()
				refresh = true
			}
		case 's', 'S':
			if len(pendingChanges) > 0 {
				// Save changes
				target := users[selectedIndex]

				// Protect User ID 1 from critical changes
				if target.ID == 1 {
					// Check if trying to change level below sysop
					if val, ok := pendingChanges["level"]; ok {
						if val.(int) < e.ServerCfg.SysOpLevel {
							statusMessage = "|01Cannot lower User #1 below SysOp level!|07"
							delete(pendingChanges, "level")
							refresh = true
							continue
						}
					}
					// Check if trying to unvalidate
					if val, ok := pendingChanges["validated"]; ok {
						if !val.(bool) {
							statusMessage = "|01Cannot unvalidate User #1!|07"
							delete(pendingChanges, "validated")
							refresh = true
							continue
						}
					}
					// Check if trying to delete
					if val, ok := pendingChanges["deleted"]; ok {
						if val.(bool) {
							statusMessage = "|01Cannot delete User #1!|07"
							delete(pendingChanges, "deleted")
							refresh = true
							continue
						}
					}
				}
				if val, ok := pendingChanges["username"]; ok {
					target.Username = val.(string)
				}
				if val, ok := pendingChanges["realname"]; ok {
					target.RealName = val.(string)
				}
				if val, ok := pendingChanges["phone"]; ok {
					target.PhoneNumber = val.(string)
				}
				if val, ok := pendingChanges["grouploc"]; ok {
					target.GroupLocation = val.(string)
				}
				if val, ok := pendingChanges["note"]; ok {
					target.PrivateNote = val.(string)
				}
				if val, ok := pendingChanges["flags"]; ok {
					target.Flags = val.(string)
				}
				if val, ok := pendingChanges["level"]; ok {
					target.AccessLevel = val.(int)
				}
				if val, ok := pendingChanges["validated"]; ok {
					target.Validated = val.(bool)
					// When validating, ensure level is set to regular user level if currently 0
					if target.Validated && target.AccessLevel == 0 {
						target.AccessLevel = e.ServerCfg.RegularUserLevel
						if target.AccessLevel <= 0 {
							target.AccessLevel = 10
						}
					}
				}
				if val, ok := pendingChanges["deleted"]; ok {
					target.DeletedUser = val.(bool)
					if target.DeletedUser {
						now := time.Now()
						target.DeletedAt = &now
					} else {
						// Clear the deletion timestamp when undeleting
						target.DeletedAt = nil
					}
				}

				if updateErr := userManager.UpdateUser(target); updateErr != nil {
					statusMessage = fmt.Sprintf("|01Save failed: %v|07", updateErr)
				} else {
					statusMessage = fmt.Sprintf("|10Changes saved for %s.|07", target.Username)
					pendingChanges = make(map[string]interface{})
					users = sortedUsersByID(userManager.GetAllUsers())
				}
				refresh = true
			} else {
				moveDown()
				refresh = true
			}
		case 'q', 'Q':
			if len(pendingChanges) > 0 {
				statusMessage = "|11Unsaved changes! Press [S] to save or [X] to abort.|07"
			} else {
				return nil, "", nil
			}
		case 'x', 'X':
			if len(pendingChanges) > 0 {
				pendingChanges = make(map[string]interface{})
				statusMessage = "|11Changes discarded.|07"
				refresh = true
			}
		case 'a', 'A':
			sel := users[selectedIndex]
			if newVal, editErr := readFieldInput("User Name", sel.Username, 30); editErr == nil {
				if newVal != sel.Username {
					pendingChanges["username"] = newVal
					statusMessage = "|10Field marked for update.|07"
				} else {
					delete(pendingChanges, "username")
					statusMessage = "|08No change.|07"
				}
				refresh = true
			} else {
				if editErr.Error() != "cancelled" {
					statusMessage = fmt.Sprintf("|01Error: %v|07", editErr)
				}
				refresh = true
			}
		case 'b', 'B':
			// Edit Real Name field
			sel := users[selectedIndex]
			if newVal, editErr := readFieldInput("Real Name", sel.RealName, 50); editErr == nil {
				if newVal != sel.RealName {
					pendingChanges["realname"] = newVal
					statusMessage = "|10Field marked for update.|07"
				} else {
					delete(pendingChanges, "realname")
					statusMessage = "|08No change.|07"
				}
				refresh = true
			} else {
				if editErr.Error() != "cancelled" {
					statusMessage = fmt.Sprintf("|01Error: %v|07", editErr)
				}
				refresh = true
			}
		case 'c', 'C':
			sel := users[selectedIndex]
			if newVal, editErr := readFieldInput("Phone", sel.PhoneNumber, 20); editErr == nil {
				if newVal != sel.PhoneNumber {
					pendingChanges["phone"] = newVal
					statusMessage = "|10Field marked for update.|07"
				} else {
					delete(pendingChanges, "phone")
					statusMessage = "|08No change.|07"
				}
				refresh = true
			} else {
				if editErr.Error() != "cancelled" {
					statusMessage = fmt.Sprintf("|01Error: %v|07", editErr)
				}
				refresh = true
			}
		case 'd', 'D':
			sel := users[selectedIndex]
			if newVal, editErr := readFieldInput("Group/Location", sel.GroupLocation, 30); editErr == nil {
				if newVal != sel.GroupLocation {
					pendingChanges["grouploc"] = newVal
					statusMessage = "|10Field marked for update.|07"
				} else {
					delete(pendingChanges, "grouploc")
					statusMessage = "|08No change.|07"
				}
				refresh = true
			} else {
				if editErr.Error() != "cancelled" {
					statusMessage = fmt.Sprintf("|01Error: %v|07", editErr)
				}
				refresh = true
			}
		case 'e', 'E':
			sel := users[selectedIndex]
			if newVal, editErr := readFieldInput("Note", sel.PrivateNote, 50); editErr == nil {
				if newVal != sel.PrivateNote {
					pendingChanges["note"] = newVal
					statusMessage = "|10Field marked for update.|07"
				} else {
					delete(pendingChanges, "note")
					statusMessage = "|08No change.|07"
				}
				refresh = true
			} else {
				if editErr.Error() != "cancelled" {
					statusMessage = fmt.Sprintf("|01Error: %v|07", editErr)
				}
				refresh = true
			}
		case 'f', 'F':
			sel := users[selectedIndex]
			if newVal, editErr := readFieldInput("Flags", sel.Flags, 20); editErr == nil {
				if newVal != sel.Flags {
					pendingChanges["flags"] = newVal
					statusMessage = "|10Field marked for update.|07"
				} else {
					delete(pendingChanges, "flags")
					statusMessage = "|08No change.|07"
				}
				refresh = true
			} else {
				if editErr.Error() != "cancelled" {
					statusMessage = fmt.Sprintf("|01Error: %v|07", editErr)
				}
				refresh = true
			}
		case 'g', 'G':
			sel := users[selectedIndex]
			levelStr := fmt.Sprintf("%d", sel.AccessLevel)
			if newVal, editErr := readFieldInput("Level", levelStr, 3); editErr == nil {
				if level, parseErr := strconv.Atoi(newVal); parseErr == nil {
					// Protect User #1 from level reduction
					if sel.ID == 1 && level < e.ServerCfg.SysOpLevel {
						statusMessage = "|01Cannot lower User #1 below SysOp level!|07"
						refresh = true
					} else if level != sel.AccessLevel {
						pendingChanges["level"] = level
						statusMessage = "|10Field marked for update.|07"
						refresh = true
					} else {
						delete(pendingChanges, "level")
						statusMessage = "|08No change.|07"
						refresh = true
					}
				} else {
					statusMessage = "|01Invalid number.|07"
					refresh = true
				}
			} else {
				if editErr.Error() != "cancelled" {
					statusMessage = fmt.Sprintf("|01Error: %v|07", editErr)
				}
				refresh = true
			}
		case 'i', 'I':
			// Toggle validated status
			sel := users[selectedIndex]
			if sel.ID == 1 && sel.Validated {
				// Don't allow unvalidating User #1
				statusMessage = "|01Cannot unvalidate User #1!|07"
				refresh = true
			} else {
				newValidated := !sel.Validated
				if newValidated != sel.Validated {
					pendingChanges["validated"] = newValidated
					if newValidated {
						statusMessage = "|10Validated status marked for update.|07"
					} else {
						statusMessage = "|11Unvalidated status marked for update.|07"
					}
				} else {
					delete(pendingChanges, "validated")
					statusMessage = "|08No change.|07"
				}
				refresh = true
			}
		case '0':
			// Toggle ban user (sets level 0, unvalidated) or unban (restore to regular level)
			sel := users[selectedIndex]
			if sel.ID == 1 {
				statusMessage = "|01Cannot ban User #1!|07"
			} else {
				// Check if user is currently banned
				isBanned := sel.AccessLevel == 0 && !sel.Validated
				if isBanned {
					// Unban: restore to regular user level and validate
					pendingChanges["validated"] = true
					pendingChanges["level"] = e.ServerCfg.RegularUserLevel
					statusMessage = fmt.Sprintf("|10Un-ban marked for update (level %d, validated).|07", e.ServerCfg.RegularUserLevel)
				} else {
					// Ban: set level 0 and unvalidated
					pendingChanges["validated"] = false
					pendingChanges["level"] = 0
					statusMessage = "|01Ban marked for update (level 0, unvalidated).|07"
				}
			}
			refresh = true
		case '9':
			// Toggle delete user (soft delete)
			sel := users[selectedIndex]
			if sel.ID == 1 {
				statusMessage = "|01Cannot delete User #1!|07"
			} else {
				newDeleted := !sel.DeletedUser
				if newDeleted != sel.DeletedUser {
					pendingChanges["deleted"] = newDeleted
					if newDeleted {
						statusMessage = "|01Delete marked for update (soft delete).|07"
					} else {
						statusMessage = "|10Undelete marked for update (restore user).|07"
					}
				} else {
					delete(pendingChanges, "deleted")
					statusMessage = "|08No change.|07"
				}
			}
			refresh = true
		case '\r', '\n':
			sel := users[selectedIndex]
			banAction := "ban"
			if sel.AccessLevel == 0 && !sel.Validated {
				banAction = "un-ban"
			}
			deleteAction := "delete"
			if sel.DeletedUser {
				deleteAction = "un-delete"
			}
			if len(pendingChanges) > 0 {
				statusMessage = fmt.Sprintf("|08Press [A-G,I] to edit, [0] %s, [9] %s, [S] save, [X] abort.|07", banAction, deleteAction)
			} else {
				statusMessage = fmt.Sprintf("|08Press [I] toggle validated, [0] %s, [9] %s, [Q] quit.|07", banAction, deleteAction)
			}
		case 27:
			time.Sleep(20 * time.Millisecond)
			seq := make([]byte, 0, 8)
			for reader.Buffered() > 0 && len(seq) < 8 {
				b, readErr := reader.ReadByte()
				if readErr != nil {
					break
				}
				seq = append(seq, b)
			}
			if len(seq) >= 2 && seq[0] == 91 {
				switch seq[1] {
				case 65:
					moveUp()
					refresh = true
				case 66:
					moveDown()
					refresh = true
				}
			} else {
				return nil, "", nil
			}
		}

		if refresh {
			if err := renderList(); err != nil {
				return nil, "", err
			}
			if err := renderDetails(statusMessage); err != nil {
				return nil, "", err
			}
		} else if statusMessage != "" {
			if err := renderDetails(statusMessage); err != nil {
				return nil, "", err
			}
		}
	}
}

// runPendingValidationNotice notifies SysOps when users are awaiting validation.
func runPendingValidationNotice(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil || userManager == nil {
		return nil, "", nil
	}

	sysOpACS := fmt.Sprintf("S%d", e.ServerCfg.SysOpLevel)
	if !checkACS(sysOpACS, currentUser, s, terminal, sessionStartTime) {
		return nil, "", nil
	}

	pendingCount := pendingValidationCount(userManager)

	if pendingCount == 0 {
		return nil, "", nil
	}

	notice := fmt.Sprintf("\r\n|11Admin: |15[V]|11 Validate user account [|15%d|11]. Press |15X|11 for Admin menu.|07\r\n", pendingCount)
	if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(notice)), outputMode); err != nil {
		return nil, "", err
	}

	return nil, "", nil
}

// runValidateUser shows a lightbar-style pending user list with details and validates on Enter.
func runValidateUser(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running VALIDATEUSER", nodeNumber)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in to validate users.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	if userManager == nil {
		msg := "\r\n|01Error: User manager is not available.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	sysOpACS := fmt.Sprintf("S%d", e.ServerCfg.SysOpLevel)
	if !checkACS(sysOpACS, currentUser, s, terminal, sessionStartTime) {
		msg := "\r\n|01Access denied.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	regularUserLevel := e.ServerCfg.RegularUserLevel
	if regularUserLevel <= 0 {
		regularUserLevel = 10
	}

	allUsers := userManager.GetAllUsers()
	pendingUsers := make([]*user.User, 0)
	for _, u := range allUsers {
		if u == nil {
			continue
		}
		if !u.Validated {
			pendingUsers = append(pendingUsers, u)
		}
	}

	if len(pendingUsers) == 0 {
		_ = terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
		msg := "\r\n|10No users are pending validation.|07"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
			if errors.Is(pauseErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", pauseErr
		}
		return nil, "", nil
	}

	sort.Slice(pendingUsers, func(i, j int) bool {
		return pendingUsers[i].ID < pendingUsers[j].ID
	})

	selectedIndex := 0
	topIndex := 0
	pageSize := 10
	reader := bufio.NewReader(s)

	truncate := func(input string, max int) string {
		runes := []rune(strings.TrimSpace(input))
		if len(runes) <= max {
			return string(runes)
		}
		if max <= 1 {
			return string(runes[:max])
		}
		return string(runes[:max-1]) + "…"
	}

	render := func() error {
		if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode); err != nil {
			return err
		}

		var b strings.Builder
		b.WriteString("\r\n|15User Validation|07\r\n")
		b.WriteString(fmt.Sprintf("|07Enter validates selected user to level |15%d|07. |08[Up/Down]|07 move, |08Q|07 quit.\r\n", regularUserLevel))
		b.WriteString("|08-------------------------------------------------------------------------------|07\r\n")

		endIndex := topIndex + pageSize
		if endIndex > len(pendingUsers) {
			endIndex = len(pendingUsers)
		}

		for idx := topIndex; idx < endIndex; idx++ {
			u := pendingUsers[idx]
			prefix := "  "
			if idx == selectedIndex {
				prefix = "» "
			}
			line := fmt.Sprintf("%s%-22s  @%-18s ID:%-4d L:%-3d", prefix, truncate(u.Handle, 22), truncate(u.Username, 18), u.ID, u.AccessLevel)
			if idx == selectedIndex {
				b.WriteString("\x1b[7m")
				b.WriteString(line)
				b.WriteString("\x1b[0m\r\n")
			} else {
				b.WriteString("|07")
				b.WriteString(line)
				b.WriteString("|07\r\n")
			}
		}

		for idx := endIndex; idx < topIndex+pageSize; idx++ {
			b.WriteString("\r\n")
		}

		sel := pendingUsers[selectedIndex]
		createdAt := "N/A"
		if !sel.CreatedAt.IsZero() {
			createdAt = sel.CreatedAt.Format("2006-01-02 15:04")
		}

		b.WriteString("|08-------------------------------------------------------------------------------|07\r\n")
		b.WriteString(fmt.Sprintf("|15Handle:|07 %-24s |15Username:|07 %s\r\n", truncate(sel.Handle, 24), truncate(sel.Username, 28)))
		b.WriteString(fmt.Sprintf("|15Real Name:|07 %-21s |15Phone:|07 %s\r\n", truncate(sel.RealName, 21), truncate(sel.PhoneNumber, 29)))
		b.WriteString(fmt.Sprintf("|15Group/Location:|07 %-16s |15Created:|07 %s\r\n", truncate(sel.GroupLocation, 16), createdAt))
		b.WriteString(fmt.Sprintf("|15Current Level:|07 %-4d |15Validated:|07 %t\r\n", sel.AccessLevel, sel.Validated))

		return terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(b.String())), outputMode)
	}

	for {
		if err := render(); err != nil {
			return nil, "", err
		}

		r, _, err := reader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", err
		}

		moveUp := func() {
			if selectedIndex > 0 {
				selectedIndex--
				if selectedIndex < topIndex {
					topIndex = selectedIndex
				}
			}
		}
		moveDown := func() {
			if selectedIndex < len(pendingUsers)-1 {
				selectedIndex++
				if selectedIndex >= topIndex+pageSize {
					topIndex = selectedIndex - pageSize + 1
				}
			}
		}

		switch r {
		case 'k', 'K', 'w', 'W':
			moveUp()
		case 'j', 'J', 's', 'S':
			moveDown()
		case 'q', 'Q':
			return nil, "", nil
		case '\r', '\n':
			targetUser := pendingUsers[selectedIndex]
			confirmPrompt := fmt.Sprintf("\r\n\r\n|07Validate |15%s|07 and set access level to |15%d|07 (Regular User)? @", targetUser.Handle, regularUserLevel)
			confirm, confirmErr := e.promptYesNo(s, terminal, confirmPrompt, outputMode, nodeNumber)
			if confirmErr != nil {
				if errors.Is(confirmErr, io.EOF) {
					return nil, "LOGOFF", io.EOF
				}
				return nil, "", confirmErr
			}

			if !confirm {
				continue
			}

			targetUser.Validated = true
			targetUser.AccessLevel = regularUserLevel

			if updateErr := userManager.UpdateUser(targetUser); updateErr != nil {
				msg := fmt.Sprintf("\r\n\r\n|01Failed to update user: %v|07", updateErr)
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
					if errors.Is(pauseErr, io.EOF) {
						return nil, "LOGOFF", io.EOF
					}
					return nil, "", pauseErr
				}
				return nil, "", updateErr
			}

			success := fmt.Sprintf("\r\n\r\n|10User validated: |15%s|10 (Regular User level |15%d|10).|07", targetUser.Handle, targetUser.AccessLevel)
			_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(success)), outputMode)
			if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
				if errors.Is(pauseErr, io.EOF) {
					return nil, "LOGOFF", io.EOF
				}
				return nil, "", pauseErr
			}
			return nil, "", nil
		case 27:
			time.Sleep(20 * time.Millisecond)
			seq := make([]byte, 0, 8)
			for reader.Buffered() > 0 && len(seq) < 8 {
				b, readErr := reader.ReadByte()
				if readErr != nil {
					break
				}
				seq = append(seq, b)
			}
			if len(seq) >= 2 && seq[0] == 91 { // '['
				switch seq[1] {
				case 65: // Up
					moveUp()
				case 66: // Down
					moveDown()
				}
			} else {
				return nil, "", nil
			}
		}
	}
}

// runUnvalidateUser removes validation status from a user account.
func runUnvalidateUser(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running UNVALIDATEUSER", nodeNumber)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in to modify users.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	if userManager == nil {
		msg := "\r\n|01Error: User manager is not available.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	sysOpACS := fmt.Sprintf("S%d", e.ServerCfg.SysOpLevel)
	if !checkACS(sysOpACS, currentUser, s, terminal, sessionStartTime) {
		msg := "\r\n|01Access denied.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	users := sortedUsersByID(userManager.GetAllUsers())
	if len(users) == 0 {
		_ = terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|10No users found.|07")), outputMode)
		if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
			if errors.Is(pauseErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", pauseErr
		}
		return nil, "", nil
	}

	targetUser, selected, pickErr := adminUserLightbarBrowser(s, terminal, users, "Unvalidate User", "Select a user. [Enter] unvalidate, [Q] quit.", outputMode, true)
	if pickErr != nil {
		if errors.Is(pickErr, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", pickErr
	}
	if !selected || targetUser == nil {
		return nil, "", nil
	}

	confirmPrompt := fmt.Sprintf("\r\n\r\n|07Set |15%s|07 to unvalidated? @", targetUser.Handle)
	confirm, confirmErr := e.promptYesNo(s, terminal, confirmPrompt, outputMode, nodeNumber)
	if confirmErr != nil {
		if errors.Is(confirmErr, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", confirmErr
	}

	if !confirm {
		msg := "\r\n|07Cancelled.|07"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
			if errors.Is(pauseErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", pauseErr
		}
		return nil, "", nil
	}

	targetUser.Validated = false

	if updateErr := userManager.UpdateUser(targetUser); updateErr != nil {
		msg := fmt.Sprintf("\r\n\r\n|01Failed to update user: %v|07", updateErr)
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
			if errors.Is(pauseErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", pauseErr
		}
		return nil, "", updateErr
	}

	success := fmt.Sprintf("\r\n\r\n|10User set to unvalidated: |15%s|10.|07", targetUser.Handle)
	_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(success)), outputMode)
	if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
		if errors.Is(pauseErr, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", pauseErr
	}

	return nil, "", nil
}

// runBanUser quickly bans a user by setting access level to 0 and validation to false.
func runBanUser(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running BANUSER", nodeNumber)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in to modify users.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	if userManager == nil {
		msg := "\r\n|01Error: User manager is not available.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	sysOpACS := fmt.Sprintf("S%d", e.ServerCfg.SysOpLevel)
	if !checkACS(sysOpACS, currentUser, s, terminal, sessionStartTime) {
		msg := "\r\n|01Access denied.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	users := sortedUsersByID(userManager.GetAllUsers())
	if len(users) == 0 {
		_ = terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|10No users found.|07")), outputMode)
		if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
			if errors.Is(pauseErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", pauseErr
		}
		return nil, "", nil
	}

	targetUser, selected, pickErr := adminUserLightbarBrowser(s, terminal, users, "Ban User", "Select a user. [Enter] ban, [Q] quit.", outputMode, true)
	if pickErr != nil {
		if errors.Is(pickErr, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", pickErr
	}
	if !selected || targetUser == nil {
		return nil, "", nil
	}

	// Protect User #1
	if targetUser.ID == 1 {
		msg := "\r\n\r\n|01Cannot ban User #1!|07"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
			if errors.Is(pauseErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", pauseErr
		}
		return nil, "", nil
	}

	confirmPrompt := fmt.Sprintf("\r\n\r\n|07Ban |15%s|07 (set level 0 + unvalidated)? @", targetUser.Handle)
	confirm, confirmErr := e.promptYesNo(s, terminal, confirmPrompt, outputMode, nodeNumber)
	if confirmErr != nil {
		if errors.Is(confirmErr, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", confirmErr
	}

	if !confirm {
		msg := "\r\n|07Cancelled.|07"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
			if errors.Is(pauseErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", pauseErr
		}
		return nil, "", nil
	}

	targetUser.Validated = false
	targetUser.AccessLevel = 0

	if updateErr := userManager.UpdateUser(targetUser); updateErr != nil {
		msg := fmt.Sprintf("\r\n\r\n|01Failed to update user: %v|07", updateErr)
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
			if errors.Is(pauseErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", pauseErr
		}
		return nil, "", updateErr
	}

	success := fmt.Sprintf("\r\n\r\n|10User banned: |15%s|10 (level 0, unvalidated).|07", targetUser.Handle)
	_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(success)), outputMode)
	if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
		if errors.Is(pauseErr, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", pauseErr
	}

	return nil, "", nil
}


// runDeleteUser soft-deletes a user by setting DeletedUser=true and recording the deletion timestamp.
func runDeleteUser(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running DELETEUSER", nodeNumber)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in to delete users.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	if userManager == nil {
		msg := "\r\n|01Error: User manager is not available.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	sysOpACS := fmt.Sprintf("S%d", e.ServerCfg.SysOpLevel)
	if !checkACS(sysOpACS, currentUser, s, terminal, sessionStartTime) {
		msg := "\r\n|01Access denied.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	users := sortedUsersByID(userManager.GetAllUsers())
	if len(users) == 0 {
		_ = terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|10No users found.|07")), outputMode)
		if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
			if errors.Is(pauseErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", pauseErr
		}
		return nil, "", nil
	}

	targetUser, selected, pickErr := adminUserLightbarBrowser(s, terminal, users, "Delete User", "Select a user. [Enter] delete, [Q] quit.", outputMode, true)
	if pickErr != nil {
		if errors.Is(pickErr, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", pickErr
	}
	if !selected || targetUser == nil {
		return nil, "", nil
	}

	// Protect User #1
	if targetUser.ID == 1 {
		msg := "\r\n\r\n|01Cannot delete User #1!|07"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
			if errors.Is(pauseErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", pauseErr
		}
		return nil, "", nil
	}

	confirmPrompt := fmt.Sprintf("\r\n\r\n|07Delete |15%s|07 (soft delete - data preserved)? @", targetUser.Handle)
	confirm, confirmErr := e.promptYesNo(s, terminal, confirmPrompt, outputMode, nodeNumber)
	if confirmErr != nil {
		if errors.Is(confirmErr, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", confirmErr
	}

	if !confirm {
		msg := "\r\n|07Cancelled.|07"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
			if errors.Is(pauseErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", pauseErr
		}
		return nil, "", nil
	}

	targetUser.DeletedUser = true
	now := time.Now()
	targetUser.DeletedAt = &now

	if updateErr := userManager.UpdateUser(targetUser); updateErr != nil {
		msg := fmt.Sprintf("\r\n\r\n|01Failed to update user: %v|07", updateErr)
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
			if errors.Is(pauseErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", pauseErr
		}
		return nil, "", updateErr
	}

	success := fmt.Sprintf("\r\n\r\n|10User deleted: |15%s|10 (soft delete - data preserved).|07", targetUser.Handle)
	_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(success)), outputMode)
	if pauseErr := e.loginPausePrompt(s, terminal, nodeNumber, outputMode); pauseErr != nil {
		if errors.Is(pauseErr, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", pauseErr
	}

	return nil, "", nil
}
// runShowVersion displays static version information.
func runShowVersion(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running SHOWVERSION", nodeNumber)

	// Define the version string (can be made dynamic later)
	versionString := "|15ViSiON/3 Go Edition - v0.1.0 (Pre-Alpha)|07"

	// Display the version
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode) // Optional: Clear screen
	terminalio.WriteProcessedBytes(terminal, []byte("\r\n\r\n"), outputMode)         // Add some spacing
	wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(versionString)), outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing SHOWVERSION output: %v", nodeNumber, wErr)
		// Don't return error, just log it
	}

	// Wait for Enter
	pausePrompt := e.LoadedStrings.PauseString // Use configured pause string
	if pausePrompt == "" {
		log.Printf("WARN: Node %d: PauseString is empty in config/strings.json. No pause prompt will be shown for SHOWVERSION.", nodeNumber)
		// Don't use a hardcoded fallback. If it's empty, it's empty.
	} else {
		terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode) // Add newline before pause only if prompt exists
		var pauseBytesToWrite []byte
		processedPausePrompt := ansi.ReplacePipeCodes([]byte(pausePrompt))
		if outputMode == ansi.OutputModeCP437 {
			var cp437Buf bytes.Buffer
			for _, r := range string(processedPausePrompt) {
				if r < 128 {
					cp437Buf.WriteByte(byte(r))
				} else if cp437Byte, ok := ansi.UnicodeToCP437[r]; ok {
					cp437Buf.WriteByte(cp437Byte)
				} else {
					cp437Buf.WriteByte('?')
				}
			}
			pauseBytesToWrite = cp437Buf.Bytes()
		} else {
			pauseBytesToWrite = processedPausePrompt
		}
		wErr = terminalio.WriteProcessedBytes(terminal, pauseBytesToWrite, outputMode)
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing SHOWVERSION pause prompt: %v", nodeNumber, wErr)
		}
	}

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during SHOWVERSION pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading input during SHOWVERSION pause: %v", nodeNumber, err)
			return nil, "", err // Return error on read failure
		}
		// Correct rune literals for Enter key check (CR or LF)
		if r == '\r' || r == '\n' {
			break
		}
	}

	return nil, "", nil // Return to the current menu
}

// displayMessageAreaList is an internal helper to display the list of accessible message areas
// grouped by conference. It does not include a pause prompt.
func displayMessageAreaList(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, currentUser *user.User, outputMode ansi.OutputMode, nodeNumber int, sessionStartTime time.Time) error {
	log.Printf("DEBUG: Node %d: Displaying message area list (helper)", nodeNumber)

	// 1. Load templates
	templateDir := filepath.Join(e.MenuSetPath, "templates")
	topTemplateBytes, errTop := os.ReadFile(filepath.Join(templateDir, "MSGAREA.TOP"))
	midTemplateBytes, errMid := os.ReadFile(filepath.Join(templateDir, "MSGAREA.MID"))
	botTemplateBytes, errBot := os.ReadFile(filepath.Join(templateDir, "MSGAREA.BOT"))

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load MSGAREA template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading Message Area screen templates.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return fmt.Errorf("failed loading MSGAREA templates")
	}

	// Conference header template (optional)
	confHdrBytes, errConf := os.ReadFile(filepath.Join(templateDir, "MSGCONF.HDR"))
	confHdrTemplate := ""
	if errConf == nil {
		confHdrTemplate = string(ansi.ReplacePipeCodes(confHdrBytes))
	}

	processedTopTemplate := ansi.ReplacePipeCodes(topTemplateBytes)
	processedMidTemplate := string(ansi.ReplacePipeCodes(midTemplateBytes))
	processedBotTemplate := ansi.ReplacePipeCodes(botTemplateBytes)

	// 2. Get areas and group by conference
	areas := e.MessageMgr.ListAreas()

	// Build conference groups: conferenceID -> []*MessageArea (ACS-filtered)
	groups := make(map[int][]*message.MessageArea)
	confIDs := make(map[int]bool)
	for _, area := range areas {
		if !checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
			continue
		}
		groups[area.ConferenceID] = append(groups[area.ConferenceID], area)
		confIDs[area.ConferenceID] = true
	}

	// Sort conference IDs (0/ungrouped first)
	var sortedConfIDs []int
	if e.ConferenceMgr != nil {
		sortedConfIDs = e.ConferenceMgr.GetSortedConferenceIDs(confIDs)
	} else {
		for cid := range confIDs {
			sortedConfIDs = append(sortedConfIDs, cid)
		}
		sort.Ints(sortedConfIDs)
	}

	// 3. Build output
	var outputBuffer bytes.Buffer
	outputBuffer.Write(processedTopTemplate)

	areasDisplayed := 0
	for _, cid := range sortedConfIDs {
		areasInConf := groups[cid]
		if len(areasInConf) == 0 {
			continue
		}

		// Check conference ACS
		if cid != 0 && e.ConferenceMgr != nil {
			conf, found := e.ConferenceMgr.GetByID(cid)
			if found && !checkACS(conf.ACS, currentUser, s, terminal, sessionStartTime) {
				continue
			}
			// Write conference header
			if found && confHdrTemplate != "" {
				hdr := confHdrTemplate
				hdr = strings.ReplaceAll(hdr, "^CN", conf.Name)
				hdr = strings.ReplaceAll(hdr, "^CT", conf.Tag)
				hdr = strings.ReplaceAll(hdr, "^CD", conf.Description)
				hdr = strings.ReplaceAll(hdr, "^CI", strconv.Itoa(conf.ID))
				outputBuffer.WriteString(hdr)
			}
		}

		for _, area := range areasInConf {
			line := processedMidTemplate
			line = strings.ReplaceAll(line, "^ID", strconv.Itoa(area.ID))
			line = strings.ReplaceAll(line, "^TAG", string(ansi.ReplacePipeCodes([]byte(area.Tag))))
			line = strings.ReplaceAll(line, "^NA", string(ansi.ReplacePipeCodes([]byte(area.Name))))
			line = strings.ReplaceAll(line, "^DS", string(ansi.ReplacePipeCodes([]byte(area.Description))))
			outputBuffer.WriteString(line)
			areasDisplayed++
		}
	}

	if areasDisplayed == 0 {
		outputBuffer.WriteString("\r\n|07   No accessible message areas found.   \r\n")
	}

	outputBuffer.Write(processedBotTemplate)

	// 4. Display
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	return terminalio.WriteProcessedBytes(terminal, outputBuffer.Bytes(), outputMode)
}

// runListMessageAreas displays a list of message areas using templates, then pauses.
func runListMessageAreas(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running LISTMSGAR", nodeNumber)

	// Filter to current conference if user is logged in, otherwise show all
	filterConfID := -1
	if currentUser != nil {
		filterConfID = currentUser.CurrentMsgConferenceID
	}
	if err := displayMessageAreaListFiltered(e, s, terminal, currentUser, outputMode, nodeNumber, sessionStartTime, filterConfID); err != nil {
		return nil, "", err
	}

	// Wait for Enter using configured PauseString
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... "
	}
	terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(pausePrompt)), outputMode)

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", err
		}
		if r == '\r' || r == '\n' {
			break
		}
	}

	return nil, "", nil
}

// runComposeMessage handles the process of composing and saving a new message.
func runComposeMessage(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running COMPOSEMSG with args: %s", nodeNumber, args)

	// 1. Determine Target Area
	var areaTag string
	var area *message.MessageArea // Use pointer type
	var exists bool

	if args == "" {
		// No args provided, use current user's area
		if currentUser == nil {
			log.Printf("WARN: Node %d: COMPOSEMSG called without user and without args.", nodeNumber)
			msg := "\r\n|01Error: Not logged in and no area specified.|07\r\n"
			wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return nil, "", nil // Return to menu
		}
		if currentUser.CurrentMessageAreaTag == "" || currentUser.CurrentMessageAreaID <= 0 {
			log.Printf("WARN: Node %d: COMPOSEMSG called by %s, but no current message area is set.", nodeNumber, currentUser.Handle)
			msg := "\r\n|01Error: No current message area selected.|07\r\n"
			wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return nil, "", nil // Return to menu
		}
		areaTag = currentUser.CurrentMessageAreaTag
		log.Printf("INFO: Node %d: COMPOSEMSG using current user area tag: %s", nodeNumber, areaTag)
		area, exists = e.MessageMgr.GetAreaByTag(areaTag)
	} else {
		// Args provided, use args as the area tag
		log.Printf("INFO: Node %d: COMPOSEMSG using provided area tag in args: %s", nodeNumber, args)
		areaTag = args
		area, exists = e.MessageMgr.GetAreaByTag(areaTag)
	}

	// Common checks after determining areaTag/area
	if !exists {
		log.Printf("ERROR: Node %d: COMPOSEMSG called with invalid Area Tag: %s", nodeNumber, areaTag)
		msg := fmt.Sprintf("\r\n|01Invalid message area: %s|07\r\n", areaTag)
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu, not an error
	}

	// Check user logged in (required for ACS check and posting)
	if currentUser == nil {
		log.Printf("WARN: Node %d: COMPOSEMSG reached ACS check without logged in user (Area: %s).", nodeNumber, areaTag)
		msg := "\r\n|01Error: You must be logged in to post messages.|07\r\n"
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	// Check ACSWrite permission for the area and currentUser
	if !checkACS(area.ACSWrite, currentUser, s, terminal, sessionStartTime) {
		log.Printf("WARN: Node %d: User %s denied post access to area %s (%s)", nodeNumber, currentUser.Handle, area.Tag, area.ACSWrite)
		// TODO: Display user-friendly error message (e.g., Access Denied String)
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu, not an error
	}

	// === PASCAL-STYLE MESSAGE POSTING FLOW ===

	// 2. Prompt for Title (30 chars)
	titlePrompt := e.LoadedStrings.MsgTitleStr
	if titlePrompt == "" {
		titlePrompt = "|07Title: |15"
	}
	terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
	wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(titlePrompt)), outputMode)
	if wErr != nil {
		log.Printf("WARN: Node %d: Failed to write title prompt: %v", nodeNumber, wErr)
	}

	subject, err := styledInput(terminal, s, outputMode, 30, "")
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during title input.", nodeNumber)
			return nil, "LOGOFF", io.EOF
		}
		log.Printf("ERROR: Node %d: Failed reading title input: %v", nodeNumber, err)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nError reading title.\r\n"), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = "(no subject)"
	}

	// 3. Prompt for To (24 chars, default "All")
	toPrompt := e.LoadedStrings.MsgToStr
	if toPrompt == "" {
		toPrompt = "|07To: |15"
	}
	wErr = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(toPrompt)), outputMode)
	if wErr != nil {
		log.Printf("WARN: Node %d: Failed to write 'to' prompt: %v", nodeNumber, wErr)
	}

	toUser, err := styledInput(terminal, s, outputMode, 24, "All")
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during 'to' input.", nodeNumber)
			return nil, "LOGOFF", io.EOF
		}
		log.Printf("ERROR: Node %d: Failed reading 'to' input: %v", nodeNumber, err)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nError reading recipient.\r\n"), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}
	toUser = strings.TrimSpace(toUser)
	if toUser == "" {
		toUser = "All"
	}

	// 4. Prompt for Anonymous (if user level >= AnonymousLevel)
	isAnonymous := false
	allowAnon := currentUser.AccessLevel >= e.ServerCfg.AnonymousLevel
	if allowAnon {
		areaAllowsAnon := true
		if area.AllowAnon != nil {
			areaAllowsAnon = *area.AllowAnon
		}
		confAllowsAnon := true
		if e.ConferenceMgr != nil && area.ConferenceID != 0 {
			if conf, ok := e.ConferenceMgr.GetByID(area.ConferenceID); ok {
				if conf.AllowAnon != nil {
					confAllowsAnon = *conf.AllowAnon
				}
			}
		}
		allowAnon = areaAllowsAnon && confAllowsAnon
	}
	if allowAnon {
		anonPrompt := e.LoadedStrings.MsgAnonStr
		if anonPrompt == "" {
			anonPrompt = "|07Anonymous? @"
		}
		isAnon, err := e.promptYesNo(s, terminal, anonPrompt, outputMode, nodeNumber)
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during anonymous input.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading anonymous input: %v", nodeNumber, err)
			isAnon = false
		}
		isAnonymous = isAnon
		terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
	}

	// 5. Prompt for Upload (Y/N)
	uploadPrompt := e.LoadedStrings.UploadMsgStr
	if uploadPrompt == "" {
		uploadPrompt = "|07Upload Message? @"
	}
	uploadYes, err := e.promptYesNo(s, terminal, uploadPrompt, outputMode, nodeNumber)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during upload input.", nodeNumber)
			return nil, "LOGOFF", io.EOF
		}
		log.Printf("ERROR: Node %d: Failed reading upload input: %v", nodeNumber, err)
		uploadYes = false
	}
	terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
	// TODO: Implement upload functionality if uploadYes == true
	_ = uploadYes // Suppress unused warning for now

	// 6. Call the Editor
	log.Printf("DEBUG: Node %d: Clearing screen before calling editor.RunEditor", nodeNumber)
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode) // Clear screen before editor

	// No quote data for new messages
	body, saved, err := editor.RunEditorWithMetadata("", s, s, outputMode, subject, toUser, isAnonymous, "", "", "", "", false, nil)
	log.Printf("DEBUG: Node %d: editor.RunEditorWithMetadata returned. Error: %v, Saved: %v, Body length: %d", nodeNumber, err, saved, len(body))

	if err != nil {
		log.Printf("ERROR: Node %d: Editor failed for user %s: %v", nodeNumber, currentUser.Handle, err)
		return nil, "", fmt.Errorf("editor error: %w", err)
	}

	// Clear screen after editor exits
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)

	if !saved {
		log.Printf("INFO: Node %d: User %s aborted message composition for area %s.", nodeNumber, currentUser.Handle, area.Tag)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nMessage aborted.\r\n"), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to current menu
	}

	if strings.TrimSpace(body) == "" {
		log.Printf("INFO: Node %d: User %s saved empty message for area %s.", nodeNumber, currentUser.Handle, area.Tag)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nMessage body empty. Aborting post.\r\n"), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to current menu
	}

	// 7. Save the Message via JAM backend
	// Determine the "from" name (may be anonymous)
	fromName := currentUser.Handle
	if isAnonymous {
		fromName = strings.TrimSpace(e.LoadedStrings.AnonymousName)
		if fromName == "" {
			fromName = "Anonymous"
		}
	}

	msgNum, err := e.MessageMgr.AddMessage(area.ID, fromName, toUser, subject, body, "")
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to save message from user %s to area %s: %v", nodeNumber, currentUser.Handle, area.Tag, err)
		errorMsg := ansi.ReplacePipeCodes([]byte("\r\n|01Error saving message!|07\r\n"))
		terminalio.WriteProcessedBytes(terminal, errorMsg, outputMode)
		time.Sleep(2 * time.Second)
		return nil, "", fmt.Errorf("failed saving message: %w", err)
	}

	// 8. Update user message counter
	currentUser.MessagesPosted++
	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to update MessagesPosted for user %s: %v", nodeNumber, currentUser.Handle, err)
	}

	// 9. Confirmation
	log.Printf("INFO: Node %d: User %s successfully posted message #%d to area %s", nodeNumber, currentUser.Handle, msgNum, area.Tag)
	confirmMsg := ansi.ReplacePipeCodes([]byte("\r\n|02Message Posted!|07\r\n"))
	terminalio.WriteProcessedBytes(terminal, confirmMsg, outputMode)
	time.Sleep(1 * time.Second)

	return nil, "", nil
}

// runPromptAndComposeMessage lists areas, prompts for selection, checks permissions, and calls runComposeMessage.
func runPromptAndComposeMessage(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running runPromptAndComposeMessage", nodeNumber)

	if currentUser == nil {
		log.Printf("WARN: Node %d: runPromptAndComposeMessage called without logged in user.", nodeNumber)
		// Display user-friendly error
		msg := "\r\n|01Error: You must be logged in to post messages.|07\r\n"
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing login required message: %v", nodeNumber, wErr)
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	// 1. Display available message areas (adapted from runListMessageAreas, without pause)
	topTemplateFilename := "MSGAREA.TOP"
	midTemplateFilename := "MSGAREA.MID"
	botTemplateFilename := "MSGAREA.BOT" // We'll use BOT template differently here
	templateDir := filepath.Join(e.MenuSetPath, "templates")
	topTemplatePath := filepath.Join(templateDir, topTemplateFilename)
	midTemplatePath := filepath.Join(templateDir, midTemplateFilename)
	botTemplatePath := filepath.Join(templateDir, botTemplateFilename) // Load BOT template

	topTemplateBytes, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplateBytes, errBot := os.ReadFile(botTemplatePath) // Load BOT template

	if errTop != nil || errMid != nil || errBot != nil { // Check BOT error too
		log.Printf("ERROR: Node %d: Failed to load one or more MSGAREA template files for prompt: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading Message Area screen templates.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading MSGAREA templates for prompt")
	}

	processedTopTemplate := ansi.ReplacePipeCodes(topTemplateBytes)
	processedMidTemplate := string(ansi.ReplacePipeCodes(midTemplateBytes))
	processedBotTemplate := ansi.ReplacePipeCodes(botTemplateBytes) // Process BOT template

	areas := e.MessageMgr.ListAreas() // Get all areas
	// Filter areas based on read access (for listing)
	// For now, list all areas, permission check happens later on selection.
	// TODO: Implement ACSRead filtering here if needed for the list display.

	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode) // Clear before displaying list
	terminalio.WriteProcessedBytes(terminal, processedTopTemplate, outputMode)       // Write TOP

	if len(areas) == 0 {
		log.Printf("DEBUG: Node %d: No message areas available to post in.", nodeNumber)
		noAreasMsg := ansi.ReplacePipeCodes([]byte("\r\n|07No message areas available.|07\r\n"))
		terminalio.WriteProcessedBytes(terminal, noAreasMsg, outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	for _, area := range areas {
		line := processedMidTemplate
		name := string(ansi.ReplacePipeCodes([]byte(area.Name)))
		desc := string(ansi.ReplacePipeCodes([]byte(area.Description)))
		idStr := strconv.Itoa(area.ID)
		tag := string(ansi.ReplacePipeCodes([]byte(area.Tag)))

		line = strings.ReplaceAll(line, "^ID", idStr)
		line = strings.ReplaceAll(line, "^TAG", tag)
		line = strings.ReplaceAll(line, "^NA", name)
		line = strings.ReplaceAll(line, "^DS", desc)

		terminalio.WriteProcessedBytes(terminal, []byte(line), outputMode) // Write MID for each area
	}

	terminalio.WriteProcessedBytes(terminal, processedBotTemplate, outputMode) // Write BOT

	// 2. Prompt for Area Selection
	// TODO: Use a configurable string for this prompt
	prompt := "\r\n|07Enter Area ID or Tag to Post In (or Enter to cancel): |15"
	log.Printf("DEBUG: Node %d: Writing prompt for message area selection bytes (hex): %x", nodeNumber, ansi.ReplacePipeCodes([]byte(prompt)))
	wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
	if wErr != nil {
		log.Printf("WARN: Node %d: Failed to write area selection prompt: %v", nodeNumber, wErr)
	}

	input, err := terminal.ReadLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during area selection.", nodeNumber)
			return nil, "LOGOFF", io.EOF
		}
		log.Printf("ERROR: Node %d: Failed reading area selection input: %v", nodeNumber, err)
		return nil, "", fmt.Errorf("failed reading area selection: %w", err)
	}

	selectedAreaStr := strings.TrimSpace(input)
	if selectedAreaStr == "" {
		log.Printf("INFO: Node %d: User cancelled message posting.", nodeNumber)
		// TODO: Need to redraw the menu screen!
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nPost cancelled.\r\n"), outputMode)
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil // Return to current menu
	}

	// 3. Find Selected Area and Check Permissions
	var selectedArea *message.MessageArea // CORRECTED TYPE to pointer
	var foundArea bool

	// Try parsing as ID first
	if areaID, err := strconv.Atoi(selectedAreaStr); err == nil {
		selectedArea, foundArea = e.MessageMgr.GetAreaByID(areaID)
	}

	// If not found by ID, try by Tag (case-insensitive)
	if !foundArea {
		selectedArea, foundArea = e.MessageMgr.GetAreaByTag(strings.ToUpper(selectedAreaStr))
	}

	if !foundArea {
		log.Printf("WARN: Node %d: Invalid area selection '%s' by user %s.", nodeNumber, selectedAreaStr, currentUser.Handle)
		// TODO: Use configurable string
		msg := fmt.Sprintf("\r\n|01Invalid area: %s|07\r\n", selectedAreaStr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		// TODO: Need to redraw menu
		return nil, "", nil // Return to menu
	}

	// Check write permission
	if !checkACS(selectedArea.ACSWrite, currentUser, s, terminal, sessionStartTime) {
		log.Printf("WARN: Node %d: User %s denied post access to selected area %s (%s)", nodeNumber, currentUser.Handle, selectedArea.Tag, selectedArea.ACSWrite)
		// TODO: Use configurable string for access denied
		msg := fmt.Sprintf("\r\n|01Access denied to post in area: %s|07\r\n", selectedArea.Name)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		// TODO: Need to redraw menu
		return nil, "", nil // Return to menu
	}

	log.Printf("INFO: Node %d: User %s selected area %s (%s) to post in.", nodeNumber, currentUser.Handle, selectedArea.Name, selectedArea.Tag)

	// 4. Call runComposeMessage with the selected Area Tag
	// Pass the area tag as the argument string
	return runComposeMessage(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, selectedArea.Tag, outputMode)
}

// runReadMsgs handles reading messages from the user's current area.
// Delegates to runMessageReader which uses Pascal-style MSGHDR templates and lightbar navigation.
func runReadMsgs(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running READMSGS", nodeNumber)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in to read messages.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	currentAreaID := currentUser.CurrentMessageAreaID
	currentAreaTag := currentUser.CurrentMessageAreaTag

	if currentAreaID <= 0 || currentAreaTag == "" {
		msg := "\r\n|01Error: No message area selected.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Prompt for header selection if not yet set
	if currentUser.MsgHdr < 1 || currentUser.MsgHdr > 14 {
		// Check if MSGHDR.ANS exists for selection screen
		selPath := filepath.Join(e.MenuSetPath, "templates", "message_headers", "MSGHDR.ANS")
		if _, statErr := os.Stat(selPath); statErr == nil {
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07Please select a message header style.|07\r\n")), outputMode)
			time.Sleep(500 * time.Millisecond)
			runGetHeaderType(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, "", outputMode)
		}
	}

	totalMessageCount, err := e.MessageMgr.GetMessageCountForArea(currentAreaID)
	if err != nil {
		msg := fmt.Sprintf("\r\n|01Error loading message info for area %s.|07\r\n", currentAreaTag)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", err
	}

	if totalMessageCount == 0 {
		msg := fmt.Sprintf("\r\n|07No messages in area |15%s|07.\r\n", currentAreaTag)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Determine starting message
	newCount, err := e.MessageMgr.GetNewMessageCount(currentAreaID, currentUser.Handle)
	if err != nil {
		newCount = 0
	}

	var currentMsgNum int
	if newCount > 0 {
		currentMsgNum = totalMessageCount - newCount + 1
	} else {
		// No new messages - prompt for specific message number
		noNewMsg := fmt.Sprintf("\r\n|07No new messages in area |15%s|07.", currentAreaTag)
		totalMsg := fmt.Sprintf(" |07Total messages: |15%d|07.", totalMessageCount)
		promptMsg := fmt.Sprintf("\r\n|07Read message # (|151-%d|07, |15Enter|07=Cancel): |15", totalMessageCount)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(noNewMsg+totalMsg)), outputMode)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(promptMsg)), outputMode)

		input, readErr := terminal.ReadLine()
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", readErr
		}
		selectedNumStr := strings.TrimSpace(input)
		if selectedNumStr == "" {
			return nil, "", nil
		}
		selectedNum, parseErr := strconv.Atoi(selectedNumStr)
		if parseErr != nil || selectedNum < 1 || selectedNum > totalMessageCount {
			msg := fmt.Sprintf("\r\n|01Invalid message number: %s|07\r\n", selectedNumStr)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(1 * time.Second)
			return nil, "", nil
		}
		currentMsgNum = selectedNum
	}

	// Delegate to the new message reader with MSGHDR templates and lightbar
	return runMessageReader(e, s, terminal, userManager, currentUser, nodeNumber,
		sessionStartTime, outputMode, currentMsgNum, totalMessageCount, false)
}

// wrapAnsiString wraps a string containing ANSI codes to a given width.
// NOTE: This is a simplified version and does NOT perfectly handle ANSI state across wrapped lines.
// It primarily prevents lines from exceeding the terminal width visually.
func wrapAnsiString(text string, width int) []string {
	if width <= 0 {
		return strings.Split(text, "\n") // No wrapping if width is invalid
	}

	var wrappedLines []string
	// Split input into lines first based on existing newlines
	inputLines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	reAnsi := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`) // Basic regex for ANSI codes

	for _, line := range inputLines {
		plainLine := reAnsi.ReplaceAllString(line, "")
		if strings.TrimSpace(plainLine) == "" {
			wrappedLines = append(wrappedLines, "")
			continue
		}
		if isQuoteLine(plainLine) || isTearLine(plainLine) || isOriginLine(plainLine) {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		currentLine := ""
		currentWidth := 0
		words := strings.Fields(line) // Split line into words

		for _, word := range words {
			// Calculate visible width of the word (stripping ANSI)
			plainWord := reAnsi.ReplaceAllString(word, "")
			wordWidth := len(plainWord)

			if currentWidth == 0 {
				// First word on the line
				if wordWidth > width {
					// Word is longer than the line width, just append it (will overflow)
					wrappedLines = append(wrappedLines, word)
					currentLine = ""
					currentWidth = 0
				} else {
					currentLine = word
					currentWidth = wordWidth
				}
			} else {
				// Subsequent words
				if currentWidth+1+wordWidth <= width {
					// Word fits on the current line
					currentLine += " " + word
					currentWidth += 1 + wordWidth
				} else {
					// Word doesn't fit, wrap to next line
					wrappedLines = append(wrappedLines, currentLine)
					if wordWidth > width {
						// Word itself is too long, put it on its own line
						wrappedLines = append(wrappedLines, word)
						currentLine = ""
						currentWidth = 0
					} else {
						// Start new line with the current word
						currentLine = word
						currentWidth = wordWidth
					}
				}
			}
		}
		// Add the last line being built
		if currentWidth > 0 {
			wrappedLines = append(wrappedLines, currentLine)
		}
	}

	return wrappedLines
}

// writeProcessedStringWithManualEncoding takes bytes that have already had pipe codes
// replaced with standard ANSI escapes and writes them to the terminal, handling
// character encoding manually based on the desired outputMode.
// It now correctly handles UTF-8 input strings containing ANSI codes.
func writeProcessedStringWithManualEncoding(terminal *term.Terminal, processedBytes []byte, outputMode ansi.OutputMode) error {
	var finalBuf bytes.Buffer
	i := 0
	processedString := string(processedBytes) // Work with the UTF-8 string

	for i < len(processedString) {
		// Check for ANSI escape sequence start
		if processedString[i] == '\x1b' { // <-- Corrected: Use character literal
			start := i
			// Find the end of the ANSI sequence (basic CSI parsing)
			if i+1 < len(processedString) && processedString[i+1] == '[' {
				i += 2 // Skip ESC [
				for i < len(processedString) {
					c := processedString[i]
					if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') { // Found terminator
						i++
						break
					}
					i++
					// Basic protection
					if i-start > 30 {
						log.Printf("WARN: [writeProcessedString] Potential runaway ANSI sequence encountered.")
						break
					}
				}
			} else {
				// Handle other potential escape sequences if necessary (e.g., ESC ( B )
				// For now, assume simple non-CSI escapes are short or handle known ones
				// Example: ESC ( B (designate US-ASCII) is 3 bytes
				if i+2 < len(processedString) && processedString[i+1] == '(' && processedString[i+2] == 'B' {
					i += 3
				} else {
					i++ // Just skip the ESC if unknown sequence
				}
			}
			// Write the entire ANSI sequence as is
			finalBuf.WriteString(processedString[start:i])
			continue // Continue outer loop
		}

		// Decode the next rune from the UTF-8 string
		r, size := utf8.DecodeRuneInString(processedString[i:])
		if r == utf8.RuneError && size <= 1 {
			// Invalid UTF-8 sequence, write a placeholder or skip
			finalBuf.WriteByte('?')
			i++ // Move past the invalid byte
			continue
		}

		// Now handle the valid rune 'r' based on outputMode
		if outputMode == ansi.OutputModeCP437 {
			if r < 128 {
				// ASCII character, write directly
				finalBuf.WriteByte(byte(r))
			} else if cp437Byte, ok := ansi.UnicodeToCP437[r]; ok {
				// Found a corresponding CP437 byte
				finalBuf.WriteByte(cp437Byte)
			} else {
				// Unicode character doesn't exist in CP437, write fallback
				finalBuf.WriteByte('?')
			}
		} else { // OutputModeUTF8 or OutputModeAuto (assuming UTF-8 if not CP437)
			// Write the original rune (which is already UTF-8)
			finalBuf.WriteRune(r)
		}

		i += size // Move past the processed rune
	}

	// Write the fully processed buffer to the terminal
	err := terminalio.WriteProcessedBytes(terminal, finalBuf.Bytes(), outputMode)
	return err
}

// runNewscan handles the message newscan with Pascal-style GetScanType setup and multi-area flow.
func runNewscan(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running NEWSCAN for user %s", nodeNumber, currentUser.Handle)

	// Determine if this is a "current area only" scan based on args
	currentOnly := strings.ToUpper(strings.TrimSpace(args)) == "CURRENT"

	return runNewScanAll(e, s, terminal, userManager, currentUser, nodeNumber,
		sessionStartTime, outputMode, currentOnly)
}

// generateReplySubject creates a suitable subject line for a reply.
// It prepends "Re: " unless the original subject already starts with it (case-insensitive).
func generateReplySubject(originalSubject string) string {
	upperSubject := strings.ToUpper(strings.TrimSpace(originalSubject))
	if strings.HasPrefix(upperSubject, "RE:") {
		return originalSubject // Already a reply
	}
	return "Re: " + originalSubject
}

// formatQuote formats the body of an original message for quoting in a reply.
// It prepends each line with the specified quotePrefix.
func formatQuote(originalMsg *message.DisplayMessage, quotePrefix string) string {
	if originalMsg == nil || originalMsg.Body == "" {
		return ""
	}

	var builder strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(originalMsg.Body))
	for scanner.Scan() {
		builder.WriteString(quotePrefix)
		builder.WriteString(scanner.Text())
		builder.WriteString("\n") // Use ACTUAL newline for editor buffer
	}
	// Check for scanner errors, although unlikely with strings.Reader
	if err := scanner.Err(); err != nil {
		log.Printf("WARN: Error scanning original message body for quoting: %v", err)
		// Return whatever was built so far, or perhaps an error indicator?
		// For now, just return the potentially partial quote.
	}
	return builder.String()
}

// runListFiles displays a paginated list of files in the current file area.
func runListFiles(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running LISTFILES", nodeNumber)

	// 1. Check User and Current File Area
	if currentUser == nil {
		log.Printf("WARN: Node %d: LISTFILES called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to list files.|07\r\n"
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	// Get current file area from user session
	currentAreaID := currentUser.CurrentFileAreaID
	currentAreaTag := currentUser.CurrentFileAreaTag

	if currentAreaID <= 0 {
		log.Printf("WARN: Node %d: User %s has no current file area selected.", nodeNumber, currentUser.Handle)
		msg := "\r\n|01Error: No file area selected.|07\r\n"
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	log.Printf("INFO: Node %d: User %s listing files for Area ID %d (%s)", nodeNumber, currentUser.Handle, currentAreaID, currentAreaTag)

	// Check Read ACS for the file area
	area, exists := e.FileMgr.GetAreaByID(currentAreaID)
	if !exists || !checkACS(area.ACSList, currentUser, s, terminal, sessionStartTime) {
		log.Printf("WARN: Node %d: User %s denied read access to file area %d (%s) due to ACS '%s'", nodeNumber, currentUser.Handle, currentAreaID, currentAreaTag, area.ACSList)
		// Display error message
		return nil, "", nil // Return to menu
	}

	// 2. Load Templates (FILELIST.TOP, FILELIST.MID, FILELIST.BOT)
	topTemplatePath := filepath.Join(e.MenuSetPath, "templates", "FILELIST.TOP")
	midTemplatePath := filepath.Join(e.MenuSetPath, "templates", "FILELIST.MID")
	botTemplatePath := filepath.Join(e.MenuSetPath, "templates", "FILELIST.BOT")

	topTemplateBytes, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplateBytes, errBot := os.ReadFile(botTemplatePath)

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load one or more FILELIST template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading File List screen templates.|07\r\n"
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading FILELIST templates")
	}

	processedTopTemplate := ansi.ReplacePipeCodes(topTemplateBytes)
	processedMidTemplate := string(ansi.ReplacePipeCodes(midTemplateBytes))
	processedBotTemplate := ansi.ReplacePipeCodes(botTemplateBytes)

	// 3. Fetch Files and Pagination Logic
	// --- Determine lines available per page ---
	termWidth := 80  // Default width
	termHeight := 24 // Default height
	ptyReq, _, isPty := s.Pty()
	if isPty && ptyReq.Window.Width > 0 && ptyReq.Window.Height > 0 {
		termWidth = ptyReq.Window.Width // Use actual width later for wrapping/truncating if needed
		termHeight = ptyReq.Window.Height
	} else {
		log.Printf("WARN: Node %d: Could not get PTY dimensions for file list, using default %dx%d", nodeNumber, termWidth, termHeight)
	}

	// Estimate lines used by header, footer, prompt
	headerLines := bytes.Count(processedTopTemplate, []byte("\n")) + 1
	footerLines := bytes.Count(processedBotTemplate, []byte("\n")) + 1
	// TODO: Make prompt configurable and count its lines accurately
	promptLines := 2 // Estimate 2 lines for prompt + input line
	fixedLines := headerLines + footerLines + promptLines
	filesPerPage := termHeight - fixedLines
	if filesPerPage < 1 {
		filesPerPage = 1 // Ensure at least 1 file can be shown
	}
	log.Printf("DEBUG: Node %d: TermHeight=%d, FixedLines=%d, FilesPerPage=%d", nodeNumber, termHeight, fixedLines, filesPerPage)

	// --- Get Total File Count ---
	// TODO: Implement GetFileCountForArea in FileManager
	totalFiles, err := e.FileMgr.GetFileCountForArea(currentAreaID)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to get file count for area %d: %v", nodeNumber, currentAreaID, err)
		msg := fmt.Sprintf("\r\n|01Error retrieving file list for area '%s'.|07\r\n", currentAreaTag)
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed getting file count: %w", err)
	}

	totalPages := 0
	if totalFiles > 0 {
		totalPages = (totalFiles + filesPerPage - 1) / filesPerPage
	}
	if totalPages == 0 { // Ensure at least one page even if no files
		totalPages = 1
	}

	currentPage := 1                  // Start on page 1
	var filesOnPage []file.FileRecord // Use actual type from file package

	// --- Fetch Initial Page ---
	if totalFiles > 0 {
		// TODO: Implement GetFilesForAreaPaginated in FileManager
		filesOnPage, err = e.FileMgr.GetFilesForAreaPaginated(currentAreaID, currentPage, filesPerPage)
		if err != nil {
			log.Printf("ERROR: Node %d: Failed to get files for area %d, page %d: %v", nodeNumber, currentAreaID, currentPage, err)
			msg := fmt.Sprintf("\r\n|01Error retrieving file list page for area '%s'.|07\r\n", currentAreaTag)
			wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return nil, "", fmt.Errorf("failed getting file page: %w", err)
		}
	} else {
		filesOnPage = []file.FileRecord{} // Ensure empty slice if no files
	}

	// 4. Display Loop
	for {
		// 4.1 Clear Screen
		writeErr := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
		if writeErr != nil {
			log.Printf("ERROR: Node %d: Failed clearing screen for LISTFILES: %v", nodeNumber, writeErr)
			// Potentially return error or try to continue
		}

		// 4.2 Display Top Template
		wErr := terminalio.WriteProcessedBytes(terminal, processedTopTemplate, outputMode)
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing LISTFILES top template: %v", nodeNumber, wErr)
			// Handle error
		}
		// Add CRLF after TOP template before listing files
		wErr = terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing CRLF after LISTFILES top template: %v", nodeNumber, wErr)
			// Handle error
		}

		// 4.3 Display Files on Current Page (using MID template)
		if len(filesOnPage) == 0 {
			// Display "No files in this area" message
			// TODO: Use a configurable string?
			noFilesMsg := "\r\n|07   No files in this area.   \r\n"
			wErr = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(noFilesMsg)), outputMode)
			if wErr != nil { /* Log? */
			}
		} else {
			for i, fileRec := range filesOnPage {
				line := processedMidTemplate
				// Calculate display number for the file on the current page
				fileNumOnPage := (currentPage-1)*filesPerPage + i + 1

				// Populate placeholders (^MARK, ^NUM, ^NAME, ^DATE, ^SIZE, ^DESC) from fileRec
				fileNumStr := strconv.Itoa(fileNumOnPage)
				// Truncate filename and description if needed based on termWidth
				fileNameStr := fileRec.Filename                  // TODO: Truncate
				dateStr := fileRec.UploadedAt.Format("01/02/06") // Example date format
				sizeStr := fmt.Sprintf("%dk", fileRec.Size/1024) // Example size in K
				descStr := fileRec.Description                   // TODO: Truncate

				// Check if file is marked for download
				markStr := " " // Default to blank
				// Assumes currentUser.TaggedFileIDs is a slice of uuid.UUID
				if currentUser.TaggedFileIDs != nil {
					for _, taggedID := range currentUser.TaggedFileIDs {
						if taggedID == fileRec.ID {
							markStr = "*" // Or use a configured marker string
							break
						}
					}
				}

				line = strings.ReplaceAll(line, "^MARK", markStr)
				line = strings.ReplaceAll(line, "^NUM", fileNumStr)
				line = strings.ReplaceAll(line, "^NAME", fileNameStr)
				line = strings.ReplaceAll(line, "^DATE", dateStr)
				line = strings.ReplaceAll(line, "^SIZE", sizeStr)
				line = strings.ReplaceAll(line, "^DESC", descStr)

				// Write line using manual encoding helper in case of CP437 chars in data
				wErr = writeProcessedStringWithManualEncoding(terminal, []byte(line), outputMode)
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing file list line %d: %v", nodeNumber, i, wErr)
					// Handle error (e.g., break loop, return error?)
				}
				// Add CRLF after writing the line
				wErr = terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing CRLF after file list line %d: %v", nodeNumber, i, wErr)
					// Handle error
				}
			}
		}

		// 4.4 Display Bottom Template (with pagination info)
		bottomLine := string(processedBotTemplate)
		bottomLine = strings.ReplaceAll(bottomLine, "^PAGE", strconv.Itoa(currentPage))
		bottomLine = strings.ReplaceAll(bottomLine, "^TOTALPAGES", strconv.Itoa(totalPages))
		wErr = terminalio.WriteProcessedBytes(terminal, []byte(bottomLine), outputMode)
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing LISTFILES bottom template: %v", nodeNumber, wErr)
			// Handle error
		}

		// 4.5 Display Prompt (Use a standard file list prompt or configure one)
		// TODO: Use configurable prompt string
		prompt := "\r\n|07File Cmd (|15N|07=Next, |15P|07=Prev, |15#|07=Mark, |15D|07=Download, |15Q|07=Quit): |15"
		wErr = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
		if wErr != nil {
			// Handle error
		}

		// 4.6 Read User Input
		input, err := terminal.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during LISTFILES.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading LISTFILES input: %v", nodeNumber, err)
			// Consider retry or exit
			return nil, "", err
		}

		upperInput := strings.ToUpper(strings.TrimSpace(input))

		// 4.7 Process Input
		switch upperInput {
		case "N", " ", "": // Next Page (Space/Enter default to Next)
			if currentPage < totalPages {
				currentPage++
				// Fetch files for the new page
				filesOnPage, err = e.FileMgr.GetFilesForAreaPaginated(currentAreaID, currentPage, filesPerPage)
				if err != nil {
					// Log error and potentially return or break the loop
					log.Printf("ERROR: Node %d: Failed to get files for page %d: %v", nodeNumber, currentPage, err)
					// Display error message to user?
					time.Sleep(1 * time.Second)
				}
			} else {
				// Indicate last page (optional feedback)
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07Already on last page.|07")), outputMode)
				time.Sleep(500 * time.Millisecond)
			}
			continue // Redraw loop
		case "P": // Previous Page
			if currentPage > 1 {
				currentPage--
				// Fetch files for the new page
				filesOnPage, err = e.FileMgr.GetFilesForAreaPaginated(currentAreaID, currentPage, filesPerPage)
				if err != nil {
					// Log error and potentially return or break the loop
					log.Printf("ERROR: Node %d: Failed to get files for page %d: %v", nodeNumber, currentPage, err)
					// Display error message to user?
					time.Sleep(1 * time.Second)
				}
			} else {
				// Indicate first page (optional feedback)
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07Already on first page.|07")), outputMode)
				time.Sleep(500 * time.Millisecond)
			}
			continue // Redraw loop
		case "Q": // Quit
			log.Printf("DEBUG: Node %d: User quit LISTFILES.", nodeNumber)
			return nil, "", nil // Return to FILEM menu
		case "D": // Download marked files
			log.Printf("DEBUG: Node %d: User %s initiated Download command in area %d.", nodeNumber, currentUser.Handle, currentAreaID)

			// 1. Check if any files are marked
			if len(currentUser.TaggedFileIDs) == 0 {
				msg := "\\r\\n|07No files marked for download. Use |15#|07 to mark files.|07\\r\\n"
				wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				if wErr != nil { /* Log? */
				}
				time.Sleep(1 * time.Second)
				continue // Go back to file list display
			}

			// 2. Confirm download
			confirmPrompt := fmt.Sprintf("Download %d marked file(s)?", len(currentUser.TaggedFileIDs))
			// Use WriteProcessedBytes for SaveCursor, positioning, and clear line
			// Need to position this prompt carefully, perhaps near the bottom prompt line.
			// For now, just display it after the main prompt. TODO: Improve positioning.
			terminalio.WriteProcessedBytes(terminal, []byte(ansi.SaveCursor()), outputMode)
			terminalio.WriteProcessedBytes(terminal, []byte("\\r\\n\\x1b[K"), outputMode) // Newline, clear line

			proceed, err := e.promptYesNo(s, terminal, confirmPrompt, outputMode, nodeNumber)
			terminalio.WriteProcessedBytes(terminal, []byte(ansi.RestoreCursor()), outputMode) // Restore cursor after prompt

			if err != nil {
				if errors.Is(err, io.EOF) {
					log.Printf("INFO: Node %d: User disconnected during download confirmation.", nodeNumber)
					return nil, "LOGOFF", io.EOF
				}
				log.Printf("ERROR: Node %d: Error getting download confirmation: %v", nodeNumber, err)
				msg := "\\r\\n|01Error during confirmation.|07\\r\\n"
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(1 * time.Second)
				continue // Back to file list
			}

			if !proceed {
				log.Printf("DEBUG: Node %d: User cancelled download.", nodeNumber)
				terminalio.WriteProcessedBytes(terminal, []byte("\\r\\n|07Download cancelled.|07"), outputMode)
				time.Sleep(500 * time.Millisecond)
				continue // Back to file list
			}

			// 3. Process downloads
			log.Printf("INFO: Node %d: User %s starting download of %d files.", nodeNumber, currentUser.Handle, len(currentUser.TaggedFileIDs))
			terminalio.WriteProcessedBytes(terminal, []byte("\\r\\n|07Preparing download...\\r\\n"), outputMode)
			time.Sleep(500 * time.Millisecond) // Small pause

			successCount := 0
			failCount := 0
			filesToDownload := make([]string, 0, len(currentUser.TaggedFileIDs))
			filenamesOnly := make([]string, 0, len(currentUser.TaggedFileIDs))

			for _, fileID := range currentUser.TaggedFileIDs {
				filePath, pathErr := e.FileMgr.GetFilePath(fileID)
				if pathErr != nil {
					log.Printf("ERROR: Node %d: Failed to get path for file ID %s: %v", nodeNumber, fileID, pathErr)
					failCount++
					continue // Skip this file
				}
				// Check if file exists before adding to list
				if _, statErr := os.Stat(filePath); os.IsNotExist(statErr) {
					log.Printf("ERROR: Node %d: File path %s for ID %s does not exist on server.", nodeNumber, filePath, fileID)
					failCount++
					continue
				} else if statErr != nil {
					log.Printf("ERROR: Node %d: Error stating file path %s for ID %s: %v", nodeNumber, filePath, fileID, statErr)
					failCount++
					continue
				}
				filesToDownload = append(filesToDownload, filePath)
				filenamesOnly = append(filenamesOnly, filepath.Base(filePath))
			}

			if len(filesToDownload) > 0 {
				// **** Actual ZMODEM Transfer using sz ****
				log.Printf("INFO: Node %d: Attempting ZMODEM transfer for files: %v", nodeNumber, filenamesOnly)
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|15Initiating ZMODEM transfer (sz)...\\r\\n")), outputMode)
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|07Please start the ZMODEM receive function in your terminal.\\r\\n")), outputMode)

				// 1. Find sz executable
				szPath, err := exec.LookPath("sz")
				if err != nil {
					log.Printf("ERROR: Node %d: 'sz' command not found in PATH: %v", nodeNumber, err)
					terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|01Error: 'sz' command not found on server. Cannot start download.\\r\\n")), outputMode)
					failCount = len(filesToDownload) // Mark all as failed
				} else {
					// 2. Prepare arguments - Re-add -e flag
					args := []string{"-b", "-e"} // Binary, Escape control chars
					args = append(args, filesToDownload...)
					log.Printf("DEBUG: Node %d: Executing command: %s %v", nodeNumber, szPath, args)

					// 3. Create command and use PTY helper
					cmd := exec.Command(szPath, args...)
					// Note: Stdin/Stdout/Stderr are handled by runCommandWithPTY

					log.Printf("INFO: Node %d: Executing Zmodem send via runCommandWithPTY: %s %v", nodeNumber, szPath, args)

					// 4. Execute using the PTY helper from the transfer package
					transferErr := transfer.RunCommandWithPTY(s, cmd) // Pass the ssh.Session and the command (Use exported name)

					// 5. Handle Result
					if transferErr != nil {
						// sz likely exited with an error (transfer failed or cancelled)
						log.Printf("ERROR: Node %d: 'sz' command execution failed: %v", nodeNumber, transferErr)
						terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|01ZMODEM transfer failed or was cancelled.\\r\\n")), outputMode)
						failCount = len(filesToDownload) // Assume all failed if sz returns error
						successCount = 0
					} else {
						// sz exited successfully (transfer presumed complete)
						log.Printf("INFO: Node %d: 'sz' command completed successfully.", nodeNumber)
						terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|07ZMODEM transfer complete.\\r\\n")), outputMode)
						successCount = len(filesToDownload) // Assume all succeeded if sz exits cleanly
						failCount = 0                       // Reset fail count determined earlier

						// Increment download counts only on successful transfer completion
						for _, fileID := range currentUser.TaggedFileIDs {
							// Check again if we had a valid path originally
							if _, pathErr := e.FileMgr.GetFilePath(fileID); pathErr == nil {
								if err := e.FileMgr.IncrementDownloadCount(fileID); err != nil {
									log.Printf("WARN: Node %d: Failed to increment download count for file %s after successful sz: %v", nodeNumber, fileID, err)
								}
							}
						}
					}
				}
				// Add a small delay after transfer attempt
				time.Sleep(1 * time.Second)
				// ---- End ZMODEM Transfer ----

			} else {
				log.Printf("WARN: Node %d: No valid file paths found for tagged files.", nodeNumber)
				msg := "\\r\\n|01Could not find any of the marked files on the server.|07\\r\\n"
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				failCount = len(currentUser.TaggedFileIDs) // Mark all as failed if none were found
			}

			// 4. Clear tags and save user state
			log.Printf("DEBUG: Node %d: Clearing %d tagged file IDs for user %s.", nodeNumber, len(currentUser.TaggedFileIDs), currentUser.Handle)
			currentUser.TaggedFileIDs = nil // Clear the list
			if err := userManager.UpdateUser(currentUser); err != nil {
				log.Printf("ERROR: Node %d: Failed to save user data after download attempt: %v", nodeNumber, err)
				// Inform user? State might be inconsistent.
				terminalio.WriteProcessedBytes(terminal, []byte("\\r\\n|01Error saving user state after download.|07"), outputMode)
			}

			// 5. Final status message
			statusMsg := fmt.Sprintf("|07Download attempt finished. Success: %d, Failed: %d.|07\r\n", successCount, failCount)
			terminalio.WriteProcessedBytes(terminal, []byte(statusMsg), outputMode)
			time.Sleep(2 * time.Second)

			// Go back to the file list (will redraw with cleared marks)
			continue
		case "U": // Upload (Placeholder)
			log.Printf("DEBUG: Node %d: Upload command entered (Not Implemented)", nodeNumber)
			msg := "\r\n|01Upload function not yet implemented.|07\r\n"
			wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			// Stay on the same page
		case "V": // View (Placeholder)
			log.Printf("DEBUG: Node %d: View command entered (Not Implemented)", nodeNumber)
			msg := "\r\n|01View function not yet implemented.|07\r\n"
			wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			// Stay on the same page
		case "A": // Area Change (Placeholder/Not implemented here, handled by menu?)
			log.Printf("DEBUG: Node %d: Area Change command entered (Handled by menu)", nodeNumber)
			msg := "\r\n|01Use menu options to change area.|07\r\n"
			wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
		default: // Includes 'T' (Tagging) and potential numeric input
			// Try to parse as a number for tagging
			fileNumToTag, err := strconv.Atoi(upperInput)
			if err == nil && fileNumToTag > 0 {
				// Valid number entered, attempt to tag/untag
				fileIndex := fileNumToTag - 1 - (currentPage-1)*filesPerPage
				if fileIndex >= 0 && fileIndex < len(filesOnPage) {
					fileToToggle := filesOnPage[fileIndex]
					found := false
					newTaggedIDs := []uuid.UUID{}
					if currentUser.TaggedFileIDs != nil {
						for _, taggedID := range currentUser.TaggedFileIDs {
							if taggedID == fileToToggle.ID {
								found = true // Mark as found to skip adding it back
							} else {
								newTaggedIDs = append(newTaggedIDs, taggedID)
							}
						}
					}
					if !found {
						// File was not tagged, so add it
						newTaggedIDs = append(newTaggedIDs, fileToToggle.ID)
						log.Printf("DEBUG: Node %d: User %s tagged file #%d (ID: %s)", nodeNumber, currentUser.Handle, fileNumToTag, fileToToggle.ID)
					} else {
						// File was tagged, so we removed it (untagged)
						log.Printf("DEBUG: Node %d: User %s untagged file #%d (ID: %s)", nodeNumber, currentUser.Handle, fileNumToTag, fileToToggle.ID)
					}
					currentUser.TaggedFileIDs = newTaggedIDs
					// No page change needed, loop will redraw with updated marks
				} else {
					// Invalid file number for current page
					log.Printf("DEBUG: Node %d: Invalid file number entered: %d", nodeNumber, fileNumToTag)
					// Optional: Add user feedback message
				}
			} else {
				// Input was not N, P, Q, D, U, V, A, or a valid number - Invalid command
				log.Printf("DEBUG: Node %d: Invalid command entered in LISTFILES: %s", nodeNumber, upperInput)
				// Optional: Add user feedback message
			}
		} // end switch
	} // end for loop

	// Should not be reached normally
	// return nil, "", nil
}

// displayFileAreaList is an internal helper to display the list of accessible file areas.
// It does not include a pause prompt.
func displayFileAreaList(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, currentUser *user.User, outputMode ansi.OutputMode, nodeNumber int, sessionStartTime time.Time) error {
	log.Printf("DEBUG: Node %d: Displaying file area list (helper)", nodeNumber)

	// 1. Define Template filenames and paths
	topTemplateFilename := "FILEAREA.TOP"
	midTemplateFilename := "FILEAREA.MID"
	botTemplateFilename := "FILEAREA.BOT"
	templateDir := filepath.Join(e.MenuSetPath, "templates")
	topTemplatePath := filepath.Join(templateDir, topTemplateFilename)
	midTemplatePath := filepath.Join(templateDir, midTemplateFilename)
	botTemplatePath := filepath.Join(templateDir, botTemplateFilename)

	// 2. Load Template Files
	topTemplateBytes, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplateBytes, errBot := os.ReadFile(botTemplatePath)

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load one or more FILEAREA template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		// Display error message to terminal
		msg := "\r\n|01Error loading File Area screen templates.|07\r\n"
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return fmt.Errorf("failed loading FILEAREA templates")
	}

	// 3. Process Pipe Codes in Templates FIRST
	processedTopTemplate := ansi.ReplacePipeCodes(topTemplateBytes)
	processedMidTemplate := string(ansi.ReplacePipeCodes(midTemplateBytes))
	processedBotTemplate := ansi.ReplacePipeCodes(botTemplateBytes)

	// Conference header template (optional)
	confHdrBytes, errConf := os.ReadFile(filepath.Join(templateDir, "FILECONF.HDR"))
	confHdrTemplate := ""
	if errConf == nil {
		confHdrTemplate = string(ansi.ReplacePipeCodes(confHdrBytes))
	}

	// 4. Get file area list data and group by conference
	areas := e.FileMgr.ListAreas()

	// Build conference groups: conferenceID -> []file.FileArea (ACS-filtered)
	groups := make(map[int][]file.FileArea)
	confIDs := make(map[int]bool)
	for _, area := range areas {
		if !checkACS(area.ACSList, currentUser, s, terminal, sessionStartTime) {
			log.Printf("TRACE: Node %d: User %s denied list access to file area %d (%s) due to ACS '%s'", nodeNumber, currentUser.Handle, area.ID, area.Tag, area.ACSList)
			continue
		}
		groups[area.ConferenceID] = append(groups[area.ConferenceID], area)
		confIDs[area.ConferenceID] = true
	}

	// Sort conference IDs (0/ungrouped first)
	var sortedConfIDs []int
	if e.ConferenceMgr != nil {
		sortedConfIDs = e.ConferenceMgr.GetSortedConferenceIDs(confIDs)
	} else {
		for cid := range confIDs {
			sortedConfIDs = append(sortedConfIDs, cid)
		}
		sort.Ints(sortedConfIDs)
	}

	// 5. Build the output string using processed templates and data
	var outputBuffer bytes.Buffer
	outputBuffer.Write(processedTopTemplate)

	areasDisplayed := 0
	for _, cid := range sortedConfIDs {
		areasInConf := groups[cid]
		if len(areasInConf) == 0 {
			continue
		}

		// Check conference ACS and write header
		if cid != 0 && e.ConferenceMgr != nil {
			conf, found := e.ConferenceMgr.GetByID(cid)
			if found && !checkACS(conf.ACS, currentUser, s, terminal, sessionStartTime) {
				continue
			}
			if found && confHdrTemplate != "" {
				hdr := confHdrTemplate
				hdr = strings.ReplaceAll(hdr, "^CN", conf.Name)
				hdr = strings.ReplaceAll(hdr, "^CT", conf.Tag)
				hdr = strings.ReplaceAll(hdr, "^CD", conf.Description)
				hdr = strings.ReplaceAll(hdr, "^CI", strconv.Itoa(conf.ID))
				outputBuffer.WriteString(hdr)
			}
		}

		for _, area := range areasInConf {
			line := processedMidTemplate
			name := string(ansi.ReplacePipeCodes([]byte(area.Name)))
			desc := string(ansi.ReplacePipeCodes([]byte(area.Description)))
			idStr := strconv.Itoa(area.ID)
			tag := string(ansi.ReplacePipeCodes([]byte(area.Tag)))
			fileCount, countErr := e.FileMgr.GetFileCountForArea(area.ID)
			if countErr != nil {
				log.Printf("WARN: Node %d: Failed getting file count for area %d (%s): %v", nodeNumber, area.ID, area.Tag, countErr)
				fileCount = 0
			}
			fileCountStr := strconv.Itoa(fileCount)

			line = strings.ReplaceAll(line, "^ID", idStr)
			line = strings.ReplaceAll(line, "^TAG", tag)
			line = strings.ReplaceAll(line, "^NA", name)
			line = strings.ReplaceAll(line, "^DS", desc)
			line = strings.ReplaceAll(line, "^NF", fileCountStr)

			outputBuffer.WriteString(line)
			areasDisplayed++
		}
	}

	if areasDisplayed == 0 {
		log.Printf("DEBUG: Node %d: No accessible file areas to display for user %s.", nodeNumber, currentUser.Handle)
		outputBuffer.WriteString("\r\n|07   No accessible file areas found.   \r\n")
	}

	outputBuffer.Write(processedBotTemplate)

	// 6. Clear screen and display the assembled content
	writeErr := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	if writeErr != nil {
		log.Printf("ERROR: Node %d: Failed clearing screen for file area list: %v", nodeNumber, writeErr)
		// Try to continue anyway?
	}

	processedContent := outputBuffer.Bytes()
	wErr := terminalio.WriteProcessedBytes(terminal, processedContent, outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing file area list output: %v", nodeNumber, wErr)
		return wErr // Return the error from writing
	}

	return nil // Success
}

// runListFileAreas displays a list of file areas using templates.
func runListFileAreas(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running LISTFILEAR", nodeNumber)

	if currentUser == nil {
		log.Printf("WARN: Node %d: LISTFILEAR called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to list file areas.|07\r\n"
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Call the helper to display the list
	if err := displayFileAreaList(e, s, terminal, currentUser, outputMode, nodeNumber, sessionStartTime); err != nil {
		// Error already logged by helper, maybe add context?
		log.Printf("ERROR: Node %d: Error occurred during displayFileAreaList from runListFileAreas: %v", nodeNumber, err)
		// Need to decide if we still pause or just return.
		// For now, return the error to prevent pause on failed display.
		return nil, "", err
	}

	// Wait for Enter using configured PauseString
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... " // Fallback
	}

	var pauseBytesToWrite []byte
	processedPausePrompt := ansi.ReplacePipeCodes([]byte(pausePrompt))
	if outputMode == ansi.OutputModeCP437 {
		var cp437Buf bytes.Buffer
		for _, r := range string(processedPausePrompt) {
			if r < 128 {
				cp437Buf.WriteByte(byte(r))
			} else if cp437Byte, ok := ansi.UnicodeToCP437[r]; ok {
				cp437Buf.WriteByte(cp437Byte)
			} else {
				cp437Buf.WriteByte('?')
			}
		}
		pauseBytesToWrite = cp437Buf.Bytes()
	} else {
		pauseBytesToWrite = processedPausePrompt
	}

	wErr := terminalio.WriteProcessedBytes(terminal, pauseBytesToWrite, outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing LISTFILEAR pause prompt: %v", nodeNumber, wErr)
	}

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during LISTFILEAR pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading input during LISTFILEAR pause: %v", nodeNumber, err)
			return nil, "", err
		}
		if r == '\r' || r == '\n' {
			break
		}
	}

	return nil, "", nil // Success, return to current menu (FILEM)
}

// runSelectFileArea prompts the user for a file area tag and changes the current user's
// active file area if valid and accessible.
func runSelectFileArea(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running SELECTFILEAREA", nodeNumber)

	if currentUser == nil {
		log.Printf("WARN: Node %d: SELECTFILEAREA called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to select a file area.|07\r\n"
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// --- Display the list first --- <--- MODIFIED
	if err := displayFileAreaList(e, s, terminal, currentUser, outputMode, nodeNumber, sessionStartTime); err != nil {
		log.Printf("ERROR: Node %d: Failed displaying file area list in SELECTFILEAREA: %v", nodeNumber, err)
		// Don't proceed if the list couldn't be displayed
		return currentUser, "", err // Return error
	}
	// Add a newline between list and prompt
	terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)

	// Prompt for area tag
	prompt := e.LoadedStrings.ChangeFileAreaStr
	if prompt == "" {
		prompt = "|07File Area Tag (?=List, Q=Quit): |15" // Updated prompt slightly
	}
	wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing SELECTFILEAREA prompt: %v", nodeNumber, wErr)
		// Return to menu, maybe signal error?
		return currentUser, "", nil
	}

	inputTag, err := terminal.ReadLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during SELECTFILEAREA prompt.", nodeNumber)
			return nil, "LOGOFF", io.EOF // Signal logoff
		}
		log.Printf("ERROR: Node %d: Error reading input for SELECTFILEAREA: %v", nodeNumber, err)
		return currentUser, "", err // Return error
	}

	inputClean := strings.TrimSpace(inputTag) // Keep original case for tag lookup if needed
	upperInput := strings.ToUpper(inputClean)

	if upperInput == "" || upperInput == "Q" { // Allow Q to quit
		log.Printf("DEBUG: Node %d: SELECTFILEAREA aborted by user.", nodeNumber)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode) // Newline after abort
		return currentUser, "", nil                                          // Return to previous menu
	}

	if upperInput == "?" { // Handle request for list (? loops back here after display)
		log.Printf("DEBUG: Node %d: User requested file area list again from SELECTFILEAREA.", nodeNumber)
		// Simply loop back by returning nil, which will re-run this function
		// which now starts by displaying the list again.
		return currentUser, "", nil
	}

	// --- NEW: Try parsing as ID first, then fallback to Tag ---
	var area *file.FileArea
	var exists bool

	// Try parsing as ID
	if inputID, err := strconv.Atoi(inputClean); err == nil {
		log.Printf("DEBUG: Node %d: User input '%s' parsed as ID %d. Looking up by ID.", nodeNumber, inputClean, inputID)
		area, exists = e.FileMgr.GetAreaByID(inputID)
		if !exists {
			log.Printf("WARN: Node %d: User %s entered non-existent file area ID: %d", nodeNumber, currentUser.Handle, inputID)
			msg := fmt.Sprintf("\r\n|01Error: File area ID '%d' not found.|07\r\n", inputID)
			wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return currentUser, "", nil // Return to menu
		}
	} else {
		// Not a valid ID, treat as Tag (use uppercase)
		log.Printf("DEBUG: Node %d: User input '%s' not an ID. Looking up by Tag '%s'.", nodeNumber, inputClean, upperInput)
		area, exists = e.FileMgr.GetAreaByTag(upperInput)
		if !exists {
			log.Printf("WARN: Node %d: User %s entered non-existent file area tag: %s", nodeNumber, currentUser.Handle, upperInput)
			msg := fmt.Sprintf("\r\n|01Error: File area tag '%s' not found.|07\r\n", upperInput)
			wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return currentUser, "", nil // Return to menu
		}
	}

	// --- END NEW LOGIC ---

	// At this point, 'area' should be valid and 'exists' should be true

	// Check ACSList permission
	if !checkACS(area.ACSList, currentUser, s, terminal, sessionStartTime) {
		log.Printf("WARN: Node %d: User %s denied access to file area %d ('%s') due to ACS '%s'", nodeNumber, currentUser.Handle, area.ID, area.Tag, area.ACSList)
		msg := fmt.Sprintf("\r\n|01Error: Access denied to file area '%s'.|07\r\n", area.Tag)
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return currentUser, "", nil // Return to menu
	}

	// Success! Update user state
	currentUser.CurrentFileAreaID = area.ID
	currentUser.CurrentFileAreaTag = area.Tag
	e.setUserFileConference(currentUser, area.ConferenceID)

	// Save the user state (important!)
	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save user data after updating file area: %v", nodeNumber, err)
		msg := "\r\n|01Error: Could not save area selection.|07\r\n"
		wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		// Don't change area if save failed?
		// Or let it proceed and hope for next save?
		// For now, proceed but log the error.
	}

	log.Printf("INFO: Node %d: User %s changed file area to ID %d ('%s')", nodeNumber, currentUser.Handle, area.ID, area.Tag)
	msg := fmt.Sprintf("\r\n|07Current file area set to: |15%s|07\r\n", area.Name)                  // Use area name for confirmation
	wErr = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode) // <-- Use = instead of :=
	if wErr != nil {                                                                                /* Log? */
	}
	time.Sleep(1 * time.Second)

	return currentUser, "", nil // Success, return to previous menu/state
}

// runSelectMessageArea displays message areas and prompts the user to select one.
func runSelectMessageArea(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running SELECTMSGAREA", nodeNumber)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in to select a message area.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Display areas filtered to current conference
	filterConfID := currentUser.CurrentMsgConferenceID
	if err := displayMessageAreaListFiltered(e, s, terminal, currentUser, outputMode, nodeNumber, sessionStartTime, filterConfID); err != nil {
		return currentUser, "", err
	}
	terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)

	// Prompt for area tag/ID
	prompt := e.LoadedStrings.ChangeBoardStr
	if prompt == "" {
		prompt = "|07Message Area Tag (?=List, Q=Quit): |15"
	}
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	inputTag, err := terminal.ReadLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return currentUser, "", err
	}

	inputClean := strings.TrimSpace(inputTag)
	upperInput := strings.ToUpper(inputClean)

	if upperInput == "" || upperInput == "Q" {
		terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
		return currentUser, "", nil
	}

	if upperInput == "?" {
		return currentUser, "", nil // Re-run will redisplay list
	}

	// Try parsing as ID first, then fallback to Tag
	var area *message.MessageArea
	var exists bool

	if inputID, parseErr := strconv.Atoi(inputClean); parseErr == nil {
		area, exists = e.MessageMgr.GetAreaByID(inputID)
		if !exists {
			msg := fmt.Sprintf("\r\n|01Error: Message area ID '%d' not found.|07\r\n", inputID)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(1 * time.Second)
			return currentUser, "", nil
		}
	} else {
		area, exists = e.MessageMgr.GetAreaByTag(upperInput)
		if !exists {
			msg := fmt.Sprintf("\r\n|01Error: Message area tag '%s' not found.|07\r\n", upperInput)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(1 * time.Second)
			return currentUser, "", nil
		}
	}

	// Check read ACS
	if !checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
		msg := fmt.Sprintf("\r\n|01Error: Access denied to message area '%s'.|07\r\n", area.Tag)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	// Update user state
	currentUser.CurrentMessageAreaID = area.ID
	currentUser.CurrentMessageAreaTag = area.Tag
	e.setUserMsgConference(currentUser, area.ConferenceID)

	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save user data after updating message area: %v", nodeNumber, err)
	}

	log.Printf("INFO: Node %d: User %s changed message area to ID %d ('%s')", nodeNumber, currentUser.Handle, area.ID, area.Tag)
	msg := fmt.Sprintf("\r\n|07Current message area set to: |15%s|07\r\n", area.Name)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(1 * time.Second)

	return currentUser, "", nil
}

// styledInput reads input with character-by-character display styling.
// Mimics Pascal NoCRInput with a shaded cursor cell, solid blue typed area,
// and a bright blue background fill for remaining space.
func styledInput(terminal *term.Terminal, session ssh.Session, outputMode ansi.OutputMode, maxLen int, defaultValue string) (string, error) {
	typedStyle := string(ansi.ReplacePipeCodes([]byte("|B4|15")))
	cursorStyle := string(ansi.ReplacePipeCodes([]byte("|B4|15")))
	remainingStyle := string(ansi.ReplacePipeCodes([]byte("|B12|15")))
	resetColor := "\x1b[0m"

	shadeChar := "\u2591"

	input := make([]byte, 0, maxLen)
	cursorStyleSet := false
	savedCursor := false

	// Function to render the current state of the input box
	renderBox := func(moveBack bool) {
		var display strings.Builder
		if savedCursor {
			display.WriteString("\x1b[u")
		}
		display.WriteString(typedStyle)
		if len(input) > 0 {
			display.Write(input)
		}
		cursorPos := len(input)
		remainingLen := 0
		if len(input) < maxLen {
			display.WriteString(cursorStyle)
			display.WriteString(shadeChar)
			remainingLen = maxLen - len(input) - 1
		}
		if remainingLen > 0 {
			display.WriteString(remainingStyle)
			display.WriteString(strings.Repeat(" ", remainingLen))
		}
		display.WriteString(resetColor)

		moveToCursor := ""
		if cursorPos < maxLen {
			moveToCursor = fmt.Sprintf("\x1b[%dD", maxLen-cursorPos)
		}
		terminalio.WriteStringCP437(terminal, []byte(display.String()+moveToCursor), outputMode)
	}

	// Display initial empty box with cursor and default padding
	if maxLen > 0 {
		terminalio.WriteProcessedBytes(terminal, []byte("\x1b[s"), outputMode)
		savedCursor = true
		terminalio.WriteProcessedBytes(terminal, []byte("\x1b[3 q"), outputMode)
		cursorStyleSet = true
		defer func() {
			if cursorStyleSet {
				terminalio.WriteProcessedBytes(terminal, []byte("\x1b[0 q"), outputMode)
			}
		}()
		renderBox(false)
	}

	// Read character by character from session
	readBuf := make([]byte, 1)

	for {
		n, err := session.Read(readBuf)
		if err != nil {
			if err == io.EOF {
				return "", err
			}
			return "", err
		}
		if n == 0 {
			continue
		}

		ch := readBuf[0]

		switch ch {
		case 13, 10: // Enter or LF
			// User pressed Enter
			result := string(input)
			if result == "" && defaultValue != "" {
				input = append(input[:0], []byte(defaultValue)...)
				if len(input) > maxLen {
					input = input[:maxLen]
				}
				renderBox(true)
				terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
				return defaultValue, nil
			}
			terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
			return strings.TrimSpace(result), nil

		case 8, 127: // Backspace or Delete
			if len(input) > 0 {
				input = input[:len(input)-1]
				renderBox(true)
			}

		case 27: // ESC - clear input
			if len(input) > 0 {
				input = input[:0]
				renderBox(true)
			}

		case 3: // Ctrl+C - abort
			terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
			return "", io.EOF

		default:
			// Printable ASCII character
			if ch >= 32 && ch < 127 && len(input) < maxLen {
				input = append(input, ch)
				renderBox(true)
			}
		}
	}
}

// runSendPrivateMail handles sending private mail to another user.
// It validates the recipient exists and sets the MSG_PRIVATE flag.
func runSendPrivateMail(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running SENDPRIVMAIL", nodeNumber)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in to send private mail.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Get PRIVMAIL area
	privmailArea, exists := e.MessageMgr.GetAreaByTag("PRIVMAIL")
	if !exists {
		log.Printf("ERROR: Node %d: PRIVMAIL area not found", nodeNumber)
		msg := "\r\n|01Error: Private mail area not configured.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Prompt for recipient username
	terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
	recipientPrompt := "|07Send private mail to: |15"
	wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(recipientPrompt)), outputMode)
	if wErr != nil {
		log.Printf("WARN: Node %d: Failed to write recipient prompt: %v", nodeNumber, wErr)
	}

	recipient, err := styledInput(terminal, s, outputMode, 24, "")
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during recipient input.", nodeNumber)
			return nil, "LOGOFF", io.EOF
		}
		log.Printf("ERROR: Node %d: Failed reading recipient input: %v", nodeNumber, err)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nError reading recipient.\r\n"), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		msg := "\r\n|01Recipient cannot be empty.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Validate recipient user exists
	recipientUser, found := userManager.GetUser(recipient)
	if !found || recipientUser == nil {
		msg := fmt.Sprintf("\r\n|01Error: User '%s' not found.|07\r\n", recipient)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Prompt for subject
	titlePrompt := "|07Subject: |15"
	wErr = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(titlePrompt)), outputMode)
	if wErr != nil {
		log.Printf("WARN: Node %d: Failed to write subject prompt: %v", nodeNumber, wErr)
	}

	subject, err := styledInput(terminal, s, outputMode, 30, "")
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during subject input.", nodeNumber)
			return nil, "LOGOFF", io.EOF
		}
		log.Printf("ERROR: Node %d: Failed reading subject input: %v", nodeNumber, err)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nError reading subject.\r\n"), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = "(no subject)"
	}

	// Launch editor
	log.Printf("DEBUG: Node %d: Clearing screen before calling editor for private mail", nodeNumber)
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)

	// Launch editor for private mail (no anonymous option for private mail)
	body, saved, err := editor.RunEditorWithMetadata("", s, s, outputMode, subject, recipientUser.Handle, false, "", "", "", "", false, nil)
	log.Printf("DEBUG: Node %d: editor.RunEditorWithMetadata returned. Error: %v, Saved: %v, Body length: %d", nodeNumber, err, saved, len(body))

	if err != nil {
		log.Printf("ERROR: Node %d: Editor failed for user %s: %v", nodeNumber, currentUser.Handle, err)
		return nil, "", fmt.Errorf("editor error: %w", err)
	}

	// Clear screen after editor exits
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)

	if !saved {
		log.Printf("INFO: Node %d: User %s aborted private mail composition.", nodeNumber, currentUser.Handle)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nMessage aborted.\r\n"), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	if strings.TrimSpace(body) == "" {
		log.Printf("INFO: Node %d: User %s saved empty private mail.", nodeNumber, currentUser.Handle)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nMessage body empty. Aborting.\r\n"), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Save the private message with MSG_PRIVATE flag
	msgNum, err := e.MessageMgr.AddPrivateMessage(privmailArea.ID, currentUser.Handle, recipientUser.Handle, subject, body, "")
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to save private message from user %s to %s: %v", nodeNumber, currentUser.Handle, recipientUser.Handle, err)
		errorMsg := ansi.ReplacePipeCodes([]byte("\r\n|01Error saving private message!|07\r\n"))
		terminalio.WriteProcessedBytes(terminal, errorMsg, outputMode)
		time.Sleep(2 * time.Second)
		return nil, "", fmt.Errorf("failed saving private message: %w", err)
	}

	// Update user message counter
	currentUser.MessagesPosted++
	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to update MessagesPosted for user %s: %v", nodeNumber, currentUser.Handle, err)
	}

	// Confirmation
	log.Printf("INFO: Node %d: User %s successfully sent private message #%d to %s", nodeNumber, currentUser.Handle, msgNum, recipientUser.Handle)
	confirmMsg := ansi.ReplacePipeCodes([]byte(fmt.Sprintf("\r\n|02Private message sent to %s!|07\r\n", recipientUser.Handle)))
	terminalio.WriteProcessedBytes(terminal, confirmMsg, outputMode)
	time.Sleep(1 * time.Second)

	return nil, "", nil
}

// runReadPrivateMail handles reading private mail for the current user.
// It filters messages to only show those addressed to the current user with MSG_PRIVATE flag.
func runReadPrivateMail(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running READPRIVMAIL", nodeNumber)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in to read private mail.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Get PRIVMAIL area
	privmailArea, exists := e.MessageMgr.GetAreaByTag("PRIVMAIL")
	if !exists {
		log.Printf("ERROR: Node %d: PRIVMAIL area not found", nodeNumber)
		msg := "\r\n|01Error: Private mail area not configured.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Get JAM base for PRIVMAIL area
	base, err := e.MessageMgr.GetBase(privmailArea.ID)
	if err != nil {
		log.Printf("ERROR: Node %d: JAM base not open for PRIVMAIL area: %v", nodeNumber, err)
		msg := "\r\n|01Error: Private mail base not available.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Get total message count
	totalMessages, err := e.MessageMgr.GetMessageCountForArea(privmailArea.ID)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to get message count for PRIVMAIL: %v", nodeNumber, err)
		msg := "\r\n|01Error loading private mail.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", err
	}

	if totalMessages == 0 {
		msg := "\r\n|07No private mail found.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Scan all messages and filter for private messages addressed to current user
	// CRITICAL SECURITY: Must check BOTH IsPrivate() AND To field matches current user
	privateMessages := []int{}
	for msgNum := 1; msgNum <= totalMessages; msgNum++ {
		msg, err := base.ReadMessage(msgNum)
		if err != nil {
			log.Printf("WARN: Node %d: Failed to read message #%d in PRIVMAIL: %v", nodeNumber, msgNum, err)
			continue
		}

		// Skip deleted messages
		if msg.IsDeleted() {
			continue
		}

		// Check if message is private AND addressed to current user (case-insensitive)
		if msg.IsPrivate() && strings.EqualFold(msg.To, currentUser.Handle) {
			privateMessages = append(privateMessages, msgNum)
		}
	}

	if len(privateMessages) == 0 {
		msg := "\r\n|07No private mail found for you.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Display count and read messages using the message reader
	confirmMsg := fmt.Sprintf("\r\n|02Found %d private message(s) for you.|07\r\n", len(privateMessages))
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(confirmMsg)), outputMode)
	time.Sleep(500 * time.Millisecond)

	// Temporarily set current area to PRIVMAIL for the message reader
	originalAreaID := currentUser.CurrentMessageAreaID
	originalAreaTag := currentUser.CurrentMessageAreaTag
	currentUser.CurrentMessageAreaID = privmailArea.ID
	currentUser.CurrentMessageAreaTag = privmailArea.Tag

	// Start reading from the first private message
	startMsgNum := privateMessages[0]

	// Call message reader with the filtered list
	updatedUser, nextMenu, err := runMessageReader(e, s, terminal, userManager, currentUser, nodeNumber,
		sessionStartTime, outputMode, startMsgNum, totalMessages, false)

	// Restore original area
	if updatedUser != nil {
		updatedUser.CurrentMessageAreaID = originalAreaID
		updatedUser.CurrentMessageAreaTag = originalAreaTag
	} else if currentUser != nil {
		currentUser.CurrentMessageAreaID = originalAreaID
		currentUser.CurrentMessageAreaTag = originalAreaTag
	}

	return updatedUser, nextMenu, err
}

// runListPrivateMail handles listing private mail for the current user.
// It temporarily switches to the PRIVMAIL area and calls the standard list function.
func runListPrivateMail(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running LISTPRIVMAIL", nodeNumber)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in to list private mail.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Get PRIVMAIL area
	privmailArea, exists := e.MessageMgr.GetAreaByTag("PRIVMAIL")
	if !exists {
		log.Printf("ERROR: Node %d: PRIVMAIL area not found", nodeNumber)
		msg := "\r\n|01Error: Private mail area not configured.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Temporarily set current area to PRIVMAIL
	originalAreaID := currentUser.CurrentMessageAreaID
	originalAreaTag := currentUser.CurrentMessageAreaTag
	currentUser.CurrentMessageAreaID = privmailArea.ID
	currentUser.CurrentMessageAreaTag = privmailArea.Tag

	// Call standard list function
	updatedUser, nextMenu, err := runListMsgs(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode)

	// Restore original area
	if updatedUser != nil {
		updatedUser.CurrentMessageAreaID = originalAreaID
		updatedUser.CurrentMessageAreaTag = originalAreaTag
	} else if currentUser != nil {
		currentUser.CurrentMessageAreaID = originalAreaID
		currentUser.CurrentMessageAreaTag = originalAreaTag
	}

	return updatedUser, nextMenu, err
}
