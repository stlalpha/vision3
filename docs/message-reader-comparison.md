# Message Reader Comparison: Vision3 vs Retrograde

## Overview
This document compares the message reading implementations in Vision3 and Retrograde, identifying key improvements to adopt from Retrograde's full-screen reader.

## Current Vision3 Implementation

### Display Flow
1. **Header Rendering**: Uses MSGHDR templates with DataFile substitution (`%B`, `%T`, `%F`, etc.)
2. **Body Display**: 
   - Wraps text to terminal width
   - Displays line-by-line with pagination
   - Shows "More" prompt when screen fills
   - User presses Q to quit or Enter to continue
3. **Navigation Bar**: 10-option lightbar menu appears at bottom after message display
4. **Commands**: Single-key navigation (N, R, A, S, T, P, J, M, L, Q)

### Key Characteristics
- **Pagination-based**: User sees message in chunks, must press Enter to continue
- **No scrolling**: Cannot scroll back up to re-read content
- **Menu after display**: Commands shown after message is fully rendered
- **Sequential flow**: Read → Pause → Continue → Command

### Code Location
- Main reader: `/home/bbs/git/vision3/internal/menu/message_reader.go`
- Function: `runMessageReader()`
- Lines 140-180: Body display with pagination logic

```go
// Display message body with pagination
linesDisplayed := 0
quitReading := false
for lineIdx, line := range wrappedBodyLines {
    terminalio.WriteProcessedBytes(terminal, []byte("\r\n"+line), outputMode)
    linesDisplayed++

    // Check if pause is needed
    if linesDisplayed >= bodyAvailHeight && lineIdx < len(wrappedBodyLines)-1 {
        pausePrompt := e.LoadedStrings.PauseString
        // Show pause prompt...
    }
}
```

## Retrograde's Full-Screen Implementation

### Display Architecture
1. **Fixed Header Region** (6-10 lines):
   - Template-based with MCI codes (`@MU`, `@MB`, `@MS`, etc.)
   - Loaded from `read.template.ans`
   - Falls back to CP437 box-drawing header if unavailable
   - Never redraws during scrolling

2. **Scrollable Body Region**:
   - All message lines pre-formatted and stored in array
   - Visible window shows subset based on scroll offset
   - Calculated as: `messageAreaHeight = termHeight - headerHeight - 2`
   - Minimum 5 lines enforced

3. **Fixed Footer Region** (2 lines):
   - Line 1: CP437 horizontal line separator
   - Line 2: Command prompt with conference/area info + scroll percentage
   - Shows `(?=Help)` right-aligned
   - Always anchored to bottom row

### Scrolling Behavior
```go
// Scroll offset tracking
case "\x1b[A": // Up arrow - scroll up one line
    if scrollOffset > 0 {
        scrollOffset--
        redrawMessageBodyWithFooter(...)
    }

case "\x1b[B": // Down arrow - scroll down one line
    if totalLines > messageAreaHeight && scrollOffset < totalLines-messageAreaHeight {
        scrollOffset++
        redrawMessageBodyWithFooter(...)
    }

case "\x1b[5~": // Page Up
    scrollOffset -= (termHeight - 5)
    if scrollOffset < 0 { scrollOffset = 0 }
    redrawMessageBodyWithFooter(...)
```

### Efficient Redrawing
- **Full Redraw**: Only on new message or toggle kludges
- **Partial Redraw**: Only message body during scrolling
- **Footer Updates**: Scroll percentage recalculated on each scroll

```go
// Only redraws body lines - header and footer untouched
func redrawMessageBody(io, formattedLines, scrollOffset, messageAreaHeight, headerHeight) {
    for i := 0; i < messageAreaHeight; i++ {
        lineNum := headerHeight + 1 + i
        io.Printf("\x1b[%d;1H", lineNum)  // Position cursor
        io.Print("\x1b[K")                 // Clear line
        if i < len(visibleLines) {
            io.Print(visibleLines[i])      // Print content
        }
    }
}
```

