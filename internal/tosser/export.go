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

// getBaseHWM reads the export high-water mark from a JAM base's .jlr file.
// Returns 0 if no mark has been recorded yet (first scan is normal).
// Logs a warning if the read fails so the silent reset is visible in logs.
func getBaseHWM(base *jam.Base) int {
	lr, err := base.GetLastRead(ScannerUser)
	if err != nil {
		log.Printf("WARN: Export: failed to read HWM from .jlr, resetting to 0 (area will be fully re-scanned): %v", err)
		return 0
	}
	return int(lr.LastReadMsg)
}

// setBaseHWM writes the export high-water mark into a JAM base's .jlr file.
func setBaseHWM(base *jam.Base, msgNum int) error {
	return base.SetLastRead(ScannerUser, uint32(msgNum), uint32(msgNum))
}

// ScanAndExport finds unsent echomail messages (DateProcessed=0) and
// creates outbound .PKT files grouped by destination link.
func (t *Tosser) ScanAndExport() TossResult {
	result := TossResult{}

	areas := t.msgMgr.ListAreas()

	// Collect messages to export, grouped by link address
	linkMsgs := make(map[string][]pendingMsg) // link address -> messages

	// Track open bases explicitly; they must stay open through the update phase
	// since pending messages hold references to their base for DateProcessed updates.
	var openBases []*jam.Base
	defer func() {
		for _, b := range openBases {
			b.Close()
		}
	}()

	for _, area := range areas {
		if area.AreaType != "echomail" && area.AreaType != "echo" {
			continue
		}

		// Only export areas belonging to this network
		if !strings.EqualFold(area.Network, t.networkName) {
			continue
		}

		base, err := t.msgMgr.GetBase(area.ID)
		if err != nil {
			log.Printf("WARN: Export: cannot get base for area %d (%s): %v", area.ID, area.Tag, err)
			continue
		}
		openBases = append(openBases, base)

		count, err := base.GetMessageCount()
		if err != nil {
			log.Printf("WARN: Export: cannot get message count for area %d: %v", area.ID, err)
			continue
		}

		// Start scanning from high-water mark stored in the base's .jlr file
		startMsg := getBaseHWM(base) + 1
		if startMsg < 1 {
			startMsg = 1
		}

		for msgNum := startMsg; msgNum <= count; msgNum++ {
			hdr, err := base.ReadMessageHeader(msgNum)
			if err != nil {
				continue
			}

			// Skip already-processed messages and advance HWM past them
			if hdr.DateProcessed != 0 {
				cur := getBaseHWM(base)
				if msgNum == cur+1 {
					if err := setBaseHWM(base, msgNum); err != nil {
						log.Printf("WARN: Export: failed to advance HWM for area %d: %v", area.ID, err)
					}
				}
				continue
			}

			// Skip deleted messages
			if hdr.Attribute&jam.MsgDeleted != 0 {
				continue
			}

			msg, err := base.ReadMessage(msgNum)
			if err != nil {
				log.Printf("WARN: Export: cannot read message %d in area %d: %v", msgNum, area.ID, err)
				continue
			}

			// Route to all links in this network.
			// Echo area subscriptions are managed via Message Areas (Network field),
			// not per-link echo_areas lists.
			for _, link := range t.config.Links {
				linkMsgs[link.Address] = append(linkMsgs[link.Address], pendingMsg{
					area:   area,
					msg:    msg,
					hdr:    hdr,
					msgNum: msgNum,
					base:   base,
				})
			}
		}
	}

	// Create packets per link
	for linkAddr, msgs := range linkMsgs {
		link := t.findLink(linkAddr)
		if link == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("link %s not found in config", linkAddr))
			continue
		}

		exported, err := t.createOutboundPacket(link, msgs)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("create packet for %s: %v", linkAddr, err))
			continue
		}

		// Mark messages as processed only AFTER the packet was written successfully
		now := uint32(time.Now().Unix())
		for _, pm := range msgs {
			pm.hdr.DateProcessed = now
			if err := pm.base.UpdateMessageHeader(pm.msgNum, pm.hdr); err != nil {
				log.Printf("WARN: Export: failed to update DateProcessed for msg %d: %v",
					pm.msgNum, err)
			}
			// Advance HWM in the base's .jlr so the next scan skips this message
			if pm.msgNum > getBaseHWM(pm.base) {
				if err := setBaseHWM(pm.base, pm.msgNum); err != nil {
					log.Printf("WARN: Export: failed to update HWM for area %d msg %d: %v",
						pm.area.ID, pm.msgNum, err)
				}
			}
		}

		result.MessagesExported += exported
	}

	t.scanAndExportNetmail(&result)

	return result
}

