package configeditor

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
)

// joinArgs converts an array of arguments into a JSON-encoded string.
// This preserves arguments with spaces, quotes, and special characters without corruption.
// Empty arrays return an empty string.
func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	// Encode as JSON array for lossless round-trip
	data, err := json.Marshal(args)
	if err != nil {
		log.Printf("WARN: Failed to encode args as JSON: %v", err)
		return ""
	}
	return string(data)
}

// splitArgs parses a JSON-encoded string into an array of arguments.
// Falls back to simple space-splitting for backward compatibility with old configs.
// Empty strings return an empty array.
func splitArgs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}

	// Try JSON decoding first
	var args []string
	if err := json.Unmarshal([]byte(s), &args); err == nil {
		return args
	}

	// Fallback: treat as space-separated for backward compatibility
	// (simple split, no quote handling - users should re-save to upgrade to JSON)
	return strings.Fields(s)
}

// fieldsProtocol returns fields for editing a transfer protocol.
func (m *Model) fieldsProtocol() []fieldDef {
	idx := m.recordEditIdx
	if idx < 0 || idx >= len(m.configs.Protocols) {
		return nil
	}
	p := &m.configs.Protocols[idx]
	return []fieldDef{
		{
			Label: "Key", Help: "Selection key shown in protocol menu", Type: ftString, Col: 3, Row: 1, Width: 10,
			Get: func() string { return p.Key },
			Set: func(val string) error { p.Key = val; return nil },
		},
		{
			Label: "Name", Help: "Protocol display name", Type: ftString, Col: 3, Row: 2, Width: 30,
			Get: func() string { return p.Name },
			Set: func(val string) error { p.Name = val; return nil },
		},
		{
			Label: "Description", Help: "Longer description of this protocol", Type: ftString, Col: 3, Row: 3, Width: 45,
			Get: func() string { return p.Description },
			Set: func(val string) error { p.Description = val; return nil },
		},
		{
			Label: "Send Command", Help: "External command for sending files", Type: ftString, Col: 3, Row: 4, Width: 30,
			Get: func() string { return p.SendCmd },
			Set: func(val string) error { p.SendCmd = val; return nil },
		},
		{
			Label: "Send Args", Help: "Arguments for send command (space-separated, use quotes for spaces)", Type: ftString, Col: 3, Row: 5, Width: 40,
			Get: func() string { return joinArgs(p.SendArgs) },
			Set: func(val string) error {
				p.SendArgs = splitArgs(val)
				return nil
			},
		},
		{
			Label: "Recv Command", Help: "External command for receiving files", Type: ftString, Col: 3, Row: 6, Width: 30,
			Get: func() string { return p.RecvCmd },
			Set: func(val string) error { p.RecvCmd = val; return nil },
		},
		{
			Label: "Recv Args", Help: "Arguments for receive command (space-separated, use quotes for spaces)", Type: ftString, Col: 3, Row: 7, Width: 40,
			Get: func() string { return joinArgs(p.RecvArgs) },
			Set: func(val string) error {
				p.RecvArgs = splitArgs(val)
				return nil
			},
		},
		{
			Label: "Batch Send", Help: "Supports sending multiple files at once", Type: ftYesNo, Col: 3, Row: 8, Width: 1,
			Get: func() string { return boolToYN(p.BatchSend) },
			Set: func(val string) error { p.BatchSend = ynToBool(val); return nil },
		},
		{
			Label: "Use PTY", Help: "Allocate PTY for protocol I/O", Type: ftYesNo, Col: 3, Row: 9, Width: 1,
			Get: func() string { return boolToYN(p.UsePTY) },
			Set: func(val string) error { p.UsePTY = ynToBool(val); return nil },
		},
		{
			Label: "Default", Help: "Set as the default transfer protocol", Type: ftYesNo, Col: 3, Row: 10, Width: 1,
			Get: func() string { return boolToYN(p.Default) },
			Set: func(val string) error { p.Default = ynToBool(val); return nil },
		},
		{
			Label: "Conn Type", Help: "Connection filter: ssh, telnet, or blank=all", Type: ftString, Col: 3, Row: 11, Width: 10,
			Get: func() string { return p.ConnectionType },
			Set: func(val string) error { p.ConnectionType = val; return nil },
		},
	}
}

