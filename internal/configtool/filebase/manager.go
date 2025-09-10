package filebase

import (
	"archive/zip"
	"bufio"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var (
	ErrFileNotFound       = errors.New("file not found")
	ErrAreaNotFound       = errors.New("file area not found")
	ErrDuplicateFile      = errors.New("duplicate file")
	ErrInvalidFile        = errors.New("invalid file")
	ErrAreaFull          = errors.New("file area full")
	ErrInsufficientSpace = errors.New("insufficient space")
	ErrUploadFailed      = errors.New("upload failed")
	ErrLockTimeout       = errors.New("lock timeout")
	ErrAreaLocked        = errors.New("area locked by another node")
)

// FileBaseManager handles binary file base operations
type FileBaseManager struct {
	BasePath      string
	NodeNumber    uint8
	lockMutex     sync.RWMutex
	locks         map[string]*os.File
	uploadSessions map[uint32]*UploadSession
	sessionCounter uint32
}

// NewFileBaseManager creates a new file base manager
func NewFileBaseManager(basePath string, nodeNumber uint8) *FileBaseManager {
	return &FileBaseManager{
		BasePath:       basePath,
		NodeNumber:     nodeNumber,
		locks:         make(map[string]*os.File),
		uploadSessions: make(map[uint32]*UploadSession),
		sessionCounter: 0,
	}
}

// Initialize creates the file base directory structure and files
func (fbm *FileBaseManager) Initialize() error {
	// Create base directory
	if err := os.MkdirAll(fbm.BasePath, 0755); err != nil {
		return fmt.Errorf("failed to create file base directory: %w", err)
	}

	// Initialize area configuration file
	areaPath := filepath.Join(fbm.BasePath, AreaConfigFile)
	if _, err := os.Stat(areaPath); os.IsNotExist(err) {
		file, err := os.Create(areaPath)
		if err != nil {
			return fmt.Errorf("failed to create area config file: %w", err)
		}
		file.Close()
	}

	// Initialize statistics file
	statsPath := filepath.Join(fbm.BasePath, StatsFile)
	if _, err := os.Stat(statsPath); os.IsNotExist(err) {
		stats := FileBaseStats{
			TotalAreas: 0,
			TotalFiles: 0,
			TotalSize:  0,
		}
		copy(stats.LastUpdate[:], time.Now().Format("2006-01-02 15:04:05"))
		copy(stats.LastPack[:], time.Now().Format("2006-01-02 15:04:05"))

		if err := fbm.writeStatsFile(stats); err != nil {
			return fmt.Errorf("failed to initialize stats file: %w", err)
		}
	}

	// Initialize file indexes
	indexPath := filepath.Join(fbm.BasePath, FileIndexFile)
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		file, err := os.Create(indexPath)
		if err != nil {
			return fmt.Errorf("failed to create file index: %w", err)
		}
		file.Close()
	}

	// Initialize duplicate detection file
	dupPath := filepath.Join(fbm.BasePath, DuplicateFile)
	if _, err := os.Stat(dupPath); os.IsNotExist(err) {
		file, err := os.Create(dupPath)
		if err != nil {
			return fmt.Errorf("failed to create duplicate index: %w", err)
		}
		file.Close()
	}

	return nil
}

// AcquireLock acquires a multi-node lock for safe operations
func (fbm *FileBaseManager) AcquireLock(lockType uint8, areaNum uint16, timeout time.Duration) error {
	fbm.lockMutex.Lock()
	defer fbm.lockMutex.Unlock()

	lockPath := filepath.Join(fbm.BasePath, LockFile)
	lockKey := fmt.Sprintf("%d-%d", lockType, areaNum)

	// Check if we already have this lock
	if _, exists := fbm.locks[lockKey]; exists {
		return nil // Already locked
	}

	start := time.Now()
	for {
		// Try to acquire exclusive lock
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			// Lock acquired, write lock record
			lock := FileBaseLock{
				NodeNum:  fbm.NodeNumber,
				LockType: lockType,
				AreaNum:  areaNum,
			}
			copy(lock.Timestamp[:], time.Now().Format("2006-01-02 15:04:05"))
			copy(lock.Process[:], "FILEBASE")

			if err := binary.Write(file, binary.LittleEndian, lock); err != nil {
				file.Close()
				os.Remove(lockPath)
				return fmt.Errorf("failed to write lock record: %w", err)
			}

			fbm.locks[lockKey] = file
			return nil
		}

		// Check if timeout exceeded
		if time.Since(start) > timeout {
			return ErrLockTimeout
		}

		// Wait a bit before retrying
		time.Sleep(100 * time.Millisecond)
	}
}

