# ViSiON/3 Message Areas Guide

This guide covers the message system in ViSiON/3, including areas, messages, JAM message bases, and FTN echomail configuration.

## Message System Overview

The message system allows users to post and read messages in different topic areas. Messages are stored in JAM (Joaquim-Andrew-Mats) binary message bases, providing indexed random access, per-user lastread tracking, and full FTN echomail compatibility.

## Message Area Configuration

Message areas are defined in `configs/message_areas.json` as an array:

```json
[
  {
    "id": 1,
    "tag": "GENERAL",
    "name": "General Chat",
    "description": "General discussion area for all users.",
    "acs_read": "",
    "acs_write": "",
    "conference_id": 1,
    "base_path": "msgbases/general",
    "area_type": "local",
    "echo_tag": "",
    "origin_addr": ""
  },
  {
    "id": 2,
    "tag": "FSX_GEN",
    "name": "fsxNet General",
    "description": "fsxNet general discussion echo.",
    "acs_read": "",
    "acs_write": "s10",
    "conference_id": 2,
    "base_path": "msgbases/fsx_gen",
    "area_type": "echomail",
    "echo_tag": "FSX_GEN",
    "origin_addr": "21:3/110"
  }
]
```

### Area Properties

- `id` - Unique numeric identifier
- `tag` - Short tag name (uppercase)
- `name` - Display name
- `description` - Area description
- `acs_read` - ACS required to read messages
- `acs_write` - ACS required to post messages
- `conference_id` - Conference this area belongs to (0 or omitted = ungrouped)
- `base_path` - Relative path to JAM base files (under `data/`). If empty, defaults to `msgbases/<tag>`
- `area_type` - Message type: `"local"`, `"echomail"`, or `"netmail"`
- `echo_tag` - FTN echo tag for echomail areas (e.g., `"FSX_GEN"`)
- `origin_addr` - FTN origin address for echomail (e.g., `"21:3/110"`)
- `max_msgs` - Maximum number of messages to retain (0 = no limit). Oldest messages are removed when the count is exceeded.
- `max_msg_age` - Maximum message age in days (0 = no limit). Messages older than this are removed.

### Area Types

- **local** - Messages stay on this BBS only. No FTN processing.
- **echomail** - Conference-style networked messages. The tosser imports/exports packets. Messages get MSGID, tearline, origin line, and SEEN-BY/PATH.
- **netmail** - Point-to-point private FTN mail between addresses.

## Private Mail System

ViSiON/3 includes a dedicated private mail system for user-to-user messaging. Private mail is stored in a special local message area with the `MSG_PRIVATE` JAM flag set.

### PRIVMAIL Area

The private mail area is defined in `message_areas.json`:

```json
{
  "id": 19,
  "tag": "PRIVMAIL",
  "name": "Private Mail",
  "description": "Private user-to-user mail",
  "acs_read": "",
  "acs_write": "",
  "allow_anonymous": false,
  "real_name_only": false,
  "conference_id": 1,
  "base_path": "msgbases/privmail",
  "area_type": "local"
}
```

### Private Mail Functions

Users access private mail through the Email Menu (press `E` from main menu):

- **SENDPRIVMAIL** - Send private mail to another user
  - Validates recipient username exists
  - Prompts for subject
  - Launches full-screen editor
  - Sets `MSG_PRIVATE` flag on message

- **READPRIVMAIL** - Read private mail
  - Filters messages by `MSG_PRIVATE` flag AND recipient matches current user
  - Shows message count
  - Launches message reader
  - **Security**: Users can only see messages addressed to them

- **LISTPRIVMAIL** - List private mail
  - Shows private mail in list view
  - Displays message headers with private flag indicator

### Security Model

Private mail uses a strict security filter:

```go
if msg.IsPrivate() && strings.EqualFold(msg.To, currentUser.Handle) {
    // User can read this message
}
```

This ensures:
- Messages must have the `MSG_PRIVATE` flag set (0x00000004)
- Message recipient (`To` field) must match the current user's handle
- Case-insensitive comparison for recipient matching
- Users cannot see private mail addressed to others

