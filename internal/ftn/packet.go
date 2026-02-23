package ftn

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// PacketType2Plus is the packet version identifier for Type-2+ packets.
const PacketType2Plus = 2

// CWValidation is the capability word validation value per FSC-0048.
const CWValidation = 0x0100

// MaxFieldLen limits null-terminated string fields.
const MaxFieldLen = 256

// Errors
var (
	ErrInvalidPacketType = errors.New("ftn: invalid packet type (expected 2)")
	ErrTruncatedPacket   = errors.New("ftn: truncated packet data")
	ErrTruncatedMessage  = errors.New("ftn: truncated message in packet")
)

// PacketHeader represents an FTN Type-2+ packet header (58 bytes).
type PacketHeader struct {
	OrigNode  uint16
	DestNode  uint16
	Year      uint16
	Month     uint16 // 0-based (0=Jan)
	Day       uint16
	Hour      uint16
	Minute    uint16
	Second    uint16
	Baud      uint16 // Unused, set to 0
	PktType   uint16 // Must be 2
	OrigNet   uint16
	DestNet   uint16
	ProdCode  uint8
	ProdRev   uint8
	Password  [8]byte
	QOrigZone uint16 // QMail orig zone
	QDestZone uint16 // QMail dest zone
	AuxNet    uint16 // Auxiliary net (point routing)
	CWCopy    uint16 // Capability word validation copy (swapped)
	ProdCode2 uint8  // Product code high byte
	ProdRev2  uint8  // Product revision minor
	CapWord   uint16 // Capability word (bit 0 = Type-2+)
	OrigZone  uint16
	DestZone  uint16
	OrigPoint uint16
	DestPoint uint16
	ProdData  [4]byte // Product-specific data
}

// PacketHeaderSize is the fixed size of a Type-2+ packet header.
const PacketHeaderSize = 58

// PackedMessage represents a single message within an FTN packet.
type PackedMessage struct {
	MsgType  uint16 // Always 2 for stored messages
	OrigNode uint16
	DestNode uint16
	OrigNet  uint16
	DestNet  uint16
	Attr     uint16 // Message attribute flags
	Cost     uint16
	DateTime string // "DD Mon YY  HH:MM:SS\x00" format
	To       string // Max 36 chars
	From     string // Max 36 chars
	Subject  string // Max 72 chars
	Body     string // Full message body including kludges
}

// Packed message attribute flags (FTS-0001).
const (
	MsgAttrPrivate  = 0x0001
	MsgAttrCrash    = 0x0002
	MsgAttrReceived = 0x0004
	MsgAttrSent     = 0x0008
	MsgAttrFile     = 0x0010
	MsgAttrTransit  = 0x0020
	MsgAttrOrphan   = 0x0040
	MsgAttrKillSent = 0x0080
	MsgAttrLocal    = 0x0100
	MsgAttrHold     = 0x0200
	MsgAttrFRQ      = 0x0800
)

// NewPacketHeader creates a header with sensible defaults for the given addresses.
func NewPacketHeader(origZone, origNet, origNode, origPoint uint16,
	destZone, destNet, destNode, destPoint uint16,
	password string) *PacketHeader {

	now := time.Now()
	h := &PacketHeader{
		OrigNode:  origNode,
		DestNode:  destNode,
		Year:      uint16(now.Year()),
		Month:     uint16(now.Month() - 1), // 0-based
		Day:       uint16(now.Day()),
		Hour:      uint16(now.Hour()),
		Minute:    uint16(now.Minute()),
		Second:    uint16(now.Second()),
		PktType:   PacketType2Plus,
		OrigNet:   origNet,
		DestNet:   destNet,
		QOrigZone: origZone,
		QDestZone: destZone,
		OrigZone:  origZone,
		DestZone:  destZone,
		OrigPoint: origPoint,
		DestPoint: destPoint,
		CapWord:   0x0001, // Type-2+ capable
		CWCopy:    CWValidation,
	}

	// Copy password (max 8 bytes, null-padded)
	pw := []byte(password)
	if len(pw) > 8 {
		pw = pw[:8]
	}
	copy(h.Password[:], pw)

	return h
}

// ReadPacketHeaderFromFile reads only the 58-byte header from a .PKT file at path.
// This is more efficient than ReadPacket when only the destination address is needed.
func ReadPacketHeaderFromFile(path string) (*PacketHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data := make([]byte, PacketHeaderSize)
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	hdr := &PacketHeader{}
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, hdr); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	if hdr.PktType != PacketType2Plus {
		return nil, fmt.Errorf("unsupported packet type %d (expected %d)", hdr.PktType, PacketType2Plus)
	}
	return hdr, nil
}

