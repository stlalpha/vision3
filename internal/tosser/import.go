package tosser

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/ftn"
	"github.com/stlalpha/vision3/internal/jam"
	"github.com/stlalpha/vision3/internal/message"
)

// inboundDirs returns all inbound directories to scan, deduplicating empty paths.
func (t *Tosser) inboundDirs() []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, d := range []string{t.paths.InboundPath, t.paths.SecureInboundPath} {
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

// ScannerUser is the synthetic username stored in each JAM base's .jlr file
// to track the export high-water mark. This follows the traditional FTN convention
// of keeping per-base scanner positions in the lastread file.
const ScannerUser = "v3mail"

// Tosser handles importing and exporting FTN echomail packets for a single network.
type Tosser struct {
	networkName    string
	config         networkConfig
	paths          pathConfig
	netmailAreaTag string // tag of the netmail area for this network, derived from message areas
	msgMgr         *message.MessageManager
	dupeDB         *DupeDB
	ownAddr        *jam.FidoAddress
}

// New creates a new Tosser instance for a single FTN network.
func New(networkName string, cfg networkConfig, globalCfg config.FTNConfig, dupeDB *DupeDB, msgMgr *message.MessageManager) (*Tosser, error) {
	addr, err := jam.ParseAddress(cfg.OwnAddress)
	if err != nil {
		return nil, fmt.Errorf("tosser[%s]: invalid own_address %q: %w", networkName, cfg.OwnAddress, err)
	}

	// Derive the netmail area tag from message areas (AreaType == "netmail" for this network).
	netmailAreaTag := ""
	for _, area := range msgMgr.ListAreas() {
		if strings.EqualFold(area.Network, networkName) && strings.EqualFold(area.AreaType, "netmail") {
			netmailAreaTag = area.Tag
			break
		}
	}

	return &Tosser{
		networkName:    networkName,
		config:         cfg,
		netmailAreaTag: netmailAreaTag,
		paths: pathConfig{
			InboundPath:       globalCfg.InboundPath,
			SecureInboundPath: globalCfg.SecureInboundPath,
			OutboundPath:      globalCfg.OutboundPath,
			BinkdOutboundPath: globalCfg.BinkdOutboundPath,
			TempPath:          globalCfg.TempPath,
			BadAreaTag:        globalCfg.BadAreaTag,
			DupeAreaTag:       globalCfg.DupeAreaTag,
		},
		msgMgr:  msgMgr,
		dupeDB:  dupeDB,
		ownAddr: addr,
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
	tempDir := filepath.Join(t.paths.TempPath, "unpack")
	pktPaths, err := ftn.ExtractBundle(path, tempDir)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("extract bundle %s: %v", name, err))
		// Move bad bundle to temp for inspection
		badPath := filepath.Join(t.paths.TempPath, name)
		if renErr := os.Rename(path, badPath); renErr != nil {
			log.Printf("WARN: Failed to move bad bundle %s: %v", path, renErr)
		}
		return
	}

	log.Printf("INFO: Unpacked bundle %s: %d .PKT files", name, len(pktPaths))

	// tossPktFile handles cleanup of each extracted .PKT (removes on success, moves to temp on error).
	allSkipped := true
	for _, pktPath := range pktPaths {
		skipped := t.tossPktFile(pktPath, filepath.Base(pktPath), result)
		if !skipped {
			allSkipped = false
		}
	}

	// If all packets in the bundle belong to a different network, leave the
	// bundle in place for the correct tosser and clean up extracted temp files.
	if allSkipped && len(pktPaths) > 0 {
		for _, pktPath := range pktPaths {
			os.Remove(pktPath)
		}
		log.Printf("TRACE: tosser[%s]: skipping foreign bundle %s", t.networkName, name)
		return
	}

	// Remove the bundle itself after processing all packets.
	if err := os.Remove(path); err != nil {
		log.Printf("WARN: Failed to remove processed bundle %s: %v", path, err)
	}
}

// tossPktFile processes a single .PKT file at path and updates result.
// Returns true if the packet was skipped (foreign network).
func (t *Tosser) tossPktFile(path, displayName string, result *TossResult) bool {
	imported, dupes, errs, skipped := t.tossPacket(path)
	if skipped {
		return true // packet doesn't belong to this network; leave for correct tosser
	}

	result.PacketsProcessed++
	result.MessagesImported += imported
	result.DupesSkipped += dupes
	result.Errors = append(result.Errors, errs...)

	if len(errs) == 0 {
		if err := os.Remove(path); err != nil {
			log.Printf("WARN: Failed to remove processed packet %s: %v", path, err)
		}
	} else {
		badPath := filepath.Join(t.paths.TempPath, displayName)
		if err := os.Rename(path, badPath); err != nil {
			log.Printf("WARN: Failed to move bad packet %s to %s: %v", path, badPath, err)
		}
	}
	return false
}

