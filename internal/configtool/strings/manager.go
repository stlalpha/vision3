package strings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/stlalpha/vision3/internal/config"
)

// StringCategory represents a logical grouping of strings
type StringCategory struct {
	Name        string
	Description string
	Fields      []StringField
}

// StringField represents a single configurable string
type StringField struct {
	Key         string
	DisplayName string
	Value       string
	Description string
	Category    string
	JSONTag     string
}

// StringManager handles all string configuration operations
type StringManager struct {
	configPath  string
	config      config.StringsConfig
	categories  []StringCategory
	fields      map[string]*StringField
	history     []HistoryEntry
	historyIdx  int
}

// HistoryEntry represents a single change in the undo/redo system
type HistoryEntry struct {
	Action    string // "modify", "batch_import", etc.
	Key       string
	OldValue  string
	NewValue  string
	Timestamp int64
}

// NewStringManager creates a new string manager instance
func NewStringManager(configPath string) (*StringManager, error) {
	sm := &StringManager{
		configPath: configPath,
		fields:     make(map[string]*StringField),
		history:    make([]HistoryEntry, 0),
		historyIdx: -1,
	}

	// Load existing configuration
	if err := sm.LoadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load string configuration: %w", err)
	}

	// Initialize categories and field mappings
	sm.initializeCategories()

	return sm, nil
}

// LoadConfig loads the strings.json configuration file
func (sm *StringManager) LoadConfig() error {
	loadedConfig, err := config.LoadStrings(sm.configPath)
	if err != nil {
		return err
	}
	sm.config = loadedConfig
	return nil
}

