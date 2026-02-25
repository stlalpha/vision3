# ViSiON/3 User Management Guide

This guide covers user management in ViSiON/3, including user accounts, access levels, and administration.

## User Database

User data is stored in `data/users/users.json`. The system automatically creates this file with a default user on first run.

## User Structure

Users are stored as a JSON array. Each user account contains:

```json
{
  "id": 1,
  "username": "felonius",
  "passwordHash": "$2a$10$...",
  "handle": "Felonius",
  "accessLevel": 10,
  "flags": "",
  "lastLogin": "2025-05-01T15:13:20Z",
  "timesCalled": 220,
  "lastBulletinRead": "0001-01-01T00:00:00Z",
  "realName": "Felonius",
  "phoneNumber": "",
  "createdAt": "0001-01-01T00:00:00Z",
  "validated": true,
  "filePoints": 0,
  "numUploads": 0,
  "timeLimit": 60,
  "privateNote": "",
  "group_location": "FAiRLiGHT/PC",
  "current_message_area_id": 1,
  "current_message_area_tag": "GENERAL",
  "last_read_message_ids": {
    "1": "6ae43f1a-cc1c-447c-9b9c-3cdeebd3fbfb"
  },
  "current_file_area_id": 1,
  "current_file_area_tag": "GENERAL",
  "tagged_file_ids": [],
  "screen_width": 80,
  "screen_height": 24
}
```

### User Fields

#### Essential Fields

- `id` - Unique numeric identifier
- `username` - Login name (case-insensitive)
- `passwordHash` - Bcrypt hashed password
- `handle` - Display name/alias

#### Access 

- `accessLevel` - Numeric access level (0-255)
- `flags` - String of single-character flags (e.g., "ABC")
- `validated` - Whether user is validated

#### Statistics

- `timesCalled` - Login count
- `lastLogin` - Last login timestamp
- `lastBulletinRead` - Last time bulletins were read
- `filePoints` - File area points
- `numUploads` - Number of file uploads
- `timeLimit` - Time limit per call in minutes (0=unlimited)

#### Personal Information

- `realName` - User's real name
- `phoneNumber` - Contact number
- `createdAt` - Account creation timestamp
- `group_location` - Group/Location affiliation
- `privateNote` - SysOp note about user

#### Terminal Preferences

- `screen_width` - Preferred terminal width (0 = use detected PTY width)
- `screen_height` - Preferred terminal height (0 = use detected PTY height)

After authentication, the system applies these preferences: if a user's stored screen dimensions are smaller than the detected PTY size (or the PTY defaults to 80x25), the stored values cap the effective terminal dimensions. ANSI art is truncated to fit the effective height to prevent scrolling.

#### Soft Delete

- `deletedUser` - Boolean flag indicating the user is soft-deleted (default: false)
- `deletedAt` - Timestamp when the user was deleted (only set if deletedUser is true)

Soft-deleted users cannot log in and are hidden from public user listings, but all their data (messages, files, call history) is preserved and attributed to them. They remain visible in admin views.

#### System State

- `current_message_area_id` - Current message area ID
- `current_message_area_tag` - Current message area tag
- `last_read_message_ids` - Map of area ID to last read message UUID
- `current_file_area_id` - Current file area ID
- `current_file_area_tag` - Current file area tag
- `tagged_file_ids` - Array of file UUIDs marked for download

## Access Levels

Access levels range from 0-255, with common levels being:

- **0**: No access (banned)
- **1-9**: Unvalidated/new users
- **10-19**: Regular validated users
- **20-49**: Trusted users
- **50-99**: Co-SysOps
- **100-255**: SysOp level

## User Flags

Flags are single characters (A-Z) that grant specific permissions:

- Used in ACS strings with the `f` prefix
- Case-insensitive when checked
- Example: Flag 'D' might mean "can download files"
- Flag 'M' might mean "can access message bases"

## Managing Users

### Default User

The system creates a default user on first run:

- Username: `felonius`
- Password: `password`
- Access Level: 10
- Validated: true

**Important**: Change this password immediately!

### User Authentication

