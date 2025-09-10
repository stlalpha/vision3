package doors

import (
	"time"
	"sync"
)

// DoorConfiguration represents a door game configuration
type DoorConfiguration struct {
	ID                   string                 `json:"id"`                     // Unique door identifier
	Name                 string                 `json:"name"`                   // Display name
	Description          string                 `json:"description"`            // Door description
	Category             DoorCategory           `json:"category"`               // Door category
	Version              string                 `json:"version"`                // Door version
	Author               string                 `json:"author"`                 // Door author
	
	// Execution settings
	Command              string                 `json:"command"`                // Executable path
	Arguments            []string               `json:"arguments"`              // Command line arguments
	WorkingDirectory     string                 `json:"working_directory"`      // Working directory
	EnvironmentVariables map[string]string      `json:"environment_variables"`  // Environment variables
	
	// Dropfile settings
	DropFileType         DropFileType           `json:"dropfile_type"`          // Type of dropfile to generate
	DropFileLocation     string                 `json:"dropfile_location"`      // Where to place dropfile
	CustomDropFile       *CustomDropFileConfig  `json:"custom_dropfile"`        // Custom dropfile configuration
	
	// I/O and interface settings
	IOMode               IOMode                 `json:"io_mode"`                // I/O mode (STDIO, SOCKET, etc.)
	RequiresRawTerminal  bool                   `json:"requires_raw_terminal"`  // Needs raw terminal mode
	TerminalType         string                 `json:"terminal_type"`          // Terminal type (ANSI, VT100, etc.)
	ScreenWidth          int                    `json:"screen_width"`           // Screen width
	ScreenHeight         int                    `json:"screen_height"`          // Screen height
	
	// Multi-node settings
	MultiNodeType        MultiNodeType          `json:"multinode_type"`         // Multi-node capability
	MaxInstances         int                    `json:"max_instances"`          // Maximum concurrent instances
	SharedResources      []string               `json:"shared_resources"`       // Shared resource files
	ExclusiveResources   []string               `json:"exclusive_resources"`    // Exclusive resource files
	NodeRotation         bool                   `json:"node_rotation"`          // Use round-robin node rotation
	
	// Time and access limits
	TimeLimit            int                    `json:"time_limit"`             // Time limit in minutes (0 = no limit)
	DailyTimeLimit       int                    `json:"daily_time_limit"`       // Daily time limit in minutes
	MinimumAccessLevel   int                    `json:"minimum_access_level"`   // Minimum user access level
	RequiredFlags        []string               `json:"required_flags"`         // Required user flags
	ForbiddenFlags       []string               `json:"forbidden_flags"`        // Forbidden user flags
	MaxUsersPerDay       int                    `json:"max_users_per_day"`      // Maximum users per day
	
	// Scheduling
	AvailableHours       []TimeSlot             `json:"available_hours"`        // When door is available
	MaintenanceHours     []TimeSlot             `json:"maintenance_hours"`      // Maintenance windows
	BlackoutDates        []BlackoutDate         `json:"blackout_dates"`         // Blackout dates
	
	// Advanced settings
	PreRunScript         string                 `json:"pre_run_script"`         // Script to run before door
	PostRunScript        string                 `json:"post_run_script"`        // Script to run after door
	CleanupScript        string                 `json:"cleanup_script"`         // Cleanup script
	CrashHandling        CrashHandlingConfig    `json:"crash_handling"`         // Crash handling settings
	Logging              LoggingConfig          `json:"logging"`                // Logging configuration
	
	// Statistics and monitoring
	Statistics           DoorStatistics         `json:"statistics"`             // Usage statistics
	Monitoring           MonitoringConfig       `json:"monitoring"`             // Monitoring settings
	
	// Configuration metadata
	Enabled              bool                   `json:"enabled"`                // Door is enabled
	Created              time.Time              `json:"created"`                // Creation time
	Modified             time.Time              `json:"modified"`               // Last modification time
	LastTested           time.Time              `json:"last_tested"`            // Last test time
	ConfigVersion        int                    `json:"config_version"`         // Configuration version
}

// DoorCategory represents door game categories
type DoorCategory int

