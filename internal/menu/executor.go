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
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/file"
	"github.com/stlalpha/vision3/internal/menu/renderer"
	"github.com/stlalpha/vision3/internal/message"
	terminalPkg "github.com/stlalpha/vision3/internal/terminal"
	"github.com/stlalpha/vision3/internal/types"
	"github.com/stlalpha/vision3/internal/user"
)

// Mutex for protecting access to the oneliners file
var onelinerMutex sync.Mutex

// RunnableFunc is defined in registry.go

// AutoRunTracker definition removed, using the one from types.go

// MenuExecutor handles the loading and execution of ViSiON/2 menus.
type MenuExecutor struct {
	ConfigPath       string                       // DEPRECATED: Use MenuSetPath + "/cfg" or RootConfigPath
	AssetsPath       string                       // DEPRECATED: Use MenuSetPath + "/ansi" or RootAssetsPath
	MenuSetPath      string                       // NEW: Path to the active menu set (e.g., "menus/v3")
	RootConfigPath   string                       // NEW: Path to global configs (e.g., "configs")
	RootAssetsPath   string                       // NEW: Path to global assets (e.g., "assets")
	RunRegistry      map[string]RunnableFunc      // Map RUN: targets to functions (Use local RunnableFunc)
	DoorRegistry     map[string]config.DoorConfig // Map DOOR: targets to configurations
	OneLiners        []string                     // Loaded oneliners (Consider if these should be menu-set specific)
	LoadedStrings    config.StringsConfig         // Loaded global strings configuration
	Theme            config.ThemeConfig           // Loaded theme configuration
	MessageMgr       *message.MessageManager      // <-- ADDED FIELD
	FileMgr          *file.FileManager            // <-- ADDED FIELD: File manager instance
	Renderer         *renderer.Engine             // Programmatic menu renderer
	RendererConfig   renderer.Config              // Active renderer configuration
	RendererSettings config.MenuRendererConfig    // Persisted renderer settings
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

	rendererSettings, err := config.LoadMenuRendererConfig(rootConfigPath)
	if err != nil {
		log.Printf("WARN: Falling back to default menu renderer config: %v", err)
		rendererSettings = config.DefaultMenuRendererConfig()
	}
	rendererSettings.Normalise()
	renderEngineConfig := rendererConfigFromSettings(rendererSettings)
	renderEngine := renderer.NewEngine(renderEngineConfig)

	return &MenuExecutor{
		MenuSetPath:      menuSetPath,    // Store path to active menu set
		RootConfigPath:   rootConfigPath, // Store path to global configs
		RootAssetsPath:   rootAssetsPath, // Store path to global assets
		RunRegistry:      runRegistry,
		DoorRegistry:     doorRegistry,
		OneLiners:        oneLiners,     // Store loaded oneliners
		LoadedStrings:    loadedStrings, // Store loaded strings
		Theme:            theme,         // Store loaded theme
		MessageMgr:       msgMgr,        // <-- ASSIGN FIELD
		FileMgr:          fileMgr,       // <-- ASSIGN FIELD
		Renderer:         renderEngine,
		RendererConfig:   renderEngineConfig,
		RendererSettings: rendererSettings,
	}
}

// registerPlaceholderRunnables and registerAppRunnables are in registry.go


func rendererConfigFromSettings(settings config.MenuRendererConfig) renderer.Config {
	cfg := renderer.Config{
		Enabled:           settings.Enable,
		DefaultTheme:      strings.ToLower(strings.TrimSpace(settings.DefaultTheme)),
		Palette:           strings.ToLower(strings.TrimSpace(settings.Palette)),
		Codepage:          strings.ToLower(strings.TrimSpace(settings.Codepage)),
		AllowExternalAnsi: settings.AllowExternalAnsi,
		MenuOverrides:     make(map[string]renderer.Override, len(settings.MenuOverrides)),
	}
	for key, override := range settings.MenuOverrides {
		o := renderer.Override{
			Mode:     strings.ToLower(strings.TrimSpace(override.Mode)),
			Theme:    strings.ToLower(strings.TrimSpace(override.Theme)),
			Palette:  strings.ToLower(strings.TrimSpace(override.Palette)),
			Codepage: strings.ToLower(strings.TrimSpace(override.Codepage)),
		}
		cfg.MenuOverrides[key] = o
	}
	return cfg
}

