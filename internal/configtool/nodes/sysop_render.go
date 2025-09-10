package nodes

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Tool-specific content renderers

// renderNodeControlContent renders the node control interface
func (st *SysopTools) renderNodeControlContent() string {
	nodes := st.nodeManager.GetAllNodes()
	
	var lines []string
	lines = append(lines, "Node Control - Select a node and perform actions")
	lines = append(lines, "")
	
	// Node list with status
	for _, node := range nodes {
		var status, userInfo string
		var lineStyle lipgloss.Style
		
		// Status indicator
		switch node.Status {
		case NodeStatusAvailable:
			status = "◯ Available"
			lineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		case NodeStatusConnected, NodeStatusLoggedIn:
			status = "● Connected"
			lineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
		case NodeStatusInMenu, NodeStatusInDoor, NodeStatusInMessage:
			status = "● Active"
			lineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
		case NodeStatusDisabled:
			status = "✗ Disabled"
			lineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		default:
			status = "? Unknown"
			lineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		}
		
		// User information
		if node.User != nil {
			userInfo = fmt.Sprintf(" - %s (%s) - %s", 
				node.User.Handle, 
				node.User.GroupLocation,
				node.Activity.Description)
		}
		
		// Highlight selected node
		if node.NodeID == st.selectedNode {
			lineStyle = lineStyle.Background(lipgloss.Color("4")).Bold(true)
		}
		
		line := fmt.Sprintf("Node %2d: %s%s", node.NodeID, status, userInfo)
		lines = append(lines, lineStyle.Render(line))
	}
	
	lines = append(lines, "")
	lines = append(lines, "Actions: [D]isconnect [R]estart [E]nable [X]disable [M]essage [C]ommand")
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(st.width - 4).
		Height(st.height - 10)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderMessageToolContent renders the messaging interface
func (st *SysopTools) renderMessageToolContent() string {
	var lines []string
	lines = append(lines, "Message Tool - Send messages to nodes")
	lines = append(lines, "")
	
	if st.broadcastMode {
		lines = append(lines, "Broadcast Mode - Message will be sent to all active nodes")
		lines = append(lines, fmt.Sprintf("Recipients: %v", st.recipientNodes))
		lines = append(lines, "")
	} else {
		lines = append(lines, fmt.Sprintf("Selected Node: %d", st.selectedNode))
		lines = append(lines, "")
	}
	
	// Show active nodes that can receive messages
	activeNodes := st.nodeManager.GetActiveNodes()
	lines = append(lines, "Active Nodes:")
	
	for _, node := range activeNodes {
		var indicator string
		if node.User != nil {
			indicator = "●"
			if st.broadcastMode {
				for _, id := range st.recipientNodes {
					if id == node.NodeID {
						indicator = "◉"
						break
					}
				}
			}
			
			line := fmt.Sprintf("  %s Node %d: %s (%s) - %s",
				indicator, node.NodeID, node.User.Handle, 
				node.User.GroupLocation, node.Activity.Description)
			lines = append(lines, line)
		}
	}
	
	lines = append(lines, "")
	lines = append(lines, "Actions:")
	lines = append(lines, "  [B]roadcast mode  [S]ingle message  [A]ll nodes  [C]lear selection")
	lines = append(lines, "  [1-9] Toggle node in broadcast list")
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(st.width - 4).
		Height(st.height - 10)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderMonitorContent renders the real-time monitoring interface
func (st *SysopTools) renderMonitorContent() string {
	systemStatus, _ := st.nodeManager.GetSystemStatus()
	
	var lines []string
	lines = append(lines, "Real-Time Node Monitor")
	lines = append(lines, "")
	
	// System overview
	lines = append(lines, fmt.Sprintf("System Status: %d/%d nodes active, %d users online",
		systemStatus.ActiveNodes, systemStatus.TotalNodes, systemStatus.ConnectedUsers))
	lines = append(lines, fmt.Sprintf("System Load: %.1f%%, Memory: %d MB",
		systemStatus.SystemLoad, systemStatus.MemoryUsage/(1024*1024)))
	lines = append(lines, "")
	
	// Node grid
	lines = append(lines, "Node Status Grid:")
	lines = append(lines, "")
	
	nodes := st.nodeManager.GetAllNodes()
	
	// Create a 4-column grid
	cols := 4
	for i := 0; i < len(nodes); i += cols {
		var rowNodes []string
		
		for j := 0; j < cols && i+j < len(nodes); j++ {
			node := nodes[i+j]
			nodeStr := st.formatNodeForGrid(node)
			
			if node.NodeID == st.selectedNode {
				style := lipgloss.NewStyle().Background(lipgloss.Color("4")).Padding(0, 1)
				nodeStr = style.Render(nodeStr)
			}
			
			rowNodes = append(rowNodes, nodeStr)
		}
		
		lines = append(lines, strings.Join(rowNodes, " "))
	}
	
	lines = append(lines, "")
	
	// Recent activity
	lines = append(lines, "Recent Activity:")
	logLines := st.outputLog
	maxRecentLines := 5
	if len(logLines) > maxRecentLines {
		logLines = logLines[len(logLines)-maxRecentLines:]
	}
	
	for _, logLine := range logLines {
		lines = append(lines, "  "+logLine)
	}
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(st.width - 4).
		Height(st.height - 10)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderLogViewContent renders the log viewer
func (st *SysopTools) renderLogViewContent() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("System Log - %d entries", len(st.outputLog)))
	lines = append(lines, "")
	
	if len(st.outputLog) == 0 {
		lines = append(lines, "No log entries")
	} else {
		// Calculate visible range
		maxVisible := st.height - 15
		startIdx := st.scrollOffset
		endIdx := startIdx + maxVisible
		
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx > len(st.outputLog) {
			endIdx = len(st.outputLog)
		}
		if startIdx >= len(st.outputLog) {
			startIdx = len(st.outputLog) - 1
		}
		
		// Show log entries
		for i := startIdx; i < endIdx; i++ {
			line := st.outputLog[i]
			if i == st.scrollOffset {
				style := lipgloss.NewStyle().Background(lipgloss.Color("4"))
				line = style.Render(line)
			}
			lines = append(lines, line)
		}
		
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Showing %d-%d of %d entries", 
			startIdx+1, endIdx, len(st.outputLog)))
	}
	
	lines = append(lines, "")
	lines = append(lines, "Navigation: ↑/↓ Line  PgUp/PgDn Page  Home/End  [C]lear log")
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(st.width - 4).
		Height(st.height - 10)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderStatsContent renders statistics and performance data
