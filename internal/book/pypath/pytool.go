// Package pypath 复刻 Python 侧 urllib.parse / posixpath / xml.sax.saxutils
// 的路径、URL 与转义语义（源出 scripts/epub_lib.py 与 core.py）。
//
// 为什么放在 book 这一层（SPEC §1 第 5 层）：本包只依赖标准库，任何层都
// 能合法 import 它。之所以选层 5 而不是更靠上的 caps（层 2）或 scan（层
// 4），是因为 internal/scan/opf 的 nav/NCX 生成器（同为层 4）也要复用这
// 里的 URL / 路径 / 转义工具——同层禁止互相 import，caps 之间也禁止互相
// import，能被 caps 与 scan/opf 同时合法 import 的最近下层只剩 book（层
// 5）：caps（2）→ book/pypath（5）合法，scan/opf（4）→ book/pypath（5）
// 也合法。book 本身（层 5）不需要、也不会反过来 import 本包（同层禁止）。
package pypath

import (
	"fmt"
	"path"
	"strings"
)

// whatwgC0ControlOrSpace 对齐 CPython _WHATWG_C0_CONTROL_OR_SPACE。
const whatwgC0ControlOrSpace = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
	"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f "

// URLParts 是 urllib.parse.urlsplit 的分解结果。
type URLParts struct {
	Scheme, Netloc, Path, Query, Fragment string
}

// URLSplit 复刻 urllib.parse.urlsplit 的相关行为（含 Python 3.10+ 的
// WHATWG 清洗：先去掉首尾 C0 控制符与空格，再全局删除 \t \r \n）。
func URLSplit(raw string) URLParts {
	var p URLParts
	raw = strings.Trim(raw, whatwgC0ControlOrSpace)
	for _, b := range []string{"\t", "\r", "\n"} {
		raw = strings.ReplaceAll(raw, b, "")
	}
	rest := raw
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		p.Fragment = rest[i+1:]
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
			p.Scheme = strings.ToLower(rest[:i])
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
		p.Netloc = rest[2:j]
		rest = rest[j:]
	}
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		p.Query = rest[i+1:]
		rest = rest[:i]
	}
	p.Path = rest
	return p
}

// URLUnsplitPath 复刻 urlunsplit(("", "", path, query, fragment))。
func URLUnsplitPath(pathPart, query, fragment string) string {
	out := pathPart
	if query != "" {
		out += "?" + query
	}
	if fragment != "" {
		out += "#" + fragment
	}
	return out
}

// IsExternalURI 复刻 is_external_uri。
func IsExternalURI(uri string) bool {
	if p := URLSplit(uri); p.Scheme != "" {
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

// Dirname 复刻 posixpath.dirname。
func Dirname(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

// Basename 复刻 posixpath.basename。
func Basename(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// SplitExt 复刻 posixpath.splitext。
func SplitExt(p string) (stem, ext string) {
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

// PathExt 返回 SplitExt 的扩展名部分。
func PathExt(p string) string {
	_, ext := SplitExt(p)
	return ext
}

// RelPath preserves the segment-level relative calculation of normalized paths.
// It does not URI-decode or quote the result.
func RelPath(target, base string) string {
	startList := splitSegments(base)
	pathList := splitSegments(target)
	i := 0
	for i < len(startList) && i < len(pathList) && startList[i] == pathList[i] {
		i++
	}
	rel := make([]string, 0, len(startList)-i+len(pathList)-i)
	for range len(startList) - i {
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

// ValidateArchivePath 复刻 epub_lib.validate_archive_path。
func ValidateArchivePath(name, label string) (string, error) {
	if name == "" || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("%s: invalid absolute or empty ZIP path: %q", label, name)
	}
	normalized := path.Clean(name)
	if normalized == ".." || strings.HasPrefix(normalized, "../") {
		return "", fmt.Errorf("%s: ZIP path escapes archive root: %q", label, name)
	}
	return normalized, nil
}

// ResolveRelativePath 复刻 epub_lib.resolve_relative_path。
func ResolveRelativePath(baseFile, uriPath string) (string, error) {
	decoded := pyUnquote(uriPath)
	return ValidateArchivePath(path.Join(Dirname(baseFile), decoded), "resource href")
}

// RelativeURI 复刻 core.relative_uri。
func RelativeURI(fromArchivePath, toArchivePath string) string {
	return pyQuote(RelativePath(fromArchivePath, toArchivePath))
}

// SplitProps 复刻 epub_lib.split_props。
func SplitProps(value string) []string {
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

// HasNavProp 对齐 "nav" in split_props(value)。
func HasNavProp(props string) bool {
	for _, p := range SplitProps(props) {
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

// addProp 复刻 core.add_prop（当前调用方均未使用，随 pytool.go 整体迁移
// 保留：与 removeProp 同源，删掉会让 add/remove 这对语义失去对称性）。
func addProp(value, prop string) string {
	props := SplitProps(value)
	if !containsStr(props, prop) {
		props = append(props, prop)
	}
	return propText(props)
}

// RemoveProp 复刻 core.remove_prop。
func RemoveProp(value, prop string) string {
	var out []string
	for _, item := range SplitProps(value) {
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

// UniqueID 复刻 core.unique_id（used 集合由调用方持有并被修改）。
// 注意 re.sub(r"[^A-Za-z0-9_.-]+", "-", base) 把连续非法字符压成一个 "-"。
func UniqueID(base string, used map[string]bool) string {
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
	folder := Dirname(p)
	base := Basename(p)
	if folder != "" {
		return folder + "/" + prefix + base
	}
	return prefix + base
}

// AllocateArchivePath 复刻 core.allocate_archive_path。
func AllocateArchivePath(preferred string, used map[string]bool, prefix string) (string, bool) {
	candidate := preferred
	renamed := false
	if used[candidate] {
		candidate = prefixedArchivePath(preferred, prefix)
		renamed = true
	}
	stem, ext := SplitExt(candidate)
	index := 2
	for used[candidate] {
		candidate = fmt.Sprintf("%s-%d%s", stem, index, ext)
		renamed = true
		index++
	}
	used[candidate] = true
	return candidate, renamed
}

// EscapeText 复刻 xml.sax.saxutils.escape（& 优先，再 > 与 <）。
func EscapeText(data string) string {
	data = strings.ReplaceAll(data, "&", "&amp;")
	data = strings.ReplaceAll(data, ">", "&gt;")
	data = strings.ReplaceAll(data, "<", "&lt;")
	return data
}

// QuoteAttr 复刻 xml.sax.saxutils.quoteattr：值含 `"` 时改用单引号包裹，
// 两种引号并存时把 `"` 转义后仍用双引号包裹。
func QuoteAttr(data string) string {
	data = EscapeText(data)
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

// CollapseSpace 复刻 " ".join(text.split())。
func CollapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
