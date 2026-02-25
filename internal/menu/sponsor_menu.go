package menu

import (
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/message"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
)

// runSponsorMenu is the handler for RUN:SPONSORMENU.
//
// Triggered by "%" in the Messages Menu. Sysop, co-sysop, or the named area
// sponsor may enter; all others are silently refused.
//
// Flow:
//  1. Resolve the user's current message area.
//  2. Gate via CanAccessSponsorMenu.
//  3. Display SPONSORM.ANS header.
//  4. Single-key loop: E=Edit Area, Q=Quit.
func runSponsorMenu(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User,
	nodeNumber int, sessionStartTime time.Time, args string,
	outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return nil, "", nil
	}

	if e.MessageMgr == nil || currentUser.CurrentMessageAreaID == 0 {
		msg := "\r\n|03No message area selected.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	area, found := e.MessageMgr.GetAreaByID(currentUser.CurrentMessageAreaID)
	if !found {
		msg := "\r\n|03No message area selected.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	cfg := e.GetServerConfig()
	if !CanAccessSponsorMenu(currentUser, area, cfg) {
		log.Printf("INFO: Node %d: User %s denied sponsor menu for area %s",
			nodeNumber, currentUser.Handle, area.Tag)
		return currentUser, "", nil
	}

	log.Printf("INFO: Node %d: User %s entering sponsor menu for area %s",
		nodeNumber, currentUser.Handle, area.Tag)

	menuMnuPath := filepath.Join(e.MenuSetPath, "mnu")
	menuRec, loadErr := LoadMenu("SPONSORM", menuMnuPath)
	if loadErr != nil {
		log.Printf("WARN: Node %d: Failed to load SPONSORM.MNU: %v. Using fallback prompt.", nodeNumber, loadErr)
		menuRec = nil
	}

	// Issue clear screen as a separate write before the ANSI file — this is the
	// canonical pattern in the codebase and ensures the terminal processes the
	// clear before any subsequent display bytes arrive.
	if menuRec != nil && menuRec.GetClrScrBefore() {
		_ = terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	}
	if err := e.displayFile(terminal, "SPONSORM.ANS", outputMode); err != nil {
		log.Printf("WARN: Node %d: Failed to display SPONSORM.ANS: %v", nodeNumber, err)
	}

	ih := getSessionIH(s)

	for {
		if menuRec != nil && menuRec.GetUsePrompt() {
			if err := e.displayPrompt(terminal, menuRec, currentUser, userManager, nodeNumber, "SPONSORM", sessionStartTime, outputMode, ""); err != nil {
				log.Printf("WARN: Node %d: displayPrompt failed for SPONSORM: %v", nodeNumber, err)
			}
		} else {
			prompt := fmt.Sprintf("\r\n|15[|14%s|15] Sponsor: |11E|07=Edit Area  |11[|07/|11]|07=Prev/Next Area  |11Q|07=Quit: ", area.Tag)
			_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
		}

		key, err := ih.ReadKey()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return currentUser, "", err
		}

		switch key {
		case int('e'), int('E'):
			updated, next, runErr := runSponsorEditArea(e, s, terminal, userManager,
				currentUser, nodeNumber, sessionStartTime, args, outputMode, termWidth, termHeight)
			if runErr != nil {
				if errors.Is(runErr, io.EOF) {
					return nil, "LOGOFF", io.EOF
				}
				return updated, "", runErr
			}
			if next != "" {
				return updated, next, nil
			}
			currentUser = updated
			// Re-fetch area in case the edit updated its name/tag display.
			if a, ok := e.MessageMgr.GetAreaByID(currentUser.CurrentMessageAreaID); ok {
				area = a
			}
			// Redisplay the sponsor menu header on a clean screen after returning
			// from the edit area so the prompt doesn't appear on a dirty screen.
			if menuRec != nil && menuRec.GetClrScrBefore() {
				_ = terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
			}
			if err := e.displayFile(terminal, "SPONSORM.ANS", outputMode); err != nil {
				log.Printf("WARN: Node %d: Failed to display SPONSORM.ANS: %v", nodeNumber, err)
			}

		case int('['), int(']'):
			forward := key == int(']')
			sponsorAreas := getSponsorableAreasInConference(e, currentUser)
			if len(sponsorAreas) > 1 {
				currentIdx := -1
				for i, a := range sponsorAreas {
					if a.ID == currentUser.CurrentMessageAreaID {
						currentIdx = i
						break
					}
				}
				var newIdx int
				if currentIdx == -1 {
					newIdx = 0
				} else if forward {
					newIdx = (currentIdx + 1) % len(sponsorAreas)
				} else {
					newIdx = (currentIdx - 1 + len(sponsorAreas)) % len(sponsorAreas)
				}
				newArea := sponsorAreas[newIdx]
				currentUser.CurrentMessageAreaID = newArea.ID
				currentUser.CurrentMessageAreaTag = newArea.Tag
				if err := userManager.UpdateUser(currentUser); err != nil {
					log.Printf("ERROR: Node %d: Failed to save user after sponsor area nav: %v", nodeNumber, err)
				}
				area = newArea
				log.Printf("INFO: Node %d: User %s sponsor-navigated to area %d (%s)", nodeNumber, currentUser.Handle, newArea.ID, newArea.Tag)
				if menuRec != nil && menuRec.GetClrScrBefore() {
					_ = terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
				}
				if err := e.displayFile(terminal, "SPONSORM.ANS", outputMode); err != nil {
					log.Printf("WARN: Node %d: Failed to display SPONSORM.ANS: %v", nodeNumber, err)
				}
			}

		case int('q'), int('Q'), 27: // ESC — clear before fallback menu loads
			_ = terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
			return currentUser, "", nil
		}
	}
}

