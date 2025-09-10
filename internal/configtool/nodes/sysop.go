package nodes

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SysopTools represents the sysop management interface
type SysopTools struct {
	nodeManager    NodeManager
	width          int
	height         int
	focused        bool
	currentTool    SysopTool
	selectedNode   int
	commandInput   string
	messageInput   string
	confirmDialog  bool
	confirmAction  string
	outputLog      []string
	scrollOffset   int
	maxLogLines    int
	broadcastMode  bool
	recipientNodes []int
}

// SysopTool represents different sysop tool modes
type SysopTool int

const (
	NodeControlTool SysopTool = iota
	MessageTool
	MonitorTool
	LogViewTool
	StatsTool
	EmergencyTool
)

// NewSysopTools creates a new sysop tools interface
func NewSysopTools(nodeManager NodeManager, width, height int) *SysopTools {
	return &SysopTools{
		nodeManager:  nodeManager,
		width:        width,
		height:       height,
		currentTool:  NodeControlTool,
		selectedNode: 1,
		outputLog:    make([]string, 0),
		maxLogLines:  50,
	}
}

// Update implements tea.Model
func (st *SysopTools) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return st.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		st.width = msg.Width
		st.height = msg.Height
	case TickMsg:
		// Auto-refresh for monitoring tool
		if st.currentTool == MonitorTool {
			return st, st.tick()
		}
	}
	return st, nil
}

// tick returns a command for periodic updates
func (st *SysopTools) tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// handleKeyPress processes keyboard input
func (st *SysopTools) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle confirmation dialog
	if st.confirmDialog {
		return st.handleConfirmDialog(msg)
	}

	// Handle input modes
	if st.isInputMode() {
		return st.handleInputMode(msg)
	}

	// Global commands
	switch msg.String() {
	case "q", "esc":
		return st, tea.Quit
	case "tab":
		st.cycleTool()
	case "f1":
		st.currentTool = NodeControlTool
	case "f2":
		st.currentTool = MessageTool
	case "f3":
		st.currentTool = MonitorTool
	case "f4":
		st.currentTool = LogViewTool
	case "f5":
		st.currentTool = StatsTool
	case "f12":
		st.currentTool = EmergencyTool
	}

	// Tool-specific commands
	switch st.currentTool {
	case NodeControlTool:
		return st.handleNodeControlKeys(msg)
	case MessageTool:
		return st.handleMessageToolKeys(msg)
	case MonitorTool:
		return st.handleMonitorToolKeys(msg)
	case LogViewTool:
		return st.handleLogViewKeys(msg)
	case StatsTool:
		return st.handleStatsToolKeys(msg)
	case EmergencyTool:
		return st.handleEmergencyToolKeys(msg)
	}

	return st, nil
}

// cycleTool cycles through available tools
func (st *SysopTools) cycleTool() {
	switch st.currentTool {
	case NodeControlTool:
		st.currentTool = MessageTool
	case MessageTool:
		st.currentTool = MonitorTool
	case MonitorTool:
		st.currentTool = LogViewTool
	case LogViewTool:
		st.currentTool = StatsTool
	case StatsTool:
		st.currentTool = EmergencyTool
	case EmergencyTool:
		st.currentTool = NodeControlTool
	}
}

// isInputMode checks if we're in an input mode
func (st *SysopTools) isInputMode() bool {
	return st.commandInput != "" || st.messageInput != "" || st.broadcastMode
}

// handleConfirmDialog handles confirmation dialog input
func (st *SysopTools) handleConfirmDialog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		st.confirmDialog = false
		return st.executeConfirmedAction()
	case "n", "N", "esc":
		st.confirmDialog = false
		st.confirmAction = ""
		st.addToLog("Action cancelled")
	}
	return st, nil
}

