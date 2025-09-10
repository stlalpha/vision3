package maintenance

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"vision3/internal/configtool/filebase"
)

// FileBaseMaintenance handles file base maintenance operations
type FileBaseMaintenance struct {
	manager  *filebase.FileBaseManager
	basePath string
	nodeNum  uint8
}

// NewFileBaseMaintenance creates a new file base maintenance handler
func NewFileBaseMaintenance(manager *filebase.FileBaseManager, basePath string, nodeNum uint8) *FileBaseMaintenance {
	return &FileBaseMaintenance{
		manager:  manager,
		basePath: basePath,
		nodeNum:  nodeNum,
	}
}

// DuplicateFileInfo contains information about duplicate files
type DuplicateFileInfo struct {
	AreaNum1    uint16
	AreaNum2    uint16
	FileName1   string
	FileName2   string
	Size        uint32
	MD5Hash     string
	CRC32       uint32
}

// OrphanFileInfo contains information about orphaned files
type OrphanFileInfo struct {
	FilePath    string
	Size        int64
	ModTime     time.Time
	InDatabase  bool
}

// PackAllAreas packs all file areas to remove deleted files
func (fbm *FileBaseMaintenance) PackAllAreas() (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	// Get all file areas
	areas, err := fbm.manager.GetFileAreas()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to get file areas: "+err.Error())
		return result, err
	}

	if len(areas) == 0 {
		result.Details = append(result.Details, "No file areas found to pack")
		result.TimeElapsed = time.Since(start)
		return result, nil
	}

	// Pack each area
	for _, area := range areas {
		areaName := strings.TrimSpace(string(area.AreaName[:]))
		result.Details = append(result.Details, fmt.Sprintf("Packing area %d: %s", area.AreaNum, areaName))

		beforeFiles := area.TotalFiles
		
		if err := fbm.manager.PackFiles(area.AreaNum); err != nil {
			result.Success = false
			result.ErrorsFound++
			result.Details = append(result.Details, fmt.Sprintf("  ERROR: %v", err))
			continue
		}

		// Get updated stats
		updatedAreas, err := fbm.manager.GetFileAreas()
		if err == nil {
			for _, updatedArea := range updatedAreas {
				if updatedArea.AreaNum == area.AreaNum {
					afterFiles := updatedArea.TotalFiles
					removed := beforeFiles - afterFiles
					result.MessagesProcessed += uint32(beforeFiles)
					result.MessagesRemoved += uint32(removed)

					result.Details = append(result.Details,
						fmt.Sprintf("  Before: %d files, After: %d files, Removed: %d",
							beforeFiles, afterFiles, removed))
					break
				}
			}
		}
	}

	result.TimeElapsed = time.Since(start)
	result.Details = append(result.Details,
		fmt.Sprintf("Pack completed in %v", result.TimeElapsed))

	return result, nil
}

// ReindexAllAreas rebuilds all file indexes
func (fbm *FileBaseMaintenance) ReindexAllAreas() (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	areas, err := fbm.manager.GetFileAreas()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to get file areas: "+err.Error())
		return result, err
	}

	for _, area := range areas {
		areaName := strings.TrimSpace(string(area.AreaName[:]))
		result.Details = append(result.Details, fmt.Sprintf("Reindexing area %d: %s", area.AreaNum, areaName))

		if err := fbm.reindexArea(area.AreaNum); err != nil {
			result.Success = false
			result.ErrorsFound++
			result.Details = append(result.Details, fmt.Sprintf("  ERROR: %v", err))
			continue
		}

		result.MessagesProcessed += uint32(area.TotalFiles)
		result.Details = append(result.Details,
			fmt.Sprintf("  Indexed %d files", area.TotalFiles))
	}

	result.TimeElapsed = time.Since(start)
	return result, nil
}

