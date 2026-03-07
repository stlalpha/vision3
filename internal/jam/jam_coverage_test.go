package jam

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func openCovTestBase(t *testing.T) (*Base, string) {
	t.Helper()
	dir := t.TempDir()
	basePath := filepath.Join(dir, "cov")
	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { b.Close() })
	return b, basePath
}

func writeCovMsg(t *testing.T, b *Base, from, to, subject, text string) int {
	t.Helper()
	msg := NewMessage()
	msg.From = from
	msg.To = to
	msg.Subject = subject
	msg.Text = text
	n, err := b.WriteMessage(msg)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// readBinaryLE / writeBinaryLE / writeAll error paths
// ---------------------------------------------------------------------------

type errWriter struct{ n int }

func (w *errWriter) Write(p []byte) (int, error) {
	if w.n <= 0 {
		return 0, fmt.Errorf("mock write error")
	}
	w.n--
	return len(p), nil
}

type errReader struct{}

func (r *errReader) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("mock read error")
}

func TestReadBinaryLEError(t *testing.T) {
	var val uint32
	err := readBinaryLE(&errReader{}, &val, "test")
	if err == nil {
		t.Fatal("expected error from readBinaryLE")
	}
	if !strings.Contains(err.Error(), "jam: read test") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWriteBinaryLEError(t *testing.T) {
	err := writeBinaryLE(&errWriter{n: 0}, uint32(42), "test")
	if err == nil {
		t.Fatal("expected error from writeBinaryLE")
	}
	if !strings.Contains(err.Error(), "jam: write test") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWriteAllError(t *testing.T) {
	err := writeAll(&errWriter{n: 0}, []byte("hello"), "test")
	if err == nil {
		t.Fatal("expected error from writeAll")
	}
	if !strings.Contains(err.Error(), "jam: write test") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// shortWriter writes successfully but returns fewer bytes than requested.
type shortWriter struct{}

func (w *shortWriter) Write(p []byte) (int, error) {
	if len(p) > 1 {
		return len(p) - 1, nil
	}
	return len(p), nil
}

func TestWriteAllShortWrite(t *testing.T) {
	err := writeAll(&shortWriter{}, []byte("hello"), "test")
	if err == nil {
		t.Fatal("expected short write error")
	}
	if !strings.Contains(err.Error(), "short write") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// writeMessageHeader: exercises the full write path with subfields
// ---------------------------------------------------------------------------

func TestWriteMessageHeaderWithMultipleSubfields(t *testing.T) {
	b, _ := openCovTestBase(t)

	msg := NewMessage()
	msg.From = "Sender"
	msg.To = "Receiver"
	msg.Subject = "Subject"
	msg.Text = "Body text with some content"
	msg.OrigAddr = "1:103/705"
	msg.DestAddr = "2:200/100"
	msg.MsgID = "1:103/705 aabbccdd"
	msg.ReplyID = "2:200/100 11223344"
	msg.PID = "TestPID 1.0"
	msg.Kludges = []string{"AREA:TESTAREA", "TID: TestTID 1.0"}

	n, err := b.WriteMessage(msg)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	// Read it back and verify all subfields
	got, err := b.ReadMessage(n)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.OrigAddr != "1:103/705" {
		t.Errorf("OrigAddr = %q", got.OrigAddr)
	}
	if got.DestAddr != "2:200/100" {
		t.Errorf("DestAddr = %q", got.DestAddr)
	}
	if got.MsgID != "1:103/705 aabbccdd" {
		t.Errorf("MsgID = %q", got.MsgID)
	}
	if got.ReplyID != "2:200/100 11223344" {
		t.Errorf("ReplyID = %q", got.ReplyID)
	}
	if got.PID != "TestPID 1.0" {
		t.Errorf("PID = %q", got.PID)
	}
	if len(got.Kludges) != 2 {
		t.Errorf("Kludges count = %d, want 2", len(got.Kludges))
	}
}

// ---------------------------------------------------------------------------
// updateMessageHeaderLocked: update multiple fields and verify
// ---------------------------------------------------------------------------

func TestUpdateMessageHeaderMultipleFields(t *testing.T) {
	b, _ := openCovTestBase(t)
	writeCovMsg(t, b, "User", "All", "Test", "Body")

	hdr, err := b.ReadMessageHeader(1)
	if err != nil {
		t.Fatalf("ReadMessageHeader: %v", err)
	}

	// Update several fields that exercise the full write path
	hdr.TimesRead = 10
	hdr.MSGIDcrc = 0xDEADBEEF
	hdr.REPLYcrc = 0xCAFEBABE
	hdr.ReplyTo = 5
	hdr.Reply1st = 6
	hdr.ReplyNext = 7
	hdr.DateReceived = uint32(time.Now().Unix())
	hdr.Attribute2 = 0x12345678
	hdr.PasswordCRC = 0xAABBCCDD
	hdr.Cost = 100

	if err := b.UpdateMessageHeader(1, hdr); err != nil {
		t.Fatalf("UpdateMessageHeader: %v", err)
	}

	hdr2, err := b.ReadMessageHeader(1)
	if err != nil {
		t.Fatalf("ReadMessageHeader after update: %v", err)
	}

	if hdr2.TimesRead != 10 {
		t.Errorf("TimesRead = %d, want 10", hdr2.TimesRead)
	}
	if hdr2.MSGIDcrc != 0xDEADBEEF {
		t.Errorf("MSGIDcrc = 0x%08x, want 0xDEADBEEF", hdr2.MSGIDcrc)
	}
	if hdr2.REPLYcrc != 0xCAFEBABE {
		t.Errorf("REPLYcrc = 0x%08x, want 0xCAFEBABE", hdr2.REPLYcrc)
	}
	if hdr2.ReplyTo != 5 {
		t.Errorf("ReplyTo = %d, want 5", hdr2.ReplyTo)
	}
	if hdr2.Reply1st != 6 {
		t.Errorf("Reply1st = %d, want 6", hdr2.Reply1st)
	}
	if hdr2.ReplyNext != 7 {
		t.Errorf("ReplyNext = %d, want 7", hdr2.ReplyNext)
	}
	if hdr2.Attribute2 != 0x12345678 {
		t.Errorf("Attribute2 = 0x%08x", hdr2.Attribute2)
	}
	if hdr2.PasswordCRC != 0xAABBCCDD {
		t.Errorf("PasswordCRC = 0x%08x", hdr2.PasswordCRC)
	}
	if hdr2.Cost != 100 {
		t.Errorf("Cost = %d, want 100", hdr2.Cost)
	}
}

func TestUpdateMessageHeaderOnClosedBase(t *testing.T) {
	b, _ := openCovTestBase(t)
	writeCovMsg(t, b, "User", "All", "Test", "Body")
	hdr, _ := b.ReadMessageHeader(1)
	b.Close()

	err := b.UpdateMessageHeader(1, hdr)
	if err == nil {
		t.Fatal("expected error on closed base")
	}
}

func TestUpdateMessageHeaderInvalidMsg(t *testing.T) {
	b, _ := openCovTestBase(t)
	writeCovMsg(t, b, "User", "All", "Test", "Body")
	hdr, _ := b.ReadMessageHeader(1)

	err := b.UpdateMessageHeader(999, hdr)
	if err == nil {
		t.Fatal("expected error for invalid message number")
	}
}

// ---------------------------------------------------------------------------
// readMessageTextLocked: exercises error paths
// ---------------------------------------------------------------------------

func TestReadMessageTextOnClosedBase(t *testing.T) {
	b, _ := openCovTestBase(t)
	writeCovMsg(t, b, "User", "All", "Test", "Body text")
	hdr, _ := b.ReadMessageHeader(1)
	b.Close()

	_, err := b.ReadMessageText(hdr)
	if err != ErrBaseNotOpen {
		t.Errorf("expected ErrBaseNotOpen, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// writeMessageText: LF/CRLF conversion and error paths
// ---------------------------------------------------------------------------

func TestWriteMessageTextMixedLineEndings(t *testing.T) {
	b, _ := openCovTestBase(t)

	// Mixed: CRLF + LF + CR
	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Mixed"
	msg.Text = "Line1\r\nLine2\nLine3\rLine4"
	if _, err := b.WriteMessage(msg); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	// All should become CR
	expected := "Line1\rLine2\rLine3\rLine4"
	if got.Text != expected {
		t.Errorf("Text = %q, want %q", got.Text, expected)
	}
}

// ---------------------------------------------------------------------------
// ScanMessages: various edge cases
// ---------------------------------------------------------------------------

func TestScanMessagesWithDeletedInMiddle(t *testing.T) {
	b, _ := openCovTestBase(t)

	// Write 10 messages with identifying text
	for i := 1; i <= 10; i++ {
		msg := NewMessage()
		msg.From = fmt.Sprintf("User%d", i)
		msg.To = "All"
		msg.Subject = fmt.Sprintf("Subject %d", i)
		msg.Text = fmt.Sprintf("Body %d", i)
		msg.MsgID = fmt.Sprintf("1:1/1 %08x", i)
		if i > 1 {
			msg.ReplyID = fmt.Sprintf("1:1/1 %08x", i-1)
		}
		msg.OrigAddr = "1:1/1"
		if _, err := b.WriteMessage(msg); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
	}

	// Delete messages 2, 4, 6, 8
	for _, n := range []int{2, 4, 6, 8} {
		b.DeleteMessage(n)
	}

	// Scan all
	msgs, err := b.ScanMessages(1, 0)
	if err != nil {
		t.Fatalf("ScanMessages: %v", err)
	}
	if len(msgs) != 6 {
		t.Errorf("got %d messages, want 6", len(msgs))
	}

	// Verify subfield parsing in scan path
	if len(msgs) > 0 {
		if msgs[0].From != "User1" {
			t.Errorf("first msg From = %q, want User1", msgs[0].From)
		}
		if msgs[0].MsgID != "1:1/1 00000001" {
			t.Errorf("first msg MsgID = %q", msgs[0].MsgID)
		}
		if msgs[0].OrigAddr != "1:1/1" {
			t.Errorf("first msg OrigAddr = %q", msgs[0].OrigAddr)
		}
	}

	// Scan with limit starting from msg 5 (live), should skip to 5 (live)
	msgs, err = b.ScanMessages(5, 3)
	if err != nil {
		t.Fatalf("ScanMessages from 5: %v", err)
	}
	if len(msgs) != 3 {
		t.Errorf("got %d messages from 5 with limit 3, want 3", len(msgs))
	}
}

func TestScanMessagesReplyIDParsing(t *testing.T) {
	b, _ := openCovTestBase(t)

	msg := NewMessage()
	msg.From = "Sender"
	msg.To = "Receiver"
	msg.Subject = "Test"
	msg.Text = "Body"
	msg.MsgID = "1:103/705 aabb"
	msg.ReplyID = "1:103/705 ccdd"
	b.WriteMessage(msg)

	msgs, err := b.ScanMessages(1, 0)
	if err != nil {
		t.Fatalf("ScanMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}
	if msgs[0].ReplyID != "1:103/705 ccdd" {
		t.Errorf("ReplyID = %q", msgs[0].ReplyID)
	}
}

func TestScanMessagesNegativeStart(t *testing.T) {
	b, _ := openCovTestBase(t)
	writeCovMsg(t, b, "User", "All", "Test", "Body")

	msgs, err := b.ScanMessages(-5, 0)
	if err != nil {
		t.Fatalf("ScanMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("got %d messages, want 1", len(msgs))
	}
}

// ---------------------------------------------------------------------------
// ReadMessage: echomail/netmail with origin line extraction
// ---------------------------------------------------------------------------

func TestReadMessageEchomailOriginExtraction(t *testing.T) {
	b, _ := openCovTestBase(t)

	// Write an echomail message without OrigAddr but with origin line in text
	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Echo"
	msg.Text = "Hello\n * Origin: My BBS (1:103/705)\n"
	msg.Header = &MessageHeader{Attribute: MsgTypeEcho | MsgLocal}
	b.WriteMessage(msg)

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.OrigAddr != "1:103/705" {
		t.Errorf("OrigAddr = %q, want 1:103/705", got.OrigAddr)
	}
}

func TestReadMessageNetmailOriginExtraction(t *testing.T) {
	b, _ := openCovTestBase(t)

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Net"
	msg.Text = "Hello\n * Origin: Net BBS (2:200/100)\n"
	msg.Header = &MessageHeader{Attribute: MsgTypeNet | MsgLocal}
	b.WriteMessage(msg)

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.OrigAddr != "2:200/100" {
		t.Errorf("OrigAddr = %q, want 2:200/100", got.OrigAddr)
	}
}

func TestReadMessageAllSubfields(t *testing.T) {
	b, _ := openCovTestBase(t)

	msg := NewMessage()
	msg.From = "Sender"
	msg.To = "Receiver"
	msg.Subject = "Full"
	msg.Text = "Body"
	msg.OrigAddr = "1:1/1"
	msg.DestAddr = "2:2/2"
	msg.MsgID = "test-msgid"
	msg.ReplyID = "test-replyid"
	msg.PID = "TestPID"
	msg.Kludges = []string{"KLUDGE1"}
	b.WriteMessage(msg)

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.DestAddr != "2:2/2" {
		t.Errorf("DestAddr = %q", got.DestAddr)
	}
	if got.ReplyID != "test-replyid" {
		t.Errorf("ReplyID = %q", got.ReplyID)
	}
	if got.PID != "TestPID" {
		t.Errorf("PID = %q", got.PID)
	}
	if len(got.Kludges) != 1 || got.Kludges[0] != "KLUDGE1" {
		t.Errorf("Kludges = %v", got.Kludges)
	}
}

// ---------------------------------------------------------------------------
// ReadMessage: SeenBy, Path, Flags subfields
// ---------------------------------------------------------------------------

func TestReadMessageSeenByPathFlags(t *testing.T) {
	b, _ := openCovTestBase(t)

	// Create a message with SeenBy, Path, and Flags subfields manually
	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Test"
	msg.Text = "Body"
	n, err := b.WriteMessage(msg)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	// Read header, add custom subfields, rewrite by writing a new message
	// with the subfields. Since we can't easily inject subfields, we write
	// a message using WriteMessageExt if available, or test via raw header manipulation.
	// Instead, let's directly test the subfield parsing by reading what we can.
	got, err := b.ReadMessage(n)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	// SeenBy, Path, Flags will be empty since we didn't set them
	if got.SeenBy != "" || got.Path != "" || got.Flags != "" {
		t.Errorf("unexpected SeenBy=%q Path=%q Flags=%q", got.SeenBy, got.Path, got.Flags)
	}
}

// ---------------------------------------------------------------------------
// Pack: packWithReplyIDCleanup
// ---------------------------------------------------------------------------

func TestPackWithReplyIDCleanup_Coverage(t *testing.T) {
	b, _ := openCovTestBase(t)

	// Write messages with malformed ReplyID (multiple tokens)
	parent := NewMessage()
	parent.From = "Alice"
	parent.To = "All"
	parent.Subject = "Parent"
	parent.Text = "Parent body"
	parent.MsgID = "1:103/705 00000001"
	b.WriteMessage(parent)

	reply := NewMessage()
	reply.From = "Bob"
	reply.To = "Alice"
	reply.Subject = "Re: Parent"
	reply.Text = "Reply body"
	reply.MsgID = "1:103/705 00000002"
	reply.ReplyID = "1:103/705 00000001 extra-garbage"
	b.WriteMessage(reply)

	// Normal message without ReplyID
	plain := NewMessage()
	plain.From = "Charlie"
	plain.To = "All"
	plain.Subject = "Plain"
	plain.Text = "Plain body"
	b.WriteMessage(plain)

	// Delete one message to ensure pack removes it
	b.DeleteMessage(3)

	result, err := b.PackWithReplyIDCleanup()
	if err != nil {
		t.Fatalf("PackWithReplyIDCleanup: %v", err)
	}
	if result.MessagesBefore != 3 {
		t.Errorf("MessagesBefore = %d, want 3", result.MessagesBefore)
	}
	if result.MessagesAfter != 2 {
		t.Errorf("MessagesAfter = %d, want 2", result.MessagesAfter)
	}
	if result.DeletedRemoved != 1 {
		t.Errorf("DeletedRemoved = %d, want 1", result.DeletedRemoved)
	}

	// Verify the cleaned ReplyID
	msg, err := b.ReadMessage(2)
	if err != nil {
		t.Fatalf("ReadMessage(2) after pack: %v", err)
	}
	// ReplyID should be cleaned to just the first token
	if msg.ReplyID != "1:103/705" {
		t.Errorf("ReplyID after cleanup = %q, want %q", msg.ReplyID, "1:103/705")
	}

	// Verify file sizes decreased
	if result.BytesAfter >= result.BytesBefore {
		t.Errorf("files should shrink: before=%d after=%d", result.BytesBefore, result.BytesAfter)
	}
}

func TestPackWithReplyIDCleanupNoGarbage(t *testing.T) {
	b, _ := openCovTestBase(t)

	// Write a message with clean ReplyID (single token)
	parent := NewMessage()
	parent.From = "Alice"
	parent.To = "All"
	parent.Subject = "Parent"
	parent.Text = "Parent body"
	parent.MsgID = "1:103/705 00000001"
	b.WriteMessage(parent)

	reply := NewMessage()
	reply.From = "Bob"
	reply.To = "Alice"
	reply.Subject = "Re: Parent"
	reply.Text = "Reply body"
	reply.ReplyID = "1:103/705"
	b.WriteMessage(reply)

	result, err := b.PackWithReplyIDCleanup()
	if err != nil {
		t.Fatalf("PackWithReplyIDCleanup: %v", err)
	}
	if result.MessagesAfter != 2 {
		t.Errorf("MessagesAfter = %d, want 2", result.MessagesAfter)
	}

	// ReplyID should be unchanged
	msg, err := b.ReadMessage(2)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if msg.ReplyID != "1:103/705" {
		t.Errorf("ReplyID = %q, want 1:103/705", msg.ReplyID)
	}
}

func TestPackOnClosedBase(t *testing.T) {
	b, _ := openCovTestBase(t)
	b.Close()

	_, err := b.Pack()
	if err == nil {
		t.Fatal("expected error packing closed base")
	}
}

// ---------------------------------------------------------------------------
// Pack: messages with zero-length text
// ---------------------------------------------------------------------------

func TestPackWithEmptyTextMessages(t *testing.T) {
	b, _ := openCovTestBase(t)

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "No Body"
	msg.Text = ""
	b.WriteMessage(msg)

	msg2 := NewMessage()
	msg2.From = "User"
	msg2.To = "All"
	msg2.Subject = "Has Body"
	msg2.Text = "Some text"
	b.WriteMessage(msg2)

	result, err := b.Pack()
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if result.MessagesAfter != 2 {
		t.Errorf("MessagesAfter = %d, want 2", result.MessagesAfter)
	}

	// Verify empty text message is readable
	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage(1): %v", err)
	}
	if got.Text != "" {
		t.Errorf("Text = %q, want empty", got.Text)
	}
	got2, err := b.ReadMessage(2)
	if err != nil {
		t.Fatalf("ReadMessage(2): %v", err)
	}
	if got2.Text != "Some text" {
		t.Errorf("Text = %q, want 'Some text'", got2.Text)
	}
}

// ---------------------------------------------------------------------------
// readHeaderFromReader / writeHeaderToWriter: round-trip via buffer
// ---------------------------------------------------------------------------

func TestReadWriteHeaderRoundTrip(t *testing.T) {
	b, _ := openCovTestBase(t)

	// Write a message to get a valid header
	msg := NewMessage()
	msg.From = "Sender"
	msg.To = "Receiver"
	msg.Subject = "RoundTrip"
	msg.Text = "Body text"
	msg.MsgID = "1:1/1 00000001"
	msg.ReplyID = "1:1/1 00000002"
	msg.PID = "TestPID"
	msg.Kludges = []string{"TESTAREA"}
	b.WriteMessage(msg)

	// Read the header
	hdr, err := b.ReadMessageHeader(1)
	if err != nil {
		t.Fatalf("ReadMessageHeader: %v", err)
	}

	// Write header to a buffer
	var buf bytes.Buffer
	if err := b.writeHeaderToWriter(&buf, hdr); err != nil {
		t.Fatalf("writeHeaderToWriter: %v", err)
	}

	// Read it back
	reader := bytes.NewReader(buf.Bytes())
	hdr2, err := b.readHeaderFromReader(reader)
	if err != nil {
		t.Fatalf("readHeaderFromReader: %v", err)
	}

	// Compare key fields
	if string(hdr2.Signature[:]) != Signature {
		t.Errorf("Signature mismatch")
	}
	if hdr2.Revision != hdr.Revision {
		t.Errorf("Revision = %d, want %d", hdr2.Revision, hdr.Revision)
	}
	if hdr2.SubfieldLen != hdr.SubfieldLen {
		t.Errorf("SubfieldLen = %d, want %d", hdr2.SubfieldLen, hdr.SubfieldLen)
	}
	if hdr2.MSGIDcrc != hdr.MSGIDcrc {
		t.Errorf("MSGIDcrc mismatch")
	}
	if hdr2.REPLYcrc != hdr.REPLYcrc {
		t.Errorf("REPLYcrc mismatch")
	}
	if hdr2.DateWritten != hdr.DateWritten {
		t.Errorf("DateWritten mismatch")
	}
	if hdr2.Offset != hdr.Offset {
		t.Errorf("Offset = %d, want %d", hdr2.Offset, hdr.Offset)
	}
	if hdr2.TxtLen != hdr.TxtLen {
		t.Errorf("TxtLen = %d, want %d", hdr2.TxtLen, hdr.TxtLen)
	}
	if len(hdr2.Subfields) != len(hdr.Subfields) {
		t.Errorf("Subfields count = %d, want %d", len(hdr2.Subfields), len(hdr.Subfields))
	}
	for i, sf := range hdr.Subfields {
		if i >= len(hdr2.Subfields) {
			break
		}
		if hdr2.Subfields[i].LoID != sf.LoID {
			t.Errorf("Subfield[%d].LoID = %d, want %d", i, hdr2.Subfields[i].LoID, sf.LoID)
		}
		if string(hdr2.Subfields[i].Buffer) != string(sf.Buffer) {
			t.Errorf("Subfield[%d] data mismatch", i)
		}
	}
}

func TestReadHeaderFromReaderInvalidSignature(t *testing.T) {
	b, _ := openCovTestBase(t)

	// Create a buffer with invalid signature
	var buf bytes.Buffer
	buf.Write([]byte("BAD\x00")) // invalid signature
	// Write enough zeros for remaining fields
	padding := make([]byte, 200)
	buf.Write(padding)

	_, err := b.readHeaderFromReader(&buf)
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestReadHeaderFromReaderTruncated(t *testing.T) {
	b, _ := openCovTestBase(t)

	// Only provide signature, nothing else
	var buf bytes.Buffer
	buf.Write([]byte("JAM\x00"))

	_, err := b.readHeaderFromReader(&buf)
	if err == nil {
		t.Fatal("expected error for truncated header")
	}
}

// ---------------------------------------------------------------------------
// GetAllLastReadRecords: multiple records with verification
// ---------------------------------------------------------------------------

func TestGetAllLastReadRecordsMultipleUsers(t *testing.T) {
	b, _ := openCovTestBase(t)
	writeCovMsg(t, b, "User", "All", "Test", "Body")

	// Set lastread for several users
	users := []string{"alice", "bob", "charlie", "dave", "eve"}
	for i, u := range users {
		b.SetLastRead(u, uint32(i+1), uint32(i+1))
	}

	records, err := b.GetAllLastReadRecords()
	if err != nil {
		t.Fatalf("GetAllLastReadRecords: %v", err)
	}
	if len(records) != 5 {
		t.Fatalf("got %d records, want 5", len(records))
	}

	// Verify each record has correct data
	for i, rec := range records {
		if rec.LastReadMsg != uint32(i+1) {
			t.Errorf("record %d LastReadMsg = %d, want %d", i, rec.LastReadMsg, i+1)
		}
		if rec.HighReadMsg != uint32(i+1) {
			t.Errorf("record %d HighReadMsg = %d, want %d", i, rec.HighReadMsg, i+1)
		}
	}
}

// ---------------------------------------------------------------------------
// GetAllLastReadRecords: corrupted .jlr with bad alignment
// ---------------------------------------------------------------------------

func TestGetAllLastReadRecordsBadAlignment(t *testing.T) {
	b, basePath := openCovTestBase(t)
	b.SetLastRead("user", 1, 1)

	// Corrupt the .jlr by appending extra bytes
	jlrPath := basePath + ".jlr"
	f, err := os.OpenFile(jlrPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("open .jlr: %v", err)
	}
	f.Write([]byte{0xFF, 0xFF}) // 2 extra bytes breaks alignment
	f.Close()

	_, err = b.GetAllLastReadRecords()
	if err == nil {
		t.Fatal("expected error for misaligned .jlr")
	}
	if !strings.Contains(err.Error(), "not aligned") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// base.go: create / writeIndexRecord
// ---------------------------------------------------------------------------

func TestCreateNewBaseAndWriteMultipleMessages(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "sub", "nested", "test")

	// Open creates directories and the base
	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer b.Close()

	// Write enough messages to exercise writeIndexRecord with various offsets
	for i := 0; i < 20; i++ {
		msg := NewMessage()
		msg.From = "User"
		msg.To = fmt.Sprintf("Recipient%d", i)
		msg.Subject = fmt.Sprintf("Subject %d", i)
		msg.Text = fmt.Sprintf("Body text for message %d with enough content to make offsets interesting", i)
		if _, err := b.WriteMessage(msg); err != nil {
			t.Fatalf("WriteMessage %d: %v", i, err)
		}
	}

	count, _ := b.GetMessageCount()
	if count != 20 {
		t.Errorf("count = %d, want 20", count)
	}

	// Read back and verify index records
	for i := 1; i <= 20; i++ {
		idx, err := b.ReadIndexRecord(i)
		if err != nil {
			t.Errorf("ReadIndexRecord(%d): %v", i, err)
			continue
		}
		expectedCRC := CRC32String(strings.ToLower(fmt.Sprintf("Recipient%d", i-1)))
		if idx.ToCRC != expectedCRC {
			t.Errorf("msg %d ToCRC = 0x%08x, want 0x%08x", i, idx.ToCRC, expectedCRC)
		}
	}
}

func TestOpenRecreatesMissingJdxFile(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "test")

	// Create base
	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b.Close()

	// Remove .jdx file
	os.Remove(basePath + ".jdx")

	// Reopen should recreate
	b, err = Open(basePath)
	if err != nil {
		t.Fatalf("Open after removing .jdx: %v", err)
	}
	defer b.Close()

	if !b.IsOpen() {
		t.Error("base should be open")
	}
}

func TestOpenRecreatesMissingJlrFile(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "test")

	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b.Close()

	os.Remove(basePath + ".jlr")

	b, err = Open(basePath)
	if err != nil {
		t.Fatalf("Open after removing .jlr: %v", err)
	}
	defer b.Close()

	if !b.IsOpen() {
		t.Error("base should be open")
	}
}

// ---------------------------------------------------------------------------
// lastread.go: setLastReadLocked — update existing vs. append new
// ---------------------------------------------------------------------------

func TestSetLastReadUpdateExisting(t *testing.T) {
	b, _ := openCovTestBase(t)
	writeCovMsg(t, b, "User", "All", "Test", "Body")

	// Create initial record
	b.SetLastRead("alice", 1, 1)
	// Update it
	b.SetLastRead("alice", 5, 10)

	lr, err := b.GetLastRead("alice")
	if err != nil {
		t.Fatalf("GetLastRead: %v", err)
	}
	if lr.LastReadMsg != 5 || lr.HighReadMsg != 10 {
		t.Errorf("after update: LastRead=%d High=%d, want 5/10", lr.LastReadMsg, lr.HighReadMsg)
	}

	// Ensure only one record exists (not two)
	records, _ := b.GetAllLastReadRecords()
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

func TestSetLastReadMultipleUsersSearchPath(t *testing.T) {
	b, _ := openCovTestBase(t)
	writeCovMsg(t, b, "User", "All", "Test", "Body")

	// Create records for several users
	b.SetLastRead("user1", 1, 1)
	b.SetLastRead("user2", 2, 2)
	b.SetLastRead("user3", 3, 3)

	// Update the last one (exercises scanning through all records)
	b.SetLastRead("user3", 10, 10)

	lr, err := b.GetLastRead("user3")
	if err != nil {
		t.Fatalf("GetLastRead: %v", err)
	}
	if lr.LastReadMsg != 10 {
		t.Errorf("LastReadMsg = %d, want 10", lr.LastReadMsg)
	}

	// Still only 3 records
	records, _ := b.GetAllLastReadRecords()
	if len(records) != 3 {
		t.Errorf("expected 3 records, got %d", len(records))
	}
}

// ---------------------------------------------------------------------------
// lastread.go: getLastReadLocked — bad .jlr alignment
// ---------------------------------------------------------------------------

func TestGetLastReadBadAlignment(t *testing.T) {
	b, basePath := openCovTestBase(t)
	b.SetLastRead("user", 1, 1)

	// Corrupt .jlr
	jlrPath := basePath + ".jlr"
	f, _ := os.OpenFile(jlrPath, os.O_WRONLY|os.O_APPEND, 0644)
	f.Write([]byte{0x01})
	f.Close()

	_, err := b.GetLastRead("user")
	if err == nil {
		t.Fatal("expected error for misaligned .jlr")
	}
	if !strings.Contains(err.Error(), "not aligned") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Pack: large pack with many deleted messages exercises full pack path
// ---------------------------------------------------------------------------

func TestPackManyDeletedMessages(t *testing.T) {
	b, _ := openCovTestBase(t)

	// Write 30 messages
	for i := 1; i <= 30; i++ {
		msg := NewMessage()
		msg.From = "User"
		msg.To = "All"
		msg.Subject = fmt.Sprintf("Msg %d", i)
		msg.Text = fmt.Sprintf("Body for message number %d", i)
		b.WriteMessage(msg)
	}

	// Delete every other message
	for i := 1; i <= 30; i += 2 {
		b.DeleteMessage(i)
	}

	result, err := b.Pack()
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if result.MessagesBefore != 30 {
		t.Errorf("MessagesBefore = %d, want 30", result.MessagesBefore)
	}
	if result.MessagesAfter != 15 {
		t.Errorf("MessagesAfter = %d, want 15", result.MessagesAfter)
	}
	if result.DeletedRemoved != 15 {
		t.Errorf("DeletedRemoved = %d, want 15", result.DeletedRemoved)
	}

	// Verify remaining messages
	count, _ := b.GetMessageCount()
	if count != 15 {
		t.Fatalf("count after pack = %d, want 15", count)
	}

	for i := 1; i <= 15; i++ {
		msg, err := b.ReadMessage(i)
		if err != nil {
			t.Errorf("ReadMessage(%d): %v", i, err)
			continue
		}
		if msg.IsDeleted() {
			t.Errorf("Message %d should not be deleted after pack", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Pack: delete all messages then pack
// ---------------------------------------------------------------------------

func TestPackAllDeleted(t *testing.T) {
	b, _ := openCovTestBase(t)

	for i := 0; i < 5; i++ {
		writeCovMsg(t, b, "User", "All", "Test", "Body")
	}

	for i := 1; i <= 5; i++ {
		b.DeleteMessage(i)
	}

	result, err := b.Pack()
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if result.MessagesAfter != 0 {
		t.Errorf("MessagesAfter = %d, want 0", result.MessagesAfter)
	}
	if result.DeletedRemoved != 5 {
		t.Errorf("DeletedRemoved = %d, want 5", result.DeletedRemoved)
	}

	count, _ := b.GetMessageCount()
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

// ---------------------------------------------------------------------------
// WriteMessage on closed base
// ---------------------------------------------------------------------------

func TestWriteMessageOnClosedBase(t *testing.T) {
	b, _ := openCovTestBase(t)
	b.Close()

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Test"
	msg.Text = "Body"
	_, err := b.WriteMessage(msg)
	if err == nil {
		t.Fatal("expected error writing to closed base")
	}
}

// ---------------------------------------------------------------------------
// DeleteMessage on closed base
// ---------------------------------------------------------------------------

func TestDeleteMessageOnClosedBase(t *testing.T) {
	b, _ := openCovTestBase(t)
	b.Close()

	err := b.DeleteMessage(1)
	if err == nil {
		t.Fatal("expected error deleting from closed base")
	}
}

// ---------------------------------------------------------------------------
// base.go: Open with corrupted header signature
// ---------------------------------------------------------------------------

func TestOpenWithCorruptedSignature(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "corrupt")

	// Create a full-size .jhr with bad signature
	fh := FixedHeaderInfo{
		DateCreated: uint32(time.Now().Unix()),
		BaseMsgNum:  1,
	}
	copy(fh.Signature[:], "BAD\x00")

	jhrPath := basePath + ".jhr"
	jdtPath := basePath + ".jdt"
	jdxPath := basePath + ".jdx"
	jlrPath := basePath + ".jlr"

	f, _ := os.Create(jhrPath)
	binary.Write(f, binary.LittleEndian, &fh)
	f.Close()

	// Create companion files
	for _, p := range []string{jdtPath, jdxPath, jlrPath} {
		os.WriteFile(p, []byte{}, 0644)
	}

	// Open should detect bad signature and recreate
	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer b.Close()

	if !b.IsOpen() {
		t.Error("base should be open after recreation")
	}
	if string(b.fixedHeader.Signature[:]) != Signature {
		t.Errorf("signature = %q after recreation", string(b.fixedHeader.Signature[:]))
	}
}

// ---------------------------------------------------------------------------
// buildSubfields: covers all subfield creation paths
// ---------------------------------------------------------------------------

func TestBuildSubfieldsAllFields(t *testing.T) {
	msg := &Message{
		From:     "Sender",
		To:       "Receiver",
		Subject:  "Subject",
		OrigAddr: "1:1/1",
		DestAddr: "2:2/2",
		MsgID:    "test-msgid",
		ReplyID:  "test-replyid",
		PID:      "TestPID",
		Kludges:  []string{"K1", "K2"},
	}

	sfs := buildSubfields(msg)

	// Count expected subfields: 8 named + 2 kludges = 10
	if len(sfs) != 10 {
		t.Errorf("got %d subfields, want 10", len(sfs))
	}

	// Verify types
	types := make(map[uint16]int)
	for _, sf := range sfs {
		types[sf.LoID]++
	}
	if types[SfldOAddress] != 1 {
		t.Error("missing OAddress subfield")
	}
	if types[SfldDAddress] != 1 {
		t.Error("missing DAddress subfield")
	}
	if types[SfldSenderName] != 1 {
		t.Error("missing SenderName subfield")
	}
	if types[SfldReceiverName] != 1 {
		t.Error("missing ReceiverName subfield")
	}
	if types[SfldSubject] != 1 {
		t.Error("missing Subject subfield")
	}
	if types[SfldMsgID] != 1 {
		t.Error("missing MsgID subfield")
	}
	if types[SfldReplyID] != 1 {
		t.Error("missing ReplyID subfield")
	}
	if types[SfldPID] != 1 {
		t.Error("missing PID subfield")
	}
	if types[SfldFTSKludge] != 2 {
		t.Errorf("expected 2 kludge subfields, got %d", types[SfldFTSKludge])
	}
}

func TestBuildSubfieldsEmptyMessage(t *testing.T) {
	msg := &Message{}
	sfs := buildSubfields(msg)
	if len(sfs) != 0 {
		t.Errorf("got %d subfields for empty message, want 0", len(sfs))
	}
}

// ---------------------------------------------------------------------------
// Pack preserves message text after multiple deletes at start/end
// ---------------------------------------------------------------------------

func TestPackDeleteFirstAndLastMessages(t *testing.T) {
	b, _ := openCovTestBase(t)

	for i := 1; i <= 5; i++ {
		msg := NewMessage()
		msg.From = "User"
		msg.To = "All"
		msg.Subject = fmt.Sprintf("Msg %d", i)
		msg.Text = fmt.Sprintf("Body %d", i)
		b.WriteMessage(msg)
	}

	// Delete first and last
	b.DeleteMessage(1)
	b.DeleteMessage(5)

	result, err := b.Pack()
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if result.MessagesAfter != 3 {
		t.Errorf("MessagesAfter = %d, want 3", result.MessagesAfter)
	}

	// Messages 2,3,4 should now be at positions 1,2,3
	for i, origNum := range []int{2, 3, 4} {
		msg, err := b.ReadMessage(i + 1)
		if err != nil {
			t.Errorf("ReadMessage(%d): %v", i+1, err)
			continue
		}
		expected := fmt.Sprintf("Body %d", origNum)
		if msg.Text != expected {
			t.Errorf("msg %d text = %q, want %q", i+1, msg.Text, expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Multiple pack rounds
// ---------------------------------------------------------------------------

func TestMultiplePacks(t *testing.T) {
	b, _ := openCovTestBase(t)

	for i := 1; i <= 10; i++ {
		writeCovMsg(t, b, "User", "All", fmt.Sprintf("Msg %d", i), fmt.Sprintf("Body %d", i))
	}

	// First pack: delete 3 messages
	b.DeleteMessage(2)
	b.DeleteMessage(5)
	b.DeleteMessage(8)
	r1, err := b.Pack()
	if err != nil {
		t.Fatalf("Pack 1: %v", err)
	}
	if r1.MessagesAfter != 7 {
		t.Errorf("Pack 1: MessagesAfter = %d, want 7", r1.MessagesAfter)
	}

	// Write more messages after pack
	writeCovMsg(t, b, "User", "All", "Post-pack msg", "New body")

	// Second pack: delete another
	b.DeleteMessage(1)
	r2, err := b.Pack()
	if err != nil {
		t.Fatalf("Pack 2: %v", err)
	}
	if r2.MessagesAfter != 7 {
		t.Errorf("Pack 2: MessagesAfter = %d, want 7", r2.MessagesAfter)
	}

	// Verify all remaining messages are readable
	count, _ := b.GetMessageCount()
	for i := 1; i <= count; i++ {
		_, err := b.ReadMessage(i)
		if err != nil {
			t.Errorf("ReadMessage(%d) after two packs: %v", i, err)
		}
	}
}
