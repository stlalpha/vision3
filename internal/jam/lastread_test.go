package jam

import (
	"path/filepath"
	"testing"
)

func TestLastReadSetAndGet(t *testing.T) {
	b := openTestBase(t)

	// Write some messages first
	for i := 0; i < 5; i++ {
		msg := NewMessage()
		msg.From = "User"
		msg.To = "All"
		msg.Subject = "Msg"
		msg.Text = "Body"
		b.WriteMessage(msg)
	}

	// No lastread yet
	_, err := b.GetLastRead("testuser")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// Set lastread
	if err := b.SetLastRead("testuser", 3, 3); err != nil {
		t.Fatalf("SetLastRead: %v", err)
	}

	// Get it back
	lr, err := b.GetLastRead("testuser")
	if err != nil {
		t.Fatalf("GetLastRead: %v", err)
	}
	if lr.LastReadMsg != 3 {
		t.Errorf("LastReadMsg = %d, want 3", lr.LastReadMsg)
	}
	if lr.HighReadMsg != 3 {
		t.Errorf("HighReadMsg = %d, want 3", lr.HighReadMsg)
	}

	// Update
	if err := b.SetLastRead("testuser", 5, 5); err != nil {
		t.Fatalf("SetLastRead update: %v", err)
	}
	lr, _ = b.GetLastRead("testuser")
	if lr.LastReadMsg != 5 {
		t.Errorf("updated LastReadMsg = %d, want 5", lr.LastReadMsg)
	}
}

func TestMarkMessageRead(t *testing.T) {
	b := openTestBase(t)

	for i := 0; i < 5; i++ {
		msg := NewMessage()
		msg.From = "User"
		msg.To = "All"
		msg.Subject = "Msg"
		msg.Text = "Body"
		b.WriteMessage(msg)
	}

	// Mark message 3 as read
	if err := b.MarkMessageRead("reader", 3); err != nil {
		t.Fatalf("MarkMessageRead: %v", err)
	}

	lr, _ := b.GetLastRead("reader")
	if lr.LastReadMsg != 3 {
		t.Errorf("LastReadMsg = %d, want 3", lr.LastReadMsg)
	}

	// Mark message 5 — should advance HighRead
	b.MarkMessageRead("reader", 5)
	lr, _ = b.GetLastRead("reader")
	if lr.HighReadMsg != 5 {
		t.Errorf("HighReadMsg = %d, want 5", lr.HighReadMsg)
	}
}

func TestGetNextUnreadMessage(t *testing.T) {
	b := openTestBase(t)

	for i := 0; i < 5; i++ {
		msg := NewMessage()
		msg.From = "User"
		msg.To = "All"
		msg.Subject = "Msg"
		msg.Text = "Body"
		b.WriteMessage(msg)
	}

	// No lastread — should return 1
	next, err := b.GetNextUnreadMessage("newuser")
	if err != nil {
		t.Fatalf("GetNextUnreadMessage: %v", err)
	}
	if next != 1 {
		t.Errorf("next = %d, want 1 for new user", next)
	}

	// After reading through 3
	b.SetLastRead("newuser", 3, 3)
	next, _ = b.GetNextUnreadMessage("newuser")
	if next != 4 {
		t.Errorf("next = %d, want 4 after reading 3", next)
	}

	// After reading all
	b.SetLastRead("newuser", 5, 5)
	_, err = b.GetNextUnreadMessage("newuser")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound when all read, got %v", err)
	}
}

func TestGetUnreadCount(t *testing.T) {
	b := openTestBase(t)

	for i := 0; i < 10; i++ {
		msg := NewMessage()
		msg.From = "User"
		msg.To = "All"
		msg.Subject = "Msg"
		msg.Text = "Body"
		b.WriteMessage(msg)
	}

	// New user: all unread
	unread, err := b.GetUnreadCount("newuser")
	if err != nil {
		t.Fatalf("GetUnreadCount: %v", err)
	}
	if unread != 10 {
		t.Errorf("unread = %d, want 10 for new user", unread)
	}

	// After reading 7
	b.SetLastRead("newuser", 7, 7)
	unread, _ = b.GetUnreadCount("newuser")
	if unread != 3 {
		t.Errorf("unread = %d, want 3 after reading 7", unread)
	}
}

func TestMultipleUsersLastRead(t *testing.T) {
	b := openTestBase(t)

	for i := 0; i < 5; i++ {
		msg := NewMessage()
		msg.From = "User"
		msg.To = "All"
		msg.Subject = "Msg"
		msg.Text = "Body"
		b.WriteMessage(msg)
	}

	b.SetLastRead("alice", 2, 2)
	b.SetLastRead("bob", 4, 4)

	lrAlice, _ := b.GetLastRead("alice")
	lrBob, _ := b.GetLastRead("bob")

	if lrAlice.LastReadMsg != 2 {
		t.Errorf("Alice lastread = %d, want 2", lrAlice.LastReadMsg)
	}
	if lrBob.LastReadMsg != 4 {
		t.Errorf("Bob lastread = %d, want 4", lrBob.LastReadMsg)
	}
}

func TestLastReadPersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "test")

	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Msg"
	msg.Text = "Body"
	b.WriteMessage(msg)
	b.SetLastRead("persist_user", 1, 1)
	b.Close()

	// Reopen
	b, err = Open(basePath)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer b.Close()

	lr, err := b.GetLastRead("persist_user")
	if err != nil {
		t.Fatalf("GetLastRead after reopen: %v", err)
	}
	if lr.LastReadMsg != 1 {
		t.Errorf("persisted LastReadMsg = %d, want 1", lr.LastReadMsg)
	}
}