func (e *MenuExecutor) refreshRendererEngine() {
	e.RendererSettings.Normalise()
	e.RendererConfig = rendererConfigFromSettings(e.RendererSettings)
	e.Renderer = renderer.NewEngine(e.RendererConfig)
}

func runSetRender(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	if currentUser == nil || currentUser.AccessLevel < 200 {
		msg := "\r\n|01Access denied. Sysop privileges required.|07\r\n"
		terminal.DisplayContent([]byte(msg))
		time.Sleep(time.Second)
		return currentUser, "", nil
	}

	params := parseKeyValueArgs(args)
	theme := params["theme"]
	palette := params["palette"]
	codepage := params["codepage"]

	validThemes := []string{"visionx", "phosphor"}
	validPalettes := []string{"amiga", "ibm_pc", "c64", "phosphor"}
	validCodepages := []string{"utf8", "amiga_topaz", "ibm_pc", "c64_petscii"}

	if theme == "" {
		theme = promptRendererOption(terminal, "Theme", e.RendererSettings.DefaultTheme, validThemes)
	}
	if palette == "" {
		palette = promptRendererOption(terminal, "Palette", e.RendererSettings.Palette, validPalettes)
	}
	if codepage == "" {
		codepage = promptRendererOption(terminal, "Codepage", e.RendererSettings.Codepage, validCodepages)
	}

	if theme != "" && !containsInsensitive(validThemes, theme) {
		msg := fmt.Sprintf("\r\n|01Unknown theme '%s'. Valid: %s|07\r\n", theme, strings.Join(validThemes, ", "))
		terminal.DisplayContent([]byte(msg))
		time.Sleep(time.Second)
		return currentUser, "", nil
	}
	if palette != "" && !containsInsensitive(validPalettes, palette) {
		msg := fmt.Sprintf("\r\n|01Unknown palette '%s'. Valid: %s|07\r\n", palette, strings.Join(validPalettes, ", "))
		terminal.DisplayContent([]byte(msg))
		time.Sleep(time.Second)
		return currentUser, "", nil
	}
	if codepage != "" && !containsInsensitive(validCodepages, codepage) {
		msg := fmt.Sprintf("\r\n|01Unknown codepage '%s'. Valid: %s|07\r\n", codepage, strings.Join(validCodepages, ", "))
		terminal.DisplayContent([]byte(msg))
		time.Sleep(time.Second)
		return currentUser, "", nil
	}

	changed := false
	if theme != "" {
		theme = strings.ToLower(strings.TrimSpace(theme))
		if theme != strings.ToLower(e.RendererSettings.DefaultTheme) {
			e.RendererSettings.DefaultTheme = theme
			changed = true
		}
	}
	if palette != "" {
		palette = strings.ToLower(strings.TrimSpace(palette))
		if palette != strings.ToLower(e.RendererSettings.Palette) {
			e.RendererSettings.Palette = palette
			changed = true
		}
	}
	if codepage != "" {
		codepage = strings.ToLower(strings.TrimSpace(codepage))
		if codepage != strings.ToLower(e.RendererSettings.Codepage) {
			e.RendererSettings.Codepage = codepage
			changed = true
		}
	}

	if !changed {
		terminal.DisplayContent([]byte("\r\n|07Renderer settings unchanged.|07\r\n"))
		time.Sleep(500 * time.Millisecond)
		return currentUser, "", nil
	}

	e.refreshRendererEngine()
	if err := config.SaveMenuRendererConfig(e.RootConfigPath, e.RendererSettings); err != nil {
		log.Printf("ERROR: Failed to save renderer configuration: %v", err)
		terminal.DisplayContent([]byte("\r\n|01Error writing menu renderer configuration.|07\r\n"))
		time.Sleep(time.Second)
		return currentUser, "", err
	}

	msg := fmt.Sprintf("\r\n|10Renderer updated to theme=%s palette=%s codepage=%s|07\r\n", e.RendererSettings.DefaultTheme, e.RendererSettings.Palette, e.RendererSettings.Codepage)
	terminal.DisplayContent([]byte(msg))
	time.Sleep(750 * time.Millisecond)
	return currentUser, "", nil
}

