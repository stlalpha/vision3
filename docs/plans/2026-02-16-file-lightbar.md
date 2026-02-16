# File Lightbar Implementation Plan

**Goal:** Add lightbar navigation to the file listing UI with sysop toggle and detail pane.

**Architecture:** Config-driven dispatch in `runListFiles` sends to either the existing template flow or a new `runListFilesLightbar` in `internal/menu/file_lightbar.go`. The lightbar reuses FILELIST templates and existing key-reading utilities.

**Tech Stack:** Go, SSH terminal I/O, ANSI escape sequences, existing `readKeySequence()` from `message_lightbar.go`

---

### Task 1: Add FileListingMode to ServerConfig

**Files:**
- Modify: `internal/config/config.go` (ServerConfig struct)
- Modify: `configs/config.json` (add field)
- Modify: `templates/configs/config.json` (add field)

**Step 1: Add field to ServerConfig struct**

In `internal/config/config.go`, add to `ServerConfig`:
```go
FileListingMode string `json:"fileListingMode"` // "lightbar" or "classic" (default: "lightbar")
```

**Step 2: Add to both config.json files**

Add `"fileListingMode": "lightbar"` to `configs/config.json` and `templates/configs/config.json`.

**Step 3: Build and verify**

Run: `go build ./...`
Expected: Clean build

**Step 4: Commit**

```bash
git add internal/config/config.go configs/config.json templates/configs/config.json
git commit -m "feat: add FileListingMode setting to server config"
```

---

### Task 2: Add dispatch logic in runListFiles

**Files:**
- Modify: `internal/menu/executor.go` (runListFiles function, ~line 7650)

**Step 1: Add dispatch at the top of the display loop**

After the existing setup code (template loading, pagination setup, initial page fetch — around line 7777 where the `for {` display loop begins), replace the display loop with a mode check:

```go
// Before the existing for { loop, add:
if strings.EqualFold(e.ServerCfg.FileListingMode, "classic") || e.ServerCfg.FileListingMode == "" && false {
    // Fall through to existing template-based loop below
} else {
    // Lightbar mode (default)
    return runListFilesLightbar(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime,
        currentAreaID, currentAreaTag, area,
        processedTopTemplate, processedMidTemplate, processedBotTemplate,
        filesPerPage, totalFiles, totalPages, outputMode)
}
```

The actual approach: wrap the existing `for {` loop body in an `if` check. If mode is not "classic", call `runListFilesLightbar` with all the already-computed values (templates, pagination, area info) and return its result. The existing loop becomes the "classic" path.

**Step 2: Build and verify**

Run: `go build ./...`
Expected: Build failure (runListFilesLightbar not yet defined) — that's expected, will be fixed in Task 3.

**Step 3: Commit**

```bash
git add internal/menu/executor.go
git commit -m "feat: add lightbar/classic dispatch in runListFiles"
```

---

### Task 3: Create file_lightbar.go — core rendering and navigation

**Files:**
- Create: `internal/menu/file_lightbar.go`

This is the main task. Build the lightbar file listing function.

**Step 1: Create the file with the function signature**

```go
package menu

import (
    "bufio"
    "errors"
    "fmt"
    "io"
    "log"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    "github.com/google/uuid"
    "github.com/stlalpha/vision3/internal/ansi"
    "github.com/stlalpha/vision3/internal/file"
    "github.com/stlalpha/vision3/internal/terminalio"
    "github.com/stlalpha/vision3/internal/user"
    "golang.org/x/crypto/ssh"
    "golang.org/x/term"
)

func runListFilesLightbar(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
    userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time,
    currentAreaID int, currentAreaTag string, area *file.FileArea,
    processedTopTemplate []byte, processedMidTemplate string, processedBotTemplate []byte,
    filesPerPage int, totalFiles int, totalPages int,
    outputMode ansi.OutputMode) (*user.User, string, error) {
```

**Step 2: Implement the core rendering**

The function should:

1. Hide cursor on entry, show on exit (`\x1b[?25l` / `\x1b[?25h`)
2. Create a `bufio.Reader` from the SSH session for single-key reads
3. Track state: `selectedIndex` (0-based within all files), `topIndex` (first visible file index), `currentPage` (derived from topIndex)
4. Get terminal dimensions from PTY
5. Calculate detail pane height (6 lines for: filename, description, uploader, date, size, downloads)
6. Calculate visible file rows = terminal height - header lines - footer lines - detail pane lines - status line

