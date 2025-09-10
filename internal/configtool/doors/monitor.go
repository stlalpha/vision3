package doors

import (
	"fmt"
	"sync"
	"time"
	"context"
	"os"
	"path/filepath"
	"encoding/json"
	"sort"
	"strings"
	"runtime"
	"syscall"
)

// DoorMonitor monitors door instances and system health
type DoorMonitor struct {
	config          *MonitorConfig
	instances       map[string]*MonitoredInstance
	metrics         map[string]*DoorMetrics
	alerts          chan *DoorAlert
	events          chan *MonitorEvent
	stopChan        chan bool
	mu              sync.RWMutex
	running         bool
	lastUpdate      time.Time
	systemMetrics   *SystemMetrics
}

// MonitorConfig contains configuration for door monitoring
type MonitorConfig struct {
	UpdateInterval     time.Duration         `json:"update_interval"`      // How often to update metrics
	AlertThresholds    *AlertThresholds      `json:"alert_thresholds"`     // Alert threshold configuration
	MetricsRetention   time.Duration         `json:"metrics_retention"`    // How long to keep metrics
	EnablePerformance  bool                  `json:"enable_performance"`   // Enable performance monitoring
	EnableResourceMon  bool                  `json:"enable_resource_mon"`  // Enable resource monitoring
	EnableHealthCheck  bool                  `json:"enable_health_check"`  // Enable health checks
	LogPath            string                `json:"log_path"`             // Path for monitor logs
	MetricsPath        string                `json:"metrics_path"`         // Path for metrics storage
	MaxMetricsFiles    int                   `json:"max_metrics_files"`    // Maximum metrics files to keep
	ProcessTracking    bool                  `json:"process_tracking"`     // Track door processes
	NetworkMonitoring  bool                  `json:"network_monitoring"`   // Monitor network usage
	DiskMonitoring     bool                  `json:"disk_monitoring"`      // Monitor disk usage
}

// AlertThresholds defines thresholds for generating alerts
type AlertThresholds struct {
	MaxMemoryMB        int           `json:"max_memory_mb"`        // Maximum memory usage in MB
	MaxCPUPercent      float64       `json:"max_cpu_percent"`      // Maximum CPU usage percentage
	MaxRuntimeMinutes  int           `json:"max_runtime_minutes"`  // Maximum runtime in minutes
	MinResponseTime    time.Duration `json:"min_response_time"`    // Minimum acceptable response time
	MaxResponseTime    time.Duration `json:"max_response_time"`    // Maximum acceptable response time
	MaxErrorRate       float64       `json:"max_error_rate"`       // Maximum error rate (0-1)
	MaxCrashCount      int           `json:"max_crash_count"`      // Maximum crashes per day
	DiskSpaceThreshold int64         `json:"disk_space_threshold"` // Minimum disk space in bytes
	NetworkTimeoutSec  int           `json:"network_timeout_sec"`  // Network timeout in seconds
}

// MonitoredInstance represents an instance being monitored
type MonitoredInstance struct {
	InstanceID      string                 `json:"instance_id"`      // Instance ID
	DoorID          string                 `json:"door_id"`          // Door ID
	NodeID          int                    `json:"node_id"`          // Node ID
	UserID          int                    `json:"user_id"`          // User ID
	ProcessID       int                    `json:"process_id"`       // Process ID
	StartTime       time.Time              `json:"start_time"`       // Start time
	LastSeen        time.Time              `json:"last_seen"`        // Last seen time
	Status          InstanceStatus         `json:"status"`           // Current status
	Metrics         *InstanceMetrics       `json:"metrics"`          // Current metrics
	HealthStatus    HealthStatus           `json:"health_status"`    // Health status
	AlertCount      int                    `json:"alert_count"`      // Number of alerts generated
	LastAlert       time.Time              `json:"last_alert"`       // Last alert time
	Metadata        map[string]interface{} `json:"metadata"`         // Additional metadata
}

