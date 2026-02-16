# VIS-54 + VIS-51: System Stats & Message Reader Footer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a template-based system statistics display command and compact the message reader footer from 3 lines to 2.

**Architecture:** VIS-54 uses the standard V3 template pattern (TOP/BOT files with inline key-value rendering) and `@TOKEN@` substitution via `lastCallerATTokenRegex`. Five new thin accessor methods expose existing data from user, message, file, and session managers. VIS-51 removes the horizontal separator line from the message reader footer, gaining one extra line for message body display.

**Tech Stack:** Go, existing `lastCallerATTokenRegex` pipeline, `ansi.ReplacePipeCodes`, template files

---

## Task 1: Add `SysOpName` to ServerConfig

**Files:**
- Modify: `internal/config/config.go:434-455`
- Modify: `configs/config.json`
- Test: `internal/config/config_test.go`

**Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestServerConfig_SysOpName(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := `{"boardName":"Test BBS","sysOpName":"The SysOp","sysOpLevel":255,"maxNodes":10}`
	os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(cfg), 0644)

	var sc ServerConfig
	data, err := os.ReadFile(filepath.Join(tmpDir, "config.json"))
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if err := json.Unmarshal(data, &sc); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if sc.SysOpName != "The SysOp" {
		t.Errorf("expected SysOpName 'The SysOp', got %q", sc.SysOpName)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestServerConfig_SysOpName -v`
Expected: FAIL — `sc.SysOpName` field does not exist.

**Step 3: Write minimal implementation**

In `internal/config/config.go`, add `SysOpName` to the `ServerConfig` struct (after `BoardPhoneNumber`):

```go
type ServerConfig struct {
	BoardName           string `json:"boardName"`
	BoardPhoneNumber    string `json:"boardPhoneNumber"`
	SysOpName           string `json:"sysOpName"`
	Timezone            string `json:"timezone,omitempty"`
	// ... rest unchanged
}
```

In `configs/config.json`, add the field after `boardPhoneNumber`:

```json
"sysOpName": "",
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestServerConfig_SysOpName -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go configs/config.json
git commit -m "feat: add SysOpName field to ServerConfig"
```

---

## Task 2: Add `GetUserCount()` and `GetTotalCalls()` to UserMgr

**Files:**
- Modify: `internal/user/manager.go`
- Create: `internal/user/manager_stats_test.go`

**Step 1: Write the failing test**

Create `internal/user/manager_stats_test.go`:

```go
package user

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestUserManager(t *testing.T) *UserMgr {
	t.Helper()
	tmpDir := t.TempDir()

	users := []User{
		{ID: 1, Username: "sysop", Handle: "SysOp", AccessLevel: 255},
		{ID: 2, Username: "user1", Handle: "User1", AccessLevel: 10},
		{ID: 3, Username: "user2", Handle: "User2", AccessLevel: 10},
	}
	data, _ := json.Marshal(users)
	os.WriteFile(filepath.Join(tmpDir, "users.json"), data, 0644)

	// Write callnumber.json so nextCallNumber loads
	callNum, _ := json.Marshal(uint64(42))
	os.WriteFile(filepath.Join(tmpDir, "callnumber.json"), callNum, 0644)

	um, err := NewUserManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create user manager: %v", err)
	}
	return um
}

func TestGetUserCount(t *testing.T) {
	um := setupTestUserManager(t)
	count := um.GetUserCount()
	if count != 3 {
		t.Errorf("expected 3 users, got %d", count)
	}
}

