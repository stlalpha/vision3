package file

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// FileManager manages file areas and their associated file records.
type FileManager struct {
	basePath    string               // Base directory for all file areas (e.g., "data/files")
	configPath  string               // Path to file_areas.json
	muAreas     sync.RWMutex         // Mutex for accessing file area definitions
	muFiles     sync.RWMutex         // Mutex for accessing file records (might need finer-grained locking later)
	fileAreas   map[int]*FileArea    // Map AreaID to FileArea definition
	fileTags    map[string]int       // Map Area Tag (uppercase) to AreaID
	fileRecords map[int][]FileRecord // Map AreaID to a slice of its FileRecords
}

// NewFileManager creates and initializes a new FileManager.
func NewFileManager(baseDataPath, baseConfigPath string) (*FileManager, error) {
	fm := &FileManager{
		basePath:    filepath.Join(baseDataPath, "files"),             // e.g., data/files
		configPath:  filepath.Join(baseConfigPath, "file_areas.json"), // e.g., configs/file_areas.json
		fileAreas:   make(map[int]*FileArea),
		fileTags:    make(map[string]int),
		fileRecords: make(map[int][]FileRecord),
	}

	log.Printf("INFO: Loading file areas from: %s", fm.configPath)
	if err := fm.loadAreas(); err != nil {
		return nil, fmt.Errorf("failed to load file areas: %w", err)
	}

	log.Printf("INFO: Loading file records from base path: %s", fm.basePath)
	if err := fm.loadAllFileRecords(); err != nil {
		// Log error but potentially continue if some areas loaded?
		log.Printf("ERROR: Failed to load one or more file record sets: %v", err)
		// Decide if this should be fatal. For now, let's allow startup.
		// return nil, fmt.Errorf("failed to load file records: %w", err)
	}

	return fm, nil
}

// loadAreas loads the FileArea definitions from the configuration file.
func (fm *FileManager) loadAreas() error {
	fm.muAreas.Lock()
	defer fm.muAreas.Unlock()

	data, err := os.ReadFile(fm.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("WARN: File areas config %s not found. No file areas loaded.", fm.configPath)
			// Create an empty file?
			emptyJSON := []byte("[]")
			if writeErr := os.WriteFile(fm.configPath, emptyJSON, 0644); writeErr != nil {
				log.Printf("ERROR: Failed to create empty file areas config %s: %v", fm.configPath, writeErr)
			}
			return nil // Not a fatal error if file doesn't exist initially
		}
		return fmt.Errorf("reading file areas config %s: %w", fm.configPath, err)
	}

	var areas []FileArea
	if err := json.Unmarshal(data, &areas); err != nil {
		return fmt.Errorf("parsing file areas config %s: %w", fm.configPath, err)
	}

	// Clear existing maps before loading new data
	fm.fileAreas = make(map[int]*FileArea)
	fm.fileTags = make(map[string]int)

	for i := range areas {
		area := &areas[i] // Take pointer to the element in the slice
		if area.ID <= 0 {
			log.Printf("WARN: Skipping file area with invalid ID <= 0: %+v", area)
			continue
		}
		if area.Tag == "" {
			log.Printf("WARN: Skipping file area with empty Tag (ID: %d)", area.ID)
			continue
		}
		// Ensure path is clean and relative (security measure)
		area.Path = filepath.Clean(area.Path)
		if filepath.IsAbs(area.Path) || strings.HasPrefix(area.Path, "..") {
			log.Printf("WARN: Skipping file area with invalid path (absolute or traversing up): %s (ID: %d)", area.Path, area.ID)
			continue
		}

		// Ensure area directory exists
		fullAreaPath := filepath.Join(fm.basePath, area.Path)
		if err := os.MkdirAll(fullAreaPath, 0755); err != nil {
			log.Printf("ERROR: Failed to create directory for file area %s (%s): %v. Skipping area.", area.Tag, fullAreaPath, err)
			continue
		}

		ucTag := strings.ToUpper(area.Tag)
		if _, exists := fm.fileTags[ucTag]; exists {
			log.Printf("WARN: Duplicate file area Tag '%s' found. Skipping duplicate definition for ID %d.", area.Tag, area.ID)
			continue
		}
		if _, exists := fm.fileAreas[area.ID]; exists {
			log.Printf("WARN: Duplicate file area ID '%d' found. Skipping duplicate definition for Tag %s.", area.ID, area.Tag)
			continue
		}

		fm.fileAreas[area.ID] = area
		fm.fileTags[ucTag] = area.ID
		log.Printf("DEBUG: Loaded File Area: ID=%d, Tag=%s, Name=%s, Path=%s", area.ID, area.Tag, area.Name, area.Path)
	}

	log.Printf("INFO: Successfully loaded %d file areas.", len(fm.fileAreas))
	return nil
}

