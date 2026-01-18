package menu

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/stlalpha/vision3/internal/file"
	"github.com/stlalpha/vision3/internal/logging"
	terminalPkg "github.com/stlalpha/vision3/internal/terminal"
	"github.com/stlalpha/vision3/internal/transfer"
	"github.com/stlalpha/vision3/internal/user"
)

func runListFiles(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	logging.Debug("Node %d: Running LISTFILES", nodeNumber)

	// 1. Check User and Current File Area
	if currentUser == nil {
		log.Printf("WARN: Node %d: LISTFILES called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to list files.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	// Get current file area from user session
	currentAreaID := currentUser.CurrentFileAreaID
	currentAreaTag := currentUser.CurrentFileAreaTag

	if currentAreaID <= 0 {
		log.Printf("WARN: Node %d: User %s has no current file area selected.", nodeNumber, currentUser.Handle)
		msg := "\r\n|01Error: No file area selected.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	log.Printf("INFO: Node %d: User %s listing files for Area ID %d (%s)", nodeNumber, currentUser.Handle, currentAreaID, currentAreaTag)

	// Check Read ACS for the file area
	area, exists := e.FileMgr.GetAreaByID(currentAreaID)
	if !exists || !checkACS(area.ACSList, currentUser, s, terminal, sessionStartTime) {
		log.Printf("WARN: Node %d: User %s denied read access to file area %d (%s) due to ACS '%s'", nodeNumber, currentUser.Handle, currentAreaID, currentAreaTag, area.ACSList)
		// Display error message
		return nil, "", nil // Return to menu
	}

	// 2. Load Templates (FILELIST.TOP, FILELIST.MID, FILELIST.BOT)
	topTemplatePath := filepath.Join(e.MenuSetPath, "templates", "FILELIST.TOP")
	midTemplatePath := filepath.Join(e.MenuSetPath, "templates", "FILELIST.MID")
	botTemplatePath := filepath.Join(e.MenuSetPath, "templates", "FILELIST.BOT")

	topTemplateBytes, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplateBytes, errBot := os.ReadFile(botTemplatePath)

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load one or more FILELIST template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading File List screen templates.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading FILELIST templates")
	}

	processedTopTemplate := string(terminal.ProcessPipeCodes(topTemplateBytes))
	processedMidTemplate := string(terminal.ProcessPipeCodes(midTemplateBytes))
	processedBotTemplate := string(terminal.ProcessPipeCodes(botTemplateBytes))

	// 3. Fetch Files and Pagination Logic
	// --- Determine lines available per page ---
	termWidth := 80  // Default width
	termHeight := 24 // Default height
	ptyReq, _, isPty := s.Pty()
	if isPty && ptyReq.Window.Width > 0 && ptyReq.Window.Height > 0 {
		termWidth = ptyReq.Window.Width // Use actual width later for wrapping/truncating if needed
		termHeight = ptyReq.Window.Height
	} else {
		log.Printf("WARN: Node %d: Could not get PTY dimensions for file list, using default %dx%d", nodeNumber, termWidth, termHeight)
	}

	// Estimate lines used by header, footer, prompt
	headerLines := bytes.Count([]byte(processedTopTemplate), []byte("\n")) + 1
	footerLines := bytes.Count([]byte(processedBotTemplate), []byte("\n")) + 1
	// TODO: Make prompt configurable and count its lines accurately
	promptLines := 2 // Estimate 2 lines for prompt + input line
	fixedLines := headerLines + footerLines + promptLines
	filesPerPage := termHeight - fixedLines
	if filesPerPage < 1 {
		filesPerPage = 1 // Ensure at least 1 file can be shown
	}
	logging.Debug("Node %d: TermHeight=%d, FixedLines=%d, FilesPerPage=%d", nodeNumber, termHeight, fixedLines, filesPerPage)

	// --- Get Total File Count ---
	// TODO: Implement GetFileCountForArea in FileManager
	totalFiles, err := e.FileMgr.GetFileCountForArea(currentAreaID)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to get file count for area %d: %v", nodeNumber, currentAreaID, err)
		msg := fmt.Sprintf("\r\n|01Error retrieving file list for area '%s'.|07\r\n", currentAreaTag)
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed getting file count: %w", err)
	}

	totalPages := 0
	if totalFiles > 0 {
		totalPages = (totalFiles + filesPerPage - 1) / filesPerPage
	}
	if totalPages == 0 { // Ensure at least one page even if no files
		totalPages = 1
	}

	currentPage := 1                  // Start on page 1
	var filesOnPage []file.FileRecord // Use actual type from file package

	// --- Fetch Initial Page ---
	if totalFiles > 0 {
		// TODO: Implement GetFilesForAreaPaginated in FileManager
		filesOnPage, err = e.FileMgr.GetFilesForAreaPaginated(currentAreaID, currentPage, filesPerPage)
		if err != nil {
			log.Printf("ERROR: Node %d: Failed to get files for area %d, page %d: %v", nodeNumber, currentAreaID, currentPage, err)
			msg := fmt.Sprintf("\r\n|01Error retrieving file list page for area '%s'.|07\r\n", currentAreaTag)
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return nil, "", fmt.Errorf("failed getting file page: %w", err)
		}
	} else {
		filesOnPage = []file.FileRecord{} // Ensure empty slice if no files
	}

	// 4. Display Loop
	for {
		// 4.1 Clear Screen
		writeErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
		if writeErr != nil {
			log.Printf("ERROR: Node %d: Failed clearing screen for LISTFILES: %v", nodeNumber, writeErr)
			// Potentially return error or try to continue
		}

		// 4.2 Display Top Template
		_, wErr := terminal.Write([]byte(processedTopTemplate))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing LISTFILES top template: %v", nodeNumber, wErr)
			// Handle error
		}
		// Add CRLF after TOP template before listing files
		_, wErr = terminal.Write([]byte("\r\n"))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing CRLF after LISTFILES top template: %v", nodeNumber, wErr)
			// Handle error
		}

		// 4.3 Display Files on Current Page (using MID template)
		if len(filesOnPage) == 0 {
			// Display "No files in this area" message
			// TODO: Use a configurable string?
			noFilesMsg := "\r\n|07   No files in this area.   \r\n"
			wErr = terminal.DisplayContent([]byte(noFilesMsg))
			if wErr != nil { /* Log? */
			}
		} else {
			for i, fileRec := range filesOnPage {
				line := processedMidTemplate
				// Calculate display number for the file on the current page
				fileNumOnPage := (currentPage-1)*filesPerPage + i + 1

				// Populate placeholders (^MARK, ^NUM, ^NAME, ^DATE, ^SIZE, ^DESC) from fileRec
				fileNumStr := strconv.Itoa(fileNumOnPage)
				// Truncate filename and description if needed based on termWidth
				fileNameStr := fileRec.Filename                  // TODO: Truncate
				dateStr := fileRec.UploadedAt.Format("01/02/06") // Example date format
				sizeStr := fmt.Sprintf("%dk", fileRec.Size/1024) // Example size in K
				descStr := fileRec.Description                   // TODO: Truncate

				// Check if file is marked for download
				markStr := " " // Default to blank
				// Assumes currentUser.TaggedFileIDs is a slice of uuid.UUID
				if currentUser.TaggedFileIDs != nil {
					for _, taggedID := range currentUser.TaggedFileIDs {
						if taggedID == fileRec.ID {
							markStr = "*" // Or use a configured marker string
							break
						}
					}
				}

				line = strings.ReplaceAll(line, "^MARK", markStr)
				line = strings.ReplaceAll(line, "^NUM", fileNumStr)
				line = strings.ReplaceAll(line, "^NAME", fileNameStr)
				line = strings.ReplaceAll(line, "^DATE", dateStr)
				line = strings.ReplaceAll(line, "^SIZE", sizeStr)
				line = strings.ReplaceAll(line, "^DESC", descStr)

				// Write line using manual encoding helper in case of CP437 chars in data
				wErr = writeProcessedStringWithManualEncoding(terminal, []byte(line), outputMode)
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing file list line %d: %v", nodeNumber, i, wErr)
					// Handle error (e.g., break loop, return error?)
				}
				// Add CRLF after writing the line
				_, wErr = terminal.Write([]byte("\r\n"))
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing CRLF after file list line %d: %v", nodeNumber, i, wErr)
					// Handle error
				}
			}
		}

		// 4.4 Display Bottom Template (with pagination info)
		bottomLine := string(processedBotTemplate)
		bottomLine = strings.ReplaceAll(bottomLine, "^PAGE", strconv.Itoa(currentPage))
		bottomLine = strings.ReplaceAll(bottomLine, "^TOTALPAGES", strconv.Itoa(totalPages))
		_, wErr = terminal.Write([]byte(bottomLine))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing LISTFILES bottom template: %v", nodeNumber, wErr)
			// Handle error
		}

		// 4.5 Display Prompt (Use a standard file list prompt or configure one)
		// TODO: Use configurable prompt string
		prompt := "\r\n|07File Cmd (|15N|07=Next, |15P|07=Prev, |15#|07=Mark, |15D|07=Download, |15Q|07=Quit): |15"
		wErr = terminal.DisplayContent([]byte(prompt))
		if wErr != nil {
			// Handle error
		}

		// 4.6 Read User Input
		input, err := terminal.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during LISTFILES.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading LISTFILES input: %v", nodeNumber, err)
			// Consider retry or exit
			return nil, "", err
		}

		upperInput := strings.ToUpper(strings.TrimSpace(input))

		// 4.7 Process Input
		switch upperInput {
		case "N", " ", "": // Next Page (Space/Enter default to Next)
			if currentPage < totalPages {
				currentPage++
				// Fetch files for the new page
				filesOnPage, err = e.FileMgr.GetFilesForAreaPaginated(currentAreaID, currentPage, filesPerPage)
				if err != nil {
					// Log error and potentially return or break the loop
					log.Printf("ERROR: Node %d: Failed to get files for page %d: %v", nodeNumber, currentPage, err)
					// Display error message to user?
					time.Sleep(1 * time.Second)
				}
			} else {
				// Indicate last page (optional feedback)
				terminal.Write([]byte("\r\n|07Already on last page.|07"))
				time.Sleep(500 * time.Millisecond)
			}
			continue // Redraw loop
		case "P": // Previous Page
			if currentPage > 1 {
				currentPage--
				// Fetch files for the new page
				filesOnPage, err = e.FileMgr.GetFilesForAreaPaginated(currentAreaID, currentPage, filesPerPage)
				if err != nil {
					// Log error and potentially return or break the loop
					log.Printf("ERROR: Node %d: Failed to get files for page %d: %v", nodeNumber, currentPage, err)
					// Display error message to user?
					time.Sleep(1 * time.Second)
				}
			} else {
				// Indicate first page (optional feedback)
				terminal.Write([]byte("\r\n|07Already on first page.|07"))
				time.Sleep(500 * time.Millisecond)
			}
			continue // Redraw loop
		case "Q": // Quit
			logging.Debug("Node %d: User quit LISTFILES.", nodeNumber)
			return nil, "", nil // Return to FILEM menu
		case "D": // Download marked files
			logging.Debug("Node %d: User %s initiated Download command in area %d.", nodeNumber, currentUser.Handle, currentAreaID)

			// 1. Check if any files are marked
			if len(currentUser.TaggedFileIDs) == 0 {
				msg := "\\r\\n|07No files marked for download. Use |15#|07 to mark files.|07\\r\\n"
				wErr := terminal.DisplayContent([]byte(msg))
				if wErr != nil { /* Log? */
				}
				time.Sleep(1 * time.Second)
				continue // Go back to file list display
			}

			// 2. Confirm download
			confirmPrompt := fmt.Sprintf("Download %d marked file(s)?", len(currentUser.TaggedFileIDs))
			// Use WriteProcessedBytes for SaveCursor, positioning, and clear line
			// Need to position this prompt carefully, perhaps near the bottom prompt line.
			// For now, just display it after the main prompt. TODO: Improve positioning.
			terminal.Write([]byte(terminalPkg.SaveCursor()))
			terminal.Write([]byte("\\r\\n\\x1b[K")) // Newline, clear line

			proceed, err := e.promptYesNoLightbar(s, terminal, confirmPrompt, outputMode, nodeNumber)
			terminal.Write([]byte(terminalPkg.RestoreCursor())) // Restore cursor after prompt

			if err != nil {
				if errors.Is(err, io.EOF) {
					log.Printf("INFO: Node %d: User disconnected during download confirmation.", nodeNumber)
					return nil, "LOGOFF", io.EOF
				}
				log.Printf("ERROR: Node %d: Error getting download confirmation: %v", nodeNumber, err)
				msg := "\\r\\n|01Error during confirmation.|07\\r\\n"
				terminal.DisplayContent([]byte(msg))
				time.Sleep(1 * time.Second)
				continue // Back to file list
			}

			if !proceed {
				logging.Debug("Node %d: User cancelled download.", nodeNumber)
				terminal.Write([]byte("\\r\\n|07Download cancelled.|07"))
				time.Sleep(500 * time.Millisecond)
				continue // Back to file list
			}

			// 3. Process downloads
			log.Printf("INFO: Node %d: User %s starting download of %d files.", nodeNumber, currentUser.Handle, len(currentUser.TaggedFileIDs))
			terminal.Write([]byte("\\r\\n|07Preparing download...\\r\\n"))
			time.Sleep(500 * time.Millisecond) // Small pause

			successCount := 0
			failCount := 0
			filesToDownload := make([]string, 0, len(currentUser.TaggedFileIDs))
			filenamesOnly := make([]string, 0, len(currentUser.TaggedFileIDs))

			for _, fileID := range currentUser.TaggedFileIDs {
				filePath, pathErr := e.FileMgr.GetFilePath(fileID)
				if pathErr != nil {
					log.Printf("ERROR: Node %d: Failed to get path for file ID %s: %v", nodeNumber, fileID, pathErr)
					failCount++
					continue // Skip this file
				}
				// Check if file exists before adding to list
				if _, statErr := os.Stat(filePath); os.IsNotExist(statErr) {
					log.Printf("ERROR: Node %d: File path %s for ID %s does not exist on server.", nodeNumber, filePath, fileID)
					failCount++
					continue
				} else if statErr != nil {
					log.Printf("ERROR: Node %d: Error stating file path %s for ID %s: %v", nodeNumber, filePath, fileID, statErr)
					failCount++
					continue
				}
				filesToDownload = append(filesToDownload, filePath)
				filenamesOnly = append(filenamesOnly, filepath.Base(filePath))
			}

			if len(filesToDownload) > 0 {
				// **** Actual ZMODEM Transfer using sz ****
				log.Printf("INFO: Node %d: Attempting ZMODEM transfer for files: %v", nodeNumber, filenamesOnly)
				terminal.Write([]byte("|15Initiating ZMODEM transfer...\\r\\n"))
				terminal.Write([]byte("|07Please start the ZMODEM receive function in your terminal.\\r\\n"))

				// Use the unified protocol manager for ZMODEM transfer
				pm := transfer.NewProtocolManager()
				transferErr := pm.SendFiles(s, "ZMODEM", filesToDownload...)

				// Handle Result
				if transferErr != nil {
					// Transfer failed or was cancelled
					log.Printf("ERROR: Node %d: ZMODEM transfer failed: %v", nodeNumber, transferErr)
					terminal.Write([]byte("|01ZMODEM transfer failed or was cancelled.\\r\\n"))
					failCount = len(filesToDownload) // Assume all failed if transfer returns error
					successCount = 0
				} else {
					// Transfer completed successfully
					log.Printf("INFO: Node %d: ZMODEM transfer completed successfully.", nodeNumber)
					terminal.Write([]byte("|07ZMODEM transfer complete.\\r\\n"))
					successCount = len(filesToDownload) // Assume all succeeded if transfer exits cleanly
					failCount = 0

					// Increment download counts only on successful transfer completion
					for _, fileID := range currentUser.TaggedFileIDs {
						// Check again if we had a valid path originally
						if _, pathErr := e.FileMgr.GetFilePath(fileID); pathErr == nil {
							if err := e.FileMgr.IncrementDownloadCount(fileID); err != nil {
								log.Printf("WARN: Node %d: Failed to increment download count for file %s after successful transfer: %v", nodeNumber, fileID, err)
							} else {
								logging.Debug("Node %d: Incremented download count for file %s after successful transfer.", nodeNumber, fileID)
							}
						}
					}
				}
				// Add a small delay after transfer attempt
				time.Sleep(1 * time.Second)
				// ---- End ZMODEM Transfer ----

			} else {
				log.Printf("WARN: Node %d: No valid file paths found for tagged files.", nodeNumber)
				msg := "\\r\\n|01Could not find any of the marked files on the server.|07\\r\\n"
				terminal.DisplayContent([]byte(msg))
				failCount = len(currentUser.TaggedFileIDs) // Mark all as failed if none were found
			}

			// 4. Clear tags and save user state
			logging.Debug("Node %d: Clearing %d tagged file IDs for user %s.", nodeNumber, len(currentUser.TaggedFileIDs), currentUser.Handle)
			currentUser.TaggedFileIDs = nil // Clear the list
			if err := userManager.SaveUsers(); err != nil {
				log.Printf("ERROR: Node %d: Failed to save user data after download attempt: %v", nodeNumber, err)
				// Inform user? State might be inconsistent.
				terminal.Write([]byte("\\r\\n|01Error saving user state after download.|07"))
			}

			// 5. Final status message
			statusMsg := fmt.Sprintf("|07Download attempt finished. Success: %d, Failed: %d.|07\r\n", successCount, failCount)
			terminal.Write([]byte(statusMsg))
			time.Sleep(2 * time.Second)

			// Go back to the file list (will redraw with cleared marks)
			continue
		case "U": // Upload (Placeholder)
			logging.Debug("Node %d: Upload command entered (Not Implemented)", nodeNumber)
			msg := "\r\n|01Upload function not yet implemented.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			// Stay on the same page
		case "V": // View (Placeholder)
			logging.Debug("Node %d: View command entered (Not Implemented)", nodeNumber)
			msg := "\r\n|01View function not yet implemented.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			// Stay on the same page
		case "A": // Area Change (Placeholder/Not implemented here, handled by menu?)
			logging.Debug("Node %d: Area Change command entered (Handled by menu)", nodeNumber)
			msg := "\r\n|01Use menu options to change area.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
		default: // Includes 'T' (Tagging) and potential numeric input
			// Try to parse as a number for tagging
			fileNumToTag, err := strconv.Atoi(upperInput)
			if err == nil && fileNumToTag > 0 {
				// Valid number entered, attempt to tag/untag
				fileIndex := fileNumToTag - 1 - (currentPage-1)*filesPerPage
				if fileIndex >= 0 && fileIndex < len(filesOnPage) {
					fileToToggle := filesOnPage[fileIndex]
					found := false
					newTaggedIDs := []uuid.UUID{}
					if currentUser.TaggedFileIDs != nil {
						for _, taggedID := range currentUser.TaggedFileIDs {
							if taggedID == fileToToggle.ID {
								found = true // Mark as found to skip adding it back
							} else {
								newTaggedIDs = append(newTaggedIDs, taggedID)
							}
						}
					}
					if !found {
						// File was not tagged, so add it
						newTaggedIDs = append(newTaggedIDs, fileToToggle.ID)
						logging.Debug("Node %d: User %s tagged file #%d (ID: %s)", nodeNumber, currentUser.Handle, fileNumToTag, fileToToggle.ID)
					} else {
						// File was tagged, so we removed it (untagged)
						logging.Debug("Node %d: User %s untagged file #%d (ID: %s)", nodeNumber, currentUser.Handle, fileNumToTag, fileToToggle.ID)
					}
					currentUser.TaggedFileIDs = newTaggedIDs
					// No page change needed, loop will redraw with updated marks
				} else {
					// Invalid file number for current page
					logging.Debug("Node %d: Invalid file number entered: %d", nodeNumber, fileNumToTag)
					// Optional: Add user feedback message
				}
			} else {
				// Input was not N, P, Q, D, U, V, A, or a valid number - Invalid command
				logging.Debug("Node %d: Invalid command entered in LISTFILES: %s", nodeNumber, upperInput)
				// Optional: Add user feedback message
			}
		} // end switch
	} // end for loop

	// Should not be reached normally
	// return nil, "", nil
}

