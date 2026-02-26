// Command config is the ViSiON/3 BBS Configuration Editor.
// It provides a TUI for managing all system configuration files,
// faithfully recreating the original Turbo Pascal CONFIG.EXE from Vision/2.
//
// Usage:
//
//	./config [--config path/to/configs/directory]
//
// If no --config flag is provided, it looks for configs/
// relative to the current working directory.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/stlalpha/vision3/internal/configeditor"
)

func main() {
	configPath := flag.String("config", "", "Path to configs directory (default: configs/)")
	flag.Parse()

	// Resolve config path
	path := *configPath
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		path = filepath.Join(cwd, "configs")
	}

	// Suppress log output from config loaders (they log to default logger)
	log.SetOutput(io.Discard)

	// Verify the directory exists
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: config directory not found: %s\n", path)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", path)
		os.Exit(1)
	}

	// Create the editor model
	model, err := configeditor.New(path)
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
