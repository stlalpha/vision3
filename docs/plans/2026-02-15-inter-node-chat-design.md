# VIS-26: Inter-Node Chat Design

## Overview

Teleconference chat room and node paging for inter-node communication.

## Teleconference

Single global chat room accessible via `C` key from main menu.

### UI

Scrolling log — messages scroll up the terminal, input line at the bottom. On entry, show the last N buffered messages for context. Announce joins and departures.

Messages display as: `<Handle> message text`

`/Q` or `/quit` exits chat and returns to the previous menu.

### Architecture

In-memory `ChatRoom` struct with:
- A ring buffer of recent messages (configurable size, e.g. 100)
- A subscriber map of channels — one per user currently in the room
- Mutex protecting both

On `Broadcast(msg)`: append to ring buffer, fan out to all subscriber channels.

Each chat participant runs a goroutine reading from their channel and writing to their terminal, plus the main goroutine reading user input and calling Broadcast.

`ChatRoom` is a field on `MenuExecutor`, initialized at startup (like `SessionRegistry`).

## Paging

Send a one-line message to a specific node. Accessible via `/SE` from the main menu. Note: `!` maps to `RUN:CHAT` (teleconference) and `/SE` maps to `RUN:PAGE` in `MAIN.CFG`.

### Flow

1. Prompt: "Page which node?" — show list of online nodes
2. User selects a node number
3. Prompt: "Message:"
4. Message is queued on the target `BbsSession`

### Delivery

Non-intrusive: pages are queued in a `PendingPages []string` field on `BbsSession`. Displayed when the target user returns to any menu prompt. Format: `|09Page from |15<handle>|09: <message>|07`

## New Files

| File | Purpose |
|---|---|
| `internal/chat/room.go` | `ChatRoom` — Subscribe, Unsubscribe, Broadcast, History |
| `internal/menu/chat.go` | `runChat` teleconference handler |
| `internal/menu/page.go` | `runPage` paging handler |

## Modified Files

| File | Change |
|---|---|
| `internal/session/session.go` | Add `PendingPages []string` and mutex-protected accessors |
| `internal/menu/executor.go` | Add `ChatRoom` field, register `CHAT` and `PAGE` commands |
| `menus/v3/cfg/MAIN.CFG` | Change C/! to `RUN:CHAT`, /SE to `RUN:PAGE` |
| Menu prompt loop | Check and display pending pages before showing prompt |
