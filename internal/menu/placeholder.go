package menu

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	"github.com/stlalpha/vision3/internal/ansi"
)

// PlaceholderMatch represents a parsed placeholder from a template.
type PlaceholderMatch struct {
	Code      string // Single letter (T, F, S, etc.)
	Width     int    // 0 = no width constraint
	AutoWidth bool   // true = use auto-calculated width from context
	FullMatch string // Complete matched text "@T###@"
	StartPos  int    // Byte offset in template
	EndPos    int    // End byte offset
}

// Regex compiled once for performance.
// Matches: @CODE@, @CODE:20@, @CODE###@, or @CODE*@
// Groups: 1=code letter, 2=:WIDTH (optional), 3=### (optional), 4=* (optional)
// G = gap fill: fills remaining line width with ─ (CP437 0xC4) characters.
var placeholderRegex = regexp.MustCompile(`@([BTFSUL#NDWPEOMAZCXGV])(?::(\d+)|([#]+)|(\*))?@`)

// parsePlaceholders extracts all @CODE@ patterns from template bytes.
func parsePlaceholders(template []byte) []PlaceholderMatch {
	matches := placeholderRegex.FindAllSubmatchIndex(template, -1)
	result := make([]PlaceholderMatch, 0, len(matches))

	for _, match := range matches {
		// match[0], match[1] = full match start/end
		// match[2], match[3] = code letter start/end
		// match[4], match[5] = :WIDTH start/end (or -1 if not present)
		// match[6], match[7] = ### start/end (or -1 if not present)
		// match[8], match[9] = * start/end (or -1 if not present)

		code := string(template[match[2]:match[3]])
		fullMatch := string(template[match[0]:match[1]])

		// Calculate width
		width := 0
		autoWidth := false
		if match[4] != -1 && match[4] < match[5] {
			// Parameter width :20 (regex captures digits only, not colon)
			widthStr := string(template[match[4]:match[5]])
			width, _ = strconv.Atoi(widthStr)
		} else if match[6] != -1 && match[6] < match[7] {
			// Visual width - use total placeholder length including @, code, #'s, and @
			width = match[1] - match[0]
		} else if match[8] != -1 {
			// Auto-width: width determined at render time from context
			autoWidth = true
		}

		result = append(result, PlaceholderMatch{
			Code:      code,
			Width:     width,
			AutoWidth: autoWidth,
			FullMatch: fullMatch,
			StartPos:  match[0],
			EndPos:    match[1],
		})
	}

	return result
}

// gapFillMarker is an internal marker that replaces @G@ during the first pass.
// Chosen to be unlikely to appear in real template content.
const gapFillMarker = "\x00GAP_FILL\x00"

// processPlaceholderTemplate replaces @CODE@ placeholders with values from substitutions map.
// Supports four formats:
//   - @T@ - Insert value as-is
//   - @T:20@ - Explicit width (parameter-based)
//   - @T###########@ - Visual width (width = count of # characters)
//   - @T*@ - Auto-width (width from autoWidths map, calculated from context)
//
// Special code @G@ (gap fill): fills remaining line width with ─ (CP437 0xC4).
// Width is determined by: @G:80@ (explicit target), @G*@ (auto-width from map),
// or @G@ (default 80). The fill count = target_width - visible_chars_on_line.
//
// autoWidths is optional (nil = no auto-width support). When provided, @CODE*@ placeholders
// look up their width from this map.
func processPlaceholderTemplate(template []byte, substitutions map[byte]string, autoWidths map[byte]int) []byte {
	matches := parsePlaceholders(template)
	if len(matches) == 0 {
		return template // No placeholders
	}

	// Track whether we have any gap fill placeholders
	hasGapFill := false

	// Build result by copying template and replacing placeholders
	result := make([]byte, 0, len(template)*2)
	lastEnd := 0

	for _, match := range matches {
		// Copy template bytes before this placeholder
		result = append(result, template[lastEnd:match.StartPos]...)

		if match.Code == "G" {
			// Gap fill: determine target width and insert marker for second pass
			hasGapFill = true
			targetWidth := 79 // default (79 avoids auto-wrap at column 80)
			if match.AutoWidth && autoWidths != nil {
				if w, ok := autoWidths['G']; ok && w > 0 {
					targetWidth = w
				}
			} else if match.Width > 0 {
				targetWidth = match.Width
			}
			// Encode target width into marker
			marker := gapFillMarker + strconv.Itoa(targetWidth) + gapFillMarker
			result = append(result, []byte(marker)...)
		} else {
			// Get substitution value (map key is byte)
			value := ""
			if len(match.Code) > 0 {
				if val, ok := substitutions[match.Code[0]]; ok {
					value = val
				}
			}

			// Apply width constraint if specified
			if match.AutoWidth && autoWidths != nil {
				if w, ok := autoWidths[match.Code[0]]; ok && w > 0 {
					value = ansi.ApplyWidthConstraint(value, w)
				}
			} else if match.Width > 0 {
				value = ansi.ApplyWidthConstraint(value, match.Width)
			}

			// Append processed value
			result = append(result, []byte(value)...)
		}

		lastEnd = match.EndPos
	}

	// Append remaining template after last placeholder
	result = append(result, template[lastEnd:]...)

	// Second pass: resolve gap fill markers
	if hasGapFill {
		result = resolveGapFills(result)
	}

	return result
}

// resolveGapFills replaces gap fill markers with ─ characters to fill lines to target width.
// Each marker encodes its target width. The fill count is calculated per-line:
// fill = targetWidth - visibleCharsOnLine (excluding the marker itself).
func resolveGapFills(data []byte) []byte {
	markerBytes := []byte(gapFillMarker)

	// Process line by line to calculate per-line visible widths
	lines := bytes.Split(data, []byte("\n"))
	for i, line := range lines {
		// Check if line contains a gap fill marker
		startIdx := bytes.Index(line, markerBytes)
		if startIdx == -1 {
			continue
		}

		// Find the full marker: \x00GAP_FILL\x00<width>\x00GAP_FILL\x00
		afterFirst := startIdx + len(markerBytes)
		endIdx := bytes.Index(line[afterFirst:], markerBytes)
		if endIdx == -1 {
			continue
		}
		endIdx += afterFirst + len(markerBytes)

		// Extract target width
		widthStr := string(line[afterFirst : afterFirst+endIdx-afterFirst-len(markerBytes)])
		targetWidth, err := strconv.Atoi(widthStr)
		if err != nil || targetWidth <= 0 {
			targetWidth = 80
		}

		// Calculate visible width of line WITHOUT the marker
		lineWithout := make([]byte, 0, len(line))
		lineWithout = append(lineWithout, line[:startIdx]...)
		lineWithout = append(lineWithout, line[endIdx:]...)
		// Strip \r for width calculation
		visibleWidth := ansi.VisibleLength(strings.TrimRight(string(lineWithout), "\r"))

		// Calculate fill count
		fillCount := targetWidth - visibleWidth
		if fillCount < 0 {
			fillCount = 0
		}

		// Build fill string: CP437 horizontal line character (0xC4)
		fill := bytes.Repeat([]byte{0xC4}, fillCount)

		// Replace marker with fill
		newLine := make([]byte, 0, len(line))
		newLine = append(newLine, line[:startIdx]...)
		newLine = append(newLine, fill...)
		newLine = append(newLine, line[endIdx:]...)
		lines[i] = newLine
	}

	return bytes.Join(lines, []byte("\n"))
}