// loadAllFileRecords iterates through loaded areas and loads their metadata.
func (fm *FileManager) loadAllFileRecords() error {
	fm.muAreas.RLock() // Need read lock on areas to iterate
	defer fm.muAreas.RUnlock()
	fm.muFiles.Lock() // Need write lock on fileRecords map
	defer fm.muFiles.Unlock()

	fm.fileRecords = make(map[int][]FileRecord) // Reset records
	var totalFilesLoaded int
	var errorsEncountered bool

	for areaID, area := range fm.fileAreas {
		metadataPath := filepath.Join(fm.basePath, area.Path, "metadata.json")
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("DEBUG: Metadata file %s not found for area %s (ID: %d). Assuming no files.", metadataPath, area.Tag, areaID)
				fm.fileRecords[areaID] = []FileRecord{} // Initialize empty slice
				continue
			}
			log.Printf("ERROR: Failed to read metadata file %s for area %s: %v", metadataPath, area.Tag, err)
			errorsEncountered = true
			continue // Skip this area on error
		}

		var records []FileRecord
		if err := json.Unmarshal(data, &records); err != nil {
			log.Printf("ERROR: Failed to parse metadata file %s for area %s: %v", metadataPath, area.Tag, err)
			errorsEncountered = true
			continue // Skip this area on error
		}

		// TODO: Validate records? Ensure filenames exist?
		fm.fileRecords[areaID] = records
		totalFilesLoaded += len(records)
		log.Printf("DEBUG: Loaded %d file records for area %s (ID: %d)", len(records), area.Tag, areaID)
	}

	log.Printf("INFO: Loaded metadata for %d areas, total %d file records.", len(fm.fileAreas), totalFilesLoaded)
	if errorsEncountered {
		return fmt.Errorf("encountered errors while loading file records")
	}
	return nil
}

// saveFileRecords saves the metadata for a specific file area.
func (fm *FileManager) saveFileRecords(areaID int) error {
	fm.muAreas.RLock() // Need read lock for area path
	area, exists := fm.fileAreas[areaID]
	if !exists {
		fm.muAreas.RUnlock()
		return fmt.Errorf("cannot save records for non-existent area ID %d", areaID)
	}
	metadataPath := filepath.Join(fm.basePath, area.Path, "metadata.json")
	fm.muAreas.RUnlock()

	fm.muFiles.Lock() // Need write lock to marshal and write
	defer fm.muFiles.Unlock()

	records, exists := fm.fileRecords[areaID]
	if !exists {
		// This might happen if an area was loaded but metadata failed initially
		// Or if called for a newly created area before any records added.
		log.Printf("DEBUG: No records found in memory for area ID %d during save. Saving empty list.", areaID)
		records = []FileRecord{} // Ensure we save an empty list, not nil
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal file records for area %d: %w", areaID, err)
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file %s: %w", metadataPath, err)
	}

	log.Printf("DEBUG: Saved %d file records for area %s (ID: %d) to %s", len(records), area.Tag, areaID, metadataPath)
	return nil
}

// --- Public API Methods --- //

// ListAreas returns a sorted list of FileArea definitions.
// Filtering by ACS should be done by the caller.
func (fm *FileManager) ListAreas() []FileArea {
	fm.muAreas.RLock()
	defer fm.muAreas.RUnlock()

	areas := make([]FileArea, 0, len(fm.fileAreas))
	for _, area := range fm.fileAreas {
		areas = append(areas, *area) // Append a copy
	}

	// Sort by ID for consistent listing
	sort.Slice(areas, func(i, j int) bool {
		return areas[i].ID < areas[j].ID
	})

	return areas
}

// GetAreaByTag returns a FileArea definition by its tag (case-insensitive).
func (fm *FileManager) GetAreaByTag(tag string) (*FileArea, bool) {
	fm.muAreas.RLock()
	defer fm.muAreas.RUnlock()

	areaID, exists := fm.fileTags[strings.ToUpper(tag)]
	if !exists {
		return nil, false
	}
	area, exists := fm.fileAreas[areaID]
	return area, exists // Return pointer directly
}

// GetAreaByID returns a FileArea definition by its ID.
func (fm *FileManager) GetAreaByID(id int) (*FileArea, bool) {
	fm.muAreas.RLock()
	defer fm.muAreas.RUnlock()

	area, exists := fm.fileAreas[id]
	return area, exists // Return pointer directly
}

