//go:build !windows

package menu

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// --- Batch File Generator ---

// writeBatchFile generates EXTERNAL.BAT for dosemu2 execution.
// driveCNodeDir is the Linux path to the per-node temp directory inside drive_c.
func writeBatchFile(ctx *DoorCtx, batchPath, driveCNodeDir string) error {
	log.Printf("INFO: Writing batch file: %s", batchPath)
	crlf := "\r\n"

	var b strings.Builder
	b.WriteString("@echo off" + crlf)
	// Map D: drive to the node's temp directory on the host filesystem
	b.WriteString(fmt.Sprintf("@lredir -f D: %s >NUL", driveCNodeDir) + crlf)
	b.WriteString("c:" + crlf)

	for _, cmd := range ctx.Config.DOSCommands {
		processed := strings.ReplaceAll(cmd, "{NODE}", ctx.NodeNumStr)
		for key, val := range ctx.Subs {
			processed = strings.ReplaceAll(processed, key, val)
		}
		b.WriteString(processed + crlf)
	}

	b.WriteString("exitemu" + crlf)

	return os.WriteFile(batchPath, []byte(b.String()), 0600)
}

// --- DOS Door Executor ---

// executeDOSDoor launches a DOS door program via dosemu2.
// Uses manual PTY setup: dosemu's stdin is a PTY slave (providing a real TTY
// for COM1 via "serial { virtual com 1 }"), stdout/stderr go to /dev/null
// (hiding boot text), and door I/O flows through COM1 → PTY as raw CP437.
func executeDOSDoor(ctx *DoorCtx) error {
	// Determine drive_c path
	driveCPath := ctx.Config.DriveCPath
	if driveCPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		driveCPath = filepath.Join(homeDir, ".dosemu", "drive_c")
	}

	// Determine dosemu binary path
	dosemuPath := ctx.Config.DosemuPath
	if dosemuPath == "" {
		dosemuPath = "/usr/bin/dosemu"
	}

	// Create per-node temp directory inside drive_c
	nodeDir := fmt.Sprintf("temp%d", ctx.NodeNumber)
	nodePath := filepath.Join(driveCPath, "nodes", nodeDir)
	if err := os.MkdirAll(nodePath, 0700); err != nil {
		return fmt.Errorf("failed to create node directory %s: %w", nodePath, err)
	}

	// Generate all dropfiles
	if err := generateAllDropfiles(ctx, nodePath); err != nil {
		return fmt.Errorf("failed to generate dropfiles: %w", err)
	}
	defer cleanupDropfiles(nodePath)

	// Write batch file
	batchPath := filepath.Join(nodePath, "EXTERNAL.BAT")
	if err := writeBatchFile(ctx, batchPath, nodePath); err != nil {
		return fmt.Errorf("failed to write batch file: %w", err)
	}

	// Build dosemu command
	dosBatchPath := fmt.Sprintf("C:\\NODES\\%s\\EXTERNAL.BAT", strings.ToUpper(nodeDir))
	logPath := filepath.Join(nodePath, "dosemu_boot.log")

	args := []string{
		"-I", "video { none }",
		"-I", "serial { virtual com 1 }",
		"-E", dosBatchPath,
		"-o", logPath,
	}

	// Add custom config file if specified
	if ctx.Config.DosemuConfig != "" {
		args = append([]string{"-f", ctx.Config.DosemuConfig}, args...)
	}

	log.Printf("INFO: Node %d: Launching DOS door '%s' via dosemu2: %s %v", ctx.NodeNumber, ctx.DoorName, dosemuPath, args)

	cmd := exec.Command(dosemuPath, args...)
	cmd.Dir = driveCPath
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "DOSEMU_QUIET=1")

	// Create PTY pair manually — dosemu needs a real TTY for virtual COM1.
	// The PTY slave becomes the controlling terminal; COM1 I/O flows through it.
	// stdout/stderr go to /dev/null so boot text is hidden from the user.
	ptmx, tty, err := pty.Open()
	if err != nil {
		return fmt.Errorf("failed to open pty for dosemu2: %w", err)
	}

	// Set PTY size to 80x25 (standard DOS)
	pty.Setsize(ptmx, &pty.Winsize{Rows: 25, Cols: 80})

	// dosemu stdin = PTY slave (provides the controlling terminal for COM1)
	cmd.Stdin = tty

	// dosemu stdout/stderr → /dev/null (hides boot messages)
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		ptmx.Close()
		tty.Close()
		return fmt.Errorf("failed to open /dev/null: %w", err)
	}
	cmd.Stdout = devNull
	cmd.Stderr = devNull

	// Make the PTY slave the controlling terminal for dosemu.
	// FD 0 (stdin = PTY slave) becomes the controlling terminal,
	// which is what "serial { virtual com 1 }" connects to.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0, // child FD 0 = stdin = PTY slave
	}

	// Start dosemu
	if err := cmd.Start(); err != nil {
		ptmx.Close()
		tty.Close()
		devNull.Close()
		return fmt.Errorf("failed to start dosemu2: %w", err)
	}

	// Parent no longer needs the slave side or /dev/null
	tty.Close()
	devNull.Close()

	// Set PTY master to raw mode for clean passthrough of CP437 bytes
	fd := int(ptmx.Fd())
	if oldState, err := term.MakeRaw(fd); err == nil {
		defer term.Restore(fd, oldState)
	}

	// Set up a read interrupt so we can cleanly stop the input goroutine
	// when the door exits, preventing it from consuming the next keypress.
	// Sessions that support SetReadInterrupt (SSH) get clean cancellation;
	// others (telnet) fall back to the old behavior where the input goroutine
	// exits on its own when ptmx is closed and the next write fails.
	readInterrupt := make(chan struct{})
	hasInterrupt := false
	if ri, ok := ctx.Session.(interface{ SetReadInterrupt(<-chan struct{}) }); ok {
		ri.SetReadInterrupt(readInterrupt)
		defer ri.SetReadInterrupt(nil)
		hasInterrupt = true
	}

	// Bidirectional I/O: SSH session ↔ PTY master (COM1 data)
	inputDone := make(chan struct{})
	outputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		_, err := io.Copy(ptmx, ctx.Session)
		if err != nil && err != io.EOF && !errors.Is(err, os.ErrClosed) {
			log.Printf("WARN: Node %d: Error copying session stdin to dosemu PTY: %v", ctx.NodeNumber, err)
		}
	}()
	go func() {
		defer close(outputDone)
		_, err := io.Copy(ctx.Session, ptmx)
		if err != nil && err != io.EOF && !errors.Is(err, os.ErrClosed) {
			log.Printf("WARN: Node %d: Error copying dosemu PTY to session: %v", ctx.NodeNumber, err)
		}
	}()

	// Wait for dosemu to exit, then cleanly shut down I/O goroutines
	cmdErr := cmd.Wait()
	log.Printf("DEBUG: Node %d: dosemu2 process exited for door '%s'", ctx.NodeNumber, ctx.DoorName)

	// Interrupt the input goroutine's blocked Read() so it exits without
	// consuming the user's next keypress, then close the PTY.
	close(readInterrupt)
	if hasInterrupt {
		<-inputDone
	}
	ptmx.Close()
	<-outputDone

	if cmdErr != nil {
		log.Printf("ERROR: Node %d: DOS door '%s' failed: %v", ctx.NodeNumber, ctx.DoorName, cmdErr)
		return cmdErr
	}

	log.Printf("INFO: Node %d: DOS door '%s' completed successfully", ctx.NodeNumber, ctx.DoorName)
	return nil
}

