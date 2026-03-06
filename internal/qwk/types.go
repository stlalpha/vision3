package qwk

import "time"

// QWK format constants.
const (
	BlockSize      = 128 // QWK messages are stored in 128-byte blocks
	HeaderBlocks   = 1   // The message header occupies exactly 1 block
	MaxConfNumber  = 65535
	StatusPublic   = ' '
	StatusPrivate  = '*'
	StatusReceived = '-' // Private message that has been read
)

// ConferenceInfo describes a conference (message area) for CONTROL.DAT.
type ConferenceInfo struct {
	Number int    // QWK conference number (area ID)
	Name   string // Display name
}

// PacketMessage is a message to be packed into a QWK MESSAGES.DAT.
type PacketMessage struct {
	Conference int
	Number     int // Message number in the base
	From       string
	To         string
	Subject    string
	DateTime   time.Time
	Body       string
	Private    bool
}

// REPMessage is a message extracted from an uploaded REP packet.
type REPMessage struct {
	Conference int
	To         string
	Subject    string
	Body       string
}
