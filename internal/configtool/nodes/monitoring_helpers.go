package nodes

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Monitoring helper methods

// renderThresholdsView renders the alert thresholds configuration view
func (ms *MonitoringSystem) renderThresholdsView() string {
	var lines []string
	lines = append(lines, "Alert Thresholds Configuration")
	lines = append(lines, "")
	
	// CPU thresholds
	lines = append(lines, "CPU Usage:")
	lines = append(lines, fmt.Sprintf("  Warning:  %.1f%%", ms.alertThresholds.CPUWarning))
	lines = append(lines, fmt.Sprintf("  Critical: %.1f%%", ms.alertThresholds.CPUCritical))
	lines = append(lines, "")
	
	// Memory thresholds
	lines = append(lines, "Memory Usage:")
	lines = append(lines, fmt.Sprintf("  Warning:  %d MB", ms.alertThresholds.MemoryWarning/(1024*1024)))
	lines = append(lines, fmt.Sprintf("  Critical: %d MB", ms.alertThresholds.MemoryCritical/(1024*1024)))
	lines = append(lines, "")
	
	// Response time thresholds
	lines = append(lines, "Response Time:")
	lines = append(lines, fmt.Sprintf("  Warning:  %s", ms.alertThresholds.ResponseTimeWarning))
	lines = append(lines, fmt.Sprintf("  Critical: %s", ms.alertThresholds.ResponseTimeCritical))
	lines = append(lines, "")
	
	// Error rate thresholds
	lines = append(lines, "Error Rate:")
	lines = append(lines, fmt.Sprintf("  Warning:  %.1f errors/min", ms.alertThresholds.ErrorRateWarning))
	lines = append(lines, fmt.Sprintf("  Critical: %.1f errors/min", ms.alertThresholds.ErrorRateCritical))
	lines = append(lines, "")
	
	// Other thresholds
	lines = append(lines, "Other Thresholds:")
	lines = append(lines, fmt.Sprintf("  Idle Time Warning: %s", ms.alertThresholds.IdleTimeWarning))
	lines = append(lines, fmt.Sprintf("  Disk I/O Warning: %d MB/s", ms.alertThresholds.DiskIOWarning/(1024*1024)))
	lines = append(lines, fmt.Sprintf("  Network Warning:  %d MB/s", ms.alertThresholds.NetworkWarning/(1024*1024)))
	lines = append(lines, "")
	
	lines = append(lines, "Note: Threshold editing not yet implemented")
	lines = append(lines, "Future versions will allow real-time threshold adjustment")
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(ms.width - 4).
		Height(ms.height - 10)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderHistoryView renders the performance history view
func (ms *MonitoringSystem) renderHistoryView() string {
	var lines []string
	lines = append(lines, "Performance History & Trends")
	lines = append(lines, "")
	
	// Overall statistics
	totalDataPoints := 0
	oldestData := time.Now()
	newestData := time.Time{}
	
	for nodeID, data := range ms.performanceData {
		totalDataPoints += len(data)
		
		if len(data) > 0 {
			if data[0].Timestamp.Before(oldestData) {
				oldestData = data[0].Timestamp
			}
			if data[len(data)-1].Timestamp.After(newestData) {
				newestData = data[len(data)-1].Timestamp
			}
		}
		
		lines = append(lines, fmt.Sprintf("Node %d: %d data points", nodeID, len(data)))
	}
	
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Total Data Points: %d", totalDataPoints))
	if !oldestData.IsZero() && !newestData.IsZero() {
		lines = append(lines, fmt.Sprintf("Data Range: %s to %s", 
			oldestData.Format("15:04:05"), newestData.Format("15:04:05")))
		lines = append(lines, fmt.Sprintf("Collection Period: %s", 
			newestData.Sub(oldestData).String()))
	}
	lines = append(lines, "")
	
	// Performance trends by node
	lines = append(lines, "Node Performance Summary:")
	nodes := ms.nodeManager.GetAllNodes()
	
	for _, node := range nodes {
		if data, exists := ms.performanceData[node.NodeID]; exists && len(data) > 0 {
			latest := data[len(data)-1]
			
			// Calculate averages
			avgCPU, avgMem := ms.calculateAverages(data)
			
			lines = append(lines, fmt.Sprintf("  Node %d (%s):", node.NodeID, node.Config.Name))
			lines = append(lines, fmt.Sprintf("    Current: CPU %.1f%%, Memory %d MB", 
				latest.CPUUsage, latest.MemoryUsage/(1024*1024)))
			lines = append(lines, fmt.Sprintf("    Average: CPU %.1f%%, Memory %.1f MB", 
				avgCPU, avgMem))
			
			// Trend indicators
			cpuTrend := ms.calculateTrend(data, "cpu")
			memTrend := ms.calculateTrend(data, "memory")
			
			lines = append(lines, fmt.Sprintf("    Trends:  CPU %s, Memory %s", 
				ms.getTrendSymbol(cpuTrend), ms.getTrendSymbol(memTrend)))
		} else {
			lines = append(lines, fmt.Sprintf("  Node %d: No data available", node.NodeID))
		}
	}
	
	lines = append(lines, "")
	
	// Alert history summary
	lines = append(lines, "Alert History Summary:")
	alertStats := ms.getAlertStatistics()
	lines = append(lines, fmt.Sprintf("  Total Alerts: %d", len(ms.alertHistory)))
	lines = append(lines, fmt.Sprintf("  Critical: %d | Warning: %d | Info: %d", 
		alertStats.Critical, alertStats.Warning, alertStats.Info))
	lines = append(lines, fmt.Sprintf("  Acknowledged: %d | Auto-cleared: %d", 
		alertStats.Acknowledged, alertStats.AutoCleared))
	
	if len(ms.alertHistory) > 0 {
		// Most common alert categories
		categoryStats := ms.getCategoryStatistics()
		lines = append(lines, "  Most Common Categories:")
		
		var categories []string
		for category, count := range categoryStats {
			categories = append(categories, fmt.Sprintf("%s: %d", category, count))
		}
		
		if len(categories) > 5 {
			categories = categories[:5] // Show top 5
		}
		
		for _, cat := range categories {
			lines = append(lines, fmt.Sprintf("    %s", cat))
		}
	}
	
	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Width(ms.width - 4).
		Height(ms.height - 10)
	
	return contentStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderStatusBar renders the monitoring status bar
func (ms *MonitoringSystem) renderStatusBar() string {
	var parts []string
	
	// Current view
	parts = append(parts, fmt.Sprintf("View: %s", ms.getCurrentViewName()))
	
	// Monitoring status
	if ms.monitoringEnabled {
		parts = append(parts, "Monitoring: ON")
	} else {
		parts = append(parts, "Monitoring: OFF")
	}
	
	// Active alerts count
	activeAlerts := ms.getActiveAlerts()
	parts = append(parts, fmt.Sprintf("Alerts: %d", len(activeAlerts)))
	
	// Selected node (for relevant views)
	if ms.currentView == NodeDetailView || ms.currentView == PerformanceGraphView {
		parts = append(parts, fmt.Sprintf("Node: %d", ms.selectedNode))
	}
	
	// Data collection info
	totalPoints := 0
	for _, data := range ms.performanceData {
		totalPoints += len(data)
	}
	parts = append(parts, fmt.Sprintf("Data Points: %d", totalPoints))
	
	status := strings.Join(parts, " | ")
	
	statusStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("8")).
		Foreground(lipgloss.Color("15")).
		Width(ms.width)
	
	return statusStyle.Render(status)
}

// renderHelpLine renders the help line
func (ms *MonitoringSystem) renderHelpLine() string {
	var help string
	
	switch ms.currentView {
	case OverviewView:
		help = "TAB:Views F1-F6:Direct M:Toggle Monitor R:Refresh ESC:Exit"
	case NodeDetailView:
		help = "↑/↓:Select Node TAB:Views M:Monitor R:Refresh ESC:Exit"
	case PerformanceGraphView:
		help = "↑/↓:Node ←/→:TimeRange G:GraphType L:Legend S:Scale TAB:Views"
	case AlertsView:
		help = "↑/↓:Scroll A:Acknowledge C:Clear PgUp/PgDn:Page TAB:Views"
	case ThresholdsView:
		help = "TAB:Views R:Refresh (Editing not yet implemented)"
	case HistoryView:
		help = "TAB:Views R:Refresh ESC:Exit"
	default:
		help = "TAB:Views F1-F6:Direct ESC:Exit"
	}
	
	helpStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("0")).
		Foreground(lipgloss.Color("7")).
		Width(ms.width)
	
	return helpStyle.Render(help)
}

