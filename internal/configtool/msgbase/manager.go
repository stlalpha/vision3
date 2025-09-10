package msgbase

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var (
	ErrMessageNotFound    = errors.New("message not found")
	ErrAreaNotFound       = errors.New("message area not found")
	ErrLockTimeout        = errors.New("lock timeout")
	ErrNodeLocked         = errors.New("area locked by another node")
	ErrInvalidMessage     = errors.New("invalid message format")
	ErrMessageBaseFull    = errors.New("message base full")
	ErrPermissionDenied   = errors.New("permission denied")
)

// MessageBaseManager handles binary message base operations
type MessageBaseManager struct {
	BasePath    string
	NodeNumber  uint8
	lockMutex   sync.RWMutex
	locks       map[string]*os.File
	semaphores  map[string]*os.File
}

// NewMessageBaseManager creates a new message base manager
func NewMessageBaseManager(basePath string, nodeNumber uint8) *MessageBaseManager {
	return &MessageBaseManager{
		BasePath:   basePath,
		NodeNumber: nodeNumber,
		locks:      make(map[string]*os.File),
		semaphores: make(map[string]*os.File),
	}
}

// Initialize creates the message base directory structure and files
func (mbm *MessageBaseManager) Initialize() error {
	// Create base directory
	if err := os.MkdirAll(mbm.BasePath, 0755); err != nil {
		return fmt.Errorf("failed to create message base directory: %w", err)
	}

	// Initialize area configuration file if it doesn't exist
	areaPath := filepath.Join(mbm.BasePath, AreaConfigFile)
	if _, err := os.Stat(areaPath); os.IsNotExist(err) {
		file, err := os.Create(areaPath)
		if err != nil {
			return fmt.Errorf("failed to create area config file: %w", err)
		}
		file.Close()
	}

	// Initialize statistics file
	statsPath := filepath.Join(mbm.BasePath, StatsFile)
	if _, err := os.Stat(statsPath); os.IsNotExist(err) {
		stats := MessageBaseStats{
			TotalAreas:  0,
			TotalMsgs:   0,
			TotalKBytes: 0,
		}
		copy(stats.LastPacked[:], time.Now().Format("2006-01-02 15:04:05"))
		copy(stats.LastMaint[:], time.Now().Format("2006-01-02 15:04:05"))
		
		if err := mbm.writeStatsFile(stats); err != nil {
			return fmt.Errorf("failed to initialize stats file: %w", err)
		}
	}

	return nil
}

// AcquireLock acquires a multi-node lock for safe operations
func (mbm *MessageBaseManager) AcquireLock(lockType uint8, areaNum uint16, timeout time.Duration) error {
	mbm.lockMutex.Lock()
	defer mbm.lockMutex.Unlock()

	lockPath := filepath.Join(mbm.BasePath, LockFile)
	lockKey := fmt.Sprintf("%d-%d", lockType, areaNum)

	// Check if we already have this lock
	if _, exists := mbm.locks[lockKey]; exists {
		return nil // Already locked
	}

	start := time.Now()
	for {
		// Try to acquire exclusive lock
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			// Lock acquired, write lock record
			lock := NodeLock{
				NodeNum:  mbm.NodeNumber,
				LockType: lockType,
				AreaNum:  areaNum,
			}
			copy(lock.Timestamp[:], time.Now().Format("2006-01-02 15:04:05"))
			copy(lock.Process[:], "MSGBASE")

			if err := binary.Write(file, binary.LittleEndian, lock); err != nil {
				file.Close()
				os.Remove(lockPath)
				return fmt.Errorf("failed to write lock record: %w", err)
			}

			mbm.locks[lockKey] = file
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
func (mbm *MessageBaseManager) ReleaseLock(lockType uint8, areaNum uint16) error {
	mbm.lockMutex.Lock()
	defer mbm.lockMutex.Unlock()

	lockKey := fmt.Sprintf("%d-%d", lockType, areaNum)
	file, exists := mbm.locks[lockKey]
	if !exists {
		return nil // Lock not held
	}

	file.Close()
	delete(mbm.locks, lockKey)

	lockPath := filepath.Join(mbm.BasePath, LockFile)
	return os.Remove(lockPath)
}

// CreateMessageArea creates a new message area
func (mbm *MessageBaseManager) CreateMessageArea(config MessageAreaConfig) error {
	if err := mbm.AcquireLock(LockTypeWrite, 0, 5*time.Second); err != nil {
		return err
	}
	defer mbm.ReleaseLock(LockTypeWrite, 0)

	// Read existing areas to check for duplicates
	areas, err := mbm.GetMessageAreas()
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

	// Create area directory
	areaPath := filepath.Join(mbm.BasePath, fmt.Sprintf("AREA%04d", config.AreaNum))
	if err := os.MkdirAll(areaPath, 0755); err != nil {
		return fmt.Errorf("failed to create area directory: %w", err)
	}

	// Initialize area files
	headerPath := filepath.Join(areaPath, MessageHeaderFile)
	dataPath := filepath.Join(areaPath, MessageDataFile)
	indexPath := filepath.Join(areaPath, MessageIndexFile)
	threadPath := filepath.Join(areaPath, ThreadIndexFile)

	for _, path := range []string{headerPath, dataPath, indexPath, threadPath} {
		file, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("failed to create area file %s: %w", path, err)
		}
		file.Close()
	}

	// Add area to configuration
	configPath := filepath.Join(mbm.BasePath, AreaConfigFile)
	file, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open area config file: %w", err)
	}
	defer file.Close()

	if err := binary.Write(file, binary.LittleEndian, config); err != nil {
		return fmt.Errorf("failed to write area config: %w", err)
	}

	// Update statistics
	stats, err := mbm.GetStats()
	if err == nil {
		stats.TotalAreas++
		mbm.writeStatsFile(stats)
	}

	return nil
}

