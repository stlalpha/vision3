# ViSiON/3 Sponsor Menus

Sponsor Menus give a designated user (the **sponsor**) elevated moderation access to a specific message area. This follows the Vision/2 tradition of per-area sponsors/moderators, extended in Vision/3 with an in-BBS area editor accessible via the `%` key.

## Overview

Each message area in `configs/message_areas.json` can have a `"sponsor"` field containing the handle of its sponsor. A sponsor can enter the Sponsor Menu for their area, where they can edit area settings directly from the BBS without sysop intervention.

Sysops and co-sysops automatically have sponsor access to all areas.

## Access Levels

The Sponsor Menu (`%` in the Messages Menu) is accessible to:

| Who | Condition |
|-----|-----------|
| SysOp | `accessLevel >= sysOpLevel` (default 255) |
| Co-SysOp | `accessLevel >= coSysOpLevel` (default 250) |
| Area Sponsor | `handle` matches the area's `"sponsor"` field (case-insensitive) |

All other users are silently refused — the `%` key does nothing for them, and no error is displayed.

## Configuring a Sponsor

Set the `"sponsor"` field in `configs/message_areas.json` to the user's handle:

```json
{
  "id": 3,
  "tag": "TECH",
  "name": "Tech Talk",
  "description": "Technology discussion",
  "acs_read": "S10",
  "acs_write": "S10",
  "base_path": "msgbases/tech",
  "area_type": "local",
  "sponsor": "TechGuru"
}
```

The handle comparison is **case-insensitive** — `"techguru"`, `"TechGuru"`, and `"TECHGURU"` all match a user whose handle is `TechGuru`.

The sponsor can also be set from within the BBS via the Edit Area screen (see below).

## Using the Sponsor Menu

From the **Messages Menu**, press `%` to enter the Sponsor Menu for the currently selected message area.

The `SPONSORM.ANS` header is displayed, followed by a prompt:

```text
[TECH] Sponsor: E=Edit Area  [/]=Prev/Next Area  Q=Quit:
```

### Sponsor Menu Keys

| Key | Action |
|-----|--------|
| `E` | Edit the current area's settings |
| `Q` | Return to the Messages Menu |

## Edit Area Screen

Press `E` from the Sponsor Menu to open the area editor. The current field values are displayed:

```text
Edit Area: TECH (ID 3)
────────────────────────────────────────────────────
T) Tag           : TECH
N) Name          : Tech Talk
D) Description   : Technology discussion
R) ACS Read      : S10
W) ACS Write     : S10
S) Sponsor       : TechGuru
M) Max Messages  : 0
G) Max Age (days): 0
A) Allow Anon    : default
L) Real Name Only: false
J) Auto Join     : false
C) Conference ID : 0
B) Base Path     : msgbases/tech
Y) Area Type     : local
E) Echo Tag      :
O) Origin Addr   :
K) Network       :
────────────────────────────────────────────────────
Edit (T N D R W S M G A L J C B Y E O K)  Q=Save/Quit  ESC=Cancel:
```

### Field Reference

| Key | Field | Max Length | Access | Notes |
|-----|-------|-----------|--------|-------|
| `T` | Tag | 32 chars | Co-SysOp+ | Area tag used in routing; changing may break FTN echomail references |
| `N` | Name | 60 chars | Sponsor | Display name shown in area lists |
| `D` | Description | 80 chars | Sponsor | Longer description |
| `R` | ACS Read | 40 chars | Sponsor | ACS string required to read (empty = public) |
| `W` | ACS Write | 40 chars | Sponsor | ACS string required to post (empty = all) |
| `S` | Sponsor | 30 chars | Sponsor | Handle of area sponsor; enter `-` to clear; validated against user database |
| `M` | Max Messages | integer | Sponsor | Maximum messages to retain (0 = unlimited) |
| `G` | Max Age (days) | integer | Sponsor | Purge messages older than N days (0 = unlimited) |
| `A` | Allow Anon | yes/no/default | Sponsor | Allow anonymous posts; `default` inherits system setting |
| `L` | Real Name Only | yes/no | Sponsor | Require users to post under their real name |
| `J` | Auto Join | yes/no | Sponsor | Automatically add new users to this area |
| `C` | Conference ID | integer | Sponsor | Groups area under a conference (0 = ungrouped) |
| `B` | Base Path | 80 chars | Co-SysOp+ | Filesystem path for message storage |
| `Y` | Area Type | 16 chars | Co-SysOp+ | `local`, `echomail`, or `netmail` |
| `E` | Echo Tag | 32 chars | Co-SysOp+ | FTN echomail tag for routing |
| `O` | Origin Address | 32 chars | Co-SysOp+ | FTN origin address |
| `K` | Network | 32 chars | Co-SysOp+ | FTN network name |

