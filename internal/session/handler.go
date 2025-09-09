package session

import (
	"fmt"
	"log"
	"time"

	"github.com/gliderlabs/ssh"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/menu"
	"github.com/stlalpha/vision3/internal/sshauth"
	"github.com/stlalpha/vision3/internal/terminal"
	"github.com/stlalpha/vision3/internal/types"
	"github.com/stlalpha/vision3/internal/user"
)

// SessionHandler manages individual BBS sessions from connection to disconnection
type SessionHandler struct {
	// Core components
	session       ssh.Session
	terminal      *terminal.Terminal
	sshAuth       *sshauth.SSHAuthenticator
	userMgr       *user.UserMgr
	menuExecutor  *menu.MenuExecutor

	// Configuration
	globalConfig  *config.GlobalConfig
	strings       config.StringsConfig

	// Session state
	nodeID        int32
	username      string  
	remoteAddr    string
	authenticatedUser *user.User
	sessionStartTime  time.Time
	autoRunLog        types.AutoRunTracker
	outputMode        ansi.OutputMode
}

// NewSessionHandler creates a new session handler for a BBS connection
func NewSessionHandler(s ssh.Session, nodeID int32, sshAuth *sshauth.SSHAuthenticator, 
	userMgr *user.UserMgr, menuExecutor *menu.MenuExecutor, 
	globalConfig *config.GlobalConfig, strings config.StringsConfig) *SessionHandler {

	return &SessionHandler{
		session:       s,
		sshAuth:       sshAuth,
		userMgr:       userMgr,
		menuExecutor:  menuExecutor,
		globalConfig:  globalConfig,
		strings:       strings,
		nodeID:        nodeID,
		username:      s.User(),
		remoteAddr:    s.RemoteAddr().String(),
		sessionStartTime: time.Now(),
		autoRunLog:    make(types.AutoRunTracker),
	}
}

// HandleConnection manages the complete session lifecycle
func (h *SessionHandler) HandleConnection() error {
	log.Printf("Node %d: Connection from %s (User: %s, Session ID: %s)",
		h.nodeID, h.remoteAddr, h.username, h.session.Context().Value(ssh.ContextKeySessionID))

	// Phase 1: Initialize terminal
	if err := h.initializeTerminal(); err != nil {
		return fmt.Errorf("terminal initialization failed: %w", err)
	}

	// Phase 2: Configure environment  
	h.configureEnvironment()

	// Phase 3: Handle authentication and user setup
	if err := h.handleAuthentication(); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Phase 4: Run main BBS session
	h.runMainSession()

	log.Printf("Node %d: Session ended for user %s", h.nodeID, h.authenticatedUser.Handle)
	return nil
}

// initializeTerminal sets up the unified terminal and determines capabilities
func (h *SessionHandler) initializeTerminal() error {
	log.Printf("Node %d: Creating unified terminal for session", h.nodeID)

	h.terminal = terminal.NewTerminal(h.session)

	// Set output mode based on terminal capabilities
	h.outputMode = h.terminal.GetOutputMode()
	log.Printf("Node %d: Using auto-detected output mode: %d", h.nodeID, h.outputMode)

	// Start window resize handling
	h.terminal.StartWindowResizeHandler()

	// Simple connectivity test
	testMsg := "\r\n\x1b[31mSimple Test: RED\x1b[0m | \x1b[32mGREEN\x1b[0m | ASCII: Hello! 123?.,;\r\n"
	log.Printf("Node %d: Writing simple test message...", h.nodeID)
	testErr := h.terminal.WriteString(testMsg)
	if testErr != nil {
		log.Printf("Node %d: Error writing test message: %v", h.nodeID, testErr)
	}
	log.Printf("Node %d: Finished writing simple test message.", h.nodeID)

	return nil
}

// configureEnvironment sets up the terminal environment variables
func (h *SessionHandler) configureEnvironment() {
	if !h.terminal.HasPTY() {
		log.Printf("Node %d: No PTY available, skipping environment configuration.", h.nodeID)
		return
	}

	log.Printf("Node %d: PTY acquired. Attempting to configure environment.", h.nodeID)

	// Helper to send environment variables (RFC 4254 Section 6.4)
	h.sendEnv("TERM", "xterm-256color")
	// Attempt to force UTF-8 locale (common variables, might not be respected by Windows SSHd)
	h.sendEnv("LANG", "en_US.UTF-8")
	h.sendEnv("LC_ALL", "en_US.UTF-8")
	h.sendEnv("LC_CTYPE", "UTF-8")

	// Short delay to allow server to potentially process requests
	log.Printf("Node %d: Waiting briefly after sending env requests...", h.nodeID)
	time.Sleep(150 * time.Millisecond)
	log.Printf("Node %d: Proceeding after environment configuration attempt.", h.nodeID)
}

