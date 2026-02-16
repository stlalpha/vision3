package menu

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
)

// runCfgToggle is a generic toggle handler for boolean user preferences.
func runCfgToggle(
	e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User,
	nodeNumber int, sessionStartTime time.Time, args string,
	outputMode ansi.OutputMode,
	fieldName string,
	getter func(*user.User) bool,
	setter func(*user.User, bool),
) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	newVal := !getter(currentUser)
	setter(currentUser, newVal)

	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save %s: %v", nodeNumber, fieldName, err)
		msg := fmt.Sprintf("\r\n|09Error saving %s.|07\r\n", fieldName)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	stateStr := "|09OFF|07"
	if newVal {
		stateStr = "|10ON|07"
	}
	msg := fmt.Sprintf("\r\n|07%s: %s\r\n", fieldName, stateStr)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(500 * time.Millisecond)
	return currentUser, "", nil
}

func runCfgHotKeys(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	return runCfgToggle(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode,
		"Hot Keys",
		func(u *user.User) bool { return u.HotKeys },
		func(u *user.User, v bool) { u.HotKeys = v },
	)
}

func runCfgMorePrompts(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	return runCfgToggle(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode,
		"More Prompts",
		func(u *user.User) bool { return u.MorePrompts },
		func(u *user.User, v bool) { u.MorePrompts = v },
	)
}

func runCfgFSEditor(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	return runCfgToggle(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode,
		"Full-Screen Editor",
		func(u *user.User) bool { return u.FullScreenEditor },
		func(u *user.User, v bool) { u.FullScreenEditor = v },
	)
}

func runCfgScreenWidth(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	current := currentUser.ScreenWidth
	if current == 0 {
		current = 80
	}
	prompt := fmt.Sprintf("\r\n|07Screen Width |08(|07current: %d, 40-255|08)|07: ", current)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	input, err := terminal.ReadLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return currentUser, "", nil
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return currentUser, "", nil
	}

	val, parseErr := strconv.Atoi(input)
	if parseErr != nil || val < 40 || val > 255 {
		msg := "\r\n|09Invalid width. Must be 40-255.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(500 * time.Millisecond)
		return currentUser, "", nil
	}

	currentUser.ScreenWidth = val
	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save screen width: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	msg := fmt.Sprintf("\r\n|07Screen Width set to |15%d|07.\r\n", val)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(500 * time.Millisecond)
	return currentUser, "", nil
}

func runCfgScreenHeight(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	current := currentUser.ScreenHeight
	if current == 0 {
		current = 25
	}
	prompt := fmt.Sprintf("\r\n|07Screen Height |08(|07current: %d, 21-60|08)|07: ", current)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	input, err := terminal.ReadLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return currentUser, "", nil
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return currentUser, "", nil
	}

	val, parseErr := strconv.Atoi(input)
	if parseErr != nil || val < 21 || val > 60 {
		msg := "\r\n|09Invalid height. Must be 21-60.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(500 * time.Millisecond)
		return currentUser, "", nil
	}

	currentUser.ScreenHeight = val
	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save screen height: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	msg := fmt.Sprintf("\r\n|07Screen Height set to |15%d|07.\r\n", val)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(500 * time.Millisecond)
	return currentUser, "", nil
}

func runCfgTermType(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	current := currentUser.OutputMode
	if current == "" {
		current = "cp437"
	}

	if current == "ansi" {
		currentUser.OutputMode = "cp437"
	} else {
		currentUser.OutputMode = "ansi"
	}

	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save terminal type: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	msg := fmt.Sprintf("\r\n|07Terminal Type: |15%s|07\r\n", strings.ToUpper(currentUser.OutputMode))
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(500 * time.Millisecond)
	return currentUser, "", nil
}
