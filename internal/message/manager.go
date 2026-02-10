package message

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const (
	messageAreaFile = "message_areas.json"
	// Add constants for message/mail directories later
)

// MessageManager handles loading, saving, and accessing message areas and messages.
type MessageManager struct {
	mu        sync.RWMutex
	dataPath  string // Base path for message data files (e.g., "data")
	areasPath string // Full path to message_areas.json (in configs/)
	// In-memory storage
	areasByID  map[int]*MessageArea
	areasByTag map[string]*MessageArea
}

// NewMessageManager creates and initializes a new MessageManager.
// configPath is the directory containing message_areas.json (e.g., "configs").
// dataPath is the directory for message JSONL files (e.g., "data").
func NewMessageManager(dataPath, configPath string) (*MessageManager, error) {
	mm := &MessageManager{
		dataPath:   dataPath,
		areasPath:  filepath.Join(configPath, messageAreaFile),
		areasByID:  make(map[int]*MessageArea),
		areasByTag: make(map[string]*MessageArea),
	}

	if err := mm.loadMessageAreas(); err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: %s not found. Starting with no message areas.", messageAreaFile)
			// Optionally create default areas here if desired
			// if createErr := mm.createDefaultAreas(); createErr != nil {
			// 	 return nil, fmt.Errorf("failed to create default message areas: %w", createErr)
			// }
		} else {
			// Other error loading file
			return nil, fmt.Errorf("failed to load message areas: %w", err)
		}
	}

	log.Printf("INFO: MessageManager initialized. Loaded %d areas.", len(mm.areasByID))
	return mm, nil
}

// loadMessageAreas loads the message area definitions from JSON.
func (mm *MessageManager) loadMessageAreas() error {
	data, err := os.ReadFile(mm.areasPath)
	if err != nil {
		return err // Return error to NewMessageManager (handles os.IsNotExist)
	}

	if len(data) == 0 {
		log.Printf("INFO: %s is empty. No message areas loaded.", mm.areasPath)
		return nil // Empty file is not an error
	}

	var areasList []*MessageArea
	if err := json.Unmarshal(data, &areasList); err != nil {
		return fmt.Errorf("failed to unmarshal message areas array from %s: %w", mm.areasPath, err)
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Clear existing maps before loading
	mm.areasByID = make(map[int]*MessageArea)
	mm.areasByTag = make(map[string]*MessageArea)

	// Populate maps
	for _, area := range areasList {
		if area == nil {
			continue // Skip nil entries
		}
		if _, exists := mm.areasByID[area.ID]; exists {
			log.Printf("WARN: Duplicate message area ID %d found in %s. Skipping subsequent entry.", area.ID, mm.areasPath)
			continue
		}
		if _, exists := mm.areasByTag[area.Tag]; exists {
			log.Printf("WARN: Duplicate message area Tag '%s' found in %s. Skipping subsequent entry.", area.Tag, mm.areasPath)
			continue
		}
		mm.areasByID[area.ID] = area
		mm.areasByTag[area.Tag] = area
		log.Printf("TRACE: Loaded message area ID %d, Tag '%s', Name '%s'", area.ID, area.Tag, area.Name)
	}

	return nil
}

// saveMessageAreas writes the current message areas back to the JSON file.
func (mm *MessageManager) saveMessageAreas() error {
	mm.mu.RLock() // Use RLock initially to read data for marshalling
	// Convert map to slice for saving
	areasList := make([]*MessageArea, 0, len(mm.areasByID))
	for _, area := range mm.areasByID {
		areasList = append(areasList, area)
	}
	// Sort by ID for consistent output (optional but good practice)
	sort.Slice(areasList, func(i, j int) bool {
		return areasList[i].ID < areasList[j].ID
	})
	mm.mu.RUnlock() // Unlock RLock

	data, err := json.MarshalIndent(areasList, "", "  ") // Use MarshalIndent for readability
	if err != nil {
		return fmt.Errorf("failed to marshal message areas: %w", err)
	}

	// Write with a write lock (though technically only needed if other writers exist)
	// Using WriteFile is generally safer as it handles permissions and atomic writes better
	mm.mu.Lock() // Acquire write lock for file operation
	defer mm.mu.Unlock()
	if err := os.WriteFile(mm.areasPath, data, 0644); err != nil { // 0644: User read/write, Group/Other read
		return fmt.Errorf("failed to write message areas to %s: %w", mm.areasPath, err)
	}

	log.Printf("INFO: Saved %d message areas to %s", len(areasList), mm.areasPath)
	return nil
}

// GetAreaByID retrieves a message area by its ID.
// Returns the area and true if found, otherwise nil and false.
func (mm *MessageManager) GetAreaByID(id int) (*MessageArea, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	area, exists := mm.areasByID[id]
	return area, exists
}

// GetAreaByTag retrieves a message area by its Tag.
// Returns the area and true if found, otherwise nil and false.
func (mm *MessageManager) GetAreaByTag(tag string) (*MessageArea, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	area, exists := mm.areasByTag[tag]
	return area, exists
}

// ListAreas returns a sorted slice of all loaded message areas.
// TODO: Implement ACS filtering based on the current user.
func (mm *MessageManager) ListAreas() []*MessageArea {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// Create a slice with capacity
	areasList := make([]*MessageArea, 0, len(mm.areasByID))

	// Populate the slice from the map
	for _, area := range mm.areasByID {
		areasList = append(areasList, area)
	}

	// Sort the slice by Area ID for consistent order
	sort.Slice(areasList, func(i, j int) bool {
		return areasList[i].ID < areasList[j].ID
	})

	return areasList
}

// AddMessage appends a new message to the appropriate area's JSONL file.
func (mm *MessageManager) AddMessage(areaID int, msg Message) error {
	mm.mu.Lock() // Ensure exclusive write access
	defer mm.mu.Unlock()

	// Construct the filename (e.g., data/messages_area_1.jsonl)
	filename := fmt.Sprintf("messages_area_%d.jsonl", areaID)
	filePath := filepath.Join(mm.dataPath, filename)

	// Marshal the single message to JSON
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message %s for area %d: %w", msg.ID, areaID, err)
	}

	// Open the file in append mode, create if it doesn't exist
	// Use 0644 permissions: User read/write, Group/Other read
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open message file %s for area %d: %w", filePath, areaID, err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			log.Printf("WARN: Failed to close message file %s: %v", filePath, cerr)
		}
	}()

	// Append the JSON data followed by a newline
	if _, err := file.Write(append(jsonData, '\n')); err != nil {
		return fmt.Errorf("failed to write message %s to file %s: %w", msg.ID, filePath, err)
	}

	log.Printf("TRACE: Appended message %s to %s", msg.ID, filePath)
	return nil
}

