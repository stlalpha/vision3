package main

import (
	"fmt"

	"github.com/stlalpha/vision3/internal/terminal"
)

func main() {
	fmt.Println("🧪 COMPREHENSIVE ANSI PATTERN TESTING")
	fmt.Println("=====================================")

	// Test patterns covering position AND content preservation
	testCases := []struct {
		name     string
		input    []byte
		expected string
		desc     string
	}{
		{
			name:     "Pure ANSI sequences",
			input:    []byte("\x1b[2J\x1b[1;1H\x1b[31mRed\x1b[0m"),
			expected: "\x1b[2J\x1b[1;1H\x1b[31mRed\x1b[0m",
			desc:     "Should pass through unchanged",
		},
		{
			name:     "ViSiON pipe codes",
			input:    []byte("|CL|04Red|RS"),
			expected: "\x1b[2J\x1b[H\x1b[31mRed\x1b[0m",
			desc:     "Should convert pipe codes to ANSI",
		},
		{
			name:     "UTF-8 box drawing",
			input:    []byte("█▄▀"),
			expected: "█▄▀",
			desc:     "Should preserve UTF-8 characters exactly",
		},
		{
			name:     "Mixed ANSI + UTF-8",
			input:    []byte("\x1b[1;1H███\x1b[31m\x1b[1;5HRED\x1b[0m\x1b[1;10H░▒▓"),
			expected: "\x1b[1;1H███\x1b[31m\x1b[1;5HRED\x1b[0m\x1b[1;10H░▒▓",
			desc:     "Should preserve both ANSI and UTF-8",
		},
		{
			name:     "Complex positioning",
			input:    []byte("\x1b[2J\x1b[1;20H\x1b[31m█\x1b[32m█\x1b[34m█\x1b[0m\n\x1b[2;20H\x1b[35m█\x1b[36m█\x1b[33m█\x1b[0m\n\x1b[3;20H\x1b[37m█\x1b[90m█\x1b[91m█"),
			expected: "\x1b[2J\x1b[1;20H\x1b[31m█\x1b[32m█\x1b[34m█\x1b[0m\n\x1b[2;20H\x1b[35m█\x1b[36m█\x1b[33m█\x1b[0m\n\x1b[3;20H\x1b[37m█\x1b[90m█\x1b[91m█",
			desc:     "Complex positioning with UTF-8 chars",
		},
	}

	// Test with UTF-8 output mode (the problematic one)
	writer := &TestWriter{}
	bbs := terminal.NewBBSFromWriter(writer, terminal.OutputModeUTF8)

	allPassed := true
	for i, tc := range testCases {
		fmt.Printf("\n📋 Test %d: %s\n", i+1, tc.name)
		fmt.Printf("   %s\n", tc.desc)
		
		// Reset writer
		writer.Reset()
		
		// Process content
		err := bbs.DisplayContent(tc.input)
		if err != nil {
			fmt.Printf("   ❌ ERROR: %v\n", err)
			allPassed = false
			continue
		}
		
		result := string(writer.data)
		
		// Check exact match
		if result == tc.expected {
			fmt.Printf("   ✅ PASS: Output matches expected exactly\n")
		} else {
			fmt.Printf("   ❌ FAIL: Output mismatch\n")
			fmt.Printf("      Expected: %q\n", tc.expected)
			fmt.Printf("      Got:      %q\n", result)
			fmt.Printf("      Expected bytes: %v\n", []byte(tc.expected))
			fmt.Printf("      Got bytes:      %v\n", writer.data)
			allPassed = false
		}
	}

	fmt.Printf("\n🎯 OVERALL RESULT: ")
	if allPassed {
		fmt.Printf("✅ ALL TESTS PASSED - Position AND content preserved!\n")
	} else {
		fmt.Printf("❌ SOME TESTS FAILED - Issues remain\n")
	}
}

// TestWriter captures written data for verification
type TestWriter struct {
	data []byte
}

func (w *TestWriter) Write(p []byte) (n int, err error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

func (w *TestWriter) Reset() {
	w.data = nil
}