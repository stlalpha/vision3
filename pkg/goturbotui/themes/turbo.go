package themes

import (
	"github.com/stlalpha/vision3/pkg/goturbotui"
)

// TurboTheme provides the classic Turbo Pascal IDE color scheme
type TurboTheme struct {
	// Desktop/Background
	Desktop goturbotui.Style
	
	// Menu bar
	MenuBar         goturbotui.Style
	MenuBarSelected goturbotui.Style
	
	// Status bar
	StatusBar goturbotui.Style
	
	// Dialog boxes
	DialogFrame     goturbotui.Style
	DialogText      goturbotui.Style
	DialogSelected  goturbotui.Style
	DialogShadow    goturbotui.Style
	
	// Buttons
	Button         goturbotui.Style
	ButtonSelected goturbotui.Style
	ButtonFocused  goturbotui.Style
	
	// Input fields
	Input         goturbotui.Style
	InputFocused  goturbotui.Style
	InputSelected goturbotui.Style
	
	// List boxes
	ListBox         goturbotui.Style
	ListBoxSelected goturbotui.Style
	ListBoxFocused  goturbotui.Style
}

// NewTurboTheme creates the classic Turbo Pascal color scheme
func NewTurboTheme() *TurboTheme {
	return &TurboTheme{
		// Desktop - Blue background with light gray pattern
		Desktop: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorGray).
			WithBackground(goturbotui.ColorBlue),
		
		// Menu bar - Cyan background with black text
		MenuBar: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorBlack).
			WithBackground(goturbotui.ColorCyan),
		MenuBarSelected: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorWhite).
			WithBackground(goturbotui.ColorBlack),
		
		// Status bar - White background with black text
		StatusBar: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorBlack).
			WithBackground(goturbotui.ColorWhite),
		
		// Dialog boxes - Gray background with black text
		DialogFrame: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorBlack).
			WithBackground(goturbotui.ColorGray),
		DialogText: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorBlack).
			WithBackground(goturbotui.ColorGray),
		DialogSelected: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorBlack).
			WithBackground(goturbotui.ColorWhite),
		DialogShadow: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorBlack).
			WithBackground(goturbotui.ColorDarkGray),
		
		// Buttons - Gray background with black text
		Button: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorBlack).
			WithBackground(goturbotui.ColorGray),
		ButtonSelected: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorWhite).
			WithBackground(goturbotui.ColorBlack),
		ButtonFocused: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorBlack).
			WithBackground(goturbotui.ColorWhite),
		
		// Input fields - White background with black text
		Input: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorBlack).
			WithBackground(goturbotui.ColorWhite),
		InputFocused: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorBlack).
			WithBackground(goturbotui.ColorWhite).
			WithAttributes(goturbotui.AttrBold),
		InputSelected: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorWhite).
			WithBackground(goturbotui.ColorBlack),
		
		// List boxes - White background with black text
		ListBox: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorBlack).
			WithBackground(goturbotui.ColorWhite),
		ListBoxSelected: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorWhite).
			WithBackground(goturbotui.ColorBlack),
		ListBoxFocused: goturbotui.NewStyle().
			WithForeground(goturbotui.ColorBlack).
			WithBackground(goturbotui.ColorWhite).
			WithAttributes(goturbotui.AttrBold),
	}
}