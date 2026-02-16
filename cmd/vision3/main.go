package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gliderlabs/ssh" // Keep for type compatibility
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"

	// Local packages (Update paths)
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/chat"
	"github.com/stlalpha/vision3/internal/conference"
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/file"
	"github.com/stlalpha/vision3/internal/menu"
	"github.com/stlalpha/vision3/internal/message"
	"github.com/stlalpha/vision3/internal/scheduler"
	"github.com/stlalpha/vision3/internal/session"
	"github.com/stlalpha/vision3/internal/sshserver"
	"github.com/stlalpha/vision3/internal/telnetserver"
	"github.com/stlalpha/vision3/internal/types"
	"github.com/stlalpha/vision3/internal/user"
)

var (
	userMgr      *user.UserMgr
	messageMgr   *message.MessageManager
	fileMgr      *file.FileManager
	confMgr      *conference.ConferenceManager
	menuExecutor    *menu.MenuExecutor
	sessionRegistry *session.SessionRegistry
	// globalConfig *config.GlobalConfig // Still commented out
	nodeCounter         int32
	activeSessions      = make(map[ssh.Session]int32)
	activeSessionsMutex sync.Mutex
	loadedStrings       config.StringsConfig
	loadedTheme         config.ThemeConfig
	// colorTestMode       bool   // Flag variable REMOVED
	outputModeFlag    string             // Output mode flag (auto, utf8, cp437)
	connectionTracker *ConnectionTracker // Global connection tracker
)

// allocateNodeIDForSession assigns the lowest available node slot (1..maxNodes)
// and records it in activeSessions. Falls back to a monotonic counter if maxNodes
// is unavailable or all slots appear occupied.
func allocateNodeIDForSession(s ssh.Session) int32 {
	activeSessionsMutex.Lock()
	defer activeSessionsMutex.Unlock()

	maxNodes := 0
	if connectionTracker != nil {
		maxNodes = connectionTracker.maxNodes
	}

	if maxNodes > 0 {
		used := make(map[int32]bool, len(activeSessions))
		for _, id := range activeSessions {
			if id > 0 && int(id) <= maxNodes {
				used[id] = true
			}
		}

		for slot := int32(1); slot <= int32(maxNodes); slot++ {
			if !used[slot] {
				activeSessions[s] = slot
				return slot
			}
		}
	}

	fallback := atomic.AddInt32(&nodeCounter, 1)
	activeSessions[s] = fallback
	return fallback
}

// IPList holds a list of IP addresses and CIDR ranges
type IPList struct {
	ips      map[string]bool // Individual IP addresses
	networks []*net.IPNet    // CIDR ranges
}

// IPLockoutTracker tracks failed login attempts per IP
type IPLockoutTracker struct {
	Attempts    int
	LastAttempt time.Time
	LockedUntil time.Time
}

// ConnectionTracker manages active connections and enforces limits
type ConnectionTracker struct {
	mu                  sync.Mutex
	activeConnections   map[string]int               // IP address -> connection count
	failedLogins        map[string]*IPLockoutTracker // IP address -> lockout tracker
	maxNodes            int
	maxConnectionsPerIP int
	totalConnections    int
	blocklist           *IPList
	allowlist           *IPList
	blocklistPath       string // Path to blocklist file for watching
	allowlistPath       string // Path to allowlist file for watching
	maxFailedLogins     int
	lockoutMinutes      int
	watcher             *fsnotify.Watcher // File system watcher for auto-reload
	watcherDone         chan bool         // Signal to stop watcher
}

// NewConnectionTracker creates a new connection tracker
func NewConnectionTracker(maxNodes, maxConnectionsPerIP, maxFailedLogins, lockoutMinutes int, blocklistPath, allowlistPath string) *ConnectionTracker {
	ct := &ConnectionTracker{
		activeConnections:   make(map[string]int),
		failedLogins:        make(map[string]*IPLockoutTracker),
		maxNodes:            maxNodes,
		maxConnectionsPerIP: maxConnectionsPerIP,
		blocklist:           nil,
		allowlist:           nil,
		blocklistPath:       blocklistPath,
		allowlistPath:       allowlistPath,
		maxFailedLogins:     maxFailedLogins,
		lockoutMinutes:      lockoutMinutes,
	}

	// Load initial IP lists
	if blocklistPath != "" {
		blocklist, err := LoadIPList(blocklistPath)
		if err != nil {
			log.Printf("ERROR: Failed to load initial blocklist: %v", err)
		} else {
			ct.blocklist = blocklist
		}
	}

	if allowlistPath != "" {
		allowlist, err := LoadIPList(allowlistPath)
		if err != nil {
			log.Printf("ERROR: Failed to load initial allowlist: %v", err)
		} else {
			ct.allowlist = allowlist
		}
	}

	// Start watching files for changes
	if err := ct.startWatching(); err != nil {
		log.Printf("ERROR: Failed to start file watcher: %v", err)
	}

	return ct
}

// LoadIPList loads an IP list from a file
// File format: one IP or CIDR range per line, # for comments
func LoadIPList(filePath string) (*IPList, error) {
	if filePath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // File doesn't exist, not an error
		}
		return nil, fmt.Errorf("failed to read IP list %s: %w", filePath, err)
	}

	list := &IPList{
		ips:      make(map[string]bool),
		networks: make([]*net.IPNet, 0),
	}

	lines := strings.Split(string(data), "\n")
	for lineNum, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check if it's a CIDR range
		if strings.Contains(line, "/") {
			_, network, err := net.ParseCIDR(line)
			if err != nil {
				log.Printf("WARN: Invalid CIDR in %s line %d: %s", filePath, lineNum+1, line)
				continue
			}
			list.networks = append(list.networks, network)
		} else {
			// Individual IP address
			ip := net.ParseIP(line)
			if ip == nil {
				log.Printf("WARN: Invalid IP in %s line %d: %s", filePath, lineNum+1, line)
				continue
			}
			list.ips[ip.String()] = true
		}
	}

	log.Printf("INFO: Loaded IP list from %s: %d IPs, %d CIDR ranges",
		filePath, len(list.ips), len(list.networks))
	return list, nil
}

