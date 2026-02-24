package transfer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"golang.org/x/term"
)

// readInterrupter is implemented by session adapters that support unblocking
// a pending Read() call (e.g. TelnetSessionAdapter).
type readInterrupter interface {
	SetReadInterrupt(ch <-chan struct{})
}

// RunCommandDirect executes an external command with its stdin/stdout/stderr
// piped directly to the SSH session — no PTY allocated. This is essential for
// binary file-transfer protocols (ZMODEM, YMODEM, XMODEM) where a PTY's line
// discipline would corrupt the data stream.
//
// ctx controls cancellation and timeout: when ctx.Done() fires, the process is
// killed and the function returns ctx.Err(). Pass context.Background() for
// no timeout. Callers should use context.WithTimeout for transfer timeouts.
func RunCommandDirect(ctx context.Context, s ssh.Session, cmd *exec.Cmd) error {
	if ctx == nil {
		ctx = context.Background()
	}
	log.Printf("DEBUG: Starting command '%s' in DIRECT (no-PTY) mode", cmd.Path)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe for '%s': %w", cmd.Path, err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe for '%s': %w", cmd.Path, err)
	}
	// Capture stderr separately — NEVER merge it into stdout.
	// External protocol drivers (e.g. sexyz) write status/progress messages
	// to stderr; merging them into stdout corrupts the binary data stream
	// (ZModem frames, etc.) and causes transfers to fail.
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command '%s': %w", cmd.Path, err)
	}

	inputDone := make(chan struct{})
	outputDone := make(chan struct{})

	// session → command stdin
	go func() {
		defer close(inputDone)
		n, cpErr := io.Copy(stdinPipe, s)
		log.Printf("DEBUG: (%s) direct stdin copy finished. Bytes: %d, Error: %v", cmd.Path, n, cpErr)
		_ = stdinPipe.Close()
	}()

	// command stdout → session
	go func() {
		defer close(outputDone)
		n, cpErr := io.Copy(s, stdoutPipe)
		log.Printf("DEBUG: (%s) direct stdout copy finished. Bytes: %d, Error: %v", cmd.Path, n, cpErr)
	}()

	// Race: ctx cancellation vs normal completion (outputDone then cmd.Wait).
	cmdDone := make(chan error, 1)
	go func() {
		<-outputDone
		cmdDone <- cmd.Wait()
	}()

	var cmdErr error
	select {
	case <-ctx.Done():
		log.Printf("DEBUG: (%s) transfer cancelled or timed out, killing process", cmd.Path)
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = stdinPipe.Close()
		<-cmdDone // Wait for goroutine to finish
		cmdErr = ctx.Err()
	case err := <-cmdDone:
		cmdErr = err
	}
	log.Printf("DEBUG: (%s) command finished (direct). Error: %v", cmd.Path, cmdErr)

	// Log stderr output from the external transfer program.
	if stderrBuf.Len() > 0 {
		for _, line := range strings.Split(strings.TrimSpace(stderrBuf.String()), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				log.Printf("INFO: [%s stderr] %s", filepath.Base(cmd.Path), line)
			}
		}
	}

	// Close stdin pipe so the next write from the stdin goroutine will fail.
	_ = stdinPipe.Close()

	// Unblock the stdin goroutine's pending s.Read() call.  SetReadInterrupt
	// causes Read to return io.EOF (telnet) or ErrReadInterrupted (SSH).
	if ri, ok := s.(readInterrupter); ok {
		interruptCh := make(chan struct{})
		close(interruptCh)
		ri.SetReadInterrupt(interruptCh)
	}

	// Wait briefly for the stdin goroutine to notice the closed pipe / interrupt.
	select {
	case <-inputDone:
		log.Printf("DEBUG: (%s) stdin goroutine finished cleanly", cmd.Path)
	case <-time.After(2 * time.Second):
		log.Printf("WARN: (%s) stdin goroutine did not finish within 2s, proceeding", cmd.Path)
	}

	// CRITICAL: Clear the read interrupt and reset the connection deadline
	// BEFORE returning.  The interrupt causes TelnetConn.Read() to return
	// io.EOF.  If the interrupt is not cleared, the next InputHandler that
	// calls s.Read() will immediately get io.EOF and report "user disconnected".
	if ri, ok := s.(readInterrupter); ok {
		ri.SetReadInterrupt(nil)
	}

	// Brief pause to let the client's terminal finish its post-transfer
	// cleanup (SyncTerm/ZModem end-of-transfer signaling).
	time.Sleep(250 * time.Millisecond)

	// Drain any leftover protocol bytes (ZModem ZFIN/OO, ACK frames) from
	// the session so they don't appear as garbage when the BBS resumes.
	drainSessionInput(s, 500*time.Millisecond)

	return cmdErr
}

