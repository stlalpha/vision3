package telnetserver

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gliderlabs/ssh"
)

// Telnet protocol constants
const (
	IAC  byte = 255 // Interpret As Command
	DONT byte = 254
	DO   byte = 253
	WONT byte = 252
	WILL byte = 251
	SB   byte = 250 // Subnegotiation Begin
	SE   byte = 240 // Subnegotiation End

	OptEcho     byte = 1  // Echo option
	OptSGA      byte = 3  // Suppress Go Ahead
	OptTermType byte = 24 // Terminal Type (RFC 1091)
	OptNAWS     byte = 31 // Negotiate About Window Size
	OptLinemode byte = 34 // Linemode

	TermTypeIs   byte = 0 // IS sub-command: client sends its terminal type
	TermTypeSend byte = 1 // SEND sub-command: server requests terminal type
)

// telnetState tracks the IAC state machine
type telnetState int

const (
	stateData telnetState = iota
	stateIAC
	stateWill
	stateWont
	stateDo
	stateDont
	stateSB
	stateSBData
	stateSBIAC
)

// TelnetConn wraps a net.Conn with telnet protocol awareness.
// Read() strips IAC commands transparently; Write() escapes 0xFF bytes.
type TelnetConn struct {
	conn   net.Conn
	reader *bufio.Reader
	writeMu sync.Mutex // protects writes to conn

	width  int
	height int
	sizeMu sync.RWMutex // protects width/height

	winCh chan ssh.Window // sends window size changes to adapter

	// IAC state machine (persists across Read calls)
	state    telnetState
	sbOption byte   // option byte for current subnegotiation
	sbData   []byte // accumulated subnegotiation data

	closed int32 // atomic flag

	// TERM_TYPE negotiation (RFC 1091)
	termType     string
	termTypeMu   sync.RWMutex
	willTermType bool // true after client responds WILL TERM_TYPE

	// Read interrupt: when the channel is closed, a goroutine sets a
	// short read deadline on the conn to unblock any pending Read().
	readInterrupt <-chan struct{}
	riMu          sync.Mutex
}

// NewTelnetConn wraps an existing net.Conn with telnet protocol handling.
func NewTelnetConn(conn net.Conn) *TelnetConn {
	return &TelnetConn{
		conn:   conn,
		reader: bufio.NewReaderSize(conn, 256),
		width:  80,
		height: 25,
		winCh:  make(chan ssh.Window, 1),
		state:  stateData,
	}
}

// Negotiate sends telnet option negotiations and waits for client responses.
// Phase 1: sends DO NAWS + DO TERM_TYPE, drains responses (500ms).
// Phase 2: if client responded WILL TERM_TYPE, sends SB TERM_TYPE SEND and
// drains again (500ms) to collect the IS <string> subnegotiation.
func (tc *TelnetConn) Negotiate() error {
	// Send telnet option negotiations:
	// IAC WILL ECHO       - server will echo input
	// IAC WILL SGA        - suppress go-ahead
	// IAC DO SGA          - client should suppress go-ahead
	// IAC DONT LINEMODE   - disable line mode
	// IAC DO NAWS         - request window size
	// IAC DO TERM_TYPE    - request terminal type
	negotiations := []byte{
		IAC, WILL, OptEcho,
		IAC, WILL, OptSGA,
		IAC, DO, OptSGA,
		IAC, DONT, OptLinemode,
		IAC, DO, OptNAWS,
		IAC, DO, OptTermType,
	}

	tc.writeMu.Lock()
	_, err := tc.conn.Write(negotiations)
	tc.writeMu.Unlock()
	if err != nil {
		return fmt.Errorf("failed to send telnet negotiations: %w", err)
	}

	// Phase 1: wait for NAWS and WILL TERM_TYPE responses
	tc.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	tc.drainNegotiations()
	tc.conn.SetReadDeadline(time.Time{})

	// Phase 2: if client agreed to send terminal type, request it
	if tc.willTermType {
		termRequest := []byte{IAC, SB, OptTermType, TermTypeSend, IAC, SE}
		tc.writeMu.Lock()
		_, err = tc.conn.Write(termRequest)
		tc.writeMu.Unlock()
		if err != nil {
			return fmt.Errorf("failed to send TERM_TYPE request: %w", err)
		}

		// Wait for the IS <string> subnegotiation response
		tc.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		tc.drainNegotiations()
		tc.conn.SetReadDeadline(time.Time{})
	}

	return nil
}

