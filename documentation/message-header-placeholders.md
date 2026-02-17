# Message Header Placeholders

Vision3 replaces `@X@` placeholders in message header templates (`menus/v3/templates/message_headers/MSGHDR.*`) at render time. These placeholders are handled by the message reader and are different from general menu pipe codes.

## Placeholder Format

Vision3 supports four placeholder formats:

1. **Simple format**: `@T@` - Inserts value as-is (no width constraint)
2. **Parameter width**: `@T:20@` - Explicit width (truncates/pads to exactly 20 characters)
3. **Visual width**: `@T############@` - Width shown by # character count (self-documenting)
4. **Auto-width**: `@T*@` - Width automatically calculated from context (see below)

The visual placeholder format is particularly useful for ANSI art templates where precise character positioning is critical. The `#` characters show exactly how much horizontal space the field will occupy, making it easy to design layouts visually.

## Available Placeholders

| Code  | Replaced With                                                                                                                       |
| ----- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `@B@` | Current message area tag                                                                                                            |
| `@T@` | Message subject                                                                                                                     |
| `@F@` | From (sender). Includes FTN origin address when available; may also include the user note when `@U@` is not present in the template |
| `@S@` | To (recipient). Includes FTN destination address when available                                                                     |
| `@U@` | User note for the message author, looked up from users.json. Blank if author is not a local user                                    |
| `@M@` | Message status (LOCAL, ECHOMAIL, NETMAIL, READ, PRIVATE, etc.)                                                                      |
| `@L@` | User access level                                                                                                                   |
| `@#@` | Current message number (1-based)                                                                                                    |
| `@N@` | Total messages in area                                                                                                              |
| `@C@` | Message count display (format: `[current/total]` e.g., `[1/24]`, `[6/10]`)                                                          |
| `@V@` | Verbose message count (format: `current of total` e.g., `1 of 24`, `6 of 10`)                                                       |
| `@D@` | Message date (`MM/DD/YY`)                                                                                                           |
| `@W@` | Message time (`h:mm am/pm`)                                                                                                         |
| `@P@` | Reply-to message number (e.g., `12`), or `None` if the message is not a reply                                                       |
| `@E@` | Thread reply count — number of other messages with the same subject (e.g., `3`). Returns `0` if no replies                          |
| `@O@` | Origin FTN address (if present)                                                                                                     |
| `@A@` | Destination FTN address (if present)                                                                                                |
| `@Z@` | Combined conference and area name (format: `CONF NAME > AREA NAME`)                                                                 |
| `@X@` | Combined conference/area and message count (format: `CONF NAME > AREA NAME [current/total]`)                                        |
| `@G@` | Gap fill: fills remaining line width with `─` (CP437 0xC4). See Gap Fill section below                                              |

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
@T########################@  - Subject field (full placeholder length characters wide)
@F###################@      - From field (full placeholder length characters wide)
@M################@         - Status field (full placeholder length characters wide)
```

**How it works:**
- The total width is determined by the complete placeholder length (including @, code, #'s, and final @)
- When rendered, the entire placeholder tag is replaced with the value padded/truncated to that exact width
- This makes templates self-documenting - you can see the exact field allocations visually
- ANSI color codes are preserved when truncating values

### Auto-Width (`*` modifier)

Use `*` after the code to have the width automatically calculated from context:

```
@#*@       - Message number padded to the width of the highest message number
@N*@       - Total messages padded to its own digit count
@C*@       - Count display [x/y] padded to max possible width
@V*@       - Verbose count "x of y" padded to max possible width
@D*@       - Date padded to 8 (always MM/DD/YY)
@W*@       - Time padded to 8 (max "12:00 pm")
@Z*@       - Conference > Area padded to its current length
@X*@       - Conference > Area [x/y] padded to max possible width
@T*@       - Subject padded to its current length
@F*@       - From padded to its current length
```

**How it works:**
- The width is determined at render time based on the actual data context
- For numeric codes like `@#*@`, the width equals the digit count of the total message count, ensuring consistent alignment across all messages (e.g., message "3" becomes "3   " when there are 1500 messages)
- For `@C*@` and `@X*@`, the width accounts for the maximum possible `[current/total]` display
- For `@Z*@`, the width matches the current conference/area name combination
- Fixed-format codes like `@D*@` and `@W*@` use their known maximum format lengths
- For other codes, the width matches the current value's length