// handleInputMode handles input mode keys
func (st *SysopTools) handleInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if st.broadcastMode {
			return st.executeBroadcast()
		} else if st.messageInput != "" {
			return st.sendMessage()
		} else if st.commandInput != "" {
			return st.executeCommand()
		}
	case "esc":
		st.commandInput = ""
		st.messageInput = ""
		st.broadcastMode = false
		st.recipientNodes = nil
	case "backspace":
		if st.broadcastMode && len(st.messageInput) > 0 {
			st.messageInput = st.messageInput[:len(st.messageInput)-1]
		} else if len(st.commandInput) > 0 {
			st.commandInput = st.commandInput[:len(st.commandInput)-1]
		} else if len(st.messageInput) > 0 {
			st.messageInput = st.messageInput[:len(st.messageInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			if st.broadcastMode {
				st.messageInput += msg.String()
			} else if st.messageInput != "" || st.currentTool == MessageTool {
				st.messageInput += msg.String()
			} else {
				st.commandInput += msg.String()
			}
		}
	}
	return st, nil
}

// Node Control Tool handlers
func (st *SysopTools) handleNodeControlKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if st.selectedNode > 1 {
			st.selectedNode--
		}
	case "down", "j":
		nodes := st.nodeManager.GetAllNodes()
		if st.selectedNode < len(nodes) {
			st.selectedNode++
		}
	case "d":
		st.confirmAction = "disconnect"
		st.confirmDialog = true
	case "r":
		st.confirmAction = "restart"
		st.confirmDialog = true
	case "e":
		st.confirmAction = "enable"
		st.confirmDialog = true
	case "x":
		st.confirmAction = "disable"
		st.confirmDialog = true
	case "m":
		st.messageInput = "Enter message for node " + strconv.Itoa(st.selectedNode) + ": "
	case "c":
		st.commandInput = "Enter command: "
	}
	return st, nil
}

// Message Tool handlers
func (st *SysopTools) handleMessageToolKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "b":
		st.broadcastMode = true
		st.messageInput = ""
		st.recipientNodes = st.getActiveNodeIDs()
	case "s":
		st.broadcastMode = false
		st.messageInput = "Enter message: "
	case "a":
		// Select all active nodes
		st.recipientNodes = st.getActiveNodeIDs()
	case "c":
		// Clear selection
		st.recipientNodes = nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		nodeID, _ := strconv.Atoi(msg.String())
		st.toggleRecipientNode(nodeID)
	}
	return st, nil
}

// Monitor Tool handlers
func (st *SysopTools) handleMonitorToolKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		st.addToLog("Monitor refreshed manually")
	case "up", "k":
		if st.selectedNode > 1 {
			st.selectedNode--
		}
	case "down", "j":
		nodes := st.nodeManager.GetAllNodes()
		if st.selectedNode < len(nodes) {
			st.selectedNode++
		}
	case "enter":
		// Show detailed info for selected node
		node, err := st.nodeManager.GetNode(st.selectedNode)
		if err == nil {
			st.addToLog(fmt.Sprintf("Node %d details: %s", node.NodeID, st.formatNodeDetails(node)))
		}
	}
	return st, nil
}

// Log View handlers
func (st *SysopTools) handleLogViewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if st.scrollOffset > 0 {
			st.scrollOffset--
		}
	case "down", "j":
		if st.scrollOffset < len(st.outputLog)-1 {
			st.scrollOffset++
		}
	case "pageup":
		st.scrollOffset -= 10
		if st.scrollOffset < 0 {
			st.scrollOffset = 0
		}
	case "pagedown":
		st.scrollOffset += 10
		if st.scrollOffset >= len(st.outputLog) {
			st.scrollOffset = len(st.outputLog) - 1
		}
	case "home":
		st.scrollOffset = 0
	case "end":
		st.scrollOffset = len(st.outputLog) - 1
	case "c":
		st.outputLog = make([]string, 0)
		st.scrollOffset = 0
		st.addToLog("Log cleared")
	}
	return st, nil
}

// Stats Tool handlers
func (st *SysopTools) handleStatsToolKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		st.addToLog("Statistics refreshed")
	case "up", "k":
		if st.selectedNode > 1 {
			st.selectedNode--
		}
	case "down", "j":
		nodes := st.nodeManager.GetAllNodes()
		if st.selectedNode < len(nodes) {
			st.selectedNode++
		}
	case "enter":
		// Show detailed stats for selected node
		stats, err := st.nodeManager.GetNodeStatistics(st.selectedNode)
		if err == nil {
			st.addToLog(fmt.Sprintf("Node %d stats: %s", st.selectedNode, st.formatNodeStats(stats)))
		}
	}
	return st, nil
}