- Usernames are case-insensitive for login
- Passwords are hashed using bcrypt
- Login increments `timesCalled` and updates `lastLogin`

### Adding Users

Users are added through:

1. New user application (type "new" at login screen)
2. The `ue` TUI user editor (see [User Editor](user-editor.md))
3. Manual editing of `users.json` (not recommended)

When adding users manually, ensure:

- Unique `id` number
- Unique `username` (case-insensitive)
- Unique `handle` (case-insensitive)
- Valid bcrypt `passwordHash`

## New User Application

When a user types "new" at the login screen username field, the new user application begins. This flow is modeled after the original ViSiON/2 Pascal `NewUser()` procedure in `GETLOGIN.PAS`.

### Entry Point

The application is triggered from either:

- `handleLoginPrompt()` — coordinate-based LOGIN.ANS screen
- `runAuthenticate()` — fallback text-based login

Both detect "new" (case-insensitive) in the username field and call `handleNewUserApplication()`.

The application can also be invoked from a menu command via `RUN:NEWUSER`.

### Application Flow

1. **NEWUSER.ANS** — Displays the welcome/info screen (`menus/v3/ansi/NEWUSER.ANS`) if it exists
2. **Apply for Access?** — Yes/No lightbar prompt using `applyAsNewStr` from `strings.json`. If the user declines, returns to login.
3. **Handle/Alias** — Prompted with `newUserNameStr`. Validated against these rules:
   - Minimum 3 characters
   - No special characters: `?`, `#`, `/`, `*`, `&`, `:`
   - Cannot be reserved words: "new", "q", "sysop"
   - Cannot be purely numeric
   - Must be unique (checked against both username and handle)
   - Up to 5 attempts before rejection
4. **Password** — Prompted with `createAPassword`. Input is masked with `*` characters.
   - Minimum 3 characters
   - Must be confirmed (prompted with `reEnterPassword`)
   - Passwords must match; up to 5 attempts
5. **Real Name** — Prompted with `enterRealName`.
   - Minimum 4 characters
   - Must contain a space (first and last name)
   - Up to 5 attempts
6. **Phone Number** — Header displayed from `enterNumberHeader`, input prompted with `enterNumber`. Optional.
7. **Group/Location** — Prompted inline. Optional.
8. **User Note** — Prompted with `enterUserNote`. Stored in `privateNote` field. Optional.
9. **Account Creation** — Calls `UserMgr.AddUser()` which:
   - Assigns the next available user ID
   - Hashes the password with bcrypt
   - Sets `accessLevel` to 1 and `validated` to false
   - Sets `timeLimit` to 60 minutes
   - Saves to `data/users/users.json`
10. **User Number** — Displays the assigned ID using `yourUserNum`
11. **Welcome** — Displays `welcomeNewUser` message
12. **Validation Notice** — Informs the user that SysOp validation is required
13. **Return to Login** — User presses Enter and returns to the LOGIN screen

### Configurable Strings

All prompts are configurable in `configs/strings.json`:

| String Key          | Purpose                                                          |
| ------------------- | ---------------------------------------------------------------- |
| `applyAsNewStr`     | "Apply for Access?" prompt                                       |
| `newUserNameStr`    | Handle/alias entry prompt                                        |
| `createAPassword`   | Password creation prompt                                         |
| `reEnterPassword`   | Password confirmation prompt                                     |
| `enterRealName`     | Real name entry prompt                                           |
| `enterNumberHeader` | Phone number format hint                                         |
| `enterNumber`       | Phone number entry prompt                                        |
| `enterUserNote`     | User note entry prompt                                           |
| `yourUserNum`       | "Your user # is" display (supports `\|UN` placeholder)           |
| `welcomeNewUser`    | Welcome message after account creation                           |
| `checkingUserBase`  | "Finding a place for you" message shown during handle validation |
| `nameAlreadyUsed`   | Duplicate name error message                                     |
| `invalidUserName`   | Invalid name error message                                       |
| `pauseString`       | Press Enter to continue prompt                                   |

### ANSI Art

Place a `NEWUSER.ANS` file in `menus/v3/ansi/` to display a welcome screen before the application begins. If the file does not exist, the application proceeds without it.

### After Signup

