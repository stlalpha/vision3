package terminal

import (
	"strings"
	"unicode/utf8"
)

// CharsetType represents different character encoding types
type CharsetType int

const (
	CharsetCP437 CharsetType = iota // IBM Code Page 437 (DOS)
	CharsetISO88591                 // ISO 8859-1 (Latin-1)
	CharsetUTF8                     // UTF-8 Unicode
	CharsetKOI8R                    // KOI8-R (Russian)
	CharsetAmiga                    // Amiga character set
	CharsetATASCII                  // Atari ASCII
)

// CP437ToUnicodeTable provides the complete CP437 to Unicode mapping
// This is the authentic DOS/BBS character set used in the 1980s-1990s
var CP437ToUnicodeTable = [256]rune{
	// 0x00-0x1F: Control characters (some with special symbols)
	0x0000, 0x263A, 0x263B, 0x2665, 0x2666, 0x2663, 0x2660, 0x2022,
	0x25D8, 0x25CB, 0x25D9, 0x2642, 0x2640, 0x266A, 0x266B, 0x263C,
	0x25BA, 0x25C4, 0x2195, 0x203C, 0x00B6, 0x00A7, 0x25AC, 0x21A8,
	0x2191, 0x2193, 0x2192, 0x2190, 0x221F, 0x2194, 0x25B2, 0x25BC,

	// 0x20-0x7F: Standard ASCII
	0x0020, 0x0021, 0x0022, 0x0023, 0x0024, 0x0025, 0x0026, 0x0027,
	0x0028, 0x0029, 0x002A, 0x002B, 0x002C, 0x002D, 0x002E, 0x002F,
	0x0030, 0x0031, 0x0032, 0x0033, 0x0034, 0x0035, 0x0036, 0x0037,
	0x0038, 0x0039, 0x003A, 0x003B, 0x003C, 0x003D, 0x003E, 0x003F,
	0x0040, 0x0041, 0x0042, 0x0043, 0x0044, 0x0045, 0x0046, 0x0047,
	0x0048, 0x0049, 0x004A, 0x004B, 0x004C, 0x004D, 0x004E, 0x004F,
	0x0050, 0x0051, 0x0052, 0x0053, 0x0054, 0x0055, 0x0056, 0x0057,
	0x0058, 0x0059, 0x005A, 0x005B, 0x005C, 0x005D, 0x005E, 0x005F,
	0x0060, 0x0061, 0x0062, 0x0063, 0x0064, 0x0065, 0x0066, 0x0067,
	0x0068, 0x0069, 0x006A, 0x006B, 0x006C, 0x006D, 0x006E, 0x006F,
	0x0070, 0x0071, 0x0072, 0x0073, 0x0074, 0x0075, 0x0076, 0x0077,
	0x0078, 0x0079, 0x007A, 0x007B, 0x007C, 0x007D, 0x007E, 0x2302,

	// 0x80-0xFF: Extended characters (accented letters, symbols, box drawing)
	0x00C7, 0x00FC, 0x00E9, 0x00E2, 0x00E4, 0x00E0, 0x00E5, 0x00E7,
	0x00EA, 0x00EB, 0x00E8, 0x00EF, 0x00EE, 0x00EC, 0x00C4, 0x00C5,
	0x00C9, 0x00E6, 0x00C6, 0x00F4, 0x00F6, 0x00F2, 0x00FB, 0x00F9,
	0x00FF, 0x00D6, 0x00DC, 0x00A2, 0x00A3, 0x00A5, 0x20A7, 0x0192,
	0x00E1, 0x00ED, 0x00F3, 0x00FA, 0x00F1, 0x00D1, 0x00AA, 0x00BA,
	0x00BF, 0x2310, 0x00AC, 0x00BD, 0x00BC, 0x00A1, 0x00AB, 0x00BB,
	0x2591, 0x2592, 0x2593, 0x2502, 0x2524, 0x2561, 0x2562, 0x2556,
	0x2555, 0x2563, 0x2551, 0x2557, 0x255D, 0x255C, 0x255B, 0x2510,
	0x2514, 0x2534, 0x252C, 0x251C, 0x2500, 0x253C, 0x255E, 0x255F,
	0x255A, 0x2554, 0x2569, 0x2566, 0x2560, 0x2550, 0x256C, 0x2567,
	0x2568, 0x2564, 0x2565, 0x2559, 0x2558, 0x2552, 0x2553, 0x256B,
	0x256A, 0x2518, 0x250C, 0x2588, 0x2584, 0x258C, 0x2590, 0x2580,
	0x03B1, 0x00DF, 0x0393, 0x03C0, 0x03A3, 0x03C3, 0x00B5, 0x03C4,
	0x03A6, 0x0398, 0x03A9, 0x03B4, 0x221E, 0x03C6, 0x03B5, 0x2229,
	0x2261, 0x00B1, 0x2265, 0x2264, 0x2320, 0x2321, 0x00F7, 0x2248,
	0x00B0, 0x2219, 0x00B7, 0x221A, 0x207F, 0x00B2, 0x25A0, 0x00A0,
}

