package doors

import (
	"fmt"
	"strings"
	
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DoorConfigTUI represents the door configuration TUI interface
type DoorConfigTUI struct {
	width         int
	height        int
	doorManager   DoorManager
	coordinator   *MultiNodeCoordinator
	currentView   DoorView
	selectedDoor  *DoorConfiguration
	selectedIndex int
	doors         []*DoorConfiguration
	menuItems     []string
	focused       bool
	
	// Forms and dialogs
	doorForm      *DoorForm
	wizardState   *WizardState
	testDialog    *TestDialog
	statsDialog   *StatsDialog
	
	// Navigation
	breadcrumbs   []string
	history       []DoorView
}

// DoorView represents different views in the door configuration TUI
type DoorView int

const (
	DoorViewMain DoorView = iota
	DoorViewList
	DoorViewEdit
	DoorViewNew
	DoorViewWizard
	DoorViewTest
	DoorViewStats
	DoorViewMonitor
	DoorViewQueue
	DoorViewResources
	DoorViewTemplates
)

// DoorForm represents a door configuration form
type DoorForm struct {
	fields        []FormField
	currentField  int
	config        *DoorConfiguration
	originalConfig *DoorConfiguration
	modified      bool
	errors        map[string]string
	mode          FormMode
}

// Update implements the tea.Model interface for DoorForm
func (df *DoorForm) Update(msg tea.Msg) (*DoorForm, tea.Cmd) {
	// Basic implementation - handle basic navigation and input
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if df.currentField > 0 {
				df.currentField--
			}
		case "down":
			if df.currentField < len(df.fields)-1 {
				df.currentField++
			}
		}
	}
	return df, nil
}

// FormField represents a field in the door configuration form
type FormField struct {
	Name         string
	Label        string
	Type         FieldType
	Value        interface{}
	Options      []string
	Required     bool
	Validation   string
	Help         string
	Readonly     bool
	Group        string
}

// FieldType represents the type of form field
type FieldType int

const (
	FieldTypeText FieldType = iota
	FieldTypeNumber
	FieldTypeBool
	FieldTypeSelect
	FieldTypeMultiSelect
	FieldTypePath
	FieldTypeTextArea
	FieldTypePassword
	FieldTypeList
)

// FormMode represents the mode of the form
type FormMode int

const (
	FormModeNew FormMode = iota
	FormModeEdit
	FormModeView
)

// WizardState represents the state of the door setup wizard
type WizardState struct {
	currentStep   int
	totalSteps    int
	stepData      map[int]interface{}
	config        *DoorConfiguration
	template      *DoorTemplate
	completed     bool
}

// TestDialog represents a door testing dialog
type TestDialog struct {
	output       []string
	running      bool
	passed       bool
	errors       []string
	currentTest  string
	progress     int
	maxProgress  int
}

// StatsDialog represents a door statistics dialog
type StatsDialog struct {
	stats        *DoorStatistics
	selectedTab  string
	tabs         []string
	chartData    map[string][]float64
}

// NewDoorConfigTUI creates a new door configuration TUI
func NewDoorConfigTUI(doorManager DoorManager, coordinator *MultiNodeCoordinator) *DoorConfigTUI {
	return &DoorConfigTUI{
		doorManager:  doorManager,
		coordinator:  coordinator,
		currentView:  DoorViewMain,
		menuItems: []string{
			"Door List",
			"New Door",
			"Setup Wizard",
			"Templates",
			"Monitor",
			"Queue Status",
			"Resources",
			"Statistics",
			"Test Doors",
			"Import/Export",
		},
		breadcrumbs: []string{"Door Configuration"},
		history:     make([]DoorView, 0),
	}
}

// Init implements tea.Model
func (dt *DoorConfigTUI) Init() tea.Cmd {
	return dt.loadDoors()
}

