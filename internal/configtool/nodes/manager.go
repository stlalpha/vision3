package nodes

import (
	"fmt"
	"log"
	"time"

	"github.com/stlalpha/vision3/internal/session"
	"github.com/stlalpha/vision3/internal/user"
)

const (
	nodesConfigFile    = "nodes.json"
	systemConfigFile   = "system.json"
	statisticsFile     = "node_stats.json"
	alertsFile         = "node_alerts.json"
	defaultMaxNodes    = 32
	defaultTimeLimit   = 60 * time.Minute
	defaultMonitorInterval = 5 * time.Second
	defaultSaveInterval    = 30 * time.Second
)

// NewNodeManager creates a new node manager instance
func NewNodeManager(dataPath string, userManager *user.UserMgr) (NodeManager, error) {
	nm := &NodeManagerImpl{
		nodes:       make(map[int]*NodeInfo),
		statistics:  make(map[int]*NodeStatistics),
		alerts:      make([]NodeAlert, 0),
		listeners:   make([]NodeEventListener, 0),
		eventChan:   make(chan NodeEvent, 100),
		stopChan:    make(chan bool),
		dataPath:    dataPath,
		userManager: userManager,
	}

	// Load configuration
	if err := nm.loadSystemConfig(); err != nil {
		log.Printf("WARN: Failed to load system config, using defaults: %v", err)
		nm.config = nm.getDefaultSystemConfig()
	}

	// Load existing node configurations
	if err := nm.loadNodeConfigs(); err != nil {
		log.Printf("WARN: Failed to load node configs: %v", err)
	}

	// Load statistics
	if err := nm.loadStatistics(); err != nil {
		log.Printf("WARN: Failed to load statistics: %v", err)
	}

	// Load alerts
	if err := nm.loadAlerts(); err != nil {
		log.Printf("WARN: Failed to load alerts: %v", err)
	}

	// Initialize default nodes if none exist
	if len(nm.nodes) == 0 {
		nm.initializeDefaultNodes()
	}

	// Start background monitoring
	go nm.monitoringLoop()
	go nm.eventProcessor()

	return nm, nil
}

// getDefaultSystemConfig returns default system configuration
func (nm *NodeManagerImpl) getDefaultSystemConfig() SystemConfig {
	return SystemConfig{
		MaxNodes:        defaultMaxNodes,
		DefaultTimeLimit: defaultTimeLimit,
		ChatEnabled:     true,
		InterNodeChat:   true,
		AlertsEnabled:   true,
		LogLevel:        "INFO",
		MonitorInterval: defaultMonitorInterval,
		SaveInterval:    defaultSaveInterval,
		BackupInterval:  24 * time.Hour,
		MaxAlerts:       100,
		AutoCleanup:     true,
		CleanupInterval: time.Hour,
	}
}

// initializeDefaultNodes creates default node configurations
func (nm *NodeManagerImpl) initializeDefaultNodes() {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Create default local nodes
	for i := 1; i <= 4; i++ {
		config := NodeConfiguration{
			NodeID:      i,
			Name:        fmt.Sprintf("Local Node %d", i),
			Enabled:     true,
			MaxUsers:    1,
			TimeLimit:   defaultTimeLimit,
			AccessLevel: 1,
			LocalNode:   true,
			ChatEnabled: true,
			NetworkSettings: NetworkConfig{
				Protocol: "telnet",
				Port:     2300 + i,
				Address:  "0.0.0.0",
			},
			DoorSettings: DoorConfig{
				AllowDoors:     true,
				MaxDoorTime:    30,
				ShareResources: true,
			},
		}

		nodeInfo := &NodeInfo{
			NodeID:       i,
			Status:       NodeStatusAvailable,
			ConnectTime:  time.Now(),
			Config:       config,
			Activity:     NodeActivity{Type: "idle", Description: "Waiting for connection"},
			LastActivity: time.Now(),
		}

		nm.nodes[i] = nodeInfo
		nm.statistics[i] = &NodeStatistics{
			NodeID:        i,
			LastRestart:   time.Now(),
			UptimePercent: 100.0,
		}
	}

	log.Printf("INFO: Initialized %d default nodes", len(nm.nodes))
}

// Node management methods
func (nm *NodeManagerImpl) GetNode(nodeID int) (*NodeInfo, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %d not found", nodeID)
	}

	// Return a copy to prevent external modification
	nodeCopy := *node
	return &nodeCopy, nil
}

func (nm *NodeManagerImpl) GetAllNodes() []*NodeInfo {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	nodes := make([]*NodeInfo, 0, len(nm.nodes))
	for _, node := range nm.nodes {
		nodeCopy := *node
		nodes = append(nodes, &nodeCopy)
	}

	return nodes
}

func (nm *NodeManagerImpl) GetActiveNodes() []*NodeInfo {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	activeNodes := make([]*NodeInfo, 0)
	for _, node := range nm.nodes {
		if node.Status != NodeStatusOffline && node.Status != NodeStatusDisabled {
			nodeCopy := *node
			activeNodes = append(activeNodes, &nodeCopy)
		}
	}

	return activeNodes
}

