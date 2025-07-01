package menu

// CommandRecord holds the definition of a single command from a .CFG file.
type CommandRecord struct {
	Keys    string `json:"KEYS"`              // Input key(s) to trigger command (space-separated)
	Command string `json:"CMD"`               // Command string (e.g., GOTO:MENU, RUN:PROG, LOGOFF)
	ACS     string `json:"ACS"`               // Access Control String
	Hidden  bool   `json:"HIDDEN"`            // Whether the command is hidden (H flag)
	AutoRun string `json:"AUTORUN,omitempty"` // Type of auto-run (e.g., "ONCE_PER_SESSION")
}

// GetHidden is a helper method to safely access the Hidden field.
// (Kept for potential future use, though direct access is fine)
func (cr *CommandRecord) GetHidden() bool {
	return cr.Hidden
}
