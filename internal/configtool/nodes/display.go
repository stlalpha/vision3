package nodes

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NodeStatusDisplay represents the real-time node monitoring screen
type NodeStatusDisplay struct {
	nodeManager    NodeManager
	width          int
	height         int
	focused        bool
	refreshRate    time.Duration
	lastUpdate     time.Time
	selectedNode   int
	viewMode       DisplayMode
	showDetails    bool
	sortBy         SortMode
	filterStatus   NodeStatus
	alertsVisible  bool
	scrollOffset   int
	maxVisible     int
}

// DisplayMode represents different view modes for the node display
type DisplayMode int

const (
	GridView DisplayMode = iota
	ListViewCompact
	ListViewDetailed
	WhoOnlineView
)

// SortMode represents different sorting options
type SortMode int

const (
	SortByNodeID SortMode = iota
	SortByStatus
	SortByUser
	SortByActivity
	SortByConnectTime
)

// NewNodeStatusDisplay creates a new node status display
func NewNodeStatusDisplay(nodeManager NodeManager, width, height int) *NodeStatusDisplay {
	return &NodeStatusDisplay{
		nodeManager:  nodeManager,
		width:        width,
		height:       height,
		refreshRate:  time.Second,
		viewMode:     GridView,
		sortBy:       SortByNodeID,
		filterStatus: NodeStatusOffline - 1, // Show all statuses
		maxVisible:   height - 8,            // Account for header and border
	}
}

// Update implements tea.Model for the Bubble Tea framework
func (nsd *NodeStatusDisplay) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return nsd.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		nsd.width = msg.Width
		nsd.height = msg.Height
		nsd.maxVisible = nsd.height - 8
	case TickMsg:
		nsd.lastUpdate = time.Now()
		return nsd, nsd.tick()
	}
	return nsd, nil
}

// TickMsg is sent periodically to update the display
type TickMsg time.Time

// tick returns a command to send another tick message
func (nsd *NodeStatusDisplay) tick() tea.Cmd {
	return tea.Tick(nsd.refreshRate, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// handleKeyPress processes keyboard input
func (nsd *NodeStatusDisplay) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return nsd, tea.Quit
	case "r", "f5":
		nsd.lastUpdate = time.Now()
		return nsd, nil
	case "tab":
		nsd.cycleViewMode()
	case "s":
		nsd.cycleSortMode()
	case "f":
		nsd.cycleFilterStatus()
	case "d":
		nsd.showDetails = !nsd.showDetails
	case "a":
		nsd.alertsVisible = !nsd.alertsVisible
	case "up", "k":
		if nsd.selectedNode > 1 {
			nsd.selectedNode--
		}
	case "down", "j":
		nodes := nsd.nodeManager.GetAllNodes()
		if nsd.selectedNode < len(nodes) {
			nsd.selectedNode++
		}
	case "pageup":
		nsd.scrollOffset -= nsd.maxVisible
		if nsd.scrollOffset < 0 {
			nsd.scrollOffset = 0
		}
	case "pagedown":
		nsd.scrollOffset += nsd.maxVisible
	case "home":
		nsd.selectedNode = 1
		nsd.scrollOffset = 0
	case "end":
		nodes := nsd.nodeManager.GetAllNodes()
		nsd.selectedNode = len(nodes)
	case "enter":
		return nsd.handleNodeAction()
	}
	return nsd, nil
}

// cycleViewMode cycles through different display modes
func (nsd *NodeStatusDisplay) cycleViewMode() {
	switch nsd.viewMode {
	case GridView:
		nsd.viewMode = ListViewCompact
	case ListViewCompact:
		nsd.viewMode = ListViewDetailed
	case ListViewDetailed:
		nsd.viewMode = WhoOnlineView
	case WhoOnlineView:
		nsd.viewMode = GridView
	}
}

