package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
	sshterminal "golang.org/x/crypto/ssh/terminal"

	"github.com/stlalpha/vision3/internal/terminal"
)

type AnsiTestServer struct {
	ansiFiles   []string
	outputMode  terminal.OutputMode
	testDir     string
}

func main() {
	log.SetOutput(os.Stderr)
	log.Println("INFO: Starting ANSI Test Server...")

	// Load host key
	keyPath := "configs/ssh_host_rsa_key"
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		log.Fatalf("FATAL: Host key not found at %s. Run from project root or generate keys.", keyPath)
	}

	hostKeyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		log.Fatalf("FATAL: Failed to read host key: %v", err)
	}

	hostKeySigner, err := gossh.ParsePrivateKey(hostKeyBytes)
	if err != nil {
		log.Fatalf("FATAL: Failed to parse host key: %v", err)
	}

	// Create test server
	testServer := &AnsiTestServer{
		testDir:    "testAnsi",
		outputMode: terminal.OutputModeUTF8,
	}

	// Load available ANSI files
	err = testServer.loadAnsiFiles()
	if err != nil {
		log.Fatalf("FATAL: Failed to load ANSI files: %v", err)
	}

	log.Printf("INFO: Loaded %d ANSI files from %s", len(testServer.ansiFiles), testServer.testDir)

	// Configure SSH server
	server := &ssh.Server{
		Addr:    ":2223", // Different port from main BBS (2222)
		Handler: testServer.sessionHandler,
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			// Accept any password for testing
			return true
		},
	}

	// Set crypto config for legacy BBS client compatibility
	server.ServerConfigCallback = func(ctx ssh.Context) *gossh.ServerConfig {
		return &gossh.ServerConfig{
			Config: gossh.Config{
				KeyExchanges: []string{
					"diffie-hellman-group1-sha1",
					"diffie-hellman-group14-sha1", 
					"diffie-hellman-group14-sha256",
					"ecdh-sha2-nistp256",
				},
				Ciphers: []string{
					"aes128-cbc",
					"aes256-cbc",
					"aes128-ctr",
					"aes256-ctr",
				},
				MACs: []string{
					"hmac-sha1",
					"hmac-sha1-96",
					"hmac-sha2-256",
				},
			},
		}
	}

	server.AddHostKey(hostKeySigner)

	log.Printf("INFO: ANSI Test Server listening on :2223")
	log.Printf("INFO: Connect with: ssh testuser@localhost -p 2223")
	
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("FATAL: Server failed: %v", err)
	}
}

func (ats *AnsiTestServer) loadAnsiFiles() error {
	ats.ansiFiles = []string{}
	
	return filepath.WalkDir(ats.testDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		if d.IsDir() {
			return nil
		}
		
		// Look for ANSI files
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".ans" || ext == ".ansi" {
			// Store relative path from testDir
			relPath, _ := filepath.Rel(ats.testDir, path)
			ats.ansiFiles = append(ats.ansiFiles, relPath)
		}
		
		return nil
	})
}

func (ats *AnsiTestServer) sessionHandler(s ssh.Session) {
	log.Printf("ANSI Test: Connection from %s", s.RemoteAddr())
	
	// Create terminal instance
	bbs := terminal.NewBBS(s)
	
	// Create proper terminal for input handling
	term := sshterminal.NewTerminal(s, "")
	
	defer func() {
		log.Printf("ANSI Test: Disconnected %s", s.RemoteAddr())
		s.Close()
	}()
	
	// Main menu loop
	for {
		ats.showMainMenu(bbs)
		
		// Read complete line using terminal
		line, err := term.ReadLine()
		if err != nil {
			log.Printf("ANSI Test: Read error: %v", err)
			return
		}
		
		command := strings.TrimSpace(line)
		command = strings.ToLower(command)
		
		if command == "q" || command == "quit" || command == "exit" {
			bbs.WriteString("\r\n\x1b[1;33mGoodbye!\x1b[0m\r\n")
			return
		}
		
		if err := ats.handleCommand(bbs, command); err != nil {
			bbs.WriteString(fmt.Sprintf("\r\n\x1b[1;31mError: %v\x1b[0m\r\n", err))
		}
		
		// Pause before showing menu again
		bbs.WriteString("\r\n\x1b[1;37mPress any key to continue...\x1b[0m")
		term.ReadLine()
	}
}

