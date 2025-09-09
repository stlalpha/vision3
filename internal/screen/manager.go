package screen

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/terminal"
)

// ScreenPosition represents different screen positioning strategies
type ScreenPosition int

const (
	PositionTop ScreenPosition = iota    // Start at top of screen
	PositionCenter                       // Center vertically 
	PositionOffset                       // Use fixed offset from top
)

// Standard BBS screen dimensions
const (
	StandardBBSWidth  = 80
	StandardBBSHeight = 25
)

// Manager handles all screen display operations with consistent positioning
type Manager struct {
	terminal       *terminal.Terminal
	defaultPosition ScreenPosition
	defaultOffset   int
}

// NewManager creates a new screen manager
func NewManager(term *terminal.Terminal) *Manager {
	return &Manager{
		terminal:       term,
		defaultPosition: PositionCenter,
		defaultOffset:   0,
	}
}

// SetDefaultPosition sets the default screen positioning strategy
func (sm *Manager) SetDefaultPosition(position ScreenPosition, offset int) {
	sm.defaultPosition = position
	sm.defaultOffset = offset
}

// DisplayMenu displays a menu ANSI file with consistent positioning
func (sm *Manager) DisplayMenu(ansiFilePath string) error {
	// Read ANSI file
	ansiContent, err := os.ReadFile(ansiFilePath)
	if err != nil {
		return fmt.Errorf("failed to read menu file %s: %w", ansiFilePath, err)
	}

	return sm.DisplayScreen(ansiContent)
}

// DisplayScreen displays ANSI content with smart positioning
func (sm *Manager) DisplayScreen(ansiContent []byte) error {
	return sm.DisplayScreenWithPosition(ansiContent, sm.defaultPosition, sm.defaultOffset)
}

// DisplayScreenWithPosition displays ANSI content with specified positioning
func (sm *Manager) DisplayScreenWithPosition(ansiContent []byte, position ScreenPosition, offset int) error {
	// Clear screen first for consistent display
	if err := sm.terminal.ClearScreen(); err != nil {
		log.Printf("WARN: Failed to clear screen: %v", err)
	}

	// Calculate starting position
	startRow := sm.calculateStartRow(position, offset)
	
	// Position cursor at calculated starting position
	if err := sm.terminal.MoveCursor(startRow, 1); err != nil {
		log.Printf("WARN: Failed to position cursor: %v", err)
	}

	// Process and display the ANSI content
	mode := sm.terminal.GetOutputMode()
	ansiProcessResult, err := ansi.ProcessAnsiAndExtractCoords(ansiContent, mode)
	if err != nil {
		return fmt.Errorf("failed to process ANSI content: %w", err)
	}

	// Write the processed content
	if _, err := sm.terminal.Write(ansiProcessResult.DisplayBytes); err != nil {
		return fmt.Errorf("failed to write screen content: %w", err)
	}

	return nil
}

// DisplayTemplate displays a template with variable substitution
func (sm *Manager) DisplayTemplate(templateContent []byte, variables map[string]string) error {
	// Replace template variables
	processedContent := string(templateContent)
	for key, value := range variables {
		processedContent = strings.ReplaceAll(processedContent, key, value)
	}

	return sm.DisplayScreen([]byte(processedContent))
}

// DisplayList displays dynamic content (like user lists, file lists) with consistent positioning
func (sm *Manager) DisplayList(content []byte) error {
	return sm.DisplayListWithPosition(content, sm.defaultPosition, sm.defaultOffset)
}

// DisplayListWithPosition displays dynamic content with specified positioning
func (sm *Manager) DisplayListWithPosition(content []byte, position ScreenPosition, offset int) error {
	// Clear screen first
	if err := sm.terminal.ClearScreen(); err != nil {
		log.Printf("WARN: Failed to clear screen: %v", err)
	}

	// Calculate starting position
	startRow := sm.calculateStartRow(position, offset)
	
	// Position cursor at calculated starting position
	if err := sm.terminal.MoveCursor(startRow, 1); err != nil {
		log.Printf("WARN: Failed to position cursor: %v", err)
	}

	// Write the content directly
	if _, err := sm.terminal.Write(content); err != nil {
		return fmt.Errorf("failed to write list content: %w", err)
	}

	return nil
}

// ShowPrompt displays a prompt without clearing the screen
func (sm *Manager) ShowPrompt(promptText string) (string, error) {
	// Process pipe codes in the prompt
	processedPrompt := ansi.ReplacePipeCodes([]byte(promptText))
	
	// Write prompt
	if _, err := sm.terminal.Write(processedPrompt); err != nil {
		return "", fmt.Errorf("failed to write prompt: %w", err)
	}

	// Read user input
	return sm.terminal.ReadLine()
}

// ShowPausePrompt displays a pause prompt and waits for input
func (sm *Manager) ShowPausePrompt(message string) (string, error) {
	if message == "" {
		message = "\r\n|07Press |15[ENTER]|07 to continue... "
	}
	
	return sm.ShowPrompt(message)
}

// DisplayMessage shows a formatted message with level-based coloring
func (sm *Manager) DisplayMessage(message, level string) error {
	capabilities := sm.terminal.GetCapabilities()
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
	
	processedMessage := ansi.ReplacePipeCodes([]byte(coloredMessage))
	_, err := sm.terminal.Write(processedMessage)
	return err
}

// calculateStartRow calculates the optimal starting row based on position strategy
func (sm *Manager) calculateStartRow(position ScreenPosition, offset int) int {
	_, termHeight := sm.terminal.GetDimensions()
	
	if termHeight <= 0 {
		return 1 // Fallback to top if we can't determine terminal height
	}
	
	var startRow int
	switch position {
	case PositionTop:
		startRow = 1
	case PositionCenter:
		// Center assuming content was designed for standard BBS dimensions
		startRow = (termHeight - StandardBBSHeight) / 2
		if startRow < 1 {
			startRow = 1
		}
	case PositionOffset:
		// Use specified offset, but ensure it's valid
		if offset < 1 {
			offset = 1
		}
		// Ensure the screen fits within terminal bounds
		if offset+StandardBBSHeight > termHeight {
			startRow = termHeight - StandardBBSHeight + 1
		} else {
			startRow = offset
		}
	default:
		startRow = 1
	}
	
	// DEBUG: Log positioning calculations
	log.Printf("DEBUG: ScreenManager - Terminal height: %d, Position: %d, Offset: %d, Calculated start row: %d", 
		termHeight, position, offset, startRow)
	
	return startRow
}

// ParsePosition converts a string position to ScreenPosition enum
func ParsePosition(posStr string) ScreenPosition {
	switch strings.ToUpper(posStr) {
	case "CENTER", "CENTRE":
		return PositionCenter
	case "TOP":
		return PositionTop
	case "OFFSET":
		return PositionOffset
	default:
		return PositionCenter // Default to center
	}
}