func TestGetTotalCalls(t *testing.T) {
	um := setupTestUserManager(t)
	calls := um.GetTotalCalls()
	// nextCallNumber is 42, so total calls = 42 - 1 = 41
	if calls != 41 {
		t.Errorf("expected 41 total calls, got %d", calls)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/user/ -run "TestGetUserCount|TestGetTotalCalls" -v`
Expected: FAIL — methods do not exist.

**Step 3: Write minimal implementation**

Add to `internal/user/manager.go` (after existing public methods):

```go
// GetUserCount returns the total number of registered users.
func (um *UserMgr) GetUserCount() int {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return len(um.users)
}

// GetTotalCalls returns the total number of calls (logins) recorded.
func (um *UserMgr) GetTotalCalls() uint64 {
	um.mu.RLock()
	defer um.mu.RUnlock()
	if um.nextCallNumber <= 1 {
		return 0
	}
	return um.nextCallNumber - 1
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/user/ -run "TestGetUserCount|TestGetTotalCalls" -v`
Expected: PASS

**Step 5: Run full user package tests**

Run: `go test ./internal/user/ -v`
Expected: All tests PASS (no regressions).

**Step 6: Commit**

```bash
git add internal/user/manager.go internal/user/manager_stats_test.go
git commit -m "feat: add GetUserCount and GetTotalCalls to UserMgr"
```

---

## Task 3: Add `GetTotalMessageCount()` to MessageManager

**Files:**
- Modify: `internal/message/manager.go`
- Create: `internal/message/manager_stats_test.go`

**Context:** `MessageManager` has `ListAreas()` (returns `[]*MessageArea`) and `GetMessageCountForArea(areaID int) (int, error)`. The new method iterates all areas and sums counts.

**Step 1: Write the failing test**

Create `internal/message/manager_stats_test.go`. Since `MessageManager` requires JAM bases on disk, use a simple integration-style approach — create a manager, call the method, and verify it returns >= 0 (the method should exist and not panic). If there are no areas, it returns 0.

```go
package message

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetTotalMessageCount_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a minimal areas.json with no areas
	os.WriteFile(filepath.Join(tmpDir, "msgareas.json"), []byte("[]"), 0644)

	mm, err := NewMessageManager(tmpDir, nil)
	if err != nil {
		t.Fatalf("failed to create message manager: %v", err)
	}

	count := mm.GetTotalMessageCount()
	if count != 0 {
		t.Errorf("expected 0 messages for empty areas, got %d", count)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/message/ -run TestGetTotalMessageCount_Empty -v`
Expected: FAIL — method does not exist.

**Step 3: Write minimal implementation**

Add to `internal/message/manager.go`:

```go
// GetTotalMessageCount returns the total number of messages across all areas.
func (mm *MessageManager) GetTotalMessageCount() int {
	areas := mm.ListAreas()
	total := 0
	for _, area := range areas {
		count, err := mm.GetMessageCountForArea(area.ID)
		if err != nil {
			continue
		}
		total += count
	}
	return total
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/message/ -run TestGetTotalMessageCount_Empty -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/message/manager.go internal/message/manager_stats_test.go
git commit -m "feat: add GetTotalMessageCount to MessageManager"
```

---

## Task 4: Add `GetTotalFileCount()` to FileManager

**Files:**
- Modify: `internal/file/manager.go`
- Modify: `internal/file/manager_test.go`

**Context:** `FileManager` has `ListAreas()` (returns `[]FileArea`) and `GetFileCountForArea(areaID int) (int, error)`. Same pattern as Task 3.

**Step 1: Write the failing test**

Add to `internal/file/manager_test.go`:

```go
func TestGetTotalFileCount_Empty(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
		{ID: 2, Tag: "GAMES", Name: "Games", Path: "games"},
	}
	fm := setupTestFileManager(t, areas)

	count := fm.GetTotalFileCount()
	if count != 0 {
		t.Errorf("expected 0 files for empty areas, got %d", count)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/file/ -run TestGetTotalFileCount_Empty -v`
Expected: FAIL — method does not exist.

**Step 3: Write minimal implementation**

Add to `internal/file/manager.go`:

```go
// GetTotalFileCount returns the total number of files across all areas.
func (fm *FileManager) GetTotalFileCount() int {
	areas := fm.ListAreas()
	total := 0
	for _, area := range areas {
		count, err := fm.GetFileCountForArea(area.ID)
		if err != nil {
			continue
		}
		total += count
	}
	return total
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/file/ -run TestGetTotalFileCount_Empty -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/file/manager.go internal/file/manager_test.go
git commit -m "feat: add GetTotalFileCount to FileManager"
```

---

## Task 5: Add `ActiveCount()` to SessionRegistry

**Files:**
- Modify: `internal/session/registry.go`
- Modify: `internal/session/registry_test.go`

**Step 1: Write the failing test**

Add to `internal/session/registry_test.go`:

```go
func TestRegistryActiveCount(t *testing.T) {
	r := NewSessionRegistry()

	if r.ActiveCount() != 0 {
		t.Errorf("expected 0 for empty registry, got %d", r.ActiveCount())
	}

	r.Register(&BbsSession{NodeID: 1, StartTime: time.Now()})
	r.Register(&BbsSession{NodeID: 2, StartTime: time.Now()})

	if r.ActiveCount() != 2 {
		t.Errorf("expected 2 active sessions, got %d", r.ActiveCount())
	}

	r.Unregister(1)
	if r.ActiveCount() != 1 {
		t.Errorf("expected 1 after unregister, got %d", r.ActiveCount())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/session/ -run TestRegistryActiveCount -v`
Expected: FAIL — method does not exist.

**Step 3: Write minimal implementation**

Add to `internal/session/registry.go`:

```go
// ActiveCount returns the number of currently active sessions.
func (r *SessionRegistry) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/session/ -run TestRegistryActiveCount -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/session/registry.go internal/session/registry_test.go
git commit -m "feat: add ActiveCount to SessionRegistry"
```

---

## Task 6: Create System Stats handler and templates

**Files:**
- Create: `internal/menu/system_stats.go`
- Modify: `internal/menu/executor.go:300-340` (register `SYSTEMSTATS`)
- Modify: `menus/v3/cfg/MAIN.CFG:105-108` (S key → `RUN:SYSTEMSTATS`)
- Create: `menus/v3/templates/SYSSTATS.TOP`
- Create: `menus/v3/templates/SYSSTATS.BOT`

**Context:** Follow the same template pattern as `runCfgViewConfig` in `internal/menu/user_config.go:393+`. The handler loads TOP/BOT template files, builds stats lines between them with `@TOKEN@` substitution using `lastCallerATTokenRegex`, and finishes with a pause prompt. The `RunnableFunc` signature is:
```go
func(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error)
```

**Step 1: Create template files**

Create `menus/v3/templates/SYSSTATS.TOP`:
```text
|12System Statistics|07
|08────────────────────────────────────────────────────────────────────────────────
```

Create `menus/v3/templates/SYSSTATS.BOT`:
```text
|08────────────────────────────────────────────────────────────────────────────────
```

**Step 2: Create the handler**

Create `internal/menu/system_stats.go`:

```go
package menu

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	term "golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/jam"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
)

func runSystemStats(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	topPath := filepath.Join(e.MenuSetPath, "templates", "SYSSTATS.TOP")
	botPath := filepath.Join(e.MenuSetPath, "templates", "SYSSTATS.BOT")

	topBytes, err := os.ReadFile(topPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("WARN: Node %d: Failed to read %s: %v", nodeNumber, topPath, err)
	}
	botBytes, err := os.ReadFile(botPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("WARN: Node %d: Failed to read %s: %v", nodeNumber, botPath, err)
	}

	topBytes = stripSauceMetadata(topBytes)
	botBytes = stripSauceMetadata(botBytes)
	topBytes = normalizePipeCodeDelimiters(topBytes)
	botBytes = normalizePipeCodeDelimiters(botBytes)

	// Build token map
	now := time.Now()
	tokens := map[string]string{
		"BBSNAME":     e.ServerCfg.BoardName,
		"SYSOP":       e.ServerCfg.SysOpName,
		"VERSION":     jam.Version,
		"TOTALUSERS":  strconv.Itoa(userManager.GetUserCount()),
		"TOTALCALLS":  strconv.FormatUint(userManager.GetTotalCalls(), 10),
		"TOTALMSGS":   strconv.Itoa(e.MessageMgr.GetTotalMessageCount()),
		"TOTALFILES":  strconv.Itoa(e.FileMgr.GetTotalFileCount()),
		"ACTIVENODES": strconv.Itoa(e.SessionRegistry.ActiveCount()),
		"MAXNODES":    strconv.Itoa(e.ServerCfg.MaxNodes),
		"DATE":        now.Format("01/02/2006"),
		"TIME":        now.Format("03:04 PM"),
	}

	// Build the stats lines
	lines := []string{
		fmt.Sprintf(" |07BBS Name:       |15%s", tokens["BBSNAME"]),
		fmt.Sprintf(" |07SysOp:          |15%s", tokens["SYSOP"]),
		fmt.Sprintf(" |07Version:        |15ViSiON/3 v%s", tokens["VERSION"]),
		"",
		fmt.Sprintf(" |07Total Users:    |15%s", tokens["TOTALUSERS"]),
		fmt.Sprintf(" |07Total Calls:    |15%s", tokens["TOTALCALLS"]),
		fmt.Sprintf(" |07Total Messages: |15%s", tokens["TOTALMSGS"]),
		fmt.Sprintf(" |07Total Files:    |15%s", tokens["TOTALFILES"]),
		fmt.Sprintf(" |07Active Nodes:   |15%s |07/ |15%s", tokens["ACTIVENODES"], tokens["MAXNODES"]),
		"",
		fmt.Sprintf(" |07Date:           |15%s", tokens["DATE"]),
		fmt.Sprintf(" |07Time:           |15%s", tokens["TIME"]),
	}

	var buf bytes.Buffer
	buf.Write([]byte(ansi.ClearScreen()))

	if len(topBytes) > 0 {
		buf.Write(ansi.ReplacePipeCodes(topBytes))
		buf.WriteString("\r\n")
	}

	for _, line := range lines {
		buf.Write(ansi.ReplacePipeCodes([]byte(line)))
		buf.WriteString("\r\n")
	}

	if len(botBytes) > 0 {
		buf.Write(ansi.ReplacePipeCodes(botBytes))
		buf.WriteString("\r\n")
	}

	terminalio.WriteProcessedBytes(terminal, buf.Bytes(), outputMode)

	// Also render any @TOKEN@ patterns in the template files themselves
	// (templates may contain @BBSNAME@ etc.)
	// Note: The inline stats above use direct string formatting, so @TOKEN@
	// substitution is only needed if templates contain tokens.

	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... "
	}
	if err := writeCenteredPausePrompt(s, terminal, pausePrompt, outputMode); err != nil {
		return nil, "", err
	}

	return nil, "", nil
}
```

**Step 3: Register SYSTEMSTATS in executor.go**

In `internal/menu/executor.go`, inside `registerAppRunnables` (around line 300-340), add:

```go
registry["SYSTEMSTATS"] = runSystemStats
```

**Step 4: Update MAIN.CFG**

In `menus/v3/cfg/MAIN.CFG`, change the S key entry from:

```json
{
    "KEYS": "S",
    "CMD": "GOTO:SYSSTATS",
    "ACS": "*",
    "HIDDEN": false
}
```

to:

```json
{
    "KEYS": "S",
    "CMD": "RUN:SYSTEMSTATS",
    "ACS": "*",
    "HIDDEN": false
}
```

**Step 5: Build and verify**

Run: `go build ./...`
Expected: PASS — no compile errors.

**Step 6: Commit**

```bash
git add internal/menu/system_stats.go internal/menu/executor.go menus/v3/cfg/MAIN.CFG menus/v3/templates/SYSSTATS.TOP menus/v3/templates/SYSSTATS.BOT
git commit -m "feat: add system stats display command (VIS-54)"
```

---

## Task 7: Compact message reader footer from 3 lines to 2 (VIS-51)

**Files:**
- Modify: `internal/menu/message_reader.go:185,253-256`

**Context:** The message reader footer currently occupies 3 rows:
- Row `termHeight-2`: horizontal separator (`|08` + CP437 char 196 repeated)
- Row `termHeight-1`: board info line (`confName > areaName [n/total] [scroll%]`)
- Row `termHeight`: lightbar via `drawMsgLightbarStatic`

The change removes the separator line and adjusts `bodyAvailHeight` from subtracting 3 (`barLines := 3`) to subtracting 2 (`barLines := 2`). The board info and lightbar row positions stay at `termHeight-1` and `termHeight` respectively.

**Step 1: Remove the separator line draw**

In `internal/menu/message_reader.go`, find and remove these 3 lines (around line 253-256):

```go
// Draw horizontal line above footer (CP437 character 196)
terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(termHeight-2, 1)), outputMode)
horizontalLine := "|08" + strings.Repeat("\xC4", termWidth-1) + "|07" // CP437 horizontal line character
terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(horizontalLine)), outputMode)
```

**Step 2: Change barLines from 3 to 2**

In `internal/menu/message_reader.go`, around line 185, change:

```go
barLines := 3                    // Horizontal line + board info line + lightbar
```

to:

```go
barLines := 2                    // Board info line + lightbar
```

**Step 3: Build and verify**

Run: `go build ./...`
Expected: PASS — no compile errors.

**Step 4: Commit**

```bash
git add internal/menu/message_reader.go
git commit -m "feat: compact message reader footer to 2 lines (VIS-51)"
```

---

## Task 8: Final verification

**Step 1: Run all tests**

Run: `go test ./...`
Expected: All tests PASS.

**Step 2: Build**

Run: `go build ./...`
Expected: Clean build.

**Step 3: Verify no untracked files**

Run: `git status`
Expected: Clean working tree, all changes committed.
