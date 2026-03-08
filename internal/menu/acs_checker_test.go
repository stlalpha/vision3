package menu

import (
	"strconv"
	"testing"
	"time"

	"github.com/stlalpha/vision3/internal/user"
)

func TestTokenizeACS_SimpleCondition(t *testing.T) {
	tokens, err := tokenizeACS("S50")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].typ != tokenCondition || tokens[0].value != "S50" {
		t.Errorf("expected condition S50, got %v", tokens[0])
	}
}

func TestTokenizeACS_AndOperator(t *testing.T) {
	tokens, err := tokenizeACS("S50&FA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].value != "S50" {
		t.Errorf("expected S50, got %s", tokens[0].value)
	}
	if tokens[1].typ != tokenOperator || tokens[1].value != "&" {
		t.Errorf("expected & operator, got %v", tokens[1])
	}
	if tokens[2].value != "FA" {
		t.Errorf("expected FA, got %s", tokens[2].value)
	}
}

func TestTokenizeACS_NotOperator(t *testing.T) {
	tokens, err := tokenizeACS("S50&!L")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 4 {
		t.Fatalf("expected 4 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[2].typ != tokenOperator || tokens[2].value != "!" {
		t.Errorf("expected ! operator at position 2, got %v", tokens[2])
	}
	if tokens[3].value != "L" {
		t.Errorf("expected L at position 3, got %s", tokens[3].value)
	}
}

func TestTokenizeACS_Parentheses(t *testing.T) {
	tokens, err := tokenizeACS("(S50|FA)&V")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 7 {
		t.Fatalf("expected 7 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].typ != tokenLParen {
		t.Errorf("expected ( at position 0, got %v", tokens[0])
	}
	if tokens[4].typ != tokenRParen {
		t.Errorf("expected ) at position 4, got %v", tokens[4])
	}
}

func TestTokenizeACS_Empty(t *testing.T) {
	tokens, err := tokenizeACS("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for empty string, got %d", len(tokens))
	}
}

func TestTokenizeACS_WithSpaces(t *testing.T) {
	tokens, err := tokenizeACS("S50 & FA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].value != "S50" || tokens[1].value != "&" || tokens[2].value != "FA" {
		t.Errorf("unexpected tokens: %v", tokens)
	}
}

func TestInfixToRPN_SimpleAnd(t *testing.T) {
	tokens := []token{
		{typ: tokenCondition, value: "S50"},
		{typ: tokenOperator, value: "&"},
		{typ: tokenCondition, value: "FA"},
	}
	rpn, err := infixToRPN(tokens)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expected RPN: S50 FA &
	if len(rpn) != 3 {
		t.Fatalf("expected 3 RPN tokens, got %d: %v", len(rpn), rpn)
	}
	if rpn[0].value != "S50" || rpn[1].value != "FA" || rpn[2].value != "&" {
		t.Errorf("expected [S50 FA &], got [%s %s %s]", rpn[0].value, rpn[1].value, rpn[2].value)
	}
}

func TestInfixToRPN_Precedence(t *testing.T) {
	// A | B & C  should be  A (B C &) |  because & has higher precedence
	tokens := []token{
		{typ: tokenCondition, value: "A"},
		{typ: tokenOperator, value: "|"},
		{typ: tokenCondition, value: "B"},
		{typ: tokenOperator, value: "&"},
		{typ: tokenCondition, value: "C"},
	}
	rpn, err := infixToRPN(tokens)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expected RPN: A B C & |
	if len(rpn) != 5 {
		t.Fatalf("expected 5 RPN tokens, got %d", len(rpn))
	}
	expected := []string{"A", "B", "C", "&", "|"}
	for i, exp := range expected {
		if rpn[i].value != exp {
			t.Errorf("position %d: expected %s, got %s", i, exp, rpn[i].value)
		}
	}
}

func TestInfixToRPN_Parentheses(t *testing.T) {
	// (A | B) & C  should be  A B | C &
	tokens := []token{
		{typ: tokenLParen, value: "("},
		{typ: tokenCondition, value: "A"},
		{typ: tokenOperator, value: "|"},
		{typ: tokenCondition, value: "B"},
		{typ: tokenRParen, value: ")"},
		{typ: tokenOperator, value: "&"},
		{typ: tokenCondition, value: "C"},
	}
	rpn, err := infixToRPN(tokens)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"A", "B", "|", "C", "&"}
	if len(rpn) != len(expected) {
		t.Fatalf("expected %d RPN tokens, got %d", len(expected), len(rpn))
	}
	for i, exp := range expected {
		if rpn[i].value != exp {
			t.Errorf("position %d: expected %s, got %s", i, exp, rpn[i].value)
		}
	}
}

