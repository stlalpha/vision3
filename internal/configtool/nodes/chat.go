package nodes

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ChatSystem represents the inter-node chat system
type ChatSystem struct {
	nodeManager     NodeManager
	width           int
	height          int
	focused         bool
	currentUser     string
	currentNodeID   int
	chatPartner     int
	chatMode        ChatMode
	messageInput    string
	chatHistory     []ChatMessage
	availableUsers  []ChatUser
	selectedUser    int
	scrollOffset    int
	maxHistory      int
	notifications   []ChatNotification
	privateChats    map[int][]ChatMessage
	channels        map[string][]ChatMessage
	currentChannel  string
	showUserList    bool
	inputMode       bool
}

// ChatMode represents different chat modes
type ChatMode int

const (
	ChatModeSelect ChatMode = iota
	ChatModePrivate
	ChatModeChannel
	ChatModePage
	ChatModeAway
)

// ChatMessage represents a chat message
type ChatMessage struct {
	FromUser     string    `json:"from_user"`
	FromNode     int       `json:"from_node"`
	ToUser       string    `json:"to_user,omitempty"`
	ToNode       int       `json:"to_node,omitempty"`
	Channel      string    `json:"channel,omitempty"`
	Message      string    `json:"message"`
	Timestamp    time.Time `json:"timestamp"`
	MessageType  string    `json:"message_type"` // "chat", "action", "system", "page"
	IsPrivate    bool      `json:"is_private"`
	IsAction     bool      `json:"is_action"`
	Priority     int       `json:"priority"`
}

// ChatUser represents a user available for chat
type ChatUser struct {
	Handle       string    `json:"handle"`
	NodeID       int       `json:"node_id"`
	Location     string    `json:"location"`
	Activity     string    `json:"activity"`
	Status       string    `json:"status"` // "available", "busy", "away", "dnd"
	LastActivity time.Time `json:"last_activity"`
	InPrivateChat bool     `json:"in_private_chat"`
}

// ChatNotification represents a chat notification
type ChatNotification struct {
	FromUser  string    `json:"from_user"`
	FromNode  int       `json:"from_node"`
	Message   string    `json:"message"`
	Type      string    `json:"type"` // "page", "chat_request", "chat_invite", "system"
	Timestamp time.Time `json:"timestamp"`
	Urgent    bool      `json:"urgent"`
}

// NewChatSystem creates a new chat system interface
func NewChatSystem(nodeManager NodeManager, width, height int, currentUser string, nodeID int) *ChatSystem {
	return &ChatSystem{
		nodeManager:   nodeManager,
		width:         width,
		height:        height,
		currentUser:   currentUser,
		currentNodeID: nodeID,
		chatMode:      ChatModeSelect,
		chatHistory:   make([]ChatMessage, 0),
		availableUsers: make([]ChatUser, 0),
		maxHistory:    200,
		notifications: make([]ChatNotification, 0),
		privateChats:  make(map[int][]ChatMessage),
		channels:      make(map[string][]ChatMessage),
		currentChannel: "General",
		showUserList:  true,
	}
}

// Update implements tea.Model
func (cs *ChatSystem) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return cs.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		cs.width = msg.Width
		cs.height = msg.Height
	case TickMsg:
		cs.refreshUserList()
		cs.checkForNewMessages()
		return cs, cs.tick()
	}
	return cs, nil
}

// tick returns a command for periodic updates
func (cs *ChatSystem) tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// handleKeyPress processes keyboard input
func (cs *ChatSystem) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle input mode
	if cs.inputMode {
		return cs.handleInputMode(msg)
	}

	// Global commands
	switch msg.String() {
	case "q", "esc":
		if cs.chatMode == ChatModePrivate || cs.chatMode == ChatModeChannel {
			cs.chatMode = ChatModeSelect
			cs.chatPartner = 0
			return cs, nil
		}
		return cs, tea.Quit
	case "tab":
		cs.showUserList = !cs.showUserList
	case "f1":
		cs.chatMode = ChatModeSelect
	case "f2":
		cs.chatMode = ChatModeChannel
		cs.currentChannel = "General"
	case "f3":
		cs.chatMode = ChatModePage
	case "f4":
		cs.chatMode = ChatModeAway
	case "r":
		cs.refreshUserList()
	case "c":
		cs.clearCurrentChat()
	}

	// Mode-specific commands
	switch cs.chatMode {
	case ChatModeSelect:
		return cs.handleSelectModeKeys(msg)
	case ChatModePrivate:
		return cs.handlePrivateModeKeys(msg)
	case ChatModeChannel:
		return cs.handleChannelModeKeys(msg)
	case ChatModePage:
		return cs.handlePageModeKeys(msg)
	case ChatModeAway:
		return cs.handleAwayModeKeys(msg)
	}

	return cs, nil
}

