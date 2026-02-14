package editor

import (
	"io"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
)

// FSEditor is the full-screen message editor
type FSEditor struct {
	// Components
	buffer      *MessageBuffer
	screen      *Screen
	input       *InputHandler
	wordWrapper *WordWrapper
	commands    *CommandHandler

	// Cursor position (1-based)
	currentLine int
	currentCol  int

	// View state
	topLine int // First line visible on screen

	// Editor state
	insertMode bool
	modified   bool
	saved      bool
	quit       bool

	// Metadata
	subject     string
	recipient   string
	isAnon      bool
	menuSetPath string

	// Terminal
	session    ssh.Session
	outputMode ansi.OutputMode
}

// NewFSEditor creates a new full-screen editor instance
func NewFSEditor(session ssh.Session, terminal io.Writer, outputMode ansi.OutputMode,
	termWidth, termHeight int, menuSetPath, yesNoHi, yesNoLo, yesText, noText, abortText string) *FSEditor {

	buffer := NewMessageBuffer()
	screen := NewScreen(terminal, outputMode, termWidth, termHeight)
	input := NewInputHandler(session)
	wordWrapper := NewWordWrapper(buffer)
	commandHandler := NewCommandHandler(screen, buffer, menuSetPath, yesNoHi, yesNoLo, yesText, noText, abortText)

	return &FSEditor{
		buffer:      buffer,
		screen:      screen,
		input:       input,
		wordWrapper: wordWrapper,
		commands:    commandHandler,
		currentLine: 1,
		currentCol:  1,
		topLine:     1,
		insertMode:  true,
		modified:    false,
		saved:       false,
		quit:        false,
		session:     session,
		outputMode:  outputMode,
		menuSetPath: menuSetPath,
	}
}

// SetMetadata sets the message metadata (subject, recipient, etc.)
func (e *FSEditor) SetMetadata(subject, recipient string, isAnon bool) {
	e.subject = subject
	e.recipient = recipient
	e.isAnon = isAnon
}

// SetQuoteData sets message data to be used for the /Q quote command
func (e *FSEditor) SetQuoteData(data *QuoteData) {
	e.commands.SetQuoteData(data)
}

// LoadContent loads initial content into the editor
func (e *FSEditor) LoadContent(content string) {
	e.buffer.LoadContent(content)
	if content != "" {
		e.modified = true
		// Position at end of content
		lineCount := e.buffer.GetLineCount()
		e.currentLine = lineCount
		e.currentCol = e.buffer.GetLineLength(lineCount) + 1
	}
}

// Run starts the editor main loop
func (e *FSEditor) Run() (string, bool, error) {
	// Load and display header
	err := e.screen.LoadHeaderTemplate(e.menuSetPath, e.subject, e.recipient, e.isAnon)
	if err != nil {
		// Non-fatal - continue with minimal header
	}

	// Initial screen draw
	e.screen.FullRedraw(e.buffer, e.topLine, e.currentLine, e.currentCol, e.insertMode)

	// Main edit loop
	for !e.quit {
		// Read key
		key, err := e.input.ReadKeyTranslated()
		if err != nil {
			return "", false, err
		}

		// Handle the key
		e.handleKey(key)

		// Check for window resize (non-blocking)
		// This would be handled by the caller (editor.go) if needed

		// Ensure view is updated to keep cursor visible
		e.ensureCursorVisible()

		// Refresh screen
		e.screen.RefreshScreen(e.buffer, e.topLine, e.currentLine, e.currentCol, e.insertMode)
	}

	// Return final content and saved status
	content := e.buffer.GetContent()
	return content, e.saved, nil
}

// handleKey processes a single key press
func (e *FSEditor) handleKey(key int) {
	// Check if user typed "/" to trigger slash command menu
	if key == '/' && e.currentCol == 1 {
		// Show slash command menu and get user selection
		cmdType := e.commands.ShowSlashMenu(e.input, e.currentLine, e.currentCol)
		if cmdType != CommandNone {
			// Clear the menu from screen
			e.commands.ClearSlashMenu(e.currentLine, 40)
			// Handle the selected command
			e.handleCommand(cmdType)
			return
		}
		// User cancelled (ESC) - just clear the menu and return
		e.commands.ClearSlashMenu(e.currentLine, 40)
		return
	}

	// Handle key based on type
	switch key {
	case KeyEnter:
		e.handleReturn()
	case KeyBackspace:
		e.handleBackspace()
	case KeyTab:
		e.handleTab()

	// Navigation
	case KeyCtrlE: // Up
		e.moveCursorUp()
	case KeyCtrlX: // Down
		e.moveCursorDown()
	case KeyCtrlS: // Left
		e.moveCursorLeft()
	case KeyCtrlD: // Right
		e.moveCursorRight()
	case KeyCtrlW: // Home
		e.moveCursorHome()
	case KeyCtrlP: // End
		e.moveCursorEnd()
	case KeyCtrlR: // Page Up
		e.pageUp()
	case KeyCtrlC: // Page Down (note: normally quit in terminals, but remapped here)
		e.pageDown()
	case KeyCtrlA: // Word Left
		e.moveCursorWordLeft()
	case KeyCtrlF: // Word Right
		e.moveCursorWordRight()

	// Edit commands
	case KeyCtrlV: // Toggle insert/overwrite
		e.toggleInsertMode()
	case KeyCtrlG: // Delete character at cursor
		e.handleDeleteKey()
	case KeyCtrlT: // Delete word
		e.deleteWord()
	case KeyCtrlY: // Delete line
		e.deleteLine()
	case KeyCtrlJ: // Join lines
		e.joinLines()
	case KeyCtrlN: // Split line
		e.splitLine()
	case KeyCtrlB: // Reformat paragraph
		e.reformatParagraph()
	case KeyCtrlL: // Redraw screen
		e.redrawScreen()

	default:
		// Check if it's a printable character
		if IsPrintable(key) {
			e.insertCharacter(rune(key))
		}
	}
}

