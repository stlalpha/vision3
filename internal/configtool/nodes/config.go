package nodes

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NodeConfigScreen represents the node configuration interface
type NodeConfigScreen struct {
	nodeManager     NodeManager
	width           int
	height          int
	focused         bool
	selectedNode    int
	currentConfig   *NodeConfiguration
	editMode        bool
	selectedField   int
	fieldValues     map[string]interface{}
	validationErrors map[string]string
	isDirty         bool
	showHelp        bool
}

// ConfigField represents a configurable field
type ConfigField struct {
	Name        string
	Label       string
	Type        string // "text", "number", "boolean", "select", "multiselect"
	Value       interface{}
	Options     []string
	Description string
	Required    bool
	Validation  func(interface{}) error
}

// NewNodeConfigScreen creates a new node configuration screen
func NewNodeConfigScreen(nodeManager NodeManager, width, height int) *NodeConfigScreen {
	return &NodeConfigScreen{
		nodeManager:      nodeManager,
		width:            width,
		height:           height,
		selectedNode:     1,
		fieldValues:      make(map[string]interface{}),
		validationErrors: make(map[string]string),
		showHelp:         false,
	}
}

// Update implements tea.Model
func (ncs *NodeConfigScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return ncs.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		ncs.width = msg.Width
		ncs.height = msg.Height
	}
	return ncs, nil
}

// handleKeyPress processes keyboard input
func (ncs *NodeConfigScreen) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if ncs.editMode {
		return ncs.handleEditModeKeys(msg)
	}

	switch msg.String() {
	case "q", "esc":
		if ncs.isDirty {
			// Show confirmation dialog
			return ncs, nil
		}
		return ncs, tea.Quit
	case "h", "f1":
		ncs.showHelp = !ncs.showHelp
	case "n":
		return ncs.handleNewNode()
	case "d":
		return ncs.handleDeleteNode()
	case "c":
		return ncs.handleCopyNode()
	case "e", "enter":
		return ncs.handleEditNode()
	case "s":
		return ncs.handleSaveConfig()
	case "r":
		return ncs.handleResetConfig()
	case "up", "k":
		if ncs.selectedNode > 1 {
			ncs.selectedNode--
			ncs.loadNodeConfig()
		}
	case "down", "j":
		nodes := ncs.nodeManager.GetAllNodes()
		if ncs.selectedNode < len(nodes) {
			ncs.selectedNode++
			ncs.loadNodeConfig()
		}
	case "left", "right":
		// Navigate between fields when editing
	case "tab":
		ncs.selectedField++
		if ncs.selectedField >= len(ncs.getConfigFields()) {
			ncs.selectedField = 0
		}
	case "shift+tab":
		ncs.selectedField--
		if ncs.selectedField < 0 {
			ncs.selectedField = len(ncs.getConfigFields()) - 1
		}
	}
	return ncs, nil
}

// handleEditModeKeys processes keys during edit mode
func (ncs *NodeConfigScreen) handleEditModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		ncs.editMode = false
		ncs.loadNodeConfig() // Reload to discard changes
	case "enter":
		if ncs.validateCurrentField() {
			ncs.editMode = false
			ncs.isDirty = true
		}
	case "tab":
		if ncs.validateCurrentField() {
			ncs.selectedField++
			if ncs.selectedField >= len(ncs.getConfigFields()) {
				ncs.selectedField = 0
			}
		}
	case "shift+tab":
		if ncs.validateCurrentField() {
			ncs.selectedField--
			if ncs.selectedField < 0 {
				ncs.selectedField = len(ncs.getConfigFields()) - 1
			}
		}
	default:
		// Handle text input for the current field
		ncs.handleTextInput(msg.String())
	}
	return ncs, nil
}

