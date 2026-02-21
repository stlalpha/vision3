package tosser

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stlalpha/vision3/internal/ftn"
	"github.com/stlalpha/vision3/internal/jam"
	"github.com/stlalpha/vision3/internal/message"
)

// testEnv sets up a minimal temp environment for tosser integration tests.
type testEnv struct {
	dir         string
	configDir   string
	dataDir     string
	inboundDir  string
	outboundDir string
	binkdDir    string
	tempDir     string
	dupeDBPath  string
	msgMgr      *message.MessageManager
	dupeDB      *DupeDB
	netCfg      networkConfig
}

// testArea is a minimal message_areas.json entry.
type testArea struct {
	ID       int    `json:"id"`
	Tag      string `json:"tag"`
	Name     string `json:"name"`
	BasePath string `json:"base_path"`
	AreaType string `json:"area_type"`
	EchoTag  string `json:"echo_tag"`
	Network  string `json:"network"`
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	dir := t.TempDir()
	configDir := filepath.Join(dir, "configs")
	dataDir := filepath.Join(dir, "data")
	inboundDir := filepath.Join(dir, "data", "ftn", "in")
	outboundDir := filepath.Join(dir, "data", "ftn", "temp_out")
	binkdDir := filepath.Join(dir, "data", "ftn", "out")
	tempDir := filepath.Join(dir, "data", "ftn", "temp_in")
	msgbasesDir := filepath.Join(dataDir, "msgbases")

	for _, d := range []string{configDir, inboundDir, outboundDir, binkdDir, tempDir, msgbasesDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Write minimal message_areas.json
	areas := []testArea{
		{
			ID:       1,
			Tag:      "FSX_TEST",
			Name:     "FSXNet Test",
			BasePath: "msgbases/fsx_test",
			AreaType: "echomail",
			EchoTag:  "FSX_TEST",
			Network:  "testnet",
		},
	}
	areasData, _ := json.Marshal(areas)
	os.WriteFile(filepath.Join(configDir, "message_areas.json"), areasData, 0644)

	// Create the JAM base directory
	os.MkdirAll(filepath.Join(dataDir, "msgbases", "fsx_test"), 0755)

	msgMgr, err := message.NewMessageManager(dataDir, configDir, "TestBBS", nil)
	if err != nil {
		t.Fatalf("NewMessageManager: %v", err)
	}

	dupeDBPath := filepath.Join(dir, "data", "ftn", "dupes.json")
	dupeDB, err := NewDupeDB(dupeDBPath, 30*24*3600e9)
	if err != nil {
		t.Fatal(err)
	}

	netCfg := networkConfig{
		InternalTosserEnabled: true,
		OwnAddress:            "21:4/158.1",
		InboundPath:           inboundDir,
		SecureInboundPath:     "",
		OutboundPath:          outboundDir,
		BinkdOutboundPath:     binkdDir,
		TempPath:              tempDir,
		Links: []linkConfig{
			{
				Address:   "21:4/158",
				Password:  "",
				Name:      "Test Hub",
				EchoAreas: []string{"FSX_TEST"},
			},
		},
	}

	return &testEnv{
		dir:         dir,
		configDir:   configDir,
		dataDir:     dataDir,
		inboundDir:  inboundDir,
		outboundDir: outboundDir,
		binkdDir:    binkdDir,
		tempDir:     tempDir,
		dupeDBPath:  dupeDBPath,
		msgMgr:      msgMgr,
		dupeDB:      dupeDB,
		netCfg:      netCfg,
	}
}

// TestTossDirectPkt tests tossing a raw .PKT file into a JAM base.
func TestTossDirectPkt(t *testing.T) {
	env := setupTestEnv(t)
	tosser, err := New("testnet", env.netCfg, env.dupeDB, env.msgMgr)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Write a .PKT into the inbound dir
	pktData := makePktSimple(t, "FSX_TEST", "Sender", "All", "Test Subject", "Hello from test!\r", "21:4/100 DEADBEEF")
	pktPath := filepath.Join(env.inboundDir, "test0001.pkt")
	os.WriteFile(pktPath, pktData, 0644)

	result := tosser.ProcessInbound()

	if result.MessagesImported != 1 {
		t.Errorf("expected 1 imported, got %d (errors: %v)", result.MessagesImported, result.Errors)
	}
	if result.DupesSkipped != 0 {
		t.Errorf("expected 0 dupes, got %d", result.DupesSkipped)
	}
	if len(result.Errors) != 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}

	// .PKT should be removed after successful toss
	if _, err := os.Stat(pktPath); !os.IsNotExist(err) {
		t.Error("processed .PKT should be removed from inbound")
	}

	// Verify message landed in the JAM base
	base, err := env.msgMgr.GetBase(1)
	if err != nil {
		t.Fatalf("GetBase: %v", err)
	}
	defer base.Close()

	count, err := base.GetMessageCount()
	if err != nil {
		t.Fatalf("GetMessageCount: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 message in base, got %d", count)
	}

	msg, err := base.ReadMessage(1)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if msg.From != "Sender" {
		t.Errorf("From: got %q, want %q", msg.From, "Sender")
	}
	if msg.Subject != "Test Subject" {
		t.Errorf("Subject: got %q, want %q", msg.Subject, "Test Subject")
	}
}

