// Package sshserver provides a pure-Go SSH server for Vision/3 BBS.
// It wraps gliderlabs/ssh (which itself wraps golang.org/x/crypto/ssh)
// and adds BBS-specific features like legacy algorithm support for
// retro terminal clients (SyncTERM, NetRunner) and read-interruptible
// sessions for clean door program I/O cancellation.
package sshserver

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// ErrReadInterrupted is returned by BBSSession.Read when a read interrupt fires.
var ErrReadInterrupted = fmt.Errorf("read interrupted")

// Config holds SSH server configuration.
type Config struct {
	HostKeyPath         string
	Host                string
	Port                int
	LegacySSHAlgorithms bool
	SessionHandler      func(ssh.Session)
	PasswordHandler     func(ctx ssh.Context, password string) bool
}

// Server wraps a gliderlabs/ssh server.
type Server struct {
	inner    *ssh.Server
	listener net.Listener
}

// NewServer creates and configures a new SSH server.
func NewServer(cfg Config) (*Server, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	// Read host key
	keyBytes, err := os.ReadFile(cfg.HostKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read host key %s: %w", cfg.HostKeyPath, err)
	}
	signer, err := gossh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse host key: %w", err)
	}

	srv := &ssh.Server{
		Addr:            addr,
		Handler:         cfg.SessionHandler,
		HostSigners:     []ssh.Signer{signer},
		PasswordHandler: cfg.PasswordHandler,
	}

	// Configure algorithm suites via ServerConfigCallback.
	// When LegacySSHAlgorithms is enabled, include older algorithms
	// (diffie-hellman-group1-sha1, 3des-cbc, hmac-sha1, ssh-rsa)
	// required by retro BBS clients.
	legacy := cfg.LegacySSHAlgorithms
	srv.ServerConfigCallback = func(ctx ssh.Context) *gossh.ServerConfig {
		sc := &gossh.ServerConfig{}
		if legacy {
			log.Printf("INFO: SSH legacy algorithms enabled for retro BBS client compatibility")
			sc.Config.KeyExchanges = []string{
				"curve25519-sha256",
				"curve25519-sha256@libssh.org",
				"ecdh-sha2-nistp256",
				"ecdh-sha2-nistp384",
				"ecdh-sha2-nistp521",
				"diffie-hellman-group14-sha256",
				"diffie-hellman-group16-sha512",
				"diffie-hellman-group14-sha1",
				"diffie-hellman-group1-sha1",
			}
			sc.Config.Ciphers = []string{
				"chacha20-poly1305@openssh.com",
				"aes128-gcm@openssh.com",
				"aes256-gcm@openssh.com",
				"aes128-ctr",
				"aes192-ctr",
				"aes256-ctr",
				"aes128-cbc",
				"aes256-cbc",
				"3des-cbc",
			}
			sc.Config.MACs = []string{
				"hmac-sha2-256-etm@openssh.com",
				"hmac-sha2-512-etm@openssh.com",
				"hmac-sha2-256",
				"hmac-sha2-512",
				"hmac-sha1",
			}
		}
		return sc
	}

	return &Server{inner: srv}, nil
}

// ListenAndServe binds to the configured address and serves SSH connections.
// It blocks until the server is closed.
func (s *Server) ListenAndServe() error {
	return s.inner.ListenAndServe()
}

// Serve starts serving on an existing listener. Blocks until closed.
func (s *Server) Serve(l net.Listener) error {
	s.listener = l
	return s.inner.Serve(l)
}

// Close shuts down the server and all active connections.
func (s *Server) Close() error {
	return s.inner.Close()
}

// Cleanup is a no-op retained for API compatibility (was used to call
// ssh_finalize in the old C libssh implementation).
func Cleanup() {}

// BBSSession wraps a gliderlabs ssh.Session with SetReadInterrupt support.
// Use WrapSession to create one.
type BBSSession struct {
	ssh.Session
	riMu          sync.Mutex
	readInterrupt <-chan struct{}
}

// WrapSession wraps a gliderlabs ssh.Session to add BBS-specific features
// (currently: SetReadInterrupt for clean door I/O cancellation).
func WrapSession(s ssh.Session) *BBSSession {
	return &BBSSession{Session: s}
}

// SetReadInterrupt registers a channel that, when closed, causes any
// blocked Read() to return ErrReadInterrupted without consuming data.
// Pass nil to clear the interrupt.
func (s *BBSSession) SetReadInterrupt(ch <-chan struct{}) {
	s.riMu.Lock()
	s.readInterrupt = ch
	s.riMu.Unlock()
}

// Read reads from the underlying SSH channel. If a read interrupt is set
// and fires before data arrives, ErrReadInterrupted is returned.
func (s *BBSSession) Read(p []byte) (int, error) {
	s.riMu.Lock()
	interrupt := s.readInterrupt
	s.riMu.Unlock()

	if interrupt == nil {
		// No interrupt — direct read (no goroutine overhead)
		return s.Session.Read(p)
	}

	// Check if already interrupted before blocking
	select {
	case <-interrupt:
		return 0, ErrReadInterrupted
	default:
	}

	// Race the read against the interrupt channel
	type readResult struct {
		n   int
		err error
	}
	ch := make(chan readResult, 1)
	go func() {
		n, err := s.Session.Read(p)
		ch <- readResult{n, err}
	}()

	select {
	case res := <-ch:
		return res.n, res.err
	case <-interrupt:
		return 0, ErrReadInterrupted
	}
}
