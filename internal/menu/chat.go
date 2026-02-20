package menu

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	term "golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/chat"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
)

func runChat(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	handle := currentUser.Handle

	// Get terminal height: prefer passed parameter, then session registry, then default
	height := 24 // default
	if termHeight > 0 {
		height = termHeight
	} else if sess := e.SessionRegistry.Get(nodeNumber); sess != nil {
		sess.Mutex.RLock()
		if sess.Height > 0 {
			height = sess.Height
		}
		sess.Mutex.RUnlock()
	}

	// Layout: line 1 = header, line 2 = top separator, lines 3..(height-2) = scroll region,
	// line (height-1) = bottom separator, line height = input
	scrollBottom := height - 2

	// Clear screen and show header
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	header := e.LoadedStrings.ChatHeader
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(header)), outputMode)

	// Separator on line 2
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(2, 1)), outputMode)
	sep := e.LoadedStrings.ChatSeparator
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(sep)), outputMode)

	// Set scroll region to lines 3..(height-2) for messages
	terminalio.WriteProcessedBytes(terminal, []byte(fmt.Sprintf("\x1B[3;%dr", scrollBottom)), outputMode)

	// rawWriter writes directly to the SSH session, bypassing term.Terminal's line editor.
	// This is needed because term.Terminal.Write() does escape processing that conflicts
	// with ReadLine() when called from another goroutine.
	var rawMu sync.Mutex
	rawWrite := func(data []byte) {
		rawMu.Lock()
		defer rawMu.Unlock()
		terminalio.WriteProcessedBytes(s, data, outputMode)
	}

	// writeChatLine writes a message into the scroll region via the raw SSH session.
	writeChatLine := func(text string) {
		seq := ansi.SaveCursor() + ansi.MoveCursor(scrollBottom, 1) + "\r\n"
		processed := ansi.ReplacePipeCodes([]byte(text))
		rawMu.Lock()
		defer rawMu.Unlock()
		terminalio.WriteProcessedBytes(s, []byte(seq), outputMode)
		terminalio.WriteProcessedBytes(s, processed, outputMode)
		terminalio.WriteProcessedBytes(s, []byte(ansi.RestoreCursor()), outputMode)
	}

	// Show recent history in scroll region
	history := e.ChatRoom.History()
	for _, msg := range history {
		writeChatLine(formatChatMessage(msg, e.LoadedStrings.ChatSystemPrefix, e.LoadedStrings.ChatMessageFormat))
	}

	// Check if user is invisible (suppress join/leave announcements)
	invisible := false
	if sess := e.SessionRegistry.Get(nodeNumber); sess != nil {
		invisible = sess.Invisible
	}

	// Subscribe to room
	msgCh := e.ChatRoom.Subscribe(nodeNumber, handle)

	// Announce join (suppress for invisible users)
	if !invisible {
		e.ChatRoom.BroadcastSystem(fmt.Sprintf(e.LoadedStrings.ChatUserEntered, handle))
	}
	writeChatLine(fmt.Sprintf(e.LoadedStrings.ChatUserEntered, handle))

	// Draw input line separator and position cursor
	inputSep := e.LoadedStrings.ChatSeparator
	rawWrite([]byte(ansi.MoveCursor(height-1, 1)))
	rawWrite(ansi.ReplacePipeCodes([]byte(inputSep)))
	rawWrite([]byte(ansi.MoveCursor(height, 1)))

	// Set terminal prompt to show handle
	prompt := fmt.Sprintf("\x1B[%d;1H\x1B[2K<%s> ", height, handle)
	terminal.SetPrompt(prompt)

	// Goroutine to receive and display messages from others
	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range msgCh {
			writeChatLine(formatChatMessage(msg, e.LoadedStrings.ChatSystemPrefix, e.LoadedStrings.ChatMessageFormat))
		}
	}()

	// Main input loop
	for {
		input, err := readLineFromSessionIH(s, terminal)
		if err != nil {
			if err == io.EOF {
				if !invisible {
					e.ChatRoom.BroadcastSystem(fmt.Sprintf(e.LoadedStrings.ChatUserLeft, handle))
				}
				e.ChatRoom.Unsubscribe(nodeNumber)
				<-done
				rawWrite([]byte("\x1B[r")) // reset scroll region
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
		writeChatLine(formatChatMessage(ownMsg, e.LoadedStrings.ChatSystemPrefix, e.LoadedStrings.ChatMessageFormat))
	}

	// Announce departure and clean up
	if !invisible {
		e.ChatRoom.BroadcastSystem(fmt.Sprintf(e.LoadedStrings.ChatUserLeft, handle))
	}
	e.ChatRoom.Unsubscribe(nodeNumber) // closes msgCh, which ends the receiver goroutine
	<-done                              // wait for receiver goroutine to finish

	// Reset scroll region and prompt before leaving
	rawWrite([]byte("\x1B[r"))
	terminal.SetPrompt("")

	return nil, "", nil
}

func formatChatMessage(msg chat.ChatMessage, systemFmt, userFmt string) string {
	if msg.IsSystem {
		return fmt.Sprintf(systemFmt, msg.Text)
	}
	return fmt.Sprintf(userFmt, msg.Handle, msg.Text)
}
