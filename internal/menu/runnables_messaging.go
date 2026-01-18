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
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/stlalpha/vision3/internal/editor"
	"github.com/stlalpha/vision3/internal/logging"
	"github.com/stlalpha/vision3/internal/message"
	terminalPkg "github.com/stlalpha/vision3/internal/terminal"
	"github.com/stlalpha/vision3/internal/user"
)

// runListMessageAreas handles displaying the list of message areas.
func runListMessageAreas(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	logging.Debug(" Node %d: Running LISTMSGAR", nodeNumber)

	// 1. Define Template filenames and paths
	topTemplateFilename := "MSGAREA.TOP"
	midTemplateFilename := "MSGAREA.MID"
	botTemplateFilename := "MSGAREA.BOT"
	templateDir := filepath.Join(e.MenuSetPath, "templates")
	topTemplatePath := filepath.Join(templateDir, topTemplateFilename)
	midTemplatePath := filepath.Join(templateDir, midTemplateFilename)
	botTemplatePath := filepath.Join(templateDir, botTemplateFilename)

	// 2. Load Template Files
	topTemplateBytes, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplateBytes, errBot := os.ReadFile(botTemplatePath)

	if errTop != nil || errMid != nil || errBot != nil {
		log.Printf("ERROR: Node %d: Failed to load one or more MSGAREA template files: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading Message Area screen templates.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading MSGAREA templates")
	}

	// 3. Process Pipe Codes in Templates FIRST
	processedTopTemplate := string(terminal.ProcessPipeCodes(topTemplateBytes))
	processedMidTemplate := string(terminal.ProcessPipeCodes(midTemplateBytes))
	processedBotTemplate := string(terminal.ProcessPipeCodes(botTemplateBytes))

	// 4. Get message area list data from MessageManager
	// TODO: Apply ACS filtering here based on currentUser
	areas := e.MessageMgr.ListAreas()

	// 5. Build the output string using processed templates and data
	var outputBuffer bytes.Buffer
	outputBuffer.Write([]byte(processedTopTemplate)) // Write processed top template

	if len(areas) == 0 {
		logging.Debug(" Node %d: No message areas to display.", nodeNumber)
		// Optional: Add a message like "|07No message areas available.\r\n" if template doesn't handle it
	} else {
		// Iterate through areas and format using processed MSGAREA.MID
		for _, area := range areas {
			line := processedMidTemplate // Start with the pipe-code-processed mid template

			// Process pipe codes in area data
			name := string(terminal.ProcessPipeCodes([]byte(area.Name)))
			desc := string(terminal.ProcessPipeCodes([]byte(area.Description)))
			idStr := strconv.Itoa(area.ID)
			tag := string(terminal.ProcessPipeCodes([]byte(area.Tag))) // Tag might have codes?

			// Replace placeholders in the MID template
			// Expected placeholders: ^ID, ^TAG, ^NA, ^DS (adjust if different)
			line = strings.ReplaceAll(line, "^ID", idStr)
			line = strings.ReplaceAll(line, "^TAG", tag)
			line = strings.ReplaceAll(line, "^NA", name)
			line = strings.ReplaceAll(line, "^DS", desc)

			outputBuffer.WriteString(line) // Add the fully substituted and processed line
		}
	}

	outputBuffer.Write([]byte(processedBotTemplate)) // Write processed bottom template

	// 6. Clear screen and display the assembled content
	writeErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
	if writeErr != nil {
		log.Printf("ERROR: Node %d: Failed clearing screen for LISTMSGAR: %v", nodeNumber, writeErr)
		return nil, "", writeErr
	}

	// Use WriteProcessedBytes for the assembled template content
	processedContent := outputBuffer.Bytes() // Contains already-processed ANSI bytes
	_, wErr := terminal.Write(processedContent)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing LISTMSGAR output: %v", nodeNumber, wErr)
		return nil, "", wErr
	}

	// 7. Wait for Enter using configured PauseString
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

	logging.Debug(" Node %d: Writing LISTMSGAR pause prompt. Mode: %d, Bytes: %q", nodeNumber, outputMode, string(pauseBytesToWrite))
	// Log hex bytes before writing
	logging.Debug(" Node %d: Writing LISTMSGAR pause bytes (hex): %x", nodeNumber, pauseBytesToWrite)
	_, wErr = terminal.Write(pauseBytesToWrite)
	if wErr != nil {
		log.Printf("ERROR: Node %d: Failed writing LISTMSGAR pause prompt: %v", nodeNumber, wErr)
	}

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during LISTMSGAR pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading input during LISTMSGAR pause: %v", nodeNumber, err)
			return nil, "", err
		}
		if r == '\r' || r == '\n' { // Check for CR or LF
			break
		}
	}

	return nil, "", nil // Success
}

