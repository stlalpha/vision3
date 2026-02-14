# ViSiON/3 Door Programs Guide

This guide covers configuring and running external door programs in ViSiON/3.

## Door System Overview

Door programs are external applications that can be launched from the BBS. These include games, utilities, and other programs that interact with users through the terminal.

## Door Configuration

Doors are configured in `configs/doors.json` as an array:

```json
[
  {
    "name": "TETRIS",
    "command": "/path/to/door/tetris",
    "args": ["-node", "{NODE}", "-time", "{TIMELEFT}"],
    "working_directory": "/path/to/door",
    "requires_raw_terminal": true,
    "dropfile_type": "DOOR.SYS",
    "io_mode": "STDIO",
    "environment_variables": {
      "BBS_NAME": "ViSiON/3 BBS",
      "BBS_PATH": "/path/to/bbs"
    }
  }
]
```

### Configuration Properties

- `name` - Unique identifier used in DOOR:NAME commands
- `command` - Path to the executable
- `args` - Command line arguments (can include placeholders)
- `working_directory` - Directory to run the command in (optional)
- `requires_raw_terminal` - Whether to use raw terminal mode (optional, defaults to false)
- `dropfile_type` - Type of dropfile: "DOOR.SYS", "CHAIN.TXT", or "NONE" (optional, defaults to NONE)
- `io_mode` - I/O handling mode: "STDIO" (optional, defaults to STDIO)
- `environment_variables` - Additional environment variables (optional)

## Placeholder Variables

These variables can be used in `args` and `environment_variables`:

- `{NODE}` - Node/line number
- `{PORT}` - Port number (simulated, same as NODE)
- `{TIMELEFT}` - Minutes remaining in session
- `{BAUD}` - Baud rate (default: 38400)
- `{USERHANDLE}` - User's handle/alias
- `{USERID}` - User ID number
- `{REALNAME}` - User's real name
- `{LEVEL}` - User's access level

### Example Usage

```json
"args": [
  "-N", "{NODE}",
  "-T", "{TIMELEFT}",
  "-U", "{USERHANDLE}"
]
```

## Dropfile Types

Dropfiles provide user/session information to door programs.

### DOOR.SYS

Standard format used by many doors:

```ini
1                    # COM port
38400                # Baud rate
N                    # Parity
1                    # Node number
Felonius             # User name
10                   # Security level
60                   # Time left
ANSI                 # Emulation
1                    # User ID
```

### CHAIN.TXT

WWIV-style dropfile:

```ini
Felonius             # User name
10                   # Security level
60                   # Time left
1                    # User ID
```

### No Dropfile

Set `dropfile_type` to `"NONE"` if the door doesn't need one.

## Setting Up a Door

### Step 1: Install the Door Program

Place door files in appropriate directory:

```bash
mkdir -p /opt/doors/mydoor
cp mydoor.exe /opt/doors/mydoor/
```

### Step 2: Configure the Door

Add to `configs/doors.json`:

```json
[
  {
    "name": "MYDOOR",
    "command": "/opt/doors/mydoor/mydoor.exe",
    "args": ["-dropfile", "DOOR.SYS"],
    "working_directory": "/opt/doors/mydoor",
    "requires_raw_terminal": true,
    "dropfile_type": "DOOR.SYS",
    "io_mode": "STDIO"
  }
]
```

### Step 3: Add to Menu

Edit menu file (e.g., `menus/v3/mnu/DOORS.MNU`):

```ini
HOTKEY:M:DOOR:MYDOOR
```

### Step 4: Test the Door

1. Restart the BBS
2. Navigate to doors menu
3. Press the hotkey
4. Verify door launches correctly

## Terminal Modes

### Raw Terminal Mode

For interactive doors that need direct terminal control:

```json
"requires_raw_terminal": true
```

- Door gets raw keyboard input
- Terminal echo is disabled
- Used for games, full-screen apps

### Standard I/O Mode

For simple doors that use standard input/output:

```json
"requires_raw_terminal": false
```

