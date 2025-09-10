package strings

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// KeyMap defines key bindings for the application
type KeyMap struct {
	Quit        key.Binding
	Help        key.Binding
	Tab         key.Binding
	ShiftTab    key.Binding
	Enter       key.Binding
	Escape      key.Binding
	Save        key.Binding
	Search      key.Binding
	ColorPicker key.Binding
	Preview     key.Binding
	Export      key.Binding
	Import      key.Binding
	Undo        key.Binding
	Redo        key.Binding
	ToggleRaw   key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?", "f1"),
			key.WithHelp("?", "help"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next pane"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev pane"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select/edit"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Save: key.NewBinding(
			key.WithKeys("ctrl+s", "f10"),
			key.WithHelp("ctrl+s", "save"),
		),
		Search: key.NewBinding(
			key.WithKeys("/", "ctrl+f"),
			key.WithHelp("/", "search"),
		),
		ColorPicker: key.NewBinding(
			key.WithKeys("f2"),
			key.WithHelp("f2", "color picker"),
		),
		Preview: key.NewBinding(
			key.WithKeys("f3"),
			key.WithHelp("f3", "toggle preview"),
		),
		Export: key.NewBinding(
			key.WithKeys("f4"),
			key.WithHelp("f4", "export"),
		),
		Import: key.NewBinding(
			key.WithKeys("f5"),
			key.WithHelp("f5", "import"),
		),
		Undo: key.NewBinding(
			key.WithKeys("ctrl+z"),
			key.WithHelp("ctrl+z", "undo"),
		),
		Redo: key.NewBinding(
			key.WithKeys("ctrl+y"),
			key.WithHelp("ctrl+y", "redo"),
		),
		ToggleRaw: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "toggle raw view"),
		),
	}
}

// Model represents the main application state
type Model struct {
	// Core components
	manager      *StringManager
	styles       *StyleSet
	keys         KeyMap
	
	// Panes
	categoryPane   *CategoryPane
	stringListPane *StringListPane
	editorPane     *EditorPane
	previewPane    *PreviewPane
	
	// Layout state
	activePane    ActivePane
	width         int
	height        int
	showHelp      bool
	showPreview   bool
	
	// Application state
	quitting      bool
	statusMessage string
	errorMessage  string
	
	// Dialog management
	dialogManager *DialogManager
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}

// NewModel creates a new TUI model
func NewModel(configPath string) (*Model, error) {
	manager, err := NewStringManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create string manager: %w", err)
	}
	
	// Create panes
	categoryPane := NewCategoryPane(manager)
	stringListPane := NewStringListPane(manager)
	editorPane := NewEditorPane(manager)
	previewPane := NewPreviewPane()
	
	// Setup initial state
	activePane := ActivePane{
		Current: PaneCategories,
		Count:   4, // Categories, StringList, Editor, Preview
	}
	
	// Set initial focus
	categoryPane.SetFocused(true)
	
	model := &Model{
		manager:        manager,
		styles:         NewStyleSet(),
		keys:           DefaultKeyMap(),
		categoryPane:   categoryPane,
		stringListPane: stringListPane,
		editorPane:     editorPane,
		previewPane:    previewPane,
		activePane:     activePane,
		width:          80,
		height:         25,
		showHelp:       false,
		showPreview:    true,
		quitting:       false,
		statusMessage:  "String Configuration Manager - Press ? for help",
		dialogManager:  NewDialogManager(),
	}
	
	return model, nil
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle dialog messages first
	if m.dialogManager.HasActiveDialog() {
		return m.handleDialogUpdate(msg)
	}
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		return m, nil
		
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
		
	case ErrorMsg:
		m.errorMessage = msg.Err.Error()
		dialog := NewDialogState()
		dialog.ShowErrorDialog(msg.Err)
		m.dialogManager.ShowDialog("error", dialog)
		return m, nil
		
	case StatusMsg:
		m.statusMessage = msg.Message
		m.errorMessage = "" // Clear any previous error
		return m, nil
		
	case DialogResultMsg:
		return m.handleDialogResult(msg)
	}
	
	// Update active pane
	return m.updateActivePane(msg)
}

// handleDialogUpdate handles updates when a dialog is active
func (m Model) handleDialogUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Let the dialog handle the message first
	cmd := m.dialogManager.Update(msg)
	if cmd != nil {
		return m, cmd
	}
	
	// Handle escape key to close dialogs
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
		m.dialogManager.HideDialog()
	}
	
	return m, nil
}

