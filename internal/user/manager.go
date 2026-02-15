package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt" // Import bcrypt
)

// Predefined errors for user management
var (
	ErrUserNotFound = errors.New("user not found")
	ErrUserExists   = errors.New("username already exists")
	ErrHandleExists = errors.New("handle already exists")
	// MaxLastLogins   = 10 // Removed MaxLastLogins constant
)

const (
	userFile         = "users.json"
	callHistoryFile  = "callhistory.json"  // Filename for call history
	callNumberFile   = "callnumber.json"   // Filename for the next call number
	adminLogFile     = "admin_activity.json" // Filename for admin activity log
	callHistoryLimit = 20                  // Max number of call records to keep
	adminLogLimit    = 1000                // Max number of admin log entries to keep
)

// UserMgr manages user data (Renamed from UserManager)
type UserMgr struct {
	users    map[string]*User
	mu       sync.RWMutex
	path     string // Path to users.json
	dataPath string // Path to the data directory (for callhistory.json etc)
	// LastLogins  []LoginEvent // Removed LastLogins field
	nextUserID     int             // Added to track the next available user ID
	callHistory    []CallRecord    // Added slice for call history
	nextCallNumber uint64          // Added counter for overall calls
	activeUserIDs  map[int32]bool  // Track which user IDs are currently online
}

// NewUserManager creates and initializes a new user manager
func NewUserManager(dataPath string) (*UserMgr, error) { // Return renamed type
	um := &UserMgr{ // Use renamed type
		users:    make(map[string]*User),
		path:     filepath.Join(dataPath, userFile), // userFile path uses dataPath now
		dataPath: dataPath,                          // Store the data path
		// LastLogins:  make([]LoginEvent, 0, MaxLastLogins), // Removed LastLogins initialization
		callHistory:    make([]CallRecord, 0, callHistoryLimit), // Initialize call history
		nextUserID:     1,                                       // Start user IDs from 1
		nextCallNumber: 1,                                       // Start call numbers from 1
		activeUserIDs:  make(map[int32]bool),                    // Initialize online user tracking
	}

	// Removed call to loadLastLogins

	// Load call history using the stored dataPath
	if err := um.loadCallHistory(); err != nil {
		// Log warning but continue
		log.Printf("WARN: Failed to load call history: %v", err)
	}

	// Load the next call number
	if err := um.loadNextCallNumber(); err != nil {
		// Log warning but continue, using the default start value of 1
		log.Printf("WARN: Failed to load next call number: %v", err)
	}

	if err := um.loadUsers(); err != nil {
		// If loading fails (e.g., file not found), create default felonius user
		if os.IsNotExist(err) {
			log.Println("INFO: users.json not found, creating default felonius user.")
			// AddUser will handle ID assignment and initialization
			// Using "password" as default password
			defaultUser, addErr := um.AddUser("felonius", "password", "Felonius", "Felonius", "", "FAiRLiGHT/PC")
			if addErr != nil {
				return nil, fmt.Errorf("failed to create default felonius user: %w", addErr)
			}
			// Update felonius user fields after AddUser returns it
			um.mu.Lock()                 // Lock necessary for direct modification
			defaultUser.AccessLevel = 10 // Default user level
			defaultUser.Validated = true // Default user is validated
			// Ensure we update the map entry after modifying
			um.users[strings.ToLower(defaultUser.Username)] = defaultUser
			um.mu.Unlock()

			// Save again to ensure level/validation is persisted
			if saveErr := um.SaveUsers(); saveErr != nil {
				return nil, fmt.Errorf("failed to save default felonius user details: %w", saveErr)
			}
			log.Println("INFO: Default felonius user created (felonius/password).")
			// Determine next user ID after creating the default user
			um.determineNextUserID()
			return um, nil // Return successfully after creating default
		} else {
			// Other load error
			return nil, fmt.Errorf("failed to load users: %w", err)
		}
	}
	// If load was successful, determine nextUserID
	um.determineNextUserID()
	return um, nil
}

