package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stlalpha/vision3/internal/archiver"
	"github.com/stlalpha/vision3/internal/ziplab"
)

func cmdFilesReextractDIZ(args []string) {
	fs := flag.NewFlagSet("files reextractdiz", flag.ExitOnError)
	areaTag := fs.String("area", "", "Specific file area tag (omit for all areas)")
	dataDir := fs.String("data", "data", "Data directory")
	configDir := fs.String("config", "configs", "Config directory")
	dryRun := fs.Bool("dry-run", false, "Show what would be updated without making changes")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: helper files reextractdiz [options]\n\n")
		fmt.Fprintf(os.Stderr, "Re-extract FILE_ID.DIZ from archives and update file descriptions.\n")
		fmt.Fprintf(os.Stderr, "Processes all areas or a specific area.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  helper files reextractdiz --area GENERAL\n")
		fmt.Fprintf(os.Stderr, "  helper files reextractdiz --dry-run\n")
	}
	fs.Parse(args)

	areas, err := loadFileAreas(*configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading file areas: %v\n", err)
		os.Exit(1)
	}

	if *areaTag != "" {
		area := findAreaByTag(areas, *areaTag)
		if area == nil {
			fmt.Fprintf(os.Stderr, "Error: file area %q not found\n", *areaTag)
			os.Exit(1)
		}
		areas = areas[:0]
		areas = append(areas, *area)
	}

	arcCfg, _ := archiver.LoadConfig(*configDir)

	totalUpdated := 0
	totalScanned := 0

	for _, area := range areas {
		areaDir := filepath.Join(*dataDir, "files", area.Path)
		records, err := loadMetadata(areaDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading metadata for %s: %v\n", area.Tag, err)
			continue
		}
		if len(records) == 0 {
			continue
		}

		fmt.Printf("Area: %s (%s) — %d files\n", area.Name, area.Tag, len(records))
		updated := 0

		for i := range records {
			rec := &records[i]
			totalScanned++

			if !arcCfg.IsSupported(rec.Filename) {
				continue
			}

			filePath := filepath.Join(areaDir, rec.Filename)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				fmt.Printf("  MISS  %-40s (file not found on disk)\n", rec.Filename)
				continue
			}

			diz, err := ziplab.ExtractDIZFromArchive(filePath, *configDir)
			if err != nil {
				fmt.Printf("  ERR   %-40s %v\n", rec.Filename, err)
				continue
			}

			if diz == "" {
				continue
			}

			if diz == rec.Description {
				continue
			}

			firstLine := strings.SplitN(diz, "\n", 2)[0]
			if len(firstLine) > 60 {
				firstLine = firstLine[:57] + "..."
			}

			if *dryRun {
				fmt.Printf("  UPD   %-40s [DIZ: %s]\n", rec.Filename, firstLine)
			} else {
				rec.Description = diz
				fmt.Printf("  UPD   %-40s [DIZ: %s]\n", rec.Filename, firstLine)
			}
			updated++
		}

		if !*dryRun && updated > 0 {
			if err := saveMetadata(areaDir, records); err != nil {
				fmt.Fprintf(os.Stderr, "  Error saving metadata for %s: %v\n", area.Tag, err)
				continue
			}
		}

		totalUpdated += updated
	}

	fmt.Printf("\nSummary: scanned %d files, updated %d descriptions\n", totalScanned, totalUpdated)
	if *dryRun {
		fmt.Println("(dry run — no files were modified)")
	}
}
