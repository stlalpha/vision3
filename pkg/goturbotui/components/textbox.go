package components

import (
	"strings"
	"unicode"
	"unicode/utf8"
	
	"github.com/stlalpha/vision3/pkg/goturbotui"
)

// TextBox represents a single-line text input field
type TextBox struct {
	*goturbotui.BaseView
	text         []rune        // Text content as runes for proper Unicode handling
	cursorPos    int           // Cursor position in runes
	scrollOffset int           // Horizontal scroll offset for long text
	maxLength    int           // Maximum text length (0 = unlimited)
	placeholder  string        // Placeholder text when empty
	passwordMode bool          // Whether to mask input with asterisks
	readOnly     bool          // Whether the field is read-only
	multiline    bool          // Whether to support multiline input
	theme        *goturbotui.Theme
	
	// Selection support
	selectionStart int         // Start of selection (-1 if no selection)
	selectionEnd   int         // End of selection (-1 if no selection)
	
	// Validation
	validator    func(string) bool // Validation function
	
	// Events
	onChange     func(string)     // Called when text changes
	onEnter      func(string)     // Called when Enter is pressed
	onEscape     func()           // Called when Escape is pressed
}

// NewTextBox creates a new text box with the specified theme
func NewTextBox(theme *goturbotui.Theme) *TextBox {
	textBox := &TextBox{
		BaseView:       goturbotui.NewBaseView(),
		text:           make([]rune, 0),
		cursorPos:      0,
		scrollOffset:   0,
		maxLength:      0,
		placeholder:    "",
		passwordMode:   false,
		readOnly:       false,
		multiline:      false,
		theme:          theme,
		selectionStart: -1,
		selectionEnd:   -1,
	}
	
	textBox.SetCanFocus(true)
	return textBox
}

// SetText sets the text content
func (tb *TextBox) SetText(text string) {
	tb.text = []rune(text)
	
	// Validate length
	if tb.maxLength > 0 && len(tb.text) > tb.maxLength {
		tb.text = tb.text[:tb.maxLength]
	}
	
	// Update cursor position to end of text
	tb.cursorPos = len(tb.text)
	tb.clearSelection()
	tb.ensureCursorVisible()
	
	// Trigger change event
	if tb.onChange != nil {
		tb.onChange(string(tb.text))
	}
}

// GetText returns the current text content
func (tb *TextBox) GetText() string {
	return string(tb.text)
}

// SetMaxLength sets the maximum allowed text length (0 = unlimited)
func (tb *TextBox) SetMaxLength(maxLength int) {
	tb.maxLength = maxLength
	
	// Truncate existing text if necessary
	if tb.maxLength > 0 && len(tb.text) > tb.maxLength {
		tb.text = tb.text[:tb.maxLength]
		if tb.cursorPos > len(tb.text) {
			tb.cursorPos = len(tb.text)
		}
		tb.clearSelection()
		tb.ensureCursorVisible()
	}
}

// SetPasswordMode enables or disables password mode (asterisk masking)
func (tb *TextBox) SetPasswordMode(passwordMode bool) {
	tb.passwordMode = passwordMode
}

// SetPlaceholder sets the placeholder text shown when the field is empty
func (tb *TextBox) SetPlaceholder(placeholder string) {
	tb.placeholder = placeholder
}

// SetReadOnly sets whether the field is read-only
func (tb *TextBox) SetReadOnly(readOnly bool) {
	tb.readOnly = readOnly
}

// SetMultiline enables or disables multiline support
func (tb *TextBox) SetMultiline(multiline bool) {
	tb.multiline = multiline
}

// SetValidator sets the input validation function
func (tb *TextBox) SetValidator(validator func(string) bool) {
	tb.validator = validator
}

// SetOnChange sets the text change callback
func (tb *TextBox) SetOnChange(callback func(string)) {
	tb.onChange = callback
}

// SetOnEnter sets the Enter key callback
func (tb *TextBox) SetOnEnter(callback func(string)) {
	tb.onEnter = callback
}

