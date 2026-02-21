package stringeditor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// DOS CGA color palette mapped to ANSI 256-color indices.
// These are the standard 16 DOS colors (0-15).
var dosColors = [16]string{
	"0",  // 0:  Black
	"4",  // 1:  Blue (DOS blue = ANSI 4)
	"2",  // 2:  Green
	"6",  // 3:  Cyan (DOS cyan = ANSI 6)
	"1",  // 4:  Red (DOS red = ANSI 1)
	"5",  // 5:  Magenta
	"3",  // 6:  Brown/Yellow (DOS brown = ANSI 3)
	"7",  // 7:  Light Gray
	"8",  // 8:  Dark Gray
	"12", // 9:  Light Blue (DOS light blue = ANSI 12)
	"10", // 10: Light Green
	"14", // 11: Light Cyan (DOS light cyan = ANSI 14)
	"9",  // 12: Light Red (DOS light red = ANSI 9)
	"13", // 13: Light Magenta
	"11", // 14: Yellow (DOS yellow = ANSI 11)
	"15", // 15: White
}

// DOS CGA background colors mapped to ANSI 256-color indices.
var dosBgColors = [8]string{
	"0", // 0: Black BG
	"1", // 1: Red BG (DOS B1 = Red BG; maps to ANSI BG 1)
	"2", // 2: Green BG
	"3", // 3: Brown BG
	"4", // 4: Blue BG
	"5", // 5: Magenta BG
	"6", // 6: Cyan BG
	"7", // 7: Light Gray BG
}

// styledSpan represents a chunk of text with a specific style.
type styledSpan struct {
	text string
	fg   string // lipgloss color string (ANSI 256 index)
	bg   string // lipgloss background color string
}

// RenderColorString converts a BBS pipe-coded string into a lipgloss-styled
// string suitable for TUI display. It parses |XX foreground codes (00-15),
// |BX background codes (B0-B7), $x/$X dollar-sign color codes, and |CR
// (converted to space). The maxWidth parameter truncates visible characters
// (not counting color codes) and adds a "»" overflow indicator.
func RenderColorString(s string, maxWidth int) string {
	spans := parseColorCodes(s)
	return renderSpans(spans, maxWidth)
}

// parseColorCodes parses a BBS string with pipe/dollar color codes into spans.
func parseColorCodes(s string) []styledSpan {
	var spans []styledSpan
	curFG := dosColors[9] // Default: light blue (DOS color 9)
	curBG := ""           // Default: no background (terminal default)

	i := 0
	textBuf := strings.Builder{}

	flushText := func() {
		if textBuf.Len() > 0 {
			spans = append(spans, styledSpan{text: textBuf.String(), fg: curFG, bg: curBG})
			textBuf.Reset()
		}
	}

	for i < len(s) {
		// Check for pipe codes: |XX
		if s[i] == '|' && i+2 < len(s) {
			code := s[i+1 : i+3]

			// Background: |B0 - |B7
			if code[0] == 'B' && code[1] >= '0' && code[1] <= '7' {
				flushText()
				bgIdx := int(code[1] - '0')
				curBG = dosBgColors[bgIdx]
				i += 3
				continue
			}

			// Foreground: |00 - |15
			if code[0] >= '0' && code[0] <= '1' && code[1] >= '0' && code[1] <= '9' {
				num := int(code[0]-'0')*10 + int(code[1]-'0')
				if num >= 0 && num <= 15 {
					flushText()
					curFG = dosColors[num]
					i += 3
					continue
				}
			}

			// Special codes
			if code == "CR" {
				textBuf.WriteByte(' ')
				i += 3
				continue
			}
			if code == "CL" || code == "DE" {
				// Clear screen / clear to EOL — skip in TUI context
				i += 3
				continue
			}

			// |@ position codes — skip
			if code[0] == '@' {
				// Skip |@ followed by position data
				i += 3
				// May have additional position bytes; skip digits
				for i < len(s) && s[i] >= '0' && s[i] <= '9' {
					i++
				}
				continue
			}

			// Unrecognized pipe code — pass through as literal
			textBuf.WriteByte('|')
			i++
			continue
		}

		// Check for dollar-sign codes: $x
		if s[i] == '$' && i+1 < len(s) {
			ch := s[i+1]
			colorIdx := dollarColorIndex(ch)
			if colorIdx >= 0 {
				flushText()
				curFG = dosColors[colorIdx]
				i += 2
				continue
			}
			// Unrecognized dollar code — pass through
			textBuf.WriteByte('$')
			i++
			continue
		}

		// Regular character
		textBuf.WriteByte(s[i])
		i++
	}

	flushText()
	return spans
}

