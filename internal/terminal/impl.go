package terminal

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/gliderlabs/ssh"
)

// terminalImpl is the concrete implementation of the Terminal interface
type terminalImpl struct {
	// Core I/O
	session ssh.Session
	writer  io.Writer
	reader  io.Reader
	
	// Terminal state
	outputMode   OutputMode
	capabilities Capabilities
	
	// Components
	renderer *ArtRenderer
	charset  *CharsetHandler
	parser   *ANSIParser
	
	// PTY information
	ptyRequest *ssh.Pty
	hasPTY     bool
	
	// Window resize handling
	windowResizeCh <-chan ssh.Window
	
	// Synchronization
	mu sync.RWMutex
	
	// State tracking
	cursorPos Position
}

// newTerminalImpl creates a new terminal implementation from an SSH session
func newTerminalImpl(session ssh.Session, outputMode OutputMode) Terminal {
	t := &terminalImpl{
		session:    session,
		writer:     session,
		reader:     session,
		outputMode: outputMode,
	}
	
	// Get PTY information if available
	ptyReq, winCh, hasPTY := session.Pty()
	if hasPTY {
		t.ptyRequest = &ptyReq
		t.hasPTY = true
		t.windowResizeCh = winCh
		
		// Initialize capabilities from PTY info
		t.capabilities = Capabilities{
			Width:           ptyReq.Window.Width,
			Height:          ptyReq.Window.Height,
			Term:            ptyReq.Term,
			TerminalType:    DetectTerminalType(ptyReq.Term),
			SupportsResize:  true,
			SupportsColor:   true, // Assume color support for BBS usage
		}
		
		// Detect additional capabilities based on terminal type
		t.detectCapabilities()
	} else {
		// No PTY, use conservative defaults
		t.capabilities = Capabilities{
			Width:           80,
			Height:          25,
			Term:            "unknown",
			TerminalType:    TerminalUnknown,
			SupportsResize:  false,
			SupportsColor:   false,
			SupportsUTF8:    false,
		}
	}
	
	// Initialize components
	t.charset = NewCharsetHandler()
	t.parser = NewANSIParser(t.capabilities.Width, t.capabilities.Height)
	t.renderer = NewArtRenderer(t.writer, t.capabilities, outputMode)
	
	// Start window resize handler if PTY is available
	if t.hasPTY && t.windowResizeCh != nil {
		go t.handleWindowResize()
	}
	
	return t
}

// newTerminalFromWriter creates a terminal implementation from a generic io.Writer
func newTerminalFromWriter(writer io.Writer, outputMode OutputMode) Terminal {
	t := &terminalImpl{
		writer:     writer,
		outputMode: outputMode,
		capabilities: Capabilities{
			Width:        80,
			Height:       25,
			Term:         "generic",
			TerminalType: TerminalUnknown,
		},
	}
	
	// Initialize components
	t.charset = NewCharsetHandler()
	t.parser = NewANSIParser(t.capabilities.Width, t.capabilities.Height)
	t.renderer = NewArtRenderer(t.writer, t.capabilities, outputMode)
	
	return t
}

// detectCapabilities analyzes the terminal type and sets appropriate capabilities
func (t *terminalImpl) detectCapabilities() {
	termLower := strings.ToLower(t.capabilities.Term)
	
	switch t.capabilities.TerminalType {
	case TerminalSyncTERM:
		t.capabilities.SupportsUTF8 = false
		t.capabilities.SupportsLineDrawing = true
		t.capabilities.SupportsMouse = true
		t.capabilities.SupportsSyncTERM = true
		t.capabilities.MaxColors = 16
		
	case TerminalVT100:
		t.capabilities.SupportsUTF8 = false
		t.capabilities.SupportsLineDrawing = true
		t.capabilities.SupportsMouse = false
		t.capabilities.MaxColors = 8
		
	case TerminalXTerm:
		t.capabilities.SupportsUTF8 = true
		t.capabilities.SupportsLineDrawing = true
		t.capabilities.SupportsMouse = true
		if strings.Contains(termLower, "256color") {
			t.capabilities.MaxColors = 256
		} else {
			t.capabilities.MaxColors = 16
		}
		
	case TerminalUTF8:
		t.capabilities.SupportsUTF8 = true
		t.capabilities.SupportsLineDrawing = true
		t.capabilities.SupportsMouse = true
		t.capabilities.MaxColors = 256
		
	case TerminalANSI:
		t.capabilities.SupportsUTF8 = false
		t.capabilities.SupportsLineDrawing = true
		t.capabilities.SupportsMouse = false
		t.capabilities.MaxColors = 16
		
	case TerminalAmiga:
		t.capabilities.SupportsUTF8 = false
		t.capabilities.SupportsLineDrawing = false // Amiga has its own character set
		t.capabilities.SupportsMouse = false
		t.capabilities.SupportsAmiga = true
		t.capabilities.MaxColors = 16 // Amiga supports 16 colors
		t.capabilities.Font = "Topaz" // Default Amiga font
		
	default:
		// Conservative defaults for unknown terminals
		t.capabilities.SupportsUTF8 = strings.Contains(termLower, "utf") || strings.Contains(termLower, "unicode")
		t.capabilities.SupportsLineDrawing = false
		t.capabilities.SupportsMouse = false
		t.capabilities.MaxColors = 8
	}
}