// GetMessageAreas returns all configured message areas
func (mbm *MessageBaseManager) GetMessageAreas() ([]MessageAreaConfig, error) {
	configPath := filepath.Join(mbm.BasePath, AreaConfigFile)
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var areas []MessageAreaConfig
	for {
		var config MessageAreaConfig
		if err := binary.Read(file, binary.LittleEndian, &config); err != nil {
			break // EOF or error
		}
		areas = append(areas, config)
	}

	return areas, nil
}

// PostMessage posts a new message to an area
func (mbm *MessageBaseManager) PostMessage(areaNum uint16, header MessageHeader, body string) (uint32, error) {
	if err := mbm.AcquireLock(LockTypeWrite, areaNum, 5*time.Second); err != nil {
		return 0, err
	}
	defer mbm.ReleaseLock(LockTypeWrite, areaNum)

	// Get area configuration
	areas, err := mbm.GetMessageAreas()
	if err != nil {
		return 0, err
	}

	var area *MessageAreaConfig
	for i := range areas {
		if areas[i].AreaNum == areaNum {
			area = &areas[i]
			break
		}
	}
	if area == nil {
		return 0, ErrAreaNotFound
	}

	areaPath := filepath.Join(mbm.BasePath, fmt.Sprintf("AREA%04d", areaNum))

	// Get next message number
	nextMsgNum := area.HighMsgNum + 1

	// Prepare message header
	header.MsgNum = nextMsgNum
	header.Status |= MsgStatusActive

	// Set date/time
	now := time.Now()
	copy(header.Date[:], now.Format("01-02-06"))
	copy(header.Time[:], now.Format("15:04:05"))

	// Calculate message body blocks
	bodyBytes := []byte(body)
	numBlocks := uint16((len(bodyBytes) + MessageBlockSize - 1) / MessageBlockSize)
	header.NumBlocks = numBlocks

	// Write header
	headerPath := filepath.Join(areaPath, MessageHeaderFile)
	headerFile, err := os.OpenFile(headerPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open header file: %w", err)
	}
	defer headerFile.Close()

	headerOffset, err := headerFile.Seek(0, 2) // Seek to end
	if err != nil {
		return 0, err
	}

	if err := binary.Write(headerFile, binary.LittleEndian, header); err != nil {
		return 0, fmt.Errorf("failed to write header: %w", err)
	}

	// Write message body
	dataPath := filepath.Join(areaPath, MessageDataFile)
	dataFile, err := os.OpenFile(dataPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open data file: %w", err)
	}
	defer dataFile.Close()

	dataOffset, err := dataFile.Seek(0, 2) // Seek to end
	if err != nil {
		return 0, err
	}

	// Write body in 128-byte blocks
	for i := 0; i < int(numBlocks); i++ {
		block := make([]byte, MessageBlockSize)
		start := i * MessageBlockSize
		end := start + MessageBlockSize
		if end > len(bodyBytes) {
			end = len(bodyBytes)
		}
		copy(block, bodyBytes[start:end])
		
		if err := binary.Write(dataFile, binary.LittleEndian, block); err != nil {
			return 0, fmt.Errorf("failed to write message body block: %w", err)
		}
	}

	// Create message index entry
	index := MessageIndex{
		MsgNum: nextMsgNum,
		Offset: uint32(headerOffset),
		Length: uint32(unsafe.Sizeof(header)) + uint32(numBlocks*MessageBlockSize),
		Hash:   crc32.ChecksumIEEE([]byte(strings.ToUpper(strings.TrimSpace(string(header.Subject[:]))))),
		Status: header.Status,
	}

	// Write index entry
	indexPath := filepath.Join(areaPath, MessageIndexFile)
	indexFile, err := os.OpenFile(indexPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open index file: %w", err)
	}
	defer indexFile.Close()

	if err := binary.Write(indexFile, binary.LittleEndian, index); err != nil {
		return 0, fmt.Errorf("failed to write index entry: %w", err)
	}

	// Update thread index if this is a reply
	if header.ReplyTo > 0 {
		if err := mbm.updateThreadIndex(areaPath, header.ReplyTo, nextMsgNum); err != nil {
			// Log error but don't fail the post
			fmt.Printf("Warning: failed to update thread index: %v\n", err)
		}
	}

	// Update area configuration
	area.HighMsgNum = nextMsgNum
	area.TotalMsgs++
	if err := mbm.updateAreaConfig(*area); err != nil {
		// Log error but don't fail the post
		fmt.Printf("Warning: failed to update area config: %v\n", err)
	}

	// Update statistics
	stats, err := mbm.GetStats()
	if err == nil {
		stats.TotalMsgs++
		stats.TotalKBytes = uint32(dataOffset+int64(numBlocks*MessageBlockSize)) / 1024
		mbm.writeStatsFile(stats)
	}

	return nextMsgNum, nil
}

