package doors

import (
	"fmt"
	"sync"
	"time"
	"errors"
	"sort"
	"math/rand"
	"encoding/json"
	"path/filepath"
	"os"
	
	"github.com/stlalpha/vision3/internal/configtool/multinode"
	"github.com/stlalpha/vision3/internal/user"
	"github.com/stlalpha/vision3/internal/session"
)

var (
	ErrDoorBusy           = errors.New("door is busy")
	ErrQueueFull          = errors.New("queue is full")
	ErrUserInQueue        = errors.New("user already in queue")
	ErrUserNotInQueue     = errors.New("user not in queue")
	ErrNodeNotAvailable   = errors.New("node not available")
	ErrMaxInstancesReached = errors.New("maximum instances reached")
	ErrNoAvailableNodes   = errors.New("no available nodes")
)

// MultiNodeCoordinator manages door access across multiple nodes
type MultiNodeCoordinator struct {
	doors           map[string]*DoorConfiguration
	instances       map[string]*DoorInstance
	queues          map[string]*DoorQueue
	nodeStates      map[int]*NodeDoorState
	resourceManager *ResourceManager
	config          *MultiNodeConfig
	mu              sync.RWMutex
	eventChan       chan DoorEvent
	stopChan        chan bool
	dataPath        string
}

// MultiNodeConfig contains configuration for multi-node coordination
type MultiNodeConfig struct {
	MaxNodes            int           `json:"max_nodes"`             // Maximum number of nodes
	QueueTimeout        time.Duration `json:"queue_timeout"`         // Queue timeout
	InstanceTimeout     time.Duration `json:"instance_timeout"`      // Instance timeout
	NodeRotationEnabled bool          `json:"node_rotation_enabled"` // Enable node rotation
	LoadBalancing       bool          `json:"load_balancing"`        // Enable load balancing
	FailoverEnabled     bool          `json:"failover_enabled"`      // Enable failover
	SyncInterval        time.Duration `json:"sync_interval"`         // Sync interval
	DataPath            string        `json:"data_path"`             // Data persistence path
	BackupInterval      time.Duration `json:"backup_interval"`       // Backup interval
	CleanupInterval     time.Duration `json:"cleanup_interval"`      // Cleanup interval
}

// NodeDoorState represents the door state for a specific node
type NodeDoorState struct {
	NodeID           int                        `json:"node_id"`            // Node ID
	Status           uint8                      `json:"status"`             // Node status
	ActiveInstances  map[string]*DoorInstance   `json:"active_instances"`   // Active door instances
	MaxInstances     int                        `json:"max_instances"`      // Maximum instances per node
	LoadScore        float64                    `json:"load_score"`         // Current load score
	LastActivity     time.Time                  `json:"last_activity"`      // Last activity time
	Capabilities     NodeCapabilities           `json:"capabilities"`       // Node capabilities
	Resources        map[string]*NodeResource   `json:"resources"`          // Node resources
	Preferences      NodePreferences            `json:"preferences"`        // Node preferences
	Maintenance      bool                       `json:"maintenance"`        // Maintenance mode
}

// NodeCapabilities represents what a node can do
type NodeCapabilities struct {
	SupportedDoors   []string          `json:"supported_doors"`    // Supported door types
	MaxMemory        int64             `json:"max_memory"`         // Maximum memory usage
	MaxCPU           float64           `json:"max_cpu"`            // Maximum CPU usage
	SpecialFeatures  []string          `json:"special_features"`   // Special features
	OSType           string            `json:"os_type"`            // Operating system
	Architecture     string            `json:"architecture"`       // CPU architecture
	NetworkAccess    bool              `json:"network_access"`     // Network access available
	LocalStorage     int64             `json:"local_storage"`      // Local storage available
}

// NodeResource represents a resource available on a node
type NodeResource struct {
	ID          string        `json:"id"`           // Resource ID
	Type        ResourceType  `json:"type"`         // Resource type
	Available   bool          `json:"available"`    // Resource available
	InUse       bool          `json:"in_use"`       // Resource in use
	ReservedBy  string        `json:"reserved_by"`  // Reserved by instance
	LastUsed    time.Time     `json:"last_used"`    // Last used time
	UsageCount  int           `json:"usage_count"`  // Usage count
}

