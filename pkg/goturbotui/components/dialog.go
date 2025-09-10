package components

import (
	"fmt"
	"github.com/stlalpha/vision3/pkg/goturbotui"
)

// Dialog represents a modal dialog box
type Dialog struct {
	*goturbotui.BaseContainer
	title       string
	theme       *goturbotui.Theme
	shadowRect  goturbotui.Rect
	contentRect goturbotui.Rect
}

// NewDialog creates a new dialog with the specified title
func NewDialog(title string, width, height int, theme *goturbotui.Theme) *Dialog {
	dialog := &Dialog{
		BaseContainer: goturbotui.NewBaseContainer(),
		title:         title,
		theme:         theme,
	}
	
	// Set initial bounds (will be centered later)
	dialog.SetBounds(goturbotui.NewRect(0, 0, width, height))
	dialog.SetCanFocus(true)
	
	return dialog
}

// SetTitle sets the dialog title
func (d *Dialog) SetTitle(title string) {
	d.title = title
}

// GetTitle returns the dialog title
func (d *Dialog) GetTitle() string {
	return d.title
}

// Center centers the dialog within the specified area
func (d *Dialog) Center(parentWidth, parentHeight int) {
	bounds := d.GetBounds()
	
	// Calculate centered position
	x := (parentWidth - bounds.W) / 2
	y := (parentHeight - bounds.H) / 2
	
	// Ensure dialog stays within bounds
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	
	// Set dialog position
	d.SetBounds(goturbotui.NewRect(x, y, bounds.W, bounds.H))
	
	// Calculate shadow rectangle (offset by 1)
	d.shadowRect = goturbotui.NewRect(x+1, y+1, bounds.W, bounds.H)
	
	// Calculate content rectangle (inside the frame)
	d.contentRect = goturbotui.NewRect(x+1, y+1, bounds.W-2, bounds.H-2)
	
	// Update child bounds
	d.updateChildBounds()
}

// Dialog implements the View interface through BaseContainer
var _ goturbotui.View = (*Dialog)(nil)

// Draw renders the dialog
func (d *Dialog) Draw(canvas goturbotui.Canvas) {
	if !d.IsVisible() {
		return
	}
	
	bounds := d.GetBounds()
	theme := d.theme
	if theme == nil {
		theme = goturbotui.DefaultTurboTheme()
	}
	
	// Draw shadow first
	if !d.shadowRect.IsEmpty() {
		canvas.Fill(d.shadowRect, '░', theme.DialogShadow)
	}
	
	// Draw dialog background
	canvas.Fill(bounds, ' ', theme.DialogFrame)
	
	// Draw border with title
	if d.title != "" {
		canvas.DrawBoxWithTitle(bounds, d.title, theme.DialogFrame)
	} else {
		canvas.DrawBox(bounds, theme.DialogFrame)
	}
	
	// Draw children
	d.BaseContainer.Draw(canvas)
}

// SetBounds overrides to update internal rectangles
func (d *Dialog) SetBounds(bounds goturbotui.Rect) {
	d.BaseContainer.SetBounds(bounds)
	d.updateChildBounds()
}

// updateChildBounds updates the bounds of all child views
func (d *Dialog) updateChildBounds() {
	// Don't auto-adjust child bounds - let them position themselves
	// This allows manual positioning within the dialog
}

// GetContentBounds returns the usable content area inside the dialog
func (d *Dialog) GetContentBounds() goturbotui.Rect {
	return d.GetBounds().Inner(1)
}

// HandleEvent handles dialog events
func (d *Dialog) HandleEvent(event goturbotui.Event) bool {
	if !d.IsVisible() {
		return false
	}
	
	fmt.Printf("DEBUG: Dialog.HandleEvent: event.Key.Code=%d, focused=%v\n", event.Key.Code, d.GetFocused() != nil)
	
	// Don't handle escape here - let parent handle dialog closing
	// if event.Type == goturbotui.EventKey && event.Key.Code == goturbotui.KeyEscape {
	//     return true  
	// }
	
	// Pass to children
	result := d.BaseContainer.HandleEvent(event)
	fmt.Printf("DEBUG: Dialog.BaseContainer.HandleEvent returned %v\n", result)
	return result
}