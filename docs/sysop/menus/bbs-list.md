# BBS Listings

The BBS Listings feature is a user-contributed directory of other BBS systems — a modernization of ViSiON/2's `BBSLIST.PAS`. Users can add, browse, and manage entries for BBSes they know about, creating a community-curated directory.

---

## How It Works

BBS listings are stored in `data/bbslist.json`. Each entry has:

| Field | Description |
|-------|-------------|
| **Name** | BBS name (up to 40 characters) |
| **SysOp** | SysOp name (up to 30 characters) |
| **Address** | Hostname or IP address (up to 60 characters) |
| **Telnet Port** | Telnet port number (blank if none) |
| **SSH Port** | SSH port number (blank if none) |
| **Web** | Web URL (blank if none) |
| **Software** | BBS software name (defaults to "ViSiON/3") |
| **Description** | One-line description (up to 200 characters) |
| **Added By** | Handle of the user who created the entry |
| **Verified** | SysOp-verified flag (toggled by CoSysOp+) |

---

## User Interface

### Split-Panel Lightbar (`RUN:BBSLIST`)

The listing display uses a split-panel lightbar interface:

- **Left panel** — scrollable list of BBS names with a highlight bar
- **Right panel** — full details for the currently selected entry (address, ports, SysOp, software, description, etc.)

The list height adapts to the user's terminal height. Navigation:

| Key | Action |
|-----|--------|
| Up/Down | Move selection |
| PgUp/PgDn | Page through list |
| Home/End | Jump to first/last entry |
| Q / Esc | Return to menu |

Verified entries show a `*` marker next to their name.

**Default menu binding:** `L` on the BBS List Menu.

---

### Adding an Entry (`RUN:BBSLISTADD`)

Prompts the user for each field in sequence:

1. **BBS Name** (required)
2. **Address** (hostname or IP address, required)
3. **Telnet Port** (optional)
4. **SSH Port** (optional)
5. **Web URL** (optional)
6. **SysOp** (optional)
7. **Software** (defaults to "ViSiON/3" if blank)
8. **Description** (optional)

At least one connection method (Telnet, SSH, or Web) should be provided, though this is not strictly enforced — the address field itself is required.

**Default menu binding:** `A` on the BBS List Menu.

---

### Editing an Entry (`RUN:BBSLISTEDIT`)

Prompts for an entry number (type `?` to see a quick list), then displays a numbered field editor:

| # | Field |
|---|-------|
| 1 | Name |
| 2 | Address |
| 3 | Telnet Port |
| 4 | SSH Port |
| 5 | Web |
| 6 | SysOp |
| 7 | Software |
| 8 | Description |

Enter a field number to change it, or `Q` to save and quit.

**Ownership:** Users can only edit their own entries. CoSysOp and above can edit any entry (matches V2's `Match(B.LeftBy,Unam)` + `IsSysop` check).

**Default menu binding:** `C` on the BBS List Menu.

---

### Deleting an Entry (`RUN:BBSLISTDELETE`)

Prompts for an entry number (type `?` to see a quick list), confirms with Yes/No, then removes the entry. Remaining entries are compacted (same behavior as V2's record shift-down).

**Ownership:** Same rules as editing — own entries only, unless CoSysOp+.

**Default menu binding:** `D` on the BBS List Menu.

---

## SysOp Commands

### Verify Entry (`RUN:BBSLISTVERIFY`)

Toggles the **Verified** flag on a listing. Requires CoSysOp access or above (`isCoSysOpOrAbove`). Verified entries display a `*` marker in the lightbar list view.

**Default menu binding:** `V` on the BBS List Menu (ACS: `S255`).

---

## Menu Configuration

### BBSLISTM Menu

The BBS List has its own submenu (`BBSLISTM`), accessed from the Main Menu via the `B` key.

**Main Menu entry** (`menus/v3/cfg/MAIN.CFG`):

```json
{
    "KEYS": "B",
    "CMD": "GOTO:BBSLISTM",
    "ACS": "*",
    "HIDDEN": false,
    "NODE_ACTIVITY": "Browsing BBS List"
}
```

**BBS List Menu definition** (`menus/v3/mnu/BBSLISTM.MNU`):

```json
{
    "TITLE": "BBS Listings",
    "CLR": true,
    "USEPROMPT": true,
    "PROMPT1": "BBS Listings Directory",
    "PROMPT2": "[BBS List] Command -> ",
    "FALLBACK": "MAIN",
    "ACS": "*"
}
```

**BBS List Menu commands** (`menus/v3/cfg/BBSLISTM.CFG`):

| Key | Command | ACS | Description |
|-----|---------|-----|-------------|
| `L` | `RUN:BBSLIST` | `*` | List/browse entries (lightbar) |
| `A` | `RUN:BBSLISTADD` | `*` | Add new entry |
| `C` | `RUN:BBSLISTEDIT` | `*` | Edit entry (own or sysop) |
| `D` | `RUN:BBSLISTDELETE` | `*` | Delete entry (own or sysop) |
| `V` | `RUN:BBSLISTVERIFY` | `S255` | Toggle verified flag |
| `Q` | `GOTO:MAIN` | `*` | Return to main menu |

### ANSI Screen

The ANSI art screen `menus/v3/ansi/BBSLISTM.ANS` is displayed when entering the BBS List Menu. It shows the available key commands (L/A/C/D/V/Q). Edit this file with any ANSI art editor to customize the appearance.

---

## Data File

BBS listings are stored in `data/bbslist.json`:

```json
{
    "listings": [
        {
            "id": 1,
            "name": "Zombie Toolshed",
            "sysop": "J0hnny A1pha",
            "address": "zombietoolshed.us",
            "telnet_port": "2323",
            "ssh_port": "2222",
            "web": "zombietoolshed.us",
            "software": "ViSiON/3",
            "description": "This is a description",
            "added_by": "J0hnny A1pha",
            "added_date": "2026-03-08T04:04:32.443918764Z",
            "verified": true
        }
    ],
    "next_id": 2
}
```

The file is created automatically when the first entry is added. The `next_id` field auto-increments to ensure unique IDs.

---

## ViSiON/2 Compatibility

| V2 Feature | V3 Equivalent |
|-------------|---------------|
| `ListBBS` + `ViewAnsi` | `RUN:BBSLIST` (split-panel lightbar) |
| `AddBBS` | `RUN:BBSLISTADD` |
| `ChangeBBS` | `RUN:BBSLISTEDIT` |
| `Deletebbs` | `RUN:BBSLISTDELETE` |
| `BBSRec.Phone` | `address` + `telnet_port` / `ssh_port` |
| `BBSRec.Baud` | Removed (irrelevant for modern BBS) |
| `BBSRec.Extended` (sector pointer) | `description` (inline text) |
| `BBSLIST.DAT` (binary records) | `data/bbslist.json` |
| `BBSANSI.TXT/MAP` (text storage) | Not needed (description stored inline) |
| Ownership check (`Match + IsSysop`) | `strings.EqualFold(AddedBy, Handle)` + `isCoSysOpOrAbove` |

---

## See Also

- [Menu System](menu-system.md) — menu and command file configuration
- [Access Control (ACS)](menu-system.md#access-control-strings-acs) — restricting commands
- [News](news.md) — similar SysOp-managed content system
