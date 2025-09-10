package doors

import (
	"fmt"
	"path/filepath"
	"strings"
	"strconv"
	"time"
	"os"
	"regexp"
	
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DoorWizard represents the door setup wizard
type DoorWizard struct {
	currentStep    int
	totalSteps     int
	steps          []WizardStep
	config         *DoorConfiguration
	template       *DoorTemplate
	selectedOption int
	inputBuffer    string
	inputMode      bool
	errors         []string
	warnings       []string
	width          int
	height         int
	
	// Step-specific data
	doorType       string
	templateList   []*DoorTemplate
	gameList       []PopularGame
	selectedGame   *PopularGame
	detectResults  *DetectionResults
	
	// Form data for each step
	basicData      *BasicDoorData
	execData       *ExecutionData
	multiNodeData  *MultiNodeData
	accessData     *AccessControlData
	advancedData   *AdvancedData
}

// WizardStep represents a step in the wizard
type WizardStep struct {
	ID          string
	Title       string
	Description string
	Required    bool
	Completed   bool
	Handler     func(*DoorWizard) WizardStepResult
}

// WizardStepResult represents the result of a wizard step
type WizardStepResult struct {
	CanProceed   bool
	Errors       []string
	Warnings     []string
	NextStep     int  // -1 for next, 0 for stay, positive for specific step
}

// PopularGame represents a popular door game template
type PopularGame struct {
	Name         string
	Description  string
	Category     DoorCategory
	Template     *DoorTemplate
	DetectPaths  []string
	DetectFiles  []string
	SetupGuide   string
	DownloadURL  string
	Requirements []string
}

// DetectionResults contains results from auto-detection
type DetectionResults struct {
	Found        bool
	GameName     string
	InstallPath  string
	Executable   string
	ConfigFiles  []string
	DataFiles    []string
	Confidence   float64
	Suggestions  []string
}

// Form data structures for wizard steps
type BasicDoorData struct {
	Name        string
	Description string
	Category    DoorCategory
	Version     string
	Author      string
}

type ExecutionData struct {
	Command          string
	WorkingDirectory string
	Arguments        []string
	Environment      map[string]string
	DropFileType     DropFileType
	DropFileLocation string
	IOMode           IOMode
	TerminalType     string
}

type MultiNodeData struct {
	MultiNodeType    MultiNodeType
	MaxInstances     int
	NodeRotation     bool
	SharedResources  []string
	ExclusiveResources []string
}

type AccessControlData struct {
	MinimumAccessLevel int
	RequiredFlags      []string
	ForbiddenFlags     []string
	TimeLimit          int
	DailyTimeLimit     int
	AvailableHours     []TimeSlot
}

type AdvancedData struct {
	PreRunScript    string
	PostRunScript   string
	CleanupScript   string
	CrashHandling   CrashHandlingConfig
	Logging         LoggingConfig
	Monitoring      MonitoringConfig
}

// NewDoorWizard creates a new door setup wizard
func NewDoorWizard() *DoorWizard {
	wizard := &DoorWizard{
		currentStep:   1,
		totalSteps:    7,
		selectedOption: 0,
		config:        &DoorConfiguration{},
		basicData:     &BasicDoorData{},
		execData:      &ExecutionData{},
		multiNodeData: &MultiNodeData{},
		accessData:    &AccessControlData{},
		advancedData:  &AdvancedData{},
	}
	
	wizard.initializeSteps()
	wizard.initializePopularGames()
	
	return wizard
}

// initializeSteps sets up the wizard steps
func (w *DoorWizard) initializeSteps() {
	w.steps = []WizardStep{
		{
			ID:          "welcome",
			Title:       "Welcome",
			Description: "Introduction to the door setup wizard",
			Required:    false,
			Handler:     w.stepWelcome,
		},
		{
			ID:          "door_type",
			Title:       "Door Type Selection",
			Description: "Choose the type of door to configure",
			Required:    true,
			Handler:     w.stepDoorType,
		},
		{
			ID:          "game_selection",
			Title:       "Game Selection",
			Description: "Select a specific door game or template",
			Required:    true,
			Handler:     w.stepGameSelection,
		},
		{
			ID:          "basic_config",
			Title:       "Basic Configuration",
			Description: "Configure basic door settings",
			Required:    true,
			Handler:     w.stepBasicConfig,
		},
		{
			ID:          "execution_config",
			Title:       "Execution Settings",
			Description: "Configure how the door is executed",
			Required:    true,
			Handler:     w.stepExecutionConfig,
		},
		{
			ID:          "multinode_config",
			Title:       "Multi-Node Settings",
			Description: "Configure multi-node behavior",
			Required:    false,
			Handler:     w.stepMultiNodeConfig,
		},
		{
			ID:          "review",
			Title:       "Review and Test",
			Description: "Review configuration and test the door",
			Required:    true,
			Handler:     w.stepReview,
		},
	}
}

// initializePopularGames sets up the list of popular door games
func (w *DoorWizard) initializePopularGames() {
	w.gameList = []PopularGame{
		{
			Name:        "Legend of the Red Dragon (LORD)",
			Description: "Classic fantasy role-playing door game",
			Category:    CategoryRPG,
			DetectPaths: []string{"LORD", "lord", "red_dragon"},
			DetectFiles: []string{"lord.exe", "start.bat", "lord.cfg"},
			SetupGuide:  "Install LORD to a directory and configure the maintenance events.",
			Requirements: []string{"DOS/Windows executable", "Daily maintenance", "User files directory"},
		},
		{
			Name:        "TradeWars 2002",
			Description: "Space trading and combat game",
			Category:    CategoryStrategy,
			DetectPaths: []string{"TW2002", "tw", "tradewars"},
			DetectFiles: []string{"tw2002.exe", "twterm.exe", "bigbang.exe"},
			SetupGuide:  "Install TradeWars 2002 and configure the game settings.",
			Requirements: []string{"DOS/Windows executable", "Game universe creation", "Player data files"},
		},
		{
			Name:        "Barren Realms Elite (BRE)",
			Description: "Strategic war simulation game",
			Category:    CategoryStrategy,
			DetectPaths: []string{"BRE", "bre", "barren"},
			DetectFiles: []string{"bre.exe", "breterm.exe"},
			SetupGuide:  "Install BRE and set up the game universe.",
			Requirements: []string{"DOS/Windows executable", "Universe files", "Player data"},
		},
		{
			Name:        "Food Fight",
			Description: "Simple action door game",
			Category:    CategoryAction,
			DetectPaths: []string{"FOOD", "food", "foodfight"},
			DetectFiles: []string{"food.exe", "fight.exe"},
			SetupGuide:  "Install Food Fight executable.",
			Requirements: []string{"DOS/Windows executable"},
		},
		{
			Name:        "Global War",
			Description: "World domination strategy game",
			Category:    CategoryStrategy,
			DetectPaths: []string{"GW", "gwar", "globalwar"},
			DetectFiles: []string{"gwar.exe", "global.exe"},
			SetupGuide:  "Install Global War and configure world settings.",
			Requirements: []string{"DOS/Windows executable", "World data files"},
		},
		{
			Name:        "Planets TEOS",
			Description: "Space exploration and colonization game",
			Category:    CategoryStrategy,
			DetectPaths: []string{"PLANETS", "planets", "teos"},
			DetectFiles: []string{"planets.exe", "teos.exe"},
			SetupGuide:  "Install Planets and set up the universe.",
			Requirements: []string{"DOS/Windows executable", "Universe files", "Race data"},
		},
		{
			Name:        "Operation Overkill",
			Description: "Post-apocalyptic role-playing game",
			Category:    CategoryRPG,
			DetectPaths: []string{"OO", "ooii", "overkill"},
			DetectFiles: []string{"oo.exe", "ooii.exe", "overkill.exe"},
			SetupGuide:  "Install Operation Overkill and configure player data.",
			Requirements: []string{"DOS/Windows executable", "Player files", "Game data"},
		},
		{
			Name:        "The Pit",
			Description: "Gladiator combat arena game",
			Category:    CategoryAction,
			DetectPaths: []string{"PIT", "pit", "thepit"},
			DetectFiles: []string{"pit.exe", "arena.exe"},
			SetupGuide:  "Install The Pit executable.",
			Requirements: []string{"DOS/Windows executable", "Fighter data"},
		},
	}
}

// Update implements tea.Model
func (w *DoorWizard) Update(msg tea.Msg) (*DoorWizard, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height
		
	case tea.KeyMsg:
		if w.inputMode {
			return w.handleInputMode(msg)
		}
		
		switch msg.String() {
		case "up", "k":
			if w.selectedOption > 0 {
				w.selectedOption--
			}
		case "down", "j":
			w.selectedOption++
		case "enter":
			return w.handleEnter()
		case "esc":
			if w.currentStep > 1 {
				w.currentStep--
			}
		case "f2":
			return w.nextStep()
		case "f10":
			// Cancel wizard
			return w, tea.Quit
		case "ctrl+d":
			return w.autoDetect()
		case "ctrl+t":
			return w.testCurrentConfig()
		}
	}
	
	return w, nil
}

