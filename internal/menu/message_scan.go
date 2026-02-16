package menu

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/message"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// ScanConfig holds the scan parameters configured by GetScanType.
type ScanConfig struct {
	ScanDate       int64 // -1 = new only, 0 = all, >0 = unix timestamp
	SearchTo       string
	SearchFrom     string
	RangeStart     int
	RangeEnd       int
	UpdatePointers bool
	WhichAreas     int // 1=tagged/marked, 2=all in conference, 3=current only
	Aborted        bool
}

// Per-area lightbar options (Pascal's 6-option bar for multi-area scan)
var scanAreaOptions = []MsgLightbarOption{
	{Label: " Read ", HotKey: 'R'},
	{Label: " Post ", HotKey: 'P'},
	{Label: " Jump ", HotKey: 'J'},
	{Label: " Skip ", HotKey: 'S'},
	{Label: " Quit ", HotKey: 'Q'},
	{Label: " NonStop ", HotKey: 'N'},
}

// runGetScanType displays the Pascal-style scan configuration menu.
func runGetScanType(reader *bufio.Reader, e *MenuExecutor, terminal *term.Terminal,
	outputMode ansi.OutputMode, numMsgs int, currentOnly bool,
	hiColor int, loColor int) (*ScanConfig, error) {

	cfg := &ScanConfig{
		ScanDate:       -1,   // Default: new messages only
		UpdatePointers: true, // Default: update pointers
		WhichAreas:     1,    // Default: tagged areas
	}
	if currentOnly {
		cfg.WhichAreas = 3
	}

	showMenu := func() {
		terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)

		// Display ANSI header (Vision/2 style - 4 rows tall)
		ansPath := "menus/v3/ansi/NSCANHDR.ANS"
		headerContent, ansErr := ansi.GetAnsiFileContent(ansPath)
		if ansErr == nil {
			terminalio.WriteProcessedBytes(terminal, headerContent, outputMode)
			// Position cursor on line 5 (after 4-row header)
			terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
		}

		// Date - Brackets: Dark grey (|08), Hotkeys: Bright cyan (|11), Labels: Dark cyan (|03), Values: Bright blue (|09)
		dateStr := "All New Messages"
		if cfg.ScanDate == 0 {
			dateStr = "ALL Messages"
		} else if cfg.ScanDate > 0 {
			dateStr = fmt.Sprintf("From: %s", time.Unix(cfg.ScanDate, 0).Format("01/02/06"))
		}
		line := fmt.Sprintf(e.LoadedStrings.ScanDateLine, dateStr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		// To
		toStr := "N/A"
		if cfg.SearchTo != "" {
			toStr = fmt.Sprintf("Search For %s", cfg.SearchTo)
		}
		line = fmt.Sprintf(e.LoadedStrings.ScanToLine, toStr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		// From
		fromStr := "N/A"
		if cfg.SearchFrom != "" {
			fromStr = fmt.Sprintf("Search For %s", cfg.SearchFrom)
		}
		line = fmt.Sprintf(e.LoadedStrings.ScanFromLine, fromStr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		// Range
		rangeStr := "All"
		if cfg.RangeStart > 0 && cfg.RangeEnd > 0 {
			rangeStr = fmt.Sprintf("%d-%d", cfg.RangeStart, cfg.RangeEnd)
		}
		line = fmt.Sprintf(e.LoadedStrings.ScanRangeLine, rangeStr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		// Update Pointers
		upStr := "Yes"
		if !cfg.UpdatePointers {
			upStr = "No"
		}
		line = fmt.Sprintf(e.LoadedStrings.ScanUpdateLine, upStr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		// Which Areas
		var whichStr string
		switch cfg.WhichAreas {
		case 1:
			whichStr = "All Tagged Areas"
		case 2:
			whichStr = "ALL Areas in Conference"
		case 3:
			whichStr = "Current Area Only"
		}
		line = fmt.Sprintf(e.LoadedStrings.ScanWhichLine, whichStr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		line = e.LoadedStrings.ScanAbortLine
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		// Prompt - "Selection;" Dark Cyan (|03), "(Cr" Bright Cyan (|11), "/" Bright Magenta (|13), "Scan) :" Bright Cyan (|11)
		prompt := e.LoadedStrings.ScanSelectionPrompt
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
	}

	for {
		showMenu()

		key, err := readSingleKey(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, io.EOF
			}
			return nil, err
		}

		upper := unicode.ToUpper(key)

		switch upper {
		case '\r', '\n':
			// Enter = start scanning
			return cfg, nil

		case 'D': // Date
			prompt := e.LoadedStrings.ScanDatePrompt
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
			input, readErr := readLineInput(reader, terminal, outputMode, 10)
			if readErr != nil {
				continue
			}
			if input != "" {
				switch unicode.ToUpper(rune(input[0])) {
				case 'A':
					cfg.ScanDate = 0
				case 'N':
					cfg.ScanDate = -1
				default:
					// Try to parse as date
					t, tErr := time.Parse("01/02/06", input)
					if tErr == nil {
						cfg.ScanDate = t.Unix()
					}
				}
			}

		case 'T': // To
			prompt := e.LoadedStrings.ScanToPrompt
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
			input, readErr := readLineInput(reader, terminal, outputMode, 30)
			if readErr != nil {
				continue
			}
			cfg.SearchTo = input

		case 'F': // From
			prompt := e.LoadedStrings.ScanFromPrompt
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
			input, readErr := readLineInput(reader, terminal, outputMode, 30)
			if readErr != nil {
				continue
			}
			cfg.SearchFrom = input

		case 'R': // Range
			prompt := fmt.Sprintf(e.LoadedStrings.ScanRangeStartPrompt, numMsgs)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
			startInput, readErr := readLineInput(reader, terminal, outputMode, 6)
			if readErr != nil {
				continue
			}
			startNum, _ := strconv.Atoi(startInput)
			if startNum < 1 || startNum > numMsgs {
				cfg.RangeStart = 0
				cfg.RangeEnd = 0
				continue
			}
			cfg.RangeStart = startNum

			prompt = fmt.Sprintf(e.LoadedStrings.ScanRangeEndPrompt, startNum, numMsgs)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
			endInput, readErr := readLineInput(reader, terminal, outputMode, 6)
			if readErr != nil {
				continue
			}
			endNum, _ := strconv.Atoi(endInput)
			if endNum < startNum || endNum > numMsgs {
				cfg.RangeEnd = 0
				continue
			}
			cfg.RangeEnd = endNum

		case 'U': // Update pointers
			cfg.UpdatePointers = !cfg.UpdatePointers

		case 'S': // Scan which areas
			prompt := e.LoadedStrings.ScanWhichPrompt
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
			aKey, aErr := readSingleKey(reader)
			if aErr != nil {
				continue
			}
			switch unicode.ToUpper(aKey) {
			case 'M':
				cfg.WhichAreas = 1
			case 'A':
				cfg.WhichAreas = 2
			case 'C':
				cfg.WhichAreas = 3
			}

		case 'A', 'Q': // Abort
			cfg.Aborted = true
			return cfg, nil
		}
	}
}

// runNewScanAll implements the multi-area newscan flow matching Pascal's NewScanAll.
func runNewScanAll(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, outputMode ansi.OutputMode,
	currentOnly bool, termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.ScanLoginRequired)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	reader := bufio.NewReader(s)

	// Get total message count for current area (for range display in setup)
	currentAreaID := currentUser.CurrentMessageAreaID
	numMsgs := 0
	if currentAreaID > 0 {
		cnt, _ := e.MessageMgr.GetMessageCountForArea(currentAreaID)
		numMsgs = cnt
	}

	hiColor := e.Theme.YesNoHighlightColor
	loColor := e.Theme.YesNoRegularColor

	// Show scan setup menu
	scanCfg, err := runGetScanType(reader, e, terminal, outputMode, numMsgs, currentOnly, hiColor, loColor)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", err
	}
	if scanCfg.Aborted {
		return nil, "", nil
	}

	// Display "Scanning Messages..."
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.ScanHeader)), outputMode)

	nonStop := false
	quitNewScan := false

	// If current area only, scan just that area
	if scanCfg.WhichAreas == 3 {
		if currentAreaID <= 0 {
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.ScanNoAreaSelected)), outputMode)
			time.Sleep(1 * time.Second)
			return nil, "", nil
		}

		totalCount, _ := e.MessageMgr.GetMessageCountForArea(currentAreaID)
		if totalCount == 0 {
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.ScanNoMessages)), outputMode)
			time.Sleep(1 * time.Second)
			return nil, "", nil
		}

		startMsg := determineStartMessage(e, scanCfg, currentAreaID, currentUser.Handle, totalCount)

		// Get terminal dimensions from user preferences
		tw := currentUser.ScreenWidth
		if tw == 0 {
			tw = 80
		}
		th := currentUser.ScreenHeight
		if th == 0 {
			th = 24
		}

		_, action, readErr := runMessageReader(e, s, terminal, userManager, currentUser,
			nodeNumber, sessionStartTime, outputMode, startMsg, totalCount, true, tw, th)
		if readErr != nil || action == "LOGOFF" {
			return nil, "LOGOFF", readErr
		}
		return nil, "", nil
	}

	// Multi-area scan: iterate through accessible areas
	allAreas := e.MessageMgr.ListAreas()

	// If tagged areas mode, check if user has any tagged areas
	if scanCfg.WhichAreas == 1 && (currentUser.TaggedMessageAreaIDs == nil || len(currentUser.TaggedMessageAreaIDs) == 0) {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.ScanNoTaggedAreas)), outputMode)
		time.Sleep(2 * time.Second)
		return nil, "", nil
	}

	// Create tagged area map for quick lookup
	taggedMap := make(map[int]bool)
	if scanCfg.WhichAreas == 1 && currentUser.TaggedMessageAreaIDs != nil {
		for _, areaID := range currentUser.TaggedMessageAreaIDs {
			taggedMap[areaID] = true
		}
	}

	for _, area := range allAreas {
		if quitNewScan {
			break
		}

		// Check ACS
		if !checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
			continue
		}

		// Filter by tagged areas if WhichAreas == 1
		if scanCfg.WhichAreas == 1 && !taggedMap[area.ID] {
			continue
		}

		// Filter by conference if WhichAreas == 2 (all in conference)
		if scanCfg.WhichAreas == 2 && area.ConferenceID != currentUser.CurrentMsgConferenceID {
			continue
		}

		totalCount, countErr := e.MessageMgr.GetMessageCountForArea(area.ID)
		if countErr != nil || totalCount == 0 {
			continue
		}

		startMsg := determineStartMessage(e, scanCfg, area.ID, currentUser.Handle, totalCount)
		if startMsg > totalCount {
			continue // No messages to show for this area
		}

		// Set current area
		currentUser.CurrentMessageAreaID = area.ID
		currentUser.CurrentMessageAreaTag = area.Tag
		e.setUserMsgConference(currentUser, area.ConferenceID)

		// Display "Current (AreaName)..."
		boardMsg := fmt.Sprintf(e.LoadedStrings.ScanAreaProgress,
			area.Tag, startMsg, totalCount)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(boardMsg)), outputMode)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)

		if !nonStop {
			// Show per-area lightbar: Read/Post/Jump/Skip/Quit/NonStop
			scanSuffix := fmt.Sprintf(" [%d/%d]", startMsg, totalCount)
			selectedKey, lbErr := runMsgLightbar(reader, terminal, scanAreaOptions, outputMode,
				hiColor, loColor, scanSuffix, 0, false, 0)
			if lbErr != nil {
				if errors.Is(lbErr, io.EOF) {
					return nil, "LOGOFF", io.EOF
				}
				break
			}

			switch selectedKey {
			case 'R': // Read this area
				// Fall through to call runMessageReader below
			case 'P': // Post
				_, _, _ = runComposeMessage(e, s, terminal, userManager, currentUser, nodeNumber,
				sessionStartTime, "", outputMode, termWidth, termHeight)
				continue
			case 'J': // Jump to message #
				handleJump(reader, terminal, outputMode, &startMsg, totalCount, e.LoadedStrings.MsgJumpPrompt, e.LoadedStrings.MsgInvalidMsgNum)
			case 'S': // Skip this area
				continue
			case 'Q': // Quit scanning
				quitNewScan = true
				continue
			case 'N': // NonStop
				nonStop = true
			}
		}

		if quitNewScan {
			break
		}

		// Get terminal dimensions from user preferences
		tw := currentUser.ScreenWidth
		if tw == 0 {
			tw = 80
		}
		th := currentUser.ScreenHeight
		if th == 0 {
			th = 24
		}

		// Read messages in this area
		_, action, readErr := runMessageReader(e, s, terminal, userManager, currentUser,
			nodeNumber, sessionStartTime, outputMode, startMsg, totalCount, true, tw, th)
		if readErr != nil || action == "LOGOFF" {
			return nil, "LOGOFF", readErr
		}
		if action == "QUIT_NEWSCAN" {
			quitNewScan = true
		}

		// Update pointers
		if scanCfg.UpdatePointers {
			if saveErr := userManager.UpdateUser(currentUser); saveErr != nil {
				log.Printf("ERROR: Node %d: Failed to save user data during newscan: %v", nodeNumber, saveErr)
			}
		}
	}

	// Newscan complete
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.ScanComplete)), outputMode)
	time.Sleep(1 * time.Second)

	return nil, "", nil
}

