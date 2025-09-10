package strings

import (
	"github.com/charmbracelet/lipgloss"
)

// TurboPascalTheme defines the classic Turbo Pascal IDE color scheme
type TurboPascalTheme struct {
	// Main colors
	Background       lipgloss.Color
	Foreground       lipgloss.Color
	MenuBackground   lipgloss.Color
	MenuForeground   lipgloss.Color
	MenuSelected     lipgloss.Color
	MenuSelectedText lipgloss.Color
	
	// Border and frame colors
	BorderNormal   lipgloss.Color
	BorderActive   lipgloss.Color
	BorderShadow   lipgloss.Color
	
	// Text colors
	TextNormal     lipgloss.Color
	TextHighlight  lipgloss.Color
	TextMuted      lipgloss.Color
	TextError      lipgloss.Color
	TextSuccess    lipgloss.Color
	
	// Editor colors
	EditorBackground lipgloss.Color
	EditorForeground lipgloss.Color
	EditorCursor     lipgloss.Color
	EditorSelection  lipgloss.Color
	
	// Status bar
	StatusBackground lipgloss.Color
	StatusForeground lipgloss.Color
}

// NewTurboPascalTheme creates the classic Turbo Pascal theme
func NewTurboPascalTheme() TurboPascalTheme {
	return TurboPascalTheme{
		// Classic blue background with white/yellow text
		Background:       lipgloss.Color("18"),   // Dark blue
		Foreground:       lipgloss.Color("15"),   // White
		MenuBackground:   lipgloss.Color("18"),   // Dark blue
		MenuForeground:   lipgloss.Color("15"),   // White
		MenuSelected:     lipgloss.Color("7"),    // Light gray
		MenuSelectedText: lipgloss.Color("0"),    // Black
		
		// Borders - classic single and double line style
		BorderNormal:   lipgloss.Color("8"),    // Dark gray
		BorderActive:   lipgloss.Color("11"),   // Yellow
		BorderShadow:   lipgloss.Color("0"),    // Black
		
		// Text variations
		TextNormal:     lipgloss.Color("15"),   // White
		TextHighlight:  lipgloss.Color("11"),   // Yellow
		TextMuted:      lipgloss.Color("8"),    // Dark gray
		TextError:      lipgloss.Color("9"),    // Red
		TextSuccess:    lipgloss.Color("10"),   // Green
		
		// Editor area
		EditorBackground: lipgloss.Color("18"),  // Dark blue
		EditorForeground: lipgloss.Color("15"),  // White
		EditorCursor:     lipgloss.Color("11"),  // Yellow
		EditorSelection:  lipgloss.Color("6"),   // Cyan
		
		// Status bar
		StatusBackground: lipgloss.Color("6"),   // Cyan
		StatusForeground: lipgloss.Color("0"),   // Black
	}
}

// StyleSet contains all the styles for the TUI
type StyleSet struct {
	Theme TurboPascalTheme
	
	// Window styles
	MainWindow    lipgloss.Style
	PaneWindow    lipgloss.Style
	ActivePane    lipgloss.Style
	InactivePane  lipgloss.Style
	Shadow        lipgloss.Style
	
	// Menu styles
	MenuBar       lipgloss.Style
	MenuItem      lipgloss.Style
	MenuSelected  lipgloss.Style
	MenuSeparator lipgloss.Style
	
	// List styles
	ListContainer lipgloss.Style
	ListItem      lipgloss.Style
	ListSelected  lipgloss.Style
	ListHeader    lipgloss.Style
	
	// Editor styles
	EditorPane    lipgloss.Style
	EditorText    lipgloss.Style
	EditorCursor  lipgloss.Style
	
	// Dialog styles
	DialogBox     lipgloss.Style
	DialogTitle   lipgloss.Style
	DialogContent lipgloss.Style
	DialogButton  lipgloss.Style
	DialogButtonSelected lipgloss.Style
	
	// Status styles
	StatusBar     lipgloss.Style
	StatusKey     lipgloss.Style
	StatusValue   lipgloss.Style
	
	// Preview styles
	PreviewPane   lipgloss.Style
	PreviewText   lipgloss.Style
	ANSIPreview   lipgloss.Style
	
	// Search styles
	SearchBox     lipgloss.Style
	SearchHighlight lipgloss.Style
	
	// Text utility styles
	TextMuted     lipgloss.Style
	TextHighlight lipgloss.Style
}