// Update implements tea.Model
func (dt *DoorConfigTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		dt.width = msg.Width
		dt.height = msg.Height
		
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if dt.currentView == DoorViewMain {
				return dt, tea.Quit
			}
			return dt, dt.goBack()
			
		case "esc":
			return dt, dt.goBack()
			
		case "enter":
			return dt, dt.handleEnter()
			
		case "up", "k":
			return dt, dt.moveUp()
			
		case "down", "j":
			return dt, dt.moveDown()
			
		case "left", "h":
			return dt, dt.moveLeft()
			
		case "right", "l":
			return dt, dt.moveRight()
			
		case "tab":
			return dt, dt.nextField()
			
		case "shift+tab":
			return dt, dt.prevField()
			
		case "f1":
			return dt, dt.showHelp()
			
		case "f2":
			return dt, dt.save()
			
		case "f3":
			return dt, dt.test()
			
		case "f4":
			return dt, dt.showStats()
			
		case "f5":
			return dt, dt.refresh()
			
		case "f9":
			return dt, dt.showWizard()
			
		case "f10":
			return dt, dt.goBack()
			
		case "ctrl+n":
			return dt, dt.newDoor()
			
		case "ctrl+e":
			return dt, dt.editDoor()
			
		case "ctrl+d":
			return dt, dt.deleteDoor()
			
		case "ctrl+t":
			return dt, dt.testDoor()
			
		default:
			// Handle text input for forms
			if dt.currentView == DoorViewEdit || dt.currentView == DoorViewNew {
				return dt, dt.handleTextInput(msg.String())
			}
		}
		
	case DoorsLoadedMsg:
		dt.doors = msg.doors
		
	case DoorSavedMsg:
		return dt, dt.loadDoors()
		
	case TestCompletedMsg:
		if dt.testDialog != nil {
			dt.testDialog.running = false
			dt.testDialog.passed = msg.passed
			dt.testDialog.errors = msg.errors
		}
	}
	
	// Update child components
	if dt.doorForm != nil {
		var cmd tea.Cmd
		dt.doorForm, cmd = dt.doorForm.Update(msg)
		cmds = append(cmds, cmd)
	}
	
	return dt, tea.Batch(cmds...)
}

// View implements tea.Model
func (dt *DoorConfigTUI) View() string {
	if dt.width < 40 || dt.height < 10 {
		return "Terminal too small"
	}
	
	// Create main layout
	header := dt.renderHeader()
	content := dt.renderContent()
	footer := dt.renderFooter()
	
	return lipgloss.JoinVertical(lipgloss.Top,
		header,
		content,
		footer,
	)
}

// renderHeader renders the header section
func (dt *DoorConfigTUI) renderHeader() string {
	// Title bar
	titleStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("1")).  // Blue background
		Foreground(lipgloss.Color("15")). // White text
		Width(dt.width).
		Padding(0, 1)
	
	title := titleStyle.Render("Vision/3 Door Configuration")
	
	// Breadcrumbs
	breadcrumbStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("7")).  // Light gray
		Foreground(lipgloss.Color("0")).  // Black text
		Width(dt.width).
		Padding(0, 1)
	
	breadcrumbText := strings.Join(dt.breadcrumbs, " > ")
	breadcrumbs := breadcrumbStyle.Render(breadcrumbText)
	
	return lipgloss.JoinVertical(lipgloss.Top, title, breadcrumbs)
}

// renderContent renders the main content area
func (dt *DoorConfigTUI) renderContent() string {
	contentHeight := dt.height - 4 // Account for header and footer
	
	contentStyle := lipgloss.NewStyle().
		Width(dt.width).
		Height(contentHeight).
		Background(lipgloss.Color("0")). // Black background
		Foreground(lipgloss.Color("7"))  // Light gray text
	
	var content string
	
	switch dt.currentView {
	case DoorViewMain:
		content = dt.renderMainMenu()
	case DoorViewList:
		content = dt.renderDoorList()
	case DoorViewEdit, DoorViewNew:
		content = dt.renderDoorForm()
	case DoorViewWizard:
		content = dt.renderWizard()
	case DoorViewTest:
		content = dt.renderTestDialog()
	case DoorViewStats:
		content = dt.renderStatsDialog()
	case DoorViewMonitor:
		content = dt.renderMonitor()
	case DoorViewQueue:
		content = dt.renderQueueStatus()
	case DoorViewResources:
		content = dt.renderResources()
	case DoorViewTemplates:
		content = dt.renderTemplates()
	default:
		content = "View not implemented"
	}
	
	return contentStyle.Render(content)
}

