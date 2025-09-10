package nodes

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// File I/O operations for the node manager

// loadSystemConfig loads the system configuration from disk
func (nm *NodeManagerImpl) loadSystemConfig() error {
	configPath := filepath.Join(nm.dataPath, systemConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Use default config if file doesn't exist
			nm.config = nm.getDefaultSystemConfig()
			return nm.saveSystemConfig()
		}
		return fmt.Errorf("failed to read system config: %w", err)
	}

	if err := json.Unmarshal(data, &nm.config); err != nil {
		return fmt.Errorf("failed to unmarshal system config: %w", err)
	}

	return nil
}

// saveSystemConfig saves the system configuration to disk
func (nm *NodeManagerImpl) saveSystemConfig() error {
	configPath := filepath.Join(nm.dataPath, systemConfigFile)
	
	// Ensure directory exists
	if err := os.MkdirAll(nm.dataPath, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(nm.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal system config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write system config: %w", err)
	}

	return nil
}

// loadNodeConfigs loads node configurations from disk
func (nm *NodeManagerImpl) loadNodeConfigs() error {
	configPath := filepath.Join(nm.dataPath, nodesConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing config file
		}
		return fmt.Errorf("failed to read nodes config: %w", err)
	}

	var nodeConfigs map[int]NodeConfiguration
	if err := json.Unmarshal(data, &nodeConfigs); err != nil {
		return fmt.Errorf("failed to unmarshal nodes config: %w", err)
	}

	// Create NodeInfo entries for loaded configurations
	for nodeID, config := range nodeConfigs {
		nodeInfo := &NodeInfo{
			NodeID:       nodeID,
			Status:       NodeStatusAvailable,
			ConnectTime:  time.Now(),
			Config:       config,
			Activity:     NodeActivity{Type: "idle", Description: "Waiting for connection"},
			LastActivity: time.Now(),
		}

		if !config.Enabled {
			nodeInfo.Status = NodeStatusDisabled
		}

		nm.nodes[nodeID] = nodeInfo
	}

	return nil
}

// saveNodeConfigs saves node configurations to disk
func (nm *NodeManagerImpl) saveNodeConfigs() error {
	configPath := filepath.Join(nm.dataPath, nodesConfigFile)
	
	// Extract configurations from nodes
	nodeConfigs := make(map[int]NodeConfiguration)
	for nodeID, node := range nm.nodes {
		nodeConfigs[nodeID] = node.Config
	}

	data, err := json.MarshalIndent(nodeConfigs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal nodes config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write nodes config: %w", err)
	}

	return nil
}

// loadStatistics loads node statistics from disk
func (nm *NodeManagerImpl) loadStatistics() error {
	statsPath := filepath.Join(nm.dataPath, statisticsFile)
	data, err := os.ReadFile(statsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing statistics file
		}
		return fmt.Errorf("failed to read statistics: %w", err)
	}

	if err := json.Unmarshal(data, &nm.statistics); err != nil {
		return fmt.Errorf("failed to unmarshal statistics: %w", err)
	}

	return nil
}

// saveStatistics saves node statistics to disk
func (nm *NodeManagerImpl) saveStatistics() error {
	statsPath := filepath.Join(nm.dataPath, statisticsFile)
	
	data, err := json.MarshalIndent(nm.statistics, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal statistics: %w", err)
	}

	if err := os.WriteFile(statsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write statistics: %w", err)
	}

	return nil
}

// loadAlerts loads alerts from disk
func (nm *NodeManagerImpl) loadAlerts() error {
	alertsPath := filepath.Join(nm.dataPath, alertsFile)
	data, err := os.ReadFile(alertsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing alerts file
		}
		return fmt.Errorf("failed to read alerts: %w", err)
	}

	if err := json.Unmarshal(data, &nm.alerts); err != nil {
		return fmt.Errorf("failed to unmarshal alerts: %w", err)
	}

	return nil
}

// saveAlerts saves alerts to disk
func (nm *NodeManagerImpl) saveAlerts() error {
	alertsPath := filepath.Join(nm.dataPath, alertsFile)
	
	data, err := json.MarshalIndent(nm.alerts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal alerts: %w", err)
	}

	if err := os.WriteFile(alertsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write alerts: %w", err)
	}

	return nil
}

// Messaging functionality

// SendMessage sends a message to a specific node
func (nm *NodeManagerImpl) SendMessage(message NodeMessage) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	targetNode, exists := nm.nodes[message.ToNode]
	if !exists {
		return fmt.Errorf("target node %d not found", message.ToNode)
	}

	// Add timestamp if not set
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	// Add message to target node's message queue
	targetNode.Messages = append(targetNode.Messages, message)

	// Limit message queue size
	maxMessages := 50
	if len(targetNode.Messages) > maxMessages {
		targetNode.Messages = targetNode.Messages[len(targetNode.Messages)-maxMessages:]
	}

	nm.emitEvent(NodeEvent{
		Type:      "message",
		NodeID:    message.ToNode,
		Timestamp: time.Now(),
		Data:      message,
	})

	log.Printf("INFO: Message sent from node %d to node %d: %s", 
		message.FromNode, message.ToNode, message.Message)

	return nil
}

