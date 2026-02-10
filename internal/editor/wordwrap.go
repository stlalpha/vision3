package editor

import (
	"strings"
	"unicode"
)

// WordWrapper handles word wrapping and paragraph formatting
type WordWrapper struct {
	buffer *MessageBuffer
}

// NewWordWrapper creates a new word wrapper
func NewWordWrapper(buffer *MessageBuffer) *WordWrapper {
	return &WordWrapper{
		buffer: buffer,
	}
}

// CheckAndWrap checks if the current line exceeds the maximum length
// and performs word wrapping if necessary
// Returns the new cursor position (line, col)
func (ww *WordWrapper) CheckAndWrap(lineNum, col int) (int, int) {
	line := ww.buffer.GetLine(lineNum)

	// Check if line exceeds maximum length
	if len(line) <= MaxLineLength {
		return lineNum, col
	}

	// Need to wrap - find the last space before or at position 79
	wrapPos := ww.findWrapPosition(line)

	if wrapPos <= 0 {
		// No space found - force wrap at MaxLineLength
		wrapPos = MaxLineLength
	}

	// Split the line
	leftPart := strings.TrimRight(line[:wrapPos], " ")
	rightPart := strings.TrimLeft(line[wrapPos:], " ")

	// Update current line with left part
	ww.buffer.SetLine(lineNum, leftPart)

	// Check if we're at the last line or need to insert a new line
	nextLine := lineNum + 1
	lineCount := ww.buffer.GetLineCount()

	if nextLine <= lineCount {
		// Join with existing next line
		existingNext := ww.buffer.GetLine(nextLine)
		if len(existingNext) > 0 {
			combined := rightPart + " " + existingNext
			ww.buffer.SetLine(nextLine, combined)
			// Recursively check if the next line also needs wrapping
			return ww.CheckAndWrap(nextLine, 1)
		} else {
			ww.buffer.SetLine(nextLine, rightPart)
		}
	} else {
		// Insert new line
		if ww.buffer.InsertLine(nextLine) {
			ww.buffer.SetLine(nextLine, rightPart)
		}
	}

	// Adjust cursor position
	if col > wrapPos {
		// Cursor was in the wrapped part - move to next line
		newCol := col - wrapPos
		if rightPart != "" && rightPart[0] == ' ' {
			newCol-- // Adjust for removed space
		}
		if newCol < 1 {
			newCol = 1
		}
		return nextLine, newCol
	}

	// Cursor stays on current line
	return lineNum, col
}

// findWrapPosition finds the best position to wrap the line
// Returns the position after the last space before MaxLineLength
func (ww *WordWrapper) findWrapPosition(line string) int {
	// Find the last space at or before position MaxLineLength
	for i := MaxLineLength; i > 0; i-- {
		if i < len(line) && line[i] == ' ' {
			return i
		}
	}

	// If no space found, look for first space after MaxLineLength
	for i := MaxLineLength; i < len(line); i++ {
		if line[i] == ' ' {
			return i
		}
	}

	// No space found at all
	return -1
}

// ReformatParagraph reformats a paragraph starting at the specified line
// This joins lines and redistributes words to maintain proper wrapping
func (ww *WordWrapper) ReformatParagraph(startLine int) int {
	// Find the extent of the paragraph (until blank line or end)
	endLine := startLine
	lineCount := ww.buffer.GetLineCount()

	for endLine < lineCount && !ww.buffer.IsLineEmpty(endLine+1) {
		endLine++
	}

	if endLine == startLine && ww.buffer.IsLineEmpty(startLine) {
		return startLine // Empty line, nothing to reformat
	}

	// Collect all words from the paragraph
	words := make([]string, 0)
	for i := startLine; i <= endLine; i++ {
		line := ww.buffer.GetLine(i)
		lineWords := strings.Fields(line)
		words = append(words, lineWords...)
	}

	if len(words) == 0 {
		return startLine
	}

	// Rebuild lines with proper wrapping
	currentLine := startLine
	currentText := ""

	for _, word := range words {
		testLine := currentText
		if len(currentText) > 0 {
			testLine += " "
		}
		testLine += word

		if len(testLine) > MaxLineLength {
			// Would exceed limit - save current line and start new one
			if currentText != "" {
				ww.buffer.SetLine(currentLine, currentText)
				currentLine++

				// Insert line if needed
				if currentLine > endLine {
					if currentLine <= MaxLines && ww.buffer.InsertLine(currentLine) {
						endLine++
					} else {
						break // Can't insert more lines
					}
				}
			}
			currentText = word
		} else {
			currentText = testLine
		}
	}

	// Save last line
	if currentText != "" {
		ww.buffer.SetLine(currentLine, currentText)
		currentLine++
	}

	// Clear any remaining lines that were part of the paragraph
	for i := currentLine; i <= endLine; i++ {
		ww.buffer.SetLine(i, "")
	}

	return currentLine - 1 // Return last line of reformatted paragraph
}

