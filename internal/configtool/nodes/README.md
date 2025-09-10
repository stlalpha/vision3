# ViSiON/3 Multi-Node Monitoring and Management System

This package provides a comprehensive multi-node monitoring and management system for the ViSiON/3 BBS, designed to recreate the authentic multi-user BBS experience with modern reliability and monitoring capabilities.

## Features

### 🖥️ Node Status Monitoring
- Real-time node activity display showing who's online and what they're doing
- Classic BBS-style "Who's Online" display with Turbo Pascal aesthetics
- Multiple display modes: Grid, List, Detailed, Table, and Graphical views
- Node performance statistics and connection tracking
- Live updating status with configurable refresh rates

### ⚙️ Multi-User Configuration Management
- Node-specific configurations (different menus, access levels per node)
- Concurrent user limits and node licensing
- Time limits and resource allocation per node
- Network protocol configuration (telnet, SSH, rlogin)
- Door game support settings with resource sharing

### 📊 Real-Time Monitoring Interface
- Live updating node status grid like classic BBS sysop monitors
- Performance graphs with multiple visualization types
- Resource usage monitoring (CPU, memory, network I/O per node)
- Historical performance data collection and analysis
- Configurable alert system for node problems or security issues

### 🛠️ Sysop Tools
- Send messages to specific nodes or broadcast to all users
- Force user disconnection with detailed logging
- Monitor user activities and chat conversations
- Set node availability (enable/disable nodes dynamically)
- Emergency shutdown capabilities

### 💬 Inter-Node Communication
- Private chat between users on different nodes
- Channel-based chat rooms (General, SysOp, Games)
- Page system for urgent communications
- Chat command system (/who, /time, /help, etc.)
- Away status and custom away messages

### 🚨 Performance Monitoring & Alerting
- Real-time performance metrics collection
- Configurable alert thresholds for CPU, memory, response time
- Alert management with acknowledgment and auto-clearing
- Historical trend analysis and reporting
- Visual performance graphs and charts

## Package Structure

```
internal/configtool/nodes/
├── types.go              # Core types and interfaces
├── manager.go            # Node manager implementation
├── manager_io.go         # File I/O and persistence
├── display.go            # Real-time node status display
├── config.go             # Node configuration management
├── sysop.go              # Sysop tools interface
├── sysop_render.go       # Sysop tools rendering
├── chat.go               # Inter-node chat system
├── chat_render.go        # Chat system rendering
├── whoonline.go          # Classic "Who's Online" display
├── monitoring.go         # Performance monitoring system
├── monitoring_render.go  # Monitoring interface rendering
├── monitoring_helpers.go # Monitoring utility functions
├── integration.go        # Session/user management integration
└── README.md            # This documentation
```

## Quick Start

### Basic Setup

```go
package main

import (
    "log"
    "github.com/stlalpha/vision3/internal/configtool/nodes"
)

func main() {
    // Create integration with session and user management
    integration, err := nodes.CreateDefaultIntegration("./data")
    if err != nil {
        log.Fatal(err)
    }
    defer integration.Stop()

    // Get the node manager
    nodeManager := integration.GetNodeManager()
    
    // Example: Get system status
    status, err := nodeManager.GetSystemStatus()
    if err != nil {
        log.Printf("Error getting system status: %v", err)
        return
    }
    
    log.Printf("System has %d active nodes with %d users online", 
        status.ActiveNodes, status.ConnectedUsers)
}
```

### Session Management

```go
// Register a new session
wrapper, err := integration.RegisterSession(bbsSession)
if err != nil {
    log.Printf("Failed to register session: %v", err)
    return
}

// Handle user login
user, err := wrapper.Login(username, password)
if err != nil {
    log.Printf("Login failed: %v", err)
    return
}

// Update activity
wrapper.SetActivity("menu", "Main Menu")
wrapper.SetActivity("door", "Legend of the Red Dragon")

// Send message to user
wrapper.SendMessage("SysOp", "Welcome to the BBS!")

// Logout and cleanup
wrapper.Logout()
integration.UnregisterSession(bbsSession)
```

### Node Configuration

```go
// Get node configuration
config, err := nodeManager.GetNodeConfig(1)
if err != nil {
    log.Printf("Error getting config: %v", err)
    return
}

// Modify configuration
config.TimeLimit = 90 * time.Minute
config.ChatEnabled = true
config.DoorSettings.AllowDoors = true

// Update configuration
err = nodeManager.UpdateNodeConfig(1, *config)
if err != nil {
    log.Printf("Error updating config: %v", err)
}
```

### Monitoring Setup

```go
// Create monitoring system
monitoring := nodes.NewMonitoringSystem(nodeManager, 80, 25)

// Run in Bubble Tea framework
p := tea.NewProgram(monitoring)
if err := p.Start(); err != nil {
    log.Fatal(err)
}
```

## User Interface Components

### Node Status Display

The node status display provides multiple views:

- **Grid View**: Compact cards showing node status, user, and key metrics
- **List View**: Tabular format with detailed information
- **Detailed View**: Multi-line entries with comprehensive information
- **Who's Online**: Classic BBS-style user listing