// Helper utility methods

// getCurrentViewName returns the current view name
func (ms *MonitoringSystem) getCurrentViewName() string {
	switch ms.currentView {
	case OverviewView:
		return "Overview"
	case NodeDetailView:
		return "Node Detail"
	case PerformanceGraphView:
		return "Performance Graphs"
	case AlertsView:
		return "Alerts"
	case ThresholdsView:
		return "Thresholds"
	case HistoryView:
		return "History"
	default:
		return "Unknown"
	}
}

// getGraphTypeName returns the current graph type name
func (ms *MonitoringSystem) getGraphTypeName() string {
	switch ms.graphType {
	case LineGraph:
		return "Line"
	case BarGraph:
		return "Bar"
	case AreaGraph:
		return "Area"
	case RealTimeGraph:
		return "Real-time"
	default:
		return "Unknown"
	}
}

// getActiveAlerts returns currently active alerts
func (ms *MonitoringSystem) getActiveAlerts() []AlertRecord {
	var active []AlertRecord
	for _, alert := range ms.alertHistory {
		if !alert.Acknowledged && !alert.AutoCleared {
			active = append(active, alert)
		}
	}
	return active
}

// getRecentAlerts returns recent alerts
func (ms *MonitoringSystem) getRecentAlerts(count int) []AlertRecord {
	if len(ms.alertHistory) == 0 {
		return nil
	}
	
	startIdx := len(ms.alertHistory) - count
	if startIdx < 0 {
		startIdx = 0
	}
	
	return ms.alertHistory[startIdx:]
}