### Message Attributes

Private messages combine two JAM attribute flags:
- `MsgLocal` (0x00000001) - Created locally
- `MsgPrivate` (0x00000004) - Private message

These flags are stored in the JAM message header's `Attribute` field.

## Message Storage (JAM Format)

Each message area is backed by a JAM message base consisting of 4 files:

| File     | Extension | Contents                                                                |
| -------- | --------- | ----------------------------------------------------------------------- |
| Header   | `.jhr`    | 1024-byte fixed header + variable-length message headers with subfields |
| Text     | `.jdt`    | Raw message text (CP437, CR line endings)                               |
| Index    | `.jdx`    | 8-byte index records (ToCRC + header offset) per message                |
| Lastread | `.jlr`    | 16-byte records tracking per-user lastread pointers                     |

JAM bases are stored under `data/msgbases/` (or the path specified by `base_path`). For example, area `GENERAL` with `base_path: "msgbases/general"` creates:

```ini
data/msgbases/general.jhr
data/msgbases/general.jdt
data/msgbases/general.jdx
data/msgbases/general.jlr
```

JAM bases are created automatically on first use if they don't exist.

### Key JAM Properties

- **1-based message numbering** - Messages are numbered starting from 1
- **Random access** - Any message can be read by number without scanning
- **Binary indexed** - Index file provides O(1) lookup by message number
- **Per-user lastread** - Tracked in .jlr file using CRC32 of username
- **Thread-safe** - All operations use sync.RWMutex for concurrent SSH/telnet sessions
- **FTN-native** - Subfields store MSGID, REPLY, AREA kludges, SEEN-BY, PATH directly

## Message Functions

### Listing Message Areas

Users can list available areas with the `LISTMSGAR` function:

- Shows areas user has read access to
- Displays area ID, tag, name, and description
- Groups areas by conference with conference headers (if conferences are configured)
- Uses templates: `MSGAREA.TOP`, `MSGAREA.MID`, `MSGAREA.BOT`, `MSGCONF.HDR`

### Selecting a Message Area

Users can select a message area with the `SELECTMSGAREA` function:

- Displays the message area list (same as `LISTMSGAR`)
- Prompts for area tag or ID
- Validates read access ACS
- Updates the user's current message area and conference

### Reading Messages

The `READMSGS` function allows reading messages:

- Random-access by message number (no need to load all messages)
- If there are unread messages, starts at the first unread
- If all messages are read, prompts for a specific message number
- Shows message header with sender, date, subject
- Tracks lastread via JAM .jlr file (per-user, per-area)
- Commands: Enter/Space (Next), P (Previous), R (Reply), Q (Quit)

### Composing Messages

The `COMPOSEMSG` function for writing new messages:

- Built-in editor for message composition
- Subject line required
- Messages addressed to "All" by default (public)
- For echomail areas, automatically adds MSGID, tearline, and origin line

### Prompt and Compose

The `PROMPTANDCOMPOSEMESSAGE` function:

- Lists all message areas
- Prompts user to select an area
- Checks write permissions
- Calls compose message for selected area

### New Message Scan

The `NEWSCAN` function scans for new messages:

- Checks all accessible areas (or user's tagged areas)
- Shows count of new messages per area
- Uses JAM lastread tracking (per-user, per-area)
- Offers to jump directly to reading new messages
- During scan setup, user can choose:
  - **All Tagged Areas** - Scans only areas tagged in newscan config
  - **ALL Areas in Conference** - Scans all areas in current conference
  - **Current Area Only** - Scans only the current message area

### Newscan Configuration

The `NEWSCANCONFIG` function (key: **Z** in message menu) allows users to configure their personal newscan preferences:

- Displays scrollable list of all accessible message areas grouped by conference
- Users can tag/untag individual areas with **SPACE** or **ENTER**
- **A** - Tag all areas
- **N** - Clear all tags (none)
- **ESC** - Save and exit
- Tagged areas are saved to the user's profile in `users.json`
- When "All Tagged Areas" is selected in newscan setup, only tagged areas are scanned
- If no areas are tagged, user is prompted to configure their newscan first

## Access Control

### Read Access

- Empty `acs_read` means public access
- Use ACS strings to restrict: `s10`, `fA`, etc.
- Checked when listing areas and reading messages

### Posting Messages

- Empty `acs_write` means all users can post
- Typically requires validation: `s10`
- Can require specific flags: `s10&fM`

## Message Templates

Message display templates are in `menus/v3/templates/`:

### Message Header Template

`MSGHEAD.TPL`:

```text
|08-=[ |15Area |11|CA|08 ]=---=[ |15Msg |11|MNUM|07/|11|MTOTAL|08 ]=-
|07From : |15|MFROM|07      To : |15|MTO|
|07Subj : |15|MSUBJ|
|07Date : |15|MDATE|
|08---------------------------------------------------------------
```

### Message Prompt Template

`MSGREAD.PROMPT`:

```text
[|MNUM/|MTOTAL] Command:
```

### Area List Templates

- `MSGAREA.TOP` - Header
- `MSGAREA.MID` - Row template
- `MSGAREA.BOT` - Footer

### Template Placeholders

- `|CA` - Current area tag
- `|MNUM` - Current message number (1-based)
- `|MTOTAL` - Total messages in area
- `|MFROM` - Sender's handle
- `|MTO` - Recipient
- `|MDATE` - Post date/time
- `|MSUBJ` - Subject line
- `^ID` - Area ID (in area lists)
- `^TAG` - Area tag
- `^NA` - Area name
- `^DS` - Area description

## FTN Echomail Configuration

ViSiON/3 includes a built-in FTN tosser for echomail with support for multiple networks.
Configure it in `configs/ftn.json` (separate from the main `config.json`):

```json
{
  "dupe_db_path": "data/ftn/dupes.json",
  "networks": {
    "fsxnet": {
      "enabled": true,
      "own_address": "21:3/110",
      "inbound_path": "data/ftn/fsxnet/inbound",
      "outbound_path": "data/ftn/fsxnet/outbound",
      "temp_path": "data/ftn/fsxnet/temp",
      "poll_interval_seconds": 300,
      "tearline": "My BBS 1.0",
      "links": [
        {
          "address": "21:1/100",
          "password": "secret",
          "name": "FSXNet Hub",
          "echo_areas": ["FSX_GEN", "FSX_BOT", "FSX_MYS"]
        }
      ]
    }
  }
}
```

### Top-Level Settings

- `dupe_db_path` - JSON file for MSGID dupe tracking (shared across all networks)
- `networks` - Map of network name to network configuration

### Per-Network Settings

Each network key (e.g., `"fsxnet"`) contains:

- `enabled` - Set to `true` to activate the tosser for this network
- `own_address` - Your FTN address on this network (e.g., `"21:3/110"`)
- `inbound_path` - Directory for incoming .PKT files
- `outbound_path` - Directory for outgoing .PKT files
- `temp_path` - Temp directory for failed packets
- `poll_interval_seconds` - How often to scan for packets (0 = manual only)
- `tearline` - Optional tearline text for new echomail posts (prefix `---` is added unless you include it)

### Link Configuration

Each link defines a connected FTN node:

- `address` - Node's FTN address
- `password` - Packet password (shared secret)
- `name` - Human-readable label
- `echo_areas` - List of echo tags to route to this link (use `"*"` for all)

### Message Area Network Field

Echomail areas in `message_areas.json` must include a `"network"` field matching
the network key in `ftn.json`:

```json
{
  "area_type": "echomail",
  "echo_tag": "FSX_GEN",
  "origin_addr": "21:3/110",
  "network": "fsxnet"
}
```

This ties each area to a specific FTN network, allowing the same BBS to
participate in multiple networks (e.g., FSXNet and FidoNet) simultaneously.

### Echomail Flow

**Inbound (receiving):**

1. External mailer (e.g., binkd) drops .PKT files in `inbound_path`
2. Tosser polls at configured interval
3. Each packet is parsed, messages extracted
4. MSGID checked against dupe database
5. AREA tag matched to configured message areas
6. SEEN-BY/PATH updated with own address
7. Message written to JAM base

**Outbound (sending):**

1. User posts message in echomail area
2. Message written to JAM with `DateProcessed=0`
3. Tosser scans for unprocessed messages
4. Creates .PKT files per destination link
5. Adds SEEN-BY/PATH with own address
6. Marks message as processed (`DateProcessed=now`)
7. External mailer picks up .PKT from `outbound_path`

## Creating a New Message Area

1. Edit `configs/message_areas.json`
2. Add new area object to the array:

```json
{
  "id": 2,
  "tag": "TECH",
  "name": "Technical Support",
  "description": "Technical questions and help",
  "acs_read": "",
  "acs_write": "s10",
  "conference_id": 1,
  "base_path": "msgbases/tech",
  "area_type": "local"
}
```

3. Restart BBS (areas are loaded at startup)
4. JAM base files will be created automatically on first access

## Message Management

### JAM Base Maintenance

JAM bases are binary files that may need periodic maintenance:

- **Backups**: Backup the `data/msgbases/` directory regularly
- **Integrity**: If a base becomes corrupted, delete the 4 JAM files and the base will be recreated (messages will be lost)
- **Growth**: JAM text files (.jdt) grow as messages are added. Deleted messages leave gaps that can be reclaimed by packing.

The `jamutil` command handles all message base maintenance. See [Nightly Message Base Maintenance](event-scheduler.md#nightly-message-base-maintenance) in the event scheduler docs for the recommended automated maintenance sequence.

### Message Purge Configuration

Per-area purge limits are set in `message_areas.json` via `max_msgs` and `max_msg_age`. Both default to `0` (no limit). When set, limits are enforced nightly by `jamutil purge --all`:

```json
{
  "tag": "FSX_GEN",
  "max_msg_age": 90
}
```

```json
{
  "tag": "GENERAL",
  "max_msgs": 500
}
```

Both limits can be set simultaneously. Age is applied first; if the remaining count still exceeds `max_msgs`, the oldest messages are removed until the limit is met.

### Dupe Database

The FTN dupe database (`data/ftn/dupes.json`) tracks MSGIDs to prevent duplicate imports. Old entries are automatically purged after 30 days.

## Current Limitations

### Not Yet Implemented

- Message deletion/moderation UI
- Anonymous posting
- Message search
- File attachments

### In Development

- Network message support is functional (echomail tosser built in)
- Message threading view

## Troubleshooting

### Messages Not Showing

- Check user's access level vs `acs_read`
- Verify JAM base files exist in `data/msgbases/`
- Check BBS logs for JAM open errors
- Ensure area ID matches configuration

### Can't Post Messages

- Verify `acs_write` requirements are met
- Check user is validated if required
- Ensure area exists in configuration
- Check write permissions on `data/msgbases/` directory

### Echomail Not Working

- Verify `ftn.enabled` is `true` in config.json
- Check `own_address` is set correctly
- Ensure echo area tags match between config and message_areas.json
- Verify inbound/outbound directories exist and are writable
- Check logs for tosser errors

## API Reference

### MessageManager Methods

- `NewMessageManager(dataPath, configPath, boardName)` - Initialize with JAM bases
- `Close()` - Close all JAM bases (call on shutdown)
- `ListAreas()` - Returns all message areas sorted by ID
- `GetAreaByID(id)` - Get area by numeric ID
- `GetAreaByTag(tag)` - Get area by tag name
- `AddMessage(areaID, from, to, subject, body, replyMsgID)` - Add new message (returns msgNum)
- `GetMessage(areaID, msgNum)` - Read single message by number
- `GetMessageCountForArea(areaID)` - Total message count
- `GetNewMessageCount(areaID, username)` - Unread count for user
- `GetLastRead(areaID, username)` - Last read message number
- `SetLastRead(areaID, username, msgNum)` - Update lastread pointer
- `GetNextUnreadMessage(areaID, username)` - Next unread message number
- `GetBase(areaID)` - Get underlying JAM base (for tosser)
