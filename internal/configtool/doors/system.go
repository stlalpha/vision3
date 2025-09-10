package doors

import (
	"fmt"
	"path/filepath"
	
	"github.com/stlalpha/vision3/internal/configtool/nodes"
	"github.com/stlalpha/vision3/internal/user"
)

// DoorSystem represents the complete door system for Vision/3 BBS
type DoorSystem struct {
	// Core components
	Manager     DoorManager
	Coordinator *MultiNodeCoordinator
	Monitor     *DoorMonitor
	Tester      *DoorTester
	Integrator  *DoorIntegrator
	
	// Configuration
	Config      *DoorSystemConfig
	DataPath    string
	
	// State
	Initialized bool
}

// DoorSystemConfig contains configuration for the entire door system
type DoorSystemConfig struct {
	// Paths
	DataPath        string `json:"data_path"`         // Base data path
	ConfigPath      string `json:"config_path"`       // Configuration path
	TemplatesPath   string `json:"templates_path"`    // Templates path
	LogsPath        string `json:"logs_path"`         // Logs path
	
	// Features
	EnableMultiNode bool   `json:"enable_multinode"`  // Enable multi-node support
	EnableMonitoring bool  `json:"enable_monitoring"` // Enable monitoring
	EnableTesting   bool   `json:"enable_testing"`    // Enable testing
	EnableIntegration bool `json:"enable_integration"` // Enable BBS integration
	
	// Limits
	MaxDoors        int    `json:"max_doors"`         // Maximum doors
	MaxInstances    int    `json:"max_instances"`     // Maximum concurrent instances
	MaxNodes        int    `json:"max_nodes"`         // Maximum nodes
	
	// Behavior
	AutoBackup      bool   `json:"auto_backup"`       // Enable auto backup
	AutoTest        bool   `json:"auto_test"`         // Auto test doors on save
	AutoCleanup     bool   `json:"auto_cleanup"`      // Enable auto cleanup
}

// NewDoorSystem creates a new complete door system
func NewDoorSystem(config *DoorSystemConfig, nodeManager nodes.NodeManager, userManager *user.UserMgr) (*DoorSystem, error) {
	if config == nil {
		config = &DoorSystemConfig{
			DataPath:          "/opt/vision3/data/doors",
			ConfigPath:        "/opt/vision3/config/doors",
			TemplatesPath:     "/opt/vision3/templates/doors",
			LogsPath:          "/opt/vision3/logs/doors",
			EnableMultiNode:   true,
			EnableMonitoring:  true,
			EnableTesting:     true,
			EnableIntegration: true,
			MaxDoors:          100,
			MaxInstances:      50,
			MaxNodes:          10,
			AutoBackup:        true,
			AutoTest:          true,
			AutoCleanup:       true,
		}
	}
	
	system := &DoorSystem{
		Config:   config,
		DataPath: config.DataPath,
	}
	
	// Initialize components
	if err := system.initializeComponents(nodeManager, userManager); err != nil {
		return nil, fmt.Errorf("failed to initialize door system: %w", err)
	}
	
	return system, nil
}

// Initialize initializes the door system
func (ds *DoorSystem) Initialize() error {
	if ds.Initialized {
		return nil
	}
	
	// Initialize manager
	if managerImpl, ok := ds.Manager.(*DoorManagerImpl); ok {
		if err := managerImpl.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize door manager: %w", err)
		}
	}
	
	// Initialize monitoring
	if ds.Config.EnableMonitoring && ds.Monitor != nil {
		if err := ds.Monitor.Start(); err != nil {
			return fmt.Errorf("failed to start door monitor: %w", err)
		}
	}
	
	// Initialize integration
	if ds.Config.EnableIntegration && ds.Integrator != nil {
		if err := ds.Integrator.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize door integrator: %w", err)
		}
	}
	
	ds.Initialized = true
	return nil
}

// Shutdown shuts down the door system
func (ds *DoorSystem) Shutdown() error {
	if !ds.Initialized {
		return nil
	}
	
	// Shutdown integrator
	if ds.Integrator != nil {
		ds.Integrator.Shutdown()
	}
	
	// Shutdown monitor
	if ds.Monitor != nil {
		ds.Monitor.Stop()
	}
	
	// Shutdown coordinator
	if ds.Coordinator != nil {
		ds.Coordinator.Stop()
	}
	
	// Shutdown manager
	if managerImpl, ok := ds.Manager.(*DoorManagerImpl); ok {
		managerImpl.Shutdown()
	}
	
	ds.Initialized = false
	return nil
}