// runComposeMessage handles the process of composing and saving a new message.
func runComposeMessage(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	logging.Debug(" Node %d: Running COMPOSEMSG with args: %s", nodeNumber, args)

	// 1. Determine Target Area
	var areaTag string
	var area *message.MessageArea // Use pointer type
	var exists bool

	if args == "" {
		// No args provided, use current user's area
		if currentUser == nil {
			log.Printf("WARN: Node %d: COMPOSEMSG called without user and without args.", nodeNumber)
			msg := "\r\n|01Error: Not logged in and no area specified.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return nil, "", nil // Return to menu
		}
		if currentUser.CurrentMessageAreaTag == "" || currentUser.CurrentMessageAreaID <= 0 {
			log.Printf("WARN: Node %d: COMPOSEMSG called by %s, but no current message area is set.", nodeNumber, currentUser.Handle)
			msg := "\r\n|01Error: No current message area selected.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return nil, "", nil // Return to menu
		}
		areaTag = currentUser.CurrentMessageAreaTag
		log.Printf("INFO: Node %d: COMPOSEMSG using current user area tag: %s", nodeNumber, areaTag)
		area, exists = e.MessageMgr.GetAreaByTag(areaTag)
	} else {
		// Args provided, use args as the area tag
		log.Printf("INFO: Node %d: COMPOSEMSG using provided area tag in args: %s", nodeNumber, args)
		areaTag = args
		area, exists = e.MessageMgr.GetAreaByTag(areaTag)
	}

	// Common checks after determining areaTag/area
	if !exists {
		log.Printf("ERROR: Node %d: COMPOSEMSG called with invalid Area Tag: %s", nodeNumber, areaTag)
		msg := fmt.Sprintf("\r\n|01Invalid message area: %s|07\r\n", areaTag)
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu, not an error
	}

	// Check user logged in (required for ACS check and posting)
	if currentUser == nil {
		log.Printf("WARN: Node %d: COMPOSEMSG reached ACS check without logged in user (Area: %s).", nodeNumber, areaTag)
		msg := "\r\n|01Error: You must be logged in to post messages.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	// Check ACSWrite permission for the area and currentUser
	if !checkACS(area.ACSWrite, currentUser, s, terminal, sessionStartTime) {
		log.Printf("WARN: Node %d: User %s denied post access to area %s (%s)", nodeNumber, currentUser.Handle, area.Tag, area.ACSWrite)
		// TODO: Display user-friendly error message (e.g., Access Denied String)
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu, not an error
	}

	// === BEGIN MOVED SUBJECT LOGIC ===
	// 2. Prompt for Subject
	prompt := e.LoadedStrings.MsgTitleStr
	if prompt == "" {
		log.Printf("WARN: Node %d: MsgTitleStr is empty in strings config. Using fallback.", nodeNumber)
		prompt = "|07Subject: |15"
	}
	// Add newline for spacing before prompt
	terminal.Write([]byte("\r\n"))
	wErr := terminal.DisplayContent([]byte(prompt))
	if wErr != nil {
		log.Printf("WARN: Node %d: Failed to write subject prompt: %v", nodeNumber, wErr)
	}

	subject, err := terminal.ReadLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during subject input.", nodeNumber)
			return nil, "LOGOFF", io.EOF
		}
		log.Printf("ERROR: Node %d: Failed reading subject input: %v", nodeNumber, err)
		terminal.Write([]byte("\r\nError reading subject.\r\n"))
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}
	subject = strings.TrimSpace(subject)
	// Allow empty subject for now, set default later if needed after editor
	// if subject == "" {
	// 	subject = "(no subject)" // Default subject
	// }
	// Check if subject is truly empty after defaulting
	// if strings.TrimSpace(subject) == "" {
	// 	terminal.Write([]byte("\r\nSubject cannot be empty. Post cancelled.\r\n"))
	// 	time.Sleep(1 * time.Second)
	// 	return nil, "", nil // Return to menu
	// }
	// === END MOVED SUBJECT LOGIC ===

	// 3. Call the Editor
	// We need to temporarily leave the menu loop's terminal control
	// The editor.RunEditor will take over the PTY.
	// Reverted to logic from commit 32f3c59...

	logging.Debug(" Node %d: Clearing screen before calling editor.RunEditor", nodeNumber)
	terminal.DisplayContent([]byte("\x1b[2J\x1b[H")) // Clear screen before editor (as per 32f3c59)
	logging.Debug(" Node %d: Calling editor.RunEditor for area %s", nodeNumber, area.Tag)

	// Get TERM env var
	termType := "unknown"
	for _, env := range s.Environ() {
		if strings.HasPrefix(env, "TERM=") {
			termType = strings.TrimPrefix(env, "TERM=")
			break
		}
	}
	logging.Debug(" Node %d: Passing TERM=%s to editor.RunEditor", nodeNumber, termType)

	body, saved, err := editor.RunEditor("", s, s, termType) // Pass session as input/output and termType
	logging.Debug(" Node %d: editor.RunEditor returned. Error: %v, Saved: %v, Body length: %d", nodeNumber, err, saved, len(body))

	if err != nil {
		log.Printf("ERROR: Node %d: Editor failed for user %s: %v", nodeNumber, currentUser.Handle, err)
		// Restore original error handling from 32f3c59 (propagate error)
		// Need to handle redraw in the main loop potentially.
		// Consider clearing screen here before returning?
		// terminal.Write([]byte(terminalPkg.ClearScreen())) // Optional clear on error
		return nil, "", fmt.Errorf("editor error: %w", err)
	}

	// IMPORTANT: Need to redraw the current menu screen after editor exits!
	// This needs to be handled by the main menu loop after this function returns (CONTINUE).
	// Clear screen *after* editor exits (as per 32f3c59 placeholder logic)
	terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
	// terminal.Write([]byte("\\r\\nEditor exited. Processing message...\\r\\n")) // Removed placeholder

	if !saved {
		log.Printf("INFO: Node %d: User %s aborted message composition for area %s.", nodeNumber, currentUser.Handle, area.Tag)
		// Restore message from 32f3c59
		terminal.Write([]byte("\r\nMessage aborted.\r\n"))
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to current menu
	}

	if strings.TrimSpace(body) == "" {
		log.Printf("INFO: Node %d: User %s saved empty message for area %s.", nodeNumber, currentUser.Handle, area.Tag)
		// Restore message from 32f3c59
		terminal.Write([]byte("\r\nMessage body empty. Aborting post.\r\n"))
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to current menu
	}

	// === MOVED: Subject prompt is now before editor ===
	// Set default subject if user left it blank earlier
	if subject == "" {
		subject = "(no subject)" // Default subject
	}

	// 4. Construct the Message (Subject is already set)
	currentNodeID := "1:1/100" // Placeholder - get from config or session

	newMessage := message.Message{
		ID:           uuid.New(),
		AreaID:       area.ID,
		FromUserName: currentUser.Handle,
		FromNodeID:   currentNodeID,        // TODO: Get actual node ID
		ToUserName:   message.MsgToUserAll, // Public message
		Subject:      subject,
		Body:         body,
		PostedAt:     time.Now(), // Set creation time
		IsPrivate:    false,
		// ReplyToID, Path, Attributes, ReadBy can be added later
	}

	// 5. Save the Message
	err = e.MessageMgr.AddMessage(area.ID, newMessage)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to save message from user %s to area %s: %v", nodeNumber, currentUser.Handle, area.Tag, err)
		// TODO: Display user-friendly error
		terminal.Write([]byte("\r\n|01Error saving message!|07\r\n"))
		time.Sleep(2 * time.Second)
		return nil, "", fmt.Errorf("failed saving message: %w", err) // Return error for now
	}

	// 6. Confirmation
	log.Printf("INFO: Node %d: User %s successfully posted message %s to area %s", nodeNumber, currentUser.Handle, newMessage.ID, area.Tag)
	// TODO: Use string config for confirmation message
	terminal.Write([]byte("\r\n|02Message Posted!|07\r\n"))
	time.Sleep(1 * time.Second)

	// Return to current menu. The menu loop should handle redraw because we return CONTINUE action ("", nil).
	return nil, "", nil
}

