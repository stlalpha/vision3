package menu

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/file"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"github.com/stlalpha/vision3/internal/ziplab"
)

// findFileInArea searches for a file by name (case-insensitive) in the given area.
func findFileInArea(fm *file.FileManager, areaID int, filename string) (*file.FileRecord, error) {
	files := fm.GetFilesForArea(areaID)
	lowerFilename := strings.ToLower(filename)

	for i := range files {
		if strings.ToLower(files[i].Filename) == lowerFilename {
			return &files[i], nil
		}
	}
	return nil, fmt.Errorf("file not found: %s", filename)
}

// formatFileSize returns a human-readable file size string.
func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(size)/1024.0)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1fM", float64(size)/(1024.0*1024.0))
	}
	return fmt.Sprintf("%.1fG", float64(size)/(1024.0*1024.0*1024.0))
}

// displayArchiveListing_toWriter writes ZIP archive contents to a writer (testable).
func displayArchiveListing_toWriter(w io.Writer, filePath string, filename string, termHeight int) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to open archive %s: %v", filePath, err)
		fmt.Fprintf(w, "\r\nError reading archive.\r\n")
		return
	}
	defer r.Close()

	fmt.Fprintf(w, "\r\n--- Archive Contents: %s ---\r\n\r\n", filename)
	fmt.Fprintf(w, "  Size       Date       Time     Name\r\n")
	fmt.Fprintf(w, "----------  ----------  -------  --------------------------------\r\n")

	totalSize := uint64(0)
	fileCount := 0

	for _, f := range r.File {
		mod := f.Modified
		sizeStr := formatFileSize(int64(f.UncompressedSize64))
		dateStr := mod.Format("01/02/2006")
		timeStr := mod.Format("15:04")

		fmt.Fprintf(w, "%10s  %s  %s  %s\r\n", sizeStr, dateStr, timeStr, f.Name)

		totalSize += f.UncompressedSize64
		fileCount++
	}

	fmt.Fprintf(w, "----------                       --------------------------------\r\n")
	fmt.Fprintf(w, "%10s                       %d file(s)\r\n", formatFileSize(int64(totalSize)), fileCount)
	fmt.Fprintf(w, "\r\n--- End of Archive ---\r\n")
}

// promptAndResolveFile handles the shared logic for file viewing commands:
// validates the user/area, prompts for filename, looks up the record, and resolves the path.
func promptAndResolveFile(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, currentUser *user.User, nodeNumber int, promptVerb string, outputMode ansi.OutputMode) (*file.FileRecord, string, *user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil, "", nil
	}

	currentAreaID := currentUser.CurrentFileAreaID
	if currentAreaID <= 0 {
		msg := "\r\n|01No file area selected.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", currentUser, "", nil
	}

	prompt := fmt.Sprintf("\r\n|07Enter filename to %s (or |15ENTER|07 to cancel): |15", promptVerb)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	input, err := terminal.ReadLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "", nil, "LOGOFF", io.EOF
		}
		return nil, "", currentUser, "", fmt.Errorf("failed reading input: %w", err)
	}

	filename := strings.TrimSpace(input)
	if filename == "" {
		return nil, "", currentUser, "", nil
	}

	record, err := findFileInArea(e.FileMgr, currentAreaID, filename)
	if err != nil {
		msg := fmt.Sprintf("\r\n|01File '%s' not found in current area.|07\r\n", filename)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", currentUser, "", nil
	}

	filePath, err := e.FileMgr.GetFilePath(record.ID)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to get path for file %s: %v", nodeNumber, record.ID, err)
		msg := "\r\n|01Error locating file on server.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", currentUser, "", nil
	}

	return record, filePath, currentUser, "", nil
}

// runViewFile prompts for a filename and displays it intelligently:
// archives show their contents listing, text files are displayed with paging.
func runViewFile(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running VIEW_FILE", nodeNumber)

	record, filePath, retUser, retAction, retErr := promptAndResolveFile(e, s, terminal, currentUser, nodeNumber, "view", outputMode)
	if record == nil {
		return retUser, retAction, retErr
	}

	if e.FileMgr.IsSupportedArchive(record.Filename) {
		ziplab.RunZipLabView(s, terminal, filePath, record.Filename, outputMode)
	} else {
		_, termHeight := getTerminalSize(s)
		displayTextWithPaging(s, terminal, filePath, record.Filename, outputMode, termHeight)
	}

	return currentUser, "", nil
}

// runTypeTextFile prompts for a filename and displays it as raw text with paging.
func runTypeTextFile(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running TYPE_TEXT_FILE", nodeNumber)

	record, filePath, retUser, retAction, retErr := promptAndResolveFile(e, s, terminal, currentUser, nodeNumber, "type", outputMode)
	if record == nil {
		return retUser, retAction, retErr
	}

	_, termHeight := getTerminalSize(s)
	displayTextWithPaging(s, terminal, filePath, record.Filename, outputMode, termHeight)

	return currentUser, "", nil
}

// viewFileByRecord displays a file given its record, used from the lightbar file list.
func viewFileByRecord(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, record *file.FileRecord, outputMode ansi.OutputMode) {
	filePath, err := e.FileMgr.GetFilePath(record.ID)
	if err != nil {
		log.Printf("ERROR: Failed to get path for file %s: %v", record.ID, err)
		msg := "\r\n|01Error locating file on server.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return
	}

	if e.FileMgr.IsSupportedArchive(record.Filename) {
		ziplab.RunZipLabView(s, terminal, filePath, record.Filename, outputMode)
	} else {
		_, termHeight := getTerminalSize(s)
		displayTextWithPaging(s, terminal, filePath, record.Filename, outputMode, termHeight)
	}
}