func TestInfixToRPN_MismatchedParentheses(t *testing.T) {
	tokens := []token{
		{typ: tokenLParen, value: "("},
		{typ: tokenCondition, value: "A"},
	}
	_, err := infixToRPN(tokens)
	if err == nil {
		t.Error("expected error for mismatched parentheses, got nil")
	}
}

func TestEvaluateRPN_SimpleAnd(t *testing.T) {
	// true & true = true
	u := &user.User{AccessLevel: 100, Flags: "A"}
	rpn := []token{
		{typ: tokenCondition, value: "S50"},  // 100 >= 50 = true
		{typ: tokenCondition, value: "FA"},   // flags contain A = true
		{typ: tokenOperator, value: "&"},
	}
	result, err := evaluateRPN(rpn, u, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true for S50&FA with level 100 and flag A")
	}
}

func TestEvaluateRPN_SimpleAndFalse(t *testing.T) {
	// true & false = false
	u := &user.User{AccessLevel: 100, Flags: ""}
	rpn := []token{
		{typ: tokenCondition, value: "S50"},  // 100 >= 50 = true
		{typ: tokenCondition, value: "FA"},   // flags empty = false
		{typ: tokenOperator, value: "&"},
	}
	result, err := evaluateRPN(rpn, u, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false for S50&FA with level 100 and no flags")
	}
}

func TestEvaluateRPN_Not(t *testing.T) {
	u := &user.User{AccessLevel: 100, Validated: true}
	rpn := []token{
		{typ: tokenCondition, value: "V"},   // validated = true
		{typ: tokenOperator, value: "!"},    // !true = false
	}
	result, err := evaluateRPN(rpn, u, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false for !V with validated user")
	}
}

func TestEvaluateRPN_Or(t *testing.T) {
	u := &user.User{AccessLevel: 10, Flags: "A"}
	rpn := []token{
		{typ: tokenCondition, value: "S50"},  // 10 >= 50 = false
		{typ: tokenCondition, value: "FA"},   // has flag A = true
		{typ: tokenOperator, value: "|"},
	}
	result, err := evaluateRPN(rpn, u, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true for S50|FA with level 10 and flag A")
	}
}

func TestEvaluateRPN_NotEnoughOperands(t *testing.T) {
	rpn := []token{
		{typ: tokenCondition, value: "S50"},
		{typ: tokenOperator, value: "&"}, // needs 2 operands, only 1
	}
	u := &user.User{AccessLevel: 100}
	_, err := evaluateRPN(rpn, u, nil, nil, time.Now())
	if err == nil {
		t.Error("expected error for not enough operands")
	}
}

func TestEvaluateCondition_SecurityLevel(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		level     int
		want      bool
	}{
		{"level meets threshold", "S50", 50, true},
		{"level exceeds threshold", "S50", 100, true},
		{"level below threshold", "S50", 10, false},
		{"level zero", "S0", 0, true},
		{"level 255", "S255", 255, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &user.User{AccessLevel: tt.level}
			got := evaluateCondition(tt.condition, u, nil, nil, time.Now())
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) with level %d = %v, want %v", tt.condition, tt.level, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_Flags(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		flags     string
		want      bool
	}{
		{"has flag", "FA", "ABC", true},
		{"missing flag", "FX", "ABC", false},
		{"empty flags", "FA", "", false},
		{"case insensitive", "Fa", "ABC", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &user.User{Flags: tt.flags}
			got := evaluateCondition(tt.condition, u, nil, nil, time.Now())
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) with flags %q = %v, want %v", tt.condition, tt.flags, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_Validated(t *testing.T) {
	u := &user.User{Validated: true}
	if !evaluateCondition("V", u, nil, nil, time.Now()) {
		t.Error("expected true for V with validated user")
	}
	u.Validated = false
	if evaluateCondition("V", u, nil, nil, time.Now()) {
		t.Error("expected false for V with unvalidated user")
	}
}

func TestEvaluateCondition_UserID(t *testing.T) {
	u := &user.User{ID: 42}
	if !evaluateCondition("U42", u, nil, nil, time.Now()) {
		t.Error("expected true for U42 with user ID 42")
	}
	if evaluateCondition("U99", u, nil, nil, time.Now()) {
		t.Error("expected false for U99 with user ID 42")
	}
}

func TestEvaluateCondition_FilePoints(t *testing.T) {
	u := &user.User{FilePoints: 100}
	if !evaluateCondition("P50", u, nil, nil, time.Now()) {
		t.Error("expected true for P50 with 100 points")
	}
	if evaluateCondition("P200", u, nil, nil, time.Now()) {
		t.Error("expected false for P200 with 100 points")
	}
}