// ReadPacket parses a complete .PKT file from the reader.
func ReadPacket(r io.Reader) (*PacketHeader, []*PackedMessage, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("ftn: read packet: %w", err)
	}

	if len(data) < PacketHeaderSize+2 { // Header + terminator
		return nil, nil, ErrTruncatedPacket
	}

	// Parse header
	hdr := &PacketHeader{}
	buf := bytes.NewReader(data[:PacketHeaderSize])
	if err := binary.Read(buf, binary.LittleEndian, hdr); err != nil {
		return nil, nil, fmt.Errorf("ftn: read header: %w", err)
	}

	if hdr.PktType != PacketType2Plus {
		return nil, nil, ErrInvalidPacketType
	}

	// Parse messages
	var msgs []*PackedMessage
	pos := PacketHeaderSize

	for pos < len(data)-1 {
		// Check for packet terminator (two null bytes for msg type)
		if data[pos] == 0 && data[pos+1] == 0 {
			break
		}

		msg, nextPos, err := readPackedMessage(data, pos)
		if err != nil {
			return hdr, msgs, fmt.Errorf("ftn: message at offset %d: %w", pos, err)
		}
		msgs = append(msgs, msg)
		pos = nextPos
	}

	return hdr, msgs, nil
}

// readPackedMessage parses a single packed message starting at pos.
// Returns the message and the position after the message.
func readPackedMessage(data []byte, pos int) (*PackedMessage, int, error) {
	if pos+14 > len(data) {
		return nil, pos, ErrTruncatedMessage
	}

	msg := &PackedMessage{
		MsgType:  binary.LittleEndian.Uint16(data[pos:]),
		OrigNode: binary.LittleEndian.Uint16(data[pos+2:]),
		DestNode: binary.LittleEndian.Uint16(data[pos+4:]),
		OrigNet:  binary.LittleEndian.Uint16(data[pos+6:]),
		DestNet:  binary.LittleEndian.Uint16(data[pos+8:]),
		Attr:     binary.LittleEndian.Uint16(data[pos+10:]),
		Cost:     binary.LittleEndian.Uint16(data[pos+12:]),
	}
	pos += 14

	// Read null-terminated fields: DateTime, To, From, Subject, Body
	var s string
	var err error

	s, pos, err = readNullTerminated(data, pos, MaxFieldLen)
	if err != nil {
		return nil, pos, fmt.Errorf("datetime: %w", err)
	}
	msg.DateTime = s

	s, pos, err = readNullTerminated(data, pos, 36)
	if err != nil {
		return nil, pos, fmt.Errorf("to: %w", err)
	}
	msg.To = s

	s, pos, err = readNullTerminated(data, pos, 36)
	if err != nil {
		return nil, pos, fmt.Errorf("from: %w", err)
	}
	msg.From = s

	s, pos, err = readNullTerminated(data, pos, 72)
	if err != nil {
		return nil, pos, fmt.Errorf("subject: %w", err)
	}
	msg.Subject = s

	// Body can be very large - no practical limit
	s, pos, err = readNullTerminated(data, pos, len(data)-pos)
	if err != nil {
		return nil, pos, fmt.Errorf("body: %w", err)
	}
	msg.Body = s

	return msg, pos, nil
}

// readNullTerminated reads a null-terminated string from data at pos.
func readNullTerminated(data []byte, pos, maxLen int) (string, int, error) {
	end := pos
	limit := pos + maxLen
	if limit > len(data) {
		limit = len(data)
	}

	for end < limit {
		if data[end] == 0 {
			s := string(data[pos:end])
			return s, end + 1, nil // Skip the null terminator
		}
		end++
	}

	return "", pos, ErrTruncatedMessage
}

// WritePacket writes a complete .PKT file to the writer.
func WritePacket(w io.Writer, hdr *PacketHeader, msgs []*PackedMessage) error {
	// Write header
	if err := binary.Write(w, binary.LittleEndian, hdr); err != nil {
		return fmt.Errorf("ftn: write header: %w", err)
	}

	// Write messages
	for i, msg := range msgs {
		if err := writePackedMessage(w, msg); err != nil {
			return fmt.Errorf("ftn: write message %d: %w", i, err)
		}
	}

	// Write packet terminator (two null bytes)
	if _, err := w.Write([]byte{0, 0}); err != nil {
		return fmt.Errorf("ftn: write terminator: %w", err)
	}

	return nil
}

// writePackedMessage writes a single packed message.
func writePackedMessage(w io.Writer, msg *PackedMessage) error {
	// Write fixed header fields
	hdr := make([]byte, 14)
	binary.LittleEndian.PutUint16(hdr[0:], msg.MsgType)
	binary.LittleEndian.PutUint16(hdr[2:], msg.OrigNode)
	binary.LittleEndian.PutUint16(hdr[4:], msg.DestNode)
	binary.LittleEndian.PutUint16(hdr[6:], msg.OrigNet)
	binary.LittleEndian.PutUint16(hdr[8:], msg.DestNet)
	binary.LittleEndian.PutUint16(hdr[10:], msg.Attr)
	binary.LittleEndian.PutUint16(hdr[12:], msg.Cost)

	if _, err := w.Write(hdr); err != nil {
		return err
	}

	// Write null-terminated string fields
	to := truncateField(msg.To, 36)
	from := truncateField(msg.From, 36)
	subject := truncateField(msg.Subject, 72)
	for _, s := range []string{msg.DateTime, to, from, subject, msg.Body} {
		if _, err := w.Write(append([]byte(s), 0)); err != nil {
			return err
		}
	}

	return nil
}

