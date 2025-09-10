// Package goturbotui provides a TUI library inspired by Borland Turbo Vision
// with authentic retro computing aesthetics and modern Go idioms.
package goturbotui

// Rect represents a rectangular area with position and dimensions
type Rect struct {
	X, Y int // Top-left position
	W, H int // Width and height
}

// NewRect creates a new rectangle with the specified bounds
func NewRect(x, y, w, h int) Rect {
	return Rect{X: x, Y: y, W: w, H: h}
}

// Right returns the X coordinate of the right edge (exclusive)
func (r Rect) Right() int {
	return r.X + r.W
}

// Bottom returns the Y coordinate of the bottom edge (exclusive)
func (r Rect) Bottom() int {
	return r.Y + r.H
}

// Center returns a rectangle centered within this rectangle
func (r Rect) Center(w, h int) Rect {
	x := r.X + (r.W-w)/2
	y := r.Y + (r.H-h)/2
	return NewRect(x, y, w, h)
}

// Inner returns a rectangle inset by the specified padding
func (r Rect) Inner(padding int) Rect {
	return NewRect(
		r.X+padding,
		r.Y+padding,
		r.W-2*padding,
		r.H-2*padding,
	)
}

// Contains checks if a point is within this rectangle
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.Right() && y >= r.Y && y < r.Bottom()
}

// Intersect returns the intersection of this rectangle with another
func (r Rect) Intersect(other Rect) Rect {
	x1 := max(r.X, other.X)
	y1 := max(r.Y, other.Y)
	x2 := min(r.Right(), other.Right())
	y2 := min(r.Bottom(), other.Bottom())
	
	if x1 >= x2 || y1 >= y2 {
		return NewRect(0, 0, 0, 0) // Empty rectangle
	}
	
	return NewRect(x1, y1, x2-x1, y2-y1)
}

// Union returns the union of this rectangle with another
func (r Rect) Union(other Rect) Rect {
	if r.W == 0 && r.H == 0 {
		return other
	}
	if other.W == 0 && other.H == 0 {
		return r
	}
	
	x1 := min(r.X, other.X)
	y1 := min(r.Y, other.Y)
	x2 := max(r.Right(), other.Right())
	y2 := max(r.Bottom(), other.Bottom())
	
	return NewRect(x1, y1, x2-x1, y2-y1)
}

// Move returns a new rectangle offset by the specified amount
func (r Rect) Move(dx, dy int) Rect {
	return NewRect(r.X+dx, r.Y+dy, r.W, r.H)
}

// Resize returns a new rectangle with the specified size
func (r Rect) Resize(w, h int) Rect {
	return NewRect(r.X, r.Y, w, h)
}

// IsEmpty returns true if the rectangle has zero area
func (r Rect) IsEmpty() bool {
	return r.W <= 0 || r.H <= 0
}

// Helper functions
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