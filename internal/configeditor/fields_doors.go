package configeditor

import (
	"strings"

	"github.com/stlalpha/vision3/internal/config"
)

// sliceToCSV joins a string slice with ", " for display.
func sliceToCSV(s []string) string {
	return strings.Join(s, ", ")
}

// csvToSlice splits a comma-separated string into a trimmed string slice.
// Returns nil for empty input.
func csvToSlice(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// doorCommandsGet returns the command string for display.
// Native doors: "command arg1, arg2, ..."
// DOS doors: "cmd1, cmd2, ..." from DOSCommands
func doorCommandsGet(d *doorEditProxy) string {
	if d.IsDOS {
		return sliceToCSV(d.DOSCommands)
	}
	if d.Command == "" {
		return ""
	}
	if len(d.Args) == 0 {
		return d.Command
	}
	return d.Command + " " + sliceToCSV(d.Args)
}

// doorCommandsSet parses the command string back into the appropriate fields.
// Native doors: first token is command, rest are comma-separated args.
// DOS doors: comma-separated list goes into DOSCommands.
func doorCommandsSet(d *doorEditProxy, val string) {
	if d.IsDOS {
		d.DOSCommands = csvToSlice(val)
		d.Command = ""
		d.Args = nil
		return
	}
	val = strings.TrimSpace(val)
	if val == "" {
		d.Command = ""
		d.Args = nil
		return
	}
	// Split first space-separated token as the command
	parts := strings.SplitN(val, " ", 2)
	d.Command = parts[0]
	if len(parts) > 1 {
		d.Args = csvToSlice(parts[1])
	} else {
		d.Args = nil
	}
}

// doorEditProxy wraps DoorConfig fields for in-place editing via closures.
type doorEditProxy = config.DoorConfig

// fieldsDoor returns fields for editing a door program.
// Fields shown depend on whether the door is DOS or native.
func (m *Model) fieldsDoor() []fieldDef {
	keys := m.doorKeys()
	idx := m.recordEditIdx
	if idx < 0 || idx >= len(keys) {
		return nil
	}
	key := keys[idx]
	d := m.configs.Doors[key]
	dPtr := &d

	// Store back closure to update the map entry
	save := func() {
		m.configs.Doors[key] = *dPtr
	}

	row := 1
	fields := []fieldDef{
		{
			Label: "Name", Help: "Door name used in DOOR:NAME menu commands", Type: ftString, Col: 3, Row: row, Width: 30,
			Get: func() string { return dPtr.Name },
			Set: func(val string) error { dPtr.Name = val; save(); return nil },
		},
	}

	row++
	fields = append(fields, fieldDef{
		Label: "Is DOS", Help: "Y=DOS door via dosemu2, N=native Linux program", Type: ftYesNo, Col: 3, Row: row, Width: 1,
		Get: func() string { return boolToYN(dPtr.IsDOS) },
		Set: func(val string) error { dPtr.IsDOS = ynToBool(val); save(); return nil },
	})

	row++
	fields = append(fields, fieldDef{
		Label: "Commands", Help: "Native: command args / DOS: comma-separated DOS commands", Type: ftString, Col: 3, Row: row, Width: 45,
		Get: func() string { return doorCommandsGet(dPtr) },
		Set: func(val string) error { doorCommandsSet(dPtr, val); save(); return nil },
	})

	row++
	fields = append(fields, fieldDef{
		Label: "Dropfile Type", Help: "DOOR.SYS, DOOR32.SYS, CHAIN.TXT, DORINFO1.DEF, or NONE", Type: ftString, Col: 3, Row: row, Width: 15,
		Get: func() string { return dPtr.DropfileType },
		Set: func(val string) error { dPtr.DropfileType = val; save(); return nil },
	})

	if dPtr.IsDOS {
		// DOS-specific fields
		row++
		fields = append(fields, fieldDef{
			Label: "Drive C Path", Help: "dosemu drive_c path (blank=~/.dosemu/drive_c)", Type: ftString, Col: 3, Row: row, Width: 45,
			Get: func() string { return dPtr.DriveCPath },
			Set: func(val string) error { dPtr.DriveCPath = val; save(); return nil },
		})
		row++
		fields = append(fields, fieldDef{
			Label: "DOSemu Path", Help: "Path to dosemu binary (blank=/usr/bin/dosemu)", Type: ftString, Col: 3, Row: row, Width: 45,
			Get: func() string { return dPtr.DosemuPath },
			Set: func(val string) error { dPtr.DosemuPath = val; save(); return nil },
		})
		row++
		fields = append(fields, fieldDef{
			Label: "DOSemu Config", Help: "Custom .dosemurc config file (optional)", Type: ftString, Col: 3, Row: row, Width: 45,
			Get: func() string { return dPtr.DosemuConfig },
			Set: func(val string) error { dPtr.DosemuConfig = val; save(); return nil },
		})
	} else {
		// Native-specific fields
		row++
		fields = append(fields, fieldDef{
			Label: "Working Dir", Help: "Directory to run the command in", Type: ftString, Col: 3, Row: row, Width: 45,
			Get: func() string { return dPtr.WorkingDirectory },
			Set: func(val string) error { dPtr.WorkingDirectory = val; save(); return nil },
		})
		row++
		fields = append(fields, fieldDef{
			Label: "I/O Mode", Help: "I/O handling mode (STDIO)", Type: ftString, Col: 3, Row: row, Width: 15,
			Get: func() string { return dPtr.IOMode },
			Set: func(val string) error { dPtr.IOMode = val; save(); return nil },
		})
		row++
		fields = append(fields, fieldDef{
			Label: "Raw Terminal", Help: "Allocate PTY for raw terminal I/O", Type: ftYesNo, Col: 3, Row: row, Width: 1,
			Get: func() string { return boolToYN(dPtr.RequiresRawTerminal) },
			Set: func(val string) error { dPtr.RequiresRawTerminal = ynToBool(val); save(); return nil },
		})
	}

	return fields
}
