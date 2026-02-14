package file

import (
	"time"

	"github.com/google/uuid"
)

// FileArea defines a logical grouping or directory for files.
type FileArea struct {
	ID          int    `json:"id"`
	Tag         string `json:"tag"`  // e.g., "UTILS", "TEXTS" (Unique, uppercase)
	Name        string `json:"name"` // e.g., "Utility Programs"
	Description string `json:"description"`
	Path        string `json:"path"`         // Server filesystem path (relative to a base path, e.g., "utils")
	ACSList     string `json:"acs_list"`     // ACS to list files in this area
	ACSUpload   string `json:"acs_upload"`   // ACS to upload to this area
	ACSDownload  string `json:"acs_download"`              // ACS to download from this area
	ConferenceID int    `json:"conference_id,omitempty"` // Conference this area belongs to (0=ungrouped)
}

// FileRecord holds metadata about a specific file within a FileArea.
type FileRecord struct {
	ID            uuid.UUID `json:"id"`          // Unique ID for the record
	AreaID        int       `json:"area_id"`     // Link to FileArea.ID
	Filename      string    `json:"filename"`    // Actual filename on disk (basename)
	Description   string    `json:"description"` // User-provided description
	Size          int64     `json:"size"`        // File size in bytes
	UploadedAt    time.Time `json:"uploaded_at"`
	UploadedBy    string    `json:"uploaded_by"` // User Handle
	DownloadCount int       `json:"download_count"`
	// TODO: Add []string Tags for keyword tagging later if needed
	// TODO: Add hash?
}
