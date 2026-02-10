package menu

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"
	"unicode"

	"github.com/robbiew/vision3/internal/ansi"
	"github.com/robbiew/vision3/internal/terminalio"
	"golang.org/x/term"
)

// MsgLightbarOption defines an option in the message reader lightbar.
type MsgLightbarOption struct {
	Label  string // Display text including padding, e.g. " Next "
	HotKey byte   // Single-char hotkey, e.g. 'N'
}

// readKeyWithEscapeHandling reads a single keypress, handling escape sequences
// for arrow keys. Returns the rune for normal keys, or special values for arrows:
//
//	arrowLeft = 0x1001, arrowRight = 0x1002
const (
	arrowLeft  rune = 0x1001
	arrowRight rune = 0x1002
)

func readKeyWithEscapeHandling(reader *bufio.Reader) (rune, error) {
	r, _, err := reader.ReadRune()
	if err != nil {
		return 0, err
	}

	if r == 27 { // ESC
		// Try to read escape sequence with a short timeout by checking if data is available
		// We peek to see if there's more data (the '[' of an escape sequence)
		peekBuf, peekErr := reader.Peek(1)
		if peekErr != nil || len(peekBuf) == 0 {
			return 27, nil // Just an ESC key
		}
		if peekBuf[0] != '[' {
			return 27, nil
		}

		// Read the '[' and direction byte
		reader.ReadByte() // consume '['
		dirByte, dirErr := reader.ReadByte()
		if dirErr != nil {
			return 27, nil
		}

		switch dirByte {
		case 'D': // Left arrow
			return arrowLeft, nil
		case 'C': // Right arrow
			return arrowRight, nil
		default:
			// Unknown escape sequence, ignore
			return 0, nil
		}
	}

	return r, nil
}

// drawMsgLightbarStatic draws the lightbar menu without waiting for input.
// This is used to display the menu during screen redraws.
// selectedIdx can be -1 for no highlight, or 0-9 to highlight a specific option.
func drawMsgLightbarStatic(terminal *term.Terminal, options []MsgLightbarOption,
	outputMode ansi.OutputMode, hiColor int, loColor int, suffix string, selectedIdx int) {

	// Move to beginning of line
	terminalio.WriteProcessedBytes(terminal, []byte("\r"), outputMode)

	for i, opt := range options {
		var colorSeq string
		if i == selectedIdx {
			colorSeq = colorCodeToAnsi(hiColor)
		} else {
			colorSeq = colorCodeToAnsi(loColor)
		}
		terminalio.WriteProcessedBytes(terminal, []byte(colorSeq+opt.Label), outputMode)
	}
	// Reset and write suffix
	terminalio.WriteProcessedBytes(terminal, []byte("\x1b[0m"+suffix), outputMode)
}

// runMsgLightbar displays a horizontal lightbar and returns the selected hotkey.
// It handles arrow key navigation and direct hotkey presses.
// The bar is drawn on a single line starting at the current cursor position.
//
// Pascal-style: options are displayed horizontally, the highlighted one is drawn
// with hiColor, others with loColor. Arrow keys move the highlight.
// Direct key presses (matching HotKey) select immediately.
// Enter selects the currently highlighted option.
//
// initialDirection: 0=none, -1=left (move to previous), 1=right (move to next)
func runMsgLightbar(reader *bufio.Reader, terminal *term.Terminal,
	options []MsgLightbarOption, outputMode ansi.OutputMode,
	hiColor int, loColor int, suffix string, initialDirection int) (byte, error) {

	if len(options) == 0 {
		return 0, fmt.Errorf("no lightbar options provided")
	}

	currentIdx := 0

	// Apply initial direction if provided
	if initialDirection < 0 {
		// Left arrow pressed initially - move to previous (wrap to end)
		currentIdx = len(options) - 1
	} else if initialDirection > 0 {
		// Right arrow pressed initially - move to next
		currentIdx = 1
		if currentIdx >= len(options) {
			currentIdx = 0
		}
	}

	// Build a map of hotkeys to indices for direct selection
	hotkeyMap := make(map[byte]int)
	for i, opt := range options {
		hotkeyMap[opt.HotKey] = i
	}

	// Pre-calculate column positions for each option
	// Options are laid out: [opt1][opt2][opt3]...
	cols := make([]int, len(options))
	col := 1 // Start at column 1
	for i, opt := range options {
		cols[i] = col
		col += len(opt.Label)
	}

	// Function to draw the bar
	drawBar := func(selectedIdx int) {
		drawMsgLightbarStatic(terminal, options, outputMode, hiColor, loColor, suffix, selectedIdx)
	}

	drawBar(currentIdx)

	for {
		key, err := readKeyWithEscapeHandling(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, io.EOF
			}
			return 0, fmt.Errorf("failed reading lightbar input: %w", err)
		}

		switch key {
		case arrowLeft:
			drawBar(-1) // Unhighlight current by drawing with no selection
			currentIdx--
			if currentIdx < 0 {
				currentIdx = len(options) - 1
			}
			drawBar(currentIdx)

		case arrowRight, ' ':
			drawBar(-1)
			currentIdx++
			if currentIdx >= len(options) {
				currentIdx = 0
			}
			drawBar(currentIdx)

		case '\r', '\n':
			// Select current option
			return options[currentIdx].HotKey, nil

		default:
			// Check for direct hotkey
			upperKey := byte(unicode.ToUpper(key))
			if idx, ok := hotkeyMap[upperKey]; ok {
				_ = idx
				return upperKey, nil
			}
			// Also handle numpad/arrow key mappings from Pascal (4=left, 6=right)
			if key == '4' {
				drawBar(-1)
				currentIdx--
				if currentIdx < 0 {
					currentIdx = len(options) - 1
				}
				drawBar(currentIdx)
			} else if key == '6' {
				drawBar(-1)
				currentIdx++
				if currentIdx >= len(options) {
					currentIdx = 0
				}
				drawBar(currentIdx)
			}
		}
	}
}

