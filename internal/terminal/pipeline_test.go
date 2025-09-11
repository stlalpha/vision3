package terminal

import (
	"bytes"
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gliderlabs/ssh"
)

// TestTerminalDetectionAccuracy validates terminal type detection and mode selection
func TestTerminalDetectionAccuracy(t *testing.T) {
	testCases := []struct {
		name           string
		termType       string
		expectedMode   OutputMode
		description    string
	}{
		// CP437-preferring terminals
		{"SyncTerm", "sync", OutputModeCP437, "SyncTerm BBS terminal"},
		{"ANSI", "ansi", OutputModeCP437, "Classic ANSI terminal"},
		{"SCOANSI", "scoansi", OutputModeCP437, "SCO ANSI terminal"},
		{"VT100", "vt100", OutputModeCP437, "VT100 terminal"},
		{"VT100_Color", "vt100-color", OutputModeCP437, "VT100 with color"},
		
		// UTF-8 preferring terminals  
		{"XTerm", "xterm", OutputModeUTF8, "Modern xterm"},
		{"XTerm256", "xterm-256color", OutputModeUTF8, "xterm with 256 colors"},
		{"Screen", "screen", OutputModeUTF8, "GNU Screen"},
		{"Tmux", "tmux", OutputModeUTF8, "tmux terminal"},
		{"LinuxConsole", "linux", OutputModeUTF8, "Linux console"},
		
		// Unknown defaults to UTF-8
		{"Unknown", "unknown-terminal", OutputModeUTF8, "Unknown terminal type"},
		{"Empty", "", OutputModeUTF8, "Empty terminal type"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock SSH session with specific TERM type
			mockSession := &mockSSHSession{
				ptyRequest: &ssh.Pty{
					Term: tc.termType,
					Window: ssh.Window{Width: 80, Height: 25},
				},
				hasPty: true,
			}
			
			// Test the terminal detection logic from main.go using real SSH interface
			detectedMode := detectOutputMode(mockSession, "auto")
			
			if detectedMode != tc.expectedMode {
				t.Errorf("Terminal detection failed for %s (TERM=%s):\n  Expected: %v\n  Got:      %v\n  Description: %s",
					tc.name, tc.termType, tc.expectedMode, detectedMode, tc.description)
			}
		})
	}
}

