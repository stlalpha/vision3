# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Building and Running
```bash
# Build the main BBS application
cd cmd/vision3
go build

# Run the BBS server
./vision3

# Run with specific output mode
./vision3 --output-mode=utf8    # Force UTF-8 output
./vision3 --output-mode=cp437   # Force CP437 output for authentic BBS experience
./vision3 --output-mode=auto    # Auto-detect based on terminal (default)
```

### Development Tools
```bash
# Build ANSI test utility
cd cmd/ansitest  
go build

# Format code
go fmt ./...

# Run linter
golangci-lint run

# Run tests
go test ./...

# Run tests for specific package
go test ./internal/ansi
```

### Setup and Configuration
```bash
# Quick setup (generates SSH keys, creates directories, builds executable)
./setup.sh

# Manual SSH key generation
cd configs
ssh-keygen -t rsa -f ssh_host_rsa_key -N ""
ssh-keygen -t ed25519 -f ssh_host_ed25519_key -N ""

# Create necessary directories
mkdir -p data/users data/files/general log
```

## High-Level Architecture

### Core System Design
ViSiON/3 is a single Go binary that implements a classic BBS (Bulletin Board System) accessible via SSH. It recreates the authentic BBS experience of the 1990s using modern Go technologies.

**Key Architectural Principles:**
- **Single Binary Deployment**: All functionality compiled into one executable
- **SSH-Only Access**: Secure remote access (no telnet), listens on port 2222 by default
- **Menu-Driven Interface**: All functionality accessed through hierarchical menu system
- **Legacy Compatibility**: Maintains ViSiON/2 pipe code compatibility for ANSI art
- **Multiple Character Encodings**: Supports CP437 (authentic DOS/BBS) and UTF-8 output modes

### Session Management Flow
1. **Connection**: SSH client connects → main accepts → sessionHandler goroutine spawned
2. **PTY Setup**: Handle PTY requests, window size changes, determine output mode (auto/utf8/cp437)
3. **Authentication Loop**: Menu executor loads LOGIN menu, handles user authentication
4. **Main Loop**: After successful login, navigate through menu system (MAIN, READ_MSG, etc.)
5. **Data Persistence**: User/message/file data persisted to JSON files in `data/` directory

### Core Components

#### Main Application (`cmd/vision3/main.go`)
- SSH server initialization using `gliderlabs/ssh`
- Session lifecycle management and connection handling
- Global managers initialization (user, message, file, menu)
- Host key loading and crypto configuration

#### Menu System (`internal/menu/`)
- **MenuExecutor**: Central coordinator for menu display and command execution
- **Loader**: Parses `.MNU` (binary menu definitions) and `.CFG` (display configs) files
- **Command Processing**: Handles menu navigation, user input, and action execution
- **Lightbar Support**: Full lightbar navigation for compatible menus
- **ACS Integration**: Access Control String evaluation for menu access

#### User Management (`internal/user/`)
- **UserManager**: User account persistence (`data/users/users.json`)
- **Authentication**: bcrypt password hashing and verification
- **Call History**: Session tracking in `data/users/callhistory.json`
- **ACS Evaluation**: Access control validation (security levels, flags, time limits)

#### ANSI/Character Handling (`internal/ansi/`)
- **Pipe Code Processing**: ViSiON/2 compatible color codes (`|00`-`|15`, `|B0`-`|B7`)
- **Character Encoding**: CP437 to UTF-8 conversion with VT100 line drawing support
- **Output Modes**: Auto-detection, forced UTF-8, or authentic CP437 rendering
- **ANSI Art Display**: Proper rendering of classic BBS artwork

#### Message System (`internal/message/`)
- **MessageManager**: Message area and message persistence
- **Configuration**: Area definitions in `data/message_areas.json`
- **Storage**: JSONL files per area (e.g., `messages_area_1.jsonl`)
- **Features**: Private/public areas, threading, newscan functionality

#### File Management (`internal/file/`)
- **FileManager**: File area management and metadata tracking
- **Configuration**: Area definitions in `configs/file_areas.json`
- **Storage**: File metadata in `data/files/` directory
- **Transfer Protocol**: ZMODEM support via external `sz`/`rz` commands

### Data Organization

```
data/
├── users/
│   ├── users.json           # User account database
│   └── callhistory.json     # Session history
├── files/                   # File area directories
│   └── general/
│       └── metadata.json    # File listings
├── logs/                    # Application logs
└── message_*.jsonl          # Message data files
```

### Menu Resources

```
menus/v3/
├── ansi/                    # ANSI art files (.ANS)
├── cfg/                     # Menu display configurations
├── mnu/                     # Menu command definitions (.MNU)
└── templates/               # Display templates (.MID files)
```

### Configuration Files

```
configs/
├── config.json              # General BBS configuration  
├── strings.json             # Customizable UI text
├── doors.json               # External program definitions
├── file_areas.json          # File area configurations
└── ssh_host_*_key          # SSH host keys
```

## Development Guidelines

### Code Organization
- Follow Go's package organization principles (functionality over type)
- Keep modules under 300 lines when possible
- Use the established internal package structure
- Respect module boundaries defined in `docs/architecture.md`

### Important Cursor Rules
From `.cursorrules`:
- **Simplicity**: Prioritize simple, maintainable solutions over complexity
- **Iterate**: Prefer improving existing code rather than rewriting from scratch
- **Documentation First**: Always check project documentation before starting tasks
- **Architecture Adherence**: Respect module boundaries and data flow patterns
- **TDD Approach**: Write tests before implementing new features
- **Security**: Never hardcode credentials, validate all input, use proper error handling

### BBS-Specific Considerations
- **Terminal Compatibility**: Consider both modern UTF-8 terminals and legacy CP437 clients
- **Menu Navigation**: Understand the hierarchical menu flow and lightbar system
- **ANSI Art**: Preserve authentic visual appearance while ensuring compatibility
- **Character Encoding**: Handle pipe codes (`|XX`) and CP437 character mapping correctly
- **Session State**: Track user context across menu navigation

### Key Technologies
- **SSH Library**: `github.com/gliderlabs/ssh` for secure remote access
- **Terminal Handling**: `golang.org/x/term` for terminal I/O operations
- **UI Components**: `github.com/charmbracelet/bubbletea` for interactive editors
- **Crypto**: `golang.org/x/crypto/bcrypt` for password security
- **Data Format**: JSON for configuration and data persistence

### Testing Commands
```bash
# Connect to running BBS (default login: felonius/password)
ssh felonius@localhost -p 2222

# Test specific menu functionality
# Navigate through menus using lightbar (arrow keys) or hotkeys
# Test message posting, file listing, door programs
```