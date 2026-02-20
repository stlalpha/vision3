// Command ue is the ViSiON/3 BBS User Editor.
// It provides a TUI for managing user accounts stored in users.json,
// faithfully recreating the original Turbo Pascal UE.EXE v1.3 from Vision/2.
//
// Usage:
//
//	./ue [--data path/to/users/directory]
//
// If no --data flag is provided, it looks for data/users/users.json
// relative to the current working directory.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/stlalpha/vision3/internal/usereditor"
)

func main() {
	dataPath := flag.String("data", "", "Path to users directory (default: data/users/)")
	flag.Parse()

	// Resolve data path
	path := *dataPath
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		path = filepath.Join(cwd, "data", "users")
	}

	// Build full path to users.json
	usersFile := filepath.Join(path, "users.json")

	// Verify the file exists
	if _, err := os.Stat(usersFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: users.json not found: %s\n", usersFile)
		os.Exit(1)
	}

	// Create the editor model
	model, err := usereditor.New(usersFile)
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