// View implements tea.Model
func (w *DoorWizard) View() string {
	if w.width < 40 || w.height < 10 {
		return "Terminal too small for wizard"
	}
	
	var sections []string
	
	// Header
	sections = append(sections, w.renderHeader())
	
	// Current step content
	sections = append(sections, w.renderCurrentStep())
	
	// Footer
	sections = append(sections, w.renderFooter())
	
	return lipgloss.JoinVertical(lipgloss.Top, sections...)
}

// renderHeader renders the wizard header
func (w *DoorWizard) renderHeader() string {
	// Title
	titleStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("4")).  // Blue
		Foreground(lipgloss.Color("15")). // White
		Bold(true).
		Width(w.width).
		Align(lipgloss.Center).
		Padding(0, 1)
	
	title := fmt.Sprintf("Door Setup Wizard - Step %d of %d", w.currentStep, w.totalSteps)
	
	// Progress bar
	progressStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("0")).  // Black
		Foreground(lipgloss.Color("7")).  // Light gray
		Width(w.width).
		Padding(0, 2)
	
	progress := float64(w.currentStep-1) / float64(w.totalSteps-1)
	progressBar := w.renderProgressBar(progress, w.width-8)
	
	// Step title
	stepStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("7")).  // Light gray
		Foreground(lipgloss.Color("0")).  // Black
		Bold(true).
		Width(w.width).
		Padding(0, 2)
	
	stepTitle := fmt.Sprintf("%s: %s", w.steps[w.currentStep-1].ID, w.steps[w.currentStep-1].Title)
	
	return lipgloss.JoinVertical(lipgloss.Top,
		titleStyle.Render(title),
		progressStyle.Render(progressBar),
		stepStyle.Render(stepTitle),
	)
}

