package menu

import (
	"bufio"
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
	"github.com/stlalpha/vision3/internal/logging"
	terminalPkg "github.com/stlalpha/vision3/internal/terminal"
	"github.com/stlalpha/vision3/internal/user"
)

// runAuthenticate handles the authentication process when RUN:AUTHENTICATE is called.
func runAuthenticate(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	// If already logged in, maybe show an error or just return?
	if currentUser != nil {
		log.Printf("WARN: Node %d: User %s tried to run AUTHENTICATE while already logged in.", nodeNumber, currentUser.Handle)
		msg := "\r\n|01You are already logged in.|07\r\n"
		// Use WriteProcessedBytes
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil {
			log.Printf("ERROR: Failed writing already logged in message: %v", wErr)
		}
		time.Sleep(1 * time.Second) // Pause after failed attempt
		return nil, "", nil         // No user change, no error
	}

	// Define approximate coordinates (MODIFY THESE based on LOGIN.ANS)
	userRow, userCol := 18, 20
	passRow, passCol := 19, 20
	errorRow := passRow + 2 // Row for error messages

	// Move to Username position, display prompt, and read input
	terminal.Write([]byte(terminalPkg.MoveCursor(userRow, userCol)))
	usernamePrompt := "|07Username/Handle: |15" // Original prompt text was in ANSI
	// Use WriteProcessedBytes for prompt
	// Use DisplayContent to handle pipe codes and display
	wErr := terminal.DisplayContent([]byte(usernamePrompt))
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing username prompt: %v", nodeNumber, wErr)
		// Continue anyway?
	}
	usernameInput, err := terminal.ReadLine()
	if err != nil {
		if err == io.EOF {
			log.Printf("INFO: Node %d: User disconnected during username input.", nodeNumber)
			// Return an error that signals disconnection to the main loop
			return nil, "LOGOFF", io.EOF // Signal logoff
		}
		log.Printf("ERROR: Node %d: Failed to read username input: %v", nodeNumber, err)
		return nil, "", fmt.Errorf("failed reading username: %w", err) // Critical error
	}
	username := strings.TrimSpace(usernameInput)
	if username == "" {
		return nil, "", nil // Empty username, just redisplay login menu
	}

	// Move to Password position, display prompt, and read input securely
	terminal.Write([]byte(terminalPkg.MoveCursor(passRow, passCol)))
	passwordPrompt := "|07Password: |15" // Original prompt text was in ANSI
	terminal.DisplayContent([]byte(passwordPrompt))
	password, err := readPasswordSecurely(s, terminal)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during password input.", nodeNumber)
			return nil, "LOGOFF", io.EOF // Signal logoff
		}
		if err.Error() == "password entry interrupted" { // Check for Ctrl+C
			log.Printf("INFO: Node %d: User interrupted password entry.", nodeNumber)
			// Treat interrupt like a failed attempt?
			terminal.Write([]byte(terminalPkg.MoveCursor(errorRow, 1))) // Move cursor for message
			msg := "\r\n|01Login cancelled.|07\r\n"
			terminal.DisplayContent([]byte(msg))
			time.Sleep(500 * time.Millisecond)
			return nil, "", nil // No user change, no critical error
		}
		log.Printf("ERROR: Node %d: Failed to read password securely: %v", nodeNumber, err)
		return nil, "", fmt.Errorf("failed reading password: %w", err) // Critical error
	}

	// Attempt Authentication via UserManager
	logging.Debug(" Node %d: Attempting authentication for user: %s", nodeNumber, username)
	authUser, authenticated := userManager.Authenticate(username, password)
	if !authenticated {
		log.Printf("WARN: Node %d: Failed authentication attempt for user: %s", nodeNumber, username)
		// Display error message to user
		terminal.Write([]byte(terminalPkg.MoveCursor(errorRow, 1))) // Move cursor for message
		errMsg := "\r\n|01Login incorrect.|07\r\n"
		// Use WriteProcessedBytes
		wErr := terminal.DisplayContent([]byte(errMsg))
		if wErr != nil {
			log.Printf("ERROR: Failed writing login incorrect message: %v", wErr)
		}
		time.Sleep(1 * time.Second) // Pause after failed attempt
		return nil, "", nil         // Failed auth, but not a critical error. Let LOGIN menu handle retries.
	}

	// Check if user is validated
	if !authUser.Validated {
		log.Printf("INFO: Node %d: Login denied for user '%s' - account not validated", nodeNumber, username)
		// Display specific message for validation issue
		terminal.Write([]byte(terminalPkg.MoveCursor(errorRow, 1))) // Move cursor for message
		errMsg := "\r\n|01Account requires validation by SysOp.|07\r\n"
		// Use WriteProcessedBytes
		wErr := terminal.DisplayContent([]byte(errMsg))
		if wErr != nil {
			log.Printf("ERROR: Failed writing validation required message: %v", wErr)
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Not validated, treat as failed login for this attempt
	}

	// Authentication Successful!
	log.Printf("INFO: Node %d: User '%s' (Handle: %s) authenticated successfully via RUN:AUTHENTICATE", nodeNumber, authUser.Username, authUser.Handle)
	// Display success message (optional) - Move cursor first
	terminal.Write([]byte(terminalPkg.MoveCursor(errorRow, 1)))
	// successMsg := "\r\n|10Login successful!|07\r\n"
	// terminal.Write(terminal.DisplayContent([]byte(successMsg)))
	// time.Sleep(500 * time.Millisecond)

	// Return the authenticated user object!
	return authUser, "", nil
}

