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

// ReflowRange reflows soft-wrapped lines starting from startLine.
// It scans forward through consecutive soft-wrapped lines (hardNewline=false)
// and stops at the first hard newline or end of buffer. Text is collected,
// re-wrapped to MaxLineLength, and placed back into the buffer.
// cursorLine/cursorCol track the cursor through the reflow using a linear
// character offset, which is more robust than the old col-wrapPos arithmetic.
// Returns the adjusted cursor position (line, col).
func (ww *WordWrapper) ReflowRange(startLine, cursorLine, cursorCol int) (int, int) {
	lineCount := ww.buffer.GetLineCount()
	if startLine < 1 || startLine > lineCount {
		return cursorLine, cursorCol
	}

	// Find paragraph bounds: scan forward until hardNewline=true or end of buffer
	endLine := startLine
	for endLine < lineCount && !ww.buffer.IsHardNewline(endLine) {
		endLine++
	}
	endHardNewline := ww.buffer.IsHardNewline(endLine)

	// Collect text from all lines in the paragraph and compute cursor offset
	var collected strings.Builder
	cursorOffset := 0
	cursorFound := false

	for i := startLine; i <= endLine; i++ {
		line := strings.TrimRight(ww.buffer.GetLine(i), " ")
		if i > startLine && collected.Len() > 0 && len(line) > 0 {
			collected.WriteByte(' ')
			if !cursorFound {
				cursorOffset++
			}
		}
		if i == cursorLine && !cursorFound {
			col := cursorCol - 1 // 0-based
			if col > len(line) {
				col = len(line)
			}
			cursorOffset += col
			cursorFound = true
		} else if !cursorFound {
			cursorOffset += len(line)
		}
		collected.WriteString(line)
	}

	text := collected.String()
	if cursorOffset > len(text) {
		cursorOffset = len(text)
	}

	// Re-wrap the collected text into lines of MaxLineLength.
	// Track start offsets in the collected text for cursor mapping.
	var outputLines []string
	var lineStarts []int
	pos := 0

	for pos < len(text) {
		lineStarts = append(lineStarts, pos)
		remaining := text[pos:]

		if len(remaining) <= MaxLineLength {
			outputLines = append(outputLines, remaining)
			break
		}

		wrapPos := ww.findWrapPosition(remaining)
		if wrapPos <= 0 {
			wrapPos = MaxLineLength
		}

		outputLines = append(outputLines, strings.TrimRight(remaining[:wrapPos], " "))

		// Advance past the wrap point and any spaces at the boundary
		pos += wrapPos
		for pos < len(text) && text[pos] == ' ' {
			pos++
		}
	}

	if len(outputLines) == 0 {
		outputLines = []string{""}
		lineStarts = []int{0}
	}

	// Place output lines back into the buffer
	originalCount := endLine - startLine + 1
	newCount := len(outputLines)

	// Set content for lines we can reuse
	commonCount := originalCount
	if newCount < commonCount {
		commonCount = newCount
	}
	for i := 0; i < commonCount; i++ {
		ww.buffer.SetLine(startLine+i, outputLines[i])
		ww.buffer.SetHardNewline(startLine+i, false)
	}

	if newCount > originalCount {
		// Need more lines — insert extras; track actual count written
		actualNew := originalCount
		for i := originalCount; i < newCount; i++ {
			if !ww.buffer.InsertLine(startLine + i) {
				// Buffer full: append remaining text to last written line
				last := startLine + actualNew - 1
				tail := text[lineStarts[i]:]
				if tail != "" {
					curr := ww.buffer.GetLine(last)
					if curr != "" {
						ww.buffer.SetLine(last, curr+" "+tail)
					} else {
						ww.buffer.SetLine(last, tail)
					}
				}
				break
			}
			ww.buffer.SetLine(startLine+i, outputLines[i])
			ww.buffer.SetHardNewline(startLine+i, false)
			actualNew++
		}
		// Truncate tracking to what was actually written
		if actualNew < newCount {
			newCount = actualNew
			outputLines = outputLines[:newCount]
			lineStarts = lineStarts[:newCount]
		}
	} else if newCount < originalCount {
		// Need fewer lines — delete extras
		for i := 0; i < originalCount-newCount; i++ {
			ww.buffer.DeleteLine(startLine + newCount)
		}
	}

	// Clamp newCount to buffer bounds
	bufLineCount := ww.buffer.GetLineCount()
	if startLine+newCount-1 > bufLineCount {
		newCount = bufLineCount - startLine + 1
	}

	// Last output line inherits the original paragraph-end hardNewline
	ww.buffer.SetHardNewline(startLine+newCount-1, endHardNewline)

	// Map cursor offset to output line/col using lineStarts.
	// Use actual buffer line lengths (not outputLines) since buffer-full
	// fallback may have appended overflow text to the last written line.
	if cursorOffset >= len(text) {
		lastLine := startLine + newCount - 1
		return lastLine, ww.buffer.GetLineLength(lastLine) + 1
	}

	for i := 0; i < len(lineStarts) && i < newCount; i++ {
		nextStart := len(text) + 1
		if i+1 < len(lineStarts) {
			nextStart = lineStarts[i+1]
		}
		if cursorOffset >= lineStarts[i] && cursorOffset < nextStart {
			localOffset := cursorOffset - lineStarts[i]
			actualLen := ww.buffer.GetLineLength(startLine + i)
			if localOffset > actualLen {
				localOffset = actualLen
			}
			return startLine + i, localOffset + 1
		}
	}

	// Fallback: cursor at end of last output line
	lastLine := startLine + newCount - 1
	return lastLine, ww.buffer.GetLineLength(lastLine) + 1
}

