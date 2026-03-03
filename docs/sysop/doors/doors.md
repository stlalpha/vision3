# Door Programs

Doors are external programs launched from the BBS. ViSiON/3 generates a dropfile, hands off the user's terminal to the door process, and resumes the BBS session when the door exits.

## Configuration

Use the [Configuration Editor](../configuration/configuration.md#configuration-editor-tui) (`./config`, section 5 — Door Programs) to add, edit, and remove door definitions interactively. This is the recommended approach.

### JSON Reference

Door programs are stored in `configs/doors.json`. Each entry is keyed by a unique name:

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

## Configuration Fields

### Native Doors

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

### DOS Doors

Set `is_dos: true` to run a 16-bit DOS door game via a DOS emulator.

| Field | Description |
|-------|-------------|
| `is_dos` | `true` = DOS door |
| `dos_commands` | Array of DOS commands to run (supports placeholders) |
| `drive_c_path` | Host path mounted as DOS C: drive (default: `~/.dosemu/drive_c`) |
| `dos_emulator` | Emulator selection: `""` or `"auto"` (default), `"dosemu"`, `"dosbox"` |
| `dosemu_path` | Path to dosemu2 binary (default: `/usr/bin/dosemu`) |
| `dosemu_config` | Custom `.dosemurc` config file path (optional) |
| `dosbox_path` | Path to dosbox-x binary (default: auto-detect `dosbox-x` or `dosbox` in `$PATH`) |
| `dosbox_config` | Base `dosbox-x.conf` template path (default: built-in BBS template) |

#### Emulator Selection

| `dos_emulator` value | Behaviour |
|---|---|
| `""` or `"auto"` | Prefer dosemu2 on Linux x86 if installed; otherwise use DOSBox-X |
| `"dosemu"` | Always use dosemu2 (Linux x86 only) |
| `"dosbox"` | Always use DOSBox-X (cross-platform: Linux, macOS, Windows) |

#### Platform Support

| Platform | dosemu2 | DOSBox-X |
|---|---|---|
| Linux x86 / x86-64 | Yes | Yes |
| Linux ARM / ARM64 | No | Yes |
| macOS (Intel or Apple Silicon) | No | Yes |
| Windows 64-bit | No | Yes |

#### I/O

Both emulators connect the door's COM1 serial port back to the user's SSH session:
- **dosemu2** — via a PTY pair (`serial { virtual com 1 }`)
- **DOSBox-X** — via a TCP loopback socket (`serial1=nullmodem`, raw CP437 bytes)

The user's terminal sees identical raw byte streams from both paths.

#### DOS Door Example (DOSBox-X)

```json
{
  "name": "LORD",
  "is_dos": true,
  "dos_emulator": "dosbox",
  "dos_commands": [
    "cd C:\\DOORS\\LORD",
    "LORD.EXE /N{NODE} /T{TIMELEFT}"
  ],
  "drive_c_path": "/opt/bbs/drive_c",
  "dropfile_type": "DOOR.SYS"
}
```

#### DOS Door Example (auto-detect)

```json
{
  "name": "USURPER",
  "is_dos": true,
  "dos_commands": [
    "cd C:\\DOORS\\USURPER",
    "USR.EXE /N{NODE}"
  ],
  "drive_c_path": "/opt/bbs/drive_c",
  "dropfile_type": "DOOR.SYS"
}
```

With no `dos_emulator` set, dosemu2 is used on Linux x86 if available, otherwise DOSBox-X is used automatically.

## Placeholders

The following placeholders can be used in `args` (native doors) and `dos_commands` (DOS doors) and are substituted at runtime:

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

Doors are launched via menu commands. Add a menu entry with the `DOOR` command type and specify the door key as the data parameter. See [Menus & ACS](../menus/menu-system.md) for details.
