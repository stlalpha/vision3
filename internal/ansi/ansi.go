package ansi

import (
	"bytes"
	"fmt"
	"log"
	"os" // Needed for chcp command
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	// NOTE: The original code used golang.org/x/text/encoding/charmap
	// The new code provides its own cp437ToUnicode map, so this might not be needed.
	// If other parts of the project rely on the init() function from the old
	// ansi.go that used charmap, this might need adjustment.
)

// OutputMode defines the character encoding strategy for terminal output.
type OutputMode int

const (
	OutputModeAuto  OutputMode = iota // Default: Detect based on TERM variable
	OutputModeUTF8                    // Force UTF-8 character output
	OutputModeCP437                   // Force raw CP437 byte output
)

// Map of CP437 box drawing bytes to their VT100 line drawing mode equivalents
var cp437ToVT100 = map[byte]byte{
	0xB3: 'x', // │ vertical line
	0xC4: 'q', // ─ horizontal line
	0xDA: 'l', // ┌ upper left corner
	0xC0: 'm', // └ lower left corner
	0xD9: 'j', // ┘ lower right corner
	0xBF: 'k', // ┐ upper right corner
	0xC3: 't', // ├ left tee
	0xB4: 'u', // ┤ right tee
	0xC2: 'w', // ┬ top tee
	0xC1: 'v', // ┴ bottom tee
	0xC5: 'n', // ┼ cross
	0xB1: 'a', // ▒ medium shade (approximate)
	0xB0: '`', // ░ light shade (approximate)
	0xB2: '0', // ▓ dark shade (approximate)
	0xDB: '0', // █ full block (approximate)
}

