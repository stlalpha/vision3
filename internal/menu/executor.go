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
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"golang.org/x/term"
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/editor"
	"github.com/stlalpha/vision3/internal/file"
	"github.com/stlalpha/vision3/internal/message"
	terminalPkg "github.com/stlalpha/vision3/internal/terminal"
	"github.com/stlalpha/vision3/internal/transfer"
	"github.com/stlalpha/vision3/internal/types"
	"github.com/stlalpha/vision3/internal/user"
)

// Mutex for protecting access to the oneliners file
var onelinerMutex sync.Mutex

// RunnableFunc defines the signature for functions executable via RUN:
// Returns: authenticatedUser, nextAction (e.g., "GOTO:MENU"), err
type RunnableFunc func(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (authenticatedUser *user.User, nextAction string, err error)

// AutoRunTracker definition removed, using the one from types.go

// MenuExecutor handles the loading and execution of ViSiON/2 menus.
type MenuExecutor struct {
	ConfigPath     string                       // DEPRECATED: Use MenuSetPath + "/cfg" or RootConfigPath
	AssetsPath     string                       // DEPRECATED: Use MenuSetPath + "/ansi" or RootAssetsPath
	MenuSetPath    string                       // NEW: Path to the active menu set (e.g., "menus/v3")
	RootConfigPath string                       // NEW: Path to global configs (e.g., "configs")
	RootAssetsPath string                       // NEW: Path to global assets (e.g., "assets")
	RunRegistry    map[string]RunnableFunc      // Map RUN: targets to functions (Use local RunnableFunc)
	DoorRegistry   map[string]config.DoorConfig // Map DOOR: targets to configurations
	OneLiners      []string                     // Loaded oneliners (Consider if these should be menu-set specific)
	LoadedStrings  config.StringsConfig         // Loaded global strings configuration
	Theme          config.ThemeConfig           // Loaded theme configuration
	MessageMgr     *message.MessageManager      // <-- ADDED FIELD
	FileMgr        *file.FileManager            // <-- ADDED FIELD: File manager instance
}

// NewExecutor creates a new MenuExecutor.
// Added oneLiners, loadedStrings, theme, messageMgr, and fileMgr parameters
// Updated paths to use new structure
// << UPDATED Signature with msgMgr and fileMgr
func NewExecutor(menuSetPath, rootConfigPath, rootAssetsPath string, oneLiners []string, doorRegistry map[string]config.DoorConfig, loadedStrings config.StringsConfig, theme config.ThemeConfig, msgMgr *message.MessageManager, fileMgr *file.FileManager) *MenuExecutor {

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
		OneLiners:      oneLiners,     // Store loaded oneliners
		LoadedStrings:  loadedStrings, // Store loaded strings
		Theme:          theme,         // Store loaded theme
		MessageMgr:     msgMgr,        // <-- ASSIGN FIELD
		FileMgr:        fileMgr,       // <-- ASSIGN FIELD
	}
}

// registerPlaceholderRunnables adds dummy functions for testing
func registerPlaceholderRunnables(registry map[string]RunnableFunc) { // Use local RunnableFunc
	// Keep READMAIL as a placeholder for now
	registry["READMAIL"] = func(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
		if currentUser == nil {
			log.Printf("WARN: Node %d: READMAIL called without logged in user.", nodeNumber)
			msg := "\r\n|01Error: You must be logged in to read mail.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil {
				log.Printf("ERROR: Failed writing READMAIL error message: %v", wErr)
			}
			time.Sleep(1 * time.Second)
			return nil, "", nil // No user change, no next action, no error
		}
		msg := fmt.Sprintf("\r\n|15Executing |11READMAIL|15 for |14%s|15... (Not Implemented)|07\r\n", currentUser.Handle)
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil {
			log.Printf("ERROR: Failed writing READMAIL placeholder message: %v", wErr)
		}
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil // No user change, no next action, no error
	}

	// Register DOOR handler
	registry["DOOR:"] = func(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, doorName string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
		if currentUser == nil {
			log.Printf("WARN: Node %d: DOOR:%s called without logged in user.", nodeNumber, doorName)
			msg := "\r\n|01Error: You must be logged in to run doors.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil {
				log.Printf("ERROR: Failed writing DOOR error message (not logged in): %v", wErr)
			}
			return nil, "", nil // Stay in menu loop
		}
		log.Printf("INFO: Node %d: User %s attempting to run door: %s", nodeNumber, currentUser.Handle, doorName)

		// 1. Look up door configuration
		doorConfig, exists := e.DoorRegistry[strings.ToUpper(doorName)] // Ensure lookup is case-insensitive
		if !exists {
			log.Printf("WARN: Door configuration not found for '%s'", doorName)
			errMsg := fmt.Sprintf("\r\n|12Error: Door '%s' is not configured.\r\nPress Enter to continue...\r\n", doorName)
			// Use WriteProcessedBytes for error message, writing to stderr
			// Need outputMode... assume CP437 for errors?
			wErr := terminal.DisplayContent([]byte(errMsg))
			if wErr != nil {
				log.Printf("ERROR: Failed writing DOOR error message (not configured) to stderr: %v", wErr)
			}
			// Wait for Enter? For now, let the menu loop handle redraw.
			return nil, "", nil // Stay in menu loop
		}

		// Gather data for substitution
		// Use the assigned node number
		nodeNumStr := strconv.Itoa(nodeNumber)
		// Set {PORT} based on the conceptual Node, as COM ports aren't directly applicable.
		portStr := nodeNumStr
		/* // Keep remote port extraction logic commented out for reference if needed later
		// Attempt to get remote port
		portStr := "?" // Default if extraction fails
		if remoteAddr := s.RemoteAddr(); remoteAddr != nil {
			addrStr := remoteAddr.String()
			if parts := strings.Split(addrStr, ":"); len(parts) > 1 {
				portStr = parts[len(parts)-1] // Get the last part as port
			}
		}
		*/
		// Calculate actual Time Left
		elapsedMinutes := int(time.Since(sessionStartTime).Minutes())
		remainingMinutes := currentUser.TimeLimit - elapsedMinutes
		if remainingMinutes < 0 {
			remainingMinutes = 0 // Prevent negative time left
		}
		timeLeftStr := strconv.Itoa(remainingMinutes)
		// TODO: Get actual Baud Rate - using placeholder
		baudStr := "38400"
		userIDStr := strconv.Itoa(currentUser.ID)

		substitutions := map[string]string{
			"{NODE}":       nodeNumStr,
			"{PORT}":       portStr,
			"{TIMELEFT}":   timeLeftStr,
			"{BAUD}":       baudStr,
			"{USERHANDLE}": currentUser.Handle,
			"{USERID}":     userIDStr,
			// Add more common placeholders if needed
			"{REALNAME}": currentUser.RealName,
			"{LEVEL}":    strconv.Itoa(currentUser.AccessLevel),
		}

		// Substitute in Arguments
		substitutedArgs := make([]string, len(doorConfig.Args))
		for i, arg := range doorConfig.Args {
			newArg := arg
			for key, val := range substitutions {
				newArg = strings.ReplaceAll(newArg, key, val)
			}
			substitutedArgs[i] = newArg
		}

		// Substitute in Environment Variables
		substitutedEnv := make(map[string]string)
		if doorConfig.EnvironmentVars != nil {
			for key, val := range doorConfig.EnvironmentVars {
				newVal := val
				for subKey, subVal := range substitutions {
					newVal = strings.ReplaceAll(newVal, subKey, subVal)
				}
				substitutedEnv[key] = newVal
			}
		}
		// --- End Placeholder Substitution ---

		// --- Dropfile Generation ---
		var dropfilePath string
		dropfileDir := "." // Default to current dir if no WorkingDirectory
		if doorConfig.WorkingDirectory != "" {
			dropfileDir = doorConfig.WorkingDirectory
		}

		dropfileTypeUpper := strings.ToUpper(doorConfig.DropfileType)

		if dropfileTypeUpper == "DOOR.SYS" || dropfileTypeUpper == "CHAIN.TXT" {
			dropfileName := dropfileTypeUpper // Use the standardized uppercase name
			dropfilePath = filepath.Join(dropfileDir, dropfileName)
			log.Printf("INFO: Generating %s dropfile at: %s", dropfileName, dropfilePath)

			var content strings.Builder
			if dropfileTypeUpper == "DOOR.SYS" {
				// Simplified Text-based DOOR.SYS
				// TODO: Use actual COM/Port info if available
				content.WriteString(fmt.Sprintf("%s\r\n", "1"))                     // COM Port (Placeholder)
				content.WriteString(fmt.Sprintf("%s\r\n", baudStr))                 // Baud Rate
				content.WriteString(fmt.Sprintf("%s\r\n", "N"))                     // Parity (Placeholder)
				content.WriteString(fmt.Sprintf("%s\r\n", nodeNumStr))              // Node Number
				content.WriteString(fmt.Sprintf("%s\r\n", currentUser.Handle))      // User Name
				content.WriteString(fmt.Sprintf("%d\r\n", currentUser.AccessLevel)) // Security Level
				content.WriteString(fmt.Sprintf("%s\r\n", timeLeftStr))             // Time Left
				content.WriteString(fmt.Sprintf("%s\r\n", "ANSI"))                  // Emulation (Placeholder)
				content.WriteString(fmt.Sprintf("%s\r\n", userIDStr))               // User ID
			} else { // CHAIN.TXT
				content.WriteString(fmt.Sprintf("%s\r\n", currentUser.Handle))
				content.WriteString(fmt.Sprintf("%d\r\n", currentUser.AccessLevel))
				content.WriteString(fmt.Sprintf("%s\r\n", timeLeftStr))
				content.WriteString(fmt.Sprintf("%s\r\n", userIDStr))
			}

			err := os.WriteFile(dropfilePath, []byte(content.String()), 0644)
			if err != nil {
				log.Printf("ERROR: Failed to write dropfile %s: %v", dropfilePath, err)
				errMsg := fmt.Sprintf("\r\n|12Error creating system file for door '%s'.\r\nPress Enter to continue...\r\n", doorName)
				terminal.DisplayContent([]byte(errMsg))
				return nil, "", nil                                                   // Abort door execution
			}

			// Ensure dropfile is cleaned up
			defer func() {
				log.Printf("DEBUG: Cleaning up dropfile: %s", dropfilePath)
				if err := os.Remove(dropfilePath); err != nil {
					log.Printf("WARN: Failed to remove dropfile %s: %v", dropfilePath, err)
				}
			}()
		}
		// --- End Dropfile Generation ---

		// 2. Prepare the command (shared part)
		cmd := exec.Command(doorConfig.Command, substitutedArgs...)

		// 3. Set working directory if specified
		if doorConfig.WorkingDirectory != "" {
			cmd.Dir = doorConfig.WorkingDirectory
			log.Printf("DEBUG: Setting working directory for door '%s' to '%s'", doorName, cmd.Dir)
		}

		// 4. Set environment variables
		// Start with current environment, then add/override from config
		cmd.Env = os.Environ()
		if len(substitutedEnv) > 0 {
			for key, val := range substitutedEnv {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
			}
		}
		// Optionally add standard BBS env vars if not present (e.g., BBS_NODE)
		// Add our substituted standard vars if they weren't in the config
		envMap := make(map[string]bool)
		for _, envPair := range cmd.Env {
			envMap[strings.SplitN(envPair, "=", 2)[0]] = true
		}
		if _, exists := envMap["BBS_USERHANDLE"]; !exists {
			cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_USERHANDLE=%s", currentUser.Handle))
		}
		if _, exists := envMap["BBS_USERID"]; !exists {
			cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_USERID=%s", userIDStr))
		}
		if _, exists := envMap["BBS_NODE"]; !exists {
			cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_NODE=%s", nodeNumStr))
		}
		if _, exists := envMap["BBS_TIMELEFT"]; !exists {
			cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_TIMELEFT=%s", timeLeftStr))
		}
		// Add others like BBS_LEVEL, BBS_PORT, BBS_BAUD?

		// --- Execute Command (Handles PTY/Raw mode) ---
		ptyReq, winChOrig, isPty := s.Pty() // Get original PTY request and window change channel
		var cmdErr error

		if doorConfig.RequiresRawTerminal && isPty {
			// --- PTY / Raw Mode Execution --- Reverted to original logic ---
			log.Printf("INFO: Node %d: Starting door '%s' with PTY/Raw mode", nodeNumber, doorName)
			ptmx, err := pty.Start(cmd)
			if err != nil {
				cmdErr = fmt.Errorf("failed to start pty for door '%s': %w", doorName, err)
			} else {
				defer func() { _ = ptmx.Close() }()

				s.Signals(nil)
				s.Break(nil)
				go func() {
					if ptyReq.Window.Width > 0 || ptyReq.Window.Height > 0 {
						pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(ptyReq.Window.Height), Cols: uint16(ptyReq.Window.Width)})
					}
					// Use the ORIGINAL winChOrig from s.Pty()
					for win := range winChOrig {
						pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(win.Height), Cols: uint16(win.Width)})
					}
				}()

				// Set terminal to raw mode using PTY fd
				fd := int(ptmx.Fd()) // Use the PTY file descriptor
				var restoreTerminal func()
				originalState, err := term.MakeRaw(fd)
				if err != nil {
					log.Printf("WARN: Node %d: Failed to put PTY into raw mode for door '%s': %v.", nodeNumber, doorName, err)
				} else {
					log.Printf("DEBUG: Node %d: PTY set to raw mode for door '%s'.", nodeNumber, doorName)
					restoreTerminal = func() {
						log.Printf("DEBUG: Node %d: Restoring PTY mode after door '%s'.", nodeNumber, doorName)
						if err := term.Restore(fd, originalState); err != nil {
							log.Printf("ERROR: Node %d: Failed to restore PTY state after door '%s': %v", nodeNumber, doorName, err)
						}
					}
					defer restoreTerminal()
				}

				// Set up I/O copying using goroutines
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					_, err := io.Copy(ptmx, s)
					if err != nil && err != io.EOF && !errors.Is(err, os.ErrClosed) {
						log.Printf("WARN: Node %d: Error copying session stdin to PTY for door '%s': %v", nodeNumber, doorName, err)
					}
					log.Printf("DEBUG: Node %d: Finished copying session stdin to PTY for door '%s'", nodeNumber, doorName)
				}()
				go func() {
					defer wg.Done()
					_, err := io.Copy(s, ptmx)
					if err != nil && err != io.EOF && !errors.Is(err, os.ErrClosed) {
						log.Printf("WARN: Node %d: Error copying PTY stdout to session stdout for door '%s': %v", nodeNumber, doorName, err)
					}
					log.Printf("DEBUG: Node %d: Finished copying PTY stdout to session stdout for door '%s'", nodeNumber, doorName)
				}()

				wg.Wait()
				log.Printf("DEBUG: Node %d: I/O copying finished for door '%s'. Waiting for command completion.", nodeNumber, doorName)
				cmdErr = cmd.Wait()
			}
		} else {
			// --- Standard I/O Execution ---
			if doorConfig.RequiresRawTerminal && !isPty {
				log.Printf("WARN: Node %d: Door '%s' requires raw terminal, but no PTY was allocated. Door may not function correctly.", nodeNumber, doorName)
			}
			log.Printf("INFO: Node %d: Starting door '%s' with standard I/O redirection", nodeNumber, doorName)
			cmd.Stdout = s
			cmd.Stderr = s
			cmd.Stdin = s
			cmdErr = cmd.Run()
		}
		// --- End Command Execution ---

		// 7. Handle result (using cmdErr)
		if cmdErr != nil {
			log.Printf("ERROR: Node %d: Door command execution failed for user %s, door %s: %v", nodeNumber, currentUser.Handle, doorName, cmdErr)
			// Send error message to user terminal
			errMsg := fmt.Sprintf("\r\n|12Error running external program '%s': %v\r\nPress Enter to continue...\r\n", doorName, cmdErr)
			terminal.DisplayContent([]byte(errMsg)) // Display error message
			// Wait for Enter? Not needed as Run/Wait blocks.
		} else {
			log.Printf("INFO: Node %d: Door command completed for user %s, door %s", nodeNumber, currentUser.Handle, doorName)
			// Optionally clear screen or show a returning message?
			// fmt.Fprintf(s, "\r\n|07Returning to menu...\r\n")
			// Need a slight pause or prompt, otherwise menu redraws instantly over door output
			terminal.DisplayContent([]byte("\r\n|07Press |15[ENTER]|07 to return to the menu... ")) // Display prompt message
			// Wait for Enter key press using the session reader
			bufioReader := bufio.NewReader(s)
			for {
				r, _, readErr := bufioReader.ReadRune()
				if readErr != nil {
					log.Printf("ERROR: Failed reading input after door '%s': %v", doorName, readErr)
					break // Exit loop on error (e.g., disconnect)
				}
				if r == '\r' || r == '\n' { // Check for Enter (CR or LF)
					break
				}
				// Ignore other characters
			}
		}

		// --- Cleanup Dropfiles --- (Handled by defer)

		return nil, "", nil // Return nil user, "", nil error to continue menu loop after door
	}
}