// handleNewNode creates a new node configuration
func (ncs *NodeConfigScreen) handleNewNode() (tea.Model, tea.Cmd) {
	// Find next available node ID
	nodes := ncs.nodeManager.GetAllNodes()
	maxID := 0
	for _, node := range nodes {
		if node.NodeID > maxID {
			maxID = node.NodeID
		}
	}
	
	newNodeID := maxID + 1
	defaultConfig := NodeConfiguration{
		NodeID:      newNodeID,
		Name:        fmt.Sprintf("Node %d", newNodeID),
		Enabled:     true,
		MaxUsers:    1,
		TimeLimit:   60 * time.Minute,
		AccessLevel: 1,
		LocalNode:   true,
		ChatEnabled: true,
		NetworkSettings: NetworkConfig{
			Protocol: "telnet",
			Port:     2300 + newNodeID,
			Address:  "0.0.0.0",
		},
		DoorSettings: DoorConfig{
			AllowDoors:     true,
			MaxDoorTime:    30,
			ShareResources: true,
		},
	}
	
	ncs.selectedNode = newNodeID
	ncs.currentConfig = &defaultConfig
	ncs.loadConfigIntoFields()
	ncs.isDirty = true
	ncs.editMode = true
	ncs.selectedField = 1 // Start with name field
	
	return ncs, nil
}

// handleDeleteNode deletes the current node configuration
func (ncs *NodeConfigScreen) handleDeleteNode() (tea.Model, tea.Cmd) {
	if ncs.selectedNode <= 0 {
		return ncs, nil
	}
	
	// Check if node is currently active
	node, err := ncs.nodeManager.GetNode(ncs.selectedNode)
	if err == nil && node.User != nil {
		// Node is active, cannot delete
		ncs.validationErrors["general"] = "Cannot delete active node with connected user"
		return ncs, nil
	}
	
	// Disable the node instead of actually deleting
	if err := ncs.nodeManager.DisableNode(ncs.selectedNode); err != nil {
		ncs.validationErrors["general"] = fmt.Sprintf("Failed to disable node: %v", err)
		return ncs, nil
	}
	
	// Move to previous node
	if ncs.selectedNode > 1 {
		ncs.selectedNode--
	}
	ncs.loadNodeConfig()
	
	return ncs, nil
}

// handleCopyNode copies the current node configuration
func (ncs *NodeConfigScreen) handleCopyNode() (tea.Model, tea.Cmd) {
	if ncs.currentConfig == nil {
		return ncs, nil
	}
	
	// Find next available node ID
	nodes := ncs.nodeManager.GetAllNodes()
	maxID := 0
	for _, node := range nodes {
		if node.NodeID > maxID {
			maxID = node.NodeID
		}
	}
	
	newNodeID := maxID + 1
	copiedConfig := *ncs.currentConfig
	copiedConfig.NodeID = newNodeID
	copiedConfig.Name = fmt.Sprintf("%s (Copy)", ncs.currentConfig.Name)
	
	// Update network port to avoid conflicts
	copiedConfig.NetworkSettings.Port = 2300 + newNodeID
	
	ncs.selectedNode = newNodeID
	ncs.currentConfig = &copiedConfig
	ncs.loadConfigIntoFields()
	ncs.isDirty = true
	
	return ncs, nil
}

// handleEditNode enters edit mode for the current node
func (ncs *NodeConfigScreen) handleEditNode() (tea.Model, tea.Cmd) {
	ncs.editMode = true
	ncs.selectedField = 0
	return ncs, nil
}

// handleSaveConfig saves the current configuration
func (ncs *NodeConfigScreen) handleSaveConfig() (tea.Model, tea.Cmd) {
	if !ncs.isDirty || ncs.currentConfig == nil {
		return ncs, nil
	}
	
	// Validate all fields
	if !ncs.validateAllFields() {
		return ncs, nil
	}
	
	// Apply field values to config
	ncs.applyFieldsToConfig()
	
	// Save to node manager
	if err := ncs.nodeManager.UpdateNodeConfig(ncs.selectedNode, *ncs.currentConfig); err != nil {
		ncs.validationErrors["general"] = fmt.Sprintf("Failed to save configuration: %v", err)
		return ncs, nil
	}
	
	ncs.isDirty = false
	delete(ncs.validationErrors, "general")
	
	return ncs, nil
}

// handleResetConfig resets the configuration to saved values
func (ncs *NodeConfigScreen) handleResetConfig() (tea.Model, tea.Cmd) {
	ncs.loadNodeConfig()
	ncs.isDirty = false
	ncs.editMode = false
	ncs.validationErrors = make(map[string]string)
	return ncs, nil
}

