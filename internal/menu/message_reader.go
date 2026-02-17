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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/editor"
	"github.com/stlalpha/vision3/internal/message"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
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
	startMsg int, totalMsgCount int, isNewScan bool,
	termWidth int, termHeight int) (*user.User, string, error) {

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
		fmt.Sprintf("MSGHDR.%d.ans", hdrStyle))
	hdrTemplateBytes, hdrErr := ansi.GetAnsiFileContent(hdrTemplatePath)
	if hdrErr != nil {
		log.Printf("ERROR: Node %d: Failed to load MSGHDR.%d.ans: %v", nodeNumber, hdrStyle, hdrErr)
		// Fallback to style 2 (simple text format)
		hdrTemplatePath = filepath.Join(e.MenuSetPath, "templates", "message_headers", "MSGHDR.2.ans")
		hdrTemplateBytes, hdrErr = ansi.GetAnsiFileContent(hdrTemplatePath)
		if hdrErr != nil {
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.MsgHdrLoadError)), outputMode)
			time.Sleep(1 * time.Second)
			return nil, "", fmt.Errorf("failed loading MSGHDR templates")
		}
	}

	// Trim trailing empty lines from header template to prevent scrolling.
	// ANSI templates are often padded to 24/25 rows, but the message reader
	// handles body/footer positioning separately, so trailing blank lines
	// just push header content off the top of the screen.
	hdrTemplateBytes = bytes.TrimRight(hdrTemplateBytes, "\r\n ")

	// Terminal dimensions are now passed as parameters to use user's adjusted preferences
	// Default to 80x24 if not provided
	if termWidth <= 0 {
		termWidth = 80
	}
	if termHeight <= 0 {
		termHeight = 24
	}

	reader := bufio.NewReader(s)

	// Lightbar colors from theme
	hiColor := e.Theme.YesNoHighlightColor
	loColor := 9 // Bright blue unselected items
	boundsColor := 1

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
		templateUsesUserNote := bytes.Contains(hdrTemplateBytes, []byte("|U")) ||
			bytes.Contains(hdrTemplateBytes, []byte("@U@")) ||
			bytes.Contains(hdrTemplateBytes, []byte("@U:")) ||
			bytes.Contains(hdrTemplateBytes, []byte("@U#")) ||
			bytes.Contains(hdrTemplateBytes, []byte("@U*"))
		replyCount := 0
		if e.MessageMgr != nil {
			if count, err := e.MessageMgr.GetThreadReplyCount(currentAreaID, currentMsg.MsgNum, currentMsg.Subject); err != nil {
				log.Printf("WARN: Failed to get reply count for area %d msg %d: %v", currentAreaID, currentMsg.MsgNum, err)
			} else {
				replyCount = count
			}
		}
		substitutions := buildMsgSubstitutions(currentMsg, currentAreaTag, currentMsgNum, totalMsgCount, currentUser.AccessLevel, !templateUsesUserNote, replyCount, confName, areaName, e.MessageMgr, currentAreaID, userManager)
		autoWidths := buildAutoWidths(substitutions, totalMsgCount, termWidth)

		// Process template with substitutions (auto-detects @CODE@ or |X format)
		processedHeader := processTemplate(hdrTemplateBytes, substitutions, autoWidths)

		// Process message body and pre-format all lines
		area, _ := e.MessageMgr.GetAreaByID(currentAreaID)
		includeOrigin := area != nil && (area.AreaType == "echomail" || area.AreaType == "netmail") &&
			currentMsg.OrigAddr != "" && !hasOriginLine(currentMsg.Body)
		formattedBody := formatMessageBody(currentMsg.Body, currentMsg.OrigAddr, includeOrigin)

		// Convert pipe codes to ANSI sequences
		processedBodyBytes := ansi.ReplacePipeCodes([]byte(formattedBody))
		processedBodyStr := string(processedBodyBytes)

		var wrappedBodyLines []string

		// Check if message contains ANSI art using improved detection
		hasAnsiArt := detectAnsiArtInMessage(processedBodyStr)
		log.Printf("DEBUG: Message %d ANSI art detection: %v (body length: %d)", currentMsgNum, hasAnsiArt, len(processedBodyStr))

		if hasAnsiArt {
			log.Printf("DEBUG: Using ANSI renderer (no-wrap mode) for message %d (termWidth=%d)", currentMsgNum, termWidth)
			// Render ANSI art into virtual buffer with NO AUTO-WRAPPING
			// Cursor positioning is relative to buffer (0,0), not terminal screen
			// Text that exceeds buffer width is clipped, not wrapped
			wrappedBodyLines = RenderANSIArtToLines(processedBodyStr, termWidth, 500)
			log.Printf("DEBUG: Rendered %d lines from ANSI art", len(wrappedBodyLines))

			// Convert CP437 bytes to UTF-8 for modern terminals
			for i, line := range wrappedBodyLines {
				wrappedBodyLines[i] = string(ansi.CP437BytesToUTF8([]byte(line)))
			}
		} else {
			log.Printf("DEBUG: Using normal text wrapping for message %d", currentMsgNum)
			// Regular text message - use normal wrapping
			wrappedBodyLines = wrapAnsiString(processedBodyStr, termWidth)
		}

		// Calculate available body height
		// Find the actual bottom row of the header using ANSI cursor tracking
		headerEndRow := findHeaderEndRow(processedHeader)
		bodyStartRow := headerEndRow + 1 // Start body on next row after header
		barLines := 2                    // Horizontal line + lightbar
		bodyAvailHeight := termHeight - bodyStartRow - barLines
		if bodyAvailHeight < 1 {
			bodyAvailHeight = 5
		}

		// Initialize scroll state for this message
		scrollOffset := 0
		totalBodyLines := len(wrappedBodyLines)
		needsRedraw := true
		needsBodyRedraw := false

		drawBody := func() {
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
		}

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
				// For CP437 mode, write raw bytes directly to avoid UTF-8 false positives
				if outputMode == ansi.OutputModeCP437 {
					terminal.Write(processedHeader)
				} else {
					terminalio.WriteProcessedBytes(terminal, processedHeader, outputMode)
				}

				drawBody()

				// Display footer: lightbar only
				var suffixText string
				if isNewScan {
					suffixText = e.LoadedStrings.MsgNewScanSuffix
				} else {
					suffixText = e.LoadedStrings.MsgReadingSuffix
				}

				// Draw horizontal line above footer (CP437 character 196)
				terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(termHeight-1, 1)), outputMode)
				horizontalLine := "|08" + strings.Repeat("\xC4", termWidth-1) + "|07" // CP437 horizontal line character
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(horizontalLine)), outputMode)

				// Position cursor at last row for lightbar
				terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(termHeight, 1)), outputMode)

				// Draw the lightbar menu
				drawMsgLightbarStatic(terminal, msgReaderOptions, outputMode, hiColor, loColor, suffixText, 0, true, boundsColor)

				needsRedraw = false
				needsBodyRedraw = false
			} else if needsBodyRedraw {
				// Only redraw body area to avoid refreshing header/footer while scrolling
				drawBody()
				needsBodyRedraw = false
			}

			// Read key sequence (handles escape sequences for arrow keys, page up/down)
			keySeq, keyErr := readKeySequence(reader)
			if keyErr != nil {
				if errors.Is(keyErr, io.EOF) {
					return nil, "LOGOFF", io.EOF
				}
				continue
			}

			// Handle scrolling keys first
			switch keySeq {
			case "\x1b": // ESC key - quit reader
				quitNewscan = true
				break readerLoop

			case "\x1b[A": // Up arrow - scroll up one line
				if scrollOffset > 0 {
					scrollOffset--
					needsBodyRedraw = true
				}
				continue

			case "\x1b[B": // Down arrow - scroll down one line
				if totalBodyLines > bodyAvailHeight && scrollOffset < totalBodyLines-bodyAvailHeight {
					scrollOffset++
					needsBodyRedraw = true
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
				needsBodyRedraw = true
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
				needsBodyRedraw = true
				continue

			case "\x1b[D", "\x1b[C": // Left or Right arrow - activate interactive lightbar
				var suffixText string
				if isNewScan {
					suffixText = e.LoadedStrings.MsgNewScanSuffix
				} else {
					suffixText = e.LoadedStrings.MsgReadingSuffix
				}

				terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(termHeight, 1)), outputMode)

				// Determine initial direction based on which arrow was pressed
				initialDir := 0
				if keySeq == "\x1b[D" {
					initialDir = -1 // Left arrow
				} else if keySeq == "\x1b[C" {
					initialDir = 1 // Right arrow
				}

				selKey, lbErr := runMsgLightbar(reader, terminal, msgReaderOptions, outputMode, hiColor, loColor, suffixText, initialDir, true, boundsColor)
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

			if selectedKey == 0 {
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
							suffixText = e.LoadedStrings.MsgNewScanSuffix
						} else {
							suffixText = e.LoadedStrings.MsgReadingSuffix
						}

						// Position cursor at last row for lightbar
						terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(termHeight, 1)), outputMode)

						selKey, lbErr := runMsgLightbar(reader, terminal, msgReaderOptions, outputMode, hiColor, loColor, suffixText, 0, true, boundsColor)
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
						suffixText = e.LoadedStrings.MsgNewScanSuffix
					} else {
						suffixText = e.LoadedStrings.MsgReadingSuffix
					}

					terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(termHeight, 1)), outputMode)

					selKey, lbErr := runMsgLightbar(reader, terminal, msgReaderOptions, outputMode, hiColor, loColor, suffixText, 0, true, boundsColor)
					if lbErr != nil {
						if errors.Is(lbErr, io.EOF) {
							return nil, "LOGOFF", io.EOF
						}
						log.Printf("ERROR: Node %d: Lightbar error: %v", nodeNumber, lbErr)
						break readerLoop
					}
					selectedKey = rune(selKey)
				}
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
					terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.MsgEndOfMessages)), outputMode)
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
					sessionStartTime, "", outputMode, termWidth, termHeight)
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
				handleThread(reader, e, terminal, outputMode, currentAreaID,
					&currentMsgNum, totalMsgCount, currentMsg.Subject)
				// Exit scroll loop to load new message if thread changed it
				break scrollLoop

			case 'J': // Jump to message number
				handleJump(reader, terminal, outputMode, &currentMsgNum, totalMsgCount, e.LoadedStrings.MsgJumpPrompt, e.LoadedStrings.MsgInvalidMsgNum)
				// Exit scroll loop to load new message
				break scrollLoop

			case 'M': // Mail reply (deferred)
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.MsgMailReplyDeferred)), outputMode)
				time.Sleep(1 * time.Second)
				needsRedraw = true
				continue

			case 'L': // List titles (deferred)
				terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.MsgListDeferred)), outputMode)
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

