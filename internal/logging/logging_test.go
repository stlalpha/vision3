// internal/logging/logging_test.go
package logging

import (
	"bytes"
	"log"
	"os"
	"testing"
)

func TestDebugDisabled(t *testing.T) {
	DebugEnabled = false
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	Debug("this should not appear")

	if buf.Len() > 0 {
		t.Errorf("Debug output when disabled: %s", buf.String())
	}
}

func TestDebugEnabled(t *testing.T) {
	DebugEnabled = true
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	Debug("test message %d", 42)

	if !bytes.Contains(buf.Bytes(), []byte("DEBUG: test message 42")) {
		t.Errorf("Expected debug output, got: %s", buf.String())
	}
	DebugEnabled = false
}