func truncateField(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

// ParsedBody holds the components of a parsed FTN message body.
type ParsedBody struct {
	Area    string   // AREA tag (echomail only, empty for netmail/local)
	Kludges []string // ^A kludge lines (without the ^A prefix)
	Text    string   // Message text (without kludges, SEEN-BY, PATH)
	SeenBy  []string // SEEN-BY lines (without "SEEN-BY: " prefix)
	Path    []string // PATH lines (without "\x01PATH: " prefix)
}

// ParsePackedMessageBody separates an FTN message body into its components.
// Kludge lines start with \x01 (SOH), SEEN-BY/PATH are at the end.
func ParsePackedMessageBody(body string) *ParsedBody {
	result := &ParsedBody{}

	// Normalize line endings to \r (FTN standard)
	body = strings.ReplaceAll(body, "\r\n", "\r")
	body = strings.ReplaceAll(body, "\n", "\r")
	lines := strings.Split(body, "\r")

	var textLines []string
	seenByStarted := false

	for _, line := range lines {
		if line == "" && !seenByStarted {
			textLines = append(textLines, line)
			continue
		}

		// Check for AREA: tag (first line)
		if strings.HasPrefix(line, "AREA:") && result.Area == "" && len(textLines) == 0 {
			result.Area = strings.TrimPrefix(line, "AREA:")
			continue
		}

		// Check for kludge lines (\x01 prefix)
		if len(line) > 0 && line[0] == '\x01' {
			kludge := line[1:] // Strip SOH

			// Check for PATH kludge
			if strings.HasPrefix(kludge, "PATH: ") {
				result.Path = append(result.Path, strings.TrimPrefix(kludge, "PATH: "))
				seenByStarted = true
				continue
			}

			result.Kludges = append(result.Kludges, kludge)
			continue
		}

		// Check for SEEN-BY lines
		if strings.HasPrefix(line, "SEEN-BY: ") {
			result.SeenBy = append(result.SeenBy, strings.TrimPrefix(line, "SEEN-BY: "))
			seenByStarted = true
			continue
		}

		if !seenByStarted {
			textLines = append(textLines, line)
		}
	}

	result.Text = strings.Join(textLines, "\r")
	// Trim trailing empty lines
	result.Text = strings.TrimRight(result.Text, "\r")

	return result
}

// FormatPackedMessageBody reassembles a message body from its components.
// Returns the body with FTN line endings (\r).
func FormatPackedMessageBody(parsed *ParsedBody) string {
	var buf strings.Builder

	// AREA tag first
	if parsed.Area != "" {
		buf.WriteString("AREA:")
		buf.WriteString(parsed.Area)
		buf.WriteString("\r")
	}

	// Kludge lines (with SOH prefix)
	for _, k := range parsed.Kludges {
		buf.WriteString("\x01")
		buf.WriteString(k)
		buf.WriteString("\r")
	}

	// Message text
	if parsed.Text != "" {
		buf.WriteString(parsed.Text)
		if !strings.HasSuffix(parsed.Text, "\r") {
			buf.WriteString("\r")
		}
	}

	// SEEN-BY lines
	for _, sb := range parsed.SeenBy {
		buf.WriteString("SEEN-BY: ")
		buf.WriteString(sb)
		buf.WriteString("\r")
	}

	// PATH lines (with SOH prefix)
	for _, p := range parsed.Path {
		buf.WriteString("\x01PATH: ")
		buf.WriteString(p)
		buf.WriteString("\r")
	}

	return buf.String()
}

// FormatFTNDateTime formats a time in FTN packed message format.
// Format: "DD Mon YY  HH:MM:SS" (note: double space before time).
func FormatFTNDateTime(t time.Time) string {
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun",
		"Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	return fmt.Sprintf("%02d %s %02d  %02d:%02d:%02d",
		t.Day(), months[t.Month()-1], t.Year()%100,
		t.Hour(), t.Minute(), t.Second())
}

// ParseFTNDateTime parses an FTN datetime string back to time.Time.
func ParseFTNDateTime(s string) (time.Time, error) {
	// Try standard FTN format: "DD Mon YY  HH:MM:SS"
	t, err := time.Parse("02 Jan 06  15:04:05", s)
	if err != nil {
		// Some implementations use single space
		t, err = time.Parse("02 Jan 06 15:04:05", s)
	}
	return t, err
}