// InstanceMetrics contains metrics for a door instance
type InstanceMetrics struct {
	// Performance metrics
	CPUUsage        float64       `json:"cpu_usage"`         // CPU usage percentage
	MemoryUsage     int64         `json:"memory_usage"`      // Memory usage in bytes
	MemoryPercent   float64       `json:"memory_percent"`    // Memory usage percentage
	DiskRead        int64         `json:"disk_read"`         // Disk read bytes
	DiskWrite       int64         `json:"disk_write"`        // Disk write bytes
	NetworkIn       int64         `json:"network_in"`        // Network bytes in
	NetworkOut      int64         `json:"network_out"`       // Network bytes out
	
	// Application metrics
	ResponseTime    time.Duration `json:"response_time"`     // Average response time
	RequestCount    int64         `json:"request_count"`     // Total requests handled
	ErrorCount      int64         `json:"error_count"`       // Number of errors
	ErrorRate       float64       `json:"error_rate"`        // Error rate (0-1)
	Uptime          time.Duration `json:"uptime"`            // Instance uptime
	
	// File system metrics
	FilesOpen       int           `json:"files_open"`        // Number of open files
	FilesCreated    int           `json:"files_created"`     // Files created this session
	FilesModified   int           `json:"files_modified"`    // Files modified this session
	
	// Connection metrics
	Connections     int           `json:"connections"`       // Active connections
	MaxConnections  int           `json:"max_connections"`   // Maximum concurrent connections
	
	// Custom metrics
	CustomMetrics   map[string]float64 `json:"custom_metrics"` // Custom application metrics
}

// DoorMetrics contains aggregated metrics for a door
type DoorMetrics struct {
	DoorID           string                    `json:"door_id"`            // Door ID
	TotalInstances   int                       `json:"total_instances"`    // Total instances launched
	ActiveInstances  int                       `json:"active_instances"`   // Currently active instances
	AverageRuntime   time.Duration             `json:"average_runtime"`    // Average runtime
	TotalRuntime     time.Duration             `json:"total_runtime"`      // Total runtime
	CrashCount       int                       `json:"crash_count"`        // Number of crashes
	ErrorCount       int64                     `json:"error_count"`        // Total errors
	SuccessRate      float64                   `json:"success_rate"`       // Success rate (0-1)
	AverageMemory    int64                     `json:"average_memory"`     // Average memory usage
	PeakMemory       int64                     `json:"peak_memory"`        // Peak memory usage
	AverageCPU       float64                   `json:"average_cpu"`        // Average CPU usage
	PeakCPU          float64                   `json:"peak_cpu"`           // Peak CPU usage
	TotalDiskIO      int64                     `json:"total_disk_io"`      // Total disk I/O
	TotalNetworkIO   int64                     `json:"total_network_io"`   // Total network I/O
	LastUpdate       time.Time                 `json:"last_update"`        // Last update time
	DailyMetrics     []DailyMetrics            `json:"daily_metrics"`      // Daily metrics history
	HourlyMetrics    []HourlyMetrics           `json:"hourly_metrics"`     // Hourly metrics history
}

// DailyMetrics contains metrics aggregated by day
type DailyMetrics struct {
	Date         time.Time `json:"date"`          // Date
	Instances    int       `json:"instances"`     // Instances launched
	Crashes      int       `json:"crashes"`       // Crashes occurred
	TotalRuntime time.Duration `json:"total_runtime"` // Total runtime
	ErrorCount   int64     `json:"error_count"`   // Errors occurred
	AvgMemory    int64     `json:"avg_memory"`    // Average memory usage
	AvgCPU       float64   `json:"avg_cpu"`       // Average CPU usage
}

// HourlyMetrics contains metrics aggregated by hour
type HourlyMetrics struct {
	Hour         time.Time `json:"hour"`          // Hour timestamp
	Instances    int       `json:"instances"`     // Instances active during hour
	AvgMemory    int64     `json:"avg_memory"`    // Average memory usage
	AvgCPU       float64   `json:"avg_cpu"`       // Average CPU usage
	ErrorCount   int64     `json:"error_count"`   // Errors during hour
	ResponseTime time.Duration `json:"response_time"` // Average response time
}

// SystemMetrics contains system-wide metrics
type SystemMetrics struct {
	TotalMemory      int64                  `json:"total_memory"`       // Total system memory
	AvailableMemory  int64                  `json:"available_memory"`   // Available memory
	MemoryUsage      float64                `json:"memory_usage"`       // Memory usage percentage
	CPUCount         int                    `json:"cpu_count"`          // Number of CPUs
	CPUUsage         float64                `json:"cpu_usage"`          // System CPU usage
	LoadAverage      []float64              `json:"load_average"`       // System load average
	DiskSpace        map[string]DiskSpace   `json:"disk_space"`         // Disk space by mount point
	NetworkStats     map[string]NetworkStat `json:"network_stats"`      // Network statistics
	ActiveProcesses  int                    `json:"active_processes"`   // Active processes
	SystemUptime     time.Duration          `json:"system_uptime"`      // System uptime
	LastUpdate       time.Time              `json:"last_update"`        // Last update time
}

