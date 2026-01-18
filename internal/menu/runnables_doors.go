package menu

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/logging"
	terminalPkg "github.com/stlalpha/vision3/internal/terminal"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// runDoor executes an external door program for the connected user.
// The doorName parameter specifies which door to run from the DoorRegistry.
// This function handles:
//   - User validation (must be logged in)
//   - Door configuration lookup
//   - Placeholder substitution in arguments and environment variables
//   - Dropfile generation (DOOR.SYS, CHAIN.TXT)
//   - Command execution (PTY/Raw mode or standard I/O)
//   - Post-door cleanup and return to menu
func runDoor(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, doorName string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {

	if currentUser == nil {
		log.Printf("WARN: Node %d: DOOR:%s called without logged in user.", nodeNumber, doorName)
		msg := "\r\n|01Error: You must be logged in to run doors.|07\r\n"
		writeErr := terminal.DisplayContent([]byte(msg))
		if writeErr != nil {
			log.Printf("ERROR: Failed writing DOOR error message (not logged in): %v", writeErr)
		}
		return nil, "", nil // Stay in menu loop
	}
	log.Printf("INFO: Node %d: User %s attempting to run door: %s", nodeNumber, currentUser.Handle, doorName)

	// 1. Look up door configuration
	doorConfig, exists := e.DoorRegistry[strings.ToUpper(doorName)] // Ensure lookup is case-insensitive
	if !exists {
		log.Printf("WARN: Door configuration not found for '%s'", doorName)
		errMsg := fmt.Sprintf("\r\n|12Error: Door '%s' is not configured.\r\nPress Enter to continue...\r\n", doorName)
		writeErr := terminal.DisplayContent([]byte(errMsg))
		if writeErr != nil {
			log.Printf("ERROR: Failed writing DOOR error message (not configured) to stderr: %v", writeErr)
		}
		return nil, "", nil // Stay in menu loop
	}

	// Build substitution map for placeholders
	substitutions := buildDoorSubstitutions(currentUser, nodeNumber, sessionStartTime)

	// Substitute in Arguments
	substitutedArgs := substituteInArgs(doorConfig.Args, substitutions)

	// Substitute in Environment Variables
	substitutedEnv := substituteInEnvVars(doorConfig.EnvironmentVars, substitutions)

	// Generate dropfile if configured
	_, cleanupDropfile, dropfileErr := generateDropfile(doorConfig, currentUser, nodeNumber, sessionStartTime, substitutions)
	if dropfileErr != nil {
		errMsg := fmt.Sprintf("\r\n|12Error creating system file for door '%s'.\r\nPress Enter to continue...\r\n", doorName)
		_ = terminal.DisplayContent([]byte(errMsg)) // Error display, ignore write error
		return nil, "", nil                         // Abort door execution
	}
	if cleanupDropfile != nil {
		defer cleanupDropfile()
	}

	// Prepare the command
	cmd := exec.Command(doorConfig.Command, substitutedArgs...)

	// Set working directory if specified
	if doorConfig.WorkingDirectory != "" {
		cmd.Dir = doorConfig.WorkingDirectory
		logging.Debug(" Setting working directory for door '%s' to '%s'", doorName, cmd.Dir)
	}

	// Set environment variables
	cmd.Env = buildDoorEnvironment(substitutedEnv, substitutions)

	// Execute Command (Handles PTY/Raw mode)
	ptyReq, windowChangeChannel, isPty := s.Pty()
	var commandErr error

	if doorConfig.RequiresRawTerminal && isPty {
		commandErr = executeDoorWithPTY(cmd, s, ptyReq, windowChangeChannel, nodeNumber, doorName)
	} else {
		if doorConfig.RequiresRawTerminal && !isPty {
			log.Printf("WARN: Node %d: Door '%s' requires raw terminal, but no PTY was allocated. Door may not function correctly.", nodeNumber, doorName)
		}
		commandErr = executeDoorWithStdIO(cmd, s, nodeNumber, doorName)
	}

	// Handle result
	handleDoorResult(commandErr, terminal, s, nodeNumber, currentUser.Handle, doorName)

	return nil, "", nil // Return nil user, "", nil error to continue menu loop after door
}

// buildDoorSubstitutions creates a map of placeholder substitutions for door arguments and environment.
func buildDoorSubstitutions(currentUser *user.User, nodeNumber int, sessionStartTime time.Time) map[string]string {
	nodeNumStr := strconv.Itoa(nodeNumber)
	portStr := nodeNumStr // Use node number as port (COM ports aren't directly applicable for SSH)

	// Calculate actual Time Left
	elapsedMinutes := int(time.Since(sessionStartTime).Minutes())
	remainingMinutes := currentUser.TimeLimit - elapsedMinutes
	if remainingMinutes < 0 {
		remainingMinutes = 0 // Prevent negative time left
	}
	timeLeftStr := strconv.Itoa(remainingMinutes)

	// TODO: Get actual Baud Rate - using placeholder
	baudStr := "38400"
	userIDStr := strconv.Itoa(currentUser.ID)

	return map[string]string{
		"{NODE}":       nodeNumStr,
		"{PORT}":       portStr,
		"{TIMELEFT}":   timeLeftStr,
		"{BAUD}":       baudStr,
		"{USERHANDLE}": currentUser.Handle,
		"{USERID}":     userIDStr,
		"{REALNAME}":   currentUser.RealName,
		"{LEVEL}":      strconv.Itoa(currentUser.AccessLevel),
	}
}

// substituteInArgs applies placeholder substitutions to command arguments.
func substituteInArgs(args []string, substitutions map[string]string) []string {
	substitutedArgs := make([]string, len(args))
	for i, arg := range args {
		newArg := arg
		for key, val := range substitutions {
			newArg = strings.ReplaceAll(newArg, key, val)
		}
		substitutedArgs[i] = newArg
	}
	return substitutedArgs
}

// substituteInEnvVars applies placeholder substitutions to environment variables.
func substituteInEnvVars(envVars map[string]string, substitutions map[string]string) map[string]string {
	substitutedEnv := make(map[string]string)
	for key, val := range envVars {
		newVal := val
		for subKey, subVal := range substitutions {
			newVal = strings.ReplaceAll(newVal, subKey, subVal)
		}
		substitutedEnv[key] = newVal
	}
	return substitutedEnv
}

// generateDropfile creates the appropriate dropfile (DOOR.SYS or CHAIN.TXT) if configured.
// Returns the dropfile path, a cleanup function, and any error.
func generateDropfile(doorConfig config.DoorConfig, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, substitutions map[string]string) (string, func(), error) {

	dropfileDir := "." // Default to current dir if no WorkingDirectory
	if doorConfig.WorkingDirectory != "" {
		dropfileDir = doorConfig.WorkingDirectory
	}

	dropfileTypeUpper := strings.ToUpper(doorConfig.DropfileType)

	if dropfileTypeUpper != "DOOR.SYS" && dropfileTypeUpper != "CHAIN.TXT" {
		return "", nil, nil // No dropfile needed
	}

	dropfileName := dropfileTypeUpper
	dropfilePath := filepath.Join(dropfileDir, dropfileName)
	log.Printf("INFO: Generating %s dropfile at: %s", dropfileName, dropfilePath)

	var content strings.Builder

	nodeNumStr := substitutions["{NODE}"]
	baudStr := substitutions["{BAUD}"]
	timeLeftStr := substitutions["{TIMELEFT}"]
	userIDStr := substitutions["{USERID}"]

	if dropfileTypeUpper == "DOOR.SYS" {
		// Simplified Text-based DOOR.SYS
		content.WriteString(fmt.Sprintf("%s\r\n", "1"))                     // COM Port (Placeholder)
		content.WriteString(fmt.Sprintf("%s\r\n", baudStr))                 // Baud Rate
		content.WriteString(fmt.Sprintf("%s\r\n", "N"))                     // Parity (Placeholder)
		content.WriteString(fmt.Sprintf("%s\r\n", nodeNumStr))              // Node Number
		content.WriteString(fmt.Sprintf("%s\r\n", currentUser.Handle))      // User Name
		content.WriteString(fmt.Sprintf("%d\r\n", currentUser.AccessLevel)) // Security Level
		content.WriteString(fmt.Sprintf("%s\r\n", timeLeftStr))             // Time Left
		content.WriteString(fmt.Sprintf("%s\r\n", "ANSI"))                  // Emulation (Placeholder)
		content.WriteString(fmt.Sprintf("%s\r\n", userIDStr))               // User ID
	} else { // CHAIN.TXT
		content.WriteString(fmt.Sprintf("%s\r\n", currentUser.Handle))
		content.WriteString(fmt.Sprintf("%d\r\n", currentUser.AccessLevel))
		content.WriteString(fmt.Sprintf("%s\r\n", timeLeftStr))
		content.WriteString(fmt.Sprintf("%s\r\n", userIDStr))
	}

	err := os.WriteFile(dropfilePath, []byte(content.String()), 0644)
	if err != nil {
		log.Printf("ERROR: Failed to write dropfile %s: %v", dropfilePath, err)
		return "", nil, err
	}

	// Return cleanup function
	cleanupFunc := func() {
		logging.Debug(" Cleaning up dropfile: %s", dropfilePath)
		if removeErr := os.Remove(dropfilePath); removeErr != nil {
			log.Printf("WARN: Failed to remove dropfile %s: %v", dropfilePath, removeErr)
		}
	}

	return dropfilePath, cleanupFunc, nil
}

// buildDoorEnvironment creates the environment variable slice for the door command.
func buildDoorEnvironment(substitutedEnv map[string]string, substitutions map[string]string) []string {
	env := os.Environ()

	// Add configured environment variables
	for key, val := range substitutedEnv {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	// Build map of existing env var names for quick lookup
	envMap := make(map[string]bool)
	for _, envPair := range env {
		envMap[strings.SplitN(envPair, "=", 2)[0]] = true
	}

	// Add standard BBS env vars if not already present
	if _, exists := envMap["BBS_USERHANDLE"]; !exists {
		env = append(env, fmt.Sprintf("BBS_USERHANDLE=%s", substitutions["{USERHANDLE}"]))
	}
	if _, exists := envMap["BBS_USERID"]; !exists {
		env = append(env, fmt.Sprintf("BBS_USERID=%s", substitutions["{USERID}"]))
	}
	if _, exists := envMap["BBS_NODE"]; !exists {
		env = append(env, fmt.Sprintf("BBS_NODE=%s", substitutions["{NODE}"]))
	}
	if _, exists := envMap["BBS_TIMELEFT"]; !exists {
		env = append(env, fmt.Sprintf("BBS_TIMELEFT=%s", substitutions["{TIMELEFT}"]))
	}

	return env
}

// executeDoorWithPTY runs the door command using a PTY for raw terminal mode.
func executeDoorWithPTY(cmd *exec.Cmd, s ssh.Session, ptyReq ssh.Pty,
	windowChangeChannel <-chan ssh.Window, nodeNumber int, doorName string) error {

	log.Printf("INFO: Node %d: Starting door '%s' with PTY/Raw mode", nodeNumber, doorName)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start pty for door '%s': %w", doorName, err)
	}
	defer func() { _ = ptmx.Close() }()

	s.Signals(nil)
	s.Break(nil)

	// Handle window size changes
	go func() {
		if ptyReq.Window.Width > 0 || ptyReq.Window.Height > 0 {
			_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(ptyReq.Window.Height), Cols: uint16(ptyReq.Window.Width)})
		}
		for win := range windowChangeChannel {
			_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(win.Height), Cols: uint16(win.Width)})
		}
	}()

	// Set terminal to raw mode using PTY fd
	fd := int(ptmx.Fd())
	var restoreTerminal func()
	originalState, err := term.MakeRaw(fd)
	if err != nil {
		log.Printf("WARN: Node %d: Failed to put PTY into raw mode for door '%s': %v.", nodeNumber, doorName, err)
	} else {
		logging.Debug(" Node %d: PTY set to raw mode for door '%s'.", nodeNumber, doorName)
		restoreTerminal = func() {
			logging.Debug(" Node %d: Restoring PTY mode after door '%s'.", nodeNumber, doorName)
			if restoreErr := term.Restore(fd, originalState); restoreErr != nil {
				log.Printf("ERROR: Node %d: Failed to restore PTY state after door '%s': %v", nodeNumber, doorName, restoreErr)
			}
		}
		defer restoreTerminal()
	}

	// Set up I/O copying using goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, copyErr := io.Copy(ptmx, s)
		if copyErr != nil && copyErr != io.EOF && !errors.Is(copyErr, os.ErrClosed) {
			log.Printf("WARN: Node %d: Error copying session stdin to PTY for door '%s': %v", nodeNumber, doorName, copyErr)
		}
		logging.Debug(" Node %d: Finished copying session stdin to PTY for door '%s'", nodeNumber, doorName)
	}()

	go func() {
		defer wg.Done()
		_, copyErr := io.Copy(s, ptmx)
		if copyErr != nil && copyErr != io.EOF && !errors.Is(copyErr, os.ErrClosed) {
			log.Printf("WARN: Node %d: Error copying PTY stdout to session stdout for door '%s': %v", nodeNumber, doorName, copyErr)
		}
		logging.Debug(" Node %d: Finished copying PTY stdout to session stdout for door '%s'", nodeNumber, doorName)
	}()

	wg.Wait()
	logging.Debug(" Node %d: I/O copying finished for door '%s'. Waiting for command completion.", nodeNumber, doorName)

	return cmd.Wait()
}

