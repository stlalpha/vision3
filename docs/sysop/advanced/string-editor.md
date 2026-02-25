# ViSiON/3 String Editor

The string editor (`strings`) is a TUI tool for editing `configs/strings.json`, which contains all customizable text prompts and messages displayed by the BBS. It is a Go reimplementation of the original Vision/2 `STRINGS.EXE` Turbo Pascal utility.

## Running

```bash
./strings                           # Edit configs/strings.json (default)
./strings --config path/to/file.json  # Edit a specific strings file
./strings --help                     # Show usage
```

## Interface

The editor uses a fullscreen 80×25 terminal layout matching the DOS original:

```text
 Current Topic Number: 1 │ ViSiON/3 BBS String Configuration │ Current Page: 1
  #  Name                     Value
  1 [Default User's Prompt ] |08██ |15|MN |08██ |13|TL |05Left|08:
  2 [System Pause String   ] |15█|07█|08█|B1|09█ |15Stroke Me! |09█...
  3 [System Password String] |08█|07█|15█ |09Login Password|01:
  ...
 (20 items per page)

 This is the Default prompt for new users
 ↑↓ Navigate │ PgUp/PgDn Pages │ Enter Edit │ F1 Edit(Prefill) │ F10 Save │ ...
```

- **Row 1** — Blue status bar showing current topic number, title, and page
- **Row 2** — Column headers (Name / Value)
- **Rows 3–22** — 20 items per page with label and color-rendered value
- **Row 24** — Description of the currently highlighted string
- **Row 25** — Yellow-on-blue keyboard shortcut reference

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move cursor up/down |
| `PgUp` / `PgDn` | Previous/next page |
| `Home` / `End` | Jump to first/last item |
| `Enter` | Edit selected string (blank input field) |
| `F1` | Edit selected string (pre-filled with current value) |
| `F3` | Revert selected string to its last-saved value |
| `F4` | Restore selected string to ViSiON/3 default (from templates/) |
| `F10` | Save changes and exit |
| `Esc` | Abort — shows confirmation dialog if unsaved changes |
| `/` | Search/filter strings by name |

### Edit Mode

When editing a string value:

- **Enter** — Save the new value
- **Esc** — Cancel editing without changes

### Confirmation Dialogs

Several actions display a centered Yes/No confirmation dialog before proceeding:

- **Esc** (with unsaved changes) — "Abort Without Saving?"
- **F3** — "Revert to Last Saved?"
- **F4** — "Restore ViSiON/3 Default?"

In any confirmation dialog:

- **←/→** — Toggle between Yes/No
- **Enter** — Confirm selection
- **Y** / **N** — Quick accept/reject
- **Esc** — Cancel and return to navigation

## BBS Color Codes

String values support BBS pipe codes that are rendered with color in the editor. These same codes produce colored output when displayed to connected users at runtime.

### Foreground Colors (`|00` – `|15`)

| Code | Color |
|------|-------|
| `\|00` | Black |
| `\|01` | Dark Blue |
| `\|02` | Dark Green |
| `\|03` | Dark Cyan |
| `\|04` | Dark Red |
| `\|05` | Dark Magenta |
| `\|06` | Brown/Dark Yellow |
| `\|07` | Light Gray |
| `\|08` | Dark Gray |
| `\|09` | Light Blue |
| `\|10` | Light Green |
| `\|11` | Light Cyan |
| `\|12` | Light Red |
| `\|13` | Light Magenta |
| `\|14` | Yellow |
| `\|15` | White |

### Background Colors (`|B0` – `|B7`)

| Code | Color |
|------|-------|
| `\|B0` | Black background |
| `\|B1` | Red background |
| `\|B2` | Green background |
| `\|B3` | Brown/Yellow background |
| `\|B4` | Blue background |
| `\|B5` | Magenta background |
| `\|B6` | Cyan background |
| `\|B7` | Light Gray background |

### Special Codes

| Code | Meaning |
|------|---------|
| `\|CR` | Carriage return / newline |
| `\|CL` | Clear screen |
| `\|DE` | Clear to end of line |
| `@` | Yes/No selection bar |

### Dollar Codes (`$x`)

Legacy Vision/2 color codes using `$` followed by a letter. These map to the same 16 DOS colors (e.g., `$a` = dark blue, `$W` = white).

### MCI Codes

