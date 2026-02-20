package menu

import (
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	term "golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
)

func runPage(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	handle := currentUser.Handle

	// Show online nodes
	sessions := e.SessionRegistry.ListActive()
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.PageOnlineNodesHeader)), outputMode)
	for _, sess := range sessions {
		sess.Mutex.RLock()
		sessHandle := "Unknown"
		if sess.User != nil {
			sessHandle = sess.User.Handle
		}
		sessNodeID := sess.NodeID
		sess.Mutex.RUnlock()

		if sessNodeID == nodeNumber {
			continue // Skip self
		}
		// Skip invisible sessions for non-CoSysOp viewers
		if sess.Invisible && !e.isCoSysOpOrAbove(currentUser) {
			continue
		}
		line := fmt.Sprintf(e.LoadedStrings.PageNodeListEntry, sessNodeID, sessHandle)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)
	}

	// Prompt for target node
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.PageWhichNodePrompt)), outputMode)
	nodeInput, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		if err == io.EOF {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", err
	}
	nodeInput = strings.TrimSpace(nodeInput)
	if strings.ToUpper(nodeInput) == "Q" || nodeInput == "" {
		return nil, "", nil
	}

	targetNodeID, err := strconv.Atoi(nodeInput)
	if err != nil {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.PageInvalidNode)), outputMode)
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil
	}

	if targetNodeID == nodeNumber {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.PageSelfError)), outputMode)
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil
	}

	targetSession := e.SessionRegistry.Get(targetNodeID)
	if targetSession == nil || (targetSession.Invisible && !e.isCoSysOpOrAbove(currentUser)) {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.PageNodeOffline)), outputMode)
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil
	}

	// Prompt for message
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.PageMessagePrompt)), outputMode)
	msgInput, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		if err == io.EOF {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", err
	}
	msgInput = strings.TrimSpace(msgInput)
	if msgInput == "" {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.PageCancelled)), outputMode)
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil
	}

	// Queue the page on target session
	pageMsg := fmt.Sprintf(e.LoadedStrings.PageMessageFormat, handle, msgInput)
	targetSession.AddPage(pageMsg)

	log.Printf("INFO: Node %d (%s) paged Node %d (%d chars)", nodeNumber, handle, targetNodeID, len(msgInput))
	confirm := fmt.Sprintf(e.LoadedStrings.PageSent, targetNodeID)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(confirm)), outputMode)
	time.Sleep(500 * time.Millisecond)

	return nil, "", nil
}