### Key Features
- **Smooth scrolling**: Arrow keys and Page Up/Down
- **Always-visible commands**: Footer shows available commands
- **Scroll indicator**: Shows percentage when message extends beyond screen
- **No interruptions**: Read entire message without pauses
- **Jump navigation**: Can jump to any line instantly
- **Explicit positioning**: Avoids 80-column auto-wrap issues

### Code Location
- Main handler: `/home/bbs/git/retrograde/internal/menu/cmdkeys_message_read.go`
- UI functions: `/home/bbs/git/retrograde/internal/menu/cmdkeys_message_ui.go`
- Key functions:
  - `handleReadMessages()`: Main reading loop (line 15-400)
  - `displayFullScreenMessage()`: Renders message (line 778-1025)
  - `redrawMessageBodyWithFooter()`: Efficient scroll redraw (line 1064-1070)
  - `drawMessageFooterWithScroll()`: Footer with scroll % (line 1079-1150)
  - `formatMessageLines()`: Pre-formats all lines (line 1178-1280)

## Comparison Matrix

| Feature                | Vision3 (Current)       | Retrograde (Target)            |
| ---------------------- | ----------------------- | ------------------------------ |
| **Display Model**      | Sequential pagination   | Fixed regions with scrolling   |
| **Scrolling**          | ❌ None                  | ✅ Arrow keys, PgUp/PgDn        |
| **Header**             | DataFile templates      | MCI code templates + fallback  |
| **Body Area**          | Variable (fills screen) | Calculated fixed region        |
| **Footer**             | Lightbar after display  | Always anchored at bottom      |
| **More Prompts**       | ✅ Yes (interrupts flow) | ❌ Never (smooth experience)    |
| **Scroll Indicator**   | ❌ None                  | ✅ Percentage on long messages  |
| **Command Visibility** | After reading only      | Always visible in footer       |
| **Redraw Efficiency**  | Full screen each time   | Partial (body only)            |
| **80-col Handling**    | Standard output         | Explicit cursor positioning    |
| **Line Formatting**    | On-the-fly              | Pre-formatted array            |
| **Terminal Height**    | PTY detection           | PTY + user preference override |

## Required Changes for Vision3

### 1. **Replace Pagination with Scrolling**
- Remove the "More" prompt system (lines 140-180 in message_reader.go)
- Pre-format all message lines before display
- Track scroll offset and visible window
- Add arrow key and Page Up/Down handling

### 2. **Implement Three-Region Layout**
**Header Region:**
- Already have template system (MSGHDR files)
- Keep existing substitution logic
- Calculate header height using `findHeaderEndRow()`

**Body Region:**
- Calculate: `messageAreaHeight = termHeight - headerHeight - 2`
- Use explicit cursor positioning for each line
- Format: `io.Printf("\x1b[%d;1H", lineNum)`

**Footer Region:**
- Create `drawMessageFooter()` function
- Always position at `termHeight - 1` and `termHeight`
- Show: horizontal line + command prompt + scroll %

### 3. **Add Scroll Management**
```go
// Add to message reader loop
scrollOffset := 0
needsFullRedraw := true

for {
    if needsFullRedraw {
        // Full redraw - header + body + footer
        displayFullMessage(...)
        needsFullRedraw = false
    }
    
    // Read key sequence
    seq := readKeySequence(...)
    
    switch seq {
    case "\x1b[A": // Up arrow
        if scrollOffset > 0 {
            scrollOffset--
            redrawBodyOnly(...)
        }
    case "\x1b[B": // Down arrow
        if can scroll down {
            scrollOffset++
            redrawBodyOnly(...)
        }
    case "\x1b[5~": // Page Up
        scrollOffset -= pageSize
        redrawBodyOnly(...)
    case "\x1b[6~": // Page Down
        scrollOffset += pageSize
        redrawBodyOnly(...)
    case 'N', '\r': // Next message
        currentMsgNum++
        needsFullRedraw = true
    }
}
```

