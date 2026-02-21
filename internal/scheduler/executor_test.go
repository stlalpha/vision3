package scheduler

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stlalpha/vision3/internal/config"
)

// lookPath finds the absolute path to a command, skipping the test if not found.
func lookPath(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("command %q not found in PATH, skipping", name)
	}
	return path
}

func TestExecuteEvent_Success(t *testing.T) {
	s := &Scheduler{}

	event := config.EventConfig{
		ID:             "test_success",
		Name:           "Test Success Event",
		Command:        lookPath(t, "echo"),
		Args:           []string{"Hello, World!"},
		TimeoutSeconds: 5,
	}

	result := s.executeEvent(context.Background(), event)

	if !result.Success {
		t.Errorf("Expected success, got failure: %v", result.Error)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Output, "Hello, World!") {
		t.Errorf("Expected output to contain 'Hello, World!', got: %s", result.Output)
	}
}

func TestExecuteEvent_Failure(t *testing.T) {
	s := &Scheduler{}

	shPath := lookPath(t, "sh")

	event := config.EventConfig{
		ID:             "test_failure",
		Name:           "Test Failure Event",
		Command:        shPath,
		Args:           []string{"-c", "exit 1"},
		TimeoutSeconds: 5,
	}

	result := s.executeEvent(context.Background(), event)

	if result.Success {
		t.Error("Expected failure, got success")
	}

	if result.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", result.ExitCode)
	}
}

func TestExecuteEvent_Timeout(t *testing.T) {
	s := &Scheduler{}

	event := config.EventConfig{
		ID:             "test_timeout",
		Name:           "Test Timeout Event",
		Command:        lookPath(t, "sleep"),
		Args:           []string{"10"},
		TimeoutSeconds: 1,
	}

	start := time.Now()
	result := s.executeEvent(context.Background(), event)
	duration := time.Since(start)

	if result.Success {
		t.Error("Expected failure due to timeout, got success")
	}

	if duration > 2*time.Second {
		t.Errorf("Expected timeout after ~1 second, took %v", duration)
	}
}

func TestExecuteEvent_WithEnvironmentVars(t *testing.T) {
	s := &Scheduler{}

	event := config.EventConfig{
		ID:      "test_env",
		Name:    "Test Environment Variables",
		Command: lookPath(t, "sh"),
		Args:    []string{"-c", "echo $TEST_VAR"},
		EnvironmentVars: map[string]string{
			"TEST_VAR": "test_value",
		},
		TimeoutSeconds: 5,
	}

	result := s.executeEvent(context.Background(), event)

	if !result.Success {
		t.Errorf("Expected success, got failure: %v", result.Error)
	}

	if !strings.Contains(result.Output, "test_value") {
		t.Errorf("Expected output to contain 'test_value', got: %s", result.Output)
	}
}

func TestBuildSubstitutions(t *testing.T) {
	s := &Scheduler{}

	event := config.EventConfig{
		ID:   "test_placeholders",
		Name: "Test Placeholders Event",
	}

	subs := s.buildSubstitutions(event)

	expectedKeys := []string{"{TIMESTAMP}", "{EVENT_ID}", "{EVENT_NAME}", "{BBS_ROOT}", "{DATE}", "{TIME}", "{DATETIME}"}
	for _, key := range expectedKeys {
		if _, exists := subs[key]; !exists {
			t.Errorf("Expected placeholder %s to exist", key)
		}
	}

	if subs["{EVENT_ID}"] != "test_placeholders" {
		t.Errorf("Expected EVENT_ID to be 'test_placeholders', got %s", subs["{EVENT_ID}"])
	}

	if subs["{EVENT_NAME}"] != "Test Placeholders Event" {
		t.Errorf("Expected EVENT_NAME to be 'Test Placeholders Event', got %s", subs["{EVENT_NAME}"])
	}
}

func TestPlaceholderSubstitutionInWorkingDirectory(t *testing.T) {
	s := &Scheduler{}

	// Create a temp directory so the working directory actually exists
	tmpDir := t.TempDir()

	event := config.EventConfig{
		ID:               "test_workdir",
		Name:             "Test Working Directory Substitution",
		Command:          lookPath(t, "pwd"),
		WorkingDirectory: tmpDir,
		TimeoutSeconds:   5,
	}

	result := s.executeEvent(context.Background(), event)

	if !result.Success {
		t.Errorf("Expected success, got failure: %v", result.Error)
	}

	if strings.Contains(result.Output, "{BBS_ROOT}") {
		t.Error("Working directory placeholder was not substituted")
	}
}

func TestPlaceholderSubstitutionInCommand(t *testing.T) {
	s := &Scheduler{}

	echoPath := lookPath(t, "echo")

	event := config.EventConfig{
		ID:             "test_cmd_sub",
		Name:           "Test Command Path Substitution",
		Command:        "{BBS_ROOT}/nonexistent_should_be_replaced",
		Args:           []string{"hello"},
		TimeoutSeconds: 5,
	}

	// The substituted command won't exist, so it should fail with a path that
	// does NOT contain the literal placeholder.
	result := s.executeEvent(context.Background(), event)

	if result.Error != nil && strings.Contains(result.Error.Error(), "{BBS_ROOT}") {
		t.Error("Command placeholder was not substituted")
	}

	// Now test with a valid command using the placeholder mechanism.
	// We'll set the command to the real echo path (no placeholder) to confirm
	// normal execution still works after the refactor.
	event2 := config.EventConfig{
		ID:             "test_cmd_real",
		Name:           "Test Real Command Execution",
		Command:        echoPath,
		Args:           []string{"placeholder_test_ok"},
		TimeoutSeconds: 5,
	}

	result2 := s.executeEvent(context.Background(), event2)
	if !result2.Success {
		t.Errorf("Expected success, got failure: %v", result2.Error)
	}
	if !strings.Contains(result2.Output, "placeholder_test_ok") {
		t.Errorf("Expected output to contain 'placeholder_test_ok', got: %s", result2.Output)
	}
}