// allowAnonEqual reports whether two *bool AllowAnon values are equal (both nil, or both non-nil with same value).
func allowAnonEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// runSponsorEditArea is the handler for RUN:SPONSOREDITAREA.
//
// Sequential field editor for the current message area. All MessageArea fields
// are editable except ID (which is immutable).
//
// Key map: T=Tag N=Name D=Description R=ACS Read W=ACS Write S=Sponsor
//   M=Max Msgs G=Max Age A=Allow Anon L=Real Name Only J=Auto Join
//   C=Conf ID B=Base Path Y=Area Type E=Echo Tag O=Origin K=Network
//   Q=Save ESC=Cancel
//
// The Sponsor field is validated against the user database. Enter "-" to clear.
func runSponsorEditArea(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User,
	nodeNumber int, sessionStartTime time.Time, args string,
	outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return nil, "", nil
	}

	if e.MessageMgr == nil || currentUser.CurrentMessageAreaID == 0 {
		msg := "\r\n|03No message area selected.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	area, found := e.MessageMgr.GetAreaByID(currentUser.CurrentMessageAreaID)
	if !found {
		msg := "\r\n|03Area not found.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	cfg := e.GetServerConfig()
	if !CanAccessSponsorMenu(currentUser, area, cfg) {
		return currentUser, "", nil
	}

	// Work on a copy; apply to the live pointer only on save.
	edited := *area

	showAllowAnon := func() string {
		if edited.AllowAnon == nil {
			return "default"
		}
		if *edited.AllowAnon {
			return "yes"
		}
		return "no"
	}
	showFields := func() {
		var b strings.Builder
		b.WriteString("\r\n")
		b.WriteString(fmt.Sprintf("|15Edit Area: |14%s|07 (ID %d)\r\n", edited.Tag, edited.ID))
		b.WriteString("|08────────────────────────────────────────────────────\r\n")
		b.WriteString(fmt.Sprintf("|11T|07) Tag           : |15%s\r\n", edited.Tag))
		b.WriteString(fmt.Sprintf("|11N|07) Name          : |15%s\r\n", edited.Name))
		b.WriteString(fmt.Sprintf("|11D|07) Description   : |15%s\r\n", edited.Description))
		b.WriteString(fmt.Sprintf("|11R|07) ACS Read      : |15%s\r\n", edited.ACSRead))
		b.WriteString(fmt.Sprintf("|11W|07) ACS Write     : |15%s\r\n", edited.ACSWrite))
		b.WriteString(fmt.Sprintf("|11S|07) Sponsor       : |15%s\r\n", edited.Sponsor))
		b.WriteString(fmt.Sprintf("|11M|07) Max Messages  : |15%d\r\n", edited.MaxMessages))
		b.WriteString(fmt.Sprintf("|11G|07) Max Age (days): |15%d\r\n", edited.MaxAge))
		b.WriteString(fmt.Sprintf("|11A|07) Allow Anon    : |15%s\r\n", showAllowAnon()))
		b.WriteString(fmt.Sprintf("|11L|07) Real Name Only: |15%t\r\n", edited.RealNameOnly))
		b.WriteString(fmt.Sprintf("|11J|07) Auto Join     : |15%t\r\n", edited.AutoJoin))
		b.WriteString(fmt.Sprintf("|11C|07) Conference ID : |15%d\r\n", edited.ConferenceID))
		b.WriteString(fmt.Sprintf("|11B|07) Base Path     : |15%s\r\n", edited.BasePath))
		b.WriteString(fmt.Sprintf("|11Y|07) Area Type     : |15%s\r\n", edited.AreaType))
		b.WriteString(fmt.Sprintf("|11E|07) Echo Tag      : |15%s\r\n", edited.EchoTag))
		b.WriteString(fmt.Sprintf("|11O|07) Origin Addr   : |15%s\r\n", edited.OriginAddr))
		b.WriteString(fmt.Sprintf("|11K|07) Network       : |15%s\r\n", edited.Network))
		b.WriteString("|08────────────────────────────────────────────────────\r\n")
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(b.String())), outputMode)
	}

	_ = terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	showFields()

	ih := getSessionIH(s)
	dirty := false

	for {
		prompt := "|07Edit (|11T|07|11N|07|11D|07|11R|07|11W|07|11S|07|11M|07|11G|07|11A|07|11L|07|11J|07|11C|07|11B|07|11Y|07|11E|07|11O|07|11K|07)  |11Q|07=Save/Quit  |03ESC|07=Cancel: "
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

		key, err := ih.ReadKey()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return currentUser, "", err
		}

		_ = terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)

		switch key {
		case int('t'), int('T'):
			if currentUser.AccessLevel < cfg.CoSysOpLevel {
				msg := "|01Tag — sysop/co-sysop only.|07\r\n"
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(1 * time.Second)
				break
			}
			newVal := promptAreaField(s, terminal, outputMode, "Tag", edited.Tag, 32)
			if newVal != edited.Tag {
				dirty = true
				edited.Tag = newVal
			}
			showFields()

		case int('n'), int('N'):
			newVal := promptAreaField(s, terminal, outputMode, "Name", edited.Name, 60)
			if newVal != edited.Name {
				dirty = true
				edited.Name = newVal
			}
			showFields()

		case int('d'), int('D'):
			newVal := promptAreaField(s, terminal, outputMode,
				"Description", edited.Description, 80)
			if newVal != edited.Description {
				dirty = true
				edited.Description = newVal
			}
			showFields()

		case int('r'), int('R'):
			newVal := promptAreaField(s, terminal, outputMode,
				"ACS Read", edited.ACSRead, 40)
			if newVal != edited.ACSRead {
				dirty = true
				edited.ACSRead = newVal
			}
			showFields()

		case int('w'), int('W'):
			newVal := promptAreaField(s, terminal, outputMode,
				"ACS Write", edited.ACSWrite, 40)
			if newVal != edited.ACSWrite {
				dirty = true
				edited.ACSWrite = newVal
			}
			showFields()

		case int('s'), int('S'):
			prevSponsor := edited.Sponsor
			newHandle := promptAreaField(s, terminal, outputMode,
				"Sponsor handle (- to clear)", edited.Sponsor, 30)
			switch {
			case newHandle == "-":
				edited.Sponsor = ""
			case newHandle != "":
				if userManager != nil {
					if _, exists := userManager.GetUserByHandle(newHandle); !exists {
						msg := fmt.Sprintf("|01User '%s' not found — sponsor unchanged.|07\r\n", newHandle)
						_ = terminalio.WriteProcessedBytes(terminal,
							ansi.ReplacePipeCodes([]byte(msg)), outputMode)
						time.Sleep(1 * time.Second)
					} else {
						edited.Sponsor = newHandle
					}
				} else {
					edited.Sponsor = newHandle
				}
			}
			if edited.Sponsor != prevSponsor {
				dirty = true
			}
			showFields()

		case int('m'), int('M'):
			prevMax := edited.MaxMessages
			raw := promptAreaField(s, terminal, outputMode,
				"Max Messages (0=unlimited)", fmt.Sprintf("%d", edited.MaxMessages), 10)
			if raw != "" {
				var n int
				if _, scanErr := fmt.Sscanf(raw, "%d", &n); scanErr == nil && n >= 0 {
					edited.MaxMessages = n
				} else {
					msg := "|01Invalid number — unchanged.|07\r\n"
					_ = terminalio.WriteProcessedBytes(terminal,
						ansi.ReplacePipeCodes([]byte(msg)), outputMode)
					time.Sleep(1 * time.Second)
				}
			}
			if edited.MaxMessages != prevMax {
				dirty = true
			}
			showFields()

		case int('g'), int('G'):
			prevMaxAge := edited.MaxAge
			raw := promptAreaField(s, terminal, outputMode,
				"Max Age days (0=unlimited)", fmt.Sprintf("%d", edited.MaxAge), 6)
			if raw != "" {
				var n int
				if _, scanErr := fmt.Sscanf(raw, "%d", &n); scanErr == nil && n >= 0 {
					edited.MaxAge = n
				} else {
					msg := "|01Invalid number — unchanged.|07\r\n"
					_ = terminalio.WriteProcessedBytes(terminal,
						ansi.ReplacePipeCodes([]byte(msg)), outputMode)
					time.Sleep(1 * time.Second)
				}
			}
			if edited.MaxAge != prevMaxAge {
				dirty = true
			}
			showFields()

		case int('a'), int('A'):
			prevAllowAnon := edited.AllowAnon
			cur := "default"
			if edited.AllowAnon != nil {
				if *edited.AllowAnon {
					cur = "yes"
				} else {
					cur = "no"
				}
			}
			raw := promptAreaField(s, terminal, outputMode,
				"Allow Anonymous (yes/no/default)", cur, 10)
			raw = strings.ToLower(strings.TrimSpace(raw))
			if raw != "" {
				switch raw {
				case "y", "yes", "1", "true":
					t := true
					edited.AllowAnon = &t
				case "n", "no", "0", "false":
					f := false
					edited.AllowAnon = &f
				case "d", "default", "":
					edited.AllowAnon = nil
				default:
					msg := "|01Enter yes, no, or default.|07\r\n"
					_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
					time.Sleep(1 * time.Second)
				}
			}
			if !allowAnonEqual(prevAllowAnon, edited.AllowAnon) {
				dirty = true
			}
			showFields()

		case int('l'), int('L'):
			prevRealName := edited.RealNameOnly
			cur := "no"
			if edited.RealNameOnly {
				cur = "yes"
			}
			raw := promptAreaField(s, terminal, outputMode,
				"Real Name Only (yes/no)", cur, 5)
			raw = strings.ToLower(strings.TrimSpace(raw))
			if raw != "" {
				edited.RealNameOnly = strings.HasPrefix(raw, "y") || raw == "1" || raw == "true"
			}
			if edited.RealNameOnly != prevRealName {
				dirty = true
			}
			showFields()

		case int('j'), int('J'):
			prevAutoJoin := edited.AutoJoin
			cur := "no"
			if edited.AutoJoin {
				cur = "yes"
			}
			raw := promptAreaField(s, terminal, outputMode,
				"Auto Join (yes/no)", cur, 5)
			raw = strings.ToLower(strings.TrimSpace(raw))
			if raw != "" {
				edited.AutoJoin = strings.HasPrefix(raw, "y") || raw == "1" || raw == "true"
			}
			if edited.AutoJoin != prevAutoJoin {
				dirty = true
			}
			showFields()

		case int('c'), int('C'):
			prevConfID := edited.ConferenceID
			raw := promptAreaField(s, terminal, outputMode,
				"Conference ID (0=ungrouped)", fmt.Sprintf("%d", edited.ConferenceID), 6)
			if raw != "" {
				var n int
				if _, scanErr := fmt.Sscanf(raw, "%d", &n); scanErr == nil && n >= 0 {
					edited.ConferenceID = n
				} else {
					msg := "|01Invalid number — unchanged.|07\r\n"
					_ = terminalio.WriteProcessedBytes(terminal,
						ansi.ReplacePipeCodes([]byte(msg)), outputMode)
					time.Sleep(1 * time.Second)
				}
			}
			if edited.ConferenceID != prevConfID {
				dirty = true
			}
			showFields()

		case int('b'), int('B'):
			if currentUser.AccessLevel < cfg.CoSysOpLevel {
				msg := "|01Base Path — sysop/co-sysop only.|07\r\n"
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(1 * time.Second)
				break
			}
			newVal := promptAreaField(s, terminal, outputMode,
				"Base Path", edited.BasePath, 80)
			if newVal != edited.BasePath {
				dirty = true
				edited.BasePath = newVal
			}
			showFields()

		case int('y'), int('Y'):
			if currentUser.AccessLevel < cfg.CoSysOpLevel {
				msg := "|01Area Type — sysop/co-sysop only.|07\r\n"
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(1 * time.Second)
				break
			}
			newVal := promptAreaField(s, terminal, outputMode,
				"Area Type (local/echomail/netmail)", edited.AreaType, 16)
			if newVal != edited.AreaType {
				dirty = true
				edited.AreaType = newVal
			}
			showFields()

		case int('e'), int('E'):
			if currentUser.AccessLevel < cfg.CoSysOpLevel {
				msg := "|01Echo Tag — sysop/co-sysop only.|07\r\n"
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(1 * time.Second)
				break
			}
			newVal := promptAreaField(s, terminal, outputMode,
				"Echo Tag", edited.EchoTag, 32)
			if newVal != edited.EchoTag {
				dirty = true
				edited.EchoTag = newVal
			}
			showFields()

		case int('o'), int('O'):
			if currentUser.AccessLevel < cfg.CoSysOpLevel {
				msg := "|01Origin Address — sysop/co-sysop only.|07\r\n"
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(1 * time.Second)
				break
			}
			newVal := promptAreaField(s, terminal, outputMode,
				"Origin Address", edited.OriginAddr, 32)
			if newVal != edited.OriginAddr {
				dirty = true
				edited.OriginAddr = newVal
			}
			showFields()

		case int('k'), int('K'):
			if currentUser.AccessLevel < cfg.CoSysOpLevel {
				msg := "|01Network — sysop/co-sysop only.|07\r\n"
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(1 * time.Second)
				break
			}
			newVal := promptAreaField(s, terminal, outputMode,
				"Network", edited.Network, 32)
			if newVal != edited.Network {
				dirty = true
				edited.Network = newVal
			}
			showFields()

		case int('q'), int('Q'): // Q = save and quit (no-op if nothing changed)
			if !dirty {
				return currentUser, "", nil
			}
			// Capture previous area so we can roll back in-memory state if SaveAreas fails.
			prevArea, _ := e.MessageMgr.GetAreaByID(edited.ID)
			if updateErr := e.MessageMgr.UpdateAreaByID(edited.ID, edited); updateErr != nil {
				log.Printf("ERROR: Node %d: Failed to update area: %v", nodeNumber, updateErr)
				msg := "|01Error updating area — changes may be lost.|07\r\n"
				_ = terminalio.WriteProcessedBytes(terminal,
					ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(2 * time.Second)
				return currentUser, "", nil
			}
			if saveErr := e.MessageMgr.SaveAreas(); saveErr != nil {
				log.Printf("ERROR: Node %d: Failed to save areas: %v", nodeNumber, saveErr)
				msg := "|01Error saving area — changes may be lost.|07\r\n"
				_ = terminalio.WriteProcessedBytes(terminal,
					ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(2 * time.Second)
				// Roll back in-memory state so edited state is not left applied without disk persist.
				if prevArea != nil {
					_ = e.MessageMgr.UpdateAreaByID(edited.ID, *prevArea)
				}
			} else {
				log.Printf("INFO: Node %d: User %s saved area %s", nodeNumber, currentUser.Handle, edited.Tag)
				msg := fmt.Sprintf("|02Area |14%s|02 saved.|07\r\n", edited.Tag)
				_ = terminalio.WriteProcessedBytes(terminal,
					ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(500 * time.Millisecond)
				// Update user's cached tag if it changed
				if currentUser.CurrentMessageAreaID == edited.ID {
					currentUser.CurrentMessageAreaTag = edited.Tag
				}
			}
			return currentUser, "", nil

		case 27: // ESC = discard and quit
			msg := "|03Changes discarded.|07\r\n"
			_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(500 * time.Millisecond)
			return currentUser, "", nil
		}
	}
}

// getSponsorableAreasInConference returns all areas in the user's current
// conference where the user has sponsor menu access (sysop, co-sysop, or
// named sponsor), sorted by ID.
func getSponsorableAreasInConference(e *MenuExecutor, currentUser *user.User) []*message.MessageArea {
	cfg := e.GetServerConfig()
	var result []*message.MessageArea
	for _, area := range e.MessageMgr.ListAreas() {
		if area.ConferenceID != currentUser.CurrentMsgConferenceID {
			continue
		}
		if !CanAccessSponsorMenu(currentUser, area, cfg) {
			continue
		}
		result = append(result, area)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// promptAreaField prints a prompt, reads a line, and returns the new value.
// If the user presses Enter with no input, the original value is returned
// unchanged.
func promptAreaField(s ssh.Session, terminal *term.Terminal,
	outputMode ansi.OutputMode, label, current string, maxLen int) string {

	prompt := fmt.Sprintf("|15%s|07 [|11%s|07]: ", label, current)
	_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	input, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		return current
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return current
	}
	if runes := []rune(input); len(runes) > maxLen {
		input = string(runes[:maxLen])
	}
	return input
}
