package transfer

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"golang.org/x/term"
)

// RunCommandWithPTY executes an external command attached to the user's SSH session using a PTY.
// This is needed for protocols that require raw terminal access (like ZMODEM).
func RunCommandWithPTY(s ssh.Session, cmd *exec.Cmd) error {
	ptyReq, winCh, isPty := s.Pty()
	if !isPty {
		// Fallback to direct execution without PTY
		log.Printf("WARN: No PTY available for session. Running command %s directly. Transfer might fail.", cmd.Path)
		cmd.Stdin = s
		cmd.Stdout = s
		cmd.Stderr = s
		return cmd.Run()
	}

	log.Printf("DEBUG: Starting command '%s' with PTY", cmd.Path)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start pty for command '%s': %w", cmd.Path, err)
	}
	defer func() { _ = ptmx.Close() }()

	// Handle window resizing
	go func() {
		if ptyReq.Window.Width > 0 || ptyReq.Window.Height > 0 {
			_ = pty.Setsize(ptmx, &pty.Winsize{
				Rows: uint16(ptyReq.Window.Height), 
				Cols: uint16(ptyReq.Window.Width),
			})
		}
		for win := range winCh {
			_ = pty.Setsize(ptmx, &pty.Winsize{
				Rows: uint16(win.Height), 
				Cols: uint16(win.Width),
			})
		}
	}()

	// Set terminal to raw mode for binary transfers
	var restoreTerminal func() = func() {}
	if f, ok := s.(interface{ Fd() uintptr }); ok {
		fd := int(f.Fd())
		if originalState, err := term.MakeRaw(fd); err == nil {
			restoreTerminal = func() {
				_ = term.Restore(fd, originalState)
			}
		}
	}
	defer restoreTerminal()

	// Copy I/O between session and PTY
	var wg sync.WaitGroup
	wg.Add(2)
	
	// Session -> PTY
	go func() {
		defer wg.Done()
		_, err := io.Copy(ptmx, s)
		if err != nil && err != io.EOF && 
		   !errors.Is(err, os.ErrClosed) && 
		   !errors.Is(err, syscall.EIO) {
			log.Printf("WARN: Error copying session stdin to PTY: %v", err)
		}
	}()
	
	// PTY -> Session
	go func() {
		defer wg.Done()
		_, err := io.Copy(s, ptmx)
		if err != nil && err != io.EOF && 
		   !errors.Is(err, os.ErrClosed) && 
		   !errors.Is(err, syscall.EIO) {
			log.Printf("WARN: Error copying PTY stdout to session: %v", err)
		}
	}()

	wg.Wait()
	return cmd.Wait()
}