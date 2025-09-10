package components

import (
	"github.com/stlalpha/vision3/pkg/goturbotui"
)

// MenuBar represents a horizontal menu bar
type MenuBar struct {
	*goturbotui.BaseView
	items         []MenuItem
	selectedIndex int
	theme         *goturbotui.Theme
	onSelect      func(index int, item MenuItem)
}

// MenuItem represents a menu item
type MenuItem struct {
	Text     string
	Hotkey   rune
	Action   func()
	Enabled  bool
}

// NewMenuBar creates a new menu bar
func NewMenuBar(theme *goturbotui.Theme) *MenuBar {
	menuBar := &MenuBar{
		BaseView:      goturbotui.NewBaseView(),
		items:         make([]MenuItem, 0),
		selectedIndex: -1,
		theme:         theme,
	}
	
	menuBar.SetCanFocus(true)
	return menuBar
}

// AddItem adds a menu item
func (mb *MenuBar) AddItem(item MenuItem) {
	mb.items = append(mb.items, item)
}

// SetItems sets all menu items
func (mb *MenuBar) SetItems(items []MenuItem) {
	mb.items = items
	mb.selectedIndex = -1
}

// GetItems returns all menu items
func (mb *MenuBar) GetItems() []MenuItem {
	return mb.items
}

// SetSelectedIndex sets the selected menu index
func (mb *MenuBar) SetSelectedIndex(index int) {
	if index < -1 || index >= len(mb.items) {
		return
	}
	mb.selectedIndex = index
}

// GetSelectedIndex returns the selected menu index
func (mb *MenuBar) GetSelectedIndex() int {
	return mb.selectedIndex
}

// SetOnSelect sets the selection callback
func (mb *MenuBar) SetOnSelect(callback func(index int, item MenuItem)) {
	mb.onSelect = callback
}

// Draw renders the menu bar
func (mb *MenuBar) Draw(canvas goturbotui.Canvas) {
	if !mb.IsVisible() {
		return
	}
	
	bounds := mb.GetBounds()
	theme := mb.theme
	if theme == nil {
		theme = goturbotui.DefaultTurboTheme()
	}
	
	// Clear background
	canvas.Fill(bounds, ' ', theme.MenuBar)
	
	// Draw menu items
	x := bounds.X + 1 // Start with a space
	for i, item := range mb.items {
		if x >= bounds.Right() {
			break
		}
		
		var style goturbotui.Style
		if i == mb.selectedIndex {
			style = theme.MenuBarSelected
		} else {
			style = theme.MenuBar
		}
		
		// Add spacing
		itemText := " " + item.Text + " "
		
		// Check if item fits
		if x+len(itemText) <= bounds.Right() {
			canvas.SetString(x, bounds.Y, itemText, style)
		}
		
		x += len(itemText)
	}
}

// HandleEvent handles menu bar events
func (mb *MenuBar) HandleEvent(event goturbotui.Event) bool {
	if !mb.IsVisible() || !mb.CanFocus() {
		return false
	}
	
	if event.Type == goturbotui.EventKey {
		switch event.Key.Code {
		case goturbotui.KeyLeft:
			mb.selectPrevious()
			return true
			
		case goturbotui.KeyRight:
			mb.selectNext()
			return true
			
		case goturbotui.KeyEnter:
			if mb.selectedIndex >= 0 && mb.selectedIndex < len(mb.items) {
				item := mb.items[mb.selectedIndex]
				if item.Enabled && item.Action != nil {
					item.Action()
				}
				if mb.onSelect != nil {
					mb.onSelect(mb.selectedIndex, item)
				}
			}
			return true
		}
		
		// Check for hotkeys
		if event.Key.Modifiers == goturbotui.ModAlt && event.Rune != 0 {
			for i, item := range mb.items {
				if item.Enabled && item.Hotkey == event.Rune {
					mb.selectedIndex = i
					if item.Action != nil {
						item.Action()
					}
					if mb.onSelect != nil {
						mb.onSelect(i, item)
					}
					return true
				}
			}
		}
	}
	
	return false
}

// selectNext moves to the next menu item
func (mb *MenuBar) selectNext() {
	if len(mb.items) == 0 {
		return
	}
	
	if mb.selectedIndex < 0 {
		mb.selectedIndex = 0
	} else if mb.selectedIndex < len(mb.items)-1 {
		mb.selectedIndex++
	} else {
		mb.selectedIndex = 0 // Wrap around
	}
}

// selectPrevious moves to the previous menu item
func (mb *MenuBar) selectPrevious() {
	if len(mb.items) == 0 {
		return
	}
	
	if mb.selectedIndex < 0 {
		mb.selectedIndex = len(mb.items) - 1
	} else if mb.selectedIndex > 0 {
		mb.selectedIndex--
	} else {
		mb.selectedIndex = len(mb.items) - 1 // Wrap around
	}
}