// runPromptAndComposeMessage lists areas, prompts for selection, checks permissions, and calls runComposeMessage.
func runPromptAndComposeMessage(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	logging.Debug(" Node %d: Running runPromptAndComposeMessage", nodeNumber)

	if currentUser == nil {
		log.Printf("WARN: Node %d: runPromptAndComposeMessage called without logged in user.", nodeNumber)
		// Display user-friendly error
		msg := "\r\n|01Error: You must be logged in to post messages.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing login required message: %v", nodeNumber, wErr)
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	// 1. Display available message areas (adapted from runListMessageAreas, without pause)
	topTemplateFilename := "MSGAREA.TOP"
	midTemplateFilename := "MSGAREA.MID"
	botTemplateFilename := "MSGAREA.BOT" // We'll use BOT template differently here
	templateDir := filepath.Join(e.MenuSetPath, "templates")
	topTemplatePath := filepath.Join(templateDir, topTemplateFilename)
	midTemplatePath := filepath.Join(templateDir, midTemplateFilename)
	botTemplatePath := filepath.Join(templateDir, botTemplateFilename) // Load BOT template

	topTemplateBytes, errTop := os.ReadFile(topTemplatePath)
	midTemplateBytes, errMid := os.ReadFile(midTemplatePath)
	botTemplateBytes, errBot := os.ReadFile(botTemplatePath) // Load BOT template

	if errTop != nil || errMid != nil || errBot != nil { // Check BOT error too
		log.Printf("ERROR: Node %d: Failed to load one or more MSGAREA template files for prompt: TOP(%v), MID(%v), BOT(%v)", nodeNumber, errTop, errMid, errBot)
		msg := "\r\n|01Error loading Message Area screen templates.|07\r\n"
		terminal.DisplayContent([]byte(msg))
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading MSGAREA templates for prompt")
	}

	processedTopTemplate := string(terminal.ProcessPipeCodes(topTemplateBytes))
	processedMidTemplate := string(terminal.ProcessPipeCodes(midTemplateBytes))
	processedBotTemplate := string(terminal.ProcessPipeCodes(botTemplateBytes)) // Process BOT template

	areas := e.MessageMgr.ListAreas() // Get all areas
	// Filter areas based on read access (for listing)
	// For now, list all areas, permission check happens later on selection.
	// TODO: Implement ACSRead filtering here if needed for the list display.

	terminal.DisplayContent([]byte("\x1b[2J\x1b[H")) // Clear before displaying list
	terminal.Write([]byte(processedTopTemplate))     // Write TOP

	if len(areas) == 0 {
		logging.Debug(" Node %d: No message areas available to post in.", nodeNumber)
		terminal.Write([]byte("\r\n|07No message areas available.|07\r\n"))
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	for _, area := range areas {
		line := processedMidTemplate
		name := string(terminal.ProcessPipeCodes([]byte(area.Name)))
		desc := string(terminal.ProcessPipeCodes([]byte(area.Description)))
		idStr := strconv.Itoa(area.ID)
		tag := string(terminal.ProcessPipeCodes([]byte(area.Tag)))

		line = strings.ReplaceAll(line, "^ID", idStr)
		line = strings.ReplaceAll(line, "^TAG", tag)
		line = strings.ReplaceAll(line, "^NA", name)
		line = strings.ReplaceAll(line, "^DS", desc)

		terminal.Write([]byte(line)) // Write MID for each area
	}

	terminal.Write([]byte(processedBotTemplate)) // Write BOT

	// 2. Prompt for Area Selection
	// TODO: Use a configurable string for this prompt
	prompt := "\r\n|07Enter Area ID or Tag to Post In (or Enter to cancel): |15"
	logging.Debug(" Node %d: Writing prompt for message area selection bytes (hex): %x", nodeNumber, terminal.ProcessPipeCodes([]byte(prompt)))
	wErr := terminal.DisplayContent([]byte(prompt))
	if wErr != nil {
		log.Printf("WARN: Node %d: Failed to write area selection prompt: %v", nodeNumber, wErr)
	}

	input, err := terminal.ReadLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during area selection.", nodeNumber)
			return nil, "LOGOFF", io.EOF
		}
		log.Printf("ERROR: Node %d: Failed reading area selection input: %v", nodeNumber, err)
		return nil, "", fmt.Errorf("failed reading area selection: %w", err)
	}

	selectedAreaStr := strings.TrimSpace(input)
	if selectedAreaStr == "" {
		log.Printf("INFO: Node %d: User cancelled message posting.", nodeNumber)
		// TODO: Need to redraw the menu screen!
		terminal.Write([]byte("\r\nPost cancelled.\r\n"))
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil // Return to current menu
	}

	// 3. Find Selected Area and Check Permissions
	var selectedArea *message.MessageArea // CORRECTED TYPE to pointer
	var foundArea bool

	// Try parsing as ID first
	if areaID, err := strconv.Atoi(selectedAreaStr); err == nil {
		selectedArea, foundArea = e.MessageMgr.GetAreaByID(areaID)
	}

	// If not found by ID, try by Tag (case-insensitive)
	if !foundArea {
		selectedArea, foundArea = e.MessageMgr.GetAreaByTag(strings.ToUpper(selectedAreaStr))
	}

	if !foundArea {
		log.Printf("WARN: Node %d: Invalid area selection '%s' by user %s.", nodeNumber, selectedAreaStr, currentUser.Handle)
		// TODO: Use configurable string
		msg := fmt.Sprintf("\r\n|01Invalid area: %s|07\r\n", selectedAreaStr)
		terminal.DisplayContent([]byte(msg))
		time.Sleep(1 * time.Second)
		// TODO: Need to redraw menu
		return nil, "", nil // Return to menu
	}

	// Check write permission
	if !checkACS(selectedArea.ACSWrite, currentUser, s, terminal, sessionStartTime) {
		log.Printf("WARN: Node %d: User %s denied post access to selected area %s (%s)", nodeNumber, currentUser.Handle, selectedArea.Tag, selectedArea.ACSWrite)
		// TODO: Use configurable string for access denied
		msg := fmt.Sprintf("\r\n|01Access denied to post in area: %s|07\r\n", selectedArea.Name)
		terminal.DisplayContent([]byte(msg))
		time.Sleep(1 * time.Second)
		// TODO: Need to redraw menu
		return nil, "", nil // Return to menu
	}

	log.Printf("INFO: Node %d: User %s selected area %s (%s) to post in.", nodeNumber, currentUser.Handle, selectedArea.Name, selectedArea.Tag)

	// 4. Call runComposeMessage with the selected Area Tag
	// Pass the area tag as the argument string
	return runComposeMessage(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, selectedArea.Tag, outputMode)
}

