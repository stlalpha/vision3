package message

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/stlalpha/vision3/internal/jam"
)

// ErrAreaNotFound is returned when a message area doesn't exist.
var ErrAreaNotFound = errors.New("message area not found")

// Error Handling Design:
// - Read operations (Get*Count, GetLastRead, GetNextUnread, etc.) treat missing
//   areas as empty (return 0, nil) to avoid failing when areas are referenced
//   but not yet configured. This allows graceful degradation.
// - Write operations (AddMessage, SetLastRead) and direct base access (GetBase,
//   GetMessage) return ErrAreaNotFound to ensure callers are aware the area
//   doesn't exist before attempting modifications.
// - All operations propagate I/O errors (not ErrAreaNotFound) so real failures
//   are never masked.

type threadIndex struct {
	total      int
	modCounter uint32
	counts     map[string]int
}

// msgidIndex maps MSGIDs to 1-based message numbers for fast reply lookups.
type msgidIndex struct {
	total      int
	modCounter uint32
	msgIDs     map[string]int // MSGID string -> 1-based message number
}

const messageAreaFile = "message_areas.json"

// MessageManager handles message areas backed by JAM message bases.
// Bases are opened on-demand and closed after each operation to allow
// external tossers (like HPT) concurrent access.
type MessageManager struct {
	mu         sync.RWMutex
	dataPath   string // Base data directory (e.g., "data")
	areasPath  string // Full path to message_areas.json
	areasByID  map[int]*MessageArea
	areasByTag map[string]*MessageArea
	boardName  string // BBS name for echomail origin lines
	// networkTearlines maps network key -> custom tearline text.
	networkTearlines map[string]string
	threadIndex      map[int]*threadIndex
	msgidIndex       map[int]*msgidIndex
}

// NewMessageManager creates and initializes a new MessageManager.
// dataPath is the directory where JAM base files are stored.
// configPath is the directory containing message_areas.json.
// boardName is the BBS name used in echomail origin lines.
// networkTearlines maps network name -> custom tearline text for echomail.
func NewMessageManager(dataPath, configPath, boardName string, networkTearlines map[string]string) (*MessageManager, error) {
	mm := &MessageManager{
		dataPath:         dataPath,
		areasPath:        filepath.Join(configPath, messageAreaFile),
		areasByID:        make(map[int]*MessageArea),
		areasByTag:       make(map[string]*MessageArea),
		boardName:        boardName,
		networkTearlines: normalizeNetworkTearlines(networkTearlines),
		threadIndex:      make(map[int]*threadIndex),
		msgidIndex:       make(map[int]*msgidIndex),
	}

	if err := mm.loadMessageAreas(); err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: %s not found. Starting with no message areas.", messageAreaFile)
		} else {
			return nil, fmt.Errorf("failed to load message areas: %w", err)
		}
	}

	log.Printf("INFO: MessageManager initialized. Loaded %d areas.", len(mm.areasByID))
	log.Printf("INFO: JAM bases will be opened on-demand for external tosser compatibility.")
	return mm, nil
}

