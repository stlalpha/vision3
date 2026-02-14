package ziplab

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
