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
	terminalPkg "github.com/stlalpha/vision3/internal/terminal"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// RunnableFunc defines the signature for functions executable via RUN:
// Returns: authenticatedUser, nextAction (e.g., "GOTO:MENU"), err
type RunnableFunc func(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (authenticatedUser *user.User, nextAction string, err error)

// registerPlaceholderRunnables registers placeholder commands that are not yet fully implemented.
func registerPlaceholderRunnables(registry map[string]RunnableFunc) {
	// Keep READMAIL as a placeholder for now
	registry["READMAIL"] = func(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
		if currentUser == nil {
			log.Printf("WARN: Node %d: READMAIL called without logged in user.", nodeNumber)
			msg := "\r\n|01Error: You must be logged in to read mail.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil {
				log.Printf("ERROR: Failed writing READMAIL error message: %v", wErr)
			}
			time.Sleep(1 * time.Second)
			return nil, "", nil // No user change, no next action, no error
		}
		msg := fmt.Sprintf("\r\n|15Executing |11READMAIL|15 for |14%s|15... (Not Implemented)|07\r\n", currentUser.Handle)
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil {
			log.Printf("ERROR: Failed writing READMAIL placeholder message: %v", wErr)
		}
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil // No user change, no next action, no error
	}

	// Register DOOR handler
	registry["DOOR:"] = func(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, doorName string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
		if currentUser == nil {
			log.Printf("WARN: Node %d: DOOR:%s called without logged in user.", nodeNumber, doorName)
			msg := "\r\n|01Error: You must be logged in to run doors.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil {
				log.Printf("ERROR: Failed writing DOOR error message (not logged in): %v", wErr)
			}
			return nil, "", nil // Stay in menu loop
		}
		log.Printf("INFO: Node %d: User %s attempting to run door: %s", nodeNumber, currentUser.Handle, doorName)

		// 1. Look up door configuration
		doorConfig, exists := e.DoorRegistry[strings.ToUpper(doorName)] // Ensure lookup is case-insensitive
		if !exists {
			log.Printf("WARN: Door configuration not found for '%s'", doorName)
			errMsg := fmt.Sprintf("\r\n|12Error: Door '%s' is not configured.\r\nPress Enter to continue...\r\n", doorName)
			// Use WriteProcessedBytes for error message, writing to stderr
			// Need outputMode... assume CP437 for errors?
			wErr := terminal.DisplayContent([]byte(errMsg))
			if wErr != nil {
				log.Printf("ERROR: Failed writing DOOR error message (not configured) to stderr: %v", wErr)
			}
			// Wait for Enter? For now, let the menu loop handle redraw.
			return nil, "", nil // Stay in menu loop
		}

		// Gather data for substitution
		// Use the assigned node number
		nodeNumStr := strconv.Itoa(nodeNumber)
		// Set {PORT} based on the conceptual Node, as COM ports aren't directly applicable.
		portStr := nodeNumStr
		/* // Keep remote port extraction logic commented out for reference if needed later
		// Attempt to get remote port
		portStr := "?" // Default if extraction fails
		if remoteAddr := s.RemoteAddr(); remoteAddr != nil {
			addrStr := remoteAddr.String()
			if parts := strings.Split(addrStr, ":"); len(parts) > 1 {
				portStr = parts[len(parts)-1] // Get the last part as port
			}
		}
		*/
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

		substitutions := map[string]string{
			"{NODE}":       nodeNumStr,
			"{PORT}":       portStr,
			"{TIMELEFT}":   timeLeftStr,
			"{BAUD}":       baudStr,
			"{USERHANDLE}": currentUser.Handle,
			"{USERID}":     userIDStr,
			// Add more common placeholders if needed
			"{REALNAME}": currentUser.RealName,
			"{LEVEL}":    strconv.Itoa(currentUser.AccessLevel),
		}

		// Substitute in Arguments
		substitutedArgs := make([]string, len(doorConfig.Args))
		for i, arg := range doorConfig.Args {
			newArg := arg
			for key, val := range substitutions {
				newArg = strings.ReplaceAll(newArg, key, val)
			}
			substitutedArgs[i] = newArg
		}

		// Substitute in Environment Variables
		substitutedEnv := make(map[string]string)
		if doorConfig.EnvironmentVars != nil {
			for key, val := range doorConfig.EnvironmentVars {
				newVal := val
				for subKey, subVal := range substitutions {
					newVal = strings.ReplaceAll(newVal, subKey, subVal)
				}
				substitutedEnv[key] = newVal
			}
		}
		// --- End Placeholder Substitution ---

		// --- Dropfile Generation ---
		var dropfilePath string
		dropfileDir := "." // Default to current dir if no WorkingDirectory
		if doorConfig.WorkingDirectory != "" {
			dropfileDir = doorConfig.WorkingDirectory
		}

		dropfileTypeUpper := strings.ToUpper(doorConfig.DropfileType)

		if dropfileTypeUpper == "DOOR.SYS" || dropfileTypeUpper == "CHAIN.TXT" {
			dropfileName := dropfileTypeUpper // Use the standardized uppercase name
			dropfilePath = filepath.Join(dropfileDir, dropfileName)
			log.Printf("INFO: Generating %s dropfile at: %s", dropfileName, dropfilePath)

			var content strings.Builder
			if dropfileTypeUpper == "DOOR.SYS" {
				// Simplified Text-based DOOR.SYS
				// TODO: Use actual COM/Port info if available
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
				errMsg := fmt.Sprintf("\r\n|12Error creating system file for door '%s'.\r\nPress Enter to continue...\r\n", doorName)
				terminal.DisplayContent([]byte(errMsg))
				return nil, "", nil // Abort door execution
			}

			// Ensure dropfile is cleaned up
			defer func() {
				log.Printf("DEBUG: Cleaning up dropfile: %s", dropfilePath)
				if err := os.Remove(dropfilePath); err != nil {
					log.Printf("WARN: Failed to remove dropfile %s: %v", dropfilePath, err)
				}
			}()
		}
		// --- End Dropfile Generation ---

		// 2. Prepare the command (shared part)
		cmd := exec.Command(doorConfig.Command, substitutedArgs...)

		// 3. Set working directory if specified
		if doorConfig.WorkingDirectory != "" {
			cmd.Dir = doorConfig.WorkingDirectory
			log.Printf("DEBUG: Setting working directory for door '%s' to '%s'", doorName, cmd.Dir)
		}

		// 4. Set environment variables
		// Start with current environment, then add/override from config
		cmd.Env = os.Environ()
		if len(substitutedEnv) > 0 {
			for key, val := range substitutedEnv {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
			}
		}
		// Optionally add standard BBS env vars if not present (e.g., BBS_NODE)
		// Add our substituted standard vars if they weren't in the config
		envMap := make(map[string]bool)
		for _, envPair := range cmd.Env {
			envMap[strings.SplitN(envPair, "=", 2)[0]] = true
		}
		if _, exists := envMap["BBS_USERHANDLE"]; !exists {
			cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_USERHANDLE=%s", currentUser.Handle))
		}
		if _, exists := envMap["BBS_USERID"]; !exists {
			cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_USERID=%s", userIDStr))
		}
		if _, exists := envMap["BBS_NODE"]; !exists {
			cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_NODE=%s", nodeNumStr))
		}
		if _, exists := envMap["BBS_TIMELEFT"]; !exists {
			cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_TIMELEFT=%s", timeLeftStr))
		}
		// Add others like BBS_LEVEL, BBS_PORT, BBS_BAUD?

		// --- Execute Command (Handles PTY/Raw mode) ---
		ptyReq, winChOrig, isPty := s.Pty() // Get original PTY request and window change channel
		var cmdErr error

		if doorConfig.RequiresRawTerminal && isPty {
			// --- PTY / Raw Mode Execution --- Reverted to original logic ---
			log.Printf("INFO: Node %d: Starting door '%s' with PTY/Raw mode", nodeNumber, doorName)
			ptmx, err := pty.Start(cmd)
			if err != nil {
				cmdErr = fmt.Errorf("failed to start pty for door '%s': %w", doorName, err)
			} else {
				defer func() { _ = ptmx.Close() }()

				s.Signals(nil)
				s.Break(nil)
				go func() {
					if ptyReq.Window.Width > 0 || ptyReq.Window.Height > 0 {
						pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(ptyReq.Window.Height), Cols: uint16(ptyReq.Window.Width)})
					}
					// Use the ORIGINAL winChOrig from s.Pty()
					for win := range winChOrig {
						pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(win.Height), Cols: uint16(win.Width)})
					}
				}()

				// Set terminal to raw mode using PTY fd
				fd := int(ptmx.Fd()) // Use the PTY file descriptor
				var restoreTerminal func()
				originalState, err := term.MakeRaw(fd)
				if err != nil {
					log.Printf("WARN: Node %d: Failed to put PTY into raw mode for door '%s': %v.", nodeNumber, doorName, err)
				} else {
					log.Printf("DEBUG: Node %d: PTY set to raw mode for door '%s'.", nodeNumber, doorName)
					restoreTerminal = func() {
						log.Printf("DEBUG: Node %d: Restoring PTY mode after door '%s'.", nodeNumber, doorName)
						if err := term.Restore(fd, originalState); err != nil {
							log.Printf("ERROR: Node %d: Failed to restore PTY state after door '%s': %v", nodeNumber, doorName, err)
						}
					}
					defer restoreTerminal()
				}

				// Set up I/O copying using goroutines
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					_, err := io.Copy(ptmx, s)
					if err != nil && err != io.EOF && !errors.Is(err, os.ErrClosed) {
						log.Printf("WARN: Node %d: Error copying session stdin to PTY for door '%s': %v", nodeNumber, doorName, err)
					}
					log.Printf("DEBUG: Node %d: Finished copying session stdin to PTY for door '%s'", nodeNumber, doorName)
				}()
				go func() {
					defer wg.Done()
					_, err := io.Copy(s, ptmx)
					if err != nil && err != io.EOF && !errors.Is(err, os.ErrClosed) {
						log.Printf("WARN: Node %d: Error copying PTY stdout to session stdout for door '%s': %v", nodeNumber, doorName, err)
					}
					log.Printf("DEBUG: Node %d: Finished copying PTY stdout to session stdout for door '%s'", nodeNumber, doorName)
				}()

				wg.Wait()
				log.Printf("DEBUG: Node %d: I/O copying finished for door '%s'. Waiting for command completion.", nodeNumber, doorName)
				cmdErr = cmd.Wait()
			}
		} else {
			// --- Standard I/O Execution ---
			if doorConfig.RequiresRawTerminal && !isPty {
				log.Printf("WARN: Node %d: Door '%s' requires raw terminal, but no PTY was allocated. Door may not function correctly.", nodeNumber, doorName)
			}
			log.Printf("INFO: Node %d: Starting door '%s' with standard I/O redirection", nodeNumber, doorName)
			cmd.Stdout = s
			cmd.Stderr = s
			cmd.Stdin = s
			cmdErr = cmd.Run()
		}
		// --- End Command Execution ---

		// 7. Handle result (using cmdErr)
		if cmdErr != nil {
			log.Printf("ERROR: Node %d: Door command execution failed for user %s, door %s: %v", nodeNumber, currentUser.Handle, doorName, cmdErr)
			// Send error message to user terminal
			errMsg := fmt.Sprintf("\r\n|12Error running external program '%s': %v\r\nPress Enter to continue...\r\n", doorName, cmdErr)
			terminal.DisplayContent([]byte(errMsg)) // Display error message
			// Wait for Enter? Not needed as Run/Wait blocks.
		} else {
			log.Printf("INFO: Node %d: Door command completed for user %s, door %s", nodeNumber, currentUser.Handle, doorName)
			// Optionally clear screen or show a returning message?
			// fmt.Fprintf(s, "\r\n|07Returning to menu...\r\n")
			// Need a slight pause or prompt, otherwise menu redraws instantly over door output
			terminal.DisplayContent([]byte("\r\n|07Press |15[ENTER]|07 to return to the menu... ")) // Display prompt message
			// Wait for Enter key press using the session reader
			bufioReader := bufio.NewReader(s)
			for {
				r, _, readErr := bufioReader.ReadRune()
				if readErr != nil {
					log.Printf("ERROR: Failed reading input after door '%s': %v", doorName, readErr)
					break // Exit loop on error (e.g., disconnect)
				}
				if r == '\r' || r == '\n' { // Check for Enter (CR or LF)
					break
				}
				// Ignore other characters
			}
		}

		// --- Cleanup Dropfiles --- (Handled by defer)

		return nil, "", nil // Return nil user, "", nil error to continue menu loop after door
	}
}

