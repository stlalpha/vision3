package menu

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gliderlabs/ssh"
	"github.com/robbiew/vision3/internal/ansi"
	"github.com/robbiew/vision3/internal/editor"
	"github.com/robbiew/vision3/internal/message"
	"github.com/robbiew/vision3/internal/terminalio"
	"github.com/robbiew/vision3/internal/user"
	"golang.org/x/term"
)

// Default message header style if user hasn't selected one
const defaultMsgHdrStyle = 5

// Message reader navigation options (Pascal's 10-option bar)
var msgReaderOptions = []MsgLightbarOption{
	{Label: " Next ", HotKey: 'N'},
	{Label: " Reply ", HotKey: 'R'},
	{Label: " Again ", HotKey: 'A'},
	{Label: " Skip ", HotKey: 'S'},
	{Label: " Thread ", HotKey: 'T'},
	{Label: " Post ", HotKey: 'P'},
	{Label: " Jump ", HotKey: 'J'},
	{Label: " Mail ", HotKey: 'M'},
	{Label: " List ", HotKey: 'L'},
	{Label: " Quit ", HotKey: 'Q'},
}

// runMessageReader is the core message reading loop matching Pascal's Scanboard + Readcurbul.
// It displays messages using MSGHDR.<n> templates with DataFile substitution,
// shows a 10-option lightbar for navigation, and handles single-key input.
func runMessageReader(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, outputMode ansi.OutputMode,
	startMsg int, totalMsgCount int, isNewScan bool) (*user.User, string, error) {

	currentAreaID := currentUser.CurrentMessageAreaID
	currentAreaTag := currentUser.CurrentMessageAreaTag

	// Get conference and area names for display
	confName := "Local"
	areaName := currentAreaTag
	if currentUser.CurrentMsgConferenceID != 0 && e.ConferenceMgr != nil {
		if conf, found := e.ConferenceMgr.GetByID(currentUser.CurrentMsgConferenceID); found {
			confName = conf.Name
		}
	}
	if area, found := e.MessageMgr.GetAreaByID(currentAreaID); found {
		areaName = area.Name
	}

	// Determine message header style
	hdrStyle := currentUser.MsgHdr
	if hdrStyle < 1 || hdrStyle > 14 {
		hdrStyle = defaultMsgHdrStyle
	}

	// Load the MSGHDR template file
	hdrTemplatePath := filepath.Join(e.MenuSetPath, "templates", "message_headers",
		fmt.Sprintf("MSGHDR.%d", hdrStyle))
	hdrTemplateBytes, hdrErr := os.ReadFile(hdrTemplatePath)
	if hdrErr != nil {
		log.Printf("ERROR: Node %d: Failed to load MSGHDR.%d: %v", nodeNumber, hdrStyle, hdrErr)
		// Fallback to style 2 (simple text format)
		hdrTemplatePath = filepath.Join(e.MenuSetPath, "templates", "message_headers", "MSGHDR.2")
		hdrTemplateBytes, hdrErr = os.ReadFile(hdrTemplatePath)
		if hdrErr != nil {
			msg := "\r\n|01Error loading message header template.|07\r\n"
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(1 * time.Second)
			return nil, "", fmt.Errorf("failed loading MSGHDR templates")
		}
	}

	// Get terminal dimensions
	termWidth := 80
	termHeight := 24
	ptyReq, _, isPty := s.Pty()
	if isPty && ptyReq.Window.Width > 0 && ptyReq.Window.Height > 0 {
		termWidth = ptyReq.Window.Width
		termHeight = ptyReq.Window.Height
	}

	// Lightbar colors from theme
	hiColor := e.Theme.YesNoHighlightColor
	loColor := e.Theme.YesNoRegularColor

	currentMsgNum := startMsg
	quitNewscan := false

readerLoop:
	for {
		if currentMsgNum > totalMsgCount || currentMsgNum < 1 {
			break readerLoop
		}

		// Load current message
		currentMsg, msgErr := e.MessageMgr.GetMessage(currentAreaID, currentMsgNum)
		if msgErr != nil {
			log.Printf("ERROR: Node %d: Failed to read message %d in area %d: %v",
				nodeNumber, currentMsgNum, currentAreaID, msgErr)
			// Try next message
			currentMsgNum++
			continue
		}

		// Skip deleted messages
		if currentMsg.IsDeleted {
			currentMsgNum++
			continue
		}

		// Build Pascal-style substitution map
		substitutions := buildMsgSubstitutions(currentMsg, currentAreaTag, currentMsgNum, totalMsgCount)

		// Process template with substitutions
		processedHeader := processDataFile(hdrTemplateBytes, substitutions)

		// Process message body and pre-format all lines
		processedBodyStr := string(ansi.ReplacePipeCodes([]byte(currentMsg.Body)))
		wrappedBodyLines := wrapAnsiString(processedBodyStr, termWidth)

		// Add origin line for echo messages to the body lines
		area, _ := e.MessageMgr.GetAreaByID(currentAreaID)
		if area != nil && (area.AreaType == "echomail" || area.AreaType == "netmail") {
			if currentMsg.OrigAddr != "" {
				originLine := fmt.Sprintf("|08 * Origin: %s", currentMsg.OrigAddr)
				wrappedBodyLines = append(wrappedBodyLines, string(ansi.ReplacePipeCodes([]byte(originLine))))
			}
		}

		// Calculate available body height
		// Find the actual bottom row of the header using ANSI cursor tracking
		headerEndRow := findHeaderEndRow(processedHeader)
		bodyStartRow := headerEndRow
		barLines := 3 // Horizontal line + board info line + lightbar
		bodyAvailHeight := termHeight - bodyStartRow - barLines
		if bodyAvailHeight < 1 {
			bodyAvailHeight = 5
		}

		// Initialize scroll state for this message
		scrollOffset := 0
		totalBodyLines := len(wrappedBodyLines)
		needsRedraw := true

		// Update lastread when first displaying message
		if lrErr := e.MessageMgr.SetLastRead(currentAreaID, currentUser.Handle, currentMsgNum); lrErr != nil {
			log.Printf("ERROR: Node %d: Failed to update last read: %v", nodeNumber, lrErr)
		}

		// Inner loop for scrolling and command handling
	scrollLoop:
		for {
			var selectedKey rune // Declare here so it's available in all code paths

			// Redraw screen if needed
			if needsRedraw {
				// Clear screen and display header
				terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
				terminalio.WriteProcessedBytes(terminal, processedHeader, outputMode)

				// Display visible portion of message body using explicit cursor positioning
				for i := 0; i < bodyAvailHeight; i++ {
					lineNum := bodyStartRow + i
					// Position cursor at specific line
					terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(lineNum, 1)), outputMode)
					// Clear line
					terminalio.WriteProcessedBytes(terminal, []byte("\x1b[K"), outputMode)
					// Display line if available
					lineIdx := scrollOffset + i
					if lineIdx < totalBodyLines {
						terminalio.WriteProcessedBytes(terminal, []byte(wrappedBodyLines[lineIdx]), outputMode)
					}
				}

				// Display footer: board info and lightbar anchored to bottom
				var suffixText string
				if isNewScan {
					suffixText = " (NewScan)"
				} else {
					suffixText = " (Reading)"
				}

				// Add scroll percentage if message is longer than display area
				if totalBodyLines > bodyAvailHeight {
					maxScroll := totalBodyLines - bodyAvailHeight
					scrollPercent := 0
					if maxScroll > 0 {
						scrollPercent = (scrollOffset * 100) / maxScroll
						if scrollPercent > 100 {
							scrollPercent = 100
						}
					}
					suffixText = fmt.Sprintf("%s |05[|13%d%%|05]", suffixText, scrollPercent)
				}

				// Draw horizontal line above footer (CP437 character 196)
				terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(termHeight-2, 1)), outputMode)
				horizontalLine := "|08" + strings.Repeat("\xC4", termWidth-1) + "|07" // CP437 horizontal line character
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(horizontalLine)), outputMode)

				// Position cursor at second-to-last row for board info line
				terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(termHeight-1, 1)), outputMode)
				boardLine := fmt.Sprintf("|13%s |09> |15%s |01[|13%d|05/|13%d|01]",
					confName, areaName, currentMsgNum, totalMsgCount)
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(boardLine)), outputMode)

				// Position cursor at last row for lightbar
				terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(termHeight, 1)), outputMode)

				// Draw the lightbar menu
				drawMsgLightbarStatic(terminal, msgReaderOptions, outputMode, hiColor, loColor, suffixText, 0)

				needsRedraw = false
			}

			// Read key sequence (handles escape sequences for arrow keys, page up/down)
			keySeq, keyErr := readKeySequence(s, 3*time.Second)
			if keyErr != nil {
				if errors.Is(keyErr, io.EOF) {
					return nil, "LOGOFF", io.EOF
				}
				// Timeout is acceptable, just continue
				continue
			}

			// Handle scrolling keys first
			switch keySeq {
			case "\x1b[A": // Up arrow - scroll up one line
				if scrollOffset > 0 {
					scrollOffset--
					needsRedraw = true
				}
				continue

			case "\x1b[B": // Down arrow - scroll down one line
				if totalBodyLines > bodyAvailHeight && scrollOffset < totalBodyLines-bodyAvailHeight {
					scrollOffset++
					needsRedraw = true
				}
				continue

			case "\x1b[5~": // Page Up
				pageSize := bodyAvailHeight - 2
				if pageSize < 5 {
					pageSize = 5
				}
				scrollOffset -= pageSize
				if scrollOffset < 0 {
					scrollOffset = 0
				}
				needsRedraw = true
				continue

			case "\x1b[6~": // Page Down
				pageSize := bodyAvailHeight - 2
				if pageSize < 5 {
					pageSize = 5
				}
				scrollOffset += pageSize
				maxScroll := totalBodyLines - bodyAvailHeight
				if maxScroll < 0 {
					maxScroll = 0
				}
				if scrollOffset > maxScroll {
					scrollOffset = maxScroll
				}
				needsRedraw = true
				continue

			case "\x1b[D", "\x1b[C": // Left or Right arrow - activate interactive lightbar
				var suffixText string
				if isNewScan {
					suffixText = " (NewScan)"
				} else {
					suffixText = " (Reading)"
				}

				if totalBodyLines > bodyAvailHeight {
					maxScroll := totalBodyLines - bodyAvailHeight
					scrollPercent := 0
					if maxScroll > 0 {
						scrollPercent = (scrollOffset * 100) / maxScroll
						if scrollPercent > 100 {
							scrollPercent = 100
						}
					}
					suffixText = fmt.Sprintf("%s |05[|13%d%%|05]", suffixText, scrollPercent)
				}

				terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(termHeight, 1)), outputMode)

				// Determine initial direction based on which arrow was pressed
				initialDir := 0
				if keySeq == "\x1b[D" {
					initialDir = -1 // Left arrow
				} else if keySeq == "\x1b[C" {
					initialDir = 1 // Right arrow
				}

				selKey, lbErr := runMsgLightbar(s, terminal, msgReaderOptions, outputMode, hiColor, loColor, suffixText, initialDir)
				if lbErr != nil {
					if errors.Is(lbErr, io.EOF) {
						return nil, "LOGOFF", io.EOF
					}
					log.Printf("ERROR: Node %d: Lightbar error: %v", nodeNumber, lbErr)
					break readerLoop
				}
				selectedKey = rune(selKey)
				// Don't continue here - fall through to handle the selected command
			}

			// If not a scrolling key, show the lightbar for command selection
			// First handle simple single-key commands that bypass the lightbar
			if len(keySeq) == 1 {
				singleKey := rune(keySeq[0])
				// Check if it's a direct command key
				switch unicode.ToUpper(singleKey) {
				case 'N', 'R', 'A', 'S', 'T', 'P', 'J', 'M', 'L', 'Q', '?':
					selectedKey = unicode.ToUpper(singleKey)
				case '\r', '\n':
					selectedKey = 'N' // Enter = Next
				case '\x1b': // ESC = Quit
					selectedKey = 'Q'
				default:
					// Not a recognized command, show lightbar
					var suffixText string
					if isNewScan {
						suffixText = " (NewScan)"
					} else {
						suffixText = " (Reading)"
					}

					// Add scroll info to suffix
					if totalBodyLines > bodyAvailHeight {
						maxScroll := totalBodyLines - bodyAvailHeight
						scrollPercent := 0
						if maxScroll > 0 {
							scrollPercent = (scrollOffset * 100) / maxScroll
							if scrollPercent > 100 {
								scrollPercent = 100
							}
						}
						suffixText = fmt.Sprintf("%s |05[|13%d%%|05]", suffixText, scrollPercent)
					}

					// Position cursor at last row for lightbar
					terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(termHeight, 1)), outputMode)

					selKey, lbErr := runMsgLightbar(s, terminal, msgReaderOptions, outputMode, hiColor, loColor, suffixText, 0)
					if lbErr != nil {
						if errors.Is(lbErr, io.EOF) {
							return nil, "LOGOFF", io.EOF
						}
						log.Printf("ERROR: Node %d: Lightbar error: %v", nodeNumber, lbErr)
						break readerLoop
					}
					selectedKey = rune(selKey)
				}
			} else {
				// Multi-byte sequence that wasn't handled as scrolling - show lightbar
				var suffixText string
				if isNewScan {
					suffixText = " (NewScan)"
				} else {
					suffixText = " (Reading)"
				}

				if totalBodyLines > bodyAvailHeight {
					maxScroll := totalBodyLines - bodyAvailHeight
					scrollPercent := 0
					if maxScroll > 0 {
						scrollPercent = (scrollOffset * 100) / maxScroll
						if scrollPercent > 100 {
							scrollPercent = 100
						}
					}
					suffixText = fmt.Sprintf("%s |05[|13%d%%|05]", suffixText, scrollPercent)
				}

				terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(termHeight, 1)), outputMode)

				selKey, lbErr := runMsgLightbar(s, terminal, msgReaderOptions, outputMode, hiColor, loColor, suffixText, 0)
				if lbErr != nil {
					if errors.Is(lbErr, io.EOF) {
						return nil, "LOGOFF", io.EOF
					}
					log.Printf("ERROR: Node %d: Lightbar error: %v", nodeNumber, lbErr)
					break readerLoop
				}
				selectedKey = rune(selKey)
			}

			// Handle command from lightbar or direct key
			if selectedKey == 0 {
				continue
			}

			// Now handle message navigation commands
			// Handle the selected command
			switch selectedKey {
			case 'N': // Next message
				if currentMsgNum < totalMsgCount {
					currentMsgNum++
					break scrollLoop // Exit scroll loop to load next message
				} else {
					terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07End of messages.|07")), outputMode)
					time.Sleep(500 * time.Millisecond)
					break readerLoop
				}

			case 'A': // Again - redisplay current message
				// Reset scroll and redraw
				scrollOffset = 0
				needsRedraw = true
				continue

			case 'R': // Reply
				replyResult := handleReply(e, s, terminal, userManager, currentUser, nodeNumber,
					outputMode, currentMsg, currentAreaID, &totalMsgCount, &currentMsgNum)
				if replyResult == "LOGOFF" {
					return nil, "LOGOFF", io.EOF
				}
				// Redraw message after reply
				needsRedraw = true
				continue

			case 'P': // Post new message
				terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
				_, _, _ = runComposeMessage(e, s, terminal, userManager, currentUser, nodeNumber,
					sessionStartTime, "", outputMode)
				// Refresh total count
				newTotal, _ := e.MessageMgr.GetMessageCountForArea(currentAreaID)
				if newTotal > 0 {
					totalMsgCount = newTotal
				}
				// Redraw message after posting
				needsRedraw = true
				continue

			case 'S': // Skip - exit current area, return to caller
				break readerLoop

			case 'T': // Thread
				handleThread(e, s, terminal, outputMode, currentAreaID,
					&currentMsgNum, totalMsgCount, currentMsg.Subject)
				// Exit scroll loop to load new message if thread changed it
				break scrollLoop

			case 'J': // Jump to message number
				handleJump(s, terminal, outputMode, &currentMsgNum, totalMsgCount)
				// Exit scroll loop to load new message
				break scrollLoop

			case 'M': // Mail reply (deferred)
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07Mail reply not yet implemented.|07")), outputMode)
				time.Sleep(1 * time.Second)
				needsRedraw = true
				continue

			case 'L': // List titles (deferred)
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07Message list not yet implemented.|07")), outputMode)
				time.Sleep(1 * time.Second)
				needsRedraw = true
				continue

			case 'Q': // Quit
				quitNewscan = true
				break readerLoop

			case '?': // Help
				displayReaderHelp(terminal, outputMode)
				needsRedraw = true
				continue

			default:
				continue
			}
		}
	}

	// Update lastread on exit
	if currentMsgNum >= 1 && currentMsgNum <= totalMsgCount {
		if lrErr := e.MessageMgr.SetLastRead(currentAreaID, currentUser.Handle, currentMsgNum); lrErr != nil {
			log.Printf("ERROR: Node %d: Failed to update last read on exit: %v", nodeNumber, lrErr)
		}
	}

	if quitNewscan {
		return nil, "QUIT_NEWSCAN", nil
	}
	return nil, "", nil
}

