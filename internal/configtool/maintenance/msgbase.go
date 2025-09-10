package maintenance

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/stlalpha/vision3/internal/configtool/msgbase"
)

// MessageBaseMaintenance handles message base maintenance operations
type MessageBaseMaintenance struct {
	manager  *msgbase.MessageBaseManager
	basePath string
	nodeNum  uint8
}

// NewMessageBaseMaintenance creates a new message base maintenance handler
func NewMessageBaseMaintenance(manager *msgbase.MessageBaseManager, basePath string, nodeNum uint8) *MessageBaseMaintenance {
	return &MessageBaseMaintenance{
		manager:  manager,
		basePath: basePath,
		nodeNum:  nodeNum,
	}
}

// MaintenanceResult contains the results of a maintenance operation
type MaintenanceResult struct {
	Success      bool
	MessagesProcessed uint32
	MessagesRemoved  uint32
	BytesRecovered   uint64
	ErrorsFound      uint32
	TimeElapsed      time.Duration
	Details          []string
}

// PackAllAreas packs all message areas to remove deleted messages
func (mbm *MessageBaseMaintenance) PackAllAreas() (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	// Get all message areas
	areas, err := mbm.manager.GetMessageAreas()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to get message areas: "+err.Error())
		return result, err
	}

	if len(areas) == 0 {
		result.Details = append(result.Details, "No message areas found to pack")
		result.TimeElapsed = time.Since(start)
		return result, nil
	}

	// Pack each area
	for _, area := range areas {
		areaName := strings.TrimSpace(string(area.AreaName[:]))
		result.Details = append(result.Details, fmt.Sprintf("Packing area %d: %s", area.AreaNum, areaName))

		beforeStats := mbm.getAreaStats(area.AreaNum)
		
		if err := mbm.manager.PackMessages(area.AreaNum); err != nil {
			result.Success = false
			result.ErrorsFound++
			result.Details = append(result.Details, fmt.Sprintf("  ERROR: %v", err))
			continue
		}

		afterStats := mbm.getAreaStats(area.AreaNum)
		removed := beforeStats.TotalMsgs - afterStats.TotalMsgs
		result.MessagesProcessed += beforeStats.TotalMsgs
		result.MessagesRemoved += removed
		result.BytesRecovered += uint64(beforeStats.TotalKBytes - afterStats.TotalKBytes) * 1024

		result.Details = append(result.Details, 
			fmt.Sprintf("  Before: %d messages, After: %d messages, Removed: %d",
				beforeStats.TotalMsgs, afterStats.TotalMsgs, removed))
	}

	result.TimeElapsed = time.Since(start)
	result.Details = append(result.Details, 
		fmt.Sprintf("Pack completed in %v", result.TimeElapsed))
	
	return result, nil
}

// ReindexAllAreas rebuilds all message indexes
func (mbm *MessageBaseMaintenance) ReindexAllAreas() (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	areas, err := mbm.manager.GetMessageAreas()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to get message areas: "+err.Error())
		return result, err
	}

	for _, area := range areas {
		areaName := strings.TrimSpace(string(area.AreaName[:]))
		result.Details = append(result.Details, fmt.Sprintf("Reindexing area %d: %s", area.AreaNum, areaName))

		if err := mbm.reindexArea(area.AreaNum); err != nil {
			result.Success = false
			result.ErrorsFound++
			result.Details = append(result.Details, fmt.Sprintf("  ERROR: %v", err))
			continue
		}

		stats := mbm.getAreaStats(area.AreaNum)
		result.MessagesProcessed += stats.TotalMsgs
		result.Details = append(result.Details, 
			fmt.Sprintf("  Indexed %d messages", stats.TotalMsgs))
	}

	result.TimeElapsed = time.Since(start)
	return result, nil
}