// drainNegotiations reads and processes any pending telnet negotiation responses.
func (tc *TelnetConn) drainNegotiations() {
	buf := make([]byte, 64)
	for {
		n, err := tc.reader.Read(buf)
		if n > 0 {
			tc.processNegotiationBytes(buf[:n])
		}
		if err != nil {
			break // Timeout or error, done draining
		}
		// If there's more buffered data, keep reading
		if tc.reader.Buffered() == 0 {
			break
		}
	}
}

// processNegotiationBytes processes bytes during the initial negotiation drain.
func (tc *TelnetConn) processNegotiationBytes(data []byte) {
	for i := 0; i < len(data); i++ {
		b := data[i]
		switch tc.state {
		case stateData:
			if b == IAC {
				tc.state = stateIAC
			}
			// Ignore non-IAC data during negotiation drain

		case stateIAC:
			switch b {
			case IAC:
				tc.state = stateData // Escaped 0xFF
			case WILL, WONT, DO, DONT:
				if b == WILL {
					tc.state = stateWill
				} else if b == WONT {
					tc.state = stateWont
				} else if b == DO {
					tc.state = stateDo
				} else {
					tc.state = stateDont
				}
			case SB:
				tc.state = stateSB
			default:
				tc.state = stateData // Unknown command, consume
			}

		case stateWill, stateWont, stateDo, stateDont:
			// Consume the option byte
			log.Printf("DEBUG: Telnet negotiation: cmd=%d option=%d", tc.state, b)
			if tc.state == stateWill && b == OptTermType {
				tc.willTermType = true
			}
			tc.state = stateData

		case stateSB:
			tc.sbOption = b
			tc.sbData = tc.sbData[:0]
			tc.state = stateSBData

		case stateSBData:
			if b == IAC {
				tc.state = stateSBIAC
			} else if len(tc.sbData) < 256 {
				tc.sbData = append(tc.sbData, b)
			}
			// else: silently discard excess subnegotiation data

		case stateSBIAC:
			if b == SE {
				// End of subnegotiation
				tc.handleSubnegotiation()
				tc.state = stateData
			} else if b == IAC {
				// Escaped 0xFF in subnegotiation data
				tc.sbData = append(tc.sbData, IAC)
				tc.state = stateSBData
			} else {
				// Unexpected, treat as end of subnegotiation
				tc.state = stateData
			}
		}
	}
}

// handleSubnegotiation processes a completed subnegotiation.
func (tc *TelnetConn) handleSubnegotiation() {
	switch tc.sbOption {
	case OptNAWS:
		if len(tc.sbData) < 4 {
			return
		}
		width := int(tc.sbData[0])<<8 | int(tc.sbData[1])
		height := int(tc.sbData[2])<<8 | int(tc.sbData[3])

		log.Printf("INFO: Telnet NAWS: %dx%d", width, height)

		// Validate and cap dimensions
		if width <= 0 || height <= 0 || width > 255 || height > 255 {
			log.Printf("WARN: Telnet NAWS: invalid dimensions %dx%d, using defaults", width, height)
			width = 80
			height = 25
		}
		if width > 80 {
			width = 80
		}
		if height > 25 {
			height = 25
		}

		tc.sizeMu.Lock()
		tc.width = width
		tc.height = height
		tc.sizeMu.Unlock()

		// Non-blocking send of window size update
		select {
		case tc.winCh <- ssh.Window{Width: width, Height: height}:
		default:
		}

	case OptTermType:
		// sbData[0] is TermTypeIs (0); terminal type string follows
		if len(tc.sbData) >= 1 && tc.sbData[0] == TermTypeIs {
			t := strings.ToLower(strings.TrimSpace(string(tc.sbData[1:])))
			if t != "" {
				tc.termTypeMu.Lock()
				tc.termType = t
				tc.termTypeMu.Unlock()
				log.Printf("INFO: Telnet TERM_TYPE: %s", t)
			}
		}
	}
}

// TermType returns the terminal type string reported by the client via TERM_TYPE
// negotiation (RFC 1091). Returns "ansi" if no type was negotiated.
func (tc *TelnetConn) TermType() string {
	tc.termTypeMu.RLock()
	t := tc.termType
	tc.termTypeMu.RUnlock()
	if t == "" {
		return "ansi"
	}
	return t
}

