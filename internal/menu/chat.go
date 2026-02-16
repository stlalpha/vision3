package menu

import (
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	term "golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/chat"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
)

func runChat(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	handle := currentUser.Handle

	// Get terminal height from session
	height := 24 // default
	if sess := e.SessionRegistry.Get(nodeNumber); sess != nil {
		sess.Mutex.RLock()
		if sess.Height > 0 {
			height = sess.Height
		}
		sess.Mutex.RUnlock()
	}

	// Layout: line 1 = header, line 2 = separator, lines 3..(height-1) = scroll region, line height = input
	scrollBottom := height - 1

	// Clear screen and show header
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	header := "|12Teleconference Chat|07  |08(type |15/Q|08 to quit)|07"
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(header)), outputMode)

	// Separator on line 2
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(2, 1)), outputMode)
	sep := "|08────────────────────────────────────────────────────────────────────────────────|07"
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(sep)), outputMode)

	// Set scroll region to lines 3..(height-1) for messages
	terminalio.WriteProcessedBytes(terminal, []byte(fmt.Sprintf("\x1B[3;%dr", scrollBottom)), outputMode)

	// writeChatLine writes a message into the scroll region, then restores cursor to input line.
	writeChatLine := func(text string) {
		// Save cursor, move to bottom of scroll region, write message (scrolls within region), restore cursor
		seq := ansi.SaveCursor() + ansi.MoveCursor(scrollBottom, 1) + "\r\n"
		terminalio.WriteProcessedBytes(terminal, []byte(seq), outputMode)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(text)), outputMode)
		terminalio.WriteProcessedBytes(terminal, []byte(ansi.RestoreCursor()), outputMode)
	}

	// Show recent history in scroll region
	history := e.ChatRoom.History()
	for _, msg := range history {
		writeChatLine(formatChatMessage(msg))
	}

	// Subscribe to room
	msgCh := e.ChatRoom.Subscribe(nodeNumber, handle)

	// Announce join
	e.ChatRoom.BroadcastSystem(fmt.Sprintf("|10%s has entered chat|07", handle))
	writeChatLine(fmt.Sprintf("|10%s has entered chat|07", handle))

	// Draw input line separator and position cursor
	inputSep := "|08────────────────────────────────────────────────────────────────────────────────|07"
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(height-1, 1)), outputMode)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(inputSep)), outputMode)
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(height, 1)), outputMode)

	// Set terminal prompt to show handle
	prompt := fmt.Sprintf("\x1B[%d;1H\x1B[2K<%s> ", height, handle)
	terminal.SetPrompt(prompt)

	// Goroutine to receive and display messages from others
	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range msgCh {
			line := formatChatMessage(msg)
			writeChatLine(line)
		}
	}()

	// Main input loop
	for {
		input, err := terminal.ReadLine()
		if err != nil {
			if err == io.EOF {
				e.ChatRoom.BroadcastSystem(fmt.Sprintf("|09%s has left chat|07", handle))
				e.ChatRoom.Unsubscribe(nodeNumber)
				<-done
				resetScrollRegion(terminal, outputMode)
				terminal.SetPrompt("")
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Chat input error: %v", nodeNumber, err)
			break
		}

		trimmed := strings.TrimSpace(input)
		if trimmed == "" {
			continue
		}

		upper := strings.ToUpper(trimmed)
		if upper == "/Q" || upper == "/QUIT" {
			break
		}

		// Broadcast message to others
		e.ChatRoom.Broadcast(nodeNumber, handle, trimmed)

		// Display own message in scroll region
		ownMsg := chat.ChatMessage{
			NodeID:    nodeNumber,
			Handle:    handle,
			Text:      trimmed,
			Timestamp: time.Now(),
		}
		writeChatLine(formatChatMessage(ownMsg))
	}

	// Announce departure and clean up
	e.ChatRoom.BroadcastSystem(fmt.Sprintf("|09%s has left chat|07", handle))
	e.ChatRoom.Unsubscribe(nodeNumber) // closes msgCh, which ends the receiver goroutine
	<-done                              // wait for receiver goroutine to finish

	// Reset scroll region and prompt before leaving
	resetScrollRegion(terminal, outputMode)
	terminal.SetPrompt("")

	return nil, "", nil
}

// resetScrollRegion restores the terminal to full-screen scrolling.
func resetScrollRegion(terminal *term.Terminal, outputMode ansi.OutputMode) {
	terminalio.WriteProcessedBytes(terminal, []byte("\x1B[r"), outputMode)
}

func formatChatMessage(msg chat.ChatMessage) string {
	if msg.IsSystem {
		return fmt.Sprintf(" |08*** %s|07", msg.Text)
	}
	return fmt.Sprintf("|11<%s|11>|07 %s", msg.Handle, msg.Text)
}
