package doors

import (
	"fmt"
	"sync"
	"time"
	"errors"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"os"
)

var (
	ErrResourceNotFound     = errors.New("resource not found")
	ErrResourceLocked      = errors.New("resource already locked")
	ErrLockNotFound        = errors.New("lock not found")
	ErrInvalidLockMode     = errors.New("invalid lock mode")
	ErrLockTimeout         = errors.New("lock timeout")
	ErrMaxLocksExceeded    = errors.New("maximum locks exceeded")
	ErrIncompatibleLock    = errors.New("incompatible lock mode")
)

// ResourceManager manages door resources and locking
type ResourceManager struct {
	resources map[string]*DoorResource
	locks     map[string]*ResourceLock
	mu        sync.RWMutex
	config    *ResourceManagerConfig
}

// ResourceManagerConfig defines configuration for resource management
type ResourceManagerConfig struct {
	DefaultTimeout    time.Duration `json:"default_timeout"`     // Default lock timeout
	MaxLockTime       time.Duration `json:"max_lock_time"`       // Maximum lock time
	CleanupInterval   time.Duration `json:"cleanup_interval"`    // Cleanup interval
	DeadlockDetection bool          `json:"deadlock_detection"`  // Enable deadlock detection
	LogResourceAccess bool          `json:"log_resource_access"` // Log resource access
	ResourceDataPath  string        `json:"resource_data_path"`  // Path for resource data
}

// NewResourceManager creates a new resource manager
func NewResourceManager(config *ResourceManagerConfig) *ResourceManager {
	if config == nil {
		config = &ResourceManagerConfig{
			DefaultTimeout:    time.Minute * 5,
			MaxLockTime:       time.Hour,
			CleanupInterval:   time.Minute,
			DeadlockDetection: true,
			LogResourceAccess: true,
			ResourceDataPath:  "/tmp/vision3/resources",
		}
	}
	
	rm := &ResourceManager{
		resources: make(map[string]*DoorResource),
		locks:     make(map[string]*ResourceLock),
		config:    config,
	}
	
	// Start cleanup goroutine
	go rm.cleanupRoutine()
	
	return rm
}

// RegisterResource registers a new resource for management
func (rm *ResourceManager) RegisterResource(resource *DoorResource) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	if resource.ID == "" {
		return errors.New("resource ID cannot be empty")
	}
	
	// Initialize resource if needed
	if resource.Locks == nil {
		resource.Locks = make([]ResourceLock, 0)
	}
	
	if resource.MaxLocks <= 0 {
		resource.MaxLocks = 1 // Default to exclusive access
	}
	
	rm.resources[resource.ID] = resource
	return nil
}

// UnregisterResource removes a resource from management
func (rm *ResourceManager) UnregisterResource(resourceID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	resource, exists := rm.resources[resourceID]
	if !exists {
		return ErrResourceNotFound
	}
	
	// Check if resource has active locks
	if resource.CurrentLocks > 0 {
		return errors.New("cannot unregister resource with active locks")
	}
	
	delete(rm.resources, resourceID)
	return nil
}

// AcquireLock attempts to acquire a lock on a resource
func (rm *ResourceManager) AcquireLock(resourceID, doorID, instanceID string, nodeID, userID int, mode LockMode, timeout time.Duration) (*ResourceLock, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	resource, exists := rm.resources[resourceID]
	if !exists {
		return nil, ErrResourceNotFound
	}
	
	// Check if lock is compatible with existing locks
	if !rm.isLockCompatible(resource, mode) {
		return nil, ErrIncompatibleLock
	}
	
	// Check maximum locks
	if resource.CurrentLocks >= resource.MaxLocks && mode != LockModeShared {
		return nil, ErrMaxLocksExceeded
	}
	
	// Create lock
	lockID := rm.generateLockID()
	if timeout == 0 {
		timeout = rm.config.DefaultTimeout
	}
	if timeout > rm.config.MaxLockTime {
		timeout = rm.config.MaxLockTime
	}
	
	lock := &ResourceLock{
		ID:         lockID,
		DoorID:     doorID,
		InstanceID: instanceID,
		NodeID:     nodeID,
		UserID:     userID,
		LockTime:   time.Now(),
		LockMode:   mode,
		Timeout:    timeout,
		RefCount:   1,
	}
	
	// Add lock to resource and global locks map
	resource.Locks = append(resource.Locks, *lock)
	resource.CurrentLocks++
	rm.locks[lockID] = lock
	
	if rm.config.LogResourceAccess {
		rm.logResourceAccess("ACQUIRE", resourceID, lockID, doorID, nodeID, userID)
	}
	
	return lock, nil
}