// handleInputMode handles input mode keys
func (cs *ChatSystem) handleInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return cs.sendCurrentMessage()
	case "esc":
		cs.inputMode = false
		cs.messageInput = ""
	case "backspace":
		if len(cs.messageInput) > 0 {
			cs.messageInput = cs.messageInput[:len(cs.messageInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			cs.messageInput += msg.String()
		}
	}
	return cs, nil
}

// handleSelectModeKeys handles user selection mode
func (cs *ChatSystem) handleSelectModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if cs.selectedUser > 0 {
			cs.selectedUser--
		}
	case "down", "j":
		if cs.selectedUser < len(cs.availableUsers)-1 {
			cs.selectedUser++
		}
	case "enter":
		if cs.selectedUser < len(cs.availableUsers) {
			user := cs.availableUsers[cs.selectedUser]
			if user.NodeID != cs.currentNodeID {
				cs.startPrivateChat(user.NodeID)
			}
		}
	case "p":
		if cs.selectedUser < len(cs.availableUsers) {
			user := cs.availableUsers[cs.selectedUser]
			cs.startPageRequest(user.NodeID)
		}
	case "i":
		cs.startChatInvite()
	}
	return cs, nil
}

// handlePrivateModeKeys handles private chat mode
func (cs *ChatSystem) handlePrivateModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if cs.scrollOffset > 0 {
			cs.scrollOffset--
		}
	case "down", "j":
		maxScroll := len(cs.getCurrentChatHistory()) - (cs.height - 10)
		if cs.scrollOffset < maxScroll {
			cs.scrollOffset++
		}
	case "pageup":
		cs.scrollOffset -= 10
		if cs.scrollOffset < 0 {
			cs.scrollOffset = 0
		}
	case "pagedown":
		cs.scrollOffset += 10
	case "t", "enter":
		cs.inputMode = true
		cs.messageInput = ""
	case "a":
		cs.sendAction()
	case "e":
		cs.endPrivateChat()
	}
	return cs, nil
}

// handleChannelModeKeys handles channel chat mode
func (cs *ChatSystem) handleChannelModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if cs.scrollOffset > 0 {
			cs.scrollOffset--
		}
	case "down", "j":
		maxScroll := len(cs.getCurrentChatHistory()) - (cs.height - 10)
		if cs.scrollOffset < maxScroll {
			cs.scrollOffset++
		}
	case "t", "enter":
		cs.inputMode = true
		cs.messageInput = ""
	case "1":
		cs.currentChannel = "General"
		cs.scrollOffset = 0
	case "2":
		cs.currentChannel = "SysOp"
		cs.scrollOffset = 0
	case "3":
		cs.currentChannel = "Games"
		cs.scrollOffset = 0
	case "l":
		cs.listChannels()
	}
	return cs, nil
}

// handlePageModeKeys handles page mode
func (cs *ChatSystem) handlePageModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if cs.selectedUser > 0 {
			cs.selectedUser--
		}
	case "down", "j":
		if cs.selectedUser < len(cs.availableUsers)-1 {
			cs.selectedUser++
		}
	case "enter":
		if cs.selectedUser < len(cs.availableUsers) {
			user := cs.availableUsers[cs.selectedUser]
			cs.startPageRequest(user.NodeID)
		}
	case "b":
		cs.startBroadcastPage()
	}
	return cs, nil
}

// handleAwayModeKeys handles away mode
func (cs *ChatSystem) handleAwayModeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "s":
		cs.setAwayStatus()
	case "r":
		cs.returnFromAway()
	case "m":
		cs.setAwayMessage()
	}
	return cs, nil
}

// Chat system methods