// buildMsgSubstitutions creates the Pascal-style substitution map for MSGHDR templates.
func buildMsgSubstitutions(msg *message.DisplayMessage, areaTag string, msgNum, totalMsgs int) map[byte]string {
	toStr := msg.To
	// Note: We don't have a "Received" flag in DisplayMessage currently,
	// but this is where it would be appended like Pascal does

	replyStr := "None"
	if msg.ReplyID != "" {
		replyStr = msg.ReplyID
	}

	return map[byte]string{
		'B': areaTag,
		'T': msg.Subject,
		'F': msg.From,
		'S': toStr,
		'U': "", // Status - not available in JAM
		'L': "", // Post level - not available
		'R': "", // Real name - not available in JAM
		'#': strconv.Itoa(msgNum),
		'N': strconv.Itoa(totalMsgs),
		'D': msg.DateTime.Format("01/02/06"),
		'W': msg.DateTime.Format("3:04 pm"),
		'P': replyStr,
		'E': "0", // Replies count - not tracked in JAM
	}
}

// findHeaderEndRow parses processed ANSI bytes and tracks cursor position
// through ESC[row;colH commands and newlines to find the maximum row used.
// This correctly handles MSGHDR templates that use absolute cursor positioning.
func findHeaderEndRow(data []byte) int {
	maxRow := 1
	curRow := 1
	i := 0
	for i < len(data) {
		if data[i] == '\n' {
			curRow++
			if curRow > maxRow {
				maxRow = curRow
			}
			i++
			continue
		}
		// Check for ESC[ sequences
		if data[i] == 0x1B && i+1 < len(data) && data[i+1] == '[' {
			i += 2
			// Parse numeric params
			params := ""
			for i < len(data) && (data[i] == ';' || (data[i] >= '0' && data[i] <= '9')) {
				params += string(data[i])
				i++
			}
			if i < len(data) {
				cmd := data[i]
				i++
				if cmd == 'H' || cmd == 'f' { // Cursor position
					parts := strings.Split(params, ";")
					if len(parts) >= 1 && parts[0] != "" {
						row, err := strconv.Atoi(parts[0])
						if err == nil {
							curRow = row
							if curRow > maxRow {
								maxRow = curRow
							}
						}
					}
				}
				// Skip other commands - they don't change row
			}
			continue
		}
		i++
	}
	return maxRow
}