func promptRendererOption(terminal *terminalPkg.BBS, label, current string, options []string) string {
	optionList := strings.Join(options, "/")
	prompt := fmt.Sprintf("\r\n|07%s [%s] (%s): ", label, current, optionList)
	terminal.DisplayContent([]byte(prompt))
	input, err := terminal.ReadLine()
	if err != nil {
		return current
	}
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return current
	}
	return input
}

func parseKeyValueArgs(args string) map[string]string {
	result := make(map[string]string)
	fields := strings.Fields(strings.ToLower(args))
	for _, field := range fields {
		if strings.Contains(field, "=") {
			parts := strings.SplitN(field, "=", 2)
			if len(parts) == 2 {
				result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}
	return result
}

func containsInsensitive(collection []string, value string) bool {
	val := strings.ToLower(strings.TrimSpace(value))
	for _, item := range collection {
		if strings.ToLower(item) == val {
			return true
		}
	}
	return false
}

func (e *MenuExecutor) buildMenuContext(menuName string, currentUser *user.User, nodeNumber int, terminal *terminalPkg.BBS, s ssh.Session, sessionStartTime time.Time) renderer.MenuContext {
	ctx := renderer.MenuContext{
		Name: strings.ToUpper(menuName),
		User: renderer.UserInfo{
			Handle: "Guest",
			Node:   nodeNumber,
		},
		Stats: renderer.Stats{
			ActiveDoors: len(e.DoorRegistry),
			OnlineCount: 1,
			Ratio:       "100%",
		},
	}

	if currentUser != nil {
		if strings.TrimSpace(currentUser.Handle) != "" {
			ctx.User.Handle = currentUser.Handle
		}
		ctx.Stats.Ratio = computeUserRatio(currentUser)
		ctx.Stats.Uploads = currentUser.NumUploads
	}

	e.populateMessageStats(&ctx, currentUser, terminal, s, sessionStartTime)
	e.populateFileStats(&ctx, currentUser, terminal, s, sessionStartTime)

	if ctx.Stats.PrimaryMessageArea == "" && len(ctx.Stats.TopMessageAreas) > 0 {
		ctx.Stats.PrimaryMessageArea = ctx.Stats.TopMessageAreas[0].Name
		ctx.Stats.PrimaryMessageUnread = ctx.Stats.TopMessageAreas[0].Unread
	}
	if ctx.Stats.PrimaryMessageUnread == 0 && ctx.Stats.UnreadMessages > 0 {
		ctx.Stats.PrimaryMessageUnread = ctx.Stats.UnreadMessages
	}
	if ctx.Stats.PrimaryMessageArea == "" && ctx.Stats.TotalMessages > 0 {
		ctx.Stats.PrimaryMessageArea = "Message Matrix"
	}
	if ctx.Stats.PrimaryFileArea == "" && ctx.Stats.TotalFiles > 0 {
		ctx.Stats.PrimaryFileArea = "File Vault"
		ctx.Stats.PrimaryFileNew = ctx.Stats.TotalFiles
	}
	if ctx.Stats.NewFiles == 0 {
		ctx.Stats.NewFiles = ctx.Stats.TotalFiles
	}

	return ctx
}

func (e *MenuExecutor) populateMessageStats(ctx *renderer.MenuContext, currentUser *user.User, terminal *terminalPkg.BBS, s ssh.Session, sessionStartTime time.Time) {
	if e.MessageMgr == nil {
		return
	}

	areas := e.MessageMgr.ListAreas()
	if len(areas) == 0 {
		return
	}

	primaryAreaID := 0
	if currentUser != nil {
		primaryAreaID = currentUser.CurrentMessageAreaID
	}

	var summaries []renderer.AreaSummary
	totalMessages := 0
	unreadTotal := 0
	primaryName := ""
	primaryUnread := 0

	for _, area := range areas {
		if currentUser != nil {
			if !checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
				continue
			}
		}

		count, err := e.MessageMgr.GetMessageCountForArea(area.ID)
		if err != nil {
			log.Printf("WARN: Failed to retrieve message count for area %d: %v", area.ID, err)
			continue
		}
		totalMessages += count

		newCount := 0
		if currentUser != nil {
			lastRead := ""
			if currentUser.LastReadMessageIDs != nil {
				lastRead = currentUser.LastReadMessageIDs[area.ID]
			}
			newCount, err = e.MessageMgr.GetNewMessageCount(area.ID, lastRead)
			if err != nil {
				log.Printf("WARN: Failed to retrieve new message count for area %d: %v", area.ID, err)
				newCount = 0
			}
			unreadTotal += newCount
			if newCount > 0 {
				summaries = append(summaries, renderer.AreaSummary{Name: area.Tag, Unread: newCount})
			}
			if primaryAreaID != 0 && area.ID == primaryAreaID {
				primaryName = area.Tag
				primaryUnread = newCount
			}
		}
	}

	if currentUser == nil {
		if len(areas) > 0 {
			primaryName = areas[0].Tag
			primaryUnread = totalMessages
		}
	} else if primaryName == "" && len(summaries) > 0 {
		primaryName = summaries[0].Name
		primaryUnread = summaries[0].Unread
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Unread == summaries[j].Unread {
			return strings.ToUpper(summaries[i].Name) < strings.ToUpper(summaries[j].Name)
		}
		return summaries[i].Unread > summaries[j].Unread
	})
	if len(summaries) > 3 {
		summaries = summaries[:3]
	}

	ctx.Stats.TotalMessages = totalMessages
	ctx.Stats.UnreadMessages = unreadTotal
	ctx.Stats.PrimaryMessageArea = primaryName
	ctx.Stats.PrimaryMessageUnread = primaryUnread
	ctx.Stats.TopMessageAreas = summaries
}