// runFullLoginSequence executes the sequence of actions after FASTLOGN option 1.
func runFullLoginSequence(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	log.Printf("INFO: Node %d: Running FULL_LOGIN_SEQUENCE for user %s", nodeNumber, currentUser.Handle)
	var nextAction string
	var err error

	// 1. Run Last Callers
	_, nextAction, err = runLastCallers(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode)
	if err != nil {
		log.Printf("ERROR: Node %d: Error during runLastCallers in sequence: %v", nodeNumber, err)
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
	}
	if nextAction == "LOGOFF" {
		return nil, "LOGOFF", nil
	}

	// 2. Run Oneliners
	_, nextAction, err = runOneliners(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode)
	if err != nil {
		log.Printf("ERROR: Node %d: Error during runOneliners in sequence: %v", nodeNumber, err)
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
	}
	if nextAction == "LOGOFF" {
		return nil, "LOGOFF", nil
	}

	// 3. Run Show Stats
	_, nextAction, err = runShowStats(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode)
	if err != nil {
		log.Printf("ERROR: Node %d: Error during runShowStats in sequence: %v", nodeNumber, err)
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
	}
	if nextAction == "LOGOFF" {
		return nil, "LOGOFF", nil
	}

	// 4. Signal transition to MAIN menu
	logging.Debug(" Node %d: FULL_LOGIN_SEQUENCE completed. Transitioning to MAIN.", nodeNumber)
	return currentUser, "GOTO:MAIN", nil
}