// findLastCursorPos finds the last ESC[row;colH cursor position command in data.
// Returns (row, col) or (0, 0) if none found.
func findLastCursorPos(data []byte) (int, int) {
	lastRow, lastCol := 0, 0
	i := 0
	for i < len(data) {
		if data[i] == 0x1B && i+1 < len(data) && data[i+1] == '[' {
			i += 2
			params := ""
			for i < len(data) && (data[i] == ';' || (data[i] >= '0' && data[i] <= '9')) {
				params += string(data[i])
				i++
			}
			if i < len(data) {
				cmd := data[i]
				i++
				if cmd == 'H' || cmd == 'f' {
					parts := strings.Split(params, ";")
					if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
						row, e1 := strconv.Atoi(parts[0])
						col, e2 := strconv.Atoi(parts[1])
						if e1 == nil && e2 == nil {
							lastRow = row
							lastCol = col
						}
					}
				}
			}
			continue
		}
		i++
	}
	return lastRow, lastCol
}

// handleReply manages the reply flow matching Pascal's reply handling.
func handleReply(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	outputMode ansi.OutputMode, currentMsg *message.DisplayMessage,
	currentAreaID int, totalMsgCount *int, currentMsgNum *int) string {

	// Get quote prefix
	quotePrefix := e.LoadedStrings.QuotePrefix
	if quotePrefix == "" {
		quotePrefix = "> "
	}

	// Format quoted text
	quotedBody := formatQuote(currentMsg, quotePrefix)

	// Get subject
	defaultSubject := generateReplySubject(currentMsg.Subject)
	subjectPromptStr := e.LoadedStrings.MsgTitleStr
	if subjectPromptStr == "" {
		subjectPromptStr = "|08[|15Title|08] : "
	}

	terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(subjectPromptStr)), outputMode)
	terminalio.WriteProcessedBytes(terminal, []byte(defaultSubject), outputMode)

	rawInput, err := terminal.ReadLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "LOGOFF"
		}
		log.Printf("ERROR: Node %d: Failed getting subject input: %v", nodeNumber, err)
		return ""
	}
	newSubject := strings.TrimSpace(rawInput)
	if newSubject == "" {
		newSubject = defaultSubject
	}
	if strings.TrimSpace(newSubject) == "" {
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nSubject cannot be empty. Reply cancelled.\r\n"), outputMode)
		time.Sleep(1 * time.Second)
		return ""
	}

	// Get TERM env var
	termType := "vt100"
	for _, env := range s.Environ() {
		if strings.HasPrefix(env, "TERM=") {
			termType = strings.TrimPrefix(env, "TERM=")
			break
		}
	}

	terminalio.WriteProcessedBytes(terminal, []byte("\r\nLaunching editor...\r\n"), outputMode)

	replyBody, saved, editErr := editor.RunEditor(quotedBody, s, s, termType)
	if editErr != nil {
		log.Printf("ERROR: Node %d: Editor failed: %v", nodeNumber, editErr)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nEditor encountered an error.\r\n"), outputMode)
		time.Sleep(2 * time.Second)
		return ""
	}

	if !saved {
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nReply cancelled.\r\n"), outputMode)
		time.Sleep(1 * time.Second)
		return ""
	}

	// Save reply
	replyMsgID := currentMsg.MsgID
	_, err = e.MessageMgr.AddMessage(currentAreaID, currentUser.Handle, currentMsg.From,
		newSubject, replyBody, replyMsgID)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to save reply: %v", nodeNumber, err)
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nError saving reply message.\r\n"), outputMode)
		time.Sleep(2 * time.Second)
	} else {
		terminalio.WriteProcessedBytes(terminal, []byte("\r\nReply posted successfully!\r\n"), outputMode)
		time.Sleep(1 * time.Second)
		*totalMsgCount++
		if *currentMsgNum < *totalMsgCount {
			*currentMsgNum++
		}
	}
	return ""
}