// DetectDuplicates finds duplicate files across all areas
func (fbm *FileBaseMaintenance) DetectDuplicates() (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	duplicates, err := fbm.findDuplicateFiles()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to detect duplicates: "+err.Error())
		return result, err
	}

	result.ErrorsFound = uint32(len(duplicates))
	result.Details = append(result.Details, fmt.Sprintf("Found %d duplicate file pairs", len(duplicates)))

	for _, dup := range duplicates {
		result.Details = append(result.Details,
			fmt.Sprintf("  Duplicate: %s (Area %d) <-> %s (Area %d) - %d bytes",
				dup.FileName1, dup.AreaNum1, dup.FileName2, dup.AreaNum2, dup.Size))
	}

	result.TimeElapsed = time.Since(start)
	return result, nil
}

// RemoveDuplicates removes duplicate files, keeping the first occurrence
func (fbm *FileBaseMaintenance) RemoveDuplicates() (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	duplicates, err := fbm.findDuplicateFiles()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to find duplicates: "+err.Error())
		return result, err
	}

	if len(duplicates) == 0 {
		result.Details = append(result.Details, "No duplicate files found")
		result.TimeElapsed = time.Since(start)
		return result, nil
	}

	removed := uint32(0)
	for _, dup := range duplicates {
		// Remove the duplicate from the second area (keep first occurrence)
		if err := fbm.manager.DeleteFile(dup.AreaNum2, dup.FileName2); err != nil {
			result.ErrorsFound++
			result.Details = append(result.Details,
				fmt.Sprintf("  Failed to remove duplicate %s from area %d: %v",
					dup.FileName2, dup.AreaNum2, err))
		} else {
			removed++
			result.BytesRecovered += uint64(dup.Size)
			result.Details = append(result.Details,
				fmt.Sprintf("  Removed duplicate: %s from area %d",
					dup.FileName2, dup.AreaNum2))
		}
	}

	result.MessagesRemoved = removed
	result.Details = append(result.Details, fmt.Sprintf("Removed %d duplicate files", removed))
	result.TimeElapsed = time.Since(start)

	return result, nil
}

// FindOrphanedFiles finds files on disk that are not in the database
func (fbm *FileBaseMaintenance) FindOrphanedFiles() (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	orphans, err := fbm.findOrphanFiles()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to find orphaned files: "+err.Error())
		return result, err
	}

	result.ErrorsFound = uint32(len(orphans))
	result.Details = append(result.Details, fmt.Sprintf("Found %d orphaned files", len(orphans)))

	totalSize := int64(0)
	for _, orphan := range orphans {
		totalSize += orphan.Size
		result.Details = append(result.Details,
			fmt.Sprintf("  Orphan: %s (%d bytes, %v)",
				orphan.FilePath, orphan.Size, orphan.ModTime.Format("2006-01-02")))
	}

	result.BytesRecovered = uint64(totalSize)
	result.Details = append(result.Details,
		fmt.Sprintf("Total orphaned file size: %d bytes", totalSize))

	result.TimeElapsed = time.Since(start)
	return result, nil
}

// RemoveOrphanedFiles removes orphaned files from disk
func (fbm *FileBaseMaintenance) RemoveOrphanedFiles() (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	orphans, err := fbm.findOrphanFiles()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to find orphaned files: "+err.Error())
		return result, err
	}

	if len(orphans) == 0 {
		result.Details = append(result.Details, "No orphaned files found")
		result.TimeElapsed = time.Since(start)
		return result, nil
	}

	removed := uint32(0)
	for _, orphan := range orphans {
		if err := os.Remove(orphan.FilePath); err != nil {
			result.ErrorsFound++
			result.Details = append(result.Details,
				fmt.Sprintf("  Failed to remove orphan %s: %v", orphan.FilePath, err))
		} else {
			removed++
			result.BytesRecovered += uint64(orphan.Size)
			result.Details = append(result.Details,
				fmt.Sprintf("  Removed orphan: %s", orphan.FilePath))
		}
	}

	result.MessagesRemoved = removed
	result.Details = append(result.Details, fmt.Sprintf("Removed %d orphaned files", removed))
	result.TimeElapsed = time.Since(start)

	return result, nil
}

