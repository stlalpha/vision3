package components

import (
	"github.com/stlalpha/vision3/pkg/goturbotui"
)

// StatusBar represents a status bar with function key hints
type StatusBar struct {
	*goturbotui.BaseView
	items   []StatusItem
	message string
	theme   *goturbotui.Theme
}

// StatusItem represents a status bar item (like F1=Help)
type StatusItem struct {
	Key    string
	Text   string
	Action func()
}

// NewStatusBar creates a new status bar
func NewStatusBar(theme *goturbotui.Theme) *StatusBar {
	return &StatusBar{
		BaseView: goturbotui.NewBaseView(),
		items:    make([]StatusItem, 0),
		theme:    theme,
	}
}

// AddItem adds a status item
func (sb *StatusBar) AddItem(item StatusItem) {
	sb.items = append(sb.items, item)
}

// SetItems sets all status items
func (sb *StatusBar) SetItems(items []StatusItem) {
	sb.items = items
}

// GetItems returns all status items
func (sb *StatusBar) GetItems() []StatusItem {
	return sb.items
}

// SetMessage sets the status message
func (sb *StatusBar) SetMessage(message string) {
	sb.message = message
}

// GetMessage returns the current status message
func (sb *StatusBar) GetMessage() string {
	return sb.message
}

// Draw renders the status bar
func (sb *StatusBar) Draw(canvas goturbotui.Canvas) {
	if !sb.IsVisible() {
		return
	}
	
	bounds := sb.GetBounds()
	theme := sb.theme
	if theme == nil {
		theme = goturbotui.DefaultTurboTheme()
	}
	
	// Clear background
	canvas.Fill(bounds, ' ', theme.StatusBar)
	
	// Draw status items (function keys)
	x := bounds.X
	for _, item := range sb.items {
		if x >= bounds.Right() {
			break
		}
		
		itemText := item.Key + "=" + item.Text + " "
		
		// Check if item fits
		if x+len(itemText) <= bounds.Right() {
			canvas.SetString(x, bounds.Y, itemText, theme.StatusBar)
			x += len(itemText)
		}
	}
	
	// Draw message on the right side
	if sb.message != "" {
		messageLen := len(sb.message)
		if messageLen <= bounds.W {
			messageX := bounds.Right() - messageLen
			if messageX > x { // Make sure it doesn't overlap with items
				canvas.SetString(messageX, bounds.Y, sb.message, theme.StatusBar)
			}
		}
	}
}

// HandleEvent handles status bar events (function keys)
func (sb *StatusBar) HandleEvent(event goturbotui.Event) bool {
	if !sb.IsVisible() {
		return false
	}
	
	if event.Type == goturbotui.EventKey {
		// Handle function keys
		keyName := ""
		switch event.Key.Code {
		case goturbotui.KeyF1:
			keyName = "F1"
		case goturbotui.KeyF2:
			keyName = "F2"
		case goturbotui.KeyF3:
			keyName = "F3"
		case goturbotui.KeyF4:
			keyName = "F4"
		case goturbotui.KeyF5:
			keyName = "F5"
		case goturbotui.KeyF6:
			keyName = "F6"
		case goturbotui.KeyF7:
			keyName = "F7"
		case goturbotui.KeyF8:
			keyName = "F8"
		case goturbotui.KeyF9:
			keyName = "F9"
		case goturbotui.KeyF10:
			keyName = "F10"
		case goturbotui.KeyF11:
			keyName = "F11"
		case goturbotui.KeyF12:
			keyName = "F12"
		}
		
		if keyName != "" {
			for _, item := range sb.items {
				if item.Key == keyName && item.Action != nil {
					item.Action()
					return true
				}
			}
		}
	}
	
	return false
}