// loadUsers loads user data from the JSON file.
func (um *UserMgr) loadUsers() error { // Receiver uses renamed type
	data, err := os.ReadFile(um.path)
	if err != nil {
		return err // Return error to NewUserManager to handle
	}

	// Temporary slice to hold users from JSON array
	// We load into a slice because the JSON is an array.
	var usersList []*User // Load into a slice of pointers to handle omitempty correctly
	if err := json.Unmarshal(data, &usersList); err != nil {
		return fmt.Errorf("failed to unmarshal users array: %w", err)
	}

	um.mu.Lock()
	defer um.mu.Unlock()
	// Ensure map is initialized
	if um.users == nil {
		um.users = make(map[string]*User)
	}

	// Populate the map from the slice
	for _, user := range usersList { // Iterate directly over the slice of pointers
		if user == nil { // Safety check for nil entries in JSON array
			continue
		}
		lowerUsername := strings.ToLower(user.Username)
		if _, exists := um.users[lowerUsername]; exists {
			log.Printf("WARN: Duplicate username found in users.json: %s. Skipping subsequent entry.", user.Username)
			continue
		}
		um.users[lowerUsername] = user
		log.Printf("TRACE: Loaded user %s (Handle: %s, Group: %s) from JSON.", user.Username, user.Handle, user.GroupLocation)
	}

	// Note: determineNextUserID should be called *after* successful load
	// but *outside* the lock (or re-acquire read lock if needed) if called from NewUserManager.
	// It's called from NewUserManager after this returns.
	return nil
}

// determineNextUserID finds the max existing ID and sets nextUserID appropriately.
// Should be called after users are loaded.
func (um *UserMgr) determineNextUserID() { // Receiver uses renamed type
	um.mu.RLock() // Use read lock
	maxID := 0
	for _, u := range um.users {
		if u.ID > maxID {
			maxID = u.ID
		}
	}
	um.mu.RUnlock()

	um.mu.Lock() // Need write lock to set nextUserID
	um.nextUserID = maxID + 1
	log.Printf("DEBUG: Determined next User ID to be %d", um.nextUserID)
	um.mu.Unlock()
}

// loadCallHistory loads the call history events from JSON.
// Now uses um.dataPath internally.
func (um *UserMgr) loadCallHistory() error {
	filePath := filepath.Join(um.dataPath, callHistoryFile) // Use stored dataPath
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: %s not found, starting with empty call history list.", callHistoryFile)
			return nil // Not an error if the file doesn't exist yet
		}
		return fmt.Errorf("failed to read %s: %w", callHistoryFile, err)
	}

	if len(data) == 0 {
		return nil // Empty file is okay
	}

	um.mu.Lock() // Lock before modifying internal slice
	defer um.mu.Unlock()
	// Ensure slice exists
	if um.callHistory == nil {
		um.callHistory = make([]CallRecord, 0, callHistoryLimit)
	}
	if err := json.Unmarshal(data, &um.callHistory); err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", callHistoryFile, err)
	}
	// Ensure capacity and length limits are respected after loading
	if len(um.callHistory) > callHistoryLimit {
		startIdx := len(um.callHistory) - callHistoryLimit
		um.callHistory = um.callHistory[startIdx:]
	}
	log.Printf("DEBUG: Loaded %d call history records from %s", len(um.callHistory), callHistoryFile)
	return nil
}

// loadNextCallNumber loads the next call number from its dedicated JSON file.
func (um *UserMgr) loadNextCallNumber() error {
	filePath := filepath.Join(um.dataPath, callNumberFile)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: %s not found, starting call numbers from 1.", callNumberFile)
			// Keep the default um.nextCallNumber = 1
			return nil // Not an error if the file doesn't exist
		}
		return fmt.Errorf("failed to read %s: %w", callNumberFile, err)
	}

	if len(data) == 0 {
		log.Printf("WARN: %s is empty, starting call numbers from 1.", callNumberFile)
		return nil // Empty file, use default
	}

	um.mu.Lock() // Lock before modifying
	defer um.mu.Unlock()
	if err := json.Unmarshal(data, &um.nextCallNumber); err != nil {
		// If unmarshal fails, log and keep the default
		log.Printf("WARN: Failed to unmarshal %s: %v. Starting call numbers from 1.", callNumberFile, err)
		um.nextCallNumber = 1
		return nil // Don't return error, just use default
	}

	log.Printf("DEBUG: Loaded next call number %d from %s", um.nextCallNumber, callNumberFile)
	return nil
}

