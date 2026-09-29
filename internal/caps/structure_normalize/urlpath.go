package structurenormalize

import (
	"fmt"
	"path"
	"strings"
)

// ---- URL / 路径工具（urllib.parse 与 posixpath 的语义） ----

type urlParts struct {
	scheme, netloc, path, query, fragment string
}

// pyURLSplit 复刻 urllib.parse.urlsplit 对本工具相关输入的行为：
// 先在首个 '#' 处切 fragment，再识别 scheme（':' 前缀全为合法 scheme
// 字符且首字符为 ASCII 字母），再识别 '//' netloc，再在首个 '?' 处切 query。
func pyURLSplit(raw string) urlParts {
	var p urlParts
	// Python 3 urlsplit 剥离首尾的 C0 控制符与空格，并移除内嵌的 \t\r\n
	// （WHATWG 规则）—— 书里实际存在带前导空格的引用，必须复刻。
	raw = strings.TrimFunc(raw, func(r rune) bool { return r <= ' ' })
	if strings.ContainsAny(raw, "\t\r\n") {
		raw = strings.Map(func(r rune) rune {
			if r == '\t' || r == '\r' || r == '\n' {
				return -1
			}
			return r
		}, raw)
	}
	rest := raw
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		p.fragment = rest[i+1:]
		rest = rest[:i]
	}
	if i := strings.IndexByte(rest, ':'); i > 0 && isASCIILetter(rest[0]) {
		valid := true
		for k := 0; k < i; k++ {
			c := rest[k]
			if !isASCIILetter(c) && !(c >= '0' && c <= '9') && c != '+' && c != '-' && c != '.' {
				valid = false
				break
			}
		}
		if valid {
			p.scheme = strings.ToLower(rest[:i])
			rest = rest[i+1:]
		}
	}
	if strings.HasPrefix(rest, "//") {
		j := 2
		for j < len(rest) {
			c := rest[j]
			if c == '/' || c == '?' || c == '#' {
				break
			}
			j++
		}
		p.netloc = rest[2:j]
		rest = rest[j:]
	}
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		p.query = rest[i+1:]
		rest = rest[:i]
	}
	p.path = rest
	return p
}

// pyURLUnsplitPath 复刻 urlunsplit(("", "", path, query, fragment))。
func pyURLUnsplitPath(pathPart, query, fragment string) string {
	out := pathPart
	if query != "" {
		out += "?" + query
	}
	if fragment != "" {
		out += "#" + fragment
	}
	return out
}

// pyIsExternalURI 复刻 is_external_uri：有 scheme 或以 / // 开头。
func pyIsExternalURI(uri string) bool {
	if p := pyURLSplit(uri); p.scheme != "" {
		return true
	}
	return strings.HasPrefix(uri, "/") || strings.HasPrefix(uri, "//")
}

// pyQuote 复刻 quote(value, safe="/:@-._~")：unreserved（字母数字与
// -._~）加上 safe 集合原样保留，其余按 UTF-8 字节转 %XX（大写）。
func pyQuote(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '-', c == '.', c == '_', c == '~', c == '/', c == ':', c == '@':
			b.WriteByte(c)
		default:
			const hex = "0123456789ABCDEF"
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0xF])
		}
	}
	return b.String()
}

// pyUnquote 复刻 unquote：字节级 %XX 解码后按 utf-8 / errors=replace 解码。
func pyUnquote(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	raw := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] == '%' && i+2 < len(s) {
			h1, ok1 := hexVal(s[i+1])
			h2, ok2 := hexVal(s[i+2])
			if ok1 && ok2 {
				raw = append(raw, h1<<4|h2)
				i += 3
				continue
			}
		}
		raw = append(raw, s[i])
		i++
	}
	// 与 CPython 的 utf-8 errors=replace 近似：每个非法子序列一个 U+FFFD。
	return strings.ToValidUTF8(string(raw), "\uFFFD")
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func pyDirname(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

func pyBasename(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// pyJoin 复刻 posixpath.join 对本工具输入的等价行为（空段跳过）。
func pyJoin(parts ...string) string {
	var out []string
	for _, part := range parts {
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, "/")
}

// pySplitExt 复刻 posixpath.splitext（含「basename 前导点不算扩展名」：
// splitext(".bashrc") == (".bashrc", "")）。
func pySplitExt(p string) (stem, ext string) {
	sep := strings.LastIndexByte(p, '/')
	dot := strings.LastIndexByte(p, '.')
	if dot > sep {
		for k := sep + 1; k < dot; k++ {
			if p[k] != '.' {
				return p[:dot], p[dot:]
			}
		}
	}
	return p, ""
}

// pathExt / pathStem 是 pySplitExt 的单值便捷形式。
func pathExt(p string) string {
	_, ext := pySplitExt(p)
	return ext
}

func pathStem(p string) string {
	stem, _ := pySplitExt(p)
	return stem
}

// pyRelPath 复刻 posixpath.relpath 对已归一相对路径的段级计算。
func pyRelPath(target, base string) string {
	startList := splitSegments(base)
	pathList := splitSegments(target)
	i := 0
	for i < len(startList) && i < len(pathList) && startList[i] == pathList[i] {
		i++
	}
	rel := make([]string, 0, len(startList)-i+len(pathList)-i)
	for k := 0; k < len(startList)-i; k++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pathList[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

func splitSegments(p string) []string {
	var out []string
	for _, seg := range strings.Split(p, "/") {
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

// validateArchivePath 复刻 epub_lib.validate_archive_path。
func validateArchivePath(name, label string) (string, error) {
	if name == "" || strings.HasPrefix(name, "/") {
		return "", toolErrf("%s: invalid absolute or empty ZIP path: %q", label, name)
	}
	normalized := path.Clean(name)
	if normalized == ".." || strings.HasPrefix(normalized, "../") {
		return "", toolErrf("%s: ZIP path escapes archive root: %q", label, name)
	}
	return normalized, nil
}

// resolveRelativePath 复刻 epub_lib.resolve_relative_path。
func resolveRelativePath(baseFile, uriPath string) (string, error) {
	decoded := pyUnquote(uriPath)
	return validateArchivePath(path.Join(pyDirname(baseFile), decoded), "resource href")
}

// resolveRootPath 复刻 resolve_root_path（encryption.xml 的 URI 目标）。
func resolveRootPath(uriPath string) (string, error) {
	return validateArchivePath(strings.TrimLeft(pyUnquote(uriPath), "/"), "encryption URI")
}

// relativeURI 复刻 relative_uri：relpath 后做 percent-quote。
func relativeURI(fromArchivePath, toArchivePath string) string {
	base := pyDirname(fromArchivePath)
	rel := toArchivePath
	if base != "" {
		rel = pyRelPath(toArchivePath, base)
	}
	return pyQuote(rel)
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
