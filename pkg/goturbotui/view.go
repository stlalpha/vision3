package goturbotui

// View is the base interface for all UI components
type View interface {
	// Draw renders the view onto the given canvas
	Draw(canvas Canvas)
	
	// HandleEvent processes an input event and returns true if handled
	HandleEvent(event Event) bool
	
	// SetBounds sets the view's boundaries
	SetBounds(bounds Rect)
	
	// GetBounds returns the view's current boundaries
	GetBounds() Rect
	
	// SetVisible sets the view's visibility
	SetVisible(visible bool)
	
	// IsVisible returns whether the view is visible
	IsVisible() bool
	
	// SetFocused sets the view's focus state
	SetFocused(focused bool)
	
	// IsFocused returns whether the view is focused
	IsFocused() bool
	
	// CanFocus returns whether the view can receive focus
	CanFocus() bool
}

// BaseView provides common functionality for views
type BaseView struct {
	bounds  Rect
	visible bool
	focused bool
	canFocus bool
}

// NewBaseView creates a new base view
func NewBaseView() *BaseView {
	return &BaseView{
		bounds:   NewRect(0, 0, 0, 0),
		visible:  true,
		focused:  false,
		canFocus: false,
	}
}

// SetBounds sets the view's boundaries
func (v *BaseView) SetBounds(bounds Rect) {
	v.bounds = bounds
}

// GetBounds returns the view's current boundaries
func (v *BaseView) GetBounds() Rect {
	return v.bounds
}

// SetVisible sets the view's visibility
func (v *BaseView) SetVisible(visible bool) {
	v.visible = visible
}

// IsVisible returns whether the view is visible
func (v *BaseView) IsVisible() bool {
	return v.visible
}

// SetFocused sets the view's focus state
func (v *BaseView) SetFocused(focused bool) {
	v.focused = focused
}

// IsFocused returns whether the view is focused
func (v *BaseView) IsFocused() bool {
	return v.focused
}

// SetCanFocus sets whether the view can receive focus
func (v *BaseView) SetCanFocus(canFocus bool) {
	v.canFocus = canFocus
}

// CanFocus returns whether the view can receive focus
func (v *BaseView) CanFocus() bool {
	return v.canFocus
}

// Draw is a default implementation (does nothing)
func (v *BaseView) Draw(canvas Canvas) {
	// Base views don't draw anything by default
}

// HandleEvent is a default implementation (returns false)
func (v *BaseView) HandleEvent(event Event) bool {
	return false // Base views don't handle events by default
}

// Container represents a view that can contain child views
type Container interface {
	View
	
	// AddChild adds a child view
	AddChild(child View)
	
	// RemoveChild removes a child view
	RemoveChild(child View)
	
	// GetChildren returns all child views
	GetChildren() []View
	
	// SetFocus sets focus to a specific child
	SetFocus(child View)
	
	// GetFocused returns the currently focused child
	GetFocused() View
}

// BaseContainer provides common functionality for container views
type BaseContainer struct {
	*BaseView
	children []View
	focused  View
}

// NewBaseContainer creates a new base container
func NewBaseContainer() *BaseContainer {
	return &BaseContainer{
		BaseView: NewBaseView(),
		children: make([]View, 0),
	}
}

// AddChild adds a child view
func (c *BaseContainer) AddChild(child View) {
	c.children = append(c.children, child)
}

// RemoveChild removes a child view
func (c *BaseContainer) RemoveChild(child View) {
	for i, v := range c.children {
		if v == child {
			c.children = append(c.children[:i], c.children[i+1:]...)
			if c.focused == child {
				c.focused = nil
			}
			break
		}
	}
}

// GetChildren returns all child views
func (c *BaseContainer) GetChildren() []View {
	return c.children
}

// SetFocus sets focus to a specific child
func (c *BaseContainer) SetFocus(child View) {
	if c.focused != nil {
		c.focused.SetFocused(false)
	}
	c.focused = child
	if child != nil && child.CanFocus() {
		child.SetFocused(true)
	}
}

// GetFocused returns the currently focused child
func (c *BaseContainer) GetFocused() View {
	return c.focused
}

// Draw draws the container and all visible children
func (c *BaseContainer) Draw(canvas Canvas) {
	for _, child := range c.children {
		if child.IsVisible() {
			child.Draw(canvas)
		}
	}
}

// HandleEvent handles events by passing them to the focused child first
func (c *BaseContainer) HandleEvent(event Event) bool {
	// Try focused child first
	if c.focused != nil && c.focused.IsVisible() {
		if c.focused.HandleEvent(event) {
			return true
		}
	}
	
	// Try other children in reverse order (top-most first)
	for i := len(c.children) - 1; i >= 0; i-- {
		child := c.children[i]
		if child != c.focused && child.IsVisible() {
			if child.HandleEvent(event) {
				return true
			}
		}
	}
	
	return false
}