// saveCallHistoryLocked saves the current callHistory slice to JSON (assumes lock is held).
// Now uses um.dataPath internally.
func (um *UserMgr) saveCallHistoryLocked() error {
	if um.callHistory == nil {
		// Avoid marshaling nil slice, treat as empty
		um.callHistory = make([]CallRecord, 0)
	}
	data, err := json.MarshalIndent(um.callHistory, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal call history: %w", err)
	}

	filePath := filepath.Join(um.dataPath, callHistoryFile) // Use stored dataPath
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", callHistoryFile, err)
	}

	// Also save the next call number (atomically with history? separate file is simpler for now)
	if err := um.saveNextCallNumberLocked(); err != nil {
		// Log error but don't fail the history save if number save fails
		log.Printf("ERROR: Failed to save next call number: %v", err)
	}

	return nil
}

// saveNextCallNumberLocked saves the current nextCallNumber to its JSON file (assumes lock is held).
func (um *UserMgr) saveNextCallNumberLocked() error {
	data, err := json.Marshal(um.nextCallNumber) // Simple marshal, no indent needed
	if err != nil {
		return fmt.Errorf("failed to marshal next call number: %w", err)
	}

	filePath := filepath.Join(um.dataPath, callNumberFile)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", callNumberFile, err)
	}
	return nil
}

// saveUsersLocked performs the actual saving without acquiring locks.
// Uses um.path (which should point to data/users.json)
func (um *UserMgr) saveUsersLocked() error { // Receiver uses renamed type
	// Convert map back to slice for saving as JSON array
	// We now store pointers in the map, so create a slice of pointers
	usersList := make([]*User, 0, len(um.users))
	for _, user := range um.users {
		usersList = append(usersList, user) // Append pointers directly
	}

	data, err := json.MarshalIndent(usersList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users slice: %w", err)
	}

	// Ensure the directory exists before writing the file
	dir := filepath.Dir(um.path)
	if err := os.MkdirAll(dir, 0750); err != nil { // Use 0750 for permissions
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// WriteFile ensures atomic write (usually via temp file)
	if err = os.WriteFile(um.path, data, 0600); err != nil {
		return fmt.Errorf("failed to write users file %s: %w", um.path, err) // Include path in error
	}
	return nil
}

// SaveUsers saves the current user data to the JSON file (acquires lock).
func (um *UserMgr) SaveUsers() error { // Receiver uses renamed type
	um.mu.Lock()
	defer um.mu.Unlock()
	return um.saveUsersLocked()
}

// UpdateUser copies the modified user back into the internal map and saves to disk.
// Use this instead of SaveUsers when you have modified a user copy obtained from
// GetUser or Authenticate and need those changes persisted.
func (um *UserMgr) UpdateUser(u *User) error {
	if u == nil {
		return fmt.Errorf("cannot update nil user")
	}
	um.mu.Lock()
	defer um.mu.Unlock()
	lowerUsername := strings.ToLower(u.Username)
	if _, exists := um.users[lowerUsername]; !exists {
		return ErrUserNotFound
	}
	// Create a defensive copy to prevent external mutations from bypassing locks
	userCopy := *u
	um.users[lowerUsername] = &userCopy
	return um.saveUsersLocked()
}

// LogAdminActivity logs an administrative action to the activity log file
func (um *UserMgr) LogAdminActivity(logEntry AdminActivityLog) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	// Load existing logs
	logPath := filepath.Join(filepath.Dir(um.path), adminLogFile)
	var logs []AdminActivityLog

	// Try to load existing logs
	if data, err := os.ReadFile(logPath); err == nil {
		_ = json.Unmarshal(data, &logs) // Ignore errors, start fresh if corrupt
	}

	// Add new entry
	logEntry.ID = len(logs) + 1
	logEntry.Timestamp = time.Now()
	logs = append(logs, logEntry)

	// Keep only recent entries (prevent file from growing indefinitely)
	if len(logs) > adminLogLimit {
		logs = logs[len(logs)-adminLogLimit:]
	}

	// Save logs
	data, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal admin logs: %w", err)
	}

	if err := os.WriteFile(logPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write admin log: %w", err)
	}

	return nil
}

