# ViSiON/3 Installation Guide

## Prerequisites

- Go 1.24.2 or higher
- Git
- SSH client for testing
- `libssh` - C library for SSH (required by the SSH server)
- `pkg-config` - Used to locate libssh during build

### Installing C Dependencies

**macOS (Homebrew):**

```bash
brew install libssh pkg-config
```

**Debian/Ubuntu:**

```bash
sudo apt install libssh-dev pkg-config
```

**Fedora:**

```bash
sudo dnf install libssh-devel pkgconf-pkg-config
```

## Installation Steps

### 1. Clone the Repository

```bash
git clone https://github.com/stlalpha/vision3.git
cd vision3
```

### 2. Build the Application

```bash
cd cmd/vision3
go build
```

This creates the `vision3` executable in the `cmd/vision3/` directory.

### 3. Generate SSH Host Key

The BBS requires an SSH host key for secure connections:

```bash
cd ../../configs
ssh-keygen -t rsa -f ssh_host_rsa_key -N ""
cd ..
```

### 4. Verify Directory Structure

Ensure these directories exist (they should be included in the repository):

- `configs/` - Configuration files
- `data/` - Runtime data
- `data/users/` - User database
- `data/msgbases/` - JAM message base files (created automatically)
- `data/logs/` - Log files
- `menus/v3/` - Menu system files

### 5. Initial Configuration

The system includes default configuration files in the `configs/` directory:

- `strings.json` - BBS text strings and prompts
- `doors.json` - External door program configurations
- `file_areas.json` - File area definitions
- `config.json` - General BBS settings (ports, security levels, connection limits)
  - `maxNodes`: Maximum simultaneous connections (default: 10, 0 = unlimited)
  - `maxConnectionsPerIP`: Maximum connections per IP (default: 3, 0 = unlimited)
  - `ipBlocklistPath`: Path to IP blocklist file (optional)
  - `ipAllowlistPath`: Path to IP allowlist file (optional, bypasses all limits)
  - `sysOpLevel`: Security level for SysOp access (default: 255)
  - `coSysOpLevel`: Security level for Co-SysOp access (default: 250)
- `message_areas.json` - Message area definitions (JAM base paths, area types)

Review and modify these as needed for your setup.

### 6. Run the Server

```bash
cd cmd/vision3
./vision3
```

By default, the server listens on port 2222.

### 7. First Login

Connect to your BBS:

```bash
ssh felonius@localhost -p 2222
```

Default credentials:

- Username: `felonius`
- Password: `password`

**Important**: Change this password immediately after first login!

## Command Line Options

```bash
./vision3 --output-mode=<mode>
```

Available output modes:

- `auto` - Automatically detect based on terminal (default)
- `utf8` - Force UTF-8 output
- `cp437` - Force CP437 output for authentic BBS experience

## Directory Structure After Installation

```text
vision3/
├── cmd/vision3/vision3    # The compiled executable
├── configs/               # Configuration files
│   ├── config.json
│   ├── doors.json
│   ├── file_areas.json
│   ├── strings.json
│   └── ssh_host_rsa_key  # SSH host key (generated)
├── data/                  # Runtime data (created automatically)
│   ├── users/
│   │   └── users.json    # User database
│   ├── msgbases/         # JAM message bases (.jhr/.jdt/.jdx/.jlr)
│   ├── ftn/              # FTN echomail data (if enabled)
│   │   ├── inbound/      # Incoming .PKT files
│   │   ├── outbound/     # Outgoing .PKT files
│   │   └── dupes.json    # MSGID dupe database
│   └── logs/
│       └── vision3.log   # Application log
├── bin/                   # External binaries (binkd, hpt for echomail, etc.) - created empty
├── scripts/               # Helper shell and Python scripts (future use) - created empty
└── menus/v3/             # Menu system files
```

## Troubleshooting

### Port Already in Use

If port 2222 is already in use, you'll need to modify the port in `cmd/vision3/main.go` (search for `sshPort := 2222`).

### Permission Denied

Ensure the executable has proper permissions:

```bash
chmod +x vision3
```

### SSH Key Issues

If you encounter SSH key errors, ensure the key was generated in the correct location (`configs/ssh_host_rsa_key`).

## Next Steps

- Review the [Configuration Guide](configuration.md) to customize your BBS
- Set up [Message Areas](message-areas.md) and [File Areas](file-areas.md)
- Configure [Door Programs](doors.md) if desired
- Refer to [User Management](user-management.md) for managing users
