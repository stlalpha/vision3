package renderer

import (
	"strings"
	"testing"
)

func TestRenderVisionXIncludesDynamicData(t *testing.T) {
	eng := NewEngine(Config{
		Enabled:       true,
		DefaultTheme:  "visionx",
		Palette:       "amiga",
		Codepage:      "utf8",
		MenuOverrides: map[string]Override{},
	})

	if eng == nil {
		t.Fatalf("expected engine to be constructed")
	}

	output, handled, err := eng.Render(MenuContext{
		Name: "MAIN",
		User: UserInfo{Handle: "Felonius", Node: 1},
		Stats: Stats{
			UnreadMessages:       12,
			TotalMessages:        42,
			PrimaryMessageArea:   "Message Matrix",
			PrimaryMessageUnread: 12,
			TopMessageAreas:      []AreaSummary{{Name: "General", Unread: 12}},
			NewFiles:             4,
			TotalFiles:           20,
			PrimaryFileArea:      "File Vault",
			PrimaryFileNew:       4,
			ActiveDoors:          6,
			OnlineCount:          3,
			Ratio:                "110%",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected renderer to handle MAIN menu")
	}

	rendered := collapseSpaces(stripANSI(string(output)))
	checks := []string{"Felonius", "unread: 12", "new files: 4", "focus: 12", "folks online: 3", "ratio 110%"}
	for _, check := range checks {
		if !strings.Contains(rendered, check) {
			t.Fatalf("expected rendered output to contain %q, got: %s", check, rendered)
		}
	}
}

func TestRenderFallsBackWhenDisabled(t *testing.T) {
	eng := NewEngine(Config{Enabled: false})
	if eng != nil {
		t.Fatalf("expected nil engine when disabled")
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inEscape {
			if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
				inEscape = false
			}
			continue
		}
		if ch == 0x1b {
			inEscape = true
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func collapseSpaces(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
