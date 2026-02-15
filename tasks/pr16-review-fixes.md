# PR #16 Review Feedback - Comprehensive Fix List

**Generated**: 2026-02-14
**Total Issues**: 22 (2 Critical, 7 Major, 8 Minor, 5 Info)

---

## 🔴 CRITICAL (Must Fix) - 2 Issues

### 1. JAM Message Base Resource Leak
**File**: `internal/message/manager.go:477`
**Severity**: 🔴 CRITICAL
**Issue**: GetBase() returns unclosed bases, causing resource leaks
**Impact**: Memory/file descriptor exhaustion over time
**Fix**:
- Add Close() calls at all GetBase() call sites
- Consider implementing context manager pattern or defer Close()
- Audit all call sites: tosser/export.go, tosser/import.go, message readers

### 2. Door Registry Missing/Incomplete
**File**: `internal/menu/door_handler.go:903`
**Severity**: 🔴 CRITICAL
**Issue**: DoorRegistry implementation not found or incomplete
**Impact**: Door execution may fail
**Fix**:
- Verify DoorRegistry exists and is properly registered
- Ensure GetDoorConfig can find door configurations
- Add integration tests for door loading

---

## 🟠 MAJOR (Should Fix) - 7 Issues

### 3. SSH Server Race Condition (readInterrupt)
**File**: `internal/sshserver/server.go:494`
**Severity**: 🟠 MAJOR
**Reviewer**: Copilot
**Issue**: readInterrupt channel accessed without mutex in Read() select statement
**Impact**: Missed interrupts, reading from closed/nil channel, potential panics
**Fix**:
```go
// Option 1: Copy channel reference under lock
mu.Lock()
interruptChan := readInterrupt
mu.Unlock()
select {
case <-interruptChan:
    // handle
}

// Option 2: Hold lock during entire select (may impact perf)
mu.Lock()
select {
case <-readInterrupt:
    // handle
}
mu.Unlock()
```