// runListUsers displays a list of users, sorted alphabetically.
func runListUsers(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	logging.Debug(" Node %d: Running LISTUSERS", nodeNumber)

	// 1. Load Templates (Corrected filenames)
	topTemplatePath := filepath.Join(e.MenuSetPath, "templates", "USERLIST.TOP")
	midTemplatePath := filepath.Join(e.MenuSetPath, "templates", "USERLIST.MID")
	botTemplatePath := filepath.Join(e.MenuSetPath, "templates", "USERLIST.BOT")

	topTemplateBytes, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplateBytes, errBot := os.ReadFile(botTemplatePath)

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load one or more USERLIST template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading User List screen templates.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading USERLIST templates")
	}

	// --- Process Pipe Codes in Templates FIRST ---
	processedTopTemplate := string(terminal.ProcessPipeCodes(topTemplateBytes))
	processedMidTemplate := string(terminal.ProcessPipeCodes(midTemplateBytes)) // Process MID template
	processedBotTemplate := string(terminal.ProcessPipeCodes(botTemplateBytes))
	// --- END Template Processing ---

	// 2. Get user list data from UserManager
	users := userManager.GetAllUsers() // Corrected method call

	// 3. Build the output string using processed templates and processed data
	var outputBuffer bytes.Buffer
	outputBuffer.Write([]byte(processedTopTemplate)) // Write processed top template

	if len(users) == 0 {
		// Optional: Handle empty state. The template might handle this.
		logging.Debug(" Node %d: No users to display.", nodeNumber)
		// If templates don't handle empty, add a message here.
	} else {
		// Iterate through user records and format using processed USERLIST.MID
		for _, user := range users {
			line := processedMidTemplate // Start with the pipe-code-processed mid template

			// Format data for substitution
			handle := user.Handle
			level := strconv.Itoa(user.AccessLevel)
			// privateNote := terminal.DisplayContent([]byte(user.PrivateNote))) // Replaced with GroupLocation
			groupLocation := user.GroupLocation // Added

			// Replace placeholders with *already processed* data
			// Match placeholders found in USERLIST.MID: |UH, |GL, |LV, |AC
			line = strings.ReplaceAll(line, "|UH", handle)        // Use |UH for Handle (Alias)
			line = strings.ReplaceAll(line, "|GL", groupLocation) // Use |GL for Group/Location (Replaces |UN)
			line = strings.ReplaceAll(line, "|LV", level)         // Use |LV for Level

			logging.Debug(" About to write line for user %s: %q", handle, line)
			outputBuffer.WriteString(line) // Add the fully substituted and processed line
			logging.Debug(" Wrote line. Buffer size now: %d", outputBuffer.Len())
		}
	}

	logging.Debug(" Finished user loop. Total buffer size before BOT: %d", outputBuffer.Len())
	outputBuffer.Write([]byte(processedBotTemplate)) // Write processed bottom template
	logging.Debug(" Added BOT template. Final buffer size: %d", outputBuffer.Len())

	// 4. Clear screen and display the assembled content
	writeErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
	if writeErr != nil {
		log.Printf("ERROR: Node %d: Failed clearing screen for USERLIST: %v", nodeNumber, writeErr)
		return nil, "", writeErr
	}

	// Use WriteProcessedBytes for the assembled template content
	processedContent := outputBuffer.Bytes() // Contains already-processed ANSI bytes
	_, wErr := terminal.Write(processedContent)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing USERLIST output: %v", nodeNumber, wErr)
		return nil, "", wErr
	}

	// 5. Wait for Enter using configured PauseString (logic remains the same)
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... " // Fallback
	}

	var pauseBytesToWrite []byte
	processedPausePrompt := string(terminal.ProcessPipeCodes([]byte(pausePrompt)))
	if outputMode == terminalPkg.OutputModeCP437 {
		var cp437Buf bytes.Buffer
		for _, r := range string(processedPausePrompt) {
			if r < 128 {
				cp437Buf.WriteByte(byte(r))
			} else if cp437Byte, ok := terminalPkg.UnicodeToCP437Table[r]; ok {
				cp437Buf.WriteByte(cp437Byte)
			} else {
				cp437Buf.WriteByte('?')
			}
		}
		pauseBytesToWrite = cp437Buf.Bytes()
	} else {
		pauseBytesToWrite = []byte(processedPausePrompt)
	}

	logging.Debug(" Node %d: Writing USERLIST pause prompt. Mode: %d, Bytes: %q", nodeNumber, outputMode, string(pauseBytesToWrite))
	// Log hex bytes before writing
	logging.Debug(" Node %d: Writing USERLIST pause bytes (hex): %x", nodeNumber, pauseBytesToWrite)
	_, wErr = terminal.Write(pauseBytesToWrite)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing USERLIST pause prompt: %v", nodeNumber, wErr)
	}

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during USERLIST pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading input during USERLIST pause: %v", nodeNumber, err)
			return nil, "", err
		}
		if r == '\r' || r == '\n' { // Check for CR or LF
			break
		}
	}

	return nil, "", nil // Success
}

