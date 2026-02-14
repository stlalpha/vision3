package editor

import (
	"bufio"
	"io"
	"net"
	"time"
)

// Special key codes for editor commands (using WordStar-style control characters)
const (
	// WordStar navigation commands
	KeyCtrlE = 0x05 // Up
	KeyCtrlX = 0x18 // Down
	KeyCtrlS = 0x13 // Left
	KeyCtrlD = 0x04 // Right
	KeyCtrlW = 0x17 // Home (start of line)
	KeyCtrlP = 0x10 // End (end of line)
	KeyCtrlR = 0x12 // Page Up
	KeyCtrlC = 0x03 // Page Down

	// Word navigation
	KeyCtrlA = 0x01 // Word Left
	KeyCtrlF = 0x06 // Word Right

	// Edit commands
	KeyCtrlV = 0x16 // Toggle Insert/Overwrite
	KeyCtrlG = 0x07 // Delete character at cursor
	KeyCtrlT = 0x14 // Delete word
	KeyCtrlY = 0x19 // Delete line
	KeyCtrlJ = 0x0A // Join lines (also Enter/LF in some contexts)
	KeyCtrlN = 0x0E // Split line (new line)
	KeyCtrlB = 0x02 // Reformat paragraph
	KeyCtrlL = 0x0C // Redisplay screen

	// Special keys
	KeyEsc       = 0x1B // Escape
	KeyEnter     = 0x0D // Carriage Return
	KeyBackspace = 0x08 // Backspace
	KeyTab       = 0x09 // Tab
	KeyDelete    = 0x7F // Delete (DEL character)

	// Special internal codes for arrow keys (outside normal byte range)
	KeyArrowUp    = 0x100 // Internal code for up arrow
	KeyArrowDown  = 0x101 // Internal code for down arrow
	KeyArrowLeft  = 0x102 // Internal code for left arrow
	KeyArrowRight = 0x103 // Internal code for right arrow
	KeyPageUp     = 0x104 // Internal code for page up
	KeyPageDown   = 0x105 // Internal code for page down
	KeyHome       = 0x106 // Internal code for home
	KeyEnd        = 0x107 // Internal code for end
	KeyInsert     = 0x108 // Internal code for insert
	KeyDeleteKey  = 0x109 // Internal code for delete key
)

// InputHandler handles keyboard input and escape sequence parsing
type InputHandler struct {
	reader         *bufio.Reader
	readDeadlineIO interface{ SetReadDeadline(time.Time) error }
	debug          bool
}

// NewInputHandler creates a new input handler
func NewInputHandler(input io.Reader) *InputHandler {
	var deadlineIO interface{ SetReadDeadline(time.Time) error }
	if conn, ok := input.(interface{ SetReadDeadline(time.Time) error }); ok {
		deadlineIO = conn
	}

	return &InputHandler{
		reader:         bufio.NewReader(input),
		readDeadlineIO: deadlineIO,
		debug:          false,
	}
}

// readByteWithTimeout reads a single byte with an optional timeout.
//
// NOTE: Known limitation - timeout may not work when data is buffered.
// If data exists in bufio.Reader, ReadByte() returns immediately regardless
// of deadline. Deadline only affects underlying Read() when buffer is empty.
// This is an acceptable trade-off for buffered I/O performance.
func (ih *InputHandler) readByteWithTimeout(timeout time.Duration) (byte, error) {
	if ih.readDeadlineIO != nil {
		if err := ih.readDeadlineIO.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return 0, err
		}
		defer ih.readDeadlineIO.SetReadDeadline(time.Time{})
	}

	return ih.reader.ReadByte()
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}

// ReadKey reads a single key, handling escape sequences
// Returns an integer code (may be > 255 for special keys)
func (ih *InputHandler) ReadKey() (int, error) {
	// Read first byte
	b, err := ih.reader.ReadByte()
	if err != nil {
		return 0, err
	}

	// Check for escape sequence
	if b == KeyEsc {
		// Peek ahead to see if this is an escape sequence
		peek, err := ih.reader.Peek(1)
		if err != nil || len(peek) == 0 {
			// Timeout or no data - treat as plain ESC
			return int(KeyEsc), nil
		}

		// Check for escape sequence start
		if peek[0] == '[' {
			// CSI sequence (ESC[)
			return ih.parseCSISequence()
		} else if peek[0] == 'O' {
			// SS3 sequence (ESC O) - used by some terminals for function keys
			return ih.parseSS3Sequence()
		}

		// Plain ESC
		return int(KeyEsc), nil
	}

	// Check for DEL character (0x7F) - map to delete
	if b == 0x7F {
		return int(KeyBackspace), nil // Treat DEL as backspace
	}

	// Normal character
	return int(b), nil
}