// renderCurrentStep renders the current wizard step
func (w *DoorWizard) renderCurrentStep() string {
	contentStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("0")).  // Black
		Foreground(lipgloss.Color("7")).  // Light gray
		Width(w.width).
		Height(w.height - 6). // Account for header and footer
		Padding(1, 2)
	
	var content string
	
	switch w.currentStep {
	case 1:
		content = w.renderWelcomeStep()
	case 2:
		content = w.renderDoorTypeStep()
	case 3:
		content = w.renderGameSelectionStep()
	case 4:
		content = w.renderBasicConfigStep()
	case 5:
		content = w.renderExecutionConfigStep()
	case 6:
		content = w.renderMultiNodeConfigStep()
	case 7:
		content = w.renderReviewStep()
	default:
		content = "Invalid step"
	}
	
	// Add errors and warnings
	if len(w.errors) > 0 {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // Red
		content += "\n\nErrors:\n"
		for _, err := range w.errors {
			content += errorStyle.Render("• " + err) + "\n"
		}
	}
	
	if len(w.warnings) > 0 {
		warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow
		content += "\n\nWarnings:\n"
		for _, warn := range w.warnings {
			content += warningStyle.Render("• " + warn) + "\n"
		}
	}
	
	return contentStyle.Render(content)
}