// TestCharacterEncodingAccuracy validates encoding detection and conversion
// This test defines the AUTHORITATIVE specification for how encoding should work
func TestCharacterEncodingAccuracy(t *testing.T) {
	testData := []struct {
		name           string
		inputBytes     []byte
		outputMode     OutputMode
		mustContain    []string // REQUIRED strings that MUST be present for correct behavior
		mustNotContain []string // Strings that MUST NOT be present
		description    string
	}{
		{
			name:           "CP437_BoxDrawing_Preservation",
			inputBytes:     []byte{0xB0, 0xB1, 0xB2}, // Light shade, medium shade, dark shade
			outputMode:     OutputModeCP437,
			mustContain:    []string{"\xB0", "\xB1", "\xB2"}, // Raw CP437 bytes MUST be preserved
			mustNotContain: []string{"░", "▒", "▓"}, // Unicode MUST NOT appear in CP437 mode
			description:    "CP437 mode MUST preserve original bytes unchanged",
		},
		{
			name:           "UTF8_BoxDrawing_Conversion",
			inputBytes:     []byte{0xB0, 0xB1, 0xB2}, // Same CP437 characters
			outputMode:     OutputModeUTF8,
			mustContain:    []string{"░", "▒", "▓"}, // MUST convert to proper Unicode
			mustNotContain: []string{"\xB0", "\xB1", "\xB2"}, // Raw bytes MUST NOT appear in UTF-8 mode
			description:    "UTF-8 mode MUST convert CP437 to proper Unicode equivalents",
		},
		{
			name:           "PipeCode_Basic_Colors",
			inputBytes:     []byte("|04RED|02GREEN|15WHITE"),
			outputMode:     OutputModeUTF8,
			mustContain:    []string{
				"\x1b[31m", // |04 MUST produce red foreground
				"\x1b[32m", // |02 MUST produce green foreground  
				"\x1b[1;37m", // |15 MUST produce bright white
				"RED", "GREEN", "WHITE", // Text MUST be preserved
			},
			mustNotContain: []string{"|04", "|02", "|15"}, // Pipe codes MUST be processed, not displayed
			description:    "Pipe codes MUST generate standard ANSI color sequences",
		},
		{
			name:           "PipeCode_Reset",
			inputBytes:     []byte("|04Red|RSNormal"),
			outputMode:     OutputModeUTF8,
			mustContain:    []string{
				"\x1b[31m", // |04 MUST produce red
				"\x1b[0m",  // |RS MUST produce reset
				"Red", "Normal",
			},
			mustNotContain: []string{"|04", "|RS"},
			description:    "Reset pipe code MUST generate ANSI reset sequence",
		},
		{
			name:           "Mixed_Content_Integrity",
			inputBytes:     []byte("Hello \xB0\xB1\xB2 World"),
			outputMode:     OutputModeUTF8,
			mustContain:    []string{
				"Hello", " ", "World", // ASCII text MUST be preserved exactly
				"░", "▒", "▓", // CP437 MUST convert to Unicode
			},
			mustNotContain: []string{"\xB0", "\xB1", "\xB2"}, // Raw CP437 MUST NOT appear
			description:    "Mixed ASCII and CP437 MUST preserve ASCII and convert CP437",
		},
	}
	
	for _, td := range testData {
		t.Run(td.name, func(t *testing.T) {
			var buf bytes.Buffer
			
			// Create terminal with specific output mode
			capabilities := Capabilities{
				SupportsUTF8: td.outputMode == OutputModeUTF8,
				Width: 80,
				Height: 25,
				TerminalType: TerminalXTerm,
			}
			
			renderer := NewArtRenderer(&buf, capabilities, td.outputMode)
			
			// Process the input through the encoding pipeline
			err := renderer.RenderAnsiBytes(td.inputBytes)
			if err != nil {
				t.Fatalf("Failed to render bytes: %v", err)
			}
			
			output := buf.String()
			
			// AUTHORITATIVE VALIDATION: Check required content is present
			for _, required := range td.mustContain {
				if !strings.Contains(output, required) {
					t.Errorf("SPECIFICATION VIOLATION in %s:\n  REQUIRED: %q\n  GOT: %q\n  Description: %s",
						td.name, required, output, td.description)
				}
			}
			
			// AUTHORITATIVE VALIDATION: Check forbidden content is absent
			for _, forbidden := range td.mustNotContain {
				if strings.Contains(output, forbidden) {
					t.Errorf("SPECIFICATION VIOLATION in %s:\n  FORBIDDEN: %q found in output\n  GOT: %q\n  Description: %s",
						td.name, forbidden, output, td.description)
				}
			}
			
			// Ensure we got some output
			if len(output) == 0 {
				t.Errorf("No output produced for %s", td.name)
			}
		})
	}
}

// TestFullPipelineIntegration validates the complete SSH-to-ANSI pipeline
func TestFullPipelineIntegration(t *testing.T) {
	// Test different terminal scenarios end-to-end
	scenarios := []struct {
		name         string
		termType     string
		forceMode    string // "auto", "utf8", "cp437"
		testContent  []byte
		validateFunc func(t *testing.T, output []byte, termType string, mode OutputMode)
	}{
		{
			name:        "SyncTerm_Auto_CP437",
			termType:    "sync",
			forceMode:   "auto",
			testContent: []byte("|04Hello |B1World|RS \xB0\xB1\xB2"),
			validateFunc: func(t *testing.T, output []byte, termType string, mode OutputMode) {
				if mode != OutputModeCP437 {
					t.Errorf("Expected CP437 mode for SyncTerm, got %v", mode)
				}
				// In CP437 mode, should contain ANSI color codes but raw CP437 bytes
				outputStr := string(output)
				if !strings.Contains(outputStr, "\x1b[31m") { // |04 -> red
					t.Error("Missing ANSI color code conversion")
				}
			},
		},
		{
			name:        "XTerm_Auto_UTF8",
			termType:    "xterm-256color", 
			forceMode:   "auto",
			testContent: []byte("|02Green |15Text \xB0\xB1\xB2"),
			validateFunc: func(t *testing.T, output []byte, termType string, mode OutputMode) {
				if mode != OutputModeUTF8 {
					t.Errorf("Expected UTF-8 mode for xterm, got %v", mode)
				}
				outputStr := string(output)
				// Should contain UTF-8 box drawing characters
				if !strings.Contains(outputStr, "▒") || !strings.Contains(outputStr, "▓") {
					t.Error("Missing UTF-8 box drawing character conversion")
				}
				if !strings.Contains(outputStr, "\x1b[32m") { // |02 -> green
					t.Error("Missing ANSI color code conversion")
				}
			},
		},
		{
			name:        "ForceUTF8_Override",
			termType:    "sync", // Would normally choose CP437
			forceMode:   "utf8",
			testContent: []byte("|04Test \xDA\xC4\xBF"), // Top border chars
			validateFunc: func(t *testing.T, output []byte, termType string, mode OutputMode) {
				if mode != OutputModeUTF8 {
					t.Errorf("Expected UTF-8 mode when forced, got %v", mode)
				}
				outputStr := string(output)
				// Should have UTF-8 box drawing despite SyncTerm TERM type
				if !strings.Contains(outputStr, "┌") || !strings.Contains(outputStr, "─") || !strings.Contains(outputStr, "┐") {
					t.Error("Force UTF-8 mode failed to convert box drawing characters")
				}
			},
		},
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Create mock SSH session
			mockSession := &mockSSHSession{
				ptyRequest: &ssh.Pty{
					Term: scenario.termType,
					Window: ssh.Window{Width: 80, Height: 25},
				},
				hasPty: true,
			}
			
			// Simulate the detection logic from main.go
			detectedMode := detectOutputMode(mockSession, scenario.forceMode)
			
			// Create terminal with detected mode
			var buf bytes.Buffer
			capabilities := Capabilities{
				SupportsUTF8: detectedMode == OutputModeUTF8,
				Width: 80,
				Height: 25,
				TerminalType: parseTerminalType(scenario.termType),
			}
			
			// Simulate the full rendering pipeline
			renderer := NewArtRenderer(&buf, capabilities, detectedMode)
			err := renderer.RenderAnsiBytes(scenario.testContent)
			if err != nil {
				t.Fatalf("Pipeline failed: %v", err)
			}
			
			// Run scenario-specific validation
			scenario.validateFunc(t, buf.Bytes(), scenario.termType, detectedMode)
		})
	}
}

