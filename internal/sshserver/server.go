package sshserver

/*
#cgo pkg-config: libssh
#include <libssh/libssh.h>
#include <libssh/server.h>
#include <libssh/callbacks.h>
#include <stdlib.h>
#include <string.h>

// Forward declarations of Go callback functions (defined in callbacks.go via //export)
extern int go_auth_password_cb(ssh_session session, const char *user, const char *password, void *userdata);
extern int go_auth_none_cb(ssh_session session, const char *user, void *userdata);
extern ssh_channel go_channel_open_cb(ssh_session session, void *userdata);
extern int go_channel_data_cb(ssh_session session, ssh_channel channel, void *data, uint32_t len, int is_stderr, void *userdata);
extern int go_channel_pty_request_cb(ssh_session session, ssh_channel channel, const char *term, int width, int height, int pxwidth, int pxheight, void *userdata);
extern int go_channel_shell_request_cb(ssh_session session, ssh_channel channel, void *userdata);
extern int go_channel_pty_window_change_cb(ssh_session session, ssh_channel channel, int width, int height, int pxwidth, int pxheight, void *userdata);
extern void go_channel_close_cb(ssh_session session, ssh_channel channel, void *userdata);
extern void go_channel_eof_cb(ssh_session session, ssh_channel channel, void *userdata);

// Allocate and initialize server callbacks.
// ssh_callbacks_init is a C macro and cannot be called from Go.
struct ssh_server_callbacks_struct* vision3_new_server_cb(void *userdata) {
	struct ssh_server_callbacks_struct *cb = calloc(1, sizeof(*cb));
	if (!cb) return NULL;
	cb->userdata = userdata;
	cb->auth_password_function = go_auth_password_cb;
	cb->auth_none_function = go_auth_none_cb;
	cb->channel_open_request_session_function = go_channel_open_cb;
	ssh_callbacks_init(cb);
	return cb;
}

// Set supported auth methods on session.
// SSH_AUTH_METHOD_* are #define macros with 'u' suffix, not accessible from Go.
void vision3_set_auth_methods(ssh_session session) {
	ssh_set_auth_methods(session, SSH_AUTH_METHOD_PASSWORD | SSH_AUTH_METHOD_NONE);
}

// Allocate and initialize channel callbacks.
struct ssh_channel_callbacks_struct* vision3_new_channel_cb(void *userdata) {
	struct ssh_channel_callbacks_struct *cb = calloc(1, sizeof(*cb));
	if (!cb) return NULL;
	cb->userdata = userdata;
	cb->channel_data_function = go_channel_data_cb;
	cb->channel_pty_request_function = go_channel_pty_request_cb;
	cb->channel_shell_request_function = go_channel_shell_request_cb;
	cb->channel_pty_window_change_function = go_channel_pty_window_change_cb;
	cb->channel_close_function = go_channel_close_cb;
	cb->channel_eof_function = go_channel_eof_cb;
	ssh_callbacks_init(cb);
	return cb;
}

// Expose SSH_BIND_OPTIONS enum values via macros for CGO access.
// These are needed for legacy algorithm configuration.
#define VISION3_SSH_BIND_OPTIONS_HOSTKEY_ALGORITHMS SSH_BIND_OPTIONS_HOSTKEY_ALGORITHMS
#define VISION3_SSH_BIND_OPTIONS_KEY_EXCHANGE SSH_BIND_OPTIONS_KEY_EXCHANGE
#define VISION3_SSH_BIND_OPTIONS_CIPHERS_C_S SSH_BIND_OPTIONS_CIPHERS_C_S
#define VISION3_SSH_BIND_OPTIONS_CIPHERS_S_C SSH_BIND_OPTIONS_CIPHERS_S_C
#define VISION3_SSH_BIND_OPTIONS_HMAC_C_S SSH_BIND_OPTIONS_HMAC_C_S
#define VISION3_SSH_BIND_OPTIONS_HMAC_S_C SSH_BIND_OPTIONS_HMAC_S_C
*/
import "C"
import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"runtime/cgo"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// closeSignal provides a thread-safe one-shot close notification
type closeSignal struct {
	ch   chan struct{}
	once sync.Once
}

func newCloseSignal() *closeSignal {
	return &closeSignal{ch: make(chan struct{})}
}

func (c *closeSignal) Close() {
	c.once.Do(func() { close(c.ch) })
}

func (c *closeSignal) Done() <-chan struct{} {
	return c.ch
}