// GetMessagesForArea reads messages from the JSONL file for a specific area ID.
// If sinceMessageID is provided, it only returns messages *after* that specific message.
// It returns a slice of messages or an error if reading fails.
func (mm *MessageManager) GetMessagesForArea(areaID int, sinceMessageID string) ([]Message, error) {
	mm.mu.RLock() // Lock for reading path and potentially area info
	defer mm.mu.RUnlock()

	// Construct the filename (e.g., data/messages_area_1.jsonl)
	filename := fmt.Sprintf("messages_area_%d.jsonl", areaID)
	filePath := filepath.Join(mm.dataPath, filename)

	log.Printf("DEBUG: Attempting to read messages from file: %s", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: Message file %s does not exist for area ID %d. Returning empty list.", filePath, areaID)
			return []Message{}, nil // No messages if file doesn't exist
		}
		return nil, fmt.Errorf("failed to open message file %s: %w", filePath, err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			log.Printf("WARN: Failed to close message file %s: %v", filePath, cerr)
		}
	}()

	var messages []Message
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue // Skip empty lines
		}

		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("WARN: Failed to unmarshal message on line %d in %s: %v. Skipping line.", lineNumber, filePath, err)
			continue // Skip lines that fail to parse
		}
		messages = append(messages, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading message file %s: %w", filePath, err)
	}

	log.Printf("DEBUG: Successfully read %d messages from %s", len(messages), filePath)

	// --- BEGIN New Scan Filtering ---
	if sinceMessageID != "" {
		foundIndex := -1
		for i, msg := range messages {
			if msg.ID.String() == sinceMessageID {
				foundIndex = i
				break
			}
		}

		if foundIndex != -1 {
			// Return messages *after* the found index
			log.Printf("DEBUG: Found last read message at index %d. Returning subsequent %d messages.", foundIndex, len(messages)-(foundIndex+1))
			if foundIndex+1 < len(messages) {
				return messages[foundIndex+1:], nil
			} else {
				// Last read was the last message in the list
				return []Message{}, nil
			}
		} else {
			// Last read message ID not found in the current list (maybe deleted?)
			log.Printf("WARN: sinceMessageID '%s' not found in area %d message list. Returning all messages.", sinceMessageID, areaID)
			// Fallthrough to return all messages
		}
	}
	// --- END New Scan Filtering ---

	// Return all messages if sinceMessageID was empty or not found
	return messages, nil
}

// GetMessageCountForArea efficiently counts the number of messages in a given area's file.
func (mm *MessageManager) GetMessageCountForArea(areaID int) (int, error) {
	mm.mu.RLock() // Lock for reading path
	defer mm.mu.RUnlock()

	filename := fmt.Sprintf("messages_area_%d.jsonl", areaID)
	filePath := filepath.Join(mm.dataPath, filename)

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // 0 messages if file doesn't exist
		}
		return 0, fmt.Errorf("failed to open message file %s for count: %w", filePath, err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			log.Printf("WARN: Failed to close message file %s during count: %v", filePath, cerr)
		}
	}()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Count non-empty lines, assuming each represents a message
		if len(bytes.TrimSpace(scanner.Bytes())) > 0 {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error scanning message file %s for count: %w", filePath, err)
	}

	log.Printf("DEBUG: Counted %d messages in %s", count, filePath)
	return count, nil
}

// GetNewMessageCount checks how many messages exist in the area after the given message ID.
func (mm *MessageManager) GetNewMessageCount(areaID int, sinceMessageID string) (int, error) {
	// Reuse GetMessagesForArea logic which includes the filtering.
	// Optimization note: This loads all message bodies into memory just to count them.
	// A more efficient approach might scan the file line-by-line and stop after finding the sinceMessageID,
	// but this requires more complex file handling.
	newMessages, err := mm.GetMessagesForArea(areaID, sinceMessageID)
	if err != nil {
		// Propagate error from GetMessagesForArea
		return 0, err
	}

	count := len(newMessages)
	log.Printf("DEBUG: GetNewMessageCount check for area %d since '%s': %d new messages", areaID, sinceMessageID, count)
	return count, nil
}

// --- Add AddArea, DeleteArea etc. later ---