// drainSessionInput reads and discards any pending bytes from the session
// for the given duration.  Uses a simple non-blocking polling loop that does
// NOT use SetReadInterrupt — avoiding goroutine races that can leave stale
// deadlines on the connection and cause spurious disconnects.
func drainSessionInput(s ssh.Session, duration time.Duration) {
	buf := make([]byte, 1024)
	totalDrained := 0
	end := time.Now().Add(duration)

	for time.Now().Before(end) {
		// Use a short-lived read interrupt for each poll cycle so we
		// don't block the entire duration if no data is available.
		if ri, ok := s.(readInterrupter); ok {
			ch := make(chan struct{})
			time.AfterFunc(50*time.Millisecond, func() { close(ch) })
			ri.SetReadInterrupt(ch)
		}

		n, readErr := s.Read(buf)
		totalDrained += n

		// Clear the interrupt immediately after each read attempt
		// so no stale deadline lingers.
		if ri, ok := s.(readInterrupter); ok {
			ri.SetReadInterrupt(nil)
		}

		if readErr != nil || n == 0 {
			// No more data or error — stop draining.
			break
		}
	}

	if totalDrained > 0 {
		log.Printf("DEBUG: Drained %d leftover bytes from session after transfer", totalDrained)
	}
}

// RunCommandWithPTY executes an external command attached to the user's SSH
// session using a PTY. It handles setting raw mode, resizing, and copying I/O.
//
// ctx controls cancellation and timeout: when ctx.Done() fires, the process is
// killed and the function returns ctx.Err(). Pass context.Background() for
// no timeout.
func RunCommandWithPTY(ctx context.Context, s ssh.Session, cmd *exec.Cmd) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ptyReq, winCh, isPty := s.Pty()
	if !isPty {
		log.Printf("WARN: No PTY available for session. Falling back to direct mode for %s.", cmd.Path)
		return RunCommandDirect(ctx, s, cmd)
	}

	log.Printf("DEBUG: Starting command '%s' with PTY", cmd.Path)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start pty for command '%s': %w", cmd.Path, err)
	}
	// ptmx is closed explicitly during shutdown sequence below

	// Handle window resizing.
	go func() {
		if ptyReq.Window.Width > 0 || ptyReq.Window.Height > 0 {
			wErr := pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(ptyReq.Window.Height), Cols: uint16(ptyReq.Window.Width)})
			if wErr != nil {
				log.Printf("WARN: Failed to set initial pty size: %v", wErr)
			}
		}
		for win := range winCh {
			wErr := pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(win.Height), Cols: uint16(win.Width)})
			if wErr != nil {
				log.Printf("WARN: Failed to resize pty: %v", wErr)
			}
		}
	}()

	// Set PTY to raw mode so binary protocol data passes through unmodified.
	fd := int(ptmx.Fd())
	var restoreTerminal func() = func() {}
	originalState, err := term.MakeRaw(fd)
	if err != nil {
		log.Printf("WARN: Failed to put PTY (fd: %d) into raw mode for command '%s': %v.", fd, cmd.Path, err)
	} else {
		restoreTerminal = func() {
			if err := term.Restore(fd, originalState); err != nil {
				log.Printf("ERROR: Failed to restore terminal state (fd: %d) after command '%s': %v", fd, cmd.Path, err)
			}
		}
	}

	// --- SetReadInterrupt for clean shutdown ---
	readInterrupt := make(chan struct{})
	hasInterrupt := false
	if ri, ok := s.(interface{ SetReadInterrupt(<-chan struct{}) }); ok {
		ri.SetReadInterrupt(readInterrupt)
		defer ri.SetReadInterrupt(nil)
		hasInterrupt = true
	}

	// --- I/O Copying ---
	inputDone := make(chan struct{})
	outputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		log.Printf("DEBUG: (%s) Goroutine: Copying session stdin -> PTY starting...", cmd.Path)
		n, err := io.Copy(ptmx, s)
		log.Printf("DEBUG: (%s) Goroutine: Copying session stdin -> PTY finished. Bytes: %d, Error: %v", cmd.Path, n, err)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) && !errors.Is(err, syscall.EIO) && !errors.Is(err, syscall.EINTR) {
			log.Printf("WARN: (%s) Error copying session stdin to PTY: %v", cmd.Path, err)
		}
	}()
	go func() {
		defer close(outputDone)
		log.Printf("DEBUG: (%s) Goroutine: Copying PTY stdout -> session starting...", cmd.Path)
		n, err := io.Copy(s, ptmx)
		log.Printf("DEBUG: (%s) Goroutine: Copying PTY stdout -> session finished. Bytes: %d, Error: %v", cmd.Path, n, err)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) && !errors.Is(err, syscall.EIO) {
			log.Printf("WARN: (%s) Error copying PTY stdout to session stdout: %v", cmd.Path, err)
		}
	}()

	// Race: ctx cancellation vs normal completion
	log.Printf("DEBUG: (%s) Waiting for command completion...", cmd.Path)
	cmdDoneCh := make(chan error, 1)
	go func() { cmdDoneCh <- cmd.Wait() }()

	var cmdErr error
	select {
	case <-ctx.Done():
		log.Printf("DEBUG: (%s) transfer cancelled or timed out, killing process", cmd.Path)
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		cmdErr = ctx.Err()
		<-cmdDoneCh
	case cmdErr = <-cmdDoneCh:
	}
	log.Printf("DEBUG: (%s) Command finished. Error: %v", cmd.Path, cmdErr)

	// Interrupt the input goroutine's blocked Read() so it exits without
	// consuming the user's next keypress
	close(readInterrupt)
	if hasInterrupt {
		<-inputDone
	}

	// Restore terminal before closing PTY
	restoreTerminal()

	// Close PTY and wait for both goroutines
	_ = ptmx.Close()
	if !hasInterrupt {
		<-inputDone // Wait for input goroutine even without interrupt support
	}
	<-outputDone

	return cmdErr
}