// startPrivateChat initiates a private chat with another user
func (cs *ChatSystem) startPrivateChat(targetNodeID int) {
	cs.chatMode = ChatModePrivate
	cs.chatPartner = targetNodeID
	cs.scrollOffset = 0
	
	// Initialize private chat history if needed
	if _, exists := cs.privateChats[targetNodeID]; !exists {
		cs.privateChats[targetNodeID] = make([]ChatMessage, 0)
	}
	
	// Send chat request
	message := NodeMessage{
		FromNode:    cs.currentNodeID,
		FromUser:    cs.currentUser,
		ToNode:      targetNodeID,
		Message:     fmt.Sprintf("%s requests private chat", cs.currentUser),
		MessageType: "chat_request",
		Priority:    2,
		Timestamp:   time.Now(),
	}
	
	cs.nodeManager.SendMessage(message)
	
	// Add system message to chat history
	chatMsg := ChatMessage{
		FromUser:    "System",
		FromNode:    0,
		Message:     fmt.Sprintf("Chat request sent to node %d", targetNodeID),
		Timestamp:   time.Now(),
		MessageType: "system",
		IsPrivate:   true,
	}
	
	cs.privateChats[targetNodeID] = append(cs.privateChats[targetNodeID], chatMsg)
}

// endPrivateChat ends the current private chat
func (cs *ChatSystem) endPrivateChat() {
	if cs.chatPartner > 0 {
		// Send end chat message
		message := NodeMessage{
			FromNode:    cs.currentNodeID,
			FromUser:    cs.currentUser,
			ToNode:      cs.chatPartner,
			Message:     fmt.Sprintf("%s has left the chat", cs.currentUser),
			MessageType: "chat_end",
			Priority:    2,
			Timestamp:   time.Now(),
		}
		
		cs.nodeManager.SendMessage(message)
		
		// Add system message
		chatMsg := ChatMessage{
			FromUser:    "System",
			FromNode:    0,
			Message:     "Private chat ended",
			Timestamp:   time.Now(),
			MessageType: "system",
			IsPrivate:   true,
		}
		
		if history, exists := cs.privateChats[cs.chatPartner]; exists {
			cs.privateChats[cs.chatPartner] = append(history, chatMsg)
		}
	}
	
	cs.chatMode = ChatModeSelect
	cs.chatPartner = 0
	cs.scrollOffset = 0
}

// sendCurrentMessage sends the current message
func (cs *ChatSystem) sendCurrentMessage() (tea.Model, tea.Cmd) {
	message := strings.TrimSpace(cs.messageInput)
	cs.messageInput = ""
	cs.inputMode = false
	
	if message == "" {
		return cs, nil
	}
	
	var chatMsg ChatMessage
	
	switch cs.chatMode {
	case ChatModePrivate:
		if cs.chatPartner == 0 {
			return cs, nil
		}
		
		// Send to specific node
		nodeMsg := NodeMessage{
			FromNode:    cs.currentNodeID,
			FromUser:    cs.currentUser,
			ToNode:      cs.chatPartner,
			Message:     message,
			MessageType: "private_chat",
			Priority:    2,
			Timestamp:   time.Now(),
		}
		
		cs.nodeManager.SendMessage(nodeMsg)
		
		// Add to local chat history
		chatMsg = ChatMessage{
			FromUser:    cs.currentUser,
			FromNode:    cs.currentNodeID,
			ToNode:      cs.chatPartner,
			Message:     message,
			Timestamp:   time.Now(),
			MessageType: "chat",
			IsPrivate:   true,
		}
		
		cs.privateChats[cs.chatPartner] = append(cs.privateChats[cs.chatPartner], chatMsg)
		
	case ChatModeChannel:
		// Broadcast to channel
		cs.nodeManager.BroadcastMessage(
			fmt.Sprintf("[%s] %s: %s", cs.currentChannel, cs.currentUser, message),
			cs.currentUser)
		
		// Add to channel history
		chatMsg = ChatMessage{
			FromUser:    cs.currentUser,
			FromNode:    cs.currentNodeID,
			Channel:     cs.currentChannel,
			Message:     message,
			Timestamp:   time.Now(),
			MessageType: "chat",
			IsPrivate:   false,
		}
		
		if _, exists := cs.channels[cs.currentChannel]; !exists {
			cs.channels[cs.currentChannel] = make([]ChatMessage, 0)
		}
		cs.channels[cs.currentChannel] = append(cs.channels[cs.currentChannel], chatMsg)
	}
	
	// Auto-scroll to bottom
	cs.scrollToBottom()
	
	return cs, nil
}

