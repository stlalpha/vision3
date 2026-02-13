# ViSiON/3 Door Programs Guide

This guide covers configuring and running external door programs in ViSiON/3. Doors can be native executables or classic DOS programs launched via dosemu2.

## Door Types

ViSiON/3 supports two types of doors:

- **Native doors** — Executables or scripts that run directly on the host OS. I/O is piped through a PTY or directly via STDIO.
- **DOS doors** — Classic DOS door games (LORD, TradeWars, Darkness, etc.) launched inside dosemu2. COM1 serial I/O is bridged to the SSH session as raw CP437 bytes.

## Door Configuration

Doors are configured in `configs/doors.json` as a JSON array. Each entry defines a door and how to launch it.

### Native Door Example

```json
{
    "name": "GD2",
    "command": "./gd2",
    "args": ["DOOR.SYS"],
    "working_directory": "/path/to/doors/galactic-dynasty-2",
    "dropfile_type": "DOOR.SYS",
    "io_mode": "STDIO",
    "requires_raw_terminal": true
}
```

### DOS Door Example (via dosemu2)

```json
{
    "name": "DARK",
    "is_dos": true,
    "dos_commands": ["cd c:\\doors\\darkness\\", "dark16 /n{NODE}"],
    "dropfile_type": "DOOR.SYS"
}
```

When `is_dos` is `true`, the `command` and `args` fields are ignored — `dos_commands` is used instead.

### Configuration Properties

#### Common Properties

| Property | Type | Description |
|----------|------|-------------|
| `name` | string | Unique identifier used in `DOOR:NAME` menu commands |
| `dropfile_type` | string | Dropfile format: `"DOOR.SYS"`, `"CHAIN.TXT"`, or `"NONE"` (default: none) |
| `requires_raw_terminal` | bool | Use raw PTY mode for full-screen/interactive doors (default: `false`) |

#### Native Door Properties

| Property | Type | Description |
|----------|------|-------------|
| `command` | string | Path to the executable (absolute or relative to `working_directory`) |
| `args` | string[] | Command-line arguments (supports placeholder variables) |
| `working_directory` | string | Directory to run the command in |
| `io_mode` | string | I/O handling: `"STDIO"` (default) |
| `environment_variables` | map | Additional environment variables (supports placeholders) |

#### DOS Door Properties

| Property | Type | Description |
|----------|------|-------------|
| `is_dos` | bool | Set to `true` to launch via dosemu2 |
| `dos_commands` | string[] | DOS commands to execute inside dosemu2 (supports placeholders) |
| `drive_c_path` | string | Custom path to `drive_c` directory (default: `~/.dosemu/drive_c`) |
| `dosemu_path` | string | Custom path to dosemu binary (default: `/usr/bin/dosemu`) |
| `dosemu_config` | string | Path to custom `.dosemurc` config file |

## Placeholder Variables

These variables can be used in `args`, `dos_commands`, and `environment_variables`:

| Variable | Description |
|----------|-------------|
| `{NODE}` | Node/line number |
| `{PORT}` | Port number (same as NODE) |
| `{TIMELEFT}` | Minutes remaining in session |
| `{BAUD}` | Baud rate (default: 38400) |
| `{USERHANDLE}` | User's handle/alias |
| `{USERID}` | User ID number |
| `{REALNAME}` | User's real name |
| `{LEVEL}` | User's access level |

## Menu Setup

Doors are launched from menus using the `DOOR:NAME` command, where `NAME` matches the door's `name` field in `doors.json`.

### Required Menu Files

A doors menu needs three files:

**`menus/v3/ans/DOORSM.ANS`** — ANSI art showing available doors and their hotkeys.

**`menus/v3/mnu/DOORSM.MNU`** — Menu display settings:

```json
{
  "CLR": true,
  "USEPROMPT": true,
  "PROMPT1": "|08[|07|TIME|08] |08[|15TL:|07|TL|08]|15 |07D|08o|15o|07r|08s |08-|15>|07 ",
  "PROMPT2": "",
  "FALLBACK": "MAIN",
  "ACS": "",
  "PASS": ""
}
```

**`menus/v3/cfg/DOORSM.CFG`** — Hotkey-to-command bindings:

```json
[
    {
        "KEYS": "A",
        "CMD": "DOOR:DARK",
        "ACS": "*",
        "HIDDEN": false
    },
    {
        "KEYS": "B",
        "CMD": "DOOR:GD2",
        "ACS": "*",
        "HIDDEN": false
    },
    {
        "KEYS": "Q",
        "CMD": "GOTO:MAIN",
        "ACS": "*",
        "HIDDEN": false
    }
]
```

The `CLR: true` setting clears the screen before displaying the ANSI art. When a door exits, the menu redraws automatically.

