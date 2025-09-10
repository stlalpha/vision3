package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

// Window represents a layered window in the TUI
type Window interface {
	Render() string
	GetPosition() (x, y int)
	GetSize() (width, height int)
	SetPosition(x, y int)
	SetSize(width, height int)
	HandleKey(tea.KeyMsg) tea.Cmd
	IsModal() bool
	GetTitle() string
}

// WindowManager manages layered windows with shadows
type WindowManager struct {
	windows []Window
	width   int
	height  int
}

// NewWindowManager creates a new window manager
func NewWindowManager() *WindowManager {
	return &WindowManager{
		windows: make([]Window, 0),
		width:   80,
		height:  25,
	}
}

// SetSize sets the dimensions of the window manager
func (wm *WindowManager) SetSize(width, height int) {
	wm.width = width
	wm.height = height
}

// AddWindow adds a window to the stack
func (wm *WindowManager) AddWindow(window Window) {
	wm.windows = append(wm.windows, window)
}

// CloseTopWindow removes the topmost window
func (wm *WindowManager) CloseTopWindow() {
	if len(wm.windows) > 0 {
		wm.windows = wm.windows[:len(wm.windows)-1]
	}
}

// HasActiveWindows returns true if there are any active windows
func (wm *WindowManager) HasActiveWindows() bool {
	return len(wm.windows) > 0
}

// GetTopWindow returns the topmost window, or nil if none
func (wm *WindowManager) GetTopWindow() Window {
	if len(wm.windows) == 0 {
		return nil
	}
	return wm.windows[len(wm.windows)-1]
}

// Update handles window manager updates
func (wm *WindowManager) Update(msg tea.Msg) (*WindowManager, tea.Cmd) {
	// Pass messages to the topmost window if it exists
	if topWindow := wm.GetTopWindow(); topWindow != nil {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			return wm, topWindow.HandleKey(keyMsg)
		}
	}
	return wm, nil
}

// RenderWithOverlays renders content with window overlays
func (wm *WindowManager) RenderWithOverlays(baseContent string) string {
	if len(wm.windows) == 0 {
		return baseContent
	}
	
	// Start with base content
	result := baseContent
	
	// Layer each window with shadow
	for i, window := range wm.windows {
		result = wm.overlayWindow(result, window, i < len(wm.windows)-1)
	}
	
	return result
}

// overlayWindow overlays a single window onto the content
func (wm *WindowManager) overlayWindow(content string, window Window, hasShadow bool) string {
	x, y := window.GetPosition()
	width, height := window.GetSize()
	
	// Render the window
	windowContent := window.Render()
	
	// Create shadow if not the topmost window or if specified
	if hasShadow {
		shadowX := x + 2
		shadowY := y + 1
		shadowContent := CreateShadow(width, height)
		content = wm.overlayAt(content, shadowContent, shadowX, shadowY)
	}
	
	// Overlay the window content
	return wm.overlayAt(content, windowContent, x, y)
}

// overlayAt overlays one content onto another at specified position
func (wm *WindowManager) overlayAt(base, overlay string, x, y int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")
	
	// Ensure we have enough lines in base
	for len(baseLines) < wm.height {
		baseLines = append(baseLines, repeatString(" ", wm.width))
	}
	
	// Overlay each line
	for i, overlayLine := range overlayLines {
		targetY := y + i
		if targetY >= 0 && targetY < len(baseLines) {
			baseLines[targetY] = wm.overlayLineAt(baseLines[targetY], overlayLine, x)
		}
	}
	
	return lipgloss.JoinVertical(lipgloss.Top, baseLines...)
}

// overlayLineAt overlays one line onto another at specified x position
func (wm *WindowManager) overlayLineAt(baseLine, overlayLine string, x int) string {
	baseRunes := []rune(baseLine)
	overlayRunes := []rune(overlayLine)
	
	// Ensure base line is long enough
	for len(baseRunes) < wm.width {
		baseRunes = append(baseRunes, ' ')
	}
	
	// Overlay the line
	for i, r := range overlayRunes {
		targetX := x + i
		if targetX >= 0 && targetX < len(baseRunes) {
			// Only overlay non-transparent characters
			if r != ' ' || i == 0 || i == len(overlayRunes)-1 {
				baseRunes[targetX] = r
			}
		}
	}
	
	return string(baseRunes)
}

// Dialog represents a modal dialog window
type Dialog struct {
	title    string
	content  string
	buttons  []string
	selected int
	x, y     int
	width    int
	height   int
	modal    bool
}

// NewDialog creates a new dialog
func NewDialog(title, content string, buttons []string) *Dialog {
	width := 60
	height := 15
	
	return &Dialog{
		title:    title,
		content:  content,
		buttons:  buttons,
		selected: 0,
		width:    width,
		height:   height,
		modal:    true,
	}
}

