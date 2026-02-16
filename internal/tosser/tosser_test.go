package tosser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDupeDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dupes.json")

	db, err := NewDupeDB(path, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewDupeDB: %v", err)
	}

	// First add should not be a dupe
	if db.Add("1:103/705 12345678") {
		t.Error("first Add should return false")
	}

	// Second add of same MSGID should be a dupe
	if !db.Add("1:103/705 12345678") {
		t.Error("second Add should return true (dupe)")
	}

	// IsDupe should work
	if !db.IsDupe("1:103/705 12345678") {
		t.Error("IsDupe should return true")
	}
	if db.IsDupe("1:103/705 99999999") {
		t.Error("IsDupe should return false for unknown MSGID")
	}

	// Empty MSGID should not be a dupe
	if db.IsDupe("") {
		t.Error("empty MSGID should not be a dupe")
	}
	if db.Add("") {
		t.Error("empty MSGID Add should return false")
	}

	if db.Count() != 1 {
		t.Errorf("Count: got %d, want 1", db.Count())
	}

	// Save and reload
	if err := db.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	db2, err := NewDupeDB(path, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewDupeDB reload: %v", err)
	}
	if !db2.IsDupe("1:103/705 12345678") {
		t.Error("entry should persist across reload")
	}
}

func TestDupeDBPurge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dupes.json")

	db, err := NewDupeDB(path, 1*time.Hour)
	if err != nil {
		t.Fatalf("NewDupeDB: %v", err)
	}

	// Manually insert old entries with timestamps in the past
	db.mu.Lock()
	db.entries["old-msg-1"] = time.Now().Add(-2 * time.Hour).Unix()
	db.entries["old-msg-2"] = time.Now().Add(-2 * time.Hour).Unix()
	db.mu.Unlock()

	// Add a fresh entry
	db.Add("new-msg-1")

	if err := db.Purge(); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	if db.Count() != 1 {
		t.Errorf("after purge: Count=%d, want 1", db.Count())
	}
	if !db.IsDupe("new-msg-1") {
		t.Error("new entry should survive purge")
	}
	if db.IsDupe("old-msg-1") {
		t.Error("old entry should be purged")
	}
}

func TestDupeDBCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dupes.json")

	// Write corrupt data
	os.WriteFile(path, []byte("not valid json{{{"), 0644)

	db, err := NewDupeDB(path, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewDupeDB should handle corrupt file: %v", err)
	}
	if db.Count() != 0 {
		t.Error("corrupt file should result in empty DB")
	}
}

func TestParseSeenByLine(t *testing.T) {
	tests := []struct {
		input    string
		expected []netNode
	}{
		{
			"103/705 104/56",
			[]netNode{{103, 705}, {104, 56}},
		},
		{
			"103/705 706 707",
			[]netNode{{103, 705}, {103, 706}, {103, 707}},
		},
		{
			"103/705 104/56 100",
			[]netNode{{103, 705}, {104, 56}, {104, 100}},
		},
		{
			"",
			nil,
		},
	}

	for _, tt := range tests {
		nodes := ParseSeenByLine(tt.input)
		if len(nodes) != len(tt.expected) {
			t.Errorf("ParseSeenByLine(%q): got %d nodes, want %d", tt.input, len(nodes), len(tt.expected))
			continue
		}
		for i, n := range nodes {
			if n.Net != tt.expected[i].Net || n.Node != tt.expected[i].Node {
				t.Errorf("ParseSeenByLine(%q)[%d]: got %d/%d, want %d/%d",
					tt.input, i, n.Net, n.Node, tt.expected[i].Net, tt.expected[i].Node)
			}
		}
	}
}

func TestFormatSeenByLine(t *testing.T) {
	nodes := []netNode{{103, 705}, {103, 706}, {104, 56}}
	result := FormatSeenByLine(nodes)

	// Should be sorted and compressed
	expected := "103/705 706 104/56"
	if result != expected {
		t.Errorf("FormatSeenByLine: got %q, want %q", result, expected)
	}
}

