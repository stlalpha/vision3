package nodes

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// WhoOnlineDisplay represents the classic "Who's Online" display
type WhoOnlineDisplay struct {
	nodeManager    NodeManager
	width          int
	height         int
	focused        bool
	displayStyle   WhoDisplayStyle
	sortMode       WhoSortMode
	showDetails    bool
	refreshRate    time.Duration
	lastUpdate     time.Time
	selectedEntry  int
	scrollOffset   int
	maxVisible     int
	filterMode     WhoFilterMode
	autoRefresh    bool
	showStatistics bool
	animationFrame int
	colorScheme    WhoColorScheme
}

// WhoDisplayStyle represents different display styles
type WhoDisplayStyle int

const (
	ClassicBBSStyle WhoDisplayStyle = iota
	CompactStyle
	DetailedStyle
	TableStyle
	GraphicalStyle
)

// WhoSortMode represents sorting options
type WhoSortMode int

const (
	SortByNode WhoSortMode = iota
	SortByHandle
	SortByLocation
	SortByOnlineTime
	SortByWhoActivity
	SortByIdleTime
)

// WhoFilterMode represents filtering options
type WhoFilterMode int

const (
	ShowAll WhoFilterMode = iota
	ShowChatting
	ShowIdle
	ShowActive
	ShowLocal
	ShowRemote
)

// WhoColorScheme represents color schemes
type WhoColorScheme int

const (
	ClassicColors WhoColorScheme = iota
	ModernColors
	MonochromeColors
	HighContrastColors
)

// NewWhoOnlineDisplay creates a new Who's Online display
func NewWhoOnlineDisplay(nodeManager NodeManager, width, height int) *WhoOnlineDisplay {
	return &WhoOnlineDisplay{
		nodeManager:   nodeManager,
		width:         width,
		height:        height,
		displayStyle:  ClassicBBSStyle,
		sortMode:      SortByNode,
		refreshRate:   time.Second * 3,
		maxVisible:    height - 10,
		filterMode:    ShowAll,
		autoRefresh:   true,
		colorScheme:   ClassicColors,
	}
}

// Update implements tea.Model
func (wod *WhoOnlineDisplay) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return wod.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		wod.width = msg.Width
		wod.height = msg.Height
		wod.maxVisible = wod.height - 10
	case TickMsg:
		wod.lastUpdate = time.Now()
		wod.animationFrame = (wod.animationFrame + 1) % 4
		if wod.autoRefresh {
			return wod, wod.tick()
		}
	}
	return wod, nil
}

// tick returns a command for periodic updates
func (wod *WhoOnlineDisplay) tick() tea.Cmd {
	return tea.Tick(wod.refreshRate, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// handleKeyPress processes keyboard input
func (wod *WhoOnlineDisplay) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return wod, tea.Quit
	case "r", "f5":
		wod.lastUpdate = time.Now()
		return wod, nil
	case "s":
		wod.cycleSortMode()
	case "f":
		wod.cycleFilterMode()
	case "d":
		wod.showDetails = !wod.showDetails
	case "tab", "space":
		wod.cycleDisplayStyle()
	case "c":
		wod.cycleColorScheme()
	case "t":
		wod.showStatistics = !wod.showStatistics
	case "a":
		wod.autoRefresh = !wod.autoRefresh
		if wod.autoRefresh {
			return wod, wod.tick()
		}
	case "up", "k":
		if wod.selectedEntry > 0 {
			wod.selectedEntry--
		}
	case "down", "j":
		entries := wod.getWhoOnlineEntries()
		if wod.selectedEntry < len(entries)-1 {
			wod.selectedEntry++
		}
	case "pageup":
		wod.scrollOffset -= wod.maxVisible
		if wod.scrollOffset < 0 {
			wod.scrollOffset = 0
		}
	case "pagedown":
		wod.scrollOffset += wod.maxVisible
	case "home":
		wod.selectedEntry = 0
		wod.scrollOffset = 0
	case "end":
		entries := wod.getWhoOnlineEntries()
		wod.selectedEntry = len(entries) - 1
	case "enter":
		return wod.handleEntryAction()
	}
	return wod, nil
}

