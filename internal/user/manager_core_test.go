package user

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// --- Helpers ---

// newTestManager creates a UserMgr backed by a temp directory.
// If seedUsers is non-nil the slice is written to users.json before init.
func newTestManager(t *testing.T, seedUsers []User) *UserMgr {
	t.Helper()
	tmpDir := t.TempDir()

	if seedUsers != nil {
		data, err := json.Marshal(seedUsers)
		if err != nil {
			t.Fatalf("marshal seed users: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "users.json"), data, 0644); err != nil {
			t.Fatalf("write seed users.json: %v", err)
		}
	}

	um, err := NewUserManager(tmpDir)
	if err != nil {
		t.Fatalf("NewUserManager: %v", err)
	}
	return um
}

// hashPassword is a test helper that bcrypt-hashes a plaintext password.
func hashPassword(t *testing.T, plain string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt hash: %v", err)
	}
	return string(h)
}

// --- NewUserManager ---

func TestNewUserManager_EmptyDir_CreatesDefault(t *testing.T) {
	tmpDir := t.TempDir()
	um, err := NewUserManager(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should create a default "felonius" user
	u, ok := um.GetUser("felonius")
	if !ok {
		t.Fatal("expected default felonius user to exist")
	}
	if u.Handle != "Felonius" {
		t.Errorf("expected handle Felonius, got %q", u.Handle)
	}
	if u.AccessLevel != 10 {
		t.Errorf("expected access level 10, got %d", u.AccessLevel)
	}
	if !u.Validated {
		t.Error("expected default user to be validated")
	}

	// users.json should exist on disk
	if _, err := os.Stat(filepath.Join(tmpDir, "users.json")); os.IsNotExist(err) {
		t.Error("expected users.json to be created on disk")
	}
}

func TestNewUserManager_LoadsExistingUsers(t *testing.T) {
	seed := []User{
		{ID: 1, Username: "alice", Handle: "Alice", PasswordHash: hashPassword(t, "pw1"), AccessLevel: 5},
		{ID: 2, Username: "bob", Handle: "Bob", PasswordHash: hashPassword(t, "pw2"), AccessLevel: 10},
	}
	um := newTestManager(t, seed)

	if um.GetUserCount() != 2 {
		t.Errorf("expected 2 users, got %d", um.GetUserCount())
	}

	alice, ok := um.GetUser("alice")
	if !ok {
		t.Fatal("expected alice to exist")
	}
	if alice.AccessLevel != 5 {
		t.Errorf("expected access level 5, got %d", alice.AccessLevel)
	}
}

func TestNewUserManager_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "users.json"), []byte("{bad json"), 0644)

	_, err := NewUserManager(tmpDir)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestNewUserManager_UTF8BOM(t *testing.T) {
	seed := []User{{ID: 1, Username: "bomuser", Handle: "BOM", AccessLevel: 1}}
	data, _ := json.Marshal(seed)
	bom := append([]byte{0xEF, 0xBB, 0xBF}, data...)

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "users.json"), bom, 0644)

	um, err := NewUserManager(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error with BOM: %v", err)
	}
	if _, ok := um.GetUser("bomuser"); !ok {
		t.Error("expected bomuser to be loaded despite BOM prefix")
	}
}

// --- AddUser ---

func TestAddUser_Success(t *testing.T) {
	um := newTestManager(t, []User{})

	u, err := um.AddUser("newuser", "secret123", "NewHandle", "Real Name", "555-1234", "SomeGroup")
	if err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	if u.Username != "newuser" {
		t.Errorf("expected username newuser, got %q", u.Username)
	}
	if u.Handle != "NewHandle" {
		t.Errorf("expected handle NewHandle, got %q", u.Handle)
	}
	if u.RealName != "Real Name" {
		t.Errorf("expected real name, got %q", u.RealName)
	}
	if u.GroupLocation != "SomeGroup" {
		t.Errorf("expected group location, got %q", u.GroupLocation)
	}
	if u.ID < 1 {
		t.Errorf("expected positive ID, got %d", u.ID)
	}

	// Password should be hashed (bcrypt)
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("secret123")); err != nil {
		t.Error("password hash does not match plaintext")
	}
}

