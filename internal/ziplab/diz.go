package ziplab

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/stlalpha/vision3/internal/ansi"
)

// stripSauceMetadata removes the SAUCE metadata block (and optional COMNT block)
// from the end of ANSI art file bytes, along with the CP/M EOF marker (0x1A).
func stripSauceMetadata(input []byte) []byte {
	if len(input) < 7 {
		return input
	}
	idx := bytes.LastIndex(input, []byte("SAUCE00"))
	if idx < 0 {
		return input
	}
	if idx < len(input)-512 {
		return input
	}
	cut := idx
	if idx+128 <= len(input) {
		comments := int(input[idx+104])
		if comments > 0 {
			commentLen := 5 + (comments * 64)
			commentStart := idx - commentLen
			if commentStart >= 0 && bytes.Equal(input[commentStart:commentStart+5], []byte("COMNT")) {
				cut = commentStart
			}
		}
	}
	if cut > 0 && input[cut-1] == 0x1A {
		cut--
	}
	return input[:cut]
}

// cleanDIZ strips trailing whitespace and DOS-era control characters
// (Ctrl-Z / 0x1A CP/M EOF marker) from FILE_ID.DIZ content, and
// converts any CP437 high bytes to their UTF-8 equivalents so the
// result is valid UTF-8 that survives JSON serialisation.
func cleanDIZ(raw string) string {
	s := strings.TrimRight(raw, " \t\r\n\x1a")
	s = strings.ReplaceAll(s, "\x1a", "")
	return cp437BytesToUTF8(s)
}

// cp437BytesToUTF8 converts a string that may contain raw CP437 high bytes
// (0x80–0xFF) to valid UTF-8, preserving bytes that are already valid UTF-8.
func cp437BytesToUTF8(s string) string {
	b := []byte(s)
	// Fast path: if already valid UTF-8 with no high bytes, return as-is.
	allASCII := true
	for _, c := range b {
		if c >= 0x80 {
			allASCII = false
			break
		}
	}
	if allASCII {
		return s
	}

	var out []byte
	i := 0
	for i < len(b) {
		r, size := utf8.DecodeRune(b[i:])
		if r != utf8.RuneError || size != 1 {
			// Already a valid UTF-8 rune — keep as-is.
			out = append(out, b[i:i+size]...)
			i += size
		} else {
			// Invalid UTF-8 byte — treat as CP437.
			cp := b[i]
			mapped := ansi.Cp437ToUnicode[cp]
			if mapped != 0 {
				out = append(out, []byte(string(mapped))...)
			}
			// Drop bytes that have no CP437 mapping (shouldn't happen).
			i++
		}
	}
	return string(out)
}

// ExtractDIZFromZip opens a ZIP archive and reads the file description,
// preferring FILE_ID.ANS over FILE_ID.DIZ. Returns empty string if neither found.
func ExtractDIZFromZip(archivePath string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip %s: %w", archivePath, err)
	}
	defer r.Close()

	var dizFile *zip.File
	for _, f := range r.File {
		baseName := filepath.Base(f.Name)
		if strings.EqualFold(baseName, "FILE_ID.ANS") {
			dizFile = f
			break // ANS takes priority; stop searching
		}
		if strings.EqualFold(baseName, "FILE_ID.DIZ") && dizFile == nil {
			dizFile = f // keep as fallback, continue in case ANS appears later
		}
	}

	if dizFile == nil {
		return "", nil
	}

	rc, err := dizFile.Open()
	if err != nil {
		return "", fmt.Errorf("failed to read %s from %s: %w", dizFile.Name, archivePath, err)
	}
	defer rc.Close()

	data, readErr := io.ReadAll(io.LimitReader(rc, 10*1024))
	if readErr != nil {
		return "", fmt.Errorf("failed to read %s from %s: %w", dizFile.Name, archivePath, readErr)
	}
	return cleanDIZ(string(stripSauceMetadata(data))), nil
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
