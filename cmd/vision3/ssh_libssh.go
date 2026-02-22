//go:build !windows

package main

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/stlalpha/vision3/internal/sshserver"
)

// sshServer wraps the libssh server to implement the sshAcceptor interface.
type sshServer struct {
	server *sshserver.Server
}

func (s *sshServer) Accept() error {
	return s.server.Accept()
}

func (s *sshServer) Close() error {
	return s.server.Close()
}

// startSSHServer creates and starts a libssh-based SSH server.
// Returns an sshAcceptor for the main accept loop and a cleanup function.
func startSSHServer(hostKeyPath, sshHost string, sshPort int, legacyAlgorithms bool) (sshAcceptor, func(), error) {
	log.Printf("INFO: Configuring libssh SSH server on %s:%d...", sshHost, sshPort)

	server, err := sshserver.NewServer(sshserver.Config{
		HostKeyPath:         hostKeyPath,
		Port:                sshPort,
		LegacySSHAlgorithms: legacyAlgorithms,
		SessionHandler:      libsshSessionHandler,
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
		return nil, nil, fmt.Errorf("failed to create SSH server: %w", err)
	}

	cleanup := func() {
		server.Close()
		sshserver.Cleanup()
	}

	if err := server.Listen(); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to start SSH server: %w", err)
	}

	log.Printf("INFO: SSH server ready - connect via: ssh <username>@%s -p %d", sshHost, sshPort)
	return &sshServer{server: server}, cleanup, nil
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

// Ensure sshServer and io.Closer are compatible at compile time.
var _ io.Closer = (*sshServer)(nil)