// renderFooter renders the wizard footer
func (w *DoorWizard) renderFooter() string {
	footerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("7")).  // Light gray
		Foreground(lipgloss.Color("0")).  // Black
		Width(w.width).
		Padding(0, 1)
	
	var keys []string
	
	if w.currentStep > 1 {
		keys = append(keys, "Esc-Back")
	}
	
	keys = append(keys, "F2-Next", "Ctrl+D-Detect", "Ctrl+T-Test", "F10-Cancel")
	
	if w.inputMode {
		keys = []string{"Enter-Confirm", "Esc-Cancel"}
	}
	
	return footerStyle.Render(strings.Join(keys, "  "))
}

// Step rendering methods

func (w *DoorWizard) renderWelcomeStep() string {
	return `Welcome to the Door Setup Wizard!

This wizard will help you configure a door game for your BBS system.
The wizard can help you:

• Set up popular door games like LORD, TradeWars, and BRE
• Configure custom executable programs
• Import door configurations from templates
• Set up multi-node door access
• Configure user access controls
• Test your door configuration

The wizard will guide you through each step and can auto-detect
many popular door games if they are already installed.

Press F2 to continue or F10 to cancel.`
}

func (w *DoorWizard) renderDoorTypeStep() string {
	options := []string{
		"Popular Door Game - Choose from a list of well-known door games",
		"Custom Executable - Configure a custom program or script",
		"Import Template - Use an existing door configuration template",
		"Auto-Detect - Scan for installed door games",
		"Door Archive - Extract and configure from a door game archive",
	}
	
	content := "Select the type of door you want to configure:\n\n"
	
	for i, option := range options {
		prefix := "  "
		if i == w.selectedOption {
			prefix = "► "
			option = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(option)
		}
		content += prefix + option + "\n"
	}
	
	content += "\nUse arrow keys to select and press Enter to continue."
	
	return content
}

func (w *DoorWizard) renderGameSelectionStep() string {
	if w.doorType == "popular" {
		content := "Select a popular door game:\n\n"
		
		for i, game := range w.gameList {
			prefix := "  "
			if i == w.selectedOption {
				prefix = "► "
				w.selectedGame = &w.gameList[i]
			}
			
			line := fmt.Sprintf("%s - %s", game.Name, game.Description)
			if i == w.selectedOption {
				line = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(line)
			}
			
			content += prefix + line + "\n"
		}
		
		if w.selectedGame != nil {
			content += "\nGame Details:\n"
			content += fmt.Sprintf("Category: %s\n", w.selectedGame.Category.String())
			content += fmt.Sprintf("Requirements: %s\n", strings.Join(w.selectedGame.Requirements, ", "))
			content += fmt.Sprintf("Setup: %s\n", w.selectedGame.SetupGuide)
		}
		
		return content
	}
	
	if w.doorType == "auto_detect" {
		if w.detectResults == nil {
			return "Auto-detecting door games...\n\nPress Ctrl+D to start detection."
		}
		
		content := "Auto-Detection Results:\n\n"
		if w.detectResults.Found {
			content += fmt.Sprintf("Found: %s\n", w.detectResults.GameName)
			content += fmt.Sprintf("Path: %s\n", w.detectResults.InstallPath)
			content += fmt.Sprintf("Executable: %s\n", w.detectResults.Executable)
			content += fmt.Sprintf("Confidence: %.0f%%\n\n", w.detectResults.Confidence*100)
			
			if len(w.detectResults.Suggestions) > 0 {
				content += "Suggestions:\n"
				for _, suggestion := range w.detectResults.Suggestions {
					content += "• " + suggestion + "\n"
				}
			}
		} else {
			content += "No door games were auto-detected.\n\n"
			content += "Try:\n"
			content += "• Installing a door game first\n"
			content += "• Using manual configuration\n"
			content += "• Checking file permissions\n"
		}
		
		return content
	}
	
	return "Game selection not implemented for this door type."
}

