package editor

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/robbiew/vision3/internal/ansi"
	"github.com/robbiew/vision3/internal/terminalio"
)

// Screen handles all screen rendering and ANSI control
type Screen struct {
	terminal      io.Writer
	outputMode    ansi.OutputMode
	termWidth     int
	termHeight    int
	editingStartY int // First Y position for text entry
	statusLineY   int // Y position of status line
	screenLines   int // Number of editing lines available
	headerHeight  int // Height of header
	headerContent string
	physicalLines map[int]string // Track last rendered state for incremental updates
}

// NewScreen creates a new screen manager
func NewScreen(terminal io.Writer, outputMode ansi.OutputMode, termWidth, termHeight int) *Screen {
	s := &Screen{
		terminal:      terminal,
		outputMode:    outputMode,
		termWidth:     termWidth,
		termHeight:    termHeight,
		physicalLines: make(map[int]string),
	}
	s.calculateGeometry()
	return s
}

// calculateGeometry calculates screen layout based on terminal dimensions
func (s *Screen) calculateGeometry() {
	// Default layout: header takes lines 1-6, status line is last line
	s.headerHeight = 6
	s.editingStartY = s.headerHeight + 1
	s.statusLineY = s.termHeight
	s.screenLines = s.termHeight - s.headerHeight - 1

	// Ensure minimum screen lines
	if s.screenLines < 5 {
		s.screenLines = 5
		s.statusLineY = s.editingStartY + s.screenLines
	}
}

// LoadHeaderTemplate loads and processes the FSEDITOR.ANS template
func (s *Screen) LoadHeaderTemplate(menuSetPath, subject, recipient string, isAnon bool) error {
	templatePath := filepath.Join(menuSetPath, "ansi", "FSEDITOR.ANS")
	content, err := ansi.GetAnsiFileContent(templatePath)
	if err != nil {
		// If template doesn't exist, create a minimal header
		s.headerContent = s.createMinimalHeader(subject, recipient)
		return nil
	}

	// Process pipe codes and substitutions
	processed := s.processPipeCodes(content, subject, recipient, isAnon)
	s.headerContent = string(processed)

	// Parse geometry markers from template if present
	s.parseGeometryMarkers(string(processed))

	return nil
}

// createMinimalHeader creates a simple header when no template is available
func (s *Screen) createMinimalHeader(subject, recipient string) string {
	var header strings.Builder
	header.WriteString(ansi.ClearScreen())
	header.WriteString("|15Full Screen Message Editor\r\n")
	header.WriteString("|07To: |11")
	header.WriteString(recipient)
	header.WriteString("\r\n")
	header.WriteString("|07Subject: |11")
	header.WriteString(subject)
	header.WriteString("\r\n")
	header.WriteString("|07" + strings.Repeat("-", 79) + "\r\n")
	return header.String()
}

// processPipeCodes processes pipe codes in the header template
func (s *Screen) processPipeCodes(content []byte, subject, recipient string, isAnon bool) []byte {
	str := string(content)

	// Get current date/time
	now := time.Now()
	dateStr := now.Format("01/02/2006")
	timeStr := now.Format("3:04 PM")

	// Ensure values have content (use space if empty)
	if recipient == "" {
		recipient = " "
	}
	if subject == "" {
		subject = " "
	}

	// Perform substitutions
	replacements := map[string]string{
		"|S": recipient,
		"|E": subject,
		"|D": dateStr,
		"|T": timeStr,
		"|A": func() string {
			if isAnon {
				return "Yes"
			}
			return "No "
		}(),
		"|I": "   ", // Insert mode indicator (will be updated dynamically)
		"|L": "   ", // Line number (will be updated dynamically)
	}

	// Apply replacements - do this BEFORE processing color codes
	for code, value := range replacements {
		str = strings.ReplaceAll(str, code, value)
	}

	// Remove control markers (|# and |=) and everything between them
	// These are special markers for screen geometry and should not be displayed
	str = s.removeControlMarkers(str)

	// Process standard pipe codes for colors
	processedBytes := ansi.ReplacePipeCodes([]byte(str))

	return processedBytes
}

