package report

import "strings"

// ShellQuote returns value as a single POSIX shell argument. ASCII alphanumerics
// and a small set of inert path/argument characters are left unquoted; every
// other value is single-quoted with embedded apostrophes escaped.
func ShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	for i := 0; i < len(value); i++ {
		b := value[i]
		if b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' {
			continue
		}
		switch b {
		case '_', '@', '%', '+', '=', ':', ',', '.', '/', '-':
			continue
		default:
			return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
		}
	}
	return value
}