// VT100LineDrawingTable maps CP437 box drawing characters to VT100 equivalents
var VT100LineDrawingTable = map[rune]rune{
	0x2500: 'q', // Horizontal line
	0x2502: 'x', // Vertical line
	0x250C: 'l', // Top-left corner
	0x2510: 'k', // Top-right corner
	0x2514: 'm', // Bottom-left corner
	0x2518: 'j', // Bottom-right corner
	0x251C: 't', // Left tee
	0x2524: 'u', // Right tee
	0x252C: 'w', // Top tee
	0x2534: 'v', // Bottom tee
	0x253C: 'n', // Cross
	0x2591: 'a', // Light shade
	0x2592: 'a', // Medium shade (approximation)
	0x2593: 'a', // Dark shade (approximation)
	0x25A0: 'a', // Solid block (approximation)
}

// ASCIIFallbackTable provides ASCII fallbacks for CP437 characters
var ASCIIFallbackTable = map[rune]rune{
	// Box drawing characters
	0x2500: '-', // Horizontal line
	0x2502: '|', // Vertical line
	0x250C: '+', // Top-left corner
	0x2510: '+', // Top-right corner
	0x2514: '+', // Bottom-left corner
	0x2518: '+', // Bottom-right corner
	0x251C: '+', // Left tee
	0x2524: '+', // Right tee
	0x252C: '+', // Top tee
	0x2534: '+', // Bottom tee
	0x253C: '+', // Cross
	
	// Double line box drawing
	0x2550: '=', // Double horizontal
	0x2551: '|', // Double vertical
	0x2554: '+', // Double top-left
	0x2557: '+', // Double top-right
	0x255A: '+', // Double bottom-left
	0x255D: '+', // Double bottom-right
	0x2560: '+', // Double left tee
	0x2563: '+', // Double right tee
	0x2566: '+', // Double top tee
	0x2569: '+', // Double bottom tee
	0x256C: '+', // Double cross
	
	// Shade characters
	0x2591: '.', // Light shade
	0x2592: ':', // Medium shade
	0x2593: '#', // Dark shade
	0x2588: '#', // Full block
	0x2584: '_', // Lower half block
	0x258C: '|', // Left half block
	0x2590: '|', // Right half block
	0x2580: '^', // Upper half block
	
	// Special characters
	0x263A: ':', // Smiling face
	0x263B: ':', // Frowning face
	0x2665: '*', // Heart
	0x2666: '*', // Diamond
	0x2663: '*', // Club
	0x2660: '*', // Spade
	0x2022: '*', // Bullet
	0x25CB: 'o', // Circle
	0x25BA: '>', // Right triangle
	0x25C4: '<', // Left triangle
	0x2195: '|', // Up-down arrow
	0x2191: '^', // Up arrow
	0x2193: 'v', // Down arrow
	0x2192: '>', // Right arrow
	0x2190: '<', // Left arrow
	0x266A: '?', // Music note
	0x266B: '?', // Music notes
}

// UnicodeToCP437Table provides Unicode to CP437 mapping for encoding
// This is generated from the CP437ToUnicodeTable for reverse lookups
var UnicodeToCP437Table map[rune]byte

// Cp437ToUnicode provides an alias for compatibility (exported for backward compatibility)
var Cp437ToUnicode = CP437ToUnicodeTable

