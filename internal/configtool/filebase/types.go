package filebase

// No imports needed

// Binary file record structure - classic .DIR format style
// Similar to PCBoard DIR files, TriBBS, and other classic BBS file databases
type FileRecord struct {
	FileName     [13]byte  // 8.3 filename + null terminator
	Description  [46]byte  // Short file description (null-terminated)
	Size         uint32    // File size in bytes
	Date         [8]byte   // Upload date MM-DD-YY (null-terminated)  
	Time         [8]byte   // Upload time HH:MM:SS (null-terminated)
	Uploader     [26]byte  // Name of uploader (null-terminated)
	DownloadCount uint16   // Number of times downloaded
	Flags        uint8     // File flags (see FileFlag constants)
	Rating       uint8     // File rating (0-10)
	Cost         uint16    // Download cost in credits/time
	Reserved     [8]byte   // Reserved for future use
} // Total: 128 bytes (classic DIR record size)

// Extended file record for additional metadata
type ExtendedFileRecord struct {
	FileRecord              // Embed basic record
	LongDesc   [256]byte    // Extended description (null-terminated)
	Keywords   [81]byte     // Keywords/tags (null-terminated)
	Category   [21]byte     // Category/section (null-terminated)
	UploaderID uint16       // Uploader user ID
	ValidateBy [26]byte     // Validated by (sysop name)
	ValidateDate [20]byte   // Validation date/time
	MD5Hash    [33]byte     // MD5 hash of file (null-terminated)
	CRC32      uint32       // CRC32 checksum
	LastAccess [20]byte     // Last download date/time
	Reserved2  [32]byte     // Additional reserved space
} // Total: 512 bytes

// File area configuration (binary format)
type FileAreaConfig struct {
	AreaNum      uint16     // Area number (1-based)
	AreaTag      [13]byte   // Area tag (null-terminated)
	AreaName     [41]byte   // Area name (null-terminated) 
	Description  [81]byte   // Area description (null-terminated)
	Path         [128]byte  // Filesystem path (null-terminated)
	UploadPath   [128]byte  // Upload path (null-terminated)
	MaxFiles     uint16     // Maximum files (0 = unlimited)
	MaxSize      uint32     // Maximum total size in KB (0 = unlimited)
	ReadLevel    uint16     // Security level required to list
	DownloadLevel uint16    // Security level required to download
	UploadLevel  uint16     // Security level required to upload
	Flags        uint32     // Area flags (see AreaFlag constants)
	TotalFiles   uint16     // Current number of files
	TotalSize    uint32     // Current total size in bytes
	TotalDowns   uint32     // Total downloads from this area
	SortOrder    uint8      // Default sort order (see SortOrder constants)
	Reserved     [32]byte   // Reserved for future expansion
} // Total: 512 bytes

// File flags (stored in FileRecord.Flags)
const (
	FileFlagActive     = 0x01 // File is active (not deleted)
	FileFlagValidated  = 0x02 // File has been validated by sysop
	FileFlagFree       = 0x04 // File is free to download
	FileFlagNew        = 0x08 // File is marked as new
	FileFlagPrivate    = 0x10 // Private file (uploader only)
	FileFlagOffline    = 0x20 // File is temporarily offline
	FileFlagCrashed    = 0x40 // File failed upload/validation
	FileFlagSpecial    = 0x80 // Special file (featured/promoted)
)

// Area flags (stored in FileAreaConfig.Flags)
const (
	AreaFlagPublic      = 0x00000001 // Public file area
	AreaFlagUpload      = 0x00000002 // Uploads allowed
	AreaFlagDownload    = 0x00000004 // Downloads allowed
	AreaFlagBrowse      = 0x00000008 // Archive browsing allowed
	AreaFlagComment     = 0x00000010 // Comments allowed
	AreaFlagAutoValidate = 0x00000020 // Auto-validate uploads
	AreaFlagRatio       = 0x00000040 // Upload/download ratio enforced
	AreaFlagCredits     = 0x00000080 // Download credits required
	AreaFlagPassword    = 0x00000100 // Password protected area
	AreaFlagHidden      = 0x00000200 // Hidden from listings
	AreaFlagCDROM       = 0x00000400 // CDROM/read-only area
	AreaFlagDuplicate   = 0x00000800 // Allow duplicate files
)

// Sort order constants
const (
	SortByName = iota
	SortByDate
	SortBySize
	SortByDownloads
	SortByRating
	SortByUploader
)