// renderFooter renders the footer with function key help
func (dt *DoorConfigTUI) renderFooter() string {
	footerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("7")).  // Light gray
		Foreground(lipgloss.Color("0")).  // Black text
		Width(dt.width)
	
	var keys []string
	
	switch dt.currentView {
	case DoorViewMain:
		keys = []string{"F1-Help", "F5-Refresh", "F9-Wizard", "F10-Exit"}
	case DoorViewList:
		keys = []string{"F1-Help", "F2-Edit", "F3-Test", "F4-Stats", "F9-New", "F10-Back"}
	case DoorViewEdit, DoorViewNew:
		keys = []string{"F1-Help", "F2-Save", "F3-Test", "F10-Cancel"}
	case DoorViewWizard:
		keys = []string{"F1-Help", "F2-Next", "F10-Cancel"}
	default:
		keys = []string{"F1-Help", "F10-Back"}
	}
	
	keyText := strings.Join(keys, "  ")
	return footerStyle.Render(" " + keyText)
}

// renderMainMenu renders the main menu
func (dt *DoorConfigTUI) renderMainMenu() string {
	var lines []string
	
	// Add some spacing
	lines = append(lines, "")
	lines = append(lines, "  Door Configuration Management")
	lines = append(lines, "  " + strings.Repeat("=", 30))
	lines = append(lines, "")
	
	// Menu items
	for i, item := range dt.menuItems {
		prefix := "  "
		if i == dt.selectedIndex {
			prefix = " >"
			item = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(item) // Yellow highlight
		}
		lines = append(lines, prefix + " " + item)
	}
	
	// Add status information
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Total Doors: %d", len(dt.doors)))
	
	if dt.coordinator != nil {
		// Add coordinator status
		lines = append(lines, "  Multi-Node Coordination: Active")
	}
	
	return strings.Join(lines, "\n")
}

// renderDoorList renders the list of doors
func (dt *DoorConfigTUI) renderDoorList() string {
	if len(dt.doors) == 0 {
		return "\n  No doors configured.\n\n  Press F9 to create a new door or run the setup wizard."
	}
	
	var lines []string
	
	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")). // White
		Bold(true)
	
	header := fmt.Sprintf("%-20s %-15s %-10s %-10s %s", 
		"Name", "Category", "Status", "Instances", "Description")
	lines = append(lines, headerStyle.Render(header))
	lines = append(lines, strings.Repeat("-", dt.width-4))
	
	// Door entries
	for i, door := range dt.doors {
		status := "Disabled"
		if door.Enabled {
			status = "Enabled"
		}
		
		instances := "0"
		if dt.coordinator != nil {
			// Get instance count from coordinator
			// instances = strconv.Itoa(dt.coordinator.GetInstanceCount(door.ID))
		}
		
		line := fmt.Sprintf("%-20s %-15s %-10s %-10s %s",
			truncate(door.Name, 20),
			truncate(door.Category.String(), 15),
			status,
			instances,
			truncate(door.Description, 30))
		
		if i == dt.selectedIndex {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("4")). // Blue background
				Foreground(lipgloss.Color("15")). // White text
				Render(line)
		}
		
		lines = append(lines, line)
	}
	
	return strings.Join(lines, "\n")
}