// handleAuthentication processes user authentication (existing users or new registration)
func (h *SessionHandler) handleAuthentication() error {
	log.Printf("Node %d: Processing pre-authenticated user: %s", h.nodeID, h.username)

	if h.sshAuth.IsNewUser(h.username) {
		return h.handleNewUserRegistration()
	} else {
		return h.handleExistingUser()
	}
}

// handleNewUserRegistration manages the new user registration flow
func (h *SessionHandler) handleNewUserRegistration() error {
	log.Printf("Node %d: Starting new user registration", h.nodeID)

	newUserReg := sshauth.NewNewUserRegistration(h.userMgr, h.globalConfig.SSHAuth, h.strings)
	newUserReg.SetOutputMode(h.outputMode)

	var err error
	h.authenticatedUser, err = newUserReg.RunRegistration(h.terminal, h.remoteAddr)
	if err != nil {
		log.Printf("Node %d: New user registration failed: %v", h.nodeID, err)
		h.terminal.WriteString(fmt.Sprintf("\r\nRegistration failed: %v\r\nPress any key to disconnect...\r\n", err))
		h.terminal.ReadLine() // Wait for keypress
		return fmt.Errorf("new user registration failed: %w", err)
	}

	log.Printf("Node %d: New user registration completed for '%s'", h.nodeID, h.authenticatedUser.Handle)

	// Check if user requires validation
	if h.globalConfig.SSHAuth.RequireValidation && !h.authenticatedUser.Validated {
		h.terminal.WriteString("\r\nYour account is pending validation. Please reconnect after approval.\r\n")
		return fmt.Errorf("user account requires validation")
	}

	return nil
}

// handleExistingUser validates and loads an existing user
func (h *SessionHandler) handleExistingUser() error {
	var exists bool
	h.authenticatedUser, exists = h.userMgr.GetUser(h.username)
	if !exists {
		log.Printf("ERROR: Node %d: SSH authenticated user '%s' not found in database", h.nodeID, h.username)
		h.terminal.WriteString("\r\nUser account not found. Please contact the SysOp.\r\n")
		return fmt.Errorf("user '%s' not found in database", h.username)
	}

	// Check if user is validated
	if !h.authenticatedUser.Validated {
		log.Printf("Node %d: User '%s' not validated", h.nodeID, h.username)
		h.terminal.WriteString(h.strings.NotValidated + "\r\n")
		return fmt.Errorf("user '%s' not validated", h.username)
	}

	log.Printf("Node %d: User '%s' loaded from database", h.nodeID, h.authenticatedUser.Handle)
	return nil
}

// runMainSession executes the main BBS menu loop
func (h *SessionHandler) runMainSession() {
	log.Printf("DEBUG: Node %d: User authentication completed for %s", h.nodeID, h.authenticatedUser.Handle)
	log.Printf("Node %d: Entering main BBS for authenticated user: %s", h.nodeID, h.authenticatedUser.Handle)

	// Set up default areas for the authenticated user
	h.setupDefaultAreas()

	// Start with MAIN menu for authenticated users (skip LOGIN entirely)
	currentMenuName := "MAIN"

	// Check for new user first-time flow
	if h.authenticatedUser.TimesCalled == 0 {
		// First time login - could start with NEWUSER or FASTLOGN menu if it exists
		currentMenuName = "FASTLOGN" // Will fall back to MAIN if menu doesn't exist
		log.Printf("Node %d: First-time user, starting with %s menu", h.nodeID, currentMenuName)
	}

	// Main BBS Loop
	for {
		if currentMenuName == "" || currentMenuName == "LOGOFF" {
			log.Printf("Node %d: User %s selected Logoff or reached end state.", h.nodeID, h.authenticatedUser.Handle)
			h.terminal.WriteString("\r\nLogging off...")
			break // Exit the loop
		}

		log.Printf("DEBUG: Node %d: Entering main loop iteration. CurrentMenu: %s, OutputMode: %d", 
			h.nodeID, currentMenuName, h.outputMode)

		// Execute the current menu (e.g., MAIN, READ_MSG, etc.)
		// Pass the authenticated user directly since SSH-level auth is complete
		nextMenuName, _, execErr := h.menuExecutor.Run(h.session, h.terminal, 
			h.userMgr, h.authenticatedUser, currentMenuName, int(h.nodeID), 
			h.sessionStartTime, h.autoRunLog, h.outputMode, "")
		
		if execErr != nil {
			log.Printf("Node %d: Error executing menu '%s': %v", h.nodeID, currentMenuName, execErr)
			h.terminal.WriteString(fmt.Sprintf("\r\nSystem error during menu execution: %v\r\n", execErr))
			// Logoff on error
			currentMenuName = "LOGOFF"
			continue
		}

		// Move to the next menu determined by the user's action in the previous menu
		currentMenuName = nextMenuName
	}
}

