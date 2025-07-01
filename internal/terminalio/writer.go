package terminalio

import (
	"io"

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

// WriteProcessedBytes writes rawBytes to the writer, handling ANSI escapes
// and character encoding based on the specified mode.
// In CP437 mode, input bytes >= 128 are passed through directly.
// In UTF8 mode, input bytes >= 128 are assumed to be CP437 and converted to UTF8 runes.
func WriteProcessedBytes(writer io.Writer, rawBytes []byte, mode ansi.OutputMode) error {
	// If mode is CP437, pass bytes directly as the terminal handles the encoding.
	// If mode is UTF8 or anything else, also pass directly for now.
	// The complex conversion logic was causing issues with raw CP437 bytes.
	// TODO: Revisit if conversion *from* UTF8 *to* CP437 is ever needed here.
	_, err := writer.Write(rawBytes)
	return err
}

// WritePipeCodes directly writes pipe code processed output without CP437 conversion.
// Useful when the underlying writer handles encoding or when UTF-8 is desired.
func WritePipeCodes(w io.Writer, data []byte) error {
	processed := ansi.ReplacePipeCodes(data)
	_, err := w.Write(processed)
	return err
}