- Normal line-based I/O
- Terminal echo remains on
- Used for questionnaires, utilities

## Common Door Examples

### DOS Door Game

```json
[
  {
    "name": "LORDGAME",
    "command": "/opt/doors/lord/START.BAT",
    "args": ["{NODE}"],
    "working_directory": "/opt/doors/lord",
    "requires_raw_terminal": true,
    "dropfile_type": "DOOR.SYS",
    "io_mode": "STDIO"
  }
]
```

### Native Linux Door

```json
[
  {
    "name": "ADVENTURE",
    "command": "/usr/games/adventure",
    "args": [],
    "working_directory": "/tmp",
    "requires_raw_terminal": false,
    "dropfile_type": "NONE",
    "io_mode": "STDIO"
  }
]
```

### Script-Based Door

```json
[
  {
    "name": "QUOTER",
    "command": "/opt/doors/scripts/quote.sh",
    "args": ["{USERHANDLE}"],
    "working_directory": "/opt/doors/scripts",
    "requires_raw_terminal": false,
    "dropfile_type": "NONE",
    "io_mode": "STDIO",
    "environment_variables": {
      "USER_LEVEL": "{LEVEL}"
    }
  }
]
```

## Running DOS Doors

For DOS-based doors, you'll need:

### DOSBox

```json
[
  {
    "name": "DOSDOOR",
    "command": "dosbox",
    "args": [
      "-c", "mount c /opt/doors/dosdoor",
      "-c", "c:",
      "-c", "door.exe {NODE}",
      "-c", "exit"
    ],
    "working_directory": "/opt/doors/dosdoor",
    "requires_raw_terminal": true,
    "dropfile_type": "DOOR.SYS",
    "io_mode": "STDIO"
  }
]
```

### DOSEMU

Similar setup but using DOSEMU instead of DOSBox.

## Troubleshooting

### Door Won't Launch

- Check executable path exists
- Verify file permissions (`chmod +x`)
- Check working directory exists
- Review BBS logs for errors

### Display Issues

- Try toggling `requires_raw_terminal`
- Check terminal type compatibility
- Test with different SSH clients

### Dropfile Problems

- Verify dropfile is created in working directory
- Check dropfile format matches door expectations
- Ensure write permissions in working directory

### Door Exits Immediately

- Check command line arguments
- Verify all required files exist
- Test running door manually
- Check for missing dependencies

## Security Considerations

### Path Security

- Use absolute paths for commands
- Avoid shell metacharacters in args
- Don't allow user input in command paths

### User Isolation

- Run doors with limited privileges
- Use separate directories per node
- Clean up temporary files

### Resource Limits

- Set time limits appropriately
- Monitor CPU/memory usage
- Implement kill timers if needed

## Best Practices

1. **Test Thoroughly**: Test each door with different users
2. **Document Requirements**: Note any special setup needs
3. **Backup Configurations**: Keep backups of working configs
4. **Monitor Usage**: Track popular doors and issues
5. **Update Regularly**: Keep door programs updated

## Advanced Configuration

### Multi-Node Setup

For doors that support multiple nodes:

```json
"working_directory": "/opt/doors/mydoor/node{NODE}"
```

### Custom Dropfiles

Some doors need specific dropfile formats. You may need to:

1. Generate custom format in code
2. Use a wrapper script
3. Modify the door

### Wrapper Scripts

Create wrapper scripts for complex setups:

```bash
#!/bin/bash
# door-wrapper.sh
cd /opt/doors/mydoor
./setup.sh $1
./door.exe
./cleanup.sh
```

## Popular BBS Doors

Common doors to consider:

- **Games**: LORD, TradeWars 2002, Usurper
- **Utilities**: File managers, user listers
- **Communication**: Inter-BBS chat, messaging
- **Entertainment**: Trivia, fortune tellers

## Future Enhancements

Planned door system improvements:

- Native door SDK
- Better dropfile support
- Door usage statistics
- Time banking system
- Door chaining support
- Automatic door installation
