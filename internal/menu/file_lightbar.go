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

	// Footer lines: command bar (1) + BOT template lines.
	botContent := strings.TrimRight(string(processedBotTemplate), "\r\n")
	botLines := 1 // command bar always takes one line
	if len(botContent) > 0 {
		botLines += strings.Count(botContent, "\n") + 1
	} else {
		botLines++ // hardcoded page indicator fallback
	}

	// +1 for the CRLF after the top template write.
	visibleRows := termHeight - headerLines - 1 - botLines
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
		if selectedIndex >= topIndex+visibleRows {
			topIndex = selectedIndex - visibleRows + 1
		}
		if topIndex < 0 {
			topIndex = 0
		}
	}

	render := func() error {
		// Clear screen.
		if err := terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode); err != nil {
			return err
		}

		// Write top template.
		if err := terminalio.WriteProcessedBytes(terminal, processedTopTemplate, outputMode); err != nil {
			return err
		}
		if err := terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode); err != nil {
			return err
		}

		// File rows — each file can take multiple lines if description wraps.
		linesUsed := 0

		if len(allFiles) == 0 {
			msg := "|07   No files in this area."
			if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg+"\r\n")), outputMode); err != nil {
				return err
			}
			linesUsed++
		} else {
			for idx := topIndex; idx < len(allFiles) && linesUsed < visibleRows; idx++ {
				fileRec := allFiles[idx]

				fileNumStr := fmt.Sprintf("%3d", idx+1)
				name := fileRec.Filename
				if len(name) > 18 {
					name = name[:18]
				}
				fileNameStr := fmt.Sprintf("%-18s", name)
				dateStr := fileRec.UploadedAt.Format("01/02/06")
				sizeStr := fmt.Sprintf("%7s", formatSize(fileRec.Size))

				// Normalize description whitespace.
				fullDesc := strings.ReplaceAll(fileRec.Description, "\r", "")
				fullDesc = strings.ReplaceAll(fullDesc, "\n", " ")
				fullDesc = strings.TrimSpace(fullDesc)

				markStr := " "
				if isFileTagged(fileRec.ID) {
					markStr = "*"
				}

				// Build the first line from the MID template with a placeholder for desc.
				line := processedMidTemplate
				line = strings.ReplaceAll(line, "^MARK", markStr)
				line = strings.ReplaceAll(line, "^NUM", fileNumStr)
				line = strings.ReplaceAll(line, "^NAME", fileNameStr)
				line = strings.ReplaceAll(line, "^DATE", dateStr)
				line = strings.ReplaceAll(line, "^SIZE", sizeStr)

				// Measure how much space the description has on the first line.
				lineWithoutDesc := strings.ReplaceAll(line, "^DESC", "")
				processedNoDesc := string(ansi.ReplacePipeCodes([]byte(lineWithoutDesc)))
				prefixLen := len(stripAnsi(processedNoDesc))
				firstLineDescWidth := termWidth - prefixLen - 1 // -1 to avoid auto-wrap
				if firstLineDescWidth < 10 {
					firstLineDescWidth = 10
				}

				// Continuation lines align with description column (PCBoard style).
				descIndent := strings.Repeat(" ", prefixLen)
				descContWidth := termWidth - prefixLen - 1
				if descContWidth < 20 {
					descContWidth = 20
				}

				// Split description into first-line portion and remainder.
				firstDesc := fullDesc
				remainDesc := ""
				if len(fullDesc) > firstLineDescWidth {
					// Word-wrap at space boundary.
					cut := firstLineDescWidth
					if spIdx := strings.LastIndex(fullDesc[:firstLineDescWidth], " "); spIdx > 0 {
						cut = spIdx
					}
					firstDesc = fullDesc[:cut]
					remainDesc = strings.TrimSpace(fullDesc[cut:])
				}

				line = strings.ReplaceAll(line, "^DESC", firstDesc)
				processed := string(ansi.ReplacePipeCodes([]byte(line)))

				// Build continuation lines from the remainder.
				var contLines []string
				remaining := remainDesc
				for remaining != "" && len(contLines) < 10 { // FILE_ID.DIZ spec: up to 10 lines
					cl := remaining
					if len(cl) > descContWidth {
						cut := descContWidth
						if spIdx := strings.LastIndex(remaining[:descContWidth], " "); spIdx > 0 {
							cut = spIdx
						}
						cl = remaining[:cut]
						remaining = strings.TrimSpace(remaining[cut:])
					} else {
						remaining = ""
					}
					contLines = append(contLines, cl)
				}

				// Check if this entry fits in remaining visible rows.
				entryLines := 1 + len(contLines)
				if linesUsed+entryLines > visibleRows {
					// Only show what fits — show the first line at minimum.
					contLines = nil
					if linesUsed+1 > visibleRows {
						break
					}
					entryLines = 1
				}

				// Render first line.
				if idx == selectedIndex {
					plain := stripAnsi(processed)
					padWidth := termWidth - 1
					if len(plain) < padWidth {
						plain += strings.Repeat(" ", padWidth-len(plain))
					}
					wrapped := hiColorSeq + plain + "\x1b[0m"
					if err := terminalio.WriteProcessedBytes(terminal, []byte(wrapped), outputMode); err != nil {
						return err
					}
				} else {
					if err := writeProcessedStringWithManualEncoding(terminal, []byte(processed), outputMode); err != nil {
						return err
					}
				}
				if err := terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode); err != nil {
					return err
				}
				linesUsed++

				// Render continuation lines (never highlighted).
				for _, cl := range contLines {
					contFormatted := "|07" + descIndent + cl
					if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(contFormatted)), outputMode); err != nil {
						return err
					}
					if err := terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode); err != nil {
						return err
					}
					linesUsed++
				}
			}
		}

		// Pad empty rows.
		for linesUsed < visibleRows {
			if err := terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode); err != nil {
				return err
			}
			linesUsed++
		}

		// Command bar (horizontal lightbar), centered with per-entry colors.
		barWidth := 0
		for ci, ent := range cmdEntries {
			barWidth += len(ent.label) + 2 // " label "
			if ci < len(cmdEntries)-1 {
				barWidth += 2 // gap between items
			}
		}
		pad := (termWidth - barWidth) / 2
		if pad < 0 {
			pad = 0
		}
		cmdBar := "\r\n" + strings.Repeat(" ", pad)
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
		if err := terminalio.WriteProcessedBytes(terminal, []byte(cmdBar), outputMode); err != nil {
			return err
		}

		// Page indicator from BOT template or hardcoded fallback.
		currentPage := 1
		if len(allFiles) > 0 && visibleRows > 0 {
			currentPage = (selectedIndex / visibleRows) + 1
		}
		calcTotalPages := 1
		if len(allFiles) > 0 && visibleRows > 0 {
			calcTotalPages = (len(allFiles) + visibleRows - 1) / visibleRows
		}

		if len(botContent) > 0 {
			// Use BOT template with substitutions.
			pageStr := botContent
			pageStr = strings.ReplaceAll(pageStr, "^PAGE", fmt.Sprintf("%d", currentPage))
			pageStr = strings.ReplaceAll(pageStr, "^TOTALPAGES", fmt.Sprintf("%d", calcTotalPages))
			// Process pipe codes in the template.
			processedPage := string(ansi.ReplacePipeCodes([]byte(pageStr)))
			// Center each line of the BOT template.
			for _, botLine := range strings.Split(processedPage, "\n") {
				botLine = strings.TrimRight(botLine, "\r")
				plainLen := len(stripAnsi(botLine))
				linePad := (termWidth - plainLen) / 2
				if linePad < 0 {
					linePad = 0
				}
				centered := "\r\n" + strings.Repeat(" ", linePad) + botLine
				if err := terminalio.WriteProcessedBytes(terminal, []byte(centered), outputMode); err != nil {
					return err
				}
			}
		} else {
			// Hardcoded fallback.
			pageText := fmt.Sprintf("Page %d of %d", currentPage, calcTotalPages)
			pagePad := (termWidth - len(pageText)) / 2
			if pagePad < 0 {
				pagePad = 0
			}
			pageLine := "\r\n" + strings.Repeat(" ", pagePad) + "|08" + pageText + "|07"
			if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(pageLine)), outputMode); err != nil {
				return err
			}
		}

		return nil
	}

	for {
		clampSelection()
		if err := render(); err != nil {
			return nil, "", err
		}

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
			}

		case "d":
			if len(currentUser.TaggedFileIDs) == 0 {
				msg := "\r\n|07No files marked for download. Use |15Space|07 to mark files.|07\r\n"
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(1 * time.Second)
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
				continue
			}

			if !proceed {
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07Download cancelled.|07")), outputMode)
				time.Sleep(500 * time.Millisecond)
				_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
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
							_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(fmt.Sprintf("|01%s transfer failed or was cancelled.\r\n", proto.Name))), outputMode)
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
		}
	}
}
