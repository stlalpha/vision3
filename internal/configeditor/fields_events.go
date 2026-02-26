package configeditor

import (
	"strconv"
)

// fieldsEvent returns fields for editing an event.
func (m *Model) fieldsEvent() []fieldDef {
	idx := m.recordEditIdx
	if idx < 0 || idx >= len(m.configs.Events.Events) {
		return nil
	}
	e := &m.configs.Events.Events[idx]
	return []fieldDef{
		{
			Label: "ID", Help: "Unique event identifier", Type: ftString, Col: 3, Row: 1, Width: 20,
			Get: func() string { return e.ID },
			Set: func(val string) error { e.ID = val; return nil },
		},
		{
			Label: "Name", Help: "Display name for this event", Type: ftString, Col: 3, Row: 2, Width: 40,
			Get: func() string { return e.Name },
			Set: func(val string) error { e.Name = val; return nil },
		},
		{
			Label: "Schedule", Help: "Cron expression (e.g. 0 3 * * * for daily at 3am)", Type: ftString, Col: 3, Row: 3, Width: 30,
			Get: func() string { return e.Schedule },
			Set: func(val string) error { e.Schedule = val; return nil },
		},
		{
			Label: "Command", Help: "Command to execute", Type: ftString, Col: 3, Row: 4, Width: 45,
			Get: func() string { return e.Command },
			Set: func(val string) error { e.Command = val; return nil },
		},
		{
			Label: "Working Dir", Help: "Directory to run the command in", Type: ftString, Col: 3, Row: 5, Width: 45,
			Get: func() string { return e.WorkingDirectory },
			Set: func(val string) error { e.WorkingDirectory = val; return nil },
		},
		{
			Label: "Timeout (sec)", Help: "Kill after this many seconds (0=no limit)", Type: ftInteger, Col: 3, Row: 6, Width: 6, Min: 0, Max: 999999,
			Get: func() string { return strconv.Itoa(e.TimeoutSeconds) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				e.TimeoutSeconds = n
				return nil
			},
		},
		{
			Label: "Enabled", Help: "Enable or disable this event", Type: ftYesNo, Col: 3, Row: 7, Width: 1,
			Get: func() string { return boolToYN(e.Enabled) },
			Set: func(val string) error { e.Enabled = ynToBool(val); return nil },
		},
		{
			Label: "Run At Start", Help: "Run when the BBS starts", Type: ftYesNo, Col: 3, Row: 8, Width: 1,
			Get: func() string { return boolToYN(e.RunAtStartup) },
			Set: func(val string) error { e.RunAtStartup = ynToBool(val); return nil },
		},
		{
			Label: "Run After", Help: "Run after this event ID completes", Type: ftString, Col: 3, Row: 9, Width: 20,
			Get: func() string { return e.RunAfter },
			Set: func(val string) error { e.RunAfter = val; return nil },
		},
		{
			Label: "Delay After", Help: "Seconds to wait after dependent event", Type: ftInteger, Col: 3, Row: 10, Width: 6, Min: 0, Max: 999999,
			Get: func() string { return strconv.Itoa(e.DelayAfterSeconds) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				e.DelayAfterSeconds = n
				return nil
			},
		},
	}
}
