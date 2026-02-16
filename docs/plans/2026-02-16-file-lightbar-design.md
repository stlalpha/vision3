# Lightbar File Listing — Design Document

**Goal:** Add lightbar (arrow-key navigable highlight bar) to the file listing UI, with a detail pane showing expanded info for the highlighted file.

**Architecture:** The existing `runListFiles` function gains a config check (`FileListingMode` in server.json) that dispatches to either the current template-based flow ("classic") or a new `runListFilesLightbar` function. The lightbar implementation lives in a new file `internal/menu/file_lightbar.go` and reuses existing FILELIST.TOP/MID/BOT templates for rendering, plus existing key-reading utilities from `message_lightbar.go`.

## Config

Add `FileListingMode string` to `ServerConfig` in `internal/config/config.go`:
- JSON key: `"fileListingMode"`
- Values: `"lightbar"` (default) or `"classic"`
- When empty/unset, defaults to `"lightbar"`

## Rendering

- **FILELIST.TOP** — displayed as header (unchanged)
- **FILELIST.MID** — used to format each file row with placeholders (`^MARK`, `^NUM`, `^NAME`, `^DATE`, `^SIZE`, `^DESC`). The selected row is rendered in reverse video (`\x1b[7m`), unselected rows rendered normally.
- **Detail pane** — below the file list rows, shows expanded info for the highlighted file: filename, full description, uploader, upload date, file size, download count
- **FILELIST.BOT** — displayed as footer with `^PAGE`/`^TOTALPAGES` (unchanged)
- **Status line** — bottom row shows available commands

## Navigation

- Arrow Up/Down — move highlight bar
- Page Up/Page Down — jump one page
- Home/End — jump to first/last file
- Auto-scroll when highlight reaches edge of visible page

## Commands (single keystroke, no Enter needed)

- **Space** or **Enter** — toggle mark/tag on highlighted file
- **D** — download all marked files (same ZMODEM flow as current)
- **V** — view highlighted file (archive view or text view)
- **U** — upload to current area
- **Q** or **Esc** — quit back to menu

## Implementation

- New file: `internal/menu/file_lightbar.go`
- Adapts the `adminUserLightbarBrowser` scrollable-list pattern
- Reuses `readKeySequence()` from `message_lightbar.go` for robust key handling
- Cursor hidden during lightbar mode, restored on exit
- All existing file commands (download/upload/view/mark) work identically — only the interaction model changes
