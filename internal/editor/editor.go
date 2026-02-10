package editor

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/robbiew/vision3/internal/terminalio"
)

// Setup debug file
var debugLogger *log.Logger
var debugFile *os.File

func init() {
	// Create debugger
	/*
		var err error
		debugFile, err = os.OpenFile("editor_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Println("WARNING: Failed to open debug log file:", err)
			return
		}
		debugLogger = log.New(debugFile, "EDITOR_DEBUG: ", log.LstdFlags|log.Lmicroseconds)
		debugLogger.Println("----------- NEW SESSION STARTED -----------")
	*/
}

func debugLog(format string, v ...interface{}) {
	if debugLogger != nil {
		debugLogger.Printf(format, v...)
	}
}

func hexDump(s string) string {
	var result strings.Builder
	for i, c := range []byte(s) {
		if i > 0 && i%16 == 0 {
			result.WriteString("\n")
		}
		result.WriteString(fmt.Sprintf("%02x ", c))
	}
	return result.String()
}

// model represents the state of the editor
type model struct {
	textarea     textarea.Model
	viewport     viewport.Model
	width        int
	height       int
	saved        bool
	quitting     bool
	err          error
	terminalType string
	lastViewTime time.Time
	viewCount    int
	updateCount  int
}

// Initialize the model
func initialModel(initialContent string, termType string) model {
	debugLog("Creating initial model with termType=%s, content length=%d", termType, len(initialContent))

	ta := textarea.New()
	ta.Placeholder = "Enter your message..."
	ta.SetValue(initialContent)
	ta.CursorEnd()
	ta.Focus()
	ta.ShowLineNumbers = false

	// Explicitly remove any border styling from textarea
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	// Initialize viewport
	vp := viewport.New(80, 24)
	vp.SetContent(initialContent)

	// Default dimensions
	defaultWidth := 80
	defaultHeight := 24

	// Initial size setting for textarea (will be updated on WindowSizeMsg)
	ta.SetWidth(defaultWidth)
	ta.SetHeight(defaultHeight - 1)

	debugLog("Initial model created - default dimensions: %dx%d", defaultWidth, defaultHeight)

	return model{
		textarea:     ta,
		viewport:     vp,
		width:        defaultWidth,
		height:       defaultHeight,
		saved:        false,
		quitting:     false,
		err:          nil,
		terminalType: termType,
		lastViewTime: time.Now(),
		viewCount:    0,
		updateCount:  0,
	}
}

// Init initializes the tea program
func (m model) Init() tea.Cmd {
	debugLog("Model.Init() called")
	return tea.Batch(
		textarea.Blink,
		tea.EnterAltScreen,
	)
}

