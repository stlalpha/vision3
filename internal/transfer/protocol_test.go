package transfer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- expandArgs ---

func TestExpandArgs_filePath_standalone(t *testing.T) {
	tmpl := []string{"-b", "-e", "{filePath}"}
	got, _ := expandArgs(tmpl, []string{"/files/a.zip", "/files/b.zip"}, "")
	want := []string{"-b", "-e", "/files/a.zip", "/files/b.zip"}
	assertStringSlice(t, want, got)
}

func TestExpandArgs_filePath_appended_when_absent(t *testing.T) {
	// lrzsz rz style: no placeholder, files appended at end
	tmpl := []string{"-b", "-r"}
	// recv call: no filePaths, so nothing appended
	got, _ := expandArgs(tmpl, nil, "/upload")
	want := []string{"-b", "-r"}
	assertStringSlice(t, want, got)
}

func TestExpandArgs_targetDir_standalone(t *testing.T) {
	tmpl := []string{"-r", "{targetDir}"}
	got, _ := expandArgs(tmpl, nil, "/upload/tmp")
	want := []string{"-r", "/upload/tmp/"}
	assertStringSlice(t, want, got)
}

func TestExpandArgs_sexyz_send(t *testing.T) {
	// sexyz -raw -8 sz file1 file2
	tmpl := []string{"-raw", "-8", "sz", "{filePath}"}
	got, _ := expandArgs(tmpl, []string{"/f/a.zip", "/f/b.zip"}, "")
	want := []string{"-raw", "-8", "sz", "/f/a.zip", "/f/b.zip"}
	assertStringSlice(t, want, got)
}

func TestExpandArgs_sexyz_recv(t *testing.T) {
	// sexyz -raw -8 rz /upload/  (dir with trailing separator)
	tmpl := []string{"-raw", "-8", "rz", "{targetDir}"}
	got, _ := expandArgs(tmpl, nil, "/upload")
	want := []string{"-raw", "-8", "rz", "/upload/"}
	assertStringSlice(t, want, got)
}

func TestExpandArgs_inline_replacement(t *testing.T) {
	// Inline replacement uses the first filePath only; remaining files are NOT appended
	// (inline substitution marks {filePath} as consumed).
	tmpl := []string{"send:{filePath}"}
	got, _ := expandArgs(tmpl, []string{"/f/a.zip", "/f/b.zip"}, "")
	want := []string{"send:/f/a.zip"}
	assertStringSlice(t, want, got)
}

func TestExpandArgs_empty_template(t *testing.T) {
	got, _ := expandArgs(nil, []string{"/f/a.zip"}, "")
	want := []string{"/f/a.zip"} // appended since no placeholder
	assertStringSlice(t, want, got)
}