// --- Native Door Executor ---

// executeNativeDoor runs a native (non-DOS) door program.
// This is extracted from the original inline DOOR: handler in executor.go.
func executeNativeDoor(ctx *DoorCtx) error {
	doorConfig := ctx.Config

	// Substitute in Arguments
	substitutedArgs := make([]string, len(doorConfig.Args))
	for i, arg := range doorConfig.Args {
		newArg := arg
		for key, val := range ctx.Subs {
			newArg = strings.ReplaceAll(newArg, key, val)
		}
		substitutedArgs[i] = newArg
	}

	// Substitute in Environment Variables
	substitutedEnv := make(map[string]string)
	if doorConfig.EnvironmentVars != nil {
		for key, val := range doorConfig.EnvironmentVars {
			newVal := val
			for subKey, subVal := range ctx.Subs {
				newVal = strings.ReplaceAll(newVal, subKey, subVal)
			}
			substitutedEnv[key] = newVal
		}
	}

	// --- Dropfile Generation ---
	var dropfilePath string
	dropfileDir := "."
	if doorConfig.WorkingDirectory != "" {
		dropfileDir = doorConfig.WorkingDirectory
	}

	dropfileTypeUpper := strings.ToUpper(doorConfig.DropfileType)

	if dropfileTypeUpper == "DOOR.SYS" || dropfileTypeUpper == "CHAIN.TXT" || dropfileTypeUpper == "DOOR32.SYS" || dropfileTypeUpper == "DORINFO1.DEF" {
		dropfilePath = filepath.Join(dropfileDir, dropfileTypeUpper)
		log.Printf("INFO: Generating %s dropfile at: %s", dropfileTypeUpper, dropfilePath)

		// Use full-format dropfile generators (standard BBS formats)
		var genErr error
		switch dropfileTypeUpper {
		case "DOOR.SYS":
			genErr = generateDoorSys(ctx, dropfileDir)
		case "DOOR32.SYS":
			genErr = generateDoor32Sys(ctx, dropfileDir)
		case "CHAIN.TXT":
			genErr = generateChainTxt(ctx, dropfileDir)
		case "DORINFO1.DEF":
			genErr = generateDorInfo(ctx, dropfileDir)
		}

		if genErr != nil {
			log.Printf("ERROR: Failed to write dropfile %s: %v", dropfilePath, genErr)
			errMsg := fmt.Sprintf(ctx.Executor.LoadedStrings.DoorDropfileError, ctx.DoorName)
			if wErr := terminalio.WriteProcessedBytes(ctx.Session.Stderr(), ansi.ReplacePipeCodes([]byte(errMsg)), ctx.OutputMode); wErr != nil {
				log.Printf("ERROR: Failed writing dropfile creation error message: %v", wErr)
			}
			return genErr
		}

		defer func() {
			log.Printf("DEBUG: Cleaning up dropfile: %s", dropfilePath)
			if err := os.Remove(dropfilePath); err != nil {
				log.Printf("WARN: Failed to remove dropfile %s: %v", dropfilePath, err)
			}
		}()
	}

	// Prepare command
	cmd := exec.Command(doorConfig.Command, substitutedArgs...)

	if doorConfig.WorkingDirectory != "" {
		cmd.Dir = doorConfig.WorkingDirectory
		log.Printf("DEBUG: Setting working directory for door '%s' to '%s'", ctx.DoorName, cmd.Dir)
	}

	// Set environment variables
	cmd.Env = os.Environ()
	if len(substitutedEnv) > 0 {
		for key, val := range substitutedEnv {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
		}
	}

	// Add standard BBS env vars if not already present
	envMap := make(map[string]bool)
	for _, envPair := range cmd.Env {
		envMap[strings.SplitN(envPair, "=", 2)[0]] = true
	}
	if _, exists := envMap["BBS_USERHANDLE"]; !exists {
		cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_USERHANDLE=%s", ctx.User.Handle))
	}
	if _, exists := envMap["BBS_USERID"]; !exists {
		cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_USERID=%s", ctx.UserIDStr))
	}
	if _, exists := envMap["BBS_NODE"]; !exists {
		cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_NODE=%s", ctx.NodeNumStr))
	}
	if _, exists := envMap["BBS_TIMELEFT"]; !exists {
		cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_TIMELEFT=%s", ctx.TimeLeftStr))
	}

	// Set LINES and COLUMNS from user's saved preferences (for terminal size detection).
	// Remove any existing LINES/COLUMNS entries first to ensure our values take precedence.
	screenHeight := ctx.User.ScreenHeight
	if screenHeight <= 0 {
		screenHeight = 25
	}
	screenWidth := ctx.User.ScreenWidth
	if screenWidth <= 0 {
		screenWidth = 80
	}
	filteredEnv := make([]string, 0, len(cmd.Env))
	for _, e := range cmd.Env {
		if !strings.HasPrefix(e, "LINES=") && !strings.HasPrefix(e, "COLUMNS=") {
			filteredEnv = append(filteredEnv, e)
		}
	}
	cmd.Env = append(filteredEnv, fmt.Sprintf("LINES=%d", screenHeight), fmt.Sprintf("COLUMNS=%d", screenWidth))
	log.Printf("DEBUG: Node %d: Set door env LINES=%d COLUMNS=%d", ctx.NodeNumber, screenHeight, screenWidth)

	// Execute command
	_, winChOrig, isPty := ctx.Session.Pty()
	var cmdErr error

	if doorConfig.RequiresRawTerminal && isPty {
		log.Printf("INFO: Node %d: Starting door '%s' with PTY/Raw mode", ctx.NodeNumber, ctx.DoorName)

		// Set PTY size from user's saved preferences - BEFORE starting the command
		doorScreenHeight := uint16(25) // default
		if ctx.User.ScreenHeight > 0 && ctx.User.ScreenHeight <= 65535 {
			doorScreenHeight = uint16(ctx.User.ScreenHeight)
		}
		doorScreenWidth := uint16(80) // default
		if ctx.User.ScreenWidth > 0 && ctx.User.ScreenWidth <= 65535 {
			doorScreenWidth = uint16(ctx.User.ScreenWidth)
		}
		doorSize := &pty.Winsize{Rows: doorScreenHeight, Cols: doorScreenWidth}
		log.Printf("DEBUG: Node %d: Starting door with PTY size %dx%d (from user preferences)", ctx.NodeNumber, doorScreenWidth, doorScreenHeight)

		ptmx, err := pty.StartWithSize(cmd, doorSize)
		if err != nil {
			cmdErr = fmt.Errorf("failed to start pty for door '%s': %w", ctx.DoorName, err)
		} else {
			ctx.Session.Signals(nil)
			ctx.Session.Break(nil)

			// Drain window resize events but don't apply them - respect user's saved preferences.
			// resizeStop is closed after cmd.Wait() to prevent this goroutine from leaking.
			resizeStop := make(chan struct{})
			go func() {
				for {
					select {
					case win, ok := <-winChOrig:
						if !ok {
							return
						}
						log.Printf("DEBUG: Node %d: Ignoring SSH resize event %dx%d (keeping user preference %dx%d)",
							ctx.NodeNumber, win.Width, win.Height, doorScreenWidth, doorScreenHeight)
					case <-resizeStop:
						return
					}
				}
			}()

			fd := int(ptmx.Fd())
			originalState, err := term.MakeRaw(fd)
			if err != nil {
				log.Printf("WARN: Node %d: Failed to put PTY into raw mode for door '%s': %v.", ctx.NodeNumber, ctx.DoorName, err)
			} else {
				log.Printf("DEBUG: Node %d: PTY set to raw mode for door '%s'.", ctx.NodeNumber, ctx.DoorName)
			}
			needsRestore := (err == nil)

			// Set up a read interrupt so we can cleanly stop the input goroutine
			// when the door exits, preventing it from consuming the next keypress.
			// Sessions that support SetReadInterrupt (SSH) get clean cancellation;
			// others (telnet) fall back to the old behavior where the input goroutine
			// exits on its own when ptmx is closed and the next write fails.
			readInterrupt := make(chan struct{})
			hasInterrupt := false
			if ri, ok := ctx.Session.(interface{ SetReadInterrupt(<-chan struct{}) }); ok {
				ri.SetReadInterrupt(readInterrupt)
				defer ri.SetReadInterrupt(nil)
				hasInterrupt = true
			}

			inputDone := make(chan struct{})
			outputDone := make(chan struct{})
			go func() {
				defer close(inputDone)
				_, err := io.Copy(ptmx, ctx.Session)
				if err != nil && err != io.EOF && !errors.Is(err, os.ErrClosed) {
					// "read interrupted" is expected when we close readInterrupt during shutdown
					if strings.Contains(err.Error(), "read interrupted") {
						log.Printf("DEBUG: Node %d: Input goroutine interrupted for door '%s' (expected during shutdown)", ctx.NodeNumber, ctx.DoorName)
					} else {
						log.Printf("WARN: Node %d: Error copying session stdin to PTY for door '%s': %v", ctx.NodeNumber, ctx.DoorName, err)
					}
				}
			}()
			go func() {
				defer close(outputDone)
				_, err := io.Copy(ctx.Session, ptmx)
				if err != nil && err != io.EOF && !errors.Is(err, os.ErrClosed) {
					// "input/output error" on PTY is expected when closing during active read
					if strings.Contains(err.Error(), "input/output error") {
						log.Printf("DEBUG: Node %d: Output goroutine I/O error for door '%s' (expected during shutdown)", ctx.NodeNumber, ctx.DoorName)
					} else {
						log.Printf("WARN: Node %d: Error copying PTY stdout to session for door '%s': %v", ctx.NodeNumber, ctx.DoorName, err)
					}
				}
			}()

			// Wait for door to exit, then cleanly shut down I/O goroutines
			cmdErr = cmd.Wait()
			close(resizeStop)
			log.Printf("DEBUG: Node %d: Door '%s' process exited", ctx.NodeNumber, ctx.DoorName)

			// Interrupt the input goroutine's blocked Read() so it exits without
			// consuming the user's next keypress, then restore PTY state and close.
			close(readInterrupt)
			if hasInterrupt {
				<-inputDone
			}

			// Restore PTY state before closing the file descriptor
			if needsRestore {
				log.Printf("DEBUG: Node %d: Restoring PTY mode after door '%s'.", ctx.NodeNumber, ctx.DoorName)
				if err := term.Restore(fd, originalState); err != nil {
					log.Printf("ERROR: Node %d: Failed to restore PTY state after door '%s': %v", ctx.NodeNumber, ctx.DoorName, err)
				}
			}

			ptmx.Close()
			<-outputDone
		}
	} else {
		if doorConfig.RequiresRawTerminal && !isPty {
			log.Printf("WARN: Node %d: Door '%s' requires raw terminal, but no PTY was allocated.", ctx.NodeNumber, ctx.DoorName)
		}
		log.Printf("INFO: Node %d: Starting door '%s' with standard I/O redirection", ctx.NodeNumber, ctx.DoorName)

		cmd.Stdout = ctx.Session
		cmd.Stderr = ctx.Session
		cmd.Stdin = ctx.Session
		cmdErr = cmd.Run()

		// Brief delay to let terminal state settle and prevent double-keypress issues
		time.Sleep(100 * time.Millisecond)
	}

	return cmdErr
}

