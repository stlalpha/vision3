package file

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestNewFileManager_EmptyDirectory verifies that when NewFileManager is called
// with an empty directory (no file_areas.json), it results in 0 areas loaded
// and creates an empty config file.
func TestNewFileManager_EmptyDirectory(t *testing.T) {
	temporaryDataDirectory := t.TempDir()
	temporaryConfigDirectory := t.TempDir()

	fileManager, err := NewFileManager(temporaryDataDirectory, temporaryConfigDirectory)
	if err != nil {
		t.Fatalf("NewFileManager failed: %v", err)
	}

	// Verify no areas were loaded
	loadedAreas := fileManager.ListAreas()
	if len(loadedAreas) != 0 {
		t.Errorf("Expected 0 areas when starting with empty directory, got %d", len(loadedAreas))
	}

	// Verify the empty config file was created
	configFilePath := filepath.Join(temporaryConfigDirectory, "file_areas.json")
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		t.Error("Expected file_areas.json to be created in empty directory")
	}
}

// TestNewFileManager_LoadsAreas verifies that NewFileManager correctly loads
// file areas from a valid file_areas.json configuration file.
func TestNewFileManager_LoadsAreas(t *testing.T) {
	temporaryDataDirectory := t.TempDir()
	temporaryConfigDirectory := t.TempDir()

	// Create a valid file_areas.json with test areas
	testFileAreas := []FileArea{
		{
			ID:          1,
			Tag:         "UTILS",
			Name:        "Utility Programs",
			Description: "General utility software",
			Path:        "utils",
			ACSList:     "",
			ACSUpload:   "S100",
			ACSDownload: "",
		},
		{
			ID:          2,
			Tag:         "TEXTS",
			Name:        "Text Files",
			Description: "Documentation and text files",
			Path:        "texts",
			ACSList:     "",
			ACSUpload:   "S100",
			ACSDownload: "",
		},
	}

	configFileAreasJSON, err := json.MarshalIndent(testFileAreas, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test file areas: %v", err)
	}

	configFilePath := filepath.Join(temporaryConfigDirectory, "file_areas.json")
	if err := os.WriteFile(configFilePath, configFileAreasJSON, 0644); err != nil {
		t.Fatalf("Failed to write test file_areas.json: %v", err)
	}

	fileManager, err := NewFileManager(temporaryDataDirectory, temporaryConfigDirectory)
	if err != nil {
		t.Fatalf("NewFileManager failed: %v", err)
	}

	// Verify both areas were loaded
	loadedAreas := fileManager.ListAreas()
	if len(loadedAreas) != 2 {
		t.Fatalf("Expected 2 areas to be loaded, got %d", len(loadedAreas))
	}

	// Verify areas are sorted by ID
	if loadedAreas[0].ID != 1 {
		t.Errorf("Expected first area ID to be 1, got %d", loadedAreas[0].ID)
	}
	if loadedAreas[1].ID != 2 {
		t.Errorf("Expected second area ID to be 2, got %d", loadedAreas[1].ID)
	}

	// Verify area properties were loaded correctly
	if loadedAreas[0].Tag != "UTILS" {
		t.Errorf("Expected first area Tag to be 'UTILS', got '%s'", loadedAreas[0].Tag)
	}
	if loadedAreas[0].Name != "Utility Programs" {
		t.Errorf("Expected first area Name to be 'Utility Programs', got '%s'", loadedAreas[0].Name)
	}
}

// TestGetAreaByID_ReturnsCorrectArea verifies that GetAreaByID retrieves
// the correct area when given a valid area ID.
func TestGetAreaByID_ReturnsCorrectArea(t *testing.T) {
	temporaryDataDirectory := t.TempDir()
	temporaryConfigDirectory := t.TempDir()

	testFileAreas := []FileArea{
		{
			ID:          1,
			Tag:         "UTILS",
			Name:        "Utility Programs",
			Description: "General utility software",
			Path:        "utils",
		},
		{
			ID:          2,
			Tag:         "TEXTS",
			Name:        "Text Files",
			Description: "Documentation and text files",
			Path:        "texts",
		},
	}

	configFileAreasJSON, _ := json.MarshalIndent(testFileAreas, "", "  ")
	configFilePath := filepath.Join(temporaryConfigDirectory, "file_areas.json")
	os.WriteFile(configFilePath, configFileAreasJSON, 0644)

	fileManager, err := NewFileManager(temporaryDataDirectory, temporaryConfigDirectory)
	if err != nil {
		t.Fatalf("NewFileManager failed: %v", err)
	}

	// Retrieve area by ID
	retrievedArea, areaExists := fileManager.GetAreaByID(2)
	if !areaExists {
		t.Fatal("Expected area with ID 2 to exist, but GetAreaByID returned false")
	}

	if retrievedArea.ID != 2 {
		t.Errorf("Expected retrieved area ID to be 2, got %d", retrievedArea.ID)
	}

	if retrievedArea.Tag != "TEXTS" {
		t.Errorf("Expected retrieved area Tag to be 'TEXTS', got '%s'", retrievedArea.Tag)
	}

	if retrievedArea.Name != "Text Files" {
		t.Errorf("Expected retrieved area Name to be 'Text Files', got '%s'", retrievedArea.Name)
	}
}

