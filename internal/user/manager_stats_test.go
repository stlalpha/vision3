package user

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestUserManager(t *testing.T) *UserMgr {
	t.Helper()
	tmpDir := t.TempDir()

	users := []User{
		{ID: 1, Username: "sysop", Handle: "SysOp", AccessLevel: 255},
		{ID: 2, Username: "user1", Handle: "User1", AccessLevel: 10},
		{ID: 3, Username: "user2", Handle: "User2", AccessLevel: 10},
	}
	data, _ := json.Marshal(users)
	os.WriteFile(filepath.Join(tmpDir, "users.json"), data, 0644)

	// Write callnumber.json so nextCallNumber loads
	callNum, _ := json.Marshal(uint64(42))
	os.WriteFile(filepath.Join(tmpDir, "callnumber.json"), callNum, 0644)

	um, err := NewUserManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create user manager: %v", err)
	}
	return um
}

func TestGetUserCount(t *testing.T) {
	um := setupTestUserManager(t)
	count := um.GetUserCount()
	if count != 3 {
		t.Errorf("expected 3 users, got %d", count)
	}
}

func TestGetTotalCalls(t *testing.T) {
	um := setupTestUserManager(t)
	calls := um.GetTotalCalls()
	// nextCallNumber is 42, so total calls = 42 - 1 = 41
	if calls != 41 {
		t.Errorf("expected 41 total calls, got %d", calls)
	}
}