// registerAppRunnables registers the actual application command functions.
func registerAppRunnables(registry map[string]RunnableFunc) { // Use local RunnableFunc
	registry["SHOWSTATS"] = runShowStats
	registry["LASTCALLERS"] = runLastCallers
	registry["AUTHENTICATE"] = runAuthenticate
	registry["ONELINER"] = runOneliners                              // Register new placeholder
	registry["FULL_LOGIN_SEQUENCE"] = runFullLoginSequence           // Register the new sequence
	registry["SHOWVERSION"] = runShowVersion                         // Register the version display runnable
	registry["LISTUSERS"] = runListUsers                             // Register the user list runnable
	registry["LISTMSGAR"] = runListMessageAreas                      // <-- ADDED: Register message area list runnable
	registry["COMPOSEMSG"] = runComposeMessage                       // <-- ADDED: Register compose message runnable
	registry["PROMPTANDCOMPOSEMESSAGE"] = runPromptAndComposeMessage // <-- ADDED: Register prompt/compose runnable (Corrected key to uppercase)
	registry["READMSGS"] = runReadMsgs                               // <-- ADDED: Register message reading runnable
	registry["NEWSCAN"] = runNewscan                                 // <-- ADDED: Register newscan runnable
	registry["LISTFILES"] = runListFiles                             // <-- ADDED: Register file list runnable
	registry["LISTFILEAR"] = runListFileAreas                        // <-- ADDED: Register file area list runnable
	registry["SELECTFILEAREA"] = runSelectFileArea                   // <-- ADDED: Register file area selection runnable
}

// runShowStats displays the user statistics screen (YOURSTAT.ANS).
func runShowStats(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		log.Printf("WARN: Node %d: SHOWSTATS called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to view stats.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil {
			log.Printf("ERROR: Failed writing SHOWSTATS error message: %v", wErr)
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Updated return
	}

	ansFilename := "YOURSTAT.ANS"
	// Use MenuSetPath for ANSI file
	fullAnsPath := filepath.Join(e.MenuSetPath, "ansi", ansFilename)
	rawAnsiContent, readErr := os.ReadFile(fullAnsPath)
	if readErr != nil {
		log.Printf("ERROR: Node %d: Failed to read %s for SHOWSTATS: %v", nodeNumber, fullAnsPath, readErr)
		msg := fmt.Sprintf("\r\n|01Error displaying stats screen (%s).|07\r\n", ansFilename)
		wErr := terminal.DisplayContent([]byte(msg))
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
	wErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
	if wErr != nil {
		// Log error but continue if possible
		log.Printf("ERROR: Node %d: Failed clearing screen for SHOWSTATS: %v", nodeNumber, wErr)
	}

	// Log hex bytes before writing
	statsDisplayBytes := []byte(substitutedContent)
	// log.Printf("DEBUG: Node %d: Writing SHOWSTATS content bytes (hex): %x", nodeNumber, statsDisplayBytes)
	_, wErr = terminal.Write(statsDisplayBytes)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing processed YOURSTAT.ANS: %v", nodeNumber, wErr)
		return nil, "", wErr // Updated return
	}

	// 5. Wait for Enter key press
	pausePrompt := e.LoadedStrings.PauseString // Use configured pause string
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... " // Fallback
	}

	// Display pause prompt directly - DisplayContent handles encoding and pipe codes
	log.Printf("DEBUG: Node %d: Displaying SHOWSTATS pause prompt: %q", nodeNumber, pausePrompt)
	wErr = terminal.DisplayContent([]byte(pausePrompt))
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

// runOneliners displays the oneliners using templates.
func runOneliners(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running ONELINER", nodeNumber)

	// --- Load current oneliners dynamically ---
	onelinerPath := filepath.Join("data", "oneliners.json")
	var currentOneLiners []string

	// --- BEGIN MUTEX PROTECTED SECTION ---
	onelinerMutex.Lock()
	log.Printf("DEBUG: Node %d: Acquired oneliner mutex.", nodeNumber)
	defer func() {
		onelinerMutex.Unlock()
		log.Printf("DEBUG: Node %d: Released oneliner mutex.", nodeNumber)
	}()

	jsonData, readErr := os.ReadFile(onelinerPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			log.Printf("INFO: %s not found, starting with empty list.", onelinerPath)
			currentOneLiners = []string{}
		} else {
			log.Printf("ERROR: Failed to read oneliners file %s: %v", onelinerPath, readErr)
			currentOneLiners = []string{}
		}
	} else {
		err := json.Unmarshal(jsonData, &currentOneLiners)
		if err != nil {
			log.Printf("ERROR: Failed to parse oneliners JSON from %s: %v. Starting with empty list.", onelinerPath, err)
			currentOneLiners = []string{}
		}
	}
	log.Printf("DEBUG: Loaded %d oneliners from %s", len(currentOneLiners), onelinerPath)

	// --- Load Templates ---
	topTemplatePath := filepath.Join(e.MenuSetPath, "templates", "ONELINER.TOP")
	midTemplatePath := filepath.Join(e.MenuSetPath, "templates", "ONELINER.MID")
	botTemplatePath := filepath.Join(e.MenuSetPath, "templates", "ONELINER.BOT")

	topTemplate, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplate, errBot := os.ReadFile(botTemplatePath)

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load one or more ONELINER template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading Oneliners screen templates.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading ONELINER templates")
	}

	// --- Process Templates ---
	processedTopTemplate := string(terminal.ProcessPipeCodes(topTemplate))
	processedMidTemplate := string(terminal.ProcessPipeCodes(midTemplateBytes))
	processedBotTemplate := string(terminal.ProcessPipeCodes(botTemplate))

	// Use WriteProcessedBytes for ClearScreen
	wErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed clearing screen for ONELINER: %v", nodeNumber, wErr)
		// Continue if possible
	}
	// Log hex bytes before writing
	log.Printf("DEBUG: Node %d: Writing ONELINER top template bytes (hex): %x", nodeNumber, []byte(processedTopTemplate))
	_, wErr = terminal.Write([]byte(processedTopTemplate))
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing ONELINER top template: %v", nodeNumber, wErr)
		return nil, "", wErr
	}

	// --- Display Last 20 (or fewer) Oneliners --- REMOVED Pagination Logic
	numLiners := len(currentOneLiners)
	maxLinesToShow := 20
	startIdx := 0
	if numLiners > maxLinesToShow {
		startIdx = numLiners - maxLinesToShow
	}

	for i := startIdx; i < numLiners; i++ {
		oneliner := currentOneLiners[i]
		
		line := processedMidTemplate
		line = strings.ReplaceAll(line, "^OL", oneliner)

		// Log hex bytes before writing
		lineBytes := []byte(line)
		log.Printf("DEBUG: Node %d: Writing ONELINER mid line %d bytes (hex): %x", nodeNumber, i, lineBytes)
		_, wErr = terminal.Write(lineBytes)
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing oneliner line %d: %v", nodeNumber, i, wErr)
			return nil, "", wErr
		}
	}

	_, wErr = terminal.Write([]byte(processedBotTemplate))
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing ONELINER bottom template: %v", nodeNumber, wErr)
		return nil, "", wErr
	}
	// Log hex bytes before writing
	log.Printf("DEBUG: Node %d: Writing ONELINER bot template bytes (hex): %x", nodeNumber, processedBotTemplate)
	_, wErr = terminal.Write([]byte(processedBotTemplate))
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing ONELINER bottom template: %v", nodeNumber, wErr)
		return nil, "", wErr
	}

	// --- Ask to Add New One --- (Logic remains the same)
	askPrompt := e.LoadedStrings.AskOneLiner
	if askPrompt == "" {
		log.Fatalf("CRITICAL: Required string 'AskOneLiner' is missing or empty in strings configuration.")
	}

	log.Printf("DEBUG: Node %d: Calling promptYesNoLightbar for ONELINER add prompt", nodeNumber)
	addYes, err := e.promptYesNoLightbar(s, terminal, askPrompt, outputMode, nodeNumber) // Pass nodeNumber
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during ONELINER add prompt.", nodeNumber)
			return nil, "LOGOFF", io.EOF
		}
		log.Printf("ERROR: Failed getting Yes/No input for ONELINER add: %v", err)
		return nil, "", err
	}

	if addYes {
		enterPrompt := e.LoadedStrings.EnterOneLiner
		if enterPrompt == "" {
			log.Fatalf("CRITICAL: Required string 'EnterOneLiner' is missing or empty in strings configuration.")
		}
		// Save cursor position
		wErr = terminal.SaveCursor()
		if wErr != nil { /* Log? */
		}
		posClearCmd := fmt.Sprintf("\x1b[%d;1H\x1b[2K", 23) // Use row 23 for input prompt
		_, wErr = terminal.Write([]byte(posClearCmd))
		if wErr != nil { /* Log? */
		}

		// Log hex bytes before writing
		enterPromptBytes := terminal.ProcessPipeCodes([]byte(enterPrompt))
		log.Printf("DEBUG: Node %d: Writing ONELINER enter prompt bytes (hex): %x", nodeNumber, enterPromptBytes)
		_, wErr = terminal.Write(enterPromptBytes)
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
		newOneliner = strings.TrimSpace(newOneliner)

		if newOneliner != "" {
			currentOneLiners = append(currentOneLiners, newOneliner)
			log.Printf("DEBUG: Node %d: Appended oneliner to local list: '%s'", nodeNumber, newOneliner)

			updatedJsonData, err := json.MarshalIndent(currentOneLiners, "", "  ")
			if err != nil {
				log.Printf("ERROR: Node %d: Failed to marshal updated oneliners list to JSON: %v", nodeNumber, err)
				msg := "\r\n|01Error preparing oneliner data for saving.|07\r\n"
				terminal.DisplayContent([]byte(msg))
			} else {
				err = os.WriteFile(onelinerPath, updatedJsonData, 0644)
				if err != nil {
					log.Printf("ERROR: Node %d: Failed to write updated oneliners JSON to %s: %v", nodeNumber, onelinerPath, err)
					msg := "\r\n|01Error writing oneliner to disk.|07\r\n"
					terminal.DisplayContent([]byte(msg))
				} else {
					log.Printf("INFO: Node %d: Successfully saved updated oneliners to %s", nodeNumber, onelinerPath)
					msg := "\r\n|10Oneliner added!|07\r\n"
					terminal.DisplayContent([]byte(msg))
					time.Sleep(500 * time.Millisecond)
				}
			}
		} else {
			msg := "\r\n|01Empty oneliner not added.|07\r\n"
			terminal.DisplayContent([]byte(msg))
			time.Sleep(500 * time.Millisecond)
		}
	} // end if addYes

	return nil, "", nil
}

// runAuthenticate handles the RUN:AUTHENTICATE command.
// Update signature to return three values
func runAuthenticate(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	// If already logged in, maybe show an error or just return?
	if currentUser != nil {
		log.Printf("WARN: Node %d: User %s tried to run AUTHENTICATE while already logged in.", nodeNumber, currentUser.Handle)
		msg := "\r\n|01You are already logged in.|07\r\n"
		// Use WriteProcessedBytes
		wErr := terminal.DisplayContent([]byte(msg))
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
	terminal.Write([]byte(terminalPkg.MoveCursor(userRow, userCol)))
	usernamePrompt := "|07Username/Handle: |15" // Original prompt text was in ANSI
	// Use WriteProcessedBytes for prompt
	// Use DisplayContent to handle pipe codes and display
	wErr := terminal.DisplayContent([]byte(usernamePrompt))
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

	// Move to Password position, display prompt, and read input securely
	terminal.Write([]byte(terminalPkg.MoveCursor(passRow, passCol)))
	passwordPrompt := "|07Password: |15" // Original prompt text was in ANSI
	terminal.DisplayContent([]byte(passwordPrompt))
	password, err := readPasswordSecurely(s, terminal)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during password input.", nodeNumber)
			return nil, "LOGOFF", io.EOF // Signal logoff
		}
		if err.Error() == "password entry interrupted" { // Check for Ctrl+C
			log.Printf("INFO: Node %d: User interrupted password entry.", nodeNumber)
			// Treat interrupt like a failed attempt?
			terminal.Write([]byte(terminalPkg.MoveCursor(errorRow, 1))) // Move cursor for message
			msg := "\r\n|01Login cancelled.|07\r\n"
			terminal.DisplayContent([]byte(msg))
			time.Sleep(500 * time.Millisecond)
			return nil, "", nil // No user change, no critical error
		}
		log.Printf("ERROR: Node %d: Failed to read password securely: %v", nodeNumber, err)
		return nil, "", fmt.Errorf("failed reading password: %w", err) // Critical error
	}

	// Attempt Authentication via UserManager
	log.Printf("DEBUG: Node %d: Attempting authentication for user: %s", nodeNumber, username)
	authUser, authenticated := userManager.Authenticate(username, password)
	if !authenticated {
		log.Printf("WARN: Node %d: Failed authentication attempt for user: %s", nodeNumber, username)
		// Display error message to user
		terminal.Write([]byte(terminalPkg.MoveCursor(errorRow, 1))) // Move cursor for message
		errMsg := "\r\n|01Login incorrect.|07\r\n"
		// Use WriteProcessedBytes
		wErr := terminal.DisplayContent([]byte(errMsg))
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
		terminal.Write([]byte(terminalPkg.MoveCursor(errorRow, 1))) // Move cursor for message
		errMsg := "\r\n|01Account requires validation by SysOp.|07\r\n"
		// Use WriteProcessedBytes
		wErr := terminal.DisplayContent([]byte(errMsg))
		if wErr != nil {
			log.Printf("ERROR: Failed writing validation required message: %v", wErr)
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Not validated, treat as failed login for this attempt
	}

	// Authentication Successful!
	log.Printf("INFO: Node %d: User '%s' (Handle: %s) authenticated successfully via RUN:AUTHENTICATE", nodeNumber, authUser.Username, authUser.Handle)
	// Display success message (optional) - Move cursor first
	terminal.Write([]byte(terminalPkg.MoveCursor(errorRow, 1)))
	// successMsg := "\r\n|10Login successful!|07\r\n"
	// terminal.Write(terminal.DisplayContent([]byte(successMsg)))
	// time.Sleep(500 * time.Millisecond)

	// Return the authenticated user object!
	return authUser, "", nil
}

