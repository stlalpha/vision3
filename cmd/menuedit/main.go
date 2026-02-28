// Command menuedit is the ViSiON/3 BBS Menu Editor.
// It provides a TUI for managing menu definitions stored in .MNU and .CFG files,
// faithfully recreating the original Turbo Pascal MENUEDIT.EXE from Vision/2.
//
// Usage:
//
//	./menuedit [--menus path/to/menus/set]
//
// If no --menus flag is provided, it looks for menus/v3 relative to the
// current working directory.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/stlalpha/vision3/internal/menueditor"
)

func main() {
	menusPath := flag.String("menus", "", "Path to menu set directory (default: menus/v3)")
	flag.Parse()

	// Resolve menus path
	path := *menusPath
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		path = filepath.Join(cwd, "menus", "v3")
	}

	// Verify the directory exists
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: menus directory not found: %s\n", path)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", path)
		os.Exit(1)
	}

	// Verify mnu/ and cfg/ subdirectories exist
	for _, sub := range []string{"mnu", "cfg"} {
		subPath := filepath.Join(path, sub)
		if _, err := os.Stat(subPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: required subdirectory not found: %s\n", subPath)
			os.Exit(1)
		}
	}

	// Create the editor model
	model, err := menueditor.New(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing editor: %v\n", err)
		os.Exit(1)
	}

	// Run the BubbleTea TUI
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