// NewStyleSet creates a complete style set with Turbo Pascal theme
func NewStyleSet() *StyleSet {
	theme := NewTurboPascalTheme()
	
	return &StyleSet{
		Theme: theme,
		
		// Main window - full screen background
		MainWindow: lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.Foreground).
			Width(80).
			Height(25),
			
		// Pane windows with borders
		PaneWindow: lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.Foreground).
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.BorderNormal),
			
		// Active pane - highlighted border
		ActivePane: lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.Foreground).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(theme.BorderActive),
			
		// Inactive pane - dimmed border
		InactivePane: lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.TextMuted).
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.BorderNormal),
			
		// Drop shadow effect
		Shadow: lipgloss.NewStyle().
			Background(theme.BorderShadow).
			Foreground(theme.BorderShadow),
			
		// Menu bar at top
		MenuBar: lipgloss.NewStyle().
			Background(theme.MenuBackground).
			Foreground(theme.MenuForeground).
			Width(80).
			Height(1).
			Align(lipgloss.Left),
			
		// Individual menu items
		MenuItem: lipgloss.NewStyle().
			Background(theme.MenuBackground).
			Foreground(theme.MenuForeground).
			Padding(0, 1),
			
		// Selected menu item
		MenuSelected: lipgloss.NewStyle().
			Background(theme.MenuSelected).
			Foreground(theme.MenuSelectedText).
			Padding(0, 1).
			Bold(true),
			
		// Menu separator
		MenuSeparator: lipgloss.NewStyle().
			Foreground(theme.BorderNormal).
			SetString("│"),
			
		// List container
		ListContainer: lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.Foreground).
			Padding(1).
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.BorderNormal),
			
		// List items
		ListItem: lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.Foreground).
			Padding(0, 1),
			
		// Selected list item
		ListSelected: lipgloss.NewStyle().
			Background(theme.MenuSelected).
			Foreground(theme.MenuSelectedText).
			Padding(0, 1).
			Bold(true),
			
		// List header
		ListHeader: lipgloss.NewStyle().
			Background(theme.MenuBackground).
			Foreground(theme.TextHighlight).
			Padding(0, 1).
			Bold(true).
			Underline(true),
			
		// Editor pane
		EditorPane: lipgloss.NewStyle().
			Background(theme.EditorBackground).
			Foreground(theme.EditorForeground).
			Padding(1).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(theme.BorderActive),
			
		// Editor text
		EditorText: lipgloss.NewStyle().
			Background(theme.EditorBackground).
			Foreground(theme.EditorForeground),
			
		// Editor cursor
		EditorCursor: lipgloss.NewStyle().
			Background(theme.EditorCursor).
			Foreground(theme.Background),
			
		// Dialog box
		DialogBox: lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.Foreground).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(theme.BorderActive).
			Padding(1).
			Align(lipgloss.Center),
			
		// Dialog title
		DialogTitle: lipgloss.NewStyle().
			Background(theme.MenuBackground).
			Foreground(theme.TextHighlight).
			Padding(0, 1).
			Bold(true).
			Align(lipgloss.Center),
			
		// Dialog content
		DialogContent: lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.Foreground).
			Padding(1).
			Align(lipgloss.Left),
			
		// Dialog button
		DialogButton: lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.Foreground).
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.BorderNormal).
			Padding(0, 2).
			Margin(0, 1),
			
		// Selected dialog button
		DialogButtonSelected: lipgloss.NewStyle().
			Background(theme.MenuSelected).
			Foreground(theme.MenuSelectedText).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(theme.BorderActive).
			Padding(0, 2).
			Margin(0, 1).
			Bold(true),
			
		// Status bar
		StatusBar: lipgloss.NewStyle().
			Background(theme.StatusBackground).
			Foreground(theme.StatusForeground).
			Width(80).
			Height(1).
			Align(lipgloss.Left),
			
		// Status bar key
		StatusKey: lipgloss.NewStyle().
			Background(theme.StatusBackground).
			Foreground(theme.StatusForeground).
			Bold(true),
			
		// Status bar value
		StatusValue: lipgloss.NewStyle().
			Background(theme.StatusBackground).
			Foreground(theme.StatusForeground),
			
		// Preview pane
		PreviewPane: lipgloss.NewStyle().
			Background(theme.EditorBackground).
			Foreground(theme.EditorForeground).
			Padding(1).
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.BorderNormal),
			
		// Preview text
		PreviewText: lipgloss.NewStyle().
			Background(theme.EditorBackground).
			Foreground(theme.EditorForeground),
			
		// ANSI preview (no background to show actual colors)
		ANSIPreview: lipgloss.NewStyle(),
		
		// Search box
		SearchBox: lipgloss.NewStyle().
			Background(theme.EditorBackground).
			Foreground(theme.EditorForeground).
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.BorderActive).
			Padding(0, 1),
			
		// Search highlight
		SearchHighlight: lipgloss.NewStyle().
			Background(theme.TextHighlight).
			Foreground(theme.Background).
			Bold(true),
			
		// Text utility styles
		TextMuted: lipgloss.NewStyle().
			Foreground(theme.TextMuted),
			
		TextHighlight: lipgloss.NewStyle().
			Foreground(theme.TextHighlight).
			Bold(true),
	}
}