func TestAddUser_DuplicateUsername(t *testing.T) {
	seed := []User{{ID: 1, Username: "existing", Handle: "Existing", AccessLevel: 1}}
	um := newTestManager(t, seed)

	_, err := um.AddUser("existing", "pw", "DifferentHandle", "", "", "")
	if err != ErrUserExists {
		t.Errorf("expected ErrUserExists, got %v", err)
	}
}

func TestAddUser_DuplicateUsername_CaseInsensitive(t *testing.T) {
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", AccessLevel: 1}}
	um := newTestManager(t, seed)

	_, err := um.AddUser("ALICE", "pw", "DifferentHandle", "", "", "")
	if err != ErrUserExists {
		t.Errorf("expected ErrUserExists for case-insensitive match, got %v", err)
	}
}

func TestAddUser_DuplicateHandle(t *testing.T) {
	seed := []User{{ID: 1, Username: "user1", Handle: "TakenHandle", AccessLevel: 1}}
	um := newTestManager(t, seed)

	_, err := um.AddUser("user2", "pw", "TakenHandle", "", "", "")
	if err != ErrHandleExists {
		t.Errorf("expected ErrHandleExists, got %v", err)
	}
}

func TestAddUser_DuplicateHandle_CaseInsensitive(t *testing.T) {
	seed := []User{{ID: 1, Username: "user1", Handle: "MyHandle", AccessLevel: 1}}
	um := newTestManager(t, seed)

	_, err := um.AddUser("user2", "pw", "myhandle", "", "", "")
	if err != ErrHandleExists {
		t.Errorf("expected ErrHandleExists for case-insensitive handle match, got %v", err)
	}
}

func TestAddUser_IDsIncrement(t *testing.T) {
	um := newTestManager(t, []User{})

	u1, _ := um.AddUser("first", "pw", "First", "", "", "")
	u2, _ := um.AddUser("second", "pw", "Second", "", "", "")

	if u2.ID <= u1.ID {
		t.Errorf("expected u2.ID (%d) > u1.ID (%d)", u2.ID, u1.ID)
	}
}

func TestAddUser_PersistsToDisk(t *testing.T) {
	tmpDir := t.TempDir()
	// Write empty users array
	os.WriteFile(filepath.Join(tmpDir, "users.json"), []byte("[]"), 0644)

	um, _ := NewUserManager(tmpDir)
	um.AddUser("persist", "pw", "PersistHandle", "", "", "")

	// Reload from disk
	um2, err := NewUserManager(tmpDir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, ok := um2.GetUser("persist"); !ok {
		t.Error("expected user to persist to disk after AddUser")
	}
}

func TestAddUser_UsesNewUserLevel(t *testing.T) {
	um := newTestManager(t, []User{})
	um.SetNewUserLevel(50)

	u, err := um.AddUser("leveltest", "pw", "LevelHandle", "", "", "")
	if err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	if u.AccessLevel != 50 {
		t.Errorf("expected access level 50, got %d", u.AccessLevel)
	}
}

// --- GetUser ---

func TestGetUser_Exists(t *testing.T) {
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", AccessLevel: 5}}
	um := newTestManager(t, seed)

	u, ok := um.GetUser("alice")
	if !ok {
		t.Fatal("expected alice to exist")
	}
	if u.Handle != "Alice" {
		t.Errorf("expected handle Alice, got %q", u.Handle)
	}
}

func TestGetUser_CaseInsensitive(t *testing.T) {
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", AccessLevel: 5}}
	um := newTestManager(t, seed)

	_, ok := um.GetUser("ALICE")
	if !ok {
		t.Error("expected case-insensitive lookup to find alice")
	}
}

func TestGetUser_NotFound(t *testing.T) {
	um := newTestManager(t, []User{})

	_, ok := um.GetUser("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent user")
	}
}

