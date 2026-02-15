package ansi

import (
	"strings"
)

// VisibleLength returns the display width of a string, ignoring ANSI escape sequences.
// This counts only the characters that would be visible on screen.
func VisibleLength(s string) int {
	visCount := 0
	i := 0

	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip ANSI escape sequence
			i += 2
			for i < len(s) && !((s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z')) {
				i++
			}
			if i < len(s) {
				i++ // Skip terminator
			}
		} else {
			// Count visible character
			visCount++
			i++
		}
	}

	return visCount
}

// TruncateVisible truncates a string to maxVisible characters while preserving ANSI codes.
// ANSI escape sequences are kept intact and do not count toward the visible character limit.
func TruncateVisible(s string, maxVisible int) string {
	if maxVisible <= 0 {
		return ""
	}

	var result strings.Builder
	result.Grow(len(s))

	visCount := 0
	i := 0
	lastResetSeq := ""

	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// ANSI escape sequence - always preserve
			start := i
			i += 2
			for i < len(s) && !((s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z')) {
				i++
			}
			if i < len(s) {
				i++ // Include terminator
			}

			escSeq := s[start:i]
			result.WriteString(escSeq)

			// Track reset codes to append at end if truncated
			if strings.Contains(escSeq, "0m") || escSeq == "\x1b[m" {
				lastResetSeq = escSeq
			}
		} else {
			// Visible character
			if visCount < maxVisible {
				result.WriteByte(s[i])
				visCount++
				i++
			} else {
				// Reached limit - append reset if we had colors
				if lastResetSeq != "" && !strings.HasSuffix(result.String(), lastResetSeq) {
					result.WriteString(lastResetSeq)
				}
				break
			}
		}
	}

	return result.String()
}

// PadVisible pads a string to the specified width using the given pad character.
// ANSI escape sequences do not count toward the width.
func PadVisible(s string, width int, padChar rune) string {
	visLen := VisibleLength(s)
	if visLen >= width {
		return s
	}

	padding := strings.Repeat(string(padChar), width-visLen)
	return s + padding
}

// ApplyWidthConstraint truncates and/or pads a string to exact width.
// ANSI escape sequences are preserved during truncation.
func ApplyWidthConstraint(s string, width int) string {
	if width <= 0 {
		return s
	}

	s = TruncateVisible(s, width)
	s = PadVisible(s, width, ' ')
	return s
}
