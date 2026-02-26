package editor

import (
	"testing"
)

func TestMessageBuffer_HardNewline_Accessors(t *testing.T) {
	mb := NewMessageBuffer()

	// Default is false
	if mb.IsHardNewline(1) {
		t.Error("expected default hardNewline to be false")
	}

	// Set and get
	mb.SetHardNewline(1, true)
	if !mb.IsHardNewline(1) {
		t.Error("expected hardNewline to be true after set")
	}

	// Clear
	mb.SetHardNewline(1, false)
	if mb.IsHardNewline(1) {
		t.Error("expected hardNewline to be false after clear")
	}

	// Out of bounds
	if mb.IsHardNewline(0) {
		t.Error("expected false for out-of-bounds line 0")
	}
	if mb.IsHardNewline(MaxLines + 1) {
		t.Error("expected false for out-of-bounds line > MaxLines")
	}
}

func TestMessageBuffer_LoadContent_SetsHardNewlines(t *testing.T) {
	mb := NewMessageBuffer()
	mb.LoadContent("line one\nline two\nline three")

	if mb.GetLineCount() != 3 {
		t.Fatalf("expected 3 lines, got %d", mb.GetLineCount())
	}

	for i := 1; i <= 3; i++ {
		if !mb.IsHardNewline(i) {
			t.Errorf("expected hardNewline=true for loaded line %d", i)
		}
	}
}

func TestMessageBuffer_InsertLine_ShiftsHardNewline(t *testing.T) {
	mb := NewMessageBuffer()
	mb.SetLine(1, "first")
	mb.SetHardNewline(1, true)
	mb.SetLine(2, "second")
	mb.SetHardNewline(2, true)

	// Insert at position 2 — should shift "second" to line 3
	mb.InsertLine(2)

	if mb.IsHardNewline(2) {
		t.Error("newly inserted line should have hardNewline=false")
	}
	if !mb.IsHardNewline(1) {
		t.Error("line 1 hardNewline should be unchanged (true)")
	}
	if !mb.IsHardNewline(3) {
		t.Error("line 3 (shifted from 2) should have hardNewline=true")
	}
}

func TestMessageBuffer_DeleteLine_ShiftsHardNewline(t *testing.T) {
	mb := NewMessageBuffer()
	mb.SetLine(1, "first")
	mb.SetHardNewline(1, true)
	mb.SetLine(2, "second")
	mb.SetHardNewline(2, false)
	mb.SetLine(3, "third")
	mb.SetHardNewline(3, true)

	// Delete line 2 — line 3 shifts up to line 2
	mb.DeleteLine(2)

	if !mb.IsHardNewline(1) {
		t.Error("line 1 should still be hardNewline=true")
	}
	if !mb.IsHardNewline(2) {
		t.Error("line 2 (was line 3) should have hardNewline=true")
	}
	if mb.GetLine(2) != "third" {
		t.Errorf("line 2 content should be 'third', got '%s'", mb.GetLine(2))
	}
}

func TestMessageBuffer_SplitLine_HardNewlineInheritance(t *testing.T) {
	mb := NewMessageBuffer()
	mb.SetLine(1, "hello world")
	mb.SetHardNewline(1, true)

	mb.SplitLine(1, 6) // split at "hello" | " world"

	if mb.IsHardNewline(1) {
		t.Error("split line (top half) should have hardNewline=false (caller sets it)")
	}
	if !mb.IsHardNewline(2) {
		t.Error("new line (bottom half) should inherit original hardNewline=true")
	}
	if mb.GetLine(1) != "hello" {
		t.Errorf("line 1 should be 'hello', got '%s'", mb.GetLine(1))
	}
	if mb.GetLine(2) != " world" {
		t.Errorf("line 2 should be ' world', got '%s'", mb.GetLine(2))
	}
}

func TestMessageBuffer_SplitLine_SoftLineInheritance(t *testing.T) {
	mb := NewMessageBuffer()
	mb.SetLine(1, "hello world")
	mb.SetHardNewline(1, false) // soft-wrapped line

	mb.SplitLine(1, 6)

	if mb.IsHardNewline(1) {
		t.Error("split line should have hardNewline=false")
	}
	if mb.IsHardNewline(2) {
		t.Error("new line should inherit original hardNewline=false")
	}
}

func TestMessageBuffer_JoinLines_HardNewlineInheritance(t *testing.T) {
	mb := NewMessageBuffer()
	mb.SetLine(1, "hello")
	mb.SetHardNewline(1, true)
	mb.SetLine(2, " world")
	mb.SetHardNewline(2, true)

	mb.JoinLines(1)

	if mb.GetLine(1) != "hello world" {
		t.Errorf("joined line should be 'hello world', got '%s'", mb.GetLine(1))
	}
	// Combined line inherits hardNewline from line 2 (the second line)
	if !mb.IsHardNewline(1) {
		t.Error("joined line should inherit hardNewline=true from second line")
	}
}

func TestMessageBuffer_JoinLines_InheritsSoftNewline(t *testing.T) {
	mb := NewMessageBuffer()
	mb.SetLine(1, "hello")
	mb.SetHardNewline(1, true)
	mb.SetLine(2, " world")
	mb.SetHardNewline(2, false) // second line is soft-wrapped

	mb.JoinLines(1)

	if mb.IsHardNewline(1) {
		t.Error("joined line should inherit hardNewline=false from second line")
	}
}

func TestMessageBuffer_GetLineCount_IncludesTrailingBlanks(t *testing.T) {
	mb := NewMessageBuffer()
	mb.SetLine(1, "content")
	mb.SetLine(2, "")
	mb.SetLine(3, "")

	count := mb.GetLineCount()
	if count != 3 {
		t.Errorf("GetLineCount should include trailing blanks, got %d, want 3", count)
	}
}

func TestMessageBuffer_GetContentLineCount_TrimsTrailingBlanks(t *testing.T) {
	mb := NewMessageBuffer()
	mb.SetLine(1, "content")
	mb.SetLine(2, "")
	mb.SetLine(3, "")

	count := mb.GetContentLineCount()
	if count != 1 {
		t.Errorf("GetContentLineCount should trim trailing blanks, got %d, want 1", count)
	}
}

func TestMessageBuffer_GetContent_UsesContentLineCount(t *testing.T) {
	mb := NewMessageBuffer()
	mb.SetLine(1, "line one")
	mb.SetLine(2, "line two")
	mb.SetLine(3, "")

	content := mb.GetContent()
	expected := "line one\nline two"
	if content != expected {
		t.Errorf("GetContent should trim trailing blanks, got %q, want %q", content, expected)
	}
}

func TestMessageBuffer_Clear_ClearsHardNewlines(t *testing.T) {
	mb := NewMessageBuffer()
	mb.LoadContent("line one\nline two")

	mb.Clear()

	for i := 1; i <= 5; i++ {
		if mb.IsHardNewline(i) {
			t.Errorf("hardNewline[%d] should be false after Clear", i)
		}
	}
	if mb.GetLineCount() != 1 {
		t.Errorf("lineCount should be 1 after Clear, got %d", mb.GetLineCount())
	}
}
