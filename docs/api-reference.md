# ViSiON/3 API Reference

This document provides a reference for the main packages and interfaces in ViSiON/3.

## Package Overview

```
internal/
├── ansi/         # ANSI and pipe code processing
├── config/       # Configuration loading and structures
├── editor/       # Text editor implementation
├── file/         # File area management
├── ftn/          # FTN packet (.PKT) library
├── jam/          # JAM message base implementation
├── menu/         # Menu system execution
├── message/      # Message area management (JAM-backed)
├── session/      # Session state management
├── terminalio/   # Terminal I/O with encoding support
├── tosser/       # FTN echomail tosser
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

Message area management backed by JAM message bases.

```go
// MessageManager handles message operations via JAM bases
type MessageManager struct {
    // Internal fields: dataPath, areasPath, areasByID, areasByTag,
    // bases (map[int]*jam.Base), boardName, sync.RWMutex
}

// Key methods:
func NewMessageManager(dataPath, configPath, boardName string) (*MessageManager, error)
func (m *MessageManager) Close() error
func (m *MessageManager) ListAreas() []*MessageArea
func (m *MessageManager) GetAreaByID(id int) (*MessageArea, bool)
func (m *MessageManager) GetAreaByTag(tag string) (*MessageArea, bool)
func (m *MessageManager) AddMessage(areaID int, from, to, subject, body, replyMsgID string) (int, error)
func (m *MessageManager) GetMessage(areaID, msgNum int) (*DisplayMessage, error)
func (m *MessageManager) GetMessageCountForArea(areaID int) (int, error)
func (m *MessageManager) GetNewMessageCount(areaID int, username string) (int, error)
func (m *MessageManager) GetLastRead(areaID int, username string) (int, error)
func (m *MessageManager) SetLastRead(areaID int, username string, msgNum int) error
func (m *MessageManager) GetNextUnreadMessage(areaID int, username string) (int, error)
func (m *MessageManager) GetBase(areaID int) (*jam.Base, error)
```

### jam

JAM (Joaquim-Andrew-Mats) binary message base implementation.

```go
// Base represents an open JAM message base (4 files: .jhr/.jdt/.jdx/.jlr)
type Base struct {
    // Thread-safe with sync.RWMutex
}

// Key methods:
func Open(basePath string) (*Base, error)
func Create(basePath string) (*Base, error)
func (b *Base) Close() error
func (b *Base) GetMessageCount() (int, error)
func (b *Base) ReadMessage(msgNum int) (*Message, error)
func (b *Base) ReadMessageHeader(msgNum int) (*MessageHeader, error)
func (b *Base) WriteMessage(msg *Message) (int, error)
func (b *Base) WriteMessageExt(msg *Message, msgType MessageType, echoTag, boardName string) (int, error)
func (b *Base) UpdateMessageHeader(msgNum int, hdr *MessageHeader) error
func (b *Base) DeleteMessage(msgNum int) error
func (b *Base) GetLastRead(username string) (int, error)
func (b *Base) SetLastRead(username string, msgNum int) error
func (b *Base) GetNextUnreadMessage(username string) (int, error)
func (b *Base) GetUnreadCount(username string) (int, error)

// Address handling:
func ParseAddress(s string) (*FidoAddress, error)
func (a *FidoAddress) String() string   // 4D: "Z:N/N.P"
func (a *FidoAddress) String2D() string // 2D: "N/N" (for SEEN-BY/PATH)
```

### ftn

FidoNet Technology Network Type-2+ packet library.

```go
// Key types:
type PacketHeader struct { /* 58-byte .PKT header */ }
type PackedMessage struct { MsgType, OrigNode, DestNode, OrigNet, DestNet, Attr uint16; DateTime, To, From, Subject, Body string }
type ParsedBody struct { Area string; Kludges []string; Text string; SeenBy, Path []string }

// Key functions:
func NewPacketHeader(origZone, origNet, origNode, origPoint, destZone, destNet, destNode, destPoint uint16, password string) *PacketHeader
func ReadPacket(r io.Reader) (*PacketHeader, []*PackedMessage, error)
func WritePacket(w io.Writer, hdr *PacketHeader, msgs []*PackedMessage) error
func ParsePackedMessageBody(body string) *ParsedBody
func FormatPackedMessageBody(parsed *ParsedBody) string
func FormatFTNDateTime(t time.Time) string
func ParseFTNDateTime(s string) (time.Time, error)
```

### tosser

Built-in FTN echomail tosser.

```go
// Key types:
type Tosser struct { /* config, msgMgr, dupeDB, ownAddr */ }
type Config struct { Enabled bool; OwnAddress, InboundPath, OutboundPath, TempPath, DupeDBPath string; PollSeconds int; Links []LinkConfig }
type LinkConfig struct { Address, Password, Name string; EchoAreas []string }
type TossResult struct { MessagesImported, MessagesExported int; Errors []string }

// Key methods:
func New(cfg *Config, msgMgr *message.MessageManager) (*Tosser, error)
func (t *Tosser) Start(ctx context.Context)
func (t *Tosser) RunOnce() TossResult
func (t *Tosser) ProcessInbound() TossResult
func (t *Tosser) ScanAndExport() TossResult
func (t *Tosser) PurgeDupes() error
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
    // Additional fields...
}
```

### MessageArea

```go
type MessageArea struct {
    ID           int    `json:"id"`
    Tag          string `json:"tag"`
    Name         string `json:"name"`
    Description  string `json:"description"`
    ACSRead      string `json:"acs_read"`
    ACSWrite     string `json:"acs_write"`
    ConferenceID int    `json:"conference_id"`
    BasePath     string `json:"base_path"`      // Relative path to JAM base files
    AreaType     string `json:"area_type"`       // "local", "echomail", "netmail"
    EchoTag      string `json:"echo_tag"`        // FTN echo tag
    OriginAddr   string `json:"origin_addr"`     // FTN origin address
}
```

### DisplayMessage

Used by the UI layer for message display (returned by `GetMessage`):

```go
type DisplayMessage struct {
    MsgNum    int       // 1-based message number
    From      string
    To        string
    Subject   string
    DateTime  time.Time
    Body      string
    IsPrivate bool
    AreaID    int
    MsgID     string    // FTN MSGID (for replies)
    ReplyID   string    // MSGID of parent message
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
- `LISTMSGAR` - List message areas (grouped by conference)
- `SELECTMSGAREA` - Select a message area
- `COMPOSEMSG` - Compose new message in current area
- `PROMPTANDCOMPOSEMESSAGE` - Select area then compose
- `READMSGS` - Read messages (random-access, JAM-backed)
- `NEWSCAN` - Scan for new messages (per-user lastread via JAM)
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