// cycleSortMode cycles through sort modes
func (wod *WhoOnlineDisplay) cycleSortMode() {
	switch wod.sortMode {
	case SortByNode:
		wod.sortMode = SortByHandle
	case SortByHandle:
		wod.sortMode = SortByLocation
	case SortByLocation:
		wod.sortMode = SortByOnlineTime
	case SortByOnlineTime:
		wod.sortMode = SortByWhoActivity
	case SortByWhoActivity:
		wod.sortMode = SortByIdleTime
	case SortByIdleTime:
		wod.sortMode = SortByNode
	}
}

// cycleFilterMode cycles through filter modes
func (wod *WhoOnlineDisplay) cycleFilterMode() {
	switch wod.filterMode {
	case ShowAll:
		wod.filterMode = ShowChatting
	case ShowChatting:
		wod.filterMode = ShowIdle
	case ShowIdle:
		wod.filterMode = ShowActive
	case ShowActive:
		wod.filterMode = ShowLocal
	case ShowLocal:
		wod.filterMode = ShowRemote
	case ShowRemote:
		wod.filterMode = ShowAll
	}
}

// cycleDisplayStyle cycles through display styles
func (wod *WhoOnlineDisplay) cycleDisplayStyle() {
	switch wod.displayStyle {
	case ClassicBBSStyle:
		wod.displayStyle = CompactStyle
	case CompactStyle:
		wod.displayStyle = DetailedStyle
	case DetailedStyle:
		wod.displayStyle = TableStyle
	case TableStyle:
		wod.displayStyle = GraphicalStyle
	case GraphicalStyle:
		wod.displayStyle = ClassicBBSStyle
	}
}

// cycleColorScheme cycles through color schemes
func (wod *WhoOnlineDisplay) cycleColorScheme() {
	switch wod.colorScheme {
	case ClassicColors:
		wod.colorScheme = ModernColors
	case ModernColors:
		wod.colorScheme = MonochromeColors
	case MonochromeColors:
		wod.colorScheme = HighContrastColors
	case HighContrastColors:
		wod.colorScheme = ClassicColors
	}
}

// handleEntryAction handles action on selected entry
func (wod *WhoOnlineDisplay) handleEntryAction() (tea.Model, tea.Cmd) {
	entries := wod.getWhoOnlineEntries()
	if wod.selectedEntry < len(entries) {
		// This could open a detailed info dialog or action menu
		wod.showDetails = !wod.showDetails
	}
	return wod, nil
}

// getWhoOnlineEntries returns the current online entries
func (wod *WhoOnlineDisplay) getWhoOnlineEntries() []WhoOnlineEntry {
	nodes := wod.nodeManager.GetActiveNodes()
	var entries []WhoOnlineEntry
	
	for _, node := range nodes {
		if node.User != nil {
			// Calculate online time
			onlineTime := time.Since(node.ConnectTime)
			if node.ConnectTime.IsZero() {
				onlineTime = 0
			}
			
			// Determine activity string
			activity := node.Activity.Description
			if activity == "" {
				activity = "Unknown"
			}
			
			// Determine baud rate (classic BBS style)
			baudRate := "33600"
			if node.Config.LocalNode {
				baudRate = "Local"
			}
			
			// Determine status
			status := "Active"
			if node.IdleTime > 5*time.Minute {
				status = "Idle"
			}
			if node.Status == NodeStatusInChat {
				status = "Chat"
			}
			
			entry := WhoOnlineEntry{
				NodeID:       node.NodeID,
				UserHandle:   node.User.Handle,
				UserLocation: node.User.GroupLocation,
				Activity:     activity,
				OnlineTime:   onlineTime,
				IdleTime:     node.IdleTime,
				BaudRate:     baudRate,
				Status:       status,
			}
			
			// Apply filter
			if wod.shouldShowEntry(entry, node) {
				entries = append(entries, entry)
			}
		}
	}
	
	// Sort entries
	wod.sortEntries(entries)
	
	return entries
}

// shouldShowEntry checks if an entry should be shown based on current filter
func (wod *WhoOnlineDisplay) shouldShowEntry(entry WhoOnlineEntry, node *NodeInfo) bool {
	switch wod.filterMode {
	case ShowAll:
		return true
	case ShowChatting:
		return entry.Status == "Chat" || node.Status == NodeStatusInChat
	case ShowIdle:
		return entry.IdleTime > 5*time.Minute
	case ShowActive:
		return entry.IdleTime <= 5*time.Minute && entry.Status != "Chat"
	case ShowLocal:
		return node.Config.LocalNode
	case ShowRemote:
		return !node.Config.LocalNode
	default:
		return true
	}
}

