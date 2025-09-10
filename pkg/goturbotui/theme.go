package goturbotui

// Theme defines the color scheme for the TUI
type Theme struct {
	// Desktop/Background
	Desktop Style
	
	// Menu bar
	MenuBar         Style
	MenuBarSelected Style
	
	// Status bar  
	StatusBar Style
	
	// Dialog boxes
	DialogFrame     Style
	DialogText      Style
	DialogSelected  Style
	DialogShadow    Style
	
	// Buttons
	Button         Style
	ButtonSelected Style
	ButtonFocused  Style
	
	// Input fields
	Input         Style
	InputFocused  Style
	InputSelected Style
	
	// List boxes
	ListBox         Style
	ListBoxSelected Style
	ListBoxFocused  Style
}

// DefaultTurboTheme returns the classic Turbo Pascal theme
func DefaultTurboTheme() *Theme {
	return &Theme{
		// Desktop - Blue background with light gray pattern
		Desktop: NewStyle().
			WithForeground(ColorGray).
			WithBackground(ColorBlue),
		
		// Menu bar - Cyan background with black text
		MenuBar: NewStyle().
			WithForeground(ColorBlack).
			WithBackground(ColorCyan),
		MenuBarSelected: NewStyle().
			WithForeground(ColorWhite).
			WithBackground(ColorBlack),
		
		// Status bar - White background with black text
		StatusBar: NewStyle().
			WithForeground(ColorBlack).
			WithBackground(ColorWhite),
		
		// Dialog boxes - Gray background with black text
		DialogFrame: NewStyle().
			WithForeground(ColorBlack).
			WithBackground(ColorGray),
		DialogText: NewStyle().
			WithForeground(ColorBlack).
			WithBackground(ColorGray),
		DialogSelected: NewStyle().
			WithForeground(ColorBlack).
			WithBackground(ColorWhite),
		DialogShadow: NewStyle().
			WithForeground(ColorBlack).
			WithBackground(ColorDarkGray),
		
		// Buttons - Gray background with black text
		Button: NewStyle().
			WithForeground(ColorBlack).
			WithBackground(ColorGray),
		ButtonSelected: NewStyle().
			WithForeground(ColorWhite).
			WithBackground(ColorBlack),
		ButtonFocused: NewStyle().
			WithForeground(ColorBlack).
			WithBackground(ColorWhite),
		
		// Input fields - White background with black text
		Input: NewStyle().
			WithForeground(ColorBlack).
			WithBackground(ColorWhite),
		InputFocused: NewStyle().
			WithForeground(ColorBlack).
			WithBackground(ColorWhite).
			WithAttributes(AttrBold),
		InputSelected: NewStyle().
			WithForeground(ColorWhite).
			WithBackground(ColorBlack),
		
		// List boxes - White background with black text  
		ListBox: NewStyle().
			WithForeground(ColorBlack).
			WithBackground(ColorWhite),
		ListBoxSelected: NewStyle().
			WithForeground(ColorWhite).
			WithBackground(ColorBlack),
		ListBoxFocused: NewStyle().
			WithForeground(ColorBlack).
			WithBackground(ColorWhite).
			WithAttributes(AttrBold),
	}
}