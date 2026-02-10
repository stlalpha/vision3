package jam

import (
	"hash/crc32"
	"strings"
)

// CRC32String calculates a JAM-specification CRC32 of a string.
// Per the JAM spec: lowercase only A-Z (not locale-aware), use IEEE
// polynomial, and invert the result.
func CRC32String(s string) uint32 {
	lower := strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return r
	}, s)

	table := crc32.MakeTable(crc32.IEEE)
	return ^crc32.Checksum([]byte(lower), table)
}