// cycleSortMode cycles through different sorting modes
func (nsd *NodeStatusDisplay) cycleSortMode() {
	switch nsd.sortBy {
	case SortByNodeID:
		nsd.sortBy = SortByStatus
	case SortByStatus:
		nsd.sortBy = SortByUser
	case SortByUser:
		nsd.sortBy = SortByActivity
	case SortByActivity:
		nsd.sortBy = SortByConnectTime
	case SortByConnectTime:
		nsd.sortBy = SortByNodeID
	}
}

// cycleFilterStatus cycles through status filter options
func (nsd *NodeStatusDisplay) cycleFilterStatus() {
	switch nsd.filterStatus {
	case NodeStatusOffline - 1: // Show all
		nsd.filterStatus = NodeStatusAvailable
	case NodeStatusAvailable:
		nsd.filterStatus = NodeStatusConnected
	case NodeStatusConnected:
		nsd.filterStatus = NodeStatusLoggedIn
	case NodeStatusLoggedIn:
		nsd.filterStatus = NodeStatusOffline - 1 // Back to show all
	default:
		nsd.filterStatus = NodeStatusOffline - 1
	}
}

// handleNodeAction handles action on selected node
func (nsd *NodeStatusDisplay) handleNodeAction() (tea.Model, tea.Cmd) {
	// This could open a detailed node management dialog
	// For now, just toggle details view
	nsd.showDetails = !nsd.showDetails
	return nsd, nil
}

// View renders the node status display
func (nsd *NodeStatusDisplay) View() string {
	var sections []string

	// Title bar
	sections = append(sections, nsd.renderTitleBar())

	// Main content based on view mode
	switch nsd.viewMode {
	case GridView:
		sections = append(sections, nsd.renderGridView())
	case ListViewCompact:
		sections = append(sections, nsd.renderListViewCompact())
	case ListViewDetailed:
		sections = append(sections, nsd.renderListViewDetailed())
	case WhoOnlineView:
		sections = append(sections, nsd.renderWhoOnlineView())
	}

	// Alerts panel if visible
	if nsd.alertsVisible {
		sections = append(sections, nsd.renderAlertsPanel())
	}

	// Status bar
	sections = append(sections, nsd.renderStatusBar())

	// Help line
	sections = append(sections, nsd.renderHelpLine())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderTitleBar renders the title bar with system status
func (nsd *NodeStatusDisplay) renderTitleBar() string {
	systemStatus, _ := nsd.nodeManager.GetSystemStatus()
	
	title := "ViSiON/3 Node Monitor"
	status := fmt.Sprintf("Nodes: %d/%d | Users: %d | Load: %.1f",
		systemStatus.ActiveNodes,
		systemStatus.TotalNodes,
		systemStatus.ConnectedUsers,
		systemStatus.SystemLoad)
	
	timestamp := time.Now().Format("15:04:05")
	
	titleStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("4")).     // Blue background
		Foreground(lipgloss.Color("15")).    // White text
		Bold(true).
		Padding(0, 1).
		Width(nsd.width)
	
	titleBar := fmt.Sprintf("%-30s %*s %10s", title, nsd.width-42, status, timestamp)
	return titleStyle.Render(titleBar)
}