// renderDoorForm renders the door configuration form
func (dt *DoorConfigTUI) renderDoorForm() string {
	if dt.doorForm == nil {
		return "Loading form..."
	}
	
	var lines []string
	
	// Form title
	title := "New Door"
	if dt.doorForm.mode == FormModeEdit {
		title = "Edit Door: " + dt.doorForm.config.Name
	}
	
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")). // White
		Bold(true)
	
	lines = append(lines, titleStyle.Render(title))
	lines = append(lines, strings.Repeat("-", len(title)))
	lines = append(lines, "")
	
	// Group fields by category
	groups := make(map[string][]FormField)
	for _, field := range dt.doorForm.fields {
		group := field.Group
		if group == "" {
			group = "General"
		}
		groups[group] = append(groups[group], field)
	}
	
	// Render groups
	groupOrder := []string{"General", "Execution", "Multi-Node", "Access", "Advanced"}
	
	fieldIndex := 0
	for _, groupName := range groupOrder {
		fields, exists := groups[groupName]
		if !exists {
			continue
		}
		
		// Group header
		groupStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")). // Cyan
			Bold(true)
		
		lines = append(lines, groupStyle.Render(groupName))
		lines = append(lines, "")
		
		// Fields in group
		for _, field := range fields {
			line := dt.renderFormField(field, fieldIndex == dt.doorForm.currentField)
			lines = append(lines, line)
			
			// Show field error if any
			if err, exists := dt.doorForm.errors[field.Name]; exists {
				errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // Red
				lines = append(lines, "  " + errorStyle.Render("Error: " + err))
			}
			
			lines = append(lines, "")
			fieldIndex++
		}
	}
	
	// Form status
	if dt.doorForm.modified {
		modifiedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow
		lines = append(lines, modifiedStyle.Render("Form modified - Press F2 to save"))
	}
	
	return strings.Join(lines, "\n")
}

// renderFormField renders a single form field
func (dt *DoorConfigTUI) renderFormField(field FormField, focused bool) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7")) // Light gray
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15")) // White
	
	if focused {
		labelStyle = labelStyle.Background(lipgloss.Color("4")) // Blue background
		valueStyle = valueStyle.Background(lipgloss.Color("4"))
	}
	
	label := field.Label
	if field.Required {
		label += " *"
	}
	
	// Format value based on type
	valueStr := ""
	switch field.Type {
	case FieldTypeBool:
		if field.Value.(bool) {
			valueStr = "Yes"
		} else {
			valueStr = "No"
		}
	case FieldTypeSelect:
		valueStr = fmt.Sprintf("%v", field.Value)
	case FieldTypeList:
		if list, ok := field.Value.([]string); ok {
			valueStr = strings.Join(list, ", ")
		}
	default:
		valueStr = fmt.Sprintf("%v", field.Value)
	}
	
	// Truncate long values
	if len(valueStr) > 40 {
		valueStr = valueStr[:37] + "..."
	}
	
	line := fmt.Sprintf("  %-20s: %s", 
		labelStyle.Render(label),
		valueStyle.Render(valueStr))
	
	return line
}

// renderWizard renders the door setup wizard
func (dt *DoorConfigTUI) renderWizard() string {
	if dt.wizardState == nil {
		return "Initializing wizard..."
	}
	
	var lines []string
	
	// Wizard header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")). // White
		Bold(true)
	
	title := fmt.Sprintf("Door Setup Wizard - Step %d of %d", 
		dt.wizardState.currentStep, dt.wizardState.totalSteps)
	lines = append(lines, headerStyle.Render(title))
	lines = append(lines, strings.Repeat("=", len(title)))
	lines = append(lines, "")
	
	// Progress bar
	progress := float64(dt.wizardState.currentStep-1) / float64(dt.wizardState.totalSteps)
	progressBar := dt.renderProgressBar(progress, 50)
	lines = append(lines, "  " + progressBar)
	lines = append(lines, "")
	
	// Step content based on current step
	switch dt.wizardState.currentStep {
	case 1:
		lines = append(lines, dt.renderWizardStep1())
	case 2:
		lines = append(lines, dt.renderWizardStep2())
	case 3:
		lines = append(lines, dt.renderWizardStep3())
	case 4:
		lines = append(lines, dt.renderWizardStep4())
	case 5:
		lines = append(lines, dt.renderWizardStep5())
	default:
		lines = append(lines, "Invalid wizard step")
	}
	
	return strings.Join(lines, "\n")
}

// renderProgressBar renders a progress bar
func (dt *DoorConfigTUI) renderProgressBar(progress float64, width int) string {
	filled := int(progress * float64(width))
	empty := width - filled
	
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	percentage := fmt.Sprintf(" %d%%", int(progress*100))
	
	return bar + percentage
}