### Linking from Main Menu

Add a `GOTO:DOORSM` entry in `menus/v3/cfg/MAIN.CFG`:

```json
{
    "KEYS": "D",
    "CMD": "GOTO:DOORSM",
    "ACS": "*",
    "HIDDEN": false
}
```

## Dropfile Formats

Dropfiles provide user and session information to door programs. ViSiON/3 generates full standard-format dropfiles with `\r\n` (DOS) line endings.

### DOOR.SYS (52-line PCBoard format)

The full 52-line format used by most BBS doors. Includes COM port, baud rate, user real name, handle, location, security level, time remaining, graphics mode, BBS name, user record number, and more.

### DOOR32.SYS (11-line format)

Contains connection type, socket handle, baud rate, BBS version, user ID, real name, handle, security level, time remaining, emulation mode, and node number.

### DORINFO1.DEF (13-line format)

Contains BBS name, sysop name, COM port, baud rate, user first/last name, location, graphics flag, security level, time remaining, and FOSSIL flag.

### CHAIN.TXT (WWIV format)

Contains user ID, handle, real name, protocol, time on, gender, screen dimensions, security level, time remaining, and BBS name.

**Native doors** generate only the configured `dropfile_type` in the `working_directory`. **DOS doors** generate all four formats in the per-node temp directory.

## Native Door Setup

### Step 1: Install the Door

```bash
mkdir -p /opt/doors/mydoor
cp mydoor /opt/doors/mydoor/
chmod +x /opt/doors/mydoor/mydoor
```

### Step 2: Add to doors.json

```json
{
    "name": "MYDOOR",
    "command": "./mydoor",
    "args": ["DOOR.SYS"],
    "working_directory": "/opt/doors/mydoor",
    "dropfile_type": "DOOR.SYS",
    "io_mode": "STDIO",
    "requires_raw_terminal": true
}
```

### Step 3: Add a Menu Hotkey

Add to `menus/v3/cfg/DOORSM.CFG`:

```json
{
    "KEYS": "M",
    "CMD": "DOOR:MYDOOR",
    "ACS": "*",
    "HIDDEN": false
}
```

Update the `DOORSM.ANS` art to show the new hotkey.

### Step 4: Test

1. Restart the BBS (door configs are loaded at startup)
2. Navigate to the doors menu
3. Press the hotkey
4. Verify the door launches and exits cleanly back to the menu

### Terminal Modes

**Raw terminal mode** (`requires_raw_terminal: true`):
- Door runs inside a PTY with raw keyboard input
- Terminal echo is disabled
- Use for games, full-screen apps, anything needing direct terminal control

**Standard I/O mode** (`requires_raw_terminal: false`):
- Door's stdin/stdout/stderr are connected directly to the SSH session
- Normal line-based I/O
- Use for simple utilities, questionnaires

### Environment Variables

Native doors automatically receive these environment variables:

| Variable | Value |
|----------|-------|
| `BBS_USERHANDLE` | User's handle |
| `BBS_USERID` | User ID number |
| `BBS_NODE` | Node number |
| `BBS_TIMELEFT` | Time remaining (minutes) |

Additional variables can be set via `environment_variables`:

```json
"environment_variables": {
    "BBS_NAME": "My BBS",
    "GAME_PATH": "/opt/doors/data"
}
```

## DOS Door Setup via dosemu2

### Prerequisites

Install dosemu2:

```bash
sudo add-apt-repository ppa:dosemu2/ppa
sudo apt-get update
sudo apt-get install -y dosemu2
```

Run `dosemu` once manually to generate initial configuration, then exit with `exitemu`.

Run `setup.sh` (or re-run it) — it automatically copies the dosemu2 configs from `templates/configs/` if dosemu is installed:

- `~/.dosemu/.dosemurc` — Used by ViSiON/3 when launching DOS doors (COM1 serial I/O, CP437 charset)
- `~/.dosemu/drive_c/.dosemurc-nocom` — Used by `scripts/dos-local.sh` for interactive sessions (no COM1)

To use a custom `.dosemurc` for a specific door, set `dosemu_config` in `doors.json`.

### Local DOS Setup

Use `scripts/dos-local.sh` to open a local DOS command prompt for installing and configuring door programs:

```bash
./scripts/dos-local.sh                    # uses default ~/.dosemu/drive_c
./scripts/dos-local.sh /opt/bbs/drive_c   # custom drive_c path
```

This launches dosemu2 without COM1 serial redirection, giving you a direct interactive session to copy files, test executables, and set up directory structures on the virtual C: drive.

### drive_c Directory Structure

dosemu2 uses `~/.dosemu/drive_c/` as the virtual C: drive. Place DOS door programs there:

