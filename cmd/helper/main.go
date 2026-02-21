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
	"github.com/stlalpha/vision3/internal/user"
)

// --- NA file types ---

type naArea struct {
	Tag         string
	Description string
}

// --- Config types (local, minimal) ---

type areaConfig struct {
	ID           int    `json:"id"`
	Tag          string `json:"tag"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ACSRead      string `json:"acs_read"`
	ACSWrite     string `json:"acs_write"`
	AllowAnon    bool   `json:"allow_anonymous"`
	RealNameOnly bool   `json:"real_name_only"`
	ConferenceID int    `json:"conference_id,omitempty"`
	BasePath     string `json:"base_path"`
	AreaType     string `json:"area_type"`
	EchoTag      string `json:"echo_tag,omitempty"`
	OriginAddr   string `json:"origin_addr,omitempty"`
	Network      string `json:"network,omitempty"`
}

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

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "ftnsetup":
		cmdFTNSetup(os.Args[2:])
	case "users":
		cmdUsers(os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: helper <command> [options]

Commands:
  ftnsetup    Import FTN echo areas from a FIDONET.NA file
  users       Manage user accounts (purge, list)

Run 'helper <command> --help' for command-specific options.
`)
}

// --- users command group ---

func cmdUsers(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, `Usage: helper users <subcommand> [options]

Subcommands:
  purge    Permanently remove soft-deleted users past the retention period
  list     List users (optionally filtered to deleted accounts)

Run 'helper users <subcommand> --help' for subcommand-specific options.
`)
		os.Exit(1)
	}

	sub := args[0]
	switch sub {
	case "purge":
		cmdUsersPurge(args[1:])
	case "list":
		cmdUsersList(args[1:])
	case "help", "--help", "-h":
		fmt.Fprintf(os.Stderr, `Usage: helper users <subcommand> [options]

Subcommands:
  purge    Permanently remove soft-deleted users past the retention period
  list     List users (optionally filtered to deleted accounts)
`)
	default:
		fmt.Fprintf(os.Stderr, "Unknown users subcommand: %s\n\n", sub)
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
		if u.DeletedAt == nil || u.DeletedAt.Before(cutoff) {
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

	// 5. Partition areas into new vs duplicate
	var newAreas []naArea
	var skipped []naArea
	for _, a := range areas {
		if _, exists := existingTags[strings.ToUpper(a.Tag)]; exists {
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
			fmt.Printf("  %-20s %s\n", a.Tag, a.Description)
		}
	} else {
		fmt.Println("No new areas to add.")
	}

	if len(skipped) > 0 {
		fmt.Printf("\nSkipped (%d duplicates):\n", len(skipped))
		for _, a := range skipped {
			fmt.Printf("  %-20s Already exists (id %d)\n", a.Tag, existingTags[strings.ToUpper(a.Tag)])
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
		fmt.Printf("  networks.%s.links: merge %d echo areas into existing link %s\n", networkKey, len(newAreas), *hub)
	} else {
		fmt.Printf("  networks.%s.links: +1 link (%s, %d echo areas)\n", networkKey, *hub, len(newAreas))
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
	echoTags := make([]string, 0, len(newAreas))
	for _, a := range newAreas {
		basePath := "msgbases/" + strings.ToLower(a.Tag)
		existingAreas = append(existingAreas, areaConfig{
			ID:           nextID,
			Tag:          a.Tag,
			Name:         a.Description,
			Description:  a.Description,
			ACSRead:      *acsRead,
			ACSWrite:     *acsWrite,
			AllowAnon:    false,
			RealNameOnly: false,
			ConferenceID: confID,
			BasePath:     basePath,
			AreaType:     "echomail",
			EchoTag:      a.Tag,
			OriginAddr:   *address,
			Network:      networkKey,
		})
		echoTags = append(echoTags, a.Tag)
		nextID++
	}

	// 8c. Update FTN network config
	netCfg.InternalTosserEnabled = true
	if netCfg.OwnAddress == "" {
		netCfg.OwnAddress = *address
	}
	if netCfg.InboundPath == "" {
		netCfg.InboundPath = fmt.Sprintf("data/ftn/%s/inbound", networkKey)
	}
	if netCfg.OutboundPath == "" {
		netCfg.OutboundPath = fmt.Sprintf("data/ftn/%s/outbound", networkKey)
	}
	if netCfg.TempPath == "" {
		netCfg.TempPath = fmt.Sprintf("data/ftn/%s/temp", networkKey)
	}
	if netCfg.PollSeconds == 0 {
		netCfg.PollSeconds = 300
	}

	if existingLinkIdx >= 0 {
		existing := make(map[string]bool)
		for _, t := range netCfg.Links[existingLinkIdx].EchoAreas {
			existing[strings.ToUpper(t)] = true
		}
		for _, t := range echoTags {
			if !existing[strings.ToUpper(t)] {
				netCfg.Links[existingLinkIdx].EchoAreas = append(netCfg.Links[existingLinkIdx].EchoAreas, t)
			}
		}
	} else {
		netCfg.Links = append(netCfg.Links, linkConfig{
			Address:   *hub,
			Password:  *hubPassword,
			Name:      *hubName,
			EchoAreas: echoTags,
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

func loadAreas(path string) ([]areaConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var areas []areaConfig
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
