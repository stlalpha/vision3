package nodes

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MonitoringSystem represents the performance monitoring and alerting system
type MonitoringSystem struct {
	nodeManager       NodeManager
	width             int
	height            int
	focused           bool
	currentView       MonitorView
	selectedNode      int
	refreshRate       time.Duration
	lastUpdate        time.Time
	alertThresholds   AlertThresholds
	monitoringEnabled bool
	performanceData   map[int][]PerformanceSnapshot
	alertHistory      []AlertRecord
	graphTimeRange    time.Duration
	scrollOffset      int
	maxDataPoints     int
	autoScale         bool
	showLegend        bool
	graphType         GraphType
	alertFilters      AlertFilters
}

// MonitorView represents different monitoring views
type MonitorView int

const (
	OverviewView MonitorView = iota
	NodeDetailView
	PerformanceGraphView
	AlertsView
	ThresholdsView
	HistoryView
)

// GraphType represents different graph types
type GraphType int

const (
	LineGraph GraphType = iota
	BarGraph
	AreaGraph
	RealTimeGraph
)

// PerformanceSnapshot represents a point-in-time performance reading
type PerformanceSnapshot struct {
	Timestamp       time.Time     `json:"timestamp"`
	NodeID          int           `json:"node_id"`
	CPUUsage        float64       `json:"cpu_usage"`        // Percentage
	MemoryUsage     int64         `json:"memory_usage"`     // Bytes
	NetworkIn       int64         `json:"network_in"`       // Bytes/sec
	NetworkOut      int64         `json:"network_out"`      // Bytes/sec
	DiskIO          int64         `json:"disk_io"`          // Bytes/sec
	ResponseTime    time.Duration `json:"response_time"`    // Average response time
	ErrorRate       float64       `json:"error_rate"`       // Errors per minute
	SessionCount    int           `json:"session_count"`    // Active sessions
	IdleTime        time.Duration `json:"idle_time"`        // Current idle time
	ConnectionCount int           `json:"connection_count"` // Total connections since start
	BytesTransferred int64        `json:"bytes_transferred"` // Total bytes transferred
}

// AlertThresholds represents configurable alert thresholds
type AlertThresholds struct {
	CPUWarning          float64       `json:"cpu_warning"`           // CPU usage warning %
	CPUCritical         float64       `json:"cpu_critical"`          // CPU usage critical %
	MemoryWarning       int64         `json:"memory_warning"`        // Memory usage warning bytes
	MemoryCritical      int64         `json:"memory_critical"`       // Memory usage critical bytes
	ResponseTimeWarning time.Duration `json:"response_time_warning"` // Response time warning
	ResponseTimeCritical time.Duration `json:"response_time_critical"` // Response time critical
	ErrorRateWarning    float64       `json:"error_rate_warning"`    // Error rate warning
	ErrorRateCritical   float64       `json:"error_rate_critical"`   // Error rate critical
	IdleTimeWarning     time.Duration `json:"idle_time_warning"`     // Idle time warning
	DiskIOWarning       int64         `json:"disk_io_warning"`       // Disk I/O warning
	NetworkWarning      int64         `json:"network_warning"`       // Network usage warning
}

// AlertRecord represents a recorded alert
type AlertRecord struct {
	ID           int           `json:"id"`
	NodeID       int           `json:"node_id"`
	AlertType    string        `json:"alert_type"`    // "warning", "critical", "info"
	Category     string        `json:"category"`      // "cpu", "memory", "network", etc.
	Message      string        `json:"message"`
	Value        interface{}   `json:"value"`        // The actual value that triggered alert
	Threshold    interface{}   `json:"threshold"`    // The threshold that was exceeded
	Timestamp    time.Time     `json:"timestamp"`
	Acknowledged bool          `json:"acknowledged"`
	AckBy        string        `json:"ack_by"`       // Who acknowledged it
	AckTime      time.Time     `json:"ack_time"`
	AutoCleared  bool          `json:"auto_cleared"`
	ClearTime    time.Time     `json:"clear_time"`
	Duration     time.Duration `json:"duration"`     // How long the alert was active
	Severity     int           `json:"severity"`     // 1=low, 2=medium, 3=high, 4=critical
}

// AlertFilters represents filtering options for alerts
type AlertFilters struct {
	ShowWarnings    bool   `json:"show_warnings"`
	ShowCritical    bool   `json:"show_critical"`
	ShowInfo        bool   `json:"show_info"`
	ShowAcknowledged bool  `json:"show_acknowledged"`
	ShowAutoCleared bool   `json:"show_auto_cleared"`
	NodeFilter      int    `json:"node_filter"`      // 0 = all nodes
	CategoryFilter  string `json:"category_filter"`  // "" = all categories
	TimeFilter      string `json:"time_filter"`      // "1h", "24h", "7d", "all"
}