// NodePreferences represents node preferences for door assignment
type NodePreferences struct {
	PreferredDoors   []string          `json:"preferred_doors"`    // Preferred door types
	AvoidDoors       []string          `json:"avoid_doors"`        // Doors to avoid
	LoadThreshold    float64           `json:"load_threshold"`     // Load threshold
	Priority         int               `json:"priority"`           // Node priority
	TimeWindows      []TimeSlot        `json:"time_windows"`       // Available time windows
	UserLimits       map[string]int    `json:"user_limits"`        // User limits per door
}

// DoorEvent represents events in the door system
type DoorEvent struct {
	Type       DoorEventType `json:"type"`        // Event type
	Timestamp  time.Time     `json:"timestamp"`   // Event timestamp
	NodeID     int           `json:"node_id"`     // Node ID
	DoorID     string        `json:"door_id"`     // Door ID
	InstanceID string        `json:"instance_id"` // Instance ID
	UserID     int           `json:"user_id"`     // User ID
	Data       interface{}   `json:"data"`        // Event data
	Severity   AlertSeverity `json:"severity"`    // Event severity
}

// DoorEventType represents the type of door event
type DoorEventType int

const (
	EventDoorLaunch DoorEventType = iota
	EventDoorFinish
	EventDoorCrash
	EventDoorTimeout
	EventQueueAdd
	EventQueueRemove
	EventQueueTimeout
	EventNodeAvailable
	EventNodeUnavailable
	EventResourceLock
	EventResourceRelease
	EventLoadBalance
	EventFailover
)

func (det DoorEventType) String() string {
	switch det {
	case EventDoorLaunch:
		return "Door Launch"
	case EventDoorFinish:
		return "Door Finish"
	case EventDoorCrash:
		return "Door Crash"
	case EventDoorTimeout:
		return "Door Timeout"
	case EventQueueAdd:
		return "Queue Add"
	case EventQueueRemove:
		return "Queue Remove"
	case EventQueueTimeout:
		return "Queue Timeout"
	case EventNodeAvailable:
		return "Node Available"
	case EventNodeUnavailable:
		return "Node Unavailable"
	case EventResourceLock:
		return "Resource Lock"
	case EventResourceRelease:
		return "Resource Release"
	case EventLoadBalance:
		return "Load Balance"
	case EventFailover:
		return "Failover"
	default:
		return "Unknown"
	}
}

// NewMultiNodeCoordinator creates a new multi-node coordinator
func NewMultiNodeCoordinator(config *MultiNodeConfig, resourceManager *ResourceManager) *MultiNodeCoordinator {
	if config == nil {
		config = &MultiNodeConfig{
			MaxNodes:            255,
			QueueTimeout:        time.Minute * 5,
			InstanceTimeout:     time.Hour * 2,
			NodeRotationEnabled: true,
			LoadBalancing:       true,
			FailoverEnabled:     true,
			SyncInterval:        time.Second * 30,
			DataPath:            "/tmp/vision3/multinode",
			BackupInterval:      time.Hour,
			CleanupInterval:     time.Minute * 10,
		}
	}
	
	coordinator := &MultiNodeCoordinator{
		doors:           make(map[string]*DoorConfiguration),
		instances:       make(map[string]*DoorInstance),
		queues:          make(map[string]*DoorQueue),
		nodeStates:      make(map[int]*NodeDoorState),
		resourceManager: resourceManager,
		config:          config,
		eventChan:       make(chan DoorEvent, 100),
		stopChan:        make(chan bool),
		dataPath:        config.DataPath,
	}
	
	// Ensure data directory exists
	os.MkdirAll(coordinator.dataPath, 0755)
	
	// Start background routines
	go coordinator.coordinationLoop()
	go coordinator.cleanupLoop()
	go coordinator.syncLoop()
	
	return coordinator
}

