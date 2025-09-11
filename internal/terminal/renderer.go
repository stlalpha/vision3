package terminal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RenderMode specifies how content should be rendered
type RenderMode int

const (
	RenderModeAuto RenderMode = iota // Auto-detect best rendering strategy
	RenderModeUTF8                   // Force UTF-8 rendering
	RenderModeCP437                  // Force CP437 byte-accurate rendering
	RenderModeVT100                  // Use VT100 line drawing sequences
	RenderModeASCII                  // Use ASCII fallbacks only
	RenderModeAmiga                  // Use Amiga character set rendering
)

// ArtRenderer handles rendering of ANSI art and text content
type ArtRenderer struct {
	parser      *ANSIParser
	charset     *CharsetHandler
	outputMode  OutputMode
	renderMode  RenderMode
	writer      io.Writer
	capabilities Capabilities
	
	// State tracking
	currentPos    Position
	placeholders  map[rune]Position
	screenBuffer  [][]rune
	colorBuffer   [][]GraphicsState
	
	// SAUCE metadata
	sauce         *SAUCEInfo
	iceColors     bool
	nonBlink      bool
}

// NewArtRenderer creates a new art renderer
func NewArtRenderer(writer io.Writer, capabilities Capabilities, outputMode OutputMode) *ArtRenderer {
	renderer := &ArtRenderer{
		parser:       NewANSIParser(capabilities.Width, capabilities.Height),
		charset:      NewCharsetHandler(),
		outputMode:   outputMode,
		renderMode:   RenderModeAuto,
		writer:       writer,
		capabilities: capabilities,
		placeholders: make(map[rune]Position),
	}
	
	// Configure charset handler based on capabilities
	switch outputMode {
	case OutputModeCP437:
		renderer.charset.SetCharset(CharsetCP437)
		renderer.renderMode = RenderModeCP437
	case OutputModeUTF8:
		renderer.charset.SetCharset(CharsetUTF8)
		renderer.renderMode = RenderModeUTF8
		if !capabilities.SupportsUTF8 {
			renderer.charset.SetFallbackMode(true)
			renderer.renderMode = RenderModeASCII
		}
	case OutputModeAuto:
		renderer.determineRenderMode()
	}
	
	// Set up parser callbacks
	renderer.setupParserCallbacks()
	
	return renderer
}

// determineRenderMode automatically determines the best render mode
func (r *ArtRenderer) determineRenderMode() {
	switch r.capabilities.TerminalType {
	case TerminalSyncTERM:
		r.renderMode = RenderModeCP437
		r.charset.SetCharset(CharsetCP437)
	case TerminalVT100:
		r.renderMode = RenderModeVT100
		r.charset.SetVT100Mode(true)
	case TerminalXTerm, TerminalUTF8:
		if r.capabilities.SupportsUTF8 {
			r.renderMode = RenderModeUTF8
			r.charset.SetCharset(CharsetUTF8)
		} else {
			r.renderMode = RenderModeASCII
			r.charset.SetFallbackMode(true)
		}
	case TerminalANSI:
		r.renderMode = RenderModeCP437
		r.charset.SetCharset(CharsetCP437)
	case TerminalAmiga:
		r.renderMode = RenderModeAmiga
		r.charset.SetCharset(CharsetAmiga)
	default:
		// Conservative fallback
		r.renderMode = RenderModeASCII
		r.charset.SetFallbackMode(true)
	}
}

// setupParserCallbacks configures the ANSI parser callbacks
func (r *ArtRenderer) setupParserCallbacks() {
	r.parser.SetCallbacks(
		r.onText,
		r.onCursor,
		r.onGraphics,
		r.onClear,
		r.onScroll,
	)
}

// RenderAnsiFile renders an ANSI art file
func (r *ArtRenderer) RenderAnsiFile(filename string) error {
	data, err := r.loadAnsiFile(filename)
	if err != nil {
		return fmt.Errorf("failed to load ANSI file %s: %w", filename, err)
	}
	
	return r.RenderAnsiBytes(data)
}

// RenderAnsiBytes renders ANSI art from byte data
func (r *ArtRenderer) RenderAnsiBytes(data []byte) error {
	// Parse SAUCE metadata if present
	sauce, artData, err := r.ParseSAUCE(data)
	if err == nil && sauce != nil {
		r.sauce = sauce
		r.iceColors = sauce.IceColors
		r.nonBlink = sauce.NonBlink
		data = artData
	}
	
	// Process pipe codes first
	processedData := r.charset.ProcessPipeCodes(data)
	
	// Reset state
	r.placeholders = make(map[rune]Position)
	r.currentPos = Position{X: 0, Y: 0}
	
	// Parse and render the ANSI data
	return r.parser.ParseBytes(processedData)
}

