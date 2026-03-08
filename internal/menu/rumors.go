package menu

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// RumorRecord represents a single rumor entry.
// Simplified from V2's RumorRec — title dropped in favor of text-only graffiti wall.
type RumorRecord struct {
	ID       int       `json:"id"`
	Author   string    `json:"author"`    // Displayed author (may be anonymous)
	RealUser string    `json:"real_user"` // Actual username (V2: Author2)
	Text     string    `json:"text"`      // Rumor text
	PostedAt time.Time `json:"posted_at"` // When posted
	MinLevel int       `json:"min_level"` // Minimum access level to view
}

// rumorsData holds all rumors with a NextID counter.
type rumorsData struct {
	Rumors []RumorRecord `json:"rumors"`
	NextID int           `json:"next_id"`
}

var rumorsMu sync.Mutex

func rumorsFilePath(rootConfigPath string) string {
	return filepath.Join(rootConfigPath, "..", "data", "rumors.json")
}

func loadRumorsData(rootConfigPath string) (*rumorsData, error) {
	data, err := os.ReadFile(rumorsFilePath(rootConfigPath))
	if err != nil {
		if os.IsNotExist(err) {
			return &rumorsData{NextID: 1}, nil
		}
		return nil, fmt.Errorf("read rumors.json: %w", err)
	}
	var rd rumorsData
	if err := json.Unmarshal(data, &rd); err != nil {
		return nil, fmt.Errorf("parse rumors.json: %w", err)
	}
	if rd.NextID < 1 {
		maxID := 0
		for _, r := range rd.Rumors {
			if r.ID > maxID {
				maxID = r.ID
			}
		}
		rd.NextID = maxID + 1
	}
	return &rd, nil
}

func saveRumorsData(rootConfigPath string, rd *rumorsData) error {
	data, err := json.MarshalIndent(rd, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal rumors data: %w", err)
	}
	return os.WriteFile(rumorsFilePath(rootConfigPath), data, 0644)
}

// visibleRumors returns indices of rumors the user can see based on access level.
func visibleRumors(rd *rumorsData, userLevel int) []int {
	var visible []int
	for i, r := range rd.Rumors {
		if userLevel >= r.MinLevel {
			visible = append(visible, i)
		}
	}
	return visible
}

// rumorDisplayAuthor returns the display name for a rumor author.
func rumorDisplayAuthor(r *RumorRecord, isSysop bool, anonymousName string) string {
	if strings.TrimSpace(anonymousName) == "" {
		anonymousName = "Anonymous"
	}
	if r.Author == "" || r.Author == anonymousName {
		if isSysop {
			return fmt.Sprintf("%s (%s)", anonymousName, r.RealUser)
		}
		return anonymousName
	}
	return r.Author
}

// rumorAnonName returns the configured anonymous display name.
func rumorAnonName(e *MenuExecutor) string {
	name := e.LoadedStrings.AnonymousName
	if strings.TrimSpace(name) == "" {
		return "Anonymous"
	}
	return name
}

// getRandomRumorText returns a random visible rumor's text for MCI substitution.
// Returns empty string if no rumors are available.
func getRandomRumorText(rootConfigPath string, userLevel int) string {
	rumorsMu.Lock()
	rd, err := loadRumorsData(rootConfigPath)
	rumorsMu.Unlock()
	if err != nil || len(rd.Rumors) == 0 {
		return ""
	}

	visible := visibleRumors(rd, userLevel)
	if len(visible) == 0 {
		return ""
	}

	idx := visible[rand.Intn(len(visible))]
	return rd.Rumors[idx].Text
}

// runRumorsList displays all visible rumors.
// Maps to V2's ListRumors procedure (simplified — no Stats/Both modes).
func runRumorsList(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return currentUser, "", nil
	}

	log.Printf("DEBUG: Node %d: Running RUMORSLIST for user %s", nodeNumber, currentUser.Handle)

	// Clear screen before listing
	wv(terminal, "\x1b[2J\x1b[H", outputMode)

	isSysop := currentUser.AccessLevel >= 255
	userLevel := currentUser.AccessLevel
	anonName := rumorAnonName(e)

	rumorsMu.Lock()
	rd, err := loadRumorsData(e.RootConfigPath)
	rumorsMu.Unlock()
	if err != nil {
		wv(terminal, "\r\n|04Error loading rumors.\r\n", outputMode)
		return currentUser, "", nil
	}

	visible := visibleRumors(rd, userLevel)
	if len(visible) == 0 {
		wv(terminal, "\r\n|07There are no rumors!\r\n", outputMode)
		return currentUser, "", nil
	}

	wv(terminal, fmt.Sprintf("\r\n|11%-4s%-42s%-16s%s\r\n", "#", "Rumor", "Author", "Date"), outputMode)
	wv(terminal, "|08"+strings.Repeat("\xc4", 70)+"\r\n", outputMode)

	for _, idx := range visible {
		r := &rd.Rumors[idx]
		author := rumorDisplayAuthor(r, isSysop, anonName)
		wv(terminal, fmt.Sprintf("|03%-4d|07%-42s|11%-16s|07%s\r\n",
			r.ID, truncateRunes(r.Text, 41), truncateRunes(author, 15), r.PostedAt.Format("01/02/06")), outputMode)
	}

	e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
	return currentUser, "", nil
}

