package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
	sshterminal "golang.org/x/crypto/ssh/terminal"
	"golang.org/x/text/encoding/charmap"

	// Use the internal terminal package
	"github.com/stlalpha/vision3/internal/terminal"
)

// --- ANSI Test Code (Moved from cmd/vision3/main.go) ---

// CP437 box drawing characters to test
var testChars = []byte{
	0xB3, 0xC4, 0xDA, 0xBF, 0xC0, 0xD9, 0xC3, 0xB4, 0xC2, 0xC1, 0xC5, 0xB0, 0xB1, 0xB2, 0xDB, 0xDF, 0xFE,
}

// Map of CP437 box drawing characters to VT100 line drawing equivalents
var cp437ToVT100 = map[byte]byte{
	0xB3: 'x', 0xC4: 'q', 0xDA: 'l', 0xBF: 'k', 0xC0: 'm', 0xD9: 'j', 0xC3: 't', 0xB4: 'u', 0xC2: 'w', 0xC1: 'v', 0xC5: 'n',
}

// ASCII fallbacks for CP437 box drawing characters
var cp437ToAscii = map[byte]string{
	0xB3: "|", 0xC4: "-", 0xDA: "+", 0xBF: "+", 0xC0: "+", 0xD9: "+", 0xC3: "+", 0xB4: "+", 0xC2: "+", 0xC1: "+", 0xC5: "+", 0xB0: ".", 0xB1: ":", 0xB2: "#", 0xDB: "#", 0xDF: "B", 0xFE: "b",
}

// ANSI Test Server handler
func handleAnsiTestSession(session ssh.Session) {
	defer session.Close()
	ptyReq, winCh, isPty := session.Pty()
	if !isPty {
		fmt.Fprintln(session, "No PTY requested. This test requires a terminal.")
		return
	}

	term := sshterminal.NewTerminal(session, "TEST> ")
	term.SetSize(ptyReq.Window.Width, ptyReq.Window.Height)

	go func() {
		for win := range winCh {
			term.SetSize(win.Width, win.Height)
		}
	}()

	fmt.Fprintln(session, "ANSI CP437 Character Display Test Suite")
	fmt.Fprintln(session, "Terminal type:", ptyReq.Term)
	fmt.Fprintln(session, "Size:", ptyReq.Window.Width, "x", ptyReq.Window.Height)
	fmt.Fprintln(session)
	fmt.Fprintln(session, "Type 'test' to run all display tests")
	fmt.Fprintln(session, "Type 'test1' through 'test7' to run individual tests")
	fmt.Fprintln(session, "Type 'file' to display a sample .ANS file with different methods")
	fmt.Fprintln(session, "Type 'terminal' to print terminal information")
	fmt.Fprintln(session, "Type 'exit' to quit")

	for {
		line, err := term.ReadLine()
		if err != nil {
			break
		}

		switch line {
		case "exit", "quit":
			fmt.Fprintln(session, "Goodbye!")
			return
		case "test":
			runAllTests(session)
		case "test1":
			testRawBytes(session)
		case "test2":
			testUnicodeEscapes(session)
		case "test3":
			testUTF8Encoding(session)
		case "test4":
			testVT100LineDrawing(session)
		case "test5":
			testASCIIFallbacks(session)
		case "test6":
			testCharacterSetSwitching(session)
		case "test7":
			testByteByByte(session)
		case "file":
			testANSFile(session)
		case "terminal":
			printTerminalInfo(session)
		default:
			fmt.Fprintln(session, "Unknown command:", line)
		}
	}
}

// --- Test Functions (Moved from cmd/vision3/main.go) ---
func runAllTests(session ssh.Session) {
	fmt.Fprintln(session, "\x1B[2J\x1B[H") // Clear screen and home cursor
	fmt.Fprintln(session, "Running all CP437 character display tests...")
	fmt.Fprintln(session)

	testRawBytes(session)
	time.Sleep(500 * time.Millisecond)
	testUnicodeEscapes(session)
	time.Sleep(500 * time.Millisecond)
	testUTF8Encoding(session)
	time.Sleep(500 * time.Millisecond)
	testVT100LineDrawing(session)
	time.Sleep(500 * time.Millisecond)
	testASCIIFallbacks(session)
	time.Sleep(500 * time.Millisecond)
	testCharacterSetSwitching(session)
	time.Sleep(500 * time.Millisecond)
	testByteByByte(session)

	fmt.Fprintln(session)
	fmt.Fprintln(session, "All tests complete. Look for any method that displays the characters correctly.")
}