// View renders the configuration screen
func (ncs *NodeConfigScreen) View() string {
	if ncs.showHelp {
		return ncs.renderHelpScreen()
	}
	
	var sections []string
	
	// Title
	sections = append(sections, ncs.renderTitle())
	
	// Node selector
	sections = append(sections, ncs.renderNodeSelector())
	
	// Configuration form
	sections = append(sections, ncs.renderConfigForm())
	
	// Status/error messages
	sections = append(sections, ncs.renderStatusMessages())
	
	// Help line
	sections = append(sections, ncs.renderHelpLine())
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderTitle renders the screen title
func (ncs *NodeConfigScreen) renderTitle() string {
	title := "Node Configuration Manager"
	if ncs.isDirty {
		title += " (Modified)"
	}
	
	titleStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("4")).
		Foreground(lipgloss.Color("15")).
		Bold(true).
		Padding(0, 1).
		Width(ncs.width)
	
	return titleStyle.Render(title)
}

// renderNodeSelector renders the node selection area
func (ncs *NodeConfigScreen) renderNodeSelector() string {
	nodes := ncs.nodeManager.GetAllNodes()
	
	var nodeList []string
	for _, node := range nodes {
		status := "○"
		if node.NodeID == ncs.selectedNode {
			status = "●"
		}
		
		var statusColor string
		switch node.Status {
		case NodeStatusAvailable:
			statusColor = "2" // Green
		case NodeStatusConnected, NodeStatusLoggedIn:
			statusColor = "3" // Yellow
		case NodeStatusDisabled:
			statusColor = "8" // Gray
		default:
			statusColor = "1" // Red
		}
		
		nodeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor))
		nodeStr := fmt.Sprintf("%s Node %d (%s)", status, node.NodeID, node.Config.Name)
		
		if node.NodeID == ncs.selectedNode {
			nodeStyle = nodeStyle.Background(lipgloss.Color("4"))
		}
		
		nodeList = append(nodeList, nodeStyle.Render(nodeStr))
	}
	
	selectorStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(ncs.width - 4)
	
	content := "Nodes: " + strings.Join(nodeList, " ")
	return selectorStyle.Render(content)
}

// renderConfigForm renders the configuration form
func (ncs *NodeConfigScreen) renderConfigForm() string {
	if ncs.currentConfig == nil {
		return "No node selected"
	}
	
	fields := ncs.getConfigFields()
	var formRows []string
	
	for i, field := range fields {
		row := ncs.renderConfigField(field, i == ncs.selectedField)
		formRows = append(formRows, row)
	}
	
	formStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(ncs.width - 4)
	
	return formStyle.Render(lipgloss.JoinVertical(lipgloss.Left, formRows...))
}

// renderConfigField renders a single configuration field
func (ncs *NodeConfigScreen) renderConfigField(field ConfigField, selected bool) string {
	var fieldStyle lipgloss.Style
	if selected {
		fieldStyle = lipgloss.NewStyle().Background(lipgloss.Color("4"))
	}
	
	label := fmt.Sprintf("%-20s:", field.Label)
	if field.Required {
		label += "*"
	}
	
	value := ncs.formatFieldValue(field)
	
	// Show validation error if present
	if errMsg, hasError := ncs.validationErrors[field.Name]; hasError {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		value += " " + errorStyle.Render(fmt.Sprintf("(%s)", errMsg))
	}
	
	// Show edit cursor if this field is being edited
	if selected && ncs.editMode {
		value += " ◄"
	}
	
	fieldLine := fmt.Sprintf("%s %s", label, value)
	
	if field.Description != "" && selected {
		fieldLine += "\n" + strings.Repeat(" ", 22) + 
			lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(field.Description)
	}
	
	return fieldStyle.Render(fieldLine)
}