func (w *DoorWizard) renderBasicConfigStep() string {
	content := "Basic Door Configuration:\n\n"
	
	fields := []struct {
		name  string
		value string
		desc  string
	}{
		{"Door Name", w.basicData.Name, "Display name for the door"},
		{"Description", w.basicData.Description, "Brief description of the door"},
		{"Category", w.basicData.Category.String(), "Door category"},
		{"Version", w.basicData.Version, "Door version"},
		{"Author", w.basicData.Author, "Door author"},
	}
	
	for i, field := range fields {
		prefix := "  "
		if i == w.selectedOption {
			prefix = "► "
			if w.inputMode {
				field.value = w.inputBuffer
			}
		}
		
		line := fmt.Sprintf("%-12s: %s", field.name, field.value)
		if i == w.selectedOption {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(line)
		}
		
		content += prefix + line + "\n"
		
		if i == w.selectedOption {
			content += fmt.Sprintf("    %s\n", field.desc)
		}
	}
	
	content += "\nUse arrow keys to navigate, Enter to edit, F2 to continue."
	
	return content
}

func (w *DoorWizard) renderExecutionConfigStep() string {
	content := "Execution Configuration:\n\n"
	
	fields := []struct {
		name  string
		value string
		desc  string
	}{
		{"Command", w.execData.Command, "Path to the door executable"},
		{"Working Dir", w.execData.WorkingDirectory, "Working directory for the door"},
		{"Arguments", strings.Join(w.execData.Arguments, " "), "Command line arguments"},
		{"Drop File", w.execData.DropFileType.String(), "Type of drop file to generate"},
		{"I/O Mode", w.execData.IOMode.String(), "Input/output mode"},
		{"Terminal", w.execData.TerminalType, "Terminal type (ANSI, VT100, etc.)"},
	}
	
	for i, field := range fields {
		prefix := "  "
		if i == w.selectedOption {
			prefix = "► "
			if w.inputMode {
				field.value = w.inputBuffer
			}
		}
		
		line := fmt.Sprintf("%-12s: %s", field.name, field.value)
		if i == w.selectedOption {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(line)
		}
		
		content += prefix + line + "\n"
		
		if i == w.selectedOption {
			content += fmt.Sprintf("    %s\n", field.desc)
		}
	}
	
	content += "\nPress Ctrl+D to auto-detect settings based on the executable."
	
	return content
}

func (w *DoorWizard) renderMultiNodeConfigStep() string {
	content := "Multi-Node Configuration:\n\n"
	
	fields := []struct {
		name  string
		value string
		desc  string
	}{
		{"Node Type", w.multiNodeData.MultiNodeType.String(), "Multi-node capability"},
		{"Max Instances", strconv.Itoa(w.multiNodeData.MaxInstances), "Maximum concurrent instances"},
		{"Node Rotation", fmt.Sprintf("%t", w.multiNodeData.NodeRotation), "Use round-robin node assignment"},
		{"Shared Resources", strings.Join(w.multiNodeData.SharedResources, ", "), "Files shared between instances"},
		{"Exclusive Resources", strings.Join(w.multiNodeData.ExclusiveResources, ", "), "Files requiring exclusive access"},
	}
	
	for i, field := range fields {
		prefix := "  "
		if i == w.selectedOption {
			prefix = "► "
			if w.inputMode {
				field.value = w.inputBuffer
			}
		}
		
		line := fmt.Sprintf("%-18s: %s", field.name, field.value)
		if i == w.selectedOption {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(line)
		}
		
		content += prefix + line + "\n"
		
		if i == w.selectedOption {
			content += fmt.Sprintf("    %s\n", field.desc)
		}
	}
	
	content += "\nNote: Multi-node settings are optional but recommended for busy systems."
	
	return content
}

