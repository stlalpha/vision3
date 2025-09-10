package multinode

import (
	"time"
)

// Node status and activity tracking structures
type NodeStatus struct {
	NodeNum        uint8      // Node number (1-255)
	Status         uint8      // Node status (see NodeStatusType constants)
	UserID         uint16     // Current user ID (0 = no user)
	UserName       [26]byte   // Current user name (null-terminated)
	Activity       [64]byte   // Current activity description (null-terminated)
	Location       [32]byte   // Geographic location/city (null-terminated)
	ConnectTime    [20]byte   // Connection start time
	LastUpdate     [20]byte   // Last status update time
	BaudRate       uint32     // Connection speed
	BytesUp        uint64     // Bytes uploaded this session
	BytesDown      uint64     // Bytes downloaded this session
	MsgsPosted     uint16     // Messages posted this session
	MsgsRead       uint16     // Messages read this session
	FilesUp        uint16     // Files uploaded this session
	FilesDown      uint16     // Files downloaded this session
	CallsToday     uint16     // Calls today by this user
	TimeLeft       uint16     // Minutes remaining for user
	Reserved       [32]byte   // Reserved for future use
} // Total: 256 bytes

// Node status types
const (
	NodeStatusOffline   = 0 // Node is offline/not in use
	NodeStatusWaiting   = 1 // Waiting for caller
	NodeStatusLogin     = 2 // User logging in
	NodeStatusMainMenu  = 3 // At main menu
	NodeStatusMsgBase   = 4 // In message base
	NodeStatusFileBase  = 5 // In file base
	NodeStatusChat      = 6 // In chat mode
	NodeStatusDoor      = 7 // Running external door
	NodeStatusTransfer  = 8 // File transfer in progress
	NodeStatusSysop     = 9 // Sysop activities
	NodeStatusMaint     = 10 // System maintenance
	NodeStatusLocal     = 11 // Local console
	NodeStatusTelnet    = 12 // Telnet connection
	NodeStatusSSH       = 13 // SSH connection
	NodeStatusWeb       = 14 // Web interface
)

// Semaphore file for coordinating critical operations
type SystemSemaphore struct {
	SemaphoreID   uint32     // Unique semaphore identifier
	NodeNum       uint8      // Node that created semaphore
	Operation     [64]byte   // Operation description
	Resource      [64]byte   // Resource being protected
	StartTime     [20]byte   // When operation started
	MaxDuration   uint32     // Maximum expected duration in seconds
	Priority      uint8      // Priority level (1-10, higher = more important)
	Exclusive     uint8      // 1 = exclusive access, 0 = shared
	ProcessID     uint32     // Process ID that created semaphore
	Reserved      [32]byte   // Reserved for future use
} // Total: 256 bytes

// Node communication message (for inter-node chat, broadcasts, etc.)
type NodeMessage struct {
	MessageID     uint32     // Unique message ID
	FromNode      uint8      // Sending node number
	ToNode        uint8      // Destination node (0 = broadcast to all)
	MessageType   uint8      // Message type (see NodeMsgType constants)
	Priority      uint8      // Message priority (1-10)
	Timestamp     [20]byte   // When message was sent
	Subject       [64]byte   // Message subject/title
	Body          [256]byte  // Message body text
	ReadStatus    [32]byte   // Bit field for read status per node
	Reserved      [16]byte   // Reserved for future use
} // Total: 512 bytes

// Node message types
const (
	NodeMsgChat      = 1  // Chat request/message
	NodeMsgBroadcast = 2  // System broadcast
	NodeMsgAlert     = 3  // System alert/warning
	NodeMsgStatus    = 4  // Status update
	NodeMsgPrivate   = 5  // Private message between nodes
	NodeMsgSysop     = 6  // Message to/from sysop
	NodeMsgSystem    = 7  // System-generated message
	NodeMsgPage      = 8  // Page sysop request
	NodeMsgEmergency = 9  // Emergency shutdown/message
	NodeMsgDebug     = 10 // Debug information
)

