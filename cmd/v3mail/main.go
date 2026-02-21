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
	"github.com/stlalpha/vision3/internal/version"
)

// areaConfig is a minimal struct for parsing message_areas.json
// without importing the heavyweight message package.
type areaConfig struct {
	ID        int    `json:"id"`
	Tag       string `json:"tag"`
	Name      string `json:"name"`
	BasePath  string `json:"base_path"`
	MaxMsgs   int    `json:"max_messages"` // 0 = no limit
	MaxMsgAge int    `json:"max_age"`      // days, 0 = no limit
}

func main() {
	if len(os.Args) < 2 {
		printUsage("")
		os.Exit(1)
	}

	cmd := os.Args[1]
	if cmd == "--version" || cmd == "-version" {
		printHeader()
		return
	}
	if cmd == "--help" || cmd == "-h" || cmd == "help" {
		printUsage("")
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
	case "link":
		cmdLink(os.Args[2:])
	case "lastread":
		cmdLastread(os.Args[2:])
	case "toss":
		cmdToss(os.Args[2:])
	case "scan":
		cmdScan(os.Args[2:])
	case "ftn-pack":
		cmdFtnPack(os.Args[2:])
	default:
		printUsage(fmt.Sprintf("Unknown command: %s", cmd))
		os.Exit(1)
	}
}

const (
	clrReset   = "\033[0m"
	clrCyan    = "\033[36m"
	clrMagenta = "\033[35m"
	clrBold    = "\033[1m"
	separator  = "────────────────────────────────────────────────────────────────────────────"
)

func printHeader() {
	fmt.Fprintf(os.Stderr, "%sViSiON/3 Mail Utility v%s  ·  MIT License%s\n",
		clrBold, version.Number, clrReset)
	fmt.Fprintln(os.Stderr, separator)
}

func bullet(msg string) string {
	return fmt.Sprintf("%s\u25a0%s  %s%s%s", clrMagenta, clrReset, clrCyan, msg, clrReset)
}

func cmd(name, desc string) string {
	return fmt.Sprintf("  %s%-10s%s - %s%s%s",
		clrCyan, name, clrReset, clrCyan, desc, clrReset)
}

func opt(flag, desc string) string {
	return fmt.Sprintf("  %s%-18s%s %s%s%s",
		clrCyan, flag, clrReset, clrCyan, desc, clrReset)
}

