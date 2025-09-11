package strings

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/stlalpha/vision3/internal/terminal"
)

// ANSIColorInfo represents information about an ANSI color code
type ANSIColorInfo struct {
	Code        string
	Name        string
	Description string
	Foreground  int
	Background  int
	Preview     string
}

// ANSIHelper provides utilities for working with ANSI codes in strings
type ANSIHelper struct {
	colorMap map[string]ANSIColorInfo
}

// NewANSIHelper creates a new ANSI helper instance
func NewANSIHelper() *ANSIHelper {
	helper := &ANSIHelper{
		colorMap: make(map[string]ANSIColorInfo),
	}
	helper.initializeColorMap()
	return helper
}

// initializeColorMap sets up the standard ViSiON/2 color codes
func (ah *ANSIHelper) initializeColorMap() {
	// Standard ViSiON/2 color codes (|00 - |15)
	standardColors := []ANSIColorInfo{
		{"|00", "Black", "Black text", 30, 40, "■"},
		{"|01", "Red", "Red text", 31, 41, "■"},
		{"|02", "Green", "Green text", 32, 42, "■"},
		{"|03", "Brown", "Brown/Yellow text", 33, 43, "■"},
		{"|04", "Blue", "Blue text", 34, 44, "■"},
		{"|05", "Magenta", "Magenta text", 35, 45, "■"},
		{"|06", "Cyan", "Cyan text", 36, 46, "■"},
		{"|07", "Gray", "Gray text", 37, 47, "■"},
		{"|08", "Dark Gray", "Dark gray (bright black)", 90, 100, "■"},
		{"|09", "Bright Red", "Bright red text", 91, 101, "■"},
		{"|10", "Bright Green", "Bright green text", 92, 102, "■"},
		{"|11", "Yellow", "Yellow text", 93, 103, "■"},
		{"|12", "Bright Blue", "Bright blue text", 94, 104, "■"},
		{"|13", "Bright Magenta", "Bright magenta text", 95, 105, "■"},
		{"|14", "Bright Cyan", "Bright cyan text", 96, 106, "■"},
		{"|15", "White", "White text", 97, 107, "■"},
	}

	// Background colors (|B0 - |B7)
	backgroundColors := []ANSIColorInfo{
		{"|B0", "Black BG", "Black background", 30, 40, "  "},
		{"|B1", "Red BG", "Red background", 31, 41, "  "},
		{"|B2", "Green BG", "Green background", 32, 42, "  "},
		{"|B3", "Brown BG", "Brown background", 33, 43, "  "},
		{"|B4", "Blue BG", "Blue background", 34, 44, "  "},
		{"|B5", "Magenta BG", "Magenta background", 35, 45, "  "},
		{"|B6", "Cyan BG", "Cyan background", 36, 46, "  "},
		{"|B7", "Gray BG", "Gray background", 37, 47, "  "},
	}

	// Special codes
	specialCodes := []ANSIColorInfo{
		{"|CL", "Clear Screen", "Clear screen and home cursor", 0, 0, "⌂"},
		{"|P", "Save Cursor", "Save cursor position", 0, 0, "⇥"},
		{"|PP", "Restore Cursor", "Restore cursor position", 0, 0, "⇤"},
		{"|23", "Reset", "Reset all attributes", 0, 0, "⟲"},
	}

	// Custom color codes (|C1 - |C7)
	customColors := []ANSIColorInfo{
		{"|C1", "Custom 1", "Custom color 1", 0, 0, "◆"},
		{"|C2", "Custom 2", "Custom color 2", 0, 0, "◆"},
		{"|C3", "Custom 3", "Custom color 3", 0, 0, "◆"},
		{"|C4", "Custom 4", "Custom color 4", 0, 0, "◆"},
		{"|C5", "Custom 5", "Custom color 5", 0, 0, "◆"},
		{"|C6", "Custom 6", "Custom color 6", 0, 0, "◆"},
		{"|C7", "Custom 7", "Custom color 7", 0, 0, "◆"},
	}

	// Add all color codes to the map
	for _, color := range standardColors {
		ah.colorMap[color.Code] = color
	}
	for _, color := range backgroundColors {
		ah.colorMap[color.Code] = color
	}
	for _, color := range specialCodes {
		ah.colorMap[color.Code] = color
	}
	for _, color := range customColors {
		ah.colorMap[color.Code] = color
	}
}

