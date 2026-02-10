package jam

import (
	"strings"
	"testing"
)

func TestWriteMessageExtEchomail(t *testing.T) {
	b := openTestBase(t)

	msg := NewMessage()
	msg.From = "Sysop"
	msg.To = "All"
	msg.Subject = "Echo Test"
	msg.Text = "Testing echomail support"
	msg.OrigAddr = "21:3/110"

	msgNum, err := b.WriteMessageExt(msg, MsgTypeEchomailMsg, "FSX_GEN", "Test BBS", "")
	if err != nil {
		t.Fatalf("WriteMessageExt: %v", err)
	}
	if msgNum != 1 {
		t.Errorf("msgNum = %d, want 1", msgNum)
	}

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	// Verify MSGID was generated
	if got.MsgID == "" {
		t.Error("MSGID should be generated for echomail")
	}
	if !strings.HasPrefix(got.MsgID, "21:3/110 ") {
		t.Errorf("MSGID = %q, should start with origin address", got.MsgID)
	}

	// Verify PID
	if got.PID == "" {
		t.Error("PID should be set for echomail")
	}
	if !strings.HasPrefix(got.PID, "Vision3") {
		t.Errorf("PID = %q, should start with Vision3", got.PID)
	}

	// Verify kludges contain AREA and TID
	var hasArea, hasTID bool
	for _, k := range got.Kludges {
		if k == "AREA:FSX_GEN" {
			hasArea = true
		}
		if strings.HasPrefix(k, "TID: ") {
			hasTID = true
		}
	}
	if !hasArea {
		t.Error("missing AREA kludge")
	}
	if !hasTID {
		t.Error("missing TID kludge")
	}

	// Verify tearline and origin in text
	if !strings.Contains(got.Text, "--- Vision3") {
		t.Error("text should contain tearline")
	}
	if !strings.Contains(got.Text, "* Origin: Test BBS (21:3/110)") {
		t.Error("text should contain origin line")
	}

	// Verify echomail attribute
	if got.Header.Attribute&MsgTypeEcho == 0 {
		t.Error("message should have MsgTypeEcho attribute")
	}

	// Verify DateProcessed is 0 (signals tosser to export)
	if got.Header.DateProcessed != 0 {
		t.Errorf("DateProcessed = %d, want 0 for echomail", got.Header.DateProcessed)
	}

	// Verify OrigAddr subfield
	if got.OrigAddr != "21:3/110" {
		t.Errorf("OrigAddr = %q, want %q", got.OrigAddr, "21:3/110")
	}
}

func TestWriteMessageExtLocal(t *testing.T) {
	b := openTestBase(t)

	msg := NewMessage()
	msg.From = "User1"
	msg.To = "User2"
	msg.Subject = "Local"
	msg.Text = "Local message"

	msgNum, err := b.WriteMessageExt(msg, MsgTypeLocalMsg, "", "Test BBS", "")
	if err != nil {
		t.Fatalf("WriteMessageExt local: %v", err)
	}
	if msgNum != 1 {
		t.Errorf("msgNum = %d, want 1", msgNum)
	}

	got, err := b.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	// Local messages should NOT have echomail elements
	if strings.Contains(got.Text, "--- Vision3") {
		t.Error("local message should not have tearline")
	}
	if strings.Contains(got.Text, "* Origin:") {
		t.Error("local message should not have origin line")
	}
	if got.Header.Attribute&MsgTypeEcho != 0 {
		t.Error("local message should not have echo attribute")
	}
	if got.Header.Attribute&MsgTypeLocal == 0 {
		t.Error("local message should have local attribute")
	}

	// DateProcessed should be non-zero for local messages
	if got.Header.DateProcessed == 0 {
		t.Error("DateProcessed should be set for local messages")
	}
}

func TestWriteMessageExtWithReply(t *testing.T) {
	b := openTestBase(t)

	// Write original
	orig := NewMessage()
	orig.From = "Alice"
	orig.To = "All"
	orig.Subject = "Original"
	orig.Text = "Original message"
	orig.OrigAddr = "1:103/705"
	b.WriteMessageExt(orig, MsgTypeEchomailMsg, "TEST", "BBS", "")

	// Write reply
	reply := NewMessage()
	reply.From = "Bob"
	reply.To = "Alice"
	reply.Subject = "Re: Original"
	reply.Text = "Reply text"
	reply.OrigAddr = "1:103/705"
	reply.ReplyID = orig.MsgID // MsgID was set by WriteMessageExt

	_, err := b.WriteMessageExt(reply, MsgTypeEchomailMsg, "TEST", "BBS", "")
	if err != nil {
		t.Fatalf("WriteMessageExt reply: %v", err)
	}

	got, _ := b.ReadMessage(2)
	if got.ReplyID != orig.MsgID {
		t.Errorf("ReplyID = %q, want %q", got.ReplyID, orig.MsgID)
	}
}

func TestMSGIDUniqueness(t *testing.T) {
	b := openTestBase(t)

	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		msg := NewMessage()
		msg.From = "User"
		msg.To = "All"
		msg.Subject = "Test"
		msg.Text = "Body"
		msg.OrigAddr = "1:103/705"
		b.WriteMessageExt(msg, MsgTypeEchomailMsg, "TEST", "BBS", "")

		if seen[msg.MsgID] {
			t.Fatalf("duplicate MSGID at iteration %d: %s", i, msg.MsgID)
		}
		seen[msg.MsgID] = true
	}
}

func TestDetermineMessageType(t *testing.T) {
	tests := []struct {
		areaType string
		echoTag  string
		want     MessageType
	}{
		{"echo", "FSX_GEN", MsgTypeEchomailMsg},
		{"echomail", "FSX_GEN", MsgTypeEchomailMsg},
		{"ECHO", "TEST", MsgTypeEchomailMsg},
		{"netmail", "", MsgTypeNetmailMsg},
		{"direct", "", MsgTypeNetmailMsg},
		{"local", "", MsgTypeLocalMsg},
		{"", "", MsgTypeLocalMsg},
		{"unknown", "", MsgTypeLocalMsg},
	}

	for _, tt := range tests {
		got := DetermineMessageType(tt.areaType, tt.echoTag)
		if got != tt.want {
			t.Errorf("DetermineMessageType(%q, %q) = %d, want %d", tt.areaType, tt.echoTag, got, tt.want)
		}
	}
}
