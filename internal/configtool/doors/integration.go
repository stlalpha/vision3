package doors

import (
	"fmt"
	"time"
	
	tea "github.com/charmbracelet/bubbletea"
	
	"github.com/stlalpha/vision3/internal/configtool/tui"
	"github.com/stlalpha/vision3/internal/configtool/nodes"
	"github.com/stlalpha/vision3/internal/user"
)

// DoorIntegrator integrates the door system with existing BBS components
type DoorIntegrator struct {
	doorManager     DoorManager
	nodeManager     nodes.NodeManager
	coordinator     *MultiNodeCoordinator
	monitor         *DoorMonitor
	userManager     *user.UserMgr
	
	// Integration configuration
	config          *IntegrationConfig
	
	// Event handlers
	eventHandlers   map[string][]EventHandler
	
	// State tracking
	activeDoors     map[int]string // nodeID -> doorID
	doorHistory     []DoorLaunchEvent
}

// IntegrationConfig contains configuration for door system integration
type IntegrationConfig struct {
	EnableNodeIntegration  bool          `json:"enable_node_integration"`   // Enable node management integration
	EnableUserIntegration  bool          `json:"enable_user_integration"`   // Enable user management integration
	EnableMenuIntegration  bool          `json:"enable_menu_integration"`   // Enable menu system integration
	EnableStatsIntegration bool          `json:"enable_stats_integration"`  // Enable statistics integration
	AutoNodeRegistration   bool          `json:"auto_node_registration"`    // Auto-register nodes
	SyncUserTime          bool          `json:"sync_user_time"`            // Sync user time limits
	TrackDoorUsage        bool          `json:"track_door_usage"`          // Track door usage statistics
	LogDoorActivity       bool          `json:"log_door_activity"`         // Log door activity
	EventBufferSize       int           `json:"event_buffer_size"`         // Event buffer size
}

// EventHandler handles integration events
type EventHandler func(event IntegrationEvent) error

// IntegrationEvent represents an event in the integration system
type IntegrationEvent struct {
	Type      IntegrationEventType `json:"type"`       // Event type
	Timestamp time.Time            `json:"timestamp"`  // Event timestamp
	NodeID    int                  `json:"node_id"`    // Node ID
	UserID    int                  `json:"user_id"`    // User ID
	DoorID    string               `json:"door_id"`    // Door ID
	Data      interface{}          `json:"data"`       // Event data
	Context   map[string]interface{} `json:"context"`  // Event context
}

// IntegrationEventType represents the type of integration event
type IntegrationEventType int

const (
	EventUserEnterDoor IntegrationEventType = iota
	EventUserExitDoor
	EventDoorLaunchFailed
	EventNodeStatusChanged
	EventUserTimeExpired
	EventDoorQueueChanged
	EventSystemAlert
	EventStatisticsUpdate
)

func (iet IntegrationEventType) String() string {
	switch iet {
	case EventUserEnterDoor:
		return "User Enter Door"
	case EventUserExitDoor:
		return "User Exit Door"
	case EventDoorLaunchFailed:
		return "Door Launch Failed"
	case EventNodeStatusChanged:
		return "Node Status Changed"
	case EventUserTimeExpired:
		return "User Time Expired"
	case EventDoorQueueChanged:
		return "Door Queue Changed"
	case EventSystemAlert:
		return "System Alert"
	case EventStatisticsUpdate:
		return "Statistics Update"
	default:
		return "Unknown"
	}
}

// DoorLaunchEvent represents a door launch event
type DoorLaunchEvent struct {
	Timestamp   time.Time `json:"timestamp"`    // Launch timestamp
	NodeID      int       `json:"node_id"`      // Node ID
	UserID      int       `json:"user_id"`      // User ID
	DoorID      string    `json:"door_id"`      // Door ID
	InstanceID  string    `json:"instance_id"`  // Instance ID
	Success     bool      `json:"success"`      // Launch success
	Duration    time.Duration `json:"duration"` // Door session duration
	ExitCode    int       `json:"exit_code"`    // Door exit code
	ErrorMsg    string    `json:"error_msg"`    // Error message if failed
}