New accounts are created with:

- `validated: false` — user cannot log in until a SysOp sets this to `true`
- `accessLevel: 1` — minimal access level
- `timeLimit: 60` — 60-minute time limit per call

The SysOp can now validate users in-BBS from the admin menu:

1. Go to MAIN menu and press `X` (SysOp level `s100` required)
2. Press `V` to run user validation
3. Select a user from the pending unvalidated user list
4. Confirm validation (access level is set to configured regular user level)

SysOp accounts now also receive an automatic notification on MAIN menu entry when there are pending unvalidated users.

Regular-user validation level is configurable in `configs/config.json` as `regularUserLevel` (default `10`).

Manual editing of `data/users/users.json` is still supported when the BBS is stopped.

### Modifying Users

To modify a user manually:

1. Stop the BBS
2. Edit `data/users/users.json`
3. Update the desired fields
4. Ensure valid JSON syntax
5. Restart the BBS

## Access Control System (ACS)

ACS strings control access to menus, areas, and functions.

### ACS Syntax

Basic conditions:

- `s10` - Security level 10 or higher
- `fA` - Must have flag A
- `v` - Must be validated
- `u5` - Must be user ID 5

Operators:

- `&` - AND
- `|` - OR
- `!` - NOT
- `()` - Grouping

### Supported ACS Codes

- `S<level>` - Security level >= value
- `F<flag>` - Has specific flag
- `U<id>` - User ID equals value
- `V` - User is validated
- `L` - Local connection
- `A` - ANSI graphics supported (PTY)
- `D<level>` - Download level >= value
- `E<ratio>` - Post/call ratio >= value
- `H<hour>` - Current hour equals value (0-23)
- `P<points>` - File points >= value
- `T<minutes>` - Time left >= value
- `W<day>` - Day of week (0=Sun, 6=Sat)
- `Y<hh:mm/hh:mm>` - Within time range
- `Z<string>` - String exists in private note

### Common ACS Examples

```text
""              # No restrictions (public)
"*"             # Wildcard - always allow
"s10"           # Validated users (level 10+)
"s50"           # Co-SysOp or higher
"s100"          # SysOp only
"v"             # Validated users only
"s10&fD"        # Level 10+ AND download flag
"s20|fS"        # Level 20+ OR special flag
"!fB"           # NOT banned flag
"(s10&fA)|s50"  # (Level 10+ AND flag A) OR level 50+
"y09:00/17:00"  # Business hours only
```

## Call History

The system tracks user calls in `data/users/callhistory.json`:

```json
{
  "userID": 1,
  "handle": "Felonius",
  "groupLocation": "FAiRLiGHT/PC",
  "nodeID": 1,
  "connectTime": "2025-01-01T10:00:00Z",
  "disconnectTime": "2025-01-01T10:30:00Z",
  "duration": 1800000000000,
  "uploadedMB": 0,
  "downloadedMB": 0,
  "actions": "",
  "baudRate": "38400",
  "callNumber": 47
}
```

### Call History Fields

- `userID` - User's ID number
- `handle` - User's handle at time of call
- `groupLocation` - User's group/location
- `nodeID` - Node number used
- `connectTime` - Connection timestamp
- `disconnectTime` - Disconnection timestamp
- `duration` - Call duration in nanoseconds
- `uploadedMB` - Data uploaded (placeholder)
- `downloadedMB` - Data downloaded (placeholder)
- `actions` - Actions performed (placeholder)
- `baudRate` - Connection speed (static)
- `callNumber` - Overall system call number

The system maintains the last 20 call records.

### Invisible Login

Call records include an `invisible` field (boolean, omitted when false) that flags calls where the user logged in invisibly:

```json
{
  "userID": 1,
  "handle": "Felonius",
  "callNumber": 48,
  "invisible": true,
  ...
}
```

