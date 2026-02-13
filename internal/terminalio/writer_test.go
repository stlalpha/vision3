package terminalio

import (
	"bytes"
	"testing"

	"github.com/robbiew/vision3/internal/ansi"
)

func TestWriteProcessedBytes_UTF8Mode_MapsCP437InvalidSpan(t *testing.T) {
	input := []byte{0xDA, 0xC4, 0xBF}

	var out bytes.Buffer
	err := WriteProcessedBytes(&out, input, ansi.OutputModeUTF8)
	if err != nil {
		t.Fatalf("WriteProcessedBytes returned error: %v", err)
	}

	want := "┌─┐"
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
	input := []byte("\x1b[31m\xDA\xC4\xBF\x1b[0m")

	var out bytes.Buffer
	err := WriteProcessedBytes(&out, input, ansi.OutputModeUTF8)
	if err != nil {
		t.Fatalf("WriteProcessedBytes returned error: %v", err)
	}

	want := "\x1b[31m┌─┐\x1b[0m"
	if out.String() != want {
		t.Fatalf("unexpected output with ANSI: got %q want %q", out.String(), want)
	}
}
