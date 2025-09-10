package doors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"sort"
	"errors"
	"strings"
)

// DoorManagerImpl implements the DoorManager interface
type DoorManagerImpl struct {
	doors           map[string]*DoorConfiguration
	templates       map[string]*DoorTemplate
	instances       map[string]*DoorInstance
	statistics      map[string]*DoorStatistics
	alerts          []*DoorAlert
	
	// Components
	coordinator     *MultiNodeCoordinator
	resourceManager *ResourceManager
	dropFileGen     *DropFileGenerator
	templateEngine  *TemplateEngine
	tester          *DoorTester
	monitor         *DoorMonitor
	
	// Configuration
	config          *DoorManagerConfig
	dataPath        string
	
	// Synchronization
	mu              sync.RWMutex
	eventChan       chan DoorManagerEvent
	stopChan        chan bool
	
	// State
	initialized     bool
	lastSave        time.Time
}

// DoorManagerConfig contains configuration for the door manager
type DoorManagerConfig struct {
	DataPath           string        `json:"data_path"`            // Data storage path
	ConfigFile         string        `json:"config_file"`          // Door config file
	TemplatesPath      string        `json:"templates_path"`       // Templates directory
	BackupPath         string        `json:"backup_path"`          // Backup directory
	SaveInterval       time.Duration `json:"save_interval"`        // Auto-save interval
	BackupInterval     time.Duration `json:"backup_interval"`      // Backup interval
	MaxBackups         int           `json:"max_backups"`          // Maximum backup files
	EnableMonitoring   bool          `json:"enable_monitoring"`    // Enable monitoring
	EnableTesting      bool          `json:"enable_testing"`       // Enable testing
	TestOnSave         bool          `json:"test_on_save"`         // Test doors when saved
	ValidateOnLoad     bool          `json:"validate_on_load"`     // Validate doors when loaded
	LogLevel           string        `json:"log_level"`            // Logging level
	EventBufferSize    int           `json:"event_buffer_size"`    // Event buffer size
}

// DoorManagerEvent represents events from the door manager
type DoorManagerEvent struct {
	Type      DoorManagerEventType `json:"type"`       // Event type
	Timestamp time.Time            `json:"timestamp"`  // Event timestamp
	DoorID    string               `json:"door_id"`    // Door ID
	Data      interface{}          `json:"data"`       // Event data
	Source    string               `json:"source"`     // Event source
	Severity  AlertSeverity        `json:"severity"`   // Event severity
}

// DoorManagerEventType represents the type of door manager event
type DoorManagerEventType int

const (
	EventDoorCreated DoorManagerEventType = iota
	EventDoorUpdated
	EventDoorDeleted
	EventDoorEnabled
	EventDoorDisabled
	EventDoorTested
	EventTemplateLoaded
	EventBackupCreated
	EventConfigSaved
	EventConfigLoaded
	EventManagerStarted
	EventManagerStopped
	EventError
)

func (dmet DoorManagerEventType) String() string {
	switch dmet {
	case EventDoorCreated:
		return "Door Created"
	case EventDoorUpdated:
		return "Door Updated"
	case EventDoorDeleted:
		return "Door Deleted"
	case EventDoorEnabled:
		return "Door Enabled"
	case EventDoorDisabled:
		return "Door Disabled"
	case EventDoorTested:
		return "Door Tested"
	case EventTemplateLoaded:
		return "Template Loaded"
	case EventBackupCreated:
		return "Backup Created"
	case EventConfigSaved:
		return "Config Saved"
	case EventConfigLoaded:
		return "Config Loaded"
	case EventManagerStarted:
		return "Manager Started"
	case EventManagerStopped:
		return "Manager Stopped"
	case EventError:
		return "Error"
	default:
		return "Unknown"
	}
}

