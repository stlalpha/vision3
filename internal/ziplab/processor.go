package ziplab

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Processor runs the ZipLab pipeline steps against an uploaded archive.
type Processor struct {
	config  Config
	baseDir string // Base directory for resolving relative paths
}

// NewProcessor creates a new ZipLab processor.
func NewProcessor(cfg Config, baseDir string) *Processor {
	return &Processor{
		config:  cfg,
		baseDir: baseDir,
	}
}

// StepTestIntegrity (Step 1) tests the archive for corruption.
// For native ZIP, it opens and reads every file entry.
// For external formats, it runs the configured test command.
func (p *Processor) StepTestIntegrity(archivePath string) error {
	if !p.config.Steps.TestIntegrity.Enabled {
		log.Printf("INFO: ZipLab step 1 (test integrity) skipped — disabled")
		return nil
	}

	at, ok := p.config.GetArchiveType(archivePath)
	if !ok {
		return fmt.Errorf("unsupported archive type: %s", filepath.Ext(archivePath))
	}

	if at.Native {
		return p.testZipIntegrity(archivePath)
	}
	return p.runExternalCommand(at.TestCommand, at.TestArgs, archivePath, "")
}

// testZipIntegrity opens a ZIP and reads every entry to verify integrity.
func (p *Processor) testZipIntegrity(zipPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip %s: %w", zipPath, err)
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("corrupt entry %s: %w", f.Name, err)
		}
		if _, err := io.Copy(io.Discard, rc); err != nil {
			rc.Close()
			return fmt.Errorf("corrupt data in %s: %w", f.Name, err)
		}
		rc.Close()
	}
	return nil
}

// StepExtract (Step 2) extracts the archive to a temporary work directory.
// Returns the path to the work directory.
func (p *Processor) StepExtract(archivePath string) (string, error) {
	if !p.config.Steps.ExtractToTemp.Enabled {
		log.Printf("INFO: ZipLab step 2 (extract) skipped — disabled")
		return "", nil
	}

	at, ok := p.config.GetArchiveType(archivePath)
	if !ok {
		return "", fmt.Errorf("unsupported archive type: %s", filepath.Ext(archivePath))
	}

	workDir, err := os.MkdirTemp("", "ziplab-extract-*")
	if err != nil {
		return "", fmt.Errorf("failed to create work directory: %w", err)
	}

	if at.Native {
		if err := p.extractZip(archivePath, workDir); err != nil {
			os.RemoveAll(workDir)
			return "", err
		}
		return workDir, nil
	}

	if err := p.runExternalCommand(at.ExtractCommand, at.ExtractArgs, archivePath, workDir); err != nil {
		os.RemoveAll(workDir)
		return "", err
	}
	return workDir, nil
}

// extractZip extracts all files from a ZIP archive to destDir.
func (p *Processor) extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip %s: %w", zipPath, err)
	}
	defer r.Close()

	for _, f := range r.File {
		targetPath := filepath.Join(destDir, f.Name)

		// Prevent zip slip
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(targetPath, 0755)
			continue
		}

		// Ensure parent directory exists
		os.MkdirAll(filepath.Dir(targetPath), 0755)

		outFile, err := os.Create(targetPath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", targetPath, err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to open zip entry %s: %w", f.Name, err)
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			rc.Close()
			outFile.Close()
			return fmt.Errorf("failed to extract %s: %w", f.Name, err)
		}

		rc.Close()
		outFile.Close()
	}
	return nil
}

// StepVirusScan (Step 3) runs a configurable external virus scanner.
func (p *Processor) StepVirusScan(workDir string) error {
	if !p.config.Steps.VirusScan.Enabled {
		log.Printf("INFO: ZipLab step 3 (virus scan) skipped — disabled")
		return nil
	}

	step := p.config.Steps.VirusScan
	return p.runExternalCommand(step.Command, step.Args, "", workDir)
}

// StepRemoveAdsAndDIZ (Step 5) extracts FILE_ID.DIZ content and removes
// unwanted files matching patterns from REMOVE.TXT.
// workDir is used to find FILE_ID.DIZ; archivePath is the ZIP to strip ad files from.
func (p *Processor) StepRemoveAdsAndDIZ(workDir, archivePath string) (string, error) {
	if !p.config.Steps.RemoveAds.Enabled {
		log.Printf("INFO: ZipLab step 5 (remove ads/DIZ) skipped — disabled")
		return "", nil
	}

	// Extract FILE_ID.DIZ (case-insensitive search)
	diz := p.findAndReadDIZ(workDir)

	// Load removal patterns
	patterns := p.loadRemovePatterns()

	// Remove matching files from work directory
	for _, pattern := range patterns {
		p.removeMatchingFiles(workDir, pattern)
	}

	// Remove matching files from the archive itself
	if len(patterns) > 0 && archivePath != "" {
		at, ok := p.config.GetArchiveType(archivePath)
		if ok && at.Native {
			if err := p.removeFilesFromZip(archivePath, patterns); err != nil {
				log.Printf("WARN: failed to remove ad files from archive: %v", err)
			}
		}
	}

	return diz, nil
}

