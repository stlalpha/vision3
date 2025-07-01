# ViSiON/3 Configuration Guide

This guide covers the configuration files used by ViSiON/3 BBS.

## Configuration Files Overview

Configuration files are split between two directories:

**In `configs/` directory:**
- `strings.json` - Customizable text strings and prompts
- `doors.json` - External door program configurations
- `file_areas.json` - File area definitions
- `config.json` - General BBS configuration
- SSH host keys (`ssh_host_rsa_key`, etc.)

**In `data/` directory:**
- `message_areas.json` - Message area definitions
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

**New User Strings:**
- `newUserNameStr` - New user alias prompt
- `createAPassword` - New user password creation
- `enterRealName` - Real name prompt
- `enterNumber` - Phone number prompt

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
    "acs_download": ""
  }
]
```

### Field Descriptions

- `id` - Unique numeric identifier
- `tag` - Short tag for the area (uppercase)
- `name` - Display name
- `description` - Area description
- `path` - Subdirectory under `data/files/`
- `acs_list` - ACS string required to list files
- `acs_upload` - ACS string required to upload
- `acs_download` - ACS string required to download

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
  "sysOpLevel": 255,
  "coSysLevel": 250,
  "logonLevel": 100
}
```

### Field Descriptions

- `boardName` - BBS name displayed to users
- `boardPhoneNumber` - Phone number (historical/display purposes)
- `sysOpLevel` - Security level for SysOp access
- `coSysLevel` - Security level for Co-SysOp access
- `logonLevel` - Security level granted after successful login

## message_areas.json

Located in the `data/` directory (not `configs/`). Defines message areas available on the BBS.

See [Message Areas Guide](message-areas.md) for detailed configuration.

## Menu Configuration

Menu files are located in `menus/v3/` with three components per menu:

### .MNU Files (Menu Definition)
Located in `menus/v3/mnu/`

Example `LOGIN.MNU`:
```
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
  "|12THiNK ELiTE|08.. |15DiAL FAST|08.. |09HANG UP LAST|08! |07-|15acidburn",
  "|10RaZoR 1911|08, |11TRiSTAR|08, |14FAiRLiGHT |07- |15THE LEGENDS LiVE ON|08!",
  "|09Got |150-day warez|09? |07Trade ratio |151:3 |07or |14GET OUT|08! |07-|13k-rad",
  "|08[|15SYSOP|08] |12iF YOU AiN'T |10ELiTE|12, YOU AiN'T |14NOTHiNG|08!",
  "|11Just grabbed |15DOOM II |11off a |14Euro courier|11 - |100-hour! |07-|15cyber",
  "|13No |09LAMERS|13, No |10AOLers|13, No |12NARCs |07- |15REAL SCENE ONLY",
  "|15New |10THG|15 release in |14File Area #3 |07- |09GET iT FAST! |07-|11phoenix",
  "|08Running |14USR Courier v.Everything |08@ |1528.8k |07- |12BLAZING SPEEDS!",
  "|12ViSiON/2 |07was |15THE BEST|07.. |10ViSiON/3 |07will |14RULE THEM ALL|08!",
  "|09Shouts to |15INC|09, |10HYBRID|09, |11PWA |09& |14RiSC |07- |13You know who you are!"
]
```

The file is a simple JSON array of strings. Each one-liner can include:
- User messages
- Pipe color codes (|00-|15)
- Any text up to the configured line length

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