// This array maps CP437 bytes (0-255) to their Unicode equivalents
var Cp437ToUnicode = [256]rune{
	// ASCII characters (0-127)
	0x0000, 0x0001, 0x0002, 0x0003, 0x0004, 0x0005, 0x0006, 0x0007,
	0x0008, 0x0009, 0x000A, 0x000B, 0x000C, 0x000D, 0x000E, 0x000F,
	0x0010, 0x0011, 0x0012, 0x0013, 0x0014, 0x0015, 0x0016, 0x0017,
	0x0018, 0x0019, 0x001A, 0x001B, 0x001C, 0x001D, 0x001E, 0x001F,
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
	// Extended CP437 characters (128-255)
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

// Reverse map: Unicode Rune -> CP437 Byte (for specific characters needed)
// Generated carefully to avoid duplicates where multiple runes might map from one byte.
var UnicodeToCP437 = map[rune]byte{
	// Box drawing/blocks - Use the primary Unicode points
	'█': 0xDB, '▄': 0xDC, '▌': 0xDD, '▐': 0xDE, '▀': 0xDF,
	'■': 0xFE, // Solid square block
	'─': 0xC4, '│': 0xB3, '┌': 0xDA, '┐': 0xBF, '└': 0xC0, '┘': 0xD9,
	'├': 0xC3, '┤': 0xB4, '┬': 0xC2, '┴': 0xC1, '┼': 0xC5,
	'═': 0xCD, '║': 0xBA, '╔': 0xC9, '╗': 0xBB, '╚': 0xC8, '╝': 0xBC,
	'╠': 0xCC, '╣': 0xB9, '╦': 0xCB, '╩': 0xCA, '╬': 0xCE,
	'░': 0xB0, '▒': 0xB1, '▓': 0xB2,

	// Other common CP437 symbols
	'Ç': 0x80, 'ü': 0x81, 'é': 0x82, 'â': 0x83, 'ä': 0x84, 'à': 0x85, 'å': 0x86, 'ç': 0x87,
	'ê': 0x88, 'ë': 0x89, 'è': 0x8A, 'ï': 0x8B, 'î': 0x8C, 'ì': 0x8D, 'Ä': 0x8E, 'Å': 0x8F,
	'É': 0x90, 'æ': 0x91, 'Æ': 0x92, 'ô': 0x93, 'ö': 0x94, 'ò': 0x95, 'û': 0x96, 'ù': 0x97,
	'ÿ': 0x98, 'Ö': 0x99, 'Ü': 0x9A, '¢': 0x9B, '£': 0x9C, '¥': 0x9D, '₧': 0x9E, 'ƒ': 0x9F,
	'á': 0xA0, 'í': 0xA1, 'ó': 0xA2, 'ú': 0xA3, 'ñ': 0xA4, 'Ñ': 0xA5, 'ª': 0xA6, 'º': 0xA7,
	'¿': 0xA8, '⌐': 0xA9, '¬': 0xAA, '½': 0xAB, '¼': 0xAC, '¡': 0xAD, '«': 0xAE, '»': 0xAF,
	// 0xB0-B2 are box chars above
	// 0xB3-B4 are box chars above
	'╡': 0xB5, '╢': 0xB6, '╖': 0xB7, '╕': 0xB8,
	// 0xB9-C5 are box chars above
	'╞': 0xC6, '╟': 0xC7,
	// 0xC8-CE are box chars above
	'╧': 0xCF, '╨': 0xD0, '╤': 0xD1, '╥': 0xD2, '╙': 0xD3, '╘': 0xD4, '╒': 0xD5, '╓': 0xD6, '╫': 0xD7,
	'╪': 0xD8,
	// 0xD9-DF are box chars above
	'α': 0xE0, 'ß': 0xE1, 'Γ': 0xE2, 'π': 0xE3, 'Σ': 0xE4, 'σ': 0xE5, 'µ': 0xE6, 'τ': 0xE7,
	'Φ': 0xE8, 'Θ': 0xE9, 'Ω': 0xEA, 'δ': 0xEB, '∞': 0xEC, 'φ': 0xED, 'ε': 0xEE, '∩': 0xEF,
	'≡': 0xF0, '±': 0xF1, '≥': 0xF2, '≤': 0xF3, '⌠': 0xF4, '⌡': 0xF5, '÷': 0xF6, '≈': 0xF7,
	'°': 0xF8, '∙': 0xF9, '·': 0xFA, '√': 0xFB, 'ⁿ': 0xFC, '²': 0xFD,
	// 0xFE is ■ above
	// ' ': 0xFF, // Avoid mapping NBSP rune to 0xFF
}

// Common pipe code replacements map - Based on ViSiON/2 GenTypes.pas VColor array
var pipeCodeReplacements = map[string]string{
	// ViSiON/2 Color Codes (|00 - |15) - Standard DOS/CGA palette
	"|00": "\x1B[0;30m", // Black
	"|01": "\x1B[0;34m", // Blue
	"|02": "\x1B[0;32m", // Green
	"|03": "\x1B[0;36m", // Cyan
	"|04": "\x1B[0;31m", // Red
	"|05": "\x1B[0;35m", // Magenta
	"|06": "\x1B[0;33m", // Brown/Yellow
	"|07": "\x1B[0;37m", // Light Gray
	"|08": "\x1B[1;30m", // Dark Gray (Bright Black)
	"|09": "\x1B[1;34m", // Light Blue (Bright Blue)
	"|10": "\x1B[1;32m", // Light Green (Bright Green)
	"|11": "\x1B[1;36m", // Light Cyan (Bright Cyan)
	"|12": "\x1B[1;31m", // Light Red (Bright Red)
	"|13": "\x1B[1;35m", // Light Magenta (Bright Magenta)
	"|14": "\x1B[1;33m", // Yellow (Bright Yellow)
	"|15": "\x1B[1;37m", // White (Bright White)

	// Background Colors (|B0 - |B7)
	"|B0":  "\x1B[40m",               // Black BG
	"|B1":  "\x1B[41m",               // Red BG
	"|B2":  "\x1B[42m",               // Green BG
	"|B3":  "\x1B[43m",               // Brown BG
	"|B4":  "\x1B[44m",               // Blue BG
	"|B5":  "\x1B[45m",               // Magenta BG
	"|B6":  "\x1B[46m",               // Cyan BG
	"|B7":  "\x1B[47m",               // Gray BG (Standard White BG)
	"|B8":  "\x1B[100m",              // Bright Black BG
	"|B9":  "\x1B[101m",              // Bright Red BG
	"|B10": "\x1B[102m",              // Bright Green BG
	"|B11": "\x1B[103m",              // Bright Yellow BG
	"|B12": "\x1B[104m\x1B[48;5;12m", // Bright Blue BG (16-color + 256-color)
	"|B13": "\x1B[105m",              // Bright Magenta BG
	"|B14": "\x1B[106m",              // Bright Cyan BG
	"|B15": "\x1B[107m",              // Bright White BG

	// Other standard codes (using common ANSI)
	"|CL": "\x1B[2J\x1B[H", // Clear screen and home cursor
	"|CR": "\r\n",          // Carriage return + line feed
	"|DE": "\x1B[K",        // Clear to end of line
	"|P":  "\x1B[s",        // Save cursor position
	"|PP": "\x1B[u",        // Restore cursor position
	"|23": "\x1B[0m",       // Reset attributes
}

// GetAnsiFileContent reads an ANSI file and returns its raw byte content.
// It replaces the previous DisplayAnsiFile which incorrectly wrote directly.
// SAUCE metadata (if present) is automatically stripped from the returned content.
func GetAnsiFileContent(filename string) ([]byte, error) {
	log.Printf("DEBUG: Reading ANSI file content for %s", filename)

	// Read the file
	data, err := os.ReadFile(filename)
	if err != nil {
		// Return error to caller to handle (e.g., display error message)
		return nil, fmt.Errorf("failed to read ansi file %s: %w", filename, err)
	}

	// Strip SAUCE metadata if present
	data = stripSAUCE(data)

	// Return the raw data. Caller is responsible for processing/displaying.
	return data, nil
}

// stripSAUCE removes SAUCE metadata from ANSI art file content.
// SAUCE (Standard Architecture for Universal Comment Extensions) is a 128-byte
// metadata record appended to the end of ANSI art files, preceded by an EOF marker (0x1A).
// This function detects and removes the SAUCE record and EOF marker to prevent
// the metadata from being displayed as garbage characters.
func stripSAUCE(data []byte) []byte {
	const sauceSize = 128
	const eofMarker = 0x1A

	// SAUCE record must be at least 128 bytes
	if len(data) < sauceSize {
		return data
	}

	// Check if the last 128 bytes start with "SAUCE"
	sauceStart := len(data) - sauceSize
	if !bytes.HasPrefix(data[sauceStart:], []byte("SAUCE")) {
		// No SAUCE metadata present
		return data
	}

	log.Printf("DEBUG: SAUCE metadata detected, stripping from content")

	// Search backwards from the SAUCE record to find the EOF marker (0x1A)
	// The EOF marker should be immediately before the SAUCE record,
	// but there may be comment blocks in between.
	eofPos := -1
	for i := sauceStart - 1; i >= 0; i-- {
		if data[i] == eofMarker {
			eofPos = i
			break
		}
		// Don't search too far back (safety limit: 64KB of comments max)
		if sauceStart-i > 65536 {
			break
		}
	}

	// If we found the EOF marker, return content up to (but not including) it
	if eofPos >= 0 {
		log.Printf("DEBUG: Found EOF marker at position %d, trimming content", eofPos)
		return data[:eofPos]
	}

	// If no EOF marker found, just remove the SAUCE record itself
	// This handles malformed SAUCE or files without the EOF marker
	log.Printf("DEBUG: No EOF marker found, removing SAUCE record only")
	return data[:sauceStart]
}

// CP437BytesToUTF8 converts raw CP437 bytes to UTF-8, preserving ANSI escape
// sequences. Use this before doing string operations (e.g. placeholder
// substitution) on ANS file content so that the writer pipeline receives
// unambiguous UTF-8 instead of raw CP437 bytes that might accidentally form
// valid UTF-8 sequences and get misinterpreted.
func CP437BytesToUTF8(data []byte) []byte {
	out := make([]byte, 0, len(data)*2)
	i := 0
	for i < len(data) {
		b := data[i]

		// Pass ANSI escape sequences through untouched.
		if b == 0x1B && i+1 < len(data) {
			start := i
			i++ // skip ESC
			if data[i] == '[' {
				i++ // skip '['
				for i < len(data) {
					c := data[i]
					i++
					if c >= '@' && c <= '~' {
						break
					}
					if i-start > 32 {
						break
					}
				}
			} else if data[i] == '(' || data[i] == ')' {
				i++ // skip charset designator
				if i < len(data) {
					i++ // skip charset ID
				}
			} else {
				i++ // simple two-byte ESC sequence
			}
			out = append(out, data[start:i]...)
			continue
		}

		// ASCII bytes pass through as-is.
		if b < 0x80 {
			out = append(out, b)
			i++
			continue
		}

		// High bytes: convert CP437 → Unicode → UTF-8.
		r := Cp437ToUnicode[b]
		out = append(out, []byte(string(r))...)
		i++
	}
	return out
}

// Strategy 1: Use VT100 Line Drawing Mode
func DisplayWithVT100LineDrawing(session ssh.Session, filename string) error {
	data, err := GetAnsiFileContent(filename)
	if err != nil {
		return err
	}

	// Create a buffer for processing
	var buf bytes.Buffer

	// First, replace all pipe codes
	processedData := ReplacePipeCodes(data)

	// Process each byte
	inVT100Mode := false // Track if we are currently in VT100 drawing mode

	switchToVT100 := func() {
		if !inVT100Mode {
			buf.WriteString("\x1B(0") // Switch to line drawing character set
			inVT100Mode = true
		}
	}
	switchToASCII := func() {
		if inVT100Mode {
			buf.WriteString("\x1B(B") // Switch back to normal character set
			inVT100Mode = false
		}
	}

	defer switchToASCII() // Ensure we switch back at the end

	for i := 0; i < len(processedData); i++ {
		b := processedData[i]

		// Handle ANSI escape sequences
		if b == 0x1B && i+1 < len(processedData) && processedData[i+1] == '[' {
			switchToASCII()  // Switch back before writing ANSI sequence
			buf.WriteByte(b) // Write ESC
			i++
			buf.WriteByte(processedData[i]) // Write [
			i++
			// Find the end of the sequence (ends with an alphabetic character)
			seqEnd := i
			for seqEnd < len(processedData) {
				c := processedData[seqEnd]
				buf.WriteByte(c)
				seqEnd++
				if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
					break
				}
				// Prevent overly long sequences (safety)
				if seqEnd-i > 30 {
					break // Or handle error
				}
			}
			i = seqEnd - 1 // Update main loop index
			continue
		} else if b == 0x1B && i+1 < len(processedData) && (processedData[i+1] == '(' || processedData[i+1] == ')') {
			// Handle character set switching sequences separately if needed
			// For now, treat them like other escape sequences (pass through)
			switchToASCII()
			buf.WriteByte(b) // ESC
			i++
			buf.WriteByte(processedData[i]) // ( or )
			i++
			if i < len(processedData) {
				buf.WriteByte(processedData[i]) // Character set identifier (0, B, etc.)
			} else {
				// Incomplete sequence, handle appropriately (e.g., ignore, log)
				continue // Skip this incomplete sequence
			}
			// Assume sequence ends here for (0, (B etc.
			continue
		} else if b == 0x1B {
			// Simple ESC, maybe part of a different sequence or stray
			switchToASCII()
			buf.WriteByte(b) // Pass through simple ESC
			continue
		}

		// Regular character
		if vt100Char, ok := cp437ToVT100[b]; ok {
			// For box drawing characters, switch to VT100 mode and write the char
			switchToVT100()
			buf.WriteByte(vt100Char)
		} else {
			// For any other character (ASCII or other CP437), switch back to ASCII mode
			switchToASCII()
			if b < 128 {
				// ASCII passes through unchanged
				buf.WriteByte(b)
			} else {
				// For other CP437 extended characters, attempt Unicode conversion
				// If it's a simple 1-byte representation, use it, otherwise fallback
				r := Cp437ToUnicode[b]
				if r > 0 && r < 128 { // Check if it maps to standard ASCII
					buf.WriteByte(byte(r))
				} else {
					// Fallback for non-drawable, non-ASCII characters in this mode
					buf.WriteByte('?') // Or handle differently
				}
			}
		}
	}

	// Ensure we switch back to ASCII at the very end
	switchToASCII()

	// Write the buffer to the session
	_, err = session.Write(buf.Bytes())
	return err
}