func testRawBytes(session ssh.Session) {
	fmt.Fprintln(session, "\x1B[1;37m=== Test 1: Raw CP437 Bytes ===\x1B[0m")
	fmt.Fprintln(session, "Sending the raw CP437 bytes directly...")
	fmt.Fprintln(session)
	fmt.Fprint(session, "Raw CP437:  ")
	session.Write(testChars)
	fmt.Fprintln(session)
	fmt.Fprintln(session)
}

func testUnicodeEscapes(session ssh.Session) {
	fmt.Fprintln(session, "\x1B[1;37m=== Test 2: Unicode Escape Sequences ===\x1B[0m")
	fmt.Fprintln(session, "Using Go string literals with Unicode escape sequences...")
	fmt.Fprintln(session)
	fmt.Fprint(session, "Unicode:    ")
	fmt.Fprint(session, "\u2502\u2500\u250C\u2510\u2514\u2518\u251C\u2524\u252C\u2534\u253C\u2591\u2592\u2593\u2588\u00DF\u00FE")
	fmt.Fprintln(session)
	fmt.Fprintln(session)
}

func testUTF8Encoding(session ssh.Session) {
	fmt.Fprintln(session, "\x1B[1;37m=== Test 3: CP437 Converted to UTF-8 ===\x1B[0m")
	fmt.Fprintln(session, "Converting each CP437 character to its Unicode equivalent (using internal/ansi map)...")
	fmt.Fprintln(session)
	fmt.Fprint(session, "UTF-8:      ")
	w := bufio.NewWriter(session)
	for _, b := range testChars {
		// Use the exported map from the ansi package
		r := terminal.Cp437ToUnicode[b]
		w.WriteRune(r)
	}
	w.Flush()
	fmt.Fprintln(session)
	fmt.Fprintln(session)
}

func testVT100LineDrawing(session ssh.Session) {
	fmt.Fprintln(session, "\x1B[1;37m=== Test 4: VT100 Line Drawing Mode ===\x1B[0m")
	fmt.Fprintln(session, "Using VT100/xterm line drawing character set...")
	fmt.Fprintln(session)
	fmt.Fprint(session, "VT100:      ")
	for _, b := range testChars {
		if vt100Char, ok := cp437ToVT100[b]; ok {
			fmt.Fprintf(session, "\x1B(0%c\x1B(B", vt100Char)
		} else {
			if fallback, ok := cp437ToAscii[b]; ok {
				fmt.Fprint(session, fallback)
			} else {
				fmt.Fprint(session, "?")
			}
		}
	}
	fmt.Fprintln(session)
	fmt.Fprintln(session)
}

func testASCIIFallbacks(session ssh.Session) {
	fmt.Fprintln(session, "\x1B[1;37m=== Test 5: ASCII Fallbacks ===\x1B[0m")
	fmt.Fprintln(session, "Using pure ASCII approximations for CP437 characters...")
	fmt.Fprintln(session)
	fmt.Fprint(session, "ASCII:      ")
	for _, b := range testChars {
		if fallback, ok := cp437ToAscii[b]; ok {
			fmt.Fprint(session, fallback)
		} else {
			fmt.Fprint(session, "?")
		}
	}
	fmt.Fprintln(session)
	fmt.Fprintln(session)
}

func testCharacterSetSwitching(session ssh.Session) {
	fmt.Fprintln(session, "\x1B[1;37m=== Test 6: Character Set Switching ===\x1B[0m")
	fmt.Fprintln(session, "Trying different terminal character set switching sequences...")
	fmt.Fprintln(session)
	fmt.Fprint(session, "ISO 8859-1: \x1B%@")
	session.Write(testChars)
	fmt.Fprintln(session, "\x1B%G")
	fmt.Fprintln(session)
	fmt.Fprint(session, "DEC Special: ")
	for _, b := range testChars {
		if vt100Char, ok := cp437ToVT100[b]; ok {
			fmt.Fprintf(session, "\x1B(0%c", vt100Char)
		} else {
			fmt.Fprint(session, "?")
		}
	}
	fmt.Fprintln(session, "\x1B(B")
	fmt.Fprintln(session)
	fmt.Fprint(session, "UTF-8:      \x1B%G")
	w := bufio.NewWriter(session)
	for _, b := range testChars {
		r := terminal.Cp437ToUnicode[b]
		w.WriteRune(r)
	}
	w.Flush()
	fmt.Fprintln(session)
	fmt.Fprintln(session)
}

