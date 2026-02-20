// Command strings is the ViSiON/3 BBS string configuration editor.
// It provides a TUI for editing the BBS prompt strings stored in strings.json,
// faithfully recreating the original Turbo Pascal STRINGS.EXE from Vision/2.
//
// Usage:
//
//./strings [--config path/to/strings.json]
//
// If no --config flag is provided, it looks for configs/strings.json
// relative to the current working directory.
package main

import (
"flag"
"fmt"
"os"
"path/filepath"

tea "github.com/charmbracelet/bubbletea"

"github.com/stlalpha/vision3/internal/stringeditor"
)

func main() {
configPath := flag.String("config", "", "Path to strings.json (default: configs/strings.json)")
flag.Parse()

// Resolve config path
path := *configPath
if path == "" {
// Default: configs/strings.json relative to CWD
cwd, err := os.Getwd()
if err != nil {
fmt.Fprintf(os.Stderr, "Error: %v\n", err)
os.Exit(1)
}
path = filepath.Join(cwd, "configs", "strings.json")
}

// Verify the config directory exists
dir := filepath.Dir(path)
if _, err := os.Stat(dir); os.IsNotExist(err) {
fmt.Fprintf(os.Stderr, "Error: config directory does not exist: %s\n", dir)
os.Exit(1)
}

// Create the editor model
model, err := stringeditor.New(path)
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
