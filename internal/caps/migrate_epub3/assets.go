// assets.go 提供本 capability 注入的静态资源与字节级文本工具：
// CJK 排版覆盖样式常量（逐字节对齐 core.enhancement_css()）、note.png
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
const enhancementCSS = `/* EPUB 3 CJK literary cleanup layer. Keep linked after source stylesheets. */
@namespace epub "http://www.idpf.org/2007/ops";

html,
body {
  margin: 0;
  padding: 0;
}

body {
  line-height: 1.65;
  text-align: justify;
  text-justify: inter-ideograph;
  word-break: normal;
  overflow-wrap: anywhere;
  color: #1f1a17;
}

p {
  margin: 0.35em 0;
  line-height: 1.65;
  text-indent: 2em;
}

h1,
h2,
h3,
h4,
h5,
.type-title,
.cp,
.front,
.back,
.zw-text1,
.chapter-title1,
.fronttitle1,
.backtitle1,
.backtitle2,
.kindle-cn-toc-title,
.kindle-en-toc-title {
  font-family: "Heiti SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif;
  text-indent: 0;
  page-break-after: avoid;
  break-after: avoid;
  page-break-inside: avoid;
  break-inside: avoid;
}

.type-body {
  font-family: "Songti SC", "SimSun", "Noto Serif CJK SC", serif;
}

.type-subtitle {
  font-family: "Heiti SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif;
  font-weight: normal;
  text-indent: 0;
}

.type-quote,
.type-meta {
  font-family: "STFangsong", "FangSong", "Noto Serif CJK SC", serif;
}

.type-note,
.type-emphasis {
  font-family: "Kaiti SC", "STKaiti", "KaiTi", serif;
}

h1,
h1.front,
h1.back,
h1.zw-text1 {
  margin: 1.4em auto 1.2em;
  color: #6f4d35;
  line-height: 1.35;
}

h2,
h3,
h4,
h5 {
  margin: 1.1em 0 0.75em;
  color: #6f4d35;
  line-height: 1.4;
}

.part-text,
.part-textc,
.part-textf,
.block,
.block1,
.block2,
.block3,
.img,
.note,
.footnote,
.fs,
.kt,
.kh {
  font-family: "Kaiti SC", "STKaiti", "KaiTi", serif;
}

.center,
.block2,
.img,
.cover {
  text-align: center;
  text-indent: 0;
}

.right,
.block1 {
  text-align: right;
}

.left {
  text-align: left;
  text-indent: 0;
}

blockquote,
.block,
.block3 {
  margin: 0.8em 0 0.8em 2em;
}

img {
  max-width: 100%;
  height: auto;
}

.cover img,
img.cover,
img.body-image-alone {
  max-width: 100%;
  height: auto;
}

a {
  color: inherit;
}

sup.note-marker {
  font-size: 1em;
  line-height: 0;
  vertical-align: baseline;
}

sup.note-marker > .noteref-icon {
  display: inline-block;
  line-height: 0;
  position: relative;
  top: -0.14em;
  text-decoration: none;
}

sup.note-marker > .noteref-icon > img {
  display: block;
  width: auto;
  height: 0.72em;
  max-width: none;
}

aside[epub|type~="footnote"],
aside[role~="doc-footnote"] {
  margin-top: 1.4em;
}

.footnote-line,
hr.xian {
  width: 60%;
  height: 1px;
  margin: 1.5em 0 1em -0.5em;
  border: none;
  border-top: 1px solid #777;
}

.footnote-list {
  margin: 0;
  padding: 0;
  list-style-type: none;
  text-align: left;
}

.footnote-item {
  margin: 0.4em 0;
  padding: 0;
  list-style-type: none;
}

.footnote {
  margin: 0.4em 0;
  text-indent: 0;
  font-size: 0.9em;
  line-height: 1.45;
  text-align: left;
}

.footnote-back {
  margin-right: 0.25em;
  text-decoration: none;
}
`

// notePNGBytes 复刻 core.note_png_bytes：NOTE_ASSET（仓库根
// skills/epub-popup-footnote-converter/assets/note.png）存在时读盘，
// 否则回退内置 base64。Python 侧按脚本位置定位资产；Go 侧从当前工作
// 目录向上找仓库根（含 go.mod 的目录），找不到再用回退字节。
func notePNGBytes() []byte {
	dir, err := os.Getwd()
	if err == nil {
		for {
			candidate := filepath.Join(dir, "skills", "epub-popup-footnote-converter", "assets", "note.png")
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