// File index entry for fast searching and sorting
type FileIndex struct {
	AreaNum     uint16 // File area number
	RecordNum   uint32 // Record number in DIR file
	NameHash    uint32 // Hash of filename for fast lookup
	DescHash    uint32 // Hash of description for searching
	Size        uint32 // File size for sorting
	Date        uint32 // Upload date as Unix timestamp
	Downloads   uint16 // Download count for sorting
	Flags       uint8  // Copy of file flags
	Reserved    [3]byte
} // Total: 24 bytes

// Duplicate file detection record
type DuplicateIndex struct {
	FileHash   uint32     // CRC32 of file content
	FileSize   uint32     // File size
	AreaNum    uint16     // Area containing file
	RecordNum  uint32     // Record number in area
	FileName   [13]byte   // Original filename
	Reserved   [7]byte
} // Total: 32 bytes

// Upload session tracking
type UploadSession struct {
	NodeNum      uint8      // Node number handling upload
	UserID       uint16     // User ID uploading
	AreaNum      uint16     // Destination area
	TempPath     [128]byte  // Temporary file path
	OrigName     [13]byte   // Original filename
	FinalName    [13]byte   // Final filename (after processing)
	Size         uint32     // File size
	StartTime    [20]byte   // Upload start time
	Status       uint8      // Upload status (see UploadStatus constants)
	Reserved     [32]byte
} // Total: 256 bytes

// Upload status constants
const (
	UploadStatusActive    = 1 // Upload in progress
	UploadStatusComplete  = 2 // Upload completed, awaiting validation
	UploadStatusValidated = 3 // Upload validated and moved to area
	UploadStatusFailed    = 4 // Upload failed
	UploadStatusDuplicate = 5 // Duplicate file rejected
	UploadStatusVirus     = 6 // Virus detected
)

// File statistics
type FileBaseStats struct {
	TotalAreas    uint16
	TotalFiles    uint32
	TotalSize     uint64    // Total size in bytes
	TotalDownloads uint64   // Total downloads across all areas
	LastUpdate    [20]byte  // Last database update
	LastPack      [20]byte  // Last pack/maintenance
	DuplicateFiles uint32   // Number of duplicate files found
	OrphanFiles   uint32    // Files on disk without database records
	MissingFiles  uint32    // Database records without files on disk
	Reserved      [32]byte
} // Total: 128 bytes

// Batch upload processing queue
type BatchUpload struct {
	QueueID      uint32     // Unique queue ID
	NodeNum      uint8      // Processing node
	UserID       uint16     // User ID
	AreaNum      uint16     // Destination area  
	FilePath     [128]byte  // Path to batch file (ZIP, etc.)
	Status       uint8      // Processing status
	FilesTotal   uint16     // Total files in batch
	FilesProcessed uint16   // Files processed so far
	FilesAccepted uint16    // Files accepted
	FilesRejected uint16    // Files rejected
	StartTime    [20]byte   // Processing start time
	Reserved     [32]byte
} // Total: 256 bytes

// FILE_ID.DIZ processing
type FileIdDiz struct {
	FileName     [13]byte   // Filename this DIZ is for
	Description  [1024]byte // Full FILE_ID.DIZ content
	LineCount    uint8      // Number of description lines
	Processed    uint8      // 1 if processed, 0 if pending
	Reserved     [14]byte
} // Total: 1056 bytes

// DESC.SDI (BBS file description) format
type DescSdi struct {
	FileName     [13]byte   // Filename
	ShortDesc    [46]byte   // Short description (DIR compatible)
	LongDesc     [256]byte  // Long description
	Keywords     [81]byte   // Keywords/tags
	Category     [21]byte   // Category
	Reserved     [32]byte
} // Total: 449 bytes

// File base database file names (classic BBS naming)
const (
	FileRecordFile    = "FILES.DIR"    // Main file records (classic DIR format)
	ExtendedFile      = "FILES.EXT"    // Extended file records
	FileIndexFile     = "FILES.IDX"    // File index for fast access
	DuplicateFile     = "FILES.DUP"    // Duplicate detection index
	AreaConfigFile    = "FILEAREAS.DAT" // Area configuration
	StatsFile         = "FILESTATS.DAT" // File base statistics
	UploadQueueFile   = "UPLOAD.QUE"    // Upload processing queue
	LockFile          = "FILEBASE.LOK"  // Multi-node lock file
	SemaphoreFile     = "FILEBASE.SEM"  // Semaphore for critical operations
)