// connState holds per-connection state for bridging C callbacks to Go
type connState struct {
	server   *Server
	session  C.ssh_session
	channel  C.ssh_channel
	chanCb   unsafe.Pointer // allocated channel callbacks struct, must be freed
	username string
	pty      *PTYRequest

	readCh     chan []byte
	writeCh    chan []byte
	winCh      chan Window
	closer     *closeSignal
	shellReady chan struct{}
	shellOnce  sync.Once // protects shellReady from double-close

	handle cgo.Handle
}

// ErrReadInterrupted is returned by Read when a read interrupt is triggered.
// This is used to cleanly cancel blocked reads (e.g., when a door program exits)
// without consuming any data from the session's read channel.
var ErrReadInterrupted = fmt.Errorf("read interrupted")

// Session represents an SSH session with PTY support
type Session struct {
	User       string
	RemoteAddr net.Addr
	PTY        *PTYRequest
	Session    C.ssh_session
	Channel    C.ssh_channel
	cancel     context.CancelFunc

	readCh        chan []byte
	readBuf       []byte
	readInterrupt <-chan struct{} // when closed, Read() returns ErrReadInterrupted
	riMu          sync.Mutex      // protects readInterrupt
	writeCh       chan []byte
	WinCh         chan Window
	closer        *closeSignal
}

// PTYRequest contains PTY parameters
type PTYRequest struct {
	Term   string
	Width  int
	Height int
}

// Window represents a window size change
type Window struct {
	Width  int
	Height int
}

// SessionHandler is called when a new SSH session is established
type SessionHandler func(*Session) error

// AuthPasswordFunc validates SSH password authentication.
// Returns true if the user/password is accepted.
type AuthPasswordFunc func(username, password string) bool

// Server represents an SSH server using libssh
type Server struct {
	bind             C.ssh_bind
	hostKeyPath      string
	port             int
	sessionHandler   SessionHandler
	authPasswordFunc AuthPasswordFunc
	sessions         sync.Map
	mu               sync.Mutex
	wg               sync.WaitGroup // tracks active connections
}

// Config holds server configuration
type Config struct {
	HostKeyPath         string
	Port                int
	SessionHandler      SessionHandler
	AuthPasswordFunc    AuthPasswordFunc
	LegacySSHAlgorithms bool
}

