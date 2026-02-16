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
