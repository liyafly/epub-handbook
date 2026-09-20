// URL/path behavior is shared with merge/split through book/pypath.
// Keep narrow adapters for this capability's sentinel error and projection.
package cover

import (
	"strings"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
)

type urlParts struct{ scheme, netloc, path, query, fragment string }

func pyURLSplit(raw string) urlParts {
	p := pypath.URLSplit(raw)
	return urlParts{p.Scheme, p.Netloc, p.Path, p.Query, p.Fragment}
}

func pyIsExternalURI(uri string) bool { return pypath.IsExternalURI(uri) }

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

func pyDirname(p string) string { return pypath.Dirname(p) }

func pySplitExt(p string) (string, string) { return pypath.SplitExt(p) }
func pathExt(p string) string              { return pypath.PathExt(p) }

// pathJoin 复刻 posixpath.join(dir, rel)。
func pathJoin(a, b string) string {
	if a == "" {
		return b
	}
	return a + "/" + b
}

func splitProps(value string) []string { return pypath.SplitProps(value) }

// addProp 复刻 core.add_prop（保序去重）。
func addProp(value, prop string) string {
	props := splitProps(value)
	if !containsStr(props, prop) {
		props = append(props, prop)
	}
	seen := map[string]bool{}
	var out []string
	for _, v := range props {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return strings.Join(out, " ")
}

// itemAttr 取无命名空间属性值（缺省 ""，对齐 attrib.get）。
func itemAttr(n interface {
	AttrByLocal(string, string) (string, bool)
}, name string) string {
	v, _ := n.AttrByLocal("", name)
	return v
}

// attrEscapeFor 按原引号字符转义属性值。
func attrEscapeFor(quote byte, value string) string {
	if quote == '\'' {
		return singleQuoteEscaper.Replace(value)
	}
	return attribEscape(value)
}