// NewMonitoringSystem creates a new monitoring system
func NewMonitoringSystem(nodeManager NodeManager, width, height int) *MonitoringSystem {
	return &MonitoringSystem{
		nodeManager:       nodeManager,
		width:             width,
		height:            height,
		currentView:       OverviewView,
		refreshRate:       2 * time.Second,
		monitoringEnabled: true,
		performanceData:   make(map[int][]PerformanceSnapshot),
		alertHistory:      make([]AlertRecord, 0),
		graphTimeRange:    time.Hour,
		maxDataPoints:     100,
		autoScale:         true,
		showLegend:        true,
		graphType:         LineGraph,
		alertThresholds:   getDefaultAlertThresholds(),
		alertFilters:      getDefaultAlertFilters(),
	}
}

// getDefaultAlertThresholds returns default alert thresholds
func getDefaultAlertThresholds() AlertThresholds {
	return AlertThresholds{
		CPUWarning:           75.0,
		CPUCritical:          90.0,
		MemoryWarning:        1024 * 1024 * 1024,     // 1GB
		MemoryCritical:       2048 * 1024 * 1024,     // 2GB
		ResponseTimeWarning:  500 * time.Millisecond,
		ResponseTimeCritical: 2 * time.Second,
		ErrorRateWarning:     5.0,  // 5 errors per minute
		ErrorRateCritical:    15.0, // 15 errors per minute
		IdleTimeWarning:      10 * time.Minute,
		DiskIOWarning:        50 * 1024 * 1024,    // 50MB/s
		NetworkWarning:       10 * 1024 * 1024,    // 10MB/s
	}
}

// getDefaultAlertFilters returns default alert filters
func getDefaultAlertFilters() AlertFilters {
	return AlertFilters{
		ShowWarnings:     true,
		ShowCritical:     true,
		ShowInfo:         true,
		ShowAcknowledged: false,
		ShowAutoCleared:  false,
		NodeFilter:       0,
		CategoryFilter:   "",
		TimeFilter:       "24h",
	}
}

// Update implements tea.Model
func (ms *MonitoringSystem) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return ms.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		ms.width = msg.Width
		ms.height = msg.Height
	case TickMsg:
		ms.lastUpdate = time.Now()
		if ms.monitoringEnabled {
			ms.collectPerformanceData()
			ms.checkAlertConditions()
		}
		return ms, ms.tick()
	}
	return ms, nil
}

// tick returns a command for periodic updates
func (ms *MonitoringSystem) tick() tea.Cmd {
	return tea.Tick(ms.refreshRate, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// handleKeyPress processes keyboard input
func (ms *MonitoringSystem) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return ms, tea.Quit
	case "r", "f5":
		ms.collectPerformanceData()
		ms.lastUpdate = time.Now()
	case "m":
		ms.monitoringEnabled = !ms.monitoringEnabled
	case "tab":
		ms.cycleView()
	case "f1":
		ms.currentView = OverviewView
	case "f2":
		ms.currentView = NodeDetailView
	case "f3":
		ms.currentView = PerformanceGraphView
	case "f4":
		ms.currentView = AlertsView
	case "f5":
		ms.currentView = ThresholdsView
	case "f6":
		ms.currentView = HistoryView
	case "up", "k":
		ms.handleUpKey()
	case "down", "j":
		ms.handleDownKey()
	case "left", "h":
		ms.handleLeftKey()
	case "right", "l":
		ms.handleRightKey()
	case "pageup":
		ms.scrollOffset -= 10
		if ms.scrollOffset < 0 {
			ms.scrollOffset = 0
		}
	case "pagedown":
		ms.scrollOffset += 10
	case "home":
		ms.scrollOffset = 0
		ms.selectedNode = 1
	case "end":
		ms.selectedNode = ms.getMaxNodeID()
	case "a":
		ms.acknowledgeSelectedAlert()
	case "c":
		ms.clearSelectedAlert()
	case "g":
		ms.cycleGraphType()
	case "l":
		ms.showLegend = !ms.showLegend
	case "s":
		ms.autoScale = !ms.autoScale
	case "t":
		ms.cycleTimeRange()
	case "enter":
		ms.handleEnterKey()
	}
	return ms, nil
}