// NewDoorIntegrator creates a new door integrator
func NewDoorIntegrator(doorManager DoorManager, nodeManager nodes.NodeManager, userManager *user.UserMgr) *DoorIntegrator {
	config := &IntegrationConfig{
		EnableNodeIntegration:  true,
		EnableUserIntegration:  true,
		EnableMenuIntegration:  true,
		EnableStatsIntegration: true,
		AutoNodeRegistration:   true,
		SyncUserTime:          true,
		TrackDoorUsage:        true,
		LogDoorActivity:       true,
		EventBufferSize:       100,
	}
	
	integrator := &DoorIntegrator{
		doorManager:   doorManager,
		nodeManager:   nodeManager,
		userManager:   userManager,
		config:        config,
		eventHandlers: make(map[string][]EventHandler),
		activeDoors:   make(map[int]string),
		doorHistory:   make([]DoorLaunchEvent, 0),
	}
	
	// Initialize components if needed
	if coordinator, ok := doorManager.(*DoorManagerImpl); ok {
		integrator.coordinator = coordinator.coordinator
		integrator.monitor = coordinator.monitor
	}
	
	// Register default event handlers
	integrator.registerDefaultHandlers()
	
	return integrator
}

// Initialize initializes the door integrator
func (di *DoorIntegrator) Initialize() error {
	// Start monitoring if available
	if di.monitor != nil {
		di.monitor.Start()
	}
	
	// Register with node manager for events
	if di.config.EnableNodeIntegration && di.nodeManager != nil {
		// TODO: Register for node events
	}
	
	// Auto-register existing nodes if enabled
	if di.config.AutoNodeRegistration {
		di.autoRegisterNodes()
	}
	
	return nil
}

// Shutdown shuts down the door integrator
func (di *DoorIntegrator) Shutdown() error {
	// Stop monitoring
	if di.monitor != nil {
		di.monitor.Stop()
	}
	
	// Force stop all active doors
	for nodeID, doorID := range di.activeDoors {
		di.forceStopDoor(nodeID, doorID)
	}
	
	return nil
}

// LaunchDoorForUser launches a door for a user on a specific node
func (di *DoorIntegrator) LaunchDoorForUser(doorID string, nodeID int, userID int) (*DoorInstance, error) {
	// Get user information
	user, exists := di.userManager.GetUserByID(userID)
	if !exists {
		return nil, fmt.Errorf("user with ID %d not found", userID)
	}
	
	// Check if user has sufficient time
	if di.config.SyncUserTime && user.TimeLeft <= 0 {
		return nil, fmt.Errorf("user has no time remaining")
	}
	
	// Get door configuration
	doorConfig, err := di.doorManager.GetDoorConfig(doorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get door config: %w", err)
	}
	
	// Check user access
	if err := di.checkUserAccess(user, doorConfig); err != nil {
		return nil, fmt.Errorf("access denied: %w", err)
	}
	
	// Check if user already has a door running
	if existingDoorID, exists := di.activeDoors[nodeID]; exists {
		return nil, fmt.Errorf("user already has door '%s' running on node %d", existingDoorID, nodeID)
	}
	
	// Update node status
	if di.config.EnableNodeIntegration && di.nodeManager != nil {
		nodeActivity := nodes.NodeActivity{
			Type:        "door",
			Description: fmt.Sprintf("Starting door: %s", doorConfig.Name),
			StartTime:   time.Now(),
			DoorName:    doorConfig.Name,
		}
		di.nodeManager.UpdateActivity(nodeID, nodeActivity)
	}
	
	// Launch the door
	instance, err := di.doorManager.LaunchDoor(doorID, nodeID, userID)
	if err != nil {
		// Emit launch failed event
		di.emitEvent(IntegrationEvent{
			Type:      EventDoorLaunchFailed,
			Timestamp: time.Now(),
			NodeID:    nodeID,
			UserID:    userID,
			DoorID:    doorID,
			Data:      err.Error(),
		})
		
		return nil, fmt.Errorf("failed to launch door: %w", err)
	}
	
	// Track active door
	di.activeDoors[nodeID] = doorID
	
	// Register instance with monitor
	if di.monitor != nil {
		di.monitor.RegisterInstance(instance)
	}
	
	// Update node status to "in door"
	if di.config.EnableNodeIntegration && di.nodeManager != nil {
		nodeActivity := nodes.NodeActivity{
			Type:        "door",
			Description: fmt.Sprintf("In door: %s", doorConfig.Name),
			StartTime:   time.Now(),
			DoorName:    doorConfig.Name,
		}
		di.nodeManager.UpdateActivity(nodeID, nodeActivity)
	}
	
	// Emit user enter door event
	di.emitEvent(IntegrationEvent{
		Type:      EventUserEnterDoor,
		Timestamp: time.Now(),
		NodeID:    nodeID,
		UserID:    userID,
		DoorID:    doorID,
		Data:      instance,
		Context: map[string]interface{}{
			"door_name": doorConfig.Name,
			"user_name": user.Handle,
		},
	})
	
	// Start monitoring door session
	go di.monitorDoorSession(instance, user)
	
	return instance, nil
}

