# File Transfer Protocols

This document covers the file transfer protocol system in ViSiON/3, including setup, configuration, and building from source.

## Overview

ViSiON/3 uses **sexyz** (Synchronet's external file transfer program) as its sole file transfer engine. sexyz provides ZModem with 8k blocks and works on both SSH and telnet connections. Protocol definitions are stored in `configs/protocols.json`.

| Protocol  | Engine             | Key | Connection Types | PTY Required |
| --------- | ------------------ | --- | ---------------- | ------------ |
| ZModem 8k | sexyz (Synchronet) | `Z` | SSH + Telnet     | No           |

### Why sexyz?

- **Universal** — Works on both SSH and telnet connections without modification
- **No PTY required** — Operates on raw I/O pipes, avoiding PTY line-discipline corruption
- **8k blocks** — Faster throughput than standard 1k ZModem
- **Battle-tested** — Used by Synchronet BBS and other BBS software for decades

## Dependencies

### sexyz (Required)

sexyz is **not available through standard package managers**. It must be built from source or obtained from Synchronet BBS.

- Website: https://www.synchro.net
- Source: https://gitlab.synchro.net

### Building sexyz from Source

The recommended approach is to build sexyz v3.2 from the Synchronet source repository:

```bash
# Clone the Synchronet source
git clone https://gitlab.synchro.net/main/sbbs.git
cd sbbs

# Build sexyz
cd src/sbbs3
make RELEASE=1 sexyz

# The binary will be at:
# src/sbbs3/*/sexyz (platform-specific subdirectory)
```

### Installing sexyz

```bash
cp sexyz /path/to/vision3/bin/sexyz
chmod +x /path/to/vision3/bin/sexyz
```

Verify it works:

```bash
bin/sexyz -help
```

You should see version and usage information from Synchronet External X/Y/ZMODEM.

### Platform Availability

| Platform     | How to Obtain                           |
| ------------ | --------------------------------------- |
| Linux x86_64 | Build from source (recommended)         |
| Linux ARM64  | Build from source                       |
| macOS        | Build from source                       |
| Windows      | Pre-built .exe from Synchronet builds   |

## Configuration

> **📋 In Development:** A TUI sysop configuration editor is planned. Once available, it will include managing `protocols.json` without hand-editing files. Until then, protocols are configured as described below.

The default protocol configuration in `configs/protocols.json`:

```json
[
  {
    "key": "Z",
    "name": "Zmodem 8k (SEXYZ)",
    "description": "ZModem-8k batch file transfer via Synchronet SEXYZ",
    "send_cmd": "bin/sexyz",
    "send_args": ["-raw", "-8", "sz", "{filePath}"],
    "recv_cmd": "bin/sexyz",
    "recv_args": ["-raw", "-8", "rz", "{targetDir}"],
    "batch_send": true,
    "use_pty": false,
    "default": true,
    "connection_type": ""
  }
]
```

### Configuration Fields

| Field             | Type     | Description                                                |
| ----------------- | -------- | ---------------------------------------------------------- |
| `key`             | string   | Short key shown in protocol selection menu                 |
| `name`            | string   | Display name shown to users                                |
| `description`     | string   | Short description for help/selection menus                 |
| `send_cmd`        | string   | Executable path for sending files (download to user)       |
| `send_args`       | string[] | Arguments for send command                                 |
| `recv_cmd`        | string   | Executable path for receiving files (upload from user)     |
| `recv_args`       | string[] | Arguments for receive command                              |
| `batch_send`      | bool     | Whether the protocol supports multi-file batch sends       |
| `use_pty`         | bool     | Whether the command requires a PTY (pseudo-terminal)       |
| `default`         | bool     | Sets this as the default protocol when user doesn't choose |
| `connection_type` | string   | `""` = any, `"ssh"` = SSH only, `"telnet"` = telnet only  |

### Argument Placeholders

- `{filePath}` — Expanded to one or more file paths (send only). If absent from `send_args`, file paths are appended at the end.
- `{targetDir}` — Expanded to the upload target directory (receive only). A trailing `/` is automatically ensured for sexyz compatibility.
- `{fileListPath}` — Expanded to a temporary file containing one file path per line (for batch sends).

### sexyz Flags

- `-raw` — Disables telnet IAC processing within sexyz. ViSiON/3's telnet layer handles IAC transparently, so sexyz must not double-process it. Harmless on SSH (no IAC exists).
- `-8` — Enables 8k ZModem blocks for faster throughput.
- `sz` / `rz` — Send (download to user) / Receive (upload from user) ZModem mode.

## How It Works

### SSH Connections

SSH provides a fully binary-transparent channel. sexyz's `-raw` flag has no effect (there are no IAC sequences to process). The data flows through `RunCommandDirect` which pipes sexyz's stdin/stdout directly to the SSH session channel.

### Telnet Connections

ViSiON/3's telnet layer (`TelnetConn`) handles IAC stripping/escaping transparently. sexyz sees a clean byte stream via `-raw` mode. The data flows through `RunCommandDirect` with the same pipe-based I/O.

### Execution Flow

1. User selects files and initiates transfer
2. The session's `InputHandler` is reset (`resetSessionIH`) to release the session reader
3. sexyz is launched via `RunCommandDirect` (no PTY)
4. Two goroutines copy data: session→sexyz stdin, sexyz stdout→session
5. When sexyz exits, the stdin goroutine is interrupted via `SetReadInterrupt`
6. Leftover protocol bytes are drained from the session
7. The `InputHandler` is recreated and the BBS resumes normal operation

## Docker Deployment

For Docker deployments, the sexyz binary must be included in the image. Place it at `bin/sexyz` before building:

```dockerfile
# Copy sexyz binary
COPY bin/sexyz ./bin/sexyz
RUN chmod +x ./bin/sexyz
```

Ensure the binary matches the Docker container's architecture (typically linux/amd64 for Alpine-based images).

## Troubleshooting

### "sexyz: command not found" or "no such file"

sexyz is not at `bin/sexyz`. Build it from source or download from Synchronet and place it in the `bin/` directory.

### Transfer hangs or times out

- Ensure your terminal client supports ZModem (SyncTerm, NetRunner, etc.)
- Check that sexyz has execute permissions: `chmod +x bin/sexyz`
- Review logs for sexyz stderr output (logged as `INFO: [sexyz stderr] ...`)

### Permission denied on sexyz binary

```bash
chmod +x bin/sexyz
```

### "No transfer protocols configured"

Ensure `configs/protocols.json` exists and contains valid protocol definitions. The `connection_type` field must match the user's connection (or be `""` for both).

## Adding Custom Protocols

While sexyz is the default and recommended engine, the protocol system supports any external transfer program. To add a custom protocol, add an entry to `configs/protocols.json` following the field definitions above. Set `use_pty: true` if the program requires a pseudo-terminal, and use `connection_type` to restrict it to specific connection types.