// registerAppRunnables registers the actual application command functions.
func registerAppRunnables(registry map[string]RunnableFunc) {
	registry["SHOWSTATS"] = runShowStats
	registry["LASTCALLERS"] = runLastCallers
	registry["AUTHENTICATE"] = runAuthenticate
	registry["ONELINER"] = runOneliners                              // Register new placeholder
	registry["FULL_LOGIN_SEQUENCE"] = runFullLoginSequence           // Register the new sequence
	registry["SHOWVERSION"] = runShowVersion                         // Register the version display runnable
	registry["LISTUSERS"] = runListUsers                             // Register the user list runnable
	registry["LISTMSGAR"] = runListMessageAreas                      // <-- ADDED: Register message area list runnable
	registry["COMPOSEMSG"] = runComposeMessage                       // <-- ADDED: Register compose message runnable
	registry["PROMPTANDCOMPOSEMESSAGE"] = runPromptAndComposeMessage // <-- ADDED: Register prompt/compose runnable (Corrected key to uppercase)
	registry["READMSGS"] = runReadMsgs                               // <-- ADDED: Register message reading runnable
	registry["NEWSCAN"] = runNewscan                                 // <-- ADDED: Register newscan runnable
	registry["LISTFILES"] = runListFiles                             // <-- ADDED: Register file list runnable
	registry["LISTFILEAR"] = runListFileAreas                        // <-- ADDED: Register file area list runnable
	registry["SELECTFILEAREA"] = runSelectFileArea                   // <-- ADDED: Register file area selection runnable
	registry["SETRENDER"] = runSetRender                             // Register renderer configuration command
}