func TestGetUser_ReturnsCopy(t *testing.T) {
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", AccessLevel: 5}}
	um := newTestManager(t, seed)

	u1, _ := um.GetUser("alice")
	u1.Handle = "MODIFIED"

	u2, _ := um.GetUser("alice")
	if u2.Handle == "MODIFIED" {
		t.Error("GetUser should return a copy; internal data was mutated")
	}
}

// --- GetUserByHandle ---

func TestGetUserByHandle_Found(t *testing.T) {
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", AccessLevel: 5}}
	um := newTestManager(t, seed)

	u, ok := um.GetUserByHandle("Alice")
	if !ok {
		t.Fatal("expected to find user by handle Alice")
	}
	if u.Username != "alice" {
		t.Errorf("expected username alice, got %q", u.Username)
	}
}

func TestGetUserByHandle_CaseInsensitive(t *testing.T) {
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", AccessLevel: 5}}
	um := newTestManager(t, seed)

	_, ok := um.GetUserByHandle("alice")
	if !ok {
		t.Error("expected case-insensitive handle lookup to work")
	}
}

func TestGetUserByHandle_NotFound(t *testing.T) {
	um := newTestManager(t, []User{})

	_, ok := um.GetUserByHandle("NoSuchHandle")
	if ok {
		t.Error("expected not found for nonexistent handle")
	}
}

// --- GetUserByID ---

func TestGetUserByID_Found(t *testing.T) {
	seed := []User{{ID: 42, Username: "alice", Handle: "Alice", AccessLevel: 5}}
	um := newTestManager(t, seed)

	u, ok := um.GetUserByID(42)
	if !ok {
		t.Fatal("expected to find user by ID 42")
	}
	if u.Username != "alice" {
		t.Errorf("expected username alice, got %q", u.Username)
	}
}

func TestGetUserByID_NotFound(t *testing.T) {
	um := newTestManager(t, []User{})

	_, ok := um.GetUserByID(999)
	if ok {
		t.Error("expected not found for nonexistent ID")
	}
}

// --- Authenticate ---

func TestAuthenticate_Success(t *testing.T) {
	pw := "correcthorse"
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", PasswordHash: hashPassword(t, pw), AccessLevel: 5}}
	um := newTestManager(t, seed)

	u, ok := um.Authenticate("alice", pw)
	if !ok {
		t.Fatal("expected authentication to succeed")
	}
	if u.Username != "alice" {
		t.Errorf("expected username alice, got %q", u.Username)
	}
}

func TestAuthenticate_CaseInsensitiveUsername(t *testing.T) {
	pw := "mypass"
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", PasswordHash: hashPassword(t, pw), AccessLevel: 5}}
	um := newTestManager(t, seed)

	_, ok := um.Authenticate("ALICE", pw)
	if !ok {
		t.Error("expected case-insensitive username auth to succeed")
	}
}

func TestAuthenticate_WrongPassword(t *testing.T) {
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", PasswordHash: hashPassword(t, "correct"), AccessLevel: 5}}
	um := newTestManager(t, seed)

	_, ok := um.Authenticate("alice", "wrong")
	if ok {
		t.Error("expected authentication to fail with wrong password")
	}
}

func TestAuthenticate_NonexistentUser(t *testing.T) {
	um := newTestManager(t, []User{})

	_, ok := um.Authenticate("ghost", "pw")
	if ok {
		t.Error("expected authentication to fail for nonexistent user")
	}
}

func TestAuthenticate_DeletedUser(t *testing.T) {
	deletedAt := time.Now()
	seed := []User{{
		ID: 1, Username: "deleted", Handle: "Del", PasswordHash: hashPassword(t, "pw"),
		AccessLevel: 5, DeletedUser: true, DeletedAt: &deletedAt,
	}}
	um := newTestManager(t, seed)

	_, ok := um.Authenticate("deleted", "pw")
	if ok {
		t.Error("expected authentication to fail for deleted user")
	}
}