// renderWizardStep1 renders wizard step 1 (Door Selection)
func (dt *DoorConfigTUI) renderWizardStep1() string {
	return "  Step 1: Select Door Type\n\n" +
		"  Choose the type of door you want to configure:\n\n" +
		"  1. Popular Door Game (LORD, TradeWars, etc.)\n" +
		"  2. Custom Executable\n" +
		"  3. Import from Template\n" +
		"  4. Door Game Archive\n\n" +
		"  Use arrow keys to select and press Enter to continue."
}

// renderWizardStep2 renders wizard step 2 (Basic Settings)
func (dt *DoorConfigTUI) renderWizardStep2() string {
	return "  Step 2: Basic Configuration\n\n" +
		"  Configure basic door settings:\n\n" +
		"  Door Name: ________________\n" +
		"  Description: ________________\n" +
		"  Category: [Select...]\n" +
		"  Command Path: ________________\n\n" +
		"  Fill in the required information and press F2 to continue."
}

// Wizard steps 3-5 would be implemented similarly...

func (dt *DoorConfigTUI) renderWizardStep3() string {
	return "  Step 3: Multi-Node Settings\n\n" +
		"  Configure multi-node behavior:\n\n" +
		"  Multi-Node Type: [Select...]\n" +
		"  Maximum Instances: ____\n" +
		"  Node Rotation: [Yes/No]\n" +
		"  Shared Resources: ________________\n\n" +
		"  Configure multi-node settings and press F2 to continue."
}

func (dt *DoorConfigTUI) renderWizardStep4() string {
	return "  Step 4: Access Control\n\n" +
		"  Configure user access:\n\n" +
		"  Minimum Access Level: ____\n" +
		"  Required Flags: ________________\n" +
		"  Time Limit: ____ minutes\n" +
		"  Available Hours: [Configure...]\n\n" +
		"  Set access restrictions and press F2 to continue."
}

func (dt *DoorConfigTUI) renderWizardStep5() string {
	return "  Step 5: Review and Test\n\n" +
		"  Review your configuration:\n\n" +
		"  Door Name: " + dt.wizardState.config.Name + "\n" +
		"  Command: " + dt.wizardState.config.Command + "\n" +
		"  Multi-Node: " + dt.wizardState.config.MultiNodeType.String() + "\n\n" +
		"  Press F2 to save and test, or F10 to go back."
}

// Command handlers

func (dt *DoorConfigTUI) loadDoors() tea.Cmd {
	return func() tea.Msg {
		doors, err := dt.doorManager.GetAllDoorConfigs()
		if err != nil {
			return ErrorMsg{err}
		}
		return DoorsLoadedMsg{doors}
	}
}

func (dt *DoorConfigTUI) goBack() tea.Cmd {
	if len(dt.history) > 0 {
		// Pop from history
		dt.currentView = dt.history[len(dt.history)-1]
		dt.history = dt.history[:len(dt.history)-1]
		
		// Update breadcrumbs
		if len(dt.breadcrumbs) > 1 {
			dt.breadcrumbs = dt.breadcrumbs[:len(dt.breadcrumbs)-1]
		}
	} else {
		dt.currentView = DoorViewMain
	}
	
	return nil
}

func (dt *DoorConfigTUI) handleEnter() tea.Cmd {
	switch dt.currentView {
	case DoorViewMain:
		return dt.selectMainMenuItem()
	case DoorViewList:
		return dt.editDoor()
	default:
		return nil
	}
}