// getNodeAlerts returns alerts for a specific node
func (ms *MonitoringSystem) getNodeAlerts(nodeID int) []AlertRecord {
	var nodeAlerts []AlertRecord
	for _, alert := range ms.alertHistory {
		if alert.NodeID == nodeID && !alert.Acknowledged && !alert.AutoCleared {
			nodeAlerts = append(nodeAlerts, alert)
		}
	}
	return nodeAlerts
}

// getFilteredAlerts returns alerts matching current filters
func (ms *MonitoringSystem) getFilteredAlerts() []AlertRecord {
	var filtered []AlertRecord
	
	for _, alert := range ms.alertHistory {
		// Apply filters
		if !ms.alertFilters.ShowWarnings && alert.AlertType == "warning" {
			continue
		}
		if !ms.alertFilters.ShowCritical && alert.AlertType == "critical" {
			continue
		}
		if !ms.alertFilters.ShowInfo && alert.AlertType == "info" {
			continue
		}
		if !ms.alertFilters.ShowAcknowledged && alert.Acknowledged {
			continue
		}
		if !ms.alertFilters.ShowAutoCleared && alert.AutoCleared {
			continue
		}
		
		// Node filter
		if ms.alertFilters.NodeFilter > 0 && alert.NodeID != ms.alertFilters.NodeFilter {
			continue
		}
		
		// Category filter
		if ms.alertFilters.CategoryFilter != "" && alert.Category != ms.alertFilters.CategoryFilter {
			continue
		}
		
		// Time filter
		if !ms.isWithinTimeFilter(alert.Timestamp) {
			continue
		}
		
		filtered = append(filtered, alert)
	}
	
	// Sort by timestamp (newest first)
	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[i].Timestamp.Before(filtered[j].Timestamp) {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}
	
	return filtered
}

// isWithinTimeFilter checks if timestamp is within the current time filter
func (ms *MonitoringSystem) isWithinTimeFilter(timestamp time.Time) bool {
	now := time.Now()
	
	switch ms.alertFilters.TimeFilter {
	case "1h":
		return now.Sub(timestamp) <= time.Hour
	case "24h":
		return now.Sub(timestamp) <= 24*time.Hour
	case "7d":
		return now.Sub(timestamp) <= 7*24*time.Hour
	case "all":
		return true
	default:
		return now.Sub(timestamp) <= 24*time.Hour
	}
}

// getAlertCounts returns counts by alert type
func (ms *MonitoringSystem) getAlertCounts(alerts []AlertRecord) (warning, critical, info int) {
	for _, alert := range alerts {
		switch alert.AlertType {
		case "warning":
			warning++
		case "critical":
			critical++
		case "info":
			info++
		}
	}
	return
}

// getNodeFilterText returns the node filter display text
func (ms *MonitoringSystem) getNodeFilterText() string {
	if ms.alertFilters.NodeFilter == 0 {
		return "All"
	}
	return fmt.Sprintf("Node %d", ms.alertFilters.NodeFilter)
}

// getAlertIcon returns an icon for the alert type
func (ms *MonitoringSystem) getAlertIcon(alertType string) string {
	switch alertType {
	case "critical":
		return "🚨"
	case "warning":
		return "⚠️"
	case "info":
		return "ℹ️"
	default:
		return "📢"
	}
}