// RegisterDoor registers a door configuration
func (mnc *MultiNodeCoordinator) RegisterDoor(door *DoorConfiguration) error {
	mnc.mu.Lock()
	defer mnc.mu.Unlock()
	
	mnc.doors[door.ID] = door
	
	// Create queue if it doesn't exist
	if _, exists := mnc.queues[door.ID]; !exists {
		mnc.queues[door.ID] = &DoorQueue{
			DoorID:      door.ID,
			Queue:       make([]DoorQueueEntry, 0),
			MaxLength:   50, // Default max queue length
			TimeoutMins: 5,  // Default timeout
			Enabled:     true,
		}
	}
	
	return nil
}

// RegisterNode registers a node for door coordination
func (mnc *MultiNodeCoordinator) RegisterNode(nodeID int, capabilities NodeCapabilities) error {
	mnc.mu.Lock()
	defer mnc.mu.Unlock()
	
	if nodeID <= 0 || nodeID > mnc.config.MaxNodes {
		return errors.New("invalid node ID")
	}
	
	nodeState := &NodeDoorState{
		NodeID:          nodeID,
		Status:          multinode.NodeStatusWaiting,
		ActiveInstances: make(map[string]*DoorInstance),
		MaxInstances:    10, // Default max instances
		LoadScore:       0.0,
		LastActivity:    time.Now(),
		Capabilities:    capabilities,
		Resources:       make(map[string]*NodeResource),
		Preferences: NodePreferences{
			LoadThreshold: 0.8,
			Priority:      5,
			UserLimits:    make(map[string]int),
		},
		Maintenance:     false,
	}
	
	mnc.nodeStates[nodeID] = nodeState
	
	// Emit event
	mnc.emitEvent(DoorEvent{
		Type:      EventNodeAvailable,
		Timestamp: time.Now(),
		NodeID:    nodeID,
		Severity:  SeverityLow,
	})
	
	return nil
}

// RequestDoorAccess requests access to a door for a user
func (mnc *MultiNodeCoordinator) RequestDoorAccess(doorID string, nodeID int, userID int, user *user.User, session *session.BbsSession) (*DoorInstance, error) {
	mnc.mu.Lock()
	defer mnc.mu.Unlock()
	
	door, exists := mnc.doors[doorID]
	if !exists {
		return nil, fmt.Errorf("door not found: %s", doorID)
	}
	
	// Check if door is enabled
	if !door.Enabled {
		return nil, errors.New("door is disabled")
	}
	
	// Check user access permissions
	if err := mnc.checkUserAccess(door, user); err != nil {
		return nil, err
	}
	
	// Check if user is already in this door
	if mnc.isUserInDoor(doorID, userID) {
		return nil, errors.New("user already in door")
	}
	
	// Try to find an available node for the door
	targetNodeID, err := mnc.selectBestNode(doorID, nodeID, user)
	if err != nil {
		// Add to queue if no nodes available
		return nil, mnc.addToQueue(doorID, nodeID, userID)
	}
	
	// Launch door instance
	instance, err := mnc.launchDoorInstance(door, targetNodeID, userID, user, session)
	if err != nil {
		return nil, err
	}
	
	return instance, nil
}

// ReleaseDoorAccess releases a door instance
func (mnc *MultiNodeCoordinator) ReleaseDoorAccess(instanceID string) error {
	mnc.mu.Lock()
	defer mnc.mu.Unlock()
	
	instance, exists := mnc.instances[instanceID]
	if !exists {
		return errors.New("instance not found")
	}
	
	// Update instance status
	instance.Status = InstanceStatusFinishing
	
	// Release resources
	if mnc.resourceManager != nil {
		for _, lockID := range instance.ResourceLocks {
			mnc.resourceManager.ReleaseLock(lockID)
		}
	}
	
	// Remove from node state
	if nodeState, exists := mnc.nodeStates[instance.NodeID]; exists {
		delete(nodeState.ActiveInstances, instanceID)
		nodeState.LoadScore = mnc.calculateNodeLoad(instance.NodeID)
		nodeState.LastActivity = time.Now()
	}
	
	// Remove from instances
	delete(mnc.instances, instanceID)
	
	// Process queue for this door
	go mnc.processQueue(instance.DoorID)
	
	// Emit event
	mnc.emitEvent(DoorEvent{
		Type:       EventDoorFinish,
		Timestamp:  time.Now(),
		NodeID:     instance.NodeID,
		DoorID:     instance.DoorID,
		InstanceID: instanceID,
		UserID:     instance.UserID,
		Severity:   SeverityLow,
	})
	
	return nil
}