func normalizeNetworkTearlines(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		key := strings.ToLower(strings.TrimSpace(k))
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (mm *MessageManager) tearlineForNetwork(network string) string {
	if mm.networkTearlines == nil {
		return ""
	}
	key := strings.ToLower(strings.TrimSpace(network))
	if key == "" {
		return ""
	}
	return mm.networkTearlines[key]
}

// Close is a no-op now that bases are opened on-demand.
// Kept for API compatibility.
func (mm *MessageManager) Close() error {
	return nil
}

// loadMessageAreas loads area definitions from JSON.
func (mm *MessageManager) loadMessageAreas() error {
	data, err := os.ReadFile(mm.areasPath)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	var areasList []*MessageArea
	if err := json.Unmarshal(data, &areasList); err != nil {
		return fmt.Errorf("failed to unmarshal areas from %s: %w", mm.areasPath, err)
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	mm.areasByID = make(map[int]*MessageArea)
	mm.areasByTag = make(map[string]*MessageArea)

	for _, area := range areasList {
		if area == nil {
			continue
		}
		if _, exists := mm.areasByID[area.ID]; exists {
			log.Printf("WARN: Duplicate area ID %d, skipping.", area.ID)
			continue
		}
		mm.areasByID[area.ID] = area
		mm.areasByTag[area.Tag] = area
		log.Printf("TRACE: Loaded area ID %d, Tag '%s', Type '%s'", area.ID, area.Tag, area.AreaType)
	}
	return nil
}

// openBase opens a JAM base on-demand. The caller must close it when done.
// This method does not hold any locks and should be called after releasing mm.mu.
// Returns ErrAreaNotFound if the area doesn't exist.
func (mm *MessageManager) openBase(areaID int) (*jam.Base, *MessageArea, error) {
	mm.mu.RLock()
	area, exists := mm.areasByID[areaID]
	mm.mu.RUnlock()

	if !exists {
		return nil, nil, ErrAreaNotFound
	}

	basePath := mm.resolveBasePath(area)
	b, err := jam.Open(basePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open JAM base for area %d: %w", areaID, err)
	}

	return b, area, nil
}

// resolveBasePath returns the absolute path for a JAM base.
func (mm *MessageManager) resolveBasePath(area *MessageArea) string {
	bp := area.BasePath
	if bp == "" {
		bp = "msgbases/" + strings.ToLower(area.Tag)
	}
	if filepath.IsAbs(bp) {
		return bp
	}
	return filepath.Join(mm.dataPath, bp)
}

// GetAreaByID retrieves a message area by its ID.
func (mm *MessageManager) GetAreaByID(id int) (*MessageArea, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	area, exists := mm.areasByID[id]
	return area, exists
}

// GetAreaByTag retrieves a message area by its tag.
func (mm *MessageManager) GetAreaByTag(tag string) (*MessageArea, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	area, exists := mm.areasByTag[tag]
	return area, exists
}

// ListAreas returns all loaded areas sorted by ID.
func (mm *MessageManager) ListAreas() []*MessageArea {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	list := make([]*MessageArea, 0, len(mm.areasByID))
	for _, area := range mm.areasByID {
		list = append(list, area)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list
}

// AddMessage creates and writes a new message to the specified area.
// For echomail areas, it automatically handles MSGID, kludges, tearline, and origin.
// Returns the 1-based message number assigned.
func (mm *MessageManager) AddMessage(areaID int, from, to, subject, body, replyToMsgID string) (int, error) {
	b, area, err := mm.openBase(areaID)
	if err != nil {
		return 0, err
	}
	defer b.Close()

	msg := jam.NewMessage()
	msg.From = from
	msg.To = to
	msg.Subject = subject
	msg.Text = body
	msg.DateTime = time.Now()

	if replyToMsgID != "" {
		msg.ReplyID = replyToMsgID
	}

	msgType := jam.DetermineMessageType(area.AreaType, area.EchoTag)

	var msgNum int
	if msgType.IsEchomail() || msgType.IsNetmail() {
		msg.OrigAddr = area.OriginAddr
		msgNum, err = b.WriteMessageExt(msg, msgType, area.EchoTag, mm.boardName, mm.tearlineForNetwork(area.Network))
	} else {
		msgNum, err = b.WriteMessage(msg)
	}

	if err == nil {
		mm.invalidateThreadIndex(areaID)
	}
	return msgNum, err
}

// AddPrivateMessage creates and writes a new private message to the specified area.
// It sets the MSG_PRIVATE flag on the message to indicate it's private user-to-user mail.
// Returns the 1-based message number assigned.
func (mm *MessageManager) AddPrivateMessage(areaID int, from, to, subject, body, replyToMsgID string) (int, error) {
	b, area, err := mm.openBase(areaID)
	if err != nil {
		return 0, err
	}
	defer b.Close()

	msg := jam.NewMessage()
	msg.From = from
	msg.To = to
	msg.Subject = subject
	msg.Text = body
	msg.DateTime = time.Now()

	// Initialize header to set the MSG_PRIVATE flag
	msg.Header = &jam.MessageHeader{
		Attribute: jam.MsgPrivate | jam.MsgLocal,
	}

	if replyToMsgID != "" {
		msg.ReplyID = replyToMsgID
	}

	msgType := jam.DetermineMessageType(area.AreaType, area.EchoTag)

	var msgNum int
	if msgType.IsEchomail() || msgType.IsNetmail() {
		msg.OrigAddr = area.OriginAddr
		msgNum, err = b.WriteMessageExt(msg, msgType, area.EchoTag, mm.boardName, mm.tearlineForNetwork(area.Network))
	} else {
		msgNum, err = b.WriteMessage(msg)
	}

	if err == nil {
		mm.invalidateThreadIndex(areaID)
	}
	return msgNum, err
}

// GetMessage reads a single message by area ID and 1-based message number.
func (mm *MessageManager) GetMessage(areaID, msgNum int) (*DisplayMessage, error) {
	b, _, err := mm.openBase(areaID)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	msg, err := b.ReadMessage(msgNum)
	if err != nil {
		return nil, err
	}

	replyToNum := 0
	if msg.Header != nil && msg.Header.ReplyTo > 0 {
		replyToNum = int(msg.Header.ReplyTo)
	}

	return &DisplayMessage{
		MsgNum:     msgNum,
		From:       msg.From,
		To:         msg.To,
		Subject:    msg.Subject,
		DateTime:   msg.DateTime,
		Body:       normalizeLineEndings(msg.Text),
		MsgID:      msg.MsgID,
		ReplyID:    msg.ReplyID,
		ReplyToNum: replyToNum,
		OrigAddr:   msg.OrigAddr,
		DestAddr:   msg.DestAddr,
		Attributes: msg.GetAttribute(),
		IsPrivate:  msg.IsPrivate(),
		IsDeleted:  msg.IsDeleted(),
		AreaID:     areaID,
	}, nil
}

// GetMessageCountForArea returns the total message count for an area.
func (mm *MessageManager) GetMessageCountForArea(areaID int) (int, error) {
	b, _, err := mm.openBase(areaID)
	if err != nil {
		if errors.Is(err, ErrAreaNotFound) {
			return 0, nil // Return 0 if area doesn't exist
		}
		return 0, err // Propagate I/O and other errors
	}
	defer b.Close()

	return b.GetMessageCount()
}

// GetTotalMessageCount returns the total number of messages across all areas.
func (mm *MessageManager) GetTotalMessageCount() int {
	areas := mm.ListAreas()
	total := 0
	for _, area := range areas {
		count, err := mm.GetMessageCountForArea(area.ID)
		if err != nil {
			continue
		}
		total += count
	}
	return total
}

// GetThreadReplyCount returns the number of other messages in the same thread.
// Threading matches Vision-2/Pascal behavior: subject-based, ignoring "Re:" prefixes
// and " -Re: #N-" suffixes.
func (mm *MessageManager) GetThreadReplyCount(areaID int, msgNum int, subject string) (int, error) {
	b, _, err := mm.openBase(areaID)
	if err != nil {
		if errors.Is(err, ErrAreaNotFound) {
			return 0, nil // Return 0 if area doesn't exist
		}
		return 0, err // Propagate I/O and other errors
	}
	defer b.Close()

	mm.mu.RLock()
	idx := mm.threadIndex[areaID]
	mm.mu.RUnlock()

	total, err := b.GetMessageCount()
	if err != nil {
		return 0, err
	}

	modCounter := uint32(0)
	modCounterErr := false
	if mc, err := b.GetModCounter(); err == nil {
		modCounter = mc
	} else {
		modCounterErr = true
	}

	if idx == nil || idx.total != total || modCounterErr || (modCounter != 0 && idx.modCounter != modCounter) {
		// Acquire write lock so only one goroutine rebuilds the index;
		// others will wait and reuse the result.
		mm.mu.Lock()
		// Re-check after acquiring write lock (another goroutine may have rebuilt it)
		idx = mm.threadIndex[areaID]
		if idx == nil || idx.total != total || modCounterErr || (modCounter != 0 && idx.modCounter != modCounter) {
			idx = mm.buildThreadIndex(b, total, modCounter)
			mm.threadIndex[areaID] = idx
		}
		mm.mu.Unlock()
	}

	key := ThreadKey(subject)
	count := idx.counts[key]
	if count <= 1 {
		return 0, nil
	}
	return count - 1, nil
}

func (mm *MessageManager) buildThreadIndex(b *jam.Base, total int, modCounter uint32) *threadIndex {
	counts := make(map[string]int)
	for i := 1; i <= total; i++ {
		hdr, err := b.ReadMessageHeader(i)
		if err != nil {
			log.Printf("WARN: Failed to read message header %d: %v", i, err)
			continue
		}
		if hdr.Attribute&jam.MsgDeleted != 0 {
			continue
		}
		subject := subjectFromHeader(hdr)
		key := ThreadKey(subject)
		counts[key]++
	}
	return &threadIndex{
		total:      total,
		modCounter: modCounter,
		counts:     counts,
	}
}

func subjectFromHeader(hdr *jam.MessageHeader) string {
	for _, sf := range hdr.Subfields {
		if sf.LoID == jam.SfldSubject {
			return string(sf.Buffer)
		}
	}
	return ""
}

func (mm *MessageManager) invalidateThreadIndex(areaID int) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	delete(mm.threadIndex, areaID)
	delete(mm.msgidIndex, areaID)
}

// FindMessageByMSGID searches for a message in the given area whose MSGID
// matches the supplied value. Returns the 1-based message number, or 0 if
// not found.  Uses a cached index that is rebuilt only when the message
// base has changed (same invalidation strategy as threadIndex).
func (mm *MessageManager) FindMessageByMSGID(areaID int, msgID string) int {
	if msgID == "" {
		return 0
	}

	b, _, err := mm.openBase(areaID)
	if err != nil {
		return 0
	}
	defer b.Close()

	total, err := b.GetMessageCount()
	if err != nil || total == 0 {
		return 0
	}

	modCounter := uint32(0)
	if mc, mcErr := b.GetModCounter(); mcErr == nil {
		modCounter = mc
	}

	// Fast path: check existing index under read lock
	mm.mu.RLock()
	idx := mm.msgidIndex[areaID]
	mm.mu.RUnlock()

	if idx == nil || idx.total != total || (modCounter != 0 && idx.modCounter != modCounter) {
		mm.mu.Lock()
		// Re-check after acquiring write lock
		idx = mm.msgidIndex[areaID]
		if idx == nil || idx.total != total || (modCounter != 0 && idx.modCounter != modCounter) {
			idx = mm.buildMSGIDIndex(b, total, modCounter)
			mm.msgidIndex[areaID] = idx
		}
		mm.mu.Unlock()
	}

	if n, ok := idx.msgIDs[msgID]; ok {
		return n
	}
	return 0
}

// buildMSGIDIndex scans all messages and builds a MSGID -> message number map.
func (mm *MessageManager) buildMSGIDIndex(b *jam.Base, total int, modCounter uint32) *msgidIndex {
	ids := make(map[string]int, total)
	for i := 1; i <= total; i++ {
		hdr, err := b.ReadMessageHeader(i)
		if err != nil {
			log.Printf("WARN: Failed to read message header %d for MSGID index: %v", i, err)
			continue
		}
		if hdr.Attribute&jam.MsgDeleted != 0 {
			continue
		}
		for _, sf := range hdr.Subfields {
			if sf.LoID == jam.SfldMsgID {
				full := string(sf.Buffer)
				ids[full] = i
				// FTN MSGIDs are "address serial" — HPT often stores REPLY
				// kludges without the serial suffix.  Index the address part
				// too so prefix-based lookups succeed.
				if idx := strings.LastIndex(full, " "); idx > 0 {
					prefix := full[:idx]
					if _, exists := ids[prefix]; !exists {
						ids[prefix] = i
					}
				}
				break
			}
		}
	}
	return &msgidIndex{
		total:      total,
		modCounter: modCounter,
		msgIDs:     ids,
	}
}

// GetNewMessageCount returns the number of unread messages for a user in an area.
func (mm *MessageManager) GetNewMessageCount(areaID int, username string) (int, error) {
	b, _, err := mm.openBase(areaID)
	if err != nil {
		if errors.Is(err, ErrAreaNotFound) {
			return 0, nil // Return 0 if area doesn't exist
		}
		return 0, err // Propagate I/O and other errors
	}
	defer b.Close()

	return b.GetUnreadCount(username)
}

// GetLastRead returns the last-read message number for a user in an area.
// Returns 0 if the user has no lastread record.
func (mm *MessageManager) GetLastRead(areaID int, username string) (int, error) {
	b, _, err := mm.openBase(areaID)
	if err != nil {
		if errors.Is(err, ErrAreaNotFound) {
			return 0, nil // Return 0 if area doesn't exist
		}
		return 0, err // Propagate I/O and other errors
	}
	defer b.Close()

	lr, err := b.GetLastRead(username)
	if err != nil {
		if err == jam.ErrNotFound {
			return 0, nil
		}
		return 0, err
	}
	return int(lr.LastReadMsg), nil
}

// SetLastRead updates the lastread pointer for a user in an area.
func (mm *MessageManager) SetLastRead(areaID int, username string, msgNum int) error {
	b, _, err := mm.openBase(areaID)
	if err != nil {
		return err
	}
	defer b.Close()

	return b.MarkMessageRead(username, msgNum)
}

// GetNextUnreadMessage returns the next unread message number for a user.
// Returns 0, nil if there are no unread messages.
func (mm *MessageManager) GetNextUnreadMessage(areaID int, username string) (int, error) {
	b, _, err := mm.openBase(areaID)
	if err != nil {
		if errors.Is(err, ErrAreaNotFound) {
			return 0, nil // Return 0 if area doesn't exist
		}
		return 0, err // Propagate I/O and other errors
	}
	defer b.Close()

	next, err := b.GetNextUnreadMessage(username)
	if err != nil {
		if err == jam.ErrNotFound {
			return 0, nil
		}
		return 0, err
	}
	return next, nil
}

// GetBase returns the underlying JAM base for an area. This is used by
// the tosser for direct base access. The caller MUST close the base when done.
func (mm *MessageManager) GetBase(areaID int) (*jam.Base, error) {
	b, _, err := mm.openBase(areaID)
	if err != nil {
		return nil, err
	}
	// Note: Caller must close the base
	return b, nil
}

// normalizeLineEndings converts JAM CR line endings to LF for display.
func normalizeLineEndings(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}