// Strategy 2: Use ASCII Fallbacks
func DisplayWithASCIIFallback(session ssh.Session, filename string) error {
	data, err := GetAnsiFileContent(filename)
	if err != nil {
		return err
	}

	// Create a buffer for processing
	var buf bytes.Buffer

	// First, replace all pipe codes
	processedData := ReplacePipeCodes(data)

	// Map of CP437 box drawing characters to ASCII fallbacks
	asciiFallbacks := map[byte]string{
		0xB3: "|", // │ vertical line
		0xC4: "-", // ─ horizontal line
		0xDA: "+", // ┌ upper left corner
		0xC0: "+", // └ lower left corner
		0xD9: "+", // ┘ lower right corner
		0xBF: "+", // ┐ upper right corner
		0xC3: "+", // ├ left tee
		0xB4: "+", // ┤ right tee
		0xC2: "+", // ┬ top tee
		0xC1: "+", // ┴ bottom tee
		0xC5: "+", // ┼ cross
		0xB1: "#", // ▒ medium shade
		0xB0: ".", // ░ light shade
		0xB2: "%", // ▓ dark shade
		0xDB: "@", // █ full block
		// Add others if needed
	}

	// Process each byte
	for i := 0; i < len(processedData); i++ {
		b := processedData[i]

		// Handle ANSI escape sequences (pass through)
		if b == 0x1B && i+1 < len(processedData) {
			escStart := i
			// Determine end of sequence
			seqEnd := i + 1
			if processedData[seqEnd] == '[' { // CSI sequence
				seqEnd++ // Move past '['
				for seqEnd < len(processedData) {
					c := processedData[seqEnd]
					if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
						seqEnd++ // Include the final character
						break
					}
					seqEnd++
					if seqEnd-escStart > 30 {
						break
					}
				}
			} else if processedData[seqEnd] == '(' || processedData[seqEnd] == ')' { // Character set sequence
				seqEnd += 2 // Include ( or ) and the identifier
			} else {
				// Other potential short sequences (e.g., ESC c - Reset)
				seqEnd++ // Assume 1 char after ESC if not CSI or charset
			}

			// Ensure seqEnd does not exceed bounds
			if seqEnd > len(processedData) {
				seqEnd = len(processedData)
			}

			// Write the entire sequence
			buf.Write(processedData[escStart:seqEnd])
			i = seqEnd - 1 // Update loop index
			continue
		}

		// Regular character
		if b < 128 {
			// ASCII passes through unchanged
			buf.WriteByte(b)
		} else if fallback, ok := asciiFallbacks[b]; ok {
			// For known box drawing/block characters, use ASCII fallback
			buf.WriteString(fallback)
		} else {
			// For other CP437 extended characters, check if it maps to standard ASCII
			r := Cp437ToUnicode[b]
			if r > 0 && r < 128 {
				buf.WriteByte(byte(r))
			} else {
				// Multi-byte character or unmapped, use a simple fallback
				buf.WriteByte('?')
			}
		}
	}

	// Write the buffer to the session
	_, err = session.Write(buf.Bytes())
	return err
}

