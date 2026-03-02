//go:build windows

package menu

import (
	"fmt"
	"log"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// executeDoor dispatches to the appropriate executor.
// DOS doors use DOSBox-X (cross-platform). Native POSIX doors are not supported on Windows.
func executeDoor(ctx *DoorCtx) error {
	if ctx.Config.IsDOS {
		return executeDOSBoxDoor(ctx)
	}
	return fmt.Errorf("native doors are not supported on Windows")
}

// doorErrorMessage writes a formatted error message to the terminal.
func doorErrorMessage(ctx *DoorCtx, msg string) {
	terminalio.WriteProcessedBytes(ctx.Terminal,
		ansi.ReplacePipeCodes([]byte(fmt.Sprintf("|12%s|07\r\n", msg))),
		ctx.OutputMode)
}

// runListDoors lists configured DOS doors. Native doors are not supported on Windows.
func runListDoors(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: runListDoors (Windows)", nodeNumber)
	terminalio.WriteProcessedBytes(terminal,
		ansi.ReplacePipeCodes([]byte("|12Native doors are not supported on Windows. DOS doors via DOSBox-X are available.|07\r\n")),
		outputMode)
	time.Sleep(1 * time.Second)
	return currentUser, "", nil
}

// runOpenDoor prompts for a door name and launches it via DOSBox-X on Windows.
func runOpenDoor(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: runOpenDoor (Windows)", nodeNumber)
	terminalio.WriteProcessedBytes(terminal,
		ansi.ReplacePipeCodes([]byte("|12Native doors are not supported on Windows. DOS doors via DOSBox-X are available.|07\r\n")),
		outputMode)
	time.Sleep(1 * time.Second)
	return currentUser, "", nil
}

// runDoorInfo shows door configuration details on Windows.
func runDoorInfo(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: runDoorInfo (Windows)", nodeNumber)
	terminalio.WriteProcessedBytes(terminal,
		ansi.ReplacePipeCodes([]byte("|12Native doors are not supported on Windows. DOS doors via DOSBox-X are available.|07\r\n")),
		outputMode)
	time.Sleep(1 * time.Second)
	return currentUser, "", nil
}
