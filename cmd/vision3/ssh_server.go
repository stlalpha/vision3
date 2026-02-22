package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/sshserver"
	gossh "golang.org/x/crypto/ssh"
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
		Version:             "Vision3",
		// BBS handles its own login flow — accept all SSH auth methods.
		// The PasswordHandler and KeyboardInteractiveHandler both return true
		// so any SSH client (SyncTERM, NetRunner, OpenSSH, etc.) can connect
		// regardless of which auth method it prefers.
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			log.Printf("DEBUG: SSH password auth from user=%q addr=%s", ctx.User(), ctx.RemoteAddr())
			return true
		},
		KeyboardInteractiveHandler: func(ctx ssh.Context, challenger gossh.KeyboardInteractiveChallenge) bool {
			log.Printf("DEBUG: SSH keyboard-interactive auth from user=%q addr=%s", ctx.User(), ctx.RemoteAddr())
			return true
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