// sortEntries sorts the entries based on current sort mode
func (wod *WhoOnlineDisplay) sortEntries(entries []WhoOnlineEntry) {
	sort.Slice(entries, func(i, j int) bool {
		switch wod.sortMode {
		case SortByNode:
			return entries[i].NodeID < entries[j].NodeID
		case SortByHandle:
			return strings.ToLower(entries[i].UserHandle) < strings.ToLower(entries[j].UserHandle)
		case SortByLocation:
			return strings.ToLower(entries[i].UserLocation) < strings.ToLower(entries[j].UserLocation)
		case SortByOnlineTime:
			return entries[i].OnlineTime > entries[j].OnlineTime // Descending
		case SortByWhoActivity:
			return strings.ToLower(entries[i].Activity) < strings.ToLower(entries[j].Activity)
		case SortByIdleTime:
			return entries[i].IdleTime > entries[j].IdleTime // Descending
		default:
			return entries[i].NodeID < entries[j].NodeID
		}
	})
}

// View renders the Who's Online display
func (wod *WhoOnlineDisplay) View() string {
	var sections []string
	
	// Title and header
	sections = append(sections, wod.renderHeader())
	
	// Main content based on display style
	switch wod.displayStyle {
	case ClassicBBSStyle:
		sections = append(sections, wod.renderClassicBBSStyle())
	case CompactStyle:
		sections = append(sections, wod.renderCompactStyle())
	case DetailedStyle:
		sections = append(sections, wod.renderDetailedStyle())
	case TableStyle:
		sections = append(sections, wod.renderTableStyle())
	case GraphicalStyle:
		sections = append(sections, wod.renderGraphicalStyle())
	}
	
	// Statistics if enabled
	if wod.showStatistics {
		sections = append(sections, wod.renderStatistics())
	}
	
	// Status bar
	sections = append(sections, wod.renderStatusBar())
	
	// Help line
	sections = append(sections, wod.renderHelpLine())
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the header with BBS-style title
func (wod *WhoOnlineDisplay) renderHeader() string {
	var lines []string
	
	// Classic BBS-style header
	lines = append(lines, "")
	lines = append(lines, wod.getColoredText("                        ViSiON/3 Multi-Node BBS", "title"))
	lines = append(lines, wod.getColoredText("                             Who's Online", "subtitle"))
	lines = append(lines, "")
	
	// Add some BBS-style decoration
	decorLine := strings.Repeat("═", wod.width-4)
	if wod.width > 80 {
		decorLine = decorLine[:76] // Limit for classic look
	}
	lines = append(lines, wod.getColoredText(decorLine, "border"))
	
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderClassicBBSStyle renders in classic BBS style
func (wod *WhoOnlineDisplay) renderClassicBBSStyle() string {
	entries := wod.getWhoOnlineEntries()
	
	var lines []string
	lines = append(lines, "")
	
	// Classic header
	headerLine := fmt.Sprintf("%-4s %-15s %-17s %-20s %-8s %-5s %-8s",
		"Node", "Handle", "Location", "Activity", "Online", "Idle", "Baud")
	lines = append(lines, wod.getColoredText(headerLine, "header"))
	
	separatorLine := fmt.Sprintf("%-4s %-15s %-17s %-20s %-8s %-5s %-8s",
		"----", "---------------", "-----------------", "--------------------", 
		"--------", "-----", "--------")
	lines = append(lines, wod.getColoredText(separatorLine, "separator"))
	
	if len(entries) == 0 {
		lines = append(lines, "")
		lines = append(lines, wod.getColoredText("                           No users currently online", "info"))
		lines = append(lines, "")
	} else {
		// Show entries with paging
		startIdx := wod.scrollOffset
		endIdx := startIdx + wod.maxVisible
		if endIdx > len(entries) {
			endIdx = len(entries)
		}
		
		for i := startIdx; i < endIdx; i++ {
			entry := entries[i]
			line := wod.formatClassicEntry(entry)
			
			// Highlight selected entry
			if i == wod.selectedEntry {
				line = wod.getStyledText(line, "selected")
			}
			
			lines = append(lines, line)
		}
	}
	
	lines = append(lines, "")
	
	// Footer with total count
	if len(entries) > 0 {
		totalLine := fmt.Sprintf("                    %d user(s) online on %d node(s)",
			len(entries), wod.getTotalActiveNodes())
		lines = append(lines, wod.getColoredText(totalLine, "footer"))
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderCompactStyle renders in compact style
func (wod *WhoOnlineDisplay) renderCompactStyle() string {
	entries := wod.getWhoOnlineEntries()
	
	var lines []string
	
	if len(entries) == 0 {
		lines = append(lines, "No users online")
	} else {
		// Show entries in compact format
		for i, entry := range entries {
			onlineStr := formatDuration(entry.OnlineTime)
			line := fmt.Sprintf("%d:%s@%s (%s) %s",
				entry.NodeID, entry.UserHandle, entry.UserLocation, onlineStr, entry.Activity)
			
			if len(line) > wod.width-4 {
				line = line[:wod.width-7] + "..."
			}
			
			// Highlight selected entry
			if i == wod.selectedEntry {
				line = wod.getStyledText(line, "selected")
			}
			
			lines = append(lines, line)
		}
	}
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(wod.width - 4)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderDetailedStyle renders in detailed style
func (wod *WhoOnlineDisplay) renderDetailedStyle() string {
	entries := wod.getWhoOnlineEntries()
	
	var lines []string
	
	if len(entries) == 0 {
		lines = append(lines, "No users currently online")
	} else {
		for i, entry := range entries {
			// Multi-line entry with details
			line1 := fmt.Sprintf("Node %d: %s", entry.NodeID, entry.UserHandle)
			line2 := fmt.Sprintf("  Location: %s", entry.UserLocation)
			line3 := fmt.Sprintf("  Activity: %s", entry.Activity)
			line4 := fmt.Sprintf("  Online: %s | Idle: %s | Connection: %s",
				formatDuration(entry.OnlineTime),
				formatDuration(entry.IdleTime),
				entry.BaudRate)
			
			if i == wod.selectedEntry {
				style := wod.getSelectionStyle()
				line1 = style.Render(line1)
				line2 = style.Render(line2)
				line3 = style.Render(line3)
				line4 = style.Render(line4)
			}
			
			lines = append(lines, line1, line2, line3, line4)
			
			if i < len(entries)-1 {
				lines = append(lines, strings.Repeat("─", wod.width-8))
			}
		}
	}
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(wod.width - 4)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderTableStyle renders in table style
func (wod *WhoOnlineDisplay) renderTableStyle() string {
	entries := wod.getWhoOnlineEntries()
	
	// Create table with borders
	var rows []string
	
	// Header
	header := fmt.Sprintf("│ %-4s │ %-15s │ %-17s │ %-20s │ %-8s │ %-5s │",
		"Node", "Handle", "Location", "Activity", "Online", "Idle")
	separator := strings.Repeat("─", len(header))
	
	rows = append(rows, "┌"+separator[1:len(separator)-1]+"┐")
	rows = append(rows, header)
	rows = append(rows, "├"+strings.Repeat("─", len(header)-2)+"┤")
	
	if len(entries) == 0 {
		emptyRow := fmt.Sprintf("│ %-*s │", len(header)-4, "No users online")
		rows = append(rows, emptyRow)
	} else {
		for i, entry := range entries {
			row := fmt.Sprintf("│ %-4d │ %-15s │ %-17s │ %-20s │ %-8s │ %-5s │",
				entry.NodeID,
				truncateString(entry.UserHandle, 15),
				truncateString(entry.UserLocation, 17),
				truncateString(entry.Activity, 20),
				formatDuration(entry.OnlineTime),
				formatDuration(entry.IdleTime))
			
			if i == wod.selectedEntry {
				row = wod.getStyledText(row, "selected")
			}
			
			rows = append(rows, row)
		}
	}
	
	rows = append(rows, "└"+strings.Repeat("─", len(header)-2)+"┘")
	
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderGraphicalStyle renders with graphical elements
func (wod *WhoOnlineDisplay) renderGraphicalStyle() string {
	entries := wod.getWhoOnlineEntries()
	
	var lines []string
	
	// ASCII art header
	lines = append(lines, "    ┌─────────────────────────────────┐")
	lines = append(lines, "    │         ONLINE USERS            │")
	lines = append(lines, "    └─────────────────────────────────┘")
	lines = append(lines, "")
	
	if len(entries) == 0 {
		lines = append(lines, "    📴 No users currently online")
	} else {
		for i, entry := range entries {
			// Use Unicode characters for visual appeal
			var statusIcon string
			switch entry.Status {
			case "Active":
				statusIcon = "🟢"
			case "Idle":
				statusIcon = "🟡"
			case "Chat":
				statusIcon = "💬"
			default:
				statusIcon = "🔵"
			}
			
			// Add animation for active users
			if entry.Status == "Active" && wod.autoRefresh {
				icons := []string{"🟢", "🔆", "✨", "⭐"}
				statusIcon = icons[wod.animationFrame]
			}
			
			line := fmt.Sprintf("    %s Node %d: %s (%s)",
				statusIcon, entry.NodeID, entry.UserHandle, entry.UserLocation)
			
			if entry.Activity != "Unknown" {
				line += fmt.Sprintf(" - %s", entry.Activity)
			}
			
			if i == wod.selectedEntry {
				line = wod.getStyledText(line, "selected")
			}
			
			lines = append(lines, line)
			
			// Add time info as sub-line
			timeLine := fmt.Sprintf("       ⏱ Online: %s", formatDuration(entry.OnlineTime))
			if entry.IdleTime > 0 {
				timeLine += fmt.Sprintf(" | Idle: %s", formatDuration(entry.IdleTime))
			}
			lines = append(lines, wod.getColoredText(timeLine, "info"))
		}
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderStatistics renders statistics panel
func (wod *WhoOnlineDisplay) renderStatistics() string {
	systemStatus, _ := wod.nodeManager.GetSystemStatus()
	entries := wod.getWhoOnlineEntries()
	
	var lines []string
	lines = append(lines, "Statistics:")
	lines = append(lines, fmt.Sprintf("  Total Nodes: %d", systemStatus.TotalNodes))
	lines = append(lines, fmt.Sprintf("  Active Nodes: %d", systemStatus.ActiveNodes))
	lines = append(lines, fmt.Sprintf("  Users Online: %d", len(entries)))
	
	// Calculate additional stats
	var activeUsers, idleUsers, chattingUsers int
	for _, entry := range entries {
		switch entry.Status {
		case "Active":
			activeUsers++
		case "Idle":
			idleUsers++
		case "Chat":
			chattingUsers++
		}
	}
	
	lines = append(lines, fmt.Sprintf("  Active: %d | Idle: %d | Chatting: %d", 
		activeUsers, idleUsers, chattingUsers))
	
	if len(entries) > 0 {
		// Calculate average online time
		var totalTime time.Duration
		for _, entry := range entries {
			totalTime += entry.OnlineTime
		}
		avgTime := totalTime / time.Duration(len(entries))
		lines = append(lines, fmt.Sprintf("  Average Online Time: %s", formatDuration(avgTime)))
	}
	
	lines = append(lines, fmt.Sprintf("  Last Update: %s", wod.lastUpdate.Format("15:04:05")))
	
	statsStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(wod.width - 4)
	
	return statsStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderStatusBar renders the status bar
func (wod *WhoOnlineDisplay) renderStatusBar() string {
	var mode, sort, filter string
	
	switch wod.displayStyle {
	case ClassicBBSStyle:
		mode = "Classic"
	case CompactStyle:
		mode = "Compact"
	case DetailedStyle:
		mode = "Detailed"
	case TableStyle:
		mode = "Table"
	case GraphicalStyle:
		mode = "Graphical"
	}
	
	switch wod.sortMode {
	case SortByNode:
		sort = "Node"
	case SortByHandle:
		sort = "Handle"
	case SortByLocation:
		sort = "Location"
	case SortByOnlineTime:
		sort = "Time"
	case SortByWhoActivity:
		sort = "Activity"
	case SortByIdleTime:
		sort = "Idle"
	}
	
	switch wod.filterMode {
	case ShowAll:
		filter = "All"
	case ShowChatting:
		filter = "Chat"
	case ShowIdle:
		filter = "Idle"
	case ShowActive:
		filter = "Active"
	case ShowLocal:
		filter = "Local"
	case ShowRemote:
		filter = "Remote"
	}
	
	refreshStatus := "Manual"
	if wod.autoRefresh {
		refreshStatus = "Auto"
	}
	
	status := fmt.Sprintf("Style: %s | Sort: %s | Filter: %s | Refresh: %s",
		mode, sort, filter, refreshStatus)
	
	statusStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("8")).
		Foreground(lipgloss.Color("15")).
		Width(wod.width)
	
	return statusStyle.Render(status)
}

// renderHelpLine renders the help line
func (wod *WhoOnlineDisplay) renderHelpLine() string {
	help := "TAB:Style S:Sort F:Filter D:Details T:Stats A:Auto R:Refresh C:Colors ESC:Exit"
	
	helpStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("0")).
		Foreground(lipgloss.Color("7")).
		Width(wod.width)
	
	return helpStyle.Render(help)
}

// Helper methods

// formatClassicEntry formats an entry in classic BBS style
func (wod *WhoOnlineDisplay) formatClassicEntry(entry WhoOnlineEntry) string {
	return fmt.Sprintf("%-4d %-15s %-17s %-20s %-8s %-5s %-8s",
		entry.NodeID,
		truncateString(entry.UserHandle, 15),
		truncateString(entry.UserLocation, 17),
		truncateString(entry.Activity, 20),
		formatDuration(entry.OnlineTime),
		formatDuration(entry.IdleTime),
		entry.BaudRate)
}

// getTotalActiveNodes returns the total number of active nodes
func (wod *WhoOnlineDisplay) getTotalActiveNodes() int {
	systemStatus, _ := wod.nodeManager.GetSystemStatus()
	return systemStatus.ActiveNodes
}

// getColoredText returns text with color based on current color scheme
func (wod *WhoOnlineDisplay) getColoredText(text, textType string) string {
	var color string
	
	switch wod.colorScheme {
	case ClassicColors:
		switch textType {
		case "title":
			color = "14" // Bright yellow
		case "subtitle":
			color = "11" // Bright cyan
		case "header":
			color = "15" // Bright white
		case "separator":
			color = "8"  // Gray
		case "info":
			color = "7"  // Light gray
		case "footer":
			color = "10" // Bright green
		case "border":
			color = "9"  // Bright red
		default:
			color = "15"
		}
	case ModernColors:
		switch textType {
		case "title":
			color = "12" // Bright blue
		case "subtitle":
			color = "13" // Bright magenta
		case "header":
			color = "15"
		case "separator":
			color = "8"
		case "info":
			color = "7"
		case "footer":
			color = "10"
		case "border":
			color = "6"  // Cyan
		default:
			color = "15"
		}
	case MonochromeColors:
		color = "15" // Always white
		if textType == "separator" || textType == "info" {
			color = "8" // Gray for separators and info
		}
	case HighContrastColors:
		switch textType {
		case "title", "subtitle", "header":
			color = "0" // Black text on bright background
		default:
			color = "15" // White text
		}
	}
	
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	return style.Render(text)
}

// getStyledText returns styled text
func (wod *WhoOnlineDisplay) getStyledText(text, styleType string) string {
	switch styleType {
	case "selected":
		return wod.getSelectionStyle().Render(text)
	default:
		return text
	}
}

// getSelectionStyle returns the selection highlight style
func (wod *WhoOnlineDisplay) getSelectionStyle() lipgloss.Style {
	switch wod.colorScheme {
	case ClassicColors:
		return lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15"))
	case ModernColors:
		return lipgloss.NewStyle().Background(lipgloss.Color("5")).Foreground(lipgloss.Color("15"))
	case MonochromeColors:
		return lipgloss.NewStyle().Background(lipgloss.Color("7")).Foreground(lipgloss.Color("0"))
	case HighContrastColors:
		return lipgloss.NewStyle().Background(lipgloss.Color("15")).Foreground(lipgloss.Color("0"))
	default:
		return lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15"))
	}
}

// Init implements tea.Model
func (wod *WhoOnlineDisplay) Init() tea.Cmd {
	if wod.autoRefresh {
		return wod.tick()
	}
	return nil
}