// Strategy 3: Use Standard UTF-8 Conversion
func DisplayWithStandardUTF8(session ssh.Session, filename string) error {
	data, err := GetAnsiFileContent(filename)
	if err != nil {
		return err
	}

	// First, replace all pipe codes
	processedData := ReplacePipeCodes(data)

	// Create a buffer for the output
	var buf bytes.Buffer

	// Process each byte
	for i := 0; i < len(processedData); i++ {
		b := processedData[i]

		// Handle ANSI escape sequences (pass through)
		if b == 0x1B && i+1 < len(processedData) {
			escStart := i
			seqEnd := i + 1
			if processedData[seqEnd] == '[' { // CSI
				seqEnd++
				for seqEnd < len(processedData) {
					c := processedData[seqEnd]
					if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
						seqEnd++
						break
					}
					seqEnd++
					if seqEnd-escStart > 30 {
						break
					}
				}
			} else if processedData[seqEnd] == '(' || processedData[seqEnd] == ')' { // Charset
				seqEnd += 2
			} else {
				seqEnd++
			}

			if seqEnd > len(processedData) {
				seqEnd = len(processedData)
			}
			buf.Write(processedData[escStart:seqEnd])
			i = seqEnd - 1
			continue
		}

		// Regular character
		if b < 128 {
			// ASCII passes through unchanged
			buf.WriteByte(b)
		} else {
			// For CP437 extended characters, convert to Unicode rune
			r := Cp437ToUnicode[b]
			if r == 0 && b != 0 {
				// Handle cases where the map might have a zero entry for a non-zero byte
				buf.WriteByte('?') // Fallback
			} else if r != 0 {
				// Encode the rune as UTF-8 and write to buffer
				buf.WriteRune(r)
			}
			// If r is 0 and b is 0, it's a null byte, often ignored or handled specially
			// Currently, null bytes are just skipped
		}
	}

	// Write the buffer to the session
	_, err = session.Write(buf.Bytes())
	return err
}

// Strategy 4: Use Raw Bytes (After Pipe Code Replacement)
func DisplayWithRawBytes(session ssh.Session, filename string) error {
	data, err := GetAnsiFileContent(filename)
	if err != nil {
		return err
	}

	// Replace pipe codes
	processedData := ReplacePipeCodes(data)

	// Write the raw processed data to the session
	_, err = session.Write(processedData)
	return err
}

// Helper function to replace pipe codes with ANSI sequences
// It should ONLY replace |XX codes and pass through all other bytes.
// Simplified version: Removed complex ||XX handling for now.
func ReplacePipeCodes(data []byte) []byte {
	var buf bytes.Buffer
	i := 0
	dataLen := len(data)

	for i < dataLen {
		// Escaped literal pipe: "||" -> "|"
		if data[i] == '|' && i+1 < dataLen && data[i+1] == '|' {
			buf.WriteByte('|')
			i += 2
			continue
		}

		// Check for potential |XX code
		if data[i] == '|' && i+2 < dataLen {
			if i+3 < dataLen {
				code := string(data[i : i+4])
				if replacement, ok := pipeCodeReplacements[code]; ok {
					// Valid |XXX code found, write ANSI replacement
					buf.WriteString(replacement)
					i += 4   // Advance past |XXX
					continue // Continue loop
				}
			}
			code := string(data[i : i+3])
			if replacement, ok := pipeCodeReplacements[code]; ok {
				// Valid |XX code found, write ANSI replacement
				buf.WriteString(replacement)
				i += 3   // Advance past |XX
				continue // Continue loop
			}
			// If not a valid |XX code, treat '|' as a literal byte
			// (fall through to default byte handling)
		}

		// Default: Write the current byte directly if it wasn't part of a valid |XX sequence
		buf.WriteByte(data[i])
		i++
	}
	return buf.Bytes()
}

