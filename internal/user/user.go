package user

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// MaxLastLogins = 10 // Max number of last logins to store (Moved to manager.go)

// LoginEvent holds information about a single login
type LoginEvent struct {
	Username  string
	Handle    string
	Timestamp time.Time
}

// User represents a user account.
type User struct {
	ID               int       `json:"id"` // Added User ID for ACS 'U' check
	Username         string    `json:"username"`
	PasswordHash     string    `json:"passwordHash"` // Changed from []byte to string
	Handle           string    `json:"handle"`
	AccessLevel      int       `json:"accessLevel"`
	Flags            string    `json:"flags"` // Added Flags string for ACS 'F' check (e.g., "XYZ")
	LastLogin        time.Time `json:"lastLogin"`
	TimesCalled      int       `json:"timesCalled"` // Used for E (NumLogons)
	LastBulletinRead time.Time `json:"lastBulletinRead"`
	RealName         string    `json:"realName"`
	PhoneNumber      string    `json:"phoneNumber"`
	CreatedAt        time.Time `json:"createdAt"`
	Validated        bool      `json:"validated"`
	FilePoints       int       `json:"filePoints"` // Added for P
	NumUploads       int       `json:"numUploads"` // Added for E
	// NumLogons is TimesCalled
	TimeLimit   int    `json:"timeLimit"`   // Added for T (in minutes)
	PrivateNote string `json:"privateNote"` // Added for Z
	// TODO: Add fields for current message/file conference if C/X needed
	GroupLocation         string         `json:"group_location,omitempty"`           // Added Group / Location field
	CurrentMessageAreaID  int            `json:"current_message_area_id,omitempty"`  // Added for default area tracking
	CurrentMessageAreaTag string         `json:"current_message_area_tag,omitempty"` // Added for default area tracking
	LastReadMessageIDs    map[int]string `json:"last_read_message_ids,omitempty"`    // Map AreaID -> Last Read Message UUID string

	// File System Related
	CurrentFileAreaID  int         `json:"current_file_area_id,omitempty"`  // Added for default file area tracking
	CurrentFileAreaTag string      `json:"current_file_area_tag,omitempty"` // Added for default file area tracking
	TaggedFileIDs      []uuid.UUID `json:"tagged_file_ids,omitempty"`       // List of FileRecord IDs marked for batch download

	// Door/Game System Fields
	Location    string    `json:"location,omitempty"`     // User's location (city/state)
	Credits     int       `json:"credits,omitempty"`      // Game/door credits
	TimeLeft    int       `json:"time_left,omitempty"`    // Minutes left in current session
	LastCall    time.Time `json:"last_call,omitempty"`    // Last call date (different from login)
	TimesOn     int       `json:"times_on,omitempty"`     // Total times online (different from calls)
	PageLength  int       `json:"page_length,omitempty"`  // Screen page length for doors
}

// HasFlag checks if a user has a specific flag
func (u *User) HasFlag(flag string) bool {
	return strings.Contains(u.Flags, flag)
}

// CallRecord stores information about a single call session.
type CallRecord struct {
	UserID         int           `json:"userID"`
	Handle         string        `json:"handle"`
	GroupLocation  string        `json:"groupLocation,omitempty"`
	NodeID         int           `json:"nodeID"`
	ConnectTime    time.Time     `json:"connectTime"`
	DisconnectTime time.Time     `json:"disconnectTime"`
	Duration       time.Duration `json:"duration"`
	UploadedMB     float64       `json:"uploadedMB"`           // Placeholder for now
	DownloadedMB   float64       `json:"downloadedMB"`         // Placeholder for now
	Actions        string        `json:"actions"`              // Placeholder for now (e.g., "D,U,M")
	BaudRate       string        `json:"baudRate"`             // Static value for now
	CallNumber     uint64        `json:"callNumber,omitempty"` // Overall call number
}