// NewDoorManager creates a new door manager
func NewDoorManager(config *DoorManagerConfig) *DoorManagerImpl {
	if config == nil {
		config = &DoorManagerConfig{
			DataPath:          "/opt/vision3/data/doors",
			ConfigFile:        "doors.json",
			TemplatesPath:     "/opt/vision3/templates/doors",
			BackupPath:        "/opt/vision3/backups/doors",
			SaveInterval:      time.Minute * 5,
			BackupInterval:    time.Hour * 24,
			MaxBackups:        30,
			EnableMonitoring:  true,
			EnableTesting:     true,
			TestOnSave:        true,
			ValidateOnLoad:    true,
			LogLevel:          "INFO",
			EventBufferSize:   100,
		}
	}
	
	manager := &DoorManagerImpl{
		doors:        make(map[string]*DoorConfiguration),
		templates:    make(map[string]*DoorTemplate),
		instances:    make(map[string]*DoorInstance),
		statistics:   make(map[string]*DoorStatistics),
		alerts:       make([]*DoorAlert, 0),
		config:       config,
		dataPath:     config.DataPath,
		eventChan:    make(chan DoorManagerEvent, config.EventBufferSize),
		stopChan:     make(chan bool),
		initialized:  false,
	}
	
	// Ensure directories exist
	os.MkdirAll(config.DataPath, 0755)
	os.MkdirAll(config.TemplatesPath, 0755)
	os.MkdirAll(config.BackupPath, 0755)
	
	// Initialize components
	manager.initializeComponents()
	
	return manager
}

// Initialize initializes the door manager
func (dm *DoorManagerImpl) Initialize() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	if dm.initialized {
		return nil
	}
	
	// Load existing configuration
	if err := dm.loadDoorsFromFile(); err != nil {
		return fmt.Errorf("failed to load doors: %w", err)
	}
	
	// Load templates
	if err := dm.loadTemplates(); err != nil {
		// Template loading failure is not critical
		dm.logError("Failed to load templates: %v", err)
	}
	
	// Validate doors if enabled
	if dm.config.ValidateOnLoad {
		dm.validateAllDoors()
	}
	
	// Start background routines
	go dm.saveLoop()
	go dm.backupLoop()
	go dm.eventLoop()
	
	if dm.config.EnableMonitoring && dm.monitor != nil {
		dm.monitor.Start()
	}
	
	dm.initialized = true
	
	// Emit started event
	dm.emitEvent(DoorManagerEvent{
		Type:      EventManagerStarted,
		Timestamp: time.Now(),
		Source:    "manager",
		Severity:  SeverityLow,
	})
	
	return nil
}

// Shutdown gracefully shuts down the door manager
func (dm *DoorManagerImpl) Shutdown() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	if !dm.initialized {
		return nil
	}
	
	// Stop monitoring
	if dm.monitor != nil {
		dm.monitor.Stop()
	}
	
	// Stop coordinator
	if dm.coordinator != nil {
		dm.coordinator.Stop()
	}
	
	// Save current state
	dm.saveDoorsToFile()
	
	// Stop background routines
	close(dm.stopChan)
	
	// Emit stopped event
	dm.emitEvent(DoorManagerEvent{
		Type:      EventManagerStopped,
		Timestamp: time.Now(),
		Source:    "manager",
		Severity:  SeverityLow,
	})
	
	dm.initialized = false
	
	return nil
}

// Door configuration management

func (dm *DoorManagerImpl) GetDoorConfig(doorID string) (*DoorConfiguration, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	door, exists := dm.doors[doorID]
	if !exists {
		return nil, fmt.Errorf("door not found: %s", doorID)
	}
	
	// Return a copy to prevent external modifications
	doorCopy := *door
	return &doorCopy, nil
}

func (dm *DoorManagerImpl) GetAllDoorConfigs() ([]*DoorConfiguration, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	doors := make([]*DoorConfiguration, 0, len(dm.doors))
	for _, door := range dm.doors {
		doorCopy := *door
		doors = append(doors, &doorCopy)
	}
	
	// Sort by name
	sort.Slice(doors, func(i, j int) bool {
		return doors[i].Name < doors[j].Name
	})
	
	return doors, nil
}

