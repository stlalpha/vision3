package menu

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/config"
	"golang.org/x/term"
)

// buildDoorCtx creates a DoorCtx from the standard RunnableFunc parameters.
func buildDoorCtx(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userID int, handle, realName string, accessLevel, timeLimit, timesCalled int,
	phoneNumber, groupLocation string, screenWidth, screenHeight int,
	nodeNumber int, sessionStartTime time.Time, outputMode ansi.OutputMode,
	doorConfig config.DoorConfig, doorName string) *DoorCtx {

	nodeNumStr := strconv.Itoa(nodeNumber)
	portStr := nodeNumStr

	elapsedMinutes := int(time.Since(sessionStartTime).Minutes())
	remainingMinutes := timeLimit - elapsedMinutes
	if remainingMinutes < 0 {
		remainingMinutes = 0
	}
	timeLeftStr := strconv.Itoa(remainingMinutes)
	baudStr := "38400"
	userIDStr := strconv.Itoa(userID)

	subs := map[string]string{
		"{NODE}":       nodeNumStr,
		"{PORT}":       portStr,
		"{TIMELEFT}":   timeLeftStr,
		"{BAUD}":       baudStr,
		"{USERHANDLE}": handle,
		"{USERID}":     userIDStr,
		"{REALNAME}":   realName,
		"{LEVEL}":      strconv.Itoa(accessLevel),
	}

	return &DoorCtx{
		Executor: e,
		Session:  s,
		Terminal: terminal,
		User: doorUserInfo{
			ID:            userID,
			Handle:        handle,
			RealName:      realName,
			AccessLevel:   accessLevel,
			TimeLimit:     timeLimit,
			TimesCalled:   timesCalled,
			PhoneNumber:   phoneNumber,
			GroupLocation: groupLocation,
			ScreenWidth:   screenWidth,
			ScreenHeight:  screenHeight,
		},
		NodeNumber:       nodeNumber,
		SessionStartTime: sessionStartTime,
		OutputMode:       outputMode,
		Config:           doorConfig,
		DoorName:         doorName,
		NodeNumStr:       nodeNumStr,
		PortStr:          portStr,
		TimeLeftMin:      remainingMinutes,
		TimeLeftStr:      timeLeftStr,
		BaudStr:          baudStr,
		UserIDStr:        userIDStr,
		Subs:             subs,
	}
}

// --- Dropfile Generators ---

// generateDoorSys writes a full 52-line PCBoard DOOR.SYS file.
func generateDoorSys(ctx *DoorCtx, dir string) error {
	path := filepath.Join(dir, "DOOR.SYS")
	log.Printf("INFO: Generating DOOR.SYS at: %s", path)

	bbsName := ctx.Executor.ServerCfg.BoardName
	timeLeftSecs := ctx.TimeLeftMin * 60

	var b strings.Builder
	crlf := "\r\n"

	b.WriteString("COM1:" + crlf)                            // 1: COM port
	b.WriteString("38400" + crlf)                            // 2: Baud rate
	b.WriteString("8" + crlf)                                // 3: Data bits
	b.WriteString(ctx.NodeNumStr + crlf)                     // 4: Node number
	b.WriteString("38400" + crlf)                            // 5: Locked baud rate
	b.WriteString("Y" + crlf)                                // 6: Screen display
	b.WriteString("N" + crlf)                                // 7: Printer toggle
	b.WriteString("Y" + crlf)                                // 8: Page bell
	b.WriteString("Y" + crlf)                                // 9: Caller alarm
	b.WriteString(ctx.User.RealName + crlf)                  // 10: User full name
	b.WriteString(ctx.User.GroupLocation + crlf)             // 11: Calling from (location)
	b.WriteString("00-0000-0000" + crlf)                     // 12: Home phone
	b.WriteString("00-0000-0000" + crlf)                     // 13: Work phone
	b.WriteString("SECRET" + crlf)                           // 14: Password (placeholder)
	b.WriteString(strconv.Itoa(ctx.User.AccessLevel) + crlf) // 15: Security level
	b.WriteString(strconv.Itoa(ctx.User.TimesCalled) + crlf) // 16: Total times on
	b.WriteString("01-01-1971" + crlf)                       // 17: Last call date
	b.WriteString(strconv.Itoa(timeLeftSecs) + crlf)         // 18: Seconds remaining
	b.WriteString("255" + crlf)                              // 19: Time limit (minutes)
	b.WriteString("GR" + crlf)                               // 20: Graphics mode

	// Use user's saved screen height, default to 25 if not set
	screenHeight := ctx.User.ScreenHeight
	if screenHeight <= 0 {
		screenHeight = 25
	}
	b.WriteString(strconv.Itoa(screenHeight) + crlf) // 21: Screen length
	b.WriteString("N" + crlf)                        // 22: Expert mode
	b.WriteString(crlf)                              // 23: Conferences registered
	b.WriteString(crlf)                              // 24: Conference exited to
	b.WriteString(crlf)                              // 25: Expiration date
	b.WriteString(ctx.UserIDStr + crlf)              // 26: User record number
	b.WriteString(crlf)                              // 27: Default protocol
	b.WriteString("0" + crlf)                        // 28: Total uploads
	b.WriteString("0" + crlf)                        // 29: Total downloads
	b.WriteString("0" + crlf)                        // 30: Daily download K-bytes total
	b.WriteString("99999" + crlf)                    // 31: Daily download K-bytes allowed
	b.WriteString("01-01-1971" + crlf)               // 32: Birth date
	b.WriteString(crlf)                              // 33: Path to callinfo/main dir
	b.WriteString(crlf)                              // 34: Path to GEN dir
	b.WriteString(bbsName + crlf)                    // 35: Sysop name (BBS name used)
	b.WriteString(ctx.User.Handle + crlf)            // 36: User handle/alias
	b.WriteString("none" + crlf)                     // 37: Next event time
	b.WriteString("Y" + crlf)                        // 38: Error free connection
	b.WriteString("N" + crlf)                        // 39: Always "N"
	b.WriteString("Y" + crlf)                        // 40: Always "Y"
	b.WriteString("7" + crlf)                        // 41: Default color
	b.WriteString("0" + crlf)                        // 42: Time credits (minutes)
	b.WriteString("01-01-1971" + crlf)               // 43: Last new file scan date
	b.WriteString("00:00" + crlf)                    // 44: Time of this call
	b.WriteString("00:00" + crlf)                    // 45: Time of last call
	b.WriteString("32768" + crlf)                    // 46: Max daily files allowed
	b.WriteString("0" + crlf)                        // 47: Files downloaded today
	b.WriteString("0" + crlf)                        // 48: Total K-bytes uploaded
	b.WriteString("0" + crlf)                        // 49: Total K-bytes downloaded
	b.WriteString("None." + crlf)                    // 50: Comment
	b.WriteString("0" + crlf)                        // 51: Total doors opened
	b.WriteString("0" + crlf)                        // 52: Total messages left

	return os.WriteFile(path, []byte(b.String()), 0600)
}