// TerminateDoorForUser terminates a door for a user
func (di *DoorIntegrator) TerminateDoorForUser(nodeID int, userID int) error {
	// Check if user has an active door
	doorID, exists := di.activeDoors[nodeID]
	if !exists {
		return fmt.Errorf("no active door found for user on node %d", nodeID)
	}
	
	// Find the instance
	instances, err := di.doorManager.GetNodeDoorInstances(nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node instances: %w", err)
	}
	
	var targetInstance *DoorInstance
	for _, instance := range instances {
		if instance.UserID == userID && instance.DoorID == doorID {
			targetInstance = instance
			break
		}
	}
	
	if targetInstance == nil {
		return fmt.Errorf("door instance not found")
	}
	
	// Terminate the instance
	if err := di.doorManager.TerminateDoorInstance(targetInstance.ID); err != nil {
		return fmt.Errorf("failed to terminate door instance: %w", err)
	}
	
	// Clean up tracking
	delete(di.activeDoors, nodeID)
	
	// Unregister from monitor
	if di.monitor != nil {
		di.monitor.UnregisterInstance(targetInstance.ID)
	}
	
	// Update node status
	if di.config.EnableNodeIntegration && di.nodeManager != nil {
		nodeActivity := nodes.NodeActivity{
			Type:        "menu",
			Description: "Returned from door",
			StartTime:   time.Now(),
		}
		di.nodeManager.UpdateActivity(nodeID, nodeActivity)
	}
	
	// Calculate session duration
	duration := time.Since(targetInstance.StartTime)
	
	// Update user time if configured
	if di.config.SyncUserTime {
		di.updateUserTime(userID, duration)
	}
	
	// Record door usage
	if di.config.TrackDoorUsage {
		di.recordDoorUsage(targetInstance, duration)
	}
	
	// Emit user exit door event
	di.emitEvent(IntegrationEvent{
		Type:      EventUserExitDoor,
		Timestamp: time.Now(),
		NodeID:    nodeID,
		UserID:    userID,
		DoorID:    doorID,
		Data:      targetInstance,
		Context: map[string]interface{}{
			"duration": duration,
			"success":  true,
		},
	})
	
	return nil
}

// GetActiveDoorForNode returns the active door for a node
func (di *DoorIntegrator) GetActiveDoorForNode(nodeID int) (string, bool) {
	doorID, exists := di.activeDoors[nodeID]
	return doorID, exists
}

// CreateDoorTUIScreen creates a TUI screen for door configuration
func (di *DoorIntegrator) CreateDoorTUIScreen() tea.Model {
	return NewDoorConfigTUI(di.doorManager, di.coordinator)
}

// RegisterEventHandler registers an event handler
func (di *DoorIntegrator) RegisterEventHandler(eventType string, handler EventHandler) {
	if di.eventHandlers[eventType] == nil {
		di.eventHandlers[eventType] = make([]EventHandler, 0)
	}
	di.eventHandlers[eventType] = append(di.eventHandlers[eventType], handler)
}

// Helper methods