func (dm *DoorManagerImpl) CreateDoorConfig(config *DoorConfiguration) error {
	if config == nil {
		return errors.New("door config cannot be nil")
	}
	
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	// Validate configuration
	if err := dm.validateDoorConfig(config); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	
	// Check for duplicate ID
	if _, exists := dm.doors[config.ID]; exists {
		return fmt.Errorf("door with ID '%s' already exists", config.ID)
	}
	
	// Set metadata
	config.Created = time.Now()
	config.Modified = time.Now()
	config.ConfigVersion = 1
	
	// Store configuration
	dm.doors[config.ID] = config
	
	// Initialize statistics
	dm.statistics[config.ID] = &DoorStatistics{
		UserRatings: make([]UserRating, 0),
		DailyStats:  make([]DailyStats, 0),
	}
	
	// Test if enabled
	if dm.config.TestOnSave && dm.tester != nil {
		go dm.testDoor(config.ID)
	}
	
	// Register with coordinator if available
	if dm.coordinator != nil {
		dm.coordinator.RegisterDoor(config)
	}
	
	// Emit event
	dm.emitEvent(DoorManagerEvent{
		Type:      EventDoorCreated,
		Timestamp: time.Now(),
		DoorID:    config.ID,
		Data:      config,
		Source:    "manager",
		Severity:  SeverityLow,
	})
	
	return nil
}

func (dm *DoorManagerImpl) UpdateDoorConfig(config *DoorConfiguration) error {
	if config == nil {
		return errors.New("door config cannot be nil")
	}
	
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	// Check if door exists
	existingConfig, exists := dm.doors[config.ID]
	if !exists {
		return fmt.Errorf("door not found: %s", config.ID)
	}
	
	// Validate configuration
	if err := dm.validateDoorConfig(config); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	
	// Preserve creation time and increment version
	config.Created = existingConfig.Created
	config.Modified = time.Now()
	config.ConfigVersion = existingConfig.ConfigVersion + 1
	
	// Store updated configuration
	dm.doors[config.ID] = config
	
	// Test if enabled
	if dm.config.TestOnSave && dm.tester != nil {
		go dm.testDoor(config.ID)
	}
	
	// Emit event
	dm.emitEvent(DoorManagerEvent{
		Type:      EventDoorUpdated,
		Timestamp: time.Now(),
		DoorID:    config.ID,
		Data:      config,
		Source:    "manager",
		Severity:  SeverityLow,
	})
	
	return nil
}

func (dm *DoorManagerImpl) DeleteDoorConfig(doorID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	// Check if door exists
	_, exists := dm.doors[doorID]
	if !exists {
		return fmt.Errorf("door not found: %s", doorID)
	}
	
	// Check for active instances
	for _, instance := range dm.instances {
		if instance.DoorID == doorID {
			return fmt.Errorf("cannot delete door with active instances")
		}
	}
	
	// Remove door
	delete(dm.doors, doorID)
	delete(dm.statistics, doorID)
	
	// Remove door-specific alerts
	newAlerts := make([]*DoorAlert, 0)
	for _, alert := range dm.alerts {
		if alert.DoorID != doorID {
			newAlerts = append(newAlerts, alert)
		}
	}
	dm.alerts = newAlerts
	
	// Emit event
	dm.emitEvent(DoorManagerEvent{
		Type:      EventDoorDeleted,
		Timestamp: time.Now(),
		DoorID:    doorID,
		Source:    "manager",
		Severity:  SeverityMedium,
	})
	
	return nil
}

// Template management

func (dm *DoorManagerImpl) GetDoorTemplate(templateID string) (*DoorTemplate, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	template, exists := dm.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}
	
	// Return a copy
	templateCopy := *template
	return &templateCopy, nil
}

func (dm *DoorManagerImpl) GetAllDoorTemplates() ([]*DoorTemplate, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	templates := make([]*DoorTemplate, 0, len(dm.templates))
	for _, template := range dm.templates {
		templateCopy := *template
		templates = append(templates, &templateCopy)
	}
	
	// Sort by name
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})
	
	return templates, nil
}

