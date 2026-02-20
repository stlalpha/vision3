# User Editor (UE) - Standalone TUI Tool

## Overview

The User Editor (`cmd/ue/`) is a standalone terminal-based user account management tool for ViSiON/3 BBS. It faithfully recreates the original Vision/2 `UE.EXE v1.3` (written in Turbo Pascal by Crimson Blade, May 1993) as a modern Go TUI application.

## Usage

```bash
# Default (looks for data/users/users.json relative to cwd)
./ue

# Explicit path to users directory
./ue --data /path/to/data/users/
```

## Architecture

Built with BubbleTea + Lipgloss, following the same patterns as the strings editor (`cmd/strings/`).

```
cmd/ue/main.go                         # CLI entry point
internal/usereditor/
  model.go                             # BubbleTea Model, state machine, Update()
  view.go                              # List browser rendering
  view_edit.go                         # Per-user field editor rendering
  colors.go                            # DOS CGA palette mapped to Lipgloss styles
  fileio.go                            # JSON load/save with optimistic concurrency
  fields.go                            # Field definitions and edit types
  dialogs.go                           # Confirmation dialogs, help screen
```

## Screens

### List Browser (Main Screen)

The primary screen showing all users in a scrollable list with 13 visible rows centered on the cursor. Five column views are available, toggled with Left/Right arrows:

| View | Columns |
|------|---------|
| 1 | Access Level, Total Calls |
| 2 | Phone Number |
| 3 | Group/Location |
| 4 | Messages Posted, Validated |
| 5 | Last Login Date, Last Login Time |

Users can be tagged (Space key) for mass operations.

### Field Editor

Opened by pressing Enter on a user in the list. Displays 28 fields in two columns:

**Left Column:**
Handle, Username, Real Name, Phone Number, Access Level, Total Calls, Group/Location, Access Flags, Private Note, File Points, Num Uploads, Messages Posted, Custom Prompt, Time Limit

**Right Column:**
Validated, Hot Keys, More Prompts, Screen Width, Screen Height, Encoding, Msg Header, Output Mode, Deleted User, Created At (read-only), Updated At (read-only), Last Login (read-only), Last Bulletin (read-only)

**Special:** Password field (triggers password reset dialog with bcrypt hashing)

## Key Bindings

### List Mode

| Key | Action |
|-----|--------|
| Up/Down | Scroll user list |
| Home/End | Jump to first/last user |
| PgUp/PgDn | Scroll by 13 users |
| Left/Right | Change column view (5 views) |
| Space | Toggle tag on current user |
| Enter | Open field editor for highlighted user |
| F2 | Delete highlighted user (confirm) |
| Shift-F2 | Delete all tagged users (confirm) |
| F3 | Toggle alphabetical/numeric sort |
| F5 | Auto-validate highlighted user |
| Shift-F5 | Auto-validate all tagged users |
| F10 | Tag all users |
| Shift-F10 | Untag all users |
| Alt-H | Show help screen |
| Esc | Exit (prompts to save if changes exist) |

### Edit Mode

| Key | Action |
|-----|--------|
| Tab/Enter/Down | Edit current field, then move to next |
| Up | Move to previous field |
| Ctrl-Home | Jump to first field |
| Ctrl-End | Jump to last field |
| PgDn | Save changes + next user |
| PgUp | Save changes + previous user |
| F2 | Delete current user |
| F5 | Set to default values |
| F10 | Abort (discard changes) |
| Esc | Save changes + return to list |

### Field Editing

| Key | Action |
|-----|--------|
| Left/Right | Move cursor within field |
| Home/End | Start/end of field |
| Insert | Toggle insert/overwrite mode |
| Delete | Delete character at cursor |
| Backspace | Delete character before cursor |

## Special Operations

### Delete User
Performs a soft-delete: sets `deletedUser=true` and records `deletedAt` timestamp. The user record is preserved in `users.json` but marked as deleted. This matches the V3 soft-delete pattern used by the BBS.

### Auto-Validate (Set Defaults)
Sets standard field values for new user approval:
- Access Level = 10
- Validated = true
- File Points = 100
- Time Limit = 60 minutes

### Alphabetize
Toggles between sorting users alphabetically by handle and numerically by user ID.

### Password Reset
In the field editor, selecting the Password field opens a password entry dialog. The new password is hashed with bcrypt before storing.

## File Safety / Concurrency

The editor uses optimistic concurrency to prevent data loss when the BBS is running simultaneously:

1. **On load**: Records the file modification time (mtime) of `users.json`
2. **On save**: Checks if mtime has changed since load
   - If unchanged: saves normally via atomic write (temp file + rename)
   - If changed: warns the user and asks whether to overwrite
3. **Atomic writes**: All saves go through a temp file (`users-*.json.tmp`) then `os.Rename()`, preventing partial writes

This approach requires no changes to the BBS server code and protects against accidental overwrites.

## Color Scheme

All colors are period-accurate, matching the original DOS CGA palette from UE.PAS:

| Element | DOS Color | Description |
|---------|-----------|-------------|
| Title/Bottom bars | Color(8,15) | DarkGray bg, White fg |
| Background fill | Fill_Screen('░',7,1) | Gray on Blue |
| List box border | Color(1,9) | Blue bg, LightBlue fg |
| List header | Color(1,14) | Blue bg, Yellow fg |
| Column titles | Color(9,15) | LightBlue bg, White fg |
| Normal list item | Color(1,15) | Blue bg, White fg |
| Highlighted item | Color(0,14) | Black bg, Yellow fg |
| Tagged marker | Color(1,14) | Blue bg, Yellow fg |
| Field labels | PColor=31 | Blue bg, White fg |
| Field values | NormColor=30 | Blue bg, Yellow fg |
| Edit cursor | InColor=14 | Yellow fg |
| Dialog boxes | Color(5,14/15) | Magenta bg, Yellow/White fg |
| Help screen | Color(4,14/15) | Red bg, Yellow/White fg |

## Comparison with Original V2 UE.EXE

| Feature | V2 UE.EXE | V3 UE |
|---------|-----------|-------|
| Data format | Binary record file (USERS.) | JSON (users.json) |
| Delete behavior | Hard delete (FillChar zero) | Soft delete (flag + timestamp) |
| Password storage | Plain text | Bcrypt hash |
| File locking | None (DOS single-user) | Optimistic concurrency |
| User index | Separate USERINDX. file | Not needed (JSON) |
| Mail cleanup | Deletes from MAIL file | Not applicable |
| Demon attacks | Supported | Not in V3 |
| File ratios | UDRatio, UDKRatio, PCR | Not in V3 |
| V3-only fields | N/A | GroupLocation, Encoding, OutputMode, etc. |

## Data File

Source: `data/users/users.json`

The editor imports `internal/user.User` struct directly - no duplicate type definitions. All fields match the User struct defined in `internal/user/user.go`.
