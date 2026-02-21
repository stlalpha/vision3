# File Transfer Protocols

This document covers the file transfer protocol system in ViSiON/3, including setup, configuration, and platform-specific dependencies.

## Overview

ViSiON/3 uses external programs for file transfers. Protocol definitions are stored in `configs/protocols.json`, allowing sysops to configure which transfer engines are available. Two protocols are supported out of the box:

| Protocol  | Engine             | Key            | Best For           | PTY Required |
| --------- | ------------------ | -------------- | ------------------ | ------------ |
| ZModem    | lrzsz (`sz`/`rz`)  | `zmodem-lrzsz` | SSH connections    | Yes          |
| ZModem 8k | sexyz (Synchronet) | `zmodem-sexyz` | Telnet connections | No           |

## Dependencies

### lrzsz (Recommended — Default Protocol)

lrzsz provides the standard ZModem `sz` (send) and `rz` (receive) commands used by the default transfer protocol. This is the primary file transfer dependency.

**Linux (Debian/Ubuntu):**

```bash
sudo apt install lrzsz
```

**Linux (Fedora/RHEL):**

```bash
sudo dnf install lrzsz
```

**Linux (Arch):**

```bash
sudo pacman -S lrzsz
```

**Linux (Alpine):**

lrzsz is not in Alpine's standard repos. It is built from source in the Dockerfile:

```dockerfile
RUN cd /tmp && \
    wget -q https://www.ohse.de/uwe/releases/lrzsz-0.12.20.tar.gz && \
    tar xzf lrzsz-0.12.20.tar.gz && \
    cd lrzsz-0.12.20 && \
    ./configure --prefix=/usr/local && \
    make && make install
```

**macOS (Homebrew):**

```bash
brew install lrzsz
```

**Windows (WSL):**

```bash
# Inside WSL (Debian/Ubuntu)
sudo apt install lrzsz
```

**Windows (native — MSYS2):**

```bash
pacman -S lrzsz
```

**Windows (native — Cygwin):**

Install `lrzsz` package from the Cygwin installer.

### sexyz (Optional — Synchronet ZModem 8k)

sexyz is Synchronet's external file transfer program providing ZModem with 8k blocks. It operates directly on TCP sockets (no PTY needed), making it ideal for telnet connections.

**sexyz is NOT available through standard package managers.** It must be obtained from Synchronet BBS:

- Website: https://www.synchro.net
- Source: https://gitlab.synchro.net

#### Obtaining sexyz

1. **From Synchronet builds:**
   - Download the appropriate build for your platform from the Synchronet website
   - Extract the `sexyz` binary

2. **From source:**
   - Clone the Synchronet repository from GitLab
   - Build with the provided makefiles
   - The sexyz binary is part of the `exec/` output

3. **Install:**
   ```bash
   cp sexyz /path/to/vision3/bin/sexyz
   chmod +x /path/to/vision3/bin/sexyz
   ```

#### Platform Availability

| Platform     | Availability                            |
| ------------ | --------------------------------------- |
| Linux x86_64 | Pre-built binaries or build from source |
| Linux ARM64  | Build from source                       |
| macOS        | Build from source                       |
| Windows      | Pre-built .exe from Synchronet builds   |

## Configuration

Protocols are defined in `configs/protocols.json`:

```json
[
  {
    "key": "zmodem-lrzsz",
    "name": "ZModem (lrzsz)",
    "description": "ZModem via lrzsz sz/rz — standard external transfer",
    "send_cmd": "sz",
    "send_args": ["-b", "-e", "{filePath}"],
    "recv_cmd": "rz",
    "recv_args": ["-b", "-r"],
    "batch_send": true,
    "use_pty": true,
    "default": true
  },
  {
    "key": "zmodem-sexyz",
    "name": "ZModem 8k (sexyz)",
    "description": "ZModem 8k via Synchronet sexyz — recommended for telnet",
    "send_cmd": "sexyz",
    "send_args": ["{socket}", "sz", "{filePath}"],
    "recv_cmd": "sexyz",
    "recv_args": ["{socket}", "rz"],
    "batch_send": true,
    "use_pty": false,
    "default": false
  }
]
```

### Configuration Fields

| Field         | Type     | Description                                                |
| ------------- | -------- | ---------------------------------------------------------- |
| `key`         | string   | Machine-readable identifier (e.g., `zmodem-lrzsz`)         |
| `name`        | string   | Display name shown to users                                |
| `description` | string   | Short description for help/selection menus                 |
| `send_cmd`    | string   | Executable path for sending files (download to user)       |
| `send_args`   | string[] | Arguments for send command                                 |
| `recv_cmd`    | string   | Executable path for receiving files (upload from user)     |
| `recv_args`   | string[] | Arguments for receive command                              |
| `batch_send`  | bool     | Whether the protocol supports multi-file batch sends       |
| `use_pty`     | bool     | Whether the command requires a PTY (pseudo-terminal)       |
| `default`     | bool     | Sets this as the default protocol when user doesn't choose |

### Argument Placeholders

- `{filePath}` — Expanded to one or more file paths (send only). If absent from `send_args`, file paths are appended at the end.
- `{targetDir}` — Expanded to the upload target directory (receive only).
- `{socket}` — Expanded to the raw TCP socket file descriptor (used by sexyz).

## Docker Deployment

The Docker image handles lrzsz automatically:

- **Build stage:** lrzsz is compiled from source (Alpine doesn't package it)
- **Runtime stage:** `lsz` and `lrz` binaries are copied to `/usr/local/bin/` with `sz`/`rz` symlinks

To add sexyz to a Docker deployment, place it in `bin/sexyz` before building and add to the Dockerfile:

```dockerfile
# Copy sexyz binary (if available)
COPY bin/sexyz ./bin/sexyz
RUN chmod +x ./bin/sexyz
```

The existing `bin/sexyz` in the repository is included with the project but is platform-specific. Ensure the binary matches the Docker container's architecture (typically linux/amd64).

## Troubleshooting

### "sz: command not found"

lrzsz is not installed. Install it for your platform (see Dependencies above).

### "sexyz: command not found"

sexyz is not in `PATH` or `bin/sexyz`. Obtain it from Synchronet and place in `bin/sexyz`.

### Transfer hangs or fails over telnet

- Try the sexyz protocol instead of lrzsz — it works better over raw telnet connections
- lrzsz requires PTY passthrough which can conflict with telnet negotiation

### Transfer fails over SSH

- Ensure `use_pty: true` is set for lrzsz-based protocols
- Check that the SSH session supports binary data passthrough

### Permission denied on transfer binaries

```bash
chmod +x bin/sexyz
# or for system-installed lrzsz, check:
which sz rz
```

## Summary Table

| Dependency | Required?   | Package Manager              | Notes                                      |
| ---------- | ----------- | ---------------------------- | ------------------------------------------ |
| `lrzsz`    | Recommended | apt/dnf/brew/pacman          | Default ZModem protocol (`sz`/`rz`)        |
| `sexyz`    | Optional    | N/A (Synchronet builds only) | ZModem 8k for telnet; place in `bin/sexyz` |
