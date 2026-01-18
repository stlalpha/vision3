# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Building and Running
```bash
# Build main BBS application
cd cmd/vision3 && go build

# Build all tools at once
go build ./cmd/...

# Run the BBS server
./cmd/vision3/vision3

# Output modes
./vision3 --output-mode=utf8    # Force UTF-8 output
./vision3 --output-mode=cp437   # Force CP437 for authentic BBS experience
./vision3 --output-mode=auto    # Auto-detect (default)
```

### Testing
```bash
go test ./...                          # Run all tests
go test -v ./internal/menu/            # Verbose tests for specific package
go test -run TestACS ./internal/menu/  # Run specific test
go test ./... -coverprofile=coverage.out  # Generate coverage report
go test -race ./...                    # Run with race detector
```

### Code Quality
```bash
go fmt ./...          # Format code
go vet ./...          # Static analysis
golangci-lint run     # Linter (configured in .golangci.yml)
```

### Using Makefile
```bash
make              # Build main binary
make build        # Build main binary
make test         # Run tests
make lint         # Run linter
make vet          # Run go vet
make fmt          # Format code
make all          # Run lint, vet, test, and build
make clean        # Remove build artifacts
```

### Setup
```bash
./setup.sh            # Quick setup (SSH keys, directories, build)

# Manual SSH key generation
cd configs
ssh-keygen -t rsa -f ssh_host_rsa_key -N ""
ssh-keygen -t ed25519 -f ssh_host_ed25519_key -N ""
```

### Connecting
```bash
ssh felonius@localhost -p 2222    # Default login: felonius/password
```

## High-Level Architecture

ViSiON/3 is a single Go binary BBS accessible via SSH, recreating the authentic 1990s BBS experience.

### Session Flow
```
SSH Client → gliderlabs/ssh server → sessionHandler goroutine
  → PTY setup + output mode detection
  → LOGIN menu (authentication)
  → MAIN menu loop
  → Menu executor processes commands
  → Data persisted to JSON/JSONL files
```

### Core Components

| Package | Purpose |
|---------|---------|
| `cmd/vision3/main.go` | SSH server, session lifecycle, manager initialization |
| `internal/menu/` | Menu system (see below) |
| `internal/user/` | User persistence, bcrypt auth, ACS evaluation |
| `internal/message/` | Message areas, JSONL storage |
| `internal/file/` | File areas, ZMODEM via external sz/rz |
| `internal/terminal/` | BBS terminal class, encoding, pipe code processing |
| `internal/logging/` | Debug logging utility with runtime toggle |

### Menu Package Structure

The menu system has been organized into focused modules:

| File | Purpose |
|------|---------|
| `executor.go` | Core MenuExecutor struct and session handling |
| `loader.go` | Menu file loading (.MNU, .CFG) |
| `dispatcher.go` | Command routing and execution |
| `registry.go` | Runnable function registration |
| `acs_checker.go` | Access Control System evaluation |
| `runnables_system.go` | System functions (login, version, user editor) |
| `runnables_messaging.go` | Message area listing, reading, composing |
| `runnables_files.go` | File area listing, downloads, uploads |
| `runnables_doors.go` | External door program execution |
| `runnables_misc.go` | Oneliners, renderer settings |

### Menu System

**File Types:**
- `.MNU` files: JSON with CLR, PROMPT1, FALLBACK, ACS, PASS fields
- `.CFG` files: JSON array of commands with KEYS, CMD, ACS, HIDDEN
- `.ANS` files: ANSI art with pipe codes (`|00`-`|15`, `|B0`-`|B7`)

**Command Types:**
- `GOTO:MENU` - Navigate to menu
- `RUN:FUNCTION` - Execute registered runnable
- `DOOR:DOORNAME` - Launch external program
- `LOGOFF` - Disconnect user
- `//` prefix - Auto-run once per session
- `~~` prefix - Auto-run every visit

### Access Control System (ACS)

