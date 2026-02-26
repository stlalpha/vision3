package editor

import (
	"strings"
	"testing"
)

// helper creates a buffer with the given lines, all with soft newlines by default.
// Lines are 1-indexed. Use hardLines to mark specific lines as hard newlines.
func setupBuffer(lines []string, hardLines ...int) (*MessageBuffer, *WordWrapper) {
	mb := NewMessageBuffer()
	// Load lines via public API; SetLine updates lineCount automatically
	for i, line := range lines {
		mb.SetLine(i+1, line)
	}
	// Mark specified lines as hard newlines
	hardSet := make(map[int]bool)
	for _, h := range hardLines {
		hardSet[h] = true
	}
	for i := 1; i <= len(lines); i++ {
		mb.SetHardNewline(i, hardSet[i])
	}
	ww := NewWordWrapper(mb)
	return mb, ww
}

func bufferLines(mb *MessageBuffer) []string {
	count := mb.GetLineCount()
	result := make([]string, count)
	for i := 1; i <= count; i++ {
		result[i-1] = mb.GetLine(i)
	}
	return result
}

// --- ReflowRange Tests ---

func TestReflowRange_NoWrapNeeded(t *testing.T) {
	mb, ww := setupBuffer([]string{"short line"}, 1)
	newLine, newCol := ww.ReflowRange(1, 1, 5)
	if newLine != 1 || newCol != 5 {
		t.Errorf("expected (1,5), got (%d,%d)", newLine, newCol)
	}
	if mb.GetLine(1) != "short line" {
		t.Errorf("line should be unchanged, got %q", mb.GetLine(1))
	}
}

func TestReflowRange_BasicWrap(t *testing.T) {
	// A single line exceeding 79 chars should wrap
	long := strings.Repeat("word ", 20) // "word word word ..." = 100 chars
	long = strings.TrimRight(long, " ")
	mb, ww := setupBuffer([]string{long}, 1)

	newLine, newCol := ww.ReflowRange(1, 1, 1)
	_ = newLine
	_ = newCol

	// First line should be ≤ 79 chars
	if len(mb.GetLine(1)) > MaxLineLength {
		t.Errorf("line 1 should be ≤ %d chars, got %d", MaxLineLength, len(mb.GetLine(1)))
	}
	// Should have created a second line
	if mb.GetLineCount() < 2 {
		t.Errorf("expected at least 2 lines, got %d", mb.GetLineCount())
	}
	// Last line should inherit hardNewline from original
	lastLine := mb.GetLineCount()
	if !mb.IsHardNewline(lastLine) {
		t.Error("last line should inherit hardNewline=true from original")
	}
}

func TestReflowRange_MultiLinePullUp(t *testing.T) {
	// Two soft-wrapped lines where content can fit on one line
	mb, ww := setupBuffer([]string{"hello", "world"}) // no hard newlines

	newLine, newCol := ww.ReflowRange(1, 1, 6)
	if mb.GetLine(1) != "hello world" {
		t.Errorf("expected 'hello world', got %q", mb.GetLine(1))
	}
	if mb.GetLineCount() != 1 {
		t.Errorf("expected 1 line after merge, got %d", mb.GetLineCount())
	}
	_ = newLine
	_ = newCol
}

func TestReflowRange_StopsAtHardNewline(t *testing.T) {
	// First paragraph: lines 1-2 (line 2 has hard newline, ending paragraph 1).
	// Second paragraph: lines 3-4. Reflow of paragraph 1 must not alter
	// second paragraph content. After merge, paragraph 2 shifts up by one line.
	mb, ww := setupBuffer(
		[]string{"aaa bbb", "ccc", "ddd eee", "fff ggg"},
		2, 4, // lines 2 and 4 are hard newlines
	)

	ww.ReflowRange(1, 1, 1)

	// First paragraph should have been merged into one line.
	if got := mb.GetLine(1); got != "aaa bbb ccc" {
		t.Errorf("first paragraph should be merged, got %q", got)
	}

	// Second paragraph content must be preserved (shifted up by one line).
	if got := mb.GetLine(2); got != "ddd eee" {
		t.Errorf("second paragraph line 1 should be unchanged, got %q", got)
	}
	if got := mb.GetLine(3); got != "fff ggg" {
		t.Errorf("second paragraph line 2 should be unchanged, got %q", got)
	}
	if mb.GetLineCount() != 3 {
		t.Errorf("expected 3 total lines, got %d", mb.GetLineCount())
	}
}

func TestReflowRange_SoftParagraphReflow(t *testing.T) {
	// Three soft-wrapped lines forming one paragraph, terminated by hard newline
	mb, ww := setupBuffer([]string{"aaa bbb", "ccc ddd", "eee"})
	mb.SetHardNewline(1, false)
	mb.SetHardNewline(2, false)
	mb.SetHardNewline(3, true) // paragraph ends here

	ww.ReflowRange(1, 1, 1)

	// All text should be collected and re-wrapped
	// "aaa bbb ccc ddd eee" = 19 chars, fits in one line
	if mb.GetLine(1) != "aaa bbb ccc ddd eee" {
		t.Errorf("expected merged line, got %q", mb.GetLine(1))
	}
	if mb.GetLineCount() != 1 {
		t.Errorf("expected 1 line, got %d", mb.GetLineCount())
	}
	if !mb.IsHardNewline(1) {
		t.Error("merged line should inherit hardNewline=true from last original line")
	}
}