// handleThread prompts for forward/backward and searches for matching subject.
func handleThread(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	outputMode ansi.OutputMode, areaID int,
	currentMsgNum *int, totalMsgs int, subject string) {

	prompt := "|09Message Threading, |08[|15F|08]|09orward or |08[|15B|08]|09ackwards : "
	terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	key, err := readSingleKey(s)
	if err != nil {
		return
	}

	forward := unicode.ToUpper(key) != 'B'

	newMsg, found := forwardBackThread(e, areaID, *currentMsgNum, totalMsgs, subject, forward)
	if found {
		*currentMsgNum = newMsg
	} else {
		dir := "forward"
		if !forward {
			dir = "backward"
		}
		msg := fmt.Sprintf("\r\n|07No %s thread found!|07", dir)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
	}
}

// forwardBackThread searches for messages with matching subjects, like Pascal's forwardbackthread.
func forwardBackThread(e *MenuExecutor, areaID int, currentMsg int,
	totalMsgs int, subject string, forward bool) (int, bool) {

	// Strip " -Re: #N-" suffix and "Re: " prefix for matching
	searchSubject := stripReplyPrefix(subject)

	if forward {
		for i := currentMsg + 1; i <= totalMsgs; i++ {
			msg, err := e.MessageMgr.GetMessage(areaID, i)
			if err != nil || msg.IsDeleted {
				continue
			}
			if subjectMatchesThread(msg.Subject, searchSubject) {
				return i, true
			}
		}
	} else {
		for i := currentMsg - 1; i >= 1; i-- {
			msg, err := e.MessageMgr.GetMessage(areaID, i)
			if err != nil || msg.IsDeleted {
				continue
			}
			if subjectMatchesThread(msg.Subject, searchSubject) {
				return i, true
			}
		}
	}
	return currentMsg, false
}