ACS strings control menu and command access. Operators: `&` (AND), `|` (OR), `!` (NOT), `()` grouping.

| Code | Meaning |
|------|---------|
| `S10` | Security level >= 10 |
| `FZ` | User has flag 'Z' |
| `U5` | User ID = 5 |
| `L` | Local connection |
| `A` | ANSI/PTY connection |
| `V` | Validated user |
| `E10` | At least 10 logons |
| `P50` | At least 50 file points |
| `*` | Always allow |
| `` (empty) | Require authentication |

Example: `S100|FA` = security level 100+ OR has flag A

## Adding Features

### New Runnable Function

1. Add function to the appropriate `internal/menu/runnables_*.go` file:
```go
func runMyFeature(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
    userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
    sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {

    msg := "|15Feature output|07\r\n"
    wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
    if wErr != nil {
        log.Printf("ERROR: Node %d: Failed writing: %v", nodeNumber, wErr)
    }
    return nil, "", nil  // Returns: updated user, next action, error
}
```

2. Register in `registerAppRunnables()`:
```go
registry["MYFEATURE"] = runMyFeature
```

3. Add to menu `.CFG` file:
```json
{"KEYS": "M", "CMD": "RUN:MYFEATURE", "ACS": ""}
```

### New Door Program

Add to `configs/doors.json`:
```json
{
  "name": "MYDOOR",
  "command": "/path/to/door",
  "args": ["-node", "{NODE}"],
  "working_directory": "/path/to/door",
  "dropfile_type": "DOOR.SYS",
  "io_mode": "STDIO",
  "requires_raw_terminal": true
}
```

## Code Patterns

### Terminal Output
```go
// Always use terminalio.WriteProcessedBytes for output
msg := "|15White text|07\r\n"
terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
```

### ANSI File Display
```go
rawContent, err := ansi.GetAnsiFileContent(filepath.Join(e.MenuSetPath, "ansi", "file.ans"))
result, err := ansi.ProcessAnsiAndExtractCoords(rawContent, outputMode)
terminalio.WriteProcessedBytes(terminal, result.DisplayBytes, outputMode)
```

### Path Resolution
```go
ansPath := filepath.Join(e.MenuSetPath, "ansi", "file.ans")      // Menu assets
configPath := filepath.Join(e.RootConfigPath, "config.json")     // Global configs
dataPath := filepath.Join("data", "users", "users.json")         // Runtime data
```

### Logging
```go
// Standard logging (always output)
log.Printf("INFO: Node %d: User %s logged in", nodeNumber, user.Handle)
log.Printf("ERROR: Node %d: Failed to load menu: %v", nodeNumber, err)

// Debug logging (controlled by logging.DebugEnabled)
import "github.com/stlalpha/vision3/internal/logging"
logging.Debug("Node %d: Processing menu %s", nodeNumber, menuName)
```

Enable debug logging by setting `logging.DebugEnabled = true` at startup.

## Development Guidelines

From `.cursorrules`:
- **Simplicity**: Prefer simple, maintainable solutions
- **Iterate**: Improve existing code rather than rewriting
- **Documentation First**: Check `docs/architecture.md` before starting
- **TDD**: Write tests before implementation
- **Keep files under 300 lines** when possible

### BBS-Specific
- Use `\r\n` for line endings (BBS standard)
- Support both UTF-8 and CP437 output modes
- Use pipe codes for colors: `|00`-`|15` foreground, `|B0`-`|B7` background
- Handle disconnection gracefully (check for io.EOF)

### What NOT to Do
- No REST APIs or web frontend
- No rewrites in other languages
- No custom build systems
- Don't commit one-time utility scripts

## Data Storage

| Location | Format | Purpose |
|----------|--------|---------|
| `data/users/users.json` | JSON | User accounts |
| `data/users/callhistory.json` | JSON | Session history |
| `data/message_areas.json` | JSON | Message area config |
| `data/messages_area_*.jsonl` | JSONL | Message content |
| `configs/` | JSON | All configuration files |