// executeDoorWithStdIO runs the door command using standard I/O redirection.
func executeDoorWithStdIO(cmd *exec.Cmd, s ssh.Session, nodeNumber int, doorName string) error {
	log.Printf("INFO: Node %d: Starting door '%s' with standard I/O redirection", nodeNumber, doorName)
	cmd.Stdout = s
	cmd.Stderr = s
	cmd.Stdin = s
	return cmd.Run()
}

// handleDoorResult processes the result of door execution and displays appropriate messages.
func handleDoorResult(commandErr error, terminal *terminalPkg.BBS, s ssh.Session,
	nodeNumber int, userHandle string, doorName string) {

	if commandErr != nil {
		log.Printf("ERROR: Node %d: Door command execution failed for user %s, door %s: %v", nodeNumber, userHandle, doorName, commandErr)
		errMsg := fmt.Sprintf("\r\n|12Error running external program '%s': %v\r\nPress Enter to continue...\r\n", doorName, commandErr)
		_ = terminal.DisplayContent([]byte(errMsg)) // Error display, ignore write error
	} else {
		log.Printf("INFO: Node %d: Door command completed for user %s, door %s", nodeNumber, userHandle, doorName)
		_ = terminal.DisplayContent([]byte("\r\n|07Press |15[ENTER]|07 to return to the menu... ")) // Prompt, ignore write error

		// Wait for Enter key press using the session reader
		bufioReader := bufio.NewReader(s)
		for {
			r, _, readErr := bufioReader.ReadRune()
			if readErr != nil {
				log.Printf("ERROR: Failed reading input after door '%s': %v", doorName, readErr)
				break
			}
			if r == '\r' || r == '\n' {
				break
			}
			// Ignore other characters
		}
	}
}