// ValidateFileIntegrity validates that files on disk match database records
func (fbm *FileBaseMaintenance) ValidateFileIntegrity() (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	areas, err := fbm.manager.GetFileAreas()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to get file areas: "+err.Error())
		return result, err
	}

	totalFiles := uint32(0)
	missingFiles := uint32(0)
	corruptFiles := uint32(0)

	for _, area := range areas {
		areaName := strings.TrimSpace(string(area.AreaName[:]))
		areaPath := strings.TrimSpace(string(area.Path[:]))
		
		result.Details = append(result.Details, fmt.Sprintf("Validating area %d: %s", area.AreaNum, areaName))

		files, err := fbm.manager.GetFileList(area.AreaNum, filebase.SortByName, "")
		if err != nil {
			result.ErrorsFound++
			result.Details = append(result.Details, fmt.Sprintf("  ERROR getting file list: %v", err))
			continue
		}

		for _, file := range files {
			totalFiles++
			fileName := strings.TrimSpace(string(file.FileName[:]))
			filePath := filepath.Join(areaPath, fileName)

			// Check if file exists
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				missingFiles++
				result.ErrorsFound++
				result.Details = append(result.Details,
					fmt.Sprintf("  MISSING: %s", fileName))
				continue
			}

			// Check file size
			if uint32(fileInfo.Size()) != file.Size {
				corruptFiles++
				result.ErrorsFound++
				result.Details = append(result.Details,
					fmt.Sprintf("  SIZE MISMATCH: %s (DB: %d, Disk: %d)",
						fileName, file.Size, fileInfo.Size()))
			}
		}

		result.Details = append(result.Details,
			fmt.Sprintf("  Checked %d files, %d missing, %d size mismatches",
				len(files), missingFiles, corruptFiles))
	}

	result.MessagesProcessed = totalFiles
	result.Details = append(result.Details,
		fmt.Sprintf("Validation completed: %d files checked, %d errors found",
			totalFiles, result.ErrorsFound))

	result.TimeElapsed = time.Since(start)
	return result, nil
}

// RepairFileDatabase attempts to repair file database inconsistencies
func (fbm *FileBaseMaintenance) RepairFileDatabase() (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	areas, err := fbm.manager.GetFileAreas()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to get file areas: "+err.Error())
		return result, err
	}

	for _, area := range areas {
		areaName := strings.TrimSpace(string(area.AreaName[:]))
		result.Details = append(result.Details, fmt.Sprintf("Repairing area %d: %s", area.AreaNum, areaName))

		repairResult := fbm.repairArea(area.AreaNum)
		result.MessagesProcessed += repairResult.MessagesProcessed
		result.ErrorsFound += repairResult.ErrorsFound

		if !repairResult.Success {
			result.Success = false
		}

		for _, detail := range repairResult.Details {
			result.Details = append(result.Details, "  "+detail)
		}
	}

	result.TimeElapsed = time.Since(start)
	return result, nil
}

// BackupFileBase creates a backup of the entire file base
func (fbm *FileBaseMaintenance) BackupFileBase(backupPath string, includeFiles bool) (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	// Create backup directory
	backupDir := filepath.Join(backupPath, fmt.Sprintf("filebase_backup_%d", time.Now().Unix()))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to create backup directory: "+err.Error())
		return result, err
	}

	result.Details = append(result.Details, "Created backup directory: "+backupDir)

	// Get all areas
	areas, err := fbm.manager.GetFileAreas()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to get file areas: "+err.Error())
		return result, err
	}

	// Backup each area
	for _, area := range areas {
		areaName := strings.TrimSpace(string(area.AreaName[:]))
		result.Details = append(result.Details, fmt.Sprintf("Backing up area %d: %s", area.AreaNum, areaName))

		if err := fbm.backupArea(area.AreaNum, backupDir, includeFiles); err != nil {
			result.Success = false
			result.ErrorsFound++
			result.Details = append(result.Details, fmt.Sprintf("  ERROR: %v", err))
			continue
		}

		result.MessagesProcessed += uint32(area.TotalFiles)
		result.Details = append(result.Details, fmt.Sprintf("  Backed up %d files", area.TotalFiles))
	}

	// Backup configuration files
	configFiles := []string{
		filebase.AreaConfigFile,
		filebase.StatsFile,
		filebase.FileIndexFile,
		filebase.DuplicateFile,
	}

	for _, configFile := range configFiles {
		srcPath := filepath.Join(fbm.basePath, configFile)
		dstPath := filepath.Join(backupDir, configFile)

		if err := fbm.copyFile(srcPath, dstPath); err != nil {
			result.ErrorsFound++
			result.Details = append(result.Details, fmt.Sprintf("Failed to backup %s: %v", configFile, err))
		}
	}

	result.TimeElapsed = time.Since(start)
	result.Details = append(result.Details, fmt.Sprintf("Backup completed in %v", result.TimeElapsed))

	return result, nil
}

