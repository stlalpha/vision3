package configeditor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// sanitizeEventID converts a display name into a valid event ID.
// Example: "Daily Maintenance Task" -> "daily_maintenance_task"
func sanitizeEventID(name string) string {
	// Convert to lowercase
	id := strings.ToLower(name)
	// Replace spaces and special chars with underscores
	reg := regexp.MustCompile(`[^a-z0-9_]+`)
	id = reg.ReplaceAllString(id, "_")
	// Remove leading/trailing underscores
	id = strings.Trim(id, "_")
	// Collapse multiple underscores
	reg = regexp.MustCompile(`_+`)
	id = reg.ReplaceAllString(id, "_")
	return id
}

// fieldsEvent returns fields for editing an event.
func (m *Model) fieldsEvent() []fieldDef {
	idx := m.recordEditIdx
	if idx < 0 || idx >= len(m.configs.Events.Events) {
		return nil
	}
	e := &m.configs.Events.Events[idx]
	return []fieldDef{
		{
			Label: "Name", Help: "Display name for this event", Type: ftString, Col: 3, Row: 1, Width: 40,
			Get: func() string { return e.Name },
			Set: func(val string) error {
				oldID := e.ID
				e.Name = val
				newID := sanitizeEventID(val)
				// Only update if ID actually changed
				if oldID != newID {
					e.ID = newID
					// Update all RunAfter references in other events
					for i := range m.configs.Events.Events {
						if i != idx && m.configs.Events.Events[i].RunAfter == oldID {
							m.configs.Events.Events[i].RunAfter = newID
						}
					}
				}
				return nil
			},
		},
		{
			Label: "Schedule", Help: "Cron expression (e.g. 0 3 * * * for daily at 3am)", Type: ftString, Col: 3, Row: 2, Width: 30,
			Get: func() string { return e.Schedule },
			Set: func(val string) error { e.Schedule = val; return nil },
		},
		{
			Label: "Command", Help: "Command to execute", Type: ftString, Col: 3, Row: 3, Width: 45,
			Get: func() string { return e.Command },
			Set: func(val string) error { e.Command = val; return nil },
		},
		{
			Label: "Working Dir", Help: "Directory to run the command in", Type: ftString, Col: 3, Row: 4, Width: 45,
			Get: func() string { return e.WorkingDirectory },
			Set: func(val string) error { e.WorkingDirectory = val; return nil },
		},
		{
			Label: "Timeout (sec)", Help: "Kill after this many seconds (0=no limit)", Type: ftInteger, Col: 3, Row: 5, Width: 6, Min: 0, Max: 999999,
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
			Label: "Enabled", Help: "Enable or disable this event", Type: ftYesNo, Col: 3, Row: 6, Width: 1,
			Get: func() string { return boolToYN(e.Enabled) },
			Set: func(val string) error { e.Enabled = ynToBool(val); return nil },
		},
		{
			Label: "Run At Start", Help: "Run when the BBS starts", Type: ftYesNo, Col: 3, Row: 7, Width: 1,
			Get: func() string { return boolToYN(e.RunAtStartup) },
			Set: func(val string) error { e.RunAtStartup = ynToBool(val); return nil },
		},
		{
			Label: "Run After", Help: "Run after this event completes (press Enter to select)", Type: ftLookup, Col: 3, Row: 8, Width: 20,
			Get: func() string {
				if e.RunAfter == "" {
					return "(none)"
				}
				return e.RunAfter
			},
			Set: func(val string) error {
				if val == "(none)" {
					e.RunAfter = ""
				} else {
					e.RunAfter = val
				}
				return nil
			},
			LookupItems: func() []LookupItem {
				items := []LookupItem{
					{Value: "(none)", Display: "(none) - No dependency"},
				}
				// Add all other events as options (exclude current event)
				for i, evt := range m.configs.Events.Events {
					if i != idx {
						items = append(items, LookupItem{
							Value:   evt.ID,
							Display: fmt.Sprintf("%s - %s", evt.ID, evt.Name),
						})
					}
				}
				return items
			},
		},
		{
			Label: "Delay After", Help: "Seconds to wait after dependent event", Type: ftInteger, Col: 3, Row: 9, Width: 6, Min: 0, Max: 999999,
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
