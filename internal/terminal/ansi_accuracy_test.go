package terminal

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestANSIAccuracy provides focused, accuracy-critical testing for BBS ANSI rendering
// This test validates the 5 most critical aspects for authentic BBS experience:
//   1. ViSiON/2 pipe code accuracy
//   2. CP437 character encoding precision  
//   3. ANSI cursor positioning validation
//   4. Character set mode switching
//   5. SAUCE metadata parsing
func TestANSIAccuracy(t *testing.T) {
	t.Run("PipeCodeAccuracy", testPipeCodeAccuracy)
	t.Run("CP437EncodingPrecision", testCP437EncodingPrecision) 
	t.Run("CursorPositioningValidation", testCursorPositioningValidation)
	t.Run("CharacterSetModeSwitching", testCharacterSetModeSwitching)
	t.Run("SAUCEMetadataParsing", testSAUCEMetadataParsing)
}

// testPipeCodeAccuracy validates ViSiON/2 pipe codes produce correct ANSI sequences
func testPipeCodeAccuracy(t *testing.T) {
	testCases := []struct {
		name          string
		pipeCode      string
		expectedANSI  string
		description   string
	}{
		// Foreground colors |00-|15
		{"Black", "|00", "\x1b[30m", "Black foreground"},
		{"Blue", "|01", "\x1b[34m", "Blue foreground"},
		{"Green", "|02", "\x1b[32m", "Green foreground"},
		{"Cyan", "|03", "\x1b[36m", "Cyan foreground"},
		{"Red", "|04", "\x1b[31m", "Red foreground"},
		{"Magenta", "|05", "\x1b[35m", "Magenta foreground"},
		{"Brown", "|06", "\x1b[33m", "Brown/Yellow foreground"},
		{"LightGray", "|07", "\x1b[37m", "Light Gray foreground"},
		{"DarkGray", "|08", "\x1b[1;30m", "Dark Gray (bright black)"},
		{"LightBlue", "|09", "\x1b[1;34m", "Light Blue"},
		{"LightGreen", "|10", "\x1b[1;32m", "Light Green"},
		{"LightCyan", "|11", "\x1b[1;36m", "Light Cyan"},
		{"LightRed", "|12", "\x1b[1;31m", "Light Red"},
		{"LightMagenta", "|13", "\x1b[1;35m", "Light Magenta"},
		{"Yellow", "|14", "\x1b[1;33m", "Yellow"},
		{"White", "|15", "\x1b[1;37m", "White"},
		
		// Background colors |B0-|B7
		{"BlackBG", "|B0", "\x1b[40m", "Black background"},
		{"BlueBG", "|B1", "\x1b[44m", "Blue background"},
		{"GreenBG", "|B2", "\x1b[42m", "Green background"},
		{"CyanBG", "|B3", "\x1b[46m", "Cyan background"},
		{"RedBG", "|B4", "\x1b[41m", "Red background"},
		{"MagentaBG", "|B5", "\x1b[45m", "Magenta background"},
		{"YellowBG", "|B6", "\x1b[43m", "Yellow background"},
		{"WhiteBG", "|B7", "\x1b[47m", "White background"},
		
		// Special codes
		{"Reset", "|RS", "\x1b[0m", "Reset all attributes"},
		{"Clear", "|CL", "\x1b[2J\x1b[H", "Clear screen and home cursor"},
		{"CarriageReturn", "|CR", "\r", "Carriage return"},
		{"LineFeed", "|LF", "\n", "Line feed"},
		{"Blink", "|BL", "\x1b[5m", "Blink"},
		{"Reverse", "|RV", "\x1b[7m", "Reverse video"},
	}

	charset := NewCharsetHandler()
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte(tc.pipeCode + "TEST")
			result := charset.ProcessPipeCodes(input)
			
			// Extract the ANSI sequence part (before "TEST")
			resultStr := string(result)
			ansiPart := strings.Replace(resultStr, "TEST", "", 1)
			
			if ansiPart != tc.expectedANSI {
				t.Errorf("%s: pipe code %s\n  Expected ANSI: %q (% x)\n  Got ANSI:      %q (% x)\n  Description: %s", 
					tc.name, tc.pipeCode, tc.expectedANSI, []byte(tc.expectedANSI), 
					ansiPart, []byte(ansiPart), tc.description)
			}
		})
	}
}