// SetOnEscape sets the Escape key callback
func (tb *TextBox) SetOnEscape(callback func()) {
	tb.onEscape = callback
}

// IsEmpty returns whether the text box is empty
func (tb *TextBox) IsEmpty() bool {
	return len(tb.text) == 0
}

// Clear clears all text
func (tb *TextBox) Clear() {
	tb.text = tb.text[:0]
	tb.cursorPos = 0
	tb.scrollOffset = 0
	tb.clearSelection()
	
	if tb.onChange != nil {
		tb.onChange("")
	}
}

// SelectAll selects all text
func (tb *TextBox) SelectAll() {
	if len(tb.text) > 0 {
		tb.selectionStart = 0
		tb.selectionEnd = len(tb.text)
	}
}

// HasSelection returns whether there is an active selection
func (tb *TextBox) HasSelection() bool {
	return tb.selectionStart >= 0 && tb.selectionEnd >= 0 && tb.selectionStart != tb.selectionEnd
}

// GetSelectedText returns the currently selected text
func (tb *TextBox) GetSelectedText() string {
	if !tb.HasSelection() {
		return ""
	}
	
	start := tb.selectionStart
	end := tb.selectionEnd
	if start > end {
		start, end = end, start
	}
	
	return string(tb.text[start:end])
}

// clearSelection clears the current selection
func (tb *TextBox) clearSelection() {
	tb.selectionStart = -1
	tb.selectionEnd = -1
}

// deleteSelection deletes the currently selected text
func (tb *TextBox) deleteSelection() bool {
	if !tb.HasSelection() {
		return false
	}
	
	start := tb.selectionStart
	end := tb.selectionEnd
	if start > end {
		start, end = end, start
	}
	
	// Delete the selected text
	tb.text = append(tb.text[:start], tb.text[end:]...)
	tb.cursorPos = start
	tb.clearSelection()
	tb.ensureCursorVisible()
	
	if tb.onChange != nil {
		tb.onChange(string(tb.text))
	}
	
	return true
}

// insertChar inserts a character at the current cursor position
func (tb *TextBox) insertChar(char rune) {
	if tb.readOnly {
		return
	}
	
	// Delete selection if any
	tb.deleteSelection()
	
	// Check if character is printable
	if !unicode.IsPrint(char) {
		return
	}
	
	// Check length limit
	if tb.maxLength > 0 && len(tb.text) >= tb.maxLength {
		return
	}
	
	// Insert character
	tb.text = append(tb.text[:tb.cursorPos], append([]rune{char}, tb.text[tb.cursorPos:]...)...)
	tb.cursorPos++
	tb.ensureCursorVisible()
	
	// Validate if validator is set
	if tb.validator != nil && !tb.validator(string(tb.text)) {
		// Revert the change
		tb.text = append(tb.text[:tb.cursorPos-1], tb.text[tb.cursorPos:]...)
		tb.cursorPos--
		return
	}
	
	if tb.onChange != nil {
		tb.onChange(string(tb.text))
	}
}

// deleteChar deletes the character at the specified position
func (tb *TextBox) deleteChar(pos int) {
	if tb.readOnly || pos < 0 || pos >= len(tb.text) {
		return
	}
	
	tb.text = append(tb.text[:pos], tb.text[pos+1:]...)
	
	if tb.cursorPos > pos {
		tb.cursorPos--
	} else if tb.cursorPos > len(tb.text) {
		tb.cursorPos = len(tb.text)
	}
	
	tb.clearSelection()
	tb.ensureCursorVisible()
	
	if tb.onChange != nil {
		tb.onChange(string(tb.text))
	}
}

// moveCursor moves the cursor to the specified position
func (tb *TextBox) moveCursor(pos int) {
	if pos < 0 {
		pos = 0
	} else if pos > len(tb.text) {
		pos = len(tb.text)
	}
	
	tb.cursorPos = pos
	tb.ensureCursorVisible()
}

