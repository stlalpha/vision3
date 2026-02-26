# ViSiON/3 Configuration Guide

This guide covers the configuration files used by ViSiON/3 BBS. You can edit these files by hand or use the **TUI Configuration Editor** (see below).

## Configuration Editor (TUI)

ViSiON/3 includes an interactive TUI configuration editor modeled after ViSiON/2's `CONFIG.EXE`. It provides a menu-driven interface for managing all system configuration without editing JSON by hand.

### Running the Editor

```bash
# From the BBS directory (uses configs/ by default)
./config

# Or specify a custom config path
./config --config /path/to/configs
```

### Main Menu

The editor opens to a main menu with sections for each configuration area:

| Key | Section | Description |
|-----|---------|-------------|
| 1 | System Configuration | BBS name, network, connection limits, access levels, defaults, IP lists |
| 2 | Message Areas | Message area records with conference grouping |
| 3 | File Areas | File area records |
| 4 | Conferences | Conference definitions and ordering |
| 5 | Door Programs | External door configurations |
| 6 | Event Scheduler | Automated event definitions |
| 7 | FTN Network | FidoNet-style network link settings |
| 8 | Transfer Protocols | File transfer protocol definitions |
| 9 | Archivers | Archive format tool definitions |
| A | Login Sequence | Login step sequence |

### Navigation

| Key | Action |
|-----|--------|
| Up/Down | Move highlight |
| Enter | Select / edit field |
| I | Insert new record |
| D | Delete record |
| P | Reorder record position (supported record types) |
| PgUp/PgDn | Navigate between records in the field editor |
| ESC | Return to previous screen |

### Reordering Records

Press **P** in a record list to enter reorder mode. Use Up/Down to choose the new position and Enter to confirm. For message areas, reordering is constrained to within the same conference.

### Saving

The editor tracks unsaved changes. When you quit with pending changes, you are prompted to save. All configuration files are written atomically.

## Configuration Files Overview

Configuration files are split between two directories:

**In `configs/` directory:**

- `strings.json` - Customizable text strings and prompts
- `doors.json` - External door program configurations
- `file_areas.json` - File area definitions
- `message_areas.json` - Message area definitions
- `conferences.json` - Conference grouping definitions
- `events.json` - Event scheduler configuration
- `config.json` - General BBS configuration
- `archivers.json` - Archive format definitions (ZIP, 7z, RAR, ARJ, LHA)
- SSH host keys (`ssh_host_rsa_key`, etc.)

**In `menus/v3/` directory (menu set):**