// Run executes the menu logic for a given starting menu name.
// Reverted s parameter back to ssh.Session
// Added outputMode parameter
// Added currentAreaName parameter
func (e *MenuExecutor) Run(s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, startMenu string, nodeNumber int, sessionStartTime time.Time, autoRunLog types.AutoRunTracker, outputMode terminalPkg.OutputMode, currentAreaName string) (string, *user.User, error) {
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
		var numericMatchAction string // Move this declaration up here as well

		// Determine ANSI filename using standard convention
		ansFilename := currentMenuName + ".ANS"
		// Use MenuSetPath for ANSI file
		fullAnsPath := filepath.Join(e.MenuSetPath, "ansi", ansFilename)

		// Process the associated ANSI file to get display bytes and coordinates
		rawAnsiContent, readErr := terminalPkg.GetAnsiFileContent(fullAnsPath)
		var ansiProcessResult terminalPkg.ProcessAnsiResult
		var processErr error
		if readErr != nil {
			log.Printf("ERROR: Failed to read ANSI file %s: %v", ansFilename, readErr)
			// Display error message to user (using new helper)
			errMsg := fmt.Sprintf("\r\n|01Error reading screen file: %s|07\r\n", ansFilename)
			wErr := terminal.DisplayContent([]byte(errMsg))
			if wErr != nil {
				log.Printf("ERROR: Failed writing screen read error: %v", wErr)
			}
			// Reading the screen file is critical, return error
			return "", nil, fmt.Errorf("failed to read screen file %s: %w", ansFilename, readErr)
		}

		// Successfully read, now process for coords and display bytes using the passed outputMode
		ansiProcessResult, processErr = terminalPkg.ProcessAnsiAndExtractCoords(rawAnsiContent, outputMode)
		if processErr != nil {
			log.Printf("ERROR: Failed to process ANSI file %s: %v. Display may be incorrect.", ansFilename, processErr)
			// Processing error is also critical, return error
			return "", nil, fmt.Errorf("failed to process screen file %s: %w", ansFilename, processErr)
		}

		// --- SPECIAL HANDLING FOR LOGIN MENU INTERACTION ---
		if currentMenuName == "LOGIN" {
			if currentUser != nil {
				log.Printf("WARN: Attempting to run LOGIN menu for already authenticated user %s. Skipping login, going to MAIN.", currentUser.Handle)
				// Still need to decide the next step. Let's assume GOTO:MAIN is the intended default.
				// This could eventually come from LOGIN.CFG's default action.
				currentMenuName = "MAIN"
				previousMenuName = "LOGIN" // Set previous explicitly here
				continue
			}

			// Process LOGIN.ANS to extract coordinates and display
			terminal.DisplayContent([]byte("\x1b[2J\x1b[H")) // Clear screen first
			_, wErr := terminal.Write(ansiProcessResult.ProcessedContent)
			if wErr != nil {
				log.Printf("ERROR: Failed to write processed LOGIN.ANS bytes to terminal: %v", wErr)
				return "", nil, fmt.Errorf("failed to display LOGIN.ANS: %w", wErr)
			}

			// Convert coordinates format for handleLoginPrompt
			coords := make(map[string]struct{ X, Y int })
			for rune, pos := range ansiProcessResult.PlaceholderCoords {
				coords[string(rune)] = struct{ X, Y int }{X: pos.X, Y: pos.Y}
			}
			
			// Handle the interactive login prompt using extracted coordinates
			authenticatedUserResult, loginErr := e.handleLoginPrompt(s, terminal, userManager, nodeNumber, coords, outputMode)

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
							currentUser.CurrentMessageAreaTag = area.Tag // Store tag too
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
							currentUser.CurrentFileAreaTag = area.Tag // Store tag too
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
			// Use DisplayContent to handle pipe codes and display
			wErr := terminal.DisplayContent([]byte(errMsg))
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
				terminal.DisplayContent([]byte(prompt))

				// Use our helper for secure input reading (using ssh.Session 's')
				inputPassword, err := readPasswordSecurely(s, terminal)
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
					wErr := terminal.DisplayContent([]byte("\r\n|07Password accepted.|07\r\n"))
					if wErr != nil {
						log.Printf("ERROR: Failed writing password accepted message: %v", wErr)
					}
					break
				} else {
					// Use new helper for feedback message
					wErr := terminal.DisplayContent([]byte("\r\n|01Incorrect Password.|07\r\n"))
					if wErr != nil {
						log.Printf("ERROR: Failed writing incorrect password message: %v", wErr)
					}
				}
			}
			if !passwordOk {
				log.Printf("WARN: User failed password entry for menu '%s' (User: %v)", currentMenuName, currentUser)
				// Use new helper for feedback message
				wErr := terminal.DisplayContent([]byte("\r\n|01Too many incorrect attempts.|07\r\n"))
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
			// Use DisplayContent to handle pipe codes and display
			wErr := terminal.DisplayContent([]byte(errMsg))
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
		// ansBackgroundBytes := ansiProcessResult.ProcessedContent
		if currentMenuName != "LOGIN" {
			if menuRec.GetClrScrBefore() {
				wErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
				if wErr != nil {
					// Log error but continue if possible
					log.Printf("ERROR: Node %d: Failed clearing screen for menu %s: %v", nodeNumber, currentMenuName, wErr)
				}
			}
			// Use new helper for ANSI display (regular case)
			// if currentMenuName == "MAIN" {
			//	log.Printf("DEBUG: Node %d: Bytes for MAIN.ANS before WriteProcessedBytes (hex): %x", nodeNumber, ansiProcessResult.ProcessedContent)
			//}
			_, wErr := terminal.Write(ansiProcessResult.ProcessedContent)
			if wErr != nil {
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
				ansBackgroundBytes := ansiProcessResult.ProcessedContent // Use the already processed bytes

				// Initially draw with first option selected
				selectedIndex := 0
				drawErr := drawLightbarMenu(terminal, ansBackgroundBytes, lightbarOptions, selectedIndex, outputMode)
				if drawErr != nil {
					log.Printf("ERROR: Failed to draw lightbar menu for %s: %v", currentMenuName, drawErr)
					isLightbarMenu = false
				} else {
					// Process keyboard navigation for lightbar
					lightbarResult := "" // Use a local variable for the result
					inputLoop := true
					for inputLoop {
						// Read keyboard input for lightbar navigation
						bufioReader := bufio.NewReader(s)
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

						// Map specific keys for navigation and selection
						switch r {
						case '1', '2', '3', '4', '5', '6', '7', '8', '9':
							// Direct selection by number
							numIndex := int(r - '1') // Convert 1-9 to 0-8
							if numIndex >= 0 && numIndex < len(lightbarOptions) {
								selectedIndex = numIndex
								drawLightbarMenu(terminal, ansBackgroundBytes, lightbarOptions, selectedIndex, outputMode)
								lightbarResult = lightbarOptions[numIndex].HotKey
								inputLoop = false
							}
						case '\r', '\n': // Enter - select current item
							if selectedIndex >= 0 && selectedIndex < len(lightbarOptions) {
								lightbarResult = lightbarOptions[selectedIndex].HotKey
								inputLoop = false
							}
						case 27: // ESC key - check for arrow keys in ANSI sequence
							escSeq := make([]byte, 2)
							n, err := bufioReader.Read(escSeq)
							if err != nil || n != 2 {
								// Just ESC pressed or error reading sequence
								continue // Ignore
							}

							// Check for arrow keys and handle navigation
							if escSeq[0] == 91 { // '['
								switch escSeq[1] {
								case 65: // Up arrow
									if selectedIndex > 0 {
										selectedIndex--
										drawLightbarMenu(terminal, ansBackgroundBytes, lightbarOptions, selectedIndex, outputMode)
									}
								case 66: // Down arrow
									if selectedIndex < len(lightbarOptions)-1 {
										selectedIndex++
										drawLightbarMenu(terminal, ansBackgroundBytes, lightbarOptions, selectedIndex, outputMode)
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
					err = e.displayPrompt(terminal, menuRec, currentUser, nodeNumber, currentMenuName, sessionStartTime, outputMode, currentAreaName) // Pass currentAreaName
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
				err = e.displayPrompt(terminal, menuRec, currentUser, nodeNumber, currentMenuName, sessionStartTime, outputMode, currentAreaName) // Pass currentAreaName
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

			// var numericMatchAction string // Declaration moved outside
			if numInput, err := strconv.Atoi(userInput); err == nil && numInput > 0 {
				log.Printf("DEBUG: User entered numeric input: %d", numInput)
				visibleCmdIndex := 0
				for _, cmdRec := range commands {
					if cmdRec.GetHidden() {
						continue // Skip hidden commands
					}
					cmdACS := cmdRec.ACS
					if !checkACS(cmdACS, currentUser, s, terminal, sessionStartTime) { // Use ssh.Session 's'
						continue // Skip commands user cannot access
					}
					visibleCmdIndex++ // Increment for each visible, accessible command
					if visibleCmdIndex == numInput {
						numericMatchAction = cmdRec.Command
						log.Printf("DEBUG: Numeric input %d matched command index %d, action: '%s'", numInput, visibleCmdIndex, numericMatchAction)
						break // Found numeric match
					}
				}
			}
			// --- End Special Input Handling ---
		} // End if isLightbarMenu / else

		// 6. Process Input / Find Command Match (userInput determined by menu type)
		matched := false
		nextAction := "" // Store the action determined by the matched command

		if numericMatchAction != "" { // Check numeric match first (only relevant for standard menus)
			nextAction = numericMatchAction
			matched = true
		} else { // Check keyword matches (relevant for both)
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
			errMsg := "\r\n|01Unknown command!|07\r\n"
			terminal.DisplayContent([]byte(errMsg))
			time.Sleep(1 * time.Second) // Brief pause on error
			continue                    // Redisplay current menu
		}
	}
}

// handleLoginPrompt manages the interactive username/password entry using coordinates.
// Added outputMode parameter.
func (e *MenuExecutor) handleLoginPrompt(s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, nodeNumber int, coords map[string]struct{ X, Y int }, outputMode terminalPkg.OutputMode) (*user.User, error) {
	// Get coordinates for username and password fields from the map
	userCoord, userOk := coords["P"] // Use 'P' for Handle/Name field based on LOGIN.ANS
	passCoord, passOk := coords["O"] // Use 'O' for Password field based on LOGIN.ANS

	log.Printf("DEBUG: LOGIN Coords Received - P: %+v (Ok: %t), O: %+v (Ok: %t)", userCoord, userOk, passCoord, passOk)

	if !userOk || !passOk {
		log.Printf("CRITICAL: LOGIN.ANS is missing required coordinate codes P or O.")
		terminal.DisplayContent([]byte("\r\n|01CRITICAL ERROR: Login screen configuration invalid (Missing P/O).|07\r\n"))
		time.Sleep(2 * time.Second)
		return nil, fmt.Errorf("missing login coordinates P/O in LOGIN.ANS")
	}

	errorRow := passCoord.Y + 2 // Default error message row below password
	if errorRow <= userCoord.Y || errorRow <= passCoord.Y {
		errorRow = userCoord.Y + 2 // Adjust if overlapping
	}

	// Move to Username position (using original X) and read input
	// Subtract 1 from Y coordinate to move cursor up one line
	terminal.Write([]byte(terminalPkg.MoveCursor(userCoord.Y-1, userCoord.X)))
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

	// Move to Password position and read input securely
	// Subtract 1 from Y coordinate to move cursor up one line
	terminal.Write([]byte(terminalPkg.MoveCursor(passCoord.Y-1, passCoord.X)))
	password, err := readPasswordSecurely(s, terminal) // Use ssh.Session 's'
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF // Signal disconnection
		}
		if err.Error() == "password entry interrupted" { // Check for Ctrl+C
			log.Printf("INFO: Node %d: User interrupted password entry.", nodeNumber)
			terminal.Write([]byte(terminalPkg.MoveCursor(errorRow, 1)))
			terminal.DisplayContent([]byte("\r\n|01Login cancelled.|07\r\n"))
			time.Sleep(500 * time.Millisecond)
			return nil, nil // Signal retry LOGIN
		}
		log.Printf("ERROR: Node %d: Failed to read password securely: %v", nodeNumber, err)
		return nil, fmt.Errorf("failed reading password: %w", err)
	}

	// Attempt Authentication via UserManager
	log.Printf("DEBUG: Node %d: Attempting authentication for user: %s", nodeNumber, username)
	authUser, authenticated := userManager.Authenticate(username, password)
	if !authenticated {
		log.Printf("WARN: Node %d: Failed authentication attempt for user: %s", nodeNumber, username)
		terminal.Write([]byte(terminalPkg.MoveCursor(errorRow, 1))) // Move cursor for message
		errMsg := "\r\n|01Login incorrect.|07\r\n"
		// Use WriteProcessedBytes with the passed outputMode
		wErr := terminal.DisplayContent([]byte(errMsg))
		if wErr != nil {
			log.Printf("ERROR: Failed writing login incorrect message: %v", wErr)
		}
		time.Sleep(1 * time.Second) // Pause after failed attempt
		return nil, nil             // Failed auth, but not a critical error. Let LOGIN menu handle retries.
	}

	if !authUser.Validated {
		log.Printf("INFO: Node %d: Login denied for user '%s' - account not validated", nodeNumber, username)
		terminal.Write([]byte(terminalPkg.MoveCursor(errorRow, 1))) // Move cursor for message
		errMsg := "\r\n|01Account requires validation by SysOp.|07\r\n"
		// Use WriteProcessedBytes with the passed outputMode
		wErr := terminal.DisplayContent([]byte(errMsg))
		if wErr != nil {
			log.Printf("ERROR: Failed writing validation required message: %v", wErr)
		}
		time.Sleep(1 * time.Second)
		return nil, nil // Not validated, treat as failed login for this attempt
	}

	log.Printf("INFO: Node %d: User '%s' (Handle: %s) authenticated successfully via LOGIN prompt", nodeNumber, authUser.Username, authUser.Handle)
	return authUser, nil // Success!
}

// readPasswordSecurely reads a password from the terminal without echoing characters
func readPasswordSecurely(s ssh.Session, terminal *terminalPkg.BBS) (string, error) {
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
			_, _ = terminal.Write([]byte("\r\n")) // Ignore errors for password prompt
			return string(password), nil
		case '\n': // Newline - often follows \r, ignore it if so.
			continue
		case 127, 8: // Backspace (DEL or BS)
			if len(password) > 0 {
				password = password[:len(password)-1]
				_, err := terminal.Write([]byte("\b \b"))
				if err != nil {
					log.Printf("WARN: Failed to write backspace sequence: %v", err)
				}
			}
		case 3: // Ctrl+C (ETX)
			_, _ = terminal.Write([]byte("^C\r\n")) // Ignore errors for interrupt
			return "", fmt.Errorf("password entry interrupted")
		default:
			if r >= 32 { // Basic check for printable ASCII
				password = append(password, r)
				byteBuf[0] = '*'
				_, err := terminal.Write(byteBuf[:])
				if err != nil {
					log.Printf("WARN: Failed to write asterisk: %v", err)
				}
			}
		}
	}
}

// executeCommandAction handles the logic for executing a command string (GOTO, RUN, DOOR, LOGOFF).
// Returns: actionType (GOTO, LOGOFF, CONTINUE), nextMenu, resultingUser, error
func (e *MenuExecutor) executeCommandAction(action string, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, outputMode terminalPkg.OutputMode) (actionType string, nextMenu string, userResult *user.User, err error) {
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
				terminal.DisplayContent([]byte(errMsg))
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
			terminal.DisplayContent([]byte(msg))
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
				terminal.DisplayContent([]byte(errMsg))
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
func runLastCallers(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running LASTCALLERS", nodeNumber)

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
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading LASTCALL templates")
	}

	// --- Process Pipe Codes in Templates FIRST ---
	processedTopTemplate := string(terminal.ProcessPipeCodes(topTemplateBytes))
	processedMidTemplate := string(terminal.ProcessPipeCodes(midTemplateBytes)) // Process MID template
	processedBotTemplate := string(terminal.ProcessPipeCodes(botTemplateBytes))
	// --- END Template Processing ---

	// 2. Get last callers data from UserManager
	lastCallers := userManager.GetLastCallers()

	// 3. Build the output string using processed templates and processed data
	var outputBuffer bytes.Buffer
	outputBuffer.Write([]byte(processedTopTemplate)) // Write processed top template

	if len(lastCallers) == 0 {
		// Optional: Handle empty state. The template might handle this.
		log.Printf("DEBUG: Node %d: No last callers to display.", nodeNumber)
		// If templates don't handle empty, add a message here.
	} else {
		// Iterate through call records and format using processed LASTCALL.MID
		for _, record := range lastCallers {
			line := processedMidTemplate // Start with the pipe-code-processed mid template

			// Format data for substitution
			baud := record.BaudRate // Static for now
			// Process pipe codes in user data *before* substitution
			name := record.Handle            // Process Handle
			groupLoc := record.GroupLocation // Process GroupLocation
			// --- END User Data Processing ---
			onTime := record.ConnectTime.Format("15:04:05") // HH:MM:SS format
			actions := record.Actions                       // Placeholder - process if it can contain pipe codes
			hours := int(record.Duration.Hours())
			mins := int(record.Duration.Minutes()) % 60
			hmm := fmt.Sprintf("%d:%02d", hours, mins)
			upM := fmt.Sprintf("%.1f", record.UploadedMB)
			dnM := fmt.Sprintf("%.1f", record.DownloadedMB)
			nodeStr := strconv.Itoa(record.NodeID)
			callNumStr := strconv.FormatUint(record.CallNumber, 10) // Format the new call number

			// Replace placeholders with *already processed* data
			// Match placeholders found in LASTCALL.MID: ^UN, ^BA etc.
			line = strings.ReplaceAll(line, "^BA", baud)       // Corrected placeholder for Baud
			line = strings.ReplaceAll(line, "^UN", name)       // Corrected placeholder for User Name/Handle
			line = strings.ReplaceAll(line, "^GL", groupLoc)   // Keep this if present in template
			line = strings.ReplaceAll(line, "^OT", onTime)     // Keep this if present in template
			line = strings.ReplaceAll(line, "^AC", actions)    // Keep this if present in template
			line = strings.ReplaceAll(line, "^HM", hmm)        // Keep this if present in template
			line = strings.ReplaceAll(line, "^UM", upM)        // Keep this if present in template
			line = strings.ReplaceAll(line, "^DM", dnM)        // Keep this if present in template
			line = strings.ReplaceAll(line, "|NU", nodeStr)    // Corrected placeholder for Node Number
			line = strings.ReplaceAll(line, "^CN", callNumStr) // Add replacement for Call Number
			// Removed ^ND placeholder as |NU is used in template

			outputBuffer.WriteString(line) // Add the fully substituted and processed line
		}
	}

	outputBuffer.Write([]byte(processedBotTemplate)) // Write processed bottom template

	// 4. Clear screen and display the assembled content
	writeErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
	if writeErr != nil {
		log.Printf("ERROR: Node %d: Failed clearing screen for LASTCALLERS: %v", nodeNumber, writeErr)
		return nil, "", writeErr
	}

	// Use WriteProcessedBytes for the assembled template content
	processedContent := outputBuffer.Bytes() // Contains already-processed ANSI bytes
	_, wErr := terminal.Write(processedContent)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing LASTCALLERS output: %v", nodeNumber, wErr)
		return nil, "", wErr
	}

	// 5. Wait for Enter using configured PauseString (logic remains the same)
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... " // Fallback
	}

	var pauseBytesToWrite []byte
	processedPausePrompt := string(terminal.ProcessPipeCodes([]byte(pausePrompt)))
	if outputMode == terminalPkg.OutputModeCP437 {
		var cp437Buf bytes.Buffer
		for _, r := range string(processedPausePrompt) {
			if r < 128 {
				cp437Buf.WriteByte(byte(r))
			} else if cp437Byte, ok := terminalPkg.UnicodeToCP437Table[r]; ok {
				cp437Buf.WriteByte(cp437Byte)
			} else {
				cp437Buf.WriteByte('?')
			}
		}
		pauseBytesToWrite = cp437Buf.Bytes()
	} else {
		pauseBytesToWrite = []byte(processedPausePrompt)
	}

	log.Printf("DEBUG: Node %d: Writing LASTCALLERS pause prompt. Mode: %d, Bytes: %q", nodeNumber, outputMode, string(pauseBytesToWrite))
	// Log hex bytes before writing
	log.Printf("DEBUG: Node %d: Writing LASTCALLERS pause bytes (hex): %x", nodeNumber, pauseBytesToWrite)
	_, wErr = terminal.Write(pauseBytesToWrite)
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

// displayFile reads and displays an ANSI file from the MENU SET's ansi directory.
func (e *MenuExecutor) displayFile(terminal *terminalPkg.BBS, filename string, outputMode terminalPkg.OutputMode) error {
	// Construct full path using MenuSetPath
	filePath := filepath.Join(e.MenuSetPath, "ansi", filename)

	// Read the file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to read ANSI file %s: %v", filePath, err)
		errMsg := fmt.Sprintf("\r\n|01Error loading file: %s|07\r\n", filename)
		// Use new helper, need outputMode... Pass it into displayFile?
		// Use the passed outputMode for the error message
		writeErr := terminal.DisplayContent([]byte(errMsg)) // Use passed outputMode
		if writeErr != nil {
			log.Printf("ERROR: Failed writing displayFile error message: %v", writeErr)
		}
		return writeErr
	}

	// Write the data using the new helper (this assumes displayFile is ONLY for ANSI files)
	// We should ideally process the file content using ProcessAnsiAndExtractCoords first,
	// but for a quick fix, let's assume CP437 output is desired here.
	// Use the passed outputMode for the file content
	_, err = terminal.Write(data) // Use passed outputMode
	if err != nil {
		log.Printf("ERROR: Failed to write ANSI file %s using WriteProcessedBytes: %v", filePath, err)
		return err
	}

	return nil
}