// handleDialogResult handles the result of dialog actions
func (m Model) handleDialogResult(msg DialogResultMsg) (tea.Model, tea.Cmd) {
	m.dialogManager.HideDialog()
	
	switch msg.Type {
	case DialogSaveConfirm:
		switch msg.Action {
		case "save":
			if err := m.editorPane.SaveField(); err != nil {
				return m, tea.Cmd(func() tea.Msg { return ErrorMsg{err} })
			}
			if err := m.manager.SaveConfig(); err != nil {
				return m, tea.Cmd(func() tea.Msg { return ErrorMsg{err} })
			}
			m.quitting = true
			return m, tea.Quit
		case "dont_save":
			m.quitting = true
			return m, tea.Quit
		}
		
	case DialogExport:
		if msg.Action == "export" {
			filename := msg.Data.(string)
			return m, m.exportToFile(filename)
		}
		
	case DialogImport:
		if msg.Action == "import" {
			filename := msg.Data.(string)
			return m, m.importFromFile(filename)
		}
		
	case DialogSearch:
		switch msg.Action {
		case "search":
			query := msg.Data.(string)
			m.stringListPane.searchInput.SetValue(query)
			m.stringListPane.performSearch()
		case "clear":
			m.stringListPane.searchInput.SetValue("")
			m.stringListPane.loadCategoryStrings(m.stringListPane.currentCategory)
		}
		
	case DialogColorPicker:
		if msg.Action == "insert" && msg.Data != nil {
			colorCode := msg.Data.(string)
			// Insert the color code into the editor
			value := m.editorPane.textInput.Value()
			cursorPos := m.editorPane.textInput.Position()
			newValue := value[:cursorPos] + colorCode + value[cursorPos:]
			m.editorPane.textInput.SetValue(newValue)
			m.editorPane.textInput.SetCursor(cursorPos + len(colorCode))
			m.editorPane.dirty = true
		}
	}
	
	return m, nil
}

// handleKeyPress handles key press events
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global key bindings
	switch {
	case key.Matches(msg, m.keys.Quit):
		if m.editorPane.IsDirty() {
			// Show save confirmation dialog
			dialog := NewDialogState()
			dialog.ShowSaveConfirmDialog()
			m.dialogManager.ShowDialog("saveconfirm", dialog)
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit
		
	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		return m, nil
		
	case key.Matches(msg, m.keys.Tab):
		m.switchPane(1)
		return m, nil
		
	case key.Matches(msg, m.keys.ShiftTab):
		m.switchPane(-1)
		return m, nil
		
	case key.Matches(msg, m.keys.Save):
		return m, m.saveCurrentString()
		
	case key.Matches(msg, m.keys.Search):
		if m.activePane.Current == PaneStringList {
			dialog := NewDialogState()
			dialog.ShowSearchDialog(m.stringListPane.searchInput.Value())
			m.dialogManager.ShowDialog("search", dialog)
		}
		return m, nil
		
	case key.Matches(msg, m.keys.ColorPicker):
		if m.activePane.Current == PaneEditor {
			dialog := NewDialogState()
			currentColor := "|15" // Default color
			if m.editorPane.field != nil {
				// Try to extract current color from cursor position
				currentColor = "|15"
			}
			dialog.ShowColorPickerDialog(currentColor)
			m.dialogManager.ShowDialog("colorpicker", dialog)
		}
		return m, nil
		
	case key.Matches(msg, m.keys.Preview):
		m.showPreview = !m.showPreview
		m.updateLayout()
		return m, nil
		
	case key.Matches(msg, m.keys.Export):
		dialog := NewDialogState()
		dialog.ShowExportDialog()
		m.dialogManager.ShowDialog("export", dialog)
		return m, nil
		
	case key.Matches(msg, m.keys.Import):
		dialog := NewDialogState()
		dialog.ShowImportDialog()
		m.dialogManager.ShowDialog("import", dialog)
		return m, nil
		
	case key.Matches(msg, m.keys.Undo):
		return m, m.undoChange()
		
	case key.Matches(msg, m.keys.Redo):
		return m, m.redoChange()
		
	case key.Matches(msg, m.keys.ToggleRaw):
		if m.activePane.Current == PanePreview {
			m.previewPane.ToggleRawView()
		}
		return m, nil
		
	case key.Matches(msg, m.keys.Enter):
		return m.handleEnterKey()
	}
	
	return m, nil
}

