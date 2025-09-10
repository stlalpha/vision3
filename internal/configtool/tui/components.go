package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

// ListBox represents a list selection component
type ListBox struct {
	items        []ListItem
	selected     int
	topIndex     int
	width        int
	height       int
	title        string
	focused      bool
	multiSelect  bool
	showBorder   bool
}

// ListItem represents an item in the list box
type ListItem struct {
	Text        string
	Value       interface{}
	Selected    bool
	Enabled     bool
	Icon        string
	Description string
}

// NewListBox creates a new list box
func NewListBox(title string, width, height int) *ListBox {
	return &ListBox{
		items:       make([]ListItem, 0),
		selected:    0,
		topIndex:    0,
		width:       width,
		height:      height,
		title:       title,
		focused:     false,
		multiSelect: false,
		showBorder:  true,
	}
}

// AddItem adds an item to the list box
func (lb *ListBox) AddItem(item ListItem) {
	lb.items = append(lb.items, item)
}

// SetItems sets all items in the list box
func (lb *ListBox) SetItems(items []ListItem) {
	lb.items = items
	lb.selected = 0
	lb.topIndex = 0
}

// GetSelected returns the currently selected item
func (lb *ListBox) GetSelected() (ListItem, bool) {
	if lb.selected >= 0 && lb.selected < len(lb.items) {
		return lb.items[lb.selected], true
	}
	return ListItem{}, false
}

// GetSelectedItems returns all selected items (for multi-select)
func (lb *ListBox) GetSelectedItems() []ListItem {
	var selected []ListItem
	for _, item := range lb.items {
		if item.Selected {
			selected = append(selected, item)
		}
	}
	return selected
}

// SetSelected sets the selected index
func (lb *ListBox) SetSelected(index int) {
	if index >= 0 && index < len(lb.items) {
		lb.selected = index
		lb.ensureVisible()
	}
}

// ensureVisible ensures the selected item is visible
func (lb *ListBox) ensureVisible() {
	visibleHeight := lb.height
	if lb.showBorder {
		visibleHeight -= 2 // Account for border
	}
	if lb.title != "" {
		visibleHeight -= 1 // Account for title
	}
	
	if lb.selected < lb.topIndex {
		lb.topIndex = lb.selected
	} else if lb.selected >= lb.topIndex+visibleHeight {
		lb.topIndex = lb.selected - visibleHeight + 1
	}
	
	if lb.topIndex < 0 {
		lb.topIndex = 0
	}
}

// SetFocus sets the focus state
func (lb *ListBox) SetFocus(focused bool) {
	lb.focused = focused
}

// SetMultiSelect enables/disables multi-select mode
func (lb *ListBox) SetMultiSelect(multiSelect bool) {
	lb.multiSelect = multiSelect
}

// SetShowBorder enables/disables border display
func (lb *ListBox) SetShowBorder(showBorder bool) {
	lb.showBorder = showBorder
}

// HandleKey processes keyboard input
func (lb *ListBox) HandleKey(msg tea.KeyMsg) tea.Cmd {
	if len(lb.items) == 0 {
		return nil
	}
	
	switch msg.Type {
	case tea.KeyUp:
		if lb.selected > 0 {
			lb.selected--
			lb.ensureVisible()
		}
	case tea.KeyDown:
		if lb.selected < len(lb.items)-1 {
			lb.selected++
			lb.ensureVisible()
		}
	case tea.KeyHome:
		lb.selected = 0
		lb.topIndex = 0
	case tea.KeyEnd:
		lb.selected = len(lb.items) - 1
		lb.ensureVisible()
	case tea.KeyPgUp:
		visibleHeight := lb.height - 2
		lb.selected -= visibleHeight
		if lb.selected < 0 {
			lb.selected = 0
		}
		lb.ensureVisible()
	case tea.KeyPgDown:
		visibleHeight := lb.height - 2
		lb.selected += visibleHeight
		if lb.selected >= len(lb.items) {
			lb.selected = len(lb.items) - 1
		}
		lb.ensureVisible()
	case tea.KeySpace:
		if lb.multiSelect && lb.selected < len(lb.items) {
			lb.items[lb.selected].Selected = !lb.items[lb.selected].Selected
		}
	case tea.KeyEnter:
		return func() tea.Msg {
			return ListSelectMsg{
				Index:    lb.selected,
				Item:     lb.items[lb.selected],
				ListBox:  lb,
			}
		}
	}
	
	return nil
}

// Render renders the list box
func (lb *ListBox) Render() string {
	if lb.showBorder {
		return lb.renderWithBorder()
	} else {
		return lb.renderContent()
	}
}

// renderWithBorder renders the list box with a border
func (lb *ListBox) renderWithBorder() string {
	content := lb.renderContent()
	
	style := ListBoxStyle.Width(lb.width).Height(lb.height)
	if lb.focused {
		style = style.BorderForeground(ColorHighlight)
	}
	
	if lb.title != "" {
		// For now, just render with border - title can be added later
		return style.Render(content)
	} else {
		return style.Render(content)
	}
}