func (st *SysopTools) renderStatsContent() string {
	systemStatus, _ := st.nodeManager.GetSystemStatus()
	
	var lines []string
	lines = append(lines, "Node Statistics and Performance")
	lines = append(lines, "")
	
	// System statistics
	lines = append(lines, "System Overview:")
	lines = append(lines, fmt.Sprintf("  Total Nodes: %d", systemStatus.TotalNodes))
	lines = append(lines, fmt.Sprintf("  Active Nodes: %d", systemStatus.ActiveNodes))
	lines = append(lines, fmt.Sprintf("  Connected Users: %d", systemStatus.ConnectedUsers))
	lines = append(lines, fmt.Sprintf("  System Load: %.1f%%", systemStatus.SystemLoad))
	lines = append(lines, fmt.Sprintf("  Memory Usage: %d MB", systemStatus.MemoryUsage/(1024*1024)))
	lines = append(lines, "")
	
	// Per-node statistics
	lines = append(lines, "Node Statistics:")
	lines = append(lines, fmt.Sprintf("%-4s %-12s %-8s %-8s %-8s %-6s", 
		"Node", "Status", "Connects", "Uptime%", "Errors", "Load%"))
	lines = append(lines, strings.Repeat("─", 60))
	
	nodes := st.nodeManager.GetAllNodes()
	for _, node := range nodes {
		stats, err := st.nodeManager.GetNodeStatistics(node.NodeID)
		var statsLine string
		
		if err != nil {
			statsLine = fmt.Sprintf("%-4d %-12s %-8s %-8s %-8s %-6s",
				node.NodeID, node.Status.String(), "N/A", "N/A", "N/A", "N/A")
		} else {
			statsLine = fmt.Sprintf("%-4d %-12s %-8d %-8.1f %-8d %-6.1f",
				node.NodeID, 
				truncateString(node.Status.String(), 12),
				stats.TotalConnections,
				stats.UptimePercent,
				stats.ErrorCount,
				node.CPUUsage)
		}
		
		// Highlight selected node
		if node.NodeID == st.selectedNode {
			style := lipgloss.NewStyle().Background(lipgloss.Color("4"))
			statsLine = style.Render(statsLine)
		}
		
		lines = append(lines, statsLine)
	}
	
	lines = append(lines, "")
	lines = append(lines, "Actions: ↑/↓ Select node  Enter: Detailed stats  R: Refresh")
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(st.width - 4).
		Height(st.height - 10)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderEmergencyContent renders the emergency tools interface
func (st *SysopTools) renderEmergencyContent() string {
	var lines []string
	lines = append(lines, "EMERGENCY TOOLS - USE WITH CAUTION")
	lines = append(lines, strings.Repeat("=", 40))
	lines = append(lines, "")
	
	// Current system status
	systemStatus, _ := st.nodeManager.GetSystemStatus()
	lines = append(lines, fmt.Sprintf("Current Status: %d active nodes, %d connected users",
		systemStatus.ActiveNodes, systemStatus.ConnectedUsers))
	lines = append(lines, "")
	
	// Emergency actions
	emergencyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	
	lines = append(lines, emergencyStyle.Render("IMMEDIATE ACTIONS:"))
	lines = append(lines, "")
	
	lines = append(lines, warningStyle.Render("[S] Shutdown All") + 
		" - Disconnect all users and disable all nodes")
	lines = append(lines, warningStyle.Render("[K] Kill All Sessions") + 
		" - Forcibly disconnect all users immediately")
	lines = append(lines, warningStyle.Render("[R] Restart All Nodes") + 
		" - Restart all node processes")
	lines = append(lines, warningStyle.Render("[M] Maintenance Mode") + 
		" - Enable maintenance mode (node 1 only)")
	lines = append(lines, warningStyle.Render("[B] Emergency Broadcast") + 
		" - Send urgent message to all users")
	
	lines = append(lines, "")
	lines = append(lines, "CURRENT ALERTS:")
	
	// Show recent alerts
	alerts := st.nodeManager.GetAlerts()
	recentAlerts := alerts
	if len(recentAlerts) > 5 {
		recentAlerts = recentAlerts[len(recentAlerts)-5:]
	}
	
	if len(recentAlerts) == 0 {
		lines = append(lines, "  No active alerts")
	} else {
		for _, alert := range recentAlerts {
			var alertStyle lipgloss.Style
			switch alert.AlertType {
			case "error":
				alertStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
			case "warning":
				alertStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
			default:
				alertStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
			}
			
			alertLine := fmt.Sprintf("  %s: Node %d - %s",
				alert.AlertType, alert.NodeID, alert.Message)
			lines = append(lines, alertStyle.Render(alertLine))
		}
	}
	
	lines = append(lines, "")
	lines = append(lines, emergencyStyle.Render("WARNING: These actions cannot be undone!"))
	lines = append(lines, "All emergency actions require confirmation.")
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("1")).
		Padding(1).
		Width(st.width - 4).
		Height(st.height - 10)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderInputArea renders the input area for commands/messages
func (st *SysopTools) renderInputArea() string {
	var prompt, input string
	
	if st.broadcastMode {
		prompt = "Broadcast message: "
		input = st.messageInput
	} else if st.messageInput != "" {
		prompt = fmt.Sprintf("Message to node %d: ", st.selectedNode)
		input = st.messageInput
	} else if st.commandInput != "" {
		prompt = "Command: "
		input = st.commandInput
	}
	
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(0, 1).
		Width(st.width - 4)
	
	inputLine := prompt + input + "█" // Show cursor
	return inputStyle.Render(inputLine)
}

// renderConfirmDialog renders the confirmation dialog
func (st *SysopTools) renderConfirmDialog() string {
	var message string
	
	switch st.confirmAction {
	case "disconnect":
		message = fmt.Sprintf("Disconnect user from node %d?", st.selectedNode)
	case "restart":
		message = fmt.Sprintf("Restart node %d?", st.selectedNode)
	case "enable":
		message = fmt.Sprintf("Enable node %d?", st.selectedNode)
	case "disable":
		message = fmt.Sprintf("Disable node %d?", st.selectedNode)
	case "shutdown_all":
		message = "EMERGENCY: Shutdown all nodes and disconnect all users?"
	case "kill_all":
		message = "EMERGENCY: Kill all user sessions immediately?"
	case "restart_all":
		message = "EMERGENCY: Restart all nodes?"
	case "maintenance_mode":
		message = "EMERGENCY: Enter maintenance mode?"
	default:
		message = "Confirm action?"
	}
	
	confirmStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("3")).
		Background(lipgloss.Color("3")).
		Foreground(lipgloss.Color("0")).
		Padding(1).
		Width(st.width - 20).
		Align(lipgloss.Center)
	
	content := fmt.Sprintf("%s\n\n[Y]es / [N]o", message)
	return confirmStyle.Render(content)
}