**When to use auto-width vs other formats:**
- Use `@#*@` instead of `@#:5@` when you want consistent number alignment without hardcoding the width
- Use `@Z*@` or `@X*@` when you want the field to fit the current area name exactly
- Use explicit `:WIDTH` or `###` when you need a specific fixed width for ANSI art layouts
- Use `@T@` (no width) when the field can be any length

**Example template:**

```
Posted on @D######@ @W########@       @M###################@
From: @F#########################@  To: @S##################@
Subj: @T#####################################################@
```

This shows exactly how much space each field occupies in the layout.

### Gap Fill (`@G@`)

The `@G@` placeholder fills remaining space on the current line with `─` (CP437 horizontal line, 0xC4) characters. This is useful for separator lines that contain variable-width fields like message numbers.

**Formats:**
- `@G@` - Fill to 80 columns (default)
- `@G:79@` - Fill to explicit column width
- `@G*@` - Fill to terminal width (auto-detected)

**How it works:**
1. All other placeholders on the line are substituted first
2. The visible character count of the line (excluding ANSI codes) is calculated
3. The fill count = target width - visible characters
4. The `@G@` marker is replaced with that many `─` characters
5. If the line already exceeds the target width, no fill is added

**Example:**

```
────────────────────────────────────────────────────── @#@ of @N@ @G*@
```

With 42 messages, message 1:
```
────────────────────────────────────────────────────── 1 of 42 ─────────────────
```

With 1500 messages, message 3:
```
────────────────────────────────────────────────────── 3 of 1500 ───────────────
```

The `─` fill automatically adjusts so the line always reaches the terminal width, regardless of the message number sizes.

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

```text
@#@: @T@
Name: @F@ [@U@]
Date: @D@ @W@
  To: @S@
```

#### Header with Width Control

```text
Msg: @#:5@/@N:5@       Posted: @D:8@ @W:9@
From: @F:30@                To: @S:25@
Subj: @T:70@
```

#### Using the Message Count Display

```text
Reading message @C@ in @B@
Subject: @T@
From: @F@
```

This would display as:
```text
Reading message [3/24] in FSX_GEN
Subject: Welcome to Vision3!
From: sysop
```

#### Using the Combined Area and Count Display

```text
@X@
Subject: @T@
From: @F@
```

This would display as:
```text
Local Areas > General Discussion [3/24]
Subject: Welcome to Vision3!
From: sysop
```

#### Using Auto-Width for Consistent Alignment

```text
Msg: @#*@/@N*@       Posted: @D*@ @W*@
From: @F@                To: @S@
Subj: @T@
```

With 1500 messages, message 3 would display as:
```text
Msg: 3   /1500       Posted: 01/15/26 2:30 pm
From: sysop                To: All
Subj: Welcome to Vision3!
```

The `@#*@` auto-pads to 4 characters (matching the width of "1500"), keeping alignment consistent across all messages.

#### ANSI Art with Visual Placeholders

```text
╔════════════════════════════════════════════════════════════╗
║ Message @C######@           Date: @D######@ @W#########@   ║
╟────────────────────────────────────────────────────────────╢
║ From: @F####################@ To: @S###################@   ║
║ Subj: @T###############################################@   ║
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
- `@U@` looks up the message author in users.json and displays their note. Works for any message type (local, echomail, netmail) — if the author is a local user, their note is shown
- If your template includes `@U@`, the user note is not also injected into `@F@`
- Visual placeholders make it easy to design precise ANSI layouts
- The `@` delimiter was chosen to avoid conflicts with ANSI editor pipe codes

## Migration from Legacy Format

If you have old templates using `|X` format, they will continue to work. To convert them to the new format:

1. Replace all `|B` with `@B@`
2. Replace all `|T` with `@T@`
3. Replace all `|F` with `@F@`
4. (Continue for all codes: B, T, F, S, U, M, L, #, N, D, W, P, E, O, A)
5. Optionally add width constraints for better layout control
6. Consider using new `@Z@` placeholder for combined conference/area display (not available in legacy format)

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
