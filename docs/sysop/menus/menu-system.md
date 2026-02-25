# ViSiON/3 Menu System Guide

The ViSiON/3 menu system is the core of the BBS user interface. This guide explains how menus work and how to create or modify them.

> **📋 In Development:** A TUI menu editor (`menuedit`) is currently in development. Once complete, it will replace the need to manually edit `.MNU`, `.CFG`, and `.BAR` files. Until then, menus are configured by hand as described in this guide.

## Menu System Overview

Each menu consists of three files:

1. `.MNU` file - Menu configuration (prompts, clear screen, access control)
2. `.CFG` file - Command definitions (what happens when keys are pressed)
3. `.ANS` file - ANSI art displayed when menu loads

All menu files are located in `menus/v3/`:

- `menus/v3/mnu/` - Menu configuration files
- `menus/v3/cfg/` - Command definition files
- `menus/v3/ansi/` - ANSI art files
- `menus/v3/bar/` - Lightbar menu files (optional)

## Menu Configuration Files (.MNU)

Menu configuration files are JSON files that define menu behavior and prompts.

### Configuration Structure

```json
{
  "CLR": true,
  "USEPROMPT": true,
  "PROMPT1": "|15Main Menu: ",
  "PROMPT2": "",
  "FALLBACK": "MAIN",
  "ACS": "",
  "PASS": ""
}
```

### Configuration Fields

- `CLR` - Clear screen before displaying menu
- `USEPROMPT` - Whether to show the prompt
- `PROMPT1` - Primary prompt text (supports pipe codes and placeholders)
- `PROMPT2` - Secondary prompt (rarely used)
- `FALLBACK` - Menu to go to if no command matches
- `ACS` - Access control string for menu access
- `PASS` - Password required to access menu

## Command Definition Files (.CFG)

Command files are JSON arrays that define what happens when users press keys.

### Command Structure

```json
[
  {
    "KEYS": "M",
    "CMD": "GOTO:MSGMENU",
    "ACS": "*",
    "HIDDEN": false
  },
  {
    "KEYS": "F",
    "CMD": "GOTO:FILEM",
    "ACS": "s10",
    "HIDDEN": false
  },
  {
    "KEYS": "//",
    "CMD": "RUN:SHOWSTATS",
    "ACS": "*",
    "HIDDEN": true
  }
]
```

### Command Fields

- `KEYS` - Key(s) that trigger the command (space-separated for multiple)
- `CMD` - Action to execute
- `ACS` - Access control string
- `HIDDEN` - Whether command is hidden from display

### Special Keys

- `//` - Auto-run once per session
- `~~` - Auto-run every time menu loads
- Numbers (`1`, `2`, etc.) - Can be used to select visible commands by index

### Command Actions

#### Goto Command

Navigate to another menu:

```json
"CMD": "GOTO:MAIN"
```

#### Run Command

Execute a built-in function:

```json
"CMD": "RUN:SHOWSTATS"
```

#### Door Command

Launch external program:

```json
"CMD": "DOOR:TETRIS"
```

#### Logoff Command

Disconnect the user:

```json
"CMD": "LOGOFF"
```

## ANSI Art Files (.ANS)

ANSI files contain the visual display for menus. They support:

- Standard ANSI escape codes
- Pipe color codes (|00-|15)
- Special placeholder codes

### Pipe Color Codes

**Foreground Colors:**

```text
|00 - Black       |08 - Dark Gray
|01 - Blue        |09 - Light Blue
|02 - Green       |10 - Light Green
|03 - Cyan        |11 - Light Cyan
|04 - Red         |12 - Light Red
|05 - Magenta     |13 - Light Magenta
|06 - Brown       |14 - Yellow
|07 - Light Gray  |15 - White
```

**Background Colors:**

```text
|B0  - Black BG       |B8  - Bright Black BG
|B1  - Red BG         |B9  - Bright Red BG
|B2  - Green BG       |B10 - Bright Green BG
|B3  - Brown BG       |B11 - Bright Yellow BG
|B4  - Blue BG        |B12 - Bright Blue BG
|B5  - Magenta BG     |B13 - Bright Magenta BG
|B6  - Cyan BG        |B14 - Bright Cyan BG
|B7  - Gray BG        |B15 - Bright White BG
```

**Control Codes:**

```text
|CL - Clear screen    |DE - Clear to end of line
|CR - Carriage return  |23 - Reset attributes
```

### Special Placeholder Codes

Dynamic content placeholders in prompts and ANSI files:

- `|UH` - User handle
- `|TL` - Time left (in minutes)
- `|CA` - Current message area tag
- `|CAN` - Current message area name (resolved display name)
- `|CFA` - Current file area tag
- `|CFAN` - Current file area name (resolved display name)
- `|DATE` - Current date (MM/DD/YY)
- `|TIME` - Current time (HH:MM)
- `|CALLS` - User's total calls
- `|NODE` - Current node number
- `|MN` - Current menu name
- `|GL` - Group/Location (from user profile)
- `|UN` - User note (privateNote from user profile)
- `|CC` - Current message conference
- `|NEWUSERS` - New user registration status (`YES` or `NO`)

### AT-Code Placeholders

Dynamic `@CODE@` placeholders available in prompts and ANSI files:

- `@U@` - Number of users currently online
- `@UC@` - Total registered users

Width formatting is supported:

- `@UC:5@` - Explicit width (pad/truncate to 5 characters)
- `@UC##@` - Visual width (value fills the same columns as the placeholder)

These codes also work in Last Caller templates (LASTCALL.TOP/MID/BOT).

### Coordinate Codes

For interactive positioning (like login screens):

- `|{P}` - Mark username input position
- `|{O}` - Mark password input position

## Access Control Strings (ACS)

Control who can access menus and commands:

- `*` - All users (including unauthenticated)
- `s10` - Security level 10+
- `fA` - Must have flag A
- `!fB` - Must NOT have flag B
- `s20&fC` - Level 20+ AND flag C
- `s10|fD` - Level 10+ OR flag D

## Creating a New Menu

### Step 1: Create Menu Configuration

Create `menus/v3/mnu/MYMENU.MNU`:

```json
{
  "CLR": true,
  "USEPROMPT": true,
  "PROMPT1": "|15Select: ",
  "PROMPT2": "",
  "FALLBACK": "MAIN",
  "ACS": "",
  "PASS": ""
}
```

### Step 2: Create Command Definitions

Create `menus/v3/cfg/MYMENU.CFG`:

```json
[
  {
    "KEYS": "1",
    "CMD": "RUN:OPTION1",
    "ACS": "*",
    "HIDDEN": false
  },
  {
    "KEYS": "2",
    "CMD": "RUN:OPTION2",
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

### Step 3: Create ANSI Art

Create `menus/v3/ansi/MYMENU.ANS` with your menu design.

### Step 4: Link to Menu

Add a command in another menu to access it:

```json
{
  "KEYS": "M",
  "CMD": "GOTO:MYMENU",
  "ACS": "*",
  "HIDDEN": false
}
```

## Pre-Login Matrix Screen

The matrix screen is a pre-authentication menu shown to telnet users before the LOGIN screen. It is based on the original ViSiON/2 Pascal `ConfigPdMatrix` procedure from `GETLOGIN.PAS` and provides a lightbar menu for choosing an action before authentication.

SSH users with known accounts skip the matrix screen entirely (auto-login to MAIN). All other users see the matrix first.

### Configuration

The matrix uses the standard menu file system with three files:

1. `menus/v3/bar/PDMATRIX.BAR` — Lightbar option layout (positions, colors, hotkeys, text)
2. `menus/v3/cfg/PDMATRIX.CFG` — Command definitions mapping hotkeys to actions
3. `menus/v3/mnu/PDMATRIX.MNU` — Menu settings (optional, for consistency)

**PDMATRIX.BAR:**

```ini
; Pre-login matrix menu
; FORMAT: X,Y,HiLitedColor,RegularColor,HotKey,ReturnValue,DisplayText
49,10,31,5,J,UNUSED, Journey onward.
49,11,31,5,C,UNUSED, Create an account.
49,12,31,5,A,UNUSED, Check your access.
49,13,31,5,D,UNUSED, Disconnect.
```

**PDMATRIX.CFG:**

```json
[
    {"KEYS": "J", "CMD": "LOGIN", "ACS": "*", "HIDDEN": false},
    {"KEYS": "C", "CMD": "NEWUSER", "ACS": "*", "HIDDEN": false},
    {"KEYS": "A", "CMD": "CHECKACCESS", "ACS": "*", "HIDDEN": false},
    {"KEYS": "D", "CMD": "DISCONNECT", "ACS": "*", "HIDDEN": false}
]
```

### BAR File Fields

Each line defines one menu option (see [Lightbar Menus](#lightbar-menus) for format details):

- `X, Y` - Screen coordinates (column, row) where the option text is drawn
- `HiLitedColor` - DOS color code when selected (e.g., 31 = bright white on blue)
- `RegularColor` - DOS color code when not selected (e.g., 5 = magenta on black)
- `HotKey` - Key that directly selects this option (must match `KEYS` in the CFG file)
- `ReturnValue` - Unused (set to `UNUSED`)
- `DisplayText` - Text drawn at the specified coordinates

### CFG File Fields

Standard command definition format (see [Command Definition Files](#command-definition-files-cfg)):

- `KEYS` - Hotkey matching the BAR file's HotKey field
- `CMD` - Action to execute (see below)
- `ACS` - Access control (use `*` for pre-login)
- `HIDDEN` - Whether command is hidden

### Available Actions

| Action        | Behavior                                                                                                |
| ------------- | ------------------------------------------------------------------------------------------------------- |
| `LOGIN`       | Displays a random PRELOGON ANSI file (if any exist), then proceeds to the LOGIN menu for authentication |
| `NEWUSER`     | Launches the new user registration form, then returns to the matrix                                     |
| `CHECKACCESS` | Prompts for a username and displays account validation status, then returns to the matrix               |
| `DISCONNECT`  | Disconnects the session                                                                                 |

### Navigation

- **Up/Down arrows** - Move selection with wrapping
- **Number keys (1-9)** - Direct selection by index (always enabled)
- **Hotkey** - Press the option's HotKey character (e.g., J, C, A, D)
- **Enter** - Confirm current selection
- **Spacebar** - Redraw the screen

After 10 actions without logging in, the session is automatically disconnected (matching the original Pascal `Tries` limit).

### ANSI Art

Place a `PDMATRIX.ANS` file in `menus/v3/ansi/` with the visual layout. The option text from the BAR file is drawn over the ANSI art at the specified X,Y coordinates — design the art to leave space for option text at those positions.

### Disabling the Matrix Screen

If `menus/v3/bar/PDMATRIX.BAR` does not exist or cannot be loaded, the matrix screen is skipped and telnet users go directly to the LOGIN menu. The same applies if the CFG file or ANSI file is missing.

## Pre-Login ANSI Files (PRELOGON)

When a user selects "Journey onward" (LOGIN action) from the matrix screen, a PRELOGON ANSI file is displayed before the login screen appears. This matches the original ViSiON/2 Pascal behavior (`Printfile(PRELOGON.x) + HoldScreen`).

### File Naming

The system looks for numbered files first, then falls back to a single file:

1. **Numbered files**: `PRELOGON.1`, `PRELOGON.2`, `PRELOGON.3`, ... (up to `PRELOGON.20`)
2. **Single file**: `PRELOGON.ANS`

All files are placed in `menus/v3/ansi/`.

### Random Selection

If multiple numbered files exist (e.g., `PRELOGON.1` through `PRELOGON.5`), one is chosen at random each time. Numbering must be sequential starting from 1 — the system stops searching at the first gap.

### Pause Behavior

After displaying the PRELOGON screen, the system shows the `pauseString` from `configs/strings.json` and waits for the user to press Enter before proceeding to the LOGIN screen.

### Disabling PRELOGON

If no PRELOGON files exist in `menus/v3/ansi/`, the step is silently skipped and the user goes directly to the LOGIN screen.

## Menu Flow Examples

### Login Flow

1. Telnet users see the pre-login matrix screen (`PDMATRIX.ANS`) — see above
2. User selects "Journey onward" (or SSH users arrive here directly)
3. A random PRELOGON ANSI file is displayed (if any exist), followed by a pause prompt
4. `LOGIN` menu displays login screen with coordinate codes
5. User enters credentials at marked positions
6. On success: System goes to `FASTLOGN` or `MAIN`
7. On failure: Stay at `LOGIN`

### Main Menu Flow

1. Load `MAIN.MNU` configuration
2. Execute auto-run commands (`//` once, `~~` always)
3. Clear screen if `CLR` is true
4. Display `MAIN.ANS`
5. Show prompt from `PROMPT1` if `USEPROMPT` is true
6. Wait for user input
7. Match input against commands in `MAIN.CFG`
8. Execute matched command or use `FALLBACK`

## Advanced Features

### Auto-Run Commands

Commands that execute automatically:

```json
{
  "KEYS": "//",
  "CMD": "RUN:SHOWSTATS",
  "ACS": "*",
  "HIDDEN": true
}
```

- `//` - Runs once per session per menu
- `~~` - Runs every time the menu loads

### Menu Passwords

Set password in .MNU file:

```json
{
  "PASS": "secret"
}
```

Users must enter the password to access the menu.

### Lightbar Menus

Enable cursor-driven selection by creating a `.BAR` file:

`menus/v3/bar/MYMENU.BAR`:

```ini
1,5,31,15,1,UNUSED,Normal Login
1,6,31,15,2,UNUSED,New User Application
```

Format: X,Y,HighlightColor,NormalColor,Hotkey,Unused,DisplayText

## Built-in Functions

Functions available via `RUN:` command:

### Authentication & User Management