// Multi-node lock record for file operations
type FileBaseLock struct {
	NodeNum      uint8     // Node holding lock
	LockType     uint8     // Type of operation (see FileLockType constants)
	AreaNum      uint16    // Area being locked (0 = all areas)  
	RecordNum    uint32    // Specific record being locked (0 = not specific)
	Timestamp    [20]byte  // When lock was acquired
	Process      [32]byte  // Process description
	Reserved     [8]byte
} // Total: 64 bytes

// File operation lock types
const (
	FileLockRead      = 1 // Reading files
	FileLockWrite     = 2 // Writing/updating files
	FileLockUpload    = 3 // Processing uploads
	FileLockMaint     = 4 // Maintenance operations
	FileLockPack      = 5 // Packing/reorganizing
	FileLockImport    = 6 // Importing files
	FileLockScan      = 7 // Virus scanning
)

// File area maintenance log
type MaintenanceLog struct {
	Timestamp    [20]byte  // When maintenance was performed
	NodeNum      uint8     // Node that performed maintenance
	Operation    uint8     // Type of operation (see MaintOp constants)
	AreaNum      uint16    // Area affected (0 = all areas)
	FilesAffected uint32   // Number of files affected
	Description  [64]byte  // Operation description
	Reserved     [8]byte
} // Total: 128 bytes

// Maintenance operation types
const (
	MaintOpPack      = 1 // Pack/defragment database
	MaintOpReindex   = 2 // Rebuild indexes
	MaintOpOrphan    = 3 // Remove orphaned records
	MaintOpDuplicate = 4 // Remove duplicate files
	MaintOpValidate  = 5 // Validate file integrity
	MaintOpImport    = 6 // Import files from disk
	MaintOpExport    = 7 // Export file listings
	MaintOpBackup    = 8 // Backup file database
	MaintOpRestore   = 9 // Restore from backup
)

// File transfer protocol settings
type TransferProtocol struct {
	ProtocolID   uint8      // Protocol identifier (1=XMODEM, 2=YMODEM, 3=ZMODEM, etc.)
	Name         [16]byte   // Protocol name
	SendCommand  [128]byte  // Command to send files
	RecvCommand  [128]byte  // Command to receive files
	Batch        uint8      // 1 if protocol supports batch transfers
	Bidirectional uint8     // 1 if protocol supports bidirectional
	Efficiency   uint8      // Efficiency rating (1-10)
	Reserved     [32]byte
} // Total: 256 bytes

// Archive formats supported for browsing
type ArchiveFormat struct {
	Extension    [4]byte    // File extension (.ZIP, .ARJ, etc.)
	Signature    [8]byte    // File signature bytes
	ListCommand  [128]byte  // Command to list archive contents
	ExtractCommand [128]byte // Command to extract files
	TestCommand  [128]byte  // Command to test archive integrity
	Supported    uint8      // 1 if format is supported
	Reserved     [32]byte
} // Total: 427 bytes

// Constants for maximum values
const (
	MaxFileAreas     = 65535 // Maximum file areas
	MaxFilesPerArea  = 65535 // Maximum files per area
	MaxFileSize      = 4294967295 // Maximum file size (4GB)
	MaxDescription   = 1024  // Maximum description length
	MaxKeywords      = 80    // Maximum keyword length
	MaxCategory      = 20    // Maximum category length
)

// File validation rules
type ValidationRule struct {
	RuleID       uint8      // Rule identifier
	Extension    [4]byte    // File extension this rule applies to
	MinSize      uint32     // Minimum file size
	MaxSize      uint32     // Maximum file size
	RequireDesc  uint8      // 1 if description required
	RequireTest  uint8      // 1 if integrity test required
	AutoValidate uint8      // 1 if auto-validation allowed
	ScanVirus    uint8      // 1 if virus scan required
	AllowDupe    uint8      // 1 if duplicates allowed
	CostCredits  uint16     // Download cost in credits
	CostTime     uint16     // Download cost in time units
	Reserved     [16]byte
} // Total: 64 bytes

// File comment/rating system
type FileComment struct {
	FileArea     uint16     // Area number
	RecordNum    uint32     // File record number
	UserID       uint16     // User who left comment
	Rating       uint8      // Rating (1-10)
	Comment      [256]byte  // Comment text
	Timestamp    [20]byte   // When comment was left
	Reserved     [8]byte
} // Total: 304 bytes