// AddToQueue adds a user to the door queue
func (mnc *MultiNodeCoordinator) addToQueue(doorID string, nodeID int, userID int) error {
	queue, exists := mnc.queues[doorID]
	if !exists {
		return errors.New("queue not found")
	}
	
	queue.mu.Lock()
	defer queue.mu.Unlock()
	
	if !queue.Enabled {
		return errors.New("queue is disabled")
	}
	
	if len(queue.Queue) >= queue.MaxLength {
		return ErrQueueFull
	}
	
	// Check if user already in queue
	for _, entry := range queue.Queue {
		if entry.UserID == userID {
			return ErrUserInQueue
		}
	}
	
	entry := DoorQueueEntry{
		UserID:    userID,
		NodeID:    nodeID,
		QueueTime: time.Now(),
		Priority:  5, // Default priority
		Notified:  false,
	}
	
	queue.Queue = append(queue.Queue, entry)
	
	// Sort queue by priority and time
	sort.Slice(queue.Queue, func(i, j int) bool {
		if queue.Queue[i].Priority != queue.Queue[j].Priority {
			return queue.Queue[i].Priority > queue.Queue[j].Priority
		}
		return queue.Queue[i].QueueTime.Before(queue.Queue[j].QueueTime)
	})
	
	// Emit event
	mnc.emitEvent(DoorEvent{
		Type:      EventQueueAdd,
		Timestamp: time.Now(),
		NodeID:    nodeID,
		DoorID:    doorID,
		UserID:    userID,
		Severity:  SeverityLow,
	})
	
	return nil
}

// RemoveFromQueue removes a user from the door queue
func (mnc *MultiNodeCoordinator) RemoveFromQueue(doorID string, userID int) error {
	mnc.mu.Lock()
	defer mnc.mu.Unlock()
	
	queue, exists := mnc.queues[doorID]
	if !exists {
		return errors.New("queue not found")
	}
	
	queue.mu.Lock()
	defer queue.mu.Unlock()
	
	for i, entry := range queue.Queue {
		if entry.UserID == userID {
			queue.Queue = append(queue.Queue[:i], queue.Queue[i+1:]...)
			
			// Emit event
			mnc.emitEvent(DoorEvent{
				Type:      EventQueueRemove,
				Timestamp: time.Now(),
				NodeID:    entry.NodeID,
				DoorID:    doorID,
				UserID:    userID,
				Severity:  SeverityLow,
			})
			
			return nil
		}
	}
	
	return ErrUserNotInQueue
}

// GetQueuePosition returns the position of a user in the queue
func (mnc *MultiNodeCoordinator) GetQueuePosition(doorID string, userID int) (int, error) {
	mnc.mu.RLock()
	defer mnc.mu.RUnlock()
	
	queue, exists := mnc.queues[doorID]
	if !exists {
		return -1, errors.New("queue not found")
	}
	
	queue.mu.RLock()
	defer queue.mu.RUnlock()
	
	for i, entry := range queue.Queue {
		if entry.UserID == userID {
			return i + 1, nil // 1-based position
		}
	}
	
	return -1, ErrUserNotInQueue
}