func TestEvaluateCondition_PrivateNote(t *testing.T) {
	u := &user.User{PrivateNote: "VIP member"}
	if !evaluateCondition("ZVIP", u, nil, nil, time.Now()) {
		t.Error("expected true for ZVIP with note 'VIP member'")
	}
	if evaluateCondition("ZADMIN", u, nil, nil, time.Now()) {
		t.Error("expected false for ZADMIN with note 'VIP member'")
	}
}

func TestEvaluateCondition_UnknownCode(t *testing.T) {
	u := &user.User{}
	if evaluateCondition("Q99", u, nil, nil, time.Now()) {
		t.Error("expected false for unknown ACS code Q")
	}
}

func TestEvaluateCondition_InvalidSecurityValue(t *testing.T) {
	u := &user.User{AccessLevel: 100}
	if evaluateCondition("Sabc", u, nil, nil, time.Now()) {
		t.Error("expected false for non-numeric security level")
	}
}

func TestCheckACS_EmptyAllows(t *testing.T) {
	if !checkACS("", nil, nil, nil, time.Now()) {
		t.Error("expected empty ACS to allow access")
	}
}

func TestCheckACS_WildcardAllows(t *testing.T) {
	if !checkACS("*", nil, nil, nil, time.Now()) {
		t.Error("expected wildcard ACS to allow access")
	}
}

func TestCheckACS_NilUserDenied(t *testing.T) {
	if checkACS("S50", nil, nil, nil, time.Now()) {
		t.Error("expected non-empty ACS to deny nil user")
	}
}

func TestCheckACS_FullExpression(t *testing.T) {
	u := &user.User{AccessLevel: 100, Flags: "A", Validated: true}
	if !checkACS("S50&FA&V", u, nil, nil, time.Now()) {
		t.Error("expected S50&FA&V to pass for level 100, flag A, validated user")
	}
}

func TestCheckACS_ComplexExpression(t *testing.T) {
	u := &user.User{AccessLevel: 10, Flags: "A"}
	// Low level but has flag A, so S255|FA should pass
	if !checkACS("S255|FA", u, nil, nil, time.Now()) {
		t.Error("expected S255|FA to pass for user with flag A")
	}
}

func TestCheckACS_MismatchedParensReturnsFalse(t *testing.T) {
	// Malformed ACS should deny access (return false), not panic
	u := &user.User{AccessLevel: 255}
	if checkACS("(S50", u, nil, nil, time.Now()) {
		t.Error("expected mismatched parentheses to deny access")
	}
}

func TestTokenizeACS_UnexpectedCharacterDropped(t *testing.T) {
	// The '@' character is not a valid operator or condition character.
	// The tokenizer silently drops it and continues parsing.
	tokens, err := tokenizeACS("S50@FA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should get S50 and FA as conditions (@ silently dropped)
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens (unexpected char dropped), got %d: %v", len(tokens), tokens)
	}
	if tokens[0].value != "S50" || tokens[1].value != "FA" {
		t.Errorf("expected [S50, FA], got [%s, %s]", tokens[0].value, tokens[1].value)
	}
}

// --- B: Upload/Download Ratio ---

func TestEvaluateCondition_BytesRatio(t *testing.T) {
	tests := []struct {
		name       string
		condition  string
		uploads    int
		downloads  int
		want       bool
	}{
		{"ratio meets threshold", "B50", 5, 10, true},     // 50% ratio
		{"ratio exceeds threshold", "B50", 10, 10, true},  // 100% ratio
		{"ratio below threshold", "B50", 2, 10, false},    // 20% ratio
		{"no downloads passes", "B50", 0, 0, true},        // no downloads = always pass
		{"zero uploads with downloads", "B50", 0, 10, false}, // 0% ratio
		{"threshold zero", "B0", 0, 10, true},              // 0% threshold always passes
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &user.User{NumUploads: tt.uploads, NumDownloads: tt.downloads}
			got := evaluateCondition(tt.condition, u, nil, nil, time.Now())
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) uploads=%d downloads=%d = %v, want %v",
					tt.condition, tt.uploads, tt.downloads, got, tt.want)
			}
		})
	}
}

// --- C: Message Conference ---

func TestEvaluateCondition_MsgConference(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		confID    int
		want      bool
	}{
		{"matching conference", "C1", 1, true},
		{"non-matching conference", "C2", 1, false},
		{"conference zero", "C0", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &user.User{CurrentMsgConferenceID: tt.confID}
			got := evaluateCondition(tt.condition, u, nil, nil, time.Now())
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) confID=%d = %v, want %v",
					tt.condition, tt.confID, got, tt.want)
			}
		})
	}
}

