package editor

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// ErrIdleTimeout is returned by ReadKeyWithTimeout when no input arrives before
// the caller-supplied deadline. It is distinct from the internal inter-byte
// escape-sequence timeout so that callers can handle user-visible idle
// disconnects without false-positive matches on sequence parsing.
var ErrIdleTimeout = errors.New("idle timeout")

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

	// Command shortcuts (shown in footer: CTRL (A)Abort (Z)Save (Q)Quote)
	KeyCtrlA = 0x01 // Abort (formerly Word Left)
	KeyCtrlZ = 0x1A // Save
	KeyCtrlQ = 0x11 // Quote

	// Word navigation
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

// InputHandler handles keyboard input and escape sequence parsing.
// A background goroutine continuously reads raw bytes from the underlying
// reader into a buffered channel. This makes select-based timeouts reliable
// regardless of whether the reader supports SetReadDeadline (e.g. ssh.Session).
type InputHandler struct {
	incoming  chan byte // raw bytes from background reader goroutine
	unreadBuf []byte   // bytes pushed back for re-reading
	debug     bool

	// idleNs is the session-level idle timeout in nanoseconds (0 = disabled).
	// Stored as int64 for lock-free atomic access. Any call to readByte()
	// that blocks longer than this fires ErrIdleTimeout. Set via
	// SetSessionIdleTimeout; read on every key-wait.
	idleNs int64 // atomic

	// Optional read interrupt integration for sessions that support it.
	readInterrupt    chan struct{}
	setReadInterrupt func(<-chan struct{})
	closeOnce        sync.Once
}

// SetSessionIdleTimeout sets the session-level idle timeout applied to every
// ReadKey call. Pass 0 to disable. Thread-safe.
func (ih *InputHandler) SetSessionIdleTimeout(d time.Duration) {
	atomic.StoreInt64(&ih.idleNs, d.Nanoseconds())
}

func (ih *InputHandler) sessionIdleTimeout() time.Duration {
	ns := atomic.LoadInt64(&ih.idleNs)
	if ns <= 0 {
		return 0
	}
	return time.Duration(ns)
}

// NewInputHandler creates a new input handler.
// A goroutine is started to read from input; it exits when input returns an error.
func NewInputHandler(input io.Reader) *InputHandler {
	ih := &InputHandler{
		incoming: make(chan byte, 256),
	}

	// If the reader supports read interruption (ssh/telnet adapters do),
	// wire one in so Close() can stop the background goroutine cleanly.
	if ri, ok := input.(interface{ SetReadInterrupt(<-chan struct{}) }); ok {
		ih.readInterrupt = make(chan struct{})
		ih.setReadInterrupt = ri.SetReadInterrupt
		ih.setReadInterrupt(ih.readInterrupt)
	}

	go func() {
		defer close(ih.incoming)
		if ih.setReadInterrupt != nil {
			defer ih.setReadInterrupt(nil)
		}

		buf := [1]byte{}
		for {
			n, err := input.Read(buf[:])
			if n > 0 {
				ih.incoming <- buf[0]
			}
			if err != nil {
				return
			}
		}
	}()
	return ih
}

// Close stops the background read loop for handlers backed by sessions that
// support SetReadInterrupt. It is safe to call multiple times.
func (ih *InputHandler) Close() {
	ih.closeOnce.Do(func() {
		if ih.readInterrupt != nil {
			close(ih.readInterrupt)
		}
	})
}

// Read implements io.Reader. It reads exactly one byte from the incoming channel,
// blocking until a byte is available or the channel is closed (EOF). This allows
// InputHandler to be wrapped by bufio.NewReader and shared between callers (e.g.
// menu loops and the full-screen editor) so that the background goroutine's bytes
// are not lost when the editor returns.
func (ih *InputHandler) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(ih.unreadBuf) > 0 {
		p[0] = ih.unreadBuf[0]
		ih.unreadBuf = ih.unreadBuf[1:]
		return 1, nil
	}
	b, ok := <-ih.incoming
	if !ok {
		return 0, io.EOF
	}
	p[0] = b
	return 1, nil
}

// readByte reads a single byte, blocking until one is available.
// If a session idle timeout is set (via SetSessionIdleTimeout) and no byte
// arrives within that window, ErrIdleTimeout is returned.
func (ih *InputHandler) readByte() (byte, error) {
	if len(ih.unreadBuf) > 0 {
		b := ih.unreadBuf[0]
		ih.unreadBuf = ih.unreadBuf[1:]
		return b, nil
	}
	if t := ih.sessionIdleTimeout(); t > 0 {
		b, err := ih.readByteWithTimeout(t)
		if err != nil {
			if isTimeoutError(err) {
				return 0, ErrIdleTimeout
			}
			return 0, err
		}
		return b, nil
	}
	b, ok := <-ih.incoming
	if !ok {
		return 0, io.EOF
	}
	return b, nil
}

// unreadByte pushes b back so it is returned by the next readByte call.
func (ih *InputHandler) unreadByte(b byte) {
	ih.unreadBuf = append([]byte{b}, ih.unreadBuf...)
}