// Authenticate checks username and compares password hash.
// Returns: (user, success)
func (um *UserMgr) Authenticate(username, password string) (*User, bool) { // Receiver uses renamed type
	lowerUsername := strings.ToLower(username)

	um.mu.RLock()
	user, exists := um.users[lowerUsername]
	if !exists {
		um.mu.RUnlock()
		return nil, false
	}
	// Deny login if user is deleted
	if user.DeletedUser {
		um.mu.RUnlock()
		return nil, false
	}
	// Copy the password hash while holding the read lock
	passwordHash := user.PasswordHash
	um.mu.RUnlock()

	// Compare hashed password outside any lock (bcrypt is CPU-intensive)
	err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err != nil {
		return nil, false
	}

	// Authentication successful - update LastLogin and TimesCalled
	um.mu.Lock()
	user = um.users[lowerUsername] // Re-fetch under write lock
	if user == nil {
		um.mu.Unlock()
		return nil, false
	}
	user.LastLogin = time.Now()
	user.TimesCalled++
	um.mu.Unlock()

	// Save outside the write lock to avoid blocking other user operations
	if err := um.SaveUsers(); err != nil {
		log.Printf("ERROR: Failed to save user data after successful login for %s: %v", username, err)
	}

	// Return a copy
	um.mu.RLock()
	userCopy := *um.users[lowerUsername]
	um.mu.RUnlock()
	return &userCopy, true
}

// GetUser retrieves a user by username.
// Returns a copy to prevent callers from mutating internal state without the lock.
func (um *UserMgr) GetUser(username string) (*User, bool) { // Receiver uses renamed type
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, exists := um.users[strings.ToLower(username)] // Use lower case for lookup
	if !exists {
		return nil, false
	}
	userCopy := *user
	return &userCopy, true
}

// GetUserByHandle retrieves a user by their handle (case-insensitive search).
func (um *UserMgr) GetUserByHandle(handle string) (*User, bool) { // Receiver uses renamed type
	um.mu.RLock()
	defer um.mu.RUnlock()

	lowerHandle := strings.ToLower(handle)
	for _, user := range um.users {
		if strings.ToLower(user.Handle) == lowerHandle {
			// Return a copy to prevent modification of the internal user data
			userCopy := *user
			return &userCopy, true
		}
	}
	return nil, false
}

// GetUserByID returns a user by their ID (for optimistic locking checks)
func (um *UserMgr) GetUserByID(id int) (*User, bool) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	for _, user := range um.users {
		if user.ID == id {
			// Return a copy to prevent modification of the internal user data
			userCopy := *user
			return &userCopy, true
		}
	}
	return nil, false
}

// NextUserID returns the ID that will be assigned to the next new user.
func (um *UserMgr) NextUserID() int {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.nextUserID
}

