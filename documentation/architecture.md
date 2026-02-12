# System Architecture

This document outlines the high-level architecture of the ViSiON/3 Go BBS.

## Overview

The system is designed as a single Go application that listens for incoming SSH connections and manages individual user sessions. The main entry point is `cmd/vision3/main.go`.

## Components

1. **Main Application (`cmd/vision3/main.go`)**
   * Initializes logging, configuration, user/message/file managers, and menu executor
   * Sets up the SSH server configuration using gliderlabs/ssh library
   * Loads SSH host keys from `configs/` directory
   * Listens for incoming TCP connections on the configured port (default: 2222)
   * Accepts connections and spawns goroutines to handle individual sessions via `sessionHandler`

2. **Session Handler (`sessionHandler` in `cmd/vision3/main.go`)**
   * Manages the lifecycle of a single SSH session
   * Handles PTY requests and window size changes
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
   * Displays ANSI screens from `menus/v3/ansi/` (`.ANS` files)
   * Handles auto-run commands and user input
   * Integrates with all managers (user, message, file)

4. **User Manager (`internal/user/manager.go`)**
   * Manages user accounts (loading, saving, finding, adding, authenticating)
   * Persists user data to `data/users/users.json`
   * Tracks call records in `data/users/callhistory.json`
   * Handles ACS (Access Control String) validation

5. **Message Manager (`internal/message/manager.go`)**
   * Manages message areas and individual messages
   * Configuration loaded from `data/message_areas.json`
   * Messages stored as JSONL files in `data/` directory (e.g., `messages_area_1.jsonl`)
   * Supports private and public message areas

6. **File Manager (`internal/file/manager.go`)**
   * Manages file areas and file listings
   * Configuration loaded from `configs/file_areas.json`
   * File metadata stored in `data/files/` directory
   * Handles file uploads/downloads and descriptions

7. **ANSI Handler (`internal/ansi/ansi.go`)**
   * Parses ViSiON/2 specific pipe codes (`|00`-`|15`, `|B0`-`|B7`, etc.)
   * Converts CP437 characters to UTF-8 or VT100 line drawing
   * Supports multiple output modes (UTF-8, CP437, Auto)
   * Handles ANSI screen processing and display

8. **Configuration (`configs/` directory)**
   * `strings.json` - Externalized UI strings and prompts
   * `config.json` - General system configuration
   * `doors.json` - External door program configurations
   * `file_areas.json` - File area definitions
   * SSH host keys (`ssh_host_rsa_key`, etc.)

## Data Flow

1. Client connects via SSH → `main` accepts → `sessionHandler` spawned
2. `sessionHandler` handles PTY setup and determines output mode
3. `sessionHandler` starts authentication loop via menu executor
4. Menu executor loads LOGIN menu and handles authentication
5. Successful login transitions to main menu loop (e.g., FASTLOGN or MAIN)
6. Menu executor processes user commands and navigates between menus
7. ANSI screens are loaded from `menus/v3/ansi/` and processed by `ansi` package
8. User/message/file data is persisted to `data/` directory

## Directory Structure

```
vision3/
├── cmd/vision3/         # Main application entry point
├── configs/             # Configuration files
├── data/               # Persistent data
│   ├── users/          # User accounts and call history
│   ├── files/          # File area directories and metadata
│   └── logs/           # Application logs
├── internal/           # Internal packages
│   ├── ansi/           # ANSI/CP437 handling
│   ├── config/         # Configuration loading
│   ├── editor/         # Text editor (placeholder)
│   ├── file/           # File area management
│   ├── menu/           # Menu system
│   ├── message/        # Message area management
│   ├── session/        # Session management
│   ├── terminalio/     # Terminal I/O utilities
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

* `cmd/vision3`: Main application loop, SSH server setup, session handling
* `internal/user`: User data structures, persistence, authentication, ACS logic
* `internal/ansi`: ViSiON/2 pipe code parsing and character encoding
* `internal/config`: Configuration file loading and parsing
* `internal/menu`: Menu loading, display, command execution
* `internal/message`: Message area management and persistence
* `internal/file`: File area management and metadata
* `internal/session`: Session state tracking (currently minimal)
* `menus/v3`: Static menu resources (ANSI art, menu definitions)
* `data`: Persisted application data

## Key Design Decisions

1. **Single Binary**: All functionality is compiled into a single Go binary for easy deployment
2. **SSH-Only**: Uses SSH for secure remote access (no telnet support)
3. **Menu-Driven**: All functionality is accessed through a hierarchical menu system
4. **Pipe Code Compatibility**: Maintains compatibility with ViSiON/2 pipe codes for colors
5. **Multiple Output Modes**: Supports both UTF-8 and CP437 output for compatibility
6. **JSON Configuration**: Uses JSON for all configuration files for easy editing 