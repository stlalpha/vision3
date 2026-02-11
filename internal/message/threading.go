package message

import (
	"regexp"
	"strings"
)

var reReplySuffix = regexp.MustCompile(` -Re: #\d+-$`)

// NormalizeThreadSubject strips Pascal-style reply markers for thread matching.
func NormalizeThreadSubject(subject string) string {
	s := strings.TrimSpace(subject)
	s = reReplySuffix.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	for strings.HasPrefix(strings.ToUpper(s), "RE: ") {
		s = strings.TrimSpace(s[4:])
	}
	return s
}

// ThreadKey returns a normalized, case-insensitive key for thread grouping.
func ThreadKey(subject string) string {
	return strings.ToLower(NormalizeThreadSubject(subject))
}

// SubjectsMatchThread checks if two subjects belong to the same thread.
func SubjectsMatchThread(msgSubject, searchSubject string) bool {
	return strings.EqualFold(NormalizeThreadSubject(msgSubject), NormalizeThreadSubject(searchSubject))
}