// ReleaseLock releases a previously acquired lock
func (fbm *FileBaseManager) ReleaseLock(lockType uint8, areaNum uint16) error {
	fbm.lockMutex.Lock()
	defer fbm.lockMutex.Unlock()

	lockKey := fmt.Sprintf("%d-%d", lockType, areaNum)
	file, exists := fbm.locks[lockKey]
	if !exists {
		return nil // Lock not held
	}

	file.Close()
	delete(fbm.locks, lockKey)

	lockPath := filepath.Join(fbm.BasePath, LockFile)
	return os.Remove(lockPath)
}

// CreateFileArea creates a new file area
func (fbm *FileBaseManager) CreateFileArea(config FileAreaConfig) error {
	if err := fbm.AcquireLock(FileLockWrite, 0, 5*time.Second); err != nil {
		return err
	}
	defer fbm.ReleaseLock(FileLockWrite, 0)

	// Read existing areas to check for duplicates
	areas, err := fbm.GetFileAreas()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Check for duplicate area numbers or tags
	areaTag := strings.TrimSpace(string(config.AreaTag[:]))
	for _, area := range areas {
		if area.AreaNum == config.AreaNum {
			return fmt.Errorf("area number %d already exists", config.AreaNum)
		}
		if strings.TrimSpace(string(area.AreaTag[:])) == areaTag {
			return fmt.Errorf("area tag %s already exists", areaTag)
		}
	}

	// Create area directory structure
	areaPath := filepath.Join(fbm.BasePath, fmt.Sprintf("AREA%04d", config.AreaNum))
	if err := os.MkdirAll(areaPath, 0755); err != nil {
		return fmt.Errorf("failed to create area directory: %w", err)
	}

	// Create file storage directory
	storagePath := strings.TrimSpace(string(config.Path[:]))
	if storagePath != "" {
		if err := os.MkdirAll(storagePath, 0755); err != nil {
			return fmt.Errorf("failed to create storage directory: %w", err)
		}
	}

	// Initialize area files
	filesDir := filepath.Join(areaPath, FileRecordFile)
	filesExt := filepath.Join(areaPath, ExtendedFile)
	filesIdx := filepath.Join(areaPath, FileIndexFile)

	for _, path := range []string{filesDir, filesExt, filesIdx} {
		file, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("failed to create area file %s: %w", path, err)
		}
		file.Close()
	}

	// Add area to configuration
	configPath := filepath.Join(fbm.BasePath, AreaConfigFile)
	file, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open area config file: %w", err)
	}
	defer file.Close()

	if err := binary.Write(file, binary.LittleEndian, config); err != nil {
		return fmt.Errorf("failed to write area config: %w", err)
	}

	// Update statistics
	stats, err := fbm.GetStats()
	if err == nil {
		stats.TotalAreas++
		fbm.writeStatsFile(stats)
	}

	return nil
}

// GetFileAreas returns all configured file areas
func (fbm *FileBaseManager) GetFileAreas() ([]FileAreaConfig, error) {
	configPath := filepath.Join(fbm.BasePath, AreaConfigFile)
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var areas []FileAreaConfig
	for {
		var config FileAreaConfig
		if err := binary.Read(file, binary.LittleEndian, &config); err != nil {
			break // EOF or error
		}
		areas = append(areas, config)
	}

	return areas, nil
}

