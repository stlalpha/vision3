package jam

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	lockRetryDelay = 200 * time.Millisecond
	lockTimeout    = 30 * time.Second
	lockStaleAfter = 10 * time.Minute
	lockMu         sync.RWMutex // Protects lock configuration variables for thread-safe test access
)

// acquireFileLock serializes cross-process writes to a JAM base using a .bsy lock file.
// It returns a release function that must be called to drop the lock.
func (b *Base) acquireFileLock() (func(), error) {
	if b.BasePath == "" {
		return func() {}, nil
	}
	lockPath := b.BasePath + ".bsy"

	// Read lock config under mutex for thread safety
	lockMu.RLock()
	timeout := lockTimeout
	retryDelay := lockRetryDelay
	staleAfter := lockStaleAfter
	lockMu.RUnlock()

	deadline := time.Now().Add(timeout)

	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "pid=%d time=%s\n", os.Getpid(), time.Now().Format(time.RFC3339))
			_ = f.Close()
			break
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("jam: lock %s: %w", lockPath, err)
		}

		if info, statErr := os.Stat(lockPath); statErr == nil {
			if time.Since(info.ModTime()) > staleAfter {
				_ = os.Remove(lockPath)
				continue
			}
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("jam: timeout waiting for lock %s", lockPath)
		}
		time.Sleep(retryDelay)
	}
	return func() {
		_ = os.Remove(lockPath)
	}, nil
}

// withFileLock runs fn with the lock held.
func (b *Base) withFileLock(fn func() error) error {
	release, err := b.acquireFileLock()
	if err != nil {
		return err
	}
	defer release()
	return fn()
}
