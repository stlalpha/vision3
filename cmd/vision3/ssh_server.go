package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/sshserver"
)

// startSSHServer creates, configures, and starts the pure-Go SSH server.
// Returns a cleanup function to shut down the server.
func startSSHServer(hostKeyPath, sshHost string, sshPort int, legacyAlgorithms bool) (func(), error) {
	log.Printf("INFO: Configuring SSH server on %s:%d...", sshHost, sshPort)

	server, err := sshserver.NewServer(sshserver.Config{
		HostKeyPath:         hostKeyPath,
		Host:                sshHost,
		Port:                sshPort,
		LegacySSHAlgorithms: legacyAlgorithms,
		SessionHandler:      sshSessionHandler,
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			username := ctx.User()
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
		return nil, fmt.Errorf("failed to create SSH server: %w", err)
	}

	cleanup := func() {
		server.Close()
		sshserver.Cleanup()
	}

	// gliderlabs/ssh handles its own accept loop — run in background
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Printf("ERROR: SSH server error: %v", err)
		}
	}()

	log.Printf("INFO: SSH server ready - connect via: ssh <username>@%s -p %d", sshHost, sshPort)
	return cleanup, nil
}
func sshSessionHandler(sess ssh.Session) {
	// Wrap the session to add SetReadInterrupt support
	wrapped := sshserver.WrapSession(sess)

	// Atomically check limits and register connection
	canAccept, reason := connectionTracker.TryAccept(wrapped.RemoteAddr())
	if !canAccept {
		log.Printf("INFO: Rejecting SSH connection from %s: %s", wrapped.RemoteAddr(), reason)
		fmt.Fprintf(wrapped, "\r\nConnection rejected: %s\r\n", reason)
		fmt.Fprintf(wrapped, "Please try again later.\r\n")
		time.Sleep(2 * time.Second)
		return
	}

	// Connection is registered; ensure it's removed when done
	defer connectionTracker.RemoveConnection(wrapped.RemoteAddr())

	// Call the existing session handler with the wrapped session
	sessionHandler(wrapped)
}
