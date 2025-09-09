package menu

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log"
	// "vision3/config" // TODO: Get MenuDir from config
)

// TODO: Replace this with path from config
const menuDir = "menus"

// LoadMenu reads a .MNU file in classic BBS text format for the given menu name.
func LoadMenu(menuName string, configPath string) (*MenuRecord, error) {
	filePath := filepath.Join(configPath, menuName+".MNU")
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("menu file not found: %s", filePath)
		}
		return nil, fmt.Errorf("failed to read menu file %s: %w", filePath, err)
	}
	defer file.Close()

	// Parse the classic BBS .MNU format
	menuRec := MenuRecord{
		ClrScrBefore: true,  // Default to clearing screen
		UsePrompt:    true,  // Default to using prompts
		ACS:          "*",   // Default to allow all
	}
	
	scanner := bufio.NewScanner(file)
	inMenuSection := false
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		
		// Check for section headers
		if line == "[MENU]" {
			inMenuSection = true
			continue
		} else if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inMenuSection = false
			continue
		}
		
		// Parse menu properties when in MENU section
		if inMenuSection {
			if strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					
					switch strings.ToUpper(key) {
					case "ANSI":
						menuRec.ANSIFile = value
					case "PROMPT":
						// Prompt template name - already defaults to using prompts
					case "POSITION":
						// Screen position - handled by screen manager
					}
				}
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading MNU file %s: %w", filePath, err)
	}

	return &menuRec, nil
}

// LoadCommandsFromMNU reads commands from a .MNU file - no fallbacks
func LoadCommandsFromMNU(menuName string, menuPath string, configPath string) ([]CommandRecord, error) {
	mnuFilePath := filepath.Join(menuPath, menuName+".MNU")
	log.Printf("DEBUG: LoadCommandsFromMNU loading from .MNU file: %s", mnuFilePath)
	return loadCommandsFromMNUFile(mnuFilePath)
}

// loadCommandsFromMNUFile parses a .MNU file and extracts commands
func loadCommandsFromMNUFile(filePath string) ([]CommandRecord, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open MNU file %s: %w", filePath, err)
	}
	defer file.Close()

	var commands []CommandRecord
	scanner := bufio.NewScanner(file)
	inCommandsSection := false
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		
		// Check for section headers
		if line == "[COMMANDS]" {
			inCommandsSection = true
			continue
		} else if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inCommandsSection = false
			continue
		}
		
		// Parse command lines when in COMMANDS section
		if inCommandsSection {
			cmd, err := parseCommandLine(line)
			if err != nil {
				log.Printf("WARN: Failed to parse command line '%s': %v", line, err)
				continue
			}
			if cmd != nil {
				commands = append(commands, *cmd)
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading MNU file %s: %w", filePath, err)
	}
	
	// Log loaded commands
	for _, cmd := range commands {
		log.Printf("TRACE: Loaded command from MNU: Keys='%s', Cmd='%s', ACS='%s', Hidden=%t", cmd.Keys, cmd.Command, cmd.ACS, cmd.Hidden)
	}
	
	return commands, nil
}

// parseCommandLine parses a single command line from .MNU file
// Format: KEYS    ACTION           ACS    "DESCRIPTION"
func parseCommandLine(line string) (*CommandRecord, error) {
	// Split by whitespace but preserve quoted strings
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return nil, fmt.Errorf("insufficient fields in command line")
	}
	
	keys := parts[0]
	action := parts[1]
	acs := parts[2]
	
	// Extract description if present (in quotes) - for future use
	_ = ""
	if len(parts) > 3 {
		// Find the quoted description
		quotedPart := strings.Join(parts[3:], " ")
		if strings.Contains(quotedPart, "\"") {
			start := strings.Index(quotedPart, "\"")
			end := strings.LastIndex(quotedPart, "\"")
			if start != -1 && end != -1 && start != end {
				_ = quotedPart[start+1:end] // Description for future use
			}
		}
	}
	
	return &CommandRecord{
		Keys:    keys,
		Command: action,
		ACS:     acs,
		Hidden:  false, // Default to visible
	}, nil
}

// LoadCommands reads a .CFG file (assumed JSON) for the given menu name.
func LoadCommands(menuName string, configPath string) ([]CommandRecord, error) {
	filePath := filepath.Join(configPath, menuName+".CFG")
	log.Printf("DEBUG: Attempting to load command file: %s (menuName='%s', configPath='%s')", filePath, menuName, configPath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// It's valid for a menu to have no commands, return empty slice
			log.Printf("WARN: Command file %s does not exist, menu will have no commands.", filePath)
			return []CommandRecord{}, nil
		}
		return nil, fmt.Errorf("failed to read command file %s: %w", filePath, err)
	}

	// Handle empty file case explicitly
	if len(data) == 0 {
		log.Printf("DEBUG: Command file %s is empty.", filePath)
		return []CommandRecord{}, nil
	}

	var commands []CommandRecord
	// Unmarshal the JSON data (expecting an array of CommandRecord)
	if err := json.Unmarshal(data, &commands); err != nil {
		// Provide more context in the error message
		return nil, fmt.Errorf("failed to decode command file %s (is this a valid JSON array of commands?): %w", filePath, err)
	}

	// Optional: Log loaded commands for tracing
	for _, cmd := range commands {
		log.Printf("TRACE: Loaded command: Keys='%s', Cmd='%s', ACS='%s', Hidden=%t", cmd.Keys, cmd.Command, cmd.ACS, cmd.Hidden)
	}

	return commands, nil
}

// NOTE: Removed duplicate struct definitions here. They should exist elsewhere.
/*
type MenuRecord struct {
	ClrScrBefore bool   `json:"CLR"`    // Clear screen before display
	UsePrompt    bool   `json:"PROMPT"` // Use custom prompt? (Seems false for LOGIN)
	Prompt1      string `json:"PROMPT1"`
	Prompt2      string `json:"PROMPT2"` // (Ignoring PROMPT2/3 for now)
	Fallback     string `json:"FALLBACK"` // Menu to go to on no match
	ACS          string `json:"ACS"`
	Password     string `json:"PASS"` // Added PASS field
	ANSIFile     string // ANSI file to display (from .MNU ANSI= line)
}

type CommandRecord struct {
	Keys    string `json:"KEYS"`
	Command string `json:"CMD"`
	ACS     string `json:"ACS"`
	Hidden  bool   `json:"HIDDEN"`
}
*/
