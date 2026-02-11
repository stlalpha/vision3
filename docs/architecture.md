# System Architecture

This document outlines the high-level architecture of the ViSiON/3 Go BBS.

## Overview

The system is designed as a single Go application that listens for incoming SSH and telnet connections and manages individual user sessions. Both protocols share the same session handler via the `gliderlabs/ssh.Session` interface. The main entry point is `cmd/vision3/main.go`.

## Components

1. **Main Application (`cmd/vision3/main.go`)**
   * Initializes logging, configuration, user/message/file managers, and menu executor
   * Sets up the SSH server (libssh via CGO) and telnet server (native Go)
   * Loads SSH host keys from `configs/` directory
   * Listens for incoming connections on configured ports (SSH default: 2222, Telnet default: 2323)
   * Accepts connections and spawns goroutines to handle individual sessions via `sessionHandler`

2. **Session Handler (`sessionHandler` in `cmd/vision3/main.go`)**
   * Manages the lifecycle of a single session (SSH or telnet)
   * Handles PTY requests and window size changes
   * Tracks terminal dimensions with `atomic.Int32` for thread-safe access between the resize goroutine and main session goroutine
   * Calls `terminal.SetSize()` on initial PTY setup and on each resize event
   * After authentication, applies user's stored `ScreenWidth`/`ScreenHeight` preferences to cap effective terminal dimensions
   * Determines output mode (UTF-8, CP437, or auto-detect)
   * Creates terminal instance for I/O
   * Manages authentication loop using the menu executor
   * Tracks session in `activeSessions` map
   * Records call history after disconnect
   * Runs main menu loop after successful authentication

3. **Menu System (`internal/menu/`)**
   * `MenuExecutor` handles menu loading, display, and command execution
   * Loads menu configurations from `menus/v3/mnu/` (`.MNU` files)
   * Loads menu display files from `menus/v3/cfg/` (`.CFG` files)
   * Displays ANSI screens from `menus/v3/ansi/` (`.ANS` files), truncated to the user's effective terminal height to prevent scrolling
   * Handles auto-run commands and user input
   * Integrates with all managers (user, message, file)

4. **User Manager (`internal/user/manager.go`)**
   * Manages user accounts (loading, saving, finding, adding, authenticating)
   * Persists user data to `data/users/users.json`
   * Tracks call records in `data/users/callhistory.json`
   * Handles ACS (Access Control String) validation

5. **Message Manager (`internal/message/manager.go`)**
   * Manages message areas and individual messages
   * Configuration loaded from `configs/message_areas.json`
   * Messages stored in JAM (Joaquim-Andrew-Mats) binary message bases under `data/msgbases/`
   * Each area has 4 files: `.jhr` (headers), `.jdt` (text), `.jdx` (index), `.jlr` (lastread)
   * Supports local, echomail, and netmail area types
   * Per-user lastread tracking via JAM `.jlr` files
   * Thread-safe with `sync.RWMutex` for concurrent SSH/telnet sessions

6. **JAM Message Base (`internal/jam/`)**
   * Binary message base implementation following the JAM specification
   * Random access by message number via index file
   * CRC32 indexing for fast lookups
   * Subfield-based message headers (MSGID, REPLY, sender, receiver, etc.)
   * Automatic base creation on first access

7. **FTN Packet Library (`internal/ftn/`)**
   * FidoNet Technology Network Type-2+ packet (.PKT) reader/writer
   * Packed message body parser/formatter (AREA, kludges, SEEN-BY, PATH)
   * FTN datetime formatting and parsing

8. **FTN Tosser (`internal/tosser/`)**
   * Built-in echomail tosser for FTN message exchange
   * Inbound processing: scans .PKT files, dupe-checks via MSGID, writes to JAM bases
   * Outbound export: scans for unprocessed messages (DateProcessed=0), creates .PKT files per link
   * SEEN-BY/PATH management with net compression
   * Background polling at configurable interval
   * JSON-based dupe database with automatic purge

9. **File Manager (`internal/file/manager.go`)**
   * Manages file areas and file listings
   * Configuration loaded from `configs/file_areas.json`
   * File metadata stored in `data/files/` directory
   * Handles file uploads/downloads and descriptions

10. **Conference Manager (`internal/conference/conference.go`)**

* Groups message areas and file areas into named conferences
* Configuration loaded from `configs/conferences.json`
* Provides conference ACS filtering for area visibility
* Optional — system operates with flat area listings if conferences.json is missing

11. **Event Scheduler (`internal/scheduler/`)**

* Cron-style task scheduler for automated maintenance and periodic operations
* Configuration loaded from `configs/events.json`
* Uses robfig/cron v3 for flexible scheduling (standard 5-field cron syntax plus special schedules)
* Features:
  - Concurrency control with configurable max concurrent events
  - Per-event execution tracking to prevent overlaps
  - Timeout support with context-based cancellation
  - Event history persistence in `data/logs/event_history.json`
  - Placeholder substitution in commands and arguments
  - Non-interactive batch execution (no PTY/TTY)
* Common use cases: FTN mail polling (binkd), echomail tossing (HPT), backups, maintenance
* Background service with graceful shutdown and history persistence

12. **ANSI Handler (`internal/ansi/ansi.go`)**