// This function is a comprehensive solution for displaying CP437 ANSI art
// It tries multiple strategies and includes detailed debugging.
// **NOTE:** This function seems largely redundant with DisplayAnsiFile now.
// Keeping it as defined in the prompt, but consider merging/removing.
// The main difference is the direct VT100 attempt and more debug prints.
func DisplayComprehensiveSolution(session ssh.Session, filename string) error {
	// Print terminal info (optional, good for debugging)
	ptyReq, _, isPty := session.Pty()
	if isPty {
		fmt.Fprintf(session, "Terminal type: %s\r\n", ptyReq.Term)
		fmt.Fprintf(session, "Window size: %dx%d\r\n", ptyReq.Window.Width, ptyReq.Window.Height)
	} else {
		fmt.Fprintf(session, "Not a PTY session.\r\n")
	}

	// Try to negotiate UTF-8 mode with the terminal
	session.SendRequest("xterm-256color", false, []byte("TERM=xterm-256color"))
	session.SendRequest("utf8-mode", false, nil)
	time.Sleep(100 * time.Millisecond) // Allow time for processing

	// Optional: Send a test pattern (Commented out by default)
	/*
	   testPattern := "\r\nTesting Character Support:\r\n"
	   testPattern += "ASCII: ABCDEFGabcdefg123456!@#$\r\n"
	   testPattern += "Latin-1 (potential 2-byte UTF-8): ß ö ñ é è ü\r\n"
	   testPattern += "Box chars (potential 3-byte UTF-8): │ ┌ ┐ └ ┘ ─ ┼\r\n"
	   testPattern += "VT100 Test: \x1B(0x q l k j m t u w v n` a 0\x1B(B\r\n" // VT100 line chars + shades + block
	   testPattern += "Blocks (potential): █ ▓ ▒ ░\r\n"
	   fmt.Fprint(session, testPattern)
	   time.Sleep(500 * time.Millisecond) // Wait for the test pattern
	*/

	// Read the ANS file
	data, err := GetAnsiFileContent(filename)
	if err != nil {
		return fmt.Errorf("failed to read file '%s': %w", filename, err)
	}

	// Optional: Create a debug log file (Commented out by default)
	/*
	   debugLog, errLog := os.Create("ansi_debug.log")
	   if errLog == nil {
	       defer debugLog.Close()
	       fmt.Fprintf(debugLog, "Processing file: %s (%d bytes)\r\n", filename, len(data))
	   }
	*/

	// Primarily use the VT100 line drawing logic directly here for this "Comprehensive" function
	// Use Fprint and remove trailing \r\n from string
	fmt.Fprint(session, "\r\n--- DisplayComprehensiveSolution (VT100 Attempt) ---")

	var buf bytes.Buffer
	processedData := ReplacePipeCodes(data)

	inVT100Mode := false
	switchToVT100 := func() {
		if !inVT100Mode {
			buf.WriteString("\x1B(0")
			inVT100Mode = true
		}
	}
	switchToASCII := func() {
		if inVT100Mode {
			buf.WriteString("\x1B(B")
			inVT100Mode = false
		}
	}
	defer switchToASCII()

	for i := 0; i < len(processedData); i++ {
		b := processedData[i]

		// Optional Debug Logging
		/*
		   if debugLog != nil && b >= 128 {
		       fmt.Fprintf(debugLog, "Char at %d: %02X (%d)\r\n", i, b, b)
		   }
		*/

		// Handle ANSI escape sequences (pass through)
		if b == 0x1B && i+1 < len(processedData) {
			switchToASCII() // Ensure ASCII mode for escape sequences
			escStart := i
			seqEnd := i + 1
			if processedData[seqEnd] == '[' { // CSI
				seqEnd++
				for seqEnd < len(processedData) {
					c := processedData[seqEnd]
					if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
						seqEnd++
						break
					}
					seqEnd++
					if seqEnd-escStart > 30 {
						break
					}
				}
			} else if processedData[seqEnd] == '(' || processedData[seqEnd] == ')' { // Charset
				seqEnd += 2
			} else {
				seqEnd++
			}

			if seqEnd > len(processedData) {
				seqEnd = len(processedData)
			}
			buf.Write(processedData[escStart:seqEnd])
			/* // Optional Debug Logging
			if debugLog != nil {
			    fmt.Fprintf(debugLog, "ANSI escape: %q\r\n", processedData[escStart:seqEnd])
			}
			*/
			i = seqEnd - 1
			continue
		}

		// Regular character processing
		if vt100Char, ok := cp437ToVT100[b]; ok {
			// Use VT100 line drawing mode
			switchToVT100()
			buf.WriteByte(vt100Char)
			/* // Optional Debug Logging
			if debugLog != nil {
			    fmt.Fprintf(debugLog, "  -> CP437 %02X to VT100 %c\r\n", b, vt100Char)
			}
			*/
		} else {
			switchToASCII() // Use ASCII mode for non-VT100 chars
			if b < 128 {
				// Standard ASCII
				buf.WriteByte(b)
			} else {
				// Other CP437 extended char - use fallback '?' in this "comprehensive" mode
				// as it's primarily demonstrating the VT100 approach.
				buf.WriteByte('?')
				/* // Optional Debug Logging
				if debugLog != nil {
				    fmt.Fprintf(debugLog, "  -> CP437 %02X to fallback ?\r\n", b)
				}
				*/
			}
		}
	}

	switchToASCII() // Final switch back

	// Write the buffer to the session
	_, err = session.Write(buf.Bytes())
	if err != nil {
		fmt.Fprintf(session, "\r\nError writing to session: %v\r\n", err)
	}
	return err
}

// Add back essential functions if they were used externally and are missing.
// Check callers of the old `ansi` package.

// Example placeholder functions (if they were used externally):
func ClearScreen() string {
	return "\x1B[2J\x1B[H" // Clear screen and home cursor
}

// MoveCursor returns an ANSI escape sequence to move the cursor to the specified row and column.
// Rows and columns are 1-indexed (1,1 is top-left).
func MoveCursor(row, col int) string {
	return fmt.Sprintf("\x1B[%d;%dH", row, col)
}

// SaveCursor returns ANSI escape sequences to save the current cursor
// position. Both SCO (\x1b[s) and DEC (DECSC: \x1b7) forms are emitted
// so that the widest range of terminal emulators will honor at least one.
func SaveCursor() string {
	return "\x1B[s\x1B7"
}

// RestoreCursor returns ANSI escape sequences to restore the cursor to the
// previously saved position. Both SCO (\x1b[u) and DEC (DECRC: \x1b8)
// forms are emitted for broad compatibility.
func RestoreCursor() string {
	return "\x1B[u\x1B8"
}