func TestAuthenticate_UpdatesLastLoginAndTimesCalled(t *testing.T) {
	pw := "pass"
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", PasswordHash: hashPassword(t, pw), AccessLevel: 5, TimesCalled: 5}}
	um := newTestManager(t, seed)

	before := time.Now()
	u, ok := um.Authenticate("alice", pw)
	if !ok {
		t.Fatal("auth should succeed")
	}

	if u.TimesCalled != 6 {
		t.Errorf("expected TimesCalled=6, got %d", u.TimesCalled)
	}
	if u.LastLogin.Before(before) {
		t.Error("expected LastLogin to be updated to recent time")
	}
}

// --- UpdateUser ---

func TestUpdateUser_Success(t *testing.T) {
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", AccessLevel: 5}}
	um := newTestManager(t, seed)

	u, _ := um.GetUser("alice")
	u.AccessLevel = 100
	u.Handle = "AliceUpdated"

	if err := um.UpdateUser(u); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	updated, _ := um.GetUser("alice")
	if updated.AccessLevel != 100 {
		t.Errorf("expected access level 100, got %d", updated.AccessLevel)
	}
	if updated.Handle != "AliceUpdated" {
		t.Errorf("expected handle AliceUpdated, got %q", updated.Handle)
	}
}

func TestUpdateUser_NonexistentUser(t *testing.T) {
	um := newTestManager(t, []User{})

	ghost := &User{Username: "ghost", Handle: "Ghost"}
	err := um.UpdateUser(ghost)
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUpdateUser_NilUser(t *testing.T) {
	um := newTestManager(t, []User{})

	err := um.UpdateUser(nil)
	if err == nil {
		t.Error("expected error for nil user")
	}
}

func TestUpdateUser_PersistsToDisk(t *testing.T) {
	tmpDir := t.TempDir()
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", AccessLevel: 5}}
	data, _ := json.Marshal(seed)
	os.WriteFile(filepath.Join(tmpDir, "users.json"), data, 0644)

	um, _ := NewUserManager(tmpDir)
	u, _ := um.GetUser("alice")
	u.AccessLevel = 200
	um.UpdateUser(u)

	// Reload from disk
	um2, _ := NewUserManager(tmpDir)
	reloaded, _ := um2.GetUser("alice")
	if reloaded.AccessLevel != 200 {
		t.Errorf("expected persisted access level 200, got %d", reloaded.AccessLevel)
	}
}

func TestUpdateUser_DefensiveCopy(t *testing.T) {
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", AccessLevel: 5}}
	um := newTestManager(t, seed)

	u, _ := um.GetUser("alice")
	u.AccessLevel = 99
	um.UpdateUser(u)

	// Mutate the user object after UpdateUser
	u.AccessLevel = 0

	// Internal state should not be affected
	internal, _ := um.GetUser("alice")
	if internal.AccessLevel != 99 {
		t.Errorf("expected internal state 99, got %d (defensive copy failed)", internal.AccessLevel)
	}
}

// --- GetAllUsers ---

func TestGetAllUsers(t *testing.T) {
	seed := []User{
		{ID: 1, Username: "alice", Handle: "Alice", AccessLevel: 5},
		{ID: 2, Username: "bob", Handle: "Bob", AccessLevel: 10},
	}
	um := newTestManager(t, seed)

	all := um.GetAllUsers()
	if len(all) != 2 {
		t.Errorf("expected 2 users, got %d", len(all))
	}

	// Verify they are copies
	for _, u := range all {
		u.Handle = "MODIFIED"
	}
	alice, _ := um.GetUser("alice")
	if alice.Handle == "MODIFIED" {
		t.Error("GetAllUsers should return copies")
	}
}

func TestGetAllUsers_Empty(t *testing.T) {
	um := newTestManager(t, []User{})
	all := um.GetAllUsers()
	if len(all) != 0 {
		t.Errorf("expected 0 users, got %d", len(all))
	}
}

// --- SetNewUserLevel ---

func TestSetNewUserLevel_Normal(t *testing.T) {
	um := newTestManager(t, []User{})
	um.SetNewUserLevel(42)

	u, _ := um.AddUser("test", "pw", "Test", "", "", "")
	if u.AccessLevel != 42 {
		t.Errorf("expected level 42, got %d", u.AccessLevel)
	}
}

func TestSetNewUserLevel_ClampsNegative(t *testing.T) {
	um := newTestManager(t, []User{})
	um.SetNewUserLevel(-5)

	u, _ := um.AddUser("test", "pw", "Test", "", "", "")
	if u.AccessLevel != 0 {
		t.Errorf("expected clamped level 0, got %d", u.AccessLevel)
	}
}

func TestSetNewUserLevel_ClampsHigh(t *testing.T) {
	um := newTestManager(t, []User{})
	um.SetNewUserLevel(999)

	u, _ := um.AddUser("test", "pw", "Test", "", "", "")
	if u.AccessLevel != 255 {
		t.Errorf("expected clamped level 255, got %d", u.AccessLevel)
	}
}

// --- NextUserID ---

func TestNextUserID(t *testing.T) {
	seed := []User{
		{ID: 5, Username: "alice", Handle: "Alice"},
		{ID: 10, Username: "bob", Handle: "Bob"},
	}
	um := newTestManager(t, seed)

	nextID := um.NextUserID()
	if nextID != 11 {
		t.Errorf("expected next ID 11 (max=10), got %d", nextID)
	}
}

// --- SaveUsers ---

func TestSaveUsers_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "sub", "data")

	// Write empty users so NewUserManager loads successfully
	os.MkdirAll(nestedDir, 0755)
	os.WriteFile(filepath.Join(nestedDir, "users.json"), []byte("[]"), 0644)

	um, _ := NewUserManager(nestedDir)
	um.AddUser("test", "pw", "Test", "", "", "")

	// Verify file was written
	data, err := os.ReadFile(filepath.Join(nestedDir, "users.json"))
	if err != nil {
		t.Fatalf("expected users.json to exist: %v", err)
	}
	if !strings.Contains(string(data), "test") {
		t.Error("expected users.json to contain 'test'")
	}
}