// stripReplyPrefix removes "Re: " prefix and " -Re: #N-" suffix from a subject.
func stripReplyPrefix(subject string) string {
	s := subject
	// Remove " -Re: #N-" suffixes (Pascal-style reply markers)
	reReply := regexp.MustCompile(` -Re: #\d+-$`)
	s = reReply.ReplaceAllString(s, "")
	// Remove standard "Re: " prefix
	s = strings.TrimSpace(s)
	for strings.HasPrefix(strings.ToUpper(s), "RE: ") {
		s = strings.TrimSpace(s[4:])
	}
	return s
}

// subjectMatchesThread checks if a message subject matches a thread search string.
func subjectMatchesThread(msgSubject, searchSubject string) bool {
	stripped := stripReplyPrefix(msgSubject)
	return strings.EqualFold(stripped, searchSubject)
}

// handleJump prompts the user for a message number to jump to.
func handleJump(s ssh.Session, terminal *term.Terminal, outputMode ansi.OutputMode,
	currentMsgNum *int, totalMsgs int) {

	prompt := fmt.Sprintf("|09Jump to message # |01(|131|05/|13%d|01) : |15", totalMsgs)
	terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	input, err := readLineInput(s, terminal, outputMode, 6)
	if err != nil {
		return
	}

	if input == "" {
		return
	}

	num, parseErr := strconv.Atoi(input)
	if parseErr != nil || num < 1 || num > totalMsgs {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01Invalid message number!|07")), outputMode)
		time.Sleep(500 * time.Millisecond)
		return
	}

	*currentMsgNum = num
}