- `AUTHENTICATE` - Prompt for login
- `FULL_LOGIN_SEQUENCE` - Complete login process
- `NEWUSER` - New user registration form
- `SHOWSTATS` - Display user statistics
- `LISTUSERS` - List all users
- `LASTCALLERS` - Show recent callers

### Messaging System

- `LISTMSGAR` - List message areas
- `COMPOSEMSG` - Write new message
- `READMSGS` - Read messages
- `NEWSCAN` - Scan for new messages

### File System

- `LISTFILES` - List files in current area
- `LISTFILEAR` - List file areas
- `SELECTFILEAREA` - Choose file area

### Private Mail

- `SENDPRIVMAIL` - Send private mail to another user
- `READPRIVMAIL` - Read private mail addressed to current user
- `LISTPRIVMAIL` - List private mail messages

### Other Functions

- `ONELINER` - One-liner system
- `SHOWVERSION` - Display BBS version
- `TOGGLEALLOWNEWUSERS` - Toggle new user registration open/closed (SysOp only)

## Template Files (.TOP / .MID / .BOT)

Several built-in functions display content using a three-part template system:

- `NAME.TOP` — Header, displayed once before any rows
- `NAME.MID` — Row template, repeated for each data record
- `NAME.BOT` — Footer, displayed once after all rows

All template files live in `menus/v3/templates/`.

### Encoding

Template files are raw CP437/ANSI binary files. The `.gitattributes` in this repository marks them as binary to prevent line-ending or encoding conversion by git or editors.

### `.ans` Extension Support

Any template file can optionally be stored with a `.ans` suffix appended to its name (e.g. `ONELINER.TOP.ans` instead of `ONELINER.TOP`). The system tries the bare name first and falls back to the `.ans` variant automatically.

This allows ANSI art editors (Moebius, PabloDraw, TheDraw, etc.) to recognise and open the file by extension without requiring a rename. The message header templates in `menus/v3/templates/message_headers/` already use `.ans` exclusively; this fallback extends that convention to all template files.

**Workflow:**

1. Rename `ONELINER.TOP` → `ONELINER.TOP.ans`
2. Open and edit in your ANSI editor
3. Save — the system finds it automatically

### Common Notes

- Pipe color delimiters are normalized; both `|` and broken-bar variants are accepted.
- SAUCE metadata is stripped automatically before display.
- `.TOP` and `.BOT` files contain raw ANSI/CP437 art. `.MID` files typically use pipe codes and placeholder tokens.

### Last Callers (`RUN:LASTCALLERS`)

`LASTCALLERS` shows recent caller history using template files in `menus/v3/templates/`.

Command usage examples:

```json
{
  "KEYS": "LC",
  "CMD": "RUN:LASTCALLERS 10",
  "ACS": "*",
  "HIDDEN": false
}
```

- `RUN:LASTCALLERS` - Uses default limit of 10 entries
- `RUN:LASTCALLERS 25` - Shows last 25 entries

Template files:

- `LASTCALL.TOP` - Header (displayed once)
- `LASTCALL.MID` - Row template (displayed once per caller)
- `LASTCALL.BOT` - Footer (displayed once)

Supported row tokens in `LASTCALL.MID` (`@CODE@` / `@CODE:width@`):

- `@LO@` - Logon time
- `@LT@` - Logoff time
- `@UN@` - User handle
- `@NOTE@` - User note (from user private note)
- `@ND@` - Node number
- `@CA@` - Caller number
- `@TO@` - Time on (minutes)
- `@LC@` - Location/group
- `@NU@` - New user marker (`*` or space)

Global token (header/footer/rows):

- `@UC@` - Total registered users (also `@USERCT@`)
- `@U@` - Number of users currently online

Width formatting is supported for all tokens:

- `@CODE:N@` - Explicit width (pad/truncate to N characters)
- `@CODE##@` - Visual width (value fills the same columns as the placeholder)

Legacy caret placeholders (`^UN`, `^ND`, etc.) remain supported for compatibility.

## Special Menu Names

Some menu names have special behavior:

- `LOGIN` - Uses coordinate codes for positioned input
- `MAIN` - Typically the main menu after login
- `FASTLOGN` - Fast login menu (often shows news/stats)

## Troubleshooting

### Menu Not Loading

- Check file names match exactly (case-sensitive)
- Verify .MNU and .CFG files are valid JSON
- Ensure .ANS file exists if referenced
- Review logs for error messages

### Commands Not Working

- Verify command syntax in .CFG file
- Check ACS requirements are met
- Ensure function is registered for RUN commands
- Check door configuration for DOOR commands

### Display Issues

- Ensure ANSI file uses correct encoding
- Test with different terminal types
- Check for unmatched pipe codes
- Verify placeholder codes are spelled correctly