// TestGetAreaByID_NotFound verifies that GetAreaByID returns false
// when querying for a non-existent area ID.
func TestGetAreaByID_NotFound(t *testing.T) {
	temporaryDataDirectory := t.TempDir()
	temporaryConfigDirectory := t.TempDir()

	testFileAreas := []FileArea{
		{
			ID:   1,
			Tag:  "UTILS",
			Name: "Utility Programs",
			Path: "utils",
		},
	}

	configFileAreasJSON, _ := json.MarshalIndent(testFileAreas, "", "  ")
	configFilePath := filepath.Join(temporaryConfigDirectory, "file_areas.json")
	os.WriteFile(configFilePath, configFileAreasJSON, 0644)

	fileManager, err := NewFileManager(temporaryDataDirectory, temporaryConfigDirectory)
	if err != nil {
		t.Fatalf("NewFileManager failed: %v", err)
	}

	_, areaExists := fileManager.GetAreaByID(999)
	if areaExists {
		t.Error("Expected GetAreaByID to return false for non-existent area ID 999")
	}
}

// TestGetAreaByTag_ReturnsCorrectArea verifies that GetAreaByTag retrieves
// the correct area when given a valid tag.
func TestGetAreaByTag_ReturnsCorrectArea(t *testing.T) {
	temporaryDataDirectory := t.TempDir()
	temporaryConfigDirectory := t.TempDir()

	testFileAreas := []FileArea{
		{
			ID:   1,
			Tag:  "UTILS",
			Name: "Utility Programs",
			Path: "utils",
		},
		{
			ID:   2,
			Tag:  "TEXTS",
			Name: "Text Files",
			Path: "texts",
		},
	}

	configFileAreasJSON, _ := json.MarshalIndent(testFileAreas, "", "  ")
	configFilePath := filepath.Join(temporaryConfigDirectory, "file_areas.json")
	os.WriteFile(configFilePath, configFileAreasJSON, 0644)

	fileManager, err := NewFileManager(temporaryDataDirectory, temporaryConfigDirectory)
	if err != nil {
		t.Fatalf("NewFileManager failed: %v", err)
	}

	// Retrieve area by tag
	retrievedArea, areaExists := fileManager.GetAreaByTag("TEXTS")
	if !areaExists {
		t.Fatal("Expected area with tag 'TEXTS' to exist, but GetAreaByTag returned false")
	}

	if retrievedArea.ID != 2 {
		t.Errorf("Expected retrieved area ID to be 2, got %d", retrievedArea.ID)
	}

	if retrievedArea.Tag != "TEXTS" {
		t.Errorf("Expected retrieved area Tag to be 'TEXTS', got '%s'", retrievedArea.Tag)
	}
}

// TestGetAreaByTag_CaseInsensitive verifies that GetAreaByTag performs
// case-insensitive tag lookup.
func TestGetAreaByTag_CaseInsensitive(t *testing.T) {
	temporaryDataDirectory := t.TempDir()
	temporaryConfigDirectory := t.TempDir()

	testFileAreas := []FileArea{
		{
			ID:   1,
			Tag:  "UTILS",
			Name: "Utility Programs",
			Path: "utils",
		},
	}

	configFileAreasJSON, _ := json.MarshalIndent(testFileAreas, "", "  ")
	configFilePath := filepath.Join(temporaryConfigDirectory, "file_areas.json")
	os.WriteFile(configFilePath, configFileAreasJSON, 0644)

	fileManager, err := NewFileManager(temporaryDataDirectory, temporaryConfigDirectory)
	if err != nil {
		t.Fatalf("NewFileManager failed: %v", err)
	}

	// Try lowercase tag lookup
	retrievedArea, areaExists := fileManager.GetAreaByTag("utils")
	if !areaExists {
		t.Fatal("Expected GetAreaByTag to be case-insensitive, but lowercase 'utils' was not found")
	}

	if retrievedArea.Tag != "UTILS" {
		t.Errorf("Expected retrieved area Tag to be 'UTILS', got '%s'", retrievedArea.Tag)
	}

	// Try mixed case tag lookup
	retrievedAreaMixed, mixedExists := fileManager.GetAreaByTag("Utils")
	if !mixedExists {
		t.Fatal("Expected GetAreaByTag to be case-insensitive, but mixed case 'Utils' was not found")
	}

	if retrievedAreaMixed.ID != 1 {
		t.Errorf("Expected mixed case lookup to return area ID 1, got %d", retrievedAreaMixed.ID)
	}
}

