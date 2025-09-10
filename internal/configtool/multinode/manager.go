package multinode

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	ErrNodeNotFound     = errors.New("node not found")
	ErrLockTimeout      = errors.New("lock acquisition timeout")
	ErrResourceBusy     = errors.New("resource is busy")
	ErrSemaphoreExists  = errors.New("semaphore already exists")
	ErrDeadlockDetected = errors.New("deadlock detected")
	ErrInvalidNode      = errors.New("invalid node number")
	ErrSystemShutdown   = errors.New("system shutdown in progress")
)

// MultiNodeManager coordinates multi-node BBS operations
type MultiNodeManager struct {
	BasePath     string
	NodeNumber   uint8
	mutex        sync.RWMutex
	localLocks   map[string]*os.File
	lockFiles    map[string]*os.File
	statusTimer  *time.Timer
	isShutdown   bool
}

// NewMultiNodeManager creates a new multi-node manager
func NewMultiNodeManager(basePath string, nodeNumber uint8) *MultiNodeManager {
	return &MultiNodeManager{
		BasePath:   basePath,
		NodeNumber: nodeNumber,
		localLocks: make(map[string]*os.File),
		lockFiles:  make(map[string]*os.File),
	}
}

// Initialize sets up the multi-node coordination system
func (mnm *MultiNodeManager) Initialize() error {
	// Create base directory
	if err := os.MkdirAll(mnm.BasePath, 0755); err != nil {
		return fmt.Errorf("failed to create multi-node directory: %w", err)
	}

	// Initialize coordination files
	coordFiles := []string{
		NodeStatusFile, SemaphoreFile, NodeMessageFile, ResourceLockFile,
		NodeConfigFile, SystemEventFile, NodeStatsFile, ActivityFile,
		ChatRoomFile, FileShareFile, ConfigSyncFile,
	}

	for _, filename := range coordFiles {
		path := filepath.Join(mnm.BasePath, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			file, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("failed to create coordination file %s: %w", filename, err)
			}
			file.Close()
		}
	}

	// Initialize node status
	if err := mnm.updateNodeStatus(NodeStatusOffline, 0, "", "Initializing"); err != nil {
		return fmt.Errorf("failed to initialize node status: %w", err)
	}

	// Start status update timer
	mnm.statusTimer = time.NewTimer(NodeStatusUpdateInterval * time.Second)
	go mnm.statusUpdateLoop()

	// Initialize deadlock detection
	go mnm.deadlockDetectionLoop()

	return nil
}

// UpdateNodeStatus updates the current node's status
func (mnm *MultiNodeManager) UpdateNodeStatus(status uint8, userID uint16, userName, activity string) error {
	return mnm.updateNodeStatus(status, userID, userName, activity)
}

func (mnm *MultiNodeManager) updateNodeStatus(status uint8, userID uint16, userName, activity string) error {
	mnm.mutex.Lock()
	defer mnm.mutex.Unlock()

	statusPath := filepath.Join(mnm.BasePath, NodeStatusFile)

	// Read existing status records
	var records []NodeStatus
	if file, err := os.Open(statusPath); err == nil {
		defer file.Close()
		for {
			var record NodeStatus
			if err := binary.Read(file, binary.LittleEndian, &record); err != nil {
				break // EOF
			}
			records = append(records, record)
		}
	}

	// Update or add our node's status
	found := false
	for i := range records {
		if records[i].NodeNum == mnm.NodeNumber {
			records[i].Status = status
			records[i].UserID = userID
			copy(records[i].UserName[:], userName)
			copy(records[i].Activity[:], activity)
			copy(records[i].LastUpdate[:], time.Now().Format("2006-01-02 15:04:05"))
			found = true
			break
		}
	}

	if !found {
		newRecord := NodeStatus{
			NodeNum: mnm.NodeNumber,
			Status:  status,
			UserID:  userID,
		}
		copy(newRecord.UserName[:], userName)
		copy(newRecord.Activity[:], activity)
		copy(newRecord.ConnectTime[:], time.Now().Format("2006-01-02 15:04:05"))
		copy(newRecord.LastUpdate[:], time.Now().Format("2006-01-02 15:04:05"))
		records = append(records, newRecord)
	}

	// Write updated records
	file, err := os.Create(statusPath)
	if err != nil {
		return fmt.Errorf("failed to create status file: %w", err)
	}
	defer file.Close()

	for _, record := range records {
		if err := binary.Write(file, binary.LittleEndian, record); err != nil {
			return fmt.Errorf("failed to write status record: %w", err)
		}
	}

	return nil
}

