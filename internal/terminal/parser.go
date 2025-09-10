package terminal

import (
	"fmt"
	"strconv"
	"strings"
)

// ParserState represents the current state of the ANSI parser
type ParserState int

const (
	StateGround ParserState = iota // Normal character processing
	StateEscape                    // After ESC character
	StateCSI                       // Control Sequence Introducer (ESC[)
	StateOSC                       // Operating System Command (ESC])
	StateDCS                       // Device Control String (ESCP)
	StateString                    // String processing state
	StateParam                     // Parameter collection state
)

// GraphicsState tracks current text attributes
type GraphicsState struct {
	ForegroundColor int  // Current foreground color (0-255)
	BackgroundColor int  // Current background color (0-255)
	Bold            bool // Bold/bright text
	Dim             bool // Dim text
	Italic          bool // Italic text
	Underline       bool // Underlined text
	Blink           bool // Blinking text
	Reverse         bool // Reverse video
	Strikethrough   bool // Strikethrough text
	DoubleUnderline bool // Double underline
}

// CursorState tracks cursor position and visibility
type CursorState struct {
	X         int  // Current column (0-based)
	Y         int  // Current row (0-based)
	SavedX    int  // Saved cursor X position
	SavedY    int  // Saved cursor Y position
	Visible   bool // Cursor visibility
	WrapMode  bool // Automatic line wrapping
	OriginMode bool // Origin mode (relative to scrolling region)
}

// ScreenState maintains the terminal screen state
type ScreenState struct {
	Width          int // Screen width in characters
	Height         int // Screen height in characters
	ScrollTop      int // Top of scrolling region
	ScrollBottom   int // Bottom of scrolling region
	TabStops       map[int]bool // Tab stop positions
	CharacterSet   int // Current character set (G0, G1, etc.)
	ApplicationMode bool // Application keypad mode
}

// ANSIParser implements a comprehensive ANSI escape sequence parser
type ANSIParser struct {
	state           ParserState
	params          []int
	paramBuffer     strings.Builder
	intermediates   strings.Builder
	finalByte       byte
	
	// Terminal state
	graphics        GraphicsState
	cursor          CursorState
	screen          ScreenState
	
	// Parsing state
	collectingParams bool
	private          bool // CSI sequence started with ? or similar
	
	// Callbacks for handling parsed sequences
	onText          func([]byte)
	onCursor        func(x, y int)
	onGraphics      func(GraphicsState)
	onClear         func(mode int)
	onScroll        func(direction int, amount int)
}

// NewANSIParser creates a new ANSI parser with default callbacks
func NewANSIParser(width, height int) *ANSIParser {
	parser := &ANSIParser{
		state: StateGround,
		graphics: GraphicsState{
			ForegroundColor: 7, // Default white foreground
			BackgroundColor: 0, // Default black background
		},
		cursor: CursorState{
			Visible:  true,
			WrapMode: true,
		},
		screen: ScreenState{
			Width:        width,
			Height:       height,
			ScrollBottom: height - 1,
			TabStops:     make(map[int]bool),
		},
	}
	
	// Set default tab stops (every 8 characters)
	for i := 8; i < width; i += 8 {
		parser.screen.TabStops[i] = true
	}
	
	return parser
}

// SetCallbacks sets the callback functions for parser events
func (p *ANSIParser) SetCallbacks(
	onText func([]byte),
	onCursor func(x, y int),
	onGraphics func(GraphicsState),
	onClear func(mode int),
	onScroll func(direction int, amount int),
) {
	p.onText = onText
	p.onCursor = onCursor
	p.onGraphics = onGraphics
	p.onClear = onClear
	p.onScroll = onScroll
}

// ParseBytes processes a buffer of bytes through the ANSI state machine
func (p *ANSIParser) ParseBytes(data []byte) error {
	for _, b := range data {
		if err := p.parseByte(b); err != nil {
			return err
		}
	}
	return nil
}

// parseByte processes a single byte through the state machine
func (p *ANSIParser) parseByte(b byte) error {
	switch p.state {
	case StateGround:
		return p.parseGround(b)
	case StateEscape:
		return p.parseEscape(b)
	case StateCSI:
		return p.parseCSI(b)
	case StateOSC:
		return p.parseOSC(b)
	case StateDCS:
		return p.parseDCS(b)
	case StateParam:
		return p.parseParam(b)
	}
	return nil
}

