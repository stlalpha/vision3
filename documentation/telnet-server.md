# Telnet Server Implementation

## Overview

ViSiON/3 includes a native telnet server alongside the SSH server. Both protocols share the same session handler and BBS logic — the telnet server adapts raw TCP connections to the `gliderlabs/ssh.Session` interface, allowing the entire BBS to work identically over both protocols.

## Architecture

```text
┌─────────────────────────────────────────────────────────┐
│                    ViSiON/3 BBS                         │
│                  (Go Application)                       │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│          internal/telnetserver/adapter.go               │
│     (Adapts TelnetConn to gliderlabs/ssh.Session)       │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│           internal/telnetserver/telnet.go               │
│     (IAC state machine, NAWS, terminal detection)       │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│                  net.Conn (TCP)                         │
│               Raw TCP connection                        │
└─────────────────────────────────────────────────────────┘
```

## Implementation Details

### Core Components

#### 1. **telnet.go** — Telnet Protocol Handler

Handles all telnet protocol concerns: IAC command parsing, option negotiation, NAWS window size detection, and ANSI cursor position reporting.

Key types:

```go
TelnetConn  // Wraps net.Conn with IAC-transparent Read/Write
telnetState // IAC state machine state (9 states)
```

Key functions:

```go
NewTelnetConn(conn net.Conn) *TelnetConn
Negotiate() error                                   // Send option negotiations
DetectTerminalSize() (width, height int, method string)
Read(p []byte) (int, error)                         // IAC-filtered read
Write(p []byte) (int, error)                        // 0xFF-escaped write
```

#### 2. **adapter.go** — Interface Compatibility Layer

Adapts `TelnetConn` to the `gliderlabs/ssh.Session` interface so the existing `sessionHandler()` works unchanged for telnet connections.

Key types:

```go
TelnetSessionAdapter  // Implements ssh.Session interface
TelnetSessionContext  // Implements context.Context + ssh.Context
```

#### 3. **server.go** — Server Lifecycle

Manages TCP listener, connection acceptance, and per-connection goroutines.

Key types:

```go
Server         // TCP listener and connection dispatcher
Config         // Port, Host, SessionHandler
SessionHandler // func(*TelnetSessionAdapter)
```

### Telnet Protocol Handling

#### Option Negotiation

On connection, the server sends the following telnet option negotiations:

| Sequence            | Meaning                                 |
| ------------------- | --------------------------------------- |
| `IAC WILL ECHO`     | Server will echo input (character mode) |
| `IAC WILL SGA`      | Server suppresses go-ahead              |
| `IAC DO SGA`        | Client should suppress go-ahead         |
| `IAC DONT LINEMODE` | Disable line mode (character-at-a-time) |
| `IAC DO NAWS`       | Request window size from client         |

After sending negotiations, the server waits 500ms to drain client responses before proceeding.

#### IAC State Machine

All reads pass through a 9-state finite state machine that strips telnet commands transparently:

```text
stateData → stateIAC → stateWill/stateWont/stateDo/stateDont → stateData
          → stateIAC → stateSB → stateSBData → stateSBIAC → stateData
```

| State                    | Description                                              |
| ------------------------ | -------------------------------------------------------- |
| `stateData`              | Normal data — passes bytes through to caller             |
| `stateIAC`               | Received IAC (0xFF) — next byte determines command       |
| `stateWill/Wont/Do/Dont` | Option negotiation — consumes option byte                |
| `stateSB`                | Subnegotiation begin — captures option byte              |
| `stateSBData`            | Accumulating subnegotiation data                         |
| `stateSBIAC`             | IAC inside subnegotiation — SE ends it, IAC escapes 0xFF |

The state machine persists across `Read()` calls, correctly handling IAC sequences that span read boundaries.

#### 0xFF Byte Escaping

Since 0xFF is the IAC control byte in the telnet protocol:

- **Read**: `IAC IAC` (0xFF 0xFF) is unescaped to a single `0xFF` byte
- **Write**: Any `0xFF` in output data is escaped to `IAC IAC`
- Write uses a fast path (direct write) when no 0xFF bytes are present

#### NAWS (Negotiate About Window Size)

When the client supports NAWS (option 31), it sends terminal dimensions via subnegotiation:

```text
IAC SB NAWS [width_high] [width_low] [height_high] [height_low] IAC SE
```

- Width and height are big-endian 16-bit values
- Dimensions are validated (must be 1-255) and capped to 80x25 for BBS compatibility
- Updates are forwarded to the session adapter via a buffered channel (non-blocking)
- Clients may send NAWS updates at any time during the session (e.g., on terminal resize)

### Terminal Size Detection

Terminal size detection uses a three-tier strategy:

#### 1. ANSI Cursor Position Report (CPR) — Primary