// --- Call History ---

func TestAddCallRecord_And_GetLastCallers(t *testing.T) {
	um := newTestManager(t, []User{})

	um.AddCallRecord(CallRecord{UserID: 1, Handle: "Alice", ConnectTime: time.Now()})
	um.AddCallRecord(CallRecord{UserID: 2, Handle: "Bob", ConnectTime: time.Now()})

	callers := um.GetLastCallers()
	if len(callers) != 2 {
		t.Errorf("expected 2 call records, got %d", len(callers))
	}
}

func TestAddCallRecord_AssignsCallNumber(t *testing.T) {
	um := newTestManager(t, []User{})

	um.AddCallRecord(CallRecord{UserID: 1, Handle: "Alice"})
	um.AddCallRecord(CallRecord{UserID: 2, Handle: "Bob"})

	callers := um.GetLastCallers()
	if callers[0].CallNumber != 1 {
		t.Errorf("expected first call number 1, got %d", callers[0].CallNumber)
	}
	if callers[1].CallNumber != 2 {
		t.Errorf("expected second call number 2, got %d", callers[1].CallNumber)
	}
}

func TestAddCallRecord_RespectsLimit(t *testing.T) {
	um := newTestManager(t, []User{})

	// Add more records than the limit
	for i := 0; i < callHistoryLimit+5; i++ {
		um.AddCallRecord(CallRecord{UserID: i, Handle: "User"})
	}

	callers := um.GetLastCallers()
	if len(callers) > callHistoryLimit {
		t.Errorf("expected at most %d records, got %d", callHistoryLimit, len(callers))
	}
}