func hasOriginLine(text string) bool {
	if text == "" {
		return false
	}
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "* Origin:") {
			return true
		}
	}
	return false
}

var quotePrefixRe = regexp.MustCompile(`^[A-Za-z0-9]{1,3}>`)

func isQuoteLine(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, ">") {
		return true
	}
	return quotePrefixRe.MatchString(trimmed)
}

func isTearLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "---")
}

func isOriginLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "* Origin:")
}

// hasANSICursorMovement checks for ANSI cursor movement codes with digits
func hasANSICursorMovement(text string) bool {
	// Look for patterns like ESC[<digits>A/B/C/D or ESC[<digits>;<digits>H/f
	for i := 0; i < len(text)-3; i++ {
		if text[i] == '\x1b' && i+1 < len(text) && text[i+1] == '[' {
			// Found ESC[, now look for cursor codes
			j := i + 2
			hasDigit := false
			for j < len(text) && ((text[j] >= '0' && text[j] <= '9') || text[j] == ';') {
				if text[j] >= '0' && text[j] <= '9' {
					hasDigit = true
				}
				j++
			}
			// Check if followed by cursor movement letter
			if hasDigit && j < len(text) {
				switch text[j] {
				case 'A', 'B', 'C', 'D', 'H', 'f': // Cursor movement or positioning
					return true
				}
			}
		}
	}
	return false
}