// GetNodeStatus returns status information for all nodes
func (mnm *MultiNodeManager) GetNodeStatus() ([]NodeStatus, error) {
	statusPath := filepath.Join(mnm.BasePath, NodeStatusFile)
	file, err := os.Open(statusPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []NodeStatus
	for {
		var record NodeStatus
		if err := binary.Read(file, binary.LittleEndian, &record); err != nil {
			break // EOF
		}
		records = append(records, record)
	}

	return records, nil
}

// AcquireResourceLock attempts to acquire a lock on a resource
func (mnm *MultiNodeManager) AcquireResourceLock(resource string, lockType uint8, timeout time.Duration) error {
	lockKey := fmt.Sprintf("%s:%d", resource, lockType)
	
	// Check if we already have this lock locally
	mnm.mutex.Lock()
	if _, exists := mnm.localLocks[lockKey]; exists {
		mnm.mutex.Unlock()
		return nil // Already have the lock
	}
	mnm.mutex.Unlock()

	start := time.Now()
	for time.Since(start) < timeout {
		if mnm.isShutdown {
			return ErrSystemShutdown
		}

		if err := mnm.tryAcquireLock(resource, lockType, lockKey); err == nil {
			return nil
		}

		// Check for deadlocks
		if err := mnm.checkDeadlock(resource, lockType); err != nil {
			return err
		}

		time.Sleep(100 * time.Millisecond)
	}

	return ErrLockTimeout
}

func (mnm *MultiNodeManager) tryAcquireLock(resource string, lockType uint8, lockKey string) error {
	lockPath := filepath.Join(mnm.BasePath, ResourceLockFile)
	
	// Try to acquire file lock on the lock file itself
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}

	// Use file locking for atomic operations
	if err := mnm.lockFile(lockFile); err != nil {
		lockFile.Close()
		return ErrResourceBusy
	}
	defer mnm.unlockFile(lockFile)

	// Read existing locks
	locks, err := mnm.readResourceLocks(lockFile)
	if err != nil {
		lockFile.Close()
		return err
	}

	// Check if resource is already locked in a conflicting way
	for _, lock := range locks {
		lockResource := string(lock.Resource[:])
		if lockResource == resource {
			// Check for conflicts
			if lockType == LockTypeWrite || lock.LockType == LockTypeWrite {
				if lock.NodeNum != mnm.NodeNumber {
					lockFile.Close()
					return ErrResourceBusy
				}
			}
		}
	}

	// Create new lock record
	newLock := ResourceLock{
		LockID:   uint32(time.Now().Unix()),
		NodeNum:  mnm.NodeNumber,
		LockType: lockType,
	}
	copy(newLock.Resource[:], resource)
	copy(newLock.Timestamp[:], time.Now().Format("2006-01-02 15:04:05"))

	locks = append(locks, newLock)

	// Write updated locks
	if err := mnm.writeResourceLocks(lockFile, locks); err != nil {
		lockFile.Close()
		return err
	}

	// Store local reference
	mnm.mutex.Lock()
	mnm.localLocks[lockKey] = lockFile
	mnm.mutex.Unlock()

	return nil
}

// ReleaseResourceLock releases a previously acquired lock
func (mnm *MultiNodeManager) ReleaseResourceLock(resource string, lockType uint8) error {
	lockKey := fmt.Sprintf("%s:%d", resource, lockType)

	mnm.mutex.Lock()
	lockFile, exists := mnm.localLocks[lockKey]
	if !exists {
		mnm.mutex.Unlock()
		return nil // Lock not held
	}
	delete(mnm.localLocks, lockKey)
	mnm.mutex.Unlock()

	// Read existing locks
	locks, err := mnm.readResourceLocks(lockFile)
	if err != nil {
		lockFile.Close()
		return err
	}

	// Remove our lock
	var updatedLocks []ResourceLock
	for _, lock := range locks {
		lockResource := string(lock.Resource[:])
		if !(lockResource == resource && lock.LockType == lockType && lock.NodeNum == mnm.NodeNumber) {
			updatedLocks = append(updatedLocks, lock)
		}
	}

	// Write updated locks
	if err := mnm.writeResourceLocks(lockFile, updatedLocks); err != nil {
		lockFile.Close()
		return err
	}

	mnm.unlockFile(lockFile)
	lockFile.Close()

	return nil
}

