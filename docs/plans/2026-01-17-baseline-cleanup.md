# Baseline Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Get the ViSiON/3 codebase to a maintainable baseline state by reorganizing executor.go, cleaning up debug noise, adding CI, and testing critical packages.

**Architecture:** Extract the 5,447-line executor.go into focused modules by responsibility (loader, dispatcher, registry) and domain (messaging, files, system, misc). Add a logging utility to gate debug output. Establish CI with linting and tests. Add unit tests for user/message/file packages.

**Tech Stack:** Go 1.24, golangci-lint, GitHub Actions, bcrypt, JSONL storage

---

## Phase 1: Foundation

### Task 1: Fix Mutex Copy Bug

**Files:**
- Modify: `internal/configtool/doors/resources.go:217,231`

**Step 1: Read the problematic code**

Lines 217 and 231 copy `*resource` which includes the embedded `sync.RWMutex` at `DoorResource.mu`. This breaks synchronization.

**Step 2: Fix GetResourceStatus (line 217)**

Replace:
```go
resourceCopy := *resource
```

With:
```go
resourceCopy := DoorResource{
    ID:           resource.ID,
    Type:         resource.Type,
    Path:         resource.Path,
    Description:  resource.Description,
    MaxLocks:     resource.MaxLocks,
    CurrentLocks: resource.CurrentLocks,
    LockMode:     resource.LockMode,
    Locks:        make([]ResourceLock, len(resource.Locks)),
}
copy(resourceCopy.Locks, resource.Locks)
```

**Step 3: Fix GetAllResources (line 231)**

Apply same fix pattern at line 231.

**Step 4: Verify fix**

Run: `go vet ./internal/configtool/...`
Expected: No mutex copy errors

**Step 5: Commit**

```bash
git add internal/configtool/doors/resources.go
git commit -m "fix: resolve mutex copy bug in DoorResource

Copying DoorResource struct was copying the embedded sync.RWMutex,
which breaks synchronization. Now manually copying fields instead."
```

---

### Task 2: Create Logging Utility

**Files:**
- Create: `internal/logging/logging.go`
- Create: `internal/logging/logging_test.go`

**Step 1: Write the test**

```go
// internal/logging/logging_test.go
package logging

import (
    "bytes"
    "log"
    "os"
    "testing"
)

func TestDebugDisabled(t *testing.T) {
    DebugEnabled = false
    var buf bytes.Buffer
    log.SetOutput(&buf)
    defer log.SetOutput(os.Stderr)

    Debug("this should not appear")

    if buf.Len() > 0 {
        t.Errorf("Debug output when disabled: %s", buf.String())
    }
}

func TestDebugEnabled(t *testing.T) {
    DebugEnabled = true
    var buf bytes.Buffer
    log.SetOutput(&buf)
    defer log.SetOutput(os.Stderr)

    Debug("test message %d", 42)

    if !bytes.Contains(buf.Bytes(), []byte("DEBUG: test message 42")) {
        t.Errorf("Expected debug output, got: %s", buf.String())
    }
    DebugEnabled = false
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/logging/...`
Expected: FAIL (package doesn't exist)

**Step 3: Write the implementation**

```go
// internal/logging/logging.go
package logging

import "log"

// DebugEnabled controls whether Debug() produces output.
// Set via -debug flag or DEBUG=1 environment variable.
var DebugEnabled bool

// Debug logs a message only when DebugEnabled is true.
func Debug(format string, args ...any) {
    if DebugEnabled {
        log.Printf("DEBUG: "+format, args...)
    }
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/logging/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/logging/
git commit -m "feat: add debug logging utility

Provides gated debug output controlled by DebugEnabled flag.
Allows cleaning up DEBUG statements without losing debug capability."
```

---

### Task 3: Create Makefile

**Files:**
- Create: `Makefile`

**Step 1: Write the Makefile**

```makefile
.PHONY: all build test test-race vet lint coverage clean

# Default target
all: lint vet test build

# Build all binaries
build:
	go build ./cmd/...

# Run tests
test:
	go test ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Run go vet
vet:
	go vet ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Generate coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Clean build artifacts
clean:
	rm -f cmd/vision3/vision3
	rm -f cmd/vision3-config/vision3-config
	rm -f cmd/vision3-bbsconfig/vision3-bbsconfig
	rm -f cmd/install/install
	rm -f coverage.out coverage.html
```

**Step 2: Test it works**

Run: `make vet test build`
Expected: All commands succeed

**Step 3: Commit**

```bash
git add Makefile
git commit -m "build: add Makefile with standard targets

Targets: all, build, test, test-race, vet, lint, coverage, clean"
```

---

### Task 4: Create Linter Configuration

**Files:**
- Create: `.golangci-lint.yml`

**Step 1: Write the config**

```yaml
# .golangci-lint.yml
run:
  timeout: 5m
  skip-dirs:
    - internal/configtool  # Skip for now - 32K lines, needs separate effort

linters:
  enable:
    - errcheck      # Unchecked errors
    - govet         # Suspicious constructs
    - staticcheck   # Bugs and simplifications
    - unused        # Dead code
    - ineffassign   # Ineffective assignments
    - gosimple      # Simplifications

linters-settings:
  errcheck:
    check-type-assertions: true

issues:
  exclude-rules:
    # Allow log.Printf without checking error (common pattern)
    - linters:
        - errcheck
      source: "log\\.Printf"
```

**Step 2: Run linter**

Run: `golangci-lint run`
Expected: May show some issues (that's fine, we'll address later)

**Step 3: Commit**

```bash
git add .golangci-lint.yml
git commit -m "build: add golangci-lint configuration

Enables errcheck, govet, staticcheck, unused, ineffassign, gosimple.
Skips configtool for now (separate cleanup effort)."
```

---

### Task 5: Create GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`

**Step 1: Create directory**

```bash
mkdir -p .github/workflows
```

**Step 2: Write the workflow**

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main, "1.0alpha"]
  pull_request:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.24"

      - name: Vet
        run: go vet ./...

      - name: Install golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest
          args: --timeout=5m

      - name: Test
        run: go test -race ./...

      - name: Build
        run: go build ./cmd/...