// selectBestNode selects the best node for a door instance
func (mnc *MultiNodeCoordinator) selectBestNode(doorID string, preferredNodeID int, user *user.User) (int, error) {
	door := mnc.doors[doorID]
	
	// Get available nodes
	availableNodes := mnc.getAvailableNodes(doorID)
	if len(availableNodes) == 0 {
		return 0, ErrNoAvailableNodes
	}
	
	// Check if preferred node is available
	if preferredNodeID > 0 {
		for _, nodeID := range availableNodes {
			if nodeID == preferredNodeID {
				return preferredNodeID, nil
			}
		}
	}
	
	// Select based on door's multi-node type
	switch door.MultiNodeType {
	case MultiNodeSingle:
		// Single user - find least loaded node
		return mnc.selectLeastLoadedNode(availableNodes), nil
		
	case MultiNodeShared:
		// Shared data - prefer nodes already running this door
		if nodeID := mnc.findNodeWithDoor(doorID, availableNodes); nodeID > 0 {
			return nodeID, nil
		}
		return mnc.selectLeastLoadedNode(availableNodes), nil
		
	case MultiNodeExclusive:
		// Exclusive instances - ensure max instances not exceeded
		for _, nodeID := range availableNodes {
			if mnc.getNodeInstanceCount(nodeID, doorID) < door.MaxInstances {
				return nodeID, nil
			}
		}
		return 0, ErrMaxInstancesReached
		
	case MultiNodeCoop, MultiNodeCompetitive:
		// Multi-user doors - use load balancing
		if mnc.config.LoadBalancing {
			return mnc.selectLoadBalancedNode(availableNodes, doorID), nil
		}
		return mnc.selectLeastLoadedNode(availableNodes), nil
	}
	
	return mnc.selectLeastLoadedNode(availableNodes), nil
}

// getAvailableNodes returns nodes available for a specific door
func (mnc *MultiNodeCoordinator) getAvailableNodes(doorID string) []int {
	var available []int
	
	for nodeID, nodeState := range mnc.nodeStates {
		if mnc.isNodeAvailableForDoor(nodeState, doorID) {
			available = append(available, nodeID)
		}
	}
	
	return available
}

// isNodeAvailableForDoor checks if a node is available for a door
func (mnc *MultiNodeCoordinator) isNodeAvailableForDoor(nodeState *NodeDoorState, doorID string) bool {
	// Check node status
	if nodeState.Status != multinode.NodeStatusWaiting && 
	   nodeState.Status != multinode.NodeStatusLogin {
		return false
	}
	
	// Check maintenance mode
	if nodeState.Maintenance {
		return false
	}
	
	// Check load threshold
	if nodeState.LoadScore > nodeState.Preferences.LoadThreshold {
		return false
	}
	
	// Check supported doors
	if len(nodeState.Capabilities.SupportedDoors) > 0 {
		supported := false
		for _, supportedDoor := range nodeState.Capabilities.SupportedDoors {
			if supportedDoor == doorID || supportedDoor == "*" {
				supported = true
				break
			}
		}
		if !supported {
			return false
		}
	}
	
	// Check avoid list
	for _, avoidDoor := range nodeState.Preferences.AvoidDoors {
		if avoidDoor == doorID {
			return false
		}
	}
	
	// Check maximum instances
	if len(nodeState.ActiveInstances) >= nodeState.MaxInstances {
		return false
	}
	
	return true
}

// selectLeastLoadedNode selects the node with the lowest load
func (mnc *MultiNodeCoordinator) selectLeastLoadedNode(nodes []int) int {
	if len(nodes) == 0 {
		return 0
	}
	
	minLoad := float64(1.0)
	selectedNode := nodes[0]
	
	for _, nodeID := range nodes {
		if nodeState, exists := mnc.nodeStates[nodeID]; exists {
			if nodeState.LoadScore < minLoad {
				minLoad = nodeState.LoadScore
				selectedNode = nodeID
			}
		}
	}
	
	return selectedNode
}