// tossPacket processes a single .PKT file, returning counts and errors.
// When multiple networks share the same inbound directory, the packet header's
// source address is checked against the network's known links. If the packet
// doesn't originate from a known link, it is skipped (returned with skipped=true)
// so the correct network's tosser can process it later.
func (t *Tosser) tossPacket(path string) (imported, dupes int, errs []string, skipped bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, []string{fmt.Sprintf("open %s: %v", path, err)}, false
	}
	defer f.Close()

	pktHdr, msgs, err := ftn.ReadPacket(f)
	if err != nil {
		return 0, 0, []string{fmt.Sprintf("parse %s: %v", path, err)}, false
	}

	// Check if this packet belongs to our network by matching the source address
	// against our known links. This prevents cross-contamination when multiple
	// networks share the same inbound directory.
	if !t.isPacketFromKnownLink(pktHdr) {
		pktZone := pktHdr.OrigZone
		if pktZone == 0 {
			pktZone = pktHdr.QOrigZone
		}
		log.Printf("TRACE: tosser[%s]: skipping packet %s from unknown link %d:%d/%d",
			t.networkName, filepath.Base(path), pktZone, pktHdr.OrigNet, pktHdr.OrigNode)
		return 0, 0, nil, true
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

	return imported, dupes, errs, false
}

var errDupe = fmt.Errorf("duplicate message")

// isPacketFromKnownLink checks whether a packet header's source address matches
// any configured link for this network. Compares zone, net, and node; point is
// ignored since hub packets typically originate from the main node address.
func (t *Tosser) isPacketFromKnownLink(hdr *ftn.PacketHeader) bool {
	pktZone := hdr.OrigZone
	if pktZone == 0 {
		pktZone = hdr.QOrigZone
	}

	hasValidLink := false
	for _, link := range t.config.Links {
		addr, err := jam.ParseAddress(link.Address)
		if err != nil {
			log.Printf("WARN: tosser[%s]: invalid link address %q: %v", t.networkName, link.Address, err)
			continue
		}
		hasValidLink = true
		if uint16(addr.Zone) == pktZone &&
			uint16(addr.Net) == hdr.OrigNet &&
			uint16(addr.Node) == hdr.OrigNode {
			return true
		}
	}
	// If no configured link addresses could be parsed, accept the packet
	// to avoid silently stalling all inbound processing.
	if !hasValidLink {
		log.Printf("WARN: tosser[%s]: no valid link addresses configured; accepting packet from %d:%d/%d",
			t.networkName, pktZone, hdr.OrigNet, hdr.OrigNode)
		return true
	}
	return false
}