// DiskSpace contains disk space information
type DiskSpace struct {
	Total     int64   `json:"total"`      // Total space in bytes
	Used      int64   `json:"used"`       // Used space in bytes
	Available int64   `json:"available"`  // Available space in bytes
	UsagePercent float64 `json:"usage_percent"` // Usage percentage
}

// NetworkStat contains network statistics
type NetworkStat struct {
	Interface    string `json:"interface"`     // Network interface name
	BytesIn      int64  `json:"bytes_in"`      // Bytes received
	BytesOut     int64  `json:"bytes_out"`     // Bytes sent
	PacketsIn    int64  `json:"packets_in"`    // Packets received
	PacketsOut   int64  `json:"packets_out"`   // Packets sent
	ErrorsIn     int64  `json:"errors_in"`     // Input errors
	ErrorsOut    int64  `json:"errors_out"`    // Output errors
}

// HealthStatus represents the health status of an instance
type HealthStatus int

const (
	HealthStatusUnknown HealthStatus = iota
	HealthStatusHealthy
	HealthStatusWarning
	HealthStatusCritical
	HealthStatusFailed
)

func (hs HealthStatus) String() string {
	switch hs {
	case HealthStatusHealthy:
		return "Healthy"
	case HealthStatusWarning:
		return "Warning"
	case HealthStatusCritical:
		return "Critical"
	case HealthStatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// MonitorEvent represents a monitoring event
type MonitorEvent struct {
	Type        MonitorEventType `json:"type"`        // Event type
	Timestamp   time.Time        `json:"timestamp"`   // Event timestamp
	InstanceID  string           `json:"instance_id"` // Instance ID
	DoorID      string           `json:"door_id"`     // Door ID
	Message     string           `json:"message"`     // Event message
	Data        interface{}      `json:"data"`        // Event data
	Severity    AlertSeverity    `json:"severity"`    // Event severity
}

// MonitorEventType represents the type of monitor event
type MonitorEventType int

const (
	EventInstanceStarted MonitorEventType = iota
	EventInstanceStopped
	EventInstanceCrashed
	EventInstanceHung
	EventThresholdExceeded
	EventHealthCheck
	EventMetricsUpdated
	EventSystemAlert
)

func (met MonitorEventType) String() string {
	switch met {
	case EventInstanceStarted:
		return "Instance Started"
	case EventInstanceStopped:
		return "Instance Stopped"
	case EventInstanceCrashed:
		return "Instance Crashed"
	case EventInstanceHung:
		return "Instance Hung"
	case EventThresholdExceeded:
		return "Threshold Exceeded"
	case EventHealthCheck:
		return "Health Check"
	case EventMetricsUpdated:
		return "Metrics Updated"
	case EventSystemAlert:
		return "System Alert"
	default:
		return "Unknown"
	}
}

// NewDoorMonitor creates a new door monitor
func NewDoorMonitor(config *MonitorConfig) *DoorMonitor {
	if config == nil {
		config = &MonitorConfig{
			UpdateInterval:    time.Second * 30,
			MetricsRetention:  time.Hour * 24 * 7, // 7 days
			EnablePerformance: true,
			EnableResourceMon: true,
			EnableHealthCheck: true,
			LogPath:           "/tmp/vision3/monitor",
			MetricsPath:       "/tmp/vision3/metrics",
			MaxMetricsFiles:   100,
			ProcessTracking:   true,
			NetworkMonitoring: true,
			DiskMonitoring:    true,
			AlertThresholds: &AlertThresholds{
				MaxMemoryMB:        512,
				MaxCPUPercent:      80.0,
				MaxRuntimeMinutes:  120,
				MinResponseTime:    time.Millisecond * 10,
				MaxResponseTime:    time.Second * 5,
				MaxErrorRate:       0.05,
				MaxCrashCount:      3,
				DiskSpaceThreshold: 1024 * 1024 * 1024, // 1GB
				NetworkTimeoutSec:  30,
			},
		}
	}
	
	monitor := &DoorMonitor{
		config:        config,
		instances:     make(map[string]*MonitoredInstance),
		metrics:       make(map[string]*DoorMetrics),
		alerts:        make(chan *DoorAlert, 100),
		events:        make(chan *MonitorEvent, 100),
		stopChan:      make(chan bool),
		running:       false,
		systemMetrics: &SystemMetrics{},
	}
	
	// Ensure directories exist
	os.MkdirAll(config.LogPath, 0755)
	os.MkdirAll(config.MetricsPath, 0755)
	
	return monitor
}

// Start starts the door monitor
func (dm *DoorMonitor) Start() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	if dm.running {
		return nil
	}
	
	dm.running = true
	
	// Start monitoring loops
	go dm.updateLoop()
	go dm.alertLoop()
	go dm.eventLoop()
	go dm.healthCheckLoop()
	go dm.metricsCleanupLoop()
	
	return nil
}

// Stop stops the door monitor
func (dm *DoorMonitor) Stop() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	if !dm.running {
		return nil
	}
	
	dm.running = false
	close(dm.stopChan)
	
	// Save current metrics
	dm.saveMetrics()
	
	return nil
}