// HandleBackspace handles backspace at the given position
// May trigger line joining if at start of line
// Returns new cursor position (line, col)
func (ww *WordWrapper) HandleBackspace(lineNum, col int) (int, int, bool) {
	if col > 1 {
		// Delete character before cursor
		if ww.buffer.DeleteChar(lineNum, col-1) {
			return lineNum, col - 1, true
		}
		return lineNum, col, false
	}

	// At start of line - join with previous line
	if lineNum > 1 {
		prevLine := ww.buffer.GetLine(lineNum - 1)
		prevLen := len(prevLine)

		if ww.buffer.JoinLines(lineNum - 1) {
			return lineNum - 1, prevLen + 1, true
		}
	}

	return lineNum, col, false
}

// HandleDelete handles delete at the given position
// May trigger line joining if at end of line
// Returns whether delete was successful
func (ww *WordWrapper) HandleDelete(lineNum, col int) bool {
	lineLen := ww.buffer.GetLineLength(lineNum)

	if col <= lineLen {
		// Delete character at cursor
		return ww.buffer.DeleteChar(lineNum, col)
	}

	// At end of line - join with next line
	if lineNum < ww.buffer.GetLineCount() {
		return ww.buffer.JoinLines(lineNum)
	}

	return false
}

// DeleteWord deletes the word to the right of the cursor
func (ww *WordWrapper) DeleteWord(lineNum, col int) bool {
	line := ww.buffer.GetLine(lineNum)
	if col > len(line) {
		return false
	}

	// Find the end of the current word (or whitespace)
	i := col - 1
	if i >= len(line) {
		return false
	}

	// Skip initial whitespace
	for i < len(line) && unicode.IsSpace(rune(line[i])) {
		i++
	}

	// Skip word characters
	for i < len(line) && !unicode.IsSpace(rune(line[i])) {
		i++
	}

	// Delete from cursor to end of word
	if i > col-1 {
		newLine := line[:col-1] + line[i:]
		ww.buffer.SetLine(lineNum, newLine)
		return true
	}

	return false
}

// FindWordLeft finds the start of the word to the left
func (ww *WordWrapper) FindWordLeft(lineNum, col int) int {
	line := ww.buffer.GetLine(lineNum)
	if col <= 1 {
		return 1
	}

	pos := col - 2 // Start before cursor

	// Skip trailing whitespace
	for pos >= 0 && pos < len(line) && unicode.IsSpace(rune(line[pos])) {
		pos--
	}

	// Skip word characters
	for pos >= 0 && pos < len(line) && !unicode.IsSpace(rune(line[pos])) {
		pos--
	}

	return pos + 2 // Convert to 1-based and move to start of word
}

// FindWordRight finds the start of the word to the right
func (ww *WordWrapper) FindWordRight(lineNum, col int) int {
	line := ww.buffer.GetLine(lineNum)
	if col > len(line) {
		return len(line) + 1
	}

	pos := col - 1 // Current position (0-based)

	// Skip current word
	for pos < len(line) && !unicode.IsSpace(rune(line[pos])) {
		pos++
	}

	// Skip whitespace
	for pos < len(line) && unicode.IsSpace(rune(line[pos])) {
		pos++
	}

	return pos + 1 // Convert to 1-based
}

// IsAtWordBoundary returns true if cursor is at a word boundary
func (ww *WordWrapper) IsAtWordBoundary(lineNum, col int) bool {
	line := ww.buffer.GetLine(lineNum)
	if col <= 1 || col > len(line) {
		return true
	}

	before := line[col-2]
	at := line[col-1]

	// Word boundary is transition from non-space to space or vice versa
	return unicode.IsSpace(rune(before)) != unicode.IsSpace(rune(at))
}