func TestGetLastCallers_ReturnsCopy(t *testing.T) {
	um := newTestManager(t, []User{})
	um.AddCallRecord(CallRecord{UserID: 1, Handle: "Alice"})

	callers := um.GetLastCallers()
	callers[0].Handle = "MODIFIED"

	original := um.GetLastCallers()
	if original[0].Handle == "MODIFIED" {
		t.Error("GetLastCallers should return a copy")
	}
}

func TestCallHistory_PersistsToDisk(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "users.json"), []byte("[]"), 0644)

	um, _ := NewUserManager(tmpDir)
	um.AddCallRecord(CallRecord{UserID: 1, Handle: "Alice"})

	// Reload and check
	um2, _ := NewUserManager(tmpDir)
	callers := um2.GetLastCallers()
	if len(callers) != 1 {
		t.Errorf("expected 1 persisted call record, got %d", len(callers))
	}
}

// --- GetTotalCalls ---

func TestGetTotalCalls_Zero(t *testing.T) {
	um := newTestManager(t, []User{})
	if um.GetTotalCalls() != 0 {
		t.Errorf("expected 0 total calls for fresh manager, got %d", um.GetTotalCalls())
	}
}

func TestGetTotalCalls_AfterRecords(t *testing.T) {
	um := newTestManager(t, []User{})
	um.AddCallRecord(CallRecord{UserID: 1, Handle: "A"})
	um.AddCallRecord(CallRecord{UserID: 2, Handle: "B"})

	total := um.GetTotalCalls()
	if total != 2 {
		t.Errorf("expected 2 total calls, got %d", total)
	}
}

// --- Online / Offline tracking ---

func TestMarkUserOnline_And_IsUserOnline(t *testing.T) {
	um := newTestManager(t, []User{})

	if um.IsUserOnline(1) {
		t.Error("expected user 1 to be offline initially")
	}

	um.MarkUserOnline(1)
	if !um.IsUserOnline(1) {
		t.Error("expected user 1 to be online after MarkUserOnline")
	}

	um.MarkUserOffline(1)
	if um.IsUserOnline(1) {
		t.Error("expected user 1 to be offline after MarkUserOffline")
	}
}

func TestMarkUserOnline_MultipleUsers(t *testing.T) {
	um := newTestManager(t, []User{})
	um.MarkUserOnline(1)
	um.MarkUserOnline(2)

	if !um.IsUserOnline(1) || !um.IsUserOnline(2) {
		t.Error("expected both users to be online")
	}
	if um.IsUserOnline(3) {
		t.Error("expected user 3 to be offline")
	}
}

// --- LogAdminActivity ---

func TestLogAdminActivity_WritesFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "users.json"), []byte("[]"), 0644)

	um, _ := NewUserManager(tmpDir)

	entry := AdminActivityLog{
		AdminUsername: "sysop",
		AdminID:       1,
		TargetUserID:  2,
		TargetHandle:  "Bob",
		Action:        "EDIT_USER",
		FieldName:     "accessLevel",
		OldValue:      "10",
		NewValue:      "50",
	}

	if err := um.LogAdminActivity(entry); err != nil {
		t.Fatalf("LogAdminActivity: %v", err)
	}

	// Verify file was created
	logPath := filepath.Join(tmpDir, adminLogFile)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected admin log file: %v", err)
	}

	var logs []AdminActivityLog
	if err := json.Unmarshal(data, &logs); err != nil {
		t.Fatalf("unmarshal admin log: %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(logs))
	}
	if logs[0].Action != "EDIT_USER" {
		t.Errorf("expected action EDIT_USER, got %q", logs[0].Action)
	}
}

func TestLogAdminActivity_AppendsToExisting(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "users.json"), []byte("[]"), 0644)

	um, _ := NewUserManager(tmpDir)

	for i := 0; i < 3; i++ {
		um.LogAdminActivity(AdminActivityLog{AdminUsername: "sysop", Action: "TEST"})
	}

	logPath := filepath.Join(tmpDir, adminLogFile)
	data, _ := os.ReadFile(logPath)
	var logs []AdminActivityLog
	json.Unmarshal(data, &logs)

	if len(logs) != 3 {
		t.Errorf("expected 3 log entries, got %d", len(logs))
	}
}