The most reliable method because it detects the actual usable terminal area, accounting for status bars (e.g., SyncTerm's status row steals one row from the NAWS-reported size).

```text
1. Save cursor position:     ESC[s
2. Move to far corner:       ESC[999;999H  (clamped by terminal)
3. Query cursor position:    ESC[6n
4. Parse response:           ESC[row;colR
5. Restore cursor position:  ESC[u
```

- Waits up to 3 seconds for CPR response
- Response parsed with regex: `\033\[(\d+);(\d+)R`
- Results capped to 80x25 (minimum 20x10)

#### 2. NAWS — Fallback

Uses dimensions from the telnet NAWS negotiation (already populated during `Negotiate()`).

#### 3. Defaults — Final Fallback

Hard defaults of 80 columns by 25 rows if neither CPR nor NAWS provides valid dimensions.

### Connection Lifecycle

```text
1. TCP Accept          → net.Conn received
2. NewTelnetConn()     → Wrap with IAC-aware reader/writer
3. Negotiate()         → Send WILL/DO/DONT options, drain responses (500ms)
4. DetectTerminalSize()→ CPR (3s timeout) → NAWS fallback → 80x25 defaults
5. NewTelnetSessionAdapter() → Create ssh.Session-compatible adapter
6. SessionHandler()    → Same handler as SSH sessions
7. Close               → TCP connection closed on session end
```

### Session Adapter Interface

The `TelnetSessionAdapter` implements every method of `gliderlabs/ssh.Session`:

| Method                                   | Telnet Behavior                                                   |
| ---------------------------------------- | ----------------------------------------------------------------- |
| `Read()` / `Write()`                     | Delegates to `TelnetConn` (IAC-filtered/escaped)                  |
| `Close()`                                | Cancels context, closes TCP connection                            |
| `Pty()`                                  | Returns `Term="ansi"`, NAWS/CPR dimensions, window change channel |
| `User()`                                 | Returns `""` (forces manual login flow)                           |
| `RemoteAddr()` / `LocalAddr()`           | From underlying TCP connection                                    |
| `Context()`                              | Returns `TelnetSessionContext`                                    |
| `SessionID()`                            | Format: `telnet-{nanotime}-{counter}`                             |
| `Stderr()`                               | Returns self (same stream as stdout for BBS)                      |
| `Environ()` / `Command()`                | Empty (shell session)                                             |
| `PublicKey()` / `Permissions()`          | `nil` / empty (no SSH auth)                                       |
| `SendRequest()`                          | Returns error (not supported)                                     |
| `CloseWrite()` / `Signals()` / `Break()` | No-op                                                             |

### SSH vs Telnet Authentication

A key difference between the two protocols:

- **SSH**: User authenticates at the protocol level → `User()` returns the authenticated username → `sessionHandler` skips the LOGIN menu
- **Telnet**: No protocol-level authentication → `User()` returns `""` → `sessionHandler` presents the LOGIN menu for manual authentication

## Configuration

### config.json

```json
{
  "telnetPort": 2323,
  "telnetHost": "0.0.0.0",
  "telnetEnabled": true
}
```

| Field           | Type   | Default   | Description                         |
| --------------- | ------ | --------- | ----------------------------------- |
| `telnetPort`    | int    | 2323      | TCP port for telnet connections     |
| `telnetHost`    | string | `0.0.0.0` | Bind address for telnet listener    |
| `telnetEnabled` | bool   | true      | Enable or disable the telnet server |

### Command-Line Testing

```bash
# Connect with system telnet
telnet localhost 2323

# Connect with SyncTerm
# Set connection type to "Telnet" and port to 2323

# Connect with netcat (raw, no NAWS support)
nc localhost 2323
```

## Supported Telnet Options

| Option   | Code | Direction     | Purpose                              |
| -------- | ---- | ------------- | ------------------------------------ |
| ECHO     | 1    | `WILL`        | Server echoes input (character mode) |
| SGA      | 3    | `WILL` + `DO` | Suppress go-ahead (full-duplex)      |
| LINEMODE | 34   | `DONT`        | Disable line buffering               |
| NAWS     | 31   | `DO`          | Request terminal dimensions          |

## Terminal Compatibility

### Tested Clients

- **SyncTerm**: Full compatibility (NAWS + CPR detection handles status bar)
- **PuTTY (Telnet mode)**: Full compatibility
- **Linux/macOS telnet**: Full compatibility
- **netcat**: Works but no NAWS support (defaults to 80x25)

## Files

### Telnet Server Package (`internal/telnetserver/`)

- `telnet.go` — TelnetConn, IAC state machine, NAWS, CPR terminal detection
- `adapter.go` — TelnetSessionAdapter + TelnetSessionContext (ssh.Session interface)
- `server.go` — Server lifecycle, TCP listener, connection handling

### Integration

- `cmd/vision3/main.go` — Session handler, telnet server startup, login flow

## Current Limitations

1. **No Encryption**: Telnet transmits data in cleartext (use SSH for secure access)
2. **No Rate Limiting**: Should add connection rate limiting
3. **No IP Filtering**: Should add IP whitelist/blacklist support
4. **80x25 Cap**: Terminal dimensions are capped to 80x25 for BBS compatibility
5. **No TTYPE**: Terminal type negotiation (RFC 1091) is not implemented — always reports "ansi"

## References

- Telnet Protocol: RFC 854 (Telnet Protocol Specification)
- NAWS: RFC 1073 (Telnet Window Size Option)
- Linemode: RFC 1184 (Telnet Linemode Option)
- SyncTerm: <https://syncterm.bbsdev.net/>
- gliderlabs/ssh: <https://github.com/gliderlabs/ssh>

---
Document created: 2026-02-09