// AddFile adds a file to the database (after successful upload)
func (fbm *FileBaseManager) AddFile(areaNum uint16, record FileRecord, extRecord ExtendedFileRecord) error {
	if err := fbm.AcquireLock(FileLockWrite, areaNum, 5*time.Second); err != nil {
		return err
	}
	defer fbm.ReleaseLock(FileLockWrite, areaNum)

	// Check for duplicates
	if err := fbm.checkDuplicate(record, extRecord); err != nil {
		return err
	}

	areaPath := filepath.Join(fbm.BasePath, fmt.Sprintf("AREA%04d", areaNum))

	// Add to main DIR file
	dirPath := filepath.Join(areaPath, FileRecordFile)
	dirFile, err := os.OpenFile(dirPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open DIR file: %w", err)
	}
	defer dirFile.Close()

	recordNum, err := fbm.getNextRecordNumber(areaNum)
	if err != nil {
		return err
	}

	record.Flags |= FileFlagActive
	copy(record.Date[:], time.Now().Format("01-02-06"))
	copy(record.Time[:], time.Now().Format("15:04:05"))

	if err := binary.Write(dirFile, binary.LittleEndian, record); err != nil {
		return fmt.Errorf("failed to write file record: %w", err)
	}

	// Add to extended file
	extPath := filepath.Join(areaPath, ExtendedFile)
	extFile, err := os.OpenFile(extPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open extended file: %w", err)
	}
	defer extFile.Close()

	if err := binary.Write(extFile, binary.LittleEndian, extRecord); err != nil {
		return fmt.Errorf("failed to write extended record: %w", err)
	}

	// Add to index
	index := FileIndex{
		AreaNum:   areaNum,
		RecordNum: recordNum,
		NameHash:  crc32.ChecksumIEEE([]byte(strings.ToUpper(strings.TrimSpace(string(record.FileName[:]))))),
		DescHash:  crc32.ChecksumIEEE([]byte(strings.ToUpper(strings.TrimSpace(string(record.Description[:]))))),
		Size:      record.Size,
		Date:      uint32(time.Now().Unix()),
		Downloads: record.DownloadCount,
		Flags:     record.Flags,
	}

	indexPath := filepath.Join(areaPath, FileIndexFile)
	indexFile, err := os.OpenFile(indexPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open index file: %w", err)
	}
	defer indexFile.Close()

	if err := binary.Write(indexFile, binary.LittleEndian, index); err != nil {
		return fmt.Errorf("failed to write index entry: %w", err)
	}

	// Add to duplicate detection index
	if err := fbm.addToDuplicateIndex(record, areaNum, recordNum); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Warning: failed to update duplicate index: %v\n", err)
	}

	// Update area statistics
	if err := fbm.updateAreaStats(areaNum, 1, int64(record.Size)); err != nil {
		fmt.Printf("Warning: failed to update area stats: %v\n", err)
	}

	return nil
}

// GetFileList returns files in an area with optional sorting and filtering
func (fbm *FileBaseManager) GetFileList(areaNum uint16, sortOrder uint8, filter string) ([]FileRecord, error) {
	if err := fbm.AcquireLock(FileLockRead, areaNum, 5*time.Second); err != nil {
		return nil, err
	}
	defer fbm.ReleaseLock(FileLockRead, areaNum)

	areaPath := filepath.Join(fbm.BasePath, fmt.Sprintf("AREA%04d", areaNum))
	dirPath := filepath.Join(areaPath, FileRecordFile)

	file, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []FileRecord
	for {
		var record FileRecord
		if err := binary.Read(file, binary.LittleEndian, &record); err != nil {
			break // EOF
		}

		// Skip inactive files
		if (record.Flags & FileFlagActive) == 0 {
			continue
		}

		// Apply filter if specified
		if filter != "" {
			fileName := strings.ToLower(strings.TrimSpace(string(record.FileName[:])))
			fileDesc := strings.ToLower(strings.TrimSpace(string(record.Description[:])))
			filterLower := strings.ToLower(filter)

			if !strings.Contains(fileName, filterLower) && !strings.Contains(fileDesc, filterLower) {
				continue
			}
		}

		records = append(records, record)
	}

	// Sort records
	fbm.sortFileRecords(records, sortOrder)

	return records, nil
}