// removeFilesFromZip rewrites a ZIP excluding entries that match any of the patterns (case-insensitive).
func (p *Processor) removeFilesFromZip(zipPath string, patterns []string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}

	tmpPath := zipPath + ".tmp"
	outFile, err := os.Create(tmpPath)
	if err != nil {
		r.Close()
		return fmt.Errorf("failed to create temp zip: %w", err)
	}

	w := zip.NewWriter(outFile)
	if r.Comment != "" {
		w.SetComment(r.Comment)
	}

	removed := 0
	for _, f := range r.File {
		if shouldRemoveFile(f.Name, patterns) {
			log.Printf("INFO: removing ad file from archive: %s", f.Name)
			removed++
			continue
		}

		fw, err := w.CreateHeader(&f.FileHeader)
		if err != nil {
			w.Close()
			outFile.Close()
			r.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to copy entry %s: %w", f.Name, err)
		}
		if !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				w.Close()
				outFile.Close()
				r.Close()
				os.Remove(tmpPath)
				return fmt.Errorf("failed to read entry %s: %w", f.Name, err)
			}
			io.Copy(fw, rc)
			rc.Close()
		}
	}

	r.Close()
	if err := w.Close(); err != nil {
		outFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to finalize zip: %w", err)
	}
	outFile.Close()

	if removed == 0 {
		os.Remove(tmpPath)
		return nil
	}

	return os.Rename(tmpPath, zipPath)
}

// shouldRemoveFile checks if a filename matches any removal pattern (case-insensitive).
func shouldRemoveFile(name string, patterns []string) bool {
	baseName := filepath.Base(name)
	for _, pattern := range patterns {
		if strings.EqualFold(baseName, pattern) {
			return true
		}
	}
	return false
}

// findAndReadDIZ searches for FILE_ID.DIZ (case-insensitive) and returns its content.
func (p *Processor) findAndReadDIZ(workDir string) string {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(entry.Name(), "FILE_ID.DIZ") {
			data, err := os.ReadFile(filepath.Join(workDir, entry.Name()))
			if err != nil {
				log.Printf("WARN: found FILE_ID.DIZ but failed to read: %v", err)
				return ""
			}
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// loadRemovePatterns reads filenames to remove from the patterns file.
func (p *Processor) loadRemovePatterns() []string {
	patternsPath := p.config.Steps.RemoveAds.PatternsFile
	if patternsPath == "" {
		return nil
	}

	f, err := os.Open(patternsPath)
	if err != nil {
		log.Printf("WARN: could not open patterns file %s: %v", patternsPath, err)
		return nil
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, ";") {
			patterns = append(patterns, line)
		}
	}
	return patterns
}

// removeMatchingFiles removes files matching a pattern (case-insensitive) from a directory.
func (p *Processor) removeMatchingFiles(dir, pattern string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(entry.Name(), pattern) {
			target := filepath.Join(dir, entry.Name())
			if err := os.Remove(target); err != nil {
				log.Printf("WARN: failed to remove %s: %v", target, err)
			} else {
				log.Printf("INFO: removed ad file: %s", entry.Name())
			}
		}
	}
}

// StepAddComment (Step 6) adds a ZIP comment from the configured comment file.
func (p *Processor) StepAddComment(archivePath string) error {
	if !p.config.Steps.AddComment.Enabled {
		log.Printf("INFO: ZipLab step 6 (add comment) skipped — disabled")
		return nil
	}

	at, ok := p.config.GetArchiveType(archivePath)
	if !ok {
		return fmt.Errorf("unsupported archive type: %s", filepath.Ext(archivePath))
	}

	commentFile := p.config.Steps.AddComment.CommentFile
	commentData, err := os.ReadFile(commentFile)
	if err != nil {
		return fmt.Errorf("failed to read comment file %s: %w", commentFile, err)
	}
	comment := strings.TrimSpace(string(commentData))

	if at.Native {
		return p.setZipComment(archivePath, comment)
	}
	return p.runExternalCommand(at.CommentCommand, at.CommentArgs, archivePath, "")
}

