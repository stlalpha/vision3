package terminalio

import (
	"io"
	"unicode/utf8"

	"github.com/stlalpha/vision3/internal/ansi"
)

// Helper to find the end of an ANSI sequence
// Returns the index *after* the terminating character
func findAnsiEnd(data []byte, start int) int {
	// Assumes data[start] == 0x1B
	if start+1 >= len(data) {
		return start + 1 // Incomplete sequence
	}

	seqEnd := start + 1
	switch data[seqEnd] {
	case '[': // CSI
		seqEnd++
		for seqEnd < len(data) {
			c := data[seqEnd]
			if c >= '@' && c <= '~' { // Check for standard CSI terminators
				seqEnd++ // Include the terminator
				return seqEnd
			}
			seqEnd++
			if seqEnd-start > 32 {
				return seqEnd
			} // Safety break
		}
		return seqEnd // Terminator not found
	case '(', ')': // Character set designation
		if start+2 < len(data) {
			return start + 3 // ESC ( B, ESC ) 0 etc.
		}
		return start + 2 // Incomplete
	default:
		// Other simple ESC sequences (like ESC M - Reverse Index)
		return start + 2 // Assume 2 bytes total: ESC + char
	}
}

// WriteProcessedBytes writes rawBytes to the writer.
// For raw CP437 data (ANS files), bytes pass through directly.
// For UTF-8 strings destined for CP437 terminals, callers should use
// WriteStringCP437 instead, which handles rune-to-byte conversion.
func WriteProcessedBytes(writer io.Writer, rawBytes []byte, mode ansi.OutputMode) error {
	_, err := writer.Write(rawBytes)
	return err
}

// WriteStringCP437 writes a UTF-8 string (e.g., from strings.json) to a
// CP437 terminal, converting multi-byte UTF-8 runes to their CP437 byte
// equivalents. ANSI escape sequences are passed through untouched.
// In non-CP437 modes, bytes are written as-is (UTF-8 passthrough).
func WriteStringCP437(writer io.Writer, data []byte, mode ansi.OutputMode) error {
	if mode != ansi.OutputModeCP437 {
		_, err := writer.Write(data)
		return err
	}

	// Fast path: if no multi-byte UTF-8, pass through directly
	hasMultibyte := false
	for _, b := range data {
		if b >= 0xC0 {
			hasMultibyte = true
			break
		}
	}
	if !hasMultibyte {
		_, err := writer.Write(data)
		return err
	}

	// Convert UTF-8 runes to CP437 bytes, preserving ANSI escapes
	out := make([]byte, 0, len(data))
	i := 0
	for i < len(data) {
		b := data[i]

		// Pass ANSI escape sequences through untouched
		if b == 0x1B {
			end := findAnsiEnd(data, i)
			out = append(out, data[i:end]...)
			i = end
			continue
		}

		// ASCII byte — pass through
		if b < 0x80 {
			out = append(out, b)
			i++
			continue
		}

		// Try to decode as multi-byte UTF-8 rune
		r, size := utf8.DecodeRune(data[i:])
		if r != utf8.RuneError || size > 1 {
			// Valid multi-byte UTF-8 rune — convert to CP437
			if cp437Byte, ok := ansi.UnicodeToCP437[r]; ok {
				out = append(out, cp437Byte)
			} else {
				out = append(out, '?')
			}
			i += size
			continue
		}

		// Invalid UTF-8 / single high byte — pass through as raw
		out = append(out, b)
		i++
	}

	_, err := writer.Write(out)
	return err
}

// WritePipeCodes directly writes pipe code processed output without CP437 conversion.
// Useful when the underlying writer handles encoding or when UTF-8 is desired.
func WritePipeCodes(w io.Writer, data []byte) error {
	processed := ansi.ReplacePipeCodes(data)
	_, err := w.Write(processed)
	return err
}
