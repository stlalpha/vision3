package jam

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"
)

// ReadMessageHeader reads a message header for the given 1-based message number.
func (b *Base) ReadMessageHeader(msgNum int) (*MessageHeader, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.readMessageHeaderLocked(msgNum)
}

func (b *Base) readMessageHeaderLocked(msgNum int) (*MessageHeader, error) {
	if !b.isOpen {
		return nil, ErrBaseNotOpen
	}

	idx, err := b.readIndexRecordLocked(msgNum)
	if err != nil {
		return nil, err
	}

	b.jhrFile.Seek(int64(idx.HdrOffset), 0)

	hdr := &MessageHeader{}
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.Signature)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.Revision)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.ReservedWord)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.SubfieldLen)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.TimesRead)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.MSGIDcrc)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.REPLYcrc)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.ReplyTo)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.Reply1st)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.ReplyNext)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.DateWritten)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.DateReceived)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.DateProcessed)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.MessageNumber)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.Attribute)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.Attribute2)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.Offset)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.TxtLen)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.PasswordCRC)
	binary.Read(b.jhrFile, binary.LittleEndian, &hdr.Cost)

	if string(hdr.Signature[:]) != Signature {
		return nil, ErrInvalidSignature
	}

	// Read subfields
	bytesRead := uint32(0)
	for bytesRead < hdr.SubfieldLen {
		sf := Subfield{}
		binary.Read(b.jhrFile, binary.LittleEndian, &sf.LoID)
		binary.Read(b.jhrFile, binary.LittleEndian, &sf.HiID)
		binary.Read(b.jhrFile, binary.LittleEndian, &sf.DatLen)
		sf.Buffer = make([]byte, sf.DatLen)
		b.jhrFile.Read(sf.Buffer)
		hdr.Subfields = append(hdr.Subfields, sf)
		bytesRead += SubfieldHdrSize + sf.DatLen
	}

	return hdr, nil
}

// WriteMessageHeader writes a message header to the .jhr file and returns
// the byte offset where it was written.
func (b *Base) writeMessageHeader(hdr *MessageHeader) (uint32, error) {
	if !b.isOpen {
		return 0, ErrBaseNotOpen
	}

	pos, err := b.jhrFile.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("jam: seek failed on .jhr: %w", err)
	}
	if pos == 0 {
		pos = HeaderSize
		b.jhrFile.Seek(pos, 0)
	}

	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Signature)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Revision)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.ReservedWord)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.SubfieldLen)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.TimesRead)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.MSGIDcrc)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.REPLYcrc)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.ReplyTo)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Reply1st)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.ReplyNext)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.DateWritten)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.DateReceived)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.DateProcessed)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.MessageNumber)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Attribute)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Attribute2)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Offset)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.TxtLen)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.PasswordCRC)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Cost)

	for _, sf := range hdr.Subfields {
		binary.Write(b.jhrFile, binary.LittleEndian, sf.LoID)
		binary.Write(b.jhrFile, binary.LittleEndian, sf.HiID)
		binary.Write(b.jhrFile, binary.LittleEndian, sf.DatLen)
		b.jhrFile.Write(sf.Buffer)
	}

	return uint32(pos), nil
}

// UpdateMessageHeader rewrites an existing message header in place.
// This is used by the tosser to update DateProcessed after export.
func (b *Base) UpdateMessageHeader(msgNum int, hdr *MessageHeader) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isOpen {
		return ErrBaseNotOpen
	}

	idx, err := b.readIndexRecordLocked(msgNum)
	if err != nil {
		return err
	}

	b.jhrFile.Seek(int64(idx.HdrOffset), 0)

	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Signature)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Revision)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.ReservedWord)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.SubfieldLen)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.TimesRead)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.MSGIDcrc)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.REPLYcrc)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.ReplyTo)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Reply1st)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.ReplyNext)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.DateWritten)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.DateReceived)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.DateProcessed)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.MessageNumber)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Attribute)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Attribute2)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Offset)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.TxtLen)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.PasswordCRC)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Cost)

	// Note: subfields are not rewritten since they don't change
	// and follow immediately after the fixed header portion.

	return nil
}

