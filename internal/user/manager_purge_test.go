package user

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupPurgeTestManager creates a UserMgr seeded with a mix of active, recently
// deleted, and long-ago deleted users for purge tests.
func setupPurgeTestManager(t *testing.T) (*UserMgr, time.Time) {
	t.Helper()
	tmpDir := t.TempDir()

	longAgo := time.Now().AddDate(0, 0, -60)   // 60 days ago — past any reasonable retention
	recently := time.Now().AddDate(0, 0, -5)   // 5 days ago — within a 30-day retention window

	users := []User{
		{ID: 1, Username: "sysop", Handle: "SysOp", AccessLevel: 255},
		{ID: 2, Username: "active1", Handle: "Active1", AccessLevel: 10},
		{ID: 3, Username: "olddeleted", Handle: "OldDeleted", AccessLevel: 10,
			DeletedUser: true, DeletedAt: &longAgo},
		{ID: 4, Username: "newdeleted", Handle: "NewDeleted", AccessLevel: 10,
			DeletedUser: true, DeletedAt: &recently},
		{ID: 5, Username: "notimestamp", Handle: "NoTimestamp", AccessLevel: 10,
			DeletedUser: true}, // DeletedAt == nil
	}

	data, _ := json.Marshal(users)
	os.WriteFile(filepath.Join(tmpDir, "users.json"), data, 0644)

	um, err := NewUserManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create user manager: %v", err)
	}
	return um, longAgo
}

// TestPurgeDeletedUsers_Disabled verifies that retentionDays=-1 is a no-op.
func TestPurgeDeletedUsers_Disabled(t *testing.T) {
	um, _ := setupPurgeTestManager(t)

	purged, err := um.PurgeDeletedUsers(-1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(purged) != 0 {
		t.Errorf("expected no purges with retentionDays=-1, got %d", len(purged))
	}
	if um.GetUserCount() != 5 {
		t.Errorf("expected 5 users after no-op purge, got %d", um.GetUserCount())
	}
}

// TestPurgeDeletedUsers_RetentionWindow verifies that only users past the
// retention window are purged, and users deleted recently are kept.
func TestPurgeDeletedUsers_RetentionWindow(t *testing.T) {
	um, _ := setupPurgeTestManager(t)

	// 30-day retention: olddeleted (60d) and notimestamp (nil) are eligible;
	// newdeleted (5d) and active users are not.
	purged, err := um.PurgeDeletedUsers(30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(purged) != 2 {
		t.Errorf("expected 2 purged users, got %d", len(purged))
	}

	// Verify purged set contains the right usernames
	purgedNames := make(map[string]bool)
	for _, p := range purged {
		purgedNames[p.Username] = true
	}
	if !purgedNames["olddeleted"] {
		t.Error("expected 'olddeleted' to be purged")
	}
	if !purgedNames["notimestamp"] {
		t.Error("expected 'notimestamp' (nil DeletedAt) to be purged")
	}
	if purgedNames["newdeleted"] {
		t.Error("did not expect 'newdeleted' (5 days ago) to be purged with 30-day retention")
	}
	if purgedNames["sysop"] || purgedNames["active1"] {
		t.Error("active users must not be purged")
	}
}

// TestPurgeDeletedUsers_ZeroRetention verifies that retentionDays=0 purges all
// soft-deleted users immediately, regardless of DeletedAt.
func TestPurgeDeletedUsers_ZeroRetention(t *testing.T) {
	um, _ := setupPurgeTestManager(t)

	purged, err := um.PurgeDeletedUsers(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All 3 deleted users (olddeleted, newdeleted, notimestamp) should be gone
	if len(purged) != 3 {
		t.Errorf("expected 3 purged users with retentionDays=0, got %d", len(purged))
	}
	if um.GetUserCount() != 2 {
		t.Errorf("expected 2 remaining users after purging all deleted, got %d", um.GetUserCount())
	}
}

// TestPurgeDeletedUsers_NoneEligible verifies that a very long retention period
// only purges users with no DeletedAt timestamp (nil = unknown, treat as eligible).
// Users with a known recent deletion timestamp are kept.
func TestPurgeDeletedUsers_NoneEligible(t *testing.T) {
	um, _ := setupPurgeTestManager(t)

	// 365-day retention: olddeleted (60d) and newdeleted (5d) are both within window.
	// notimestamp has no DeletedAt — treated as immediately eligible regardless of retention.
	purged, err := um.PurgeDeletedUsers(365)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(purged) != 1 {
		t.Errorf("expected 1 purged user (notimestamp) with 365-day retention, got %d", len(purged))
	}
	if purged[0].Username != "notimestamp" {
		t.Errorf("expected 'notimestamp' to be purged, got %q", purged[0].Username)
	}
	if um.GetUserCount() != 4 {
		t.Errorf("expected 4 remaining users after purging notimestamp, got %d", um.GetUserCount())
	}
}

// TestPurgeDeletedUsers_Persistence verifies that purged users are removed from
// the on-disk users.json, not just from memory.
func TestPurgeDeletedUsers_Persistence(t *testing.T) {
	um, _ := setupPurgeTestManager(t)
	dataPath := filepath.Dir(um.path)

	_, err := um.PurgeDeletedUsers(30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Re-load from disk into a fresh manager
	um2, err := NewUserManager(dataPath)
	if err != nil {
		t.Fatalf("failed to reload user manager: %v", err)
	}

	// newdeleted (5 days) should still be present
	if _, ok := um2.GetUser("newdeleted"); !ok {
		t.Error("expected 'newdeleted' to survive purge and be present after reload")
	}
	// olddeleted should be gone
	if _, ok := um2.GetUser("olddeleted"); ok {
		t.Error("expected 'olddeleted' to be absent after reload")
	}
	// notimestamp should be gone
	if _, ok := um2.GetUser("notimestamp"); ok {
		t.Error("expected 'notimestamp' to be absent after reload")
	}
}

// TestPurgeDeletedUsers_PurgeResultFields verifies PurgeResult carries correct metadata.
func TestPurgeDeletedUsers_PurgeResultFields(t *testing.T) {
	um, deletedAt := setupPurgeTestManager(t)

	purged, err := um.PurgeDeletedUsers(30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, p := range purged {
		if p.Username == "olddeleted" {
			if p.ID != 3 {
				t.Errorf("expected ID=3 for olddeleted, got %d", p.ID)
			}
			if p.Handle != "OldDeleted" {
				t.Errorf("expected Handle=OldDeleted, got %q", p.Handle)
			}
			// Timestamps should be within a second of each other
			diff := p.DeletedAt.Sub(deletedAt)
			if diff < -time.Second || diff > time.Second {
				t.Errorf("unexpected DeletedAt for olddeleted: %v (expected ~%v)", p.DeletedAt, deletedAt)
			}
		}
		if p.Username == "notimestamp" {
			if !p.DeletedAt.IsZero() {
				t.Errorf("expected zero DeletedAt for notimestamp, got %v", p.DeletedAt)
			}
		}
	}
}
