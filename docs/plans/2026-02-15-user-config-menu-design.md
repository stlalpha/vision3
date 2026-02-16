# VIS-52: User Config Menu

## Overview

User-facing configuration menu allowing users to manage their terminal preferences, personal info, colors, password, and other settings. Full parity with ViSiON/2's user configuration system, adapted for V3.

## Architecture

Standard V3 menu pattern: ANSI art screen + CFG hotkeys + RUN: command handlers. Accessed from main menu via K key (replacing CONFIGLM with USERCFG). Implementation split into a dedicated `internal/menu/user_config.go` file to keep executor.go manageable.

## Menu Options

| Key | Setting | Type | V2 Equivalent |
|-----|---------|------|---------------|
| A | Screen Width | Numeric input (40-255) | `eightycols` |
| B | Screen Height / Page Length | Numeric input (21-60) | `DisplayLen` |
| C | Terminal Type | Toggle ANSI/CP437 | `ansigraphics/avatar/vt52` |
| D | Full-Screen Editor | Toggle on/off | `fseditor` |
| E | Hot Keys | Toggle on/off | `hotkeys` |
| F | Pause/More Prompts | Toggle on/off | `moreprompts` |
| G | Message Header Style | Existing GETHEADERTYPE | header selection |
| H | Custom Prompt | String input | `Prompt` format codes |
| I | Prompt Color | Color picker (0-15) | `Color1` |
| J | Input Color | Color picker (0-15) | `Color2` |
| K | Text Color | Color picker (0-15) | `Color3` |
| L | Stat Color | Color picker (0-15) | `Color4` |
| M | Real Name | String input | `RealName` |
| N | Phone Number | String input | `PhoneNumber` |
| O | User Note | String input | `SysOpNote` |
| P | Change Password | Old/new/confirm | `Password` |
| V | View Config | Template display | `View` |
| Q | Return to Main | GOTO:MAIN | — |

## Data Model

New fields added to `User` struct in `internal/user/user.go`:

- `HotKeys bool` — single keypress mode
- `MorePrompts bool` — pause at screen end
- `FullScreenEditor bool` — full-screen editor for messages
- `CustomPrompt string` — custom prompt format string with V2 codes
- `OutputMode string` — "ansi" or "cp437"
- `Colors [7]int` — Prompt, Input, Text, Stat, Text2, Stat2, Bar (ANSI 0-15)

Persisted via existing `userManager.UpdateUser()` flow.

## Implementation Details

### Toggle Commands

Flip the bool field, save via UpdateUser, display confirmation with new state.

### Numeric Inputs

Prompt for value, validate bounds (width: 40-255, height: 21-60), save.

### String Inputs

Prompt for value, validate max length, save. Real Name (40 chars), Phone (15 chars), User Note (35 chars), Custom Prompt (80 chars).

### Password Change

1. Prompt for current password
2. Verify against bcrypt hash
3. Prompt for new password
4. Prompt to confirm new password
5. Hash with bcrypt, save

### Color Picker

Display 16 ANSI colors numbered 0-15 inline. User types number to select. Same prompt reused for all 7 color slots.

### Custom Prompt Format Codes

Stored raw, rendered at display time. Supported codes: `|MN` (menu name), `|TL` (time left), `|TN` (time), `|DN` (date), `|CR` (carriage return), plus pipe color codes.

### View Config

Template-based (USRCFGV.TOP/BOT) with `@` token substitution showing all current settings.

## Files

| File | Action |
|---|---|
| `internal/user/user.go` | Modify — add preference fields |
| `internal/menu/user_config.go` | Create — all config command handlers |
| `internal/menu/executor.go` | Modify — register new RUN: commands |
| `menus/v3/cfg/USERCFG.CFG` | Create — menu hotkey config |
| `menus/v3/mnu/USERCFG.MNU` | Create — menu display definition |
| `menus/v3/ansi/USERCFG.ANS` | Create — ANSI art screen |
| `menus/v3/templates/USRCFGV.TOP` | Create — view config header |
| `menus/v3/templates/USRCFGV.BOT` | Create — view config footer |
| `menus/v3/cfg/MAIN.CFG` | Modify — K key → GOTO:USERCFG |
