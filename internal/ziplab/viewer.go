package ziplab

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gliderlabs/ssh"
	"golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/transfer"
)

// viewerFormatFileSize returns a human-readable file size string.
// Same logic as internal/menu/file_viewer.go formatFileSize.
func viewerFormatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(size)/1024.0)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1fM", float64(size)/(1024.0*1024.0))
	}
	return fmt.Sprintf("%.1fG", float64(size)/(1024.0*1024.0*1024.0))
}

// formatArchiveListing opens a ZIP file and writes a numbered, pipe-code-formatted
// listing to w. Returns the file count and any error opening the archive.
func formatArchiveListing(w io.Writer, zipPath string, filename string, termHeight int) (int, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open archive: %w", err)
	}
	defer r.Close()

	// Header
	fmt.Fprintf(w, "\r\n|15--- Archive Contents: %s ---|07\r\n\r\n", filename)

	// Column headers
	fmt.Fprintf(w, "|14  #   Size       Date       Name|07\r\n")
	fmt.Fprintf(w, "|08 ---  ---------  ---------- --------------------------------|07\r\n")

	var totalSize uint64
	fileCount := 0

	for _, f := range r.File {
		fileCount++
		sizeStr := viewerFormatFileSize(int64(f.UncompressedSize64))
		dateStr := f.Modified.Format("01/02/2006")

		fmt.Fprintf(w, "|07 %3d  %9s  %s  |15%s|07\r\n",
			fileCount, sizeStr, dateStr, f.Name)

		totalSize += f.UncompressedSize64
	}

	// Summary
	fmt.Fprintf(w, "\r\n|07 %d file(s), %s total\r\n",
		fileCount, viewerFormatFileSize(int64(totalSize)))

	// Prompt
	fmt.Fprintf(w, "\r\n|07[|15#|07]=Extract  [|15Q|07]=Quit\r\n")

	return fileCount, nil
}

// extractSingleEntry extracts a single file from a ZIP archive by 1-based index.
// Returns the path to the extracted file, a cleanup function that removes the
// temp directory, and any error. On error, cleanup is handled internally and
// the returned cleanup is a no-op.
func extractSingleEntry(zipPath string, entryNum int) (string, func(), error) {
	noop := func() {}

	if entryNum < 1 {
		return "", noop, fmt.Errorf("entry number must be >= 1, got %d", entryNum)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", noop, fmt.Errorf("failed to open archive: %w", err)
	}
	defer r.Close()

	if entryNum > len(r.File) {
		return "", noop, fmt.Errorf("entry %d out of range (archive has %d entries)", entryNum, len(r.File))
	}

	entry := r.File[entryNum-1]

	tmpDir, err := os.MkdirTemp("", "ziplab-extract-*")
	if err != nil {
		return "", noop, fmt.Errorf("failed to create temp dir: %w", err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	// Use Base to prevent zip slip
	destPath := filepath.Join(tmpDir, filepath.Base(entry.Name))

	rc, err := entry.Open()
	if err != nil {
		cleanup()
		return "", noop, fmt.Errorf("failed to open entry: %w", err)
	}
	defer rc.Close()

	outFile, err := os.Create(destPath)
	if err != nil {
		cleanup()
		return "", noop, fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, rc); err != nil {
		cleanup()
		return "", noop, fmt.Errorf("failed to extract file: %w", err)
	}

	return destPath, cleanup, nil
}

// RunZipLabView presents an interactive archive viewer that lets the user
// browse entries and extract individual files via ZMODEM.
func RunZipLabView(s ssh.Session, terminal *term.Terminal, filePath string, filename string, outputMode ansi.OutputMode) {
	// Build the listing into a buffer to get the file count.
	var buf bytes.Buffer
	fileCount, err := formatArchiveListing(&buf, filePath, filename, 24)
	if err != nil {
		msg := "\r\n|01Error reading archive.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		pauseEnterViewer(s, terminal, outputMode)
		return
	}
	if fileCount == 0 {
		msg := "\r\n|01Archive is empty.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		pauseEnterViewer(s, terminal, outputMode)
		return
	}

	// Display the listing.
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes(buf.Bytes()), outputMode)

	for {
		prompt := fmt.Sprintf("\r\n|07ZipLab [|15#|07/|15Q|07]: |15")
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

		line, err := terminal.ReadLine()
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)

		if line == "" {
			// Redisplay listing.
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes(buf.Bytes()), outputMode)
			continue
		}

		if strings.EqualFold(line, "Q") {
			return
		}

		num, err := strconv.Atoi(line)
		if err != nil || num < 1 || num > fileCount {
			msg := fmt.Sprintf("\r\n|01Invalid selection. Enter 1-%d or Q.|07\r\n", fileCount)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			continue
		}

		extractedPath, cleanup, err := extractSingleEntry(filePath, num)
		if err != nil {
			log.Printf("ziplab: extraction failed: %v", err)
			msg := "\r\n|01Extraction failed.|07\r\n"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			continue
		}

		baseName := filepath.Base(extractedPath)
		sendMsg := fmt.Sprintf("\r\n|07Sending |15%s|07 via ZMODEM...\r\n", baseName)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(sendMsg)), outputMode)

		if err := transfer.ExecuteZmodemSend(s, extractedPath); err != nil {
			log.Printf("ziplab: zmodem send failed: %v", err)
			msg := "\r\n|01Transfer failed.|07\r\n"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		} else {
			msg := "\r\n|10Transfer complete.|07\r\n"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		}

		cleanup()
	}
}

// pauseEnterViewer waits for the user to press ENTER before continuing.
func pauseEnterViewer(s ssh.Session, terminal *term.Terminal, outputMode ansi.OutputMode) {
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