// RegisterInstance registers an instance for monitoring
func (dm *DoorMonitor) RegisterInstance(instance *DoorInstance) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	monitored := &MonitoredInstance{
		InstanceID:   instance.ID,
		DoorID:       instance.DoorID,
		NodeID:       instance.NodeID,
		UserID:       instance.UserID,
		ProcessID:    instance.ProcessID,
		StartTime:    instance.StartTime,
		LastSeen:     time.Now(),
		Status:       instance.Status,
		HealthStatus: HealthStatusUnknown,
		Metrics: &InstanceMetrics{
			CustomMetrics: make(map[string]float64),
		},
		Metadata: make(map[string]interface{}),
	}
	
	dm.instances[instance.ID] = monitored
	
	// Initialize door metrics if not present
	if _, exists := dm.metrics[instance.DoorID]; !exists {
		dm.metrics[instance.DoorID] = &DoorMetrics{
			DoorID:        instance.DoorID,
			DailyMetrics:  make([]DailyMetrics, 0),
			HourlyMetrics: make([]HourlyMetrics, 0),
		}
	}
	
	// Emit event
	dm.emitEvent(&MonitorEvent{
		Type:       EventInstanceStarted,
		Timestamp:  time.Now(),
		InstanceID: instance.ID,
		DoorID:     instance.DoorID,
		Message:    "Instance registered for monitoring",
		Severity:   SeverityLow,
	})
	
	return nil
}

// UnregisterInstance unregisters an instance from monitoring
func (dm *DoorMonitor) UnregisterInstance(instanceID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	instance, exists := dm.instances[instanceID]
	if !exists {
		return fmt.Errorf("instance not found: %s", instanceID)
	}
	
	// Update door metrics
	if doorMetrics, exists := dm.metrics[instance.DoorID]; exists {
		doorMetrics.ActiveInstances--
		if doorMetrics.ActiveInstances < 0 {
			doorMetrics.ActiveInstances = 0
		}
	}
	
	// Remove instance
	delete(dm.instances, instanceID)
	
	// Emit event
	dm.emitEvent(&MonitorEvent{
		Type:       EventInstanceStopped,
		Timestamp:  time.Now(),
		InstanceID: instanceID,
		DoorID:     instance.DoorID,
		Message:    "Instance unregistered from monitoring",
		Severity:   SeverityLow,
	})
	
	return nil
}

// GetInstanceMetrics returns metrics for a specific instance
func (dm *DoorMonitor) GetInstanceMetrics(instanceID string) (*InstanceMetrics, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	instance, exists := dm.instances[instanceID]
	if !exists {
		return nil, fmt.Errorf("instance not found: %s", instanceID)
	}
	
	// Return a copy
	metricsCopy := *instance.Metrics
	return &metricsCopy, nil
}

// GetDoorMetrics returns aggregated metrics for a door
func (dm *DoorMonitor) GetDoorMetrics(doorID string) (*DoorMetrics, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	metrics, exists := dm.metrics[doorID]
	if !exists {
		return nil, fmt.Errorf("metrics not found for door: %s", doorID)
	}
	
	// Return a copy
	metricsCopy := *metrics
	return &metricsCopy, nil
}

// GetSystemMetrics returns system-wide metrics
func (dm *DoorMonitor) GetSystemMetrics() *SystemMetrics {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	// Return a copy
	metricsCopy := *dm.systemMetrics
	return &metricsCopy
}