// renderContent renders the list box content
func (lb *ListBox) renderContent() string {
	if len(lb.items) == 0 {
		emptyMsg := "No items"
		return lipgloss.NewStyle().
			Width(lb.width-2).
			Height(lb.height-2).
			Align(lipgloss.Center, lipgloss.Center).
			Render(emptyMsg)
	}
	
	var lines []string
	contentHeight := lb.height
	if lb.showBorder {
		contentHeight -= 2
	}
	if lb.title != "" {
		contentHeight -= 1
	}
	
	// Render visible items
	endIndex := lb.topIndex + contentHeight
	if endIndex > len(lb.items) {
		endIndex = len(lb.items)
	}
	
	for i := lb.topIndex; i < endIndex; i++ {
		item := lb.items[i]
		line := lb.renderItem(item, i == lb.selected)
		lines = append(lines, line)
	}
	
	// Fill remaining space
	for len(lines) < contentHeight {
		lines = append(lines, strings.Repeat(" ", lb.width-2))
	}
	
	return lipgloss.JoinVertical(lipgloss.Top, lines...)
}

// renderItem renders a single list item
func (lb *ListBox) renderItem(item ListItem, isSelected bool) string {
	text := item.Text
	contentWidth := lb.width
	if lb.showBorder {
		contentWidth -= 2
	}
	
	// Add icon if present
	if item.Icon != "" {
		text = item.Icon + " " + text
	}
	
	// Add multi-select indicator
	if lb.multiSelect {
		if item.Selected {
			text = CheckMark + " " + text
		} else {
			text = "  " + text
		}
	}
	
	// Truncate if too long
	if len(text) > contentWidth {
		text = text[:contentWidth-3] + "..."
	}
	
	// Pad to full width
	for len(text) < contentWidth {
		text += " "
	}
	
	// Apply styling
	if isSelected {
		if lb.focused {
			return ListItemSelectedStyle.Render(text)
		} else {
			// Selected but not focused - different style
			return lipgloss.NewStyle().
				Background(ColorDialog).
				Foreground(ColorText).
				Render(text)
		}
	} else if !item.Enabled {
		return lipgloss.NewStyle().
			Foreground(ColorDisabled).
			Background(ColorText).
			Render(text)
	} else {
		return ListItemStyle.Render(text)
	}
}

// InputField represents a text input component
type InputField struct {
	value       string
	placeholder string
	cursor      int
	focused     bool
	width       int
	maxLength   int
	masked      bool
	maskChar    rune
	title       string
}

// NewInputField creates a new input field
func NewInputField(title, placeholder string, width int) *InputField {
	return &InputField{
		value:       "",
		placeholder: placeholder,
		cursor:      0,
		focused:     false,
		width:       width,
		maxLength:   -1, // No limit
		masked:      false,
		maskChar:    '*',
		title:       title,
	}
}

// SetValue sets the input field value
func (inp *InputField) SetValue(value string) {
	inp.value = value
	if inp.cursor > len(inp.value) {
		inp.cursor = len(inp.value)
	}
}

// GetValue returns the input field value
func (inp *InputField) GetValue() string {
	return inp.value
}

// SetFocus sets the focus state
func (inp *InputField) SetFocus(focused bool) {
	inp.focused = focused
}

// SetMaxLength sets the maximum length
func (inp *InputField) SetMaxLength(maxLength int) {
	inp.maxLength = maxLength
}

// SetMasked enables/disables password masking
func (inp *InputField) SetMasked(masked bool) {
	inp.masked = masked
}

// HandleKey processes keyboard input
func (inp *InputField) HandleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyLeft:
		if inp.cursor > 0 {
			inp.cursor--
		}
	case tea.KeyRight:
		if inp.cursor < len(inp.value) {
			inp.cursor++
		}
	case tea.KeyHome, tea.KeyCtrlA:
		inp.cursor = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		inp.cursor = len(inp.value)
	case tea.KeyBackspace:
		if inp.cursor > 0 {
			inp.value = inp.value[:inp.cursor-1] + inp.value[inp.cursor:]
			inp.cursor--
		}
	case tea.KeyDelete, tea.KeyCtrlD:
		if inp.cursor < len(inp.value) {
			inp.value = inp.value[:inp.cursor] + inp.value[inp.cursor+1:]
		}
	case tea.KeyCtrlK:
		inp.value = inp.value[:inp.cursor]
	case tea.KeyCtrlU:
		inp.value = inp.value[inp.cursor:]
		inp.cursor = 0
	default:
		if len(msg.Runes) > 0 {
			char := msg.Runes[0]
			if char >= 32 && char <= 126 { // Printable ASCII
				if inp.maxLength == -1 || len(inp.value) < inp.maxLength {
					inp.value = inp.value[:inp.cursor] + string(char) + inp.value[inp.cursor:]
					inp.cursor++
				}
			}
		}
	}
	return nil
}

