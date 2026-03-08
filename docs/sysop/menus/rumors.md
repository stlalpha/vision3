# Rumors

The Rumors feature is a community graffiti wall where users can post short anonymous or signed messages — a modernization of ViSiON/2's `RUMORS.PAS`. Users can browse, add, delete, and search rumors, with access-level filtering and newscan support.

---

## How It Works

Rumors are stored in `data/rumors.json`. Each entry has:

| Field | Description |
|-------|-------------|
| **Text** | The rumor message text |
| **Author** | Displayed author name (or anonymous name) |
| **RealUser** | Actual username (always stored, even for anonymous posts) |
| **MinLevel** | Minimum security level required to view the rumor (1–255) |
| **PostedAt** | Timestamp when the rumor was posted |

A maximum of 999 rumors can be stored (matching V2's limit).

---

## User Interface

### Listing Rumors (`RUN:RUMORSLIST`)

Clears the screen and displays all visible rumors in a columnar list:

```text
#   Rumor                                     Author          Date
──────────────────────────────────────────────────────────────────────────
1   The sysop is hiding something...          Anonymous       03/08/26
2   Best BBS ever!                            J0hnny A1pha    03/08/26
```

Rumors below the user's access level are hidden. SysOps see the real author in parentheses after anonymous names.

**Default menu binding:** `L` on the Rumors Menu.

---

### Adding a Rumor (`RUN:RUMORSADD`)

Prompts the user through:

1. **Anonymous?** — Only shown if the user's access level meets the configured `AnonymousLevel` threshold. Yes/No lightbar prompt.
2. **Minimum Level** — Security level required to view this rumor (1–255). Validates input and re-prompts on invalid entries. Defaults to 1 if left blank.
3. **Enter Rumor** — The rumor text. Empty input aborts.

After saving, displays a confirmation message for 1 second.

Requires access level 2 or higher to post.

**Default menu binding:** `A` on the Rumors Menu.

---

### Deleting a Rumor (`RUN:RUMORSDELETE`)

Prompts for a rumor number (type `?` to see a quick list), shows the rumor text, then confirms with Yes/No.

**Ownership:** Users can only delete their own rumors. SysOps (level 255) can delete any rumor.

**Default menu binding:** `D` on the Rumors Menu.

---

### Searching Rumors (`RUN:RUMORSSEARCH`)

Prompts for search text, then displays all matching rumors. Searches across:

- Rumor text
- Author name

Results show the rumor number, text, and author. Case-insensitive matching.

**Default menu binding:** `S` on the Rumors Menu.

---

### Rumors Newscan (`RUN:RUMORSNEWSCAN`)

Clears the screen and displays all rumors posted since the user's last login. Shows rumor number, text, and author for each new rumor.

**Default menu binding:** `N` on the Rumors Menu.

---

### Random Rumor (`RUN:RANDOMRUMOR`)

Displays a single random rumor centered on screen, enclosed in brackets:

```text
                        [ Best BBS ever! ]
```

This respects access level filtering. It can be used:

- From the Rumors Menu via the `*` key
- In the **login sequence** by adding to `configs/config.json`:

```json
{
    "loginSequence": [
        {"command": "LASTCALLS"},
        {"command": "RANDOMRUMOR"},
        {"command": "ONELINERS"}
    ]
}
```

---

## MCI Code: `@RR@`

A random rumor can be embedded in any ANSI file or menu prompt using the `@RR@` AT-code. This picks a random visible rumor (respecting the user's access level) and substitutes the text.

### Supported Formats

| Format | Result |
|--------|--------|
| `@RR@` | Raw rumor text, no padding |
| `@RR:60@` | Left-aligned in 60-character field |
| `@RR\|R:60@` | Right-aligned in 60-character field |
| `@RR\|C:60@` | Centered in 60-character field |
| `@RR\|C##########@` | Centered, field width equals placeholder length |
| `@RR\|R8@` | Right-aligned in 8-character field |

### Example Usage in ANSI Art

Place `@RR|C:70@` in any `.ANS` file to display a centered random rumor in a 70-character field. This works in menu screens, prompts (`PROMPT1`/`PROMPT2`), and file includes (`%%file.ans%%`).

---

## Menu Configuration

### RUMORM Menu

The Rumors feature has its own submenu (`RUMORM`), accessed from the Main Menu via the `R` key.

**Main Menu entry** (`menus/v3/cfg/MAIN.CFG`):

```json
{
    "KEYS": "R",
    "CMD": "GOTO:RUMORM",
    "ACS": "*",
    "HIDDEN": false,
    "NODE_ACTIVITY": "Reading Rumors"
}
```

**Rumors Menu definition** (`menus/v3/mnu/RUMORM.MNU`):

```json
{
    "TITLE": "Rumors",
    "CLR": true,
    "USEPROMPT": true,
    "PROMPT1": "Rumors",
    "PROMPT2": "[Rumors] Command -> ",
    "FALLBACK": "",
    "ACS": "*"
}
```

**Rumors Menu commands** (`menus/v3/cfg/RUMORM.CFG`):

| Key | Command | ACS | Description |
|-----|---------|-----|-------------|
| `L` | `RUN:RUMORSLIST` | `*` | List all rumors |
| `A` | `RUN:RUMORSADD` | `*` | Add a new rumor |
| `N` | `RUN:RUMORSNEWSCAN` | `*` | Newscan (new since last login) |
| `S` | `RUN:RUMORSSEARCH` | `*` | Search rumors |
| `D` | `RUN:RUMORSDELETE` | `*` | Delete a rumor |
| `*` | `RUN:RANDOMRUMOR` | `*` | Display a random rumor |
| `Q` | `GOTO:MAIN` | `*` | Return to main menu |

### ANSI Screen

The ANSI art screen `menus/v3/ansi/RUMORM.ANS` is displayed when entering the Rumors Menu. Edit this file with any ANSI art editor to customize the appearance.

---

## Customizable Strings

The following prompts can be customized in `configs/strings.json`:

| Key | Default | Description |
|-----|---------|-------------|
| `addRumorAnonymous` | `Anonymous? @` | Anonymous posting prompt |
| `enterRumorLevel` | `Level :` | Minimum level prompt |
| `enterRumorPrompt` | `Enter Rumor (Enter/Abort):` | Rumor text entry prompt |
| `rumorAdded` | `Rumor has been added!` | Confirmation message |
| `anonymousName` | `Anonymous` | Display name for anonymous posts |

---

## Data File

Rumors are stored in `data/rumors.json`:

```json
{
    "rumors": [
        {
            "id": 1,
            "author": "J0hnny A1pha",
            "real_user": "j0hnny a1pha",
            "text": "This is a rumor!",
            "posted_at": "2026-03-08T05:24:41.950603Z",
            "min_level": 1
        }
    ],
    "next_id": 2
}
```

The file is created automatically when the first rumor is added. The `next_id` field auto-increments to ensure unique IDs. Concurrent access is mutex-protected.

---

## ViSiON/2 Compatibility

| V2 Feature | V3 Equivalent |
|------------|---------------|
| `ListRumors` (R/S/B modes) | `RUN:RUMORSLIST` (single list view) |
| `AddRumor` | `RUN:RUMORSADD` |
| `DeleteRumor` | `RUN:RUMORSDELETE` |
| `SearchForText` | `RUN:RUMORSSEARCH` |
| `RumorsNewscan` | `RUN:RUMORSNEWSCAN` |
| `RandomRumor` (MAINR2.PAS) | `RUN:RANDOMRUMOR` + `@RR@` MCI code |
| `RumorRec.Title` | Removed (simplified to text-only) |
| `RumorRec.Author` / `Author2` | `author` / `real_user` |
| `Cfg.RumChar[1..2]` (bracket chars) | Hardcoded `[ ]` brackets in random display |
| `ShowRumors` user config flag | Login sequence configuration |
| `RUMORS.DAT` (binary records) | `data/rumors.json` |

---

## See Also

- [Menu System](menu-system.md) — menu and command file configuration
- [Access Control (ACS)](menu-system.md#access-control-strings-acs) — restricting commands
- [News](news.md) — similar SysOp-managed content system
- [BBS Listings](bbs-list.md) — similar user-contributed content system