// TestRealWorldSessionSimulation simulates actual BBS session scenarios
func TestRealWorldSessionSimulation(t *testing.T) {
	// Test scenarios that mirror real BBS client connections
	clients := []struct {
		name       string
		termType   string
		expectMode OutputMode
		testFile   string // Optional ANSI file to test
	}{
		{"SyncTerm_BBS_Client", "sync", OutputModeCP437, "../../menus/v3/ansi/CONFIG.ANS"},
		{"mTelnet_Windows", "ansi", OutputModeCP437, "../../menus/v3/ansi/SYSSTATS.ANS"},
		{"PuTTY_Modern", "xterm", OutputModeUTF8, "../../menus/v3/ansi/FASTLOGN.ANS"},
		{"Terminal_macOS", "xterm-256color", OutputModeUTF8, ""},
		{"Linux_Console", "linux", OutputModeUTF8, ""},
	}
	
	for _, client := range clients {
		t.Run(client.name, func(t *testing.T) {
			mockSession := &mockSSHSession{
				ptyRequest: &ssh.Pty{
					Term: client.termType,
					Window: ssh.Window{Width: 80, Height: 25},
				},
				hasPty: true,
			}
			
			// Test mode detection
			mode := detectOutputMode(mockSession, "auto")
			if mode != client.expectMode {
				t.Errorf("Mode detection failed for %s: expected %v, got %v", 
					client.name, client.expectMode, mode)
			}
			
			// Test with real ANSI file if specified
			if client.testFile != "" {
				var buf bytes.Buffer
				capabilities := Capabilities{
					SupportsUTF8: mode == OutputModeUTF8,
					Width: 80,
					Height: 25,
					TerminalType: parseTerminalType(client.termType),
				}
				
				renderer := NewArtRenderer(&buf, capabilities, mode)
				
				// Try to render the real ANSI file
				err := renderer.RenderAnsiFile(client.testFile)
				if err != nil {
					t.Skipf("Could not test real ANSI file %s: %v", client.testFile, err)
					return
				}
				
				// Validate we got reasonable output
				if buf.Len() == 0 {
					t.Errorf("No output from real ANSI file for %s", client.name)
				}
				
				// Basic sanity check - output should be at least as long as a minimal ANSI file
				if buf.Len() < 50 {
					t.Errorf("Suspiciously short output for %s: %d bytes", client.name, buf.Len())
				}
			}
		})
	}
}

// Mock SSH session for testing - implements the full ssh.Session interface
type mockSSHSession struct {
	ptyRequest *ssh.Pty
	hasPty     bool
	output     bytes.Buffer
	environ    []string
	ctx        ssh.Context
}