// View-specific key handlers
func (ms *MonitoringSystem) handleUpKey() {
	switch ms.currentView {
	case NodeDetailView, PerformanceGraphView:
		if ms.selectedNode > 1 {
			ms.selectedNode--
		}
	case AlertsView:
		// Navigate alerts
		if ms.scrollOffset > 0 {
			ms.scrollOffset--
		}
	}
}

func (ms *MonitoringSystem) handleDownKey() {
	switch ms.currentView {
	case NodeDetailView, PerformanceGraphView:
		maxNode := ms.getMaxNodeID()
		if ms.selectedNode < maxNode {
			ms.selectedNode++
		}
	case AlertsView:
		// Navigate alerts
		ms.scrollOffset++
	}
}

func (ms *MonitoringSystem) handleLeftKey() {
	switch ms.currentView {
	case PerformanceGraphView:
		// Adjust time range
		if ms.graphTimeRange > time.Minute {
			ms.graphTimeRange = ms.graphTimeRange / 2
		}
	}
}

func (ms *MonitoringSystem) handleRightKey() {
	switch ms.currentView {
	case PerformanceGraphView:
		// Adjust time range
		if ms.graphTimeRange < 24*time.Hour {
			ms.graphTimeRange = ms.graphTimeRange * 2
		}
	}
}

func (ms *MonitoringSystem) handleEnterKey() {
	switch ms.currentView {
	case OverviewView:
		ms.currentView = NodeDetailView
	case AlertsView:
		ms.acknowledgeSelectedAlert()
	}
}

// cycleView cycles through monitoring views
func (ms *MonitoringSystem) cycleView() {
	switch ms.currentView {
	case OverviewView:
		ms.currentView = NodeDetailView
	case NodeDetailView:
		ms.currentView = PerformanceGraphView
	case PerformanceGraphView:
		ms.currentView = AlertsView
	case AlertsView:
		ms.currentView = ThresholdsView
	case ThresholdsView:
		ms.currentView = HistoryView
	case HistoryView:
		ms.currentView = OverviewView
	}
}

// cycleGraphType cycles through graph types
func (ms *MonitoringSystem) cycleGraphType() {
	switch ms.graphType {
	case LineGraph:
		ms.graphType = BarGraph
	case BarGraph:
		ms.graphType = AreaGraph
	case AreaGraph:
		ms.graphType = RealTimeGraph
	case RealTimeGraph:
		ms.graphType = LineGraph
	}
}

// cycleTimeRange cycles through time ranges for graphs
func (ms *MonitoringSystem) cycleTimeRange() {
	ranges := []time.Duration{
		5 * time.Minute,
		15 * time.Minute,
		time.Hour,
		6 * time.Hour,
		24 * time.Hour,
	}
	
	currentIndex := 0
	for i, r := range ranges {
		if r == ms.graphTimeRange {
			currentIndex = i
			break
		}
	}
	
	nextIndex := (currentIndex + 1) % len(ranges)
	ms.graphTimeRange = ranges[nextIndex]
}

// Data collection and analysis

// collectPerformanceData collects current performance data from all nodes
func (ms *MonitoringSystem) collectPerformanceData() {
	nodes := ms.nodeManager.GetAllNodes()
	timestamp := time.Now()
	
	for _, node := range nodes {
		if node.Status != NodeStatusOffline {
			snapshot := PerformanceSnapshot{
				Timestamp:       timestamp,
				NodeID:          node.NodeID,
				CPUUsage:        node.CPUUsage,
				MemoryUsage:     node.MemoryUsage,
				NetworkIn:       ms.calculateNetworkIn(node),
				NetworkOut:      ms.calculateNetworkOut(node),
				DiskIO:          ms.calculateDiskIO(node),
				ResponseTime:    ms.calculateResponseTime(node),
				ErrorRate:       ms.calculateErrorRate(node),
				SessionCount:    ms.getSessionCount(node),
				IdleTime:        node.IdleTime,
				ConnectionCount: ms.getConnectionCount(node),
				BytesTransferred: node.BytesSent + node.BytesReceived,
			}
			
			// Add to performance data
			if _, exists := ms.performanceData[node.NodeID]; !exists {
				ms.performanceData[node.NodeID] = make([]PerformanceSnapshot, 0)
			}
			
			ms.performanceData[node.NodeID] = append(ms.performanceData[node.NodeID], snapshot)
			
			// Limit data points to prevent memory growth
			if len(ms.performanceData[node.NodeID]) > ms.maxDataPoints {
				ms.performanceData[node.NodeID] = ms.performanceData[node.NodeID][1:]
			}
		}
	}
}