// ReadMessageText reads the raw message text (CP437) for the given header.
func (b *Base) ReadMessageText(hdr *MessageHeader) (string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.readMessageTextLocked(hdr)
}

func (b *Base) readMessageTextLocked(hdr *MessageHeader) (string, error) {
	if !b.isOpen {
		return "", ErrBaseNotOpen
	}
	if hdr.TxtLen == 0 {
		return "", nil
	}
	b.jdtFile.Seek(int64(hdr.Offset), 0)
	buf := make([]byte, hdr.TxtLen)
	if _, err := b.jdtFile.Read(buf); err != nil {
		return "", fmt.Errorf("jam: failed to read text: %w", err)
	}
	return string(buf), nil
}

// writeMessageText appends text to the .jdt file. LF is converted to CR
// per the JAM specification. Returns the offset and byte length written.
func (b *Base) writeMessageText(text string) (uint32, uint32, error) {
	if !b.isOpen {
		return 0, 0, ErrBaseNotOpen
	}
	text = strings.ReplaceAll(text, "\n", "\r")
	pos, err := b.jdtFile.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, 0, fmt.Errorf("jam: seek failed on .jdt: %w", err)
	}
	buf := []byte(text)
	if _, err := b.jdtFile.Write(buf); err != nil {
		return 0, 0, fmt.Errorf("jam: failed to write text: %w", err)
	}
	return uint32(pos), uint32(len(buf)), nil
}

// ReadMessage reads a complete message (header + subfields + text) for
// the given 1-based message number.
func (b *Base) ReadMessage(msgNum int) (*Message, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	hdr, err := b.readMessageHeaderLocked(msgNum)
	if err != nil {
		return nil, err
	}
	text, err := b.readMessageTextLocked(hdr)
	if err != nil {
		return nil, err
	}

	msg := &Message{
		Header:   hdr,
		Text:     text,
		DateTime: time.Unix(int64(hdr.DateWritten), 0),
	}

	for _, sf := range hdr.Subfields {
		val := string(sf.Buffer)
		switch sf.LoID {
		case SfldOAddress:
			msg.OrigAddr = val
		case SfldDAddress:
			msg.DestAddr = val
		case SfldSenderName:
			msg.From = val
		case SfldReceiverName:
			msg.To = val
		case SfldMsgID:
			msg.MsgID = val
		case SfldReplyID:
			msg.ReplyID = val
		case SfldSubject:
			msg.Subject = val
		case SfldPID:
			msg.PID = val
		case SfldFTSKludge:
			msg.Kludges = append(msg.Kludges, val)
		case SfldSeenBy2D:
			msg.SeenBy = val
		case SfldPath2D:
			msg.Path = val
		case SfldFlags:
			msg.Flags = val
		}
	}

	// If echomail/netmail but OrigAddr missing, try to extract from origin line
	isEcho := (hdr.Attribute & MsgTypeEcho) != 0
	isNet := (hdr.Attribute & MsgTypeNet) != 0
	if (isEcho || isNet) && msg.OrigAddr == "" {
		msg.OrigAddr = extractAddressFromOriginLine(text)
	}

	return msg, nil
}

