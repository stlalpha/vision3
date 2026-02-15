package menu

import (
	"bytes"
)

// processTemplate detects the placeholder format and routes to the appropriate processor.
// Supports two formats:
//   - New format: @CODE@, @CODE:20@, @CODE###@ (Retrograde-style)
//   - Legacy format: |X (Vision/2 Pascal-style)
//
// Format detection is based on presence of @-delimited codes (@T@, @F@, @S@).
func processTemplate(fileBytes []byte, substitutions map[byte]string) []byte {
	// Check for new @CODE@ format
	// Simple heuristic: Look for common placeholder patterns
	if bytes.Contains(fileBytes, []byte("@T@")) ||
		bytes.Contains(fileBytes, []byte("@F@")) ||
		bytes.Contains(fileBytes, []byte("@S@")) ||
		bytes.Contains(fileBytes, []byte("@T:")) ||
		bytes.Contains(fileBytes, []byte("@T#")) ||
		bytes.Contains(fileBytes, []byte("@F:")) ||
		bytes.Contains(fileBytes, []byte("@F#")) {
		return processPlaceholderTemplate(fileBytes, substitutions)
	}

	// Fall back to legacy |X format
	return processDataFile(fileBytes, substitutions)
}

// processDataFile performs Pascal-style DataFile substitution on raw file bytes.
// Substitution codes are |X where X is a single non-digit character (letter or symbol).
// These are distinct from pipe color codes (|XX where XX are two digits like |07, |15).
//
// Pascal substitution codes for message headers:
//
//	|B = Board/Area name    |T = Title/Subject    |F = From
//	|S = To (+ Read flag)   |U = Status           |L = Level
//	|R = Real name          |# = Current msg num  |N = Total msgs
//	|D = Date               |W = Time             |P = Reply number
//	|E = Replies count
//
// DEPRECATED: This function is maintained for backward compatibility with legacy |X templates.
// New templates should use @CODE@ format and will be processed via processPlaceholderTemplate().
func processDataFile(fileBytes []byte, substitutions map[byte]string) []byte {
	if len(fileBytes) == 0 {
		return fileBytes
	}

	result := make([]byte, 0, len(fileBytes)*2)
	i := 0

	for i < len(fileBytes) {
		if fileBytes[i] == '|' && i+1 < len(fileBytes) {
			next := fileBytes[i+1]

			// Check if next byte is a substitution key (non-digit)
			if val, ok := substitutions[next]; ok {
				// Found a matching substitution code - replace |X with value
				result = append(result, []byte(val)...)
				i += 2 // Skip past |X
				continue
			}

			// If next byte is a digit, it's a pipe color code (|07, |15, etc.)
			// Pass through unchanged - let ansi.ReplacePipeCodes handle it later
			// Also pass through any unrecognized |X patterns
			result = append(result, fileBytes[i])
			i++
			continue
		}

		// Regular byte - pass through
		result = append(result, fileBytes[i])
		i++
	}

	return result
}