func (w *DoorWizard) renderReviewStep() string {
	content := "Review Configuration:\n\n"
	
	// Basic info
	content += lipgloss.NewStyle().Bold(true).Render("Basic Information:") + "\n"
	content += fmt.Sprintf("  Name: %s\n", w.basicData.Name)
	content += fmt.Sprintf("  Description: %s\n", w.basicData.Description)
	content += fmt.Sprintf("  Category: %s\n", w.basicData.Category.String())
	content += "\n"
	
	// Execution
	content += lipgloss.NewStyle().Bold(true).Render("Execution:") + "\n"
	content += fmt.Sprintf("  Command: %s\n", w.execData.Command)
	content += fmt.Sprintf("  Working Directory: %s\n", w.execData.WorkingDirectory)
	content += fmt.Sprintf("  Drop File: %s\n", w.execData.DropFileType.String())
	content += "\n"
	
	// Multi-node
	content += lipgloss.NewStyle().Bold(true).Render("Multi-Node:") + "\n"
	content += fmt.Sprintf("  Type: %s\n", w.multiNodeData.MultiNodeType.String())
	content += fmt.Sprintf("  Max Instances: %d\n", w.multiNodeData.MaxInstances)
	content += "\n"
	
	// Options
	options := []string{
		"Save and Test Configuration",
		"Save Configuration Only",
		"Back to Edit",
	}
	
	for i, option := range options {
		prefix := "  "
		if i == w.selectedOption {
			prefix = "► "
			option = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(option)
		}
		content += prefix + option + "\n"
	}
	
	return content
}

// Event handlers

func (w *DoorWizard) handleEnter() (*DoorWizard, tea.Cmd) {
	result := w.steps[w.currentStep-1].Handler(w)
	
	w.errors = result.Errors
	w.warnings = result.Warnings
	
	if result.CanProceed {
		if result.NextStep > 0 {
			w.currentStep = result.NextStep
		} else if result.NextStep == -1 {
			return w.nextStep()
		}
	}
	
	return w, nil
}

func (w *DoorWizard) nextStep() (*DoorWizard, tea.Cmd) {
	if w.currentStep < w.totalSteps {
		w.currentStep++
		w.selectedOption = 0
		w.inputMode = false
		w.inputBuffer = ""
	}
	return w, nil
}

func (w *DoorWizard) handleInputMode(msg tea.KeyMsg) (*DoorWizard, tea.Cmd) {
	switch msg.String() {
	case "enter":
		w.inputMode = false
		// Apply the input to the current field
		w.applyInput()
	case "esc":
		w.inputMode = false
		w.inputBuffer = ""
	case "backspace":
		if len(w.inputBuffer) > 0 {
			w.inputBuffer = w.inputBuffer[:len(w.inputBuffer)-1]
		}
	default:
		// Add printable characters to buffer
		if len(msg.String()) == 1 && msg.String()[0] >= 32 {
			w.inputBuffer += msg.String()
		}
	}
	
	return w, nil
}

func (w *DoorWizard) applyInput() {
	switch w.currentStep {
	case 4: // Basic config
		switch w.selectedOption {
		case 0:
			w.basicData.Name = w.inputBuffer
		case 1:
			w.basicData.Description = w.inputBuffer
		case 3:
			w.basicData.Version = w.inputBuffer
		case 4:
			w.basicData.Author = w.inputBuffer
		}
	case 5: // Execution config
		switch w.selectedOption {
		case 0:
			w.execData.Command = w.inputBuffer
		case 1:
			w.execData.WorkingDirectory = w.inputBuffer
		case 2:
			w.execData.Arguments = strings.Fields(w.inputBuffer)
		case 5:
			w.execData.TerminalType = w.inputBuffer
		}
	case 6: // Multi-node config
		switch w.selectedOption {
		case 1:
			if val, err := strconv.Atoi(w.inputBuffer); err == nil {
				w.multiNodeData.MaxInstances = val
			}
		case 3:
			w.multiNodeData.SharedResources = strings.Split(w.inputBuffer, ",")
		case 4:
			w.multiNodeData.ExclusiveResources = strings.Split(w.inputBuffer, ",")
		}
	}
	
	w.inputBuffer = ""
}

func (w *DoorWizard) autoDetect() (*DoorWizard, tea.Cmd) {
	// Simulate auto-detection
	w.detectResults = &DetectionResults{
		Found:       false,
		Confidence:  0.0,
		Suggestions: []string{"Install a door game to a standard location", "Check file permissions"},
	}
	
	// Try to detect popular door games
	searchPaths := []string{
		"/opt/bbs/doors",
		"/bbs/doors",
		"C:\\BBS\\DOORS",
		"C:\\DOORS",
		"/usr/local/bbs/doors",
	}
	
	for _, basePath := range searchPaths {
		if _, err := os.Stat(basePath); err == nil {
			// Search for door games in this path
			if result := w.scanForDoors(basePath); result != nil {
				w.detectResults = result
				break
			}
		}
	}
	
	return w, nil
}

