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
	HostKeyPath                string
	Host                       string
	Port                       int
	LegacySSHAlgorithms        bool
	SessionHandler             func(ssh.Session)
	PasswordHandler            func(ctx ssh.Context, password string) bool
	KeyboardInteractiveHandler func(ctx ssh.Context, challenger gossh.KeyboardInteractiveChallenge) bool
	Version                    string // SSH server banner version (default: "Vision3")
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
		Version:         cfg.Version,
		ConnectionFailedCallback: func(conn net.Conn, err error) {
			log.Printf("WARN: SSH connection failed from %s: %v", conn.RemoteAddr(), err)
		},
	}
	if cfg.KeyboardInteractiveHandler != nil {
		srv.KeyboardInteractiveHandler = cfg.KeyboardInteractiveHandler
	}

	// Configure algorithm suites via ServerConfigCallback.
	// When LegacySSHAlgorithms is enabled, include older algorithms
	// (diffie-hellman-group1-sha1, 3des-cbc, hmac-sha1, ssh-rsa)
	// required by retro BBS clients.
	legacy := cfg.LegacySSHAlgorithms
	srv.ServerConfigCallback = func(ctx ssh.Context) *gossh.ServerConfig {
		sc := &gossh.ServerConfig{}
		if legacy {
			log.Printf("DEBUG: SSH legacy algorithms enabled for retro BBS client compatibility")
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

// readResult holds the outcome of a background read from the SSH channel.
type readResult struct {
	data []byte
	err  error
}

// BBSSession wraps a gliderlabs ssh.Session with SetReadInterrupt support.
// Use WrapSession to create one.
//
// Design invariant: at most ONE goroutine reads from the underlying
// ssh.Session at any time. When a read is interrupted, the orphaned
// goroutine reference is kept in orphanCh so the next Read() call waits
// for it to finish before issuing a new read. This prevents two
// concurrent readers from racing for bytes and silently eating keypresses.
type BBSSession struct {
	ssh.Session
	riMu          sync.Mutex
	readInterrupt <-chan struct{}
	// orphanCh is the result channel from a goroutine that was still
	// blocked on s.Session.Read() when a read interrupt fired. The next
	// Read() waits on this channel first, ensuring only one goroutine
	// is ever reading from the underlying session.
	orphanCh chan readResult
	// pending holds leftover bytes when an orphan drain returned more
	// data than the caller's buffer could hold.
	pending *readResult
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
//
// When an interrupted Read() leaves an orphaned goroutine blocked on the
// underlying session, the next Read() call waits for that goroutine to
// finish first. This guarantees only one reader is active at a time and
// prevents keypresses from being silently consumed by a stale goroutine.
func (s *BBSSession) Read(p []byte) (int, error) {
	s.riMu.Lock()

	// 1. Drain any leftover bytes from a previous orphan drain that
	//    returned more data than the caller's buffer could hold.
	if s.pending != nil {
		res := s.pending
		s.pending = nil
		s.riMu.Unlock()
		if len(res.data) > 0 {
			n := copy(p, res.data)
			if n < len(res.data) {
				s.riMu.Lock()
				s.pending = &readResult{data: res.data[n:], err: res.err}
				s.riMu.Unlock()
				return n, nil
			}
			return n, res.err
		}
		return 0, res.err
	}

	// 2. If a previous Read() was interrupted and left an orphaned
	//    goroutine reading from the session, wait for it to complete
	//    before starting a new read. This is the key invariant: only
	//    one goroutine reads from the underlying session at a time.
	if s.orphanCh != nil {
		ch := s.orphanCh
		s.orphanCh = nil
		s.riMu.Unlock()

		// Block until the orphan completes (user presses a key or EOF).
		// The caller is expecting to block for input anyway.
		res := <-ch
		if len(res.data) > 0 {
			n := copy(p, res.data)
			if n < len(res.data) {
				s.riMu.Lock()
				s.pending = &readResult{data: res.data[n:], err: res.err}
				s.riMu.Unlock()
				return n, nil
			}
			return n, res.err
		}
		if res.err != nil {
			return 0, res.err
		}
		// Orphan returned 0 bytes, no error — fall through to normal read
	} else {
		s.riMu.Unlock()
	}

	// 3. Normal read path.
	s.riMu.Lock()
	interrupt := s.readInterrupt
	s.riMu.Unlock()

	if interrupt == nil {
		// No interrupt registered — direct read (no goroutine overhead)
		return s.Session.Read(p)
	}

	// Check if already interrupted before blocking
	select {
	case <-interrupt:
		return 0, ErrReadInterrupted
	default:
	}

	// Race the read against the interrupt channel.
	// Use a private buffer so the orphaned goroutine doesn't write into
	// the caller's (now-returned) slice.
	buf := make([]byte, len(p))
	ch := make(chan readResult, 1)
	go func() {
		n, err := s.Session.Read(buf)
		ch <- readResult{data: buf[:n], err: err}
	}()

	select {
	case res := <-ch:
		n := copy(p, res.data)
		return n, res.err
	case <-interrupt:
		// Save the orphaned goroutine's channel so the next Read()
		// waits for it instead of starting a competing reader.
		s.riMu.Lock()
		s.orphanCh = ch
		s.riMu.Unlock()
		return 0, ErrReadInterrupted
	}
}