// detectAnsiArtInMessage checks if message body contains ANSI art
// using the same logic as Retrograde
func detectAnsiArtInMessage(text string) bool {
	// Must contain ANSI codes
	if !strings.Contains(text, "\x1b[") {
		return false
	}

	// Check for common ANSI art indicators:
	// 1. Home cursor without row/col (ESC[H)
	// 2. Explicit cursor positioning (ESC[f)
	// 3. Cursor movement with digits (ESC[5A, ESC[10;20H, etc.)
	return strings.Contains(text, "\x1b[H") ||
		strings.Contains(text, "\x1b[f") ||
		hasANSICursorMovement(text)
}

func formatMessageBody(body, originAddr string, includeOrigin bool) string {
	// Check if body contains ANSI art BEFORE normalizing line endings
	// ANSI art uses \r for cursor positioning and must NOT be modified
	if detectAnsiArtInMessage(body) {
		// For ANSI art: Return raw body unchanged
		// The ANSI renderer will handle all cursor positioning (\r, \n, ESC codes)
		// Converting \r to \r\n would break layering effects in ANSI art
		if includeOrigin && originAddr != "" && !hasOriginLine(body) {
			return body + "\r\n* Origin: " + originAddr
		}
		return body
	}

	// Regular text: normalize line endings to \n
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	lines := strings.Split(normalized, "\n")
	if includeOrigin {
		lines = append(lines, fmt.Sprintf("* Origin: %s", originAddr))
	}

	out := make([]string, 0, len(lines))
	prevWasQuote := false
	prevWasTear := false

	for i, rawLine := range lines {
		line := rawLine
		isQuote := isQuoteLine(line)
		isTear := isTearLine(line)
		isOrigin := isOriginLine(line)
		isOriginBlock := isTear || isOrigin

		if isOriginBlock {
			if !prevWasTear && len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			if isTear {
				out = append(out, fmt.Sprintf("|08%s|07", line))
				prevWasTear = true
			} else {
				out = append(out, fmt.Sprintf("|09%s|07", line))
				prevWasTear = false
			}
			prevWasQuote = false
			continue
		}

		if isQuote {
			if !prevWasQuote && len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}

			out = append(out, fmt.Sprintf("|14%s|07", line))

			nextLine := ""
			if i+1 < len(lines) {
				nextLine = lines[i+1]
			}
			nextIsQuote := i+1 < len(lines) && isQuoteLine(nextLine)
			nextIsOriginBlock := i+1 < len(lines) && (isTearLine(nextLine) || isOriginLine(nextLine))
			if !nextIsQuote && strings.TrimSpace(nextLine) != "" && !nextIsOriginBlock {
				out = append(out, "")
			}

			prevWasQuote = true
			prevWasTear = false
			continue
		}

		out = append(out, line)
		prevWasQuote = false
		prevWasTear = false
	}

	return strings.Join(out, "\n")
}

// findMessageByMSGID searches for a message in the current area that has the given MSGID.
// Returns the message number (1-based) if found, or 0 if not found.
// Delegates to MessageManager.FindMessageByMSGID which uses a cached index.
func findMessageByMSGID(msgMgr *message.MessageManager, areaID int, msgID string) int {
	return msgMgr.FindMessageByMSGID(areaID, msgID)
}