// runReadMsgs handles reading messages from the user's current area.
func runReadMsgs(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	logging.Debug(" Node %d: Running READMSGS", nodeNumber)

	// 1. Check User and Area
	if currentUser == nil {
		log.Printf("WARN: Node %d: READMSGS called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to read messages.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	currentAreaID := currentUser.CurrentMessageAreaID
	currentAreaTag := currentUser.CurrentMessageAreaTag

	if currentAreaID <= 0 || currentAreaTag == "" {
		log.Printf("WARN: Node %d: User %s has no current message area set (ID: %d, Tag: %s).", nodeNumber, currentUser.Handle, currentAreaID, currentAreaTag)
		msg := "\r\n|01Error: No message area selected.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	log.Printf("INFO: Node %d: User %s checking messages for Area ID %d (%s)", nodeNumber, currentUser.Handle, currentAreaID, currentAreaTag)

	// --- NEW LOGIC START ---

	// 2. Fetch total message count first
	totalMessageCount, err := e.MessageMgr.GetMessageCountForArea(currentAreaID)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to get total message count for area %d: %v", nodeNumber, currentAreaID, err)
		msg := fmt.Sprintf("\r\n|01Error loading message info for area %s.|07\r\n", currentAreaTag)
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", err
	}

	if totalMessageCount == 0 {
		log.Printf("INFO: Node %d: No messages found in area %s (%d).", nodeNumber, currentAreaTag, currentAreaID)
		msg := fmt.Sprintf("\r\n|07No messages in area |15%s|07.\r\n", currentAreaTag)
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	// 3. Check for New Messages
	lastReadID := "" // Default to empty (fetch all if no record)
	if currentUser.LastReadMessageIDs != nil {
		lastReadID = currentUser.LastReadMessageIDs[currentAreaID]
	}
	newCount, err := e.MessageMgr.GetNewMessageCount(currentAreaID, lastReadID)
	if err != nil {
		log.Printf("ERROR: Node %d: Failed to get new message count for area %d: %v", nodeNumber, currentAreaID, err)
		msg := fmt.Sprintf("\r\n|01Error checking for new messages in area %s.|07\r\n", currentAreaTag)
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", err
	}

	var messages []message.Message
	currentMessageIndex := 0
	totalMessages := 0 // Will be set based on context (new or all)

	var readingNewMessages bool // Flag to track if we started by reading new messages

	if newCount > 0 {
		// 4a. Load Only New Messages
		readingNewMessages = true // <<< SET FLAG
		log.Printf("INFO: Node %d: Found %d new messages in area %d since ID '%s'. Loading new.", nodeNumber, newCount, currentAreaID, lastReadID)
		messages, err = e.MessageMgr.GetMessagesForArea(currentAreaID, lastReadID)
		if err != nil {
			log.Printf("ERROR: Node %d: Failed to load new messages for area %d: %v", nodeNumber, currentAreaID, err)
			msg := fmt.Sprintf("\r\n|01Error loading new messages for area %s.|07\r\n", currentAreaTag)
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return nil, "", err
		}
		totalMessages = len(messages) // Total for the reader loop is just the new ones
		currentMessageIndex = 0       // Start at the first new message
	} else {
		// 4b. No New Messages - Prompt for specific message
		readingNewMessages = false // <<< SET FLAG
		log.Printf("INFO: Node %d: No new messages in area %d since ID '%s'. Prompting for specific message.", nodeNumber, currentAreaID, lastReadID)
		noNewMsg := fmt.Sprintf("\r\n|07No new messages in area |15%s|07.", currentAreaTag)
		totalMsg := fmt.Sprintf(" |07Total messages: |15%d|07.", totalMessageCount)
		promptMsg := fmt.Sprintf("\r\n|07Read message # (|151-%d|07, |15Enter|07=Cancel): |15", totalMessageCount)

		terminal.DisplayContent([]byte(noNewMsg + totalMsg))
		terminal.DisplayContent([]byte(promptMsg))

		input, readErr := terminal.ReadLine()
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during message number input.", nodeNumber)
				return nil, "LOGOFF", io.EOF // Signal logoff
			}
			log.Printf("ERROR: Node %d: Failed reading message number input: %v", nodeNumber, readErr)
			return nil, "", readErr // Return error
		}

		selectedNumStr := strings.TrimSpace(input)
		if selectedNumStr == "" {
			log.Printf("INFO: Node %d: User cancelled message reading.", nodeNumber)
			return nil, "", nil // Return to menu
		}

		selectedNum, parseErr := strconv.Atoi(selectedNumStr)
		if parseErr != nil || selectedNum < 1 || selectedNum > totalMessageCount {
			log.Printf("WARN: Node %d: Invalid message number input: '%s'", nodeNumber, selectedNumStr)
			msg := fmt.Sprintf("\r\n|01Invalid message number: %s|07\r\n", selectedNumStr)
			terminal.DisplayContent([]byte(msg))
			time.Sleep(1 * time.Second)
			return nil, "", nil // Return to menu
		}

		// Load all messages for reading the specific one
		logging.Debug(" Node %d: Loading all %d messages to read #%d", nodeNumber, totalMessageCount, selectedNum)
		allMessages, err := e.MessageMgr.GetMessagesForArea(currentAreaID, "") // Load all
		if err != nil {
			log.Printf("ERROR: Node %d: Failed to load all messages for area %d: %v", nodeNumber, currentAreaID, err)
			msg := fmt.Sprintf("\r\n|01Error loading messages for area %s.|07\r\n", currentAreaTag)
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil { /* Log? */
			}
			time.Sleep(1 * time.Second)
			return nil, "", err
		}
		messages = allMessages                // Use all messages for the reader
		totalMessages = len(messages)         // Total for loop is all messages
		currentMessageIndex = selectedNum - 1 // Set index to the chosen message
	}

	// --- NEW LOGIC END ---

	// 4. Load Templates (Load once before loop)
	templateDir := filepath.Join(e.MenuSetPath, "templates")
	headTemplatePath := filepath.Join(templateDir, "MSGHEAD.TPL")
	promptTemplatePath := filepath.Join(templateDir, "MSGREAD.PROMPT")

	headTemplateBytes, errHead := os.ReadFile(headTemplatePath)
	promptTemplateBytes, errPrompt := os.ReadFile(promptTemplatePath)

	if errHead != nil || errPrompt != nil {
		log.Printf("ERROR: Node %d: Failed to load MSGHEAD/MSGREAD templates: Head(%v), Prompt(%v)", nodeNumber, errHead, errPrompt)
		msg := "\r\n|01Error loading message display templates.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", fmt.Errorf("failed loading message templates")
	}
	processedHeadTemplate := string(terminal.ProcessPipeCodes(headTemplateBytes))
	processedPromptTemplate := string(terminal.ProcessPipeCodes(promptTemplateBytes))

	// 5. Main Reader Loop
readerLoop:
	for {
		// --- Get Terminal Dimensions ---
		termWidth := 80  // Default width
		termHeight := 24 // Default height
		ptyReq, _, isPty := s.Pty()
		if isPty && ptyReq.Window.Width > 0 && ptyReq.Window.Height > 0 {
			termWidth = ptyReq.Window.Width
			termHeight = ptyReq.Window.Height
			logging.Debug(" [runReadMsgs] Terminal dimensions: %d x %d", termWidth, termHeight)
		} else {
			log.Printf("WARN: [runReadMsgs] Could not get PTY dimensions, using default %d x %d", termWidth, termHeight)
		}

		// --- Calculate Available Height (Estimate) ---
		// Count lines in header and prompt templates (simple newline count)
		headerLines := strings.Count(processedHeadTemplate, "\n") + 1 // +1 because template might not end with newline
		promptLines := strings.Count(processedPromptTemplate, "\n") + 1
		// Add extra lines for spacing (e.g., CRLF before body, CRLF before prompt)
		headerAndPromptLines := headerLines + promptLines + 2
		bodyAvailableHeight := termHeight - headerAndPromptLines
		if bodyAvailableHeight < 1 {
			bodyAvailableHeight = 1 // Ensure at least one line is available
		}
		logging.Debug(" [runReadMsgs] HeaderLines: %d, PromptLines: %d, Available Body Height: %d", headerLines, promptLines, bodyAvailableHeight)

		// 5.1 Get Current Message
		currentMsg := messages[currentMessageIndex]

		// --- Determine Absolute Message Number ---
		absoluteMessageNumber := 0
		if readingNewMessages {
			// Calculate the number of messages *before* the new ones
			// Note: totalMessageCount and newCount were fetched earlier
			numOldMessages := totalMessageCount - newCount
			absoluteMessageNumber = numOldMessages + 1 + currentMessageIndex // 1-based index
		} else {
			// When not reading new, currentMessageIndex is relative to the *full* list
			absoluteMessageNumber = currentMessageIndex + 1 // 1-based index
		}
		// --- End Absolute Message Number ---

		// 5.2 Create Placeholders
		placeholders := map[string]string{
			"|CA":     currentAreaTag,
			"|MNUM":   strconv.Itoa(absoluteMessageNumber), // Use absolute number
			"|MTOTAL": strconv.Itoa(totalMessageCount),     // Use total count for the area
			"|MFROM":  string(terminal.ProcessPipeCodes([]byte(currentMsg.FromUserName))),
			"|MTO":    string(terminal.ProcessPipeCodes([]byte(currentMsg.ToUserName))),
			"|MSUBJ":  string(terminal.ProcessPipeCodes([]byte(currentMsg.Subject))),
			"|MDATE":  currentMsg.PostedAt.Format(time.RFC822), // Use a standard format like RFC822
			// Add more placeholders from message struct if needed by templates
		}

		// 5.3 Substitute into Templates (Explicit Order)
		displayHeader := processedHeadTemplate
		displayPrompt := processedPromptTemplate

		// Replace known placeholders in a fixed order
		displayHeader = strings.ReplaceAll(displayHeader, "|CA", placeholders["|CA"])
		displayHeader = strings.ReplaceAll(displayHeader, "|MNUM", placeholders["|MNUM"])
		displayHeader = strings.ReplaceAll(displayHeader, "|MTOTAL", placeholders["|MTOTAL"])
		displayHeader = strings.ReplaceAll(displayHeader, "|MFROM", placeholders["|MFROM"])
		displayHeader = strings.ReplaceAll(displayHeader, "|MTO", placeholders["|MTO"])
		displayHeader = strings.ReplaceAll(displayHeader, "|MSUBJ", placeholders["|MSUBJ"])
		displayHeader = strings.ReplaceAll(displayHeader, "|MDATE", placeholders["|MDATE"])

		displayPrompt = strings.ReplaceAll(displayPrompt, "|MNUM", placeholders["|MNUM"])
		displayPrompt = strings.ReplaceAll(displayPrompt, "|MTOTAL", placeholders["|MTOTAL"])
		// Add other placeholders needed by displayPrompt here if any

		// --- Remove Old Substitution Loop ---
		// for key, val := range placeholders {
		// 	displayHeader = strings.ReplaceAll(displayHeader, key, val)
		// 	displayPrompt = strings.ReplaceAll(displayPrompt, key, val)
		// }

		// +++ DEBUG LOGGING (Keep temporarily) +++
		logging.Debug(" [runReadMsgs] Placeholders Map: %+v", placeholders)
		logging.Debug(" [runReadMsgs] Final Display Prompt String: %q", displayPrompt)
		// +++ END DEBUG LOGGING +++

		// 5.4 Process Message Body Pipe Codes
		processedBodyString := string(terminal.ProcessPipeCodes([]byte(currentMsg.Body)))
		// 5.4.1 Word Wrap the Processed Body
		wrappedBodyLines := wrapAnsiString(processedBodyString, termWidth)

		// 5.5 Display Output (with Pagination)
		wErr := terminal.DisplayContent([]byte("\x1b[2J\x1b[H"))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed clearing screen: %v", nodeNumber, wErr)
		}

		_, wErr = terminal.Write([]byte(displayHeader))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing message header: %v", nodeNumber, wErr)
		}

		// -- Display Body with Pagination --
		linesDisplayedThisPage := 0
		for lineIdx, line := range wrappedBodyLines {
			if linesDisplayedThisPage == 0 {
				// Add CRLF before the first line of the body (or first line of a new page)
				_, wErr = terminal.Write([]byte("\r\n"))
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing CRLF before body line: %v", nodeNumber, wErr)
				}
			}

			_, wErr = terminal.Write([]byte(line))
			if wErr != nil {
				log.Printf("ERROR: Node %d: Failed writing wrapped body line %d: %v", nodeNumber, lineIdx, wErr)
			}
			linesDisplayedThisPage++

			// Check if pause is needed (if page is full AND it's not the last line)
			if linesDisplayedThisPage >= bodyAvailableHeight && lineIdx < len(wrappedBodyLines)-1 {
				// Display Pause Prompt using system setting
				pausePrompt := e.LoadedStrings.PauseString
				if pausePrompt == "" {
					pausePrompt = "|07-- More -- (|15Enter|07=Continue, |15Q|07=Quit) : |15" // Basic fallback
				}
				// Add CRLF before pause prompt
				_, wErr = terminal.Write([]byte("\r\n")) // Still use WriteProcessedBytes for simple CRLF
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing CRLF before pause prompt: %v", nodeNumber, wErr)
				}

				// Process pipe codes first
				processedPausePrompt := string(terminal.ProcessPipeCodes([]byte(pausePrompt)))
				// +++ DEBUG LOGGING +++
				logging.Debug(" [runReadMsgs Pagination] OutputMode: %d (CP437=%d, UTF8=%d)", outputMode, terminalPkg.OutputModeCP437, terminalPkg.OutputModeUTF8)
				logging.Debug(" [runReadMsgs Pagination] Processed Pause Prompt Bytes (hex): %x", processedPausePrompt)
				// +++ END DEBUG LOGGING +++
				// Use the new helper function for writing the prompt
				wErr = writeProcessedStringWithManualEncoding(terminal, []byte(processedPausePrompt), outputMode)
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing pause prompt with manual encoding: %v", nodeNumber, wErr)
				}

				// Wait for input
				bufioReader := bufio.NewReader(s)
				pauseInputRune, _, pauseErr := bufioReader.ReadRune()
				if pauseErr != nil {
					if errors.Is(pauseErr, io.EOF) {
						log.Printf("INFO: Node %d: User disconnected during message pagination pause.", nodeNumber)
						return nil, "LOGOFF", io.EOF
					}
					log.Printf("ERROR: Node %d: Failed reading pagination pause input: %v", nodeNumber, pauseErr)
					// Decide how to handle read error - maybe quit the reader?
					break readerLoop // Exit reader on error
				}

				// Clear the pause prompt line before continuing
				// Assuming prompt is on the last line. Move up, clear line, move back down?
				// Or just rely on the next screen clear? Simpler: rely on next screen clear.
				// For now, just write a CR to go to beginning of line for next write.
				_, wErr = terminal.Write([]byte("\r"))
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing CR after pause prompt: %v", nodeNumber, wErr)
				}

				if unicode.ToUpper(pauseInputRune) == 'Q' {
					logging.Debug(" Node %d: User quit message reader during pagination.", nodeNumber)
					// --- Update Last Read only if reading new ---
					if readingNewMessages && len(messages) > 0 {
						lastViewedID := messages[currentMessageIndex].ID.String()
						logging.Debug(" Node %d: Updating last read ID (pagination quit) for user %s, area %d to %s", nodeNumber, currentUser.Handle, currentAreaID, lastViewedID)
						if currentUser.LastReadMessageIDs == nil {
							currentUser.LastReadMessageIDs = make(map[int]string)
						}
						currentUser.LastReadMessageIDs[currentAreaID] = lastViewedID
						if err := userManager.SaveUsers(); err != nil {
							log.Printf("ERROR: Node %d: Failed to save user data (pagination quit): %v", nodeNumber, err)
						}
					}
					// --- End Update ---
					break readerLoop // Exit outer reader loop
				}
				// Otherwise (Enter, Space, etc.), reset counter and continue display loop
				linesDisplayedThisPage = 0
			} else if lineIdx < len(wrappedBodyLines)-1 {
				// Add CRLF after lines that are not the last line and didn't trigger a pause
				_, wErr = terminal.Write([]byte("\r\n"))
				if wErr != nil {
					log.Printf("ERROR: Node %d: Failed writing CRLF after body line: %v", nodeNumber, wErr)
				}
			}
		} // -- End Body Display Loop --

		// Write prompt (add CRLF before)
		_, wErr = terminal.Write([]byte("\r\n"))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing CRLF before prompt: %v", nodeNumber, wErr)
		}
		_, wErr = terminal.Write([]byte(displayPrompt))
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing message prompt: %v", nodeNumber, wErr)
		}

		// 6. Input Handling (Main Reader Commands)
		input, err := terminal.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during message reading.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading message reader input: %v", nodeNumber, err)
			// Maybe break loop on error? Or just retry?
			terminal.Write([]byte("\r\n|01Error reading input.|07\r\n"))
			time.Sleep(1 * time.Second)
			continue // Retry reading input
		}

		upperInput := strings.ToUpper(strings.TrimSpace(input))

		switch upperInput {
		case "", " ": // Next message (Enter or Space defaults to Next)
			if currentMessageIndex < totalMessages-1 {
				currentMessageIndex++
			} else {
				// Optionally wrap around or stay at last message
				// For now, stay at last message and indicate
				terminal.Write([]byte("\r\n|07Last message.|07"))
				time.Sleep(500 * time.Millisecond)
			}
		case "N": // Explicit 'N' - Do nothing now, or maybe show help?
			// Currently does nothing, just loops back
			logging.Debug(" Node %d: 'N' pressed, doing nothing.", nodeNumber)
		case "P": // Previous message
			if currentMessageIndex > 0 {
				currentMessageIndex--
			} else {
				// TODO: Display "First message" indicator?
				terminal.Write([]byte("\r\n|07First message.|07"))
				time.Sleep(500 * time.Millisecond)
			}
		case "Q":
			logging.Debug(" Node %d: User quit message reader ('Q' pressed).", nodeNumber)
			// --- Update Last Read only if reading new ---
			if readingNewMessages && len(messages) > 0 {
				lastViewedID := messages[currentMessageIndex].ID.String()
				logging.Debug(" Node %d: Updating last read ID ('Q') for user %s, area %d to %s", nodeNumber, currentUser.Handle, currentAreaID, lastViewedID)
				if currentUser.LastReadMessageIDs == nil {
					currentUser.LastReadMessageIDs = make(map[int]string)
				}
				currentUser.LastReadMessageIDs[currentAreaID] = lastViewedID
				if err := userManager.SaveUsers(); err != nil {
					log.Printf("ERROR: Node %d: Failed to save user data ('Q'): %v", nodeNumber, err)
				}
			} else {
				logging.Debug(" Node %d: Skipping last read ID update because readingNewMessages is false or no messages.", nodeNumber)
			}
			// --- End Update ---
			break readerLoop // Exit the labeled loop
		case "R": // Reply
			// 1. Get Quote Prefix from config
			quotePrefix := e.LoadedStrings.QuotePrefix // Direct access
			if quotePrefix == "" {                     // Default if empty
				quotePrefix = "> "
			}

			// 2. Format quoted text
			quotedBody := formatQuote(&currentMsg, quotePrefix)

			// 3. Run Editor
			terminal.Write([]byte("\r\nLaunching editor...\r\n"))

			// Get TERM env var
			termType := "vt100" // Default TERM
			for _, env := range s.Environ() {
				if strings.HasPrefix(env, "TERM=") {
					termType = strings.TrimPrefix(env, "TERM=")
					break
				}
			}
			logging.Debug(" Node %d: Launching editor with TERM=%s", nodeNumber, termType)

			// === START: Moved Subject Logic ===
			// 4. Get Subject
			defaultSubject := generateReplySubject(currentMsg.Subject)
			subjectPromptStr := e.LoadedStrings.MsgTitleStr // Direct access
			if subjectPromptStr == "" {
				subjectPromptStr = "|08[|15Title|08] : " // Default if empty
			}
			// Process pipe codes using correct function (assuming ReplacePipeCodes)
			subjectPromptBytes := terminal.ProcessPipeCodes([]byte(subjectPromptStr)) // Use ReplacePipeCodes

			// Display the prompt (clear line first?)
			// TODO: Consider clearing a specific line if layout is fixed
			terminal.Write([]byte("\r\n")) // Add newline before prompt
			terminal.Write(subjectPromptBytes)
			terminal.Write([]byte(defaultSubject))

			// Read the input using standard ReadLine
			rawInput, err := terminal.ReadLine()
			if err != nil {
				if errors.Is(err, io.EOF) {
					log.Printf("INFO: Node %d: User disconnected during subject input.", nodeNumber)
					return nil, "LOGOFF", io.EOF // Signal logoff
				}
				log.Printf("ERROR: Node %d: Failed getting subject input: %v", nodeNumber, err)
				terminal.Write([]byte("\r\nError getting subject.\r\n"))
				time.Sleep(1 * time.Second)
				continue // Abort reply, redraw reader
			}
			newSubject := strings.TrimSpace(rawInput)
			if newSubject == "" {
				newSubject = defaultSubject
			}
			// Ensure subject is not completely empty after defaulting
			if strings.TrimSpace(newSubject) == "" {
				terminal.Write([]byte("\r\nSubject cannot be empty. Reply cancelled.\r\n"))
				time.Sleep(1 * time.Second)
				continue // Abort reply, redraw reader
			}
			// === END: Moved Subject Logic ===

			// Call editor with correct signature
			replyBody, saved, editErr := editor.RunEditor(quotedBody, s, s, termType)

			if editErr != nil {
				log.Printf("ERROR: Node %d: Editor failed: %v", nodeNumber, editErr)
				terminal.Write([]byte("\r\nEditor encountered an error.\r\n"))
				time.Sleep(2 * time.Second)
				// Screen needs redraw - continue will force it
				continue
			}

			if !saved {
				terminal.Write([]byte("\r\nReply cancelled.\r\n"))
				time.Sleep(1 * time.Second)
				// Screen needs redraw - continue will force it
				continue
			}

			// 5. Construct Message (Subject 'newSubject' already obtained)
			replyMsg := message.Message{
				ID:           uuid.New(), // Use uuid.New()
				AreaID:       currentAreaID,
				FromUserName: currentUser.Handle,
				ToUserName:   currentMsg.FromUserName, // Reply goes back to original sender
				Subject:      newSubject,              // Use subject from before editor
				Body:         replyBody,
				PostedAt:     time.Now(),    // Use PostedAt field
				ReplyToID:    currentMsg.ID, // Link to the original message
			}

			// 6. Save Message
			err = e.MessageMgr.AddMessage(currentAreaID, replyMsg) // Pass value, not pointer
			if err != nil {
				log.Printf("ERROR: Node %d: Failed to save reply message: %v", nodeNumber, err)
				terminal.Write([]byte("\r\nError saving reply message.\r\n"))
				time.Sleep(2 * time.Second)
			} else {
				terminal.Write([]byte("\r\nReply posted successfully!\r\n"))
				time.Sleep(1 * time.Second)
				// --- Update Counters and Slice ---
				totalMessages++         // Increment count of messages loaded in the *current slice*
				totalMessageCount++     // Increment the *overall* area count
				if readingNewMessages { // <<< ADDED Check
					newCount++ // <<< ADDED: Increment newCount if we started by reading new
				}
				messages = append(messages, replyMsg) // Add reply to the *current slice*
				// --- End Update ---

				// -->> Advance to the next message after replying <<--
				if currentMessageIndex < totalMessages-1 { // Check bounds just in case, though append should make it safe
					currentMessageIndex++
				}
			}
			continue

		default:
			// Optional: Display invalid command message
			logging.Debug(" Node %d: Invalid command '%s' in message reader.", nodeNumber, upperInput)
			terminal.Write([]byte("\r\n|01Invalid command.|07"))
			time.Sleep(500 * time.Millisecond)
		}
		// continue loop implicitly unless 'Q' was pressed
	}

	// 7. Return (after loop breaks)
	logging.Debug(" Node %d: Exiting message reader function.", nodeNumber)
	return nil, "", nil // Return to MSGMENU
}

