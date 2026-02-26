# Door Programs

Doors are external programs launched from the BBS. ViSiON/3 generates a dropfile, hands off the user's terminal to the door process, and resumes the BBS session when the door exits.

## Configuration

Door programs are defined in `configs/doors.json`. Each entry is keyed by a unique name:

```json
{
  "lord": {
    "name": "Legend of the Red Dragon",
    "command": "lord.exe",
    "args": ["/N{NODE}", "/P{PORT}", "/T{TIMELEFT}"],
    "working_directory": "/opt/bbs/doors/lord",
    "dropfile_type": "DOOR.SYS",
    "io_mode": "STDIO",
    "requires_raw_terminal": true,
    "environment_variables": {
      "TERM": "ansi"
    }
  }
}
```

You can also manage doors through the [TUI Configuration Editor](configuration/configuration.md#configuration-editor-tui) (section 5).

## Configuration Fields

| Field | Description |
|-------|-------------|
| `name` | Display name for the door |
| `command` | Path to the executable |
| `args` | Command-line arguments (supports placeholders) |
| `working_directory` | Directory to run the command in |
| `dropfile_type` | Dropfile format: `DOOR.SYS`, `CHAIN.TXT`, or `NONE` |
| `io_mode` | I/O handling: `STDIO` |
| `requires_raw_terminal` | Whether raw terminal mode is needed |
| `environment_variables` | Additional environment variables to set |

## Placeholders

The following placeholders can be used in `args` and are substituted at runtime:

| Placeholder | Value |
|-------------|-------|
| `{NODE}` | Node number |
| `{PORT}` | Port number |
| `{TIMELEFT}` | Minutes remaining in session |
| `{BAUD}` | Baud rate (simulated) |
| `{USERHANDLE}` | User's handle |
| `{USERID}` | User ID number |
| `{REALNAME}` | User's real name |
| `{LEVEL}` | Access level |

## Menu Integration

Doors are launched via menu commands. Add a menu entry with the `DOOR` command type and specify the door key as the data parameter. See [Menus & ACS](menus/menu-system.md) for details.
