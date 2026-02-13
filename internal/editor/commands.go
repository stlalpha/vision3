package editor

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/robbiew/vision3/internal/ansi"
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

// ShowSlashMenu displays the slash command menu and waits for user input
// Returns the command type selected by the user
func (ch *CommandHandler) ShowSlashMenu(inputHandler *InputHandler, currentLine, currentCol int) CommandType {
	// Save cursor position
	ch.screen.GoXY(currentCol, ch.screen.GetEditingStartY()+(currentLine-1))

	// Display the slash menu - show /Q only if quoting is available
	// |11 = bright cyan for hotkeys, |03 = cyan for text, |08 = dark gray for separators
	var menuText string
	hasQuotedText := ch.quoteData != nil && len(ch.quoteData.Lines) > 0

	if hasQuotedText {
		// Include Quote option when replying
		menuText = "|11S|03ave|08/|11Q|03uote|08/|11A|03bort|08/|11H|03elp : "
	} else {
		// No Quote option for new messages
		menuText = "|11S|03ave|08/|11A|03bort|08/|11H|03elp : "
	}
	ch.screen.WriteDirectProcessed(menuText)

	// Wait for user to press a command key
	for {
		key, err := inputHandler.ReadKey()
		if err != nil {
			return CommandNone
		}

		// Convert to uppercase for comparison
		keyUpper := key
		if key >= 'a' && key <= 'z' {
			keyUpper = key - 32
		}

		switch keyUpper {
		case 'S':
			return CommandSave
		case 'A':
			return CommandAbort
		case 'Q':
			// Only allow quote if quoted text is available
			if hasQuotedText {
				return CommandQuote
			}
			// Invalid key if no quoted text - continue waiting
		case 'H', '?':
			return CommandHelp
		case 'V':
			return CommandView
		case KeyEsc:
			return CommandNone
		default:
			// Invalid key - just return none
			return CommandNone
		}
	}
}

// ClearSlashMenu clears the slash command menu from the screen
func (ch *CommandHandler) ClearSlashMenu(lineNum int, menuLength int) {
	// Position at the start of the menu
	screenY := ch.screen.GetEditingStartY() + (lineNum - 1)
	ch.screen.GoXY(1, screenY)
	ch.screen.ClearEOL()
}

// HandleSave handles the /S (save) command
// Returns true to signal save and exit
func (ch *CommandHandler) HandleSave() bool {
	// Check if message has content
	content := ch.buffer.GetContent()
	if strings.TrimSpace(content) == "" {
		ch.screen.GoXY(1, ch.screen.statusLineY)
		ch.screen.WriteDirectProcessed("|12Cannot save empty message! Press any key...")
		return false
	}

	// Signal to save and exit
	return true
}

// HandleAbort handles the /A (abort) command
// Returns true to signal abort and exit
func (ch *CommandHandler) HandleAbort(inputHandler *InputHandler) bool {
	// Display one blank row, then the confirmation prompt.
	if ch.screen.statusLineY > 1 {
		ch.screen.GoXY(1, ch.screen.statusLineY-1)
		ch.screen.ClearEOL()
	}

	// Display confirmation prompt
	ch.screen.GoXY(1, ch.screen.statusLineY)
	ch.screen.ClearEOL()
	ch.screen.WriteDirectProcessed(ch.abortText)

	// Save cursor position, hide cursor, then render inline lightbar.
	ch.screen.WriteDirect("\x1b[s")
	ch.screen.WriteDirect("\x1b[?25l")
	defer ch.screen.WriteDirect("\x1b[?25h")

	selectedIndex := 0 // 0=No (default), 1=Yes

	drawInline := func(sel int) {
		ch.screen.WriteDirect("\x1b[u")
		yesColor, noColor := ch.yesNoLo, ch.yesNoLo
		if sel == 1 {
			yesColor = ch.yesNoHi
		} else {
			noColor = ch.yesNoHi
		}
		ch.screen.WriteDirect(" " + yesColor + ch.yesText + " " + "\x1b[0m" + " " + noColor + ch.noText + " " + "\x1b[0m")
	}

	drawInline(selectedIndex)

	for {
		key, err := inputHandler.ReadKey()
		if err != nil {
			return false
		}

		switch key {
		case 'Y', 'y':
			return true
		case 'N', 'n':
			ch.screen.ClearEOL()
			return false
		case ' ', KeyEnter:
			if selectedIndex == 1 {
				return true
			}
			ch.screen.ClearEOL()
			return false
		case KeyArrowLeft, KeyArrowRight:
			selectedIndex = 1 - selectedIndex
			drawInline(selectedIndex)
		default:
			// Ignore other input until a selection is made.
		}
	}
}

