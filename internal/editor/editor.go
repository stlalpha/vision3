package editor

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gliderlabs/ssh"
	"github.com/robbiew/vision3/internal/ansi"
	"github.com/robbiew/vision3/internal/config"
	"github.com/robbiew/vision3/internal/terminalio"
)

func resolveEditorPaths() (menuSetPath, rootConfigPath string) {
	menuSetPath = os.Getenv("VISION3_MENU_PATH")
	rootConfigPath = os.Getenv("VISION3_CONFIG_PATH")

	if menuSetPath == "" {
		menuSetPath = "menus/v3"
	}
	if rootConfigPath == "" {
		rootConfigPath = "configs"
	}

	if _, err := os.Stat(menuSetPath); err == nil {
		if _, err := os.Stat(rootConfigPath); err == nil {
			return menuSetPath, rootConfigPath
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		candidateMenu := filepath.Join(cwd, "menus/v3")
		candidateConfig := filepath.Join(cwd, "configs")
		if _, err := os.Stat(candidateMenu); err == nil {
			if _, err := os.Stat(candidateConfig); err == nil {
				return candidateMenu, candidateConfig
			}
		}
	}

	return menuSetPath, rootConfigPath
}

// RunEditor takes initial text, the input/output streams from the SSH session,
// the terminal type string (TERM environment variable), runs a full-screen
// editor, and returns the final text content, whether it was saved, and any error.
func RunEditor(initialContent string, input io.Reader, output io.Writer, termType string) (content string, saved bool, err error) {
	// Determine output mode based on terminal type
	outputMode := ansi.OutputModeUTF8
	termLower := strings.ToLower(termType)
	if termLower == "ansi" || termLower == "sync" || termLower == "syncterm" || termLower == "scoansi" {
		outputMode = ansi.OutputModeCP437
	}

	// Get session from input (must be ssh.Session)
	session, ok := input.(ssh.Session)
	if !ok {
		return initialContent, false, nil
	}

	// Get terminal dimensions
	termWidth := 80
	termHeight := 24

	// Try to get PTY size from session
	ptyReq, winCh, isPty := session.Pty()
	if isPty {
		termWidth = ptyReq.Window.Width
		termHeight = ptyReq.Window.Height
	}

	// Ensure minimum dimensions
	if termWidth < 80 {
		termWidth = 80
	}
	if termHeight < 24 {
		termHeight = 24
	}

	// Wrap output with CP437 encoder if needed
	var wrappedOutput io.Writer
	if outputMode == ansi.OutputModeCP437 {
		wrappedOutput = terminalio.NewSelectiveCP437Writer(output)
	} else {
		wrappedOutput = output
	}

	menuSetPath, rootConfigPath := resolveEditorPaths()

	// Load theme colors for Yes/No lightbar prompts
	theme, _ := config.LoadThemeConfig(menuSetPath)
	yesNoHi := colorCodeToAnsi(theme.YesNoHighlightColor)
	yesNoLo := colorCodeToAnsi(theme.YesNoRegularColor)

	// Load configurable Yes/No labels for prompts.
	stringsCfg, stringsErr := config.LoadStrings(rootConfigPath)
	yesText := "Yes"
	noText := "No"
	abortText := "|14Abort message?"
	if stringsErr == nil {
		if v := strings.TrimSpace(stringsCfg.YesPromptText); v != "" {
			yesText = v
		}
		if v := strings.TrimSpace(stringsCfg.NoPromptText); v != "" {
			noText = v
		}
		if v := strings.TrimSpace(stringsCfg.AbortMessagePrompt); v != "" {
			abortText = v
		}
	}

	// Create the full-screen editor
	editor := NewFSEditor(session, wrappedOutput, outputMode, termWidth, termHeight, menuSetPath, yesNoHi, yesNoLo, yesText, noText, abortText)

	// Load initial content
	if initialContent != "" {
		editor.LoadContent(initialContent)
	}

	// Handle window resize events in background if we have PTY
	done := make(chan struct{})
	defer close(done)

	if isPty && winCh != nil {
		go func() {
			for {
				select {
				case win, ok := <-winCh:
					if !ok {
						return
					}
					editor.HandleResize(win.Width, win.Height)
				case <-done:
					return
				}
			}
		}()
	}

	// Run the editor
	finalContent, wasSaved, editorErr := editor.Run()

	// Return results
	return finalContent, wasSaved, editorErr
}

// RunEditorWithMetadata is an extended version that accepts message metadata
func RunEditorWithMetadata(initialContent string, input io.Reader, output io.Writer, termType string,
	subject, recipient string, isAnon bool, quoteFrom, quoteTitle, quoteDate, quoteTime string, quoteIsAnon bool, quoteLines []string) (content string, saved bool, err error) {

	// Determine output mode based on terminal type
	outputMode := ansi.OutputModeUTF8
	termLower := strings.ToLower(termType)
	if termLower == "ansi" || termLower == "sync" || termLower == "syncterm" || termLower == "scoansi" {
		outputMode = ansi.OutputModeCP437
	}

	// Get session from input (must be ssh.Session)
	session, ok := input.(ssh.Session)
	if !ok {
		return initialContent, false, nil
	}

	// Get terminal dimensions
	termWidth := 80
	termHeight := 24

	// Try to get PTY size from session
	ptyReq, winCh, isPty := session.Pty()
	if isPty {
		termWidth = ptyReq.Window.Width
		termHeight = ptyReq.Window.Height
	}

	// Ensure minimum dimensions
	if termWidth < 80 {
		termWidth = 80
	}
	if termHeight < 24 {
		termHeight = 24
	}

	// Wrap output with CP437 encoder if needed
	var wrappedOutput io.Writer
	if outputMode == ansi.OutputModeCP437 {
		wrappedOutput = terminalio.NewSelectiveCP437Writer(output)
	} else {
		wrappedOutput = output
	}

	menuSetPath, rootConfigPath := resolveEditorPaths()

	// Load theme colors for Yes/No lightbar prompts
	theme, _ := config.LoadThemeConfig(menuSetPath)
	yesNoHi := colorCodeToAnsi(theme.YesNoHighlightColor)
	yesNoLo := colorCodeToAnsi(theme.YesNoRegularColor)

	// Load configurable Yes/No labels for prompts.
	stringsCfg, stringsErr := config.LoadStrings(rootConfigPath)
	yesText := "Yes"
	noText := "No"
	abortText := "|14Abort message?"
	if stringsErr == nil {
		if v := strings.TrimSpace(stringsCfg.YesPromptText); v != "" {
			yesText = v
		}
		if v := strings.TrimSpace(stringsCfg.NoPromptText); v != "" {
			noText = v
		}
		if v := strings.TrimSpace(stringsCfg.AbortMessagePrompt); v != "" {
			abortText = v
		}
	}

	// Create the full-screen editor
	editor := NewFSEditor(session, wrappedOutput, outputMode, termWidth, termHeight, menuSetPath, yesNoHi, yesNoLo, yesText, noText, abortText)

	// Set metadata
	editor.SetMetadata(subject, recipient, isAnon)

	// Set quote data for /Q command
	if len(quoteLines) > 0 {
		quoteData := &QuoteData{
			From:   quoteFrom,
			Title:  quoteTitle,
			Date:   quoteDate,
			Time:   quoteTime,
			IsAnon: quoteIsAnon,
			Lines:  quoteLines,
		}
		editor.SetQuoteData(quoteData)
	}

	// Load initial content
	if initialContent != "" {
		editor.LoadContent(initialContent)
	}

	// Handle window resize events in background if we have PTY
	done := make(chan struct{})
	defer close(done)

	if isPty && winCh != nil {
		go func() {
			for {
				select {
				case win, ok := <-winCh:
					if !ok {
						return
					}
					editor.HandleResize(win.Width, win.Height)
				case <-done:
					return
				}
			}
		}()
	}

	// Run the editor
	finalContent, wasSaved, editorErr := editor.Run()

	// Return results
	return finalContent, wasSaved, editorErr
}