// fieldsArchiver returns fields for editing an archiver.
func (m *Model) fieldsArchiver() []fieldDef {
	idx := m.recordEditIdx
	if idx < 0 || idx >= len(m.configs.Archivers.Archivers) {
		return nil
	}
	a := &m.configs.Archivers.Archivers[idx]
	return []fieldDef{
		{
			Label: "ID", Help: "Unique archiver identifier", Type: ftString, Col: 3, Row: 1, Width: 10,
			Get: func() string { return a.ID },
			Set: func(val string) error { a.ID = val; return nil },
		},
		{
			Label: "Name", Help: "Archiver display name", Type: ftString, Col: 3, Row: 2, Width: 30,
			Get: func() string { return a.Name },
			Set: func(val string) error { a.Name = val; return nil },
		},
		{
			Label: "Extension", Help: "File extension (e.g. .zip, .arj, .lha)", Type: ftString, Col: 3, Row: 3, Width: 10,
			Get: func() string { return a.Extension },
			Set: func(val string) error { a.Extension = val; return nil },
		},
		{
			Label: "Magic Bytes", Help: "Hex signature for auto-detection (e.g. 504B0304)", Type: ftString, Col: 3, Row: 4, Width: 20,
			Get: func() string { return a.Magic },
			Set: func(val string) error { a.Magic = val; return nil },
		},
		{
			Label: "Native", Help: "Use Go native implementation (no external cmd)", Type: ftYesNo, Col: 3, Row: 5, Width: 1,
			Get: func() string { return boolToYN(a.Native) },
			Set: func(val string) error { a.Native = ynToBool(val); return nil },
		},
		{
			Label: "Enabled", Help: "Enable or disable this archiver", Type: ftYesNo, Col: 3, Row: 6, Width: 1,
			Get: func() string { return boolToYN(a.Enabled) },
			Set: func(val string) error { a.Enabled = ynToBool(val); return nil },
		},
		{
			Label: "Pack Cmd", Help: "Command to create archives", Type: ftString, Col: 3, Row: 7, Width: 30,
			Get: func() string { return a.Pack.Command },
			Set: func(val string) error { a.Pack.Command = val; return nil },
		},
		{
			Label: "Pack Args", Help: "Args for pack (space-sep, use {ARCHIVE} {FILES})", Type: ftString, Col: 3, Row: 8, Width: 40,
			Get: func() string { return joinArgs(a.Pack.Args) },
			Set: func(val string) error {
				a.Pack.Args = splitArgs(val)
				return nil
			},
		},
		{
			Label: "Unpack Cmd", Help: "Command to extract archives", Type: ftString, Col: 3, Row: 9, Width: 30,
			Get: func() string { return a.Unpack.Command },
			Set: func(val string) error { a.Unpack.Command = val; return nil },
		},
		{
			Label: "Unpack Args", Help: "Args for unpack (space-sep, use {ARCHIVE} {OUTDIR})", Type: ftString, Col: 3, Row: 10, Width: 40,
			Get: func() string { return joinArgs(a.Unpack.Args) },
			Set: func(val string) error {
				a.Unpack.Args = splitArgs(val)
				return nil
			},
		},
		{
			Label: "Test Cmd", Help: "Command to test archive integrity", Type: ftString, Col: 3, Row: 11, Width: 30,
			Get: func() string { return a.Test.Command },
			Set: func(val string) error { a.Test.Command = val; return nil },
		},
		{
			Label: "Test Args", Help: "Args for test (space-sep, use {ARCHIVE})", Type: ftString, Col: 3, Row: 12, Width: 40,
			Get: func() string { return joinArgs(a.Test.Args) },
			Set: func(val string) error {
				a.Test.Args = splitArgs(val)
				return nil
			},
		},
		{
			Label: "List Cmd", Help: "Command to list archive contents", Type: ftString, Col: 3, Row: 13, Width: 30,
			Get: func() string { return a.List.Command },
			Set: func(val string) error { a.List.Command = val; return nil },
		},
		{
			Label: "List Args", Help: "Args for list (space-sep, use {ARCHIVE})", Type: ftString, Col: 3, Row: 14, Width: 40,
			Get: func() string { return joinArgs(a.List.Args) },
			Set: func(val string) error {
				a.List.Args = splitArgs(val)
				return nil
			},
		},
		{
			Label: "Comment Cmd", Help: "Command to add comment (optional)", Type: ftString, Col: 3, Row: 15, Width: 30,
			Get: func() string { return a.Comment.Command },
			Set: func(val string) error { a.Comment.Command = val; return nil },
		},
		{
			Label: "Comment Args", Help: "Args for comment (space-sep, use {ARCHIVE} {FILE})", Type: ftString, Col: 3, Row: 16, Width: 40,
			Get: func() string { return joinArgs(a.Comment.Args) },
			Set: func(val string) error {
				a.Comment.Args = splitArgs(val)
				return nil
			},
		},
		{
			Label: "AddFile Cmd", Help: "Command to add file to archive (optional)", Type: ftString, Col: 3, Row: 17, Width: 30,
			Get: func() string { return a.AddFile.Command },
			Set: func(val string) error { a.AddFile.Command = val; return nil },
		},
		{
			Label: "AddFile Args", Help: "Args for addFile (space-sep, use {ARCHIVE} {FILE})", Type: ftString, Col: 3, Row: 18, Width: 40,
			Get: func() string { return joinArgs(a.AddFile.Args) },
			Set: func(val string) error {
				a.AddFile.Args = splitArgs(val)
				return nil
			},
		},
	}
}

