package nodes

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// View renders the monitoring system interface
func (ms *MonitoringSystem) View() string {
	var sections []string
	
	// Title bar
	sections = append(sections, ms.renderTitleBar())
	
	// View tabs
	sections = append(sections, ms.renderViewTabs())
	
	// Main content based on current view
	switch ms.currentView {
	case OverviewView:
		sections = append(sections, ms.renderOverviewView())
	case NodeDetailView:
		sections = append(sections, ms.renderNodeDetailView())
	case PerformanceGraphView:
		sections = append(sections, ms.renderPerformanceGraphView())
	case AlertsView:
		sections = append(sections, ms.renderAlertsView())
	case ThresholdsView:
		sections = append(sections, ms.renderThresholdsView())
	case HistoryView:
		sections = append(sections, ms.renderHistoryView())
	}
	
	// Status bar
	sections = append(sections, ms.renderStatusBar())
	
	// Help line
	sections = append(sections, ms.renderHelpLine())
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderTitleBar renders the monitoring title bar
func (ms *MonitoringSystem) renderTitleBar() string {
	title := "ViSiON/3 Performance Monitor & Alert System"
	status := ""
	
	if ms.monitoringEnabled {
		status = "🟢 Active"
	} else {
		status = "🔴 Disabled"
	}
	
	// Add alert count if any active alerts
	activeAlerts := ms.getActiveAlerts()
	if len(activeAlerts) > 0 {
		status += fmt.Sprintf(" | 🚨 %d alerts", len(activeAlerts))
	}
	
	timestamp := ms.lastUpdate.Format("15:04:05")
	
	titleStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("5")).     // Magenta background
		Foreground(lipgloss.Color("15")).    // White text
		Bold(true).
		Padding(0, 1).
		Width(ms.width)
	
	titleBar := fmt.Sprintf("%-40s %*s %10s", title, ms.width-52, status, timestamp)
	return titleStyle.Render(titleBar)
}

// renderViewTabs renders the view selection tabs
func (ms *MonitoringSystem) renderViewTabs() string {
	tabs := []struct {
		key  string
		name string
		view MonitorView
	}{
		{"F1", "Overview", OverviewView},
		{"F2", "Nodes", NodeDetailView},
		{"F3", "Graphs", PerformanceGraphView},
		{"F4", "Alerts", AlertsView},
		{"F5", "Thresholds", ThresholdsView},
		{"F6", "History", HistoryView},
	}
	
	var tabElements []string
	for _, tab := range tabs {
		tabStyle := lipgloss.NewStyle().Padding(0, 1)
		
		if tab.view == ms.currentView {
			tabStyle = tabStyle.Background(lipgloss.Color("5")).Foreground(lipgloss.Color("15"))
		} else {
			tabStyle = tabStyle.Background(lipgloss.Color("7")).Foreground(lipgloss.Color("0"))
		}
		
		tabText := fmt.Sprintf("%s:%s", tab.key, tab.name)
		
		// Add alert indicator for alerts tab
		if tab.view == AlertsView {
			activeAlerts := ms.getActiveAlerts()
			if len(activeAlerts) > 0 {
				tabText += fmt.Sprintf(" (%d)", len(activeAlerts))
			}
		}
		
		tabElements = append(tabElements, tabStyle.Render(tabText))
	}
	
	return lipgloss.JoinHorizontal(lipgloss.Top, tabElements...)
}

