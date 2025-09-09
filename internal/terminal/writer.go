package terminal

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/stlalpha/vision3/internal/ansi"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// WriteProcessed writes data to the terminal using the appropriate encoding and processing
func (t *Terminal) WriteProcessed(data []byte) error {
	capabilities := t.GetCapabilities()
	mode := t.GetOutputMode()
	
	// Use capability-aware processing
	return t.writeWithEncoding(data, mode, capabilities)
}

// WritePipeCodes processes pipe codes and writes to the terminal
func (t *Terminal) WritePipeCodes(data []byte) error {
	processed := ansi.ReplacePipeCodes(data)
	return t.WriteProcessed(processed)
}

// WriteString writes a string to the terminal
func (t *Terminal) WriteString(s string) error {
	return t.WriteProcessed([]byte(s))
}

// WriteLine writes a line with CRLF termination
func (t *Terminal) WriteLine(s string) error {
	return t.WriteString(s + "\r\n")
}

// WriteANSI writes ANSI data optimized for the terminal type
func (t *Terminal) WriteANSI(data []byte) error {
	capabilities := t.GetCapabilities()
	
	// If terminal doesn't support color, strip color codes
	if !capabilities.SupportsColor {
		processed := ansi.StripANSICodes(data)
		return t.WriteProcessed(processed)
	}
	
	// If terminal supports line drawing but not UTF-8, convert to line drawing
	if capabilities.SupportsLineDrawing && !capabilities.SupportsUTF8 {
		converted := ansi.ConvertToLineDrawing(data)
		return t.WriteProcessed(converted)
	}
	
	// Standard processing
	return t.WriteProcessed(data)
}

// ClearScreen clears the terminal screen
func (t *Terminal) ClearScreen() error {
	return t.WriteString(ansi.ClearScreen())
}

// MoveCursor moves the cursor to specified position (1-based)
func (t *Terminal) MoveCursor(row, col int) error {
	return t.WriteString(fmt.Sprintf("\x1b[%d;%dH", row, col))
}

// SetColor sets the foreground and background colors
func (t *Terminal) SetColor(fg, bg int) error {
	capabilities := t.GetCapabilities()
	
	if !capabilities.SupportsColor {
		return nil // Ignore color commands for non-color terminals
	}
	
	// Clamp colors to terminal capabilities
	maxColors := capabilities.MaxColors
	if maxColors > 0 {
		fg = fg % maxColors
		bg = bg % maxColors
	}
	
	return t.WriteString(fmt.Sprintf("\x1b[%d;%dm", 30+fg, 40+bg))
}

// ResetAttributes resets all terminal attributes to default
func (t *Terminal) ResetAttributes() error {
	return t.WriteString("\x1b[0m")
}

// WriteWithCapabilities writes data with terminal capability awareness
func (t *Terminal) WriteWithCapabilities(data []byte) error {
	capabilities := t.GetCapabilities()
	mode := t.GetOutputMode()
	
	// Use the enhanced encoding pipeline
	return t.writeWithEncoding(data, mode, capabilities)
}

// Prompt displays a prompt and waits for input
func (t *Terminal) Prompt(promptText string) (string, error) {
	err := t.WriteString(promptText)
	if err != nil {
		return "", fmt.Errorf("failed to write prompt: %w", err)
	}
	
	return t.ReadLine()
}

// PromptWithDefault displays a prompt with a default value
func (t *Terminal) PromptWithDefault(promptText, defaultValue string) (string, error) {
	fullPrompt := fmt.Sprintf("%s [%s]: ", promptText, defaultValue)
	response, err := t.Prompt(fullPrompt)
	if err != nil {
		return "", err
	}
	
	if response == "" {
		return defaultValue, nil
	}
	
	return response, nil
}

// DisplayStatus shows a status message with optional timeout
func (t *Terminal) DisplayStatus(message string) error {
	return t.WriteLine(fmt.Sprintf("[STATUS] %s", message))
}

// DisplayError shows an error message
func (t *Terminal) DisplayError(message string) error {
	capabilities := t.GetCapabilities()
	
	if capabilities.SupportsColor {
		// Red text for errors if color is supported
		err := t.WriteString("\x1b[31m[ERROR] " + message + "\x1b[0m\r\n")
		return err
	} else {
		return t.WriteLine("[ERROR] " + message)
	}
}

