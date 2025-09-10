package session

import (
	"net"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"

	// "github.com/stlalpha/vision3/internal/menu" // Removed menu import
	"github.com/stlalpha/vision3/internal/types" // Import the new types package
	"github.com/stlalpha/vision3/internal/user"
	// Remove main import main "github.com/stlalpha/vision3"
)

// Session represents an active user connection to the BBS.
type BbsSession struct {
	ID          int // Unique identifier for the session/node
	Conn        gossh.Conn
	Channel     gossh.Channel // Store the SSH channel for direct I/O
	Term        *term.Terminal
	User        *user.User // Logged-in user, nil if not logged in
	Width       int
	Height      int
	RemoteAddr  net.Addr
	CurrentMenu string               // Tracks the current ViSiON/2 menu the user is in
	NodeID      int                  // Node ID for the session
	AssetsPath  string               // Store required path directly
	Mutex       sync.RWMutex         // For thread-safe access to session state if needed later
	Pty         *ssh.Pty             // Store PTY info
	AutoRunLog  types.AutoRunTracker // Tracks run-once commands executed (Use types.AutoRunTracker)
	LastMenu    string               // Tracks the previously visited menu
	StartTime   time.Time            // Tracks the session start time
	
	// Door/Game System Fields
	ConnectTime    time.Time // Connection time for door/game compatibility
	BaudRate       int       // Connection baud rate (often simulated for SSH)
	ConnectionType string    // Connection type (SSH, Telnet, etc.)
	CallerID       string    // Caller ID information
	TerminalType   string    // Terminal type (ANSI, VT100, etc.)
	ScreenWidth    int       // Screen width (same as Width but for door compatibility)
	ScreenHeight   int       // Screen height (same as Height but for door compatibility)
	ANSISupport    bool      // ANSI support capability
	ColorSupport   bool      // Color support capability
	IBMChars       bool      // IBM character support
}

// NewSession creates a new Session object.
// func NewSession(id int, conn ssh.Conn, term *term.Terminal, width, height int, remoteAddr net.Addr) *Session {
// 	return &Session{
// 		ID:          id,
// 		Conn:        conn,
// 		Term:        term,
// 		Width:       width,
// 		Height:      height,
// 		RemoteAddr:  remoteAddr,
// 		CurrentMenu: "", // Initialize CurrentMenu
// 	}
// }

// TODO: Implement methods for managing session state, e.g.:
// func (s *Session) SetUser(u *user.User) { ... }
// func (s *Session) GetUser() *user.User { ... }
// func (s *Session) SetCurrentMenu(menuName string) { ... }
// func (s *Session) GetCurrentMenu() string { ... }
