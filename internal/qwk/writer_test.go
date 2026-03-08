package qwk

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestPacketWriter_BasicPacket(t *testing.T) {
	pw := NewPacketWriter("VISION3", "ViSiON/3 BBS", "SysOp")
	pw.SetPersonalTo("testuser")
	pw.AddConference(0, "Email")
	pw.AddConference(1, "General")

	pw.AddMessage(PacketMessage{
		Conference: 1,
		Number:     42,
		From:       "SysOp",
		To:         "TestUser",
		Subject:    "Hello World",
		DateTime:   time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		Body:       "This is a test message.\nSecond line here.",
		Private:    false,
	})

	var buf bytes.Buffer
	if err := pw.WritePacket(&buf); err != nil {
		t.Fatalf("WritePacket failed: %v", err)
	}

	// Verify it's a valid ZIP
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("failed to open ZIP: %v", err)
	}

	fileNames := make(map[string]bool)
	for _, f := range zr.File {
		fileNames[f.Name] = true
	}

	required := []string{"CONTROL.DAT", "DOOR.ID", "MESSAGES.DAT", "001.NDX", "PERSONAL.NDX"}
	for _, name := range required {
		if !fileNames[name] {
			t.Errorf("missing expected file: %s", name)
		}
	}
}

func TestPacketWriter_ControlDAT(t *testing.T) {
	pw := NewPacketWriter("TEST", "Test BBS", "Admin")
	pw.SetPersonalTo("joe")
	pw.AddConference(0, "Mail")
	pw.AddConference(1, "Chat")

	var buf bytes.Buffer
	if err := pw.WritePacket(&buf); err != nil {
		t.Fatalf("WritePacket failed: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("failed to open ZIP: %v", err)
	}

	for _, f := range zr.File {
		if f.Name == "CONTROL.DAT" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			var content bytes.Buffer
			if _, err := content.ReadFrom(rc); err != nil {
				rc.Close()
				t.Fatalf("ReadFrom CONTROL.DAT: %v", err)
			}
			rc.Close()

			lines := strings.Split(content.String(), "\r\n")
			if lines[0] != "Test BBS" {
				t.Errorf("line 0: want 'Test BBS', got %q", lines[0])
			}
			if lines[3] != "Admin" {
				t.Errorf("line 3: want 'Admin', got %q", lines[3])
			}
			if lines[6] != "joe" {
				t.Errorf("line 6: want 'joe', got %q", lines[6])
			}
			// Line 10: num conferences - 1
			if lines[10] != "1" {
				t.Errorf("line 10: want '1' (2 confs - 1), got %q", lines[10])
			}
			return
		}
	}
	t.Fatal("CONTROL.DAT not found in archive")
}

func TestFormatMessage_HeaderLayout(t *testing.T) {
	msg := PacketMessage{
		Conference: 1,
		Number:     42,
		From:       "SysOp",
		To:         "AllUsers",
		Subject:    "Test Subject",
		DateTime:   time.Date(2026, 3, 5, 14, 30, 0, 0, time.UTC),
		Body:       "Hello",
		Private:    false,
	}

	data := formatMessage(msg)

	if len(data) < BlockSize {
		t.Fatalf("message too short: %d bytes", len(data))
	}

	// Status byte
	if data[0] != ' ' {
		t.Errorf("status: want ' ', got %q", data[0])
	}

	// Message number (positions 1-7)
	numStr := strings.TrimSpace(string(data[1:8]))
	if numStr != "42" {
		t.Errorf("message number: want '42', got %q", numStr)
	}

	// Date (positions 8-15)
	dateStr := string(data[8:16])
	if dateStr != "03-05-26" {
		t.Errorf("date: want '03-05-26', got %q", dateStr)
	}

	// To (positions 21-45)
	toStr := strings.TrimSpace(string(data[21:46]))
	if toStr != "AllUsers" {
		t.Errorf("to: want 'AllUsers', got %q", toStr)
	}

	// From (positions 46-70)
	fromStr := strings.TrimSpace(string(data[46:71]))
	if fromStr != "SysOp" {
		t.Errorf("from: want 'SysOp', got %q", fromStr)
	}

	// Subject (positions 71-95)
	subjectStr := strings.TrimSpace(string(data[71:96]))
	if subjectStr != "Test Subject" {
		t.Errorf("subject: want 'Test Subject', got %q", subjectStr)
	}

	// Conference (positions 123-124)
	confNum := int(data[123]) | int(data[124])<<8
	if confNum != 1 {
		t.Errorf("conference: want 1, got %d", confNum)
	}

	// Active flag
	if data[122] != 0xE1 {
		t.Errorf("active flag: want 0xE1, got 0x%02X", data[122])
	}
}

