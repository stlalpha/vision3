package jam

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openTestBase(t *testing.T) *Base {
	t.Helper()
	dir := t.TempDir()
	b, err := Open(filepath.Join(dir, "test"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func TestWriteAndReadMessage(t *testing.T) {
	b := openTestBase(t)

	msg := NewMessage()
	msg.From = "John Doe"
	msg.To = "All"
	msg.Subject = "Test Subject"
	msg.Text = "Hello, world!"
	msg.DateTime = time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	msgNum, err := b.WriteMessage(msg)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	if msgNum != 1 {
		t.Errorf("msgNum = %d, want 1", msgNum)
	}

	// Verify counts
	count, _ := b.GetMessageCount()
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
	if b.GetActiveMessageCount() != 1 {
		t.Errorf("active = %d, want 1", b.GetActiveMessageCount())
	}

	// Read it back
	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.From != "John Doe" {
		t.Errorf("From = %q, want %q", got.From, "John Doe")
	}
	if got.To != "All" {
		t.Errorf("To = %q, want %q", got.To, "All")
	}
	if got.Subject != "Test Subject" {
		t.Errorf("Subject = %q, want %q", got.Subject, "Test Subject")
	}
	// JAM stores CR, so text will have \r instead of \n
	if got.Text != "Hello, world!" {
		t.Errorf("Text = %q, want %q", got.Text, "Hello, world!")
	}
	if got.DateTime.Unix() != msg.DateTime.Unix() {
		t.Errorf("DateTime mismatch")
	}
}

func TestWriteMultipleMessages(t *testing.T) {
	b := openTestBase(t)

	for i := 0; i < 10; i++ {
		msg := NewMessage()
		msg.From = "User"
		msg.To = "All"
		msg.Subject = "Message"
		msg.Text = "Body"
		if _, err := b.WriteMessage(msg); err != nil {
			t.Fatalf("WriteMessage %d: %v", i, err)
		}
	}

	count, _ := b.GetMessageCount()
	if count != 10 {
		t.Errorf("count = %d, want 10", count)
	}

	// Read message 5
	m, err := b.ReadMessage(5)
	if err != nil {
		t.Fatalf("ReadMessage(5): %v", err)
	}
	if m.From != "User" {
		t.Errorf("msg 5 From = %q, want %q", m.From, "User")
	}
}

func TestDeleteMessage(t *testing.T) {
	b := openTestBase(t)

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Delete me"
	msg.Text = "Temporary"
	b.WriteMessage(msg)

	if err := b.DeleteMessage(1); err != nil {
		t.Fatalf("DeleteMessage: %v", err)
	}

	if b.GetActiveMessageCount() != 0 {
		t.Errorf("active = %d after delete, want 0", b.GetActiveMessageCount())
	}

	// Reading the deleted message should still work but show deleted flag
	m, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage after delete: %v", err)
	}
	if !m.IsDeleted() {
		t.Error("message should be marked deleted")
	}
}

func TestScanMessages(t *testing.T) {
	b := openTestBase(t)

	for i := 0; i < 5; i++ {
		msg := NewMessage()
		msg.From = "User"
		msg.To = "All"
		msg.Subject = "Msg"
		msg.Text = "Body"
		b.WriteMessage(msg)
	}

	// Delete message 3
	b.DeleteMessage(3)

	msgs, err := b.ScanMessages(1, 0)
	if err != nil {
		t.Fatalf("ScanMessages: %v", err)
	}
	if len(msgs) != 4 {
		t.Errorf("scanned %d messages, want 4 (5 minus 1 deleted)", len(msgs))
	}

	// Scan with limit
	msgs, err = b.ScanMessages(1, 2)
	if err != nil {
		t.Fatalf("ScanMessages with limit: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("scanned %d messages with limit 2, want 2", len(msgs))
	}
}

func TestTextLFtoCRConversion(t *testing.T) {
	b := openTestBase(t)

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Test"
	msg.Text = "Line 1\nLine 2\nLine 3"

	b.WriteMessage(msg)

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	// JAM spec uses CR, not LF
	if strings.Contains(got.Text, "\n") {
		t.Error("text should not contain LF after write")
	}
	if !strings.Contains(got.Text, "\r") {
		t.Error("text should contain CR after write")
	}
}

func TestMessageWithSubfields(t *testing.T) {
	b := openTestBase(t)

	msg := NewMessage()
	msg.From = "Sysop"
	msg.To = "User"
	msg.Subject = "Private"
	msg.Text = "Secret message"
	msg.OrigAddr = "1:103/705"
	msg.MsgID = "1:103/705 00000001"
	msg.PID = "Vision3 0.1.0/darwin"

	b.WriteMessage(msg)

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.OrigAddr != "1:103/705" {
		t.Errorf("OrigAddr = %q, want %q", got.OrigAddr, "1:103/705")
	}
	if got.MsgID != "1:103/705 00000001" {
		t.Errorf("MsgID = %q, want %q", got.MsgID, "1:103/705 00000001")
	}
	if got.PID != "Vision3 0.1.0/darwin" {
		t.Errorf("PID = %q, want %q", got.PID, "Vision3 0.1.0/darwin")
	}
}

func TestReadInvalidMessageNumber(t *testing.T) {
	b := openTestBase(t)

	_, err := b.ReadMessage(1)
	if err != ErrInvalidMessage {
		t.Errorf("expected ErrInvalidMessage, got %v", err)
	}

	_, err = b.ReadMessage(0)
	if err != ErrInvalidMessage {
		t.Errorf("expected ErrInvalidMessage for msg 0, got %v", err)
	}
}

func TestCP437ToUnicode(t *testing.T) {
	// Test ASCII passthrough
	ascii := []byte("Hello World")
	got := CP437ToUnicode(ascii)
	if got != "Hello World" {
		t.Errorf("ASCII passthrough failed: %q", got)
	}

	// Test box-drawing character (0xC4 = single horizontal line)
	box := []byte{0xC4, 0xC4, 0xC4}
	got = CP437ToUnicode(box)
	if got != "\u2500\u2500\u2500" {
		t.Errorf("box drawing conversion failed: %q", got)
	}
}

func TestExtractAddressFromOriginLine(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{" * Origin: My BBS (1:103/705)\r", "1:103/705"},
		{"Hello\r * Origin: Test (21:3/110)\r", "21:3/110"},
		{"No origin line here", ""},
		{" * Origin: Missing parens", ""},
	}
	for _, tt := range tests {
		got := extractAddressFromOriginLine(tt.text)
		if got != tt.want {
			t.Errorf("extractAddress(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}
