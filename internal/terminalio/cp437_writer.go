package terminalio

import (
	"bytes"
	"io"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// ansiState tracks the parser state for ANSI escape sequences.
type ansiState int

const (
	ansiStateGround ansiState = iota // Normal text processing
	ansiStateEscape                  // Saw ESC (\x1b)
	ansiStateCSI                     // Saw ESC [ (Control Sequence Introducer)
)

// SelectiveCP437Writer selectively encodes printable text to CP437,
// while passing ANSI escape sequences through unmodified.
type SelectiveCP437Writer struct {
	w       io.Writer             // Underlying writer (e.g., ssh.Session)
	encoder transform.Transformer // CP437 encoder
	state   ansiState             // Current ANSI parsing state
	ansiBuf bytes.Buffer          // Buffer for accumulating ANSI sequence bytes
}

// NewSelectiveCP437Writer creates a new selective CP437 writer.
func NewSelectiveCP437Writer(w io.Writer) *SelectiveCP437Writer {
	return &SelectiveCP437Writer{
		w:       w,
		encoder: charmap.CodePage437.NewEncoder(),
		state:   ansiStateGround,
	}
}

// Write implements the io.Writer interface.
// It parses input bytes, encodes text to CP437, and passes ANSI sequences.
func (sw *SelectiveCP437Writer) Write(p []byte) (n int, err error) {
	var processedBytes int
	var textChunk bytes.Buffer // Buffer for text chunks to be encoded

	flushTextChunk := func() error {
		if textChunk.Len() > 0 {
			// Encode the accumulated text chunk to CP437
			encodedBytes, _, encErr := transform.Bytes(sw.encoder, textChunk.Bytes())
			textChunk.Reset() // Clear the buffer regardless of error
			if encErr != nil {
				// Don't stop processing, but log or handle error? Maybe write placeholders?
				// For now, just write what we could encode (might be empty or partial)
				// return fmt.Errorf("cp437 encoding error: %w", encErr)
			}
			if len(encodedBytes) > 0 {
				_, writeErr := sw.w.Write(encodedBytes)
				if writeErr != nil {
					return writeErr // Return underlying write error immediately
				}
			}
		}
		return nil
	}

	flushAnsiBuf := func() error {
		if sw.ansiBuf.Len() > 0 {
			_, writeErr := sw.w.Write(sw.ansiBuf.Bytes())
			sw.ansiBuf.Reset()
			if writeErr != nil {
				return writeErr
			}
		}
		return nil
	}

	for i := 0; i < len(p); i++ {
		b := p[i]

		switch sw.state {
		case ansiStateGround:
			if b == 0x1b { // ESC - Start of potential ANSI sequence
				// Flush any pending text before handling escape
				if err := flushTextChunk(); err != nil {
					return processedBytes, err
				}
				sw.ansiBuf.WriteByte(b)
				sw.state = ansiStateEscape
			} else {
				// Append to text chunk - handle multi-byte UTF-8 potentially later
				// For simplicity now, assume valid UTF-8 coming from bubbletea
				// and buffer it. Check if it's a valid start byte if needed.
				textChunk.WriteByte(b)
			}

		case ansiStateEscape:
			sw.ansiBuf.WriteByte(b)
			if b == '[' { // CSI (Control Sequence Introducer)
				sw.state = ansiStateCSI
			} else {
				// Not a CSI sequence (e.g., could be other ESC codes)
				// For simplicity, assume any non-CSI ESC sequence is short
				// and flush it immediately.
				if err := flushAnsiBuf(); err != nil {
					return processedBytes, err
				}
				sw.state = ansiStateGround
			}

		case ansiStateCSI:
			sw.ansiBuf.WriteByte(b)
			// Check if the byte terminates the CSI sequence (A-Z, a-z)
			if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
				// End of sequence found, flush the ANSI buffer
				if err := flushAnsiBuf(); err != nil {
					return processedBytes, err
				}
				sw.state = ansiStateGround
			}
			// Continue buffering if it's part of the sequence (digits, ';', etc.)
		}
		processedBytes++
	}

	// After loop, flush any remaining buffered text or partial ANSI sequence
	if err := flushTextChunk(); err != nil {
		return processedBytes, err
	}
	if err := flushAnsiBuf(); err != nil { // Flush potential incomplete ANSI seq
		return processedBytes, err
	}

	// Check UTF-8 validity before returning? Optional.
	if !utf8.Valid(p[:processedBytes]) {
		// Maybe log a warning? Doesn't necessarily mean CP437 failed.
	}

	return processedBytes, nil
}
