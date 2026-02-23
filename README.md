# ViSiON/3 BBS Software

![ViSiON/3](ViSiON3.png)

## Overview

This project is a work-in-progress refactor and modernization of the classic ViSiON/2 BBS software, written in Go. The goal is to recreate the core functionality of the classic BBS experience using modern technologies.

This version uses a **pure-Go SSH server** (`github.com/gliderlabs/ssh`) for SSH functionality, providing full compatibility with legacy BBS terminal software like SyncTerm while maintaining modern security standards. No C dependencies — builds with `CGO_ENABLED=0` and cross-compiles to all platforms.

**Note:** This is currently under active development and is not yet feature-complete.

## STUFF WE NEED

### 🎯 Contributors - Yes, You

**What This Is:** A moderately amusing, functional anachronism.

Are you the kind of person who can wrangle a Go codebase while arguing about why HSLINK was underrated? Do you have strong opinions about ANSI art but also know when to use a mutex? We need someone to lead this glorious mess.

**Technical Chops:**

- Strong Go experience (not just "I vibe coded an app and apparently it works")
- Deep understanding of terminal emulation, ANSI/VT100, character encodings
- Bonus: Network programming experience (SSH, raw sockets)
- Comfortable with legacy protocol implementation (ZMODEM, etc.)
- Can read Pascal/C when needed to understand the original implementations

**Cultural Fit:**

- Either lived through the BBS era OR has become genuinely obsessed with it
- Gets why pipe codes matter and what makes a good door game
- Understands this isn't about making money or padding a resume
- Has opinions about which transfer protocol was best
- Won't try to "modernize" it into a web app

**Working Team:**

- Wrangling this wacko codebase into something proper
- Help building a community of contributors
- Make technical decisions when we need them
- Keep the codebase from turning into spaghetti

**What we won't do:**

- Rewrite this in Rust/JavaScript/whatever
- Add a REST API and React frontend
- Turn it into a web app
- Modernize away what makes it a BBS

If this sounds like your particular flavor of madness, email: **spaceman@vision3bbs.com**

### 🎨 Period-Correct ANSI Artists & Art

Are you an old school ANSi artist (are you younger and infatuated for some reason with that time-period and style)? Do you need one more goddamn thing to do? Consider spending valuable free-time, compensated by nothing more than unyielding appreciation and thanks from the people that enjoy this kind of thing. There's at least 12 of us!

**What we need:**

- **Menu screens** - Main, Message, File, Door menus with that classic warez BBS aesthetic
- **Login/Logoff screens** - Welcome screens, new user applications, goodbye screens
- **Headers and prompts** - Message headers, file listings, user stats displays
- **Transition screens** - Loading screens, pause prompts, error messages
- **Special effects** - Matrix rain, plasma effects, classic BBS animations

**Style we're after:**

- Authentic early 90s underground/warez BBS aesthetic
- Classic color schemes (cyan/magenta highlights, ice colors)
- Scene-style fonts and logos
- Period-appropriate group shoutouts and "greetz"

If you have skills, old .ANS files gathering dust, or just love the aesthetic of the golden age of BBSing, we want to hear from you! Contact us via GitHub issues or pull requests.

### 💻 Go Developers Who Give a Damn

Do you write Go? Do you have fond memories of waiting 3 minutes for a single GIF to download at 14.4k? Are you looking for a project that will impress exactly nobody at your day job but might make a dozen middle-aged nerds unreasonably happy? Boy, do we have the unpaid volunteer opportunity for you!

If you aren't old enough to have experienced it first-hand, have you read a weird text file or listened to some wild-eyed GenX nutjob ramble on about how much we enjoyed it and decided "I need me some of that?"

**Areas where we need help:**

- File transfer protocols (ZMODEM upload support, XMODEM, YMODEM)
- File base management (tagging, batch downloads, upload processing)
- QWK networking support
- TUI/console app creation for admin tools (user editor, system configuration)
- Performance optimization and scalability
- Terminal emulation improvements
- Modern features while maintaining the classic feel
- Testing, bug fixes, and code reviews
- Documentation and examples

Your reward? The satisfaction of knowing that somewhere, someone is reliving their misspent youth thanks to your code. Also, we'll put your handle in the credits. Not your real name though - this is a BBS, we have standards.

**Please submit PRs!**

### 💬 Discord Community Manager

Do we need a Discord? Do you want to host it? Contact me!