// ReleaseLock releases a lock on a resource
func (rm *ResourceManager) ReleaseLock(lockID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	lock, exists := rm.locks[lockID]
	if !exists {
		return ErrLockNotFound
	}
	
	// Find the resource
	var targetResource *DoorResource
	var lockIndex int = -1
	
	for _, resource := range rm.resources {
		for i, resLock := range resource.Locks {
			if resLock.ID == lockID {
				targetResource = resource
				lockIndex = i
				break
			}
		}
		if targetResource != nil {
			break
		}
	}
	
	if targetResource == nil {
		return ErrResourceNotFound
	}
	
	// Remove lock from resource
	targetResource.Locks = append(targetResource.Locks[:lockIndex], targetResource.Locks[lockIndex+1:]...)
	targetResource.CurrentLocks--
	
	// Remove from global locks map
	delete(rm.locks, lockID)
	
	if rm.config.LogResourceAccess {
		rm.logResourceAccess("RELEASE", targetResource.ID, lockID, lock.DoorID, lock.NodeID, lock.UserID)
	}
	
	return nil
}

// GetResourceStatus returns the current status of a resource
func (rm *ResourceManager) GetResourceStatus(resourceID string) (*DoorResource, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	resource, exists := rm.resources[resourceID]
	if !exists {
		return nil, ErrResourceNotFound
	}
	
	// Return a copy to prevent external modifications
	resourceCopy := *resource
	resourceCopy.Locks = make([]ResourceLock, len(resource.Locks))
	copy(resourceCopy.Locks, resource.Locks)
	
	return &resourceCopy, nil
}

// GetAllResources returns all registered resources
func (rm *ResourceManager) GetAllResources() map[string]*DoorResource {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	result := make(map[string]*DoorResource)
	for id, resource := range rm.resources {
		resourceCopy := *resource
		resourceCopy.Locks = make([]ResourceLock, len(resource.Locks))
		copy(resourceCopy.Locks, resource.Locks)
		result[id] = &resourceCopy
	}
	
	return result
}

// GetLockInfo returns information about a specific lock
func (rm *ResourceManager) GetLockInfo(lockID string) (*ResourceLock, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	lock, exists := rm.locks[lockID]
	if !exists {
		return nil, ErrLockNotFound
	}
	
	// Return a copy
	lockCopy := *lock
	return &lockCopy, nil
}

// GetLocksForDoor returns all locks held by a specific door
func (rm *ResourceManager) GetLocksForDoor(doorID string) []*ResourceLock {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	var result []*ResourceLock
	for _, lock := range rm.locks {
		if lock.DoorID == doorID {
			lockCopy := *lock
			result = append(result, &lockCopy)
		}
	}
	
	return result
}

// GetLocksForNode returns all locks held by a specific node
func (rm *ResourceManager) GetLocksForNode(nodeID int) []*ResourceLock {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	var result []*ResourceLock
	for _, lock := range rm.locks {
		if lock.NodeID == nodeID {
			lockCopy := *lock
			result = append(result, &lockCopy)
		}
	}
	
	return result
}

// CleanupExpiredLocks removes expired locks
func (rm *ResourceManager) CleanupExpiredLocks() int {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	now := time.Now()
	cleanedCount := 0
	
	for lockID, lock := range rm.locks {
		if now.Sub(lock.LockTime) > lock.Timeout {
			// Find and remove from resource
			for _, resource := range rm.resources {
				for i, resLock := range resource.Locks {
					if resLock.ID == lockID {
						resource.Locks = append(resource.Locks[:i], resource.Locks[i+1:]...)
						resource.CurrentLocks--
						break
					}
				}
			}
			
			delete(rm.locks, lockID)
			cleanedCount++
			
			if rm.config.LogResourceAccess {
				rm.logResourceAccess("EXPIRE", "", lockID, lock.DoorID, lock.NodeID, lock.UserID)
			}
		}
	}
	
	return cleanedCount
}

