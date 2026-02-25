# Admin Menu

The Admin Menu provides in-BBS access to user management functions without leaving the BBS or editing JSON files directly. It is accessible only to SysOps (access level ≥ `sysOpLevel`).

## Accessing the Admin Menu

From the **Main Menu**, press `X`. You are taken to the Admin Menu (`ADMIN.MNU`), which presents a prompt:

```text
[hh:mm] Admin Menu -> _
```

> **Access control:** The Admin Menu requires `S255` ACS — only the primary SysOp account (user #1 or any user at `sysOpLevel`) can enter it.

---

## Menu Commands

| Key | Function | Description |
|-----|----------|-------------|
| `V` | Validate Users | Browse unvalidated accounts and approve them |
| `E` | Edit Users | Full user browser — view and edit any account |
| `N` | Toggle New Users | Open or close new user registrations |
| `P` | Purge Deleted Users | Permanently remove soft-deleted accounts past retention period |
| `Q` | Quit | Return to Main Menu |

---

## Pending Validation Notice

When you log in to the Main Menu and there are users awaiting validation, ViSiON/3 automatically displays a notice:

```text
Admin: [V] Validate user account [N]. Press X for Admin menu.
```

Where `N` is the count of pending unvalidated users. This fires automatically; no menu configuration is needed.

---

## V — Validate Users

Opens a lightbar browser showing only **unvalidated** user accounts. The lower half of the screen shows details for the highlighted user. Changes are staged and must be explicitly saved.

### Navigation

| Key | Action |
|-----|--------|
| Up / Down / K / J | Move cursor |
| W | Move up (alternate) |

### Actions

| Key | Action |
|-----|--------|
| `H` | Toggle validated status (Validate ↔ Un-Validate) |
| `P` | Set new password (bcrypt-hashed before saving) |
| `0` | Toggle ban status — ban sets level 0 + unvalidated; un-ban restores regular level + validated |
| `9` | Toggle soft delete — delete sets `deletedUser=true`; un-delete restores the account |
| `A` | Edit username |
| `B` | Edit real name |
| `C` | Edit phone number |
| `D` | Edit group/location |
| `E` | Edit private note (sysop-only memo) |
| `F` | Edit access flags |
| `G` | Edit access level (numeric) |
| `S` | **Save** all staged changes |
| `X` | **Abort** — discard all staged changes |
| `Q` / Esc | Quit |

Changes to individual fields are **staged** (shown with a `*` prefix). Nothing is written to disk until you press `S`. Pressing `Q` with unsaved changes will warn rather than exit.

> **Validation behaviour:** When `H` is used to validate a user whose access level is currently 0, the level is automatically raised to `regularUserLevel` (configured in `config.json`, default 10).
>
> **User #1 protection:** The primary SysOp account (user ID 1) cannot be unvalidated, banned, or deleted through any admin screen.

---

## E — Edit Users

Opens the same lightbar browser as `V`, but shows **all users** (not just unvalidated ones). Useful for editing any account field, resetting passwords, checking stats, or managing deleted accounts.

The layout, key bindings, and staging behaviour are identical to the Validate Users screen above. The only difference is the user list shown: all accounts vs. pending-only.

### Detail Panel Fields

The lower half of the screen shows the following for the highlighted user:

| Key | Editable Field | Read-Only Stats |
|-----|---------------|----------------|
| `A` | Username | Total calls |
| `B` | Real name | Uploads |
| `C` | Phone number | File points |
| `D` | Group/Location | Messages posted |
| `E` | Private note | Created date |
| `F` | Access flags | Last login |
| `G` | Access level | Deleted status |
| `H` | Validated (toggle) | Deleted date |

### Online Indicator

A `*` in the list next to a user's entry means that user is **currently connected** to the BBS. Changes can still be made to online users; they take effect immediately.

---

## N — Toggle New User Registrations

Immediately toggles the `allowNewUsers` flag in `configs/config.json` and writes the change to disk. The new state is reported:

```text
New user registrations: OPEN
```

or

```text
New user registrations: CLOSED
```

When closed, the "new" keyword at the login prompt will be rejected and new user applications will not be accepted. This takes effect immediately without a BBS restart.

---

## P — Purge Deleted Users

Permanently removes soft-deleted user accounts that have been deleted for longer than `deletedUserRetentionDays` (set in `config.json`).

The screen shows a list of eligible accounts with their deletion date, then prompts for confirmation before deleting. This action **cannot be undone** — the records are permanently removed from `users.json`.

If `deletedUserRetentionDays` is set to `-1`, purge is disabled and `P` will report that.

See [User Management — Soft Delete and Purge](user-management.md#soft-deleting-users) for retention period configuration.

---

## Admin Activity Logging

All changes made through the admin screens (both `V` and `E`) are logged to `data/users/admin_activity.log`. Each entry records:

- Which admin made the change
- Which user was changed
- Which field was modified
- The old and new values
- Timestamp

Password changes are logged as `********` — the actual values are never written to the log.

---

## See Also

- [User Management](user-management.md) — accounts, access levels, soft delete, and purge configuration
- [User Editor (`ue`)](user-editor.md) — offline TUI for bulk operations and detailed field editing