// Helper functions

func (fbm *FileBaseMaintenance) reindexArea(areaNum uint16) error {
	areaPath := filepath.Join(fbm.basePath, fmt.Sprintf("AREA%04d", areaNum))

	// Acquire write lock
	if err := fbm.manager.AcquireLock(filebase.FileLockWrite, areaNum, 30*time.Second); err != nil {
		return err
	}
	defer fbm.manager.ReleaseLock(filebase.FileLockWrite, areaNum)

	// Read all file records
	dirPath := filepath.Join(areaPath, filebase.FileRecordFile)
	dirFile, err := os.Open(dirPath)
	if err != nil {
		return fmt.Errorf("failed to open DIR file: %w", err)
	}
	defer dirFile.Close()

	// Rebuild index file
	indexPath := filepath.Join(areaPath, filebase.FileIndexFile)
	indexFile, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("failed to create index file: %w", err)
	}
	defer indexFile.Close()

	recordNum := uint32(0)
	for {
		var record filebase.FileRecord
		if err := binary.Read(dirFile, binary.LittleEndian, &record); err != nil {
			break // EOF
		}

		if (record.Flags & filebase.FileFlagActive) != 0 {
			// Create index entry
			fileName := strings.TrimSpace(string(record.FileName[:]))
			description := strings.TrimSpace(string(record.Description[:]))

			index := filebase.FileIndex{
				AreaNum:   areaNum,
				RecordNum: recordNum,
				NameHash:  crc32.ChecksumIEEE([]byte(strings.ToUpper(fileName))),
				DescHash:  crc32.ChecksumIEEE([]byte(strings.ToUpper(description))),
				Size:      record.Size,
				Date:      uint32(time.Now().Unix()), // Simplified - should parse actual date
				Downloads: record.DownloadCount,
				Flags:     record.Flags,
			}

			if err := binary.Write(indexFile, binary.LittleEndian, index); err != nil {
				return fmt.Errorf("failed to write index entry: %w", err)
			}
		}

		recordNum++
	}

	return nil
}

func (fbm *FileBaseMaintenance) findDuplicateFiles() ([]DuplicateFileInfo, error) {
	var duplicates []DuplicateFileInfo
	fileMap := make(map[string][]struct {
		AreaNum  uint16
		FileName string
		Size     uint32
	})

	areas, err := fbm.manager.GetFileAreas()
	if err != nil {
		return nil, err
	}

	// Build map of files by size
	for _, area := range areas {
		files, err := fbm.manager.GetFileList(area.AreaNum, filebase.SortByName, "")
		if err != nil {
			continue
		}

		for _, file := range files {
			fileName := strings.TrimSpace(string(file.FileName[:]))
			sizeKey := fmt.Sprintf("%d", file.Size)

			fileMap[sizeKey] = append(fileMap[sizeKey], struct {
				AreaNum  uint16
				FileName string
				Size     uint32
			}{
				AreaNum:  area.AreaNum,
				FileName: fileName,
				Size:     file.Size,
			})
		}
	}

	// Find duplicates by comparing files with same size
	for _, filesWithSameSize := range fileMap {
		if len(filesWithSameSize) < 2 {
			continue
		}

		// Compare each pair
		for i := 0; i < len(filesWithSameSize); i++ {
			for j := i + 1; j < len(filesWithSameSize); j++ {
				file1 := filesWithSameSize[i]
				file2 := filesWithSameSize[j]

				// Check if files are actually identical by comparing content
				if fbm.areFilesIdentical(file1.AreaNum, file1.FileName, file2.AreaNum, file2.FileName) {
					duplicates = append(duplicates, DuplicateFileInfo{
						AreaNum1:  file1.AreaNum,
						AreaNum2:  file2.AreaNum,
						FileName1: file1.FileName,
						FileName2: file2.FileName,
						Size:      file1.Size,
					})
				}
			}
		}
	}

	return duplicates, nil
}