// renderOverviewView renders the system overview
func (ms *MonitoringSystem) renderOverviewView() string {
	systemStatus, _ := ms.nodeManager.GetSystemStatus()
	
	var lines []string
	lines = append(lines, "System Performance Overview")
	lines = append(lines, "")
	
	// System metrics
	lines = append(lines, fmt.Sprintf("Total Nodes: %d | Active: %d | Users Online: %d",
		systemStatus.TotalNodes, systemStatus.ActiveNodes, systemStatus.ConnectedUsers))
	lines = append(lines, fmt.Sprintf("System Load: %.1f%% | Memory: %d MB",
		systemStatus.SystemLoad, systemStatus.MemoryUsage/(1024*1024)))
	lines = append(lines, "")
	
	// Node status grid
	lines = append(lines, "Node Status:")
	nodes := ms.nodeManager.GetAllNodes()
	
	// Create a grid view of nodes with their current metrics
	cols := 4
	for i := 0; i < len(nodes); i += cols {
		var rowNodes []string
		
		for j := 0; j < cols && i+j < len(nodes); j++ {
			node := nodes[i+j]
			nodeCard := ms.renderNodeStatusCard(node, (ms.width-8)/cols)
			rowNodes = append(rowNodes, nodeCard)
		}
		
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, rowNodes...))
	}
	
	lines = append(lines, "")
	
	// Recent alerts summary
	recentAlerts := ms.getRecentAlerts(5)
	lines = append(lines, "Recent Alerts:")
	if len(recentAlerts) == 0 {
		lines = append(lines, "  No recent alerts")
	} else {
		for _, alert := range recentAlerts {
			alertLine := fmt.Sprintf("  %s [%s] Node %d: %s",
				ms.getAlertIcon(alert.AlertType),
				alert.Timestamp.Format("15:04"),
				alert.NodeID,
				alert.Message)
			lines = append(lines, ms.colorAlertLine(alertLine, alert.AlertType))
		}
	}
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(ms.width - 4).
		Height(ms.height - 10)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderNodeStatusCard renders a small status card for a node