// --- I: Internet Access Flag ---

func TestEvaluateCondition_InternetFlag(t *testing.T) {
	tests := []struct {
		name  string
		flags string
		want  bool
	}{
		{"has I flag", "ABI", true},
		{"no I flag", "AB", false},
		{"empty flags", "", false},
		{"only I flag", "I", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &user.User{Flags: tt.flags}
			got := evaluateCondition("I", u, nil, nil, time.Now())
			if got != tt.want {
				t.Errorf("evaluateCondition('I') flags=%q = %v, want %v",
					tt.flags, got, tt.want)
			}
		})
	}
}

// --- X: File Conference ---

func TestEvaluateCondition_FileConference(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		confID    int
		want      bool
	}{
		{"matching file conference", "X1", 1, true},
		{"non-matching file conference", "X2", 1, false},
		{"file conference zero", "X0", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &user.User{CurrentFileConferenceID: tt.confID}
			got := evaluateCondition(tt.condition, u, nil, nil, time.Now())
			if got != tt.want {
				t.Errorf("evaluateCondition(%q) confID=%d = %v, want %v",
					tt.condition, tt.confID, got, tt.want)
			}
		})
	}
}

// --- D: Download Level ---

func TestEvaluateCondition_DownloadLevel(t *testing.T) {
	u := &user.User{AccessLevel: 50}
	if !evaluateCondition("D50", u, nil, nil, time.Now()) {
		t.Error("expected true for D50 with level 50")
	}
	if evaluateCondition("D100", u, nil, nil, time.Now()) {
		t.Error("expected false for D100 with level 50")
	}
}

// --- E: Post/Call Ratio ---

func TestEvaluateCondition_PostCallRatio(t *testing.T) {
	u := &user.User{NumUploads: 10, TimesCalled: 20}
	if !evaluateCondition("E50", u, nil, nil, time.Now()) {
		t.Error("expected true for E50 with 10/20 ratio (50%)")
	}
	if evaluateCondition("E75", u, nil, nil, time.Now()) {
		t.Error("expected false for E75 with 10/20 ratio (50%)")
	}
	// Zero calls
	u2 := &user.User{NumUploads: 5, TimesCalled: 0}
	if evaluateCondition("E50", u2, nil, nil, time.Now()) {
		t.Error("expected false for E50 with zero calls")
	}
}

// --- T: Time Left ---

func TestEvaluateCondition_TimeLeft(t *testing.T) {
	// User with 60-minute limit, session just started
	u := &user.User{TimeLimit: 60}
	startTime := time.Now()
	if !evaluateCondition("T30", u, nil, nil, startTime) {
		t.Error("expected true for T30 with 60 minutes remaining")
	}
	// User with no time limit (unlimited)
	u2 := &user.User{TimeLimit: 0}
	if !evaluateCondition("T30", u2, nil, nil, startTime) {
		t.Error("expected true for T30 with unlimited time")
	}
}

// --- W: Day of Week ---

func TestEvaluateCondition_DayOfWeek(t *testing.T) {
	u := &user.User{}
	today := int(time.Now().Weekday())
	todayStr := "W" + string(rune('0'+today))
	if !evaluateCondition(todayStr, u, nil, nil, time.Now()) {
		t.Errorf("expected true for %s (today)", todayStr)
	}
	// Wrong day
	wrongDay := (today + 1) % 7
	wrongStr := "W" + string(rune('0'+wrongDay))
	if evaluateCondition(wrongStr, u, nil, nil, time.Now()) {
		t.Errorf("expected false for %s (not today)", wrongStr)
	}
}

// --- SYSOP/COSYSOP keyword conditions ---

func TestEvaluateCondition_SysOpKeyword(t *testing.T) {
	u := &user.User{AccessLevel: 255}
	if !evaluateCondition("SYSOP", u, nil, nil, time.Now()) {
		t.Error("expected true for SYSOP with level 255")
	}
	u.AccessLevel = 250
	if evaluateCondition("SYSOP", u, nil, nil, time.Now()) {
		t.Error("expected false for SYSOP with level 250")
	}
}

func TestEvaluateCondition_CoSysOpKeyword(t *testing.T) {
	u := &user.User{AccessLevel: 250}
	if !evaluateCondition("COSYSOP", u, nil, nil, time.Now()) {
		t.Error("expected true for COSYSOP with level 250")
	}
	u.AccessLevel = 200
	if evaluateCondition("COSYSOP", u, nil, nil, time.Now()) {
		t.Error("expected false for COSYSOP with level 200")
	}
}

