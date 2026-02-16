package menu

import (
	"bufio"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/message"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// MessageListEntry represents a single message in the list view
type MessageListEntry struct {
	MsgNum    int
	Subject   string
	From      string
	To        string
	IsPrivate bool
	IsRead    bool // Based on JAM lastread pointer
}

// MessageListState manages the list display and navigation
type MessageListState struct {
	AreaID        int
	TotalMessages int
	Entries       []MessageListEntry
	CurrentPage   int
	ItemsPerPage  int
	SelectedIndex int // Index within current page (0-based)
	LastRead      int
}

// truncateString truncates a string to maxLen, adding "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatStatusChar returns the status character for a message entry
func formatStatusChar(entry MessageListEntry, isHighlighted bool) string {
	if isHighlighted {
		// When highlighted, use plain characters without color codes
		if !entry.IsRead {
			return "N"
		}
		if entry.IsPrivate {
			return "P"
		}
		return " "
	}
	// Normal (non-highlighted) display with colors
	if !entry.IsRead {
		return "|12N|07" // Bright red N for new/unread
	}
	if entry.IsPrivate {
		return "|12P|07" // Bright red P for private
	}
	return " " // Space for read messages
}

// calculatePagination calculates the start and end indices for a page
func calculatePagination(total, perPage, currentPage int) (start, end int) {
	if total == 0 || perPage == 0 {
		return 0, 0
	}
	start = (currentPage - 1) * perPage
	end = start + perPage
	if end > total {
		end = total
	}
	return start, end
}

// buildMessageList fetches message metadata from the current area
func buildMessageList(msgMgr *message.MessageManager, areaID int, username string) ([]MessageListEntry, int, error) {
	// Get total message count for area
	totalCount, err := msgMgr.GetMessageCountForArea(areaID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get message count: %w", err)
	}

	if totalCount == 0 {
		return []MessageListEntry{}, 0, nil
	}

	// Get lastread pointer
	lastRead, err := msgMgr.GetLastRead(areaID, username)
	if err != nil {
		log.Printf("WARN: Failed to get lastread for area %d, user %s: %v", areaID, username, err)
		lastRead = 0 // Default to all unread
	}

	// Build list of message entries
	entries := make([]MessageListEntry, 0, totalCount)

	for msgNum := 1; msgNum <= totalCount; msgNum++ {
		msg, err := msgMgr.GetMessage(areaID, msgNum)
		if err != nil {
			// Skip deleted or unreadable messages
			log.Printf("WARN: Failed to read message %d in area %d: %v", msgNum, areaID, err)
			continue
		}

		// Check if message is private
		isPrivate := msg.IsPrivate

		// Check if message is read
		isRead := msgNum <= lastRead

		entry := MessageListEntry{
			MsgNum:    msgNum,
			Subject:   msg.Subject,
			From:      msg.From,
			To:        msg.To,
			IsPrivate: isPrivate,
			IsRead:    isRead,
		}
		entries = append(entries, entry)
	}

	return entries, lastRead, nil
}

// drawMessageListScreen renders the complete message list display
func drawMessageListScreen(terminal *term.Terminal, state *MessageListState, areaName string, confName string, outputMode ansi.OutputMode) error {
	log.Printf("DEBUG: drawMessageListScreen outputMode = %v", outputMode)

	// Reset attributes directly (bypass WriteProcessedBytes to avoid UTF-8 decode issues)
	if _, err := terminal.Write([]byte("\x1b[0m")); err != nil {
		return err
	}

	// Hide cursor for cleaner display
	if err := terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25l"), outputMode); err != nil {
		return err
	}

	// Clear screen
	clearSeq := ansi.ClearScreen()
	if err := terminalio.WriteProcessedBytes(terminal, []byte(clearSeq), outputMode); err != nil {
		return err
	}

	// Move to home position
	if err := terminalio.WriteProcessedBytes(terminal, []byte("\x1b[H"), outputMode); err != nil {
		return err
	}

	// Calculate total pages
	totalPages := (state.TotalMessages + state.ItemsPerPage - 1) / state.ItemsPerPage
	if totalPages < 1 {
		totalPages = 1
	}

	// Draw header with CP437 box characters (bright cyan borders)
	// Top border: ┌─...─┐ (total width: 79 chars = 1 + 77 + 1)
	header := fmt.Sprintf("|11┌%s┐|07\r\n", strings.Repeat("─", 77))
	if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(header)), outputMode); err != nil {
		return err
	}

	// Title line with conference and area name (bright magenta text)
	title := fmt.Sprintf("%s - Message List", areaName)
	if confName != "" && confName != "Local" {
		title = fmt.Sprintf("%s > %s - Message List", confName, areaName)
	}
	title = truncateString(title, 75)
	padding := (77 - len(title)) / 2
	titleLine := fmt.Sprintf("|11│|13%s%s%s|11│|07\r\n",
		strings.Repeat(" ", padding),
		title,
		strings.Repeat(" ", 77-padding-len(title)))
	if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(titleLine)), outputMode); err != nil {
		return err
	}

	// Separator: ├─...─┤ (total width: 79 chars = 1 + 77 + 1)
	separator := fmt.Sprintf("|11├%s┤|07\r\n", strings.Repeat("─", 77))
	if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(separator)), outputMode); err != nil {
		return err
	}

	// Column headers (bright white text, total interior: 77 chars)
	// Layout: Status(1) + " N#" (3) + "  " (2) + "Subject" + pad (33) + "    " (4) + "From" + pad (17) + "  " (2) + "To" + pad (15) = 77
	columnHeaders := "|11│|15 N#  Subject                               From               To             |11│|07\r\n"
	if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(columnHeaders)), outputMode); err != nil {
		return err
	}

	// Separator
	if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(separator)), outputMode); err != nil {
		return err
	}

	// Draw messages for current page
	start, end := calculatePagination(len(state.Entries), state.ItemsPerPage, state.CurrentPage)
	for i := start; i < end; i++ {
		isHighlighted := (i - start) == state.SelectedIndex
		if err := drawMessageListLine(terminal, state.Entries[i], isHighlighted, outputMode); err != nil {
			return err
		}
	}

	// Fill remaining lines with empty rows if needed
	linesShown := end - start
	for i := linesShown; i < state.ItemsPerPage; i++ {
		emptyLine := fmt.Sprintf("|11│|07%s|11│|07\r\n", strings.Repeat(" ", 77))
		if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(emptyLine)), outputMode); err != nil {
			return err
		}
	}

	// Bottom separator
	if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(separator)), outputMode); err != nil {
		return err
	}

	// Pagination info (centered, bright cyan text)
	pageInfo := fmt.Sprintf("Page %d of %d [%d-%d of %d messages]",
		state.CurrentPage, totalPages,
		start+1, end, state.TotalMessages)
	pageInfoLen := len(pageInfo)
	if pageInfoLen > 77 {
		pageInfoLen = 77
		pageInfo = truncateString(pageInfo, 77)
	}
	leftPad := (77 - pageInfoLen) / 2
	rightPad := 77 - pageInfoLen - leftPad
	pageInfoPadded := fmt.Sprintf("%s%s%s", strings.Repeat(" ", leftPad), pageInfo, strings.Repeat(" ", rightPad))
	pageLine := fmt.Sprintf("|11│|11%s|11│|07\r\n", pageInfoPadded)
	if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(pageLine)), outputMode); err != nil {
		return err
	}

	// Separator
	if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(separator)), outputMode); err != nil {
		return err
	}

	// Help/command line (centered, bright magenta text, with CP437 arrows: \x18=↑, \x19=↓)
	helpText := "\x18/\x19: Navigate  Enter: Read  Q: Quit"
	helpTextLen := len(helpText)
	leftPad = (77 - helpTextLen) / 2
	rightPad = 77 - helpTextLen - leftPad
	helpPadded := fmt.Sprintf("%s%s%s", strings.Repeat(" ", leftPad), helpText, strings.Repeat(" ", rightPad))
	helpLine := fmt.Sprintf("|11│|13%s|11│|07\r\n", helpPadded)
	if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(helpLine)), outputMode); err != nil {
		return err
	}

	// Bottom border: └─...─┘ (total width: 79 chars = 1 + 77 + 1)
	// NOTE: No \r\n at end to prevent scrolling when cursor reaches bottom of terminal
	footer := fmt.Sprintf("|11└%s┘|07", strings.Repeat("─", 77))
	if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(footer)), outputMode); err != nil {
		return err
	}

	return nil
}

