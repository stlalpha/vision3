package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfigScreen represents a configuration screen with forms and controls
type ConfigScreen struct {
	title       string
	fields      []ConfigField
	buttons     []*Button
	selected    int
	width       int
	height      int
	focused     bool
	fieldType   string // "system", "user", "network", etc.
}

// ConfigField represents a configuration field
type ConfigField struct {
	Label       string
	InputField  *InputField
	ListBox     *ListBox
	Type        string // "input", "list", "checkbox", "radio"
	Value       interface{}
	Options     []string
	Description string
	Required    bool
	ReadOnly    bool
}

// NewConfigScreen creates a new configuration screen
func NewConfigScreen(title, fieldType string, width, height int) *ConfigScreen {
	cs := &ConfigScreen{
		title:     title,
		fieldType: fieldType,
		width:     width,
		height:    height,
		focused:   false,
		selected:  0,
	}
	
	// Initialize based on field type
	switch fieldType {
	case "system":
		cs.initSystemFields()
	case "user":
		cs.initUserFields()
	case "network":
		cs.initNetworkFields()
	default:
		cs.initDefaultFields()
	}
	
	// Add standard buttons
	cs.buttons = []*Button{
		NewButton("Save", "save"),
		NewButton("Cancel", "cancel"),
		NewButton("Help", "help"),
	}
	
	return cs
}

// initSystemFields initializes system configuration fields
func (cs *ConfigScreen) initSystemFields() {
	cs.fields = []ConfigField{
		{
			Label:       "System Name:",
			InputField:  NewInputField("", "Enter BBS name", 40),
			Type:        "input",
			Description: "The name of your BBS system",
			Required:    true,
		},
		{
			Label:       "System Description:",
			InputField:  NewInputField("", "Brief description", 60),
			Type:        "input",
			Description: "A brief description of your BBS",
			Required:    false,
		},
		{
			Label:       "Telnet Port:",
			InputField:  NewInputField("", "23", 10),
			Type:        "input",
			Description: "Port for Telnet connections",
			Required:    true,
		},
		{
			Label:       "SSH Port:",
			InputField:  NewInputField("", "22", 10),
			Type:        "input",
			Description: "Port for SSH connections",
			Required:    true,
		},
		{
			Label:       "Time Zone:",
			ListBox:     NewListBox("Select Time Zone", 30, 8),
			Type:        "list",
			Options:     []string{"UTC", "EST", "CST", "MST", "PST", "GMT+1", "GMT+2"},
			Description: "System time zone",
			Required:    true,
		},
	}
	
	// Initialize time zone list
	if cs.fields[4].ListBox != nil {
		for _, tz := range cs.fields[4].Options {
			cs.fields[4].ListBox.AddItem(NewListItem(tz, tz))
		}
	}
}

// initUserFields initializes user management fields
func (cs *ConfigScreen) initUserFields() {
	cs.fields = []ConfigField{
		{
			Label:       "New User Security Level:",
			InputField:  NewInputField("", "10", 5),
			Type:        "input",
			Description: "Default security level for new users",
			Required:    true,
		},
		{
			Label:       "Maximum Users:",
			InputField:  NewInputField("", "1000", 10),
			Type:        "input",
			Description: "Maximum number of user accounts",
			Required:    true,
		},
		{
			Label:       "Password Policy:",
			ListBox:     NewListBox("Select Policy", 30, 6),
			Type:        "list",
			Options:     []string{"None", "Simple", "Complex", "Very Complex"},
			Description: "Password complexity requirements",
			Required:    true,
		},
		{
			Label:       "Auto-Delete Inactive Users:",
			InputField:  NewInputField("", "365", 5),
			Type:        "input",
			Description: "Days before inactive users are deleted (0=never)",
			Required:    false,
		},
	}
	
	// Initialize password policy list
	if cs.fields[2].ListBox != nil {
		for _, policy := range cs.fields[2].Options {
			cs.fields[2].ListBox.AddItem(NewListItem(policy, policy))
		}
	}
}

// initNetworkFields initializes network configuration fields
func (cs *ConfigScreen) initNetworkFields() {
	cs.fields = []ConfigField{
		{
			Label:       "Internet Email Gateway:",
			InputField:  NewInputField("", "mail.example.com", 40),
			Type:        "input",
			Description: "SMTP server for internet email",
			Required:    false,
		},
		{
			Label:       "Network Node Number:",
			InputField:  NewInputField("", "1:123/456", 15),
			Type:        "input",
			Description: "FidoNet-style node number",
			Required:    false,
		},
		{
			Label:       "Mailer Type:",
			ListBox:     NewListBox("Select Mailer", 25, 5),
			Type:        "list",
			Options:     []string{"None", "FrontDoor", "InterMail", "D'Bridge"},
			Description: "External mailer program",
			Required:    false,
		},
	}
	
	// Initialize mailer type list
	if cs.fields[2].ListBox != nil {
		for _, mailer := range cs.fields[2].Options {
			cs.fields[2].ListBox.AddItem(NewListItem(mailer, mailer))
		}
	}
}

// initDefaultFields initializes default placeholder fields
func (cs *ConfigScreen) initDefaultFields() {
	cs.fields = []ConfigField{
		{
			Label:       "Configuration Item:",
			InputField:  NewInputField("", "Enter value", 30),
			Type:        "input",
			Description: "Generic configuration field",
			Required:    false,
		},
	}
}