// SetReadInterrupt sets a channel that, when closed, causes any blocked Read()
// to return io.EOF without consuming data. This works by setting a past read
// deadline on the underlying connection to unblock the blocked read syscall.
// Pass nil to clear the interrupt and reset the deadline.
func (tc *TelnetConn) SetReadInterrupt(ch <-chan struct{}) {
	tc.riMu.Lock()
	tc.readInterrupt = ch
	tc.riMu.Unlock()

	if ch == nil {
		// Clear any deadline set by a previous interrupt
		tc.conn.SetReadDeadline(time.Time{})
		return
	}

	// Watch for the interrupt and unblock the read when it fires
	go func(ch <-chan struct{}) {
		<-ch
		tc.riMu.Lock()
		stillCurrent := tc.readInterrupt == ch
		tc.riMu.Unlock()
		if stillCurrent {
			tc.conn.SetReadDeadline(time.Now())
		}
	}(ch)
}

// Read reads data from the telnet connection, stripping IAC commands transparently.
func (tc *TelnetConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	written := 0
	for written == 0 {
		// Read from underlying reader
		buf := make([]byte, len(p))
		n, err := tc.reader.Read(buf)

		// Process bytes through state machine
		for i := 0; i < n && written < len(p); i++ {
			b := buf[i]
			switch tc.state {
			case stateData:
				if b == IAC {
					tc.state = stateIAC
				} else {
					p[written] = b
					written++
				}

			case stateIAC:
				switch b {
				case IAC:
					// Escaped 0xFF - output as literal byte
					p[written] = 0xFF
					written++
					tc.state = stateData
				case WILL:
					tc.state = stateWill
				case WONT:
					tc.state = stateWont
				case DO:
					tc.state = stateDo
				case DONT:
					tc.state = stateDont
				case SB:
					tc.state = stateSB
				default:
					// Other IAC commands (BRK, IP, AYT, etc.) - consume
					tc.state = stateData
				}

			case stateWill, stateWont, stateDo, stateDont:
				// Consume the option byte and return to data state
				tc.state = stateData

			case stateSB:
				tc.sbOption = b
				tc.sbData = tc.sbData[:0]
				tc.state = stateSBData

			case stateSBData:
				if b == IAC {
					tc.state = stateSBIAC
				} else if len(tc.sbData) < 256 {
					tc.sbData = append(tc.sbData, b)
				}

			case stateSBIAC:
				if b == SE {
					tc.handleSubnegotiation()
					tc.state = stateData
				} else if b == IAC {
					if len(tc.sbData) < 256 {
						tc.sbData = append(tc.sbData, IAC)
					}
					tc.state = stateSBData
				} else {
					tc.state = stateData
				}
			}
		}

		if err != nil {
			// Check if this error was triggered by a read interrupt
			tc.riMu.Lock()
			interrupt := tc.readInterrupt
			tc.riMu.Unlock()
			if interrupt != nil {
				select {
				case <-interrupt:
					if written > 0 {
						return written, nil
					}
					return 0, io.EOF
				default:
				}
			}

			if written > 0 {
				return written, nil // Return data we have, error on next call
			}
			return 0, err
		}
	}

	return written, nil
}

// Write writes data to the telnet connection, escaping any 0xFF bytes as IAC IAC.
func (tc *TelnetConn) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	tc.writeMu.Lock()
	defer tc.writeMu.Unlock()

	// Fast path: no 0xFF bytes, write directly
	if !bytes.Contains(p, []byte{0xFF}) {
		return tc.conn.Write(p)
	}

	// Slow path: escape 0xFF bytes
	var escaped []byte
	for _, b := range p {
		if b == 0xFF {
			escaped = append(escaped, IAC, IAC)
		} else {
			escaped = append(escaped, b)
		}
	}

	n, err := tc.conn.Write(escaped)
	if err != nil {
		return 0, err
	}

	// Return the original byte count, not the escaped count
	if n >= len(escaped) {
		return len(p), nil
	}
	// Partial write - estimate original bytes written
	return len(p), nil
}

// Close closes the telnet connection.
func (tc *TelnetConn) Close() error {
	if atomic.CompareAndSwapInt32(&tc.closed, 0, 1) {
		close(tc.winCh)
		return tc.conn.Close()
	}
	return nil
}

// RemoteAddr returns the remote network address.
func (tc *TelnetConn) RemoteAddr() net.Addr {
	return tc.conn.RemoteAddr()
}

// LocalAddr returns the local network address.
func (tc *TelnetConn) LocalAddr() net.Addr {
	return tc.conn.LocalAddr()
}

// WindowSize returns the current terminal dimensions.
func (tc *TelnetConn) WindowSize() (width, height int) {
	tc.sizeMu.RLock()
	defer tc.sizeMu.RUnlock()
	return tc.width, tc.height
}