func printUsage(errMsg string) {
	w := os.Stderr
	printHeader()
	fmt.Fprintln(w)
	if errMsg != "" {
		fmt.Fprintln(w, bullet(errMsg))
	}
	fmt.Fprintln(w, bullet("v3mail needs a little more information!"))
	fmt.Fprintln(w, bullet("Required Format: v3mail <command> [options] [base_path...]"))
	fmt.Fprintln(w, bullet("Valid Commands Are As Follows..."))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sJAM Base Commands:%s\n", clrBold, clrReset)
	fmt.Fprintln(w, cmd("STATS", "Display message base statistics"))
	fmt.Fprintln(w, cmd("PACK", "Defragment base, removing deleted messages"))
	fmt.Fprintln(w, cmd("PURGE", "Delete messages exceeding age or count limits"))
	fmt.Fprintln(w, cmd("FIX", "Verify and repair JAM base integrity"))
	fmt.Fprintln(w, cmd("LINK", "Build reply-threading chains (ReplyTo/Reply1st/ReplyNext)"))
	fmt.Fprintln(w, cmd("LASTREAD", "Show or reset per-user lastread pointers"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sFTN Echomail Commands:%s\n", clrBold, clrReset)
	fmt.Fprintln(w, cmd("TOSS", "Unpack inbound FTN bundles and toss .PKT files into JAM bases"))
	fmt.Fprintln(w, cmd("SCAN", "Scan JAM bases for unsent echomail; create outbound .PKT files"))
	fmt.Fprintln(w, cmd("FTN-PACK", "Pack outbound .PKT files into ZIP bundles for binkd"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sGlobal Options:%s\n", clrBold, clrReset)
	fmt.Fprintln(w, opt("--all", "Operate on all areas in message_areas.json"))
	fmt.Fprintln(w, opt("--config DIR", "Config directory (default: configs)"))
	fmt.Fprintln(w, opt("--data DIR", "Data directory (default: data)"))
	fmt.Fprintln(w, opt("-q", "Suppress output"))
	fmt.Fprintln(w, opt("--network NAME", "FTN: limit to a single network"))
	fmt.Fprintln(w)
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
	Path    string
	Tag     string
	Name    string
	MaxMsgs int // 0 = no limit
	MaxAge  int // days, 0 = no limit
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
			Path:    filepath.Join(dataDir, bp),
			Tag:     a.Tag,
			Name:    a.Name,
			MaxMsgs: a.MaxMsgs,
			MaxAge:  a.MaxMsgAge,
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
// When --all is used, per-area max_msg_age and max_msgs from message_areas.json
// take precedence; --days and --keep serve as fallback defaults for areas
// without per-area limits configured.
func cmdPurge(args []string) {
	fs := flag.NewFlagSet("purge", flag.ExitOnError)
	allFlag, configDir, dataDir, quiet := addGlobalFlags(fs)
	days := fs.Int("days", 0, "Delete messages older than N days (fallback when --all is used)")
	keep := fs.Int("keep", 0, "Keep only the newest N messages (fallback when --all is used)")
	dryRun := fs.Bool("dry-run", false, "Report what would happen without modifying")
	fs.Parse(args)

	// --days or --keep are required for manual (non-all) invocations.
	if !*allFlag && *days == 0 && *keep == 0 {
		fmt.Fprintf(os.Stderr, "Error: --days or --keep is required\n")
		os.Exit(1)
	}

	paths, err := resolveBasePaths(*allFlag, *configDir, *dataDir, fs.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, meta := range paths {
		// Resolve effective limits: per-area config takes precedence over CLI flags.
		effectiveDays := *days
		effectiveKeep := *keep
		if meta.MaxAge > 0 {
			effectiveDays = meta.MaxAge
		}
		if meta.MaxMsgs > 0 {
			effectiveKeep = meta.MaxMsgs
		}

		if effectiveDays == 0 && effectiveKeep == 0 {
			if !*quiet && *allFlag {
				fmt.Printf("%s: no purge limits configured, skipping\n", meta.Tag)
			}
			continue
		}

		b, err := jam.Open(meta.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", meta.Path, err)
			continue
		}

		total, _ := b.GetMessageCount()

		// Collect all active message numbers in order (oldest first).
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

		toDeleteSet := make(map[int]bool)

		// Age pass: mark messages older than effectiveDays for deletion.
		if effectiveDays > 0 {
			cutoff := uint32(time.Now().Add(-time.Duration(effectiveDays) * 24 * time.Hour).Unix())
			for _, n := range active {
				hdr, err := b.ReadMessageHeader(n)
				if err != nil {
					continue
				}
				if hdr.DateWritten < cutoff {
					toDeleteSet[n] = true
				}
			}
		}

		// Count pass: after age purge, if still over effectiveKeep, remove oldest.
		if effectiveKeep > 0 {
			remaining := make([]int, 0, len(active))
			for _, n := range active {
				if !toDeleteSet[n] {
					remaining = append(remaining, n)
				}
			}
			if len(remaining) > effectiveKeep {
				for _, n := range remaining[:len(remaining)-effectiveKeep] {
					toDeleteSet[n] = true
				}
			}
		}

		toDelete := make([]int, 0, len(toDeleteSet))
		for n := range toDeleteSet {
			toDelete = append(toDelete, n)
		}

		if *dryRun {
			if !*quiet {
				fmt.Printf("%s: would delete %d messages (age>%dd, keep<=%d)\n",
					meta.Tag, len(toDelete), effectiveDays, effectiveKeep)
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
		if *repair {
			// Use specialized pack operation that cleans ReplyIDs during rebuild
			cleanedReplyIDs = cleanReplyIDsInBase(b, *quiet)
			if cleanedReplyIDs > 0 && !*quiet {
				fmt.Printf("  REPAIR: Rebuilt message base with cleaned ReplyIDs\n")
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

// cmdLink builds reply threading chains by matching MSGID/ReplyID subfields
// and updating the ReplyTo, Reply1st, and ReplyNext header fields in-place.
func cmdLink(args []string) {
	fs := flag.NewFlagSet("link", flag.ExitOnError)
	allFlag, configDir, dataDir, quiet := addGlobalFlags(fs)
	fs.Parse(args)

	paths, err := resolveBasePaths(*allFlag, *configDir, *dataDir, fs.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	totalUpdated := 0
	for _, meta := range paths {
		b, err := jam.Open(meta.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", meta.Path, err)
			continue
		}

		updated, linkErr := linkBase(b, *quiet, meta.Tag)
		b.Close()
		if linkErr != nil {
			fmt.Fprintf(os.Stderr, "Error linking %s: %v\n", meta.Path, linkErr)
			continue
		}
		totalUpdated += updated
	}

	if !*quiet && len(paths) > 1 {
		fmt.Printf("\nTotal: %d links updated across %d areas\n", totalUpdated, len(paths))
	}
}

// linkBase builds reply chains for a single JAM base by matching MSGID ↔ ReplyID
// and writing ReplyTo/Reply1st/ReplyNext in-place via UpdateMessageHeader.
func linkBase(b *jam.Base, quiet bool, tag string) (int, error) {
	total, err := b.GetMessageCount()
	if err != nil {
		return 0, err
	}
	if total == 0 {
		if !quiet {
			fmt.Printf("%s: no messages\n", tag)
		}
		return 0, nil
	}

	// Phase 1: Scan all headers and build MSGID → msgNum / ReplyID → []msgNum maps.
	type hdrInfo struct {
		hdr     *jam.MessageHeader
		msgNum  int
		msgID   string
		replyID string
	}

	var headers []hdrInfo
	msgIDToNum := make(map[string]int)      // MSGID string → 1-based message number
	replyIDToNums := make(map[string][]int) // ReplyID string → list of replying message numbers

	for n := 1; n <= total; n++ {
		hdr, readErr := b.ReadMessageHeader(n)
		if readErr != nil {
			continue
		}
		if hdr.Attribute&jam.MsgDeleted != 0 {
			continue
		}

		var msgID, replyID string
		for _, sf := range hdr.Subfields {
			switch sf.LoID {
			case jam.SfldMsgID:
				msgID = string(sf.Buffer)
			case jam.SfldReplyID:
				replyID = string(sf.Buffer)
			}
		}

		headers = append(headers, hdrInfo{hdr: hdr, msgNum: n, msgID: msgID, replyID: replyID})
		if msgID != "" {
			msgIDToNum[msgID] = n
			// FTN MSGIDs are "address serial" — HPT often stores REPLY
			// kludges without the serial suffix.  Index the address part
			// too so prefix-based lookups succeed.
			if idx := strings.LastIndex(msgID, " "); idx > 0 {
				prefix := msgID[:idx]
				if _, exists := msgIDToNum[prefix]; !exists {
					msgIDToNum[prefix] = n
				}
			}
		}
		if replyID != "" {
			replyIDToNums[replyID] = append(replyIDToNums[replyID], n)
		}
	}

	if len(headers) == 0 {
		if !quiet {
			fmt.Printf("%s: no active messages\n", tag)
		}
		return 0, nil
	}

	// Phase 2: Compute desired threading fields.
	updated := 0

	for i := range headers {
		h := &headers[i]
		changed := false

		// ReplyTo: if this message has a ReplyID, find the parent's message number.
		if h.replyID != "" {
			if parentNum, ok := msgIDToNum[h.replyID]; ok {
				if h.hdr.ReplyTo != uint32(parentNum) {
					h.hdr.ReplyTo = uint32(parentNum)
					changed = true
				}
			}
		}

		// Reply1st: if this message has a MSGID with replies, point to the first reply.
		// Check both the full MSGID and the address-only prefix (without serial)
		// since HPT may store REPLY kludges without the serial suffix.
		if h.msgID != "" {
			replies := replyIDToNums[h.msgID]
			if len(replies) == 0 {
				if idx := strings.LastIndex(h.msgID, " "); idx > 0 {
					replies = replyIDToNums[h.msgID[:idx]]
				}
			}
			if len(replies) > 0 {
				firstReply := replies[0] // replies are in scan order (ascending)
				if h.hdr.Reply1st != uint32(firstReply) {
					h.hdr.Reply1st = uint32(firstReply)
					changed = true
				}
			} else if h.hdr.Reply1st != 0 {
				// No replies exist (anymore) — clear stale pointer
				h.hdr.Reply1st = 0
				changed = true
			}
		}

		// ReplyNext: chain sibling replies to the same parent.
		if h.replyID != "" {
			if siblings, ok := replyIDToNums[h.replyID]; ok && len(siblings) > 1 {
				// Find our position and point to the next sibling.
				nextSibling := uint32(0)
				for j, sn := range siblings {
					if sn == h.msgNum && j+1 < len(siblings) {
						nextSibling = uint32(siblings[j+1])
						break
					}
				}
				if h.hdr.ReplyNext != nextSibling {
					h.hdr.ReplyNext = nextSibling
					changed = true
				}
			} else if h.hdr.ReplyNext != 0 {
				h.hdr.ReplyNext = 0
				changed = true
			}
		}

		if changed {
			if err := b.UpdateMessageHeader(h.msgNum, h.hdr); err != nil {
				return updated, fmt.Errorf("updating message %d: %w", h.msgNum, err)
			}
			updated++
		}
	}

	if !quiet {
		if updated > 0 {
			fmt.Printf("%s: %d messages, %d links updated\n", tag, len(headers), updated)
		} else {
			fmt.Printf("%s: %d messages, all links current\n", tag, len(headers))
		}
	}

	return updated, nil
}

// cleanReplyIDsInBase performs a pack operation that cleans malformed ReplyIDs during rebuild.
func cleanReplyIDsInBase(b *jam.Base, quiet bool) int {
	cleanedCount := 0

	// Count messages that need cleaning first
	messages, err := b.ScanMessages(1, 0)
	if err != nil {
		return 0
	}

	type repairEntry struct{ orig, fixed string }
	var repairs []repairEntry
	for _, msg := range messages {
		if msg.ReplyID != "" {
			if parts := strings.Fields(msg.ReplyID); len(parts) > 1 {
				repairs = append(repairs, repairEntry{orig: msg.ReplyID, fixed: parts[0]})
			}
		}
	}

	if len(repairs) > 0 {
		// Attempt the pack before printing repair messages so we only report
		// success when the rebuild actually succeeds.
		if err := cleanReplyIDsPack(b); err != nil {
			if !quiet {
				fmt.Printf("  ERROR: Failed to rebuild message base: %v\n", err)
			}
			return 0
		}
		// Pack succeeded — now report what was cleaned.
		cleanedCount = len(repairs)
		if !quiet {
			for _, r := range repairs {
				fmt.Printf("  REPAIR: Cleaned ReplyID %q -> %q\n", r.orig, r.fixed)
			}
		}
	}

	return cleanedCount
}

// cleanReplyIDsPack performs a pack operation while cleaning ReplyIDs.
func cleanReplyIDsPack(b *jam.Base) error {
	// Use the new PackWithReplyIDCleanup function
	_, err := b.PackWithReplyIDCleanup()
	return err
}