// drawMessageListLine renders a single message line with optional highlighting
func drawMessageListLine(terminal *term.Terminal, entry MessageListEntry, isHighlighted bool, outputMode ansi.OutputMode) error {
	// Format status character (aware of highlight state)
	statusStr := formatStatusChar(entry, isHighlighted)

	// Format message number (right-aligned, 3 chars)
	numStr := fmt.Sprintf("%3d", entry.MsgNum)

	// Truncate fields to fit columns
	// Layout: Status(1) + Num(3) + Sep(2) + Subject(33) + Sep(4) + From(17) + Sep(2) + To(15) = 77 chars
	subject := truncateString(entry.Subject, 33)
	from := truncateString(entry.From, 17)
	to := truncateString(entry.To, 15)

	// Format the line (total width: 79 chars including borders)
	// Interior: Status(1) + Num(3) + Spaces(2) + Subject(33) + Spaces(4) + From(17) + Spaces(2) + To(15) = 77
	// Total: Border(1) + Interior(77) + Border(1) = 79
	var line string
	if isHighlighted {
		// Use ANSI reverse video for highlighting (black on white)
		line = fmt.Sprintf("|11│\x1b[7m%s%s  %-33s    %-17s  %-15s\x1b[27m|11│|07\r\n",
			statusStr,
			numStr,
			subject,
			from,
			to)
	} else {
		// Normal display (bright white text on black)
		line = fmt.Sprintf("|11│|15%s%s  %-33s    %-17s  %-15s|11│|07\r\n",
			statusStr,
			numStr,
			subject,
			from,
			to)
	}

	return terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(line)), outputMode)
}