// Emergency Tool handlers
func (st *SysopTools) handleEmergencyToolKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "s":
		st.confirmAction = "shutdown_all"
		st.confirmDialog = true
	case "k":
		st.confirmAction = "kill_all"
		st.confirmDialog = true
	case "r":
		st.confirmAction = "restart_all"
		st.confirmDialog = true
	case "m":
		st.confirmAction = "maintenance_mode"
		st.confirmDialog = true
	case "b":
		st.broadcastMode = true
		st.messageInput = ""
		st.recipientNodes = st.getActiveNodeIDs()
	}
	return st, nil
}

// executeConfirmedAction executes the confirmed action
func (st *SysopTools) executeConfirmedAction() (tea.Model, tea.Cmd) {
	switch st.confirmAction {
	case "disconnect":
		err := st.nodeManager.DisconnectUser(st.selectedNode, "Disconnected by SysOp")
		if err != nil {
			st.addToLog(fmt.Sprintf("Failed to disconnect node %d: %v", st.selectedNode, err))
		} else {
			st.addToLog(fmt.Sprintf("Node %d user disconnected", st.selectedNode))
		}
	case "restart":
		err := st.nodeManager.RestartNode(st.selectedNode)
		if err != nil {
			st.addToLog(fmt.Sprintf("Failed to restart node %d: %v", st.selectedNode, err))
		} else {
			st.addToLog(fmt.Sprintf("Node %d restarted", st.selectedNode))
		}
	case "enable":
		err := st.nodeManager.EnableNode(st.selectedNode)
		if err != nil {
			st.addToLog(fmt.Sprintf("Failed to enable node %d: %v", st.selectedNode, err))
		} else {
			st.addToLog(fmt.Sprintf("Node %d enabled", st.selectedNode))
		}
	case "disable":
		err := st.nodeManager.DisableNode(st.selectedNode)
		if err != nil {
			st.addToLog(fmt.Sprintf("Failed to disable node %d: %v", st.selectedNode, err))
		} else {
			st.addToLog(fmt.Sprintf("Node %d disabled", st.selectedNode))
		}
	case "shutdown_all":
		st.executeShutdownAll()
	case "kill_all":
		st.executeKillAll()
	case "restart_all":
		st.executeRestartAll()
	case "maintenance_mode":
		st.executeMaintenanceMode()
	}
	
	st.confirmAction = ""
	return st, nil
}

// executeCommand executes a sysop command
func (st *SysopTools) executeCommand() (tea.Model, tea.Cmd) {
	command := strings.TrimSpace(st.commandInput)
	st.commandInput = ""
	
	if command == "" {
		return st, nil
	}
	
	st.addToLog(fmt.Sprintf("Command: %s", command))
	
	// Parse and execute command
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return st, nil
	}
	
	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "status":
		st.executeStatusCommand(parts[1:])
	case "kick":
		st.executeKickCommand(parts[1:])
	case "msg":
		st.executeMessageCommand(parts[1:])
	case "broadcast":
		st.executeBroadcastCommand(parts[1:])
	case "enable":
		st.executeEnableCommand(parts[1:])
	case "disable":
		st.executeDisableCommand(parts[1:])
	case "restart":
		st.executeRestartCommand(parts[1:])
	case "help":
		st.executeHelpCommand()
	default:
		st.addToLog(fmt.Sprintf("Unknown command: %s", cmd))
	}
	
	return st, nil
}

// sendMessage sends a message to the selected node
func (st *SysopTools) sendMessage() (tea.Model, tea.Cmd) {
	message := strings.TrimSpace(st.messageInput)
	st.messageInput = ""
	
	if message == "" {
		return st, nil
	}
	
	err := st.nodeManager.SendUserMessage(st.selectedNode, message)
	if err != nil {
		st.addToLog(fmt.Sprintf("Failed to send message to node %d: %v", st.selectedNode, err))
	} else {
		st.addToLog(fmt.Sprintf("Message sent to node %d: %s", st.selectedNode, message))
	}
	
	return st, nil
}

// executeBroadcast sends a broadcast message
func (st *SysopTools) executeBroadcast() (tea.Model, tea.Cmd) {
	message := strings.TrimSpace(st.messageInput)
	st.messageInput = ""
	st.broadcastMode = false
	
	if message == "" {
		return st, nil
	}
	
	err := st.nodeManager.BroadcastMessage(message, "SysOp")
	if err != nil {
		st.addToLog(fmt.Sprintf("Failed to broadcast message: %v", err))
	} else {
		st.addToLog(fmt.Sprintf("Broadcast sent: %s", message))
	}
	
	st.recipientNodes = nil
	return st, nil
}

