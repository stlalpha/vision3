package jam

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPackEmptyBase(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "empty")

	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer b.Close()

	result, err := b.Pack()
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if result.MessagesBefore != 0 || result.MessagesAfter != 0 || result.DeletedRemoved != 0 {
		t.Errorf("Pack empty: got before=%d after=%d deleted=%d",
			result.MessagesBefore, result.MessagesAfter, result.DeletedRemoved)
	}
}

func TestPackNoDeleted(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "nodeleted")

	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer b.Close()

	// Write 5 messages
	for i := 1; i <= 5; i++ {
		msg := NewMessage()
		msg.From = "Sender"
		msg.To = "All"
		msg.Subject = fmt.Sprintf("Message %d", i)
		msg.Text = fmt.Sprintf("Body of message %d", i)
		if _, err := b.WriteMessage(msg); err != nil {
			t.Fatalf("WriteMessage %d: %v", i, err)
		}
	}

	result, err := b.Pack()
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if result.MessagesBefore != 5 || result.MessagesAfter != 5 || result.DeletedRemoved != 0 {
		t.Errorf("Pack no-deleted: got before=%d after=%d deleted=%d",
			result.MessagesBefore, result.MessagesAfter, result.DeletedRemoved)
	}

	// Verify messages are still readable with correct text
	for i := 1; i <= 5; i++ {
		msg, err := b.ReadMessage(i)
		if err != nil {
			t.Errorf("ReadMessage %d after pack: %v", i, err)
			continue
		}
		expected := fmt.Sprintf("Body of message %d", i)
		if msg.Text != expected {
			t.Errorf("Message %d text: got %q, want %q", i, msg.Text, expected)
		}
		if msg.Subject != fmt.Sprintf("Message %d", i) {
			t.Errorf("Message %d subject: got %q", i, msg.Subject)
		}
	}
}

func TestPackWithDeleted(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "withdeleted")

	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer b.Close()

	// Write 10 messages
	for i := 1; i <= 10; i++ {
		msg := NewMessage()
		msg.From = "Sender"
		msg.To = "All"
		msg.Subject = fmt.Sprintf("Message %d", i)
		msg.Text = fmt.Sprintf("Body of message %d with some extra text to take up space", i)
		if _, err := b.WriteMessage(msg); err != nil {
			t.Fatalf("WriteMessage %d: %v", i, err)
		}
	}

	// Delete messages 3, 5, 7
	for _, n := range []int{3, 5, 7} {
		if err := b.DeleteMessage(n); err != nil {
			t.Fatalf("DeleteMessage %d: %v", n, err)
		}
	}

	// Record sizes before pack
	sizeBefore := int64(0)
	for _, ext := range []string{".jhr", ".jdt", ".jdx"} {
		info, _ := os.Stat(basePath + ext)
		sizeBefore += info.Size()
	}

	result, err := b.Pack()
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if result.MessagesBefore != 10 {
		t.Errorf("MessagesBefore: got %d, want 10", result.MessagesBefore)
	}
	if result.MessagesAfter != 7 {
		t.Errorf("MessagesAfter: got %d, want 7", result.MessagesAfter)
	}
	if result.DeletedRemoved != 3 {
		t.Errorf("DeletedRemoved: got %d, want 3", result.DeletedRemoved)
	}

	// File sizes should have decreased
	sizeAfter := int64(0)
	for _, ext := range []string{".jhr", ".jdt", ".jdx"} {
		info, _ := os.Stat(basePath + ext)
		sizeAfter += info.Size()
	}
	if sizeAfter >= sizeBefore {
		t.Errorf("Files did not shrink: before=%d after=%d", sizeBefore, sizeAfter)
	}

	// Verify remaining messages are readable
	count, _ := b.GetMessageCount()
	if count != 7 {
		t.Fatalf("GetMessageCount after pack: got %d, want 7", count)
	}

	// Original messages 1,2,4,6,8,9,10 should now be at positions 1-7
	expectedOriginals := []int{1, 2, 4, 6, 8, 9, 10}
	for i, origNum := range expectedOriginals {
		msg, err := b.ReadMessage(i + 1)
		if err != nil {
			t.Errorf("ReadMessage %d: %v", i+1, err)
			continue
		}
		expected := fmt.Sprintf("Body of message %d with some extra text to take up space", origNum)
		if msg.Text != expected {
			t.Errorf("Message %d (orig %d) text mismatch", i+1, origNum)
		}
	}
}

func TestPackPreservesLastread(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "lastread")

	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer b.Close()

	// Write a message and set lastread
	msg := NewMessage()
	msg.From = "Test"
	msg.To = "All"
	msg.Subject = "Test"
	msg.Text = "Test body"
	b.WriteMessage(msg)
	b.SetLastRead("testuser", 1, 1)

	// Get .jlr contents before pack
	jlrBefore, _ := os.ReadFile(basePath + ".jlr")

	result, err := b.Pack()
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if result.MessagesAfter != 1 {
		t.Errorf("MessagesAfter: got %d, want 1", result.MessagesAfter)
	}

	// Verify .jlr is unchanged
	jlrAfter, _ := os.ReadFile(basePath + ".jlr")
	if len(jlrBefore) != len(jlrAfter) {
		t.Errorf(".jlr size changed: before=%d after=%d", len(jlrBefore), len(jlrAfter))
	}
	for i := range jlrBefore {
		if jlrBefore[i] != jlrAfter[i] {
			t.Error(".jlr contents changed after pack")
			break
		}
	}

	// Verify lastread still works
	lr, err := b.GetLastRead("testuser")
	if err != nil {
		t.Fatalf("GetLastRead after pack: %v", err)
	}
	if lr.LastReadMsg != 1 || lr.HighReadMsg != 1 {
		t.Errorf("LastRead: got %d/%d, want 1/1", lr.LastReadMsg, lr.HighReadMsg)
	}
}

func TestPackPreservesFixedHeader(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "fixedhdr")

	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer b.Close()

	fhBefore := b.GetFixedHeader()
	origCreated := fhBefore.DateCreated
	origBaseMsgNum := fhBefore.BaseMsgNum

	// Get a serial number to set the counter
	b.GetNextMsgSerial()
	serialBefore := binary.LittleEndian.Uint32(b.GetFixedHeader().Reserved[0:4])

	msg := NewMessage()
	msg.From = "Test"
	msg.To = "All"
	msg.Subject = "Test"
	msg.Text = "Body"
	b.WriteMessage(msg)

	if _, err := b.Pack(); err != nil {
		t.Fatalf("Pack: %v", err)
	}

	fhAfter := b.GetFixedHeader()
	if fhAfter.DateCreated != origCreated {
		t.Errorf("DateCreated changed: %d -> %d", origCreated, fhAfter.DateCreated)
	}
	if fhAfter.BaseMsgNum != origBaseMsgNum {
		t.Errorf("BaseMsgNum changed: %d -> %d", origBaseMsgNum, fhAfter.BaseMsgNum)
	}
	serialAfter := binary.LittleEndian.Uint32(fhAfter.Reserved[0:4])
	if serialAfter != serialBefore {
		t.Errorf("Serial changed: %d -> %d", serialBefore, serialAfter)
	}
}
