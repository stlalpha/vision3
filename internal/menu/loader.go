// Package menu provides menu system functionality for ViSiON/3 BBS.
//
// loader.go contains functions for loading menu definitions, commands,
// lightbar configurations, and related ANSI file processing helpers.
package menu

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// LightbarOption represents a single option in a lightbar menu
type LightbarOption struct {
	X, Y           int    // Screen coordinates
	Text           string // Display text
	HotKey         string // Command hotkey
	HighlightColor int    // Color code when highlighted
	RegularColor   int    // Color code when not highlighted
}

// ANSI foreground color codes (standard and bright)
var ansiFg = map[int]int{
	0: 30, 1: 34, 2: 32, 3: 36, 4: 31, 5: 35, 6: 33, 7: 37, // Standard
	8: 90, 9: 94, 10: 92, 11: 96, 12: 91, 13: 95, 14: 93, 15: 97, // Bright
}

// ANSI background color codes (standard)
var ansiBg = map[int]int{
	0: 40, 1: 44, 2: 42, 3: 46, 4: 41, 5: 45, 6: 43, 7: 47,
}

// colorCodeToAnsi converts a DOS-style color code (0-255) to ANSI escape sequence.
// Assumes Color = Background*16 + Foreground
func colorCodeToAnsi(code int) string {
	fgCode := code % 16
	bgCode := code / 16

	fgAnsi, okFg := ansiFg[fgCode]
	if !okFg {
		fgAnsi = 97 // Default to bright white if invalid fg code
	}

	// Use standard background colors (40-47). Bright backgrounds (100-107) have less support.
	bgAnsi, okBg := ansiBg[bgCode%8]
	if !okBg {
		bgAnsi = 40 // Default to black background if invalid bg code
	}

	return fmt.Sprintf("\x1b[%d;%dm", fgAnsi, bgAnsi)
}

// loadLightbarOptions loads and parses lightbar options from configuration files
func loadLightbarOptions(menuName string, e *MenuExecutor) ([]LightbarOption, error) {
	// Determine paths using MenuSetPath
	cfgFilename := menuName + ".CFG"
	barFilename := menuName + ".BAR"
	cfgPath := filepath.Join(e.MenuSetPath, "cfg", cfgFilename)
	barPath := filepath.Join(e.MenuSetPath, "bar", barFilename)

	log.Printf("DEBUG: Loading CFG: %s", cfgPath)
	log.Printf("DEBUG: Loading BAR: %s", barPath)

	// Try to load commands from CFG file
	commandsByHotkey := make(map[string]string)
	cfgFile, err := os.Open(cfgPath)
	if err != nil {
		log.Printf("WARN: Failed to load CFG file %s: %v", cfgPath, err)
	} else {
		defer cfgFile.Close()
		scanner := bufio.NewScanner(cfgFile)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ";") {
				continue // Skip empty lines and comments
			}

			parts := strings.SplitN(line, " ", 2)
			if len(parts) != 2 {
				continue // Skip malformed lines
			}

			hotkey := strings.ToUpper(strings.TrimSpace(parts[0]))
			command := strings.TrimSpace(parts[1])
			commandsByHotkey[hotkey] = command
		}
	}

	// Parse BAR file
	barFile, err := os.Open(barPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open BAR file %s: %w", barPath, err)
	}
	defer barFile.Close()

	var options []LightbarOption
	scanner := bufio.NewScanner(barFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue // Skip empty lines and comments
		}

		// Parse record in format: X,Y,HiLitedColor,RegularColor,HotKey,ReturnValue,HiLitedString
		parts := strings.SplitN(line, ",", 7) // Split into 7 parts
		if len(parts) != 7 {                  // Check for 7 parts
			log.Printf("WARN: Malformed BAR line (expected 7 fields): %s", line)
			continue
		}

		x, xerr := strconv.Atoi(strings.TrimSpace(parts[0]))
		y, yerr := strconv.Atoi(strings.TrimSpace(parts[1]))
		if xerr != nil || yerr != nil {
			log.Printf("WARN: Invalid coordinates in BAR line: %s", line)
			continue
		}

		// Parse color codes
		highlightColor, hcErr := strconv.Atoi(strings.TrimSpace(parts[2]))
		regularColor, rcErr := strconv.Atoi(strings.TrimSpace(parts[3]))
		if hcErr != nil || rcErr != nil {
			log.Printf("WARN: Invalid color codes in BAR line: %s", line)
			// Default colors
			highlightColor = 7 // Default: White on Black (inverse)
			regularColor = 15  // Default: Bright White on Black
		}

		hotkey := strings.ToUpper(strings.TrimSpace(parts[4])) // HotKey is the 5th field (index 4)
		// Field 5 is ReturnValue - ignore for now
		displayText := strings.TrimSpace(parts[6]) // DisplayText is the 7th field (index 6)

		// Verify the hotkey maps to a command
		if _, exists := commandsByHotkey[hotkey]; !exists {
			log.Printf("WARN: Hotkey '%s' in BAR file has no matching command in CFG", hotkey)
		}

		options = append(options, LightbarOption{
			X:              x,
			Y:              y,
			Text:           displayText,
			HotKey:         hotkey,
			HighlightColor: highlightColor,
			RegularColor:   regularColor,
		})
	}

	return options, nil
}

// calculateANSIOffset analyzes ANSI content to determine coordinate offset
// caused by leading blank lines, reset sequences, etc.
func calculateANSIOffset(ansiContent []byte) int {
	content := string(ansiContent)
	lines := strings.Split(content, "\n")

	offset := 0
	for _, line := range lines {
		// Remove ANSI escape sequences to check if line has actual content
		cleanLine := removeANSIEscapes(line)
		cleanLine = strings.TrimSpace(cleanLine)

		// If line is empty or only whitespace after removing ANSI codes, it's an offset
		if cleanLine == "" {
			offset++
		} else {
			// Found first line with actual content, stop counting
			break
		}
	}

	return offset
}

// removeANSIEscapes removes ANSI escape sequences from a string
func removeANSIEscapes(s string) string {
	// Simple ANSI escape sequence removal
	// Matches \x1b[...m patterns (color codes, etc.)
	result := ""
	i := 0
	for i < len(s) {
		if i < len(s)-1 && s[i] == '\x1b' && s[i+1] == '[' {
			// Find end of escape sequence (letter after digits/semicolons)
			j := i + 2
			for j < len(s) {
				c := s[j]
				if (c >= '0' && c <= '9') || c == ';' || c == '?' {
					j++
				} else {
					// Found the command letter, skip it too
					j++
					break
				}
			}
			i = j
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}
