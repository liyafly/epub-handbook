package pypath

import "strings"

// EscapeAttribute escapes the contents of a fixed-double-quoted XML attribute,
// including XML-normalized whitespace. Unlike QuoteAttr it never picks quotes.
func EscapeAttribute(value string) string {
	return strings.NewReplacer(
		"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;",
		"\r", "&#13;", "\n", "&#10;", "\t", "&#09;",
	).Replace(value)
}