```

**Step 3: Commit**

```bash
git add .github/
git commit -m "ci: add GitHub Actions workflow

Runs vet, lint, test (with race detector), and build on push/PR."
```

---

## Phase 2: Executor Reorganization

### Task 6: Extract loader.go

**Files:**
- Create: `internal/menu/loader.go`
- Modify: `internal/menu/executor.go`

**Step 1: Create loader.go with menu/config loading functions**

Extract these functions from executor.go:
- `LoadMenu` (around line 270-400)
- `loadMenuConfig`
- ANSI file loading helpers

```go
// internal/menu/loader.go
package menu

// LoadMenu loads a menu definition from a .MNU file.
// [Move the LoadMenu function here]

// loadMenuConfig loads menu command configuration from a .CFG file.
// [Move the loadMenuConfig function here]
```

**Step 2: Update executor.go imports and remove moved functions**

Remove the moved functions from executor.go. Ensure loader.go has the necessary imports.

**Step 3: Verify build**

Run: `go build ./cmd/vision3`
Expected: Build succeeds

**Step 4: Run tests**

Run: `go test ./internal/menu/...`
Expected: All tests pass

**Step 5: Commit**

```bash
git add internal/menu/loader.go internal/menu/executor.go
git commit -m "refactor: extract menu loading into loader.go