func (w *DoorWizard) scanForDoors(basePath string) *DetectionResults {
	for _, game := range w.gameList {
		for _, detectPath := range game.DetectPaths {
			fullPath := filepath.Join(basePath, detectPath)
			if _, err := os.Stat(fullPath); err == nil {
				// Found potential game directory, check for executables
				for _, detectFile := range game.DetectFiles {
					exePath := filepath.Join(fullPath, detectFile)
					if _, err := os.Stat(exePath); err == nil {
						return &DetectionResults{
							Found:       true,
							GameName:    game.Name,
							InstallPath: fullPath,
							Executable:  exePath,
							Confidence:  0.9,
							Suggestions: []string{
								"Verify the executable path",
								"Check game configuration files",
								"Test the door before deployment",
							},
						}
					}
				}
			}
		}
	}
	
	return nil
}

func (w *DoorWizard) testCurrentConfig() (*DoorWizard, tea.Cmd) {
	// TODO: Implement door testing
	return w, nil
}

// Step handlers

func (w *DoorWizard) stepWelcome(wizard *DoorWizard) WizardStepResult {
	return WizardStepResult{CanProceed: true, NextStep: -1}
}

func (w *DoorWizard) stepDoorType(wizard *DoorWizard) WizardStepResult {
	switch w.selectedOption {
	case 0:
		w.doorType = "popular"
	case 1:
		w.doorType = "custom"
	case 2:
		w.doorType = "template"
	case 3:
		w.doorType = "auto_detect"
	case 4:
		w.doorType = "archive"
	default:
		return WizardStepResult{
			CanProceed: false,
			Errors:     []string{"Please select a door type"},
		}
	}
	
	return WizardStepResult{CanProceed: true, NextStep: -1}
}

func (w *DoorWizard) stepGameSelection(wizard *DoorWizard) WizardStepResult {
	if w.doorType == "popular" && w.selectedGame == nil {
		return WizardStepResult{
			CanProceed: false,
			Errors:     []string{"Please select a door game"},
		}
	}
	
	if w.doorType == "auto_detect" && (w.detectResults == nil || !w.detectResults.Found) {
		return WizardStepResult{
			CanProceed: false,
			Errors:     []string{"No door games detected. Try manual configuration."},
		}
	}
	
	// Pre-populate basic data based on selection
	if w.selectedGame != nil {
		w.basicData.Name = w.selectedGame.Name
		w.basicData.Description = w.selectedGame.Description
		w.basicData.Category = w.selectedGame.Category
	}
	
	if w.detectResults != nil && w.detectResults.Found {
		w.basicData.Name = w.detectResults.GameName
		w.execData.Command = w.detectResults.Executable
		w.execData.WorkingDirectory = w.detectResults.InstallPath
	}
	
	return WizardStepResult{CanProceed: true, NextStep: -1}
}

func (w *DoorWizard) stepBasicConfig(wizard *DoorWizard) WizardStepResult {
	errors := make([]string, 0)
	
	if w.basicData.Name == "" {
		errors = append(errors, "Door name is required")
	}
	
	if w.basicData.Description == "" {
		errors = append(errors, "Door description is required")
	}
	
	if len(errors) > 0 {
		return WizardStepResult{
			CanProceed: false,
			Errors:     errors,
		}
	}
	
	return WizardStepResult{CanProceed: true, NextStep: -1}
}

