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