**spaceman@vision3bbs.com**

## Current Status

| Feature                       | Status        | Notes                                                                                                               |
| ----------------------------- | ------------- | ------------------------------------------------------------------------------------------------------------------- |
| **Networking**                |               |                                                                                                                     |
| SSH Server                    | ✅ Working     | libssh, PTY support, SyncTerm compatible, modern algorithms, auto-login                                             |
| Telnet Server                 | ✅ Working     | Full IAC negotiation, TERM_TYPE detection                                                                           |
| **Users**                     |               |                                                                                                                     |
| Signup & Authentication       | ✅ Working     | bcrypt hashed passwords, JSON persistence                                                                           |
| User Listings & Stats         | ✅ Working     | Last callers, user listing, call history, stats display                                                             |
| TUI User Editor (`ue`)        | ✅ Working     | Full-screen terminal user management                                                                                |
| **Menus**                     |               |                                                                                                                     |
| Menu System                   | ✅ Working     | `.MNU`, `.CFG`, `.ANS` files, ACS evaluation, password protection                                                   |
| **Messaging**                 |               |                                                                                                                     |
| Message Areas                 | ✅ Working     | JAM format, echomail/netmail, conferences, lightbar reader, threading, quoting, vi-style editor, newscan, last read |
| Private Mail                  | ✅ Working     | User-to-user messaging, send/read/list                                                                              |
| Message List View (scan)      | ✅ Working     | Title/subject scan view                                                                                             |
| **Files**                     |               |                                                                                                                     |
| File Areas (basic)            | ✅ Working     | List areas, list files, select area                                                                                 |
| File Transfers                | ✅ Working     | ZMODEM working via `sexyz`                                                                                          |
| Full File Base                | 📋 In Progress | Tagging, batch downloads, upload processing                                                                         |
| **Doors**                     |               |                                                                                                                     |
| Door/External Programs        | ✅ Working     | Dropfile generation, PTY passthrough                                                                                |
| **Networking/FTN**            |               |                                                                                                                     |
| FTN Echomail/Netmail          | ✅ Working     | JAM-backed, tosser, import/export, dupe checking                                                                    |
| **Admin & Tools**             |               |                                                                                                                     |
| Event Scheduler               | ✅ Working     | Cron-style, automated maintenance, FTN polling                                                                      |
| One-liner System              | ✅ Working     | Retrograde-style                                                                                                    |
| TUI String Editor (`strings`) | ✅ Working     | Full-screen BBS string customizations                                                                               |
| Config Hot Reload             | ✅ Working     | Live reload via fsnotify, no restart required                                                                       |
| Invisible SysOp Login         | ✅ Working     | SysOp/CoSysOp login without appearing in caller log                                                                 |
| SysOp Config TUI              | 📋 Planned     | System configuration editor                                                                                         |
| **Quality**                   |               |                                                                                                                     |
| Comprehensive Testing         | 📋 Planned     |                                                                                                                     |
| Complete Documentation        | 📋 Planned     |                                                                                                                     |

See `tasks/tasks.md` for development history and completed features.

## Technology Stack

*   **Language:** Go 1.24+ (pure Go, no CGO, `CGO_ENABLED=0`)
*   **SSH Server:** `github.com/gliderlabs/ssh` — pure-Go SSH server with legacy algorithm support (SyncTerm, NetRunner compatible)
*   **TUI Framework:** Charmbracelet BubbleTea (`github.com/charmbracelet/bubbletea`) — full-screen terminal editors and admin tools
*   **Event Scheduling:** `github.com/robfig/cron/v3` — cron-style event scheduler
*   **Config Monitoring:** `github.com/fsnotify/fsnotify` — live configuration hot reload
*   **PTY Support:** `github.com/creack/pty` — PTY handling for door programs
*   **Terminal Handling:** `golang.org/x/term`
*   **Password Hashing:** `golang.org/x/crypto/bcrypt`
*   **Message Base:** JAM binary format (echomail/netmail compatible)
*   **Data Format:** JSON (for users and configuration)

## Platform Support

Linux x86_64 is the primary development and testing platform. Support for additional platforms is planned — contributions welcome.