// runShowStats displays the user's stats screen (YOURSTAT.ANS) with placeholders replaced.
func runShowStats(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		log.Printf("WARN: Node %d: SHOWSTATS called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to view stats.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil {
			log.Printf("ERROR: Failed writing SHOWSTATS error message: %v", wErr)
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Updated return
	}

	ansFilename := "YOURSTAT.ANS"
	// Use MenuSetPath for ANSI file
	fullAnsPath := filepath.Join(e.MenuSetPath, "ansi", ansFilename)
	rawAnsiContent, readErr := os.ReadFile(fullAnsPath)
	if readErr != nil {
		log.Printf("ERROR: Node %d: Failed to read %s for SHOWSTATS: %v", nodeNumber, fullAnsPath, readErr)
		msg := fmt.Sprintf("\r\n|01Error displaying stats screen (%s).|07\r\n", ansFilename)
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil {
			log.Printf("ERROR: Failed writing SHOWSTATS file read error message: %v", wErr)
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed to read %s: %w", ansFilename, readErr) // Updated return
	}

	placeholders := map[string]string{
		"|UH": currentUser.Handle,
		"|UN": currentUser.PrivateNote,
		"|UL": strconv.Itoa(currentUser.AccessLevel),
		"|FL": strconv.Itoa(currentUser.AccessLevel),
		"|UK": strconv.Itoa(currentUser.NumUploads),
		"|NU": strconv.Itoa(currentUser.NumUploads),
		"|DK": "0", "|ND": "0", "|TP": "0", "|NM": "0", "|LC": "N/A",
	}
	if currentUser.TimeLimit <= 0 {
		placeholders["|TL"] = "Unlimited"
	} else {
		elapsedSeconds := time.Since(sessionStartTime).Seconds()
		totalSeconds := float64(currentUser.TimeLimit * 60)
		remainingSeconds := totalSeconds - elapsedSeconds
		if remainingSeconds < 0 {
			remainingSeconds = 0
		}
		placeholders["|TL"] = strconv.Itoa(int(remainingSeconds / 60))
	}

	substitutedContent := string(rawAnsiContent)
	for key, val := range placeholders {
		substitutedContent = strings.ReplaceAll(substitutedContent, key, val)
	}

	// Use WriteProcessedBytes for ClearScreen
	wErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
	if wErr != nil {
		// Log error but continue if possible
		log.Printf("ERROR: Node %d: Failed clearing screen for SHOWSTATS: %v", nodeNumber, wErr)
	}

	// Display content with proper pipe code processing
	statsDisplayBytes := []byte(substitutedContent)
	wErr = terminal.DisplayContent(statsDisplayBytes)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing processed YOURSTAT.ANS: %v", nodeNumber, wErr)
		return nil, "", wErr // Updated return
	}

	// 5. Wait for Enter key press
	pausePrompt := e.LoadedStrings.PauseString // Use configured pause string
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... " // Fallback
	}

	// Display pause prompt directly - DisplayContent handles encoding and pipe codes
	logging.Debug(" Node %d: Displaying SHOWSTATS pause prompt: %q", nodeNumber, pausePrompt)
	wErr = terminal.DisplayContent([]byte(pausePrompt))
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing SHOWSTATS pause prompt: %v", nodeNumber, wErr)
	}
	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during SHOWSTATS pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF // Updated return (Signal logoff)
			}
			log.Printf("ERROR: Failed reading input during SHOWSTATS pause: %v", err)
			return nil, "", err // Updated return
		}
		if r == '\r' || r == '\n' {
			break
		}
	}
	return nil, "", nil // Updated return (Success)
}