// runNewscan checks all accessible message areas for new messages.
func runNewscan(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
	logging.Debug(" Node %d: Running NEWSCAN for user %s", nodeNumber, currentUser.Handle)

	if currentUser == nil {
		log.Printf("WARN: Node %d: NEWSCAN called without logged in user.", nodeNumber)
		msg := "\r\n|01Error: You must be logged in to scan for new messages.|07\r\n"
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil { /* Log? */
		}
		time.Sleep(1 * time.Second)
		return nil, "", nil // Return to menu
	}

	allAreas := e.MessageMgr.ListAreas()
	// Store results as a slice of structs to maintain order and store counts
	type areaResult struct {
		Tag   string
		Count int
	}
	var resultsWithNew []areaResult // Changed variable name

	logging.Debug(" Node %d: Scanning %d total areas for new messages.", nodeNumber, len(allAreas))

	for _, area := range allAreas {
		if !checkACS(area.ACSRead, currentUser, s, terminal, sessionStartTime) {
			log.Printf("TRACE: Node %d: Skipping area %d ('%s') due to read ACS '%s'", nodeNumber, area.ID, area.Tag, area.ACSRead)
			continue
		}

		lastReadID := ""
		if currentUser.LastReadMessageIDs != nil {
			lastReadID = currentUser.LastReadMessageIDs[area.ID]
		}

		// Get the count of new messages (Corrected function call)
		newCount, err := e.MessageMgr.GetNewMessageCount(area.ID, lastReadID)
		if err != nil {
			log.Printf("ERROR: Node %d: Error checking new message count in area %d ('%s'): %v", nodeNumber, area.ID, area.Tag, err)
			continue // Skip area on error
		}

		if newCount > 0 {
			logging.Debug(" Node %d: Found %d new messages in area %d ('%s')", nodeNumber, newCount, area.ID, area.Tag)
			resultsWithNew = append(resultsWithNew, areaResult{Tag: area.Tag, Count: newCount}) // Correctly append
		}
	}

	// 5. Display Results & Handle Actions
	var outputBuffer bytes.Buffer
	outputBuffer.WriteString("\r\n") // Start with a newline

	if len(resultsWithNew) == 0 { // Use corrected variable name
		// No new messages found
		outputBuffer.WriteString("|07No new messages found in any accessible areas.\r\n")
		log.Printf("INFO: Node %d: Newscan completed, no new messages found.", nodeNumber)

		// Write the result message
		wErr := terminal.DisplayContent(outputBuffer.Bytes())
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing NEWSCAN (no new) results: %v", nodeNumber, wErr)
		}
		// Fall through to standard pause prompt

	} else {
		// New messages found - build summary and ask to read
		var summaryParts []string
		for _, res := range resultsWithNew { // Use corrected variable name
			summaryParts = append(summaryParts, fmt.Sprintf("%s(%d)", res.Tag, res.Count))
		}
		summary := strings.Join(summaryParts, ", ")
		outputBuffer.WriteString(fmt.Sprintf("|07New messages: |15%s|07\r\n", summary))
		log.Printf("INFO: Node %d: Newscan completed, new messages found: %s", nodeNumber, summary)

		// Write the summary message first
		wErr := terminal.DisplayContent(outputBuffer.Bytes())
		if wErr != nil {
			log.Printf("ERROR: Node %d: Failed writing NEWSCAN summary results: %v", nodeNumber, wErr)
			// Fall through to pause anyway?
		}

		// Ask the user if they want to read
		// TODO: Use configurable string
		readNow, err := e.promptYesNoLightbar(s, terminal, "Read new messages now?", outputMode, nodeNumber)
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during NEWSCAN read prompt.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Failed getting Yes/No input for NEWSCAN read: %v", err)
			// Don't pause if prompt failed, just return to menu
			return nil, "", err
		}

		if readNow {
			// User wants to read - jump to the first area with new messages
			firstAreaTag := resultsWithNew[0].Tag // Use corrected variable name
			logging.Debug(" Node %d: User chose to read. Jumping to first area: %s", nodeNumber, firstAreaTag)

			// Get the Area ID for the tag
			firstArea, found := e.MessageMgr.GetAreaByTag(firstAreaTag)
			if !found {
				// Should not happen if ListAreas and GetNewMessageCount worked
				log.Printf("ERROR: Node %d: Could not find area details for tag '%s' after newscan.", nodeNumber, firstAreaTag)
				// Fall through to pause prompt as a fallback
			} else {
				// Update user's current area
				currentUser.CurrentMessageAreaID = firstArea.ID
				currentUser.CurrentMessageAreaTag = firstArea.Tag
				log.Printf("INFO: Node %d: Set current message area for user %s to %d (%s)", nodeNumber, currentUser.Handle, firstArea.ID, firstArea.Tag)

				// Save the user state *before* calling runReadMsgs
				if err := userManager.SaveUsers(); err != nil {
					log.Printf("ERROR: Node %d: Failed to save user data after setting current area in newscan: %v", nodeNumber, err)
					// Proceed anyway, but state might be lost
				}

				// Directly call runReadMsgs. It will handle its own loop and return.
				return runReadMsgs(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, args, outputMode)
			}
		} else {
			// User chose not to read now - fall through to standard pause
			logging.Debug(" Node %d: User chose not to read new messages now.", nodeNumber)
		}
	}

	// 6. Wait for Enter (Standard Pause - only reached if no new messages or user chose 'No')
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
		log.Printf("ERROR: Node %d: Failed writing NEWSCAN pause prompt: %v", nodeNumber, wErr)
	}

	bufioReader := bufio.NewReader(s)
	for {
		r, _, err := bufioReader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Node %d: User disconnected during NEWSCAN pause.", nodeNumber)
				return nil, "LOGOFF", io.EOF
			}
			log.Printf("ERROR: Node %d: Failed reading input during NEWSCAN pause: %v", nodeNumber, err)
			return nil, "", err
		}
		if r == '\r' || r == '\n' { // Check for CR or LF
			break
		}
	}

	return nil, "", nil // Return to the current menu (MSGMENU)
}

