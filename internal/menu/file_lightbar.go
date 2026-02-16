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
	// Detail pane: separator + 5 info lines.
	detailPaneLines := 6
	// +1 for the CRLF after the top template write.
	visibleRows := termHeight - headerLines - 1 - footerLines - detailPaneLines
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

	// truncate limits a string to maxLen characters.
	truncate := func(s string, maxLen int) string {
		if len(s) <= maxLen {
			return s
		}
		return s[:maxLen]
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

		// File rows.
		endIndex := topIndex + visibleRows
		if len(allFiles) > 0 && endIndex > len(allFiles) {
			endIndex = len(allFiles)
		}

		if len(allFiles) == 0 {
			// Pad all visible rows, first one gets a message.
			msg := "|07   No files in this area."
			if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg+"\r\n")), outputMode); err != nil {
				return err
			}
			for i := 1; i < visibleRows; i++ {
				if err := terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode); err != nil {
					return err
				}
			}
		} else {
			for idx := topIndex; idx < endIndex; idx++ {
				fileRec := allFiles[idx]
				line := processedMidTemplate

				fileNumStr := strconv.Itoa(idx + 1)
				fileNameStr := fileRec.Filename
				dateStr := fileRec.UploadedAt.Format("01/02/06")
				sizeStr := formatSize(fileRec.Size)
				// Truncate description to prevent line wrapping.
				// Replace any embedded newlines with spaces first.
				descStr := strings.ReplaceAll(fileRec.Description, "\n", " ")
				descStr = strings.ReplaceAll(descStr, "\r", "")

				markStr := " "
				if isFileTagged(fileRec.ID) {
					markStr = "*"
				}

				line = strings.ReplaceAll(line, "^MARK", markStr)
				line = strings.ReplaceAll(line, "^NUM", fileNumStr)
				line = strings.ReplaceAll(line, "^NAME", fileNameStr)
				line = strings.ReplaceAll(line, "^DATE", dateStr)
				line = strings.ReplaceAll(line, "^SIZE", sizeStr)
				line = strings.ReplaceAll(line, "^DESC", descStr)

				// Process pipe codes, then measure visible length and truncate to terminal width.
				processed := string(ansi.ReplacePipeCodes([]byte(line)))
				visibleLen := len(stripAnsi(processed))
				if visibleLen > termWidth {
					// Truncate the visible text to terminal width.
					plain := stripAnsi(processed)
					descStr = truncate(descStr, len(descStr)-(visibleLen-termWidth))
					line = processedMidTemplate
					line = strings.ReplaceAll(line, "^MARK", markStr)
					line = strings.ReplaceAll(line, "^NUM", fileNumStr)
					line = strings.ReplaceAll(line, "^NAME", fileNameStr)
					line = strings.ReplaceAll(line, "^DATE", dateStr)
					line = strings.ReplaceAll(line, "^SIZE", sizeStr)
					line = strings.ReplaceAll(line, "^DESC", descStr)
					processed = string(ansi.ReplacePipeCodes([]byte(line)))
					_ = plain // used for length calculation
				}

				if idx == selectedIndex {
					// Strip ANSI colors so highlight color applies uniformly.
					plain := stripAnsi(processed)
					// Pad to terminal width so highlight bar spans the full row.
					if len(plain) < termWidth {
						plain += strings.Repeat(" ", termWidth-len(plain))
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
			}

			// Pad empty rows.
			for i := endIndex; i < topIndex+visibleRows; i++ {
				if err := terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode); err != nil {
					return err
				}
			}
		}

		// Separator line.
		sep := "|08" + strings.Repeat("-", 78) + "|07"
		if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(sep+"\r\n")), outputMode); err != nil {
			return err
		}

		// Detail pane.
		if len(allFiles) == 0 {
			if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("|07No files in this area.\r\n")), outputMode); err != nil {
				return err
			}
			for i := 0; i < 4; i++ {
				if err := terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode); err != nil {
					return err
				}
			}
		} else {
			sel := allFiles[selectedIndex]
			d1 := fmt.Sprintf("|15Filename  : |07%-40s |15Size : |07%s\r\n", sel.Filename, formatSize(sel.Size))
			d2 := fmt.Sprintf("|15Desc      : |07%s\r\n", sel.Description)
			d3 := fmt.Sprintf("|15Uploaded  : |07%-20s |15By   : |07%s\r\n", sel.UploadedAt.Format("01/02/2006 15:04"), sel.UploadedBy)
			d4 := fmt.Sprintf("|15Downloads : |07%d\r\n", sel.DownloadCount)
			d5 := "\r\n"
			for _, line := range []string{d1, d2, d3, d4, d5} {
				if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode); err != nil {
					return err
				}
			}
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
		statusLine := "\r\n|08[|15Up/Dn|08] Navigate  [|15Space|08] Mark  [|15V|08]iew  [|15D|08]ownload  [|15U|08]pload  [|15Q|08]uit"
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