// Command execution methods
func (st *SysopTools) executeStatusCommand(args []string) {
	if len(args) == 0 {
		// Show system status
		systemStatus, err := st.nodeManager.GetSystemStatus()
		if err != nil {
			st.addToLog(fmt.Sprintf("Failed to get system status: %v", err))
			return
		}
		st.addToLog(fmt.Sprintf("System Status - Nodes: %d/%d, Users: %d, Load: %.1f",
			systemStatus.ActiveNodes, systemStatus.TotalNodes,
			systemStatus.ConnectedUsers, systemStatus.SystemLoad))
	} else {
		// Show specific node status
		nodeID, err := strconv.Atoi(args[0])
		if err != nil {
			st.addToLog("Invalid node ID")
			return
		}
		
		node, err := st.nodeManager.GetNode(nodeID)
		if err != nil {
			st.addToLog(fmt.Sprintf("Node %d not found", nodeID))
			return
		}
		
		st.addToLog(fmt.Sprintf("Node %d Status: %s", nodeID, st.formatNodeDetails(node)))
	}
}

func (st *SysopTools) executeKickCommand(args []string) {
	if len(args) == 0 {
		st.addToLog("Usage: kick <node_id> [reason]")
		return
	}
	
	nodeID, err := strconv.Atoi(args[0])
	if err != nil {
		st.addToLog("Invalid node ID")
		return
	}
	
	reason := "Kicked by SysOp"
	if len(args) > 1 {
		reason = strings.Join(args[1:], " ")
	}
	
	err = st.nodeManager.DisconnectUser(nodeID, reason)
	if err != nil {
		st.addToLog(fmt.Sprintf("Failed to kick user from node %d: %v", nodeID, err))
	} else {
		st.addToLog(fmt.Sprintf("User kicked from node %d", nodeID))
	}
}

func (st *SysopTools) executeMessageCommand(args []string) {
	if len(args) < 2 {
		st.addToLog("Usage: msg <node_id> <message>")
		return
	}
	
	nodeID, err := strconv.Atoi(args[0])
	if err != nil {
		st.addToLog("Invalid node ID")
		return
	}
	
	message := strings.Join(args[1:], " ")
	err = st.nodeManager.SendUserMessage(nodeID, message)
	if err != nil {
		st.addToLog(fmt.Sprintf("Failed to send message to node %d: %v", nodeID, err))
	} else {
		st.addToLog(fmt.Sprintf("Message sent to node %d", nodeID))
	}
}

func (st *SysopTools) executeBroadcastCommand(args []string) {
	if len(args) == 0 {
		st.addToLog("Usage: broadcast <message>")
		return
	}
	
	message := strings.Join(args, " ")
	err := st.nodeManager.BroadcastMessage(message, "SysOp")
	if err != nil {
		st.addToLog(fmt.Sprintf("Failed to broadcast: %v", err))
	} else {
		st.addToLog("Broadcast message sent")
	}
}

func (st *SysopTools) executeEnableCommand(args []string) {
	if len(args) == 0 {
		st.addToLog("Usage: enable <node_id>")
		return
	}
	
	nodeID, err := strconv.Atoi(args[0])
	if err != nil {
		st.addToLog("Invalid node ID")
		return
	}
	
	err = st.nodeManager.EnableNode(nodeID)
	if err != nil {
		st.addToLog(fmt.Sprintf("Failed to enable node %d: %v", nodeID, err))
	} else {
		st.addToLog(fmt.Sprintf("Node %d enabled", nodeID))
	}
}

func (st *SysopTools) executeDisableCommand(args []string) {
	if len(args) == 0 {
		st.addToLog("Usage: disable <node_id>")
		return
	}
	
	nodeID, err := strconv.Atoi(args[0])
	if err != nil {
		st.addToLog("Invalid node ID")
		return
	}
	
	err = st.nodeManager.DisableNode(nodeID)
	if err != nil {
		st.addToLog(fmt.Sprintf("Failed to disable node %d: %v", nodeID, err))
	} else {
		st.addToLog(fmt.Sprintf("Node %d disabled", nodeID))
	}
}