// removeControlMarkers removes control markers like |#8 and |=15; from the template
func (s *Screen) removeControlMarkers(content string) string {
	// Remove |# markers (e.g., |#8)
	for {
		idx := strings.Index(content, "|#")
		if idx == -1 {
			break
		}
		// Find the end of the control sequence (look for next non-digit/non-punctuation)
		endIdx := idx + 2
		for endIdx < len(content) && (content[endIdx] >= '0' && content[endIdx] <= '9' || content[endIdx] == ';') {
			endIdx++
		}
		content = content[:idx] + content[endIdx:]
	}

	// Remove |= markers (e.g., |=15;)
	for {
		idx := strings.Index(content, "|=")
		if idx == -1 {
			break
		}
		// Find the end of the control sequence
		endIdx := idx + 2
		for endIdx < len(content) && (content[endIdx] >= '0' && content[endIdx] <= '9' || content[endIdx] == ';') {
			endIdx++
		}
		content = content[:idx] + content[endIdx:]
	}

	return content
}

// parseGeometryMarkers parses special markers in the template for screen geometry
func (s *Screen) parseGeometryMarkers(content string) {
	// Look for |# marker which may contain geometry information
	if idx := strings.Index(content, "|#"); idx != -1 {
		// Extract number after |#
		rest := content[idx+2:]
		if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
			lineNum := int(rest[0] - '0')
			if lineNum > 0 && lineNum < 20 {
				s.editingStartY = lineNum
				s.screenLines = s.termHeight - s.editingStartY - 1
			}
		}
	}
}

// DisplayHeader displays the header template
func (s *Screen) DisplayHeader() {
	terminalio.WriteProcessedBytes(s.terminal, []byte(s.headerContent), s.outputMode)
}

// GoXY moves the cursor to the specified position (1-based)
func (s *Screen) GoXY(x, y int) {
	terminalio.WriteProcessedBytes(s.terminal, []byte(fmt.Sprintf("\x1b[%d;%dH", y, x)), s.outputMode)
}

// ClearScreen clears the entire screen and homes the cursor
func (s *Screen) ClearScreen() {
	terminalio.WriteProcessedBytes(s.terminal, []byte(ansi.ClearScreen()), s.outputMode)
}

// ClearEOL clears from cursor to end of line
func (s *Screen) ClearEOL() {
	terminalio.WriteProcessedBytes(s.terminal, []byte("\x1b[K"), s.outputMode)
}

// DisplayStatusLine displays the status line at the bottom
func (s *Screen) DisplayStatusLine(insertMode bool, currentLine, totalLines int) {
	// Update dynamic header fields instead of status line
	s.updateDynamicHeaderFields(insertMode, currentLine, totalLines)
}

// updateDynamicHeaderFields updates the Ins and Line values in the header
func (s *Screen) updateDynamicHeaderFields(insertMode bool, currentLine, totalLines int) {
	// Update Insert mode indicator (typically at line 3, col 59 based on FSEDITOR.ANS)
	modeStr := "Ins"
	if !insertMode {
		modeStr = "Ovr"
	}
	// Pad to 3 characters to overwrite any previous content
	modeStr = fmt.Sprintf("%-3s", modeStr)
	s.GoXY(59, 3)
	s.WriteDirect(modeStr)

	// Update Line number (typically at line 4, col 59)
	lineStr := fmt.Sprintf("%-3d", currentLine) // Left-align, pad to 3 chars
	s.GoXY(59, 4)
	s.WriteDirect(lineStr)
}