// ensureCursorVisible adjusts scroll offset to keep cursor visible
func (tb *TextBox) ensureCursorVisible() {
	bounds := tb.GetBounds()
	visibleWidth := bounds.W
	
	if visibleWidth <= 0 {
		return
	}
	
	// Adjust scroll offset to keep cursor visible
	if tb.cursorPos < tb.scrollOffset {
		tb.scrollOffset = tb.cursorPos
	} else if tb.cursorPos >= tb.scrollOffset+visibleWidth {
		tb.scrollOffset = tb.cursorPos - visibleWidth + 1
	}
	
	// Ensure scroll offset is valid
	if tb.scrollOffset < 0 {
		tb.scrollOffset = 0
	}
	maxScroll := len(tb.text) - visibleWidth + 1
	if maxScroll < 0 {
		maxScroll = 0
	}
	if tb.scrollOffset > maxScroll {
		tb.scrollOffset = maxScroll
	}
}

// getDisplayText returns the text to display (with masking for password mode)
func (tb *TextBox) getDisplayText() string {
	if tb.passwordMode {
		return strings.Repeat("*", len(tb.text))
	}
	return string(tb.text)
}

// Draw renders the text box
func (tb *TextBox) Draw(canvas goturbotui.Canvas) {
	if !tb.IsVisible() {
		return
	}
	
	bounds := tb.GetBounds()
	theme := tb.theme
	if theme == nil {
		theme = goturbotui.DefaultTurboTheme()
	}
	
	// Choose style based on focus and state
	var style goturbotui.Style
	if tb.IsFocused() {
		style = theme.InputFocused
	} else {
		style = theme.Input
	}
	
	// Clear background
	canvas.Fill(bounds, ' ', style)
	
	// Get display text
	displayText := tb.getDisplayText()
	
	// Show placeholder if empty and not focused
	if len(tb.text) == 0 && !tb.IsFocused() && tb.placeholder != "" {
		// Truncate placeholder if too long
		placeholder := tb.placeholder
		if utf8.RuneCountInString(placeholder) > bounds.W {
			placeholder = string([]rune(placeholder)[:bounds.W])
		}
		
		// Use a dimmed style for placeholder
		placeholderStyle := style.WithForeground(goturbotui.ColorDarkGray)
		canvas.SetString(bounds.X, bounds.Y, placeholder, placeholderStyle)
		return
	}
	
	// Calculate visible portion of text
	visibleStart := tb.scrollOffset
	visibleEnd := tb.scrollOffset + bounds.W
	if visibleEnd > len(displayText) {
		visibleEnd = len(displayText)
	}
	
	var visibleText string
	if visibleStart < len(displayText) {
		visibleText = displayText[visibleStart:visibleEnd]
	}
	
	// Draw selection background if any
	if tb.HasSelection() && tb.IsFocused() {
		start := tb.selectionStart
		end := tb.selectionEnd
		if start > end {
			start, end = end, start
		}
		
		// Adjust selection to visible area
		selStart := start - tb.scrollOffset
		selEnd := end - tb.scrollOffset
		
		if selStart < 0 {
			selStart = 0
		}
		if selEnd > bounds.W {
			selEnd = bounds.W
		}
		
		// Draw selection background
		if selStart < selEnd && selStart < len(visibleText) {
			selectionStyle := theme.InputSelected
			for i := selStart; i < selEnd && i < len(visibleText); i++ {
				char := rune(visibleText[i])
				canvas.SetCell(bounds.X+i, bounds.Y, char, selectionStyle)
			}
		}
	}
	
	// Draw visible text
	if len(visibleText) > 0 {
		canvas.SetString(bounds.X, bounds.Y, visibleText, style)
	}
	
	// Draw cursor if focused
	if tb.IsFocused() {
		cursorX := tb.cursorPos - tb.scrollOffset
		if cursorX >= 0 && cursorX < bounds.W {
			// Show cursor as a reverse block or underline
			cursorChar := ' '
			if cursorX < len(visibleText) {
				cursorChar = rune(visibleText[cursorX])
			}
			cursorStyle := style.WithAttributes(goturbotui.AttrReverse)
			canvas.SetCell(bounds.X+cursorX, bounds.Y, cursorChar, cursorStyle)
		}
	}
}

