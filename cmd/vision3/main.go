package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh" // Alias standard crypto/ssh

	// Local packages (Update paths)
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/file"
	"github.com/stlalpha/vision3/internal/menu"
	"github.com/stlalpha/vision3/internal/message"
	"github.com/stlalpha/vision3/internal/session"
	"github.com/stlalpha/vision3/internal/terminal"
	"github.com/stlalpha/vision3/internal/types"
	"github.com/stlalpha/vision3/internal/user"
	// Needed for test code / shared types? Keep imports but ensure they are still needed.
	// Removed imports only used by test code
	// terminal "golang.org/x/crypto/ssh/terminal" // Alias to avoid conflict
	// "golang.org/x/text/encoding/charmap"
)

var (
	userMgr      *user.UserMgr
	messageMgr   *message.MessageManager
	fileMgr      *file.FileManager
	menuExecutor *menu.MenuExecutor
	// globalConfig *config.GlobalConfig // Still commented out
	nodeCounter         int32
	activeSessions      = make(map[ssh.Session]int32)
	activeSessionsMutex sync.Mutex
	loadedStrings       config.StringsConfig
	loadedTheme         config.ThemeConfig
	// colorTestMode       bool   // Flag variable REMOVED
	outputModeFlag string // Output mode flag (auto, utf8, cp437)
)

// --- ANSI Test Server Code REMOVED ---

