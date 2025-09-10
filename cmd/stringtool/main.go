package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/stlalpha/vision3/internal/configtool/strings"
)

func main() {
	var (
		configPath = flag.String("config", "./configs", "Path to configuration directory")
		version    = flag.Bool("version", false, "Show version information")
		help       = flag.Bool("help", false, "Show help information")
	)
	flag.Parse()

	if *version {
		fmt.Println("ViSiON/3 String Configuration Tool v1.0.0")
		fmt.Println("Turbo Pascal-style String Editor with ANSI Color Support")
		os.Exit(0)
	}

	if *help {
		showHelp()
		os.Exit(0)
	}

	// Validate config path
	if !pathExists(*configPath) {
		log.Fatalf("Configuration path does not exist: %s", *configPath)
	}

	// Check if strings.json exists
	stringsFile := filepath.Join(*configPath, "strings.json")
	if !pathExists(stringsFile) {
		log.Fatalf("strings.json not found in configuration path: %s", stringsFile)
	}

	// Run the TUI
	fmt.Println("Starting ViSiON/3 String Configuration Manager...")
	fmt.Println("Loading configuration from:", *configPath)
	fmt.Println("Press ? for help once the interface loads.")
	fmt.Println()

	if err := strings.RunTUI(*configPath); err != nil {
		log.Fatalf("Error running string manager: %v", err)
	}

	fmt.Println("String Configuration Manager closed.")
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func showHelp() {
	fmt.Println("ViSiON/3 String Configuration Tool")
	fmt.Println("==================================")
	fmt.Println()
	fmt.Println("A Turbo Pascal-style TUI for editing BBS string configurations.")
	fmt.Println("Features:")
	fmt.Println("  • Multi-pane interface with categories, string list, and editor")
	fmt.Println("  • Real-time ANSI color preview")
	fmt.Println("  • Color picker with vintage-style dialog boxes")
	fmt.Println("  • Search and filter functionality")
	fmt.Println("  • Undo/redo support")
	fmt.Println("  • Bulk import/export")
	fmt.Println("  • Live preview pane showing formatted output")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  stringtool [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -config <path>     Path to configuration directory (default: ./configs)")
	fmt.Println("  -version           Show version information")
	fmt.Println("  -help              Show this help")
	fmt.Println()
	fmt.Println("Key Features:")
	fmt.Println("  • Categories Pane: Browse string categories (Login, Messages, etc.)")
	fmt.Println("  • String List: View all strings in selected category")
	fmt.Println("  • Editor Pane: Edit strings with ANSI color support")
	fmt.Println("  • Preview Pane: See real-time formatted output")
	fmt.Println()
	fmt.Println("Keyboard Shortcuts:")
	fmt.Println("  Tab/Shift+Tab      Navigate between panes")
	fmt.Println("  Enter              Select/Edit current item")
	fmt.Println("  F2                 Open color picker")
	fmt.Println("  F3                 Toggle preview pane")
	fmt.Println("  Ctrl+S             Save current string")
	fmt.Println("  Ctrl+Z/Ctrl+Y      Undo/Redo changes")
	fmt.Println("  /                  Search strings")
	fmt.Println("  F4/F5              Export/Import configuration")
	fmt.Println("  ?                  Show help")
	fmt.Println("  q/Ctrl+C           Quit")
	fmt.Println()
	fmt.Println("ANSI Color Codes:")
	fmt.Println("  |00-|15            Standard and bright colors")
	fmt.Println("  |B0-|B7            Background colors")
	fmt.Println("  |C1-|C7            Custom configurable colors")
	fmt.Println("  |CL                Clear screen")
	fmt.Println("  |P/|PP             Save/restore cursor position")
	fmt.Println("  |23                Reset all attributes")
	fmt.Println()
	fmt.Println("Example Usage:")
	fmt.Println("  stringtool                           # Use default config path")
	fmt.Println("  stringtool -config /path/to/configs  # Use custom config path")
	fmt.Println()
	fmt.Println("Configuration Files:")
	fmt.Println("  The tool expects to find 'strings.json' in the specified")
	fmt.Println("  configuration directory. This file contains all 95+ configurable")
	fmt.Println("  strings used by the ViSiON/3 BBS system.")
	fmt.Println()
}