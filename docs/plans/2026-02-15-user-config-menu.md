# User Config Menu Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Full user-facing configuration menu with V2 parity — terminal prefs, colors, personal info, password change, and view config.

**Architecture:** Standard V3 menu (ANSI + CFG + MNU) with a new `internal/menu/user_config.go` file containing all RUN: handlers. New preference fields on the User struct persisted via `userManager.UpdateUser()`. Menu accessed via K key from MAIN.

**Tech Stack:** Go standard library, existing template/pipe-code pipeline, bcrypt for password.

---

### Task 1: Add User Preference Fields

**Files:**
- Modify: `internal/user/user.go:58-61`

**Step 1: Add new fields to User struct**

After the existing terminal preferences block (line 61, after `MsgHdr`), add:

```go
	// User Configuration Preferences
	HotKeys          bool   `json:"hotKeys,omitempty"`          // Single keypress mode
	MorePrompts      bool   `json:"morePrompts,omitempty"`      // Pause at screen end
	FullScreenEditor bool   `json:"fullScreenEditor,omitempty"` // Use full-screen editor
	CustomPrompt     string `json:"customPrompt,omitempty"`     // Custom prompt format string
	OutputMode       string `json:"outputMode,omitempty"`       // "ansi" or "cp437"
	Colors           [7]int `json:"colors,omitempty"`           // Prompt, Input, Text, Stat, Text2, Stat2, Bar
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: Success (JSON tags make fields backward-compatible with existing users.json)

**Step 3: Commit**

```bash
git add internal/user/user.go
git commit -m "feat(user): add configuration preference fields to User struct"
```

---

### Task 2: Create Menu Files (USERCFG.CFG, USERCFG.MNU, USERCFG.ANS)

**Files:**
- Create: `menus/v3/cfg/USERCFG.CFG`
- Create: `menus/v3/mnu/USERCFG.MNU`
- Create: `menus/v3/ansi/USERCFG.ANS`
- Modify: `menus/v3/cfg/MAIN.CFG:57-60`

**Step 1: Create USERCFG.CFG**

```json
[
    {
        "KEYS": "A",
        "CMD": "RUN:CFG_SCREENWIDTH",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "B",
        "CMD": "RUN:CFG_SCREENHEIGHT",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "C",
        "CMD": "RUN:CFG_TERMTYPE",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "D",
        "CMD": "RUN:CFG_FSEDITOR",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "E",
        "CMD": "RUN:CFG_HOTKEYS",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "F",
        "CMD": "RUN:CFG_MOREPROMPTS",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "G",
        "CMD": "RUN:GETHEADERTYPE",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "H",
        "CMD": "RUN:CFG_CUSTOMPROMPT",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "I",
        "CMD": "RUN:CFG_COLOR 0",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "J",
        "CMD": "RUN:CFG_COLOR 1",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "K",
        "CMD": "RUN:CFG_COLOR 2",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "L",
        "CMD": "RUN:CFG_COLOR 3",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "M",
        "CMD": "RUN:CFG_REALNAME",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "N",
        "CMD": "RUN:CFG_PHONE",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "O",
        "CMD": "RUN:CFG_NOTE",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "P",
        "CMD": "RUN:CFG_PASSWORD",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "V",
        "CMD": "RUN:CFG_VIEWCONFIG",
        "ACS": "",
        "HIDDEN": false
    },
    {
        "KEYS": "Q",
        "CMD": "GOTO:MAIN",
        "ACS": "",
        "HIDDEN": false
    }
]
```

**Step 2: Create USERCFG.MNU**

```json
{
  "CLR": true,
  "USEPROMPT": true,
  "PROMPT1": "|08[|07Config|08] |15>|07 ",
  "FALLBACK": "USERCFG",
  "ACS": "",
  "PASS": ""
}
```

**Step 3: Create USERCFG.ANS**

A simple pipe-coded text file (no binary ANSI art needed for now):

```
|12User Configuration|07
|08────────────────────────────────────────────────────────────────────────────────
|15 Terminal                          |15 Personal Info
|08 ──────────                        |08 ─────────────
|07 [|15A|07] Screen Width              |07 [|15M|07] Real Name
|07 [|15B|07] Screen Height             |07 [|15N|07] Phone Number
|07 [|15C|07] Terminal Type             |07 [|15O|07] User Note
|07 [|15D|07] Full-Screen Editor        |07 [|15P|07] Change Password
|07 [|15E|07] Hot Keys
|07 [|15F|07] More Prompts              |15 Colors
|07 [|15G|07] Message Header Style      |08 ──────
                                     |07 [|15I|07] Prompt Color
