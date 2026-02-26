package editor

import (
	"strings"
)

const (
	// MaxLines is the maximum number of lines in a message (100 lines, 1-based indexing)
	MaxLines = 100
	// MaxLineLength is the maximum length of a line before word wrap (79 chars)
	MaxLineLength = 79
)

// MessageBuffer manages the text content of the message being edited
type MessageBuffer struct {
	lines       [MaxLines + 1]string // 1-based indexing, line 0 unused
	hardNewline [MaxLines + 1]bool   // true = user-created line break (Enter); false = soft wrap
	lineCount   int                  // Number of lines with content
}

// NewMessageBuffer creates a new message buffer
func NewMessageBuffer() *MessageBuffer {
	return &MessageBuffer{
		lineCount: 1, // Start with at least one line
	}
}

// IsHardNewline returns whether the line ends with a hard return (user-created).
func (mb *MessageBuffer) IsHardNewline(lineNum int) bool {
	if lineNum < 1 || lineNum > MaxLines {
		return false
	}
	return mb.hardNewline[lineNum]
}

// SetHardNewline sets or clears the hard-newline flag for a line.
func (mb *MessageBuffer) SetHardNewline(lineNum int, hard bool) {
	if lineNum < 1 || lineNum > MaxLines {
		return
	}
	mb.hardNewline[lineNum] = hard
}

// LoadContent loads initial content into the buffer
func (mb *MessageBuffer) LoadContent(content string) {
	mb.lines = [MaxLines + 1]string{}
	mb.hardNewline = [MaxLines + 1]bool{}
	mb.lineCount = 1

	if content == "" {
		return
	}

	lines := strings.Split(content, "\n")
	count := 0
	for i, line := range lines {
		if i >= MaxLines {
			break
		}
		mb.lines[i+1] = line // 1-based indexing
		mb.hardNewline[i+1] = true // loaded content = real line breaks
		count++
	}

	if count == 0 {
		count = 1
	}
	mb.lineCount = count
}

// GetLine returns the content of a line (1-based)
func (mb *MessageBuffer) GetLine(lineNum int) string {
	if lineNum < 1 || lineNum > MaxLines {
		return ""
	}
	return mb.lines[lineNum]
}

// SetLine sets the content of a line (1-based)
func (mb *MessageBuffer) SetLine(lineNum int, content string) {
	if lineNum < 1 || lineNum > MaxLines {
		return
	}
	mb.lines[lineNum] = content

	// Update line count if necessary
	if lineNum > mb.lineCount {
		mb.lineCount = lineNum
	}
}

// GetLineCount returns the number of navigable lines (includes trailing blank
// lines created by the user). Used for cursor bounds and navigation.
func (mb *MessageBuffer) GetLineCount() int {
	if mb.lineCount < 1 {
		return 1
	}
	return mb.lineCount
}

// GetContentLineCount returns the line count for content output, trimming
// trailing empty lines. Used by GetContent() for storage.
func (mb *MessageBuffer) GetContentLineCount() int {
	count := mb.lineCount
	for count > 0 && strings.TrimSpace(mb.lines[count]) == "" {
		count--
	}
	if count == 0 {
		count = 1
	}
	return count
}

// GetLineLength returns the length of a line
func (mb *MessageBuffer) GetLineLength(lineNum int) int {
	if lineNum < 1 || lineNum > MaxLines {
		return 0
	}
	return len(mb.lines[lineNum])
}

// InsertChar inserts a character at the specified position (1-based line and col)
func (mb *MessageBuffer) InsertChar(lineNum, col int, ch rune) bool {
	if lineNum < 1 || lineNum > MaxLines {
		return false
	}

	line := mb.lines[lineNum]

	// Ensure col is valid
	if col < 1 {
		col = 1
	}
	if col > len(line)+1 {
		col = len(line) + 1
	}

	// Insert character
	if col == 1 {
		line = string(ch) + line
	} else if col > len(line) {
		// Pad with spaces if needed and append
		for len(line) < col-1 {
			line += " "
		}
		line += string(ch)
	} else {
		line = line[:col-1] + string(ch) + line[col-1:]
	}

	mb.lines[lineNum] = line

	// Update line count if needed
	if lineNum > mb.lineCount {
		mb.lineCount = lineNum
	}

	return true
}

// DeleteChar deletes a character at the specified position (1-based)
func (mb *MessageBuffer) DeleteChar(lineNum, col int) bool {
	if lineNum < 1 || lineNum > MaxLines {
		return false
	}

	line := mb.lines[lineNum]
	if col < 1 || col > len(line) {
		return false
	}

	// Delete character
	mb.lines[lineNum] = line[:col-1] + line[col:]

	return true
}

// OverwriteChar overwrites a character at the specified position (1-based)
func (mb *MessageBuffer) OverwriteChar(lineNum, col int, ch rune) bool {
	if lineNum < 1 || lineNum > MaxLines {
		return false
	}

	line := mb.lines[lineNum]

	// Ensure col is valid
	if col < 1 {
		col = 1
	}

	// Pad with spaces if needed
	for len(line) < col-1 {
		line += " "
	}

	if col > len(line) {
		line += string(ch)
	} else {
		line = line[:col-1] + string(ch) + line[col:]
	}

	mb.lines[lineNum] = line

	// Update line count if needed
	if lineNum > mb.lineCount {
		mb.lineCount = lineNum
	}

	return true
}