// CursorBackward returns a CSI CUB sequence that moves the cursor left by n
// columns. This is universally supported and avoids reliance on cursor
// save/restore, which is inconsistent across terminal emulators.
func CursorBackward(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("\x1B[%dD", n)
}

// Add StripAnsi if it was used externally
func StripAnsi(str string) string {
	// Simple ANSI removal regex (may not cover all cases)
	// For more robust stripping, consider a dedicated library or more complex state machine.
	const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-Za-z~]))"
	// This regex is complex, for basic stripping:
	var result strings.Builder
	inEscape := false
	for i := 0; i < len(str); i++ {
		if str[i] == '\x1b' && i+1 < len(str) && str[i+1] == '[' {
			inEscape = true
			i++ // Skip '['
		} else if inEscape && (str[i] >= 'a' && str[i] <= 'z' || str[i] >= 'A' && str[i] <= 'Z') {
			inEscape = false
		} else if !inEscape {
			result.WriteByte(str[i])
		}
	}
	return result.String()
}

// If ProcessCodes or SubstitutePlaceholders were used, add them back or update callers.
// The new code focuses only on display, not general ANSI/Pipe code processing logic
// beyond simple color/position replacements needed for display.

// ProcessAnsiResult holds the results of processing an ANSI file, including field coordinates.
type ProcessAnsiResult struct {
	DisplayBytes []byte                                     // The processed bytes with standard ANSI escapes, ready for display.
	FieldCoords  map[string]struct{ X, Y int }              // Map of |XX or ~XX codes to their calculated coordinates.
	FieldColors  map[string]string                          // Map of |XX or ~XX codes to their ANSI color escape sequence at that position.
}

