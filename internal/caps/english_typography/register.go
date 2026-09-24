package englishtypography

import "regexp"

var languageTagPattern = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{1,8})*$`)

// ValidLang reports whether value matches the conservative BCP 47 form accepted
// by the public english typography capability.
func ValidLang(value string) bool {
	return languageTagPattern.MatchString(value)
}
