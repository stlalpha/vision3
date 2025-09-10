package components

import (
	"github.com/stlalpha/vision3/pkg/goturbotui"
)

// Window represents the main application window
type Window struct {
	*goturbotui.BaseContainer
	title     string
	theme     *goturbotui.Theme
	menuBar   *MenuBar
	statusBar *StatusBar
	client    goturbotui.View
}

// NewWindow creates a new window with optional menu and status bars
func NewWindow(title string, theme *goturbotui.Theme) *Window {
	window := &Window{
		BaseContainer: goturbotui.NewBaseContainer(),
		title:         title,
		theme:         theme,
	}
	
	return window
}

// SetMenuBar sets the window's menu bar
func (w *Window) SetMenuBar(menuBar *MenuBar) {
	w.menuBar = menuBar
	if menuBar != nil {
		w.AddChild(menuBar)
	}
}

// GetMenuBar returns the window's menu bar
func (w *Window) GetMenuBar() *MenuBar {
	return w.menuBar
}

// SetStatusBar sets the window's status bar
func (w *Window) SetStatusBar(statusBar *StatusBar) {
	w.statusBar = statusBar
	if statusBar != nil {
		w.AddChild(statusBar)
	}
}

// GetStatusBar returns the window's status bar
func (w *Window) GetStatusBar() *StatusBar {
	return w.statusBar
}

// SetClient sets the main client area view
func (w *Window) SetClient(client goturbotui.View) {
	w.client = client
	if client != nil {
		w.AddChild(client)
	}
}

// GetClient returns the main client area view
func (w *Window) GetClient() goturbotui.View {
	return w.client
}

// SetBounds overrides to layout child components
func (w *Window) SetBounds(bounds goturbotui.Rect) {
	w.BaseContainer.SetBounds(bounds)
	w.layoutChildren()
}

// layoutChildren positions the menu bar, status bar, and client area
func (w *Window) layoutChildren() {
	bounds := w.GetBounds()
	
	currentY := bounds.Y
	remainingHeight := bounds.H
	
	// Layout menu bar at top
	if w.menuBar != nil && w.menuBar.IsVisible() {
		menuBounds := goturbotui.NewRect(bounds.X, currentY, bounds.W, 1)
		w.menuBar.SetBounds(menuBounds)
		currentY++
		remainingHeight--
	}
	
	// Layout status bar at bottom
	if w.statusBar != nil && w.statusBar.IsVisible() {
		statusBounds := goturbotui.NewRect(bounds.X, bounds.Bottom()-1, bounds.W, 1)
		w.statusBar.SetBounds(statusBounds)
		remainingHeight--
	}
	
	// Layout client area in the middle
	if w.client != nil && w.client.IsVisible() {
		clientBounds := goturbotui.NewRect(bounds.X, currentY, bounds.W, remainingHeight)
		w.client.SetBounds(clientBounds)
	}
}

// Draw renders the window
func (w *Window) Draw(canvas goturbotui.Canvas) {
	if !w.IsVisible() {
		return
	}
	
	bounds := w.GetBounds()
	theme := w.theme
	if theme == nil {
		theme = goturbotui.DefaultTurboTheme()
	}
	
	// Fill background with desktop pattern
	canvas.Fill(bounds, '░', theme.Desktop)
	
	// Draw child components
	w.BaseContainer.Draw(canvas)
}

// HandleEvent handles window events
func (w *Window) HandleEvent(event goturbotui.Event) bool {
	if !w.IsVisible() {
		return false
	}
	
	// Handle resize events
	if event.Type == goturbotui.EventResize {
		w.SetBounds(goturbotui.NewRect(0, 0, event.Resize.Width, event.Resize.Height))
		return true
	}
	
	// Pass to children
	return w.BaseContainer.HandleEvent(event)
}