// ProcessAnsiAndExtractCoords processes ANSI data and extracts placeholder coordinates
func (r *ArtRenderer) ProcessAnsiAndExtractCoords(rawContent []byte) (ProcessAnsiResult, error) {
	// Parse SAUCE metadata
	sauce, artData, err := r.ParseSAUCE(rawContent)
	if err == nil && sauce != nil {
		r.sauce = sauce
		rawContent = artData
	}
	
	// Process pipe codes
	processedData := r.charset.ProcessPipeCodes(rawContent)
	
	// Create a coordinate extraction renderer
	coordExtractor := &coordExtractingRenderer{
		placeholders: make(map[rune]Position),
		maxX:         0,
		maxY:         0,
	}
	
	// Set up a temporary parser for coordinate extraction
	tempParser := NewANSIParser(r.capabilities.Width, r.capabilities.Height)
	tempParser.SetCallbacks(
		coordExtractor.onText,
		coordExtractor.onCursor,
		nil, // graphics callback not needed for coord extraction
		nil, // clear callback not needed
		nil, // scroll callback not needed
	)
	
	// Parse to extract coordinates
	if err := tempParser.ParseBytes(processedData); err != nil {
		return ProcessAnsiResult{}, err
	}
	
	return ProcessAnsiResult{
		ProcessedContent:  processedData,
		PlaceholderCoords: coordExtractor.placeholders,
		OriginalSize:     Position{X: coordExtractor.maxX, Y: coordExtractor.maxY},
	}, nil
}

// coordExtractingRenderer is a helper for extracting placeholder coordinates
type coordExtractingRenderer struct {
	placeholders map[rune]Position
	currentX     int
	currentY     int
	maxX         int
	maxY         int
}

func (c *coordExtractingRenderer) onText(data []byte) {
	for _, b := range data {
		if b >= 'A' && b <= 'Z' {
			// Check if this might be a placeholder character
			c.placeholders[rune(b)] = Position{X: c.currentX, Y: c.currentY}
		}
		c.currentX++
		if c.currentX > c.maxX {
			c.maxX = c.currentX
		}
	}
}

func (c *coordExtractingRenderer) onCursor(x, y int) {
	c.currentX = x
	c.currentY = y
	if y > c.maxY {
		c.maxY = y
	}
}

// loadAnsiFile loads an ANSI file with proper error handling
func (r *ArtRenderer) loadAnsiFile(filename string) ([]byte, error) {
	// Handle relative paths
	if !filepath.IsAbs(filename) {
		// Try common ANSI art directories
		searchPaths := []string{
			"menus/v3/ansi",
			"ansi",
			"art",
			".",
		}
		
		for _, basePath := range searchPaths {
			fullPath := filepath.Join(basePath, filename)
			if _, err := os.Stat(fullPath); err == nil {
				filename = fullPath
				break
			}
		}
	}
	
	// Add appropriate extension if not present
	lowerFilename := strings.ToLower(filename)
	if !strings.HasSuffix(lowerFilename, ".ans") && 
	   !strings.HasSuffix(lowerFilename, ".ansi") &&
	   !strings.HasSuffix(lowerFilename, ".amiga") &&
	   !strings.HasSuffix(lowerFilename, ".txt") {
		// Default to .ANS for backward compatibility
		filename += ".ANS"
	}
	
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot read file %s: %w", filename, err)
	}
	
	return data, nil
}