// Helper methods for calculating metrics
func (ms *MonitoringSystem) calculateNetworkIn(node *NodeInfo) int64 {
	// Placeholder - would calculate actual network input rate
	return int64(node.NodeID * 1024) // Fake data
}

func (ms *MonitoringSystem) calculateNetworkOut(node *NodeInfo) int64 {
	// Placeholder - would calculate actual network output rate
	return int64(node.NodeID * 512) // Fake data
}

func (ms *MonitoringSystem) calculateDiskIO(node *NodeInfo) int64 {
	// Placeholder - would calculate actual disk I/O rate
	return int64(node.NodeID * 256) // Fake data
}

func (ms *MonitoringSystem) calculateResponseTime(node *NodeInfo) time.Duration {
	// Placeholder - would calculate actual response time
	baseTime := 50 * time.Millisecond
	variability := time.Duration(node.NodeID*10) * time.Millisecond
	return baseTime + variability
}

func (ms *MonitoringSystem) calculateErrorRate(node *NodeInfo) float64 {
	// Placeholder - would calculate actual error rate
	if node.CPUUsage > 80 {
		return float64(node.NodeID) * 0.5 // Higher errors with high CPU
	}
	return 0.1
}

func (ms *MonitoringSystem) getSessionCount(node *NodeInfo) int {
	if node.User != nil {
		return 1
	}
	return 0
}

func (ms *MonitoringSystem) getConnectionCount(node *NodeInfo) int {
	// Would get from node statistics
	stats, err := ms.nodeManager.GetNodeStatistics(node.NodeID)
	if err == nil {
		return int(stats.TotalConnections)
	}
	return 0
}

// Alert processing

// checkAlertConditions checks current metrics against alert thresholds
func (ms *MonitoringSystem) checkAlertConditions() {
	nodes := ms.nodeManager.GetAllNodes()
	
	for _, node := range nodes {
		if node.Status == NodeStatusOffline {
			continue
		}
		
		// CPU usage alerts
		ms.checkCPUAlert(node)
		
		// Memory usage alerts
		ms.checkMemoryAlert(node)
		
		// Response time alerts
		ms.checkResponseTimeAlert(node)
		
		// Error rate alerts
		ms.checkErrorRateAlert(node)
		
		// Idle time alerts
		ms.checkIdleTimeAlert(node)
		
		// Connection status alerts
		ms.checkConnectionAlert(node)
	}
}

func (ms *MonitoringSystem) checkCPUAlert(node *NodeInfo) {
	if node.CPUUsage >= ms.alertThresholds.CPUCritical {
		ms.createAlert(node.NodeID, "critical", "cpu", 
			fmt.Sprintf("Critical CPU usage: %.1f%%", node.CPUUsage),
			node.CPUUsage, ms.alertThresholds.CPUCritical, 4)
	} else if node.CPUUsage >= ms.alertThresholds.CPUWarning {
		ms.createAlert(node.NodeID, "warning", "cpu",
			fmt.Sprintf("High CPU usage: %.1f%%", node.CPUUsage),
			node.CPUUsage, ms.alertThresholds.CPUWarning, 2)
	}
}

func (ms *MonitoringSystem) checkMemoryAlert(node *NodeInfo) {
	if node.MemoryUsage >= ms.alertThresholds.MemoryCritical {
		ms.createAlert(node.NodeID, "critical", "memory",
			fmt.Sprintf("Critical memory usage: %d MB", node.MemoryUsage/(1024*1024)),
			node.MemoryUsage, ms.alertThresholds.MemoryCritical, 4)
	} else if node.MemoryUsage >= ms.alertThresholds.MemoryWarning {
		ms.createAlert(node.NodeID, "warning", "memory",
			fmt.Sprintf("High memory usage: %d MB", node.MemoryUsage/(1024*1024)),
			node.MemoryUsage, ms.alertThresholds.MemoryWarning, 2)
	}
}

func (ms *MonitoringSystem) checkResponseTimeAlert(node *NodeInfo) {
	responseTime := ms.calculateResponseTime(node)
	if responseTime >= ms.alertThresholds.ResponseTimeCritical {
		ms.createAlert(node.NodeID, "critical", "response_time",
			fmt.Sprintf("Critical response time: %s", responseTime),
			responseTime, ms.alertThresholds.ResponseTimeCritical, 4)
	} else if responseTime >= ms.alertThresholds.ResponseTimeWarning {
		ms.createAlert(node.NodeID, "warning", "response_time",
			fmt.Sprintf("High response time: %s", responseTime),
			responseTime, ms.alertThresholds.ResponseTimeWarning, 2)
	}
}