// SaveConfig saves the current configuration to strings.json
func (sm *StringManager) SaveConfig() error {
	filePath := filepath.Join(sm.configPath, "strings.json")
	
	data, err := json.MarshalIndent(sm.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// initializeCategories sets up the string categories and field mappings
func (sm *StringManager) initializeCategories() {
	// Use reflection to map struct fields to categories
	configValue := reflect.ValueOf(sm.config)
	configType := reflect.TypeOf(sm.config)

	sm.categories = []StringCategory{
		{
			Name:        "Login",
			Description: "Login and authentication prompts",
			Fields:      []StringField{},
		},
		{
			Name:        "Messages",
			Description: "Message system prompts and notifications",
			Fields:      []StringField{},
		},
		{
			Name:        "Files",
			Description: "File area and transfer prompts",
			Fields:      []StringField{},
		},
		{
			Name:        "User",
			Description: "User management and profile prompts",
			Fields:      []StringField{},
		},
		{
			Name:        "System",
			Description: "System messages and status displays",
			Fields:      []StringField{},
		},
		{
			Name:        "Mail",
			Description: "Mail and feedback system prompts",
			Fields:      []StringField{},
		},
		{
			Name:        "Chat",
			Description: "Chat and communication prompts",
			Fields:      []StringField{},
		},
		{
			Name:        "Prompts",
			Description: "General user interface prompts",
			Fields:      []StringField{},
		},
		{
			Name:        "Colors",
			Description: "Default color settings",
			Fields:      []StringField{},
		},
	}

	// Categorize fields by analyzing field names and content
	for i := 0; i < configType.NumField(); i++ {
		field := configType.Field(i)
		fieldValue := configValue.Field(i)
		
		if !fieldValue.CanInterface() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		category := sm.categorizeField(field.Name, jsonTag)
		displayName := sm.generateDisplayName(field.Name)
		description := sm.generateDescription(field.Name, jsonTag)

		stringField := StringField{
			Key:         field.Name,
			DisplayName: displayName,
			Value:       fmt.Sprintf("%v", fieldValue.Interface()),
			Description: description,
			Category:    category,
			JSONTag:     jsonTag,
		}

		// Add to category
		for j := range sm.categories {
			if sm.categories[j].Name == category {
				sm.categories[j].Fields = append(sm.categories[j].Fields, stringField)
				break
			}
		}

		// Add to field map for quick lookup
		sm.fields[field.Name] = &stringField
	}

	// Sort fields within each category
	for i := range sm.categories {
		sort.Slice(sm.categories[i].Fields, func(a, b int) bool {
			return sm.categories[i].Fields[a].DisplayName < sm.categories[i].Fields[b].DisplayName
		})
	}
}

// categorizeField determines which category a field belongs to
func (sm *StringManager) categorizeField(fieldName, jsonTag string) string {
	fieldLower := strings.ToLower(fieldName)
	tagLower := strings.ToLower(jsonTag)

	// Login related
	if strings.Contains(fieldLower, "login") || strings.Contains(fieldLower, "password") ||
		strings.Contains(fieldLower, "alias") || strings.Contains(fieldLower, "newuser") ||
		strings.Contains(tagLower, "password") || strings.Contains(tagLower, "login") {
		return "Login"
	}

	// Message related
	if strings.Contains(fieldLower, "msg") || strings.Contains(fieldLower, "message") ||
		strings.Contains(fieldLower, "quote") || strings.Contains(fieldLower, "post") ||
		strings.Contains(fieldLower, "scan") || strings.Contains(fieldLower, "board") ||
		strings.Contains(tagLower, "msg") || strings.Contains(tagLower, "message") {
		return "Messages"
	}

	// File related
	if strings.Contains(fieldLower, "file") || strings.Contains(fieldLower, "upload") ||
		strings.Contains(fieldLower, "download") || strings.Contains(fieldLower, "batch") ||
		strings.Contains(fieldLower, "area") && strings.Contains(fieldLower, "file") {
		return "Files"
	}

	// User related
	if strings.Contains(fieldLower, "user") || strings.Contains(fieldLower, "nup") ||
		strings.Contains(fieldLower, "vote") || strings.Contains(fieldLower, "validate") ||
		strings.Contains(tagLower, "user") || strings.Contains(tagLower, "nup") {
		return "User"
	}

	// Mail related
	if strings.Contains(fieldLower, "mail") || strings.Contains(fieldLower, "feedback") ||
		strings.Contains(fieldLower, "email") || strings.Contains(tagLower, "mail") {
		return "Mail"
	}

	// Chat related
	if strings.Contains(fieldLower, "chat") || strings.Contains(fieldLower, "sysop") ||
		strings.Contains(tagLower, "chat") {
		return "Chat"
	}

	// System related
	if strings.Contains(fieldLower, "system") || strings.Contains(fieldLower, "header") ||
		strings.Contains(fieldLower, "pause") || strings.Contains(fieldLower, "working") ||
		strings.Contains(fieldLower, "dos") {
		return "System"
	}

	// Colors
	if strings.Contains(fieldLower, "color") || strings.HasPrefix(fieldLower, "def") {
		return "Colors"
	}

	// Default to Prompts
	return "Prompts"
}

// generateDisplayName creates a human-readable name from the field name
func (sm *StringManager) generateDisplayName(fieldName string) string {
	// Convert camelCase to space-separated words
	var result strings.Builder
	for i, r := range fieldName {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune(' ')
		}
		if i == 0 {
			result.WriteRune(r)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// generateDescription creates a description for the field
func (sm *StringManager) generateDescription(fieldName, jsonTag string) string {
	// Basic descriptions based on field patterns
	fieldLower := strings.ToLower(fieldName)
	
	if strings.Contains(fieldLower, "prompt") {
		return "User interface prompt text"
	}
	if strings.Contains(fieldLower, "str") && strings.Contains(fieldLower, "prompt") {
		return "Display text for user interaction"
	}
	if strings.Contains(fieldLower, "header") {
		return "Header or title display text"
	}
	if strings.Contains(fieldLower, "color") {
		return "Color code setting (0-255)"
	}
	
	return "Configurable text string"
}

// GetCategories returns all string categories
func (sm *StringManager) GetCategories() []StringCategory {
	return sm.categories
}

// GetCategoryByName returns a specific category by name
func (sm *StringManager) GetCategoryByName(name string) *StringCategory {
	for i := range sm.categories {
		if sm.categories[i].Name == name {
			return &sm.categories[i]
		}
	}
	return nil
}

// GetFieldByKey returns a field by its key
func (sm *StringManager) GetFieldByKey(key string) *StringField {
	return sm.fields[key]
}

// SearchFields searches for fields matching the query
func (sm *StringManager) SearchFields(query string) []StringField {
	var results []StringField
	queryLower := strings.ToLower(query)

	for _, field := range sm.fields {
		if strings.Contains(strings.ToLower(field.DisplayName), queryLower) ||
			strings.Contains(strings.ToLower(field.Key), queryLower) ||
			strings.Contains(strings.ToLower(field.Value), queryLower) ||
			strings.Contains(strings.ToLower(field.Description), queryLower) {
			results = append(results, *field)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].DisplayName < results[j].DisplayName
	})

	return results
}

// UpdateField updates a field value and records it in history
func (sm *StringManager) UpdateField(key, newValue string) error {
	field := sm.fields[key]
	if field == nil {
		return fmt.Errorf("field not found: %s", key)
	}

	oldValue := field.Value

	// Record in history
	sm.addToHistory("modify", key, oldValue, newValue)

	// Update the field
	field.Value = newValue

	// Update the underlying config struct using reflection
	if err := sm.updateConfigField(key, newValue); err != nil {
		return err
	}

	// Update the field in the category as well
	for i := range sm.categories {
		for j := range sm.categories[i].Fields {
			if sm.categories[i].Fields[j].Key == key {
				sm.categories[i].Fields[j].Value = newValue
				break
			}
		}
	}

	return nil
}

// updateConfigField updates the underlying config struct
func (sm *StringManager) updateConfigField(key, value string) error {
	configValue := reflect.ValueOf(&sm.config).Elem()
	field := configValue.FieldByName(key)
	
	if !field.IsValid() {
		return fmt.Errorf("field not found in config: %s", key)
	}

	if !field.CanSet() {
		return fmt.Errorf("cannot set field: %s", key)
	}

	// Handle different field types
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Uint8:
		// For color fields
		var colorVal uint8
		if _, err := fmt.Sscanf(value, "%d", &colorVal); err != nil {
			return fmt.Errorf("invalid color value: %s", value)
		}
		field.SetUint(uint64(colorVal))
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	return nil
}

// addToHistory adds an entry to the undo/redo history
func (sm *StringManager) addToHistory(action, key, oldValue, newValue string) {
	// Truncate history if we're not at the end
	if sm.historyIdx < len(sm.history)-1 {
		sm.history = sm.history[:sm.historyIdx+1]
	}

	entry := HistoryEntry{
		Action:    action,
		Key:       key,
		OldValue:  oldValue,
		NewValue:  newValue,
		Timestamp: 0, // Could add actual timestamp
	}

	sm.history = append(sm.history, entry)
	sm.historyIdx = len(sm.history) - 1

	// Limit history size
	if len(sm.history) > 100 {
		sm.history = sm.history[1:]
		sm.historyIdx--
	}
}

// CanUndo returns whether undo is possible
func (sm *StringManager) CanUndo() bool {
	return sm.historyIdx >= 0
}

// CanRedo returns whether redo is possible
func (sm *StringManager) CanRedo() bool {
	return sm.historyIdx < len(sm.history)-1
}

// Undo undoes the last change
func (sm *StringManager) Undo() error {
	if !sm.CanUndo() {
		return fmt.Errorf("nothing to undo")
	}

	entry := sm.history[sm.historyIdx]
	sm.historyIdx--

	// Restore the old value
	return sm.UpdateField(entry.Key, entry.OldValue)
}

// Redo redoes the next change
func (sm *StringManager) Redo() error {
	if !sm.CanRedo() {
		return fmt.Errorf("nothing to redo")
	}

	sm.historyIdx++
	entry := sm.history[sm.historyIdx]

	// Apply the new value
	return sm.UpdateField(entry.Key, entry.NewValue)
}

// ExportToJSON exports all strings to a JSON format
func (sm *StringManager) ExportToJSON() ([]byte, error) {
	return json.MarshalIndent(sm.config, "", "  ")
}

// ImportFromJSON imports strings from JSON data
func (sm *StringManager) ImportFromJSON(data []byte) error {
	var importedConfig config.StringsConfig
	if err := json.Unmarshal(data, &importedConfig); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Record batch import in history
	sm.addToHistory("batch_import", "", "", "")

	// Update config and reinitialize
	sm.config = importedConfig
	sm.initializeCategories()

	return nil
}

// ValidateString checks if a string is valid (basic validation)
func (sm *StringManager) ValidateString(value string) error {
	// Basic validation - check for invalid characters, length, etc.
	if len(value) > 1000 {
		return fmt.Errorf("string too long (max 1000 characters)")
	}

	// Could add more validation rules here
	return nil
}

// GetFieldCount returns the total number of configurable fields
func (sm *StringManager) GetFieldCount() int {
	return len(sm.fields)
}

// GetCategoryCount returns the number of categories
func (sm *StringManager) GetCategoryCount() int {
	return len(sm.categories)
}