package ftn

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestPacketRoundTrip(t *testing.T) {
	hdr := NewPacketHeader(1, 103, 705, 0, 1, 104, 56, 0, "secret")

	msgs := []*PackedMessage{
		{
			MsgType:  2,
			OrigNode: 705,
			DestNode: 56,
			OrigNet:  103,
			DestNet:  104,
			Attr:     MsgAttrLocal,
			DateTime: FormatFTNDateTime(time.Now()),
			To:       "All",
			From:     "Test User",
			Subject:  "Test Subject",
			Body:     "AREA:GENERAL\r\x01MSGID: 1:103/705 12345678\rHello World!\r--- Vision3\r * Origin: Test BBS (1:103/705)\rSEEN-BY: 103/705\r\x01PATH: 103/705\r",
		},
	}

	// Write
	var buf bytes.Buffer
	err := WritePacket(&buf, hdr, msgs)
	if err != nil {
		t.Fatalf("WritePacket: %v", err)
	}

	// Read back
	hdr2, msgs2, err := ReadPacket(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}

	// Verify header
	if hdr2.OrigNode != 705 || hdr2.DestNode != 56 {
		t.Errorf("header nodes: got %d->%d, want 705->56", hdr2.OrigNode, hdr2.DestNode)
	}
	if hdr2.OrigNet != 103 || hdr2.DestNet != 104 {
		t.Errorf("header nets: got %d->%d, want 103->104", hdr2.OrigNet, hdr2.DestNet)
	}
	if hdr2.OrigZone != 1 || hdr2.DestZone != 1 {
		t.Errorf("header zones: got %d->%d, want 1->1", hdr2.OrigZone, hdr2.DestZone)
	}
	if string(bytes.TrimRight(hdr2.Password[:], "\x00")) != "secret" {
		t.Errorf("password: got %q, want %q", hdr2.Password, "secret")
	}
	if hdr2.PktType != PacketType2Plus {
		t.Errorf("packet type: got %d, want %d", hdr2.PktType, PacketType2Plus)
	}

	// Verify messages
	if len(msgs2) != 1 {
		t.Fatalf("message count: got %d, want 1", len(msgs2))
	}

	msg := msgs2[0]
	if msg.To != "All" {
		t.Errorf("To: got %q, want %q", msg.To, "All")
	}
	if msg.From != "Test User" {
		t.Errorf("From: got %q, want %q", msg.From, "Test User")
	}
	if msg.Subject != "Test Subject" {
		t.Errorf("Subject: got %q, want %q", msg.Subject, "Test Subject")
	}
	if msg.Body != msgs[0].Body {
		t.Errorf("Body mismatch:\ngot:  %q\nwant: %q", msg.Body, msgs[0].Body)
	}
	if msg.Attr != MsgAttrLocal {
		t.Errorf("Attr: got 0x%04x, want 0x%04x", msg.Attr, MsgAttrLocal)
	}
}