// handleCommand processes slash commands
func (e *FSEditor) handleCommand(cmdType CommandType) {
	switch cmdType {
	case CommandSave:
		if e.commands.HandleSave() {
			e.saved = true
			e.quit = true
		} else {
			// Redraw status after error message
			e.input.ReadKey() // Wait for key press
			e.screen.RefreshScreen(e.buffer, e.topLine, e.currentLine, e.currentCol, e.insertMode)
		}

	case CommandAbort:
		if e.commands.HandleAbort(e.input) {
			e.saved = false
			e.quit = true
		} else {
			e.screen.RefreshScreen(e.buffer, e.topLine, e.currentLine, e.currentCol, e.insertMode)
		}

	case CommandQuote:
		line, col := e.commands.HandleQuote(e.input, e.currentLine, e.currentCol)
		e.currentLine = line
		e.currentCol = col
		e.modified = true
		e.screen.FullRedraw(e.buffer, e.topLine, e.currentLine, e.currentCol, e.insertMode)

	case CommandHelp:
		e.commands.HandleHelp(e.input)
		e.screen.FullRedraw(e.buffer, e.topLine, e.currentLine, e.currentCol, e.insertMode)

	case CommandView:
		e.commands.HandleView(e.input)
		e.screen.FullRedraw(e.buffer, e.topLine, e.currentLine, e.currentCol, e.insertMode)
	}
}

// insertCharacter inserts a printable character at the cursor
func (e *FSEditor) insertCharacter(ch rune) {
	if e.insertMode {
		e.buffer.InsertChar(e.currentLine, e.currentCol, ch)
	} else {
		e.buffer.OverwriteChar(e.currentLine, e.currentCol, ch)
	}

	e.currentCol++
	e.modified = true

	// Check for word wrap
	newLine, newCol := e.wordWrapper.CheckAndWrap(e.currentLine, e.currentCol)
	e.currentLine = newLine
	e.currentCol = newCol
}

// handleReturn processes the Enter key
func (e *FSEditor) handleReturn() {
	// Split line at cursor position
	if e.buffer.SplitLine(e.currentLine, e.currentCol) {
		e.currentLine++
		e.currentCol = 1
		e.modified = true
	}
}

// handleBackspace processes the Backspace key
func (e *FSEditor) handleBackspace() {
	newLine, newCol, changed := e.wordWrapper.HandleBackspace(e.currentLine, e.currentCol)
	if changed {
		e.currentLine = newLine
		e.currentCol = newCol
		e.modified = true
	}
}

// handleDeleteKey processes the Delete key
func (e *FSEditor) handleDeleteKey() {
	if e.wordWrapper.HandleDelete(e.currentLine, e.currentCol) {
		e.modified = true
	}
}

// handleTab inserts tab spaces
func (e *FSEditor) handleTab() {
	// Insert 4 spaces for tab
	for i := 0; i < 4; i++ {
		e.insertCharacter(' ')
	}
}

// moveCursorUp moves cursor up one line
func (e *FSEditor) moveCursorUp() {
	if e.currentLine > 1 {
		e.currentLine--
		// Adjust column if line is shorter
		lineLen := e.buffer.GetLineLength(e.currentLine)
		if e.currentCol > lineLen+1 {
			e.currentCol = lineLen + 1
		}
	}
}

// moveCursorDown moves cursor down one line
func (e *FSEditor) moveCursorDown() {
	lineCount := e.buffer.GetLineCount()
	if e.currentLine < lineCount {
		e.currentLine++
		// Adjust column if line is shorter
		lineLen := e.buffer.GetLineLength(e.currentLine)
		if e.currentCol > lineLen+1 {
			e.currentCol = lineLen + 1
		}
	}
}

// moveCursorLeft moves cursor left one character
func (e *FSEditor) moveCursorLeft() {
	if e.currentCol > 1 {
		e.currentCol--
	} else if e.currentLine > 1 {
		// Move to end of previous line
		e.currentLine--
		e.currentCol = e.buffer.GetLineLength(e.currentLine) + 1
	}
}

