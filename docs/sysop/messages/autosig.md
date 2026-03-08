# Auto-Signature

The Auto-Signature feature allows users to set a personal signature that is automatically appended to every message they post. This applies to public message areas, private mail, and QWK replies.

---

## How It Works

Each user has an `AutoSignature` field stored in their user record (`data/users/users.json`). When a message is composed and saved, the signature text is appended after two blank lines:

```
Message body text here.

--- user's auto-signature ---
```

The signature is appended in all message posting contexts:

- Composing new messages (`RUN:COMPOSEMSG`)
- Replying to messages in the message reader
- Private mail composition
- QWK reply packet uploads

The signature is **not** appended to anonymous messages.

---

## User Interface (`RUN:CFG_AUTOSIG`)

Users manage their auto-signature through a simple menu:

```
Auto-Signature
An Auto-Signature is appended to the end of any message you post.

Your current Auto-Signature is...

  --- my sig here ---

Change/create  Delete  Quit :
```

| Key | Action |
|-----|--------|
| `C` | Open the ANSI editor to create or modify the signature |
| `D` | Delete the current signature |
| `Q` | Return to menu |

### Editor

Pressing `C` launches the full-screen ANSI message editor with the current signature pre-loaded. The editor supports pipe color codes, so users can create colorful signatures. On save, the signature is truncated to a maximum of **5 lines** to prevent abuse.

If the user saves an empty editor session, the signature is cleared.

---

## Menu Configuration

The auto-signature editor is available from two default locations:

### Message Menu (`S` key)

```json
{
    "KEYS": "S",
    "CMD": "RUN:CFG_AUTOSIG",
    "ACS": "*",
    "HIDDEN": false,
    "NODE_ACTIVITY": "Editing Auto-Signature"
}
```

### User Settings Menu (`R` key)

```json
{
    "KEYS": "R",
    "CMD": "RUN:CFG_AUTOSIG",
    "ACS": "",
    "HIDDEN": false
}
```

You can add `RUN:CFG_AUTOSIG` to any menu CFG file to make it accessible from other locations.

---

## Limits

| Setting | Value |
|---------|-------|
| Maximum lines | 5 |
| Line length | Limited by editor width (typically 79 characters) |

If a user's signature exceeds 5 lines, it is silently truncated and a notification is displayed:

```
Signature truncated to 5 lines.
```

The `maxAutoSigLines` constant is defined in `internal/menu/user_config.go`.

---

## Data Storage

The auto-signature is stored as a string in the user's JSON record:

```json
{
    "username": "example",
    "auto_signature": "--- my sig ---\nLine 2\nLine 3"
}
```

Lines are separated by `\n`. The field is empty (`""`) when no signature is set.

---

## See Also

- [User Management](../users/user-management.md) — user record fields
- [Message Areas](message-areas.md) — where auto-signatures appear
- [QWK Offline Mail](qwk.md) — auto-signature in QWK replies
