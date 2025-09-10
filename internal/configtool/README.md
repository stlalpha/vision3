# Vision/3 BBS Configuration Tool

## Multi-Node Binary Database System

This is a comprehensive BBS (Bulletin Board System) configuration tool that implements classic binary database formats with modern multi-node safety mechanisms. It provides authentic BBS-style data storage with the reliability needed for multi-user environments.

## Features

### Binary Message Base System (`msgbase/`)
- **Classic BBS Storage Format**: Fixed-length binary records similar to PCBoard, TriBBS, and other traditional BBS systems
- **Message Threading**: Reply chain linking and thread navigation
- **Binary File Format**: 
  - `MESSAGES.HDR` - 256-byte message headers with fixed fields
  - `MESSAGES.DAT` - Message text stored in 128-byte blocks
  - `MESSAGES.IDX` - Fast message indexing and lookup
  - `MESSAGES.THD` - Thread linking information
- **Multi-Node Safety**: File locking prevents corruption during concurrent access
- **Maintenance Tools**: Pack, reindex, and repair utilities

### Binary File Base System (`filebase/`)
- **Classic .DIR Format**: Compatible with traditional BBS file databases
- **File Record Structure**: 128-byte fixed records with file metadata
- **Extended Records**: Additional 512-byte records for enhanced file information
- **Features**:
  - FILE_ID.DIZ extraction from ZIP archives
  - DESC.SDI support for file descriptions
  - Duplicate file detection using MD5/CRC32
  - File validation and integrity checking
  - Batch file import from disk
- **Multi-Node Operations**: Safe concurrent file uploads and downloads

### Multi-Node Coordination (`multinode/`)
- **Node Status Tracking**: Real-time monitoring of active nodes
- **Resource Locking**: Prevents database corruption during concurrent operations
- **Inter-Node Messaging**: Communication between nodes
- **System Events**: Comprehensive event logging
- **Semaphore System**: Critical section protection
- **Deadlock Detection**: Automatic detection and prevention
- **Activity Monitoring**: Real-time user activity tracking

### Configuration Interface (`ui/`)
- **Turbo Pascal Style**: Authentic retro text-mode interface
- **Box Drawing**: Classic CP437/DOS character set
- **Color Support**: Full 16-color DOS palette
- **Dialog System**: Modal dialogs, input forms, list boxes
- **Menu System**: Hierarchical menus with hotkeys
- **Real-Time Displays**: Live node status and system monitoring

### Database Maintenance (`maintenance/`)
- **Message Base**:
  - Pack messages (remove deleted entries)
  - Rebuild indexes and thread chains
  - Repair corrupted data
  - Verify database integrity
  - Purge old messages
  - Backup and restore
- **File Base**:
  - Pack file records
  - Duplicate file detection and removal
  - Orphaned file cleanup
  - File integrity validation
  - Database repair
  - Backup with optional file copying

## Architecture

### File Structure
```
internal/configtool/
├── msgbase/           # Message base system
│   ├── types.go       # Data structures and constants
│   └── manager.go     # Core message operations
├── filebase/          # File base system  
│   ├── types.go       # File record structures
│   └── manager.go     # File operations and management
├── multinode/         # Multi-node coordination
│   ├── types.go       # Node communication structures
│   └── manager.go     # Node management and locking
├── ui/                # User interface
│   ├── turbo.go       # Turbo Pascal-style UI toolkit
│   └── config.go      # Configuration screens
└── maintenance/       # Database maintenance
    ├── msgbase.go     # Message base maintenance
    └── filebase.go    # File base maintenance
```

### Database Files

#### Message Base Files (per area)
- `MESSAGES.HDR` - Fixed 256-byte message headers
- `MESSAGES.DAT` - Variable-length message text (128-byte blocks)
- `MESSAGES.IDX` - 20-byte index entries for fast access
- `MESSAGES.THD` - 20-byte thread index for reply chains
- `AREAS.DAT` - 512-byte area configuration records
- `STATS.DAT` - 64-byte statistics record