// buildMsgSubstitutions creates the Pascal-style substitution map for MSGHDR templates.
func buildMsgSubstitutions(msg *message.DisplayMessage, areaTag string, msgNum, totalMsgs int, userLevel int, includeNoteInFrom bool, replyCount int, confName string, areaName string, msgMgr *message.MessageManager, areaID int, userMgr *user.UserMgr) map[byte]string {
	// Import jam constants
	const (
		msgTypeLocal = 0x00800000
		msgTypeEcho  = 0x01000000
		msgTypeNet   = 0x02000000
		msgPrivate   = 0x00000004
		msgRead      = 0x00000008
		msgSent      = 0x00000010
	)

	// Build message status flags (also used for note display)
	isEcho := (msg.Attributes & msgTypeEcho) != 0
	isNet := (msg.Attributes & msgTypeNet) != 0
	isSent := (msg.Attributes & msgSent) != 0
	isRead := (msg.Attributes & msgRead) != 0
	isPrivate := (msg.Attributes & msgPrivate) != 0

	// Look up the message author's user note from users.json
	userNoteToUse := ""
	if userMgr != nil {
		if authorUser, found := userMgr.GetUserByHandle(msg.From); found {
			userNoteToUse = authorUser.PrivateNote
		}
	}

	// Truncate user note if too long (max 25 characters for display)
	const maxUserNoteLen = 25
	truncatedNote := userNoteToUse
	if len(userNoteToUse) > maxUserNoteLen {
		truncatedNote = userNoteToUse[:maxUserNoteLen-3] + "..."
	}

	// Build From field with user note and/or FidoNet address
	fromStr := msg.From
	if includeNoteInFrom && truncatedNote != "" && msg.OrigAddr != "" {
		// Both user note and FidoNet address
		fromStr = fmt.Sprintf("%s \"%s\" (%s)", msg.From, truncatedNote, msg.OrigAddr)
	} else if includeNoteInFrom && truncatedNote != "" {
		// Just user note
		fromStr = fmt.Sprintf("%s \"%s\"", msg.From, truncatedNote)
	} else if msg.OrigAddr != "" {
		// Just FidoNet address
		fromStr = fmt.Sprintf("%s (%s)", msg.From, msg.OrigAddr)
	}

	// Build To field with FidoNet destination address if available
	toStr := msg.To
	if msg.DestAddr != "" {
		toStr = fmt.Sprintf("%s (%s)", msg.To, msg.DestAddr)
	}

	// Build message status string from message attributes
	var statusParts []string

	// Determine message type
	if isEcho {
		if isSent {
			statusParts = append(statusParts, "ECHOMAIL SENT")
		} else {
			statusParts = append(statusParts, "ECHOMAIL")
		}
	} else if isNet {
		if isSent {
			statusParts = append(statusParts, "NETMAIL SENT")
		} else {
			statusParts = append(statusParts, "NETMAIL UNSENT")
		}
	} else {
		statusParts = append(statusParts, "LOCAL")
	}

	// Add additional status flags
	if isRead {
		statusParts = append(statusParts, "READ")
	}
	if isPrivate {
		statusParts = append(statusParts, "PRIVATE")
	}

	msgStatusStr := strings.Join(statusParts, " ")

	// Determine reply-to display: use JAM header ReplyTo (message number) first,
	// then fall back to MSGID index lookup, then "None".
	replyStr := "None"
	if msg.ReplyToNum > 0 {
		// JAM header already has the parent message number (set by linker/tosser)
		replyStr = strconv.Itoa(msg.ReplyToNum)
	} else if msg.ReplyID != "" {
		// Header ReplyTo not set — try MSGID index lookup as fallback
		if replyMsgNum := findMessageByMSGID(msgMgr, areaID, msg.ReplyID); replyMsgNum > 0 {
			replyStr = strconv.Itoa(replyMsgNum)
		}
		// If neither works, leave as "None" — no confusing text for users
	}

	return map[byte]string{
		'B': areaTag,
		'T': msg.Subject,
		'F': fromStr,                 // From with FidoNet address
		'S': toStr,                   // To with FidoNet address
		'U': userNoteToUse,           // User note from user profile (local only)
		'M': msgStatusStr,            // Message status (LOCAL, PRIVATE, ECHOMAIL, NETMAIL)
		'L': strconv.Itoa(userLevel), // User level/access level
		'#': strconv.Itoa(msgNum),
		'N': strconv.Itoa(totalMsgs),
		'C': fmt.Sprintf("[%d/%d]", msgNum, totalMsgs), // Message count display
		'D': msg.DateTime.Format("01/02/06"),
		'W': msg.DateTime.Format("3:04 pm"),
		'P': replyStr,
		'E': strconv.Itoa(replyCount),
		'O': msg.OrigAddr,                                                          // Origin address
		'A': msg.DestAddr,                                                          // Destination address
		'Z': fmt.Sprintf("%s > %s", confName, areaName),                            // Conference > Area Name
		'V': fmt.Sprintf("%d of %d", msgNum, totalMsgs),                                // Verbose count: "1 of 24"
		'X': fmt.Sprintf("%s > %s [%d/%d]", confName, areaName, msgNum, totalMsgs),      // Conference > Area [current/total]
	}
}

