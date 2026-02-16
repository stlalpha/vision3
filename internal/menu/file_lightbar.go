package menu

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/file"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/transfer"
	"github.com/stlalpha/vision3/internal/user"
	"github.com/stlalpha/vision3/internal/ziplab"
)

func runListFilesLightbar(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time,
	currentAreaID int, currentAreaTag string, area *file.FileArea,
	processedTopTemplate []byte, processedMidTemplate string, processedBotTemplate []byte,
	filesPerPage int, totalFiles int, totalPages int,
	outputMode ansi.OutputMode) (*user.User, string, error) {

	// Hide cursor on entry, show on exit.
	_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
	defer terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode)

	// Fetch all files for the area.
	allFiles := e.FileMgr.GetFilesForArea(currentAreaID)

	selectedIndex := 0
	topIndex := 0
	reader := bufio.NewReader(s)

	// Use theme highlight color matching the message lightbar.
	hiColorSeq := colorCodeToAnsi(e.Theme.YesNoHighlightColor)

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
	// Footer: bot template line + status line.
	footerLines := 2
	// +1 for the CRLF after the top template write.
	visibleRows := termHeight - headerLines - 1 - footerLines
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
				fileNameStr := fmt.Sprintf("%-12s", fileRec.Filename)
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

				// Render continuation lines.
				for _, cl := range contLines {
					contText := descIndent + cl
					if idx == selectedIndex {
						padWidth := termWidth - 1
						if len(contText) < padWidth {
							contText += strings.Repeat(" ", padWidth-len(contText))
						}
						wrapped := hiColorSeq + contText + "\x1b[0m"
						if err := terminalio.WriteProcessedBytes(terminal, []byte(wrapped), outputMode); err != nil {
							return err
						}
					} else {
						contFormatted := "|07" + descIndent + cl
						if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(contFormatted)), outputMode); err != nil {
							return err
						}
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

		// Bottom template with pagination.
		currentPage := 1
		if len(allFiles) > 0 && visibleRows > 0 {
			currentPage = (selectedIndex / visibleRows) + 1
		}
		calcTotalPages := 1
		if len(allFiles) > 0 && visibleRows > 0 {
			calcTotalPages = (len(allFiles) + visibleRows - 1) / visibleRows
		}
		bottomLine := string(processedBotTemplate)
		bottomLine = strings.ReplaceAll(bottomLine, "^PAGE", strconv.Itoa(currentPage))
		bottomLine = strings.ReplaceAll(bottomLine, "^TOTALPAGES", strconv.Itoa(calcTotalPages))
		if err := terminalio.WriteProcessedBytes(terminal, []byte(bottomLine), outputMode); err != nil {
			return err
		}

		// Status line.
		statusLine := "\r\n|08[|15Up/Dn|08] Navigate  [|15Space|08] Mark  [|15I|08]nfo  [|15V|08]iew  [|15D|08]ownload  [|15U|08]pload  [|15Q|08]uit"
		if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(statusLine)), outputMode); err != nil {
			return err
		}

		return nil
	}

	for {
		clampSelection()
		if err := render(); err != nil {
			return nil, "", err
		}

		key, err := readKeySequence(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", err
		}

		switch key {
		// Arrow keys and navigation.
		case "\x1b[A": // Up
			selectedIndex--
		case "\x1b[B": // Down
			selectedIndex++
		case "\x1b[5~": // Page Up
			selectedIndex -= visibleRows
		case "\x1b[6~": // Page Down
			selectedIndex += visibleRows
		case "\x1b[H", "\x1b[1~": // Home
			selectedIndex = 0
		case "\x1b[F", "\x1b[4~": // End
			if len(allFiles) > 0 {
				selectedIndex = len(allFiles) - 1
			}
		case "\x1b": // Bare Esc
			return nil, "", nil

		case " ", "\r", "\n": // Space or Enter: toggle mark
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

		case "q", "Q":
			return nil, "", nil

		case "i", "I": // Info: show file detail overlay
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
				_, _ = readKeySequence(reader)
			}

		case "v", "V":
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
					viewFileByRecord(e, s, terminal, sel, outputMode)
				}
				// Hide cursor again.
				_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
			}

		case "d", "D":
			if len(currentUser.TaggedFileIDs) == 0 {
				msg := "\r\n|07No files marked for download. Use |15Space|07 to mark files.|07\r\n"
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(1 * time.Second)
				continue
			}

			confirmPrompt := fmt.Sprintf("Download %d marked file(s)?", len(currentUser.TaggedFileIDs))
			_ = terminalio.WriteProcessedBytes(terminal, []byte("\r\n\x1b[K"), outputMode)
			_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode)

			proceed, promptErr := e.promptYesNo(s, terminal, confirmPrompt, outputMode, nodeNumber)
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
				log.Printf("INFO: Node %d: Attempting ZMODEM transfer for files: %v", nodeNumber, filenamesOnly)
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|15Initiating ZMODEM transfer (sz)...\r\n")), outputMode)
				_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|07Please start the ZMODEM receive function in your terminal.\r\n")), outputMode)

				szPath, lookErr := exec.LookPath("sz")
				if lookErr != nil {
					log.Printf("ERROR: Node %d: 'sz' command not found in PATH: %v", nodeNumber, lookErr)
					_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|01Error: 'sz' command not found on server. Cannot start download.\r\n")), outputMode)
					failCount = len(filesToDownload)
				} else {
					args := []string{"-b", "-e"}
					args = append(args, filesToDownload...)
					cmd := exec.Command(szPath, args...)
					log.Printf("INFO: Node %d: Executing Zmodem send: %s %v", nodeNumber, szPath, args)

					transferErr := transfer.RunCommandWithPTY(s, cmd)
					if transferErr != nil {
						log.Printf("ERROR: Node %d: 'sz' command execution failed: %v", nodeNumber, transferErr)
						_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|01ZMODEM transfer failed or was cancelled.\r\n")), outputMode)
						failCount = len(filesToDownload)
						successCount = 0
					} else {
						log.Printf("INFO: Node %d: 'sz' command completed successfully.", nodeNumber)
						_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|07ZMODEM transfer complete.\r\n")), outputMode)
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

		case "u", "U":
			_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode)
			uploadErr := e.runUploadFiles(s, terminal, currentUser, userManager, currentAreaID, currentAreaTag, outputMode, nodeNumber, sessionStartTime)
			if uploadErr != nil {
				log.Printf("ERROR: Node %d: Upload error: %v", nodeNumber, uploadErr)
			}
			// Refresh file list after upload.
			allFiles = e.FileMgr.GetFilesForArea(currentAreaID)
			if selectedIndex >= len(allFiles) && len(allFiles) > 0 {
				selectedIndex = len(allFiles) - 1
			}
			_ = terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
		}
	}
}
