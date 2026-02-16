# VIS-54 + VIS-51: System Stats & Message Reader Footer

## VIS-54: System Stats Command

### Overview

Template-based system statistics display showing BBS-wide metrics. Registered as `RUN:SYSTEMSTATS`, replacing the current `GOTO:SYSSTATS` on the S key in MAIN.CFG.

### Architecture

Standard V3 template pattern with `SYSSTATS.TOP` / `SYSSTATS.BOT` and inline key-value rendering between them (same approach as View Config in VIS-52). Uses `@TOKEN@` substitution via the existing `lastCallerATTokenRegex` pipeline.

### Data Sources

| Token | Description | Source | Status |
|---|---|---|---|
| `@BBSNAME@` | BBS name | `e.ServerCfg.BoardName` | Exists |
| `@SYSOP@` | SysOp name | `e.ServerCfg.SysOpName` | **New field** |
| `@VERSION@` | Software version | `jam.Version` | Exists |
| `@TOTALUSERS@` | Total user count | `userManager.GetUserCount()` | **New method** |
| `@TOTALCALLS@` | Total calls | `userManager.GetTotalCalls()` | **New method** |
| `@TOTALMSGS@` | Total messages | `e.MessageMgr.GetTotalMessageCount()` | **New method** |
| `@TOTALFILES@` | Total files | `e.FileMgr.GetTotalFileCount()` | **New method** |
| `@ACTIVENODES@` | Active node count | `e.SessionRegistry.ActiveCount()` | **New method** |
| `@MAXNODES@` | Max nodes allowed | `e.ServerCfg.MaxNodes` | Exists |
| `@DATE@` | Current date | `time.Now()` formatted | N/A |
| `@TIME@` | Current time | `time.Now()` formatted | N/A |

### New Methods Required

**`internal/user/manager.go`:**
- `GetUserCount() int` — returns `len(um.users)` (under RLock)
- `GetTotalCalls() uint64` — returns `um.nextCallNumber - 1` (under RLock)

**`internal/message/manager.go`:**
- `GetTotalMessageCount() int` — iterates all areas, sums `GetMessageCountForArea()`

**`internal/file/manager.go`:**
- `GetTotalFileCount() int` — iterates all areas, sums `GetFileCountForArea()`

**`internal/session/registry.go`:**
- `ActiveCount() int` — returns `len(sr.sessions)` (under RLock)

### New Config Field

**`internal/config/config.go`:** Add `SysOpName string` to `ServerConfig` struct with `json:"sysOpName"`.

**`configs/config.json`:** Add `"sysOpName": ""` field.

### Display

Stats rendered inline between TOP/BOT templates, pipe-coded:

```text
 BBS Name:       @BBSNAME@
 SysOp:          @SYSOP@
 Version:        ViSiON/3 v@VERSION@

 Total Users:    @TOTALUSERS@
 Total Calls:    @TOTALCALLS@
 Total Messages: @TOTALMSGS@
 Total Files:    @TOTALFILES@
 Active Nodes:   @ACTIVENODES@ / @MAXNODES@

 Date:           @DATE@
 Time:           @TIME@
```

Followed by pause prompt, then return.

### Menu Integration

- Register `SYSTEMSTATS` in `registerAppRunnables`
- Change MAIN.CFG S key from `GOTO:SYSSTATS` to `RUN:SYSTEMSTATS`
- Implementation in `internal/menu/system_stats.go` (new file)

### Files

| File | Action |
|---|---|
| `internal/config/config.go` | Modify — add `SysOpName` to `ServerConfig` |
| `configs/config.json` | Modify — add `"sysOpName"` field |
| `internal/user/manager.go` | Modify — add `GetUserCount()`, `GetTotalCalls()` |
| `internal/message/manager.go` | Modify — add `GetTotalMessageCount()` |
| `internal/file/manager.go` | Modify — add `GetTotalFileCount()` |
| `internal/session/registry.go` | Modify — add `ActiveCount()` |
| `internal/menu/system_stats.go` | Create — `runSystemStats` handler |
| `internal/menu/executor.go` | Modify — register `SYSTEMSTATS` |
| `menus/v3/templates/SYSSTATS.TOP` | Create — header template |
| `menus/v3/templates/SYSSTATS.BOT` | Create — footer template |
| `menus/v3/cfg/MAIN.CFG` | Modify — S key → `RUN:SYSTEMSTATS` |

---

## VIS-51: Compact Message Reader Footer

### Overview

Reduce the message reader footer from 3 lines to 2 by dropping the horizontal separator and keeping the board info line and lightbar on separate rows.

### Current Layout (3 lines)

```
Row termHeight-2: ────────────────────────────── (separator)
Row termHeight-1: Conference > Area [3/15] [50%]
Row termHeight:   Next Reply Again Skip Thread Post Jump Mail List Quit
```

### New Layout (2 lines)

```
Row termHeight-1: Conference > Area [3/15] [50%] (NewScan)
Row termHeight:   Next Reply Again Skip Thread Post Jump Mail List Quit
```

### Changes

- Remove the horizontal separator line draw at `termHeight-2`
- Move board info line from `termHeight-1` to `termHeight-1` (same row, but now top of footer)
- Lightbar stays at `termHeight`
- Adjust `bodyAvailHeight` calculation to gain 1 extra line for message body display

### Files

| File | Action |
|---|---|
| `internal/menu/message_reader.go` | Modify — remove separator, adjust row positions, update bodyAvailHeight |
