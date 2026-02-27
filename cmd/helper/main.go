package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/ftn"
	"github.com/stlalpha/vision3/internal/jam"
	"github.com/stlalpha/vision3/internal/message"
	"github.com/stlalpha/vision3/internal/user"
	"github.com/stlalpha/vision3/internal/version"
)

// --- NA file types ---

type naArea struct {
	Tag         string
	Description string
}

// --- Config types (local, minimal) ---

type conferenceConfig struct {
	ID          int    `json:"id"`
	Tag         string `json:"tag"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ACS         string `json:"acs"`
}

// Use canonical config types to avoid struct drift.
type ftnConfig = config.FTNConfig
type ftnNetworkConfig = config.FTNNetworkConfig
type linkConfig = config.FTNLinkConfig

const (
	clrReset   = "\033[0m"
	clrCyan    = "\033[36m"
	clrMagenta = "\033[35m"
	clrBold    = "\033[1m"
	separator  = "────────────────────────────────────────────────────────────────────────────"
)

func printHeader() {
	fmt.Fprintf(os.Stderr, "%sViSiON/3 Helper Utility v%s  ·  MIT License%s\n",
		clrBold, version.Number, clrReset)
	fmt.Fprintln(os.Stderr, separator)
}

func bullet(msg string) string {
	return fmt.Sprintf("%s\u25a0%s  %s%s%s", clrMagenta, clrReset, clrCyan, msg, clrReset)
}

func helpcmd(name, desc string) string {
	return fmt.Sprintf("  %s%-20s%s - %s%s%s",
		clrCyan, name, clrReset, clrCyan, desc, clrReset)
}

func helpopt(flag, desc string) string {
	return fmt.Sprintf("  %s%-18s%s %s%s%s",
		clrCyan, flag, clrReset, clrCyan, desc, clrReset)
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
	case "ftnsetup":
		cmdFTNSetup(os.Args[2:])
	case "areafix", "aerafix":
		cmdAreafix(os.Args[2:])
	case "users":
		cmdUsers(os.Args[2:])
	case "files":
		cmdFiles(os.Args[2:])
	default:
		printUsage(fmt.Sprintf("Unknown command: %s", cmd))
		os.Exit(1)
	}
}

func printUsage(errMsg string) {
	w := os.Stderr
	printHeader()
	fmt.Fprintln(w)
	if errMsg != "" {
		fmt.Fprintln(w, bullet(errMsg))
	}
	fmt.Fprintln(w, bullet("helper needs a little more information!"))
	fmt.Fprintln(w, bullet("Required Format: helper <command> [options]"))
	fmt.Fprintln(w, bullet("Valid Commands Are As Follows..."))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sFTN Commands:%s\n", clrBold, clrReset)
	fmt.Fprintln(w, helpcmd("FTNSETUP", "Import FTN echo areas from a FIDONET.NA file"))
	fmt.Fprintln(w, helpcmd("AREAFIX", "Send an AreaFix netmail command to a network hub"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sUser Commands:%s\n", clrBold, clrReset)
	fmt.Fprintln(w, helpcmd("USERS PURGE", "Permanently remove soft-deleted users past retention"))
	fmt.Fprintln(w, helpcmd("USERS LIST", "List user accounts"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sFile Commands:%s\n", clrBold, clrReset)
	fmt.Fprintln(w, helpcmd("FILES IMPORT", "Bulk import files from a directory into a file area"))
	fmt.Fprintln(w, helpcmd("FILES REEXTRACTDIZ", "Re-extract FILE_ID.DIZ and update descriptions"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sGlobal Options:%s\n", clrBold, clrReset)
	fmt.Fprintln(w, helpopt("--config DIR", "Config directory (default: configs)"))
	fmt.Fprintln(w, helpopt("--data DIR", "Data directory (default: data)"))
	fmt.Fprintln(w)
}

// --- users command group ---

func printUsersHelp(errMsg string) {
	w := os.Stderr
	printHeader()
	fmt.Fprintln(w)
	if errMsg != "" {
		fmt.Fprintln(w, bullet(errMsg))
	}
	fmt.Fprintln(w, bullet("Required Format: helper users <subcommand> [options]"))
	fmt.Fprintln(w, bullet("Valid Subcommands Are As Follows..."))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sUser Subcommands:%s\n", clrBold, clrReset)
	fmt.Fprintln(w, helpcmd("PURGE", "Permanently remove soft-deleted users past retention"))
	fmt.Fprintln(w, helpcmd("LIST", "List user accounts (optionally filtered to deleted)"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sOptions:%s\n", clrBold, clrReset)
	fmt.Fprintln(w, helpopt("--config DIR", "Config directory (default: configs)"))
	fmt.Fprintln(w, helpopt("--data DIR", "Data directory (default: data/users)"))
	fmt.Fprintln(w, helpopt("--days N", "Retention days override (purge)"))
	fmt.Fprintln(w, helpopt("--dry-run", "Show what would happen without making changes"))
	fmt.Fprintln(w, helpopt("--deleted", "Show only soft-deleted accounts (list)"))
	fmt.Fprintln(w)
}

func cmdUsers(args []string) {
	if len(args) < 1 {
		printUsersHelp("")
		os.Exit(1)
	}

	sub := args[0]
	switch sub {
	case "purge":
		cmdUsersPurge(args[1:])
	case "list":
		cmdUsersList(args[1:])
	case "help", "--help", "-h":
		printUsersHelp("")
	default:
		printUsersHelp(fmt.Sprintf("Unknown subcommand: %s", sub))
		os.Exit(1)
	}
}

// --- files command group ---

func printFilesHelp(errMsg string) {
	w := os.Stderr
	printHeader()
	fmt.Fprintln(w)
	if errMsg != "" {
		fmt.Fprintln(w, bullet(errMsg))
	}
	fmt.Fprintln(w, bullet("Required Format: helper files <subcommand> [options]"))
	fmt.Fprintln(w, bullet("Valid Subcommands Are As Follows..."))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sFile Subcommands:%s\n", clrBold, clrReset)
	fmt.Fprintln(w, helpcmd("IMPORT", "Bulk import files from a directory into a file area"))
	fmt.Fprintln(w, helpcmd("REEXTRACTDIZ", "Re-extract FILE_ID.DIZ and update descriptions"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sImport Options:%s\n", clrBold, clrReset)
	fmt.Fprintln(w, helpopt("--dir DIR", "Source directory containing files (required)"))
	fmt.Fprintln(w, helpopt("--area TAG", "Target file area tag (required)"))
	fmt.Fprintln(w, helpopt("--uploader NAME", "Uploader handle (default: Sysop)"))
	fmt.Fprintln(w, helpopt("--move", "Move files instead of copying"))
	fmt.Fprintln(w, helpopt("--preserve-dates", "Use file modification time as upload date"))
	fmt.Fprintln(w, helpopt("--no-diz", "Skip FILE_ID.DIZ extraction from archives"))
	fmt.Fprintln(w, helpopt("--dry-run", "Show what would happen without making changes"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %sGlobal Options:%s\n", clrBold, clrReset)
	fmt.Fprintln(w, helpopt("--config DIR", "Config directory (default: configs)"))
	fmt.Fprintln(w, helpopt("--data DIR", "Data directory (default: data)"))
	fmt.Fprintln(w)
}

func cmdFiles(args []string) {
	if len(args) < 1 {
		printFilesHelp("")
		os.Exit(1)
	}

	sub := args[0]
	switch sub {
	case "import":
		cmdFilesImport(args[1:])
	case "reextractdiz":
		cmdFilesReextractDIZ(args[1:])
	case "help", "--help", "-h":
		printFilesHelp("")
	default:
		printFilesHelp(fmt.Sprintf("Unknown subcommand: %s", sub))
		os.Exit(1)
	}
}

func cmdUsersPurge(args []string) {
	fs := flag.NewFlagSet("users purge", flag.ExitOnError)
	configDir := fs.String("config", "configs", "Config directory")
	dataDir := fs.String("data", "data/users", "User data directory")
	days := fs.Int("days", -1, "Retention days override (default: read from config.json)")
	dryRun := fs.Bool("dry-run", false, "Show what would be purged without making changes")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: helper users purge [options]\n\n")
		fmt.Fprintf(os.Stderr, "Permanently remove soft-deleted user accounts past the retention period.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  helper users purge\n")
		fmt.Fprintf(os.Stderr, "  helper users purge --days 90 --dry-run\n")
	}
	fs.Parse(args)

	// Load config to get retention days if not overridden
	retentionDays := *days
	if retentionDays < 0 {
		cfg, err := config.LoadServerConfig(*configDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
		retentionDays = cfg.DeletedUserRetentionDays
	}

	if retentionDays < 0 {
		fmt.Println("Retention days is -1 (never purge). Nothing to do.")
		return
	}

	// Load user manager
	um, err := user.NewUserManager(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading users: %v\n", err)
		os.Exit(1)
	}

	if *dryRun {
		// Show eligible users without purging
		cutoff := time.Now().AddDate(0, 0, -retentionDays)
		eligible := eligibleForPurge(um.GetAllUsers(), cutoff)
		if len(eligible) == 0 {
			fmt.Printf("Dry run: no users eligible for purge (retention: %d days).\n", retentionDays)
			return
		}
		fmt.Printf("Dry run: %d user(s) would be purged (retention: %d days, cutoff: %s):\n\n",
			len(eligible), retentionDays, cutoff.Format("2006-01-02"))
		printPurgeCandidates(eligible, retentionDays)
		return
	}

	purged, err := um.PurgeDeletedUsers(retentionDays)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error purging users: %v\n", err)
		os.Exit(1)
	}

	if len(purged) == 0 {
		fmt.Printf("No users eligible for purge (retention: %d days).\n", retentionDays)
		return
	}

	fmt.Printf("Purged %d user account(s) (retention: %d days):\n\n", len(purged), retentionDays)
	for _, p := range purged {
		if p.DeletedAt.IsZero() {
			fmt.Printf("  #%-4d  %-20s  %s  (no deletion timestamp)\n", p.ID, p.Username, p.Handle)
		} else {
			fmt.Printf("  #%-4d  %-20s  %-20s  deleted %s\n",
				p.ID, p.Username, p.Handle, p.DeletedAt.Format("2006-01-02"))
		}
	}
}

func cmdUsersList(args []string) {
	fs := flag.NewFlagSet("users list", flag.ExitOnError)
	dataDir := fs.String("data", "data/users", "User data directory")
	configDir := fs.String("config", "configs", "Config directory")
	deletedOnly := fs.Bool("deleted", false, "Show only soft-deleted accounts")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: helper users list [options]\n\n")
		fmt.Fprintf(os.Stderr, "List user accounts.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	um, err := user.NewUserManager(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading users: %v\n", err)
		os.Exit(1)
	}

	// Load retention days for the "days remaining" column
	retentionDays := -1
	if cfg, cfgErr := config.LoadServerConfig(*configDir); cfgErr == nil {
		retentionDays = cfg.DeletedUserRetentionDays
	}

	all := um.GetAllUsers()
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })

	if *deletedOnly {
		var deleted []*user.User
		for _, u := range all {
			if u.DeletedUser {
				deleted = append(deleted, u)
			}
		}
		if len(deleted) == 0 {
			fmt.Println("No soft-deleted users found.")
			return
		}
		fmt.Printf("%-6s  %-20s  %-20s  %-12s  %s\n", "ID", "Username", "Handle", "Deleted On", "Days Until Purge")
		fmt.Println(strings.Repeat("-", 78))
		for _, u := range deleted {
			deletedOn := "(no timestamp)"
			daysLeft := "n/a"
			if u.DeletedAt != nil {
				deletedOn = u.DeletedAt.Format("2006-01-02")
				if retentionDays >= 0 {
					purgeDate := u.DeletedAt.AddDate(0, 0, retentionDays)
					remaining := int(time.Until(purgeDate).Hours() / 24)
					if remaining <= 0 {
						daysLeft = "eligible now"
					} else {
						daysLeft = fmt.Sprintf("%d days", remaining)
					}
				}
			}
			fmt.Printf("%-6d  %-20s  %-20s  %-12s  %s\n", u.ID, u.Username, u.Handle, deletedOn, daysLeft)
		}
		return
	}

	fmt.Printf("%-6s  %-20s  %-20s  %-10s  %s\n", "ID", "Username", "Handle", "Level", "Status")
	fmt.Println(strings.Repeat("-", 72))
	for _, u := range all {
		status := "active"
		if u.DeletedUser {
			status = "DELETED"
			if u.DeletedAt != nil {
				status = fmt.Sprintf("DELETED %s", u.DeletedAt.Format("2006-01-02"))
			}
		} else if !u.Validated {
			status = "unvalidated"
		}
		fmt.Printf("%-6d  %-20s  %-20s  %-10d  %s\n", u.ID, u.Username, u.Handle, u.AccessLevel, status)
	}
	fmt.Printf("\nTotal: %d user(s)\n", len(all))
}

// eligibleForPurge returns users that are soft-deleted and past the cutoff time.
func eligibleForPurge(users []*user.User, cutoff time.Time) []*user.User {
	var result []*user.User
	for _, u := range users {
		if !u.DeletedUser {
			continue
		}
		if u.DeletedAt != nil && u.DeletedAt.Before(cutoff) {
			result = append(result, u)
		}
	}
	return result
}

// printPurgeCandidates prints a table of users eligible for purge.
func printPurgeCandidates(users []*user.User, retentionDays int) {
	fmt.Printf("  %-6s  %-20s  %-20s  %s\n", "ID", "Username", "Handle", "Deleted On")
	fmt.Println("  " + strings.Repeat("-", 64))
	for _, u := range users {
		deletedOn := "(no timestamp)"
		if u.DeletedAt != nil {
			deletedOn = u.DeletedAt.Format("2006-01-02")
		}
		fmt.Printf("  %-6d  %-20s  %-20s  %s\n", u.ID, u.Username, u.Handle, deletedOn)
	}
}

// --- ftnsetup command ---

func cmdFTNSetup(args []string) {
	fs := flag.NewFlagSet("ftnsetup", flag.ExitOnError)
	naFile := fs.String("na", "", "Path to FIDONET.NA file (required)")
	address := fs.String("address", "", "Our FTN address, e.g. 21:3/110 (required)")
	hub := fs.String("hub", "", "Hub/uplink FTN address, e.g. 21:1/100 (required)")
	hubPassword := fs.String("hub-password", "", "Packet password for hub")
	hubName := fs.String("hub-name", "", "Human-readable hub name (default: derived from address)")
	network := fs.String("network", "", "Network name for conference (default: derived from NA filename)")
	conferenceID := fs.Int("conference-id", 0, "Use existing conference ID instead of creating new one")
	acsRead := fs.String("acs-read", "", "ACS string for reading areas")
	acsWrite := fs.String("acs-write", "", "ACS string for writing areas")
	tagPrefix := fs.String("tag-prefix", "", "Prefix for area tags and base paths (e.g. fd_ for Fidonet)")
	configDir := fs.String("config", "configs", "Config directory")
	dryRun := fs.Bool("dry-run", false, "Show what would be done without modifying files")
	quiet := fs.Bool("quiet", false, "Suppress detailed output")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: helper ftnsetup [options]\n\n")
		fmt.Fprintf(os.Stderr, "Import FTN echo areas from a FIDONET.NA file.\n")
		fmt.Fprintf(os.Stderr, "Updates ftn.json, message_areas.json, and conferences.json.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  helper ftnsetup --na fsxnet.na --address 21:3/110 --hub 21:1/100 --network FSxNet\n")
		fmt.Fprintf(os.Stderr, "  helper ftnsetup --na fidonet.na --address 3:633/2744.11 --hub 3:633/2744 --tag-prefix fd_\n")
	}
	fs.Parse(args)

	if *naFile == "" || *address == "" || *hub == "" {
		fmt.Fprintf(os.Stderr, "Error: --na, --address, and --hub are required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	// Derive defaults
	if *hubName == "" {
		*hubName = "Hub " + *hub
	}
	if *network == "" {
		base := filepath.Base(*naFile)
		ext := filepath.Ext(base)
		*network = strings.TrimSuffix(base, ext)
		if len(*network) > 0 {
			*network = strings.ToUpper((*network)[:1]) + (*network)[1:]
		}
	}

	// 1. Parse the NA file
	areas, err := parseNAFile(*naFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing NA file: %v\n", err)
		os.Exit(1)
	}
	if !*quiet {
		fmt.Printf("Parsed %d echo areas from %s\n", len(areas), *naFile)
	}

	// 2. Load existing configs
	ftnPath := filepath.Join(*configDir, "ftn.json")
	areasPath := filepath.Join(*configDir, "message_areas.json")
	confsPath := filepath.Join(*configDir, "conferences.json")

	ftn, err := loadFTNConfig(ftnPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", ftnPath, err)
		os.Exit(1)
	}

	existingAreas, err := loadAreas(areasPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", areasPath, err)
		os.Exit(1)
	}

	conferences, err := loadConferences(confsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", confsPath, err)
		os.Exit(1)
	}

	// 3. Build existing tag set for duplicate detection
	existingTags := make(map[string]int) // tag -> area ID
	maxAreaID := 0
	for _, a := range existingAreas {
		existingTags[strings.ToUpper(a.Tag)] = a.ID
		if a.ID > maxAreaID {
			maxAreaID = a.ID
		}
	}

	// 4. Determine conference
	var confID int
	confIsNew := false
	networkTag := strings.ToUpper(strings.ReplaceAll(*network, " ", "_"))
	if *conferenceID > 0 {
		found := false
		for _, c := range conferences {
			if c.ID == *conferenceID {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Error: conference ID %d not found in %s\n", *conferenceID, confsPath)
			os.Exit(1)
		}
		confID = *conferenceID
	} else {
		maxConfID := 0
		for _, c := range conferences {
			if strings.EqualFold(c.Tag, networkTag) {
				confID = c.ID
				break
			}
			if c.ID > maxConfID {
				maxConfID = c.ID
			}
		}
		if confID == 0 {
			confID = maxConfID + 1
			confIsNew = true
		}
	}

	// 5. Partition areas into new vs duplicate (using effective tag = prefix + tag)
	var newAreas []naArea
	var skipped []naArea
	for _, a := range areas {
		effectiveTag := strings.ToUpper(*tagPrefix + a.Tag)
		if !isValidEchoTag(effectiveTag) {
			if !*quiet {
				fmt.Fprintf(os.Stderr, "Warn: skipping %s — tag %q exceeds length or invalid chars\n", a.Tag, effectiveTag)
			}
			continue
		}
		if _, exists := existingTags[effectiveTag]; exists {
			skipped = append(skipped, a)
		} else {
			newAreas = append(newAreas, a)
		}
	}

	// 6. Check for existing link in the target network
	networkKey := strings.ToLower(strings.ReplaceAll(*network, " ", "_"))
	netCfg := ftn.Networks[networkKey] // zero value if new network
	existingLinkIdx := -1
	for i, link := range netCfg.Links {
		if link.Address == *hub {
			existingLinkIdx = i
			break
		}
	}

	// 7. Print summary
	fmt.Println()
	fmt.Printf("Network:     %s (key: %s)\n", *network, networkKey)
	if confIsNew {
		fmt.Printf("Conference:  %s (id: %d) [NEW]\n", *network, confID)
	} else {
		fmt.Printf("Conference:  id %d [EXISTING]\n", confID)
	}
	fmt.Printf("Our Address: %s\n", *address)
	fmt.Printf("Hub:         %s\n", *hub)
	fmt.Println()

	if len(newAreas) > 0 {
		fmt.Printf("Areas to add (%d):\n", len(newAreas))
		for _, a := range newAreas {
			effectiveTag := strings.ToUpper(*tagPrefix + a.Tag)
			fmt.Printf("  %-20s %s\n", effectiveTag, a.Description)
		}
	} else {
		fmt.Println("No new areas to add.")
	}

	if len(skipped) > 0 {
		fmt.Printf("\nSkipped (%d duplicates):\n", len(skipped))
		for _, a := range skipped {
			effectiveTag := strings.ToUpper(*tagPrefix + a.Tag)
			fmt.Printf("  %-20s Already exists (id %d)\n", effectiveTag, existingTags[effectiveTag])
		}
	}

	fmt.Println("\nConfig changes (ftn.json):")
	if !netCfg.InternalTosserEnabled {
		fmt.Printf("  networks.%s.internal_tosser_enabled: false -> true\n", networkKey)
	}
	if netCfg.OwnAddress == "" {
		fmt.Printf("  networks.%s.own_address: \"\" -> %q\n", networkKey, *address)
	} else if netCfg.OwnAddress != *address {
		fmt.Printf("  networks.%s.own_address: %q (unchanged)\n", networkKey, netCfg.OwnAddress)
	}
	if existingLinkIdx >= 0 {
		fmt.Printf("  networks.%s.links: existing link %s (no change)\n", networkKey, *hub)
	} else {
		fmt.Printf("  networks.%s.links: +1 link (%s)\n", networkKey, *hub)
	}

	if len(newAreas) == 0 {
		fmt.Println("\nNothing to do — all areas already exist.")
		return
	}

	if *dryRun {
		fmt.Println("\n--dry-run: no files modified.")
		return
	}

	// 8. Apply changes

	// 8a. Add conference if new
	if confIsNew {
		conferences = append(conferences, conferenceConfig{
			ID:          confID,
			Tag:         networkTag,
			Name:        *network,
			Description: fmt.Sprintf("%s message areas", *network),
			ACS:         "",
		})
	}

	// 8b. Add message areas
	nextID := maxAreaID + 1
	for _, a := range newAreas {
		effectiveTag := strings.ToUpper(*tagPrefix + a.Tag)
		basePath := "msgbases/" + strings.ToLower(effectiveTag)
		existingAreas = append(existingAreas, message.MessageArea{
			ID:           nextID,
			Tag:          effectiveTag,
			Name:         a.Description,
			Description:  a.Description,
			ACSRead:      *acsRead,
			ACSWrite:     *acsWrite,
			ConferenceID: confID,
			BasePath:     basePath,
			AreaType:     "echomail",
			EchoTag:      a.Tag,
			OriginAddr:   *address,
			Network:      networkKey,
			MaxMessages:  1000,
			MaxAge:       365,
		})
		nextID++
	}

	// 8c. Update FTN network config
	netCfg.InternalTosserEnabled = true
	if netCfg.OwnAddress == "" {
		netCfg.OwnAddress = *address
	}
	if ftn.InboundPath == "" {
		ftn.InboundPath = "data/ftn/in"
	}
	if ftn.OutboundPath == "" {
		ftn.OutboundPath = "data/ftn/temp_out"
	}
	if ftn.BinkdOutboundPath == "" {
		ftn.BinkdOutboundPath = "data/ftn/out"
	}
	if ftn.TempPath == "" {
		ftn.TempPath = "data/ftn/temp_in"
	}
	if netCfg.PollSeconds == 0 {
		netCfg.PollSeconds = 300
	}

	if existingLinkIdx < 0 {
		// Add link if not already present; echo area routing is managed via Message Areas.
		netCfg.Links = append(netCfg.Links, linkConfig{
			Address:        *hub,
			PacketPassword: *hubPassword,
			Name:           *hubName,
		})
	}

	ftn.Networks[networkKey] = netCfg
	if ftn.DupeDBPath == "" {
		ftn.DupeDBPath = "data/ftn/dupes.json"
	}

	// 9. Write files
	if err := writeJSON(confsPath, conferences); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", confsPath, err)
		os.Exit(1)
	}
	if !*quiet {
		fmt.Printf("\nWrote %s\n", confsPath)
	}

	if err := writeJSON(areasPath, existingAreas); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", areasPath, err)
		os.Exit(1)
	}
	if !*quiet {
		fmt.Printf("Wrote %s\n", areasPath)
	}

	if err := writeJSON(ftnPath, ftn); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", ftnPath, err)
		os.Exit(1)
	}
	if !*quiet {
		fmt.Printf("Wrote %s\n", ftnPath)
	}

	fmt.Printf("\nDone. Added %d echo areas for %s.\n", len(newAreas), *network)
}

// --- areafix command ---

func cmdAreafix(args []string) {
	fs := flag.NewFlagSet("areafix", flag.ExitOnError)
	network := fs.String("network", "", "FTN network name (required)")
	command := fs.String("command", "", "AreaFix command, e.g. %LIST or +LINUX (required unless --seed)")
	seed := fs.Bool("seed", false, "Subscribe to all echomail areas in message_areas.json for this network, with R=<n> messages")
	seedMessages := fs.Int("seed-messages", 25, "Number of old messages to rescan when using --seed")
	linkAddr := fs.String("link", "", "Hub address (default: first link of network)")
	configDir := fs.String("config", "configs", "Config directory")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: helper areafix [options]\n\n")
		fmt.Fprintf(os.Stderr, "Send an AreaFix netmail message to a network hub.\n")
		fmt.Fprintf(os.Stderr, "To: AreaFix, Subject: <areafix_password>, Body: <command>---\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nAreaFix commands: %%HELP %%LIST %%QUERY %%UNLINKED +area -area =area,R=<n> %%RESCAN\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  helper areafix --network fidonet --command \"%%LIST\"\n")
		fmt.Fprintf(os.Stderr, "  helper areafix --network fidonet --seed\n")
		fmt.Fprintf(os.Stderr, "  helper areafix --network fidonet --seed --seed-messages 50\n")
	}
	fs.Parse(args)

	if *network == "" {
		fmt.Fprintf(os.Stderr, "Error: --network is required\n\n")
		fs.Usage()
		os.Exit(1)
	}
	if !*seed && *command == "" {
		fmt.Fprintf(os.Stderr, "Error: --command or --seed is required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	ftnPath := filepath.Join(*configDir, "ftn.json")
	ftnCfg, err := loadFTNConfig(ftnPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", ftnPath, err)
		os.Exit(1)
	}

	// Find network (case-insensitive)
	netKey := ""
	for k := range ftnCfg.Networks {
		if strings.EqualFold(k, *network) {
			netKey = k
			break
		}
	}
	if netKey == "" {
		fmt.Fprintf(os.Stderr, "Error: network %q not found in %s\n", *network, ftnPath)
		os.Exit(1)
	}

	netCfg := ftnCfg.Networks[netKey]
	if len(netCfg.Links) == 0 {
		fmt.Fprintf(os.Stderr, "Error: network %s has no links\n", netKey)
		os.Exit(1)
	}

	var link *config.FTNLinkConfig
	if *linkAddr != "" {
		for i := range netCfg.Links {
			if netCfg.Links[i].Address == *linkAddr {
				link = &netCfg.Links[i]
				break
			}
		}
		if link == nil {
			fmt.Fprintf(os.Stderr, "Error: link %s not found in network %s\n", *linkAddr, netKey)
			os.Exit(1)
		}
	} else {
		link = &netCfg.Links[0]
	}

	if link.AreafixPassword == "" {
		fmt.Fprintf(os.Stderr, "Error: link %s has no areafix_password configured\n", link.Address)
		os.Exit(1)
	}

	// Use SysOp name from config so return messages reach the SysOp's inbox
	fromName := "SysOp"
	if cfg, err := config.LoadServerConfig(*configDir); err == nil && cfg.SysOpName != "" {
		fromName = cfg.SysOpName
	}

	ownAddr, err := jam.ParseAddress(netCfg.OwnAddress)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid own_address %q: %v\n", netCfg.OwnAddress, err)
		os.Exit(1)
	}

	destAddr, err := jam.ParseAddress(link.Address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid link address %q: %v\n", link.Address, err)
		os.Exit(1)
	}

	// Build netmail body: command(s) + "---" per areafix spec
	var body string
	var seedCount int
	if *seed {
		areasPath := filepath.Join(*configDir, "message_areas.json")
		areas, err := loadAreas(areasPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", areasPath, err)
			os.Exit(1)
		}
		var lines []string
		for _, a := range areas {
			if (a.AreaType != "echomail" && a.AreaType != "echo") || !strings.EqualFold(a.Network, netKey) {
				continue
			}
			tag := a.EchoTag
			if tag == "" {
				tag = a.Tag
			}
			if tag == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("+%s,R=%d", tag, *seedMessages))
		}
		if len(lines) == 0 {
			fmt.Fprintf(os.Stderr, "Error: no echomail areas found for network %s in message_areas.json\n", netKey)
			os.Exit(1)
		}
		seedCount = len(lines)
		body = strings.Join(lines, "\r") + "\r---\r"
		fmt.Printf("Seeding %d areas with +area,R=%d for network %s\n", seedCount, *seedMessages, netKey)
	} else {
		body = strings.TrimRight(*command, "\r\n") + "\r---\r"
	}

	// Netmail has no AREA kludge; add MSGID, PID for routing
	msgID := fmt.Sprintf("%s %08X", netCfg.OwnAddress, uint32(time.Now().UnixNano()&0xFFFFFFFF))
	parsed := &ftn.ParsedBody{
		Text:    body,
		Kludges: []string{"MSGID: " + msgID, "PID: " + jam.FormatPID()},
	}
	bodyBytes := ftn.FormatPackedMessageBody(parsed)

	msg := &ftn.PackedMessage{
		MsgType:  2,
		OrigNode: uint16(ownAddr.Node),
		DestNode: uint16(destAddr.Node),
		OrigNet:  uint16(ownAddr.Net),
		DestNet:  uint16(destAddr.Net),
		Attr:     ftn.MsgAttrLocal | ftn.MsgAttrCrash,
		Cost:     0,
		DateTime: ftn.FormatFTNDateTime(time.Now()),
		To:       "AreaFix",
		From:     fromName,
		Subject:  link.AreafixPassword,
		Body:     bodyBytes,
	}

	hdr := ftn.NewPacketHeader(
		uint16(ownAddr.Zone), uint16(ownAddr.Net), uint16(ownAddr.Node), uint16(ownAddr.Point),
		uint16(destAddr.Zone), uint16(destAddr.Net), uint16(destAddr.Node), uint16(destAddr.Point),
		link.PacketPassword,
	)

	outboundPath := ftnCfg.OutboundPath
	if outboundPath == "" {
		outboundPath = "data/ftn/temp_out"
	}
	if err := os.MkdirAll(outboundPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating outbound dir: %v\n", err)
		os.Exit(1)
	}

	f, err := os.CreateTemp(outboundPath, "areafix_*.pkt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating packet: %v\n", err)
		os.Exit(1)
	}
	pktPath := f.Name()
	if err := ftn.WritePacket(f, hdr, []*ftn.PackedMessage{msg}); err != nil {
		f.Close()
		os.Remove(pktPath)
		fmt.Fprintf(os.Stderr, "Error writing packet: %v\n", err)
		os.Exit(1)
	}
	f.Close()

	fmt.Printf("AreaFix netmail written to %s\n", pktPath)
	fmt.Printf("  To: AreaFix @ %s\n", link.Address)
	if *seed {
		fmt.Printf("  Seeded %d areas with R=%d messages each\n", seedCount, *seedMessages)
	} else {
		fmt.Printf("  Command: %s\n", *command)
	}
}

// --- NA file parser ---

func parseNAFile(path string) ([]naArea, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	var areas []naArea
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		tag := parts[0]
		desc := strings.Join(parts[1:], " ")

		if !isValidEchoTag(tag) {
			continue
		}

		areas = append(areas, naArea{Tag: tag, Description: desc})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	if len(areas) == 0 {
		return nil, fmt.Errorf("no valid areas found")
	}
	return areas, nil
}

func isValidEchoTag(tag string) bool {
	if tag == "" || len(tag) > 50 {
		return false
	}
	for _, r := range tag {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}

// --- Config I/O ---

func loadFTNConfig(path string) (ftnConfig, error) {
	var cfg ftnConfig
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg.Networks = make(map[string]ftnNetworkConfig)
			return cfg, nil // Fresh config
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Networks == nil {
		cfg.Networks = make(map[string]ftnNetworkConfig)
	}
	return cfg, nil
}

func loadAreas(path string) ([]message.MessageArea, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var areas []message.MessageArea
	if err := json.Unmarshal(data, &areas); err != nil {
		return nil, err
	}
	return areas, nil
}

func loadConferences(path string) ([]conferenceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var confs []conferenceConfig
	if err := json.Unmarshal(data, &confs); err != nil {
		return nil, err
	}
	return confs, nil
}

func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
