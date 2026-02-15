# Message Reader Scrolling Implementation

## Summary

Successfully implemented scrollable message reading in Vision3 while preserving existing features (MSGHDR templates, lightbar menu, thread searching). This is a hybrid approach combining Vision3's existing structure with Retrograde's scrolling capabilities.

## What Was Implemented

### 1. Scrollable Body Region

**New Function:** `readKeySequence()` in `internal/menu/message_lightbar.go`

- Reads complete escape sequences for arrow keys and Page Up/Down
- Handles multi-byte sequences with timeout protection
- Returns string for easy switch statement handling

**Supported Scrolling Keys:**

- `↑` Up Arrow - Scroll up one line
- `↓` Down Arrow - Scroll down one line
- `Page Up` - Scroll up one page (bodyHeight - 2 lines)
- `Page Down` - Scroll down one page

### 2. Fixed Footer Region

**Footer Always Anchored at Bottom:**

- Line `termHeight - 1`: Board info showing current area and message count
- Line `termHeight`: Lightbar menu with commands

**Scroll Percentage Indicator:**

- Shows percentage (e.g., `[45%]`) when message is longer than display area
- Calculated as: `(scrollOffset * 100) / maxScroll`
- Dynamically updates as user scrolls

### 3. Three-Region Display Architecture

```text
┌─────────────────────────────────────┐
│   Header (MSGHDR template)          │ ← Fixed, never redraws during scroll
│   Variable height (6-10 lines)      │
├─────────────────────────────────────┤
│                                     │
│   Scrollable Body Region            │ ← Redraws on scroll with new window
│   termHeight - headerHeight - 2     │
│                                     │
├─────────────────────────────────────┤
│ Current (AREA) [msg/total]          │ ← Fixed footer line 1
│ Next Reply Again Skip... [25%]      │ ← Fixed footer line 2 (lightbar)
└─────────────────────────────────────┘
```

### 4. Inner Scroll Loop

**New Control Flow:**

1. Outer loop (`readerLoop`): Iterates through messages
2. Inner loop (`scrollLoop`): Handles scrolling within current message
3. User can scroll up/down/pageup/pagedown without changing message
4. Command keys (N, R, P, etc.) break out of scroll loop to change messages

### 5. Efficient Rendering

**Full Redraw** (only when needed):

- New message loaded
- After reply/post commands
- After help display
- When needsRedraw flag is set

**Partial Redraw** (during scrolling):

- Only message body lines are repositioned
- Header stays in place
- Footer updates with new scroll percentage

**Explicit Cursor Positioning:**

```go
for i := 0; i < bodyAvailHeight; i++ {
    lineNum := bodyStartRow + i
    // Position cursor at specific line
    terminalio.WriteProcessedBytes(terminal, []byte(ansi.MoveCursor(lineNum, 1)), outputMode)
    // Clear line
    terminalio.WriteProcessedBytes(terminal, []byte("\x1b[K"), outputMode)
    // Display line if available
    lineIdx := scrollOffset + i
    if lineIdx < totalBodyLines {
        terminalio.WriteProcessedBytes(terminal, []byte(wrappedBodyLines[lineIdx]), outputMode)
    }
}
```

## What Was Preserved

### Vision3 Features Kept Intact

1. **MSGHDR Template System**
   - Uses @CODE@ placeholder format (`@T@`, `@F@`, `@S@`, etc.) with width control
   - Backward compatible with legacy |X format
   - All pre-made header styles work unchanged
   - Template selection via lightbar navigation (unlimited templates supported)

2. **Lightbar Menu**
   - 10-option lightbar at bottom
   - Same command keys (N, R, A, S, T, P, J, M, L, Q)
   - Same colors from theme system

3. **Thread Searching**
   - Forward/backward thread navigation
   - Subject matching logic unchanged

4. **Message Area Management**
   - Area switching
   - NewScan mode
   - Message counts and tracking

5. **Command Handling**
   - Reply to message
   - Post new message
   - Jump to message number
   - Help system
   - All existing commands work as before

## Key Improvements Over Previous Implementation