// parseGround handles normal character processing
func (p *ANSIParser) parseGround(b byte) error {
	switch b {
	case 0x1B: // ESC
		p.state = StateEscape
		p.resetParser()
	case 0x08: // Backspace
		if p.cursor.X > 0 {
			p.cursor.X--
			if p.onCursor != nil {
				p.onCursor(p.cursor.X, p.cursor.Y)
			}
		}
	case 0x09: // Tab
		p.handleTab()
	case 0x0A: // Line Feed
		p.handleLineFeed()
	case 0x0D: // Carriage Return
		p.cursor.X = 0
		if p.onCursor != nil {
			p.onCursor(p.cursor.X, p.cursor.Y)
		}
	case 0x07: // Bell
		// Handle bell (could trigger callback for audio/visual bell)
	default:
		if b >= 0x20 && b <= 0x7E { // Printable ASCII
			if p.onText != nil {
				p.onText([]byte{b})
			}
			p.advanceCursor()
		} else if b >= 0x80 { // Extended ASCII/UTF-8
			if p.onText != nil {
				p.onText([]byte{b})
			}
			p.advanceCursor()
		}
	}
	return nil
}

// parseEscape handles escape sequence processing
func (p *ANSIParser) parseEscape(b byte) error {
	switch b {
	case '[': // CSI - Control Sequence Introducer
		p.state = StateCSI
	case ']': // OSC - Operating System Command
		p.state = StateOSC
	case 'P': // DCS - Device Control String
		p.state = StateDCS
	case '7': // DECSC - Save cursor position
		p.cursor.SavedX = p.cursor.X
		p.cursor.SavedY = p.cursor.Y
		p.state = StateGround
	case '8': // DECRC - Restore cursor position
		p.cursor.X = p.cursor.SavedX
		p.cursor.Y = p.cursor.SavedY
		if p.onCursor != nil {
			p.onCursor(p.cursor.X, p.cursor.Y)
		}
		p.state = StateGround
	case 'c': // RIS - Reset to Initial State
		p.resetTerminal()
		p.state = StateGround
	case 'D': // IND - Index (move cursor down, scroll if necessary)
		p.handleLineFeed()
		p.state = StateGround
	case 'E': // NEL - Next Line
		p.cursor.X = 0
		p.handleLineFeed()
		p.state = StateGround
	case 'M': // RI - Reverse Index (move cursor up, scroll if necessary)
		if p.cursor.Y > p.screen.ScrollTop {
			p.cursor.Y--
		} else if p.onScroll != nil {
			p.onScroll(-1, 1) // Scroll up
		}
		if p.onCursor != nil {
			p.onCursor(p.cursor.X, p.cursor.Y)
		}
		p.state = StateGround
	default:
		// Unknown escape sequence, return to ground state
		p.state = StateGround
	}
	return nil
}

// parseCSI handles Control Sequence Introducer sequences
func (p *ANSIParser) parseCSI(b byte) error {
	if b >= 0x30 && b <= 0x3F { // Parameter bytes or private markers
		if b == '?' || b == '>' || b == '=' || b == '<' {
			p.private = true
		} else {
			p.paramBuffer.WriteByte(b)
		}
		return nil
	}
	
	if b >= 0x20 && b <= 0x2F { // Intermediate bytes
		p.intermediates.WriteByte(b)
		return nil
	}
	
	if b >= 0x40 && b <= 0x7E { // Final byte
		p.finalByte = b
		p.parseCSIParameters()
		p.executeCSISequence()
		p.state = StateGround
		return nil
	}
	
	return fmt.Errorf("invalid CSI sequence byte: %02X", b)
}

// parseOSC handles Operating System Command sequences
func (p *ANSIParser) parseOSC(b byte) error {
	if b == 0x07 || b == 0x1B { // Bell or ESC terminates OSC
		p.state = StateGround
	}
	// For now, we ignore OSC sequences
	return nil
}

// parseDCS handles Device Control String sequences
func (p *ANSIParser) parseDCS(b byte) error {
	if b == 0x1B { // ESC terminates DCS
		p.state = StateGround
	}
	// For now, we ignore DCS sequences
	return nil
}

// parseParam handles parameter parsing state
func (p *ANSIParser) parseParam(b byte) error {
	// This state is used for complex parameter parsing if needed
	p.state = StateGround
	return nil
}

// parseCSIParameters parses the parameter string into integers
func (p *ANSIParser) parseCSIParameters() {
	p.params = nil
	if p.paramBuffer.Len() == 0 {
		return
	}
	
	paramStr := p.paramBuffer.String()
	parts := strings.Split(paramStr, ";")
	
	for _, part := range parts {
		if part == "" {
			p.params = append(p.params, 0) // Default parameter
		} else {
			if num, err := strconv.Atoi(part); err == nil {
				p.params = append(p.params, num)
			} else {
				p.params = append(p.params, 0)
			}
		}
	}
}

