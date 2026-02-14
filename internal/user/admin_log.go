package user

import "time"

// AdminActivityLog records all administrative actions on user accounts.
//
// SECURITY/PRIVACY NOTICE:
// - OldValue and NewValue may contain user PII (name, email, phone, real name, address)
// - Ensure admin_activity.json has restrictive file permissions (0600)
// - Implement appropriate data retention policies per GDPR/CCPA requirements
// - Consider redacting or hashing sensitive fields for audit purposes
type AdminActivityLog struct {
	ID            int       `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	AdminUsername string    `json:"adminUsername"` // Admin who made the change
	AdminID       int       `json:"adminId"`       // Admin user ID
	TargetUserID  int       `json:"targetUserId"`  // User being modified
	TargetHandle  string    `json:"targetHandle"`  // Handle of user being modified
	Action        string    `json:"action"`        // Type of action (e.g., "EDIT_USER", "BAN_USER", "DELETE_USER")
	FieldName     string    `json:"fieldName"`     // Field that was changed (for edits)
	OldValue      string    `json:"oldValue"`      // Previous value - may contain PII
	NewValue      string    `json:"newValue"`      // New value - may contain PII
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
