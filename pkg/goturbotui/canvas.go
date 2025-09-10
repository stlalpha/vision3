package goturbotui

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Canvas represents a drawing surface for rendering UI elements
type Canvas interface {
	// Size returns the canvas dimensions
	Size() (width, height int)
	
	// SetCell sets a character at the specified position with style
	SetCell(x, y int, char rune, style Style)
	
	// SetString renders a string at the specified position with style
	SetString(x, y int, text string, style Style)
	
	// Fill fills a rectangle with the specified character and style
	Fill(rect Rect, char rune, style Style)
	
	// DrawBox draws a box outline with the specified style
	DrawBox(rect Rect, style Style)
	
	// DrawBoxWithTitle draws a box with a title
	DrawBoxWithTitle(rect Rect, title string, style Style)
	
	// Clear clears the entire canvas
	Clear(style Style)
	
	// Render outputs the canvas to the terminal
	Render() error
}

// Cell represents a single character cell with styling
type Cell struct {
	Char  rune
	Style Style
}

// MemoryCanvas is an in-memory implementation of Canvas
type MemoryCanvas struct {
	width       int
	height      int
	cells       [][]Cell
	dirty       bool
	firstRender bool
}

// NewMemoryCanvas creates a new memory-based canvas
func NewMemoryCanvas(width, height int) *MemoryCanvas {
	cells := make([][]Cell, height)
	for i := range cells {
		cells[i] = make([]Cell, width)
		for j := range cells[i] {
			cells[i][j] = Cell{
				Char:  ' ',
				Style: NewStyle(),
			}
		}
	}
	
	return &MemoryCanvas{
		width:       width,
		height:      height,
		cells:       cells,
		dirty:       true,
		firstRender: true,
	}
}

// Size returns the canvas dimensions
func (c *MemoryCanvas) Size() (width, height int) {
	return c.width, c.height
}

// SetCell sets a character at the specified position with style
func (c *MemoryCanvas) SetCell(x, y int, char rune, style Style) {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		c.cells[y][x] = Cell{Char: char, Style: style}
		c.dirty = true
	}
}

// SetString renders a string at the specified position with style
func (c *MemoryCanvas) SetString(x, y int, text string, style Style) {
	if y < 0 || y >= c.height {
		return
	}
	
	currentX := x
	for _, char := range text {
		if currentX >= c.width {
			break
		}
		if currentX >= 0 {
			c.SetCell(currentX, y, char, style)
		}
		currentX++
	}
}

// Fill fills a rectangle with the specified character and style
func (c *MemoryCanvas) Fill(rect Rect, char rune, style Style) {
	for y := rect.Y; y < rect.Bottom() && y < c.height; y++ {
		if y < 0 {
			continue
		}
		for x := rect.X; x < rect.Right() && x < c.width; x++ {
			if x < 0 {
				continue
			}
			c.SetCell(x, y, char, style)
		}
	}
}

// DrawBox draws a box outline with the specified style
func (c *MemoryCanvas) DrawBox(rect Rect, style Style) {
	if rect.W < 2 || rect.H < 2 {
		return
	}
	
	// Box drawing characters
	topLeft := '┌'
	topRight := '┐'
	bottomLeft := '└'
	bottomRight := '┘'
	horizontal := '─'
	vertical := '│'
	
	// Top and bottom borders
	for x := rect.X + 1; x < rect.Right()-1; x++ {
		c.SetCell(x, rect.Y, horizontal, style)
		c.SetCell(x, rect.Bottom()-1, horizontal, style)
	}
	
	// Left and right borders
	for y := rect.Y + 1; y < rect.Bottom()-1; y++ {
		c.SetCell(rect.X, y, vertical, style)
		c.SetCell(rect.Right()-1, y, vertical, style)
	}
	
	// Corners
	c.SetCell(rect.X, rect.Y, topLeft, style)
	c.SetCell(rect.Right()-1, rect.Y, topRight, style)
	c.SetCell(rect.X, rect.Bottom()-1, bottomLeft, style)
	c.SetCell(rect.Right()-1, rect.Bottom()-1, bottomRight, style)
}

// DrawBoxWithTitle draws a box with a title
func (c *MemoryCanvas) DrawBoxWithTitle(rect Rect, title string, style Style) {
	c.DrawBox(rect, style)
	
	if title != "" && rect.W > 4 {
		// Calculate title position (centered)
		titleLen := utf8.RuneCountInString(title)
		maxLen := rect.W - 4 // Leave space for "─ " and " ─"
		if titleLen > maxLen {
			title = title[:maxLen]
			titleLen = maxLen
		}
		
		titleX := rect.X + 2
		titleText := "─ " + title + " "
		
		c.SetString(titleX, rect.Y, titleText, style)
	}
}

// Clear clears the entire canvas
func (c *MemoryCanvas) Clear(style Style) {
	for y := 0; y < c.height; y++ {
		for x := 0; x < c.width; x++ {
			c.cells[y][x] = Cell{Char: ' ', Style: style}
		}
	}
	c.dirty = true
}

// Render outputs the canvas to the terminal
func (c *MemoryCanvas) Render() error {
	if !c.dirty {
		return nil
	}
	
	var output strings.Builder
	var lastStyle *Style
	
	// Clear screen only on first render to eliminate flash
	if c.firstRender {
		output.WriteString("\033[2J\033[H")
		c.firstRender = false
	} else {
		output.WriteString("\033[H")
	}
	
	for y := 0; y < c.height; y++ {
		// Position cursor at start of each row
		output.WriteString(fmt.Sprintf("\033[%d;1H", y+1))
		
		for x := 0; x < c.width; x++ {
			cell := c.cells[y][x]
			
			// Only output style changes when needed
			if lastStyle == nil || *lastStyle != cell.Style {
				output.WriteString(cell.Style.ToANSI())
				lastStyle = &cell.Style
			}
			
			output.WriteRune(cell.Char)
		}
	}
	
	// Reset style at the end
	output.WriteString(Reset())
	
	// Output to stdout in one atomic operation
	print(output.String())
	
	c.dirty = false
	return nil
}

// Resize resizes the canvas to new dimensions
func (c *MemoryCanvas) Resize(width, height int) {
	if width == c.width && height == c.height {
		return
	}
	
	newCells := make([][]Cell, height)
	defaultCell := Cell{Char: ' ', Style: NewStyle()}
	
	for i := range newCells {
		newCells[i] = make([]Cell, width)
		for j := range newCells[i] {
			// Copy existing cells or use default
			if i < c.height && j < c.width {
				newCells[i][j] = c.cells[i][j]
			} else {
				newCells[i][j] = defaultCell
			}
		}
	}
	
	c.width = width
	c.height = height
	c.cells = newCells
	c.dirty = true
}