func (dm *DoorManagerImpl) CreateDoorFromTemplate(templateID string, vars map[string]interface{}) (*DoorConfiguration, error) {
	template, err := dm.GetDoorTemplate(templateID)
	if err != nil {
		return nil, err
	}
	
	// Create base configuration from template
	config := template.Config
	config.ID = dm.generateUniqueID(config.Name)
	config.Created = time.Now()
	config.Modified = time.Now()
	config.ConfigVersion = 1
	
	// Apply template variables
	if dm.templateEngine != nil {
		// Process command
		config.Command, err = dm.templateEngine.ProcessTemplate(config.Command, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to process command template: %w", err)
		}
		
		// Process arguments
		config.Arguments, err = dm.templateEngine.ProcessArguments(config.Arguments, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to process arguments template: %w", err)
		}
		
		// Process working directory
		config.WorkingDirectory, err = dm.templateEngine.ProcessTemplate(config.WorkingDirectory, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to process working directory template: %w", err)
		}
		
		// Process environment variables
		config.EnvironmentVariables, err = dm.templateEngine.ProcessEnvironment(config.EnvironmentVariables, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to process environment variables: %w", err)
		}
	}
	
	return &config, nil
}

// Instance management

func (dm *DoorManagerImpl) LaunchDoor(doorID string, nodeID int, userID int) (*DoorInstance, error) {
	if dm.coordinator != nil {
		// Use coordinator for multi-node management
		return dm.coordinator.RequestDoorAccess(doorID, nodeID, userID, nil, nil)
	}
	
	// Fallback to simple launch
	return dm.launchDoorSimple(doorID, nodeID, userID)
}

func (dm *DoorManagerImpl) launchDoorSimple(doorID string, nodeID int, userID int) (*DoorInstance, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	door, exists := dm.doors[doorID]
	if !exists {
		return nil, fmt.Errorf("door not found: %s", doorID)
	}
	
	if !door.Enabled {
		return nil, fmt.Errorf("door is disabled: %s", doorID)
	}
	
	// Create instance
	instanceID := fmt.Sprintf("%s_%d_%d_%d", doorID, nodeID, userID, time.Now().Unix())
	instance := &DoorInstance{
		ID:            instanceID,
		DoorID:        doorID,
		NodeID:        nodeID,
		UserID:        userID,
		StartTime:     time.Now(),
		Status:        InstanceStatusStarting,
		ResourceLocks: make([]string, 0),
		LastActivity:  time.Now(),
	}
	
	dm.instances[instanceID] = instance
	
	// TODO: Actually launch the door process
	
	return instance, nil
}

func (dm *DoorManagerImpl) GetDoorInstance(instanceID string) (*DoorInstance, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	instance, exists := dm.instances[instanceID]
	if !exists {
		return nil, fmt.Errorf("instance not found: %s", instanceID)
	}
	
	// Return a copy
	instanceCopy := *instance
	return &instanceCopy, nil
}

func (dm *DoorManagerImpl) GetActiveDoorInstances() ([]*DoorInstance, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	instances := make([]*DoorInstance, 0)
	for _, instance := range dm.instances {
		if instance.Status == InstanceStatusRunning || instance.Status == InstanceStatusStarting {
			instanceCopy := *instance
			instances = append(instances, &instanceCopy)
		}
	}
	
	return instances, nil
}

func (dm *DoorManagerImpl) GetNodeDoorInstances(nodeID int) ([]*DoorInstance, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	instances := make([]*DoorInstance, 0)
	for _, instance := range dm.instances {
		if instance.NodeID == nodeID {
			instanceCopy := *instance
			instances = append(instances, &instanceCopy)
		}
	}
	
	return instances, nil
}

func (dm *DoorManagerImpl) TerminateDoorInstance(instanceID string) error {
	if dm.coordinator != nil {
		return dm.coordinator.ReleaseDoorAccess(instanceID)
	}
	
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	instance, exists := dm.instances[instanceID]
	if !exists {
		return fmt.Errorf("instance not found: %s", instanceID)
	}
	
	instance.Status = InstanceStatusFinished
	delete(dm.instances, instanceID)
	
	return nil
}

// Queue management (delegate to coordinator)

func (dm *DoorManagerImpl) AddToQueue(doorID string, nodeID int, userID int) error {
	if dm.coordinator != nil {
		return dm.coordinator.RemoveFromQueue(doorID, userID)
	}
	return errors.New("queue management not available without coordinator")
}

