package sshauth

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/user"
)

// SSHAuthenticator handles SSH authentication with brute force protection
type SSHAuthenticator struct {
	userMgr           *user.UserMgr
	config            config.SSHAuthConfig
	failedAttempts    map[string][]time.Time // IP -> list of failed attempt times
	activeConnections map[string]int         // IP -> connection count
	mutex             sync.RWMutex
}

// NewSSHAuthenticator creates a new SSH authenticator
func NewSSHAuthenticator(userMgr *user.UserMgr, authConfig config.SSHAuthConfig) *SSHAuthenticator {
	return &SSHAuthenticator{
		userMgr:           userMgr,
		config:            authConfig,
		failedAttempts:    make(map[string][]time.Time),
		activeConnections: make(map[string]int),
	}
}

// extractIP extracts IP address from remote address string
func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr // Fallback to original string
	}
	return host
}

// isRateLimited checks if an IP is currently rate limited
func (a *SSHAuthenticator) isRateLimited(ip string) bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	
	attempts, exists := a.failedAttempts[ip]
	if !exists {
		return false
	}
	
	// Clean up old attempts outside the rate limit window
	cutoff := time.Now().Add(-time.Duration(a.config.RateLimitDuration) * time.Second)
	validAttempts := 0
	for _, attemptTime := range attempts {
		if attemptTime.After(cutoff) {
			validAttempts++
		}
	}
	
	return validAttempts >= a.config.MaxFailedAttempts
}

// recordFailedAttempt records a failed authentication attempt
func (a *SSHAuthenticator) recordFailedAttempt(ip string) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	
	now := time.Now()
	if _, exists := a.failedAttempts[ip]; !exists {
		a.failedAttempts[ip] = make([]time.Time, 0)
	}
	
	// Add the new attempt
	a.failedAttempts[ip] = append(a.failedAttempts[ip], now)
	
	// Clean up old attempts
	cutoff := now.Add(-time.Duration(a.config.RateLimitDuration) * time.Second)
	validAttempts := make([]time.Time, 0)
	for _, attemptTime := range a.failedAttempts[ip] {
		if attemptTime.After(cutoff) {
			validAttempts = append(validAttempts, attemptTime)
		}
	}
	a.failedAttempts[ip] = validAttempts
}

// incrementConnection increments the connection count for an IP
func (a *SSHAuthenticator) incrementConnection(ip string) bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	
	count := a.activeConnections[ip]
	if count >= a.config.MaxConnectionsPerIP {
		return false // Too many connections
	}
	
	a.activeConnections[ip] = count + 1
	return true
}

// decrementConnection decrements the connection count for an IP
func (a *SSHAuthenticator) decrementConnection(ip string) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	
	if count, exists := a.activeConnections[ip]; exists && count > 0 {
		a.activeConnections[ip] = count - 1
		if a.activeConnections[ip] == 0 {
			delete(a.activeConnections, ip)
		}
	}
}

// PasswordHandler returns an SSH password authentication handler
func (a *SSHAuthenticator) PasswordHandler() ssh.PasswordHandler {
	return func(ctx ssh.Context, password string) bool {
		username := ctx.User()
		remoteAddr := ctx.RemoteAddr().String()
		ip := extractIP(remoteAddr)
		
		log.Printf("DEBUG: SSH auth attempt from %s for user '%s'", remoteAddr, username)
		
		// Check rate limiting
		if a.isRateLimited(ip) {
			log.Printf("WARN: Rate limited IP %s attempting to connect", ip)
			time.Sleep(5 * time.Second) // Slow down brute force attempts
			return false
		}
		
		// Check connection limits
		if !a.incrementConnection(ip) {
			log.Printf("WARN: Too many connections from IP %s", ip)
			return false
		}
		
		// Handle special "new" username for registration
		if username == "new" {
			if !a.config.AllowNewUsers {
				log.Printf("INFO: New user registration disabled, rejecting 'new' user from %s", ip)
				a.decrementConnection(ip)
				a.recordFailedAttempt(ip)
				return false
			}
			
			// Validate password strength for new users
			if len(password) < a.config.MinPasswordLength {
				log.Printf("INFO: New user password too short from %s (got %d, need %d)", ip, len(password), a.config.MinPasswordLength)
				a.decrementConnection(ip)
				a.recordFailedAttempt(ip)
				return false
			}
			
			log.Printf("INFO: New user registration attempt accepted from %s", ip)
			return true
		}
		
		// Handle existing users
		_, exists := a.userMgr.GetUser(username)
		if !exists {
			log.Printf("INFO: Unknown user '%s' from %s", username, ip)
			a.decrementConnection(ip)
			a.recordFailedAttempt(ip)
			return false
		}
		
		// Validate password using Authenticate method
		_, authenticated := a.userMgr.Authenticate(username, password)
		if !authenticated {
			log.Printf("INFO: Invalid password for user '%s' from %s", username, ip)
			a.decrementConnection(ip)
			a.recordFailedAttempt(ip)
			return false
		}
		
		log.Printf("INFO: Successfully authenticated user '%s' from %s", username, ip)
		return true
	}
}

// ConnectionHandler returns a connection handler that tracks connections
func (a *SSHAuthenticator) ConnectionHandler(originalHandler ssh.Handler) ssh.Handler {
	return func(s ssh.Session) {
		remoteAddr := s.RemoteAddr().String()
		ip := extractIP(remoteAddr)
		
		// Ensure connection count is decremented when session ends
		defer a.decrementConnection(ip)
		
		// Call the original handler
		originalHandler(s)
	}
}

// IsNewUser checks if the authenticated user is registering (username "new")
func (a *SSHAuthenticator) IsNewUser(username string) bool {
	return username == "new"
}

// ValidateNewUserPassword validates a password for new user registration
func (a *SSHAuthenticator) ValidateNewUserPassword(password string) bool {
	return len(password) >= a.config.MinPasswordLength
}

// GetConfig returns the current SSH auth configuration
func (a *SSHAuthenticator) GetConfig() config.SSHAuthConfig {
	return a.config
}

// UpdateConfig updates the SSH auth configuration
func (a *SSHAuthenticator) UpdateConfig(newConfig config.SSHAuthConfig) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.config = newConfig
}

// GetConnectionStats returns connection statistics
func (a *SSHAuthenticator) GetConnectionStats() map[string]interface{} {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	
	totalConnections := 0
	for _, count := range a.activeConnections {
		totalConnections += count
	}
	
	rateLimitedIPs := 0
	for ip := range a.failedAttempts {
		if a.isRateLimited(ip) {
			rateLimitedIPs++
		}
	}
	
	return map[string]interface{}{
		"totalActiveConnections": totalConnections,
		"uniqueIPs":              len(a.activeConnections),
		"rateLimitedIPs":         rateLimitedIPs,
		"maxConnectionsPerIP":    a.config.MaxConnectionsPerIP,
		"newRegistrationEnabled": a.config.AllowNewUsers,
	}
}