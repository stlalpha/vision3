# ZIPLAB_VIEW Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Interactive archive file viewer with single-file ZMODEM extraction, triggered by V key on archives in LISTFILES.

**Architecture:** New `RunZipLabView` function in `internal/ziplab/viewer.go` handles the interactive loop (listing, extraction, ZMODEM send). The V key handler in `executor.go` routes archives to this function instead of `viewFileByRecord`. Built-in display formatting with optional template fallback.

**Tech Stack:** Go `archive/zip`, `transfer.ExecuteZmodemSend`, `gliderlabs/ssh`, `golang.org/x/term`, pipe code ANSI formatting.

---

### Task 1: Core Viewer — Archive Listing Display

Build the testable core: open a ZIP, format a numbered listing, write to an `io.Writer`.

**Files:**
- Create: `internal/ziplab/viewer.go`
- Create: `internal/ziplab/viewer_test.go`

**Step 1: Write the failing test**

In `internal/ziplab/viewer_test.go`:

```go
package ziplab

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatArchiveListing_BasicOutput(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := createTestZipWithTimes(t, tmpDir, "test.zip", []testZipEntry{
		{Name: "README.TXT", Content: "hello world", Year: 1995, Month: 1, Day: 15},
		{Name: "PROGRAM.EXE", Content: strings.Repeat("x", 45000), Year: 1995, Month: 1, Day: 15},
	})

	var buf bytes.Buffer
	count, err := formatArchiveListing(&buf, zipPath, "TEST.ZIP", 24)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 files, got %d", count)
	}

	output := buf.String()
	if !strings.Contains(output, "README.TXT") {
		t.Error("output should contain README.TXT")
	}
	if !strings.Contains(output, "PROGRAM.EXE") {
		t.Error("output should contain PROGRAM.EXE")
	}
	// Should have numbered entries
	if !strings.Contains(output, "1") {
		t.Error("output should contain entry number 1")
	}
	if !strings.Contains(output, "2") {
		t.Error("output should contain entry number 2")
	}
}
```

Note: `createTestZip` already exists in `processor_test.go` but doesn't set timestamps. Add a new helper `createTestZipWithTimes` in `viewer_test.go` that creates entries with specific dates:

```go
type testZipEntry struct {
	Name    string
	Content string
	Year    int
	Month   int
	Day     int
}

func createTestZipWithTimes(t *testing.T, dir, name string, entries []testZipEntry) string {
	t.Helper()
	zipPath := filepath.Join(dir, name)
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for _, entry := range entries {
		header := &zip.FileHeader{
			Name:     entry.Name,
			Method:   zip.Deflate,
			Modified: time.Date(entry.Year, time.Month(entry.Month), entry.Day, 12, 0, 0, 0, time.UTC),
		}
		fw, err := w.CreateHeader(header)
		if err != nil {
			t.Fatalf("failed to add %s: %v", entry.Name, err)
		}
		if _, err := fw.Write([]byte(entry.Content)); err != nil {
			t.Fatalf("failed to write %s: %v", entry.Name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}
	return zipPath
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ziplab/ -run TestFormatArchiveListing_BasicOutput -v`
Expected: FAIL — `formatArchiveListing` undefined

**Step 3: Write minimal implementation**

In `internal/ziplab/viewer.go`:

```go
package ziplab

import (
	"archive/zip"
	"fmt"
	"io"
)

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

// formatArchiveListing writes a numbered ZIP listing to w. Returns file count or error.
// termHeight controls paging (0 = no paging, used in tests).
func formatArchiveListing(w io.Writer, zipPath string, filename string, termHeight int) (int, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open archive: %w", err)
	}
	defer r.Close()

	fmt.Fprintf(w, "\r\n|15--- Archive Contents: %s ---|07\r\n\r\n", filename)
	fmt.Fprintf(w, "|14  #   Size       Date       Name|07\r\n")
	fmt.Fprintf(w, "|08 ---  ---------  ---------- --------------------------------|07\r\n")

	totalSize := uint64(0)
	fileCount := 0

	for _, f := range r.File {
		fileCount++
		sizeStr := formatFileSize(int64(f.UncompressedSize64))
		dateStr := f.Modified.Format("01/02/2006")

		fmt.Fprintf(w, "|07%3d  %10s  %s  |15%s|07\r\n", fileCount, sizeStr, dateStr, f.Name)
		totalSize += f.UncompressedSize64
	}

	fmt.Fprintf(w, "\r\n|07 %d file(s), %s total\r\n", fileCount, formatFileSize(int64(totalSize)))
	fmt.Fprintf(w, "\r\n|07[|15#|07]=Extract  [|15Q|07]=Quit\r\n")

	return fileCount, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/ziplab/ -run TestFormatArchiveListing -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ziplab/viewer.go internal/ziplab/viewer_test.go
git commit -m "feat(ziplab): archive listing display for ZIPLAB_VIEW"
```

---

### Task 2: Additional Listing Tests — Edge Cases

**Files:**
- Modify: `internal/ziplab/viewer_test.go`

**Step 1: Write the failing tests**

Add to `internal/ziplab/viewer_test.go`:

```go
func TestFormatArchiveListing_EmptyZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := createTestZipWithTimes(t, tmpDir, "empty.zip", nil)

	var buf bytes.Buffer
	count, err := formatArchiveListing(&buf, zipPath, "EMPTY.ZIP", 24)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 files, got %d", count)
	}
	output := buf.String()
	if !strings.Contains(output, "0 file(s)") {
		t.Error("should show 0 files in summary")
	}
}

func TestFormatArchiveListing_CorruptZip(t *testing.T) {
	tmpDir := t.TempDir()
	corruptPath := filepath.Join(tmpDir, "corrupt.zip")
	os.WriteFile(corruptPath, []byte("not a zip"), 0644)

	var buf bytes.Buffer
	_, err := formatArchiveListing(&buf, corruptPath, "CORRUPT.ZIP", 24)
	if err == nil {
		t.Fatal("expected error for corrupt zip")
	}
}

func TestFormatArchiveListing_ManyFiles(t *testing.T) {
	tmpDir := t.TempDir()
	var entries []testZipEntry
	for i := 0; i < 50; i++ {
		entries = append(entries, testZipEntry{
			Name:    fmt.Sprintf("file%03d.txt", i+1),
			Content: "content",
			Year:    1995, Month: 6, Day: 1,
		})
	}
	zipPath := createTestZipWithTimes(t, tmpDir, "many.zip", entries)

	var buf bytes.Buffer
	count, err := formatArchiveListing(&buf, zipPath, "MANY.ZIP", 24)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 50 {
		t.Errorf("expected 50 files, got %d", count)
	}
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./internal/ziplab/ -run TestFormatArchiveListing -v`
Expected: PASS (implementation from Task 1 handles these)

**Step 3: Commit**

```bash
git add internal/ziplab/viewer_test.go
git commit -m "test(ziplab): edge case tests for archive listing"
```

---

### Task 3: Single File Extraction

Extract a single entry from a ZIP to a temp directory, returning the path.

**Files:**
- Modify: `internal/ziplab/viewer.go`
- Modify: `internal/ziplab/viewer_test.go`

**Step 1: Write the failing test**

Add to `internal/ziplab/viewer_test.go`:

```go
func TestExtractSingleEntry_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := createTestZipWithTimes(t, tmpDir, "test.zip", []testZipEntry{
		{Name: "README.TXT", Content: "hello world", Year: 1995, Month: 1, Day: 15},
		{Name: "PROGRAM.EXE", Content: "binary content", Year: 1995, Month: 1, Day: 15},
	})

	// Extract entry #2 (1-based)
	extractedPath, cleanup, err := extractSingleEntry(zipPath, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	content, err := os.ReadFile(extractedPath)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if string(content) != "binary content" {
		t.Errorf("expected 'binary content', got %q", string(content))
	}
	if filepath.Base(extractedPath) != "PROGRAM.EXE" {
		t.Errorf("expected filename PROGRAM.EXE, got %s", filepath.Base(extractedPath))
	}
}

func TestExtractSingleEntry_InvalidIndex(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := createTestZipWithTimes(t, tmpDir, "test.zip", []testZipEntry{
		{Name: "only.txt", Content: "one file", Year: 1995, Month: 1, Day: 15},
	})

	_, _, err := extractSingleEntry(zipPath, 0) // 0 is invalid (1-based)
	if err == nil {
		t.Fatal("expected error for index 0")
	}

	_, _, err = extractSingleEntry(zipPath, 2) // only 1 file
	if err == nil {
		t.Fatal("expected error for out-of-range index")
	}
}

func TestExtractSingleEntry_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := createTestZipWithTimes(t, tmpDir, "test.zip", []testZipEntry{
		{Name: "temp.txt", Content: "temporary", Year: 1995, Month: 1, Day: 15},
	})

	extractedPath, cleanup, err := extractSingleEntry(zipPath, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should exist before cleanup
	if _, err := os.Stat(extractedPath); err != nil {
		t.Fatal("extracted file should exist before cleanup")
	}

	cleanup()

	// Temp directory should be gone after cleanup
	dir := filepath.Dir(extractedPath)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("temp directory should be removed after cleanup")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/ziplab/ -run TestExtractSingleEntry -v`
Expected: FAIL — `extractSingleEntry` undefined

**Step 3: Write minimal implementation**

Add to `internal/ziplab/viewer.go`:

```go
// extractSingleEntry extracts a single file from a ZIP by 1-based index.
// Returns the path to the extracted file and a cleanup function that removes the temp directory.
func extractSingleEntry(zipPath string, entryNum int) (string, func(), error) {
	noopCleanup := func() {}

	if entryNum < 1 {
		return "", noopCleanup, fmt.Errorf("invalid entry number: %d", entryNum)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", noopCleanup, fmt.Errorf("failed to open archive: %w", err)
	}
	defer r.Close()

	if entryNum > len(r.File) {
		return "", noopCleanup, fmt.Errorf("entry %d out of range (archive has %d files)", entryNum, len(r.File))
	}

	entry := r.File[entryNum-1]

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "ziplab-extract-*")
	if err != nil {
		return "", noopCleanup, fmt.Errorf("failed to create temp dir: %w", err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	// Use only the base name to prevent zip slip
	baseName := filepath.Base(entry.Name)
	destPath := filepath.Join(tmpDir, baseName)

	rc, err := entry.Open()
	if err != nil {
		cleanup()
		return "", noopCleanup, fmt.Errorf("failed to open entry: %w", err)
	}
	defer rc.Close()

	outFile, err := os.Create(destPath)
	if err != nil {
		cleanup()
		return "", noopCleanup, fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, rc); err != nil {
		cleanup()
		return "", noopCleanup, fmt.Errorf("failed to extract: %w", err)
	}

	return destPath, cleanup, nil
}
```

Add `"os"` and `"path/filepath"` to the imports in `viewer.go`.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/ziplab/ -run TestExtractSingleEntry -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ziplab/viewer.go internal/ziplab/viewer_test.go
git commit -m "feat(ziplab): single file extraction from ZIP archives"
```

---

### Task 4: Interactive Viewer Function — `RunZipLabView`

Wire up the listing + extraction + ZMODEM into the interactive terminal loop.

**Files:**
- Modify: `internal/ziplab/viewer.go`
- Modify: `internal/ziplab/viewer_test.go`

**Step 1: Write the failing test**

The interactive viewer requires SSH session and terminal, which are hard to unit test directly. Write a test for the non-interactive parts and a basic smoke test for the function signature.

Add to `internal/ziplab/viewer_test.go`:

```go
func TestRunZipLabView_Exists(t *testing.T) {
	// Verify the function signature compiles correctly.
	// The actual interactive behavior requires an SSH session.
	var fn func(ssh.Session, *term.Terminal, string, string, ansi.OutputMode)
	fn = RunZipLabView
	_ = fn
}
```

This requires adding imports for `ssh`, `term`, and `ansi` packages:

```go
import (
	"github.com/gliderlabs/ssh"
	"golang.org/x/term"
	"github.com/stlalpha/vision3/internal/ansi"
)
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/ziplab/ -run TestRunZipLabView_Exists -v`
Expected: FAIL — `RunZipLabView` undefined

**Step 3: Write minimal implementation**

Add to `internal/ziplab/viewer.go`:

```go
import (
	"bufio"
	"log"
	"strconv"

	"github.com/gliderlabs/ssh"
	"golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/transfer"
)

