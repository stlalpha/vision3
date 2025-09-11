package terminal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/gliderlabs/ssh"
	"golang.org/x/crypto/ssh/terminal"
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
	session    ssh.Session
	inputTerm  *terminal.Terminal
}

// NewBBS creates a new BBS terminal system from SSH session
func NewBBS(session ssh.Session) *BBS {
	// Auto-detect output mode
	termType := getTermValue(session.Environ())
	outputMode := DetectOutputMode(termType)
	
	// Create input terminal for reading user input
	inputTerm := terminal.NewTerminal(session, "")
	
	return &BBS{
		writer:     session,
		outputMode: outputMode,
		session:    session,
		inputTerm:  inputTerm,
	}
}

// NewBBSFromWriter creates a BBS terminal from generic writer
func NewBBSFromWriter(writer io.Writer, outputMode OutputMode) *BBS {
	return &BBS{
		writer:     writer,
		outputMode: outputMode,
		session:    nil,
		inputTerm:  nil,
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

// ProcessPipeCodes processes ViSiON/2 pipe codes in content without displaying
// This is useful for template processing where display happens later
func (b *BBS) ProcessPipeCodes(data []byte) []byte {
	// Step 1: Convert CP437 to UTF-8 if needed
	processedData, err := b.convertEncoding(data)
	if err != nil {
		// Return original data if conversion fails
		return data
	}
	
	// Step 2: Process ViSiON/2 pipe codes to ANSI
	return b.processPipeCodes(processedData)
}

// ReadLine reads a line of input from the terminal
func (b *BBS) ReadLine() (string, error) {
	if b.inputTerm == nil {
		return "", errors.New("no input terminal available - BBS not created with SSH session")
	}
	return b.inputTerm.ReadLine()
}

// SaveCursor saves the current cursor position (ANSI escape sequence)
func (b *BBS) SaveCursor() error {
	_, err := b.writer.Write([]byte("\x1b[s"))
	return err
}

// RestoreCursor restores the saved cursor position (ANSI escape sequence)  
func (b *BBS) RestoreCursor() error {
	_, err := b.writer.Write([]byte("\x1b[u"))
	return err
}

// MoveCursor returns ANSI escape sequence to move cursor to specified row/column (1-based)
func MoveCursor(row, col int) string {
	return fmt.Sprintf("\x1b[%d;%dH", row, col)
}

// GetAnsiFileContent reads and returns the content of an ANSI file
func GetAnsiFileContent(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// SaveCursor returns ANSI escape sequence to save cursor position
func SaveCursor() string {
	return "\x1b[s"
}

// RestoreCursor returns ANSI escape sequence to restore cursor position
func RestoreCursor() string {
	return "\x1b[u"
}

// ProcessAnsiResult represents the result of ANSI processing
type ProcessAnsiResult struct {
	ProcessedContent    []byte
	PlaceholderCoords   map[string]struct{ X, Y int }
}

// ProcessAnsiAndExtractCoords processes ANSI content and extracts coordinates
func ProcessAnsiAndExtractCoords(data []byte, outputMode OutputMode) (ProcessAnsiResult, error) {
	// Create a temporary BBS instance to use the same processing pipeline
	tempBBS := &BBS{
		writer:     nil, // Not needed for processing
		outputMode: outputMode,
	}
	
	// Process the data using the same pipeline as DisplayContent
	processedData, err := tempBBS.convertEncoding(data)
	if err != nil {
		return ProcessAnsiResult{}, err
	}
	
	// Extract coordinates BEFORE processing pipe codes
	coords := extractCoordinates(string(processedData))
	
	// Remove coordinate markers before processing pipe codes
	cleanedData := removeCoordinateMarkers(string(processedData))
	
	// Process ViSiON/2 pipe codes to ANSI
	finalData := tempBBS.processPipeCodes([]byte(cleanedData))
	
	return ProcessAnsiResult{
		ProcessedContent:  finalData,
		PlaceholderCoords: coords,
	}, nil
}

// extractCoordinates parses ANSI content to find coordinate markers like |P, |O
func extractCoordinates(content string) map[string]struct{ X, Y int } {
	coords := make(map[string]struct{ X, Y int })
	
	lines := strings.Split(content, "\n")
	for lineNum, line := range lines {
		currentX := 1
		i := 0
		
		for i < len(line) {
			char := line[i]
			
			// Handle ANSI escape sequences properly
			if char == '\x1b' && i+1 < len(line) && line[i+1] == '[' {
				// Find the end of the escape sequence
				j := i + 2
				for j < len(line) {
					c := line[j]
					if c >= '0' && c <= '9' || c == ';' || c == '?' {
						j++
					} else {
						// This is the command letter - include it and stop
						j++
						break
					}
				}
				
				// Check if this is a cursor movement command that affects X position
				escapeSeq := line[i:j]
				if strings.Contains(escapeSeq, "C") { // Cursor forward
					// Extract number before C
					numStr := strings.TrimPrefix(escapeSeq, "\x1b[")
					numStr = strings.TrimSuffix(numStr, "C")
					if num := parseInt(numStr); num > 0 {
						currentX += num
					} else {
						currentX += 1 // Default to 1 if no number
					}
				} else if strings.Contains(escapeSeq, "D") { // Cursor backward
					numStr := strings.TrimPrefix(escapeSeq, "\x1b[")
					numStr = strings.TrimSuffix(numStr, "D")
					if num := parseInt(numStr); num > 0 {
						currentX -= num
					} else {
						currentX -= 1
					}
					if currentX < 1 {
						currentX = 1
					}
				} else if strings.Contains(escapeSeq, "H") || strings.Contains(escapeSeq, "f") {
					// Absolute cursor positioning - parse row;col
					params := strings.TrimPrefix(escapeSeq, "\x1b[")
					params = strings.TrimSuffix(params, "H")
					params = strings.TrimSuffix(params, "f")
					if strings.Contains(params, ";") {
						parts := strings.Split(params, ";")
						if len(parts) >= 2 {
							if col := parseInt(parts[1]); col > 0 {
								currentX = col
							}
						}
					}
				}
				
				i = j
				continue
			}
			
			// Handle carriage return
			if char == '\r' {
				currentX = 1
				i++
				continue
			}
			
			// Look for coordinate markers like |P, |O
			if char == '|' && i+1 < len(line) {
				marker := line[i+1]
				if marker == 'P' || marker == 'O' {
					coords[string(marker)] = struct{ X, Y int }{
						X: currentX,
						Y: lineNum + 1, // Convert to 1-based
					}
					i += 2 // Skip the marker
					continue
				}
			}
			
			// Regular character - advance cursor
			currentX++
			i++
		}
	}
	
	return coords
}

// parseInt parses a string to int, returns 0 if invalid
func parseInt(s string) int {
	if s == "" {
		return 0
	}
	result := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return result
}

// removeCoordinateMarkers removes coordinate markers like |P, |O from content
func removeCoordinateMarkers(content string) string {
	// Remove |P and |O markers
	content = strings.ReplaceAll(content, "|P", "")
	content = strings.ReplaceAll(content, "|O", "")
	return content
}

// UnicodeToCP437Table is a placeholder for character conversion
var UnicodeToCP437Table = make(map[rune]byte)

// Cp437ToUnicode is a mapping from CP437 bytes to Unicode runes
var Cp437ToUnicode = make(map[byte]rune)

// Cp437ToUnicodeString converts CP437 bytes to Unicode string
func Cp437ToUnicodeString(data []byte) string {
	// Simple implementation - convert to string
	return string(data)
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

// ProcessPipeCodes converts ViSiON/2 pipe codes to ANSI escape sequences (public function)
func ProcessPipeCodes(data []byte) []byte {
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