// getConfigFields returns the configuration fields for the current node
func (ncs *NodeConfigScreen) getConfigFields() []ConfigField {
	if ncs.currentConfig == nil {
		return nil
	}
	
	return []ConfigField{
		{
			Name:        "node_id",
			Label:       "Node ID",
			Type:        "number",
			Value:       ncs.currentConfig.NodeID,
			Description: "Unique identifier for this node",
			Required:    true,
		},
		{
			Name:        "name",
			Label:       "Node Name",
			Type:        "text",
			Value:       ncs.currentConfig.Name,
			Description: "Friendly name for this node",
			Required:    true,
		},
		{
			Name:        "enabled",
			Label:       "Enabled",
			Type:        "boolean",
			Value:       ncs.currentConfig.Enabled,
			Description: "Whether this node accepts connections",
		},
		{
			Name:        "max_users",
			Label:       "Max Users",
			Type:        "number",
			Value:       ncs.currentConfig.MaxUsers,
			Description: "Maximum concurrent users (usually 1 for BBS)",
			Required:    true,
		},
		{
			Name:        "time_limit",
			Label:       "Time Limit (min)",
			Type:        "number",
			Value:       int(ncs.currentConfig.TimeLimit.Minutes()),
			Description: "Maximum session time in minutes",
			Required:    true,
		},
		{
			Name:        "access_level",
			Label:       "Access Level",
			Type:        "number",
			Value:       ncs.currentConfig.AccessLevel,
			Description: "Minimum access level required",
		},
		{
			Name:        "local_node",
			Label:       "Local Node",
			Type:        "boolean",
			Value:       ncs.currentConfig.LocalNode,
			Description: "True for local, false for remote nodes",
		},
		{
			Name:        "chat_enabled",
			Label:       "Chat Enabled",
			Type:        "boolean",
			Value:       ncs.currentConfig.ChatEnabled,
			Description: "Allow inter-node chat",
		},
		{
			Name:        "network_protocol",
			Label:       "Protocol",
			Type:        "select",
			Value:       ncs.currentConfig.NetworkSettings.Protocol,
			Options:     []string{"telnet", "ssh", "rlogin"},
			Description: "Network protocol for connections",
		},
		{
			Name:        "network_port",
			Label:       "Network Port",
			Type:        "number",
			Value:       ncs.currentConfig.NetworkSettings.Port,
			Description: "TCP port for network connections",
		},
		{
			Name:        "network_address",
			Label:       "Bind Address",
			Type:        "text",
			Value:       ncs.currentConfig.NetworkSettings.Address,
			Description: "IP address to bind to (0.0.0.0 for all)",
		},
		{
			Name:        "allow_doors",
			Label:       "Allow Doors",
			Type:        "boolean",
			Value:       ncs.currentConfig.DoorSettings.AllowDoors,
			Description: "Allow door game execution",
		},
		{
			Name:        "max_door_time",
			Label:       "Max Door Time",
			Type:        "number",
			Value:       ncs.currentConfig.DoorSettings.MaxDoorTime,
			Description: "Maximum time in doors (minutes)",
		},
		{
			Name:        "share_resources",
			Label:       "Share Resources",
			Type:        "boolean",
			Value:       ncs.currentConfig.DoorSettings.ShareResources,
			Description: "Share files between nodes",
		},
	}
}

// formatFieldValue formats a field value for display
func (ncs *NodeConfigScreen) formatFieldValue(field ConfigField) string {
	// Check if we have a modified value in fieldValues
	if val, exists := ncs.fieldValues[field.Name]; exists {
		field.Value = val
	}
	
	switch field.Type {
	case "boolean":
		if field.Value.(bool) {
			return "Yes"
		}
		return "No"
	case "select":
		return field.Value.(string)
	case "number":
		return fmt.Sprintf("%v", field.Value)
	default:
		return fmt.Sprintf("%v", field.Value)
	}
}

// handleTextInput handles text input for the current field
func (ncs *NodeConfigScreen) handleTextInput(input string) {
	fields := ncs.getConfigFields()
	if ncs.selectedField >= len(fields) {
		return
	}
	
	field := fields[ncs.selectedField]
	currentValue := ""
	
	if val, exists := ncs.fieldValues[field.Name]; exists {
		currentValue = fmt.Sprintf("%v", val)
	} else {
		currentValue = fmt.Sprintf("%v", field.Value)
	}
	
	// Handle different input types
	switch input {
	case "backspace":
		if len(currentValue) > 0 {
			currentValue = currentValue[:len(currentValue)-1]
		}
	case "space":
		if field.Type == "text" {
			currentValue += " "
		} else if field.Type == "boolean" {
			// Toggle boolean
			if currentValue == "true" || currentValue == "Yes" {
				currentValue = "false"
			} else {
				currentValue = "true"
			}
		}
	default:
		if len(input) == 1 {
			currentValue += input
		}
	}
	
	ncs.fieldValues[field.Name] = currentValue
}