// moveCursorRight moves cursor right one character
func (e *FSEditor) moveCursorRight() {
	lineLen := e.buffer.GetLineLength(e.currentLine)
	if e.currentCol <= lineLen {
		e.currentCol++
	} else if e.currentLine < e.buffer.GetLineCount() {
		// Move to start of next line
		e.currentLine++
		e.currentCol = 1
	}
}

// moveCursorHome moves cursor to start of line
func (e *FSEditor) moveCursorHome() {
	e.currentCol = 1
}

// moveCursorEnd moves cursor to end of line
func (e *FSEditor) moveCursorEnd() {
	lineLen := e.buffer.GetLineLength(e.currentLine)
	e.currentCol = lineLen + 1
}

// moveCursorWordLeft moves cursor to start of previous word
func (e *FSEditor) moveCursorWordLeft() {
	e.currentCol = e.wordWrapper.FindWordLeft(e.currentLine, e.currentCol)
}

// moveCursorWordRight moves cursor to start of next word
func (e *FSEditor) moveCursorWordRight() {
	e.currentCol = e.wordWrapper.FindWordRight(e.currentLine, e.currentCol)
}

// pageUp scrolls up one page
func (e *FSEditor) pageUp() {
	scrollAmount := e.screen.GetScreenLines() - 1
	e.currentLine -= scrollAmount
	if e.currentLine < 1 {
		e.currentLine = 1
	}
}

// pageDown scrolls down one page
func (e *FSEditor) pageDown() {
	scrollAmount := e.screen.GetScreenLines() - 1
	lineCount := e.buffer.GetLineCount()
	e.currentLine += scrollAmount
	if e.currentLine > lineCount {
		e.currentLine = lineCount
	}
	if e.currentLine < 1 {
		e.currentLine = 1
	}
}

// toggleInsertMode toggles between insert and overwrite modes
func (e *FSEditor) toggleInsertMode() {
	e.insertMode = !e.insertMode
}

// deleteWord deletes the word to the right of the cursor
func (e *FSEditor) deleteWord() {
	if e.wordWrapper.DeleteWord(e.currentLine, e.currentCol) {
		e.modified = true
	}
}

// deleteLine deletes the current line
func (e *FSEditor) deleteLine() {
	if e.buffer.DeleteLine(e.currentLine) {
		e.modified = true
		// Ensure cursor is on a valid line
		if e.currentLine > e.buffer.GetLineCount() {
			e.currentLine = e.buffer.GetLineCount()
			if e.currentLine < 1 {
				e.currentLine = 1
			}
		}
		e.currentCol = 1
	}
}

// joinLines joins current line with next line
func (e *FSEditor) joinLines() {
	if e.buffer.JoinLines(e.currentLine) {
		e.modified = true
	}
}

// splitLine splits the current line at the cursor
func (e *FSEditor) splitLine() {
	if e.buffer.SplitLine(e.currentLine, e.currentCol) {
		e.currentLine++
		e.currentCol = 1
		e.modified = true
	}
}

// reformatParagraph reformats the current paragraph
func (e *FSEditor) reformatParagraph() {
	lastLine := e.wordWrapper.ReformatParagraph(e.currentLine)
	e.modified = true
	// Keep cursor on a valid line
	if e.currentLine > lastLine {
		e.currentLine = lastLine
	}
	e.currentCol = 1
	// Force full redraw
	e.screen.FullRedraw(e.buffer, e.topLine, e.currentLine, e.currentCol, e.insertMode)
}

// redrawScreen forces a complete screen redraw
func (e *FSEditor) redrawScreen() {
	e.screen.FullRedraw(e.buffer, e.topLine, e.currentLine, e.currentCol, e.insertMode)
}

// ensureCursorVisible adjusts the view to keep the cursor visible
func (e *FSEditor) ensureCursorVisible() {
	screenLines := e.screen.GetScreenLines()

	// Check if cursor is above visible area
	if e.currentLine < e.topLine {
		e.topLine = e.currentLine
	}

	// Check if cursor is below visible area
	if e.currentLine >= e.topLine+screenLines {
		e.topLine = e.currentLine - screenLines + 1
		if e.topLine < 1 {
			e.topLine = 1
		}
	}
}

// GetContent returns the editor content
func (e *FSEditor) GetContent() string {
	return e.buffer.GetContent()
}

// IsSaved returns whether the message was saved
func (e *FSEditor) IsSaved() bool {
	return e.saved
}

// IsModified returns whether the content was modified
func (e *FSEditor) IsModified() bool {
	return e.modified
}

// HandleResize handles terminal resize events
func (e *FSEditor) HandleResize(newWidth, newHeight int) {
	e.screen.Resize(newWidth, newHeight)
	e.screen.FullRedraw(e.buffer, e.topLine, e.currentLine, e.currentCol, e.insertMode)
}

// GetBuffer returns the message buffer (for testing)
func (e *FSEditor) GetBuffer() *MessageBuffer {
	return e.buffer
}