// RepairAllAreas attempts to repair corrupted message areas
func (mbm *MessageBaseMaintenance) RepairAllAreas() (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	areas, err := mbm.manager.GetMessageAreas()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to get message areas: "+err.Error())
		return result, err
	}

	for _, area := range areas {
		areaName := strings.TrimSpace(string(area.AreaName[:]))
		result.Details = append(result.Details, fmt.Sprintf("Repairing area %d: %s", area.AreaNum, areaName))

		repairResult := mbm.repairArea(area.AreaNum)
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

// VerifyIntegrity verifies the integrity of all message areas
func (mbm *MessageBaseMaintenance) VerifyIntegrity() (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	areas, err := mbm.manager.GetMessageAreas()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to get message areas: "+err.Error())
		return result, err
	}

	for _, area := range areas {
		areaName := strings.TrimSpace(string(area.AreaName[:]))
		result.Details = append(result.Details, fmt.Sprintf("Verifying area %d: %s", area.AreaNum, areaName))

		verifyResult := mbm.verifyAreaIntegrity(area.AreaNum)
		result.MessagesProcessed += verifyResult.MessagesProcessed
		result.ErrorsFound += verifyResult.ErrorsFound

		if !verifyResult.Success {
			result.Success = false
		}

		for _, detail := range verifyResult.Details {
			result.Details = append(result.Details, "  "+detail)
		}
	}

	result.TimeElapsed = time.Since(start)
	return result, nil
}

// PurgeOldMessages removes messages older than specified days
func (mbm *MessageBaseMaintenance) PurgeOldMessages(daysOld int) (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	if daysOld <= 0 {
		result.Success = false
		result.Details = append(result.Details, "Invalid days parameter")
		return result, fmt.Errorf("days must be greater than 0")
	}

	cutoffDate := time.Now().AddDate(0, 0, -daysOld)
	result.Details = append(result.Details, fmt.Sprintf("Purging messages older than %v", cutoffDate.Format("2006-01-02")))

	areas, err := mbm.manager.GetMessageAreas()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to get message areas: "+err.Error())
		return result, err
	}

	for _, area := range areas {
		areaName := strings.TrimSpace(string(area.AreaName[:]))
		result.Details = append(result.Details, fmt.Sprintf("Purging area %d: %s", area.AreaNum, areaName))

		purged, err := mbm.purgeAreaMessages(area.AreaNum, cutoffDate)
		if err != nil {
			result.Success = false
			result.ErrorsFound++
			result.Details = append(result.Details, fmt.Sprintf("  ERROR: %v", err))
			continue
		}

		result.MessagesRemoved += purged
		result.Details = append(result.Details, fmt.Sprintf("  Purged %d old messages", purged))
	}

	result.TimeElapsed = time.Since(start)
	return result, nil
}

// BackupMessageBase creates a backup of the entire message base
func (mbm *MessageBaseMaintenance) BackupMessageBase(backupPath string) (*MaintenanceResult, error) {
	start := time.Now()
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	// Create backup directory
	backupDir := filepath.Join(backupPath, fmt.Sprintf("msgbase_backup_%d", time.Now().Unix()))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to create backup directory: "+err.Error())
		return result, err
	}

	result.Details = append(result.Details, "Created backup directory: "+backupDir)

	// Get all areas
	areas, err := mbm.manager.GetMessageAreas()
	if err != nil {
		result.Success = false
		result.Details = append(result.Details, "Failed to get message areas: "+err.Error())
		return result, err
	}

	// Backup each area
	for _, area := range areas {
		areaName := strings.TrimSpace(string(area.AreaName[:]))
		result.Details = append(result.Details, fmt.Sprintf("Backing up area %d: %s", area.AreaNum, areaName))

		if err := mbm.backupArea(area.AreaNum, backupDir); err != nil {
			result.Success = false
			result.ErrorsFound++
			result.Details = append(result.Details, fmt.Sprintf("  ERROR: %v", err))
			continue
		}

		stats := mbm.getAreaStats(area.AreaNum)
		result.MessagesProcessed += stats.TotalMsgs
		result.Details = append(result.Details, fmt.Sprintf("  Backed up %d messages", stats.TotalMsgs))
	}

	// Backup configuration files
	configFiles := []string{
		msgbase.AreaConfigFile,
		msgbase.StatsFile,
	}

	for _, configFile := range configFiles {
		srcPath := filepath.Join(mbm.basePath, configFile)
		dstPath := filepath.Join(backupDir, configFile)
		
		if err := mbm.copyFile(srcPath, dstPath); err != nil {
			result.ErrorsFound++
			result.Details = append(result.Details, fmt.Sprintf("Failed to backup %s: %v", configFile, err))
		}
	}

	result.TimeElapsed = time.Since(start)
	result.Details = append(result.Details, fmt.Sprintf("Backup completed in %v", result.TimeElapsed))
	
	return result, nil
}