// TestGetAreaByTag_NotFound verifies that GetAreaByTag returns false
// when querying for a non-existent tag.
func TestGetAreaByTag_NotFound(t *testing.T) {
	temporaryDataDirectory := t.TempDir()
	temporaryConfigDirectory := t.TempDir()

	testFileAreas := []FileArea{
		{
			ID:   1,
			Tag:  "UTILS",
			Name: "Utility Programs",
			Path: "utils",
		},
	}

	configFileAreasJSON, _ := json.MarshalIndent(testFileAreas, "", "  ")
	configFilePath := filepath.Join(temporaryConfigDirectory, "file_areas.json")
	os.WriteFile(configFilePath, configFileAreasJSON, 0644)

	fileManager, err := NewFileManager(temporaryDataDirectory, temporaryConfigDirectory)
	if err != nil {
		t.Fatalf("NewFileManager failed: %v", err)
	}

	_, areaExists := fileManager.GetAreaByTag("NONEXISTENT")
	if areaExists {
		t.Error("Expected GetAreaByTag to return false for non-existent tag 'NONEXISTENT'")
	}
}

// TestNewFileManager_SkipsInvalidAreas verifies that NewFileManager skips
// areas with invalid IDs (zero or negative) or empty tags.
func TestNewFileManager_SkipsInvalidAreas(t *testing.T) {
	temporaryDataDirectory := t.TempDir()
	temporaryConfigDirectory := t.TempDir()

	// Create config with some invalid areas mixed in
	testFileAreas := []FileArea{
		{
			ID:   0, // Invalid - ID must be > 0
			Tag:  "INVALID1",
			Name: "Should be skipped",
			Path: "invalid1",
		},
		{
			ID:   1,
			Tag:  "VALID",
			Name: "Valid Area",
			Path: "valid",
		},
		{
			ID:   -1, // Invalid - negative ID
			Tag:  "INVALID2",
			Name: "Should be skipped",
			Path: "invalid2",
		},
		{
			ID:   2,
			Tag:  "", // Invalid - empty tag
			Name: "Should be skipped",
			Path: "invalid3",
		},
	}

	configFileAreasJSON, _ := json.MarshalIndent(testFileAreas, "", "  ")
	configFilePath := filepath.Join(temporaryConfigDirectory, "file_areas.json")
	os.WriteFile(configFilePath, configFileAreasJSON, 0644)

	fileManager, err := NewFileManager(temporaryDataDirectory, temporaryConfigDirectory)
	if err != nil {
		t.Fatalf("NewFileManager failed: %v", err)
	}

	// Only the valid area should be loaded
	loadedAreas := fileManager.ListAreas()
	if len(loadedAreas) != 1 {
		t.Fatalf("Expected only 1 valid area to be loaded, got %d", len(loadedAreas))
	}

	if loadedAreas[0].Tag != "VALID" {
		t.Errorf("Expected the only loaded area to be 'VALID', got '%s'", loadedAreas[0].Tag)
	}
}

// TestGetFilesForArea_EmptyArea verifies that GetFilesForArea returns an
// empty slice for an area that exists but has no files.
func TestGetFilesForArea_EmptyArea(t *testing.T) {
	temporaryDataDirectory := t.TempDir()
	temporaryConfigDirectory := t.TempDir()

	testFileAreas := []FileArea{
		{
			ID:   1,
			Tag:  "EMPTY",
			Name: "Empty Area",
			Path: "empty",
		},
	}

	configFileAreasJSON, _ := json.MarshalIndent(testFileAreas, "", "  ")
	configFilePath := filepath.Join(temporaryConfigDirectory, "file_areas.json")
	os.WriteFile(configFilePath, configFileAreasJSON, 0644)

	fileManager, err := NewFileManager(temporaryDataDirectory, temporaryConfigDirectory)
	if err != nil {
		t.Fatalf("NewFileManager failed: %v", err)
	}

	filesInArea := fileManager.GetFilesForArea(1)
	if filesInArea == nil {
		t.Fatal("Expected GetFilesForArea to return an empty slice, not nil")
	}

	if len(filesInArea) != 0 {
		t.Errorf("Expected 0 files in empty area, got %d", len(filesInArea))
	}
}