#### File Base Files (per area) 
- `FILES.DIR` - 128-byte file records (classic DIR format)
- `FILES.EXT` - 512-byte extended file records
- `FILES.IDX` - 24-byte index entries
- `FILES.DUP` - 32-byte duplicate detection records
- `FILEAREAS.DAT` - 512-byte area configuration
- `FILESTATS.DAT` - 128-byte statistics

#### Multi-Node Files
- `NODESTATUS.DAT` - 256-byte node status records
- `LOCKS.DAT` - 256-byte resource lock records
- `SEMAPHORE.DAT` - 256-byte semaphore records
- `NODEMSG.DAT` - 512-byte inter-node messages
- `EVENTS.DAT` - 512-byte system event log
- `ACTIVITY.DAT` - 256-byte real-time activity monitor

## Multi-Node Safety

### Locking Mechanisms
- **File-Level Locking**: Uses OS file locking (flock) for atomic operations
- **Resource Locks**: Database-level locks for specific operations
- **Semaphores**: System-wide critical section protection
- **Deadlock Prevention**: Automatic detection and timeout handling

### Lock Types
- **Read Locks**: Shared access for reading operations
- **Write Locks**: Exclusive access for modifications
- **Maintenance Locks**: Block all access during maintenance
- **Import/Export Locks**: Coordinate bulk operations

### Node Coordination
- **Status Updates**: Periodic node heartbeats
- **Message Passing**: Inter-node communication system
- **Event Logging**: Centralized system event tracking
- **Resource Monitoring**: Track resource usage per node

## Usage

### Command Line
```bash
# Start configuration tool on node 1
./vision3-bbsconfig -node 1 -path /bbs/data

# Show version information  
./vision3-bbsconfig -version

# Show help
./vision3-bbsconfig -help
```

### Configuration Menus

#### Main Menu
- Message Base Configuration
- File Base Configuration  
- Multi-Node Setup
- Database Maintenance
- Real-Time Monitor
- System Statistics

#### Message Base Menu
- Create/Edit Message Areas
- Pack Message Base
- Import/Export Messages
- View Statistics
- Repair Database

#### File Base Menu
- Create/Edit File Areas
- Import Files from Disk
- Pack File Base
- Duplicate Detection
- File Maintenance
- View Statistics

#### Multi-Node Menu
- Node Status Display
- Configure Node Limits
- Inter-Node Messages
- Resource Lock Monitor
- System Event Log
- Broadcast Messages

## Technical Details

### Binary Formats
All data structures use little-endian byte order and fixed-length records for compatibility with classic BBS software. Strings are null-terminated and padded to fixed lengths.

### Performance Features
- **Binary Search**: Indexed access to large datasets
- **Block-Based I/O**: Efficient disk access patterns
- **Memory Mapping**: Optional for high-performance scenarios
- **Concurrent Access**: Multi-reader, single-writer patterns

### Error Handling
- **Graceful Degradation**: Continue operation despite non-critical errors
- **Automatic Recovery**: Self-healing for minor corruption
- **Detailed Logging**: Comprehensive error reporting
- **User Feedback**: Clear error messages in UI

### Security Considerations
- **Access Control**: Security levels for read/write operations
- **File Validation**: Prevent malicious file uploads
- **Resource Limits**: Prevent resource exhaustion
- **Audit Trail**: Track all administrative operations

## Development

### Building
```bash
go build -o vision3-bbsconfig cmd/vision3-bbsconfig/main.go
```

### Testing
```bash
go test ./internal/configtool/...
```

### Dependencies
- Standard Go library only
- Unix-specific features use golang.org/x/sys/unix
- No external dependencies for core functionality

### Compatibility
- Linux/Unix systems (primary target)
- macOS support
- Windows support with some limitations (file locking)
- Terminal requirements: ANSI color support, UTF-8 for box drawing

## License

This software is part of the Vision/3 BBS system and follows the same licensing terms as the parent project.