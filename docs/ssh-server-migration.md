# SSH Server Migration: golang.org/x/crypto/ssh to libssh

## Overview

This document details the migration from `golang.org/x/crypto/ssh` to `libssh` (via CGO) for ViSiON/3 BBS SSH server implementation.

## Problem Statement

The original SSH server implementation using `golang.org/x/crypto/ssh` could not connect with SyncTerm and other retro BBS terminal software. These legacy terminals require SSH protocol support that was removed from modern golang.org/x/crypto/ssh versions.

### Initial Investigation

- **Attempted Solution**: Downgrade `golang.org/x/crypto/ssh` to December 2020 version
- **Result**: Failed - Legacy algorithm support was incomplete
- **User Decision**: Switch to libssh via CGO for full legacy compatibility

## Solution: libssh Implementation

### Why libssh?

1. **Complete Legacy Support**: libssh natively supports legacy SSH algorithms when needed
2. **Flexible Configuration**: Can enable/disable legacy algorithms via configuration
3. **Battle-Tested**: Mature C library used in production SSH implementations
4. **Modern Algorithms**: Defaults to secure modern algorithms, legacy only when configured

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    ViSiON/3 BBS                         │
│                  (Go Application)                       │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│           internal/sshserver/adapter.go                 │
│        (Adapts libssh to gliderlabs/ssh types)          │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│    internal/sshserver/server.go + callbacks.go          │
│     (libssh CGO wrapper with callback-based API)        │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│                   libssh (C library)                    │
│              System-installed library                   │
└─────────────────────────────────────────────────────────┘
```

## Implementation Details

### Core Components

#### 1. **server.go** - libssh CGO Wrapper & Event Loop

C preamble contains helper functions for callback struct allocation (since `ssh_callbacks_init` is a C macro inaccessible from Go). Go code manages the SSH server lifecycle and per-connection event loop.

Key functions:
```go
NewServer(config Config) (*Server, error)
Listen() error
Accept() error
handleConnection(session C.ssh_session)  // Event loop
flushWrites(cs *connState)               // Drain write channel
```

Key types:
```go
connState   // Per-connection state bridging C callbacks to Go channels
closeSignal // Thread-safe one-shot close notification (sync.Once + chan)
```

#### 2. **callbacks.go** - libssh Callback Functions

Contains `//export` Go functions invoked by libssh during `ssh_event_dopoll()`:

|Callback|Purpose|
|---|---|
|`go_auth_password_cb`|Password authentication|
|`go_auth_none_cb`|None authentication|
|`go_channel_open_cb`|Channel open + set channel callbacks|
|`go_channel_data_cb`|Incoming data → Go read channel|
|`go_channel_pty_request_cb`|PTY request (term, dimensions)|
|`go_channel_shell_request_cb`|Shell request → start session|
|`go_channel_pty_window_change_cb`|Window resize events|
|`go_channel_close_cb` / `go_channel_eof_cb`|Connection close|

#### 3. **adapter.go** - Interface Compatibility Layer
- Adapts libssh Session to `gliderlabs/ssh.Session` interface
- Bridges `sshserver.Window` events to `ssh.Window` via goroutine
- Provides context management
- Maintains compatibility with existing BBS code

Key types:
```go
BBSSessionAdapter - Implements ssh.Session interface
BBSSessionContext - Implements ssh.Context interface
```

#### 4. **main.go** - Integration
- SSH authentication bypass for users authenticated at SSH protocol level
- Terminal type detection (CP437 for SyncTerm)
- Session handler integration

### Callback-Based Event Loop

The SSH connection lifecycle uses libssh's callback API with a single `ssh_event_dopoll()` event loop:

```
1. ssh_set_server_callbacks()     ← Register auth + channel open callbacks
2. ssh_set_auth_methods()         ← Advertise PASSWORD | NONE
3. ssh_handle_key_exchange()      ← Blocking key exchange
4. ssh_event_new() + add_session  ← Create event context
5. ssh_event_dopoll() loop        ← Drives all callbacks
   ├── Auth callbacks fire        → Store username
   ├── Channel open fires         → Create channel, set channel callbacks
   ├── PTY request fires          → Store term/dimensions
   └── Shell request fires        → Signal to start session handler
6. Session handler runs in goroutine
7. Event loop continues           ← Window resize, data, close events
```

**Critical ordering**: Callbacks and auth methods MUST be set BEFORE `ssh_handle_key_exchange()`.

### I/O Architecture

Data flows through Go channels, keeping all libssh calls on a single OS-locked thread:

```text
Client → ssh_event_dopoll → channel_data_cb → readCh → Session.Read() → BBS
Client ← ssh_channel_write ← flushWrites ← writeCh ← Session.Write() ← BBS
```

- `readCh` (buffered 64): Data callback writes, Session.Read() consumes
- `writeCh` (buffered 256): Session.Write() produces, event loop flushes
- `winCh` (buffered 4): Window change callback → adapter bridge → ssh.Window
- `closeSignal`: Thread-safe shutdown coordination (sync.Once + chan)

### CGO Bridge Details

- `cgo.Handle` passes Go `*connState` through C `void* userdata`
- `//export` functions in callbacks.go recover state via `cgo.Handle(uintptr(userdata))`
- C helper functions allocate callback structs and call `ssh_callbacks_init` macro
- `runtime.LockOSThread()` ensures all libssh calls for a session use one OS thread
- `SSH_AUTH_METHOD_*` are `#define` macros with `u` suffix — wrapped in C helper

## Configuration

### config.json
```json
{
  "sshPort": 2222,
  "sshHost": "0.0.0.0",
  "sshEnabled": true,
  "telnetPort": 2323,
  "telnetHost": "0.0.0.0",
  "telnetEnabled": true
}
```

### SSH Keys

**In use:**

- `ssh_host_rsa_key` + `.pub` - RSA 2048-bit key

## Algorithm Support

### Modern Algorithms (Used by Default)
libssh defaults provide secure modern algorithms that work with SyncTerm:

**Ciphers:**
- aes128-ctr, aes192-ctr, aes256-ctr
- chacha20-poly1305@openssh.com

**Key Exchange:**
- ecdh-sha2-nistp256/384/521
- diffie-hellman-group14/16/18-sha256/512
- curve25519-sha256

**MACs:**
- hmac-sha2-256, hmac-sha2-512
- hmac-sha2-256-etm@openssh.com, hmac-sha2-512-etm@openssh.com

## Testing Results

### SyncTerm Compatibility

- **Connection**: Successfully connects with modern algorithms
- **Authentication**: Password authentication works
- **PTY**: Terminal properly initialized (80x24, TERM=syncterm)
- **Display**: CP437 ANSI graphics render correctly
- **Input**: Keyboard input works properly
- **SSH Auto-login**: Users skip LOGIN screen when SSH-authenticated
- **Window Resize**: Detected via callback (previously broken with polling API)

### Modern SSH Clients

- **OpenSSH**: Fully compatible
- **PuTTY**: Fully compatible
- **mRemoteNG**: Fully compatible

## Files

### SSH Server Package (`internal/sshserver/`)

- `server.go` - C preamble helpers, event loop, Session I/O, Server lifecycle
- `callbacks.go` - `//export` callback functions for libssh
- `adapter.go` - `BBSSessionAdapter` (gliderlabs/ssh.Session interface)

### Integration

- `cmd/vision3/main.go` - Session handler, auto-login, terminal detection

## Build Requirements

### System Dependencies
```bash
# Ubuntu/Debian
apt-get install libssh-dev

# Fedora/RHEL
dnf install libssh-devel

# macOS
brew install libssh
```

### Go Build
No special flags needed - CGO picks up libssh via pkg-config:
```bash
cd cmd/vision3
go build -o ../../vision3
```

## Current Limitations

1. **Password Auth**: Currently accepts any password (TODO: validate against BBS user database)
2. **No Rate Limiting**: Should add connection rate limiting
3. **No IP Filtering**: Should add IP whitelist/blacklist support
4. **Remote Address**: Reports 0.0.0.0 (libssh doesn't easily expose client IP)

## Migration History

### Phase 1: golang.org/x/crypto/ssh → libssh polling API

- Replaced Go SSH library with libssh via CGO
- Used `ssh_message_get()` polling loops for auth, channel, and PTY handling
- Fixed race conditions in message polling (single-threaded message access)

### Phase 2: libssh polling API → callback API

- Migrated from deprecated `ssh_message_get()` to callback-based API
- Eliminated ~10 deprecation warnings from libssh 0.11.x
- Fixed window resize bug (polling loop exited after shell request, missing resize events)
- Introduced channel-based I/O for thread-safe data flow
- Added `ssh_event_dopoll()` event loop running for full connection lifetime

## References

- libssh documentation: https://www.libssh.org/
- SyncTerm project: https://syncterm.bbsdev.net/
- SSH Protocol RFC: RFC 4253 (SSH Transport Layer Protocol)
- Go CGO documentation: https://pkg.go.dev/cmd/cgo

---
*Document created: 2026-02-09*
*Last updated: 2026-02-09*