// AmigaToUnicodeTable provides the Amiga character set to Unicode mapping
// Based on the Amiga's custom character set used in BBS and terminal applications
var AmigaToUnicodeTable = [256]rune{
	// 0x00-0x1F: Control characters (same as ASCII)
	0x0000, 0x0001, 0x0002, 0x0003, 0x0004, 0x0005, 0x0006, 0x0007,
	0x0008, 0x0009, 0x000A, 0x000B, 0x000C, 0x000D, 0x000E, 0x000F,
	0x0010, 0x0011, 0x0012, 0x0013, 0x0014, 0x0015, 0x0016, 0x0017,
	0x0018, 0x0019, 0x001A, 0x001B, 0x001C, 0x001D, 0x001E, 0x001F,

	// 0x20-0x7F: Standard ASCII
	0x0020, 0x0021, 0x0022, 0x0023, 0x0024, 0x0025, 0x0026, 0x0027,
	0x0028, 0x0029, 0x002A, 0x002B, 0x002C, 0x002D, 0x002E, 0x002F,
	0x0030, 0x0031, 0x0032, 0x0033, 0x0034, 0x0035, 0x0036, 0x0037,
	0x0038, 0x0039, 0x003A, 0x003B, 0x003C, 0x003D, 0x003E, 0x003F,
	0x0040, 0x0041, 0x0042, 0x0043, 0x0044, 0x0045, 0x0046, 0x0047,
	0x0048, 0x0049, 0x004A, 0x004B, 0x004C, 0x004D, 0x004E, 0x004F,
	0x0050, 0x0051, 0x0052, 0x0053, 0x0054, 0x0055, 0x0056, 0x0057,
	0x0058, 0x0059, 0x005A, 0x005B, 0x005C, 0x005D, 0x005E, 0x005F,
	0x0060, 0x0061, 0x0062, 0x0063, 0x0064, 0x0065, 0x0066, 0x0067,
	0x0068, 0x0069, 0x006A, 0x006B, 0x006C, 0x006D, 0x006E, 0x006F,
	0x0070, 0x0071, 0x0072, 0x0073, 0x0074, 0x0075, 0x0076, 0x0077,
	0x0078, 0x0079, 0x007A, 0x007B, 0x007C, 0x007D, 0x007E, 0x007F,

	// 0x80-0xFF: Amiga-specific extended characters
	// Based on Amiga's ISO 8859-1 with some modifications for graphics
	0x00C7, 0x00FC, 0x00E9, 0x00E2, 0x00E4, 0x00E0, 0x00E5, 0x00E7,
	0x00EA, 0x00EB, 0x00E8, 0x00EF, 0x00EE, 0x00EC, 0x00C4, 0x00C5,
	0x00C9, 0x00E6, 0x00C6, 0x00F4, 0x00F6, 0x00F2, 0x00FB, 0x00F9,
	0x00FF, 0x00D6, 0x00DC, 0x00A2, 0x00A3, 0x00A5, 0x20A7, 0x0192,
	0x00E1, 0x00ED, 0x00F3, 0x00FA, 0x00F1, 0x00D1, 0x00AA, 0x00BA,
	0x00BF, 0x2310, 0x00AC, 0x00BD, 0x00BC, 0x00A1, 0x00AB, 0x00BB,
	// Amiga-specific box drawing and special characters
	0x2591, 0x2592, 0x2593, 0x2502, 0x2524, 0x2561, 0x2562, 0x2556,
	0x2555, 0x2563, 0x2551, 0x2557, 0x255D, 0x255C, 0x255B, 0x2510,
	0x2514, 0x2534, 0x252C, 0x251C, 0x2500, 0x253C, 0x255E, 0x255F,
	0x255A, 0x2554, 0x2569, 0x2566, 0x2560, 0x2550, 0x256C, 0x2567,
	0x2568, 0x2564, 0x2565, 0x2559, 0x2558, 0x2552, 0x2553, 0x256B,
	0x256A, 0x2518, 0x250C, 0x2588, 0x2584, 0x258C, 0x2590, 0x2580,
	// More Amiga characters
	0x03B1, 0x00DF, 0x0393, 0x03C0, 0x03A3, 0x03C3, 0x00B5, 0x03C4,
	0x03A6, 0x0398, 0x03A9, 0x03B4, 0x221E, 0x03C6, 0x03B5, 0x2229,
	0x2261, 0x00B1, 0x2265, 0x2264, 0x2320, 0x2321, 0x00F7, 0x2248,
	0x00B0, 0x2219, 0x00B7, 0x221A, 0x207F, 0x00B2, 0x25A0, 0x00A0,
}

// UnicodeToAmigaTable provides Unicode to Amiga mapping for encoding
var UnicodeToAmigaTable map[rune]byte