// --- BBS sessionHandler (Original logic) ---
func sessionHandler(s ssh.Session) {
	nodeID := atomic.AddInt32(&nodeCounter, 1)
	remoteAddr := s.RemoteAddr().String()
	log.Printf("Node %d: Connection from %s (User: %s, Session ID: %s)", nodeID, remoteAddr, s.User(), s.Context().SessionID())

	// Add session to active sessions map
	activeSessionsMutex.Lock()
	activeSessions[s] = nodeID // Use session as key
	activeSessionsMutex.Unlock()

	// Capture start time and declare authenticatedUser *before* the defer
	capturedStartTime := time.Now()        // Capture start time close to session start
	var authenticatedUser *user.User = nil // Declare here so the closure can capture it

	// Defer removal from active sessions map and logging disconnection
	// The deferred function now uses a closure to access authenticatedUser
	defer func(startTime time.Time) {
		log.Printf("Node %d: Disconnected %s (User: %s)", nodeID, remoteAddr, s.User())
		activeSessionsMutex.Lock()
		delete(activeSessions, s) // Remove using session as key
		activeSessionsMutex.Unlock()

		// --- Record Call History ---
		if authenticatedUser != nil {
			log.Printf("DEBUG: Node %d: Adding call record for user %s (ID: %d)", nodeID, authenticatedUser.Handle, authenticatedUser.ID)
			disconnectTime := time.Now()
			duration := disconnectTime.Sub(startTime) // Use the captured startTime
			callRec := user.CallRecord{
				UserID:         authenticatedUser.ID,
				Handle:         authenticatedUser.Handle,
				GroupLocation:  authenticatedUser.GroupLocation,
				NodeID:         int(nodeID),
				ConnectTime:    startTime,
				DisconnectTime: disconnectTime,
				Duration:       duration,
				UploadedMB:     0.0,
				DownloadedMB:   0.0,
				Actions:        "",
				BaudRate:       "38400",
			}
			userMgr.AddCallRecord(callRec)
		} else {
			log.Printf("DEBUG: Node %d: No authenticated user found, skipping call record.", nodeID)
		}
		// ------------------------
		s.Close() // Ensure the session is closed
	}(capturedStartTime) // Pass only the startTime value

	// Create the session state object *early*
	sessionState := &session.BbsSession{
		// Conn:    s.Conn,     // Need the underlying gossh.Conn if possible, might need context
		Channel:    nil,         // Channel might not be directly available here, depends on gliderlabs/ssh context
		User:       nil,         // Set after authentication
		ID:         int(nodeID), // Use correct field name 'ID'
		StartTime:  time.Now(),  // Record session start time
		Pty:        nil,         // Will be set if/when PTY is granted
		AutoRunLog: make(types.AutoRunTracker),
	}

	// --- PTY Request Handling ---
	ptyReq, winCh, isPty := s.Pty() // Get PTY info from the original ssh.Session 's'
	if isPty {
		log.Printf("Node %d: PTY Request Accepted: %s", nodeID, ptyReq.Term)
		// Store PTY info in session state
		sessionState.Pty = &ptyReq // Store a pointer to the ptyReq
		sessionState.Width = ptyReq.Window.Width
		sessionState.Height = ptyReq.Window.Height
	} else {
		log.Printf("Node %d: No PTY Request received. Proceeding without PTY.", nodeID)
		// Handle non-PTY sessions gracefully, maybe just print a message and exit?
		// For a BBS, PTY is usually required.
		// fmt.Fprintln(s, "PTY is required for BBS access.")
		// return // Exit if no PTY? Or try to proceed?
	}

	// --- Determine Output Mode ---
	effectiveMode := terminal.OutputModeAuto // Start with Auto as the base
	switch outputModeFlag {              // Check the global flag first
	case "utf8":
		effectiveMode = terminal.OutputModeUTF8
		log.Printf("Node %d: Output mode forced to UTF-8 by flag.", nodeID)
	case "cp437":
		effectiveMode = terminal.OutputModeCP437
		log.Printf("Node %d: Output mode forced to CP437 by flag.", nodeID)
	case "auto":
		// Auto mode: Use PTY info if available
		if isPty {
			termType := strings.ToLower(ptyReq.Term)
			log.Printf("Node %d: Auto mode detecting based on TERM='%s'", nodeID, termType)
			// Heuristic: Check for known CP437-preferring TERM types
			if termType == "sync" || termType == "ansi" || termType == "scoansi" || strings.HasPrefix(termType, "vt100") {
				log.Printf("Node %d: Auto mode selecting CP437 output for TERM='%s'", nodeID, termType)
				effectiveMode = terminal.OutputModeCP437
			} else {
				log.Printf("Node %d: Auto mode selecting UTF-8 output for TERM='%s'", nodeID, termType)
				effectiveMode = terminal.OutputModeUTF8
			}
		} else {
			// No PTY, safer to default to UTF-8? Or CP437?
			// Let's default to UTF-8 for non-PTY as it's more common for raw streams.
			log.Printf("Node %d: Auto mode selecting UTF-8 output (no PTY requested).", nodeID)
			effectiveMode = terminal.OutputModeUTF8
		}
	}

	// --- Create Terminal ---
	log.Printf("Node %d: Creating terminal for session", nodeID)
	terminalInstance := terminal.NewBBS(s) // Use new simplified terminal API

	// --- Simple Test Output ---
	testMsg := "\r\n\x1b[31mSimple Test: RED\x1b[0m | \x1b[32mGREEN\x1b[0m | ASCII: Hello! 123?.,;\r\n"
	log.Printf("Node %d: Writing simple test message...", nodeID)
	_, testErr := terminalInstance.Write([]byte(testMsg))
	if testErr != nil {
		log.Printf("Node %d: Error writing test message: %v", nodeID, testErr)
	}
	log.Printf("Node %d: Finished writing simple test message.", nodeID)
	// ------------------------

	// --- Attempt PTY/Environment Negotiation ---
	// Send requests AFTER terminal is created, potentially influencing the PTY env
	if isPty {
		log.Printf("Node %d: PTY acquired. Attempting to configure environment.", nodeID)

		// Helper to send environment variables (RFC 4254 Section 6.4)
		sendEnv(s, "TERM", "xterm-256color")
		// Attempt to force UTF-8 locale (common variables, might not be respected by Windows SSHd)
		sendEnv(s, "LANG", "en_US.UTF-8")
		sendEnv(s, "LC_ALL", "en_US.UTF-8")
		sendEnv(s, "LC_CTYPE", "UTF-8")

		// Short delay to allow server to potentially process requests
		log.Printf("Node %d: Waiting briefly after sending env requests...", nodeID)
		time.Sleep(150 * time.Millisecond) // Slightly longer pause
		log.Printf("Node %d: Proceeding after environment configuration attempt.", nodeID)

	} else {
		log.Printf("Node %d: No PTY requested, skipping environment configuration.", nodeID)
	}
	// -------------------------------------------

	// --- Handle Window Size Changes ---
	// Need to start this gouroutine regardless of initial PTY request
	// because a PTY might be granted later or window size changes can occur
	go func() {
		for win := range winCh { // Loop until the channel is closed
			log.Printf("Node %d: Window resize event: %+v", nodeID, win)
			// TODO: Check if SetSize is implemented and safe to call
			// err := terminal.SetSize(win.Width, win.Height)
			// if err != nil {
			// 	log.Printf("Node %d: Error setting terminal size: %v", nodeID, err)
			// }
			// Store the latest window size if needed elsewhere
			// currentWidth = win.Width
			// currentHeight = win.Height

			// If you need to redraw the screen on resize, signal the main loop here
			// Example: send a special value on a channel, or set a flag
			// redrawSignal <- true
		}
		log.Printf("Node %d: Window change channel closed.", nodeID)
	}()

	// Attempt to set raw mode (might fail, proceed anyway)
	if isPty {
		// Original terminal state
		// Note: term.MakeRaw requires an Fd(). This might not work directly with
		// the ssh.Session object on all platforms, especially Windows without
		// a proper underlying file descriptor for the PTY.
		// originalState, err := term.MakeRaw(int(s.Pty().Fd)) // This line is problematic
		// if err == nil {
		// 	 defer term.Restore(int(s.Pty().Fd), originalState)
		// 	 log.Printf("Node %d: Raw mode enabled.", nodeID)
		// } else {
		// 	 log.Printf("Node %d: Failed to enable raw mode: %v. Proceeding without raw mode.", nodeID, err)
		// 	 // Continue without raw mode? Some BBS functions might rely on it.
		// }
		log.Printf("Node %d: Skipping raw mode attempt (known issue with gliderlabs/ssh on Windows).", nodeID)
	} else {
		log.Printf("Node %d: Skipping raw mode attempt as no PTY was requested.", nodeID)
	}

	// --- Authentication and Main Loop ---
	log.Printf("Node %d: Starting BBS logic...", nodeID)
	sessionStartTime := time.Now()
	currentMenuName := "LOGIN"               // Start with LOGIN
	var nextActionAfterLogin string          // << NEW: Variable to store the action after successful login
	autoRunLog := make(types.AutoRunTracker) // Initialize tracker for this session

	// Login Loop
	for authenticatedUser == nil {
		if currentMenuName == "" || currentMenuName == "LOGOFF" {
			log.Printf("Node %d: Login failed or aborted. Terminating session.", nodeID)
			fmt.Fprintln(terminalInstance, "\r\nLogin failed or aborted.")
			return
		}

		// Execute the current menu (e.g., LOGIN)
		// Run returns the next menu name, the authenticated user (if successful), or an error.
		// Pass nodeID directly as int, use sessionStartTime from context
		// Pass the session's autoRunLog
		// Pass "" for currentAreaName during login
		nextMenuName, authUser, execErr := menuExecutor.Run(s, terminalInstance, userMgr, nil, currentMenuName, int(nodeID), sessionStartTime, autoRunLog, effectiveMode, "")
		if execErr != nil {
			// Log the error and decide how to proceed
			log.Printf("Node %d: Error executing menu '%s': %v", nodeID, currentMenuName, execErr)
			// Optionally display an error message to the user
			fmt.Fprintf(terminalInstance, "\r\nSystem error during menu execution: %v\r\n", execErr)
			// Maybe force logoff or retry?
			currentMenuName = "LOGOFF" // Force logoff on error for now
			continue
		}

		// Check if authentication was successful during this menu execution
		if authUser != nil {
			authenticatedUser = authUser
			log.Printf("Node %d: User '%s' authenticated successfully.", nodeID, authenticatedUser.Handle)
			// Login successful! Record event, STORE the next action, and break.
			nextActionAfterLogin = nextMenuName

			// --- START MOVED Login Event Recording --- Removed call to AddLoginEvent
			/* // Removed LoginEvent tracking
			event := user.LoginEvent{
				Username:  authenticatedUser.Username,
				Handle:    authenticatedUser.Handle,
				Timestamp: time.Now(), // Record time NOW
			}
			userMgr.AddLoginEvent(event)
			*/
			// --- END MOVED Login Event Recording ---

			break // Force exit from the login loop
		} else {
			// Authentication did not occur, proceed to the next menu in the login sequence
			currentMenuName = nextMenuName
		}
	} // End Login Loop

	log.Printf("DEBUG: *** Login Loop Completed ***")

	// --- Post-Authentication Main Loop ---
	// Safety check still useful here in case break logic fails somehow
	if authenticatedUser == nil {
		log.Printf("ERROR: Node %d: Reached post-auth loop but authenticatedUser is nil. Logging off.", nodeID)
		return
	}
	log.Printf("Node %d: Entering main loop for authenticated user: %s", nodeID, authenticatedUser.Handle)

	// << NEW: Set the initial menu for the main loop based on the action returned from login
	parts := strings.SplitN(nextActionAfterLogin, ":", 2) // Split action like "GOTO:MENU"
	if len(parts) == 2 {
		currentMenuName = strings.ToUpper(parts[1]) // Use the part after the colon
	} else {
		log.Printf("WARN: Node %d: Could not parse next action '%s' after login. Defaulting to MAIN.", nodeID, nextActionAfterLogin)
		currentMenuName = "MAIN" // Default if format is unexpected
	}

	// currentMenuName should now hold the name of the first menu AFTER successful login (e.g., "FASTLOGN" or "MAIN")
	for {
		if currentMenuName == "" || currentMenuName == "LOGOFF" {
			log.Printf("Node %d: User %s selected Logoff or reached end state.", nodeID, authenticatedUser.Handle)
			fmt.Fprintln(terminalInstance, "\r\nLogging off...")
			// Add any cleanup tasks before closing the session
			break // Exit the loop
		}

		// *** ADD LOGGING HERE ***
		log.Printf("DEBUG: Node %d: Entering main loop iteration. CurrentMenu: %s, OutputMode: %d", nodeID, currentMenuName, effectiveMode)

		// Execute the current menu (e.g., MAIN, READ_MSG, etc.)
		// Pass nodeID directly as int, use sessionStartTime from context
		// Pass the session's autoRunLog
		// Pass "" for currentAreaName for now (TODO: Pass actual session area name)
		nextMenuName, _, execErr := menuExecutor.Run(s, terminalInstance, userMgr, authenticatedUser, currentMenuName, int(nodeID), sessionStartTime, autoRunLog, effectiveMode, "")
		if execErr != nil {
			log.Printf("Node %d: Error executing menu '%s': %v", nodeID, currentMenuName, execErr)
			fmt.Fprintf(terminalInstance, "\r\nSystem error during menu execution: %v\r\n", execErr)
			// Logoff on error?
			currentMenuName = "LOGOFF"
			continue
		}

		// Move to the next menu determined by the user's action in the previous menu
		currentMenuName = nextMenuName
	}

	log.Printf("Node %d: Session handler finished for %s.", nodeID, authenticatedUser.Handle)
}