// GetFile returns a specific file record
func (fbm *FileBaseManager) GetFile(areaNum uint16, fileName string) (*FileRecord, *ExtendedFileRecord, error) {
	if err := fbm.AcquireLock(FileLockRead, areaNum, 5*time.Second); err != nil {
		return nil, nil, err
	}
	defer fbm.ReleaseLock(FileLockRead, areaNum)

	areaPath := filepath.Join(fbm.BasePath, fmt.Sprintf("AREA%04d", areaNum))
	dirPath := filepath.Join(areaPath, FileRecordFile)
	extPath := filepath.Join(areaPath, ExtendedFile)

	// Search in DIR file
	dirFile, err := os.Open(dirPath)
	if err != nil {
		return nil, nil, err
	}
	defer dirFile.Close()

	extFile, err := os.Open(extPath)
	if err != nil {
		return nil, nil, err
	}
	defer extFile.Close()

	recordNum := uint32(0)
	fileNameUpper := strings.ToUpper(fileName)

	for {
		var record FileRecord
		if err := binary.Read(dirFile, binary.LittleEndian, &record); err != nil {
			break // EOF
		}

		if (record.Flags&FileFlagActive) != 0 &&
			strings.ToUpper(strings.TrimSpace(string(record.FileName[:]))) == fileNameUpper {

			// Found the record, now get extended info
			extFile.Seek(int64(recordNum)*int64(unsafe.Sizeof(ExtendedFileRecord{})), 0)
			var extRecord ExtendedFileRecord
			if err := binary.Read(extFile, binary.LittleEndian, &extRecord); err == nil {
				return &record, &extRecord, nil
			}
			return &record, nil, nil
		}
		recordNum++
	}

	return nil, nil, ErrFileNotFound
}

// DeleteFile marks a file as deleted (classic BBS style)
func (fbm *FileBaseManager) DeleteFile(areaNum uint16, fileName string) error {
	if err := fbm.AcquireLock(FileLockWrite, areaNum, 5*time.Second); err != nil {
		return err
	}
	defer fbm.ReleaseLock(FileLockWrite, areaNum)

	areaPath := filepath.Join(fbm.BasePath, fmt.Sprintf("AREA%04d", areaNum))
	dirPath := filepath.Join(areaPath, FileRecordFile)

	// Find and update record
	file, err := os.OpenFile(dirPath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	fileNameUpper := strings.ToUpper(fileName)
	recordNum := uint32(0)

	for {
		var record FileRecord
		offset := int64(recordNum) * int64(unsafe.Sizeof(FileRecord{}))

		if _, err := file.Seek(offset, 0); err != nil {
			break
		}

		if err := binary.Read(file, binary.LittleEndian, &record); err != nil {
			break // EOF
		}

		if (record.Flags&FileFlagActive) != 0 &&
			strings.ToUpper(strings.TrimSpace(string(record.FileName[:]))) == fileNameUpper {

			// Mark as deleted
			record.Flags &= ^uint8(FileFlagActive)
			file.Seek(offset, 0)
			binary.Write(file, binary.LittleEndian, record)

			// Update area statistics
			fbm.updateAreaStats(areaNum, -1, -int64(record.Size))
			return nil
		}
		recordNum++
	}

	return ErrFileNotFound
}

// ImportFromDisk scans a directory and imports files with FILE_ID.DIZ descriptions
func (fbm *FileBaseManager) ImportFromDisk(areaNum uint16, scanPath string, autoValidate bool) error {
	if err := fbm.AcquireLock(FileLockImport, areaNum, 30*time.Second); err != nil {
		return err
	}
	defer fbm.ReleaseLock(FileLockImport, areaNum)

	return filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip hidden files and common system files
		fileName := info.Name()
		if strings.HasPrefix(fileName, ".") || strings.HasSuffix(strings.ToLower(fileName), ".bak") {
			return nil
		}

		// Check if file already exists
		existing, _, err := fbm.GetFile(areaNum, fileName)
		if err == nil && existing != nil {
			return nil // File already exists
		}

		// Create file record
		record := FileRecord{
			Size:   uint32(info.Size()),
			Flags:  FileFlagActive,
			Rating: 5, // Default rating
		}
		copy(record.FileName[:], fileName)

		// Create extended record
		extRecord := ExtendedFileRecord{
			FileRecord: record,
		}

		// Try to extract FILE_ID.DIZ description
		description := fbm.extractFileIdDiz(path)
		if description != "" {
			copy(record.Description[:], description[:min(len(description), 45)])
			copy(extRecord.LongDesc[:], description)
		} else {
			// Use filename as description
			copy(record.Description[:], fileName)
		}

		// Calculate file hash
		if hash, err := fbm.calculateMD5(path); err == nil {
			copy(extRecord.MD5Hash[:], hash)
		}

		if autoValidate {
			record.Flags |= FileFlagValidated
			copy(extRecord.ValidateBy[:], "AUTO-IMPORT")
			copy(extRecord.ValidateDate[:], time.Now().Format("2006-01-02 15:04:05"))
		}

		// Add to database
		return fbm.AddFile(areaNum, record, extRecord)
	})
}

