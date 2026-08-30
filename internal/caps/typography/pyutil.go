// pyutil.go 私有复刻 Python 侧 posixpath / 文本处理语义。caps 互不
// import（SPEC §1），因此每个迁移包自带一份；逐条对齐 scripts/epub_lib.py
// 与 Python 标准库行为，保证 parity 产物字节一致。
package typography

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// decodeUTF8Replace 复刻 bytes.decode("utf-8", errors="replace")：
// 非法子序列替换为 U+FFFD。
func decodeUTF8Replace(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	return strings.ToValidUTF8(string(data), "\uFFFD")
}

// pyDirname / pyBasename 复刻 posixpath.dirname / basename。
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

// pyJoinPath 复刻 posixpath.join(base, name)（本工具只用两段形态）。
func pyJoinPath(base, name string) string {
	if strings.HasPrefix(name, "/") {
		return name
	}
	if base == "" || strings.HasSuffix(base, "/") {
		return base + name
	}
	return base + "/" + name
}

// pyNormPath 复刻 posixpath.normpath（保留一或两层前导斜杠，
// ".." 只在非绝对且无前段时保留）。
func pyNormPath(p string) string {
	if p == "" {
		return "."
	}
	initialSlashes := 0
	if strings.HasPrefix(p, "/") {
		initialSlashes = 1
		if strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///") {
			initialSlashes = 2
		}
	}
	comps := strings.Split(p, "/")
	var out []string
	for _, comp := range comps {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initialSlashes == 0 && len(out) == 0) {
			out = append(out, comp)
		} else if len(out) > 0 && out[len(out)-1] != ".." {
			out = out[:len(out)-1]
		} else if initialSlashes == 0 {
			out = append(out, "..")
		}
	}
	joined := strings.Join(out, "/")
	if initialSlashes > 0 {
		joined = strings.Repeat("/", initialSlashes) + joined
	}
	if joined == "" {
		return "."
	}
	return joined
}

// normJoin 复刻 epub_lib.norm_join：先去掉 fragment 再 join + normpath。
func normJoin(base, href string) string {
	clean := href
	if i := strings.IndexByte(href, '#'); i >= 0 {
		clean = href[:i]
	}
	return pyNormPath(pyJoinPath(base, clean))
}

// pySplitExt 复刻 posixpath.splitext（basename 前导点不算扩展名）。
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

func pathStem(p string) string {
	stem, _ := pySplitExt(p)
	return stem
}

// pyPathStem 复刻 pathlib.Path(p).stem：basename 的 stem
// （Path("OEBPS/Styles/a.css").stem == "a"，与 posixpath.splitext 的
// 全路径 stem 不同）。
func pyPathStem(p string) string {
	stem, _ := pySplitExt(pyBasename(p))
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

// relHref 复刻 epub_lib.rel_href。
func relHref(fromZipPath, toZipPath string) string {
	base := pyDirname(fromZipPath)
	if base == "" {
		return toZipPath
	}
	return pyRelPath(toZipPath, base)
}

// ---- 文本处理（Python str 语义） ----

func isSpaceRune(r rune) bool { return unicode.IsSpace(r) }

// pyStrip 复刻 str.strip()（无参：剥两侧 Unicode 空白）。
func pyStrip(s string) string { return strings.TrimFunc(s, func(r rune) bool { return isSpaceRune(r) }) }

// pyRStrip 复刻 str.rstrip()。
func pyRStrip(s string) string {
	return strings.TrimRightFunc(s, func(r rune) bool { return isSpaceRune(r) })
}

// normalizeSpace 复刻 re.sub(r"\s+", " ", value).strip()。
func normalizeSpace(value string) string {
	var b strings.Builder
	started := false
	pendingSpace := false
	for _, r := range value {
		if isSpaceRune(r) {
			if started {
				pendingSpace = true
			}
			continue
		}
		if pendingSpace {
			b.WriteByte(' ')
			pendingSpace = false
		}
		started = true
		b.WriteRune(r)
	}
	return b.String()
}

// removeAllSpace 复刻 re.sub(r"\s+", "", value)。
func removeAllSpace(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if !isSpaceRune(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---- 正则语义助手（RE2 无反向引用/前瞻处手工实现） ----

// isWordRune 对齐 Python \w（字母、数字、下划线，Unicode 感知）。
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// prevIsWord 报告 text[i] 之前的字符是否属于 \w。
func prevIsWord(text string, i int) bool {
	if i <= 0 || i > len(text) {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(text[:i])
	return isWordRune(r)
}

// wordBoundaryAt 对齐 Python \b（位置 i 处的词边界）。
func wordBoundaryAt(text string, i int) bool {
	before := prevIsWord(text, i)
	after := false
	if i < len(text) {
		r, _ := utf8.DecodeRuneInString(text[i:])
		after = isWordRune(r)
	}
	return before != after
}

// skipPySpace 跳过 Unicode 空白（含换行），对齐正则 \s* 的贪心消耗。
func skipPySpace(text string, i int) int {
	for i < len(text) {
		r, size := utf8.DecodeRuneInString(text[i:])
		if !isSpaceRune(r) {
			break
		}
		i += size
	}
	return i
}

// indexFold 返回 s[from:] 中首个与 sub 大小写不敏感匹配的位置。
func indexFold(s, sub string, from int) int {
	n := len(sub)
	if n == 0 {
		return from
	}
	for i := from; i+n <= len(s); i++ {
		if strings.EqualFold(s[i:i+n], sub) {
			return i
		}
	}
	return -1
}

// pyRepr 近似 Python 的 str repr（错误消息里的 {layer!r}）。
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
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// decodeRune / decodeLastRune 是 utf8 包的便捷封装。
func decodeRune(s string) (rune, int) { return utf8.DecodeRuneInString(s) }

func decodeLastRune(s string) (rune, int) { return utf8.DecodeLastRuneInString(s) }

func utf8Valid(data []byte) bool { return utf8.Valid(data) }

// pyLineCount 复刻 len(text.splitlines())：按 \r\n / \r / \n 以及
// \v \f \x1c \x1d \x1e \x85 \u2028 \u2029 切行的行数。
func pyLineCount(text string) int {
	if text == "" {
		return 0
	}
	n := 0
	i := 0
	for i < len(text) {
		r, size := utf8.DecodeRuneInString(text[i:])
		i += size
		switch r {
		case '\r':
			if i < len(text) && text[i] == '\n' {
				i++
			}
			n++
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			n++
		}
	}
	return n
}
