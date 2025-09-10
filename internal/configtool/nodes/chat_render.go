package nodes

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Chat system rendering methods

// renderTitleBar renders the chat title bar
func (cs *ChatSystem) renderTitleBar() string {
	var title string
	
	switch cs.chatMode {
	case ChatModeSelect:
		title = fmt.Sprintf("Inter-Node Chat - %s@Node%d - Select User", cs.currentUser, cs.currentNodeID)
	case ChatModePrivate:
		if cs.chatPartner > 0 {
			partnerName := cs.getPartnerName(cs.chatPartner)
			title = fmt.Sprintf("Private Chat - %s@Node%d ↔ %s@Node%d", 
				cs.currentUser, cs.currentNodeID, partnerName, cs.chatPartner)
		} else {
			title = fmt.Sprintf("Private Chat - %s@Node%d", cs.currentUser, cs.currentNodeID)
		}
	case ChatModeChannel:
		title = fmt.Sprintf("Channel Chat - %s@Node%d - #%s", cs.currentUser, cs.currentNodeID, cs.currentChannel)
	case ChatModePage:
		title = fmt.Sprintf("Page System - %s@Node%d", cs.currentUser, cs.currentNodeID)
	case ChatModeAway:
		title = fmt.Sprintf("Away Mode - %s@Node%d", cs.currentUser, cs.currentNodeID)
	}
	
	titleStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("5")).     // Magenta background
		Foreground(lipgloss.Color("15")).    // White text
		Bold(true).
		Padding(0, 1).
		Width(cs.width)
	
	return titleStyle.Render(title)
}

// renderModeTabs renders the chat mode tabs
func (cs *ChatSystem) renderModeTabs() string {
	modes := []struct {
		key  string
		name string
		mode ChatMode
	}{
		{"F1", "Select", ChatModeSelect},
		{"F2", "Channel", ChatModeChannel},
		{"F3", "Page", ChatModePage},
		{"F4", "Away", ChatModeAway},
	}
	
	var tabs []string
	for _, m := range modes {
		tabStyle := lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.RoundedBorder(), false, true, false, false)
		
		if m.mode == cs.chatMode {
			tabStyle = tabStyle.Background(lipgloss.Color("5")).Foreground(lipgloss.Color("15"))
		} else {
			tabStyle = tabStyle.Background(lipgloss.Color("7")).Foreground(lipgloss.Color("0"))
		}
		
		tabText := fmt.Sprintf("%s:%s", m.key, m.name)
		tabs = append(tabs, tabStyle.Render(tabText))
	}
	
	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// renderSplitView renders the split view with chat and user list
func (cs *ChatSystem) renderSplitView() string {
	chatWidth := (cs.width * 2) / 3
	userWidth := cs.width - chatWidth - 2
	
	chatView := cs.renderChatContent(chatWidth)
	userList := cs.renderUserList(userWidth)
	
	return lipgloss.JoinHorizontal(lipgloss.Top, chatView, userList)
}

// renderChatView renders the full chat view
func (cs *ChatSystem) renderChatView() string {
	return cs.renderChatContent(cs.width - 4)
}

// renderChatContent renders the chat content area
func (cs *ChatSystem) renderChatContent(width int) string {
	history := cs.getCurrentChatHistory()
	
	var lines []string
	
	// Calculate visible range
	contentHeight := cs.height - 12 // Account for title, tabs, input, help
	if cs.inputMode {
		contentHeight -= 3
	}
	if len(cs.notifications) > 0 {
		contentHeight -= 4
	}
	
	startIdx := cs.scrollOffset
	endIdx := startIdx + contentHeight
	
	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > len(history) {
		endIdx = len(history)
	}
	
	// Show chat messages
	if len(history) == 0 {
		lines = append(lines, "No messages yet...")
		if cs.chatMode == ChatModePrivate {
			lines = append(lines, "Press 'T' or Enter to start typing")
		} else if cs.chatMode == ChatModeChannel {
			lines = append(lines, fmt.Sprintf("Welcome to #%s channel", cs.currentChannel))
			lines = append(lines, "Press 'T' or Enter to start typing")
		}
	} else {
		for i := startIdx; i < endIdx; i++ {
			msg := history[i]
			line := cs.formatChatMessage(msg, width-4)
			lines = append(lines, line)
		}
	}
	
	// Add scroll indicator
	if len(history) > contentHeight {
		scrollInfo := fmt.Sprintf("Showing %d-%d of %d messages", 
			startIdx+1, endIdx, len(history))
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Render(scrollInfo))
	}
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(width).
		Height(contentHeight + 2)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderUserList renders the user list
