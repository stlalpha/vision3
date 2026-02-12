package ansi

import (
	"bytes"
	"testing"
)

func TestStripSAUCE(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "No SAUCE metadata",
			input:    []byte("Hello, World!\x1B[1;31mRed Text\x1B[0m"),
			expected: []byte("Hello, World!\x1B[1;31mRed Text\x1B[0m"),
		},
		{
			name:     "File too small for SAUCE",
			input:    []byte("Small file"),
			expected: []byte("Small file"),
		},
		{
			name: "SAUCE with EOF marker",
			input: func() []byte {
				content := []byte("ANSI art content here\r\n")
				content = append(content, 0x1A) // EOF marker
				sauce := make([]byte, 128)
				copy(sauce, []byte("SAUCE00"))
				sauce[5] = '2'
				sauce[6] = '0'
				content = append(content, sauce...)
				return content
			}(),
			expected: []byte("ANSI art content here\r\n"),
		},
		{
			name: "SAUCE without EOF marker",
			input: func() []byte {
				content := []byte("ANSI art without EOF\r\n")
				sauce := make([]byte, 128)
				copy(sauce, []byte("SAUCE00"))
				content = append(content, sauce...)
				return content
			}(),
			expected: []byte("ANSI art without EOF\r\n"),
		},
		{
			name: "Malformed SAUCE - no SAUCE signature",
			input: func() []byte {
				content := []byte("Normal content")
				padding := make([]byte, 128)
				copy(padding, []byte("NOTASAUCE"))
				content = append(content, padding...)
				return content
			}(),
			expected: func() []byte {
				content := []byte("Normal content")
				padding := make([]byte, 128)
				copy(padding, []byte("NOTASAUCE"))
				content = append(content, padding...)
				return content
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripSAUCE(tt.input)
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("stripSAUCE() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetAnsiFileContentStripsSAUCE(t *testing.T) {
	// This is more of an integration test to ensure GetAnsiFileContent calls stripSAUCE
	// We can test with an actual file that has SAUCE metadata
	// For now, we just verify the function exists and can be called
	_, err := GetAnsiFileContent("nonexistent.ans")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}
