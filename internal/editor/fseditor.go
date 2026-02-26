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
	topLine     int // First line visible on screen
	lastTopLine int // Track last topLine to detect scrolling

	// Editor state
	insertMode     bool
	lastInsertMode bool
	modified       bool
	saved          bool
	quit           bool

	// Metadata
	subject     string
	recipient   string
	fromName    string // sender display name: handle, real name, or anonymous string
	isAnon      bool
	menuSetPath string

	// Terminal
	session    ssh.Session
	outputMode ansi.OutputMode
}

// NewFSEditor creates a new full-screen editor instance.
// ih is an optional pre-created InputHandler to reuse (pass nil to create a new one).
// Passing a shared InputHandler prevents the editor's background goroutine from
// racing with the caller's reader for bytes after the editor exits.
func NewFSEditor(session ssh.Session, terminal io.Writer, outputMode ansi.OutputMode,
	termWidth, termHeight int, menuSetPath, yesNoHi, yesNoLo, yesText, noText, abortText string,
	ih *InputHandler) *FSEditor {

	buffer := NewMessageBuffer()
	screen := NewScreen(terminal, outputMode, termWidth, termHeight)
	if ih == nil {
		ih = NewInputHandler(session)
	}
	input := ih
	wordWrapper := NewWordWrapper(buffer)
	commandHandler := NewCommandHandler(screen, buffer, menuSetPath, yesNoHi, yesNoLo, yesText, noText, abortText)

	return &FSEditor{
		buffer:         buffer,
		screen:         screen,
		input:          input,
		wordWrapper:    wordWrapper,
		commands:       commandHandler,
		currentLine:    1,
		currentCol:     1,
		topLine:        1,
		lastTopLine:    1,
		insertMode:     true,
		lastInsertMode: true,
		modified:       false,
		saved:          false,
		quit:           false,
		session:        session,
		outputMode:     outputMode,
		menuSetPath:    menuSetPath,
	}
}

// SetMetadata sets the message metadata (subject, recipient, sender, etc.)
func (e *FSEditor) SetMetadata(subject, recipient, fromName string, isAnon bool) {
	e.subject = subject
	e.recipient = recipient
	e.fromName = fromName
	e.isAnon = isAnon
}

// SetTimezone configures the timezone used for date/time display in the editor header.
func (e *FSEditor) SetTimezone(configTZ string) {
	e.screen.configTimezone = configTZ
}

// SetBoardName sets the BBS board name substituted into the footer @B@ placeholder.
func (e *FSEditor) SetBoardName(name string) {
	e.screen.boardName = name
}

// SetEditorContext sets optional context fields displayed in the editor header
// (node number, next message number, conference > area name).
func (e *FSEditor) SetEditorContext(ctx EditorContext) {
	e.screen.nodeNumber = ctx.NodeNumber
	e.screen.nextMsgNum = ctx.NextMsgNum
	e.screen.confArea = ctx.ConfArea
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
	err := e.screen.LoadHeaderTemplate(e.menuSetPath, e.subject, e.recipient, e.fromName, e.isAnon)
	if err != nil {
		// Non-fatal - continue with minimal header
	}

	// Load footer template — adjusts statusLineY to reserve the bottom 2 rows.
	// Must be called after LoadHeaderTemplate so editingStartY is already set from |#N.
	_ = e.screen.LoadFooterTemplate(e.menuSetPath)

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

		// Determine if we need to update status line fields
		statusChanged := (e.insertMode != e.lastInsertMode) || (e.topLine != e.lastTopLine)

		// Refresh screen
		e.screen.RefreshScreen(e.buffer, e.topLine, e.currentLine, e.currentCol, e.insertMode, statusChanged)

		// Update tracking variables
		e.lastInsertMode = e.insertMode
		e.lastTopLine = e.topLine
	}

	// Return final content and saved status
	content := e.buffer.GetContent()
	return content, e.saved, nil
}

