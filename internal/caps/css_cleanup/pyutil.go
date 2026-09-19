// pyutil.go 私有复刻 Python 侧 posixpath / 文本处理语义。caps 互不
// import（SPEC §1），因此每个迁移包自带一份；逐条对齐 scripts/epub_lib.py
// 与 Python 标准库行为，保证 parity 产物字节一致。
package csscleanup

import (
	"strings"
	"unicode"
)

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