// sendAction sends an action message
func (cs *ChatSystem) sendAction() {
	cs.inputMode = true
	cs.messageInput = "/me "
}

// startPageRequest initiates a page to another user
func (cs *ChatSystem) startPageRequest(targetNodeID int) {
	cs.inputMode = true
	cs.chatPartner = targetNodeID
	cs.messageInput = ""
}

// startBroadcastPage initiates a broadcast page
func (cs *ChatSystem) startBroadcastPage() {
	cs.inputMode = true
	cs.chatPartner = 0 // 0 indicates broadcast
	cs.messageInput = ""
}

// setAwayStatus sets the user's away status
func (cs *ChatSystem) setAwayStatus() {
	cs.inputMode = true
	cs.messageInput = "Away: "
}

// returnFromAway returns from away status
func (cs *ChatSystem) returnFromAway() {
	// Send system message about returning
	cs.nodeManager.BroadcastMessage(
		fmt.Sprintf("%s has returned from away", cs.currentUser),
		"System")
}

// setAwayMessage sets a custom away message
func (cs *ChatSystem) setAwayMessage() {
	cs.inputMode = true
	cs.messageInput = "Away message: "
}

// startChatInvite sends a chat invite to multiple users
func (cs *ChatSystem) startChatInvite() {
	// This could open a multi-selection dialog
	// For now, just add a system message
	chatMsg := ChatMessage{
		FromUser:    "System",
		FromNode:    0,
		Message:     "Chat invite feature coming soon",
		Timestamp:   time.Now(),
		MessageType: "system",
	}
	
	cs.chatHistory = append(cs.chatHistory, chatMsg)
}

// listChannels shows available channels
func (cs *ChatSystem) listChannels() {
	channels := []string{"General", "SysOp", "Games", "Help"}
	
	chatMsg := ChatMessage{
		FromUser:    "System",
		FromNode:    0,
		Message:     fmt.Sprintf("Available channels: %s", strings.Join(channels, ", ")),
		Timestamp:   time.Now(),
		MessageType: "system",
	}
	
	cs.chatHistory = append(cs.chatHistory, chatMsg)
}

// clearCurrentChat clears the current chat history
func (cs *ChatSystem) clearCurrentChat() {
	switch cs.chatMode {
	case ChatModePrivate:
		if cs.chatPartner > 0 {
			cs.privateChats[cs.chatPartner] = make([]ChatMessage, 0)
		}
	case ChatModeChannel:
		cs.channels[cs.currentChannel] = make([]ChatMessage, 0)
	default:
		cs.chatHistory = make([]ChatMessage, 0)
	}
	cs.scrollOffset = 0
}

// refreshUserList updates the list of available users
func (cs *ChatSystem) refreshUserList() {
	nodes := cs.nodeManager.GetActiveNodes()
	users := make([]ChatUser, 0)
	
	for _, node := range nodes {
		if node.User != nil && node.NodeID != cs.currentNodeID {
			user := ChatUser{
				Handle:       node.User.Handle,
				NodeID:       node.NodeID,
				Location:     node.User.GroupLocation,
				Activity:     node.Activity.Description,
				Status:       "available",
				LastActivity: node.LastActivity,
				InPrivateChat: false,
			}
			
			// Determine status based on activity
			switch node.Status {
			case NodeStatusInChat:
				user.Status = "busy"
				user.InPrivateChat = true
			case NodeStatusInDoor:
				user.Status = "busy"
			case NodeStatusInMessage:
				user.Status = "away"
			}
			
			// Check idle time
			if node.IdleTime > 5*time.Minute {
				user.Status = "away"
			}
			
			users = append(users, user)
		}
	}
	
	// Sort users by handle
	sort.Slice(users, func(i, j int) bool {
		return users[i].Handle < users[j].Handle
	})
	
	cs.availableUsers = users
	
	// Adjust selected user if list changed
	if cs.selectedUser >= len(users) {
		cs.selectedUser = len(users) - 1
	}
	if cs.selectedUser < 0 {
		cs.selectedUser = 0
	}
}

