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

func fileListModeDisplay(mode string) string {
	switch strings.ToLower(mode) {
	case "classic":
		return "Classic"
	default:
		return "Lightbar"
	}
}

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
		msg := fmt.Sprintf(e.LoadedStrings.CfgSaveError, fieldName)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	newVal := !original

	stateStr := e.LoadedStrings.CfgToggleOff
	if newVal {
		stateStr = e.LoadedStrings.CfgToggleOn
	}
	msg := fmt.Sprintf(e.LoadedStrings.CfgToggleFormat, fieldName, stateStr)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(500 * time.Millisecond)
	return currentUser, "", nil
}

func runCfgHotKeys(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	return runCfgToggle(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode,
		"Hot Keys",
		func(u *user.User) bool { return u.HotKeys },
		func(u *user.User, v bool) { u.HotKeys = v },
	)
}

func runCfgMorePrompts(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	return runCfgToggle(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode,
		"More Prompts",
		func(u *user.User) bool { return u.MorePrompts },
		func(u *user.User, v bool) { u.MorePrompts = v },
	)
}


func runCfgScreenWidth(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	current := currentUser.ScreenWidth
	if current == 0 {
		current = 80
	}
	prompt := fmt.Sprintf(e.LoadedStrings.CfgScreenWidthPrompt, current)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	input, err := readLineFromSessionIH(s, terminal)
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
		msg := e.LoadedStrings.CfgScreenWidthInvalid
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(500 * time.Millisecond)
		return currentUser, "", nil
	}

	currentUser.ScreenWidth = val
	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save screen width: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	msg := fmt.Sprintf(e.LoadedStrings.CfgScreenWidthSet, val)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(500 * time.Millisecond)
	return currentUser, "", nil
}

func runCfgScreenHeight(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	current := currentUser.ScreenHeight
	if current == 0 {
		current = 25
	}
	prompt := fmt.Sprintf(e.LoadedStrings.CfgScreenHeightPrompt, current)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	input, err := readLineFromSessionIH(s, terminal)
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
		msg := e.LoadedStrings.CfgScreenHeightInvalid
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(500 * time.Millisecond)
		return currentUser, "", nil
	}

	currentUser.ScreenHeight = val
	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save screen height: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	msg := fmt.Sprintf(e.LoadedStrings.CfgScreenHeightSet, val)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(500 * time.Millisecond)
	return currentUser, "", nil
}

func runCfgTermType(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
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

	msg := fmt.Sprintf(e.LoadedStrings.CfgTermTypeSet, strings.ToUpper(currentUser.OutputMode))
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
	prompt := fmt.Sprintf(e.LoadedStrings.CfgStringPrompt, fieldName, maxLen)
	if current != "" {
		prompt = fmt.Sprintf(e.LoadedStrings.CfgStringPromptCurrent, fieldName, current, maxLen)
	}
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	input, err := readLineFromSessionIH(s, terminal)
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

	msg := fmt.Sprintf(e.LoadedStrings.CfgStringUpdated, fieldName)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(500 * time.Millisecond)
	return currentUser, "", nil
}

func runCfgRealName(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	return runCfgStringInput(e, s, terminal, userManager, currentUser, nodeNumber, outputMode,
		"Real Name", 40,
		func(u *user.User) string { return u.RealName },
		func(u *user.User, v string) { u.RealName = v },
	)
}

func runCfgPhone(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	return runCfgStringInput(e, s, terminal, userManager, currentUser, nodeNumber, outputMode,
		"Phone Number", 15,
		func(u *user.User) string { return u.PhoneNumber },
		func(u *user.User, v string) { u.PhoneNumber = v },
	)
}

func runCfgNote(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	return runCfgStringInput(e, s, terminal, userManager, currentUser, nodeNumber, outputMode,
		"User Note", 35,
		func(u *user.User) string { return u.PrivateNote },
		func(u *user.User, v string) { u.PrivateNote = v },
	)
}

func runCfgPassword(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	// Prompt for current password
	msg := e.LoadedStrings.CfgCurrentPwPrompt
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
		msg := e.LoadedStrings.CfgIncorrectPw
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	// Prompt for new password using existing helper
	newPw, err := e.promptForPassword(s, terminal, nodeNumber, outputMode, termWidth, termHeight)
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

	msg = e.LoadedStrings.CfgPasswordChanged
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(1 * time.Second)
	return currentUser, "", nil
}

