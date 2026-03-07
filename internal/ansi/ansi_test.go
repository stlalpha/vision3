package ansi

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// ReplacePipeCodes
// ---------------------------------------------------------------------------

func TestReplacePipeCodes(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want []byte
	}{
		// Foreground colors
		{
			name: "pipe 00 black foreground",
			in:   []byte("|00"),
			want: []byte("\x1B[0;30m"),
		},
		{
			name: "pipe 01 blue foreground",
			in:   []byte("|01"),
			want: []byte("\x1B[0;34m"),
		},
		{
			name: "pipe 09 light blue",
			in:   []byte("|09"),
			want: []byte("\x1B[1;34m"),
		},
		{
			name: "pipe 15 white",
			in:   []byte("|15"),
			want: []byte("\x1B[1;37m"),
		},
		// Background colors
		{
			name: "pipe B0 black background",
			in:   []byte("|B0"),
			want: []byte("\x1B[40m"),
		},
		{
			name: "pipe B7 gray background",
			in:   []byte("|B7"),
			want: []byte("\x1B[47m"),
		},
		{
			name: "pipe B15 bright white background",
			in:   []byte("|B15"),
			want: []byte("\x1B[107m"),
		},
		{
			name: "pipe B12 bright blue background (dual escape)",
			in:   []byte("|B12"),
			want: []byte("\x1B[104m\x1B[48;5;12m"),
		},
		// Special codes
		{
			name: "pipe CL clear screen",
			in:   []byte("|CL"),
			want: []byte("\x1B[2J\x1B[H"),
		},
		{
			name: "pipe CR newline",
			in:   []byte("|CR"),
			want: []byte("\r\n"),
		},
		{
			name: "pipe DE clear to end of line",
			in:   []byte("|DE"),
			want: []byte("\x1B[K"),
		},
		{
			name: "pipe P save cursor",
			in:   []byte("|P "),
			want: []byte("\x1B[s "),
		},
		{
			name: "pipe PP restore cursor",
			in:   []byte("|PP"),
			want: []byte("\x1B[u"),
		},
		{
			name: "pipe 23 reset attributes",
			in:   []byte("|23"),
			want: []byte("\x1B[0m"),
		},
		// Escaped literal pipe
		{
			name: "double pipe becomes literal pipe",
			in:   []byte("||"),
			want: []byte("|"),
		},
		{
			name: "double pipe followed by text",
			in:   []byte("||hello"),
			want: []byte("|hello"),
		},
		// Unknown codes passed through
		{
			name: "unknown pipe code literal",
			in:   []byte("|ZZ"),
			want: []byte("|ZZ"),
		},
		{
			name: "pipe at end of input",
			in:   []byte("end|"),
			want: []byte("end|"),
		},
		{
			name: "pipe with only one char left",
			in:   []byte("x|A"),
			want: []byte("x|A"),
		},
		// Multiple codes in sequence
		{
			name: "multiple codes",
			in:   []byte("|01Hello|23"),
			want: []byte("\x1B[0;34mHello\x1B[0m"),
		},
		// No codes at all
		{
			name: "no pipe codes",
			in:   []byte("plain text"),
			want: []byte("plain text"),
		},
		// Empty input
		{
			name: "empty input",
			in:   []byte{},
			want: []byte{},
		},
		// Mixed escaped and real pipes
		{
			name: "escaped pipe then real code",
			in:   []byte("|||01"),
			want: []byte("|\x1B[0;34m"),
		},
		// Three-char code (|B10-|B15) takes priority over two-char
		{
			name: "four char code B10",
			in:   []byte("|B10"),
			want: []byte("\x1B[102m"),
		},
		// Pipe code embedded in text
		{
			name: "pipe code in middle of text",
			in:   []byte("Hello |09World|23!"),
			want: []byte("Hello \x1B[1;34mWorld\x1B[0m!"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplacePipeCodes(tt.in)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("ReplacePipeCodes(%q)\n  got  %q\n  want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CP437BytesToUTF8
// ---------------------------------------------------------------------------

func TestCP437BytesToUTF8(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want []byte
	}{
		{
			name: "pure ASCII passes through",
			in:   []byte("Hello World"),
			want: []byte("Hello World"),
		},
		{
			name: "empty input",
			in:   []byte{},
			want: []byte{},
		},
		{
			name: "high byte converts to UTF-8 rune",
			// 0xDB is CP437 full block -> Unicode U+2588 -> UTF-8 \xe2\x96\x88
			in:   []byte{0xDB},
			want: []byte("█"),
		},
		{
			name: "multiple high bytes",
			// 0xB0=░ 0xB1=▒ 0xB2=▓
			in:   []byte{0xB0, 0xB1, 0xB2},
			want: []byte("░▒▓"),
		},
		{
			name: "ANSI CSI escape passed through unchanged",
			in:   []byte("\x1B[1;31m"),
			want: []byte("\x1B[1;31m"),
		},
		{
			name: "ANSI escape with high byte after",
			in:   []byte{0x1B, '[', '3', '1', 'm', 0xDB},
			want: []byte("\x1B[31m█"),
		},
		{
			name: "charset designator ESC(0 passed through",
			in:   []byte{0x1B, '(', '0'},
			want: []byte("\x1B(0"),
		},
		{
			name: "charset designator ESC)B passed through",
			in:   []byte{0x1B, ')', 'B'},
			want: []byte("\x1B)B"),
		},
		{
			name: "simple two-byte ESC sequence",
			in:   []byte{0x1B, '7'},
			want: []byte("\x1B7"),
		},
		{
			name: "mixed ASCII, ANSI, and CP437",
			in:   []byte{'H', 'i', 0x1B, '[', '0', 'm', 0xC4, '!'},
			// 0xC4 = ─ (U+2500)
			want: append([]byte("Hi\x1B[0m"), append([]byte("─"), '!')...),
		},
		{
			name: "ESC at end of input (no following byte)",
			in:   []byte{0x1B},
			// When ESC is at end, i++ happens, then loop ends, ESC is appended
			want: []byte{0x1B},
		},
		{
			name: "0x80 converts to C cedilla",
			// CP437 0x80 = Ç (U+00C7)
			in:   []byte{0x80},
			want: []byte("Ç"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CP437BytesToUTF8(tt.in)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("CP437BytesToUTF8(%v)\n  got  %v (%q)\n  want %v (%q)", tt.in, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestCP437BytesToUTF8_LongAnsiSequence(t *testing.T) {
	// Construct an ANSI CSI sequence longer than 32 bytes to trigger the safety break
	seq := make([]byte, 0, 40)
	seq = append(seq, 0x1B, '[')
	for i := 0; i < 35; i++ {
		seq = append(seq, '0') // parameter bytes
	}
	seq = append(seq, 'm') // terminator

	got := CP437BytesToUTF8(seq)
	// The function should still produce output without panicking.
	// It breaks out of the inner loop after 32 bytes from start, so
	// the terminator 'm' and trailing '0's are handled as separate bytes.
	if len(got) == 0 {
		t.Error("expected non-empty output for long ANSI sequence")
	}
}

// ---------------------------------------------------------------------------
// StripAnsi
// ---------------------------------------------------------------------------

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no ANSI codes",
			in:   "Hello World",
			want: "Hello World",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "single color code",
			in:   "\x1B[31mRed\x1B[0m",
			want: "Red",
		},
		{
			name: "bold and color",
			in:   "\x1B[1;31mBold Red\x1B[0m",
			want: "Bold Red",
		},
		{
			name: "multiple sequences",
			in:   "\x1B[31mRed\x1B[0m Normal \x1B[32mGreen\x1B[0m",
			want: "Red Normal Green",
		},
		{
			name: "cursor movement",
			in:   "\x1B[10;20H",
			want: "",
		},
		{
			name: "clear screen",
			in:   "\x1B[2J\x1B[HContent",
			want: "Content",
		},
		{
			name: "text with no terminator after ESC[",
			// ESC[ followed by digits but no letter terminator at end of string
			in:   "before\x1B[31",
			want: "before",
		},
		{
			name: "ESC not followed by bracket",
			// StripAnsi only strips ESC[ sequences; lone ESC is kept
			in:   "a\x1Bb",
			want: "a\x1Bb",
		},
		{
			name: "cursor erase line",
			in:   "text\x1B[Kmore",
			want: "textmore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripAnsi(tt.in)
			if got != tt.want {
				t.Errorf("StripAnsi(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ClearScreen
// ---------------------------------------------------------------------------

func TestClearScreen(t *testing.T) {
	want := "\x1B[2J\x1B[H"
	got := ClearScreen()
	if got != want {
		t.Errorf("ClearScreen() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// MoveCursor
// ---------------------------------------------------------------------------

func TestMoveCursor(t *testing.T) {
	tests := []struct {
		name     string
		row, col int
		want     string
	}{
		{"top left", 1, 1, "\x1B[1;1H"},
		{"middle", 10, 20, "\x1B[10;20H"},
		{"large values", 100, 200, "\x1B[100;200H"},
		{"zero values", 0, 0, "\x1B[0;0H"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MoveCursor(tt.row, tt.col)
			if got != tt.want {
				t.Errorf("MoveCursor(%d, %d) = %q, want %q", tt.row, tt.col, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SaveCursor / RestoreCursor
// ---------------------------------------------------------------------------

func TestSaveCursor(t *testing.T) {
	want := "\x1B[s\x1B7"
	got := SaveCursor()
	if got != want {
		t.Errorf("SaveCursor() = %q, want %q", got, want)
	}
}

func TestRestoreCursor(t *testing.T) {
	want := "\x1B[u\x1B8"
	got := RestoreCursor()
	if got != want {
		t.Errorf("RestoreCursor() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// CursorBackward
// ---------------------------------------------------------------------------

func TestCursorBackward(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want string
	}{
		{"positive", 5, "\x1B[5D"},
		{"one", 1, "\x1B[1D"},
		{"zero returns empty", 0, ""},
		{"negative returns empty", -3, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CursorBackward(tt.n)
			if got != tt.want {
				t.Errorf("CursorBackward(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// stripSAUCE (additional cases beyond sauce_test.go)
// ---------------------------------------------------------------------------

func TestStripSAUCE_Exactly128Bytes(t *testing.T) {
	// Input is exactly 128 bytes and starts with SAUCE
	data := make([]byte, 128)
	copy(data, []byte("SAUCE00"))
	got := stripSAUCE(data)
	if len(got) != 0 {
		t.Errorf("stripSAUCE with 128-byte SAUCE-only input: got len %d, want 0", len(got))
	}
}

func TestStripSAUCE_EmptyInput(t *testing.T) {
	got := stripSAUCE([]byte{})
	if len(got) != 0 {
		t.Errorf("stripSAUCE with empty input: got len %d, want 0", len(got))
	}
}

func TestStripSAUCE_EOFMarkerFarBack(t *testing.T) {
	// EOF marker is present but content between EOF and SAUCE is substantial
	content := bytes.Repeat([]byte("A"), 200)
	content = append(content, 0x1A) // EOF marker
	content = append(content, bytes.Repeat([]byte("C"), 50)...) // comment block
	sauce := make([]byte, 128)
	copy(sauce, []byte("SAUCE00"))
	content = append(content, sauce...)

	got := stripSAUCE(content)
	// Should strip everything from EOF marker onward
	want := bytes.Repeat([]byte("A"), 200)
	if !bytes.Equal(got, want) {
		t.Errorf("stripSAUCE with distant EOF marker: got len %d, want len %d", len(got), len(want))
	}
}

// ---------------------------------------------------------------------------
// GetAnsiFileContent
// ---------------------------------------------------------------------------

func TestGetAnsiFileContent_NonexistentFile(t *testing.T) {
	_, err := GetAnsiFileContent("/nonexistent/path/file.ans")
	if err == nil {
		t.Error("GetAnsiFileContent with nonexistent file: expected error, got nil")
	}
}

func TestGetAnsiFileContent_ValidFile(t *testing.T) {
	// Create a temp file with ANSI content
	dir := t.TempDir()
	path := filepath.Join(dir, "test.ans")
	content := []byte("Hello \x1B[31mWorld\x1B[0m")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	got, err := GetAnsiFileContent(path)
	if err != nil {
		t.Fatalf("GetAnsiFileContent: unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("GetAnsiFileContent: got %q, want %q", got, content)
	}
}

func TestGetAnsiFileContent_FileWithSAUCE(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "art.ans")

	art := []byte("ANSI art\r\n")
	art = append(art, 0x1A) // EOF marker
	sauce := make([]byte, 128)
	copy(sauce, []byte("SAUCE00"))
	art = append(art, sauce...)

	if err := os.WriteFile(path, art, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	got, err := GetAnsiFileContent(path)
	if err != nil {
		t.Fatalf("GetAnsiFileContent: unexpected error: %v", err)
	}
	want := []byte("ANSI art\r\n")
	if !bytes.Equal(got, want) {
		t.Errorf("GetAnsiFileContent with SAUCE: got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// ProcessAnsiAndExtractCoords — basic cases
// ---------------------------------------------------------------------------

func TestProcessAnsiAndExtractCoords_PlainText(t *testing.T) {
	input := []byte("Hello")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(result.DisplayBytes, []byte("Hello")) {
		t.Errorf("DisplayBytes = %q, want %q", result.DisplayBytes, "Hello")
	}
	if len(result.FieldCoords) != 0 {
		t.Errorf("expected no field coords, got %d", len(result.FieldCoords))
	}
}

func TestProcessAnsiAndExtractCoords_PipeColorCode(t *testing.T) {
	input := []byte("|01Blue")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The pipe code |01 should be replaced with its ANSI equivalent
	if !bytes.Contains(result.DisplayBytes, []byte("\x1B[0;34m")) {
		t.Errorf("DisplayBytes should contain ANSI blue code, got %q", result.DisplayBytes)
	}
	if !bytes.Contains(result.DisplayBytes, []byte("Blue")) {
		t.Errorf("DisplayBytes should contain 'Blue', got %q", result.DisplayBytes)
	}
}

func TestProcessAnsiAndExtractCoords_FieldPlaceholder(t *testing.T) {
	// |AB should be recognized as a field placeholder (uppercase letters)
	input := []byte("|AB")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	coord, ok := result.FieldCoords["AB"]
	if !ok {
		t.Fatal("expected field coord 'AB' to be recorded")
	}
	if coord.X != 1 || coord.Y != 1 {
		t.Errorf("field coord AB = (%d, %d), want (1, 1)", coord.X, coord.Y)
	}
}

func TestProcessAnsiAndExtractCoords_TildeFieldCode(t *testing.T) {
	input := []byte("Hi~XY")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	coord, ok := result.FieldCoords["XY"]
	if !ok {
		t.Fatal("expected field coord 'XY' to be recorded from ~ code")
	}
	// "Hi" = 2 chars, so X=3
	if coord.X != 3 || coord.Y != 1 {
		t.Errorf("field coord XY = (%d, %d), want (3, 1)", coord.X, coord.Y)
	}
}

func TestProcessAnsiAndExtractCoords_Newline(t *testing.T) {
	input := []byte("A\r\nB")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After normalization \r\n -> \n, A is at (1,1), then newline -> (1,2), B at (1,2)
	if !bytes.Contains(result.DisplayBytes, []byte("A")) {
		t.Error("DisplayBytes should contain 'A'")
	}
	if !bytes.Contains(result.DisplayBytes, []byte("B")) {
		t.Error("DisplayBytes should contain 'B'")
	}
}

func TestProcessAnsiAndExtractCoords_CursorPosition(t *testing.T) {
	// ESC[5;10H should set cursor to row 5, col 10
	input := []byte("\x1B[5;10HX")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The ANSI sequence should be in the display output
	if !bytes.Contains(result.DisplayBytes, []byte("\x1B[5;10H")) {
		t.Errorf("DisplayBytes should contain cursor position sequence, got %q", result.DisplayBytes)
	}
}

func TestProcessAnsiAndExtractCoords_ClearScreenResetsCursor(t *testing.T) {
	// |CL is a known pipe command (clear screen + cursor home), not a field placeholder.
	// After |CL the cursor resets to (1,1), so |CD is recorded at (1,1).
	input := []byte("AB|CL|CD")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// |CL must NOT be captured as a placeholder
	if _, ok := result.FieldCoords["CL"]; ok {
		t.Error("'CL' should not be recorded as a placeholder (it is a pipe command)")
	}
	coord, ok := result.FieldCoords["CD"]
	if !ok {
		t.Fatal("expected field coord 'CD' to be recorded")
	}
	// Cursor resets to (1,1) after |CL, so CD is at (1,1)
	if coord.X != 1 || coord.Y != 1 {
		t.Errorf("field coord CD = (%d, %d), want (1, 1)", coord.X, coord.Y)
	}
}

func TestProcessAnsiAndExtractCoords_UTF8Mode(t *testing.T) {
	// In UTF8 mode, high bytes should be converted to Unicode
	input := []byte{0xDB} // CP437 full block
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeUTF8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(result.DisplayBytes, []byte("█")) {
		t.Errorf("UTF8 mode: DisplayBytes = %q, expected Unicode full block", result.DisplayBytes)
	}
}

func TestProcessAnsiAndExtractCoords_CP437Mode(t *testing.T) {
	// In CP437 mode, high bytes should be written raw
	input := []byte{0xDB}
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(result.DisplayBytes, []byte{0xDB}) {
		t.Errorf("CP437 mode: DisplayBytes = %v, want [0xDB]", result.DisplayBytes)
	}
}

func TestProcessAnsiAndExtractCoords_DollarColorCode(t *testing.T) {
	input := []byte("$3text")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// $3 should produce \x1b[33m
	if !bytes.Contains(result.DisplayBytes, []byte("\x1b[33m")) {
		t.Errorf("DisplayBytes should contain $3 color code, got %q", result.DisplayBytes)
	}
}

func TestProcessAnsiAndExtractCoords_CaretBEL(t *testing.T) {
	input := []byte("^G")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(result.DisplayBytes, []byte{0x07}) {
		t.Errorf("DisplayBytes should contain BEL (0x07), got %q", result.DisplayBytes)
	}
}

func TestProcessAnsiAndExtractCoords_EmptyInput(t *testing.T) {
	result, err := ProcessAnsiAndExtractCoords([]byte{}, OutputModeAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.DisplayBytes) != 0 {
		t.Errorf("expected empty DisplayBytes, got %q", result.DisplayBytes)
	}
	if len(result.FieldCoords) != 0 {
		t.Errorf("expected empty FieldCoords, got %d entries", len(result.FieldCoords))
	}
}

func TestProcessAnsiAndExtractCoords_FieldColor(t *testing.T) {
	// Set a color via pipe code, then record a field placeholder
	input := []byte("|01|AB")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	color, ok := result.FieldColors["AB"]
	if !ok {
		t.Fatal("expected FieldColors entry for 'AB'")
	}
	// After |01 (blue fg = \x1b[0;34m]), the SGR state should have foreground=34
	if color == "" {
		t.Error("expected non-empty color for field AB after |01")
	}
}

// ---------------------------------------------------------------------------
// Cp437ToUnicode / UnicodeToCP437 map consistency
// ---------------------------------------------------------------------------

func TestCp437ToUnicodeASCIIRange(t *testing.T) {
	// Verify ASCII printable range (0x20-0x7E) maps to itself
	for b := byte(0x20); b <= 0x7E; b++ {
		r := Cp437ToUnicode[b]
		if r != rune(b) {
			t.Errorf("Cp437ToUnicode[0x%02X] = U+%04X, want U+%04X", b, r, b)
		}
	}
}

func TestUnicodeToCP437RoundTrip(t *testing.T) {
	// For every entry in UnicodeToCP437, verify the forward map agrees
	for r, b := range UnicodeToCP437 {
		forward := Cp437ToUnicode[b]
		if forward != r {
			t.Errorf("UnicodeToCP437[U+%04X]=0x%02X, but Cp437ToUnicode[0x%02X]=U+%04X (expected U+%04X)",
				r, b, b, forward, r)
		}
	}
}

// ---------------------------------------------------------------------------
// pipeCodeReplacements map sanity
// ---------------------------------------------------------------------------

func TestPipeCodeReplacementsCompleteness(t *testing.T) {
	// Verify all foreground codes |00 through |15 exist
	for i := 0; i <= 15; i++ {
		var code string
		if i < 10 {
			code = "|0" + string(rune('0'+i))
		} else {
			code = "|1" + string(rune('0'+i-10))
		}
		if _, ok := pipeCodeReplacements[code]; !ok {
			t.Errorf("missing pipe code replacement for %s", code)
		}
	}

	// Verify background codes |B0 through |B15
	for i := 0; i <= 15; i++ {
		code := "|B" + intToStr(i)
		if _, ok := pipeCodeReplacements[code]; !ok {
			t.Errorf("missing pipe code replacement for %s", code)
		}
	}

	// Verify special codes
	for _, code := range []string{"|CL", "|CR", "|DE", "|P", "|PP", "|23"} {
		if _, ok := pipeCodeReplacements[code]; !ok {
			t.Errorf("missing pipe code replacement for %s", code)
		}
	}
}

// helper to avoid importing strconv
func intToStr(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// ---------------------------------------------------------------------------
// ReplacePipeCodes edge cases
// ---------------------------------------------------------------------------

func TestReplacePipeCodes_OnlyPipes(t *testing.T) {
	// A string of just || characters
	got := ReplacePipeCodes([]byte("||||||"))
	want := []byte("|||")
	if !bytes.Equal(got, want) {
		t.Errorf("ReplacePipeCodes(||||||) = %q, want %q", got, want)
	}
}

func TestReplacePipeCodes_PipeAtBoundary(t *testing.T) {
	// Pipe with exactly one character remaining (not enough for |XX)
	got := ReplacePipeCodes([]byte("|0"))
	want := []byte("|0")
	if !bytes.Equal(got, want) {
		t.Errorf("ReplacePipeCodes(|0) = %q, want %q", got, want)
	}
}

func TestReplacePipeCodes_ConsecutiveCodes(t *testing.T) {
	// Two consecutive valid codes with no text between them
	got := ReplacePipeCodes([]byte("|01|02"))
	want := []byte("\x1B[0;34m\x1B[0;32m")
	if !bytes.Equal(got, want) {
		t.Errorf("ReplacePipeCodes(|01|02) = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// OutputMode constants
// ---------------------------------------------------------------------------

func TestOutputModeConstants(t *testing.T) {
	if OutputModeAuto != 0 {
		t.Errorf("OutputModeAuto = %d, want 0", OutputModeAuto)
	}
	if OutputModeUTF8 != 1 {
		t.Errorf("OutputModeUTF8 = %d, want 1", OutputModeUTF8)
	}
	if OutputModeCP437 != 2 {
		t.Errorf("OutputModeCP437 = %d, want 2", OutputModeCP437)
	}
}

// ---------------------------------------------------------------------------
// ProcessAnsiAndExtractCoords — additional coverage tests
// ---------------------------------------------------------------------------

func TestProcessAnsiAndExtractCoords_SGRAttributes(t *testing.T) {
	// Test SGR attribute tracking: bold, dim, italic, underline, blink, reverse, hidden
	// and their corresponding reset codes
	tests := []struct {
		name      string
		input     string
		wantColor string // expected color from buildColorSequence after SGR
	}{
		{
			name:  "bold attribute",
			input: "\x1b[1m|AB",
		},
		{
			name:  "dim attribute",
			input: "\x1b[2m|AB",
		},
		{
			name:  "italic attribute",
			input: "\x1b[3m|AB",
		},
		{
			name:  "underline attribute",
			input: "\x1b[4m|AB",
		},
		{
			name:  "blink attribute",
			input: "\x1b[5m|AB",
		},
		{
			name:  "reverse attribute",
			input: "\x1b[7m|AB",
		},
		{
			name:  "hidden attribute",
			input: "\x1b[8m|AB",
		},
		{
			name:  "reset bold with 22",
			input: "\x1b[1m\x1b[22m|AB",
		},
		{
			name:  "reset italic with 23",
			input: "\x1b[3m\x1b[23m|AB",
		},
		{
			name:  "reset underline with 24",
			input: "\x1b[4m\x1b[24m|AB",
		},
		{
			name:  "reset blink with 25",
			input: "\x1b[5m\x1b[25m|AB",
		},
		{
			name:  "reset reverse with 27",
			input: "\x1b[7m\x1b[27m|AB",
		},
		{
			name:  "reset hidden with 28",
			input: "\x1b[8m\x1b[28m|AB",
		},
		{
			name:  "default foreground 39",
			input: "\x1b[31m\x1b[39m|AB",
		},
		{
			name:  "default background 49",
			input: "\x1b[41m\x1b[49m|AB",
		},
		{
			name:  "bright foreground 90-97",
			input: "\x1b[91m|AB",
		},
		{
			name:  "bright background 100-107",
			input: "\x1b[104m|AB",
		},
		{
			name:  "background color 40-47",
			input: "\x1b[42m|AB",
		},
		{
			name:  "SGR reset with no params (ESC[m)",
			input: "\x1b[1;31m\x1b[m|AB",
		},
		{
			name:  "multiple SGR params in one sequence",
			input: "\x1b[1;3;4;31;42m|AB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ProcessAnsiAndExtractCoords([]byte(tt.input), OutputModeCP437)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Verify field was recorded
			if _, ok := result.FieldCoords["AB"]; !ok {
				t.Error("expected field coord 'AB' to be recorded")
			}
			// Verify a color was recorded (may be empty for reset cases)
			if _, ok := result.FieldColors["AB"]; !ok {
				t.Error("expected field color 'AB' to be recorded")
			}
		})
	}
}

func TestProcessAnsiAndExtractCoords_CursorMovements(t *testing.T) {
	tests := []struct {
		name  string
		input string
		wantX int
		wantY int
	}{
		{
			name:  "cursor up",
			input: "\x1b[5;10H\x1b[2A|AB",
			wantX: 10,
			wantY: 3,
		},
		{
			name:  "cursor up clamped to 1",
			input: "\x1b[1;5H\x1b[10A|AB",
			wantX: 5,
			wantY: 1,
		},
		{
			name:  "cursor down",
			input: "\x1b[1;1H\x1b[3B|AB",
			wantX: 1,
			wantY: 4,
		},
		{
			name:  "cursor forward",
			input: "\x1b[1;1H\x1b[5C|AB",
			wantX: 6,
			wantY: 1,
		},
		{
			name:  "cursor back",
			input: "\x1b[1;10H\x1b[3D|AB",
			wantX: 7,
			wantY: 1,
		},
		{
			name:  "cursor back clamped to 1",
			input: "\x1b[1;2H\x1b[10D|AB",
			wantX: 1,
			wantY: 1,
		},
		{
			name:  "cursor position with f terminator",
			input: "\x1b[3;7f|AB",
			wantX: 7,
			wantY: 3,
		},
		{
			name:  "cursor position defaults (ESC[H)",
			input: "\x1b[5;5H\x1b[H|AB",
			wantX: 1,
			wantY: 1,
		},
		{
			name:  "cursor up default param (no number)",
			input: "\x1b[3;5H\x1b[A|AB",
			wantX: 5,
			wantY: 2,
		},
		{
			name:  "cursor down default param",
			input: "\x1b[1;5H\x1b[B|AB",
			wantX: 5,
			wantY: 2,
		},
		{
			name:  "cursor forward default param",
			input: "\x1b[1;5H\x1b[C|AB",
			wantX: 6,
			wantY: 1,
		},
		{
			name:  "cursor back default param",
			input: "\x1b[1;5H\x1b[D|AB",
			wantX: 4,
			wantY: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ProcessAnsiAndExtractCoords([]byte(tt.input), OutputModeCP437)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			coord, ok := result.FieldCoords["AB"]
			if !ok {
				t.Fatal("expected field coord 'AB' to be recorded")
			}
			if coord.X != tt.wantX || coord.Y != tt.wantY {
				t.Errorf("field coord AB = (%d, %d), want (%d, %d)", coord.X, coord.Y, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestProcessAnsiAndExtractCoords_SaveRestoreCursor(t *testing.T) {
	// ESC[s and ESC[u are logged but otherwise ignored
	input := []byte("\x1b[sHello\x1b[u|AB")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	coord, ok := result.FieldCoords["AB"]
	if !ok {
		t.Fatal("expected field coord 'AB' to be recorded")
	}
	// "Hello" is 5 chars, cursor at X=6 after save/restore (ignored)
	if coord.X != 6 || coord.Y != 1 {
		t.Errorf("field coord AB = (%d, %d), want (6, 1)", coord.X, coord.Y)
	}
}

func TestProcessAnsiAndExtractCoords_DECPrivateMode(t *testing.T) {
	// DEC private mode sequences like ESC[?7h should be parsed without error
	input := []byte("\x1b[?7hHello|AB")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	coord, ok := result.FieldCoords["AB"]
	if !ok {
		t.Fatal("expected field coord 'AB'")
	}
	if coord.X != 6 || coord.Y != 1 {
		t.Errorf("field coord AB = (%d, %d), want (6, 1)", coord.X, coord.Y)
	}
}

func TestProcessAnsiAndExtractCoords_NonCSIEscape(t *testing.T) {
	// ESC(0 for VT100 line drawing and ESC(B for ASCII
	input := []byte("\x1b(0abc\x1b(B|AB")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	coord, ok := result.FieldCoords["AB"]
	if !ok {
		t.Fatal("expected field coord 'AB'")
	}
	// "abc" = 3 chars
	if coord.X != 4 || coord.Y != 1 {
		t.Errorf("field coord AB = (%d, %d), want (4, 1)", coord.X, coord.Y)
	}
}

func TestProcessAnsiAndExtractCoords_NonCSIEscapeOther(t *testing.T) {
	// ESC followed by a non-( character, should write ESC byte literally
	input := []byte("\x1bM|AB")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.FieldCoords["AB"]; !ok {
		t.Fatal("expected field coord 'AB'")
	}
}

func TestProcessAnsiAndExtractCoords_IncompleteCSI(t *testing.T) {
	// ESC[ at end of input with no terminator
	input := []byte("Hi\x1b[")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still produce display output for "Hi"
	if !bytes.Contains(result.DisplayBytes, []byte("Hi")) {
		t.Errorf("DisplayBytes should contain 'Hi', got %q", result.DisplayBytes)
	}
}

func TestProcessAnsiAndExtractCoords_CarriageReturn(t *testing.T) {
	// Standalone \r (not \r\n) should reset X to 1
	input := []byte("Hello\rAB|XY")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	coord, ok := result.FieldCoords["XY"]
	if !ok {
		t.Fatal("expected field coord 'XY'")
	}
	// "Hello" then \r resets X to 1, "AB" advances to X=3
	if coord.X != 3 || coord.Y != 1 {
		t.Errorf("field coord XY = (%d, %d), want (3, 1)", coord.X, coord.Y)
	}
}

func TestProcessAnsiAndExtractCoords_DollarCodeEdgeCases(t *testing.T) {
	// Dollar code with invalid digit
	input := []byte("$Xtext")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// $X is not a valid color code (X is not 0-7), should be written literally
	if !bytes.Contains(result.DisplayBytes, []byte("$")) {
		t.Errorf("expected literal $ in output, got %q", result.DisplayBytes)
	}

	// Dollar at end of input
	input2 := []byte("end$")
	result2, err := ProcessAnsiAndExtractCoords(input2, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(result2.DisplayBytes, []byte("end$")) {
		t.Errorf("expected 'end$' in output, got %q", result2.DisplayBytes)
	}
}

func TestProcessAnsiAndExtractCoords_CaretEdgeCases(t *testing.T) {
	// ^g (lowercase)
	input := []byte("^g")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(result.DisplayBytes, []byte{0x07}) {
		t.Errorf("^g should produce BEL, got %q", result.DisplayBytes)
	}

	// ^7 (numeric BEL)
	input2 := []byte("^7")
	result2, err := ProcessAnsiAndExtractCoords(input2, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(result2.DisplayBytes, []byte{0x07}) {
		t.Errorf("^7 should produce BEL, got %q", result2.DisplayBytes)
	}

	// ^X (other control code, written literally)
	input3 := []byte("^X")
	result3, err := ProcessAnsiAndExtractCoords(input3, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(result3.DisplayBytes, []byte("^X")) {
		t.Errorf("^X should be written literally, got %q", result3.DisplayBytes)
	}

	// ^ at end of input
	input4 := []byte("end^")
	result4, err := ProcessAnsiAndExtractCoords(input4, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(result4.DisplayBytes, []byte("end^")) {
		t.Errorf("expected 'end^' in output, got %q", result4.DisplayBytes)
	}
}

func TestProcessAnsiAndExtractCoords_TildeEdgeCases(t *testing.T) {
	// Invalid tilde code (lowercase letters)
	input := []byte("~ab")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ~ab is not valid (need uppercase), should write ~ literally
	if !bytes.Contains(result.DisplayBytes, []byte("~")) {
		t.Errorf("expected literal ~ in output, got %q", result.DisplayBytes)
	}

	// Tilde at end of input (less than 3 bytes remaining)
	input2 := []byte("x~")
	result2, err := ProcessAnsiAndExtractCoords(input2, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(result2.DisplayBytes, []byte("x~")) {
		t.Errorf("expected 'x~' in output, got %q", result2.DisplayBytes)
	}
}

func TestProcessAnsiAndExtractCoords_SingleLetterPlaceholder(t *testing.T) {
	// Single uppercase letter after pipe: |X (not followed by another uppercase)
	input := []byte("|A1")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	coord, ok := result.FieldCoords["A"]
	if !ok {
		t.Fatal("expected single-letter field coord 'A' to be recorded")
	}
	if coord.X != 1 || coord.Y != 1 {
		t.Errorf("field coord A = (%d, %d), want (1, 1)", coord.X, coord.Y)
	}
}

func TestProcessAnsiAndExtractCoords_PipeLiteralFallback(t *testing.T) {
	// Pipe followed by non-uppercase, non-code character
	input := []byte("|!text")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// | should be written literally and advance cursor
	if !bytes.Contains(result.DisplayBytes, []byte("|")) {
		t.Errorf("expected literal | in output, got %q", result.DisplayBytes)
	}
}

func TestProcessAnsiAndExtractCoords_PipeColorUpdatesState(t *testing.T) {
	// Verify that pipe color codes update the SGR state for FieldColors
	// |09 is light blue (bold + blue fg = 1;34)
	input := []byte("|09|AB")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	color, ok := result.FieldColors["AB"]
	if !ok {
		t.Fatal("expected FieldColors for 'AB'")
	}
	// After |09 (bold blue: \x1b[1;34m), should have bold=true, fg=34
	if color == "" {
		t.Error("expected non-empty color after |09")
	}
}

func TestProcessAnsiAndExtractCoords_PipeCLResetsCoords(t *testing.T) {
	// |CR is a known pipe command (newline), not a field placeholder.
	// After |CR the cursor advances to (1,2), so |AB is recorded at (1,2).
	input := []byte("|CR|AB")
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// |CR must NOT be captured as a placeholder
	if _, ok := result.FieldCoords["CR"]; ok {
		t.Error("'CR' should not be recorded as a placeholder (it is a pipe command)")
	}
	coord, ok := result.FieldCoords["AB"]
	if !ok {
		t.Fatal("expected field coord 'AB' to be recorded")
	}
	// After |CR (newline), cursor is at (1,2)
	if coord.X != 1 || coord.Y != 2 {
		t.Errorf("field coord AB = (%d, %d), want (1, 2)", coord.X, coord.Y)
	}
}

func TestProcessAnsiAndExtractCoords_ControlCharsNoAdvance(t *testing.T) {
	// Control characters below 0x20 (except \r, \n) should not advance cursor
	input := []byte{0x01, 0x02, 'A', '|', 'X', 'Y'}
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	coord, ok := result.FieldCoords["XY"]
	if !ok {
		t.Fatal("expected field coord 'XY'")
	}
	// Control chars 0x01, 0x02 don't advance cursor, 'A' advances to X=2
	if coord.X != 2 || coord.Y != 1 {
		t.Errorf("field coord XY = (%d, %d), want (2, 1)", coord.X, coord.Y)
	}
}

func TestProcessAnsiAndExtractCoords_HighByteAdvancesCursor(t *testing.T) {
	// CP437 bytes >= 0x80 should advance cursor
	input := []byte{0xDB, 0xDB, '|', 'A', 'B'}
	result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	coord, ok := result.FieldCoords["AB"]
	if !ok {
		t.Fatal("expected field coord 'AB'")
	}
	// Two high bytes advance cursor to X=3
	if coord.X != 3 || coord.Y != 1 {
		t.Errorf("field coord AB = (%d, %d), want (3, 1)", coord.X, coord.Y)
	}
}

func TestProcessAnsiAndExtractCoords_DollarAllColors(t *testing.T) {
	// Test all valid dollar color codes $0 through $7
	for digit := byte('0'); digit <= byte('7'); digit++ {
		input := []byte{'$', digit, 'X'}
		result, err := ProcessAnsiAndExtractCoords(input, OutputModeCP437)
		if err != nil {
			t.Fatalf("unexpected error for $%c: %v", digit, err)
		}
		// Should contain ANSI color code
		if !bytes.Contains(result.DisplayBytes, []byte("\x1b[")) {
			t.Errorf("$%c: expected ANSI code in output, got %q", digit, result.DisplayBytes)
		}
	}
}
