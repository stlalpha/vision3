package terminal

import (
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/gliderlabs/ssh"
	"golang.org/x/text/encoding/charmap"
)

// OutputMode specifies how terminal output should be processed
type OutputMode int

const (
	OutputModeAuto OutputMode = iota
	OutputModeUTF8
	OutputModeCP437
)

// BBS represents a simple, direct BBS terminal system
type BBS struct {
	writer     io.Writer
	outputMode OutputMode
}

// NewBBS creates a new BBS terminal system from SSH session
func NewBBS(session ssh.Session) *BBS {
	// Auto-detect output mode
	termType := getTermValue(session.Environ())
	outputMode := DetectOutputMode(termType)
	
	return &BBS{
		writer:     session,
		outputMode: outputMode,
	}
}

// NewBBSFromWriter creates a BBS terminal from generic writer
func NewBBSFromWriter(writer io.Writer, outputMode OutputMode) *BBS {
	return &BBS{
		writer:     writer,
		outputMode: outputMode,
	}
}

// DisplayFile processes and displays an ANSI file
func (b *BBS) DisplayFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	
	return b.DisplayContent(data)
}

// DisplayContent processes and displays raw content
func (b *BBS) DisplayContent(data []byte) error {
	// Step 1: Convert CP437 to UTF-8 if needed
	processedData, err := b.convertEncoding(data)
	if err != nil {
		return err
	}
	
	// Step 2: Process ViSiON/2 pipe codes
	ansiData := b.processPipeCodes(processedData)
	
	// Step 3: Write directly to SSH client
	_, err = b.writer.Write(ansiData)
	return err
}

// Write writes raw data directly to the client
func (b *BBS) Write(data []byte) (int, error) {
	return b.writer.Write(data)
}

// WriteString writes a string directly to the client
func (b *BBS) WriteString(s string) (int, error) {
	return b.writer.Write([]byte(s))
}

// convertEncoding handles character encoding conversion with smart detection
func (b *BBS) convertEncoding(data []byte) ([]byte, error) {
	if b.outputMode == OutputModeUTF8 {
		// Only convert if input appears to be raw CP437, not already UTF-8
		if isLikelyCP437(data) {
			decoder := charmap.CodePage437.NewDecoder()
			return decoder.Bytes(data)
		}
		// Already UTF-8 or valid ASCII, return as-is
		return data, nil
	}
	// For CP437 mode or auto, return as-is
	return data, nil
}

// isLikelyCP437 determines if data contains raw CP437 bytes that need conversion
func isLikelyCP437(data []byte) bool {
	// If data is valid UTF-8, it's probably already UTF-8
	if utf8.Valid(data) {
		// Check if it contains high-bit characters that would be CP437
		// UTF-8 sequences for box drawing chars start with 0xE2
		hasHighChars := false
		for _, b := range data {
			if b >= 0x80 {
				hasHighChars = true
				break
			}
		}
		// If no high chars, it's just ASCII - safe to convert
		// If has high chars and valid UTF-8, probably already UTF-8
		return !hasHighChars
	}
	
	// Invalid UTF-8 suggests raw CP437 bytes
	return true
}

// processPipeCodes converts ViSiON/2 pipe codes to ANSI escape sequences
func (b *BBS) processPipeCodes(data []byte) []byte {
	content := string(data)
	
	// ViSiON/2 pipe code mappings to ANSI escape sequences
	pipeCodes := map[string]string{
		// Foreground colors
		"|00": "\x1b[30m",    // Black
		"|01": "\x1b[34m",    // Blue
		"|02": "\x1b[32m",    // Green
		"|03": "\x1b[36m",    // Cyan
		"|04": "\x1b[31m",    // Red
		"|05": "\x1b[35m",    // Magenta
		"|06": "\x1b[33m",    // Brown/Yellow
		"|07": "\x1b[37m",    // Light Gray
		"|08": "\x1b[1;30m",  // Dark Gray (bright black)
		"|09": "\x1b[1;34m",  // Light Blue
		"|10": "\x1b[1;32m",  // Light Green
		"|11": "\x1b[1;36m",  // Light Cyan
		"|12": "\x1b[1;31m",  // Light Red
		"|13": "\x1b[1;35m",  // Light Magenta
		"|14": "\x1b[1;33m",  // Yellow
		"|15": "\x1b[1;37m",  // White
		
		// Background colors
		"|B0": "\x1b[40m",    // Black BG
		"|B1": "\x1b[44m",    // Blue BG
		"|B2": "\x1b[42m",    // Green BG
		"|B3": "\x1b[46m",    // Cyan BG
		"|B4": "\x1b[41m",    // Red BG
		"|B5": "\x1b[45m",    // Magenta BG
		"|B6": "\x1b[43m",    // Brown BG
		"|B7": "\x1b[47m",    // Light Gray BG
		
		// Special codes
		"|RS": "\x1b[0m",       // Reset
		"|CL": "\x1b[2J\x1b[H", // Clear screen
		"|CR": "\r",            // Carriage return
		"|LF": "\n",            // Line feed
		"|BL": "\x1b[5m",       // Blink
		"|RV": "\x1b[7m",       // Reverse
	}
	
	// Replace all pipe codes with ANSI sequences
	for pipeCode, ansiCode := range pipeCodes {
		content = strings.ReplaceAll(content, pipeCode, ansiCode)
	}
	
	return []byte(content)
}

// Helper functions

// getTermValue extracts TERM value from environment
func getTermValue(environ []string) string {
	for _, env := range environ {
		if strings.HasPrefix(env, "TERM=") {
			return strings.TrimPrefix(env, "TERM=")
		}
	}
	return "unknown"
}

// DetectOutputMode determines output mode from terminal type
func DetectOutputMode(termType string) OutputMode {
	termType = strings.ToLower(termType)
	
	// BBS terminals prefer CP437
	if strings.Contains(termType, "sync") || strings.Contains(termType, "ansi") {
		return OutputModeCP437
	}
	
	// Modern terminals use UTF-8
	return OutputModeUTF8
}