func (di *DoorIntegrator) checkUserAccess(user *user.User, doorConfig *DoorConfiguration) error {
	// Check access level
	if user.AccessLevel < doorConfig.MinimumAccessLevel {
		return fmt.Errorf("insufficient access level: required %d, user has %d", doorConfig.MinimumAccessLevel, user.AccessLevel)
	}
	
	// Check required flags
	for _, requiredFlag := range doorConfig.RequiredFlags {
		if !user.HasFlag(requiredFlag) {
			return fmt.Errorf("missing required flag: %s", requiredFlag)
		}
	}
	
	// Check forbidden flags
	for _, forbiddenFlag := range doorConfig.ForbiddenFlags {
		if user.HasFlag(forbiddenFlag) {
			return fmt.Errorf("user has forbidden flag: %s", forbiddenFlag)
		}
	}
	
	// Check time limits
	if doorConfig.TimeLimit > 0 && user.TimeLeft < doorConfig.TimeLimit {
		return fmt.Errorf("insufficient time: required %d minutes, user has %d", doorConfig.TimeLimit, user.TimeLeft)
	}
	
	return nil
}

func (di *DoorIntegrator) autoRegisterNodes() {
	if di.nodeManager == nil {
		return
	}
	
	// Get all nodes
	nodes := di.nodeManager.GetAllNodes()
	
	// Register each node with the coordinator
	if di.coordinator != nil {
		for _, node := range nodes {
			capabilities := NodeCapabilities{
				SupportedDoors: []string{"*"}, // Support all doors by default
				MaxMemory:      1024 * 1024 * 1024, // 1GB
				MaxCPU:         80.0,
				OSType:         "linux",
				Architecture:   "x86_64",
				NetworkAccess:  true,
				LocalStorage:   10 * 1024 * 1024 * 1024, // 10GB
			}
			
			di.coordinator.RegisterNode(node.NodeID, capabilities)
		}
	}
}

func (di *DoorIntegrator) monitorDoorSession(instance *DoorInstance, user *user.User) {
	ticker := time.NewTicker(time.Second * 30) // Check every 30 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Check if instance is still active
			currentInstance, err := di.doorManager.GetDoorInstance(instance.ID)
			if err != nil {
				// Instance no longer exists, stop monitoring
				return
			}
			
			// Check for timeout
			doorConfig, err := di.doorManager.GetDoorConfig(instance.DoorID)
			if err == nil && doorConfig.TimeLimit > 0 {
				runtime := time.Since(instance.StartTime)
				if runtime > time.Duration(doorConfig.TimeLimit)*time.Minute {
					// Door has exceeded time limit, terminate it
					di.TerminateDoorForUser(instance.NodeID, instance.UserID)
					
					// Emit time expired event
					di.emitEvent(IntegrationEvent{
						Type:      EventUserTimeExpired,
						Timestamp: time.Now(),
						NodeID:    instance.NodeID,
						UserID:    instance.UserID,
						DoorID:    instance.DoorID,
						Data:      runtime,
					})
					return
				}
			}
			
			// Check if instance finished
			if currentInstance.Status == InstanceStatusFinished || 
			   currentInstance.Status == InstanceStatusCrashed ||
			   currentInstance.Status == InstanceStatusKilled {
				// Clean up
				delete(di.activeDoors, instance.NodeID)
				
				// Update user time
				duration := time.Since(instance.StartTime)
				if di.config.SyncUserTime {
					di.updateUserTime(instance.UserID, duration)
				}
				
				// Record usage
				if di.config.TrackDoorUsage {
					di.recordDoorUsage(instance, duration)
				}
				
				return
			}
			
		case <-time.After(time.Hour * 2): // Maximum monitoring time
			// Stop monitoring after 2 hours
			return
		}
	}
}

func (di *DoorIntegrator) updateUserTime(userID int, duration time.Duration) {
	user, exists := di.userManager.GetUserByID(userID)
	if !exists {
		return
	}
	
	// Deduct time used
	timeUsed := int(duration.Minutes())
	if user.TimeLeft > timeUsed {
		user.TimeLeft -= timeUsed
	} else {
		user.TimeLeft = 0
	}
	
	// Update user
	di.userManager.UpdateUser(user)
}

