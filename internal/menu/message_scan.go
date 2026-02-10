package menu

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gliderlabs/ssh"
	"github.com/robbiew/vision3/internal/ansi"
	"github.com/robbiew/vision3/internal/terminalio"
	"github.com/robbiew/vision3/internal/user"
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
func runGetScanType(s ssh.Session, terminal *term.Terminal,
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

		header := "|15Message Scanning Setup|07\r\n" +
			"|08" + strings.Repeat("-", 40) + "|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(header)), outputMode)

		// Date
		dateStr := "All New Messages"
		if cfg.ScanDate == 0 {
			dateStr = "ALL Messages"
		} else if cfg.ScanDate > 0 {
			dateStr = fmt.Sprintf("From: %s", time.Unix(cfg.ScanDate, 0).Format("01/02/06"))
		}
		line := fmt.Sprintf("|15D|07ate: |14%s|07\r\n", dateStr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		// To
		toStr := "N/A"
		if cfg.SearchTo != "" {
			toStr = fmt.Sprintf("Search For %s", cfg.SearchTo)
		}
		line = fmt.Sprintf("|15T|07o  : |14%s|07\r\n", toStr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		// From
		fromStr := "N/A"
		if cfg.SearchFrom != "" {
			fromStr = fmt.Sprintf("Search For %s", cfg.SearchFrom)
		}
		line = fmt.Sprintf("|15F|07rom: |14%s|07\r\n", fromStr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		// Range
		rangeStr := "All"
		if cfg.RangeStart > 0 && cfg.RangeEnd > 0 {
			rangeStr = fmt.Sprintf("%d-%d", cfg.RangeStart, cfg.RangeEnd)
		}
		line = fmt.Sprintf("|15R|07ange: |14%s|07\r\n", rangeStr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		// Update Pointers
		upStr := "Yes"
		if !cfg.UpdatePointers {
			upStr = "No"
		}
		line = fmt.Sprintf("|15U|07pdate NewScan Pointers: |14%s|07\r\n", upStr)
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
		line = fmt.Sprintf("|15S|07can Which Areas?: |14%s|07\r\n", whichStr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		line = "|15A|07bort Message Scanning\r\n\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)

		prompt := "|07Selection; (|15Cr|07/|15Scan|07) : |15"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
	}

	for {
		showMenu()

		key, err := readSingleKey(s)
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
			prompt := "\r\n|07Scan From; |15A|07ll, |15N|07ew Messages, or Enter |15Date|07: |15"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
			input, readErr := readLineInput(s, terminal, outputMode, 10)
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
			prompt := "\r\n|07\"To\" string to Search for (Cr/cancel): |15"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
			input, readErr := readLineInput(s, terminal, outputMode, 30)
			if readErr != nil {
				continue
			}
			cfg.SearchTo = input

		case 'F': // From
			prompt := "\r\n|07\"From\" string to Search for: |15"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
			input, readErr := readLineInput(s, terminal, outputMode, 30)
			if readErr != nil {
				continue
			}
			cfg.SearchFrom = input

		case 'R': // Range
			prompt := fmt.Sprintf("\r\n|07Range Start (|151-%d|07) : |15", numMsgs)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
			startInput, readErr := readLineInput(s, terminal, outputMode, 6)
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

			prompt = fmt.Sprintf("\r\n|07Range End (|15%d-%d|07) : |15", startNum, numMsgs)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
			endInput, readErr := readLineInput(s, terminal, outputMode, 6)
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
			prompt := "\r\n|15M|07arked Areas, |15A|07ll Areas, |15C|07urrent Area : |15"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)
			aKey, aErr := readSingleKey(s)
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
	currentOnly bool) (*user.User, string, error) {

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in to scan for new messages.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

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
	scanCfg, err := runGetScanType(s, terminal, outputMode, numMsgs, currentOnly, hiColor, loColor)
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
	header := "\r\n|15Scanning Messages...|07\r\n"
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(header)), outputMode)

	nonStop := false
	quitNewScan := false

	// If current area only, scan just that area
	if scanCfg.WhichAreas == 3 {
		if currentAreaID <= 0 {
			msg := "\r\n|01No message area selected.|07\r\n"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(1 * time.Second)
			return nil, "", nil
		}

		totalCount, _ := e.MessageMgr.GetMessageCountForArea(currentAreaID)
		if totalCount == 0 {
			msg := "\r\n|07No messages in current area.|07\r\n"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(1 * time.Second)
			return nil, "", nil
		}

		startMsg := determineStartMessage(e, scanCfg, currentAreaID, currentUser.Handle, totalCount)

		_, action, readErr := runMessageReader(e, s, terminal, userManager, currentUser,
			nodeNumber, sessionStartTime, outputMode, startMsg, totalCount, true)
		if readErr != nil || action == "LOGOFF" {
			return nil, "LOGOFF", readErr
		}
		return nil, "", nil
	}

	// Multi-area scan: iterate through accessible areas
	allAreas := e.MessageMgr.ListAreas()
	for _, area := range allAreas {
		if quitNewScan {
			break
		}

		// Check ACS
		if !checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
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
		boardMsg := fmt.Sprintf("\r\n|09Scanning |01(|13%s|01)... |07[|15%d|07/|15%d|07 msgs]",
			area.Tag, startMsg, totalCount)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(boardMsg)), outputMode)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)

		if !nonStop {
			// Show per-area lightbar: Read/Post/Jump/Skip/Quit/NonStop
			scanSuffix := fmt.Sprintf(" [%d/%d]", startMsg, totalCount)
			selectedKey, lbErr := runMsgLightbar(s, terminal, scanAreaOptions, outputMode,
				hiColor, loColor, scanSuffix, 0)
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
					sessionStartTime, "", outputMode)
				continue
			case 'J': // Jump to message #
				handleJump(s, terminal, outputMode, &startMsg, totalCount)
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

		// Read messages in this area
		_, action, readErr := runMessageReader(e, s, terminal, userManager, currentUser,
			nodeNumber, sessionStartTime, outputMode, startMsg, totalCount, true)
		if readErr != nil || action == "LOGOFF" {
			return nil, "LOGOFF", readErr
		}
		if action == "QUIT_NEWSCAN" {
			quitNewScan = true
		}

		// Update pointers
		if scanCfg.UpdatePointers {
			if saveErr := userManager.SaveUsers(); saveErr != nil {
				log.Printf("ERROR: Node %d: Failed to save user data during newscan: %v", nodeNumber, saveErr)
			}
		}
	}

	// Newscan complete
	complete := "\r\n|15Newscan complete!|07\r\n"
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(complete)), outputMode)
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
