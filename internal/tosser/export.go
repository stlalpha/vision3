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

// ScanAndExport finds unsent echomail messages (DateProcessed=0) and
// creates outbound .PKT files grouped by destination link.
func (t *Tosser) ScanAndExport() TossResult {
	result := TossResult{}

	areas := t.msgMgr.ListAreas()

	// Collect messages to export, grouped by link address
	linkMsgs := make(map[string][]pendingMsg) // link address -> messages

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
		// TODO: Consider refactoring to close bases earlier for systems with many areas.
		// Current defer keeps all bases open until ScanAndExport returns. For large area
		// counts or long export runs, could close after processing each area or batch
		// DateProcessed updates to reduce open file descriptors.
		defer base.Close()

		count, err := base.GetMessageCount()
		if err != nil {
			log.Printf("WARN: Export: cannot get message count for area %d: %v", area.ID, err)
			continue
		}

		// Start scanning from high-water mark to avoid re-scanning exported messages
		startMsg := t.exportHighWater[area.ID] + 1
		if startMsg < 1 {
			startMsg = 1
		}

		for msgNum := startMsg; msgNum <= count; msgNum++ {
			hdr, err := base.ReadMessageHeader(msgNum)
			if err != nil {
				continue
			}

			// Skip already-processed messages
			if hdr.DateProcessed != 0 {
				// Advance high-water mark past consecutive processed messages
				if msgNum == t.exportHighWater[area.ID]+1 {
					t.exportHighWater[area.ID] = msgNum
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

			// Find which links should receive this message
			for _, link := range t.config.Links {
				for _, echoTag := range link.EchoAreas {
					if strings.EqualFold(echoTag, area.EchoTag) || echoTag == "*" {
						linkMsgs[link.Address] = append(linkMsgs[link.Address], pendingMsg{
							area:   area,
							msg:    msg,
							hdr:    hdr,
							msgNum: msgNum,
							base:   base,
						})
						break
					}
				}
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
		for _, pm := range msgs {
			pm.hdr.DateProcessed = uint32(time.Now().Unix())
			if err := pm.base.UpdateMessageHeader(pm.msgNum, pm.hdr); err != nil {
				log.Printf("WARN: Export: failed to update DateProcessed for msg %d: %v",
					pm.msgNum, err)
			}
		}

		result.MessagesExported += exported
	}

	return result
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
		link.Password,
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
			Attr:     ftn.MsgAttrLocal,
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
	if err := os.MkdirAll(t.config.OutboundPath, 0755); err != nil {
		return 0, fmt.Errorf("create outbound dir: %w", err)
	}
	f, err := os.CreateTemp(t.config.OutboundPath, "*.pkt")
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
	finalPath := filepath.Join(t.config.OutboundPath, finalName)
	if err := os.Rename(pktPath, finalPath); err != nil {
		// Temp file is already a valid .pkt, just log the rename failure
		log.Printf("WARN: Export: rename %s -> %s failed: %v (temp file kept)", pktPath, finalPath, err)
		finalName = filepath.Base(pktPath)
	}

	log.Printf("INFO: Exported %d messages to %s for link %s", len(packedMsgs), finalName, link.Address)
	return len(packedMsgs), nil
}