// selectLoadBalancedNode selects a node using load balancing
func (mnc *MultiNodeCoordinator) selectLoadBalancedNode(nodes []int, doorID string) int {
	if len(nodes) == 0 {
		return 0
	}
	
	// Weight nodes by inverse load score
	weights := make([]float64, len(nodes))
	totalWeight := 0.0
	
	for i, nodeID := range nodes {
		if nodeState, exists := mnc.nodeStates[nodeID]; exists {
			// Inverse weight (lower load = higher weight)
			weight := 1.0 - nodeState.LoadScore
			if weight < 0.1 {
				weight = 0.1 // Minimum weight
			}
			
			// Bonus for preferred doors
			for _, preferred := range nodeState.Preferences.PreferredDoors {
				if preferred == doorID {
					weight *= 1.5
					break
				}
			}
			
			weights[i] = weight
			totalWeight += weight
		}
	}
	
	// Select randomly based on weights
	r := rand.Float64() * totalWeight
	cumulative := 0.0
	
	for i, weight := range weights {
		cumulative += weight
		if r <= cumulative {
			return nodes[i]
		}
	}
	
	return nodes[0] // Fallback
}

// findNodeWithDoor finds a node already running a specific door
func (mnc *MultiNodeCoordinator) findNodeWithDoor(doorID string, availableNodes []int) int {
	for _, nodeID := range availableNodes {
		if nodeState, exists := mnc.nodeStates[nodeID]; exists {
			for _, instance := range nodeState.ActiveInstances {
				if instance.DoorID == doorID {
					return nodeID
				}
			}
		}
	}
	return 0
}

// getNodeInstanceCount returns the number of instances of a door on a node
func (mnc *MultiNodeCoordinator) getNodeInstanceCount(nodeID int, doorID string) int {
	nodeState, exists := mnc.nodeStates[nodeID]
	if !exists {
		return 0
	}
	
	count := 0
	for _, instance := range nodeState.ActiveInstances {
		if instance.DoorID == doorID {
			count++
		}
	}
	
	return count
}

// calculateNodeLoad calculates the current load score for a node
func (mnc *MultiNodeCoordinator) calculateNodeLoad(nodeID int) float64 {
	nodeState, exists := mnc.nodeStates[nodeID]
	if !exists {
		return 1.0 // Maximum load if node not found
	}
	
	// Base load from active instances
	instanceLoad := float64(len(nodeState.ActiveInstances)) / float64(nodeState.MaxInstances)
	
	// TODO: Add CPU, memory, and other resource metrics
	// For now, just use instance count
	
	return instanceLoad
}

// checkUserAccess checks if a user has access to a door
func (mnc *MultiNodeCoordinator) checkUserAccess(door *DoorConfiguration, user *user.User) error {
	// Check access level
	if user.AccessLevel < door.MinimumAccessLevel {
		return errors.New("insufficient access level")
	}
	
	// Check required flags
	for _, requiredFlag := range door.RequiredFlags {
		if !user.HasFlag(requiredFlag) {
			return fmt.Errorf("missing required flag: %s", requiredFlag)
		}
	}
	
	// Check forbidden flags
	for _, forbiddenFlag := range door.ForbiddenFlags {
		if user.HasFlag(forbiddenFlag) {
			return fmt.Errorf("user has forbidden flag: %s", forbiddenFlag)
		}
	}
	
	// Check time limits
	if door.TimeLimit > 0 && user.TimeLeft < door.TimeLimit {
		return errors.New("insufficient time remaining")
	}
	
	return nil
}

// isUserInDoor checks if a user is already in a door
func (mnc *MultiNodeCoordinator) isUserInDoor(doorID string, userID int) bool {
	for _, instance := range mnc.instances {
		if instance.DoorID == doorID && instance.UserID == userID {
			return true
		}
	}
	return false
}

