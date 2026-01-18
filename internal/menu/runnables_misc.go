package menu

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/config"
	terminalPkg "github.com/stlalpha/vision3/internal/terminal"
	"github.com/stlalpha/vision3/internal/user"
)

// Mutex for protecting access to the oneliners file
var onelinerMutex sync.Mutex

// runSetRender allows sysops to update the menu renderer configuration at runtime.
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

// promptRendererOption prompts for a renderer option value interactively.
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

// parseKeyValueArgs parses a space-separated string of key=value pairs.
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

// containsInsensitive checks if a slice contains a value (case-insensitive).
func containsInsensitive(collection []string, value string) bool {
	val := strings.ToLower(strings.TrimSpace(value))
	for _, item := range collection {
		if strings.ToLower(item) == val {
			return true
		}
	}
	return false
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
