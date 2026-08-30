// util.go 收纳 Python 语义的字符串/路径工具。
// 说明：caps 包之间禁止互相 import，normJoin/isPySpace 等工具在本包内
// 有意自持一份（与 image_layout 包重复），这是层级隔离的代价而非疏漏。
package contentanalyze

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// dirName 复刻 posixpath.dirname（Go path.Dir 对无斜杠路径返回 "."，不同）。
func dirName(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

// normJoin 复刻 epub_lib.norm_join：剥 #fragment 后 posixpath.join + normpath。
// 与 opf.ResolveHref 不同：这里不做百分号解码（Python 侧同样不解码）。
func normJoin(base, href string) string {
	clean := href
	if i := strings.IndexByte(clean, '#'); i >= 0 {
		clean = clean[:i]
	}
	var joined string
	switch {
	case strings.HasPrefix(clean, "/"):
		joined = clean // posixpath.join：绝对分量直接替换
	case base == "":
		joined = clean
	default:
		joined = strings.TrimSuffix(base, "/") + "/" + clean
	}
	return normPath(joined)
}

// normPath 复刻 posixpath.normpath：折叠 "."/".." 与重复斜杠，
// 保留恰好两个前导斜杠的 POSIX 特例，根路径的 ".." 直接丢弃。
func normPath(p string) string {
	if p == "" {
		return "."
	}
	rooted := false
	doubleSlash := false
	if strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///") {
		doubleSlash = true
	} else if strings.HasPrefix(p, "/") {
		rooted = true
	}
	p = strings.TrimLeft(p, "/")
	var out []string
	for _, part := range strings.Split(p, "/") {
		switch part {
		case "", ".":
		case "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
			} else if !rooted && !doubleSlash {
				out = append(out, "..")
			}
		default:
			out = append(out, part)
		}
	}
	joined := strings.Join(out, "/")
	switch {
	case doubleSlash:
		return "//" + joined
	case rooted:
		if joined == "" {
			return "/"
		}
		return "/" + joined
	default:
		if joined == "" {
			return "."
		}
		return joined
	}
}

// isPySpace 覆盖 Python str.isspace / 正则 \s 的全集：
// unicode.White_Space + \x1c-\x1f。
func isPySpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// pyTrimSpace 复刻 str.strip()（按 Unicode 空白去两端）。
func pyTrimSpace(s string) string {
	return strings.TrimFunc(s, isPySpace)
}

// splitPyFields 复刻 str.split()（按 Unicode 空白切分，无空元素）。
func splitPyFields(value string) []string {
	fields := strings.FieldsFunc(value, isPySpace)
	if fields == nil {
		return []string{}
	}
	return fields
}

// pySplitLines 复刻 str.splitlines() 的行边界全集：
// \n \r \r\n \v \f \x1c \x1d \x1e \x85 \u2028 \u2029，且末尾边界不产生空尾行。
func pySplitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '\r':
			out = append(out, s[start:i])
			i += size
			if i < len(s) && s[i] == '\n' {
				i++
			}
			start = i
		case '\n', '\v', '\f', '\x85', '\u2028', '\u2029', '\x1c', '\x1d', '\x1e':
			out = append(out, s[start:i])
			i += size
			start = i
		default:
			i += size
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// runeCut 按码点截断（Python text[:n] 语义，不是按字节）。
func runeCut(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

// sourceSuffix 复刻 pathlib.Path(source).suffix.lower()：
// 取文件名里最后一个点，且点后须还有字符；点开头（dotfile）不算后缀。
func sourceSuffix(source string) string {
	base := source
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	if i := strings.LastIndexByte(strings.TrimSuffix(base, "/"), '.'); i > 0 && i < len(base)-1 {
		return strings.ToLower(base[i:])
	}
	return ""
}
