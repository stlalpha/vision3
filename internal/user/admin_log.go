package user

import "time"

// AdminActivityLog records all administrative actions on user accounts
type AdminActivityLog struct {
	ID            int       `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	AdminUsername string    `json:"adminUsername"` // Admin who made the change
	AdminID       int       `json:"adminId"`       // Admin user ID
	TargetUserID  int       `json:"targetUserId"`  // User being modified
	TargetHandle  string    `json:"targetHandle"`  // Handle of user being modified
	Action        string    `json:"action"`        // Type of action (e.g., "EDIT_USER", "BAN_USER", "DELETE_USER")
	FieldName     string    `json:"fieldName"`     // Field that was changed (for edits)
	OldValue      string    `json:"oldValue"`      // Previous value
	NewValue      string    `json:"newValue"`      // New value
	Notes         string    `json:"notes"`         // Optional notes/reason
}

// AdminActivityLogEntry creates a formatted log entry for a single field change
func AdminActivityLogEntry(adminUsername string, adminID int, targetUserID int, targetHandle string, fieldName string, oldValue string, newValue string) AdminActivityLog {
	return AdminActivityLog{
		Timestamp:     time.Now(),
		AdminUsername: adminUsername,
		AdminID:       adminID,
		TargetUserID:  targetUserID,
		TargetHandle:  targetHandle,
		Action:        "EDIT_USER",
		FieldName:     fieldName,
		OldValue:      oldValue,
		NewValue:      newValue,
	}
}