// launchDoorInstance launches a new door instance
func (mnc *MultiNodeCoordinator) launchDoorInstance(door *DoorConfiguration, nodeID int, userID int, user *user.User, session *session.BbsSession) (*DoorInstance, error) {
	// Generate instance ID
	instanceID := fmt.Sprintf("%s_%d_%d_%d", door.ID, nodeID, userID, time.Now().Unix())
	
	// Create instance
	instance := &DoorInstance{
		ID:            instanceID,
		DoorID:        door.ID,
		NodeID:        nodeID,
		UserID:        userID,
		StartTime:     time.Now(),
		Status:        InstanceStatusStarting,
		ResourceLocks: make([]string, 0),
		LastActivity:  time.Now(),
	}
	
	// Acquire resources if needed
	if err := mnc.acquireInstanceResources(instance, door); err != nil {
		return nil, err
	}
	
	// Add to instances map
	mnc.instances[instanceID] = instance
	
	// Add to node state
	if nodeState, exists := mnc.nodeStates[nodeID]; exists {
		nodeState.ActiveInstances[instanceID] = instance
		nodeState.LoadScore = mnc.calculateNodeLoad(nodeID)
		nodeState.LastActivity = time.Now()
	}
	
	// Update instance status
	instance.Status = InstanceStatusRunning
	
	// Emit event
	mnc.emitEvent(DoorEvent{
		Type:       EventDoorLaunch,
		Timestamp:  time.Now(),
		NodeID:     nodeID,
		DoorID:     door.ID,
		InstanceID: instanceID,
		UserID:     userID,
		Severity:   SeverityLow,
	})
	
	return instance, nil
}

// acquireInstanceResources acquires resources for a door instance
func (mnc *MultiNodeCoordinator) acquireInstanceResources(instance *DoorInstance, door *DoorConfiguration) error {
	if mnc.resourceManager == nil {
		return nil // No resource management
	}
	
	// Acquire shared resources
	for _, resourcePath := range door.SharedResources {
		lock, err := mnc.resourceManager.AcquireLock(
			resourcePath, door.ID, instance.ID, 
			instance.NodeID, instance.UserID, 
			LockModeShared, 0,
		)
		if err != nil {
			// Clean up already acquired locks
			mnc.releaseInstanceResources(instance)
			return err
		}
		instance.ResourceLocks = append(instance.ResourceLocks, lock.ID)
	}
	
	// Acquire exclusive resources
	for _, resourcePath := range door.ExclusiveResources {
		lock, err := mnc.resourceManager.AcquireLock(
			resourcePath, door.ID, instance.ID,
			instance.NodeID, instance.UserID,
			LockModeExclusive, 0,
		)
		if err != nil {
			// Clean up already acquired locks
			mnc.releaseInstanceResources(instance)
			return err
		}
		instance.ResourceLocks = append(instance.ResourceLocks, lock.ID)
	}
	
	return nil
}

// releaseInstanceResources releases resources for a door instance
func (mnc *MultiNodeCoordinator) releaseInstanceResources(instance *DoorInstance) {
	if mnc.resourceManager == nil {
		return
	}
	
	for _, lockID := range instance.ResourceLocks {
		mnc.resourceManager.ReleaseLock(lockID)
	}
	
	instance.ResourceLocks = make([]string, 0)
}

// processQueue processes the queue for a door
func (mnc *MultiNodeCoordinator) processQueue(doorID string) {
	mnc.mu.Lock()
	defer mnc.mu.Unlock()
	
	queue, exists := mnc.queues[doorID]
	if !exists || !queue.Enabled {
		return
	}
	
	queue.mu.Lock()
	defer queue.mu.Unlock()
	
	if len(queue.Queue) == 0 {
		return
	}
	
	// Try to process queue entries
	processed := 0
	for i := 0; i < len(queue.Queue) && processed < 5; i++ { // Limit processing
		entry := queue.Queue[i]
		
		// Check if we can launch this entry
		_, err := mnc.selectBestNode(doorID, entry.NodeID, nil) // TODO: Get user
		if err == nil {
			// Remove from queue and process
			queue.Queue = append(queue.Queue[:i], queue.Queue[i+1:]...)
			i-- // Adjust index
			processed++
			
			// TODO: Notify user that door is available
			// This would typically involve sending a message to the user's node
		}
	}
}

// emitEvent emits a door event
func (mnc *MultiNodeCoordinator) emitEvent(event DoorEvent) {
	select {
	case mnc.eventChan <- event:
		// Event sent
	default:
		// Channel full, drop event
	}
}

// Background loops

func (mnc *MultiNodeCoordinator) coordinationLoop() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			mnc.performCoordination()
		case <-mnc.stopChan:
			return
		}
	}
}

func (mnc *MultiNodeCoordinator) cleanupLoop() {
	ticker := time.NewTicker(mnc.config.CleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			mnc.performCleanup()
		case <-mnc.stopChan:
			return
		}
	}
}