func init() {
	// Build the reverse mapping table for CP437
	UnicodeToCP437Table = make(map[rune]byte, 256)
	for i, unicode := range CP437ToUnicodeTable {
		UnicodeToCP437Table[unicode] = byte(i)
	}
	
	// Build the reverse mapping table for Amiga
	UnicodeToAmigaTable = make(map[rune]byte, 256)
	for i, unicode := range AmigaToUnicodeTable {
		UnicodeToAmigaTable[unicode] = byte(i)
	}
}

// CharsetHandler manages character encoding conversion and font handling
type CharsetHandler struct {
	currentCharset CharsetType
	fallbackMode   bool // When true, use ASCII fallbacks
	vt100Mode      bool // When true, use VT100 line drawing
}

// NewCharsetHandler creates a new character set handler
func NewCharsetHandler() *CharsetHandler {
	return &CharsetHandler{
		currentCharset: CharsetCP437,
		fallbackMode:   false,
		vt100Mode:      false,
	}
}

// SetCharset changes the current character set
func (c *CharsetHandler) SetCharset(charset CharsetType) {
	c.currentCharset = charset
}

// SetFallbackMode enables/disables ASCII fallback mode
func (c *CharsetHandler) SetFallbackMode(enabled bool) {
	c.fallbackMode = enabled
}

// SetVT100Mode enables/disables VT100 line drawing mode
func (c *CharsetHandler) SetVT100Mode(enabled bool) {
	c.vt100Mode = enabled
}

// ConvertCP437ToUTF8 converts CP437 bytes to UTF-8 string
func (c *CharsetHandler) ConvertCP437ToUTF8(data []byte) string {
	var result strings.Builder
	result.Grow(len(data) * 2) // Estimate size
	
	for _, b := range data {
		unicode := CP437ToUnicodeTable[b]
		
		if c.vt100Mode {
			if vt100Char, exists := VT100LineDrawingTable[unicode]; exists {
				// Use VT100 line drawing sequence
				result.WriteString("\x0E") // Shift Out (SO) - enter line drawing mode
				result.WriteRune(vt100Char)
				result.WriteString("\x0F") // Shift In (SI) - exit line drawing mode
				continue
			}
		}
		
		if c.fallbackMode {
			if fallback, exists := ASCIIFallbackTable[unicode]; exists {
				result.WriteRune(fallback)
				continue
			}
		}
		
		result.WriteRune(unicode)
	}
	
	return result.String()
}

// ConvertCP437ByteToUTF8 converts a single CP437 byte to UTF-8 rune
func (c *CharsetHandler) ConvertCP437ByteToUTF8(b byte) rune {
	unicode := CP437ToUnicodeTable[b]
	
	if c.fallbackMode {
		if fallback, exists := ASCIIFallbackTable[unicode]; exists {
			return fallback
		}
	}
	
	return unicode
}

// ProcessPipeCodes processes ViSiON/2 style pipe codes in data
func (c *CharsetHandler) ProcessPipeCodes(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	
	result := make([]byte, 0, len(data)*2) // Estimate expanded size
	i := 0
	
	for i < len(data) {
		if data[i] == '|' && i+2 < len(data) {
			// Check for pipe code pattern |XX
			code := string(data[i+1 : i+3])
			if ansiSeq := c.pipeCodeToANSI(code); ansiSeq != "" {
				result = append(result, []byte(ansiSeq)...)
				i += 3 // Skip the pipe code
				continue
			}
		}
		result = append(result, data[i])
		i++
	}
	
	return result
}

// pipeCodeToANSI converts pipe codes to ANSI escape sequences
func (c *CharsetHandler) pipeCodeToANSI(code string) string {
	switch code {
	// Foreground colors (|00-|15)
	case "00":
		return "\x1b[30m" // Black
	case "01":
		return "\x1b[34m" // Blue
	case "02":
		return "\x1b[32m" // Green
	case "03":
		return "\x1b[36m" // Cyan
	case "04":
		return "\x1b[31m" // Red
	case "05":
		return "\x1b[35m" // Magenta
	case "06":
		return "\x1b[33m" // Brown/Yellow
	case "07":
		return "\x1b[37m" // Light Gray
	case "08":
		return "\x1b[1;30m" // Dark Gray (bright black)
	case "09":
		return "\x1b[1;34m" // Light Blue
	case "10":
		return "\x1b[1;32m" // Light Green
	case "11":
		return "\x1b[1;36m" // Light Cyan
	case "12":
		return "\x1b[1;31m" // Light Red
	case "13":
		return "\x1b[1;35m" // Light Magenta
	case "14":
		return "\x1b[1;33m" // Yellow
	case "15":
		return "\x1b[1;37m" // White

	// Background colors (|B0-|B7)
	case "B0":
		return "\x1b[40m" // Black background
	case "B1":
		return "\x1b[44m" // Blue background
	case "B2":
		return "\x1b[42m" // Green background
	case "B3":
		return "\x1b[46m" // Cyan background
	case "B4":
		return "\x1b[41m" // Red background
	case "B5":
		return "\x1b[45m" // Magenta background
	case "B6":
		return "\x1b[43m" // Yellow background
	case "B7":
		return "\x1b[47m" // White background

	// Special codes
	case "RS":
		return "\x1b[0m" // Reset all attributes
	case "CL":
		return "\x1b[2J\x1b[H" // Clear screen and home cursor
	case "CR":
		return "\r" // Carriage return
	case "LF":
		return "\n" // Line feed
	case "BL":
		return "\x1b[5m" // Blink
	case "RV":
		return "\x1b[7m" // Reverse video
	}
	
	return "" // Unknown pipe code
}