// --- AdminActivityLogEntry helper ---

func TestAdminActivityLogEntry_Helper(t *testing.T) {
	entry := AdminActivityLogEntry("admin", 1, 2, "Bob", "accessLevel", "10", "50")

	if entry.AdminUsername != "admin" {
		t.Errorf("expected admin, got %q", entry.AdminUsername)
	}
	if entry.Action != "EDIT_USER" {
		t.Errorf("expected EDIT_USER, got %q", entry.Action)
	}
	if entry.FieldName != "accessLevel" {
		t.Errorf("expected accessLevel, got %q", entry.FieldName)
	}
	if entry.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

// --- StripUTF8BOM ---

func TestStripUTF8BOM_WithBOM(t *testing.T) {
	input := append([]byte{0xEF, 0xBB, 0xBF}, []byte("hello")...)
	result := StripUTF8BOM(input)
	if string(result) != "hello" {
		t.Errorf("expected 'hello', got %q", string(result))
	}
}

func TestStripUTF8BOM_WithoutBOM(t *testing.T) {
	input := []byte("hello")
	result := StripUTF8BOM(input)
	if string(result) != "hello" {
		t.Errorf("expected 'hello', got %q", string(result))
	}
}

func TestStripUTF8BOM_Empty(t *testing.T) {
	result := StripUTF8BOM([]byte{})
	if len(result) != 0 {
		t.Errorf("expected empty, got %d bytes", len(result))
	}
}

func TestStripUTF8BOM_ShortInput(t *testing.T) {
	result := StripUTF8BOM([]byte{0xEF, 0xBB})
	if len(result) != 2 {
		t.Errorf("expected 2 bytes (partial BOM not stripped), got %d", len(result))
	}
}

// --- Concurrent access ---

func TestConcurrentAddAndGet(t *testing.T) {
	um := newTestManager(t, []User{})

	var wg sync.WaitGroup
	errs := make(chan error, 20)

	// Add 10 users concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := strings.ToLower(string(rune('a'+idx))) + "user"
			handle := strings.ToUpper(string(rune('a'+idx))) + "Handle"
			_, err := um.AddUser(name, "pw", handle, "", "", "")
			if err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent AddUser error: %v", err)
	}

	if um.GetUserCount() != 10 {
		t.Errorf("expected 10 users after concurrent adds, got %d", um.GetUserCount())
	}
}

func TestConcurrentAuthenticate(t *testing.T) {
	pw := "secret"
	seed := []User{{ID: 1, Username: "alice", Handle: "Alice", PasswordHash: hashPassword(t, pw), AccessLevel: 5}}
	um := newTestManager(t, seed)

	failures := make(chan string, 5)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok := um.Authenticate("alice", pw)
			if !ok {
				failures <- "concurrent auth should succeed"
			}
		}()
	}
	wg.Wait()
	close(failures)
	for msg := range failures {
		t.Error(msg)
	}
}

// --- loadNextCallNumber ---

func TestLoadNextCallNumber_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "users.json"), []byte("[]"), 0644)

	callNum, _ := json.Marshal(uint64(100))
	os.WriteFile(filepath.Join(tmpDir, "callnumber.json"), callNum, 0644)

	um, _ := NewUserManager(tmpDir)
	// After loading, total calls = nextCallNumber - 1 = 99
	if um.GetTotalCalls() != 99 {
		t.Errorf("expected 99 total calls, got %d", um.GetTotalCalls())
	}
}

func TestLoadNextCallNumber_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "users.json"), []byte("[]"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "callnumber.json"), []byte(""), 0644)

	um, _ := NewUserManager(tmpDir)
	// Should default to 1 (0 total calls)
	if um.GetTotalCalls() != 0 {
		t.Errorf("expected 0 total calls for empty callnumber, got %d", um.GetTotalCalls())
	}
}

