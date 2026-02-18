package ansi

import (
	"regexp"
	"strconv"
)

// editorPlaceholderRegex matches @CODE@ placeholders with optional alignment modifiers.
// Formats:
//
//	@T@           — value as-is
//	@T:20@        — explicit width, left-aligned (default)
//	@T########@   — visual width, left-aligned
//	@T|R8@        — modifier R, width 8 (Synchronet-style)
//	@T|R:8@       — modifier R, explicit width 8
//	@T|R########@ — modifier R, visual width
//	@T|R@         — modifier R, no width (value as-is)
//
// Modifiers: L (left-justify), R (right-justify), C (center).
// Groups: 1=code, 2=modifier(opt), 3=width-after-modifier(opt), 4=:N width(opt), 5=###(opt)
var editorPlaceholderRegex = regexp.MustCompile(`@([A-Z#])(?:\|([LRC])(\d+)?)?(?::(\d+)|([#]+))?@`)

// ProcessEditorPlaceholders replaces @CODE@ placeholders in editor template files.
//
// Supported formats:
//
//	@S@          — insert value as-is (no width constraint)
//	@S:20@       — explicit width: truncate/pad to exactly 20 visible characters
//	@S########@  — visual width: total placeholder length (including delimiters) is the field width
//	@S|R8@       — right-justify in 8-char field (Synchronet-style width)
//	@S|R:20@     — right-justify in 20-char field (explicit colon width)
//	@S|R#######@ — right-justify in visual-width field
//	@S|C:20@     — center in 20-char field
//
// Unknown codes (not present in substitutions) are preserved unchanged.
// Color ANSI codes within values are preserved during truncation via ApplyWidthConstraintAligned.
func ProcessEditorPlaceholders(template []byte, substitutions map[byte]string) []byte {
	matches := editorPlaceholderRegex.FindAllSubmatchIndex(template, -1)
	if len(matches) == 0 {
		return template
	}

	result := make([]byte, 0, len(template))
	lastEnd := 0

	for _, match := range matches {
		// match[0:1]   = full match extent
		// match[2:3]   = code letter
		// match[4:5]   = modifier letter L/R/C (or -1 if absent)
		// match[6:7]   = digits after modifier (or -1 if absent)
		// match[8:9]   = :N digits (or -1 if absent)
		// match[10:11] = ### chars (or -1 if absent)

		result = append(result, template[lastEnd:match[0]]...)

		code := template[match[2]]
		value, ok := substitutions[code]
		if !ok {
			// Unknown code: preserve the original placeholder bytes unchanged.
			result = append(result, template[match[0]:match[1]]...)
			lastEnd = match[1]
			continue
		}

		// Parse alignment modifier
		align := AlignLeft
		if match[4] != -1 {
			align = ParseAlignment(string(template[match[4]:match[5]]))
		}

		// Parse width: digits-after-modifier > colon-width > visual-hash-width
		width := 0
		if match[6] != -1 { // digits immediately after modifier (e.g. @T|R8@)
			width, _ = strconv.Atoi(string(template[match[6]:match[7]]))
		} else if match[8] != -1 { // explicit :N width (e.g. @T:20@ or @T|R:20@)
			width, _ = strconv.Atoi(string(template[match[8]:match[9]]))
		} else if match[10] != -1 { // visual width = total placeholder byte length
			width = match[1] - match[0]
		}

		if width > 0 {
			value = ApplyWidthConstraintAligned(value, width, align)
		}

		result = append(result, []byte(value)...)
		lastEnd = match[1]
	}

	result = append(result, template[lastEnd:]...)
	return result
}