// DisplayWarning shows a warning message  
func (t *Terminal) DisplayWarning(message string) error {
	capabilities := t.GetCapabilities()
	
	if capabilities.SupportsColor {
		// Yellow text for warnings if color is supported
		err := t.WriteString("\x1b[33m[WARNING] " + message + "\x1b[0m\r\n")
		return err
	} else {
		return t.WriteLine("[WARNING] " + message)
	}
}

// DisplayInfo shows an informational message
func (t *Terminal) DisplayInfo(message string) error {
	capabilities := t.GetCapabilities()
	
	if capabilities.SupportsColor {
		// Cyan text for info if color is supported  
		err := t.WriteString("\x1b[36m[INFO] " + message + "\x1b[0m\r\n")
		return err
	} else {
		return t.WriteLine("[INFO] " + message)
	}
}

// writeWithEncoding handles character encoding based on terminal capabilities and output mode
func (t *Terminal) writeWithEncoding(data []byte, mode ansi.OutputMode, capabilities Capabilities) error {
	// Process data based on capabilities
	processed := data
	
	// Handle color support
	if !capabilities.SupportsColor {
		processed = ansi.StripANSICodes(processed)
		log.Printf("Terminal: Stripped ANSI color codes for non-color terminal")
	}
	
	// Handle UTF-8 support and encoding
	if !capabilities.SupportsUTF8 && mode == ansi.OutputModeUTF8 {
		// Force CP437 mode for non-UTF8 terminals
		mode = ansi.OutputModeCP437
		log.Printf("Terminal: Forced CP437 mode for non-UTF8 terminal")
	}
	
	// Handle line drawing
	if capabilities.SupportsLineDrawing && mode == ansi.OutputModeCP437 {
		processed = ansi.ConvertToLineDrawing(processed)
		log.Printf("Terminal: Converted to line drawing characters")
	}
	
	// Apply encoding based on mode
	switch mode {
	case ansi.OutputModeCP437:
		// For CP437 mode, encode text while preserving ANSI codes
		return t.writeWithCP437Encoding(processed)
	case ansi.OutputModeUTF8:
		// For UTF-8 mode, write directly
		_, err := t.writer.Write(processed)
		return err
	case ansi.OutputModeAuto:
		// Auto mode: decide based on capabilities
		if capabilities.SupportsUTF8 {
			_, err := t.writer.Write(processed)
			return err
		} else {
			return t.writeWithCP437Encoding(processed)
		}
	default:
		// Fallback to direct writing
		_, err := t.writer.Write(processed)
		return err
	}
}

// ScreenPositioning represents screen positioning strategies
type ScreenPositioning int

const (
	PositionTop ScreenPositioning = iota    // Start at top of screen
	PositionCenter                          // Center vertically 
	PositionOffset                          // Use fixed offset from top
)

// Standard BBS screen sizes
const (
	StandardBBSWidth80  = 80
	StandardBBSWidth132 = 132
	StandardBBSHeight25 = 25
	StandardBBSHeight43 = 43
	StandardBBSHeight50 = 50
)

// analyzeScreenHeight analyzes ANSI content to determine its effective height
func (t *Terminal) analyzeScreenHeight(displayBytes []byte) int {
	if len(displayBytes) == 0 {
		return 0
	}
	
	// Count the number of line breaks to estimate height
	lineCount := 1 // Start with 1 for the first line
	
	// Count \n, \r\n, and ANSI cursor movements that indicate new lines
	for i := 0; i < len(displayBytes); i++ {
		if displayBytes[i] == '\n' {
			lineCount++
		} else if displayBytes[i] == '\r' && i+1 < len(displayBytes) && displayBytes[i+1] == '\n' {
			lineCount++
			i++ // Skip the \n after \r\n
		}
	}
	
	return lineCount
}

// calculateStartRow calculates the optimal starting row for screen display
func (t *Terminal) calculateStartRow(screenHeight int, positioning ScreenPositioning, offset int) int {
	_, termHeight := t.GetDimensions()
	
	if termHeight <= 0 {
		return 1 // Fallback to top if we can't determine terminal height
	}
	
	// Assume ANSI content is designed for standard BBS screen size (80x25)
	// Center based on standard dimensions, not actual screen height
	standardHeight := StandardBBSHeight25
	
	switch positioning {
	case PositionTop:
		return 1
	case PositionCenter:
		// Center assuming content was designed for 80x25 terminal
		startRow := (termHeight - standardHeight) / 2
		if startRow < 1 {
			startRow = 1
		}
		return startRow
	case PositionOffset:
		// Use fixed offset, but ensure it's valid
		if offset < 1 {
			offset = 1
		}
		// Ensure the screen fits within terminal bounds
		if offset+standardHeight > termHeight {
			return termHeight - standardHeight + 1
		}
		return offset
	default:
		return 1
	}
}

