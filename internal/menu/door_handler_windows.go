//go:build windows

package menu

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// DoorCtx is the actual context used throughout door_handler.go.
type DoorCtx struct {
	Executor         *MenuExecutor
	Session          ssh.Session
	Terminal         *term.Terminal
	User             doorUserInfo
	NodeNumber       int
	SessionStartTime time.Time
	OutputMode       ansi.OutputMode
	Config           config.DoorConfig
	DoorName         string
	// Pre-computed values
	NodeNumStr  string
	PortStr     string
	TimeLeftMin int
	TimeLeftStr string
	BaudStr     string
	UserIDStr   string
	Subs        map[string]string
}

// doorUserInfo holds the user fields needed for dropfile generation.
type doorUserInfo struct {
	ID            int
	Handle        string
	RealName      string
	AccessLevel   int
	TimeLimit     int
	TimesCalled   int
	PhoneNumber   string
	GroupLocation string
	ScreenWidth   int
	ScreenHeight  int
}

var errDoorsNotSupported = errors.New("doors are not supported on Windows")

// buildDoorCtx creates a DoorCtx (stub on Windows).
func buildDoorCtx(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userID int, handle, realName string, accessLevel, timeLimit, timesCalled int,
	phoneNumber, groupLocation string, screenWidth, screenHeight int,
	nodeNumber int, sessionStartTime time.Time, outputMode ansi.OutputMode,
	doorConfig config.DoorConfig, doorName string) *DoorCtx {
	return &DoorCtx{
		Executor:         e,
		Session:          s,
		Terminal:         terminal,
		NodeNumber:       nodeNumber,
		SessionStartTime: sessionStartTime,
		OutputMode:       outputMode,
		Config:           doorConfig,
		DoorName:         doorName,
	}
}

// executeDoor is not supported on Windows.
func executeDoor(ctx *DoorCtx) error {
	return errDoorsNotSupported
}

// doorErrorMessage writes an error to the terminal.
func doorErrorMessage(ctx *DoorCtx, msg string) {
	terminalio.WriteProcessedBytes(ctx.Terminal,
		ansi.ReplacePipeCodes([]byte(fmt.Sprintf("|12%s|07\r\n", msg))),
		ctx.OutputMode)
}

// runListDoors displays a message that doors are not supported on Windows.
func runListDoors(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Doors are not supported on Windows", nodeNumber)
	terminalio.WriteProcessedBytes(terminal,
		ansi.ReplacePipeCodes([]byte("|12Doors are not supported on Windows.|07\r\n")),
		outputMode)
	time.Sleep(1 * time.Second)
	return currentUser, "", nil
}

// runOpenDoor is not supported on Windows.
func runOpenDoor(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Doors are not supported on Windows", nodeNumber)
	terminalio.WriteProcessedBytes(terminal,
		ansi.ReplacePipeCodes([]byte("|12Doors are not supported on Windows.|07\r\n")),
		outputMode)
	time.Sleep(1 * time.Second)
	return currentUser, "", nil
}

// runDoorInfo is not supported on Windows.
func runDoorInfo(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Doors are not supported on Windows", nodeNumber)
	terminalio.WriteProcessedBytes(terminal,
		ansi.ReplacePipeCodes([]byte("|12Doors are not supported on Windows.|07\r\n")),
		outputMode)
	time.Sleep(1 * time.Second)
	return currentUser, "", nil
}