// WrapAfterInsert handles word wrap after a character insertion.
// Fast path: only reflows if the current line exceeds MaxLineLength.
func (ww *WordWrapper) WrapAfterInsert(lineNum, cursorCol int) (int, int) {
	line := ww.buffer.GetLine(lineNum)
	if len(line) <= MaxLineLength {
		return lineNum, cursorCol
	}
	return ww.ReflowRange(lineNum, lineNum, cursorCol)
}

// findWrapPosition finds the best position to wrap the line.
// Returns the index of the last space at or before MaxLineLength.
func (ww *WordWrapper) findWrapPosition(line string) int {
	for i := MaxLineLength; i > 0; i-- {
		if i < len(line) && line[i] == ' ' {
			return i
		}
	}
	return -1
}

// ReformatParagraph reformats a paragraph starting at the specified line.
// Uses ReflowRange to redistribute words across soft-wrapped lines.
// Stops at hard newlines or blank lines.
func (ww *WordWrapper) ReformatParagraph(startLine int) int {
	if ww.buffer.IsLineEmpty(startLine) {
		return startLine
	}

	// ReflowRange handles paragraph bounds via hardNewline flags
	newLine, _ := ww.ReflowRange(startLine, startLine, 1)

	// Find the last line of the reformatted paragraph
	endLine := newLine
	lineCount := ww.buffer.GetLineCount()
	for endLine < lineCount && !ww.buffer.IsHardNewline(endLine) && !ww.buffer.IsLineEmpty(endLine+1) {
		endLine++
	}
	return endLine
}

// HandleBackspace handles backspace at the given position.
// Deletes the character before the cursor and reflows the paragraph to pull
// words up from subsequent soft-wrapped lines. At start-of-line, joins with
// the previous line and reflows.
// Returns new cursor position (line, col) and whether a change was made.
func (ww *WordWrapper) HandleBackspace(lineNum, col int) (int, int, bool) {
	if col > 1 {
		if ww.buffer.DeleteChar(lineNum, col-1) {
			newLine, newCol := ww.ReflowRange(lineNum, lineNum, col-1)
			return newLine, newCol, true
		}
		return lineNum, col, false
	}

	// At start of line - join with previous line
	if lineNum > 1 {
		prevLine := ww.buffer.GetLine(lineNum - 1)
		prevLen := len(prevLine)

		if ww.buffer.JoinLines(lineNum - 1) {
			// Clear hardNewline after successful join so reflow flows across boundary
			ww.buffer.SetHardNewline(lineNum-1, false)
			newLine, newCol := ww.ReflowRange(lineNum-1, lineNum-1, prevLen+1)
			return newLine, newCol, true
		}
	}

	return lineNum, col, false
}

// HandleDelete handles delete at the given position.
// Deletes the character at the cursor and reflows, or joins with the next
// line if at end-of-line.
// Returns new cursor position (line, col) and whether a change was made.
func (ww *WordWrapper) HandleDelete(lineNum, col int) (int, int, bool) {
	lineLen := ww.buffer.GetLineLength(lineNum)

	if col <= lineLen {
		if ww.buffer.DeleteChar(lineNum, col) {
			newLine, newCol := ww.ReflowRange(lineNum, lineNum, col)
			return newLine, newCol, true
		}
		return lineNum, col, false
	}

	// At end of line - join with next line
	if lineNum < ww.buffer.GetLineCount() {
		if ww.buffer.JoinLines(lineNum) {
			// Clear hardNewline after successful join so reflow flows across boundary
			ww.buffer.SetHardNewline(lineNum, false)
			newLine, newCol := ww.ReflowRange(lineNum, lineNum, col)
			return newLine, newCol, true
		}
	}

	return lineNum, col, false
}

// DeleteWord deletes the word to the right of the cursor and reflows.
// Returns new cursor position (line, col) and whether a change was made.
func (ww *WordWrapper) DeleteWord(lineNum, col int) (int, int, bool) {
	line := ww.buffer.GetLine(lineNum)
	if col > len(line) {
		return lineNum, col, false
	}

	i := col - 1
	if i >= len(line) {
		return lineNum, col, false
	}

	// Skip initial whitespace
	for i < len(line) && unicode.IsSpace(rune(line[i])) {
		i++
	}
	// Skip word characters
	for i < len(line) && !unicode.IsSpace(rune(line[i])) {
		i++
	}

	if i > col-1 {
		newLine := line[:col-1] + line[i:]
		ww.buffer.SetLine(lineNum, newLine)
		newLn, newCol := ww.ReflowRange(lineNum, lineNum, col)
		return newLn, newCol, true
	}

	return lineNum, col, false
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

	return unicode.IsSpace(rune(before)) != unicode.IsSpace(rune(at))
}
