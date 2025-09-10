package strings

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DialogType represents the type of dialog being shown
type DialogType int

const (
	DialogNone DialogType = iota
	DialogSaveConfirm
	DialogExport
	DialogImport
	DialogError
	DialogInfo
	DialogColorPicker
	DialogSearch
)

// DialogState represents the state of a dialog
type DialogState struct {
	Type        DialogType
	Title       string
	Message     string
	Buttons     []string
	Selected    int
	TextInput   textinput.Model
	FilePicker  filepicker.Model
	Data        interface{}
	Width       int
	Height      int
	Visible     bool
}

// NewDialogState creates a new dialog state
func NewDialogState() *DialogState {
	ti := textinput.New()
	ti.Focus()
	
	fp := filepicker.New()
	fp.AllowedTypes = []string{".json"}
	fp.CurrentDirectory, _ = os.Getwd()
	
	return &DialogState{
		Type:       DialogNone,
		Buttons:    []string{"OK", "Cancel"},
		Selected:   0,
		TextInput:  ti,
		FilePicker: fp,
		Width:      60,
		Height:     15,
		Visible:    false,
	}
}

// ShowSaveConfirmDialog shows a save confirmation dialog
func (ds *DialogState) ShowSaveConfirmDialog() {
	ds.Type = DialogSaveConfirm
	ds.Title = "Unsaved Changes"
	ds.Message = "You have unsaved changes. Save before closing?"
	ds.Buttons = []string{"Save", "Don't Save", "Cancel"}
	ds.Selected = 0
	ds.Width = 50
	ds.Height = 10
	ds.Visible = true
}

// ShowExportDialog shows an export dialog
func (ds *DialogState) ShowExportDialog() {
	ds.Type = DialogExport
	ds.Title = "Export Configuration"
	ds.Message = "Enter filename for export:"
	ds.TextInput.SetValue("strings_export.json")
	ds.TextInput.Focus()
	ds.Buttons = []string{"Export", "Cancel"}
	ds.Selected = 0
	ds.Width = 60
	ds.Height = 12
	ds.Visible = true
}

// ShowImportDialog shows an import dialog
func (ds *DialogState) ShowImportDialog() {
	ds.Type = DialogImport
	ds.Title = "Import Configuration"
	ds.Message = "Select JSON file to import:"
	ds.Buttons = []string{"Import", "Cancel"}
	ds.Selected = 0
	ds.Width = 70
	ds.Height = 20
	ds.Visible = true
}

// ShowErrorDialog shows an error dialog
func (ds *DialogState) ShowErrorDialog(err error) {
	ds.Type = DialogError
	ds.Title = "Error"
	ds.Message = err.Error()
	ds.Buttons = []string{"OK"}
	ds.Selected = 0
	ds.Width = 60
	ds.Height = 10
	ds.Visible = true
}

// ShowInfoDialog shows an info dialog
func (ds *DialogState) ShowInfoDialog(title, message string) {
	ds.Type = DialogInfo
	ds.Title = title
	ds.Message = message
	ds.Buttons = []string{"OK"}
	ds.Selected = 0
	ds.Width = 50
	ds.Height = 8
	ds.Visible = true
}

// ShowColorPickerDialog shows the color picker dialog
func (ds *DialogState) ShowColorPickerDialog(currentColor string) {
	ds.Type = DialogColorPicker
	ds.Title = "ANSI Color Picker"
	ds.Message = "Select a color code:"
	ds.Data = currentColor
	ds.Buttons = []string{"Insert", "Cancel"}
	ds.Selected = 0
	ds.Width = 70
	ds.Height = 25
	ds.Visible = true
}

// ShowSearchDialog shows the search dialog
func (ds *DialogState) ShowSearchDialog(currentQuery string) {
	ds.Type = DialogSearch
	ds.Title = "Search Strings"
	ds.Message = "Enter search query:"
	ds.TextInput.SetValue(currentQuery)
	ds.TextInput.Focus()
	ds.Buttons = []string{"Search", "Clear", "Cancel"}
	ds.Selected = 0
	ds.Width = 50
	ds.Height = 10
	ds.Visible = true
}

// Hide hides the dialog
func (ds *DialogState) Hide() {
	ds.Visible = false
	ds.Type = DialogNone
	ds.TextInput.Blur()
}

