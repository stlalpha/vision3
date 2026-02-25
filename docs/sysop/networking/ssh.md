# SSH Server

ViSiON/3 includes a built-in SSH server implemented in pure Go — no CGO or native libraries required.

## Implementation

The SSH server (`internal/sshserver/`) wraps [`gliderlabs/ssh`](https://github.com/gliderlabs/ssh), which itself wraps `golang.org/x/crypto/ssh`. All sessions are handled via the `gliderlabs/ssh.Session` interface, which the BBS session handler uses for both SSH and telnet connections.

Key features:

- **Pure Go** — no CGO, no system library dependencies
- **Legacy algorithm support** — optional older SSH algorithms for retro BBS clients (SyncTERM, NetRunner)
- **Read interrupt support** — `BBSSession` wraps `ssh.Session` to allow clean door program I/O cancellation
- **Password and keyboard-interactive authentication**
- **Configurable server banner** (`Version` field)

## Configuration

SSH settings live in `configs/config.json`:

```json
{
  "sshPort": 2222,
  "sshHost": "0.0.0.0",
  "sshEnabled": true,
  "legacySSHAlgorithms": false
}
```

### Fields

- `sshPort` — TCP port to listen on (default: `2222`)
- `sshHost` — Interface to bind (default: `"0.0.0.0"` for all interfaces)
- `sshEnabled` — Enable/disable the SSH server
- `legacySSHAlgorithms` — Enable older SSH algorithms for retro client compatibility (see below)

## SSH Host Keys

Host keys are stored in `configs/`:

- `ssh_host_rsa_key` — RSA private key (auto-generated on first run)
- `ssh_host_rsa_key.pub` — RSA public key

Host keys are generated automatically by the Docker entrypoint or the BBS startup if they don't exist.

## Legacy Algorithm Support

When `legacySSHAlgorithms` is `true`, the server advertises a broader set of SSH algorithms needed by retro BBS terminal software:

| Category | Additional algorithms |
| -------- | --------------------- |
| Key exchange | `diffie-hellman-group14-sha1`, `diffie-hellman-group1-sha1` |
| Ciphers | `aes128-cbc`, `aes256-cbc`, `3des-cbc` |
| MACs | `hmac-sha1` |

When `false` (default), only modern secure algorithms are offered.

> **Tip:** Enable `legacySSHAlgorithms` if SyncTERM or other retro clients fail to connect.

## Supported Clients

Tested and working:

- **SyncTERM** — BBS-specific terminal with CP437/ANSI graphics support
- **OpenSSH** (`ssh` command line)
- **PuTTY**
- **mRemoteNG**

## Troubleshooting

### Connection Refused

- Check `sshEnabled: true` in `configs/config.json`
- Verify the port is not in use: `netstat -ln | grep 2222`
- Check that `configs/ssh_host_rsa_key` exists and is readable

### SyncTERM or Retro Client Fails to Connect

- Enable `legacySSHAlgorithms: true` in `configs/config.json` and restart the BBS

### Authentication Fails

- Verify the username and password are correct in the user database
- Check BBS logs for authentication error messages

## Technical Reference

### Package: `internal/sshserver`

- **`server.go`** — `Server` struct wrapping `gliderlabs/ssh.Server`; `BBSSession` wrapping `ssh.Session` with read interrupt support
- **`Config`** — Server configuration (host key path, address, port, legacy algorithms, session/password/keyboard-interactive handlers)
- **`BBSSession.SetReadInterrupt(ch)`** — Registers a channel that cancels a blocked `Read()` with `ErrReadInterrupted`, used by door programs for clean I/O teardown

### Session Interface

Both SSH and telnet sessions implement `gliderlabs/ssh.Session`, allowing the BBS session handler to be protocol-agnostic. SSH sessions are created directly by `gliderlabs/ssh`; telnet sessions use a compatible adapter.
