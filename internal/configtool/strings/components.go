package strings

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PaneType represents the type of pane
type PaneType int

const (
	PaneCategories PaneType = iota
	PaneStringList
	PaneEditor
	PanePreview
)

// ActivePane tracks which pane is currently active
type ActivePane struct {
	Current PaneType
	Count   int
}

// Next moves to the next pane
func (ap *ActivePane) Next() {
	ap.Current = PaneType((int(ap.Current) + 1) % ap.Count)
}

// Prev moves to the previous pane
func (ap *ActivePane) Prev() {
	current := int(ap.Current)
	current--
	if current < 0 {
		current = ap.Count - 1
	}
	ap.Current = PaneType(current)
}

// CategoryItem represents a category in the category list
type CategoryItem struct {
	Name        string
	Desc        string
	Count       int
}

func (i CategoryItem) FilterValue() string { return i.Name }
func (i CategoryItem) Title() string       { return i.Name }
func (i CategoryItem) Description() string { 
	return fmt.Sprintf("%s (%d strings)", i.Desc, i.Count) 
}

// StringItem represents a string field in the string list
type StringItem struct {
	Field StringField
}

func (i StringItem) FilterValue() string { return i.Field.DisplayName + " " + i.Field.Value }
func (i StringItem) Title() string       { return i.Field.DisplayName }
func (i StringItem) Description() string { return i.Field.Value }

// CategoryPane manages the category selection pane
type CategoryPane struct {
	list     list.Model
	manager  *StringManager
	width    int
	height   int
	focused  bool
}

// NewCategoryPane creates a new category pane
func NewCategoryPane(manager *StringManager) *CategoryPane {
	categories := manager.GetCategories()
	items := make([]list.Item, len(categories))
	
	for i, cat := range categories {
		items[i] = CategoryItem{
			Name:  cat.Name,
			Desc:  cat.Description,
			Count: len(cat.Fields),
		}
	}
	
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Categories"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	
	return &CategoryPane{
		list:    l,
		manager: manager,
		width:   25,
		height:  20,
		focused: false,
	}
}

// Init initializes the category pane
func (cp *CategoryPane) Init() tea.Cmd {
	return nil
}

// Update handles category pane updates
func (cp *CategoryPane) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	cp.list, cmd = cp.list.Update(msg)
	return cmd
}

// View renders the category pane
func (cp *CategoryPane) View(styles *StyleSet) string {
	cp.list.SetSize(cp.width-2, cp.height-2)
	
	var style lipgloss.Style
	if cp.focused {
		style = styles.ActivePane
	} else {
		style = styles.InactivePane
	}
	
	return style.
		Width(cp.width).
		Height(cp.height).
		Render(cp.list.View())
}

// SetSize sets the pane dimensions
func (cp *CategoryPane) SetSize(width, height int) {
	cp.width = width
	cp.height = height
}

// SetFocused sets the focus state
func (cp *CategoryPane) SetFocused(focused bool) {
	cp.focused = focused
}

// GetSelectedCategory returns the currently selected category
func (cp *CategoryPane) GetSelectedCategory() string {
	if item, ok := cp.list.SelectedItem().(CategoryItem); ok {
		return item.Name
	}
	return ""
}

// StringListPane manages the string list pane
type StringListPane struct {
	list            list.Model
	manager         *StringManager
	currentCategory string
	width           int
	height          int
	focused         bool
	searchMode      bool
	searchInput     textinput.Model
	searchResults   []StringField
}

// NewStringListPane creates a new string list pane
func NewStringListPane(manager *StringManager) *StringListPane {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Strings"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	
	// Setup search input
	ti := textinput.New()
	ti.Placeholder = "Search strings..."
	ti.CharLimit = 50
	ti.Width = 20
	
	return &StringListPane{
		list:        l,
		manager:     manager,
		width:       40,
		height:      20,
		focused:     false,
		searchMode:  false,
		searchInput: ti,
	}
}

// Init initializes the string list pane
func (slp *StringListPane) Init() tea.Cmd {
	return nil
}

