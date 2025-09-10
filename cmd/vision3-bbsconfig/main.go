package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/stlalpha/vision3/internal/configtool/ui"
)

const (
	Version = "1.0.0"
	Banner  = `
██╗   ██╗██╗███████╗██╗ ██████╗ ███╗   ██╗    ██████╗ 
██║   ██║██║██╔════╝██║██╔═══██╗████╗  ██║    ╚════██╗
██║   ██║██║███████╗██║██║   ██║██╔██╗ ██║     █████╔╝
╚██╗ ██╔╝██║╚════██║██║██║   ██║██║╚██╗██║     ╚═══██╗
 ╚████╔╝ ██║███████║██║╚██████╔╝██║ ╚████║    ██████╔╝
  ╚═══╝  ╚═╝╚══════╝╚═╝ ╚═════╝ ╚═╝  ╚═══╝    ╚═════╝ 

                BBS Configuration Tool
             Multi-Node Binary Database System
                      Version %s
`
)

func main() {
	var (
		basePath = flag.String("path", "./bbsdata", "Base path for BBS data")
		nodeNum  = flag.Int("node", 1, "Node number (1-255)")
		version  = flag.Bool("version", false, "Show version information")
		help     = flag.Bool("help", false, "Show help information")
	)
	flag.Parse()

	if *version {
		fmt.Printf("Vision/3 BBS Configuration Tool v%s\n", Version)
		fmt.Println("Multi-Node Binary Database System")
		fmt.Println("Copyright (c) 2024 Vision/3 Development Team")
		return
	}

	if *help {
		fmt.Printf(Banner, Version)
		fmt.Println("\nUsage:")
		flag.PrintDefaults()
		fmt.Println("\nFeatures:")
		fmt.Println("  • Binary message base system with classic BBS-style storage")
		fmt.Println("  • Binary file base system with .DIR format compatibility") 
		fmt.Println("  • Multi-node safety with file locking mechanisms")
		fmt.Println("  • Real-time node monitoring and coordination")
		fmt.Println("  • Database maintenance utilities (pack, reindex, repair)")
		fmt.Println("  • Duplicate file detection and management")
		fmt.Println("  • Turbo Pascal-style configuration interface")
		fmt.Println("  • System backup and restore capabilities")
		return
	}

	// Validate node number
	if *nodeNum < 1 || *nodeNum > 255 {
		log.Fatalf("ERROR: Node number must be between 1 and 255")
	}

	// Ensure base path exists
	absBasePath, err := filepath.Abs(*basePath)
	if err != nil {
		log.Fatalf("ERROR: Invalid base path: %v", err)
	}

	if err := os.MkdirAll(absBasePath, 0755); err != nil {
		log.Fatalf("ERROR: Failed to create base directory: %v", err)
	}

	// Initialize logging
	logPath := filepath.Join(absBasePath, "logs")
	if err := os.MkdirAll(logPath, 0755); err != nil {
		log.Printf("WARNING: Failed to create log directory: %v", err)
	} else {
		logFile := filepath.Join(logPath, fmt.Sprintf("config-node%d.log", *nodeNum))
		if file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			log.SetOutput(file)
			defer file.Close()
		}
	}

	log.Printf("Starting Vision/3 BBS Configuration Tool v%s", Version)
	log.Printf("Node: %d, Base Path: %s", *nodeNum, absBasePath)

	// Show banner
	fmt.Printf(Banner, Version)
	fmt.Printf("\nNode %d - Base Path: %s\n", *nodeNum, absBasePath)
	fmt.Println("Initializing multi-node binary database system...")

	// Create and run configuration manager
	configManager, err := ui.NewConfigManager(absBasePath, uint8(*nodeNum))
	if err != nil {
		log.Fatalf("ERROR: Failed to initialize configuration manager: %v", err)
	}

	// Setup signal handling for graceful shutdown
	setupSignalHandling(configManager)

	fmt.Println("Starting configuration interface...")
	if err := configManager.Run(); err != nil {
		log.Printf("ERROR: Configuration manager error: %v", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	log.Println("Configuration tool shutdown complete")
	fmt.Println("Configuration tool exited normally.")
}

func setupSignalHandling(configManager *ui.ConfigManager) {
	// This is a simplified signal handler - in a full implementation
	// you would use os/signal package to handle SIGINT, SIGTERM, etc.
	// For now, we'll rely on the UI's cleanup when the program exits
}