* Parses ViSiON/2 specific pipe codes (`|00`-`|15`, `|B0`-`|B7`, etc.)
* Converts CP437 characters to UTF-8 or VT100 line drawing
* Supports multiple output modes (UTF-8, CP437, Auto)
* Handles ANSI screen processing and display

13. **Configuration (`configs/` directory)**

* `strings.json` - Externalized UI strings and prompts
* `config.json` - General system configuration
* `doors.json` - External door program configurations
* `file_areas.json` - File area definitions
* `message_areas.json` - Message area definitions
* `conferences.json` - Conference grouping definitions
* `events.json` - Event scheduler configuration
* SSH host keys (`ssh_host_rsa_key`, etc.)

## Data Flow

1. Client connects via SSH or telnet → `main` accepts → `sessionHandler` spawned
2. `sessionHandler` handles PTY setup and determines output mode
3. SSH users with known accounts are auto-logged in, skipping to step 6
4. Telnet users see the pre-login matrix screen (`PDMATRIX.ANS`) with options: login, create account, check access, or disconnect
5. `sessionHandler` starts authentication loop via menu executor (LOGIN menu)
6. Successful login transitions to main menu loop (e.g., FASTLOGN or MAIN)
7. Menu executor processes user commands and navigates between menus
8. ANSI screens are loaded from `menus/v3/ansi/` and processed by `ansi` package
9. User/message/file data is persisted to `data/` directory

## Directory Structure

```text
vision3/
├── cmd/vision3/         # Main application entry point
├── configs/             # Configuration files
├── data/               # Persistent data
│   ├── users/          # User accounts and call history
│   ├── files/          # File area directories and metadata
│   ├── msgbases/       # JAM message base files (.jhr/.jdt/.jdx/.jlr)
│   ├── ftn/            # FTN echomail data
│   │   ├── inbound/    # Incoming .PKT files
│   │   ├── outbound/   # Outgoing .PKT files
│   │   ├── temp/       # Temp for failed packets
│   │   └── dupes.json  # MSGID dupe database
│   ├── events/         # Event scheduler history
│   │   └── event_history.json  # Event execution statistics
│   └── logs/           # Application logs
├── internal/           # Internal packages
│   ├── ansi/           # ANSI/CP437 handling
│   ├── conference/     # Conference grouping
│   ├── config/         # Configuration loading
│   ├── editor/         # Text editor (placeholder)
│   ├── file/           # File area management
│   ├── ftn/            # FTN packet (.PKT) library
│   ├── jam/            # JAM message base implementation
│   ├── menu/           # Menu system
│   ├── message/        # Message area management (JAM-backed)
│   ├── scheduler/      # Event scheduler (cron-style)
│   ├── session/        # Session management
│   ├── sshserver/      # SSH server (libssh via CGO)
│   ├── telnetserver/   # Telnet server (native Go)
│   ├── terminalio/     # Terminal I/O utilities
│   ├── tosser/         # FTN echomail tosser
│   ├── transfer/       # File transfer protocols
│   ├── types/          # Shared types
│   └── user/           # User management
└── menus/v3/           # Menu resources
    ├── ansi/           # ANSI art files
    ├── cfg/            # Menu display configurations
    ├── mnu/            # Menu command definitions
    └── templates/      # Display templates
```

## Module Boundaries

* `cmd/vision3`: Main application loop, SSH/telnet server setup, session handling
* `internal/sshserver`: SSH server using libssh via CGO
* `internal/telnetserver`: Telnet server with IAC protocol handling
* `internal/user`: User data structures, persistence, authentication, ACS logic
* `internal/ansi`: ViSiON/2 pipe code parsing and character encoding
* `internal/conference`: Conference grouping for message and file areas
* `internal/config`: Configuration file loading and parsing
* `internal/menu`: Menu loading, display, command execution
* `internal/jam`: JAM binary message base (read/write/index/lastread)
* `internal/ftn`: FTN Type-2+ packet parsing and creation
* `internal/message`: Message area management, JAM-backed storage
* `internal/tosser`: FTN echomail import/export, dupe checking, SEEN-BY/PATH
* `internal/scheduler`: Cron-style event scheduler for automated tasks
* `internal/file`: File area management and metadata
* `internal/session`: Session state tracking (currently minimal)
* `menus/v3`: Static menu resources (ANSI art, menu definitions)
* `data`: Persisted application data (users, JAM bases, FTN packets, logs)

## Key Design Decisions

1. **Single Binary**: All functionality is compiled into a single Go binary for easy deployment
2. **Dual Protocol**: Supports both SSH (libssh) and telnet, both adapting to `gliderlabs/ssh.Session`
3. **Menu-Driven**: All functionality is accessed through a hierarchical menu system
4. **Pipe Code Compatibility**: Maintains compatibility with ViSiON/2 pipe codes for colors
5. **Multiple Output Modes**: Supports both UTF-8 and CP437 output for compatibility
6. **JSON Configuration**: Uses JSON for all configuration files for easy editing
7. **JAM Message Bases**: Industry-standard binary message format with indexed random access and per-user lastread tracking
8. **Built-in FTN Tosser**: Native echomail support without external tools (HPT, etc.)
