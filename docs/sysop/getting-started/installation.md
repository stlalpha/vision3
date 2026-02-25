# ViSiON/3 Installation Guide

> **Current Version: v0.1.0** — [GitHub Releases](https://github.com/stlalpha/vision3/releases)
>
> ⚠️ **Early Development:** ViSiON/3 is under active development and not yet feature-complete. Expect rough edges and missing features. Use at your own risk. Check the [releases page](https://github.com/stlalpha/vision3/releases) and the [README](https://github.com/stlalpha/vision3) for the current status of features.

---

## Installation Options

| Option | Best For |
|--------|----------|
| [Download Pre-Built Release](#option-1-download-pre-built-release) | Fastest path — no Go toolchain required |
| [Build from Source](#option-2-build-from-source) | Contributors, or to run unreleased code |
| [Docker Deployment](docker.md) | Containerized / production setup |

---

## Option 1: Download Pre-Built Release

No Go toolchain or build tools required. Download, extract, and run.

### Available Platforms

| Platform | Architecture | Archive |
|----------|-------------|---------|
| Linux | x86_64 (amd64) | `vision3_linux_amd64.tar.gz` |
| Linux | ARM64 | `vision3_linux_arm64.tar.gz` |
| Linux | ARMv7 (Raspberry Pi 3) | `vision3_linux_armv7.tar.gz` |
| macOS | Universal (Intel + Apple Silicon) | `vision3_darwin_universal.tar.gz` |
| Windows | x86_64 | `vision3_windows_amd64.zip` |

Download from: [https://github.com/stlalpha/vision3/releases](https://github.com/stlalpha/vision3/releases)

### Steps

1. **Download and extract the archive for your platform:**

   **Linux / macOS:**
   ```bash
   tar -xzf vision3_<platform>.tar.gz
   cd vision3
   ```

   **Windows (PowerShell):**
   ```powershell
   Expand-Archive vision3_windows_amd64.zip
   cd vision3
   ```

2. **Run the setup script:**

   **Linux / macOS:**
   ```bash
   ./setup.sh
   ```

   **Windows (PowerShell):**
   ```powershell
   .\setup.ps1
   ```

   This will:
   - Generate SSH host keys
   - Copy template configs to `configs/`
   - Create required directory structure
   - Create initial data files

3. **Configure your BBS:**

   ```bash
   nano configs/config.json       # Linux/macOS
   notepad configs\config.json    # Windows
   ```

   See the [Configuration Guide](../configuration/configuration.md) for all settings.

4. **Start the BBS:**

   **Linux / macOS:**
   ```bash
   ./vision3
   ```

   **Windows:**
   ```powershell
   .\vision3.exe
   ```

5. **Connect and verify:**

   ```bash
   ssh felonius@localhost -p 2222
   # Default password: password
   ```

   **Important:** Change the default password immediately after first login!

> **Note:** Release archives include all binaries: `vision3`, `v3mail`, `helper`, `strings`, `ue`, and `sexyz` (ZModem file transfers).

---

## Option 2: Build from Source

### Prerequisites

- **Go 1.24+** — the only build requirement ([install Go](https://golang.org/dl/))
- Git
- SSH client for testing

> **Note:** ViSiON/3 uses a pure-Go SSH server (`gliderlabs/ssh`). No CGO, libssh, or pkg-config is required.

### 1. Clone the Repository

```bash
git clone https://github.com/stlalpha/vision3.git
cd vision3
```

### 2. Run the Setup Script

**Linux / macOS:**
```bash
./setup.sh
```

**Windows (PowerShell):**
```powershell
.\setup.ps1
```

This will:
- Generate SSH host keys
- Copy template configuration files to `configs/`
- Create the necessary directory structure and initial data files
- Build all binaries (`vision3`, `v3mail`, `helper`, `strings`, `ue`)

### 3. Configure Your BBS

```bash
nano configs/config.json
```

See the [Configuration Guide](../configuration/configuration.md) for all settings.

### 4. Start the Server

```bash
./vision3
```

The server listens on port 2222 (SSH) and 2323 (Telnet) by default.

### 5. First Login

```bash
ssh felonius@localhost -p 2222
# Default password: password
```

**Important:** Change the default password immediately after first login.

---

## Command Line Options

```bash
./vision3 --output-mode=<mode>
```

Available output modes:

- `auto` — Detect based on terminal type (default)
- `utf8` — Force UTF-8 output
- `cp437` — Force CP437 for authentic BBS experience

---

## File Transfer Binary: sexyz

**sexyz** is Synchronet's ZModem 8k implementation used for file transfers on both SSH and telnet connections. It is included in the release archive and in `bin/sexyz` in the source tree. No separate installation is needed.

If you need to build it for a different platform, see [File Transfer Protocols](../files/file-transfer.md).

---

## Directory Structure After Installation

```text
vision3/
├── vision3              # Main BBS server binary
├── v3mail               # JAM message base / FTN mail processor
├── helper               # FTN setup utility
├── strings              # TUI string configuration editor
├── ue                   # TUI user editor
├── configs/             # Configuration files
│   ├── config.json
│   ├── doors.json
│   ├── file_areas.json
│   ├── strings.json
│   └── ssh_host_rsa_key # SSH host key (auto-generated)
├── data/                # Runtime data (created automatically)
│   ├── users/           # User database and call history
│   ├── msgbases/        # JAM message bases
│   ├── ftn/             # FidoNet/echomail data
│   └── logs/            # Application logs (vision3.log)
├── bin/
│   └── sexyz            # ZModem file transfer binary
└── menus/v3/            # Menu system files
```

---

## Troubleshooting

### Port Already in Use

Change `"sshPort": 2222` in `configs/config.json` and restart the server.

### Permission Denied

```bash
chmod +x vision3
```

### SSH Key Issues

If you encounter SSH key errors, ensure the key exists at `configs/ssh_host_rsa_key`. Re-run `setup.sh` to regenerate it.

---

## Next Steps

- Review the [Configuration Guide](../configuration/configuration.md) to customize your BBS
- Set up [Message Areas](../messages/message-areas.md) and [File Areas](../files/file-areas.md)
- Configure [Door Programs](../doors/doors.md) if desired
- Review [File Transfer Protocols](../files/file-transfer.md) (sexyz ZModem 8k)
- Refer to [User Management](../users/user-management.md) for managing users