// buildAutoWidths calculates the maximum display width for each placeholder code.
// Used by the @CODE*@ auto-width modifier so templates don't need hardcoded widths.
// Width is based on the maximum possible value for each code in the current context:
//   - Numeric codes (#, N, C): based on totalMsgs digit count
//   - Fixed-format codes (D, W): known max format lengths
//   - Context codes (Z, X): based on current conference/area names + max count width
//   - All others: width of the current substitution value
func buildAutoWidths(subs map[byte]string, totalMsgs int, termWidth int) map[byte]int {
	widths := make(map[byte]int)

	maxMsgNumWidth := len(strconv.Itoa(totalMsgs))

	// Fixed-format codes
	widths['D'] = 8 // MM/DD/YY always 8
	widths['W'] = 8 // max "12:00 pm" = 8

	// Numeric codes: pad to width of largest possible message number
	widths['#'] = maxMsgNumWidth
	widths['N'] = maxMsgNumWidth

	// Count display [current/total]: max when current = totalMsgs
	maxCountStr := fmt.Sprintf("[%d/%d]", totalMsgs, totalMsgs)
	widths['C'] = len(maxCountStr)

	// Verbose count "X of Y": max when current = totalMsgs
	maxVerboseStr := fmt.Sprintf("%d of %d", totalMsgs, totalMsgs)
	widths['V'] = len(maxVerboseStr)

	// Z = "confName > areaName" (same for all messages in area)
	if zVal, ok := subs['Z']; ok {
		widths['Z'] = len(zVal)
		// X = Z + " " + [current/total], max when current = totalMsgs
		widths['X'] = len(zVal) + 1 + len(maxCountStr)
	}

	// Gap fill: target width is terminal width minus 1 to avoid auto-wrap
	// at column 80 which creates blank lines for sequential text output
	if termWidth > 1 {
		widths['G'] = termWidth - 1
	}

	// All other codes: use current value length
	for code, val := range subs {
		if _, exists := widths[code]; !exists {
			widths[code] = len(val)
		}
	}

	return widths
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

	// Prepare quote data for /Q command
	// Split message body into lines for quoting
	quoteLines := strings.Split(currentMsg.Body, "\n")

	// Format date/time from message
	quoteDate := currentMsg.DateTime.Format("01/02/2006")
	quoteTime := currentMsg.DateTime.Format("3:04 PM")

	// Auto-generate subject with "RE: " prefix (no prompt needed)
	newSubject := generateReplySubject(currentMsg.Subject)
	if strings.TrimSpace(newSubject) == "" {
		terminalio.WriteProcessedBytes(terminal, []byte(e.LoadedStrings.MsgReplySubjectEmpty), outputMode)
		time.Sleep(1 * time.Second)
		return ""
	}

	terminalio.WriteProcessedBytes(terminal, []byte(e.LoadedStrings.MsgLaunchingEditor), outputMode)

	// Start with empty editor - user will use /Q command to quote if desired
	// Pass message metadata for quoting (from, title, date, time, isAnon, lines)
	replyBody, saved, editErr := editor.RunEditorWithMetadata("", s, s, outputMode, newSubject, currentMsg.To, false,
		currentMsg.From, currentMsg.Subject, quoteDate, quoteTime, false, quoteLines)
	if editErr != nil {
		log.Printf("ERROR: Node %d: Editor failed: %v", nodeNumber, editErr)
		terminalio.WriteProcessedBytes(terminal, []byte(e.LoadedStrings.MsgEditorError), outputMode)
		time.Sleep(2 * time.Second)
		return ""
	}

	if !saved {
		terminalio.WriteProcessedBytes(terminal, []byte(e.LoadedStrings.MsgReplyCancelled), outputMode)
		time.Sleep(1 * time.Second)
		return ""
	}

	// Save reply
	replyMsgID := currentMsg.MsgID
	_, err := e.MessageMgr.AddMessage(currentAreaID, currentUser.Handle, currentMsg.From,
		newSubject, replyBody, replyMsgID)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to save reply: %v", nodeNumber, err)
		terminalio.WriteProcessedBytes(terminal, []byte(e.LoadedStrings.MsgReplyError), outputMode)
		time.Sleep(2 * time.Second)
	} else {
		currentUser.MessagesPosted++
		if err := userManager.UpdateUser(currentUser); err != nil {
			log.Printf("ERROR: Node %d: Failed to update MessagesPosted for user %s: %v", nodeNumber, currentUser.Handle, err)
		}
		terminalio.WriteProcessedBytes(terminal, []byte(e.LoadedStrings.MsgReplySuccess), outputMode)
		time.Sleep(1 * time.Second)
		*totalMsgCount++
		if *currentMsgNum < *totalMsgCount {
			*currentMsgNum++
		}
	}
	return ""
}

// handleThread prompts for forward/backward and searches for matching subject.
func handleThread(reader *bufio.Reader, e *MenuExecutor, terminal *term.Terminal,
	outputMode ansi.OutputMode, areaID int,
	currentMsgNum *int, totalMsgs int, subject string) {

	terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.MsgThreadPrompt)), outputMode)

	key, err := readSingleKey(reader)
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
		msg := fmt.Sprintf(e.LoadedStrings.MsgNoThreadFound, dir)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
	}
}