var colorSlotNames = [7]string{"Prompt", "Input", "Text", "Stat", "Text2", "Stat2", "Bar"}

func runCfgColor(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
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
	palette.WriteString(fmt.Sprintf(e.LoadedStrings.CfgColorSelectPrompt, slotName))
	for i := 0; i < 16; i++ {
		palette.WriteString(fmt.Sprintf("|%02d  %2d  ", i, i))
		if i == 7 {
			palette.WriteString("\r\n")
		}
	}
	palette.WriteString(e.LoadedStrings.CfgColorInputPrompt)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(palette.String())), outputMode)

	input, err := readLineFromSessionIH(s, terminal)
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
		msg := e.LoadedStrings.CfgColorInvalid
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(500 * time.Millisecond)
		return currentUser, "", nil
	}

	currentUser.Colors[slot] = val
	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save color: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	msg := fmt.Sprintf(e.LoadedStrings.CfgColorSet, slotName, val, val)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(500 * time.Millisecond)
	return currentUser, "", nil
}

func runCfgViewConfig(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
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
			return e.LoadedStrings.CfgToggleOn
		}
		return e.LoadedStrings.CfgToggleOff
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
		fmt.Sprintf(e.LoadedStrings.CfgViewScreenWidth, width),
		fmt.Sprintf(e.LoadedStrings.CfgViewScreenHeight, height),
		fmt.Sprintf(e.LoadedStrings.CfgViewTermType, strings.ToUpper(outMode)),
fmt.Sprintf(e.LoadedStrings.CfgViewHotKeys, boolStr(currentUser.HotKeys)),
		fmt.Sprintf(e.LoadedStrings.CfgViewMorePrompts, boolStr(currentUser.MorePrompts)),
		fmt.Sprintf(e.LoadedStrings.CfgViewFileListMode, fileListModeDisplay(currentUser.FileListingMode)),
		fmt.Sprintf(e.LoadedStrings.CfgViewMsgHeader, currentUser.MsgHdr),
		fmt.Sprintf(e.LoadedStrings.CfgViewCustomPrompt, currentUser.CustomPrompt),
		"",
		fmt.Sprintf(e.LoadedStrings.CfgViewPromptColor, currentUser.Colors[0], currentUser.Colors[0], currentUser.Colors[1], currentUser.Colors[1]),
		fmt.Sprintf(e.LoadedStrings.CfgViewTextColor, currentUser.Colors[2], currentUser.Colors[2], currentUser.Colors[3], currentUser.Colors[3]),
		fmt.Sprintf(e.LoadedStrings.CfgViewText2Color, currentUser.Colors[4], currentUser.Colors[4], currentUser.Colors[5], currentUser.Colors[5]),
		fmt.Sprintf(e.LoadedStrings.CfgViewBarColor, currentUser.Colors[6], currentUser.Colors[6]),
		"",
		fmt.Sprintf(e.LoadedStrings.CfgViewRealName, currentUser.RealName),
		fmt.Sprintf(e.LoadedStrings.CfgViewPhone, currentUser.PhoneNumber),
		fmt.Sprintf(e.LoadedStrings.CfgViewNote, currentUser.PrivateNote),
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
	if err := writeCenteredPausePrompt(s, terminal, pausePrompt, outputMode, termWidth, termHeight); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
	}

	return currentUser, "", nil
}

func runCfgFileListMode(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	originalMode := currentUser.FileListingMode
	if strings.ToLower(originalMode) != "classic" {
		currentUser.FileListingMode = "classic"
	} else {
		currentUser.FileListingMode = "lightbar"
	}

	if err := userManager.UpdateUser(currentUser); err != nil {
		currentUser.FileListingMode = originalMode
		log.Printf("ERROR: Node %d: Failed to save file listing mode: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	msg := fmt.Sprintf(e.LoadedStrings.CfgFileListModeSet, fileListModeDisplay(currentUser.FileListingMode))
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
	time.Sleep(500 * time.Millisecond)
	return currentUser, "", nil
}

func runCfgCustomPrompt(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	help := e.LoadedStrings.CfgCustomPromptHelp
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(help)), outputMode)

	return runCfgStringInput(e, s, terminal, userManager, currentUser, nodeNumber, outputMode,
		"Custom Prompt", 80,
		func(u *user.User) string { return u.CustomPrompt },
		func(u *user.User, v string) { u.CustomPrompt = v },
	)
}