func (dm *DoorManagerImpl) RemoveFromQueue(doorID string, userID int) error {
	if dm.coordinator != nil {
		return dm.coordinator.RemoveFromQueue(doorID, userID)
	}
	return errors.New("queue management not available without coordinator")
}

func (dm *DoorManagerImpl) GetQueuePosition(doorID string, userID int) (int, error) {
	if dm.coordinator != nil {
		return dm.coordinator.GetQueuePosition(doorID, userID)
	}
	return -1, errors.New("queue management not available without coordinator")
}

func (dm *DoorManagerImpl) GetQueueStatus(doorID string) (*DoorQueue, error) {
	// This would need to be implemented in the coordinator
	return nil, errors.New("queue status not implemented")
}

// Resource management (delegate to resource manager)

func (dm *DoorManagerImpl) AcquireResource(resourceID string, doorID string, instanceID string, mode LockMode) (*ResourceLock, error) {
	if dm.resourceManager != nil {
		return dm.resourceManager.AcquireLock(resourceID, doorID, instanceID, 0, 0, mode, 0)
	}
	return nil, errors.New("resource management not available")
}

func (dm *DoorManagerImpl) ReleaseResource(lockID string) error {
	if dm.resourceManager != nil {
		return dm.resourceManager.ReleaseLock(lockID)
	}
	return errors.New("resource management not available")
}

func (dm *DoorManagerImpl) GetResourceStatus(resourceID string) (*DoorResource, error) {
	if dm.resourceManager != nil {
		return dm.resourceManager.GetResourceStatus(resourceID)
	}
	return nil, errors.New("resource management not available")
}

// Testing and validation

func (dm *DoorManagerImpl) TestDoorConfig(doorID string) error {
	if dm.tester == nil {
		return errors.New("testing not available")
	}
	
	_, err := dm.GetDoorConfig(doorID)
	if err != nil {
		return err
	}
	
	go dm.testDoor(doorID)
	
	return nil
}

func (dm *DoorManagerImpl) testDoor(doorID string) {
	door, err := dm.GetDoorConfig(doorID)
	if err != nil {
		dm.logError("Failed to get door config for testing: %v", err)
		return
	}
	
	result, err := dm.tester.TestDoorConfiguration(door)
	if err != nil {
		dm.logError("Failed to test door %s: %v", doorID, err)
		return
	}
	
	// Emit test event
	dm.emitEvent(DoorManagerEvent{
		Type:      EventDoorTested,
		Timestamp: time.Now(),
		DoorID:    doorID,
		Data:      result,
		Source:    "tester",
		Severity:  SeverityLow,
	})
	
	// Create alerts for test failures
	if !result.Passed {
		alert := &DoorAlert{
			ID:           fmt.Sprintf("test_fail_%s_%d", doorID, time.Now().Unix()),
			DoorID:       doorID,
			AlertType:    AlertTypeError,
			Severity:     SeverityHigh,
			Message:      "Door test failed",
			Details:      fmt.Sprintf("Test score: %.1f, Errors: %d", result.Score, len(result.Errors)),
			Timestamp:    time.Now(),
			Acknowledged: false,
			AutoClear:    false,
			Actions:      []string{"Check door configuration", "Review test results", "Fix reported issues"},
		}
		
		dm.mu.Lock()
		dm.alerts = append(dm.alerts, alert)
		dm.mu.Unlock()
	}
}

func (dm *DoorManagerImpl) ValidateDoorConfig(config *DoorConfiguration) error {
	return dm.validateDoorConfig(config)
}

func (dm *DoorManagerImpl) validateDoorConfig(config *DoorConfiguration) error {
	if config.ID == "" {
		return errors.New("door ID is required")
	}
	
	if config.Name == "" {
		return errors.New("door name is required")
	}
	
	if config.Command == "" {
		return errors.New("door command is required")
	}
	
	// Additional validation can be added here
	
	return nil
}

func (dm *DoorManagerImpl) validateAllDoors() {
	for doorID, door := range dm.doors {
		if err := dm.validateDoorConfig(door); err != nil {
			dm.logError("Door %s failed validation: %v", doorID, err)
		}
	}
}

// Statistics management

