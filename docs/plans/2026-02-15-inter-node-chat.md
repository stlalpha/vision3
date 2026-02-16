# VIS-26: Inter-Node Chat Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add teleconference chat room and inter-node paging to the BBS.

**Architecture:** A `ChatRoom` struct manages an in-memory message ring buffer and a subscriber map of Go channels. Each chat participant runs a reader goroutine that receives broadcasts and writes to their terminal, while the main goroutine reads user input. Paging uses a `PendingPages` slice on `BbsSession` delivered at menu prompts.

**Tech Stack:** Go, goroutines/channels for fan-out, `sync.RWMutex` for thread safety, existing `SessionRegistry` for node lookup

---

## Task 1: Add PendingPages to BbsSession

**Files:**
- Modify: `internal/session/session.go:19-37`
- Modify: `internal/session/registry_test.go`

**Step 1: Write the failing test**

Add to `internal/session/registry_test.go`:

```go
func TestSessionPendingPages(t *testing.T) {
	s := &BbsSession{NodeID: 1, StartTime: time.Now()}

	// Initially empty
	pages := s.DrainPages()
	if len(pages) != 0 {
		t.Errorf("expected 0 pending pages, got %d", len(pages))
	}

	// Add pages
	s.AddPage("Page from SysOp: Hello!")
	s.AddPage("Page from User1: Hey!")

	// Drain returns all and clears
	pages = s.DrainPages()
	if len(pages) != 2 {
		t.Errorf("expected 2 pending pages, got %d", len(pages))
	}
	if pages[0] != "Page from SysOp: Hello!" {
		t.Errorf("unexpected first page: %q", pages[0])
	}

	// After drain, should be empty
	pages = s.DrainPages()
	if len(pages) != 0 {
		t.Errorf("expected 0 after drain, got %d", len(pages))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/session/ -run TestSessionPendingPages -v`
Expected: FAIL — methods do not exist.

**Step 3: Write minimal implementation**

In `internal/session/session.go`, add the field to `BbsSession` and two methods:

```go
type BbsSession struct {
	// ... existing fields ...
	PendingPages []string // Queued page messages for delivery at next prompt
}

// AddPage queues a page message for delivery at the user's next menu prompt.
func (s *BbsSession) AddPage(msg string) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	s.PendingPages = append(s.PendingPages, msg)
}

// DrainPages returns all pending pages and clears the queue.
func (s *BbsSession) DrainPages() []string {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	if len(s.PendingPages) == 0 {
		return nil
	}
	pages := s.PendingPages
	s.PendingPages = nil
	return pages
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/session/ -run TestSessionPendingPages -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/session/session.go internal/session/registry_test.go
git commit -m "feat: add PendingPages to BbsSession for inter-node paging"
```

---

## Task 2: Create ChatRoom

**Files:**
- Create: `internal/chat/room.go`
- Create: `internal/chat/room_test.go`

**Step 1: Write the failing test**

Create `internal/chat/room_test.go`:

```go
package chat

import (
	"testing"
	"time"
)

func TestChatRoom_BroadcastAndHistory(t *testing.T) {
	room := NewChatRoom(50)

	// Subscribe
	ch := room.Subscribe(1, "SysOp")
	defer room.Unsubscribe(1)

	// Broadcast
	room.Broadcast(2, "User1", "Hello everyone!")

	// Subscriber should receive
	select {
	case msg := <-ch:
		if msg.Handle != "User1" {
			t.Errorf("expected handle 'User1', got %q", msg.Handle)
		}
		if msg.Text != "Hello everyone!" {
			t.Errorf("expected text 'Hello everyone!', got %q", msg.Text)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message")
	}

	// History should contain the message
	history := room.History()
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Text != "Hello everyone!" {
		t.Errorf("unexpected history text: %q", history[0].Text)
	}
}

func TestChatRoom_SubscribeUnsubscribe(t *testing.T) {
	room := NewChatRoom(50)

	ch := room.Subscribe(1, "SysOp")
	if room.ActiveCount() != 1 {
		t.Errorf("expected 1 active, got %d", room.ActiveCount())
	}

	room.Unsubscribe(1)
	if room.ActiveCount() != 0 {
		t.Errorf("expected 0 active after unsubscribe, got %d", room.ActiveCount())
	}

	// Channel should be closed after unsubscribe
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed")
		}
	default:
		// Channel might not be closed yet in buffered scenario, that's OK
	}
}

func TestChatRoom_HistoryRingBuffer(t *testing.T) {
	room := NewChatRoom(3) // Small buffer

	room.Broadcast(1, "A", "msg1")
	room.Broadcast(1, "A", "msg2")
	room.Broadcast(1, "A", "msg3")
	room.Broadcast(1, "A", "msg4") // Should push out msg1

	history := room.History()
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}
	if history[0].Text != "msg2" {
		t.Errorf("expected oldest to be 'msg2', got %q", history[0].Text)
	}
}

func TestChatRoom_SelfExclude(t *testing.T) {
	room := NewChatRoom(50)

	ch := room.Subscribe(1, "SysOp")
	defer room.Unsubscribe(1)

	// Broadcast from same node — should NOT receive own message
	room.Broadcast(1, "SysOp", "talking to myself")

	select {
	case <-ch:
		t.Error("should not receive own broadcast")
	case <-time.After(50 * time.Millisecond):
		// Expected — no message received
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/chat/ -v`
Expected: FAIL — package does not exist.

**Step 3: Write minimal implementation**

Create `internal/chat/room.go`:

```go
package chat

import (
	"sync"
	"time"
)

// ChatMessage represents a single chat message.
type ChatMessage struct {
	NodeID    int
	Handle    string
	Text      string
	Timestamp time.Time
	IsSystem  bool // Join/leave announcements
}

// subscriber tracks a connected chat participant.
type subscriber struct {
	nodeID int
	handle string
	ch     chan ChatMessage
}

// ChatRoom manages a single global teleconference room.
type ChatRoom struct {
	mu          sync.RWMutex
	subscribers map[int]*subscriber
	history     []ChatMessage
	maxHistory  int
}

// NewChatRoom creates a chat room with the given history buffer size.
func NewChatRoom(maxHistory int) *ChatRoom {
	return &ChatRoom{
		subscribers: make(map[int]*subscriber),
		history:     make([]ChatMessage, 0, maxHistory),
		maxHistory:  maxHistory,
	}
}

// Subscribe adds a node to the chat room and returns its message channel.
func (r *ChatRoom) Subscribe(nodeID int, handle string) <-chan ChatMessage {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan ChatMessage, 64)
	r.subscribers[nodeID] = &subscriber{
		nodeID: nodeID,
		handle: handle,
		ch:     ch,
	}
	return ch
}

// Unsubscribe removes a node from the chat room and closes its channel.
func (r *ChatRoom) Unsubscribe(nodeID int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if sub, ok := r.subscribers[nodeID]; ok {
		close(sub.ch)
		delete(r.subscribers, nodeID)
	}
}

// Broadcast sends a message to all subscribers except the sender,
// and appends it to the history ring buffer.
func (r *ChatRoom) Broadcast(senderNodeID int, handle string, text string) {
	msg := ChatMessage{
		NodeID:    senderNodeID,
		Handle:    handle,
		Text:      text,
		Timestamp: time.Now(),
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Append to ring buffer
	if len(r.history) >= r.maxHistory {
		r.history = r.history[1:]
	}
	r.history = append(r.history, msg)

	// Fan out to subscribers (skip sender)
	for _, sub := range r.subscribers {
		if sub.nodeID == senderNodeID {
			continue
		}
		select {
		case sub.ch <- msg:
		default:
			// Drop message if subscriber channel is full
		}
	}
}

// BroadcastSystem sends a system message (join/leave) to all subscribers.
func (r *ChatRoom) BroadcastSystem(text string) {
	msg := ChatMessage{
		Text:      text,
		Timestamp: time.Now(),
		IsSystem:  true,
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.history) >= r.maxHistory {
		r.history = r.history[1:]
	}
	r.history = append(r.history, msg)

	for _, sub := range r.subscribers {
		select {
		case sub.ch <- msg:
		default:
		}
	}
}

// History returns a copy of the message history in chronological order.
func (r *ChatRoom) History() []ChatMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ChatMessage, len(r.history))
	copy(result, r.history)
	return result
}

// ActiveCount returns the number of users currently in the chat room.
func (r *ChatRoom) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.subscribers)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/chat/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/chat/room.go internal/chat/room_test.go
git commit -m "feat: add ChatRoom with broadcast, subscribe, and ring buffer history"
```