// Contains checks if an IP address is in the list
func (list *IPList) Contains(ipStr string) bool {
	if list == nil {
		return false
	}

	// Check individual IPs
	if list.ips[ipStr] {
		return true
	}

	// Check CIDR ranges
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, network := range list.networks {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// CanAccept checks if a new connection from the given IP can be accepted
func (ct *ConnectionTracker) CanAccept(remoteAddr net.Addr) (bool, string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	return ct.canAcceptLocked(remoteAddr)
}

// canAcceptLocked performs the accept check without acquiring the lock.
func (ct *ConnectionTracker) canAcceptLocked(remoteAddr net.Addr) (bool, string) {
	// Extract IP from address (strip port)
	ip := extractIP(remoteAddr)

	// Check allowlist first - if IP is on allowlist, skip all other checks
	if ct.allowlist != nil && ct.allowlist.Contains(ip) {
		log.Printf("DEBUG: IP %s is on allowlist, bypassing all checks", ip)
		return true, ""
	}

	// Check blocklist
	if ct.blocklist != nil && ct.blocklist.Contains(ip) {
		return false, "IP address is blocked"
	}

	// Check max nodes limit
	if ct.maxNodes > 0 && ct.totalConnections >= ct.maxNodes {
		return false, "maximum nodes reached"
	}

	// Check per-IP limit
	if ct.maxConnectionsPerIP > 0 {
		if count, exists := ct.activeConnections[ip]; exists && count >= ct.maxConnectionsPerIP {
			return false, "maximum connections per IP reached"
		}
	}

	return true, ""
}

// TryAccept atomically checks limits and registers the connection.
// Returns (true, "") on success, (false, reason) on rejection.
func (ct *ConnectionTracker) TryAccept(remoteAddr net.Addr) (bool, string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ok, reason := ct.canAcceptLocked(remoteAddr)
	if !ok {
		return false, reason
	}

	ip := extractIP(remoteAddr)
	ct.activeConnections[ip]++
	ct.totalConnections++

	log.Printf("INFO: Connection added from %s. IP count: %d, Total: %d/%d",
		ip, ct.activeConnections[ip], ct.totalConnections, ct.maxNodes)
	return true, ""
}

// AddConnection registers a new connection from the given IP
func (ct *ConnectionTracker) AddConnection(remoteAddr net.Addr) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ip := extractIP(remoteAddr)
	ct.activeConnections[ip]++
	ct.totalConnections++

	log.Printf("INFO: Connection added from %s. IP count: %d, Total: %d/%d",
		ip, ct.activeConnections[ip], ct.totalConnections, ct.maxNodes)
}

// RemoveConnection unregisters a connection from the given IP
func (ct *ConnectionTracker) RemoveConnection(remoteAddr net.Addr) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ip := extractIP(remoteAddr)
	if count, exists := ct.activeConnections[ip]; exists {
		if count <= 1 {
			delete(ct.activeConnections, ip)
		} else {
			ct.activeConnections[ip]--
		}
	}
	if ct.totalConnections > 0 {
		ct.totalConnections--
	}

	log.Printf("INFO: Connection removed from %s. IP count: %d, Total: %d/%d",
		ip, ct.activeConnections[ip], ct.totalConnections, ct.maxNodes)
}

// GetStats returns current connection statistics
func (ct *ConnectionTracker) GetStats() (totalConns, uniqueIPs int) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.totalConnections, len(ct.activeConnections)
}

// extractIP extracts the IP address from a net.Addr, stripping the port
func extractIP(addr net.Addr) string {
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return tcpAddr.IP.String()
	}
	// Fallback: parse string representation
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String() // Return as-is if parsing fails
	}
	return host
}

// IsIPLockedOut checks if an IP address is currently locked out due to failed login attempts.
// Returns (isLocked, lockedUntil, remainingAttempts)
func (ct *ConnectionTracker) IsIPLockedOut(ip string) (bool, time.Time, int) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	tracker, exists := ct.failedLogins[ip]
	if !exists {
		return false, time.Time{}, ct.maxFailedLogins
	}

	// Check if lockout has expired
	if !tracker.LockedUntil.IsZero() && time.Now().Before(tracker.LockedUntil) {
		return true, tracker.LockedUntil, 0
	}

	// Lockout expired, clear it
	if !tracker.LockedUntil.IsZero() && time.Now().After(tracker.LockedUntil) {
		delete(ct.failedLogins, ip)
		return false, time.Time{}, ct.maxFailedLogins
	}

	remainingAttempts := ct.maxFailedLogins - tracker.Attempts
	if remainingAttempts < 0 {
		remainingAttempts = 0
	}
	return false, time.Time{}, remainingAttempts
}

// RecordFailedLoginAttempt records a failed login attempt from an IP address.
// Returns true if the IP was just locked out.
func (ct *ConnectionTracker) RecordFailedLoginAttempt(ip string) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	// Don't track if feature is disabled
	if ct.maxFailedLogins == 0 {
		return false
	}

	tracker, exists := ct.failedLogins[ip]
	if !exists {
		tracker = &IPLockoutTracker{}
		ct.failedLogins[ip] = tracker
	}

	tracker.Attempts++
	tracker.LastAttempt = time.Now()

	// Check if lockout threshold reached
	if tracker.Attempts >= ct.maxFailedLogins {
		tracker.LockedUntil = time.Now().Add(time.Duration(ct.lockoutMinutes) * time.Minute)
		log.Printf("SECURITY: IP %s locked out after %d failed login attempts. Locked until %s",
			ip, tracker.Attempts, tracker.LockedUntil.Format(time.RFC3339))
		return true
	}

	log.Printf("SECURITY: Failed login attempt from IP %s (%d/%d)",
		ip, tracker.Attempts, ct.maxFailedLogins)
	return false
}

// ClearFailedLoginAttempts clears the failed login counter for an IP on successful authentication.
func (ct *ConnectionTracker) ClearFailedLoginAttempts(ip string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	if tracker, exists := ct.failedLogins[ip]; exists {
		if tracker.Attempts > 0 {
			log.Printf("INFO: Cleared failed login attempts for IP %s (%d attempts)",
				ip, tracker.Attempts)
		}
		delete(ct.failedLogins, ip)
	}
}

// reloadIPLists reloads the blocklist and allowlist from disk.
// Both lists are loaded outside the lock and swapped atomically under a single lock.
func (ct *ConnectionTracker) reloadIPLists() {
	log.Printf("INFO: Reloading IP filter lists...")

	// Load both lists outside the lock (I/O can be slow)
	var newBlocklist, newAllowlist *IPList

	if ct.blocklistPath != "" {
		bl, err := LoadIPList(ct.blocklistPath)
		if err != nil {
			log.Printf("ERROR: Failed to reload blocklist from %s: %v", ct.blocklistPath, err)
		} else {
			newBlocklist = bl
		}
	}

	if ct.allowlistPath != "" {
		al, err := LoadIPList(ct.allowlistPath)
		if err != nil {
			log.Printf("ERROR: Failed to reload allowlist from %s: %v", ct.allowlistPath, err)
		} else {
			newAllowlist = al
		}
	}

	// Swap both lists atomically under a single lock
	ct.mu.Lock()
	if ct.blocklistPath != "" {
		ct.blocklist = newBlocklist
	}
	if ct.allowlistPath != "" {
		ct.allowlist = newAllowlist
	}
	ct.mu.Unlock()

	log.Printf("INFO: IP filter lists reloaded")
}