// parseCSISequence parses ANSI CSI escape sequences (ESC[...)
func (ih *InputHandler) parseCSISequence() (int, error) {
	// Read the '[' character
	_, err := ih.reader.ReadByte()
	if err != nil {
		return int(KeyEsc), err
	}

	// Read sequence bytes. CSI sequences arrive in a burst from the terminal,
	// so use a short inter-byte timeout where possible.
	sequence := make([]byte, 0, 10)

	for {
		b, err := ih.readByteWithTimeout(100 * time.Millisecond)
		if err != nil {
			if isTimeoutError(err) {
				break
			}
			break
		}

		sequence = append(sequence, b)

		// Check if this is the final byte (a letter)
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '~' {
			break
		}
	}

	// Parse the sequence
	if len(sequence) == 0 {
		return int(KeyEsc), nil
	}

	// Get the final character
	final := sequence[len(sequence)-1]

	// Handle common sequences
	switch final {
	case 'A': // Up arrow
		return KeyArrowUp, nil
	case 'B': // Down arrow
		return KeyArrowDown, nil
	case 'C': // Right arrow
		return KeyArrowRight, nil
	case 'D': // Left arrow
		return KeyArrowLeft, nil
	case 'H': // Home
		return KeyHome, nil
	case 'F': // End
		return KeyEnd, nil
	case '~':
		// Sequences ending with ~ (like 5~ for Page Up)
		if len(sequence) >= 2 {
			switch sequence[0] {
			case '1':
				return KeyHome, nil
			case '2':
				return KeyInsert, nil
			case '3':
				return KeyDeleteKey, nil
			case '4':
				return KeyEnd, nil
			case '5':
				return KeyPageUp, nil
			case '6':
				return KeyPageDown, nil
			}
		}
	}

	// Unknown sequence - return ESC
	return int(KeyEsc), nil
}

// parseSS3Sequence parses ANSI SS3 escape sequences (ESC O...)
func (ih *InputHandler) parseSS3Sequence() (int, error) {
	// Read the 'O' character
	_, err := ih.reader.ReadByte()
	if err != nil {
		return int(KeyEsc), err
	}

	// Read the next byte
	b, err := ih.reader.ReadByte()
	if err != nil {
		return int(KeyEsc), err
	}

	// Map SS3 sequences (used by some terminals for arrow keys)
	switch b {
	case 'A': // Up arrow
		return KeyArrowUp, nil
	case 'B': // Down arrow
		return KeyArrowDown, nil
	case 'C': // Right arrow
		return KeyArrowRight, nil
	case 'D': // Left arrow
		return KeyArrowLeft, nil
	case 'H': // Home
		return KeyHome, nil
	case 'F': // End
		return KeyEnd, nil
	}

	// Unknown sequence
	return int(KeyEsc), nil
}

// TranslateToWordStar translates arrow keys to WordStar equivalents
func TranslateToWordStar(key int) int {
	switch key {
	case KeyArrowUp:
		return KeyCtrlE
	case KeyArrowDown:
		return KeyCtrlX
	case KeyArrowLeft:
		return KeyCtrlS
	case KeyArrowRight:
		return KeyCtrlD
	case KeyHome:
		return KeyCtrlW
	case KeyEnd:
		return KeyCtrlP
	case KeyPageUp:
		return KeyCtrlR
	case KeyPageDown:
		return KeyCtrlC
	case KeyInsert:
		return KeyCtrlV
	case KeyDeleteKey:
		return KeyCtrlG
	default:
		return key
	}
}

// ReadKeyTranslated reads a key and translates arrow keys to WordStar commands
func (ih *InputHandler) ReadKeyTranslated() (int, error) {
	key, err := ih.ReadKey()
	if err != nil {
		return 0, err
	}
	return TranslateToWordStar(key), nil
}

// IsControlKey returns true if the key is a control character
func IsControlKey(key int) bool {
	return key < 32 || key == 127
}

// IsPrintable returns true if the key is a printable character
func IsPrintable(key int) bool {
	return key >= 32 && key < 127 && key != KeyEsc
}

// KeyName returns a human-readable name for a key code
func KeyName(key int) string {
	switch key {
	case KeyCtrlE:
		return "Ctrl+E (Up)"
	case KeyCtrlX:
		return "Ctrl+X (Down)"
	case KeyCtrlS:
		return "Ctrl+S (Left)"
	case KeyCtrlD:
		return "Ctrl+D (Right)"
	case KeyCtrlW:
		return "Ctrl+W (Home)"
	case KeyCtrlP:
		return "Ctrl+P (End)"
	case KeyCtrlR:
		return "Ctrl+R (Page Up)"
	case KeyCtrlC:
		return "Ctrl+C (Page Down)"
	case KeyCtrlA:
		return "Ctrl+A (Word Left)"
	case KeyCtrlF:
		return "Ctrl+F (Word Right)"
	case KeyCtrlV:
		return "Ctrl+V (Toggle Insert)"
	case KeyCtrlG:
		return "Ctrl+G (Delete)"
	case KeyCtrlT:
		return "Ctrl+T (Delete Word)"
	case KeyCtrlY:
		return "Ctrl+Y (Delete Line)"
	case KeyCtrlJ:
		return "Ctrl+J (Join Lines)"
	case KeyCtrlN:
		return "Ctrl+N (New Line)"
	case KeyCtrlB:
		return "Ctrl+B (Reformat)"
	case KeyCtrlL:
		return "Ctrl+L (Redraw)"
	case KeyEsc:
		return "Escape"
	case KeyEnter:
		return "Enter"
	case KeyBackspace:
		return "Backspace"
	case KeyTab:
		return "Tab"
	default:
		if key >= 32 && key < 127 {
			return string(rune(key))
		}
		return "Unknown"
	}
}
