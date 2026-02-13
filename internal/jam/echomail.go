package jam

import (
	"fmt"
	"strings"
	"time"
)

// WriteMessageExt writes a message with full echomail support. For echomail
// messages it generates a MSGID, adds AREA/PID/TID kludges, tearline, and
// origin line. DateProcessed is set to 0 so the tosser knows to export it.
//
// For local messages this behaves identically to WriteMessage.
func (b *Base) WriteMessageExt(msg *Message, msgType MessageType, echoTag, bbsName, tearline string) (int, error) {
	var msgNum int
	err := b.withFileLock(func() error {
		b.mu.Lock()
		defer b.mu.Unlock()

		if !b.isOpen {
			return ErrBaseNotOpen
		}
		if err := b.readFixedHeader(); err != nil {
			return err
		}

		attr := msgType.GetJAMAttribute()
		if msg.Header != nil && msg.Header.Attribute != 0 {
			attr |= msg.Header.Attribute
		}

		hdr := &MessageHeader{
			Revision:      1,
			DateWritten:   uint32(msg.DateTime.Unix()),
			DateReceived:  0,
			DateProcessed: 0, // 0 signals tosser to scan/export this message
			Attribute:     attr,
		}
		copy(hdr.Signature[:], Signature)

		// For local messages, set DateProcessed to now (already "processed")
		if msgType.IsLocal() {
			hdr.DateProcessed = uint32(time.Now().Unix())
		}

		hdr.Subfields = []Subfield{}

		// Origin/destination addresses
		if msg.OrigAddr != "" {
			hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldOAddress, msg.OrigAddr))
		}
		if msgType.IsNetmail() && msg.DestAddr != "" {
			hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldDAddress, msg.DestAddr))
		}

		// Standard subfields
		if msg.From != "" {
			hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldSenderName, msg.From))
		}
		if msg.To != "" {
			hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldReceiverName, msg.To))
		}
		if msg.Subject != "" {
			hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldSubject, msg.Subject))
		}

		// Echomail-specific kludges and formatting
		if msgType.IsEchomail() {
			if msg.MsgID == "" && msg.OrigAddr != "" {
				msgID, err := b.generateMSGIDLocked(msg.OrigAddr)
				if err != nil {
					return fmt.Errorf("jam: MSGID generation failed: %w", err)
				}
				msg.MsgID = msgID
			}
			if msg.MsgID != "" {
				hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldMsgID, msg.MsgID))
				hdr.MSGIDcrc = CRC32String(msg.MsgID)
			}
			if echoTag != "" {
				hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldFTSKludge, "AREA:"+echoTag))
			}
			hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldPID, FormatPID()))
			hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldFTSKludge, "TID: "+FormatTID()))

			if bbsName != "" && msg.OrigAddr != "" {
				msg.Text = AddCustomTearline(msg.Text, tearline)
				msg.Text = AddOriginLine(msg.Text, bbsName, msg.OrigAddr)
			}
			// SEEN-BY and PATH are NOT added here — that is the tosser's job.
		} else {
			if msg.MsgID != "" {
				hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldMsgID, msg.MsgID))
				hdr.MSGIDcrc = CRC32String(msg.MsgID)
			}
		}

		// SEEN-BY and PATH subfields (set by tosser during import)
		if msg.SeenBy != "" {
			hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldSeenBy2D, msg.SeenBy))
		}
		if msg.Path != "" {
			hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldPath2D, msg.Path))
		}

		// Reply handling (all message types)
		if msg.ReplyID != "" {
			hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldReplyID, msg.ReplyID))
			hdr.REPLYcrc = CRC32String(msg.ReplyID)
		}

		// Additional caller-provided kludges
		for _, kludge := range msg.Kludges {
			hdr.Subfields = append(hdr.Subfields, CreateSubfield(SfldFTSKludge, kludge))
		}

		// Calculate total subfield length
		hdr.SubfieldLen = 0
		for _, sf := range hdr.Subfields {
			hdr.SubfieldLen += SubfieldHdrSize + sf.DatLen
		}

		// Write text
		offset, txtLen, err := b.writeMessageText(msg.Text)
		if err != nil {
			return err
		}
		hdr.Offset = offset
		hdr.TxtLen = txtLen

		// Assign message number
		count, err := b.getMessageCountLocked()
		if err != nil {
			return err
		}
		msgNum = count + 1
		hdr.MessageNumber = uint32(msgNum) + b.fixedHeader.BaseMsgNum - 1

		// Write header
		hdrOffset, err := b.writeMessageHeader(hdr)
		if err != nil {
			return err
		}

		// Write index
		idx := &IndexRecord{
			ToCRC:     CRC32String(strings.ToLower(msg.To)),
			HdrOffset: hdrOffset,
		}
		if err := b.writeIndexRecord(msgNum, idx); err != nil {
			return err
		}

		// Update counters
		b.fixedHeader.ActiveMsgs++
		b.fixedHeader.ModCounter++
		if err := b.writeFixedHeader(); err != nil {
			return err
		}

		// Sync all files to ensure consistency for external readers (e.g., HPT)
		b.jdtFile.Sync()
		b.jdxFile.Sync()
		b.jhrFile.Sync()

		return nil
	})
	return msgNum, err
}