func (e *MenuExecutor) populateFileStats(ctx *renderer.MenuContext, currentUser *user.User, terminal *terminalPkg.BBS, s ssh.Session, sessionStartTime time.Time) {
	if e.FileMgr == nil {
		return
	}

	areas := e.FileMgr.ListAreas()
	if len(areas) == 0 {
		return
	}

	primaryAreaID := 0
	if currentUser != nil {
		primaryAreaID = currentUser.CurrentFileAreaID
	}

	totalFiles := 0
	primaryName := ""
	primaryCount := 0

	for _, area := range areas {
		if currentUser != nil {
			if !checkACS(area.ACSList, currentUser, s, terminal, sessionStartTime) {
				continue
			}
		}

		count, err := e.FileMgr.GetFileCountForArea(area.ID)
		if err != nil {
			log.Printf("WARN: Failed to retrieve file count for area %d: %v", area.ID, err)
			continue
		}
		totalFiles += count
		if primaryAreaID != 0 && area.ID == primaryAreaID {
			primaryName = area.Tag
			primaryCount = count
		}
	}

	if primaryName == "" && len(areas) > 0 {
		primaryName = areas[0].Tag
		primaryCount = totalFiles
	}

	ctx.Stats.TotalFiles = totalFiles
	ctx.Stats.NewFiles = totalFiles
	ctx.Stats.PrimaryFileArea = primaryName
	ctx.Stats.PrimaryFileNew = primaryCount
}