func (cs *ChatSystem) renderUserList(width int) string {
	var lines []string
	
	if cs.chatMode == ChatModeSelect {
		lines = append(lines, "Available Users:")
	} else if cs.chatMode == ChatModePage {
		lines = append(lines, "Users to Page:")
	}
	lines = append(lines, "")
	
	if len(cs.availableUsers) == 0 {
		lines = append(lines, "No other users online")
	} else {
		for i, user := range cs.availableUsers {
			var statusIcon, statusColor string
			
			switch user.Status {
			case "available":
				statusIcon = "●"
				statusColor = "2" // Green
			case "busy":
				statusIcon = "●"
				statusColor = "3" // Yellow
			case "away":
				statusIcon = "●"
				statusColor = "8" // Gray
			default:
				statusIcon = "○"
				statusColor = "1" // Red
			}
			
			userLine := fmt.Sprintf("%s %s", statusIcon, user.Handle)
			if user.Location != "" {
				userLine += fmt.Sprintf(" (%s)", user.Location)
			}
			
			if user.InPrivateChat {
				userLine += " [Chat]"
			}
			
			userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor))
			
			// Highlight selected user
			if i == cs.selectedUser {
				userStyle = userStyle.Background(lipgloss.Color("4")).Bold(true)
			}
			
			// Truncate if too long
			if len(userLine) > width-4 {
				userLine = userLine[:width-7] + "..."
			}
			
			lines = append(lines, userStyle.Render(userLine))
		}
	}
	
	lines = append(lines, "")
	
	// Add mode-specific help
	if cs.chatMode == ChatModeSelect {
		lines = append(lines, "Enter: Chat")
		lines = append(lines, "P: Page")
		lines = append(lines, "I: Invite")
	} else if cs.chatMode == ChatModePage {
		lines = append(lines, "Enter: Page User")
		lines = append(lines, "B: Broadcast Page")
	}
	
	listStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(width)
	
	return listStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderInputArea renders the message input area
func (cs *ChatSystem) renderInputArea() string {
	var prompt string
	
	switch cs.chatMode {
	case ChatModePrivate:
		if cs.chatPartner > 0 {
			partnerName := cs.getPartnerName(cs.chatPartner)
			prompt = fmt.Sprintf("To %s: ", partnerName)
		} else {
			prompt = "Message: "
		}
	case ChatModeChannel:
		prompt = fmt.Sprintf("#%s: ", cs.currentChannel)
	case ChatModePage:
		if cs.chatPartner > 0 {
			partnerName := cs.getPartnerName(cs.chatPartner)
			prompt = fmt.Sprintf("Page %s: ", partnerName)
		} else {
			prompt = "Broadcast Page: "
		}
	case ChatModeAway:
		prompt = "Away: "
	default:
		prompt = "Message: "
	}
	
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("5")).
		Padding(0, 1).
		Width(cs.width - 4)
	
	inputLine := prompt + cs.messageInput + "█" // Show cursor
	return inputStyle.Render(inputLine)
}

// renderNotifications renders pending notifications
func (cs *ChatSystem) renderNotifications() string {
	var lines []string
	lines = append(lines, "Notifications:")
	
	// Show last 3 notifications
	maxNotifs := 3
	startIdx := len(cs.notifications) - maxNotifs
	if startIdx < 0 {
		startIdx = 0
	}
	
	for i := startIdx; i < len(cs.notifications); i++ {
		notif := cs.notifications[i]
		var icon, style string
		
		switch notif.Type {
		case "page":
			icon = "📟"
			style = "1" // Red for urgent pages
		case "chat_request":
			icon = "💬"
			style = "3" // Yellow for chat requests
		case "system":
			icon = "ℹ"
			style = "4" // Blue for system messages
		default:
			icon = "📢"
			style = "7"
		}
		
		timestamp := notif.Timestamp.Format("15:04")
		notifLine := fmt.Sprintf("%s [%s] %s: %s", 
			icon, timestamp, notif.FromUser, notif.Message)
		
		notifStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(style))
		if notif.Urgent {
			notifStyle = notifStyle.Bold(true).Blink(true)
		}
		
		lines = append(lines, notifStyle.Render(notifLine))
	}
	
	notifStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("3")).
		Padding(0, 1).
		Width(cs.width - 4)
	
	return notifStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderHelpLine renders the help line