---

## Task 3: Wire ChatRoom into MenuExecutor

**Files:**
- Modify: `internal/menu/executor.go:63-112` (add ChatRoom field, update NewExecutor)
- Modify: `cmd/vision3/main.go:1396` (pass ChatRoom to NewExecutor)

**Step 1: Add ChatRoom field to MenuExecutor**

In `internal/menu/executor.go`, add the import:

```go
"github.com/stlalpha/vision3/internal/chat"
```

Add the field to the `MenuExecutor` struct (after `SessionRegistry`):

```go
ChatRoom        *chat.ChatRoom               // Global teleconference chat room
```

Update the `NewExecutor` function signature to accept `chatRoom *chat.ChatRoom` as the last parameter, and assign it in the return struct:

```go
ChatRoom:        chatRoom,
```

**Step 2: Create and pass ChatRoom in main.go**

In `cmd/vision3/main.go`, before the `menu.NewExecutor` call (around line 1396), create the chat room:

```go
chatRoom := chat.NewChatRoom(100)
```

Add the import for `"github.com/stlalpha/vision3/internal/chat"` and pass `chatRoom` as the last argument to `NewExecutor`.

**Step 3: Build and verify**

Run: `go build ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/menu/executor.go cmd/vision3/main.go
git commit -m "feat: wire ChatRoom into MenuExecutor"
```

---

## Task 4: Create teleconference chat handler

**Files:**
- Create: `internal/menu/chat.go`
- Modify: `internal/menu/executor.go:300-340` (register `CHAT`)
- Modify: `menus/v3/cfg/MAIN.CFG` (C and ! keys → `RUN:CHAT`)

**Context:** The handler enters an interactive loop. It subscribes to the ChatRoom, spawns a goroutine to receive and display messages from others, then reads user input line by line. On `/Q` or disconnect, it unsubscribes and returns. The `RunnableFunc` signature is:
```go
func(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error)
```

The terminal is `*term.Terminal` from `golang.org/x/term`. Use `terminal.ReadLine()` for input. Use `terminalio.WriteProcessedBytes(terminal, data, outputMode)` for output. Use `ansi.ReplacePipeCodes([]byte(text))` to process pipe codes.

**Step 1: Create the handler**

Create `internal/menu/chat.go`:

```go
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
```

**Note:** The code above uses `nodeNumber`, which is the parameter name from the `RunnableFunc` signature.

**Step 2: Register CHAT in executor.go**

In `registerAppRunnables`, add:

```go
registry["CHAT"] = runChat
```

**Step 3: Update MAIN.CFG**

Change the C key from `RUN:PLACEHOLDER Chat` to `RUN:CHAT`.
Change the ! key from `RUN:PLACEHOLDER Chat` to `RUN:CHAT`.

**Step 4: Build and verify**

Run: `go build ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/menu/chat.go internal/menu/executor.go menus/v3/cfg/MAIN.CFG
git commit -m "feat: add teleconference chat handler (VIS-26)"
```

---

## Task 5: Create paging handler

**Files:**
- Create: `internal/menu/page.go`
- Modify: `internal/menu/executor.go:300-340` (register `PAGE`)
- Modify: `menus/v3/cfg/MAIN.CFG` (/SE key → `RUN:PAGE`)

**Context:** The handler shows online nodes, prompts for a target node number, prompts for a message, then queues it on the target `BbsSession` via `AddPage()`. The session is looked up via `e.SessionRegistry.Get(targetNodeID)`.

**Step 1: Create the handler**

Create `internal/menu/page.go`:

```go
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

func runPage(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	handle := currentUser.Handle

	// Show online nodes
	sessions := e.SessionRegistry.ListActive()
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|12Online Nodes:|07\r\n")), outputMode)
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
		line := fmt.Sprintf(" |15Node %d|07: %s\r\n", sessNodeID, sessHandle)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)
	}

	// Prompt for target node
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07Page which node? (|15Q|07 to cancel): ")), outputMode)
	nodeInput, err := terminal.ReadLine()
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
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|09Invalid node number.|07\r\n")), outputMode)
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil
	}

	if targetNodeID == nodeNumber {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|09You can't page yourself.|07\r\n")), outputMode)
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil
	}

	targetSession := e.SessionRegistry.Get(targetNodeID)
	if targetSession == nil {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|09That node is not online.|07\r\n")), outputMode)
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil
	}

	// Prompt for message
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|07Message: ")), outputMode)
	msgInput, err := terminal.ReadLine()
	if err != nil {
		if err == io.EOF {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", err
	}
	msgInput = strings.TrimSpace(msgInput)
	if msgInput == "" {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|09Page cancelled.|07\r\n")), outputMode)
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil
	}

	// Queue the page on target session
	pageMsg := fmt.Sprintf("|09Page from |15%s|09: %s|07", handle, msgInput)
	targetSession.AddPage(pageMsg)

	log.Printf("INFO: Node %d (%s) paged Node %d: %s", nodeNumber, handle, targetNodeID, msgInput)
	confirm := fmt.Sprintf("|10Page sent to Node %d.|07\r\n", targetNodeID)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(confirm)), outputMode)
	time.Sleep(500 * time.Millisecond)

	return nil, "", nil
}
```

**Step 2: Register PAGE in executor.go**

In `registerAppRunnables`, add:

```go
registry["PAGE"] = runPage
```

**Step 3: Update MAIN.CFG**

Change the /SE key from `RUN:PLACEHOLDER SendMessage` to `RUN:PAGE`.

**Step 4: Build and verify**

Run: `go build ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/menu/page.go internal/menu/executor.go menus/v3/cfg/MAIN.CFG
git commit -m "feat: add inter-node paging handler (VIS-26)"
```

---

## Task 6: Deliver pending pages at menu prompts

**Files:**
- Modify: `internal/menu/executor.go:1866-1870,1893-1898` (inject page delivery before `displayPrompt` calls)

**Context:** There are two places where `displayPrompt` is called in the menu loop — one for lightbar fallback (line ~1870) and one for standard menus (line ~1898). Before each call, we check the current node's session for pending pages and display them.

The `BbsSession` for the current node is available via `e.SessionRegistry.Get(nodeNumber)`.

**Step 1: Create a helper method**

Add to `internal/menu/executor.go` (near the `displayPrompt` function):

```go
// deliverPendingPages checks for and displays any queued page messages.
func (e *MenuExecutor) deliverPendingPages(terminal *term.Terminal, nodeNumber int, outputMode ansi.OutputMode) {
	sess := e.SessionRegistry.Get(nodeNumber)
	if sess == nil {
		return
	}
	pages := sess.DrainPages()
	for _, page := range pages {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n"+page+"\r\n")), outputMode)
	}
}
```

**Step 2: Inject before displayPrompt calls**

Find the two `e.displayPrompt(...)` calls in the menu loop (approximately lines 1870 and 1898). Add `e.deliverPendingPages(terminal, nodeNumber, outputMode)` immediately before each call.

Before the first `displayPrompt` call (~line 1870):
```go
				e.deliverPendingPages(terminal, nodeNumber, outputMode)
				err = e.displayPrompt(terminal, menuRec, ...
```

Before the second `displayPrompt` call (~line 1898):
```go
			e.deliverPendingPages(terminal, nodeNumber, outputMode)
			err = e.displayPrompt(terminal, menuRec, ...
```

**Step 3: Build and verify**

Run: `go build ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/menu/executor.go
git commit -m "feat: deliver pending pages at menu prompts"
```

---

## Task 7: Final verification

**Step 1: Run all tests**

Run: `go test ./...`
Expected: All tests PASS.

**Step 2: Build**

Run: `go build ./...`
Expected: Clean build.

**Step 3: Verify clean working tree**

Run: `git status`
Expected: Clean working tree, all changes committed.