// FindEditorPlaceholderPos returns the terminal row and column (both 1-based) at which
// the first occurrence of a placeholder for the given code letter begins in template,
// along with the ANSI SGR escape sequence that restores the color/attribute state active
// at that position. This lets callers dynamically overwrite the placeholder later while
// preserving the original template colors.
//
// It works by scanning through the raw template bytes (before substitution) and tracking
// the terminal cursor position via ANSI CSI escape sequences. Because visual-width
// placeholders (@X####@) occupy the same byte count as their rendered field width, cursor
// tracking against the raw template gives the same result as tracking the rendered output.
//
// Returns (0, 0, "") if the placeholder is not found.
func FindEditorPlaceholderPos(template []byte, code byte) (row, col int, colorEsc string) {
	row, col = 1, 1
	i := 0
	n := len(template)

	// SGR attribute accumulator — tracks the "current color" state as we scan.
	var sgr sgrState

	for i < n {
		// Detect our placeholder: starts with @<code> followed by @, :, #, or |
		if i+2 <= n && template[i] == '@' && template[i+1] == code {
			if i+2 < n {
				next := template[i+2]
				if next == '@' || next == ':' || next == '#' || next == '|' {
					return row, col, sgr.escape()
				}
			}
		}

		// ANSI/VT escape sequence: ESC [
		if template[i] == 0x1b && i+1 < n && template[i+1] == '[' {
			seqStart := i
			i += 2
			// Collect parameter bytes (digits, semicolons, spaces)
			paramStart := i
			for i < n && (template[i] >= '0' && template[i] <= '9' || template[i] == ';' || template[i] == ' ') {
				i++
			}
			if i >= n {
				break
			}
			cmd := template[i]
			i++
			_ = seqStart

			switch cmd {
			case 'H', 'f': // cursor absolute position ESC[row;colH
				paramBytes := template[paramStart : i-1]
				r, c := 1, 1
				semi := -1
				for j, b := range paramBytes {
					if b == ';' {
						semi = j
						break
					}
				}
				if semi == -1 {
					// Just a row
					if v, err := strconv.Atoi(string(paramBytes)); err == nil && v > 0 {
						r = v
					}
				} else {
					if v, err := strconv.Atoi(string(paramBytes[:semi])); err == nil && v > 0 {
						r = v
					}
					if v, err := strconv.Atoi(string(paramBytes[semi+1:])); err == nil && v > 0 {
						c = v
					}
				}
				row, col = r, c
			case 'A': // cursor up
				n2 := parseSingleParam(template[paramStart:i-1], 1)
				row -= n2
			case 'B': // cursor down
				n2 := parseSingleParam(template[paramStart:i-1], 1)
				row += n2
			case 'C': // cursor forward (right)
				n2 := parseSingleParam(template[paramStart:i-1], 1)
				col += n2
			case 'D': // cursor back (left)
				n2 := parseSingleParam(template[paramStart:i-1], 1)
				col -= n2
			case 'm': // SGR — color/attribute change
				sgr.applyParams(string(template[paramStart : i-1]))
				// All other sequences (J, K, s, u, etc.) don't move the cursor
			}
			continue
		}

		// ESC without [ — single-char escape, consume and skip
		if template[i] == 0x1b {
			i += 2
			continue
		}

		// Normal characters
		switch template[i] {
		case '\r':
			col = 1
		case '\n':
			row++
			col = 1
		case '\t':
			col = ((col-1)/8+1)*8 + 1 // advance to next tab stop
		default:
			if template[i] >= 0x20 { // printable (ASCII + high-byte CP437)
				col++
			}
		}
		i++
	}

	return 0, 0, "" // not found
}

// sgrState tracks the accumulated SGR (Select Graphic Rendition) attributes
// as we scan through ANSI content. This lets us reconstruct the active
// foreground, background, bold, blink state at any point.
type sgrState struct {
	bold  bool
	faint bool
	blink bool
	fg    int // -1 = default, 30-37 = normal, 90-97 = bright
	bg    int // -1 = default, 40-47 = normal, 100-107 = bright
}

// applyParams processes a semicolon-separated SGR parameter string (e.g. "1;36").
func (s *sgrState) applyParams(paramStr string) {
	if paramStr == "" {
		// ESC[m with no params is equivalent to ESC[0m (reset)
		*s = sgrState{fg: -1, bg: -1}
		return
	}
	parts := splitSGR(paramStr)
	for _, p := range parts {
		switch {
		case p == 0: // reset
			*s = sgrState{fg: -1, bg: -1}
		case p == 1:
			s.bold = true
		case p == 2:
			s.faint = true
		case p == 5:
			s.blink = true
		case p == 22: // normal intensity
			s.bold = false
			s.faint = false
		case p == 25: // blink off
			s.blink = false
		case p >= 30 && p <= 37:
			s.fg = p
		case p == 39: // default fg
			s.fg = -1
		case p >= 40 && p <= 47:
			s.bg = p
		case p == 49: // default bg
			s.bg = -1
		case p >= 90 && p <= 97: // bright fg
			s.fg = p
		case p >= 100 && p <= 107: // bright bg
			s.bg = p
		}
	}
}