// runShowVersion displays static version information.
func runShowVersion(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	logging.Debug(" Node %d: Running SHOWVERSION", nodeNumber)

	// Define the version string (can be made dynamic later)
	versionString := "|15ViSiON/3 Go Edition - v0.1.0 (Pre-Alpha)|07"

	// Display the version
	terminal.DisplayContent([]byte("\x1b[2J\x1b[H")) // Optional: Clear screen
	terminal.Write([]byte("\r\n\r\n"))               // Add some spacing
	wErr := terminal.DisplayContent([]byte(versionString))
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing SHOWVERSION output: %v", nodeNumber, wErr)
		// Don't return error, just log it
	}

	// Wait for Enter
	pausePrompt := e.LoadedStrings.PauseString // Use configured pause string
	if pausePrompt == "" {
		log.Printf("WARN: Node %d: PauseString is empty in config/strings.json. No pause prompt will be shown for SHOWVERSION.", nodeNumber)
		// Don't use a hardcoded fallback. If it's empty, it's empty.
	} else {
		terminal.Write([]byte("\r\n")) // Add newline before pause only if prompt exists
		var pauseBytesToWrite []byte
		processedPausePrompt := string(terminal.ProcessPipeCodes([]byte(pausePrompt)))
		if outputMode == terminalPkg.OutputModeCP437 {
			var cp437Buf bytes.Buffer
			for _, r := range string(processedPausePrompt) {
				if r < 128 {
					cp437Buf.WriteByte(byte(r))
				} else if cp437Byte, ok := terminalPkg.UnicodeToCP437Table[r]; ok {
					cp437Buf.WriteByte(cp437Byte)
				} else {
					cp437Buf.WriteByte('?')
				}
			}
			pauseBytesToWrite = cp437Buf.Bytes()
		} else {
			pauseBytesToWrite = []byte(processedPausePrompt)
		}
		_, wErr = terminal.Write(pauseBytesToWrite)
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing SHOWVERSION pause prompt: %v", nodeNumber, wErr)
		}
	}

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during SHOWVERSION pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading input during SHOWVERSION pause: %v", nodeNumber, err)
			return nil, "", err // Return error on read failure
		}
		// Correct rune literals for Enter key check (CR or LF)
		if r == '\r' || r == '\n' {
			break
		}
	}

	return nil, "", nil // Return to the current menu
}