// pendingNetmail holds a single outbound netmail message pending export.
type pendingNetmail struct {
	msg      *jam.Message
	hdr      *jam.MessageHeader
	msgNum   int
	base     *jam.Base
	destAddr *jam.FidoAddress
}

// scanAndExportNetmail finds unsent netmail messages (DateProcessed=0) in netmail areas
// and creates outbound .PKT files grouped by hub link.
func (t *Tosser) scanAndExportNetmail(result *TossResult) {
	linkMsgs := make(map[string][]pendingNetmail)

	var openBases []*jam.Base
	defer func() {
		for _, b := range openBases {
			b.Close()
		}
	}()

	for _, area := range t.msgMgr.ListAreas() {
		if area.AreaType != "netmail" {
			continue
		}
		if !strings.EqualFold(area.Network, t.networkName) {
			continue
		}

		base, err := t.msgMgr.GetBase(area.ID)
		if err != nil {
			log.Printf("WARN: NetmailExport: cannot get base for area %d (%s): %v", area.ID, area.Tag, err)
			continue
		}
		openBases = append(openBases, base)

		count, err := base.GetMessageCount()
		if err != nil {
			log.Printf("WARN: NetmailExport: cannot get message count for area %d: %v", area.ID, err)
			continue
		}

		for msgNum := 1; msgNum <= count; msgNum++ {
			hdr, err := base.ReadMessageHeader(msgNum)
			if err != nil {
				continue
			}
			if hdr.DateProcessed != 0 || hdr.Attribute&jam.MsgDeleted != 0 {
				continue
			}

			msg, err := base.ReadMessage(msgNum)
			if err != nil {
				log.Printf("WARN: NetmailExport: cannot read msg %d in area %s: %v", msgNum, area.Tag, err)
				continue
			}

			// Determine destination address: use stored DestAddr or fall back to hub.
			destAddrStr := msg.DestAddr
			if destAddrStr == "" {
				if len(t.config.Links) == 0 {
					log.Printf("WARN: NetmailExport: no links for network %s, cannot route msg %d", t.networkName, msgNum)
					continue
				}
				destAddrStr = t.config.Links[0].Address
			}
			destAddr, err := jam.ParseAddress(destAddrStr)
			if err != nil {
				log.Printf("WARN: NetmailExport: invalid dest addr %q for msg %d: %v", destAddrStr, msgNum, err)
				continue
			}

			// Route to the best link (direct match or hub fallback).
			link := t.findLinkForNetmail(destAddr)
			if link == nil {
				log.Printf("WARN: NetmailExport: no link available for %s, skipping msg %d", destAddrStr, msgNum)
				continue
			}

			linkMsgs[link.Address] = append(linkMsgs[link.Address], pendingNetmail{
				msg: msg, hdr: hdr, msgNum: msgNum, base: base, destAddr: destAddr,
			})
		}
	}

	for linkAddr, msgs := range linkMsgs {
		link := t.findLink(linkAddr)
		if link == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("netmail: link %s not found", linkAddr))
			continue
		}

		exported, err := t.createOutboundNetmailPacket(link, msgs)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("netmail packet for %s: %v", linkAddr, err))
			continue
		}

		now := uint32(time.Now().Unix())
		for _, pm := range msgs {
			pm.hdr.DateProcessed = now
			if err := pm.base.UpdateMessageHeader(pm.msgNum, pm.hdr); err != nil {
				log.Printf("WARN: NetmailExport: failed to mark msg %d as processed: %v", pm.msgNum, err)
			}
		}
		result.MessagesExported += exported
	}
}

// findLinkForNetmail returns the best link for routing a netmail message.
// Prefers a direct match (same zone:net/node); falls back to the first configured link.
func (t *Tosser) findLinkForNetmail(destAddr *jam.FidoAddress) *linkConfig {
	for i := range t.config.Links {
		addr, err := jam.ParseAddress(t.config.Links[i].Address)
		if err != nil {
			continue
		}
		if addr.Zone == destAddr.Zone && addr.Net == destAddr.Net && addr.Node == destAddr.Node {
			return &t.config.Links[i]
		}
	}
	if len(t.config.Links) > 0 {
		return &t.config.Links[0]
	}
	return nil
}