// ParseSAUCE parses SAUCE metadata from file data
func (r *ArtRenderer) ParseSAUCE(data []byte) (*SAUCEInfo, []byte, error) {
	if len(data) < 128 {
		return nil, data, nil // No SAUCE possible
	}
	
	// Look for SAUCE signature at end of file
	sauceStart := len(data) - 128
	if string(data[sauceStart:sauceStart+5]) != "SAUCE" {
		return nil, data, nil // No SAUCE present
	}
	
	sauce := &SAUCEInfo{}
	
	// Parse SAUCE record (128 bytes total)
	reader := bytes.NewReader(data[sauceStart:])
	
	// Skip "SAUCE" signature (5 bytes)
	reader.Seek(5, io.SeekCurrent)
	
	// Version (2 bytes)
	version := make([]byte, 2)
	reader.Read(version)
	sauce.Version = strings.TrimSpace(string(version))
	
	// Title (35 bytes)
	title := make([]byte, 35)
	reader.Read(title)
	sauce.Title = strings.TrimSpace(string(title))
	
	// Author (20 bytes)
	author := make([]byte, 20)
	reader.Read(author)
	sauce.Author = strings.TrimSpace(string(author))
	
	// Group (20 bytes)
	group := make([]byte, 20)
	reader.Read(group)
	sauce.Group = strings.TrimSpace(string(group))
	
	// Date (8 bytes) - CCYYMMDD format
	dateStr := make([]byte, 8)
	reader.Read(dateStr)
	if date, err := time.Parse("20060102", string(dateStr)); err == nil {
		sauce.Date = date
	}
	
	// File size (4 bytes, little-endian)
	var fileSize uint32
	for i := 0; i < 4; i++ {
		var b byte
		reader.ReadByte()
		fileSize |= uint32(b) << (uint(i) * 8)
	}
	sauce.FileSize = int(fileSize)
	
	// Data type (1 byte)
	dataType, _ := reader.ReadByte()
	sauce.DataType = int(dataType)
	
	// File type (1 byte)
	fileType, _ := reader.ReadByte()
	sauce.FileType = int(fileType)
	
	// Type info (4 bytes)
	tinfo := make([]byte, 4)
	reader.Read(tinfo)
	sauce.TInfo1 = int(tinfo[0]) | (int(tinfo[1]) << 8)
	sauce.TInfo2 = int(tinfo[2]) | (int(tinfo[3]) << 8)
	
	// More type info (4 bytes)
	tinfo2 := make([]byte, 4)
	reader.Read(tinfo2)
	sauce.TInfo3 = int(tinfo2[0]) | (int(tinfo2[1]) << 8)
	sauce.TInfo4 = int(tinfo2[2]) | (int(tinfo2[3]) << 8)
	
	// Comments (1 byte)
	comments, _ := reader.ReadByte()
	
	// Flags (1 byte)
	flags, _ := reader.ReadByte()
	sauce.IceColors = (flags & 0x01) != 0
	sauce.NonBlink = (flags & 0x02) != 0
	
	// Skip filler (22 bytes)
	reader.Seek(22, io.SeekCurrent)
	
	// Parse comments if present
	if comments > 0 {
		commentStart := sauceStart - int(comments)*64 - 5
		if commentStart >= 0 && string(data[commentStart:commentStart+5]) == "COMNT" {
			for i := 0; i < int(comments); i++ {
				commentData := data[commentStart+5+i*64 : commentStart+5+(i+1)*64]
				sauce.Comments = append(sauce.Comments, strings.TrimSpace(string(commentData)))
			}
			// Return data without SAUCE and comments
			return sauce, data[:commentStart], nil
		}
	}
	
	// Return data without SAUCE
	return sauce, data[:sauceStart], nil
}

// Callback functions for ANSI parser

func (r *ArtRenderer) onText(data []byte) {
	switch r.renderMode {
	case RenderModeCP437:
		// Write raw CP437 bytes
		r.writer.Write(data)
	case RenderModeUTF8:
		// Convert CP437 to UTF-8
		utf8Text := r.charset.ConvertCP437ToUTF8(data)
		r.writer.Write([]byte(utf8Text))
	case RenderModeVT100:
		// Convert to VT100 with line drawing
		utf8Text := r.charset.ConvertCP437ToUTF8(data)
		vt100Text := r.charset.ConvertToVT100LineDrawing(utf8Text)
		r.writer.Write([]byte(vt100Text))
	case RenderModeASCII:
		// Use ASCII fallbacks
		for _, b := range data {
			rune := r.charset.ConvertCP437ByteToUTF8(b)
			r.writer.Write([]byte(string(rune)))
		}
	case RenderModeAmiga:
		// Convert Amiga character set to UTF-8
		utf8Text := r.charset.ConvertAmigaToUTF8(data)
		r.writer.Write([]byte(utf8Text))
	}
	
	// Update current position
	r.currentPos.X += len(data)
}

func (r *ArtRenderer) onCursor(x, y int) {
	r.currentPos.X = x
	r.currentPos.Y = y
	
	// Send cursor positioning command
	cursorCmd := fmt.Sprintf("\x1b[%d;%dH", y+1, x+1) // Convert to 1-based
	r.writer.Write([]byte(cursorCmd))
}