func TestFormatMessage_BodyEncoding(t *testing.T) {
	msg := PacketMessage{
		Number:   1,
		Body:     "Line one\nLine two\nLine three",
		DateTime: time.Now(),
	}

	data := formatMessage(msg)
	body := data[BlockSize:]

	// Check that \n was converted to 0xE3
	if !bytes.Contains(body, []byte{0xE3}) {
		t.Error("body should contain QWK line endings (0xE3)")
	}
	if bytes.Contains(body, []byte{'\n'}) {
		t.Error("body should not contain raw newlines")
	}
}

func TestFormatMessage_Private(t *testing.T) {
	msg := PacketMessage{
		Number:   1,
		Private:  true,
		DateTime: time.Now(),
	}

	data := formatMessage(msg)
	if data[0] != '*' {
		t.Errorf("private status: want '*', got %q", data[0])
	}
}

func TestMakeNDXRecord(t *testing.T) {
	// QWK NDX requires Microsoft BASIC single-precision float (MSBIN4), not IEEE 754.
	// For offset=1 (1.0): IEEE exp=127 → MBF exp=129 (0x81), mantissa=0, sign=0
	// Expected bytes (little-endian): [0x00, 0x00, 0x00, 0x81, conf]
	rec := makeNDXRecord(1, 1)
	if len(rec) != 5 {
		t.Fatalf("NDX record length: want 5, got %d", len(rec))
	}
	if rec[0] != 0x00 || rec[1] != 0x00 || rec[2] != 0x00 || rec[3] != 0x81 {
		t.Errorf("NDX MBF4 bytes for offset=1: want [00 00 00 81], got [%02X %02X %02X %02X]",
			rec[0], rec[1], rec[2], rec[3])
	}
	if rec[4] != 1 {
		t.Errorf("NDX conference: want 1, got %d", rec[4])
	}

	// offset=2: IEEE exp=128 → MBF exp=130 (0x82), mantissa=0
	rec2 := makeNDXRecord(2, 3)
	if rec2[0] != 0x00 || rec2[1] != 0x00 || rec2[2] != 0x00 || rec2[3] != 0x82 {
		t.Errorf("NDX MBF4 bytes for offset=2: want [00 00 00 82], got [%02X %02X %02X %02X]",
			rec2[0], rec2[1], rec2[2], rec2[3])
	}
	if rec2[4] != 3 {
		t.Errorf("NDX conference: want 3, got %d", rec2[4])
	}

	// offset=0: all float bytes must be zero
	rec0 := makeNDXRecord(0, 0)
	for i := 0; i < 4; i++ {
		if rec0[i] != 0 {
			t.Errorf("NDX MBF4 bytes for offset=0: byte[%d] want 0, got 0x%02X", i, rec0[i])
		}
	}
}

func TestPacketWriter_NoMessages(t *testing.T) {
	pw := NewPacketWriter("TEST", "Test BBS", "Admin")
	pw.AddConference(0, "Mail")

	var buf bytes.Buffer
	if err := pw.WritePacket(&buf); err != nil {
		t.Fatalf("WritePacket failed: %v", err)
	}

	// Should still produce a valid ZIP with CONTROL.DAT and MESSAGES.DAT
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("failed to open ZIP: %v", err)
	}

	hasControl := false
	hasMessages := false
	for _, f := range zr.File {
		if f.Name == "CONTROL.DAT" {
			hasControl = true
		}
		if f.Name == "MESSAGES.DAT" {
			hasMessages = true
		}
	}
	if !hasControl {
		t.Error("missing CONTROL.DAT")
	}
	if !hasMessages {
		t.Error("missing MESSAGES.DAT")
	}
}

func TestPacketWriter_BBSID_Truncation(t *testing.T) {
	pw := NewPacketWriter("LONGERNAME123", "Test", "Admin")
	if pw.bbsID != "LONGERNA" {
		t.Errorf("bbsID truncation: want 'LONGERNA', got %q", pw.bbsID)
	}
}
