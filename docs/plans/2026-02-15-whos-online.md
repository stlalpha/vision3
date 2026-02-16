# Who's Online Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show all active BBS connections (authenticated and unauthenticated) via a template-based display accessible from the main menu.

**Architecture:** Create a `SessionRegistry` in `internal/session/` that tracks active sessions. `main.go` registers/unregisters on connect/disconnect and passes the registry to `MenuExecutor`. A new `RUN:WHOISONLINE` command queries the registry and renders output using WHOONLN.TOP/MID/BOT templates.

**Tech Stack:** Go standard library, existing template/pipe-code rendering pipeline.

---

### Task 1: Create SessionRegistry

**Files:**
- Create: `internal/session/registry.go`
- Create: `internal/session/registry_test.go`

**Step 1: Write the test file**

```go
package session

import (
	"testing"
	"time"
)

func TestRegistryRegisterAndList(t *testing.T) {
	r := NewSessionRegistry()

	s1 := &BbsSession{NodeID: 1, StartTime: time.Now(), CurrentMenu: "MAIN"}
	s2 := &BbsSession{NodeID: 3, StartTime: time.Now(), CurrentMenu: "LOGIN"}

	r.Register(s1)
	r.Register(s2)

	active := r.ListActive()
	if len(active) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(active))
	}
	// Should be sorted by NodeID
	if active[0].NodeID != 1 || active[1].NodeID != 3 {
		t.Errorf("expected sorted by NodeID [1,3], got [%d,%d]", active[0].NodeID, active[1].NodeID)
	}
}

func TestRegistryUnregister(t *testing.T) {
	r := NewSessionRegistry()

	s1 := &BbsSession{NodeID: 1, StartTime: time.Now()}
	r.Register(s1)
	r.Unregister(1)

	active := r.ListActive()
	if len(active) != 0 {
		t.Fatalf("expected 0 sessions after unregister, got %d", len(active))
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewSessionRegistry()

	s1 := &BbsSession{NodeID: 2, StartTime: time.Now()}
	r.Register(s1)

	got := r.Get(2)
	if got == nil || got.NodeID != 2 {
		t.Errorf("expected session with NodeID 2, got %v", got)
	}

	if r.Get(99) != nil {
		t.Error("expected nil for nonexistent node")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/session/ -run TestRegistry -v`
Expected: FAIL — `NewSessionRegistry` undefined

**Step 3: Write the implementation**