func (w *DoorWizard) stepExecutionConfig(wizard *DoorWizard) WizardStepResult {
	errors := make([]string, 0)
	warnings := make([]string, 0)
	
	if w.execData.Command == "" {
		errors = append(errors, "Command path is required")
	} else {
		// Check if command exists
		if _, err := os.Stat(w.execData.Command); err != nil {
			warnings = append(warnings, "Command file not found: "+w.execData.Command)
		}
	}
	
	if w.execData.WorkingDirectory != "" {
		if _, err := os.Stat(w.execData.WorkingDirectory); err != nil {
			warnings = append(warnings, "Working directory not found: "+w.execData.WorkingDirectory)
		}
	}
	
	if len(errors) > 0 {
		return WizardStepResult{
			CanProceed: false,
			Errors:     errors,
			Warnings:   warnings,
		}
	}
	
	return WizardStepResult{
		CanProceed: true,
		NextStep:   -1,
		Warnings:   warnings,
	}
}

func (w *DoorWizard) stepMultiNodeConfig(wizard *DoorWizard) WizardStepResult {
	warnings := make([]string, 0)
	
	if w.multiNodeData.MaxInstances <= 0 {
		w.multiNodeData.MaxInstances = 1
		warnings = append(warnings, "Max instances set to 1 (minimum value)")
	}
	
	if w.multiNodeData.MaxInstances > 10 {
		warnings = append(warnings, "High instance count may impact system performance")
	}
	
	return WizardStepResult{
		CanProceed: true,
		NextStep:   -1,
		Warnings:   warnings,
	}
}

func (w *DoorWizard) stepReview(wizard *DoorWizard) WizardStepResult {
	switch w.selectedOption {
	case 0: // Save and test
		w.buildFinalConfig()
		// TODO: Save and test
		return WizardStepResult{CanProceed: true, NextStep: 0} // Stay on this step to show results
	case 1: // Save only
		w.buildFinalConfig()
		// TODO: Save configuration
		return WizardStepResult{CanProceed: true, NextStep: 0}
	case 2: // Back to edit
		return WizardStepResult{CanProceed: true, NextStep: 4} // Go back to basic config
	}
	
	return WizardStepResult{CanProceed: false}
}

// buildFinalConfig constructs the final door configuration
func (w *DoorWizard) buildFinalConfig() {
	w.config = &DoorConfiguration{
		ID:          w.generateDoorID(),
		Name:        w.basicData.Name,
		Description: w.basicData.Description,
		Category:    w.basicData.Category,
		Version:     w.basicData.Version,
		Author:      w.basicData.Author,
		
		Command:          w.execData.Command,
		Arguments:        w.execData.Arguments,
		WorkingDirectory: w.execData.WorkingDirectory,
		DropFileType:     w.execData.DropFileType,
		IOMode:           w.execData.IOMode,
		TerminalType:     w.execData.TerminalType,
		
		MultiNodeType: w.multiNodeData.MultiNodeType,
		MaxInstances:  w.multiNodeData.MaxInstances,
		NodeRotation:  w.multiNodeData.NodeRotation,
		SharedResources:   w.multiNodeData.SharedResources,
		ExclusiveResources: w.multiNodeData.ExclusiveResources,
		
		MinimumAccessLevel: w.accessData.MinimumAccessLevel,
		RequiredFlags:      w.accessData.RequiredFlags,
		ForbiddenFlags:     w.accessData.ForbiddenFlags,
		TimeLimit:          w.accessData.TimeLimit,
		DailyTimeLimit:     w.accessData.DailyTimeLimit,
		
		Enabled:      true,
		Created:      time.Now(),
		Modified:     time.Now(),
		ConfigVersion: 1,
	}
}

func (w *DoorWizard) generateDoorID() string {
	// Generate a unique ID based on the door name
	name := strings.ToLower(w.basicData.Name)
	name = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	
	if name == "" {
		name = "door"
	}
	
	// Add timestamp to ensure uniqueness
	return fmt.Sprintf("%s_%d", name, time.Now().Unix())
}

func (w *DoorWizard) renderProgressBar(progress float64, width int) string {
	if width <= 0 {
		return ""
	}
	
	filled := int(progress * float64(width))
	if filled > width {
		filled = width
	}
	
	empty := width - filled
	
	progressStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Green
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))     // Dark gray
	
	bar := progressStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))
	
	percentage := fmt.Sprintf(" %d%%", int(progress*100))
	
	return bar + percentage
}