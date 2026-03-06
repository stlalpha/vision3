# New User Voting (NUV)

New User Voting is ViSiON/3's implementation of ViSiON/2's NUV system. When a new user registers, their handle is placed in a pending queue. Existing users who meet a configurable minimum access level can then cast YES or NO votes on each candidate. When vote totals reach configured thresholds, the BBS can automatically validate or delete the account — or simply notify the SysOp to act manually.

> This is a community-driven vetting system, not a replacement for SysOp validation. Both can coexist: NUV handles the community vote; the SysOp retains final authority.

---

## How It Works

1. **New user registers** — if `autoAddNuv` is enabled, their handle is automatically placed in `data/nuv.json`.
2. **Eligible users are notified at login** (`CHECKNUV`) — if there are candidates they haven't voted on, a prompt appears.
3. **Users vote** via `SCANNUV` (or `LISTNUV` to review the queue first).
4. **Thresholds trigger** — when yes or no votes reach the configured limit, the BBS either acts automatically or logs a notice for the SysOp.

---

## Configuration

NUV is configured in `configs/config.json`:

```json
{
  "useNuv": true,
  "autoAddNuv": true,
  "nuvUseLevel": 25,
  "nuvYesVotes": 5,
  "nuvNoVotes": 5,
  "nuvValidate": true,
  "nuvKill": false,
  "nuvLevel": 25
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `useNuv` | `false` | Enable the NUV system. If `false`, all NUV commands are silent no-ops |
| `autoAddNuv` | `false` | Automatically queue new users after registration |
| `nuvUseLevel` | `25` | Minimum access level required to vote |
| `nuvYesVotes` | `5` | Number of YES votes needed to trigger the yes threshold |
| `nuvNoVotes` | `5` | Number of NO votes needed to trigger the no threshold |
| `nuvValidate` | `true` | If `true` and yes threshold is reached, automatically validate the user and set their level to `nuvLevel`. If `false`, log a notice for the SysOp to act manually |
| `nuvKill` | `false` | If `true` and no threshold is reached, automatically soft-delete the user. If `false`, log a notice for the SysOp to act manually |
| `nuvLevel` | `25` | Access level assigned when a user is auto-validated via NUV |

---

## Threshold Behaviour

### YES threshold reached (`nuvYesVotes`)

| `nuvValidate` | Result |
|---|---|
| `true` | User's access level set to `nuvLevel`, `validated = true`; candidate removed from queue |
| `false` | Log message only: `NUV: '<handle>' reached YES threshold — notify SysOp to validate`; candidate removed from queue |

### NO threshold reached (`nuvNoVotes`)

| `nuvKill` | Result |
|---|---|
| `true` | User's account soft-deleted (`deletedUser = true`); candidate removed from queue |
| `false` | Log message only: `NUV: '<handle>' reached NO threshold — notify SysOp to delete`; candidate removed from queue |

In both cases the candidate is removed from the queue once a threshold is reached — regardless of the auto-act setting.

---

## Menu Commands

### `RUN:CHECKNUV` — Login notification

Runs automatically in the login sequence. If the current user's access level is `>= nuvUseLevel` and there are candidates they haven't voted on, they see:

```
New User Voting: 3 candidate(s) awaiting your vote!
Vote now? [Y/N]:
```

Pressing `Y` immediately starts a scan of unvoted candidates. Pressing `N` continues the login sequence. If there are no unvoted candidates (or `useNuv` is disabled), this step is completely silent.

**Login sequence entry:**
```json
{"command": "CHECKNUV"}
```

---

### `RUN:SCANNUV` — Vote on pending candidates

Steps through all candidates the user hasn't yet voted on. For each:

```
New User Voting — Candidate #1
──────────────────────────────────────────────────
Voting for : JohnDoe
Yes votes  : 2
No votes   : 1
Added      : 01/15/2026

Voter Comments:
────────────────────────────────────────
SysOp               Yes Seems legit
OldTimer            No  No referral

[Y]es  [N]o  [C]omment  [R]eshow  [Q]uit:
```

| Key | Action |
|-----|--------|
| `Y` | Cast or change vote to YES |
| `N` | Cast or change vote to NO |
| `C` | Add or update a comment on your vote (must vote first) |
| `R` | Redisplay the candidate stats |
| `Q` / Esc | Quit scan (remaining candidates not shown) |

The scan stops after the user quits or after all unvoted candidates have been shown. Voted-on candidates are not shown again unless the user revisits via `SCANNUV`.

**Default menu binding:** `H` or `NUV` on the Main Menu.

---

### `RUN:LISTNUV` — View the full candidate queue

Displays a table of all pending candidates with their current vote tallies and whether the current user has voted on each:

```
New User Voting Queue — 3 Candidate(s)
────────────────────────────────────────────────────────────
#    Handle               Added        Yes   No Voted?
────────────────────────────────────────────────────────────
1    JohnDoe              01/15/26        2    1 Yes
2    CoolUser99           01/16/26        0    0 No
3    RetroFan             01/17/26        3    2 Yes
```

Read-only — no voting from this view. Use `SCANNUV` to vote.

**Default binding:** Admin Menu `U` key.

---

## SysOp Access

The Admin Menu includes `U` → `LISTNUV` for reviewing the queue at a glance. SysOps can also use `SCANNUV` from any menu to cast their own votes. All threshold-triggered actions (auto-validate, auto-delete) are logged to the application log.

To manually manage a user affected by NUV thresholds when auto-action is disabled, use the Admin Menu `V` (Validate) or `E` (Edit Users) screens.

---

## Data File

Candidates are stored in `data/nuv.json`:

```json
{
    "candidates": [
        {
            "handle": "JohnDoe",
            "when": "2026-01-15T18:30:00Z",
            "votes": [
                {
                    "voter": "SysOp",
                    "yes": true,
                    "comment": "Seems legit"
                },
                {
                    "voter": "OldTimer",
                    "yes": false,
                    "comment": "No referral"
                }
            ]
        }
    ]
}
```

The file is created automatically when the first candidate is added. It is safe to edit manually (e.g., to remove a stuck candidate or pre-populate votes).

---

## Manually Adding a Candidate

To add a user to the NUV queue without them re-registering (e.g., if `autoAddNuv` was off at the time), edit `data/nuv.json` and add an entry to the `candidates` array. The `votes` array can be empty.

---

## See Also

- [Login Sequence](login-sequence.md) — configuring `CHECKNUV` in `login.json`
- [Admin Menu](admin-menu.md) — `U` key for NUV queue review
- [User Management](user-management.md) — manual validation and soft-delete
- [Configuration](../configuration/configuration.md) — `useNuv`, `nuvYesVotes`, and related fields
