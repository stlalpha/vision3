package menu

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/editor"
	"github.com/stlalpha/vision3/internal/file"
	"github.com/stlalpha/vision3/internal/terminalio"
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

// fileListPlaceholderRegex matches @FPAGE@, @FTOTAL@, @FCONFPATH@ with optional alignment and width.
// Modifier: | (0x7C) or │ (CP437 0xB3, common in ANSI art) followed by L/R/C — matches message-header format.
// Groups: 1=code (FPAGE|FTOTAL|FCONFPATH), 2=modifier (L|R|C), 3=:N digits, 4=# sequence
var fileListPlaceholderRegex = regexp.MustCompile(`@(FPAGE|FTOTAL|FCONFPATH)(?:[\x7C\xB3]([LRC]))?(?::(\d+)|(#+))?@`)

// processFileListPlaceholders replaces file-list-specific pipe codes and @-placeholders
// with current page, total pages, total file count, and conference path. Use in FILELIST.TOP and FILELIST.BOT.
// Pipe codes: |FPAGE ("Page X of Y"), |FTOTAL (total file count), |FCONFPATH (Conference > File Area).
// Placeholders support alignment modifiers: @FPAGE|R###@, @FTOTAL|C:5@, @FCONFPATH|R############@
// fconfpath is the pre-formatted "Conference > Area" string with pipe codes (from resolveFileConferencePath).
func processFileListPlaceholders(data []byte, currentPage, totalPages, totalFiles int, fconfpath string) []byte {
	s := string(data)
	pageStr := fmt.Sprintf("Page %d of %d", currentPage, totalPages)
	totalStr := strconv.Itoa(totalFiles)

	// Process @-placeholders FIRST so |FPAGE inside @FPAGE|R#####@ isn't consumed by pipe codes.
	// @CODE@ placeholders with optional alignment modifier (|L, |R, |C) and width (:N or ###)
	// For ###: width = total placeholder length (entire token) so replacement preserves ANSI layout.
	// E.g. @FPAGE|R###########@ is 20 cols — output is padded/truncated to 20 visible chars.
	s = fileListPlaceholderRegex.ReplaceAllStringFunc(s, func(match string) string {
		subs := fileListPlaceholderRegex.FindStringSubmatch(match)
		if len(subs) < 2 {
			return match
		}
		code := subs[1]
		modifier := ""
		if len(subs) > 2 {
			modifier = subs[2]
		}
		width := 0
		if len(subs) > 3 && subs[3] != "" {
			width, _ = strconv.Atoi(subs[3])
		} else if len(subs) > 4 && subs[4] != "" {
			// Visual width: entire placeholder length (matches message-header / editor placeholder behavior)
			width = len(match)
		}
		align := ansi.AlignLeft
		if modifier != "" {
			align = ansi.ParseAlignment(modifier)
		}

		var value string
		switch code {
		case "FPAGE":
			value = pageStr
		case "FTOTAL":
			value = totalStr
		case "FCONFPATH":
			value = fconfpath
		default:
			return match
		}
		if width <= 0 {
			return value
		}
		return ansi.ApplyWidthConstraintAligned(value, width, align)
	})

	// Pipe codes AFTER @-placeholders so |FPAGE inside @FPAGE|R#####@ isn't destroyed.
	s = strings.ReplaceAll(s, "|FPAGE", pageStr)
	s = strings.ReplaceAll(s, "|FTOTAL", totalStr)
	s = strings.ReplaceAll(s, "|FCONFPATH", fconfpath)

	return []byte(s)
}

