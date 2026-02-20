package usereditor

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/stlalpha/vision3/internal/user"
)

// fieldType defines the edit behavior for a field.
type fieldType int

const (
	ftString  fieldType = iota // Free-text string input
	ftInteger                  // Integer with min/max validation
	ftYesNo                    // Y/N boolean toggle
	ftDisplay                  // Read-only display
	ftAction                   // Special action (e.g., password reset)
)

// fieldDef defines a single editable field on the edit screen.
type fieldDef struct {
	Label   string    // Display label (e.g., "User Handle")
	Type    fieldType // Edit type
	Col     int       // Column position (x) — 3 for left, 50 for right
	Row     int       // Row position (y) — relative to edit area top
	Width   int       // Input field width
	Min     int       // Minimum value (for ftInteger)
	Max     int       // Maximum value (for ftInteger)
	Get     func(u *user.User) string
	Set     func(u *user.User, val string) error
}

// editFields returns the ordered list of editable fields.
// Layout matches UE.PAS v1.3 Proc_Entry: left column (x=3) and right column (x=50).
func editFields() []fieldDef {
	return []fieldDef{
		// Left column (x=3, rows 4-18)
		{
			Label: "Handle", Type: ftString, Col: 3, Row: 4, Width: 22,
			Get: func(u *user.User) string { return u.Handle },
			Set: func(u *user.User, val string) error { u.Handle = val; return nil },
		},
		{
			Label: "Username", Type: ftString, Col: 3, Row: 5, Width: 22,
			Get: func(u *user.User) string { return u.Username },
			Set: func(u *user.User, val string) error { u.Username = val; return nil },
		},
		{
			Label: "Real Name", Type: ftString, Col: 3, Row: 6, Width: 22,
			Get: func(u *user.User) string { return u.RealName },
			Set: func(u *user.User, val string) error { u.RealName = val; return nil },
		},
		{
			Label: "Phone Number", Type: ftString, Col: 3, Row: 7, Width: 15,
			Get: func(u *user.User) string { return u.PhoneNumber },
			Set: func(u *user.User, val string) error { u.PhoneNumber = val; return nil },
		},
		{
			Label: "Access Level", Type: ftInteger, Col: 3, Row: 8, Width: 5, Min: 0, Max: 255,
			Get: func(u *user.User) string { return strconv.Itoa(u.AccessLevel) },
			Set: func(u *user.User, val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				u.AccessLevel = n
				return nil
			},
		},
		{
			Label: "Total Calls", Type: ftInteger, Col: 3, Row: 9, Width: 5, Min: 0, Max: 32767,
			Get: func(u *user.User) string { return strconv.Itoa(u.TimesCalled) },
			Set: func(u *user.User, val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				u.TimesCalled = n
				return nil
			},
		},
		{
			Label: "Group/Location", Type: ftString, Col: 3, Row: 10, Width: 22,
			Get: func(u *user.User) string { return u.GroupLocation },
			Set: func(u *user.User, val string) error { u.GroupLocation = val; return nil },
		},
		{
			Label: "Access Flags", Type: ftString, Col: 3, Row: 11, Width: 22,
			Get: func(u *user.User) string { return u.Flags },
			Set: func(u *user.User, val string) error { u.Flags = strings.ToUpper(val); return nil },
		},
		{
			Label: "Private Note", Type: ftString, Col: 3, Row: 12, Width: 22,
			Get: func(u *user.User) string { return u.PrivateNote },
			Set: func(u *user.User, val string) error { u.PrivateNote = val; return nil },
		},
		{
			Label: "File Points", Type: ftInteger, Col: 3, Row: 13, Width: 5, Min: 0, Max: 32767,
			Get: func(u *user.User) string { return strconv.Itoa(u.FilePoints) },
			Set: func(u *user.User, val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				u.FilePoints = n
				return nil
			},
		},
		{
			Label: "Custom Prompt", Type: ftString, Col: 3, Row: 14, Width: 22,
			Get: func(u *user.User) string { return u.CustomPrompt },
			Set: func(u *user.User, val string) error { u.CustomPrompt = val; return nil },
		},
		{
			Label: "Time Limit", Type: ftInteger, Col: 3, Row: 15, Width: 6, Min: 0, Max: 1440,
			Get: func(u *user.User) string { return strconv.Itoa(u.TimeLimit) },
			Set: func(u *user.User, val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				u.TimeLimit = n
				return nil
			},
		},
		{
			Label: "Password", Type: ftAction, Col: 3, Row: 16, Width: 22,
			Get: func(u *user.User) string {
				if u.PasswordHash == "" {
					return "(not set)"
				}
				return "(set)"
			},
		},

		// Right column (x=50, rows 4-16)
		{
			Label: "Validated", Type: ftYesNo, Col: 50, Row: 4, Width: 1,
			Get: func(u *user.User) string { return boolToYN(u.Validated) },
			Set: func(u *user.User, val string) error { u.Validated = ynToBool(val); return nil },
		},
		{
			Label: "Hot Keys", Type: ftYesNo, Col: 50, Row: 5, Width: 1,
			Get: func(u *user.User) string { return boolToYN(u.HotKeys) },
			Set: func(u *user.User, val string) error { u.HotKeys = ynToBool(val); return nil },
		},
		{
			Label: "More Prompts", Type: ftYesNo, Col: 50, Row: 6, Width: 1,
			Get: func(u *user.User) string { return boolToYN(u.MorePrompts) },
			Set: func(u *user.User, val string) error { u.MorePrompts = ynToBool(val); return nil },
		},
		{
			Label: "Screen Width", Type: ftInteger, Col: 50, Row: 7, Width: 3, Min: 40, Max: 200,
			Get: func(u *user.User) string {
				if u.ScreenWidth == 0 {
					return "80"
				}
				return strconv.Itoa(u.ScreenWidth)
			},
			Set: func(u *user.User, val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				u.ScreenWidth = n
				return nil
			},
		},
		{
			Label: "Screen Height", Type: ftInteger, Col: 50, Row: 8, Width: 3, Min: 24, Max: 60,
			Get: func(u *user.User) string {
				if u.ScreenHeight == 0 {
					return "24"
				}
				return strconv.Itoa(u.ScreenHeight)
			},
			Set: func(u *user.User, val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				u.ScreenHeight = n
				return nil
			},
		},
		{
			Label: "Encoding", Type: ftString, Col: 50, Row: 9, Width: 5,
			Get: func(u *user.User) string { return u.PreferredEncoding },
			Set: func(u *user.User, val string) error { u.PreferredEncoding = val; return nil },
		},
		{
			Label: "Msg Header", Type: ftInteger, Col: 50, Row: 10, Width: 2, Min: 0, Max: 14,
			Get: func(u *user.User) string { return strconv.Itoa(u.MsgHdr) },
			Set: func(u *user.User, val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				u.MsgHdr = n
				return nil
			},
		},
		{
			Label: "Output Mode", Type: ftString, Col: 50, Row: 11, Width: 10,
			Get: func(u *user.User) string { return u.OutputMode },
			Set: func(u *user.User, val string) error { u.OutputMode = val; return nil },
		},
		{
			Label: "Deleted User", Type: ftYesNo, Col: 50, Row: 12, Width: 1,
			Get: func(u *user.User) string { return boolToYN(u.DeletedUser) },
			Set: func(u *user.User, val string) error {
				u.DeletedUser = ynToBool(val)
				if u.DeletedUser && u.DeletedAt == nil {
					now := time.Now()
					u.DeletedAt = &now
				} else if !u.DeletedUser {
					u.DeletedAt = nil
				}
				return nil
			},
		},

		// Row 17: separator rendered by view_edit.go

		// Read-only display fields (rows 18-21, below separator)
		// Left column display fields
		{
			Label: "Num Uploads", Type: ftDisplay, Col: 3, Row: 18, Width: 6,
			Get: func(u *user.User) string { return strconv.Itoa(u.NumUploads) },
		},
		{
			Label: "Msgs Posted", Type: ftDisplay, Col: 3, Row: 19, Width: 6,
			Get: func(u *user.User) string { return strconv.Itoa(u.MessagesPosted) },
		},
		// Right column display fields
		{
			Label: "Created", Type: ftDisplay, Col: 50, Row: 18, Width: 16,
			Get: func(u *user.User) string { return formatTime(u.CreatedAt) },
		},
		{
			Label: "Updated", Type: ftDisplay, Col: 50, Row: 19, Width: 16,
			Get: func(u *user.User) string { return formatTime(u.UpdatedAt) },
		},
		{
			Label: "Last Login", Type: ftDisplay, Col: 50, Row: 20, Width: 16,
			Get: func(u *user.User) string { return formatTime(u.LastLogin) },
		},
		{
			Label: "Bulletin", Type: ftDisplay, Col: 50, Row: 21, Width: 16,
			Get: func(u *user.User) string { return formatTime(u.LastBulletinRead) },
		},
	}
}

// boolToYN converts a bool to "Y" or "N".
func boolToYN(b bool) string {
	if b {
		return "Y"
	}
	return "N"
}

// ynToBool converts "Y"/"y" to true, anything else to false.
func ynToBool(s string) bool {
	return strings.ToUpper(s) == "Y"
}

// formatTime formats a time for display in the editor (compact to fit right column).
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}
	return t.Format("01/02/06 3:04PM")
}

// formatDate formats a time as date only.
func formatDate(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}
	return t.Format("01/02/06")
}

// formatTimeOnly formats a time as time only.
func formatTimeOnly(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("03:04 PM")
}

// padRight pads a string to width with spaces, truncating if longer.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// padLeft pads a string on the left to width.
func padLeft(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// intFieldLabel returns a formatted label with colon for field display.
func intFieldLabel(label string) string {
	return fmt.Sprintf("%s : ", label)
}