func testByteByByte(session ssh.Session) {
	fmt.Fprintln(session, "\x1B[1;37m=== Test 7: Byte-by-Byte Testing ===\x1B[0m")
	fmt.Fprintln(session, "Testing each character individually with different methods...")
	fmt.Fprintln(session)
	fmt.Fprintln(session, "CP437 Byte | Raw | UTF-8 | VT100 | ASCII")
	fmt.Fprintln(session, "---------------------------------------")
	w := bufio.NewWriter(session)
	for _, b := range testChars {
		fmt.Fprintf(w, "  0x%02X    |  ", b)
		w.WriteByte(b)
		fmt.Fprint(w, "  |  ")
		r := terminal.Cp437ToUnicode[b]
		w.WriteRune(r)
		fmt.Fprint(w, "  |  ")
		if vt100Char, ok := cp437ToVT100[b]; ok {
			fmt.Fprintf(w, "\x1B(0%c\x1B(B", vt100Char)
		} else {
			fmt.Fprint(w, " ")
		}
		fmt.Fprint(w, "  |  ")
		if fallback, ok := cp437ToAscii[b]; ok {
			fmt.Fprint(w, fallback)
		} else {
			fmt.Fprint(w, "?")
		}
		fmt.Fprintln(w)
	}
	w.Flush()
	fmt.Fprintln(session)
}

func testANSFile(session ssh.Session) {
	fmt.Fprintln(session, "\x1B[2J\x1B[H")
	fmt.Fprintln(session, "\x1B[1;37m=== Testing .ANS File Display Methods ===\x1B[0m")
	// Assumes sample.ans is in a relative 'assets' dir; adjust if needed
	filename := filepath.Join("..", "assets", "sample.ans") // Adjust path relative to cmd/ansitest
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Fprintln(session, "Sample ANSI file not found at:", filename)
		fmt.Fprintln(session, "Ensure sample.ans exists in the assets directory relative to the project root.")
		return
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintln(session, "Error reading sample file:", err)
		return
	}
	term := sshterminal.NewTerminal(session, "")

	fmt.Fprintln(session, "Press Enter to see the file displayed with raw bytes...")
	term.ReadLine()
	fmt.Fprintln(session, "\x1B[2J\x1B[H\x1B[1;37m--- Raw Bytes Method --- \x1B[0m")
	session.Write(data)
	fmt.Fprintln(session)
	fmt.Fprintln(session, "Press Enter to continue...")
	term.ReadLine()

	fmt.Fprintln(session, "\x1B[2J\x1B[H\x1B[1;37m--- UTF-8 Conversion Method --- \x1B[0m")
	decoder := charmap.CodePage437.NewDecoder()
	utf8Data, _ := decoder.Bytes(data)
	session.Write(utf8Data)
	fmt.Fprintln(session)
	fmt.Fprintln(session, "Press Enter to continue...")
	term.ReadLine()

	fmt.Fprintln(session, "\x1B[2J\x1B[H\x1B[1;37m--- VT100 Line Drawing Method --- \x1B[0m")
	displayWithVT100(session, data)
	fmt.Fprintln(session)
	fmt.Fprintln(session, "Press Enter to continue...")
	term.ReadLine()

	fmt.Fprintln(session, "\x1B[2J\x1B[H\x1B[1;37m--- ASCII Fallbacks Method --- \x1B[0m")
	displayWithAscii(session, data)
	fmt.Fprintln(session)
	fmt.Fprintln(session, "Press Enter to return to the main menu...")
	term.ReadLine()
}

func displayWithVT100(session ssh.Session, data []byte) {
	var inEscape bool
	for i := 0; i < len(data); i++ {
		b := data[i]
		if inEscape {
			session.Write([]byte{b})
			if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
				inEscape = false
			}
			continue
		}
		if b == 0x1B {
			session.Write([]byte{b})
			inEscape = true
			continue
		}
		if b < 128 {
			session.Write([]byte{b})
		} else if vt100Char, ok := cp437ToVT100[b]; ok {
			fmt.Fprintf(session, "\x1B(0%c\x1B(B", vt100Char)
		} else {
			if fallback, ok := cp437ToAscii[b]; ok {
				fmt.Fprint(session, fallback)
			} else {
				session.Write([]byte{b}) // Pass through unknown bytes
			}
		}
	}
}

