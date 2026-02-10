package message

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robbiew/vision3/internal/jam"
)

const messageAreaFile = "message_areas.json"

// MessageManager handles message areas backed by JAM message bases.
type MessageManager struct {
	mu         sync.RWMutex
	dataPath   string                // Base data directory (e.g., "data")
	areasPath  string                // Full path to message_areas.json
	areasByID  map[int]*MessageArea
	areasByTag map[string]*MessageArea
	bases      map[int]*jam.Base // Open JAM bases keyed by area ID
	boardName  string            // BBS name for echomail origin lines
}

// NewMessageManager creates and initializes a new MessageManager.
// dataPath is the directory where JAM base files are stored.
// configPath is the directory containing message_areas.json.
// boardName is the BBS name used in echomail origin lines.
func NewMessageManager(dataPath, configPath, boardName string) (*MessageManager, error) {
	mm := &MessageManager{
		dataPath:   dataPath,
		areasPath:  filepath.Join(configPath, messageAreaFile),
		areasByID:  make(map[int]*MessageArea),
		areasByTag: make(map[string]*MessageArea),
		bases:      make(map[int]*jam.Base),
		boardName:  boardName,
	}

	if err := mm.loadMessageAreas(); err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: %s not found. Starting with no message areas.", messageAreaFile)
		} else {
			return nil, fmt.Errorf("failed to load message areas: %w", err)
		}
	}

	if err := mm.initializeBases(); err != nil {
		return nil, fmt.Errorf("failed to initialize JAM bases: %w", err)
	}

	log.Printf("INFO: MessageManager initialized. Loaded %d areas.", len(mm.areasByID))
	return mm, nil
}

// Close closes all open JAM bases. Call this during server shutdown.
func (mm *MessageManager) Close() error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	var errs []error
	for id, b := range mm.bases {
		if err := b.Close(); err != nil {
			errs = append(errs, fmt.Errorf("area %d: %w", id, err))
		}
	}
	mm.bases = make(map[int]*jam.Base)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing bases: %v", errs)
	}
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

// initializeBases opens (or creates) a JAM base for each configured area.
func (mm *MessageManager) initializeBases() error {
	for id, area := range mm.areasByID {
		basePath := mm.resolveBasePath(area)
		b, err := jam.Open(basePath)
		if err != nil {
			log.Printf("WARN: Failed to open JAM base for area %d (%s): %v", id, area.Tag, err)
			continue
		}
		mm.bases[id] = b
		log.Printf("TRACE: Opened JAM base for area %d (%s) at %s", id, area.Tag, basePath)
	}
	return nil
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
	mm.mu.RLock()
	area, exists := mm.areasByID[areaID]
	b, baseExists := mm.bases[areaID]
	mm.mu.RUnlock()

	if !exists {
		return 0, fmt.Errorf("message area %d not found", areaID)
	}
	if !baseExists {
		return 0, fmt.Errorf("JAM base not open for area %d", areaID)
	}

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

	if msgType.IsEchomail() || msgType.IsNetmail() {
		msg.OrigAddr = area.OriginAddr
		return b.WriteMessageExt(msg, msgType, area.EchoTag, mm.boardName)
	}

	return b.WriteMessage(msg)
}

// GetMessage reads a single message by area ID and 1-based message number.
func (mm *MessageManager) GetMessage(areaID, msgNum int) (*DisplayMessage, error) {
	mm.mu.RLock()
	b, exists := mm.bases[areaID]
	mm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("JAM base not open for area %d", areaID)
	}

	msg, err := b.ReadMessage(msgNum)
	if err != nil {
		return nil, err
	}

	return &DisplayMessage{
		MsgNum:    msgNum,
		From:      msg.From,
		To:        msg.To,
		Subject:   msg.Subject,
		DateTime:  msg.DateTime,
		Body:      normalizeLineEndings(msg.Text),
		MsgID:     msg.MsgID,
		ReplyID:   msg.ReplyID,
		OrigAddr:  msg.OrigAddr,
		IsPrivate: msg.IsPrivate(),
		IsDeleted: msg.IsDeleted(),
		AreaID:    areaID,
	}, nil
}

// GetMessageCountForArea returns the total message count for an area.
func (mm *MessageManager) GetMessageCountForArea(areaID int) (int, error) {
	mm.mu.RLock()
	b, exists := mm.bases[areaID]
	mm.mu.RUnlock()

	if !exists {
		return 0, nil
	}
	return b.GetMessageCount()
}

// GetNewMessageCount returns the number of unread messages for a user in an area.
func (mm *MessageManager) GetNewMessageCount(areaID int, username string) (int, error) {
	mm.mu.RLock()
	b, exists := mm.bases[areaID]
	mm.mu.RUnlock()

	if !exists {
		return 0, nil
	}
	return b.GetUnreadCount(username)
}

// GetLastRead returns the last-read message number for a user in an area.
// Returns 0 if the user has no lastread record.
func (mm *MessageManager) GetLastRead(areaID int, username string) (int, error) {
	mm.mu.RLock()
	b, exists := mm.bases[areaID]
	mm.mu.RUnlock()

	if !exists {
		return 0, nil
	}
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
	mm.mu.RLock()
	b, exists := mm.bases[areaID]
	mm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("JAM base not open for area %d", areaID)
	}
	return b.MarkMessageRead(username, msgNum)
}

// GetNextUnreadMessage returns the next unread message number for a user.
// Returns 0, nil if there are no unread messages.
func (mm *MessageManager) GetNextUnreadMessage(areaID int, username string) (int, error) {
	mm.mu.RLock()
	b, exists := mm.bases[areaID]
	mm.mu.RUnlock()

	if !exists {
		return 0, nil
	}
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
// the tosser for direct base access.
func (mm *MessageManager) GetBase(areaID int) (*jam.Base, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	b, exists := mm.bases[areaID]
	if !exists {
		return nil, fmt.Errorf("JAM base not open for area %d", areaID)
	}
	return b, nil
}

// normalizeLineEndings converts JAM CR line endings to LF for display.
func normalizeLineEndings(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}