// BroadcastMessage sends a message to all active nodes
func (nm *NodeManagerImpl) BroadcastMessage(messageText string, fromUser string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	broadcast := NodeMessage{
		FromNode:    0, // System message
		FromUser:    fromUser,
		Message:     messageText,
		Timestamp:   time.Now(),
		MessageType: "broadcast",
		Priority:    2, // Normal priority
	}

	sentCount := 0
	for nodeID, node := range nm.nodes {
		if node.Status != NodeStatusOffline && node.Status != NodeStatusDisabled && node.User != nil {
			broadcast.ToNode = nodeID
			node.Messages = append(node.Messages, broadcast)
			
			// Limit message queue size
			maxMessages := 50
			if len(node.Messages) > maxMessages {
				node.Messages = node.Messages[len(node.Messages)-maxMessages:]
			}
			
			sentCount++
		}
	}

	nm.emitEvent(NodeEvent{
		Type:      "broadcast",
		NodeID:    0,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{
			"message":    messageText,
			"from_user":  fromUser,
			"sent_count": sentCount,
		},
	})

	log.Printf("INFO: Broadcast message sent to %d nodes from %s: %s", 
		sentCount, fromUser, messageText)

	return nil
}

// GetMessages retrieves pending messages for a node
func (nm *NodeManagerImpl) GetMessages(nodeID int) []NodeMessage {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return nil
	}

	// Return a copy of the messages
	messages := make([]NodeMessage, len(node.Messages))
	copy(messages, node.Messages)

	return messages
}

// Force operations

// DisconnectUser forcibly disconnects a user from a node
func (nm *NodeManagerImpl) DisconnectUser(nodeID int, reason string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	return nm.disconnectUserLocked(nodeID, reason)
}

// SendUserMessage sends a message directly to a user on a specific node
func (nm *NodeManagerImpl) SendUserMessage(nodeID int, messageText string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	if node.User == nil {
		return fmt.Errorf("no user logged in on node %d", nodeID)
	}

	message := NodeMessage{
		FromNode:    0, // System message
		FromUser:    "SysOp",
		ToNode:      nodeID,
		Message:     messageText,
		Timestamp:   time.Now(),
		MessageType: "system",
		Priority:    3, // High priority
	}

	node.Messages = append(node.Messages, message)

	// Limit message queue size
	maxMessages := 50
	if len(node.Messages) > maxMessages {
		node.Messages = node.Messages[len(node.Messages)-maxMessages:]
	}

	nm.emitEvent(NodeEvent{
		Type:      "user_message",
		NodeID:    nodeID,
		Timestamp: time.Now(),
		Data:      message,
	})

	log.Printf("INFO: System message sent to user %s on node %d: %s", 
		node.User.Handle, nodeID, messageText)

	return nil
}

// Statistics methods

// UpdateStatistics updates statistics for a specific node
func (nm *NodeManagerImpl) UpdateStatistics(nodeID int, stats NodeStatistics) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.statistics[nodeID] = &stats

	return nm.saveStatistics()
}

// GetHistoricalData retrieves historical statistics for a node (placeholder implementation)
func (nm *NodeManagerImpl) GetHistoricalData(nodeID int, from, to time.Time) ([]NodeStatistics, error) {
	// Placeholder implementation
	// In a real implementation, this would query a time-series database
	
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	stats, exists := nm.statistics[nodeID]
	if !exists {
		return nil, fmt.Errorf("no statistics found for node %d", nodeID)
	}

	// Return current stats as a single data point
	// Real implementation would have historical data points
	return []NodeStatistics{*stats}, nil
}

// Background monitoring and event processing

// monitoringLoop runs the background monitoring process
func (nm *NodeManagerImpl) monitoringLoop() {
	ticker := time.NewTicker(nm.config.MonitorInterval)
	saveTicker := time.NewTicker(nm.config.SaveInterval)
	
	defer ticker.Stop()
	defer saveTicker.Stop()

	for {
		select {
		case <-ticker.C:
			nm.performMonitoringTasks()
		case <-saveTicker.C:
			nm.performSaveTasks()
		case <-nm.stopChan:
			log.Println("INFO: Node monitoring loop stopped")
			return
		}
	}
}

// performMonitoringTasks executes periodic monitoring tasks
func (nm *NodeManagerImpl) performMonitoringTasks() {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	now := time.Now()
	
	for nodeID, node := range nm.nodes {
		// Update idle time
		if !node.LastActivity.IsZero() {
			node.IdleTime = now.Sub(node.LastActivity)
		}

		// Check for timeout conditions
		if node.Session != nil && node.User != nil {
			sessionDuration := now.Sub(node.ConnectTime)
			
			// Check time limit
			if sessionDuration > node.Config.TimeLimit {
				nm.disconnectUserLocked(nodeID, "Time limit exceeded")
				continue
			}
			
			// Check idle timeout (15 minutes default)
			idleTimeout := 15 * time.Minute
			if node.IdleTime > idleTimeout {
				nm.disconnectUserLocked(nodeID, "Idle timeout")
				continue
			}
		}

		// Update performance metrics (placeholder)
		node.CPUUsage = nm.getNodeCPUUsage(nodeID)
		node.MemoryUsage = nm.getNodeMemoryUsage(nodeID)

		// Check for alert conditions
		nm.checkNodeAlerts(nodeID, node)
	}
}

