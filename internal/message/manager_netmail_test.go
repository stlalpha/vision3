package message

import "testing"

func TestSplitNetmailTo(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantAddr string
	}{
		{"AreaFix", "AreaFix", ""},
		{"All", "All", ""},
		{"areafix@3:633/2744", "areafix", "3:633/2744"},
		{"AreaFix@21:1/100", "AreaFix", "21:1/100"},
		{"SysOp@3:633/2744.11", "SysOp", "3:633/2744.11"},
		{"user@example.com", "user@example.com", ""},  // email-like, not FTN
		{"@3:633/2744", "@3:633/2744", ""},             // no username before @
		{"J0hn Doe@1:2/3", "J0hn Doe", "1:2/3"},       // space in name
		{"", "", ""},
	}

	for _, tt := range tests {
		name, addr := splitNetmailTo(tt.input)
		if name != tt.wantName || addr != tt.wantAddr {
			t.Errorf("splitNetmailTo(%q) = (%q, %q), want (%q, %q)",
				tt.input, name, addr, tt.wantName, tt.wantAddr)
		}
	}
}