func displayWithAscii(session ssh.Session, data []byte) {
	var inEscape bool
	for i := 0; i < len(data); i++ {
		b := data[i]
		if inEscape {
			session.Write([]byte{b})
			if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
				inEscape = false
			}
			continue
		}
		if b == 0x1B {
			session.Write([]byte{b})
			inEscape = true
			continue
		}
		if b < 128 {
			session.Write([]byte{b})
		} else {
			if fallback, ok := cp437ToAscii[b]; ok {
				fmt.Fprint(session, fallback)
			} else {
				fmt.Fprint(session, "?")
			}
		}
	}
}

func printTerminalInfo(session ssh.Session) {
	fmt.Fprintln(session, "\x1B[2J\x1B[H")
	fmt.Fprintln(session, "\x1B[1;37m=== Terminal Information ===\x1B[0m")
	ptyReq, _, isPty := session.Pty()
	if !isPty {
		fmt.Fprintln(session, "No PTY requested. Terminal information unavailable.")
		return
	}
	fmt.Fprintln(session, "Terminal Type:", ptyReq.Term)
	fmt.Fprintln(session, "Window Size:", ptyReq.Window.Width, "x", ptyReq.Window.Height)
	fmt.Fprintln(session)
	fmt.Fprintln(session, "Requesting terminal attributes...")
	session.SendRequest("xterm-256color", false, []byte("TERM=xterm-256color"))
	fmt.Fprintln(session, "Testing UTF-8 support:")
	fmt.Fprintln(session, "  ASCII:  ABCDEFG123456!@#$")
	fmt.Fprintln(session, "  Latin1: ñ é ü ß ö ø å")
	fmt.Fprintln(session, "  CJK:    你好 こんにちは 안녕하세요")
	fmt.Fprintln(session, "  Emoji:  🚀 💻 📦 🔥")
	fmt.Fprintln(session)
	fmt.Fprintln(session, "Testing line drawing modes:")
	fmt.Fprintln(session, "  VT100:  \x1B(0lqwqk\x1B(B")
	fmt.Fprintln(session, "          \x1B(0x x x\x1B(B")
	fmt.Fprintln(session, "          \x1B(0mqjm\x1B(B")
	fmt.Fprintln(session)
	fmt.Fprintln(session, "Testing character set switching sequences:")
	fmt.Fprintln(session, "  Default UTF-8")
	fmt.Fprintln(session, "  Switch to ISO 8859-1: \x1B%@Test\x1B%G")
	fmt.Fprintln(session, "  Switch to UTF-8: \x1B%GTest")
	fmt.Fprintln(session)
	fmt.Fprintln(session, "If you see boxes or question marks instead of special characters")
	fmt.Fprintln(session, "above, your terminal may not support UTF-8 properly.")
}

// loadHostKey loads a private key for the SSH server.
// Note: This assumes keys are relative to the cmd/ansitest directory or use absolute paths.
func loadHostKey(path string) ssh.Signer {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("FATAL: Failed to read host key %s: %v", path, err)
	}
	signer, err := gossh.ParsePrivateKey(keyBytes)
	if err != nil {
		log.Fatalf("FATAL: Failed to parse host key %s: %v", path, err)
	}
	log.Printf("INFO: Host key loaded successfully from %s", path)
	return signer
}

func main() {
	log.SetOutput(os.Stderr)
	log.Println("INFO: Starting ANSI Test server mode...")

	// Reuse the config path logic, maybe point to ../../configs
	configPath := filepath.Join("..", "..", "configs") // Path relative to cmd/ansitest
	hostKeyPath := filepath.Join(configPath, "ssh_host_rsa_key")
	log.Printf("INFO: Test Server - Attempting to load host key from: %s", hostKeyPath)
	if _, err := os.Stat(hostKeyPath); os.IsNotExist(err) {
		log.Printf("WARN: Host key not found at %s. Server might be inaccessible.", hostKeyPath)
		// Consider generating a temporary key if none exists for pure testing
	}
	hostKeySigner := loadHostKey(hostKeyPath)

	server := &ssh.Server{
		Addr:    ":2223", // Use a different port for testing
		Handler: handleAnsiTestSession,
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return true // Allow any password for testing
		},
	}

	server.AddHostKey(hostKeySigner)

	log.Printf("INFO: Starting ANSI Test server on :2223...")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Fatalf("FATAL: Failed to start ANSI Test server: %v", err)
	}
	log.Println("INFO: ANSI Test server shut down.")
}
