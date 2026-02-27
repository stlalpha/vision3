package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDoors_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	doors := []DoorConfig{
		{Name: "LORD", Command: "/usr/bin/lord", Args: []string{"-n", "{NODE}"}},
		{Name: "BRE", Command: "/usr/bin/bre"},
	}
	data, _ := json.Marshal(doors)
	os.WriteFile(filepath.Join(tmpDir, "doors.json"), data, 0644)

	result, err := LoadDoors(filepath.Join(tmpDir, "doors.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 doors, got %d", len(result))
	}
	if result["LORD"].Command != "/usr/bin/lord" {
		t.Errorf("expected LORD command /usr/bin/lord, got %s", result["LORD"].Command)
	}
}

func TestLoadDoors_MissingFile(t *testing.T) {
	result, err := LoadDoors("/nonexistent/doors.json")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map for missing file, got %d entries", len(result))
	}
}

func TestLoadDoors_DuplicateNames(t *testing.T) {
	tmpDir := t.TempDir()
	doors := []DoorConfig{
		{Name: "LORD", Command: "/usr/bin/lord"},
		{Name: "LORD", Command: "/usr/bin/lord2"},
	}
	data, _ := json.Marshal(doors)
	os.WriteFile(filepath.Join(tmpDir, "doors.json"), data, 0644)

	_, err := LoadDoors(filepath.Join(tmpDir, "doors.json"))
	if err == nil {
		t.Error("expected error for duplicate door names")
	}
}

func TestLoadDoors_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "doors.json"), []byte("not json"), 0644)

	_, err := LoadDoors(filepath.Join(tmpDir, "doors.json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadLoginSequence_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	items := []LoginItem{
		{Command: "LASTCALLS"},
		{Command: "displayfile", Data: "welcome.ans", ClearScreen: true},
	}
	data, _ := json.Marshal(items)
	os.WriteFile(filepath.Join(tmpDir, "login.json"), data, 0644)

	result, err := LoadLoginSequence(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	// Commands should be normalized to uppercase
	if result[1].Command != "DISPLAYFILE" {
		t.Errorf("expected DISPLAYFILE, got %s", result[1].Command)
	}
	if !result[1].ClearScreen {
		t.Error("expected ClearScreen to be true")
	}
}

func TestLoadLoginSequence_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := LoadLoginSequence(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return default sequence
	if len(result) != 3 {
		t.Fatalf("expected default 3-item sequence, got %d", len(result))
	}
	if result[0].Command != "LASTCALLS" {
		t.Errorf("expected LASTCALLS as first default item, got %s", result[0].Command)
	}
}

func TestLoadServerConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	// No config.json — should return defaults
	result, err := LoadServerConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SSHPort != 2222 {
		t.Errorf("expected default SSH port 2222, got %d", result.SSHPort)
	}
	if result.MaxNodes != 10 {
		t.Errorf("expected default MaxNodes 10, got %d", result.MaxNodes)
	}
	if result.BoardName != "ViSiON/3 BBS" {
		t.Errorf("expected default board name, got %s", result.BoardName)
	}
}

func TestLoadServerConfig_CustomValues(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := map[string]interface{}{
		"boardName": "Test BBS",
		"sshPort":   3333,
		"maxNodes":  50,
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(tmpDir, "config.json"), data, 0644)

	result, err := LoadServerConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BoardName != "Test BBS" {
		t.Errorf("expected 'Test BBS', got %s", result.BoardName)
	}
	if result.SSHPort != 3333 {
		t.Errorf("expected SSH port 3333, got %d", result.SSHPort)
	}
	if result.MaxNodes != 50 {
		t.Errorf("expected MaxNodes 50, got %d", result.MaxNodes)
	}
}

func TestLoadThemeConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := LoadThemeConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.YesNoHighlightColor != 112 {
		t.Errorf("expected default highlight color 112, got %d", result.YesNoHighlightColor)
	}
	if result.YesNoRegularColor != 15 {
		t.Errorf("expected default regular color 15, got %d", result.YesNoRegularColor)
	}
}