// testCP437EncodingPrecision validates CP437 character conversion accuracy
func testCP437EncodingPrecision(t *testing.T) {
	charset := NewCharsetHandler()
	
	// Test critical CP437 characters that must render correctly for BBS authenticity
	criticalChars := map[byte]rune{
		// Box drawing characters (essential for BBS menus)
		0xB0: 0x2591, // Light shade ░
		0xB1: 0x2592, // Medium shade ▒ 
		0xB2: 0x2593, // Dark shade ▓
		0xB3: 0x2502, // Vertical line │
		0xB4: 0x2524, // Right tee ┤
		0xB5: 0x2561, // Right tee with double vertical ╡
		0xB6: 0x2562, // Right tee with double horizontal ╢
		0xB7: 0x2556, // Top right corner with double vertical ╖
		0xB8: 0x2555, // Top right corner with double horizontal ╕
		0xB9: 0x2563, // Right tee with double ╣
		0xBA: 0x2551, // Double vertical ║
		0xBB: 0x2557, // Top right corner with double ╗
		0xBC: 0x255D, // Bottom right corner with double ╝
		0xBD: 0x255C, // Bottom right corner with double horizontal ╜
		0xBE: 0x255B, // Bottom right corner with double vertical ╛
		0xBF: 0x2510, // Top right corner ┐
		0xC0: 0x2514, // Bottom left corner └
		0xC1: 0x2534, // Bottom tee ┴
		0xC2: 0x252C, // Top tee ┬
		0xC3: 0x251C, // Left tee ├
		0xC4: 0x2500, // Horizontal ─
		0xC5: 0x253C, // Cross ┼
		0xDA: 0x250C, // Top left corner ┌
		0xDB: 0x2588, // Full block █
		0xDC: 0x2584, // Lower half block ▄
		0xDD: 0x258C, // Left half block ▌
		0xDE: 0x2590, // Right half block ▐
		0xDF: 0x2580, // Upper half block ▀
		
		// Special symbols often used in BBS art
		0x01: 0x263A, // Smiling face ☺
		0x02: 0x263B, // Frowning face ☻
		0x03: 0x2665, // Heart ♥
		0x04: 0x2666, // Diamond ♦
		0x05: 0x2663, // Club ♣
		0x06: 0x2660, // Spade ♠
		0x07: 0x2022, // Bullet •
		0x10: 0x25BA, // Right triangle ►
		0x11: 0x25C4, // Left triangle ◄
		0x1E: 0x25B2, // Up triangle ▲
		0x1F: 0x25BC, // Down triangle ▼
	}
	
	for cp437Byte, expectedUnicode := range criticalChars {
		t.Run(fmt.Sprintf("CP437_0x%02X", cp437Byte), func(t *testing.T) {
			actualUnicode := charset.ConvertCP437ByteToUTF8(cp437Byte)
			
			if actualUnicode != expectedUnicode {
				t.Errorf("CP437 byte 0x%02X conversion failed:\n  Expected Unicode: U+%04X (%c)\n  Got Unicode:      U+%04X (%c)", 
					cp437Byte, expectedUnicode, expectedUnicode, actualUnicode, actualUnicode)
			}
		})
	}
	
	// Test round-trip conversion for ASCII characters (should be unchanged)
	for i := 0x20; i <= 0x7E; i++ {
		t.Run(fmt.Sprintf("ASCII_0x%02X", i), func(t *testing.T) {
			original := byte(i)
			unicode := charset.ConvertCP437ByteToUTF8(original)
			
			if unicode != rune(original) {
				t.Errorf("ASCII character 0x%02X should remain unchanged, got U+%04X", original, unicode)
			}
		})
	}
}