// checkForNewMessages checks for incoming messages
func (cs *ChatSystem) checkForNewMessages() {
	messages := cs.nodeManager.GetMessages(cs.currentNodeID)
	
	for _, msg := range messages {
		if msg.MessageType == "private_chat" || msg.MessageType == "chat_request" ||
		   msg.MessageType == "chat_end" || msg.MessageType == "page" {
			
			cs.processIncomingMessage(msg)
		}
	}
}

// processIncomingMessage processes an incoming chat message
func (cs *ChatSystem) processIncomingMessage(msg NodeMessage) {
	chatMsg := ChatMessage{
		FromUser:    msg.FromUser,
		FromNode:    msg.FromNode,
		ToNode:      cs.currentNodeID,
		Message:     msg.Message,
		Timestamp:   msg.Timestamp,
		MessageType: msg.MessageType,
		IsPrivate:   true,
		Priority:    msg.Priority,
	}
	
	switch msg.MessageType {
	case "private_chat":
		// Add to private chat history
		if _, exists := cs.privateChats[msg.FromNode]; !exists {
			cs.privateChats[msg.FromNode] = make([]ChatMessage, 0)
		}
		cs.privateChats[msg.FromNode] = append(cs.privateChats[msg.FromNode], chatMsg)
		
		// If we're in private chat with this user, auto-scroll
		if cs.chatMode == ChatModePrivate && cs.chatPartner == msg.FromNode {
			cs.scrollToBottom()
		}
		
	case "chat_request":
		// Add notification
		notification := ChatNotification{
			FromUser:  msg.FromUser,
			FromNode:  msg.FromNode,
			Message:   fmt.Sprintf("%s wants to chat", msg.FromUser),
			Type:      "chat_request",
			Timestamp: time.Now(),
			Urgent:    false,
		}
		cs.notifications = append(cs.notifications, notification)
		
	case "page":
		// Add page notification
		notification := ChatNotification{
			FromUser:  msg.FromUser,
			FromNode:  msg.FromNode,
			Message:   msg.Message,
			Type:      "page",
			Timestamp: time.Now(),
			Urgent:    true,
		}
		cs.notifications = append(cs.notifications, notification)
	}
}

// getCurrentChatHistory returns the current chat history based on mode
func (cs *ChatSystem) getCurrentChatHistory() []ChatMessage {
	switch cs.chatMode {
	case ChatModePrivate:
		if cs.chatPartner > 0 {
			if history, exists := cs.privateChats[cs.chatPartner]; exists {
				return history
			}
		}
		return make([]ChatMessage, 0)
	case ChatModeChannel:
		if history, exists := cs.channels[cs.currentChannel]; exists {
			return history
		}
		return make([]ChatMessage, 0)
	default:
		return cs.chatHistory
	}
}

// scrollToBottom scrolls chat to the bottom
func (cs *ChatSystem) scrollToBottom() {
	history := cs.getCurrentChatHistory()
	maxVisible := cs.height - 10
	if len(history) > maxVisible {
		cs.scrollOffset = len(history) - maxVisible
	} else {
		cs.scrollOffset = 0
	}
}

// View renders the chat system interface (continued in next file)
func (cs *ChatSystem) View() string {
	var sections []string
	
	// Title bar
	sections = append(sections, cs.renderTitleBar())
	
	// Mode tabs
	sections = append(sections, cs.renderModeTabs())
	
	// Main content area
	if cs.showUserList && (cs.chatMode == ChatModeSelect || cs.chatMode == ChatModePage) {
		// Split view: chat history and user list
		sections = append(sections, cs.renderSplitView())
	} else {
		// Full chat view
		sections = append(sections, cs.renderChatView())
	}
	
	// Input area
	if cs.inputMode {
		sections = append(sections, cs.renderInputArea())
	}
	
	// Notifications
	if len(cs.notifications) > 0 {
		sections = append(sections, cs.renderNotifications())
	}
	
	// Help line
	sections = append(sections, cs.renderHelpLine())
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Init implements tea.Model
func (cs *ChatSystem) Init() tea.Cmd {
	cs.refreshUserList()
	return cs.tick()
}

// Rendering methods will be continued in the next file...