// WriteMessage writes a complete local message to the base.
// Returns the 1-based message number assigned to the new message.
func (b *Base) WriteMessage(msg *Message) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isOpen {
		return 0, ErrBaseNotOpen
	}
	if err := b.readFixedHeader(); err != nil {
		return 0, err
	}

	hdr := &MessageHeader{
		Revision:      1,
		DateWritten:   uint32(msg.DateTime.Unix()),
		DateProcessed: uint32(time.Now().Unix()),
		Attribute:     msg.GetAttribute(),
	}
	copy(hdr.Signature[:], Signature)

	hdr.Subfields = buildSubfields(msg)

	// Calculate total subfield length
	hdr.SubfieldLen = 0
	for _, sf := range hdr.Subfields {
		hdr.SubfieldLen += SubfieldHdrSize + sf.DatLen
	}

	offset, txtLen, err := b.writeMessageText(msg.Text)
	if err != nil {
		return 0, err
	}
	hdr.Offset = offset
	hdr.TxtLen = txtLen

	count, err := b.getMessageCountLocked()
	if err != nil {
		return 0, err
	}
	msgNum := count + 1
	hdr.MessageNumber = uint32(msgNum) + b.fixedHeader.BaseMsgNum - 1

	hdrOffset, err := b.writeMessageHeader(hdr)
	if err != nil {
		return 0, err
	}

	idx := &IndexRecord{
		ToCRC:     CRC32String(strings.ToLower(msg.To)),
		HdrOffset: hdrOffset,
	}
	if err := b.writeIndexRecord(msgNum, idx); err != nil {
		return 0, err
	}

	b.fixedHeader.ActiveMsgs++
	b.fixedHeader.ModCounter++
	if err := b.writeFixedHeader(); err != nil {
		return 0, err
	}

	return msgNum, nil
}

// DeleteMessage marks a message as deleted and zeroes its text length.
func (b *Base) DeleteMessage(msgNum int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isOpen {
		return ErrBaseNotOpen
	}

	hdr, err := b.readMessageHeaderLocked(msgNum)
	if err != nil {
		return err
	}

	hdr.Attribute |= MsgDeleted
	hdr.TxtLen = 0

	idx, err := b.readIndexRecordLocked(msgNum)
	if err != nil {
		return err
	}

	// Rewrite header at original offset
	b.jhrFile.Seek(int64(idx.HdrOffset), 0)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Signature)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Revision)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.ReservedWord)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.SubfieldLen)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.TimesRead)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.MSGIDcrc)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.REPLYcrc)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.ReplyTo)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Reply1st)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.ReplyNext)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.DateWritten)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.DateReceived)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.DateProcessed)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.MessageNumber)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Attribute)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Attribute2)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Offset)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.TxtLen)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.PasswordCRC)
	binary.Write(b.jhrFile, binary.LittleEndian, hdr.Cost)

	b.fixedHeader.ActiveMsgs--
	b.fixedHeader.ModCounter++
	return b.writeFixedHeader()
}

// ScanMessages reads up to maxMessages starting from startMsg (1-based),
// skipping deleted messages. If maxMessages is 0, reads all.
func (b *Base) ScanMessages(startMsg, maxMessages int) ([]*Message, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.isOpen {
		return nil, ErrBaseNotOpen
	}
	count, err := b.getMessageCountLocked()
	if err != nil {
		return nil, err
	}
	if startMsg < 1 {
		startMsg = 1
	}

	var messages []*Message
	read := 0
	for n := startMsg; n <= count && (maxMessages == 0 || read < maxMessages); n++ {
		hdr, err := b.readMessageHeaderLocked(n)
		if err != nil {
			continue
		}
		if hdr.Attribute&MsgDeleted != 0 {
			continue
		}
		text, err := b.readMessageTextLocked(hdr)
		if err != nil {
			continue
		}
		msg := &Message{
			Header:   hdr,
			Text:     text,
			DateTime: time.Unix(int64(hdr.DateWritten), 0),
		}
		for _, sf := range hdr.Subfields {
			val := string(sf.Buffer)
			switch sf.LoID {
			case SfldSenderName:
				msg.From = val
			case SfldReceiverName:
				msg.To = val
			case SfldSubject:
				msg.Subject = val
			case SfldMsgID:
				msg.MsgID = val
			case SfldOAddress:
				msg.OrigAddr = val
			}
		}
		messages = append(messages, msg)
		read++
	}
	return messages, nil
}