// RunZipLabView displays an interactive archive viewer with extraction support.
// The user sees a numbered file listing and can extract individual files via ZMODEM.
func RunZipLabView(s ssh.Session, terminal *term.Terminal, filePath string, filename string, outputMode ansi.OutputMode) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to open archive %s: %v", filePath, err)
		msg := "\r\n|01Error reading archive.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		pauseEnterViewer(s, terminal, outputMode)
		return
	}
	fileCount := len(r.File)
	r.Close()

	if fileCount == 0 {
		msg := "\r\n|01Archive is empty.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		pauseEnterViewer(s, terminal, outputMode)
		return
	}

	// Display the listing
	var listBuf bytes.Buffer
	formatArchiveListing(&listBuf, filePath, filename, 0)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes(listBuf.Bytes()), outputMode)

	// Interactive loop
	for {
		prompt := "\r\n|07ZipLab [|15#|07/|15Q|07]: |15"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

		input, err := terminal.ReadLine()
		if err != nil {
			return // EOF or disconnect
		}

		input = strings.TrimSpace(input)
		if input == "" {
			// Redisplay listing
			var buf bytes.Buffer
			formatArchiveListing(&buf, filePath, filename, 0)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes(buf.Bytes()), outputMode)
			continue
		}

		upper := strings.ToUpper(input)
		if upper == "Q" {
			return
		}

		// Try to parse as a file number
		num, parseErr := strconv.Atoi(input)
		if parseErr != nil || num < 1 || num > fileCount {
			msg := fmt.Sprintf("\r\n|01Invalid selection. Enter 1-%d or Q.|07\r\n", fileCount)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			continue
		}

		// Extract and send via ZMODEM
		extractedPath, cleanup, extractErr := extractSingleEntry(filePath, num)
		if extractErr != nil {
			log.Printf("ERROR: Failed to extract entry %d from %s: %v", num, filePath, extractErr)
			msg := "\r\n|01Extraction failed.|07\r\n"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			continue
		}

		sendMsg := fmt.Sprintf("\r\n|07Sending |15%s|07 via ZMODEM...\r\n", filepath.Base(extractedPath))
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(sendMsg)), outputMode)

		sendErr := transfer.ExecuteZmodemSend(s, extractedPath)
		cleanup()

		if sendErr != nil {
			log.Printf("ERROR: ZMODEM send failed for %s: %v", extractedPath, sendErr)
			msg := "\r\n|01Transfer failed.|07\r\n"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		} else {
			msg := "\r\n|10Transfer complete.|07\r\n"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		}
	}
}

// pauseEnterViewer displays a simple "press Enter" prompt for the viewer.
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
```

Add `"bytes"` to imports.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/ziplab/ -run TestRunZipLabView -v`
Expected: PASS

Then run ALL ziplab tests:
Run: `go test ./internal/ziplab/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/ziplab/viewer.go internal/ziplab/viewer_test.go
git commit -m "feat(ziplab): interactive ZIPLAB_VIEW with ZMODEM extraction"
```

---

### Task 5: Route V Key to ZIPLAB_VIEW for Archives

Modify LISTFILES V key handler to route archives to `RunZipLabView` instead of `viewFileByRecord`.

**Files:**
- Modify: `internal/menu/executor.go:7216-7217`

