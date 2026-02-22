package transfer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"golang.org/x/term"
)

// RunCommandDirect executes an external command with its stdin/stdout/stderr
// piped directly to the SSH session — no PTY allocated. This is essential for
// binary file-transfer protocols (ZMODEM, YMODEM, XMODEM) where a PTY's line
// discipline would corrupt the data stream.
func RunCommandDirect(s ssh.Session, cmd *exec.Cmd) error {
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

	// Wait for the command to exit
	cmdErr := cmd.Wait()
	log.Printf("DEBUG: (%s) command finished (direct). Error: %v", cmd.Path, cmdErr)

	// Log stderr output from the external transfer program (status/progress
	// messages that must NOT be sent to the client).
	if stderrBuf.Len() > 0 {
		for _, line := range strings.Split(strings.TrimSpace(stderrBuf.String()), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				log.Printf("INFO: [%s stderr] %s", filepath.Base(cmd.Path), line)
			}
		}
	}

	// Close stdin pipe to unblock the input goroutine if it's still reading
	_ = stdinPipe.Close()

	<-outputDone
	// Don't block forever on inputDone — the session read may hang until the
	// client sends more data.  The stdinPipe.Close() above will cause the
	// io.Copy to return on the next read attempt.
	<-inputDone

	return cmdErr
}

// RunCommandWithPTY executes an external command attached to the user's SSH
// session using a PTY. It handles setting raw mode, resizing, and copying I/O.
func RunCommandWithPTY(s ssh.Session, cmd *exec.Cmd) error {
	ptyReq, winCh, isPty := s.Pty()
	if !isPty {
		log.Printf("WARN: No PTY available for session. Falling back to direct mode for %s.", cmd.Path)
		return RunCommandDirect(s, cmd)
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

	// Wait for command to complete, then clean shutdown
	log.Printf("DEBUG: (%s) Waiting for command completion...", cmd.Path)
	cmdErr := cmd.Wait()
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

// executeZmodemSend initiates a Zmodem send (sz) of one or more files using a PTY.
// It requires the 'sz' command to be available on the system path.
// filePaths should be absolute paths to the files being sent.
func ExecuteZmodemSend(s ssh.Session, filePaths ...string) error {
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
	cmd := exec.Command(szPath, args...)

	log.Printf("INFO: Executing Zmodem send: %s %v", szPath, args)

	// Execute using the PTY helper
	err = RunCommandWithPTY(s, cmd)
	if err != nil {
		log.Printf("ERROR: Zmodem send command ('%s') failed: %v", szPath, err)
		return fmt.Errorf("Zmodem send failed: %w", err)
	}

	log.Printf("INFO: Zmodem send completed successfully for files: %v", filePaths)
	return nil
}

// executeZmodemReceive initiates a Zmodem receive (rz) into a specified directory using a PTY.
// It requires the 'rz' command to be available on the system path.
// targetDir should be the absolute path to the directory where received files will be stored.
func ExecuteZmodemReceive(s ssh.Session, targetDir string) error {
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
	cmd := exec.Command(rzPath, args...)
	cmd.Dir = absTargetDir // Run rz in the target directory

	log.Printf("INFO: Executing Zmodem receive in directory '%s': %s %v", absTargetDir, rzPath, args)

	// 4. Execute using the PTY helper
	err = RunCommandWithPTY(s, cmd)
	if err != nil {
		log.Printf("ERROR: Zmodem receive command ('%s') failed: %v", rzPath, err)
		return fmt.Errorf("Zmodem receive failed: %w", err)
	}

	log.Printf("INFO: Zmodem receive completed successfully into directory: %s", absTargetDir)
	return nil
}