// Update handles string list pane updates
func (slp *StringListPane) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	
	if slp.searchMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEnter:
				slp.performSearch()
				slp.searchMode = false
				return nil
			case tea.KeyEsc:
				slp.searchMode = false
				slp.searchInput.SetValue("")
				slp.loadCategoryStrings(slp.currentCategory)
				return nil
			}
		}
		
		slp.searchInput, cmd = slp.searchInput.Update(msg)
		return cmd
	}
	
	slp.list, cmd = slp.list.Update(msg)
	return cmd
}

// View renders the string list pane
func (slp *StringListPane) View(styles *StyleSet) string {
	slp.list.SetSize(slp.width-2, slp.height-2)
	
	var style lipgloss.Style
	if slp.focused {
		style = styles.ActivePane
	} else {
		style = styles.InactivePane
	}
	
	content := slp.list.View()
	
	// Add search box if in search mode
	if slp.searchMode {
		searchBox := styles.SearchBox.Width(slp.width - 4).Render(slp.searchInput.View())
		content = lipgloss.JoinVertical(lipgloss.Left, searchBox, content)
	}
	
	return style.
		Width(slp.width).
		Height(slp.height).
		Render(content)
}

// SetSize sets the pane dimensions
func (slp *StringListPane) SetSize(width, height int) {
	slp.width = width
	slp.height = height
}

// SetFocused sets the focus state
func (slp *StringListPane) SetFocused(focused bool) {
	slp.focused = focused
	if focused && slp.searchMode {
		slp.searchInput.Focus()
	} else {
		slp.searchInput.Blur()
	}
}

// LoadCategory loads strings for a specific category
func (slp *StringListPane) LoadCategory(categoryName string) {
	slp.currentCategory = categoryName
	slp.loadCategoryStrings(categoryName)
}

// loadCategoryStrings loads strings for the specified category
func (slp *StringListPane) loadCategoryStrings(categoryName string) {
	category := slp.manager.GetCategoryByName(categoryName)
	if category == nil {
		return
	}
	
	items := make([]list.Item, len(category.Fields))
	for i, field := range category.Fields {
		items[i] = StringItem{Field: field}
	}
	
	slp.list.SetItems(items)
	slp.list.Title = fmt.Sprintf("Strings - %s", categoryName)
}

// performSearch performs a search and updates the list
func (slp *StringListPane) performSearch() {
	query := slp.searchInput.Value()
	if query == "" {
		slp.loadCategoryStrings(slp.currentCategory)
		return
	}
	
	results := slp.manager.SearchFields(query)
	slp.searchResults = results
	
	items := make([]list.Item, len(results))
	for i, field := range results {
		items[i] = StringItem{Field: field}
	}
	
	slp.list.SetItems(items)
	slp.list.Title = fmt.Sprintf("Search: %s (%d results)", query, len(results))
}

// StartSearch starts search mode
func (slp *StringListPane) StartSearch() {
	slp.searchMode = true
	slp.searchInput.Focus()
}

// GetSelectedField returns the currently selected field
func (slp *StringListPane) GetSelectedField() *StringField {
	if item, ok := slp.list.SelectedItem().(StringItem); ok {
		return &item.Field
	}
	return nil
}

// EditorPane manages the string editor pane
type EditorPane struct {
	textInput    textinput.Model
	field        *StringField
	manager      *StringManager
	ansiHelper   *ANSIHelper
	colorEditor  *ColorCodeEditor
	width        int
	height       int
	focused      bool
	showingHelp  bool
	colorMode    bool
	dirty        bool
}

// NewEditorPane creates a new editor pane
func NewEditorPane(manager *StringManager) *EditorPane {
	ti := textinput.New()
	ti.Placeholder = "Select a string to edit..."
	ti.CharLimit = 1000
	ti.Width = 50
	
	return &EditorPane{
		textInput:   ti,
		manager:     manager,
		ansiHelper:  NewANSIHelper(),
		colorEditor: NewColorCodeEditor(),
		width:       50,
		height:      15,
		focused:     false,
		showingHelp: false,
		colorMode:   false,
		dirty:       false,
	}
}