// adjustCursorPositions modifies ANSI cursor positioning commands by adding row/column offsets
func (t *Terminal) adjustCursorPositions(data []byte, rowOffset, colOffset int) []byte {
	if rowOffset == 0 && colOffset == 0 {
		return data // No adjustment needed
	}
	
	result := make([]byte, 0, len(data))
	i := 0
	
	for i < len(data) {
		if data[i] == '\x1b' && i+1 < len(data) && data[i+1] == '[' {
			// Found ANSI escape sequence
			start := i
			i += 2 // Skip ESC [
			
			// Parse parameters
			var params []int
			currentParam := ""
			
			for i < len(data) && (data[i] >= '0' && data[i] <= '9' || data[i] == ';') {
				if data[i] == ';' {
					if currentParam != "" {
						if val := parseInt(currentParam); val >= 0 {
							params = append(params, val)
						}
						currentParam = ""
					} else {
						params = append(params, 1) // Default parameter
					}
				} else {
					currentParam += string(data[i])
				}
				i++
			}
			
			// Add final parameter
			if currentParam != "" {
				if val := parseInt(currentParam); val >= 0 {
					params = append(params, val)
				}
			}
			
			// Check the command character
			if i < len(data) {
				cmd := data[i]
				i++ // Include command character
				
				// Adjust cursor positioning commands
				if cmd == 'H' || cmd == 'f' { // Cursor Position
					if len(params) >= 2 {
						// ESC[row;colH format - adjust both
						newRow := params[0] + rowOffset
						newCol := params[1] + colOffset
						if newRow < 1 { newRow = 1 }
						if newCol < 1 { newCol = 1 }
						result = append(result, []byte(fmt.Sprintf("\x1b[%d;%dH", newRow, newCol))...)
						continue
					} else if len(params) == 1 {
						// ESC[rowH format - adjust row only  
						newRow := params[0] + rowOffset
						if newRow < 1 { newRow = 1 }
						result = append(result, []byte(fmt.Sprintf("\x1b[%dH", newRow))...)
						continue
					} else {
						// ESC[H (home) - adjust to offset position
						result = append(result, []byte(fmt.Sprintf("\x1b[%d;%dH", rowOffset+1, colOffset+1))...)
						continue
					}
				}
			}
			
			// Copy original sequence for non-positioning commands
			result = append(result, data[start:i]...)
		} else {
			result = append(result, data[i])
			i++
		}
	}
	
	return result
}

// parseInt safely converts string to int, returns -1 on error
func parseInt(s string) int {
	result := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			result = result*10 + int(r-'0')
		} else {
			return -1
		}
	}
	return result
}

// DisplayScreen processes and displays a complete ANSI screen file with smart positioning
func (t *Terminal) DisplayScreen(ansiContent []byte) error {
	mode := t.GetOutputMode()
	
	// Clear screen first for consistent display
	clearErr := t.ClearScreen()
	if clearErr != nil {
		log.Printf("WARN: Failed to clear screen before display: %v", clearErr)
	}
	
	// Calculate starting position for centering
	_, termHeight := t.GetDimensions()
	startRow := 1
	
	// Center the content assuming it's designed for 80x25
	if termHeight > StandardBBSHeight25 {
		startRow = (termHeight - StandardBBSHeight25) / 2
		if startRow < 1 {
			startRow = 1
		}
	}
	
	// Position cursor at the calculated starting position
	moveErr := t.MoveCursor(startRow, 1)
	if moveErr != nil {
		log.Printf("WARN: Failed to position cursor: %v", moveErr)
	}
	
	// Process the ANSI content through the sophisticated pipeline
	ansiProcessResult, processErr := ansi.ProcessAnsiAndExtractCoords(ansiContent, mode)
	if processErr != nil {
		return fmt.Errorf("failed to process ANSI screen: %w", processErr)
	}
	
	// Write the processed display bytes directly - ANSI positioning will work relative to cursor
	_, wErr := t.Write(ansiProcessResult.DisplayBytes)
	if wErr != nil {
		return fmt.Errorf("failed to write processed screen: %w", wErr)
	}
	
	return nil
}