See [Invisible Login](#invisible-login-sysopco-sysop) below for details.

## Invisible Login (SysOp/Co-SysOp)

Users at or above the configured `coSysOpLevel` (default 50) are prompted at login:

```text
Invisible Logon? Y/n
```

If they choose **Yes**, the session is flagged invisible for its duration. Invisible sessions are:

- **Hidden from Last Callers** — non-CoSysOp users do not see the call in `RUN:LASTCALLERS`. The call is still logged to `callhistory.json` with `"invisible": true`.
- **Hidden from Who's Online** — invisible sessions are excluded from `RUN:WHOONLINE` listings and the `NODECT` token count for non-CoSysOp viewers.
- **Hidden from Page** — invisible nodes do not appear in the page node list and are treated as offline for non-CoSysOp users attempting to page them.
- **Silent in Chat** — join and leave announcements are suppressed in `RUN:CHAT` for invisible users (they can still chat normally).

**CoSysOp/SysOp users can always see invisible sessions** across all of the above views.

### Configuration

The login prompt text is configurable in `configs/strings.json`:

```json
"invisibleLogonPrompt": " |03Invisible Logon?|07"
```

The access level threshold is driven by `coSysOpLevel` in `configs/config.json` (the same value that controls other CoSysOp privileges).

## Security Considerations

### Password Storage

- Passwords are hashed using bcrypt with default cost
- Never store plain text passwords
- Hash includes salt automatically

### Session Management

- Each login creates a new session
- Sessions tracked by node number
- Automatic logout on disconnect

### Access Control

- Always use appropriate ACS strings
- Test access levels thoroughly
- Document what each flag means

## User Administration Tasks

### Resetting a Password

Since passwords are hashed, you cannot recover them. To reset:

**Option 1 — `ue` TUI (recommended):** Run `./ue` from the BBS root, select the user, and use the password reset field. See [User Editor](user-editor.md).

**Option 2 — manual JSON edit:**

1. Generate a new bcrypt hash:

```go
password := "newpassword"
hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(hash))
```

2. Update the `passwordHash` field in `users.json`
3. Inform the user of their new password

### Validating New Users

Recommended: use the in-BBS admin menu (`X` from MAIN, then `V`) to set `validated=true` and assign access level.

Manual alternative: change `validated` from `false` to `true` and set appropriate `accessLevel` (typically 10) while the BBS is stopped.

Additional admin actions:

- `U` in ADMIN menu: set a user to unvalidated
- `E` in ADMIN menu, then press `0`: toggle ban/unban a user
- `E` in ADMIN menu, then press `9`: toggle delete/undelete a user (soft delete)

### Banning Users

The ban feature sets a user's access level to 0 and marks them as unvalidated, preventing login.

#### How to Ban a User

**Option 1: From Admin User Browser**
1. Press `X` from MAIN menu to enter Admin menu
2. Press `E` to run Edit Users (Admin List Users)
3. Navigate to the user you want to ban
4. Press `0` to toggle ban status (bans if not banned, or unbans if already banned)
5. Press `S` to save changes

Note: Pressing `0` toggles the ban status. If a user is already banned (level 0, unvalidated), pressing `0` will unban them by restoring them to the regular user level (default 10) and setting validated to true.

**Option 2: Manual Edit (while BBS is stopped)**
1. Stop the BBS
2. Edit `data/users/users.json`
3. Set `"accessLevel": 0` and `"validated": false`
4. Restart the BBS

**Alternative Methods:**
1. Add a ban flag (e.g., 'B') and use `!fB` in ACS strings
2. Soft-delete the user (see below)
3. Permanently delete the user entry from JSON (not recommended)

**IMPORTANT: User #1 Protection**

User #1 is protected from administrative actions that could lock you out of the system:
- Cannot be banned (pressing `0` shows an error message)
- Cannot be deleted (pressing `9` shows an error message)
- Cannot be unvalidated (pressing `U` shows an error message)
- Cannot have access level reduced below SysOp level when saving changes

This protection ensures you always have access to the system administration features.

### Deleting Users (Soft Delete)

The soft-delete feature marks users as deleted without removing their data. This is the recommended approach for removing user accounts.

#### How to Delete a User

**Option 1: From Admin User Browser**
1. Press `X` from MAIN menu to enter Admin menu
2. Press `E` to run Edit Users (Admin List Users)
3. Navigate to the user you want to delete
4. Press `9` to toggle deletion (marks for delete if not deleted, or undeletes if already deleted)
5. Press `S` to save changes

Note: Pressing `9` toggles the deletion status. If a user is already deleted, pressing `9` will undelete them and clear the deletion timestamp.

**Option 2: Manual Edit (while BBS is stopped)**
1. Stop the BBS
2. Edit `data/users/users.json`
3. Set `"deletedUser": true` and `"deletedAt": "2026-02-14T12:00:00Z"`
4. Restart the BBS

#### Effects of Soft Delete

When a user is soft-deleted:
- **Cannot log in**: Authentication is denied even with correct credentials
- **Hidden from public views**: Does not appear in user listings or searches
- **Visible in admin tools**: Still appears in Admin User Browser with 'DEL' status
- **Data preserved**: All messages, files, uploads, and call history remain intact
- **Attribution maintained**: Their posts and files remain attributed to their handle

#### Purging Deleted Users

Soft-deleted users are retained for a configurable number of days before they can be permanently purged. The retention period is set in `configs/config.json`:

```json
"deletedUserRetentionDays": 30
```

- **`0`** — purge immediately (on next purge run)
- **`30`** (default) — retain for 30 days after `deletedAt`
- **`-1`** — never purge automatically

Users with no `deletedAt` timestamp (not yet soft-deleted) are never eligible for purge and are always skipped.

**Important:** Purged user IDs are not recycled. The `nextUserID` counter continues to increment, leaving a gap. This is intentional to prevent new accounts from inheriting message attribution from old accounts.

##### CLI: `helper users purge`

Run from the BBS root directory:

```bash
# Preview what would be purged (no changes made)
./helper users purge --dry-run

# Purge using the retention period from config.json
./helper users purge

# Override retention period for this run only
./helper users purge --days 90
```

Each purge run logs affected accounts to `data/users/admin_activity.json` with action `PURGE_USER`.

##### CLI: `helper users list`

```bash
# List all users with status column
./helper users list

# List only soft-deleted users with days-until-purge column
./helper users list --deleted
```

##### Automated Purge via Scheduler

Add to `configs/events.json` to run nightly:

```json
{
  "id": "purge_deleted_users",
  "name": "Nightly Deleted User Purge",
  "schedule": "0 3 * * *",
  "command": "{BBS_ROOT}/helper",
  "args": ["users", "purge"],
  "working_directory": "{BBS_ROOT}",
  "timeout_seconds": 60,
  "enabled": true
}
```

##### Interactive Purge from SysOp Menu

Add `RUN:PURGEUSERS` to a SysOp menu definition. When triggered, it displays all users eligible for purge with their deletion dates, prompts for confirmation, and reports the result. Requires SysOp access level.

### Promoting Users

Increase `accessLevel` and/or add appropriate flags.

## File Management

### Call Number Tracking

- Next call number stored in `data/users/callnumber.json`
- Automatically increments with each connection

### File Locations

- `data/users/users.json` - User database
- `data/users/callhistory.json` - Recent calls
- `data/users/callnumber.json` - Next call number
- `data/users/admin_activity.json` - Admin action log (validate, ban, delete, purge)

## Best Practices

1. **Regular Backups**: Backup all files in `data/users/` regularly
2. **Access Levels**: Use consistent level schemes across your BBS
3. **Flag Documentation**: Document what each flag means
4. **Security**: Never share or expose user data files
5. **Validation**: Validate new users promptly
6. **Unique IDs**: Ensure each user has a unique ID number

## Future Enhancements

The following user management features are planned:

- Password change function (self-service from within BBS)
- Import/export tools
- User statistics and reports
- Time bank system
- Ratio enforcement

## Troubleshooting

### User Can't Login

- Check username (case-insensitive)
- Verify `validated` is true
- Check `accessLevel` > 0
- **Check if user is soft-deleted** (`deletedUser` should be false)
- Ensure password is correct
- Check for duplicate usernames

### Access Denied

- Check menu/area ACS requirements
- Verify user's access level and flags
- Check if user is validated
- Review time restrictions

### Corrupted User File

- Keep backups of all user data
- Validate JSON syntax
- Check for duplicate user IDs
- Ensure array format (not object)
