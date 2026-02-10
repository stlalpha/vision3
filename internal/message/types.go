package message

import (
	"time"

	"github.com/google/uuid"
)

// MessageArea defines the structure for a message base/forum.
type MessageArea struct {
	ID            int    `json:"id"`                        // Unique local ID for the area
	Tag           string `json:"tag"`                       // Short, unique tag (e.g., "GENERAL", "SYSOP", "NETWORK_TAG")
	Name          string `json:"name"`                      // Display name of the area
	Description   string `json:"description"`               // Longer description
	ACSRead       string `json:"acs_read"`                  // ACS string required to read messages
	ACSWrite      string `json:"acs_write"`                 // ACS string required to post messages
	IsNetworked   bool   `json:"is_networked"`              // Flag indicating if this area is networked (echomail)
	OriginNodeID  string `json:"origin_node_id"`            // ID of the node where this networked area originated
	LastMessageID string `json:"last_message_id,omitempty"` // Optional: ID of the last message posted (for high water mark)
	ConferenceID  int    `json:"conference_id,omitempty"`   // Conference this area belongs to (0=ungrouped)
}

// Message defines the structure for a single message, usable for both public posts and private mail.
type Message struct {
	ID           uuid.UUID `json:"id"`                     // Globally unique identifier (UUID)
	AreaID       int       `json:"area_id"`                // Local ID of the MessageArea this belongs to
	FromUserName string    `json:"from_user_name"`         // Handle of the sender
	FromNodeID   string    `json:"from_node_id"`           // Node ID where the message originated
	ToUserName   string    `json:"to_user_name,omitempty"` // Target user handle (used for private mail)
	ToNodeID     string    `json:"to_node_id,omitempty"`   // Target node ID (used for private mail)
	Subject      string    `json:"subject"`                // Message subject
	Body         string    `json:"body"`                   // Message content
	PostedAt     time.Time `json:"posted_at"`              // Timestamp when the message was originally posted
	MsgID        string    `json:"msg_id,omitempty"`       // Optional: FidoNet-style MSGID (OriginID + Serial) - may be redundant with ID
	ReplyToID    uuid.UUID `json:"reply_to_id,omitempty"`  // ID (UUID) of the message this is a reply to (use uuid.Nil for no reply)
	Path         []string  `json:"path,omitempty"`         // List of node IDs this message traversed (for network loop prevention)
	IsPrivate    bool      `json:"is_private"`             // True for private mail, False for public posts
	ReadBy       []int     `json:"read_by,omitempty"`      // Optional: List of User IDs who have read this (for tracking new mail/posts) - can get large!
	Attributes   []string  `json:"attributes,omitempty"`   // Optional: FidoNet-style attributes (e.g., "PVT", "CRASH", "RCVD")
}

// Constants for standard message fields
const (
	MsgToUserAll = "All" // Standard ToUserName for public messages
)
