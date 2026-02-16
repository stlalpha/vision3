# VIS-53: Who's Online

## Overview

Display all active BBS connections — authenticated users and unauthenticated sessions — using the standard template pattern (TOP/MID/BOT).

## Architecture

### Session Registry

New `SessionRegistry` in `internal/session/registry.go` — a mutex-protected map of active `BbsSession` pointers keyed by node ID.

### API

- `NewSessionRegistry() *SessionRegistry`
- `Register(session *BbsSession)`
- `Unregister(nodeID int)`
- `ListActive() []*BbsSession` — returns snapshot sorted by node ID
- `Get(nodeID int) *BbsSession` — single session lookup (useful for future chat/paging)

### Lifecycle

`main.go` creates the registry at startup, calls `Register` on SSH connect, `Unregister` on disconnect. Passes registry reference to `MenuExecutor`.

### Idle Tracking

Add `LastActivity time.Time` field to `BbsSession`. Updated on each keypress/input event. The who's online display calculates idle as `time.Since(LastActivity)`.

### Data Fields (Template Placeholders)

| Placeholder | Description | Example |
|---|---|---|
| `@ND@` | Node number | `1` |
| `@UN@` | Handle (or "Logging In..." if unauthenticated) | `Felonius` |
| `@LO@` | Current menu/activity | `File Menu` |
| `@TO@` | Time online (HH:MM) | `01:23` |
| `@ID@` | Idle time (MM:SS) | `02:15` |

### Display

Template-based using `WHOONLN.TOP`, `WHOONLN.MID`, `WHOONLN.BOT`. MID repeats for each connected node. Same rendering pipeline as Last Callers.

### Menu Integration

Registered as `RUN:WHOISONLINE` in executor command registry. Added to `MAIN.CFG` (W key).

## Files

| File | Action |
|---|---|
| `internal/session/registry.go` | Create — SessionRegistry |
| `internal/session/session.go` | Modify — add LastActivity field |
| `internal/menu/executor.go` | Modify — add runWhoIsOnline, accept registry |
| `cmd/vision3/main.go` | Modify — create registry, register/unregister, pass to executor |
| `menus/v3/templates/WHOONLN.TOP` | Create — header template |
| `menus/v3/templates/WHOONLN.MID` | Create — per-node line template |
| `menus/v3/templates/WHOONLN.BOT` | Create — footer template |
| `menus/v3/cfg/MAIN.CFG` | Modify — add W key mapping |
