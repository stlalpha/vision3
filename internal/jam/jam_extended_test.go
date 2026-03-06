package jam

import (
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

func openExtTestBase(t *testing.T) *Base {
	t.Helper()
	dir := t.TempDir()
	b, err := Open(filepath.Join(dir, "ext"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func writeSimpleMsg(t *testing.T, b *Base, from, to, subject, text string) int {
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
// Types: NewMessage, IsDeleted, IsPrivate, GetAttribute
// ---------------------------------------------------------------------------

func TestNewMessageDefaults(t *testing.T) {
	m := NewMessage()
	if m == nil {
		t.Fatal("NewMessage returned nil")
	}
	if m.DateTime.IsZero() {
		t.Error("DateTime should default to now, not zero")
	}
	if m.Header != nil {
		t.Error("Header should be nil for a new message")
	}
}

func TestIsDeletedNilHeader(t *testing.T) {
	m := &Message{}
	if m.IsDeleted() {
		t.Error("IsDeleted should return false when Header is nil")
	}
}

func TestIsPrivateNilHeader(t *testing.T) {
	m := &Message{}
	if m.IsPrivate() {
		t.Error("IsPrivate should return false when Header is nil")
	}
}

func TestIsPrivateTrue(t *testing.T) {
	m := &Message{
		Header: &MessageHeader{Attribute: MsgPrivate},
	}
	if !m.IsPrivate() {
		t.Error("IsPrivate should return true when MsgPrivate flag is set")
	}
}

func TestGetAttributeNilHeader(t *testing.T) {
	m := &Message{}
	attr := m.GetAttribute()
	if attr != MsgLocal|MsgTypeLocal {
		t.Errorf("GetAttribute with nil header = 0x%08x, want 0x%08x", attr, MsgLocal|MsgTypeLocal)
	}
}

func TestGetAttributeWithHeader(t *testing.T) {
	m := &Message{
		Header: &MessageHeader{Attribute: MsgPrivate | MsgLocal},
	}
	if m.GetAttribute() != MsgPrivate|MsgLocal {
		t.Errorf("GetAttribute = 0x%08x, want 0x%08x", m.GetAttribute(), MsgPrivate|MsgLocal)
	}
}

// ---------------------------------------------------------------------------
// Types: CreateSubfield, GetSubfieldByType, GetAllSubfieldsByType
// ---------------------------------------------------------------------------

func TestCreateSubfield(t *testing.T) {
	sf := CreateSubfield(SfldSenderName, "John")
	if sf.LoID != SfldSenderName {
		t.Errorf("LoID = %d, want %d", sf.LoID, SfldSenderName)
	}
	if sf.HiID != 0 {
		t.Errorf("HiID = %d, want 0", sf.HiID)
	}
	if sf.DatLen != 4 {
		t.Errorf("DatLen = %d, want 4", sf.DatLen)
	}
	if string(sf.Buffer) != "John" {
		t.Errorf("Buffer = %q, want %q", string(sf.Buffer), "John")
	}
}

func TestCreateSubfieldEmpty(t *testing.T) {
	sf := CreateSubfield(SfldSubject, "")
	if sf.DatLen != 0 {
		t.Errorf("DatLen = %d, want 0 for empty data", sf.DatLen)
	}
	if len(sf.Buffer) != 0 {
		t.Errorf("Buffer len = %d, want 0", len(sf.Buffer))
	}
}

func TestGetSubfieldByType(t *testing.T) {
	hdr := &MessageHeader{
		Subfields: []Subfield{
			CreateSubfield(SfldSenderName, "Alice"),
			CreateSubfield(SfldReceiverName, "Bob"),
			CreateSubfield(SfldSubject, "Hello"),
		},
	}

	sf := hdr.GetSubfieldByType(SfldReceiverName)
	if sf == nil {
		t.Fatal("GetSubfieldByType returned nil for existing type")
	}
	if string(sf.Buffer) != "Bob" {
		t.Errorf("Buffer = %q, want %q", string(sf.Buffer), "Bob")
	}

	sf = hdr.GetSubfieldByType(SfldMsgID)
	if sf != nil {
		t.Error("GetSubfieldByType should return nil for missing type")
	}
}

func TestGetAllSubfieldsByType(t *testing.T) {
	hdr := &MessageHeader{
		Subfields: []Subfield{
			CreateSubfield(SfldFTSKludge, "AREA:TEST"),
			CreateSubfield(SfldSenderName, "User"),
			CreateSubfield(SfldFTSKludge, "TID: Test"),
			CreateSubfield(SfldFTSKludge, "TZUTC: 0000"),
		},
	}

	kludges := hdr.GetAllSubfieldsByType(SfldFTSKludge)
	if len(kludges) != 3 {
		t.Fatalf("got %d kludges, want 3", len(kludges))
	}
	if string(kludges[0].Buffer) != "AREA:TEST" {
		t.Errorf("kludge[0] = %q", string(kludges[0].Buffer))
	}

	empty := hdr.GetAllSubfieldsByType(SfldMsgID)
	if len(empty) != 0 {
		t.Errorf("got %d results for missing type, want 0", len(empty))
	}
}

// ---------------------------------------------------------------------------
// Base: GetFixedHeader, GetModCounter, RefreshFixedHeader, GetActiveMessageCount
// ---------------------------------------------------------------------------

func TestGetFixedHeader(t *testing.T) {
	b := openExtTestBase(t)

	fh := b.GetFixedHeader()
	if fh == nil {
		t.Fatal("GetFixedHeader returned nil")
	}
	if string(fh.Signature[:]) != Signature {
		t.Errorf("Signature = %q, want %q", string(fh.Signature[:]), Signature)
	}
	if fh.BaseMsgNum != 1 {
		t.Errorf("BaseMsgNum = %d, want 1", fh.BaseMsgNum)
	}
}

func TestGetFixedHeaderReturnsCopy(t *testing.T) {
	b := openExtTestBase(t)
	fh1 := b.GetFixedHeader()
	fh1.ActiveMsgs = 9999 // Mutate the copy
	fh2 := b.GetFixedHeader()
	if fh2.ActiveMsgs == 9999 {
		t.Error("GetFixedHeader should return a copy, not a reference")
	}
}

func TestGetModCounter(t *testing.T) {
	b := openExtTestBase(t)

	mc1, err := b.GetModCounter()
	if err != nil {
		t.Fatalf("GetModCounter: %v", err)
	}

	writeSimpleMsg(t, b, "User", "All", "Test", "Body")

	mc2, err := b.GetModCounter()
	if err != nil {
		t.Fatalf("GetModCounter after write: %v", err)
	}
	if mc2 <= mc1 {
		t.Errorf("ModCounter should increase after write: before=%d after=%d", mc1, mc2)
	}
}

func TestRefreshFixedHeader(t *testing.T) {
	b := openExtTestBase(t)
	fh, err := b.RefreshFixedHeader()
	if err != nil {
		t.Fatalf("RefreshFixedHeader: %v", err)
	}
	if fh == nil {
		t.Fatal("RefreshFixedHeader returned nil")
	}
}

func TestGetActiveMessageCountNilHeader(t *testing.T) {
	b := &Base{} // No fixedHeader
	if b.GetActiveMessageCount() != 0 {
		t.Error("GetActiveMessageCount should return 0 with nil fixedHeader")
	}
}

// ---------------------------------------------------------------------------
// Base: operations on closed base should return ErrBaseNotOpen
// ---------------------------------------------------------------------------

func TestClosedBaseErrors(t *testing.T) {
	b := openExtTestBase(t)
	b.Close()

	if _, err := b.GetMessageCount(); err != ErrBaseNotOpen {
		t.Errorf("GetMessageCount on closed base: got %v, want ErrBaseNotOpen", err)
	}
	if _, err := b.ReadIndexRecord(1); err != ErrBaseNotOpen {
		t.Errorf("ReadIndexRecord on closed base: got %v, want ErrBaseNotOpen", err)
	}
	if _, err := b.ReadMessageHeader(1); err != ErrBaseNotOpen {
		t.Errorf("ReadMessageHeader on closed base: got %v, want ErrBaseNotOpen", err)
	}
	if _, err := b.ReadMessage(1); err != ErrBaseNotOpen {
		t.Errorf("ReadMessage on closed base: got %v, want ErrBaseNotOpen", err)
	}
	if _, err := b.ScanMessages(1, 0); err != ErrBaseNotOpen {
		t.Errorf("ScanMessages on closed base: got %v, want ErrBaseNotOpen", err)
	}
	if _, err := b.GetLastRead("user"); err != ErrBaseNotOpen {
		t.Errorf("GetLastRead on closed base: got %v, want ErrBaseNotOpen", err)
	}
	if err := b.SetLastRead("user", 1, 1); err != ErrBaseNotOpen {
		t.Errorf("SetLastRead on closed base: got %v, want ErrBaseNotOpen", err)
	}
	if err := b.MarkMessageRead("user", 1); err != ErrBaseNotOpen {
		t.Errorf("MarkMessageRead on closed base: got %v, want ErrBaseNotOpen", err)
	}
	if _, err := b.GetNextUnreadMessage("user"); err != ErrBaseNotOpen {
		t.Errorf("GetNextUnreadMessage on closed base: got %v, want ErrBaseNotOpen", err)
	}
	if _, err := b.GetUnreadCount("user"); err != ErrBaseNotOpen {
		t.Errorf("GetUnreadCount on closed base: got %v, want ErrBaseNotOpen", err)
	}
	if _, err := b.RefreshFixedHeader(); err != ErrBaseNotOpen {
		t.Errorf("RefreshFixedHeader on closed base: got %v, want ErrBaseNotOpen", err)
	}
	if _, err := b.GetNextMsgSerial(); err != ErrBaseNotOpen {
		t.Errorf("GetNextMsgSerial on closed base: got %v, want ErrBaseNotOpen", err)
	}
	if _, err := b.GetAllLastReadRecords(); err != ErrBaseNotOpen {
		t.Errorf("GetAllLastReadRecords on closed base: got %v, want ErrBaseNotOpen", err)
	}
}

// ---------------------------------------------------------------------------
// Base: Open with missing companion files should recreate
// ---------------------------------------------------------------------------

func TestOpenMissingCompanionFilesRecreates(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "partial")

	// Create a valid base first
	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b.Close()

	// Remove just the .jdt file to simulate partial corruption
	os.Remove(basePath + ".jdt")

	// Reopen should recreate
	b, err = Open(basePath)
	if err != nil {
		t.Fatalf("Open after removing .jdt: %v", err)
	}
	defer b.Close()

	if !b.IsOpen() {
		t.Error("base should be open after recreation")
	}
	for _, ext := range []string{".jhr", ".jdt", ".jdx", ".jlr"} {
		if _, err := os.Stat(basePath + ext); err != nil {
			t.Errorf("missing %s after recreation: %v", ext, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Index: CRC32 verification, deleted index sentinel
// ---------------------------------------------------------------------------

func TestIndexRecordCRC32MatchesRecipient(t *testing.T) {
	b := openExtTestBase(t)

	msg := NewMessage()
	msg.From = "Sender"
	msg.To = "TestRecipient"
	msg.Subject = "CRC Test"
	msg.Text = "Body"
	writeNum, _ := b.WriteMessage(msg)

	idx, err := b.ReadIndexRecord(writeNum)
	if err != nil {
		t.Fatalf("ReadIndexRecord: %v", err)
	}

	expectedCRC := CRC32String(strings.ToLower("TestRecipient"))
	if idx.ToCRC != expectedCRC {
		t.Errorf("ToCRC = 0x%08x, want 0x%08x", idx.ToCRC, expectedCRC)
	}
}

func TestReadIndexRecordOutOfRange(t *testing.T) {
	b := openExtTestBase(t)
	writeSimpleMsg(t, b, "User", "All", "Test", "Body")

	// Valid
	_, err := b.ReadIndexRecord(1)
	if err != nil {
		t.Fatalf("ReadIndexRecord(1): %v", err)
	}

	// Out of range
	_, err = b.ReadIndexRecord(2)
	if err != ErrInvalidMessage {
		t.Errorf("ReadIndexRecord(2) = %v, want ErrInvalidMessage", err)
	}
	_, err = b.ReadIndexRecord(0)
	if err != ErrInvalidMessage {
		t.Errorf("ReadIndexRecord(0) = %v, want ErrInvalidMessage", err)
	}
	_, err = b.ReadIndexRecord(-1)
	if err != ErrInvalidMessage {
		t.Errorf("ReadIndexRecord(-1) = %v, want ErrInvalidMessage", err)
	}
}

// ---------------------------------------------------------------------------
// Message: empty text, Windows CRLF conversion, header fields
// ---------------------------------------------------------------------------

func TestWriteAndReadEmptyText(t *testing.T) {
	b := openExtTestBase(t)

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Empty"
	msg.Text = ""
	b.WriteMessage(msg)

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.Text != "" {
		t.Errorf("Text = %q, want empty", got.Text)
	}
}

func TestTextCRLFConversion(t *testing.T) {
	b := openExtTestBase(t)

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "CRLF"
	msg.Text = "Line1\r\nLine2\r\nLine3"
	b.WriteMessage(msg)

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	// \r\n should become \r (not \r\r)
	if strings.Contains(got.Text, "\n") {
		t.Error("text should not contain LF after CRLF conversion")
	}
	expected := "Line1\rLine2\rLine3"
	if got.Text != expected {
		t.Errorf("Text = %q, want %q", got.Text, expected)
	}
}

func TestMessageHeaderSignatureIsJAM(t *testing.T) {
	b := openExtTestBase(t)
	writeSimpleMsg(t, b, "User", "All", "Test", "Body")

	hdr, err := b.ReadMessageHeader(1)
	if err != nil {
		t.Fatalf("ReadMessageHeader: %v", err)
	}
	if string(hdr.Signature[:]) != Signature {
		t.Errorf("header signature = %q, want %q", string(hdr.Signature[:]), Signature)
	}
	if hdr.Revision != 1 {
		t.Errorf("Revision = %d, want 1", hdr.Revision)
	}
}

func TestMessageHeaderDateFields(t *testing.T) {
	b := openExtTestBase(t)

	now := time.Now()
	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Date Test"
	msg.Text = "Body"
	msg.DateTime = now
	b.WriteMessage(msg)

	hdr, err := b.ReadMessageHeader(1)
	if err != nil {
		t.Fatalf("ReadMessageHeader: %v", err)
	}
	if hdr.DateWritten != uint32(now.Unix()) {
		t.Errorf("DateWritten = %d, want %d", hdr.DateWritten, uint32(now.Unix()))
	}
	if hdr.DateProcessed == 0 {
		t.Error("DateProcessed should be set for local messages via WriteMessage")
	}
}

func TestMessageNumberAssignment(t *testing.T) {
	b := openExtTestBase(t)

	for i := 1; i <= 5; i++ {
		n := writeSimpleMsg(t, b, "User", "All", fmt.Sprintf("Msg %d", i), "Body")
		if n != i {
			t.Errorf("message %d got number %d", i, n)
		}
	}

	hdr, err := b.ReadMessageHeader(3)
	if err != nil {
		t.Fatalf("ReadMessageHeader(3): %v", err)
	}
	// BaseMsgNum is 1, so MessageNumber should equal the 1-based index
	if hdr.MessageNumber != 3 {
		t.Errorf("MessageNumber = %d, want 3", hdr.MessageNumber)
	}
}

// ---------------------------------------------------------------------------
// Message: write with MSGID/ReplyID sets CRC fields
// ---------------------------------------------------------------------------

func TestWriteMessageSetsCRCFields(t *testing.T) {
	b := openExtTestBase(t)

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "CRC test"
	msg.Text = "Body"
	msg.MsgID = "1:103/705 aabbccdd"
	msg.ReplyID = "1:103/705 11223344"
	b.WriteMessage(msg)

	hdr, err := b.ReadMessageHeader(1)
	if err != nil {
		t.Fatalf("ReadMessageHeader: %v", err)
	}

	expectedMsgIDCRC := CRC32String(msg.MsgID)
	if hdr.MSGIDcrc != expectedMsgIDCRC {
		t.Errorf("MSGIDcrc = 0x%08x, want 0x%08x", hdr.MSGIDcrc, expectedMsgIDCRC)
	}

	expectedReplyCRC := CRC32String(msg.ReplyID)
	if hdr.REPLYcrc != expectedReplyCRC {
		t.Errorf("REPLYcrc = 0x%08x, want 0x%08x", hdr.REPLYcrc, expectedReplyCRC)
	}
}

// ---------------------------------------------------------------------------
// Message: ReadMessageText with zero-length text
// ---------------------------------------------------------------------------

func TestReadMessageTextZeroLength(t *testing.T) {
	b := openExtTestBase(t)
	writeSimpleMsg(t, b, "User", "All", "Empty", "")

	hdr, err := b.ReadMessageHeader(1)
	if err != nil {
		t.Fatalf("ReadMessageHeader: %v", err)
	}

	text, err := b.ReadMessageText(hdr)
	if err != nil {
		t.Fatalf("ReadMessageText: %v", err)
	}
	if text != "" {
		t.Errorf("text = %q, want empty", text)
	}
}

// ---------------------------------------------------------------------------
// Message: ScanMessages with startMsg adjustment
// ---------------------------------------------------------------------------

func TestScanMessagesStartAdjusted(t *testing.T) {
	b := openExtTestBase(t)
	for i := 0; i < 5; i++ {
		writeSimpleMsg(t, b, "User", "All", fmt.Sprintf("Msg %d", i+1), "Body")
	}

	// Start before 1 should be adjusted to 1
	msgs, err := b.ScanMessages(0, 0)
	if err != nil {
		t.Fatalf("ScanMessages: %v", err)
	}
	if len(msgs) != 5 {
		t.Errorf("got %d messages, want 5", len(msgs))
	}

	// Start at 3
	msgs, err = b.ScanMessages(3, 0)
	if err != nil {
		t.Fatalf("ScanMessages: %v", err)
	}
	if len(msgs) != 3 {
		t.Errorf("got %d messages from start=3, want 3", len(msgs))
	}
}

// ---------------------------------------------------------------------------
// LastRead: GetAllLastReadRecords, ResetLastRead
// ---------------------------------------------------------------------------

func TestGetAllLastReadRecords(t *testing.T) {
	b := openExtTestBase(t)
	writeSimpleMsg(t, b, "User", "All", "Test", "Body")

	b.SetLastRead("alice", 1, 1)
	b.SetLastRead("bob", 1, 1)
	b.SetLastRead("charlie", 1, 1)

	records, err := b.GetAllLastReadRecords()
	if err != nil {
		t.Fatalf("GetAllLastReadRecords: %v", err)
	}
	if len(records) != 3 {
		t.Errorf("got %d records, want 3", len(records))
	}
}

func TestGetAllLastReadRecordsEmpty(t *testing.T) {
	b := openExtTestBase(t)

	records, err := b.GetAllLastReadRecords()
	if err != nil {
		t.Fatalf("GetAllLastReadRecords: %v", err)
	}
	if records != nil {
		t.Errorf("expected nil for empty .jlr, got %d records", len(records))
	}
}

func TestResetLastRead(t *testing.T) {
	b := openExtTestBase(t)
	writeSimpleMsg(t, b, "User", "All", "Test", "Body")

	b.SetLastRead("testuser", 5, 10)
	if err := b.ResetLastRead("testuser"); err != nil {
		t.Fatalf("ResetLastRead: %v", err)
	}

	lr, err := b.GetLastRead("testuser")
	if err != nil {
		t.Fatalf("GetLastRead after reset: %v", err)
	}
	if lr.LastReadMsg != 0 || lr.HighReadMsg != 0 {
		t.Errorf("after reset: LastRead=%d High=%d, want 0/0", lr.LastReadMsg, lr.HighReadMsg)
	}
}

// ---------------------------------------------------------------------------
// LastRead: GetUnreadCount edge cases
// ---------------------------------------------------------------------------

func TestGetUnreadCountEmptyBase(t *testing.T) {
	b := openExtTestBase(t)

	count, err := b.GetUnreadCount("user")
	if err != nil {
		t.Fatalf("GetUnreadCount: %v", err)
	}
	if count != 0 {
		t.Errorf("unread = %d, want 0 for empty base", count)
	}
}

func TestGetUnreadCountLastReadBeyondCount(t *testing.T) {
	b := openExtTestBase(t)
	writeSimpleMsg(t, b, "User", "All", "Test", "Body")
	b.SetLastRead("user", 100, 100) // beyond actual count

	count, err := b.GetUnreadCount("user")
	if err != nil {
		t.Fatalf("GetUnreadCount: %v", err)
	}
	if count != 0 {
		t.Errorf("unread = %d, want 0 when lastread exceeds count", count)
	}
}

// ---------------------------------------------------------------------------
// LastRead: GetNextUnreadMessage empty base
// ---------------------------------------------------------------------------

func TestGetNextUnreadMessageEmptyBase(t *testing.T) {
	b := openExtTestBase(t)

	_, err := b.GetNextUnreadMessage("user")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// CRC32: additional cases
// ---------------------------------------------------------------------------

func TestCRC32StringNonASCII(t *testing.T) {
	// Non-A-Z characters should pass through unchanged
	c1 := CRC32String("user123")
	c2 := CRC32String("user123")
	if c1 != c2 {
		t.Error("same non-alpha string should produce same CRC")
	}

	// Only A-Z is lowered, not locale chars
	c3 := CRC32String("user 123!")
	c4 := CRC32String("USER 123!")
	if c3 != c4 {
		t.Error("A-Z lowering should apply regardless of other chars")
	}
}

func TestCRC32InvertedOutput(t *testing.T) {
	// Verify the result is inverted (XOR with 0xFFFFFFFF)
	// CRC32 of empty string with IEEE polynomial is 0x00000000
	// Inverted: 0xFFFFFFFF
	c := CRC32String("")
	if c != 0xFFFFFFFF {
		t.Errorf("CRC32 of empty = 0x%08x, want 0xFFFFFFFF", c)
	}
}

// ---------------------------------------------------------------------------
// Format: AddTearline, AddCustomTearline, AddOriginLine, FormatPID
// ---------------------------------------------------------------------------

func TestAddTearline(t *testing.T) {
	text := "Hello world"
	result := AddTearline(text)
	if !strings.Contains(result, "--- ViSiON/3") {
		t.Errorf("AddTearline should add ViSiON/3 tearline, got %q", result)
	}
	if !strings.HasSuffix(result, "\n") {
		t.Error("tearline should end with newline")
	}
}

func TestAddCustomTearlineEmpty(t *testing.T) {
	result := AddCustomTearline("Text", "")
	if !strings.Contains(result, "--- ViSiON/3") {
		t.Error("empty custom tearline should use default")
	}
}

func TestAddCustomTearlineWithPrefix(t *testing.T) {
	result := AddCustomTearline("Text", "--- MyTearline")
	if !strings.Contains(result, "--- MyTearline") {
		t.Error("tearline starting with --- should be used as-is")
	}
	// Should NOT add an extra "--- " prefix
	if strings.Contains(result, "--- --- ") {
		t.Error("should not double the --- prefix")
	}
}

func TestAddCustomTearlineCustom(t *testing.T) {
	result := AddCustomTearline("Text", "CustomSoft 1.0")
	if !strings.Contains(result, "--- CustomSoft 1.0") {
		t.Errorf("expected custom tearline, got %q", result)
	}
}

func TestAddTearlineAppendsNewline(t *testing.T) {
	// Input without trailing newline
	result := AddTearline("NoNewline")
	lines := strings.Split(result, "\n")
	// Should have added a newline before tearline
	if len(lines) < 2 {
		t.Error("should have multiple lines after adding tearline")
	}
}

func TestAddOriginLine(t *testing.T) {
	result := AddOriginLine("Text", "My BBS", "1:103/705")
	if !strings.Contains(result, " * Origin: My BBS (1:103/705)") {
		t.Errorf("origin line not found in %q", result)
	}
}

func TestAddOriginLineAppendsNewline(t *testing.T) {
	result := AddOriginLine("NoNewline", "BBS", "1:1/1")
	if !strings.HasSuffix(result, "\n") {
		t.Error("origin line should end with newline")
	}
}

func TestFormatPID(t *testing.T) {
	pid := FormatPID()
	if !strings.HasPrefix(pid, "ViSiON/3") {
		t.Errorf("FormatPID = %q, should start with ViSiON/3", pid)
	}
}

func TestFormatTID(t *testing.T) {
	tid := FormatTID()
	if tid != FormatPID() {
		t.Error("FormatTID should equal FormatPID")
	}
}

// ---------------------------------------------------------------------------
// MessageType methods
// ---------------------------------------------------------------------------

func TestMessageTypeMethods(t *testing.T) {
	if !MsgTypeLocalMsg.IsLocal() {
		t.Error("MsgTypeLocalMsg.IsLocal should be true")
	}
	if MsgTypeLocalMsg.IsEchomail() {
		t.Error("MsgTypeLocalMsg.IsEchomail should be false")
	}
	if MsgTypeLocalMsg.IsNetmail() {
		t.Error("MsgTypeLocalMsg.IsNetmail should be false")
	}

	if !MsgTypeEchomailMsg.IsEchomail() {
		t.Error("MsgTypeEchomailMsg.IsEchomail should be true")
	}
	if MsgTypeEchomailMsg.IsLocal() {
		t.Error("MsgTypeEchomailMsg.IsLocal should be false")
	}

	if !MsgTypeNetmailMsg.IsNetmail() {
		t.Error("MsgTypeNetmailMsg.IsNetmail should be true")
	}
	if MsgTypeNetmailMsg.IsEchomail() {
		t.Error("MsgTypeNetmailMsg.IsEchomail should be false")
	}
}

func TestGetJAMAttribute(t *testing.T) {
	tests := []struct {
		mt   MessageType
		want uint32
	}{
		{MsgTypeLocalMsg, MsgLocal | MsgTypeLocal},
		{MsgTypeEchomailMsg, MsgLocal | MsgTypeEcho},
		{MsgTypeNetmailMsg, MsgLocal | MsgTypeNet},
	}
	for _, tt := range tests {
		got := tt.mt.GetJAMAttribute()
		if got != tt.want {
			t.Errorf("GetJAMAttribute(%d) = 0x%08x, want 0x%08x", tt.mt, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// GenerateMSGID
// ---------------------------------------------------------------------------

func TestGenerateMSGID(t *testing.T) {
	b := openExtTestBase(t)

	msgID, err := b.GenerateMSGID("1:103/705")
	if err != nil {
		t.Fatalf("GenerateMSGID: %v", err)
	}
	if !strings.HasPrefix(msgID, "1:103/705 ") {
		t.Errorf("MSGID = %q, should start with address", msgID)
	}
	// Should have 8 hex chars after space
	parts := strings.SplitN(msgID, " ", 2)
	if len(parts) != 2 || len(parts[1]) != 8 {
		t.Errorf("MSGID serial part length wrong: %q", msgID)
	}
}

func TestGenerateMSGIDIncrementing(t *testing.T) {
	b := openExtTestBase(t)

	id1, _ := b.GenerateMSGID("1:1/1")
	id2, _ := b.GenerateMSGID("1:1/1")
	if id1 == id2 {
		t.Error("consecutive MSGIDs should differ")
	}
}

// ---------------------------------------------------------------------------
// UpdateMessageHeader
// ---------------------------------------------------------------------------

func TestUpdateMessageHeader(t *testing.T) {
	b := openExtTestBase(t)
	writeSimpleMsg(t, b, "User", "All", "Test", "Body")

	hdr, err := b.ReadMessageHeader(1)
	if err != nil {
		t.Fatalf("ReadMessageHeader: %v", err)
	}

	// Update TimesRead
	hdr.TimesRead = 42
	if err := b.UpdateMessageHeader(1, hdr); err != nil {
		t.Fatalf("UpdateMessageHeader: %v", err)
	}

	hdr2, err := b.ReadMessageHeader(1)
	if err != nil {
		t.Fatalf("ReadMessageHeader after update: %v", err)
	}
	if hdr2.TimesRead != 42 {
		t.Errorf("TimesRead = %d, want 42", hdr2.TimesRead)
	}
}

// ---------------------------------------------------------------------------
// Delete then read: deleted message text length should be zero
// ---------------------------------------------------------------------------

func TestDeletedMessageTextLengthZero(t *testing.T) {
	b := openExtTestBase(t)
	writeSimpleMsg(t, b, "User", "All", "Test", "Some text here")
	b.DeleteMessage(1)

	hdr, err := b.ReadMessageHeader(1)
	if err != nil {
		t.Fatalf("ReadMessageHeader: %v", err)
	}
	if hdr.TxtLen != 0 {
		t.Errorf("deleted msg TxtLen = %d, want 0", hdr.TxtLen)
	}
	if hdr.Attribute&MsgDeleted == 0 {
		t.Error("deleted msg should have MsgDeleted flag")
	}
}

// ---------------------------------------------------------------------------
// Delete: invalid message number
// ---------------------------------------------------------------------------

func TestDeleteInvalidMessage(t *testing.T) {
	b := openExtTestBase(t)
	err := b.DeleteMessage(1)
	if err == nil {
		t.Error("expected error deleting from empty base")
	}
}

// ---------------------------------------------------------------------------
// Link: reply threading
// ---------------------------------------------------------------------------

func TestLinkBasicThreading(t *testing.T) {
	b := openExtTestBase(t)

	// Write parent with MSGID
	parent := NewMessage()
	parent.From = "Alice"
	parent.To = "All"
	parent.Subject = "Parent"
	parent.Text = "Parent body"
	parent.MsgID = "1:103/705 00000001"
	b.WriteMessage(parent)

	// Write reply referencing parent MSGID
	reply := NewMessage()
	reply.From = "Bob"
	reply.To = "Alice"
	reply.Subject = "Re: Parent"
	reply.Text = "Reply body"
	reply.MsgID = "1:103/705 00000002"
	reply.ReplyID = "1:103/705 00000001"
	b.WriteMessage(reply)

	result, err := b.Link()
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if result.MessagesScanned != 2 {
		t.Errorf("MessagesScanned = %d, want 2", result.MessagesScanned)
	}
	if result.LinksUpdated == 0 {
		t.Error("LinksUpdated should be > 0")
	}

	// Parent should have Reply1st pointing to msg 2
	parentHdr, _ := b.ReadMessageHeader(1)
	if parentHdr.Reply1st != 2 {
		t.Errorf("parent Reply1st = %d, want 2", parentHdr.Reply1st)
	}

	// Reply should have ReplyTo pointing to msg 1
	replyHdr, _ := b.ReadMessageHeader(2)
	if replyHdr.ReplyTo != 1 {
		t.Errorf("reply ReplyTo = %d, want 1", replyHdr.ReplyTo)
	}
}

func TestLinkEmptyBase(t *testing.T) {
	b := openExtTestBase(t)
	result, err := b.Link()
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if result.MessagesScanned != 0 || result.LinksUpdated != 0 {
		t.Errorf("Link empty: scanned=%d updated=%d", result.MessagesScanned, result.LinksUpdated)
	}
}

func TestLinkSiblingReplies(t *testing.T) {
	b := openExtTestBase(t)

	// Parent
	parent := NewMessage()
	parent.From = "Alice"
	parent.To = "All"
	parent.Subject = "Thread"
	parent.Text = "Start"
	parent.MsgID = "1:1/1 00000001"
	b.WriteMessage(parent)

	// Reply 1
	r1 := NewMessage()
	r1.From = "Bob"
	r1.To = "Alice"
	r1.Subject = "Re: Thread"
	r1.Text = "Reply 1"
	r1.MsgID = "1:1/2 00000001"
	r1.ReplyID = "1:1/1 00000001"
	b.WriteMessage(r1)

	// Reply 2 (sibling)
	r2 := NewMessage()
	r2.From = "Charlie"
	r2.To = "Alice"
	r2.Subject = "Re: Thread"
	r2.Text = "Reply 2"
	r2.MsgID = "1:1/3 00000001"
	r2.ReplyID = "1:1/1 00000001"
	b.WriteMessage(r2)

	b.Link()

	// First reply should have ReplyNext pointing to second reply
	r1Hdr, _ := b.ReadMessageHeader(2)
	if r1Hdr.ReplyNext != 3 {
		t.Errorf("r1 ReplyNext = %d, want 3", r1Hdr.ReplyNext)
	}

	// Second reply should have ReplyNext = 0 (last sibling)
	r2Hdr, _ := b.ReadMessageHeader(3)
	if r2Hdr.ReplyNext != 0 {
		t.Errorf("r2 ReplyNext = %d, want 0", r2Hdr.ReplyNext)
	}
}

// ---------------------------------------------------------------------------
// PackWithReplyIDCleanup
// ---------------------------------------------------------------------------

func TestPackWithReplyIDCleanup(t *testing.T) {
	b := openExtTestBase(t)

	// Write a message with a malformed ReplyID (multiple tokens)
	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Test"
	msg.Text = "Body"
	msg.MsgID = "1:1/1 aabbccdd"
	msg.ReplyID = "1:1/1 11223344 extragarbage"
	b.WriteMessage(msg)

	result, err := b.PackWithReplyIDCleanup()
	if err != nil {
		t.Fatalf("PackWithReplyIDCleanup: %v", err)
	}
	if result.MessagesAfter != 1 {
		t.Errorf("MessagesAfter = %d, want 1", result.MessagesAfter)
	}

	// Read back and verify ReplyID was cleaned
	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if strings.Contains(got.ReplyID, "extragarbage") {
		t.Errorf("ReplyID not cleaned: %q", got.ReplyID)
	}
	if got.ReplyID != "1:1/1" {
		t.Errorf("ReplyID = %q, want %q", got.ReplyID, "1:1/1")
	}
}

// ---------------------------------------------------------------------------
// Base: double close should not panic
// ---------------------------------------------------------------------------

func TestDoubleClose(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(filepath.Join(dir, "dblclose"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := b.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close should not panic (files are nil)
	err = b.Close()
	// No assertion on error — just verifying no panic
	_ = err
}

// ---------------------------------------------------------------------------
// Base: persistence across close/reopen
// ---------------------------------------------------------------------------

func TestMessagePersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "persist")

	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	msg := NewMessage()
	msg.From = "Persistent"
	msg.To = "All"
	msg.Subject = "Survive Reopen"
	msg.Text = "This should persist"
	b.WriteMessage(msg)
	b.Close()

	// Reopen
	b, err = Open(basePath)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer b.Close()

	count, _ := b.GetMessageCount()
	if count != 1 {
		t.Fatalf("count after reopen = %d, want 1", count)
	}

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage after reopen: %v", err)
	}
	if got.From != "Persistent" {
		t.Errorf("From = %q, want %q", got.From, "Persistent")
	}
	if got.Subject != "Survive Reopen" {
		t.Errorf("Subject = %q", got.Subject)
	}
}

// ---------------------------------------------------------------------------
// Base: Open with invalid signature in .jhr should recreate
// ---------------------------------------------------------------------------

func TestOpenInvalidSignatureRecreates(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "badsig")

	// Create a valid base, then corrupt the signature
	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b.Close()

	// Overwrite first 4 bytes of .jhr with wrong signature (but keep full 1024 bytes)
	data, _ := os.ReadFile(basePath + ".jhr")
	copy(data[0:4], []byte("BAD\x00"))
	os.WriteFile(basePath+".jhr", data, 0644)

	// Reopen should detect invalid signature and recreate
	b, err = Open(basePath)
	if err != nil {
		t.Fatalf("Open with bad signature: %v", err)
	}
	defer b.Close()

	if !b.IsOpen() {
		t.Error("should be open after recreating from bad signature")
	}
}

// ---------------------------------------------------------------------------
// WriteMessageExt: netmail message type
// ---------------------------------------------------------------------------

func TestWriteMessageExtNetmail(t *testing.T) {
	b := openExtTestBase(t)

	msg := NewMessage()
	msg.From = "Sysop"
	msg.To = "Remote User"
	msg.Subject = "Netmail Test"
	msg.Text = "Direct message"
	msg.OrigAddr = "1:103/705"
	msg.DestAddr = "1:103/706"

	_, err := b.WriteMessageExt(msg, MsgTypeNetmailMsg, "", "BBS", "")
	if err != nil {
		t.Fatalf("WriteMessageExt netmail: %v", err)
	}

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.Header.Attribute&MsgTypeNet == 0 {
		t.Error("netmail should have MsgTypeNet attribute")
	}
	if got.DestAddr != "1:103/706" {
		t.Errorf("DestAddr = %q, want %q", got.DestAddr, "1:103/706")
	}
}

// ---------------------------------------------------------------------------
// WriteMessageExt: echomail with custom tearline
// ---------------------------------------------------------------------------

func TestWriteMessageExtCustomTearline(t *testing.T) {
	b := openExtTestBase(t)

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Custom TL"
	msg.Text = "Body text"
	msg.OrigAddr = "1:1/1"

	_, err := b.WriteMessageExt(msg, MsgTypeEchomailMsg, "TEST", "BBS", "MySoft 2.0")
	if err != nil {
		t.Fatalf("WriteMessageExt: %v", err)
	}

	got, _ := b.ReadMessage(1)
	if !strings.Contains(got.Text, "--- MySoft 2.0") {
		t.Errorf("custom tearline not found in %q", got.Text)
	}
}

// ---------------------------------------------------------------------------
// WriteMessageExt: echomail with SeenBy and Path
// ---------------------------------------------------------------------------

func TestWriteMessageExtSeenByPath(t *testing.T) {
	b := openExtTestBase(t)

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "SeenBy"
	msg.Text = "Body"
	msg.OrigAddr = "1:1/1"
	msg.SeenBy = "103/705"
	msg.Path = "103/705"

	b.WriteMessageExt(msg, MsgTypeEchomailMsg, "TEST", "BBS", "")

	got, _ := b.ReadMessage(1)
	if got.SeenBy != "103/705" {
		t.Errorf("SeenBy = %q, want %q", got.SeenBy, "103/705")
	}
	if got.Path != "103/705" {
		t.Errorf("Path = %q, want %q", got.Path, "103/705")
	}
}

// ---------------------------------------------------------------------------
// buildSubfields: coverage of all fields
// ---------------------------------------------------------------------------

func TestBuildSubfieldsComprehensive(t *testing.T) {
	msg := &Message{
		From:     "Alice",
		To:       "Bob",
		Subject:  "Test",
		OrigAddr: "1:1/1",
		DestAddr: "1:1/2",
		MsgID:    "1:1/1 abcdef00",
		ReplyID:  "1:1/1 00000001",
		PID:      "TestPID",
		Kludges:  []string{"AREA:TEST", "TID: Test"},
	}

	sfs := buildSubfields(msg)

	typeMap := make(map[uint16]int)
	for _, sf := range sfs {
		typeMap[sf.LoID]++
	}

	expected := map[uint16]int{
		SfldOAddress:     1,
		SfldDAddress:     1,
		SfldSenderName:   1,
		SfldReceiverName: 1,
		SfldSubject:      1,
		SfldMsgID:        1,
		SfldReplyID:      1,
		SfldPID:          1,
		SfldFTSKludge:    2,
	}

	for typ, count := range expected {
		if typeMap[typ] != count {
			t.Errorf("subfield type %d: got %d, want %d", typ, typeMap[typ], count)
		}
	}
}

func TestBuildSubfieldsMinimal(t *testing.T) {
	msg := &Message{}
	sfs := buildSubfields(msg)
	if len(sfs) != 0 {
		t.Errorf("buildSubfields for empty msg should return 0, got %d", len(sfs))
	}
}

// ---------------------------------------------------------------------------
// Pack: verify base is operational after pack
// ---------------------------------------------------------------------------

func TestPackThenWriteMore(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "packwrite")

	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer b.Close()

	// Write and delete
	for i := 0; i < 5; i++ {
		writeSimpleMsg(t, b, "User", "All", fmt.Sprintf("M%d", i+1), "Body")
	}
	b.DeleteMessage(2)
	b.DeleteMessage(4)

	if _, err := b.Pack(); err != nil {
		t.Fatalf("Pack: %v", err)
	}

	// Write more after pack
	n := writeSimpleMsg(t, b, "NewUser", "All", "After Pack", "New body")
	if n != 4 { // 3 survived + 1 new
		t.Errorf("msg number after pack = %d, want 4", n)
	}

	got, err := b.ReadMessage(n)
	if err != nil {
		t.Fatalf("ReadMessage after pack+write: %v", err)
	}
	if got.From != "NewUser" {
		t.Errorf("From = %q", got.From)
	}
}

// ---------------------------------------------------------------------------
// ReadMessage: echomail address extraction from origin line
// ---------------------------------------------------------------------------

func TestReadMessageExtractsOriginAddress(t *testing.T) {
	b := openExtTestBase(t)

	// Simulate a message with echomail attribute but no OAddress subfield,
	// which has an origin line in the text. We use WriteMessage with a
	// header that has the echo attribute.
	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Test"
	msg.Text = "Hello\r * Origin: My BBS (1:103/705)\r"
	msg.Header = &MessageHeader{Attribute: MsgLocal | MsgTypeEcho}
	b.WriteMessage(msg)

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.OrigAddr != "1:103/705" {
		t.Errorf("OrigAddr extracted = %q, want %q", got.OrigAddr, "1:103/705")
	}
}

// ---------------------------------------------------------------------------
// Serial counter persistence
// ---------------------------------------------------------------------------

func TestSerialCounterPersists(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "serial")

	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s1, _ := b.GetNextMsgSerial()
	s2, _ := b.GetNextMsgSerial()
	b.Close()

	// Reopen and check serial continues
	b, err = Open(basePath)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer b.Close()

	s3, _ := b.GetNextMsgSerial()
	if s3 <= s2 {
		t.Errorf("serial after reopen %d should be > %d", s3, s2)
	}
	_ = s1
}

// ---------------------------------------------------------------------------
// FixedHeader: BaseMsgNum used correctly in message numbering
// ---------------------------------------------------------------------------

func TestFixedHeaderBaseMsgNum(t *testing.T) {
	b := openExtTestBase(t)
	fh := b.GetFixedHeader()
	if fh.BaseMsgNum != 1 {
		t.Errorf("BaseMsgNum = %d, want 1", fh.BaseMsgNum)
	}
}

// ---------------------------------------------------------------------------
// writeIndexRecord / readIndexRecord round-trip via WriteMessage
// ---------------------------------------------------------------------------

func TestIndexRecordHdrOffset(t *testing.T) {
	b := openExtTestBase(t)
	writeSimpleMsg(t, b, "User", "All", "Test", "Body")

	idx, err := b.ReadIndexRecord(1)
	if err != nil {
		t.Fatalf("ReadIndexRecord: %v", err)
	}
	// HdrOffset should be at least HeaderSize (1024), since the fixed header occupies bytes 0-1023
	if idx.HdrOffset < uint32(HeaderSize) {
		t.Errorf("HdrOffset = %d, should be >= %d", idx.HdrOffset, HeaderSize)
	}
}

// ---------------------------------------------------------------------------
// ModCounter increments on write and delete
// ---------------------------------------------------------------------------

func TestModCounterIncrementsOnWriteAndDelete(t *testing.T) {
	b := openExtTestBase(t)

	mc0, _ := b.GetModCounter()
	writeSimpleMsg(t, b, "User", "All", "M1", "Body")
	mc1, _ := b.GetModCounter()
	if mc1 <= mc0 {
		t.Errorf("ModCounter should increase after write: %d -> %d", mc0, mc1)
	}

	b.DeleteMessage(1)
	mc2, _ := b.GetModCounter()
	if mc2 <= mc1 {
		t.Errorf("ModCounter should increase after delete: %d -> %d", mc1, mc2)
	}
}

// ---------------------------------------------------------------------------
// LastRead: CRC is case-insensitive
// ---------------------------------------------------------------------------

func TestLastReadCaseInsensitive(t *testing.T) {
	b := openExtTestBase(t)
	writeSimpleMsg(t, b, "User", "All", "Test", "Body")

	b.SetLastRead("TestUser", 1, 1)
	lr, err := b.GetLastRead("testuser")
	if err != nil {
		t.Fatalf("GetLastRead with different case: %v", err)
	}
	if lr.LastReadMsg != 1 {
		t.Errorf("LastReadMsg = %d, want 1", lr.LastReadMsg)
	}
}

// ---------------------------------------------------------------------------
// Multiple index records: verify separate offsets
// ---------------------------------------------------------------------------

func TestMultipleIndexRecordsSeparateOffsets(t *testing.T) {
	b := openExtTestBase(t)
	writeSimpleMsg(t, b, "User", "Alice", "M1", "Body1")
	writeSimpleMsg(t, b, "User", "Bob", "M2", "Body2")
	writeSimpleMsg(t, b, "User", "Charlie", "M3", "Body3")

	offsets := make(map[uint32]bool)
	for i := 1; i <= 3; i++ {
		idx, err := b.ReadIndexRecord(i)
		if err != nil {
			t.Fatalf("ReadIndexRecord(%d): %v", i, err)
		}
		if offsets[idx.HdrOffset] {
			t.Errorf("duplicate HdrOffset %d for message %d", idx.HdrOffset, i)
		}
		offsets[idx.HdrOffset] = true
	}
}

// ---------------------------------------------------------------------------
// Subfield round-trip for all message fields
// ---------------------------------------------------------------------------

func TestSubfieldRoundTripAllFields(t *testing.T) {
	b := openExtTestBase(t)

	msg := NewMessage()
	msg.From = "Sender"
	msg.To = "Receiver"
	msg.Subject = "Topic"
	msg.Text = "Message body text"
	msg.OrigAddr = "1:103/705"
	msg.DestAddr = "1:103/706"
	msg.MsgID = "1:103/705 12345678"
	msg.ReplyID = "1:103/705 00000001"
	msg.PID = "TestApp 1.0"
	msg.Kludges = []string{"AREA:TESTECHO", "TID: TestApp 1.0"}
	b.WriteMessage(msg)

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.From != "Sender" {
		t.Errorf("From = %q", got.From)
	}
	if got.To != "Receiver" {
		t.Errorf("To = %q", got.To)
	}
	if got.Subject != "Topic" {
		t.Errorf("Subject = %q", got.Subject)
	}
	if got.OrigAddr != "1:103/705" {
		t.Errorf("OrigAddr = %q", got.OrigAddr)
	}
	if got.DestAddr != "1:103/706" {
		t.Errorf("DestAddr = %q", got.DestAddr)
	}
	if got.MsgID != "1:103/705 12345678" {
		t.Errorf("MsgID = %q", got.MsgID)
	}
	if got.ReplyID != "1:103/705 00000001" {
		t.Errorf("ReplyID = %q", got.ReplyID)
	}
	if got.PID != "TestApp 1.0" {
		t.Errorf("PID = %q", got.PID)
	}
	if len(got.Kludges) != 2 {
		t.Errorf("Kludges count = %d, want 2", len(got.Kludges))
	}
}

// ---------------------------------------------------------------------------
// WriteMessageExt: kludges passed through
// ---------------------------------------------------------------------------

func TestWriteMessageExtKludges(t *testing.T) {
	b := openExtTestBase(t)

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Kludge Test"
	msg.Text = "Body"
	msg.Kludges = []string{"CUSTOM: value1", "CUSTOM: value2"}

	b.WriteMessageExt(msg, MsgTypeLocalMsg, "", "", "")

	got, _ := b.ReadMessage(1)
	found := 0
	for _, k := range got.Kludges {
		if strings.HasPrefix(k, "CUSTOM:") {
			found++
		}
	}
	if found != 2 {
		t.Errorf("expected 2 CUSTOM kludges, got %d", found)
	}
}

// ---------------------------------------------------------------------------
// CP437ToUnicode: extended range
// ---------------------------------------------------------------------------

func TestCP437ToUnicodeExtended(t *testing.T) {
	// 0x80 = U+00C7 (Latin capital C with cedilla)
	result := CP437ToUnicode([]byte{0x80})
	if result != "\u00C7" {
		t.Errorf("0x80 = %q, want U+00C7", result)
	}

	// 0xFF = U+00A0 (non-breaking space)
	result = CP437ToUnicode([]byte{0xFF})
	if result != "\u00A0" {
		t.Errorf("0xFF = %q, want U+00A0", result)
	}

	// Mixed ASCII and high bytes
	result = CP437ToUnicode([]byte{'A', 0x80, 'B'})
	if result != "A\u00C7B" {
		t.Errorf("mixed = %q", result)
	}
}

// ---------------------------------------------------------------------------
// Large message write and read
// ---------------------------------------------------------------------------

func TestLargeMessageBody(t *testing.T) {
	b := openExtTestBase(t)

	largeText := strings.Repeat("ABCDEFGHIJ", 1000) // 10KB
	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Large"
	msg.Text = largeText
	b.WriteMessage(msg)

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if len(got.Text) != 10000 {
		t.Errorf("text length = %d, want 10000", len(got.Text))
	}
}

// ---------------------------------------------------------------------------
// Pack closed base should error
// ---------------------------------------------------------------------------

func TestPackClosedBase(t *testing.T) {
	b := openExtTestBase(t)
	b.Close()

	_, err := b.Pack()
	if err != ErrBaseNotOpen {
		t.Errorf("Pack on closed base: got %v, want ErrBaseNotOpen", err)
	}
}

// ---------------------------------------------------------------------------
// Link closed base should error
// ---------------------------------------------------------------------------

func TestLinkClosedBase(t *testing.T) {
	b := openExtTestBase(t)
	b.Close()

	_, err := b.Link()
	if err != ErrBaseNotOpen {
		t.Errorf("Link on closed base: got %v, want ErrBaseNotOpen", err)
	}
}

// ---------------------------------------------------------------------------
// Base: Open with empty BasePath
// ---------------------------------------------------------------------------

func TestAcquireFileLockEmptyBasePath(t *testing.T) {
	b := &Base{BasePath: ""}
	release, err := b.acquireFileLock()
	if err != nil {
		t.Fatalf("acquireFileLock with empty path: %v", err)
	}
	release() // Should be a no-op
}

// ---------------------------------------------------------------------------
// FixedHeader: serial counter starts from time-based seed
// ---------------------------------------------------------------------------

func TestSerialCounterSeed(t *testing.T) {
	b := openExtTestBase(t)

	// First serial should be based on time.Now().Unix()
	s, err := b.GetNextMsgSerial()
	if err != nil {
		t.Fatalf("GetNextMsgSerial: %v", err)
	}
	now := uint32(time.Now().Unix())
	// Should be roughly equal to now (within a few seconds)
	diff := int64(s) - int64(now)
	if diff < -5 || diff > 5 {
		t.Errorf("first serial %d too far from now %d (diff=%d)", s, now, diff)
	}
}

// ---------------------------------------------------------------------------
// writeFixedHeader round trip via GetFixedHeader after serial increment
// ---------------------------------------------------------------------------

func TestFixedHeaderSerialStoredInReserved(t *testing.T) {
	b := openExtTestBase(t)
	s1, _ := b.GetNextMsgSerial()

	fh := b.GetFixedHeader()
	stored := binary.LittleEndian.Uint32(fh.Reserved[0:4])
	if stored != s1 {
		t.Errorf("Reserved[0:4] = %d, want %d (latest serial)", stored, s1)
	}
}

// ---------------------------------------------------------------------------
// WriteMessage on closed base
// ---------------------------------------------------------------------------

func TestWriteMessageClosedBase(t *testing.T) {
	b := openExtTestBase(t)
	b.Close()

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Test"
	msg.Text = "Body"
	_, err := b.WriteMessage(msg)
	if err != ErrBaseNotOpen {
		t.Errorf("WriteMessage on closed base: got %v, want ErrBaseNotOpen", err)
	}
}

// ---------------------------------------------------------------------------
// DeleteMessage on closed base
// ---------------------------------------------------------------------------

func TestDeleteMessageClosedBase(t *testing.T) {
	b := openExtTestBase(t)
	b.Close()

	err := b.DeleteMessage(1)
	if err != ErrBaseNotOpen {
		t.Errorf("DeleteMessage on closed base: got %v, want ErrBaseNotOpen", err)
	}
}

// ---------------------------------------------------------------------------
// Echomail: WriteMessageExt with existing header attributes
// ---------------------------------------------------------------------------

func TestWriteMessageExtPreservesExistingAttributes(t *testing.T) {
	b := openExtTestBase(t)

	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Attr"
	msg.Text = "Body"
	msg.OrigAddr = "1:1/1"
	msg.Header = &MessageHeader{Attribute: MsgPrivate}

	b.WriteMessageExt(msg, MsgTypeEchomailMsg, "TEST", "BBS", "")

	got, _ := b.ReadMessage(1)
	if got.Header.Attribute&MsgPrivate == 0 {
		t.Error("should preserve MsgPrivate from existing header")
	}
	if got.Header.Attribute&MsgTypeEcho == 0 {
		t.Error("should also have MsgTypeEcho")
	}
}

// ---------------------------------------------------------------------------
// Flags subfield via WriteMessageExt
// ---------------------------------------------------------------------------

func TestReadMessageFlags(t *testing.T) {
	b := openExtTestBase(t)

	// Manually write a message with Flags subfield
	msg := NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "Flags"
	msg.Text = "Body"
	msg.Flags = "" // Flags is read-only from subfields; test via header subfield
	b.WriteMessage(msg)

	// Verify the message can be read without issues
	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.From != "User" {
		t.Errorf("From = %q", got.From)
	}
}

// ---------------------------------------------------------------------------
// DetermineMessageType additional cases
// ---------------------------------------------------------------------------

func TestDetermineMessageTypeWhitespace(t *testing.T) {
	mt := DetermineMessageType("  ECHO  ", "TEST")
	if mt != MsgTypeEchomailMsg {
		t.Errorf("trimmed 'ECHO' should be echomail, got %d", mt)
	}
}

// ---------------------------------------------------------------------------
// extractAddressFromOriginLine: LF vs CR vs CRLF
// ---------------------------------------------------------------------------

func TestExtractAddressFromOriginLineVariousLineEndings(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"Hello\n * Origin: BBS (1:1/1)\n", "1:1/1"},
		{"Hello\r\n * Origin: BBS (2:2/2)\r\n", "2:2/2"},
		{"Hello\r * Origin: BBS (3:3/3)\r", "3:3/3"},
	}
	for _, tt := range tests {
		got := extractAddressFromOriginLine(tt.text)
		if got != tt.want {
			t.Errorf("extractAddress(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// withFileLock: executes function and returns its error
// ---------------------------------------------------------------------------

func TestWithFileLock(t *testing.T) {
	b := openExtTestBase(t)

	called := false
	err := b.withFileLock(func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("withFileLock: %v", err)
	}
	if !called {
		t.Error("function was not called")
	}

	// Test error propagation
	expectedErr := fmt.Errorf("test error")
	err = b.withFileLock(func() error {
		return expectedErr
	})
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}
