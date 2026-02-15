package menu

import (
	"regexp"
	"strconv"

	"github.com/stlalpha/vision3/internal/ansi"
)

// PlaceholderMatch represents a parsed placeholder from a template.
type PlaceholderMatch struct {
	Code      string // Single letter (T, F, S, etc.)
	Width     int    // 0 = no width constraint
	FullMatch string // Complete matched text "@T###@"
	StartPos  int    // Byte offset in template
	EndPos    int    // End byte offset
}

// Regex compiled once for performance.
// Matches: @CODE@, @CODE:20@, or @CODE###@
// Groups: 1=code letter, 2=:WIDTH (optional), 3=### (optional)
var placeholderRegex = regexp.MustCompile(`@([BTFSULR#NDWPEOMA])(?::(\d+)|([#]+))?@`)

// parsePlaceholders extracts all @CODE@ patterns from template bytes.
func parsePlaceholders(template []byte) []PlaceholderMatch {
	matches := placeholderRegex.FindAllSubmatchIndex(template, -1)
	result := make([]PlaceholderMatch, 0, len(matches))

	for _, match := range matches {
		// match[0], match[1] = full match start/end
		// match[2], match[3] = code letter start/end
		// match[4], match[5] = :WIDTH start/end (or -1 if not present)
		// match[6], match[7] = ### start/end (or -1 if not present)

		code := string(template[match[2]:match[3]])
		fullMatch := string(template[match[0]:match[1]])

		// Calculate width
		width := 0
		if match[4] != -1 && match[4] < match[5] {
			// Parameter width :20 (regex captures digits only, not colon)
			widthStr := string(template[match[4]:match[5]])
			width, _ = strconv.Atoi(widthStr)
		} else if match[6] != -1 && match[6] < match[7] {
			// Visual width ### (count # chars)
			width = match[7] - match[6]
		}

		result = append(result, PlaceholderMatch{
			Code:      code,
			Width:     width,
			FullMatch: fullMatch,
			StartPos:  match[0],
			EndPos:    match[1],
		})
	}

	return result
}

// processPlaceholderTemplate replaces @CODE@ placeholders with values from substitutions map.
// Supports three formats:
//   - @T@ - Insert value as-is
//   - @T:20@ - Explicit width (parameter-based)
//   - @T###########@ - Visual width (width = count of # characters)
func processPlaceholderTemplate(template []byte, substitutions map[byte]string) []byte {
	matches := parsePlaceholders(template)
	if len(matches) == 0 {
		return template // No placeholders
	}

	// Build result by copying template and replacing placeholders
	result := make([]byte, 0, len(template)*2)
	lastEnd := 0

	for _, match := range matches {
		// Copy template bytes before this placeholder
		result = append(result, template[lastEnd:match.StartPos]...)

		// Get substitution value (map key is byte)
		value := ""
		if len(match.Code) > 0 {
			if val, ok := substitutions[match.Code[0]]; ok {
				value = val
			}
		}

		// Apply width constraint if specified
		if match.Width > 0 {
			value = ansi.ApplyWidthConstraint(value, match.Width)
		}

		// Append processed value
		result = append(result, []byte(value)...)

		lastEnd = match.EndPos
	}

	// Append remaining template after last placeholder
	result = append(result, template[lastEnd:]...)

	return result
}
