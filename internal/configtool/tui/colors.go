package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Turbo Pascal IDE Color Scheme
// Based on the classic Turbo Pascal 7.0 IDE colors

var (
	// Primary colors matching Turbo Pascal IDE
	ColorBackground    = lipgloss.Color("4")   // Blue background
	ColorText          = lipgloss.Color("7")   // White text
	ColorHighlight     = lipgloss.Color("14")  // Yellow highlights
	ColorMenuBar       = lipgloss.Color("6")   // Cyan menu bar
	ColorMenuText      = lipgloss.Color("0")   // Black text on cyan
	ColorStatusBar     = lipgloss.Color("6")   // Cyan status bar
	ColorSelected      = lipgloss.Color("15")  // Bright white for selected items
	ColorDisabled      = lipgloss.Color("8")   // Dark gray for disabled items
	ColorBorder        = lipgloss.Color("15")  // Bright white for borders
	ColorShadow        = lipgloss.Color("0")   // Black for shadows
	ColorError         = lipgloss.Color("12")  // Bright red for errors
	ColorSuccess       = lipgloss.Color("10")  // Bright green for success
	ColorWarning       = lipgloss.Color("14")  // Yellow for warnings
	ColorDialog        = lipgloss.Color("7")   // Light gray for dialog backgrounds
	ColorDialogBorder  = lipgloss.Color("15")  // White for dialog borders
	ColorButton        = lipgloss.Color("7")   // Light gray for buttons
	ColorButtonActive  = lipgloss.Color("0")   // Black for active button text
	ColorListSelected  = lipgloss.Color("14")  // Yellow background for selected list items
)

// Style definitions for common UI elements
var (
	// Base styles
	BaseStyle = lipgloss.NewStyle().
		Background(ColorBackground).
		Foreground(ColorText)
	
	// Menu bar styles
	MenuBarStyle = lipgloss.NewStyle().
		Background(ColorMenuBar).
		Foreground(ColorMenuText).
		Bold(true)
	
	MenuItemStyle = lipgloss.NewStyle().
		Background(ColorMenuBar).
		Foreground(ColorMenuText).
		Padding(0, 1)
	
	MenuItemActiveStyle = lipgloss.NewStyle().
		Background(ColorText).
		Foreground(ColorMenuBar).
		Padding(0, 1).
		Bold(true)
	
	// Status bar styles
	StatusBarStyle = lipgloss.NewStyle().
		Background(ColorStatusBar).
		Foreground(ColorMenuText).
		Bold(true)
	
	// Window and dialog styles
	WindowStyle = lipgloss.NewStyle().
		Background(ColorDialog).
		Foreground(ColorText).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorDialogBorder)
	
	DialogStyle = lipgloss.NewStyle().
		Background(ColorDialog).
		Foreground(ColorText).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(ColorDialogBorder).
		Padding(1, 2)
	
	// Shadow style for windows
	ShadowStyle = lipgloss.NewStyle().
		Background(ColorShadow).
		Foreground(ColorShadow)
	
	// Button styles
	ButtonStyle = lipgloss.NewStyle().
		Background(ColorButton).
		Foreground(ColorText).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 2).
		Margin(0, 1)
	
	ButtonActiveStyle = lipgloss.NewStyle().
		Background(ColorHighlight).
		Foreground(ColorButtonActive).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 2).
		Margin(0, 1).
		Bold(true)
	
	// Input field styles
	InputStyle = lipgloss.NewStyle().
		Background(ColorText).
		Foreground(ColorBackground).
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)
	
	InputFocusStyle = lipgloss.NewStyle().
		Background(ColorText).
		Foreground(ColorBackground).
		Border(lipgloss.ThickBorder()).
		BorderForeground(ColorHighlight).
		Padding(0, 1)
	
	// List box styles
	ListBoxStyle = lipgloss.NewStyle().
		Background(ColorText).
		Foreground(ColorBackground).
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder)
	
	ListItemStyle = lipgloss.NewStyle().
		Background(ColorText).
		Foreground(ColorBackground).
		Padding(0, 1)
	
	ListItemSelectedStyle = lipgloss.NewStyle().
		Background(ColorListSelected).
		Foreground(ColorBackground).
		Padding(0, 1).
		Bold(true)
	
	// Text styles
	TitleStyle = lipgloss.NewStyle().
		Foreground(ColorHighlight).
		Bold(true).
		Align(lipgloss.Center)
	
	ErrorStyle = lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true)
	
	SuccessStyle = lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true)
	
	WarningStyle = lipgloss.NewStyle().
		Foreground(ColorWarning).
		Bold(true)
	
	HelpStyle = lipgloss.NewStyle().
		Foreground(ColorText).
		Italic(true)
)