// handleWindowResize monitors and handles window resize events
func (t *terminalImpl) handleWindowResize() {
	for window := range t.windowResizeCh {
		t.mu.Lock()
		oldWidth, oldHeight := t.capabilities.Width, t.capabilities.Height
		t.capabilities.Width = window.Width
		t.capabilities.Height = window.Height
		
		// Update parser dimensions
		if t.parser != nil {
			t.parser.SetDimensions(window.Width, window.Height)
		}
		t.mu.Unlock()
		
		// Log resize event (could be made configurable)
		if oldWidth != window.Width || oldHeight != window.Height {
			// Optionally notify about resize
		}
	}
}

// Core I/O operations

func (t *terminalImpl) Write(data []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Process the data through our rendering pipeline
	err := t.renderer.WriteProcessedBytes(data)
	if err != nil {
		return 0, err
	}
	
	return len(data), nil
}

func (t *terminalImpl) WriteString(s string) (int, error) {
	return t.Write([]byte(s))
}

func (t *terminalImpl) Read(data []byte) (int, error) {
	if t.reader == nil {
		return 0, fmt.Errorf("no reader available")
	}
	return t.reader.Read(data)
}

func (t *terminalImpl) ReadLine() (string, error) {
	if t.session != nil {
		// Create a line reader for the session
		reader := strings.Builder{}
		buffer := make([]byte, 1)
		
		for {
			n, err := t.session.Read(buffer)
			if err != nil {
				if err == io.EOF {
					break
				}
				return "", err
			}
			
			if n > 0 {
				char := buffer[0]
				
				// Handle special characters
				switch char {
				case '\r', '\n': // Enter key
					return reader.String(), nil
				case '\b', 127: // Backspace or DEL
					str := reader.String()
					if len(str) > 0 {
						// Remove last character and send backspace sequence
						str = str[:len(str)-1]
						reader.Reset()
						reader.WriteString(str)
						t.Write([]byte("\b \b")) // Backspace, space, backspace
					}
				case 3: // Ctrl+C
					return "", fmt.Errorf("interrupted")
				case 4: // Ctrl+D (EOF)
					return reader.String(), io.EOF
				default:
					if char >= 32 && char <= 126 { // Printable ASCII
						reader.WriteByte(char)
						t.Write([]byte{char}) // Echo the character
					}
				}
			}
		}
		
		return reader.String(), nil
	}
	
	return "", fmt.Errorf("no session available for ReadLine")
}

// Terminal control operations

func (t *terminalImpl) SetOutputMode(mode OutputMode) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.outputMode = mode
	if t.renderer != nil {
		// Recreate renderer with new output mode
		t.renderer = NewArtRenderer(t.writer, t.capabilities, mode)
	}
}

func (t *terminalImpl) GetOutputMode() OutputMode {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.outputMode
}

func (t *terminalImpl) GetCapabilities() Capabilities {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.capabilities
}

// ANSI art and content rendering

func (t *terminalImpl) RenderAnsiFile(filename string) error {
	return t.renderer.RenderAnsiFile(filename)
}

func (t *terminalImpl) RenderAnsiBytes(data []byte) error {
	return t.renderer.RenderAnsiBytes(data)
}

func (t *terminalImpl) ProcessPipeCodes(data []byte) []byte {
	return t.charset.ProcessPipeCodes(data)
}

func (t *terminalImpl) ProcessAnsiAndExtractCoords(rawContent []byte) (ProcessAnsiResult, error) {
	return t.renderer.ProcessAnsiAndExtractCoords(rawContent)
}

// Screen control operations

func (t *terminalImpl) ClearScreen() error {
	_, err := t.writer.Write([]byte("\x1b[2J\x1b[H"))
	return err
}

func (t *terminalImpl) MoveCursor(row, col int) error {
	cmd := fmt.Sprintf("\x1b[%d;%dH", row+1, col+1) // Convert to 1-based
	_, err := t.writer.Write([]byte(cmd))
	if err == nil {
		t.mu.Lock()
		t.cursorPos.X = col
		t.cursorPos.Y = row
		t.mu.Unlock()
	}
	return err
}

func (t *terminalImpl) SaveCursor() error {
	_, err := t.writer.Write([]byte("\x1b[s"))
	return err
}