// drawMessageLineAtRow draws a single message line at a specific screen row (optimized refresh)
func drawMessageLineAtRow(terminal *term.Terminal, entry MessageListEntry, row int, isHighlighted bool, outputMode ansi.OutputMode) error {
	// Position cursor at the specified row
	positionCmd := fmt.Sprintf("\x1b[%d;1H", row)
	if err := terminalio.WriteProcessedBytes(terminal, []byte(positionCmd), outputMode); err != nil {
		return err
	}

	// Draw the line
	return drawMessageListLine(terminal, entry, isHighlighted, outputMode)
}

// drawPageInfoAtRow draws the page info line at a specific row
func drawPageInfoAtRow(terminal *term.Terminal, state *MessageListState, row int, outputMode ansi.OutputMode) error {
	// Calculate total pages and message range
	totalPages := (state.TotalMessages + state.ItemsPerPage - 1) / state.ItemsPerPage
	if totalPages < 1 {
		totalPages = 1
	}
	start, end := calculatePagination(len(state.Entries), state.ItemsPerPage, state.CurrentPage)

	// Format page info (centered, bright cyan text)
	pageInfo := fmt.Sprintf("Page %d of %d [%d-%d of %d messages]",
		state.CurrentPage, totalPages,
		start+1, end, state.TotalMessages)
	pageInfoLen := len(pageInfo)
	if pageInfoLen > 77 {
		pageInfoLen = 77
		pageInfo = truncateString(pageInfo, 77)
	}
	leftPad := (77 - pageInfoLen) / 2
	rightPad := 77 - pageInfoLen - leftPad
	pageInfoPadded := fmt.Sprintf("%s%s%s", strings.Repeat(" ", leftPad), pageInfo, strings.Repeat(" ", rightPad))
	pageLine := fmt.Sprintf("|11│|11%s|11│|07", pageInfoPadded)

	// Position cursor and draw
	positionCmd := fmt.Sprintf("\x1b[%d;1H", row)
	if err := terminalio.WriteProcessedBytes(terminal, []byte(positionCmd), outputMode); err != nil {
		return err
	}
	return terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(pageLine)), outputMode)
}

// refreshPageContent redraws message lines and page info for page changes (optimized, no screen clear)
func refreshPageContent(terminal *term.Terminal, state *MessageListState, outputMode ansi.OutputMode) error {
	// Calculate pagination
	start, _ := calculatePagination(len(state.Entries), state.ItemsPerPage, state.CurrentPage)

	// Redraw all message lines for current page
	// Message lines start at row 6 (1: top border, 2: title, 3: sep, 4: headers, 5: sep, 6+: messages)
	startRow := 6
	for i := 0; i < state.ItemsPerPage; i++ {
		row := startRow + i
		actualIndex := start + i

		if actualIndex < len(state.Entries) {
			// Draw message line
			isHighlighted := i == state.SelectedIndex
			if err := drawMessageLineAtRow(terminal, state.Entries[actualIndex], row, isHighlighted, outputMode); err != nil {
				return err
			}
		} else {
			// Draw empty line for remaining slots
			positionCmd := fmt.Sprintf("\x1b[%d;1H", row)
			if err := terminalio.WriteProcessedBytes(terminal, []byte(positionCmd), outputMode); err != nil {
				return err
			}
			emptyLine := fmt.Sprintf("|11│|07%s|11│|07", strings.Repeat(" ", 77))
			if err := terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(emptyLine)), outputMode); err != nil {
				return err
			}
		}
	}

	// Update page info line
	// Page info is at: startRow + itemsPerPage (messages end) + 1 (separator) = startRow + itemsPerPage + 1
	pageInfoRow := startRow + state.ItemsPerPage + 1
	return drawPageInfoAtRow(terminal, state, pageInfoRow, outputMode)
}