// Helper functions

func (mbm *MessageBaseMaintenance) getAreaStats(areaNum uint16) msgbase.MessageAreaConfig {
	areas, err := mbm.manager.GetMessageAreas()
	if err != nil {
		return msgbase.MessageAreaConfig{}
	}

	for _, area := range areas {
		if area.AreaNum == areaNum {
			return area
		}
	}

	return msgbase.MessageAreaConfig{}
}

func (mbm *MessageBaseMaintenance) reindexArea(areaNum uint16) error {
	areaPath := filepath.Join(mbm.basePath, fmt.Sprintf("AREA%04d", areaNum))
	
	// Acquire write lock
	if err := mbm.manager.AcquireLock(msgbase.LockTypeWrite, areaNum, 30*time.Second); err != nil {
		return err
	}
	defer mbm.manager.ReleaseLock(msgbase.LockTypeWrite, areaNum)

	// Read all message headers
	headerPath := filepath.Join(areaPath, msgbase.MessageHeaderFile)
	headerFile, err := os.Open(headerPath)
	if err != nil {
		return fmt.Errorf("failed to open header file: %w", err)
	}
	defer headerFile.Close()

	// Rebuild index file
	indexPath := filepath.Join(areaPath, msgbase.MessageIndexFile)
	indexFile, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("failed to create index file: %w", err)
	}
	defer indexFile.Close()

	// Rebuild thread index file
	threadPath := filepath.Join(areaPath, msgbase.ThreadIndexFile)
	threadFile, err := os.Create(threadPath)
	if err != nil {
		return fmt.Errorf("failed to create thread file: %w", err)
	}
	defer threadFile.Close()

	// Process all headers
	offset := uint32(0)
	threads := make(map[uint32]*msgbase.ThreadIndex)

	for {
		var header msgbase.MessageHeader
		if err := binary.Read(headerFile, binary.LittleEndian, &header); err != nil {
			break // EOF
		}

		if (header.Status & msgbase.MsgStatusActive) != 0 {
			// Create index entry
			index := msgbase.MessageIndex{
				MsgNum: header.MsgNum,
				Offset: offset,
				Length: uint32(unsafe.Sizeof(header)) + uint32(header.NumBlocks*msgbase.MessageBlockSize),
				Status: header.Status,
			}

			if err := binary.Write(indexFile, binary.LittleEndian, index); err != nil {
				return fmt.Errorf("failed to write index entry: %w", err)
			}

			// Update thread information
			if header.ReplyTo > 0 {
				if thread, exists := threads[header.ReplyTo]; exists {
					thread.LastReply = header.MsgNum
					thread.ReplyCount++
				} else {
					threads[header.ReplyTo] = &msgbase.ThreadIndex{
						MsgNum:     header.ReplyTo,
						FirstReply: header.MsgNum,
						LastReply:  header.MsgNum,
						ReplyCount: 1,
					}
				}
			}
		}

		offset += uint32(unsafe.Sizeof(header))
	}

	// Write thread index
	for _, thread := range threads {
		if err := binary.Write(threadFile, binary.LittleEndian, *thread); err != nil {
			return fmt.Errorf("failed to write thread entry: %w", err)
		}
	}

	return nil
}