// StripAnsi removes ANSI escape sequences from text
func (c *CharsetHandler) StripAnsi(text string) string {
	var result strings.Builder
	inEscape := false
	inCSI := false
	
	for _, r := range text {
		if r == '\x1b' { // ESC character
			inEscape = true
			continue
		}
		
		if inEscape {
			if r == '[' {
				inCSI = true
				continue
			} else if !inCSI {
				inEscape = false
				result.WriteRune(r)
				continue
			}
		}
		
		if inCSI {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				// End of CSI sequence
				inCSI = false
				inEscape = false
				continue
			}
			// Skip CSI parameters and intermediate characters
			continue
		}
		
		if !inEscape && !inCSI {
			result.WriteRune(r)
		}
	}
	
	return result.String()
}

// ValidateUTF8 checks if data is valid UTF-8 and optionally repairs it
func (c *CharsetHandler) ValidateUTF8(data []byte, repair bool) ([]byte, bool) {
	if utf8.Valid(data) {
		return data, true
	}
	
	if !repair {
		return data, false
	}
	
	// Attempt to repair by converting invalid sequences
	result := make([]byte, 0, len(data))
	for i := 0; i < len(data); {
		if utf8.RuneStart(data[i]) {
			r, size := utf8.DecodeRune(data[i:])
			if r != utf8.RuneError {
				result = append(result, data[i:i+size]...)
				i += size
				continue
			}
		}
		
		// Invalid UTF-8, treat as CP437
		r := c.ConvertCP437ByteToUTF8(data[i])
		utf8Bytes := make([]byte, 4)
		n := utf8.EncodeRune(utf8Bytes, r)
		result = append(result, utf8Bytes[:n]...)
		i++
	}
	
	return result, true
}

// GetCharsetInfo returns information about the current character set
func (c *CharsetHandler) GetCharsetInfo() (CharsetType, string) {
	names := map[CharsetType]string{
		CharsetCP437:    "IBM CP437 (DOS)",
		CharsetISO88591: "ISO 8859-1 (Latin-1)",
		CharsetUTF8:     "UTF-8 Unicode",
		CharsetKOI8R:    "KOI8-R (Russian)",
		CharsetAmiga:    "Amiga",
		CharsetATASCII:  "ATASCII",
	}
	
	return c.currentCharset, names[c.currentCharset]
}

// IsBoxDrawingCharacter checks if a rune is a box drawing character
func (c *CharsetHandler) IsBoxDrawingCharacter(r rune) bool {
	_, exists := VT100LineDrawingTable[r]
	return exists
}

// ConvertToVT100LineDrawing converts Unicode box drawing to VT100 sequences
func (c *CharsetHandler) ConvertToVT100LineDrawing(text string) string {
	var result strings.Builder
	inDrawingMode := false
	
	for _, r := range text {
		if vt100Char, exists := VT100LineDrawingTable[r]; exists {
			if !inDrawingMode {
				result.WriteString("\x0E") // Enter line drawing mode
				inDrawingMode = true
			}
			result.WriteRune(vt100Char)
		} else {
			if inDrawingMode {
				result.WriteString("\x0F") // Exit line drawing mode
				inDrawingMode = false
			}
			result.WriteRune(r)
		}
	}
	
	if inDrawingMode {
		result.WriteString("\x0F") // Ensure we exit line drawing mode
	}
	
	return result.String()
}