// createOutboundNetmailPacket creates a .PKT file for a batch of netmail messages to one link.
func (t *Tosser) createOutboundNetmailPacket(link *linkConfig, msgs []pendingNetmail) (int, error) {
	linkAddr, err := jam.ParseAddress(link.Address)
	if err != nil {
		return 0, fmt.Errorf("parse link address %q: %w", link.Address, err)
	}

	hdr := ftn.NewPacketHeader(
		uint16(t.ownAddr.Zone), uint16(t.ownAddr.Net), uint16(t.ownAddr.Node), uint16(t.ownAddr.Point),
		uint16(linkAddr.Zone), uint16(linkAddr.Net), uint16(linkAddr.Node), uint16(linkAddr.Point),
		link.PacketPassword,
	)

	var packedMsgs []*ftn.PackedMessage
	for _, pm := range msgs {
		// INTL kludge is required for all netmail
		intl := fmt.Sprintf("INTL %d:%d/%d %d:%d/%d",
			pm.destAddr.Zone, pm.destAddr.Net, pm.destAddr.Node,
			t.ownAddr.Zone, t.ownAddr.Net, t.ownAddr.Node)

		var kludges []string
		kludges = append(kludges, intl)
		if t.ownAddr.Point != 0 {
			kludges = append(kludges, fmt.Sprintf("FMPT %d", t.ownAddr.Point))
		}
		if pm.destAddr.Point != 0 {
			kludges = append(kludges, fmt.Sprintf("TOPT %d", pm.destAddr.Point))
		}

		msgIDStr := pm.msg.MsgID
		if msgIDStr == "" {
			msgIDStr = fmt.Sprintf("%s %08X", t.ownAddr.String(), uint32(time.Now().UnixNano()&0xFFFFFFFF))
		}
		kludges = append(kludges, "MSGID: "+msgIDStr)
		kludges = append(kludges, "PID: "+jam.FormatPID())
		kludges = append(kludges, pm.msg.Kludges...)

		parsed := &ftn.ParsedBody{
			Area:    "", // No AREA kludge for netmail
			Text:    pm.msg.Text,
			Kludges: kludges,
		}

		// Strip any "@address" suffix from To — the address belongs in the
		// packet header and INTL/TOPT kludges, not the username field.
		toName := pm.msg.To
		if idx := strings.LastIndex(toName, "@"); idx > 0 {
			toName = strings.TrimSpace(toName[:idx])
		}

		packedMsgs = append(packedMsgs, &ftn.PackedMessage{
			MsgType:  2,
			OrigNode: uint16(t.ownAddr.Node),
			DestNode: uint16(pm.destAddr.Node),
			OrigNet:  uint16(t.ownAddr.Net),
			DestNet:  uint16(pm.destAddr.Net),
			Attr:     ftn.MsgAttrLocal | ftn.MsgAttrPrivate,
			DateTime: ftn.FormatFTNDateTime(pm.msg.DateTime),
			To:       toName,
			From:     pm.msg.From,
			Subject:  pm.msg.Subject,
			Body:     ftn.FormatPackedMessageBody(parsed),
		})
	}

	if len(packedMsgs) == 0 {
		return 0, nil
	}

	if err := os.MkdirAll(t.paths.OutboundPath, 0755); err != nil {
		return 0, fmt.Errorf("create outbound dir: %w", err)
	}
	f, err := os.CreateTemp(t.paths.OutboundPath, "*.pkt")
	if err != nil {
		return 0, fmt.Errorf("create temp packet: %w", err)
	}
	pktPath := f.Name()

	if err := ftn.WritePacket(f, hdr, packedMsgs); err != nil {
		f.Close()
		os.Remove(pktPath)
		return 0, fmt.Errorf("write packet: %w", err)
	}
	f.Close()

	finalName := fmt.Sprintf("%08x.pkt", time.Now().UnixNano()&0xFFFFFFFF)
	finalPath := filepath.Join(t.paths.OutboundPath, finalName)
	if err := os.Rename(pktPath, finalPath); err != nil {
		log.Printf("WARN: NetmailExport: rename %s -> %s failed: %v (temp file kept)", pktPath, finalPath, err)
		finalName = filepath.Base(pktPath)
	}

	log.Printf("INFO: Exported %d netmail(s) to %s for link %s", len(packedMsgs), finalName, link.Address)
	return len(packedMsgs), nil
}

// findLink looks up a link config by address.
func (t *Tosser) findLink(address string) *linkConfig {
	for i := range t.config.Links {
		if t.config.Links[i].Address == address {
			return &t.config.Links[i]
		}
	}
	return nil
}

type pendingMsg struct {
	area   *message.MessageArea
	msg    *jam.Message
	hdr    *jam.MessageHeader
	msgNum int
	base   *jam.Base
}