// ExecuteZmodemSend initiates a Zmodem send (sz) of one or more files using a PTY.
// It requires the 'sz' command to be available on the system path.
// filePaths should be absolute paths to the files being sent.
// ctx controls cancellation and timeout; pass nil for no timeout.
func ExecuteZmodemSend(ctx context.Context, s ssh.Session, filePaths ...string) error {
	log.Printf("DEBUG: executeZmodemSend called with files: %v", filePaths)

	if len(filePaths) == 0 {
		return fmt.Errorf("no files provided for Zmodem send")
	}

	// Check if sz command exists
	szPath, err := exec.LookPath("sz")
	if err != nil {
		log.Printf("ERROR: 'sz' command not found in PATH: %v", err)
		return fmt.Errorf("'sz' command not found, Zmodem send unavailable")
	}
	log.Printf("DEBUG: Found 'sz' command at: %s", szPath)

	// Construct command: sz [-b] <files...>
	args := append([]string{"-b"}, filePaths...)
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, szPath, args...)

	log.Printf("INFO: Executing Zmodem send: %s %v", szPath, args)
	// Execute using the PTY helper
	err = RunCommandWithPTY(ctx, s, cmd)
	if err != nil {
		log.Printf("ERROR: Zmodem send command ('%s') failed: %v", szPath, err)
		return fmt.Errorf("Zmodem send failed: %w", err)
	}

	log.Printf("INFO: Zmodem send completed successfully for files: %v", filePaths)
	return nil
}

// ExecuteZmodemReceive initiates a Zmodem receive (rz) into a specified directory using a PTY.
// It requires the 'rz' command to be available on the system path.
// targetDir should be the absolute path to the directory where received files will be stored.
// ctx controls cancellation and timeout; pass nil for no timeout.
func ExecuteZmodemReceive(ctx context.Context, s ssh.Session, targetDir string) error {
	log.Printf("DEBUG: executeZmodemReceive called for target directory: %s", targetDir)

	// 1. Validate and ensure target directory exists
	if targetDir == "" {
		return fmt.Errorf("target directory cannot be empty for Zmodem receive")
	}
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for target directory '%s': %w", targetDir, err)
	}
	if err := os.MkdirAll(absTargetDir, 0755); err != nil {
		if fileInfo, statErr := os.Stat(absTargetDir); !(statErr == nil && fileInfo.IsDir()) {
			return fmt.Errorf("failed to create or access target directory '%s': %w", absTargetDir, err)
		}
	}

	// 2. Check if rz command exists
	rzPath, err := exec.LookPath("rz")
	if err != nil {
		log.Printf("ERROR: 'rz' command not found in PATH: %v", err)
		return fmt.Errorf("'rz' command not found, Zmodem receive unavailable")
	}
	log.Printf("DEBUG: Found 'rz' command at: %s", rzPath)

	// 3. Construct command: rz -b -r
	args := []string{"-b", "-r"} // Binary mode, Restricted mode (prevents path traversal)
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, rzPath, args...)
	cmd.Dir = absTargetDir // Run rz in the target directory

	log.Printf("INFO: Executing Zmodem receive in directory '%s': %s %v", absTargetDir, rzPath, args)
	// 4. Execute using the PTY helper
	err = RunCommandWithPTY(ctx, s, cmd)
	if err != nil {
		log.Printf("ERROR: Zmodem receive command ('%s') failed: %v", rzPath, err)
		return fmt.Errorf("Zmodem receive failed: %w", err)
	}

	log.Printf("INFO: Zmodem receive completed successfully into directory: %s", absTargetDir)
	return nil
}
