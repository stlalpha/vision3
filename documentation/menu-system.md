# ViSiON/3 Menu System Guide

The ViSiON/3 menu system is the core of the BBS user interface. This guide explains how menus work and how to create or modify them.

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

```
|00 - Black       |08 - Dark Gray
|01 - Red         |09 - Light Red
|02 - Green       |10 - Light Green
|03 - Brown       |11 - Yellow
|04 - Blue        |12 - Light Blue
|05 - Magenta     |13 - Light Magenta
|06 - Cyan        |14 - Light Cyan
|07 - Light Gray  |15 - White
```

### Special Placeholder Codes

Dynamic content placeholders in prompts and ANSI files:
- `|UH` - User handle
- `|TL` - Time left (in minutes)
- `|CA` - Current area (message or file area tag)
- `|DATE` - Current date (MM/DD/YY)
- `|TIME` - Current time (HH:MM)
- `|CALLS` - User's total calls
- `|NODE` - Current node number
- `|MN` - Current menu name

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

## Menu Flow Examples

### Login Flow
1. `LOGIN` menu displays login screen with coordinate codes
2. User enters credentials at marked positions
3. On success: System goes to `FASTLOGN` or `MAIN`
4. On failure: Stay at `LOGIN`

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
```
1,5,31,15,1,UNUSED,Normal Login
1,6,31,15,2,UNUSED,New User Application
```

Format: X,Y,HighlightColor,NormalColor,Hotkey,Unused,DisplayText

## Built-in Functions

Functions available via `RUN:` command:

### Authentication & User Management
- `AUTHENTICATE` - Prompt for login
- `FULL_LOGIN_SEQUENCE` - Complete login process
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

### Other Functions
- `ONELINER` - One-liner system
- `SHOWVERSION` - Display BBS version
- `READMAIL` - Read private mail (placeholder)

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