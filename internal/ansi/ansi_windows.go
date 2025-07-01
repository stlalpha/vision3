//go:build windows

package ansi

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

// setConsoleMode enables VT100 processing for Windows console if possible
func setConsoleMode() error {
	// We'll try the syscalls; they will fail harmlessly on non-Windows.

	kernel32, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		// This likely means not on Windows, which is fine.
		return fmt.Errorf("LoadDLL kernel32.dll failed: %w (likely not Windows)", err)
	}
	// No need to defer kernel32.Release() as LoadDLL does not require explicit release

	getStdHandle, err := kernel32.FindProc("GetStdHandle")
	if err != nil {
		return fmt.Errorf("FindProc GetStdHandle failed: %w", err)
	}

	setConsoleModeProc, err := kernel32.FindProc("SetConsoleMode")
	if err != nil {
		return fmt.Errorf("FindProc SetConsoleMode failed: %w", err)
	}

	getConsoleModeProc, err := kernel32.FindProc("GetConsoleMode")
	if err != nil {
		return fmt.Errorf("FindProc GetConsoleMode failed: %w", err)
	}

	const STD_OUTPUT_HANDLE = ^uintptr(11) + 1 // -11 cast to uintptr correctly
	const ENABLE_VIRTUAL_TERMINAL_PROCESSING uint32 = 0x0004

	// Get the handle for stdout
	hConsole, _, err := getStdHandle.Call(uintptr(STD_OUTPUT_HANDLE))
	// Check error explicitly. On non-windows err will likely be non-nil.
	if hConsole == 0 || hConsole == uintptr(syscall.InvalidHandle) || err != nil && !strings.Contains(err.Error(), "The operation completed successfully.") {
		// Return error here as subsequent calls will fail. Include the original error.
		// The "operation completed successfully" check is needed on some Windows versions where Call returns non-nil error despite success.
		return fmt.Errorf("GetStdHandle failed: %w (err: %v)", syscall.GetLastError(), err)
	}

	// Get the current console mode
	var originalMode uint32
	ret, _, err := getConsoleModeProc.Call(hConsole, uintptr(unsafe.Pointer(&originalMode)))
	// Check error explicitly
	if ret == 0 {
		return fmt.Errorf("GetConsoleMode failed: %w (err: %v)", syscall.GetLastError(), err)
	}

	// Enable virtual terminal processing if not already enabled
	if originalMode&ENABLE_VIRTUAL_TERMINAL_PROCESSING == 0 {
		newMode := originalMode | ENABLE_VIRTUAL_TERMINAL_PROCESSING
		ret, _, err = setConsoleModeProc.Call(hConsole, uintptr(newMode))
		// Check error explicitly
		if ret == 0 {
			// Don't return fatal error if setting VT mode fails, just means it won't work
			// fmt.Printf("Warning: SetConsoleMode failed: %v\n", err) // Optional debug print
			return fmt.Errorf("SetConsoleMode failed to enable VT processing: %w (err: %v)", syscall.GetLastError(), err)
		}
	}

	// Also try setting the code page to UTF-8 using chcp
	// Note: This affects the *current process'* console, may not impact SSH client interpretation directly
	// It can be useful if the Go app itself needs to interact with the console using UTF-8
	cmd := exec.Command("chcp", "65001")
	cmd.Stdout = io.Discard // Prevent chcp output from mixing with BBS output
	cmd.Stderr = io.Discard
	cmd.Run() // Run and ignore errors, it's best effort

	return nil // Successfully attempted to set mode
}