// displayArchiveListing shows ZIP archive contents with paging on the terminal.
func displayArchiveListing(s ssh.Session, terminal *term.Terminal, filePath string, filename string, outputMode ansi.OutputMode, termHeight int) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to open archive %s: %v", filePath, err)
		msg := "\r\n|01Error reading archive.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return
	}
	defer r.Close()

	header := fmt.Sprintf("\r\n|15--- Archive Contents: %s ---|07\r\n\r\n", filename)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(header)), outputMode)

	colHeader := "|14  Size       Date       Time     Name|07\r\n"
	colHeader += "|08----------  ----------  -------  --------------------------------|07\r\n"
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(colHeader)), outputMode)

	linesPerPage := termHeight - 6
	if linesPerPage < 5 {
		linesPerPage = 5
	}

	lineCount := 0
	totalSize := uint64(0)
	fileCount := 0

	for _, f := range r.File {
		mod := f.Modified
		sizeStr := formatFileSize(int64(f.UncompressedSize64))
		dateStr := mod.Format("01/02/2006")
		timeStr := mod.Format("15:04")

		line := fmt.Sprintf("|07%10s  %s  %s  |15%s|07\r\n", sizeStr, dateStr, timeStr, f.Name)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		totalSize += f.UncompressedSize64
		fileCount++
		lineCount++

		if lineCount >= linesPerPage {
			if !pauseMore(s, terminal, outputMode) {
				return
			}
			lineCount = 0
		}
	}

	summary := "\r\n|08----------                       --------------------------------|07\r\n"
	summary += fmt.Sprintf("|07%10s                       |15%d file(s)|07\r\n", formatFileSize(int64(totalSize)), fileCount)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(summary)), outputMode)

	footer := "\r\n|15--- End of Archive ---|07\r\n"
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(footer)), outputMode)
	pauseEnter(s, terminal, outputMode)
}

// displayTextWithPaging shows text file contents with paging on the terminal.
func displayTextWithPaging(s ssh.Session, terminal *term.Terminal, filePath string, filename string, outputMode ansi.OutputMode, termHeight int) {
	f, err := os.Open(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to open file %s: %v", filePath, err)
		msg := "\r\n|01Error opening file.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return
	}
	defer f.Close()

	header := fmt.Sprintf("\r\n|15--- Viewing: %s ---|07\r\n\r\n", filename)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(header)), outputMode)

	linesPerPage := termHeight - 4
	if linesPerPage < 5 {
		linesPerPage = 5
	}

	lineCount := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 4096)

	for scanner.Scan() {
		line := scanner.Text()
		terminalio.WriteProcessedBytes(terminal, []byte(line+"\r\n"), outputMode)
		lineCount++

		if lineCount >= linesPerPage {
			if !pauseMore(s, terminal, outputMode) {
				return
			}
			lineCount = 0
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("WARN: Error reading file %s: %v", filePath, err)
	}

	footer := "\r\n|15--- End of File ---|07\r\n"
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(footer)), outputMode)
	pauseEnter(s, terminal, outputMode)
}

// pauseMore displays a "More" prompt and waits for user input.
// Returns true to continue, false to abort.
func pauseMore(s ssh.Session, terminal *term.Terminal, outputMode ansi.OutputMode) bool {
	prompt := "|07[|15MORE|07: |15ENTER|07=Continue, |15Q|07=Quit] "
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			return false
		}
		if r == 'q' || r == 'Q' {
			terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
			return false
		}
		if r == '\r' || r == '\n' || r == ' ' {
			terminalio.WriteProcessedBytes(terminal, []byte("\r\x1b[K"), outputMode)
			return true
		}
	}
}

// pauseEnter displays a simple "press Enter" prompt.
func pauseEnter(s ssh.Session, terminal *term.Terminal, outputMode ansi.OutputMode) {
	prompt := "\r\n|07Press |15[ENTER]|07 to continue... "
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			return
		}
		if r == '\r' || r == '\n' {
			return
		}
	}
}

// getTerminalSize returns the terminal width and height from the session.
func getTerminalSize(s ssh.Session) (int, int) {
	ptyReq, _, isPty := s.Pty()
	if isPty && ptyReq.Window.Width > 0 && ptyReq.Window.Height > 0 {
		return ptyReq.Window.Width, ptyReq.Window.Height
	}
	return 80, 24
}

// displayTextWithPaging_toWriter writes text file contents to a writer (testable).
func displayTextWithPaging_toWriter(w io.Writer, filePath string, filename string, termHeight int) {
	f, err := os.Open(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to open file %s: %v", filePath, err)
		fmt.Fprintf(w, "\r\nError opening file.\r\n")
		return
	}
	defer f.Close()

	fmt.Fprintf(w, "\r\n--- Viewing: %s ---\r\n\r\n", filename)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 4096)

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintf(w, "%s\r\n", line)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("WARN: Error reading file %s: %v", filePath, err)
	}

	fmt.Fprintf(w, "\r\n--- End of File ---\r\n")
}