// dollarColorIndex maps a dollar-sign color code character to a DOS color index.
// Returns -1 for unrecognized characters.
// Matches the Pascal WriteColor() procedure's $x handling.
func dollarColorIndex(ch byte) int {
	switch ch {
	case 'a':
		return 0 // Black
	case 'b':
		return 1 // Blue
	case 'g':
		return 2 // Green
	case 'c':
		return 3 // Cyan
	case 'r':
		return 4 // Red
	case 'p':
		return 5 // Magenta
	case 'y':
		return 6 // Brown
	case 'w':
		return 7 // Light Gray
	case 'A':
		return 8 // Dark Gray
	case 'B':
		return 9 // Light Blue
	case 'G':
		return 10 // Light Green
	case 'C':
		return 11 // Light Cyan
	case 'R':
		return 12 // Light Red
	case 'P':
		return 13 // Light Magenta
	case 'Y':
		return 14 // Yellow
	case 'W':
		return 15 // White
	default:
		return -1
	}
}

// renderSpans converts styled spans to a lipgloss-rendered string,
// truncating to maxWidth visible characters.
func renderSpans(spans []styledSpan, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 80
	}

	var result strings.Builder
	visibleLen := 0

	for _, span := range spans {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(span.fg))
		if span.bg != "" {
			style = style.Background(lipgloss.Color(span.bg))
		}

		for _, ch := range span.text {
			if visibleLen >= maxWidth-1 {
				// Overflow indicator
				overflow := lipgloss.NewStyle().
					Foreground(lipgloss.Color(dosColors[15])).
					Background(lipgloss.Color(dosColors[5]))
				result.WriteString(overflow.Render("»"))
				return result.String()
			}
			result.WriteString(style.Render(string(ch)))
			visibleLen++
		}
	}

	return result.String()
}

// PlainTextLength returns the visible character count of a BBS pipe-coded string,
// stripping all color codes.
func PlainTextLength(s string) int {
	spans := parseColorCodes(s)
	total := 0
	for _, span := range spans {
		total += len([]rune(span.text))
	}
	return total
}

// DOSColorStyle returns a lipgloss style for a given DOS color attribute byte.
// The byte encodes fg (lower 4 bits) and bg (upper 4 bits), matching the
// Pascal TextAttr format: attr = bg*16 + fg.
func DOSColorStyle(attr byte) lipgloss.Style {
	fg := attr & 0x0F
	bg := (attr >> 4) & 0x07
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(dosColors[fg]))
	if bg > 0 {
		style = style.Background(lipgloss.Color(dosBgColors[bg]))
	}
	return style
}

// DOSFGStyle returns a lipgloss style for a DOS foreground color index (0-15).
func DOSFGStyle(colorIdx int) lipgloss.Style {
	if colorIdx < 0 || colorIdx > 15 {
		colorIdx = 7
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(dosColors[colorIdx]))
}

// FormatItemNumber formats an item number with consistent width for display.
func FormatItemNumber(n, total int) string {
	width := len(fmt.Sprintf("%d", total))
	return fmt.Sprintf("%*d", width, n)
}