// displayFileAreaList is an internal helper to display the list of accessible file areas.
// It does not include a pause prompt.
func displayFileAreaList(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, currentUser *user.User, outputMode terminalPkg.OutputMode, nodeNumber int, sessionStartTime time.Time) error {
	logging.Debug("Node %d: Displaying file area list (helper)", nodeNumber)

	// 1. Define Template filenames and paths
	topTemplateFilename := "FILEAREA.TOP"
	midTemplateFilename := "FILEAREA.MID"
	botTemplateFilename := "FILEAREA.BOT"
	templateDir := filepath.Join(e.MenuSetPath, "templates")
	topTemplatePath := filepath.Join(templateDir, topTemplateFilename)
	midTemplatePath := filepath.Join(templateDir, midTemplateFilename)
	botTemplatePath := filepath.Join(templateDir, botTemplateFilename)

	// 2. Load Template Files
	topTemplateBytes, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplateBytes, errBot := os.ReadFile(botTemplatePath)

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load one or more FILEAREA template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		// Display error message to terminal
		msg := "\r\n|01Error loading File Area screen templates.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return fmt.Errorf("failed loading FILEAREA templates")
	}

	// 3. Process Pipe Codes in Templates FIRST
	processedTopTemplate := string(terminal.ProcessPipeCodes(topTemplateBytes))
	processedMidTemplate := string(terminal.ProcessPipeCodes(midTemplateBytes))
	processedBotTemplate := string(terminal.ProcessPipeCodes(botTemplateBytes))

	// 4. Get file area list data from FileManager
	areas := e.FileMgr.ListAreas()

	// 5. Build the output string using processed templates and data
	var outputBuffer bytes.Buffer
	outputBuffer.Write([]byte(processedTopTemplate)) // Write processed top template

	areasDisplayed := 0
	if len(areas) > 0 {
		for _, area := range areas {
			if !checkACS(area.ACSList, currentUser, s, terminal, sessionStartTime) {
				log.Printf("TRACE: Node %d: User %s denied list access to file area %d (%s) due to ACS '%s'", nodeNumber, currentUser.Handle, area.ID, area.Tag, area.ACSList)
				continue
			}

			line := processedMidTemplate
			name := string(terminal.ProcessPipeCodes([]byte(area.Name)))
			desc := string(terminal.ProcessPipeCodes([]byte(area.Description)))
			idStr := strconv.Itoa(area.ID)
			tag := string(terminal.ProcessPipeCodes([]byte(area.Tag)))
			fileCount, countErr := e.FileMgr.GetFileCountForArea(area.ID)
			if countErr != nil {
				log.Printf("WARN: Node %d: Failed getting file count for area %d (%s): %v", nodeNumber, area.ID, area.Tag, countErr)
				fileCount = 0
			}
			fileCountStr := strconv.Itoa(fileCount)

			line = strings.ReplaceAll(line, "^ID", idStr)
			line = strings.ReplaceAll(line, "^TAG", tag)
			line = strings.ReplaceAll(line, "^NA", name)
			line = strings.ReplaceAll(line, "^DS", desc)
			line = strings.ReplaceAll(line, "^NF", fileCountStr)

			outputBuffer.WriteString(line)
			areasDisplayed++
		}
	}

	if areasDisplayed == 0 {
		logging.Debug("Node %d: No accessible file areas to display for user %s.", nodeNumber, currentUser.Handle)
		outputBuffer.WriteString("\r\n|07   No accessible file areas found.   \r\n")
	}

	outputBuffer.Write([]byte(processedBotTemplate))

	// 6. Clear screen and display the assembled content
	writeErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
	if writeErr != nil {
		log.Printf("ERROR: Node %d: Failed clearing screen for file area list: %v", nodeNumber, writeErr)
		// Try to continue anyway?
	}

	processedContent := outputBuffer.Bytes()
	_, wErr := terminal.Write(processedContent)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing file area list output: %v", nodeNumber, wErr)
		return wErr // Return the error from writing
	}

	return nil // Success
}