// GetFilesForArea returns a slice of FileRecord for a given area ID.
// Returns an empty slice if the area doesn't exist or has no files.
func (fm *FileManager) GetFilesForArea(areaID int) []FileRecord {
	fm.muFiles.RLock()
	defer fm.muFiles.RUnlock()

	records, exists := fm.fileRecords[areaID]
	if !exists {
		return []FileRecord{} // Return empty slice, not nil
	}

	// Return a copy to prevent external modification of the internal slice
	// TODO: Consider sorting options here (filename, date, etc.)
	recordsCopy := make([]FileRecord, len(records))
	copy(recordsCopy, records)
	return recordsCopy
}

// GetFileCountForArea returns the total number of file records for a given area ID.
// Returns 0 if the area doesn't exist or has no files.
func (fm *FileManager) GetFileCountForArea(areaID int) (int, error) {
	fm.muFiles.RLock()         // Acquire read lock for accessing file records
	defer fm.muFiles.RUnlock() // Ensure lock is released

	records, exists := fm.fileRecords[areaID]
	if !exists {
		// Area might not exist or simply hasn't had metadata loaded/saved yet.
		// Returning 0 is appropriate for the caller (runListFiles).
		log.Printf("DEBUG: GetFileCountForArea called for non-existent or unloaded area ID %d. Returning 0.", areaID)
		return 0, nil
	}

	return len(records), nil
}

// GetFilesForAreaPaginated returns a slice of FileRecord for a given area ID,
// limited to the specified page and pageSize.
// Returns an empty slice if the area doesn't exist, has no files, or the page is out of bounds.
func (fm *FileManager) GetFilesForAreaPaginated(areaID int, page int, pageSize int) ([]FileRecord, error) {
	fm.muFiles.RLock()
	defer fm.muFiles.RUnlock()

	if page <= 0 || pageSize <= 0 {
		log.Printf("WARN: GetFilesForAreaPaginated called with invalid page (%d) or pageSize (%d)", page, pageSize)
		// Optionally return an error, but returning empty slice might be simpler for the caller
		return []FileRecord{}, fmt.Errorf("invalid page number or page size")
	}

	records, exists := fm.fileRecords[areaID]
	if !exists || len(records) == 0 {
		// log.Printf("DEBUG: GetFilesForAreaPaginated called for non-existent, unloaded, or empty area ID %d.", areaID)
		return []FileRecord{}, nil // Return empty slice if area empty or doesn't exist
	}

	totalFiles := len(records)
	startIndex := (page - 1) * pageSize

	// Check if start index is out of bounds
	if startIndex >= totalFiles {
		log.Printf("DEBUG: GetFilesForAreaPaginated requested page %d which is out of bounds (Total: %d, PageSize: %d)", page, totalFiles, pageSize)
		return []FileRecord{}, nil // Requested page is beyond the last file
	}

	endIndex := startIndex + pageSize
	if endIndex > totalFiles {
		endIndex = totalFiles // Adjust end index if it exceeds the total number of files
	}

	// Slice the records for the current page
	pageRecords := records[startIndex:endIndex]

	// Return a copy to prevent external modification
	recordsCopy := make([]FileRecord, len(pageRecords))
	copy(recordsCopy, pageRecords)

	// log.Printf("DEBUG: GetFilesForAreaPaginated returning %d records for area %d, page %d", len(recordsCopy), areaID, page)
	return recordsCopy, nil
}

// AddFileRecord adds a new file record to the specified area and saves.
func (fm *FileManager) AddFileRecord(record FileRecord) error {
	// Basic validation
	if record.ID == uuid.Nil {
		return fmt.Errorf("file record must have a valid ID")
	}
	if record.AreaID <= 0 {
		return fmt.Errorf("file record must have a valid AreaID")
	}
	if record.Filename == "" {
		return fmt.Errorf("file record must have a Filename")
	}

	fm.muAreas.RLock() // Check if area exists
	_, areaExists := fm.fileAreas[record.AreaID]
	fm.muAreas.RUnlock()
	if !areaExists {
		return fmt.Errorf("cannot add record to non-existent area ID %d", record.AreaID)
	}

	fm.muFiles.Lock()
	defer fm.muFiles.Unlock()

	// Check for duplicate filename within the same area?
	// Or should uploads handle overwrites/renaming?
	// For now, allow duplicates, but log a warning.
	existingRecords := fm.fileRecords[record.AreaID]
	for _, existing := range existingRecords {
		if strings.EqualFold(existing.Filename, record.Filename) {
			log.Printf("WARN: Adding file record with duplicate filename '%s' in area ID %d", record.Filename, record.AreaID)
			break
		}
	}

	fm.fileRecords[record.AreaID] = append(fm.fileRecords[record.AreaID], record)

	// Save changes for this area - release lock during save
	fm.muFiles.Unlock()
	err := fm.saveFileRecords(record.AreaID)
	fm.muFiles.Lock() // Re-acquire lock before returning

	if err != nil {
		// Attempt to rollback the in-memory addition? Complex.
		log.Printf("ERROR: Failed to save file records after adding %s to area %d: %v. In-memory state might be inconsistent!", record.Filename, record.AreaID, err)
		// Maybe remove the last added record if save fails?
		// fm.fileRecords[record.AreaID] = fm.fileRecords[record.AreaID][:len(fm.fileRecords[record.AreaID])-1]
		return err // Propagate save error
	}

	log.Printf("INFO: Added file record '%s' (ID: %s) to area %d.", record.Filename, record.ID, record.AreaID)
	return nil
}