Moves LoadMenu and config loading functions out of executor.go.
First step in breaking up the 5,447-line file."
```

---

### Task 7: Extract dispatcher.go

**Files:**
- Create: `internal/menu/dispatcher.go`
- Modify: `internal/menu/executor.go`

**Step 1: Create dispatcher.go with command routing**

Extract command parsing and routing logic:
- Command parsing (GOTO:, RUN:, DOOR:, etc.)
- Dispatch to appropriate handlers

**Step 2: Verify build and tests**

Run: `go build ./cmd/vision3 && go test ./internal/menu/...`
Expected: Success

**Step 3: Commit**

```bash
git add internal/menu/dispatcher.go internal/menu/executor.go
git commit -m "refactor: extract command dispatch into dispatcher.go"
```

---

### Task 8: Extract registry.go

**Files:**
- Create: `internal/menu/registry.go`
- Modify: `internal/menu/executor.go`

**Step 1: Create registry.go**

Move:
- `RunnableFunc` type definition
- `registerPlaceholderRunnables` function
- `registerAppRunnables` function (just the registration calls, not the runnable implementations)

**Step 2: Verify build and tests**

Run: `go build ./cmd/vision3 && go test ./internal/menu/...`
Expected: Success

**Step 3: Commit**

```bash
git add internal/menu/registry.go internal/menu/executor.go
git commit -m "refactor: extract runnable registry into registry.go"
```

---

### Task 9: Extract runnables_messaging.go

**Files:**
- Create: `internal/menu/runnables_messaging.go`
- Modify: `internal/menu/executor.go`

**Step 1: Create runnables_messaging.go**

Move these functions:
- `runListMessageAreas` (line 3278)
- `runComposeMessage` (line 3412)
- `runPromptAndComposeMessage` (line 3614)
- `runReadMsgs` (line 3757)
- `runNewscan` (line 4456)

**Step 2: Verify build and tests**

Run: `go build ./cmd/vision3 && go test ./internal/menu/...`
Expected: Success

**Step 3: Commit**

```bash
git add internal/menu/runnables_messaging.go internal/menu/executor.go
git commit -m "refactor: extract messaging runnables into runnables_messaging.go"
```

---

### Task 10: Extract runnables_files.go

**Files:**
- Create: `internal/menu/runnables_files.go`
- Modify: `internal/menu/executor.go`

**Step 1: Create runnables_files.go**

Move these functions:
- `runListFiles` (line 4663)
- `runListFileAreas` (line 5242)
- `runSelectFileArea` (line 5314)

**Step 2: Verify build and tests**

Run: `go build ./cmd/vision3 && go test ./internal/menu/...`
Expected: Success

**Step 3: Commit**

```bash
git add internal/menu/runnables_files.go internal/menu/executor.go
git commit -m "refactor: extract file runnables into runnables_files.go"
```

---

### Task 11: Extract runnables_system.go

**Files:**
- Create: `internal/menu/runnables_system.go`
- Modify: `internal/menu/executor.go`

**Step 1: Create runnables_system.go**

Move these functions:
- `runAuthenticate` (line 1085)
- `runFullLoginSequence` (line 2412)
- `runListUsers` (line 3079)
- `runShowStats` (line 794)
- `runShowVersion` (line 3212)
- `runLastCallers` (line 2075)

**Step 2: Verify build and tests**

Run: `go build ./cmd/vision3 && go test ./internal/menu/...`
Expected: Success

**Step 3: Commit**

```bash
git add internal/menu/runnables_system.go internal/menu/executor.go
git commit -m "refactor: extract system runnables into runnables_system.go"
```

---

### Task 12: Extract runnables_misc.go

**Files:**
- Create: `internal/menu/runnables_misc.go`
- Modify: `internal/menu/executor.go`

**Step 1: Create runnables_misc.go**

Move remaining runnables:
- `runOneliners` (line 893)
- `runSetRender` (line 464)
- Any other remaining runnable functions

**Step 2: Verify build and tests**

Run: `go build ./cmd/vision3 && go test ./internal/menu/...`
Expected: Success

**Step 3: Commit**

```bash
git add internal/menu/runnables_misc.go internal/menu/executor.go
git commit -m "refactor: extract misc runnables into runnables_misc.go"
```

---

### Task 13: Extract runnables_doors.go

**Files:**
- Create: `internal/menu/runnables_doors.go`
- Modify: `internal/menu/executor.go`

**Step 1: Create runnables_doors.go**

Move door-related code:
- The `DOOR:` handler in registerPlaceholderRunnables
- Any door setup/teardown helpers

**Step 2: Verify build and tests**

Run: `go build ./cmd/vision3 && go test ./internal/menu/...`
Expected: Success

**Step 3: Commit**

```bash
git add internal/menu/runnables_doors.go internal/menu/executor.go
git commit -m "refactor: extract door handling into runnables_doors.go"
```

---

### Task 14: Clean Up Debug Logging

**Files:**
- Modify: `internal/menu/executor.go` (and new extracted files)
- Modify: `cmd/vision3/main.go`

**Step 1: Replace DEBUG statements with logging.Debug**

In each file, replace:
```go
log.Printf("DEBUG: ...")
```

With:
```go
logging.Debug("...")
```

Add import: `"github.com/stlalpha/vision3/internal/logging"`

**Step 2: Delete redundant debug statements**

Remove statements that just say "entering function X" or provide no diagnostic value.

**Step 3: Wire up -debug flag in main.go**

```go
import "github.com/stlalpha/vision3/internal/logging"

func main() {
    debugFlag := flag.Bool("debug", false, "Enable debug logging")
    flag.Parse()

    logging.DebugEnabled = *debugFlag || os.Getenv("DEBUG") == "1"
    // ...
}
```

**Step 4: Verify build and test**

Run: `go build ./cmd/vision3 && go test ./...`
Expected: Success

**Step 5: Commit**

```bash
git add -A
git commit -m "refactor: gate debug logging behind -debug flag