func (st *SysopTools) executeRestartCommand(args []string) {
	if len(args) == 0 {
		st.addToLog("Usage: restart <node_id>")
		return
	}
	
	nodeID, err := strconv.Atoi(args[0])
	if err != nil {
		st.addToLog("Invalid node ID")
		return
	}
	
	err = st.nodeManager.RestartNode(nodeID)
	if err != nil {
		st.addToLog(fmt.Sprintf("Failed to restart node %d: %v", nodeID, err))
	} else {
		st.addToLog(fmt.Sprintf("Node %d restarted", nodeID))
	}
}

func (st *SysopTools) executeHelpCommand() {
	helpText := []string{
		"Available commands:",
		"  status [node_id]     - Show system or node status",
		"  kick <node_id> [reason] - Disconnect user from node",
		"  msg <node_id> <text> - Send message to node",
		"  broadcast <text>     - Send message to all nodes",
		"  enable <node_id>     - Enable a node",
		"  disable <node_id>    - Disable a node",
		"  restart <node_id>    - Restart a node",
		"  help                 - Show this help",
	}
	
	for _, line := range helpText {
		st.addToLog(line)
	}
}

// Emergency operations
func (st *SysopTools) executeShutdownAll() {
	nodes := st.nodeManager.GetAllNodes()
	count := 0
	
	for _, node := range nodes {
		if node.User != nil {
			err := st.nodeManager.DisconnectUser(node.NodeID, "System shutdown")
			if err == nil {
				count++
			}
		}
		st.nodeManager.DisableNode(node.NodeID)
	}
	
	st.addToLog(fmt.Sprintf("Emergency shutdown: %d users disconnected, all nodes disabled", count))
}

func (st *SysopTools) executeKillAll() {
	nodes := st.nodeManager.GetAllNodes()
	count := 0
	
	for _, node := range nodes {
		if node.User != nil {
			err := st.nodeManager.DisconnectUser(node.NodeID, "Emergency disconnection")
			if err == nil {
				count++
			}
		}
	}
	
	st.addToLog(fmt.Sprintf("Emergency kill: %d users forcibly disconnected", count))
}

func (st *SysopTools) executeRestartAll() {
	nodes := st.nodeManager.GetAllNodes()
	count := 0
	
	for _, node := range nodes {
		err := st.nodeManager.RestartNode(node.NodeID)
		if err == nil {
			count++
		}
	}
	
	st.addToLog(fmt.Sprintf("Mass restart: %d nodes restarted", count))
}

func (st *SysopTools) executeMaintenanceMode() {
	// Disconnect all users and disable all nodes except node 1
	nodes := st.nodeManager.GetAllNodes()
	count := 0
	
	for _, node := range nodes {
		if node.User != nil {
			st.nodeManager.DisconnectUser(node.NodeID, "System entering maintenance mode")
			count++
		}
		if node.NodeID != 1 {
			st.nodeManager.DisableNode(node.NodeID)
		}
	}
	
	st.addToLog(fmt.Sprintf("Maintenance mode: %d users disconnected, nodes 2+ disabled", count))
}

// Helper methods
func (st *SysopTools) getActiveNodeIDs() []int {
	nodes := st.nodeManager.GetActiveNodes()
	var nodeIDs []int
	
	for _, node := range nodes {
		if node.User != nil {
			nodeIDs = append(nodeIDs, node.NodeID)
		}
	}
	
	return nodeIDs
}

func (st *SysopTools) toggleRecipientNode(nodeID int) {
	for i, id := range st.recipientNodes {
		if id == nodeID {
			// Remove from selection
			st.recipientNodes = append(st.recipientNodes[:i], st.recipientNodes[i+1:]...)
			return
		}
	}
	
	// Add to selection
	st.recipientNodes = append(st.recipientNodes, nodeID)
	sort.Ints(st.recipientNodes)
}

func (st *SysopTools) addToLog(message string) {
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] %s", timestamp, message)
	
	st.outputLog = append(st.outputLog, logEntry)
	
	// Limit log size
	if len(st.outputLog) > st.maxLogLines {
		st.outputLog = st.outputLog[len(st.outputLog)-st.maxLogLines:]
	}
	
	// Auto-scroll to bottom
	st.scrollOffset = len(st.outputLog) - 1
}