// IncrementDownloadCount increments the download count for a file and saves.
func (fm *FileManager) IncrementDownloadCount(fileID uuid.UUID) error {
	fm.muFiles.Lock()
	defer fm.muFiles.Unlock()

	var foundAreaID int = -1
	var foundIndex int = -1

	// Find the file across all areas
searchLoop:
	for areaID, records := range fm.fileRecords {
		for i := range records {
			if records[i].ID == fileID {
				foundAreaID = areaID
				foundIndex = i
				break searchLoop
			}
		}
	}

	if foundAreaID == -1 {
		return fmt.Errorf("file record with ID %s not found", fileID)
	}

	// Increment count directly on the pointer within the slice
	fm.fileRecords[foundAreaID][foundIndex].DownloadCount++
	newCount := fm.fileRecords[foundAreaID][foundIndex].DownloadCount
	filename := fm.fileRecords[foundAreaID][foundIndex].Filename

	// Save changes for this area - release lock during save
	fm.muFiles.Unlock()
	err := fm.saveFileRecords(foundAreaID)
	fm.muFiles.Lock() // Re-acquire lock before returning

	if err != nil {
		log.Printf("ERROR: Failed to save file records after incrementing download count for %s (ID: %s): %v. In-memory state might be inconsistent!", filename, fileID, err)
		// Attempt rollback?
		// fm.fileRecords[foundAreaID][foundIndex].DownloadCount--
		return err
	}

	log.Printf("DEBUG: Incremented download count for file '%s' (ID: %s) to %d.", filename, fileID, newCount)
	return nil
}

// GetFilePath returns the full, absolute path to a file given its record ID.
// It checks that the file exists and constructs the path safely.
func (fm *FileManager) GetFilePath(fileID uuid.UUID) (string, error) {
	fm.muFiles.RLock() // Need read lock to find the file record
	defer fm.muFiles.RUnlock()
	fm.muAreas.RLock() // Need read lock to get area path
	defer fm.muAreas.RUnlock()

	var foundArea *FileArea
	var foundRecord *FileRecord

searchLoop:
	for areaID, records := range fm.fileRecords {
		for i := range records {
			if records[i].ID == fileID {
				// Get corresponding area
				area, areaExists := fm.fileAreas[areaID]
				if !areaExists {
					// Should not happen if data is consistent
					return "", fmt.Errorf("internal inconsistency: area %d not found for file %s", areaID, fileID)
				}
				foundArea = area
				foundRecord = &records[i] // Get pointer to the record
				break searchLoop
			}
		}
	}

	if foundRecord == nil {
		return "", fmt.Errorf("file record with ID %s not found", fileID)
	}

	// Construct path safely
	// Base path should be absolute for security
	absBasePath, err := filepath.Abs(fm.basePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute base path: %w", err)
	}
	// Area path is relative to base path
	// Filename should be just the base name
	safeFilename := filepath.Base(foundRecord.Filename)
	if safeFilename == "." || safeFilename == "/" || strings.Contains(safeFilename, "..") {
		return "", fmt.Errorf("invalid filename in record: %s", foundRecord.Filename)
	}

	fullPath := filepath.Join(absBasePath, foundArea.Path, safeFilename)

	// Final check: Ensure the resolved path is still within the intended base directory
	if !strings.HasPrefix(fullPath, absBasePath) {
		return "", fmt.Errorf("constructed file path '%s' is outside base directory '%s'", fullPath, absBasePath)
	}

	// Optional: Check if file actually exists on disk?
	// if _, err := os.Stat(fullPath); os.IsNotExist(err) {
	// 	return "", fmt.Errorf("file '%s' not found on disk at expected path '%s'", safeFilename, fullPath)
	// }

	return fullPath, nil
}

// IsSupportedArchive checks if the filename suggests a supported archive type.
// Currently only supports .zip (case-insensitive).
func (fm *FileManager) IsSupportedArchive(filename string) bool {
	lowerFilename := strings.ToLower(filename)
	return strings.HasSuffix(lowerFilename, ".zip")
}
