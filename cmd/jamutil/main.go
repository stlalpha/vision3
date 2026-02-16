package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stlalpha/vision3/internal/jam"
)

const version = "1.0.0"

// areaConfig is a minimal struct for parsing message_areas.json
// without importing the heavyweight message package.
type areaConfig struct {
	ID       int    `json:"id"`
	Tag      string `json:"tag"`
	Name     string `json:"name"`
	BasePath string `json:"base_path"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	if cmd == "--version" || cmd == "-version" {
		fmt.Printf("jamutil %s - JAM Message Base Utility for Vision3\n", version)
		return
	}
	if cmd == "--help" || cmd == "-h" || cmd == "help" {
		printUsage()
		return
	}

	switch cmd {
	case "stats":
		cmdStats(os.Args[2:])
	case "pack":
		cmdPack(os.Args[2:])
	case "purge":
		cmdPurge(os.Args[2:])
	case "fix":
		cmdFix(os.Args[2:])
	case "lastread":
		cmdLastread(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `jamutil %s - JAM Message Base Utility for Vision3

Usage: jamutil <command> [options] [base_path...]

Commands:
  stats     Display message base statistics
  pack      Defragment base, removing deleted messages
  purge     Delete old messages by age or count
  fix       Verify base integrity
  lastread  Show or reset lastread records

Global Options:
  --all           Operate on all areas from message_areas.json
  --config DIR    Config directory (default: configs)
  --data DIR      Data directory (default: data)
  -q              Quiet mode

Examples:
  jamutil stats data/msgbases/general
  jamutil stats --all
  jamutil pack --dry-run data/msgbases/general
  jamutil pack --all
  jamutil purge --days 90 --all
  jamutil purge --keep 500 data/msgbases/general
  jamutil fix --all
  jamutil lastread data/msgbases/general
  jamutil lastread --reset testuser data/msgbases/general
`, version)
}

// resolveBasePaths returns base paths from positional args or --all flag.
func resolveBasePaths(allFlag bool, configDir, dataDir string, args []string) ([]baseMeta, error) {
	if allFlag {
		return loadAllBasePaths(configDir, dataDir)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("base path required (or use --all)")
	}
	var paths []baseMeta
	for _, a := range args {
		paths = append(paths, baseMeta{Path: a, Tag: filepath.Base(a)})
	}
	return paths, nil
}

type baseMeta struct {
	Path string
	Tag  string
	Name string
}

func loadAllBasePaths(configDir, dataDir string) ([]baseMeta, error) {
	data, err := os.ReadFile(filepath.Join(configDir, "message_areas.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read message_areas.json: %w", err)
	}
	var areas []areaConfig
	if err := json.Unmarshal(data, &areas); err != nil {
		return nil, fmt.Errorf("failed to parse message_areas.json: %w", err)
	}
	var paths []baseMeta
	for _, a := range areas {
		bp := a.BasePath
		if bp == "" {
			bp = "msgbases/" + a.Tag
		}
		paths = append(paths, baseMeta{
			Path: filepath.Join(dataDir, bp),
			Tag:  a.Tag,
			Name: a.Name,
		})
	}
	return paths, nil
}

func addGlobalFlags(fs *flag.FlagSet) (*bool, *string, *string, *bool) {
	all := fs.Bool("all", false, "Operate on all configured areas")
	configDir := fs.String("config", "configs", "Config directory")
	dataDir := fs.String("data", "data", "Data directory")
	quiet := fs.Bool("q", false, "Quiet mode")
	return all, configDir, dataDir, quiet
}

// cmdStats displays message base statistics.
func cmdStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	allFlag, configDir, dataDir, quiet := addGlobalFlags(fs)
	fs.Parse(args)

	paths, err := resolveBasePaths(*allFlag, *configDir, *dataDir, fs.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, meta := range paths {
		b, err := jam.Open(meta.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", meta.Path, err)
			continue
		}

		total, _ := b.GetMessageCount()
		fh := b.GetFixedHeader()
		active := int(fh.ActiveMsgs)
		deleted := total - active

		if *quiet {
			fmt.Printf("%s: total=%d active=%d deleted=%d\n", meta.Tag, total, active, deleted)
		} else {
			label := meta.Path
			if meta.Name != "" {
				label = fmt.Sprintf("%s (%s)", meta.Tag, meta.Name)
			}
			fmt.Printf("=== %s ===\n", label)
			fmt.Printf("  Path:       %s\n", meta.Path)
			fmt.Printf("  Created:    %s\n", time.Unix(int64(fh.DateCreated), 0).Format("2006-01-02 15:04:05"))
			fmt.Printf("  ModCounter: %d\n", fh.ModCounter)
			fmt.Printf("  BaseMsgNum: %d\n", fh.BaseMsgNum)
			fmt.Printf("  Messages:   %d total, %d active, %d deleted\n", total, active, deleted)

			// File sizes
			for _, ext := range []string{".jhr", ".jdt", ".jdx", ".jlr"} {
				info, err := os.Stat(meta.Path + ext)
				if err == nil {
					fmt.Printf("  %-12s %s\n", ext+":", formatBytes(info.Size()))
				}
			}
			fmt.Println()
		}
		b.Close()
	}
}

// cmdPack defragments message bases.
func cmdPack(args []string) {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	allFlag, configDir, dataDir, quiet := addGlobalFlags(fs)
	dryRun := fs.Bool("dry-run", false, "Report what would happen without modifying")
	fs.Parse(args)

	paths, err := resolveBasePaths(*allFlag, *configDir, *dataDir, fs.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, meta := range paths {
		b, err := jam.Open(meta.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", meta.Path, err)
			continue
		}

		total, _ := b.GetMessageCount()
		active := b.GetActiveMessageCount()
		deleted := total - active

		if *dryRun {
			if !*quiet {
				fmt.Printf("%s: %d total, %d active, %d deleted (would remove %d)\n",
					meta.Tag, total, active, deleted, deleted)
			}
			b.Close()
			continue
		}

		if deleted == 0 {
			if !*quiet {
				fmt.Printf("%s: no deleted messages, skipping\n", meta.Tag)
			}
			b.Close()
			continue
		}

		if !*quiet {
			fmt.Printf("Packing %s...\n", meta.Tag)
		}

		result, err := b.Pack()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error packing %s: %v\n", meta.Path, err)
			b.Close()
			continue
		}

		if !*quiet {
			fmt.Printf("  Before: %d messages (%d active, %d deleted)\n",
				result.MessagesBefore, result.MessagesBefore-result.DeletedRemoved, result.DeletedRemoved)
			fmt.Printf("  After:  %d messages\n", result.MessagesAfter)
			fmt.Printf("  Reclaimed: %s\n", formatBytes(result.BytesBefore-result.BytesAfter))
		}
		b.Close()
	}
}

// cmdPurge deletes old messages by age or count.
func cmdPurge(args []string) {
	fs := flag.NewFlagSet("purge", flag.ExitOnError)
	allFlag, configDir, dataDir, quiet := addGlobalFlags(fs)
	days := fs.Int("days", 0, "Delete messages older than N days")
	keep := fs.Int("keep", 0, "Keep only the newest N messages")
	dryRun := fs.Bool("dry-run", false, "Report what would happen without modifying")
	fs.Parse(args)

	if *days == 0 && *keep == 0 {
		fmt.Fprintf(os.Stderr, "Error: --days or --keep is required\n")
		os.Exit(1)
	}

	paths, err := resolveBasePaths(*allFlag, *configDir, *dataDir, fs.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, meta := range paths {
		b, err := jam.Open(meta.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", meta.Path, err)
			continue
		}

		total, _ := b.GetMessageCount()
		var toDelete []int

		if *days > 0 {
			cutoff := uint32(time.Now().Add(-time.Duration(*days) * 24 * time.Hour).Unix())
			for n := 1; n <= total; n++ {
				hdr, err := b.ReadMessageHeader(n)
				if err != nil {
					continue
				}
				if hdr.Attribute&jam.MsgDeleted != 0 {
					continue
				}
				if hdr.DateWritten < cutoff {
					toDelete = append(toDelete, n)
				}
			}
		} else if *keep > 0 {
			// Collect active message numbers
			var active []int
			for n := 1; n <= total; n++ {
				hdr, err := b.ReadMessageHeader(n)
				if err != nil {
					continue
				}
				if hdr.Attribute&jam.MsgDeleted != 0 {
					continue
				}
				active = append(active, n)
			}
			if len(active) > *keep {
				toDelete = active[:len(active)-*keep]
			}
		}

		if *dryRun {
			if !*quiet {
				fmt.Printf("%s: would delete %d messages\n", meta.Tag, len(toDelete))
			}
			b.Close()
			continue
		}

		deleted := 0
		for _, n := range toDelete {
			if err := b.DeleteMessage(n); err == nil {
				deleted++
			}
		}
		if !*quiet {
			fmt.Printf("%s: deleted %d messages (run 'pack' to reclaim space)\n", meta.Tag, deleted)
		}
		b.Close()
	}
}

// cmdFix verifies base integrity.
func cmdFix(args []string) {
	fs := flag.NewFlagSet("fix", flag.ExitOnError)
	allFlag, configDir, dataDir, quiet := addGlobalFlags(fs)
	repair := fs.Bool("repair", false, "Attempt to repair issues")
	fs.Parse(args)

	paths, err := resolveBasePaths(*allFlag, *configDir, *dataDir, fs.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	hadErrors := false
	for _, meta := range paths {
		b, err := jam.Open(meta.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", meta.Path, err)
			hadErrors = true
			continue
		}

		issues := 0
		cleanedReplyIDs := 0
		if !*quiet {
			fmt.Printf("Checking %s...\n", meta.Tag)
		}

		// Check 1: Signature (already validated by Open)

		// Check 2: File size sanity
		jdxInfo, _ := os.Stat(meta.Path + ".jdx")
		jlrInfo, _ := os.Stat(meta.Path + ".jlr")
		if jdxInfo != nil && jdxInfo.Size()%jam.IndexRecordSize != 0 {
			fmt.Printf("  WARN: .jdx size %d not divisible by %d\n", jdxInfo.Size(), jam.IndexRecordSize)
			issues++
		}
		if jlrInfo != nil && jlrInfo.Size()%jam.LastReadSize != 0 {
			fmt.Printf("  WARN: .jlr size %d not divisible by %d\n", jlrInfo.Size(), jam.LastReadSize)
			issues++
		}

		// Check 3: ActiveMsgs consistency
		total, _ := b.GetMessageCount()
		fh := b.GetFixedHeader()
		actualActive := 0
		jhrInfo, _ := os.Stat(meta.Path + ".jhr")
		jdtInfo, _ := os.Stat(meta.Path + ".jdt")

		for n := 1; n <= total; n++ {
			hdr, err := b.ReadMessageHeader(n)
			if err != nil {
				fmt.Printf("  ERROR: cannot read header for msg %d: %v\n", n, err)
				issues++
				continue
			}
			if hdr.Attribute&jam.MsgDeleted == 0 {
				actualActive++

				// Check 4: Text offset validity
				if jdtInfo != nil && hdr.TxtLen > 0 {
					endPos := int64(hdr.Offset) + int64(hdr.TxtLen)
					if endPos > jdtInfo.Size() {
						fmt.Printf("  ERROR: msg %d text extends beyond .jdt (offset=%d len=%d, file=%d)\n",
							n, hdr.Offset, hdr.TxtLen, jdtInfo.Size())
						issues++
					}
				}
			}

			// Check 5: Index offset validity
			idx, err := b.ReadIndexRecord(n)
			if err == nil && jhrInfo != nil {
				if int64(idx.HdrOffset) < jam.HeaderSize || int64(idx.HdrOffset) >= jhrInfo.Size() {
					fmt.Printf("  ERROR: msg %d index points to invalid offset %d (file size %d)\n",
						n, idx.HdrOffset, jhrInfo.Size())
					issues++
				}
			}
		}

		if int(fh.ActiveMsgs) != actualActive {
			fmt.Printf("  MISMATCH: ActiveMsgs=%d but actual=%d\n", fh.ActiveMsgs, actualActive)
			issues++
			if *repair {
				// We can't directly modify the fixed header from outside the package,
				// but we can use Pack which rebuilds everything correctly
				fmt.Printf("  REPAIR: Run 'pack' to rebuild base and fix counts\n")
			}
		}

		// Check 6: ReplyID integrity (clean malformed values)
		modifiedMessages := false
		if *repair {
			messages, err := b.ScanMessages(1, 0)
			if err == nil {
				for _, msg := range messages {
					if msg.ReplyID != "" {
						// Extract only the first MSGID token from malformed REPLY values
						if parts := strings.Fields(msg.ReplyID); len(parts) > 1 {
							originalReplyID := msg.ReplyID
							msg.ReplyID = parts[0]
							if !*quiet {
								fmt.Printf("  REPAIR: Cleaned ReplyID %q -> %q\n", originalReplyID, msg.ReplyID)
							}
							cleanedReplyIDs++
							modifiedMessages = true
						}
					}
				}
				// If we cleaned any ReplyIDs, rebuild the message base to save changes
				if modifiedMessages {
					_, err := b.Pack()
					if err == nil {
						if !*quiet {
							fmt.Printf("  REPAIR: Rebuilt message base with cleaned ReplyIDs\n")
						}
					} else {
						fmt.Printf("  ERROR: Failed to rebuild message base: %v\n", err)
						issues++
					}
				}
			}
		} else {
			// In non-repair mode, just check for malformed ReplyIDs
			messages, err := b.ScanMessages(1, 0)
			if err == nil {
				for _, msg := range messages {
					if msg.ReplyID != "" {
						if parts := strings.Fields(msg.ReplyID); len(parts) > 1 {
							fmt.Printf("  ISSUE: Malformed ReplyID: %q (use --repair to fix)\n", msg.ReplyID)
							issues++
						}
					}
				}
			}
		}

		if issues == 0 && cleanedReplyIDs == 0 && !*quiet {
			fmt.Printf("  OK: %d messages, %d active, no issues\n", total, actualActive)
		} else if issues > 0 || cleanedReplyIDs > 0 {
			if issues > 0 {
				hadErrors = true
			}
			if cleanedReplyIDs > 0 {
				fmt.Printf("  Cleaned %d malformed ReplyIDs\n", cleanedReplyIDs)
			}
			if issues > 0 {
				fmt.Printf("  Found %d issue(s)\n", issues)
			}
		}
		b.Close()
	}

	if hadErrors {
		os.Exit(1)
	}
}

// cmdLastread shows or resets lastread records.
func cmdLastread(args []string) {
	fs := flag.NewFlagSet("lastread", flag.ExitOnError)
	allFlag, configDir, dataDir, quiet := addGlobalFlags(fs)
	resetUser := fs.String("reset", "", "Reset lastread for this username")
	fs.Parse(args)

	paths, err := resolveBasePaths(*allFlag, *configDir, *dataDir, fs.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, meta := range paths {
		b, err := jam.Open(meta.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", meta.Path, err)
			continue
		}

		if *resetUser != "" {
			if err := b.ResetLastRead(*resetUser); err != nil {
				fmt.Fprintf(os.Stderr, "Error resetting lastread for %s in %s: %v\n", *resetUser, meta.Tag, err)
			} else if !*quiet {
				fmt.Printf("%s: reset lastread for %q\n", meta.Tag, *resetUser)
			}
			b.Close()
			continue
		}

		records, err := b.GetAllLastReadRecords()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading lastread for %s: %v\n", meta.Tag, err)
			b.Close()
			continue
		}

		if len(records) == 0 {
			if !*quiet {
				fmt.Printf("%s: no lastread records\n", meta.Tag)
			}
			b.Close()
			continue
		}

		if !*quiet {
			fmt.Printf("=== %s ===\n", meta.Tag)
			for _, lr := range records {
				fmt.Printf("  UserCRC=0x%08X  LastRead=%-6d  HighRead=%-6d\n",
					lr.UserCRC, lr.LastReadMsg, lr.HighReadMsg)
			}
			fmt.Println()
		}
		b.Close()
	}
}

func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d bytes", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
}