func runListFilesLightbar(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time,
	currentAreaID int, currentAreaTag string, area *file.FileArea,
	topTemplateBytes []byte, processedMidTemplate string, processedBotTemplate []byte,
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

	// Count header lines from top template (line count is invariant to page/file counts).
	fconfpath := e.resolveFileConferencePath(currentUser)
	processedTopSample := ansi.ReplacePipeCodes(processFileListPlaceholders(topTemplateBytes, 1, 1, totalFiles, fconfpath))
	headerLines := strings.Count(string(processedTopSample), "\n")
	if headerLines < 1 {
		headerLines = 1
	}

	// Reserve rows for the separator, command bar, and optional BOT template.
	// Derive botLineCount from the expanded string (after placeholder + pipe-code
	// processing) so it matches what renderPageIndicator actually renders.
	botContent := strings.TrimRight(string(processedBotTemplate), "\r\n")
	botLineCount := 0
	if len(botContent) > 0 {
		expandedBotSample := string(ansi.ReplacePipeCodes(processFileListPlaceholders([]byte(botContent), 1, 1, totalFiles, fconfpath)))
		expandedBotSample = strings.ReplaceAll(expandedBotSample, "^PAGE", "1")
		expandedBotSample = strings.ReplaceAll(expandedBotSample, "^TOTALPAGES", "1")
		botLineCount = len(strings.Split(expandedBotSample, "\n"))
	}
	reservedBottom := 2 // separator + command bar
	if botLineCount > 0 {
		reservedBottom = 2 + botLineCount // separator + command bar + BOT lines
	}
	visibleRows := termHeight - headerLines - reservedBottom - 1
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

	// Description widths used below are calculated dynamically from the MID
	// template; the DIZ-level constraints (45 cols, 11 lines) are applied by
	// formatDIZLines before the display-width pass.

	// Description is rendered on separate continuation lines below the
	// metadata row; the DIZ-level constraints (45 cols, 11 lines) are
	// applied by formatDIZLines before any display-width truncation.

	// fileEntryHeight returns the number of screen lines a file at idx takes:
	// first line (metadata + first DIZ line) + continuation DIZ lines.
	fileEntryHeight := func(idx int) int {
		if idx < 0 || idx >= len(allFiles) {
			return 1
		}
		dizCount := len(formatDIZLines(allFiles[idx].Description, dizMaxWidth, dizMaxLines))
		if dizCount < 1 {
			return 1
		}
		return dizCount
	}

	// filesVisibleFrom counts how many files fit in visibleRows starting from startIdx,
	// accounting for each entry's variable height (DIZ lines).
	filesVisibleFrom := func(startIdx int) int {
		usedLines := 0
		count := 0
		for idx := startIdx; idx < len(allFiles) && usedLines < visibleRows; idx++ {
			h := fileEntryHeight(idx)
			if usedLines+1 > visibleRows {
				break
			}
			if usedLines+h > visibleRows {
				h = 1 // truncate DIZ, first line still fits
			}
			usedLines += h
			count++
		}
		return count
	}

	// topIndexForPrevPage walks backward from the current topIndex to find where
	// the previous page should start, filling visibleRows from bottom to top.
	topIndexForPrevPage := func() int {
		if topIndex <= 0 {
			return 0
		}
		usedLines := 0
		newTop := topIndex
		for idx := topIndex - 1; idx >= 0; idx-- {
			h := fileEntryHeight(idx)
			if usedLines+h > visibleRows {
				break
			}
			usedLines += h
			newTop = idx
		}
		return newTop
	}

	// calculatePageInfo walks all files with variable heights to determine
	// which page topIndex falls on and how many total pages exist.
	calculatePageInfo := func() (currentPage int, totalPagesCalc int) {
		if len(allFiles) == 0 {
			return 1, 1
		}
		page := 0
		idx := 0
		foundCurrent := false
		for idx < len(allFiles) {
			page++
			usedLines := 0
			pageStart := idx
			for idx < len(allFiles) && usedLines < visibleRows {
				h := fileEntryHeight(idx)
				if usedLines+1 > visibleRows {
					break
				}
				if usedLines+h > visibleRows {
					h = 1
				}
				usedLines += h
				idx++
			}
			if !foundCurrent && topIndex >= pageStart && topIndex < idx {
				currentPage = page
				foundCurrent = true
			}
		}
		if !foundCurrent {
			currentPage = page
		}
		totalPagesCalc = page
		return currentPage, totalPagesCalc
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
		if len(name) > 12 {
			name = name[:12]
		}
		fileNameStr := fmt.Sprintf("%-12s", name)
		dateStr := fileRec.UploadedAt.Format("01/02/06")
		sizeStr := fmt.Sprintf("%7s", formatSize(fileRec.Size))

		dizLines := formatDIZLines(fileRec.Description, dizMaxWidth, dizMaxLines)

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

		// Build prefix (everything except desc) for highlight and indent calc.
		prefixLine := strings.ReplaceAll(line, "^DESC", "")
		processedPrefix := string(ansi.ReplacePipeCodes([]byte(prefixLine)))
		prefixLen := len(stripAnsi(processedPrefix))
		descIndent := strings.Repeat(" ", prefixLen)
		descWidth := termWidth - prefixLen - 1
		if descWidth < 20 {
			descWidth = 20
		}

		firstDesc := ""
		if len(dizLines) > 0 {
			firstDesc = dizLines[0]
			if ansi.VisibleLength(firstDesc) > descWidth {
				firstDesc = ansi.TruncateVisible(firstDesc, descWidth)
			}
		}

		var contLines []string
		for i := 1; i < len(dizLines); i++ {
			cl := dizLines[i]
			if ansi.VisibleLength(cl) > descWidth {
				cl = ansi.TruncateVisible(cl, descWidth)
			}
			contLines = append(contLines, cl)
		}

		if 1+len(contLines) > maxLines {
			contLines = nil
		}

		var result []string

		if highlighted {
			plainPrefix := stripAnsi(processedPrefix)
			result = append(result, hiColorSeq+plainPrefix+"\x1b[0m"+firstDesc)
		} else {
			fullLine := strings.ReplaceAll(line, "^DESC", firstDesc)
			processed := string(ansi.ReplacePipeCodes([]byte(fullLine)))
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

	// Layout: separator row, then command bar, then optional BOT.
	cmdBarRow := max(1, termHeight-botLineCount)
	separatorRow := max(1, cmdBarRow-1)

	// renderSeparator draws the separator line above the command bar.
	// Uses CP437 0xFA (·) and 0xC4 (─) to match the header separator style.
	renderSeparator := func() error {
		sep := "\xfa" + strings.Repeat("\xc4", termWidth-2) + "\xfa"
		sepLine := ansi.MoveCursor(separatorRow, 1) + "\x1b[2K" + string(ansi.ReplacePipeCodes([]byte("|08"+sep+"|07")))
		return terminalio.WriteProcessedBytes(terminal, []byte(sepLine), outputMode)
	}

	// renderCmdBar redraws only the horizontal command bar.
	renderCmdBar := func() error {
		if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(cmdBarRow, 1)+"\x1b[2K"), outputMode); err != nil {
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
		currentPage, calcTotalPages := calculatePageInfo()

		if len(botContent) > 0 {
			// Replace ^PAGE/^TOTALPAGES (legacy) and |FPAGE/|FTOTAL/@FPAGE@/@FTOTAL@ (new).
			pageStr := string(processFileListPlaceholders([]byte(botContent), currentPage, calcTotalPages, len(allFiles), fconfpath))
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
		}
		return nil
	}

	// renderTop writes the TOP template with |FPAGE, |FTOTAL, @FPAGE@, @FTOTAL@ substituted.
	renderTop := func() error {
		if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(1, 1)), outputMode); err != nil {
			return err
		}
		curPage, calcTotalPages := calculatePageInfo()
		processed := ansi.ReplacePipeCodes(processFileListPlaceholders(topTemplateBytes, curPage, calcTotalPages, len(allFiles), fconfpath))
		return terminalio.WriteProcessedBytes(terminal, processed, outputMode)
	}

	// renderFull performs a complete screen redraw (used on first display and
	// after overlay commands that clobber the screen).
	renderFull := func() error {
		if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode); err != nil {
			return err
		}
		if err := renderTop(); err != nil {
			return err
		}
		if err := renderFileArea(); err != nil {
			return err
		}
		if err := renderSeparator(); err != nil {
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
	prevPage := -1
	needFullRedraw := true

	for {
		clampSelection()

		if needFullRedraw {
			if err := renderFull(); err != nil {
				return nil, "", err
			}
			needFullRedraw = false
		} else if topIndex != prevTopIndex {
			// Viewport scrolled — full redraw of all regions to prevent overlap.
			if err := renderTop(); err != nil {
				return nil, "", err
			}
			if err := renderFileArea(); err != nil {
				return nil, "", err
			}
			if err := renderSeparator(); err != nil {
				return nil, "", err
			}
			if err := renderCmdBar(); err != nil {
				return nil, "", err
			}
			if err := renderPageIndicator(); err != nil {
				return nil, "", err
			}
		} else if selectedIndex != prevSelectedIndex {
			// Same viewport, selection changed — redraw old/new rows; redraw TOP if page changed.
			curPage, _ := calculatePageInfo()
			if curPage != prevPage {
				if err := renderTop(); err != nil {
					return nil, "", err
				}
			}
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
		prevPage, _ = calculatePageInfo()

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
			newTop := topIndexForPrevPage()
			topIndex = newTop
			selectedIndex = newTop
			continue
		case "\x1b[6~": // Page Down
			count := filesVisibleFrom(topIndex)
			nextTop := topIndex + count
			if nextTop >= len(allFiles) {
				if len(allFiles) > 0 {
					selectedIndex = len(allFiles) - 1
				}
			} else {
				topIndex = nextTop
				selectedIndex = nextTop
			}
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
				descLines := formatDIZLines(sel.Description, dizMaxWidth, dizMaxLines)

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
					ctx, cancel := e.transferContext(s.Context())
					ziplab.RunZipLabView(ctx, s, terminal, filePath, sel.Filename, outputMode)
					cancel()
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
			fileIDsToDownload := make([]uuid.UUID, 0, len(currentUser.TaggedFileIDs))

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
				fileIDsToDownload = append(fileIDsToDownload, fileID)
			}

			if len(filesToDownload) > 0 {
				log.Printf("INFO: Node %d: Initiating transfer for %d file(s)", nodeNumber, len(filesToDownload))

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
					successCount, failCount = e.runTransferSend(s, terminal, proto, filesToDownload, fileIDsToDownload, outputMode, nodeNumber)
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

const (
	dizMaxWidth = 45
	dizMaxLines = 11
)

// formatDIZLines splits FILE_ID.DIZ content into display-ready lines.
// Each line is truncated to maxWidth visible characters (ANSI-aware).
// Returns at most maxLines lines, with trailing blank lines trimmed.
func formatDIZLines(content string, maxWidth, maxLines int) []string {
	if content == "" {
		return nil
	}

	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	rawLines := strings.Split(content, "\n")

	var lines []string
	for _, line := range rawLines {
		if len(lines) >= maxLines {
			break
		}
		line = strings.TrimRight(line, " \t")
		if ansi.VisibleLength(line) > maxWidth {
			line = ansi.TruncateVisible(line, maxWidth)
		}
		lines = append(lines, line)
	}

	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}