// DetectDeadlocks detects potential deadlock situations
func (rm *ResourceManager) DetectDeadlocks() []DeadlockInfo {
	if !rm.config.DeadlockDetection {
		return nil
	}
	
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	var deadlocks []DeadlockInfo
	
	// Simple deadlock detection: look for circular wait conditions
	// This is a basic implementation; more sophisticated algorithms exist
	doorResources := make(map[string][]string) // door -> resources it holds
	
	// Build dependency graph
	for _, lock := range rm.locks {
		doorResources[lock.DoorID] = append(doorResources[lock.DoorID], lock.ID)
	}
	
	// Check for cycles (simplified)
	for doorA, resourcesA := range doorResources {
		for doorB, resourcesB := range doorResources {
			if doorA != doorB && rm.hasCircularDependency(doorA, resourcesA, doorB, resourcesB) {
				deadlock := DeadlockInfo{
					ID:        rm.generateLockID(),
					Timestamp: time.Now(),
					Doors:     []string{doorA, doorB},
					Resources: append(resourcesA, resourcesB...),
					Severity:  SeverityHigh,
				}
				deadlocks = append(deadlocks, deadlock)
			}
		}
	}
	
	return deadlocks
}

// ForceReleaseLocks forcibly releases all locks for a door/instance
func (rm *ResourceManager) ForceReleaseLocks(doorID, instanceID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	var locksToRelease []string
	
	for lockID, lock := range rm.locks {
		if (doorID != "" && lock.DoorID == doorID) || 
		   (instanceID != "" && lock.InstanceID == instanceID) {
			locksToRelease = append(locksToRelease, lockID)
		}
	}
	
	for _, lockID := range locksToRelease {
		lock := rm.locks[lockID]
		
		// Find and remove from resource
		for _, resource := range rm.resources {
			for i, resLock := range resource.Locks {
				if resLock.ID == lockID {
					resource.Locks = append(resource.Locks[:i], resource.Locks[i+1:]...)
					resource.CurrentLocks--
					break
				}
			}
		}
		
		delete(rm.locks, lockID)
		
		if rm.config.LogResourceAccess {
			rm.logResourceAccess("FORCE_RELEASE", "", lockID, lock.DoorID, lock.NodeID, lock.UserID)
		}
	}
	
	return nil
}

// Auto-discovery methods for common door resources
func (rm *ResourceManager) AutoDiscoverFileResources(basePath string) error {
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip hidden files and directories
		if filepath.Base(path)[0] == '.' {
			return nil
		}
		
		// Create resource for important files
		ext := filepath.Ext(path)
		if rm.isImportantFileType(ext) || info.IsDir() {
			resourceType := ResourceTypeFile
			if info.IsDir() {
				resourceType = ResourceTypeDirectory
			}
			
			resource := &DoorResource{
				ID:          path,
				Type:        resourceType,
				Path:        path,
				Description: fmt.Sprintf("Auto-discovered %s", filepath.Base(path)),
				MaxLocks:    rm.getDefaultMaxLocks(ext),
				LockMode:    rm.getDefaultLockMode(ext),
				Locks:       make([]ResourceLock, 0),
			}
			
			rm.RegisterResource(resource)
		}
		
		return nil
	})
	
	return err
}

// Helper methods

func (rm *ResourceManager) isLockCompatible(resource *DoorResource, newMode LockMode) bool {
	if resource.CurrentLocks == 0 {
		return true
	}
	
	// Check compatibility with existing locks
	for _, lock := range resource.Locks {
		if !rm.areModesCompatible(lock.LockMode, newMode) {
			return false
		}
	}
	
	return true
}

func (rm *ResourceManager) areModesCompatible(mode1, mode2 LockMode) bool {
	// Shared locks are compatible with other shared locks
	if mode1 == LockModeShared && mode2 == LockModeShared {
		return true
	}
	
	// Read-only locks are compatible with other read-only locks
	if mode1 == LockModeReadOnly && mode2 == LockModeReadOnly {
		return true
	}
	
	// All other combinations are incompatible
	return false
}

func (rm *ResourceManager) generateLockID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (rm *ResourceManager) logResourceAccess(action, resourceID, lockID, doorID string, nodeID, userID int) {
	// TODO: Implement proper logging
	fmt.Printf("[RESOURCE] %s: Resource=%s Lock=%s Door=%s Node=%d User=%d\n", 
		action, resourceID, lockID, doorID, nodeID, userID)
}

