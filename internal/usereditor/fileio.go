package usereditor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/stlalpha/vision3/internal/user"
)

// LoadUsers reads users.json and returns the user slice plus the file's mtime.
func LoadUsers(path string) ([]*user.User, time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("stat %s: %w", path, err)
	}
	mtime := info.ModTime()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("read %s: %w", path, err)
	}

	var users []*user.User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, time.Time{}, fmt.Errorf("unmarshal %s: %w", path, err)
	}

	// Sort by ID for consistent display
	sort.Slice(users, func(i, j int) bool {
		return users[i].ID < users[j].ID
	})

	return users, mtime, nil
}

// CheckFileChanged compares the current mtime of the file against a stored mtime.
// Returns true if the file was modified externally.
func CheckFileChanged(path string, storedMtime time.Time) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false // If we can't stat, assume unchanged
	}
	return !info.ModTime().Equal(storedMtime)
}

// SaveUsers writes the user slice to disk atomically (temp file + rename).
// Returns the new file mtime after writing.
func SaveUsers(path string, users []*user.User) (time.Time, error) {
	// Sort by ID before saving for consistent output
	sorted := make([]*user.User, len(users))
	copy(sorted, users)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	data, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return time.Time{}, fmt.Errorf("marshal users: %w", err)
	}

	// Atomic write: write to temp file, then rename
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "users-*.json.tmp")
	if err != nil {
		return time.Time{}, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return time.Time{}, fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return time.Time{}, fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return time.Time{}, fmt.Errorf("rename temp to %s: %w", path, err)
	}

	// Get the new mtime
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, fmt.Errorf("stat after save: %w", err)
	}

	return info.ModTime(), nil
}

// CloneUser creates a deep copy of a user record.
func CloneUser(u *user.User) *user.User {
	if u == nil {
		return nil
	}
	c := *u
	// Deep copy map fields
	if u.LastReadMessageIDs != nil {
		c.LastReadMessageIDs = make(map[int]string, len(u.LastReadMessageIDs))
		for k, v := range u.LastReadMessageIDs {
			c.LastReadMessageIDs[k] = v
		}
	}
	// Deep copy slice fields (uuid.UUID is [16]byte value type, simple copy works)
	if u.TaggedFileIDs != nil {
		c.TaggedFileIDs = make([]uuid.UUID, len(u.TaggedFileIDs))
		copy(c.TaggedFileIDs, u.TaggedFileIDs)
	}
	if u.TaggedMessageAreaTags != nil {
		c.TaggedMessageAreaTags = make([]string, len(u.TaggedMessageAreaTags))
		copy(c.TaggedMessageAreaTags, u.TaggedMessageAreaTags)
	}
	if u.DeletedAt != nil {
		t := *u.DeletedAt
		c.DeletedAt = &t
	}
	return &c
}