// executeCSISequence executes the parsed CSI sequence
func (p *ANSIParser) executeCSISequence() {
	switch p.finalByte {
	case 'A': // CUU - Cursor Up
		amount := 1
		if len(p.params) > 0 && p.params[0] > 0 {
			amount = p.params[0]
		}
		p.cursor.Y = max(p.cursor.Y-amount, p.screen.ScrollTop)
		if p.onCursor != nil {
			p.onCursor(p.cursor.X, p.cursor.Y)
		}
		
	case 'B': // CUD - Cursor Down
		amount := 1
		if len(p.params) > 0 && p.params[0] > 0 {
			amount = p.params[0]
		}
		p.cursor.Y = min(p.cursor.Y+amount, p.screen.ScrollBottom)
		if p.onCursor != nil {
			p.onCursor(p.cursor.X, p.cursor.Y)
		}
		
	case 'C': // CUF - Cursor Forward
		amount := 1
		if len(p.params) > 0 && p.params[0] > 0 {
			amount = p.params[0]
		}
		p.cursor.X = min(p.cursor.X+amount, p.screen.Width-1)
		if p.onCursor != nil {
			p.onCursor(p.cursor.X, p.cursor.Y)
		}
		
	case 'D': // CUB - Cursor Backward
		amount := 1
		if len(p.params) > 0 && p.params[0] > 0 {
			amount = p.params[0]
		}
		p.cursor.X = max(p.cursor.X-amount, 0)
		if p.onCursor != nil {
			p.onCursor(p.cursor.X, p.cursor.Y)
		}
		
	case 'H', 'f': // CUP - Cursor Position
		row, col := 1, 1 // Default to top-left (1,1)
		if len(p.params) > 0 {
			row = max(p.params[0], 1)
		}
		if len(p.params) > 1 {
			col = max(p.params[1], 1)
		}
		p.cursor.Y = min(row-1, p.screen.Height-1) // Convert to 0-based
		p.cursor.X = min(col-1, p.screen.Width-1)
		if p.onCursor != nil {
			p.onCursor(p.cursor.X, p.cursor.Y)
		}
		
	case 'J': // ED - Erase Display
		mode := 0
		if len(p.params) > 0 {
			mode = p.params[0]
		}
		if p.onClear != nil {
			p.onClear(mode)
		}
		
	case 'K': // EL - Erase Line
		mode := 0
		if len(p.params) > 0 {
			mode = p.params[0]
		}
		// Handle line erasing (mode: 0=to end, 1=to beginning, 2=entire line)
		_ = mode // Suppress unused variable warning
		
	case 'm': // SGR - Select Graphic Rendition
		p.handleSGR()
		
	case 's': // DECSC - Save cursor (private)
		p.cursor.SavedX = p.cursor.X
		p.cursor.SavedY = p.cursor.Y
		
	case 'u': // DECRC - Restore cursor (private)
		p.cursor.X = p.cursor.SavedX
		p.cursor.Y = p.cursor.SavedY
		if p.onCursor != nil {
			p.onCursor(p.cursor.X, p.cursor.Y)
		}
		
	case 'r': // DECSTBM - Set scrolling region
		top, bottom := 1, p.screen.Height
		if len(p.params) > 0 {
			top = max(p.params[0], 1)
		}
		if len(p.params) > 1 {
			bottom = min(p.params[1], p.screen.Height)
		}
		p.screen.ScrollTop = top - 1    // Convert to 0-based
		p.screen.ScrollBottom = bottom - 1
		p.cursor.X = 0 // Move cursor to home position
		p.cursor.Y = p.screen.ScrollTop
		if p.onCursor != nil {
			p.onCursor(p.cursor.X, p.cursor.Y)
		}
	}
}