// GetActiveInstances returns all currently monitored instances
func (dm *DoorMonitor) GetActiveInstances() []*MonitoredInstance {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	instances := make([]*MonitoredInstance, 0, len(dm.instances))
	for _, instance := range dm.instances {
		instanceCopy := *instance
		instances = append(instances, &instanceCopy)
	}
	
	return instances
}

// Background monitoring loops

func (dm *DoorMonitor) updateLoop() {
	ticker := time.NewTicker(dm.config.UpdateInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			dm.updateMetrics()
		case <-dm.stopChan:
			return
		}
	}
}

func (dm *DoorMonitor) updateMetrics() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	now := time.Now()
	
	// Update system metrics
	dm.updateSystemMetrics()
	
	// Update instance metrics
	for instanceID, instance := range dm.instances {
		if err := dm.updateInstanceMetrics(instance); err != nil {
			// Instance may have died, mark as failed
			instance.HealthStatus = HealthStatusFailed
			instance.LastSeen = now
			
			// Emit event
			dm.emitEvent(&MonitorEvent{
				Type:       EventInstanceCrashed,
				Timestamp:  now,
				InstanceID: instanceID,
				DoorID:     instance.DoorID,
				Message:    fmt.Sprintf("Instance update failed: %v", err),
				Severity:   SeverityHigh,
			})
		} else {
			instance.LastSeen = now
		}
		
		// Check thresholds
		dm.checkThresholds(instance)
	}
	
	// Update door metrics
	dm.updateDoorMetrics()
	
	dm.lastUpdate = now
	
	// Emit metrics updated event
	dm.emitEvent(&MonitorEvent{
		Type:      EventMetricsUpdated,
		Timestamp: now,
		Message:   "Metrics updated",
		Severity:  SeverityLow,
	})
}

func (dm *DoorMonitor) updateSystemMetrics() {
	now := time.Now()
	
	// Update basic system info
	dm.systemMetrics.CPUCount = runtime.NumCPU()
	dm.systemMetrics.LastUpdate = now
	
	// Get memory info (platform-specific implementation would go here)
	dm.systemMetrics.TotalMemory = dm.getTotalMemory()
	dm.systemMetrics.AvailableMemory = dm.getAvailableMemory()
	if dm.systemMetrics.TotalMemory > 0 {
		dm.systemMetrics.MemoryUsage = float64(dm.systemMetrics.TotalMemory-dm.systemMetrics.AvailableMemory) / float64(dm.systemMetrics.TotalMemory) * 100
	}
	
	// Get CPU usage (would need platform-specific implementation)
	dm.systemMetrics.CPUUsage = dm.getCPUUsage()
	
	// Get disk space
	if dm.config.DiskMonitoring {
		dm.systemMetrics.DiskSpace = dm.getDiskSpace()
	}
	
	// Get network stats
	if dm.config.NetworkMonitoring {
		dm.systemMetrics.NetworkStats = dm.getNetworkStats()
	}
	
	// Count active processes
	dm.systemMetrics.ActiveProcesses = len(dm.instances)
}

func (dm *DoorMonitor) updateInstanceMetrics(instance *MonitoredInstance) error {
	if instance.ProcessID <= 0 {
		return fmt.Errorf("invalid process ID")
	}
	
	// Check if process is still running
	if !dm.isProcessRunning(instance.ProcessID) {
		return fmt.Errorf("process not running")
	}
	
	metrics := instance.Metrics
	
	// Update performance metrics
	if dm.config.EnablePerformance {
		metrics.CPUUsage = dm.getProcessCPUUsage(instance.ProcessID)
		metrics.MemoryUsage = dm.getProcessMemoryUsage(instance.ProcessID)
		
		// Calculate memory percentage
		if dm.systemMetrics.TotalMemory > 0 {
			metrics.MemoryPercent = float64(metrics.MemoryUsage) / float64(dm.systemMetrics.TotalMemory) * 100
		}
	}
	
	// Update resource metrics
	if dm.config.EnableResourceMon {
		metrics.FilesOpen = dm.getProcessOpenFiles(instance.ProcessID)
		metrics.Connections = dm.getProcessConnections(instance.ProcessID)
	}
	
	// Update uptime
	metrics.Uptime = time.Since(instance.StartTime)
	
	// Calculate health status
	instance.HealthStatus = dm.calculateHealthStatus(instance)
	
	return nil
}

