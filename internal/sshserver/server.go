package sshserver

/*
#cgo pkg-config: libssh
#include <libssh/libssh.h>
#include <libssh/server.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"unsafe"
)

// Session represents an SSH session with PTY support
type Session struct {
	User       string
	RemoteAddr net.Addr
	PTY        *PTYRequest
	Session    C.ssh_session
	Channel    C.ssh_channel
	cancel     context.CancelFunc
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

// Server represents an SSH server using libssh
type Server struct {
	bind           C.ssh_bind
	hostKeyPath    string
	port           int
	sessionHandler SessionHandler
	sessions       sync.Map
	mu             sync.Mutex
}

// Config holds server configuration
type Config struct {
	HostKeyPath    string
	Port           int
	SessionHandler SessionHandler
}

// NewServer creates a new SSH server instance
func NewServer(config Config) (*Server, error) {
	bind := C.ssh_bind_new()
	if bind == nil {
		return nil, fmt.Errorf("failed to create SSH bind")
	}

	server := &Server{
		bind:           bind,
		hostKeyPath:    config.HostKeyPath,
		port:           config.Port,
		sessionHandler: config.SessionHandler,
	}

	// Set host key
	cHostKey := C.CString(config.HostKeyPath)
	defer C.free(unsafe.Pointer(cHostKey))

	if C.ssh_bind_options_set(bind, C.SSH_BIND_OPTIONS_RSAKEY, unsafe.Pointer(cHostKey)) != C.SSH_OK {
		C.ssh_bind_free(bind)
		return nil, fmt.Errorf("failed to set host key")
	}

	// Set port
	cPort := C.int(config.Port)
	if C.ssh_bind_options_set(bind, C.SSH_BIND_OPTIONS_BINDPORT, unsafe.Pointer(&cPort)) != C.SSH_OK {
		C.ssh_bind_free(bind)
		return nil, fmt.Errorf("failed to set port")
	}

	return server, nil
}

// Listen starts the SSH server
func (s *Server) Listen() error {
	if C.ssh_bind_listen(s.bind) != C.SSH_OK {
		errMsg := C.GoString(C.ssh_get_error(unsafe.Pointer(s.bind)))
		return fmt.Errorf("failed to listen: %s", errMsg)
	}

	log.Printf("INFO: SSH server listening on port %d", s.port)
	return nil
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

// handleConnection processes an SSH connection
func (s *Server) handleConnection(session C.ssh_session) {
	defer C.ssh_free(session)

	// Perform key exchange
	if C.ssh_handle_key_exchange(session) != C.SSH_OK {
		errMsg := C.GoString(C.ssh_get_error(unsafe.Pointer(session)))
		log.Printf("ERROR: Key exchange failed: %s", errMsg)
		return
	}

	log.Printf("INFO: SSH handshake successful")

	// Authenticate
	authenticated := false
	var username string

	for !authenticated {
		message := C.ssh_message_get(session)
		if message == nil {
			log.Printf("ERROR: Failed to get SSH message")
			return
		}

		msgType := C.ssh_message_type(message)
		msgSubtype := C.ssh_message_subtype(message)

		if msgType == C.SSH_REQUEST_AUTH {
			user := C.ssh_message_auth_user(message)
			username = C.GoString(user)

			switch msgSubtype {
			case C.SSH_AUTH_METHOD_PASSWORD:
				password := C.ssh_message_auth_password(message)
				_ = C.GoString(password) // Convert but don't validate for now

				// Simple authentication - accept any password for now
				log.Printf("INFO: Password auth for user: %s", username)
				C.ssh_message_auth_reply_success(message, 0)
				authenticated = true

			case C.SSH_AUTH_METHOD_NONE:
				log.Printf("INFO: Auth none for user: %s", username)
				C.ssh_message_auth_reply_success(message, 0)
				authenticated = true

			default:
				C.ssh_message_auth_set_methods(message, C.SSH_AUTH_METHOD_PASSWORD|C.SSH_AUTH_METHOD_NONE)
				C.ssh_message_reply_default(message)
			}
		} else {
			C.ssh_message_reply_default(message)
		}

		C.ssh_message_free(message)
	}

	log.Printf("INFO: User %s authenticated", username)

	// Handle channel requests
	log.Printf("DEBUG: Starting channel handler for user %s", username)
	s.handleChannels(session, username)
	log.Printf("DEBUG: Channel handler finished for user %s", username)
}

// handleChannels processes channel open requests
func (s *Server) handleChannels(session C.ssh_session, username string) {
	log.Printf("DEBUG: handleChannels called for user %s", username)
	for {
		log.Printf("DEBUG: Waiting for channel message from %s...", username)
		message := C.ssh_message_get(session)
		if message == nil {
			log.Printf("DEBUG: ssh_message_get returned nil for %s, exiting channel loop", username)
			break
		}
		log.Printf("DEBUG: Received message from %s", username)

		msgType := C.ssh_message_type(message)
		log.Printf("DEBUG: Message type: %d for user %s", msgType, username)

		if msgType == C.SSH_REQUEST_CHANNEL_OPEN {
			subtype := C.ssh_message_subtype(message)

			if subtype == C.SSH_CHANNEL_SESSION {
				channel := C.ssh_message_channel_request_open_reply_accept(message)
				C.ssh_message_free(message)

				if channel != nil {
					// Call handleChannel directly (NOT in goroutine) to avoid race condition
					// Only one goroutine should call ssh_message_get on a session
					s.handleChannel(session, channel, username)
				}
				return // Exit handleChannels after handling the channel
			}
		}

		C.ssh_message_reply_default(message)
		C.ssh_message_free(message)
	}
}

// handleChannel processes a session channel
func (s *Server) handleChannel(session C.ssh_session, channel C.ssh_channel, username string) {
	defer C.ssh_channel_free(channel)

	log.Printf("DEBUG: handleChannel called for user %s", username)

	// Get remote address (create a dummy one for now - libssh doesn't expose this easily)
	remoteAddr := &net.TCPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 0,
	}

	sess := &Session{
		User:       username,
		RemoteAddr: remoteAddr,
		Session:    session,
		Channel:    channel,
	}

	// Handle channel requests (PTY, shell, exec)
	for {
		message := C.ssh_message_get(session)
		if message == nil {
			break
		}

		msgType := C.ssh_message_type(message)

		if msgType == C.SSH_REQUEST_CHANNEL {
			subtype := C.ssh_message_subtype(message)

			switch subtype {
			case C.SSH_CHANNEL_REQUEST_PTY:
				term := C.ssh_message_channel_request_pty_term(message)
				width := C.ssh_message_channel_request_pty_width(message)
				height := C.ssh_message_channel_request_pty_height(message)

				sess.PTY = &PTYRequest{
					Term:   C.GoString(term),
					Width:  int(width),
					Height: int(height),
				}

				log.Printf("INFO: PTY request: term=%s, size=%dx%d", sess.PTY.Term, sess.PTY.Width, sess.PTY.Height)
				C.ssh_message_channel_request_reply_success(message)

			case C.SSH_CHANNEL_REQUEST_SHELL:
				log.Printf("INFO: Shell request for user: %s", username)
				C.ssh_message_channel_request_reply_success(message)
				C.ssh_message_free(message)

				// Start session handler
				if s.sessionHandler != nil {
					if err := s.sessionHandler(sess); err != nil {
						log.Printf("ERROR: Session handler failed: %v", err)
					}
				}
				return

			case C.SSH_CHANNEL_REQUEST_EXEC:
				log.Printf("INFO: Exec request")
				C.ssh_message_channel_request_reply_success(message)

			default:
				C.ssh_message_reply_default(message)
			}
		} else {
			C.ssh_message_reply_default(message)
		}

		C.ssh_message_free(message)
	}
}

// Read reads data from the SSH channel
func (s *Session) Read(buf []byte) (int, error) {
	if s.Channel == nil {
		return 0, fmt.Errorf("channel is nil")
	}

	// Handle empty buffer
	if len(buf) == 0 {
		return 0, nil
	}

	n := C.ssh_channel_read(s.Channel, unsafe.Pointer(&buf[0]), C.uint(len(buf)), 0)
	if n < 0 {
		return 0, io.EOF
	}
	if n == 0 {
		// Check if it's EOF or just no data available
		if C.ssh_channel_is_eof(s.Channel) != 0 || C.ssh_channel_is_open(s.Channel) == 0 {
			return 0, io.EOF
		}
		// No data available yet, but channel is still open - this is blocking read, so shouldn't happen
		// Return 0 bytes read (valid for io.Reader)
		return 0, nil
	}

	return int(n), nil
}

// Write writes data to the SSH channel
func (s *Session) Write(buf []byte) (int, error) {
	if s.Channel == nil {
		return 0, fmt.Errorf("channel is nil")
	}

	// Check if channel is still open
	if C.ssh_channel_is_open(s.Channel) == 0 {
		return 0, io.ErrClosedPipe
	}

	// Handle empty buffer
	if len(buf) == 0 {
		return 0, nil
	}

	n := C.ssh_channel_write(s.Channel, unsafe.Pointer(&buf[0]), C.uint(len(buf)))
	if n < 0 {
		return 0, io.ErrClosedPipe
	}

	return int(n), nil
}

// Close closes the SSH session
func (s *Session) Close() error {
	if s.Channel != nil {
		C.ssh_channel_close(s.Channel)
		s.Channel = nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

// Close closes the SSH server
func (s *Server) Close() error {
	if s.bind != nil {
		C.ssh_bind_free(s.bind)
		s.bind = nil
	}
	return nil
}

// Cleanup performs global libssh cleanup
func Cleanup() {
	C.ssh_finalize()
}