// Render implements Window interface
func (d *Dialog) Render() string {
	// Create the dialog box
	box := CreateBox(d.width, d.height-3, d.title, d.content, true)
	
	// Add buttons at the bottom
	buttonRow := d.renderButtons()
	
	// Combine box and buttons
	dialogContent := lipgloss.JoinVertical(lipgloss.Top, box, buttonRow)
	
	return DialogStyle.Width(d.width).Height(d.height).Render(dialogContent)
}

// renderButtons renders the dialog buttons
func (d *Dialog) renderButtons() string {
	if len(d.buttons) == 0 {
		return ""
	}
	
	var buttons []string
	for i, button := range d.buttons {
		style := ButtonStyle
		if i == d.selected {
			style = ButtonActiveStyle
		}
		buttons = append(buttons, style.Render(button))
	}
	
	buttonRow := lipgloss.JoinHorizontal(lipgloss.Center, buttons...)
	return lipgloss.NewStyle().Width(d.width).Align(lipgloss.Center).Render(buttonRow)
}

// GetPosition implements Window interface
func (d *Dialog) GetPosition() (int, int) {
	return d.x, d.y
}

// GetSize implements Window interface
func (d *Dialog) GetSize() (int, int) {
	return d.width, d.height
}

// SetPosition implements Window interface
func (d *Dialog) SetPosition(x, y int) {
	d.x = x
	d.y = y
}

// SetSize implements Window interface
func (d *Dialog) SetSize(width, height int) {
	d.width = width
	d.height = height
}

// HandleKey implements Window interface
func (d *Dialog) HandleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyLeft:
		if d.selected > 0 {
			d.selected--
		}
	case tea.KeyRight:
		if d.selected < len(d.buttons)-1 {
			d.selected++
		}
	case tea.KeyTab:
		d.selected = (d.selected + 1) % len(d.buttons)
	case tea.KeyShiftTab:
		d.selected = (d.selected - 1 + len(d.buttons)) % len(d.buttons)
	case tea.KeyEnter:
		// Return a command to close the dialog
		return func() tea.Msg {
			return DialogCloseMsg{ButtonIndex: d.selected, ButtonText: d.buttons[d.selected]}
		}
	case tea.KeyEsc:
		// Cancel dialog
		return func() tea.Msg {
			return DialogCloseMsg{ButtonIndex: -1, ButtonText: "Cancel"}
		}
	}
	return nil
}

// IsModal implements Window interface
func (d *Dialog) IsModal() bool {
	return d.modal
}

// GetTitle implements Window interface
func (d *Dialog) GetTitle() string {
	return d.title
}

// Center centers the dialog on screen
func (d *Dialog) Center(screenWidth, screenHeight int) {
	d.x = (screenWidth - d.width) / 2
	d.y = (screenHeight - d.height) / 2
}

// DialogCloseMsg is sent when a dialog is closed
type DialogCloseMsg struct {
	ButtonIndex int
	ButtonText  string
}

// MessageBox creates a simple message dialog
func MessageBox(title, message string) *Dialog {
	return NewDialog(title, message, []string{"OK"})
}

// ConfirmDialog creates a yes/no confirmation dialog
func ConfirmDialog(title, message string) *Dialog {
	return NewDialog(title, message, []string{"Yes", "No"})
}

// ErrorDialog creates an error message dialog
func ErrorDialog(message string) *Dialog {
	return NewDialog("Error", ErrorStyle.Render(message), []string{"OK"})
}

// InfoWindow represents an informational window (non-modal)
type InfoWindow struct {
	title   string
	content string
	x, y    int
	width   int
	height  int
}

// NewInfoWindow creates a new info window
func NewInfoWindow(title, content string, width, height int) *InfoWindow {
	return &InfoWindow{
		title:   title,
		content: content,
		width:   width,
		height:  height,
	}
}

// Render implements Window interface
func (iw *InfoWindow) Render() string {
	box := CreateBox(iw.width, iw.height, iw.title, iw.content, false)
	return WindowStyle.Width(iw.width).Height(iw.height).Render(box)
}

// GetPosition implements Window interface
func (iw *InfoWindow) GetPosition() (int, int) {
	return iw.x, iw.y
}

// GetSize implements Window interface
func (iw *InfoWindow) GetSize() (int, int) {
	return iw.width, iw.height
}

// SetPosition implements Window interface
func (iw *InfoWindow) SetPosition(x, y int) {
	iw.x = x
	iw.y = y
}

// SetSize implements Window interface
func (iw *InfoWindow) SetSize(width, height int) {
	iw.width = width
	iw.height = height
}

// HandleKey implements Window interface
func (iw *InfoWindow) HandleKey(msg tea.KeyMsg) tea.Cmd {
	// Info windows typically don't handle keys unless they become focused
	return nil
}

// IsModal implements Window interface
func (iw *InfoWindow) IsModal() bool {
	return false
}

// GetTitle implements Window interface
func (iw *InfoWindow) GetTitle() string {
	return iw.title
}