func (mnc *MultiNodeCoordinator) syncLoop() {
	ticker := time.NewTicker(mnc.config.SyncInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			mnc.performSync()
		case <-mnc.stopChan:
			return
		}
	}
}

func (mnc *MultiNodeCoordinator) performCoordination() {
	mnc.mu.Lock()
	defer mnc.mu.Unlock()
	
	// Update node load scores
	for nodeID := range mnc.nodeStates {
		mnc.nodeStates[nodeID].LoadScore = mnc.calculateNodeLoad(nodeID)
	}
	
	// Process queues
	for doorID := range mnc.queues {
		go mnc.processQueue(doorID)
	}
}

func (mnc *MultiNodeCoordinator) performCleanup() {
	mnc.mu.Lock()
	defer mnc.mu.Unlock()
	
	now := time.Now()
	
	// Clean up timed out instances
	for instanceID, instance := range mnc.instances {
		if now.Sub(instance.StartTime) > mnc.config.InstanceTimeout {
			// Force terminate instance
			mnc.forceTerminateInstance(instanceID)
		}
	}
	
	// Clean up queue timeouts
	for doorID, queue := range mnc.queues {
		queue.mu.Lock()
		newQueue := make([]DoorQueueEntry, 0)
		for _, entry := range queue.Queue {
			if now.Sub(entry.QueueTime) < time.Duration(queue.TimeoutMins)*time.Minute {
				newQueue = append(newQueue, entry)
			} else {
				// Emit timeout event
				mnc.emitEvent(DoorEvent{
					Type:      EventQueueTimeout,
					Timestamp: now,
					NodeID:    entry.NodeID,
					DoorID:    doorID,
					UserID:    entry.UserID,
					Severity:  SeverityMedium,
				})
			}
		}
		queue.Queue = newQueue
		queue.mu.Unlock()
	}
}

func (mnc *MultiNodeCoordinator) performSync() {
	// Sync state to disk
	mnc.saveState()
}

func (mnc *MultiNodeCoordinator) forceTerminateInstance(instanceID string) {
	instance, exists := mnc.instances[instanceID]
	if !exists {
		return
	}
	
	// Release resources
	mnc.releaseInstanceResources(instance)
	
	// Remove from node state
	if nodeState, exists := mnc.nodeStates[instance.NodeID]; exists {
		delete(nodeState.ActiveInstances, instanceID)
		nodeState.LoadScore = mnc.calculateNodeLoad(instance.NodeID)
	}
	
	// Remove from instances
	delete(mnc.instances, instanceID)
	
	// Emit event
	mnc.emitEvent(DoorEvent{
		Type:       EventDoorTimeout,
		Timestamp:  time.Now(),
		NodeID:     instance.NodeID,
		DoorID:     instance.DoorID,
		InstanceID: instanceID,
		UserID:     instance.UserID,
		Severity:   SeverityHigh,
	})
}

// saveState saves the coordinator state to disk
func (mnc *MultiNodeCoordinator) saveState() error {
	mnc.mu.RLock()
	defer mnc.mu.RUnlock()
	
	state := map[string]interface{}{
		"instances":   mnc.instances,
		"queues":      mnc.queues,
		"nodeStates":  mnc.nodeStates,
		"timestamp":   time.Now(),
	}
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	
	statePath := filepath.Join(mnc.dataPath, "coordinator_state.json")
	return os.WriteFile(statePath, data, 0644)
}

// loadState loads the coordinator state from disk
func (mnc *MultiNodeCoordinator) loadState() error {
	statePath := filepath.Join(mnc.dataPath, "coordinator_state.json")
	
	data, err := os.ReadFile(statePath)
	if err != nil {
		return err // Not a critical error
	}
	
	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	
	// TODO: Restore state from loaded data
	// This is complex because we need to handle type conversions
	
	return nil
}

// Stop stops the multi-node coordinator
func (mnc *MultiNodeCoordinator) Stop() {
	close(mnc.stopChan)
	mnc.saveState()
}