// setZipComment rewrites a ZIP file with the given comment.
func (p *Processor) setZipComment(zipPath, comment string) error {
	// Read existing ZIP
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}

	// Create temp file for rewrite
	tmpPath := zipPath + ".tmp"
	outFile, err := os.Create(tmpPath)
	if err != nil {
		r.Close()
		return fmt.Errorf("failed to create temp zip: %w", err)
	}

	w := zip.NewWriter(outFile)
	w.SetComment(comment)

	// Copy all entries
	for _, f := range r.File {
		fw, err := w.CreateHeader(&f.FileHeader)
		if err != nil {
			w.Close()
			outFile.Close()
			r.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to copy entry %s: %w", f.Name, err)
		}
		if !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				w.Close()
				outFile.Close()
				r.Close()
				os.Remove(tmpPath)
				return fmt.Errorf("failed to read entry %s: %w", f.Name, err)
			}
			if _, err := io.Copy(fw, rc); err != nil {
				rc.Close()
				w.Close()
				outFile.Close()
				r.Close()
				os.Remove(tmpPath)
				return fmt.Errorf("failed to copy data for %s: %w", f.Name, err)
			}
			rc.Close()
		}
	}

	r.Close()
	if err := w.Close(); err != nil {
		outFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to finalize zip: %w", err)
	}
	outFile.Close()

	// Replace original with rewritten file
	return os.Rename(tmpPath, zipPath)
}

// StepIncludeFile (Step 7) adds a file (e.g., BBS.AD) into the archive.
func (p *Processor) StepIncludeFile(archivePath string) error {
	if !p.config.Steps.IncludeFile.Enabled {
		log.Printf("INFO: ZipLab step 7 (include file) skipped — disabled")
		return nil
	}

	at, ok := p.config.GetArchiveType(archivePath)
	if !ok {
		return fmt.Errorf("unsupported archive type: %s", filepath.Ext(archivePath))
	}

	includeFilePath := p.config.Steps.IncludeFile.FilePath
	includeData, err := os.ReadFile(includeFilePath)
	if err != nil {
		return fmt.Errorf("failed to read include file %s: %w", includeFilePath, err)
	}

	if at.Native {
		return p.addFileToZip(archivePath, filepath.Base(includeFilePath), includeData)
	}
	return p.runExternalCommand(at.AddCommand, at.AddArgs, archivePath, "")
}

// addFileToZip rewrites a ZIP adding a new file entry.
func (p *Processor) addFileToZip(zipPath, name string, data []byte) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}

	tmpPath := zipPath + ".tmp"
	outFile, err := os.Create(tmpPath)
	if err != nil {
		r.Close()
		return fmt.Errorf("failed to create temp zip: %w", err)
	}

	w := zip.NewWriter(outFile)

	// Preserve existing comment
	if r.Comment != "" {
		w.SetComment(r.Comment)
	}

	// Copy existing entries
	for _, f := range r.File {
		fw, err := w.CreateHeader(&f.FileHeader)
		if err != nil {
			w.Close()
			outFile.Close()
			r.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to copy entry %s: %w", f.Name, err)
		}
		if !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				w.Close()
				outFile.Close()
				r.Close()
				os.Remove(tmpPath)
				return err
			}
			io.Copy(fw, rc)
			rc.Close()
		}
	}

	r.Close()

	// Add the new file
	fw, err := w.Create(name)
	if err != nil {
		w.Close()
		outFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to add %s: %w", name, err)
	}
	if _, err := fw.Write(data); err != nil {
		w.Close()
		outFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write %s: %w", name, err)
	}

	if err := w.Close(); err != nil {
		outFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to finalize zip: %w", err)
	}
	outFile.Close()

	return os.Rename(tmpPath, zipPath)
}

// runExternalCommand runs an external command with placeholder substitution.
func (p *Processor) runExternalCommand(command string, args []string, archivePath, workDir string) error {
	if command == "" {
		return fmt.Errorf("no command configured")
	}

	expandedArgs := make([]string, len(args))
	for i, arg := range args {
		arg = strings.ReplaceAll(arg, "{FILE}", archivePath)
		arg = strings.ReplaceAll(arg, "{WORKDIR}", workDir)
		expandedArgs[i] = arg
	}

	timeout := 60 * time.Second
	// Note: timeout from step config would be applied by the caller

	cmd := exec.Command(command, expandedArgs...)
	cmd.Dir = workDir

	output, err := runWithTimeout(cmd, timeout)
	if err != nil {
		return fmt.Errorf("command %s failed: %w (output: %s)", command, err, output)
	}
	return nil
}

// runWithTimeout executes a command with a timeout.
func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) (string, error) {
	done := make(chan error, 1)
	var output []byte

	go func() {
		var err error
		output, err = cmd.CombinedOutput()
		done <- err
	}()

	select {
	case err := <-done:
		return string(output), err
	case <-time.After(timeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return "", fmt.Errorf("command timed out after %v", timeout)
	}
}
