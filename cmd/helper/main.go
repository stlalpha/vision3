package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	ConferenceID int    `json:"conference_id,omitempty"`
	BasePath     string `json:"base_path"`
	AreaType     string `json:"area_type"`
	EchoTag      string `json:"echo_tag,omitempty"`
	OriginAddr   string `json:"origin_addr,omitempty"`
}

type conferenceConfig struct {
	ID          int    `json:"id"`
	Tag         string `json:"tag"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ACS         string `json:"acs"`
}

type ftnConfig struct {
	Enabled      bool         `json:"enabled"`
	OwnAddress   string       `json:"own_address"`
	InboundPath  string       `json:"inbound_path"`
	OutboundPath string       `json:"outbound_path"`
	TempPath     string       `json:"temp_path"`
	DupeDBPath   string       `json:"dupe_db_path"`
	PollSeconds  int          `json:"poll_interval_seconds"`
	Links        []linkConfig `json:"links"`
}

type linkConfig struct {
	Address   string   `json:"address"`
	Password  string   `json:"password"`
	Name      string   `json:"name"`
	EchoAreas []string `json:"echo_areas"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "ftnsetup":
		cmdFTNSetup(os.Args[2:])
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

Run 'helper <command> --help' for command-specific options.
`)
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
		fmt.Fprintf(os.Stderr, "Updates config.json, message_areas.json, and conferences.json.\n\n")
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
	configPath := filepath.Join(*configDir, "config.json")
	areasPath := filepath.Join(*configDir, "message_areas.json")
	confsPath := filepath.Join(*configDir, "conferences.json")

	serverCfg, err := loadServerConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", configPath, err)
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

	// 6. Check for existing link
	ftn := extractFTN(serverCfg)
	existingLinkIdx := -1
	for i, link := range ftn.Links {
		if link.Address == *hub {
			existingLinkIdx = i
			break
		}
	}

	// 7. Print summary
	fmt.Println()
	fmt.Printf("Network:     %s\n", *network)
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

	fmt.Println("\nConfig changes:")
	if !ftn.Enabled {
		fmt.Println("  ftn.enabled: false -> true")
	}
	if ftn.OwnAddress == "" {
		fmt.Printf("  ftn.own_address: \"\" -> %q\n", *address)
	} else if ftn.OwnAddress != *address {
		fmt.Printf("  ftn.own_address: %q (unchanged)\n", ftn.OwnAddress)
	}
	if existingLinkIdx >= 0 {
		fmt.Printf("  ftn.links: merge %d echo areas into existing link %s\n", len(newAreas), *hub)
	} else {
		fmt.Printf("  ftn.links: +1 link (%s, %d echo areas)\n", *hub, len(newAreas))
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
			ConferenceID: confID,
			BasePath:     basePath,
			AreaType:     "echomail",
			EchoTag:      a.Tag,
			OriginAddr:   *address,
		})
		echoTags = append(echoTags, a.Tag)
		nextID++
	}

	// 8c. Update FTN config
	ftn.Enabled = true
	if ftn.OwnAddress == "" {
		ftn.OwnAddress = *address
	}
	if ftn.InboundPath == "" {
		ftn.InboundPath = "data/ftn/inbound"
	}
	if ftn.OutboundPath == "" {
		ftn.OutboundPath = "data/ftn/outbound"
	}
	if ftn.TempPath == "" {
		ftn.TempPath = "data/ftn/temp"
	}
	if ftn.DupeDBPath == "" {
		ftn.DupeDBPath = "data/ftn/dupes.json"
	}
	if ftn.PollSeconds == 0 {
		ftn.PollSeconds = 300
	}

	if existingLinkIdx >= 0 {
		existing := make(map[string]bool)
		for _, t := range ftn.Links[existingLinkIdx].EchoAreas {
			existing[strings.ToUpper(t)] = true
		}
		for _, t := range echoTags {
			if !existing[strings.ToUpper(t)] {
				ftn.Links[existingLinkIdx].EchoAreas = append(ftn.Links[existingLinkIdx].EchoAreas, t)
			}
		}
	} else {
		ftn.Links = append(ftn.Links, linkConfig{
			Address:   *hub,
			Password:  *hubPassword,
			Name:      *hubName,
			EchoAreas: echoTags,
		})
	}

	setFTN(serverCfg, ftn)

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

	if err := writeJSON(configPath, serverCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", configPath, err)
		os.Exit(1)
	}
	if !*quiet {
		fmt.Printf("Wrote %s\n", configPath)
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

func loadServerConfig(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func extractFTN(cfg map[string]json.RawMessage) ftnConfig {
	var ftn ftnConfig
	raw, ok := cfg["ftn"]
	if !ok {
		return ftn
	}
	json.Unmarshal(raw, &ftn) //nolint: zero-valued on error is fine
	return ftn
}

func setFTN(cfg map[string]json.RawMessage, ftn ftnConfig) {
	data, _ := json.Marshal(ftn)
	cfg["ftn"] = json.RawMessage(data)
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