// HandleKey processes keyboard input for the config screen
func (cs *ConfigScreen) HandleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyTab:
		cs.nextField()
	case tea.KeyShiftTab:
		cs.prevField()
	case tea.KeyEnter:
		return cs.handleEnter()
	case tea.KeyEsc:
		return func() tea.Msg {
			return ConfigScreenMsg{Action: "cancel", Screen: cs}
		}
	case tea.KeyF1:
		return func() tea.Msg {
			return ConfigScreenMsg{Action: "help", Screen: cs}
		}
	case tea.KeyF2:
		return func() tea.Msg {
			return ConfigScreenMsg{Action: "save", Screen: cs}
		}
	default:
		// Pass to active field
		return cs.handleFieldInput(msg)
	}
	return nil
}

// nextField moves to the next field
func (cs *ConfigScreen) nextField() {
	totalItems := len(cs.fields) + len(cs.buttons)
	cs.selected = (cs.selected + 1) % totalItems
	cs.updateFieldFocus()
}

// prevField moves to the previous field
func (cs *ConfigScreen) prevField() {
	totalItems := len(cs.fields) + len(cs.buttons)
	cs.selected = (cs.selected - 1 + totalItems) % totalItems
	cs.updateFieldFocus()
}

// updateFieldFocus updates focus for fields and buttons
func (cs *ConfigScreen) updateFieldFocus() {
	// Clear all focus
	for i := range cs.fields {
		if cs.fields[i].InputField != nil {
			cs.fields[i].InputField.SetFocus(false)
		}
		if cs.fields[i].ListBox != nil {
			cs.fields[i].ListBox.SetFocus(false)
		}
	}
	for _, btn := range cs.buttons {
		btn.SetFocus(false)
	}
	
	// Set focus on selected item
	if cs.selected < len(cs.fields) {
		field := &cs.fields[cs.selected]
		if field.InputField != nil {
			field.InputField.SetFocus(true)
		}
		if field.ListBox != nil {
			field.ListBox.SetFocus(true)
		}
	} else {
		// Focus on button
		buttonIndex := cs.selected - len(cs.fields)
		if buttonIndex < len(cs.buttons) {
			cs.buttons[buttonIndex].SetFocus(true)
		}
	}
}

// handleEnter handles Enter key press
func (cs *ConfigScreen) handleEnter() tea.Cmd {
	if cs.selected >= len(cs.fields) {
		// Button pressed
		buttonIndex := cs.selected - len(cs.fields)
		if buttonIndex < len(cs.buttons) {
			return cs.buttons[buttonIndex].HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
		}
	}
	return nil
}

// handleFieldInput passes input to the active field
func (cs *ConfigScreen) handleFieldInput(msg tea.KeyMsg) tea.Cmd {
	if cs.selected < len(cs.fields) {
		field := &cs.fields[cs.selected]
		if field.InputField != nil {
			return field.InputField.HandleKey(msg)
		}
		if field.ListBox != nil {
			return field.ListBox.HandleKey(msg)
		}
	}
	return nil
}

// Render renders the configuration screen
func (cs *ConfigScreen) Render() string {
	// Create main container
	var content []string
	
	// Title
	titleStyle := TitleStyle.Width(cs.width - 4)
	content = append(content, titleStyle.Render(cs.title))
	content = append(content, "")
	
	// Fields
	for _, field := range cs.fields {
		// Field label
		labelStyle := lipgloss.NewStyle().
			Foreground(ColorText).
			Width(20).
			Align(lipgloss.Right)
		
		// Field content
		var fieldContent string
		if field.InputField != nil {
			fieldContent = field.InputField.Render()
		} else if field.ListBox != nil {
			fieldContent = field.ListBox.Render()
		}
		
		// Required indicator
		label := field.Label
		if field.Required {
			label = label + " *"
		}
		
		// Combine label and field
		fieldLine := lipgloss.JoinHorizontal(
			lipgloss.Top,
			labelStyle.Render(label),
			" ",
			fieldContent,
		)
		
		content = append(content, fieldLine)
		
		// Description
		if field.Description != "" {
			descStyle := HelpStyle.
				Width(cs.width - 4).
				MarginLeft(22)
			content = append(content, descStyle.Render(field.Description))
		}
		
		content = append(content, "")
	}
	
	// Buttons
	var buttonRow []string
	for _, btn := range cs.buttons {
		buttonRow = append(buttonRow, btn.Render())
	}
	
	buttonLine := lipgloss.JoinHorizontal(lipgloss.Center, buttonRow...)
	buttonStyle := lipgloss.NewStyle().Width(cs.width - 4).Align(lipgloss.Center)
	content = append(content, "", buttonStyle.Render(buttonLine))
	
	// Join all content
	screenContent := lipgloss.JoinVertical(lipgloss.Left, content...)
	
	// Create border
	return CreateBox(cs.width, cs.height, "", screenContent, true)
}

// SetFocus sets the focus state for the screen
func (cs *ConfigScreen) SetFocus(focused bool) {
	cs.focused = focused
	if focused {
		cs.updateFieldFocus()
	}
}

// GetValues returns the current field values
func (cs *ConfigScreen) GetValues() map[string]interface{} {
	values := make(map[string]interface{})
	
	for _, field := range cs.fields {
		if field.InputField != nil {
			values[field.Label] = field.InputField.GetValue()
		} else if field.ListBox != nil {
			if item, ok := field.ListBox.GetSelected(); ok {
				values[field.Label] = item.Value
			}
		}
	}
	
	return values
}

// ConfigScreenMsg represents a configuration screen message
type ConfigScreenMsg struct {
	Action string
	Screen *ConfigScreen
	Data   interface{}
}