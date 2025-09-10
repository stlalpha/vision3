package nodes

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/stlalpha/vision3/internal/session"
	"github.com/stlalpha/vision3/internal/user"
)

// Integration provides the bridge between the node management system
// and the existing session/user management systems
type Integration struct {
	nodeManager     NodeManager
	userManager     *user.UserMgr
	activeSessions  map[int]*session.BbsSession // nodeID -> session
	sessionNodes    map[*session.BbsSession]int // session -> nodeID
	eventListeners  []IntegrationEventListener
	mu              sync.RWMutex
	running         bool
	stopChan        chan bool
	monitorInterval time.Duration
}

// IntegrationEventListener receives events from the integration system
type IntegrationEventListener interface {
	OnSessionStart(nodeID int, session *session.BbsSession, user *user.User) error
	OnSessionEnd(nodeID int, session *session.BbsSession, user *user.User, duration time.Duration) error
	OnUserLogin(nodeID int, session *session.BbsSession, user *user.User) error
	OnUserLogout(nodeID int, session *session.BbsSession, user *user.User) error
	OnActivityChange(nodeID int, session *session.BbsSession, activity NodeActivity) error
}

// SessionWrapper wraps a BBS session with node management functionality
type SessionWrapper struct {
	*session.BbsSession
	integration *Integration
	nodeID      int
	startTime   time.Time
	lastActivity time.Time
	activityStack []string
}

// NewIntegration creates a new integration between node management and session/user systems
func NewIntegration(dataPath string) (*Integration, error) {
	// Create user manager
	userManager, err := user.NewUserManager(dataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create user manager: %w", err)
	}
	
	// Create node manager
	nodeManager, err := NewNodeManager(dataPath, userManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create node manager: %w", err)
	}
	
	integration := &Integration{
		nodeManager:     nodeManager,
		userManager:     userManager,
		activeSessions:  make(map[int]*session.BbsSession),
		sessionNodes:    make(map[*session.BbsSession]int),
		eventListeners:  make([]IntegrationEventListener, 0),
		stopChan:        make(chan bool),
		monitorInterval: 5 * time.Second,
	}
	
	return integration, nil
}

// Start begins the integration monitoring
func (i *Integration) Start() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	if i.running {
		return fmt.Errorf("integration already running")
	}
	
	i.running = true
	go i.monitorLoop()
	
	log.Println("INFO: Node integration started")
	return nil
}

// Stop stops the integration monitoring
func (i *Integration) Stop() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	if !i.running {
		return fmt.Errorf("integration not running")
	}
	
	i.running = false
	close(i.stopChan)
	
	log.Println("INFO: Node integration stopped")
	return nil
}

// RegisterSession registers a new session with the node management system
func (i *Integration) RegisterSession(session *session.BbsSession) (*SessionWrapper, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	// Find an available node
	nodeID := i.findAvailableNode()
	if nodeID == 0 {
		return nil, fmt.Errorf("no available nodes")
	}
	
	// Register with node manager
	if err := i.nodeManager.RegisterSession(nodeID, session); err != nil {
		return nil, fmt.Errorf("failed to register session with node manager: %w", err)
	}
	
	// Track session
	i.activeSessions[nodeID] = session
	i.sessionNodes[session] = nodeID
	
	// Create wrapper
	wrapper := &SessionWrapper{
		BbsSession:    session,
		integration:   i,
		nodeID:        nodeID,
		startTime:     time.Now(),
		lastActivity:  time.Now(),
		activityStack: make([]string, 0),
	}
	
	// Notify listeners
	for _, listener := range i.eventListeners {
		if err := listener.OnSessionStart(nodeID, session, nil); err != nil {
			log.Printf("ERROR: Session start listener error: %v", err)
		}
	}
	
	log.Printf("INFO: Session registered on node %d", nodeID)
	return wrapper, nil
}