// --- Complex checkACS integration tests ---

func TestCheckACS_NotWithParens(t *testing.T) {
	u := &user.User{AccessLevel: 100, Flags: "A", Validated: false}
	// !(V) & S50 — not validated AND level >= 50 => should pass
	if !checkACS("!V&S50", u, nil, nil, time.Now()) {
		t.Error("expected !V&S50 to pass for unvalidated user with level 100")
	}
}

func TestCheckACS_NestedParens(t *testing.T) {
	u := &user.User{AccessLevel: 100, Flags: "A", Validated: true}
	// (S50&V)|(FA&S200) — first group passes
	if !checkACS("(S50&V)|(FA&S200)", u, nil, nil, time.Now()) {
		t.Error("expected (S50&V)|(FA&S200) to pass")
	}
}

func TestCheckACS_AllOr(t *testing.T) {
	u := &user.User{AccessLevel: 5, Flags: "", Validated: false, ID: 42}
	// Nothing should match except U42
	if !checkACS("S255|FA|V|U42", u, nil, nil, time.Now()) {
		t.Error("expected S255|FA|V|U42 to pass via U42")
	}
}

func TestCheckACS_InternetAndConference(t *testing.T) {
	u := &user.User{Flags: "I", CurrentMsgConferenceID: 3}
	if !checkACS("I&C3", u, nil, nil, time.Now()) {
		t.Error("expected I&C3 to pass for user with flag I in conference 3")
	}
	u.CurrentMsgConferenceID = 1
	if checkACS("I&C3", u, nil, nil, time.Now()) {
		t.Error("expected I&C3 to fail for user in conference 1")
	}
}

// --- Y: Time Range ---

func TestEvaluateCondition_TimeRange(t *testing.T) {
	u := &user.User{}
	// Use a range that covers the current time (00:00 to 23:59)
	if !evaluateCondition("Y00:00/23:59", u, nil, nil, time.Now()) {
		t.Error("expected true for Y00:00/23:59 (all day range)")
	}
	// Invalid format
	if evaluateCondition("Ybadformat", u, nil, nil, time.Now()) {
		t.Error("expected false for invalid time range format")
	}
	// Invalid time values
	if evaluateCondition("Y25:00/26:00", u, nil, nil, time.Now()) {
		t.Error("expected false for invalid time values")
	}
}

// --- H: Hour ---

func TestEvaluateCondition_Hour(t *testing.T) {
	u := &user.User{}
	currentHour := time.Now().Hour()
	hourStr := "H" + strconv.Itoa(currentHour)
	if !evaluateCondition(hourStr, u, nil, nil, time.Now()) {
		t.Errorf("expected true for %s (current hour)", hourStr)
	}
	wrongHour := (currentHour + 12) % 24
	wrongStr := "H" + strconv.Itoa(wrongHour)
	if evaluateCondition(wrongStr, u, nil, nil, time.Now()) {
		t.Errorf("expected false for %s (not current hour)", wrongStr)
	}
	// Invalid hour
	if evaluateCondition("H25", u, nil, nil, time.Now()) {
		t.Error("expected false for H25 (invalid hour)")
	}
	if evaluateCondition("Habc", u, nil, nil, time.Now()) {
		t.Error("expected false for Habc (non-numeric)")
	}
}

// --- Error paths for various ACS codes ---

func TestEvaluateCondition_InvalidValues(t *testing.T) {
	u := &user.User{}
	tests := []struct {
		name      string
		condition string
	}{
		{"invalid B value", "Babc"},
		{"invalid C value", "Cabc"},
		{"invalid X value", "Xabc"},
		{"invalid D value", "Dabc"},
		{"invalid E value", "Eabc"},
		{"invalid T value", "Tabc"},
		{"invalid W value", "Wabc"},
		{"invalid U value", "Uabc"},
		{"invalid P value", "Pabc"},
		{"invalid W out of range", "W9"},
		{"empty condition", ""},
		{"flag too long", "FAB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCondition(tt.condition, u, nil, nil, time.Now())
			if got {
				t.Errorf("evaluateCondition(%q) = true, want false", tt.condition)
			}
		})
	}
}

// Note on nil ssh.Session in tests: evaluateCondition and evaluateRPN tests pass
// nil for the ssh.Session parameter. This is safe for conditions that don't touch
// the session (S, F, V, U, P, Z, etc.) but would panic for conditions like L
// (local connection check) or A (ANSI/PTY check) which call s.RemoteAddr() or
// s.Pty(). If adding tests for those conditions, provide a mock session.