// startWatching starts watching the IP list files for changes
func (ct *ConnectionTracker) startWatching() error {
	// Don't start watcher if no files to watch
	if ct.blocklistPath == "" && ct.allowlistPath == "" {
		log.Printf("DEBUG: No IP list files to watch, file watching disabled")
		return nil
	}

	var err error
	ct.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	ct.watcherDone = make(chan bool)

	// Add files to watch
	filesToWatch := []string{}
	if ct.blocklistPath != "" {
		if _, err := os.Stat(ct.blocklistPath); err == nil {
			filesToWatch = append(filesToWatch, ct.blocklistPath)
		} else {
			log.Printf("WARN: Blocklist file %s does not exist, will not watch for changes", ct.blocklistPath)
		}
	}
	if ct.allowlistPath != "" {
		if _, err := os.Stat(ct.allowlistPath); err == nil {
			filesToWatch = append(filesToWatch, ct.allowlistPath)
		} else {
			log.Printf("WARN: Allowlist file %s does not exist, will not watch for changes", ct.allowlistPath)
		}
	}

	for _, file := range filesToWatch {
		if err := ct.watcher.Add(file); err != nil {
			log.Printf("ERROR: Failed to watch file %s: %v", file, err)
		} else {
			log.Printf("INFO: Watching %s for changes (auto-reload enabled)", file)
		}
	}

	// Start watching in a goroutine
	go ct.watchLoop()

	return nil
}

// watchLoop handles file system events
func (ct *ConnectionTracker) watchLoop() {
	// Debounce timer to avoid reloading on rapid successive writes
	var debounceTimer *time.Timer
	debounceDuration := 500 * time.Millisecond

	for {
		select {
		case event, ok := <-ct.watcher.Events:
			if !ok {
				return
			}

			// Only care about Write and Create events
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("DEBUG: File change detected: %s (%s)", event.Name, event.Op)

				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(debounceDuration, func() {
					ct.reloadIPLists()
				})
			}

		case err, ok := <-ct.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("ERROR: File watcher error: %v", err)

		case <-ct.watcherDone:
			log.Printf("INFO: Stopping IP list file watcher")
			return
		}
	}
}

// StopWatching stops the file watcher
func (ct *ConnectionTracker) StopWatching() {
	if ct.watcher != nil {
		close(ct.watcherDone)
		ct.watcher.Close()
	}
}

// --- SSH Session Types (golang.org/x/crypto/ssh adapter) ---
// Use gliderlabs/ssh types for compatibility

// SessionContext provides session context information and implements gliderlabs/ssh.Context
type SessionContext struct {
	ctx         context.Context
	sessionID   string
	user        string
	remoteAddr  net.Addr
	localAddr   net.Addr
	clientVer   string
	serverVer   string
	permissions *ssh.Permissions
	mu          sync.Mutex
	values      map[interface{}]interface{}
}

// context.Context methods
func (sc *SessionContext) Value(key interface{}) interface{} {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if v, ok := sc.values[key]; ok {
		return v
	}
	return sc.ctx.Value(key)
}

func (sc *SessionContext) Deadline() (deadline time.Time, ok bool) {
	return sc.ctx.Deadline()
}

func (sc *SessionContext) Done() <-chan struct{} {
	return sc.ctx.Done()
}

func (sc *SessionContext) Err() error {
	return sc.ctx.Err()
}

// sync.Locker methods
func (sc *SessionContext) Lock() {
	sc.mu.Lock()
}

func (sc *SessionContext) Unlock() {
	sc.mu.Unlock()
}

// gliderlabs/ssh.Context methods
func (sc *SessionContext) User() string {
	return sc.user
}

func (sc *SessionContext) SessionID() string {
	return sc.sessionID
}

func (sc *SessionContext) ClientVersion() string {
	return sc.clientVer
}

func (sc *SessionContext) ServerVersion() string {
	return sc.serverVer
}

func (sc *SessionContext) RemoteAddr() net.Addr {
	return sc.remoteAddr
}

func (sc *SessionContext) LocalAddr() net.Addr {
	return sc.localAddr
}

func (sc *SessionContext) Permissions() *ssh.Permissions {
	return sc.permissions
}

func (sc *SessionContext) SetValue(key, value interface{}) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.values == nil {
		sc.values = make(map[interface{}]interface{})
	}
	sc.values[key] = value
}

// SessionAdapter adapts golang.org/x/crypto/ssh to gliderlabs/ssh Session interface
type SessionAdapter struct {
	conn        *gossh.ServerConn
	channel     gossh.Channel
	requests    <-chan *gossh.Request
	user        string
	remoteAddr  net.Addr
	localAddr   net.Addr
	environ     []string
	command     []string
	rawCommand  string
	subsystem   string
	ptyMutex    sync.Mutex
	pty         *ssh.Pty
	winch       chan ssh.Window
	hasPty      bool
	ctx         *SessionContext
	cancel      context.CancelFunc
	signalsChan chan<- ssh.Signal
	breakChan   chan<- bool
}

// Implement gossh.Channel methods
func (s *SessionAdapter) Read(p []byte) (n int, err error) {
	return s.channel.Read(p)
}

func (s *SessionAdapter) Write(p []byte) (n int, err error) {
	return s.channel.Write(p)
}

func (s *SessionAdapter) CloseWrite() error {
	return s.channel.CloseWrite()
}

func (s *SessionAdapter) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	return s.channel.SendRequest(name, wantReply, payload)
}

func (s *SessionAdapter) Stderr() io.ReadWriter {
	return s.channel.Stderr()
}

// Implement Session interface methods
func (s *SessionAdapter) User() string {
	return s.user
}

func (s *SessionAdapter) RemoteAddr() net.Addr {
	return s.remoteAddr
}

func (s *SessionAdapter) LocalAddr() net.Addr {
	return s.localAddr
}

func (s *SessionAdapter) Context() ssh.Context {
	return s.ctx
}

func (s *SessionAdapter) SessionID() string {
	return s.ctx.sessionID
}

func (s *SessionAdapter) Environ() []string {
	return s.environ
}

func (s *SessionAdapter) Command() []string {
	return s.command
}

func (s *SessionAdapter) RawCommand() string {
	return s.rawCommand
}

func (s *SessionAdapter) Subsystem() string {
	return s.subsystem
}

func (s *SessionAdapter) PublicKey() ssh.PublicKey {
	return nil // BBS uses password auth
}

func (s *SessionAdapter) Permissions() ssh.Permissions {
	if s.ctx != nil && s.ctx.permissions != nil {
		return *s.ctx.permissions
	}
	return ssh.Permissions{}
}

func (s *SessionAdapter) Pty() (ssh.Pty, <-chan ssh.Window, bool) {
	s.ptyMutex.Lock()
	defer s.ptyMutex.Unlock()
	if s.hasPty && s.pty != nil {
		return *s.pty, s.winch, true
	}
	return ssh.Pty{}, nil, false
}

func (s *SessionAdapter) Exit(code int) error {
	// Send exit status
	status := struct{ Status uint32 }{uint32(code)}
	payload := gossh.Marshal(status)
	s.SendRequest("exit-status", false, payload)
	return s.Close()
}

func (s *SessionAdapter) Signals(c chan<- ssh.Signal) {
	s.signalsChan = c
}