// validateCurrentField validates the currently selected field
func (ncs *NodeConfigScreen) validateCurrentField() bool {
	fields := ncs.getConfigFields()
	if ncs.selectedField >= len(fields) {
		return true
	}
	
	field := fields[ncs.selectedField]
	value, exists := ncs.fieldValues[field.Name]
	if !exists {
		value = field.Value
	}
	
	// Perform validation based on field type
	if err := ncs.validateFieldValue(field, value); err != nil {
		ncs.validationErrors[field.Name] = err.Error()
		return false
	}
	
	delete(ncs.validationErrors, field.Name)
	return true
}

// validateAllFields validates all configuration fields
func (ncs *NodeConfigScreen) validateAllFields() bool {
	fields := ncs.getConfigFields()
	valid := true
	
	for _, field := range fields {
		value, exists := ncs.fieldValues[field.Name]
		if !exists {
			value = field.Value
		}
		
		if err := ncs.validateFieldValue(field, value); err != nil {
			ncs.validationErrors[field.Name] = err.Error()
			valid = false
		} else {
			delete(ncs.validationErrors, field.Name)
		}
	}
	
	return valid
}

// validateFieldValue validates a specific field value
func (ncs *NodeConfigScreen) validateFieldValue(field ConfigField, value interface{}) error {
	// Required field check
	if field.Required && (value == nil || value == "" || value == 0) {
		return fmt.Errorf("required field")
	}
	
	// Type-specific validation
	switch field.Type {
	case "number":
		// Ensure it's a valid number and within reasonable bounds
		switch field.Name {
		case "node_id":
			if num, ok := value.(int); ok && (num < 1 || num > 999) {
				return fmt.Errorf("must be between 1 and 999")
			}
		case "max_users":
			if num, ok := value.(int); ok && (num < 1 || num > 10) {
				return fmt.Errorf("must be between 1 and 10")
			}
		case "time_limit":
			if num, ok := value.(int); ok && (num < 1 || num > 1440) {
				return fmt.Errorf("must be between 1 and 1440 minutes")
			}
		case "network_port":
			if num, ok := value.(int); ok && (num < 1 || num > 65535) {
				return fmt.Errorf("must be between 1 and 65535")
			}
		}
	case "select":
		// Ensure value is in options list
		valueStr := fmt.Sprintf("%v", value)
		for _, option := range field.Options {
			if option == valueStr {
				return nil
			}
		}
		return fmt.Errorf("invalid option")
	}
	
	return nil
}

// loadNodeConfig loads the configuration for the selected node
func (ncs *NodeConfigScreen) loadNodeConfig() {
	config, err := ncs.nodeManager.GetNodeConfig(ncs.selectedNode)
	if err != nil {
		ncs.currentConfig = nil
		ncs.validationErrors["general"] = fmt.Sprintf("Failed to load config: %v", err)
		return
	}
	
	ncs.currentConfig = config
	ncs.loadConfigIntoFields()
	delete(ncs.validationErrors, "general")
}

// loadConfigIntoFields loads the current config into field values
func (ncs *NodeConfigScreen) loadConfigIntoFields() {
	if ncs.currentConfig == nil {
		return
	}
	
	ncs.fieldValues = make(map[string]interface{})
	// Values will be loaded on-demand during rendering
}

