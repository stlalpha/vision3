package menu

import (
	"errors"
	"io"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/editor"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// runChangeMsgConferenceLightbar is the lightbar version of runChangeMsgConference.
func runChangeMsgConferenceLightbar(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User,
	nodeNumber int, sessionStartTime time.Time, args string,
	outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {

	log.Printf("DEBUG: Node %d: Running CHANGEMSGCONF (lightbar)", nodeNumber)

	// Resolve terminal dimensions: prefer passed values, then user prefs, then defaults.
	if termWidth <= 0 && currentUser != nil {
		termWidth = currentUser.ScreenWidth
	}
	if termWidth <= 0 {
		termWidth = 80
	}
	if termHeight <= 0 && currentUser != nil {
		termHeight = currentUser.ScreenHeight
	}
	if termHeight <= 0 {
		termHeight = 24
	}

	if currentUser == nil {
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.ConfLoginRequired)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	if e.ConferenceMgr == nil {
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.ConfNoConferences)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	templateDir := filepath.Join(e.MenuSetPath, "templates")
	topBytes, errTop := readTemplateFile(filepath.Join(templateDir, "MSGCONF.TOP"))
	midBytes, errMid := readTemplateFile(filepath.Join(templateDir, "MSGCONF.MID"))

	if errTop != nil || errMid != nil {
		log.Printf("WARN: Node %d: MSGCONF templates unavailable (%v/%v), using text mode", nodeNumber, errTop, errMid)
		return runChangeMsgConference(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode, termWidth, termHeight)
	}

	processedMidTemplate := string(ansi.ReplacePipeCodes(midBytes))

	buildItemLine := func(name, description string, displayIdx int) string {
		line := processedMidTemplate
		line = strings.ReplaceAll(line, "^CI", padRight(strconv.Itoa(displayIdx), 3))
		line = strings.ReplaceAll(line, "^CN", padRight(truncateStr(name, 33), 33))
		line = strings.ReplaceAll(line, "^CD", truncateStr(description, 40))
		return strings.TrimRight(line, "\r\n")
	}

	type confItem struct {
		id          int
		name        string
		description string
	}

	// Build the accessible conference list.
	var confs []confItem
	currentConfName := "None"
	for _, conf := range e.ConferenceMgr.ListConferences() {
		if !checkACS(conf.ACS, currentUser, s, terminal, sessionStartTime) {
			continue
		}
		confs = append(confs, confItem{id: conf.ID, name: conf.Name, description: conf.Description})
		if conf.ID == currentUser.CurrentMsgConferenceID {
			currentConfName = conf.Name
		}
	}

	if len(confs) == 0 {
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.ConfNoConferences)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	// Load optional highlight BAR file (MSGCONFHI.BAR) — same pattern as FILELISTHI.BAR.
	hiBarOptions, hiBarErr := loadBarFile("MSGCONFHI", e)
	if hiBarErr != nil {
		log.Printf("WARN: Node %d: Failed to load MSGCONFHI.BAR: %v", nodeNumber, hiBarErr)
	}

	// Measure header rows using the same pipeline as renderTop.
	sampleTop := strings.ReplaceAll(string(topBytes), "^CN", "")
	sampleWithTokens := e.applyCommonTemplateTokens([]byte(sampleTop), currentUser, nodeNumber)
	processedSample := string(ansi.ReplacePipeCodes(sampleWithTokens))
	headerLines := strings.Count(processedSample, "\n")
	// If the template's last row has no trailing \n, bump headerLines so
	// itemAreaStartRow is placed past the separator, not on it.
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
		hint := "|08[ |15Up|08/|15Dn|08 ] Navigate  [ |15PgUp|08/|15PgDn|08 ] Page  [ |15Enter|08 ] Select  [ |15Q|08 ] Quit"
		line := ansi.MoveCursor(hintRow, 1) + "\x1b[2K" + string(ansi.ReplacePipeCodes([]byte(hint)))
		return terminalio.WriteProcessedBytes(terminal, []byte(line), outputMode)
	}

	renderItemArea := func(topIndex, selectedIndex int) error {
		for row := 0; row < visibleRows; row++ {
			absRow := itemAreaStartRow + row
			if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(absRow, 1)+"\x1b[2K"), outputMode); err != nil {
				return err
			}
			idx := topIndex + row
			if idx >= len(confs) {
				continue
			}
			line := buildItemLine(confs[idx].name, confs[idx].description, idx+1)
			if idx == selectedIndex {
				stripped := stripAreaAnsi(line)
				if len(stripped) > termWidth {
					stripped = stripped[:termWidth]
				}
				rendered := hiColorSeq + padRight(stripped, termWidth) + "\x1b[0m"
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

	renderFull := func(topIndex, selectedIndex int) error {
		if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode); err != nil {
			return err
		}
		if err := renderTop(currentConfName); err != nil {
			return err
		}
		if err := renderItemArea(topIndex, selectedIndex); err != nil {
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

	// Pre-select the current conference.
	selectedIndex := 0
	for i, c := range confs {
		if c.id == currentUser.CurrentMsgConferenceID {
			selectedIndex = i
			break
		}
	}
	topIndex := 0

	clampSelection := func() {
		if len(confs) == 0 {
			selectedIndex, topIndex = 0, 0
			return
		}
		if selectedIndex < 0 {
			selectedIndex = 0
		}
		if selectedIndex >= len(confs) {
			selectedIndex = len(confs) - 1
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
			if err := renderFull(topIndex, selectedIndex); err != nil {
				return nil, "", err
			}
			needFullRedraw = false
		} else if topIndex != prevTopIndex {
			if err := renderItemArea(topIndex, selectedIndex); err != nil {
				return nil, "", err
			}
		} else if selectedIndex != prevSelectedIndex {
			if prevSelectedIndex >= topIndex && prevSelectedIndex < topIndex+visibleRows {
				oldRow := itemAreaStartRow + (prevSelectedIndex - topIndex)
				oldLine := buildItemLine(confs[prevSelectedIndex].name, confs[prevSelectedIndex].description, prevSelectedIndex+1)
				if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(oldRow, 1)+"\x1b[2K"), outputMode); err != nil {
					return nil, "", err
				}
				if err := terminalio.WriteProcessedBytes(terminal, []byte(oldLine), outputMode); err != nil {
					return nil, "", err
				}
			}
			if selectedIndex >= topIndex && selectedIndex < topIndex+visibleRows {
				newRow := itemAreaStartRow + (selectedIndex - topIndex)
				newLine := buildItemLine(confs[selectedIndex].name, confs[selectedIndex].description, selectedIndex+1)
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
			if len(confs) > 0 {
				selectedIndex = len(confs) - 1
			}

		case editor.KeyEnter:
			if len(confs) == 0 {
				continue
			}
			chosen := confs[selectedIndex]
			e.setUserMsgConference(currentUser, chosen.id)

			firstArea := findFirstAccessibleAreaInConference(e, s, terminal, currentUser, chosen.id, sessionStartTime)
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

			confName := chosen.name
			confTag := ""
			if conf, ok := e.ConferenceMgr.GetByID(chosen.id); ok {
				confName = conf.Name
				confTag = conf.Tag
			}
			joinedMsg := e.LoadedStrings.JoinedMsgConf
			if joinedMsg == "" {
				joinedMsg = "\r\n|07(|15^CN|07) |15Conference Joined!|07\r\n"
			}
			joinedMsg = strings.ReplaceAll(joinedMsg, "^CN", confName)
			joinedMsg = strings.ReplaceAll(joinedMsg, "^CT", confTag)
			if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(joinedMsg)), outputMode); err != nil {
				log.Printf("WARN: Node %d: Failed to write joined conference message: %v", nodeNumber, err)
			}
			time.Sleep(1 * time.Second)

			log.Printf("INFO: Node %d: User %s changed to conference %d (%s), area: %s",
				nodeNumber, currentUser.Handle, chosen.id, confTag, currentUser.CurrentMessageAreaTag)
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
					if idx < len(confs) {
						selectedIndex = idx
					}
				case ch == '0':
					if 9 < len(confs) {
						selectedIndex = 9
					}
				}
			}
		}
	}
}
