package jam

import "time"

// FixedHeaderInfo represents the JAM base header (1024 bytes on disk).
// The first 76 bytes contain structured data; bytes 76-1023 are reserved.
// Reserved[0:4] stores the MSGID serial counter.
type FixedHeaderInfo struct {
	Signature   [4]byte
	DateCreated uint32
	ModCounter  uint32
	ActiveMsgs  uint32
	PasswordCRC uint32
	BaseMsgNum  uint32
	Reserved    [1000]byte
}

// MessageHeader represents a JAM message header stored in the .jhr file.
type MessageHeader struct {
	Signature     [4]byte
	Revision      uint16
	ReservedWord  uint16
	SubfieldLen   uint32
	TimesRead     uint32
	MSGIDcrc      uint32
	REPLYcrc      uint32
	ReplyTo       uint32
	Reply1st      uint32
	ReplyNext     uint32
	DateWritten   uint32
	DateReceived  uint32
	DateProcessed uint32
	MessageNumber uint32
	Attribute     uint32
	Attribute2    uint32
	Offset        uint32 // Offset into .jdt file
	TxtLen        uint32 // Length of text in .jdt
	PasswordCRC   uint32
	Cost          uint32
	Subfields     []Subfield
}

// Subfield represents a variable-length field attached to a message header.
type Subfield struct {
	LoID   uint16
	HiID   uint16
	DatLen uint32
	Buffer []byte
}

// IndexRecord represents an entry in the .jdx index file (8 bytes).
type IndexRecord struct {
	ToCRC     uint32 // CRC32 of lowercase recipient name
	HdrOffset uint32 // Byte offset of header in .jhr
}

// LastReadRecord represents a per-user lastread entry in the .jlr file (16 bytes).
type LastReadRecord struct {
	UserCRC     uint32 // CRC32 of lowercase username
	UserID      uint32
	LastReadMsg uint32
	HighReadMsg uint32
}

// Message is a high-level message structure combining header data,
// subfield-parsed fields, and message text.
type Message struct {
	Header   *MessageHeader
	From     string
	To       string
	Subject  string
	DateTime time.Time
	Text     string // CP437-encoded message body
	OrigAddr string // FidoNet origin address
	DestAddr string // FidoNet destination address
	MsgID    string
	ReplyID  string
	PID      string
	Flags    string
	SeenBy   string
	Path     string
	Kludges  []string
}

// NewMessage creates a new Message with default values.
func NewMessage() *Message {
	return &Message{
		DateTime: time.Now(),
	}
}

// IsDeleted reports whether the message has been marked as deleted.
func (m *Message) IsDeleted() bool {
	if m.Header == nil {
		return false
	}
	return m.Header.Attribute&MsgDeleted != 0
}

// IsPrivate reports whether the message is private.
func (m *Message) IsPrivate() bool {
	if m.Header == nil {
		return false
	}
	return m.Header.Attribute&MsgPrivate != 0
}

// GetAttribute returns the JAM attribute flags for this message.
func (m *Message) GetAttribute() uint32 {
	if m.Header != nil {
		return m.Header.Attribute
	}
	return MsgLocal | MsgTypeLocal
}

// CreateSubfield creates a Subfield from a type identifier and string data.
func CreateSubfield(fieldType uint16, data string) Subfield {
	buf := []byte(data)
	return Subfield{
		LoID:   fieldType,
		HiID:   0,
		DatLen: uint32(len(buf)),
		Buffer: buf,
	}
}

// GetSubfieldByType returns the first subfield matching the given type,
// or nil if none is found.
func (h *MessageHeader) GetSubfieldByType(fieldType uint16) *Subfield {
	for i := range h.Subfields {
		if h.Subfields[i].LoID == fieldType {
			return &h.Subfields[i]
		}
	}
	return nil
}

// GetAllSubfieldsByType returns all subfields matching the given type.
func (h *MessageHeader) GetAllSubfieldsByType(fieldType uint16) []Subfield {
	var fields []Subfield
	for _, sf := range h.Subfields {
		if sf.LoID == fieldType {
			fields = append(fields, sf)
		}
	}
	return fields
}