func TestExpandArgs_fileListPath(t *testing.T) {
	// sexyz-style batch send with @{fileListPath}
	tmpl := []string{"-raw", "-8", "sz", "@{fileListPath}"}
	files := []string{"/files/a.zip", "/files/b.zip"}
	got, listFile := expandArgs(tmpl, files, "")
	if listFile == "" {
		t.Fatal("expected listFile path, got empty")
	}
	defer os.Remove(listFile)

	// The @{fileListPath} arg should contain "@" + temp file path
	if len(got) != 4 {
		t.Fatalf("want 4 args, got %d: %v", len(got), got)
	}
	if got[3] != "@"+listFile {
		t.Errorf("expected @tempfile, got %q", got[3])
	}

	// Verify file contents
	data, err := os.ReadFile(listFile)
	if err != nil {
		t.Fatalf("failed to read list file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "/files/a.zip") || !strings.Contains(content, "/files/b.zip") {
		t.Errorf("list file missing expected paths, got: %q", content)
	}
}

// --- LoadProtocols ---

func TestLoadProtocols_valid(t *testing.T) {
	protocols := []ProtocolConfig{
		{Key: "Z", Name: "Zmodem", SendCmd: "sz", SendArgs: []string{"-b", "-e"}, RecvCmd: "rz", RecvArgs: []string{"-b", "-r"}, BatchSend: true, UsePTY: true, Default: true},
		{Key: "Y", Name: "Ymodem", SendCmd: "sb", SendArgs: []string{"-b"}, RecvCmd: "rb", RecvArgs: []string{"-b"}, BatchSend: true, UsePTY: true},
	}
	path := writeTempProtocols(t, protocols)

	loaded, err := LoadProtocols(path)
	if err != nil {
		t.Fatalf("LoadProtocols error: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("want 2 protocols, got %d", len(loaded))
	}
	if loaded[0].Key != "Z" || loaded[0].Name != "Zmodem" {
		t.Errorf("unexpected first protocol: %+v", loaded[0])
	}
}

func TestLoadProtocols_missing_file_returns_defaults(t *testing.T) {
	loaded, err := LoadProtocols("/nonexistent/path/protocols.json")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(loaded) == 0 {
		t.Fatal("expected built-in defaults when file missing, got empty slice")
	}
	def, ok := DefaultProtocol(loaded)
	if !ok || def.SendCmd == "" {
		t.Errorf("built-in default has no SendCmd: %+v", def)
	}
}

func TestLoadProtocols_malformed_json(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "protocols*.json")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("{not valid json")
	f.Close()

	_, err = LoadProtocols(f.Name())
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// --- DefaultProtocol ---

func TestDefaultProtocol_marked(t *testing.T) {
	ps := []ProtocolConfig{
		{Key: "Y", Name: "Ymodem", Default: false},
		{Key: "Z", Name: "Zmodem", Default: true},
	}
	got, ok := DefaultProtocol(ps)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Key != "Z" {
		t.Errorf("want key Z, got %q", got.Key)
	}
}

func TestDefaultProtocol_first_when_none_marked(t *testing.T) {
	ps := []ProtocolConfig{
		{Key: "Y", Name: "Ymodem"},
		{Key: "Z", Name: "Zmodem"},
	}
	got, ok := DefaultProtocol(ps)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Key != "Y" {
		t.Errorf("want first key Y, got %q", got.Key)
	}
}

func TestDefaultProtocol_empty(t *testing.T) {
	_, ok := DefaultProtocol(nil)
	if ok {
		t.Fatal("expected ok=false for empty slice")
	}
}

// --- FindProtocol ---

func TestFindProtocol_found(t *testing.T) {
	ps := []ProtocolConfig{
		{Key: "Y", Name: "Ymodem"},
		{Key: "Z", Name: "Zmodem", Default: true},
	}
	got, ok := FindProtocol(ps, "z")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Name != "Zmodem" {
		t.Errorf("want Zmodem, got %q", got.Name)
	}
}

func TestFindProtocol_case_insensitive(t *testing.T) {
	ps := []ProtocolConfig{{Key: "Z", Name: "Zmodem", Default: true}}
	got, ok := FindProtocol(ps, "z")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Key != "Z" {
		t.Errorf("want Z, got %q", got.Key)
	}
}

func TestFindProtocol_not_found_returns_default(t *testing.T) {
	ps := []ProtocolConfig{
		{Key: "Z", Name: "Zmodem", Default: true},
	}
	got, ok := FindProtocol(ps, "X")
	// ok is false (key not found), but got should be the default
	if ok {
		t.Error("expected ok=false for unknown key")
	}
	if got.Key != "Z" {
		t.Errorf("want default Z, got %q", got.Key)
	}
}

// --- Command-line argument assembly sanity checks ---

// TestLRZSZSendArgs verifies the lrzsz sz command args are assembled correctly.
func TestLRZSZSendArgs(t *testing.T) {
	p := ProtocolConfig{
		Name:     "Zmodem (lrzsz)",
		SendCmd:  "sz",
		SendArgs: []string{"-b", "-e"},
	}
	files := []string{"/files/area1/foo.zip", "/files/area1/bar.txt"}
	args, _ := expandArgs(p.SendArgs, files, "")
	// sz -b -e /files/area1/foo.zip /files/area1/bar.txt
	want := []string{"-b", "-e", "/files/area1/foo.zip", "/files/area1/bar.txt"}
	assertStringSlice(t, want, args)
}

// TestLRZSZRecvArgs verifies the lrzsz rz command args (no file args, dir via cmd.Dir).
func TestLRZSZRecvArgs(t *testing.T) {
	p := ProtocolConfig{
		Name:     "Zmodem (lrzsz)",
		RecvCmd:  "rz",
		RecvArgs: []string{"-b", "-r"},
	}
	args, _ := expandArgs(p.RecvArgs, nil, "/upload/tmp-123")
	want := []string{"-b", "-r"}
	assertStringSlice(t, want, args)
}

// TestSexyZSendArgs verifies the sexyz send command: sexyz -raw -8 sz file1 file2
func TestSexyZSendArgs(t *testing.T) {
	p := ProtocolConfig{
		Name:     "Zmodem 8k (sexyz)",
		SendCmd:  "sexyz",
		SendArgs: []string{"-raw", "-8", "sz", "{filePath}"},
	}
	files := []string{"/files/area1/foo.zip", "/files/area1/bar.txt"}
	args, _ := expandArgs(p.SendArgs, files, "")
	want := []string{"-raw", "-8", "sz", "/files/area1/foo.zip", "/files/area1/bar.txt"}
	assertStringSlice(t, want, args)
}

// TestSexyZRecvArgs verifies the sexyz receive command: sexyz -raw -8 rz /targetDir/
// The trailing separator is critical — sexyz concatenates dir+filename without
// inserting a separator (Synchronet bug: backslash() call is commented out).
func TestSexyZRecvArgs(t *testing.T) {
	p := ProtocolConfig{
		Name:     "Zmodem 8k (sexyz)",
		RecvCmd:  "sexyz",
		RecvArgs: []string{"-raw", "-8", "rz", "{targetDir}"},
	}
	args, _ := expandArgs(p.RecvArgs, nil, "/upload/tmp-123")
	want := []string{"-raw", "-8", "rz", "/upload/tmp-123/"}
	assertStringSlice(t, want, args)
}

// TestExpandArgs_targetDir_already_has_separator ensures no double slash.
func TestExpandArgs_targetDir_already_has_separator(t *testing.T) {
	tmpl := []string{"rz", "{targetDir}"}
	got, _ := expandArgs(tmpl, nil, "/upload/tmp/")
	want := []string{"rz", "/upload/tmp/"}
	assertStringSlice(t, want, got)
}

// TestExpandArgs_targetDir_empty stays empty.
func TestExpandArgs_targetDir_empty(t *testing.T) {
	tmpl := []string{"rz", "{targetDir}"}
	got, _ := expandArgs(tmpl, nil, "")
	want := []string{"rz", ""}
	assertStringSlice(t, want, got)
}

// TestYmodemSendArgs verifies lrzsz sb command args.
func TestYmodemSendArgs(t *testing.T) {
	p := ProtocolConfig{
		Name:     "Ymodem (lrzsz)",
		SendCmd:  "sb",
		SendArgs: []string{"-b"},
	}
	files := []string{"/files/foo.txt"}
	args, _ := expandArgs(p.SendArgs, files, "")
	want := []string{"-b", "/files/foo.txt"}
	assertStringSlice(t, want, args)
}

// --- helpers ---

func writeTempProtocols(t *testing.T, protocols []ProtocolConfig) string {
	t.Helper()
	data, err := json.Marshal(protocols)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "protocols.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertStringSlice(t *testing.T, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Errorf("length mismatch: want %d %v, got %d %v", len(want), want, len(got), got)
		return
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("index %d: want %q, got %q", i, want[i], got[i])
		}
	}
}