// --- Door Dispatcher ---

// executeDoor dispatches to the appropriate door executor based on config.
func executeDoor(ctx *DoorCtx) error {
	if ctx.Config.IsDOS {
		switch resolveDOSEmulator(ctx) {
		case "dosemu":
			return executeDOSDoor(ctx)
		default:
			return executeDOSBoxDoor(ctx)
		}
	}
	return executeNativeDoor(ctx)
}

// --- Door Post-Execution ---

// doorErrorMessage sends a formatted error message to the user.
func doorErrorMessage(ctx *DoorCtx, msg string) {
	errMsg := fmt.Sprintf(ctx.Executor.LoadedStrings.DoorErrorFormat, msg)
	wErr := terminalio.WriteProcessedBytes(ctx.Session.Stderr(), ansi.ReplacePipeCodes([]byte(errMsg)), ctx.OutputMode)
	if wErr != nil {
		log.Printf("ERROR: Failed writing door error message: %v", wErr)
	}
}

// --- Door Menu Runnables ---

// runListDoors displays a list of all configured doors.
func runListDoors(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running LISTDOORS", nodeNumber)

	if currentUser == nil {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.DoorLoginRequired)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Load templates
	topPath := filepath.Join(e.MenuSetPath, "templates", "DOORLIST.TOP")
	midPath := filepath.Join(e.MenuSetPath, "templates", "DOORLIST.MID")
	botPath := filepath.Join(e.MenuSetPath, "templates", "DOORLIST.BOT")

	topBytes, errTop := readTemplateFile(topPath)
	midBytes, errMid := readTemplateFile(midPath)
	botBytes, errBot := readTemplateFile(botPath)

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load DOORLIST templates: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.DoorTemplateError)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	// Display header
	// For CP437 mode, write raw bytes directly to avoid UTF-8 false positives
	processedTop := ansi.ReplacePipeCodes(topBytes)
	if outputMode == ansi.OutputModeCP437 {
		terminal.Write(processedTop)
	} else {
		terminalio.WriteProcessedBytes(terminal, processedTop, outputMode)
	}

	// Get door registry atomically
	e.configMu.RLock()
	doorRegistryCopy := make(map[string]config.DoorConfig, len(e.DoorRegistry))
	for k, v := range e.DoorRegistry {
		doorRegistryCopy[k] = v
	}
	e.configMu.RUnlock()

	// Sort door names for consistent display
	doorNames := make([]string, 0, len(doorRegistryCopy))
	for name := range doorRegistryCopy {
		doorNames = append(doorNames, name)
	}
	sort.Strings(doorNames)

	// Display each door
	midTemplate := string(ansi.ReplacePipeCodes(midBytes))
	for i, name := range doorNames {
		doorCfg := doorRegistryCopy[name]
		doorType := "Native"
		if doorCfg.IsDOS {
			doorType = "DOS"
		}

		line := midTemplate
		line = strings.ReplaceAll(line, "^ID", fmt.Sprintf("%-3d", i+1))
		line = strings.ReplaceAll(line, "^NA", fmt.Sprintf("%-30s", name))
		line = strings.ReplaceAll(line, "^TY", doorType)

		terminalio.WriteProcessedBytes(terminal, []byte(line), outputMode)
	}

	if len(doorNames) == 0 {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.DoorNoneConfigured)), outputMode)
	}

	// Display footer
	processedBot := ansi.ReplacePipeCodes(botBytes)
	if outputMode == ansi.OutputModeCP437 {
		terminal.Write(processedBot)
	} else {
		terminalio.WriteProcessedBytes(terminal, processedBot, outputMode)
	}

	return currentUser, "", nil
}