### 4. Telnet Server Race Condition (readInterrupt)
**File**: `internal/telnetserver/telnet.go:348`
**Severity**: 🟠 MAJOR
**Reviewer**: Copilot
**Issue**: Same race condition as SSH server
**Impact**: Deadlocks or missed interrupts
**Fix**: Apply same solution as SSH server (#3)

### 5. ConfigWatcher Race Condition
**File**: `cmd/vision3/config_watcher.go:125`
**Severity**: 🟠 MAJOR
**Reviewer**: CodeRabbit
**Issue**: Stop() nils out watcher while watchLoop still selects on it
**Impact**: Panic or race condition
**Fix** (provided by CodeRabbit):
```diff
-	go cw.watchLoop()
+	go cw.watchLoop(watcher)

-func (cw *ConfigWatcher) watchLoop() {
+func (cw *ConfigWatcher) watchLoop(w *fsnotify.Watcher) {
-		case event, ok := <-cw.watcher.Events:
+		case event, ok := <-w.Events:
-		case err, ok := <-cw.watcher.Errors:
+		case err, ok := <-w.Errors:
```

### 6. Message Manager Error Handling
**File**: `internal/message/manager.go:155`
**Severity**: 🟠 MAJOR
**Reviewer**: CodeRabbit
**Issue**: Returns (0, nil) for any openBase error, masking I/O failures
**Impact**: Silent failures, incorrect counts
**Fix**:
- Create sentinel error for "area not found"
- Propagate I/O and corruption errors
- Only return (0, nil) for missing areas

### 7. SSH Password Auth Default-Deny
**File**: `internal/sshserver/callbacks.go:35`
**Severity**: 🟠 MAJOR
**Reviewer**: CodeRabbit
**Issue**: If authPasswordFunc is nil, any password is accepted
**Impact**: Open access with misconfiguration
**Fix**:
```go
if cs.server.authPasswordFunc == nil {
    log.Printf("SECURITY: No password handler - denying access")
    return false
}
return cs.server.authPasswordFunc(cs.username, pwd)
```

### 8. Dropfile Permissions Too Permissive
**File**: `internal/menu/door_handler.go:317`
**Severity**: 🟠 MAJOR
**Reviewer**: CodeRabbit
**Issue**: Dropfiles written with 0644/0755 - world readable
**Impact**: PII exposure (real name, location, phone) on multi-user systems
**Fix**: Change to 0600 for files, 0700 for directories

### 9. ANSI Renderer Strips Styled Spaces
**File**: `internal/menu/ansi_renderer.go:409`
**Severity**: 🟠 MAJOR
**Reviewer**: CodeRabbit
**Issue**: Trailing space trimming removes styled spaces (background colors)
**Impact**: ANSI art loses background formatting
**Fix**: Treat styled spaces as significant or disable trimming

---

## 🟡 MINOR (Nice to Fix) - 8 Issues

### 10. JAM Lock Thread Safety
**File**: `internal/jam/lock.go:12`
**Severity**: 🟡 MINOR
**Reviewer**: Copilot
**Issue**: Changed lock constants to variables - breaks thread safety
**Impact**: Concurrent tests could cause races
**Fix Options**:
- Make lock params per-instance (preferred)
- Revert to constants and use test-specific timeouts

### 11. readByteWithTimeout Buffering Issue
**File**: `internal/editor/input.go:86`
**Severity**: 🟡 MINOR
**Reviewer**: Copilot
**Issue**: Deadline on readDeadlineIO but read from bufio.Reader
**Impact**: Timeout inconsistent depending on buffer state
**Fix**: Ensure deadline applies to actual read operation

### 12. Timezone Comment Misleading
**File**: `templates/configs/config.json:4`
**Severity**: 🟡 MINOR
**Reviewer**: Copilot
**Issue**: Example shows "America/Los_Angeles" but BBS is in St. Louis (314)
**Impact**: Confusing for sysops during setup
**Fix**: Change to "America/Chicago" or neutral example

### 13. CP437/UTF-8 Mixed Encoding
**File**: `internal/terminalio/writer.go:47`
**Severity**: 🟡 MINOR
**Reviewer**: CodeRabbit
**Issue**: Treats any span with invalid byte as CP437, corrupting valid UTF-8
**Impact**: Mixed UTF-8 + CP437 text renders incorrectly
**Fix**: Decode rune-by-rune, only map invalid bytes to CP437

### 14. Admin Log PII Concerns
**File**: `internal/user/admin_log.go:17`
**Severity**: 🟡 MINOR
**Reviewer**: CodeRabbit
**Issue**: OldValue/NewValue may contain PII
**Impact**: Privacy compliance (GDPR/CCPA)
**Fix**:
- Document PII handling in comments
- Consider redacting sensitive fields
- Ensure proper access controls on admin_activity.json

### 15. DOORLIST.BOT File Encoding Corrupt
**File**: `menus/v3/templates/DOORLIST.BOT:1`
**Severity**: 🟡 MINOR
**Reviewer**: CodeRabbit
**Issue**: File encoding corrupted (contains U+FFFD replacement chars)
**Impact**: Door list display broken
**Fix**: Restore proper ESC bytes and CP437 encoding

### 16. DOORLIST.TOP Encoding Issue
**File**: `menus/v3/templates/DOORLIST.TOP:2`
**Severity**: 🟡 MINOR
**Reviewer**: CodeRabbit
**Issue**: ESC byte corrupted to U+FFFD
**Impact**: Display formatting broken
**Fix**: Restore ESC byte and CP437 encoding

### 17. ONELINER.TOP Encoding Check
**File**: `menus/v3/templates/ONELINER.TOP:11`
**Severity**: 🟡 MINOR
**Reviewer**: CodeRabbit
**Issue**: Potential encoding issue
**Impact**: One-liner display may be affected
**Fix**: Verify and restore proper CP437 encoding

---

## 📘 INFO/Documentation (Optional) - 5 Issues

### 18. doors.json Hot-Reload Config
**File**: `documentation/doors.md:219`
**Severity**: 📘 INFO
**Reviewer**: CodeRabbit
**Issue**: Verify doors.json is included in hot-reload configuration
**Fix**: Document or add doors.json to config watcher

### 19. Missing Code Block Language (doors.md)
**File**: `documentation/doors.md:301`
**Severity**: 📘 INFO
**Reviewer**: CodeRabbit
**Issue**: Fenced code block without language specifier (MD040)
**Fix**: Add ```text to directory structure diagrams

### 20. Missing Code Block Language (login-sequence.md)
**File**: `documentation/login-sequence.md:88`
**Severity**: 📘 INFO
**Reviewer**: CodeRabbit
**Issue**: Fenced code block without language tag (MD040)
**Fix**: Add ```text to example output

### 21-22. Additional Documentation Issues
**Files**: Various documentation files
**Severity**: 📘 INFO
**Reviewer**: CodeRabbit
**Issue**: Minor markdown linting issues
**Fix**: Run markdownlint and fix formatting

---

## Fix Priority Order

### Phase 1: Critical Blockers (Must complete before merge)
1. ✅ JAM Message Base resource leaks (#1)
2. ✅ Door Registry verification (#2)

### Phase 2: Major Issues (Should complete before merge)
3. ✅ SSH readInterrupt race (#3)
4. ✅ Telnet readInterrupt race (#4)
5. ✅ ConfigWatcher race (#5)
6. ✅ Message Manager error handling (#6)
7. ✅ SSH password auth default-deny (#7)
8. ✅ Dropfile permissions (#8)
9. ✅ ANSI renderer styled spaces (#9)

### Phase 3: Minor Issues (Can defer or address incrementally)
10-17: Minor fixes (thread safety, encoding, etc.)

### Phase 4: Documentation (Low priority)
18-22: Documentation and linting

---

## Testing Plan

After fixes:
1. Run all existing tests: `go test ./...`
2. Add race detector: `go test -race ./...`
3. Manual testing:
   - SSH/Telnet connection with interrupts
   - Door execution
   - Config hot-reload
   - Message reading/writing
4. Verify no resource leaks (monitor file descriptors)
5. Build and smoke test: `./build.sh && ./bin/vision3`

---

## Notes

- Most critical issues are concurrency/race conditions
- Several security issues (auth, permissions, PII)
- Multiple encoding corruption issues in template files
- No human reviewer feedback yet - this is all automated analysis