// GetSystemStatus returns the current system status
func (ds *DoorSystem) GetSystemStatus() *DoorSystemStatus {
	status := &DoorSystemStatus{
		Initialized:      ds.Initialized,
		ComponentStatus:  make(map[string]bool),
		ActiveInstances:  0,
		TotalDoors:      0,
		SystemHealth:    "Unknown",
	}
	
	// Check component status
	status.ComponentStatus["manager"] = ds.Manager != nil
	status.ComponentStatus["coordinator"] = ds.Coordinator != nil
	status.ComponentStatus["monitor"] = ds.Monitor != nil
	status.ComponentStatus["tester"] = ds.Tester != nil
	status.ComponentStatus["integrator"] = ds.Integrator != nil
	
	// Get door count
	if ds.Manager != nil {
		if doors, err := ds.Manager.GetAllDoorConfigs(); err == nil {
			status.TotalDoors = len(doors)
		}
		
		// Get active instances
		if instances, err := ds.Manager.GetActiveDoorInstances(); err == nil {
			status.ActiveInstances = len(instances)
		}
	}
	
	// Determine system health
	healthyComponents := 0
	totalComponents := len(status.ComponentStatus)
	
	for _, healthy := range status.ComponentStatus {
		if healthy {
			healthyComponents++
		}
	}
	
	if healthyComponents == totalComponents {
		status.SystemHealth = "Healthy"
	} else if healthyComponents > totalComponents/2 {
		status.SystemHealth = "Degraded"
	} else {
		status.SystemHealth = "Critical"
	}
	
	return status
}

// DoorSystemStatus represents the status of the door system
type DoorSystemStatus struct {
	Initialized     bool              `json:"initialized"`      // System initialized
	ComponentStatus map[string]bool   `json:"component_status"` // Status of each component
	ActiveInstances int               `json:"active_instances"` // Number of active instances
	TotalDoors      int               `json:"total_doors"`      // Total configured doors
	SystemHealth    string            `json:"system_health"`    // Overall system health
	LastUpdate      string            `json:"last_update"`      // Last status update
}

// initializeComponents initializes all door system components
func (ds *DoorSystem) initializeComponents(nodeManager nodes.NodeManager, userManager *user.UserMgr) error {
	// Initialize manager
	managerConfig := &DoorManagerConfig{
		DataPath:           ds.Config.DataPath,
		ConfigFile:         "doors.json",
		TemplatesPath:      ds.Config.TemplatesPath,
		BackupPath:         filepath.Join(ds.Config.DataPath, "backups"),
		EnableMonitoring:   ds.Config.EnableMonitoring,
		EnableTesting:      ds.Config.EnableTesting,
		TestOnSave:         ds.Config.AutoTest,
		ValidateOnLoad:     true,
	}
	
	ds.Manager = NewDoorManager(managerConfig)
	
	// Initialize resource manager for multi-node support
	var resourceManager *ResourceManager
	if ds.Config.EnableMultiNode {
		resourceConfig := &ResourceManagerConfig{
			DefaultTimeout:    300, // 5 minutes
			MaxLockTime:       3600, // 1 hour
			CleanupInterval:   60,   // 1 minute
			DeadlockDetection: true,
			LogResourceAccess: true,
		}
		resourceManager = NewResourceManager(resourceConfig)
	}
	
	// Initialize coordinator
	if ds.Config.EnableMultiNode {
		coordinatorConfig := &MultiNodeConfig{
			MaxNodes:            ds.Config.MaxNodes,
			QueueTimeout:        300,  // 5 minutes
			InstanceTimeout:     7200, // 2 hours
			NodeRotationEnabled: true,
			LoadBalancing:       true,
			FailoverEnabled:     true,
			SyncInterval:        30,   // 30 seconds
		}
		ds.Coordinator = NewMultiNodeCoordinator(coordinatorConfig, resourceManager)
	}
	
	// Initialize monitor
	if ds.Config.EnableMonitoring {
		monitorConfig := &MonitorConfig{
			UpdateInterval:    30,   // 30 seconds
			MetricsRetention:  168,  // 7 days in hours
			EnablePerformance: true,
			EnableResourceMon: true,
			EnableHealthCheck: true,
			LogPath:          ds.Config.LogsPath,
			ProcessTracking:  true,
		}
		ds.Monitor = NewDoorMonitor(monitorConfig)
	}
	
	// Initialize tester
	if ds.Config.EnableTesting {
		testConfig := &DoorTestConfig{
			TestTimeout:      300, // 5 minutes
			TestDataPath:     filepath.Join(ds.Config.DataPath, "test"),
			CleanupAfterTest: true,
			LogTestOutput:    true,
			TestTypes: []TestType{
				TestTypeBasic,
				TestTypeExecution,
				TestTypeDropFile,
				TestTypePermissions,
			},
		}
		ds.Tester = NewDoorTester(testConfig)
	}
	
	// Initialize integrator
	if ds.Config.EnableIntegration {
		ds.Integrator = NewDoorIntegrator(ds.Manager, nodeManager, userManager)
	}
	
	// Wire components together if manager is our implementation
	if managerImpl, ok := ds.Manager.(*DoorManagerImpl); ok {
		managerImpl.coordinator = ds.Coordinator
		managerImpl.resourceManager = resourceManager
		managerImpl.tester = ds.Tester
		managerImpl.monitor = ds.Monitor
	}
	
	return nil
}