// ProcessAnsiAndExtractCoords processes raw CP437 byte content containing ViSiON/2 codes and ANSI escapes.
// It translates ViSiON codes to standard ANSI, handles cursor positioning,
// extracts coordinates for field codes (|XX, ~XX), strips specific codes,
// and returns the final byte slice for display and the coordinate map.
// Added outputMode parameter to control character encoding.
func ProcessAnsiAndExtractCoords(rawContent []byte, outputMode OutputMode) (ProcessAnsiResult, error) {
	result := ProcessAnsiResult{
		FieldCoords: make(map[string]struct{ X, Y int }),
		FieldColors: make(map[string]string),
	}
	var displayBuf bytes.Buffer

	// Normalize line endings: Replace \r\n with \n, handle standalone \r later
	normalizedContent := bytes.ReplaceAll(rawContent, []byte{0x0d, 0x0a}, []byte{0x0a})

	currentX := 1
	currentY := 1
	// Track cumulative SGR state for accurate color reproduction
	var sgrState struct {
		bold       bool
		dim        bool
		italic     bool
		underline  bool
		blink      bool
		reverse    bool
		hidden     bool
		foreground int // 0 = default, 30-37 = colors, 90-97 = bright colors
		background int // 0 = default, 40-47 = colors, 100-107 = bright colors
	}
	content := normalizedContent // Work directly with the byte slice

	// Helper to build current color sequence from SGR state
	buildColorSequence := func() string {
		if sgrState.foreground == 0 && sgrState.background == 0 && !sgrState.bold && !sgrState.dim && !sgrState.italic && !sgrState.underline && !sgrState.blink && !sgrState.reverse && !sgrState.hidden {
			return "" // No attributes set
		}
		var attrs []string
		if sgrState.bold {
			attrs = append(attrs, "1")
		}
		if sgrState.dim {
			attrs = append(attrs, "2")
		}
		if sgrState.italic {
			attrs = append(attrs, "3")
		}
		if sgrState.underline {
			attrs = append(attrs, "4")
		}
		if sgrState.blink {
			attrs = append(attrs, "5")
		}
		if sgrState.reverse {
			attrs = append(attrs, "7")
		}
		if sgrState.hidden {
			attrs = append(attrs, "8")
		}
		if sgrState.foreground > 0 {
			attrs = append(attrs, fmt.Sprintf("%d", sgrState.foreground))
		}
		if sgrState.background > 0 {
			attrs = append(attrs, fmt.Sprintf("%d", sgrState.background))
		}
		if len(attrs) == 0 {
			return ""
		}
		return fmt.Sprintf("\x1b[%sm", strings.Join(attrs, ";"))
	}

	i := 0
	for i < len(content) {
		b := content[i]
		consumed := 1 // Default consumption

		switch b {
		case 0x1b: // --- ANSI Escape Sequence (ESC) ---
			start := i
			if i+1 < len(content) && content[i+1] == '[' { // Check for CSI (Control Sequence Introducer)
				seqStart := i + 2
				params := []int{}
				currentParam := 0
				paramStarted := false
				terminatorIndex := -1
				terminator := byte(0)

				// Parse parameters and find terminator
				for j := seqStart; j < len(content); j++ {
					char := content[j]
					if char >= '0' && char <= '9' {
						currentParam = currentParam*10 + int(char-'0')
						paramStarted = true
					} else if char == ';' {
						params = append(params, currentParam)
						currentParam = 0
						paramStarted = false
					} else if char == '?' || char == '=' || char == '>' || char == '<' {
						// DEC private mode and other special CSI prefixes
						// These appear after '[' and before parameters (e.g., ESC[?7h)
						// Just skip them - they don't affect our coordinate tracking
						continue
					} else if char >= 0x40 && char <= 0x7e { // Found terminator
						terminatorIndex = j
						terminator = char
						if paramStarted { // Add last parameter if any
							params = append(params, currentParam)
						}
						break
					} else {
						// Invalid character in sequence, break parsing for this sequence
						log.Printf("WARN: Invalid char 0x%X in ANSI sequence at index %d", char, j)
						terminatorIndex = j - 1 // Treat previous char as end? Or handle differently?
						break
					}
				}

				if terminatorIndex != -1 {
					// Write the full sequence regardless of interpretation
					displayBuf.Write(content[start : terminatorIndex+1])
					consumed = (terminatorIndex + 1) - start

					// Helper to get parameter or default
					getParam := func(idx, def int) int {
						if idx < len(params) {
							// ANSI params default to 1 if omitted or 0
							if params[idx] == 0 {
								return def
							}
							return params[idx]
						}
						return def
					}

					// Interpret common sequences affecting coordinates
					switch terminator {
					case 'm': // SGR (Select Graphic Rendition) - Color/Style
						// Parse and update SGR state for cumulative color tracking
						if len(params) == 0 {
							params = []int{0} // ESC[m is equivalent to ESC[0m (reset)
						}
						for _, param := range params {
							switch param {
							case 0: // Reset all attributes
								sgrState.bold = false
								sgrState.dim = false
								sgrState.italic = false
								sgrState.underline = false
								sgrState.blink = false
								sgrState.reverse = false
								sgrState.hidden = false
								sgrState.foreground = 0
								sgrState.background = 0
							case 1: // Bold
								sgrState.bold = true
							case 2: // Dim
								sgrState.dim = true
							case 3: // Italic
								sgrState.italic = true
							case 4: // Underline
								sgrState.underline = true
							case 5: // Blink
								sgrState.blink = true
							case 7: // Reverse
								sgrState.reverse = true
							case 8: // Hidden
								sgrState.hidden = true
							case 22: // Normal intensity (not bold, not dim)
								sgrState.bold = false
								sgrState.dim = false
							case 23: // Not italic
								sgrState.italic = false
							case 24: // Not underlined
								sgrState.underline = false
							case 25: // Not blinking
								sgrState.blink = false
							case 27: // Not reversed
								sgrState.reverse = false
							case 28: // Not hidden
								sgrState.hidden = false
							case 30, 31, 32, 33, 34, 35, 36, 37: // Foreground colors
								sgrState.foreground = param
							case 39: // Default foreground color
								sgrState.foreground = 0
							case 40, 41, 42, 43, 44, 45, 46, 47: // Background colors
								sgrState.background = param
							case 49: // Default background color
								sgrState.background = 0
							case 90, 91, 92, 93, 94, 95, 96, 97: // Bright foreground colors
								sgrState.foreground = param
							case 100, 101, 102, 103, 104, 105, 106, 107: // Bright background colors
								sgrState.background = param
							}
						}
					case 'A': // Cursor Up
						currentY -= getParam(0, 1)
						if currentY < 1 {
							currentY = 1
						}
					case 'B': // Cursor Down
						currentY += getParam(0, 1)
					case 'C': // Cursor Forward
						currentX += getParam(0, 1)
					case 'D': // Cursor Back
						currentX -= getParam(0, 1)
						if currentX < 1 {
							currentX = 1
						}
					case 'H', 'f': // Cursor Position [y;xH] or [;H] or [H] or [y;xf]
						row := getParam(0, 1)
						col := getParam(1, 1)
						currentY = row
						currentX = col
						if currentY < 1 {
							currentY = 1
						}
						if currentX < 1 {
							currentX = 1
						}
					case 's': // Save cursor position (DEC)
						// We might need to store this if |P uses it, but handle simple cases first
						log.Printf("TRACE: Ignoring ANSI Save Cursor (ESC[s)")
					case 'u': // Restore cursor position (DEC)
						// We might need to store this if |PP uses it, but handle simple cases first
						log.Printf("TRACE: Ignoring ANSI Restore Cursor (ESC[u)")
						// Add other sequences like J (Erase), K (Erase Line) if needed,
						// but they usually don't affect cursor position themselves.
					}
					// Log the coordinate change for debugging
					// log.Printf("TRACE: ANSI Seq '%s', new coords (%d, %d)", string(content[start:terminatorIndex+1]), currentX, currentY)

				} else {
					log.Printf("WARN: Incomplete or invalid ANSI CSI sequence starting at index %d", start)
					displayBuf.WriteByte(b) // Write only ESC as fallback
				}
			} else {
				// Not a CSI sequence (e.g., ESC M, ESC(0 ), write ESC literally or handle specific non-CSI?
				// Let's pass through common non-CSI like ESC(0 and ESC(B for VT100 line drawing
				if i+1 < len(content) && (content[i+1] == '(' || content[i+1] == ')') && i+2 < len(content) {
					displayBuf.Write(content[i : i+3])
					consumed = 3
				} else {
					displayBuf.WriteByte(b)
				}
			}

		case 0x0d: // --- Carriage Return (CR) --- handle standalone \r
			currentX = 1
			displayBuf.WriteByte(b) // Write CR to output (terminal handles it)
			consumed = 1
		case 0x0a: // --- Line Feed (LF) --- // Already normalized, implies CR+LF
			currentY++
			currentX = 1
			displayBuf.WriteByte(b) // Write LF byte to output
			consumed = 1
		case '|': // --- ViSiON/2 Pipe Codes ---
			pipeCodeFound := false
			if i+1 < len(content) { // Need at least |X
				char1 := content[i+1]
				// Check for placeholder codes |XX or |X (These get consumed, not written)
				if char1 >= 'A' && char1 <= 'Z' {
					if i+2 < len(content) && content[i+2] >= 'A' && content[i+2] <= 'Z' {
						// Double letter |XX
						placeholderCode := string(content[i+1 : i+3])
						result.FieldCoords[placeholderCode] = struct{ X, Y int }{X: currentX, Y: currentY}
						result.FieldColors[placeholderCode] = buildColorSequence()
						log.Printf("DEBUG: ProcessAnsi recorded placeholder coord '%s' at (%d, %d) with color '%s'", placeholderCode, currentX, currentY, buildColorSequence())
						consumed = 3
						pipeCodeFound = true
					} else if i+1 < len(content) { // Check bounds for single letter
						// Single letter |X
						placeholderCode := string(content[i+1 : i+2])
						result.FieldCoords[placeholderCode] = struct{ X, Y int }{X: currentX, Y: currentY}
						result.FieldColors[placeholderCode] = buildColorSequence()
						log.Printf("DEBUG: ProcessAnsi recorded placeholder coord '%s' at (%d, %d) with color '%s'", placeholderCode, currentX, currentY, buildColorSequence())
						consumed = 2
						pipeCodeFound = true
					}
				} // End placeholder check

				// If not a placeholder, check for other |XX codes (colors, commands)
				// Only check if i+2 is valid
				if !pipeCodeFound && i+2 < len(content) {
					codeBytes := content[i : i+3] // Get the full |XX
					codeStr := string(codeBytes)
					if replacement, ok := pipeCodeReplacements[codeStr]; ok {
						displayBuf.WriteString(replacement)
						consumed = 3
						pipeCodeFound = true
						// Update SGR state so FieldColors captures correct colors after pipe codes.
						// Parse SGR params from the replacement ANSI string (e.g. "\x1b[0;34m" -> [0,34]).
						for ri := 0; ri < len(replacement); ri++ {
							if replacement[ri] == 0x1b && ri+1 < len(replacement) && replacement[ri+1] == '[' {
								ri += 2
								var sgrParams []int
								num := 0
								hasNum := false
								for ri < len(replacement) && replacement[ri] != 'm' {
									if replacement[ri] >= '0' && replacement[ri] <= '9' {
										num = num*10 + int(replacement[ri]-'0')
										hasNum = true
									} else if replacement[ri] == ';' {
										if hasNum {
											sgrParams = append(sgrParams, num)
										}
										num = 0
										hasNum = false
									}
									ri++
								}
								if hasNum {
									sgrParams = append(sgrParams, num)
								}
								if ri < len(replacement) && replacement[ri] == 'm' {
									if len(sgrParams) == 0 {
										sgrParams = []int{0}
									}
									for _, p := range sgrParams {
										switch {
										case p == 0:
											sgrState.bold = false
											sgrState.dim = false
											sgrState.italic = false
											sgrState.underline = false
											sgrState.blink = false
											sgrState.reverse = false
											sgrState.hidden = false
											sgrState.foreground = 0
											sgrState.background = 0
										case p == 1:
											sgrState.bold = true
										case p >= 30 && p <= 37:
											sgrState.foreground = p
										case p == 39:
											sgrState.foreground = 0
										case p >= 40 && p <= 47:
											sgrState.background = p
										case p == 49:
											sgrState.background = 0
										case p >= 90 && p <= 97:
											sgrState.foreground = p
										case p >= 100 && p <= 107:
											sgrState.background = p
										}
									}
								}
							}
						}
						if codeStr == "|CL" {
							currentX = 1
							currentY = 1
						}
					} // Add other non-placeholder pipe codes if needed
				}
			}

			if !pipeCodeFound {
				displayBuf.WriteByte(b) // Write '|' literally if not part of a valid code
				currentX++
			}

		case '$': // --- ViSiON/2 Dollar Color Codes ---
			if i+1 < len(content) {
				colorDigit := content[i+1]
				if colorDigit >= '0' && colorDigit <= '7' {
					ansiCode := 30 + int(colorDigit-'0')
					displayBuf.WriteString(fmt.Sprintf("\x1b[%dm", ansiCode))
					consumed = 2 // Consume $X
				} else {
					// Unhandled $ code, write literally
					displayBuf.WriteByte(b)
					currentX++
				}
			} else {
				// $ at end of input
				displayBuf.WriteByte(b)
				currentX++
			}

		case '^': // --- ViSiON/2 Control Codes (^G BEL) ---
			if i+1 < len(content) {
				controlChar := content[i+1]
				if controlChar == 'G' || controlChar == 'g' || controlChar == '7' { // Check for BEL (^G or ^7)
					displayBuf.WriteByte(0x07) // Write ASCII BEL
					consumed = 2
				} else {
					// Other ^ codes - write literally?
					displayBuf.WriteByte(b)           // Write '^'
					displayBuf.WriteByte(controlChar) // Write the char after ^
					currentX += 2                     // Assume they advance cursor
					consumed = 2
				}
			} else {
				// ^ at end of input
				displayBuf.WriteByte(b)
				currentX++
			}

		case '~': // --- ViSiON/2 Tilde Codes (Coordinates) ---
			if i+2 < len(content) && content[i+1] >= 'A' && content[i+1] <= 'Z' && content[i+2] >= 'A' && content[i+2] <= 'Z' {
				code := string(content[i+1 : i+3])
				result.FieldCoords[code] = struct{ X, Y int }{X: currentX, Y: currentY}
				result.FieldColors[code] = buildColorSequence()
				log.Printf("DEBUG: ProcessAnsi found coord code %s at (%d, %d) with color '%s'", code, currentX, currentY, buildColorSequence())
				consumed = 3 // Consume ~XX, write nothing
			} else {
				// Invalid ~ sequence, write literally
				displayBuf.WriteByte(b)
				currentX++
			}
		// Add cases for '$', '^', '@' if they need special handling beyond just display
		// For now, assume they don't affect coordinates and are handled by display logic if needed.

		default: // --- Normal Character Byte ---
			if outputMode == OutputModeUTF8 && b >= 128 {
				// UTF-8 Mode: Convert CP437 extended characters (> 127) to UTF-8
				r := Cp437ToUnicode[b] // Convert CP437 byte to Unicode rune
				if r == 0 && b != 0 {
					// Handle cases where the map might have a zero entry for a non-zero byte
					displayBuf.WriteByte('?') // Fallback character
				} else if r != 0 {
					// Encode the rune as UTF-8 and write to buffer
					displayBuf.WriteRune(r)
				}
				// If r is 0 and b is 0, it's a null byte, which is skipped
			} else {
				// CP437 Mode (or Auto detected as CP437) OR ASCII (< 128):
				// Write the original byte directly.
				displayBuf.WriteByte(b)
			}

			// Only increment X for printable characters.
			if b >= 0x20 && b != 0x7f { // Basic check for printable ASCII range (excluding DEL)
				currentX++
			} else if b >= 0x80 { // Assume CP437 chars > 127 are printable and advance cursor
				currentX++
			}
			// We don't increment X for other control chars (0x00-0x1f except \n, \r)
		}

		i += consumed
	}

	result.DisplayBytes = displayBuf.Bytes()
	return result, nil
}