func (dt *DoorConfigTUI) selectMainMenuItem() tea.Cmd {
	if dt.selectedIndex < 0 || dt.selectedIndex >= len(dt.menuItems) {
		return nil
	}
	
	// Push current view to history
	dt.history = append(dt.history, dt.currentView)
	
	switch dt.selectedIndex {
	case 0: // Door List
		dt.currentView = DoorViewList
		dt.breadcrumbs = append(dt.breadcrumbs, "Door List")
		dt.selectedIndex = 0
	case 1: // New Door
		return dt.newDoor()
	case 2: // Setup Wizard
		return dt.showWizard()
	case 3: // Templates
		dt.currentView = DoorViewTemplates
		dt.breadcrumbs = append(dt.breadcrumbs, "Templates")
	case 4: // Monitor
		dt.currentView = DoorViewMonitor
		dt.breadcrumbs = append(dt.breadcrumbs, "Monitor")
	case 5: // Queue Status
		dt.currentView = DoorViewQueue
		dt.breadcrumbs = append(dt.breadcrumbs, "Queue Status")
	case 6: // Resources
		dt.currentView = DoorViewResources
		dt.breadcrumbs = append(dt.breadcrumbs, "Resources")
	case 7: // Statistics
		return dt.showStats()
	case 8: // Test Doors
		return dt.test()
	}
	
	return nil
}

func (dt *DoorConfigTUI) moveUp() tea.Cmd {
	if dt.selectedIndex > 0 {
		dt.selectedIndex--
	}
	return nil
}

func (dt *DoorConfigTUI) moveDown() tea.Cmd {
	maxIndex := 0
	switch dt.currentView {
	case DoorViewMain:
		maxIndex = len(dt.menuItems) - 1
	case DoorViewList:
		maxIndex = len(dt.doors) - 1
	}
	
	if dt.selectedIndex < maxIndex {
		dt.selectedIndex++
	}
	return nil
}

func (dt *DoorConfigTUI) moveLeft() tea.Cmd {
	// TODO: Implement left navigation for forms
	return nil
}

func (dt *DoorConfigTUI) moveRight() tea.Cmd {
	// TODO: Implement right navigation for forms
	return nil
}

func (dt *DoorConfigTUI) nextField() tea.Cmd {
	if dt.doorForm != nil && dt.doorForm.currentField < len(dt.doorForm.fields)-1 {
		dt.doorForm.currentField++
	}
	return nil
}

func (dt *DoorConfigTUI) prevField() tea.Cmd {
	if dt.doorForm != nil && dt.doorForm.currentField > 0 {
		dt.doorForm.currentField--
	}
	return nil
}

func (dt *DoorConfigTUI) newDoor() tea.Cmd {
	dt.history = append(dt.history, dt.currentView)
	dt.currentView = DoorViewNew
	dt.breadcrumbs = append(dt.breadcrumbs, "New Door")
	
	// Create new door form
	dt.doorForm = dt.createDoorForm(&DoorConfiguration{}, FormModeNew)
	
	return nil
}

func (dt *DoorConfigTUI) editDoor() tea.Cmd {
	if dt.currentView != DoorViewList || dt.selectedIndex >= len(dt.doors) {
		return nil
	}
	
	door := dt.doors[dt.selectedIndex]
	
	dt.history = append(dt.history, dt.currentView)
	dt.currentView = DoorViewEdit
	dt.breadcrumbs = append(dt.breadcrumbs, "Edit: "+door.Name)
	
	// Create edit door form
	dt.doorForm = dt.createDoorForm(door, FormModeEdit)
	
	return nil
}

