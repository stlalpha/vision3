package menu

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadBBSListDataEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	bld, err := loadBBSListData(filepath.Join(tmpDir, "configs"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bld.Listings) != 0 {
		t.Errorf("expected 0 listings, got %d", len(bld.Listings))
	}
	if bld.NextID != 1 {
		t.Errorf("expected NextID=1, got %d", bld.NextID)
	}
}

func TestSaveAndLoadBBSListData(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(tmpDir, "configs")

	now := time.Now().Truncate(time.Second)
	bld := &bbsListData{
		NextID: 3,
		Listings: []BBSListing{
			{
				ID:          1,
				Name:        "Test BBS",
				Sysop:       "TestSysOp",
				Address:     "test.bbs.com",
				TelnetPort:  "23",
				SSHPort:     "2222",
				Web:         "https://test.bbs.com",
				Software:    "ViSiON/3",
				Description: "A test BBS",
				AddedBy:     "user1",
				AddedDate:   now,
				Verified:    false,
			},
			{
				ID:         2,
				Name:       "Another BBS",
				Sysop:      "OtherOp",
				Address:    "other.bbs.com",
				TelnetPort: "2323",
				Software:   "Mystic",
				AddedBy:    "user2",
				AddedDate:  now,
				Verified:   true,
			},
		},
	}

	if err := saveBBSListData(configPath, bld); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	fp := bbsListFilePath(configPath)
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		t.Fatal("bbslist.json was not created")
	}

	loaded, err := loadBBSListData(configPath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.NextID != 3 {
		t.Errorf("NextID: got %d, want 3", loaded.NextID)
	}
	if len(loaded.Listings) != 2 {
		t.Fatalf("listings count: got %d, want 2", len(loaded.Listings))
	}

	e := loaded.Listings[0]
	if e.Name != "Test BBS" {
		t.Errorf("Name: got %q, want %q", e.Name, "Test BBS")
	}
	if e.Address != "test.bbs.com" {
		t.Errorf("Address: got %q, want %q", e.Address, "test.bbs.com")
	}
	if e.TelnetPort != "23" {
		t.Errorf("TelnetPort: got %q, want %q", e.TelnetPort, "23")
	}
	if e.SSHPort != "2222" {
		t.Errorf("SSHPort: got %q, want %q", e.SSHPort, "2222")
	}
	if e.Web != "https://test.bbs.com" {
		t.Errorf("Web: got %q, want %q", e.Web, "https://test.bbs.com")
	}
	if e.Verified {
		t.Error("first entry should not be verified")
	}

	e2 := loaded.Listings[1]
	if !e2.Verified {
		t.Error("second entry should be verified")
	}
	if e2.SSHPort != "" {
		t.Errorf("second entry SSHPort should be empty, got %q", e2.SSHPort)
	}
}

func TestBBSListDataJSON(t *testing.T) {
	bld := &bbsListData{
		NextID: 2,
		Listings: []BBSListing{
			{
				ID:         1,
				Name:       "JSON Test",
				Address:    "json.bbs.com",
				TelnetPort: "23",
				Software:   "TestWare",
			},
		},
	}

	data, err := json.MarshalIndent(bld, "", "    ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var loaded bbsListData
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if loaded.NextID != 2 {
		t.Errorf("NextID: got %d, want 2", loaded.NextID)
	}
	if len(loaded.Listings) != 1 {
		t.Fatalf("listings: got %d, want 1", len(loaded.Listings))
	}
	if loaded.Listings[0].Address != "json.bbs.com" {
		t.Errorf("Address: got %q, want %q", loaded.Listings[0].Address, "json.bbs.com")
	}
	if loaded.Listings[0].TelnetPort != "23" {
		t.Errorf("TelnetPort: got %q, want %q", loaded.Listings[0].TelnetPort, "23")
	}
}

func TestBBSListNextIDStartsAt1(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}

	// With no listings, NextID should default to 1.
	badData := []byte(`{"listings":[],"next_id":0}`)
	fp := filepath.Join(dataDir, "bbslist.json")
	if err := os.WriteFile(fp, badData, 0644); err != nil {
		t.Fatal(err)
	}

	bld, err := loadBBSListData(filepath.Join(tmpDir, "configs"))
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if bld.NextID != 1 {
		t.Errorf("NextID should default to 1, got %d", bld.NextID)
	}
}