// Utility functions for easy access to system features

// LaunchDoor launches a door with full system integration
func (ds *DoorSystem) LaunchDoor(doorID string, nodeID int, userID int) (*DoorInstance, error) {
	if ds.Integrator != nil {
		return ds.Integrator.LaunchDoorForUser(doorID, nodeID, userID)
	}
	return ds.Manager.LaunchDoor(doorID, nodeID, userID)
}

// CreateDoor creates a new door configuration
func (ds *DoorSystem) CreateDoor(config *DoorConfiguration) error {
	return ds.Manager.CreateDoorConfig(config)
}

// TestDoor tests a door configuration
func (ds *DoorSystem) TestDoor(doorID string) error {
	return ds.Manager.TestDoorConfig(doorID)
}

// GetDoorList returns a list of all configured doors
func (ds *DoorSystem) GetDoorList() ([]*DoorConfiguration, error) {
	return ds.Manager.GetAllDoorConfigs()
}

// GetActiveInstances returns all active door instances
func (ds *DoorSystem) GetActiveInstances() ([]*DoorInstance, error) {
	return ds.Manager.GetActiveDoorInstances()
}

// GetSystemMetrics returns system-wide door metrics
func (ds *DoorSystem) GetSystemMetrics() (*SystemMetrics, error) {
	if ds.Monitor != nil {
		return ds.Monitor.GetSystemMetrics(), nil
	}
	return nil, fmt.Errorf("monitoring not enabled")
}

// CreateWizard creates a door setup wizard
func (ds *DoorSystem) CreateWizard() *DoorWizard {
	return NewDoorWizard()
}

// CreateTUI creates a door configuration TUI
func (ds *DoorSystem) CreateTUI() *DoorConfigTUI {
	return NewDoorConfigTUI(ds.Manager, ds.Coordinator)
}

// ValidateSystem validates the entire door system configuration
func (ds *DoorSystem) ValidateSystem() []string {
	var issues []string
	
	// Check basic configuration
	if ds.Config.MaxDoors <= 0 {
		issues = append(issues, "MaxDoors must be greater than 0")
	}
	
	if ds.Config.MaxInstances <= 0 {
		issues = append(issues, "MaxInstances must be greater than 0")
	}
	
	if ds.Config.MaxNodes <= 0 {
		issues = append(issues, "MaxNodes must be greater than 0")
	}
	
	// Check component integration
	if ds.Integrator != nil {
		integrationIssues := ds.Integrator.ValidateIntegration()
		issues = append(issues, integrationIssues...)
	}
	
	// Check paths
	requiredPaths := []string{
		ds.Config.DataPath,
		ds.Config.ConfigPath,
		ds.Config.TemplatesPath,
		ds.Config.LogsPath,
	}
	
	for _, path := range requiredPaths {
		if path == "" {
			issues = append(issues, "Required path is empty")
		}
	}
	
	return issues
}

// BackupConfiguration creates a backup of the door system configuration
func (ds *DoorSystem) BackupConfiguration() error {
	return ds.Manager.BackupDoorConfigs()
}

// RestoreConfiguration restores the door system configuration from backup
func (ds *DoorSystem) RestoreConfiguration(backupPath string) error {
	return ds.Manager.RestoreDoorConfigs(backupPath)
}

// GetVersion returns the door system version information
func (ds *DoorSystem) GetVersion() string {
	return "Vision/3 Door System v1.0"
}

// Example usage and initialization function
func InitializeDoorSystem(nodeManager nodes.NodeManager, userManager *user.UserMgr) (*DoorSystem, error) {
	// Create door system with default configuration
	config := &DoorSystemConfig{
		DataPath:          "/opt/vision3/data/doors",
		ConfigPath:        "/opt/vision3/config/doors",
		TemplatesPath:     "/opt/vision3/templates/doors",
		LogsPath:          "/opt/vision3/logs/doors",
		EnableMultiNode:   true,
		EnableMonitoring:  true,
		EnableTesting:     true,
		EnableIntegration: true,
		MaxDoors:          100,
		MaxInstances:      50,
		MaxNodes:          10,
		AutoBackup:        true,
		AutoTest:          true,
		AutoCleanup:       true,
	}
	
	system, err := NewDoorSystem(config, nodeManager, userManager)
	if err != nil {
		return nil, err
	}
	
	// Initialize the system
	if err := system.Initialize(); err != nil {
		return nil, err
	}
	
	return system, nil
}