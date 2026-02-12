# Login Sequence

The ViSiON/3 BBS includes a configurable login sequence that controls what happens after a user authenticates but before they reach the main menu. Sysops can customize the order, content, and visibility of each login step without modifying code.

## Overview

The login sequence provides:

- **Configurable item order**: Arrange login steps in any order
- **Built-in commands**: Last callers, oneliners, user stats, new mail scan, and more
- **ANSI file display**: Show bulletins, welcome screens, or news files
- **External scripts**: Run door programs or scripts during login
- **Security level gating**: Show or skip items based on user access level
- **Per-item options**: Clear screen before, pause after, or both

## Configuration

The login sequence is configured in `configs/login.json`. If the file is missing, a built-in default sequence is used (Last Callers, Oneliners, User Stats) to maintain backward compatibility.

### Basic Structure

```json
[
    {
        "command": "LASTCALLS",
        "clear_screen": false,
        "pause_after": false,
        "sec_level": 0
    },
    {
        "command": "ONELINERS"
    },
    {
        "command": "USERSTATS"
    }
]
```

The file is a JSON array of login items. Items are executed in order from top to bottom. Fields with default values can be omitted.

### Configuration Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| **command** | string | Yes | — | The login command to execute (see Built-in Commands below) |
| **data** | string | No | — | Command-specific data (filename, script path, etc.) |
| **clear_screen** | boolean | No | `false` | Clear the screen before executing this item |
| **pause_after** | boolean | No | `false` | Show a "Press [ENTER] to continue" prompt after this item |
| **sec_level** | integer | No | `0` | Minimum security level required to see this item (0 = everyone) |

## Built-in Commands

### LASTCALLS

Displays the last callers list using templates from `menus/v3/templates/` (`LASTCALL.TOP`, `LASTCALL.MID`, `LASTCALL.BOT`).

```json
{"command": "LASTCALLS"}
```

### ONELINERS

Displays recent oneliners and prompts the user to add a new one. Uses templates from `menus/v3/templates/` (`ONELINER.TOP`, `ONELINER.MID`, `ONELINER.BOT`). Data is stored in `data/oneliners.json`.

```json
{"command": "ONELINERS"}
```

### USERSTATS

Displays user statistics using the `YOURSTAT.ANS` file from `menus/v3/ansi/`. Supports pipe code placeholders for user data (`|UH` for handle, `|UL` for level, etc.).

```json
{"command": "USERSTATS"}
```

### NMAILSCAN

Scans the PRIVMAIL message area for unread private mail addressed to the current user and displays a count. Requires a `PRIVMAIL` tagged message area to be configured in `configs/message_areas.json`. If the area is not configured, the item is silently skipped.

```json
{"command": "NMAILSCAN"}
```

**Example output:**
```
You have 3 new private mail message(s).
```

### DISPLAYFILE

Displays an ANSI art file from the `menus/v3/ansi/` directory. The **data** field specifies the filename. Useful for bulletins, welcome screens, system news, or any custom display.

```json
{"command": "DISPLAYFILE", "data": "BULLETIN.ANS"}
```

If the file does not exist, a warning is logged and the sequence continues.

### RUNDOOR

Executes an external script or program during the login sequence. The **data** field contains the full path to the script. The node number is passed as the first command-line argument.

```json
{"command": "RUNDOOR", "data": "/opt/bbs/scripts/login_script.sh"}
```

**Script invocation:**
```bash
/opt/bbs/scripts/login_script.sh <node_number>
```

The script's stdin, stdout, and stderr are connected to the user's terminal session. If the script is not found or exits with an error, a warning is logged and the sequence continues.

### FASTLOGIN

Presents the fast login / quick login menu inline within the login sequence. Displays `FASTLOGN.ANS` and loads options from `menus/v3/cfg/FASTLOGN.CFG`. The user can choose to:

- Continue with the remaining login sequence items
- Skip directly to the main menu
- Jump to the file or message menu

```json
{"command": "FASTLOGIN"}
```

If the user selects a jump option (e.g., skip to MAIN), the remaining login items are skipped and the user goes directly to the chosen menu.

## Security Level Gating

Each item can require a minimum security level. If the user's access level is below the threshold, the item is silently skipped.

```json
[
    {"command": "LASTCALLS"},
    {"command": "ONELINERS"},
    {"command": "USERSTATS"},
    {"command": "DISPLAYFILE", "data": "SYSOP_NEWS.ANS", "sec_level": 200}
]
```

