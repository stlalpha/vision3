package tosser

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

// DupeDB tracks seen MSGIDs to prevent duplicate message tossing.
// It persists to a JSON file on disk.
type DupeDB struct {
	mu      sync.Mutex
	path    string
	entries map[string]int64 // MSGID -> Unix timestamp when first seen
	maxAge  time.Duration    // How long to keep entries
}

// dupeFile is the on-disk representation.
type dupeFile struct {
	Entries map[string]int64 `json:"entries"`
}

// NewDupeDB creates or loads a duplicate database.
func NewDupeDB(path string, maxAge time.Duration) (*DupeDB, error) {
	db := &DupeDB{
		path:    path,
		entries: make(map[string]int64),
		maxAge:  maxAge,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return db, nil // Fresh database
		}
		return nil, err
	}

	if len(data) > 0 {
		var f dupeFile
		if err := json.Unmarshal(data, &f); err != nil {
			log.Printf("WARN: Corrupt dupe DB at %s, starting fresh: %v", path, err)
			return db, nil
		}
		if f.Entries != nil {
			db.entries = f.Entries
		}
	}

	return db, nil
}

// IsDupe returns true if the MSGID has been seen before.
func (db *DupeDB) IsDupe(msgID string) bool {
	if msgID == "" {
		return false // No MSGID = can't dupe-check
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	_, exists := db.entries[msgID]
	return exists
}

// Add records a MSGID as seen. Returns true if it was already a dupe.
func (db *DupeDB) Add(msgID string) bool {
	if msgID == "" {
		return false
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if _, exists := db.entries[msgID]; exists {
		return true
	}
	db.entries[msgID] = time.Now().Unix()
	return false
}

// Purge removes entries older than maxAge and saves to disk.
func (db *DupeDB) Purge() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	cutoff := time.Now().Add(-db.maxAge).Unix()
	for id, ts := range db.entries {
		if ts < cutoff {
			delete(db.entries, id)
		}
	}

	return db.saveLocked()
}

// Save persists the database to disk.
func (db *DupeDB) Save() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.saveLocked()
}

func (db *DupeDB) saveLocked() error {
	f := dupeFile{Entries: db.entries}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(db.path, data, 0644)
}

// Count returns the number of entries in the database.
func (db *DupeDB) Count() int {
	db.mu.Lock()
	defer db.mu.Unlock()
	return len(db.entries)
}