// runMessageListNavigation handles keyboard input for list navigation
func runMessageListNavigation(reader *bufio.Reader, state *MessageListState) (action string, selectedMsg int, err error) {
	for {
		keySeq, err := readKeySequence(reader)
		if err != nil {
			return "ERROR", 0, err
		}

		// Calculate total pages
		totalPages := (state.TotalMessages + state.ItemsPerPage - 1) / state.ItemsPerPage
		if totalPages < 1 {
			totalPages = 1
		}

		// Calculate current page boundaries
		start, end := calculatePagination(len(state.Entries), state.ItemsPerPage, state.CurrentPage)
		itemsOnPage := end - start

		switch keySeq {
		case "\x1b[A": // Up arrow
			if state.SelectedIndex > 0 {
				state.SelectedIndex--
				return "REFRESH_LINE", 0, nil
			} else if state.CurrentPage > 1 {
				// Move to previous page, select last item
				state.CurrentPage--
				start, end := calculatePagination(len(state.Entries), state.ItemsPerPage, state.CurrentPage)
				state.SelectedIndex = (end - start) - 1
				return "REFRESH_FULL", 0, nil
			}

		case "\x1b[B": // Down arrow
			if state.SelectedIndex < itemsOnPage-1 {
				state.SelectedIndex++
				return "REFRESH_LINE", 0, nil
			} else if state.CurrentPage < totalPages {
				// Move to next page, select first item
				state.CurrentPage++
				state.SelectedIndex = 0
				return "REFRESH_FULL", 0, nil
			}

		case "\x1b[5~": // Page Up
			if state.CurrentPage > 1 {
				state.CurrentPage--
				state.SelectedIndex = 0
				return "REFRESH_FULL", 0, nil
			}

		case "\x1b[6~": // Page Down
			if state.CurrentPage < totalPages {
				state.CurrentPage++
				state.SelectedIndex = 0
				return "REFRESH_FULL", 0, nil
			}

		case "\x1b[H": // Home
			if state.CurrentPage != 1 || state.SelectedIndex != 0 {
				state.CurrentPage = 1
				state.SelectedIndex = 0
				return "REFRESH_FULL", 0, nil
			}

		case "\x1b[F": // End
			lastPage := totalPages
			state.CurrentPage = lastPage
			start, end := calculatePagination(len(state.Entries), state.ItemsPerPage, state.CurrentPage)
			state.SelectedIndex = (end - start) - 1
			return "REFRESH_FULL", 0, nil

		case "\r", "\n": // Enter - read selected message
			actualIndex := start + state.SelectedIndex
			if actualIndex < len(state.Entries) {
				return "READ", state.Entries[actualIndex].MsgNum, nil
			}

		case "Q", "q": // Quit
			return "QUIT", 0, nil

		case "?": // Help (future enhancement)
			// TODO: Show help screen
			return "REFRESH_FULL", 0, nil
		}
	}
}