func (mbm *MessageBaseMaintenance) repairArea(areaNum uint16) *MaintenanceResult {
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	areaPath := filepath.Join(mbm.basePath, fmt.Sprintf("AREA%04d", areaNum))

	// Check if area directory exists
	if _, err := os.Stat(areaPath); os.IsNotExist(err) {
		result.Success = false
		result.ErrorsFound++
		result.Details = append(result.Details, "Area directory does not exist")
		return result
	}

	// Check required files
	requiredFiles := []string{
		msgbase.MessageHeaderFile,
		msgbase.MessageDataFile,
		msgbase.MessageIndexFile,
		msgbase.ThreadIndexFile,
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

	// Verify header/data file consistency
	if err := mbm.verifyHeaderDataConsistency(areaNum, result); err != nil {
		result.Success = false
		result.ErrorsFound++
		result.Details = append(result.Details, "Header/data consistency error: "+err.Error())
	}

	return result
}

func (mbm *MessageBaseMaintenance) verifyAreaIntegrity(areaNum uint16) *MaintenanceResult {
	result := &MaintenanceResult{
		Success: true,
		Details: make([]string, 0),
	}

	areaPath := filepath.Join(mbm.basePath, fmt.Sprintf("AREA%04d", areaNum))
	
	// Verify header file
	headerPath := filepath.Join(areaPath, msgbase.MessageHeaderFile)
	headerInfo, err := os.Stat(headerPath)
	if err != nil {
		result.Success = false
		result.ErrorsFound++
		result.Details = append(result.Details, "Header file error: "+err.Error())
		return result
	}

	// Check if header file size is multiple of header size
	headerSize := unsafe.Sizeof(msgbase.MessageHeader{})
	if headerInfo.Size()%int64(headerSize) != 0 {
		result.Success = false
		result.ErrorsFound++
		result.Details = append(result.Details, "Header file size is not aligned to header record size")
	}

	expectedRecords := uint32(headerInfo.Size() / int64(headerSize))
	result.MessagesProcessed = expectedRecords

	// Verify data file consistency
	if err := mbm.verifyDataFileIntegrity(areaNum, result); err != nil {
		result.Success = false
		result.ErrorsFound++
		result.Details = append(result.Details, "Data file integrity error: "+err.Error())
	}

	// Verify index file consistency
	if err := mbm.verifyIndexFileIntegrity(areaNum, result); err != nil {
		result.Success = false
		result.ErrorsFound++
		result.Details = append(result.Details, "Index file integrity error: "+err.Error())
	}

	if result.ErrorsFound == 0 {
		result.Details = append(result.Details, "Area integrity verification passed")
	}

	return result
}

func (mbm *MessageBaseMaintenance) verifyHeaderDataConsistency(areaNum uint16, result *MaintenanceResult) error {
	areaPath := filepath.Join(mbm.basePath, fmt.Sprintf("AREA%04d", areaNum))
	
	headerPath := filepath.Join(areaPath, msgbase.MessageHeaderFile)
	dataPath := filepath.Join(areaPath, msgbase.MessageDataFile)

	headerFile, err := os.Open(headerPath)
	if err != nil {
		return err
	}
	defer headerFile.Close()

	dataInfo, err := os.Stat(dataPath)
	if err != nil {
		return err
	}

	expectedDataSize := uint64(0)
	msgCount := uint32(0)

	for {
		var header msgbase.MessageHeader
		if err := binary.Read(headerFile, binary.LittleEndian, &header); err != nil {
			break // EOF
		}

		if (header.Status & msgbase.MsgStatusActive) != 0 {
			expectedDataSize += uint64(header.NumBlocks) * msgbase.MessageBlockSize
			msgCount++
		}
	}

	if uint64(dataInfo.Size()) != expectedDataSize {
		result.ErrorsFound++
		result.Details = append(result.Details, 
			fmt.Sprintf("Data file size mismatch: expected %d, actual %d", 
				expectedDataSize, dataInfo.Size()))
	}

	result.MessagesProcessed = msgCount
	return nil
}

func (mbm *MessageBaseMaintenance) verifyDataFileIntegrity(areaNum uint16, result *MaintenanceResult) error {
	areaPath := filepath.Join(mbm.basePath, fmt.Sprintf("AREA%04d", areaNum))
	dataPath := filepath.Join(areaPath, msgbase.MessageDataFile)

	dataInfo, err := os.Stat(dataPath)
	if err != nil {
		return err
	}

	// Data file size should be multiple of block size
	if dataInfo.Size()%msgbase.MessageBlockSize != 0 {
		result.ErrorsFound++
		result.Details = append(result.Details, 
			"Data file size is not aligned to block size")
	}

	return nil
}

func (mbm *MessageBaseMaintenance) verifyIndexFileIntegrity(areaNum uint16, result *MaintenanceResult) error {
	areaPath := filepath.Join(mbm.basePath, fmt.Sprintf("AREA%04d", areaNum))
	indexPath := filepath.Join(areaPath, msgbase.MessageIndexFile)

	indexInfo, err := os.Stat(indexPath)
	if err != nil {
		return err
	}

	// Index file size should be multiple of index entry size
	indexSize := unsafe.Sizeof(msgbase.MessageIndex{})
	if indexInfo.Size()%int64(indexSize) != 0 {
		result.ErrorsFound++
		result.Details = append(result.Details, 
			"Index file size is not aligned to index entry size")
	}

	return nil
}

func (mbm *MessageBaseMaintenance) purgeAreaMessages(areaNum uint16, cutoffDate time.Time) (uint32, error) {
	// Acquire write lock
	if err := mbm.manager.AcquireLock(msgbase.LockTypeWrite, areaNum, 30*time.Second); err != nil {
		return 0, err
	}
	defer mbm.manager.ReleaseLock(msgbase.LockTypeWrite, areaNum)

	areaPath := filepath.Join(mbm.basePath, fmt.Sprintf("AREA%04d", areaNum))
	headerPath := filepath.Join(areaPath, msgbase.MessageHeaderFile)

	// Read all headers and mark old ones for deletion
	headerFile, err := os.OpenFile(headerPath, os.O_RDWR, 0644)
	if err != nil {
		return 0, err
	}
	defer headerFile.Close()

	var purgedCount uint32
	offset := int64(0)

	for {
		var header msgbase.MessageHeader
		currentOffset := offset

		if err := binary.Read(headerFile, binary.LittleEndian, &header); err != nil {
			break // EOF
		}

		if (header.Status & msgbase.MsgStatusActive) != 0 {
			// Parse message date (MM-DD-YY format)
			dateStr := strings.TrimSpace(string(header.Date[:]))
			if msgDate, err := time.Parse("01-02-06", dateStr); err == nil {
				if msgDate.Before(cutoffDate) {
					// Mark as deleted
					header.Status &= ^uint8(msgbase.MsgStatusActive)
					
					if _, err := headerFile.Seek(currentOffset, 0); err != nil {
						return purgedCount, err
					}
					
					if err := binary.Write(headerFile, binary.LittleEndian, header); err != nil {
						return purgedCount, err
					}
					
					purgedCount++
				}
			}
		}

		offset += int64(unsafe.Sizeof(header))
	}

	return purgedCount, nil
}

func (mbm *MessageBaseMaintenance) backupArea(areaNum uint16, backupDir string) error {
	areaPath := filepath.Join(mbm.basePath, fmt.Sprintf("AREA%04d", areaNum))
	backupAreaPath := filepath.Join(backupDir, fmt.Sprintf("AREA%04d", areaNum))

	// Create backup area directory
	if err := os.MkdirAll(backupAreaPath, 0755); err != nil {
		return err
	}

	// Files to backup
	filesToBackup := []string{
		msgbase.MessageHeaderFile,
		msgbase.MessageDataFile,
		msgbase.MessageIndexFile,
		msgbase.ThreadIndexFile,
	}

	for _, filename := range filesToBackup {
		srcPath := filepath.Join(areaPath, filename)
		dstPath := filepath.Join(backupAreaPath, filename)
		
		if err := mbm.copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to backup %s: %w", filename, err)
		}
	}

	return nil
}

func (mbm *MessageBaseMaintenance) copyFile(src, dst string) error {
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