func (dt *DoorConfigTUI) createDoorForm(config *DoorConfiguration, mode FormMode) *DoorForm {
	form := &DoorForm{
		config:         config,
		originalConfig: config, // TODO: Make a copy
		mode:           mode,
		errors:         make(map[string]string),
		currentField:   0,
	}
	
	// Create form fields
	form.fields = []FormField{
		// General group
		{Name: "Name", Label: "Door Name", Type: FieldTypeText, Value: config.Name, Required: true, Group: "General"},
		{Name: "Description", Label: "Description", Type: FieldTypeTextArea, Value: config.Description, Group: "General"},
		{Name: "Category", Label: "Category", Type: FieldTypeSelect, Value: config.Category, Options: dt.getCategoryOptions(), Group: "General"},
		{Name: "Enabled", Label: "Enabled", Type: FieldTypeBool, Value: config.Enabled, Group: "General"},
		
		// Execution group
		{Name: "Command", Label: "Command", Type: FieldTypePath, Value: config.Command, Required: true, Group: "Execution"},
		{Name: "WorkingDirectory", Label: "Working Directory", Type: FieldTypePath, Value: config.WorkingDirectory, Group: "Execution"},
		{Name: "Arguments", Label: "Arguments", Type: FieldTypeList, Value: config.Arguments, Group: "Execution"},
		{Name: "DropFileType", Label: "Drop File Type", Type: FieldTypeSelect, Value: config.DropFileType, Options: dt.getDropFileOptions(), Group: "Execution"},
		
		// Multi-Node group
		{Name: "MultiNodeType", Label: "Multi-Node Type", Type: FieldTypeSelect, Value: config.MultiNodeType, Options: dt.getMultiNodeOptions(), Group: "Multi-Node"},
		{Name: "MaxInstances", Label: "Max Instances", Type: FieldTypeNumber, Value: config.MaxInstances, Group: "Multi-Node"},
		{Name: "NodeRotation", Label: "Node Rotation", Type: FieldTypeBool, Value: config.NodeRotation, Group: "Multi-Node"},
		
		// Access group
		{Name: "MinimumAccessLevel", Label: "Min Access Level", Type: FieldTypeNumber, Value: config.MinimumAccessLevel, Group: "Access"},
		{Name: "TimeLimit", Label: "Time Limit (min)", Type: FieldTypeNumber, Value: config.TimeLimit, Group: "Access"},
		{Name: "RequiredFlags", Label: "Required Flags", Type: FieldTypeList, Value: config.RequiredFlags, Group: "Access"},
	}
	
	return form
}

func (dt *DoorConfigTUI) getCategoryOptions() []string {
	return []string{
		"Action/Adventure", "Strategy", "Role-Playing", "Puzzle",
		"Card Games", "Board Games", "Trivia", "Utilities",
		"Chat", "Editors", "System", "Custom",
	}
}

func (dt *DoorConfigTUI) getDropFileOptions() []string {
	return []string{
		"DOOR.SYS", "CHAIN.TXT", "DORINFO1.DEF", "CALLINFO.BBS",
		"USERINFO.DAT", "MODINFO.DAT", "Custom", "None",
	}
}

func (dt *DoorConfigTUI) getMultiNodeOptions() []string {
	return []string{
		"Single User", "Shared Data", "Exclusive Instances",
		"Cooperative", "Competitive",
	}
}

// Placeholder implementations for other methods
func (dt *DoorConfigTUI) showWizard() tea.Cmd { return nil }
func (dt *DoorConfigTUI) showHelp() tea.Cmd { return nil }
func (dt *DoorConfigTUI) save() tea.Cmd { return nil }
func (dt *DoorConfigTUI) test() tea.Cmd { return nil }
func (dt *DoorConfigTUI) showStats() tea.Cmd { return nil }
func (dt *DoorConfigTUI) refresh() tea.Cmd { return nil }
func (dt *DoorConfigTUI) deleteDoor() tea.Cmd { return nil }
func (dt *DoorConfigTUI) testDoor() tea.Cmd { return nil }
func (dt *DoorConfigTUI) handleTextInput(input string) tea.Cmd { return nil }

// Placeholder render methods
func (dt *DoorConfigTUI) renderTestDialog() string { return "Test dialog not implemented" }
func (dt *DoorConfigTUI) renderStatsDialog() string { return "Stats dialog not implemented" }
func (dt *DoorConfigTUI) renderMonitor() string { return "Monitor not implemented" }
func (dt *DoorConfigTUI) renderQueueStatus() string { return "Queue status not implemented" }
func (dt *DoorConfigTUI) renderResources() string { return "Resources not implemented" }
func (dt *DoorConfigTUI) renderTemplates() string { return "Templates not implemented" }

// Helper functions
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Message types
type DoorsLoadedMsg struct {
	doors []*DoorConfiguration
}

type DoorSavedMsg struct {
	door *DoorConfiguration
}

type TestCompletedMsg struct {
	passed bool
	errors []string
}

type ErrorMsg struct {
	err error
}