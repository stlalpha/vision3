# Message Header Placeholders

Vision3 replaces `@X@` placeholders in message header templates (`menus/v3/templates/message_headers/MSGHDR.*`) at render time. These placeholders are handled by the message reader and are different from general menu pipe codes.

## Placeholder Format

Vision3 supports three placeholder formats:

1. **Simple format**: `@T@` - Inserts value as-is (no width constraint)
2. **Parameter width**: `@T:20@` - Explicit width (truncates/pads to exactly 20 characters)
3. **Visual width**: `@T############@` - Width shown by # character count (self-documenting)

The visual placeholder format is particularly useful for ANSI art templates where precise character positioning is critical. The `#` characters show exactly how much horizontal space the field will occupy, making it easy to design layouts visually.

## Available Placeholders

| Code   | Replaced With                                                                                                                           |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------- |
| `@B@`  | Current message area tag                                                                                                                |
| `@T@`  | Message subject                                                                                                                         |
| `@F@`  | From (sender). Includes FTN origin address when available; may also include the user note when `@U@` is not present in the template |
| `@S@`  | To (recipient). Includes FTN destination address when available                                                                         |
| `@U@`  | User note (local messages only). Blank for echomail/netmail                                                                             |
| `@M@`  | Message status (LOCAL, ECHOMAIL, NETMAIL, READ, PRIVATE, etc.)                                                                          |
| `@L@`  | User access level                                                                                                                       |
| `@R@`  | Real name (not available in JAM; currently empty)                                                                                       |
| `@#@`  | Current message number (1-based)                                                                                                        |
| `@N@`  | Total messages in area                                                                                                                  |
| `@D@`  | Message date (`MM/DD/YY`)                                                                                                               |
| `@W@`  | Message time (`h:mm am/pm`)                                                                                                             |
| `@P@`  | Reply message ID (or `None`)                                                                                                            |
| `@E@`  | Replies count (not tracked in JAM; currently `0`)                                                                                       |
| `@O@`  | Origin FTN address (if present)                                                                                                         |
| `@A@`  | Destination FTN address (if present)                                                                                                    |

## Width Control Features

### Parameter-Based Width

Use `:NUMBER` after the code for explicit width control:

```
@T:40@     - Subject truncated/padded to exactly 40 characters
@F:25@     - From field formatted to 25 characters
@M:20@     - Message status formatted to 20 characters
@D:8@ @W:8@ - Date and time with fixed widths
```

### Visual Placeholder Width

Use `#` characters to show the intended field width directly in your template:

```
@T########################@  - Subject field (24 characters wide)
@F###################@      - From field (19 characters wide)
@M################@         - Status field (16 characters wide)
```

**How it works:**
- The total width is determined by counting the `#` characters
- When rendered, the entire placeholder tag is replaced with the value padded/truncated to that exact width
- This makes templates self-documenting - you can see the field allocations visually
- ANSI color codes are preserved when truncating values

**Example template:**

```
Posted on @D@@@@@@@@ @W@@@@@@@@       @M@@@@@@@@@@@@@@@@@@@
From: @F@@@@@@@@@@@@@@@@@@@@@@@@@  To: @S@@@@@@@@@@@@@@@@@@
Subj: @T@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
```

This shows exactly how much space each field occupies in the layout.

## Editing ANSI Templates

### Direct Editing (Recommended)

You can now edit message header ANSI files **directly** in any ANSI editor without pre/post-processing:

1. Open the template file in your preferred ANSI editor (Moebius, TheDraw, PabloDraw, etc.)
2. The `@` delimiter does not conflict with ANSI editor pipe codes
3. Edit the template as needed
4. Save the file
5. Done!

### Example Templates

#### Simple Text Header (MSGHDR.2)

```
@#@: @T@
Name: @F@ [@U@]
Date: @D@ @W@
  To: @S@
```

#### Header with Width Control

```
Msg: @#:5@/@N:5@       Posted: @D:8@ @W:9@
From: @F:30@                To: @S:25@
Subj: @T:70@
```

#### ANSI Art with Visual Placeholders

```
╔════════════════════════════════════════════════════════════╗
║ Message @#:4@/@N:4@         Date: @D@@@@@@@ @W@@@@@@@@  ║
╟────────────────────────────────────────────────────────────╢
║ From: @F@@@@@@@@@@@@@@@@@@@@ To: @S@@@@@@@@@@@@@@@@@@@@ ║
║ Subj: @T@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@ ║
╚════════════════════════════════════════════════════════════╝
```

## Technical Details

### Runtime Processing

Placeholders are processed when messages are displayed, not when templates are edited. This means:

- No encoding/decoding workflow required
- Changes to templates take effect immediately
- Template files remain human-readable
- ANSI editors work without special handling

### ANSI-Aware String Handling

When width constraints are applied:

- **ANSI color codes are preserved** during truncation
- Escape sequences don't count toward the visible width
- Padding is applied after ANSI codes to maintain exact field widths
- Color resets are appended if text is truncated mid-color

Example: `\x1b[31mLong Red Text\x1b[0m` truncated to 5 chars becomes `\x1b[31mLong \x1b[0m` (color codes intact, 5 visible characters)

### Backward Compatibility

Vision3 maintains support for legacy `|X` format templates:

- Template format is auto-detected at runtime
- Presence of `@T@`, `@F@`, or `@S@` triggers new format processing
- Otherwise, legacy `|X` format is used
- Both formats work in the same system
- Gradual migration is supported

## Notes

- Templates are raw ANSI with absolute cursor positioning
- `@U@` only shows for local messages. FTN messages do not have a local user note
- If your template includes `@U@`, the user note is not also injected into `@F@`
- Visual placeholders make it easy to design precise ANSI layouts
- The `@` delimiter was chosen to avoid conflicts with ANSI editor pipe codes

## Migration from Legacy Format

If you have old templates using `|X` format, they will continue to work. To convert them to the new format:

1. Replace all `|B` with `@B@`
2. Replace all `|T` with `@T@`
3. Replace all `|F` with `@F@`
4. (Continue for all 16 codes: B, T, F, S, U, M, L, R, #, N, D, W, P, E, O, A)
5. Optionally add width constraints for better layout control

Example conversion:
```
Before: |#: |T
After:  @#@: @T@

With width: @#:4@: @T:50@
```

## Advantages Over Legacy Format

- ✅ **No ANSI editor conflicts** - `@` delimiter doesn't interfere with pipe color codes
- ✅ **Visual design aid** - `###` placeholders show field widths in the template
- ✅ **Direct editing** - Edit templates in any ANSI editor without conversion
- ✅ **Self-documenting** - Field allocations are visible in the template
- ✅ **Width control** - Precise layouts with parameter or visual width
- ✅ **ANSI-aware** - Color codes preserved during truncation
- ✅ **Simple workflow** - No encode/decode scripts required