// forwardBackThread searches for messages with matching subjects, like Pascal's forwardbackthread.
func forwardBackThread(e *MenuExecutor, areaID int, currentMsg int,
	totalMsgs int, subject string, forward bool) (int, bool) {

	// Strip " -Re: #N-" suffix and "Re: " prefix for matching
	searchSubject := message.NormalizeThreadSubject(subject)

	if forward {
		for i := currentMsg + 1; i <= totalMsgs; i++ {
			msg, err := e.MessageMgr.GetMessage(areaID, i)
			if err != nil || msg.IsDeleted {
				continue
			}
			if message.SubjectsMatchThread(msg.Subject, searchSubject) {
				return i, true
			}
		}
	} else {
		for i := currentMsg - 1; i >= 1; i-- {
			msg, err := e.MessageMgr.GetMessage(areaID, i)
			if err != nil || msg.IsDeleted {
				continue
			}
			if message.SubjectsMatchThread(msg.Subject, searchSubject) {
				return i, true
			}
		}
	}
	return currentMsg, false
}

// handleJump prompts the user for a message number to jump to.
func handleJump(reader *bufio.Reader, terminal *term.Terminal, outputMode ansi.OutputMode,
	currentMsgNum *int, totalMsgs int, jumpPromptFmt string, invalidMsgStr string) {

	prompt := fmt.Sprintf(jumpPromptFmt, totalMsgs)
	terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	input, err := readLineInput(reader, terminal, outputMode, 6)
	if err != nil {
		return
	}

	if input == "" {
		return
	}

	num, parseErr := strconv.Atoi(input)
	if parseErr != nil || num < 1 || num > totalMsgs {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(invalidMsgStr)), outputMode)
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

// MessageHeaderTemplate represents a discovered message header template file.
type MessageHeaderTemplate struct {
	Number      int    // Template number (e.g., 1, 2, 15)
	Filename    string // Full filename (e.g., "MSGHDR.15.ans")
	DisplayName string // Human-readable name (e.g., "Header Style 15")
}

// extractHeaderNumber extracts the numeric ID from a message header filename.
// Examples: "MSGHDR.1.ans" -> 1, "MSGHDR.15.ans" -> 15
func extractHeaderNumber(filename string) (int, error) {
	// Remove .ans extension and MSGHDR. prefix
	name := strings.TrimSuffix(filename, ".ans")
	name = strings.TrimPrefix(name, "MSGHDR.")

	return strconv.Atoi(name)
}

// headerDisplayNames maps template numbers to their actual display names.
var headerDisplayNames = map[int]string{
	1:  "Generic Blue Box",
	2:  "Extremely Simple Message Header",
	3:  "LiQUiD Blue Box Header",
	4:  "Generic TCS Header",
	5:  "Gray/White ViSiON/2 Header",
	6:  "Cool V/2 Quick Header!",
	7:  "ViSiON-X Header",
	8:  "Bouncer Neato Header",
	9:  "Celerity Header",
	10: "PC Express Header",
	11: "Extreme Header",
	12: "LSD Header",
	13: "Renegade Header",
	14: "ViSiON-X Grey/White Header",
}

// discoverMessageHeaders finds all MSGHDR.*.ans template files in the templates/message_headers directory.
// Returns templates sorted by number (1, 2, ..., 14, 15, etc.).
func discoverMessageHeaders(templatesPath string) ([]MessageHeaderTemplate, error) {
	pattern := filepath.Join(templatesPath, "message_headers", "MSGHDR.*.ans")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob header templates: %w", err)
	}

	var templates []MessageHeaderTemplate
	for _, file := range files {
		base := filepath.Base(file)

		// Skip MSGHDR.ANS (selection screen, not a template)
		if base == "MSGHDR.ANS" {
			continue
		}

		num, err := extractHeaderNumber(base)
		if err != nil {
			log.Printf("WARN: Skipping malformed header filename: %s (error: %v)", base, err)
			continue
		}

		// Get display name from map, or fall back to generic name
		displayName, ok := headerDisplayNames[num]
		if !ok {
			displayName = fmt.Sprintf("Header Style %d", num)
		}

		templates = append(templates, MessageHeaderTemplate{
			Number:      num,
			Filename:    base,
			DisplayName: displayName,
		})
	}

	// Sort by number
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Number < templates[j].Number
	})

	return templates, nil
}