// Resource lock for database and file operations
type ResourceLock struct {
	LockID        uint32     // Unique lock identifier
	NodeNum       uint8      // Node that owns the lock
	LockType      uint8      // Type of lock (see LockType constants)
	Resource      [128]byte  // Resource identifier (path, database name, etc.)
	Timestamp     [20]byte   // When lock was acquired
	Duration      uint32     // Lock duration in seconds (0 = indefinite)
	ProcessID     uint32     // Process ID that owns lock
	RefCount      uint16     // Reference count for shared locks
	WaitQueue     [32]byte   // Nodes waiting for this resource (bit field)
	Reserved      [16]byte   // Reserved for future use
} // Total: 256 bytes

// Lock types for resource management
const (
	LockTypeRead      = 1 // Shared read lock
	LockTypeWrite     = 2 // Exclusive write lock
	LockTypeDelete    = 3 // Exclusive delete lock
	LockTypeMaint     = 4 // Maintenance lock (prevents all access)
	LockTypeBackup    = 5 // Backup operation lock
	LockTypeReindex   = 6 // Index rebuilding lock
	LockTypeCompact   = 7 // Database compaction lock
	LockTypeImport    = 8 // Data import lock
	LockTypeExport    = 9 // Data export lock
)

// Node configuration and limits
type NodeConfig struct {
	NodeNum          uint8      // Node number
	NodeName         [32]byte   // Descriptive name for node
	NodeType         uint8      // Node type (see NodeType constants)
	MaxUsers         uint16     // Maximum concurrent users
	MaxTime          uint16     // Maximum time per user (minutes)
	MaxCalls         uint16     // Maximum calls per day
	BaudRate         uint32     // Connection speed
	ModemInit        [128]byte  // Modem initialization string
	AnswerString     [64]byte   // Modem answer string
	PortSettings     [32]byte   // Serial port settings
	TelnetPort       uint16     // Telnet port number
	SSHPort          uint16     // SSH port number
	WebPort          uint16     // Web interface port
	LogLevel         uint8      // Logging level (0-10)
	AutoMaint        uint8      // Auto-maintenance enabled (1/0)
	AllowNewUsers    uint8      // Allow new user registration (1/0)
	AllowGuests      uint8      // Allow guest access (1/0)
	RequireRealName  uint8      // Require real names (1/0)
	RequirePhone     uint8      // Require phone verification (1/0)
	AllowChatReq     uint8      // Allow chat requests (1/0)
	AllowFileReq     uint8      // Allow file requests (1/0)
	Reserved         [64]byte   // Reserved for future use
} // Total: 512 bytes

// Node types
const (
	NodeTypeLocal    = 1 // Local console node
	NodeTypeDialup   = 2 // Dial-up modem node
	NodeTypeTelnet   = 3 // Telnet node
	NodeTypeSSH      = 4 // SSH node
	NodeTypeWeb      = 5 // Web interface node
	NodeTypeRLogin   = 6 // RLogin node
	NodeTypeDoor     = 7 // Door game server node
	NodeTypeMaint    = 8 // Maintenance-only node
)

// System-wide events and notifications
type SystemEvent struct {
	EventID       uint32     // Unique event identifier
	EventType     uint8      // Event type (see EventType constants)
	Severity      uint8      // Severity level (1-10)
	NodeNum       uint8      // Node that generated event
	UserID        uint16     // User involved (if applicable)
	Timestamp     [20]byte   // When event occurred
	Category      [32]byte   // Event category
	Description   [128]byte  // Event description
	Data          [256]byte  // Additional event data
	Acknowledged  uint8      // 1 if event has been acknowledged
	Reserved      [32]byte   // Reserved for future use
} // Total: 512 bytes