// GetMessage retrieves a message by number from an area
func (mbm *MessageBaseManager) GetMessage(areaNum uint16, msgNum uint32) (*MessageHeader, string, error) {
	if err := mbm.AcquireLock(LockTypeRead, areaNum, 5*time.Second); err != nil {
		return nil, "", err
	}
	defer mbm.ReleaseLock(LockTypeRead, areaNum)

	areaPath := filepath.Join(mbm.BasePath, fmt.Sprintf("AREA%04d", areaNum))

	// Find message in index
	indexPath := filepath.Join(areaPath, MessageIndexFile)
	indexFile, err := os.Open(indexPath)
	if err != nil {
		return nil, "", err
	}
	defer indexFile.Close()

	var foundIndex *MessageIndex
	for {
		var index MessageIndex
		if err := binary.Read(indexFile, binary.LittleEndian, &index); err != nil {
			break // EOF
		}
		if index.MsgNum == msgNum && (index.Status&MsgStatusActive) != 0 {
			foundIndex = &index
			break
		}
	}

	if foundIndex == nil {
		return nil, "", ErrMessageNotFound
	}

	// Read message header
	headerPath := filepath.Join(areaPath, MessageHeaderFile)
	headerFile, err := os.Open(headerPath)
	if err != nil {
		return nil, "", err
	}
	defer headerFile.Close()

	if _, err := headerFile.Seek(int64(foundIndex.Offset), 0); err != nil {
		return nil, "", err
	}

	var header MessageHeader
	if err := binary.Read(headerFile, binary.LittleEndian, &header); err != nil {
		return nil, "", err
	}

	// Read message body
	dataPath := filepath.Join(areaPath, MessageDataFile)
	dataFile, err := os.Open(dataPath)
	if err != nil {
		return nil, "", err
	}
	defer dataFile.Close()

	// Calculate data offset (after all headers up to this message)
	dataOffset := int64(foundIndex.Offset) + int64(unsafe.Sizeof(header))
	if _, err := dataFile.Seek(dataOffset, 0); err != nil {
		return nil, "", err
	}

	// Read message blocks
	bodyBytes := make([]byte, int(header.NumBlocks)*MessageBlockSize)
	if err := binary.Read(dataFile, binary.LittleEndian, bodyBytes); err != nil {
		return nil, "", err
	}

	// Remove null padding and convert to string
	body := string(bodyBytes)
	body = strings.TrimRight(body, "\x00")

	return &header, body, nil
}