```go
package session

import (
	"sort"
	"sync"
)

// SessionRegistry tracks all active BBS sessions.
type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[int]*BbsSession // keyed by NodeID
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions: make(map[int]*BbsSession),
	}
}

func (r *SessionRegistry) Register(s *BbsSession) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.NodeID] = s
}

func (r *SessionRegistry) Unregister(nodeID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, nodeID)
}

func (r *SessionRegistry) Get(nodeID int) *BbsSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessions[nodeID]
}

func (r *SessionRegistry) ListActive() []*BbsSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*BbsSession, 0, len(r.sessions))
	for _, s := range r.sessions {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].NodeID < result[j].NodeID
	})
	return result
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/session/ -run TestRegistry -v`
Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add internal/session/registry.go internal/session/registry_test.go
git commit -m "feat(session): add SessionRegistry for tracking active sessions"
```

---

### Task 2: Add LastActivity field to BbsSession

**Files:**
- Modify: `internal/session/session.go:19-36` — add `LastActivity time.Time` field

**Step 1: Add the field**

In `internal/session/session.go`, add `LastActivity time.Time` after the `StartTime` field (line 35):

```go
StartTime    time.Time            // Tracks the session start time
LastActivity time.Time            // Tracks last user input for idle calculation
```

**Step 2: Verify build**

Run: `go build ./internal/session/`
Expected: Success

**Step 3: Commit**

```bash
git add internal/session/session.go
git commit -m "feat(session): add LastActivity field for idle tracking"
```

---

### Task 3: Wire SessionRegistry into main.go

**Files:**
- Modify: `cmd/vision3/main.go`

This task replaces the existing `activeSessions` map with `SessionRegistry` and creates `BbsSession` objects for each connection.

**Step 1: Add registry as package-level var**

In `cmd/vision3/main.go`, in the `var` block near line 47, add:

```go
sessionRegistry *session.SessionRegistry
```

Add import for `"github.com/stlalpha/vision3/internal/session"` if not already present.

**Step 2: Initialize registry before executor creation**

Near line 1366 (before `menuExecutor = menu.NewExecutor(...)`), add:

```go
sessionRegistry = session.NewSessionRegistry()
```

**Step 3: Register sessions in sessionHandler**

In `sessionHandler()` (near line 946 where `sessionStartTime := time.Now()`), after the session start time is set, create and register a BbsSession:

```go
sessionStartTime := time.Now()
bbsSession := &session.BbsSession{
	NodeID:       int(nodeID),
	StartTime:    sessionStartTime,
	LastActivity: sessionStartTime,
	CurrentMenu:  "LOGIN",
	RemoteAddr:   s.RemoteAddr(),
}
sessionRegistry.Register(bbsSession)
```

**Step 4: Unregister in the defer cleanup**

In the deferred cleanup function (near line 798-800 where `delete(activeSessions, s)` happens), add after that line:

```go
sessionRegistry.Unregister(int(nodeID))
```

**Step 5: Update bbsSession.User after authentication**

After authentication succeeds (where `authenticatedUser` is set), update the session:

```go
bbsSession.User = authenticatedUser
bbsSession.CurrentMenu = currentMenuName
```

Search for where `authenticatedUser` is assigned after login and add the update there.

**Step 6: Pass registry to MenuExecutor**

This requires modifying the `NewExecutor` signature (Task 4), so for now just verify the build compiles with the registry creation and session registration. The `activeSessions` map stays for now — both can coexist until Task 4 wires it through.

Run: `go build ./cmd/vision3/`
Expected: Success

**Step 7: Commit**

```bash
git add cmd/vision3/main.go
git commit -m "feat: register BbsSessions with SessionRegistry in main.go"
```

---

### Task 4: Add SessionRegistry to MenuExecutor

**Files:**
- Modify: `internal/menu/executor.go:62-112` — add field and constructor param

**Step 1: Add field to MenuExecutor struct**

In `internal/menu/executor.go`, add to the `MenuExecutor` struct (after line 78, `IPLockoutCheck`):

```go
SessionRegistry *session.SessionRegistry // Active session registry for who's online
```

Add import: `"github.com/stlalpha/vision3/internal/session"`

**Step 2: Update NewExecutor signature and body**

Add `sessionRegistry *session.SessionRegistry` parameter to `NewExecutor` (line 86) and assign it in the return struct.

**Step 3: Update the call site in main.go**

In `cmd/vision3/main.go` line 1366, add `sessionRegistry` to the `NewExecutor` call.

**Step 4: Verify build**

Run: `go build ./...`
Expected: Success

**Step 5: Commit**

```bash
git add internal/menu/executor.go cmd/vision3/main.go
git commit -m "feat: pass SessionRegistry to MenuExecutor"
```

---

### Task 5: Create WHOONLN template files

**Files:**
- Create: `menus/v3/templates/WHOONLN.TOP`
- Create: `menus/v3/templates/WHOONLN.MID`
- Create: `menus/v3/templates/WHOONLN.BOT`

**Step 1: Create WHOONLN.TOP**

Simple header. Use pipe codes for color (consistent with existing templates):

```
|12Who's Online|07
|08────────────────────────────────────────────────────────────────────────────────
|08 Node  User Name            Activity             Time On  Idle
|08────────────────────────────────────────────────────────────────────────────────
```

**Step 2: Create WHOONLN.MID**

Template line with AT-token placeholders (same pattern as LASTCALL.MID):

```
|07 @ND:4@ |15@UN:20@ |07@LO:20@ |14@TO:8@ |08@ID:5@
```

**Step 3: Create WHOONLN.BOT**

Footer:

```
|08────────────────────────────────────────────────────────────────────────────────
|07 @NODECT@ node(s) active
```

**Step 4: Commit**

```bash
git add menus/v3/templates/WHOONLN.TOP menus/v3/templates/WHOONLN.MID menus/v3/templates/WHOONLN.BOT
git commit -m "feat: add WHOONLN template files for who's online display"
```

---

### Task 6: Implement runWhoIsOnline command

**Files:**
- Modify: `internal/menu/executor.go` — add `runWhoIsOnline` function and register it

**Step 1: Register the command**

In `registerAppRunnables` (line 297), add:

```go
registry["WHOISONLINE"] = runWhoIsOnline
```

**Step 2: Implement runWhoIsOnline**

Follow the same pattern as `runLastCallers` (line 2297). The function:

1. Loads WHOONLN.TOP/MID/BOT templates
2. Strips SAUCE, normalizes pipe delimiters, processes pipe codes
3. Gets active sessions from `e.SessionRegistry.ListActive()`
4. For each session, substitutes tokens in MID template:
   - `@ND@` / `@ND:W@` — NodeID
   - `@UN@` / `@UN:W@` — User handle, or "Logging In..." if User is nil
   - `@LO@` / `@LO:W@` — CurrentMenu
   - `@TO@` / `@TO:W@` — time online as HH:MM (`time.Since(session.StartTime)`)
   - `@ID@` / `@ID:W@` — idle time as MM:SS (`time.Since(session.LastActivity)`)
5. Substitutes `@NODECT@` in BOT template with active session count
6. Writes output to terminal, waits for keypress

```go
func runWhoIsOnline(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running WHOISONLINE", nodeNumber)

	topPath := filepath.Join(e.MenuSetPath, "templates", "WHOONLN.TOP")
	midPath := filepath.Join(e.MenuSetPath, "templates", "WHOONLN.MID")
	botPath := filepath.Join(e.MenuSetPath, "templates", "WHOONLN.BOT")

	topBytes, errTop := os.ReadFile(topPath)
	midBytes, errMid := os.ReadFile(midPath)
	botBytes, errBot := os.ReadFile(botPath)

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load WHOONLN templates: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading Who's Online templates.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading WHOONLN templates")
	}

	// Process templates: strip SAUCE, normalize pipes, replace pipe codes
	topBytes = stripSauceMetadata(topBytes)
	midBytes = stripSauceMetadata(midBytes)
	botBytes = stripSauceMetadata(botBytes)
	topBytes = normalizePipeCodeDelimiters(topBytes)
	midBytes = normalizePipeCodeDelimiters(midBytes)
	botBytes = normalizePipeCodeDelimiters(botBytes)

	processedTop := string(ansi.ReplacePipeCodes(topBytes))
	processedMid := string(ansi.ReplacePipeCodes(midBytes))
	processedBot := string(ansi.ReplacePipeCodes(botBytes))

	// Get active sessions
	sessions := e.SessionRegistry.ListActive()

	// Build output
	var buf bytes.Buffer
	buf.WriteString(processedTop)
	if !strings.HasSuffix(processedTop, "\r\n") && !strings.HasSuffix(processedTop, "\n") {
		buf.WriteString("\r\n")
	}

	now := time.Now()
	for _, sess := range sessions {
		line := processedMid

		// Node ID
		nodeStr := strconv.Itoa(sess.NodeID)
		line = replaceWhoOnlineToken(line, "ND", nodeStr)

		// Username
		userName := "Logging In..."
		if sess.User != nil {
			userName = sess.User.Handle
		}
		line = replaceWhoOnlineToken(line, "UN", userName)

		// Activity (current menu)
		activity := sess.CurrentMenu
		if activity == "" {
			activity = "---"
		}
		line = replaceWhoOnlineToken(line, "LO", activity)

		// Time online (HH:MM)
		dur := now.Sub(sess.StartTime)
		hours := int(dur.Hours())
		mins := int(dur.Minutes()) % 60
		timeOn := fmt.Sprintf("%d:%02d", hours, mins)
		line = replaceWhoOnlineToken(line, "TO", timeOn)

		// Idle time (MM:SS)
		idle := now.Sub(sess.LastActivity)
		idleMins := int(idle.Minutes())
		idleSecs := int(idle.Seconds()) % 60
		idleStr := fmt.Sprintf("%d:%02d", idleMins, idleSecs)
		line = replaceWhoOnlineToken(line, "ID", idleStr)

		buf.WriteString(line)
		if !strings.HasSuffix(line, "\r\n") && !strings.HasSuffix(line, "\n") {
			buf.WriteString("\r\n")
		}
	}

	// Process bot template — replace @NODECT@
	processedBot = replaceWhoOnlineToken(processedBot, "NODECT", strconv.Itoa(len(sessions)))
	buf.WriteString(processedBot)
	if !strings.HasSuffix(processedBot, "\r\n") && !strings.HasSuffix(processedBot, "\n") {
		buf.WriteString("\r\n")
	}

	terminalio.WriteProcessedBytes(terminal, []byte(buf.String()), outputMode)

	// Wait for keypress
	e.waitForAnyKey(s, terminal, nodeNumber)

	return currentUser, "", nil
}
```

**Step 3: Add the token replacement helper**

Use the existing `lastCallerATTokenRegex` pattern. Add a helper that replaces `@TOKEN@` and `@TOKEN:WIDTH@`:

```go
func replaceWhoOnlineToken(line, token, value string) string {
	return lastCallerATTokenRegex.ReplaceAllStringFunc(line, func(match string) string {
		parts := lastCallerATTokenRegex.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		code := strings.ToUpper(parts[1])
		if code != token {
			return match
		}
		if len(parts) > 2 && parts[2] != "" {
			if width, err := strconv.Atoi(parts[2]); err == nil {
				return formatLastCallerATWidth(value, width, true)
			}
		}
		return value
	})
}
```

**Step 4: Verify build**

Run: `go build ./...`
Expected: Success

**Step 5: Commit**

```bash
git add internal/menu/executor.go
git commit -m "feat: implement RUN:WHOISONLINE command (VIS-53)"
```

---

### Task 7: Update menu config and update bbsSession state

**Files:**
- Modify: `menus/v3/cfg/MAIN.CFG` — change `/W` from placeholder to `RUN:WHOISONLINE`
- Modify: `cmd/vision3/main.go` — update `bbsSession.CurrentMenu` during menu navigation

**Step 1: Update MAIN.CFG**

Change the `/W` entry (line 189-193) from:

```json
"CMD": "RUN:PLACEHOLDER WhosOnline",
```

to:

```json
"CMD": "RUN:WHOISONLINE",
```

**Step 2: Update bbsSession.CurrentMenu during menu loop**

In `cmd/vision3/main.go`, in the main menu loop where `currentMenuName` changes, update the bbsSession:

```go
bbsSession.CurrentMenu = currentMenuName
```

This should go right after `currentMenuName` is reassigned in the loop.

**Step 3: Update bbsSession.LastActivity on input**

Find where user input is read in the main loop or in `MenuExecutor.Run()` and update `bbsSession.LastActivity = time.Now()`. The simplest approach: update it each time the menu loop iterates (each menu execution represents user activity).

In the main menu loop in `main.go`, before calling `menuExecutor.Run()`:

```go
bbsSession.LastActivity = time.Now()
```

**Step 4: Verify build**

Run: `go build ./...`
Expected: Success

**Step 5: Manual test**

SSH into the BBS, navigate to Main Menu, type `/W`. Should display the who's online screen showing your own session.

**Step 6: Commit**

```bash
git add menus/v3/cfg/MAIN.CFG cmd/vision3/main.go
git commit -m "feat: wire /W to WHOISONLINE and update session state in menu loop"
```

---

### Task 8: Final verification and cleanup

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All pass

**Step 2: Run vet and build**

Run: `go vet ./... && go build ./...`
Expected: Clean

**Step 3: Final commit if any cleanup needed**

**Step 4: Close VIS-53 in Linear**