func (cs *ChatSystem) renderHelpLine() string {
	var help string
	
	if cs.inputMode {
		help = "Enter:Send ESC:Cancel"
	} else {
		switch cs.chatMode {
		case ChatModeSelect:
			help = "↑/↓:Select Enter:Chat P:Page I:Invite TAB:Toggle List F1-F4:Modes"
		case ChatModePrivate:
			help = "T/Enter:Type A:Action E:End ↑/↓:Scroll C:Clear ESC:Back"
		case ChatModeChannel:
			help = "T/Enter:Type 1-3:Channels L:List ↑/↓:Scroll C:Clear"
		case ChatModePage:
			help = "↑/↓:Select Enter:Page B:Broadcast ESC:Back"
		case ChatModeAway:
			help = "S:Set Away R:Return M:Message ESC:Back"
		}
		help += " R:Refresh Q:Quit"
	}
	
	helpStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("0")).
		Foreground(lipgloss.Color("7")).
		Width(cs.width)
	
	return helpStyle.Render(help)
}

// formatChatMessage formats a chat message for display
func (cs *ChatSystem) formatChatMessage(msg ChatMessage, maxWidth int) string {
	timestamp := msg.Timestamp.Format("15:04:05")
	var prefix, content string
	var style lipgloss.Style
	
	switch msg.MessageType {
	case "system":
		prefix = "*** "
		content = msg.Message
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
		
	case "chat":
		if msg.IsAction {
			prefix = fmt.Sprintf("* %s ", msg.FromUser)
			content = msg.Message
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Italic(true)
		} else {
			prefix = fmt.Sprintf("<%s> ", msg.FromUser)
			content = msg.Message
			
			// Different colors for different users
			if msg.FromUser == cs.currentUser {
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // Cyan for self
			} else {
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("15")) // White for others
			}
		}
		
	case "page":
		prefix = fmt.Sprintf("📟 %s pages: ", msg.FromUser)
		content = msg.Message
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
		
	case "chat_request":
		prefix = "💬 "
		content = msg.Message
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
		
	default:
		prefix = fmt.Sprintf("[%s] ", msg.FromUser)
		content = msg.Message
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	}
	
	// Wrap long messages
	fullMessage := prefix + content
	if len(fullMessage) > maxWidth-12 { // Account for timestamp
		// Word wrap
		words := strings.Fields(content)
		var lines []string
		currentLine := prefix
		
		for _, word := range words {
			if len(currentLine)+len(word)+1 > maxWidth-12 {
				lines = append(lines, currentLine)
				currentLine = strings.Repeat(" ", len(prefix)) + word
			} else {
				if currentLine != prefix {
					currentLine += " "
				}
				currentLine += word
			}
		}
		
		if currentLine != prefix {
			lines = append(lines, currentLine)
		}
		
		// Format all lines with timestamp on first line
		var formattedLines []string
		for i, line := range lines {
			if i == 0 {
				formattedLines = append(formattedLines, 
					fmt.Sprintf("[%s] %s", timestamp, style.Render(line)))
			} else {
				formattedLines = append(formattedLines, 
					fmt.Sprintf("         %s", style.Render(line)))
			}
		}
		
		return strings.Join(formattedLines, "\n")
	}
	
	// Single line message
	return fmt.Sprintf("[%s] %s", timestamp, style.Render(fullMessage))
}

// getPartnerName gets the name of the chat partner
func (cs *ChatSystem) getPartnerName(nodeID int) string {
	for _, user := range cs.availableUsers {
		if user.NodeID == nodeID {
			return user.Handle
		}
	}
	
	// Try to get from node manager
	node, err := cs.nodeManager.GetNode(nodeID)
	if err == nil && node.User != nil {
		return node.User.Handle
	}
	
	return fmt.Sprintf("Node%d", nodeID)
}

// Additional chat commands and features