func (fbm *FileBaseMaintenance) areFilesIdentical(area1 uint16, file1 string, area2 uint16, file2 string) bool {
	// Get file paths
	areas, err := fbm.manager.GetFileAreas()
	if err != nil {
		return false
	}

	var path1, path2 string
	for _, area := range areas {
		if area.AreaNum == area1 {
			path1 = filepath.Join(strings.TrimSpace(string(area.Path[:])), file1)
		}
		if area.AreaNum == area2 {
			path2 = filepath.Join(strings.TrimSpace(string(area.Path[:])), file2)
		}
	}

	if path1 == "" || path2 == "" {
		return false
	}

	// Compare MD5 hashes
	hash1, err1 := fbm.calculateMD5(path1)
	hash2, err2 := fbm.calculateMD5(path2)

	return err1 == nil && err2 == nil && hash1 == hash2
}

func (fbm *FileBaseMaintenance) calculateMD5(filePath string) (string, error) {
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

func (fbm *FileBaseMaintenance) findOrphanFiles() ([]OrphanFileInfo, error) {
	var orphans []OrphanFileInfo
	databaseFiles := make(map[string]bool)

	areas, err := fbm.manager.GetFileAreas()
	if err != nil {
		return nil, err
	}

	// Build map of files in database
	for _, area := range areas {
		files, err := fbm.manager.GetFileList(area.AreaNum, filebase.SortByName, "")
		if err != nil {
			continue
		}

		areaPath := strings.TrimSpace(string(area.Path[:]))
		for _, file := range files {
			fileName := strings.TrimSpace(string(file.FileName[:]))
			fullPath := filepath.Join(areaPath, fileName)
			databaseFiles[fullPath] = true
		}
	}

	// Scan disk for files not in database
	for _, area := range areas {
		areaPath := strings.TrimSpace(string(area.Path[:]))
		if areaPath == "" {
			continue
		}

		err := filepath.Walk(areaPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}

			// Skip hidden files and system files
			fileName := info.Name()
			if strings.HasPrefix(fileName, ".") || strings.HasSuffix(strings.ToLower(fileName), ".bak") {
				return nil
			}

			if !databaseFiles[path] {
				orphans = append(orphans, OrphanFileInfo{
					FilePath:   path,
					Size:       info.Size(),
					ModTime:    info.ModTime(),
					InDatabase: false,
				})
			}

			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return orphans, nil
}

func (fbm *FileBaseMaintenance) repairArea(areaNum uint16) *MaintenanceResult {
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	areaPath := filepath.Join(fbm.basePath, fmt.Sprintf("AREA%04d", areaNum))

	// Check if area directory exists
	if _, err := os.Stat(areaPath); os.IsNotExist(err) {
		result.Success = false
		result.ErrorsFound++
		result.Details = append(result.Details, "Area directory does not exist")
		return result
	}

	// Check required files
	requiredFiles := []string{
		filebase.FileRecordFile,
		filebase.ExtendedFile,
		filebase.FileIndexFile,
	}

	for _, filename := range requiredFiles {
		filePath := filepath.Join(areaPath, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			// Create missing file
			if file, err := os.Create(filePath); err == nil {
				file.Close()
				result.Details = append(result.Details, "Created missing file: "+filename)
			} else {
				result.Success = false
				result.ErrorsFound++
				result.Details = append(result.Details, "Failed to create missing file: "+filename)
			}
		}
	}

	// Verify file record consistency
	if err := fbm.verifyFileRecordConsistency(areaNum, result); err != nil {
		result.Success = false
		result.ErrorsFound++
		result.Details = append(result.Details, "File record consistency error: "+err.Error())
	}

	return result
}

func (fbm *FileBaseMaintenance) verifyFileRecordConsistency(areaNum uint16, result *MaintenanceResult) error {
	areaPath := filepath.Join(fbm.basePath, fmt.Sprintf("AREA%04d", areaNum))

	// Check DIR file
	dirPath := filepath.Join(areaPath, filebase.FileRecordFile)
	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		return err
	}

	// Check if DIR file size is multiple of record size
	recordSize := unsafe.Sizeof(filebase.FileRecord{})
	if dirInfo.Size()%int64(recordSize) != 0 {
		result.ErrorsFound++
		result.Details = append(result.Details,
			"DIR file size is not aligned to record size")
	}

	// Check extended file
	extPath := filepath.Join(areaPath, filebase.ExtendedFile)
	extInfo, err := os.Stat(extPath)
	if err == nil {
		extRecordSize := unsafe.Sizeof(filebase.ExtendedFileRecord{})
		if extInfo.Size()%int64(extRecordSize) != 0 {
			result.ErrorsFound++
			result.Details = append(result.Details,
				"Extended file size is not aligned to record size")
		}

		// Check if DIR and extended files have matching record counts
		dirRecords := dirInfo.Size() / int64(recordSize)
		extRecords := extInfo.Size() / int64(extRecordSize)
		if dirRecords != extRecords {
			result.ErrorsFound++
			result.Details = append(result.Details,
				fmt.Sprintf("Record count mismatch: DIR=%d, Extended=%d", dirRecords, extRecords))
		}
	}

	expectedRecords := uint32(dirInfo.Size() / int64(recordSize))
	result.MessagesProcessed = expectedRecords

	return nil
}

func (fbm *FileBaseMaintenance) backupArea(areaNum uint16, backupDir string, includeFiles bool) error {
	areaPath := filepath.Join(fbm.basePath, fmt.Sprintf("AREA%04d", areaNum))
	backupAreaPath := filepath.Join(backupDir, fmt.Sprintf("AREA%04d", areaNum))

	// Create backup area directory
	if err := os.MkdirAll(backupAreaPath, 0755); err != nil {
		return err
	}

	// Files to backup (database files)
	filesToBackup := []string{
		filebase.FileRecordFile,
		filebase.ExtendedFile,
		filebase.FileIndexFile,
	}

	for _, filename := range filesToBackup {
		srcPath := filepath.Join(areaPath, filename)
		dstPath := filepath.Join(backupAreaPath, filename)

		if err := fbm.copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to backup %s: %w", filename, err)
		}
	}

	// Optionally backup actual files
	if includeFiles {
		areas, err := fbm.manager.GetFileAreas()
		if err != nil {
			return err
		}

		for _, area := range areas {
			if area.AreaNum == areaNum {
				storagePath := strings.TrimSpace(string(area.Path[:]))
				if storagePath != "" {
					backupStoragePath := filepath.Join(backupAreaPath, "files")
					if err := os.MkdirAll(backupStoragePath, 0755); err != nil {
						return err
					}

					// Copy all files
					return filepath.Walk(storagePath, func(path string, info os.FileInfo, err error) error {
						if err != nil || info.IsDir() {
							return nil
						}

						relPath, _ := filepath.Rel(storagePath, path)
						dstPath := filepath.Join(backupStoragePath, relPath)

						// Create directory if needed
						if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
							return err
						}

						return fbm.copyFile(path, dstPath)
					})
				}
				break
			}
		}
	}

	return nil
}

func (fbm *FileBaseMaintenance) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	buf := make([]byte, 32768) // 32KB buffer
	for {
		n, err := srcFile.Read(buf)
		if n > 0 {
			if _, err := dstFile.Write(buf[:n]); err != nil {
				return err
			}
		}
		if err != nil {
			break
		}
	}

	return nil
}