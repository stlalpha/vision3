package ziplab

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// cleanDIZ strips trailing whitespace and DOS-era control characters
// (Ctrl-Z / 0x1A CP/M EOF marker) from FILE_ID.DIZ content.
func cleanDIZ(raw string) string {
	s := strings.TrimRight(raw, " \t\r\n\x1a")
	s = strings.ReplaceAll(s, "\x1a", "")
	return s
}

// ExtractDIZFromZip opens a ZIP archive and reads FILE_ID.DIZ directly
// without extracting to disk. Returns empty string if no DIZ found.
func ExtractDIZFromZip(archivePath string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip %s: %w", archivePath, err)
	}
	defer r.Close()

	for _, f := range r.File {
		baseName := filepath.Base(f.Name)
		if strings.EqualFold(baseName, "FILE_ID.DIZ") {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("failed to read FILE_ID.DIZ from %s: %w", archivePath, err)
			}
			defer rc.Close()

			data, readErr := io.ReadAll(io.LimitReader(rc, 10*1024))
			if readErr != nil {
				return "", fmt.Errorf("failed to read FILE_ID.DIZ from %s: %w", archivePath, readErr)
			}
			return cleanDIZ(string(data)), nil
		}
	}
	return "", nil
}

// ExtractDIZFromArchive extracts FILE_ID.DIZ from a supported archive.
// For native ZIP files, it reads directly from the archive without extraction.
// For external formats, it extracts to a temp directory, searches for the DIZ,
// and cleans up. Returns empty string if no DIZ is found.
func ExtractDIZFromArchive(archivePath, configPath string) (string, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		cfg = DefaultConfig()
	}

	at, ok := cfg.GetArchiveType(archivePath)
	if !ok {
		return "", nil
	}

	if at.Native {
		return ExtractDIZFromZip(archivePath)
	}

	if at.ExtractCommand == "" {
		return "", fmt.Errorf("no extract command configured for %s", filepath.Ext(archivePath))
	}

	p := NewProcessor(cfg, filepath.Dir(archivePath))
	workDir, err := p.StepExtract(archivePath)
	if err != nil {
		return "", fmt.Errorf("extraction failed: %w", err)
	}
	if workDir == "" {
		return "", nil
	}
	defer os.RemoveAll(workDir)

	return p.findAndReadDIZ(workDir), nil
}