### 4. **Efficient Redraw Functions**
Create separate functions:
- `displayFullMessage()` - Complete render (header + body + footer)
- `redrawBodyOnly()` - Only update body lines
- `redrawBodyWithFooter()` - Update body + footer (for scroll %)
- `drawFooter()` - Footer with optional scroll percentage

### 5. **Key Sequence Handling**
Change from single-key lightbar to escape sequence handling:
- Replace `readSingleKey()` with `readKeySequence()` 
- Handle multi-byte escape sequences (arrow keys, Page Up/Down)
- Keep single-key commands (N, R, P, Q, etc.)

### 6. **Explicit Cursor Positioning**
Avoid relying on sequential output:
```go
// Instead of:
io.Print("\r\n" + line)

// Use:
io.Printf("\x1b[%d;1H", lineNum)  // Position cursor
io.Print("\x1b[K")                 // Clear line
io.Print(line)                     // Print content
```

### 7. **Pre-format All Lines**
Instead of displaying line-by-line:
```go
// Format all lines first
formattedLines := []string{}
for _, line := range bodyLines {
    wrapped := wrapLine(line, termWidth)
    formattedLines = append(formattedLines, wrapped...)
}

// Then display window based on scroll offset
visibleLines := formattedLines[scrollOffset:scrollOffset+messageAreaHeight]
for i, line := range visibleLines {
    displayLineAt(headerHeight + 1 + i, line)
}
```

## Implementation Strategy

### Phase 1: Add Scrolling Infrastructure (Non-Breaking)
1. Add scroll offset tracking
2. Pre-format message lines into array
3. Calculate three regions (header/body/footer)
4. Implement redraw functions

### Phase 2: Replace Display Logic
1. Remove pagination/"More" prompts
2. Implement full-screen display
3. Add footer rendering
4. Update key handling for arrow keys

### Phase 3: Optimization
1. Efficient partial redraws
2. Scroll percentage calculation
3. Handle edge cases (small terminals, long messages)

### Phase 4: Testing
1. Test with various terminal heights (24, 25, 43, 50)
2. Test with very long messages
3. Test scrolling performance
4. Test command integration (reply, jump, etc.)

## Benefits of Adopting Retrograde's Approach

1. **Better UX**: No interruptions, fluid reading experience
2. **More Powerful**: Can scroll back up to re-read content
3. **Clear Navigation**: Always see available commands
4. **Visual Feedback**: Scroll percentage shows position in long messages
5. **Efficient**: Only redraws what changed
6. **Professional**: Matches modern terminal application expectations
7. **Accessible**: Users control pace without forced pauses

## Technical Debt Considerations

### Keep from Vision3
- MSGHDR template system (works well with substitution)
- 14 pre-made header styles
- Thread searching
- Message area management

### Adopt from Retrograde
- Full-screen layout architecture
- Scrolling implementation
- Footer design and anchoring
- Efficient redraw patterns
- Explicit cursor positioning
- Pre-formatting approach

### Hybrid Approach
Consider supporting both modes via configuration:
```json
{
  "message_reader_mode": "fullscreen",  // or "paginated"
  "reader_scroll_enabled": true,
  "reader_show_footer": true
}
```

This allows users to choose their preferred reading style while maintaining backward compatibility.

## Related Files

### Vision3
- `/internal/menu/message_reader.go` - Main reader implementation
- `/internal/menu/message_light_bar.go` - Lightbar navigation
- `/internal/message/manager.go` - Message management

### Retrograde
- `/internal/menu/cmdkeys_message_read.go` - Reader with scrolling
- `/internal/menu/cmdkeys_message_ui.go` - UI helper functions
- `/internal/ui/ansi.go` - ANSI helper functions
- `/internal/mci/processor.go` - MCI code processing

## Next Steps

1. Review this comparison with the development team
2. Decide on implementation approach (full replacement vs. hybrid)
3. Create implementation tasks with priorities
4. Plan migration path for existing users
5. Update documentation and help screens
