package menu

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"golang.org/x/crypto/bcrypt"
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

	original := getter(currentUser)
	setter(currentUser, !original)

	if err := userManager.UpdateUser(currentUser); err != nil {
		setter(currentUser, original)
		log.Printf("ERROR: Node %d: Failed to save %s: %v", nodeNumber, fieldName, err)
		msg := fmt.Sprintf("\r\n|09Error saving %s.|07\r\n", fieldName)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	newVal := !original

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

	msg := fmt.Sprintf("\r\n|07Terminal Type: |15%s|07 |08(takes effect next session)|07\r\n", strings.ToUpper(currentUser.OutputMode))
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(500 * time.Millisecond)
	return currentUser, "", nil
}

// runCfgStringInput is a generic string input handler for user preferences.
func runCfgStringInput(
	e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User,
	nodeNumber int, outputMode ansi.OutputMode,
	fieldName string, maxLen int,
	getter func(*user.User) string,
	setter func(*user.User, string),
) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	current := getter(currentUser)
	prompt := fmt.Sprintf("\r\n|07%s |08(|07max %d chars|08)|07: ", fieldName, maxLen)
	if current != "" {
		prompt = fmt.Sprintf("\r\n|07%s |08[|07%s|08] (|07max %d|08)|07: ", fieldName, current, maxLen)
	}
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

	if len(input) > maxLen {
		input = input[:maxLen]
	}

	setter(currentUser, input)
	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save %s: %v", nodeNumber, fieldName, err)
		return currentUser, "", nil
	}

	msg := fmt.Sprintf("\r\n|07%s updated.|07\r\n", fieldName)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(500 * time.Millisecond)
	return currentUser, "", nil
}

func runCfgRealName(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	return runCfgStringInput(e, s, terminal, userManager, currentUser, nodeNumber, outputMode,
		"Real Name", 40,
		func(u *user.User) string { return u.RealName },
		func(u *user.User, v string) { u.RealName = v },
	)
}

func runCfgPhone(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	return runCfgStringInput(e, s, terminal, userManager, currentUser, nodeNumber, outputMode,
		"Phone Number", 15,
		func(u *user.User) string { return u.PhoneNumber },
		func(u *user.User, v string) { u.PhoneNumber = v },
	)
}

func runCfgNote(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	return runCfgStringInput(e, s, terminal, userManager, currentUser, nodeNumber, outputMode,
		"User Note", 35,
		func(u *user.User) string { return u.PrivateNote },
		func(u *user.User, v string) { u.PrivateNote = v },
	)
}

func runCfgPassword(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	// Prompt for current password
	msg := "\r\n|07Current Password: "
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)

	oldPw, err := readPasswordSecurely(s, terminal, outputMode)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return currentUser, "", nil
	}

	// Verify current password
	if bcryptErr := bcrypt.CompareHashAndPassword([]byte(currentUser.PasswordHash), []byte(oldPw)); bcryptErr != nil {
		msg := "\r\n|09Incorrect password.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	// Prompt for new password using existing helper
	newPw, err := e.promptForPassword(s, terminal, nodeNumber, outputMode)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return currentUser, "", nil
	}
	if newPw == "" {
		return currentUser, "", nil
	}

	// Hash and save
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPw), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to hash new password: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	currentUser.PasswordHash = string(hashed)
	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save password: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	msg = "\r\n|10Password changed.|07\r\n"
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(1 * time.Second)
	return currentUser, "", nil
}

var colorSlotNames = [7]string{"Prompt", "Input", "Text", "Stat", "Text2", "Stat2", "Bar"}

func runCfgColor(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	slot := 0
	if trimmed := strings.TrimSpace(args); trimmed != "" {
		if parsed, err := strconv.Atoi(trimmed); err == nil && parsed >= 0 && parsed < 7 {
			slot = parsed
		}
	}

	slotName := colorSlotNames[slot]

	// Display color palette
	var palette strings.Builder
	palette.WriteString(fmt.Sprintf("\r\n|07Select %s Color:\r\n\r\n", slotName))
	for i := 0; i < 16; i++ {
		palette.WriteString(fmt.Sprintf("|%02d  %2d  ", i, i))
		if i == 7 {
			palette.WriteString("\r\n")
		}
	}
	palette.WriteString("|07\r\n\r\nColor number (0-15): ")
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(palette.String())), outputMode)

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
	if parseErr != nil || val < 0 || val > 15 {
		msg := "\r\n|09Invalid color. Must be 0-15.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(500 * time.Millisecond)
		return currentUser, "", nil
	}

	currentUser.Colors[slot] = val
	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save color: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	msg := fmt.Sprintf("\r\n|07%s Color set to |%02d%d|07.\r\n", slotName, val, val)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(500 * time.Millisecond)
	return currentUser, "", nil
}

