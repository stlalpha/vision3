package ftn

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// zipMagic is the 4-byte magic number for ZIP archives (PK\x03\x04).
var zipMagic = []byte{0x50, 0x4B, 0x03, 0x04}

// BundleExtension reports whether a filename looks like an FTN echomail bundle
// based on its extension. Binkd uses day-of-week suffixes for bundles:
//
//	.mo0 .tu0 .we0 .th0 .fr0 .sa0 .su0  (day-based, normal)
//	.mo1 .tu1 ...                          (day-based, overflow)
//	.out                                   (normal outbound bundle)
//	.zip                                   (explicit ZIP bundle)
//
// It also handles uppercase variants.
func BundleExtension(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".zip":
		return true
	case ".out":
		// .out files can be flow files (text) or bundles; we test for ZIP magic.
		return true
	}
	// Day-of-week bundle extensions: .mo0-.su9
	if len(ext) == 4 && ext[0] == '.' {
		day := ext[1:3]
		digit := ext[3]
		switch day {
		case "mo", "tu", "we", "th", "fr", "sa", "su":
			if digit >= '0' && digit <= '9' {
				return true
			}
		}
	}
	return false
}

// IsZIPBundle reports whether the file at path begins with the ZIP magic bytes.
func IsZIPBundle(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	magic := make([]byte, 4)
	n, err := f.Read(magic)
	if err != nil || n < 4 {
		return false, nil
	}
	return bytes.Equal(magic, zipMagic), nil
}

// ExtractBundle extracts .PKT files from a ZIP bundle at srcPath into destDir.
// Returns the paths of extracted .PKT files.
func ExtractBundle(srcPath, destDir string) ([]string, error) {
	r, err := zip.OpenReader(srcPath)
	if err != nil {
		return nil, fmt.Errorf("open bundle %s: %w", filepath.Base(srcPath), err)
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("create dest dir %s: %w", destDir, err)
	}

	var extracted []string
	for _, zf := range r.File {
		name := filepath.Base(zf.Name) // ignore any directory component in the ZIP
		if strings.ToLower(filepath.Ext(name)) != ".pkt" {
			continue
		}

		destPath := filepath.Join(destDir, name)
		if err := extractZipFile(zf, destPath); err != nil {
			return extracted, fmt.Errorf("extract %s from bundle: %w", name, err)
		}
		extracted = append(extracted, destPath)
	}
	return extracted, nil
}

// extractZipFile writes a single zip.File entry to destPath.
func extractZipFile(zf *zip.File, destPath string) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

// CreateBundle creates a ZIP bundle archive at bundlePath containing the .PKT
// files listed in pktPaths. Returns the number of files bundled.
// It writes to a temporary file and renames on success so a partial bundle
// is never left at bundlePath if an error occurs.
func CreateBundle(bundlePath string, pktPaths []string) (int, error) {
	if len(pktPaths) == 0 {
		return 0, nil
	}

	if err := os.MkdirAll(filepath.Dir(bundlePath), 0755); err != nil {
		return 0, fmt.Errorf("create bundle dir: %w", err)
	}

	tmpPath := bundlePath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("create bundle file: %w", err)
	}

	zw := zip.NewWriter(f)
	count := 0
	for _, pktPath := range pktPaths {
		if err := addFileToZip(zw, pktPath); err != nil {
			zw.Close()
			f.Close()
			os.Remove(tmpPath)
			return count, fmt.Errorf("add %s to bundle: %w", filepath.Base(pktPath), err)
		}
		count++
	}

	if err := zw.Close(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return count, fmt.Errorf("close zip writer: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return count, fmt.Errorf("close bundle file: %w", err)
	}
	if err := os.Rename(tmpPath, bundlePath); err != nil {
		os.Remove(tmpPath)
		return 0, fmt.Errorf("rename bundle: %w", err)
	}
	return count, nil
}

// addFileToZip adds a single file to an open zip.Writer using only the base name.
func addFileToZip(zw *zip.Writer, filePath string) error {
	in, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.Base(filePath) // flat structure, no subdirs
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, in)
	return err
}

// BundleFileName returns the standard FTN bundle filename for a given
// destination link and day-of-week index (0=Mon, 6=Sun), using the BSO naming
// convention: NNNNFFFF.DDD where NNNN=destNet (4 hex digits),
// FFFF=destNode (4 hex digits), DDD=day extension (e.g., mo0).
func BundleFileName(destNet, destNode uint16, dayIndex int) string {
	days := []string{"mo", "tu", "we", "th", "fr", "sa", "su"}
	day := "mo"
	if dayIndex >= 0 && dayIndex < len(days) {
		day = days[dayIndex]
	}
	return fmt.Sprintf("%04x%04x.%s0", destNet, destNode, day)
}