// GetStats returns message base statistics
func (mbm *MessageBaseManager) GetStats() (MessageBaseStats, error) {
	statsPath := filepath.Join(mbm.BasePath, StatsFile)
	file, err := os.Open(statsPath)
	if err != nil {
		return MessageBaseStats{}, err
	}
	defer file.Close()

	var stats MessageBaseStats
	if err := binary.Read(file, binary.LittleEndian, &stats); err != nil {
		return MessageBaseStats{}, err
	}

	return stats, nil
}

// PackMessages removes deleted messages and compacts the database
func (mbm *MessageBaseManager) PackMessages(areaNum uint16) error {
	if err := mbm.AcquireLock(LockTypePack, areaNum, 30*time.Second); err != nil {
		return err
	}
	defer mbm.ReleaseLock(LockTypePack, areaNum)

	areaPath := filepath.Join(mbm.BasePath, fmt.Sprintf("AREA%04d", areaNum))

	// Read all active messages
	var activeHeaders []MessageHeader
	var activeBodies []string
	var activeIndexes []MessageIndex

	// Read existing messages
	headerPath := filepath.Join(areaPath, MessageHeaderFile)
	headerFile, err := os.Open(headerPath)
	if err != nil {
		return err
	}
	defer headerFile.Close()

	msgNum := uint32(1)
	for {
		var header MessageHeader
		if err := binary.Read(headerFile, binary.LittleEndian, &header); err != nil {
			break // EOF
		}

		if (header.Status & MsgStatusActive) != 0 {
			// Get message body
			_, body, err := mbm.GetMessage(areaNum, header.MsgNum)
			if err != nil {
				continue // Skip corrupted messages
			}

			// Update message number for compacted sequence
			header.MsgNum = msgNum
			activeHeaders = append(activeHeaders, header)
			activeBodies = append(activeBodies, body)

			// Create new index entry
			index := MessageIndex{
				MsgNum: msgNum,
				Offset: uint32(len(activeHeaders)-1) * uint32(unsafe.Sizeof(MessageHeader{})),
				Status: header.Status,
				Hash:   crc32.ChecksumIEEE([]byte(strings.ToUpper(strings.TrimSpace(string(header.Subject[:]))))),
			}
			activeIndexes = append(activeIndexes, index)

			msgNum++
		}
	}

	// Write compacted files
	// Backup original files first
	backupSuffix := fmt.Sprintf(".bak.%d", time.Now().Unix())
	os.Rename(headerPath, headerPath+backupSuffix)
	os.Rename(filepath.Join(areaPath, MessageDataFile), filepath.Join(areaPath, MessageDataFile)+backupSuffix)
	os.Rename(filepath.Join(areaPath, MessageIndexFile), filepath.Join(areaPath, MessageIndexFile)+backupSuffix)

	// Write new header file
	newHeaderFile, err := os.Create(headerPath)
	if err != nil {
		return err
	}
	defer newHeaderFile.Close()

	// Write new data file
	newDataFile, err := os.Create(filepath.Join(areaPath, MessageDataFile))
	if err != nil {
		return err
	}
	defer newDataFile.Close()

	// Write new index file
	newIndexFile, err := os.Create(filepath.Join(areaPath, MessageIndexFile))
	if err != nil {
		return err
	}
	defer newIndexFile.Close()

	// Write all active messages
	for i, header := range activeHeaders {
		if err := binary.Write(newHeaderFile, binary.LittleEndian, header); err != nil {
			return err
		}

		// Write body in blocks
		body := activeBodies[i]
		bodyBytes := []byte(body)
		numBlocks := (len(bodyBytes) + MessageBlockSize - 1) / MessageBlockSize

		for j := 0; j < numBlocks; j++ {
			block := make([]byte, MessageBlockSize)
			start := j * MessageBlockSize
			end := start + MessageBlockSize
			if end > len(bodyBytes) {
				end = len(bodyBytes)
			}
			copy(block, bodyBytes[start:end])

			if err := binary.Write(newDataFile, binary.LittleEndian, block); err != nil {
				return err
			}
		}

		// Update index with correct length
		activeIndexes[i].Length = uint32(unsafe.Sizeof(header)) + uint32(numBlocks*MessageBlockSize)
		if err := binary.Write(newIndexFile, binary.LittleEndian, activeIndexes[i]); err != nil {
			return err
		}
	}

	// Update area configuration
	areas, err := mbm.GetMessageAreas()
	if err != nil {
		return err
	}

	for i := range areas {
		if areas[i].AreaNum == areaNum {
			areas[i].HighMsgNum = uint32(len(activeHeaders))
			areas[i].TotalMsgs = uint32(len(activeHeaders))
			if err := mbm.updateAreaConfig(areas[i]); err != nil {
				return err
			}
			break
		}
	}

	// Update statistics
	stats, err := mbm.GetStats()
	if err == nil {
		stats.PackedMsgs = uint32(msgNum) - uint32(len(activeHeaders))
		copy(stats.LastPacked[:], time.Now().Format("2006-01-02 15:04:05"))
		mbm.writeStatsFile(stats)
	}

	return nil
}