// sendEnv sends an environment variable to the SSH session
func (h *SessionHandler) sendEnv(key, value string) {
	// Create payload manually instead of using ssh.Marshal
	payload := []byte{}
	keyBytes := []byte(key)
	valueBytes := []byte(value)
	
	// SSH protocol format: 4-byte length + string data
	keyLen := len(keyBytes)
	valueLen := len(valueBytes)
	
	// Append key length and key
	payload = append(payload, byte(keyLen>>24), byte(keyLen>>16), byte(keyLen>>8), byte(keyLen))
	payload = append(payload, keyBytes...)
	
	// Append value length and value
	payload = append(payload, byte(valueLen>>24), byte(valueLen>>16), byte(valueLen>>8), byte(valueLen))
	payload = append(payload, valueBytes...)

	_, err := h.session.SendRequest("env", false, payload)
	if err != nil {
		log.Printf("Node %d: Failed to send %s=%s: %v", h.nodeID, key, value, err)
	} else {
		log.Printf("Node %d: Sent %s=%s", h.nodeID, key, value)
	}
}

// setupDefaultAreas sets up default message and file areas for the authenticated user
func (h *SessionHandler) setupDefaultAreas() {
	// Set Default Message Area
	if h.authenticatedUser != nil && h.menuExecutor != nil && h.menuExecutor.MessageMgr != nil {
		allAreas := h.menuExecutor.MessageMgr.ListAreas() // Already sorted by ID
		log.Printf("DEBUG: Found %d message areas for user %s.", len(allAreas), h.authenticatedUser.Handle)
		foundDefaultArea := false
		for _, area := range allAreas {
			// Check if user has read access to this area (simplified ACS check)
			// For now, assume all areas are accessible - proper ACS checking would require session context
			log.Printf("INFO: Setting default message area for user %s to Area ID %d (%s)", h.authenticatedUser.Handle, area.ID, area.Tag)
			h.authenticatedUser.CurrentMessageAreaID = area.ID
			h.authenticatedUser.CurrentMessageAreaTag = area.Tag // Store tag too
			foundDefaultArea = true
			break // Found the first area
		}
		if !foundDefaultArea {
			log.Printf("WARN: User %s has no accessible message areas.", h.authenticatedUser.Handle)
			h.authenticatedUser.CurrentMessageAreaID = 0 // Set to 0 if no areas found
			h.authenticatedUser.CurrentMessageAreaTag = ""
		}
	} else {
		log.Printf("WARN: Cannot set default message area: missing components.")
	}

	// Set Default File Area
	if h.authenticatedUser != nil && h.menuExecutor != nil && h.menuExecutor.FileMgr != nil {
		allFileAreas := h.menuExecutor.FileMgr.ListAreas() // Assumes ListAreas is sorted by ID
		log.Printf("DEBUG: Found %d file areas for user %s.", len(allFileAreas), h.authenticatedUser.Handle)
		foundDefaultFileArea := false
		for _, area := range allFileAreas {
			// Check if user has list access to this area (simplified ACS check)
			// For now, assume all areas are accessible
			log.Printf("INFO: Setting default file area for user %s to Area ID %d (%s)", h.authenticatedUser.Handle, area.ID, area.Tag)
			h.authenticatedUser.CurrentFileAreaID = area.ID
			h.authenticatedUser.CurrentFileAreaTag = area.Tag // Store tag too
			foundDefaultFileArea = true
			break // Found the first area
		}
		if !foundDefaultFileArea {
			log.Printf("WARN: User %s has no accessible file areas.", h.authenticatedUser.Handle)
			h.authenticatedUser.CurrentFileAreaID = 0 // Set to 0 if no areas found
			h.authenticatedUser.CurrentFileAreaTag = ""
		}
	} else {
		log.Printf("WARN: Cannot set default file area: missing components.")
	}
}

// GetAuthenticatedUser returns the authenticated user for call history recording
func (h *SessionHandler) GetAuthenticatedUser() *user.User {
	return h.authenticatedUser
}