// updateActivePane updates the currently active pane
func (m Model) updateActivePane(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch m.activePane.Current {
	case PaneCategories:
		cmd = m.categoryPane.Update(msg)
		// Check if category selection changed
		if selectedCategory := m.categoryPane.GetSelectedCategory(); selectedCategory != "" {
			m.stringListPane.LoadCategory(selectedCategory)
		}
		
	case PaneStringList:
		cmd = m.stringListPane.Update(msg)
		// Check if string selection changed
		if selectedField := m.stringListPane.GetSelectedField(); selectedField != nil {
			m.editorPane.LoadField(selectedField)
			m.previewPane.LoadField(selectedField)
		}
		
	case PaneEditor:
		cmd = m.editorPane.Update(msg)
		
	case PanePreview:
		cmd = m.previewPane.Update(msg)
	}
	
	return m, cmd
}

// switchPane switches to the next/previous pane
func (m *Model) switchPane(direction int) {
	// Clear focus from current pane
	m.setAllPanesFocused(false)
	
	// Switch pane
	if direction > 0 {
		m.activePane.Next()
	} else {
		m.activePane.Prev()
	}
	
	// If preview is hidden, skip it
	if m.activePane.Current == PanePreview && !m.showPreview {
		if direction > 0 {
			m.activePane.Next()
		} else {
			m.activePane.Prev()
		}
	}
	
	// Set focus on new pane
	m.setAllPanesFocused(false)
	switch m.activePane.Current {
	case PaneCategories:
		m.categoryPane.SetFocused(true)
	case PaneStringList:
		m.stringListPane.SetFocused(true)
	case PaneEditor:
		m.editorPane.SetFocused(true)
	case PanePreview:
		m.previewPane.SetFocused(true)
	}
}

// setAllPanesFocused sets focus state for all panes
func (m *Model) setAllPanesFocused(focused bool) {
	m.categoryPane.SetFocused(focused)
	m.stringListPane.SetFocused(focused)
	m.editorPane.SetFocused(focused)
	m.previewPane.SetFocused(focused)
}

// handleEnterKey handles the enter key based on current pane
func (m Model) handleEnterKey() (tea.Model, tea.Cmd) {
	switch m.activePane.Current {
	case PaneCategories:
		// Switch to string list pane
		m.switchPane(1)
		
	case PaneStringList:
		// Switch to editor pane
		m.switchPane(1)
		
	case PaneEditor:
		// Could implement inline editing or validation
		
	case PanePreview:
		// Toggle raw view
		m.previewPane.ToggleRawView()
	}
	
	return m, nil
}

// updateLayout updates the layout based on current window size
func (m *Model) updateLayout() {
	// Reserve space for menu bar and status bar
	contentHeight := m.height - 2
	contentWidth := m.width
	
	// Calculate pane sizes
	categoryWidth := 25
	previewWidth := 0
	if m.showPreview {
		previewWidth = 30
	}
	
	stringListWidth := 35
	editorWidth := contentWidth - categoryWidth - stringListWidth - previewWidth
	
	// Ensure minimum widths
	if editorWidth < 30 {
		editorWidth = 30
		stringListWidth = contentWidth - categoryWidth - editorWidth - previewWidth
	}
	
	// Set pane sizes
	m.categoryPane.SetSize(categoryWidth, contentHeight)
	m.stringListPane.SetSize(stringListWidth, contentHeight)
	m.editorPane.SetSize(editorWidth, contentHeight)
	if m.showPreview {
		m.previewPane.SetSize(previewWidth, contentHeight)
	}
}

