package goturbotui

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	
	"golang.org/x/term"
)

// Screen represents the terminal screen interface
type Screen interface {
	// Init initializes the screen
	Init() error
	
	// Close cleans up the screen
	Close() error
	
	// Size returns the current screen dimensions
	Size() (width, height int)
	
	// PollEvents returns a channel for receiving events
	PollEvents() <-chan Event
	
	// Clear clears the screen
	Clear()
	
	// Flush flushes any pending output
	Flush() error
}

// TerminalScreen implements Screen for terminal interfaces
type TerminalScreen struct {
	width      int
	height     int
	events     chan Event
	oldState   *term.State
	done       chan struct{}
	sigwinch   chan os.Signal
}

// NewTerminalScreen creates a new terminal screen
func NewTerminalScreen() *TerminalScreen {
	return &TerminalScreen{
		events:   make(chan Event, 100),
		done:     make(chan struct{}),
		sigwinch: make(chan os.Signal, 1),
	}
}

// Init initializes the terminal screen
func (s *TerminalScreen) Init() error {
	// Get initial size
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// Default size if we can't get terminal size
		width, height = 80, 25
	}
	s.width = width
	s.height = height
	
	// Set up raw mode for input
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}
	s.oldState = oldState
	
	// Set up signal handling for window resize
	signal.Notify(s.sigwinch, syscall.SIGWINCH)
	
	// Clear screen and hide cursor
	fmt.Print("\033[2J\033[H\033[?25l")
	
	// Start input goroutine
	go s.inputLoop()
	go s.signalLoop()
	
	return nil
}

// Close cleans up the terminal screen
func (s *TerminalScreen) Close() error {
	close(s.done)
	
	// Show cursor and reset
	fmt.Print("\033[?25h\033[0m\033[2J\033[H")
	
	// Restore terminal state
	if s.oldState != nil {
		term.Restore(int(os.Stdin.Fd()), s.oldState)
	}
	
	// Stop signal handling
	signal.Stop(s.sigwinch)
	
	return nil
}

// Size returns the current screen dimensions
func (s *TerminalScreen) Size() (width, height int) {
	return s.width, s.height
}

// PollEvents returns a channel for receiving events
func (s *TerminalScreen) PollEvents() <-chan Event {
	return s.events
}

// Clear clears the screen
func (s *TerminalScreen) Clear() {
	fmt.Print("\033[2J\033[H")
}

// Flush flushes any pending output
func (s *TerminalScreen) Flush() error {
	return nil // Terminal output is typically unbuffered
}

// inputLoop processes keyboard input
func (s *TerminalScreen) inputLoop() {
	buf := make([]byte, 256)
	
	for {
		select {
		case <-s.done:
			return
		default:
			n, err := os.Stdin.Read(buf)
			if err != nil {
				continue
			}
			
			s.parseInput(buf[:n])
		}
	}
}

// signalLoop handles terminal signals
func (s *TerminalScreen) signalLoop() {
	for {
		select {
		case <-s.done:
			return
		case <-s.sigwinch:
			// Handle window resize
			width, height, err := term.GetSize(int(os.Stdout.Fd()))
			if err == nil && (width != s.width || height != s.height) {
				s.width = width
				s.height = height
				
				select {
				case s.events <- Event{
					Type: EventResize,
					Resize: ResizeEvent{
						Width:  width,
						Height: height,
					},
				}:
				case <-s.done:
					return
				}
			}
		}
	}
}

// parseInput parses keyboard input and generates events
func (s *TerminalScreen) parseInput(data []byte) {
	if len(data) == 0 {
		return
	}
	
	// Handle escape sequences
	if data[0] == 27 { // ESC
		if len(data) == 1 {
			// Just escape key
			s.sendKeyEvent(KeyEscape, ModNone, 0)
			return
		}
		
		if len(data) >= 3 && data[1] == '[' {
			// ANSI escape sequence
			switch data[2] {
			case 'A':
				s.sendKeyEvent(KeyUp, ModNone, 0)
			case 'B':
				s.sendKeyEvent(KeyDown, ModNone, 0)
			case 'C':
				s.sendKeyEvent(KeyRight, ModNone, 0)
			case 'D':
				s.sendKeyEvent(KeyLeft, ModNone, 0)
			case 'H':
				s.sendKeyEvent(KeyHome, ModNone, 0)
			case 'F':
				s.sendKeyEvent(KeyEnd, ModNone, 0)
			}
			return
		}
		
		// Function keys and other sequences
		if len(data) >= 4 && data[1] == '[' {
			switch {
			case data[2] == '1' && data[3] == '~':
				s.sendKeyEvent(KeyHome, ModNone, 0)
			case data[2] == '4' && data[3] == '~':
				s.sendKeyEvent(KeyEnd, ModNone, 0)
			case data[2] == '5' && data[3] == '~':
				s.sendKeyEvent(KeyPageUp, ModNone, 0)
			case data[2] == '6' && data[3] == '~':
				s.sendKeyEvent(KeyPageDown, ModNone, 0)
			}
			
			// Function keys (F1-F12)
			if len(data) >= 5 && data[2] == '1' {
				switch {
				case data[3] == '1' && data[4] == '~':
					s.sendKeyEvent(KeyF1, ModNone, 0)
				case data[3] == '2' && data[4] == '~':
					s.sendKeyEvent(KeyF2, ModNone, 0)
				case data[3] == '3' && data[4] == '~':
					s.sendKeyEvent(KeyF3, ModNone, 0)
				case data[3] == '4' && data[4] == '~':
					s.sendKeyEvent(KeyF4, ModNone, 0)
				case data[3] == '5' && data[4] == '~':
					s.sendKeyEvent(KeyF5, ModNone, 0)
				}
			}
		}
		return
	}
	
	// Handle regular keys
	switch data[0] {
	case 13: // Enter
		s.sendKeyEvent(KeyEnter, ModNone, '\r')
	case 9: // Tab
		s.sendKeyEvent(KeyTab, ModNone, '\t')
	case 127, 8: // Backspace
		s.sendKeyEvent(KeyBackspace, ModNone, 0)
	case 3: // Ctrl+C
		s.sendKeyEvent(KeyUnknown, ModCtrl, 'c')
	default:
		// Regular character
		if data[0] >= 32 && data[0] < 127 {
			s.sendKeyEvent(KeyUnknown, ModNone, rune(data[0]))
		}
	}
}

// sendKeyEvent sends a keyboard event
func (s *TerminalScreen) sendKeyEvent(key KeyCode, mod KeyMod, char rune) {
	select {
	case s.events <- Event{
		Type: EventKey,
		Key: Key{
			Code:      key,
			Modifiers: mod,
		},
		Rune: char,
	}:
	case <-s.done:
	}
}