**Step 1: Write the failing test**

This is a routing change in the interactive menu loop — it's tested via the existing integration flow. Verify the build compiles correctly.

**Step 2: Modify the V key handler**

In `internal/menu/executor.go`, replace lines 7216-7217:

```go
// BEFORE:
fileToView := filesOnPage[viewIndex]
viewFileByRecord(e, s, terminal, &fileToView, outputMode)

// AFTER:
fileToView := filesOnPage[viewIndex]
if e.FileMgr.IsSupportedArchive(fileToView.Filename) {
	viewFilePath, pathErr := e.FileMgr.GetFilePath(fileToView.ID)
	if pathErr != nil {
		log.Printf("ERROR: Node %d: Failed to get path for file %s: %v", nodeNumber, fileToView.ID, pathErr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01Error locating file.|07\r\n")), outputMode)
		time.Sleep(1 * time.Second)
	} else {
		ziplab.RunZipLabView(s, terminal, viewFilePath, fileToView.Filename, outputMode)
	}
} else {
	viewFileByRecord(e, s, terminal, &fileToView, outputMode)
}
```

The `ziplab` import already exists in `executor.go` (line 33).

**Step 3: Build to verify it compiles**

Run: `go build ./...`
Expected: SUCCESS

**Step 4: Run all tests**

Run: `go test ./internal/ziplab/ -v && go test ./internal/menu/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/menu/executor.go
git commit -m "feat(ziplab): route V key to ZIPLAB_VIEW for archives"
```

---

### Task 6: Also Route VIEW_FILE Command to ZIPLAB_VIEW

The standalone `VIEW_FILE` runnable (triggered from menus) should also route archives to ZIPLAB_VIEW.

**Files:**
- Modify: `internal/menu/file_viewer.go:143-148`

**Step 1: Modify runViewFile**

In `internal/menu/file_viewer.go`, replace lines 143-148 in `runViewFile`:

```go
// BEFORE:
if e.FileMgr.IsSupportedArchive(record.Filename) {
	displayArchiveListing(s, terminal, filePath, record.Filename, outputMode, termHeight)
} else {
	displayTextWithPaging(s, terminal, filePath, record.Filename, outputMode, termHeight)
}

// AFTER:
if e.FileMgr.IsSupportedArchive(record.Filename) {
	ziplab.RunZipLabView(s, terminal, filePath, record.Filename, outputMode)
} else {
	displayTextWithPaging(s, terminal, filePath, record.Filename, outputMode, termHeight)
}
```

Add `"github.com/stlalpha/vision3/internal/ziplab"` to the imports in `file_viewer.go`.

Also update `viewFileByRecord` (line 181-182) the same way:

```go
// BEFORE:
if e.FileMgr.IsSupportedArchive(record.Filename) {
	displayArchiveListing(s, terminal, filePath, record.Filename, outputMode, termHeight)

// AFTER:
if e.FileMgr.IsSupportedArchive(record.Filename) {
	ziplab.RunZipLabView(s, terminal, filePath, record.Filename, outputMode)
```

Note: `viewFileByRecord` is still called for non-archive files from the V key handler, and may be called from other places. We update it too so all archive views go through ZIPLAB_VIEW consistently.

**Step 2: Build to verify it compiles**

Run: `go build ./...`
Expected: SUCCESS

**Step 3: Run all tests**

Run: `go test ./... 2>&1 | tail -20`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add internal/menu/file_viewer.go
git commit -m "feat(ziplab): route VIEW_FILE to ZIPLAB_VIEW for archives"
```

---

### Task 7: Integration Verification

Run the full test suite and verify everything works together.

**Step 1: Run all tests**

Run: `go test ./... -v 2>&1 | tail -40`
Expected: ALL PASS

**Step 2: Build the binary**

Run: `go build ./...`
Expected: SUCCESS with no warnings

**Step 3: Final commit if any cleanup needed**

If tests revealed issues that needed fixes, commit those fixes.