**Render function** (called each loop iteration):
1. Clear screen
2. Write FILELIST.TOP template
3. For each visible file (topIndex to topIndex+visibleRows):
   - Fill FILELIST.MID template with file data (same placeholder logic as existing code)
   - If this is the selected file, wrap in `\x1b[7m` (reverse video) ... `\x1b[0m`
   - Write the line
4. Pad remaining rows if fewer files than visible slots
5. Write separator line
6. Write detail pane for selected file (filename, description, uploader, date, size, download count)
7. Write FILELIST.BOT template with `^PAGE`/`^TOTALPAGES`
8. Write status line: `|08[|15↑↓|08] Navigate  [|15Space|08] Mark  [|15V|08]iew  [|15D|08]ownload  [|15U|08]pload  [|15Q|08]uit`

**Step 3: Implement navigation loop**

Main loop reads keys via `readKeySequence()` and handles:
- `\x1b[A` (Up arrow): move selectedIndex up, scroll if needed
- `\x1b[B` (Down arrow): move selectedIndex down, scroll if needed
- `\x1b[5~` (Page Up): move up by visibleRows
- `\x1b[6~` (Page Down): move down by visibleRows
- `\x1b[H` (Home): jump to first file
- `\x1b[F` (End): jump to last file
- `\x1b` (bare Esc): quit
- `q`/`Q`: quit
- `' '` (Space) or `\r` (Enter): toggle mark on selected file
- `d`/`D`: download marked files (call existing download logic)
- `v`/`V`: view selected file (call existing view logic)
- `u`/`U`: upload (call existing upload logic)

**Step 4: Implement file fetching**

When the selected file scrolls past the current page boundary, fetch the appropriate page using `e.FileMgr.GetFilesForAreaPaginated()`. Keep a local cache of fetched pages or fetch all files for the area upfront (simpler, and file counts are typically small).

Simpler approach: fetch ALL files for the area once using `e.FileMgr.GetFilesForArea(currentAreaID)` and index into the slice directly. This avoids pagination complexity in the lightbar — the pagination is purely visual (which files are visible on screen).

**Step 5: Implement mark/tag toggle**

Same logic as existing code in the `default` case of runListFiles — toggle the file's UUID in `currentUser.TaggedFileIDs`.

**Step 6: Implement download/view/upload handlers**

These delegate to the same code paths as the existing runListFiles:
- Download: same ZMODEM flow (check marked files, confirm, execute sz)
- View: call `ziplab.RunZipLabView` or `viewFileByRecord`
- Upload: call `e.runUploadFiles`

After upload, refresh the file list. After download, clear tags and refresh.

**Step 7: Build and verify**

Run: `go build ./...`
Expected: Clean build

**Step 8: Commit**

```bash
git add internal/menu/file_lightbar.go
git commit -m "feat: add lightbar file listing with detail pane"
```

---

### Task 4: Integration testing and polish

**Files:**
- Modify: `internal/menu/file_lightbar.go` (if fixes needed)
- Modify: `internal/menu/executor.go` (if dispatch needs adjustment)

**Step 1: Manual testing checklist**

Connect to the BBS and test:
- [ ] File listing shows lightbar with highlight bar on first file
- [ ] Arrow up/down moves highlight
- [ ] Page up/down scrolls pages
- [ ] Detail pane updates as highlight moves
- [ ] Space toggles mark (asterisk appears/disappears)
- [ ] D downloads marked files via ZMODEM
- [ ] V views the highlighted file
- [ ] U opens upload flow
- [ ] Q quits back to menu
- [ ] Esc quits back to menu
- [ ] Empty file area shows "No files" message
- [ ] Setting `fileListingMode: "classic"` falls back to old behavior

**Step 2: Fix any issues found**

**Step 3: Build and verify**

Run: `go build ./...`
Expected: Clean build

**Step 4: Commit**

```bash
git add -A
git commit -m "fix: polish lightbar file listing edge cases"
```
