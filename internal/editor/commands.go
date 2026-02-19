package editor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stlalpha/vision3/internal/ansi"
)

// Footer/lightbar color constants for consistent editor UI styling.
const (
	footerTextColor    = "\x1b[37m"      // White for regular footer text
	footerSpecialColor = "\x1b[1;34m"    // Bright blue for special chars/punctuation
	lbSelected         = "\x1b[1;36;44m" // Bright cyan on blue bg (selected lightbar item)
	lbUnselected       = "\x1b[37m"      // White (unselected lightbar item)
)

// CommandType represents a special editor command
type CommandType int

const (
	CommandNone  CommandType = iota
	CommandSave              // /S - Save and exit
	CommandAbort             // /A - Abort editing
	CommandQuote             // /Q - Quote previous message
	CommandHelp              // /H or /? - Show help
	CommandView              // /V - View message (not implemented in this version)
)

// QuoteData holds message metadata for quoting
type QuoteData struct {
	From   string   // Message author
	Title  string   // Message subject/title
	Date   string   // Message date
	Time   string   // Message time
	IsAnon bool     // Anonymous flag
	Lines  []string // Message content lines
}

// CommandHandler handles special slash commands
type CommandHandler struct {
	screen      *Screen
	buffer      *MessageBuffer
	quoteData   *QuoteData // Message data to quote when /Q is used
	menuSetPath string     // Path to menu files
	yesNoHi     string     // ANSI sequence for highlighted Yes/No
	yesNoLo     string     // ANSI sequence for regular Yes/No
	yesText     string     // Configurable Yes label
	noText      string     // Configurable No label
	abortText   string     // Configurable abort confirmation prompt
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(screen *Screen, buffer *MessageBuffer, menuSetPath, yesNoHi, yesNoLo, yesText, noText, abortText string) *CommandHandler {
	yesText = strings.TrimSpace(yesText)
	if yesText == "" {
		yesText = "Yes"
	}

	noText = strings.TrimSpace(noText)
	if noText == "" {
		noText = "No"
	}

	abortText = strings.TrimSpace(abortText)
	if abortText == "" {
		abortText = "|14Abort message?"
	}

	return &CommandHandler{
		screen:      screen,
		buffer:      buffer,
		menuSetPath: menuSetPath,
		yesNoHi:     yesNoHi,
		yesNoLo:     yesNoLo,
		yesText:     yesText,
		noText:      noText,
		abortText:   abortText,
	}
}

// SetQuoteData sets the message data to be used for the /Q quote command
func (ch *CommandHandler) SetQuoteData(data *QuoteData) {
	ch.quoteData = data
}

// HandleSave handles the Save command (CTRL-Z).
// Returns true to signal save and exit; on false, an error is written to PromptRow.
func (ch *CommandHandler) HandleSave() bool {
	content := ch.buffer.GetContent()
	if strings.TrimSpace(content) == "" {
		ch.screen.GoXY(1, ch.screen.PromptRow())
		ch.screen.ClearEOL()
		ch.screen.WriteDirectProcessed("|12Cannot save empty message! Press any key...")
		return false
	}
	return true
}

// HandleAbort handles the Abort command (CTRL-A).
// Displays a lightbar Yes/No confirmation in the last footer row (PromptRow).
// Returns true to signal abort and exit; false restores the footer row and continues.
func (ch *CommandHandler) HandleAbort(inputHandler *InputHandler) bool {
	promptRow := ch.screen.PromptRow()

	// Without a footer, clear the row above the prompt as a visual separator.
	// With a footer the row above is the first footer row — leave it intact.
	if !ch.screen.HasFooter() && promptRow > 1 {
		ch.screen.GoXY(1, promptRow-1)
		ch.screen.ClearEOL()
	}

	// Write confirmation prompt to the last footer (or status) row.
	ch.screen.GoXY(1, promptRow)
	ch.screen.ClearEOL()
	ch.screen.WriteDirect(" ") // 1 col indent
	ch.screen.WriteDirectProcessed(ch.abortText)

	// Save cursor position for inline lightbar, hide cursor.
	ch.screen.WriteDirect("\x1b[s")
	ch.screen.WriteDirect("\x1b[?25l")
	defer ch.screen.WriteDirect("\x1b[?25h")

	selectedIndex := 0 // 0=No (default), 1=Yes

	drawInline := func(sel int) {
		ch.screen.WriteDirect("\x1b[u")
		yesColor := lbUnselected
		noColor := lbUnselected
		if sel == 1 {
			yesColor = lbSelected
		} else {
			noColor = lbSelected
		}
		ch.screen.WriteDirect("  " + yesColor + " " + ch.yesText + " " + "\x1b[0m" + "  " + noColor + " " + ch.noText + " " + "\x1b[0m")
	}

	drawInline(selectedIndex)

	for {
		key, err := inputHandler.ReadKey()
		if err != nil {
			ch.screen.DisplayFooter()
			return false
		}

		switch key {
		case 'Y', 'y':
			return true // caller exits; footer not needed
		case 'N', 'n':
			ch.screen.DisplayFooter() // restore footer row before continuing
			return false
		case ' ', KeyEnter:
			if selectedIndex == 1 {
				return true
			}
			ch.screen.DisplayFooter()
			return false
		case KeyArrowLeft, KeyArrowRight:
			selectedIndex = 1 - selectedIndex
			drawInline(selectedIndex)
		}
	}
}

// HandleQuote handles the Quote command (CTRL-Q).
// Follows Pascal flow: display message inline, prompt for line range, insert quote.
// Prompts appear in PromptRow (last footer row); footer is restored by the caller's FullRedraw.
func (ch *CommandHandler) HandleQuote(inputHandler *InputHandler, currentLine, currentCol int) (int, int) {
	promptRow := ch.screen.PromptRow()

	if ch.quoteData == nil || len(ch.quoteData.Lines) == 0 {
		ch.screen.GoXY(1, promptRow)
		ch.screen.ClearEOL()
		ch.screen.WriteDirectProcessed("|12You are not replying to anything! Press any key...")
		inputHandler.ReadKey()
		return currentLine, 1
	}

	// Display quote UI inline at current position (no screen clear).
	screenY := ch.screen.GetEditingStartY() + (currentLine - 1)
	ch.screen.GoXY(1, screenY)
	ch.screen.WriteDirectProcessed("|09Message # to Quote |03(|15Cr/1|03)|09: |151\r\n\r\n")
	ch.screen.WriteDirectProcessed("|09You are quoting |15" + ch.quoteData.Title + " |09by |15" + ch.quoteData.From + "\r\n\r\n")

	maxLines := len(ch.quoteData.Lines)
	for i := 0; i < maxLines && i < 99; i++ {
		lineText := ch.quoteData.Lines[i]
		if len(lineText) > 75 {
			lineText = lineText[:75]
		}
		ch.screen.WriteDirectProcessed(fmt.Sprintf("|12%d|10: |15%s\r\n", i+1, lineText))
	}

	// Prompt for start line in the last footer row.
	ch.screen.GoXY(1, promptRow)
	ch.screen.ClearEOL()
	ch.screen.WriteDirectProcessed(fmt.Sprintf("|09Start Quoting @ |03(|151|03-|15%d|03)|09, |03Q|09=quit |09: ", maxLines))

	clearPrompt := func() {
		ch.screen.GoXY(1, promptRow)
		ch.screen.ClearEOL()
	}

	startStr := ""
	key, err := inputHandler.ReadKey()
	if err != nil {
		clearPrompt()
		return currentLine, 1
	}
	for key != KeyEnter && key != KeyEsc {
		if key >= '0' && key <= '9' {
			startStr += string(rune(key))
			ch.screen.WriteDirect(string(rune(key)))
		} else if key == KeyBackspace && len(startStr) > 0 {
			startStr = startStr[:len(startStr)-1]
			ch.screen.WriteDirect("\b \b")
		} else if key == 'Q' || key == 'q' {
			clearPrompt()
			return currentLine, 1
		}
		key, err = inputHandler.ReadKey()
		if err != nil {
			clearPrompt()
			return currentLine, 1
		}
	}
	if key == KeyEsc {
		clearPrompt()
		return currentLine, 1
	}
	if startStr == "" {
		startStr = "1"
	}

	startLine := 1
	if n, err := strconv.Atoi(startStr); err == nil && n > 0 && n <= maxLines {
		startLine = n
	}

	// Prompt for end line.
	maxEnd := startLine + 20
	if maxEnd > maxLines {
		maxEnd = maxLines
	}
	ch.screen.GoXY(1, promptRow)
	ch.screen.ClearEOL()
	ch.screen.WriteDirectProcessed(fmt.Sprintf("|09End Quoting @ |03(|15%d|03-|15%d|03)|09, |03Q|09=quit |09: ", startLine, maxEnd))

	endStr := ""
	key, err = inputHandler.ReadKey()
	if err != nil {
		clearPrompt()
		return currentLine, 1
	}
	for key != KeyEnter && key != KeyEsc {
		if key >= '0' && key <= '9' {
			endStr += string(rune(key))
			ch.screen.WriteDirect(string(rune(key)))
		} else if key == KeyBackspace && len(endStr) > 0 {
			endStr = endStr[:len(endStr)-1]
			ch.screen.WriteDirect("\b \b")
		} else if key == 'Q' || key == 'q' {
			clearPrompt()
			return currentLine, 1
		}
		key, err = inputHandler.ReadKey()
		if err != nil {
			clearPrompt()
			return currentLine, 1
		}
	}
	if key == KeyEsc {
		clearPrompt()
		return currentLine, 1
	}
	if endStr == "" {
		endStr = strconv.Itoa(maxEnd)
	}

	endLine := maxEnd
	if n, err := strconv.Atoi(endStr); err == nil && n >= startLine && n <= maxEnd {
		endLine = n
	}

	clearPrompt()

	// Format the quote with header and footer
	insertLine := currentLine

	// Insert quote header: "--- [Name] Said ---"
	// Using ASCII hyphens (CP437 box drawing character 0xC4 causes UTF-8 issues)
	// Colors: |08 = dark gray, |15 = bright white, |07 = light gray
	quoteTop := ch.processQuoteCodes("|08--- |15^N |07Said |08---")
	quoteTop = ch.processForBuffer(quoteTop)
	if insertLine <= MaxLines {
		if insertLine > ch.buffer.GetLineCount() {
			ch.buffer.InsertLine(insertLine)
		}
		ch.buffer.SetLine(insertLine, quoteTop)
		insertLine++
	}

	// Insert quoted lines with space prefix
	for i := startLine - 1; i < endLine && i < len(ch.quoteData.Lines); i++ {
		if insertLine > MaxLines {
			break
		}
		if insertLine > ch.buffer.GetLineCount() {
			ch.buffer.InsertLine(insertLine)
		}

		// Prefix with space and truncate to 79 chars
		quotedLine := " " + ch.filterPipeCodes(ch.quoteData.Lines[i])
		if len(quotedLine) > 79 {
			quotedLine = quotedLine[:79]
		}
		ch.buffer.SetLine(insertLine, quotedLine)
		insertLine++
	}

	// Insert quote footer: "--- [Name] Done ---"
	// Add |07 at end to reset color to light gray for continued editing
	quoteBottom := ch.processQuoteCodes("|08--- |15^N |07Done |08---|07")
	quoteBottom = ch.processForBuffer(quoteBottom)
	if insertLine <= MaxLines {
		if insertLine > ch.buffer.GetLineCount() {
			ch.buffer.InsertLine(insertLine)
		}
		ch.buffer.SetLine(insertLine, quoteBottom)
		insertLine++
	}

	// Position cursor after quoted text
	return insertLine, 1
}

// processQuoteCodes processes ^N, ^T, ^D, ^W codes in quote strings
func (ch *CommandHandler) processQuoteCodes(text string) string {
	result := ""
	i := 0
	for i < len(text) {
		if text[i] == '^' && i+1 < len(text) {
			code := text[i+1]
			switch code {
			case 'N', 'n':
				if ch.quoteData.IsAnon {
					result += "Anonymous"
				} else {
					result += ch.quoteData.From
				}
				i += 2
			case 'T', 't':
				result += ch.quoteData.Title
				i += 2
			case 'D', 'd':
				result += ch.quoteData.Date
				i += 2
			case 'W', 'w':
				result += ch.quoteData.Time
				i += 2
			default:
				result += string(text[i])
				i++
			}
		} else {
			result += string(text[i])
			i++
		}
	}
	return result
}

// filterPipeCodes removes pipe codes from quoted text if needed
func (ch *CommandHandler) filterPipeCodes(text string) string {
	// For now, always filter pipe codes from quoted text
	result := text
	i := len(result) - 2
	for i >= 0 {
		if result[i] == '|' && i+2 < len(result) {
			// Check if next two chars are digits
			if result[i+1] >= '0' && result[i+1] <= '9' && result[i+2] >= '0' && result[i+2] <= '9' {
				result = result[:i] + result[i+3:]
			}
		}
		i--
	}
	return result
}

// processForBuffer processes pipe codes to ANSI escape sequences for buffer storage
// This is needed because buffer content is displayed without pipe code processing
func (ch *CommandHandler) processForBuffer(text string) string {
	// Convert pipe codes to ANSI escape sequences
	processed := ansi.ReplacePipeCodes([]byte(text))
	return string(processed)
}

// HandleHelp handles the /H (help) command
// Displays the help screen
func (ch *CommandHandler) HandleHelp(inputHandler *InputHandler) {
	// Try to load EDITHELP.ANS file
	helpPath := filepath.Join(ch.menuSetPath, "ansi", "EDITHELP.ANS")
	helpContent, err := ansi.GetAnsiFileContent(helpPath)

	ch.screen.ClearScreen()

	if err == nil {
		// Display the help file
		ch.screen.WriteDirect(string(helpContent))
	} else {
		// Display built-in help
		ch.displayBuiltInHelp()
	}

	// Wait for key press
	ch.screen.GoXY(1, ch.screen.termHeight)
	ch.screen.WriteDirectProcessed("|15Press any key to continue...")
	inputHandler.ReadKey()
}

// displayBuiltInHelp displays built-in help text
func (ch *CommandHandler) displayBuiltInHelp() {
	help := `|15Full Screen Message Editor Help|07

|11Navigation Commands:|07
  Ctrl+E or Up Arrow     - Move up one line
  Ctrl+X or Down Arrow   - Move down one line
  Ctrl+S or Left Arrow   - Move left one character
  Ctrl+D or Right Arrow  - Move right one character
  Ctrl+W or Home         - Move to start of line
  Ctrl+P or End          - Move to end of line
  Ctrl+R or Page Up      - Scroll up one page
  Ctrl+C or Page Down    - Scroll down one page
  Ctrl+A                 - Move left one word
  Ctrl+F                 - Move right one word

|11Edit Commands:|07
  Ctrl+V or Insert       - Toggle Insert/Overwrite mode
  Ctrl+G or Delete       - Delete character at cursor
  Ctrl+T                 - Delete word to the right
  Ctrl+Y                 - Delete current line
  Ctrl+J                 - Join current line with next
  Ctrl+N                 - Split line at cursor
  Ctrl+B                 - Reformat paragraph
  Ctrl+L                 - Redraw screen
  Backspace              - Delete character to the left
  Tab                    - Insert tab (spaces)

|11Special Commands:|07
  /S                     - Save message and exit
  /A                     - Abort message (with confirmation)
  /Q                     - Quote previous message (when replying)
  /H or /?               - Display this help

|11Word Wrapping:|07
  Lines automatically wrap at 79 characters.
  Words are kept together when wrapping.
  Use Ctrl+B to reformat paragraphs.

`
	ch.screen.WriteDirectProcessed(help)
}

// HandleView handles the /V (view) command
// Displays the current message (not fully implemented)
func (ch *CommandHandler) HandleView(inputHandler *InputHandler) {
	ch.screen.ClearScreen()

	// Display message
	ch.screen.WriteDirectProcessed("|15Current Message:|07\r\n\r\n")

	lineCount := ch.buffer.GetLineCount()
	for i := 1; i <= lineCount; i++ {
		line := ch.buffer.GetLine(i)
		ch.screen.WriteDirect(line + "\r\n")
	}

	// Wait for key press
	ch.screen.GoXY(1, ch.screen.termHeight)
	ch.screen.WriteDirectProcessed("|15Press any key to continue...")
	inputHandler.ReadKey()
}

// ShowEscapeMenu displays a lightbar selection menu at PromptRow when Escape is pressed.
// Items: Save, Abort, Edit (continue), Help, Quote.
// Returns the selected CommandType, or CommandNone to continue editing.
func (ch *CommandHandler) ShowEscapeMenu(inputHandler *InputHandler) CommandType {
	type menuItem struct {
		label   string
		cmdType CommandType
	}
	items := []menuItem{
		{"Save", CommandSave},
		{"Abort", CommandAbort},
		{"Edit", CommandNone},
		{"Help", CommandHelp},
		{"Quote", CommandQuote},
	}

	promptRow := ch.screen.PromptRow()
	selectedIndex := 2 // Default: "Edit" (continue editing)

	ch.screen.WriteDirect("\x1b[?25l") // hide cursor
	defer ch.screen.WriteDirect("\x1b[?25h")

	drawMenu := func(sel int) {
		ch.screen.GoXY(1, promptRow)
		ch.screen.ClearEOL()
		ch.screen.WriteDirect(" " + footerTextColor + "Select an Option" + footerSpecialColor + ":" + footerTextColor + " ")
		for i, item := range items {
			if i > 0 {
				ch.screen.WriteDirect("  ") // 2 spaces between items
			}
			if i == sel {
				ch.screen.WriteDirect(lbSelected + " " + item.label + " " + "\x1b[0m")
			} else {
				ch.screen.WriteDirect(lbUnselected + " " + item.label + " " + "\x1b[0m")
			}
		}
	}

	drawMenu(selectedIndex)

	for {
		key, err := inputHandler.ReadKey()
		if err != nil {
			ch.screen.DisplayFooter()
			return CommandNone
		}
		switch key {
		case KeyArrowLeft:
			if selectedIndex > 0 {
				selectedIndex--
				drawMenu(selectedIndex)
			}
		case KeyArrowRight:
			if selectedIndex < len(items)-1 {
				selectedIndex++
				drawMenu(selectedIndex)
			}
		case KeyEnter, ' ':
			ch.screen.DisplayFooter()
			return items[selectedIndex].cmdType
		case KeyEsc:
			ch.screen.DisplayFooter()
			return CommandNone
		}
	}
}
