# ViSiON/3 BBS Software

![ViSiON/3](ViSiON3.png)

[![Go Report Card](https://goreportcard.com/badge/github.com/stlalpha/vision3)](https://goreportcard.com/report/github.com/stlalpha/vision3)

## Overview

This project is a work-in-progress refactor and modernization of the classic ViSiON/2 BBS software, written in Go. The goal is to recreate the core functionality of the classic BBS experience using modern technologies.

This version utilizes the `gliderlabs/ssh` library for handling SSH connections.

**Note:** This is currently under active development and is not yet feature-complete.

## STUFF WE NEED

### 🎯 Project Lead - Yes, You

**What This Is:** A moderately amusing, functional anachronism.

Are you the kind of person who can wrangle a Go codebase while arguing about why HSLINK was underrated? Do you have strong opinions about ANSI art but also know when to use a mutex? We need someone to lead this glorious mess.

**Technical Chops:**
- Strong Go experience (not just "I did the tour once")
- Deep understanding of terminal emulation, ANSI/VT100, character encodings
- Network programming experience (SSH, raw sockets)
- Comfortable with legacy protocol implementation (ZMODEM, etc.)
- Can read Pascal/C when needed to understand the original implementations

**Cultural Fit:**
- Either lived through the BBS era OR has become genuinely obsessed with it
- Gets why pipe codes matter and what makes a good door game
- Understands this isn't about making money or padding a resume
- Has opinions about which transfer protocol was best
- Won't try to "modernize" it into a web app

**Working Relationship:**

**I will provide:**
- Funding when we need something specific
- Cover any actual costs so nobody's out of pocket
- Keep the lights on while you focus on the code

**You will provide:**
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

Are you a 40+ old school ANSi artist (are you younger and infatuated for some reason with that time-period and style)? Do you need one more goddamn thing to do? Consider spending valuable free-time, compensated by nothing more than unyielding appreciation and thanks from the people that enjoy this kind of thing. There's at least 12 of us!

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

If you have TheDraw skills, old .ANS files gathering dust, or just love the aesthetic of the golden age of BBSing, we want to hear from you! Contact us via GitHub issues or pull requests.

### 💻 Go Developers Who Give a Damn

Do you write Go? Do you have fond memories of waiting 3 minutes for a single GIF to download at 14.4k? Are you looking for a project that will impress exactly nobody at your day job but might make a dozen middle-aged nerds unreasonably happy? Boy, do we have the unpaid volunteer opportunity for you!

If you aren't old enough to have experienced it first-hand, have you read a weird text file or listened to some wild-eyed GenX nutjob ramble on about how much we enjoyed it and decided "I need me some of that?"

**Areas where we need help:**
- File transfer protocols (ZMODEM upload support, XMODEM, YMODEM)
- Message threading and advanced message base features
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

### Working Features

*   SSH Server with PTY support (via `gliderlabs/ssh`)
*   User Authentication (bcrypt hashed passwords)
*   User Persistence (`data/users/users.json`)
*   Menu System Loading & Execution (`.MNU`, `.CFG`, `.ANS` files)
*   Access Control System (ACS) Evaluation with basic operators (`!`, `&`, `|`, `()`)
*   Menu Password Protection
*   Message Areas (basic implementation):
    *   List message areas
    *   Compose messages
    *   Read messages
    *   Newscan functionality
*   File Areas (basic implementation):
    *   List files
    *   List file areas
    *   Select file area
*   User Statistics Display
*   Last Callers Display
*   User Listing
*   One-liner System
*   Door/External Program Support (with dropfile generation)
*   Call History Tracking

### In Development / TODO

*   Full Message Base Implementation (threading, replies, etc.)
*   File Transfer Protocols (upload/download)
*   Complete SysOp Tools
*   User Editor
*   Full File Base Implementation
*   Comprehensive Testing
*   Complete Documentation

See `docs/status.md` for detailed progress and `tasks/tasks.md` for specific development tasks.

## Technology Stack

*   **Language:** Go 1.24.2
*   **SSH Library:** `github.com/gliderlabs/ssh`
*   **Terminal Handling:** `golang.org/x/term`
*   **Password Hashing:** `golang.org/x/crypto/bcrypt`
*   **Data Format:** JSON (for users, configuration, and message storage)

## Project Structure

```
vision3/
├── cmd/
│   ├── ansitest/        # ANSI color test utility
│   └── vision3/         # Main BBS server application
├── configs/             # Global configuration files
│   ├── config.json      
│   ├── doors.json       # Door/external program configurations
│   ├── file_areas.json  # File area definitions
│   ├── strings.json     # BBS string customizations
│   └── ssh_host_rsa_key # SSH host key
├── data/                # Runtime data
│   ├── users/           # User database and call history
│   ├── files/           # File areas
│   ├── logs/            # Application logs
│   └── message_*.jsonl  # Message base files
├── internal/            # Internal packages
│   ├── ansi/            # ANSI/pipe code processing
│   ├── config/          # Configuration loading
│   ├── editor/          # Text editor
│   ├── file/            # File area management
│   ├── menu/            # Menu system
│   ├── message/         # Message base system
│   ├── session/         # Session management
│   ├── terminalio/      # Terminal I/O handling
│   ├── transfer/        # File transfer protocols
│   ├── types/           # Shared types
│   └── user/            # User management
├── menus/v3/            # Menu set files
│   ├── ansi/            # ANSI art files
│   ├── cfg/             # Menu configuration files
│   ├── mnu/             # Menu definition files
│   └── templates/       # Display templates
├── docs/                # Documentation
└── tasks/               # Development task tracking
```

## Setup & Installation

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

   This script will:
   - Generate SSH host keys
   - Create necessary directories
   - Initialize data files
   - Build the BBS executable

3. **Run the server:**
    ```bash
    cd cmd/vision3
    ./vision3
    ```

### Manual Setup

If you prefer to set up manually:

1. **Build the application:**
    ```bash
    cd cmd/vision3
    go build
    ```

2. **Generate SSH Host Keys:**
    ```bash
    cd configs
    ssh-keygen -t rsa -f ssh_host_rsa_key -N ""
    ssh-keygen -t ed25519 -f ssh_host_ed25519_key -N ""
    ssh-keygen -t dsa -f ssh_host_dsa_key -N ""
    ```

3. **Create directories:**
    ```bash
    mkdir -p data/users data/files/general log
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

- `strings.json`: Customize BBS prompts and messages
- `doors.json`: Configure external door programs
- `file_areas.json`: Define file areas
- `config.json`: General BBS configuration

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## Acknowledgments

This project is built in tribute to ViSiON/2 and my friend Crimson Blade.

For the original ViSiON/2 BBS (Pascal version), see: [vision-2-bbs](https://github.com/stlalpha/vision-2-bbs) 