**Key Controls:**
- `TAB`: Switch view modes
- `S`: Change sorting (Node, Status, User, Activity, Time)
- `F`: Filter nodes (All, Active, Idle, etc.)
- `↑/↓`: Navigate nodes
- `R`: Refresh display

### Chat System

The inter-node chat system supports:

- **Private Chat**: One-on-one conversations between users
- **Channel Chat**: Group conversations in themed channels
- **Page System**: Urgent messaging system
- **Away Mode**: Status management for idle users

**Chat Commands:**
- `/me <action>`: Send action message
- `/who`: List online users
- `/time`: Show current time
- `/help`: Show available commands
- `/clear`: Clear chat history

### Sysop Tools

Comprehensive system administration tools:

- **Node Control**: Enable/disable nodes, restart, force disconnect
- **Messaging**: Send messages to users, broadcast announcements
- **Monitoring**: Real-time node status and performance
- **Emergency Tools**: System shutdown, maintenance mode

**Tool Categories:**
- `F1`: Node Control - Direct node management
- `F2`: Messaging - User communication tools
- `F3`: Monitor - Real-time system monitoring
- `F4`: Log View - System event logs
- `F5`: Statistics - Performance statistics
- `F12`: Emergency - Emergency system controls

### Performance Monitoring

Advanced monitoring with multiple visualization types:

- **Overview**: System-wide performance summary
- **Node Detail**: Individual node metrics and configuration
- **Performance Graphs**: Visual charts of historical data
- **Alerts**: Alert management and threshold configuration

**Graph Types:**
- Line graphs for trend analysis
- Bar graphs for discrete measurements
- Area graphs for cumulative data
- Real-time scrolling graphs

## Configuration

### Node Configuration Options

```go
type NodeConfiguration struct {
    NodeID          int           // Unique node identifier
    Name            string        // Friendly name
    Enabled         bool          // Node availability
    MaxUsers        int           // Concurrent user limit
    TimeLimit       time.Duration // Session time limit
    AccessLevel     int           // Minimum access level
    LocalNode       bool          // Local vs remote node
    ChatEnabled     bool          // Inter-node chat
    NetworkSettings NetworkConfig // Network configuration
    DoorSettings    DoorConfig    // Door game settings
}
```

### Alert Thresholds

```go
type AlertThresholds struct {
    CPUWarning          float64       // CPU usage warning %
    CPUCritical         float64       // CPU usage critical %
    MemoryWarning       int64         // Memory warning bytes
    MemoryCritical      int64         // Memory critical bytes
    ResponseTimeWarning time.Duration // Response time warning
    ErrorRateWarning    float64       // Error rate warning
    IdleTimeWarning     time.Duration // Idle time warning
}
```

## Integration Points

### Session Integration

The package integrates with the existing session management:

```go
// Wrap existing sessions
wrapper, err := integration.RegisterSession(bbsSession)

// Enhanced session with node management
wrapper.SetActivity("menu", "File Areas")
wrapper.SendMessage("SysOp", "System message")
nodeID := wrapper.GetNodeID()
onlineTime := wrapper.GetOnlineTime()
```

### User Management Integration

Seamlessly works with the existing user system:

```go
// Existing user manager is used internally
userManager := integration.GetUserManager()

// User authentication through node system
user, err := wrapper.Login(username, password)

// Call records are automatically created
// Statistics are automatically updated
```

## Event System

The system provides comprehensive event notifications:

```go
type IntegrationEventListener interface {
    OnSessionStart(nodeID int, session *session.BbsSession, user *user.User) error
    OnSessionEnd(nodeID int, session *session.BbsSession, user *user.User, duration time.Duration) error
    OnUserLogin(nodeID int, session *session.BbsSession, user *user.User) error
    OnUserLogout(nodeID int, session *session.BbsSession, user *user.User) error
    OnActivityChange(nodeID int, session *session.BbsSession, activity NodeActivity) error
}

// Add custom event listener
integration.AddEventListener(myListener)
```

## Security Features

- **Access Control**: Node-specific access levels and user flags
- **Session Management**: Automatic timeout and idle detection
- **Resource Limits**: CPU, memory, and time limit enforcement
- **Activity Monitoring**: Real-time user activity tracking
- **Alert System**: Security event notifications

## Performance Considerations

- **Efficient Data Structures**: Optimized for real-time updates
- **Configurable Refresh Rates**: Balance responsiveness with CPU usage
- **Memory Management**: Automatic cleanup of old performance data
- **Concurrent Safety**: Thread-safe operations throughout

## Future Enhancements

- **Web Interface**: Browser-based monitoring dashboard
- **API Integration**: REST API for external monitoring tools
- **Database Backend**: Optional database storage for large installations
- **Cluster Support**: Multi-server BBS configurations
- **Mobile Interface**: Mobile-friendly monitoring applications

## Contributing

This system is designed to be extensible. Key areas for contribution:

1. **New Display Modes**: Additional visualization options
2. **Enhanced Alerting**: More sophisticated alert conditions
3. **Performance Optimizations**: Memory and CPU usage improvements
4. **Integration Adapters**: Support for additional session types
5. **Documentation**: Examples and tutorials

## License

This code is part of the ViSiON/3 BBS project and follows the same licensing terms.