### Before (Pagination-Based)

- User sees message in chunks
- Must press Enter to continue reading
- "More" prompts interrupt flow
- Cannot scroll back up
- Commands only available after reading entire message
- No indication of message length or position

### After (Scrollable)

- See entire message (or scrollable portion)
- No interruptions - read at your own pace
- Smooth arrow key and page up/down scrolling
- Can scroll back up to re-read content
- Commands always visible in footer
- Scroll percentage shows position in long messages

## Technical Details

### Pre-formatted Message Lines

All body lines are now formatted before display:

```go
// Process message body and pre-format all lines
processedBodyStr := string(ansi.ReplacePipeCodes([]byte(currentMsg.Body)))
wrappedBodyLines := wrapAnsiString(processedBodyStr, termWidth)

// Add origin line for echo messages
if area != nil && (area.AreaType == "echomail" || area.AreaType == "netmail") {
    if currentMsg.OrigAddr != "" {
        originLine := fmt.Sprintf("|08 * Origin: %s", currentMsg.OrigAddr)
        wrappedBodyLines = append(wrappedBodyLines, string(ansi.ReplacePipeCodes([]byte(originLine))))
    }
}
```

### Scroll State Management

```go
scrollOffset := 0              // Current scroll position (0 = top)
totalBodyLines := len(wrappedBodyLines)  // Total lines in message
needsRedraw := true            // Flag to trigger full redraw
```

### Scroll Bounds Checking

```go
// Page Down example
scrollOffset += pageSize
maxScroll := totalBodyLines - bodyAvailHeight
if maxScroll < 0 {
    maxScroll = 0
}
if scrollOffset > maxScroll {
    scrollOffset = maxScroll
}
```

### Command Key Handling

**Direct Command Keys** (bypass lightbar):

- Single letters: N, R, A, S, T, P, J, M, L, Q, ?
- Enter = Next message
- ESC = Quit

**Other Keys** (show lightbar):

- Any unrecognized key displays the lightbar
- User selects command from menu
- Maintains familiar interface

## Files Modified

### 1. `/internal/menu/message_lightbar.go`

**Added:**

- `readKeySequence()` function for escape sequence handling

### 2. `/internal/menu/message_reader.go`

**Modified:**

- `runMessageReader()` function completely refactored
- Removed pagination/"More" prompt logic
- Added inner `scrollLoop` for scrolling
- Added scroll offset tracking
- Implemented explicit cursor positioning
- Added scroll percentage calculation
- Preserved all command handling logic

## Usage

### For Users

1. Read messages as before using the menu system
2. Use arrow keys to scroll up/down one line
3. Use Page Up/Down to scroll faster
4. Press any command key (N, R, etc.) to execute commands
5. Watch the percentage indicator for long messages

### For Developers

The implementation maintains backward compatibility while adding new functionality. No configuration changes needed - it "just works" with existing Vision3 installations.

## Testing Recommendations

1. **Various Terminal Heights:** Test with 24, 25, 43, 50 line terminals
2. **Message Lengths:** Short messages (< screen height), long messages (> screen height)
3. **Scrolling:** Up/down arrows, page up/down, boundaries (top/bottom)
4. **Commands:** All 10 lightbar options (N, R, A, S, T, P, J, M, L, Q)
5. **Thread Navigation:** Forward/backward thread searching
6. **EchoMail/NetMail:** Origin line display
7. **NewScan Mode:** Scroll percentage display in newscan

## Future Enhancements (Optional)

1. **Smooth Scrolling Indicators**
   - Add up/down arrows in footer when not at top/bottom
   - Visual cue for scrollable content

2. **Jump to Top/Bottom**
   - Home key = jump to top
   - End key = jump to bottom

3. **Search Within Message**
   - Find text within current message
   - Highlight matches

4. **Quote Mode Selection**
   - Arrow keys to select lines to quote when replying
   - Multi-line quote selection

## Conclusion

Successfully implemented a modern scrollable message reader while preserving Vision3's classic BBS features. Users get a professional, fluid reading experience without losing any existing functionality.