// UnregisterSession removes a session from the node management system
func (i *Integration) UnregisterSession(session *session.BbsSession) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	nodeID, exists := i.sessionNodes[session]
	if !exists {
		return fmt.Errorf("session not found")
	}
	
	// Get user for event notification
	node, _ := i.nodeManager.GetNode(nodeID)
	var currentUser *user.User
	if node != nil {
		currentUser = node.User
	}
	
	// Calculate session duration
	startTime := time.Now() // Default if we can't get real start time
	if node != nil && !node.ConnectTime.IsZero() {
		startTime = node.ConnectTime
	}
	duration := time.Since(startTime)
	
	// Unregister from node manager
	if err := i.nodeManager.UnregisterSession(nodeID); err != nil {
		log.Printf("ERROR: Failed to unregister session from node manager: %v", err)
	}
	
	// Update tracking
	delete(i.activeSessions, nodeID)
	delete(i.sessionNodes, session)
	
	// Notify listeners
	for _, listener := range i.eventListeners {
		if err := listener.OnSessionEnd(nodeID, session, currentUser, duration); err != nil {
			log.Printf("ERROR: Session end listener error: %v", err)
		}
	}
	
	// Add call record to user manager if user was logged in
	if currentUser != nil {
		callRecord := user.CallRecord{
			UserID:         currentUser.ID,
			Handle:         currentUser.Handle,
			GroupLocation:  currentUser.GroupLocation,
			NodeID:         nodeID,
			ConnectTime:    startTime,
			DisconnectTime: time.Now(),
			Duration:       duration,
			UploadedMB:     0,   // Placeholder
			DownloadedMB:   0,   // Placeholder
			Actions:        "U", // Placeholder
			BaudRate:       "33600",
		}
		
		i.userManager.AddCallRecord(callRecord)
	}
	
	log.Printf("INFO: Session unregistered from node %d", nodeID)
	return nil
}

// SessionWrapper methods for enhanced session management