// runRumorsAdd lets users post a new rumor.
// Maps to V2's AddRumor procedure.
func runRumorsAdd(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return currentUser, "", nil
	}

	log.Printf("DEBUG: Node %d: Running RUMORSADD for user %s", nodeNumber, currentUser.Handle)

	userLevel := currentUser.AccessLevel
	anonName := rumorAnonName(e)

	if userLevel < 2 {
		wv(terminal, "\r\n|04You need at least level 2 to add rumors.\r\n", outputMode)
		return currentUser, "", nil
	}

	// Check max rumors (V2: 999 limit)
	rumorsMu.Lock()
	rd, err := loadRumorsData(e.RootConfigPath)
	rumorsMu.Unlock()
	if err != nil {
		wv(terminal, "\r\n|04Error loading rumors.\r\n", outputMode)
		return currentUser, "", nil
	}
	if len(rd.Rumors) >= 999 {
		wv(terminal, "\r\n|04Sorry, there are too many rumors! Ask the SysOp to delete some.\r\n", outputMode)
		return currentUser, "", nil
	}

	// Anonymous option (V2: only if user level >= AnonymousLevel)
	author := currentUser.Handle
	realUser := currentUser.Username
	allowAnon := userLevel >= e.ServerCfg.AnonymousLevel
	if allowAnon {
		anonPrompt := e.LoadedStrings.AddRumorAnonymous
		if anonPrompt == "" {
			anonPrompt = "|09Anonymous? @"
		}
		anonYes, anonErr := e.PromptYesNo(s, terminal, anonPrompt, outputMode, nodeNumber, termWidth, termHeight, false)
		if anonErr != nil {
			if errors.Is(anonErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
		} else if anonYes {
			author = anonName
		}
	}

	// Min level to see (V2: Level_To_See_Rumor)
	wv(terminal, "|08Minimum security level required to view this rumor |07(|151-255|07, |15Enter|07=1|07)\r\n", outputMode)
	levelPrompt := e.LoadedStrings.EnterRumorLevel
	if levelPrompt == "" {
		levelPrompt = "|09Level|08 : "
	}
	minLevel := 1
	for {
		wv(terminal, levelPrompt, outputMode)
		levelInput, err := readLineFromSessionIH(s, terminal)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return currentUser, "", nil
		}
		if strings.TrimSpace(levelInput) == "" {
			break // default to 1
		}
		v, nerr := strconv.Atoi(strings.TrimSpace(levelInput))
		if nerr != nil || v < 1 || v > 255 {
			wv(terminal, "|04Invalid level. Enter a number from 1-255.\r\n", outputMode)
			continue
		}
		minLevel = v
		break
	}

	// Rumor text
	enterPrompt := e.LoadedStrings.EnterRumorPrompt
	if enterPrompt == "" {
		enterPrompt = "|09Enter Rumor |08(|15Enter|08/|15Abort|08)|07:\r\n"
	}
	wv(terminal, enterPrompt, outputMode)
	rumorText, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return currentUser, "", nil
	}
	if strings.TrimSpace(rumorText) == "" {
		return currentUser, "", nil
	}

	newRumor := RumorRecord{
		Author:   author,
		RealUser: realUser,
		Text:     rumorText,
		PostedAt: time.Now().UTC(),
		MinLevel: minLevel,
	}

	rumorsMu.Lock()
	rd, err = loadRumorsData(e.RootConfigPath)
	if err != nil {
		rumorsMu.Unlock()
		wv(terminal, "\r\n|04Error saving rumor.\r\n", outputMode)
		return currentUser, "", nil
	}
	if len(rd.Rumors) >= 999 {
		rumorsMu.Unlock()
		wv(terminal, "\r\n|04Sorry, there are too many rumors! Ask the SysOp to delete some.\r\n", outputMode)
		return currentUser, "", nil
	}
	newRumor.ID = rd.NextID
	rd.NextID++
	rd.Rumors = append(rd.Rumors, newRumor)
	saveErr := saveRumorsData(e.RootConfigPath, rd)
	rumorsMu.Unlock()

	if saveErr != nil {
		log.Printf("ERROR: Node %d: Failed to save rumor: %v", nodeNumber, saveErr)
		wv(terminal, "\r\n|04Error saving rumor.\r\n", outputMode)
		return currentUser, "", nil
	}

	addedMsg := e.LoadedStrings.RumorAdded
	if addedMsg == "" {
		addedMsg = "|10Rumor has been added!"
	}
	wv(terminal, "\r\n"+addedMsg+"\r\n", outputMode)
	time.Sleep(1 * time.Second)
	log.Printf("INFO: Node %d: %s added rumor #%d", nodeNumber, currentUser.Handle, newRumor.ID)
	return currentUser, "", nil
}