// generateDoor32Sys writes an 11-line DOOR32.SYS file.
func generateDoor32Sys(ctx *DoorCtx, dir string) error {
	path := filepath.Join(dir, "DOOR32.SYS")
	log.Printf("INFO: Generating DOOR32.SYS at: %s", path)

	bbsName := ctx.Executor.ServerCfg.BoardName
	crlf := "\r\n"

	var b strings.Builder
	b.WriteString("0" + crlf)                                // 1: Comm type (0=local)
	b.WriteString("0" + crlf)                                // 2: Comm/socket handle
	b.WriteString("38400" + crlf)                            // 3: Baud rate
	b.WriteString(bbsName + crlf)                            // 4: BBS software name/version
	b.WriteString(ctx.UserIDStr + crlf)                      // 5: User record number
	b.WriteString(ctx.User.RealName + crlf)                  // 6: User's real name
	b.WriteString(ctx.User.Handle + crlf)                    // 7: User's handle/alias
	b.WriteString(strconv.Itoa(ctx.User.AccessLevel) + crlf) // 8: Security level
	b.WriteString(strconv.Itoa(ctx.TimeLeftMin) + crlf)      // 9: Time remaining (minutes)
	b.WriteString("1" + crlf)                                // 10: Emulation (1=ANSI)
	b.WriteString(ctx.NodeNumStr + crlf)                     // 11: Node number

	return os.WriteFile(path, []byte(b.String()), 0600)
}

// generateDorInfo writes a 13-line DORINFO1.DEF file.
func generateDorInfo(ctx *DoorCtx, dir string) error {
	path := filepath.Join(dir, "DORINFO1.DEF")
	log.Printf("INFO: Generating DORINFO1.DEF at: %s", path)

	bbsName := ctx.Executor.ServerCfg.BoardName
	crlf := "\r\n"

	// Split real name into first/last
	firstName := ctx.User.RealName
	lastName := " "
	if idx := strings.Index(ctx.User.RealName, " "); idx >= 0 {
		firstName = ctx.User.RealName[:idx]
		if idx < len(ctx.User.RealName)-1 {
			lastName = ctx.User.RealName[idx+1:]
		}
	}

	location := ctx.User.GroupLocation
	if location == "" {
		location = "Somewhere"
	}

	var b strings.Builder
	b.WriteString(bbsName + crlf)                            // 1: BBS name
	b.WriteString("Sysop" + crlf)                            // 2: Sysop name
	b.WriteString(" " + crlf)                                // 3: Blank
	b.WriteString("COM1" + crlf)                             // 4: COM port
	b.WriteString("115200 BAUD,N,8,1" + crlf)                // 5: Baud, parity, data, stop
	b.WriteString("0" + crlf)                                // 6: Networked (0=local/no network, 1=networked)
	b.WriteString(firstName + crlf)                          // 7: User first name
	b.WriteString(lastName + crlf)                           // 8: User last name
	b.WriteString(location + crlf)                           // 9: Location
	b.WriteString("1" + crlf)                                // 10: Graphics (1=yes)
	b.WriteString(strconv.Itoa(ctx.User.AccessLevel) + crlf) // 11: Security level
	b.WriteString(strconv.Itoa(ctx.TimeLeftMin) + crlf)      // 12: Time remaining (minutes)
	b.WriteString("-1" + crlf)                               // 13: FOSSIL flag (-1=no)

	return os.WriteFile(path, []byte(b.String()), 0600)
}

