package menu

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/robbiew/vision3/internal/ansi"
	"github.com/robbiew/vision3/internal/message"
	"github.com/robbiew/vision3/internal/terminalio"
	"github.com/robbiew/vision3/internal/user"
	"golang.org/x/term"
)

// runChangeMsgConference lists accessible conferences and lets the user select one.
// On selection, updates the user's current conference and sets the first accessible area.
func runChangeMsgConference(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running CHANGEMSGCONF", nodeNumber)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in to change conferences.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	if e.ConferenceMgr == nil {
		msg := "\r\n|01Error: No conferences configured.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	// Display conference list
	if err := displayConferenceList(e, s, terminal, currentUser, outputMode, nodeNumber, sessionStartTime); err != nil {
		return currentUser, "", err
	}

	// Prompt for selection
	prompt := e.LoadedStrings.ConfPrompt
	if prompt == "" {
		prompt = "\r\n|07Conference |08[|15#|08/|15Tag|07, |15Q|08=|15Quit|08]|07: |15"
	}
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	inputLine, err := terminal.ReadLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return currentUser, "", err
	}

	inputClean := strings.TrimSpace(inputLine)
	upperInput := strings.ToUpper(inputClean)

	if upperInput == "" || upperInput == "Q" {
		terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
		return currentUser, "", nil
	}

	// Try to match by ID (numeric) then by tag
	var confID int
	var matched bool

	if id, parseErr := strconv.Atoi(inputClean); parseErr == nil {
		if conf, found := e.ConferenceMgr.GetByID(id); found {
			if checkACS(conf.ACS, currentUser, s, terminal, sessionStartTime) {
				confID = conf.ID
				matched = true
			}
		}
	}

	if !matched {
		if conf, found := e.ConferenceMgr.GetByTag(upperInput); found {
			if checkACS(conf.ACS, currentUser, s, terminal, sessionStartTime) {
				confID = conf.ID
				matched = true
			}
		}
	}

	if !matched {
		msg := fmt.Sprintf("\r\n|01Error: Conference '%s' not found or access denied.|07\r\n", inputClean)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	// Update user conference
	e.setUserMsgConference(currentUser, confID)

	// Find first accessible area in new conference
	firstArea := findFirstAccessibleAreaInConference(e, s, terminal, currentUser, confID, sessionStartTime)
	if firstArea != nil {
		currentUser.CurrentMessageAreaID = firstArea.ID
		currentUser.CurrentMessageAreaTag = firstArea.Tag
	} else {
		currentUser.CurrentMessageAreaID = 0
		currentUser.CurrentMessageAreaTag = ""
	}

	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save user after conference change: %v", nodeNumber, err)
	}

	// Display confirmation
	conf, _ := e.ConferenceMgr.GetByID(confID)
	joinedMsg := e.LoadedStrings.JoinedMsgConf
	if joinedMsg == "" {
		joinedMsg = "\r\n|07(|15^CN|07) |15Conference Joined!|07\r\n"
	}
	joinedMsg = strings.ReplaceAll(joinedMsg, "^CN", conf.Name)
	joinedMsg = strings.ReplaceAll(joinedMsg, "^CT", conf.Tag)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(joinedMsg)), outputMode)
	time.Sleep(1 * time.Second)

	log.Printf("INFO: Node %d: User %s changed to conference %d (%s), area: %s",
		nodeNumber, currentUser.Handle, confID, conf.Tag, currentUser.CurrentMessageAreaTag)

	return currentUser, "", nil
}

// runNextMsgArea moves to the next accessible message area within the current conference.
func runNextMsgArea(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	return navigateMsgArea(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, outputMode, true)
}

// runPrevMsgArea moves to the previous accessible message area within the current conference.
func runPrevMsgArea(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	return navigateMsgArea(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, outputMode, false)
}