// Render renders the input field
func (inp *InputField) Render() string {
	displayValue := inp.value
	
	// Apply masking if enabled
	if inp.masked && len(displayValue) > 0 {
		displayValue = strings.Repeat(string(inp.maskChar), len(displayValue))
	}
	
	// Show placeholder if empty
	if len(inp.value) == 0 && inp.placeholder != "" {
		displayValue = inp.placeholder
	}
	
	// Create the input content
	content := displayValue
	
	// Add cursor if focused
	if inp.focused {
		if inp.cursor <= len(displayValue) {
			if inp.cursor == len(displayValue) {
				content = displayValue + "█"
			} else {
				content = displayValue[:inp.cursor] + "█" + displayValue[inp.cursor+1:]
			}
		}
	}
	
	// Pad or truncate to fit width
	contentWidth := inp.width - 2 // Account for padding
	if len(content) > contentWidth {
		content = content[:contentWidth]
	}
	for len(content) < contentWidth {
		content += " "
	}
	
	// Apply styling
	style := InputStyle
	if inp.focused {
		style = InputFocusStyle
	}
	
	result := style.Width(inp.width).Render(content)
	
	// Add title if present
	if inp.title != "" {
		title := lipgloss.NewStyle().Foreground(ColorText).Render(inp.title)
		result = lipgloss.JoinVertical(lipgloss.Left, title, result)
	}
	
	return result
}

// Button represents a clickable button component
type Button struct {
	text     string
	action   string
	focused  bool
	enabled  bool
	width    int
	isCancel bool
}

// NewButton creates a new button
func NewButton(text, action string) *Button {
	return &Button{
		text:     text,
		action:   action,
		focused:  false,
		enabled:  true,
		width:    len(text) + 4,
		isCancel: false,
	}
}

// SetText sets the button text
func (btn *Button) SetText(text string) {
	btn.text = text
	btn.width = len(text) + 4
}

// SetFocus sets the focus state
func (btn *Button) SetFocus(focused bool) {
	btn.focused = focused
}

// SetEnabled sets the enabled state
func (btn *Button) SetEnabled(enabled bool) {
	btn.enabled = enabled
}

// SetWidth sets the button width
func (btn *Button) SetWidth(width int) {
	btn.width = width
}

// SetCancel marks this as a cancel button
func (btn *Button) SetCancel(isCancel bool) {
	btn.isCancel = isCancel
}

// HandleKey processes keyboard input
func (btn *Button) HandleKey(msg tea.KeyMsg) tea.Cmd {
	if !btn.enabled {
		return nil
	}
	
	switch msg.Type {
	case tea.KeyEnter, tea.KeySpace:
		return func() tea.Msg {
			return ButtonPressMsg{
				Action: btn.action,
				Button: btn,
			}
		}
	case tea.KeyEsc:
		if btn.isCancel {
			return func() tea.Msg {
				return ButtonPressMsg{
					Action: btn.action,
					Button: btn,
				}
			}
		}
	}
	return nil
}

// Render renders the button
func (btn *Button) Render() string {
	style := ButtonStyle
	if btn.focused && btn.enabled {
		style = ButtonActiveStyle
	} else if !btn.enabled {
		style = lipgloss.NewStyle().
			Background(ColorDisabled).
			Foreground(ColorText).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorDisabled).
			Padding(0, 2).
			Margin(0, 1)
	}
	
	return style.Width(btn.width).Render(btn.text)
}

// Message types for components
type ListSelectMsg struct {
	Index   int
	Item    ListItem
	ListBox *ListBox
}

type ButtonPressMsg struct {
	Action string
	Button *Button
}

type InputChangeMsg struct {
	Value      string
	InputField *InputField
}

// Helper functions for creating common list items
func NewListItem(text string, value interface{}) ListItem {
	return ListItem{
		Text:    text,
		Value:   value,
		Enabled: true,
	}
}

func NewListItemWithIcon(text, icon string, value interface{}) ListItem {
	return ListItem{
		Text:    text,
		Icon:    icon,
		Value:   value,
		Enabled: true,
	}
}

func NewSeparatorItem() ListItem {
	return ListItem{
		Text:    strings.Repeat(BoxHorizontal, 20),
		Enabled: false,
	}
}

// Helper functions for creating common buttons
func NewOKButton() *Button {
	return NewButton("OK", "ok")
}

func NewCancelButton() *Button {
	btn := NewButton("Cancel", "cancel")
	btn.SetCancel(true)
	return btn
}

func NewYesButton() *Button {
	return NewButton("Yes", "yes")
}

func NewNoButton() *Button {
	btn := NewButton("No", "no")
	btn.SetCancel(true)
	return btn
}