// RefreshLine redraws a single line if it has changed
func (s *Screen) RefreshLine(lineNum int, lineContent string, topLine int) {
	// Calculate screen position
	screenLine := lineNum - topLine + 1
	if screenLine < 1 || screenLine > s.screenLines {
		return // Line not visible
	}

	// Check if line changed
	lastContent, exists := s.physicalLines[lineNum]
	if exists && lastContent == lineContent {
		return // No change
	}

	// Update physical state
	s.physicalLines[lineNum] = lineContent

	// Position cursor and draw line
	s.GoXY(1, s.editingStartY+screenLine-1)
	terminalio.WriteProcessedBytes(s.terminal, []byte(lineContent), s.outputMode)
	s.ClearEOL()
}

// RefreshScreen redraws all visible lines (incremental update)
func (s *Screen) RefreshScreen(buffer *MessageBuffer, topLine, currentLine, currentCol int, insertMode bool) {
	lineCount := buffer.GetLineCount()

	// Draw visible lines
	for i := 0; i < s.screenLines; i++ {
		msgLine := topLine + i
		if msgLine <= lineCount {
			lineContent := buffer.GetLine(msgLine)
			s.RefreshLine(msgLine, lineContent, topLine)
		} else {
			// Clear empty lines below message content
			screenY := s.editingStartY + i
			if screenY < s.statusLineY {
				s.GoXY(1, screenY)
				s.ClearEOL()
			}
		}
	}

	// Update status line
	s.DisplayStatusLine(insertMode, currentLine, lineCount)

	// Position cursor at editing position
	s.Reposition(currentLine, currentCol, topLine)
}

// Reposition moves cursor to the current editing position
func (s *Screen) Reposition(currentLine, currentCol, topLine int) {
	screenLine := currentLine - topLine + 1
	if screenLine < 1 || screenLine > s.screenLines {
		return
	}

	screenY := s.editingStartY + screenLine - 1
	s.GoXY(currentCol, screenY)
}

// FullRedraw performs a complete screen redraw
func (s *Screen) FullRedraw(buffer *MessageBuffer, topLine, currentLine, currentCol int, insertMode bool) {
	// Clear physical state to force full redraw
	s.physicalLines = make(map[int]string)

	// Clear screen and display header
	s.ClearScreen()
	s.DisplayHeader()

	// Refresh all lines
	s.RefreshScreen(buffer, topLine, currentLine, currentCol, insertMode)
}

// UpdateDynamicFields updates dynamic fields in the status line (like Insert/Line indicators)
func (s *Screen) UpdateDynamicFields(insertMode bool, currentLine, totalLines int) {
	// For now, just update the status line
	s.DisplayStatusLine(insertMode, currentLine, totalLines)
}

// ScrollUp scrolls the display up by the specified number of lines
func (s *Screen) ScrollUp(lines int) {
	// For simplicity, we'll do a full refresh when scrolling
	// ANSI scroll sequences could be used for optimization
}

// ScrollDown scrolls the display down by the specified number of lines
func (s *Screen) ScrollDown(lines int) {
	// For simplicity, we'll do a full refresh when scrolling
}

// GetScreenLines returns the number of available editing lines
func (s *Screen) GetScreenLines() int {
	return s.screenLines
}

// GetEditingStartY returns the Y position where text editing begins
func (s *Screen) GetEditingStartY() int {
	return s.editingStartY
}

// Resize handles terminal resize events
func (s *Screen) Resize(newWidth, newHeight int) {
	s.termWidth = newWidth
	s.termHeight = newHeight
	s.calculateGeometry()
	// Clear physical state to force full redraw on next refresh
	s.physicalLines = make(map[int]string)
}

// WriteDirect writes directly to the terminal (for special messages)
func (s *Screen) WriteDirect(text string) {
	terminalio.WriteProcessedBytes(s.terminal, []byte(text), s.outputMode)
}

// WriteDirectProcessed writes directly to the terminal with pipe code processing
func (s *Screen) WriteDirectProcessed(text string) {
	processed := ansi.ReplacePipeCodes([]byte(text))
	terminalio.WriteProcessedBytes(s.terminal, processed, s.outputMode)
}