// runLastCallers displays the last callers list using templates.
func runLastCallers(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	logging.Debug(" Node %d: Running LASTCALLERS", nodeNumber)

	// 1. Load Template Files from MenuSetPath/templates
	topTemplatePath := filepath.Join(e.MenuSetPath, "templates", "LASTCALL.TOP")
	midTemplatePath := filepath.Join(e.MenuSetPath, "templates", "LASTCALL.MID")
	botTemplatePath := filepath.Join(e.MenuSetPath, "templates", "LASTCALL.BOT")

	topTemplateBytes, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplateBytes, errBot := os.ReadFile(botTemplatePath)

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load one or more LASTCALL template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading Last Callers screen templates.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading LASTCALL templates")
	}

	// --- Process Pipe Codes in Templates FIRST ---
	processedTopTemplate := string(terminal.ProcessPipeCodes(topTemplateBytes))
	processedMidTemplate := string(terminal.ProcessPipeCodes(midTemplateBytes)) // Process MID template
	processedBotTemplate := string(terminal.ProcessPipeCodes(botTemplateBytes))
	// --- END Template Processing ---

	// 2. Get last callers data from UserManager
	lastCallers := userManager.GetLastCallers()

	// 3. Build the output string using processed templates and processed data
	var outputBuffer bytes.Buffer
	outputBuffer.Write([]byte(processedTopTemplate)) // Write processed top template

	if len(lastCallers) == 0 {
		// Optional: Handle empty state. The template might handle this.
		logging.Debug(" Node %d: No last callers to display.", nodeNumber)
		// If templates don't handle empty, add a message here.
	} else {
		// Iterate through call records and format using processed LASTCALL.MID
		for _, record := range lastCallers {
			line := processedMidTemplate // Start with the pipe-code-processed mid template

			// Format data for substitution
			baud := record.BaudRate // Static for now
			// Process pipe codes in user data *before* substitution
			name := record.Handle            // Process Handle
			groupLoc := record.GroupLocation // Process GroupLocation
			// --- END User Data Processing ---
			onTime := record.ConnectTime.Format("15:04:05") // HH:MM:SS format
			actions := record.Actions                       // Placeholder - process if it can contain pipe codes
			hours := int(record.Duration.Hours())
			mins := int(record.Duration.Minutes()) % 60
			hmm := fmt.Sprintf("%d:%02d", hours, mins)
			upM := fmt.Sprintf("%.1f", record.UploadedMB)
			dnM := fmt.Sprintf("%.1f", record.DownloadedMB)
			nodeStr := strconv.Itoa(record.NodeID)
			callNumStr := strconv.FormatUint(record.CallNumber, 10) // Format the new call number

			// Replace placeholders with *already processed* data
			// Match placeholders found in LASTCALL.MID: ^UN, ^BA etc.
			line = strings.ReplaceAll(line, "^BA", baud)       // Corrected placeholder for Baud
			line = strings.ReplaceAll(line, "^UN", name)       // Corrected placeholder for User Name/Handle
			line = strings.ReplaceAll(line, "^GL", groupLoc)   // Keep this if present in template
			line = strings.ReplaceAll(line, "^OT", onTime)     // Keep this if present in template
			line = strings.ReplaceAll(line, "^AC", actions)    // Keep this if present in template
			line = strings.ReplaceAll(line, "^HM", hmm)        // Keep this if present in template
			line = strings.ReplaceAll(line, "^UM", upM)        // Keep this if present in template
			line = strings.ReplaceAll(line, "^DM", dnM)        // Keep this if present in template
			line = strings.ReplaceAll(line, "|NU", nodeStr)    // Corrected placeholder for Node Number
			line = strings.ReplaceAll(line, "^CN", callNumStr) // Add replacement for Call Number
			// Removed ^ND placeholder as |NU is used in template

			outputBuffer.WriteString(line) // Add the fully substituted and processed line
		}
	}

	outputBuffer.Write([]byte(processedBotTemplate)) // Write processed bottom template

	// 4. Clear screen and display the assembled content
	writeErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
	if writeErr != nil {
		log.Printf("ERROR: Node %d: Failed clearing screen for LASTCALLERS: %v", nodeNumber, writeErr)
		return nil, "", writeErr
	}

	// Use WriteProcessedBytes for the assembled template content
	processedContent := outputBuffer.Bytes() // Contains already-processed ANSI bytes
	_, wErr := terminal.Write(processedContent)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing LASTCALLERS output: %v", nodeNumber, wErr)
		return nil, "", wErr
	}

	// 5. Wait for Enter using configured PauseString (logic remains the same)
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... " // Fallback
	}

	var pauseBytesToWrite []byte
	processedPausePrompt := string(terminal.ProcessPipeCodes([]byte(pausePrompt)))
	if outputMode == terminalPkg.OutputModeCP437 {
		var cp437Buf bytes.Buffer
		for _, r := range string(processedPausePrompt) {
			if r < 128 {
				cp437Buf.WriteByte(byte(r))
			} else if cp437Byte, ok := terminalPkg.UnicodeToCP437Table[r]; ok {
				cp437Buf.WriteByte(cp437Byte)
			} else {
				cp437Buf.WriteByte('?')
			}
		}
		pauseBytesToWrite = cp437Buf.Bytes()
	} else {
		pauseBytesToWrite = []byte(processedPausePrompt)
	}

	logging.Debug(" Node %d: Writing LASTCALLERS pause prompt. Mode: %d, Bytes: %q", nodeNumber, outputMode, string(pauseBytesToWrite))
	// Log hex bytes before writing
	logging.Debug(" Node %d: Writing LASTCALLERS pause bytes (hex): %x", nodeNumber, pauseBytesToWrite)
	_, wErr = terminal.Write(pauseBytesToWrite)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing LASTCALLERS pause prompt: %v", nodeNumber, wErr)
	}

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during LASTCALLERS pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading input during LASTCALLERS pause: %v", nodeNumber, err)
			return nil, "", err
		}
		if r == '\r' || r == '\n' { // Check for CR or LF
			break
		}
	}

	return nil, "", nil // Success
}