// runGetHeaderType allows the user to select a message header style (unlimited templates via lightbar).
// Discovers all MSGHDR.*.ans templates dynamically and presents them in a lightbar menu.
func runGetHeaderType(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return nil, "", nil
	}

	// Load lightbar options from MSGHDR.BAR
	options, err := loadLightbarOptions("MSGHDR", e)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to load MSGHDR.BAR: %v", nodeNumber, err)
		msg := "\r\n|01Error loading MSGHDR.BAR!|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	if len(options) == 0 {
		log.Printf("ERROR: Node %d: No options in MSGHDR.BAR", nodeNumber)
		msg := "\r\n|01No message header options found!|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Verify template files exist and extract template numbers
	templatesPath := filepath.Join(e.MenuSetPath, "templates", "message_headers")
	var validOptions []LightbarOption
	for _, opt := range options {
		// Parse template number from return value (not hotkey, since 10+ use letters)
		templateNum, parseErr := strconv.Atoi(opt.ReturnValue)
		if parseErr != nil {
			log.Printf("WARN: Invalid return value in MSGHDR.BAR: %s", opt.ReturnValue)
			continue
		}

		// Verify template file exists
		templateFile := filepath.Join(templatesPath, fmt.Sprintf("MSGHDR.%d.ans", templateNum))
		if _, statErr := os.Stat(templateFile); statErr != nil {
			log.Printf("WARN: Template file not found: MSGHDR.%d.ans", templateNum)
			continue
		}

		validOptions = append(validOptions, opt)
	}

	if len(validOptions) == 0 {
		log.Printf("ERROR: Node %d: No valid message header templates found", nodeNumber)
		msg := "\r\n|01No valid message header templates!|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	options = validOptions
	log.Printf("INFO: Node %d: Loaded %d message header options from BAR file", nodeNumber, len(options))

	// Display the header selection ANSI screen
	selectionPath := filepath.Join(e.MenuSetPath, "templates", "message_headers", "MSGHDR.ANS")
	selectionBytes, err := ansi.GetAnsiFileContent(selectionPath)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to load MSGHDR.ANS: %v", nodeNumber, err)
		msg := "\r\n|01MSGHDR.ANS not found! Please notify SysOp.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Hide cursor during lightbar selection
	terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode)
	// Ensure cursor is restored on exit
	defer terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode)

	reader := bufio.NewReader(s)

	const maxVisibleItems = 15 // Maximum items visible in lightbar (Y=6 to Y=20)
	selectedIndex := 0         // Currently highlighted option

	// Helper to draw one lightbar option using BAR file attributes
	drawOption := func(idx int, highlighted bool) {
		if idx < 0 || idx >= len(options) {
			return
		}

		opt := options[idx]

		// Extract template number from hotkey
		templateNum, _ := strconv.Atoi(opt.ReturnValue)

		// Position cursor using BAR file coordinates
		terminalio.WriteProcessedBytes(terminal, []byte(fmt.Sprintf("\x1b[%d;%dH", opt.Y, opt.X)), outputMode)

		if highlighted {
			// Highlighted: use highlight color from BAR file
			colorSeq := colorCodeToAnsi(opt.HighlightColor)
			terminalio.WriteProcessedBytes(terminal, []byte(colorSeq), outputMode)

			// Draw text padded to 39 columns
			displayText := fmt.Sprintf("[%-2d] - %-30s", templateNum, opt.Text)
			if len(displayText) > 39 {
				displayText = displayText[:39]
			} else if len(displayText) < 39 {
				displayText = fmt.Sprintf("%-39s", displayText)
			}
			terminalio.WriteProcessedBytes(terminal, []byte(displayText), outputMode)
		} else {
			// Inactive: white for brackets/number, bright blue for name
			bracketPart := fmt.Sprintf("[%-2d] - ", templateNum)
			namePart := fmt.Sprintf("%-30s", opt.Text)

			// Ensure total is 39 columns
			totalText := bracketPart + namePart
			if len(totalText) > 39 {
				totalText = totalText[:39]
			} else if len(totalText) < 39 {
				totalText = fmt.Sprintf("%-39s", totalText)
			}

			// White for bracket part
			terminalio.WriteProcessedBytes(terminal, []byte("\x1b[37m"+bracketPart), outputMode)
			// Bright blue for name part (pad remaining space)
			remaining := totalText[len(bracketPart):]
			terminalio.WriteProcessedBytes(terminal, []byte("\x1b[1;34m"+remaining), outputMode)
		}

		// Reset color
		terminalio.WriteProcessedBytes(terminal, []byte("\x1b[0m"), outputMode)
	}

	// Helper to redraw all options
	redrawAll := func() {
		terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
		// For CP437 mode, write raw bytes directly to avoid UTF-8 false positives
		processedSelBytes := ansi.ReplacePipeCodes(selectionBytes)
		if outputMode == ansi.OutputModeCP437 {
			terminal.Write(processedSelBytes)
		} else {
			terminalio.WriteProcessedBytes(terminal, processedSelBytes, outputMode)
		}

		// Draw all options from BAR file
		for i := 0; i < len(options); i++ {
			drawOption(i, i == selectedIndex)
		}

		// Show navigation hint at bottom (row 24), centered
		const hintY = 24 // Fixed row for footer
		// Use CP437 arrow characters: \x18=↑, \x19=↓
		hint := "|08Use |14\x18|08/|14\x19|08 arrows, |14ENTER|08 to preview, |14SPACE|08 to select, |14Q|08 to quit|07"
		// Calculate visible text length (without pipe codes) for centering
		visibleHint := "Use \x18/\x19 arrows, ENTER to preview, SPACE to select, Q to quit"
		hintX := (80 - len(visibleHint)) / 2
		if hintX < 1 {
			hintX = 1
		}
		terminalio.WriteProcessedBytes(terminal, []byte(fmt.Sprintf("\x1b[%d;%dH", hintY, hintX)), outputMode)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(hint)), outputMode)
	}

	// Initial draw
	redrawAll()

	// Main selection loop
	for {
		// Read single key/character
		r, _, readErr := reader.ReadRune()
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", nil
		}

		// Handle escape sequences for arrow keys
		if r == '\x1b' {
			// Check for arrow keys: ESC [ A (up) or ESC [ B (down)
			next, _, _ := reader.ReadRune()
			if next == '[' {
				arrow, _, _ := reader.ReadRune()
				if arrow == 'A' {
					// Up arrow
					if selectedIndex > 0 {
						selectedIndex--
						drawOption(selectedIndex+1, false)
						drawOption(selectedIndex, true)
					}
					continue
				} else if arrow == 'B' {
					// Down arrow
					if selectedIndex < len(options)-1 {
						selectedIndex++
						drawOption(selectedIndex-1, false)
						drawOption(selectedIndex, true)
					}
					continue
				}
			}
			continue
		}

		// Handle Q to quit
		if r == 'q' || r == 'Q' {
			break
		}

		// Handle ENTER to preview
		if r == '\r' || r == '\n' {
			opt := options[selectedIndex]
			templateNum, _ := strconv.Atoi(opt.ReturnValue)
			hdrPath := filepath.Join(e.MenuSetPath, "templates", "message_headers", fmt.Sprintf("MSGHDR.%d.ans", templateNum))

			// Preview with sample data
			hdrBytes, readErr := ansi.GetAnsiFileContent(hdrPath)
			if readErr != nil {
				log.Printf("ERROR: Node %d: Failed to read header file MSGHDR.%d.ans: %v", nodeNumber, templateNum, readErr)
				continue
			}
			hdrBytes = bytes.TrimRight(hdrBytes, "\r\n ")

			sampleSubs := map[byte]string{
				'B': "GENERAL",
				'T': "ViSiON/3 Rocks!",
				'F': currentUser.Handle,
				'S': "Everybody",
				'U': currentUser.PrivateNote,
				'M': "LOCAL",
				'L': strconv.Itoa(currentUser.AccessLevel),
				'#': "1",
				'N': "42",
				'C': "[1/42]",
				'V': "1 of 42",
				'D': time.Now().Format("01/02/06"),
				'W': time.Now().Format("3:04 pm"),
				'P': "None",
				'E': "0",
				'O': "",
				'A': "",
				'Z': "GENERAL > General Discussion",
				'X': "GENERAL > General Discussion [1/42]",
			}
			sampleAutoWidths := buildAutoWidths(sampleSubs, 42, 80)

			processedPreview := processTemplate(hdrBytes, sampleSubs, sampleAutoWidths)
			terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
			// For CP437 mode, write raw bytes directly to avoid UTF-8 false positives
			if outputMode == ansi.OutputModeCP437 {
				terminal.Write(processedPreview)
			} else {
				terminalio.WriteProcessedBytes(terminal, processedPreview, outputMode)
			}

			// Ask "Pick this header?" - centered at row 14
			pickPrompt := "|08P|07i|15ck |08t|07h|15is |08h|07e|15ader? "
			// Full prompt includes: "Pick this header?    Nah    Yeah    " (approximately 40 chars)
			fullPromptWidth := 40 // Prompt text + spacing + options
			promptX := (80 - fullPromptWidth) / 2
			if promptX < 1 {
				promptX = 1
			}
			terminalio.WriteProcessedBytes(terminal, []byte(fmt.Sprintf("\x1b[14;%dH", promptX)), outputMode)

			pickYes, pickErr := e.promptYesNo(s, terminal, pickPrompt, outputMode, nodeNumber, termWidth, termHeight)
			if pickErr != nil {
				if errors.Is(pickErr, io.EOF) {
					return nil, "LOGOFF", io.EOF
				}
				redrawAll()
				continue
			}

			if pickYes {
				// User selected this header - save preference
				currentUser.MsgHdr = templateNum
				if saveErr := userManager.UpdateUser(currentUser); saveErr != nil {
					log.Printf("ERROR: Node %d: Failed to save user after header selection: %v", nodeNumber, saveErr)
				}
				log.Printf("INFO: Node %d: User %s selected header style %d", nodeNumber, currentUser.Handle, templateNum)
				break
			}

			// User declined - redraw menu
			redrawAll()
			continue
		}

		// Handle SPACE to select immediately (without preview)
		if r == ' ' {
			opt := options[selectedIndex]
			templateNum, _ := strconv.Atoi(opt.ReturnValue)
			currentUser.MsgHdr = templateNum
			if saveErr := userManager.UpdateUser(currentUser); saveErr != nil {
				log.Printf("ERROR: Node %d: Failed to save user after header selection: %v", nodeNumber, saveErr)
			}
			log.Printf("INFO: Node %d: User %s selected header style %d", nodeNumber, currentUser.Handle, templateNum)
			break
		}

		// Handle numeric hotkeys (1-9) for direct selection
		if r >= '1' && r <= '9' {
			digit := int(r - '0')
			// Find option with this template number
			for i, opt := range options {
				templateNum, _ := strconv.Atoi(opt.ReturnValue)
				if templateNum == digit {
					selectedIndex = i
					drawOption(selectedIndex, false)
					selectedIndex = i
					drawOption(selectedIndex, true)
					break
				}
			}
			continue
		}
	}

	return nil, "", nil
}
