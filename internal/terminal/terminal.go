package terminal

import (
	"io"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
)

// OutputMode specifies how terminal output should be processed
type OutputMode int

const (
	OutputModeAuto OutputMode = iota // Auto-detect based on terminal
	OutputModeUTF8                   // Force UTF-8 output
	OutputModeCP437                  // Force CP437 output for authentic BBS experience
)

// TerminalType represents different types of terminals we support
type TerminalType int

const (
	TerminalUnknown TerminalType = iota
	TerminalANSI                 // Traditional ANSI/BBS terminals
	TerminalVT100                // VT100/VT102 compatible
	TerminalXTerm                // Modern xterm-like terminals
	TerminalSyncTERM             // SyncTERM BBS client
	TerminalUTF8                 // UTF-8 capable modern terminals
	TerminalAmiga                // Amiga terminal/emulator
)

// Capabilities represents what a terminal can do
type Capabilities struct {
	SupportsColor       bool         // Can display colors
	SupportsUTF8        bool         // Can display UTF-8 characters
	SupportsLineDrawing bool         // Can display line drawing characters
	SupportsMouse       bool         // Can handle mouse input
	SupportsResize      bool         // Can handle window resize events
	SupportsSyncTERM    bool         // Supports SyncTERM extensions
	SupportsAmiga       bool         // Supports Amiga-specific features
	MaxColors           int          // Maximum colors supported (0, 8, 16, 256, etc.)
	TerminalType        TerminalType // Detected terminal type
	Width               int          // Current terminal width
	Height              int          // Current terminal height
	Term                string       // TERM environment variable value
	Font                string       // Font name for Amiga/SyncTERM terminals
}

// Position represents a cursor position
type Position struct {
	X int // Column (0-based)
	Y int // Row (0-based)
}

// ProcessAnsiResult contains the result of ANSI processing with coordinate extraction
type ProcessAnsiResult struct {
	ProcessedContent []byte                // Processed ANSI content
	PlaceholderCoords map[rune]Position    // Placeholder coordinates found during processing
	OriginalSize     Position             // Original dimensions of the content
}

// SAUCEInfo contains SAUCE metadata information
type SAUCEInfo struct {
	ID          string    // SAUCE record identifier
	Version     string    // SAUCE version
	Title       string    // Artwork title
	Author      string    // Artist name
	Group       string    // Artist group
	Date        time.Time // Creation date
	FileSize    int       // Original file size
	DataType    int       // Type of data (1=char, 2=bitmap, etc.)
	FileType    int       // Subtype within DataType
	TInfo1      int       // Type-specific info (usually width)
	TInfo2      int       // Type-specific info (usually height)
	TInfo3      int       // Type-specific info (usually font)
	TInfo4      int       // Type-specific info (flags)
	Comments    []string  // SAUCE comments
	IceColors   bool      // Whether ice colors are enabled
	NonBlink    bool      // Whether blinking is disabled
}

// Terminal provides a comprehensive terminal interface for BBS operations
type Terminal interface {
	// Core I/O operations
	Write([]byte) (int, error)
	WriteString(string) (int, error)
	Read([]byte) (int, error)
	ReadLine() (string, error)
	
	// Terminal control
	SetOutputMode(OutputMode)
	GetOutputMode() OutputMode
	GetCapabilities() Capabilities
	
	// ANSI art and content rendering
	RenderAnsiFile(filename string) error
	RenderAnsiBytes(data []byte) error
	ProcessPipeCodes(data []byte) []byte
	ProcessAnsiAndExtractCoords(rawContent []byte) (ProcessAnsiResult, error)
	
	// Screen control
	ClearScreen() error
	MoveCursor(row, col int) error
	SaveCursor() error
	RestoreCursor() error
	SetCursorVisible(visible bool) error
	
	// Content processing
	StripAnsi(content string) string
	GetAnsiFileContent(filename string) ([]byte, error)
	
	// SAUCE metadata
	ParseSAUCE(data []byte) (*SAUCEInfo, []byte, error)
	
	// Terminal state
	GetDimensions() (width, height int)
	SetDimensions(width, height int)
	GetCursorPosition() Position
	
	// Utility
	Close() error
}

// Factory function to create a new terminal instance
func New(session ssh.Session, outputMode OutputMode) Terminal {
	return newTerminalImpl(session, outputMode)
}

// NewFromWriter creates a terminal instance from a generic io.Writer
func NewFromWriter(writer io.Writer, outputMode OutputMode) Terminal {
	return newTerminalFromWriter(writer, outputMode)
}

// DetectTerminalType attempts to determine terminal type from TERM environment variable
func DetectTerminalType(termEnv string) TerminalType {
	termLower := strings.ToLower(termEnv)
	
	switch {
	case termEnv == "syncterm":
		return TerminalSyncTERM
	case termEnv == "ansi" || termEnv == "scoansi":
		return TerminalANSI
	case termEnv == "vt100" || termEnv == "vt102":
		return TerminalVT100
	case termEnv == "xterm" || termEnv == "xterm-256color" || termEnv == "screen":
		return TerminalXTerm
	case strings.Contains(termLower, "amiga") || termEnv == "amiga":
		return TerminalAmiga
	default:
		// Try to detect UTF-8 capability
		if termEnv == "xterm-256color" || termEnv == "screen-256color" {
			return TerminalUTF8
		}
		return TerminalUnknown
	}
}