// System event types
const (
	EventLogon       = 1  // User logon
	EventLogoff      = 2  // User logoff
	EventNewUser     = 3  // New user registration
	EventUpload      = 4  // File upload
	EventDownload    = 5  // File download
	EventMessage     = 6  // Message posted
	EventChat        = 7  // Chat session
	EventDoor        = 8  // Door game access
	EventError       = 9  // System error
	EventWarning     = 10 // System warning
	EventMaint       = 11 // Maintenance operation
	EventSecurity    = 12 // Security violation
	EventHackAttempt = 13 // Hack attempt detected
	EventSystemDown  = 14 // System shutdown
	EventSystemUp    = 15 // System startup
)

// Node performance statistics
type NodeStats struct {
	NodeNum           uint8      // Node number
	TotalCalls        uint64     // Total calls handled
	TotalTime         uint64     // Total connect time (minutes)
	TotalUploads      uint32     // Total files uploaded
	TotalDownloads    uint32     // Total files downloaded
	TotalMessages     uint32     // Total messages posted
	TotalNewUsers     uint32     // Total new users registered
	BytesTransferred  uint64     // Total bytes transferred
	AverageCallTime   uint32     // Average call length (minutes)
	PeakUsers         uint16     // Peak concurrent users
	PeakTime          [20]byte   // When peak was reached
	LastReset         [20]byte   // When stats were last reset
	UptimeHours       uint64     // Total uptime hours
	CrashCount        uint16     // Number of crashes/restarts
	ErrorCount        uint32     // Total errors logged
	Reserved          [32]byte   // Reserved for future use
} // Total: 256 bytes

// Real-time activity monitor
type ActivityMonitor struct {
	NodeNum       uint8      // Node being monitored
	UserID        uint16     // Current user ID
	Activity      uint8      // Current activity code
	Location      [64]byte   // Current location/menu
	StartTime     [20]byte   // When current activity started
	BytesSent     uint64     // Bytes sent this activity
	BytesReceived uint64     // Bytes received this activity
	KeysPressed   uint32     // Keys pressed this activity
	MenuLevel     uint8      // Current menu depth level
	LastInput     [20]byte   // Last input timestamp
	IdleTime      uint32     // Seconds idle
	Reserved      [32]byte   // Reserved for future use
} // Total: 256 bytes

// Chat room management
type ChatRoom struct {
	RoomID        uint8      // Room identifier (1-255)
	RoomName      [32]byte   // Room name
	Topic         [128]byte  // Current room topic
	Creator       [26]byte   // User who created room
	CreateTime    [20]byte   // When room was created
	MaxUsers      uint8      // Maximum users allowed
	CurrentUsers  uint8      // Current user count
	UserList      [32]byte   // Bit field of nodes in room
	IsPrivate     uint8      // 1 = private room, 0 = public
	RequireInvite uint8      // 1 = invitation required
	Moderated     uint8      // 1 = moderated room
	LogHistory    uint8      // 1 = log chat history
	Reserved      [32]byte   // Reserved for future use
} // Total: 256 bytes

// File sharing between nodes
type NodeFileShare struct {
	ShareID       uint32     // Unique share identifier
	OwnerNode     uint8      // Node sharing the file
	FilePath      [256]byte  // Full path to shared file
	ShareName     [64]byte   // Friendly name for share
	Description   [128]byte  // File description
	FileSize      uint64     // File size in bytes
	CreateTime    [20]byte   // When share was created
	AccessCount   uint32     // Number of times accessed
	Permissions   uint8      // Access permissions (bit field)
	ExpiryTime    [20]byte   // When share expires (empty = never)
	Password      [16]byte   // Share password (optional)
	Reserved      [32]byte   // Reserved for future use
} // Total: 512 bytes

// System-wide configuration synchronization
type ConfigSync struct {
	ConfigID      uint32     // Configuration item identifier
	ConfigType    uint8      // Type of configuration (see ConfigType constants)
	NodeNum       uint8      // Node that made change
	Timestamp     [20]byte   // When change was made
	ConfigName    [64]byte   // Configuration item name
	OldValue      [256]byte  // Previous value
	NewValue      [256]byte  // New value
	SyncStatus    [32]byte   // Sync status per node (bit field)
	Priority      uint8      // Sync priority (1-10)
	Reserved      [32]byte   // Reserved for future use
} // Total: 512 bytes

