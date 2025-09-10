package nodes

import (
	"net"
	"sync"
	"time"

	"github.com/stlalpha/vision3/internal/session"
	"github.com/stlalpha/vision3/internal/user"
)

// NodeStatus represents the current state of a BBS node
type NodeStatus int

const (
	NodeStatusOffline NodeStatus = iota
	NodeStatusAvailable
	NodeStatusConnected
	NodeStatusLoggedIn
	NodeStatusInMenu
	NodeStatusInDoor
	NodeStatusInMessage
	NodeStatusInFile
	NodeStatusInChat
	NodeStatusDisconnecting
	NodeStatusMaintenance
	NodeStatusDisabled
)

func (ns NodeStatus) String() string {
	switch ns {
	case NodeStatusOffline:
		return "Offline"
	case NodeStatusAvailable:
		return "Available"
	case NodeStatusConnected:
		return "Connected"
	case NodeStatusLoggedIn:
		return "Logged In"
	case NodeStatusInMenu:
		return "In Menu"
	case NodeStatusInDoor:
		return "In Door"
	case NodeStatusInMessage:
		return "In Message"
	case NodeStatusInFile:
		return "In File"
	case NodeStatusInChat:
		return "In Chat"
	case NodeStatusDisconnecting:
		return "Disconnecting"
	case NodeStatusMaintenance:
		return "Maintenance"
	case NodeStatusDisabled:
		return "Disabled"
	default:
		return "Unknown"
	}
}

// NodeActivity represents what the user is currently doing
type NodeActivity struct {
	Type        string    `json:"type"`        // "menu", "door", "message", "file", "chat", etc.
	Description string    `json:"description"` // Human readable description
	StartTime   time.Time `json:"start_time"`  // When this activity started
	MenuName    string    `json:"menu_name"`   // Current menu if applicable
	DoorName    string    `json:"door_name"`   // Door program name if applicable
	AreaName    string    `json:"area_name"`   // Message/File area name if applicable
}

// NodeConfiguration holds node-specific settings
type NodeConfiguration struct {
	NodeID          int           `json:"node_id"`
	Name            string        `json:"name"`             // Friendly name for the node
	Enabled         bool          `json:"enabled"`          // Whether node is enabled
	MaxUsers        int           `json:"max_users"`        // Maximum concurrent users
	TimeLimit       time.Duration `json:"time_limit"`       // Time limit per session
	AccessLevel     int           `json:"access_level"`     // Minimum access level required
	AllowedFlags    []string      `json:"allowed_flags"`    // Required user flags
	LocalNode       bool          `json:"local_node"`       // True for local, false for remote
	ModemSettings   ModemConfig   `json:"modem_settings"`   // Modem configuration if applicable
	NetworkSettings NetworkConfig `json:"network_settings"` // Network configuration
	DoorSettings    DoorConfig    `json:"door_settings"`    // Door game settings
	ChatEnabled     bool          `json:"chat_enabled"`     // Inter-node chat enabled
	ScheduledHours  []TimeSlot    `json:"scheduled_hours"`  // When node is available
}

// ModemConfig holds modem-specific settings for dial-up nodes
type ModemConfig struct {
	Port     string `json:"port"`      // Serial port (e.g., "COM1", "/dev/ttyS0")
	BaudRate int    `json:"baud_rate"` // Connection speed
	InitString string `json:"init_string"` // Modem initialization string
	AnswerMode bool   `json:"answer_mode"` // Auto-answer mode
}

// NetworkConfig holds network settings for telnet/ssh nodes
type NetworkConfig struct {
	Protocol string `json:"protocol"` // "telnet", "ssh", "rlogin"
	Port     int    `json:"port"`     // Network port
	Address  string `json:"address"`  // Bind address
}

// DoorConfig holds door game configuration
type DoorConfig struct {
	AllowDoors      bool     `json:"allow_doors"`       // Whether doors are allowed
	MaxDoorTime     int      `json:"max_door_time"`     // Maximum time in doors (minutes)
	DoorPaths       []string `json:"door_paths"`        // Paths to door programs
	ShareResources  bool     `json:"share_resources"`   // Share files between nodes
	ExclusiveDoors  []string `json:"exclusive_doors"`   // Doors that require exclusive access
}

// TimeSlot represents a time period when a node is available
type TimeSlot struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	DaysOfWeek []int     `json:"days_of_week"` // 0=Sunday, 1=Monday, etc.
}

// NodeInfo represents current information about an active node
type NodeInfo struct {
	NodeID       int              `json:"node_id"`
	Status       NodeStatus       `json:"status"`
	User         *user.User       `json:"user,omitempty"`        // Currently logged in user
	Session      *session.BbsSession `json:"session,omitempty"`  // Active session
	Activity     NodeActivity     `json:"activity"`
	ConnectTime  time.Time        `json:"connect_time"`
	RemoteAddr   net.Addr         `json:"remote_addr,omitempty"`
	BytesSent    int64            `json:"bytes_sent"`
	BytesReceived int64           `json:"bytes_received"`
	MenuPath     []string         `json:"menu_path"`        // Stack of visited menus
	IdleTime     time.Duration    `json:"idle_time"`        // Time since last activity
	Config       NodeConfiguration `json:"config"`          // Node configuration
	
	// Performance metrics
	CPUUsage     float64   `json:"cpu_usage"`
	MemoryUsage  int64     `json:"memory_usage"`
	LastActivity time.Time `json:"last_activity"`
	
	// Chat and messaging
	InChat       bool      `json:"in_chat"`
	ChatPartner  int       `json:"chat_partner,omitempty"` // Node ID of chat partner
	Messages     []NodeMessage `json:"messages,omitempty"` // Pending messages
}