func (r *ArtRenderer) onGraphics(state GraphicsState) {
	// If this is a reset state, output reset sequence
	if state.Reset {
		r.writer.Write([]byte("\x1b[0m"))
		return
	}
	
	// Build SGR sequence based on graphics state
	var params []string
	
	// Handle foreground color
	if state.ForegroundColor >= 0 && state.ForegroundColor <= 15 {
		if state.ForegroundColor < 8 {
			// For normal colors (0-7), check if bold is set to make it bright
			if state.Bold {
				// Bold + normal color = bright color format
				params = append(params, "1", strconv.Itoa(30+state.ForegroundColor))
			} else {
				params = append(params, strconv.Itoa(30+state.ForegroundColor))
			}
		} else {
			// For bright colors (8-15), use the standard bright color format: bold + base color
			params = append(params, "1", strconv.Itoa(30+(state.ForegroundColor-8)))
		}
	}
	
	// Handle background color - only include if it was explicitly set (not default)
	// For BBS accuracy, don't include background color unless it was explicitly changed
	// from default black background (0)
	if state.BackgroundColor >= 0 && state.BackgroundColor <= 7 && state.BackgroundColor != 0 {
		params = append(params, strconv.Itoa(40+state.BackgroundColor))
	}
	
	// Handle text attributes
	// Don't add bold if we already added it for bright foreground colors or bold+normal color
	addedBoldForBrightColor := state.ForegroundColor >= 8 && state.ForegroundColor <= 15
	addedBoldForNormalColor := state.ForegroundColor >= 0 && state.ForegroundColor <= 7 && state.Bold
	if state.Bold && !addedBoldForBrightColor && !addedBoldForNormalColor {
		params = append(params, "1")
	}
	if state.Dim {
		params = append(params, "2")
	}
	if state.Italic {
		params = append(params, "3")
	}
	if state.Underline {
		params = append(params, "4")
	}
	if state.Blink && !r.nonBlink {
		params = append(params, "5")
	}
	if state.Reverse {
		params = append(params, "7")
	}
	if state.Strikethrough {
		params = append(params, "9")
	}
	
	if len(params) > 0 {
		sgrCmd := fmt.Sprintf("\x1b[%sm", strings.Join(params, ";"))
		r.writer.Write([]byte(sgrCmd))
	}
}

func (r *ArtRenderer) onClear(mode int) {
	var clearCmd string
	switch mode {
	case 0: // Clear from cursor to end of screen
		clearCmd = "\x1b[0J"
	case 1: // Clear from beginning of screen to cursor
		clearCmd = "\x1b[1J"
	case 2: // Clear entire screen
		clearCmd = "\x1b[2J"
	default:
		clearCmd = "\x1b[2J" // Default to full clear
	}
	r.writer.Write([]byte(clearCmd))
}

func (r *ArtRenderer) onScroll(direction int, amount int) {
	var scrollCmd string
	if direction > 0 {
		// Scroll down (text moves up)
		for i := 0; i < amount; i++ {
			scrollCmd += "\x1b[S"
		}
	} else {
		// Scroll up (text moves down)
		for i := 0; i < amount; i++ {
			scrollCmd += "\x1b[T"
		}
	}
	if scrollCmd != "" {
		r.writer.Write([]byte(scrollCmd))
	}
}

// GetPlaceholderCoords returns the coordinates of placeholder characters
func (r *ArtRenderer) GetPlaceholderCoords() map[rune]Position {
	return r.placeholders
}

// GetSAUCEInfo returns the SAUCE metadata for the last rendered file
func (r *ArtRenderer) GetSAUCEInfo() *SAUCEInfo {
	return r.sauce
}

// SetRenderMode changes the rendering mode
func (r *ArtRenderer) SetRenderMode(mode RenderMode) {
	r.renderMode = mode
	switch mode {
	case RenderModeCP437:
		r.charset.SetCharset(CharsetCP437)
		r.charset.SetFallbackMode(false)
		r.charset.SetVT100Mode(false)
	case RenderModeUTF8:
		r.charset.SetCharset(CharsetUTF8)
		r.charset.SetFallbackMode(false)
		r.charset.SetVT100Mode(false)
	case RenderModeVT100:
		r.charset.SetCharset(CharsetUTF8)
		r.charset.SetFallbackMode(false)
		r.charset.SetVT100Mode(true)
	case RenderModeASCII:
		r.charset.SetCharset(CharsetUTF8)
		r.charset.SetFallbackMode(true)
		r.charset.SetVT100Mode(false)
	}
}

// ProcessContent processes raw content with current settings
func (r *ArtRenderer) ProcessContent(data []byte) ([]byte, error) {
	var output bytes.Buffer
	originalWriter := r.writer
	r.writer = &output
	
	err := r.RenderAnsiBytes(data)
	r.writer = originalWriter
	
	return output.Bytes(), err
}

// WriteProcessedBytes writes data through the terminal processing pipeline
func (r *ArtRenderer) WriteProcessedBytes(data []byte) error {
	var processedData []byte
	
	// Process based on current charset
	if r.charset.currentCharset == CharsetAmiga {
		processedData = r.charset.ProcessAmigaContent(data)
	} else {
		processedData = r.charset.ProcessPipeCodes(data)
	}
	
	return r.parser.ParseBytes(processedData)
}