// handleChatCommand processes special chat commands
func (cs *ChatSystem) handleChatCommand(message string) bool {
	if !strings.HasPrefix(message, "/") {
		return false
	}
	
	parts := strings.Fields(message)
	if len(parts) == 0 {
		return false
	}
	
	command := strings.ToLower(parts[0])
	
	switch command {
	case "/me":
		// Action message
		if len(parts) > 1 {
			action := strings.Join(parts[1:], " ")
			cs.sendActionMessage(action)
		}
		return true
		
	case "/who":
		// List online users
		cs.showOnlineUsers()
		return true
		
	case "/time":
		// Show current time
		cs.showSystemTime()
		return true
		
	case "/help":
		// Show chat help
		cs.showChatHelp()
		return true
		
	case "/clear":
		// Clear chat history
		cs.clearCurrentChat()
		return true
		
	case "/quit", "/exit":
		// Exit chat mode
		cs.chatMode = ChatModeSelect
		return true
		
	default:
		// Unknown command
		cs.addSystemMessage(fmt.Sprintf("Unknown command: %s", command))
		return true
	}
}

// sendActionMessage sends an action message
func (cs *ChatSystem) sendActionMessage(action string) {
	chatMsg := ChatMessage{
		FromUser:    cs.currentUser,
		FromNode:    cs.currentNodeID,
		Message:     action,
		Timestamp:   time.Now(),
		MessageType: "chat",
		IsAction:    true,
	}
	
	switch cs.chatMode {
	case ChatModePrivate:
		if cs.chatPartner > 0 {
			chatMsg.ToNode = cs.chatPartner
			chatMsg.IsPrivate = true
			cs.privateChats[cs.chatPartner] = append(cs.privateChats[cs.chatPartner], chatMsg)
			
			// Send to other node
			nodeMsg := NodeMessage{
				FromNode:    cs.currentNodeID,
				FromUser:    cs.currentUser,
				ToNode:      cs.chatPartner,
				Message:     action,
				MessageType: "action",
				Priority:    2,
				Timestamp:   time.Now(),
			}
			cs.nodeManager.SendMessage(nodeMsg)
		}
		
	case ChatModeChannel:
		chatMsg.Channel = cs.currentChannel
		chatMsg.IsPrivate = false
		cs.channels[cs.currentChannel] = append(cs.channels[cs.currentChannel], chatMsg)
		
		// Broadcast action
		cs.nodeManager.BroadcastMessage(
			fmt.Sprintf("[%s] * %s %s", cs.currentChannel, cs.currentUser, action),
			cs.currentUser)
	}
}

// showOnlineUsers shows list of online users
func (cs *ChatSystem) showOnlineUsers() {
	var userList []string
	for _, user := range cs.availableUsers {
		status := user.Status
		if user.InPrivateChat {
			status += " (chatting)"
		}
		userList = append(userList, fmt.Sprintf("%s@Node%d (%s)", 
			user.Handle, user.NodeID, status))
	}
	
	message := fmt.Sprintf("Online users: %s", strings.Join(userList, ", "))
	if len(userList) == 0 {
		message = "No other users online"
	}
	
	cs.addSystemMessage(message)
}

// showSystemTime shows current system time
func (cs *ChatSystem) showSystemTime() {
	message := fmt.Sprintf("Current time: %s", time.Now().Format("15:04:05 MST"))
	cs.addSystemMessage(message)
}

// showChatHelp shows chat help
func (cs *ChatSystem) showChatHelp() {
	helpMessages := []string{
		"Chat Commands:",
		"/me <action> - Send action message",
		"/who - List online users",
		"/time - Show current time",
		"/clear - Clear chat history",
		"/help - Show this help",
		"/quit - Exit chat mode",
	}
	
	for _, msg := range helpMessages {
		cs.addSystemMessage(msg)
	}
}

// addSystemMessage adds a system message to current chat
func (cs *ChatSystem) addSystemMessage(message string) {
	chatMsg := ChatMessage{
		FromUser:    "System",
		FromNode:    0,
		Message:     message,
		Timestamp:   time.Now(),
		MessageType: "system",
	}
	
	switch cs.chatMode {
	case ChatModePrivate:
		if cs.chatPartner > 0 {
			cs.privateChats[cs.chatPartner] = append(cs.privateChats[cs.chatPartner], chatMsg)
		}
	case ChatModeChannel:
		cs.channels[cs.currentChannel] = append(cs.channels[cs.currentChannel], chatMsg)
	default:
		cs.chatHistory = append(cs.chatHistory, chatMsg)
	}
}