func TestLoadNextCallNumber_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "users.json"), []byte("[]"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "callnumber.json"), []byte("notanumber"), 0644)

	um, _ := NewUserManager(tmpDir)
	// Should default to 1 (0 total calls)
	if um.GetTotalCalls() != 0 {
		t.Errorf("expected 0 total calls for invalid callnumber, got %d", um.GetTotalCalls())
	}
}

// --- loadCallHistory ---

func TestLoadCallHistory_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "users.json"), []byte("[]"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "callhistory.json"), []byte(""), 0644)

	um, _ := NewUserManager(tmpDir)
	callers := um.GetLastCallers()
	if len(callers) != 0 {
		t.Errorf("expected 0 callers for empty history file, got %d", len(callers))
	}
}

func TestLoadCallHistory_ExistingRecords(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "users.json"), []byte("[]"), 0644)

	records := []CallRecord{
		{UserID: 1, Handle: "Alice", CallNumber: 1},
		{UserID: 2, Handle: "Bob", CallNumber: 2},
	}
	data, _ := json.Marshal(records)
	os.WriteFile(filepath.Join(tmpDir, "callhistory.json"), data, 0644)

	um, _ := NewUserManager(tmpDir)
	callers := um.GetLastCallers()
	if len(callers) != 2 {
		t.Errorf("expected 2 callers, got %d", len(callers))
	}
}

func TestLoadCallHistory_TruncatesOverLimit(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "users.json"), []byte("[]"), 0644)

	// Create more records than the limit
	records := make([]CallRecord, callHistoryLimit+10)
	for i := range records {
		records[i] = CallRecord{UserID: i, Handle: "User", CallNumber: uint64(i + 1)}
	}
	data, _ := json.Marshal(records)
	os.WriteFile(filepath.Join(tmpDir, "callhistory.json"), data, 0644)

	um, _ := NewUserManager(tmpDir)
	callers := um.GetLastCallers()
	if len(callers) != callHistoryLimit {
		t.Errorf("expected %d callers after truncation, got %d", callHistoryLimit, len(callers))
	}
}

// --- determineNextUserID ---

func TestDetermineNextUserID_GapInIDs(t *testing.T) {
	seed := []User{
		{ID: 1, Username: "a", Handle: "A"},
		{ID: 5, Username: "b", Handle: "B"},
		{ID: 3, Username: "c", Handle: "C"},
	}
	um := newTestManager(t, seed)

	// Max ID is 5, so next should be 6
	if um.NextUserID() != 6 {
		t.Errorf("expected next ID 6, got %d", um.NextUserID())
	}
}

// --- Edge cases ---

func TestAddUser_EmptyPassword(t *testing.T) {
	um := newTestManager(t, []User{})

	// bcrypt should handle empty password (it's valid)
	u, err := um.AddUser("emptypass", "", "EmptyPass", "", "", "")
	if err != nil {
		t.Fatalf("AddUser with empty password: %v", err)
	}

	// Verify the hash works for empty password
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("")); err != nil {
		t.Error("empty password hash should match empty string")
	}
}

func TestAuthenticate_EmptyPassword(t *testing.T) {
	seed := []User{{ID: 1, Username: "emptypass", Handle: "EP", PasswordHash: hashPassword(t, ""), AccessLevel: 1}}
	um := newTestManager(t, seed)

	_, ok := um.Authenticate("emptypass", "")
	if !ok {
		t.Error("expected auth with empty password to succeed when hash matches")
	}

	_, ok = um.Authenticate("emptypass", "notempty")
	if ok {
		t.Error("expected auth with wrong password to fail")
	}
}

func TestGetUserCount_AfterAddAndPurge(t *testing.T) {
	um := newTestManager(t, []User{})
	um.AddUser("a", "pw", "A", "", "", "")
	um.AddUser("b", "pw", "B", "", "", "")

	if um.GetUserCount() != 2 {
		t.Errorf("expected 2, got %d", um.GetUserCount())
	}
}
