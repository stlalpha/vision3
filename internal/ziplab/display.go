package ziplab

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// Status represents the D/P/F state of a ZipLab step.
type Status string

const (
	StatusDoing Status = "D"
	StatusPass  Status = "P"
	StatusFail  Status = "F"
)

// NFOEntry holds the parsed display coordinates and colors for one step+status.
type NFOEntry struct {
	Step         int
	Status       Status
	Col          int    // X position (1-based)
	Row          int    // Y position (1-based)
	NormalColor  int    // DOS color attribute (bg*16 + fg)
	HiColor      int    // DOS highlight color attribute
	DisplayChars string // Characters to display (e.g., "███")
}

// NFOConfig holds all parsed entries from ZIPLAB.NFO.
type NFOConfig struct {
	Entries map[string]NFOEntry // key = "1D", "1P", "1F", etc.
}

// entryKey builds the map key for a step+status combination.
func entryKey(step int, status Status) string {
	return fmt.Sprintf("%d%s", step, status)
}

// GetEntry returns the NFO entry for a given step and status.
func (n *NFOConfig) GetEntry(step int, status Status) (NFOEntry, bool) {
	entry, ok := n.Entries[entryKey(step, status)]
	return entry, ok
}

// HasStep returns true if the NFO has any entries for the given step number.
func (n *NFOConfig) HasStep(step int) bool {
	_, ok := n.Entries[entryKey(step, StatusDoing)]
	return ok
}

// BuildStatusSequence generates an ANSI escape sequence that positions the cursor
// and renders the status indicator for a given step+status.
func (n *NFOConfig) BuildStatusSequence(step int, status Status) string {
	entry, ok := n.GetEntry(step, status)
	if !ok {
		return ""
	}

	// Choose color based on status
	dosColor := entry.NormalColor
	if status == StatusPass || status == StatusFail {
		dosColor = entry.HiColor
	}

	fg, bg, bold := DOSColorToANSI(dosColor)

	var sb strings.Builder
	// Cursor position
	sb.WriteString(fmt.Sprintf("\x1b[%d;%dH", entry.Row, entry.Col))
	// Color
	if bold {
		sb.WriteString(fmt.Sprintf("\x1b[1;%d;%dm", 30+(fg%8), 40+bg))
	} else {
		sb.WriteString(fmt.Sprintf("\x1b[0;%d;%dm", 30+fg, 40+bg))
	}
	// Display chars
	sb.WriteString(entry.DisplayChars)
	// Reset
	sb.WriteString("\x1b[0m")

	return sb.String()
}

// ParseNFO reads and parses a ZIPLAB.NFO file.
// Format: {step}{status} = X,Y,NormalColor,HiColor,DisplayChars
func ParseNFO(filePath string) (*NFOConfig, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open NFO file %s: %w", filePath, err)
	}
	defer f.Close()

	nfo := &NFOConfig{
		Entries: make(map[string]NFOEntry),
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// Parse: "1D = 45,10,112,116,███"
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			log.Printf("WARN: NFO line %d: invalid format (no '='): %s", lineNum, line)
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if len(key) < 2 {
			log.Printf("WARN: NFO line %d: key too short: %s", lineNum, key)
			continue
		}

		// Parse key: digit(s) + status letter
		statusChar := key[len(key)-1:]
		stepStr := key[:len(key)-1]

		step, err := strconv.Atoi(stepStr)
		if err != nil {
			log.Printf("WARN: NFO line %d: invalid step number: %s", lineNum, stepStr)
			continue
		}

		var status Status
		switch strings.ToUpper(statusChar) {
		case "D":
			status = StatusDoing
		case "P":
			status = StatusPass
		case "F":
			status = StatusFail
		default:
			log.Printf("WARN: NFO line %d: unknown status '%s'", lineNum, statusChar)
			continue
		}

		// Parse value: X,Y,NormalColor,HiColor,DisplayChars
		valueParts := strings.SplitN(value, ",", 5)
		if len(valueParts) < 5 {
			log.Printf("WARN: NFO line %d: expected 5 comma-separated values, got %d", lineNum, len(valueParts))
			continue
		}

		col, err := strconv.Atoi(strings.TrimSpace(valueParts[0]))
		if err != nil {
			log.Printf("WARN: NFO line %d: invalid X coordinate: %s", lineNum, valueParts[0])
			continue
		}
		row, err := strconv.Atoi(strings.TrimSpace(valueParts[1]))
		if err != nil {
			log.Printf("WARN: NFO line %d: invalid Y coordinate: %s", lineNum, valueParts[1])
			continue
		}
		normalColor, err := strconv.Atoi(strings.TrimSpace(valueParts[2]))
		if err != nil {
			log.Printf("WARN: NFO line %d: invalid normal color: %s", lineNum, valueParts[2])
			continue
		}
		hiColor, err := strconv.Atoi(strings.TrimSpace(valueParts[3]))
		if err != nil {
			log.Printf("WARN: NFO line %d: invalid hi color: %s", lineNum, valueParts[3])
			continue
		}
		displayChars := strings.TrimSpace(valueParts[4])

		nfo.Entries[entryKey(step, status)] = NFOEntry{
			Step:         step,
			Status:       status,
			Col:          col,
			Row:          row,
			NormalColor:  normalColor,
			HiColor:      hiColor,
			DisplayChars: displayChars,
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading NFO file %s: %w", filePath, err)
	}

	log.Printf("DEBUG: Parsed %d NFO entries from %s", len(nfo.Entries), filePath)
	return nfo, nil
}

// DOSColorToANSI converts a DOS color attribute (bg*16 + fg) to ANSI components.
// Returns foreground color (0-15), background color (0-7), and whether bold is needed.
func DOSColorToANSI(dosAttr int) (fg int, bg int, bold bool) {
	fg = dosAttr & 0x0F   // Lower 4 bits = foreground (0-15)
	bg = (dosAttr >> 4) & 0x07 // Bits 4-6 = background (0-7)
	bold = fg >= 8         // High bit of foreground = bold/bright
	return fg, bg, bold
}