func (ms *MonitoringSystem) checkErrorRateAlert(node *NodeInfo) {
	errorRate := ms.calculateErrorRate(node)
	if errorRate >= ms.alertThresholds.ErrorRateCritical {
		ms.createAlert(node.NodeID, "critical", "error_rate",
			fmt.Sprintf("Critical error rate: %.1f/min", errorRate),
			errorRate, ms.alertThresholds.ErrorRateCritical, 4)
	} else if errorRate >= ms.alertThresholds.ErrorRateWarning {
		ms.createAlert(node.NodeID, "warning", "error_rate",
			fmt.Sprintf("High error rate: %.1f/min", errorRate),
			errorRate, ms.alertThresholds.ErrorRateWarning, 2)
	}
}

func (ms *MonitoringSystem) checkIdleTimeAlert(node *NodeInfo) {
	if node.User != nil && node.IdleTime >= ms.alertThresholds.IdleTimeWarning {
		ms.createAlert(node.NodeID, "info", "idle_time",
			fmt.Sprintf("User idle for %s", formatDuration(node.IdleTime)),
			node.IdleTime, ms.alertThresholds.IdleTimeWarning, 1)
	}
}

func (ms *MonitoringSystem) checkConnectionAlert(node *NodeInfo) {
	if node.Config.Enabled && node.Status == NodeStatusOffline {
		ms.createAlert(node.NodeID, "critical", "connection",
			"Node is offline but should be enabled",
			"offline", "online", 4)
	}
}

// createAlert creates a new alert record
func (ms *MonitoringSystem) createAlert(nodeID int, alertType, category, message string, 
	value, threshold interface{}, severity int) {
	
	// Check if similar alert already exists and is recent
	if ms.isDuplicateAlert(nodeID, category, alertType) {
		return
	}
	
	alert := AlertRecord{
		ID:        len(ms.alertHistory) + 1,
		NodeID:    nodeID,
		AlertType: alertType,
		Category:  category,
		Message:   message,
		Value:     value,
		Threshold: threshold,
		Timestamp: time.Now(),
		Severity:  severity,
	}
	
	ms.alertHistory = append(ms.alertHistory, alert)
	
	// Send to node manager as well
	nodeAlert := NodeAlert{
		NodeID:    nodeID,
		AlertType: alertType,
		Message:   message,
		Timestamp: time.Now(),
		AutoClear: alertType != "critical",
	}
	
	ms.nodeManager.AddAlert(nodeAlert)
}

// isDuplicateAlert checks if a similar alert was recently created
func (ms *MonitoringSystem) isDuplicateAlert(nodeID int, category, alertType string) bool {
	now := time.Now()
	duplicateWindow := 5 * time.Minute
	
	for _, alert := range ms.alertHistory {
		if alert.NodeID == nodeID && 
		   alert.Category == category && 
		   alert.AlertType == alertType &&
		   !alert.Acknowledged &&
		   now.Sub(alert.Timestamp) < duplicateWindow {
			return true
		}
	}
	return false
}

// acknowledgeSelectedAlert acknowledges the selected alert
func (ms *MonitoringSystem) acknowledgeSelectedAlert() {
	// Implementation would depend on how alerts are selected in the UI
	// For now, acknowledge the most recent unacknowledged alert
	for i := len(ms.alertHistory) - 1; i >= 0; i-- {
		if !ms.alertHistory[i].Acknowledged {
			ms.alertHistory[i].Acknowledged = true
			ms.alertHistory[i].AckBy = "SysOp"
			ms.alertHistory[i].AckTime = time.Now()
			break
		}
	}
}

// clearSelectedAlert clears the selected alert
func (ms *MonitoringSystem) clearSelectedAlert() {
	for i := len(ms.alertHistory) - 1; i >= 0; i-- {
		if !ms.alertHistory[i].AutoCleared {
			ms.alertHistory[i].AutoCleared = true
			ms.alertHistory[i].ClearTime = time.Now()
			ms.alertHistory[i].Duration = time.Since(ms.alertHistory[i].Timestamp)
			break
		}
	}
}

// getMaxNodeID returns the maximum node ID
func (ms *MonitoringSystem) getMaxNodeID() int {
	nodes := ms.nodeManager.GetAllNodes()
	maxID := 1
	for _, node := range nodes {
		if node.NodeID > maxID {
			maxID = node.NodeID
		}
	}
	return maxID
}

// View rendering will be continued in the next file...