func TestPacketMultipleMessages(t *testing.T) {
	hdr := NewPacketHeader(21, 3, 110, 0, 21, 1, 100, 0, "")

	msgs := []*PackedMessage{
		{MsgType: 2, OrigNode: 110, DestNode: 100, OrigNet: 3, DestNet: 1,
			DateTime: "09 Feb 26  12:00:00", To: "User1", From: "Sender1",
			Subject: "Msg 1", Body: "Body one\r"},
		{MsgType: 2, OrigNode: 110, DestNode: 100, OrigNet: 3, DestNet: 1,
			DateTime: "09 Feb 26  12:01:00", To: "User2", From: "Sender2",
			Subject: "Msg 2", Body: "Body two\r"},
		{MsgType: 2, OrigNode: 110, DestNode: 100, OrigNet: 3, DestNet: 1,
			DateTime: "09 Feb 26  12:02:00", To: "User3", From: "Sender3",
			Subject: "Msg 3", Body: "Body three\r"},
	}

	var buf bytes.Buffer
	if err := WritePacket(&buf, hdr, msgs); err != nil {
		t.Fatalf("WritePacket: %v", err)
	}

	_, msgs2, err := ReadPacket(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	if len(msgs2) != 3 {
		t.Fatalf("message count: got %d, want 3", len(msgs2))
	}
	for i, m := range msgs2 {
		if m.Subject != msgs[i].Subject {
			t.Errorf("msg[%d] subject: got %q, want %q", i, m.Subject, msgs[i].Subject)
		}
	}
}

func TestParsePackedMessageBody(t *testing.T) {
	body := "AREA:FSX_GEN\r\x01MSGID: 1:103/705 12345678\r\x01REPLY: 1:104/56 87654321\rHello World!\r\rThis is a test message.\r--- Vision3 0.1.0\r * Origin: Test BBS (1:103/705)\rSEEN-BY: 103/705 104/56\r\x01PATH: 103/705\r"

	parsed := ParsePackedMessageBody(body)

	if parsed.Area != "FSX_GEN" {
		t.Errorf("Area: got %q, want %q", parsed.Area, "FSX_GEN")
	}

	if len(parsed.Kludges) != 2 {
		t.Fatalf("Kludges count: got %d, want 2", len(parsed.Kludges))
	}
	if parsed.Kludges[0] != "MSGID: 1:103/705 12345678" {
		t.Errorf("Kludge[0]: got %q", parsed.Kludges[0])
	}
	if parsed.Kludges[1] != "REPLY: 1:104/56 87654321" {
		t.Errorf("Kludge[1]: got %q", parsed.Kludges[1])
	}

	if len(parsed.SeenBy) != 1 {
		t.Fatalf("SeenBy count: got %d, want 1", len(parsed.SeenBy))
	}
	if parsed.SeenBy[0] != "103/705 104/56" {
		t.Errorf("SeenBy[0]: got %q", parsed.SeenBy[0])
	}

	if len(parsed.Path) != 1 {
		t.Fatalf("Path count: got %d, want 1", len(parsed.Path))
	}
	if parsed.Path[0] != "103/705" {
		t.Errorf("Path[0]: got %q", parsed.Path[0])
	}

	// Text should not contain kludges, SEEN-BY, PATH, or AREA
	if strings.Contains(parsed.Text, "AREA:") {
		t.Error("Text should not contain AREA line")
	}
	if strings.Contains(parsed.Text, "\x01") {
		t.Error("Text should not contain kludge lines")
	}
	if strings.Contains(parsed.Text, "SEEN-BY") {
		t.Error("Text should not contain SEEN-BY lines")
	}
	if !strings.Contains(parsed.Text, "Hello World!") {
		t.Error("Text should contain message body")
	}
	if !strings.Contains(parsed.Text, "Origin: Test BBS") {
		t.Error("Text should contain origin line")
	}
}

func TestFormatPackedMessageBody(t *testing.T) {
	parsed := &ParsedBody{
		Area:    "FSX_GEN",
		Kludges: []string{"MSGID: 1:103/705 12345678"},
		Text:    "Hello World!\r--- Vision3\r * Origin: Test BBS (1:103/705)",
		SeenBy:  []string{"103/705"},
		Path:    []string{"103/705"},
	}

	body := FormatPackedMessageBody(parsed)

	if !strings.HasPrefix(body, "AREA:FSX_GEN\r") {
		t.Error("body should start with AREA tag")
	}
	if !strings.Contains(body, "\x01MSGID: 1:103/705 12345678\r") {
		t.Error("body should contain MSGID kludge with SOH prefix")
	}
	if !strings.Contains(body, "Hello World!") {
		t.Error("body should contain message text")
	}
	if !strings.Contains(body, "SEEN-BY: 103/705\r") {
		t.Error("body should contain SEEN-BY line")
	}
	if !strings.Contains(body, "\x01PATH: 103/705\r") {
		t.Error("body should contain PATH line with SOH prefix")
	}
}

func TestFormatAndParseFTNDateTime(t *testing.T) {
	now := time.Date(2026, 2, 9, 14, 30, 45, 0, time.UTC)
	formatted := FormatFTNDateTime(now)

	expected := "09 Feb 26  14:30:45"
	if formatted != expected {
		t.Errorf("FormatFTNDateTime: got %q, want %q", formatted, expected)
	}

	parsed, err := ParseFTNDateTime(formatted)
	if err != nil {
		t.Fatalf("ParseFTNDateTime: %v", err)
	}
	if parsed.Day() != 9 || parsed.Month() != 2 || parsed.Hour() != 14 || parsed.Minute() != 30 {
		t.Errorf("Parsed time mismatch: got %v", parsed)
	}
}

func TestEmptyPacket(t *testing.T) {
	hdr := NewPacketHeader(1, 100, 1, 0, 1, 200, 2, 0, "")

	var buf bytes.Buffer
	if err := WritePacket(&buf, hdr, nil); err != nil {
		t.Fatalf("WritePacket empty: %v", err)
	}

	hdr2, msgs, err := ReadPacket(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ReadPacket empty: %v", err)
	}
	if hdr2.OrigNode != 1 || hdr2.DestNode != 2 {
		t.Error("header mismatch on empty packet")
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestTruncatedPacket(t *testing.T) {
	// Too short for header
	_, _, err := ReadPacket(bytes.NewReader([]byte{0, 1, 2}))
	if err == nil {
		t.Error("expected error for truncated packet")
	}
}

func TestParseLocalMessageBody(t *testing.T) {
	// Local message - no AREA, no SEEN-BY/PATH
	body := "Hello World!\rThis is a local message.\r"

	parsed := ParsePackedMessageBody(body)

	if parsed.Area != "" {
		t.Errorf("Area should be empty for local, got %q", parsed.Area)
	}
	if len(parsed.Kludges) != 0 {
		t.Errorf("expected 0 kludges, got %d", len(parsed.Kludges))
	}
	if len(parsed.SeenBy) != 0 {
		t.Errorf("expected 0 SEEN-BY, got %d", len(parsed.SeenBy))
	}
	if !strings.Contains(parsed.Text, "Hello World!") {
		t.Error("Text should contain message body")
	}
}
