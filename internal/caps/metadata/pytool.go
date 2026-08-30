// pytool.go 复刻 metadata 依赖的 URL / 路径工具（与 merge/split 包同源）。
package metadata

import (
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

// pyIsExternalURI 复刻 is_external_uri。
func pyIsExternalURI(uri string) bool {
	if p := pyURLSplit(uri); p.scheme != "" {
		return true
	}
	return strings.HasPrefix(uri, "/") || strings.HasPrefix(uri, "//")
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
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

// resolveRelativePath 复刻 epub_lib.resolve_relative_path（% 解码宽容）。
func resolveRelativePath(baseFile, uriPath string) (string, error) {
	decoded := pyUnquote(uriPath)
	return validateArchivePath(path.Join(pyDirname(baseFile), decoded), "resource href")
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

func pyDirname(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

func strPtr(s string) *string { return &s }