func (st *SysopTools) formatNodeDetails(node *NodeInfo) string {
	var details []string
	
	details = append(details, fmt.Sprintf("Status: %s", node.Status.String()))
	
	if node.User != nil {
		details = append(details, fmt.Sprintf("User: %s (%s)", 
			node.User.Handle, node.User.GroupLocation))
		details = append(details, fmt.Sprintf("Activity: %s", node.Activity.Description))
		
		if !node.ConnectTime.IsZero() {
			duration := time.Since(node.ConnectTime)
			details = append(details, fmt.Sprintf("Online: %s", formatDuration(duration)))
		}
		
		if node.IdleTime > 0 {
			details = append(details, fmt.Sprintf("Idle: %s", formatDuration(node.IdleTime)))
		}
		
		if node.RemoteAddr != nil {
			details = append(details, fmt.Sprintf("From: %s", node.RemoteAddr.String()))
		}
	} else {
		details = append(details, "No user connected")
	}
	
	return strings.Join(details, ", ")
}

func (st *SysopTools) formatNodeStats(stats *NodeStatistics) string {
	var statStrings []string
	
	statStrings = append(statStrings, fmt.Sprintf("Connections: %d", stats.TotalConnections))
	statStrings = append(statStrings, fmt.Sprintf("Total Time: %s", stats.TotalTime.String()))
	statStrings = append(statStrings, fmt.Sprintf("Avg Session: %s", stats.AverageSession.String()))
	statStrings = append(statStrings, fmt.Sprintf("Uptime: %.1f%%", stats.UptimePercent))
	
	if stats.ErrorCount > 0 {
		statStrings = append(statStrings, fmt.Sprintf("Errors: %d", stats.ErrorCount))
	}
	
	return strings.Join(statStrings, ", ")
}

// View renders the sysop tools interface
func (st *SysopTools) View() string {
	var sections []string
	
	// Title bar
	sections = append(sections, st.renderTitleBar())
	
	// Tool tabs
	sections = append(sections, st.renderToolTabs())
	
	// Main content area
	sections = append(sections, st.renderMainContent())
	
	// Input area
	if st.isInputMode() {
		sections = append(sections, st.renderInputArea())
	}
	
	// Confirmation dialog
	if st.confirmDialog {
		sections = append(sections, st.renderConfirmDialog())
	}
	
	// Help line
	sections = append(sections, st.renderHelpLine())
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderTitleBar renders the title bar
func (st *SysopTools) renderTitleBar() string {
	title := "ViSiON/3 SysOp Tools"
	
	titleStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("1")).
		Foreground(lipgloss.Color("15")).
		Bold(true).
		Padding(0, 1).
		Width(st.width)
	
	return titleStyle.Render(title)
}

// renderToolTabs renders the tool selection tabs
func (st *SysopTools) renderToolTabs() string {
	tabs := []string{
		"F1:Node Control", "F2:Messaging", "F3:Monitor", 
		"F4:Log View", "F5:Statistics", "F12:Emergency",
	}
	
	var tabElements []string
	for i, tab := range tabs {
		tabStyle := lipgloss.NewStyle().Padding(0, 1)
		
		if SysopTool(i) == st.currentTool {
			tabStyle = tabStyle.Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15"))
		} else {
			tabStyle = tabStyle.Background(lipgloss.Color("7")).Foreground(lipgloss.Color("0"))
		}
		
		tabElements = append(tabElements, tabStyle.Render(tab))
	}
	
	return lipgloss.JoinHorizontal(lipgloss.Top, tabElements...)
}

// renderMainContent renders the main content area based on current tool
func (st *SysopTools) renderMainContent() string {
	switch st.currentTool {
	case NodeControlTool:
		return st.renderNodeControlContent()
	case MessageTool:
		return st.renderMessageToolContent()
	case MonitorTool:
		return st.renderMonitorContent()
	case LogViewTool:
		return st.renderLogViewContent()
	case StatsTool:
		return st.renderStatsContent()
	case EmergencyTool:
		return st.renderEmergencyContent()
	default:
		return "Unknown tool"
	}
}

// Tool-specific content renderers will be continued in the next part...