func (t *terminalImpl) RestoreCursor() error {
	_, err := t.writer.Write([]byte("\x1b[u"))
	return err
}

func (t *terminalImpl) SetCursorVisible(visible bool) error {
	var cmd string
	if visible {
		cmd = "\x1b[?25h" // Show cursor
	} else {
		cmd = "\x1b[?25l" // Hide cursor
	}
	_, err := t.writer.Write([]byte(cmd))
	return err
}

// Content processing

func (t *terminalImpl) StripAnsi(content string) string {
	return t.charset.StripAnsi(content)
}

func (t *terminalImpl) GetAnsiFileContent(filename string) ([]byte, error) {
	// Load file and process through charset handler
	renderer := NewArtRenderer(io.Discard, t.capabilities, t.outputMode)
	data, err := renderer.loadAnsiFile(filename)
	if err != nil {
		return nil, err
	}
	
	// Process pipe codes
	return t.charset.ProcessPipeCodes(data), nil
}

// SAUCE metadata

func (t *terminalImpl) ParseSAUCE(data []byte) (*SAUCEInfo, []byte, error) {
	return t.renderer.ParseSAUCE(data)
}

// Terminal state

func (t *terminalImpl) GetDimensions() (width, height int) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.capabilities.Width, t.capabilities.Height
}

func (t *terminalImpl) SetDimensions(width, height int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.capabilities.Width = width
	t.capabilities.Height = height
	
	if t.parser != nil {
		t.parser.SetDimensions(width, height)
	}
}

func (t *terminalImpl) GetCursorPosition() Position {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.cursorPos
}

// Utility

func (t *terminalImpl) Close() error {
	// Clean up resources
	if t.session != nil {
		return t.session.Close()
	}
	return nil
}

// Legacy compatibility functions for existing ViSiON/3 code patterns
// These will be used during the migration process

// DisplayWithVT100LineDrawing provides VT100 line drawing display
func DisplayWithVT100LineDrawing(session ssh.Session, filename string) error {
	term := New(session, OutputModeAuto)
	renderer := term.(*terminalImpl).renderer
	renderer.SetRenderMode(RenderModeVT100)
	return term.RenderAnsiFile(filename)
}

// DisplayWithASCIIFallback provides ASCII fallback display
func DisplayWithASCIIFallback(session ssh.Session, filename string) error {
	term := New(session, OutputModeAuto)
	renderer := term.(*terminalImpl).renderer
	renderer.SetRenderMode(RenderModeASCII)
	return term.RenderAnsiFile(filename)
}

// DisplayWithStandardUTF8 provides UTF-8 display
func DisplayWithStandardUTF8(session ssh.Session, filename string) error {
	term := New(session, OutputModeUTF8)
	return term.RenderAnsiFile(filename)
}

// DisplayWithRawBytes provides raw byte display
func DisplayWithRawBytes(session ssh.Session, filename string) error {
	term := New(session, OutputModeCP437)
	return term.RenderAnsiFile(filename)
}

// ReplacePipeCodes processes pipe codes in data (legacy compatibility)
func ReplacePipeCodes(data []byte) []byte {
	charset := NewCharsetHandler()
	return charset.ProcessPipeCodes(data)
}

// ClearScreen returns ANSI clear screen sequence
func ClearScreen() string {
	return "\x1b[2J\x1b[H"
}

// MoveCursor returns ANSI cursor movement sequence
func MoveCursor(row, col int) string {
	return fmt.Sprintf("\x1b[%d;%dH", row+1, col+1)
}

// SaveCursor returns ANSI save cursor sequence
func SaveCursor() string {
	return "\x1b[s"
}

// RestoreCursor returns ANSI restore cursor sequence
func RestoreCursor() string {
	return "\x1b[u"
}

// StripAnsi removes ANSI sequences from string
func StripAnsi(str string) string {
	charset := NewCharsetHandler()
	return charset.StripAnsi(str)
}

// WriteProcessedBytes writes data through terminal processing
func WriteProcessedBytes(writer io.Writer, rawBytes []byte, mode OutputMode) error {
	term := NewFromWriter(writer, mode)
	_, err := term.Write(rawBytes)
	return err
}

// GetAnsiFileContent loads and processes an ANSI file
func GetAnsiFileContent(filename string) ([]byte, error) {
	term := NewFromWriter(io.Discard, OutputModeAuto)
	return term.GetAnsiFileContent(filename)
}

// ProcessAnsiAndExtractCoords processes ANSI and extracts coordinates
func ProcessAnsiAndExtractCoords(rawContent []byte, outputMode OutputMode) (ProcessAnsiResult, error) {
	term := NewFromWriter(io.Discard, outputMode)
	return term.ProcessAnsiAndExtractCoords(rawContent)
}