// Init initializes the editor pane
func (ep *EditorPane) Init() tea.Cmd {
	return nil
}

// Update handles editor pane updates
func (ep *EditorPane) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	
	if ep.colorMode {
		// Handle color picker mode
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEsc:
				ep.colorMode = false
				return nil
			case tea.KeyEnter:
				// Insert color code at cursor position
				value := ep.textInput.Value()
				cursorPos := ep.textInput.Position()
				newValue := value[:cursorPos] + ep.colorEditor.CurrentCode + value[cursorPos:]
				ep.textInput.SetValue(newValue)
				ep.textInput.SetCursor(cursorPos + len(ep.colorEditor.CurrentCode))
				ep.colorMode = false
				ep.dirty = true
				return nil
			case tea.KeyUp:
				ep.colorEditor.PrevColor()
				return nil
			case tea.KeyDown:
				ep.colorEditor.NextColor()
				return nil
			}
		}
		return nil
	}
	
	if ep.focused {
		oldValue := ep.textInput.Value()
		ep.textInput, cmd = ep.textInput.Update(msg)
		if ep.textInput.Value() != oldValue {
			ep.dirty = true
		}
	}
	
	return cmd
}

// View renders the editor pane
func (ep *EditorPane) View(styles *StyleSet) string {
	var style lipgloss.Style
	if ep.focused {
		style = styles.ActivePane
	} else {
		style = styles.InactivePane
	}
	
	var content string
	
	if ep.colorMode {
		content = ep.renderColorPicker(styles)
	} else {
		content = ep.renderEditor(styles)
	}
	
	return style.
		Width(ep.width).
		Height(ep.height).
		Render(content)
}

// renderEditor renders the main editor view
func (ep *EditorPane) renderEditor(styles *StyleSet) string {
	title := "Editor"
	if ep.field != nil {
		title = fmt.Sprintf("Editor - %s", ep.field.DisplayName)
		if ep.dirty {
			title += " *"
		}
	}
	
	// Setup text input size
	ep.textInput.Width = ep.width - 4
	
	content := ep.textInput.View()
	
	if ep.showingHelp {
		help := "\nF2: Color Picker | F3: Preview | Ctrl+S: Save | Esc: Cancel"
		content = lipgloss.JoinVertical(lipgloss.Left, content, styles.TextMuted.Render(help))
	}
	
	if ep.field != nil {
		// Show field description
		desc := styles.TextMuted.Render(ep.field.Description)
		content = lipgloss.JoinVertical(lipgloss.Left, desc, content)
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, styles.ListHeader.Render(title), content)
}

// renderColorPicker renders the color picker interface
func (ep *EditorPane) renderColorPicker(styles *StyleSet) string {
	title := styles.ListHeader.Render("Color Picker")
	
	// Current color info
	colorInfo := fmt.Sprintf("Current: %s - %s", 
		ep.colorEditor.CurrentCode, 
		ep.colorEditor.GetColorDescription())
	
	// Preview
	preview := ep.ansiHelper.BuildColoredPreview(
		ep.colorEditor.CurrentCode + "Sample Text")
	
	// Instructions
	instructions := styles.TextMuted.Render("↑/↓: Change Color | Enter: Insert | Esc: Cancel")
	
	content := lipgloss.JoinVertical(lipgloss.Left,
		colorInfo,
		preview,
		"",
		instructions)
	
	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}

// SetSize sets the pane dimensions
func (ep *EditorPane) SetSize(width, height int) {
	ep.width = width
	ep.height = height
}

// SetFocused sets the focus state
func (ep *EditorPane) SetFocused(focused bool) {
	ep.focused = focused
	if focused {
		ep.textInput.Focus()
	} else {
		ep.textInput.Blur()
	}
}