// ConvertAmigaToUTF8 converts Amiga bytes to UTF-8 string
func (c *CharsetHandler) ConvertAmigaToUTF8(data []byte) string {
	var result strings.Builder
	result.Grow(len(data) * 2) // Estimate size
	
	for _, b := range data {
		unicode := AmigaToUnicodeTable[b]
		
		if c.fallbackMode {
			if fallback, exists := ASCIIFallbackTable[unicode]; exists {
				result.WriteRune(fallback)
				continue
			}
		}
		
		result.WriteRune(unicode)
	}
	
	return result.String()
}

// ConvertAmigaByteToUTF8 converts a single Amiga byte to UTF-8 rune
func (c *CharsetHandler) ConvertAmigaByteToUTF8(b byte) rune {
	unicode := AmigaToUnicodeTable[b]
	
	if c.fallbackMode {
		if fallback, exists := ASCIIFallbackTable[unicode]; exists {
			return fallback
		}
	}
	
	return unicode
}

// ConvertUTF8ToAmiga converts UTF-8 string to Amiga bytes
func (c *CharsetHandler) ConvertUTF8ToAmiga(text string) []byte {
	result := make([]byte, 0, len(text))
	
	for _, r := range text {
		if amigaByte, exists := UnicodeToAmigaTable[r]; exists {
			result = append(result, amigaByte)
		} else if r < 256 {
			// For unmapped characters, try direct byte conversion
			result = append(result, byte(r))
		} else {
			// For characters that can't be represented, use a placeholder
			result = append(result, '?')
		}
	}
	
	return result
}

// ProcessAmigaContent processes content for Amiga terminals
func (c *CharsetHandler) ProcessAmigaContent(data []byte) []byte {
	if c.currentCharset != CharsetAmiga {
		return data
	}
	
	// Process pipe codes first
	processed := c.ProcessPipeCodes(data)
	
	// Convert any special Amiga sequences
	return c.processAmigaSpecialCodes(processed)
}

// processAmigaSpecialCodes handles Amiga-specific escape sequences and codes
func (c *CharsetHandler) processAmigaSpecialCodes(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	
	result := make([]byte, 0, len(data)*2)
	i := 0
	
	for i < len(data) {
		// Check for Amiga-specific escape sequences
		if data[i] == '\x1b' && i+1 < len(data) {
			// Handle Amiga font change sequences
			if data[i+1] == 'F' && i+2 < len(data) {
				// \x1bF<font> - Amiga font change sequence
				fontCode := data[i+2]
				switch fontCode {
				case '0':
					// Topaz 8 (default Amiga font)
					result = append(result, []byte("\x1b]50;Topaz\x07")...)
				case '1':
					// Topaz 11
					result = append(result, []byte("\x1b]50;Topaz11\x07")...)
				case '2':
					// Microknight
					result = append(result, []byte("\x1b]50;Microknight\x07")...)
				case '3':
					// MicroKnight+
					result = append(result, []byte("\x1b]50;MicroKnight+\x07")...)
				default:
					// Unknown font, keep original
					result = append(result, data[i:i+3]...)
				}
				i += 3
				continue
			}
			
			// Handle Amiga color sequences (different from ANSI)
			if data[i+1] == 'c' && i+2 < len(data) {
				// \x1bc<color> - Amiga color change
				colorCode := data[i+2]
				if colorCode >= '0' && colorCode <= '9' {
					// Convert Amiga color to ANSI
					ansiColor := c.amigaColorToANSI(colorCode)
					result = append(result, []byte(ansiColor)...)
					i += 3
					continue
				}
			}
		}
		
		// Regular character, copy as-is
		result = append(result, data[i])
		i++
	}
	
	return result
}

// amigaColorToANSI converts Amiga color codes to ANSI escape sequences
func (c *CharsetHandler) amigaColorToANSI(colorCode byte) string {
	switch colorCode {
	case '0':
		return "\x1b[30m" // Black
	case '1':
		return "\x1b[34m" // Blue
	case '2':
		return "\x1b[32m" // Green
	case '3':
		return "\x1b[36m" // Cyan
	case '4':
		return "\x1b[31m" // Red
	case '5':
		return "\x1b[35m" // Magenta
	case '6':
		return "\x1b[33m" // Yellow/Brown
	case '7':
		return "\x1b[37m" // White
	case '8':
		return "\x1b[1;30m" // Bright Black (Dark Gray)
	case '9':
		return "\x1b[1;34m" // Bright Blue
	default:
		return "\x1b[0m" // Reset
	}
}