// TestTossDuplicateDetection verifies that the same MSGID is not imported twice.
func TestTossDuplicateDetection(t *testing.T) {
	env := setupTestEnv(t)
	tosser, err := New("testnet", env.netCfg, env.dupeDB, env.msgMgr)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	msgID := "21:4/100 CAFEBABE"
	pktData := makePktSimple(t, "FSX_TEST", "Sender", "All", "Dupe Test", "Body\r", msgID)

	// First toss
	pkt1 := filepath.Join(env.inboundDir, "first.pkt")
	os.WriteFile(pkt1, pktData, 0644)
	r1 := tosser.ProcessInbound()
	if r1.MessagesImported != 1 {
		t.Errorf("first toss: expected 1 imported, got %d", r1.MessagesImported)
	}

	// Second toss of the same packet
	pkt2 := filepath.Join(env.inboundDir, "second.pkt")
	os.WriteFile(pkt2, pktData, 0644)
	r2 := tosser.ProcessInbound()
	if r2.DupesSkipped != 1 {
		t.Errorf("second toss: expected 1 dupe, got %d", r2.DupesSkipped)
	}
	if r2.MessagesImported != 0 {
		t.Errorf("second toss: expected 0 imported, got %d", r2.MessagesImported)
	}
}

// TestTossBundlePkt tests that a ZIP bundle is unpacked and tossed correctly.
func TestTossBundlePkt(t *testing.T) {
	env := setupTestEnv(t)
	tosser, err := New("testnet", env.netCfg, env.dupeDB, env.msgMgr)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Create a .PKT and wrap it in a ZIP bundle
	pktData := makePktSimple(t, "FSX_TEST", "BundleSender", "All", "Bundle Subject", "From a bundle!\r", "21:4/100 BABE1234")
	bundlePath := filepath.Join(env.inboundDir, "0004009e.mo0")
	f, err := os.Create(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("bundled.pkt")
	w.Write(pktData)
	zw.Close()
	f.Close()

	result := tosser.ProcessInbound()

	if result.MessagesImported != 1 {
		t.Errorf("bundle toss: expected 1 imported, got %d (errors: %v)", result.MessagesImported, result.Errors)
	}

	// Bundle should be removed after successful toss
	if _, err := os.Stat(bundlePath); !os.IsNotExist(err) {
		t.Error("processed bundle should be removed from inbound")
	}
}

// TestScanExportCreatesPacket verifies that messages with DateProcessed=0 are exported.
func TestScanExportCreatesPacket(t *testing.T) {
	env := setupTestEnv(t)

	// Seed the JAM base with an unprocessed echomail message (DateProcessed=0)
	base, err := env.msgMgr.GetBase(1)
	if err != nil {
		t.Fatalf("GetBase: %v", err)
	}

	msg := jam.NewMessage()
	msg.From = "Local User"
	msg.To = "All"
	msg.Subject = "Outgoing Message"
	msg.Text = "This should be exported.\r"
	msg.MsgID = "21:4/158.1 AABBCCDD"

	// Use the echomail write path so DateProcessed=0 (pending export)
	area, _ := env.msgMgr.GetAreaByTag("FSX_TEST")
	msgType := jam.DetermineMessageType(area.AreaType, area.EchoTag)
	_, err = base.WriteMessageExt(msg, msgType, area.EchoTag, "TestBBS", "")
	if err != nil {
		t.Fatalf("WriteMessageExt: %v", err)
	}
	base.Close()

	tosser, err := New("testnet", env.netCfg, env.dupeDB, env.msgMgr)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result := tosser.ScanAndExport()

	if result.MessagesExported != 1 {
		t.Errorf("expected 1 exported, got %d (errors: %v)", result.MessagesExported, result.Errors)
	}

	// Verify a .PKT file was created in the staging directory
	entries, _ := os.ReadDir(env.outboundDir)
	var pktCount int
	for _, e := range entries {
		if strings.ToLower(filepath.Ext(e.Name())) == ".pkt" {
			pktCount++
		}
	}
	if pktCount != 1 {
		t.Errorf("expected 1 .PKT in outbound, got %d", pktCount)
	}
}

// TestHighWaterMarkAdvances verifies the HWM is updated after a successful scan.
func TestHighWaterMarkAdvances(t *testing.T) {
	env := setupTestEnv(t)

	// Write a message to the JAM base
	base, _ := env.msgMgr.GetBase(1)
	msg := jam.NewMessage()
	msg.From = "User"
	msg.To = "All"
	msg.Subject = "HWM Test"
	msg.Text = "Testing HWM\r"
	msg.MsgID = "21:4/158.1 11223344"

	area, _ := env.msgMgr.GetAreaByTag("FSX_TEST")
	msgType := jam.DetermineMessageType(area.AreaType, area.EchoTag)
	base.WriteMessageExt(msg, msgType, area.EchoTag, "TestBBS", "")
	base.Close()

	tosser, _ := New("testnet", env.netCfg, env.dupeDB, env.msgMgr)
	tosser.ScanAndExport()

	// HWM should be persisted in the base's .jlr file
	base2, err := env.msgMgr.GetBase(1)
	if err != nil {
		t.Fatalf("GetBase after scan: %v", err)
	}
	defer base2.Close()

	lr, err := base2.GetLastRead(ScannerUser)
	if err != nil {
		t.Fatalf("GetLastRead(%s): %v", ScannerUser, err)
	}

	// After the scan, the HWM for area 1 should be >= 1
	if lr.LastReadMsg < 1 {
		t.Errorf("HWM for area 1 should be >= 1 after scan, got %d", lr.LastReadMsg)
	}
}

// TestPackOutboundCreatesBundle verifies that staged .PKT files are bundled.
func TestPackOutboundCreatesBundle(t *testing.T) {
	env := setupTestEnv(t)

	// Put a proper .PKT file in the staging/outbound dir (destNet=4, destNode=158 matches test link)
	pktData := makeStagedPkt(t)
	pktPath := filepath.Join(env.outboundDir, "staged.pkt")
	os.WriteFile(pktPath, pktData, 0644)

	tosser, _ := New("testnet", env.netCfg, env.dupeDB, env.msgMgr)
	result := tosser.PackOutbound()

	if result.BundlesCreated != 1 {
		t.Errorf("expected 1 bundle, got %d (errors: %v)", result.BundlesCreated, result.Errors)
	}
	if result.PacketsPacked != 1 {
		t.Errorf("expected 1 packet packed, got %d", result.PacketsPacked)
	}

	// Staged .PKT should be removed
	if _, err := os.Stat(pktPath); !os.IsNotExist(err) {
		t.Error("staged .PKT should be removed after packing")
	}

	// Bundle should exist in binkd outbound dir
	entries, _ := os.ReadDir(env.binkdDir)
	if len(entries) != 1 {
		t.Errorf("expected 1 bundle in binkd outbound, got %d", len(entries))
	}

	// Verify it's a valid ZIP
	if len(entries) > 0 {
		bundlePath := filepath.Join(env.binkdDir, entries[0].Name())
		isZip, err := ftn.IsZIPBundle(bundlePath)
		if err != nil || !isZip {
			t.Error("created bundle should be a valid ZIP file")
		}
	}
}

// setupExtendedTestEnv sets up a test environment with NETMAIL, BAD, and DUPE areas.
func setupExtendedTestEnv(t *testing.T) (*testEnv, networkConfig) {
	t.Helper()
	env := setupTestEnv(t)

	// Add NETMAIL, BAD, DUPE areas to message_areas.json
	areas := []testArea{
		{ID: 1, Tag: "FSX_TEST", Name: "FSXNet Test", BasePath: "msgbases/fsx_test",
			AreaType: "echomail", EchoTag: "FSX_TEST", Network: "testnet"},
		{ID: 2, Tag: "NETMAIL", Name: "Netmail", BasePath: "msgbases/netmail",
			AreaType: "netmail", EchoTag: "NETMAIL", Network: "testnet"},
		{ID: 3, Tag: "BAD", Name: "Bad Messages", BasePath: "msgbases/bad",
			AreaType: "echomail", EchoTag: "BAD", Network: "testnet"},
		{ID: 4, Tag: "DUPE", Name: "Duplicate Messages", BasePath: "msgbases/dupe",
			AreaType: "echomail", EchoTag: "DUPE", Network: "testnet"},
	}
	areasData, _ := json.Marshal(areas)
	os.WriteFile(filepath.Join(env.configDir, "message_areas.json"), areasData, 0644)
	for _, sub := range []string{"netmail", "bad", "dupe"} {
		os.MkdirAll(filepath.Join(env.dataDir, "msgbases", sub), 0755)
	}

	// Re-create the MessageManager to pick up the updated areas file
	var err error
	env.msgMgr, err = message.NewMessageManager(env.dataDir, env.configDir, "TestBBS", nil)
	if err != nil {
		t.Fatalf("NewMessageManager (extended): %v", err)
	}

	extCfg := networkConfig{
		InternalTosserEnabled: true,
		OwnAddress:            "21:4/158.1",
		InboundPath:           env.inboundDir,
		OutboundPath:          env.outboundDir,
		BinkdOutboundPath:     env.binkdDir,
		TempPath:              env.tempDir,
		NetmailAreaTag:        "NETMAIL",
		BadAreaTag:            "BAD",
		DupeAreaTag:           "DUPE",
		Links: []linkConfig{
			{Address: "21:4/158", Password: "", Name: "Test Hub", EchoAreas: []string{"FSX_TEST"}},
		},
	}
	return env, extCfg
}

// TestTossNetmail verifies that a message without AREA kludge is routed to the netmail area.
func TestTossNetmail(t *testing.T) {
	env, extCfg := setupExtendedTestEnv(t)
	tosser, err := New("testnet", extCfg, env.dupeDB, env.msgMgr)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Netmail has no AREA tag — pass empty string to makePktSimple and it will omit AREA
	pktData := makePktSimpleNetmail(t, "Remote User", "Local Sysop", "Private Note", "Secret content\r", "21:4/100 FEEDFACE")
	pktPath := filepath.Join(env.inboundDir, "netmail.pkt")
	os.WriteFile(pktPath, pktData, 0644)

	result := tosser.ProcessInbound()

	if result.MessagesImported != 1 {
		t.Errorf("expected 1 imported, got %d (errors: %v)", result.MessagesImported, result.Errors)
	}

	base, err := env.msgMgr.GetBase(2) // NETMAIL is area ID 2
	if err != nil {
		t.Fatalf("GetBase(NETMAIL): %v", err)
	}
	defer base.Close()

	count, _ := base.GetMessageCount()
	if count != 1 {
		t.Errorf("expected 1 message in NETMAIL base, got %d", count)
	}
	msg, _ := base.ReadMessage(1)
	if msg.From != "Remote User" {
		t.Errorf("netmail From: got %q, want %q", msg.From, "Remote User")
	}
}

// TestTossBadArea verifies that a message for an unknown area is routed to the bad area.
func TestTossBadArea(t *testing.T) {
	env, extCfg := setupExtendedTestEnv(t)
	tosser, err := New("testnet", extCfg, env.dupeDB, env.msgMgr)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	pktData := makePktSimple(t, "UNKNOWN_AREA", "Sender", "All", "Bad Subject", "Goes to bad area\r", "21:4/100 BADBEEF1")
	pktPath := filepath.Join(env.inboundDir, "badarea.pkt")
	os.WriteFile(pktPath, pktData, 0644)

	result := tosser.ProcessInbound()

	// Bad area routing counts as imported (message was preserved)
	if result.MessagesImported != 1 {
		t.Errorf("expected 1 imported (via bad area), got %d (errors: %v)", result.MessagesImported, result.Errors)
	}

	base, err := env.msgMgr.GetBase(3) // BAD is area ID 3
	if err != nil {
		t.Fatalf("GetBase(BAD): %v", err)
	}
	defer base.Close()

	count, _ := base.GetMessageCount()
	if count != 1 {
		t.Errorf("expected 1 message in BAD base, got %d", count)
	}
}

// TestTossDupeArea verifies that a duplicate MSGID is routed to the dupe area.
func TestTossDupeArea(t *testing.T) {
	env, extCfg := setupExtendedTestEnv(t)
	tosser, err := New("testnet", extCfg, env.dupeDB, env.msgMgr)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	msgID := "21:4/100 DUPE0001"
	pktData := makePktSimple(t, "FSX_TEST", "Sender", "All", "Dupe Test", "First copy\r", msgID)

	// First toss — should land in FSX_TEST
	pkt1 := filepath.Join(env.inboundDir, "dupe_first.pkt")
	os.WriteFile(pkt1, pktData, 0644)
	r1 := tosser.ProcessInbound()
	if r1.MessagesImported != 1 {
		t.Errorf("first toss: expected 1 imported, got %d", r1.MessagesImported)
	}

	// Second toss — same MSGID, should go to DUPE
	pkt2 := filepath.Join(env.inboundDir, "dupe_second.pkt")
	os.WriteFile(pkt2, pktData, 0644)
	r2 := tosser.ProcessInbound()
	if r2.DupesSkipped != 1 {
		t.Errorf("second toss: expected 1 dupe, got %d", r2.DupesSkipped)
	}

	base, err := env.msgMgr.GetBase(4) // DUPE is area ID 4
	if err != nil {
		t.Fatalf("GetBase(DUPE): %v", err)
	}
	defer base.Close()

	count, _ := base.GetMessageCount()
	if count != 1 {
		t.Errorf("expected 1 message in DUPE base, got %d", count)
	}
}

// TestPackFlowFileCreated verifies that a .clo flow file is written when link flavour is Crash.
func TestPackFlowFileCreated(t *testing.T) {
	env := setupTestEnv(t)

	// Configure link with Crash flavour
	env.netCfg.Links = []linkConfig{
		{Address: "21:4/158", Password: "", Name: "Test Hub",
			EchoAreas: []string{"FSX_TEST"}, Flavour: "Crash"},
	}

	// Put a proper .PKT in the outbound dir (destNet=4, destNode=158 matches test link)
	os.WriteFile(filepath.Join(env.outboundDir, "staged.pkt"), makeStagedPkt(t), 0644)

	tosser, _ := New("testnet", env.netCfg, env.dupeDB, env.msgMgr)
	result := tosser.PackOutbound()

	if result.BundlesCreated != 1 {
		t.Fatalf("expected 1 bundle, got %d (errors: %v)", result.BundlesCreated, result.Errors)
	}

	// Flow file: net=4 (0x0004), node=158 (0x009e) → 0004009e.clo
	flowPath := filepath.Join(env.binkdDir, "0004009e.clo")
	data, err := os.ReadFile(flowPath)
	if err != nil {
		t.Fatalf(".clo flow file not created: %v", err)
	}
	content := string(data)
	line := strings.TrimSpace(content)
	if !strings.HasPrefix(line, "^") {
		t.Errorf(".clo line should start with '^', got: %q", content)
	}
	// The referenced bundle file should actually exist in the binkd dir
	bundlePath := strings.TrimPrefix(line, "^")
	if _, err := os.Stat(bundlePath); err != nil {
		t.Errorf(".clo references non-existent bundle %q: %v", bundlePath, err)
	}
}

// makePktSimpleNetmail creates a netmail packet (no AREA tag).
func makePktSimpleNetmail(t *testing.T, from, to, subject, body, msgID string) []byte {
	t.Helper()

	hdr := ftn.NewPacketHeader(21, 4, 100, 0, 21, 4, 158, 1, "")

	parsedBody := &ftn.ParsedBody{
		Area:    "", // No AREA for netmail
		Text:    body,
		Kludges: []string{"MSGID: " + msgID},
	}

	packed := &ftn.PackedMessage{
		MsgType:  2,
		OrigNode: 100,
		DestNode: 158,
		OrigNet:  4,
		DestNet:  4,
		Attr:     0x0001, // Private flag for netmail
		DateTime: "21 Feb 26  12:00:00",
		To:       to,
		From:     from,
		Subject:  subject,
		Body:     ftn.FormatPackedMessageBody(parsedBody),
	}

	var buf bytes.Buffer
	if err := ftn.WritePacket(&buf, hdr, []*ftn.PackedMessage{packed}); err != nil {
		t.Fatalf("WritePacket: %v", err)
	}
	return buf.Bytes()
}

// makePktSimple creates a test FTN packet (Type-2+) with one message.
func makePktSimple(t *testing.T, areaTag, from, to, subject, body, msgID string) []byte {
	t.Helper()

	hdr := ftn.NewPacketHeader(21, 4, 100, 0, 21, 4, 158, 1, "")

	parsedBody := &ftn.ParsedBody{
		Area:    areaTag,
		Text:    body,
		Kludges: []string{"MSGID: " + msgID},
		SeenBy:  []string{"4/100"},
		Path:    []string{"4/100"},
	}

	packed := &ftn.PackedMessage{
		MsgType:  2,
		OrigNode: 100,
		DestNode: 158,
		OrigNet:  4,
		DestNet:  4,
		Attr:     0,
		DateTime: "21 Feb 26  12:00:00",
		To:       to,
		From:     from,
		Subject:  subject,
		Body:     ftn.FormatPackedMessageBody(parsedBody),
	}

	var buf bytes.Buffer
	if err := ftn.WritePacket(&buf, hdr, []*ftn.PackedMessage{packed}); err != nil {
		t.Fatalf("WritePacket: %v", err)
	}
	return buf.Bytes()
}

// makeStagedPkt creates a minimal valid outbound FTN packet from our node (21:4/158.1)
// to the test hub (21:4/158), matching the test link configuration.
func makeStagedPkt(t *testing.T) []byte {
	t.Helper()

	// orig=21:4/158.1 (own address), dest=21:4/158 (hub link)
	hdr := ftn.NewPacketHeader(21, 4, 158, 1, 21, 4, 158, 0, "")

	packed := &ftn.PackedMessage{
		MsgType:  2,
		OrigNode: 158,
		DestNode: 158,
		OrigNet:  4,
		DestNet:  4,
		Attr:     0,
		DateTime: "21 Feb 26  12:00:00",
		To:       "All",
		From:     "Test Sender",
		Subject:  "Test outbound",
		Body:     "\x01AREA: FSX_TEST\r\nTest message body\r",
	}

	var buf bytes.Buffer
	if err := ftn.WritePacket(&buf, hdr, []*ftn.PackedMessage{packed}); err != nil {
		t.Fatalf("WritePacket: %v", err)
	}
	return buf.Bytes()
}