func (dm *DoorManagerImpl) GetDoorStatistics(doorID string) (*DoorStatistics, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	stats, exists := dm.statistics[doorID]
	if !exists {
		return nil, fmt.Errorf("statistics not found for door: %s", doorID)
	}
	
	// Return a copy
	statsCopy := *stats
	return &statsCopy, nil
}

func (dm *DoorManagerImpl) UpdateDoorStatistics(doorID string, stats *DoorStatistics) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	if _, exists := dm.doors[doorID]; !exists {
		return fmt.Errorf("door not found: %s", doorID)
	}
	
	dm.statistics[doorID] = stats
	return nil
}

func (dm *DoorManagerImpl) GetSystemDoorStats() (map[string]*DoorStatistics, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	result := make(map[string]*DoorStatistics)
	for doorID, stats := range dm.statistics {
		statsCopy := *stats
		result[doorID] = &statsCopy
	}
	
	return result, nil
}

// Alert management

func (dm *DoorManagerImpl) GetDoorAlerts(doorID string) ([]*DoorAlert, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	var alerts []*DoorAlert
	for _, alert := range dm.alerts {
		if alert.DoorID == doorID {
			alertCopy := *alert
			alerts = append(alerts, &alertCopy)
		}
	}
	
	return alerts, nil
}

func (dm *DoorManagerImpl) GetAllDoorAlerts() ([]*DoorAlert, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	alerts := make([]*DoorAlert, len(dm.alerts))
	for i, alert := range dm.alerts {
		alertCopy := *alert
		alerts[i] = &alertCopy
	}
	
	return alerts, nil
}

func (dm *DoorManagerImpl) AcknowledgeAlert(alertID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	for _, alert := range dm.alerts {
		if alert.ID == alertID {
			alert.Acknowledged = true
			return nil
		}
	}
	
	return fmt.Errorf("alert not found: %s", alertID)
}

func (dm *DoorManagerImpl) ResolveAlert(alertID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	for _, alert := range dm.alerts {
		if alert.ID == alertID {
			alert.Resolved = true
			alert.ResolvedAt = time.Now()
			return nil
		}
	}
	
	return fmt.Errorf("alert not found: %s", alertID)
}

func (dm *DoorManagerImpl) AddAlert(alert *DoorAlert) error {
	if alert == nil {
		return errors.New("alert cannot be nil")
	}
	
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	// Generate ID if not set
	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert_%d", time.Now().UnixNano())
	}
	
	// Set timestamp if not set
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}
	
	dm.alerts = append(dm.alerts, alert)
	return nil
}

// Maintenance operations

func (dm *DoorManagerImpl) CleanupExpiredLocks() error {
	if dm.resourceManager != nil {
		cleaned := dm.resourceManager.CleanupExpiredLocks()
		dm.logInfo("Cleaned up %d expired resource locks", cleaned)
		return nil
	}
	return errors.New("resource manager not available")
}

func (dm *DoorManagerImpl) CleanupFinishedInstances() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	count := 0
	for instanceID, instance := range dm.instances {
		if instance.Status == InstanceStatusFinished || 
		   instance.Status == InstanceStatusCrashed ||
		   instance.Status == InstanceStatusKilled {
			delete(dm.instances, instanceID)
			count++
		}
	}
	
	dm.logInfo("Cleaned up %d finished door instances", count)
	return nil
}

func (dm *DoorManagerImpl) PerformDoorMaintenance(doorID string) error {
	// TODO: Implement door-specific maintenance
	return nil
}

func (dm *DoorManagerImpl) BackupDoorConfigs() error {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(dm.config.BackupPath, fmt.Sprintf("doors_backup_%s.json", timestamp))
	
	// Create backup data
	backupData := map[string]interface{}{
		"timestamp":   time.Now(),
		"version":     "1.0",
		"doors":       dm.doors,
		"statistics":  dm.statistics,
		"alerts":      dm.alerts,
	}
	
	// Write backup file
	data, err := json.MarshalIndent(backupData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal backup data: %w", err)
	}
	
	if err := os.WriteFile(backupFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}
	
	// Clean up old backups
	dm.cleanupOldBackups()
	
	// Emit event
	dm.emitEvent(DoorManagerEvent{
		Type:      EventBackupCreated,
		Timestamp: time.Now(),
		Data:      backupFile,
		Source:    "manager",
		Severity:  SeverityLow,
	})
	
	return nil
}