// displayPrompt handles rendering the menu prompt, including file includes and placeholder substitution.
// Added currentAreaName parameter
func (e *MenuExecutor) displayPrompt(terminal *terminalPkg.BBS, menu *MenuRecord, currentUser *user.User, nodeNumber int, currentMenuName string, sessionStartTime time.Time, outputMode terminalPkg.OutputMode, currentAreaName string) error {
	promptString := menu.Prompt1 // Use Prompt1

	// Special handling for MSGMENU prompt (Corrected menu name)
	if currentMenuName == "MSGMENU" && e.LoadedStrings.MessageMenuPrompt != "" {
		promptString = e.LoadedStrings.MessageMenuPrompt
		log.Printf("DEBUG: Using MessageMenuPrompt for MSGMENU")
	} else if promptString == "" {
		if e.LoadedStrings.DefPrompt != "" { // Use loaded strings
			promptString = e.LoadedStrings.DefPrompt
		} else {
			log.Printf("WARN: Default prompt (DefPrompt) is empty in config/strings.json and Prompt1/MessageMenuPrompt is empty for menu %s. No prompt will be displayed.", currentMenuName)
			return nil // Explicitly return nil if no prompt string can be determined
		}
	}

	log.Printf("DEBUG: Displaying menu prompt for: %s", currentMenuName)

	placeholders := map[string]string{
		"|NODE":   strconv.Itoa(nodeNumber), // Node Number
		"|DATE":   time.Now().Format("01/02/06"),
		"|TIME":   time.Now().Format("15:04"),
		"|MN":     currentMenuName, // Menu Name
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
	rawPromptBytes := terminal.ProcessPipeCodes([]byte(processedPrompt))

	// 4. Process character encoding based on outputMode (Reverted to manual loop)
	var finalBuf bytes.Buffer
	finalBuf.Write([]byte("\r\n")) // Add newline prefix

	for i := 0; i < len(rawPromptBytes); i++ {
		b := rawPromptBytes[i]
		if b < 128 || outputMode == terminalPkg.OutputModeCP437 {
			// ASCII or CP437 mode, write raw byte
			finalBuf.WriteByte(b)
		} else {
			// UTF-8 mode, convert extended characters
			r := terminalPkg.Cp437ToUnicode[b] // Use the exported map
			if r == 0 && b != 0 {
				finalBuf.WriteByte('?') // Fallback
			} else if r != 0 {
				finalBuf.WriteRune(r)
			}
		}
	}

	// 5. Write the final processed bytes using the terminal's standard Write (Reverted)
	_, err = terminal.Write(finalBuf.Bytes())
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

// runFullLoginSequence executes the sequence of actions after FASTLOGN option 1.
func runFullLoginSequence(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	log.Printf("INFO: Node %d: Running FULL_LOGIN_SEQUENCE for user %s", nodeNumber, currentUser.Handle)
	var nextAction string
	var err error

	// 1. Run Last Callers
	_, nextAction, err = runLastCallers(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode)
	if err != nil {
		log.Printf("ERROR: Node %d: Error during runLastCallers in sequence: %v", nodeNumber, err)
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
	}
	if nextAction == "LOGOFF" {
		return nil, "LOGOFF", nil
	}

	// 2. Run Oneliners
	_, nextAction, err = runOneliners(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode)
	if err != nil {
		log.Printf("ERROR: Node %d: Error during runOneliners in sequence: %v", nodeNumber, err)
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
	}
	if nextAction == "LOGOFF" {
		return nil, "LOGOFF", nil
	}

	// 3. Run Show Stats
	_, nextAction, err = runShowStats(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode)
	if err != nil {
		log.Printf("ERROR: Node %d: Error during runShowStats in sequence: %v", nodeNumber, err)
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
	}
	if nextAction == "LOGOFF" {
		return nil, "LOGOFF", nil
	}

	// 4. Signal transition to MAIN menu
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

	// Try to load commands from CFG file
	commandsByHotkey := make(map[string]string)
	cfgFile, err := os.Open(cfgPath)
	if err != nil {
		log.Printf("WARN: Failed to load CFG file %s: %v", cfgPath, err)
	} else {
		defer cfgFile.Close()
		scanner := bufio.NewScanner(cfgFile)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ";") {
				continue // Skip empty lines and comments
			}

			parts := strings.SplitN(line, " ", 2)
			if len(parts) != 2 {
				continue // Skip malformed lines
			}

			hotkey := strings.ToUpper(strings.TrimSpace(parts[0]))
			command := strings.TrimSpace(parts[1])
			commandsByHotkey[hotkey] = command
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
func drawLightbarMenu(terminal *terminalPkg.BBS, backgroundBytes []byte, options []LightbarOption, selectedIndex int, outputMode terminalPkg.OutputMode) error {
	// Draw static background
	// We might need to clear attributes before drawing background if it has colors
	// _, err := terminal.Write([]byte(attrReset))
	// if err != nil {
	// 	return fmt.Errorf("failed resetting attributes before background: %w", err)
	// }
	_, err := terminal.Write(backgroundBytes)
	if err != nil {
		return fmt.Errorf("failed writing lightbar background: %w", err)
	}

	// Draw each option, highlighting the selected one
	for i, opt := range options {
		// Position cursor
		posCmd := fmt.Sprintf("\x1b[%d;%dH", opt.Y, opt.X)
		_, err := terminal.Write([]byte(posCmd))
		if err != nil {
			return fmt.Errorf("failed positioning cursor for lightbar option: %w", err)
		}

		// Apply correct color based on selection
		var colorCode int
		if i == selectedIndex {
			colorCode = opt.HighlightColor
		} else {
			colorCode = opt.RegularColor
		}
		ansiColorSequence := colorCodeToAnsi(colorCode)
		_, err = terminal.Write([]byte(ansiColorSequence))

		if err != nil {
			return fmt.Errorf("failed setting color for lightbar option: %w", err)
		}

		// Write the option text
		// Ensure text fits, potentially pad/truncate based on some assumed width?
		// For now, just write the text.
		_, err = terminal.Write([]byte(opt.Text))
		if err != nil {
			return fmt.Errorf("failed writing lightbar option text: %w", err)
		}

		// Always reset attributes after each option to ensure clean display
		_, err = terminal.Write([]byte(attrReset))
		if err != nil {
			return fmt.Errorf("failed resetting attributes after lightbar option: %w", err)
		}
	}

	return nil
}

// Helper function to request and parse cursor position
// Returns row, col, error
func requestCursorPosition(s ssh.Session, terminal *terminalPkg.BBS) (int, int, error) {
	// Ensure terminal is in a state to respond (raw mode might be needed temporarily,
	// but the main loop often handles raw mode via terminal.ReadLine() or pty)
	// If not in raw mode, the response might not be read correctly.

	_, err := terminal.Write([]byte("\x1b[6n")) // DSR - Device Status Report - Request cursor position
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

// promptYesNoLightbar displays a Yes/No prompt with lightbar selection.
// Returns true for Yes, false for No, and error on issues like disconnect.
func (e *MenuExecutor) promptYesNoLightbar(s ssh.Session, terminal *terminalPkg.BBS, promptText string, outputMode terminalPkg.OutputMode, nodeNumber int) (bool, error) {
	// Use nodeNumber in logging calls instead of e.nodeID
	ptyReq, _, isPty := s.Pty()
	hasPtyHeight := isPty && ptyReq.Window.Height > 0

	if hasPtyHeight {
		// --- Dynamic Lightbar Logic (if terminal height is known) ---
		log.Printf("DEBUG: Terminal height known (%d), using lightbar prompt.", ptyReq.Window.Height)
		promptRow := ptyReq.Window.Height // Use last row
		promptCol := 3
		yesOptionText := " Yes "
		noOptionText := " No " // Ensure consistent padding
		yesNoSpacing := 2      // Spaces between prompt and first option (after cursor)
		optionSpacing := 2     // Spaces between Yes and No
		highlightColor := e.Theme.YesNoHighlightColor
		regularColor := e.Theme.YesNoRegularColor

		// Use WriteProcessedBytes for ANSI codes
		saveCursorBytes := []byte(terminalPkg.SaveCursor())
		log.Printf("DEBUG: Node %d: Writing prompt save cursor bytes (hex): %x", nodeNumber, saveCursorBytes) // Use nodeNumber
		_, wErr := terminal.Write(saveCursorBytes)
		if wErr != nil {
			log.Printf("WARN: Failed saving cursor: %v", wErr)
		}
		defer func() {
			restoreCursorBytes := []byte(terminalPkg.RestoreCursor())
			log.Printf("DEBUG: Node %d: Writing prompt restore cursor bytes (hex): %x", nodeNumber, restoreCursorBytes) // Use nodeNumber
			_, wErr := terminal.Write(restoreCursorBytes)
			if wErr != nil {
				log.Printf("WARN: Failed restoring cursor: %v", wErr)
			}
		}()

		// Clear the prompt line first
		clearCmdBytes := []byte(fmt.Sprintf("\x1b[%d;1H\x1b[2K", promptRow))                               // Move + Clear line
		log.Printf("DEBUG: Node %d: Writing prompt clear line bytes (hex): %x", nodeNumber, clearCmdBytes) // Use nodeNumber
		_, wErr = terminal.Write(clearCmdBytes)
		if wErr != nil {
			log.Printf("WARN: Failed clearing prompt line: %v", wErr)
		}

		// Move to prompt column and display prompt text
		promptPosCmdBytes := []byte(fmt.Sprintf("\x1b[%d;%dH", promptRow, promptCol))
		log.Printf("DEBUG: Node %d: Writing prompt position bytes (hex): %x", nodeNumber, promptPosCmdBytes) // Use nodeNumber
		_, wErr = terminal.Write(promptPosCmdBytes)
		if wErr != nil {
			log.Printf("WARN: Failed positioning for prompt: %v", wErr)
		}

		promptDisplayBytes := terminal.ProcessPipeCodes([]byte(promptText))
		log.Printf("DEBUG: Node %d: Writing prompt text bytes (hex): %x", nodeNumber, promptDisplayBytes) // Use nodeNumber
		_, err := terminal.Write(promptDisplayBytes)
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
			saveCursorBytes := []byte(terminalPkg.SaveCursor())
			log.Printf("DEBUG: Node %d: Writing prompt drawOpt save cursor bytes (hex): %x", nodeNumber, saveCursorBytes) // Use nodeNumber
			_, wErr := terminal.Write(saveCursorBytes)
			if wErr != nil {
				log.Printf("WARN: Failed saving cursor in drawOptions: %v", wErr)
			}
			defer func() {
				restoreCursorBytes := []byte(terminalPkg.RestoreCursor())
				log.Printf("DEBUG: Node %d: Writing prompt drawOpt restore cursor bytes (hex): %x", nodeNumber, restoreCursorBytes) // Use nodeNumber
				_, wErr := terminal.Write(restoreCursorBytes)
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
				_, wErr = terminal.Write(posCmdBytes)
				if wErr != nil {
					log.Printf("WARN: Failed positioning cursor for option %d: %v", i, wErr)
				}

				colorCode := opt.RegularColor
				if i == currentSelection {
					colorCode = opt.HighlightColor
				}
				ansiColorSequenceBytes := []byte(colorCodeToAnsi(colorCode))
				log.Printf("DEBUG: Node %d: Writing prompt option %d color bytes (hex): %x", nodeNumber, i, ansiColorSequenceBytes) // Use nodeNumber
				_, wErr = terminal.Write(ansiColorSequenceBytes)
				if wErr != nil {
					log.Printf("WARN: Failed setting color for option %d: %v", i, wErr)
				}

				optionTextBytes := []byte(opt.Text)
				log.Printf("DEBUG: Node %d: Writing prompt option %d text bytes (hex): %x", nodeNumber, i, optionTextBytes) // Use nodeNumber
				_, wErr = terminal.Write(optionTextBytes)
				if wErr != nil {
					log.Printf("WARN: Failed writing text for option %d: %v", i, wErr)
				}

				resetBytes := []byte("\x1b[0m")                                                                         // Reset attributes
				log.Printf("DEBUG: Node %d: Writing prompt option %d reset bytes (hex): %x", nodeNumber, i, resetBytes) // Use nodeNumber
				_, wErr = terminal.Write(resetBytes)
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
			_, wErr := terminal.Write([]byte(posCmd))
			if wErr != nil {
				log.Printf("WARN: Failed positioning cursor for input: %v", wErr)
			}

			r, _, err := bufioReader.ReadRune()
			if err != nil {
				// Clear line on error using WriteProcessedBytes
				clearCmd := fmt.Sprintf("\x1b[%d;1H\x1b[2K", promptRow)
				log.Printf("DEBUG: Node %d: Writing prompt clear on read error bytes (hex): %x", nodeNumber, []byte(clearCmd)) // Use nodeNumber
				_, wErr := terminal.Write([]byte(clearCmd))
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
				_, wErr := terminal.Write(clearCmdBytes)
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

		// Write the prompt. Add CRLF before it for spacing - Use WriteProcessedBytes
		_, wErr := terminal.Write([]byte("\r\n"))
		if wErr != nil {
			log.Printf("WARN: Failed writing CRLF: %v", wErr)
		}

		processedPromptBytes := terminal.ProcessPipeCodes([]byte(fullPrompt))
		_, err := terminal.Write(processedPromptBytes)
		if err != nil {
			log.Printf("ERROR: Node %d: Failed writing Yes/No prompt text (fallback mode): %v", nodeNumber, err) // Use nodeNumber
			return false, fmt.Errorf("failed writing fallback prompt text: %w", err)
		}

		// Read user input
		input, err := terminal.ReadLine()
		if err != nil {
			// Clean up line on error using WriteProcessedBytes
			_, wErr := terminal.Write([]byte("\r\n")) // Assuming CRLF is enough cleanup here
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
func runListUsers(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
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
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading USERLIST templates")
	}

	// --- Process Pipe Codes in Templates FIRST ---
	processedTopTemplate := string(terminal.ProcessPipeCodes(topTemplateBytes))
	processedMidTemplate := string(terminal.ProcessPipeCodes(midTemplateBytes)) // Process MID template
	processedBotTemplate := string(terminal.ProcessPipeCodes(botTemplateBytes))
	// --- END Template Processing ---

	// 2. Get user list data from UserManager
	users := userManager.GetAllUsers() // Corrected method call

	// 3. Build the output string using processed templates and processed data
	var outputBuffer bytes.Buffer
	outputBuffer.Write([]byte(processedTopTemplate)) // Write processed top template

	if len(users) == 0 {
		// Optional: Handle empty state. The template might handle this.
		log.Printf("DEBUG: Node %d: No users to display.", nodeNumber)
		// If templates don't handle empty, add a message here.
	} else {
		// Iterate through user records and format using processed USERLIST.MID
		for _, user := range users {
			line := processedMidTemplate // Start with the pipe-code-processed mid template

			// Format data for substitution
			handle := user.Handle
			level := strconv.Itoa(user.AccessLevel)
			// privateNote := terminal.DisplayContent([]byte(user.PrivateNote))) // Replaced with GroupLocation
			groupLocation := user.GroupLocation // Added

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
	outputBuffer.Write([]byte(processedBotTemplate)) // Write processed bottom template
	log.Printf("DEBUG: Added BOT template. Final buffer size: %d", outputBuffer.Len())

	// 4. Clear screen and display the assembled content
	writeErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
	if writeErr != nil {
		log.Printf("ERROR: Node %d: Failed clearing screen for USERLIST: %v", nodeNumber, writeErr)
		return nil, "", writeErr
	}

	// Use WriteProcessedBytes for the assembled template content
	processedContent := outputBuffer.Bytes() // Contains already-processed ANSI bytes
	_, wErr := terminal.Write(processedContent)
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
	processedPausePrompt := string(terminal.ProcessPipeCodes([]byte(pausePrompt)))
	if outputMode == terminalPkg.OutputModeCP437 {
		var cp437Buf bytes.Buffer
		for _, r := range string(processedPausePrompt) {
			if r < 128 {
				cp437Buf.WriteByte(byte(r))
			} else if cp437Byte, ok := terminalPkg.UnicodeToCP437Table[r]; ok {
				cp437Buf.WriteByte(cp437Byte)
			} else {
				cp437Buf.WriteByte('?')
			}
		}
		pauseBytesToWrite = cp437Buf.Bytes()
	} else {
		pauseBytesToWrite = []byte(processedPausePrompt)
	}

	log.Printf("DEBUG: Node %d: Writing USERLIST pause prompt. Mode: %d, Bytes: %q", nodeNumber, outputMode, string(pauseBytesToWrite))
	// Log hex bytes before writing
	log.Printf("DEBUG: Node %d: Writing USERLIST pause bytes (hex): %x", nodeNumber, pauseBytesToWrite)
	_, wErr = terminal.Write(pauseBytesToWrite)
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

// runShowVersion displays static version information.
func runShowVersion(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running SHOWVERSION", nodeNumber)

	// Define the version string (can be made dynamic later)
	versionString := "|15ViSiON/3 Go Edition - v0.1.0 (Pre-Alpha)|07"

	// Display the version
	terminal.DisplayContent([]byte("\x1b[2J\x1b[H")) // Optional: Clear screen
	terminal.Write([]byte("\r\n\r\n"))         // Add some spacing
	wErr := terminal.DisplayContent([]byte(versionString))
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
		terminal.Write([]byte("\r\n")) // Add newline before pause only if prompt exists
		var pauseBytesToWrite []byte
		processedPausePrompt := string(terminal.ProcessPipeCodes([]byte(pausePrompt)))
		if outputMode == terminalPkg.OutputModeCP437 {
			var cp437Buf bytes.Buffer
			for _, r := range string(processedPausePrompt) {
				if r < 128 {
					cp437Buf.WriteByte(byte(r))
				} else if cp437Byte, ok := terminalPkg.UnicodeToCP437Table[r]; ok {
					cp437Buf.WriteByte(cp437Byte)
				} else {
					cp437Buf.WriteByte('?')
				}
			}
			pauseBytesToWrite = cp437Buf.Bytes()
		} else {
			pauseBytesToWrite = []byte(processedPausePrompt)
		}
		_, wErr = terminal.Write(pauseBytesToWrite)
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

// runListMessageAreas displays a list of message areas using templates.
func runListMessageAreas(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running LISTMSGAR", nodeNumber)

	// 1. Define Template filenames and paths
	topTemplateFilename := "MSGAREA.TOP"
	midTemplateFilename := "MSGAREA.MID"
	botTemplateFilename := "MSGAREA.BOT"
	templateDir := filepath.Join(e.MenuSetPath, "templates")
	topTemplatePath := filepath.Join(templateDir, topTemplateFilename)
	midTemplatePath := filepath.Join(templateDir, midTemplateFilename)
	botTemplatePath := filepath.Join(templateDir, botTemplateFilename)

	// 2. Load Template Files
	topTemplateBytes, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplateBytes, errBot := os.ReadFile(botTemplatePath)

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load one or more MSGAREA template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading Message Area screen templates.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading MSGAREA templates")
	}

	// 3. Process Pipe Codes in Templates FIRST
	processedTopTemplate := string(terminal.ProcessPipeCodes(topTemplateBytes))
	processedMidTemplate := string(terminal.ProcessPipeCodes(midTemplateBytes))
	processedBotTemplate := string(terminal.ProcessPipeCodes(botTemplateBytes))

	// 4. Get message area list data from MessageManager
	// TODO: Apply ACS filtering here based on currentUser
	areas := e.MessageMgr.ListAreas()

	// 5. Build the output string using processed templates and data
	var outputBuffer bytes.Buffer
	outputBuffer.Write([]byte(processedTopTemplate)) // Write processed top template

	if len(areas) == 0 {
		log.Printf("DEBUG: Node %d: No message areas to display.", nodeNumber)
		// Optional: Add a message like "|07No message areas available.\r\n" if template doesn't handle it
	} else {
		// Iterate through areas and format using processed MSGAREA.MID
		for _, area := range areas {
			line := processedMidTemplate // Start with the pipe-code-processed mid template

			// Process pipe codes in area data
			name := string(terminal.ProcessPipeCodes([]byte(area.Name)))
			desc := string(terminal.ProcessPipeCodes([]byte(area.Description)))
			idStr := strconv.Itoa(area.ID)
			tag := string(terminal.ProcessPipeCodes([]byte(area.Tag))) // Tag might have codes?

			// Replace placeholders in the MID template
			// Expected placeholders: ^ID, ^TAG, ^NA, ^DS (adjust if different)
			line = strings.ReplaceAll(line, "^ID", idStr)
			line = strings.ReplaceAll(line, "^TAG", tag)
			line = strings.ReplaceAll(line, "^NA", name)
			line = strings.ReplaceAll(line, "^DS", desc)

			outputBuffer.WriteString(line) // Add the fully substituted and processed line
		}
	}

	outputBuffer.Write([]byte(processedBotTemplate)) // Write processed bottom template

	// 6. Clear screen and display the assembled content
	writeErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
	if writeErr != nil {
		log.Printf("ERROR: Node %d: Failed clearing screen for LISTMSGAR: %v", nodeNumber, writeErr)
		return nil, "", writeErr
	}

	// Use WriteProcessedBytes for the assembled template content
	processedContent := outputBuffer.Bytes() // Contains already-processed ANSI bytes
	_, wErr := terminal.Write(processedContent)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing LISTMSGAR output: %v", nodeNumber, wErr)
		return nil, "", wErr
	}

	// 7. Wait for Enter using configured PauseString
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... " // Fallback
	}

	var pauseBytesToWrite []byte
	processedPausePrompt := string(terminal.ProcessPipeCodes([]byte(pausePrompt)))
	if outputMode == terminalPkg.OutputModeCP437 {
		var cp437Buf bytes.Buffer
		for _, r := range string(processedPausePrompt) {
			if r < 128 {
				cp437Buf.WriteByte(byte(r))
			} else if cp437Byte, ok := terminalPkg.UnicodeToCP437Table[r]; ok {
				cp437Buf.WriteByte(cp437Byte)
			} else {
				cp437Buf.WriteByte('?')
			}
		}
		pauseBytesToWrite = cp437Buf.Bytes()
	} else {
		pauseBytesToWrite = []byte(processedPausePrompt)
	}

	log.Printf("DEBUG: Node %d: Writing LISTMSGAR pause prompt. Mode: %d, Bytes: %q", nodeNumber, outputMode, string(pauseBytesToWrite))
	// Log hex bytes before writing
	log.Printf("DEBUG: Node %d: Writing LISTMSGAR pause bytes (hex): %x", nodeNumber, pauseBytesToWrite)
	_, wErr = terminal.Write(pauseBytesToWrite)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing LISTMSGAR pause prompt: %v", nodeNumber, wErr)
	}

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during LISTMSGAR pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading input during LISTMSGAR pause: %v", nodeNumber, err)
			return nil, "", err
		}
		if r == '\r' || r == '\n' { // Check for CR or LF
			break
		}
	}

	return nil, "", nil // Success
}

// runComposeMessage handles the process of composing and saving a new message.
func runComposeMessage(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
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
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return nil, "", nil // Return to menu
		}
		if currentUser.CurrentMessageAreaTag == "" || currentUser.CurrentMessageAreaID <= 0 {
			log.Printf("WARN: Node %d: COMPOSEMSG called by %s, but no current message area is set.", nodeNumber, currentUser.Handle)
			msg := "\r\n|01Error: No current message area selected.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
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
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu, not an error
	}

	// Check user logged in (required for ACS check and posting)
	if currentUser == nil {
		log.Printf("WARN: Node %d: COMPOSEMSG reached ACS check without logged in user (Area: %s).", nodeNumber, areaTag)
		msg := "\r\n|01Error: You must be logged in to post messages.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
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

	// === BEGIN MOVED SUBJECT LOGIC ===
	// 2. Prompt for Subject
	prompt := e.LoadedStrings.MsgTitleStr
	if prompt == "" {
		log.Printf("WARN: Node %d: MsgTitleStr is empty in strings config. Using fallback.", nodeNumber)
		prompt = "|07Subject: |15"
	}
	// Add newline for spacing before prompt
	terminal.Write([]byte("\r\n"))
	wErr := terminal.DisplayContent([]byte(prompt))
	if wErr != nil {
		log.Printf("WARN: Node %d: Failed to write subject prompt: %v", nodeNumber, wErr)
	}

	subject, err := terminal.ReadLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during subject input.", nodeNumber)
			return nil, "LOGOFF", io.EOF
		}
		log.Printf("ERROR: Node %d: Failed reading subject input: %v", nodeNumber, err)
		terminal.Write([]byte("\r\nError reading subject.\r\n"))
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}
	subject = strings.TrimSpace(subject)
	// Allow empty subject for now, set default later if needed after editor
	// if subject == "" {
	// 	subject = "(no subject)" // Default subject
	// }
	// Check if subject is truly empty after defaulting
	// if strings.TrimSpace(subject) == "" {
	// 	terminal.Write([]byte("\r\nSubject cannot be empty. Post cancelled.\r\n"))
	// 	time.Sleep(1 * time.Second)
	// 	return nil, "", nil // Return to menu
	// }
	// === END MOVED SUBJECT LOGIC ===

	// 3. Call the Editor
	// We need to temporarily leave the menu loop's terminal control
	// The editor.RunEditor will take over the PTY.
	// Reverted to logic from commit 32f3c59...

	log.Printf("DEBUG: Node %d: Clearing screen before calling editor.RunEditor", nodeNumber)
	terminal.DisplayContent([]byte("\x1b[2J\x1b[H")) // Clear screen before editor (as per 32f3c59)
	log.Printf("DEBUG: Node %d: Calling editor.RunEditor for area %s", nodeNumber, area.Tag)

	// Get TERM env var
	termType := "unknown"
	for _, env := range s.Environ() {
		if strings.HasPrefix(env, "TERM=") {
			termType = strings.TrimPrefix(env, "TERM=")
			break
		}
	}
	log.Printf("DEBUG: Node %d: Passing TERM=%s to editor.RunEditor", nodeNumber, termType)

	body, saved, err := editor.RunEditor("", s, s, termType) // Pass session as input/output and termType
	log.Printf("DEBUG: Node %d: editor.RunEditor returned. Error: %v, Saved: %v, Body length: %d", nodeNumber, err, saved, len(body))

	if err != nil {
		log.Printf("ERROR: Node %d: Editor failed for user %s: %v", nodeNumber, currentUser.Handle, err)
		// Restore original error handling from 32f3c59 (propagate error)
		// Need to handle redraw in the main loop potentially.
		// Consider clearing screen here before returning?
		// terminal.Write([]byte(terminalPkg.ClearScreen())) // Optional clear on error
		return nil, "", fmt.Errorf("editor error: %w", err)
	}

	// IMPORTANT: Need to redraw the current menu screen after editor exits!
	// This needs to be handled by the main menu loop after this function returns (CONTINUE).
	// Clear screen *after* editor exits (as per 32f3c59 placeholder logic)
	terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
	// terminal.Write([]byte("\\r\\nEditor exited. Processing message...\\r\\n")) // Removed placeholder

	if !saved {
		log.Printf("INFO: Node %d: User %s aborted message composition for area %s.", nodeNumber, currentUser.Handle, area.Tag)
		// Restore message from 32f3c59
		terminal.Write([]byte("\r\nMessage aborted.\r\n"))
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to current menu
	}

	if strings.TrimSpace(body) == "" {
		log.Printf("INFO: Node %d: User %s saved empty message for area %s.", nodeNumber, currentUser.Handle, area.Tag)
		// Restore message from 32f3c59
		terminal.Write([]byte("\r\nMessage body empty. Aborting post.\r\n"))
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to current menu
	}

	// === MOVED: Subject prompt is now before editor ===
	// Set default subject if user left it blank earlier
	if subject == "" {
		subject = "(no subject)" // Default subject
	}

	// 4. Construct the Message (Subject is already set)
	currentNodeID := "1:1/100" // Placeholder - get from config or session

	newMessage := message.Message{
		ID:           uuid.New(),
		AreaID:       area.ID,
		FromUserName: currentUser.Handle,
		FromNodeID:   currentNodeID,        // TODO: Get actual node ID
		ToUserName:   message.MsgToUserAll, // Public message
		Subject:      subject,
		Body:         body,
		PostedAt:     time.Now(), // Set creation time
		IsPrivate:    false,
		// ReplyToID, Path, Attributes, ReadBy can be added later
	}

	// 5. Save the Message
	err = e.MessageMgr.AddMessage(area.ID, newMessage)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to save message from user %s to area %s: %v", nodeNumber, currentUser.Handle, area.Tag, err)
		// TODO: Display user-friendly error
		terminal.Write([]byte("\r\n|01Error saving message!|07\r\n"))
		time.Sleep(2 * time.Second)
		return nil, "", fmt.Errorf("failed saving message: %w", err) // Return error for now
	}

	// 6. Confirmation
	log.Printf("INFO: Node %d: User %s successfully posted message %s to area %s", nodeNumber, currentUser.Handle, newMessage.ID, area.Tag)
	// TODO: Use string config for confirmation message
	terminal.Write([]byte("\r\n|02Message Posted!|07\r\n"))
	time.Sleep(1 * time.Second)

	// Return to current menu. The menu loop should handle redraw because we return CONTINUE action ("", nil).
	return nil, "", nil
}

// runPromptAndComposeMessage lists areas, prompts for selection, checks permissions, and calls runComposeMessage.
func runPromptAndComposeMessage(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running runPromptAndComposeMessage", nodeNumber)

	if currentUser == nil {
		log.Printf("WARN: Node %d: runPromptAndComposeMessage called without logged in user.", nodeNumber)
		// Display user-friendly error
		msg := "\r\n|01Error: You must be logged in to post messages.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
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
		terminal.DisplayContent([]byte(msg))
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading MSGAREA templates for prompt")
	}

	processedTopTemplate := string(terminal.ProcessPipeCodes(topTemplateBytes))
	processedMidTemplate := string(terminal.ProcessPipeCodes(midTemplateBytes))
	processedBotTemplate := string(terminal.ProcessPipeCodes(botTemplateBytes)) // Process BOT template

	areas := e.MessageMgr.ListAreas() // Get all areas
	// Filter areas based on read access (for listing)
	// For now, list all areas, permission check happens later on selection.
	// TODO: Implement ACSRead filtering here if needed for the list display.

	terminal.DisplayContent([]byte("\x1b[2J\x1b[H")) // Clear before displaying list
	terminal.Write([]byte(processedTopTemplate))       // Write TOP

	if len(areas) == 0 {
		log.Printf("DEBUG: Node %d: No message areas available to post in.", nodeNumber)
		terminal.Write([]byte("\r\n|07No message areas available.|07\r\n"))
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	for _, area := range areas {
		line := processedMidTemplate
		name := string(terminal.ProcessPipeCodes([]byte(area.Name)))
		desc := string(terminal.ProcessPipeCodes([]byte(area.Description)))
		idStr := strconv.Itoa(area.ID)
		tag := string(terminal.ProcessPipeCodes([]byte(area.Tag)))

		line = strings.ReplaceAll(line, "^ID", idStr)
		line = strings.ReplaceAll(line, "^TAG", tag)
		line = strings.ReplaceAll(line, "^NA", name)
		line = strings.ReplaceAll(line, "^DS", desc)

		terminal.Write([]byte(line)) // Write MID for each area
	}

	terminal.Write([]byte(processedBotTemplate)) // Write BOT

	// 2. Prompt for Area Selection
	// TODO: Use a configurable string for this prompt
	prompt := "\r\n|07Enter Area ID or Tag to Post In (or Enter to cancel): |15"
	log.Printf("DEBUG: Node %d: Writing prompt for message area selection bytes (hex): %x", nodeNumber, terminal.ProcessPipeCodes([]byte(prompt)))
	wErr := terminal.DisplayContent([]byte(prompt))
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
		terminal.Write([]byte("\r\nPost cancelled.\r\n"))
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
		terminal.DisplayContent([]byte(msg))
		time.Sleep(1 * time.Second)
		// TODO: Need to redraw menu
		return nil, "", nil // Return to menu
	}

	// Check write permission
	if !checkACS(selectedArea.ACSWrite, currentUser, s, terminal, sessionStartTime) {
		log.Printf("WARN: Node %d: User %s denied post access to selected area %s (%s)", nodeNumber, currentUser.Handle, selectedArea.Tag, selectedArea.ACSWrite)
		// TODO: Use configurable string for access denied
		msg := fmt.Sprintf("\r\n|01Access denied to post in area: %s|07\r\n", selectedArea.Name)
		terminal.DisplayContent([]byte(msg))
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
func runReadMsgs(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running READMSGS", nodeNumber)

	// 1. Check User and Area
	if currentUser == nil {
		log.Printf("WARN: Node %d: READMSGS called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to read messages.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	currentAreaID := currentUser.CurrentMessageAreaID
	currentAreaTag := currentUser.CurrentMessageAreaTag

	if currentAreaID <= 0 || currentAreaTag == "" {
		log.Printf("WARN: Node %d: User %s has no current message area set (ID: %d, Tag: %s).", nodeNumber, currentUser.Handle, currentAreaID, currentAreaTag)
		msg := "\r\n|01Error: No message area selected.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	log.Printf("INFO: Node %d: User %s checking messages for Area ID %d (%s)", nodeNumber, currentUser.Handle, currentAreaID, currentAreaTag)

	// --- NEW LOGIC START ---

	// 2. Fetch total message count first
	totalMessageCount, err := e.MessageMgr.GetMessageCountForArea(currentAreaID)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to get total message count for area %d: %v", nodeNumber, currentAreaID, err)
		msg := fmt.Sprintf("\r\n|01Error loading message info for area %s.|07\r\n", currentAreaTag)
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", err
	}

	if totalMessageCount == 0 {
		log.Printf("INFO: Node %d: No messages found in area %s (%d).", nodeNumber, currentAreaTag, currentAreaID)
		msg := fmt.Sprintf("\r\n|07No messages in area |15%s|07.\r\n", currentAreaTag)
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	// 3. Check for New Messages
	lastReadID := "" // Default to empty (fetch all if no record)
	if currentUser.LastReadMessageIDs != nil {
		lastReadID = currentUser.LastReadMessageIDs[currentAreaID]
	}
	newCount, err := e.MessageMgr.GetNewMessageCount(currentAreaID, lastReadID)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to get new message count for area %d: %v", nodeNumber, currentAreaID, err)
		msg := fmt.Sprintf("\r\n|01Error checking for new messages in area %s.|07\r\n", currentAreaTag)
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", err
	}

	var messages []message.Message
	currentMessageIndex := 0
	totalMessages := 0 // Will be set based on context (new or all)

	var readingNewMessages bool // Flag to track if we started by reading new messages

	if newCount > 0 {
		// 4a. Load Only New Messages
		readingNewMessages = true // <<< SET FLAG
		log.Printf("INFO: Node %d: Found %d new messages in area %d since ID '%s'. Loading new.", nodeNumber, newCount, currentAreaID, lastReadID)
		messages, err = e.MessageMgr.GetMessagesForArea(currentAreaID, lastReadID)
		if err != nil {
			log.Printf("ERROR: Node %d: Failed to load new messages for area %d: %v", nodeNumber, currentAreaID, err)
			msg := fmt.Sprintf("\r\n|01Error loading new messages for area %s.|07\r\n", currentAreaTag)
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return nil, "", err
		}
		totalMessages = len(messages) // Total for the reader loop is just the new ones
		currentMessageIndex = 0       // Start at the first new message
	} else {
		// 4b. No New Messages - Prompt for specific message
		readingNewMessages = false // <<< SET FLAG
		log.Printf("INFO: Node %d: No new messages in area %d since ID '%s'. Prompting for specific message.", nodeNumber, currentAreaID, lastReadID)
		noNewMsg := fmt.Sprintf("\r\n|07No new messages in area |15%s|07.", currentAreaTag)
		totalMsg := fmt.Sprintf(" |07Total messages: |15%d|07.", totalMessageCount)
		promptMsg := fmt.Sprintf("\r\n|07Read message # (|151-%d|07, |15Enter|07=Cancel): |15", totalMessageCount)

		terminal.DisplayContent([]byte(noNewMsg+totalMsg))
		terminal.DisplayContent([]byte(promptMsg))

		input, readErr := terminal.ReadLine()
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during message number input.", nodeNumber)
				return nil, "LOGOFF", io.EOF // Signal logoff
			}
			log.Printf("ERROR: Node %d: Failed reading message number input: %v", nodeNumber, readErr)
			return nil, "", readErr // Return error
		}

		selectedNumStr := strings.TrimSpace(input)
		if selectedNumStr == "" {
			log.Printf("INFO: Node %d: User cancelled message reading.", nodeNumber)
			return nil, "", nil // Return to menu
		}

		selectedNum, parseErr := strconv.Atoi(selectedNumStr)
		if parseErr != nil || selectedNum < 1 || selectedNum > totalMessageCount {
			log.Printf("WARN: Node %d: Invalid message number input: '%s'", nodeNumber, selectedNumStr)
			msg := fmt.Sprintf("\r\n|01Invalid message number: %s|07\r\n", selectedNumStr)
			terminal.DisplayContent([]byte(msg))
			time.Sleep(1 * time.Second)
			return nil, "", nil // Return to menu
		}

		// Load all messages for reading the specific one
		log.Printf("DEBUG: Node %d: Loading all %d messages to read #%d", nodeNumber, totalMessageCount, selectedNum)
		allMessages, err := e.MessageMgr.GetMessagesForArea(currentAreaID, "") // Load all
		if err != nil {
			log.Printf("ERROR: Node %d: Failed to load all messages for area %d: %v", nodeNumber, currentAreaID, err)
			msg := fmt.Sprintf("\r\n|01Error loading messages for area %s.|07\r\n", currentAreaTag)
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return nil, "", err
		}
		messages = allMessages                // Use all messages for the reader
		totalMessages = len(messages)         // Total for loop is all messages
		currentMessageIndex = selectedNum - 1 // Set index to the chosen message
	}

	// --- NEW LOGIC END ---

	// 4. Load Templates (Load once before loop)
	templateDir := filepath.Join(e.MenuSetPath, "templates")
	headTemplatePath := filepath.Join(templateDir, "MSGHEAD.TPL")
	promptTemplatePath := filepath.Join(templateDir, "MSGREAD.PROMPT")

	headTemplateBytes, errHead := os.ReadFile(headTemplatePath)
	promptTemplateBytes, errPrompt := os.ReadFile(promptTemplatePath)

	if errHead != nil || errPrompt != nil {
		log.Printf("ERROR: Node %d: Failed to load MSGHEAD/MSGREAD templates: Head(%v), Prompt(%v)", nodeNumber, errHead, errPrompt)
		msg := "\r\n|01Error loading message display templates.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading message templates")
	}
	processedHeadTemplate := string(terminal.ProcessPipeCodes(headTemplateBytes))
	processedPromptTemplate := string(terminal.ProcessPipeCodes(promptTemplateBytes))

	// 5. Main Reader Loop
readerLoop:
	for {
		// --- Get Terminal Dimensions ---
		termWidth := 80  // Default width
		termHeight := 24 // Default height
		ptyReq, _, isPty := s.Pty()
		if isPty && ptyReq.Window.Width > 0 && ptyReq.Window.Height > 0 {
			termWidth = ptyReq.Window.Width
			termHeight = ptyReq.Window.Height
			log.Printf("DEBUG: [runReadMsgs] Terminal dimensions: %d x %d", termWidth, termHeight)
		} else {
			log.Printf("WARN: [runReadMsgs] Could not get PTY dimensions, using default %d x %d", termWidth, termHeight)
		}

		// --- Calculate Available Height (Estimate) ---
		// Count lines in header and prompt templates (simple newline count)
		headerLines := strings.Count(processedHeadTemplate, "\n") + 1 // +1 because template might not end with newline
		promptLines := strings.Count(processedPromptTemplate, "\n") + 1
		// Add extra lines for spacing (e.g., CRLF before body, CRLF before prompt)
		headerAndPromptLines := headerLines + promptLines + 2
		bodyAvailableHeight := termHeight - headerAndPromptLines
		if bodyAvailableHeight < 1 {
			bodyAvailableHeight = 1 // Ensure at least one line is available
		}
		log.Printf("DEBUG: [runReadMsgs] HeaderLines: %d, PromptLines: %d, Available Body Height: %d", headerLines, promptLines, bodyAvailableHeight)

		// 5.1 Get Current Message
		currentMsg := messages[currentMessageIndex]

		// --- Determine Absolute Message Number ---
		absoluteMessageNumber := 0
		if readingNewMessages {
			// Calculate the number of messages *before* the new ones
			// Note: totalMessageCount and newCount were fetched earlier
			numOldMessages := totalMessageCount - newCount
			absoluteMessageNumber = numOldMessages + 1 + currentMessageIndex // 1-based index
		} else {
			// When not reading new, currentMessageIndex is relative to the *full* list
			absoluteMessageNumber = currentMessageIndex + 1 // 1-based index
		}
		// --- End Absolute Message Number ---

		// 5.2 Create Placeholders
		placeholders := map[string]string{
			"|CA":     currentAreaTag,
			"|MNUM":   strconv.Itoa(absoluteMessageNumber), // Use absolute number
			"|MTOTAL": strconv.Itoa(totalMessageCount),     // Use total count for the area
			"|MFROM":  string(terminal.ProcessPipeCodes([]byte(currentMsg.FromUserName))),
			"|MTO":    string(terminal.ProcessPipeCodes([]byte(currentMsg.ToUserName))),
			"|MSUBJ":  string(terminal.ProcessPipeCodes([]byte(currentMsg.Subject))),
			"|MDATE":  currentMsg.PostedAt.Format(time.RFC822), // Use a standard format like RFC822
			// Add more placeholders from message struct if needed by templates
		}

		// 5.3 Substitute into Templates (Explicit Order)
		displayHeader := processedHeadTemplate
		displayPrompt := processedPromptTemplate

		// Replace known placeholders in a fixed order
		displayHeader = strings.ReplaceAll(displayHeader, "|CA", placeholders["|CA"])
		displayHeader = strings.ReplaceAll(displayHeader, "|MNUM", placeholders["|MNUM"])
		displayHeader = strings.ReplaceAll(displayHeader, "|MTOTAL", placeholders["|MTOTAL"])
		displayHeader = strings.ReplaceAll(displayHeader, "|MFROM", placeholders["|MFROM"])
		displayHeader = strings.ReplaceAll(displayHeader, "|MTO", placeholders["|MTO"])
		displayHeader = strings.ReplaceAll(displayHeader, "|MSUBJ", placeholders["|MSUBJ"])
		displayHeader = strings.ReplaceAll(displayHeader, "|MDATE", placeholders["|MDATE"])

		displayPrompt = strings.ReplaceAll(displayPrompt, "|MNUM", placeholders["|MNUM"])
		displayPrompt = strings.ReplaceAll(displayPrompt, "|MTOTAL", placeholders["|MTOTAL"])
		// Add other placeholders needed by displayPrompt here if any

		// --- Remove Old Substitution Loop ---
		// for key, val := range placeholders {
		// 	displayHeader = strings.ReplaceAll(displayHeader, key, val)
		// 	displayPrompt = strings.ReplaceAll(displayPrompt, key, val)
		// }

		// +++ DEBUG LOGGING (Keep temporarily) +++
		log.Printf("DEBUG: [runReadMsgs] Placeholders Map: %+v", placeholders)
		log.Printf("DEBUG: [runReadMsgs] Final Display Prompt String: %q", displayPrompt)
		// +++ END DEBUG LOGGING +++

		// 5.4 Process Message Body Pipe Codes
		processedBodyString := string(terminal.ProcessPipeCodes([]byte(currentMsg.Body)))
		// 5.4.1 Word Wrap the Processed Body
		wrappedBodyLines := wrapAnsiString(processedBodyString, termWidth)

		// 5.5 Display Output (with Pagination)
		wErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed clearing screen: %v", nodeNumber, wErr)
		}

		_, wErr = terminal.Write([]byte(displayHeader))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing message header: %v", nodeNumber, wErr)
		}

		// -- Display Body with Pagination --
		linesDisplayedThisPage := 0
		for lineIdx, line := range wrappedBodyLines {
			if linesDisplayedThisPage == 0 {
				// Add CRLF before the first line of the body (or first line of a new page)
				_, wErr = terminal.Write([]byte("\r\n"))
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing CRLF before body line: %v", nodeNumber, wErr)
				}
			}

			_, wErr = terminal.Write([]byte(line))
			if wErr != nil {
				log.Printf("ERROR: Node %d: Failed writing wrapped body line %d: %v", nodeNumber, lineIdx, wErr)
			}
			linesDisplayedThisPage++

			// Check if pause is needed (if page is full AND it's not the last line)
			if linesDisplayedThisPage >= bodyAvailableHeight && lineIdx < len(wrappedBodyLines)-1 {
				// Display Pause Prompt using system setting
				pausePrompt := e.LoadedStrings.PauseString
				if pausePrompt == "" {
					pausePrompt = "|07-- More -- (|15Enter|07=Continue, |15Q|07=Quit) : |15" // Basic fallback
				}
				// Add CRLF before pause prompt
				_, wErr = terminal.Write([]byte("\r\n")) // Still use WriteProcessedBytes for simple CRLF
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing CRLF before pause prompt: %v", nodeNumber, wErr)
				}

				// Process pipe codes first
				processedPausePrompt := string(terminal.ProcessPipeCodes([]byte(pausePrompt)))
				// +++ DEBUG LOGGING +++
				log.Printf("DEBUG: [runReadMsgs Pagination] OutputMode: %d (CP437=%d, UTF8=%d)", outputMode, terminalPkg.OutputModeCP437, terminalPkg.OutputModeUTF8)
				log.Printf("DEBUG: [runReadMsgs Pagination] Processed Pause Prompt Bytes (hex): %x", processedPausePrompt)
				// +++ END DEBUG LOGGING +++
				// Use the new helper function for writing the prompt
				wErr = writeProcessedStringWithManualEncoding(terminal, []byte(processedPausePrompt), outputMode)
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing pause prompt with manual encoding: %v", nodeNumber, wErr)
				}

				// Wait for input
				bufioReader := bufio.NewReader(s)
				pauseInputRune, _, pauseErr := bufioReader.ReadRune()
				if pauseErr != nil {
					if errors.Is(pauseErr, io.EOF) {
						log.Printf("INFO: Node %d: User disconnected during message pagination pause.", nodeNumber)
						return nil, "LOGOFF", io.EOF
					}
					log.Printf("ERROR: Node %d: Failed reading pagination pause input: %v", nodeNumber, pauseErr)
					// Decide how to handle read error - maybe quit the reader?
					break readerLoop // Exit reader on error
				}

				// Clear the pause prompt line before continuing
				// Assuming prompt is on the last line. Move up, clear line, move back down?
				// Or just rely on the next screen clear? Simpler: rely on next screen clear.
				// For now, just write a CR to go to beginning of line for next write.
				_, wErr = terminal.Write([]byte("\r"))
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing CR after pause prompt: %v", nodeNumber, wErr)
				}

				if unicode.ToUpper(pauseInputRune) == 'Q' {
					log.Printf("DEBUG: Node %d: User quit message reader during pagination.", nodeNumber)
					// --- Update Last Read only if reading new ---
					if readingNewMessages && len(messages) > 0 {
						lastViewedID := messages[currentMessageIndex].ID.String()
						log.Printf("DEBUG: Node %d: Updating last read ID (pagination quit) for user %s, area %d to %s", nodeNumber, currentUser.Handle, currentAreaID, lastViewedID)
						if currentUser.LastReadMessageIDs == nil {
							currentUser.LastReadMessageIDs = make(map[int]string)
						}
						currentUser.LastReadMessageIDs[currentAreaID] = lastViewedID
						if err := userManager.SaveUsers(); err != nil {
							log.Printf("ERROR: Node %d: Failed to save user data (pagination quit): %v", nodeNumber, err)
						}
					}
					// --- End Update ---
					break readerLoop // Exit outer reader loop
				}
				// Otherwise (Enter, Space, etc.), reset counter and continue display loop
				linesDisplayedThisPage = 0
			} else if lineIdx < len(wrappedBodyLines)-1 {
				// Add CRLF after lines that are not the last line and didn't trigger a pause
				_, wErr = terminal.Write([]byte("\r\n"))
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing CRLF after body line: %v", nodeNumber, wErr)
				}
			}
		} // -- End Body Display Loop --

		// Write prompt (add CRLF before)
		_, wErr = terminal.Write([]byte("\r\n"))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing CRLF before prompt: %v", nodeNumber, wErr)
		}
		_, wErr = terminal.Write([]byte(displayPrompt))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing message prompt: %v", nodeNumber, wErr)
		}

		// 6. Input Handling (Main Reader Commands)
		input, err := terminal.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during message reading.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading message reader input: %v", nodeNumber, err)
			// Maybe break loop on error? Or just retry?
			terminal.Write([]byte("\r\n|01Error reading input.|07\r\n"))
			time.Sleep(1 * time.Second)
			continue // Retry reading input
		}

		upperInput := strings.ToUpper(strings.TrimSpace(input))

		switch upperInput {
		case "", " ": // Next message (Enter or Space defaults to Next)
			if currentMessageIndex < totalMessages-1 {
				currentMessageIndex++
			} else {
				// Optionally wrap around or stay at last message
				// For now, stay at last message and indicate
				terminal.Write([]byte("\r\n|07Last message.|07"))
				time.Sleep(500 * time.Millisecond)
			}
		case "N": // Explicit 'N' - Do nothing now, or maybe show help?
			// Currently does nothing, just loops back
			log.Printf("DEBUG: Node %d: 'N' pressed, doing nothing.", nodeNumber)
		case "P": // Previous message
			if currentMessageIndex > 0 {
				currentMessageIndex--
			} else {
				// TODO: Display "First message" indicator?
				terminal.Write([]byte("\r\n|07First message.|07"))
				time.Sleep(500 * time.Millisecond)
			}
		case "Q":
			log.Printf("DEBUG: Node %d: User quit message reader ('Q' pressed).", nodeNumber)
			// --- Update Last Read only if reading new ---
			if readingNewMessages && len(messages) > 0 {
				lastViewedID := messages[currentMessageIndex].ID.String()
				log.Printf("DEBUG: Node %d: Updating last read ID ('Q') for user %s, area %d to %s", nodeNumber, currentUser.Handle, currentAreaID, lastViewedID)
				if currentUser.LastReadMessageIDs == nil {
					currentUser.LastReadMessageIDs = make(map[int]string)
				}
				currentUser.LastReadMessageIDs[currentAreaID] = lastViewedID
				if err := userManager.SaveUsers(); err != nil {
					log.Printf("ERROR: Node %d: Failed to save user data ('Q'): %v", nodeNumber, err)
				}
			} else {
				log.Printf("DEBUG: Node %d: Skipping last read ID update because readingNewMessages is false or no messages.", nodeNumber)
			}
			// --- End Update ---
			break readerLoop // Exit the labeled loop
		case "R": // Reply
			// 1. Get Quote Prefix from config
			quotePrefix := e.LoadedStrings.QuotePrefix // Direct access
			if quotePrefix == "" {                     // Default if empty
				quotePrefix = "> "
			}

			// 2. Format quoted text
			quotedBody := formatQuote(&currentMsg, quotePrefix)

			// 3. Run Editor
			terminal.Write([]byte("\r\nLaunching editor...\r\n"))

			// Get TERM env var
			termType := "vt100" // Default TERM
			for _, env := range s.Environ() {
				if strings.HasPrefix(env, "TERM=") {
					termType = strings.TrimPrefix(env, "TERM=")
					break
				}
			}
			log.Printf("DEBUG: Node %d: Launching editor with TERM=%s", nodeNumber, termType)

			// === START: Moved Subject Logic ===
			// 4. Get Subject
			defaultSubject := generateReplySubject(currentMsg.Subject)
			subjectPromptStr := e.LoadedStrings.MsgTitleStr // Direct access
			if subjectPromptStr == "" {
				subjectPromptStr = "|08[|15Title|08] : " // Default if empty
			}
			// Process pipe codes using correct function (assuming ReplacePipeCodes)
			subjectPromptBytes := terminal.ProcessPipeCodes([]byte(subjectPromptStr)) // Use ReplacePipeCodes

			// Display the prompt (clear line first?)
			// TODO: Consider clearing a specific line if layout is fixed
			terminal.Write([]byte("\r\n")) // Add newline before prompt
			terminal.Write(subjectPromptBytes)
			terminal.Write([]byte(defaultSubject))

			// Read the input using standard ReadLine
			rawInput, err := terminal.ReadLine()
			if err != nil {
				if errors.Is(err, io.EOF) {
					log.Printf("INFO: Node %d: User disconnected during subject input.", nodeNumber)
					return nil, "LOGOFF", io.EOF // Signal logoff
				}
				log.Printf("ERROR: Node %d: Failed getting subject input: %v", nodeNumber, err)
				terminal.Write([]byte("\r\nError getting subject.\r\n"))
				time.Sleep(1 * time.Second)
				continue // Abort reply, redraw reader
			}
			newSubject := strings.TrimSpace(rawInput)
			if newSubject == "" {
				newSubject = defaultSubject
			}
			// Ensure subject is not completely empty after defaulting
			if strings.TrimSpace(newSubject) == "" {
				terminal.Write([]byte("\r\nSubject cannot be empty. Reply cancelled.\r\n"))
				time.Sleep(1 * time.Second)
				continue // Abort reply, redraw reader
			}
			// === END: Moved Subject Logic ===

			// Call editor with correct signature
			replyBody, saved, editErr := editor.RunEditor(quotedBody, s, s, termType)

			if editErr != nil {
				log.Printf("ERROR: Node %d: Editor failed: %v", nodeNumber, editErr)
				terminal.Write([]byte("\r\nEditor encountered an error.\r\n"))
				time.Sleep(2 * time.Second)
				// Screen needs redraw - continue will force it
				continue
			}

			if !saved {
				terminal.Write([]byte("\r\nReply cancelled.\r\n"))
				time.Sleep(1 * time.Second)
				// Screen needs redraw - continue will force it
				continue
			}

			// 5. Construct Message (Subject 'newSubject' already obtained)
			replyMsg := message.Message{
				ID:           uuid.New(), // Use uuid.New()
				AreaID:       currentAreaID,
				FromUserName: currentUser.Handle,
				ToUserName:   currentMsg.FromUserName, // Reply goes back to original sender
				Subject:      newSubject,              // Use subject from before editor
				Body:         replyBody,
				PostedAt:     time.Now(),    // Use PostedAt field
				ReplyToID:    currentMsg.ID, // Link to the original message
			}

			// 6. Save Message
			err = e.MessageMgr.AddMessage(currentAreaID, replyMsg) // Pass value, not pointer
			if err != nil {
				log.Printf("ERROR: Node %d: Failed to save reply message: %v", nodeNumber, err)
				terminal.Write([]byte("\r\nError saving reply message.\r\n"))
				time.Sleep(2 * time.Second)
			} else {
				terminal.Write([]byte("\r\nReply posted successfully!\r\n"))
				time.Sleep(1 * time.Second)
				// --- Update Counters and Slice ---
				totalMessages++         // Increment count of messages loaded in the *current slice*
				totalMessageCount++     // Increment the *overall* area count
				if readingNewMessages { // <<< ADDED Check
					newCount++ // <<< ADDED: Increment newCount if we started by reading new
				}
				messages = append(messages, replyMsg) // Add reply to the *current slice*
				// --- End Update ---

				// -->> Advance to the next message after replying <<--
				if currentMessageIndex < totalMessages-1 { // Check bounds just in case, though append should make it safe
					currentMessageIndex++
				}
			}
			continue

		default:
			// Optional: Display invalid command message
			log.Printf("DEBUG: Node %d: Invalid command '%s' in message reader.", nodeNumber, upperInput)
			terminal.Write([]byte("\r\n|01Invalid command.|07"))
			time.Sleep(500 * time.Millisecond)
		}
		// continue loop implicitly unless 'Q' was pressed
	}

	// 7. Return (after loop breaks)
	log.Printf("DEBUG: Node %d: Exiting message reader function.", nodeNumber)
	return nil, "", nil // Return to MSGMENU
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
func writeProcessedStringWithManualEncoding(terminal *terminalPkg.BBS, processedBytes []byte, outputMode terminalPkg.OutputMode) error {
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
		if outputMode == terminalPkg.OutputModeCP437 {
			if r < 128 {
				// ASCII character, write directly
				finalBuf.WriteByte(byte(r))
			} else if cp437Byte, ok := terminalPkg.UnicodeToCP437Table[r]; ok {
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
	_, err := terminal.Write(finalBuf.Bytes())
	return err
}

// runNewscan checks all accessible message areas for new messages.
func runNewscan(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running NEWSCAN for user %s", nodeNumber, currentUser.Handle)

	if currentUser == nil {
		log.Printf("WARN: Node %d: NEWSCAN called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to scan for new messages.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	allAreas := e.MessageMgr.ListAreas()
	// Store results as a slice of structs to maintain order and store counts
	type areaResult struct {
		Tag   string
		Count int
	}
	var resultsWithNew []areaResult // Changed variable name

	log.Printf("DEBUG: Node %d: Scanning %d total areas for new messages.", nodeNumber, len(allAreas))

	for _, area := range allAreas {
		if !checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
			log.Printf("TRACE: Node %d: Skipping area %d ('%s') due to read ACS '%s'", nodeNumber, area.ID, area.Tag, area.ACSRead)
			continue
		}

		lastReadID := ""
		if currentUser.LastReadMessageIDs != nil {
			lastReadID = currentUser.LastReadMessageIDs[area.ID]
		}

		// Get the count of new messages (Corrected function call)
		newCount, err := e.MessageMgr.GetNewMessageCount(area.ID, lastReadID)
		if err != nil {
			log.Printf("ERROR: Node %d: Error checking new message count in area %d ('%s'): %v", nodeNumber, area.ID, area.Tag, err)
			continue // Skip area on error
		}

		if newCount > 0 {
			log.Printf("DEBUG: Node %d: Found %d new messages in area %d ('%s')", nodeNumber, newCount, area.ID, area.Tag)
			resultsWithNew = append(resultsWithNew, areaResult{Tag: area.Tag, Count: newCount}) // Correctly append
		}
	}

	// 5. Display Results & Handle Actions
	var outputBuffer bytes.Buffer
	outputBuffer.WriteString("\r\n") // Start with a newline

	if len(resultsWithNew) == 0 { // Use corrected variable name
		// No new messages found
		outputBuffer.WriteString("|07No new messages found in any accessible areas.\r\n")
		log.Printf("INFO: Node %d: Newscan completed, no new messages found.", nodeNumber)

		// Write the result message
		wErr := terminal.DisplayContent(outputBuffer.Bytes())
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing NEWSCAN (no new) results: %v", nodeNumber, wErr)
		}
		// Fall through to standard pause prompt

	} else {
		// New messages found - build summary and ask to read
		var summaryParts []string
		for _, res := range resultsWithNew { // Use corrected variable name
			summaryParts = append(summaryParts, fmt.Sprintf("%s(%d)", res.Tag, res.Count))
		}
		summary := strings.Join(summaryParts, ", ")
		outputBuffer.WriteString(fmt.Sprintf("|07New messages: |15%s|07\r\n", summary))
		log.Printf("INFO: Node %d: Newscan completed, new messages found: %s", nodeNumber, summary)

		// Write the summary message first
		wErr := terminal.DisplayContent(outputBuffer.Bytes())
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing NEWSCAN summary results: %v", nodeNumber, wErr)
			// Fall through to pause anyway?
		}

		// Ask the user if they want to read
		// TODO: Use configurable string
		readNow, err := e.promptYesNoLightbar(s, terminal, "Read new messages now?", outputMode, nodeNumber)
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during NEWSCAN read prompt.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Failed getting Yes/No input for NEWSCAN read: %v", err)
			// Don't pause if prompt failed, just return to menu
			return nil, "", err
		}

		if readNow {
			// User wants to read - jump to the first area with new messages
			firstAreaTag := resultsWithNew[0].Tag // Use corrected variable name
			log.Printf("DEBUG: Node %d: User chose to read. Jumping to first area: %s", nodeNumber, firstAreaTag)

			// Get the Area ID for the tag
			firstArea, found := e.MessageMgr.GetAreaByTag(firstAreaTag)
			if !found {
				// Should not happen if ListAreas and GetNewMessageCount worked
				log.Printf("ERROR: Node %d: Could not find area details for tag '%s' after newscan.", nodeNumber, firstAreaTag)
				// Fall through to pause prompt as a fallback
			} else {
				// Update user's current area
				currentUser.CurrentMessageAreaID = firstArea.ID
				currentUser.CurrentMessageAreaTag = firstArea.Tag
				log.Printf("INFO: Node %d: Set current message area for user %s to %d (%s)", nodeNumber, currentUser.Handle, firstArea.ID, firstArea.Tag)

				// Save the user state *before* calling runReadMsgs
				if err := userManager.SaveUsers(); err != nil {
					log.Printf("ERROR: Node %d: Failed to save user data after setting current area in newscan: %v", nodeNumber, err)
					// Proceed anyway, but state might be lost
				}

				// Directly call runReadMsgs. It will handle its own loop and return.
				return runReadMsgs(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode)
			}
		} else {
			// User chose not to read now - fall through to standard pause
			log.Printf("DEBUG: Node %d: User chose not to read new messages now.", nodeNumber)
		}
	}

	// 6. Wait for Enter (Standard Pause - only reached if no new messages or user chose 'No')
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... " // Fallback
	}

	var pauseBytesToWrite []byte
	processedPausePrompt := string(terminal.ProcessPipeCodes([]byte(pausePrompt)))
	if outputMode == terminalPkg.OutputModeCP437 {
		var cp437Buf bytes.Buffer
		for _, r := range string(processedPausePrompt) {
			if r < 128 {
				cp437Buf.WriteByte(byte(r))
			} else if cp437Byte, ok := terminalPkg.UnicodeToCP437Table[r]; ok {
				cp437Buf.WriteByte(cp437Byte)
			} else {
				cp437Buf.WriteByte('?')
			}
		}
		pauseBytesToWrite = cp437Buf.Bytes()
	} else {
		pauseBytesToWrite = []byte(processedPausePrompt)
	}

	_, wErr := terminal.Write(pauseBytesToWrite)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing NEWSCAN pause prompt: %v", nodeNumber, wErr)
	}

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during NEWSCAN pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading input during NEWSCAN pause: %v", nodeNumber, err)
			return nil, "", err
		}
		if r == '\r' || r == '\n' { // Check for CR or LF
			break
		}
	}

	return nil, "", nil // Return to the current menu (MSGMENU)
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
func formatQuote(originalMsg *message.Message, quotePrefix string) string {
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
func runListFiles(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running LISTFILES", nodeNumber)

	// 1. Check User and Current File Area
	if currentUser == nil {
		log.Printf("WARN: Node %d: LISTFILES called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to list files.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
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
		wErr := terminal.DisplayContent([]byte(msg))
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
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading FILELIST templates")
	}

	processedTopTemplate := string(terminal.ProcessPipeCodes(topTemplateBytes))
	processedMidTemplate := string(terminal.ProcessPipeCodes(midTemplateBytes))
	processedBotTemplate := string(terminal.ProcessPipeCodes(botTemplateBytes))

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
	headerLines := bytes.Count([]byte(processedTopTemplate), []byte("\n")) + 1
	footerLines := bytes.Count([]byte(processedBotTemplate), []byte("\n")) + 1
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
		wErr := terminal.DisplayContent([]byte(msg))
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
			wErr := terminal.DisplayContent([]byte(msg))
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
		writeErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
		if writeErr != nil {
			log.Printf("ERROR: Node %d: Failed clearing screen for LISTFILES: %v", nodeNumber, writeErr)
			// Potentially return error or try to continue
		}

		// 4.2 Display Top Template
		_, wErr := terminal.Write([]byte(processedTopTemplate))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing LISTFILES top template: %v", nodeNumber, wErr)
			// Handle error
		}
		// Add CRLF after TOP template before listing files
		_, wErr = terminal.Write([]byte("\r\n"))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing CRLF after LISTFILES top template: %v", nodeNumber, wErr)
			// Handle error
		}

		// 4.3 Display Files on Current Page (using MID template)
		if len(filesOnPage) == 0 {
			// Display "No files in this area" message
			// TODO: Use a configurable string?
			noFilesMsg := "\r\n|07   No files in this area.   \r\n"
			wErr = terminal.DisplayContent([]byte(noFilesMsg))
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
				_, wErr = terminal.Write([]byte("\r\n"))
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
		_, wErr = terminal.Write([]byte(bottomLine))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing LISTFILES bottom template: %v", nodeNumber, wErr)
			// Handle error
		}

		// 4.5 Display Prompt (Use a standard file list prompt or configure one)
		// TODO: Use configurable prompt string
		prompt := "\r\n|07File Cmd (|15N|07=Next, |15P|07=Prev, |15#|07=Mark, |15D|07=Download, |15Q|07=Quit): |15"
		wErr = terminal.DisplayContent([]byte(prompt))
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
				terminal.Write([]byte("\r\n|07Already on last page.|07"))
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
				terminal.Write([]byte("\r\n|07Already on first page.|07"))
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
				wErr := terminal.DisplayContent([]byte(msg))
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
			terminal.Write([]byte(terminalPkg.SaveCursor()))
			terminal.Write([]byte("\\r\\n\\x1b[K")) // Newline, clear line

			proceed, err := e.promptYesNoLightbar(s, terminal, confirmPrompt, outputMode, nodeNumber)
			terminal.Write([]byte(terminalPkg.RestoreCursor())) // Restore cursor after prompt

			if err != nil {
				if errors.Is(err, io.EOF) {
					log.Printf("INFO: Node %d: User disconnected during download confirmation.", nodeNumber)
					return nil, "LOGOFF", io.EOF
				}
				log.Printf("ERROR: Node %d: Error getting download confirmation: %v", nodeNumber, err)
				msg := "\\r\\n|01Error during confirmation.|07\\r\\n"
				terminal.DisplayContent([]byte(msg))
				time.Sleep(1 * time.Second)
				continue // Back to file list
			}

			if !proceed {
				log.Printf("DEBUG: Node %d: User cancelled download.", nodeNumber)
				terminal.Write([]byte("\\r\\n|07Download cancelled.|07"))
				time.Sleep(500 * time.Millisecond)
				continue // Back to file list
			}

			// 3. Process downloads
			log.Printf("INFO: Node %d: User %s starting download of %d files.", nodeNumber, currentUser.Handle, len(currentUser.TaggedFileIDs))
			terminal.Write([]byte("\\r\\n|07Preparing download...\\r\\n"))
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
				terminal.Write([]byte("|15Initiating ZMODEM transfer...\\r\\n"))
				terminal.Write([]byte("|07Please start the ZMODEM receive function in your terminal.\\r\\n"))

				// Use the unified protocol manager for ZMODEM transfer
				pm := transfer.NewProtocolManager()
				transferErr := pm.SendFiles(s, "ZMODEM", filesToDownload...)

				// Handle Result
				if transferErr != nil {
					// Transfer failed or was cancelled
					log.Printf("ERROR: Node %d: ZMODEM transfer failed: %v", nodeNumber, transferErr)
					terminal.Write([]byte("|01ZMODEM transfer failed or was cancelled.\\r\\n"))
					failCount = len(filesToDownload) // Assume all failed if transfer returns error
					successCount = 0
				} else {
					// Transfer completed successfully
					log.Printf("INFO: Node %d: ZMODEM transfer completed successfully.", nodeNumber)
					terminal.Write([]byte("|07ZMODEM transfer complete.\\r\\n"))
					successCount = len(filesToDownload) // Assume all succeeded if transfer exits cleanly
					failCount = 0

					// Increment download counts only on successful transfer completion
					for _, fileID := range currentUser.TaggedFileIDs {
						// Check again if we had a valid path originally
						if _, pathErr := e.FileMgr.GetFilePath(fileID); pathErr == nil {
							if err := e.FileMgr.IncrementDownloadCount(fileID); err != nil {
								log.Printf("WARN: Node %d: Failed to increment download count for file %s after successful transfer: %v", nodeNumber, fileID, err)
							} else {
								log.Printf("DEBUG: Node %d: Incremented download count for file %s after successful transfer.", nodeNumber, fileID)
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
				terminal.DisplayContent([]byte(msg))
				failCount = len(currentUser.TaggedFileIDs) // Mark all as failed if none were found
			}

			// 4. Clear tags and save user state
			log.Printf("DEBUG: Node %d: Clearing %d tagged file IDs for user %s.", nodeNumber, len(currentUser.TaggedFileIDs), currentUser.Handle)
			currentUser.TaggedFileIDs = nil // Clear the list
			if err := userManager.SaveUsers(); err != nil {
				log.Printf("ERROR: Node %d: Failed to save user data after download attempt: %v", nodeNumber, err)
				// Inform user? State might be inconsistent.
				terminal.Write([]byte("\\r\\n|01Error saving user state after download.|07"))
			}

			// 5. Final status message
			statusMsg := fmt.Sprintf("|07Download attempt finished. Success: %d, Failed: %d.|07\r\n", successCount, failCount)
			terminal.Write([]byte(statusMsg))
			time.Sleep(2 * time.Second)

			// Go back to the file list (will redraw with cleared marks)
			continue
		case "U": // Upload (Placeholder)
			log.Printf("DEBUG: Node %d: Upload command entered (Not Implemented)", nodeNumber)
			msg := "\r\n|01Upload function not yet implemented.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			// Stay on the same page
		case "V": // View (Placeholder)
			log.Printf("DEBUG: Node %d: View command entered (Not Implemented)", nodeNumber)
			msg := "\r\n|01View function not yet implemented.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			// Stay on the same page
		case "A": // Area Change (Placeholder/Not implemented here, handled by menu?)
			log.Printf("DEBUG: Node %d: Area Change command entered (Handled by menu)", nodeNumber)
			msg := "\r\n|01Use menu options to change area.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
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
func displayFileAreaList(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, currentUser *user.User, outputMode terminalPkg.OutputMode, nodeNumber int, sessionStartTime time.Time) error {
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
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return fmt.Errorf("failed loading FILEAREA templates")
	}

	// 3. Process Pipe Codes in Templates FIRST
	processedTopTemplate := string(terminal.ProcessPipeCodes(topTemplateBytes))
	processedMidTemplate := string(terminal.ProcessPipeCodes(midTemplateBytes))
	processedBotTemplate := string(terminal.ProcessPipeCodes(botTemplateBytes))

	// 4. Get file area list data from FileManager
	areas := e.FileMgr.ListAreas()

	// 5. Build the output string using processed templates and data
	var outputBuffer bytes.Buffer
	outputBuffer.Write([]byte(processedTopTemplate)) // Write processed top template

	areasDisplayed := 0
	if len(areas) > 0 {
		for _, area := range areas {
			if !checkACS(area.ACSList, currentUser, s, terminal, sessionStartTime) {
				log.Printf("TRACE: Node %d: User %s denied list access to file area %d (%s) due to ACS '%s'", nodeNumber, currentUser.Handle, area.ID, area.Tag, area.ACSList)
				continue
			}

			line := processedMidTemplate
			name := string(terminal.ProcessPipeCodes([]byte(area.Name)))
			desc := string(terminal.ProcessPipeCodes([]byte(area.Description)))
			idStr := strconv.Itoa(area.ID)
			tag := string(terminal.ProcessPipeCodes([]byte(area.Tag)))
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

	outputBuffer.Write([]byte(processedBotTemplate))

	// 6. Clear screen and display the assembled content
	writeErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
	if writeErr != nil {
		log.Printf("ERROR: Node %d: Failed clearing screen for file area list: %v", nodeNumber, writeErr)
		// Try to continue anyway?
	}

	processedContent := outputBuffer.Bytes()
	_, wErr := terminal.Write(processedContent)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing file area list output: %v", nodeNumber, wErr)
		return wErr // Return the error from writing
	}

	return nil // Success
}

// runListFileAreas displays a list of file areas using templates.
func runListFileAreas(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running LISTFILEAR", nodeNumber)

	if currentUser == nil {
		log.Printf("WARN: Node %d: LISTFILEAR called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to list file areas.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
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
	processedPausePrompt := string(terminal.ProcessPipeCodes([]byte(pausePrompt)))
	if outputMode == terminalPkg.OutputModeCP437 {
		var cp437Buf bytes.Buffer
		for _, r := range string(processedPausePrompt) {
			if r < 128 {
				cp437Buf.WriteByte(byte(r))
			} else if cp437Byte, ok := terminalPkg.UnicodeToCP437Table[r]; ok {
				cp437Buf.WriteByte(cp437Byte)
			} else {
				cp437Buf.WriteByte('?')
			}
		}
		pauseBytesToWrite = cp437Buf.Bytes()
	} else {
		pauseBytesToWrite = []byte(processedPausePrompt)
	}

	_, wErr := terminal.Write(pauseBytesToWrite)
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
func runSelectFileArea(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running SELECTFILEAREA", nodeNumber)

	if currentUser == nil {
		log.Printf("WARN: Node %d: SELECTFILEAREA called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to select a file area.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
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
	terminal.Write([]byte("\r\n"))

	// Prompt for area tag
	prompt := e.LoadedStrings.ChangeFileAreaStr
	if prompt == "" {
		prompt = "|07File Area Tag (?=List, Q=Quit): |15" // Updated prompt slightly
	}
	wErr := terminal.DisplayContent([]byte(prompt))
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
		terminal.Write([]byte("\r\n")) // Newline after abort
		return currentUser, "", nil    // Return to previous menu
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
			wErr := terminal.DisplayContent([]byte(msg))
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
			wErr := terminal.DisplayContent([]byte(msg))
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
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return currentUser, "", nil // Return to menu
	}

	// Success! Update user state
	currentUser.CurrentFileAreaID = area.ID
	currentUser.CurrentFileAreaTag = area.Tag

	// Save the user state (important!)
	if err := userManager.SaveUsers(); err != nil {
		log.Printf("ERROR: Node %d: Failed to save user data after updating file area: %v", nodeNumber, err)
		// Should we inform the user? Proceed anyway?
		msg := "\r\n|01Error: Could not save area selection.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		// Don't change area if save failed?
		// Or let it proceed and hope for next save?
		// For now, proceed but log the error.
	}

	log.Printf("INFO: Node %d: User %s changed file area to ID %d ('%s')", nodeNumber, currentUser.Handle, area.ID, area.Tag)
	msg := fmt.Sprintf("\r\n|07Current file area set to: |15%s|07\r\n", area.Name)                  // Use area name for confirmation
	wErr = terminal.DisplayContent([]byte(msg)) // <-- Use = instead of :=
	if wErr != nil {                                                                                /* Log? */
	}
	time.Sleep(1 * time.Second)

	return currentUser, "", nil // Success, return to previous menu/state
}