// handleKey processes a single key press
func (e *FSEditor) handleKey(key int) {
	// Handle key based on type
	switch key {
	case KeyEnter:
		e.handleReturn()
	case KeyBackspace:
		e.handleBackspace()
	case KeyTab:
		e.handleTab()

	// Escape: show option lightbar menu (Save / Abort / Edit / Help / Quote)
	case KeyEsc:
		cmdType := e.commands.ShowEscapeMenu(e.input)
		if cmdType != CommandNone {
			e.handleCommand(cmdType)
		}

	// Editor commands (shown in footer: CTRL (A)Abort (Z)Save (Q)Quote)
	case KeyCtrlA: // Abort
		e.handleCommand(CommandAbort)
	case KeyCtrlZ: // Save
		e.handleCommand(CommandSave)
	case KeyCtrlQ: // Quote
		e.handleCommand(CommandQuote)

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

// handleCommand processes editor commands (CTRL-A/Z/Q and help/view)
func (e *FSEditor) handleCommand(cmdType CommandType) {
	switch cmdType {
	case CommandSave:
		if e.commands.HandleSave() {
			// Show "Saving..." in the prompt row before exiting
			e.screen.GoXY(1, e.screen.PromptRow())
			e.screen.ClearEOL()
			e.screen.WriteDirectProcessed("|15Saving...")
			e.saved = true
			e.quit = true
		} else {
			// Error message already written; wait for key then restore footer
			e.input.ReadKey()
			e.screen.DisplayFooter()
			e.screen.RefreshScreen(e.buffer, e.topLine, e.currentLine, e.currentCol, e.insertMode, true)
		}

	case CommandAbort:
		if e.commands.HandleAbort(e.input) {
			e.saved = false
			e.quit = true
		} else {
			e.screen.RefreshScreen(e.buffer, e.topLine, e.currentLine, e.currentCol, e.insertMode, true)
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
	newLine, newCol := e.wordWrapper.WrapAfterInsert(e.currentLine, e.currentCol)
	e.currentLine = newLine
	e.currentCol = newCol
}

// handleReturn processes the Enter key
func (e *FSEditor) handleReturn() {
	// Split line at cursor position
	if e.buffer.SplitLine(e.currentLine, e.currentCol) {
		// Mark the split line as a hard newline (user-created break)
		e.buffer.SetHardNewline(e.currentLine, true)
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
	newLine, newCol, changed := e.wordWrapper.HandleDelete(e.currentLine, e.currentCol)
	if changed {
		e.currentLine = newLine
		e.currentCol = newCol
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
	newLine, newCol, changed := e.wordWrapper.DeleteWord(e.currentLine, e.currentCol)
	if changed {
		e.currentLine = newLine
		e.currentCol = newCol
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

// joinLines joins current line with next line and reflows
func (e *FSEditor) joinLines() {
	if e.currentLine >= e.buffer.GetLineCount() {
		return
	}
	// Clear hardNewline so reflow can flow across the boundary
	e.buffer.SetHardNewline(e.currentLine, false)
	if e.buffer.JoinLines(e.currentLine) {
		e.modified = true
		newLine, newCol := e.wordWrapper.ReflowRange(e.currentLine, e.currentLine, e.currentCol)
		e.currentLine = newLine
		e.currentCol = newCol
	}
}

// splitLine splits the current line at the cursor (user-initiated Ctrl+N)
func (e *FSEditor) splitLine() {
	if e.buffer.SplitLine(e.currentLine, e.currentCol) {
		// Mark the split line as a hard newline (user-created break)
		e.buffer.SetHardNewline(e.currentLine, true)
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
	oldTopLine := e.topLine

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

	// If topLine changed (scrolling occurred), clear screen cache to force redraw
	if e.topLine != oldTopLine {
		e.screen.ClearCache()
		e.lastTopLine = e.topLine
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