// DetectTerminalSize performs terminal size detection using ANSI cursor position
// reporting (CPR) as the primary method, with NAWS fallback and 80x25 defaults.
// CPR is the most reliable method because it detects the actual usable area,
// accounting for status bars (e.g., SyncTerm's status row steals one row from NAWS).
func (tc *TelnetConn) DetectTerminalSize() (width, height int, method string) {
	// Step 1: Try ANSI cursor position reporting (CPR)
	w, h, err := tc.detectViaCursorPositioning()
	if err == nil && w > 0 && h > 0 {
		tc.sizeMu.Lock()
		tc.width = w
		tc.height = h
		tc.sizeMu.Unlock()

		// Non-blocking send of updated window size
		select {
		case tc.winCh <- ssh.Window{Width: w, Height: h}:
		default:
		}

		log.Printf("INFO: Telnet terminal size detected via ANSI CPR: %dx%d", w, h)
		return w, h, "ANSI"
	}
	if err != nil {
		log.Printf("DEBUG: Telnet ANSI CPR detection failed: %v", err)
	}

	// Step 2: Fall back to NAWS values (already populated by Negotiate)
	tc.sizeMu.RLock()
	nawsW, nawsH := tc.width, tc.height
	tc.sizeMu.RUnlock()

	if nawsW > 0 && nawsH > 0 && nawsW <= 80 && nawsH <= 25 {
		log.Printf("INFO: Telnet terminal size via NAWS: %dx%d", nawsW, nawsH)
		return nawsW, nawsH, "NAWS"
	}

	// Step 3: Safe defaults
	tc.sizeMu.Lock()
	tc.width = 80
	tc.height = 25
	tc.sizeMu.Unlock()

	log.Printf("INFO: Telnet terminal size using defaults: 80x25")
	return 80, 25, "DEFAULT"
}

// detectViaCursorPositioning uses ANSI escape sequences to detect actual usable
// terminal size. Moves cursor to far bottom-right (clamped by terminal), queries
// position, and parses the response. This detects status bars in SyncTerm etc.
func (tc *TelnetConn) detectViaCursorPositioning() (width, height int, err error) {
	// Ensure cursor is restored even on panic
	defer func() {
		if r := recover(); r != nil {
			tc.writeMu.Lock()
			tc.conn.Write([]byte("\033[u"))
			tc.writeMu.Unlock()
		}
	}()

	// Send: save cursor, move to 999;999 (clamped to actual size), query position
	query := []byte("\033[s\033[999;999H\033[6n")
	tc.writeMu.Lock()
	_, err = tc.conn.Write(query)
	tc.writeMu.Unlock()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to send CPR query: %w", err)
	}

	// Read response with timeout — collect bytes until we find ESC[row;colR
	var allData []byte
	deadline := time.Now().Add(3 * time.Second)
	cprPattern := regexp.MustCompile(`\033\[(\d+);(\d+)R`)

	for time.Now().Before(deadline) {
		if tc.reader.Buffered() > 0 {
			buf := make([]byte, tc.reader.Buffered())
			n, readErr := tc.reader.Read(buf)
			if readErr == nil && n > 0 {
				allData = append(allData, buf[:n]...)
			}
		} else {
			// Short read deadline to avoid blocking
			tc.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			buf := make([]byte, 32)
			n, readErr := tc.reader.Read(buf)
			tc.conn.SetReadDeadline(time.Time{})

			if n > 0 {
				allData = append(allData, buf[:n]...)
			}

			if readErr != nil {
				if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
					// Expected timeout, continue
				} else {
					break
				}
			}
		}

		// Check for complete CPR response
		if cprPattern.Match(allData) {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	// Restore cursor position
	tc.writeMu.Lock()
	tc.conn.Write([]byte("\033[u"))
	tc.writeMu.Unlock()

	// Process any telnet IAC commands that were in the raw response
	tc.processNegotiationBytes(allData)

	// Parse CPR response: ESC[row;colR
	if len(allData) == 0 {
		return 0, 0, fmt.Errorf("no CPR response received")
	}

	matches := cprPattern.FindStringSubmatch(string(allData))
	if len(matches) != 3 {
		return 0, 0, fmt.Errorf("no valid CPR response found in %d bytes", len(allData))
	}

	var rows, cols int
	fmt.Sscanf(matches[1], "%d", &rows)
	fmt.Sscanf(matches[2], "%d", &cols)

	// Apply BBS-compatible limits
	if cols > 80 {
		cols = 80
	}
	if rows > 25 {
		rows = 25
	}
	if cols < 20 {
		cols = 80
	}
	if rows < 10 {
		rows = 25
	}

	return cols, rows, nil
}
