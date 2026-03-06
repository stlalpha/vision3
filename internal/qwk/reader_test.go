package qwk

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestReadREP_BasicMessage(t *testing.T) {
	// Build a REP packet: ZIP containing VISION3.MSG
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	// Build the .MSG content with spacer + one message
	var msgBuf bytes.Buffer

	// Spacer block
	spacer := make([]byte, BlockSize)
	for i := range spacer {
		spacer[i] = ' '
	}
	msgBuf.Write(spacer)

	// Build a test message
	msg := PacketMessage{
		Conference: 1,
		Number:     1,
		From:       "TestUser",
		To:         "SysOp",
		Subject:    "Reply Test",
		DateTime:   time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC),
		Body:       "This is my reply.",
		Private:    false,
	}
	msgData := formatMessage(msg)
	numBlocks := (len(msgData) + BlockSize - 1) / BlockSize
	padded := make([]byte, numBlocks*BlockSize)
	for i := range padded {
		padded[i] = ' '
	}
	copy(padded, msgData)
	msgBuf.Write(padded)

	w, err := zw.Create("VISION3.MSG")
	if err != nil {
		t.Fatal(err)
	}
	w.Write(msgBuf.Bytes())
	zw.Close()

	// Read the REP packet
	data := zipBuf.Bytes()
	messages, err := ReadREP(bytes.NewReader(data), int64(len(data)), "VISION3")
	if err != nil {
		t.Fatalf("ReadREP failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(messages))
	}

	m := messages[0]
	if m.Conference != 1 {
		t.Errorf("conference: want 1, got %d", m.Conference)
	}
	if m.To != "SysOp" {
		t.Errorf("to: want 'SysOp', got %q", m.To)
	}
	if m.Subject != "Reply Test" {
		t.Errorf("subject: want 'Reply Test', got %q", m.Subject)
	}
	if !strings.Contains(m.Body, "This is my reply") {
		t.Errorf("body should contain reply text, got %q", m.Body)
	}
}

func TestReadREP_MissingMSGFile(t *testing.T) {
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, _ := zw.Create("OTHER.TXT")
	w.Write([]byte("hello"))
	zw.Close()

	data := zipBuf.Bytes()
	_, err := ReadREP(bytes.NewReader(data), int64(len(data)), "VISION3")
	if err == nil {
		t.Fatal("expected error for missing .MSG file")
	}
}

func TestDecodeQWKBody(t *testing.T) {
	// 0xE3 is QWK line ending
	input := []byte("Line one\xe3Line two\xe3Line three   ")
	result := decodeQWKBody(input)
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %q", len(lines), result)
	}
	if lines[0] != "Line one" {
		t.Errorf("line 0: want 'Line one', got %q", lines[0])
	}
}
