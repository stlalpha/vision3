package menu

import (
	"errors"
	"io"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/editor"
	"github.com/stlalpha/vision3/internal/message"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// runSelectMessageAreaLightbar is the lightbar version of runSelectMessageArea.
// It uses arrow-key navigation and paging for large area lists.
func runSelectMessageAreaLightbar(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User,
	nodeNumber int, sessionStartTime time.Time, args string,
	outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {

	log.Printf("DEBUG: Node %d: Running SELECTMSGAREA (lightbar)", nodeNumber)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in to select a message area.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	templateDir := filepath.Join(e.MenuSetPath, "templates")
	topBytes, errTop := readTemplateFile(filepath.Join(templateDir, "MSGAREA.TOP"))
	midBytes, errMid := readTemplateFile(filepath.Join(templateDir, "MSGAREA.MID"))

	if errTop != nil || errMid != nil {
		log.Printf("WARN: Node %d: MSGAREA templates unavailable (%v/%v), using text mode", nodeNumber, errTop, errMid)
		return runSelectMessageArea(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode, termWidth, termHeight)
	}

	processedMidTemplate := string(ansi.ReplacePipeCodes(midBytes))

	// Build accessible conference list for left/right navigation.
	var accessibleConfs []accessibleConf
	if e.ConferenceMgr != nil {
		for _, conf := range e.ConferenceMgr.ListConferences() {
			if checkACS(conf.ACS, currentUser, s, terminal, sessionStartTime) {
				accessibleConfs = append(accessibleConfs, accessibleConf{id: conf.ID, name: conf.Name})
			}
		}
	}

	filterConfID := currentUser.CurrentMsgConferenceID

	buildAreaList := func(confID int) []*message.MessageArea {
		var areas []*message.MessageArea
		for _, area := range e.MessageMgr.ListAreas() {
			if area.ConferenceID != confID {
				continue
			}
			if !checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
				continue
			}
			areas = append(areas, area)
		}
		sort.Slice(areas, func(i, j int) bool {
			return areas[i].Position < areas[j].Position
		})
		return areas
	}

	buildItemLine := func(area *message.MessageArea, displayIdx int) string {
		line := processedMidTemplate
		line = strings.ReplaceAll(line, "^ID", padRight(strconv.Itoa(displayIdx), 3))
		line = strings.ReplaceAll(line, "^TAG", padRight(truncateStr(area.Tag, 16), 16))
		line = strings.ReplaceAll(line, "^NA", padRight(truncateStr(area.Name, 38), 38))
		line = strings.ReplaceAll(line, "^DS", area.AreaType)
		return strings.TrimRight(line, "\r\n")
	}

	confNameFor := func(confID int) string {
		if e.ConferenceMgr != nil {
			if conf, ok := e.ConferenceMgr.GetByID(confID); ok {
				return conf.Name
			}
		}
		return "None"
	}

	// Load optional highlight BAR file (MSGAREAHI.BAR) — same pattern as FILELISTHI.BAR.
	hiBarOptions, hiBarErr := loadBarFile("MSGAREAHI", e)
	if hiBarErr != nil {
		log.Printf("WARN: Node %d: Failed to load MSGAREAHI.BAR: %v", nodeNumber, hiBarErr)
	}

	// Measure header rows using the same pipeline as renderTop so the count
	// is accurate even when applyCommonTemplateTokens expands multi-line tokens.
	sampleTop := strings.ReplaceAll(string(topBytes), "^CN", "")
	sampleWithTokens := e.applyCommonTemplateTokens([]byte(sampleTop), currentUser, nodeNumber)
	processedSample := string(ansi.ReplacePipeCodes(sampleWithTokens))
	headerLines := strings.Count(processedSample, "\n")
	// If the template's last row has no trailing \n, the cursor stays on that
	// row and headerLines is undercounted by 1 — causing itemAreaStartRow to
	// land on the separator line, which renderItemArea then overwrites.
	// Detect visible content after the last \n and bump headerLines if found.
	{
		lastNL := strings.LastIndex(processedSample, "\n")
		tail := processedSample
		if lastNL >= 0 {
			tail = processedSample[lastNL+1:]
		}
		tail = areaLightbarAnsiRe.ReplaceAllString(tail, "")
		tail = strings.Trim(tail, "\r")
		if len(tail) > 0 {
			headerLines++
		}
	}
	if headerLines < 1 {
		headerLines = 1
	}

	itemAreaStartRow := headerLines + 1
	separatorRow := termHeight - 1
	hintRow := termHeight
	if separatorRow <= itemAreaStartRow {
		separatorRow = itemAreaStartRow + 1
	}
	visibleRows := separatorRow - itemAreaStartRow
	if visibleRows < 3 {
		visibleRows = 3
	}

	hiColorSeq := colorCodeToAnsi(e.Theme.YesNoHighlightColor)
	if len(hiBarOptions) > 0 {
		hiColorSeq = colorCodeToAnsi(hiBarOptions[0].HighlightColor)
	}

	renderTop := func(confName string) error {
		topStr := strings.ReplaceAll(string(topBytes), "^CN", confName)
		withTokens := e.applyCommonTemplateTokens([]byte(topStr), currentUser, nodeNumber)
		processed := ansi.ReplacePipeCodes(withTokens)
		if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(1, 1)), outputMode); err != nil {
			return err
		}
		return terminalio.WriteProcessedBytes(terminal, processed, outputMode)
	}

	renderSeparator := func() error {
		count := termWidth - 2
		if count < 0 {
			count = 0
		}
		sep := strings.Repeat("\xc4", count)
		line := ansi.MoveCursor(separatorRow, 1) + "\x1b[2K" + string(ansi.ReplacePipeCodes([]byte("|08\xfa"+sep+"\xfa|07")))
		return terminalio.WriteProcessedBytes(terminal, []byte(line), outputMode)
	}

	renderHint := func() error {
		hint := "|08[ |15Up|08/|15Dn|08 ] Nav  [ |15Lt|08/|15Rt|08 ] Conf  [ |15PgUp|08/|15PgDn|08 ] Page  [ |15Enter|08 ] Select  [ |15Q|08 ] Quit"
		line := ansi.MoveCursor(hintRow, 1) + "\x1b[2K" + string(ansi.ReplacePipeCodes([]byte(hint)))
		return terminalio.WriteProcessedBytes(terminal, []byte(line), outputMode)
	}

	renderItemArea := func(areas []*message.MessageArea, topIndex, selectedIndex int) error {
		for row := 0; row < visibleRows; row++ {
			absRow := itemAreaStartRow + row
			if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(absRow, 1)+"\x1b[2K"), outputMode); err != nil {
				return err
			}
			idx := topIndex + row
			if idx >= len(areas) {
				continue
			}
			line := buildItemLine(areas[idx], idx+1)
			if idx == selectedIndex {
				rendered := hiColorSeq + padRight(stripAreaAnsi(line), termWidth) + "\x1b[0m"
				if err := terminalio.WriteProcessedBytes(terminal, []byte(rendered), outputMode); err != nil {
					return err
				}
			} else {
				if err := terminalio.WriteProcessedBytes(terminal, []byte(line), outputMode); err != nil {
					return err
				}
			}
		}
		return nil
	}

	renderFull := func(areas []*message.MessageArea, confName string, topIndex, selectedIndex int) error {
		if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode); err != nil {
			return err
		}
		if err := renderTop(confName); err != nil {
			return err
		}
		if err := renderItemArea(areas, topIndex, selectedIndex); err != nil {
			return err
		}
		if err := renderSeparator(); err != nil {
			return err
		}
		return renderHint()
	}

	ih := getSessionIH(s)
	_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
	defer terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode)

	areas := buildAreaList(filterConfID)
	confName := confNameFor(filterConfID)
	selectedIndex := 0
	topIndex := 0

	clampSelection := func() {
		if len(areas) == 0 {
			selectedIndex, topIndex = 0, 0
			return
		}
		if selectedIndex < 0 {
			selectedIndex = 0
		}
		if selectedIndex >= len(areas) {
			selectedIndex = len(areas) - 1
		}
		if selectedIndex < topIndex {
			topIndex = selectedIndex
		}
		if selectedIndex >= topIndex+visibleRows {
			topIndex = selectedIndex - visibleRows + 1
		}
		if topIndex < 0 {
			topIndex = 0
		}
	}

	prevSelectedIndex := -1
	prevTopIndex := -1
	needFullRedraw := true

	for {
		clampSelection()

		if needFullRedraw {
			if err := renderFull(areas, confName, topIndex, selectedIndex); err != nil {
				return nil, "", err
			}
			needFullRedraw = false
		} else if topIndex != prevTopIndex {
			if err := renderItemArea(areas, topIndex, selectedIndex); err != nil {
				return nil, "", err
			}
		} else if selectedIndex != prevSelectedIndex {
			// Redraw only the two changed rows.
			if prevSelectedIndex >= topIndex && prevSelectedIndex < topIndex+visibleRows {
				oldRow := itemAreaStartRow + (prevSelectedIndex - topIndex)
				oldLine := buildItemLine(areas[prevSelectedIndex], prevSelectedIndex+1)
				if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(oldRow, 1)+"\x1b[2K"), outputMode); err != nil {
					return nil, "", err
				}
				if err := terminalio.WriteProcessedBytes(terminal, []byte(oldLine), outputMode); err != nil {
					return nil, "", err
				}
			}
			if selectedIndex >= topIndex && selectedIndex < topIndex+visibleRows {
				newRow := itemAreaStartRow + (selectedIndex - topIndex)
				newLine := buildItemLine(areas[selectedIndex], selectedIndex+1)
				rendered := hiColorSeq + padRight(stripAreaAnsi(newLine), termWidth) + "\x1b[0m"
				if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(newRow, 1)+"\x1b[2K"), outputMode); err != nil {
					return nil, "", err
				}
				if err := terminalio.WriteProcessedBytes(terminal, []byte(rendered), outputMode); err != nil {
					return nil, "", err
				}
			}
		}

		prevSelectedIndex = selectedIndex
		prevTopIndex = topIndex

		keyInt, err := ih.ReadKey()
		if err != nil {
			if errors.Is(err, editor.ErrIdleTimeout) {
				return nil, "LOGOFF", editor.ErrIdleTimeout
			}
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", err
		}

		switch keyInt {
		case editor.KeyArrowUp:
			selectedIndex--

		case editor.KeyArrowDown:
			selectedIndex++

		case editor.KeyPageUp, editor.KeyCtrlR:
			selectedIndex -= visibleRows
			topIndex -= visibleRows
			if topIndex < 0 {
				topIndex = 0
			}

		case editor.KeyPageDown, editor.KeyCtrlC:
			selectedIndex += visibleRows
			topIndex += visibleRows

		case editor.KeyHome:
			selectedIndex = 0

		case editor.KeyEnd:
			if len(areas) > 0 {
				selectedIndex = len(areas) - 1
			}

		case editor.KeyArrowLeft:
			filterConfID = prevConf(accessibleConfs, filterConfID)
			areas = buildAreaList(filterConfID)
			confName = confNameFor(filterConfID)
			selectedIndex, topIndex = 0, 0
			needFullRedraw = true

		case editor.KeyArrowRight:
			filterConfID = nextConf(accessibleConfs, filterConfID)
			areas = buildAreaList(filterConfID)
			confName = confNameFor(filterConfID)
			selectedIndex, topIndex = 0, 0
			needFullRedraw = true

		case editor.KeyEnter:
			if len(areas) == 0 {
				continue
			}
			area := areas[selectedIndex]
			if !checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
				continue
			}
			currentUser.CurrentMessageAreaID = area.ID
			currentUser.CurrentMessageAreaTag = area.Tag
			e.setUserMsgConference(currentUser, area.ConferenceID)
			if err := userManager.UpdateUser(currentUser); err != nil {
				log.Printf("ERROR: Node %d: Failed to save user after area change: %v", nodeNumber, err)
			}
			log.Printf("INFO: Node %d: User %s changed message area to ID %d ('%s')",
				nodeNumber, currentUser.Handle, area.ID, area.Tag)
			return currentUser, "", nil

		case editor.KeyEsc:
			return currentUser, "", nil

		default:
			if keyInt >= 32 && keyInt < 127 {
				ch := rune(keyInt)
				switch {
				case ch == 'q' || ch == 'Q':
					return currentUser, "", nil
				case ch >= '1' && ch <= '9':
					idx := int(ch - '1')
					if idx < len(areas) {
						selectedIndex = idx
					}
				case ch == '0':
					if 9 < len(areas) {
						selectedIndex = 9
					}
				}
			}
		}
	}
}
