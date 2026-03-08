# ViSiON/3 BBS Software

![ViSiON/3](ViSiON3.png)

## Overview

This project is a work-in-progress refactor and modernization of the classic ViSiON/2 BBS software, written in Go. The goal is to recreate the core functionality of the classic BBS experience using modern technologies.

This version uses a **pure-Go SSH server** (`github.com/gliderlabs/ssh`) for SSH functionality, providing full compatibility with legacy BBS terminal software like SyncTerm while maintaining modern security standards.

**Note:** This is currently under active development and is not yet feature-complete.

## Community

Join us on Discord to get involved, follow development and to connect with other contributors and BBS enthusiasts:

👉 **[Join the ViSiON/3 Discord](https://discord.gg/trZ9VSsF3r)**

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

- File transfer protocols beyond Zmodem (XMODEM, YMODEM)
- Performance optimization and scalability
- Terminal emulation improvements
- Modern features while maintaining the classic feel
- Testing, bug fixes, and code reviews

Your reward? The satisfaction of knowing that somewhere, someone is reliving their misspent youth thanks to your code. Also, we'll put your handle in the credits. Not your real name though - this is a BBS, we have standards.

**Please submit PRs!**

## Current Status

| Feature                       | Status        | Notes                                                                                                               |
| ----------------------------- | ------------- | ------------------------------------------------------------------------------------------------------------------- |
| **Networking**                |               |                                                                                                                     |
| SSH Server                    | ✅ Working     | Pure-Go (gliderlabs/ssh), PTY support, SyncTerm compatible, legacy algorithms, auto-login                           |
| Telnet Server                 | ✅ Working     | Full IAC negotiation, TERM_TYPE detection                                                                           |
| **Users**                     |               |                                                                                                                     |
| Signup & Authentication       | ✅ Working     | bcrypt hashed passwords, JSON persistence                                                                           |
| User Listings & Stats         | ✅ Working     | Last callers, user listing, call history, stats display                                                             |
| NUV (New User Verification)   | ✅ Working     | Voting-based approval system for new user accounts                                                                  |
| TUI User Editor (`ue`)        | ✅ Working     | Full-screen terminal user management                                                                                |
| **Menus**                     |               |                                                                                                                     |
| Menu System                   | ✅ Working     | `.MNU`, `.CFG`, `.ANS` files, ACS evaluation, password protection                                                   |
| TUI Menu Editor (`menuedit`)  | ✅ Working     | Full-screen menu configuration editor                                                                               |
| **Messaging**                 |               |                                                                                                                     |
| Message Areas                 | ✅ Working     | JAM format, echomail/netmail, conferences, lightbar reader, threading, quoting, vi-style editor, newscan, last read |
| Private Mail                  | ✅ Working     | User-to-user messaging, send/read/list                                                                              |
| Message List View (scan)      | ✅ Working     | Title/subject scan view                                                                                             |
| QWK Offline Mail              | ✅ Working     | QWK packet download/upload for offline reading                                                                      |
| **Files**                     |               |                                                                                                                     |
| File Areas                    | ✅ Working     | List areas, list files, select area, archive viewing                                                                |
| File Transfers                | ✅ Working     | ZMODEM upload/download via `sexyz`                                                                                  |
| File Management               | ✅ Working     | SysOp file delete, move between areas, edit descriptions                                                            |
| **Doors**                     |               |                                                                                                                     |
| Door/External Programs        | ✅ Working     | Dropfile generation, PTY passthrough                                                                                |
| **Networking/FTN**            |               |                                                                                                                     |
| FTN Echomail/Netmail          | ✅ Working     | JAM-backed, tosser, import/export, dupe checking                                                                    |
| **Community Features**        |               |                                                                                                                     |
| Voting System                 | ✅ Working     | Voting booths, mandatory topics, SysOp management                                                                   |
| News System                   | ✅ Working     | SysOp-managed news items, auto-display on login                                                                     |
| Chat & SysOp Paging           | ✅ Working     | Inter-node chat, SysOp page                                                                                         |
| One-liner System              | ✅ Working     |                                                                                                                      |
| Sponsor Menus                 | ✅ Working     | Area management for SysOps and area sponsors                                                                        |
| **Admin & Tools**             |               |                                                                                                                     |
| Event Scheduler               | ✅ Working     | Cron-style, automated maintenance, FTN polling                                                                      |
| Admin Functions               | ✅ Working     | User validation, banning, deletion, purge                                                                           |
| TUI String Editor (`strings`) | ✅ Working     | Full-screen BBS string customizations                                                                               |
| Config Hot Reload             | ✅ Working     | Live reload via fsnotify, no restart required                                                                       |
| Invisible SysOp Login         | ✅ Working     | SysOp/CoSysOp login without appearing in caller log                                                                 |
| SysOp Config TUI (`config`)   | ✅ Working     | Full-screen TUI configuration editor (`cmd/config`)                                                                 |
| **Quality**                   |               |                                                                                                                     |
| Comprehensive Testing         | 📋 Planned     |                                                                                                                     |
| Complete Documentation        | 📋 Planned     |                                                                                                                     |

See `tasks/tasks.md` for development history and completed features.

## Technology Stack

*   **Language:** Go 1.24+
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

Pre-built releases are available for all major platforms. Linux x86_64 is the primary development platform.

| Platform | Architecture          | Status      | Notes                               |
| -------- | --------------------- | ----------- | ----------------------------------- |
| Linux    | x86_64                | ✅ Tested    | Primary development platform        |
| Linux    | ARM64                 | ✅ Released  | Includes Raspberry Pi 4/5 (64-bit)  |
| Linux    | ARMv7                 | ✅ Released  | Raspberry Pi 3 and earlier          |
| macOS    | Universal             | ✅ Released  | Intel + Apple Silicon (M1/M2/M3/M4) |
| Windows  | x86_64                | ✅ Released  |                                     |

> **Note:** ViSiON/3 is pure Go. The standard Go toolchain is all that's required to build for any supported platform.

## Project Structure

```
vision3/
├── cmd/
│   ├── ansitest/           # ANSI color test utility
│   ├── config/             # TUI system configuration editor
│   ├── helper/             # FTN setup utility (import echomail areas)
│   ├── menuedit/           # TUI menu editor
│   ├── strings/            # TUI string configuration editor
│   ├── ue/                 # TUI user editor
│   ├── v3mail/             # JAM message base and FTN mail processor
│   └── vision3/            # Main BBS server application
├── configs/                # Active configuration files (not tracked in git)
│   ├── allowlist.txt       # IP allowlist for connection filtering
│   ├── archivers.json      # Archive handler configurations
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
│   ├── archiver/           # Archive format handling (ZIP, etc.)
│   ├── chat/               # Inter-node chat and sysop paging
│   ├── config/             # Configuration loading
│   ├── configeditor/       # TUI configuration editor (BubbleTea)
│   ├── conference/         # Conference management
│   ├── editor/             # Full-screen text editor (BubbleTea)
│   ├── file/               # File area management
│   ├── ftn/                # FidoNet/echomail support
│   ├── jam/                # JAM message base format
│   ├── menu/               # Menu system & lightbar UI
│   ├── menueditor/         # TUI menu editor (BubbleTea)
│   ├── message/            # Message base management
│   ├── qwk/                # QWK offline mail packet format
│   ├── scheduler/          # Cron-style event scheduler
│   ├── session/            # Session management
│   ├── sshserver/          # pure-Go SSH server (gliderlabs/ssh wrapper)
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
│       └── message_headers/ # Customizable message header styles (unlimited, 16 included)
├── bin/                    # External helper binaries (not tracked in git)
├── output/                 # Output support files
├── scripts/                # Utility scripts
├── docs/                   # GitHub Pages website (vision3bbs.com)
├── docs/sysop/             # Sysop documentation
└── tasks/                  # Development task tracking
```

## Setup & Installation

> **Third-party binaries:** `sexyz` (ZMODEM file transfers) and `binkd` (FTN echomail/netmail) are not built by this project. Pre-compiled binaries for these — along with all ViSiON/3 binaries — are available on the [GitHub Releases page](https://github.com/stlalpha/vision3/releases).

### Option 1: Download a Pre-Built Release

The fastest way to get started — no Go toolchain required.

1. Download the archive for your platform from the [GitHub Releases page](https://github.com/stlalpha/vision3/releases)
2. Extract it and run the setup script:
    ```bash
    tar -xzf vision3_<platform>.tar.gz   # Linux/macOS
    cd vision3
    ./setup.sh                            # Linux/macOS
    .\setup.ps1                           # Windows (PowerShell)
    ```
3. Edit `configs/config.json` with your BBS settings
4. Run the server:
    ```bash
    ./vision3         # Linux/macOS
    .\vision3.exe     # Windows
    ```

### Option 2: Docker (Recommended for Production)

```bash
git clone https://github.com/stlalpha/vision3.git
cd vision3
docker-compose up -d
```

See [Docker Deployment Guide](docs/sysop/getting-started/docker.md) for detailed instructions.

### Option 3: Build from Source

**Go 1.24+** is the only build requirement.

1. **Clone and set up:**
    ```bash
    git clone https://github.com/stlalpha/vision3.git
    cd vision3
    ./setup.sh          # Linux/macOS
    .\setup.ps1         # Windows (PowerShell)
    ```

    `setup.sh` will generate SSH host keys, copy template configs to `configs/`, create the required directory structure, and build all binaries (`vision3`, `helper`, `v3mail`, `strings`, `ue`, `config`, `menuedit`).

2. **Configure your BBS:**
    ```bash
    nano configs/config.json
    ```

3. **Run the server:**
    ```bash
    ./vision3           # Linux/macOS
    .\vision3.exe       # Windows
    ```

The server listens on port 2222 (SSH) and 2323 (Telnet) by default.

## Default Login

The system creates a default user on first run:
- **Username:** felonius
- **Password:** password

**IMPORTANT:** Change this password after first login!

## Connecting

```bash
ssh felonius@localhost -p 2222
```

### SyncTerm and Retro Terminal Support

ViSiON/3 fully supports **SyncTerm**, **NetRunner**, and other classic BBS terminal emulators:
- Automatic CP437 encoding for authentic ANSI graphics
- Compatible with modern SSH algorithms

Download SyncTerm: https://syncterm.bbsdev.net/

## Command Line Options

```bash
./vision3 --output-mode=auto
```

- `--output-mode`: Terminal output mode (`auto` / `utf8` / `cp437`)
  - `auto`: Automatically detect based on terminal type (default)
  - `utf8`: Force UTF-8 output
  - `cp437`: Force CP437 output for authentic DOS/BBS experience

## Configuration

All configuration files live in `configs/` and are generated from templates in `templates/configs/` during setup. See the [SysOp Documentation](https://vision3bbs.com/sysop/) for full configuration reference.

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## Acknowledgments

This project is built in tribute to ViSiON/2 and my friend Crimson Blade.

For the original ViSiON/2 BBS (Pascal version), see: [vision-2-bbs](https://github.com/stlalpha/vision-2-bbs) 