// DisplayTemplate processes template variables and displays the ANSI screen
func (t *Terminal) DisplayTemplate(template []byte, variables map[string]string) error {
	// Replace template variables
	processedContent := string(template)
	for key, value := range variables {
		processedContent = strings.ReplaceAll(processedContent, key, value)
	}
	
	return t.DisplayScreen([]byte(processedContent))
}

// ShowPausePrompt displays a pause prompt and waits for input
func (t *Terminal) ShowPausePrompt(message string) (string, error) {
	if message == "" {
		message = "\r\n|07Press |15[ENTER]|07 to continue... "
	}
	
	// Use WritePipeCodes for prompt with pipe codes
	wErr := t.WritePipeCodes([]byte(message))
	if wErr != nil {
		return "", fmt.Errorf("failed to write pause prompt: %w", wErr)
	}
	
	// Read user input
	return t.ReadLine()
}

// DisplayMessage shows a formatted message with level-based coloring
func (t *Terminal) DisplayMessage(message, level string) error {
	capabilities := t.GetCapabilities()
	var coloredMessage string
	
	if capabilities.SupportsColor {
		switch strings.ToLower(level) {
		case "error":
			coloredMessage = fmt.Sprintf("\r\n|01[ERROR]|07 %s\r\n", message)
		case "warning":
			coloredMessage = fmt.Sprintf("\r\n|03[WARNING]|07 %s\r\n", message) 
		case "info":
			coloredMessage = fmt.Sprintf("\r\n|06[INFO]|07 %s\r\n", message)
		case "success":
			coloredMessage = fmt.Sprintf("\r\n|02[SUCCESS]|07 %s\r\n", message)
		default:
			coloredMessage = fmt.Sprintf("\r\n%s\r\n", message)
		}
	} else {
		// No color support - use plain text
		switch strings.ToLower(level) {
		case "error":
			coloredMessage = fmt.Sprintf("\r\n[ERROR] %s\r\n", message)
		case "warning":
			coloredMessage = fmt.Sprintf("\r\n[WARNING] %s\r\n", message)
		case "info":
			coloredMessage = fmt.Sprintf("\r\n[INFO] %s\r\n", message)
		case "success":
			coloredMessage = fmt.Sprintf("\r\n[SUCCESS] %s\r\n", message)
		default:
			coloredMessage = fmt.Sprintf("\r\n%s\r\n", message)
		}
	}
	
	return t.WritePipeCodes([]byte(coloredMessage))
}

// DisplayScreenFromFile loads and displays an ANSI file from disk
func (t *Terminal) DisplayScreenFromFile(filePath string) error {
	ansiContent, readErr := os.ReadFile(filePath)
	if readErr != nil {
		return fmt.Errorf("failed to read screen file %s: %w", filePath, readErr)
	}
	
	return t.DisplayScreen(ansiContent)
}

// DisplayTemplateFromFile loads a template file, processes variables, and displays it
func (t *Terminal) DisplayTemplateFromFile(filePath string, variables map[string]string) error {
	templateContent, readErr := os.ReadFile(filePath)
	if readErr != nil {
		return fmt.Errorf("failed to read template file %s: %w", filePath, readErr)
	}
	
	return t.DisplayTemplate(templateContent, variables)
}

// writeWithCP437Encoding encodes text to CP437 while preserving ANSI escape sequences
func (t *Terminal) writeWithCP437Encoding(data []byte) error {
	encoder := charmap.CodePage437.NewEncoder()
	
	// Simple approach: use ANSI-aware encoding similar to SelectiveCP437Writer
	var result []byte
	i := 0
	
	for i < len(data) {
		if data[i] == 0x1b && i+1 < len(data) && data[i+1] == '[' {
			// Found ANSI escape sequence, find the end
			start := i
			i += 2 // Skip ESC [
			
			// Find the terminator (A-Z, a-z)
			for i < len(data) && !((data[i] >= 'A' && data[i] <= 'Z') || (data[i] >= 'a' && data[i] <= 'z')) {
				i++
			}
			if i < len(data) {
				i++ // Include the terminator
			}
			
			// Append the ANSI sequence as-is
			result = append(result, data[start:i]...)
		} else {
			// Regular character, encode to CP437
			char := []byte{data[i]}
			encoded, _, err := transform.Bytes(encoder, char)
			if err == nil && len(encoded) > 0 {
				result = append(result, encoded...)
			} else {
				// Fallback for unencodable characters
				result = append(result, '?')
			}
			i++
		}
	}
	
	_, err := t.writer.Write(result)
	return err
}