// performSaveTasks saves data to disk periodically
func (nm *NodeManagerImpl) performSaveTasks() {
	if err := nm.saveStatistics(); err != nil {
		log.Printf("ERROR: Failed to save statistics: %v", err)
	}
	
	if err := nm.saveAlerts(); err != nil {
		log.Printf("ERROR: Failed to save alerts: %v", err)
	}
	
	if err := nm.saveNodeConfigs(); err != nil {
		log.Printf("ERROR: Failed to save node configs: %v", err)
	}
}

// checkNodeAlerts checks for alert conditions on a node
func (nm *NodeManagerImpl) checkNodeAlerts(nodeID int, node *NodeInfo) {
	// CPU usage alert
	if node.CPUUsage > 90.0 {
		alert := NodeAlert{
			NodeID:    nodeID,
			AlertType: "warning",
			Message:   fmt.Sprintf("High CPU usage on node %d: %.1f%%", nodeID, node.CPUUsage),
			Timestamp: time.Now(),
			AutoClear: true,
		}
		nm.alerts = append(nm.alerts, alert)
	}

	// Memory usage alert
	if node.MemoryUsage > 1024*1024*1024 { // 1GB
		alert := NodeAlert{
			NodeID:    nodeID,
			AlertType: "warning",
			Message:   fmt.Sprintf("High memory usage on node %d: %d MB", nodeID, node.MemoryUsage/(1024*1024)),
			Timestamp: time.Now(),
			AutoClear: true,
		}
		nm.alerts = append(nm.alerts, alert)
	}

	// Connection failure alert
	if node.Status == NodeStatusOffline && node.Config.Enabled {
		alert := NodeAlert{
			NodeID:    nodeID,
			AlertType: "error",
			Message:   fmt.Sprintf("Node %d is offline but should be enabled", nodeID),
			Timestamp: time.Now(),
			AutoClear: false,
		}
		nm.alerts = append(nm.alerts, alert)
	}
}

// eventProcessor handles events from the event channel
func (nm *NodeManagerImpl) eventProcessor() {
	for {
		select {
		case event := <-nm.eventChan:
			nm.processEvent(event)
		case <-nm.stopChan:
			log.Println("INFO: Node event processor stopped")
			return
		}
	}
}

// processEvent processes a single node event
func (nm *NodeManagerImpl) processEvent(event NodeEvent) {
	// Notify all registered listeners
	for _, listener := range nm.listeners {
		if err := listener.OnNodeEvent(event); err != nil {
			log.Printf("ERROR: Event listener error: %v", err)
		}
	}

	// Log important events
	switch event.Type {
	case "connect", "disconnect", "alert":
		log.Printf("INFO: Node event - Type: %s, NodeID: %d, Time: %s", 
			event.Type, event.NodeID, event.Timestamp.Format(time.RFC3339))
	}
}

// Helper methods for performance monitoring
func (nm *NodeManagerImpl) getNodeCPUUsage(nodeID int) float64 {
	// Placeholder implementation
	// Real implementation would measure actual CPU usage
	return float64(nodeID) * 2.5 // Fake data for demo
}

func (nm *NodeManagerImpl) getNodeMemoryUsage(nodeID int) int64 {
	// Placeholder implementation
	// Real implementation would measure actual memory usage
	return int64(nodeID) * 1024 * 1024 * 10 // Fake data for demo
}

// AddEventListener adds an event listener for real-time updates
func (nm *NodeManagerImpl) AddEventListener(listener NodeEventListener) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	nm.listeners = append(nm.listeners, listener)
}

// RemoveEventListener removes an event listener
func (nm *NodeManagerImpl) RemoveEventListener(listener NodeEventListener) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	for i, l := range nm.listeners {
		if l == listener {
			nm.listeners = append(nm.listeners[:i], nm.listeners[i+1:]...)
			break
		}
	}
}

// Shutdown gracefully shuts down the node manager
func (nm *NodeManagerImpl) Shutdown() error {
	log.Println("INFO: Shutting down node manager...")
	
	// Stop background processes
	close(nm.stopChan)
	
	// Save all data
	if err := nm.saveSystemConfig(); err != nil {
		log.Printf("ERROR: Failed to save system config during shutdown: %v", err)
	}
	
	if err := nm.saveNodeConfigs(); err != nil {
		log.Printf("ERROR: Failed to save node configs during shutdown: %v", err)
	}
	
	if err := nm.saveStatistics(); err != nil {
		log.Printf("ERROR: Failed to save statistics during shutdown: %v", err)
	}
	
	if err := nm.saveAlerts(); err != nil {
		log.Printf("ERROR: Failed to save alerts during shutdown: %v", err)
	}
	
	log.Println("INFO: Node manager shutdown complete")
	return nil
}