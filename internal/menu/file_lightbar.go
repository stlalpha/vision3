package menu

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/editor"
	"github.com/stlalpha/vision3/internal/file"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/transfer"
	"github.com/stlalpha/vision3/internal/user"
	"github.com/stlalpha/vision3/internal/ziplab"
)

// readKeySequenceIH reads one key via the session-scoped InputHandler and maps it
// to the same ANSI string format that readKeySequence returns, preventing the
// "double key press" race between the InputHandler goroutine and a raw bufio.Reader.
func readKeySequenceIH(ih *editor.InputHandler) (string, error) {
	key, err := ih.ReadKey()
	if err != nil {
		return "", err
	}
	switch key {
	case editor.KeyArrowUp:
		return "\x1b[A", nil
	case editor.KeyArrowDown:
		return "\x1b[B", nil
	case editor.KeyArrowRight:
		return "\x1b[C", nil
	case editor.KeyArrowLeft:
		return "\x1b[D", nil
	case editor.KeyPageUp:
		return "\x1b[5~", nil
	case editor.KeyPageDown:
		return "\x1b[6~", nil
	case editor.KeyHome:
		return "\x1b[H", nil
	case editor.KeyEnd:
		return "\x1b[F", nil
	case editor.KeyEsc:
		return "\x1b", nil
	default:
		return string(rune(key)), nil
	}
}