| Platform | Architecture          | Status    | Notes                              |
| -------- | --------------------- | --------- | ---------------------------------- |
| Linux    | x86_64                | ✅ Tested  | Primary development platform       |
| Linux    | ARM64                 | 📋 Planned | Includes Raspberry Pi 4/5 (64-bit) |
| Linux    | ARM (32-bit)          | 📋 Planned | Raspberry Pi 3 and earlier         |
| macOS    | Apple Silicon (ARM64) | 📋 Planned | M1/M2/M3/M4                        |
| macOS    | Intel (x86_64)        | 📋 Planned |                                    |
| Windows  | x86_64                | 📋 Planned |                                    |

> **Note:** ViSiON/3 is pure Go with no C dependencies. Standard Go toolchain is all that's required to build for any supported platform.

## Project Structure

```
vision3/
├── cmd/
│   ├── ansitest/           # ANSI color test utility
│   ├── helper/             # FTN setup utility (import echomail areas)
│   ├── strings/            # TUI string configuration editor
│   ├── ue/                 # TUI user editor
│   ├── v3mail/             # JAM message base and FTN mail processor
│   └── vision3/            # Main BBS server application
├── configs/                # Active configuration files (not tracked in git)
│   ├── allowlist.txt       # IP allowlist for connection filtering
│   ├── blocklist.txt       # IP blocklist for connection filtering
│   ├── config.json         # Main BBS configuration
│   ├── conferences.json    # Message/file conference definitions
│   ├── doors.json          # Door/external program configurations
│   ├── events.json         # Event scheduler (cron-style tasks)
│   ├── file_areas.json     # File area definitions
│   ├── ftn.json            # FidoNet/FTN network configuration
│   ├── login.json          # Login sequence flow definition
│   ├── message_areas.json  # Message area definitions
│   ├── protocols.json      # File transfer protocol configuration
│   ├── strings.json        # BBS string customizations
│   └── ssh_host_rsa_key    # SSH host key
├── templates/              # Configuration templates (tracked in git)
│   └── configs/            # Template configuration files
├── data/                   # Runtime data
│   ├── users/              # User database and call history
│   ├── msgbases/           # JAM format message bases
│   │   ├── general/        # General discussion area
│   │   └── sysop/          # Sysop area
│   ├── files/              # File areas
│   ├── ftn/                # FidoNet/FTN data (packets, tosses, etc.)
│   └── logs/               # Application logs
├── internal/               # Internal packages
│   ├── ansi/               # ANSI/pipe code processing
│   ├── chat/               # Inter-node chat and sysop paging
│   ├── config/             # Configuration loading
│   ├── conference/         # Conference management
│   ├── editor/             # Full-screen text editor (BubbleTea)
│   ├── file/               # File area management
│   ├── ftn/                # FidoNet/echomail support
│   ├── jam/                # JAM message base format
│   ├── menu/               # Menu system & lightbar UI
│   ├── message/            # Message base management
│   ├── scheduler/          # Cron-style event scheduler
│   ├── session/            # Session management
│   ├── sshserver/          # libssh wrapper and adapter
│   ├── stringeditor/       # TUI string configuration editor
│   ├── telnetserver/       # Telnet server
│   ├── terminalio/         # Terminal I/O handling
│   ├── tosser/             # FTN echomail tosser (import/export)
│   ├── transfer/           # File transfer protocols
│   ├── types/              # Shared types
│   ├── user/               # User management
│   ├── usereditor/         # TUI user editor
│   ├── util/               # Utility functions
│   ├── version/            # Version information
│   └── ziplab/             # ZIP archive processing and viewer
├── menus/v3/               # Menu set files
│   ├── ansi/               # ANSI art files
│   ├── bar/                # Lightbar menu definitions
│   ├── cfg/                # Menu configuration files
│   ├── mnu/                # Menu definition files
│   └── templates/          # Display templates
│       └── message_headers/ # Customizable message header styles (unlimited, 14 included)
├── bin/                    # External helper binaries (not tracked in git)
├── output/                 # Output support files
├── scripts/                # Utility scripts
├── docs/                   # GitHub Pages website (vision3bbs.com)
├── documentation/          # Project documentation
└── tasks/                  # Development task tracking
```

## Setup & Installation

### Docker Deployment (Recommended)

The easiest way to run ViSiON/3 is using Docker:

```bash
git clone https://github.com/stlalpha/vision3.git
cd vision3
docker-compose up -d
```

See [Docker Deployment Guide](documentation/docker-deployment.md) for detailed instructions.

### Manual Installation

#### System Requirements

