package terminalio

import (
	"io"
	"unicode/utf8"

	"github.com/stlalpha/vision3/internal/ansi"
)

func writeUTF8Mode(writer io.Writer, data []byte) error {
	out := make([]byte, 0, len(data))
	i := 0

	for i < len(data) {
		b := data[i]

		if b == 0x1B {
			end := findAnsiEnd(data, i)
			out = append(out, data[i:end]...)
			i = end
			continue
		}

		// Process a text span up to the next ANSI escape.
		spanStart := i
		for i < len(data) && data[i] != 0x1B {
			i++
		}
		span := data[spanStart:i]

		if utf8.Valid(span) {
			out = append(out, span...)
			continue
		}

		for _, sb := range span {
			if sb < 0x80 {
				out = append(out, sb)
				continue
			}
			mapped := ansi.Cp437ToUnicode[sb]
			if mapped == 0 {
				out = append(out, '?')
			} else {
				out = append(out, []byte(string(mapped))...)
			}
		}
	}

	_, err := writer.Write(out)
	return err
}

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

// WriteProcessedBytes writes bytes while honoring the configured output mode.
// CP437 mode converts UTF-8 runes to CP437 where possible and passes raw
// single bytes through unchanged. UTF-8 mode converts raw CP437 high bytes
// to UTF-8 when they are not valid UTF-8 sequences.
func WriteProcessedBytes(writer io.Writer, rawBytes []byte, mode ansi.OutputMode) error {
	switch mode {
	case ansi.OutputModeCP437:
		return WriteStringCP437(writer, rawBytes, mode)
	case ansi.OutputModeUTF8:
		return writeUTF8Mode(writer, rawBytes)
	default:
		_, err := writer.Write(rawBytes)
		return err
	}
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

	// Convert UTF-8 runes to CP437 bytes only for spans that are entirely valid UTF-8,
	// preserving ANSI escapes and passing raw CP437 bytes through unchanged.
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

		// Process a text span up to next ANSI escape.
		spanStart := i
		for i < len(data) && data[i] != 0x1B {
			i++
		}
		span := data[spanStart:i]

		if !utf8.Valid(span) {
			out = append(out, span...)
			continue
		}

		for _, r := range string(span) {
			if r < 0x80 {
				out = append(out, byte(r))
				continue
			}
			if cp437Byte, ok := ansi.UnicodeToCP437[r]; ok {
				out = append(out, cp437Byte)
			} else {
				out = append(out, '?')
			}
		}
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
