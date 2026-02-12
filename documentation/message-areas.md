# ViSiON/3 Message Areas Guide

This guide covers the message system in ViSiON/3, including areas, messages, and configuration.

## Message System Overview

The message system allows users to post and read messages in different topic areas. Messages are stored in JSON Lines format for efficiency.

## Message Area Configuration

Message areas are defined in `data/message_areas.json` as an array:

```json
[
  {
    "id": 1,
    "tag": "GENERAL",
    "name": "General Chat",
    "description": "General discussion area for all users.",
    "acs_read": "",
    "acs_write": "",
    "is_networked": false,
    "origin_node_id": ""
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
- `is_networked` - Whether this area is networked (echomail)
- `origin_node_id` - ID of the node where this networked area originated
- `last_message_id` - Optional: ID of the last message posted

Note: Fields like moderation, anonymous posting, and message limits are not yet implemented.

## Message Storage

Messages are stored in `data/messages_area_X.jsonl` where X is the area ID. Each line is a JSON message object:

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "area_id": 1,
  "from_user_name": "Felonius",
  "from_node_id": "1:1/100",
  "to_user_name": "All",
  "to_node_id": "",
  "subject": "Welcome to ViSiON/3",
  "body": "This is the message body...",
  "posted_at": "2025-01-01T10:00:00Z",
  "reply_to_id": "",
  "is_private": false,
  "msg_id": "",
  "path": [],
  "read_by": [],
  "attributes": []
}
```

### Message Fields

- `id` - UUID of the message
- `area_id` - ID of the message area
- `from_user_name` - Handle of the sender
- `from_node_id` - Node ID where message originated
- `to_user_name` - Target user or "All" for public
- `to_node_id` - Target node ID (for private mail)
- `subject` - Message subject
- `body` - Message content
- `posted_at` - Timestamp when posted
- `reply_to_id` - UUID of message this replies to
- `is_private` - True for private mail
- `msg_id` - Optional FidoNet-style MSGID
- `path` - Node IDs message traversed
- `read_by` - User IDs who have read this
- `attributes` - FidoNet-style attributes

## Message Functions

### Listing Message Areas

Users can list available areas with the `LISTMSGAR` function:
- Shows areas user has read access to
- Displays area ID, tag, name, and description
- Uses templates: `MSGAREA.TOP`, `MSGAREA.MID`, `MSGAREA.BOT`

### Reading Messages

The `READMSGS` function allows reading messages:
- Sequential reading with navigation commands
- Shows message header with sender, date, subject
- Tracks last read message per area per user
- Reply functionality
- Commands: N (Next), P (Previous), R (Reply), Q (Quit), A (Again), E (Enter new)

### Composing Messages

The `COMPOSEMSG` function for writing new messages:
- Built-in line editor
- Subject line required
- Messages are addressed to "All" (public)
- Supports reply threading via reply_to_id

### Prompt and Compose

The `PROMPTANDCOMPOSEMESSAGE` function:
- Lists all message areas
- Prompts user to select an area
- Checks write permissions
- Calls compose message for selected area

### New Message Scan

The `NEWSCAN` function scans for new messages:
- Checks all accessible areas
- Shows count of new messages per area
- Tracks using user's last_read_message_ids map

## Access Control

### Reading Messages
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
`MSGHEAD.TPL` or `message_headers/MSGHDR.X` where X is a style number:
```
|08-=[ |15Area |11|CA|08 ]=---=[ |15Msg |11|MNUM|07/|11|MTOTAL|08 ]=-
|07From : |15|MFROM|07      To : |15|MTO|
|07Subj : |15|MSUBJ|
|07Date : |15|MDATE|
|08---------------------------------------------------------------
```

### Message Prompt Template
`MSGREAD.PROMPT`:
```
[|MNUM/|MTOTAL] Command: 
```

### Area List Templates
- `MSGAREA.TOP` - Header
- `MSGAREA.MID` - Row template
- `MSGAREA.BOT` - Footer

### Template Placeholders
- `|CA` - Current area tag
- `|MNUM` - Message number in sequence
- `|MTOTAL` - Total messages in area
- `|MFROM` - Sender's handle
- `|MTO` - Recipient
- `|MDATE` - Post date/time
- `|MSUBJ` - Subject line
- `^ID` - Area ID (in area lists)
- `^TAG` - Area tag
- `^NA` - Area name
- `^DS` - Area description

## Creating a New Message Area

1. Edit `data/message_areas.json`
2. Add new area object to the array:
```json
{
  "id": 2,
  "tag": "TECH",
  "name": "Technical Support",
  "description": "Technical questions and help",
  "acs_read": "",
  "acs_write": "s10",
  "is_networked": false,
  "origin_node_id": ""
}
```
3. Restart BBS (areas are loaded at startup)

## Message Management

### Storage Format
- JSON Lines format (`.jsonl`)
- One message per line
- Efficient for append operations
- Each line must be valid JSON

### Manual Message Operations
Currently requires direct file editing:
1. Stop BBS
2. Edit `data/messages_area_X.jsonl`
3. Add/remove message lines
4. Ensure valid JSON on each line
5. Restart BBS

## Current Limitations

### Not Yet Implemented
- Message deletion/moderation
- Anonymous posting
- Message limits per area
- Message search
- Bulk import/export
- File attachments
- Private messages between users
- Message voting/rating

### In Development
- Network message support (FidoNet style)
- Message threading view
- Automated maintenance

## Best Practices

1. **Area Organization**: Create focused topic areas
2. **Access Control**: Set appropriate ACS levels
3. **Regular Backups**: Backup message files regularly
4. **Monitor Growth**: Watch file sizes as messages accumulate
5. **JSON Validation**: Ensure message files remain valid JSON Lines

## Troubleshooting

### Messages Not Showing
- Check user's access level vs `acs_read`
- Verify message file exists: `data/messages_area_X.jsonl`
- Check for JSON parsing errors in the file
- Ensure area ID matches filename

### Can't Post Messages
- Verify `acs_write` requirements are met
- Check user is validated if required
- Ensure area exists in configuration
- Check write permissions on data directory

### Corrupt Message File
- Each line must be valid JSON
- Remove problematic lines
- Use a JSON validator on individual lines
- Check for incomplete writes

## API Reference

### MessageManager Methods
- `ListAreas()` - Returns all message areas
- `GetAreaByID(id)` - Get area by numeric ID
- `GetAreaByTag(tag)` - Get area by tag name
- `AddMessage(areaID, message)` - Add new message
- `GetMessagesForArea(areaID, sinceID)` - Get messages
- `GetMessageCountForArea(areaID)` - Count messages
- `GetNewMessageCount(areaID, sinceID)` - Count new messages 