// runListMsgs is the main entry point for the message list command
func runListMsgs(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {

	// Validate user is logged in
	if currentUser == nil {
		log.Printf("WARN: Node %d: LISTMSGS called without logged in user.", nodeNumber)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.MsgListLoginRequired)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	// Check if user has selected a message area
	currentAreaID := currentUser.CurrentMessageAreaID
	if currentAreaID == 0 {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.MsgListNoAreaSelected)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	// Get area information
	area, found := e.MessageMgr.GetAreaByID(currentAreaID)
	if !found {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.MsgListAreaNotFound)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	// Get conference name
	confName := "Local"
	if currentUser.CurrentMsgConferenceID != 0 && e.ConferenceMgr != nil {
		if conf, found := e.ConferenceMgr.GetByID(currentUser.CurrentMsgConferenceID); found {
			confName = conf.Name
		}
	}

	// Build message list
	entries, lastRead, err := buildMessageList(e.MessageMgr, currentAreaID, currentUser.Handle)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to build message list: %v", nodeNumber, err)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.MsgListLoadError)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	// Check if area is empty
	if len(entries) == 0 {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(e.LoadedStrings.MsgListNoMessages)), outputMode)
		time.Sleep(2 * time.Second)
		return currentUser, "", nil
	}

	// Use passed termHeight (from user preferences or PTY detection in main.go)
	// Don't read from PTY here as it may not reflect user's saved preferences

	// Calculate items per page based on terminal height
	// Header: 5 lines (top border, title, separator, column headers, separator)
	// Footer: 5 lines (separator, page info, separator, help, bottom border)
	headerHeight := 5
	footerHeight := 5
	itemsPerPage := termHeight - headerHeight - footerHeight
	if itemsPerPage < 3 {
		itemsPerPage = 3 // Minimum
	}

	// Initialize state
	state := &MessageListState{
		AreaID:        currentAreaID,
		TotalMessages: len(entries),
		Entries:       entries,
		CurrentPage:   1,
		ItemsPerPage:  itemsPerPage,
		SelectedIndex: 0,
		LastRead:      lastRead,
	}

	// Main navigation loop
	reader := bufio.NewReader(s)

	// Ensure cursor is restored when exiting
	defer func() {
		terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h"), outputMode) // Show cursor
	}()

	// Draw initial screen
	if err := drawMessageListScreen(terminal, state, area.Name, confName, outputMode); err != nil {
		log.Printf("ERROR: Node %d: Failed to draw message list: %v", nodeNumber, err)
		return currentUser, "", err
	}

	previousIndex := state.SelectedIndex // Track previous selection for optimized refresh

	for {
		// Handle navigation
		action, selectedMsg, err := runMessageListNavigation(reader, state)
		if err != nil {
			log.Printf("ERROR: Node %d: Navigation error: %v", nodeNumber, err)
			return currentUser, "LOGOFF", err
		}

		switch action {
		case "QUIT":
			return currentUser, "", nil

		case "READ":
			// Get terminal dimensions from user preferences
			tw := currentUser.ScreenWidth
			if tw == 0 {
				tw = 80
			}
			th := currentUser.ScreenHeight
			if th == 0 {
				th = 24
			}

			// Call message reader
			_, nextAction, err := runMessageReader(e, s, terminal, userManager,
				currentUser, nodeNumber, sessionStartTime, outputMode,
				selectedMsg, state.TotalMessages, false, tw, th)

			if err != nil {
				log.Printf("ERROR: Node %d: Message reader error: %v", nodeNumber, err)
				return currentUser, "", err
			}

			// Handle return action
			if nextAction == "LOGOFF" {
				return currentUser, "LOGOFF", nil
			}
			// Redraw full screen after returning from reader
			if err := drawMessageListScreen(terminal, state, area.Name, confName, outputMode); err != nil {
				log.Printf("ERROR: Node %d: Failed to redraw message list: %v", nodeNumber, err)
				return currentUser, "", err
			}
			previousIndex = state.SelectedIndex // Track highlighted line after redraw

		case "REFRESH_FULL":
			// Optimized page refresh (only redraw message lines and page info, no screen clear)
			if err := refreshPageContent(terminal, state, outputMode); err != nil {
				log.Printf("ERROR: Node %d: Failed to refresh page content: %v", nodeNumber, err)
				return currentUser, "", err
			}
			previousIndex = state.SelectedIndex // Track highlighted line after redraw

		case "REFRESH_LINE":
			// Optimized refresh: only redraw changed lines
			// Message lines start at row 6 (1: top border, 2: title, 3: sep, 4: headers, 5: sep, 6+: messages)
			start, _ := calculatePagination(len(state.Entries), state.ItemsPerPage, state.CurrentPage)

			// Unhighlight previous line
			if previousIndex >= 0 && previousIndex < state.ItemsPerPage {
				actualIndex := start + previousIndex
				if actualIndex < len(state.Entries) {
					row := 6 + previousIndex
					drawMessageLineAtRow(terminal, state.Entries[actualIndex], row, false, outputMode)
				}
			}

			// Highlight current line
			if state.SelectedIndex >= 0 && state.SelectedIndex < state.ItemsPerPage {
				actualIndex := start + state.SelectedIndex
				if actualIndex < len(state.Entries) {
					row := 6 + state.SelectedIndex
					drawMessageLineAtRow(terminal, state.Entries[actualIndex], row, true, outputMode)
				}
			}

			previousIndex = state.SelectedIndex

		case "ERROR":
			return currentUser, "LOGOFF", err
		}
	}
}