func TestLoadThemeConfig_CustomValues(t *testing.T) {
	tmpDir := t.TempDir()
	theme := map[string]interface{}{
		"yesNoHighlightColor": 200,
		"yesNoRegularColor":   7,
	}
	data, _ := json.Marshal(theme)
	os.WriteFile(filepath.Join(tmpDir, "theme.json"), data, 0644)

	result, err := LoadThemeConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.YesNoHighlightColor != 200 {
		t.Errorf("expected highlight color 200, got %d", result.YesNoHighlightColor)
	}
}

func TestLoadEventsConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := LoadEventsConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Enabled {
		t.Error("expected events disabled by default")
	}
	if result.MaxConcurrentEvents != 3 {
		t.Errorf("expected default max concurrent 3, got %d", result.MaxConcurrentEvents)
	}
}

func TestLoadEventsConfig_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := EventsConfig{
		Enabled:             true,
		MaxConcurrentEvents: 5,
		Events: []EventConfig{
			{ID: "test", Name: "Test Event", Schedule: "0 * * * *", Command: "echo", Enabled: true},
			{ID: "disabled", Name: "Disabled", Schedule: "0 0 * * *", Command: "echo", Enabled: false},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(tmpDir, "events.json"), data, 0644)

	result, err := LoadEventsConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Enabled {
		t.Error("expected events enabled")
	}
	if result.MaxConcurrentEvents != 5 {
		t.Errorf("expected max concurrent 5, got %d", result.MaxConcurrentEvents)
	}
	if len(result.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result.Events))
	}
}

func TestLoadEventsConfig_ZeroMaxConcurrent(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := map[string]interface{}{
		"enabled":               true,
		"max_concurrent_events": 0,
		"events":                []interface{}{},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(tmpDir, "events.json"), data, 0644)

	result, err := LoadEventsConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should default to 3 when 0 or negative
	if result.MaxConcurrentEvents != 3 {
		t.Errorf("expected default max concurrent 3 when set to 0, got %d", result.MaxConcurrentEvents)
	}
}

func TestLoadFTNConfig_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := LoadFTNConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Networks == nil {
		t.Error("expected initialized (empty) Networks map")
	}
	if len(result.Networks) != 0 {
		t.Errorf("expected 0 networks, got %d", len(result.Networks))
	}
}

func TestLoadFTNConfig_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := FTNConfig{
		DupeDBPath:   "data/ftn/dupes.json",
		InboundPath:  "data/ftn/in",
		OutboundPath: "data/ftn/temp_out",
		Networks: map[string]FTNNetworkConfig{
			"fsxnet": {
				InternalTosserEnabled: true,
				OwnAddress:            "21:3/110",
				Links: []FTNLinkConfig{
					{Address: "21:1/100", PacketPassword: "secret", Name: "Hub"},
				},
			},
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(tmpDir, "ftn.json"), data, 0644)

	result, err := LoadFTNConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Networks) != 1 {
		t.Fatalf("expected 1 network, got %d", len(result.Networks))
	}
	net := result.Networks["fsxnet"]
	if net.OwnAddress != "21:3/110" {
		t.Errorf("expected address 21:3/110, got %s", net.OwnAddress)
	}
	if len(net.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(net.Links))
	}
}

func TestLoadServerConfig_PartialOverlayPreservesDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	// Only override boardName — everything else should keep defaults
	cfg := map[string]interface{}{
		"boardName": "Custom BBS",
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(tmpDir, "config.json"), data, 0644)

	result, err := LoadServerConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BoardName != "Custom BBS" {
		t.Errorf("expected 'Custom BBS', got %s", result.BoardName)
	}
	// Verify defaults are preserved for unset fields
	if result.SysOpLevel != 255 {
		t.Errorf("expected default SysOpLevel 255, got %d", result.SysOpLevel)
	}
	if result.SSHPort != 2222 {
		t.Errorf("expected default SSHPort 2222, got %d", result.SSHPort)
	}
	if result.MaxFailedLogins != 5 {
		t.Errorf("expected default MaxFailedLogins 5, got %d", result.MaxFailedLogins)
	}
	if result.LockoutMinutes != 30 {
		t.Errorf("expected default LockoutMinutes 30, got %d", result.LockoutMinutes)
	}
}

