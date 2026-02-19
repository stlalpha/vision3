package menu

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/editor"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// RunMatrixScreen displays the pre-login matrix menu and returns the selected action.
// Actions: "LOGIN", "NEWUSER", "CHECKACCESS", "DISCONNECT"
// Called from main.go sessionHandler before the login loop for telnet users.
// Uses standard .BAR/.CFG menu files (PDMATRIX.BAR, PDMATRIX.CFG) for configuration.
func (e *MenuExecutor) RunMatrixScreen(
	s ssh.Session,
	terminal *term.Terminal,
	userManager *user.UserMgr,
	nodeNumber int,
	outputMode ansi.OutputMode,
	termWidth int,
	termHeight int,
) (string, error) {
	const menuName = "PDMATRIX"

	// Load lightbar options from PDMATRIX.BAR
	options, err := loadLightbarOptions(menuName, e)
	if err != nil {
		log.Printf("WARN: Node %d: Failed to load %s.BAR: %v, skipping matrix", nodeNumber, menuName, err)
		return "LOGIN", nil
	}
	if len(options) == 0 {
		log.Printf("WARN: Node %d: No options in %s.BAR, skipping matrix", nodeNumber, menuName)
		return "LOGIN", nil
	}

	// Load commands from PDMATRIX.CFG to map hotkeys to actions
	cfgPath := filepath.Join(e.MenuSetPath, "cfg")
	commands, err := LoadCommands(menuName, cfgPath)
	if err != nil {
		log.Printf("WARN: Node %d: Failed to load %s.CFG: %v, skipping matrix", nodeNumber, menuName, err)
		return "LOGIN", nil
	}

	// Build hotkey → command map
	commandMap := make(map[string]string)
	for _, cmd := range commands {
		commandMap[strings.ToUpper(cmd.Keys)] = strings.ToUpper(cmd.Command)
	}

	// Load the ANSI background (convention: PDMATRIX.ANS)
	// Use GetAnsiFileContent to automatically strip SAUCE metadata
	ansPath := filepath.Join(e.MenuSetPath, "ansi", menuName+".ANS")
	ansBackground, err := ansi.GetAnsiFileContent(ansPath)
	if err != nil {
		log.Printf("WARN: Node %d: Failed to load %s.ANS: %v, skipping matrix", nodeNumber, menuName, err)
		return "LOGIN", nil
	}

	log.Printf("INFO: Node %d: Displaying pre-login matrix screen (%d options)", nodeNumber, len(options))

	// Ensure cursor is restored when we exit the matrix screen
	defer func() {
		terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode)
	}()

	selectedIndex := 0
	maxTries := 10
	tries := 0

	// Draw the initial screen
	if err := drawMatrixScreen(terminal, ansBackground, options, selectedIndex, outputMode); err != nil {
		log.Printf("ERROR: Node %d: Failed to draw matrix screen: %v", nodeNumber, err)
		return "LOGIN", nil
	}

	// Apply the pre-login idle timeout on the shared InputHandler.
	// nil = no authenticated user yet; sysop exemption applies post-login only.
	getSessionIH(s).SetSessionIdleTimeout(e.idleTimeout(nil))

	sessionIH := getSessionIH(s)
	for tries < maxTries {
		key, err := sessionIH.ReadKey()
		if err != nil {
			if errors.Is(err, editor.ErrIdleTimeout) {
				e.handleIdleTimeout(terminal, outputMode, nodeNumber, termHeight)
				return "DISCONNECT", nil
			}
			if errors.Is(err, io.EOF) {
				return "DISCONNECT", io.EOF
			}
			return "DISCONNECT", fmt.Errorf("failed reading matrix input: %w", err)
		}

		newIndex := selectedIndex
		selectionMade := false

		switch key {
		case editor.KeyArrowUp:
			newIndex = selectedIndex - 1
			if newIndex < 0 {
				newIndex = len(options) - 1 // Wrap to bottom
			}

		case editor.KeyArrowDown:
			newIndex = selectedIndex + 1
			if newIndex >= len(options) {
				newIndex = 0 // Wrap to top
			}

		case editor.KeyEnter:
			selectionMade = true

		case editor.KeyEsc:
			// Bare ESC (sequences already decoded by ReadKey) — ignore

		case ' ':
			// Spacebar redraws screen (matches Pascal behavior)
			drawMatrixScreen(terminal, ansBackground, options, selectedIndex, outputMode)

		default:
			if key < 32 || key > 126 {
				continue // ignore non-printable / special keys
			}
			r := rune(key)

			// Direct selection by number
			if r >= '1' && r <= '9' {
				numIndex := int(r - '1')
				if numIndex < len(options) {
					selectedIndex = numIndex
					drawMatrixOptions(terminal, options, selectedIndex, outputMode)
					selectionMade = true
				}
				break
			}

			// Check for hotkey match (explicit HotKey field from BAR file)
			keyStr := strings.ToUpper(string(r))
			matchedHotkey := false
			for i, opt := range options {
				if keyStr == opt.HotKey {
					selectedIndex = i
					drawMatrixOptions(terminal, options, selectedIndex, outputMode)
					selectionMade = true
					matchedHotkey = true
					break
				}
			}
			if !matchedHotkey {
				e.showUndefinedMenuInput(terminal, outputMode, nodeNumber)
				drawMatrixScreen(terminal, ansBackground, options, selectedIndex, outputMode)
			}
		}

		if newIndex != selectedIndex {
			selectedIndex = newIndex
			drawMatrixOptions(terminal, options, selectedIndex, outputMode)
		}

		if selectionMade {
			// Look up the command for this option's hotkey
			hotkey := options[selectedIndex].HotKey
			action, ok := commandMap[hotkey]
			if !ok {
				log.Printf("WARN: Node %d: No command mapped for hotkey '%s'", nodeNumber, hotkey)
				continue
			}
			log.Printf("INFO: Node %d: Matrix selection: %s (%s)", nodeNumber, options[selectedIndex].Text, action)

			result, err := e.processMatrixAction(action, s, terminal, userManager, nodeNumber, outputMode, termWidth, termHeight)
			if err != nil {
				return result, err
			}
			if result == "LOGIN" || result == "DISCONNECT" {
				return result, nil
			}

			// For actions that return to the matrix (like NEWUSER, CHECKACCESS),
			// redraw the screen and continue
			tries++
			selectedIndex = 0
			drawMatrixScreen(terminal, ansBackground, options, selectedIndex, outputMode)
		}
	}

	// Max tries exceeded
	log.Printf("INFO: Node %d: Matrix max tries exceeded, disconnecting", nodeNumber)
	return "DISCONNECT", nil
}