// Configuration types for synchronization
const (
	ConfigTypeSystem    = 1 // System configuration
	ConfigTypeUser      = 2 // User settings
	ConfigTypeMessage   = 3 // Message base settings
	ConfigTypeFile      = 4 // File base settings
	ConfigTypeDoor      = 5 // Door configuration
	ConfigTypeMenu      = 6 // Menu configuration
	ConfigTypeDisplay   = 7 // Display settings
	ConfigTypeSecurity  = 8 // Security settings
	ConfigTypeNetwork   = 9 // Network settings
	ConfigTypeProtocol  = 10 // Protocol settings
)

// File names for multi-node coordination
const (
	NodeStatusFile    = "NODESTATUS.DAT" // Current node status
	SemaphoreFile     = "SEMAPHORE.DAT"  // System semaphores
	NodeMessageFile   = "NODEMSG.DAT"    // Inter-node messages
	ResourceLockFile  = "LOCKS.DAT"      // Resource locks
	NodeConfigFile    = "NODECONF.DAT"   // Node configurations
	SystemEventFile   = "EVENTS.DAT"     // System event log
	NodeStatsFile     = "NODESTATS.DAT"  // Node statistics
	ActivityFile      = "ACTIVITY.DAT"   // Real-time activity
	ChatRoomFile      = "CHATROOM.DAT"   // Chat room data
	FileShareFile     = "FILESHARE.DAT"  // File sharing data
	ConfigSyncFile    = "CONFIGSYNC.DAT" // Configuration sync
	MasterLockFile    = "MASTER.LOK"     // Master lock file
)

// Node communication protocols
type CommProtocol struct {
	ProtocolID    uint8      // Protocol identifier
	ProtocolName  [16]byte   // Protocol name
	Version       [8]byte    // Protocol version
	Features      uint32     // Supported features (bit field)
	MaxPacketSize uint16     // Maximum packet size
	TimeoutSecs   uint16     // Timeout in seconds
	RetryCount    uint8      // Number of retries
	Compression   uint8      // Compression supported (1/0)
	Encryption    uint8      // Encryption supported (1/0)
	Reserved      [32]byte   // Reserved for future use
} // Total: 128 bytes

// Network node information (for multi-BBS networks)
type NetworkNode struct {
	NetAddress    [32]byte   // Network address (FidoNet style)
	SystemName    [64]byte   // BBS system name
	SysopName     [32]byte   // Sysop name
	Location      [32]byte   // Geographic location
	PhoneNumber   [16]byte   // Phone number
	BaudRate      uint32     // Maximum baud rate
	Protocols     uint32     // Supported protocols (bit field)
	MailHours     [32]byte   // Mail hours
	Capabilities  uint32     // Node capabilities (bit field)
	LastContact   [20]byte   // Last successful contact
	CallCount     uint32     // Number of calls made/received
	Status        uint8      // Node status (online/offline/etc.)
	Reserved      [32]byte   // Reserved for future use
} // Total: 256 bytes

// Constants for maximum values
const (
	MaxNodes        = 255  // Maximum number of nodes
	MaxChatRooms    = 255  // Maximum chat rooms
	MaxFileShares   = 1000 // Maximum file shares
	MaxSemaphores   = 100  // Maximum system semaphores
	MaxResourceLocks = 500 // Maximum resource locks
	MaxNodeMessages = 1000 // Maximum pending node messages
)

// Timeouts and intervals (in seconds)
const (
	NodeStatusUpdateInterval = 30   // How often to update node status
	LockTimeoutDefault      = 300   // Default lock timeout
	SemaphoreTimeoutDefault = 600   // Default semaphore timeout
	MessageRetentionDays    = 7     // Days to keep node messages
	EventRetentionDays      = 30    // Days to keep system events
	ActivityUpdateInterval  = 10    // Activity monitor update interval
	DeadlockCheckInterval   = 60    // Deadlock detection interval
)