// HandleQuote handles the /Q (quote) command
// Follows Pascal flow: display message inline, prompt for lines, insert quote
func (ch *CommandHandler) HandleQuote(inputHandler *InputHandler, currentLine, currentCol int) (int, int) {
	// Clear the /Q command line
	ch.buffer.SetLine(currentLine, "")

	if ch.quoteData == nil || len(ch.quoteData.Lines) == 0 {
		// No quoted text available (shouldn't happen if menu is correct)
		ch.screen.GoXY(1, ch.screen.statusLineY)
		ch.screen.WriteDirectProcessed("|12You are not replying to anything! Press any key...")
		inputHandler.ReadKey()
		return currentLine, 1
	}

	// Display quote UI inline at current position (NO screen clear!)
	// Position at start of current line
	screenY := ch.screen.GetEditingStartY() + (currentLine - 1)
	ch.screen.GoXY(1, screenY)

	// Display "Message # to Quote" prompt (matching Pascal)
	ch.screen.WriteDirectProcessed("|09Message # to Quote |03(|15Cr/1|03)|09: |151\r\n\r\n")

	// Display quote title: "You are quoting |ST by Author"
	ch.screen.WriteDirectProcessed("|09You are quoting |15" + ch.quoteData.Title + " |09by |15" + ch.quoteData.From + "\r\n\r\n")

	// Display all message lines with line numbers (matching Pascal colors)
	// Pascal: ^R (red/|12) for number, ^A (bright/|10) for colon, ^S (bright/|15) for text
	maxLines := len(ch.quoteData.Lines)
	for i := 0; i < maxLines && i < 99; i++ {
		lineText := ch.quoteData.Lines[i]
		// Truncate long lines for display
		if len(lineText) > 75 {
			lineText = lineText[:75]
		}
		// Match Pascal colors exactly
		ch.screen.WriteDirectProcessed(fmt.Sprintf("|12%d|10: |15%s\r\n", i+1, lineText))
	}

	// Prompt for start line at status line (matching Pascal: "Start Quoting @ (1-X) :")
	ch.screen.GoXY(1, ch.screen.statusLineY)
	ch.screen.ClearEOL()
	ch.screen.WriteDirectProcessed(fmt.Sprintf("|09Start Quoting @ |03(|151|03-|15%d|03) |09: ", maxLines))

	startStr := ""
	key, _ := inputHandler.ReadKey()
	for key != KeyEnter && key != KeyEsc {
		if key >= '0' && key <= '9' {
			startStr += string(rune(key))
			ch.screen.WriteDirect(string(rune(key)))
		} else if key == KeyBackspace && len(startStr) > 0 {
			startStr = startStr[:len(startStr)-1]
			ch.screen.WriteDirect("\b \b")
		} else if key == 'Q' || key == 'q' {
			// Allow 'Q' to quit - return to editor
			ch.screen.GoXY(1, ch.screen.statusLineY)
			ch.screen.ClearEOL()
			return currentLine, 1
		}
		key, _ = inputHandler.ReadKey()
	}
	if key == KeyEsc {
		ch.screen.GoXY(1, ch.screen.statusLineY)
		ch.screen.ClearEOL()
		return currentLine, 1
	}
	if startStr == "" {
		startStr = "1"
	}

	startLine := 1
	if n, err := strconv.Atoi(startStr); err == nil && n > 0 && n <= maxLines {
		startLine = n
	}

	// Prompt for end line at status line
	maxEnd := startLine + 20 // Limit quote length (Pascal uses Cfg.MaxQuotedLines)
	if maxEnd > maxLines {
		maxEnd = maxLines
	}
	ch.screen.GoXY(1, ch.screen.statusLineY)
	ch.screen.ClearEOL()
	ch.screen.WriteDirectProcessed(fmt.Sprintf("|09End Quoting @ |03(|15%d|03-|15%d|03) |09: ", startLine, maxEnd))

	endStr := ""
	key, _ = inputHandler.ReadKey()
	for key != KeyEnter && key != KeyEsc {
		if key >= '0' && key <= '9' {
			endStr += string(rune(key))
			ch.screen.WriteDirect(string(rune(key)))
		} else if key == KeyBackspace && len(endStr) > 0 {
			endStr = endStr[:len(endStr)-1]
			ch.screen.WriteDirect("\b \b")
		} else if key == 'Q' || key == 'q' {
			// Allow 'Q' to quit - return to editor
			ch.screen.GoXY(1, ch.screen.statusLineY)
			ch.screen.ClearEOL()
			return currentLine, 1
		}
		key, _ = inputHandler.ReadKey()
	}
	if key == KeyEsc {
		ch.screen.GoXY(1, ch.screen.statusLineY)
		ch.screen.ClearEOL()
		return currentLine, 1
	}
	if endStr == "" {
		endStr = strconv.Itoa(maxEnd)
	}

	endLine := maxEnd
	if n, err := strconv.Atoi(endStr); err == nil && n >= startLine && n <= maxEnd {
		endLine = n
	}

	// Clear status line after getting input
	ch.screen.GoXY(1, ch.screen.statusLineY)
	ch.screen.ClearEOL()

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
	for i > 0 {
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

// ClearCommandLine clears the current command line and prepares for editing
func (ch *CommandHandler) ClearCommandLine(lineNum int) {
	ch.buffer.SetLine(lineNum, "")
}

// IsCommandChar returns true if the character could start a command
func IsCommandChar(ch rune) bool {
	return ch == '/'
}

// ParseCommandLine parses a command line to determine if it's complete
// Returns the command type if complete, CommandNone otherwise
func ParseCommandLine(line string) CommandType {
	trimmed := strings.TrimSpace(strings.ToUpper(line))

	switch trimmed {
	case "/S":
		return CommandSave
	case "/A":
		return CommandAbort
	case "/Q":
		return CommandQuote
	case "/H", "/?":
		return CommandHelp
	case "/V":
		return CommandView
	default:
		return CommandNone
	}
}
