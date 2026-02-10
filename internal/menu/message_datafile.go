package menu

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