// colorAlertLine colors an alert line based on type
func (ms *MonitoringSystem) colorAlertLine(line, alertType string) string {
	var color string
	switch alertType {
	case "critical":
		color = "1" // Red
	case "warning":
		color = "3" // Yellow
	case "info":
		color = "4" // Blue
	default:
		color = "7" // White
	}
	
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	return style.Render(line)
}

// filterDataByTimeRange filters performance data by current time range
func (ms *MonitoringSystem) filterDataByTimeRange(data []PerformanceSnapshot) []PerformanceSnapshot {
	if len(data) == 0 {
		return data
	}
	
	cutoff := time.Now().Add(-ms.graphTimeRange)
	var filtered []PerformanceSnapshot
	
	for _, snapshot := range data {
		if snapshot.Timestamp.After(cutoff) {
			filtered = append(filtered, snapshot)
		}
	}
	
	return filtered
}

// calculateAverages calculates average CPU and memory usage
func (ms *MonitoringSystem) calculateAverages(data []PerformanceSnapshot) (avgCPU, avgMem float64) {
	if len(data) == 0 {
		return 0, 0
	}
	
	var totalCPU, totalMem float64
	for _, snapshot := range data {
		totalCPU += snapshot.CPUUsage
		totalMem += float64(snapshot.MemoryUsage) / (1024 * 1024) // Convert to MB
	}
	
	count := float64(len(data))
	return totalCPU / count, totalMem / count
}

// calculateTrend calculates the trend direction for a metric
func (ms *MonitoringSystem) calculateTrend(data []PerformanceSnapshot, metric string) float64 {
	if len(data) < 2 {
		return 0
	}
	
	// Take first and last 25% of data points to calculate trend
	segmentSize := len(data) / 4
	if segmentSize < 1 {
		segmentSize = 1
	}
	
	var firstValues, lastValues []float64
	
	for i := 0; i < segmentSize && i < len(data); i++ {
		var value float64
		switch metric {
		case "cpu":
			value = data[i].CPUUsage
		case "memory":
			value = float64(data[i].MemoryUsage) / (1024 * 1024)
		}
		firstValues = append(firstValues, value)
	}
	
	for i := len(data) - segmentSize; i < len(data); i++ {
		var value float64
		switch metric {
		case "cpu":
			value = data[i].CPUUsage
		case "memory":
			value = float64(data[i].MemoryUsage) / (1024 * 1024)
		}
		lastValues = append(lastValues, value)
	}
	
	// Calculate averages
	var firstAvg, lastAvg float64
	for _, v := range firstValues {
		firstAvg += v
	}
	firstAvg /= float64(len(firstValues))
	
	for _, v := range lastValues {
		lastAvg += v
	}
	lastAvg /= float64(len(lastValues))
	
	// Return trend (positive = increasing, negative = decreasing)
	return lastAvg - firstAvg
}

// getTrendSymbol returns a symbol representing the trend direction
func (ms *MonitoringSystem) getTrendSymbol(trend float64) string {
	if trend > 5 {
		return "↗️"  // Strong increase
	} else if trend > 1 {
		return "↗"   // Increase
	} else if trend < -5 {
		return "↘️"  // Strong decrease
	} else if trend < -1 {
		return "↘"   // Decrease
	} else {
		return "→"   // Stable
	}
}

// getAlertStatistics returns alert statistics
func (ms *MonitoringSystem) getAlertStatistics() struct {
	Critical     int
	Warning      int
	Info         int
	Acknowledged int
	AutoCleared  int
} {
	var stats struct {
		Critical     int
		Warning      int
		Info         int
		Acknowledged int
		AutoCleared  int
	}
	
	for _, alert := range ms.alertHistory {
		switch alert.AlertType {
		case "critical":
			stats.Critical++
		case "warning":
			stats.Warning++
		case "info":
			stats.Info++
		}
		
		if alert.Acknowledged {
			stats.Acknowledged++
		}
		if alert.AutoCleared {
			stats.AutoCleared++
		}
	}
	
	return stats
}

// getCategoryStatistics returns alert statistics by category
func (ms *MonitoringSystem) getCategoryStatistics() map[string]int {
	categories := make(map[string]int)
	
	for _, alert := range ms.alertHistory {
		categories[alert.Category]++
	}
	
	return categories
}

// Init implements tea.Model
func (ms *MonitoringSystem) Init() tea.Cmd {
	if ms.monitoringEnabled {
		return ms.tick()
	}
	return nil
}