func (ats *AnsiTestServer) showMainMenu(bbs *terminal.BBS) {
	bbs.WriteString("\x1b[2J\x1b[H") // Clear screen, home cursor
	
	// Use proper CP437 box drawing characters with exact character alignment
	header := "\x1b[1;37m" + // Bright white
		"┌──────────────────────────────────────────────────────────────────────────────┐\r\n" +
		"│                            ANSI TEST SERVER                                  │\r\n" +
		"│                          ViSiON/3 ANSI Renderer                              │\r\n" +
		"└──────────────────────────────────────────────────────────────────────────────┘\x1b[0m\r\n"
	
	bbs.WriteString(header)
	
	modeStr := "UTF-8"
	switch ats.outputMode {
	case terminal.OutputModeCP437:
		modeStr = "CP437"
	case terminal.OutputModeAuto:
		modeStr = "Auto"
	}
	
	menu := fmt.Sprintf(`
Available Commands:

 [L]ist      - List all available ANSI files
 [D]isplay   - Display ANSI file by number (e.g., 'd 1')
 [R]aw       - Show raw file content (no processing)
 [M]ode      - Change output mode (UTF-8/CP437/Auto)
 [I]nfo      - Show file information
 [Q]uit      - Exit test server

Current Output Mode: %s
Available ANSI Files: %d

Command: `, modeStr, len(ats.ansiFiles))
	
	bbs.WriteString(menu)
}

func (ats *AnsiTestServer) handleCommand(bbs *terminal.BBS, command string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("no command specified")
	}
	
	cmd := parts[0]
	
	switch cmd {
	case "l", "list":
		return ats.listFiles(bbs)
		
	case "d", "display":
		if len(parts) < 2 {
			return fmt.Errorf("usage: display <number>")
		}
		fileNum, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid file number: %s", parts[1])
		}
		return ats.displayFile(bbs, fileNum, false)
		
	case "r", "raw":
		if len(parts) < 2 {
			return fmt.Errorf("usage: raw <number>")
		}
		fileNum, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid file number: %s", parts[1])
		}
		return ats.displayFile(bbs, fileNum, true)
		
	case "m", "mode":
		return ats.changeMode(bbs, parts[1:])
		
	case "i", "info":
		if len(parts) < 2 {
			return fmt.Errorf("usage: info <number>")
		}
		fileNum, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid file number: %s", parts[1])
		}
		return ats.showFileInfo(bbs, fileNum)
		
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func (ats *AnsiTestServer) listFiles(bbs *terminal.BBS) error {
	bbs.WriteString("\x1b[2J\x1b[H") // Clear screen
	bbs.WriteString("\x1b[1;36mAvailable ANSI Files:\x1b[0m\r\n\r\n")
	
	sort.Strings(ats.ansiFiles)
	
	for i, file := range ats.ansiFiles {
		// Get file size
		fullPath := filepath.Join(ats.testDir, file)
		info, err := os.Stat(fullPath)
		size := "unknown"
		if err == nil {
			size = fmt.Sprintf("%d bytes", info.Size())
		}
		
		bbs.WriteString(fmt.Sprintf("\x1b[1;33m%2d.\x1b[0m \x1b[37m%-30s\x1b[0m \x1b[90m(%s)\x1b[0m\r\n", 
			i+1, file, size))
	}
	
	bbs.WriteString("\r\n")
	return nil
}

