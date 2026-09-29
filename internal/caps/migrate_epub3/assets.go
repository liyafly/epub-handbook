// assets.go 提供本 capability 使用的静态资源与字节级文本工具：note.png
// 图标（优先读取 skills 资产，回退内置 base64，对齐 core.note_png_bytes）、
// Python「utf-8 errors=replace」解码语义与 body 字体锁定探测。
package migrateepub3

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// notePNGBase64 是 core.py 内置的 note.png 回退字节。
const notePNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAwAAAAMCAYAAABWdVznAAAAHklEQVR4nGNgGAWjYBSMglEwCkbBKBgFo2AUDAMABRwAAf1xD6YAAAAASUVORK5CYII="

// notePNGBytes 复刻 core.note_png_bytes：优先读取仓库的 note.png 资产，
// 否则使用内置 base64 回退值。
func notePNGBytes() []byte {
	dir, err := os.Getwd()
	if err == nil {
		for {
			candidate := filepath.Join(dir, "skills", "epub-cleanup", "assets", "note.png")
			if data, rerr := os.ReadFile(candidate); rerr == nil {
				return data
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	data, derr := base64.StdEncoding.DecodeString(notePNGBase64)
	if derr != nil {
		return []byte{}
	}
	return data
}

// utf8ReplaceDecode 复刻 bytes.decode("utf-8", errors="replace")：
// 按 Unicode「最长合法子串」建议，每个非法子段产出一个 U+FFFD
// （截断的多字节前缀整体算一个子段，与 CPython 探针一致）。
func utf8ReplaceDecode(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	var b strings.Builder
	i := 0
	for i < len(data) {
		c := data[i]
		if c < 0x80 {
			b.WriteByte(c)
			i++
			continue
		}
		size, firstLo, firstHi := utf8StartSpec(c)
		if size == 0 {
			b.WriteRune(utf8.RuneError)
			i++
			continue
		}
		n := 1
		for n < size {
			if i+n >= len(data) {
				break
			}
			cc := data[i+n]
			lo, hi := byte(0x80), byte(0xBF)
			if n == 1 {
				lo, hi = firstLo, firstHi
			}
			if cc < lo || cc > hi {
				break
			}
			n++
		}
		if n == size {
			r, sz := utf8.DecodeRune(data[i : i+size])
			if int(sz) == size {
				b.WriteRune(r)
				i += size
				continue
			}
		}
		b.WriteRune(utf8.RuneError)
		i += n
	}
	return b.String()
}

// utf8StartSpec 返回 (期望总长, 首个续字节的下界, 上界)；非法起始字节返回 (0,0,0)。
func utf8StartSpec(c byte) (int, byte, byte) {
	switch {
	case c >= 0xC2 && c <= 0xDF:
		return 2, 0x80, 0xBF
	case c == 0xE0:
		return 3, 0xA0, 0xBF
	case c >= 0xE1 && c <= 0xEC:
		return 3, 0x80, 0xBF
	case c == 0xED:
		return 3, 0x80, 0x9F // 代理区编码非法
	case c >= 0xEE && c <= 0xEF:
		return 3, 0x80, 0xBF
	case c == 0xF0:
		return 4, 0x90, 0xBF
	case c >= 0xF1 && c <= 0xF3:
		return 4, 0x80, 0xBF
	case c == 0xF4:
		return 4, 0x80, 0x8F
	}
	return 0, 0, 0
}

// bodyFontLockedWord 检查 class 值里是否有词边界包裹的 body-font-locked。
func bodyFontLockedWord(value []byte) bool {
	const word = "body-font-locked"
	for i := 0; i+len(word) <= len(value); i++ {
		if !bytes.Equal(value[i:i+len(word)], []byte(word)) {
			continue
		}
		if i > 0 && isASCIILetterByte(value[i-1]) {
			continue
		}
		if i+len(word) < len(value) && isASCIILetterByte(value[i+len(word)]) {
			continue
		}
		return true
	}
	return false
}

func isASCIILetterByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// skipPySpaceBytes 跳过字节模式的 \s（[ \t\n\r\f\v]）。
func skipPySpaceBytes(data []byte, i int) int {
	for i < len(data) {
		switch data[i] {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			i++
		default:
			return i
		}
	}
	return i
}

// bodyFontLockedInXHTML 逐字复刻 BODY_FONT_LOCKED_RE 的字节搜索：
// <body[^>]*\bclass\s*=\s*(['"])[^'"]*\bbody-font-locked\b[^'"]*\1（区分大小写）。
func bodyFontLockedInXHTML(data []byte) bool {
	const needle = "<body"
	for i := 0; i < len(data); {
		idx := bytes.Index(data[i:], []byte(needle))
		if idx < 0 {
			return false
		}
		start := i + idx
		p := start + len(needle)
		end := p
		for end < len(data) && data[end] != '>' {
			end++
		}
		// 在 [p, end] 范围内查找任何能让整体匹配成功的 \bclass= 位置。
		for q := p; q+5 <= end; q++ {
			if !wordBoundaryBytes(data, q) {
				continue
			}
			if !bytes.HasPrefix(data[q:], []byte("class")) {
				continue
			}
			j := skipPySpaceBytes(data, q+5)
			if j >= len(data) || data[j] != '=' {
				continue
			}
			j = skipPySpaceBytes(data, j+1)
			if j >= len(data) || (data[j] != '\'' && data[j] != '"') {
				continue
			}
			vEnd := j + 1
			for vEnd < len(data) && data[vEnd] != '\'' && data[vEnd] != '"' {
				vEnd++
			}
			if bodyFontLockedWord(data[j+1 : vEnd]) {
				return true
			}
		}
		i = start + len(needle)
	}
	return false
}

func wordBoundaryBytes(data []byte, i int) bool {
	before := i > 0 && isASCIILetterByte(data[i-1])
	after := i < len(data) && isASCIILetterByte(data[i])
	return before != after
}

// stripCSSComments 复刻 re.sub(r"/\*.*?\*/", "", css, flags=re.S)。
func stripCSSComments(css string) string {
	if !strings.Contains(css, "/*") {
		return css
	}
	var b strings.Builder
	i := 0
	for i < len(css) {
		idx := strings.Index(css[i:], "/*")
		if idx < 0 {
			b.WriteString(css[i:])
			break
		}
		b.WriteString(css[i : i+idx])
		start := i + idx
		end := strings.Index(css[start+2:], "*/")
		if end < 0 {
			// 该 /* 无闭合：Python 的 finditer 会继续向后扫，但不存在
			// 能成功的起点（*/ 缺失），此后不再有可剥离的注释。
			b.WriteString(css[start:])
			break
		}
		i = start + 2 + end + 2
	}
	return b.String()
}