// generateReplySubject creates a suitable subject line for a reply.
// It prepends "Re: " unless the original subject already starts with it (case-insensitive).
func generateReplySubject(originalSubject string) string {
	upperSubject := strings.ToUpper(strings.TrimSpace(originalSubject))
	if strings.HasPrefix(upperSubject, "RE:") {
		return originalSubject // Already a reply
	}
	return "Re: " + originalSubject
}

// formatQuote formats the body of an original message for quoting in a reply.
// It prepends each line with the specified quotePrefix.
func formatQuote(originalMsg *message.Message, quotePrefix string) string {
	if originalMsg == nil || originalMsg.Body == "" {
		return ""
	}

	var builder strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(originalMsg.Body))
	for scanner.Scan() {
		builder.WriteString(quotePrefix)
		builder.WriteString(scanner.Text())
		builder.WriteString("\n") // Use ACTUAL newline for editor buffer
	}
	// Check for scanner errors, although unlikely with strings.Reader
	if err := scanner.Err(); err != nil {
		log.Printf("WARN: Error scanning original message body for quoting: %v", err)
		// Return whatever was built so far, or perhaps an error indicator?
		// For now, just return the potentially partial quote.
	}
	return builder.String()
}

// wrapAnsiString wraps a string containing ANSI codes to a given width.
// NOTE: This is a simplified version and does NOT perfectly handle ANSI state across wrapped lines.
// It primarily prevents lines from exceeding the terminal width visually.
func wrapAnsiString(text string, width int) []string {
	if width <= 0 {
		return strings.Split(text, "\n") // No wrapping if width is invalid
	}

	var wrappedLines []string
	// Split input into lines first based on existing newlines
	inputLines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	reAnsi := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`) // Basic regex for ANSI codes

	for _, line := range inputLines {
		currentLine := ""
		currentWidth := 0
		words := strings.Fields(line) // Split line into words

		for _, word := range words {
			// Calculate visible width of the word (stripping ANSI)
			plainWord := reAnsi.ReplaceAllString(word, "")
			wordWidth := len(plainWord)

			if currentWidth == 0 {
				// First word on the line
				if wordWidth > width {
					// Word is longer than the line width, just append it (will overflow)
					wrappedLines = append(wrappedLines, word)
					currentLine = ""
					currentWidth = 0
				} else {
					currentLine = word
					currentWidth = wordWidth
				}
			} else {
				// Subsequent words
				if currentWidth+1+wordWidth <= width {
					// Word fits on the current line
					currentLine += " " + word
					currentWidth += 1 + wordWidth
				} else {
					// Word doesn't fit, wrap to next line
					wrappedLines = append(wrappedLines, currentLine)
					if wordWidth > width {
						// Word itself is too long, put it on its own line
						wrappedLines = append(wrappedLines, word)
						currentLine = ""
						currentWidth = 0
					} else {
						// Start new line with the current word
						currentLine = word
						currentWidth = wordWidth
					}
				}
			}
		}
		// Add the last line being built
		if currentWidth > 0 {
			wrappedLines = append(wrappedLines, currentLine)
		}
	}

	return wrappedLines
}

// writeProcessedStringWithManualEncoding takes bytes that have already had pipe codes
// replaced with standard ANSI escapes and writes them to the terminal, handling
// character encoding manually based on the desired outputMode.
// It now correctly handles UTF-8 input strings containing ANSI codes.
func writeProcessedStringWithManualEncoding(terminal *terminalPkg.BBS, processedBytes []byte, outputMode terminalPkg.OutputMode) error {
	var finalBuf bytes.Buffer
	i := 0
	processedString := string(processedBytes) // Work with the UTF-8 string

	for i < len(processedString) {
		// Check for ANSI escape sequence start
		if processedString[i] == '\x1b' { // <-- Corrected: Use character literal
			start := i
			// Find the end of the ANSI sequence (basic CSI parsing)
			if i+1 < len(processedString) && processedString[i+1] == '[' {
				i += 2 // Skip ESC [
				for i < len(processedString) {
					c := processedString[i]
					if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') { // Found terminator
						i++
						break
					}
					i++
					// Basic protection
					if i-start > 30 {
						log.Printf("WARN: [writeProcessedString] Potential runaway ANSI sequence encountered.")
						break
					}
				}
			} else {
				// Handle other potential escape sequences if necessary (e.g., ESC ( B )
				// For now, assume simple non-CSI escapes are short or handle known ones
				// Example: ESC ( B (designate US-ASCII) is 3 bytes
				if i+2 < len(processedString) && processedString[i+1] == '(' && processedString[i+2] == 'B' {
					i += 3
				} else {
					i++ // Just skip the ESC if unknown sequence
				}
			}
			// Write the entire ANSI sequence as is
			finalBuf.WriteString(processedString[start:i])
			continue // Continue outer loop
		}

		// Decode the next rune from the UTF-8 string
		r, size := utf8.DecodeRuneInString(processedString[i:])
		if r == utf8.RuneError && size <= 1 {
			// Invalid UTF-8 sequence, write a placeholder or skip
			finalBuf.WriteByte('?')
			i++ // Move past the invalid byte
			continue
		}

		// Now handle the valid rune 'r' based on outputMode
		if outputMode == terminalPkg.OutputModeCP437 {
			if r < 128 {
				// ASCII character, write directly
				finalBuf.WriteByte(byte(r))
			} else if cp437Byte, ok := terminalPkg.UnicodeToCP437Table[r]; ok {
				// Found a corresponding CP437 byte
				finalBuf.WriteByte(cp437Byte)
			} else {
				// Unicode character doesn't exist in CP437, write fallback
				finalBuf.WriteByte('?')
			}
		} else { // OutputModeUTF8 or OutputModeAuto (assuming UTF-8 if not CP437)
			// Write the original rune (which is already UTF-8)
			finalBuf.WriteRune(r)
		}

		i += size // Move past the processed rune
	}

	// Write the fully processed buffer to the terminal
	_, err := terminal.Write(finalBuf.Bytes())
	return err
}