// buildSubfields assembles the standard subfield list for a message.
func buildSubfields(msg *Message) []Subfield {
	var sfs []Subfield
	if msg.OrigAddr != "" {
		sfs = append(sfs, CreateSubfield(SfldOAddress, msg.OrigAddr))
	}
	if msg.DestAddr != "" {
		sfs = append(sfs, CreateSubfield(SfldDAddress, msg.DestAddr))
	}
	if msg.From != "" {
		sfs = append(sfs, CreateSubfield(SfldSenderName, msg.From))
	}
	if msg.To != "" {
		sfs = append(sfs, CreateSubfield(SfldReceiverName, msg.To))
	}
	if msg.Subject != "" {
		sfs = append(sfs, CreateSubfield(SfldSubject, msg.Subject))
	}
	if msg.MsgID != "" {
		sfs = append(sfs, CreateSubfield(SfldMsgID, msg.MsgID))
	}
	if msg.ReplyID != "" {
		sfs = append(sfs, CreateSubfield(SfldReplyID, msg.ReplyID))
	}
	if msg.PID != "" {
		sfs = append(sfs, CreateSubfield(SfldPID, msg.PID))
	}
	for _, kludge := range msg.Kludges {
		sfs = append(sfs, CreateSubfield(SfldFTSKludge, kludge))
	}
	return sfs
}

// extractAddressFromOriginLine parses a FidoNet address from the origin line.
// Format: " * Origin: BBS Name (address)"
func extractAddressFromOriginLine(text string) string {
	normalized := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "* Origin:") {
			start := strings.LastIndex(line, "(")
			end := strings.LastIndex(line, ")")
			if start != -1 && end != -1 && end > start {
				return strings.TrimSpace(line[start+1 : end])
			}
		}
	}
	return ""
}

// CP437ToUnicode converts CP437-encoded bytes to a Unicode string.
func CP437ToUnicode(input []byte) string {
	cp437 := []rune{
		0x00C7, 0x00FC, 0x00E9, 0x00E2, 0x00E4, 0x00E0, 0x00E5, 0x00E7,
		0x00EA, 0x00EB, 0x00E8, 0x00EF, 0x00EE, 0x00EC, 0x00C4, 0x00C5,
		0x00C9, 0x00E6, 0x00C6, 0x00F4, 0x00F6, 0x00F2, 0x00FB, 0x00F9,
		0x00FF, 0x00D6, 0x00DC, 0x00A2, 0x00A3, 0x00A5, 0x20A7, 0x0192,
		0x00E1, 0x00ED, 0x00F3, 0x00FA, 0x00F1, 0x00D1, 0x00AA, 0x00BA,
		0x00BF, 0x2310, 0x00AC, 0x00BD, 0x00BC, 0x00A1, 0x00AB, 0x00BB,
		0x2591, 0x2592, 0x2593, 0x2502, 0x2524, 0x2561, 0x2562, 0x2556,
		0x2555, 0x2563, 0x2551, 0x2557, 0x255D, 0x255C, 0x255B, 0x2510,
		0x2514, 0x2534, 0x252C, 0x251C, 0x2500, 0x253C, 0x255E, 0x255F,
		0x255A, 0x2554, 0x2569, 0x2566, 0x2560, 0x2550, 0x256C, 0x2567,
		0x2568, 0x2564, 0x2565, 0x2559, 0x2558, 0x2552, 0x2553, 0x256B,
		0x256A, 0x2518, 0x250C, 0x2588, 0x2584, 0x258C, 0x2590, 0x2580,
		0x03B1, 0x00DF, 0x0393, 0x03C0, 0x03A3, 0x03C3, 0x00B5, 0x03C4,
		0x03A6, 0x0398, 0x03A9, 0x03B4, 0x221E, 0x03C6, 0x03B5, 0x2229,
		0x2261, 0x00B1, 0x2265, 0x2264, 0x2320, 0x2321, 0x00F7, 0x2248,
		0x00B0, 0x2219, 0x00B7, 0x221A, 0x207F, 0x00B2, 0x25A0, 0x00A0,
	}

	result := make([]rune, 0, len(input))
	for _, ch := range input {
		if ch < 0x80 {
			result = append(result, rune(ch))
		} else {
			result = append(result, cp437[ch-0x80])
		}
	}
	return string(result)
}