// runListFileAreas displays a list of file areas using templates.
func runListFileAreas(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	logging.Debug("Node %d: Running LISTFILEAR", nodeNumber)

	if currentUser == nil {
		log.Printf("WARN: Node %d: LISTFILEAR called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to list file areas.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Call the helper to display the list
	if err := displayFileAreaList(e, s, terminal, currentUser, outputMode, nodeNumber, sessionStartTime); err != nil {
		// Error already logged by helper, maybe add context?
		log.Printf("ERROR: Node %d: Error occurred during displayFileAreaList from runListFileAreas: %v", nodeNumber, err)
		// Need to decide if we still pause or just return.
		// For now, return the error to prevent pause on failed display.
		return nil, "", err
	}

	// Wait for Enter using configured PauseString
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... " // Fallback
	}

	var pauseBytesToWrite []byte
	processedPausePrompt := string(terminal.ProcessPipeCodes([]byte(pausePrompt)))
	if outputMode == terminalPkg.OutputModeCP437 {
		var cp437Buf bytes.Buffer
		for _, r := range string(processedPausePrompt) {
			if r < 128 {
				cp437Buf.WriteByte(byte(r))
			} else if cp437Byte, ok := terminalPkg.UnicodeToCP437Table[r]; ok {
				cp437Buf.WriteByte(cp437Byte)
			} else {
				cp437Buf.WriteByte('?')
			}
		}
		pauseBytesToWrite = cp437Buf.Bytes()
	} else {
		pauseBytesToWrite = []byte(processedPausePrompt)
	}

	_, wErr := terminal.Write(pauseBytesToWrite)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing LISTFILEAR pause prompt: %v", nodeNumber, wErr)
	}

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during LISTFILEAR pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading input during LISTFILEAR pause: %v", nodeNumber, err)
			return nil, "", err
		}
		if r == '\r' || r == '\n' {
			break
		}
	}

	return nil, "", nil // Success, return to current menu (FILEM)
}

