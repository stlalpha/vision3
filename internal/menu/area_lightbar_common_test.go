package menu

import "testing"

func TestNextConf_Wraps(t *testing.T) {
	confs := []accessibleConf{
		{id: 1, name: "A"},
		{id: 2, name: "B"},
		{id: 3, name: "C"},
	}

	// Normal forward
	if got := nextConf(confs, 1); got != 2 {
		t.Errorf("nextConf(1) = %d, want 2", got)
	}
	// Wrap around from last to first
	if got := nextConf(confs, 3); got != 1 {
		t.Errorf("nextConf(3) = %d, want 1", got)
	}
}

func TestPrevConf_Wraps(t *testing.T) {
	confs := []accessibleConf{
		{id: 1, name: "A"},
		{id: 2, name: "B"},
		{id: 3, name: "C"},
	}

	// Normal backward
	if got := prevConf(confs, 2); got != 1 {
		t.Errorf("prevConf(2) = %d, want 1", got)
	}
	// Wrap around from first to last
	if got := prevConf(confs, 1); got != 3 {
		t.Errorf("prevConf(1) = %d, want 3", got)
	}
}

func TestNextConf_EmptyList(t *testing.T) {
	if got := nextConf(nil, 5); got != 5 {
		t.Errorf("nextConf(nil, 5) = %d, want 5", got)
	}
}

func TestPrevConf_EmptyList(t *testing.T) {
	if got := prevConf(nil, 5); got != 5 {
		t.Errorf("prevConf(nil, 5) = %d, want 5", got)
	}
}

func TestNextConf_UnknownID(t *testing.T) {
	confs := []accessibleConf{
		{id: 1, name: "A"},
		{id: 2, name: "B"},
	}
	// Unknown ID should return first conference
	if got := nextConf(confs, 99); got != 1 {
		t.Errorf("nextConf(99) = %d, want 1", got)
	}
}

func TestPrevConf_UnknownID(t *testing.T) {
	confs := []accessibleConf{
		{id: 1, name: "A"},
		{id: 2, name: "B"},
	}
	// Unknown ID should return last conference
	if got := prevConf(confs, 99); got != 2 {
		t.Errorf("prevConf(99) = %d, want 2", got)
	}
}

func TestNextConf_SingleItem(t *testing.T) {
	confs := []accessibleConf{{id: 1, name: "A"}}
	if got := nextConf(confs, 1); got != 1 {
		t.Errorf("nextConf(1) = %d, want 1", got)
	}
}

func TestPrevConf_SingleItem(t *testing.T) {
	confs := []accessibleConf{{id: 1, name: "A"}}
	if got := prevConf(confs, 1); got != 1 {
		t.Errorf("prevConf(1) = %d, want 1", got)
	}
}

func TestStripAreaAnsi(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b[1;32mbold green\x1b[0m text", "bold green text"},
		{"no ansi here", "no ansi here"},
		{"", ""},
	}
	for _, tt := range tests {
		got := stripAreaAnsi(tt.input)
		if got != tt.want {
			t.Errorf("stripAreaAnsi(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
