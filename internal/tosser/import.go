package tosser

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stlalpha/vision3/internal/ftn"
	"github.com/stlalpha/vision3/internal/jam"
	"github.com/stlalpha/vision3/internal/message"
)

// inboundDirs returns all inbound directories to scan, deduplicating empty paths.
func (t *Tosser) inboundDirs() []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, d := range []string{t.config.InboundPath, t.config.SecureInboundPath} {
		if d != "" && !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	return dirs
}

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
	hwm         *HighWaterMark // persistent export position tracking
}

// New creates a new Tosser instance for a single FTN network.
// The dupeDB and hwm are shared across networks.
func New(networkName string, cfg networkConfig, dupeDB *DupeDB, hwm *HighWaterMark, msgMgr *message.MessageManager) (*Tosser, error) {
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
		hwm:         hwm,
	}, nil
}

// NewDupeDBFromPath creates a shared DupeDB for use across multiple tossers.
func NewDupeDBFromPath(dupeDBPath string) (*DupeDB, error) {
	maxAge := 30 * 24 * time.Hour // 30 day dupe history
	return NewDupeDB(dupeDBPath, maxAge)
}

// ProcessInbound scans all configured inbound directories for .PKT files and
// ZIP bundles, unpacking bundles as needed, then tosses each packet.
func (t *Tosser) ProcessInbound() TossResult {
	result := TossResult{}

	for _, inboundDir := range t.inboundDirs() {
		t.processInboundDir(inboundDir, &result)
	}

	// Save dupe DB after processing all directories
	if err := t.dupeDB.Save(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("save dupe DB: %v", err))
	}

	return result
}

// processInboundDir scans a single directory for bundles and .PKT files.
func (t *Tosser) processInboundDir(dir string, result *TossResult) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		result.Errors = append(result.Errors, fmt.Sprintf("read inbound dir %s: %v", dir, err))
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		nameLower := strings.ToLower(name)
		path := filepath.Join(dir, name)

		if strings.HasSuffix(nameLower, ".pkt") {
			// Direct .PKT file
			t.tossPktFile(path, name, result)
			continue
		}

		if ftn.BundleExtension(nameLower) {
			// Potential ZIP bundle — verify magic bytes first
			isZIP, err := ftn.IsZIPBundle(path)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("check bundle %s: %v", name, err))
				continue
			}
			if !isZIP {
				continue // .flo or other non-ZIP file, skip
			}
			t.processBundle(path, name, result)
		}
	}
}

// processBundle unpacks a ZIP bundle, tosses its .PKT contents, then removes it.
func (t *Tosser) processBundle(path, name string, result *TossResult) {
	tempDir := filepath.Join(t.config.TempPath, "unpack")
	pktPaths, err := ftn.ExtractBundle(path, tempDir)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("extract bundle %s: %v", name, err))
		// Move bad bundle to temp for inspection
		badPath := filepath.Join(t.config.TempPath, name)
		if renErr := os.Rename(path, badPath); renErr != nil {
			log.Printf("WARN: Failed to move bad bundle %s: %v", path, renErr)
		}
		return
	}

	log.Printf("INFO: Unpacked bundle %s: %d .PKT files", name, len(pktPaths))

	// tossPktFile handles cleanup of each extracted .PKT (removes on success, moves to temp on error).
	for _, pktPath := range pktPaths {
		t.tossPktFile(pktPath, filepath.Base(pktPath), result)
	}

	// Remove the bundle itself after processing all packets.
	if err := os.Remove(path); err != nil {
		log.Printf("WARN: Failed to remove processed bundle %s: %v", path, err)
	}
}

// tossPktFile processes a single .PKT file at path and updates result.
func (t *Tosser) tossPktFile(path, displayName string, result *TossResult) {
	imported, dupes, errs := t.tossPacket(path)
	result.PacketsProcessed++
	result.MessagesImported += imported
	result.DupesSkipped += dupes
	result.Errors = append(result.Errors, errs...)

	if len(errs) == 0 {
		if err := os.Remove(path); err != nil {
			log.Printf("WARN: Failed to remove processed packet %s: %v", path, err)
		}
	} else {
		badPath := filepath.Join(t.config.TempPath, displayName)
		if err := os.Rename(path, badPath); err != nil {
			log.Printf("WARN: Failed to move bad packet %s to %s: %v", path, badPath, err)
		}
	}
}

// tossPacket processes a single .PKT file, returning counts and errors.
func (t *Tosser) tossPacket(path string) (imported, dupes int, errs []string) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, []string{fmt.Sprintf("open %s: %v", path, err)}
	}
	defer f.Close()

	pktHdr, msgs, err := ftn.ReadPacket(f)
	if err != nil {
		return 0, 0, []string{fmt.Sprintf("parse %s: %v", path, err)}
	}

	for i, msg := range msgs {
		if err := t.tossMessage(msg, pktHdr); err != nil {
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
func (t *Tosser) tossMessage(msg *ftn.PackedMessage, pktHdr *ftn.PacketHeader) error {
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

	// Dupe check (only meaningful if message has a MSGID)
	if msgID != "" && t.dupeDB.Add(msgID) {
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
	defer base.Close()

	// Update SEEN-BY and PATH with our address
	own2D := t.ownAddr.String2D()
	parsed.SeenBy = MergeSeenBy(parsed.SeenBy, own2D)
	parsed.Path = AppendPath(parsed.Path, own2D)

	// Build JAM message
	jamMsg := jam.NewMessage()
	jamMsg.From = msg.From
	jamMsg.To = msg.To
	jamMsg.Subject = msg.Subject
	jamMsg.Text = parsed.Text // Store only the message text, not kludges/SEEN-BY/PATH

	// Store SEEN-BY and PATH as JAM subfields for proper roundtrip with export
	if len(parsed.SeenBy) > 0 {
		jamMsg.SeenBy = strings.Join(parsed.SeenBy, " ")
	}
	if len(parsed.Path) > 0 {
		jamMsg.Path = strings.Join(parsed.Path, " ")
	}

	// Parse datetime
	dt, err := ftn.ParseFTNDateTime(msg.DateTime)
	if err != nil {
		dt = time.Now()
	}
	jamMsg.DateTime = dt

	// Set origin address from the packet header and message
	origZone := pktHdr.OrigZone
	if origZone == 0 {
		origZone = pktHdr.QOrigZone // Fallback to QMail zone field
	}
	if origZone == 0 {
		origZone = uint16(t.ownAddr.Zone) // Last resort: assume same zone
	}
	origAddr := fmt.Sprintf("%d:%d/%d", origZone, msg.OrigNet, msg.OrigNode)
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
			replyValue := strings.TrimPrefix(k, "REPLY: ")
			// Extract only the first MSGID - split on spaces and take first token
			// This handles cases where REPLY contains multiple MSGIDs or malformed data
			if parts := strings.Fields(replyValue); len(parts) > 0 {
				replyID := parts[0]

				// Check if the reply looks like a MSGID with embedded FTN address
				// Format: "xxxx.something@zone:net/node" or "xxxx.something@zone:net/node.point"
				if atPos := strings.Index(replyID, "@"); atPos != -1 && atPos < len(replyID)-1 {
					// Extract the FTN address after the @ symbol
					ftnPart := replyID[atPos+1:]
					// Validate it looks like a proper FTN address (contains : and /)
					if strings.Contains(ftnPart, ":") && strings.Contains(ftnPart, "/") {
						replyID = ftnPart
					}
				}

				jamMsg.ReplyID = replyID
			}
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