### Editing a Field

Press the field's letter. The current value is shown in brackets:

```text
Name [Tech Talk]:
```

- Type a new value and press **Enter** to apply it.
- Press **Enter** with no input to keep the current value.
- Input is truncated to the field's maximum length automatically.

### Sponsor Field Validation

When editing the Sponsor (`S`) field, the entered handle is validated against the user database. If no user with that handle exists, the change is rejected:

```text
User 'Nobody' not found — sponsor unchanged.
```

To remove the sponsor (leave the area with no designated sponsor), enter a single dash (`-`).

### Saving and Cancelling

| Key | Action |
|-----|--------|
| `Q` | Save all changes to `configs/message_areas.json` and return |
| `ESC` | Discard all changes and return |

Changes are written atomically using a temporary file rename, so a crash or disconnect during save cannot corrupt `message_areas.json`.

## ANSI File

The Sponsor Menu header is loaded from:

```text
menus/v3/ansi/SPONSORM.ANS
```

If the file cannot be displayed (missing or read error), the menu continues without the header — a text prompt is always shown.

## Menu Configuration

The `%` key is wired in `menus/v3/cfg/MSGMENU.CFG`:

```json
{
  "KEYS": "%",
  "CMD": "RUN:SPONSORMENU",
  "ACS": "*",
  "HIDDEN": true,
  "NODE_ACTIVITY": "Sponsor Menu"
}
```

The entry is `"HIDDEN": true` so it does not appear in the Messages Menu listing. It is accessible only to those who know the `%` shortcut (sysops can communicate this to their sponsors).

The sponsor sub-menu is defined in `menus/v3/cfg/SPONSORM.CFG`:

```json
[
  {
    "KEYS": "E",
    "CMD": "RUN:SPONSOREDITAREA",
    "ACS": "*",
    "HIDDEN": false,
    "NODE_ACTIVITY": "Editing Message Area"
  },
  {
    "KEYS": "Q",
    "CMD": "QUIT",
    "ACS": "*",
    "HIDDEN": false
  }
]
```

The `[` (previous area) and `]` (next area) navigation keys are handled internally by `runSponsorMenu` and do not need CFG entries. They are always active within the Sponsor Menu regardless of `SPONSORM.CFG`.

## Implementation Notes

### Access Check

`CanAccessSponsorMenu(user, area, serverConfig)` in `internal/menu/sponsor_access.go` performs the three-way check. It is called by both `runSponsorMenu` and `runSponsorEditArea` so neither handler can be reached without passing the gate.

### Persistence

`MessageManager.SaveAreas()` (`internal/message/manager.go`) serialises all areas to a temp file in the same directory as `message_areas.json`, then renames it into place. This is the same atomic-write pattern used elsewhere in Vision/3 for JSON data files.

### Vision/2 Lineage

In Vision/2, the `BoardRec.Sponsor` (message areas) and `AreaRec.Sponsor` (file areas) fields stored a sponsor username. The `sponsoron()` function checked `match(curboard.sponsor, unam) OR tempsysop`. Vision/3's `CanAccessSponsorMenu` mirrors this logic, using `strings.EqualFold` for the case-insensitive handle match and `accessLevel` thresholds for sysop/co-sysop.

Vision/2 had no dedicated sponsor menu — sponsor status conferred only the ability to delete and edit others' messages, plus exemption from upload/download ratios. The Sponsor Menu with Edit Area is a Vision/3 enhancement.
