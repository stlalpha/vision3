package user

import (
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
	UpdatedAt        time.Time `json:"updatedAt"` // For optimistic locking - tracks last modification
	Validated        bool      `json:"validated"`
	FilePoints       int       `json:"filePoints"`    // Added for P
	NumUploads       int       `json:"numUploads"`    // Added for E
	MessagesPosted   int       `json:"messagesPosted,omitempty"` // Number of messages posted by user
	// NumLogons is TimesCalled
	TimeLimit   int    `json:"timeLimit"`   // Added for T (in minutes)
	PrivateNote string `json:"privateNote"` // Added for Z
	// Conference tracking for ACS codes C (message conference) and X (file conference)
	CurrentMsgConferenceID   int    `json:"current_msg_conference_id,omitempty"`
	CurrentMsgConferenceTag  string `json:"current_msg_conference_tag,omitempty"`
	CurrentFileConferenceID  int    `json:"current_file_conference_id,omitempty"`
	CurrentFileConferenceTag string `json:"current_file_conference_tag,omitempty"`
	GroupLocation         string         `json:"group_location,omitempty"`           // Added Group / Location field
	CurrentMessageAreaID  int            `json:"current_message_area_id,omitempty"`  // Added for default area tracking
	CurrentMessageAreaTag string         `json:"current_message_area_tag,omitempty"` // Added for default area tracking
	LastReadMessageIDs    map[int]string `json:"last_read_message_ids,omitempty"`    // Map AreaID -> Last Read Message UUID string

	// File System Related
	CurrentFileAreaID  int         `json:"current_file_area_id,omitempty"`  // Added for default file area tracking
	CurrentFileAreaTag string      `json:"current_file_area_tag,omitempty"` // Added for default file area tracking
	TaggedFileIDs      []uuid.UUID `json:"tagged_file_ids,omitempty"`       // List of FileRecord IDs marked for batch download

	// Message System Related
	TaggedMessageAreaIDs []int `json:"tagged_message_area_ids,omitempty"` // List of message area IDs tagged for newscan

	// Terminal Preferences
	ScreenWidth  int `json:"screenWidth,omitempty"`  // Detected/preferred terminal width (default 80)
	ScreenHeight int `json:"screenHeight,omitempty"` // Detected/preferred terminal height (default 25)
	MsgHdr       int `json:"msgHdr,omitempty"`       // Selected message header style (1-14, 0=unset)

	// User Configuration Preferences
	HotKeys          bool   `json:"hotKeys,omitempty"`
	MorePrompts      bool   `json:"morePrompts,omitempty"`
	FullScreenEditor bool   `json:"fullScreenEditor,omitempty"`
	CustomPrompt     string `json:"customPrompt,omitempty"`
	OutputMode       string `json:"outputMode,omitempty"`
	Colors           [7]int `json:"colors,omitempty"`

	// Soft Delete (user marked as deleted but data preserved)
	DeletedUser bool       `json:"deletedUser,omitempty"` // True if user is soft-deleted
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`   // Timestamp when user was deleted (nil if not deleted)
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

/* // Moving UserManager and its methods to manager.go
type UserManager struct {
	users      map[string]*User
	mu         sync.RWMutex
	path       string
	LastLogins []LoginEvent // Exported slice to store recent logins
}

func NewUserManager(dataPath string) (*UserManager, error) {
	um := &UserManager{
		users:      make(map[string]*User),
		path:       filepath.Join(dataPath, "users.json"),
		LastLogins: make([]LoginEvent, 0, MaxLastLogins), // Initialize slice
	}

	if err := um.loadUsers(); err != nil {
		return nil, err
	}

	return um, nil
}

func (um *UserManager) loadUsers() error {
	// Read file first, outside the lock
	data, err := os.ReadFile(um.path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, create default user and save.
			// Lock inside SaveUsers will handle concurrent saves.
			um.mu.Lock() // Lock needed here to safely modify um.users map
			um.users["admin"] = &User{
				Username:    "admin",
				Password:    "admin", // In production, this should be hashed
				Handle:      "SysOp",
				AccessLevel: 255,
				// Initialize time fields to avoid zero time issues later if needed
				LastLogin:   time.Time{}, // Or time.Now() if preferred for first save?
				TimesCalled: 0,
			}
			um.mu.Unlock()
			// SaveUsers will acquire its own lock
			return um.SaveUsers()
		}
		// Other read error
		return err
	}

	// File exists, acquire lock to unmarshal into the map
	um.mu.Lock()
	defer um.mu.Unlock()
	return json.Unmarshal(data, &um.users)
}

func (um *UserManager) SaveUsers() error {
	// Acquire write lock before saving
	um.mu.Lock()
	defer um.mu.Unlock()

	data, err := json.MarshalIndent(um.users, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(um.path, data, 0600)
}

func (um *UserManager) Authenticate(username, password string) (*User, bool) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, exists := um.users[username]
	if !exists {
		return nil, false
	}

	// In production, use proper password hashing
	if user.Password != password {
		return nil, false
	}

	return user, true
}

// GetUser retrieves a user by username without checking password
func (um *UserManager) GetUser(username string) (*User, bool) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, exists := um.users[username]
	return user, exists
}

func (um *UserManager) CreateUser(username, password, handle, realName, phoneNum string) error {
	um.mu.Lock() // Lock before checking/modifying map

	if _, exists := um.users[username]; exists {
		um.mu.Unlock() // Unlock before returning error
		return os.ErrExist
	}

	um.users[username] = &User{
		Username:    username,
		Password:    password, // In production, this should be hashed
		Handle:      handle,
		RealName:    realName,
		PhoneNumber: phoneNum,
		AccessLevel: 1, // Default access level for new users
		// Initialize time fields
		LastLogin:        time.Time{},
		LastBulletinRead: time.Time{},
		CreatedAt:        time.Now(),
	}

	// Unlock map *before* calling SaveUsers (which has its own lock)
	um.mu.Unlock()

	return um.SaveUsers()
}
*/