// NewServer creates a new SSH server instance
func NewServer(config Config) (*Server, error) {
	bind := C.ssh_bind_new()
	if bind == nil {
		return nil, fmt.Errorf("failed to create SSH bind")
	}

	server := &Server{
		bind:             bind,
		hostKeyPath:      config.HostKeyPath,
		port:             config.Port,
		sessionHandler:   config.SessionHandler,
		authPasswordFunc: config.AuthPasswordFunc,
	}

	// Set host key
	cHostKey := C.CString(config.HostKeyPath)
	defer C.free(unsafe.Pointer(cHostKey))

	if C.ssh_bind_options_set(bind, C.SSH_BIND_OPTIONS_RSAKEY, unsafe.Pointer(cHostKey)) != C.SSH_OK {
		C.ssh_bind_free(bind)
		return nil, fmt.Errorf("failed to set host key")
	}

	// Bind to dual-stack (IPv6 + IPv4) address.
	// WSL2 mirrored networking requires dual-stack sockets for external access.
	cAddr := C.CString("::")
	defer C.free(unsafe.Pointer(cAddr))
	if C.ssh_bind_options_set(bind, C.SSH_BIND_OPTIONS_BINDADDR, unsafe.Pointer(cAddr)) != C.SSH_OK {
		C.ssh_bind_free(bind)
		return nil, fmt.Errorf("failed to set bind address")
	}

	// Set port
	cPort := C.int(config.Port)
	if C.ssh_bind_options_set(bind, C.SSH_BIND_OPTIONS_BINDPORT, unsafe.Pointer(&cPort)) != C.SSH_OK {
		C.ssh_bind_free(bind)
		return nil, fmt.Errorf("failed to set port")
	}

	// Enable legacy host key algorithms for older SSH clients (e.g., NetRunner, SyncTerm).
	// Modern clients will still prefer rsa-sha2-512/256, but legacy clients can fall back to ssh-rsa.
	// Security note: ssh-rsa uses SHA-1 which is cryptographically weak, but is necessary
	// for compatibility with retro BBS terminal software.
	cHostKeyAlgos := C.CString("rsa-sha2-512,rsa-sha2-256,ssh-rsa")
	defer C.free(unsafe.Pointer(cHostKeyAlgos))
	if C.ssh_bind_options_set(bind, C.VISION3_SSH_BIND_OPTIONS_HOSTKEY_ALGORITHMS, unsafe.Pointer(cHostKeyAlgos)) != C.SSH_OK {
		C.ssh_bind_free(bind)
		return nil, fmt.Errorf("failed to set host key algorithms")
	}

	// Configure key exchange algorithms, ciphers, and MACs.
	// When legacySSHAlgorithms is enabled, include older algorithms (diffie-hellman-group1-sha1,
	// 3des-cbc, hmac-sha1) needed by retro BBS terminal software.
	// When disabled, only modern secure algorithms are offered.
	var kexAlgos, ciphers, macs string
	if config.LegacySSHAlgorithms {
		log.Printf("INFO: SSH legacy algorithms enabled for retro BBS client compatibility")
		kexAlgos = "curve25519-sha256,curve25519-sha256@libssh.org,ecdh-sha2-nistp256,ecdh-sha2-nistp384,ecdh-sha2-nistp521,diffie-hellman-group18-sha512,diffie-hellman-group16-sha512,diffie-hellman-group-exchange-sha256,diffie-hellman-group14-sha256,diffie-hellman-group14-sha1,diffie-hellman-group1-sha1"
		ciphers = "chacha20-poly1305@openssh.com,aes256-gcm@openssh.com,aes128-gcm@openssh.com,aes256-ctr,aes192-ctr,aes128-ctr,aes256-cbc,aes192-cbc,aes128-cbc,3des-cbc"
		macs = "hmac-sha2-256-etm@openssh.com,hmac-sha2-512-etm@openssh.com,hmac-sha2-256,hmac-sha2-512,hmac-sha1"
	} else {
		kexAlgos = "curve25519-sha256,curve25519-sha256@libssh.org,ecdh-sha2-nistp256,ecdh-sha2-nistp384,ecdh-sha2-nistp521,diffie-hellman-group18-sha512,diffie-hellman-group16-sha512,diffie-hellman-group-exchange-sha256,diffie-hellman-group14-sha256"
		ciphers = "chacha20-poly1305@openssh.com,aes256-gcm@openssh.com,aes128-gcm@openssh.com,aes256-ctr,aes192-ctr,aes128-ctr"
		macs = "hmac-sha2-256-etm@openssh.com,hmac-sha2-512-etm@openssh.com,hmac-sha2-256,hmac-sha2-512"
	}

	cKexAlgos := C.CString(kexAlgos)
	defer C.free(unsafe.Pointer(cKexAlgos))
	if C.ssh_bind_options_set(bind, C.VISION3_SSH_BIND_OPTIONS_KEY_EXCHANGE, unsafe.Pointer(cKexAlgos)) != C.SSH_OK {
		C.ssh_bind_free(bind)
		return nil, fmt.Errorf("failed to set key exchange algorithms")
	}

	cCiphers := C.CString(ciphers)
	defer C.free(unsafe.Pointer(cCiphers))
	if C.ssh_bind_options_set(bind, C.VISION3_SSH_BIND_OPTIONS_CIPHERS_C_S, unsafe.Pointer(cCiphers)) != C.SSH_OK {
		C.ssh_bind_free(bind)
		return nil, fmt.Errorf("failed to set client-to-server ciphers")
	}
	if C.ssh_bind_options_set(bind, C.VISION3_SSH_BIND_OPTIONS_CIPHERS_S_C, unsafe.Pointer(cCiphers)) != C.SSH_OK {
		C.ssh_bind_free(bind)
		return nil, fmt.Errorf("failed to set server-to-client ciphers")
	}

	cMACs := C.CString(macs)
	defer C.free(unsafe.Pointer(cMACs))
	if C.ssh_bind_options_set(bind, C.VISION3_SSH_BIND_OPTIONS_HMAC_C_S, unsafe.Pointer(cMACs)) != C.SSH_OK {
		C.ssh_bind_free(bind)
		return nil, fmt.Errorf("failed to set client-to-server MACs")
	}
	if C.ssh_bind_options_set(bind, C.VISION3_SSH_BIND_OPTIONS_HMAC_S_C, unsafe.Pointer(cMACs)) != C.SSH_OK {
		C.ssh_bind_free(bind)
		return nil, fmt.Errorf("failed to set server-to-client MACs")
	}

	return server, nil
}

