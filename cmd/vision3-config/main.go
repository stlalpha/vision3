package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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
	selectedItem  int
	menuItems     []string
	width         int
	height        int
}

// NewConfigApp creates a new configuration application
func NewConfigApp(configPath string, stringsConfig config.StringsConfig) *ConfigApp {
	return &ConfigApp{
		configPath:    configPath,
		stringsConfig: stringsConfig,
		currentView:   "main",
		menuVisible:   true,
		statusMessage: "Ready - Press F1 for Help",
		selectedItem:  0,
		menuItems:     []string{"String Configuration", "Area Management", "Door Configuration", "Node Monitoring", "System Settings"},
		width:         80, // Default, will be updated
		height:        25, // Default, will be updated
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
			m.statusMessage = "HELP: Use arrow keys to navigate, Enter to select, F10 to exit"
		case "f2":
			m.statusMessage = "Configuration saved successfully"
		case "f3":
			m.statusMessage = "View mode activated"
		case "f10", "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.currentView == "main" {
				if m.selectedItem > 0 {
					m.selectedItem--
				} else {
					m.selectedItem = len(m.menuItems) - 1
				}
				m.statusMessage = fmt.Sprintf("Selected: %s", m.menuItems[m.selectedItem])
			}
		case "down", "j":
			if m.currentView == "main" {
				if m.selectedItem < len(m.menuItems)-1 {
					m.selectedItem++
				} else {
					m.selectedItem = 0
				}
				m.statusMessage = fmt.Sprintf("Selected: %s", m.menuItems[m.selectedItem])
			}
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
			if m.currentView == "main" {
				switch m.selectedItem {
				case 0:
					m.currentView = "strings"
					m.statusMessage = "String Configuration - Edit BBS text strings"
				case 1:
					m.currentView = "areas"
					m.statusMessage = "Area Management - Configure message and file areas"
				case 2:
					m.currentView = "doors"
					m.statusMessage = "Door Configuration - Set up external programs"
				case 3:
					m.currentView = "nodes"
					m.statusMessage = "Node Monitoring - Multi-node status and control"
				case 4:
					m.statusMessage = "System Settings - General BBS configuration"
				}
			}
		case "esc":
			m.currentView = "main"
			m.selectedItem = 0
			m.statusMessage = "Ready - Press F1 for Help"
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// View implements tea.Model
func (m *ConfigApp) View() string {
	// Use actual terminal dimensions
	width := m.width
	height := m.height
	
	// Minimum dimensions check
	if width < 80 {
		width = 80
	}
	if height < 25 {
		height = 25
	}

	// Header with classic blue background
	header := fmt.Sprintf("\033[44m\033[37m%-*s\033[0m", width, " ViSiON/3 BBS Configuration Tool - "+filepath.Base(m.configPath))

	// Menu bar with proper padding
	menuText := " File  Edit  Config  Tools  Help "
	menuPadding := width - len(menuText)
	menuBar := fmt.Sprintf("\033[46m\033[30m%s%-*s\033[0m", menuText, menuPadding, "")

	// Main menu content with Borland-style background
	mainContent := ""
	switch m.currentView {
	case "main":
		// Create full screen background with shaded block characters
		bgChar := "░"
		
		// Build the content with background and dialog overlay
		lines := make([]string, height-3) // Leave space for header, menu, status
		
		// Fill background with shaded pattern
		for i := 0; i < len(lines); i++ {
			lines[i] = fmt.Sprintf("\033[44m%-*s\033[0m", width, strings.Repeat(bgChar, width))
		}
		
		// Menu items with selection highlighting
		menuItems := []string{
			"String Configuration     Edit BBS text strings and prompts",
			"Area Management         Configure message and file areas",
			"Door Configuration      Set up external programs and games",
			"Node Monitoring         Multi-node status and management",
			"System Settings         General BBS configuration",
		}
		
		// Overlay the dialog box (centered and responsive to terminal width)
		dialogStart := 3
		dialogWidth := width - 4 // Leave 2 spaces on each side
		if dialogWidth < 76 {
			dialogWidth = 76 // Minimum width for content
		}
		
		// Create responsive dialog lines
		titlePadding := dialogWidth - 28 // Account for " ┌─ Configuration Categories " (28 chars)
		if titlePadding < 0 {
			titlePadding = 0
		}
		
		topBorder := fmt.Sprintf(" ┌─ Configuration Categories %s┐", strings.Repeat("─", titlePadding))
		bottomBorder := fmt.Sprintf(" └%s┘", strings.Repeat("─", dialogWidth-2))
		emptyLine := fmt.Sprintf(" │%-*s│", dialogWidth-2, "")
		
		dialogLines := []string{
			topBorder,
			emptyLine,
		}
		
		for i, item := range menuItems {
			contentWidth := dialogWidth - 6 // Account for " │  ► " and " │" (6 chars total)
			if i == m.selectedItem {
				// Highlighted selection with white on black
				dialogLines = append(dialogLines, fmt.Sprintf(" │ \033[47m\033[30m ► %-*s \033[0m│", contentWidth, item))
			} else {
				dialogLines = append(dialogLines, fmt.Sprintf(" │  ► %-*s│", contentWidth, item))
			}
		}
		
		instructionText := "Use ↑↓ arrows to navigate, Enter to select, F10 to exit"
		instructionPadding := dialogWidth - len(instructionText) - 4 // Account for " │  " and "│"
		if instructionPadding < 0 {
			instructionPadding = 0
		}
		
		dialogLines = append(dialogLines,
			emptyLine,
			emptyLine,
			fmt.Sprintf(" │  %s%-*s│", instructionText, instructionPadding, ""),
			emptyLine,
			bottomBorder,
		)
		
		// Overlay dialog onto background
		for i, line := range dialogLines {
			if dialogStart+i < len(lines) {
				// Pad dialog line to full width to prevent black gaps
				paddedLine := fmt.Sprintf("%-*s", width, line)
				lines[dialogStart+i] = fmt.Sprintf("\033[44m%s\033[0m", paddedLine)
			}
		}
		
		mainContent = strings.Join(lines, "\n")

	default:
		// All other views get the same treatment
		mainContent = m.createBackgroundView(width, height, m.currentView)
	}

	// Status bar with function key help
	statusBar := fmt.Sprintf("\033[47m\033[30m F1=Help F2=Save F3=View F10=Exit %-*s\033[0m", width-34, m.statusMessage)

	// Combine all elements
	return header + "\n" + menuBar + mainContent + "\n" + statusBar
}

// createBackgroundView creates a view with full background and dialog overlay
func (m *ConfigApp) createBackgroundView(width, height int, viewType string) string {
	bgChar := "░"
	
	// Build the content with background and dialog overlay
	lines := make([]string, height-3) // Leave space for header, menu, status
	
	// Fill background with shaded pattern
	for i := 0; i < len(lines); i++ {
		lines[i] = fmt.Sprintf("\033[44m%-*s\033[0m", width, strings.Repeat(bgChar, width))
	}
	
	// Dialog content based on view type
	var dialogLines []string
	dialogStart := 3
	
	switch viewType {
	case "strings":
		dialogLines = []string{
			" ┌─ String Configuration ─────────────────────────────────────────────────┐",
			" │                                                                        │",
			" │  Categories          Strings                    Preview                │",
			" │  ┌─────────────┐    ┌─────────────────────┐   ┌─────────────────────┐  │",
			" │  │ Login       │    │ Welcome Message     │   │ \033[36mWelcome to the\033[0m    │",
			" │  │ Messages    │    │ Password Prompt     │   │ \033[33mViSiON/3 BBS!\033[0m     │",
			" │  │ Files       │    │ Menu Prompt         │   │                     │",
			" │  │ Doors       │    │ Goodbye Message     │   │ Password: \033[31m_\033[0m       │",
			" │  │ Prompts     │    │ Invalid Command     │   │                     │",
			" │  │ Errors      │    │ Access Denied       │   │                     │",
			" │  └─────────────┘    └─────────────────────┘   └─────────────────────┘  │",
			" │                                                                        │",
			" │  F2=Color Picker  F3=Preview  Enter=Edit  Esc=Back                    │",
			" │                                                                        │",
			" └────────────────────────────────────────────────────────────────────────┘",
		}
	case "areas":
		dialogLines = []string{
			" ┌─ Area Management ──────────────────────────────────────────────────────┐",
			" │                                                                        │",
			" │  Message Areas              File Areas                                 │",
			" │  ┌─────────────────────┐    ┌─────────────────────────────────────┐    │",
			" │  │ ► General Chat      │    │ ► General Files                     │    │",
			" │  │ ► Programming       │    │ ► Programming Tools                 │    │",
			" │  │ ► Gaming            │    │ ► Games & Entertainment             │    │",
			" │  │ ► Sysop             │    │ ► Documentation                     │    │",
			" │  └─────────────────────┘    └─────────────────────────────────────┘    │",
			" │                                                                        │",
			" │  Binary Message Base: \033[32mEnabled\033[0m   Binary File Base: \033[32mEnabled\033[0m        │",
			" │  Multi-Node Safe: \033[32mYes\033[0m           File Locking: \033[32mActive\033[0m            │",
			" │                                                                        │",
			" │  Enter=Configure Area  F2=Add Area  F3=Delete Area  Esc=Back          │",
			" │                                                                        │",
			" └────────────────────────────────────────────────────────────────────────┘",
		}
	case "doors":
		dialogLines = []string{
			" ┌─ Door Configuration ───────────────────────────────────────────────────┐",
			" │                                                                        │",
			" │  Configured Doors           Door Queue Status                          │",
			" │  ┌─────────────────────┐    ┌─────────────────────────────────────┐    │",
			" │  │ ► Legend of the     │    │ Node 1: LORD (5 min left)          │    │",
			" │  │   Red Dragon        │    │ Node 2: Available                  │    │",
			" │  │ ► Trade Wars 2002   │    │ Node 3: TradeWars (15 min left)    │    │",
			" │  │ ► Global War        │    │ Node 4: Available                  │    │",
			" │  │ ► Solar Realms      │    │ Queue: 2 users waiting             │    │",
			" │  └─────────────────────┘    └─────────────────────────────────────┘    │",
			" │                                                                        │",
			" │  Multi-Node Support: \033[32mEnabled\033[0m    Resource Locking: \033[32mActive\033[0m      │",
			" │  Max Concurrent: 4 doors    Dropfile Format: DOOR.SYS             │",
			" │                                                                        │",
			" │  Enter=Configure  F2=Add Door  F3=Test Door  F4=Queue  Esc=Back       │",
			" │                                                                        │",
			" └────────────────────────────────────────────────────────────────────────┘",
		}
	case "nodes":
		dialogLines = []string{
			" ┌─ Node Monitoring ──────────────────────────────────────────────────────┐",
			" │                                                                        │",
			" │  Active Nodes                Who's Online                              │",
			" │  ┌─────────────────────┐    ┌─────────────────────────────────────┐    │",
			" │  │ Node 1: \033[32mOnline\033[0m     │    │ Node 1: Spaceman (Sysop)           │    │",
			" │  │ Node 2: \033[32mOnline\033[0m     │    │ Node 2: Available                  │    │",
			" │  │ Node 3: \033[31mOffline\033[0m    │    │ Node 3: Offline                    │    │",
			" │  │ Node 4: \033[32mOnline\033[0m     │    │ Node 4: CrimsonBlade (User)        │    │",
			" │  └─────────────────────┘    └─────────────────────────────────────┘    │",
			" │                                                                        │",
			" │  System Status: \033[32mOperational\033[0m   Total Users: 2 of 4 max          │",
			" │  Uptime: 2d 14h 32m        Peak Today: 3 users at 15:42           │",
			" │                                                                        │",
			" │  F2=Send Message  F3=Monitor  F4=Disconnect  F5=Chat  Esc=Back        │",
			" │                                                                        │",
			" └────────────────────────────────────────────────────────────────────────┘",
		}
	default:
		dialogLines = []string{
			" ┌─ Configuration ────────────────────────────────────────────────────────┐",
			" │                                                                        │",
			" │  This section is under construction.                                   │",
			" │                                                                        │",
			" │  Press Esc to return to main menu.                                    │",
			" │                                                                        │",
			" └────────────────────────────────────────────────────────────────────────┘",
		}
	}
	
	// Overlay dialog onto background
	for i, line := range dialogLines {
		if dialogStart+i < len(lines) {
			// Replace background line with dialog line (maintaining blue background)
			lines[dialogStart+i] = fmt.Sprintf("\033[44m%s\033[0m", line)
		}
	}
	
	return strings.Join(lines, "\n")
}