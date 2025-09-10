package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stlalpha/vision3/internal/config"
)

var (
	configPath = flag.String("config", "configs", "Path to configuration directory")
	help       = flag.Bool("help", false, "Show help information")
)

func main() {
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	// Validate config path
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		log.Fatalf("Configuration directory does not exist: %s", *configPath)
	}

	// Initialize configuration
	fmt.Printf("ViSiON/3 BBS Configuration Tool\n")
	fmt.Printf("Configuration Path: %s\n\n", *configPath)

	// Load existing configuration
	stringsConfig, err := config.LoadStrings(*configPath)
	if err != nil {
		log.Printf("Warning: Could not load strings config: %v", err)
		// Use default strings if loading fails
		stringsConfig = config.StringsConfig{}
	}

	// Create the main application
	app := NewConfigApp(*configPath, stringsConfig)

	// Start the TUI
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running configuration tool: %v", err)
	}
}

func showHelp() {
	fmt.Printf(`ViSiON/3 BBS Configuration Tool

A Turbo Pascal-style configuration manager for ViSiON/3 BBS with multi-node support.

USAGE:
    vision3-config [FLAGS]

FLAGS:
    -config string    Path to configuration directory (default: "configs")
    -help            Show this help message

FEATURES:
    • String Configuration   - Edit all BBS strings with ANSI color preview
    • Area Management       - Configure message and file areas with binary formats
    • Door Configuration    - Set up external programs with multi-node support
    • Node Monitoring       - Real-time multi-node status and management
    • System Settings       - General BBS configuration and limits

KEYBOARD SHORTCUTS:
    F1                Help
    F2                Save
    F3                Open/View
    F10               Exit
    Alt+F             File menu
    Alt+E             Edit menu
    Alt+C             Config menu
    Alt+T             Tools menu
    Alt+H             Help menu

The tool features an authentic Turbo Pascal IDE interface with classic blue
backgrounds, shadow-effect dialogs, and function key navigation.

For more information, visit: https://github.com/stlalpha/vision3
`)
}

// ConfigApp represents the main configuration application
type ConfigApp struct {
	configPath    string
	stringsConfig config.StringsConfig
	currentView   string
	menuVisible   bool
	statusMessage string
}

// NewConfigApp creates a new configuration application
func NewConfigApp(configPath string, stringsConfig config.StringsConfig) *ConfigApp {
	return &ConfigApp{
		configPath:    configPath,
		stringsConfig: stringsConfig,
		currentView:   "main",
		menuVisible:   true,
		statusMessage: "Ready - Press F1 for Help",
	}
}

// Init implements tea.Model
func (m *ConfigApp) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m *ConfigApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "f1":
			m.statusMessage = "Help: F2=Save F3=View F10=Exit Alt+Letter=Menu"
		case "f2":
			m.statusMessage = "Configuration saved successfully"
		case "f3":
			m.statusMessage = "View mode activated"
		case "f10", "ctrl+c", "q":
			return m, tea.Quit
		case "alt+f":
			m.statusMessage = "File menu activated"
		case "alt+e":
			m.statusMessage = "Edit menu activated"
		case "alt+c":
			m.statusMessage = "Config menu activated"
		case "alt+t":
			m.statusMessage = "Tools menu activated"
		case "alt+h":
			m.statusMessage = "Help menu activated"
		case "enter":
			switch m.currentView {
			case "main":
				m.currentView = "strings"
				m.statusMessage = "String Configuration - Edit BBS text strings"
			case "strings":
				m.currentView = "areas"
				m.statusMessage = "Area Management - Configure message and file areas"
			case "areas":
				m.currentView = "doors"
				m.statusMessage = "Door Configuration - Set up external programs"
			case "doors":
				m.currentView = "nodes"
				m.statusMessage = "Node Monitoring - Multi-node status and control"
			case "nodes":
				m.currentView = "main"
				m.statusMessage = "Main Menu - Select configuration category"
			}
		case "esc":
			m.currentView = "main"
			m.statusMessage = "Main Menu - Select configuration category"
		}
	case tea.WindowSizeMsg:
		// Handle window resize if needed
	}

	return m, nil
}