func (rm *ResourceManager) hasCircularDependency(doorA string, resourcesA []string, doorB string, resourcesB []string) bool {
	// Simple check: if doorA holds resources that doorB needs and vice versa
	// This is a simplified deadlock detection algorithm
	
	aHoldsForB := false
	bHoldsForA := false
	
	for _, resA := range resourcesA {
		for _, resB := range resourcesB {
			if resA == resB {
				aHoldsForB = true
				break
			}
		}
	}
	
	for _, resB := range resourcesB {
		for _, resA := range resourcesA {
			if resB == resA {
				bHoldsForA = true
				break
			}
		}
	}
	
	return aHoldsForB && bHoldsForA
}

func (rm *ResourceManager) isImportantFileType(ext string) bool {
	importantExts := map[string]bool{
		".dat": true, ".idx": true, ".cfg": true, ".ini": true,
		".db": true, ".dbf": true, ".log": true, ".txt": true,
		".scr": true, ".ans": true, ".asc": true,
	}
	return importantExts[ext]
}

func (rm *ResourceManager) getDefaultMaxLocks(ext string) int {
	switch ext {
	case ".log", ".txt":
		return 10 // Log files can have multiple readers
	case ".cfg", ".ini":
		return 5  // Config files can have some shared access
	default:
		return 1  // Most files should be exclusive
	}
}

func (rm *ResourceManager) getDefaultLockMode(ext string) LockMode {
	switch ext {
	case ".log":
		return LockModeShared // Log files are usually append-only
	case ".txt", ".ans", ".asc":
		return LockModeReadOnly // Text files are usually read-only
	default:
		return LockModeExclusive // Default to exclusive access
	}
}

func (rm *ResourceManager) cleanupRoutine() {
	ticker := time.NewTicker(rm.config.CleanupInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		cleaned := rm.CleanupExpiredLocks()
		if cleaned > 0 && rm.config.LogResourceAccess {
			fmt.Printf("[RESOURCE] Cleaned up %d expired locks\n", cleaned)
		}
		
		// Check for deadlocks
		deadlocks := rm.DetectDeadlocks()
		if len(deadlocks) > 0 && rm.config.LogResourceAccess {
			fmt.Printf("[RESOURCE] Detected %d potential deadlocks\n", len(deadlocks))
		}
	}
}

// DeadlockInfo represents information about a detected deadlock
type DeadlockInfo struct {
	ID        string        `json:"id"`        // Deadlock ID
	Timestamp time.Time     `json:"timestamp"` // Detection time
	Doors     []string      `json:"doors"`     // Doors involved
	Resources []string      `json:"resources"` // Resources involved
	Severity  AlertSeverity `json:"severity"`  // Severity level
}

// ResourceStatistics holds statistics about resource usage
type ResourceStatistics struct {
	ResourceID     string        `json:"resource_id"`     // Resource ID
	TotalLocks     int64         `json:"total_locks"`     // Total locks acquired
	CurrentLocks   int           `json:"current_locks"`   // Current active locks
	AverageLockTime time.Duration `json:"average_lock_time"` // Average lock duration
	MaxConcurrent  int           `json:"max_concurrent"`  // Maximum concurrent locks
	ContentionCount int64        `json:"contention_count"` // Number of contentions
	TimeoutCount   int64         `json:"timeout_count"`   // Number of timeouts
	LastAccessed   time.Time     `json:"last_accessed"`   // Last access time
	MostUsedBy     string        `json:"most_used_by"`    // Door that uses it most
}

// GetResourceStatistics returns statistics for a resource
func (rm *ResourceManager) GetResourceStatistics(resourceID string) (*ResourceStatistics, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	resource, exists := rm.resources[resourceID]
	if !exists {
		return nil, ErrResourceNotFound
	}
	
	stats := &ResourceStatistics{
		ResourceID:   resourceID,
		CurrentLocks: resource.CurrentLocks,
		LastAccessed: time.Now(), // This should be tracked properly
	}
	
	// Calculate statistics from current locks
	if len(resource.Locks) > 0 {
		var totalDuration time.Duration
		for _, lock := range resource.Locks {
			totalDuration += time.Since(lock.LockTime)
		}
		stats.AverageLockTime = totalDuration / time.Duration(len(resource.Locks))
	}
	
	return stats, nil
}