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

	// Clear screen and show header
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	header := fmt.Sprintf("|12Teleconference Chat|07  |08(type |15/Q|08 to quit)\r\n|08────────────────────────────────────────────────────────────────────────────────\r\n")
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(header)), outputMode)

	// Show recent history
	history := e.ChatRoom.History()
	for _, msg := range history {
		line := formatChatMessage(msg)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)
	}

	// Subscribe to room
	msgCh := e.ChatRoom.Subscribe(nodeNumber, handle)
	defer e.ChatRoom.Unsubscribe(nodeNumber)

	// Announce join
	e.ChatRoom.BroadcastSystem(fmt.Sprintf("|10%s has entered chat|07", handle))
	joinMsg := fmt.Sprintf("|10%s has entered chat|07\r\n", handle)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(joinMsg)), outputMode)

	// Goroutine to receive and display messages from others
	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range msgCh {
			line := formatChatMessage(msg)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)
		}
	}()

	// Main input loop
	for {
		input, err := terminal.ReadLine()
		if err != nil {
			if err == io.EOF {
				e.ChatRoom.BroadcastSystem(fmt.Sprintf("|09%s has left chat|07", handle))
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

		// Display own message locally
		ownMsg := chat.ChatMessage{
			NodeID:    nodeNumber,
			Handle:    handle,
			Text:      trimmed,
			Timestamp: time.Now(),
		}
		line := formatChatMessage(ownMsg)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)
	}

	// Announce departure
	e.ChatRoom.BroadcastSystem(fmt.Sprintf("|09%s has left chat|07", handle))

	// Wait for receiver goroutine to finish (Unsubscribe closes the channel)
	<-done

	return nil, "", nil
}

func formatChatMessage(msg chat.ChatMessage) string {
	if msg.IsSystem {
		return fmt.Sprintf(" |08*** %s\r\n", msg.Text)
	}
	return fmt.Sprintf("|11<%s|11>|07 %s\r\n", msg.Handle, msg.Text)
}