func (dm *DoorManagerImpl) RestoreDoorConfigs(backupPath string) error {
	// Read backup file
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}
	
	// Parse backup data
	var backupData map[string]interface{}
	if err := json.Unmarshal(data, &backupData); err != nil {
		return fmt.Errorf("failed to parse backup data: %w", err)
	}
	
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	// Restore doors
	if doorsData, exists := backupData["doors"]; exists {
		doorsJSON, _ := json.Marshal(doorsData)
		var doors map[string]*DoorConfiguration
		if err := json.Unmarshal(doorsJSON, &doors); err == nil {
			dm.doors = doors
		}
	}
	
	// Restore statistics
	if statsData, exists := backupData["statistics"]; exists {
		statsJSON, _ := json.Marshal(statsData)
		var stats map[string]*DoorStatistics
		if err := json.Unmarshal(statsJSON, &stats); err == nil {
			dm.statistics = stats
		}
	}
	
	// Restore alerts
	if alertsData, exists := backupData["alerts"]; exists {
		alertsJSON, _ := json.Marshal(alertsData)
		var alerts []*DoorAlert
		if err := json.Unmarshal(alertsJSON, &alerts); err == nil {
			dm.alerts = alerts
		}
	}
	
	// Save restored configuration
	dm.saveDoorsToFile()
	
	return nil
}

// Helper methods

func (dm *DoorManagerImpl) initializeComponents() {
	// Initialize resource manager
	dm.resourceManager = NewResourceManager(nil)
	
	// Initialize drop file generator
	dm.dropFileGen = NewDropFileGenerator(nil)
	
	// Initialize template engine
	dm.templateEngine = NewTemplateEngine(nil)
	
	// Initialize tester if enabled
	if dm.config.EnableTesting {
		dm.tester = NewDoorTester(nil)
	}
	
	// Initialize monitor if enabled
	if dm.config.EnableMonitoring {
		dm.monitor = NewDoorMonitor(nil)
	}
	
	// Initialize coordinator
	dm.coordinator = NewMultiNodeCoordinator(nil, dm.resourceManager)
}

func (dm *DoorManagerImpl) loadDoorsFromFile() error {
	configPath := filepath.Join(dm.dataPath, dm.config.ConfigFile)
	
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// File doesn't exist, start with empty configuration
		return nil
	}
	
	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	
	// Parse JSON
	var doors []*DoorConfiguration
	if err := json.Unmarshal(data, &doors); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}
	
	// Load doors into memory
	for _, door := range doors {
		dm.doors[door.ID] = door
		
		// Initialize statistics if not present
		if _, exists := dm.statistics[door.ID]; !exists {
			dm.statistics[door.ID] = &DoorStatistics{
				UserRatings: make([]UserRating, 0),
				DailyStats:  make([]DailyStats, 0),
			}
		}
	}
	
	// Emit event
	dm.emitEvent(DoorManagerEvent{
		Type:      EventConfigLoaded,
		Timestamp: time.Now(),
		Data:      len(doors),
		Source:    "manager",
		Severity:  SeverityLow,
	})
	
	return nil
}

func (dm *DoorManagerImpl) saveDoorsToFile() error {
	configPath := filepath.Join(dm.dataPath, dm.config.ConfigFile)
	
	// Convert doors to slice for JSON serialization
	doors := make([]*DoorConfiguration, 0, len(dm.doors))
	for _, door := range dm.doors {
		doors = append(doors, door)
	}
	
	// Marshal to JSON
	data, err := json.MarshalIndent(doors, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal doors: %w", err)
	}
	
	// Write file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	dm.lastSave = time.Now()
	
	// Emit event
	dm.emitEvent(DoorManagerEvent{
		Type:      EventConfigSaved,
		Timestamp: time.Now(),
		Data:      len(doors),
		Source:    "manager",
		Severity:  SeverityLow,
	})
	
	return nil
}

