package jam

import (
	"hash/crc32"
	"strings"
)

// crcTable is the IEEE CRC32 table used by JAM, computed once at init time.
var crcTable = crc32.MakeTable(crc32.IEEE)

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

	return ^crc32.Checksum([]byte(lower), crcTable)
}