In this example, `SYSOP_NEWS.ANS` is only shown to users with access level 200 or higher.

## Examples

### Default (Legacy-Compatible)

This matches the original hard-coded behavior:

```json
[
    {"command": "LASTCALLS"},
    {"command": "ONELINERS"},
    {"command": "USERSTATS"}
]
```

### Full-Featured Login

```json
[
    {"command": "DISPLAYFILE", "data": "WELCOME.ANS", "clear_screen": true},
    {"command": "NMAILSCAN"},
    {"command": "LASTCALLS"},
    {"command": "ONELINERS"},
    {"command": "DISPLAYFILE", "data": "BULLETIN.ANS", "clear_screen": true, "pause_after": true},
    {"command": "USERSTATS"},
    {"command": "FASTLOGIN"}
]
```

This sequence:
1. Clears the screen and shows a welcome ANSI
2. Scans for new private mail
3. Shows last callers
4. Shows oneliners (with add prompt)
5. Clears screen, shows a bulletin, and pauses
6. Shows user statistics
7. Offers the fast login menu to skip or jump

### Minimal Login

```json
[
    {"command": "NMAILSCAN"},
    {"command": "FASTLOGIN"}
]
```

Scans for mail, then immediately offers the fast login menu.

### With External Script

```json
[
    {"command": "LASTCALLS"},
    {"command": "RUNDOOR", "data": "/opt/bbs/scripts/weather.sh"},
    {"command": "USERSTATS", "pause_after": true}
]
```

Runs a weather display script between last callers and user stats.

### Tiered Access

```json
[
    {"command": "LASTCALLS"},
    {"command": "ONELINERS"},
    {"command": "USERSTATS"},
    {"command": "DISPLAYFILE", "data": "COSYSOP.ANS", "sec_level": 100},
    {"command": "DISPLAYFILE", "data": "SYSOP.ANS", "sec_level": 200}
]
```

Co-sysops (level 100+) see `COSYSOP.ANS`. Sysops (level 200+) see both `COSYSOP.ANS` and `SYSOP.ANS`.

## How It Works

The login sequence is loaded at BBS startup from `configs/login.json` and stored in the menu executor. Both SSH and telnet users go through the FASTLOGN menu after authentication. When the user selects the full login sequence (or when FASTLOGN routes to `RUN:FULL_LOGIN_SEQUENCE`), the sequence is executed:

1. For each item in the array (in order):
   - Check `sec_level` against the user's access level — skip if too low
   - Clear the screen if `clear_screen` is `true`
   - Execute the command handler
   - Show pause prompt if `pause_after` is `true`
   - If any handler returns a navigation action (GOTO or LOGOFF), the sequence ends immediately
2. After all items complete, the user transitions to the MAIN menu

### Error Handling

- **Missing files**: DISPLAYFILE and RUNDOOR log warnings but continue the sequence
- **Missing PRIVMAIL area**: NMAILSCAN silently skips if the area is not configured
- **User disconnect**: Detected via EOF and properly handled (session ends)
- **Script errors**: RUNDOOR logs the error but continues with the next item
- **Missing login.json**: The built-in default sequence (LASTCALLS, ONELINERS, USERSTATS) is used

## Menu System Integration

All login sequence commands are also registered as menu runnables and can be used from any menu via `RUN:` commands:

| Command | Menu Usage |
|---------|-----------|
| LASTCALLS | `RUN:LASTCALLERS` (existing) |
| ONELINERS | `RUN:ONELINER` (existing) |
| USERSTATS | `RUN:SHOWSTATS` (existing) |
| NMAILSCAN | `RUN:NMAILSCAN` |
| DISPLAYFILE | `RUN:DISPLAYFILE <filename>` |
| RUNDOOR | `RUN:RUNDOOR <script_path>` |
| FASTLOGIN | `RUN:FASTLOGIN` |

## File Locations

| File | Purpose |
|------|---------|
| `configs/login.json` | Active login sequence configuration |
| `templates/configs/login.json` | Default template for new installations |
| `menus/v3/ansi/FASTLOGN.ANS` | Fast login menu ANSI art |
| `menus/v3/cfg/FASTLOGN.CFG` | Fast login menu command definitions |
| `menus/v3/ansi/YOURSTAT.ANS` | User statistics ANSI art |
| `menus/v3/templates/LASTCALL.*` | Last callers display templates |
| `menus/v3/templates/ONELINER.*` | Oneliners display templates |
