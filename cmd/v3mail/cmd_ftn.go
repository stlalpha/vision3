package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/message"
	"github.com/stlalpha/vision3/internal/tosser"
)

// cmdToss implements 'v3mail toss': unpack FTN bundles and toss .PKT files into JAM bases.
func cmdToss(args []string) {
	fs := flag.NewFlagSet("toss", flag.ExitOnError)
	configDir := fs.String("config", "configs", "Config directory")
	dataDir := fs.String("data", "data", "Data directory")
	networkName := fs.String("network", "", "Limit to a single network (default: all enabled)")
	quiet := fs.Bool("q", false, "Quiet mode")
	fs.Parse(args)

	ftnCfg, msgMgr, dupeDB, err := loadFTNDeps(*configDir, *dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	totalImported, totalDupes, totalPackets := 0, 0, 0
	hadErrors := false

	for name, netCfg := range ftnCfg.Networks {
		if !netCfg.InternalTosserEnabled {
			continue
		}
		if *networkName != "" && name != *networkName {
			continue
		}

		t, err := tosser.New(name, netCfg, ftnCfg, dupeDB, msgMgr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating tosser for %s: %v\n", name, err)
			hadErrors = true
			continue
		}

		result := t.ProcessInbound()
		totalPackets += result.PacketsProcessed
		totalImported += result.MessagesImported
		totalDupes += result.DupesSkipped

		if !*quiet {
			fmt.Printf("[%s] toss: %d packets, %d imported, %d dupes",
				name, result.PacketsProcessed, result.MessagesImported, result.DupesSkipped)
			if len(result.Errors) > 0 {
				fmt.Printf(", %d errors", len(result.Errors))
			}
			fmt.Println()
		}
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  [%s] ERROR: %s\n", name, e)
			hadErrors = true
		}
	}

	if !*quiet {
		fmt.Printf("Toss complete: %d packets, %d messages imported, %d dupes skipped\n",
			totalPackets, totalImported, totalDupes)
	}

	if hadErrors {
		os.Exit(1)
	}
}

// cmdScan implements 'v3mail scan': scan JAM bases for unsent echomail and create outbound .PKT files.
func cmdScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	configDir := fs.String("config", "configs", "Config directory")
	dataDir := fs.String("data", "data", "Data directory")
	networkName := fs.String("network", "", "Limit to a single network (default: all enabled)")
	quiet := fs.Bool("q", false, "Quiet mode")
	fs.Parse(args)

	ftnCfg, msgMgr, dupeDB, err := loadFTNDeps(*configDir, *dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	totalExported := 0
	hadErrors := false

	for name, netCfg := range ftnCfg.Networks {
		if !netCfg.InternalTosserEnabled {
			continue
		}
		if *networkName != "" && name != *networkName {
			continue
		}

		t, err := tosser.New(name, netCfg, ftnCfg, dupeDB, msgMgr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating tosser for %s: %v\n", name, err)
			hadErrors = true
			continue
		}

		result := t.ScanAndExport()
		totalExported += result.MessagesExported

		if !*quiet {
			fmt.Printf("[%s] scan: %d messages exported",
				name, result.MessagesExported)
			if len(result.Errors) > 0 {
				fmt.Printf(", %d errors", len(result.Errors))
			}
			fmt.Println()
		}
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  [%s] ERROR: %s\n", name, e)
			hadErrors = true
		}
	}

	if !*quiet {
		fmt.Printf("Scan complete: %d messages exported to outbound\n", totalExported)
	}

	if hadErrors {
		os.Exit(1)
	}
}

