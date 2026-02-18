package editor

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/terminalio"
)

// Screen handles all screen rendering and ANSI control
type Screen struct {
	terminal         io.Writer
	outputMode       ansi.OutputMode
	termWidth        int
	termHeight       int
	editingStartY    int // First Y position for text entry
	statusLineY      int // Y position of status line
	screenLines      int // Number of editing lines available
	headerHeight     int // Height of header
	headerContent    string
	physicalLines    map[int]string // Track last rendered state for incremental updates
	lastInsertMode   bool           // Track last insert mode to avoid redundant updates
	lastCurrentLine  int            // Track last current line to avoid redundant updates
	lastStatusUpdate string         // Track last status to avoid redundant updates
	insertModeRow    int            // Terminal row of the @I@ insert-mode indicator (1-based)
	insertModeCol    int            // Terminal col of the @I@ insert-mode indicator (1-based)
	insertModeColor  string         // ANSI SGR escape to restore colors at @I@ position
	timeLoc          *time.Location // Timezone for date/time display
	configTimezone   string         // Raw timezone string from config
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

// LoadHeaderTemplate loads and processes the FSEDITOR.ANS template.
// fromName is the sender display name: handle, real name, or anonymous string.
func (s *Screen) LoadHeaderTemplate(menuSetPath, subject, recipient, fromName string, isAnon bool) error {
	templatePath := filepath.Join(menuSetPath, "ansi", "FSEDITOR.ANS")
	content, err := ansi.GetAnsiFileContent(templatePath)
	if err != nil {
		// If template doesn't exist, create a minimal header
		s.headerContent = s.createMinimalHeader(subject, recipient)
		return nil
	}

	// Process pipe codes and substitutions
	s.parseGeometryMarkers(string(content))
	processed := s.processPipeCodes(content, subject, recipient, fromName, isAnon)
	s.headerContent = string(processed)

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

// isEditorNewFormat reports whether the template uses @CODE@ placeholder syntax.
func isEditorNewFormat(content string) bool {
	for _, pat := range []string{"@S@", "@S#", "@S:", "@S|", "@F@", "@F#", "@F:", "@F|", "@E@", "@E#", "@E:", "@E|", "@T@", "@T#", "@T:", "@T|", "@I@", "@I#", "@I|"} {
		if strings.Contains(content, pat) {
			return true
		}
	}
	return false
}

// processPipeCodes processes substitution codes in the header template.
// It auto-detects the template format:
//   - New format: @CODE@ / @CODE####@ visual-width placeholders (preferred)
//   - Legacy format: |X pipe codes (backward compatibility)
func (s *Screen) processPipeCodes(content []byte, subject, recipient, fromName string, isAnon bool) []byte {
	str := string(content)

	loc := s.timeLoc
	if loc == nil {
		loc = config.LoadTimezone(s.configTimezone)
		s.timeLoc = loc
	}
	now := time.Now().In(loc)
	dateStr := now.Format("01/02/2006")
	timeStr := now.Format("3:04 PM")

	if recipient == "" {
		recipient = " "
	}
	if subject == "" {
		subject = " "
	}
	if fromName == "" {
		fromName = " "
	}

	if isEditorNewFormat(str) {
		// --- New @CODE@ format ---

		// Locate @I@ in the ORIGINAL raw content (before any substitution).
		// Visual cursor tracking works on raw CP437 bytes because:
		//  - ANSI escape sequences use ASCII only (unaffected by encoding)
		//  - Visual-width placeholders (@S####@) occupy the same byte count as their
		//    rendered field width, so the cursor column count is unchanged
		//  - CP437 high bytes each count as one column
		if row, col, colorEsc := ansi.FindEditorPlaceholderPos(content, 'I'); row > 0 {
			s.insertModeRow = row
			s.insertModeCol = col
			s.insertModeColor = colorEsc
		}

		// All substitutions in a single pass (including @I@ initial placeholder).
		subs := map[byte]string{
			'S': recipient,
			'F': fromName,
			'E': subject,
			'T': timeStr,
			'D': dateStr,
			'I': "   ", // initial value; updated dynamically by updateDynamicHeaderFields
		}

		// Always convert CP437→UTF-8 before placeholder substitution,
		// regardless of output mode. This prevents WriteStringCP437 from
		// misinterpreting pairs of raw CP437 high bytes (e.g. 0xC4 0xBF)
		// as valid UTF-8 multi-byte sequences, which would produce '?'
		// instead of the intended box-drawing characters (─┐).
		// WriteStringCP437 correctly round-trips UTF-8 → CP437 via the
		// UnicodeToCP437 reverse map.
		utf8Content := ansi.CP437BytesToUTF8(content)
		str = string(ansi.ProcessEditorPlaceholders(utf8Content, subs))
	} else {
		// --- Legacy |X format ---
		anonStr := "No "
		if isAnon {
			anonStr = "Yes"
		}
		replacements := map[string]string{
			"|S": recipient,
			"|E": subject,
			"|D": dateStr,
			"|T": timeStr,
			"|A": anonStr,
			"|I": "   ", // Insert mode indicator (updated dynamically)
			"|L": "   ", // Line number (updated dynamically)
		}
		for code, value := range replacements {
			str = strings.ReplaceAll(str, code, value)
		}
	}

	// Strip geometry markers (|#N, |=N) — these are processed by parseGeometryMarkers
	// and must not appear in the rendered output.
	str = s.removeControlMarkers(str)

	// Process any residual |XX pipe color codes (no-op for raw ANSI files).
	return ansi.ReplacePipeCodes([]byte(str))
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
		lineNum := 0
		for i := 0; i < len(rest); i++ {
			if rest[i] < '0' || rest[i] > '9' {
				break
			}
			lineNum = lineNum*10 + int(rest[i]-'0')
		}
		if lineNum > 0 && lineNum < s.termHeight {
			editingStartY := lineNum
			screenLines := s.termHeight - editingStartY - 1
			// Only apply the marker if it leaves at least 5 lines for editing
			if screenLines >= 5 {
				s.editingStartY = editingStartY
				s.screenLines = screenLines
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

// updateDynamicHeaderFields updates the insert-mode indicator in the header.
// The position is determined at template load time by tracking the @I@ placeholder.
// The ANSI color state captured from the original template is restored before
// writing so the indicator inherits the template's foreground/background colors.
func (s *Screen) updateDynamicHeaderFields(insertMode bool, currentLine, totalLines int) {
	if s.insertModeRow == 0 {
		return // position not known (template has no @I@ placeholder)
	}
	modeStr := "Ins"
	if !insertMode {
		modeStr = "Ovr"
	}
	// Save cursor, move to @I@ position, restore template color, write, restore cursor
	s.WriteDirect("\x1b[s") // save cursor position
	s.GoXY(s.insertModeCol, s.insertModeRow)
	if s.insertModeColor != "" {
		s.WriteDirect(s.insertModeColor) // restore template color at @I@ position
	}
	s.WriteDirect(fmt.Sprintf("%-3s", modeStr))
	s.WriteDirect("\x1b[u") // restore cursor position
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
func (s *Screen) RefreshScreen(buffer *MessageBuffer, topLine, currentLine, currentCol int, insertMode bool, forceStatusUpdate bool) {
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

	// Update status line only if values changed or forced
	if forceStatusUpdate || insertMode != s.lastInsertMode || currentLine != s.lastCurrentLine {
		s.DisplayStatusLine(insertMode, currentLine, lineCount)
		s.lastInsertMode = insertMode
		s.lastCurrentLine = currentLine
	}

	// Position cursor at editing position
	s.Reposition(currentLine, currentCol, topLine)
}

// ClearCache clears the screen cache to force a full redraw on next refresh
func (s *Screen) ClearCache() {
	s.physicalLines = make(map[int]string)
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

	// Refresh all lines (force status update on full redraw)
	s.RefreshScreen(buffer, topLine, currentLine, currentCol, insertMode, true)
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
