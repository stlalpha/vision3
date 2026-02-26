package message

import "time"

// MessageArea defines the structure for a message base/forum.
type MessageArea struct {
	ID           int    `json:"id"`                        // Unique local ID for the area
	Position     int    `json:"position"`                  // Display/sort order (1-based)
	Tag          string `json:"tag"`                       // Short, unique tag (e.g., "GENERAL", "FSX_GEN")
	Name         string `json:"name"`                      // Display name
	Description  string `json:"description"`               // Longer description
	ACSRead      string `json:"acs_read"`                  // ACS string required to read
	ACSWrite     string `json:"acs_write"`                 // ACS string required to post
	AllowAnon    *bool  `json:"allow_anonymous,omitempty"` // Optional: allow anonymous posts (nil defaults to true)
	RealNameOnly bool   `json:"real_name_only,omitempty"`  // Require real name for posts in this area
	ConferenceID int    `json:"conference_id,omitempty"`   // Conference this area belongs to (0=ungrouped)
	BasePath     string `json:"base_path"`                 // Relative path to JAM base (e.g., "msgbases/general")
	MaxMessages  int    `json:"max_messages,omitempty"`    // Max messages to retain (0=unlimited)
	MaxAge       int    `json:"max_age,omitempty"`         // Auto-purge messages older than N days (0=unlimited)
	AutoJoin     bool   `json:"auto_join,omitempty"`       // Auto-join this area for new users
	AreaType     string `json:"area_type"`                 // "local", "echomail", "netmail"
	EchoTag      string `json:"echo_tag,omitempty"`        // FTN echo tag (e.g., "FSX_GEN")
	OriginAddr   string `json:"origin_addr,omitempty"`     // FTN origin address (e.g., "21:3/110")
	Network      string `json:"network,omitempty"`         // FTN network name (e.g., "fsxnet")
	Sponsor      string `json:"sponsor,omitempty"`         // Handle of the area sponsor/moderator
}

// DisplayMessage is a high-level message view for the UI layer.
// It wraps the JAM binary data into a form suitable for display and
// interaction in the message reader/composer.
type DisplayMessage struct {
	MsgNum     int // 1-based message number in the JAM base
	From       string
	To         string
	Subject    string
	DateTime   time.Time
	Body       string // Decoded message body for display
	MsgID      string // FTN MSGID (for reply linking)
	ReplyID    string // FTN REPLYID (message this replies to)
	ReplyToNum int    // JAM ReplyTo: message number of parent (0 = none)
	OrigAddr   string // FTN origin address
	DestAddr   string // FTN destination address
	Attributes uint32 // JAM message attribute flags
	IsPrivate  bool
	IsDeleted  bool
	AreaID     int // Area this message belongs to
}

// Constants for standard message fields.
const (
	MsgToUserAll = "All" // Standard To value for public messages
)