// cmdFtnPack implements 'v3mail ftn-pack': create ZIP bundles from staged .PKT files for binkd.
func cmdFtnPack(args []string) {
	fs := flag.NewFlagSet("ftn-pack", flag.ExitOnError)
	configDir := fs.String("config", "configs", "Config directory")
	dataDir := fs.String("data", "data", "Data directory")
	networkName := fs.String("network", "", "Limit to a single network (default: all enabled)")
	quiet := fs.Bool("q", false, "Quiet mode")
	fs.Parse(args)

	ftnCfg, msgMgr, dupeDB, err := loadFTNDeps(*configDir, *dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	totalBundles, totalPackets := 0, 0
	hadErrors := false

	for name, netCfg := range ftnCfg.Networks {
		if !netCfg.InternalTosserEnabled {
			continue
		}
		if *networkName != "" && name != *networkName {
			continue
		}

		t, err := tosser.New(name, netCfg, ftnCfg, dupeDB, msgMgr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating tosser for %s: %v\n", name, err)
			hadErrors = true
			continue
		}

		result := t.PackOutbound()
		totalBundles += result.BundlesCreated
		totalPackets += result.PacketsPacked

		if !*quiet {
			fmt.Printf("[%s] ftn-pack: %d bundles created (%d packets)",
				name, result.BundlesCreated, result.PacketsPacked)
			if len(result.Errors) > 0 {
				fmt.Printf(", %d errors", len(result.Errors))
			}
			fmt.Println()
		}
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  [%s] ERROR: %s\n", name, e)
			hadErrors = true
		}
	}

	if !*quiet {
		fmt.Printf("Pack complete: %d bundles created, %d packets packed\n", totalBundles, totalPackets)
	}

	if hadErrors {
		os.Exit(1)
	}
}

// resolveFTNPath makes path absolute by joining with root if it is not already absolute.
// Root is the BBS root (directory containing the data folder).
func resolveFTNPath(root, path string) string {
	if path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

// loadFTNDeps loads all shared dependencies needed by toss/scan/ftn-pack commands.
// FTN paths in ftn.json are resolved relative to the BBS root (parent of dataDir)
// so toss/scan/pack work correctly regardless of CWD when v3mail is run.
func loadFTNDeps(configDir, dataDir string) (config.FTNConfig, *message.MessageManager, *tosser.DupeDB, error) {
	ftnCfg, err := config.LoadFTNConfig(configDir)
	if err != nil {
		return config.FTNConfig{}, nil, nil, fmt.Errorf("load ftn config: %w", err)
	}

	// BBS root = directory containing the data folder (for resolving relative FTN paths)
	absData, err := filepath.Abs(dataDir)
	if err != nil {
		absData = dataDir
	}
	bbsRoot := filepath.Dir(absData)

	// Resolve relative FTN paths against BBS root
	ftnCfg.InboundPath = resolveFTNPath(bbsRoot, ftnCfg.InboundPath)
	ftnCfg.SecureInboundPath = resolveFTNPath(bbsRoot, ftnCfg.SecureInboundPath)
	ftnCfg.OutboundPath = resolveFTNPath(bbsRoot, ftnCfg.OutboundPath)
	ftnCfg.BinkdOutboundPath = resolveFTNPath(bbsRoot, ftnCfg.BinkdOutboundPath)
	ftnCfg.TempPath = resolveFTNPath(bbsRoot, ftnCfg.TempPath)

	// Build tearlines map for MessageManager
	tearlines := make(map[string]string)
	for name, net := range ftnCfg.Networks {
		tearlines[name] = net.Tearline
	}

	// Load server config for board name
	serverCfg, err := config.LoadServerConfig(configDir)
	boardName := "Vision3 BBS"
	if err == nil {
		boardName = serverCfg.BoardName
	}

	msgMgr, err := message.NewMessageManager(dataDir, configDir, boardName, tearlines)
	if err != nil {
		return config.FTNConfig{}, nil, nil, fmt.Errorf("init message manager: %w", err)
	}

	// Load or create dupe database (resolve path if from ftn.json)
	dupeDBPath := ftnCfg.DupeDBPath
	if dupeDBPath == "" {
		dupeDBPath = filepath.Join(dataDir, "ftn", "dupes.json")
	} else {
		dupeDBPath = resolveFTNPath(bbsRoot, dupeDBPath)
	}
	dupeDB, err := tosser.NewDupeDBFromPath(dupeDBPath)
	if err != nil {
		return config.FTNConfig{}, nil, nil, fmt.Errorf("load dupe db: %w", err)
	}

	return ftnCfg, msgMgr, dupeDB, nil
}