func (di *DoorIntegrator) recordDoorUsage(instance *DoorInstance, duration time.Duration) {
	// Record in door history
	event := DoorLaunchEvent{
		Timestamp:  instance.StartTime,
		NodeID:     instance.NodeID,
		UserID:     instance.UserID,
		DoorID:     instance.DoorID,
		InstanceID: instance.ID,
		Success:    instance.Status == InstanceStatusFinished,
		Duration:   duration,
		ExitCode:   instance.ErrorCount, // Using error count as proxy for exit code
	}
	
	di.doorHistory = append(di.doorHistory, event)
	
	// Update door statistics
	stats, err := di.doorManager.GetDoorStatistics(instance.DoorID)
	if err != nil {
		// Create new statistics
		stats = &DoorStatistics{
			UserRatings: make([]UserRating, 0),
			DailyStats:  make([]DailyStats, 0),
		}
	}
	
	// Update statistics
	stats.TotalRuns++
	stats.TotalTime += duration
	stats.LastRun = time.Now()
	
	if stats.TotalRuns > 0 {
		stats.AverageTime = stats.TotalTime / time.Duration(stats.TotalRuns)
	}
	
	if !event.Success {
		stats.CrashCount++
		stats.LastCrash = time.Now()
	}
	
	// Update daily stats
	today := time.Now().Truncate(24 * time.Hour)
	var todayStats *DailyStats
	
	for i := range stats.DailyStats {
		if stats.DailyStats[i].Date.Equal(today) {
			todayStats = &stats.DailyStats[i]
			break
		}
	}
	
	if todayStats == nil {
		stats.DailyStats = append(stats.DailyStats, DailyStats{
			Date: today,
		})
		todayStats = &stats.DailyStats[len(stats.DailyStats)-1]
	}
	
	todayStats.Runs++
	todayStats.TotalTime += duration
	if !event.Success {
		// todayStats.Crashes++ // Field doesn't exist in DailyStats, would need to add
	}
	
	// Save updated statistics
	di.doorManager.UpdateDoorStatistics(instance.DoorID, stats)
}

func (di *DoorIntegrator) forceStopDoor(nodeID int, doorID string) {
	// Get instances for the node
	instances, err := di.doorManager.GetNodeDoorInstances(nodeID)
	if err != nil {
		return
	}
	
	// Find and terminate instances for this door
	for _, instance := range instances {
		if instance.DoorID == doorID {
			di.doorManager.TerminateDoorInstance(instance.ID)
		}
	}
	
	// Clean up tracking
	delete(di.activeDoors, nodeID)
}

func (di *DoorIntegrator) emitEvent(event IntegrationEvent) {
	// Call registered event handlers
	eventType := event.Type.String()
	if handlers, exists := di.eventHandlers[eventType]; exists {
		for _, handler := range handlers {
			go func(h EventHandler, e IntegrationEvent) {
				h(e)
			}(handler, event)
		}
	}
	
	// Call "all" event handlers
	if handlers, exists := di.eventHandlers["*"]; exists {
		for _, handler := range handlers {
			go func(h EventHandler, e IntegrationEvent) {
				h(e)
			}(handler, event)
		}
	}
}

func (di *DoorIntegrator) registerDefaultHandlers() {
	// Register default logging handler
	di.RegisterEventHandler("*", func(event IntegrationEvent) error {
		if di.config.LogDoorActivity {
			fmt.Printf("[DOOR] %s: Node=%d User=%d Door=%s\n", 
				event.Type.String(), event.NodeID, event.UserID, event.DoorID)
		}
		return nil
	})
	
	// Register statistics update handler
	di.RegisterEventHandler(EventStatisticsUpdate.String(), func(event IntegrationEvent) error {
		if di.config.EnableStatsIntegration {
			// Update system statistics
			// TODO: Implement statistics integration
		}
		return nil
	})
	
	// Register node status handler
	di.RegisterEventHandler(EventNodeStatusChanged.String(), func(event IntegrationEvent) error {
		if di.config.EnableNodeIntegration && di.nodeManager != nil {
			// Handle node status changes
			// TODO: Implement node status handling
		}
		return nil
	})
}

// GetDoorHistory returns the door launch history
func (di *DoorIntegrator) GetDoorHistory() []DoorLaunchEvent {
	return di.doorHistory
}