func (s *SessionAdapter) Break(c chan<- bool) {
	s.breakChan = c
}

func (s *SessionAdapter) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return s.channel.Close()
}

// --- ANSI Test Server Code REMOVED ---

// --- BBS sessionHandler (Original logic) ---
func sessionHandler(s ssh.Session) {
	nodeID := allocateNodeIDForSession(s)
	remoteAddr := s.RemoteAddr().String()

	// Extract session ID if available (type-specific)
	sessionID := fmt.Sprintf("node-%d", nodeID)
	if ctx, ok := s.Context().(interface{ SessionID() string }); ok {
		sessionID = ctx.SessionID()
	}

	log.Printf("Node %d: Connection from %s (User: %s, Session ID: %s)", nodeID, remoteAddr, s.User(), sessionID)
	log.Printf("Node %d: Environment: %v", nodeID, s.Environ())
	log.Printf("Node %d: Command: %v", nodeID, s.Command())

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
		if sessionRegistry != nil {
			sessionRegistry.Unregister(int(nodeID))
		}

		// --- Record Call History ---
		if authenticatedUser != nil {
			log.Printf("DEBUG: Node %d: Adding call record for user %s (ID: %d)", nodeID, authenticatedUser.Handle, authenticatedUser.ID)

			// Mark user as offline
			userMgr.MarkUserOffline(authenticatedUser.ID)

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

	// Create the session state object *early* - COMMENTED OUT (not used, type mismatch)
	// sessionState := &session.BbsSession{
	// 	// Conn:    s.Conn,     // Need the underlying gossh.Conn if possible, might need context
	// 	Channel:    nil,         // Channel might not be directly available here, depends on gliderlabs/ssh context
	// 	User:       nil,         // Set after authentication
	// 	ID:         int(nodeID), // Use correct field name 'ID'
	// 	StartTime:  time.Now(),  // Record session start time
	// 	Pty:        nil,         // Will be set if/when PTY is granted
	// 	AutoRunLog: make(types.AutoRunTracker),
	// }

	// --- PTY Request Handling ---
	ptyReq, winCh, isPty := s.Pty() // Get PTY info from SessionAdapter
	var termWidth, termHeight atomic.Int32
	termWidth.Store(80)  // Default terminal width
	termHeight.Store(25) // Default terminal height
	if isPty {
		log.Printf("Node %d: PTY Request Accepted: %s", nodeID, ptyReq.Term)
		if ptyReq.Window.Width > 0 {
			termWidth.Store(int32(ptyReq.Window.Width))
		}
		if ptyReq.Window.Height > 0 {
			termHeight.Store(int32(ptyReq.Window.Height))
		}
		log.Printf("Node %d: Terminal size from PTY: %dx%d", nodeID, termWidth.Load(), termHeight.Load())
	} else {
		log.Printf("Node %d: No PTY Request received. Proceeding without PTY.", nodeID)
	}

	// --- Determine Output Mode ---
	effectiveMode := ansi.OutputModeAuto // Start with Auto as the base
	switch outputModeFlag {              // Check the global flag first
	case "utf8":
		effectiveMode = ansi.OutputModeUTF8
		log.Printf("Node %d: Output mode forced to UTF-8 by flag.", nodeID)
	case "cp437":
		effectiveMode = ansi.OutputModeCP437
		log.Printf("Node %d: Output mode forced to CP437 by flag.", nodeID)
	case "auto":
		// Auto mode: Use PTY info if available
		if isPty {
			termType := strings.ToLower(ptyReq.Term)
			log.Printf("Node %d: Auto mode detecting based on TERM='%s'", nodeID, termType)
			// Heuristic: Check for known CP437-preferring TERM types
			if termType == "sync" || termType == "syncterm" || termType == "ansi" || termType == "scoansi" || strings.HasPrefix(termType, "vt100") {
				log.Printf("Node %d: Auto mode selecting CP437 output for TERM='%s'", nodeID, termType)
				effectiveMode = ansi.OutputModeCP437
			} else {
				log.Printf("Node %d: Auto mode selecting UTF-8 output for TERM='%s'", nodeID, termType)
				effectiveMode = ansi.OutputModeUTF8
			}
		} else {
			// No PTY, safer to default to UTF-8? Or CP437?
			// Let's default to UTF-8 for non-PTY as it's more common for raw streams.
			log.Printf("Node %d: Auto mode selecting UTF-8 output (no PTY requested).", nodeID)
			effectiveMode = ansi.OutputModeUTF8
		}
	}

	// --- Create Terminal ---
	log.Printf("Node %d: Creating terminal for session", nodeID)
	terminal := term.NewTerminal(s, "") // Use session 's' as the R/W source for the terminal

	// Set initial terminal size from PTY request (term.NewTerminal defaults to 80 columns)
	if isPty {
		tw, th := int(termWidth.Load()), int(termHeight.Load())
		if tw > 0 && th > 0 {
			_ = terminal.SetSize(tw, th)
			log.Printf("Node %d: Set terminal size to %dx%d", nodeID, tw, th)
		}
	}

	// --- Handle Window Size Changes ---
	// Forward resize events to both our atomic values and the term.Terminal.
	// Guard against nil winCh (ranging a nil channel blocks forever).
	if isPty && winCh != nil {
		go func() {
			for win := range winCh {
				log.Printf("Node %d: Window resize event: %dx%d", nodeID, win.Width, win.Height)
				if win.Width > 0 {
					termWidth.Store(int32(win.Width))
				}
				if win.Height > 0 {
					termHeight.Store(int32(win.Height))
				}
				if win.Width > 0 && win.Height > 0 {
					_ = terminal.SetSize(win.Width, win.Height)
				}
			}
			log.Printf("Node %d: Window change channel closed.", nodeID)
		}()
	}

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

	bbsSession := &session.BbsSession{
		NodeID:       int(nodeID),
		StartTime:    sessionStartTime,
		LastActivity: sessionStartTime,
		CurrentMenu:  "LOGIN",
		RemoteAddr:   s.RemoteAddr(),
	}
	sessionRegistry.Register(bbsSession)

	currentMenuName := "LOGIN"               // Start with LOGIN
	autoRunLog := make(types.AutoRunTracker) // Initialize tracker for this session

	// Check if user is already authenticated via SSH
	sshUsername := s.User()
	if sshUsername != "" {
		log.Printf("Node %d: SSH user '%s' detected, attempting auto-login", nodeID, sshUsername)
		// Try to load the user from the database
		sshUser, found := userMgr.GetUser(sshUsername)
		if found && sshUser != nil {
			// User exists in database, authenticate them automatically
			authenticatedUser = sshUser
			bbsSession.Mutex.Lock()
			bbsSession.User = authenticatedUser
			bbsSession.Mutex.Unlock()
			log.Printf("Node %d: SSH auto-login successful for user '%s' (Handle: %s)", nodeID, sshUsername, sshUser.Handle)

			// Mark user as online
			userMgr.MarkUserOnline(authenticatedUser.ID)

			// Set user's terminal dimensions from detected PTY size only when not already stored.
			tw, th := int(termWidth.Load()), int(termHeight.Load())
			updatedScreenPrefs := false
			if tw > 0 && authenticatedUser.ScreenWidth == 0 {
				authenticatedUser.ScreenWidth = tw
				updatedScreenPrefs = true
			}
			if th > 0 && authenticatedUser.ScreenHeight == 0 {
				authenticatedUser.ScreenHeight = th
				updatedScreenPrefs = true
			}
			if updatedScreenPrefs {
				log.Printf("Node %d: Set user %s screen preferences to %dx%d", nodeID, sshUser.Handle, authenticatedUser.ScreenWidth, authenticatedUser.ScreenHeight)
				if saveErr := userMgr.UpdateUser(authenticatedUser); saveErr != nil {
					log.Printf("ERROR: Node %d: Failed to save user screen preferences: %v", nodeID, saveErr)
				}
			}
			// Set default message area if not already set
			if authenticatedUser.CurrentMessageAreaID == 0 && messageMgr != nil {
				for _, area := range messageMgr.ListAreas() {
					if area.ACSRead == "" || authenticatedUser.AccessLevel > 0 {
						authenticatedUser.CurrentMessageAreaID = area.ID
						authenticatedUser.CurrentMessageAreaTag = area.Tag
						authenticatedUser.CurrentMsgConferenceID = area.ConferenceID
						if confMgr != nil {
							if conf, ok := confMgr.GetByID(area.ConferenceID); ok {
								authenticatedUser.CurrentMsgConferenceTag = conf.Tag
							}
						}
						break
					}
				}
			}
			// Set default file area if not already set
			if authenticatedUser.CurrentFileAreaID == 0 && fileMgr != nil {
				for _, area := range fileMgr.ListAreas() {
					if area.ACSList == "" || authenticatedUser.AccessLevel > 0 {
						authenticatedUser.CurrentFileAreaID = area.ID
						authenticatedUser.CurrentFileAreaTag = area.Tag
						authenticatedUser.CurrentFileConferenceID = area.ConferenceID
						if confMgr != nil {
							if conf, ok := confMgr.GetByID(area.ConferenceID); ok {
								authenticatedUser.CurrentFileConferenceTag = conf.Tag
							}
						}
						break
					}
				}
			}
			// Persist defaults
			if saveErr := userMgr.UpdateUser(authenticatedUser); saveErr != nil {
				log.Printf("ERROR: Node %d: Failed to save user default area selections: %v", nodeID, saveErr)
			}

			currentMenuName = "MAIN" // Will be overridden by login sequence result
		} else {
			log.Printf("Node %d: SSH user '%s' not found in BBS database, requiring manual login", nodeID, sshUsername)
			// User not in database, proceed with normal LOGIN flow
		}
	}

	// Pre-login matrix screen for telnet users (no SSH auto-login)
	if authenticatedUser == nil && sshUsername == "" {
		matrixAction, matrixErr := menuExecutor.RunMatrixScreen(s, terminal, userMgr, int(nodeID), effectiveMode)
		if matrixErr != nil {
			log.Printf("Node %d: Matrix screen error: %v", nodeID, matrixErr)
			return
		}
		if matrixAction == "DISCONNECT" {
			log.Printf("Node %d: User selected disconnect from matrix screen", nodeID)
			return
		}
		// matrixAction == "LOGIN" — proceed to normal login loop
	}

	// Login Loop
	for authenticatedUser == nil {
		if currentMenuName == "" || currentMenuName == "LOGOFF" {
			log.Printf("Node %d: Login failed or aborted. Terminating session.", nodeID)
			fmt.Fprintln(terminal, "\r\nLogin failed or aborted.")
			return
		}

		// Execute the current menu (e.g., LOGIN)
		// Run returns the next menu name, the authenticated user (if successful), or an error.
		// Pass nodeID directly as int, use sessionStartTime from context
		// Pass the session's autoRunLog
		// Pass "" for currentAreaName during login
		nextMenuName, authUser, execErr := menuExecutor.Run(s, terminal, userMgr, nil, currentMenuName, int(nodeID), sessionStartTime, autoRunLog, effectiveMode, "", int(termWidth.Load()), int(termHeight.Load()))
		if execErr != nil {
			// Log the error and decide how to proceed
			log.Printf("Node %d: Error executing menu '%s': %v", nodeID, currentMenuName, execErr)
			// Optionally display an error message to the user
			fmt.Fprintf(terminal, "\r\nSystem error during menu execution: %v\r\n", execErr)
			// Maybe force logoff or retry?
			currentMenuName = "LOGOFF" // Force logoff on error for now
			continue
		}

		// Check if authentication was successful during this menu execution
		if authUser != nil {
			authenticatedUser = authUser
			bbsSession.Mutex.Lock()
			bbsSession.User = authenticatedUser
			bbsSession.Mutex.Unlock()
			log.Printf("Node %d: User '%s' authenticated successfully.", nodeID, authenticatedUser.Handle)

			// Mark user as online
			userMgr.MarkUserOnline(authenticatedUser.ID)

			// Login successful! Break out of login loop.

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

	// Apply user's stored screen size preferences (caps terminal to user setting)
	if authenticatedUser.ScreenHeight > 0 {
		th := int32(authenticatedUser.ScreenHeight)
		if th < termHeight.Load() || termHeight.Load() == 25 {
			termHeight.Store(th)
		}
	}
	if authenticatedUser.ScreenWidth > 0 {
		tw := int32(authenticatedUser.ScreenWidth)
		if tw < termWidth.Load() || termWidth.Load() == 80 {
			termWidth.Store(tw)
		}
	}
	_ = terminal.SetSize(int(termWidth.Load()), int(termHeight.Load()))
	log.Printf("Node %d: Effective terminal size for %s: %dx%d", nodeID, authenticatedUser.Handle, termWidth.Load(), termHeight.Load())

	// Run the configurable login sequence (login.json) directly after authentication.
	// This replaces the old FASTLOGN menu routing — FASTLOGIN is now an optional login.json item.
	loginNextMenu, loginErr := menuExecutor.RunLoginSequence(s, terminal, userMgr, authenticatedUser, int(nodeID), sessionStartTime, effectiveMode)
	if loginErr != nil {
		if errors.Is(loginErr, io.EOF) {
			log.Printf("Node %d: User disconnected during login sequence.", nodeID)
			return
		}
		log.Printf("ERROR: Node %d: Login sequence error: %v", nodeID, loginErr)
		if loginNextMenu == "" {
			currentMenuName = "LOGOFF"
		} else {
			currentMenuName = loginNextMenu
		}
	} else {
		currentMenuName = loginNextMenu
	}
	for {
		if currentMenuName == "" || currentMenuName == "LOGOFF" {
			log.Printf("Node %d: User %s selected Logoff or reached end state.", nodeID, authenticatedUser.Handle)
			fmt.Fprintln(terminal, "\r\nLogging off...")
			// Add any cleanup tasks before closing the session
			break // Exit the loop
		}

		// *** ADD LOGGING HERE ***
		log.Printf("DEBUG: Node %d: Entering main loop iteration. CurrentMenu: %s, OutputMode: %d", nodeID, currentMenuName, effectiveMode)

		// Update session state for who's online tracking
		bbsSession.Mutex.Lock()
		bbsSession.CurrentMenu = currentMenuName
		bbsSession.LastActivity = time.Now()
		bbsSession.Mutex.Unlock()

		// Execute the current menu (e.g., MAIN, READ_MSG, etc.)
		// Pass nodeID directly as int, use sessionStartTime from context
		// Pass the session's autoRunLog
		// Pass "" for currentAreaName for now (TODO: Pass actual session area name)
		nextMenuName, _, execErr := menuExecutor.Run(s, terminal, userMgr, authenticatedUser, currentMenuName, int(nodeID), sessionStartTime, autoRunLog, effectiveMode, "", int(termWidth.Load()), int(termHeight.Load()))
		if execErr != nil {
			log.Printf("Node %d: Error executing menu '%s': %v", nodeID, currentMenuName, execErr)
			fmt.Fprintf(terminal, "\r\nSystem error during menu execution: %v\r\n", execErr)
			// Logoff on error?
			currentMenuName = "LOGOFF"
			continue
		}

		// Move to the next menu determined by the user's action in the previous menu
		currentMenuName = nextMenuName
	}

	log.Printf("Node %d: Session handler finished for %s.", nodeID, authenticatedUser.Handle)
}

// libsshSessionHandler adapts libssh sessions to the existing BBS session handler
func libsshSessionHandler(sess *sshserver.Session) error {
	// Create adapter that implements ssh.Session interface
	adapter := sshserver.NewBBSSessionAdapter(sess)

	// Atomically check limits and register connection
	canAccept, reason := connectionTracker.TryAccept(adapter.RemoteAddr())
	if !canAccept {
		log.Printf("INFO: Rejecting SSH connection from %s: %s", adapter.RemoteAddr(), reason)
		fmt.Fprintf(adapter, "\r\nConnection rejected: %s\r\n", reason)
		fmt.Fprintf(adapter, "Please try again later.\r\n")
		time.Sleep(2 * time.Second) // Brief delay before closing
		return fmt.Errorf("connection limit exceeded: %s", reason)
	}

	// Connection is registered; ensure it's removed when done
	defer connectionTracker.RemoveConnection(adapter.RemoteAddr())

	// Call the existing session handler with the adapter
	sessionHandler(adapter)

	return nil
}

// telnetSessionHandler adapts telnet sessions to the existing BBS session handler
func telnetSessionHandler(adapter *telnetserver.TelnetSessionAdapter) {
	// Atomically check limits and register connection
	canAccept, reason := connectionTracker.TryAccept(adapter.RemoteAddr())
	if !canAccept {
		log.Printf("INFO: Rejecting Telnet connection from %s: %s", adapter.RemoteAddr(), reason)
		fmt.Fprintf(adapter, "\r\nConnection rejected: %s\r\n", reason)
		fmt.Fprintf(adapter, "Please try again later.\r\n")
		time.Sleep(2 * time.Second) // Brief delay before closing
		return
	}

	// Connection is registered; ensure it's removed when done
	defer connectionTracker.RemoveConnection(adapter.RemoteAddr())

	sessionHandler(adapter)
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

	// Load server configuration
	serverConfig, err := config.LoadServerConfig(rootConfigPath)
	if err != nil {
		log.Fatalf("Failed to load server configuration: %v", err)
	}

	// Initialize connection tracker with configured limits and IP filter file paths
	// This will load the initial lists and start watching for file changes
	connectionTracker = NewConnectionTracker(
		serverConfig.MaxNodes,
		serverConfig.MaxConnectionsPerIP,
		serverConfig.MaxFailedLogins,
		serverConfig.LockoutMinutes,
		serverConfig.IPBlocklistPath,
		serverConfig.IPAllowlistPath,
	)
	defer connectionTracker.StopWatching() // Ensure file watcher is stopped on shutdown

	log.Printf("INFO: Connection security configured - Max Nodes: %d, Max Connections Per IP: %d, Max Failed Logins: %d, Lockout: %d min",
		serverConfig.MaxNodes, serverConfig.MaxConnectionsPerIP, serverConfig.MaxFailedLogins, serverConfig.LockoutMinutes)

	// Log IP filter status
	if serverConfig.IPBlocklistPath != "" {
		log.Printf("INFO: IP blocklist enabled from %s (auto-reload on file change)", serverConfig.IPBlocklistPath)
	}
	if serverConfig.IPAllowlistPath != "" {
		log.Printf("INFO: IP allowlist enabled from %s (auto-reload on file change)", serverConfig.IPAllowlistPath)
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

	// Load FTN configuration early so message manager can use per-network tearlines.
	ftnConfig, ftnErr := config.LoadFTNConfig(rootConfigPath)
	if ftnErr != nil {
		log.Printf("ERROR: Failed to load FTN config: %v. Echomail disabled.", ftnErr)
	}
	networkTearlines := make(map[string]string)
	if ftnErr == nil {
		for name, netCfg := range ftnConfig.Networks {
			if strings.TrimSpace(netCfg.Tearline) == "" {
				continue
			}
			networkTearlines[strings.ToLower(strings.TrimSpace(name))] = netCfg.Tearline
		}
	}
	if len(networkTearlines) == 0 {
		networkTearlines = nil
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

	// Initialize MessageManager (areas config from configs/, message data from data/)
	messageMgr, err = message.NewMessageManager(dataPath, rootConfigPath, serverConfig.BoardName, networkTearlines)
	if err != nil {
		log.Fatalf("Failed to initialize message manager: %v", err)
	}
	defer messageMgr.Close() // Ensure JAM bases are closed on shutdown

	// Initialize FileManager (using dataPath)
	fileMgr, err = file.NewFileManager(dataPath, rootConfigPath)
	if err != nil {
		log.Fatalf("Failed to initialize file manager: %v", err)
	}

	// Initialize ConferenceManager (non-fatal if conferences.json is missing)
	confMgr, err = conference.NewConferenceManager(rootConfigPath)
	if err != nil {
		log.Printf("WARN: Failed to initialize conference manager: %v. Conferences disabled.", err)
		confMgr = nil
	}

	// Load login sequence configuration
	loginSequence, err := config.LoadLoginSequence(rootConfigPath)
	if err != nil {
		log.Fatalf("Failed to load login sequence configuration: %v", err)
	}

	// Initialize session registry for who's online tracking
	sessionRegistry = session.NewSessionRegistry()

	// Initialize global chat room for teleconference
	chatRoom := chat.NewChatRoom(100)

	// Initialize MenuExecutor with new paths, loaded theme, server config, message manager, and connection tracker
	menuExecutor = menu.NewExecutor(menuSetPath, rootConfigPath, rootAssetsPath, oneliners, loadedDoors, loadedStrings, loadedTheme, serverConfig, messageMgr, fileMgr, confMgr, connectionTracker, loginSequence, sessionRegistry, chatRoom)

	// Initialize configuration file watcher for hot reload
	var serverConfigMu sync.RWMutex
	configWatcher, err := NewConfigWatcher(rootConfigPath, menuSetPath, menuExecutor, &serverConfig, &serverConfigMu)
	if err != nil {
		log.Printf("WARN: Failed to start config file watcher: %v. Hot reload disabled.", err)
	} else {
		defer configWatcher.Stop()
		log.Printf("INFO: Configuration hot reload enabled for doors.json, login.json, strings.json, theme.json, server.json")
	}

	if ftnErr == nil && len(ftnConfig.Networks) > 0 {
		log.Printf("INFO: Internal FTN tosser disabled; use external tosser (e.g., hpt).")
	}

	// Load event scheduler configuration
	eventsConfig, eventsErr := config.LoadEventsConfig(rootConfigPath)
	if eventsErr != nil {
		log.Printf("WARN: Failed to load events config: %v", eventsErr)
		eventsConfig = config.EventsConfig{Enabled: false}
	}

	// Start event scheduler if enabled
	var eventScheduler *scheduler.Scheduler
	var schedulerCtx context.Context
	var schedulerCancel context.CancelFunc
	if eventsConfig.Enabled {
		historyPath := filepath.Join(dataPath, "logs", "event_history.json")
		eventScheduler = scheduler.NewScheduler(eventsConfig, historyPath)
		schedulerCtx, schedulerCancel = context.WithCancel(context.Background())
		defer func() {
			if schedulerCancel != nil {
				log.Printf("INFO: Shutting down event scheduler...")
				schedulerCancel()
			}
		}()

		go eventScheduler.Start(schedulerCtx)
		log.Printf("INFO: Event scheduler started with %d events", len(eventsConfig.Events))
	} else {
		log.Printf("INFO: Event scheduler disabled")
	}

	// Host key path for libssh
	hostKeyPath := filepath.Join(rootConfigPath, "ssh_host_rsa_key")

	// Verify host key exists
	if _, err := os.Stat(hostKeyPath); err != nil {
		log.Fatalf("FATAL: Host key not found at %s: %v", hostKeyPath, err)
	}
	log.Printf("INFO: Host key found at %s", hostKeyPath)

	// Ensure at least one protocol is enabled
	if !serverConfig.SSHEnabled && !serverConfig.TelnetEnabled {
		log.Fatalf("FATAL: Neither SSH nor Telnet is enabled in config. Enable at least one protocol.")
	}

	// Start SSH server if enabled
	var server *sshserver.Server
	if serverConfig.SSHEnabled {
		sshPort := serverConfig.SSHPort
		sshHost := serverConfig.SSHHost
		log.Printf("INFO: Configuring libssh SSH server on %s:%d...", sshHost, sshPort)

		var err error
		server, err = sshserver.NewServer(sshserver.Config{
			HostKeyPath:    hostKeyPath,
			Port:           sshPort,
			SessionHandler: libsshSessionHandler,
			AuthPasswordFunc: func(username, password string) bool {
				// If user exists in BBS database, validate password
				u, found := userMgr.GetUser(username)
				if !found {
					// Unknown user — allow through to BBS login/new user flow
					return true
				}
				// Existing user — verify bcrypt password
				_, ok := userMgr.Authenticate(username, password)
				if !ok {
					log.Printf("WARN: SSH password auth failed for existing user: %s", username)
				} else {
					log.Printf("INFO: SSH password auth verified for user: %s (ID: %d)", username, u.ID)
				}
				return ok
			},
		})
		if err != nil {
			log.Fatalf("FATAL: Failed to create SSH server: %v", err)
		}
		defer server.Close()
		defer sshserver.Cleanup()

		if err := server.Listen(); err != nil {
			log.Fatalf("FATAL: Failed to start SSH server: %v", err)
		}

		log.Printf("INFO: SSH server ready - connect via: ssh <username>@%s -p %d", sshHost, sshPort)
	} else {
		log.Printf("INFO: SSH server disabled in config")
	}

	// Start telnet server if enabled
	if serverConfig.TelnetEnabled {
		telnetPort := serverConfig.TelnetPort
		telnetHost := serverConfig.TelnetHost
		log.Printf("INFO: Configuring telnet server on %s:%d...", telnetHost, telnetPort)

		telnetSrv, telnetErr := telnetserver.NewServer(telnetserver.Config{
			Port:           telnetPort,
			Host:           telnetHost,
			SessionHandler: telnetSessionHandler,
		})
		if telnetErr != nil {
			log.Fatalf("FATAL: Failed to create telnet server: %v", telnetErr)
		}
		defer telnetSrv.Close()

		go func() {
			if listenErr := telnetSrv.ListenAndServe(); listenErr != nil {
				log.Printf("ERROR: Telnet server error: %v", listenErr)
			}
		}()

		log.Printf("INFO: Telnet server ready - connect via: telnet %s %d", telnetHost, telnetPort)
	} else {
		log.Printf("INFO: Telnet server disabled in config")
	}

	// Main accept loop — SSH accepts if enabled, otherwise block on signal
	if server != nil {
		for {
			if err := server.Accept(); err != nil {
				log.Printf("ERROR: Failed to accept connection: %v", err)
				time.Sleep(100 * time.Millisecond)
			}
		}
	} else {
		// Telnet-only mode: block until interrupted
		log.Printf("INFO: Running in telnet-only mode")
		select {}
	}

}

// --- SSH Helper Functions ---

// parsePtyRequest parses a PTY request payload
func parsePtyRequest(payload []byte) (ssh.Pty, bool) {
	if len(payload) < 4 {
		return ssh.Pty{}, false
	}
	termLen := binary.BigEndian.Uint32(payload[:4])
	if len(payload) < int(4+termLen+16) {
		return ssh.Pty{}, false
	}
	term := string(payload[4 : 4+termLen])
	w := binary.BigEndian.Uint32(payload[4+termLen:])
	h := binary.BigEndian.Uint32(payload[8+termLen:])
	return ssh.Pty{
		Term: term,
		Window: ssh.Window{
			Width:  int(w),
			Height: int(h),
		},
	}, true
}

// parseWinchRequest parses a window-change request payload
func parseWinchRequest(payload []byte) (ssh.Window, bool) {
	if len(payload) < 8 {
		return ssh.Window{}, false
	}
	w := binary.BigEndian.Uint32(payload[:4])
	h := binary.BigEndian.Uint32(payload[4:8])
	return ssh.Window{Width: int(w), Height: int(h)}, true
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddInt32(&nodeCounter, 1))
}

// handleSSHConnection handles an SSH connection
func handleSSHConnection(tcpConn net.Conn, sshConfig *gossh.ServerConfig) {
	defer tcpConn.Close()

	remoteAddr := tcpConn.RemoteAddr().String()
	log.Printf("New TCP connection from %s", remoteAddr)

	// Set a deadline for the handshake to prevent hanging connections
	tcpConn.SetDeadline(time.Now().Add(30 * time.Second))

	// Perform SSH handshake
	log.Printf("DEBUG: Starting SSH handshake with %s", remoteAddr)
	sshConn, chans, reqs, err := gossh.NewServerConn(tcpConn, sshConfig)
	if err != nil {
		log.Printf("ERROR: SSH handshake failed from %s: %v", remoteAddr, err)
		log.Printf("DEBUG: This typically means algorithm negotiation failed or client disconnected")
		return
	}
	defer sshConn.Close()

	// Clear the handshake deadline so the session doesn't timeout after 30s
	tcpConn.SetDeadline(time.Time{})

	log.Printf("SSH handshake successful from %s (user: %s)", tcpConn.RemoteAddr(), sshConn.User())

	// Discard global requests
	go gossh.DiscardRequests(reqs)

	// Handle incoming channels
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(gossh.UnknownChannelType, "unknown channel type")
			log.Printf("Rejected channel type: %s", newChannel.ChannelType())
			continue
		}

		// Accept session channel
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Failed to accept channel: %v", err)
			continue
		}

		// Handle session in goroutine
		go handleSessionChannel(sshConn, channel, requests)
	}
}

// handleSessionChannel processes a session channel and its requests
func handleSessionChannel(conn *gossh.ServerConn, channel gossh.Channel, requests <-chan *gossh.Request) {
	defer channel.Close()

	// Create session adapter
	baseCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sessionID := generateSessionID()

	sessionCtx := &SessionContext{
		ctx:        baseCtx,
		sessionID:  sessionID,
		user:       conn.User(),
		remoteAddr: conn.RemoteAddr(),
		localAddr:  conn.LocalAddr(),
		clientVer:  string(conn.ClientVersion()),
		serverVer:  string(conn.ServerVersion()),
		values:     make(map[interface{}]interface{}),
	}

	adapter := &SessionAdapter{
		conn:       conn,
		channel:    channel,
		requests:   requests,
		user:       conn.User(),
		remoteAddr: conn.RemoteAddr(),
		localAddr:  conn.LocalAddr(),
		environ:    make([]string, 0),
		command:    make([]string, 0),
		ctx:        sessionCtx,
		cancel:     cancel,
		winch:      make(chan ssh.Window, 1),
	}

	// Process channel requests until shell/exec
	shellRequested := false
	for req := range requests {
		switch req.Type {
		case "pty-req":
			pty, ok := parsePtyRequest(req.Payload)
			if ok {
				adapter.ptyMutex.Lock()
				adapter.pty = &pty
				adapter.hasPty = true
				select {
				case adapter.winch <- pty.Window:
				default:
				}
				adapter.ptyMutex.Unlock()
				req.Reply(true, nil)
				log.Printf("PTY Request accepted: Term=%s, Width=%d, Height=%d",
					pty.Term, pty.Window.Width, pty.Window.Height)
			} else {
				req.Reply(false, nil)
				log.Printf("PTY Request failed to parse")
			}

		case "window-change":
			win, ok := parseWinchRequest(req.Payload)
			if ok && adapter.hasPty {
				adapter.ptyMutex.Lock()
				adapter.pty.Window = win
				select {
				case adapter.winch <- win:
				default:
				}
				adapter.ptyMutex.Unlock()
				req.Reply(true, nil)
				log.Printf("Window change: %dx%d", win.Width, win.Height)
			} else {
				req.Reply(false, nil)
			}

		case "env":
			// Parse KEY=VALUE from payload
			if len(req.Payload) >= 8 {
				keyLen := binary.BigEndian.Uint32(req.Payload[:4])
				if len(req.Payload) >= int(8+keyLen) {
					key := string(req.Payload[4 : 4+keyLen])
					valLen := binary.BigEndian.Uint32(req.Payload[4+keyLen:])
					if len(req.Payload) >= int(8+keyLen+valLen) {
						val := string(req.Payload[8+keyLen : 8+keyLen+valLen])
						adapter.environ = append(adapter.environ, fmt.Sprintf("%s=%s", key, val))
					}
				}
			}
			req.Reply(true, nil)

		case "shell":
			req.Reply(true, nil)
			shellRequested = true
			log.Printf("Shell request accepted")
			// Continue processing requests in background for window-change
			go func() {
				for req := range requests {
					if req.Type == "window-change" {
						win, ok := parseWinchRequest(req.Payload)
						if ok && adapter.hasPty {
							adapter.ptyMutex.Lock()
							adapter.pty.Window = win
							select {
							case adapter.winch <- win:
							default:
							}
							adapter.ptyMutex.Unlock()
							req.Reply(true, nil)
						} else {
							req.Reply(false, nil)
						}
					} else {
						req.Reply(false, nil)
					}
				}
			}()
			// Invoke BBS session handler
			sessionHandler(adapter)
			return

		case "exec":
			// Parse command
			if len(req.Payload) >= 4 {
				cmdLen := binary.BigEndian.Uint32(req.Payload[:4])
				if len(req.Payload) >= int(4+cmdLen) {
					cmd := string(req.Payload[4 : 4+cmdLen])
					adapter.command = strings.Fields(cmd)
				}
			}
			req.Reply(true, nil)
			shellRequested = true
			log.Printf("Exec request: %v", adapter.command)
			// Background request processing (same as shell)
			go func() {
				for req := range requests {
					if req.Type == "window-change" {
						win, ok := parseWinchRequest(req.Payload)
						if ok && adapter.hasPty {
							adapter.ptyMutex.Lock()
							adapter.pty.Window = win
							select {
							case adapter.winch <- win:
							default:
							}
							adapter.ptyMutex.Unlock()
							req.Reply(true, nil)
						} else {
							req.Reply(false, nil)
						}
					} else {
						req.Reply(false, nil)
					}
				}
			}()
			sessionHandler(adapter)
			return

		default:
			req.Reply(false, nil)
		}
	}

	// Channel closed without shell/exec request
	if !shellRequested {
		log.Printf("Channel closed without shell/exec request")
	}
}

// --- Helper Functions (Existing loadHostKey, sendEnv) ---
func loadHostKey(path string) gossh.Signer {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("FATAL: Failed to read host key %s: %v", path, err)
	}
	signer, err := gossh.ParsePrivateKey(keyBytes)
	if err != nil {
		log.Fatalf("FATAL: Failed to parse host key %s: %v", path, err)
	}
	log.Printf("INFO: Host key loaded successfully from %s", path)
	log.Printf("DEBUG: Host key type: %s", signer.PublicKey().Type())
	return signer
}

func sendEnv(s *SessionAdapter, name, value string) {
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
