package components

import (
	"unicode/utf8"
	
	"github.com/stlalpha/vision3/pkg/goturbotui"
)

// ListBox represents a selectable list of items
type ListBox struct {
	*goturbotui.BaseView
	items         []string
	selectedIndex int
	topIndex      int
	theme         *goturbotui.Theme
	onSelect      func(index int, item string)
}

// NewListBox creates a new list box
func NewListBox(theme *goturbotui.Theme) *ListBox {
	listBox := &ListBox{
		BaseView:      goturbotui.NewBaseView(),
		items:         make([]string, 0),
		selectedIndex: 0,
		topIndex:      0,
		theme:         theme,
	}
	
	listBox.SetCanFocus(true)
	return listBox
}

// SetItems sets the list items
func (lb *ListBox) SetItems(items []string) {
	lb.items = items
	lb.selectedIndex = 0
	lb.topIndex = 0
	
	// Ensure selection is valid
	if len(lb.items) == 0 {
		lb.selectedIndex = -1
	}
}

// GetItems returns the list items
func (lb *ListBox) GetItems() []string {
	return lb.items
}

// AddItem adds an item to the list
func (lb *ListBox) AddItem(item string) {
	lb.items = append(lb.items, item)
	if lb.selectedIndex == -1 && len(lb.items) > 0 {
		lb.selectedIndex = 0
	}
}

// RemoveItem removes an item from the list
func (lb *ListBox) RemoveItem(index int) {
	if index < 0 || index >= len(lb.items) {
		return
	}
	
	lb.items = append(lb.items[:index], lb.items[index+1:]...)
	
	// Adjust selection
	if lb.selectedIndex >= len(lb.items) {
		lb.selectedIndex = len(lb.items) - 1
	}
	if lb.selectedIndex < 0 && len(lb.items) > 0 {
		lb.selectedIndex = 0
	}
	if len(lb.items) == 0 {
		lb.selectedIndex = -1
	}
}

// GetSelectedIndex returns the currently selected index
func (lb *ListBox) GetSelectedIndex() int {
	return lb.selectedIndex
}

// GetSelectedItem returns the currently selected item
func (lb *ListBox) GetSelectedItem() string {
	if lb.selectedIndex >= 0 && lb.selectedIndex < len(lb.items) {
		return lb.items[lb.selectedIndex]
	}
	return ""
}

// SetSelectedIndex sets the selected index
func (lb *ListBox) SetSelectedIndex(index int) {
	if index < 0 || index >= len(lb.items) {
		return
	}
	
	lb.selectedIndex = index
	lb.ensureVisible()
}

// SetOnSelect sets the selection callback
func (lb *ListBox) SetOnSelect(callback func(index int, item string)) {
	lb.onSelect = callback
}

// ensureVisible ensures the selected item is visible
func (lb *ListBox) ensureVisible() {
	if lb.selectedIndex < 0 {
		return
	}
	
	bounds := lb.GetBounds()
	visibleHeight := bounds.H
	
	// Adjust top index to keep selection visible
	if lb.selectedIndex < lb.topIndex {
		lb.topIndex = lb.selectedIndex
	} else if lb.selectedIndex >= lb.topIndex+visibleHeight {
		lb.topIndex = lb.selectedIndex - visibleHeight + 1
	}
	
	// Ensure top index is valid
	if lb.topIndex < 0 {
		lb.topIndex = 0
	}
	maxTop := len(lb.items) - visibleHeight
	if maxTop < 0 {
		maxTop = 0
	}
	if lb.topIndex > maxTop {
		lb.topIndex = maxTop
	}
}

// Draw renders the list box
func (lb *ListBox) Draw(canvas goturbotui.Canvas) {
	if !lb.IsVisible() {
		return
	}
	
	bounds := lb.GetBounds()
	theme := lb.theme
	if theme == nil {
		theme = goturbotui.DefaultTurboTheme()
	}
	
	// Clear background
	canvas.Fill(bounds, ' ', theme.ListBox)
	
	// Draw items
	for i := 0; i < bounds.H && (lb.topIndex+i) < len(lb.items); i++ {
		itemIndex := lb.topIndex + i
		item := lb.items[itemIndex]
		y := bounds.Y + i
		
		var style goturbotui.Style
		var prefix string
		
		if itemIndex == lb.selectedIndex {
			if lb.IsFocused() {
				style = theme.ListBoxSelected
			} else {
				style = theme.ListBoxFocused
			}
			prefix = " ► "
		} else {
			style = theme.ListBox
			prefix = "   "
		}
		
		// Truncate item if too long
		maxLen := bounds.W - utf8.RuneCountInString(prefix)
		if utf8.RuneCountInString(item) > maxLen {
			item = string([]rune(item)[:maxLen])
		}
		
		// Draw prefix and item
		canvas.SetString(bounds.X, y, prefix+item, style)
		
		// Fill remainder of line with spaces
		remaining := bounds.W - utf8.RuneCountInString(prefix) - utf8.RuneCountInString(item)
		if remaining > 0 {
			spaces := make([]rune, remaining)
			for i := range spaces {
				spaces[i] = ' '
			}
			canvas.SetString(bounds.X+utf8.RuneCountInString(prefix)+utf8.RuneCountInString(item), y, string(spaces), style)
		}
	}
}

// HandleEvent handles list box events
func (lb *ListBox) HandleEvent(event goturbotui.Event) bool {
	if !lb.IsVisible() || !lb.CanFocus() {
		return false
	}
	
	if event.Type == goturbotui.EventKey {
		switch event.Key.Code {
		case goturbotui.KeyUp:
			lb.selectPrevious()
			return true
			
		case goturbotui.KeyDown:
			lb.selectNext()
			return true
			
		case goturbotui.KeyHome:
			lb.SetSelectedIndex(0)
			return true
			
		case goturbotui.KeyEnd:
			if len(lb.items) > 0 {
				lb.SetSelectedIndex(len(lb.items) - 1)
			}
			return true
			
		case goturbotui.KeyPageUp:
			bounds := lb.GetBounds()
			newIndex := lb.selectedIndex - bounds.H
			if newIndex < 0 {
				newIndex = 0
			}
			lb.SetSelectedIndex(newIndex)
			return true
			
		case goturbotui.KeyPageDown:
			bounds := lb.GetBounds()
			newIndex := lb.selectedIndex + bounds.H
			if newIndex >= len(lb.items) {
				newIndex = len(lb.items) - 1
			}
			lb.SetSelectedIndex(newIndex)
			return true
			
		case goturbotui.KeyEnter:
			if lb.onSelect != nil && lb.selectedIndex >= 0 {
				lb.onSelect(lb.selectedIndex, lb.GetSelectedItem())
			}
			return true
		}
	}
	
	return false
}

// selectNext moves selection to the next item
func (lb *ListBox) selectNext() {
	if len(lb.items) == 0 {
		return
	}
	
	if lb.selectedIndex < len(lb.items)-1 {
		lb.selectedIndex++
		lb.ensureVisible()
	}
}

// selectPrevious moves selection to the previous item
func (lb *ListBox) selectPrevious() {
	if len(lb.items) == 0 {
		return
	}
	
	if lb.selectedIndex > 0 {
		lb.selectedIndex--
		lb.ensureVisible()
	}
}