// HandleEvent handles text box events
func (tb *TextBox) HandleEvent(event goturbotui.Event) bool {
	if !tb.IsVisible() || !tb.CanFocus() {
		return false
	}
	
	if event.Type == goturbotui.EventKey {
		switch event.Key.Code {
		case goturbotui.KeyLeft:
			if event.Key.Modifiers&goturbotui.ModShift != 0 {
				// Extend selection
				if !tb.HasSelection() {
					tb.selectionStart = tb.cursorPos
				}
				tb.moveCursor(tb.cursorPos - 1)
				tb.selectionEnd = tb.cursorPos
			} else {
				tb.clearSelection()
				tb.moveCursor(tb.cursorPos - 1)
			}
			return true
			
		case goturbotui.KeyRight:
			if event.Key.Modifiers&goturbotui.ModShift != 0 {
				// Extend selection
				if !tb.HasSelection() {
					tb.selectionStart = tb.cursorPos
				}
				tb.moveCursor(tb.cursorPos + 1)
				tb.selectionEnd = tb.cursorPos
			} else {
				tb.clearSelection()
				tb.moveCursor(tb.cursorPos + 1)
			}
			return true
			
		case goturbotui.KeyHome:
			if event.Key.Modifiers&goturbotui.ModShift != 0 {
				// Extend selection
				if !tb.HasSelection() {
					tb.selectionStart = tb.cursorPos
				}
				tb.moveCursor(0)
				tb.selectionEnd = tb.cursorPos
			} else {
				tb.clearSelection()
				tb.moveCursor(0)
			}
			return true
			
		case goturbotui.KeyEnd:
			if event.Key.Modifiers&goturbotui.ModShift != 0 {
				// Extend selection
				if !tb.HasSelection() {
					tb.selectionStart = tb.cursorPos
				}
				tb.moveCursor(len(tb.text))
				tb.selectionEnd = tb.cursorPos
			} else {
				tb.clearSelection()
				tb.moveCursor(len(tb.text))
			}
			return true
			
		case goturbotui.KeyBackspace:
			if tb.readOnly {
				return true
			}
			if tb.HasSelection() {
				tb.deleteSelection()
			} else if tb.cursorPos > 0 {
				tb.deleteChar(tb.cursorPos - 1)
			}
			return true
			
		case goturbotui.KeyDelete:
			if tb.readOnly {
				return true
			}
			if tb.HasSelection() {
				tb.deleteSelection()
			} else if tb.cursorPos < len(tb.text) {
				tb.deleteChar(tb.cursorPos)
			}
			return true
			
		case goturbotui.KeyEnter:
			if tb.onEnter != nil {
				tb.onEnter(string(tb.text))
			}
			return true
			
		case goturbotui.KeyEscape:
			tb.clearSelection()
			if tb.onEscape != nil {
				tb.onEscape()
			}
			return true
			
		case goturbotui.KeyTab:
			// Let parent handle tab navigation
			return false
			
		default:
			// Handle Ctrl key combinations
			if event.Key.Modifiers&goturbotui.ModCtrl != 0 {
				switch event.Rune {
				case 'a', 'A':
					tb.SelectAll()
					return true
				case 'c', 'C':
					// Copy to clipboard (would need clipboard support)
					return true
				case 'v', 'V':
					// Paste from clipboard (would need clipboard support)
					return true
				case 'x', 'X':
					// Cut to clipboard (would need clipboard support)
					if tb.HasSelection() {
						tb.deleteSelection()
					}
					return true
				}
			}
			
			// Handle printable characters
			if event.Rune != 0 && unicode.IsPrint(event.Rune) {
				tb.insertChar(event.Rune)
				return true
			}
		}
	}
	
	return false
}