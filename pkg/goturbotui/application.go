package goturbotui

import (
	"context"
	"fmt"
)

// Application represents the main TUI application
type Application struct {
	screen     Screen
	canvas     Canvas
	desktop    Container
	modalStack []View
	running    bool
	theme      *Theme
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewApplication creates a new TUI application
func NewApplication() *Application {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Application{
		screen:     NewTerminalScreen(),
		modalStack: make([]View, 0),
		running:    false,
		theme:      DefaultTurboTheme(),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// SetDesktop sets the desktop (main) view
func (a *Application) SetDesktop(desktop Container) {
	a.desktop = desktop
}

// GetDesktop returns the desktop view
func (a *Application) GetDesktop() Container {
	return a.desktop
}

// ShowModal displays a modal dialog
func (a *Application) ShowModal(modal View) {
	a.modalStack = append(a.modalStack, modal)
}

// CloseModal closes the topmost modal dialog
func (a *Application) CloseModal() {
	if len(a.modalStack) > 0 {
		a.modalStack = a.modalStack[:len(a.modalStack)-1]
	}
}

// GetTopModal returns the topmost modal dialog
func (a *Application) GetTopModal() View {
	if len(a.modalStack) > 0 {
		return a.modalStack[len(a.modalStack)-1]
	}
	return nil
}

// SetTheme sets the application theme
func (a *Application) SetTheme(theme *Theme) {
	a.theme = theme
}

// GetTheme returns the current theme
func (a *Application) GetTheme() *Theme {
	return a.theme
}

// Run starts the application main loop
func (a *Application) Run() error {
	if a.running {
		return fmt.Errorf("application is already running")
	}
	
	// Initialize screen
	if err := a.screen.Init(); err != nil {
		return fmt.Errorf("failed to initialize screen: %w", err)
	}
	defer a.screen.Close()
	
	// Create canvas
	width, height := a.screen.Size()
	a.canvas = NewMemoryCanvas(width, height)
	
	// Set desktop bounds
	if a.desktop != nil {
		a.desktop.SetBounds(NewRect(0, 0, width, height))
	}
	
	a.running = true
	defer func() { a.running = false }()
	
	// Initial draw
	a.draw()
	
	// Main event loop
	events := a.screen.PollEvents()
	
	for a.running {
		select {
		case <-a.ctx.Done():
			return nil
			
		case event := <-events:
			a.handleEvent(event)
			a.draw()
		}
	}
	
	return nil
}

// Stop stops the application
func (a *Application) Stop() {
	a.running = false
	a.cancel()
}

// draw renders the entire application
func (a *Application) draw() {
	// Clear canvas with desktop background
	if a.theme != nil {
		a.canvas.Clear(a.theme.Desktop)
	} else {
		a.canvas.Clear(NewStyle().WithBackground(ColorBlue))
	}
	
	// Draw desktop
	if a.desktop != nil && a.desktop.IsVisible() {
		a.desktop.Draw(a.canvas)
	}
	
	// Draw modals in order
	for _, modal := range a.modalStack {
		if modal.IsVisible() {
			modal.Draw(a.canvas)
		}
	}
	
	// Render to screen
	a.canvas.Render()
}

// handleEvent processes input events
func (a *Application) handleEvent(event Event) {
	// Handle global keys first
	if event.Type == EventKey {
		// Handle resize
		if event.Type == EventResize {
			width := event.Resize.Width
			height := event.Resize.Height
			
			// Resize canvas
			if memCanvas, ok := a.canvas.(*MemoryCanvas); ok {
				memCanvas.Resize(width, height)
			}
			
			// Update desktop bounds
			if a.desktop != nil {
				a.desktop.SetBounds(NewRect(0, 0, width, height))
			}
			
			// Update modal positions (modals should handle their own centering)
			// This is just a basic resize notification
			for _, modal := range a.modalStack {
				if resizeHandler, ok := modal.(interface{ Resize(int, int) }); ok {
					resizeHandler.Resize(width, height)
				}
			}
			return
		}
		
		// Global shortcuts
		switch event.Key.Code {
		case KeyF10:
			if event.Key.Modifiers == ModNone {
				a.Stop()
				return
			}
		}
		
		// Handle Ctrl+C
		if event.Key.Modifiers == ModCtrl && event.Rune == 'c' {
			a.Stop()
			return
		}
	}
	
	// Try topmost modal first
	if topModal := a.GetTopModal(); topModal != nil {
		if topModal.HandleEvent(event) {
			return
		}
	}
	
	// Then try desktop
	if a.desktop != nil {
		a.desktop.HandleEvent(event)
	}
}

// Quit is a convenience method to stop the application
func (a *Application) Quit() {
	a.Stop()
}

// GetCanvas returns the application canvas
func (a *Application) GetCanvas() Canvas {
	return a.canvas
}

// GetScreen returns the application screen
func (a *Application) GetScreen() Screen {
	return a.screen
}