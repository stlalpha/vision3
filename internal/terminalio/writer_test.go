package terminalio

import (
	"bytes"
	"testing"

	"github.com/stlalpha/vision3/internal/ansi"
)

func TestWriteProcessedBytes_UTF8Mode_MapsCP437InvalidSpan(t *testing.T) {
	// Use CP437 bytes separated by ASCII space to prevent UTF-8 sequence formation
	// 0xB3 = │ (vertical line), 0xBA = ║ (double vertical line)
	input := []byte{0xB3, 0x20, 0xBA} // │ ║ in CP437

	var out bytes.Buffer
	err := WriteProcessedBytes(&out, input, ansi.OutputModeUTF8)
	if err != nil {
		t.Fatalf("WriteProcessedBytes returned error: %v", err)
	}

	want := "│ ║"
	if out.String() != want {
		t.Fatalf("unexpected output: got %q want %q", out.String(), want)
	}
}

func TestWriteProcessedBytes_UTF8Mode_PreservesValidUTF8Span(t *testing.T) {
	input := []byte("Hello π")

	var out bytes.Buffer
	err := WriteProcessedBytes(&out, input, ansi.OutputModeUTF8)
	if err != nil {
		t.Fatalf("WriteProcessedBytes returned error: %v", err)
	}

	if !bytes.Equal(out.Bytes(), input) {
		t.Fatalf("valid UTF-8 should pass through unchanged: got %q want %q", out.String(), string(input))
	}
}

func TestWriteProcessedBytes_UTF8Mode_PreservesANSIAndMapsCP437(t *testing.T) {
	// Use CP437 bytes separated by spaces to prevent UTF-8 sequence formation
	input := []byte("\x1b[31m\xB3\x20\xBA\x1b[0m")

	var out bytes.Buffer
	err := WriteProcessedBytes(&out, input, ansi.OutputModeUTF8)
	if err != nil {
		t.Fatalf("WriteProcessedBytes returned error: %v", err)
	}

	want := "\x1b[31m│ ║\x1b[0m"
	if out.String() != want {
		t.Fatalf("unexpected output with ANSI: got %q want %q", out.String(), want)
	}
}