func (nm *NodeManagerImpl) EnableNode(nodeID int) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	node.Config.Enabled = true
	node.Status = NodeStatusAvailable
	node.LastActivity = time.Now()

	nm.emitEvent(NodeEvent{
		Type:      "status",
		NodeID:    nodeID,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"status": "enabled"},
	})

	return nm.saveNodeConfigs()
}

func (nm *NodeManagerImpl) DisableNode(nodeID int) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	// If user is connected, disconnect them first
	if node.Session != nil && node.User != nil {
		nm.disconnectUserLocked(nodeID, "Node disabled by system operator")
	}

	node.Config.Enabled = false
	node.Status = NodeStatusDisabled
	node.LastActivity = time.Now()

	nm.emitEvent(NodeEvent{
		Type:      "status",
		NodeID:    nodeID,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"status": "disabled"},
	})

	return nm.saveNodeConfigs()
}

func (nm *NodeManagerImpl) RestartNode(nodeID int) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	// Disconnect any connected user
	if node.Session != nil && node.User != nil {
		nm.disconnectUserLocked(nodeID, "Node restarted by system operator")
	}

	// Reset node state
	node.Status = NodeStatusAvailable
	node.Session = nil
	node.User = nil
	node.Activity = NodeActivity{Type: "idle", Description: "Node restarted"}
	node.ConnectTime = time.Now()
	node.LastActivity = time.Now()
	node.BytesSent = 0
	node.BytesReceived = 0
	node.MenuPath = nil
	node.Messages = nil

	// Update statistics
	if stats, exists := nm.statistics[nodeID]; exists {
		stats.LastRestart = time.Now()
	}

	nm.emitEvent(NodeEvent{
		Type:      "restart",
		NodeID:    nodeID,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"reason": "manual_restart"},
	})

	return nil
}

// Session management methods
func (nm *NodeManagerImpl) RegisterSession(nodeID int, bbsSession *session.BbsSession) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	if !node.Config.Enabled {
		return fmt.Errorf("node %d is disabled", nodeID)
	}

	node.Session = bbsSession
	node.Status = NodeStatusConnected
	node.ConnectTime = time.Now()
	node.LastActivity = time.Now()
	node.RemoteAddr = bbsSession.RemoteAddr
	node.Activity = NodeActivity{
		Type:        "connected",
		Description: "User connected",
		StartTime:   time.Now(),
	}

	// Update statistics
	if stats, exists := nm.statistics[nodeID]; exists {
		stats.TotalConnections++
	}

	nm.emitEvent(NodeEvent{
		Type:      "connect",
		NodeID:    nodeID,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"remote_addr": bbsSession.RemoteAddr.String()},
	})

	return nil
}

func (nm *NodeManagerImpl) UnregisterSession(nodeID int) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	// Calculate session duration
	var sessionDuration time.Duration
	if !node.ConnectTime.IsZero() {
		sessionDuration = time.Since(node.ConnectTime)
	}

	// Update statistics
	if stats, exists := nm.statistics[nodeID]; exists {
		stats.TotalTime += sessionDuration
		if stats.TotalConnections > 0 {
			stats.AverageSession = stats.TotalTime / time.Duration(stats.TotalConnections)
		}
	}

	// Reset node state
	node.Session = nil
	node.User = nil
	node.Status = NodeStatusAvailable
	node.Activity = NodeActivity{
		Type:        "idle",
		Description: "Waiting for connection",
		StartTime:   time.Now(),
	}
	node.RemoteAddr = nil
	node.MenuPath = nil
	node.Messages = nil

	nm.emitEvent(NodeEvent{
		Type:      "disconnect",
		NodeID:    nodeID,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"session_duration": sessionDuration.String()},
	})

	return nil
}

func (nm *NodeManagerImpl) UpdateActivity(nodeID int, activity NodeActivity) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	node.Activity = activity
	node.LastActivity = time.Now()

	// Update status based on activity type
	switch activity.Type {
	case "menu":
		node.Status = NodeStatusInMenu
	case "door":
		node.Status = NodeStatusInDoor
	case "message":
		node.Status = NodeStatusInMessage
	case "file":
		node.Status = NodeStatusInFile
	case "chat":
		node.Status = NodeStatusInChat
	case "login":
		node.Status = NodeStatusLoggedIn
	}

	nm.emitEvent(NodeEvent{
		Type:      "activity",
		NodeID:    nodeID,
		Timestamp: time.Now(),
		Data:      activity,
	})

	return nil
}

func (nm *NodeManagerImpl) GetNodeActivity(nodeID int) (NodeActivity, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return NodeActivity{}, fmt.Errorf("node %d not found", nodeID)
	}

	return node.Activity, nil
}

// Configuration methods
func (nm *NodeManagerImpl) GetNodeConfig(nodeID int) (*NodeConfiguration, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %d not found", nodeID)
	}

	configCopy := node.Config
	return &configCopy, nil
}