- `bar/PDMATRIX.BAR`, `cfg/PDMATRIX.CFG`, `mnu/PDMATRIX.MNU` - Pre-login matrix menu (see [Menu System Guide](../menus/menu-system.md#pre-login-matrix-screen))
- `theme.json` - Theme color settings
- `ansi/PRELOGON.ANS` (or `PRELOGON.1`, `PRELOGON.2`, ...) - Pre-login ANSI screens shown before LOGIN (see [Menu System Guide](../menus/menu-system.md#pre-login-ansi-files-prelogon))

**In `data/` directory:**

- `oneliners.json` - One-liner messages (JSON array)
- `oneliners.dat` - Legacy one-liner format (plain text, optional)

## strings.json

This file contains all the customizable text strings displayed by the BBS. You can modify these to personalize your system.

### Key String Categories

**Login/Authentication Strings:**

- `whatsYourAlias` - Login username prompt
- `whatsYourPw` - Login password prompt
- `systemPasswordStr` - System password prompt
- `wrongPassword` - Invalid password message

**User Interface Strings:**

- `pauseString` - Pause prompt (e.g., "Press Enter to continue")
- `defPrompt` - Default menu prompt
- `continueStr` - More prompt for paginated displays

**One-Liner Strings:**

- `askOneLiner` - Prompt asking whether to post a one-liner
- `oneLinerAnonymousPrompt` - Prompt asking whether to post anonymously
- `enterOneLiner` - Prompt for one-liner text entry
- `anonymousName` - Display label used when `anonymous=true` (for example, "Anonymous Coward")

**New User Strings:**

- `newUserNameStr` - New user alias prompt
- `createAPassword` - New user password creation
- `enterRealName` - Real name prompt
- `enterNumber` - Phone number prompt
- `checkingUserBase` - Message shown while validating handle uniqueness

**Message System Strings:**

- `msgTitleStr` - Message title prompt
- `msgToStr` - Message recipient prompt
- `changeBoardStr` - Message area selection
- `postOnBoardStr` - Post confirmation

**File System Strings:**

- `changeFileAreaStr` - File area selection
- `downloadStr` - Download prompt
- `uploadFileStr` - Upload prompt

### Example Customization

```json
{
  "pauseString": "|15■|07■|08■ |B4|15SlAm eNtEr!|B0 |08■|07■|15■",
  "defPrompt": "|08•• |15|MN |08•• |13|TL |05Left: "
}
```

### Pipe Color Codes

The strings support pipe color codes:

- `|00-|15` - Standard 16 colors
- `|B0-|B7` - Background colors
- Special codes: `|CR` (carriage return), `|DE` (clear to end)

## doors.json

Configures external door programs that can be launched from the BBS. The file contains an array of door configurations.

### Door Configuration Structure

```json
[
  {
    "name": "LORD",
    "command": "lord.exe",
    "args": ["/N{NODE}", "/P{PORT}", "/T{TIMELEFT}"],
    "working_directory": "C:/BBS/DOORS/LORD",
    "dropfile_type": "DOOR.SYS",
    "io_mode": "STDIO",
    "requires_raw_terminal": true,
    "environment_variables": {
      "TERM": "ansi",
      "BBS_NAME": "My Vision3 BBS"
    }
  }
]
```

### Configuration Fields

- `name` - Unique identifier used in DOOR commands
- `command` - Path to the executable
- `args` - Command-line arguments (can include placeholders)
- `working_directory` - Directory to run the command in (optional)
- `dropfile_type` - Type of dropfile: "DOOR.SYS", "CHAIN.TXT", or "NONE" (optional)
- `io_mode` - I/O handling mode: "STDIO" (optional)
- `requires_raw_terminal` - Whether raw terminal mode is needed (optional)
- `environment_variables` - Additional environment variables (optional)

### Available Placeholders

- `{NODE}` - Node number
- `{PORT}` - Port number
- `{TIMELEFT}` - Minutes remaining
- `{BAUD}` - Baud rate (simulated)
- `{USERHANDLE}` - User's handle
- `{USERID}` - User ID number
- `{REALNAME}` - User's real name
- `{LEVEL}` - Access level

## archivers.json

Defines archive formats and the external tools used to pack, unpack, test, and list them. This centralized configuration ensures all subsystems (ZipLab upload pipeline, file area management, archive viewing) use the same archiver definitions, and that different platforms can specify their preferred tool versions.

This follows the classic BBS pattern where archiver definitions are configured once system-wide rather than hardcoded.

### Default Configuration

If `archivers.json` is missing, built-in defaults are used (ZIP native via Go stdlib).

### Archiver Definition Structure

```json
{
    "archivers": [
        {
            "id": "zip",
            "name": "ZIP Archive",
            "extension": ".zip",
            "magic": "504B0304",
            "native": true,
            "enabled": true,
            "pack":    { "command": "zip",   "args": ["-j", "{ARCHIVE}", "{FILES}"] },
            "unpack":  { "command": "unzip", "args": ["-o", "{ARCHIVE}", "-d", "{OUTDIR}"] },
            "test":    { "command": "unzip", "args": ["-t", "{ARCHIVE}"] },
            "list":    { "command": "unzip", "args": ["-l", "{ARCHIVE}"] },
            "comment": { "command": "zip",   "args": ["-z", "{ARCHIVE}"] },
            "addFile": { "command": "zip",   "args": ["-j", "{ARCHIVE}", "{FILE}"] }
        }
    ]
}
```

### Archiver Field Descriptions

- `id` - Unique short identifier (e.g., "zip", "rar", "7z", "arj", "lha")
- `name` - Human-readable display name
- `extension` - Primary file extension including the dot (e.g., ".zip")
- `extensions` - Additional file extensions for this format (e.g., `[".lzh"]` for LHA)
- `magic` - Hex-encoded magic bytes at file offset 0 for format detection (e.g., "504B0304" for ZIP)
- `native` - When `true`, Go's built-in `archive/zip` stdlib is used for core operations and external commands are ignored for basic pack/unpack. Currently only ZIP supports native mode.
- `enabled` - Controls whether this archiver is active. Disabled archivers are skipped during detection and processing.
- `pack` - Command to create an archive. Placeholders: `{ARCHIVE}`, `{FILES}`, `{WORKDIR}`
- `unpack` - Command to extract an archive. Placeholders: `{ARCHIVE}`, `{OUTDIR}`
- `test` - Command to verify archive integrity. Placeholder: `{ARCHIVE}`
- `list` - Command to list archive contents. Placeholder: `{ARCHIVE}`
- `comment` - Command to add a comment to an archive. Placeholders: `{ARCHIVE}`, `{FILE}`
- `addFile` - Command to add a file to an existing archive. Placeholders: `{ARCHIVE}`, `{FILE}`

### Built-in Archiver Definitions

| ID    | Name            | Extension(s) | Magic Bytes    | Native | Default State |
| ----- | --------------- | ------------ | -------------- | ------ | ------------- |
| `zip` | ZIP Archive     | .zip         | `504B0304`     | Yes    | Enabled       |
| `7z`  | 7-Zip Archive   | .7z          | `377ABCAF271C` | No     | Disabled      |
| `rar` | RAR Archive     | .rar         | `526172211A07` | No     | Disabled      |
| `arj` | ARJ Archive     | .arj         | `60EA`         | No     | Disabled      |
| `lha` | LHA/LZH Archive | .lha, .lzh   | —              | No     | Disabled      |

To enable additional archive formats, set `"enabled": true` and ensure the corresponding external tool is installed on the system.

### FTN Bundle Note

FTN echomail bundles always use ZIP format (per FidoNet standard practice) and are handled natively by Go's `archive/zip` regardless of this configuration. This config applies to user-facing archive operations: file area uploads, archive viewing, ZipLab pipeline, etc.

## file_areas.json

Defines file areas available on the BBS. The file contains an array of file area configurations.

### File Area Structure

```json
[
  {
    "id": 1,
    "tag": "GENERAL",
    "name": "General Files",
    "description": "General purpose file area",
    "path": "general",
    "acs_list": "",
    "acs_upload": "",
    "acs_download": "",
    "conference_id": 1
  }
]
```

### File Area Field Descriptions

- `id` - Unique numeric identifier
- `tag` - Short tag for the area (uppercase)
- `name` - Display name
- `description` - Area description
- `path` - Subdirectory under `data/files/`
- `acs_list` - ACS string required to list files
- `acs_upload` - ACS string required to upload
- `acs_download` - ACS string required to download
- `conference_id` - Conference this area belongs to (0 or omitted = ungrouped)

### Access Control Strings (ACS)

- `s10` - Security level 10 or higher
- `fA` - Flag A must be set
- `!fB` - Flag B must NOT be set
- `s20&fC` - Level 20+ AND flag C
- `s10|fD` - Level 10+ OR flag D

## config.json

General BBS configuration settings.

### Current Structure

```json
{
  "boardName": "PiRATE MiND STATiON",
  "boardPhoneNumber": "314-567-3833",
  "timezone": "America/Los_Angeles",
  "sysOpLevel": 255,
  "coSysOpLevel": 250,
  "logonLevel": 100,
  "sshPort": 2222,
  "sshHost": "0.0.0.0",
  "sshEnabled": true,
  "telnetPort": 2323,
  "telnetHost": "0.0.0.0",
  "telnetEnabled": true,
  "maxNodes": 10,
  "maxConnectionsPerIP": 3,
  "ipBlocklistPath": "",
  "ipAllowlistPath": "",
  "maxFailedLogins": 5,
  "lockoutMinutes": 30
}
```

### General Configuration Field Descriptions

**BBS Settings:**

- `boardName` - BBS name displayed to users
- `boardPhoneNumber` - Phone number (historical/display purposes)
- `timezone` - IANA timezone for display formatting (example: `America/Los_Angeles`)
- `sysOpLevel` - Security level for SysOp access
- `coSysOpLevel` - Security level for Co-SysOp access
- `logonLevel` - Security level granted after successful login

**SSH Server:**

- `sshPort` - Port for SSH connections (default: 2222)
- `sshHost` - Bind address for SSH listener (default: `0.0.0.0`)
- `sshEnabled` - Enable or disable the SSH server

**Telnet Server:**

- `telnetPort` - Port for telnet connections (default: 2323)
- `telnetHost` - Bind address for telnet listener (default: `0.0.0.0`)
- `telnetEnabled` - Enable or disable the telnet server

**Connection Security:**

- `maxNodes` - Maximum simultaneous connections allowed (default: 10, 0 = unlimited)
- `maxConnectionsPerIP` - Maximum simultaneous connections per IP address (default: 3, 0 = unlimited)
- `ipBlocklistPath` - Path to IP blocklist file (optional, leave empty to disable)
- `ipAllowlistPath` - Path to IP allowlist file (optional, leave empty to disable)

**Authentication Security:**

- `maxFailedLogins` - Maximum failed login attempts from a single IP before lockout (default: 5, 0 = disabled)
- `lockoutMinutes` - Duration of IP lockout in minutes (default: 30)

**Timezone behavior:**

- Last Callers time fields use `config.json` `timezone` first.
- If unset, the app checks environment variables `VISION3_TIMEZONE`, then `TZ`.
- If none are set or invalid, server local timezone is used.

### IP Blocklist/Allowlist Files

Both blocklist and allowlist files use the same format:

```text
# Comments start with #
# One IP or CIDR range per line

# Block specific IPs
192.168.1.100
10.0.0.50

# Block entire subnets
192.168.100.0/24
172.16.0.0/16

# IPv6 support
2001:db8::1
2001:db8::/32
```

**How it works:**

1. **Allowlist takes precedence**: If an IP is on the allowlist, it bypasses all other checks (blocklist, max nodes, per-IP limits)
2. **Blocklist checked next**: If an IP is on the blocklist, the connection is rejected
3. **Other limits apply**: If not on either list, normal connection limits apply

**Auto-Reload:**

- Files are **automatically monitored** for changes using file system watching
- When you edit and save either file, changes apply **within seconds** (no BBS restart needed)
- Debouncing (500ms) handles rapid successive edits
- All reloads are logged for debugging
- See [Security Guide](security.md#auto-reload-feature) for detailed usage

**Example setup:**

```json
{
  "ipBlocklistPath": "configs/blocklist.txt",
  "ipAllowlistPath": "configs/allowlist.txt"
}
```

Leave paths empty (`""`) to disable the feature.

## message_areas.json

Located in the `configs/` directory. Defines message areas available on the BBS.

See [Message Areas Guide](../messages/message-areas.md) for detailed configuration.

## conferences.json

Located in the `configs/` directory. Defines conferences that group message areas and file areas together for organized display.

### Conference Structure

```json
[
  {
    "id": 1,
    "tag": "LOCAL",
    "name": "Local Areas",
    "description": "Local BBS discussion and file areas",
    "acs": ""
  }
]
```

### Conference Field Descriptions

- `id` - Unique numeric identifier (must be > 0; areas with `conference_id` of 0 or omitted are ungrouped)
- `tag` - Short tag name (uppercase)
- `name` - Display name shown in area listings
- `description` - Conference description
- `acs` - ACS string required to see this conference's areas (empty = visible to all)

### How Conference Grouping Works

Message areas and file areas each have an optional `conference_id` field that links them to a conference. When listing areas:

1. Areas with `conference_id` of 0 (or omitted) appear first as ungrouped
2. Areas belonging to conferences are grouped under a conference header
3. Conference ACS is checked — if a user doesn't meet the ACS requirement, the entire conference group is hidden
4. Individual area ACS still applies independently within each conference

### Conference Header Templates

Conference headers displayed in area listings use templates in `menus/v3/templates/`:

- `MSGCONF.HDR` - Header shown before each conference group in message area listings
- `FILECONF.HDR` - Header shown before each conference group in file area listings

Template placeholders:

- `^CN` - Conference name
- `^CT` - Conference tag

### ACS Codes for Conferences

Two ACS condition codes reference the user's current conference:

- `C` - Message conference (e.g., `C1` = user is in message conference 1)
- `X` - File conference (e.g., `X1` = user is in file conference 1)

The user's current conference is set automatically when they select an area.

### Graceful Degradation

If `conferences.json` is missing or empty, the system operates as before — area listings are flat with no conference headers.

## events.json

The event scheduler configuration file defines automated tasks that run on cron-style schedules.

See the complete [Event Scheduler Guide](../advanced/event-scheduler.md) for detailed documentation.

### Basic Structure

```json
{
  "enabled": true,
  "max_concurrent_events": 3,
  "events": [
    {
      "id": "event_id",
      "name": "Event Name",
      "schedule": "*/15 * * * *",
      "command": "/path/to/command",
      "args": ["arg1", "arg2"],
      "working_directory": "/path/to/workdir",
      "timeout_seconds": 300,
      "enabled": true,
      "environment_vars": {
        "VAR_NAME": "value"
      }
    }
  ]
}
```

### Root Configuration

- **enabled** (boolean): Enable/disable the entire scheduler
- **max_concurrent_events** (integer): Maximum simultaneous event executions (default: 3)
- **events** (array): List of event configurations

### Event Configuration Fields

- **id** (string, required): Unique event identifier
- **name** (string, required): Human-readable name for logging
- **schedule** (string, required): Cron expression (e.g., `"*/15 * * * *"` or `"@hourly"`)
- **command** (string, required): Absolute path to executable
- **args** (array): Command-line arguments (each element is a separate argument)
- **working_directory** (string): Directory to run command in
- **timeout_seconds** (integer): Maximum execution time (0 = no timeout)
- **enabled** (boolean): Enable/disable this event
- **environment_vars** (object): Environment variables to set

### Cron Schedule Syntax

Standard 5-field cron format:

```text
┌─ minute (0-59)
│ ┌─ hour (0-23)
│ │ ┌─ day of month (1-31)
│ │ │ ┌─ month (1-12)
│ │ │ │ ┌─ day of week (0-6, Sunday=0)
│ │ │ │ │
* * * * *
```

**Examples:**
- `* * * * *` - Every minute
- `*/15 * * * *` - Every 15 minutes
- `0 3 * * *` - Daily at 3:00 AM
- `@hourly` - Every hour
- `@daily` - Once per day at midnight

### Available Placeholders

Use in command arguments or working_directory:

- `{TIMESTAMP}` - Unix timestamp
- `{EVENT_ID}` - Event identifier
- `{EVENT_NAME}` - Event name
- `{BBS_ROOT}` - BBS installation directory
- `{DATE}` - Current date (YYYY-MM-DD)
- `{TIME}` - Current time (HH:MM:SS)
- `{DATETIME}` - Date and time (YYYY-MM-DD HH:MM:SS)

### Common Use Cases

**FTN Mail Polling:**
```json
{
  "id": "ftn_poll",
  "schedule": "*/15 * * * *",
  "command": "/usr/local/bin/binkd",
  "args": ["-P", "21:4/158@fsxnet", "-D", "data/ftn/binkd.conf"],
  "timeout_seconds": 300,
  "enabled": true
}
```

**Daily Backup:**
```json
{
  "id": "backup",
  "schedule": "0 2 * * *",
  "command": "/usr/bin/tar",
  "args": ["-czf", "/backups/bbs-{DATE}.tar.gz", "{BBS_ROOT}/data"],
  "timeout_seconds": 7200,
  "enabled": true
}
```

See `templates/configs/events.json` for more examples.

## Menu Configuration

Menu files are located in `menus/v3/` with three components per menu:

### .MNU Files (Menu Definition)

Located in `menus/v3/mnu/`

Example `LOGIN.MNU`:

```text
RUN:FULL_LOGIN_SEQUENCE
COND:LI:GOTO:MAIN
HOTKEY:A:RUN:AUTHENTICATE
```

### .CFG Files (Menu Configuration)

Located in `menus/v3/cfg/`

Contains menu settings like:

- ACS requirements
- Password protection
- Display options

### .ANS Files (Menu Display)

Located in `menus/v3/ansi/`

ANSI art files displayed when the menu loads.

## Theme Configuration

The `menus/v3/theme.json` file controls color schemes:

```json
{
  "yesNoHighlightColor": 31,
  "yesNoRegularColor": 15
}
```

### Theme Fields

- `yesNoHighlightColor` - DOS color code for highlighted yes/no prompts
- `yesNoRegularColor` - DOS color code for regular yes/no prompts

Standard DOS color codes range from 0-255, where:

- 0-15: Standard 16 colors
- 16-231: Extended color palette
- 232-255: Grayscale

## oneliners.json

Located in the `data/` directory. Stores user-submitted one-liner messages displayed on the BBS.

### Structure

```json
[
  {
    "text": "first post from a hidden handle",
    "anonymous": true,
    "posted_by_username": "guest42",
    "posted_by_handle": "AcidBurn",
    "posted_at": "2026-02-13T17:30:00Z"
  },
  {
    "text": "long live the scene",
    "posted_by_username": "zerocool",
    "posted_by_handle": "ZeroCool",
    "posted_at": "2026-02-13T17:32:10Z"
  }
]
```

The file is a JSON array of one-liner objects. Each one-liner includes:

- `text` (displayed one-liner text, max 51 visible chars; pipe color codes are supported)
- `anonymous` (if true, on-screen display is anonymous)
- `posted_by_username` / `posted_by_handle` (actual poster identity for sysop traceability)
- `posted_at` (UTC RFC3339 timestamp)

Displayed name is derived automatically: `anonymousName` (from `strings.json`) when `anonymous=true`, otherwise `posted_by_handle` (fallback `posted_by_username`).

Legacy string-array entries are still read for backward compatibility and are normalized on write.

The system dynamically loads this file when displaying oneliners and saves new entries when users add them.

## SSH Host Keys

The `configs/` directory contains SSH host keys:

- `ssh_host_rsa_key` - RSA host key (required)
- `ssh_host_ed25519_key` - Ed25519 host key (optional)
- `ssh_host_dsa_key` - DSA host key (optional)

The RSA host key must be generated before starting the BBS:

```bash
cd configs
ssh-keygen -t rsa -f ssh_host_rsa_key -N ""
```

The BBS will fail to start if the host key is missing.

## Best Practices

1. **Backup Before Editing**: Always backup configuration files before making changes
2. **Test Changes**: Test configuration changes with a test user account
3. **Use Valid JSON**: Ensure JSON syntax is valid (use a JSON validator)
4. **Document Changes**: Keep notes on what you've customized

## Applying Configuration Changes

Most configuration changes take effect:

- **Immediately**: String changes, theme changes
- **On user login**: User-specific settings
- **On restart**: Door configurations, file areas, general config

Some changes may require a server restart:

```bash
# Stop the server (Ctrl+C)
# Start it again
./vision3
```
