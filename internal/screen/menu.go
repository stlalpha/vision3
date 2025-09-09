package screen

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MenuCommand represents a single menu command
type MenuCommand struct {
	Keys        string // Command keys (e.g., "M", "G", "/W")
	Action      string // Command action (e.g., "GOTO:MSGMENU", "RUN:SHOWSTATS", "LOGOFF")
	ACS         string // Access control string (e.g., "*", "s255")
	Description string // Human readable description
	Hidden      bool   // Whether command should be hidden from help
}

// MenuConfig represents a complete menu configuration
type MenuConfig struct {
	// Menu Properties
	Name        string        // Human readable menu name
	ANSIFile    string        // ANSI file to display (relative to ANSI path)
	Prompt      string        // Prompt template from strings.json
	Position    ScreenPosition // Screen positioning (CENTER, TOP, OFFSET)
	Offset      int           // Offset for OFFSET positioning
	Theme       string        // Theme name (for future use)
	Parent      string        // Parent menu (for breadcrumbs/navigation)
	
	// Commands
	Commands    []MenuCommand // List of menu commands
}

// MenuManager handles loading and managing menu configurations
type MenuManager struct {
	menuPath    string                    // Base path for menu files
	ansiPath    string                    // Path to ANSI files
	menus       map[string]*MenuConfig    // Loaded menu configurations
	startMenu   string                    // Starting menu after authentication
}

// NewMenuManager creates a new menu manager
func NewMenuManager(menuPath, ansiPath, startMenu string) *MenuManager {
	return &MenuManager{
		menuPath:  menuPath,
		ansiPath:  ansiPath,
		menus:     make(map[string]*MenuConfig),
		startMenu: startMenu,
	}
}

// LoadMenu loads a menu configuration from a .MNU file
func (mm *MenuManager) LoadMenu(menuName string) (*MenuConfig, error) {
	// Check if already loaded
	if menu, exists := mm.menus[menuName]; exists {
		return menu, nil
	}

	// Load from file
	menuFile := filepath.Join(mm.menuPath, menuName+".MNU")
	menu, err := mm.parseMenuFile(menuFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load menu %s: %w", menuName, err)
	}

	// Cache the menu
	mm.menus[menuName] = menu
	return menu, nil
}

// GetStartMenu returns the configured starting menu
func (mm *MenuManager) GetStartMenu() string {
	return mm.startMenu
}

// GetANSIPath returns the full path to a menu's ANSI file
func (mm *MenuManager) GetANSIPath(ansiFile string) string {
	return filepath.Join(mm.ansiPath, ansiFile)
}

// parseMenuFile parses a classic BBS-style .MNU configuration file
func (mm *MenuManager) parseMenuFile(filePath string) (*MenuConfig, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open menu file: %w", err)
	}
	defer file.Close()

	menu := &MenuConfig{
		Position: PositionCenter, // Default position
		Commands: make([]MenuCommand, 0),
	}

	scanner := bufio.NewScanner(file)
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for section headers
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.ToUpper(strings.Trim(line, "[]"))
			continue
		}

		// Parse based on current section
		switch currentSection {
		case "MENU":
			if err := mm.parseMenuProperty(menu, line); err != nil {
				return nil, fmt.Errorf("error parsing menu property '%s': %w", line, err)
			}
		case "COMMANDS":
			if err := mm.parseMenuCommand(menu, line); err != nil {
				return nil, fmt.Errorf("error parsing command '%s': %w", line, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading menu file: %w", err)
	}

	// Validate required fields
	if menu.ANSIFile == "" {
		return nil, fmt.Errorf("menu file missing required ANSI property")
	}

	return menu, nil
}

// parseMenuProperty parses a menu property line (e.g., "Name=Main Menu")
func (mm *MenuManager) parseMenuProperty(menu *MenuConfig, line string) error {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid property format, expected KEY=VALUE")
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	switch strings.ToUpper(key) {
	case "NAME":
		menu.Name = value
	case "ANSI":
		menu.ANSIFile = value
	case "PROMPT":
		menu.Prompt = value
	case "POSITION":
		menu.Position = ParsePosition(value)
	case "OFFSET":
		offset, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid offset value: %w", err)
		}
		menu.Offset = offset
	case "THEME":
		menu.Theme = value
	case "PARENT":
		menu.Parent = value
	default:
		// Ignore unknown properties for forward compatibility
	}

	return nil
}

// parseMenuCommand parses a menu command line (e.g., "M    GOTO:MSGMENU     *    \"Message Menu\"")
func (mm *MenuManager) parseMenuCommand(menu *MenuConfig, line string) error {
	// Split the line into parts (keys, action, acs, description)
	// Expected format: KEYS    ACTION    ACS    "DESCRIPTION"
	
	// First, extract quoted description if present
	var description string
	if strings.Contains(line, "\"") {
		// Find the quoted description
		firstQuote := strings.Index(line, "\"")
		lastQuote := strings.LastIndex(line, "\"")
		if firstQuote != lastQuote && firstQuote >= 0 {
			description = line[firstQuote+1 : lastQuote]
			line = line[:firstQuote] // Remove description from line for further parsing
		}
	}

	// Split remaining line by whitespace
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 3 {
		return fmt.Errorf("command line must have at least KEYS ACTION ACS")
	}

	command := MenuCommand{
		Keys:        fields[0],
		Action:      fields[1],
		ACS:         fields[2],
		Description: description,
		Hidden:      false, // Default to visible
	}

	// Handle special cases for keys
	if strings.HasPrefix(command.Keys, "!") {
		command.Hidden = true
		command.Keys = strings.TrimPrefix(command.Keys, "!")
	}

	menu.Commands = append(menu.Commands, command)
	return nil
}

// FindCommand finds a command by key sequence
func (menu *MenuConfig) FindCommand(keys string) (*MenuCommand, bool) {
	upperKeys := strings.ToUpper(keys)
	
	for i := range menu.Commands {
		if strings.ToUpper(menu.Commands[i].Keys) == upperKeys {
			return &menu.Commands[i], true
		}
	}
	
	return nil, false
}

// GetVisibleCommands returns all non-hidden commands
func (menu *MenuConfig) GetVisibleCommands() []MenuCommand {
	var visible []MenuCommand
	for _, cmd := range menu.Commands {
		if !cmd.Hidden {
			visible = append(visible, cmd)
		}
	}
	return visible
}