// Login handles user login for the session
func (sw *SessionWrapper) Login(username, password string) (*user.User, error) {
	authenticatedUser, success := sw.integration.userManager.Authenticate(username, password)
	if !success {
		return nil, fmt.Errorf("authentication failed")
	}
	
	// Update node with user information
	node, err := sw.integration.nodeManager.GetNode(sw.nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	
	// Update the session's user field
	sw.User = authenticatedUser
	
	// Update node manager with activity
	loginActivity := NodeActivity{
		Type:        "login",
		Description: fmt.Sprintf("User %s logged in", authenticatedUser.Handle),
		StartTime:   time.Now(),
	}
	
	if err := sw.integration.nodeManager.UpdateActivity(sw.nodeID, loginActivity); err != nil {
		log.Printf("ERROR: Failed to update node activity: %v", err)
	}
	
	// Notify listeners
	for _, listener := range sw.integration.eventListeners {
		if err := listener.OnUserLogin(sw.nodeID, sw.BbsSession, authenticatedUser); err != nil {
			log.Printf("ERROR: User login listener error: %v", err)
		}
	}
	
	log.Printf("INFO: User %s logged in on node %d", authenticatedUser.Handle, sw.nodeID)
	return authenticatedUser, nil
}

// Logout handles user logout for the session
func (sw *SessionWrapper) Logout() error {
	if sw.User == nil {
		return fmt.Errorf("no user logged in")
	}
	
	currentUser := sw.User
	
	// Update activity
	logoutActivity := NodeActivity{
		Type:        "logout",
		Description: "User logged out",
		StartTime:   time.Now(),
	}
	
	if err := sw.integration.nodeManager.UpdateActivity(sw.nodeID, logoutActivity); err != nil {
		log.Printf("ERROR: Failed to update node activity: %v", err)
	}
	
	// Clear user from session
	sw.User = nil
	
	// Notify listeners
	for _, listener := range sw.integration.eventListeners {
		if err := listener.OnUserLogout(sw.nodeID, sw.BbsSession, currentUser); err != nil {
			log.Printf("ERROR: User logout listener error: %v", err)
		}
	}
	
	log.Printf("INFO: User %s logged out from node %d", currentUser.Handle, sw.nodeID)
	return nil
}

// SetActivity updates the current activity for the session
func (sw *SessionWrapper) SetActivity(activityType, description string) error {
	activity := NodeActivity{
		Type:        activityType,
		Description: description,
		StartTime:   time.Now(),
	}
	
	// Update last activity time
	sw.lastActivity = time.Now()
	
	// Update activity stack
	sw.activityStack = append(sw.activityStack, description)
	if len(sw.activityStack) > 10 { // Keep last 10 activities
		sw.activityStack = sw.activityStack[1:]
	}
	
	// Set current menu if applicable
	if activityType == "menu" {
		sw.CurrentMenu = description
		activity.MenuName = description
	}
	
	// Update node manager
	if err := sw.integration.nodeManager.UpdateActivity(sw.nodeID, activity); err != nil {
		return fmt.Errorf("failed to update activity: %w", err)
	}
	
	// Notify listeners
	for _, listener := range sw.integration.eventListeners {
		if err := listener.OnActivityChange(sw.nodeID, sw.BbsSession, activity); err != nil {
			log.Printf("ERROR: Activity change listener error: %v", err)
		}
	}
	
	return nil
}

// GetNodeID returns the node ID for this session
func (sw *SessionWrapper) GetNodeID() int {
	return sw.nodeID
}

// GetOnlineTime returns how long the session has been active
func (sw *SessionWrapper) GetOnlineTime() time.Duration {
	return time.Since(sw.startTime)
}

// GetIdleTime returns how long since the last activity
func (sw *SessionWrapper) GetIdleTime() time.Duration {
	return time.Since(sw.lastActivity)
}

// GetActivityHistory returns the recent activity history
func (sw *SessionWrapper) GetActivityHistory() []string {
	sw.integration.mu.RLock()
	defer sw.integration.mu.RUnlock()
	
	// Return a copy of the activity stack
	history := make([]string, len(sw.activityStack))
	copy(history, sw.activityStack)
	return history
}

// SendMessage sends a message to this session
func (sw *SessionWrapper) SendMessage(from string, message string) error {
	nodeMessage := NodeMessage{
		FromNode:    0, // System message
		FromUser:    from,
		ToNode:      sw.nodeID,
		Message:     message,
		MessageType: "system",
		Priority:    2,
		Timestamp:   time.Now(),
	}
	
	return sw.integration.nodeManager.SendMessage(nodeMessage)
}

// Integration management methods

// AddEventListener adds an event listener to the integration
func (i *Integration) AddEventListener(listener IntegrationEventListener) {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	i.eventListeners = append(i.eventListeners, listener)
}

// RemoveEventListener removes an event listener from the integration
func (i *Integration) RemoveEventListener(listener IntegrationEventListener) {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	for idx, l := range i.eventListeners {
		if l == listener {
			i.eventListeners = append(i.eventListeners[:idx], i.eventListeners[idx+1:]...)
			break
		}
	}
}

// GetNodeManager returns the node manager instance
func (i *Integration) GetNodeManager() NodeManager {
	return i.nodeManager
}

// GetUserManager returns the user manager instance
func (i *Integration) GetUserManager() *user.UserMgr {
	return i.userManager
}

// GetActiveSessionCount returns the number of active sessions
func (i *Integration) GetActiveSessionCount() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	
	return len(i.activeSessions)
}

// GetActiveSessions returns information about all active sessions
func (i *Integration) GetActiveSessions() map[int]*session.BbsSession {
	i.mu.RLock()
	defer i.mu.RUnlock()
	
	// Return a copy to prevent external modification
	sessions := make(map[int]*session.BbsSession)
	for nodeID, session := range i.activeSessions {
		sessions[nodeID] = session
	}
	
	return sessions
}

// BroadcastMessage sends a message to all active sessions
func (i *Integration) BroadcastMessage(from, message string) error {
	return i.nodeManager.BroadcastMessage(message, from)
}

// KickUser forcibly disconnects a user from a specific node
func (i *Integration) KickUser(nodeID int, reason string) error {
	return i.nodeManager.DisconnectUser(nodeID, reason)
}

// Private helper methods

// findAvailableNode finds an available node for a new session
func (i *Integration) findAvailableNode() int {
	nodes := i.nodeManager.GetAllNodes()
	
	// Find the first available node
	for _, node := range nodes {
		if node.Config.Enabled && 
		   node.Status == NodeStatusAvailable && 
		   len(i.activeSessions) < node.Config.MaxUsers {
			return node.NodeID
		}
	}
	
	return 0 // No available nodes
}