Replaced 200+ log.Printf(\"DEBUG: ...\") with logging.Debug().
Debug output now only appears with -debug flag or DEBUG=1."
```

---

## Phase 3: Core Package Tests

### Task 15: Add User Package Tests

**Files:**
- Create: `internal/user/manager_test.go`

**Step 1: Write tests for user authentication**

```go
// internal/user/manager_test.go
package user

import (
    "os"
    "path/filepath"
    "testing"
)

func TestNewUserManager_CreatesDefaultUser(t *testing.T) {
    tmpDir := t.TempDir()

    um, err := NewUserManager(tmpDir)
    if err != nil {
        t.Fatalf("NewUserManager failed: %v", err)
    }

    user := um.GetUserByUsername("felonius")
    if user == nil {
        t.Error("Expected default felonius user to exist")
    }
}

func TestAuthenticate_ValidPassword(t *testing.T) {
    tmpDir := t.TempDir()
    um, _ := NewUserManager(tmpDir)

    user, err := um.Authenticate("felonius", "password")
    if err != nil {
        t.Fatalf("Authenticate failed: %v", err)
    }
    if user.Username != "felonius" {
        t.Errorf("Expected felonius, got %s", user.Username)
    }
}

func TestAuthenticate_InvalidPassword(t *testing.T) {
    tmpDir := t.TempDir()
    um, _ := NewUserManager(tmpDir)

    _, err := um.Authenticate("felonius", "wrongpassword")
    if err == nil {
        t.Error("Expected error for invalid password")
    }
}

func TestAuthenticate_UnknownUser(t *testing.T) {
    tmpDir := t.TempDir()
    um, _ := NewUserManager(tmpDir)

    _, err := um.Authenticate("nobody", "password")
    if err == nil {
        t.Error("Expected error for unknown user")
    }
}

func TestAddUser_CreatesWithHashedPassword(t *testing.T) {
    tmpDir := t.TempDir()
    um, _ := NewUserManager(tmpDir)

    user, err := um.AddUser("testuser", "testpass", "Test User", "TestHandle", "", "Test")
    if err != nil {
        t.Fatalf("AddUser failed: %v", err)
    }

    if user.Username != "testuser" {
        t.Errorf("Expected testuser, got %s", user.Username)
    }

    // Password should be hashed, not plaintext
    if user.PasswordHash == "testpass" {
        t.Error("Password should be hashed, not stored as plaintext")
    }
}

func TestAddUser_RejectsDuplicateUsername(t *testing.T) {
    tmpDir := t.TempDir()
    um, _ := NewUserManager(tmpDir)

    _, err := um.AddUser("felonius", "pass", "Name", "Handle", "", "Group")
    if err != ErrUserExists {
        t.Errorf("Expected ErrUserExists, got %v", err)
    }
}
```

**Step 2: Run tests**

Run: `go test -v ./internal/user/...`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/user/manager_test.go
git commit -m "test: add user package tests

Tests authentication, password hashing, user creation, and error cases."
```

---

### Task 16: Add Message Package Tests

**Files:**
- Create: `internal/message/manager_test.go`

**Step 1: Write tests for message area loading**

```go
// internal/message/manager_test.go
package message

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
)

func TestNewMessageManager_EmptyDirectory(t *testing.T) {
    tmpDir := t.TempDir()

    mm, err := NewMessageManager(tmpDir)
    if err != nil {
        t.Fatalf("NewMessageManager failed: %v", err)
    }

    areas := mm.GetAllAreas()
    if len(areas) != 0 {
        t.Errorf("Expected 0 areas, got %d", len(areas))
    }
}

func TestNewMessageManager_LoadsAreas(t *testing.T) {
    tmpDir := t.TempDir()

    // Create test areas file
    areas := []*MessageArea{
        {ID: 1, Tag: "GENERAL", Name: "General Discussion"},
        {ID: 2, Tag: "TECH", Name: "Technical Talk"},
    }
    data, _ := json.Marshal(areas)
    os.WriteFile(filepath.Join(tmpDir, "message_areas.json"), data, 0644)

    mm, err := NewMessageManager(tmpDir)
    if err != nil {
        t.Fatalf("NewMessageManager failed: %v", err)
    }

    loadedAreas := mm.GetAllAreas()
    if len(loadedAreas) != 2 {
        t.Errorf("Expected 2 areas, got %d", len(loadedAreas))
    }
}

func TestGetAreaByID(t *testing.T) {
    tmpDir := t.TempDir()
    areas := []*MessageArea{
        {ID: 1, Tag: "GENERAL", Name: "General Discussion"},
    }
    data, _ := json.Marshal(areas)
    os.WriteFile(filepath.Join(tmpDir, "message_areas.json"), data, 0644)

    mm, _ := NewMessageManager(tmpDir)

    area := mm.GetAreaByID(1)
    if area == nil {
        t.Fatal("Expected area, got nil")
    }
    if area.Tag != "GENERAL" {
        t.Errorf("Expected GENERAL, got %s", area.Tag)
    }
}

func TestGetAreaByID_NotFound(t *testing.T) {
    tmpDir := t.TempDir()
    mm, _ := NewMessageManager(tmpDir)

    area := mm.GetAreaByID(999)
    if area != nil {
        t.Error("Expected nil for non-existent area")
    }
}
```

**Step 2: Run tests**

Run: `go test -v ./internal/message/...`
Expected: PASS (or identify missing methods to implement)

**Step 3: Commit**

```bash
git add internal/message/manager_test.go
git commit -m "test: add message package tests

Tests area loading, lookup by ID/tag, and empty state handling."
```

---

### Task 17: Add File Package Tests

**Files:**
- Create: `internal/file/manager_test.go`

**Step 1: Write tests for file area loading**

```go
// internal/file/manager_test.go
package file

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
)

func TestNewFileManager_EmptyDirectory(t *testing.T) {
    tmpDir := t.TempDir()

    fm, err := NewFileManager(tmpDir, tmpDir)
    if err != nil {
        t.Fatalf("NewFileManager failed: %v", err)
    }

    areas := fm.GetAllAreas()
    if len(areas) != 0 {
        t.Errorf("Expected 0 areas, got %d", len(areas))
    }
}

func TestNewFileManager_LoadsAreas(t *testing.T) {
    tmpDir := t.TempDir()
    configDir := t.TempDir()

    // Create test areas file
    areas := []FileArea{
        {ID: 1, Tag: "UPLOADS", Name: "User Uploads", Path: "/files/uploads"},
    }
    data, _ := json.Marshal(areas)
    os.WriteFile(filepath.Join(configDir, "file_areas.json"), data, 0644)

    fm, err := NewFileManager(tmpDir, configDir)
    if err != nil {
        t.Fatalf("NewFileManager failed: %v", err)
    }

    loadedAreas := fm.GetAllAreas()
    if len(loadedAreas) != 1 {
        t.Errorf("Expected 1 area, got %d", len(loadedAreas))
    }
}
```

**Step 2: Run tests**

Run: `go test -v ./internal/file/...`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/file/manager_test.go
git commit -m "test: add file package tests

Tests file area loading and lookup."
```

---

## Phase 4: Verification

### Task 18: Full Test Suite with Race Detection

**Step 1: Run full test suite**

Run: `go test -race ./...`
Expected: All tests pass, no race conditions detected

**Step 2: Run linter**

Run: `golangci-lint run`
Expected: No errors (warnings may exist in configtool, which is excluded)

**Step 3: Verify build**

Run: `go build ./cmd/...`
Expected: All binaries build successfully

---

### Task 19: Update Documentation

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update package structure section**

Add the new files to the architecture documentation:
- loader.go, dispatcher.go, registry.go
- runnables_*.go files
- internal/logging package

**Step 2: Add Makefile usage**

Document the new make targets.

**Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update architecture documentation

Reflects new package structure after executor.go refactor."
```

---

### Task 20: Final Verification and Tag

**Step 1: Run CI locally**

Run: `make all`
Expected: lint, vet, test, build all succeed

**Step 2: Check file sizes**

Run: `wc -l internal/menu/*.go`
Expected: No file over 600 lines (executor.go should be ~300-400 now)

**Step 3: Commit any final changes and push**

```bash
git push origin 1.0alpha
```

---

## Summary

After completing all tasks:

- **executor.go**: Reduced from 5,447 lines to ~300-400 lines
- **New files**: loader.go, dispatcher.go, registry.go, runnables_*.go, logging/
- **CI**: GitHub Actions running vet, lint, test, build
- **Tests**: User, message, file packages covered
- **Debug logging**: Gated behind -debug flag
- **Mutex bug**: Fixed

Codebase is now at a maintainable baseline ready for continued feature development.