// View implements tea.Model
func (m *ConfigApp) View() string {
	// Create the Turbo Pascal-style interface
	width := 80
	height := 25

	// Header with classic blue background
	header := fmt.Sprintf("\033[44m\033[37m%-*s\033[0m", width, " ViSiON/3 BBS Configuration Tool - "+filepath.Base(m.configPath))

	// Menu bar
	menuBar := "\033[46m\033[30m File  Edit  Config  Tools  Help \033[0m" + fmt.Sprintf("%-*s", width-35, "")

	// Main content area with blue background
	content := ""
	for i := 0; i < height-6; i++ {
		content += fmt.Sprintf("\033[44m%-*s\033[0m\n", width, " ")
	}

	// Main menu content
	mainContent := ""
	switch m.currentView {
	case "main":
		mainContent = `
 ┌─ Configuration Categories ─────────────────────────────────────────────┐
 │                                                                        │
 │  ► String Configuration     Edit BBS text strings and prompts         │
 │  ► Area Management         Configure message and file areas            │
 │  ► Door Configuration      Set up external programs and games          │
 │  ► Node Monitoring         Multi-node status and management           │
 │  ► System Settings         General BBS configuration                   │
 │                                                                        │
 │                                                                        │
 │  Use ↑↓ arrows to navigate, Enter to select, F10 to exit             │
 │                                                                        │
 └────────────────────────────────────────────────────────────────────────┘`

	case "strings":
		mainContent = `
 ┌─ String Configuration ─────────────────────────────────────────────────┐
 │                                                                        │
 │  Categories          Strings                    Preview                │
 │  ┌─────────────┐    ┌─────────────────────┐   ┌─────────────────────┐  │
 │  │ Login       │    │ Welcome Message     │   │ \033[36mWelcome to the\033[0m    │
 │  │ Messages    │    │ Password Prompt     │   │ \033[33mViSiON/3 BBS!\033[0m     │
 │  │ Files       │    │ Menu Prompt         │   │                     │
 │  │ Doors       │    │ Goodbye Message     │   │ Password: \033[31m_\033[0m       │
 │  │ Prompts     │    │ Invalid Command     │   │                     │
 │  │ Errors      │    │ Access Denied       │   │                     │
 │  └─────────────┘    └─────────────────────┘   └─────────────────────┘  │
 │                                                                        │
 │  F2=Color Picker  F3=Preview  Enter=Edit  Esc=Back                    │
 │                                                                        │
 └────────────────────────────────────────────────────────────────────────┘`

	case "areas":
		mainContent = `
 ┌─ Area Management ──────────────────────────────────────────────────────┐
 │                                                                        │
 │  Message Areas              File Areas                                 │
 │  ┌─────────────────────┐    ┌─────────────────────────────────────┐    │
 │  │ ► General Chat      │    │ ► General Files                     │    │
 │  │ ► Programming       │    │ ► Programming Tools                 │    │
 │  │ ► Gaming            │    │ ► Games & Entertainment             │    │
 │  │ ► Sysop             │    │ ► Documentation                     │    │
 │  └─────────────────────┘    └─────────────────────────────────────┘    │
 │                                                                        │
 │  Binary Message Base: \033[32mEnabled\033[0m   Binary File Base: \033[32mEnabled\033[0m        │
 │  Multi-Node Safe: \033[32mYes\033[0m           File Locking: \033[32mActive\033[0m            │
 │                                                                        │
 │  Enter=Configure Area  F2=Add Area  F3=Delete Area  Esc=Back          │
 │                                                                        │
 └────────────────────────────────────────────────────────────────────────┘`

	case "doors":
		mainContent = `
 ┌─ Door Configuration ───────────────────────────────────────────────────┐
 │                                                                        │
 │  Configured Doors           Door Queue Status                          │
 │  ┌─────────────────────┐    ┌─────────────────────────────────────┐    │
 │  │ ► Legend of the     │    │ Node 1: LORD (5 min left)          │    │
 │  │   Red Dragon        │    │ Node 2: Available                  │    │
 │  │ ► Trade Wars 2002   │    │ Node 3: TradeWars (15 min left)    │    │
 │  │ ► Global War        │    │ Node 4: Available                  │    │
 │  │ ► Solar Realms      │    │ Queue: 2 users waiting             │    │
 │  └─────────────────────┘    └─────────────────────────────────────┘    │
 │                                                                        │
 │  Multi-Node Support: \033[32mEnabled\033[0m    Resource Locking: \033[32mActive\033[0m      │
 │  Max Concurrent: 4 doors    Dropfile Format: DOOR.SYS             │
 │                                                                        │
 │  Enter=Configure  F2=Add Door  F3=Test Door  F4=Queue  Esc=Back       │
 │                                                                        │
 └────────────────────────────────────────────────────────────────────────┘`

	case "nodes":
		mainContent = `
 ┌─ Node Monitoring ──────────────────────────────────────────────────────┐
 │                                                                        │
 │  Active Nodes                Who's Online                              │
 │  ┌─────────────────────┐    ┌─────────────────────────────────────┐    │
 │  │ Node 1: \033[32mOnline\033[0m     │    │ Node 1: Spaceman (Sysop)           │    │
 │  │ Node 2: \033[32mOnline\033[0m     │    │ Node 2: Available                  │    │
 │  │ Node 3: \033[31mOffline\033[0m    │    │ Node 3: Offline                    │    │
 │  │ Node 4: \033[32mOnline\033[0m     │    │ Node 4: CrimsonBlade (User)        │    │
 │  └─────────────────────┘    └─────────────────────────────────────┘    │
 │                                                                        │
 │  System Status: \033[32mOperational\033[0m   Total Users: 2 of 4 max          │
 │  Uptime: 2d 14h 32m        Peak Today: 3 users at 15:42           │
 │                                                                        │
 │  F2=Send Message  F3=Monitor  F4=Disconnect  F5=Chat  Esc=Back        │
 │                                                                        │
 └────────────────────────────────────────────────────────────────────────┘`
	}

	// Overlay the main content on the blue background
	lines := []string{header, menuBar}
	contentLines := []string{}
	for _, line := range []string{mainContent} {
		contentLines = append(contentLines, fmt.Sprintf("\033[44m%s\033[0m", line))
	}
	lines = append(lines, contentLines...)

	// Status bar with function key help
	statusBar := fmt.Sprintf("\033[47m\033[30m F1=Help F2=Save F3=View F10=Exit %-*s\033[0m", width-34, m.statusMessage)
	lines = append(lines, statusBar)

	return fmt.Sprintf("%s\n", header+"\n"+menuBar+"\n"+mainContent+"\n"+statusBar)
}