|15 Display                           |07 [|15J|07] Input Color
|08 ───────                           |07 [|15K|07] Text Color
|07 [|15H|07] Custom Prompt             |07 [|15L|07] Stat Color
|07 [|15V|07] View Current Config

|07 [|15Q|07] Return to Main Menu
|08────────────────────────────────────────────────────────────────────────────────
```

**Step 4: Update MAIN.CFG**

Change line 58 from `"CMD": "GOTO:CONFIGLM"` to `"CMD": "GOTO:USERCFG"`.

**Step 5: Verify build**

Run: `go build ./...`
Expected: Success

**Step 6: Commit**

```bash
git add menus/v3/cfg/USERCFG.CFG menus/v3/mnu/USERCFG.MNU menus/v3/ansi/USERCFG.ANS menus/v3/cfg/MAIN.CFG
git commit -m "feat: add USERCFG menu files and wire K key"
```

---

### Task 3: Implement Toggle Commands

**Files:**
- Create: `internal/menu/user_config.go`
- Modify: `internal/menu/executor.go:297-348` (registerAppRunnables)

**Step 1: Create user_config.go with toggle handlers**

Create `internal/menu/user_config.go`. All user config RUN: handlers go here.

```go
package menu

import (
	"fmt"
	"io"
	"log"
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

	// Toggle the value
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
```

**Step 2: Register in executor.go**

In `registerAppRunnables`, add:

```go
registry["CFG_HOTKEYS"] = runCfgHotKeys
registry["CFG_MOREPROMPTS"] = runCfgMorePrompts
registry["CFG_FSEDITOR"] = runCfgFSEditor
```

**Step 3: Verify build**

Run: `go build ./...`
Expected: Success

**Step 4: Commit**

```bash
git add internal/menu/user_config.go internal/menu/executor.go
git commit -m "feat: add toggle config commands (hot keys, more prompts, fs editor)"
```

---

### Task 4: Implement Numeric Input Commands (Screen Width/Height)

**Files:**
- Modify: `internal/menu/user_config.go`
- Modify: `internal/menu/executor.go` (register)

**Step 1: Add numeric input handlers to user_config.go**

```go
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
```

**Step 2: Register in executor.go**

```go
registry["CFG_SCREENWIDTH"] = runCfgScreenWidth
registry["CFG_SCREENHEIGHT"] = runCfgScreenHeight
```

**Step 3: Add needed imports to user_config.go**

Add `"errors"`, `"strconv"`, `"strings"`, `"io"` to the import block.

**Step 4: Verify build**

Run: `go build ./...`
Expected: Success

**Step 5: Commit**

```bash
git add internal/menu/user_config.go internal/menu/executor.go
git commit -m "feat: add screen width/height config commands"
```

---

### Task 5: Implement Terminal Type Toggle

**Files:**
- Modify: `internal/menu/user_config.go`
- Modify: `internal/menu/executor.go` (register)

**Step 1: Add terminal type handler**

```go
func runCfgTermType(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	current := currentUser.OutputMode
	if current == "" {
		current = "cp437"
	}

	// Toggle between modes
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
```

**Step 2: Register**

```go
registry["CFG_TERMTYPE"] = runCfgTermType
```

**Step 3: Verify build and commit**

```bash
git add internal/menu/user_config.go internal/menu/executor.go
git commit -m "feat: add terminal type toggle config command"
```

---

### Task 6: Implement String Input Commands (Real Name, Phone, Note, Custom Prompt)

**Files:**
- Modify: `internal/menu/user_config.go`
- Modify: `internal/menu/executor.go` (register)

**Step 1: Add generic string input helper and handlers**

```go
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
		return currentUser, "", nil // Keep current value
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

func runCfgCustomPrompt(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	// Show available format codes
	help := "\r\n|08Format codes: |07|MN|08=Menu |07|TL|08=Time Left |07|TN|08=Time |07|DN|08=Date |07|CR|08=Return\r\n"
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(help)), outputMode)

	return runCfgStringInput(e, s, terminal, userManager, currentUser, nodeNumber, outputMode,
		"Custom Prompt", 80,
		func(u *user.User) string { return u.CustomPrompt },
		func(u *user.User, v string) { u.CustomPrompt = v },
	)
}
```

**Step 2: Register in executor.go**

```go
registry["CFG_REALNAME"] = runCfgRealName
registry["CFG_PHONE"] = runCfgPhone
registry["CFG_NOTE"] = runCfgNote
registry["CFG_CUSTOMPROMPT"] = runCfgCustomPrompt
```

**Step 3: Verify build and commit**

```bash
git add internal/menu/user_config.go internal/menu/executor.go
git commit -m "feat: add string input config commands (name, phone, note, prompt)"
```

---

### Task 7: Implement Color Picker

**Files:**
- Modify: `internal/menu/user_config.go`
- Modify: `internal/menu/executor.go` (register)

**Step 1: Add color picker handler**

The `args` parameter carries the color slot index (0-3 from CFG file: `RUN:CFG_COLOR 0`).

```go
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
```

**Step 2: Register**

```go
registry["CFG_COLOR"] = runCfgColor
```

**Step 3: Verify build and commit**

```bash
git add internal/menu/user_config.go internal/menu/executor.go
git commit -m "feat: add color picker config command"
```

---

### Task 8: Implement Password Change

**Files:**
- Modify: `internal/menu/user_config.go`
- Modify: `internal/menu/executor.go` (register)

**Step 1: Add password change handler**

Reuses `readPasswordSecurely` from `internal/menu/newuser.go` and bcrypt from `golang.org/x/crypto/bcrypt`.

```go
func runCfgPassword(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	// 1. Prompt for current password
	msg := "\r\n|07Current Password: "
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)

	oldPw, err := readPasswordSecurely(s, terminal, outputMode)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return currentUser, "", nil
	}

	// 2. Verify current password
	if bcryptErr := bcrypt.CompareHashAndPassword([]byte(currentUser.PasswordHash), []byte(oldPw)); bcryptErr != nil {
		msg := "\r\n|09Incorrect password.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	// 3. Prompt for new password using existing helper
	newPw, err := e.promptForPassword(s, terminal, nodeNumber, outputMode)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return currentUser, "", nil
	}
	if newPw == "" {
		return currentUser, "", nil // Cancelled
	}

	// 4. Hash and save
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
```

**Step 2: Add import for bcrypt**

Add `"golang.org/x/crypto/bcrypt"` to user_config.go imports.

**Step 3: Register**

```go
registry["CFG_PASSWORD"] = runCfgPassword
```

**Step 4: Verify build and commit**

```bash
git add internal/menu/user_config.go internal/menu/executor.go
git commit -m "feat: add password change config command"
```

---

### Task 9: Implement View Config

**Files:**
- Create: `menus/v3/templates/USRCFGV.TOP`
- Create: `menus/v3/templates/USRCFGV.BOT`
- Modify: `internal/menu/user_config.go`
- Modify: `internal/menu/executor.go` (register)

**Step 1: Create USRCFGV.TOP template**

```
|12Current Configuration|07
|08────────────────────────────────────────────────────────────────────────────────
```

**Step 2: Create USRCFGV.BOT template**

```
|08────────────────────────────────────────────────────────────────────────────────
```

**Step 3: Add view config handler**

This doesn't use a MID template — it renders settings directly since they're key-value pairs with mixed formatting.

```go
func runCfgViewConfig(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	// Load header/footer templates
	topPath := filepath.Join(e.MenuSetPath, "templates", "USRCFGV.TOP")
	botPath := filepath.Join(e.MenuSetPath, "templates", "USRCFGV.BOT")

	topBytes, _ := os.ReadFile(topPath)
	botBytes, _ := os.ReadFile(botPath)

	topBytes = stripSauceMetadata(topBytes)
	botBytes = stripSauceMetadata(botBytes)
	topBytes = normalizePipeCodeDelimiters(topBytes)
	botBytes = normalizePipeCodeDelimiters(botBytes)

	var buf bytes.Buffer

	if len(topBytes) > 0 {
		buf.Write(ansi.ReplacePipeCodes(topBytes))
		buf.WriteString("\r\n")
	}

	// Helper to format bool
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
	e.waitForAnyKey(s, terminal, nodeNumber)
	return currentUser, "", nil
}
```

**Step 4: Add imports**

Add `"bytes"`, `"os"`, `"path/filepath"` to user_config.go imports.

**Step 5: Register**

```go
registry["CFG_VIEWCONFIG"] = runCfgViewConfig
```

**Step 6: Verify build and commit**

```bash
git add menus/v3/templates/USRCFGV.TOP menus/v3/templates/USRCFGV.BOT internal/menu/user_config.go internal/menu/executor.go
git commit -m "feat: add view config command with template display"
```

---

### Task 10: Final Verification

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All pass

**Step 2: Run vet and build**

Run: `go vet ./... && go build ./...`
Expected: Clean (ignoring pre-existing sshserver warnings)

**Step 3: Verify menu loads**

Ensure all JSON config files are valid:

```bash
python3 -c "import json; json.load(open('menus/v3/cfg/USERCFG.CFG'))"
```

**Step 4: Close VIS-52 in Linear**
