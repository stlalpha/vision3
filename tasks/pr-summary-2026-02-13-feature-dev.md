# PR Summary: feature/dev -> main (2026-02-13)

## Scope
This PR bundles all work currently on `feature/dev` that is not yet in `main`, including menu/input UX fixes, login/menu flow reliability improvements, CP437/terminal rendering hardening, hot-reload/config improvements, and related documentation updates.

## Highlights

### Menu and Input UX
- Standardized handling for undefined keys across menu flows.
- Added canonical unknown-command feedback with short display duration and prompt refresh behavior.
- Added robust logoff behavior split:
  - `G` = confirm with Yes/No
  - `/G` = immediate logoff
- Added `GOODBYE.ANS` display before disconnect on logoff runnables.
- Applied explicit `G`/`/G` mappings in key menus (`MAIN`, `MSGMENU`, `DOORSM`, `FASTLOGN`).
- Fixed FASTLOGN invalid key handling to avoid repeated unknown-command spam and redraw cleanly.
- Hardened matrix/lightbar loops against control-byte and escape-sequence noise.
- Removed positional numeric command matching so numeric input only works when explicitly defined in command keys.

### Login/Session and Terminal Reliability
- FASTLOGIN and login sequence refinements.
- CP437 rendering and mixed-byte artifact fixes in terminal output paths.
- PTY and door execution stability improvements.

### Config, Runtime, and Integration
- Configuration hot-reload improvements and watcher updates.
- Door and event execution enhancements.
- FTN and related config/schema updates.

### Docs and Project Updates
- Extensive updates under `documentation/` and web docs under `docs/`.
- Task changelog updates in `tasks/tasks.md`.

## Notable Follow-up
- `BBSLISTM` is referenced by menu config but corresponding `.MNU` is currently missing, so navigating to that target still errors until mapped or added.

## Validation performed
- Rebuilt project successfully with:
  - `go build ./cmd/vision3`

## Commit Range
- Compare: `origin/main...feature/dev`
- Head commit at time of summary: `1a96f5d`