// Helper functions

func (mbm *MessageBaseManager) writeStatsFile(stats MessageBaseStats) error {
	statsPath := filepath.Join(mbm.BasePath, StatsFile)
	file, err := os.Create(statsPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return binary.Write(file, binary.LittleEndian, stats)
}

func (mbm *MessageBaseManager) updateAreaConfig(config MessageAreaConfig) error {
	// This is a simplified implementation - in practice, you'd want to 
	// read all configs, update the specific one, and rewrite the file
	return nil
}

func (mbm *MessageBaseManager) updateThreadIndex(areaPath string, replyTo, newMsgNum uint32) error {
	threadPath := filepath.Join(areaPath, ThreadIndexFile)
	
	// Read existing thread indexes
	var threads []ThreadIndex
	if file, err := os.Open(threadPath); err == nil {
		defer file.Close()
		for {
			var thread ThreadIndex
			if err := binary.Read(file, binary.LittleEndian, &thread); err != nil {
				break
			}
			threads = append(threads, thread)
		}
	}

	// Find or create thread entry
	found := false
	for i := range threads {
		if threads[i].MsgNum == replyTo {
			threads[i].LastReply = newMsgNum
			threads[i].ReplyCount++
			found = true
			break
		}
	}

	if !found {
		thread := ThreadIndex{
			MsgNum:     replyTo,
			FirstReply: newMsgNum,
			LastReply:  newMsgNum,
			ReplyCount: 1,
		}
		threads = append(threads, thread)
	}

	// Rewrite thread index file
	file, err := os.Create(threadPath)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, thread := range threads {
		if err := binary.Write(file, binary.LittleEndian, thread); err != nil {
			return err
		}
	}

	return nil
}

// GetNextMessageNumber returns the next available message number for an area
func (mbm *MessageBaseManager) GetNextMessageNumber(areaNum uint16) (uint32, error) {
	areas, err := mbm.GetMessageAreas()
	if err != nil {
		return 0, err
	}

	for _, area := range areas {
		if area.AreaNum == areaNum {
			return area.HighMsgNum + 1, nil
		}
	}

	return 0, ErrAreaNotFound
}

// DeleteMessage marks a message as deleted (classic BBS style - mark don't remove)
func (mbm *MessageBaseManager) DeleteMessage(areaNum uint16, msgNum uint32) error {
	if err := mbm.AcquireLock(LockTypeWrite, areaNum, 5*time.Second); err != nil {
		return err
	}
	defer mbm.ReleaseLock(LockTypeWrite, areaNum)

	areaPath := filepath.Join(mbm.BasePath, fmt.Sprintf("AREA%04d", areaNum))

	// Find message in index and get its offset
	indexPath := filepath.Join(areaPath, MessageIndexFile)
	indexFile, err := os.OpenFile(indexPath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer indexFile.Close()

	var headerOffset int64 = -1
	var indexOffset int64 = 0

	for {
		var index MessageIndex
		currentOffset := indexOffset
		if err := binary.Read(indexFile, binary.LittleEndian, &index); err != nil {
			break // EOF
		}

		if index.MsgNum == msgNum {
			headerOffset = int64(index.Offset)
			// Mark as deleted in index
			index.Status &= ^MsgStatusActive
			indexFile.Seek(currentOffset, 0)
			binary.Write(indexFile, binary.LittleEndian, index)
			break
		}
		indexOffset += int64(unsafe.Sizeof(MessageIndex{}))
	}

	if headerOffset == -1 {
		return ErrMessageNotFound
	}

	// Mark as deleted in header
	headerPath := filepath.Join(areaPath, MessageHeaderFile)
	headerFile, err := os.OpenFile(headerPath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer headerFile.Close()

	headerFile.Seek(headerOffset, 0)
	var header MessageHeader
	if err := binary.Read(headerFile, binary.LittleEndian, &header); err != nil {
		return err
	}

	header.Status &= ^MsgStatusActive
	headerFile.Seek(headerOffset, 0)
	return binary.Write(headerFile, binary.LittleEndian, header)
}

// GetMessageList returns a list of message headers for an area
func (mbm *MessageBaseManager) GetMessageList(areaNum uint16, startNum, count uint32) ([]MessageHeader, error) {
	if err := mbm.AcquireLock(LockTypeRead, areaNum, 5*time.Second); err != nil {
		return nil, err
	}
	defer mbm.ReleaseLock(LockTypeRead, areaNum)

	areaPath := filepath.Join(mbm.BasePath, fmt.Sprintf("AREA%04d", areaNum))
	headerPath := filepath.Join(areaPath, MessageHeaderFile)

	file, err := os.Open(headerPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var headers []MessageHeader
	current := uint32(1)

	for {
		var header MessageHeader
		if err := binary.Read(file, binary.LittleEndian, &header); err != nil {
			break // EOF
		}

		if (header.Status&MsgStatusActive) != 0 && current >= startNum {
			headers = append(headers, header)
			if uint32(len(headers)) >= count {
				break
			}
		}
		current++
	}

	return headers, nil
}

// SearchMessages searches for messages containing specific text
func (mbm *MessageBaseManager) SearchMessages(areaNum uint16, searchText string, searchFields []string) ([]uint32, error) {
	if err := mbm.AcquireLock(LockTypeRead, areaNum, 5*time.Second); err != nil {
		return nil, err
	}
	defer mbm.ReleaseLock(LockTypeRead, areaNum)

	var matches []uint32
	searchText = strings.ToLower(searchText)

	// Get all messages in area
	headers, err := mbm.GetMessageList(areaNum, 1, 999999)
	if err != nil {
		return nil, err
	}

	for _, header := range headers {
		found := false

		// Search in specified fields
		for _, field := range searchFields {
			var searchIn string
			switch strings.ToLower(field) {
			case "subject":
				searchIn = strings.ToLower(strings.TrimSpace(string(header.Subject[:])))
			case "from":
				searchIn = strings.ToLower(strings.TrimSpace(string(header.FromUser[:])))
			case "to":
				searchIn = strings.ToLower(strings.TrimSpace(string(header.ToUser[:])))
			case "body":
				_, body, err := mbm.GetMessage(areaNum, header.MsgNum)
				if err != nil {
					continue
				}
				searchIn = strings.ToLower(body)
			}

			if strings.Contains(searchIn, searchText) {
				found = true
				break
			}
		}

		if found {
			matches = append(matches, header.MsgNum)
		}
	}

	return matches, nil
}

// Close cleans up any open locks and files
func (mbm *MessageBaseManager) Close() error {
	mbm.lockMutex.Lock()
	defer mbm.lockMutex.Unlock()

	// Release all locks
	for key, file := range mbm.locks {
		file.Close()
		delete(mbm.locks, key)
	}

	// Clean up lock files
	lockPath := filepath.Join(mbm.BasePath, LockFile)
	os.Remove(lockPath)

	return nil
}