// runOpenDoor prompts the user for a door name and launches it.
func runOpenDoor(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running OPENDOOR", nodeNumber)

	if currentUser == nil {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.DoorLoginRequired)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Prompt for door name
	renderedPrompt := ansi.ReplacePipeCodes([]byte(e.LoadedStrings.DoorPrompt))
	curUpClear := "\x1b[A\r\x1b[2K"

	terminalio.WriteProcessedBytes(terminal, renderedPrompt, outputMode)

	for {
		inputName, err := readLineFromSessionIH(s, terminal)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Error reading OPENDOOR input: %v", nodeNumber, err)
			return currentUser, "", err
		}

		inputClean := strings.TrimSpace(inputName)
		upperInput := strings.ToUpper(inputClean)

		if upperInput == "Q" {
			terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
			return currentUser, "", nil
		}
		if upperInput == "" {
			terminalio.WriteProcessedBytes(terminal, renderedPrompt, outputMode)
			continue
		}

		if upperInput == "?" {
			runListDoors(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, "", outputMode, termWidth, termHeight)
			terminalio.WriteProcessedBytes(terminal, renderedPrompt, outputMode)
			continue
		}

		// Look up door
		doorConfig, exists := e.GetDoorConfig(upperInput)
		if !exists {
			terminalio.WriteProcessedBytes(terminal, []byte(curUpClear), outputMode)
			msg := fmt.Sprintf(e.LoadedStrings.DoorNotFoundFormat, inputClean)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(1 * time.Second)
			terminalio.WriteProcessedBytes(terminal, []byte("\r\x1b[2K"), outputMode)
			terminalio.WriteProcessedBytes(terminal, renderedPrompt, outputMode)
			continue
		}

		// Build context and execute
		ctx := buildDoorCtx(e, s, terminal,
			currentUser.ID, currentUser.Handle, currentUser.RealName,
			currentUser.AccessLevel, currentUser.TimeLimit, currentUser.TimesCalled,
			currentUser.PhoneNumber, currentUser.GroupLocation,
			termWidth, termHeight,
			nodeNumber, sessionStartTime, outputMode,
			doorConfig, upperInput)

		resetSessionIH(s)
		cmdErr := executeDoor(ctx)
		_ = getSessionIH(s)

		if cmdErr != nil {
			log.Printf("ERROR: Node %d: Door execution failed for user %s, door %s: %v", nodeNumber, currentUser.Handle, upperInput, cmdErr)
			doorErrorMessage(ctx, fmt.Sprintf("Error running door '%s': %v", upperInput, cmdErr))
		} else {
			log.Printf("INFO: Node %d: Door completed for user %s, door %s", nodeNumber, currentUser.Handle, upperInput)
		}

		return currentUser, "", nil
	}
}