// determineStartMessage calculates the starting message number based on scan config.
func determineStartMessage(e *MenuExecutor, cfg *ScanConfig, areaID int, username string, totalCount int) int {
	if cfg.RangeStart > 0 {
		return cfg.RangeStart
	}

	if cfg.ScanDate == 0 {
		// All messages
		return 1
	}

	if cfg.ScanDate == -1 {
		// New messages only
		newCount, err := e.MessageMgr.GetNewMessageCount(areaID, username)
		if err != nil || newCount == 0 {
			return totalCount + 1 // No new messages, skip area
		}
		return totalCount - newCount + 1
	}

	// Date-based scan: start from message 1, filtering will happen in reader
	return 1
}

// areaListItem represents an item in the newscan config list (area or conference header)
type areaListItem struct {
	area     *message.MessageArea
	confName string
	isHeader bool
}

// runNewscanConfig allows users to tag/untag message areas for their personal newscan.
// Similar to retrograde's subscription system but using Vision3 styling.
func runNewscanConfig(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.ScanConfigLoginRequired)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	reader := bufio.NewReader(s)

	// Get all accessible message areas
	allAreas := e.MessageMgr.ListAreas()
	if len(allAreas) == 0 {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.ScanNoAreasAvailable)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Sort areas by conference ID to prevent duplicate conference headers
	sort.Slice(allAreas, func(i, j int) bool {
		if allAreas[i].ConferenceID != allAreas[j].ConferenceID {
			return allAreas[i].ConferenceID < allAreas[j].ConferenceID
		}
		return allAreas[i].ID < allAreas[j].ID
	})

	// Build accessible areas list
	var accessibleAreas []areaListItem

	// Group areas by conference
	confMap := make(map[int]string)
	if e.ConferenceMgr != nil {
		for _, conf := range e.ConferenceMgr.ListConferences() {
			confMap[conf.ID] = conf.Name
		}
	}

	// Build area list with conference headers
	currentConfID := -1
	for _, area := range allAreas {
		// Check read access
		if !checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
			continue
		}

		// Add conference header if changed
		if area.ConferenceID != currentConfID {
			currentConfID = area.ConferenceID
			confName := confMap[area.ConferenceID]
			if confName == "" {
				confName = "General"
			}
			accessibleAreas = append(accessibleAreas, areaListItem{
				confName: confName,
				isHeader: true,
			})
		}

		accessibleAreas = append(accessibleAreas, areaListItem{
			area:     area,
			confName: confMap[area.ConferenceID],
			isHeader: false,
		})
	}

	if len(accessibleAreas) == 0 {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.ScanNoAccessibleAreas)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Create tagged map for quick lookups
	taggedMap := make(map[int]bool)
	if currentUser.TaggedMessageAreaIDs != nil {
		for _, areaID := range currentUser.TaggedMessageAreaIDs {
			taggedMap[areaID] = true
		}
	}

	// UI state
	currentIdx := 0
	// Skip to first non-header
	for currentIdx < len(accessibleAreas) && accessibleAreas[currentIdx].isHeader {
		currentIdx++
	}

	// Get terminal dimensions (default 24x80)
	termHeight = currentUser.ScreenHeight
	if termHeight == 0 {
		termHeight = 24
	}
	termWidth = currentUser.ScreenWidth
	if termWidth == 0 {
		termWidth = 80
	}

	// Calculate horizontal centering
	// Content width: prefix(3) + name(40) + " [" (2) + icon(1) + "]" (1) + space(1) = 48 chars
	contentWidth := 48
	leftPadding := (termWidth - contentWidth) / 2
	if leftPadding < 0 {
		leftPadding = 0
	}

	// Calculate viewport dimensions
	headerLines := 6  // ANSI art header (subscribe.ans is 6 lines)
	spacingLines := 2 // Blank line after header, blank line before footer
	footerLines := 1  // Command line at bottom
	availableRows := termHeight - headerLines - spacingLines - footerLines
	if availableRows < 5 {
		availableRows = 5
	}

	viewportOffset := 0
	previousIdx := -1
	previousViewportOffset := 0

	// Helper to pad string to width
	padRight := func(s string, width int) string {
		if len(s) >= width {
			return s
		}
		return s + strings.Repeat(" ", width-len(s))
	}

	// Format single area line (matching Retrograde layout)
	formatAreaLine := func(item areaListItem, selected bool, tagged bool) string {
		paddingStr := strings.Repeat(" ", leftPadding)

		if item.isHeader {
			// Conference header - cyan color with one space before name
			return fmt.Sprintf("%s\x1b[0;96m %s\x1b[0m", paddingStr, item.confName)
		}

		prefix := "   "
		if selected {
			prefix = " > "
		}

		statusIcon := " "
		if tagged {
			statusIcon = "\xFB" // CP437 checkmark (√)
		}

		// Use plain grey for unselected, dark cyan bg + white text for selected
		colorSeq := "\x1b[37m" // Plain grey (ANSI 37)
		if selected {
			colorSeq = "\x1b[97;46m" // Bright white text on cyan background
		}

		// Truncate area name if too long
		areaName := item.area.Name
		if len(areaName) > 40 {
			areaName = areaName[:37] + "..."
		}

		var builder strings.Builder
		builder.WriteString(paddingStr) // Add left padding for centering
		builder.WriteString(colorSeq)
		builder.WriteString(prefix)
		builder.WriteString(padRight(areaName, 40))
		builder.WriteString(" [")

		if tagged {
			builder.WriteString("\x1b[96m") // Bright cyan for checkmark
			builder.WriteString(statusIcon)
			builder.WriteString(colorSeq)
		} else {
			builder.WriteString(statusIcon)
		}

		builder.WriteString("]")
		builder.WriteString(" ")
		builder.WriteString("\x1b[0m")

		return builder.String()
	}

	// Adjust viewport to ensure currentIdx is visible
	adjustViewport := func() {
		if currentIdx < viewportOffset {
			viewportOffset = currentIdx
			// Include conference header if present
			if currentIdx > 0 && accessibleAreas[currentIdx-1].isHeader {
				viewportOffset = currentIdx - 1
			}
		}
		if currentIdx >= viewportOffset+availableRows {
			viewportOffset = currentIdx - availableRows + 1
		}
		if viewportOffset < 0 {
			viewportOffset = 0
		}
		maxOffset := len(accessibleAreas) - availableRows
		if maxOffset < 0 {
			maxOffset = 0
		}
		if viewportOffset > maxOffset {
			viewportOffset = maxOffset
		}
	}

	// Draw only the viewport items
	drawItems := func() {
		// Position cursor after header + blank line
		itemStartLine := headerLines + 2 // +1 for header, +1 for blank line
		terminalio.WriteProcessedBytes(terminal, []byte(fmt.Sprintf("\x1b[%d;1H", itemStartLine)), outputMode)

		endIdx := viewportOffset + availableRows
		if endIdx > len(accessibleAreas) {
			endIdx = len(accessibleAreas)
		}

		lineNum := 0
		for i := viewportOffset; i < endIdx && lineNum < availableRows; i++ {
			item := accessibleAreas[i]
			selected := (i == currentIdx)
			tagged := false
			if !item.isHeader && item.area != nil {
				tagged = taggedMap[item.area.ID]
			}

			// Clear line and draw
			terminalio.WriteProcessedBytes(terminal, []byte("\x1b[2K"), outputMode)
			line := formatAreaLine(item, selected, tagged)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line+"\r\n")), outputMode)
			lineNum++
		}

		// Clear remaining lines in viewport
		for lineNum < availableRows {
			terminalio.WriteProcessedBytes(terminal, []byte("\x1b[2K\r\n"), outputMode)
			lineNum++
		}
	}

	// Smart redraw - only updates changed lines
	smartRedraw := func() {
		if previousViewportOffset != viewportOffset {
			// Viewport changed - full redraw of items
			drawItems()
		} else if previousIdx >= viewportOffset && previousIdx < viewportOffset+availableRows &&
			currentIdx >= viewportOffset && currentIdx < viewportOffset+availableRows {
			// Same viewport, just selection changed - redraw two lines
			// Redraw previous selection
			itemStartLine := headerLines + 2
			prevLineNum := previousIdx - viewportOffset
			terminalio.WriteProcessedBytes(terminal, []byte(fmt.Sprintf("\x1b[%d;1H", itemStartLine+prevLineNum)), outputMode)
			terminalio.WriteProcessedBytes(terminal, []byte("\x1b[2K"), outputMode)
			item := accessibleAreas[previousIdx]
			tagged := false
			if !item.isHeader && item.area != nil {
				tagged = taggedMap[item.area.ID]
			}
			line := formatAreaLine(item, false, tagged)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

			// Redraw current selection
			currLineNum := currentIdx - viewportOffset
			terminalio.WriteProcessedBytes(terminal, []byte(fmt.Sprintf("\x1b[%d;1H", itemStartLine+currLineNum)), outputMode)
			terminalio.WriteProcessedBytes(terminal, []byte("\x1b[2K"), outputMode)
			item = accessibleAreas[currentIdx]
			tagged = false
			if !item.isHeader && item.area != nil {
				tagged = taggedMap[item.area.ID]
			}
			line = formatAreaLine(item, true, tagged)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)
		} else {
			// Full redraw needed
			drawItems()
		}

		previousIdx = currentIdx
		previousViewportOffset = viewportOffset
	}

	// Initial screen draw
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)

	// Try to display ANSI header
	ansPath := filepath.Join(e.MenuSetPath, "ansi", "NEWSCAN.ANS")
	headerContent, ansErr := ansi.GetAnsiFileContent(ansPath)
	if ansErr == nil {
		terminalio.WriteProcessedBytes(terminal, headerContent, outputMode)
	} else {
		// Fallback to text header
		header := "|15Newscan Configuration|07\r\n" +
			"|08" + strings.Repeat("-", 40) + "|07\r\n" +
			"|07Tag areas to scan for new messages|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(header)), outputMode)
	}

	// Find next selectable index (skip headers)
	findNextSelectable := func(startIdx int, direction int) int {
		idx := startIdx
		for {
			idx += direction
			if idx < 0 || idx >= len(accessibleAreas) {
				return startIdx // Can't move
			}
			if !accessibleAreas[idx].isHeader {
				return idx
			}
		}
	}

	// Hide cursor for cleaner display
	terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)

	// Ensure cursor is restored when we exit
	defer terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode)

	adjustViewport()
	drawItems()

	// Draw footer at bottom with centering (with blank line above it)
	footerLine := termHeight // Footer on last line, blank line naturally above it
	terminalio.WriteProcessedBytes(terminal, []byte(fmt.Sprintf("\x1b[%d;1H", footerLine)), outputMode)

	// Match Retrograde footer style: Cyan command + Yellow colon + White description
	footerText := "\x1b[36mSPACE\x1b[93m:\x1b[37mToggle  " +
		"\x1b[36mA\x1b[93m:\x1b[37mAll  " +
		"\x1b[36mN\x1b[93m:\x1b[37mNone  " +
		"\x1b[36mESC\x1b[93m:\x1b[37mExit\x1b[0m"

	// Center the footer (approximate visible length without ANSI codes)
	footerVisibleLen := 34 // "SPACE:Toggle  A:All  N:None  ESC:Exit"
	footerPadding := (termWidth - footerVisibleLen) / 2
	if footerPadding > 0 {
		terminalio.WriteProcessedBytes(terminal, []byte(strings.Repeat(" ", footerPadding)), outputMode)
	}
	terminalio.WriteProcessedBytes(terminal, []byte(footerText), outputMode)

	for {
		seq, err := readKeySequence(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", err
		}

		switch seq {
		case "\x1b[A": // Up arrow
			newIdx := findNextSelectable(currentIdx, -1)
			if newIdx != currentIdx {
				currentIdx = newIdx
				adjustViewport()
				smartRedraw()
			}

		case "\x1b[B": // Down arrow
			newIdx := findNextSelectable(currentIdx, 1)
			if newIdx != currentIdx {
				currentIdx = newIdx
				adjustViewport()
				smartRedraw()
			}

		case "\x1b[5~": // Page Up
			moved := 0
			newIdx := currentIdx
			for moved < availableRows && newIdx > 0 {
				testIdx := findNextSelectable(newIdx, -1)
				if testIdx == newIdx {
					break
				}
				newIdx = testIdx
				moved++
			}
			if newIdx != currentIdx {
				currentIdx = newIdx
				adjustViewport()
				drawItems()
				previousIdx = currentIdx
				previousViewportOffset = viewportOffset
			}

		case "\x1b[6~": // Page Down
			moved := 0
			newIdx := currentIdx
			for moved < availableRows && newIdx < len(accessibleAreas)-1 {
				testIdx := findNextSelectable(newIdx, 1)
				if testIdx == newIdx {
					break
				}
				newIdx = testIdx
				moved++
			}
			if newIdx != currentIdx {
				currentIdx = newIdx
				adjustViewport()
				drawItems()
				previousIdx = currentIdx
				previousViewportOffset = viewportOffset
			}

		case " ", "\r", "\n": // Space or Enter - toggle
			if !accessibleAreas[currentIdx].isHeader {
				area := accessibleAreas[currentIdx].area
				if taggedMap[area.ID] {
					// Untag
					delete(taggedMap, area.ID)
				} else {
					// Tag
					taggedMap[area.ID] = true
				}
				// Redraw just current line
				itemStartLine := headerLines + 2
				currLineNum := currentIdx - viewportOffset
				terminalio.WriteProcessedBytes(terminal, []byte(fmt.Sprintf("\x1b[%d;1H", itemStartLine+currLineNum)), outputMode)
				terminalio.WriteProcessedBytes(terminal, []byte("\x1b[2K"), outputMode)
				line := formatAreaLine(accessibleAreas[currentIdx], true, taggedMap[area.ID])
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)
			}

		case "A", "a": // Tag all
			for _, item := range accessibleAreas {
				if !item.isHeader {
					taggedMap[item.area.ID] = true
				}
			}
			drawItems()

		case "N", "n": // Untag all
			taggedMap = make(map[int]bool)
			drawItems()

		case "\x1b", "Q", "q": // ESC or Q - exit
			// Save tagged areas to user
			var taggedIDs []int
			for areaID := range taggedMap {
				taggedIDs = append(taggedIDs, areaID)
			}
			currentUser.TaggedMessageAreaIDs = taggedIDs

			terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
			if err := userManager.UpdateUser(currentUser); err != nil {
				log.Printf("ERROR: Node %d: Failed to save newscan config: %v", nodeNumber, err)
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.ScanConfigError)), outputMode)
			} else {
				msg := fmt.Sprintf(e.LoadedStrings.ScanConfigSaved, len(taggedIDs))
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			}
			time.Sleep(1 * time.Second)
			return currentUser, "", nil
		}
	}
}