// LoadField loads a field for editing
func (ep *EditorPane) LoadField(field *StringField) {
	ep.field = field
	if field != nil {
		ep.textInput.SetValue(field.Value)
		ep.textInput.Placeholder = fmt.Sprintf("Edit %s...", field.DisplayName)
		ep.colorEditor.SetPreviewText(field.Value)
	} else {
		ep.textInput.SetValue("")
		ep.textInput.Placeholder = "Select a string to edit..."
	}
	ep.dirty = false
}

// SaveField saves the current field
func (ep *EditorPane) SaveField() error {
	if ep.field == nil || !ep.dirty {
		return nil
	}
	
	newValue := ep.textInput.Value()
	if err := ep.manager.UpdateField(ep.field.Key, newValue); err != nil {
		return err
	}
	
	ep.field.Value = newValue
	ep.dirty = false
	return nil
}

// ToggleHelp toggles the help display
func (ep *EditorPane) ToggleHelp() {
	ep.showingHelp = !ep.showingHelp
}

// StartColorPicker starts the color picker mode
func (ep *EditorPane) StartColorPicker() {
	ep.colorMode = true
	if ep.field != nil {
		ep.colorEditor.SetPreviewText(ep.field.Value)
	}
}

// IsDirty returns whether the editor has unsaved changes
func (ep *EditorPane) IsDirty() bool {
	return ep.dirty
}

// PreviewPane manages the preview pane
type PreviewPane struct {
	viewport   viewport.Model
	field      *StringField
	ansiHelper *ANSIHelper
	width      int
	height     int
	focused    bool
	showRaw    bool
}

// NewPreviewPane creates a new preview pane
func NewPreviewPane() *PreviewPane {
	vp := viewport.New(40, 10)
	vp.SetContent("Select a string to preview...")
	
	return &PreviewPane{
		viewport:   vp,
		ansiHelper: NewANSIHelper(),
		width:      40,
		height:     10,
		focused:    false,
		showRaw:    false,
	}
}

// Init initializes the preview pane
func (pp *PreviewPane) Init() tea.Cmd {
	return nil
}

// Update handles preview pane updates
func (pp *PreviewPane) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	pp.viewport, cmd = pp.viewport.Update(msg)
	return cmd
}

// View renders the preview pane
func (pp *PreviewPane) View(styles *StyleSet) string {
	var style lipgloss.Style
	if pp.focused {
		style = styles.ActivePane
	} else {
		style = styles.InactivePane
	}
	
	pp.viewport.Width = pp.width - 2
	pp.viewport.Height = pp.height - 3
	
	title := "Preview"
	if pp.showRaw {
		title += " (Raw)"
	}
	
	content := lipgloss.JoinVertical(lipgloss.Left,
		styles.ListHeader.Render(title),
		pp.viewport.View())
	
	return style.
		Width(pp.width).
		Height(pp.height).
		Render(content)
}

// SetSize sets the pane dimensions
func (pp *PreviewPane) SetSize(width, height int) {
	pp.width = width
	pp.height = height
}

// SetFocused sets the focus state
func (pp *PreviewPane) SetFocused(focused bool) {
	pp.focused = focused
}

// LoadField loads a field for preview
func (pp *PreviewPane) LoadField(field *StringField) {
	pp.field = field
	pp.updatePreview()
}

// UpdatePreview updates the preview with current field value
func (pp *PreviewPane) updatePreview() {
	if pp.field == nil {
		pp.viewport.SetContent("Select a string to preview...")
		return
	}
	
	var content string
	if pp.showRaw {
		content = pp.field.Value
	} else {
		content = pp.ansiHelper.BuildColoredPreview(pp.field.Value)
	}
	
	// Add some context information
	info := fmt.Sprintf("Field: %s\nCategory: %s\nDescription: %s\n\n",
		pp.field.DisplayName,
		pp.field.Category,
		pp.field.Description)
	
	pp.viewport.SetContent(info + content)
}

// ToggleRawView toggles between rendered and raw view
func (pp *PreviewPane) ToggleRawView() {
	pp.showRaw = !pp.showRaw
	pp.updatePreview()
}