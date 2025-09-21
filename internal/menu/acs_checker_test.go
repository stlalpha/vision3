package menu

import (
	"io"
	"net"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/user"
)

type stubChannel struct{}

func (stubChannel) Read(p []byte) (int, error)  { return 0, io.EOF }
func (stubChannel) Write(p []byte) (int, error) { return len(p), nil }
func (stubChannel) Close() error                { return nil }
func (stubChannel) CloseWrite() error           { return nil }
func (stubChannel) SendRequest(string, bool, []byte) (bool, error) {
	return false, nil
}
func (stubChannel) Stderr() io.ReadWriter { return nopBuffer{} }

// nopBuffer is a minimal io.ReadWriter used for stubChannel.Stderr().
type nopBuffer struct{}

func (nopBuffer) Write(p []byte) (int, error) { return len(p), nil }
func (nopBuffer) Read(p []byte) (int, error)  { return 0, io.EOF }

type stubAddr struct {
	network string
	addr    string
}

func (a stubAddr) Network() string { return a.network }
func (a stubAddr) String() string  { return a.addr }

type stubSession struct {
	stubChannel
	username string
	remote   net.Addr
	environ  []string
	pty      ssh.Pty
	hasPty   bool
}

func (s stubSession) User() string                            { return s.username }
func (s stubSession) RemoteAddr() net.Addr                    { return s.remote }
func (s stubSession) LocalAddr() net.Addr                     { return stubAddr{"tcp", "127.0.0.1:2222"} }
func (s stubSession) Environ() []string                       { return append([]string(nil), s.environ...) }
func (s stubSession) Exit(int) error                          { return nil }
func (s stubSession) Command() []string                       { return nil }
func (s stubSession) RawCommand() string                      { return "" }
func (s stubSession) Subsystem() string                       { return "" }
func (s stubSession) PublicKey() ssh.PublicKey                { return nil }
func (s stubSession) Context() ssh.Context                    { return nil }
func (s stubSession) Permissions() ssh.Permissions            { return ssh.Permissions{} }
func (s stubSession) Pty() (ssh.Pty, <-chan ssh.Window, bool) { return s.pty, nil, s.hasPty }
func (s stubSession) Signals(chan<- ssh.Signal)               {}
func (s stubSession) Break(chan<- bool)                       {}

// Ensure stubSession satisfies the ssh.Session interface at compile time.
var _ ssh.Session = (*stubSession)(nil)

func TestTokenizeACS(t *testing.T) {
	tokens, err := tokenizeACS("S10 & (FZ | !V)")
	if err != nil {
		t.Fatalf("tokenizeACS returned error: %v", err)
	}
	if len(tokens) != 8 {
		t.Fatalf("expected 8 tokens, got %d", len(tokens))
	}
}

func TestInfixToRPN_MismatchedParentheses(t *testing.T) {
	tokens := []token{{typ: tokenLParen, value: "("}, {typ: tokenCondition, value: "S10"}}
	if _, err := infixToRPN(tokens); err == nil {
		t.Fatalf("expected error for mismatched parentheses")
	}
}

func TestCheckACSBasicCases(t *testing.T) {
	sess := stubSession{
		username: "guest",
		remote:   stubAddr{"tcp", "127.0.0.1:5000"},
		hasPty:   true,
	}
	u := &user.User{AccessLevel: 50, Flags: "Z", Validated: true}
	now := time.Now()

	tests := []struct {
		name    string
		acs     string
		expect  bool
		user    *user.User
		session ssh.Session
	}{
		{name: "empty", acs: "", expect: true, user: u, session: sess},
		{name: "wildcard", acs: "*", expect: true, user: u, session: sess},
		{name: "unauthenticated", acs: "S10", expect: false, user: nil, session: sess},
		{name: "security level ok", acs: "S10", expect: true, user: u, session: sess},
		{name: "security level fail", acs: "S99", expect: false, user: u, session: sess},
		{name: "flag and", acs: "S40 & FZ", expect: true, user: u, session: sess},
		{name: "negated flag", acs: "S40 & !FY", expect: true, user: u, session: sess},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := checkACS(tc.acs, tc.user, tc.session, nil, now)
			if result != tc.expect {
				t.Fatalf("checkACS(%s) = %t, expected %t", tc.acs, result, tc.expect)
			}
		})
	}
}

func TestEvaluateRPN_InvalidExpression(t *testing.T) {
	u := &user.User{}
	sess := stubSession{}
	_, err := evaluateRPN([]token{{typ: tokenOperator, value: "&"}}, u, sess, nil, time.Now())
	if err == nil {
		t.Fatalf("expected error for invalid RPN expression")
	}
}

func TestEvaluateCondition_LocalAndAnsi(t *testing.T) {
	sess := stubSession{
		remote: stubAddr{"tcp", "127.0.0.1:5000"},
		pty: ssh.Pty{
			Term:   "ansi",
			Window: ssh.Window{Width: 80, Height: 25},
		},
		hasPty: true,
	}
	u := &user.User{Flags: "ABC"}
	if !evaluateCondition("L", u, sess, nil, time.Now()) {
		t.Fatalf("expected local ACS condition to pass")
	}
	if !evaluateCondition("A", u, sess, nil, time.Now()) {
		t.Fatalf("expected ANSI ACS condition to pass")
	}
	if evaluateCondition("FY", u, sess, nil, time.Now()) {
		t.Fatalf("expected missing flag to fail")
	}
}

// Prevent unused import errors for gossh when build tags change.
var _ gossh.Channel = stubChannel{}