// displayReaderHelp shows the help screen for message reader commands.
func displayReaderHelp(terminal *term.Terminal, outputMode ansi.OutputMode) {
	help := "\r\n" +
		"|15Message Reader Help|07\r\n" +
		"|08" + strings.Repeat("-", 40) + "|07\r\n" +
		"|15N|07ext Message          |15#|07 Read Message #\r\n" +
		"|15A|07 Read Again          |15R|07eply to Message\r\n" +
		"|15P|07ost a Message        |15S|07kip to Next Area\r\n" +
		"|15T|07hread Search         |15J|07ump to Message #\r\n" +
		"|15M|07ail Reply            |15L|07ist Titles\r\n" +
		"|15Q|07uit Reader\r\n" +
		"|08" + strings.Repeat("-", 40) + "|07\r\n"

	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(help)), outputMode)
	time.Sleep(2 * time.Second)
}

// runGetHeaderType allows the user to select a message header style (MSGHDR.1-14).
// Displays the MSGHDR.ANS selection screen and previews the selected header.
func runGetHeaderType(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {

	if currentUser == nil {
		return nil, "", nil
	}

	// Display the header selection ANSI screen
	selectionPath := filepath.Join(e.MenuSetPath, "templates", "message_headers", "MSGHDR.ANS")
	selectionBytes, err := os.ReadFile(selectionPath)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to load MSGHDR.ANS: %v", nodeNumber, err)
		msg := "\r\n|01MSGHDR.ANS not found! Please notify SysOp.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Find the input field position (last ESC[row;colH in the file)
	inputRow, inputCol := findLastCursorPos(selectionBytes)
	if inputRow == 0 {
		inputRow = 22 // fallback
		inputCol = 45
	}

	for {
		// Display selection screen (process pipe codes like |B2 for background colors)
		terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes(selectionBytes), outputMode)

		// Position cursor at the input field and set background color
		terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(inputRow, inputCol)), outputMode)
		terminalio.WriteProcessedBytes(terminal, []byte("\x1b[1;37;42m"), outputMode) // bright white on green bg

		// Read selection number
		input, readErr := readLineInput(s, terminal, outputMode, 2)
		// Reset colors after input
		terminalio.WriteProcessedBytes(terminal, []byte("\x1b[0m"), outputMode)
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", nil
		}

		if strings.ToUpper(input) == "Q" || input == "" {
			break
		}

		num, parseErr := strconv.Atoi(input)
		if parseErr != nil || num < 1 || num > 14 {
			continue
		}

		// Check if header file exists
		hdrPath := filepath.Join(e.MenuSetPath, "templates", "message_headers",
			fmt.Sprintf("MSGHDR.%d", num))
		if _, statErr := os.Stat(hdrPath); statErr != nil {
			terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
			msg := fmt.Sprintf("\r\n|01Message Header #%d is not found!|07\r\n", num)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(1 * time.Second)
			continue
		}

		// Preview with sample data
		hdrBytes, _ := os.ReadFile(hdrPath)
		sampleSubs := map[byte]string{
			'B': "GENERAL",
			'T': "ViSiON/3 Rocks!",
			'F': currentUser.Handle,
			'S': "Everybody",
			'U': "User Note",
			'L': "100",
			'R': currentUser.RealName,
			'#': "1",
			'N': "42",
			'D': time.Now().Format("01/02/06"),
			'W': time.Now().Format("3:04 pm"),
			'P': "None",
			'E': "0",
		}
		processedPreview := processDataFile(hdrBytes, sampleSubs)
		terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
		terminalio.WriteProcessedBytes(terminal, processedPreview, outputMode)

		// Ask "Pick this header?"
		pickPrompt := "|08P|07i|15ck |08t|07h|15is |08h|07e|15ader? "
		pickYes, pickErr := e.promptYesNoLightbar(s, terminal, pickPrompt, outputMode, nodeNumber)
		if pickErr != nil {
			if errors.Is(pickErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			break
		}

		if pickYes {
			currentUser.MsgHdr = num
			if saveErr := userManager.SaveUsers(); saveErr != nil {
				log.Printf("ERROR: Node %d: Failed to save user after header selection: %v", nodeNumber, saveErr)
			}
			break
		}
	}

	return nil, "", nil
}

// Compile-time reference to suppress unused import
var _ = bufio.NewReader