// PackFiles removes deleted files and compacts the database
func (fbm *FileBaseManager) PackFiles(areaNum uint16) error {
	if err := fbm.AcquireLock(FileLockPack, areaNum, 30*time.Second); err != nil {
		return err
	}
	defer fbm.ReleaseLock(FileLockPack, areaNum)

	areaPath := filepath.Join(fbm.BasePath, fmt.Sprintf("AREA%04d", areaNum))
	dirPath := filepath.Join(areaPath, FileRecordFile)
	extPath := filepath.Join(areaPath, ExtendedFile)
	idxPath := filepath.Join(areaPath, FileIndexFile)

	// Read active records
	var activeRecords []FileRecord
	var activeExtended []ExtendedFileRecord
	var activeIndexes []FileIndex

	// Read DIR file
	dirFile, err := os.Open(dirPath)
	if err != nil {
		return err
	}
	defer dirFile.Close()

	extFile, err := os.Open(extPath)
	if err != nil {
		return err
	}
	defer extFile.Close()

	recordNum := uint32(0)
	for {
		var record FileRecord
		if err := binary.Read(dirFile, binary.LittleEndian, &record); err != nil {
			break // EOF
		}

		if (record.Flags & FileFlagActive) != 0 {
			activeRecords = append(activeRecords, record)

			// Read corresponding extended record
			extFile.Seek(int64(recordNum)*int64(unsafe.Sizeof(ExtendedFileRecord{})), 0)
			var extRecord ExtendedFileRecord
			if err := binary.Read(extFile, binary.LittleEndian, &extRecord); err == nil {
				activeExtended = append(activeExtended, extRecord)
			}

			// Create new index entry
			index := FileIndex{
				AreaNum:   areaNum,
				RecordNum: uint32(len(activeRecords) - 1),
				NameHash:  crc32.ChecksumIEEE([]byte(strings.ToUpper(strings.TrimSpace(string(record.FileName[:]))))),
				DescHash:  crc32.ChecksumIEEE([]byte(strings.ToUpper(strings.TrimSpace(string(record.Description[:]))))),
				Size:      record.Size,
				Date:      uint32(time.Now().Unix()),
				Downloads: record.DownloadCount,
				Flags:     record.Flags,
			}
			activeIndexes = append(activeIndexes, index)
		}
		recordNum++
	}

	// Backup original files
	backupSuffix := fmt.Sprintf(".bak.%d", time.Now().Unix())
	os.Rename(dirPath, dirPath+backupSuffix)
	os.Rename(extPath, extPath+backupSuffix)
	os.Rename(idxPath, idxPath+backupSuffix)

	// Write compacted files
	newDirFile, err := os.Create(dirPath)
	if err != nil {
		return err
	}
	defer newDirFile.Close()

	newExtFile, err := os.Create(extPath)
	if err != nil {
		return err
	}
	defer newExtFile.Close()

	newIdxFile, err := os.Create(idxPath)
	if err != nil {
		return err
	}
	defer newIdxFile.Close()

	// Write active records
	for i, record := range activeRecords {
		if err := binary.Write(newDirFile, binary.LittleEndian, record); err != nil {
			return err
		}

		if i < len(activeExtended) {
			if err := binary.Write(newExtFile, binary.LittleEndian, activeExtended[i]); err != nil {
				return err
			}
		}

		if i < len(activeIndexes) {
			if err := binary.Write(newIdxFile, binary.LittleEndian, activeIndexes[i]); err != nil {
				return err
			}
		}
	}

	// Update area configuration
	if err := fbm.updateAreaFileCount(areaNum, uint16(len(activeRecords))); err != nil {
		fmt.Printf("Warning: failed to update area file count: %v\n", err)
	}

	return nil
}