func (dm *DoorMonitor) updateDoorMetrics() {
	for doorID, doorMetrics := range dm.metrics {
		// Count active instances
		activeCount := 0
		totalMemory := int64(0)
		totalCPU := float64(0)
		
		for _, instance := range dm.instances {
			if instance.DoorID == doorID {
				activeCount++
				totalMemory += instance.Metrics.MemoryUsage
				totalCPU += instance.Metrics.CPUUsage
			}
		}
		
		doorMetrics.ActiveInstances = activeCount
		
		if activeCount > 0 {
			doorMetrics.AverageMemory = totalMemory / int64(activeCount)
			doorMetrics.AverageCPU = totalCPU / float64(activeCount)
		}
		
		doorMetrics.LastUpdate = time.Now()
		
		// Update peak values
		if totalMemory > doorMetrics.PeakMemory {
			doorMetrics.PeakMemory = totalMemory
		}
		if totalCPU > doorMetrics.PeakCPU {
			doorMetrics.PeakCPU = totalCPU
		}
	}
}

func (dm *DoorMonitor) checkThresholds(instance *MonitoredInstance) {
	thresholds := dm.config.AlertThresholds
	alerts := make([]*DoorAlert, 0)
	
	// Check memory threshold
	if thresholds.MaxMemoryMB > 0 && instance.Metrics.MemoryUsage > int64(thresholds.MaxMemoryMB)*1024*1024 {
		alert := &DoorAlert{
			ID:        fmt.Sprintf("mem_%s_%d", instance.InstanceID, time.Now().Unix()),
			DoorID:    instance.DoorID,
			InstanceID: instance.InstanceID,
			NodeID:    instance.NodeID,
			AlertType: AlertTypePerformance,
			Severity:  SeverityHigh,
			Message:   "Memory usage threshold exceeded",
			Details:   fmt.Sprintf("Memory usage: %.1f MB, Threshold: %d MB", float64(instance.Metrics.MemoryUsage)/1024/1024, thresholds.MaxMemoryMB),
			Timestamp: time.Now(),
			Actions:   []string{"Check for memory leaks", "Restart door instance", "Increase memory limits"},
		}
		alerts = append(alerts, alert)
	}
	
	// Check CPU threshold
	if thresholds.MaxCPUPercent > 0 && instance.Metrics.CPUUsage > thresholds.MaxCPUPercent {
		alert := &DoorAlert{
			ID:        fmt.Sprintf("cpu_%s_%d", instance.InstanceID, time.Now().Unix()),
			DoorID:    instance.DoorID,
			InstanceID: instance.InstanceID,
			NodeID:    instance.NodeID,
			AlertType: AlertTypePerformance,
			Severity:  SeverityMedium,
			Message:   "CPU usage threshold exceeded",
			Details:   fmt.Sprintf("CPU usage: %.1f%%, Threshold: %.1f%%", instance.Metrics.CPUUsage, thresholds.MaxCPUPercent),
			Timestamp: time.Now(),
			Actions:   []string{"Check for infinite loops", "Optimize door performance", "Check system load"},
		}
		alerts = append(alerts, alert)
	}
	
	// Check runtime threshold
	if thresholds.MaxRuntimeMinutes > 0 && instance.Metrics.Uptime > time.Duration(thresholds.MaxRuntimeMinutes)*time.Minute {
		alert := &DoorAlert{
			ID:        fmt.Sprintf("runtime_%s_%d", instance.InstanceID, time.Now().Unix()),
			DoorID:    instance.DoorID,
			InstanceID: instance.InstanceID,
			NodeID:    instance.NodeID,
			AlertType: AlertTypeTimeout,
			Severity:  SeverityMedium,
			Message:   "Maximum runtime exceeded",
			Details:   fmt.Sprintf("Runtime: %v, Threshold: %d minutes", instance.Metrics.Uptime, thresholds.MaxRuntimeMinutes),
			Timestamp: time.Now(),
			Actions:   []string{"Check if door is hung", "Force terminate if necessary", "Review door time limits"},
		}
		alerts = append(alerts, alert)
	}
	
	// Check error rate threshold
	if thresholds.MaxErrorRate > 0 && instance.Metrics.ErrorRate > thresholds.MaxErrorRate {
		alert := &DoorAlert{
			ID:        fmt.Sprintf("error_%s_%d", instance.InstanceID, time.Now().Unix()),
			DoorID:    instance.DoorID,
			InstanceID: instance.InstanceID,
			NodeID:    instance.NodeID,
			AlertType: AlertTypeError,
			Severity:  SeverityHigh,
			Message:   "Error rate threshold exceeded",
			Details:   fmt.Sprintf("Error rate: %.2f%%, Threshold: %.2f%%", instance.Metrics.ErrorRate*100, thresholds.MaxErrorRate*100),
			Timestamp: time.Now(),
			Actions:   []string{"Check door logs", "Review error patterns", "Update door configuration"},
		}
		alerts = append(alerts, alert)
	}
	
	// Send alerts
	for _, alert := range alerts {
		select {
		case dm.alerts <- alert:
			instance.AlertCount++
			instance.LastAlert = time.Now()
		default:
			// Alert channel full
		}
	}
	
	// Emit threshold exceeded event if any alerts were generated
	if len(alerts) > 0 {
		dm.emitEvent(&MonitorEvent{
			Type:       EventThresholdExceeded,
			Timestamp:  time.Now(),
			InstanceID: instance.InstanceID,
			DoorID:     instance.DoorID,
			Message:    fmt.Sprintf("%d thresholds exceeded", len(alerts)),
			Data:       alerts,
			Severity:   SeverityHigh,
		})
	}
}