func runCfgViewConfig(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	topPath := filepath.Join(e.MenuSetPath, "templates", "USRCFGV.TOP")
	botPath := filepath.Join(e.MenuSetPath, "templates", "USRCFGV.BOT")

	topBytes, err := os.ReadFile(topPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("WARN: Node %d: Failed to read %s: %v", nodeNumber, topPath, err)
	}
	botBytes, err := os.ReadFile(botPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("WARN: Node %d: Failed to read %s: %v", nodeNumber, botPath, err)
	}

	topBytes = stripSauceMetadata(topBytes)
	botBytes = stripSauceMetadata(botBytes)
	topBytes = normalizePipeCodeDelimiters(topBytes)
	botBytes = normalizePipeCodeDelimiters(botBytes)

	var buf bytes.Buffer

	if len(topBytes) > 0 {
		buf.Write(ansi.ReplacePipeCodes(topBytes))
		buf.WriteString("\r\n")
	}

	boolStr := func(v bool) string {
		if v {
			return "|10ON|07"
		}
		return "|09OFF|07"
	}

	width := currentUser.ScreenWidth
	if width == 0 {
		width = 80
	}
	height := currentUser.ScreenHeight
	if height == 0 {
		height = 25
	}
	outMode := currentUser.OutputMode
	if outMode == "" {
		outMode = "cp437"
	}

	lines := []string{
		fmt.Sprintf(" |07Screen Width:       |15%d", width),
		fmt.Sprintf(" |07Screen Height:      |15%d", height),
		fmt.Sprintf(" |07Terminal Type:       |15%s", strings.ToUpper(outMode)),
		fmt.Sprintf(" |07Full-Screen Editor:  %s", boolStr(currentUser.FullScreenEditor)),
		fmt.Sprintf(" |07Hot Keys:            %s", boolStr(currentUser.HotKeys)),
		fmt.Sprintf(" |07More Prompts:        %s", boolStr(currentUser.MorePrompts)),
		fmt.Sprintf(" |07Message Header:      |15%d", currentUser.MsgHdr),
		fmt.Sprintf(" |07Custom Prompt:       |15%s", currentUser.CustomPrompt),
		"",
		fmt.Sprintf(" |07Prompt Color:  |%02d%d|07    Input Color: |%02d%d|07", currentUser.Colors[0], currentUser.Colors[0], currentUser.Colors[1], currentUser.Colors[1]),
		fmt.Sprintf(" |07Text Color:    |%02d%d|07    Stat Color:  |%02d%d|07", currentUser.Colors[2], currentUser.Colors[2], currentUser.Colors[3], currentUser.Colors[3]),
		fmt.Sprintf(" |07Text2 Color:   |%02d%d|07    Stat2 Color: |%02d%d|07", currentUser.Colors[4], currentUser.Colors[4], currentUser.Colors[5], currentUser.Colors[5]),
		fmt.Sprintf(" |07Bar Color:     |%02d%d|07", currentUser.Colors[6], currentUser.Colors[6]),
		"",
		fmt.Sprintf(" |07Real Name:     |15%s", currentUser.RealName),
		fmt.Sprintf(" |07Phone:         |15%s", currentUser.PhoneNumber),
		fmt.Sprintf(" |07Note:          |15%s", currentUser.PrivateNote),
	}

	for _, line := range lines {
		buf.Write(ansi.ReplacePipeCodes([]byte(line)))
		buf.WriteString("\r\n")
	}

	if len(botBytes) > 0 {
		buf.Write(ansi.ReplacePipeCodes(botBytes))
		buf.WriteString("\r\n")
	}

	terminalio.WriteProcessedBytes(terminal, buf.Bytes(), outputMode)

	// Pause
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... "
	}
	if err := writeCenteredPausePrompt(s, terminal, pausePrompt, outputMode); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
	}

	return currentUser, "", nil
}

func runCfgCustomPrompt(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	help := "\r\n|08Format codes: |07|MN|08=Menu |07|TL|08=Time Left |07|TN|08=Time |07|DN|08=Date |07|CR|08=Return\r\n"
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(help)), outputMode)

	return runCfgStringInput(e, s, terminal, userManager, currentUser, nodeNumber, outputMode,
		"Custom Prompt", 80,
		func(u *user.User) string { return u.CustomPrompt },
		func(u *user.User, v string) { u.CustomPrompt = v },
	)
}