Runtime placeholder codes like `|MN` (menu name), `|TL` (time left), `|CB` (current board), `|UN` (user number), etc. These are displayed as literal text in the editor but expanded by the BBS server when shown to users.

## File Format

`configs/strings.json` is a flat JSON object mapping string keys to their values:

```json
{
    "applyAsNewStr": "|08A|07p|15ply |08F|07o|15r |08A|07c|15cess? @",
    "connectionStr": "|08► |15Connect |08(|03|BR|08)",
    "defPrompt": "|08██ |15|MN |08██ |13|TL |05Left|08: ",
    "pauseString": "|15█|07█|08█|B1|09► |15Stroke Me! |09►|B0|08█|07█|15█",
    ...
}
```

Keys prefixed with `_` (e.g., `_extra3`) are reserved placeholders and cannot be edited.

When saving, the editor writes keys in sorted order and omits internal `_`-prefixed keys, producing clean deterministic output.

## String Descriptions

Each string entry has a built-in description explaining its purpose. These descriptions are compiled into the editor binary in `internal/stringeditor/metadata.go` and are displayed on row 24 as you navigate. They match the original Vision/2 `things[].descrip` help text.

## String Categories

The editor contains approximately 250 string entries organized by function:

| Range | Category |
|-------|----------|
| 1–20 | Core prompts (login, pause, chat, quoting) |
| 21–40 | Message base prompts (posting, boards, scanning) |
| 41–60 | User system (registration, passwords, phone) |
| 61–80 | Mail and feedback prompts |
| 81–100 | New user voting, rumors, BBS list |
| 101–120 | File system prompts (upload, download, batch) |
| 121–140 | File operations (ratios, listings, newscan) |
| 141–160 | QWK mail, chat, announcements |
| 161–178 | Advanced file/message operations |
| 179+ | ViSiON/3 additions (SSH, ANSI, conference, etc.) |

## Building

The string editor is built automatically by `build.sh`:

```bash
./build.sh    # Builds all binaries including ./strings
```

Or build it standalone:

```bash
go build -o strings ./cmd/strings
```

## Origin

This tool is a faithful recreation of the Vision/2 BBS `STRINGS.EXE` (Turbo Pascal, ~1400 lines in `SRC/STRINGS.PAS`). The Go version preserves the original's 20-item paginated layout, DOS color scheme, and editing workflow while adding search functionality and BubbleTea-based modern terminal rendering.

## Developer Reference

### How Defaults Are Loaded

The editor supports two layers of value restoration:

1. **Last-Saved Values (F3 "Revert")** — When the editor starts, it snapshots all values from `configs/strings.json` into `origValues`. F3 restores the currently selected string to the value it had when the editor was opened (i.e., whatever is on disk). This snapshot lives in memory only and is set in `stringeditor.New()`.

2. **ViSiON/3 Defaults (F4 "Restore")** — A separate set of shipped default values loaded from `templates/configs/strings.json`. This file represents the canonical out-of-the-box string values that ship with ViSiON/3. F4 restores the selected string to this shipped value.

### Default Loading Flow

```text
cmd/strings/main.go
  ├─ loadShippedDefaults()
       ├─ Try:  ./templates/configs/strings.json  (relative to CWD)
       ├─ Try:  <exe-dir>/templates/configs/strings.json  (relative to binary)
       └─ Returns: map[string]string (or nil if not found)

  └─ stringeditor.New(configPath, shippedDefaults)
       ├─ loads configs/strings.json into values
       ├─ copies values → origValues  (F3 revert source)
       └─ stores shippedDefaults      (F4 restore source)
```

- `loadShippedDefaults()` in `cmd/strings/main.go` attempts to read the template file from two locations: first relative to the working directory, then relative to the executable path. This allows the editor to work both during development (`cd /opt/vision3 && ./strings`) and from an installed location.
- If the template file is missing or unparseable, `shippedDefaults` is `nil` and F4 will display "No shipped defaults available".
- The template file (`templates/configs/strings.json`) must be kept in sync with `configs/strings.json` when new string keys are added to the system.

### Adding a New String

1. Add a `StringEntry` struct to the appropriate position in `internal/stringeditor/metadata.go` (Label, Key, Description)
2. Add the key with its default value to `configs/strings.json`
3. Add the same key and value to `templates/configs/strings.json` (the ViSiON/3 defaults template)
4. The editor will pick up the new entry automatically on next run
