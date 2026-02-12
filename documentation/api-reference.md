# ViSiON/3 API Reference

This document provides a reference for the main packages and interfaces in ViSiON/3.

## Package Overview

```
internal/
├── ansi/         # ANSI and pipe code processing
├── config/       # Configuration loading and structures
├── editor/       # Text editor implementation
├── file/         # File area management
├── menu/         # Menu system execution
├── message/      # Message base management
├── session/      # Session state management
├── terminalio/   # Terminal I/O with encoding support
├── transfer/     # File transfer protocols
├── types/        # Shared type definitions
└── user/         # User management
```

## Core Packages

### user

User management and authentication.

```go
// UserMgr manages user data
type UserMgr struct {
    // Internal fields
}

// Key methods:
func NewUserManager(dataPath string) (*UserMgr, error)
func (um *UserMgr) Authenticate(username, password string) (*User, bool)
func (um *UserMgr) GetUser(username string) (*User, bool)
func (um *UserMgr) AddUser(username, password, handle, realName, phoneNum, groupLocation string) (*User, error)
func (um *UserMgr) SaveUsers() error
func (um *UserMgr) GetAllUsers() []*User
func (um *UserMgr) AddCallRecord(record CallRecord)
func (um *UserMgr) GetLastCallers() []CallRecord
```

### menu

Menu system loading and execution.

```go
// MenuExecutor handles menu operations
type MenuExecutor struct {
    MenuSetPath    string
    RootConfigPath string
    RootAssetsPath string
    RunRegistry    map[string]RunnableFunc
    DoorRegistry   map[string]config.DoorConfig
    OneLiners      []string
    LoadedStrings  config.StringsConfig
    Theme          config.ThemeConfig
    MessageMgr     *message.MessageManager
    FileMgr        *file.FileManager
}

// Key methods:
func NewExecutor(menuSetPath, rootConfigPath, rootAssetsPath string, ...) *MenuExecutor
func (e *MenuExecutor) Run(s ssh.Session, terminal *term.Terminal, ...) (string, *User, error)
```

### message

Message area and message management.

```go
// MessageManager handles message operations
type MessageManager struct {
    // Internal fields
}

// Key methods:
func NewMessageManager(dataPath string) (*MessageManager, error)
func (m *MessageManager) ListAreas() []*MessageArea
func (m *MessageManager) GetAreaByID(id int) (*MessageArea, bool)
func (m *MessageManager) GetAreaByTag(tag string) (*MessageArea, bool)
func (m *MessageManager) AddMessage(areaID int, msg Message) error
func (m *MessageManager) GetMessagesForArea(areaID int, sinceMessageID string) ([]Message, error)
func (m *MessageManager) GetMessageCountForArea(areaID int) (int, error)
func (m *MessageManager) GetNewMessageCount(areaID int, sinceMessageID string) (int, error)
```

### file

File area management.

```go
// FileManager manages file areas
type FileManager struct {
    // Internal fields
}

// Key methods:
func NewFileManager(dataPath, configPath string) (*FileManager, error)
func (fm *FileManager) GetAllAreas() []FileArea
func (fm *FileManager) GetAreaByID(id int) (*FileArea, error)
func (fm *FileManager) GetFilesForArea(areaID int) ([]FileRecord, error)
```

### ansi

ANSI escape sequence and pipe code processing.

```go
// Key functions:
func ReplacePipeCodes(data []byte) []byte
func ProcessAnsiAndExtractCoords(rawContent []byte, outputMode OutputMode) (ProcessAnsiResult, error)
func GetAnsiFileContent(filename string) ([]byte, error)
func ClearScreen() string
func MoveCursor(row, col int) string
func StripAnsi(str string) string

// Output modes:
const (
    OutputModeAuto  OutputMode = iota
    OutputModeUTF8
    OutputModeCP437
)
```

### config

Configuration structures and loading.

```go
// Key types:
type StringsConfig map[string]string
type ThemeConfig map[string]interface{}
type DoorConfig struct {
    Name                string
    Command             string
    Args                []string
    WorkingDirectory    string
    RequiresRawTerminal bool
    DropfileType        string
    EnvironmentVars     map[string]string
}

// Key functions:
func LoadStrings(configPath string) (StringsConfig, error)
func LoadDoors(path string) (map[string]DoorConfig, error)
func LoadThemeConfig(menuSetPath string) (ThemeConfig, error)
```

### terminalio

Terminal I/O with encoding support.

```go
// Key functions:
func WriteProcessedBytes(terminal io.Writer, processedBytes []byte, outputMode ansi.OutputMode) error
```

### types

Shared type definitions.

```go
// AutoRunTracker tracks commands that have been auto-run
type AutoRunTracker map[string]bool

// Other shared types used across packages
```

## Data Structures

### User

```go
type User struct {
    ID                 int
    Username           string
    PasswordHash       string
    Handle             string
    RealName           string
    PhoneNumber        string
    GroupLocation      string
    AccessLevel        int
    Flags              string
    LastLogin          time.Time
    TimesCalled        int
    Validated          bool
    TimeLimit          int
    CurrentMessageAreaID   int
    CurrentMessageAreaTag  string
    CurrentFileAreaID      int
    CurrentFileAreaTag     string
    LastReadMessageIDs     map[int]string
    // Additional fields...
}
```

### Message

```go
type Message struct {
    ID           uuid.UUID
    AreaID       int
    FromUserName string
    FromNodeID   string
    ToUserName   string
    ToNodeID     string
    Subject      string
    Body         string
    PostedAt     time.Time
    ReplyToID    uuid.UUID
    IsPrivate    bool
    Path         []string
    // Additional fields...
}
```

### FileRecord

```go
type FileRecord struct {
    ID            uuid.UUID
    AreaID        int
    Filename      string
    Description   string
    Size          int64
    UploadedAt    time.Time
    UploadedBy    string
    DownloadCount int
}
```

## Menu System

### Menu Commands

- `GOTO:<menu>` - Navigate to another menu
- `RUN:<function>` - Execute a function
- `DOOR:<doorname>` - Launch external door
- `LOGOFF` - Disconnect user
- `COND:<condition>:<action>` - Conditional execution
- `HOTKEY:<key>:<action>` - Hotkey binding

### Runnable Functions

Currently implemented functions that can be called via `RUN:`:

- `AUTHENTICATE` - User login
- `FULL_LOGIN_SEQUENCE` - Complete login process
- `SHOWSTATS` - Display user statistics
- `LASTCALLERS` - Show recent callers
- `LISTUSERS` - List all users
- `ONELINER` - One-liner system
- `LISTMSGAR` - List message areas
- `COMPOSEMSG` - Compose new message
- `READMSGS` - Read messages
- `NEWSCAN` - Scan for new messages
- `LISTFILES` - List files in current area
- `LISTFILEAR` - List file areas
- `SELECTFILEAREA` - Select file area

## Error Handling

Most functions return an error as the last return value. Always check:

```go
result, err := someFunction()
if err != nil {
    // Handle error
}
```

## Logging

The system uses Go's standard `log` package. Log entries include:
- Node number
- User information (when available)
- Severity level (INFO, WARN, ERROR, DEBUG)

## Session Context

Sessions carry context including:
- SSH session reference
- Terminal instance
- User reference (after authentication)
- Node ID
- Start time
- Auto-run tracker 