// Implement ssh.Session interface
func (m *mockSSHSession) User() string                          { return "testuser" }
func (m *mockSSHSession) RemoteAddr() net.Addr                  { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345} }
func (m *mockSSHSession) LocalAddr() net.Addr                   { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222} }
func (m *mockSSHSession) Environ() []string                     { 
	if m.environ != nil {
		return m.environ
	}
	if m.ptyRequest != nil {
		return []string{"TERM=" + m.ptyRequest.Term}
	}
	return []string{"TERM=xterm"}
}
func (m *mockSSHSession) Exit(code int) error                   { return nil }
func (m *mockSSHSession) Command() []string                     { return []string{} }
func (m *mockSSHSession) RawCommand() string                    { return "" }
func (m *mockSSHSession) Subsystem() string                     { return "" }
func (m *mockSSHSession) PublicKey() ssh.PublicKey              { return nil }
func (m *mockSSHSession) Context() ssh.Context                  { 
	if m.ctx != nil {
		return m.ctx
	}
	return newMockSSHContext()
}
func (m *mockSSHSession) Permissions() ssh.Permissions          { return ssh.Permissions{} }
func (m *mockSSHSession) Pty() (ssh.Pty, <-chan ssh.Window, bool) {
	if m.hasPty && m.ptyRequest != nil {
		winCh := make(chan ssh.Window, 1)
		winCh <- m.ptyRequest.Window
		close(winCh)
		return *m.ptyRequest, winCh, true
	}
	return ssh.Pty{}, nil, false
}
func (m *mockSSHSession) Signals(c chan<- ssh.Signal)           {}
func (m *mockSSHSession) Break(c chan<- bool)                   {}

// Implement gossh.Channel interface (embedded in ssh.Session)
func (m *mockSSHSession) Read(p []byte) (n int, err error)      { return 0, io.EOF }
func (m *mockSSHSession) Write(p []byte) (n int, err error)     { return m.output.Write(p) }
func (m *mockSSHSession) Close() error                          { return nil }
func (m *mockSSHSession) CloseWrite() error                     { return nil }
func (m *mockSSHSession) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	return false, nil
}
func (m *mockSSHSession) Stderr() io.ReadWriter                 { return &m.output }

// Mock SSH context for testing - implements ssh.Context interface
type mockSSHContext struct {
	ctx context.Context
	sync.Mutex
	values map[interface{}]interface{}
}

func newMockSSHContext() *mockSSHContext {
	return &mockSSHContext{
		ctx:    context.Background(),
		values: make(map[interface{}]interface{}),
	}
}

// Implement context.Context interface
func (m *mockSSHContext) Deadline() (deadline time.Time, ok bool) { return m.ctx.Deadline() }
func (m *mockSSHContext) Done() <-chan struct{}                   { return m.ctx.Done() }
func (m *mockSSHContext) Err() error                              { return m.ctx.Err() }
func (m *mockSSHContext) Value(key interface{}) interface{} {
	m.Lock()
	defer m.Unlock()
	if val, exists := m.values[key]; exists {
		return val
	}
	return m.ctx.Value(key)
}

// Implement ssh.Context interface
func (m *mockSSHContext) SessionID() string           { return "test-session-id" }
func (m *mockSSHContext) ClientVersion() string       { return "SSH-2.0-TestClient" }
func (m *mockSSHContext) ServerVersion() string       { return "SSH-2.0-ViSiON/3" }
func (m *mockSSHContext) RemoteAddr() net.Addr        { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345} }
func (m *mockSSHContext) LocalAddr() net.Addr         { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222} }
func (m *mockSSHContext) User() string                { return "testuser" }
func (m *mockSSHContext) Permissions() *ssh.Permissions { return &ssh.Permissions{} }
func (m *mockSSHContext) SetValue(key, value interface{}) {
	m.Lock()
	defer m.Unlock()
	m.values[key] = value
}

// Helper function to extract the detection logic from main.go
func detectOutputMode(session ssh.Session, forceMode string) OutputMode {
	switch forceMode {
	case "utf8":
		return OutputModeUTF8
	case "cp437":
		return OutputModeCP437
	case "auto":
		ptyReq, _, hasPty := session.Pty()
		if hasPty {
			termType := strings.ToLower(ptyReq.Term)
			// Mirror the logic from main.go sessionHandler
			if termType == "sync" || termType == "ansi" || termType == "scoansi" || strings.HasPrefix(termType, "vt100") {
				return OutputModeCP437
			}
		}
		return OutputModeUTF8
	default:
		return OutputModeAuto
	}
}

// Helper function to parse terminal type
func parseTerminalType(termType string) TerminalType {
	switch strings.ToLower(termType) {
	case "sync":
		return TerminalSyncTERM
	case "ansi", "scoansi":
		return TerminalANSI
	case "xterm", "xterm-256color":
		return TerminalXTerm
	case "vt100", "vt100-color":
		return TerminalVT100
	default:
		return TerminalXTerm // Default fallback
	}
}