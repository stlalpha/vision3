package main

import (
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
	"github.com/stlalpha/vision3/internal/sshauth"
	"github.com/stlalpha/vision3/internal/user"
)

var (
	userMgr         *user.UserMgr
	messageMgr      *message.MessageManager
	fileMgr         *file.FileManager
	menuExecutor    *menu.MenuExecutor
	sshAuthenticator *sshauth.SSHAuthenticator
	globalConfig    config.GlobalConfig
	nodeCounter         int32
	activeSessions      = make(map[ssh.Session]int32)
	activeSessionsMutex sync.Mutex
	loadedStrings       config.StringsConfig
	loadedTheme         config.ThemeConfig
	// colorTestMode       bool   // Flag variable REMOVED
	outputModeFlag string // Output mode flag (auto, utf8, cp437)
)

// --- ANSI Test Server Code REMOVED ---

// --- BBS sessionHandler (Updated logic) ---
func sessionHandler(s ssh.Session) {
	nodeID := atomic.AddInt32(&nodeCounter, 1)
	remoteAddr := s.RemoteAddr().String()
	username := s.User()

	// Add session to active sessions map
	activeSessionsMutex.Lock()
	activeSessions[s] = nodeID // Use session as key
	activeSessionsMutex.Unlock()

	// Capture start time for call history
	capturedStartTime := time.Now()
	var authenticatedUser *user.User = nil

	// Defer cleanup - session removal and call history recording
	defer func(startTime time.Time) {
		log.Printf("Node %d: Disconnected %s (User: %s)", nodeID, remoteAddr, username)
		activeSessionsMutex.Lock()
		delete(activeSessions, s) // Remove using session as key
		activeSessionsMutex.Unlock()

		// Record call history if user was authenticated
		if authenticatedUser != nil {
			log.Printf("DEBUG: Node %d: Adding call record for user %s (ID: %d)", nodeID, authenticatedUser.Handle, authenticatedUser.ID)
			disconnectTime := time.Now()
			duration := disconnectTime.Sub(startTime)
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
		s.Close() // Ensure the session is closed
	}(capturedStartTime)

	// Create and run the session handler
	handler := session.NewSessionHandler(s, nodeID, sshAuthenticator, userMgr, menuExecutor, &globalConfig, loadedStrings)
	
	err := handler.HandleConnection()
	if err != nil {
		log.Printf("Node %d: Session handler error: %v", nodeID, err)
		return
	}

	// Get the authenticated user for call history recording
	authenticatedUser = handler.GetAuthenticatedUser()
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

	// Load global configuration
	globalConfig, err = config.LoadGlobalConfig(rootConfigPath)
	if err != nil {
		log.Fatalf("Failed to load global configuration: %v", err)
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
	
	// Initialize SSH Authenticator
	sshAuthenticator = sshauth.NewSSHAuthenticator(userMgr, globalConfig.SSHAuth)
	log.Printf("INFO: SSH authenticator initialized (New users: %v, Min password: %d)", 
		globalConfig.SSHAuth.AllowNewUsers, globalConfig.SSHAuth.MinPasswordLength)

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

	// Initialize MenuExecutor with new paths, loaded theme, message manager, and menu config
	menuExecutor = menu.NewExecutor(menuSetPath, rootConfigPath, rootAssetsPath, oneliners, loadedDoors, loadedStrings, loadedTheme, messageMgr, fileMgr, globalConfig.Menus)

	// Load Host Key
	hostKeyPath := filepath.Join(rootConfigPath, "ssh_host_rsa_key") // Example host key path
	hostKeySigner := loadHostKey(hostKeyPath)

	sshPort := 2222
	sshHost := "0.0.0.0"
	log.Printf("INFO: Configuring BBS SSH server on %s:%d...", sshHost, sshPort)
	log.Printf("INFO: SSH Authentication - New users: %v, Rate limit: %d attempts/%d seconds", 
		globalConfig.SSHAuth.AllowNewUsers, globalConfig.SSHAuth.MaxFailedAttempts, globalConfig.SSHAuth.RateLimitDuration)

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
	// Legacy algorithms for older BBS clients
	legacyKexAlgos := []string{
		"diffie-hellman-group14-sha1", // Legacy for older SSH clients
		"diffie-hellman-group1-sha1",  // Very old clients
	}

	server := &ssh.Server{
		Addr:            fmt.Sprintf("%s:%d", sshHost, sshPort),
		Handler:         sshAuthenticator.ConnectionHandler(sessionHandler), // Wrap with connection tracking
		PasswordHandler: sshAuthenticator.PasswordHandler(),                 // Use new authenticator
		// Note: Crypto config is set via ServerConfigCallback below
	}

	// Set the custom crypto config callback
	server.ServerConfigCallback = func(ctx ssh.Context) *gossh.ServerConfig {
		// Create the ServerConfig with modern algorithms
		cfg := &gossh.ServerConfig{
			Config: gossh.Config{
				// Modern + legacy key exchange algorithms for broad compatibility
				KeyExchanges: append([]string{
					"ecdh-sha2-nistp256",
					"ecdh-sha2-nistp384", 
					"ecdh-sha2-nistp521",
					"diffie-hellman-group16-sha512",
					"diffie-hellman-group14-sha256",
				}, legacyKexAlgos...),
				// Modern ciphers
				Ciphers: []string{
					"chacha20-poly1305@openssh.com",
					"aes256-gcm@openssh.com",
					"aes128-gcm@openssh.com",
					"aes256-ctr",
					"aes192-ctr", 
					"aes128-ctr",
				},
				// Modern MACs
				MACs: []string{
					"hmac-sha2-256-etm@openssh.com",
					"hmac-sha2-512-etm@openssh.com",
					"hmac-sha2-256",
					"hmac-sha2-512",
					"hmac-sha1",
				},
			},
		}

		log.Printf("DEBUG: ServerConfigCallback invoked with modern + legacy SSH algorithms")
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