// generateChainTxt writes a CHAIN.TXT file (WWIV format).
func generateChainTxt(ctx *DoorCtx, dir string) error {
	path := filepath.Join(dir, "CHAIN.TXT")
	log.Printf("INFO: Generating CHAIN.TXT at: %s", path)

	bbsName := ctx.Executor.ServerCfg.BoardName
	timeLeftSecs := ctx.TimeLeftMin * 60
	crlf := "\r\n"

	var b strings.Builder
	b.WriteString(ctx.UserIDStr + crlf)     // 1: User number
	b.WriteString(ctx.User.Handle + crlf)   // 2: User alias
	b.WriteString(ctx.User.RealName + crlf) // 3: Real name
	b.WriteString("NONE" + crlf)                                                    // 4: Default protocol
	b.WriteString(strconv.Itoa(int(time.Since(ctx.SessionStartTime).Minutes())) + crlf) // 5: Time on (minutes)
	b.WriteString("M" + crlf)               // 6: Gender
	b.WriteString("0" + crlf)               // 7: Pause (0=no)
	b.WriteString("01/01/71" + crlf)        // 8: Last call date

	// Use user's saved screen dimensions, default to 80x25 if not set
	screenWidth := ctx.User.ScreenWidth
	if screenWidth <= 0 {
		screenWidth = 80
	}
	screenHeight := ctx.User.ScreenHeight
	if screenHeight <= 0 {
		screenHeight = 25
	}
	b.WriteString(strconv.Itoa(screenWidth) + crlf)          // 9: Screen width
	b.WriteString(strconv.Itoa(screenHeight) + crlf)         // 10: Screen height
	b.WriteString(strconv.Itoa(ctx.User.AccessLevel) + crlf) // 11: Security level
	b.WriteString("0" + crlf)                                // 12: CO-sysop flag
	b.WriteString("0" + crlf)                                // 13: File ratio flag
	b.WriteString("1" + crlf)                                // 14: ANSI status
	b.WriteString("1" + crlf)                                // 15: Another flag
	b.WriteString(strconv.Itoa(timeLeftSecs) + crlf)         // 16: Time left (seconds)
	b.WriteString(dir + crlf)                                // 17: Gfiles path
	b.WriteString(dir + crlf)                                // 18: Temp path
	b.WriteString("NOLOG" + crlf)                            // 19: Sysop log
	b.WriteString("9600" + crlf)                             // 20: Baud rate
	b.WriteString("1" + crlf)                                // 21: Another flag
	b.WriteString(bbsName + crlf)                            // 22: BBS name
	b.WriteString("Sysop" + crlf)                            // 23: Sysop name
	b.WriteString("0" + crlf)                                // 24-28: Flags
	b.WriteString("0" + crlf)
	b.WriteString("0" + crlf)
	b.WriteString("0" + crlf)
	b.WriteString("0" + crlf)
	b.WriteString("0" + crlf)   // 29: Flag
	b.WriteString("8N1" + crlf) // 30: Serial config

	return os.WriteFile(path, []byte(b.String()), 0600)
}

// generateAllDropfiles generates all four standard dropfile formats in the given directory.
func generateAllDropfiles(ctx *DoorCtx, dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create dropfile directory %s: %w", dir, err)
	}

	if err := generateDoorSys(ctx, dir); err != nil {
		return fmt.Errorf("failed to generate DOOR.SYS: %w", err)
	}
	if err := generateDoor32Sys(ctx, dir); err != nil {
		return fmt.Errorf("failed to generate DOOR32.SYS: %w", err)
	}
	if err := generateDorInfo(ctx, dir); err != nil {
		return fmt.Errorf("failed to generate DORINFO1.DEF: %w", err)
	}
	if err := generateChainTxt(ctx, dir); err != nil {
		return fmt.Errorf("failed to generate CHAIN.TXT: %w", err)
	}

	log.Printf("INFO: All dropfiles generated in %s", dir)
	return nil
}

// cleanupDropfiles removes all generated dropfiles from the directory.
func cleanupDropfiles(dir string) {
	files := []string{"DOOR.SYS", "DOOR32.SYS", "DORINFO1.DEF", "CHAIN.TXT", "EXTERNAL.BAT"}
	for _, f := range files {
		path := filepath.Join(dir, f)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("WARN: Failed to remove %s: %v", path, err)
		}
	}
	log.Printf("DEBUG: Cleaned up dropfiles in %s", dir)
}