func (ats *AnsiTestServer) displayFile(bbs *terminal.BBS, fileNum int, raw bool) error {
	if fileNum < 1 || fileNum > len(ats.ansiFiles) {
		return fmt.Errorf("file number %d out of range (1-%d)", fileNum, len(ats.ansiFiles))
	}
	
	filename := ats.ansiFiles[fileNum-1]
	fullPath := filepath.Join(ats.testDir, filename)
	
	bbs.WriteString("\x1b[2J\x1b[H") // Clear screen
	
	if raw {
		bbs.WriteString(fmt.Sprintf("\x1b[1;31mRAW CONTENT: %s\x1b[0m\r\n", filename))
		bbs.WriteString("\x1b[90m" + strings.Repeat("─", 80) + "\x1b[0m\r\n")
		
		// Display raw bytes
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to read file: %v", err)
		}
		
		bbs.Write(data)
	} else {
		bbs.WriteString(fmt.Sprintf("\x1b[1;32mRENDERED: %s\x1b[0m\r\n", filename))
		bbs.WriteString("\x1b[90m" + strings.Repeat("─", 80) + "\x1b[0m\r\n")
		
		// Use terminal system to process and display
		startTime := time.Now()
		err := bbs.DisplayFile(fullPath)
		renderTime := time.Since(startTime)
		
		if err != nil {
			return fmt.Errorf("failed to display file: %v", err)
		}
		
		bbs.WriteString(fmt.Sprintf("\r\n\x1b[90m[Rendered in %v]\x1b[0m\r\n", renderTime))
	}
	
	return nil
}

func (ats *AnsiTestServer) changeMode(bbs *terminal.BBS, args []string) error {
	if len(args) == 0 {
		bbs.WriteString("\x1b[1;36mAvailable modes:\x1b[0m\r\n")
		bbs.WriteString("  \x1b[1;33mutf8\x1b[0m   - UTF-8 output (modern terminals)\r\n")
		bbs.WriteString("  \x1b[1;33mcp437\x1b[0m  - CP437 output (authentic BBS)\r\n")
		bbs.WriteString("  \x1b[1;33mauto\x1b[0m   - Auto-detect based on terminal\r\n")
		return nil
	}
	
	mode := strings.ToLower(args[0])
	switch mode {
	case "utf8":
		ats.outputMode = terminal.OutputModeUTF8
		bbs.WriteString("\x1b[1;32mOutput mode set to UTF-8\x1b[0m\r\n")
	case "cp437":
		ats.outputMode = terminal.OutputModeCP437
		bbs.WriteString("\x1b[1;32mOutput mode set to CP437\x1b[0m\r\n")
	case "auto":
		ats.outputMode = terminal.OutputModeAuto
		bbs.WriteString("\x1b[1;32mOutput mode set to Auto-detect\x1b[0m\r\n")
	default:
		return fmt.Errorf("invalid mode: %s", mode)
	}
	
	return nil
}

func (ats *AnsiTestServer) showFileInfo(bbs *terminal.BBS, fileNum int) error {
	if fileNum < 1 || fileNum > len(ats.ansiFiles) {
		return fmt.Errorf("file number %d out of range (1-%d)", fileNum, len(ats.ansiFiles))
	}
	
	filename := ats.ansiFiles[fileNum-1]
	fullPath := filepath.Join(ats.testDir, filename)
	
	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}
	
	bbs.WriteString(fmt.Sprintf("\x1b[1;36mFile Information:\x1b[0m\r\n\r\n"))
	bbs.WriteString(fmt.Sprintf("  \x1b[1;33mName:\x1b[0m      %s\r\n", filename))
	bbs.WriteString(fmt.Sprintf("  \x1b[1;33mSize:\x1b[0m      %d bytes\r\n", info.Size()))
	bbs.WriteString(fmt.Sprintf("  \x1b[1;33mModified:\x1b[0m  %s\r\n", info.ModTime().Format("2006-01-02 15:04:05")))
	
	// Try to detect if file contains SAUCE metadata
	data, err := os.ReadFile(fullPath)
	if err == nil {
		if strings.Contains(string(data), "SAUCE") {
			bbs.WriteString(fmt.Sprintf("  \x1b[1;33mSAUCE:\x1b[0m     \x1b[1;32mDetected\x1b[0m\r\n"))
		} else {
			bbs.WriteString(fmt.Sprintf("  \x1b[1;33mSAUCE:\x1b[0m     \x1b[90mNone\x1b[0m\r\n"))
		}
		
		// Count ANSI escape sequences
		escapeCount := strings.Count(string(data), "\x1b[")
		bbs.WriteString(fmt.Sprintf("  \x1b[1;33mANSI Seq:\x1b[0m  %d sequences\r\n", escapeCount))
	}
	
	bbs.WriteString("\r\n")
	return nil
}