package jam

import "testing"

func TestCRC32String(t *testing.T) {
	// Verify basic properties: same input = same output, case insensitive for A-Z
	c1 := CRC32String("hello")
	c2 := CRC32String("hello")
	if c1 != c2 {
		t.Error("same input should produce same CRC")
	}

	// A-Z case insensitivity
	c3 := CRC32String("Hello")
	c4 := CRC32String("HELLO")
	if c3 != c4 {
		t.Error("A-Z should be case insensitive")
	}

	// Different strings should produce different CRCs
	c5 := CRC32String("alice")
	c6 := CRC32String("bob")
	if c5 == c6 {
		t.Error("different strings should produce different CRCs")
	}

	// Empty string should produce a non-zero value (inverted CRC of empty)
	c7 := CRC32String("")
	if c7 == 0 {
		t.Error("empty string CRC should be non-zero (inverted)")
	}
}