// TestGetFileCountForArea_EmptyArea verifies that GetFileCountForArea returns 0
// for an area that exists but has no files.
func TestGetFileCountForArea_EmptyArea(t *testing.T) {
	temporaryDataDirectory := t.TempDir()
	temporaryConfigDirectory := t.TempDir()

	testFileAreas := []FileArea{
		{
			ID:   1,
			Tag:  "EMPTY",
			Name: "Empty Area",
			Path: "empty",
		},
	}

	configFileAreasJSON, _ := json.MarshalIndent(testFileAreas, "", "  ")
	configFilePath := filepath.Join(temporaryConfigDirectory, "file_areas.json")
	os.WriteFile(configFilePath, configFileAreasJSON, 0644)

	fileManager, err := NewFileManager(temporaryDataDirectory, temporaryConfigDirectory)
	if err != nil {
		t.Fatalf("NewFileManager failed: %v", err)
	}

	fileCount, err := fileManager.GetFileCountForArea(1)
	if err != nil {
		t.Fatalf("GetFileCountForArea failed: %v", err)
	}

	if fileCount != 0 {
		t.Errorf("Expected file count to be 0 for empty area, got %d", fileCount)
	}
}

// TestGetFileCountForArea_NonExistentArea verifies that GetFileCountForArea
// returns 0 for a non-existent area ID.
func TestGetFileCountForArea_NonExistentArea(t *testing.T) {
	temporaryDataDirectory := t.TempDir()
	temporaryConfigDirectory := t.TempDir()

	fileManager, err := NewFileManager(temporaryDataDirectory, temporaryConfigDirectory)
	if err != nil {
		t.Fatalf("NewFileManager failed: %v", err)
	}

	fileCount, err := fileManager.GetFileCountForArea(999)
	if err != nil {
		t.Fatalf("GetFileCountForArea failed: %v", err)
	}

	if fileCount != 0 {
		t.Errorf("Expected file count to be 0 for non-existent area, got %d", fileCount)
	}
}

// TestListAreas_ReturnsSortedByID verifies that ListAreas returns areas
// sorted by ID in ascending order.
func TestListAreas_ReturnsSortedByID(t *testing.T) {
	temporaryDataDirectory := t.TempDir()
	temporaryConfigDirectory := t.TempDir()

	// Create areas with non-sequential IDs to test sorting
	testFileAreas := []FileArea{
		{
			ID:   5,
			Tag:  "FIFTH",
			Name: "Fifth Area",
			Path: "fifth",
		},
		{
			ID:   2,
			Tag:  "SECOND",
			Name: "Second Area",
			Path: "second",
		},
		{
			ID:   8,
			Tag:  "EIGHTH",
			Name: "Eighth Area",
			Path: "eighth",
		},
		{
			ID:   1,
			Tag:  "FIRST",
			Name: "First Area",
			Path: "first",
		},
	}

	configFileAreasJSON, _ := json.MarshalIndent(testFileAreas, "", "  ")
	configFilePath := filepath.Join(temporaryConfigDirectory, "file_areas.json")
	os.WriteFile(configFilePath, configFileAreasJSON, 0644)

	fileManager, err := NewFileManager(temporaryDataDirectory, temporaryConfigDirectory)
	if err != nil {
		t.Fatalf("NewFileManager failed: %v", err)
	}

	loadedAreas := fileManager.ListAreas()
	if len(loadedAreas) != 4 {
		t.Fatalf("Expected 4 areas, got %d", len(loadedAreas))
	}

	// Verify sorted order: 1, 2, 5, 8
	expectedIDs := []int{1, 2, 5, 8}
	for i, expectedID := range expectedIDs {
		if loadedAreas[i].ID != expectedID {
			t.Errorf("Expected area at index %d to have ID %d, got %d", i, expectedID, loadedAreas[i].ID)
		}
	}
}

// TestIsSupportedArchive_ZipFiles verifies that IsSupportedArchive correctly
// identifies .zip files as supported archives.
func TestIsSupportedArchive_ZipFiles(t *testing.T) {
	temporaryDataDirectory := t.TempDir()
	temporaryConfigDirectory := t.TempDir()

	fileManager, err := NewFileManager(temporaryDataDirectory, temporaryConfigDirectory)
	if err != nil {
		t.Fatalf("NewFileManager failed: %v", err)
	}

	testCases := []struct {
		filename        string
		expectedSupport bool
	}{
		{"file.zip", true},
		{"FILE.ZIP", true},
		{"archive.Zip", true},
		{"document.txt", false},
		{"image.png", false},
		{"archive.tar.gz", false},
		{"archive.rar", false},
		{"noextension", false},
		{"", false},
	}

	for _, testCase := range testCases {
		isSupported := fileManager.IsSupportedArchive(testCase.filename)
		if isSupported != testCase.expectedSupport {
			t.Errorf("IsSupportedArchive(%q) = %v, expected %v",
				testCase.filename, isSupported, testCase.expectedSupport)
		}
	}
}
