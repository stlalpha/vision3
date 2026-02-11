# ViSiON/3 Developer Guide

## Setup

```bash
git clone https://github.com/stlalpha/vision3.git
cd vision3
cd cmd/vision3 && go build
```

## Architecture

```
cmd/vision3/main.go → SSH Server → Session Handler → Menu Executor → Runnable Functions
```

### Key Components

- **Managers**: UserMgr, MessageManager, FileManager
- **Menu System**: Loads .MNU/.CFG/.ANS files, executes commands
- **Runnables**: Functions callable from menus via `RUN:` command

## Adding Features

### New Runnable Function

1. Add to `internal/menu/executor.go`:

```go
func runMyFeature(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, 
    userManager *user.UserMgr, currentUser *user.User, nodeNumber int, 
    sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
    
    msg := "|15Feature output|07\r\n"
    wErr := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
    if wErr != nil {
        log.Printf("ERROR: Node %d: Failed writing feature output: %v", nodeNumber, wErr)
    }
    return nil, "", nil
}
```

1. Register in `registerAppRunnables()`:

```go
registry["MYFEATURE"] = runMyFeature
```

1. Use in menu (.MNU file):

```
HOTKEY:M:RUN:MYFEATURE
```

### New Door

Add to `configs/doors.json`:

```json
[
  {
    "name": "MYDOOR",
    "command": "/path/to/door",
    "args": ["-node", "{NODE}"],
    "working_directory": "/path/to/door",
    "dropfile_type": "DOOR.SYS",
    "io_mode": "STDIO",
    "requires_raw_terminal": true
  }
]
```

## Code Standards

- Use `\r\n` for line endings
- Support both UTF-8 and CP437 modes
- Log with node number: `log.Printf("INFO: Node %d: ...", nodeNumber, ...)`
- Handle errors properly, check all write operations
- Use pipe codes for colors: `|15` (white), `|07` (gray)

## Common Patterns

### ANSI Display

```go
// Load ANSI file
rawContent, err := ansi.GetAnsiFileContent(filepath.Join(e.MenuSetPath, "ansi", "file.ans"))
if err != nil {
    log.Printf("ERROR: Node %d: Failed loading ANSI: %v", nodeNumber, err)
    return nil, "", err
}

// Process for display
result, err := ansi.ProcessAnsiAndExtractCoords(rawContent, outputMode)
if err != nil {
    log.Printf("ERROR: Node %d: Failed processing ANSI: %v", nodeNumber, err)
    return nil, "", err
}

// Write to terminal
wErr := terminalio.WriteProcessedBytes(terminal, result.DisplayBytes, outputMode)
if wErr != nil {
    log.Printf("ERROR: Node %d: Failed writing display: %v", nodeNumber, wErr)
}
```

### User Input

```go
// Read line input
input, err := terminal.ReadLine()
if err != nil {
    if errors.Is(err, io.EOF) {
        return nil, "LOGOFF", io.EOF  // User disconnected
    }
    return nil, "", err
}

// Read password (masked)
password, err := terminal.ReadPassword("Password: ")
```

### Path Management

```go
ansPath := filepath.Join(e.MenuSetPath, "ansi", "file.ans")
configPath := filepath.Join(e.RootConfigPath, "config.json")
dataPath := filepath.Join("data", "users", "users.json")
```

## Testing

- Test with both `--output-mode=utf8` and `--output-mode=cp437`
- Test with multiple concurrent users
- Verify menu navigation and error cases
- Test disconnection handling (Ctrl+C, network drops)

## Contributing

1. Fork and create feature branch
2. Make changes following code standards
3. Test thoroughly
4. Submit PR with clear description

## Common Issues

- **Import errors**: Make sure all imports use `github.com/stlalpha/vision3/internal/...`
- **ANSI display issues**: Always use `terminalio.WriteProcessedBytes()` for output
- **Path issues**: Use `e.MenuSetPath` for menu files, `e.RootConfigPath` for configs
