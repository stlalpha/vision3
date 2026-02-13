package telnetserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gliderlabs/ssh"
)

var telnetSessionCounter int32

// TelnetSessionContext implements context.Context and provides session metadata.
type TelnetSessionContext struct {
	ctx        context.Context
	cancel     context.CancelFunc
	sessionID  string
	remoteAddr net.Addr
	localAddr  net.Addr
	mu         sync.Mutex
	values     map[interface{}]interface{}
}

func (c *TelnetSessionContext) Value(key interface{}) interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.values[key]; ok {
		return v
	}
	return c.ctx.Value(key)
}

func (c *TelnetSessionContext) Deadline() (deadline time.Time, ok bool) {
	return c.ctx.Deadline()
}

func (c *TelnetSessionContext) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *TelnetSessionContext) Err() error {
	return c.ctx.Err()
}

func (c *TelnetSessionContext) Lock() {
	c.mu.Lock()
}

func (c *TelnetSessionContext) Unlock() {
	c.mu.Unlock()
}

func (c *TelnetSessionContext) User() string {
	return "" // Telnet has no username - forces manual login
}

func (c *TelnetSessionContext) SessionID() string {
	return c.sessionID
}

func (c *TelnetSessionContext) ClientVersion() string {
	return "telnet"
}

func (c *TelnetSessionContext) ServerVersion() string {
	return "vision3-telnet"
}

func (c *TelnetSessionContext) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *TelnetSessionContext) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *TelnetSessionContext) Permissions() *ssh.Permissions {
	return nil
}

func (c *TelnetSessionContext) SetValue(key, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = value
}

// TelnetSessionAdapter adapts a telnet connection to the gliderlabs/ssh.Session interface
// so that the existing sessionHandler() works unchanged.
type TelnetSessionAdapter struct {
	telnetConn *TelnetConn
	ctx        *TelnetSessionContext
	winCh      chan ssh.Window
	ptyMu      sync.Mutex // protects pty from concurrent access
	pty        ssh.Pty
}

// NewTelnetSessionAdapter creates an adapter that implements ssh.Session for a telnet connection.
func NewTelnetSessionAdapter(tc *TelnetConn) *TelnetSessionAdapter {
	ctx, cancel := context.WithCancel(context.Background())

	sessionID := fmt.Sprintf("telnet-%d-%d", time.Now().UnixNano(), atomic.AddInt32(&telnetSessionCounter, 1))

	sessCtx := &TelnetSessionContext{
		ctx:        ctx,
		cancel:     cancel,
		sessionID:  sessionID,
		remoteAddr: tc.RemoteAddr(),
		localAddr:  tc.LocalAddr(),
		values:     make(map[interface{}]interface{}),
	}

	w, h := tc.WindowSize()

	adapter := &TelnetSessionAdapter{
		telnetConn: tc,
		ctx:        sessCtx,
		winCh:      make(chan ssh.Window, 1),
		pty: ssh.Pty{
			Term:   "ansi",
			Window: ssh.Window{Width: w, Height: h},
		},
	}

	// Send initial window size
	adapter.winCh <- ssh.Window{Width: w, Height: h}

	// Forward NAWS updates from TelnetConn to the adapter's winCh
	go func() {
		for win := range tc.winCh {
			// Update stored PTY window size under mutex
			adapter.ptyMu.Lock()
			adapter.pty.Window = win
			adapter.ptyMu.Unlock()
			// Forward to the session's window change channel
			select {
			case adapter.winCh <- win:
			default:
			}
		}
	}()

	return adapter
}

// Read reads from the telnet connection (IAC-filtered).
func (a *TelnetSessionAdapter) Read(p []byte) (int, error) {
	return a.telnetConn.Read(p)
}

// Write writes to the telnet connection (IAC-escaped).
func (a *TelnetSessionAdapter) Write(p []byte) (int, error) {
	return a.telnetConn.Write(p)
}

// Close closes the telnet session.
func (a *TelnetSessionAdapter) Close() error {
	a.ctx.cancel()
	return a.telnetConn.Close()
}

// CloseWrite is a no-op for telnet (no half-close support).
func (a *TelnetSessionAdapter) CloseWrite() error {
	return nil
}

// SendRequest is not supported on telnet.
func (a *TelnetSessionAdapter) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	return false, fmt.Errorf("SendRequest not supported on telnet")
}

// Stderr returns a writer for stderr (same as stdout for BBS).
func (a *TelnetSessionAdapter) Stderr() io.ReadWriter {
	return a
}

// User returns empty string for telnet (triggers manual login flow).
func (a *TelnetSessionAdapter) User() string {
	return ""
}

// RemoteAddr returns the remote network address.
func (a *TelnetSessionAdapter) RemoteAddr() net.Addr {
	return a.telnetConn.RemoteAddr()
}

// LocalAddr returns the local network address.
func (a *TelnetSessionAdapter) LocalAddr() net.Addr {
	return a.telnetConn.LocalAddr()
}

// Environ returns an empty environment for telnet.
func (a *TelnetSessionAdapter) Environ() []string {
	return []string{}
}

// Command returns an empty command for telnet (shell session).
func (a *TelnetSessionAdapter) Command() []string {
	return []string{}
}

// RawCommand returns an empty raw command.
func (a *TelnetSessionAdapter) RawCommand() string {
	return ""
}

// Subsystem returns an empty subsystem.
func (a *TelnetSessionAdapter) Subsystem() string {
	return ""
}

// PublicKey returns nil (telnet has no public key auth).
func (a *TelnetSessionAdapter) PublicKey() ssh.PublicKey {
	return nil
}

// Context returns the session context.
func (a *TelnetSessionAdapter) Context() ssh.Context {
	return a.ctx
}

// SessionID returns the unique session identifier.
func (a *TelnetSessionAdapter) SessionID() string {
	return a.ctx.SessionID()
}

// Permissions returns empty permissions.
func (a *TelnetSessionAdapter) Permissions() ssh.Permissions {
	return ssh.Permissions{}
}

// Pty returns the synthesized PTY info with Term="ansi" and NAWS-detected window size.
func (a *TelnetSessionAdapter) Pty() (ssh.Pty, <-chan ssh.Window, bool) {
	a.ptyMu.Lock()
	pty := a.pty
	a.ptyMu.Unlock()
	return pty, a.winCh, true
}

// Exit closes the session.
func (a *TelnetSessionAdapter) Exit(code int) error {
	return a.Close()
}

// Signals is a no-op for telnet.
func (a *TelnetSessionAdapter) Signals(c chan<- ssh.Signal) {
}

// Break is a no-op for telnet.
func (a *TelnetSessionAdapter) Break(c chan<- bool) {
}

// SetReadInterrupt sets a channel that, when closed, causes any blocked Read()
// to return without consuming data. Used to cleanly stop door I/O goroutines.
func (a *TelnetSessionAdapter) SetReadInterrupt(ch <-chan struct{}) {
	a.telnetConn.SetReadInterrupt(ch)
}
