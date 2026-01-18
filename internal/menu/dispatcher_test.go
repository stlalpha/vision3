package menu

import (
	"testing"
)

// TestParseCommand verifies the ParseCommand function correctly identifies
// command types and extracts arguments from command strings.
func TestParseCommand(t *testing.T) {
	testCases := []struct {
		name         string
		input        string
		expectedType CommandType
		expectedArg  string
	}{
		{
			name:         "GOTO command",
			input:        "GOTO:MAIN",
			expectedType: CommandTypeGoto,
			expectedArg:  "MAIN",
		},
		{
			name:         "GOTO command lowercase",
			input:        "GOTO:main",
			expectedType: CommandTypeGoto,
			expectedArg:  "MAIN", // Should be uppercased
		},
		{
			name:         "LOGOFF command",
			input:        "LOGOFF",
			expectedType: CommandTypeLogoff,
			expectedArg:  "",
		},
		{
			name:         "RUN command without args",
			input:        "RUN:LASTCALLERS",
			expectedType: CommandTypeRun,
			expectedArg:  "LASTCALLERS",
		},
		{
			name:         "RUN command with args",
			input:        "RUN:SETRENDER MODE=LIGHTBAR",
			expectedType: CommandTypeRun,
			expectedArg:  "SETRENDER MODE=LIGHTBAR",
		},
		{
			name:         "RUN command lowercase target",
			input:        "RUN:lastcallers",
			expectedType: CommandTypeRun,
			expectedArg:  "LASTCALLERS",
		},
		{
			name:         "DOOR command",
			input:        "DOOR:LORD",
			expectedType: CommandTypeDoor,
			expectedArg:  "LORD",
		},
		{
			name:         "Unknown command",
			input:        "UNKNOWN:FOO",
			expectedType: CommandTypeUnknown,
			expectedArg:  "UNKNOWN:FOO",
		},
		{
			name:         "Empty command",
			input:        "",
			expectedType: CommandTypeUnknown,
			expectedArg:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdType, cmdArg := ParseCommand(tc.input)
			if cmdType != tc.expectedType {
				t.Errorf("ParseCommand(%q) type = %v, want %v", tc.input, cmdType, tc.expectedType)
			}
			if cmdArg != tc.expectedArg {
				t.Errorf("ParseCommand(%q) arg = %q, want %q", tc.input, cmdArg, tc.expectedArg)
			}
		})
	}
}

// TestExtractRunTarget verifies correct extraction of RUN target and arguments.
func TestExtractRunTarget(t *testing.T) {
	testCases := []struct {
		name           string
		input          string
		expectedTarget string
		expectedArgs   string
	}{
		{
			name:           "simple target",
			input:          "LASTCALLERS",
			expectedTarget: "LASTCALLERS",
			expectedArgs:   "",
		},
		{
			name:           "target with args",
			input:          "SETRENDER MODE=LIGHTBAR",
			expectedTarget: "SETRENDER",
			expectedArgs:   "MODE=LIGHTBAR",
		},
		{
			name:           "target with multiple args",
			input:          "SOMECOMMAND ARG1 ARG2 ARG3",
			expectedTarget: "SOMECOMMAND",
			expectedArgs:   "ARG1 ARG2 ARG3",
		},
		{
			name:           "empty input",
			input:          "",
			expectedTarget: "",
			expectedArgs:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			target, args := ExtractRunTarget(tc.input)
			if target != tc.expectedTarget {
				t.Errorf("ExtractRunTarget(%q) target = %q, want %q", tc.input, target, tc.expectedTarget)
			}
			if args != tc.expectedArgs {
				t.Errorf("ExtractRunTarget(%q) args = %q, want %q", tc.input, args, tc.expectedArgs)
			}
		})
	}
}

// TestActionTypeConstants verifies the action type constants are defined correctly.
func TestActionTypeConstants(t *testing.T) {
	// These should be distinct string values
	if ActionTypeGoto == ActionTypeLogoff {
		t.Errorf("ActionTypeGoto and ActionTypeLogoff should be distinct")
	}
	if ActionTypeLogoff == ActionTypeContinue {
		t.Errorf("ActionTypeLogoff and ActionTypeContinue should be distinct")
	}
	if ActionTypeGoto == ActionTypeContinue {
		t.Errorf("ActionTypeGoto and ActionTypeContinue should be distinct")
	}

	// Verify expected values
	if ActionTypeGoto != "GOTO" {
		t.Errorf("ActionTypeGoto = %q, want %q", ActionTypeGoto, "GOTO")
	}
	if ActionTypeLogoff != "LOGOFF" {
		t.Errorf("ActionTypeLogoff = %q, want %q", ActionTypeLogoff, "LOGOFF")
	}
	if ActionTypeContinue != "CONTINUE" {
		t.Errorf("ActionTypeContinue = %q, want %q", ActionTypeContinue, "CONTINUE")
	}
}

// TestCommandTypeConstants verifies the command type constants are defined correctly.
func TestCommandTypeConstants(t *testing.T) {
	// These should be distinct values
	if CommandTypeGoto == CommandTypeLogoff {
		t.Errorf("CommandTypeGoto and CommandTypeLogoff should be distinct")
	}
	if CommandTypeRun == CommandTypeDoor {
		t.Errorf("CommandTypeRun and CommandTypeDoor should be distinct")
	}
	if CommandTypeUnknown == CommandTypeGoto {
		t.Errorf("CommandTypeUnknown and CommandTypeGoto should be distinct")
	}
}