// GetAvailableColors returns all available color codes
func (ah *ANSIHelper) GetAvailableColors() []ANSIColorInfo {
	var colors []ANSIColorInfo
	for _, color := range ah.colorMap {
		colors = append(colors, color)
	}
	return colors
}

// GetColorsByCategory returns colors grouped by category
func (ah *ANSIHelper) GetColorsByCategory() map[string][]ANSIColorInfo {
	categories := map[string][]ANSIColorInfo{
		"Standard":   {},
		"Bright":     {},
		"Background": {},
		"Special":    {},
		"Custom":     {},
	}

	for _, colorInfo := range ah.colorMap {
		switch {
		case strings.HasPrefix(colorInfo.Code, "|B"):
			categories["Background"] = append(categories["Background"], colorInfo)
		case strings.HasPrefix(colorInfo.Code, "|C"):
			categories["Custom"] = append(categories["Custom"], colorInfo)
		case colorInfo.Code == "|CL" || colorInfo.Code == "|P" || colorInfo.Code == "|PP" || colorInfo.Code == "|23":
			categories["Special"] = append(categories["Special"], colorInfo)
		case strings.Contains(colorInfo.Name, "Bright"):
			categories["Bright"] = append(categories["Bright"], colorInfo)
		default:
			categories["Standard"] = append(categories["Standard"], colorInfo)
		}
	}

	return categories
}

// ExtractANSICodes finds all ANSI codes in a string
func (ah *ANSIHelper) ExtractANSICodes(text string) []string {
	// Regex to match ViSiON/2 color codes
	re := regexp.MustCompile(`\|([0-9]{2}|B[0-7]|C[1-7]|CL|P{1,2}|23)`)
	matches := re.FindAllString(text, -1)
	
	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, match := range matches {
		if !seen[match] {
			seen[match] = true
			unique = append(unique, match)
		}
	}
	
	return unique
}

// RenderWithColors converts ViSiON/2 codes to ANSI escape sequences for preview
func (ah *ANSIHelper) RenderWithColors(text string) string {
	// Use the existing ANSI package pipe code replacement
	return string(terminal.ProcessPipeCodes([]byte(text)))
}

// StripColors removes all color codes from text
func (ah *ANSIHelper) StripColors(text string) string {
	re := regexp.MustCompile(`\|([0-9]{2}|B[0-7]|C[1-7]|CL|P{1,2}|23)`)
	return re.ReplaceAllString(text, "")
}

// GetColorInfo returns information about a specific color code
func (ah *ANSIHelper) GetColorInfo(code string) (ANSIColorInfo, bool) {
	colorInfo, exists := ah.colorMap[code]
	return colorInfo, exists
}

// ValidateColorCode checks if a color code is valid
func (ah *ANSIHelper) ValidateColorCode(code string) bool {
	_, exists := ah.colorMap[code]
	return exists
}

// BuildColoredPreview creates a preview string with colors applied
func (ah *ANSIHelper) BuildColoredPreview(text string) string {
	if text == "" {
		return ""
	}

	// Render the text with colors
	rendered := ah.RenderWithColors(text)
	
	// Add reset at the end to prevent color bleeding
	return rendered + "\x1b[0m"
}

// GetColorPalette returns a formatted color palette for display
func (ah *ANSIHelper) GetColorPalette() string {
	var palette strings.Builder
	
	// Standard colors
	palette.WriteString("Standard Colors:\n")
	for i := 0; i < 8; i++ {
		code := fmt.Sprintf("|%02d", i)
		if info, exists := ah.colorMap[code]; exists {
			coloredText := ah.RenderWithColors(code + info.Preview)
			palette.WriteString(fmt.Sprintf("%s %-10s ", coloredText, info.Name))
		}
	}
	palette.WriteString("\x1b[0m\n\n")
	
	// Bright colors
	palette.WriteString("Bright Colors:\n")
	for i := 8; i < 16; i++ {
		code := fmt.Sprintf("|%02d", i)
		if info, exists := ah.colorMap[code]; exists {
			coloredText := ah.RenderWithColors(code + info.Preview)
			palette.WriteString(fmt.Sprintf("%s %-10s ", coloredText, info.Name))
		}
	}
	palette.WriteString("\x1b[0m\n\n")
	
	// Background colors
	palette.WriteString("Background Colors:\n")
	for i := 0; i < 8; i++ {
		code := fmt.Sprintf("|B%d", i)
		if _, exists := ah.colorMap[code]; exists {
			coloredText := ah.RenderWithColors("|15" + code + "BG" + "|B0")
			palette.WriteString(fmt.Sprintf("%s ", coloredText))
		}
	}
	palette.WriteString("\x1b[0m\n")
	
	return palette.String()
}

