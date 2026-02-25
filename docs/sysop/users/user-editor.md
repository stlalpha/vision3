# User Editor (`ue`)

The `ue` command is a full-screen TUI tool for managing user accounts on a running ViSiON/3 BBS. Run it directly from your server as the sysop — no web interface, no JSON editing required.

## Running the Editor

From your ViSiON/3 directory:

```bash
./ue                              # uses data/users/users.json by default
./ue --data /path/to/data/users/  # explicit path
```

The editor reads and writes `data/users/users.json`. It uses optimistic concurrency — if the BBS modifies the user file while you have it open, you'll be warned before any save overwrites it.

---

## Screens

### User List (Main Screen)

Displays all users in a scrollable list. Five column views are available, toggled with Left/Right arrows:

| View | Columns |
|------|---------|
| 1 | Access Level, Total Calls |
| 2 | Phone Number |
| 3 | Group/Location |
| 4 | Messages Posted, Validated |
| 5 | Last Login Date, Last Login Time |

Users can be tagged with Space for bulk operations (validate, delete).

### Field Editor

Press Enter on any user to open the field editor. Displays 28 fields across two columns:

**Left column:** Handle, Username, Real Name, Phone Number, Access Level, Total Calls, Group/Location, Access Flags, Private Note, File Points, Num Uploads, Messages Posted, Custom Prompt, Time Limit

**Right column:** Validated, Hot Keys, More Prompts, Screen Width, Screen Height, Encoding, Msg Header, Output Mode, Deleted User, Created At, Updated At, Last Login, Last Bulletin

The Password field opens a dialog — new password is bcrypt-hashed before saving.

---

## Key Bindings

### User List

| Key | Action |
|-----|--------|
| Up / Down | Scroll user list |
| Home / End | Jump to first / last user |
| PgUp / PgDn | Scroll by 13 users |
| Left / Right | Cycle column views |
| Space | Toggle tag on current user |
| Enter | Open field editor |
| F2 | Delete highlighted user (confirm) |
| Shift-F2 | Delete all tagged users (confirm) |
| F3 | Toggle alphabetical / numeric sort |
| F5 | Auto-validate highlighted user |
| Shift-F5 | Auto-validate all tagged users |
| F10 | Tag all users |
| Shift-F10 | Untag all users |
| Alt-H | Help screen |
| Esc | Exit (prompts to save if unsaved changes exist) |

### Field Editor

| Key | Action |
|-----|--------|
| Tab / Enter / Down | Edit current field, advance to next |
| Up | Move to previous field |
| Ctrl-Home | Jump to first field |
| Ctrl-End | Jump to last field |
| PgDn | Save changes + open next user |
| PgUp | Save changes + open previous user |
| F2 | Delete current user |
| F5 | Reset to default values |
| F10 | Abort (discard changes) |
| Esc | Save changes + return to list |

### Within a Field

| Key | Action |
|-----|--------|
| Left / Right | Move cursor |
| Home / End | Start / end of field |
| Insert | Toggle insert / overwrite |
| Delete | Delete character at cursor |
| Backspace | Delete character before cursor |

---

## Operations

### Auto-Validate

Sets standard approval values for a new user:

- Access Level → 10
- Validated → true
- File Points → 100
- Time Limit → 60 minutes

### Delete User

Soft-delete: sets `deletedUser=true` and records a `deletedAt` timestamp. The record stays in `users.json` but is flagged as deleted — matching the V3 soft-delete pattern used by the BBS itself.

### Sort Order

F3 toggles between alphabetical sort (by handle) and numeric sort (by user ID).

### Password Reset

Selecting the Password field in the editor opens a secure entry dialog. The new value is bcrypt-hashed before writing — the plain-text password is never stored.

---

## Technical Reference

### Architecture

Built with BubbleTea + Lipgloss, following the same patterns as the strings editor (`cmd/strings/`).

```text
cmd/ue/main.go                    # CLI entry point
internal/usereditor/
  model.go                        # BubbleTea Model, state machine, Update()
  view.go                         # List browser rendering
  view_edit.go                    # Per-user field editor rendering
  colors.go                       # DOS CGA palette mapped to Lipgloss styles
  fileio.go                       # JSON load/save with optimistic concurrency
  fields.go                       # Field definitions and edit types
  dialogs.go                      # Confirmation dialogs, help screen
```

Source data: `data/users/users.json`. The editor imports `internal/user.User` directly — no duplicate type definitions.

### File Safety

The editor uses optimistic concurrency to prevent data loss when the BBS is running simultaneously:

1. **On load** — records the mtime of `users.json`
2. **On save** — checks if mtime has changed
   - Unchanged: saves via atomic write (temp file + `os.Rename()`)
   - Changed: warns and asks whether to overwrite
3. All saves go through a temp file (`users-*.json.tmp`) to prevent partial writes

### Color Scheme

Period-accurate, matching the original DOS CGA palette from `UE.PAS`:

| Element | DOS Color | Description |
|---------|-----------|-------------|
| Title / bottom bars | Color(8,15) | DarkGray bg, White fg |
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

### Comparison with Original V2 UE.EXE

`ue` faithfully recreates `UE.EXE v1.3` (Turbo Pascal, Crimson Blade, May 1993) as a modern Go TUI.

| Feature | V2 UE.EXE | V3 ue |
|---------|-----------|-------|
| Data format | Binary record file (`USERS.`) | JSON (`users.json`) |
| Delete behavior | Hard delete (FillChar zero) | Soft delete (flag + timestamp) |
| Password storage | Plain text | bcrypt hash |
| File locking | None (DOS single-user) | Optimistic concurrency |
| User index | Separate `USERINDX.` file | Not needed (JSON) |
| Mail cleanup | Deletes from MAIL file | Not applicable |
| Demon attacks | Supported | Not in V3 |
| File ratios | UDRatio, UDKRatio, PCR | Not in V3 |
| V3-only fields | N/A | GroupLocation, Encoding, OutputMode, etc. |