func (nm *NodeManagerImpl) UpdateNodeConfig(nodeID int, config NodeConfiguration) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	node.Config = config
	
	// Update node availability based on enabled status
	if !config.Enabled {
		if node.Session != nil && node.User != nil {
			nm.disconnectUserLocked(nodeID, "Node configuration changed")
		}
		node.Status = NodeStatusDisabled
	} else if node.Status == NodeStatusDisabled {
		node.Status = NodeStatusAvailable
	}

	return nm.saveNodeConfigs()
}

func (nm *NodeManagerImpl) GetSystemConfig() (*SystemConfig, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	configCopy := nm.config
	return &configCopy, nil
}

func (nm *NodeManagerImpl) UpdateSystemConfig(config SystemConfig) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.config = config
	return nm.saveSystemConfig()
}

// Monitoring methods
func (nm *NodeManagerImpl) GetSystemStatus() (*SystemStatus, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	totalNodes := len(nm.nodes)
	activeNodes := 0
	connectedUsers := 0

	for _, node := range nm.nodes {
		if node.Status != NodeStatusOffline && node.Status != NodeStatusDisabled {
			activeNodes++
		}
		if node.User != nil {
			connectedUsers++
		}
	}

	// Copy statistics map
	nodeStats := make(map[int]NodeStatistics)
	for id, stats := range nm.statistics {
		nodeStats[id] = *stats
	}

	// Copy alerts slice
	alertsCopy := make([]NodeAlert, len(nm.alerts))
	copy(alertsCopy, nm.alerts)

	return &SystemStatus{
		TotalNodes:     totalNodes,
		ActiveNodes:    activeNodes,
		ConnectedUsers: connectedUsers,
		SystemLoad:     nm.getSystemLoad(),
		MemoryUsage:    nm.getMemoryUsage(),
		Uptime:         time.Since(time.Now().Add(-24 * time.Hour)), // Placeholder
		LastUpdate:     time.Now(),
		Alerts:         alertsCopy,
		NodeStats:      nodeStats,
	}, nil
}

func (nm *NodeManagerImpl) GetNodeStatistics(nodeID int) (*NodeStatistics, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	stats, exists := nm.statistics[nodeID]
	if !exists {
		return nil, fmt.Errorf("statistics for node %d not found", nodeID)
	}

	statsCopy := *stats
	return &statsCopy, nil
}

func (nm *NodeManagerImpl) AddAlert(alert NodeAlert) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.alerts = append(nm.alerts, alert)

	// Limit alert history
	if len(nm.alerts) > nm.config.MaxAlerts {
		nm.alerts = nm.alerts[len(nm.alerts)-nm.config.MaxAlerts:]
	}

	nm.emitEvent(NodeEvent{
		Type:      "alert",
		NodeID:    alert.NodeID,
		Timestamp: time.Now(),
		Data:      alert,
	})

	return nm.saveAlerts()
}

func (nm *NodeManagerImpl) GetAlerts() []NodeAlert {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	alertsCopy := make([]NodeAlert, len(nm.alerts))
	copy(alertsCopy, nm.alerts)
	return alertsCopy
}

func (nm *NodeManagerImpl) AcknowledgeAlert(alertID int) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if alertID < 0 || alertID >= len(nm.alerts) {
		return fmt.Errorf("alert ID %d not found", alertID)
	}

	nm.alerts[alertID].Acknowledged = true
	return nm.saveAlerts()
}

// Helper methods for system metrics
func (nm *NodeManagerImpl) getSystemLoad() float64 {
	// Placeholder implementation
	// In a real implementation, this would read from /proc/loadavg or equivalent
	return 0.5
}

func (nm *NodeManagerImpl) getMemoryUsage() int64 {
	// Placeholder implementation
	// In a real implementation, this would read memory statistics
	return 1024 * 1024 * 512 // 512MB placeholder
}

// disconnectUserLocked forcibly disconnects a user (assumes lock is held)
func (nm *NodeManagerImpl) disconnectUserLocked(nodeID int, reason string) error {
	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	if node.Session != nil {
		// Close the session connection
		if node.Session.Channel != nil {
			node.Session.Channel.Close()
		}
		if node.Session.Conn != nil {
			node.Session.Conn.Close()
		}
	}

	// Add alert for forced disconnection
	alert := NodeAlert{
		NodeID:    nodeID,
		AlertType: "info",
		Message:   fmt.Sprintf("User %s disconnected: %s", node.User.Handle, reason),
		Timestamp: time.Now(),
	}
	nm.alerts = append(nm.alerts, alert)

	return nil
}

// emitEvent sends an event to the event channel
func (nm *NodeManagerImpl) emitEvent(event NodeEvent) {
	select {
	case nm.eventChan <- event:
	default:
		log.Printf("WARN: Event channel full, dropping event for node %d", event.NodeID)
	}
}

// Event processing and monitoring loop will be continued in the next file...