// SuggestColors suggests appropriate colors based on text content
func (ah *ANSIHelper) SuggestColors(text string) []string {
	text = strings.ToLower(text)
	var suggestions []string
	
	// Analyze text content and suggest appropriate colors
	if strings.Contains(text, "error") || strings.Contains(text, "fail") {
		suggestions = append(suggestions, "|09") // Bright red
	}
	if strings.Contains(text, "success") || strings.Contains(text, "ok") {
		suggestions = append(suggestions, "|10") // Bright green
	}
	if strings.Contains(text, "warning") || strings.Contains(text, "warn") {
		suggestions = append(suggestions, "|11") // Yellow
	}
	if strings.Contains(text, "info") || strings.Contains(text, "note") {
		suggestions = append(suggestions, "|14") // Bright cyan
	}
	if strings.Contains(text, "prompt") || strings.Contains(text, "enter") {
		suggestions = append(suggestions, "|15") // White
	}
	if strings.Contains(text, "title") || strings.Contains(text, "header") {
		suggestions = append(suggestions, "|11") // Yellow
	}
	
	// Default suggestions if no specific matches
	if len(suggestions) == 0 {
		suggestions = []string{"|07", "|15", "|11", "|14"} // Gray, White, Yellow, Cyan
	}
	
	return suggestions
}

// ColorCodeEditor represents a color code editor state
type ColorCodeEditor struct {
	CurrentCode   string
	CursorPos     int
	ValidCodes    []string
	PreviewText   string
	IsEditing     bool
}

// NewColorCodeEditor creates a new color code editor
func NewColorCodeEditor() *ColorCodeEditor {
	helper := NewANSIHelper()
	var validCodes []string
	for code := range helper.colorMap {
		validCodes = append(validCodes, code)
	}
	
	return &ColorCodeEditor{
		CurrentCode: "|15",
		CursorPos:   0,
		ValidCodes:  validCodes,
		PreviewText: "Sample Text",
		IsEditing:   false,
	}
}

// SetPreviewText sets the text to preview with colors
func (cce *ColorCodeEditor) SetPreviewText(text string) {
	cce.PreviewText = text
}

// GetRenderedPreview returns the preview text with current color applied
func (cce *ColorCodeEditor) GetRenderedPreview() string {
	helper := NewANSIHelper()
	colored := cce.CurrentCode + cce.PreviewText
	return helper.BuildColoredPreview(colored)
}

// NextColor cycles to the next color code
func (cce *ColorCodeEditor) NextColor() {
	currentIdx := -1
	for i, code := range cce.ValidCodes {
		if code == cce.CurrentCode {
			currentIdx = i
			break
		}
	}
	
	if currentIdx == -1 {
		cce.CurrentCode = cce.ValidCodes[0]
	} else {
		nextIdx := (currentIdx + 1) % len(cce.ValidCodes)
		cce.CurrentCode = cce.ValidCodes[nextIdx]
	}
}

// PrevColor cycles to the previous color code
func (cce *ColorCodeEditor) PrevColor() {
	currentIdx := -1
	for i, code := range cce.ValidCodes {
		if code == cce.CurrentCode {
			currentIdx = i
			break
		}
	}
	
	if currentIdx == -1 {
		cce.CurrentCode = cce.ValidCodes[0]
	} else {
		prevIdx := currentIdx - 1
		if prevIdx < 0 {
			prevIdx = len(cce.ValidCodes) - 1
		}
		cce.CurrentCode = cce.ValidCodes[prevIdx]
	}
}

// SetColorByCode sets the current color by code
func (cce *ColorCodeEditor) SetColorByCode(code string) bool {
	for _, validCode := range cce.ValidCodes {
		if validCode == code {
			cce.CurrentCode = code
			return true
		}
	}
	return false
}

// GetColorDescription returns a description of the current color
func (cce *ColorCodeEditor) GetColorDescription() string {
	helper := NewANSIHelper()
	if info, exists := helper.GetColorInfo(cce.CurrentCode); exists {
		return info.Description
	}
	return "Unknown color"
}