// processMatrixAction handles the selected matrix menu action.
func (e *MenuExecutor) processMatrixAction(
	action string,
	s ssh.Session,
	terminal *term.Terminal,
	userManager *user.UserMgr,
	nodeNumber int,
	outputMode ansi.OutputMode,
	termWidth int,
	termHeight int,
) (string, error) {
	switch action {
	case "LOGIN":
		// Show PRELOGON ANSI file before login screen (matches Pascal: Printfile(PRELOGON.x) + HoldScreen)
		e.showPrelogon(s, terminal, nodeNumber, outputMode, termWidth, termHeight)
		return "LOGIN", nil

	case "NEWUSER":
		// Clear screen immediately when transitioning from matrix to new user flow
		terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
		terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode) // Show cursor
		err := e.handleNewUserApplication(s, terminal, userManager, nodeNumber, outputMode, termWidth, termHeight)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "DISCONNECT", io.EOF
			}
			log.Printf("ERROR: Node %d: New user application error from matrix: %v", nodeNumber, err)
		}
		return "MATRIX", nil // Return to matrix after signup

	case "CHECKACCESS":
		e.handleCheckAccess(s, terminal, userManager, nodeNumber, outputMode)
		return "MATRIX", nil // Return to matrix after check

	case "DISCONNECT":
		terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.MatrixDisconnecting)), outputMode)
		return "DISCONNECT", nil

	default:
		log.Printf("WARN: Node %d: Unknown matrix action: %s", nodeNumber, action)
		e.showUndefinedMenuInput(terminal, outputMode, nodeNumber)
		return "MATRIX", nil
	}
}

// handleCheckAccess prompts for a username and shows their validation status.
func (e *MenuExecutor) handleCheckAccess(
	s ssh.Session,
	terminal *term.Terminal,
	userManager *user.UserMgr,
	nodeNumber int,
	outputMode ansi.OutputMode,
) {
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)

	terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.MatrixCheckAccessPrompt)), outputMode)

	input, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		return
	}

	username := strings.TrimSpace(input)
	if username == "" {
		return
	}

	foundUser, exists := userManager.GetUser(strings.ToLower(username))
	if !exists {
		// Also check by handle
		foundUser, exists = userManager.GetUserByHandle(username)
	}

	if !exists {
		terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.MatrixUserNotFound)), outputMode)
	} else if foundUser.Validated {
		msg := fmt.Sprintf(e.LoadedStrings.MatrixAccountValidated, foundUser.Handle, foundUser.AccessLevel)
		terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	} else {
		msg := fmt.Sprintf(e.LoadedStrings.MatrixAccountNotValidated, foundUser.Handle)
		terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	}

	// Pause
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... "
	}
	terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(pausePrompt)), outputMode)
	_, _ = readLineFromSessionIH(s, terminal)
}