func TestReflowRange_CursorTracking(t *testing.T) {
	// Cursor is in the middle of text that gets reflowed
	// "The quick brown" + "fox" → merged to "The quick brown fox"
	mb, ww := setupBuffer([]string{"The quick brown", "fox"})
	mb.SetHardNewline(2, true)

	// Cursor on "fox" at col 2 (the 'o')
	newLine, newCol := ww.ReflowRange(1, 2, 2)

	// After merge: "The quick brown fox", 'o' is at col 18
	if newLine != 1 {
		t.Errorf("expected cursor on line 1, got %d", newLine)
	}
	if newCol != 18 {
		t.Errorf("expected cursor at col 18, got %d", newCol)
	}
}

func TestReflowRange_CursorAtEnd(t *testing.T) {
	mb, ww := setupBuffer([]string{"hello", "world"})
	mb.SetHardNewline(2, true)

	// Cursor past end of "world" (col 6)
	newLine, newCol := ww.ReflowRange(1, 2, 6)

	// After merge: "hello world" (11 chars), cursor at col 12 (past end)
	if newLine != 1 || newCol != 12 {
		t.Errorf("expected (1,12), got (%d,%d)", newLine, newCol)
	}
}

func TestReflowRange_ForceBreak(t *testing.T) {
	// A word longer than MaxLineLength should force-break
	longWord := strings.Repeat("x", MaxLineLength+10)
	mb, ww := setupBuffer([]string{longWord}, 1)

	ww.ReflowRange(1, 1, 1)

	if len(mb.GetLine(1)) > MaxLineLength {
		t.Errorf("line 1 should be ≤ %d after force break, got %d", MaxLineLength, len(mb.GetLine(1)))
	}
	if mb.GetLineCount() < 2 {
		t.Error("expected overflow to create line 2")
	}
}

// --- WrapAfterInsert Tests ---

func TestWrapAfterInsert_NoWrapNeeded(t *testing.T) {
	mb, ww := setupBuffer([]string{"short"}, 1)
	newLine, newCol := ww.WrapAfterInsert(1, 6)
	if newLine != 1 || newCol != 6 {
		t.Errorf("expected (1,6), got (%d,%d)", newLine, newCol)
	}
	if mb.GetLine(1) != "short" {
		t.Errorf("line should be unchanged, got %q", mb.GetLine(1))
	}
}

func TestWrapAfterInsert_WrapsLongLine(t *testing.T) {
	// Build a line just over MaxLineLength
	line := strings.Repeat("a ", 40) // 80 chars
	line = strings.TrimRight(line, " ")
	mb, ww := setupBuffer([]string{line}, 1)

	ww.WrapAfterInsert(1, len(line)+1)

	if len(mb.GetLine(1)) > MaxLineLength {
		t.Errorf("line 1 should be wrapped, got len=%d", len(mb.GetLine(1)))
	}
}

// --- HandleBackspace Tests ---

func TestHandleBackspace_MidLine(t *testing.T) {
	// Delete char mid-line, reflow pulls words up
	mb, ww := setupBuffer([]string{"hello world", "next line"})
	mb.SetHardNewline(2, true)

	newLine, newCol, changed := ww.HandleBackspace(1, 6) // delete 'o' of "hello"
	if !changed {
		t.Error("expected change")
	}
	// "hell world" + "next line" → "hell world next line" (20 chars, fits)
	if mb.GetLine(1) != "hell world next line" {
		t.Errorf("expected merged 'hell world next line', got %q", mb.GetLine(1))
	}
	if newLine != 1 || newCol != 5 {
		t.Errorf("expected cursor at (1,5), got (%d,%d)", newLine, newCol)
	}
}

func TestHandleBackspace_StartOfLine_JoinsAndReflows(t *testing.T) {
	mb, ww := setupBuffer([]string{"hello", "world"})
	mb.SetHardNewline(1, true)
	mb.SetHardNewline(2, true)

	newLine, newCol, changed := ww.HandleBackspace(2, 1) // at start of line 2
	if !changed {
		t.Error("expected change")
	}
	if mb.GetLine(1) != "helloworld" {
		t.Errorf("expected 'helloworld', got %q", mb.GetLine(1))
	}
	if newLine != 1 || newCol != 6 {
		t.Errorf("expected cursor at (1,6), got (%d,%d)", newLine, newCol)
	}
}

