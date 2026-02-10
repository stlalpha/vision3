package tosser

// Config holds FTN tosser configuration.
type Config struct {
	Enabled      bool         `json:"enabled"`
	OwnAddress   string       `json:"own_address"`           // e.g., "21:3/110"
	InboundPath  string       `json:"inbound_path"`          // e.g., "data/ftn/inbound"
	OutboundPath string       `json:"outbound_path"`         // e.g., "data/ftn/outbound"
	TempPath     string       `json:"temp_path"`             // e.g., "data/ftn/temp"
	DupeDBPath   string       `json:"dupe_db_path"`          // e.g., "data/ftn/dupes.json"
	PollSeconds  int          `json:"poll_interval_seconds"` // 0 = manual only
	Links        []LinkConfig `json:"links"`
}

// LinkConfig defines an FTN link (uplink/downlink node).
type LinkConfig struct {
	Address   string   `json:"address"`    // e.g., "21:1/100"
	Password  string   `json:"password"`   // Packet password
	Name      string   `json:"name"`       // Human-readable name
	EchoAreas []string `json:"echo_areas"` // Echo tags routed to this link
}
