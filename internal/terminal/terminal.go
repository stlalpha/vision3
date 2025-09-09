package terminal

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/gliderlabs/ssh"
	"golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
)

// TerminalType represents different types of terminals
type TerminalType int

const (
	TerminalUnknown TerminalType = iota
	TerminalANSI                 // Traditional ANSI/BBS terminals
	TerminalVT100                // VT100/VT102 compatible
	TerminalXTerm                // Modern xterm-like terminals
	TerminalUTF8                 // UTF-8 capable terminals
	TerminalSyncTERM             // SyncTERM BBS client
)

// Capabilities represents what a terminal can do
type Capabilities struct {
	SupportsColor      bool // Can display colors
	SupportsUTF8       bool // Can display UTF-8 characters
	SupportsLineDrawing bool // Can display line drawing characters
	SupportsMouse      bool // Can handle mouse input
	SupportsResize     bool // Can handle window resize events
	MaxColors          int  // Maximum colors supported (0, 8, 16, 256, etc.)
	DefaultWidth       int  // Default width if not specified
	DefaultHeight      int  // Default height if not specified
}

// Terminal represents a unified terminal interface
type Terminal struct {
	// Connection details
	session    ssh.Session
	rawTerminal *term.Terminal
	writer     io.Writer
	reader     io.Reader
	
	// Terminal identification
	termType     string
	terminalType TerminalType
	capabilities Capabilities
	
	// Current state
	width      int
	height     int
	outputMode ansi.OutputMode
	
	// PTY information
	ptyRequest *ssh.Pty
	hasPTY     bool
	
	// Synchronization
	mutex sync.RWMutex
	
	// Window resize channel
	windowResizeCh <-chan ssh.Window
}

// NewTerminal creates a new unified terminal instance
func NewTerminal(session ssh.Session) *Terminal {
	t := &Terminal{
		session: session,
		writer:  session,
		reader:  session,
		outputMode: ansi.OutputModeAuto,
	}
	
	// Get PTY information if available
	ptyReq, winCh, hasPTY := session.Pty()
	if hasPTY {
		t.ptyRequest = &ptyReq
		t.hasPTY = true
		t.termType = ptyReq.Term
		t.width = ptyReq.Window.Width
		t.height = ptyReq.Window.Height
		t.windowResizeCh = winCh
		
		log.Printf("Terminal: PTY detected - Type: %s, Size: %dx%d", 
			t.termType, t.width, t.height)
	} else {
		// No PTY, use defaults
		t.termType = "unknown"
		t.width = 80
		t.height = 25
		
		log.Printf("Terminal: No PTY detected, using defaults: %dx%d", 
			t.width, t.height)
	}
	
	// Detect terminal type and capabilities
	t.detectTerminalType()
	t.detectCapabilities()
	
	// Determine output mode
	t.determineOutputMode()
	
	// Create raw terminal for line-based input
	t.rawTerminal = term.NewTerminal(session, "")
	
	return t
}

// detectTerminalType analyzes the TERM environment variable to classify the terminal
func (t *Terminal) detectTerminalType() {
	termLower := strings.ToLower(t.termType)
	
	switch {
	case strings.Contains(termLower, "syncterm"):
		t.terminalType = TerminalSyncTERM
	case strings.HasPrefix(termLower, "xterm"):
		t.terminalType = TerminalXTerm
	case strings.Contains(termLower, "ansi"):
		t.terminalType = TerminalANSI
	case strings.HasPrefix(termLower, "vt100"), strings.HasPrefix(termLower, "vt102"):
		t.terminalType = TerminalVT100
	case strings.Contains(termLower, "256color"), strings.Contains(termLower, "color"):
		t.terminalType = TerminalUTF8
	default:
		t.terminalType = TerminalUnknown
	}
	
	log.Printf("Terminal: Detected type %d for TERM='%s'", t.terminalType, t.termType)
}