func TestLoadStrings_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := map[string]interface{}{
		"pauseString":   "Press any key...",
		"anonymousName": "Anonymous Coward",
		"defColor1":     14,
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(tmpDir, "strings.json"), data, 0644)

	result, err := LoadStrings(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PauseString != "Press any key..." {
		t.Errorf("expected 'Press any key...', got %s", result.PauseString)
	}
	if result.AnonymousName != "Anonymous Coward" {
		t.Errorf("expected 'Anonymous Coward', got %s", result.AnonymousName)
	}
	if result.DefColor1 != 14 {
		t.Errorf("expected DefColor1 14, got %d", result.DefColor1)
	}
}

func TestLoadStrings_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := LoadStrings(tmpDir)
	if err == nil {
		t.Error("expected error for missing strings.json")
	}
}

func TestLoadStrings_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "strings.json"), []byte("{bad json"), 0644)

	_, err := LoadStrings(tmpDir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadOneLiners_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	content := "First liner\nSecond liner\nThird liner\n"
	os.WriteFile(filepath.Join(tmpDir, "oneliners.dat"), []byte(content), 0644)

	result, err := LoadOneLiners(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 oneliners, got %d", len(result))
	}
	if result[0] != "First liner" {
		t.Errorf("expected 'First liner', got %s", result[0])
	}
}

func TestLoadOneLiners_WindowsLineEndings(t *testing.T) {
	tmpDir := t.TempDir()
	content := "Line one\r\nLine two\r\nLine three\r\n"
	os.WriteFile(filepath.Join(tmpDir, "oneliners.dat"), []byte(content), 0644)

	result, err := LoadOneLiners(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 oneliners with CRLF, got %d", len(result))
	}
	if result[0] != "Line one" {
		t.Errorf("expected 'Line one', got %q", result[0])
	}
}

func TestLoadOneLiners_SkipsEmptyLines(t *testing.T) {
	tmpDir := t.TempDir()
	content := "Line one\n\n\nLine two\n  \n"
	os.WriteFile(filepath.Join(tmpDir, "oneliners.dat"), []byte(content), 0644)

	result, err := LoadOneLiners(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 oneliners (empty lines skipped), got %d: %v", len(result), result)
	}
}

func TestLoadOneLiners_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := LoadOneLiners(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 oneliners for missing file, got %d", len(result))
	}
}

func TestLoadOneLiners_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "oneliners.dat"), []byte(""), 0644)

	result, err := LoadOneLiners(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 oneliners from empty file, got %d", len(result))
	}
}

func TestLoadServerConfig_AllowNewUsersDefaultTrue(t *testing.T) {
	tmpDir := t.TempDir()
	// No config.json — AllowNewUsers should default to true
	result, err := LoadServerConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AllowNewUsers {
		t.Error("expected AllowNewUsers to default to true")
	}
}

func TestLoadServerConfig_AllowNewUsersExplicitFalse(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := map[string]interface{}{
		"allowNewUsers": false,
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(tmpDir, "config.json"), data, 0644)

	result, err := LoadServerConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AllowNewUsers {
		t.Error("expected AllowNewUsers to be false when explicitly set")
	}
}

func TestLoadServerConfig_AllowNewUsersExplicitTrue(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := map[string]interface{}{
		"boardName":     "Test BBS",
		"allowNewUsers": true,
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(tmpDir, "config.json"), data, 0644)

	result, err := LoadServerConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AllowNewUsers {
		t.Error("expected AllowNewUsers to be true when explicitly set")
	}
	if result.BoardName != "Test BBS" {
		t.Errorf("expected 'Test BBS', got %s", result.BoardName)
	}
}

func TestLoadServerConfig_AllowNewUsersPreservedInPartialOverlay(t *testing.T) {
	tmpDir := t.TempDir()
	// Only override boardName — AllowNewUsers should keep default (true)
	cfg := map[string]interface{}{
		"boardName": "Partial BBS",
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(tmpDir, "config.json"), data, 0644)

	result, err := LoadServerConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AllowNewUsers {
		t.Error("expected AllowNewUsers to remain true when not specified in config")
	}
}

func TestLoadStrings_NewUsersClosedStr(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := map[string]interface{}{
		"newUsersClosedStr": "|12Registration is closed.|07",
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(tmpDir, "strings.json"), data, 0644)

	result, err := LoadStrings(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NewUsersClosedStr != "|12Registration is closed.|07" {
		t.Errorf("expected custom NewUsersClosedStr, got %q", result.NewUsersClosedStr)
	}
}