// Update updates the dialog state
func (ds *DialogState) Update(msg tea.Msg) tea.Cmd {
	if !ds.Visible {
		return nil
	}
	
	var cmd tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch ds.Type {
		case DialogExport, DialogSearch:
			// Handle text input dialogs
			switch msg.String() {
			case "enter":
				if ds.Selected == 0 { // OK/Export/Search button
					return ds.handleDialogAction()
				}
				ds.Hide()
				return nil
			case "esc":
				ds.Hide()
				return nil
			case "tab", "shift+tab":
				ds.Selected = (ds.Selected + 1) % len(ds.Buttons)
				return nil
			default:
				ds.TextInput, cmd = ds.TextInput.Update(msg)
				return cmd
			}
			
		case DialogImport:
			// Handle file picker dialogs
			switch msg.String() {
			case "enter":
				if ds.Selected == 0 { // Import button
					return ds.handleDialogAction()
				}
				ds.Hide()
				return nil
			case "esc":
				ds.Hide()
				return nil
			case "tab", "shift+tab":
				ds.Selected = (ds.Selected + 1) % len(ds.Buttons)
				return nil
			default:
				ds.FilePicker, cmd = ds.FilePicker.Update(msg)
				return cmd
			}
			
		case DialogColorPicker:
			// Handle color picker
			return ds.handleColorPickerInput(msg)
			
		default:
			// Handle simple dialogs (confirm, error, info)
			switch msg.String() {
			case "enter":
				return ds.handleDialogAction()
			case "esc":
				ds.Hide()
				return nil
			case "left", "h":
				if ds.Selected > 0 {
					ds.Selected--
				}
				return nil
			case "right", "l":
				if ds.Selected < len(ds.Buttons)-1 {
					ds.Selected++
				}
				return nil
			case "tab":
				ds.Selected = (ds.Selected + 1) % len(ds.Buttons)
				return nil
			case "shift+tab":
				ds.Selected--
				if ds.Selected < 0 {
					ds.Selected = len(ds.Buttons) - 1
				}
				return nil
			}
		}
	}
	
	return nil
}

// handleColorPickerInput handles input for the color picker
func (ds *DialogState) handleColorPickerInput(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		if ds.Selected == 0 { // Insert button
			return ds.handleDialogAction()
		}
		ds.Hide()
		return nil
	case "esc":
		ds.Hide()
		return nil
	case "tab", "shift+tab":
		ds.Selected = (ds.Selected + 1) % len(ds.Buttons)
		return nil
	case "up", "k":
		// Previous color in color picker
		return tea.Cmd(func() tea.Msg {
			return ColorPickerPrevMsg{}
		})
	case "down", "j":
		// Next color in color picker
		return tea.Cmd(func() tea.Msg {
			return ColorPickerNextMsg{}
		})
	case "left", "h":
		// Previous category in color picker
		return tea.Cmd(func() tea.Msg {
			return ColorPickerPrevCategoryMsg{}
		})
	case "right", "l":
		// Next category in color picker
		return tea.Cmd(func() tea.Msg {
			return ColorPickerNextCategoryMsg{}
		})
	}
	return nil
}

// handleDialogAction handles the action when a dialog button is pressed
func (ds *DialogState) handleDialogAction() tea.Cmd {
	switch ds.Type {
	case DialogSaveConfirm:
		switch ds.Selected {
		case 0: // Save
			return tea.Cmd(func() tea.Msg {
				return DialogResultMsg{Type: ds.Type, Action: "save", Data: nil}
			})
		case 1: // Don't Save
			return tea.Cmd(func() tea.Msg {
				return DialogResultMsg{Type: ds.Type, Action: "dont_save", Data: nil}
			})
		case 2: // Cancel
			ds.Hide()
			return nil
		}
		
	case DialogExport:
		if ds.Selected == 0 { // Export
			filename := ds.TextInput.Value()
			if filename == "" {
				filename = "strings_export.json"
			}
			return tea.Cmd(func() tea.Msg {
				return DialogResultMsg{Type: ds.Type, Action: "export", Data: filename}
			})
		}
		
	case DialogImport:
		if ds.Selected == 0 { // Import
			selectedFile := ds.FilePicker.Path
			return tea.Cmd(func() tea.Msg {
				return DialogResultMsg{Type: ds.Type, Action: "import", Data: selectedFile}
			})
		}
		
	case DialogSearch:
		switch ds.Selected {
		case 0: // Search
			query := ds.TextInput.Value()
			return tea.Cmd(func() tea.Msg {
				return DialogResultMsg{Type: ds.Type, Action: "search", Data: query}
			})
		case 1: // Clear
			return tea.Cmd(func() tea.Msg {
				return DialogResultMsg{Type: ds.Type, Action: "clear", Data: ""}
			})
		}
		
	case DialogColorPicker:
		if ds.Selected == 0 { // Insert
			return tea.Cmd(func() tea.Msg {
				return DialogResultMsg{Type: ds.Type, Action: "insert", Data: ds.Data}
			})
		}
		
	case DialogError, DialogInfo:
		ds.Hide()
		return nil
	}
	
	ds.Hide()
	return nil
}