// GetStats returns file base statistics
func (fbm *FileBaseManager) GetStats() (FileBaseStats, error) {
	statsPath := filepath.Join(fbm.BasePath, StatsFile)
	file, err := os.Open(statsPath)
	if err != nil {
		return FileBaseStats{}, err
	}
	defer file.Close()

	var stats FileBaseStats
	if err := binary.Read(file, binary.LittleEndian, &stats); err != nil {
		return FileBaseStats{}, err
	}

	return stats, nil
}

// Helper functions

func (fbm *FileBaseManager) writeStatsFile(stats FileBaseStats) error {
	statsPath := filepath.Join(fbm.BasePath, StatsFile)
	file, err := os.Create(statsPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return binary.Write(file, binary.LittleEndian, stats)
}

func (fbm *FileBaseManager) getNextRecordNumber(areaNum uint16) (uint32, error) {
	areaPath := filepath.Join(fbm.BasePath, fmt.Sprintf("AREA%04d", areaNum))
	dirPath := filepath.Join(areaPath, FileRecordFile)

	file, err := os.Open(dirPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Count existing records
	count := uint32(0)
	for {
		var record FileRecord
		if err := binary.Read(file, binary.LittleEndian, &record); err != nil {
			break // EOF
		}
		count++
	}

	return count, nil
}

func (fbm *FileBaseManager) checkDuplicate(record FileRecord, extRecord ExtendedFileRecord) error {
	// Check filename duplicates
	fileName := strings.TrimSpace(string(record.FileName[:]))
	dupPath := filepath.Join(fbm.BasePath, DuplicateFile)

	file, err := os.Open(dupPath)
	if err != nil {
		return nil // No duplicate file exists yet
	}
	defer file.Close()

	for {
		var dup DuplicateIndex
		if err := binary.Read(file, binary.LittleEndian, &dup); err != nil {
			break // EOF
		}

		dupFileName := strings.TrimSpace(string(dup.FileName[:]))
		if strings.EqualFold(fileName, dupFileName) && record.Size == dup.FileSize {
			return ErrDuplicateFile
		}
	}

	return nil
}

func (fbm *FileBaseManager) addToDuplicateIndex(record FileRecord, areaNum uint16, recordNum uint32) error {
	dupPath := filepath.Join(fbm.BasePath, DuplicateFile)
	file, err := os.OpenFile(dupPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	dup := DuplicateIndex{
		FileHash:  crc32.ChecksumIEEE([]byte(strings.TrimSpace(string(record.FileName[:])))),
		FileSize:  record.Size,
		AreaNum:   areaNum,
		RecordNum: recordNum,
	}
	copy(dup.FileName[:], record.FileName[:])

	return binary.Write(file, binary.LittleEndian, dup)
}

func (fbm *FileBaseManager) updateAreaStats(areaNum uint16, fileDelta int, sizeDelta int64) error {
	// Update area configuration with new file count and size
	areas, err := fbm.GetFileAreas()
	if err != nil {
		return err
	}

	for i := range areas {
		if areas[i].AreaNum == areaNum {
			if fileDelta > 0 {
				areas[i].TotalFiles += uint16(fileDelta)
			} else if fileDelta < 0 && areas[i].TotalFiles > 0 {
				areas[i].TotalFiles -= uint16(-fileDelta)
			}

			if sizeDelta > 0 {
				areas[i].TotalSize += uint32(sizeDelta)
			} else if sizeDelta < 0 && areas[i].TotalSize > uint32(-sizeDelta) {
				areas[i].TotalSize -= uint32(-sizeDelta)
			}

			// Write updated config (simplified implementation)
			return fbm.updateAreaConfig(areas[i])
		}
	}

	return ErrAreaNotFound
}

func (fbm *FileBaseManager) updateAreaConfig(config FileAreaConfig) error {
	// This is a simplified implementation - in practice, you'd want to 
	// read all configs, update the specific one, and rewrite the file
	return nil
}

func (fbm *FileBaseManager) updateAreaFileCount(areaNum uint16, fileCount uint16) error {
	areas, err := fbm.GetFileAreas()
	if err != nil {
		return err
	}

	for i := range areas {
		if areas[i].AreaNum == areaNum {
			areas[i].TotalFiles = fileCount
			return fbm.updateAreaConfig(areas[i])
		}
	}

	return ErrAreaNotFound
}

func (fbm *FileBaseManager) sortFileRecords(records []FileRecord, sortOrder uint8) {
	switch sortOrder {
	case SortByName:
		sort.Slice(records, func(i, j int) bool {
			nameI := strings.TrimSpace(string(records[i].FileName[:]))
			nameJ := strings.TrimSpace(string(records[j].FileName[:]))
			return strings.ToLower(nameI) < strings.ToLower(nameJ)
		})
	case SortBySize:
		sort.Slice(records, func(i, j int) bool {
			return records[i].Size > records[j].Size // Descending
		})
	case SortByDownloads:
		sort.Slice(records, func(i, j int) bool {
			return records[i].DownloadCount > records[j].DownloadCount // Descending
		})
	case SortByDate:
		sort.Slice(records, func(i, j int) bool {
			// Parse date strings and compare (simplified)
			dateI := strings.TrimSpace(string(records[i].Date[:]))
			dateJ := strings.TrimSpace(string(records[j].Date[:]))
			return dateI > dateJ // Most recent first
		})
	}
}

func (fbm *FileBaseManager) extractFileIdDiz(filePath string) string {
	// Try to extract FILE_ID.DIZ from ZIP files
	if !strings.HasSuffix(strings.ToLower(filePath), ".zip") {
		return ""
	}

	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return ""
	}
	defer reader.Close()

	for _, file := range reader.File {
		if strings.EqualFold(file.Name, "FILE_ID.DIZ") || strings.EqualFold(file.Name, "DESC.SDI") {
			rc, err := file.Open()
			if err != nil {
				continue
			}
			defer rc.Close()

			scanner := bufio.NewScanner(rc)
			var lines []string
			lineCount := 0

			for scanner.Scan() && lineCount < 10 { // Limit to first 10 lines
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					lines = append(lines, line)
					lineCount++
				}
			}

			if len(lines) > 0 {
				return strings.Join(lines, " ")
			}
		}
	}

	return ""
}

func (fbm *FileBaseManager) calculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SearchFiles searches for files matching criteria
func (fbm *FileBaseManager) SearchFiles(areaNum uint16, searchText string, searchFields []string) ([]FileRecord, error) {
	if err := fbm.AcquireLock(FileLockRead, areaNum, 5*time.Second); err != nil {
		return nil, err
	}
	defer fbm.ReleaseLock(FileLockRead, areaNum)

	var matches []FileRecord
	searchText = strings.ToLower(searchText)

	// Get all files in area
	files, err := fbm.GetFileList(areaNum, SortByName, "")
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		found := false

		// Search in specified fields
		for _, field := range searchFields {
			var searchIn string
			switch strings.ToLower(field) {
			case "filename":
				searchIn = strings.ToLower(strings.TrimSpace(string(file.FileName[:])))
			case "description":
				searchIn = strings.ToLower(strings.TrimSpace(string(file.Description[:])))
			case "uploader":
				searchIn = strings.ToLower(strings.TrimSpace(string(file.Uploader[:])))
			}

			if strings.Contains(searchIn, searchText) {
				found = true
				break
			}
		}

		if found {
			matches = append(matches, file)
		}
	}

	return matches, nil
}

// Close cleans up any open locks and files
func (fbm *FileBaseManager) Close() error {
	fbm.lockMutex.Lock()
	defer fbm.lockMutex.Unlock()

	// Release all locks
	for key, file := range fbm.locks {
		file.Close()
		delete(fbm.locks, key)
	}

	// Clean up lock files
	lockPath := filepath.Join(fbm.BasePath, LockFile)
	os.Remove(lockPath)

	return nil
}