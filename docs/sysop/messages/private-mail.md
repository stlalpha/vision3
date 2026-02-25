# Private Mail

ViSiON/3 includes a dedicated user-to-user mail system. "Private" here means the message is addressed to a specific user rather than posted publicly to a board — it is **not** encrypted or secure in any modern sense. Messages are stored as plaintext in a JAM base on disk, and sysops can read them. This is how BBS mail worked in the 90s.

The `MSG_PRIVATE` JAM flag simply controls whether the BBS filters the message out of other users' mail readers.

## Setup

Define the private mail area in `configs/message_areas.json`:

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

## User Access

Users access private mail through the Email Menu (press `E` from the main menu):

- **SENDPRIVMAIL** — Send private mail to another user; validates recipient exists, prompts for subject, launches the full-screen editor
- **READPRIVMAIL** — Read private mail; shows only messages addressed to the current user
- **LISTPRIVMAIL** — List private mail headers

---

## Technical Reference

### Read Filter

The private mail reader applies a two-part filter before showing any message:

```go
if msg.IsPrivate() && strings.EqualFold(msg.To, currentUser.Handle) {
    // User can read this message
}
```

This means:
- The message must have the `MSG_PRIVATE` flag set (0x00000004)
- The `To` field must match the current user's handle (case-insensitive)
- Other users' mail readers will not show the message — but it is not encrypted and sysops with filesystem access can read anything

### Message Attributes

Private messages combine two JAM attribute flags stored in the header's `Attribute` field:

- `MsgLocal` (0x00000001) — Created locally
- `MsgPrivate` (0x00000004) — Private message