// readByteWithTimeout reads a single byte, returning errTimeout if none
// arrives within the given duration.
func (ih *InputHandler) readByteWithTimeout(timeout time.Duration) (byte, error) {
	if len(ih.unreadBuf) > 0 {
		b := ih.unreadBuf[0]
		ih.unreadBuf = ih.unreadBuf[1:]
		return b, nil
	}
	select {
	case b, ok := <-ih.incoming:
		if !ok {
			return 0, io.EOF
		}
		return b, nil
	case <-time.After(timeout):
		return 0, errTimeout
	}
}

// errTimeout is the sentinel returned when readByteWithTimeout expires.
var errTimeout = &inputTimeoutError{}

type inputTimeoutError struct{}

func (e *inputTimeoutError) Error() string   { return "i/o timeout" }
func (e *inputTimeoutError) Timeout() bool   { return true }
func (e *inputTimeoutError) Temporary() bool { return true }

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if te, ok := err.(interface{ Timeout() bool }); ok {
		return te.Timeout()
	}
	return false
}

// ReadKey reads a single key, handling escape sequences.
// Returns an integer code (may be > 255 for special keys).
func (ih *InputHandler) ReadKey() (int, error) {
	b, err := ih.readByte()
	if err != nil {
		return 0, err
	}

	// Check for escape sequence.
	// A 50 ms timeout distinguishes a lone ESC keypress from sequences like
	// arrow keys (ESC [ A). Because reads go through a channel, this timeout
	// works reliably even if the reader does not support SetReadDeadline.
	if b == KeyEsc {
		next, err := ih.readByteWithTimeout(50 * time.Millisecond)
		if err != nil {
			// Timeout — no following byte, so this is a plain ESC.
			return int(KeyEsc), nil
		}
		switch next {
		case '[':
			// CSI sequence (ESC [) — '[' already consumed.
			return ih.parseCSISequence()
		case 'O':
			// SS3 sequence (ESC O) — 'O' already consumed.
			return ih.parseSS3Sequence()
		default:
			// Unexpected byte after ESC; push it back and return plain ESC.
			ih.unreadByte(next)
			return int(KeyEsc), nil
		}
	}

	// DEL (0x7F) → backspace
	if b == 0x7F {
		return int(KeyBackspace), nil
	}

	// CR (0x0D) → normalize CR+LF to plain CR.
	// SSH clients often send CR+LF for the Enter key. Discard the LF so that
	// callers (lightbars, menus) don't see a phantom keypress after Enter.
	if b == KeyEnter {
		if next, err := ih.readByteWithTimeout(10 * time.Millisecond); err == nil && next != 0x0A {
			ih.unreadByte(next)
		}
		return int(KeyEnter), nil
	}

	return int(b), nil
}

// ReadKeyWithTimeout is identical to ReadKey but waits at most idleTimeout for
// the very first byte. If no input arrives within that window it returns
// (0, ErrIdleTimeout). Inter-byte timeouts for escape-sequence parsing are
// unaffected. This is the extensible primitive for idle-disconnect logic.
func (ih *InputHandler) ReadKeyWithTimeout(idleTimeout time.Duration) (int, error) {
	// Wait for the first byte with the caller's deadline.
	first, err := ih.readByteWithTimeout(idleTimeout)
	if err != nil {
		if isTimeoutError(err) {
			return 0, ErrIdleTimeout
		}
		return 0, err
	}
	// Push it back so ReadKey sees a normal byte and handles escape sequences.
	ih.unreadByte(first)
	return ih.ReadKey()
}

// parseCSISequence parses ANSI CSI escape sequences (ESC [ ...).
// '[' has already been consumed by ReadKey before this is called.
func (ih *InputHandler) parseCSISequence() (int, error) {
	// CSI sequences arrive in a burst; use a short inter-byte timeout.
	sequence := make([]byte, 0, 10)
	for {
		b, err := ih.readByteWithTimeout(100 * time.Millisecond)
		if err != nil {
			break
		}
		sequence = append(sequence, b)
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '~' {
			break
		}
	}

	if len(sequence) == 0 {
		return int(KeyEsc), nil
	}

	final := sequence[len(sequence)-1]
	switch final {
	case 'A':
		return KeyArrowUp, nil
	case 'B':
		return KeyArrowDown, nil
	case 'C':
		return KeyArrowRight, nil
	case 'D':
		return KeyArrowLeft, nil
	case 'H':
		return KeyHome, nil
	case 'F':
		return KeyEnd, nil
	case '~':
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

	return int(KeyEsc), nil
}

// parseSS3Sequence parses ANSI SS3 escape sequences (ESC O ...).
// 'O' has already been consumed by ReadKey before this is called.
func (ih *InputHandler) parseSS3Sequence() (int, error) {
	b, err := ih.readByte()
	if err != nil {
		return int(KeyEsc), err
	}
	switch b {
	case 'A':
		return KeyArrowUp, nil
	case 'B':
		return KeyArrowDown, nil
	case 'C':
		return KeyArrowRight, nil
	case 'D':
		return KeyArrowLeft, nil
	case 'H':
		return KeyHome, nil
	case 'F':
		return KeyEnd, nil
	}
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
		return "Ctrl+A (Abort)"
	case KeyCtrlZ:
		return "Ctrl+Z (Save)"
	case KeyCtrlQ:
		return "Ctrl+Q (Quote)"
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