// runRumorsDelete lets users delete their own rumors; sysops can delete any.
// Maps to V2's DeleteRumor procedure.
func runRumorsDelete(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return currentUser, "", nil
	}

	log.Printf("DEBUG: Node %d: Running RUMORSDELETE for user %s", nodeNumber, currentUser.Handle)

	isSysop := currentUser.AccessLevel >= 255
	userLevel := currentUser.AccessLevel
	anonName := rumorAnonName(e)

	rumorsMu.Lock()
	rd, err := loadRumorsData(e.RootConfigPath)
	rumorsMu.Unlock()
	if err != nil {
		wv(terminal, "\r\n|04Error loading rumors.\r\n", outputMode)
		return currentUser, "", nil
	}

	visible := visibleRumors(rd, userLevel)
	if len(visible) == 0 {
		wv(terminal, "\r\n|07No rumors to delete.\r\n", outputMode)
		return currentUser, "", nil
	}

	wv(terminal, "\r\n|07Rumor number to delete [?=List]: ", outputMode)
	input, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return currentUser, "", nil
	}
	if strings.TrimSpace(input) == "" {
		return currentUser, "", nil
	}

	if strings.TrimSpace(input) == "?" {
		for _, idx := range visible {
			r := &rd.Rumors[idx]
			author := rumorDisplayAuthor(r, isSysop, anonName)
			wv(terminal, fmt.Sprintf("|03%-4d|07%-50s |11%s\r\n", r.ID, truncateRunes(r.Text, 48), author), outputMode)
		}
		wv(terminal, "\r\n|07Rumor number to delete: ", outputMode)
		input, err = readLineFromSessionIH(s, terminal)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return currentUser, "", nil
		}
		if strings.TrimSpace(input) == "" {
			return currentUser, "", nil
		}
	}

	num, nerr := strconv.Atoi(strings.TrimSpace(input))
	if nerr != nil {
		wv(terminal, "\r\n|04Invalid number.\r\n", outputMode)
		return currentUser, "", nil
	}

	// Find rumor by ID
	rumorIdx := -1
	for i, r := range rd.Rumors {
		if r.ID == num {
			rumorIdx = i
			break
		}
	}
	if rumorIdx < 0 {
		wv(terminal, "\r\n|04Rumor not found.\r\n", outputMode)
		return currentUser, "", nil
	}

	r := &rd.Rumors[rumorIdx]
	if userLevel < r.MinLevel {
		wv(terminal, "\r\n|04Rumor not found.\r\n", outputMode)
		return currentUser, "", nil
	}

	// Ownership check (V2: only sysop or author can delete)
	if !isSysop && !strings.EqualFold(r.RealUser, currentUser.Username) {
		wv(terminal, "\r\n|04You didn't post that!\r\n", outputMode)
		return currentUser, "", nil
	}

	// Confirm
	wv(terminal, fmt.Sprintf("\r\n|07%s\r\n", r.Text), outputMode)
	delYes, delErr := e.PromptYesNo(s, terminal, "|09Delete this rumor? @", outputMode, nodeNumber, termWidth, termHeight, false)
	if delErr != nil || !delYes {
		return currentUser, "", nil
	}

	rumorsMu.Lock()
	rd, err = loadRumorsData(e.RootConfigPath)
	if err != nil {
		rumorsMu.Unlock()
		return currentUser, "", nil
	}
	for i, rr := range rd.Rumors {
		if rr.ID == num {
			rd.Rumors = append(rd.Rumors[:i], rd.Rumors[i+1:]...)
			break
		}
	}
	saveErr := saveRumorsData(e.RootConfigPath, rd)
	rumorsMu.Unlock()

	if saveErr != nil {
		log.Printf("ERROR: Node %d: Failed to delete rumor #%d: %v", nodeNumber, num, saveErr)
		wv(terminal, "\r\n|04Error deleting rumor.\r\n", outputMode)
		return currentUser, "", nil
	}

	wv(terminal, "\r\n|10Rumor deleted.\r\n", outputMode)
	log.Printf("INFO: Node %d: %s deleted rumor #%d", nodeNumber, currentUser.Handle, num)
	return currentUser, "", nil
}

