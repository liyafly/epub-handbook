// register.go 收纳 alite 的不可变表（INV-7 白名单）与共享小工具：
// 样式层常量、报告 rawMessage、OPF/manifest 字节区间编辑。
package alite

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// cssManifestIDBase 对齐 unique_id(root, "anthology-refinement-css") 的基名。
const cssManifestIDBase = "anthology-refinement-css"

// idSanitizeRe 对齐 re.sub(r"[^A-Za-z0-9_.-]+", "-", base)。
var idSanitizeRe = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// ---- 正则表（逐条对齐 scripts/epub_anthology_refinement.py 顶部常量；
// INV-7 白名单要求包级 var 只住在本文件） ----

// titleRe 对齐 TITLE_RE（re.I | re.S）。
var titleRe = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title>`)

// bodyRe 对齐 BODY_RE（re.I | re.S）：组 1 开标签、组 2 body、组 3 结束标签。
var bodyRe = regexp.MustCompile(`(?is)(<body\b[^>]*>)(.*?)(</body\s*>)`)

// imgRe 对齐 IMG_RE（re.I | re.S）：组 1 为属性串（贪婪 [^>]* 会吃掉自闭合斜杠，
// 与 Python 一致）。
var imgRe = regexp.MustCompile(`(?is)<img\b([^>]*)/?>`)

// tagRe 对齐 TAG_RE（re.S）。
var tagRe = regexp.MustCompile(`(?s)<!--.*?-->|<[^>]+>`)

// ulListRe 对齐 is_copyright_page 内联的 <ul class*=list> 搜索。
var ulListRe = regexp.MustCompile(`(?i)<ul\b[^>]*\bclass=["'][^"']*\blist\b`)

// cardRe 对齐 refine_copyright 内联的 copyright-card 搜索。
var cardRe = regexp.MustCompile(`(?i)\bclass=["'][^"']*\bcopyright-card\b`)

// headEndRe 对齐 epub_lib.HEAD_END_RE（re.I）。
var headEndRe = regexp.MustCompile(`(?i)</head\s*>`)

// stylesheetRstripped 对齐 stylesheet(...) 调用点的 rstrip() + "\n"。
func stylesheetRstripped(posterImages []posterImageLine) string {
	return strings.TrimRight(stylesheet(posterImages), "\n") + "\n"
}

// stylesheet 复刻 anthology stylesheet()：固定层 + 每卷背景图。
func stylesheet(posterImages []posterImageLine) string {
	lines := []string{
		"/* Anthology volume poster and copyright refinement layer. */",
		"@page {",
		"  margin: 0;",
		"  padding: 0;",
		"}",
		"",
		"html {",
		"  width: 100%;",
		"  height: 100%;",
		"  min-height: 100%;",
		"}",
		"",
		"body.fullpage {",
		"  width: 100%;",
		"  height: 100%;",
		"  min-height: 100%;",
		"  margin: 0;",
		"  padding: 0;",
		"  -webkit-box-sizing: border-box;",
		"  box-sizing: border-box;",
		"  page-break-before: always;",
		"  page-break-after: always;",
		"  page-break-inside: avoid;",
		"  -webkit-page-break-before: always;",
		"  -webkit-page-break-after: always;",
		"  -webkit-page-break-inside: avoid;",
		"  overflow: hidden;",
		"}",
		"",
		"body.poster-bg {",
		"  background-repeat: no-repeat;",
		"  background-position: center center;",
		"  background-size: contain;",
		"}",
		"",
		".fullframe {",
		"  width: 100%;",
		"  height: 100%;",
		"  min-height: 100%;",
		"  margin: 0;",
		"  padding: 0;",
		"  -webkit-box-sizing: border-box;",
		"  box-sizing: border-box;",
		"  overflow: visible;",
		"  page-break-inside: avoid;",
		"  -webkit-page-break-inside: avoid;",
		"}",
		"",
		".poster-fallback {",
		"  display: block;",
		"  width: 100%;",
		"  max-width: 100%;",
		"  height: auto;",
		"  max-height: 100%;",
		"  margin: 0 auto;",
		"  page-break-inside: avoid;",
		"  -webkit-page-break-inside: avoid;",
		"}",
		"",
		"@supports (background-size: contain) {",
		"  body.poster-bg .poster-fallback {",
		"    visibility: hidden;",
		"  }",
		"}",
		"",
		"body.anthology-copyright-page {",
		"  max-width: 36em;",
		"  margin: 0 auto;",
		"  padding: 8% 6%;",
		"  -webkit-box-sizing: border-box;",
		"  box-sizing: border-box;",
		"}",
		"",
		".anthology-copyright-page .copyright-card {",
		"  margin: 0 auto;",
		"}",
		"",
		".anthology-copyright-page .copyright-heading {",
		"  margin: 0 0 1.2em;",
		"  text-indent: 0;",
		"  font-size: 1.25em;",
		"  font-weight: bold;",
		"}",
		"",
		".anthology-copyright-page .copyright-meta {",
		"  margin: 0;",
		"  padding: 0;",
		"  list-style: none;",
		"}",
		"",
		".anthology-copyright-page .copyright-meta-item {",
		"  margin: 0.38em 0;",
		"  padding: 0;",
		"  line-height: 1.55;",
		"  list-style: none;",
		"  text-indent: 0;",
		"}",
		"",
	}
	for _, pi := range posterImages {
		lines = append(lines,
			fmt.Sprintf("body.poster-bg-volume-%03d {", pi.volume),
			fmt.Sprintf("  background-image: url(\"%s\");", pi.href),
			"}",
			"",
		)
	}
	return strings.Join(lines, "\n")
	// 注意：Python 末尾 rstrip() + "\n" 由调用方处理。
}

type posterImageLine struct {
	volume int
	href   string
}

// rawMessage 去 MarshalLegacy 的尾换行并存为 json.RawMessage。
func rawMessage(b []byte) json.RawMessage {
	return json.RawMessage(bytes.TrimSuffix(b, []byte("\n")))
}

func bytesEqualString(b []byte, s string) bool {
	return string(b) == s
}

// loadOPF 定位并读取 OPF。
func loadOPF(b interface {
	Names() []string
	Has(string) bool
	Current(string) ([]byte, error)
}) (string, []byte, error) {
	container, err := b.Current(opf.ContainerPath)
	if err != nil {
		return "", nil, refinementErrf("missing META-INF/container.xml")
	}
	opfPath, err := opf.FindOPFPath(container)
	if err != nil {
		return "", nil, refinementErrf("%v", err)
	}
	data, err := b.Current(opfPath)
	if err != nil {
		return "", nil, refinementErrf("%v", err)
	}
	return opfPath, data, nil
}

// spineXHTMLPaths 复刻 spine_xhtml_paths（只读 OPF 投影）。
// Parse 的路径参数仅用于推导 OPF 目录；href → 容器路径由 OPFDir 手工拼接。
func spineXHTMLPaths(opfData []byte) ([]string, error) {
	p, err := opf.Parse("OEBPS/content.opf", opfData)
	if err != nil {
		return nil, refinementErrf("%v", err)
	}
	byID := map[string]opf.ManifestItem{}
	for _, it := range p.Manifest {
		if it.ID != "" {
			byID[it.ID] = it
		}
	}
	var paths []string
	for _, ref := range p.Spine {
		item, ok := byID[ref.IDRef]
		if !ok || item.MediaType != "application/xhtml+xml" || item.Href == "" {
			continue
		}
		paths = append(paths, normJoin(p.OPFDir(), item.Href))
	}
	return paths, nil
}

// manifestItemEdit 生成在 </manifest> 前追加 anthology CSS item 的编辑。
// 返回 added=false 表示 manifest 已有同 href 项（对齐 add_css_manifest_item）。
func manifestItemEdit(opfPath string, opfData []byte, opfDir, cssZipPath string) (bool, *editset.Edit, error) {
	root, err := opf.ScanSpanTree(opfData)
	if err != nil {
		return false, nil, refinementErrf("%v", err)
	}
	href := cssZipPath
	if opfDir != "" {
		href = pyRelPath(cssZipPath, opfDir)
	}
	var manifest *opf.SpanNode
	idSeen := map[string]bool{}
	for _, m := range root.Kids {
		if m.Name.Space != opf.OPFURI || m.Name.Local != "manifest" {
			continue
		}
		manifest = m
		for _, c := range m.Kids {
			if c.Name.Space != opf.OPFURI || c.Name.Local != "item" {
				continue
			}
			if h, ok := c.AttrByLocal("", "href"); ok && h == href {
				return false, nil, nil
			}
			if id, ok := c.AttrByLocal("", "id"); ok {
				idSeen[id] = true
			}
		}
	}
	if manifest == nil {
		return false, nil, refinementErrf("OPF missing manifest")
	}
	itemID := uniqueID(idSeen, cssManifestIDBase)
	elem := `<item id="` + attribEscape(itemID) + `" href="` + attribEscape(href) + `" media-type="text/css" />`
	edit := editset.Insert(opfPath, int64(manifest.Close.Start), []byte(elem))
	return true, &edit, nil
}

// uniqueID 复刻 epub_lib.unique_id。
func uniqueID(idSeen map[string]bool, base string) string {
	candidate := idSanitizeRe.ReplaceAllString(base, "-")
	candidate = strings.Trim(candidate, "-")
	if candidate == "" {
		candidate = "item"
	}
	if len(candidate) > 0 && candidate[0] >= '0' && candidate[0] <= '9' {
		candidate = "x-" + candidate
	}
	index := 2
	result := candidate
	for idSeen[result] {
		result = candidate + "-" + itoa(index)
		index++
	}
	return result
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func attribEscape(v string) string {
	v = strings.ReplaceAll(v, "&", "&amp;")
	v = strings.ReplaceAll(v, "<", "&lt;")
	v = strings.ReplaceAll(v, ">", "&gt;")
	return strings.ReplaceAll(v, `"`, "&quot;")
}

// ---- 路径工具（与 Python posixpath 对齐） ----

func pyDirname(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

// normJoin 对齐 epub_lib.norm_join（去 fragment 后 join+normpath）。
func normJoin(base, href string) string {
	clean := href
	if i := strings.IndexByte(clean, '#'); i >= 0 {
		clean = clean[:i]
	}
	if base == "" {
		return pyNormPath(clean)
	}
	return pyNormPath(base + "/" + clean)
}

func pyNormPath(p string) string {
	parts := strings.Split(p, "/")
	var out []string
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
				continue
			}
			out = append(out, part)
		default:
			out = append(out, part)
		}
	}
	joined := strings.Join(out, "/")
	if strings.HasPrefix(p, "/") && !strings.HasPrefix(joined, "/") {
		return "/" + joined
	}
	if joined == "" {
		return "."
	}
	return joined
}

// relHref 复刻 epub_lib.rel_href。
func relHref(fromZipPath, toZipPath string) string {
	base := pyDirname(fromZipPath)
	if base == "" {
		return toZipPath
	}
	return pyRelPath(toZipPath, base)
}

// pyRelPath 复刻 posixpath.relpath。
func pyRelPath(target, base string) string {
	tParts := strings.Split(pyNormPath(target), "/")
	bParts := strings.Split(pyNormPath(base), "/")
	i := 0
	for i < len(bParts) && i < len(tParts) && bParts[i] == tParts[i] {
		i++
	}
	var out []string
	for range bParts[i:] {
		out = append(out, "..")
	}
	out = append(out, tParts[i:]...)
	if len(out) == 0 {
		return "."
	}
	return strings.Join(out, "/")
}

// decodeUTF8Replace 对齐 decode("utf-8", errors="replace")。
func decodeUTF8Replace(data []byte) string {
	return string(bytes.ToValidUTF8(data, []byte("\uFFFD")))
}

// findRepoRoot 向上找含 contracts 的目录（与其它 caps 一致）。
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "contracts", "capabilities", "v1")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("alite: 未找到仓库根")
		}
		dir = parent
	}
}

var _ = report.StatusComplete