// applyFieldsToConfig applies field values to the current configuration
func (ncs *NodeConfigScreen) applyFieldsToConfig() {
	if ncs.currentConfig == nil {
		return
	}
	
	for name, value := range ncs.fieldValues {
		switch name {
		case "node_id":
			if num, ok := value.(int); ok {
				ncs.currentConfig.NodeID = num
			}
		case "name":
			ncs.currentConfig.Name = fmt.Sprintf("%v", value)
		case "enabled":
			if str := fmt.Sprintf("%v", value); str == "true" || str == "Yes" {
				ncs.currentConfig.Enabled = true
			} else {
				ncs.currentConfig.Enabled = false
			}
		case "max_users":
			if num, ok := value.(int); ok {
				ncs.currentConfig.MaxUsers = num
			}
		case "time_limit":
			if num, ok := value.(int); ok {
				ncs.currentConfig.TimeLimit = time.Duration(num) * time.Minute
			}
		case "access_level":
			if num, ok := value.(int); ok {
				ncs.currentConfig.AccessLevel = num
			}
		case "local_node":
			if str := fmt.Sprintf("%v", value); str == "true" || str == "Yes" {
				ncs.currentConfig.LocalNode = true
			} else {
				ncs.currentConfig.LocalNode = false
			}
		case "chat_enabled":
			if str := fmt.Sprintf("%v", value); str == "true" || str == "Yes" {
				ncs.currentConfig.ChatEnabled = true
			} else {
				ncs.currentConfig.ChatEnabled = false
			}
		case "network_protocol":
			ncs.currentConfig.NetworkSettings.Protocol = fmt.Sprintf("%v", value)
		case "network_port":
			if num, ok := value.(int); ok {
				ncs.currentConfig.NetworkSettings.Port = num
			}
		case "network_address":
			ncs.currentConfig.NetworkSettings.Address = fmt.Sprintf("%v", value)
		case "allow_doors":
			if str := fmt.Sprintf("%v", value); str == "true" || str == "Yes" {
				ncs.currentConfig.DoorSettings.AllowDoors = true
			} else {
				ncs.currentConfig.DoorSettings.AllowDoors = false
			}
		case "max_door_time":
			if num, ok := value.(int); ok {
				ncs.currentConfig.DoorSettings.MaxDoorTime = num
			}
		case "share_resources":
			if str := fmt.Sprintf("%v", value); str == "true" || str == "Yes" {
				ncs.currentConfig.DoorSettings.ShareResources = true
			} else {
				ncs.currentConfig.DoorSettings.ShareResources = false
			}
		}
	}
}

// renderStatusMessages renders status and error messages
func (ncs *NodeConfigScreen) renderStatusMessages() string {
	var messages []string
	
	if errMsg, hasError := ncs.validationErrors["general"]; hasError {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		messages = append(messages, errorStyle.Render("Error: "+errMsg))
	}
	
	if ncs.isDirty {
		dirtyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
		messages = append(messages, dirtyStyle.Render("Configuration modified - press 's' to save"))
	}
	
	if len(messages) == 0 {
		return ""
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, messages...)
}

// renderHelpLine renders the help line
func (ncs *NodeConfigScreen) renderHelpLine() string {
	if ncs.editMode {
		help := "ENTER:Confirm ESC:Cancel TAB:Next Field"
	} else {
		help := "E:Edit N:New D:Delete C:Copy S:Save R:Reset H:Help ESC:Exit"
	}
	
	helpStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("0")).
		Foreground(lipgloss.Color("7")).
		Width(ncs.width)
	
	return helpStyle.Render(help)
}

// renderHelpScreen renders the help screen
func (ncs *NodeConfigScreen) renderHelpScreen() string {
	helpContent := `
Node Configuration Manager Help

Navigation:
  ↑/↓, k/j     - Select node
  TAB          - Next field (when editing)
  Shift+TAB    - Previous field (when editing)

Commands:
  E, Enter     - Edit selected node
  N            - Create new node
  D            - Delete/disable selected node  
  C            - Copy selected node
  S            - Save changes
  R            - Reset/discard changes
  H, F1        - Toggle this help
  ESC, Q       - Exit

Editing:
  When editing a field:
  - Type to enter text/numbers
  - Space to toggle boolean fields
  - Enter to confirm changes
  - ESC to cancel changes

Field Types:
  Text fields   - Type directly
  Numbers       - Type digits only
  Boolean       - Space to toggle Yes/No
  Select        - Use arrow keys (if implemented)

Notes:
  - Required fields are marked with *
  - Node ID must be unique
  - Changes are not saved until you press 'S'
  - Active nodes with users cannot be deleted
`
	
	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(ncs.width - 4)
	
	return helpStyle.Render(helpContent)
}

// Init implements tea.Model
func (ncs *NodeConfigScreen) Init() tea.Cmd {
	ncs.loadNodeConfig()
	return nil
}