func TestBBSListNextIDFromMaxExisting(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}

	// With existing listings and next_id=0, NextID should be max(ID)+1.
	data := []byte(`{"listings":[{"id":5,"name":"A"},{"id":3,"name":"B"},{"id":10,"name":"C"}],"next_id":0}`)
	fp := filepath.Join(dataDir, "bbslist.json")
	if err := os.WriteFile(fp, data, 0644); err != nil {
		t.Fatal(err)
	}

	bld, err := loadBBSListData(filepath.Join(tmpDir, "configs"))
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if bld.NextID != 11 {
		t.Errorf("NextID should be 11 (max existing ID 10 + 1), got %d", bld.NextID)
	}
}

func TestBBSListDeleteCompacts(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(tmpDir, "configs")

	bld := &bbsListData{
		NextID: 4,
		Listings: []BBSListing{
			{ID: 1, Name: "First", AddedBy: "user1"},
			{ID: 2, Name: "Second", AddedBy: "user2"},
			{ID: 3, Name: "Third", AddedBy: "user3"},
		},
	}

	if err := saveBBSListData(configPath, bld); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Delete the middle entry (index 1) — same operation as runBBSListDelete
	idx := 1
	bld.Listings = append(bld.Listings[:idx], bld.Listings[idx+1:]...)

	if err := saveBBSListData(configPath, bld); err != nil {
		t.Fatalf("save after delete failed: %v", err)
	}

	// Reload and verify compaction
	loaded, err := loadBBSListData(configPath)
	if err != nil {
		t.Fatalf("load after delete failed: %v", err)
	}

	if len(loaded.Listings) != 2 {
		t.Fatalf("expected 2 listings after delete, got %d", len(loaded.Listings))
	}
	if loaded.Listings[0].Name != "First" {
		t.Errorf("listings[0]: got %q, want %q", loaded.Listings[0].Name, "First")
	}
	if loaded.Listings[0].ID != 1 {
		t.Errorf("listings[0] ID: got %d, want 1", loaded.Listings[0].ID)
	}
	if loaded.Listings[1].Name != "Third" {
		t.Errorf("listings[1]: got %q, want %q", loaded.Listings[1].Name, "Third")
	}
	if loaded.Listings[1].ID != 3 {
		t.Errorf("listings[1] ID: got %d, want 3", loaded.Listings[1].ID)
	}
	if loaded.NextID != 4 {
		t.Errorf("NextID should be preserved as 4, got %d", loaded.NextID)
	}
}

func TestBBSListSanitize(t *testing.T) {
	tests := []struct {
		input, expect string
	}{
		{"Normal text", "Normal text"},
		{"|15colored|07 text", "\xc2\xa615colored\xc2\xa607 text"},
		{"no pipes here", "no pipes here"},
		{"", ""},
		{"one|pipe", "one\xc2\xa6pipe"},
	}
	for _, tt := range tests {
		got := bbsListSanitize(tt.input)
		if got != tt.expect {
			t.Errorf("bbsListSanitize(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestBBSListConnectionSummary(t *testing.T) {
	tests := []struct {
		name   string
		entry  BBSListing
		expect string
	}{
		{"both ports", BBSListing{TelnetPort: "23", SSHPort: "22", Web: "https://bbs.com"}, "T:23 S:22 Web"},
		{"telnet only", BBSListing{TelnetPort: "23"}, "T:23"},
		{"ssh only", BBSListing{SSHPort: "2222"}, "S:2222"},
		{"web only", BBSListing{Web: "https://bbs.com"}, "Web"},
		{"telnet and ssh", BBSListing{TelnetPort: "23", SSHPort: "22"}, "T:23 S:22"},
		{"empty", BBSListing{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bbsListConnectionSummary(&tt.entry)
			if got != tt.expect {
				t.Errorf("got %q, want %q", got, tt.expect)
			}
		})
	}
}