// Update handles events and updates the model
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.updateCount++
	debugLog("Model.Update() #%d called with msg type: %T", m.updateCount, msg)

	var cmds []tea.Cmd
	var taCmd, vpCmd tea.Cmd

	originalValue := m.textarea.Value()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		debugLog("KeyMsg: %s (type=%v, runes=%q)", msg.String(), msg.Type, msg.Runes)

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			debugLog("Quit triggered via %v", msg.Type)
			m.quitting = true
			m.saved = false
			return m, tea.Quit

		case tea.KeyCtrlS:
			debugLog("Save and quit triggered")
			m.quitting = true
			m.saved = true
			return m, tea.Quit
		}

		// Let textarea handle the key press
		m.textarea, taCmd = m.textarea.Update(msg)
		cmds = append(cmds, taCmd)

		// Log value change after key processing
		if m.textarea.Value() != originalValue {
			debugLog("Content changed after key: '%s' -> '%s'",
				originalValue, m.textarea.Value())
		}

	case tea.WindowSizeMsg:
		debugLog("WindowSizeMsg: %dx%d", msg.Width, msg.Height)
		m.width = msg.Width
		m.height = msg.Height

		// Reserve just one line for the status bar; lipgloss handles layout
		textAreaHeight := m.height - 1
		if textAreaHeight < 1 {
			textAreaHeight = 1
		}

		// Set textarea size
		m.textarea.SetWidth(m.width)
		m.textarea.SetHeight(textAreaHeight)

		// Update viewport dimensions (might be less relevant now)
		m.viewport.Width = m.width
		m.viewport.Height = textAreaHeight
		debugLog("Adjusted dimensions: textarea/viewport: %dx%d", m.width, textAreaHeight)
		return m, nil

	case error:
		m.err = msg
		m.quitting = true
		debugLog("Error received: %v", msg)
		return m, tea.Quit
	}

	// Update viewport state
	m.viewport.SetContent(m.textarea.Value())
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI using lipgloss for layout
func (m model) View() string {
	now := time.Now()
	timeSinceLast := now.Sub(m.lastViewTime)
	m.viewCount++
	m.lastViewTime = now

	debugLog("View() #%d called (%.2fms since last view)", m.viewCount, float64(timeSinceLast.Microseconds())/1000.0)

	if m.quitting {
		debugLog("View() returning empty string due to quitting state")
		return ""
	}

	if m.err != nil {
		errorMsg := "Error: " + m.err.Error()
		debugLog("View() returning error message: %s", errorMsg)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(errorMsg),
		)
	}

	// Define heights
	statusBarHeight := 1
	textAreaHeight := m.height - statusBarHeight
	if textAreaHeight < 1 {
		textAreaHeight = 1 // Ensure minimum height
	}
	debugLog("View dimensions: Total=%dx%d, TextArea=%dx%d, Status=%dx%d",
		m.width, m.height, m.width, textAreaHeight, m.width, statusBarHeight)

	// Render the text area within a fixed-size container
	textAreaStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(textAreaHeight)
		// We are relying on the textarea's internal scrolling now.
		// Borders etc. are handled by the textarea itself if configured,
		// or removed as they were in initialModel.

	textAreaRendered := textAreaStyle.Render(m.textarea.View())
	debugLog("Rendered Text Area content (len=%d)", len(textAreaRendered))

	// Render the status bar
	statusBarText := "Ctrl+S: Save | Esc: Cancel"
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")).
		Background(lipgloss.Color("235")).
		Width(m.width)

	statusBarRendered := statusStyle.Render(statusBarText)
	debugLog("Rendered Status Bar (len=%d): %s", len(statusBarRendered), statusBarText)

	// Join the text area and status bar vertically
	finalView := lipgloss.JoinVertical(lipgloss.Top, textAreaRendered, statusBarRendered)
	debugLog("Joined final view (len=%d)", len(finalView))

	// Log first few bytes for debugging
	if len(finalView) > 0 { // Ensure not empty before slicing
		maxHexBytes := 100
		if len(finalView) < maxHexBytes {
			maxHexBytes = len(finalView)
		}
		debugLog("First %d bytes (hex): %s", maxHexBytes, hexDump(finalView[:maxHexBytes]))
	}

	return finalView
}

// RunEditor takes initial text, the input/output streams from the SSH session,
// the terminal type string (TERM environment variable), runs a full-screen
// editor, and returns the final text content, whether it was saved, and any error.
func RunEditor(initialContent string, input io.Reader, output io.Writer, termType string) (content string, saved bool, err error) {
	debugLog("RunEditor called. InitialContent length=%d, termType=%s", len(initialContent), termType)

	m := initialModel(initialContent, termType)

	// Create a tee writer to capture all output
	outputCapture := &strings.Builder{}
	teeOutput := io.MultiWriter(output, outputCapture)

	// Wrap with CP437 encoder
	cp437Output := terminalio.NewSelectiveCP437Writer(teeOutput)
	debugLog("Created SelectiveCP437Writer wrapper")

	p := tea.NewProgram(
		m,
		tea.WithInput(input),
		tea.WithOutput(cp437Output),
		tea.WithAltScreen(),
	)
	debugLog("Created tea program with AltScreen")

	// Set the debug logger to capture program events
	/*
		if debugLogger != nil {
			tea.LogToFile("tea_debug.log", "DEBUG")
			debugLog("Set up Bubbletea internal logging to tea_debug.log")
		}
	*/

	debugLog("Starting editor program...")
	finalModel, runErr := p.Run()
	if runErr != nil {
		debugLog("Editor program failed: %v", runErr)
		return initialContent, false, runErr
	}
	debugLog("Editor program completed successfully")

	finalState, ok := finalModel.(model)
	if !ok {
		finalErr := errors.New("internal error: could not cast final model")
		debugLog("Type assertion failed: %v", finalErr)
		return initialContent, false, finalErr
	}

	if finalState.err != nil {
		debugLog("Editor finished with error state: %v", finalState.err)
		return finalState.textarea.Value(), finalState.saved, finalState.err
	}

	// Log final output details
	outputLen := outputCapture.Len()
	debugLog("Total bytes written to output: %d", outputLen)

	// Only log a sample of the output if it's large
	if outputLen > 1000 {
		debugLog("First 500 bytes of output: %s", hexDump(outputCapture.String()[:500]))
		debugLog("Last 500 bytes of output: %s", hexDump(outputCapture.String()[outputLen-500:]))
	}

	content = finalState.textarea.Value()
	saved = finalState.saved
	debugLog("Exiting RunEditor. Content length=%d, saved=%v", len(content), saved)

	/*
		if debugFile != nil {
			debugFile.Close()
		}
	*/

	return content, saved, nil
}
