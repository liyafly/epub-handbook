// urlpath.go keeps structure-normalize error adapters and Python repr formatting.
package structurenormalize

import (
	"fmt"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
)

func validateArchivePath(name, label string) (string, error) {
	value, err := pypath.ValidateArchivePath(name, label)
	if err != nil {
		return "", toolErrf("%v", err)
	}
	return value, nil
}

func resolveRelativePath(baseFile, uriPath string) (string, error) {
	value, err := pypath.ResolveRelativePath(baseFile, uriPath)
	if err != nil {
		return "", toolErrf("%v", err)
	}
	return value, nil
}

func resolveRootPath(uriPath string) (string, error) {
	value, err := pypath.ResolveRootPath(uriPath)
	if err != nil {
		return "", toolErrf("%v", err)
	}
	return value, nil
}

// pyRepr 近似 Python 的 str repr（错误消息里的 {uri!r}）。
func pyRepr(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('\'')
	return b.String()
}