func (dm *DoorMonitor) calculateHealthStatus(instance *MonitoredInstance) HealthStatus {
	thresholds := dm.config.AlertThresholds
	
	// Check critical conditions
	if instance.Metrics.ErrorRate > thresholds.MaxErrorRate*2 {
		return HealthStatusFailed
	}
	
	if instance.Metrics.CPUUsage > thresholds.MaxCPUPercent*1.5 {
		return HealthStatusCritical
	}
	
	if instance.Metrics.MemoryUsage > int64(thresholds.MaxMemoryMB)*1024*1024*2 {
		return HealthStatusCritical
	}
	
	// Check warning conditions
	warningCount := 0
	
	if instance.Metrics.CPUUsage > thresholds.MaxCPUPercent {
		warningCount++
	}
	
	if instance.Metrics.MemoryUsage > int64(thresholds.MaxMemoryMB)*1024*1024 {
		warningCount++
	}
	
	if instance.Metrics.ErrorRate > thresholds.MaxErrorRate {
		warningCount++
	}
	
	if warningCount >= 2 {
		return HealthStatusCritical
	} else if warningCount >= 1 {
		return HealthStatusWarning
	}
	
	return HealthStatusHealthy
}

func (dm *DoorMonitor) alertLoop() {
	for {
		select {
		case alert := <-dm.alerts:
			dm.processAlert(alert)
		case <-dm.stopChan:
			return
		}
	}
}

func (dm *DoorMonitor) eventLoop() {
	for {
		select {
		case event := <-dm.events:
			dm.processEvent(event)
		case <-dm.stopChan:
			return
		}
	}
}

func (dm *DoorMonitor) healthCheckLoop() {
	if !dm.config.EnableHealthCheck {
		return
	}
	
	ticker := time.NewTicker(time.Minute * 5) // Health check every 5 minutes
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			dm.performHealthChecks()
		case <-dm.stopChan:
			return
		}
	}
}

func (dm *DoorMonitor) metricsCleanupLoop() {
	ticker := time.NewTicker(time.Hour) // Cleanup every hour
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			dm.cleanupOldMetrics()
		case <-dm.stopChan:
			return
		}
	}
}

func (dm *DoorMonitor) performHealthChecks() {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	for instanceID, instance := range dm.instances {
		// Check if instance is responsive
		if time.Since(instance.LastSeen) > time.Minute*2 {
			// Instance hasn't been seen recently, mark as potentially hung
			dm.emitEvent(&MonitorEvent{
				Type:       EventInstanceHung,
				Timestamp:  time.Now(),
				InstanceID: instanceID,
				DoorID:     instance.DoorID,
				Message:    "Instance appears to be hung",
				Severity:   SeverityHigh,
			})
		}
		
		// Perform health check
		healthStatus := dm.calculateHealthStatus(instance)
		if healthStatus != instance.HealthStatus {
			instance.HealthStatus = healthStatus
			
			dm.emitEvent(&MonitorEvent{
				Type:       EventHealthCheck,
				Timestamp:  time.Now(),
				InstanceID: instanceID,
				DoorID:     instance.DoorID,
				Message:    fmt.Sprintf("Health status changed to %s", healthStatus.String()),
				Data:       healthStatus,
				Severity:   SeverityMedium,
			})
		}
	}
}