// Listen starts the SSH server
func (s *Server) Listen() error {
	// Snapshot open fds before libssh creates its listening socket.
	// libssh's C socket() call does not set CLOEXEC, so we must do it
	// ourselves to prevent child processes from inheriting the fd.
	beforeFds := openFds()

	if C.ssh_bind_listen(s.bind) != C.SSH_OK {
		errMsg := C.GoString(C.ssh_get_error(unsafe.Pointer(s.bind)))
		return fmt.Errorf("failed to listen: %s", errMsg)
	}

	setCloexecNewFds(beforeFds)

	log.Printf("INFO: SSH server listening on port %d", s.port)
	return nil
}

// openFds returns the set of currently open file descriptors.
func openFds() map[int]bool {
	fds := make(map[int]bool)
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return fds
	}
	for _, e := range entries {
		if fd, err := strconv.Atoi(e.Name()); err == nil {
			fds[fd] = true
		}
	}
	return fds
}

// setCloexecNewFds sets close-on-exec on any file descriptors that were not
// present in the before set. This prevents C-created sockets (e.g. libssh
// bind socket) from leaking to child processes spawned by the scheduler.
func setCloexecNewFds(before map[int]bool) {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return
	}
	for _, e := range entries {
		fd, err := strconv.Atoi(e.Name())
		if err != nil || fd <= 2 || before[fd] {
			continue
		}
		syscall.CloseOnExec(fd)
		log.Printf("DEBUG: Set CLOEXEC on fd %d (libssh socket)", fd)
	}
}

// Accept waits for and accepts a new SSH connection
func (s *Server) Accept() error {
	session := C.ssh_new()
	if session == nil {
		return fmt.Errorf("failed to create SSH session")
	}

	// Accept connection
	if C.ssh_bind_accept(s.bind, session) != C.SSH_OK {
		errMsg := C.GoString(C.ssh_get_error(unsafe.Pointer(s.bind)))
		C.ssh_free(session)
		return fmt.Errorf("failed to accept: %s", errMsg)
	}

	// Handle in goroutine
	go s.handleConnection(session)
	return nil
}

// handleConnection processes an SSH connection using callback-based API
func (s *Server) handleConnection(sshSession C.ssh_session) {
	s.wg.Add(1)
	defer s.wg.Done()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer C.ssh_free(sshSession)

	// Create connection state
	cs := &connState{
		server:     s,
		session:    sshSession,
		readCh:     make(chan []byte, 64),
		writeCh:    make(chan []byte, 256),
		winCh:      make(chan Window, 4),
		closer:     newCloseSignal(),
		shellReady: make(chan struct{}),
	}
	cs.handle = cgo.NewHandle(cs)
	defer cs.handle.Delete()

	// Register server callbacks BEFORE key exchange
	// Auth callbacks must be in place before the protocol proceeds
	serverCb := C.vision3_new_server_cb(unsafe.Pointer(uintptr(cs.handle)))
	if serverCb == nil {
		log.Printf("ERROR: Failed to allocate server callbacks")
		return
	}
	defer C.free(unsafe.Pointer(serverCb))

	if C.ssh_set_server_callbacks(sshSession, serverCb) != C.SSH_OK {
		log.Printf("ERROR: Failed to set server callbacks")
		return
	}

	// Advertise supported auth methods
	C.vision3_set_auth_methods(sshSession)

	// Perform key exchange
	if C.ssh_handle_key_exchange(sshSession) != C.SSH_OK {
		errMsg := C.GoString(C.ssh_get_error(unsafe.Pointer(sshSession)))
		log.Printf("ERROR: Key exchange failed: %s", errMsg)
		return
	}

	log.Printf("INFO: SSH handshake successful")

	// Create event context and add session
	event := C.ssh_event_new()
	if event == nil {
		log.Printf("ERROR: Failed to create SSH event")
		return
	}
	defer C.ssh_event_free(event)

	if C.ssh_event_add_session(event, sshSession) != C.SSH_OK {
		log.Printf("ERROR: Failed to add session to event")
		return
	}

	// Phase 1: Poll until shell request is received (with 30s timeout)
	// Auth, channel open, PTY, and shell requests are all handled via callbacks
	phase1Deadline := time.Now().Add(30 * time.Second)
	for {
		select {
		case <-cs.shellReady:
			goto startSession
		default:
		}
		if time.Now().After(phase1Deadline) {
			log.Printf("WARN: Shell request timeout for session, closing")
			return
		}
		rc := C.ssh_event_dopoll(event, 100)
		if rc == C.SSH_ERROR {
			errMsg := C.GoString(C.ssh_get_error(unsafe.Pointer(sshSession)))
			log.Printf("ERROR: Event poll error during setup: %s", errMsg)
			return
		}
	}

startSession:
	log.Printf("INFO: Shell ready for user %s, starting session handler", cs.username)

	// Create session object (context is created by NewBBSSessionAdapter to avoid leaking a second context)
	sess := &Session{
		User:       cs.username,
		RemoteAddr: &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0},
		PTY:        cs.pty,
		Session:    sshSession,
		Channel:    cs.channel,
		readCh:     cs.readCh,
		writeCh:    cs.writeCh,
		WinCh:      cs.winCh,
		closer:     cs.closer,
	}

	// Start session handler in goroutine
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		if s.sessionHandler != nil {
			if err := s.sessionHandler(sess); err != nil {
				log.Printf("ERROR: Session handler failed: %v", err)
			}
		}
	}()

	// Phase 2: Event loop - handle I/O, window resize, etc.
	for {
		// Check termination conditions
		select {
		case <-handlerDone:
			goto cleanup
		case <-cs.closer.Done():
			goto cleanup
		default:
		}

		// Flush pending writes to SSH channel
		s.flushWrites(cs)

		// Poll for events (short timeout for responsive writes)
		rc := C.ssh_event_dopoll(event, 10)
		if rc == C.SSH_ERROR {
			log.Printf("DEBUG: Event poll error, closing session for %s", cs.username)
			goto cleanup
		}
	}