// CreateSemaphore creates a system-wide semaphore for critical operations
func (mnm *MultiNodeManager) CreateSemaphore(semaphoreID uint32, operation, resource string, maxDuration time.Duration, exclusive bool) error {
	semPath := filepath.Join(mnm.BasePath, SemaphoreFile)
	
	file, err := os.OpenFile(semPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open semaphore file: %w", err)
	}
	defer file.Close()

	if err := mnm.lockFile(file); err != nil {
		return ErrResourceBusy
	}
	defer mnm.unlockFile(file)

	// Read existing semaphores
	semaphores, err := mnm.readSemaphores(file)
	if err != nil {
		return err
	}

	// Check if semaphore already exists
	for _, sem := range semaphores {
		if sem.SemaphoreID == semaphoreID {
			return ErrSemaphoreExists
		}
	}

	// Create new semaphore
	newSem := SystemSemaphore{
		SemaphoreID: semaphoreID,
		NodeNum:     mnm.NodeNumber,
		MaxDuration: uint32(maxDuration.Seconds()),
		ProcessID:   uint32(os.Getpid()),
	}
	copy(newSem.Operation[:], operation)
	copy(newSem.Resource[:], resource)
	copy(newSem.StartTime[:], time.Now().Format("2006-01-02 15:04:05"))
	
	if exclusive {
		newSem.Exclusive = 1
	}

	semaphores = append(semaphores, newSem)

	return mnm.writeSemaphores(file, semaphores)
}

