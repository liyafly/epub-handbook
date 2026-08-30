// pytool.go 复刻 scripts/epub_lib.py 与 core.py 里的 URL / 路径 / 属性
// 工具函数（urllib.parse、posixpath、saxutils 的精确语义）。
package split

import (
	"fmt"
	"path"
	"strings"
)

// whatwgC0ControlOrSpace 对齐 CPython _WHATWG_C0_CONTROL_OR_SPACE。
const whatwgC0ControlOrSpace = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
	"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f "

type urlParts struct {
	scheme, netloc, path, query, fragment string
}

// pyURLSplit 复刻 urllib.parse.urlsplit 的相关行为（含 Python 3.10+ 的
// WHATWG 清洗：先去掉首尾 C0 控制符与空格，再全局删除 \t \r \n）。
func pyURLSplit(raw string) urlParts {
	var p urlParts
	raw = strings.Trim(raw, whatwgC0ControlOrSpace)
	for _, b := range []string{"\t", "\r", "\n"} {
		raw = strings.ReplaceAll(raw, b, "")
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

// pyIsExternalURI 复刻 is_external_uri。
func pyIsExternalURI(uri string) bool {
	if p := pyURLSplit(uri); p.scheme != "" {
		return true
	}
	return strings.HasPrefix(uri, "/") || strings.HasPrefix(uri, "//")
}

// pyQuote 复刻 quote(value, safe="/:@-._~")。
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

// pyUnquote 复刻 unquote（非法 % 序列原样保留）。
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
	return string(raw)
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

// pySplitExt 复刻 posixpath.splitext。
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

// pathExt 返回 pySplitExt 的扩展名部分。
func pathExt(p string) string {
	_, ext := pySplitExt(p)
	return ext
}

// pyRelPath 复刻 posixpath.relpath 的段级计算。
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

// relativeURI 复刻 core.relative_uri。
func relativeURI(fromArchivePath, toArchivePath string) string {
	base := pyDirname(fromArchivePath)
	rel := toArchivePath
	if base != "" {
		rel = pyRelPath(toArchivePath, base)
	}
	return pyQuote(rel)
}

// splitProps 复刻 epub_lib.split_props。
func splitProps(value string) []string {
	if value == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Fields(value) {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// hasNavProp 对齐 "nav" in split_props(value)。
func hasNavProp(props string) bool {
	for _, p := range splitProps(props) {
		if p == "nav" {
			return true
		}
	}
	return false
}

// propText 复刻 core.prop_text（保序去重）。
func propText(values []string) string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return strings.Join(out, " ")
}

// addProp / removeProp 复刻 core.add_prop / core.remove_prop。
func addProp(value, prop string) string {
	props := splitProps(value)
	if !containsStr(props, prop) {
		props = append(props, prop)
	}
	return propText(props)
}

func removeProp(value, prop string) string {
	var out []string
	for _, item := range splitProps(value) {
		if item != prop {
			out = append(out, item)
		}
	}
	return propText(out)
}

func containsStr(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// uniqueID 复刻 core.unique_id（used 集合由调用方持有并被修改）。
// 注意 re.sub(r"[^A-Za-z0-9_.-]+", "-", base) 把连续非法字符压成一个 "-"。
func uniqueID(base string, used map[string]bool) string {
	var b strings.Builder
	prevInvalid := false
	for _, r := range base {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '_' || r == '.' || r == '-' {
			b.WriteRune(r)
			prevInvalid = false
			continue
		}
		if !prevInvalid {
			b.WriteByte('-')
			prevInvalid = true
		}
	}
	candidate := strings.Trim(b.String(), "-")
	if candidate == "" {
		candidate = "item"
	}
	if candidate[0] >= '0' && candidate[0] <= '9' {
		candidate = "x-" + candidate
	}
	result := candidate
	index := 2
	for used[result] {
		result = fmt.Sprintf("%s-%d", candidate, index)
		index++
	}
	used[result] = true
	return result
}

// prefixedArchivePath 复刻 core.prefixed_archive_path。
func prefixedArchivePath(p, prefix string) string {
	folder := pyDirname(p)
	base := pyBasename(p)
	if folder != "" {
		return folder + "/" + prefix + base
	}
	return prefix + base
}

// allocateArchivePath 复刻 core.allocate_archive_path。
func allocateArchivePath(preferred string, used map[string]bool, prefix string) (string, bool) {
	candidate := preferred
	renamed := false
	if used[candidate] {
		candidate = prefixedArchivePath(preferred, prefix)
		renamed = true
	}
	stem, ext := pySplitExt(candidate)
	index := 2
	for used[candidate] {
		candidate = fmt.Sprintf("%s-%d%s", stem, index, ext)
		renamed = true
		index++
	}
	used[candidate] = true
	return candidate, renamed
}

// pyEscapeText 复刻 xml.sax.saxutils.escape（& 优先，再 > 与 <）。
func pyEscapeText(data string) string {
	data = strings.ReplaceAll(data, "&", "&amp;")
	data = strings.ReplaceAll(data, ">", "&gt;")
	data = strings.ReplaceAll(data, "<", "&lt;")
	return data
}

// pyQuoteAttr 复刻 xml.sax.saxutils.quoteattr：值含 `"` 时改用单引号包裹，
// 两种引号并存时把 `"` 转义后仍用双引号包裹。
func pyQuoteAttr(data string) string {
	data = pyEscapeText(data)
	switch {
	case strings.Contains(data, `"`):
		if strings.Contains(data, "'") {
			return `"` + strings.ReplaceAll(data, `"`, "&quot;") + `"`
		}
		return "'" + data + "'"
	default:
		return `"` + data + `"`
	}
}

// collapseSpace 复刻 " ".join(text.split())。
func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