// detectCapabilities determines what the terminal can do based on its type
func (t *Terminal) detectCapabilities() {
	switch t.terminalType {
	case TerminalSyncTERM:
		t.capabilities = Capabilities{
			SupportsColor:      true,
			SupportsUTF8:       false, // SyncTERM typically uses CP437
			SupportsLineDrawing: true,
			SupportsMouse:      true,
			SupportsResize:     true,
			MaxColors:          16,
			DefaultWidth:       80,
			DefaultHeight:      25,
		}
	case TerminalXTerm:
		t.capabilities = Capabilities{
			SupportsColor:      true,
			SupportsUTF8:       true,
			SupportsLineDrawing: true,
			SupportsMouse:      true,
			SupportsResize:     true,
			MaxColors:          256,
			DefaultWidth:       80,
			DefaultHeight:       24,
		}
	case TerminalUTF8:
		t.capabilities = Capabilities{
			SupportsColor:      true,
			SupportsUTF8:       true,
			SupportsLineDrawing: true,
			SupportsMouse:      false,
			SupportsResize:     true,
			MaxColors:          256,
			DefaultWidth:       80,
			DefaultHeight:       24,
		}
	case TerminalANSI:
		t.capabilities = Capabilities{
			SupportsColor:      true,
			SupportsUTF8:       false,
			SupportsLineDrawing: true,
			SupportsMouse:      false,
			SupportsResize:     false,
			MaxColors:          16,
			DefaultWidth:       80,
			DefaultHeight:       25,
		}
	case TerminalVT100:
		t.capabilities = Capabilities{
			SupportsColor:      false,
			SupportsUTF8:       false,
			SupportsLineDrawing: true,
			SupportsMouse:      false,
			SupportsResize:     false,
			MaxColors:          0,
			DefaultWidth:       80,
			DefaultHeight:       24,
		}
	default:
		// Conservative defaults for unknown terminals
		t.capabilities = Capabilities{
			SupportsColor:      true,
			SupportsUTF8:       false,
			SupportsLineDrawing: false,
			SupportsMouse:      false,
			SupportsResize:     false,
			MaxColors:          8,
			DefaultWidth:       80,
			DefaultHeight:       24,
		}
	}
	
	log.Printf("Terminal: Capabilities - Color: %v, UTF8: %v, LineDrawing: %v, MaxColors: %d",
		t.capabilities.SupportsColor, t.capabilities.SupportsUTF8, 
		t.capabilities.SupportsLineDrawing, t.capabilities.MaxColors)
}

// determineOutputMode sets the appropriate output mode based on terminal capabilities
func (t *Terminal) determineOutputMode() {
	if t.capabilities.SupportsUTF8 {
		t.outputMode = ansi.OutputModeUTF8
	} else {
		t.outputMode = ansi.OutputModeCP437
	}
	
	log.Printf("Terminal: Output mode set to %d", t.outputMode)
}

// GetOutputMode returns the current output mode
func (t *Terminal) GetOutputMode() ansi.OutputMode {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.outputMode
}

// SetOutputMode manually sets the output mode
func (t *Terminal) SetOutputMode(mode ansi.OutputMode) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.outputMode = mode
	log.Printf("Terminal: Output mode manually set to %d", mode)
}

// GetDimensions returns the current terminal dimensions
func (t *Terminal) GetDimensions() (width, height int) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.width, t.height
}

// SetDimensions updates the terminal dimensions
func (t *Terminal) SetDimensions(width, height int) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.width = width
	t.height = height
	log.Printf("Terminal: Dimensions set to %dx%d", width, height)
}

// GetCapabilities returns the terminal capabilities
func (t *Terminal) GetCapabilities() Capabilities {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.capabilities
}

// GetTermType returns the terminal type string
func (t *Terminal) GetTermType() string {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.termType
}

// GetTerminalType returns the classified terminal type
func (t *Terminal) GetTerminalType() TerminalType {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.terminalType
}

// HasPTY returns whether this terminal has PTY support
func (t *Terminal) HasPTY() bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.hasPTY
}

// GetPTY returns the PTY information if available
func (t *Terminal) GetPTY() *ssh.Pty {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.ptyRequest
}

// GetRawTerminal returns the underlying raw terminal for compatibility
func (t *Terminal) GetRawTerminal() *term.Terminal {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.rawTerminal
}

// Write writes data to the terminal
func (t *Terminal) Write(data []byte) (int, error) {
	return t.writer.Write(data)
}

// Read reads data from the terminal  
func (t *Terminal) Read(data []byte) (int, error) {
	return t.reader.Read(data)
}

// ReadLine reads a line of input from the terminal
func (t *Terminal) ReadLine() (string, error) {
	if t.rawTerminal == nil {
		return "", fmt.Errorf("raw terminal not available")
	}
	return t.rawTerminal.ReadLine()
}

// StartWindowResizeHandler starts a goroutine to handle window resize events
func (t *Terminal) StartWindowResizeHandler() {
	if !t.hasPTY || t.windowResizeCh == nil {
		log.Printf("Terminal: No PTY or resize channel available")
		return
	}
	
	go func() {
		for window := range t.windowResizeCh {
			log.Printf("Terminal: Window resize event - %dx%d", window.Width, window.Height)
			t.SetDimensions(window.Width, window.Height)
			
			// Notify that the terminal was resized (could emit events here)
			// For now just log the change
		}
		log.Printf("Terminal: Window resize handler stopped")
	}()
}

// Close closes the terminal and cleans up resources
func (t *Terminal) Close() error {
	// Close the SSH session if we have access to it
	if t.session != nil {
		return t.session.Close()
	}
	return nil
}