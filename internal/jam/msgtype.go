package jam

import "strings"

// MessageType represents the type of message being created.
type MessageType int

const (
	MsgTypeLocalMsg   MessageType = iota // Local BBS-only message
	MsgTypeEchomailMsg                   // FTN conference/echo message
	MsgTypeNetmailMsg                    // FTN direct network mail
)

// IsEchomail reports whether this is an echomail message.
func (mt MessageType) IsEchomail() bool { return mt == MsgTypeEchomailMsg }

// IsNetmail reports whether this is a netmail message.
func (mt MessageType) IsNetmail() bool { return mt == MsgTypeNetmailMsg }

// IsLocal reports whether this is a local message.
func (mt MessageType) IsLocal() bool { return mt == MsgTypeLocalMsg }

// GetJAMAttribute returns the JAM attribute flags for this message type.
func (mt MessageType) GetJAMAttribute() uint32 {
	switch mt {
	case MsgTypeEchomailMsg:
		return MsgLocal | MsgTypeEcho
	case MsgTypeNetmailMsg:
		return MsgLocal | MsgTypeNet
	default:
		return MsgLocal | MsgTypeLocal
	}
}

// DetermineMessageType returns the MessageType based on area configuration.
func DetermineMessageType(areaType, echoTag string) MessageType {
	switch strings.ToLower(strings.TrimSpace(areaType)) {
	case "echo", "echomail":
		return MsgTypeEchomailMsg
	case "netmail", "direct":
		return MsgTypeNetmailMsg
	default:
		return MsgTypeLocalMsg
	}
}
