package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stlalpha/vision3/internal/archiver"
	"github.com/stlalpha/vision3/internal/file"
	"github.com/stlalpha/vision3/internal/ziplab"
)

type importStats struct {
	imported int
	skipped  int
	errors   int
}

func cmdFilesImport(args []string) {
	fs := flag.NewFlagSet("files import", flag.ExitOnError)
	dir := fs.String("dir", "", "Source directory containing files to import (required)")
	areaTag := fs.String("area", "", "Target file area tag, e.g. GENERAL (required)")
	uploader := fs.String("uploader", "Sysop", "Uploader handle")
	dataDir := fs.String("data", "data", "Data directory")
	configDir := fs.String("config", "configs", "Config directory")
	moveFiles := fs.Bool("move", false, "Move files instead of copying")
	dryRun := fs.Bool("dry-run", false, "Show what would be imported without making changes")
	noDIZ := fs.Bool("no-diz", false, "Skip FILE_ID.DIZ extraction from archives")
	preserveDates := fs.Bool("preserve-dates", false, "Use file modification time as upload date")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: helper files import [options]\n\n")
		fmt.Fprintf(os.Stderr, "Bulk import files from a directory into a file area.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  helper files import --dir /mnt/cdrom/files --area GENERAL\n")
		fmt.Fprintf(os.Stderr, "  helper files import --dir ~/staging --area UTILS --preserve-dates --dry-run\n")
		fmt.Fprintf(os.Stderr, "  helper files import --dir /tmp/incoming --area UPLOADS --move --uploader Admin\n")
	}
	fs.Parse(args)

	if *dir == "" || *areaTag == "" {
		fmt.Fprintf(os.Stderr, "Error: --dir and --area are required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	srcInfo, err := os.Stat(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s: %v\n", *dir, err)
		os.Exit(1)
	}
	if !srcInfo.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", *dir)
		os.Exit(1)
	}

	areas, err := loadFileAreas(*configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading file areas: %v\n", err)
		os.Exit(1)
	}

	area := findAreaByTag(areas, *areaTag)
	if area == nil {
		fmt.Fprintf(os.Stderr, "Error: file area %q not found\n", *areaTag)
		fmt.Fprintf(os.Stderr, "\nAvailable areas:\n")
		for _, a := range areas {
			fmt.Fprintf(os.Stderr, "  %-12s %s\n", a.Tag, a.Name)
		}
		os.Exit(1)
	}

	areaDir := filepath.Join(*dataDir, "files", area.Path)
	if err := os.MkdirAll(areaDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating area directory %s: %v\n", areaDir, err)
		os.Exit(1)
	}

	existingRecords, err := loadMetadata(areaDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading metadata for area %s: %v\n", area.Tag, err)
		os.Exit(1)
	}

	existingNames := make(map[string]bool, len(existingRecords))
	for _, r := range existingRecords {
		existingNames[strings.ToUpper(r.Filename)] = true
	}

	entries, err := os.ReadDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading source directory: %v\n", err)
		os.Exit(1)
	}

	var candidates []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if shouldSkipFile(name) {
			continue
		}
		candidates = append(candidates, entry)
	}

	if len(candidates) == 0 {
		fmt.Println("No importable files found in source directory.")
		return
	}

	arcCfg, err := archiver.LoadConfig(*configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load archiver config from %s: %v (using defaults)\n", *configDir, err)
	}

	fmt.Printf("Import to: %s (%s)\n", area.Name, area.Tag)
	fmt.Printf("Area path: %s\n", areaDir)
	fmt.Printf("Source:    %s\n", *dir)
	fmt.Printf("Uploader:  %s\n", *uploader)
	fmt.Printf("Files:     %d candidates\n", len(candidates))
	if *dryRun {
		fmt.Println("Mode:      DRY RUN")
	} else if *moveFiles {
		fmt.Println("Mode:      MOVE")
	} else {
		fmt.Println("Mode:      COPY")
	}
	fmt.Println()

	stats := importStats{}
	var newRecords []file.FileRecord

	for _, entry := range candidates {
		name := entry.Name()
		srcPath := filepath.Join(*dir, name)

		if existingNames[strings.ToUpper(name)] {
			fmt.Printf("  SKIP  %-40s (duplicate)\n", name)
			stats.skipped++
			continue
		}

		info, err := entry.Info()
		if err != nil {
			fmt.Printf("  ERR   %-40s %v\n", name, err)
			stats.errors++
			continue
		}

		uploadTime := time.Now()
		if *preserveDates {
			uploadTime = info.ModTime()
		}

		var description string
		if !*noDIZ && arcCfg.IsSupported(name) {
			diz, dizErr := ziplab.ExtractDIZFromArchive(srcPath, *configDir)
			if dizErr != nil {
				fmt.Printf("  WARN  %-40s DIZ extraction failed: %v\n", name, dizErr)
			} else if diz != "" {
				description = diz
			}
		}

		if *dryRun {
			dizNote := ""
			if description != "" {
				firstLine := strings.SplitN(description, "\n", 2)[0]
				if len(firstLine) > 50 {
					firstLine = firstLine[:47] + "..."
				}
				dizNote = fmt.Sprintf(" [DIZ: %s]", firstLine)
			}
			fmt.Printf("  ADD   %-40s %10s%s\n", name, formatSize(info.Size()), dizNote)
			stats.imported++
			continue
		}

		destPath := filepath.Join(areaDir, name)
		if *moveFiles {
			err = moveFile(srcPath, destPath)
		} else {
			err = copyFile(srcPath, destPath)
		}
		if err != nil {
			fmt.Printf("  ERR   %-40s %v\n", name, err)
			stats.errors++
			continue
		}

		record := file.FileRecord{
			ID:          uuid.New(),
			AreaID:      area.ID,
			Filename:    name,
			Description: description,
			Size:        info.Size(),
			UploadedAt:  uploadTime,
			UploadedBy:  *uploader,
		}
		newRecords = append(newRecords, record)
		existingNames[strings.ToUpper(name)] = true

		dizTag := ""
		if description != "" {
			dizTag = " +DIZ"
		}
		fmt.Printf("  OK    %-40s %10s%s\n", name, formatSize(info.Size()), dizTag)
		stats.imported++
	}

	if !*dryRun && len(newRecords) > 0 {
		allRecords := append(existingRecords, newRecords...)
		if err := saveMetadata(areaDir, allRecords); err != nil {
			fmt.Fprintf(os.Stderr, "\nError saving metadata: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("\nSummary: %d imported, %d skipped, %d errors\n", stats.imported, stats.skipped, stats.errors)
	if *dryRun {
		fmt.Println("(dry run — no files were modified)")
	}
}

func shouldSkipFile(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(name, ".") {
		return true
	}
	skipNames := []string{"files.bbs", "metadata.json", "thumbs.db", ".ds_store", "desktop.ini"}
	for _, skip := range skipNames {
		if lower == skip {
			return true
		}
	}
	return false
}

func findAreaByTag(areas []file.FileArea, tag string) *file.FileArea {
	upper := strings.ToUpper(tag)
	for i := range areas {
		if strings.ToUpper(areas[i].Tag) == upper {
			return &areas[i]
		}
	}
	return nil
}

func loadFileAreas(configDir string) ([]file.FileArea, error) {
	path := filepath.Join(configDir, "file_areas.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var areas []file.FileArea
	if err := json.Unmarshal(data, &areas); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return areas, nil
}

func loadMetadata(areaDir string) ([]file.FileRecord, error) {
	path := filepath.Join(areaDir, "metadata.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var records []file.FileRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return records, nil
}

func saveMetadata(areaDir string, records []file.FileRecord) error {
	path := filepath.Join(areaDir, "metadata.json")
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		if cpErr := copyFile(tmp, path); cpErr != nil {
			os.Remove(tmp)
			return err
		}
		os.Remove(tmp)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		if err := copyFile(src, dst); err != nil {
			return err
		}
		return os.Remove(src)
	}
	return nil
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