func runListFilesLightbar(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time,
	currentAreaID int, currentAreaTag string, area *file.FileArea,
	processedTopTemplate []byte, processedMidTemplate string, processedBotTemplate []byte,
	filesPerPage int, totalFiles int, totalPages int,
	cmdBarOptions []LightbarOption, hiBarOptions []LightbarOption,
	outputMode ansi.OutputMode) (*user.User, string, error) {

	// Hide cursor on entry, show on exit.
	_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
	defer terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode)

	// Fetch all files for the area.
	allFiles := e.FileMgr.GetFilesForArea(currentAreaID)

	selectedIndex := 0
	topIndex := 0
	cmdIndex := 0
	ih := getSessionIH(s)

	// Build command bar entries from BAR file or defaults.
	type cmdEntry struct {
		label          string
		hotkey         string
		highlightColor string
		regularColor   string
	}

	var cmdEntries []cmdEntry
	if len(cmdBarOptions) > 0 {
		for _, opt := range cmdBarOptions {
			cmdEntries = append(cmdEntries, cmdEntry{
				label:          opt.Text,
				hotkey:         strings.ToLower(opt.HotKey),
				highlightColor: colorCodeToAnsi(opt.HighlightColor),
				regularColor:   colorCodeToAnsi(opt.RegularColor),
			})
		}
	} else {
		// Default entries using theme colors.
		defHi := colorCodeToAnsi(e.Theme.YesNoHighlightColor)
		defLo := colorCodeToAnsi(e.Theme.YesNoRegularColor)
		defaults := []struct {
			label  string
			hotkey string
		}{
			{"Mark", " "},
			{"Info", "i"},
			{"View", "v"},
			{"Download", "d"},
			{"Upload", "u"},
			{"Quit", "q"},
		}
		for _, d := range defaults {
			cmdEntries = append(cmdEntries, cmdEntry{
				label:          d.label,
				hotkey:         d.hotkey,
				highlightColor: defHi,
				regularColor:   defLo,
			})
		}
	}

	// Highlight colors for file rows.
	hiColorSeq := colorCodeToAnsi(e.Theme.YesNoHighlightColor)
	if len(hiBarOptions) > 0 {
		hiColorSeq = colorCodeToAnsi(hiBarOptions[0].HighlightColor)
	}

	// Determine terminal dimensions.
	termHeight := 24
	termWidth := 80
	if ptyReq, _, ok := s.Pty(); ok && ptyReq.Window.Height > 0 {
		termHeight = ptyReq.Window.Height
		if ptyReq.Window.Width > 0 {
			termWidth = ptyReq.Window.Width
		}
	}

	// Count header lines from top template (each CRLF-terminated line).
	headerLines := strings.Count(string(processedTopTemplate), "\n")
	if headerLines < 1 {
		headerLines = 1
	}

	// Reserve the last 2 rows for the command bar (row termHeight-1) and BOT (row termHeight).
	// Both are rendered with absolute cursor positioning so the row count is exact.
	// File area occupies rows (headerLines+2) through (termHeight-2):
	//   visibleRows = (termHeight-2) - (headerLines+2) + 1 = termHeight - headerLines - 3
	botContent := strings.TrimRight(string(processedBotTemplate), "\r\n")
	visibleRows := termHeight - headerLines - 3
	if visibleRows < 3 {
		visibleRows = 3
	}

	isFileTagged := func(fileID uuid.UUID) bool {
		for _, taggedID := range currentUser.TaggedFileIDs {
			if taggedID == fileID {
				return true
			}
		}
		return false
	}

	formatSize := func(size int64) string {
		if size < 1024 {
			return fmt.Sprintf("%dB", size)
		}
		return fmt.Sprintf("%dk", size/1024)
	}

	// stripAnsi removes all ANSI escape sequences from a string.
	ansiRe := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	stripAnsi := func(s string) string {
		return ansiRe.ReplaceAllString(s, "")
	}

	// wrapText splits text into lines of at most maxWidth characters.
	wrapText := func(text string, maxWidth int, maxLines int) []string {
		// Normalize whitespace: replace newlines with spaces.
		text = strings.ReplaceAll(text, "\r", "")
		text = strings.ReplaceAll(text, "\n", " ")
		text = strings.TrimSpace(text)

		var lines []string
		for len(text) > 0 && len(lines) < maxLines {
			if len(text) <= maxWidth {
				lines = append(lines, text)
				break
			}
			// Find last space within maxWidth for word wrap.
			cut := maxWidth
			if idx := strings.LastIndex(text[:maxWidth], " "); idx > 0 {
				cut = idx
			}
			lines = append(lines, text[:cut])
			text = strings.TrimSpace(text[cut:])
		}
		return lines
	}

	// Compute the constant prefix length from the MID template so we can
	// calculate how many screen lines each file entry occupies (description
	// wrapping).  All substitution fields have fixed widths, so the prefix
	// length is the same for every file entry.
	sampleLine := processedMidTemplate
	sampleLine = strings.ReplaceAll(sampleLine, "^MARK", " ")
	sampleLine = strings.ReplaceAll(sampleLine, "^NUM", "  1")
	sampleLine = strings.ReplaceAll(sampleLine, "^NAME", fmt.Sprintf("%-18s", "x"))
	sampleLine = strings.ReplaceAll(sampleLine, "^DATE", "01/01/01")
	sampleLine = strings.ReplaceAll(sampleLine, "^SIZE", fmt.Sprintf("%7s", "0B"))
	sampleNoDesc := strings.ReplaceAll(sampleLine, "^DESC", "")
	midPrefixLen := len(stripAnsi(string(ansi.ReplacePipeCodes([]byte(sampleNoDesc)))))
	firstDescWidth := termWidth - midPrefixLen - 1
	if firstDescWidth < 10 {
		firstDescWidth = 10
	}
	contDescWidth := termWidth - midPrefixLen - 1
	if contDescWidth < 20 {
		contDescWidth = 20
	}

	// fileEntryHeight returns the number of screen lines a file at idx takes,
	// accounting for description word-wrapping (matches the render logic).
	fileEntryHeight := func(idx int) int {
		if idx < 0 || idx >= len(allFiles) {
			return 1
		}
		desc := strings.ReplaceAll(allFiles[idx].Description, "\r", "")
		desc = strings.ReplaceAll(desc, "\n", " ")
		desc = strings.TrimSpace(desc)
		if len(desc) <= firstDescWidth {
			return 1
		}
		// First-line word wrap cut.
		cut := firstDescWidth
		if spIdx := strings.LastIndex(desc[:firstDescWidth], " "); spIdx > 0 {
			cut = spIdx
		}
		remainder := strings.TrimSpace(desc[cut:])
		lines := 1
		for remainder != "" && lines < 11 { // up to 10 continuation lines (FILE_ID.DIZ spec)
			if len(remainder) <= contDescWidth {
				lines++
				break
			}
			cut = contDescWidth
			if spIdx := strings.LastIndex(remainder[:contDescWidth], " "); spIdx > 0 {
				cut = spIdx
			}
			remainder = strings.TrimSpace(remainder[cut:])
			lines++
		}
		return lines
	}

	clampSelection := func() {
		if len(allFiles) == 0 {
			selectedIndex = 0
			topIndex = 0
			return
		}
		if selectedIndex < 0 {
			selectedIndex = 0
		}
		if selectedIndex >= len(allFiles) {
			selectedIndex = len(allFiles) - 1
		}
		if selectedIndex < topIndex {
			topIndex = selectedIndex
		}
		// Scroll down: advance topIndex until selectedIndex fits within the
		// visible screen area, accounting for multi-line file entries.
		for topIndex < selectedIndex {
			usedLines := 0
			fits := false
			for idx := topIndex; idx < len(allFiles) && usedLines < visibleRows; idx++ {
				h := fileEntryHeight(idx)
				if usedLines+1 > visibleRows {
					break // can't fit even the first line
				}
				if usedLines+h > visibleRows {
					h = 1 // truncate continuation lines, first line still fits
				}
				usedLines += h
				if idx == selectedIndex {
					fits = true
					break
				}
			}
			if fits {
				break
			}
			topIndex++
		}
		if topIndex < 0 {
			topIndex = 0
		}
	}

	// --- Rendering helpers for smart refresh ---

	// fileAreaStartRow is the absolute terminal row where file entries begin.
	fileAreaStartRow := headerLines + 2

	// buildFileEntry produces the ANSI output for a single file entry at the
	// given file index, returning the rendered lines (first line + continuations)
	// already processed through pipe codes.  If highlighted is true the first
	// line uses the highlight color with stripped ANSI (lightbar style).
	// The caller specifies how many screen lines are available (maxLines) so the
	// function can truncate continuation lines to fit.
	buildFileEntry := func(idx int, highlighted bool, maxLines int) []string {
		if idx < 0 || idx >= len(allFiles) {
			return nil
		}
		fileRec := allFiles[idx]

		fileNumStr := fmt.Sprintf("%3d", idx+1)
		name := fileRec.Filename
		if len(name) > 18 {
			name = name[:18]
		}
		fileNameStr := fmt.Sprintf("%-18s", name)
		dateStr := fileRec.UploadedAt.Format("01/02/06")
		sizeStr := fmt.Sprintf("%7s", formatSize(fileRec.Size))

		fullDesc := strings.ReplaceAll(fileRec.Description, "\r", "")
		fullDesc = strings.ReplaceAll(fullDesc, "\n", " ")
		fullDesc = strings.TrimSpace(fullDesc)

		markStr := " "
		if isFileTagged(fileRec.ID) {
			markStr = "*"
		}

		line := processedMidTemplate
		line = strings.ReplaceAll(line, "^MARK", markStr)
		line = strings.ReplaceAll(line, "^NUM", fileNumStr)
		line = strings.ReplaceAll(line, "^NAME", fileNameStr)
		line = strings.ReplaceAll(line, "^DATE", dateStr)
		line = strings.ReplaceAll(line, "^SIZE", sizeStr)

		lineWithoutDesc := strings.ReplaceAll(line, "^DESC", "")
		processedNoDesc := string(ansi.ReplacePipeCodes([]byte(lineWithoutDesc)))
		prefixLen := len(stripAnsi(processedNoDesc))
		flDescWidth := termWidth - prefixLen - 1
		if flDescWidth < 10 {
			flDescWidth = 10
		}
		descIndent := strings.Repeat(" ", prefixLen)
		dcWidth := termWidth - prefixLen - 1
		if dcWidth < 20 {
			dcWidth = 20
		}

		firstDesc := fullDesc
		remainDesc := ""
		if len(fullDesc) > flDescWidth {
			cut := flDescWidth
			if spIdx := strings.LastIndex(fullDesc[:flDescWidth], " "); spIdx > 0 {
				cut = spIdx
			}
			firstDesc = fullDesc[:cut]
			remainDesc = strings.TrimSpace(fullDesc[cut:])
		}

		line = strings.ReplaceAll(line, "^DESC", firstDesc)
		processed := string(ansi.ReplacePipeCodes([]byte(line)))

		var contLines []string
		rem := remainDesc
		for rem != "" && len(contLines) < 10 {
			cl := rem
			if len(cl) > dcWidth {
				cut := dcWidth
				if spIdx := strings.LastIndex(rem[:dcWidth], " "); spIdx > 0 {
					cut = spIdx
				}
				cl = rem[:cut]
				rem = strings.TrimSpace(rem[cut:])
			} else {
				rem = ""
			}
			contLines = append(contLines, cl)
		}

		// Truncate continuation lines to fit maxLines.
		if 1+len(contLines) > maxLines {
			contLines = nil
		}

		var result []string

		// First line: highlighted or normal.
		if highlighted {
			plain := stripAnsi(processed)
			padWidth := termWidth - 1
			if len(plain) < padWidth {
				plain += strings.Repeat(" ", padWidth-len(plain))
			}
			result = append(result, hiColorSeq+plain+"\x1b[0m")
		} else {
			result = append(result, processed)
		}

		for _, cl := range contLines {
			result = append(result, string(ansi.ReplacePipeCodes([]byte("|07"+descIndent+cl))))
		}
		return result
	}

	// screenRowForFile returns the absolute terminal row a file entry starts on,
	// given the current topIndex.  Returns -1 if the file is not in the viewport.
	screenRowForFile := func(fileIdx int) (startRow int, height int) {
		if fileIdx < topIndex {
			return -1, 0
		}
		row := fileAreaStartRow
		for idx := topIndex; idx < len(allFiles) && (row-fileAreaStartRow) < visibleRows; idx++ {
			h := fileEntryHeight(idx)
			remaining := visibleRows - (row - fileAreaStartRow)
			if h > remaining {
				h = 1
			}
			if remaining < 1 {
				break
			}
			if idx == fileIdx {
				return row, h
			}
			row += h
		}
		return -1, 0
	}

	// writeFileRow renders a single file entry at the given absolute screen row,
	// clearing the lines it occupies first.
	writeFileRow := func(screenRow int, fileIdx int, highlighted bool, height int) error {
		lines := buildFileEntry(fileIdx, highlighted, height)
		for i, ln := range lines {
			r := screenRow + i
			if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(r, 1)+"\x1b[2K"), outputMode); err != nil {
				return err
			}
			if highlighted && i == 0 {
				// First line already has raw ANSI from buildFileEntry; write directly.
				if err := terminalio.WriteProcessedBytes(terminal, []byte(ln), outputMode); err != nil {
					return err
				}
			} else if i == 0 {
				if err := writeProcessedStringWithManualEncoding(terminal, []byte(ln), outputMode); err != nil {
					return err
				}
			} else {
				if err := terminalio.WriteProcessedBytes(terminal, []byte(ln), outputMode); err != nil {
					return err
				}
			}
		}
		// Clear any remaining lines in this entry's allocated height.
		for i := len(lines); i < height; i++ {
			r := screenRow + i
			if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(r, 1)+"\x1b[2K"), outputMode); err != nil {
				return err
			}
		}
		return nil
	}

	// renderFileArea redraws only the file list rows using absolute cursor
	// positioning and line-clear (no full screen clear).
	renderFileArea := func() error {
		if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(fileAreaStartRow, 1)), outputMode); err != nil {
			return err
		}

		linesUsed := 0
		if len(allFiles) == 0 {
			msg := "|07   No files in this area."
			if err := terminalio.WriteProcessedBytes(terminal, []byte("\x1b[2K"), outputMode); err != nil {
				return err
			}
			if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode); err != nil {
				return err
			}
			linesUsed = 1
		} else {
			for idx := topIndex; idx < len(allFiles) && linesUsed < visibleRows; idx++ {
				h := fileEntryHeight(idx)
				remaining := visibleRows - linesUsed
				if h > remaining {
					h = 1
				}
				if remaining < 1 {
					break
				}
				row := fileAreaStartRow + linesUsed
				if err := writeFileRow(row, idx, idx == selectedIndex, h); err != nil {
					return err
				}
				linesUsed += h
			}
		}

		// Clear unused rows.
		for linesUsed < visibleRows {
			r := fileAreaStartRow + linesUsed
			if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(r, 1)+"\x1b[2K"), outputMode); err != nil {
				return err
			}
			linesUsed++
		}
		return nil
	}

	// renderCmdBar redraws only the horizontal command bar.
	renderCmdBar := func() error {
		if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(termHeight-1, 1)+"\x1b[2K"), outputMode); err != nil {
			return err
		}
		barWidth := 0
		for ci, ent := range cmdEntries {
			barWidth += len(ent.label) + 2
			if ci < len(cmdEntries)-1 {
				barWidth += 2
			}
		}
		pad := (termWidth - barWidth) / 2
		if pad < 0 {
			pad = 0
		}
		cmdBar := strings.Repeat(" ", pad)
		for ci, ent := range cmdEntries {
			if ci == cmdIndex {
				cmdBar += ent.highlightColor + " " + ent.label + " " + "\x1b[0m"
			} else {
				cmdBar += ent.regularColor + " " + ent.label + " " + "\x1b[0m"
			}
			if ci < len(cmdEntries)-1 {
				cmdBar += "  "
			}
		}
		return terminalio.WriteProcessedBytes(terminal, []byte(cmdBar), outputMode)
	}

	// renderPageIndicator redraws only the page/bot indicator row(s).
	renderPageIndicator := func() error {
		currentPage := 1
		if len(allFiles) > 0 && visibleRows > 0 {
			currentPage = (topIndex / visibleRows) + 1
			// More accurately: count how many viewports topIndex has scrolled past.
			// But use selectedIndex-based page for user-facing consistency.
			currentPage = (selectedIndex / visibleRows) + 1
		}
		calcTotalPages := 1
		if len(allFiles) > 0 && visibleRows > 0 {
			calcTotalPages = (len(allFiles) + visibleRows - 1) / visibleRows
		}

		if len(botContent) > 0 {
			pageStr := botContent
			pageStr = strings.ReplaceAll(pageStr, "^PAGE", fmt.Sprintf("%d", currentPage))
			pageStr = strings.ReplaceAll(pageStr, "^TOTALPAGES", fmt.Sprintf("%d", calcTotalPages))
			processedPage := string(ansi.ReplacePipeCodes([]byte(pageStr)))
			botLineSlice := strings.Split(processedPage, "\n")
			for i, botLine := range botLineSlice {
				botLine = strings.TrimRight(botLine, "\r")
				plainLen := len(stripAnsi(botLine))
				linePad := (termWidth - plainLen) / 2
				if linePad < 0 {
					linePad = 0
				}
				row := termHeight - len(botLineSlice) + 1 + i
				if row < 1 {
					row = 1
				}
				centered := ansi.MoveCursor(row, 1) + "\x1b[2K" + strings.Repeat(" ", linePad) + botLine
				if err := terminalio.WriteProcessedBytes(terminal, []byte(centered), outputMode); err != nil {
					return err
				}
			}
		} else {
			pageText := fmt.Sprintf("Page %d of %d", currentPage, calcTotalPages)
			pagePad := (termWidth - len(pageText)) / 2
			if pagePad < 0 {
				pagePad = 0
			}
			pageLine := ansi.MoveCursor(termHeight, 1) + "\x1b[2K" + strings.Repeat(" ", pagePad) + "|08" + pageText + "|07"
			if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(pageLine)), outputMode); err != nil {
				return err
			}
		}
		return nil
	}

	// renderFull performs a complete screen redraw (used on first display and
	// after overlay commands that clobber the screen).
	renderFull := func() error {
		if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode); err != nil {
			return err
		}
		if err := terminalio.WriteProcessedBytes(terminal, processedTopTemplate, outputMode); err != nil {
			return err
		}
		if err := renderFileArea(); err != nil {
			return err
		}
		if err := renderCmdBar(); err != nil {
			return err
		}
		return renderPageIndicator()
	}

	// Track previous state for smart refresh.
	prevSelectedIndex := -1
	prevTopIndex := -1
	prevCmdIndex := -1
	needFullRedraw := true

	for {
		clampSelection()

		if needFullRedraw {
			if err := renderFull(); err != nil {
				return nil, "", err
			}
			needFullRedraw = false
		} else if topIndex != prevTopIndex {
			// Viewport scrolled — redraw file area + page indicator.
			if err := renderFileArea(); err != nil {
				return nil, "", err
			}
			if err := renderPageIndicator(); err != nil {
				return nil, "", err
			}
		} else if selectedIndex != prevSelectedIndex {
			// Same viewport, selection changed — redraw only old and new rows.
			if prevSelectedIndex >= 0 && prevSelectedIndex < len(allFiles) {
				if row, h := screenRowForFile(prevSelectedIndex); row >= 0 {
					if err := writeFileRow(row, prevSelectedIndex, false, h); err != nil {
						return nil, "", err
					}
				}
			}
			if row, h := screenRowForFile(selectedIndex); row >= 0 {
				if err := writeFileRow(row, selectedIndex, true, h); err != nil {
					return nil, "", err
				}
			}
		}
		if cmdIndex != prevCmdIndex {
			if err := renderCmdBar(); err != nil {
				return nil, "", err
			}
		}

		prevSelectedIndex = selectedIndex
		prevTopIndex = topIndex
		prevCmdIndex = cmdIndex

		key, err := readKeySequenceIH(ih)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", err
		}

		// Navigation keys.
		switch key {
		case "\x1b[A": // Up
			selectedIndex--
			continue
		case "\x1b[B": // Down
			selectedIndex++
			continue
		case "\x1b[C": // Right — command bar
			cmdIndex++
			if cmdIndex >= len(cmdEntries) {
				cmdIndex = 0
			}
			continue
		case "\x1b[D": // Left — command bar
			cmdIndex--
			if cmdIndex < 0 {
				cmdIndex = len(cmdEntries) - 1
			}
			continue
		case "\x1b[5~": // Page Up
			selectedIndex -= visibleRows
			continue
		case "\x1b[6~": // Page Down
			selectedIndex += visibleRows
			continue
		case "\x1b[H", "\x1b[1~": // Home
			selectedIndex = 0
			continue
		case "\x1b[F", "\x1b[4~": // End
			if len(allFiles) > 0 {
				selectedIndex = len(allFiles) - 1
			}
			continue
		case "\x1b": // Bare Esc
			return nil, "", nil
		case "\r", "\n": // Enter: execute selected command bar item
			key = cmdEntries[cmdIndex].hotkey
		}

		// Command dispatch (direct hotkeys or Enter-selected command).
		switch strings.ToLower(key) {
		case " ": // Space: toggle mark
			if len(allFiles) > 0 {
				fileID := allFiles[selectedIndex].ID
				found := false
				newTaggedIDs := make([]uuid.UUID, 0, len(currentUser.TaggedFileIDs))
				for _, taggedID := range currentUser.TaggedFileIDs {
					if taggedID == fileID {
						found = true
					} else {
						newTaggedIDs = append(newTaggedIDs, taggedID)
					}
				}
				if !found {
					newTaggedIDs = append(newTaggedIDs, fileID)
				}
				currentUser.TaggedFileIDs = newTaggedIDs
				if err := userManager.UpdateUser(currentUser); err != nil {
					log.Printf("ERROR: Node %d: Failed to save user after tag toggle: %v", nodeNumber, err)
				}
				// Redraw just the toggled row to show/hide the mark.
				if row, h := screenRowForFile(selectedIndex); row >= 0 {
					_ = writeFileRow(row, selectedIndex, true, h)
				}
			}

		case "q":
			return nil, "", nil

		case "i": // Info: show file detail overlay
			if len(allFiles) > 0 {
				sel := allFiles[selectedIndex]
				descWidth := termWidth - 14
				descMaxLines := 10
				descLines := wrapText(sel.Description, descWidth, descMaxLines)

				_ = terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)

				d1 := fmt.Sprintf("|15Filename  : |07%s\r\n", sel.Filename)
				d2 := fmt.Sprintf("|15Size      : |07%s\r\n", formatSize(sel.Size))
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(d1)), outputMode)
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(d2)), outputMode)

				for i, dl := range descLines {
					var dLine string
					if i == 0 {
						dLine = fmt.Sprintf("|15Desc      : |07%s\r\n", dl)
					} else {
						dLine = fmt.Sprintf("|07            %s\r\n", dl)
					}
					_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(dLine)), outputMode)
				}
				if len(descLines) == 0 {
					_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|15Desc      : |07(none)\r\n")), outputMode)
				}

				d3 := fmt.Sprintf("|15Uploaded  : |07%s\r\n", sel.UploadedAt.Format("01/02/2006 15:04"))
				d4 := fmt.Sprintf("|15Uploader  : |07%s\r\n", sel.UploadedBy)
				d5 := fmt.Sprintf("|15Downloads : |07%d\r\n", sel.DownloadCount)
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(d3)), outputMode)
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(d4)), outputMode)
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(d5)), outputMode)

				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|08Press any key to return...|07")), outputMode)
				_, _ = readKeySequenceIH(ih)
				needFullRedraw = true
			}

		case "v":
			if len(allFiles) > 0 {
				sel := &allFiles[selectedIndex]
				filePath, pathErr := e.FileMgr.GetFilePath(sel.ID)
				if pathErr != nil {
					log.Printf("ERROR: Node %d: Failed to get path for file %s: %v", nodeNumber, sel.ID, pathErr)
					continue
				}
				// Show cursor for the viewer.
				_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode)
				if e.FileMgr.IsSupportedArchive(sel.Filename) {
					ziplab.RunZipLabView(s, terminal, filePath, sel.Filename, outputMode)
				} else {
					termWidth, termHeight := getTerminalSize(s)
					viewFileByRecord(e, s, terminal, sel, outputMode, termWidth, termHeight)
				}
				// Hide cursor again.
				_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
				needFullRedraw = true
			}

		case "d":
			if len(currentUser.TaggedFileIDs) == 0 {
				msg := "\r\n|07No files marked for download. Use |15Space|07 to mark files.|07\r\n"
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(1 * time.Second)
				needFullRedraw = true
				continue
			}

			confirmPrompt := fmt.Sprintf("Download %d marked file(s)?", len(currentUser.TaggedFileIDs))
			_ = terminalio.WriteProcessedBytes(terminal, []byte("\r\n\x1b[K"), outputMode)
			_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode)

			termWidth, termHeight := getTerminalSize(s)
			proceed, promptErr := e.PromptYesNo(s, terminal, confirmPrompt, outputMode, nodeNumber, termWidth, termHeight, false)
			if promptErr != nil {
				if errors.Is(promptErr, io.EOF) {
					return nil, "LOGOFF", io.EOF
				}
				log.Printf("ERROR: Node %d: Error getting download confirmation: %v", nodeNumber, promptErr)
				_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
				needFullRedraw = true
				continue
			}

			if !proceed {
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07Download cancelled.|07")), outputMode)
				time.Sleep(500 * time.Millisecond)
				_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
				needFullRedraw = true
				continue
			}

			log.Printf("INFO: Node %d: User %s starting download of %d files.", nodeNumber, currentUser.Handle, len(currentUser.TaggedFileIDs))
			_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07Preparing download...\r\n")), outputMode)
			time.Sleep(500 * time.Millisecond)

			successCount := 0
			failCount := 0
			filesToDownload := make([]string, 0, len(currentUser.TaggedFileIDs))
			filenamesOnly := make([]string, 0, len(currentUser.TaggedFileIDs))

			for _, fileID := range currentUser.TaggedFileIDs {
				fp, pathErr := e.FileMgr.GetFilePath(fileID)
				if pathErr != nil {
					log.Printf("ERROR: Node %d: Failed to get path for file ID %s: %v", nodeNumber, fileID, pathErr)
					failCount++
					continue
				}
				if _, statErr := os.Stat(fp); os.IsNotExist(statErr) {
					log.Printf("ERROR: Node %d: File path %s for ID %s does not exist.", nodeNumber, fp, fileID)
					failCount++
					continue
				} else if statErr != nil {
					log.Printf("ERROR: Node %d: Error stating file path %s for ID %s: %v", nodeNumber, fp, fileID, statErr)
					failCount++
					continue
				}
				filesToDownload = append(filesToDownload, fp)
				filenamesOnly = append(filenamesOnly, filepath.Base(fp))
			}

			if len(filesToDownload) > 0 {
				log.Printf("INFO: Node %d: Initiating transfer for files: %v", nodeNumber, filenamesOnly)

				// Use protocol selection (respects connection type - SSH vs Telnet)
				proto, protoOK, protoErr := e.selectTransferProtocol(s, terminal, outputMode)
				if protoErr != nil {
					if errors.Is(protoErr, io.EOF) {
						return nil, "LOGOFF", io.EOF
					}
					log.Printf("ERROR: Node %d: Protocol selection error: %v", nodeNumber, protoErr)
					_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01Error: No transfer protocols configured on this system.|07\r\n")), outputMode)
					failCount = len(filesToDownload)
				} else if !protoOK {
					_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07Download cancelled.|07\r\n")), outputMode)
				} else {
					_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(fmt.Sprintf("|15Initiating %s transfer...\r\n", proto.Name))), outputMode)
					_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|07Please start the receive function in your terminal.\r\n")), outputMode)

					log.Printf("INFO: Node %d: Executing %s send: %v", nodeNumber, proto.Name, filenamesOnly)

					resetSessionIH(s)
					if proto.BatchSend && len(filesToDownload) > 1 {
						transferErr := proto.ExecuteSend(s, filesToDownload...)
						if transferErr != nil {
							log.Printf("ERROR: Node %d: %s batch send failed: %v", nodeNumber, proto.Name, transferErr)
							if errors.Is(transferErr, transfer.ErrBinaryNotFound) {
								_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01File transfer program not found!|07\r\n|07The SysOp needs to install the transfer binary (sexyz).\r\n|07See documentation/file-transfer-protocols.md for setup instructions.\r\n")), outputMode)
							} else {
								_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(fmt.Sprintf("|01%s transfer failed or was cancelled.\r\n", proto.Name))), outputMode)
							}
							failCount = len(filesToDownload)
							successCount = 0
						} else {
							log.Printf("INFO: Node %d: %s batch send completed successfully.", nodeNumber, proto.Name)
							_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(fmt.Sprintf("|07%s transfer complete.\r\n", proto.Name))), outputMode)
							successCount = len(filesToDownload)
							failCount = 0
							for _, fileID := range currentUser.TaggedFileIDs {
								if _, pathErr := e.FileMgr.GetFilePath(fileID); pathErr == nil {
									if incErr := e.FileMgr.IncrementDownloadCount(fileID); incErr != nil {
										log.Printf("WARN: Node %d: Failed to increment download count for file %s: %v", nodeNumber, fileID, incErr)
									}
								}
							}
						}
					} else {
						for i, fp := range filesToDownload {
							_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(fmt.Sprintf("|15[%d/%d]|07 Sending: |14%s|07...", i+1, len(filesToDownload), filenamesOnly[i]))), outputMode)
							if sendErr := proto.ExecuteSend(s, fp); sendErr != nil {
								log.Printf("ERROR: Node %d: %s send failed for %s: %v", nodeNumber, proto.Name, filenamesOnly[i], sendErr)
								if errors.Is(sendErr, transfer.ErrBinaryNotFound) {
									_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01File transfer program not found!|07\r\n|07The SysOp needs to install the transfer binary (sexyz).\r\n|07See documentation/file-transfer-protocols.md for setup instructions.\r\n")), outputMode)
									failCount += len(filesToDownload) - i
									break
								}
								_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(" |01FAILED|07\r\n")), outputMode)
								failCount++
							} else {
								log.Printf("INFO: Node %d: %s sent %s OK", nodeNumber, proto.Name, filenamesOnly[i])
								_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(" |02OK|07\r\n")), outputMode)
								successCount++
								if _, pathErr := e.FileMgr.GetFilePath(currentUser.TaggedFileIDs[i]); pathErr == nil {
									if incErr := e.FileMgr.IncrementDownloadCount(currentUser.TaggedFileIDs[i]); incErr != nil {
										log.Printf("WARN: Node %d: Failed to increment download count for file %s: %v", nodeNumber, currentUser.TaggedFileIDs[i], incErr)
									}
								}
							}
						}
					}
					time.Sleep(250 * time.Millisecond)
					ih = getSessionIH(s)
				}
				time.Sleep(1 * time.Second)
			} else {
				log.Printf("WARN: Node %d: No valid file paths found for tagged files.", nodeNumber)
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01Could not find any of the marked files on the server.|07\r\n")), outputMode)
				failCount = len(currentUser.TaggedFileIDs)
			}

			// Clear tags and save.
			currentUser.TaggedFileIDs = nil
			if saveErr := userManager.UpdateUser(currentUser); saveErr != nil {
				log.Printf("ERROR: Node %d: Failed to save user data after download: %v", nodeNumber, saveErr)
			}

			statusMsg := fmt.Sprintf("|07Download finished. Success: %d, Failed: %d.|07\r\n", successCount, failCount)
			_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(statusMsg)), outputMode)
			time.Sleep(2 * time.Second)

			// Refresh file list.
			allFiles = e.FileMgr.GetFilesForArea(currentAreaID)
			if selectedIndex >= len(allFiles) && len(allFiles) > 0 {
				selectedIndex = len(allFiles) - 1
			}
			_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
			needFullRedraw = true

		case "u":
			_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode)
			uploadErr := e.runUploadFiles(s, terminal, currentUser, userManager, currentAreaID, currentAreaTag, outputMode, nodeNumber, sessionStartTime)
			if uploadErr != nil {
				log.Printf("ERROR: Node %d: Upload error: %v", nodeNumber, uploadErr)
			}
			// runUploadFiles calls resetSessionIH/getSessionIH internally,
			// so the local ih is now stale — refresh it.
			ih = getSessionIH(s)
			// Refresh file list after upload.
			allFiles = e.FileMgr.GetFilesForArea(currentAreaID)
			if selectedIndex >= len(allFiles) && len(allFiles) > 0 {
				selectedIndex = len(allFiles) - 1
			}
			_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
			needFullRedraw = true
		}
	}
}