// navigateMsgArea handles forward/backward area navigation within the current conference.
func navigateMsgArea(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, outputMode ansi.OutputMode, forward bool) (*user.User, string, error) {
	direction := "NEXTMSGAREA"
	if !forward {
		direction = "PREVMSGAREA"
	}
	log.Printf("DEBUG: Node %d: Running %s", nodeNumber, direction)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Get accessible areas in current conference
	accessibleAreas := getAccessibleAreasInConference(e, s, terminal, currentUser, currentUser.CurrentMsgConferenceID, sessionStartTime)

	if len(accessibleAreas) == 0 {
		msg := "\r\n|01No accessible message areas in this conference.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	// Find current position
	currentIdx := -1
	for i, area := range accessibleAreas {
		if area.ID == currentUser.CurrentMessageAreaID {
			currentIdx = i
			break
		}
	}

	// Calculate new index with wrapping
	var newIdx int
	if currentIdx == -1 {
		newIdx = 0
	} else if forward {
		newIdx = (currentIdx + 1) % len(accessibleAreas)
	} else {
		newIdx = (currentIdx - 1 + len(accessibleAreas)) % len(accessibleAreas)
	}

	newArea := accessibleAreas[newIdx]
	currentUser.CurrentMessageAreaID = newArea.ID
	currentUser.CurrentMessageAreaTag = newArea.Tag

	if err := userManager.UpdateUser(currentUser); err != nil {
		log.Printf("ERROR: Node %d: Failed to save user after area change: %v", nodeNumber, err)
	}

	msg := fmt.Sprintf("\r\n|07Current area: |15%s|07 (%s)\r\n", newArea.Name, newArea.Tag)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)

	log.Printf("INFO: Node %d: User %s navigated to area %d (%s)", nodeNumber, currentUser.Handle, newArea.ID, newArea.Tag)

	return currentUser, "", nil
}

// displayConferenceList renders the conference list using templates.
func displayConferenceList(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, currentUser *user.User, outputMode ansi.OutputMode, nodeNumber int, sessionStartTime time.Time) error {
	templateDir := filepath.Join(e.MenuSetPath, "templates")

	topBytes, errTop := os.ReadFile(filepath.Join(templateDir, "MSGCONF.TOP"))
	midBytes, errMid := os.ReadFile(filepath.Join(templateDir, "MSGCONF.MID"))
	botBytes, errBot := os.ReadFile(filepath.Join(templateDir, "MSGCONF.BOT"))

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load MSGCONF templates: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading conference templates.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		return fmt.Errorf("failed loading MSGCONF templates")
	}

	processedTop := ansi.ReplacePipeCodes(topBytes)
	processedMid := string(ansi.ReplacePipeCodes(midBytes))
	processedBot := ansi.ReplacePipeCodes(botBytes)

	conferences := e.ConferenceMgr.ListConferences()

	var buf bytes.Buffer
	buf.Write(processedTop)

	displayed := 0
	for _, conf := range conferences {
		if !checkACS(conf.ACS, currentUser, s, terminal, sessionStartTime) {
			continue
		}
		line := processedMid
		line = strings.ReplaceAll(line, "^CI", padRight(strconv.Itoa(conf.ID), 3))
		line = strings.ReplaceAll(line, "^CN", padRight(truncateStr(conf.Name, 33), 33))
		line = strings.ReplaceAll(line, "^CD", truncateStr(conf.Description, 40))
		buf.WriteString(line)
		displayed++
	}

	if displayed == 0 {
		buf.WriteString("\r\n|07   No accessible conferences found.   \r\n")
	}

	buf.Write(processedBot)

	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	return terminalio.WriteProcessedBytes(terminal, buf.Bytes(), outputMode)
}

