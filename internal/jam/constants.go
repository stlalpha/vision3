// Package jam implements the JAM message base format (JAM-001 specification).
// This is a fresh implementation for Vision3 BBS, using the retrograde BBS
// implementation as an architectural reference.
package jam

import "errors"

// JAM file format constants
const (
	Signature       = "JAM\x00"
	HeaderSize      = 1024 // Fixed header occupies first 1024 bytes of .jhr
	FixedHeaderSize = 76   // Actual data size within the fixed header
	SubfieldHdrSize = 8    // LoID(2) + HiID(2) + DatLen(4)
	IndexRecordSize = 8    // ToCRC(4) + HdrOffset(4)
	LastReadSize    = 16   // UserCRC(4) + UserID(4) + LastReadMsg(4) + HighReadMsg(4)
)

// Message attribute flags from JAM specification
const (
	MsgLocal       = 0x00000001 // Created locally
	MsgInTransit   = 0x00000002 // In-transit
	MsgPrivate     = 0x00000004 // Private message
	MsgRead        = 0x00000008 // Read by addressee
	MsgSent        = 0x00000010 // Sent to remote
	MsgKillSent    = 0x00000020 // Kill when sent
	MsgArchiveSent = 0x00000040 // Archive when sent
	MsgHold        = 0x00000080 // Hold for pick-up
	MsgCrash       = 0x00000100 // Crash
	MsgImmediate   = 0x00000200 // Send immediately
	MsgDirect      = 0x00000400 // Send directly
	MsgGate        = 0x00000800 // Send via gateway
	MsgFileRequest = 0x00001000 // File request
	MsgFileAttach  = 0x00002000 // File(s) attached
	MsgTruncFile   = 0x00004000 // Truncate file(s)
	MsgKillFile    = 0x00008000 // Delete file(s)
	MsgReceiptReq  = 0x00010000 // Return receipt requested
	MsgConfirmReq  = 0x00020000 // Confirmation receipt requested
	MsgOrphan      = 0x00040000 // Unknown destination
	MsgEncrypt     = 0x00080000 // Encrypted
	MsgCompress    = 0x00100000 // Compressed
	MsgEscaped     = 0x00200000 // Seven bit ASCII
	MsgFPU         = 0x00400000 // Force pickup
	MsgTypeLocal   = 0x00800000 // Local use only
	MsgTypeEcho    = 0x01000000 // Conference/echo mail
	MsgTypeNet     = 0x02000000 // Direct network mail
	MsgNoDisp      = 0x20000000 // May not be displayed
	MsgLocked      = 0x40000000 // Locked
	MsgDeleted     = 0x80000000 // Deleted
)

// Subfield type identifiers from JAM specification
const (
	SfldOAddress     = 0    // Origin network address
	SfldDAddress     = 1    // Destination network address
	SfldSenderName   = 2    // Sender name
	SfldReceiverName = 3    // Receiver name
	SfldMsgID        = 4    // Message ID (FTN MSGID)
	SfldReplyID      = 5    // Reply ID (FTN REPLY)
	SfldSubject      = 6    // Subject
	SfldPID          = 7    // Program ID
	SfldTrace        = 8    // Trace info
	SfldFTSKludge    = 2000 // FTN kludge line
	SfldSeenBy2D     = 2001 // SEEN-BY in 2D format
	SfldPath2D       = 2002 // PATH in 2D format
	SfldFlags        = 2003 // Message flags
	SfldTZUTCInfo    = 2004 // Timezone/UTC info
)

// Sentinel errors
var (
	ErrInvalidSignature = errors.New("jam: invalid JAM signature")
	ErrInvalidMessage   = errors.New("jam: invalid message number")
	ErrBaseNotOpen      = errors.New("jam: message base not open")
	ErrNotFound         = errors.New("jam: not found")
)