// monitorLoop runs periodic monitoring and maintenance tasks
func (i *Integration) monitorLoop() {
	ticker := time.NewTicker(i.monitorInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			i.performMonitoringTasks()
		case <-i.stopChan:
			log.Println("INFO: Integration monitoring loop stopped")
			return
		}
	}
}

// performMonitoringTasks performs periodic monitoring and cleanup
func (i *Integration) performMonitoringTasks() {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	// Check for stale sessions
	for nodeID, session := range i.activeSessions {
		node, err := i.nodeManager.GetNode(nodeID)
		if err != nil {
			continue
		}
		
		// Check for timeout conditions
		if node.User != nil {
			sessionDuration := time.Since(node.ConnectTime)
			
			// Check time limit
			if sessionDuration > node.Config.TimeLimit {
				log.Printf("INFO: Session on node %d exceeded time limit, disconnecting", nodeID)
				i.nodeManager.DisconnectUser(nodeID, "Time limit exceeded")
				continue
			}
			
			// Check idle timeout
			idleTimeout := 15 * time.Minute
			if time.Since(node.LastActivity) > idleTimeout {
				log.Printf("INFO: Session on node %d idle too long, disconnecting", nodeID)
				i.nodeManager.DisconnectUser(nodeID, "Idle timeout")
				continue
			}
		}
		
		// Update session heartbeat
		if session != nil {
			// Could update last seen time or perform health checks
		}
	}
	
	// Sync node statuses with actual sessions
	i.syncNodeStatuses()
}

// syncNodeStatuses synchronizes node statuses with actual session states
func (i *Integration) syncNodeStatuses() {
	nodes := i.nodeManager.GetAllNodes()
	
	for _, node := range nodes {
		session, hasSession := i.activeSessions[node.NodeID]
		
		// If node thinks it has a user but we don't have a session, clean it up
		if node.User != nil && !hasSession {
			log.Printf("WARN: Node %d has user but no active session, cleaning up", node.NodeID)
			i.nodeManager.UnregisterSession(node.NodeID)
		}
		
		// If we have a session but node doesn't know about user, update it
		if hasSession && session.User != nil && node.User == nil {
			log.Printf("WARN: Session exists but node %d doesn't have user info, updating", node.NodeID)
			// Could trigger a sync operation here
		}
	}
}

// Example integration event listener implementation
type DefaultIntegrationListener struct{}

func (dil *DefaultIntegrationListener) OnSessionStart(nodeID int, session *session.BbsSession, user *user.User) error {
	log.Printf("INFO: Session started on node %d", nodeID)
	return nil
}

func (dil *DefaultIntegrationListener) OnSessionEnd(nodeID int, session *session.BbsSession, user *user.User, duration time.Duration) error {
	if user != nil {
		log.Printf("INFO: Session ended on node %d for user %s (duration: %s)", nodeID, user.Handle, duration)
	} else {
		log.Printf("INFO: Session ended on node %d (duration: %s)", nodeID, duration)
	}
	return nil
}

func (dil *DefaultIntegrationListener) OnUserLogin(nodeID int, session *session.BbsSession, user *user.User) error {
	log.Printf("INFO: User %s logged in on node %d", user.Handle, nodeID)
	return nil
}

func (dil *DefaultIntegrationListener) OnUserLogout(nodeID int, session *session.BbsSession, user *user.User) error {
	log.Printf("INFO: User %s logged out from node %d", user.Handle, nodeID)
	return nil
}

func (dil *DefaultIntegrationListener) OnActivityChange(nodeID int, session *session.BbsSession, activity NodeActivity) error {
	log.Printf("DEBUG: Activity change on node %d: %s - %s", nodeID, activity.Type, activity.Description)
	return nil
}

// CreateDefaultIntegration creates a basic integration with default settings
func CreateDefaultIntegration(dataPath string) (*Integration, error) {
	integration, err := NewIntegration(dataPath)
	if err != nil {
		return nil, err
	}
	
	// Add default event listener
	defaultListener := &DefaultIntegrationListener{}
	integration.AddEventListener(defaultListener)
	
	// Start the integration
	if err := integration.Start(); err != nil {
		return nil, fmt.Errorf("failed to start integration: %w", err)
	}
	
	return integration, nil
}