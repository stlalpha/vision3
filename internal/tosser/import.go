package tosser

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robbiew/vision3/internal/ftn"
	"github.com/robbiew/vision3/internal/jam"
	"github.com/robbiew/vision3/internal/message"
)

// TossResult holds the results of a toss/scan cycle.
type TossResult struct {
	PacketsProcessed int
	MessagesImported int
	MessagesExported int
	DupesSkipped     int
	Errors           []string
}

// Tosser handles importing and exporting FTN echomail packets for a single network.
type Tosser struct {
	networkName string
	config      networkConfig
	msgMgr      *message.MessageManager
	dupeDB      *DupeDB
	ownAddr     *jam.FidoAddress
}

// New creates a new Tosser instance for a single FTN network.
// The dupeDB is shared across networks (MSGIDs are globally unique).
func New(networkName string, cfg networkConfig, dupeDB *DupeDB, msgMgr *message.MessageManager) (*Tosser, error) {
	addr, err := jam.ParseAddress(cfg.OwnAddress)
	if err != nil {
		return nil, fmt.Errorf("tosser[%s]: invalid own_address %q: %w", networkName, cfg.OwnAddress, err)
	}

	return &Tosser{
		networkName: networkName,
		config:      cfg,
		msgMgr:      msgMgr,
		dupeDB:      dupeDB,
		ownAddr:     addr,
	}, nil
}

// NewDupeDBFromPath creates a shared DupeDB for use across multiple tossers.
func NewDupeDBFromPath(dupeDBPath string) (*DupeDB, error) {
	maxAge := 30 * 24 * time.Hour // 30 day dupe history
	return NewDupeDB(dupeDBPath, maxAge)
}

// ProcessInbound scans the inbound directory for .PKT files and tosses them.
func (t *Tosser) ProcessInbound() TossResult {
	result := TossResult{}

	entries, err := os.ReadDir(t.config.InboundPath)
	if err != nil {
		if os.IsNotExist(err) {
			return result // No inbound directory = nothing to do
		}
		result.Errors = append(result.Errors, fmt.Sprintf("read inbound dir: %v", err))
		return result
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".pkt") {
			continue
		}

		pktPath := filepath.Join(t.config.InboundPath, entry.Name())
		imported, dupes, errs := t.tossPacket(pktPath)
		result.PacketsProcessed++
		result.MessagesImported += imported
		result.DupesSkipped += dupes
		result.Errors = append(result.Errors, errs...)

		// Move processed packet to temp (or delete)
		if len(errs) == 0 {
			if err := os.Remove(pktPath); err != nil {
				log.Printf("WARN: Failed to remove processed packet %s: %v", pktPath, err)
			}
		} else {
			// Move to temp for inspection
			badPath := filepath.Join(t.config.TempPath, entry.Name())
			if err := os.Rename(pktPath, badPath); err != nil {
				log.Printf("WARN: Failed to move bad packet %s to %s: %v", pktPath, badPath, err)
			}
		}
	}

	// Save dupe DB after processing
	if err := t.dupeDB.Save(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("save dupe DB: %v", err))
	}

	return result
}

// tossPacket processes a single .PKT file, returning counts and errors.
func (t *Tosser) tossPacket(path string) (imported, dupes int, errs []string) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, []string{fmt.Sprintf("open %s: %v", path, err)}
	}
	defer f.Close()

	_, msgs, err := ftn.ReadPacket(f)
	if err != nil {
		return 0, 0, []string{fmt.Sprintf("parse %s: %v", path, err)}
	}

	for i, msg := range msgs {
		if err := t.tossMessage(msg); err != nil {
			if err == errDupe {
				dupes++
				continue
			}
			errs = append(errs, fmt.Sprintf("msg %d in %s: %v", i, filepath.Base(path), err))
			continue
		}
		imported++
	}

	return imported, dupes, errs
}

var errDupe = fmt.Errorf("duplicate message")

// tossMessage processes a single message from a packet.
func (t *Tosser) tossMessage(msg *ftn.PackedMessage) error {
	parsed := ftn.ParsePackedMessageBody(msg.Body)

	// Echomail must have an AREA tag
	if parsed.Area == "" {
		log.Printf("TRACE: Skipping non-echo message (no AREA tag) from %s to %s", msg.From, msg.To)
		return nil // Skip netmail/local for now
	}

	// Extract MSGID from kludges for dupe checking
	msgID := ""
	for _, k := range parsed.Kludges {
		if strings.HasPrefix(k, "MSGID: ") {
			msgID = strings.TrimPrefix(k, "MSGID: ")
			break
		}
	}

	// Dupe check
	if t.dupeDB.Add(msgID) {
		log.Printf("TRACE: Dupe message MSGID=%s in area %s", msgID, parsed.Area)
		return errDupe
	}

	// Find the target area by echo tag
	area, found := t.msgMgr.GetAreaByTag(parsed.Area)
	if !found {
		log.Printf("WARN: Unknown echo area %q, skipping message from %s", parsed.Area, msg.From)
		return fmt.Errorf("unknown area %q", parsed.Area)
	}

	base, err := t.msgMgr.GetBase(area.ID)
	if err != nil {
		return fmt.Errorf("get base for area %d: %w", area.ID, err)
	}

	// Update SEEN-BY and PATH with our address
	own2D := t.ownAddr.String2D()
	parsed.SeenBy = MergeSeenBy(parsed.SeenBy, own2D)
	parsed.Path = AppendPath(parsed.Path, own2D)

	// Build JAM message
	jamMsg := jam.NewMessage()
	jamMsg.From = msg.From
	jamMsg.To = msg.To
	jamMsg.Subject = msg.Subject
	jamMsg.Text = ftn.FormatPackedMessageBody(parsed)

	// Parse datetime
	dt, err := ftn.ParseFTNDateTime(msg.DateTime)
	if err != nil {
		dt = time.Now()
	}
	jamMsg.DateTime = dt

	// Set origin address from the packet message
	origAddr := fmt.Sprintf("%d:%d/%d", origZone(t.ownAddr), msg.OrigNet, msg.OrigNode)
	jamMsg.OrigAddr = origAddr

	// Set MSGID if we have one
	if msgID != "" {
		jamMsg.MsgID = msgID
	}

	// Preserve kludges
	for _, k := range parsed.Kludges {
		if strings.HasPrefix(k, "MSGID: ") || strings.HasPrefix(k, "REPLY: ") {
			continue // Handled separately
		}
		jamMsg.Kludges = append(jamMsg.Kludges, k)
	}

	// Extract REPLY kludge
	for _, k := range parsed.Kludges {
		if strings.HasPrefix(k, "REPLY: ") {
			jamMsg.ReplyID = strings.TrimPrefix(k, "REPLY: ")
			break
		}
	}

	// Write to JAM base with echomail handling
	msgType := jam.DetermineMessageType(area.AreaType, area.EchoTag)
	_, err = base.WriteMessageExt(jamMsg, msgType, area.EchoTag, "", "")
	if err != nil {
		return fmt.Errorf("write to JAM: %w", err)
	}

	log.Printf("INFO: Tossed message from %s to %s in %s (MSGID: %s)", msg.From, msg.To, parsed.Area, msgID)
	return nil
}

// origZone extracts the zone from the tosser's own address for packet context.
func origZone(addr *jam.FidoAddress) uint16 {
	if addr == nil {
		return 1
	}
	return uint16(addr.Zone)
}