// View renders the dialog
func (ds *DialogState) View(styles *StyleSet) string {
	if !ds.Visible {
		return ""
	}
	
	switch ds.Type {
	case DialogColorPicker:
		return ds.renderColorPickerDialog(styles)
	case DialogImport:
		return ds.renderFilePickerDialog(styles)
	default:
		return ds.renderStandardDialog(styles)
	}
}

// renderStandardDialog renders a standard dialog with message and buttons
func (ds *DialogState) renderStandardDialog(styles *StyleSet) string {
	// Title
	title := styles.DialogTitle.Width(ds.Width - 4).Render(ds.Title)
	
	// Message
	message := styles.DialogContent.Width(ds.Width - 4).Render(ds.Message)
	
	// Text input if applicable
	var textInputView string
	if ds.Type == DialogExport || ds.Type == DialogSearch {
		ds.TextInput.Width = ds.Width - 6
		textInputView = styles.SearchBox.Render(ds.TextInput.View())
	}
	
	// Buttons
	var buttonViews []string
	for i, buttonText := range ds.Buttons {
		var buttonStyle lipgloss.Style
		if i == ds.Selected {
			buttonStyle = styles.DialogButtonSelected
		} else {
			buttonStyle = styles.DialogButton
		}
		buttonViews = append(buttonViews, buttonStyle.Render(buttonText))
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Center, buttonViews...)
	
	// Combine content
	var content string
	if textInputView != "" {
		content = lipgloss.JoinVertical(lipgloss.Left,
			title,
			"",
			message,
			"",
			textInputView,
			"",
			buttons,
		)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left,
			title,
			"",
			message,
			"",
			buttons,
		)
	}
	
	return styles.DialogBox.Width(ds.Width).Height(ds.Height).Render(content)
}

// renderFilePickerDialog renders a file picker dialog
func (ds *DialogState) renderFilePickerDialog(styles *StyleSet) string {
	title := styles.DialogTitle.Width(ds.Width - 4).Render(ds.Title)
	
	// File picker
	ds.FilePicker.Height = ds.Height - 8
	filePicker := ds.FilePicker.View()
	
	// Buttons
	var buttonViews []string
	for i, buttonText := range ds.Buttons {
		var buttonStyle lipgloss.Style
		if i == ds.Selected {
			buttonStyle = styles.DialogButtonSelected
		} else {
			buttonStyle = styles.DialogButton
		}
		buttonViews = append(buttonViews, buttonStyle.Render(buttonText))
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Center, buttonViews...)
	
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		filePicker,
		"",
		buttons,
	)
	
	return styles.DialogBox.Width(ds.Width).Height(ds.Height).Render(content)
}