// Box drawing characters for authentic Turbo Pascal look
const (
	// Single line box drawing
	BoxTopLeft     = "┌"
	BoxTopRight    = "┐"
	BoxBottomLeft  = "└"
	BoxBottomRight = "┘"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
	BoxCross       = "┼"
	BoxTeeUp       = "┴"
	BoxTeeDown     = "┬"
	BoxTeeLeft     = "┤"
	BoxTeeRight    = "├"
	
	// Double line box drawing
	BoxTopLeftDouble     = "╔"
	BoxTopRightDouble    = "╗"
	BoxBottomLeftDouble  = "╚"
	BoxBottomRightDouble = "╝"
	BoxHorizontalDouble  = "═"
	BoxVerticalDouble    = "║"
	BoxCrossDouble       = "╬"
	BoxTeeUpDouble       = "╩"
	BoxTeeDownDouble     = "╦"
	BoxTeeLeftDouble     = "╣"
	BoxTeeRightDouble    = "╠"
	
	// Shadow characters
	ShadowChar     = "▓"
	ShadowCharMed  = "▒"
	ShadowCharLight = "░"
	
	// Special characters
	CheckMark      = "√"
	CrossMark      = "×"
	Arrow          = "►"
	Bullet         = "•"
	Diamond        = "◆"
)

// CreateBox creates a box with the specified style and content
func CreateBox(width, height int, title string, content string, doubled bool) string {
	var topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical string
	
	if doubled {
		topLeft = BoxTopLeftDouble
		topRight = BoxTopRightDouble
		bottomLeft = BoxBottomLeftDouble
		bottomRight = BoxBottomRightDouble
		horizontal = BoxHorizontalDouble
		vertical = BoxVerticalDouble
	} else {
		topLeft = BoxTopLeft
		topRight = BoxTopRight
		bottomLeft = BoxBottomLeft
		bottomRight = BoxBottomRight
		horizontal = BoxHorizontal
		vertical = BoxVertical
	}
	
	// Build the box
	var lines []string
	
	// Top border with title
	topBorder := topLeft
	if title != "" {
		titleLen := lipgloss.Width(title)
		padding := (width - titleLen - 4) / 2
		if padding < 0 {
			padding = 0
		}
		topBorder += repeatString(horizontal, padding)
		topBorder += " " + title + " "
		topBorder += repeatString(horizontal, width-padding-titleLen-4)
	} else {
		topBorder += repeatString(horizontal, width-2)
	}
	topBorder += topRight
	lines = append(lines, topBorder)
	
	// Content lines
	contentLines := lipgloss.Height(content)
	maxContentHeight := height - 2
	
	if contentLines > maxContentHeight {
		contentLines = maxContentHeight
	}
	
	// Add content with vertical borders
	for i := 0; i < contentLines; i++ {
		line := vertical + repeatString(" ", width-2) + vertical
		lines = append(lines, line)
	}
	
	// Fill remaining space
	for i := contentLines; i < maxContentHeight; i++ {
		line := vertical + repeatString(" ", width-2) + vertical
		lines = append(lines, line)
	}
	
	// Bottom border
	bottomBorder := bottomLeft + repeatString(horizontal, width-2) + bottomRight
	lines = append(lines, bottomBorder)
	
	return lipgloss.JoinVertical(lipgloss.Top, lines...)
}

// repeatString repeats a string n times
func repeatString(s string, n int) string {
	if n <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

// CenterText centers text within specified dimensions
func CenterText(width, height int, text string) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, text)
}

// CreateShadow creates a shadow effect for windows
func CreateShadow(width, height int) string {
	var lines []string
	for i := 0; i < height; i++ {
		lines = append(lines, repeatString(ShadowChar, width))
	}
	return ShadowStyle.Render(lipgloss.JoinVertical(lipgloss.Top, lines...))
}