// readSingleKey reads a single keypress using the session's reader.
// It does NOT handle escape sequences - use readKeyWithEscapeHandling for that.
func readSingleKey(reader *bufio.Reader) (rune, error) {
	r, _, err := reader.ReadRune()
	return r, err
}

// readLineInput reads a line of text input, echoing characters.
// Returns the entered string (trimmed). Empty string on just Enter.
func readLineInput(reader *bufio.Reader, terminal *term.Terminal, outputMode ansi.OutputMode, maxLen int) (string, error) {
	var buf strings.Builder

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			return "", err
		}

		switch {
		case r == '\r' || r == '\n':
			return strings.TrimSpace(buf.String()), nil
		case r == 8 || r == 127: // Backspace or Delete
			if buf.Len() > 0 {
				s := buf.String()
				buf.Reset()
				buf.WriteString(s[:len(s)-1])
				terminalio.WriteProcessedBytes(terminal, []byte("\b \b"), outputMode)
			}
		case r >= 32 && r < 127:
			if maxLen <= 0 || buf.Len() < maxLen {
				buf.WriteRune(r)
				terminalio.WriteProcessedBytes(terminal, []byte(string(r)), outputMode)
			}
		}
	}
}

// promptSingleChar shows a prompt and waits for a single keypress.
// Returns the uppercase character pressed.
func promptSingleChar(reader *bufio.Reader, terminal *term.Terminal, prompt string, outputMode ansi.OutputMode) (rune, error) {
	processedPrompt := ansi.ReplacePipeCodes([]byte(prompt))
	wErr := terminalio.WriteProcessedBytes(terminal, processedPrompt, outputMode)
	if wErr != nil {
		log.Printf("ERROR: Failed writing prompt: %v", wErr)
	}

	r, err := readSingleKey(reader)
	if err != nil {
		return 0, err
	}

	return unicode.ToUpper(r), nil
}

// readKeySequence reads a key sequence, handling escape sequences for arrow keys, page up/down, etc.
// Returns the complete sequence as a string for switch handling.
func readKeySequence(reader *bufio.Reader) (string, error) {
	// Read first byte
	firstByte, err := reader.ReadByte()
	if err != nil {
		return "", err
	}

	// Check if it's an escape sequence
	if firstByte == 0x1B { // ESC
		// Try to read next byte(s) quickly - check if buffered
		time.Sleep(25 * time.Millisecond) // Small delay for escape sequence

		if reader.Buffered() > 0 {
			secondByte, err := reader.ReadByte()
			if err == nil && secondByte == '[' {
				// ANSI escape sequence - read until we get a letter or tilde
				seq := []byte{0x1B, '['}
				for {
					if reader.Buffered() == 0 {
						time.Sleep(10 * time.Millisecond)
					}
					if reader.Buffered() > 0 {
						b, err := reader.ReadByte()
						if err != nil {
							break
						}
						seq = append(seq, b)
						// End of sequence is typically a letter or tilde
						if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '~' {
							break
						}
						if len(seq) > 8 { // Safety limit
							break
						}
					} else {
						break
					}
				}
				return string(seq), nil
			}
			if err == nil {
				// ESC followed by something else
				return string([]byte{0x1B, secondByte}), nil
			}
		}
		// Just ESC by itself
		return "\x1b", nil
	}

	// Single character
	return string(firstByte), nil
}