func (dm *DoorMonitor) cleanupOldMetrics() {
	// Clean up old metrics files
	entries, err := os.ReadDir(dm.config.MetricsPath)
	if err != nil {
		return
	}
	
	var metricsFiles []os.DirEntry
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			metricsFiles = append(metricsFiles, entry)
		}
	}
	
	if len(metricsFiles) <= dm.config.MaxMetricsFiles {
		return
	}
	
	// Sort by modification time
	sort.Slice(metricsFiles, func(i, j int) bool {
		infoI, _ := metricsFiles[i].Info()
		infoJ, _ := metricsFiles[j].Info()
		return infoI.ModTime().Before(infoJ.ModTime())
	})
	
	// Remove oldest files
	toRemove := len(metricsFiles) - dm.config.MaxMetricsFiles
	for i := 0; i < toRemove; i++ {
		filePath := filepath.Join(dm.config.MetricsPath, metricsFiles[i].Name())
		os.Remove(filePath)
	}
}

func (dm *DoorMonitor) processAlert(alert *DoorAlert) {
	// TODO: Process alert (log, send notification, etc.)
	fmt.Printf("[ALERT] %s: %s\n", alert.AlertType.String(), alert.Message)
}

func (dm *DoorMonitor) processEvent(event *MonitorEvent) {
	// TODO: Process event (log, metrics, etc.)
	fmt.Printf("[EVENT] %s: %s\n", event.Type.String(), event.Message)
}

func (dm *DoorMonitor) emitEvent(event *MonitorEvent) {
	select {
	case dm.events <- event:
		// Event sent
	default:
		// Event channel full, drop event
	}
}

func (dm *DoorMonitor) saveMetrics() {
	timestamp := time.Now().Format("20060102_150405")
	metricsFile := filepath.Join(dm.config.MetricsPath, fmt.Sprintf("metrics_%s.json", timestamp))
	
	dm.mu.RLock()
	metricsData := map[string]interface{}{
		"timestamp":      time.Now(),
		"system_metrics": dm.systemMetrics,
		"door_metrics":   dm.metrics,
		"instances":      dm.instances,
	}
	dm.mu.RUnlock()
	
	data, err := json.MarshalIndent(metricsData, "", "  ")
	if err != nil {
		return
	}
	
	os.WriteFile(metricsFile, data, 0644)
}

// Platform-specific implementations (stubs for now)
// These would need to be implemented for each target platform

func (dm *DoorMonitor) getTotalMemory() int64 {
	// Platform-specific implementation needed
	return 8 * 1024 * 1024 * 1024 // 8GB default
}

func (dm *DoorMonitor) getAvailableMemory() int64 {
	// Platform-specific implementation needed
	return 4 * 1024 * 1024 * 1024 // 4GB default
}

func (dm *DoorMonitor) getCPUUsage() float64 {
	// Platform-specific implementation needed
	return 25.0 // 25% default
}

func (dm *DoorMonitor) getDiskSpace() map[string]DiskSpace {
	// Platform-specific implementation needed
	return map[string]DiskSpace{
		"/": {
			Total:        100 * 1024 * 1024 * 1024, // 100GB
			Used:         50 * 1024 * 1024 * 1024,  // 50GB
			Available:    50 * 1024 * 1024 * 1024,  // 50GB
			UsagePercent: 50.0,
		},
	}
}

func (dm *DoorMonitor) getNetworkStats() map[string]NetworkStat {
	// Platform-specific implementation needed
	return map[string]NetworkStat{
		"eth0": {
			Interface: "eth0",
			BytesIn:   1024 * 1024,
			BytesOut:  512 * 1024,
		},
	}
}

func (dm *DoorMonitor) isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	
	// Platform-specific implementation
	if runtime.GOOS == "windows" {
		// Windows implementation
		return true // Stub
	} else {
		// Unix-like implementation
		process, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		
		// Send signal 0 to check if process exists
		err = process.Signal(syscall.Signal(0))
		return err == nil
	}
}

func (dm *DoorMonitor) getProcessCPUUsage(pid int) float64 {
	// Platform-specific implementation needed
	return 10.0 // 10% default
}

func (dm *DoorMonitor) getProcessMemoryUsage(pid int) int64 {
	// Platform-specific implementation needed
	return 64 * 1024 * 1024 // 64MB default
}

func (dm *DoorMonitor) getProcessOpenFiles(pid int) int {
	// Platform-specific implementation needed
	return 10 // 10 files default
}

func (dm *DoorMonitor) getProcessConnections(pid int) int {
	// Platform-specific implementation needed
	return 1 // 1 connection default
}