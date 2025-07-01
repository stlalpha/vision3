package menu

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"log"
	// "vision3/config" // TODO: Get MenuDir from config
)

// TODO: Replace this with path from config
const menuDir = "menus"

// LoadMenu reads a .MNU file (assumed JSON) for the given menu name.
func LoadMenu(menuName string, configPath string) (*MenuRecord, error) {
	filePath := filepath.Join(configPath, menuName+".MNU")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("menu file not found: %s", filePath)
		}
		return nil, fmt.Errorf("failed to read menu file %s: %w", filePath, err)
	}

	var menuRec MenuRecord
	// Unmarshal the JSON data
	if err := json.Unmarshal(data, &menuRec); err != nil {
		// Provide more context in the error message
		return nil, fmt.Errorf("failed to decode menu file %s (is this a valid JSON .MNU file?): %w", filePath, err)
	}

	// Optional: Add validation if needed

	return &menuRec, nil
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
}

type CommandRecord struct {
	Keys    string `json:"KEYS"`
	Command string `json:"CMD"`
	ACS     string `json:"ACS"`
	Hidden  bool   `json:"HIDDEN"`
}
*/