cleanup:
	cs.closer.Close()
	if cs.channel != nil {
		C.ssh_channel_close(cs.channel)
		C.ssh_channel_free(cs.channel)
	}
	if cs.chanCb != nil {
		C.free(cs.chanCb)
	}
	log.Printf("DEBUG: Connection handler finished for user %s", cs.username)
}

// flushWrites drains pending write data and sends to SSH channel.
// Limits the number of writes per call to prevent starving the event loop.
func (s *Server) flushWrites(cs *connState) {
	const maxFlushPerPoll = 64
	for i := 0; i < maxFlushPerPoll; i++ {
		select {
		case data := <-cs.writeCh:
			if cs.channel != nil && len(data) > 0 {
				written := C.ssh_channel_write(cs.channel, unsafe.Pointer(&data[0]), C.uint(len(data)))
				if written < 0 {
					log.Printf("WARN: ssh_channel_write failed for user %s", cs.username)
					cs.closer.Close()
					return
				}
			}
		default:
			return
		}
	}
}

// SetReadInterrupt sets a channel that, when closed, causes any blocked Read()
// to return ErrReadInterrupted without consuming data from the read channel.
// Pass nil to clear the interrupt. This is used to cleanly stop I/O goroutines
// (e.g., when a door program exits) without losing pending user input.
func (s *Session) SetReadInterrupt(ch <-chan struct{}) {
	s.riMu.Lock()
	s.readInterrupt = ch
	s.riMu.Unlock()
}

// Read reads data from the SSH channel
func (s *Session) Read(buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}

	// Serve leftover bytes from previous read
	if len(s.readBuf) > 0 {
		n := copy(buf, s.readBuf)
		s.readBuf = s.readBuf[n:]
		return n, nil
	}

	// Grab the current interrupt channel (nil channel blocks forever in select,
	// so it effectively acts as "no interrupt" when not set).
	s.riMu.Lock()
	interrupt := s.readInterrupt
	s.riMu.Unlock()

	// Wait for new data, session close, or read interrupt
	select {
	case data, ok := <-s.readCh:
		if !ok {
			return 0, io.EOF
		}
		n := copy(buf, data)
		if n < len(data) {
			s.readBuf = data[n:]
		}
		return n, nil
	case <-s.closer.Done():
		return 0, io.EOF
	case <-interrupt:
		return 0, ErrReadInterrupted
	}
}

// Write writes data to the SSH channel via the event loop
func (s *Session) Write(buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}

	// Check if session is already closed
	select {
	case <-s.closer.Done():
		return 0, io.ErrClosedPipe
	default:
	}

	data := make([]byte, len(buf))
	copy(data, buf)

	select {
	case s.writeCh <- data:
		return len(buf), nil
	case <-s.closer.Done():
		return 0, io.ErrClosedPipe
	}
}

// Close closes the SSH session
func (s *Session) Close() error {
	s.closer.Close()
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

// Close closes the SSH server and waits for active connections to finish.
func (s *Server) Close() error {
	if s.bind != nil {
		C.ssh_bind_free(s.bind)
		s.bind = nil
	}
	s.wg.Wait() // Wait for active connections to finish
	return nil
}

// Cleanup performs global libssh cleanup
func Cleanup() {
	C.ssh_finalize()
}
