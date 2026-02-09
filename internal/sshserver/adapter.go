package sshserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/gliderlabs/ssh"
)

// BBSSessionAdapter adapts libssh Session to gliderlabs/ssh.Session interface
type BBSSessionAdapter struct {
	session  *Session
	ctx      *BBSSessionContext
	winCh    chan ssh.Window
	commands []string
	env      []string
}

// BBSSessionContext implements context.Context and ssh.Context
type BBSSessionContext struct {
	ctx      context.Context
	session  *Session
	values   map[interface{}]interface{}
}

func (c *BBSSessionContext) Value(key interface{}) interface{} {
	if v, ok := c.values[key]; ok {
		return v
	}
	return c.ctx.Value(key)
}

func (c *BBSSessionContext) Deadline() (deadline time.Time, ok bool) {
	return c.ctx.Deadline()
}

func (c *BBSSessionContext) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *BBSSessionContext) Err() error {
	return c.ctx.Err()
}

func (c *BBSSessionContext) User() string {
	return c.session.User
}

func (c *BBSSessionContext) SessionID() string {
	return fmt.Sprintf("libssh-%p", c.session.Session)
}

func (c *BBSSessionContext) ClientVersion() string {
	return "libssh-client"
}

func (c *BBSSessionContext) ServerVersion() string {
	return "libssh-0.10.6"
}

func (c *BBSSessionContext) RemoteAddr() net.Addr {
	return c.session.RemoteAddr
}

func (c *BBSSessionContext) LocalAddr() net.Addr {
	// Return a dummy local addr for now
	addr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:2222")
	return addr
}

func (c *BBSSessionContext) Permissions() *ssh.Permissions {
	return nil
}

func (c *BBSSessionContext) SetValue(key, value interface{}) {
	c.values[key] = value
}

// NewBBSSessionAdapter creates an adapter for BBS session handler
func NewBBSSessionAdapter(sess *Session) *BBSSessionAdapter {
	ctx, cancel := context.WithCancel(context.Background())
	sess.cancel = cancel

	bbsCtx := &BBSSessionContext{
		ctx:     ctx,
		session: sess,
		values:  make(map[interface{}]interface{}),
	}

	sshWinCh := make(chan ssh.Window, 4)

	adapter := &BBSSessionAdapter{
		session:  sess,
		ctx:      bbsCtx,
		winCh:    sshWinCh,
		commands: []string{},
		env:      []string{},
	}

	// Send initial window size if PTY was requested
	if sess.PTY != nil {
		sshWinCh <- ssh.Window{
			Width:  sess.PTY.Width,
			Height: sess.PTY.Height,
		}
	}

	// Bridge window resize events from Session.WinCh (callback-driven)
	// to the ssh.Window channel consumed by the BBS session handler
	go func() {
		for {
			select {
			case w, ok := <-sess.WinCh:
				if !ok {
					return
				}
				select {
				case sshWinCh <- ssh.Window{Width: w.Width, Height: w.Height}:
				default:
					// Drop if full, resize is best-effort
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return adapter
}

// Implement ssh.Session interface

func (a *BBSSessionAdapter) User() string {
	return a.session.User
}

func (a *BBSSessionAdapter) RemoteAddr() net.Addr {
	return a.session.RemoteAddr
}

func (a *BBSSessionAdapter) LocalAddr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:2222")
	return addr
}

func (a *BBSSessionAdapter) Environ() []string {
	return a.env
}

func (a *BBSSessionAdapter) Command() []string {
	return a.commands
}

func (a *BBSSessionAdapter) Subsystem() string {
	return ""
}

func (a *BBSSessionAdapter) PublicKey() ssh.PublicKey {
	return nil
}

func (a *BBSSessionAdapter) Context() context.Context {
	return a.ctx
}

func (a *BBSSessionAdapter) SessionID() string {
	return a.ctx.SessionID()
}

func (a *BBSSessionAdapter) Pty() (ssh.Pty, <-chan ssh.Window, bool) {
	if a.session.PTY == nil {
		return ssh.Pty{}, nil, false
	}

	pty := ssh.Pty{
		Term:   a.session.PTY.Term,
		Window: ssh.Window{Width: a.session.PTY.Width, Height: a.session.PTY.Height},
	}

	return pty, a.winCh, true
}

func (a *BBSSessionAdapter) Signals(c chan<- ssh.Signal) {
	// Ignore signals for now
}

func (a *BBSSessionAdapter) Break(c chan<- bool) {
	// Ignore break requests for now
}

func (a *BBSSessionAdapter) Exit(code int) error {
	return a.Close()
}

func (a *BBSSessionAdapter) Read(p []byte) (int, error) {
	return a.session.Read(p)
}

func (a *BBSSessionAdapter) Write(p []byte) (int, error) {
	return a.session.Write(p)
}

func (a *BBSSessionAdapter) Close() error {
	return a.session.Close()
}

func (a *BBSSessionAdapter) CloseWrite() error {
	// libssh doesn't have separate write close, just close the whole channel
	return nil
}

// SendRequest is not implemented for libssh
func (a *BBSSessionAdapter) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	return false, fmt.Errorf("SendRequest not supported in libssh adapter")
}

// Stderr returns a writer for stderr (not separated in BBS context)
func (a *BBSSessionAdapter) Stderr() io.ReadWriter {
	return a // Just return self, stderr = stdout for BBS
}

// RawCommand returns the raw command string (empty for shell sessions)
func (a *BBSSessionAdapter) RawCommand() string {
	return ""
}

// Permissions returns SSH permissions
func (a *BBSSessionAdapter) Permissions() ssh.Permissions {
	return ssh.Permissions{}
}