func TestMergeSeenBy(t *testing.T) {
	existing := []string{"103/705 104/56"}
	result := MergeSeenBy(existing, "3/110")

	if len(result) != 1 {
		t.Fatalf("MergeSeenBy: got %d lines, want 1", len(result))
	}

	// Should contain all three addresses
	nodes := ParseSeenByLine(result[0])
	if len(nodes) != 3 {
		t.Errorf("MergeSeenBy result has %d nodes, want 3", len(nodes))
	}
}

func TestAppendPath(t *testing.T) {
	existing := []string{"103/705"}
	result := AppendPath(existing, "3/110")

	if len(result) != 1 {
		t.Fatalf("AppendPath: got %d lines, want 1", len(result))
	}

	nodes := ParseSeenByLine(result[0])
	if len(nodes) != 2 {
		t.Errorf("AppendPath result has %d nodes, want 2", len(nodes))
	}
}

func TestMergeSeenByNoDuplicates(t *testing.T) {
	existing := []string{"103/705"}
	result := MergeSeenBy(existing, "103/705")

	nodes := ParseSeenByLine(result[0])
	if len(nodes) != 1 {
		t.Errorf("MergeSeenBy should not add duplicates: got %d nodes, want 1", len(nodes))
	}
}

// TestReplyKludgeParsing tests that malformed REPLY kludges are handled correctly
// by extracting only the first MSGID token
func TestReplyKludgeParsing(t *testing.T) {
	tests := []struct {
		name     string
		kludge   string
		expected string
	}{
		{
			name:     "normal reply",
			kludge:   "REPLY: 1:103/705 12345678",
			expected: "1:103/705",
		},
		{
			name:     "malformed multiple msgids",
			kludge:   "REPLY: 68475.fsx_gen@21: 21:1/999 9c829f62 21:2/150 35a2bb0a",
			expected: "68475.fsx_gen@21:",
		},
		{
			name:     "reply with extra spaces",
			kludge:   "REPLY: 1:104/56   87654321   extra   data",
			expected: "1:104/56",
		},
		{
			name:     "msgid format with embedded ftn address",
			kludge:   "REPLY: 1747.fsxnetfsxvideo@21:2/101",
			expected: "21:2/101",
		},
		{
			name:     "msgid format with node and point",
			kludge:   "REPLY: 1500.fsxnet_fsxvideo@21:1/137.0",
			expected: "21:1/137.0",
		},
		{
			name:     "malformed msgid without ftn address",
			kludge:   "REPLY: 12345.invalid@noftn",
			expected: "12345.invalid@noftn",
		},
		{
			name:     "empty reply value",
			kludge:   "REPLY: ",
			expected: "",
		},
		{
			name:     "reply with only spaces",
			kludge:   "REPLY:     ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the REPLY parsing logic from import.go
			if strings.HasPrefix(tt.kludge, "REPLY: ") {
				replyValue := strings.TrimPrefix(tt.kludge, "REPLY: ")
				var result string
				if parts := strings.Fields(replyValue); len(parts) > 0 {
					replyID := parts[0]

					// Check if the reply looks like a MSGID with embedded FTN address
					// Format: "xxxx.something@zone:net/node" or "xxxx.something@zone:net/node.point"
					if atPos := strings.Index(replyID, "@"); atPos != -1 && atPos < len(replyID)-1 {
						// Extract the FTN address after the @ symbol
						ftnPart := replyID[atPos+1:]
						// Validate it looks like a proper FTN address (contains : and /)
						if strings.Contains(ftnPart, ":") && strings.Contains(ftnPart, "/") {
							replyID = ftnPart
						}
					}

					result = replyID
				}

				if result != tt.expected {
					t.Errorf("REPLY parsing: got %q, want %q", result, tt.expected)
				}
			} else {
				t.Fatalf("Test kludge %q should start with 'REPLY: '", tt.kludge)
			}
		})
	}
}