// renderGridView renders nodes in a grid layout
func (nsd *NodeStatusDisplay) renderGridView() string {
	nodes := nsd.getFilteredAndSortedNodes()
	
	var rows []string
	cols := 4 // Number of columns in grid
	nodeWidth := (nsd.width - 10) / cols
	
	for i := 0; i < len(nodes); i += cols {
		var row []string
		for j := 0; j < cols && i+j < len(nodes); j++ {
			node := nodes[i+j]
			row = append(row, nsd.renderNodeCard(node, nodeWidth))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderNodeCard renders a single node as a card
func (nsd *NodeStatusDisplay) renderNodeCard(node *NodeInfo, width int) string {
	var bgColor, fgColor lipgloss.Color
	
	// Color coding based on status
	switch node.Status {
	case NodeStatusAvailable:
		bgColor, fgColor = "2", "0"  // Green
	case NodeStatusConnected, NodeStatusLoggedIn:
		bgColor, fgColor = "3", "0"  // Yellow
	case NodeStatusInMenu, NodeStatusInDoor:
		bgColor, fgColor = "6", "0"  // Cyan
	case NodeStatusInChat:
		bgColor, fgColor = "5", "15" // Magenta
	case NodeStatusOffline, NodeStatusDisabled:
		bgColor, fgColor = "8", "15" // Gray
	default:
		bgColor, fgColor = "1", "15" // Red
	}
	
	cardStyle := lipgloss.NewStyle().
		Background(bgColor).
		Foreground(fgColor).
		Border(lipgloss.RoundedBorder()).
		Width(width).
		Height(6).
		Padding(0, 1)
	
	var content []string
	content = append(content, fmt.Sprintf("Node %d", node.NodeID))
	content = append(content, node.Status.String())
	
	if node.User != nil {
		content = append(content, node.User.Handle)
		content = append(content, node.Activity.Description)
	} else {
		content = append(content, "Available")
		content = append(content, "")
	}
	
	if !node.ConnectTime.IsZero() {
		duration := time.Since(node.ConnectTime)
		content = append(content, fmt.Sprintf("%02d:%02d", 
			int(duration.Minutes()), int(duration.Seconds())%60))
	}
	
	return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, content...))
}

// renderListViewCompact renders nodes in compact list format
func (nsd *NodeStatusDisplay) renderListViewCompact() string {
	nodes := nsd.getFilteredAndSortedNodes()
	
	// Header
	header := fmt.Sprintf("%-4s %-12s %-15s %-20s %-8s %-8s",
		"Node", "Status", "User", "Activity", "Time", "Idle")
	
	headerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("7")).
		Foreground(lipgloss.Color("0")).
		Bold(true)
	
	var rows []string
	rows = append(rows, headerStyle.Render(header))
	rows = append(rows, strings.Repeat("─", nsd.width))
	
	// Node rows
	startIdx := nsd.scrollOffset
	endIdx := startIdx + nsd.maxVisible
	if endIdx > len(nodes) {
		endIdx = len(nodes)
	}
	
	for i := startIdx; i < endIdx; i++ {
		node := nodes[i]
		row := nsd.formatNodeRow(node, false)
		
		// Highlight selected row
		if node.NodeID == nsd.selectedNode {
			rowStyle := lipgloss.NewStyle().Background(lipgloss.Color("4"))
			row = rowStyle.Render(row)
		}
		
		rows = append(rows, row)
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderListViewDetailed renders nodes with detailed information
func (nsd *NodeStatusDisplay) renderListViewDetailed() string {
	nodes := nsd.getFilteredAndSortedNodes()
	
	var rows []string
	
	startIdx := nsd.scrollOffset
	endIdx := startIdx + nsd.maxVisible/3 // Each entry takes 3 lines
	if endIdx > len(nodes) {
		endIdx = len(nodes)
	}
	
	for i := startIdx; i < endIdx; i++ {
		node := nodes[i]
		rows = append(rows, nsd.renderDetailedNodeEntry(node))
		if i < endIdx-1 {
			rows = append(rows, strings.Repeat("─", nsd.width))
		}
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderDetailedNodeEntry renders a detailed entry for a single node
func (nsd *NodeStatusDisplay) renderDetailedNodeEntry(node *NodeInfo) string {
	var lines []string
	
	// First line: Node info
	line1 := fmt.Sprintf("Node %d (%s) - %s", 
		node.NodeID, node.Config.Name, node.Status.String())
	
	if node.NodeID == nsd.selectedNode {
		style := lipgloss.NewStyle().Background(lipgloss.Color("4")).Bold(true)
		line1 = style.Render(line1)
	}
	lines = append(lines, line1)
	
	// Second line: User and activity
	if node.User != nil {
		line2 := fmt.Sprintf("  User: %s (%s) | Activity: %s",
			node.User.Handle, node.User.GroupLocation, node.Activity.Description)
		lines = append(lines, line2)
	} else {
		lines = append(lines, "  No user connected")
	}
	
	// Third line: Connection info and metrics
	var connInfo []string
	if !node.ConnectTime.IsZero() {
		duration := time.Since(node.ConnectTime)
		connInfo = append(connInfo, fmt.Sprintf("Online: %s", formatDuration(duration)))
	}
	if node.IdleTime > 0 {
		connInfo = append(connInfo, fmt.Sprintf("Idle: %s", formatDuration(node.IdleTime)))
	}
	if node.RemoteAddr != nil {
		connInfo = append(connInfo, fmt.Sprintf("From: %s", node.RemoteAddr.String()))
	}
	
	if len(connInfo) > 0 {
		line3 := "  " + strings.Join(connInfo, " | ")
		lines = append(lines, line3)
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderWhoOnlineView renders the classic BBS "Who's Online" display
func (nsd *NodeStatusDisplay) renderWhoOnlineView() string {
	nodes := nsd.getFilteredAndSortedNodes()
	
	// Classic BBS style header
	var lines []string
	lines = append(lines, "")
	lines = append(lines, "                        ViSiON/3 - Who's Online")
	lines = append(lines, "")
	lines = append(lines, "Node User Handle      Location          Activity              Time  Idle")
	lines = append(lines, "---- --------------- ----------------- --------------------- ----- -----")
	
	// Filter to only show nodes with users
	var onlineEntries []WhoOnlineEntry
	for _, node := range nodes {
		if node.User != nil {
			entry := WhoOnlineEntry{
				NodeID:       node.NodeID,
				UserHandle:   node.User.Handle,
				UserLocation: node.User.GroupLocation,
				Activity:     node.Activity.Description,
				OnlineTime:   time.Since(node.ConnectTime),
				IdleTime:     node.IdleTime,
				BaudRate:     "33600", // Classic BBS speed
				Status:       node.Status.String(),
			}
			onlineEntries = append(onlineEntries, entry)
		}
	}
	
	if len(onlineEntries) == 0 {
		lines = append(lines, "")
		lines = append(lines, "                           No users online")
		lines = append(lines, "")
	} else {
		for _, entry := range onlineEntries {
			line := fmt.Sprintf("%4d %-15s %-17s %-21s %5s %5s",
				entry.NodeID,
				truncateString(entry.UserHandle, 15),
				truncateString(entry.UserLocation, 17),
				truncateString(entry.Activity, 21),
				formatDuration(entry.OnlineTime),
				formatDuration(entry.IdleTime))
			lines = append(lines, line)
		}
	}
	
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("                    %d user(s) online on %d node(s)",
		len(onlineEntries), len(nodes)))
	
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderAlertsPanel renders the alerts panel
func (nsd *NodeStatusDisplay) renderAlertsPanel() string {
	alerts := nsd.nodeManager.GetAlerts()
	
	var lines []string
	lines = append(lines, "")
	lines = append(lines, "Recent Alerts:")
	lines = append(lines, strings.Repeat("─", nsd.width))
	
	// Show last 5 alerts
	maxAlerts := 5
	if len(alerts) > maxAlerts {
		alerts = alerts[len(alerts)-maxAlerts:]
	}
	
	for _, alert := range alerts {
		var icon string
		switch alert.AlertType {
		case "error":
			icon = "✗"
		case "warning":
			icon = "⚠"
		default:
			icon = "ℹ"
		}
		
		timestamp := alert.Timestamp.Format("15:04:05")
		line := fmt.Sprintf("%s [%s] Node %d: %s",
			icon, timestamp, alert.NodeID, alert.Message)
		
		if len(line) > nsd.width {
			line = line[:nsd.width-3] + "..."
		}
		
		lines = append(lines, line)
	}
	
	if len(alerts) == 0 {
		lines = append(lines, "No recent alerts")
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderStatusBar renders the status bar with current view info
func (nsd *NodeStatusDisplay) renderStatusBar() string {
	var mode string
	switch nsd.viewMode {
	case GridView:
		mode = "Grid"
	case ListViewCompact:
		mode = "List"
	case ListViewDetailed:
		mode = "Detail"
	case WhoOnlineView:
		mode = "Who's Online"
	}
	
	var sort string
	switch nsd.sortBy {
	case SortByNodeID:
		sort = "Node"
	case SortByStatus:
		sort = "Status"
	case SortByUser:
		sort = "User"
	case SortByActivity:
		sort = "Activity"
	case SortByConnectTime:
		sort = "Time"
	}
	
	var filter string
	if nsd.filterStatus == NodeStatusOffline-1 {
		filter = "All"
	} else {
		filter = nsd.filterStatus.String()
	}
	
	status := fmt.Sprintf("View: %s | Sort: %s | Filter: %s | Node: %d",
		mode, sort, filter, nsd.selectedNode)
	
	statusStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("8")).
		Foreground(lipgloss.Color("15")).
		Width(nsd.width)
	
	return statusStyle.Render(status)
}

// renderHelpLine renders the help line with key commands
func (nsd *NodeStatusDisplay) renderHelpLine() string {
	help := "TAB:View S:Sort F:Filter D:Details A:Alerts R:Refresh ESC:Exit"
	
	helpStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("0")).
		Foreground(lipgloss.Color("7")).
		Width(nsd.width)
	
	return helpStyle.Render(help)
}

// Helper methods

// getFilteredAndSortedNodes returns nodes filtered and sorted according to current settings
func (nsd *NodeStatusDisplay) getFilteredAndSortedNodes() []*NodeInfo {
	allNodes := nsd.nodeManager.GetAllNodes()
	
	// Filter by status
	var filtered []*NodeInfo
	for _, node := range allNodes {
		if nsd.filterStatus == NodeStatusOffline-1 || node.Status == nsd.filterStatus {
			filtered = append(filtered, node)
		}
	}
	
	// Sort
	sort.Slice(filtered, func(i, j int) bool {
		switch nsd.sortBy {
		case SortByNodeID:
			return filtered[i].NodeID < filtered[j].NodeID
		case SortByStatus:
			return filtered[i].Status < filtered[j].Status
		case SortByUser:
			userI := ""
			if filtered[i].User != nil {
				userI = filtered[i].User.Handle
			}
			userJ := ""
			if filtered[j].User != nil {
				userJ = filtered[j].User.Handle
			}
			return userI < userJ
		case SortByActivity:
			return filtered[i].Activity.Description < filtered[j].Activity.Description
		case SortByConnectTime:
			return filtered[i].ConnectTime.Before(filtered[j].ConnectTime)
		default:
			return filtered[i].NodeID < filtered[j].NodeID
		}
	})
	
	return filtered
}

// formatNodeRow formats a single node as a table row
func (nsd *NodeStatusDisplay) formatNodeRow(node *NodeInfo, detailed bool) string {
	user := "Available"
	activity := "Waiting"
	connTime := ""
	idle := ""
	
	if node.User != nil {
		user = node.User.Handle
		activity = node.Activity.Description
		
		if !node.ConnectTime.IsZero() {
			connTime = formatDuration(time.Since(node.ConnectTime))
		}
		if node.IdleTime > 0 {
			idle = formatDuration(node.IdleTime)
		}
	}
	
	return fmt.Sprintf("%-4d %-12s %-15s %-20s %-8s %-8s",
		node.NodeID,
		truncateString(node.Status.String(), 12),
		truncateString(user, 15),
		truncateString(activity, 20),
		connTime,
		idle)
}

// formatDuration formats a duration in MM:SS format
func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// truncateString truncates a string to the specified length
func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	if length <= 3 {
		return s[:length]
	}
	return s[:length-3] + "..."
}

// Init implements tea.Model
func (nsd *NodeStatusDisplay) Init() tea.Cmd {
	return nsd.tick()
}