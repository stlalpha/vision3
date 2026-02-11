# Message Header Placeholders

Vision3 replaces `|X` placeholders in message header templates (`menus/v3/templates/message_headers/MSGHDR.*`) at render time. These placeholders are handled by the message reader and are different from general menu pipe codes.

## Available Placeholders

| Code  | Replaced With                                                                                                                           |
| ----- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `\|B` | Current message area tag                                                                                                                |
| `\|T` | Message subject                                                                                                                         |
| `\|F` | From (sender). Includes FTN origin address when available; may also include the user note when `&#124;U` is not present in the template |
| `\|S` | To (recipient). Includes FTN destination address when available                                                                         |
| `\|U` | User note (local messages only). Blank for echomail/netmail                                                                             |
| `\|M` | Message status (LOCAL, ECHOMAIL, NETMAIL, READ, PRIVATE, etc.)                                                                          |
| `\|L` | User access level                                                                                                                       |
| `\|R` | Real name (not available in JAM; currently empty)                                                                                       |
| `\|#` | Current message number (1-based)                                                                                                        |
| `\|N` | Total messages in area                                                                                                                  |
| `\|D` | Message date (`MM/DD/YY`)                                                                                                               |
| `\|W` | Message time (`h:mm am/pm`)                                                                                                             |
| `\|P` | Reply message ID (or `None`)                                                                                                            |
| `\|E` | Replies count (not tracked in JAM; currently `0`)                                                                                       |
| `\|O` | Origin FTN address (if present)                                                                                                         |
| `\|A` | Destination FTN address (if present)                                                                                                    |

**Notes**

- Templates are raw ANSI with absolute cursor positioning. Use a binary-safe editor or placeholder round-trip when editing.
- `&#124;U` only shows for local messages. FTN messages do not have a local user note.
- If your template includes `&#124;U`, the user note is not also injected into `&#124;F`.