func (dm *DoorManagerImpl) loadTemplates() error {
	templatesDir := dm.config.TemplatesPath
	
	// Check if templates directory exists
	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		return nil // No templates directory
	}
	
	// Read template files
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		return fmt.Errorf("failed to read templates directory: %w", err)
	}
	
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		
		templatePath := filepath.Join(templatesDir, entry.Name())
		if err := dm.loadTemplate(templatePath); err != nil {
			dm.logError("Failed to load template %s: %v", templatePath, err)
		}
	}
	
	return nil
}

func (dm *DoorManagerImpl) loadTemplate(templatePath string) error {
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}
	
	var template DoorTemplate
	if err := json.Unmarshal(data, &template); err != nil {
		return err
	}
	
	dm.templates[template.ID] = &template
	
	// Emit event
	dm.emitEvent(DoorManagerEvent{
		Type:      EventTemplateLoaded,
		Timestamp: time.Now(),
		Data:      template.ID,
		Source:    "manager",
		Severity:  SeverityLow,
	})
	
	return nil
}

func (dm *DoorManagerImpl) generateUniqueID(baseName string) string {
	// Convert name to valid ID
	id := strings.ToLower(baseName)
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, "-", "_")
	
	// Ensure uniqueness
	originalID := id
	counter := 1
	
	for {
		if _, exists := dm.doors[id]; !exists {
			break
		}
		id = fmt.Sprintf("%s_%d", originalID, counter)
		counter++
	}
	
	return id
}

func (dm *DoorManagerImpl) cleanupOldBackups() {
	entries, err := os.ReadDir(dm.config.BackupPath)
	if err != nil {
		return
	}
	
	// Filter backup files and sort by modification time
	var backupFiles []os.DirEntry
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "doors_backup_") && strings.HasSuffix(entry.Name(), ".json") {
			backupFiles = append(backupFiles, entry)
		}
	}
	
	if len(backupFiles) <= dm.config.MaxBackups {
		return
	}
	
	// Sort by name (which includes timestamp)
	sort.Slice(backupFiles, func(i, j int) bool {
		return backupFiles[i].Name() < backupFiles[j].Name()
	})
	
	// Remove oldest files
	toRemove := len(backupFiles) - dm.config.MaxBackups
	for i := 0; i < toRemove; i++ {
		backupPath := filepath.Join(dm.config.BackupPath, backupFiles[i].Name())
		os.Remove(backupPath)
	}
}

func (dm *DoorManagerImpl) emitEvent(event DoorManagerEvent) {
	select {
	case dm.eventChan <- event:
		// Event sent
	default:
		// Channel full, drop event
	}
}

func (dm *DoorManagerImpl) logInfo(format string, args ...interface{}) {
	// TODO: Implement proper logging
	fmt.Printf("[INFO] "+format+"\n", args...)
}

func (dm *DoorManagerImpl) logError(format string, args ...interface{}) {
	// TODO: Implement proper logging
	fmt.Printf("[ERROR] "+format+"\n", args...)
}

// Background loops

func (dm *DoorManagerImpl) saveLoop() {
	ticker := time.NewTicker(dm.config.SaveInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if time.Since(dm.lastSave) >= dm.config.SaveInterval {
				dm.saveDoorsToFile()
			}
		case <-dm.stopChan:
			return
		}
	}
}

func (dm *DoorManagerImpl) backupLoop() {
	ticker := time.NewTicker(dm.config.BackupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			dm.BackupDoorConfigs()
		case <-dm.stopChan:
			return
		}
	}
}

func (dm *DoorManagerImpl) eventLoop() {
	for {
		select {
		case event := <-dm.eventChan:
			// Process event (could send to monitoring system, log, etc.)
			dm.processEvent(event)
		case <-dm.stopChan:
			return
		}
	}
}

func (dm *DoorManagerImpl) processEvent(event DoorManagerEvent) {
	// TODO: Process events (logging, monitoring, notifications, etc.)
	switch event.Type {
	case EventError:
		dm.logError("Door manager event: %s", event.Data)
	default:
		if dm.config.LogLevel == "DEBUG" {
			dm.logInfo("Door manager event: %s - %s", event.Type.String(), event.DoorID)
		}
	}
}