// --- Test Functions REMOVED ---

// --- Main Function --- //
func main() {
	// Define and parse the --colortest flag REMOVED
	// flag.BoolVar(&colorTestMode, "colortest", false, "Run ANSI color test mode instead of BBS")
	// Define output mode flag
	flag.StringVar(&outputModeFlag, "output-mode", "auto", "Terminal output mode: auto (default), utf8, cp437")
	flag.Parse()

	// Validate output mode flag
	outputModeFlag = strings.ToLower(outputModeFlag)
	if outputModeFlag != "auto" && outputModeFlag != "utf8" && outputModeFlag != "cp437" {
		log.Fatalf("FATAL: Invalid --output-mode value '%s'. Must be 'auto', 'utf8', or 'cp437'.", outputModeFlag)
	}
	log.Printf("INFO: Output mode set to: %s", outputModeFlag)

	log.SetOutput(os.Stderr) // Ensure logs go to stderr
	log.Println("INFO: Starting ViSiON/3 BBS Server (using crypto/ssh)...")

	// REMOVED if colorTestMode block

	// --- Run Normal BBS Server --- //
	var err error
	fmt.Println("Starting ViSiON/3 BBS...") // Changed startup message

	// Determine base paths
	basePath, err := os.Getwd() // Or use a more robust method if needed
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}
	menuSetPath := filepath.Join(basePath, "menus", "v3") // Default menu set
	rootConfigPath := filepath.Join(basePath, "configs")
	rootAssetsPath := filepath.Join(basePath, "assets") // Keep this path definition for now
	dataPath := filepath.Join(basePath, "data")         // For user data, logs, etc.
	userDataPath := filepath.Join(dataPath, "users")
	logFilePath := filepath.Join(dataPath, "logs", "vision3.log") // Example log path

	// Ensure data directories exist (optional, depends on usage)
	// os.MkdirAll(userDataPath, 0755)
	os.MkdirAll(filepath.Dir(logFilePath), 0755) // Ensure the log directory exists

	// Setup Logging (example - adapt to your logging package if different)
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("WARN: Failed to open log file %s: %v. Logging to stderr.", logFilePath, err)
	} else {
		log.SetOutput(io.MultiWriter(os.Stderr, logFile)) // Log to both file and stderr
		log.Printf("INFO: Logging to file: %s", logFilePath)
		defer logFile.Close()
	}

	// Load global strings configuration from the new location
	loadedStrings, err = config.LoadStrings(rootConfigPath)
	if err != nil {
		log.Fatalf("Failed to load strings configuration: %v", err)
	}

	// Load theme configuration from the menu set path
	loadedTheme, err = config.LoadThemeConfig(menuSetPath)
	if err != nil {
		log.Printf("WARN: Proceeding with default theme due to error loading %s: %v", filepath.Join(menuSetPath, "theme.json"), err)
	}

	// Load door configurations from the new location
	loadedDoors, err := config.LoadDoors(filepath.Join(rootConfigPath, "doors.json")) // Expects full path
	if err != nil {
		log.Fatalf("Failed to load door configuration: %v", err)
	}

	// Load oneliners (Assuming they are still global for now, adjust if needed)
	// oneliners, err := config.LoadOneLiners(filepath.Join(dataPath, "oneliners.dat")) // Example path
	// if err != nil {
	// 	log.Printf("WARN: Failed to load oneliners: %v", err)
	// 	oneliners = []string{} // Use empty list if loading fails
	// }
	// Initialize oneliners as empty slice since loading is now handled by the runnable
	oneliners := []string{}

	// Initialize UserManager (using dataPath)
	userMgr, err = user.NewUserManager(userDataPath) // Pass the directory for users.json
	if err != nil {
		log.Fatalf("Failed to initialize user manager: %v", err)
	}

	// Initialize MessageManager (using dataPath)
	messageMgr, err = message.NewMessageManager(dataPath) // Pass the base data directory
	if err != nil {
		log.Fatalf("Failed to initialize message manager: %v", err)
	}

	// Initialize FileManager (using dataPath)
	fileMgr, err = file.NewFileManager(dataPath, rootConfigPath)
	if err != nil {
		log.Fatalf("Failed to initialize file manager: %v", err)
	}

	// Initialize MenuExecutor with new paths, loaded theme, and message manager
	menuExecutor = menu.NewExecutor(menuSetPath, rootConfigPath, rootAssetsPath, oneliners, loadedDoors, loadedStrings, loadedTheme, messageMgr, fileMgr)

	// Load Host Key
	hostKeyPath := filepath.Join(rootConfigPath, "ssh_host_rsa_key") // Example host key path
	hostKeySigner := loadHostKey(hostKeyPath)

	sshPort := 2222
	sshHost := "0.0.0.0"
	log.Printf("INFO: Configuring BBS SSH server on %s:%d...", sshHost, sshPort)

	passwordHandler := func(ctx ssh.Context, password string) bool {
		log.Printf("DEBUG: Password handler called for user '%s'. Allowing connection attempt, auth deferred to session handler.", ctx.User())
		return true
	}

	// Define cryptographic algorithm lists (defaults + legacy for compatibility)
	// Use only non-ETM MACs for potentially better compatibility with CBC modes
	/* // Commented out as unused in this test
	legacyCiphers := []string{
		"aes128-cbc", // Legacy added
		// "3des-cbc",   // Removed - Very weak
	}
	legacyMACs := []string{
		"hmac-sha1-96", // Legacy added
	}
	*/

	server := &ssh.Server{
		Addr:            fmt.Sprintf("%s:%d", sshHost, sshPort),
		Handler:         sessionHandler,
		PasswordHandler: passwordHandler,
		// Note: Crypto config is set via ServerConfigCallback below
	}

	// Set the custom crypto config callback
	server.ServerConfigCallback = func(ctx ssh.Context) *gossh.ServerConfig {
		// Create the ServerConfig with supported algorithms
		cfg := &gossh.ServerConfig{
			Config: gossh.Config{
				// Maximum compatibility - include legacy algorithms for old BBS clients like SyncTERM
				KeyExchanges: []string{
					// Enigma-BBS proven configuration for SyncTerm compatibility
					"curve25519-sha256",
					"curve25519-sha256@libssh.org",
					"ecdh-sha2-nistp256",
					"ecdh-sha2-nistp384",
					"ecdh-sha2-nistp521",
					"diffie-hellman-group14-sha1", // Essential for SyncTerm
					"diffie-hellman-group1-sha1",  // Ancient but needed
				},
				Ciphers: []string{
					// Enigma-BBS proven configuration for legacy BBS client compatibility
					"aes128-ctr",
					"aes192-ctr",
					"aes256-ctr",
					"aes128-gcm",
					"aes128-gcm@openssh.com",
					"aes256-gcm",
					"aes256-gcm@openssh.com",
					"aes256-cbc",
					"aes192-cbc",
					"aes128-cbc",
					"3des-cbc", // Essential for SyncTerm and legacy terminals
				},
				MACs: []string{
					// Enigma-BBS proven configuration for legacy BBS client compatibility
					"hmac-sha2-256",
					"hmac-sha2-512",
					"hmac-sha1",
					"hmac-md5",
					"hmac-sha2-256-96",
					"hmac-sha2-512-96",
					"hmac-ripemd160",
					"hmac-sha1-96",
					"hmac-md5-96",
				},
			},
		}

		log.Printf("DEBUG: ServerConfigCallback invoked, returning config with modern + legacy KEX algorithms")
		return cfg
	}

	server.AddHostKey(hostKeySigner)

	// Revert log message back to original
	log.Printf("INFO: Starting BBS server on %s:%d...", sshHost, sshPort)
	if err := server.ListenAndServe(); err != nil {
		if !errors.Is(err, ssh.ErrServerClosed) {
			log.Fatalf("FATAL: Failed to start BBS server: %v", err)
		} else {
			log.Println("INFO: BBS server closed gracefully.")
		}
	}
	log.Println("INFO: BBS Application shutting down.")

}

// --- Helper Functions (Existing loadHostKey, sendEnv) ---
func loadHostKey(path string) ssh.Signer {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("FATAL: Failed to read host key %s: %v", path, err)
	}
	signer, err := gossh.ParsePrivateKey(keyBytes)
	if err != nil {
		log.Fatalf("FATAL: Failed to parse host key %s: %v", path, err)
	}
	log.Printf("INFO: Host key loaded successfully from %s", path)
	return signer
}

func sendEnv(s ssh.Session, name, value string) {
	payload := &bytes.Buffer{}
	binary.Write(payload, binary.BigEndian, uint32(len(name)))
	payload.WriteString(name)
	binary.Write(payload, binary.BigEndian, uint32(len(value)))
	payload.WriteString(value)
	_, err := s.SendRequest("env", false, payload.Bytes())
	if err != nil {
		// Log quietly
	} else {
		// log.Printf("Node ?: Sent env request: %s=%s", name, value)
	}
}