// showPrelogon displays a random PRELOGON ANSI file before the login screen.
// Matches Pascal: Printfile(PRELOGON.x) + HoldScreen where x is random 1..NumPrelogon.
// Looks for numbered files (PRELOGON.1, PRELOGON.2, ...) first, falls back to PRELOGON.ANS.
func (e *MenuExecutor) showPrelogon(s ssh.Session, terminal *term.Terminal, nodeNumber int, outputMode ansi.OutputMode, termWidth, termHeight int) {
	ansiDir := filepath.Join(e.MenuSetPath, "ansi")

	// Look for numbered PRELOGON files (Pascal pattern: PRELOGON.1, PRELOGON.2, ...)
	var candidates []string
	for i := 1; i <= 20; i++ {
		path := filepath.Join(ansiDir, fmt.Sprintf("PRELOGON.%d", i))
		if _, err := os.Stat(path); err == nil {
			candidates = append(candidates, path)
		} else {
			break // Stop at first gap
		}
	}

	// Fall back to single PRELOGON.ANS
	if len(candidates) == 0 {
		path := filepath.Join(ansiDir, "PRELOGON.ANS")
		if _, err := os.Stat(path); err == nil {
			candidates = append(candidates, path)
		}
	}

	if len(candidates) == 0 {
		return // No PRELOGON files found
	}

	// Pick a random file
	idx := 0
	if len(candidates) > 1 {
		idx = int(time.Now().UnixNano() % int64(len(candidates)))
	}

	// Use GetAnsiFileContent to automatically strip SAUCE metadata
	rawContent, err := ansi.GetAnsiFileContent(candidates[idx])
	if err != nil {
		log.Printf("WARN: Node %d: Failed to read prelogon file %s: %v", nodeNumber, candidates[idx], err)
		return
	}

	log.Printf("INFO: Node %d: Displaying prelogon screen: %s", nodeNumber, filepath.Base(candidates[idx]))
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	// For CP437 mode, write raw bytes directly to avoid UTF-8 false positives
	if outputMode == ansi.OutputModeCP437 {
		terminal.Write(rawContent)
	} else {
		terminalio.WriteProcessedBytes(terminal, rawContent, outputMode)
	}

	// HoldScreen — pause before proceeding to login
	terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
	e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
}

// drawMatrixScreen clears the screen, draws the ANSI background, and highlights the selected option.
func drawMatrixScreen(
	terminal *term.Terminal,
	ansBackground []byte,
	options []LightbarOption,
	selectedIndex int,
	outputMode ansi.OutputMode,
) error {
	// Clear screen and draw background
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	// For CP437 mode, write raw bytes directly to avoid UTF-8 false positives
	if outputMode == ansi.OutputModeCP437 {
		terminal.Write(ansBackground)
	} else {
		terminalio.WriteProcessedBytes(terminal, ansBackground, outputMode)
	}

	// Draw options with highlighting
	return drawMatrixOptions(terminal, options, selectedIndex, outputMode)
}

// drawMatrixOptions redraws the menu option text with the current selection highlighted.
// Uses DOS color codes from LightbarOption (via colorCodeToAnsi) for rendering.
func drawMatrixOptions(
	terminal *term.Terminal,
	options []LightbarOption,
	selectedIndex int,
	outputMode ansi.OutputMode,
) error {
	for i, opt := range options {
		// Position cursor at this option
		posCmd := fmt.Sprintf("\x1b[%d;%dH", opt.Y, opt.X)
		terminalio.WriteProcessedBytes(terminal, []byte(posCmd), outputMode)

		// Apply color based on selection (DOS color code → ANSI escape)
		var colorCode int
		if i == selectedIndex {
			colorCode = opt.HighlightColor
		} else {
			colorCode = opt.RegularColor
		}
		terminalio.WriteProcessedBytes(terminal, []byte(colorCodeToAnsi(colorCode)), outputMode)

		// Write the option text
		terminalio.WriteProcessedBytes(terminal, []byte(opt.Text), outputMode)

		// Reset attributes
		terminalio.WriteProcessedBytes(terminal, []byte("\x1b[0m"), outputMode)
	}

	// Hide cursor after drawing
	terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
	return nil
}