// testCursorPositioningValidation ensures ANSI cursor movements produce exact coordinates
func testCursorPositioningValidation(t *testing.T) {
	testCases := []struct {
		name         string
		ansiInput    []byte
		expectedX    int
		expectedY    int
		description  string
	}{
		{"Home", []byte("\x1b[H"), 0, 0, "Cursor home position"},
		{"Position_5_10", []byte("\x1b[5;10H"), 9, 4, "Cursor to row 5, col 10 (0-based: 4,9)"},
		{"Position_1_1", []byte("\x1b[1;1H"), 0, 0, "Cursor to row 1, col 1 (0-based: 0,0)"},
		{"CursorUp", []byte("\x1b[10;10H\x1b[3A"), 9, 6, "Move to 10,10 then up 3 lines"},
		{"CursorDown", []byte("\x1b[5;5H\x1b[2B"), 4, 6, "Move to 5,5 then down 2 lines"},
		{"CursorRight", []byte("\x1b[1;1H\x1b[4C"), 4, 0, "Move to 1,1 then right 4 columns"},
		{"CursorLeft", []byte("\x1b[1;10H\x1b[3D"), 6, 0, "Move to 1,10 then left 3 columns"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewANSIParser(80, 25)
			
			var finalX, finalY int
			parser.SetCallbacks(
				nil, // onText
				func(x, y int) { finalX, finalY = x, y }, // onCursor
				nil, // onGraphics
				nil, // onClear
				nil, // onScroll
			)
			
			err := parser.ParseBytes(tc.ansiInput)
			if err != nil {
				t.Fatalf("Failed to parse ANSI input: %v", err)
			}
			
			if finalX != tc.expectedX || finalY != tc.expectedY {
				t.Errorf("%s: cursor positioning failed\n  Expected: (%d, %d)\n  Got:      (%d, %d)\n  Description: %s",
					tc.name, tc.expectedX, tc.expectedY, finalX, finalY, tc.description)
			}
		})
	}
}

// testCharacterSetModeSwitching validates different output modes work correctly
func testCharacterSetModeSwitching(t *testing.T) {
	testData := []byte{0xB0, 0xB1, 0xB2} // Light, medium, dark shade
	
	testModes := []struct {
		name        string
		outputMode  OutputMode
		expectUTF8  bool
		expectFallback bool
	}{
		{"UTF8Mode", OutputModeUTF8, true, false},
		{"CP437Mode", OutputModeCP437, false, false},
		{"AutoMode", OutputModeAuto, true, false}, // Will default to UTF8 in test environment
	}
	
	for _, tm := range testModes {
		t.Run(tm.name, func(t *testing.T) {
			var buf bytes.Buffer
			capabilities := Capabilities{
				SupportsUTF8: tm.expectUTF8,
				Width: 80,
				Height: 25,
				TerminalType: TerminalXTerm,
			}
			
			renderer := NewArtRenderer(&buf, capabilities, tm.outputMode)
			
			err := renderer.RenderAnsiBytes(testData)
			if err != nil {
				t.Fatalf("Failed to render: %v", err)
			}
			
			output := buf.Bytes()
			
			// Verify output characteristics based on mode
			if tm.expectUTF8 {
				// UTF-8 output should contain multibyte sequences for box drawing
				if len(output) <= len(testData) {
					t.Errorf("UTF-8 mode should produce expanded output, got %d bytes for %d input bytes", 
						len(output), len(testData))
				}
			}
			
			// Verify we got some output
			if len(output) == 0 {
				t.Error("No output produced")
			}
		})
	}
}