// renderColorPickerDialog renders the color picker dialog
func (ds *DialogState) renderColorPickerDialog(styles *StyleSet) string {
	title := styles.DialogTitle.Width(ds.Width - 4).Render(ds.Title)
	
	// Create color palette display
	ansiHelper := NewANSIHelper()
	
	var paletteLines []string
	
	// Standard colors
	paletteLines = append(paletteLines, styles.ListHeader.Render("Standard Colors (|00-|07):"))
	var standardColors []string
	for i := 0; i < 8; i++ {
		code := fmt.Sprintf("|%02d", i)
		sample := ansiHelper.BuildColoredPreview(code + "██")
		standardColors = append(standardColors, fmt.Sprintf("%s %s", code, sample))
	}
	paletteLines = append(paletteLines, strings.Join(standardColors, " "))
	paletteLines = append(paletteLines, "")
	
	// Bright colors
	paletteLines = append(paletteLines, styles.ListHeader.Render("Bright Colors (|08-|15):"))
	var brightColors []string
	for i := 8; i < 16; i++ {
		code := fmt.Sprintf("|%02d", i)
		sample := ansiHelper.BuildColoredPreview(code + "██")
		brightColors = append(brightColors, fmt.Sprintf("%s %s", code, sample))
	}
	paletteLines = append(paletteLines, strings.Join(brightColors, " "))
	paletteLines = append(paletteLines, "")
	
	// Background colors
	paletteLines = append(paletteLines, styles.ListHeader.Render("Background Colors (|B0-|B7):"))
	var bgColors []string
	for i := 0; i < 8; i++ {
		code := fmt.Sprintf("|B%d", i)
		sample := ansiHelper.BuildColoredPreview("|15" + code + "BG|B0")
		bgColors = append(bgColors, fmt.Sprintf("%s %s", code, sample))
	}
	paletteLines = append(paletteLines, strings.Join(bgColors, " "))
	paletteLines = append(paletteLines, "")
	
	// Special codes
	paletteLines = append(paletteLines, styles.ListHeader.Render("Special Codes:"))
	specialCodes := []string{"|CL (Clear)", "|P (Save Pos)", "|PP (Restore)", "|23 (Reset)"}
	paletteLines = append(paletteLines, strings.Join(specialCodes, " "))
	
	palette := lipgloss.JoinVertical(lipgloss.Left, paletteLines...)
	
	// Current selection highlight
	currentCode := ""
	if ds.Data != nil {
		currentCode = ds.Data.(string)
	}
	selection := styles.TextHighlight.Render(fmt.Sprintf("Selected: %s", currentCode))
	
	// Instructions
	instructions := styles.TextMuted.Render("↑/↓: Navigate | ←/→: Categories | Enter: Insert | Esc: Cancel")
	
	// Buttons
	var buttonViews []string
	for i, buttonText := range ds.Buttons {
		var buttonStyle lipgloss.Style
		if i == ds.Selected {
			buttonStyle = styles.DialogButtonSelected
		} else {
			buttonStyle = styles.DialogButton
		}
		buttonViews = append(buttonViews, buttonStyle.Render(buttonText))
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Center, buttonViews...)
	
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		palette,
		"",
		selection,
		"",
		instructions,
		"",
		buttons,
	)
	
	return styles.DialogBox.Width(ds.Width).Height(ds.Height).Render(content)
}

// Message types for dialog results

type DialogResultMsg struct {
	Type   DialogType
	Action string
	Data   interface{}
}

type ColorPickerNextMsg struct{}
type ColorPickerPrevMsg struct{}
type ColorPickerNextCategoryMsg struct{}
type ColorPickerPrevCategoryMsg struct{}

// DialogManager manages multiple dialogs
type DialogManager struct {
	dialogs map[string]*DialogState
	current string
}

// NewDialogManager creates a new dialog manager
func NewDialogManager() *DialogManager {
	return &DialogManager{
		dialogs: make(map[string]*DialogState),
		current: "",
	}
}

// ShowDialog shows a dialog by name
func (dm *DialogManager) ShowDialog(name string, dialog *DialogState) {
	dm.dialogs[name] = dialog
	dm.current = name
	dialog.Visible = true
}

// HideDialog hides the current dialog
func (dm *DialogManager) HideDialog() {
	if dm.current != "" {
		if dialog, exists := dm.dialogs[dm.current]; exists {
			dialog.Hide()
		}
		dm.current = ""
	}
}

// Update updates the current dialog
func (dm *DialogManager) Update(msg tea.Msg) tea.Cmd {
	if dm.current == "" {
		return nil
	}
	
	if dialog, exists := dm.dialogs[dm.current]; exists {
		return dialog.Update(msg)
	}
	
	return nil
}

// View renders the current dialog
func (dm *DialogManager) View(styles *StyleSet) string {
	if dm.current == "" {
		return ""
	}
	
	if dialog, exists := dm.dialogs[dm.current]; exists {
		return dialog.View(styles)
	}
	
	return ""
}

// HasActiveDialog returns whether there's an active dialog
func (dm *DialogManager) HasActiveDialog() bool {
	return dm.current != ""
}