func computeUserRatio(u *user.User) string {
	if u == nil {
		return "100%"
	}
	uploads := u.NumUploads
	logons := u.TimesCalled
	if uploads <= 0 && logons <= 0 {
		return "100%"
	}
	if logons <= 0 {
		return "999%"
	}
	ratio := (uploads * 100) / logons
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 999 {
		ratio = 999
	}
	return fmt.Sprintf("%d%%", ratio)
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

	// Display content with proper pipe code processing
	statsDisplayBytes := []byte(substitutedContent)
	wErr = terminal.DisplayContent(statsDisplayBytes)
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
		var rawAnsiContent []byte
		var readErr error
		var useRenderer bool

		if e.Renderer != nil && currentMenuName != "LOGIN" {
			renderCtx := e.buildMenuContext(currentMenuName, currentUser, nodeNumber, terminal, s, sessionStartTime)
			if rendered, handled, renderErr := e.Renderer.Render(renderCtx); renderErr != nil {
				log.Printf("WARN: Renderer fallback for %s due to error: %v", currentMenuName, renderErr)
			} else if handled {
				rawAnsiContent = rendered
				useRenderer = true
			}
		}

		if !useRenderer {
			rawAnsiContent, readErr = terminalPkg.GetAnsiFileContent(fullAnsPath)
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
		}

		// Successfully read, now process for coords and display bytes using the passed outputMode
		var ansiProcessResult terminalPkg.ProcessAnsiResult
		var processErr error
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

	// Move to Username position for user input
	terminal.Write([]byte(terminalPkg.MoveCursor(userCoord.Y, userCoord.X)))
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
	terminal.Write([]byte(terminalPkg.MoveCursor(passCoord.Y, passCoord.X)))
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

// drawLightbarMenu draws the lightbar menu with the specified option selected
func drawLightbarMenu(terminal *terminalPkg.BBS, backgroundBytes []byte, options []LightbarOption, selectedIndex int, outputMode terminalPkg.OutputMode) error {
	// Clear screen and reset cursor to ensure clean redraw and prevent layering
	_, err := terminal.Write([]byte("\x1b[2J\x1b[H"))
	if err != nil {
		return fmt.Errorf("failed clearing screen for lightbar: %w", err)
	}

	// Calculate offset caused by leading lines in ANSI content
	offset := calculateANSIOffset(backgroundBytes)
	log.Printf("DEBUG: Calculated ANSI offset: %d lines", offset)

	// Draw static background
	// We might need to clear attributes before drawing background if it has colors
	// _, err := terminal.Write([]byte(attrReset))
	// if err != nil {
	// 	return fmt.Errorf("failed resetting attributes before background: %w", err)
	// }
	_, err = terminal.Write(backgroundBytes)
	if err != nil {
		return fmt.Errorf("failed writing lightbar background: %w", err)
	}

	// Draw each option, highlighting the selected one
	for i, opt := range options {
		log.Printf("DEBUG: Drawing option %d (%s) at Y=%d, X=%d, selected=%t", i, opt.Text, opt.Y, opt.X, i == selectedIndex)

		// Position cursor
		posCmd := fmt.Sprintf("\x1b[%d;%dH", opt.Y, opt.X)
		log.Printf("DEBUG: Positioning cursor with command: %q", posCmd)
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
		log.Printf("DEBUG: Applying color code %d -> %q", colorCode, ansiColorSequence)
		_, err = terminal.Write([]byte(ansiColorSequence))

		if err != nil {
			return fmt.Errorf("failed setting color for lightbar option: %w", err)
		}

		// Save cursor position before writing text
		log.Printf("DEBUG: Saving cursor position")
		_, err = terminal.Write([]byte("\x1b[s"))
		if err != nil {
			return fmt.Errorf("failed saving cursor position: %w", err)
		}

		// Write the option text
		log.Printf("DEBUG: Writing text %q (length: %d)", opt.Text, len(opt.Text))
		_, err = terminal.Write([]byte(opt.Text))
		if err != nil {
			return fmt.Errorf("failed writing lightbar option text: %w", err)
		}

		// Always reset attributes after each option to ensure clean display
		log.Printf("DEBUG: Resetting attributes with %q", attrReset)
		_, err = terminal.Write([]byte(attrReset))
		if err != nil {
			return fmt.Errorf("failed resetting attributes after lightbar option: %w", err)
		}

		// Restore cursor position to avoid leaving cursor in middle of ANSI art
		log.Printf("DEBUG: Restoring cursor position")
		_, err = terminal.Write([]byte("\x1b[u"))
		if err != nil {
			return fmt.Errorf("failed restoring cursor position: %w", err)
		}
		log.Printf("DEBUG: Completed drawing option %d", i)
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
	terminal.Write([]byte("\r\n\r\n"))               // Add some spacing
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

// NOTE: File-related runnables (runListFiles, displayFileAreaList, runListFileAreas, runSelectFileArea)
// have been moved to runnables_files.go