```
~/.dosemu/drive_c/
├── doors/
│   ├── darkness/         # Darkness door files
│   │   ├── dark16.exe
│   │   └── ...
│   └── lord/             # LORD door files
│       ├── lord.exe
│       └── ...
└── nodes/                # Per-node temp dirs (auto-created)
    ├── temp1/            # Node 1 dropfiles + EXTERNAL.BAT
    ├── temp2/            # Node 2
    └── ...
```

### DOS Door Configuration

Add to `configs/doors.json`:

```json
{
    "name": "DARK",
    "is_dos": true,
    "dos_commands": ["cd c:\\doors\\darkness\\", "dark16 /n{NODE}"],
    "dropfile_type": "DOOR.SYS"
}
```

Optional overrides:

```json
{
    "name": "TW2002",
    "is_dos": true,
    "dos_commands": ["cd c:\\doors\\tw2002\\", "tw2002 /n{NODE}"],
    "dropfile_type": "DOOR.SYS",
    "drive_c_path": "/opt/bbs/drive_c",
    "dosemu_path": "/usr/local/bin/dosemu",
    "dosemu_config": "/opt/bbs/custom.dosemurc"
}
```

### How DOS Doors Work

For `dos_commands: ["cd c:\\doors\\darkness\\", "dark16 /n{NODE}"]` on node 3:

1. Creates per-node temp directory: `~/.dosemu/drive_c/nodes/temp3/`
2. Generates all four dropfiles (`DOOR.SYS`, `DOOR32.SYS`, `DORINFO1.DEF`, `CHAIN.TXT`)
3. Writes `EXTERNAL.BAT`:
   ```batch
   @echo off
   @lredir -f D: /home/user/.dosemu/drive_c/nodes/temp3 >NUL
   c:
   cd c:\doors\darkness\
   dark16 /n3
   exitemu
   ```
4. Creates a PTY pair. The slave side becomes dosemu's controlling terminal.
5. Launches dosemu2 with:
   - `-I "video { none }"` — disables video console (no boot text shown)
   - `-I "serial { virtual com 1 }"` — connects COM1 to the controlling terminal (the PTY)
   - dosemu stdout/stderr go to `/dev/null` (hides any remaining boot output)
6. Door I/O flows: **door program** → COM1 → controlling terminal (PTY slave) → PTY master → **SSH session**
7. Raw CP437 bytes pass through untranslated — no charset conversion
8. On exit: dropfiles and batch file are cleaned up automatically

The `lredir` command maps `D:` to the node's temp directory, so doors can find dropfiles at `D:\DOOR.SYS`.

## Complete doors.json Example

```json
[
    {
        "name": "DARK",
        "is_dos": true,
        "dos_commands": ["cd c:\\doors\\darkness\\", "dark16 /n{NODE}"],
        "dropfile_type": "DOOR.SYS"
    },
    {
        "name": "GD2",
        "command": "./gd2",
        "args": ["DOOR.SYS"],
        "working_directory": "/path/to/doors/galactic-dynasty-2/GalacticDynasty2",
        "dropfile_type": "DOOR.SYS",
        "io_mode": "STDIO",
        "requires_raw_terminal": true
    }
]
```

## Troubleshooting

### Door Won't Launch

- Check executable path exists and has execute permissions (`chmod +x`)
- Verify `working_directory` exists
- Review BBS logs for error messages
- For DOS doors, verify `dosemu` is installed at the expected path

### Door Exits Immediately

- Native doors: verify the dropfile format matches what the door expects
- Check that all required game data files are present
- Test running the door manually from the command line with a valid dropfile

### Key Presses Not Working

- Set `requires_raw_terminal` to `true` for interactive doors
- The door must read from stdin (not directly from `/dev/tty`)

### Display Issues

- For DOS doors, ensure your SSH client supports CP437 encoding (SyncTERM, NetRunner, etc.)
- Check `.dosemurc` has `$_external_char_set = "cp437"` and `$_internal_char_set = "cp437"`
- Toggle `requires_raw_terminal` for native doors

### DOS Door Issues

- **dosemu not found** — Verify `/usr/bin/dosemu` exists or set `dosemu_path`
- **Door hangs** — Check `~/.dosemu/drive_c/nodes/temp{N}/dosemu_boot.log`
- **Drive C: not found** — Run `dosemu` once manually to generate initial config
- **Dropfiles not found** — Doors should access dropfiles from `D:\` (mapped via `lredir`)
- **No output from door** — Ensure the door writes to COM1; `serial { virtual com 1 }` must be working

## Security Considerations

- Use absolute paths for commands when possible
- Each node gets its own temp directory to prevent conflicts
- Dropfiles and batch files are cleaned up automatically on exit
- Avoid user-controlled values in command paths
- Run doors with appropriate privileges