// GetDoorStats returns door usage statistics
func (di *DoorIntegrator) GetDoorStats() map[string]interface{} {
	stats := make(map[string]interface{})
	
	// Count active doors
	activeDoorCount := len(di.activeDoors)
	stats["active_doors"] = activeDoorCount
	
	// Count total launches
	stats["total_launches"] = len(di.doorHistory)
	
	// Calculate success rate
	successCount := 0
	for _, event := range di.doorHistory {
		if event.Success {
			successCount++
		}
	}
	
	if len(di.doorHistory) > 0 {
		stats["success_rate"] = float64(successCount) / float64(len(di.doorHistory))
	} else {
		stats["success_rate"] = 0.0
	}
	
	// Most popular doors
	doorCounts := make(map[string]int)
	for _, event := range di.doorHistory {
		doorCounts[event.DoorID]++
	}
	stats["door_usage"] = doorCounts
	
	return stats
}

// CreateNodeIntegrationHandler creates a handler for node manager integration
func (di *DoorIntegrator) CreateNodeIntegrationHandler() nodes.NodeEventListener {
	return &nodeEventHandler{integrator: di}
}

// nodeEventHandler implements nodes.NodeEventListener
type nodeEventHandler struct {
	integrator *DoorIntegrator
}

func (neh *nodeEventHandler) OnNodeEvent(event nodes.NodeEvent) error {
	// Handle node events and integrate with door system
	switch event.Type {
	case "connect":
		// Node connected, register with coordinator if needed
		if neh.integrator.coordinator != nil && neh.integrator.config.AutoNodeRegistration {
			capabilities := NodeCapabilities{
				SupportedDoors: []string{"*"},
				MaxMemory:      1024 * 1024 * 1024,
				MaxCPU:         80.0,
				OSType:         "linux",
				Architecture:   "x86_64",
				NetworkAccess:  true,
				LocalStorage:   10 * 1024 * 1024 * 1024,
			}
			neh.integrator.coordinator.RegisterNode(event.NodeID, capabilities)
		}
		
	case "disconnect":
		// Node disconnected, clean up any active doors
		if doorID, exists := neh.integrator.activeDoors[event.NodeID]; exists {
			neh.integrator.forceStopDoor(event.NodeID, doorID)
			
			// Emit node status changed event
			neh.integrator.emitEvent(IntegrationEvent{
				Type:      EventNodeStatusChanged,
				Timestamp: time.Now(),
				NodeID:    event.NodeID,
				DoorID:    doorID,
				Data:      "node_disconnected",
			})
		}
	}
	
	return nil
}

// Integration utilities

// LaunchDoorFromMenu launches a door from a menu context
func (di *DoorIntegrator) LaunchDoorFromMenu(doorID string, nodeID int, userID int, menuContext map[string]interface{}) (*DoorInstance, error) {
	// Add menu context to the launch
	instance, err := di.LaunchDoorForUser(doorID, nodeID, userID)
	if err != nil {
		return nil, err
	}
	
	// Store menu context in instance metadata if available
	if monitoredInstance, exists := di.monitor.instances[instance.ID]; exists {
		for key, value := range menuContext {
			monitoredInstance.Metadata[key] = value
		}
	}
	
	return instance, nil
}

// GetDoorMenuItems returns door menu items for TUI integration
func (di *DoorIntegrator) GetDoorMenuItems() ([]tui.MenuItem, error) {
	doors, err := di.doorManager.GetAllDoorConfigs()
	if err != nil {
		return nil, err
	}
	
	menuItems := make([]tui.MenuItem, 0, len(doors))
	
	for _, door := range doors {
		if door.Enabled {
			menuItem := tui.MenuItem{
				Label:   door.Name,
				Key:     rune(door.Name[0]), // Use first character as hotkey
				Action:  func() tea.Cmd { return nil }, // Placeholder action
				Enabled: true,
			}
			menuItems = append(menuItems, menuItem)
		}
	}
	
	return menuItems, nil
}

// ValidateIntegration validates the integration setup
func (di *DoorIntegrator) ValidateIntegration() []string {
	var issues []string
	
	if di.doorManager == nil {
		issues = append(issues, "Door manager not initialized")
	}
	
	if di.config.EnableNodeIntegration && di.nodeManager == nil {
		issues = append(issues, "Node integration enabled but node manager not available")
	}
	
	if di.config.EnableUserIntegration && di.userManager == nil {
		issues = append(issues, "User integration enabled but user manager not available")
	}
	
	if di.coordinator == nil {
		issues = append(issues, "Multi-node coordinator not available")
	}
	
	if di.monitor == nil {
		issues = append(issues, "Door monitoring not available")
	}
	
	return issues
}