// testSAUCEMetadataParsing validates SAUCE record parsing and ice colors
func testSAUCEMetadataParsing(t *testing.T) {
	// Create a proper SAUCE record (128 bytes total)
	sauceData := make([]byte, 128)
	copy(sauceData[0:5], "SAUCE")           // Signature (5 bytes)
	copy(sauceData[5:7], "00")              // Version (2 bytes)
	copy(sauceData[7:42], "Test ANSI Art")  // Title (35 bytes)
	copy(sauceData[42:62], "Test Artist")   // Author (20 bytes)
	copy(sauceData[62:82], "Test Group")    // Group (20 bytes) 
	copy(sauceData[82:90], "20250910")      // Date CCYYMMDD (8 bytes)
	// File size (4 bytes at offset 90-93) - leave as 0
	// Data type (1 byte at offset 94) - leave as 0
	// File type (1 byte at offset 95) - leave as 0
	// TInfo1/2 (4 bytes at offset 96-99) - leave as 0
	// TInfo3/4 (4 bytes at offset 100-103) - leave as 0
	// Comments (1 byte at offset 104) - leave as 0
	sauceData[105] = 0x01 // Flags (1 byte at offset 105) - set ice colors flag
	// Filler (22 bytes at offset 106-127) - leave as 0
	
	artData := []byte("Test ANSI content")
	fullData := append(artData, sauceData...)
	
	var buf bytes.Buffer
	capabilities := Capabilities{Width: 80, Height: 25, TerminalType: TerminalXTerm}
	renderer := NewArtRenderer(&buf, capabilities, OutputModeUTF8)
	
	err := renderer.RenderAnsiBytes(fullData)
	if err != nil {
		t.Fatalf("Failed to render with SAUCE: %v", err)
	}
	
	sauce := renderer.GetSAUCEInfo()
	if sauce == nil {
		t.Fatal("SAUCE metadata was not parsed")
	}
	
	expectedTitle := "Test ANSI Art"
	expectedAuthor := "Test Artist"
	
	// SAUCE fields are null-padded, so we need to trim null bytes as well as spaces
	actualTitle := strings.TrimRight(strings.TrimSpace(sauce.Title), "\x00")
	actualAuthor := strings.TrimRight(strings.TrimSpace(sauce.Author), "\x00")
	
	if actualTitle != expectedTitle {
		t.Errorf("SAUCE title mismatch:\n  Expected: %q\n  Got:      %q", expectedTitle, actualTitle)
	}
	
	if actualAuthor != expectedAuthor {
		t.Errorf("SAUCE author mismatch:\n  Expected: %q\n  Got:      %q", expectedAuthor, actualAuthor)
	}
	
	if !sauce.IceColors {
		t.Errorf("Ice colors flag should be set (flags byte was 0x%02x)", sauceData[105])
	}
}

// TestANSIRealWorldCompatibility tests against actual BBS ANSI files
func TestANSIRealWorldCompatibility(t *testing.T) {
	// Test against actual ANSI files from the menus directory
	ansiFiles := []string{
		"../../menus/v3/ansi/CONFIG.ANS",
		"../../menus/v3/ansi/SYSSTATS.ANS", 
		"../../menus/v3/ansi/FASTLOGN.ANS",
	}
	
	for _, filename := range ansiFiles {
		t.Run(filename, func(t *testing.T) {
			// Check if file exists
			if _, err := os.Stat(filename); os.IsNotExist(err) {
				t.Skipf("ANSI file %s not found, skipping", filename)
				return
			}
			
			data, err := os.ReadFile(filename)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", filename, err)
			}
			
			var buf bytes.Buffer
			capabilities := Capabilities{
				SupportsUTF8: true,
				Width: 80,
				Height: 25,
				TerminalType: TerminalXTerm,
			}
			
			renderer := NewArtRenderer(&buf, capabilities, OutputModeUTF8)
			
			// Should not crash or error on real ANSI files
			err = renderer.RenderAnsiBytes(data)
			if err != nil {
				t.Errorf("Failed to render real ANSI file %s: %v", filename, err)
			}
			
			// Should produce some output
			if buf.Len() == 0 {
				t.Errorf("No output produced for %s", filename)
			}
			
			// Output should be at least as long as input (ANSI codes may expand)
			if buf.Len() < len(data)/2 {
				t.Errorf("Output suspiciously short for %s: got %d bytes from %d input bytes", 
					filename, buf.Len(), len(data))
			}
		})
	}
}

// BenchmarkANSIProcessing benchmarks ANSI processing performance
func BenchmarkANSIProcessing(b *testing.B) {
	// Create test data with pipe codes and CP437 characters
	testData := []byte("|04Hello |02World |15with |B1colors|RS and |08box chars: \xB0\xB1\xB2\xDB")
	
	var buf bytes.Buffer
	capabilities := Capabilities{Width: 80, Height: 25, TerminalType: TerminalXTerm}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		renderer := NewArtRenderer(&buf, capabilities, OutputModeUTF8)
		renderer.RenderAnsiBytes(testData)
	}
}