// fieldsLogin returns fields for editing a login sequence item.
func (m *Model) fieldsLogin() []fieldDef {
	idx := m.recordEditIdx
	if idx < 0 || idx >= len(m.configs.LoginSeq) {
		return nil
	}
	l := &m.configs.LoginSeq[idx]
	return []fieldDef{
		{
			Label: "Command", Help: "Login step: DISPLAY, PAUSE, MATRIX, etc.", Type: ftString, Col: 3, Row: 1, Width: 20,
			Get: func() string { return l.Command },
			Set: func(val string) error { l.Command = val; return nil },
		},
		{
			Label: "Data", Help: "Command argument (filename, text, etc.)", Type: ftString, Col: 3, Row: 2, Width: 45,
			Get: func() string { return l.Data },
			Set: func(val string) error { l.Data = val; return nil },
		},
		{
			Label: "Clear Screen", Help: "Clear screen before this step", Type: ftYesNo, Col: 3, Row: 3, Width: 1,
			Get: func() string { return boolToYN(l.ClearScreen) },
			Set: func(val string) error { l.ClearScreen = ynToBool(val); return nil },
		},
		{
			Label: "Pause After", Help: "Wait for keypress after this step", Type: ftYesNo, Col: 3, Row: 4, Width: 1,
			Get: func() string { return boolToYN(l.PauseAfter) },
			Set: func(val string) error { l.PauseAfter = ynToBool(val); return nil },
		},
		{
			Label: "Sec Level", Help: "Minimum security level to show this step (0=all)", Type: ftInteger, Col: 3, Row: 5, Width: 3, Min: 0, Max: 255,
			Get: func() string { return strconv.Itoa(l.SecLevel) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				l.SecLevel = n
				return nil
			},
		},
	}
}