// BoxChars contains the classic IBM PC box drawing characters
type BoxChars struct {
	Horizontal     string
	Vertical       string
	TopLeft        string
	TopRight       string
	BottomLeft     string
	BottomRight    string
	Cross          string
	TopTee         string
	BottomTee      string
	LeftTee        string
	RightTee       string
	
	// Double line variants
	DoubleHorizontal     string
	DoubleVertical       string
	DoubleTopLeft        string
	DoubleTopRight       string
	DoubleBottomLeft     string
	DoubleBottomRight    string
	DoubleCross          string
	DoubleTopTee         string
	DoubleBottomTee      string
	DoubleLeftTee        string
	DoubleRightTee       string
	
	// Mixed variants (single/double combinations)
	MixedTopLeft         string
	MixedTopRight        string
	MixedBottomLeft      string
	MixedBottomRight     string
}

// NewBoxChars returns the IBM PC box drawing character set
func NewBoxChars() BoxChars {
	return BoxChars{
		// Single line
		Horizontal:     "─",
		Vertical:       "│",
		TopLeft:        "┌",
		TopRight:       "┐",
		BottomLeft:     "└",
		BottomRight:    "┘",
		Cross:          "┼",
		TopTee:         "┬",
		BottomTee:      "┴",
		LeftTee:        "├",
		RightTee:       "┤",
		
		// Double line
		DoubleHorizontal:     "═",
		DoubleVertical:       "║",
		DoubleTopLeft:        "╔",
		DoubleTopRight:       "╗",
		DoubleBottomLeft:     "╚",
		DoubleBottomRight:    "╝",
		DoubleCross:          "╬",
		DoubleTopTee:         "╦",
		DoubleBottomTee:      "╩",
		DoubleLeftTee:        "╠",
		DoubleRightTee:       "╣",
		
		// Mixed
		MixedTopLeft:         "╓",
		MixedTopRight:        "╖",
		MixedBottomLeft:     "╙",
		MixedBottomRight:    "╜",
	}
}

// CreateWindow creates a styled window with title
func (s *StyleSet) CreateWindow(title string, width, height int, active bool) lipgloss.Style {
	var baseStyle lipgloss.Style
	if active {
		baseStyle = s.ActivePane
	} else {
		baseStyle = s.InactivePane
	}
	
	return baseStyle.
		Width(width).
		Height(height).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true)
}

// CreateDialog creates a centered dialog box
func (s *StyleSet) CreateDialog(title string, width, height int) lipgloss.Style {
	return s.DialogBox.
		Width(width).
		Height(height).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true)
}

// FormatMenuBar creates a formatted menu bar
func (s *StyleSet) FormatMenuBar(items []string, selected int) string {
	var menuItems []string
	
	for i, item := range items {
		if i == selected {
			menuItems = append(menuItems, s.MenuSelected.Render(item))
		} else {
			menuItems = append(menuItems, s.MenuItem.Render(item))
		}
		
		// Add separator between items (except last)
		if i < len(items)-1 {
			menuItems = append(menuItems, s.MenuSeparator.Render())
		}
	}
	
	return s.MenuBar.Render(lipgloss.JoinHorizontal(lipgloss.Top, menuItems...))
}

// FormatStatusBar creates a formatted status bar
func (s *StyleSet) FormatStatusBar(items map[string]string) string {
	var statusItems []string
	
	for key, value := range items {
		keyPart := s.StatusKey.Render(key + ":")
		valuePart := s.StatusValue.Render(value)
		statusItems = append(statusItems, keyPart+" "+valuePart)
	}
	
	statusText := lipgloss.JoinHorizontal(lipgloss.Top, statusItems...)
	return s.StatusBar.Render(statusText)
}

// CreateShadowedBox creates a box with drop shadow effect
func (s *StyleSet) CreateShadowedBox(content string, width, height int) string {
	// Create the main box
	box := s.DialogBox.
		Width(width).
		Height(height).
		Render(content)
		
	// Create shadow by adding dark background to the right and bottom
	shadowWidth := 2
	shadowHeight := 1
	
	// This is a simplified shadow - in a real implementation you'd want
	// to properly layer the shadow behind the box
	shadow := s.Shadow.
		Width(shadowWidth).
		Height(height + shadowHeight).
		Render("")
		
	return lipgloss.JoinHorizontal(lipgloss.Top, box, shadow)
}