// NodeMessage represents a message sent to a node
type NodeMessage struct {
	FromNode    int       `json:"from_node"`
	FromUser    string    `json:"from_user"`
	ToNode      int       `json:"to_node"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	MessageType string    `json:"message_type"` // "chat", "system", "alert", "broadcast"
	Priority    int       `json:"priority"`     // 1=low, 2=normal, 3=high, 4=urgent
}

// NodeStatistics holds historical data about a node
type NodeStatistics struct {
	NodeID           int           `json:"node_id"`
	TotalConnections int64         `json:"total_connections"`
	TotalTime        time.Duration `json:"total_time"`
	AverageSession   time.Duration `json:"average_session"`
	PeakConcurrent   int           `json:"peak_concurrent"`
	LastRestart      time.Time     `json:"last_restart"`
	UptimePercent    float64       `json:"uptime_percent"`
	ErrorCount       int64         `json:"error_count"`
	LastError        time.Time     `json:"last_error"`
	BytesTransferred int64         `json:"bytes_transferred"`
}

// NodeAlert represents an alert condition for a node
type NodeAlert struct {
	NodeID      int       `json:"node_id"`
	AlertType   string    `json:"alert_type"`   // "error", "warning", "info"
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Acknowledged bool     `json:"acknowledged"`
	AutoClear   bool      `json:"auto_clear"`   // Automatically clear when condition resolves
}

// SystemStatus represents overall system status
type SystemStatus struct {
	TotalNodes      int                    `json:"total_nodes"`
	ActiveNodes     int                    `json:"active_nodes"`
	ConnectedUsers  int                    `json:"connected_users"`
	SystemLoad      float64                `json:"system_load"`
	MemoryUsage     int64                  `json:"memory_usage"`
	DiskUsage       int64                  `json:"disk_usage"`
	Uptime          time.Duration          `json:"uptime"`
	LastUpdate      time.Time              `json:"last_update"`
	Alerts          []NodeAlert            `json:"alerts"`
	NodeStats       map[int]NodeStatistics `json:"node_stats"`
}

// NodeManager interface defines the contract for managing nodes
type NodeManager interface {
	// Node management
	GetNode(nodeID int) (*NodeInfo, error)
	GetAllNodes() []*NodeInfo
	GetActiveNodes() []*NodeInfo
	EnableNode(nodeID int) error
	DisableNode(nodeID int) error
	RestartNode(nodeID int) error
	
	// Session management
	RegisterSession(nodeID int, session *session.BbsSession) error
	UnregisterSession(nodeID int) error
	UpdateActivity(nodeID int, activity NodeActivity) error
	GetNodeActivity(nodeID int) (NodeActivity, error)
	
	// Configuration
	GetNodeConfig(nodeID int) (*NodeConfiguration, error)
	UpdateNodeConfig(nodeID int, config NodeConfiguration) error
	GetSystemConfig() (*SystemConfig, error)
	UpdateSystemConfig(config SystemConfig) error
	
	// Monitoring
	GetSystemStatus() (*SystemStatus, error)
	GetNodeStatistics(nodeID int) (*NodeStatistics, error)
	AddAlert(alert NodeAlert) error
	GetAlerts() []NodeAlert
	AcknowledgeAlert(alertID int) error
	
	// Messaging
	SendMessage(message NodeMessage) error
	BroadcastMessage(message string, fromUser string) error
	GetMessages(nodeID int) []NodeMessage
	
	// Force operations
	DisconnectUser(nodeID int, reason string) error
	SendUserMessage(nodeID int, message string) error
	
	// Statistics
	UpdateStatistics(nodeID int, stats NodeStatistics) error
	GetHistoricalData(nodeID int, from, to time.Time) ([]NodeStatistics, error)
}

// SystemConfig represents global system configuration
type SystemConfig struct {
	MaxNodes        int           `json:"max_nodes"`
	DefaultTimeLimit time.Duration `json:"default_time_limit"`
	ChatEnabled     bool          `json:"chat_enabled"`
	InterNodeChat   bool          `json:"inter_node_chat"`
	AlertsEnabled   bool          `json:"alerts_enabled"`
	LogLevel        string        `json:"log_level"`
	MonitorInterval time.Duration `json:"monitor_interval"`
	SaveInterval    time.Duration `json:"save_interval"`
	BackupInterval  time.Duration `json:"backup_interval"`
	MaxAlerts       int           `json:"max_alerts"`
	AutoCleanup     bool          `json:"auto_cleanup"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
}

// Event types for real-time updates
type NodeEvent struct {
	Type      string      `json:"type"`      // "connect", "disconnect", "activity", "status", "alert"
	NodeID    int         `json:"node_id"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`      // Event-specific data
}

// NodeEventListener interface for receiving real-time updates
type NodeEventListener interface {
	OnNodeEvent(event NodeEvent) error
}

// WhoOnlineEntry represents an entry in the classic "Who's Online" display
type WhoOnlineEntry struct {
	NodeID      int           `json:"node_id"`
	UserHandle  string        `json:"user_handle"`
	UserLocation string       `json:"user_location"`
	Activity    string        `json:"activity"`
	OnlineTime  time.Duration `json:"online_time"`
	IdleTime    time.Duration `json:"idle_time"`
	BaudRate    string        `json:"baud_rate"`
	Status      string        `json:"status"`
}

// Mutex for thread-safe access to shared resources
type NodeManagerImpl struct {
	nodes       map[int]*NodeInfo
	config      SystemConfig
	statistics  map[int]*NodeStatistics
	alerts      []NodeAlert
	listeners   []NodeEventListener
	mu          sync.RWMutex
	eventChan   chan NodeEvent
	stopChan    chan bool
	dataPath    string
	userManager *user.UserMgr
}