// View renders the entire application
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	
	// Render menu bar
	menuItems := []string{"File", "Edit", "Search", "Tools", "Help"}
	menuBar := m.styles.FormatMenuBar(menuItems, -1)
	
	// Render main content
	content := m.renderMainContent()
	
	// Render status bar
	statusItems := map[string]string{
		"Strings": fmt.Sprintf("%d", m.manager.GetFieldCount()),
		"Categories": fmt.Sprintf("%d", m.manager.GetCategoryCount()),
	}
	
	if m.manager.CanUndo() {
		statusItems["Undo"] = "Available"
	}
	if m.manager.CanRedo() {
		statusItems["Redo"] = "Available"
	}
	
	if m.errorMessage != "" {
		statusItems["Error"] = m.errorMessage
	} else if m.statusMessage != "" {
		statusItems["Status"] = m.statusMessage
	}
	
	statusBar := m.styles.FormatStatusBar(statusItems)
	
	// Combine all parts
	screen := lipgloss.JoinVertical(lipgloss.Left,
		menuBar,
		content,
		statusBar,
	)
	
	// Show help overlay if requested
	if m.showHelp {
		helpContent := m.renderHelp()
		screen = lipgloss.Place(m.width, m.height, 
			lipgloss.Center, lipgloss.Center, helpContent)
	}
	
	// Show dialog overlay if requested
	if m.dialogManager.HasActiveDialog() {
		dialogContent := m.dialogManager.View(m.styles)
		if dialogContent != "" {
			screen = lipgloss.Place(m.width, m.height,
				lipgloss.Center, lipgloss.Center, dialogContent)
		}
	}
	
	return screen
}

// renderMainContent renders the main content area with panes
func (m Model) renderMainContent() string {
	var panes []string
	
	// Categories pane
	panes = append(panes, m.categoryPane.View(m.styles))
	
	// String list pane
	panes = append(panes, m.stringListPane.View(m.styles))
	
	// Editor pane
	panes = append(panes, m.editorPane.View(m.styles))
	
	// Preview pane (if shown)
	if m.showPreview {
		panes = append(panes, m.previewPane.View(m.styles))
	}
	
	return lipgloss.JoinHorizontal(lipgloss.Top, panes...)
}

// renderHelp renders the help dialog
func (m Model) renderHelp() string {
	helpText := `
String Configuration Manager Help

NAVIGATION:
  Tab / Shift+Tab    Switch between panes
  Enter              Select item / Edit string
  Esc                Cancel / Go back
  q / Ctrl+C         Quit application

EDITING:
  F2                 Open color picker
  F3                 Toggle preview pane
  Ctrl+S             Save current string
  Ctrl+Z             Undo last change
  Ctrl+Y             Redo last change

SEARCH & TOOLS:
  /                  Search strings
  F4                 Export configuration
  F5                 Import configuration
  r                  Toggle raw view (in preview)

PANES:
  Categories         Browse string categories
  String List        View strings in category
  Editor             Edit selected string
  Preview            Preview with ANSI colors

Press any key to close help...
`
	
	return m.styles.CreateDialog("Help", 60, 20).Render(
		strings.TrimSpace(helpText))
}

// Command functions

// saveCurrentString saves the currently edited string
func (m Model) saveCurrentString() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if err := m.editorPane.SaveField(); err != nil {
			return ErrorMsg{err}
		}
		return StatusMsg{"String saved successfully"}
	})
}

// exportToFile exports the configuration to a specific file
func (m Model) exportToFile(filename string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		data, err := m.manager.ExportToJSON()
		if err != nil {
			return ErrorMsg{err}
		}
		
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return ErrorMsg{err}
		}
		
		return StatusMsg{fmt.Sprintf("Configuration exported to %s", filename)}
	})
}

// importFromFile imports a configuration from a specific file
func (m Model) importFromFile(filename string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		data, err := os.ReadFile(filename)
		if err != nil {
			return ErrorMsg{fmt.Errorf("failed to read %s: %w", filename, err)}
		}
		
		if err := m.manager.ImportFromJSON(data); err != nil {
			return ErrorMsg{err}
		}
		
		// Reinitialize the panes with new data
		return StatusMsg{"Configuration imported successfully"}
	})
}

// undoChange undos the last change
func (m Model) undoChange() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if err := m.manager.Undo(); err != nil {
			return ErrorMsg{err}
		}
		return StatusMsg{"Change undone"}
	})
}

// redoChange redos the last undone change
func (m Model) redoChange() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if err := m.manager.Redo(); err != nil {
			return ErrorMsg{err}
		}
		return StatusMsg{"Change redone"}
	})
}


// Message types for commands

type ErrorMsg struct {
	Err error
}

type StatusMsg struct {
	Message string
}

// RunTUI runs the TUI application
func RunTUI(configPath string) error {
	model, err := NewModel(configPath)
	if err != nil {
		return err
	}
	
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}