// renderHelpLine renders the help line
func (st *SysopTools) renderHelpLine() string {
	var help string
	
	if st.confirmDialog {
		help = "Y:Confirm N:Cancel"
	} else if st.isInputMode() {
		help = "Enter:Send ESC:Cancel"
	} else {
		switch st.currentTool {
		case NodeControlTool:
			help = "↑/↓:Select D:Disconnect R:Restart E:Enable X:Disable M:Message C:Command"
		case MessageTool:
			help = "B:Broadcast S:Single A:All C:Clear 1-9:Toggle"
		case MonitorTool:
			help = "↑/↓:Select Enter:Details R:Refresh"
		case LogViewTool:
			help = "↑/↓:Scroll PgUp/PgDn:Page Home/End C:Clear"
		case StatsTool:
			help = "↑/↓:Select Enter:Details R:Refresh"
		case EmergencyTool:
			help = "S:Shutdown K:Kill R:Restart M:Maintenance B:Broadcast"
		}
		help += " TAB:Switch F1-F12:Tools ESC:Exit"
	}
	
	helpStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("0")).
		Foreground(lipgloss.Color("7")).
		Width(st.width)
	
	return helpStyle.Render(help)
}

// formatNodeForGrid formats a node for the grid display
func (st *SysopTools) formatNodeForGrid(node *NodeInfo) string {
	var status string
	var style lipgloss.Style
	
	switch node.Status {
	case NodeStatusAvailable:
		status = fmt.Sprintf("%2d:Wait", node.NodeID)
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	case NodeStatusConnected, NodeStatusLoggedIn:
		if node.User != nil {
			status = fmt.Sprintf("%2d:%-4s", node.NodeID, truncateString(node.User.Handle, 4))
		} else {
			status = fmt.Sprintf("%2d:Conn", node.NodeID)
		}
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	case NodeStatusInMenu, NodeStatusInDoor, NodeStatusInMessage:
		if node.User != nil {
			status = fmt.Sprintf("%2d:%-4s", node.NodeID, truncateString(node.User.Handle, 4))
		} else {
			status = fmt.Sprintf("%2d:Actv", node.NodeID)
		}
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	case NodeStatusDisabled:
		status = fmt.Sprintf("%2d:Off ", node.NodeID)
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	default:
		status = fmt.Sprintf("%2d:Err ", node.NodeID)
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	}
	
	// Fixed width for grid alignment
	gridStyle := style.Width(8).Align(lipgloss.Left)
	return gridStyle.Render(status)
}

// Init implements tea.Model
func (st *SysopTools) Init() tea.Cmd {
	st.addToLog("SysOp Tools initialized")
	if st.currentTool == MonitorTool {
		return st.tick()
	}
	return nil
}