// ReleaseSemaphore removes a system semaphore
func (mnm *MultiNodeManager) ReleaseSemaphore(semaphoreID uint32) error {
	semPath := filepath.Join(mnm.BasePath, SemaphoreFile)
	
	file, err := os.OpenFile(semPath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := mnm.lockFile(file); err != nil {
		return ErrResourceBusy
	}
	defer mnm.unlockFile(file)

	// Read existing semaphores
	semaphores, err := mnm.readSemaphores(file)
	if err != nil {
		return err
	}

	// Remove our semaphore
	var updatedSems []SystemSemaphore
	for _, sem := range semaphores {
		if !(sem.SemaphoreID == semaphoreID && sem.NodeNum == mnm.NodeNumber) {
			updatedSems = append(updatedSems, sem)
		}
	}

	return mnm.writeSemaphores(file, updatedSems)
}

// SendNodeMessage sends a message to another node or broadcasts to all nodes
func (mnm *MultiNodeManager) SendNodeMessage(toNode uint8, msgType uint8, subject, body string) error {
	msgPath := filepath.Join(mnm.BasePath, NodeMessageFile)
	
	file, err := os.OpenFile(msgPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open message file: %w", err)
	}
	defer file.Close()

	message := NodeMessage{
		MessageID:   uint32(time.Now().UnixNano()),
		FromNode:    mnm.NodeNumber,
		ToNode:      toNode,
		MessageType: msgType,
		Priority:    5, // Normal priority
	}
	copy(message.Timestamp[:], time.Now().Format("2006-01-02 15:04:05"))
	copy(message.Subject[:], subject)
	copy(message.Body[:], body)

	return binary.Write(file, binary.LittleEndian, message)
}

// GetNodeMessages retrieves messages for this node
func (mnm *MultiNodeManager) GetNodeMessages() ([]NodeMessage, error) {
	msgPath := filepath.Join(mnm.BasePath, NodeMessageFile)
	
	file, err := os.Open(msgPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var messages []NodeMessage
	for {
		var msg NodeMessage
		if err := binary.Read(file, binary.LittleEndian, &msg); err != nil {
			break // EOF
		}

		// Include messages addressed to this node or broadcasts
		if msg.ToNode == mnm.NodeNumber || msg.ToNode == 0 {
			messages = append(messages, msg)
		}
	}

	return messages, nil
}

// LogSystemEvent logs a system event
func (mnm *MultiNodeManager) LogSystemEvent(eventType uint8, severity uint8, userID uint16, category, description, data string) error {
	eventPath := filepath.Join(mnm.BasePath, SystemEventFile)
	
	file, err := os.OpenFile(eventPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open event file: %w", err)
	}
	defer file.Close()

	event := SystemEvent{
		EventID:   uint32(time.Now().UnixNano()),
		EventType: eventType,
		Severity:  severity,
		NodeNum:   mnm.NodeNumber,
		UserID:    userID,
	}
	copy(event.Timestamp[:], time.Now().Format("2006-01-02 15:04:05"))
	copy(event.Category[:], category)
	copy(event.Description[:], description)
	copy(event.Data[:], data)

	return binary.Write(file, binary.LittleEndian, event)
}

// GetActiveNodes returns a list of currently active nodes
func (mnm *MultiNodeManager) GetActiveNodes() ([]uint8, error) {
	statuses, err := mnm.GetNodeStatus()
	if err != nil {
		return nil, err
	}

	var activeNodes []uint8
	for _, status := range statuses {
		if status.Status != NodeStatusOffline {
			activeNodes = append(activeNodes, status.NodeNum)
		}
	}

	return activeNodes, nil
}

// IsNodeOnline checks if a specific node is online
func (mnm *MultiNodeManager) IsNodeOnline(nodeNum uint8) (bool, error) {
	statuses, err := mnm.GetNodeStatus()
	if err != nil {
		return false, err
	}

	for _, status := range statuses {
		if status.NodeNum == nodeNum {
			return status.Status != NodeStatusOffline, nil
		}
	}

	return false, ErrNodeNotFound
}

// BroadcastShutdown sends a shutdown notification to all nodes
func (mnm *MultiNodeManager) BroadcastShutdown(delaySeconds int) error {
	message := fmt.Sprintf("System shutdown in %d seconds", delaySeconds)
	return mnm.SendNodeMessage(0, NodeMsgEmergency, "SYSTEM SHUTDOWN", message)
}

// Shutdown gracefully shuts down the multi-node manager
func (mnm *MultiNodeManager) Shutdown() error {
	mnm.mutex.Lock()
	mnm.isShutdown = true
	mnm.mutex.Unlock()

	// Stop status updates
	if mnm.statusTimer != nil {
		mnm.statusTimer.Stop()
	}

	// Release all local locks
	for lockKey, lockFile := range mnm.localLocks {
		mnm.unlockFile(lockFile)
		lockFile.Close()
		delete(mnm.localLocks, lockKey)
	}

	// Update node status to offline
	mnm.updateNodeStatus(NodeStatusOffline, 0, "", "Shutting down")

	// Log shutdown event
	mnm.LogSystemEvent(EventSystemDown, 5, 0, "SYSTEM", "Node shutdown", "")

	return nil
}

// Helper functions

func (mnm *MultiNodeManager) statusUpdateLoop() {
	for !mnm.isShutdown {
		select {
		case <-mnm.statusTimer.C:
			// Update timestamp to show we're still alive
			mnm.updateNodeStatus(NodeStatusWaiting, 0, "", "Periodic update")
			mnm.statusTimer.Reset(NodeStatusUpdateInterval * time.Second)
		}
	}
}

func (mnm *MultiNodeManager) deadlockDetectionLoop() {
	ticker := time.NewTicker(DeadlockCheckInterval * time.Second)
	defer ticker.Stop()

	for !mnm.isShutdown {
		select {
		case <-ticker.C:
			mnm.detectDeadlocks()
		}
	}
}

func (mnm *MultiNodeManager) detectDeadlocks() {
	// Simple deadlock detection - check for circular wait conditions
	// This is a simplified implementation
	lockPath := filepath.Join(mnm.BasePath, ResourceLockFile)
	
	file, err := os.Open(lockPath)
	if err != nil {
		return
	}
	defer file.Close()

	locks, err := mnm.readResourceLocks(file)
	if err != nil {
		return
	}

	// Group locks by node
	nodeResources := make(map[uint8][]string)
	for _, lock := range locks {
		resource := string(lock.Resource[:])
		nodeResources[lock.NodeNum] = append(nodeResources[lock.NodeNum], resource)
	}

	// Check for nodes holding multiple resources (potential deadlock)
	for nodeNum, resources := range nodeResources {
		if len(resources) > 1 && nodeNum != mnm.NodeNumber {
			mnm.LogSystemEvent(EventWarning, 7, 0, "DEADLOCK", 
				fmt.Sprintf("Node %d holds multiple resources", nodeNum), "")
		}
	}
}

func (mnm *MultiNodeManager) checkDeadlock(resource string, lockType uint8) error {
	// Check if we're creating a potential deadlock
	// This is a simplified check - a full implementation would be more sophisticated
	return nil
}

func (mnm *MultiNodeManager) lockFile(file *os.File) error {
	// Use flock for file locking (Unix-style)
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func (mnm *MultiNodeManager) unlockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

func (mnm *MultiNodeManager) readResourceLocks(file *os.File) ([]ResourceLock, error) {
	file.Seek(0, 0)
	var locks []ResourceLock
	
	for {
		var lock ResourceLock
		if err := binary.Read(file, binary.LittleEndian, &lock); err != nil {
			break // EOF
		}
		locks = append(locks, lock)
	}

	return locks, nil
}

func (mnm *MultiNodeManager) writeResourceLocks(file *os.File, locks []ResourceLock) error {
	file.Seek(0, 0)
	file.Truncate(0)

	for _, lock := range locks {
		if err := binary.Write(file, binary.LittleEndian, lock); err != nil {
			return err
		}
	}

	return nil
}

func (mnm *MultiNodeManager) readSemaphores(file *os.File) ([]SystemSemaphore, error) {
	file.Seek(0, 0)
	var semaphores []SystemSemaphore
	
	for {
		var sem SystemSemaphore
		if err := binary.Read(file, binary.LittleEndian, &sem); err != nil {
			break // EOF
		}
		semaphores = append(semaphores, sem)
	}

	return semaphores, nil
}

func (mnm *MultiNodeManager) writeSemaphores(file *os.File, semaphores []SystemSemaphore) error {
	file.Seek(0, 0)
	file.Truncate(0)

	for _, sem := range semaphores {
		if err := binary.Write(file, binary.LittleEndian, sem); err != nil {
			return err
		}
	}

	return nil
}

// UpdateActivity updates the current activity for real-time monitoring
func (mnm *MultiNodeManager) UpdateActivity(userID uint16, activity uint8, location string) error {
	actPath := filepath.Join(mnm.BasePath, ActivityFile)
	
	// Read existing activities
	var activities []ActivityMonitor
	if file, err := os.Open(actPath); err == nil {
		defer file.Close()
		for {
			var act ActivityMonitor
			if err := binary.Read(file, binary.LittleEndian, &act); err != nil {
				break
			}
			activities = append(activities, act)
		}
	}

	// Update or add our activity
	found := false
	for i := range activities {
		if activities[i].NodeNum == mnm.NodeNumber {
			activities[i].UserID = userID
			activities[i].Activity = activity
			copy(activities[i].Location[:], location)
			copy(activities[i].LastInput[:], time.Now().Format("2006-01-02 15:04:05"))
			found = true
			break
		}
	}

	if !found {
		newActivity := ActivityMonitor{
			NodeNum:  mnm.NodeNumber,
			UserID:   userID,
			Activity: activity,
		}
		copy(newActivity.Location[:], location)
		copy(newActivity.StartTime[:], time.Now().Format("2006-01-02 15:04:05"))
		copy(newActivity.LastInput[:], time.Now().Format("2006-01-02 15:04:05"))
		activities = append(activities, newActivity)
	}

	// Write updated activities
	file, err := os.Create(actPath)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, act := range activities {
		if err := binary.Write(file, binary.LittleEndian, act); err != nil {
			return err
		}
	}

	return nil
}

// GetSystemLoad returns current system load information
func (mnm *MultiNodeManager) GetSystemLoad() (int, int, error) {
	statuses, err := mnm.GetNodeStatus()
	if err != nil {
		return 0, 0, err
	}

	activeNodes := 0
	totalUsers := 0

	for _, status := range statuses {
		if status.Status != NodeStatusOffline {
			activeNodes++
			if status.UserID > 0 {
				totalUsers++
			}
		}
	}

	return activeNodes, totalUsers, nil
}