const (
	CategoryUnknown DoorCategory = iota
	CategoryAction        // Action/Adventure games
	CategoryStrategy      // Strategy games
	CategoryRPG          // Role-playing games
	CategoryPuzzle       // Puzzle games
	CategoryCard         // Card games
	CategoryBoard        // Board games
	CategoryTrivia       // Trivia games
	CategoryUtility      // Utility programs
	CategoryChat         // Chat programs
	CategoryEditor       // Text editors
	CategoryBrowser      // File browsers
	CategoryMail         // Mail utilities
	CategorySystem       // System utilities
	CategoryAdult        // Adult content
	CategoryEducational  // Educational software
	CategorySports       // Sports games
	CategorySimulation   // Simulation games
	CategoryCustom       // Custom category
)

func (dc DoorCategory) String() string {
	switch dc {
	case CategoryAction:
		return "Action/Adventure"
	case CategoryStrategy:
		return "Strategy"
	case CategoryRPG:
		return "Role-Playing"
	case CategoryPuzzle:
		return "Puzzle"
	case CategoryCard:
		return "Card Games"
	case CategoryBoard:
		return "Board Games"
	case CategoryTrivia:
		return "Trivia"
	case CategoryUtility:
		return "Utilities"
	case CategoryChat:
		return "Chat"
	case CategoryEditor:
		return "Editors"
	case CategoryBrowser:
		return "Browsers"
	case CategoryMail:
		return "Mail"
	case CategorySystem:
		return "System"
	case CategoryAdult:
		return "Adult"
	case CategoryEducational:
		return "Educational"
	case CategorySports:
		return "Sports"
	case CategorySimulation:
		return "Simulation"
	case CategoryCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// DropFileType represents the type of dropfile to generate
type DropFileType int

const (
	DropFileNone DropFileType = iota
	DropFileDoorSys    // DOOR.SYS (most common)
	DropFileChainTxt   // CHAIN.TXT
	DropFileDorinfo1   // DORINFO1.DEF
	DropFileCallinfo   // CALLINFO.BBS
	DropFileUserinfo   // USERINFO.DAT
	DropFileModinfo    // MODINFO.DAT
	DropFileCustom     // Custom format
)

func (dt DropFileType) String() string {
	switch dt {
	case DropFileDoorSys:
		return "DOOR.SYS"
	case DropFileChainTxt:
		return "CHAIN.TXT"
	case DropFileDorinfo1:
		return "DORINFO1.DEF"
	case DropFileCallinfo:
		return "CALLINFO.BBS"
	case DropFileUserinfo:
		return "USERINFO.DAT"
	case DropFileModinfo:
		return "MODINFO.DAT"
	case DropFileCustom:
		return "Custom"
	default:
		return "None"
	}
}

// CustomDropFileConfig defines a custom dropfile format
type CustomDropFileConfig struct {
	FileName     string            `json:"file_name"`     // Custom dropfile name
	Template     string            `json:"template"`      // Template content
	Variables    map[string]string `json:"variables"`     // Variable mappings
	LineEnding   string            `json:"line_ending"`   // Line ending style
	Encoding     string            `json:"encoding"`      // File encoding
}

// IOMode represents the I/O mode for door communication
type IOMode int

const (
	IOModeStdio IOMode = iota
	IOModeSocket
	IOModeFossil
	IOModeSerial
	IOModeTelnet
	IOModeSSH
	IOModeCustom
)

func (io IOMode) String() string {
	switch io {
	case IOModeStdio:
		return "STDIO"
	case IOModeSocket:
		return "Socket"
	case IOModeFossil:
		return "FOSSIL"
	case IOModeSerial:
		return "Serial"
	case IOModeTelnet:
		return "Telnet"
	case IOModeSSH:
		return "SSH"
	case IOModeCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// MultiNodeType represents multi-node capabilities
type MultiNodeType int

const (
	MultiNodeSingle MultiNodeType = iota // Single user only
	MultiNodeShared                      // Multiple users, shared data
	MultiNodeExclusive                   // Multiple users, exclusive instances
	MultiNodeCoop                        // Cooperative multi-user
	MultiNodeCompetitive                 // Competitive multi-user
)

func (mn MultiNodeType) String() string {
	switch mn {
	case MultiNodeSingle:
		return "Single User"
	case MultiNodeShared:
		return "Shared Data"
	case MultiNodeExclusive:
		return "Exclusive Instances"
	case MultiNodeCoop:
		return "Cooperative"
	case MultiNodeCompetitive:
		return "Competitive"
	default:
		return "Unknown"
	}
}

// TimeSlot represents a time period
type TimeSlot struct {
	StartTime   time.Time `json:"start_time"`   // Start time
	EndTime     time.Time `json:"end_time"`     // End time
	DaysOfWeek  []int     `json:"days_of_week"` // Days of week (0=Sunday)
	Description string    `json:"description"`  // Description
}

// BlackoutDate represents a date when door is unavailable
type BlackoutDate struct {
	Date        time.Time `json:"date"`        // Blackout date
	StartTime   time.Time `json:"start_time"`  // Start time (optional)
	EndTime     time.Time `json:"end_time"`    // End time (optional)
	Reason      string    `json:"reason"`      // Reason for blackout
	AllDay      bool      `json:"all_day"`     // All day blackout
}

// CrashHandlingConfig defines crash handling behavior
type CrashHandlingConfig struct {
	AutoRestart       bool          `json:"auto_restart"`        // Automatically restart on crash
	MaxRestarts       int           `json:"max_restarts"`        // Maximum restart attempts
	RestartDelay      time.Duration `json:"restart_delay"`       // Delay between restarts
	NotifySysop       bool          `json:"notify_sysop"`        // Notify sysop on crash
	SaveCrashLogs     bool          `json:"save_crash_logs"`     // Save crash logs
	CrashLogPath      string        `json:"crash_log_path"`      // Crash log directory
	KillOnTimeout     bool          `json:"kill_on_timeout"`     // Kill process on timeout
	TimeoutSeconds    int           `json:"timeout_seconds"`     // Timeout in seconds
}

// LoggingConfig defines logging settings
type LoggingConfig struct {
	Enabled           bool          `json:"enabled"`             // Logging enabled
	LogLevel          string        `json:"log_level"`           // Log level
	LogPath           string        `json:"log_path"`            // Log file path
	MaxLogSize        int64         `json:"max_log_size"`        // Maximum log size
	MaxLogFiles       int           `json:"max_log_files"`       // Maximum log files
	LogRotation       bool          `json:"log_rotation"`        // Enable log rotation
	LogUserActivity   bool          `json:"log_user_activity"`   // Log user activity
	LogSystemEvents   bool          `json:"log_system_events"`   // Log system events
	LogDebug          bool          `json:"log_debug"`           // Log debug information
}

// MonitoringConfig defines monitoring settings
type MonitoringConfig struct {
	Enabled           bool          `json:"enabled"`             // Monitoring enabled
	CheckInterval     time.Duration `json:"check_interval"`      // Check interval
	PerformanceTrack  bool          `json:"performance_track"`   // Track performance
	ResourceMonitor   bool          `json:"resource_monitor"`    // Monitor resources
	AlertOnHang       bool          `json:"alert_on_hang"`       // Alert on hang
	HangTimeout       time.Duration `json:"hang_timeout"`        // Hang detection timeout
	AlertOnCrash      bool          `json:"alert_on_crash"`      // Alert on crash
	StatsCollection   bool          `json:"stats_collection"`    // Collect statistics
}

// DoorStatistics holds door usage statistics
type DoorStatistics struct {
	TotalRuns         int64         `json:"total_runs"`          // Total times run
	TotalTime         time.Duration `json:"total_time"`          // Total time used
	AverageTime       time.Duration `json:"average_time"`        // Average session time
	LastRun           time.Time     `json:"last_run"`            // Last run time
	PopularityRank    int           `json:"popularity_rank"`     // Popularity ranking
	CrashCount        int           `json:"crash_count"`         // Number of crashes
	LastCrash         time.Time     `json:"last_crash"`          // Last crash time
	UserRatings       []UserRating  `json:"user_ratings"`        // User ratings
	AverageRating     float64       `json:"average_rating"`      // Average rating
	PeakUsers         int           `json:"peak_users"`          // Peak concurrent users
	PeakTime          time.Time     `json:"peak_time"`           // When peak occurred
	TotalUsers        int           `json:"total_users"`         // Total unique users
	ReturnUsers       int           `json:"return_users"`        // Returning users
	DailyStats        []DailyStats  `json:"daily_stats"`         // Daily statistics
}

// UserRating represents a user's rating of a door
type UserRating struct {
	UserID    int       `json:"user_id"`    // User ID
	Rating    int       `json:"rating"`     // Rating (1-5)
	Comment   string    `json:"comment"`    // Optional comment
	Date      time.Time `json:"date"`       // Rating date
}

// DailyStats represents daily usage statistics
type DailyStats struct {
	Date      time.Time     `json:"date"`       // Date
	Runs      int           `json:"runs"`       // Number of runs
	TotalTime time.Duration `json:"total_time"` // Total time
	Users     int           `json:"users"`      // Number of users
	Crashes   int           `json:"crashes"`    // Number of crashes
}

// DoorInstance represents a running door instance
type DoorInstance struct {
	ID               string            `json:"id"`                // Instance ID
	DoorID           string            `json:"door_id"`           // Door configuration ID
	NodeID           int               `json:"node_id"`           // Node running the door
	UserID           int               `json:"user_id"`           // User running the door
	ProcessID        int               `json:"process_id"`        // Process ID
	StartTime        time.Time         `json:"start_time"`        // Start time
	Status           InstanceStatus    `json:"status"`            // Current status
	DropFilePath     string            `json:"dropfile_path"`     // Dropfile location
	WorkingDir       string            `json:"working_dir"`       // Working directory
	Environment      map[string]string `json:"environment"`       // Environment variables
	ResourceLocks    []string          `json:"resource_locks"`    // Locked resources
	LastActivity     time.Time         `json:"last_activity"`     // Last activity
	BytesSent        int64             `json:"bytes_sent"`        // Bytes sent
	BytesReceived    int64             `json:"bytes_received"`    // Bytes received
	ErrorCount       int               `json:"error_count"`       // Error count
	WarningCount     int               `json:"warning_count"`     // Warning count
}

// InstanceStatus represents the status of a door instance
type InstanceStatus int

const (
	InstanceStatusStarting InstanceStatus = iota
	InstanceStatusRunning
	InstanceStatusSuspended
	InstanceStatusFinishing
	InstanceStatusFinished
	InstanceStatusCrashed
	InstanceStatusKilled
	InstanceStatusTimedOut
)

func (is InstanceStatus) String() string {
	switch is {
	case InstanceStatusStarting:
		return "Starting"
	case InstanceStatusRunning:
		return "Running"
	case InstanceStatusSuspended:
		return "Suspended"
	case InstanceStatusFinishing:
		return "Finishing"
	case InstanceStatusFinished:
		return "Finished"
	case InstanceStatusCrashed:
		return "Crashed"
	case InstanceStatusKilled:
		return "Killed"
	case InstanceStatusTimedOut:
		return "Timed Out"
	default:
		return "Unknown"
	}
}

// DoorQueue represents a queue of users waiting for a door
type DoorQueue struct {
	DoorID       string                `json:"door_id"`        // Door ID
	Queue        []DoorQueueEntry      `json:"queue"`          // Queue entries
	MaxLength    int                   `json:"max_length"`     // Maximum queue length
	TimeoutMins  int                   `json:"timeout_mins"`   // Queue timeout in minutes
	Enabled      bool                  `json:"enabled"`        // Queue enabled
	mu           sync.RWMutex          `json:"-"`              // Mutex for thread safety
}

// DoorQueueEntry represents an entry in the door queue
type DoorQueueEntry struct {
	UserID       int       `json:"user_id"`       // User ID
	NodeID       int       `json:"node_id"`       // Node ID
	QueueTime    time.Time `json:"queue_time"`    // Time added to queue
	Priority     int       `json:"priority"`      // Queue priority
	Notified     bool      `json:"notified"`      // User has been notified
}

// DoorResource represents a resource used by doors
type DoorResource struct {
	ID           string            `json:"id"`             // Resource ID
	Type         ResourceType      `json:"type"`           // Resource type
	Path         string            `json:"path"`           // Resource path
	Description  string            `json:"description"`    // Description
	MaxLocks     int               `json:"max_locks"`      // Maximum concurrent locks
	CurrentLocks int               `json:"current_locks"`  // Current lock count
	LockMode     LockMode          `json:"lock_mode"`      // Lock mode
	Locks        []ResourceLock    `json:"locks"`          // Active locks
	mu           sync.RWMutex      `json:"-"`              // Mutex for thread safety
}

// ResourceType represents the type of resource
type ResourceType int

const (
	ResourceTypeFile ResourceType = iota
	ResourceTypeDirectory
	ResourceTypeDatabase
	ResourceTypeNetwork
	ResourceTypeDevice
	ResourceTypeMemory
	ResourceTypeCustom
)

func (rt ResourceType) String() string {
	switch rt {
	case ResourceTypeFile:
		return "File"
	case ResourceTypeDirectory:
		return "Directory"
	case ResourceTypeDatabase:
		return "Database"
	case ResourceTypeNetwork:
		return "Network"
	case ResourceTypeDevice:
		return "Device"
	case ResourceTypeMemory:
		return "Memory"
	case ResourceTypeCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// LockMode represents the type of resource lock
type LockMode int

const (
	LockModeShared LockMode = iota
	LockModeExclusive
	LockModeReadOnly
	LockModeWriteOnly
)

func (lm LockMode) String() string {
	switch lm {
	case LockModeShared:
		return "Shared"
	case LockModeExclusive:
		return "Exclusive"
	case LockModeReadOnly:
		return "Read Only"
	case LockModeWriteOnly:
		return "Write Only"
	default:
		return "Unknown"
	}
}

// ResourceLock represents a lock on a resource
type ResourceLock struct {
	ID           string        `json:"id"`            // Lock ID
	DoorID       string        `json:"door_id"`       // Door that owns the lock
	InstanceID   string        `json:"instance_id"`   // Instance that owns the lock
	NodeID       int           `json:"node_id"`       // Node that owns the lock
	UserID       int           `json:"user_id"`       // User that owns the lock
	LockTime     time.Time     `json:"lock_time"`     // When lock was acquired
	LockMode     LockMode      `json:"lock_mode"`     // Lock mode
	Timeout      time.Duration `json:"timeout"`       // Lock timeout
	RefCount     int           `json:"ref_count"`     // Reference count
}

// DoorTemplate represents a door configuration template
type DoorTemplate struct {
	ID           string                    `json:"id"`            // Template ID
	Name         string                    `json:"name"`          // Template name
	Description  string                    `json:"description"`   // Description
	Category     DoorCategory              `json:"category"`      // Category
	Version      string                    `json:"version"`       // Template version
	Author       string                    `json:"author"`        // Template author
	Config       DoorConfiguration         `json:"config"`        // Base configuration
	Variables    map[string]TemplateVar    `json:"variables"`     // Template variables
	Instructions string                    `json:"instructions"`  // Setup instructions
	Required     []string                  `json:"required"`      // Required files/settings
	Optional     []string                  `json:"optional"`      // Optional files/settings
	Created      time.Time                 `json:"created"`       // Creation time
	Modified     time.Time                 `json:"modified"`      // Last modification
}

// TemplateVar represents a template variable
type TemplateVar struct {
	Name         string        `json:"name"`          // Variable name
	Type         VarType       `json:"type"`          // Variable type
	Description  string        `json:"description"`   // Description
	DefaultValue interface{}   `json:"default_value"` // Default value
	Required     bool          `json:"required"`      // Required variable
	Options      []string      `json:"options"`       // Valid options (for enum types)
	Validation   string        `json:"validation"`    // Validation regex
	MinValue     interface{}   `json:"min_value"`     // Minimum value
	MaxValue     interface{}   `json:"max_value"`     // Maximum value
}

// VarType represents template variable types
type VarType int

const (
	VarTypeString VarType = iota
	VarTypeInt
	VarTypeFloat
	VarTypeBool
	VarTypePath
	VarTypeEnum
	VarTypeList
)

func (vt VarType) String() string {
	switch vt {
	case VarTypeString:
		return "String"
	case VarTypeInt:
		return "Integer"
	case VarTypeFloat:
		return "Float"
	case VarTypeBool:
		return "Boolean"
	case VarTypePath:
		return "Path"
	case VarTypeEnum:
		return "Enum"
	case VarTypeList:
		return "List"
	default:
		return "Unknown"
	}
}

// DoorAlert represents a door-related alert
type DoorAlert struct {
	ID          string        `json:"id"`          // Alert ID
	DoorID      string        `json:"door_id"`     // Door ID
	InstanceID  string        `json:"instance_id"` // Instance ID (if applicable)
	NodeID      int           `json:"node_id"`     // Node ID
	AlertType   AlertType     `json:"alert_type"`  // Alert type
	Severity    AlertSeverity `json:"severity"`    // Alert severity
	Message     string        `json:"message"`     // Alert message
	Details     string        `json:"details"`     // Additional details
	Timestamp   time.Time     `json:"timestamp"`   // Alert timestamp
	Acknowledged bool         `json:"acknowledged"` // Alert acknowledged
	AutoClear   bool          `json:"auto_clear"`  // Auto-clear when resolved
	Resolved    bool          `json:"resolved"`    // Alert resolved
	ResolvedAt  time.Time     `json:"resolved_at"` // Resolution time
	Actions     []string      `json:"actions"`     // Suggested actions
}

// AlertType represents the type of alert
type AlertType int

const (
	AlertTypeInfo AlertType = iota
	AlertTypeWarning
	AlertTypeError
	AlertTypeCrash
	AlertTypeHang
	AlertTypeTimeout
	AlertTypeResource
	AlertTypeSecurity
	AlertTypePerformance
	AlertTypeCustom
)

func (at AlertType) String() string {
	switch at {
	case AlertTypeInfo:
		return "Info"
	case AlertTypeWarning:
		return "Warning"
	case AlertTypeError:
		return "Error"
	case AlertTypeCrash:
		return "Crash"
	case AlertTypeHang:
		return "Hang"
	case AlertTypeTimeout:
		return "Timeout"
	case AlertTypeResource:
		return "Resource"
	case AlertTypeSecurity:
		return "Security"
	case AlertTypePerformance:
		return "Performance"
	case AlertTypeCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// AlertSeverity represents the severity of an alert
type AlertSeverity int

const (
	SeverityLow AlertSeverity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

func (as AlertSeverity) String() string {
	switch as {
	case SeverityLow:
		return "Low"
	case SeverityMedium:
		return "Medium"
	case SeverityHigh:
		return "High"
	case SeverityCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// DoorManager interface defines the contract for door management
type DoorManager interface {
	// Configuration management
	GetDoorConfig(doorID string) (*DoorConfiguration, error)
	GetAllDoorConfigs() ([]*DoorConfiguration, error)
	CreateDoorConfig(config *DoorConfiguration) error
	UpdateDoorConfig(config *DoorConfiguration) error
	DeleteDoorConfig(doorID string) error
	
	// Template management
	GetDoorTemplate(templateID string) (*DoorTemplate, error)
	GetAllDoorTemplates() ([]*DoorTemplate, error)
	CreateDoorFromTemplate(templateID string, vars map[string]interface{}) (*DoorConfiguration, error)
	
	// Instance management
	LaunchDoor(doorID string, nodeID int, userID int) (*DoorInstance, error)
	GetDoorInstance(instanceID string) (*DoorInstance, error)
	GetActiveDoorInstances() ([]*DoorInstance, error)
	GetNodeDoorInstances(nodeID int) ([]*DoorInstance, error)
	TerminateDoorInstance(instanceID string) error
	
	// Queue management
	AddToQueue(doorID string, nodeID int, userID int) error
	RemoveFromQueue(doorID string, userID int) error
	GetQueuePosition(doorID string, userID int) (int, error)
	GetQueueStatus(doorID string) (*DoorQueue, error)
	
	// Resource management
	AcquireResource(resourceID string, doorID string, instanceID string, mode LockMode) (*ResourceLock, error)
	ReleaseResource(lockID string) error
	GetResourceStatus(resourceID string) (*DoorResource, error)
	
	// Testing and validation
	TestDoorConfig(doorID string) error
	ValidateDoorConfig(config *DoorConfiguration) error
	
	// Statistics and monitoring
	GetDoorStatistics(doorID string) (*DoorStatistics, error)
	UpdateDoorStatistics(doorID string, stats *DoorStatistics) error
	GetSystemDoorStats() (map[string]*DoorStatistics, error)
	
	// Alert management
	GetDoorAlerts(doorID string) ([]*DoorAlert, error)
	GetAllDoorAlerts() ([]*DoorAlert, error)
	AcknowledgeAlert(alertID string) error
	ResolveAlert(alertID string) error
	AddAlert(alert *DoorAlert) error
	
	// Maintenance
	CleanupExpiredLocks() error
	CleanupFinishedInstances() error
	PerformDoorMaintenance(doorID string) error
	BackupDoorConfigs() error
	RestoreDoorConfigs(backupPath string) error
}