// createOutboundPacket creates a .PKT file in the outbound directory.
func (t *Tosser) createOutboundPacket(link *linkConfig, msgs []pendingMsg) (int, error) {
	destAddr, err := jam.ParseAddress(link.Address)
	if err != nil {
		return 0, fmt.Errorf("parse link address %q: %w", link.Address, err)
	}

	hdr := ftn.NewPacketHeader(
		uint16(t.ownAddr.Zone), uint16(t.ownAddr.Net), uint16(t.ownAddr.Node), uint16(t.ownAddr.Point),
		uint16(destAddr.Zone), uint16(destAddr.Net), uint16(destAddr.Node), uint16(destAddr.Point),
		link.PacketPassword,
	)

	var packedMsgs []*ftn.PackedMessage
	own2D := t.ownAddr.String2D()

	for _, pm := range msgs {
		// Build body with AREA, kludges, text, SEEN-BY, PATH
		parsed := &ftn.ParsedBody{
			Area: pm.area.EchoTag,
			Text: pm.msg.Text,
		}

		// Add MSGID kludge
		if pm.msg.MsgID != "" {
			parsed.Kludges = append(parsed.Kludges, "MSGID: "+pm.msg.MsgID)
		}

		// Add REPLY kludge
		if pm.msg.ReplyID != "" {
			parsed.Kludges = append(parsed.Kludges, "REPLY: "+pm.msg.ReplyID)
		}

		// Add PID
		parsed.Kludges = append(parsed.Kludges, "PID: "+jam.FormatPID())

		// Existing kludges from the message
		parsed.Kludges = append(parsed.Kludges, pm.msg.Kludges...)

		// SEEN-BY and PATH
		if pm.msg.SeenBy != "" {
			parsed.SeenBy = []string{pm.msg.SeenBy}
		}
		parsed.SeenBy = MergeSeenBy(parsed.SeenBy, own2D)

		if pm.msg.Path != "" {
			parsed.Path = []string{pm.msg.Path}
		}
		parsed.Path = AppendPath(parsed.Path, own2D)

		body := ftn.FormatPackedMessageBody(parsed)

		packed := &ftn.PackedMessage{
			MsgType:  2,
			OrigNode: uint16(t.ownAddr.Node),
			DestNode: uint16(destAddr.Node),
			OrigNet:  uint16(t.ownAddr.Net),
			DestNet:  uint16(destAddr.Net),
			Attr:     linkMsgAttr(link.Flavour),
			DateTime: ftn.FormatFTNDateTime(pm.msg.DateTime),
			To:       pm.msg.To,
			From:     pm.msg.From,
			Subject:  pm.msg.Subject,
			Body:     body,
		}
		packedMsgs = append(packedMsgs, packed)
	}

	if len(packedMsgs) == 0 {
		return 0, nil
	}

	// Use os.CreateTemp to avoid filename collisions
	if err := os.MkdirAll(t.paths.OutboundPath, 0755); err != nil {
		return 0, fmt.Errorf("create outbound dir: %w", err)
	}
	f, err := os.CreateTemp(t.paths.OutboundPath, "*.pkt")
	if err != nil {
		return 0, fmt.Errorf("create temp packet: %w", err)
	}
	pktPath := f.Name()

	if err := ftn.WritePacket(f, hdr, packedMsgs); err != nil {
		f.Close()
		os.Remove(pktPath) // Clean up on error
		return 0, fmt.Errorf("write packet: %w", err)
	}
	f.Close()

	// Rename to a proper .pkt filename
	finalName := fmt.Sprintf("%08x.pkt", time.Now().UnixNano()&0xFFFFFFFF)
	finalPath := filepath.Join(t.paths.OutboundPath, finalName)
	if err := os.Rename(pktPath, finalPath); err != nil {
		// Temp file is already a valid .pkt, just log the rename failure
		log.Printf("WARN: Export: rename %s -> %s failed: %v (temp file kept)", pktPath, finalPath, err)
		finalName = filepath.Base(pktPath)
	}

	log.Printf("INFO: Exported %d messages to %s for link %s", len(packedMsgs), finalName, link.Address)
	return len(packedMsgs), nil
}

// linkMsgAttr returns the FTN packet attribute flags for a link's delivery flavour.
func linkMsgAttr(flavour string) uint16 {
	switch strings.ToUpper(flavour) {
	case "CRASH":
		return ftn.MsgAttrLocal | ftn.MsgAttrCrash
	case "HOLD":
		return ftn.MsgAttrLocal | ftn.MsgAttrHold
	default: // "NORMAL", "DIRECT", ""
		return ftn.MsgAttrLocal
	}
}
