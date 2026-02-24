package menu

import (
	"strings"
	"testing"
)

func TestFormatDIZLines_Empty(t *testing.T) {
	lines := formatDIZLines("", 45, 11)
	if lines != nil {
		t.Errorf("expected nil for empty input, got %v", lines)
	}
}

func TestFormatDIZLines_SingleLine(t *testing.T) {
	lines := formatDIZLines("Cool utility v1.0", 45, 11)
	if len(lines) != 1 || lines[0] != "Cool utility v1.0" {
		t.Errorf("expected single line, got %v", lines)
	}
}

func TestFormatDIZLines_MultiLineCRLF(t *testing.T) {
	input := "Line one\r\nLine two\r\nLine three"
	lines := formatDIZLines(input, 45, 11)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "Line one" || lines[1] != "Line two" || lines[2] != "Line three" {
		t.Errorf("unexpected lines: %v", lines)
	}
}

func TestFormatDIZLines_TruncatesLongLines(t *testing.T) {
	long := strings.Repeat("X", 60)
	lines := formatDIZLines(long, 45, 11)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if len(lines[0]) != 45 {
		t.Errorf("expected 45 chars, got %d: %q", len(lines[0]), lines[0])
	}
}

func TestFormatDIZLines_MaxLines(t *testing.T) {
	var parts []string
	for i := 0; i < 20; i++ {
		parts = append(parts, "line")
	}
	input := strings.Join(parts, "\n")
	lines := formatDIZLines(input, 45, 11)
	if len(lines) != 11 {
		t.Errorf("expected 11 lines max, got %d", len(lines))
	}
}

func TestFormatDIZLines_TrimsTrailingBlanks(t *testing.T) {
	input := "Content\n\n\n"
	lines := formatDIZLines(input, 45, 11)
	if len(lines) != 1 {
		t.Errorf("expected trailing blanks trimmed to 1 line, got %d: %v", len(lines), lines)
	}
}

func TestFormatDIZLines_PreservesInternalBlanks(t *testing.T) {
	input := "Line one\n\nLine three"
	lines := formatDIZLines(input, 45, 11)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (with blank), got %d: %v", len(lines), lines)
	}
	if lines[1] != "" {
		t.Errorf("expected empty middle line, got %q", lines[1])
	}
}

func TestFormatDIZLines_ANSIDoesNotCountTowardWidth(t *testing.T) {
	ansiLine := "\x1b[1;31m" + strings.Repeat("X", 45) + "\x1b[0m"
	lines := formatDIZLines(ansiLine, 45, 11)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "\x1b[1;31m") {
		t.Error("expected ANSI codes to be preserved")
	}
	if !strings.Contains(lines[0], strings.Repeat("X", 45)) {
		t.Error("expected all 45 visible chars preserved")
	}
}

func TestFormatDIZLines_RealDIZ(t *testing.T) {
	input := "    __,---,__  ,---,__  ,---,____,---,____\r\n" +
		"    \\        \\/       \\/        /    /   /\r\n" +
		" ////    /        /        / __       --/\r\n" +
		"   /    /   /        /    ,   /    /   ////\r\n" +
		"  /___     /___ /   /___ /   /___ /   _\\\r\n" +
		"      `---'    `---'    `---'    `---'\r\n" +
		"           -- Darkness v2.0.0 --\r\n" +
		"\r\n" +
		"    NEW FOR 2020: Completely recoded and\r\n" +
		"  redesigned version of the cyberpunk LORD\r\n" +
		"            style BBS door game!"

	lines := formatDIZLines(input, 45, 11)
	if len(lines) != 11 {
		t.Errorf("expected 11 lines, got %d", len(lines))
	}
	for i, l := range lines {
		if len(l) > 45 {
			t.Errorf("line %d exceeds 45 chars: %d %q", i, len(l), l)
		}
	}
}