// handleSGR processes Select Graphic Rendition sequences
func (p *ANSIParser) handleSGR() {
	if len(p.params) == 0 {
		p.params = []int{0} // Default to reset
	}
	
	for i := 0; i < len(p.params); i++ {
		param := p.params[i]
		switch param {
		case 0: // Reset all attributes
			p.graphics = GraphicsState{
				ForegroundColor: 7, // Default white
				BackgroundColor: 0, // Default black
			}
		case 1: // Bold/bright
			p.graphics.Bold = true
		case 2: // Dim
			p.graphics.Dim = true
		case 3: // Italic
			p.graphics.Italic = true
		case 4: // Underline
			p.graphics.Underline = true
		case 5, 6: // Blink
			p.graphics.Blink = true
		case 7: // Reverse video
			p.graphics.Reverse = true
		case 9: // Strikethrough
			p.graphics.Strikethrough = true
		case 21: // Double underline
			p.graphics.DoubleUnderline = true
		case 22: // Normal intensity (not bold/dim)
			p.graphics.Bold = false
			p.graphics.Dim = false
		case 23: // Not italic
			p.graphics.Italic = false
		case 24: // Not underlined
			p.graphics.Underline = false
			p.graphics.DoubleUnderline = false
		case 25: // Not blinking
			p.graphics.Blink = false
		case 27: // Not reverse
			p.graphics.Reverse = false
		case 29: // Not strikethrough
			p.graphics.Strikethrough = false
		case 30, 31, 32, 33, 34, 35, 36, 37: // Foreground colors
			p.graphics.ForegroundColor = param - 30
		case 38: // Extended foreground color
			if i+1 < len(p.params) && p.params[i+1] == 5 && i+2 < len(p.params) {
				// 256-color mode: ESC[38;5;n
				p.graphics.ForegroundColor = p.params[i+2]
				i += 2
			}
		case 39: // Default foreground color
			p.graphics.ForegroundColor = 7
		case 40, 41, 42, 43, 44, 45, 46, 47: // Background colors
			p.graphics.BackgroundColor = param - 40
		case 48: // Extended background color
			if i+1 < len(p.params) && p.params[i+1] == 5 && i+2 < len(p.params) {
				// 256-color mode: ESC[48;5;n
				p.graphics.BackgroundColor = p.params[i+2]
				i += 2
			}
		case 49: // Default background color
			p.graphics.BackgroundColor = 0
		case 90, 91, 92, 93, 94, 95, 96, 97: // Bright foreground colors
			p.graphics.ForegroundColor = param - 90 + 8
		case 100, 101, 102, 103, 104, 105, 106, 107: // Bright background colors
			p.graphics.BackgroundColor = param - 100 + 8
		}
	}
	
	if p.onGraphics != nil {
		p.onGraphics(p.graphics)
	}
}

// Helper functions
func (p *ANSIParser) advanceCursor() {
	p.cursor.X++
	if p.cursor.X >= p.screen.Width {
		if p.cursor.WrapMode {
			p.cursor.X = 0
			p.handleLineFeed()
		} else {
			p.cursor.X = p.screen.Width - 1
		}
	}
	if p.onCursor != nil {
		p.onCursor(p.cursor.X, p.cursor.Y)
	}
}

func (p *ANSIParser) handleLineFeed() {
	if p.cursor.Y >= p.screen.ScrollBottom {
		if p.onScroll != nil {
			p.onScroll(1, 1) // Scroll down
		}
	} else {
		p.cursor.Y++
	}
	if p.onCursor != nil {
		p.onCursor(p.cursor.X, p.cursor.Y)
	}
}

func (p *ANSIParser) handleTab() {
	nextTab := ((p.cursor.X / 8) + 1) * 8
	if nextTab < p.screen.Width {
		p.cursor.X = nextTab
	} else {
		p.cursor.X = p.screen.Width - 1
	}
	if p.onCursor != nil {
		p.onCursor(p.cursor.X, p.cursor.Y)
	}
}

func (p *ANSIParser) resetParser() {
	p.params = nil
	p.paramBuffer.Reset()
	p.intermediates.Reset()
	p.finalByte = 0
	p.collectingParams = false
	p.private = false
}

func (p *ANSIParser) resetTerminal() {
	p.graphics = GraphicsState{
		ForegroundColor: 7,
		BackgroundColor: 0,
	}
	p.cursor = CursorState{
		X:        0,
		Y:        0,
		Visible:  true,
		WrapMode: true,
	}
	p.screen.ScrollTop = 0
	p.screen.ScrollBottom = p.screen.Height - 1
}

// GetGraphicsState returns the current graphics state
func (p *ANSIParser) GetGraphicsState() GraphicsState {
	return p.graphics
}

// GetCursorState returns the current cursor state
func (p *ANSIParser) GetCursorState() CursorState {
	return p.cursor
}

// SetDimensions updates the terminal dimensions
func (p *ANSIParser) SetDimensions(width, height int) {
	p.screen.Width = width
	p.screen.Height = height
	p.screen.ScrollBottom = height - 1
}

// Helper functions for min/max
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}