// runRumorsSearch searches rumors by text or author.
// Maps to V2's SearchForText procedure.
func runRumorsSearch(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return currentUser, "", nil
	}

	log.Printf("DEBUG: Node %d: Running RUMORSSEARCH for user %s", nodeNumber, currentUser.Handle)

	isSysop := currentUser.AccessLevel >= 255
	userLevel := currentUser.AccessLevel
	anonName := rumorAnonName(e)

	rumorsMu.Lock()
	rd, err := loadRumorsData(e.RootConfigPath)
	rumorsMu.Unlock()
	if err != nil {
		wv(terminal, "\r\n|04Error loading rumors.\r\n", outputMode)
		return currentUser, "", nil
	}

	if len(rd.Rumors) == 0 {
		wv(terminal, "\r\n|07No rumors exist!\r\n", outputMode)
		return currentUser, "", nil
	}

	wv(terminal, "\r\n|15Search for text in rumors\r\n|07Enter text to search for:\r\n|07> ", outputMode)
	searchInput, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return currentUser, "", nil
	}
	if strings.TrimSpace(searchInput) == "" {
		return currentUser, "", nil
	}
	searchTerm := strings.ToUpper(strings.TrimSpace(searchInput))

	wv(terminal, "\r\n", outputMode)
	found := 0
	for _, r := range rd.Rumors {
		if userLevel < r.MinLevel {
			continue
		}
		match := strings.Contains(strings.ToUpper(r.Text), searchTerm) ||
			strings.Contains(strings.ToUpper(r.Author), searchTerm)
		if !match {
			continue
		}
		found++
		author := rumorDisplayAuthor(&r, isSysop, anonName)
		wv(terminal, fmt.Sprintf("|03%-4d|07%s |08by |11%s\r\n", r.ID, r.Text, author), outputMode)
	}

	if found == 0 {
		wv(terminal, "|07No matching rumors found.\r\n", outputMode)
	}

	return currentUser, "", nil
}

// runRumorsNewscan shows rumors posted since the user's last login.
// Maps to V2's RumorsNewscan procedure.
func runRumorsNewscan(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return currentUser, "", nil
	}

	log.Printf("DEBUG: Node %d: Running RUMORSNEWSCAN for user %s", nodeNumber, currentUser.Handle)

	// Clear screen before newscan
	wv(terminal, "\x1b[2J\x1b[H", outputMode)

	isSysop := currentUser.AccessLevel >= 255
	userLevel := currentUser.AccessLevel
	anonName := rumorAnonName(e)

	rumorsMu.Lock()
	rd, err := loadRumorsData(e.RootConfigPath)
	rumorsMu.Unlock()
	if err != nil {
		wv(terminal, "\r\n|04Error loading rumors.\r\n", outputMode)
		return currentUser, "", nil
	}

	wv(terminal, "\r\n|15Rumors Newscan\r\n|08"+strings.Repeat("\xc4", 50)+"\r\n", outputMode)

	lastLogin := currentUser.LastLogin
	found := 0
	for _, r := range rd.Rumors {
		if userLevel < r.MinLevel {
			continue
		}
		if !r.PostedAt.After(lastLogin) && !lastLogin.IsZero() {
			continue
		}
		found++
		author := rumorDisplayAuthor(&r, isSysop, anonName)
		wv(terminal, fmt.Sprintf("|03%-4d|07%s |08by |11%s\r\n", r.ID, r.Text, author), outputMode)
	}

	if found == 0 {
		wv(terminal, "|07No new rumors since your last login.\r\n", outputMode)
	}

	e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
	return currentUser, "", nil
}

// runRandomRumor displays a random rumor at login (V2: randomrumor from MAINR2.PAS).
// Intended for use in the login sequence.
func runRandomRumor(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return currentUser, "", nil
	}

	rumorsMu.Lock()
	rd, err := loadRumorsData(e.RootConfigPath)
	rumorsMu.Unlock()
	if err != nil {
		return currentUser, "", nil
	}

	visible := visibleRumors(rd, currentUser.AccessLevel)
	if len(visible) == 0 {
		return currentUser, "", nil
	}

	idx := visible[rand.Intn(len(visible))]
	r := &rd.Rumors[idx]

	// Center the rumor text (V2 centered it on 80-col screen)
	rumorText := r.Text
	displayWidth := ansi.VisibleLength(rumorText) + 4 // add brackets + spaces
	padding := 0
	tw := termWidth
	if tw <= 0 {
		tw = 80
	}
	if displayWidth < tw {
		padding = (tw - displayWidth) / 2
	}

	wv(terminal, "\r\n"+strings.Repeat(" ", padding)+"|07[ |15"+rumorText+" |07]\r\n", outputMode)

	return currentUser, "", nil
}