// InsertLine inserts a new blank line at the specified position (1-based)
func (mb *MessageBuffer) InsertLine(lineNum int) bool {
	if lineNum < 1 || lineNum > MaxLines || mb.lineCount >= MaxLines {
		return false
	}

	// Shift lines and hardNewline flags down from the insertion point
	for i := mb.lineCount; i >= lineNum; i-- {
		if i+1 <= MaxLines {
			mb.lines[i+1] = mb.lines[i]
			mb.hardNewline[i+1] = mb.hardNewline[i]
		}
	}

	// Clear the new line
	mb.lines[lineNum] = ""
	mb.hardNewline[lineNum] = false
	mb.lineCount++

	return true
}

// DeleteLine deletes a line at the specified position (1-based)
func (mb *MessageBuffer) DeleteLine(lineNum int) bool {
	if lineNum < 1 || lineNum > mb.lineCount {
		return false
	}

	// Shift lines and hardNewline flags up from the deletion point
	for i := lineNum; i < mb.lineCount; i++ {
		mb.lines[i] = mb.lines[i+1]
		mb.hardNewline[i] = mb.hardNewline[i+1]
	}

	// Clear the last line
	mb.lines[mb.lineCount] = ""
	mb.hardNewline[mb.lineCount] = false
	mb.lineCount--

	// Always have at least one line
	if mb.lineCount < 1 {
		mb.lineCount = 1
		mb.lines[1] = ""
	}

	return true
}

// SplitLine splits a line at the specified column (1-based).
// The split line (lineNum) gets hardNewline=false; the caller sets it to true
// for user-initiated splits (Enter, Ctrl+N). The new line (lineNum+1) inherits
// the original hardNewline value of lineNum.
// Returns true if successful.
func (mb *MessageBuffer) SplitLine(lineNum, col int) bool {
	if lineNum < 1 || lineNum > mb.lineCount || mb.lineCount >= MaxLines {
		return false
	}

	line := mb.lines[lineNum]
	oldHardNewline := mb.hardNewline[lineNum]

	// Split the line at the column
	var leftPart, rightPart string
	if col <= 1 {
		leftPart = ""
		rightPart = line
	} else if col > len(line) {
		leftPart = line
		rightPart = ""
	} else {
		leftPart = line[:col-1]
		rightPart = line[col-1:]
	}

	// Set the current line to the left part
	mb.lines[lineNum] = leftPart

	// Insert a new line for the right part
	if !mb.InsertLine(lineNum + 1) {
		// Restore if insertion failed
		mb.lines[lineNum] = line
		mb.hardNewline[lineNum] = oldHardNewline
		return false
	}
	mb.lines[lineNum+1] = rightPart

	// Split line gets false (caller decides); new line inherits old value
	mb.hardNewline[lineNum] = false
	mb.hardNewline[lineNum+1] = oldHardNewline

	return true
}

// JoinLines joins the current line with the next line.
// The combined line inherits hardNewline from the second line (lineNum+1),
// since the end of the combined line is where line lineNum+1 used to end.
func (mb *MessageBuffer) JoinLines(lineNum int) bool {
	if lineNum < 1 || lineNum >= mb.lineCount {
		return false
	}

	// Save the second line's hardNewline before it gets shifted by DeleteLine
	nextHardNewline := mb.hardNewline[lineNum+1]

	// Combine the two lines
	combined := mb.lines[lineNum] + mb.lines[lineNum+1]
	mb.lines[lineNum] = combined

	// Delete the next line
	if !mb.DeleteLine(lineNum + 1) {
		return false
	}

	// Combined line inherits the second line's hard newline status
	mb.hardNewline[lineNum] = nextHardNewline
	return true
}

// RemoveTrailingSpaces removes trailing spaces from a line
func (mb *MessageBuffer) RemoveTrailingSpaces(lineNum int) {
	if lineNum < 1 || lineNum > MaxLines {
		return
	}
	mb.lines[lineNum] = strings.TrimRight(mb.lines[lineNum], " ")
}

// GetContent returns the entire buffer content as a string.
// Uses GetContentLineCount to trim trailing empty lines for storage.
func (mb *MessageBuffer) GetContent() string {
	var result strings.Builder

	lineCount := mb.GetContentLineCount()
	for i := 1; i <= lineCount; i++ {
		if i > 1 {
			result.WriteString("\n")
		}
		result.WriteString(mb.lines[i])
	}

	return result.String()
}

// Clear clears the buffer
func (mb *MessageBuffer) Clear() {
	for i := 1; i <= MaxLines; i++ {
		mb.lines[i] = ""
		mb.hardNewline[i] = false
	}
	mb.lineCount = 1
}

// GetLastChar returns the last character on a line
func (mb *MessageBuffer) GetLastChar(lineNum int) rune {
	if lineNum < 1 || lineNum > MaxLines {
		return ' '
	}
	line := mb.lines[lineNum]
	if len(line) == 0 {
		return ' '
	}
	return rune(line[len(line)-1])
}

// GetCharAt returns the character at a specific position (1-based)
func (mb *MessageBuffer) GetCharAt(lineNum, col int) rune {
	if lineNum < 1 || lineNum > MaxLines {
		return ' '
	}
	line := mb.lines[lineNum]
	if col < 1 || col > len(line) {
		return ' '
	}
	return rune(line[col-1])
}

// IsLineEmpty returns true if a line is empty or contains only whitespace
func (mb *MessageBuffer) IsLineEmpty(lineNum int) bool {
	if lineNum < 1 || lineNum > MaxLines {
		return true
	}
	return strings.TrimSpace(mb.lines[lineNum]) == ""
}