func (ms *MonitoringSystem) renderNodeStatusCard(node *NodeInfo, width int) string {
	var lines []string
	
	// Node header
	lines = append(lines, fmt.Sprintf("Node %d", node.NodeID))
	
	// Status and user
	if node.User != nil {
		lines = append(lines, fmt.Sprintf("👤 %s", truncateString(node.User.Handle, width-4)))
	} else {
		lines = append(lines, "⭕ Available")
	}
	
	// Performance indicators
	cpuBar := ms.renderMiniBar(node.CPUUsage, 100.0, width-6)
	lines = append(lines, fmt.Sprintf("CPU %s", cpuBar))
	
	memoryPercent := float64(node.MemoryUsage) / float64(1024*1024*1024) * 100 // Assume 1GB max
	memBar := ms.renderMiniBar(memoryPercent, 100.0, width-6)
	lines = append(lines, fmt.Sprintf("MEM %s", memBar))
	
	// Alert indicator
	nodeAlerts := ms.getNodeAlerts(node.NodeID)
	if len(nodeAlerts) > 0 {
		lines = append(lines, fmt.Sprintf("🚨 %d", len(nodeAlerts)))
	} else {
		lines = append(lines, "✅ OK")
	}
	
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(width).
		Height(8).
		Padding(0, 1)
	
	// Color based on status
	if len(nodeAlerts) > 0 {
		cardStyle = cardStyle.BorderForeground(lipgloss.Color("1")) // Red border for alerts
	} else if node.User != nil {
		cardStyle = cardStyle.BorderForeground(lipgloss.Color("2")) // Green border for active
	} else {
		cardStyle = cardStyle.BorderForeground(lipgloss.Color("8")) // Gray border for available
	}
	
	return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderMiniBar renders a mini progress bar
func (ms *MonitoringSystem) renderMiniBar(value, max float64, width int) string {
	if width < 3 {
		return ""
	}
	
	barWidth := width - 2 // Account for brackets
	percentage := value / max
	if percentage > 1.0 {
		percentage = 1.0
	}
	
	filled := int(percentage * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	
	var color string
	if percentage > 0.9 {
		color = "1" // Red
	} else if percentage > 0.7 {
		color = "3" // Yellow
	} else {
		color = "2" // Green
	}
	
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	return fmt.Sprintf("[%s]", style.Render(bar))
}

// renderNodeDetailView renders detailed node information
func (ms *MonitoringSystem) renderNodeDetailView() string {
	node, err := ms.nodeManager.GetNode(ms.selectedNode)
	if err != nil {
		return fmt.Sprintf("Node %d not found", ms.selectedNode)
	}
	
	var lines []string
	lines = append(lines, fmt.Sprintf("Node %d Details - %s", node.NodeID, node.Config.Name))
	lines = append(lines, "")
	
	// Basic info
	lines = append(lines, fmt.Sprintf("Status: %s", node.Status.String()))
	if node.User != nil {
		lines = append(lines, fmt.Sprintf("User: %s (%s)", node.User.Handle, node.User.GroupLocation))
		lines = append(lines, fmt.Sprintf("Activity: %s", node.Activity.Description))
		if !node.ConnectTime.IsZero() {
			lines = append(lines, fmt.Sprintf("Online Time: %s", formatDuration(time.Since(node.ConnectTime))))
		}
		if node.IdleTime > 0 {
			lines = append(lines, fmt.Sprintf("Idle Time: %s", formatDuration(node.IdleTime)))
		}
	}
	lines = append(lines, "")
	
	// Performance metrics
	lines = append(lines, "Performance Metrics:")
	lines = append(lines, fmt.Sprintf("  CPU Usage: %.1f%%", node.CPUUsage))
	lines = append(lines, fmt.Sprintf("  Memory Usage: %d MB", node.MemoryUsage/(1024*1024)))
	lines = append(lines, fmt.Sprintf("  Bytes Sent: %d", node.BytesSent))
	lines = append(lines, fmt.Sprintf("  Bytes Received: %d", node.BytesReceived))
	lines = append(lines, "")
	
	// Configuration
	lines = append(lines, "Configuration:")
	lines = append(lines, fmt.Sprintf("  Enabled: %t", node.Config.Enabled))
	lines = append(lines, fmt.Sprintf("  Max Users: %d", node.Config.MaxUsers))
	lines = append(lines, fmt.Sprintf("  Time Limit: %s", node.Config.TimeLimit.String()))
	lines = append(lines, fmt.Sprintf("  Network: %s:%d", node.Config.NetworkSettings.Protocol, node.Config.NetworkSettings.Port))
	lines = append(lines, "")
	
	// Recent performance data (mini graph)
	if data, exists := ms.performanceData[node.NodeID]; exists && len(data) > 0 {
		lines = append(lines, "CPU Usage Trend (last 20 readings):")
		cpuGraph := ms.renderMiniGraph(data, "cpu", 20, ms.width-10)
		lines = append(lines, cpuGraph)
		lines = append(lines, "")
		
		lines = append(lines, "Memory Usage Trend:")
		memGraph := ms.renderMiniGraph(data, "memory", 20, ms.width-10)
		lines = append(lines, memGraph)
	}
	
	// Node-specific alerts
	nodeAlerts := ms.getNodeAlerts(node.NodeID)
	if len(nodeAlerts) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Active Alerts (%d):", len(nodeAlerts)))
		for _, alert := range nodeAlerts {
			alertLine := fmt.Sprintf("  %s %s: %s",
				ms.getAlertIcon(alert.AlertType),
				alert.Category,
				alert.Message)
			lines = append(lines, ms.colorAlertLine(alertLine, alert.AlertType))
		}
	}
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(ms.width - 4).
		Height(ms.height - 10)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderMiniGraph renders a small ASCII graph
func (ms *MonitoringSystem) renderMiniGraph(data []PerformanceSnapshot, metric string, maxPoints, width int) string {
	if len(data) == 0 {
		return "No data available"
	}
	
	// Get recent data points
	startIdx := len(data) - maxPoints
	if startIdx < 0 {
		startIdx = 0
	}
	
	recentData := data[startIdx:]
	if len(recentData) == 0 {
		return "No data available"
	}
	
	// Extract values based on metric type
	var values []float64
	for _, snapshot := range recentData {
		switch metric {
		case "cpu":
			values = append(values, snapshot.CPUUsage)
		case "memory":
			memMB := float64(snapshot.MemoryUsage) / (1024 * 1024)
			values = append(values, memMB)
		case "network_in":
			values = append(values, float64(snapshot.NetworkIn))
		case "network_out":
			values = append(values, float64(snapshot.NetworkOut))
		}
	}
	
	if len(values) == 0 {
		return "No data available"
	}
	
	// Find min/max for scaling
	minVal, maxVal := values[0], values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	
	// Create ASCII graph
	graphHeight := 5
	var graphLines []string
	
	// Create each line of the graph (top to bottom)
	for row := graphHeight - 1; row >= 0; row-- {
		var line strings.Builder
		threshold := minVal + (maxVal-minVal)*float64(row)/float64(graphHeight-1)
		
		for i, value := range values {
			if i >= width {
				break
			}
			
			if value >= threshold {
				line.WriteString("█")
			} else {
				line.WriteString(" ")
			}
		}
		graphLines = append(graphLines, line.String())
	}
	
	// Add scale labels
	result := make([]string, len(graphLines))
	for i, line := range graphLines {
		scaleValue := maxVal - (maxVal-minVal)*float64(i)/float64(len(graphLines)-1)
		result[i] = fmt.Sprintf("%6.1f │%s", scaleValue, line)
	}
	
	return strings.Join(result, "\n")
}

// renderPerformanceGraphView renders performance graphs
func (ms *MonitoringSystem) renderPerformanceGraphView() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Performance Graphs - Node %d", ms.selectedNode))
	lines = append(lines, fmt.Sprintf("Time Range: %s | Graph Type: %s", 
		ms.graphTimeRange.String(), ms.getGraphTypeName()))
	lines = append(lines, "")
	
	// Get performance data for selected node
	data, exists := ms.performanceData[ms.selectedNode]
	if !exists || len(data) == 0 {
		lines = append(lines, "No performance data available for this node")
		lines = append(lines, "Monitoring must be enabled and running to collect data")
	} else {
		// Filter data by time range
		filteredData := ms.filterDataByTimeRange(data)
		
		if len(filteredData) == 0 {
			lines = append(lines, fmt.Sprintf("No data available for the last %s", ms.graphTimeRange.String()))
		} else {
			// Render multiple graphs
			graphWidth := ms.width - 20
			
			lines = append(lines, "CPU Usage (%):")
			cpuGraph := ms.renderGraph(filteredData, "cpu", graphWidth, 8)
			lines = append(lines, cpuGraph)
			lines = append(lines, "")
			
			lines = append(lines, "Memory Usage (MB):")
			memGraph := ms.renderGraph(filteredData, "memory", graphWidth, 8)
			lines = append(lines, memGraph)
			lines = append(lines, "")
			
			if ms.showLegend {
				lines = append(lines, "Legend:")
				lines = append(lines, "  █ High │ ▓ Medium │ ░ Low │ · Baseline")
			}
		}
	}
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(ms.width - 4).
		Height(ms.height - 10)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderGraph renders a performance graph
func (ms *MonitoringSystem) renderGraph(data []PerformanceSnapshot, metric string, width, height int) string {
	if len(data) == 0 {
		return "No data"
	}
	
	// Extract values
	var values []float64
	var timestamps []time.Time
	
	for _, snapshot := range data {
		timestamps = append(timestamps, snapshot.Timestamp)
		switch metric {
		case "cpu":
			values = append(values, snapshot.CPUUsage)
		case "memory":
			memMB := float64(snapshot.MemoryUsage) / (1024 * 1024)
			values = append(values, memMB)
		case "network_in":
			values = append(values, float64(snapshot.NetworkIn)/(1024*1024)) // Convert to MB/s
		case "network_out":
			values = append(values, float64(snapshot.NetworkOut)/(1024*1024)) // Convert to MB/s
		}
	}
	
	// Find min/max for scaling
	minVal, maxVal := values[0], values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	
	// Auto-scale if enabled
	if ms.autoScale {
		// Add some padding
		padding := (maxVal - minVal) * 0.1
		minVal -= padding
		maxVal += padding
		if minVal < 0 {
			minVal = 0
		}
	}
	
	// Render based on graph type
	switch ms.graphType {
	case LineGraph:
		return ms.renderLineGraph(values, minVal, maxVal, width, height)
	case BarGraph:
		return ms.renderBarGraph(values, minVal, maxVal, width, height)
	case AreaGraph:
		return ms.renderAreaGraph(values, minVal, maxVal, width, height)
	case RealTimeGraph:
		return ms.renderRealTimeGraph(values, minVal, maxVal, width, height)
	default:
		return ms.renderLineGraph(values, minVal, maxVal, width, height)
	}
}

// renderLineGraph renders a line graph
func (ms *MonitoringSystem) renderLineGraph(values []float64, minVal, maxVal float64, width, height int) string {
	var lines []string
	
	// Create graph matrix
	graph := make([][]rune, height)
	for i := range graph {
		graph[i] = make([]rune, width)
		for j := range graph[i] {
			graph[i][j] = ' '
		}
	}
	
	// Plot points
	for i, value := range values {
		if i >= width {
			break
		}
		
		// Calculate y position
		normalizedVal := (value - minVal) / (maxVal - minVal)
		if normalizedVal > 1.0 {
			normalizedVal = 1.0
		}
		if normalizedVal < 0.0 {
			normalizedVal = 0.0
		}
		
		y := height - 1 - int(normalizedVal*float64(height-1))
		if y >= 0 && y < height {
			graph[y][i] = '█'
		}
		
		// Connect to previous point (simple line drawing)
		if i > 0 && i-1 < len(values) {
			prevVal := (values[i-1] - minVal) / (maxVal - minVal)
			if prevVal > 1.0 {
				prevVal = 1.0
			}
			if prevVal < 0.0 {
				prevVal = 0.0
			}
			prevY := height - 1 - int(prevVal*float64(height-1))
			
			// Draw line between points
			startY, endY := prevY, y
			if startY > endY {
				startY, endY = endY, startY
			}
			
			for lineY := startY; lineY <= endY; lineY++ {
				if lineY >= 0 && lineY < height && i-1 >= 0 && i-1 < width {
					if graph[lineY][i-1] == ' ' {
						graph[lineY][i-1] = '▓'
					}
				}
			}
		}
	}
	
	// Convert to strings with scale
	for i, row := range graph {
		scaleValue := maxVal - (maxVal-minVal)*float64(i)/float64(height-1)
		line := fmt.Sprintf("%8.1f │%s", scaleValue, string(row))
		lines = append(lines, line)
	}
	
	return strings.Join(lines, "\n")
}

// renderBarGraph renders a bar graph
func (ms *MonitoringSystem) renderBarGraph(values []float64, minVal, maxVal float64, width, height int) string {
	var lines []string
	
	// Create bars
	for row := height - 1; row >= 0; row-- {
		var line strings.Builder
		threshold := minVal + (maxVal-minVal)*float64(row)/float64(height-1)
		
		for i, value := range values {
			if i >= width {
				break
			}
			
			if value >= threshold {
				line.WriteString("█")
			} else {
				line.WriteString(" ")
			}
		}
		
		scaleValue := minVal + (maxVal-minVal)*float64(row)/float64(height-1)
		lineStr := fmt.Sprintf("%8.1f │%s", scaleValue, line.String())
		lines = append(lines, lineStr)
	}
	
	return strings.Join(lines, "\n")
}

// renderAreaGraph renders an area graph
func (ms *MonitoringSystem) renderAreaGraph(values []float64, minVal, maxVal float64, width, height int) string {
	var lines []string
	
	// Similar to line graph but fill area below
	graph := make([][]rune, height)
	for i := range graph {
		graph[i] = make([]rune, width)
		for j := range graph[i] {
			graph[i][j] = ' '
		}
	}
	
	// Plot and fill areas
	for i, value := range values {
		if i >= width {
			break
		}
		
		normalizedVal := (value - minVal) / (maxVal - minVal)
		if normalizedVal > 1.0 {
			normalizedVal = 1.0
		}
		if normalizedVal < 0.0 {
			normalizedVal = 0.0
		}
		
		fillHeight := int(normalizedVal * float64(height-1))
		
		// Fill from bottom up
		for y := height - 1; y >= height-1-fillHeight; y-- {
			if y >= 0 && y < height {
				if y == height-1-fillHeight {
					graph[y][i] = '█' // Top of fill
				} else {
					graph[y][i] = '▓' // Fill area
				}
			}
		}
	}
	
	// Convert to strings
	for i, row := range graph {
		scaleValue := maxVal - (maxVal-minVal)*float64(i)/float64(height-1)
		line := fmt.Sprintf("%8.1f │%s", scaleValue, string(row))
		lines = append(lines, line)
	}
	
	return strings.Join(lines, "\n")
}

// renderRealTimeGraph renders a real-time scrolling graph
func (ms *MonitoringSystem) renderRealTimeGraph(values []float64, minVal, maxVal float64, width, height int) string {
	// Take only the most recent values that fit in width
	startIdx := len(values) - width
	if startIdx < 0 {
		startIdx = 0
	}
	recentValues := values[startIdx:]
	
	return ms.renderLineGraph(recentValues, minVal, maxVal, width, height)
}

// renderAlertsView renders the alerts management view
func (ms *MonitoringSystem) renderAlertsView() string {
	alerts := ms.getFilteredAlerts()
	
	var lines []string
	lines = append(lines, fmt.Sprintf("Alert Management - %d alerts", len(alerts)))
	lines = append(lines, "")
	
	// Filter controls
	lines = append(lines, "Filters:")
	lines = append(lines, fmt.Sprintf("  Warnings: %t | Critical: %t | Info: %t",
		ms.alertFilters.ShowWarnings, ms.alertFilters.ShowCritical, ms.alertFilters.ShowInfo))
	lines = append(lines, fmt.Sprintf("  Show Ack'd: %t | Time: %s | Node: %s",
		ms.alertFilters.ShowAcknowledged, ms.alertFilters.TimeFilter,
		ms.getNodeFilterText()))
	lines = append(lines, "")
	
	// Alert summary
	warningCount, criticalCount, infoCount := ms.getAlertCounts(alerts)
	lines = append(lines, fmt.Sprintf("Summary: %d Critical | %d Warning | %d Info",
		criticalCount, warningCount, infoCount))
	lines = append(lines, "")
	
	// Alert list
	if len(alerts) == 0 {
		lines = append(lines, "No alerts match current filters")
	} else {
		// Headers
		headerLine := fmt.Sprintf("%-4s %-8s %-6s %-12s %-30s %-12s %-3s",
			"Node", "Type", "Cat", "Time", "Message", "Status", "Sev")
		lines = append(lines, headerLine)
		lines = append(lines, strings.Repeat("─", len(headerLine)))
		
		// Show alerts with paging
		maxVisible := ms.height - 20
		startIdx := ms.scrollOffset
		endIdx := startIdx + maxVisible
		if endIdx > len(alerts) {
			endIdx = len(alerts)
		}
		
		for i := startIdx; i < endIdx; i++ {
			alert := alerts[i]
			status := "Active"
			if alert.Acknowledged {
				status = "Ack'd"
			}
			if alert.AutoCleared {
				status = "Cleared"
			}
			
			alertLine := fmt.Sprintf("%-4d %-8s %-6s %-12s %-30s %-12s %-3d",
				alert.NodeID,
				alert.AlertType,
				alert.Category,
				alert.Timestamp.Format("15:04:05"),
				truncateString(alert.Message, 30),
				status,
				alert.Severity)
			
			lines = append(lines, ms.colorAlertLine(alertLine, alert.AlertType))
		}
		
		if len(alerts) > maxVisible {
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("Showing %d-%d of %d alerts", 
				startIdx+1, endIdx, len(alerts)))
		}
	}
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(ms.width - 4).
		Height(ms.height - 10)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// Helper methods continued in next response due to length...