// escape returns the ANSI escape sequence that restores this SGR state.
// Returns "" if the state is completely default.
func (s *sgrState) escape() string {
	var parts []byte
	parts = append(parts, '0') // always start with reset for a clean slate
	if s.bold {
		parts = append(parts, ';', '1')
	}
	if s.faint {
		parts = append(parts, ';', '2')
	}
	if s.blink {
		parts = append(parts, ';', '5')
	}
	if s.fg >= 0 {
		parts = append(parts, ';')
		parts = strconv.AppendInt(parts, int64(s.fg), 10)
	}
	if s.bg >= 0 {
		parts = append(parts, ';')
		parts = strconv.AppendInt(parts, int64(s.bg), 10)
	}
	// Only "0" with no other attributes → fully default, still emit for safety.
	return "\x1b[" + string(parts) + "m"
}

// splitSGR splits a semicolon-separated parameter string into ints.
func splitSGR(s string) []int {
	var result []int
	val := 0
	hasDigit := false
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			val = val*10 + int(s[i]-'0')
			hasDigit = true
		} else if s[i] == ';' {
			if hasDigit {
				result = append(result, val)
			} else {
				result = append(result, 0) // missing param = 0
			}
			val = 0
			hasDigit = false
		}
	}
	if hasDigit {
		result = append(result, val)
	} else {
		result = append(result, 0)
	}
	return result
}

// FindEditorColorAtPos returns the ANSI SGR escape sequence active at the specified
// terminal row and column (both 1-based) in the raw template bytes. It processes ANSI
// escape sequences (which update SGR state without advancing the column counter) before
// checking the column position, so the returned state accurately reflects the color that
// would apply when drawing the character at that column.
//
// Use this to capture the color context of a specific template cell for cursor-overlay
// rendering — for example, to get the dark shade color at the ░ padding position
// adjacent to a dynamic number field.
//
// Returns "" if the target position is not reached in the template.
func FindEditorColorAtPos(template []byte, targetRow, targetCol int) string {
	row, col := 1, 1
	i := 0
	n := len(template)

	var sgr sgrState

	for i < n {
		// Process ANSI/VT escape sequences first — they update SGR/cursor without
		// advancing the visible column counter.
		if template[i] == 0x1b && i+1 < n && template[i+1] == '[' {
			i += 2
			paramStart := i
			for i < n && (template[i] >= '0' && template[i] <= '9' || template[i] == ';' || template[i] == ' ') {
				i++
			}
			if i >= n {
				break
			}
			cmd := template[i]
			i++

			switch cmd {
			case 'H', 'f': // cursor absolute ESC[row;colH
				paramBytes := template[paramStart : i-1]
				r, c := 1, 1
				semi := -1
				for j, b := range paramBytes {
					if b == ';' {
						semi = j
						break
					}
				}
				if semi == -1 {
					if v, err := strconv.Atoi(string(paramBytes)); err == nil && v > 0 {
						r = v
					}
				} else {
					if v, err := strconv.Atoi(string(paramBytes[:semi])); err == nil && v > 0 {
						r = v
					}
					if v, err := strconv.Atoi(string(paramBytes[semi+1:])); err == nil && v > 0 {
						c = v
					}
				}
				row, col = r, c
			case 'A':
				row -= parseSingleParam(template[paramStart:i-1], 1)
			case 'B':
				row += parseSingleParam(template[paramStart:i-1], 1)
			case 'C':
				col += parseSingleParam(template[paramStart:i-1], 1)
			case 'D':
				col -= parseSingleParam(template[paramStart:i-1], 1)
			case 'm':
				sgr.applyParams(string(template[paramStart : i-1]))
			}
			continue
		}

		// ESC without [ — single-char escape, skip
		if template[i] == 0x1b {
			i += 2
			continue
		}

		// Check target AFTER processing any ANSI sequences at this position,
		// BEFORE advancing the column for the visible character.
		if row == targetRow && col == targetCol {
			return sgr.escape()
		}
		if row > targetRow {
			return "" // passed the target row
		}

		// Advance cursor position for visible characters
		switch template[i] {
		case '\r':
			col = 1
		case '\n':
			row++
			col = 1
		case '\t':
			col = ((col-1)/8+1)*8 + 1
		default:
			if template[i] >= 0x20 {
				col++
			}
		}
		i++
	}

	return "" // target position not reached
}

// parseSingleParam parses an optional integer from CSI parameter bytes, returning def if absent.
func parseSingleParam(b []byte, def int) int {
	if len(b) == 0 {
		return def
	}
	if v, err := strconv.Atoi(string(b)); err == nil && v > 0 {
		return v
	}
	return def
}