// displayMessageAreaListFiltered is like displayMessageAreaList but filters to a single conference.
// If filterConfID is -1, all conferences are shown (same as unfiltered behavior).
func displayMessageAreaListFiltered(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, currentUser *user.User, outputMode ansi.OutputMode, nodeNumber int, sessionStartTime time.Time, filterConfID int) error {
	log.Printf("DEBUG: Node %d: Displaying message area list (filtered, confID=%d)", nodeNumber, filterConfID)

	templateDir := filepath.Join(e.MenuSetPath, "templates")
	topTemplateBytes, errTop := os.ReadFile(filepath.Join(templateDir, "MSGAREA.TOP"))
	midTemplateBytes, errMid := os.ReadFile(filepath.Join(templateDir, "MSGAREA.MID"))
	botTemplateBytes, errBot := os.ReadFile(filepath.Join(templateDir, "MSGAREA.BOT"))

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load MSGAREA template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading Message Area screen templates.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return fmt.Errorf("failed loading MSGAREA templates")
	}

	confHdrBytes, errConf := os.ReadFile(filepath.Join(templateDir, "MSGCONF.HDR"))
	confHdrTemplate := ""
	if errConf == nil {
		confHdrTemplate = string(ansi.ReplacePipeCodes(confHdrBytes))
	}

	processedTopTemplate := ansi.ReplacePipeCodes(topTemplateBytes)
	processedMidTemplate := string(ansi.ReplacePipeCodes(midTemplateBytes))
	processedBotTemplate := ansi.ReplacePipeCodes(botTemplateBytes)

	areas := e.MessageMgr.ListAreas()

	// Build conference groups, applying conference filter
	groups := make(map[int][]*message.MessageArea)
	confIDs := make(map[int]bool)
	for _, area := range areas {
		if filterConfID >= 0 && area.ConferenceID != filterConfID {
			continue
		}
		if !checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
			continue
		}
		groups[area.ConferenceID] = append(groups[area.ConferenceID], area)
		confIDs[area.ConferenceID] = true
	}

	var sortedConfIDs []int
	if e.ConferenceMgr != nil {
		sortedConfIDs = e.ConferenceMgr.GetSortedConferenceIDs(confIDs)
	} else {
		for cid := range confIDs {
			sortedConfIDs = append(sortedConfIDs, cid)
		}
		sort.Ints(sortedConfIDs)
	}

	var outputBuffer bytes.Buffer
	outputBuffer.Write(processedTopTemplate)

	areasDisplayed := 0
	for _, cid := range sortedConfIDs {
		areasInConf := groups[cid]
		if len(areasInConf) == 0 {
			continue
		}

		if cid != 0 && e.ConferenceMgr != nil {
			conf, found := e.ConferenceMgr.GetByID(cid)
			if found && !checkACS(conf.ACS, currentUser, s, terminal, sessionStartTime) {
				continue
			}
			// Only show conference header when not filtering to a single conference
			if found && confHdrTemplate != "" && filterConfID < 0 {
				hdr := confHdrTemplate
				hdr = strings.ReplaceAll(hdr, "^CN", conf.Name)
				hdr = strings.ReplaceAll(hdr, "^CT", conf.Tag)
				hdr = strings.ReplaceAll(hdr, "^CD", conf.Description)
				hdr = strings.ReplaceAll(hdr, "^CI", strconv.Itoa(conf.ID))
				outputBuffer.WriteString(hdr)
			}
		}

		for _, area := range areasInConf {
			line := processedMidTemplate
			line = strings.ReplaceAll(line, "^ID", padRight(strconv.Itoa(area.ID), 3))
			line = strings.ReplaceAll(line, "^TAG", padRight(area.Tag, 12))
			line = strings.ReplaceAll(line, "^NA", padRight(truncateStr(area.Name, 30), 30))
			line = strings.ReplaceAll(line, "^DS", area.AreaType)
			outputBuffer.WriteString(line)
			areasDisplayed++
		}
	}

	if areasDisplayed == 0 {
		outputBuffer.WriteString("\r\n|07   No accessible message areas found.   \r\n")
	}

	outputBuffer.Write(processedBotTemplate)

	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	return terminalio.WriteProcessedBytes(terminal, outputBuffer.Bytes(), outputMode)
}

// padRight pads s with spaces on the right to the given width. If s is already wider, it is returned as-is.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// truncateStr truncates s to maxLen characters, appending ".." if truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 2 {
		return s[:maxLen]
	}
	return s[:maxLen-2] + ".."
}

// findFirstAccessibleAreaInConference returns the first area the user can read in the given conference.
func findFirstAccessibleAreaInConference(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, currentUser *user.User, conferenceID int, sessionStartTime time.Time) *message.MessageArea {
	areas := getAccessibleAreasInConference(e, s, terminal, currentUser, conferenceID, sessionStartTime)
	if len(areas) > 0 {
		return areas[0]
	}
	return nil
}

// getAccessibleAreasInConference returns all areas in a conference the user can read, sorted by ID.
func getAccessibleAreasInConference(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, currentUser *user.User, conferenceID int, sessionStartTime time.Time) []*message.MessageArea {
	allAreas := e.MessageMgr.ListAreas()
	var result []*message.MessageArea
	for _, area := range allAreas {
		if area.ConferenceID != conferenceID {
			continue
		}
		if !checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
			continue
		}
		result = append(result, area)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}