// runDoorInfo displays information about a specific door.
func runDoorInfo(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running DOORINFO", nodeNumber)

	if currentUser == nil {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.DoorInfoLoginRequired)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Prompt for door name
	renderedPrompt := ansi.ReplacePipeCodes([]byte(e.LoadedStrings.DoorPrompt))
	curUpClear := "\x1b[A\r\x1b[2K"

	terminalio.WriteProcessedBytes(terminal, renderedPrompt, outputMode)

	for {
		inputName, err := readLineFromSessionIH(s, terminal)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Error reading DOORINFO input: %v", nodeNumber, err)
			return currentUser, "", err
		}

		inputClean := strings.TrimSpace(inputName)
		upperInput := strings.ToUpper(inputClean)

		if upperInput == "Q" {
			terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
			return currentUser, "", nil
		}
		if upperInput == "" {
			terminalio.WriteProcessedBytes(terminal, renderedPrompt, outputMode)
			continue
		}

		if upperInput == "?" {
			runListDoors(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, "", outputMode, termWidth, termHeight)
			terminalio.WriteProcessedBytes(terminal, renderedPrompt, outputMode)
			continue
		}

		// Look up door
		doorConfig, exists := e.GetDoorConfig(upperInput)
		if !exists {
			terminalio.WriteProcessedBytes(terminal, []byte(curUpClear), outputMode)
			msg := fmt.Sprintf(e.LoadedStrings.DoorNotFoundFormat, inputClean)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(1 * time.Second)
			terminalio.WriteProcessedBytes(terminal, []byte("\r\x1b[2K"), outputMode)
			terminalio.WriteProcessedBytes(terminal, renderedPrompt, outputMode)
			continue
		}

		// Display door info
		terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
		doorType := "Native Linux"
		if doorConfig.IsDOS {
			doorType = "DOS (dosemu2)"
		}

		info := fmt.Sprintf("|15Door: |07%s\r\n|15Type: |07%s\r\n", upperInput, doorType)
		if doorConfig.Command != "" {
			info += fmt.Sprintf("|15Command: |07%s\r\n", doorConfig.Command)
		}
		if doorConfig.WorkingDirectory != "" {
			info += fmt.Sprintf("|15Directory: |07%s\r\n", doorConfig.WorkingDirectory)
		}
		if doorConfig.DropfileType != "" {
			info += fmt.Sprintf("|15Dropfile: |07%s\r\n", doorConfig.DropfileType)
		}
		if doorConfig.IsDOS && len(doorConfig.DOSCommands) > 0 {
			info += fmt.Sprintf("|15DOS Commands: |07%s\r\n", strings.Join(doorConfig.DOSCommands, " && "))
		}

		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(info)), outputMode)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)

		return currentUser, "", nil
	}
}