// tossMessage processes a single message from a packet.
func (t *Tosser) tossMessage(msg *ftn.PackedMessage, pktHdr *ftn.PacketHeader) error {
	parsed := ftn.ParsePackedMessageBody(msg.Body)

	// Extract MSGID from kludges for dupe checking
	msgID := ""
	for _, k := range parsed.Kludges {
		if strings.HasPrefix(k, "MSGID: ") {
			msgID = strings.TrimPrefix(k, "MSGID: ")
			break
		}
	}

	// Netmail: messages without AREA: kludge are private point-to-point messages.
	if parsed.Area == "" {
		if t.netmailAreaTag != "" {
			if err := t.writeMsgToArea(t.netmailAreaTag, msg, pktHdr, parsed, msgID); err != nil {
				return fmt.Errorf("netmail to area %q: %w", t.netmailAreaTag, err)
			}
			log.Printf("INFO: Tossed netmail from %s to %s (MSGID: %s)", msg.From, msg.To, msgID)
			return nil
		}
		log.Printf("TRACE: Skipping netmail from %s to %s (no netmail area configured for network %s)", msg.From, msg.To, t.networkName)
		return nil
	}

	// Dupe check (only meaningful if message has a MSGID)
	if msgID != "" && t.dupeDB.Add(msgID) {
		log.Printf("TRACE: Dupe MSGID=%s area %s", msgID, parsed.Area)
		if t.paths.DupeAreaTag != "" {
			if err := t.writeMsgToArea(t.paths.DupeAreaTag, msg, pktHdr, parsed, msgID); err != nil {
				log.Printf("WARN: Dupe area write failed: %v", err)
			}
		}
		return errDupe
	}

	// Find the target area by echo tag.
	// Fall back to EchoTag index for areas using a local tag-prefix
	// (e.g. Tag="FD_LINUX", EchoTag="LINUX" when --tag-prefix was used during setup).
	// Constrain the fallback to the current network to avoid cross-network misrouting
	// when two networks share the same echo tag.
	area, found := t.msgMgr.GetAreaByTag(parsed.Area)
	if !found {
		if a, ok := t.msgMgr.GetAreaByEchoTag(parsed.Area); ok && strings.EqualFold(a.Network, t.networkName) {
			area, found = a, true
		}
	}
	if !found {
		log.Printf("WARN: Unknown echo area %q from %s", parsed.Area, msg.From)
		if t.paths.BadAreaTag != "" {
			if err := t.writeMsgToArea(t.paths.BadAreaTag, msg, pktHdr, parsed, msgID); err != nil {
				log.Printf("WARN: Bad area write failed for area %q: %v", parsed.Area, err)
				return fmt.Errorf("unknown area %q", parsed.Area)
			}
			log.Printf("INFO: Routed unknown area %q message from %s to bad area", parsed.Area, msg.From)
			return nil // counted as imported
		}
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

	// Extract REPLY kludge. Keep the full MSGID value (including the unique
	// hash portion for @-style addresses like "a1b2c3d4@1:2/3") so that
	// reply linking can match against the MSGID index.
	for _, k := range parsed.Kludges {
		if strings.HasPrefix(k, "REPLY: ") {
			replyValue := strings.TrimPrefix(k, "REPLY: ")
			// FTN MSGID is "address unique" — take the first complete pair
			// (addr + unique) when multiple MSGIDs are present in the kludge.
			if parts := strings.Fields(replyValue); len(parts) >= 2 {
				jamMsg.ReplyID = parts[0] + " " + parts[1]
			} else if len(parts) == 1 {
				jamMsg.ReplyID = parts[0]
			}
			break
		}
	}

	// Write to JAM base with echomail handling
	msgType := jam.DetermineMessageType(area.AreaType, area.EchoTag)
	msgNum, err := base.WriteMessageExt(jamMsg, msgType, area.EchoTag, "", "")
	if err != nil {
		return fmt.Errorf("write to JAM: %w", err)
	}

	// WriteMessageExt sets DateProcessed=0 for echomail to signal "needs export".
	// Inbound messages have already been processed by the network, so mark them
	// as processed now to prevent v3mail scan from re-exporting them to the uplink.
	if hdr, herr := base.ReadMessageHeader(msgNum); herr == nil {
		hdr.DateProcessed = uint32(time.Now().Unix())
		if uerr := base.UpdateMessageHeader(msgNum, hdr); uerr != nil {
			log.Printf("WARN: toss: failed to mark msg %d as processed in area %s: %v", msgNum, area.Tag, uerr)
		}
	} else {
		log.Printf("WARN: toss: failed to read header for msg %d in area %s to mark as processed: %v", msgNum, area.Tag, herr)
	}

	log.Printf("INFO: Tossed message from %s to %s in %s (MSGID: %s)", msg.From, msg.To, parsed.Area, msgID)
	return nil
}

// writeMsgToArea writes a packet message to any JAM area by tag.
// Used for netmail, bad-area, and dupe-area routing.
func (t *Tosser) writeMsgToArea(areaTag string, msg *ftn.PackedMessage, pktHdr *ftn.PacketHeader, parsed *ftn.ParsedBody, msgID string) error {
	area, found := t.msgMgr.GetAreaByTag(areaTag)
	if !found {
		return fmt.Errorf("area %q not configured", areaTag)
	}
	base, err := t.msgMgr.GetBase(area.ID)
	if err != nil {
		return fmt.Errorf("get base for area %q: %w", areaTag, err)
	}
	defer base.Close()

	jamMsg := jam.NewMessage()
	jamMsg.From = msg.From
	jamMsg.To = msg.To
	jamMsg.Subject = msg.Subject
	jamMsg.Text = parsed.Text
	if msgID != "" {
		jamMsg.MsgID = msgID
	}

	dt, err := ftn.ParseFTNDateTime(msg.DateTime)
	if err != nil {
		dt = time.Now()
	}
	jamMsg.DateTime = dt

	origZone := pktHdr.OrigZone
	if origZone == 0 {
		origZone = pktHdr.QOrigZone
	}
	if origZone == 0 {
		origZone = uint16(t.ownAddr.Zone)
	}
	jamMsg.OrigAddr = fmt.Sprintf("%d:%d/%d", origZone, msg.OrigNet, msg.OrigNode)

	// Preserve kludges (excluding MSGID/REPLY which are handled separately)
	for _, k := range parsed.Kludges {
		if strings.HasPrefix(k, "MSGID: ") || strings.HasPrefix(k, "REPLY: ") {
			continue
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

	// Preserve SEEN-BY and PATH
	if len(parsed.SeenBy) > 0 {
		jamMsg.SeenBy = strings.Join(parsed.SeenBy, " ")
	}
	if len(parsed.Path) > 0 {
		jamMsg.Path = strings.Join(parsed.Path, " ")
	}

	msgType := jam.DetermineMessageType(area.AreaType, area.EchoTag)
	msgNum, err := base.WriteMessageExt(jamMsg, msgType, area.EchoTag, "", "")
	if err != nil {
		return err
	}

	// Same as tossMessage: mark as processed so scan doesn't re-export routed messages.
	if hdr, herr := base.ReadMessageHeader(msgNum); herr == nil {
		hdr.DateProcessed = uint32(time.Now().Unix())
		if uerr := base.UpdateMessageHeader(msgNum, hdr); uerr != nil {
			log.Printf("WARN: toss: failed to mark routed msg %d in %s as processed: %v", msgNum, areaTag, uerr)
		}
	} else {
		log.Printf("WARN: toss: failed to read header for routed msg %d in %s to mark as processed: %v", msgNum, areaTag, herr)
	}
	return nil
}
