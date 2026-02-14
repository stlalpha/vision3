package menu

import (
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

// Note on nil ssh.Session in tests: evaluateCondition and evaluateRPN tests pass
// nil for the ssh.Session parameter. This is safe for conditions that don't touch
// the session (S, F, V, U, P, Z, etc.) but would panic for conditions like L
// (local connection check) or A (ANSI/PTY check) which call s.RemoteAddr() or
// s.Pty(). If adding tests for those conditions, provide a mock session.
