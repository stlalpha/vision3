package jam

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAcquireFileLockSuccess(t *testing.T) {
	dir := t.TempDir()
	b := &Base{BasePath: filepath.Join(dir, "lock-success")}

	release, err := b.acquireFileLock()
	if err != nil {
		t.Fatalf("acquireFileLock: %v", err)
	}

	lockPath := b.BasePath + ".bsy"
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lock file to exist: %v", err)
	}

	release()

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock file to be removed, got: %v", err)
	}
}

func TestAcquireFileLockTimeout(t *testing.T) {
	origRetry := lockRetryDelay
	origTimeout := lockTimeout
	origStale := lockStaleAfter
	lockRetryDelay = 5 * time.Millisecond
	lockTimeout = 50 * time.Millisecond
	lockStaleAfter = time.Hour
	defer func() {
		lockRetryDelay = origRetry
		lockTimeout = origTimeout
		lockStaleAfter = origStale
	}()

	dir := t.TempDir()
	b := &Base{BasePath: filepath.Join(dir, "lock-timeout")}

	release, err := b.acquireFileLock()
	if err != nil {
		t.Fatalf("first acquireFileLock: %v", err)
	}
	defer release()

	_, err = b.acquireFileLock()
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timeout waiting for lock") {
		t.Fatalf("expected timeout waiting for lock error, got: %v", err)
	}
}

func TestAcquireFileLockRemovesStaleLock(t *testing.T) {
	origRetry := lockRetryDelay
	origTimeout := lockTimeout
	origStale := lockStaleAfter
	lockRetryDelay = 5 * time.Millisecond
	lockTimeout = 200 * time.Millisecond
	lockStaleAfter = 10 * time.Millisecond
	defer func() {
		lockRetryDelay = origRetry
		lockTimeout = origTimeout
		lockStaleAfter = origStale
	}()

	dir := t.TempDir()
	basePath := filepath.Join(dir, "lock-stale")
	lockPath := basePath + ".bsy"

	if err := os.WriteFile(lockPath, []byte("stale"), 0644); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("set stale lock mtime: %v", err)
	}

	b := &Base{BasePath: basePath}
	release, err := b.acquireFileLock()
	if err != nil {
		t.Fatalf("acquireFileLock with stale lock: %v", err)
	}
	defer release()

	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lock file to exist after re-acquire: %v", err)
	}
}