func TestHandleBackspace_PullsUpFromSoftLine(t *testing.T) {
	// Backspace mid-line where next line is soft-wrapped continuation
	mb, ww := setupBuffer([]string{
		strings.Repeat("a", 78), // 78 chars
		"bb cc",                 // soft continuation
	})
	mb.SetHardNewline(1, false)
	mb.SetHardNewline(2, true)

	// Delete one char from line 1 → now 77 chars → "bb" can pull up
	newLine, newCol, changed := ww.HandleBackspace(1, 40)
	if !changed {
		t.Error("expected change")
	}
	_ = newLine
	_ = newCol

	// Line 1 should now contain more text (some pulled up from line 2)
	line1 := mb.GetLine(1)
	if len(line1) > MaxLineLength {
		t.Errorf("line 1 should be ≤ %d, got %d", MaxLineLength, len(line1))
	}
}

// --- HandleDelete Tests ---

func TestHandleDelete_MidLine(t *testing.T) {
	mb, ww := setupBuffer([]string{"hello world", "foo"})
	mb.SetHardNewline(2, true)

	newLine, newCol, changed := ww.HandleDelete(1, 6) // delete space in "hello world"
	if !changed {
		t.Error("expected change")
	}
	// "helloworld" + "foo" → "helloworld foo" (14 chars)
	if mb.GetLine(1) != "helloworld foo" {
		t.Errorf("expected 'helloworld foo', got %q", mb.GetLine(1))
	}
	if newLine != 1 || newCol != 6 {
		t.Errorf("expected cursor at (1,6), got (%d,%d)", newLine, newCol)
	}
}

func TestHandleDelete_EndOfLine_JoinsWithNext(t *testing.T) {
	mb, ww := setupBuffer([]string{"hello", "world"})
	mb.SetHardNewline(1, true)
	mb.SetHardNewline(2, true)

	newLine, newCol, changed := ww.HandleDelete(1, 6) // at end of "hello"
	if !changed {
		t.Error("expected change")
	}
	if mb.GetLine(1) != "helloworld" {
		t.Errorf("expected 'helloworld', got %q", mb.GetLine(1))
	}
	if newLine != 1 || newCol != 6 {
		t.Errorf("expected cursor at (1,6), got (%d,%d)", newLine, newCol)
	}
}

// --- DeleteWord Tests ---

func TestDeleteWord_TriggersReflow(t *testing.T) {
	mb, ww := setupBuffer([]string{"hello big world", "foo bar"})
	mb.SetHardNewline(2, true)

	newLine, newCol, changed := ww.DeleteWord(1, 7) // delete "big" at col 7
	if !changed {
		t.Error("expected change")
	}
	_ = newLine
	_ = newCol

	// "hello  world" + "foo bar" should be reflowed
	// Text after delete: "hello world" → pulled "foo bar" up if it fits
	line1 := mb.GetLine(1)
	if len(line1) > MaxLineLength {
		t.Errorf("line 1 should be ≤ %d, got %d", MaxLineLength, len(line1))
	}
}

// --- ReformatParagraph Tests ---

func TestReformatParagraph_RespectsHardNewlines(t *testing.T) {
	mb, ww := setupBuffer([]string{
		"short line one",
		"short line two",
		"separate paragraph",
	})
	mb.SetHardNewline(1, false)
	mb.SetHardNewline(2, true) // end of first paragraph
	mb.SetHardNewline(3, true) // separate paragraph

	ww.ReformatParagraph(1)

	// Lines 1-2 should be merged (they're one soft paragraph ending at hard newline on 2)
	if mb.GetLine(1) != "short line one short line two" {
		t.Errorf("expected merged paragraph, got %q", mb.GetLine(1))
	}
	// Line 3 (now line 2 after merge) should be unchanged
	line2 := mb.GetLine(2)
	if line2 != "separate paragraph" {
		t.Errorf("separate paragraph should be unchanged, got %q", line2)
	}
}

func TestReformatParagraph_EmptyLine(t *testing.T) {
	_, ww := setupBuffer([]string{""}, 1)
	result := ww.ReformatParagraph(1)
	if result != 1 {
		t.Errorf("expected 1 for empty line, got %d", result)
	}
}

// --- Word Navigation Tests ---

func TestFindWordLeft(t *testing.T) {
	_, ww := setupBuffer([]string{"hello world test"})
	tests := []struct {
		col  int
		want int
	}{
		{16, 13}, // from end → start of "test"
		{12, 7},  // from space before "test" → start of "world"
		{6, 1},   // from space before "world" → start of "hello"
		{1, 1},   // already at start
	}
	for _, tt := range tests {
		got := ww.FindWordLeft(1, tt.col)
		if got != tt.want {
			t.Errorf("FindWordLeft(1, %d) = %d, want %d", tt.col, got, tt.want)
		}
	}
}

func TestFindWordRight(t *testing.T) {
	_, ww := setupBuffer([]string{"hello world test"})
	tests := []struct {
		col  int
		want int
	}{
		{1, 7},  // from start → start of "world"
		{7, 13}, // from "world" → start of "test"
		{13, 17}, // from "test" → past end
	}
	for _, tt := range tests {
		got := ww.FindWordRight(1, tt.col)
		if got != tt.want {
			t.Errorf("FindWordRight(1, %d) = %d, want %d", tt.col, got, tt.want)
		}
	}
}