// AddUser creates a new user, hashes the password, assigns an ID, and saves.
// Added GroupLocation parameter.
func (um *UserMgr) AddUser(username, password, handle, realName, phoneNum, groupLocation string) (*User, error) { // Receiver uses renamed type
	lowerUsername := strings.ToLower(username)
	lowerHandle := strings.ToLower(handle)

	um.mu.Lock()
	defer um.mu.Unlock()

	// Check if username already exists
	if _, exists := um.users[lowerUsername]; exists {
		return nil, ErrUserExists
	}

	// Check if handle already exists
	for _, u := range um.users {
		if strings.ToLower(u.Handle) == lowerHandle {
			return nil, ErrHandleExists
		}
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create new user
	newUser := &User{
		ID:            um.nextUserID, // Assign the next available ID
		Username:      username,
		PasswordHash:  string(hashedPassword),
		Handle:        handle,
		RealName:      realName,
		PhoneNumber:   phoneNum,
		GroupLocation: groupLocation,
		AccessLevel:   1,           // Default access level for new users
		TimeLimit:     60,          // Default time limit (e.g., 60 minutes)
		Validated:     false,       // New users require validation
		LastLogin:     time.Time{}, // Initialize zero time
		// Initialize other fields as needed
	}

	// Add to map and increment nextUserID
	um.users[lowerUsername] = newUser
	um.nextUserID++

	// Save the updated user list *while still holding the lock*
	if err := um.saveUsersLocked(); err != nil {
		log.Printf("ERROR: Failed to save users after adding %s: %v", username, err)
		// If save fails, should we attempt to roll back the in-memory add?
		// For now, return the error, the user exists in memory but not saved.
		delete(um.users, lowerUsername) // Rollback in-memory add on save failure
		um.nextUserID--                 // Rollback ID increment
		return nil, err
	}

	log.Printf("INFO: Added user %s (Handle: %s, ID: %d)", newUser.Username, newUser.Handle, newUser.ID)
	return newUser, nil // Return the newly created user
}

// AddCallRecord adds a call record to the history and saves.
func (um *UserMgr) AddCallRecord(record CallRecord) {
	um.mu.Lock()
	defer um.mu.Unlock()

	// Ensure slice exists
	if um.callHistory == nil {
		um.callHistory = make([]CallRecord, 0, callHistoryLimit)
	}

	// Assign the current call number and increment the counter
	record.CallNumber = um.nextCallNumber
	um.nextCallNumber++

	// Append the new record
	um.callHistory = append(um.callHistory, record)

	// Limit the size of the history
	if len(um.callHistory) > callHistoryLimit {
		// Remove the oldest entry
		um.callHistory = um.callHistory[1:]
	}

	// Save the updated history *while still holding the lock*
	if err := um.saveCallHistoryLocked(); err != nil {
		log.Printf("ERROR: Failed to save call history after adding record for user %d: %v", record.UserID, err)
		// Maybe try to rollback the append? Less critical than user add.
	}
}

// GetLastCallers retrieves the recent call history (from memory).
func (um *UserMgr) GetLastCallers() []CallRecord {
	um.mu.RLock()
	defer um.mu.RUnlock()

	// Return a copy to prevent modification of the internal slice
	historyCopy := make([]CallRecord, len(um.callHistory))
	copy(historyCopy, um.callHistory)
	return historyCopy
}

// GetAllUsers returns a slice containing copies of all user records.
// Returns copies to prevent callers from mutating internal state.
func (um *UserMgr) GetAllUsers() []*User {
	um.mu.RLock()
	defer um.mu.RUnlock()

	usersSlice := make([]*User, 0, len(um.users))
	for _, user := range um.users {
		userCopy := *user
		usersSlice = append(usersSlice, &userCopy)
	}
	return usersSlice
}

// MarkUserOnline marks a user as currently online/connected
func (um *UserMgr) MarkUserOnline(userID int) {
	um.mu.Lock()
	defer um.mu.Unlock()
	um.activeUserIDs[int32(userID)] = true
	log.Printf("DEBUG: User ID %d marked as ONLINE", userID)
}

// MarkUserOffline marks a user as offline/disconnected
func (um *UserMgr) MarkUserOffline(userID int) {
	um.mu.Lock()
	defer um.mu.Unlock()
	delete(um.activeUserIDs, int32(userID))
	log.Printf("DEBUG: User ID %d marked as OFFLINE", userID)
}

// IsUserOnline returns true if the user is currently connected
func (um *UserMgr) IsUserOnline(userID int) bool {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.activeUserIDs[int32(userID)]
}

