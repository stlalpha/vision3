package goturbotui

import "fmt"

// Color represents a terminal color
type Color int

// Standard 16 colors
const (
	ColorBlack Color = iota
	ColorDarkRed
	ColorDarkGreen
	ColorDarkYellow
	ColorDarkBlue
	ColorDarkMagenta
	ColorDarkCyan
	ColorGray
	ColorDarkGray
	ColorRed
	ColorGreen
	ColorYellow
	ColorBlue
	ColorMagenta
	ColorCyan
	ColorWhite
)

// Attribute represents text attributes
type Attribute int

const (
	AttrNone Attribute = 0
	AttrBold Attribute = 1 << iota
	AttrDim
	AttrItalic
	AttrUnderline
	AttrBlink
	AttrReverse
	AttrStrikethrough
)

// Style represents text styling with foreground, background, and attributes
type Style struct {
	Foreground Color
	Background Color
	Attributes Attribute
}

// NewStyle creates a new style with default values
func NewStyle() Style {
	return Style{
		Foreground: ColorWhite,
		Background: ColorBlack,
		Attributes: AttrNone,
	}
}

// WithForeground returns a new style with the specified foreground color
func (s Style) WithForeground(color Color) Style {
	s.Foreground = color
	return s
}

// WithBackground returns a new style with the specified background color
func (s Style) WithBackground(color Color) Style {
	s.Background = color
	return s
}

// WithAttributes returns a new style with the specified attributes
func (s Style) WithAttributes(attr Attribute) Style {
	s.Attributes = attr
	return s
}

// ToANSI converts the style to ANSI escape sequence
func (s Style) ToANSI() string {
	var codes []string
	
	// Reset
	codes = append(codes, "0")
	
	// Foreground color
	if s.Foreground < 8 {
		codes = append(codes, fmt.Sprintf("3%d", int(s.Foreground)))
	} else {
		codes = append(codes, fmt.Sprintf("9%d", int(s.Foreground)-8))
	}
	
	// Background color
	if s.Background < 8 {
		codes = append(codes, fmt.Sprintf("4%d", int(s.Background)))
	} else {
		codes = append(codes, fmt.Sprintf("10%d", int(s.Background)-8))
	}
	
	// Attributes
	if s.Attributes&AttrBold != 0 {
		codes = append(codes, "1")
	}
	if s.Attributes&AttrDim != 0 {
		codes = append(codes, "2")
	}
	if s.Attributes&AttrItalic != 0 {
		codes = append(codes, "3")
	}
	if s.Attributes&AttrUnderline != 0 {
		codes = append(codes, "4")
	}
	if s.Attributes&AttrBlink != 0 {
		codes = append(codes, "5")
	}
	if s.Attributes&AttrReverse != 0 {
		codes = append(codes, "7")
	}
	if s.Attributes&AttrStrikethrough != 0 {
		codes = append(codes, "9")
	}
	
	result := "\033["
	for i, code := range codes {
		if i > 0 {
			result += ";"
		}
		result += code
	}
	result += "m"
	
	return result
}

// Reset returns the ANSI reset sequence
func Reset() string {
	return "\033[0m"
}