**Go 1.24+** is the only build requirement. No system libraries or C compiler needed.

```bash
# Verify Go version
go version
```

### Quick Setup

1. **Clone the repository:**
    ```bash
    git clone https://github.com/stlalpha/vision3.git
    cd vision3
    ```

2. **Run the setup script:**
    ```bash
    ./setup.sh
    ```

   This will:
   - Generate SSH host keys
   - Copy template configuration files from `templates/configs/` to `configs/`
   - Create necessary directory structure
   - Create initial data files
   - Build the application

4. **Configure your BBS:**
    ```bash
    # Edit configs/config.json with your BBS settings
    nano configs/config.json
    ```

5. **Build and run the BBS:**
    ```bash
    ./build.sh
    ./vision3
    ```

### Manual Setup

If you prefer to set up manually:

1. **Copy configuration templates:**
    ```bash
    cp templates/configs/*.json configs/
    # Edit configs/config.json with your BBS settings
    ```

3. **Build the application:**
    ```bash
    ./build.sh
    ```
    This builds all binaries (`vision3`, `helper`, `v3mail`, `strings`, `ue`) to the root directory.

4. **Generate SSH Host Key:**
    ```bash
    ssh-keygen -t rsa -b 2048 -f configs/ssh_host_rsa_key -N ""
    ```

5. **Create directories:**
    ```bash
    mkdir -p data/users data/files/general data/logs data/msgbases
    ```

6. **Run the server:**
    ```bash
    ./vision3
    ```

The server listens on port 2222 by default.

## Default Login

The system creates a default user on first run:
- **Username:** felonius
- **Password:** password

**IMPORTANT:** Change this password after first login!

## Connecting

Connect using any SSH client:

```bash
ssh felonius@localhost -p 2222
```

### SyncTerm and Retro Terminal Support

ViSiON/3 fully supports **SyncTerm** and other classic BBS terminal emulators:
- Automatic CP437 encoding for authentic ANSI graphics
- Compatible with modern SSH algorithms (no legacy crypto needed)
- SSH-authenticated users bypass the login screen

Download SyncTerm: https://syncterm.bbsdev.net/

## Command Line Options

```bash
./vision3 --output-mode=auto
```

- `--output-mode`: Terminal output mode (auto, utf8, cp437)
  - `auto`: Automatically detect based on terminal type (default)
  - `utf8`: Force UTF-8 output
  - `cp437`: Force CP437 output for authentic DOS/BBS experience

## Configuration

Configuration files are located in the `configs/` directory:

### Core Configuration

All configuration files are located in `configs/` and are generated from templates in `templates/configs/` during setup:

- **`config.json`**: Main BBS configuration (BBS name, ports, paths, connection limits)
  - **Note:** Not tracked in git. Copied from template during setup
  - Includes connection security settings: `maxNodes` and `maxConnectionsPerIP`
- **`theme.json`**: Color theme configuration (lightbar colors, prompt colors, ANSI codes)
- **`strings.json`**: Customizable BBS prompts and messages

### Message System

- **`message_areas.json`**: Message area definitions (local, echomail, netmail areas)
- **`conferences.json`**: Conference groupings for message and file areas
- **`ftn.json`**: FidoNet/FTN network configuration (addresses, uplinks, packet passwords)

### File System

- **`file_areas.json`**: File area definitions (paths, descriptions, access levels)

### External Programs

- **`doors.json`**: Door/external program configurations (paths, dropfile types, pause settings)

### Security

- **`ssh_host_rsa_key`**: SSH host key (auto-generated on first run if missing)

All configuration files use JSON format and support comments for documentation.

## Documentation

- [Docker Deployment Guide](documentation/docker-deployment.md) - Deploy using Docker and Docker Compose
- [Security Guide](documentation/security.md) - Connection security, IP filtering, and best practices
- [Configuration Guide](documentation/configuration.md) - Detailed configuration options
- [SSH Server Migration Guide](documentation/ssh-server-migration.md) - Details about the libssh implementation
- [Developer Guide](documentation/developer-guide.md) - Guide for contributors
- [Development Tasks](tasks/tasks.md) - Development history and completed features

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## Acknowledgments

This project is built in tribute to ViSiON/2 and my friend Crimson Blade.

For the original ViSiON/2 BBS (Pascal version), see: [vision-2-bbs](https://github.com/stlalpha/vision-2-bbs) 