// runSelectFileArea prompts the user for a file area tag and changes the current user's
// active file area if valid and accessible.
func runSelectFileArea(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	logging.Debug("Node %d: Running SELECTFILEAREA", nodeNumber)

	if currentUser == nil {
		log.Printf("WARN: Node %d: SELECTFILEAREA called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to select a file area.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// --- Display the list first --- <--- MODIFIED
	if err := displayFileAreaList(e, s, terminal, currentUser, outputMode, nodeNumber, sessionStartTime); err != nil {
		log.Printf("ERROR: Node %d: Failed displaying file area list in SELECTFILEAREA: %v", nodeNumber, err)
		// Don't proceed if the list couldn't be displayed
		return currentUser, "", err // Return error
	}
	// Add a newline between list and prompt
	terminal.Write([]byte("\r\n"))

	// Prompt for area tag
	prompt := e.LoadedStrings.ChangeFileAreaStr
	if prompt == "" {
		prompt = "|07File Area Tag (?=List, Q=Quit): |15" // Updated prompt slightly
	}
	wErr := terminal.DisplayContent([]byte(prompt))
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing SELECTFILEAREA prompt: %v", nodeNumber, wErr)
		// Return to menu, maybe signal error?
		return currentUser, "", nil
	}

	inputTag, err := terminal.ReadLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during SELECTFILEAREA prompt.", nodeNumber)
			return nil, "LOGOFF", io.EOF // Signal logoff
		}
		log.Printf("ERROR: Node %d: Error reading input for SELECTFILEAREA: %v", nodeNumber, err)
		return currentUser, "", err // Return error
	}

	inputClean := strings.TrimSpace(inputTag) // Keep original case for tag lookup if needed
	upperInput := strings.ToUpper(inputClean)

	if upperInput == "" || upperInput == "Q" { // Allow Q to quit
		logging.Debug("Node %d: SELECTFILEAREA aborted by user.", nodeNumber)
		terminal.Write([]byte("\r\n")) // Newline after abort
		return currentUser, "", nil    // Return to previous menu
	}

	if upperInput == "?" { // Handle request for list (? loops back here after display)
		logging.Debug("Node %d: User requested file area list again from SELECTFILEAREA.", nodeNumber)
		// Simply loop back by returning nil, which will re-run this function
		// which now starts by displaying the list again.
		return currentUser, "", nil
	}

	// --- NEW: Try parsing as ID first, then fallback to Tag ---
	var area *file.FileArea
	var exists bool

	// Try parsing as ID
	if inputID, err := strconv.Atoi(inputClean); err == nil {
		logging.Debug("Node %d: User input '%s' parsed as ID %d. Looking up by ID.", nodeNumber, inputClean, inputID)
		area, exists = e.FileMgr.GetAreaByID(inputID)
		if !exists {
			log.Printf("WARN: Node %d: User %s entered non-existent file area ID: %d", nodeNumber, currentUser.Handle, inputID)
			msg := fmt.Sprintf("\r\n|01Error: File area ID '%d' not found.|07\r\n", inputID)
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return currentUser, "", nil // Return to menu
		}
	} else {
		// Not a valid ID, treat as Tag (use uppercase)
		logging.Debug("Node %d: User input '%s' not an ID. Looking up by Tag '%s'.", nodeNumber, inputClean, upperInput)
		area, exists = e.FileMgr.GetAreaByTag(upperInput)
		if !exists {
			log.Printf("WARN: Node %d: User %s entered non-existent file area tag: %s", nodeNumber, currentUser.Handle, upperInput)
			msg := fmt.Sprintf("\r\n|01Error: File area tag '%s' not found.|07\r\n", upperInput)
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return currentUser, "", nil // Return to menu
		}
	}

	// --- END NEW LOGIC ---

	// At this point, 'area' should be valid and 'exists' should be true

	// Check ACSList permission
	if !checkACS(area.ACSList, currentUser, s, terminal, sessionStartTime) {
		log.Printf("WARN: Node %d: User %s denied access to file area %d ('%s') due to ACS '%s'", nodeNumber, currentUser.Handle, area.ID, area.Tag, area.ACSList)
		msg := fmt.Sprintf("\r\n|01Error: Access denied to file area '%s'.|07\r\n", area.Tag)
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return currentUser, "", nil // Return to menu
	}

	// Success! Update user state
	currentUser.CurrentFileAreaID = area.ID
	currentUser.CurrentFileAreaTag = area.Tag

	// Save the user state (important!)
	if err := userManager.SaveUsers(); err != nil {
		log.Printf("ERROR: Node %d: Failed to save user data after updating file area: %v", nodeNumber, err)
		// Should we inform the user? Proceed anyway?
		msg := "\r\n|01Error: Could not save area selection.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		// Don't change area if save failed?
		// Or let it proceed and hope for next save?
		// For now, proceed but log the error.
	}

	log.Printf("INFO: Node %d: User %s changed file area to ID %d ('%s')", nodeNumber, currentUser.Handle, area.ID, area.Tag)
	msg := fmt.Sprintf("\r\n|07Current file area set to: |15%s|07\r\n", area.Name) // Use area name for confirmation
	wErr = terminal.DisplayContent([]byte(msg))                                    // <-- Use = instead of :=
	if wErr != nil {                                                               /* Log? */
	}
	time.Sleep(1 * time.Second)

	return currentUser, "", nil // Success, return to previous menu/state
}
