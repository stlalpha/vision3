package msgbase

import (
	"time"
)

// Binary message header structure - fixed length for classic BBS feel
// Similar to PCBoard, TriBBS, and other classic BBS message formats
type MessageHeader struct {
	MsgNum       uint32    // Message number (1-based, sequential)
	ReplyTo      uint32    // Message number this is a reply to (0 = original)
	NextReply    uint32    // Next message in reply chain (0 = end of chain)
	Date         [8]byte   // Date in MM-DD-YY format (null-terminated)
	Time         [8]byte   // Time in HH:MM:SS format (null-terminated)
	ToUser       [36]byte  // Destination user name (null-terminated)
	FromUser     [36]byte  // Sender user name (null-terminated)
	Subject      [73]byte  // Message subject (null-terminated)
	Password     [13]byte  // Message password (null-terminated)
	RefMsgNum    uint32    // Reference message number
	NumBlocks    uint16    // Number of 128-byte blocks in message body
	Status       uint8     // Message status flags (see MsgStatus constants)
	Reserved     [5]byte   // Reserved for future use
} // Total: 256 bytes

// Message status bit flags (classic BBS style)
const (
	MsgStatusActive    = 0x01 // Message is active (not deleted)
	MsgStatusPrivate   = 0x02 // Private message
	MsgStatusReceived  = 0x04 // Message has been received/read
	MsgStatusProtected = 0x08 // Protected from deletion
	MsgStatusLocal     = 0x10 // Local message (not networked)
	MsgStatusNetmail   = 0x20 // Netmail message
	MsgStatusEchomail  = 0x40 // Echomail message
	MsgStatusUrgent    = 0x80 // Urgent/priority message
)

// Message index entry for fast access and threading
type MessageIndex struct {
	MsgNum    uint32 // Message number
	Offset    uint32 // Byte offset in .MSG file
	Length    uint32 // Total length including header
	Hash      uint32 // Hash of subject for thread grouping
	Status    uint8  // Copy of message status
	Reserved  [3]byte
} // Total: 20 bytes

// Thread index for reply chain navigation
type ThreadIndex struct {
	MsgNum      uint32 // Original message number
	FirstReply  uint32 // First reply in chain
	LastReply   uint32 // Last reply in chain
	ReplyCount  uint16 // Number of replies
	Reserved    [6]byte
} // Total: 20 bytes

// Message area configuration (binary format)
type MessageAreaConfig struct {
	AreaNum      uint16    // Area number (1-based)
	AreaTag      [13]byte  // Area tag (null-terminated)
	AreaName     [81]byte  // Area name (null-terminated)
	Description  [256]byte // Area description (null-terminated)
	Path         [81]byte  // Base path for message files (null-terminated)
	MaxMsgs      uint32    // Maximum messages (0 = unlimited)
	DaysOld      uint16    // Days to keep messages (0 = forever)
	ReadLevel    uint16    // Security level required to read
	WriteLevel   uint16    // Security level required to write
	SysopLevel   uint16    // Security level for sysop functions
	Flags        uint32    // Area flags (see AreaFlag constants)
	HighMsgNum   uint32    // Highest message number used
	TotalMsgs    uint32    // Total active messages
	Reserved     [32]byte  // Reserved for future expansion
} // Total: 512 bytes

// Message area flags
const (
	AreaFlagPublic    = 0x00000001 // Public area
	AreaFlagPrivate   = 0x00000002 // Private mail area
	AreaFlagEcho      = 0x00000004 // Echomail area
	AreaFlagNet       = 0x00000008 // Netmail area
	AreaFlagModerated = 0x00000010 // Moderated area
	AreaFlagRealName  = 0x00000020 // Real names required
	AreaFlagAnonymous = 0x00000040 // Anonymous posting allowed
	AreaFlagNoDelete  = 0x00000080 // No user deletion allowed
	AreaFlagReadOnly  = 0x00000100 // Read-only area
	AreaFlagSysopOnly = 0x00000200 // Sysop-only area
)

// Message base statistics
type MessageBaseStats struct {
	TotalAreas   uint16
	TotalMsgs    uint32
	TotalKBytes  uint32
	LastPacked   [20]byte // Last pack date/time
	LastMaint    [20]byte // Last maintenance date/time
	PackedMsgs   uint32   // Messages removed during last pack
	FragPercent  uint8    // Fragmentation percentage
	Reserved     [23]byte
} // Total: 64 bytes

// File names for message base files (classic BBS naming)
const (
	MessageHeaderFile = "MESSAGES.HDR" // Message headers
	MessageDataFile   = "MESSAGES.DAT" // Message text data
	MessageIndexFile  = "MESSAGES.IDX" // Message index
	ThreadIndexFile   = "MESSAGES.THD" // Thread index
	AreaConfigFile    = "AREAS.DAT"    // Area configuration
	StatsFile         = "STATS.DAT"    // Statistics
	LockFile          = "MESSAGE.LOK"  // Lock file for multi-node safety
)

// Lock types for multi-node operations
const (
	LockTypeNone   = 0
	LockTypeRead   = 1
	LockTypeWrite  = 2
	LockTypePack   = 3
	LockTypeMaint  = 4
)

// Multi-node lock record
type NodeLock struct {
	NodeNum   uint8     // Node number (1-255)
	LockType  uint8     // Type of lock (see LockType constants)
	AreaNum   uint16    // Area being locked (0 = all areas)
	MsgNum    uint32    // Specific message (0 = not specific)
	Timestamp [20]byte  // When lock was acquired
	Process   [32]byte  // Process name/description
	Reserved  [12]byte
} // Total: 72 bytes

// Semaphore file for critical operations
type Semaphore struct {
	NodeNum     uint8    // Node holding semaphore
	Operation   [32]byte // Operation description
	StartTime   [20]byte // When operation started
	Reserved    [11]byte
} // Total: 64 bytes

// Message body storage format (variable length blocks)
type MessageBody struct {
	// Message text stored in 128-byte blocks for classic BBS compatibility
	// Each block is null-padded to 128 bytes
	// Last block may be partial but still padded to 128 bytes
}

// Constants for message body storage
const (
	MessageBlockSize = 128    // Size of each message text block
	MaxMessageBlocks = 65535  // Maximum blocks per message (8MB max)
	MaxMessageSize   = MaxMessageBlocks * MessageBlockSize
)

// Network message attributes (FidoNet-style)
type NetAttributes struct {
	Flags    uint16   // Network flags
	Cost     uint16   // Message cost
	OrigNet  uint16   // Origin network
	OrigNode uint16   // Origin node
	DestNet  uint16   // Destination network  
	DestNode uint16   // Destination node
	Reserved [8]byte
} // Total: 20 bytes

// Network flags (FidoNet compatible)
const (
	NetFlagPrivate   = 0x0001
	NetFlagCrash     = 0x0002
	NetFlagReceived  = 0x0004
	NetFlagSent      = 0x0008
	NetFlagFileAttach = 0x0010
	NetFlagInTransit = 0x0020
	NetFlagOrphan    = 0x0040
	NetFlagKillSent  = 0x0080
	NetFlagLocal     = 0x0100
	NetFlagHoldForPickup = 0x0200
	NetFlagFileRequest   = 0x0800
	NetFlagReturnReceipt = 0x1000
	NetFlagAuditRequest  = 0x2000
	NetFlagFileUpdate    = 0x4000
)