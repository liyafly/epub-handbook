// Package alite 移植 epub.alite.convert（scripts/epub_anthology_refinement.py）：
// 文集本海报页（封面）与相邻版权页的精排，含固定样式层注入。
//
// XHTML 层全部是最小 diff 字符串变换（Python 同为字符串替换）；
// OPF 只做 manifest 追加一项，走字节区间编辑（INV-2：不整文档重序列化；
// Python 是 ET 整树重排，OPF 格式差异在 parity 测试与 tools/parity/allow.md 登记）。
package alite

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/xhtml"
)

// CapabilityID 是本能力的契约 id。
const CapabilityID = "epub.alite.convert"

// ErrRefinement 对齐 RefinementError：精排无法继续。
var ErrRefinement = errors.New("alite: refinement error")

func refinementErrf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrRefinement, fmt.Sprintf(format, args...))
}

// Params 是本能力的参数。
type Params struct {
	// ExpectVolumes 对齐 --expect-volumes；nil 表示不校验。
	ExpectVolumes *int
	// Output 是 pipeline 透传的输出路径；本包不落盘（INV-3），也不再
	// 把它写进报告，仅保留字段以维持 Params 形状。
	Output string
}

// refinementReport 是精排结果的内部累积形态，字段一一映射到 facts。
type refinementReport struct {
	OPF                   string
	PosterPagesRefined    int
	CopyrightPagesRefined int
	StylesheetsAdded      int
	PosterPages           []string
	CopyrightPages        []string
	Warnings              []string
	// ScanWarnings 是 xhtml.ScanRegions 截断告警（见 ensureStylesheetLink /
	// addClassToTag / hasClassToken），与 Warnings（无相邻版权页）分开计数，
	// 各自映射独立的 finding ID，互不干扰既有 alite.no-copyright 断言。
	ScanWarnings []string
}

// Run 执行精排（SPEC §6.1 三段式）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	res := report.Result{Capability: CapabilityID, Status: report.StatusComplete}
	rep := refinementReport{
		PosterPages:    []string{},
		CopyrightPages: []string{},
		Warnings:       []string{},
		ScanWarnings:   []string{},
	}

	opfPath, opfData, err := loadOPF(b)
	if err != nil {
		return report.Result{}, err
	}
	rep.OPF = opfPath
	opfDir := pyDirname(opfPath)
	cssZipPath := normJoin(opfDir, "Styles/anthology-refinement.css")

	paths, err := spineXHTMLPaths(opfData)
	if err != nil {
		return report.Result{}, err
	}

	type candidate struct {
		poster        string
		imageHref     string
		copyrightPath string // 可为空
	}
	var candidates []candidate
	for i, path := range paths {
		if !b.Has(path) {
			continue
		}
		raw, err := b.Current(path)
		if err != nil {
			continue
		}
		text := decodeUTF8Replace(raw)
		imageHref := posterImageHref(text)
		if imageHref == "" {
			continue
		}
		copyrightPath := ""
		if i+1 < len(paths) {
			cp := paths[i+1]
			if b.Has(cp) {
				cRaw, err := b.Current(cp)
				if err == nil && isCopyrightPage(decodeUTF8Replace(cRaw)) {
					copyrightPath = cp
				}
			}
		}
		candidates = append(candidates, candidate{path, imageHref, copyrightPath})
	}

	if p.ExpectVolumes != nil && len(candidates) != *p.ExpectVolumes {
		return report.Result{}, refinementErrf("expected %d volume poster pages, found %d",
			*p.ExpectVolumes, len(candidates))
	}
	if len(candidates) == 0 {
		return report.Result{}, refinementErrf("no single-image volume poster pages found")
	}

	var edits []editset.Edit
	var posterImages []posterImageLine
	for volume, cand := range candidates {
		vol := volume + 1
		styleHref := relHref(cand.poster, cssZipPath)
		raw, err := b.Current(cand.poster)
		if err != nil {
			return report.Result{}, err
		}
		refined, posterWarnings, err := refinePoster(decodeUTF8Replace(raw), vol, cand.imageHref, styleHref)
		if err != nil {
			return report.Result{}, err
		}
		rep.ScanWarnings = append(rep.ScanWarnings, prefixWarnings(cand.poster, posterWarnings)...)
		edits = append(edits, editset.Replace(cand.poster, 0, int64(len(raw)), []byte(refined)))
		rep.PosterPages = append(rep.PosterPages, cand.poster)

		imageZipPath := normJoin(pyDirname(cand.poster), cand.imageHref)
		posterImages = append(posterImages, posterImageLine{vol, relHref(cssZipPath, imageZipPath)})

		if cand.copyrightPath != "" {
			cRaw, err := b.Current(cand.copyrightPath)
			if err != nil {
				return report.Result{}, err
			}
			copyrightStyleHref := relHref(cand.copyrightPath, cssZipPath)
			cRefined, copyrightWarnings, err := refineCopyright(decodeUTF8Replace(cRaw), copyrightStyleHref)
			if err != nil {
				return report.Result{}, err
			}
			rep.ScanWarnings = append(rep.ScanWarnings, prefixWarnings(cand.copyrightPath, copyrightWarnings)...)
			edits = append(edits, editset.Replace(cand.copyrightPath, 0, int64(len(cRaw)), []byte(cRefined)))
			rep.CopyrightPages = append(rep.CopyrightPages, cand.copyrightPath)
		} else {
			rep.Warnings = append(rep.Warnings,
				fmt.Sprintf("poster page has no adjacent copyright page: %s", cand.poster))
		}
	}
	rep.PosterPagesRefined = len(rep.PosterPages)
	rep.CopyrightPagesRefined = len(rep.CopyrightPages)

	// CSS 层：不存在则新建，存在则整文件替换（Python files[css_zip_path] = ...）。
	cssText := stylesheetRstripped(posterImages)
	if b.Has(cssZipPath) {
		cur, err := b.Current(cssZipPath)
		if err != nil {
			return report.Result{}, err
		}
		if !bytesEqualString(cur, cssText) {
			edits = append(edits, editset.Replace(cssZipPath, 0, int64(len(cur)), []byte(cssText)))
		}
	} else {
		edits = append(edits, editset.Replace(cssZipPath, 0, 0, []byte(cssText)))
	}

	// manifest 追加（字节区间编辑；语义对齐 add_css_manifest_item）。
	added, manifestEdit, err := manifestItemEdit(opfPath, opfData, opfDir, cssZipPath)
	if err != nil {
		return report.Result{}, err
	}
	if added {
		edits = append(edits, *manifestEdit)
		rep.StylesheetsAdded = 1
	}

	if err := b.Apply(edits); err != nil {
		return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
	}

	res.Facts = map[string]any{
		"opf":                   rep.OPF,
		"posterPagesRefined":    rep.PosterPagesRefined,
		"copyrightPagesRefined": rep.CopyrightPagesRefined,
		"stylesheetsAdded":      rep.StylesheetsAdded,
		"posterPages":           rep.PosterPages,
		"copyrightPages":        rep.CopyrightPages,
		"warnings":              rep.Warnings,
		"scanWarnings":          rep.ScanWarnings,
	}
	for _, w := range rep.Warnings {
		res.Findings = append(res.Findings, report.Finding{
			Level: "warn", ID: "alite.no-copyright", Title: w,
		})
	}
	for _, w := range rep.ScanWarnings {
		res.Findings = append(res.Findings, report.Finding{
			Level: "warn", ID: "alite.markup-scan-truncated", Title: w,
		})
	}
	return res, nil
}

// ---- 页面识别与改写（逐行复刻 Python） ----

// readQuotedAttr 精确复刻 Python `\bname=(["'])([^"']*)(\1)`（re.I）：
// = 后必须紧跟引号，值内不允许任何引号字符，按同引号闭合。
// minChars 对齐值组的重复次数（SRC_RE 为 +，CLASS_RE 为 *）。
func readQuotedAttr(attrs, name string, minChars int) (string, bool) {
	_, _, value, ok := quotedAttrSpan(attrs, name, minChars)
	return value, ok
}

// quotedAttrSpan 返回首个完整匹配的 [起点, 终点)（含引号）与值。
func quotedAttrSpan(attrs, name string, minChars int) (int, int, string, bool) {
	lower := strings.ToLower(attrs)
	n := len(name)
	for i := 0; i+n+1 <= len(attrs); i++ {
		if lower[i:i+n] != name || !hasWordBoundaryBefore(attrs, i) {
			continue
		}
		eq := i + n
		if eq >= len(attrs) || attrs[eq] != '=' {
			continue
		}
		if eq+1 >= len(attrs) {
			return 0, 0, "", false
		}
		q := attrs[eq+1]
		if q != '"' && q != '\'' {
			continue
		}
		j := eq + 2
		for j < len(attrs) && attrs[j] != '"' && attrs[j] != '\'' {
			j++
		}
		if j >= len(attrs) || attrs[j] != q || j-(eq+2) < minChars {
			continue // 值内出现任一引号或未按同引号闭合 → 该起点匹配失败
		}
		return i, j + 1, attrs[eq+2 : j], true
	}
	return 0, 0, "", false
}

// hasWordBoundaryBefore 对齐 Python \b：词字符仅 [A-Za-z0-9_]。
func hasWordBoundaryBefore(s string, i int) bool {
	if i == 0 {
		return true
	}
	return !isPyWordByte(s[i-1])
}

func isPyWordByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// classTokens 对齐 class_tokens（首个 CLASS_RE 匹配的引号内分词）。
func classTokens(attrs string) []string {
	v, ok := readQuotedAttr(attrs, "class", 0)
	if !ok {
		return nil
	}
	return strings.Fields(v)
}

// addClassToAttrs 对齐 add_class_to_attrs：替换首个 CLASS_RE 匹配区间为
// class="joined"（统一双引号）；无匹配则尾部追加。
func addClassToAttrs(attrs, className string) (string, bool) {
	classes := classTokens(attrs)
	for _, c := range classes {
		if c == className {
			return attrs, false
		}
	}
	classes = append(classes, className)
	joined := strings.Join(classes, " ")
	if start, end, _, ok := quotedAttrSpan(attrs, "class", 0); ok {
		return attrs[:start] + `class="` + joined + `"` + attrs[end:], true
	}
	return attrs + ` class="` + className + `"`, true
}

// addClassToTag 对齐 add_class_to_tag：pattern = <tag\b(?P<attrs>[^>]*)>（re.I），
// required_class 不在首个 class 属性分词中则原样保留。
//
// 原实现手写 strings.Index(lower, "<"+tag) 扫描全文，对注释 / CDATA /
// <script> 里同形的 `<tag …>` 文字没有排除；`>` 的定位也不感知引号，
// 属性值里出现 `>` 会截断标签。改为基于 xhtml.ScanRegions 的 RegionTag：
// 只在真实标签字节内匹配，引号感知天然继承自扫描器。
//
// 命中匹配标签（tag 名 + 满足 requiredClass）时始终按 `<tag newAttrs>`
// 原样输出（tag 用调用方传入的规范小写名，不用原文大小写），即使
// className 已存在也照常重写——与 Python/原 Go 实现一致（未改动此侧写）。
//
// 第二个返回值是告警：区域扫描截断时，截断点之后的 <tag> 不再加 class，
// 调用方必须转成 finding，不能静默半改。
func addClassToTag(value, tag, className, requiredClass string) (string, []string) {
	regions, stop := xhtml.ScanRegions(value)
	var warnings []string
	if stop != xhtml.ScanComplete {
		warnings = append(warnings, fmt.Sprintf(
			"markup scan stopped at byte offset %d (unterminated comment/CDATA/PI/declaration/tag or unclosed style/script); <%s> class additions after this offset left unchanged",
			stop, tag))
	}
	if len(regions) == 0 {
		return value, warnings
	}
	var out strings.Builder
	out.Grow(len(value))
	last := 0
	for _, r := range regions {
		if r.Kind != xhtml.RegionTag {
			continue
		}
		name, attrs, closing := xhtml.TagParts(value[r.Start:r.End])
		if closing || !strings.EqualFold(name, tag) {
			continue
		}
		if requiredClass != "" && !containsToken(classTokens(attrs), requiredClass) {
			continue
		}
		newAttrs, _ := addClassToAttrs(attrs, className)
		out.WriteString(value[last:r.Start])
		out.WriteString("<" + tag + newAttrs + ">")
		last = r.End
	}
	out.WriteString(value[last:])
	return out.String(), warnings
}

func containsToken(tokens []string, want string) bool {
	for _, t := range tokens {
		if t == want {
			return true
		}
	}
	return false
}

// visibleText 对齐 visible_text：去注释/标签 → 实体解码 → 去全部空白。
func visibleText(value string) string {
	stripped := tagRe.ReplaceAllString(value, "")
	stripped = unescapeEntities(stripped)
	var b strings.Builder
	for _, r := range stripped {
		if !isSpaceRune(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isSpaceRune 对齐 Python re \s（str 模式）：unicode 空白 + U+0085/U+00A0
// （unicode.IsSpace 已含）+ \x1c-\x1f 文件分隔符。
func isSpaceRune(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// unescapeEntities 是 html.unescape 的常用子集（任务允许的近似；
// 覆盖命名实体 amp/lt/gt/quot/apos/nbsp 与数字/十六进制形式）。
func unescapeEntities(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '&' {
			b.WriteByte(s[i])
			i++
			continue
		}
		semi := strings.IndexByte(s[i:], ';')
		if semi < 0 || semi > 12 {
			b.WriteByte('&')
			i++
			continue
		}
		entity := s[i+1 : i+semi]
		switch entity {
		case "amp":
			b.WriteByte('&')
		case "lt":
			b.WriteByte('<')
		case "gt":
			b.WriteByte('>')
		case "quot":
			b.WriteByte('"')
		case "apos":
			b.WriteByte('\'')
		case "nbsp":
			b.WriteRune(0xA0)
		default:
			if strings.HasPrefix(entity, "#x") || strings.HasPrefix(entity, "#X") {
				if v, ok := parseHex(entity[2:]); ok {
					b.WriteRune(v)
				} else {
					b.WriteString(s[i : i+semi+1])
				}
			} else if strings.HasPrefix(entity, "#") {
				if v, ok := parseDec(entity[1:]); ok {
					b.WriteRune(v)
				} else {
					b.WriteString(s[i : i+semi+1])
				}
			} else {
				b.WriteString(s[i : i+semi+1])
			}
		}
		i += semi + 1
	}
	return b.String()
}

func parseHex(s string) (rune, bool) {
	v := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			v = v*16 + int(c-'0')
		case c >= 'a' && c <= 'f':
			v = v*16 + int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			v = v*16 + int(c-'A') + 10
		default:
			return 0, false
		}
		if v > 0x10FFFF {
			return 0, false
		}
	}
	return rune(v), v != 0
}

func parseDec(s string) (rune, bool) {
	v := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + int(c-'0')
		if v > 0x10FFFF {
			return 0, false
		}
	}
	return rune(v), v != 0
}

// titleText 对齐 title_text（识别结果已经过 visible_text）。
func titleText(value string) string {
	m := titleRe.FindStringSubmatch(value)
	if m == nil {
		return ""
	}
	return visibleText(m[1])
}

// posterImageHref 对齐 poster_image_href：封面页且 body 恰一张图。
func posterImageHref(value string) string {
	if titleText(value) != "封面" {
		return ""
	}
	body := bodyRe.FindStringSubmatch(value)
	if body == nil || visibleText(body[2]) != "" {
		return ""
	}
	images := imgRe.FindAllStringSubmatch(body[2], -1)
	if len(images) != 1 {
		return ""
	}
	src, ok := readQuotedAttr(images[0][1], "src", 1)
	if !ok {
		return ""
	}
	return src
}

// isCopyrightPage 对齐 is_copyright_page。
func isCopyrightPage(value string) bool {
	if titleText(value) != "版权信息" {
		return false
	}
	body := bodyRe.FindStringSubmatch(value)
	if body == nil {
		return false
	}
	return ulListRe.MatchString(body[2])
}

// prefixWarnings 给一批扫描截断告警统一加上文档路径前缀，方便调用方把
// 它们摊平进 rep.ScanWarnings 后仍能定位是哪个 XHTML。
func prefixWarnings(path string, warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	out := make([]string, len(warnings))
	for i, w := range warnings {
		out[i] = path + ": " + w
	}
	return out
}

// ensureStylesheetLink 复刻 epub_lib.ensure_stylesheet_link：幂等判据是
// href 子串已在文本中；命中的真实 </head> 结束标签被整个替换为
// link + "</head>"（原标签内的空白/大小写不保留，与原实现一致）。
//
// 原实现用 headEndRe 对整页文本跑正则，一段「展示旧写法」的 HTML 注释里
// 若原样写出 `</head>`，新链接会被插进注释里 —— 已修复：只在
// xhtml.ScanRegions 认定的真实 RegionTag（且是 head 的闭合标签）内定位。
//
// 第三个返回值是告警：扫描遇到无法闭合的结构而截断时，截断点之后即便
// 存在真实 </head> 也找不到，必须转成 finding，不能静默跳过插入。
func ensureStylesheetLink(text, href string) (string, bool, []string) {
	if strings.Contains(text, href) {
		return text, false, nil
	}
	regions, stop := xhtml.ScanRegions(text)
	var warnings []string
	if stop != xhtml.ScanComplete {
		warnings = append(warnings, fmt.Sprintf(
			"markup scan stopped at byte offset %d (unterminated comment/CDATA/PI/declaration/tag or unclosed style/script); stylesheet link insertion point search stopped early",
			stop))
	}
	for _, r := range regions {
		if r.Kind != xhtml.RegionTag {
			continue
		}
		name, _, closing := xhtml.TagParts(text[r.Start:r.End])
		if !closing || !strings.EqualFold(name, "head") {
			continue
		}
		link := `  <link href="` + href + `" type="text/css" rel="stylesheet"/>` + "\n"
		return text[:r.Start] + link + "</head>" + text[r.End:], true, warnings
	}
	return text, false, warnings
}

// hasClassToken 判断 content 内是否存在真实标签、其 class 属性含 want
// token。原判据 cardRe 是裸正则（`\bclass=["'][^"']*\bcopyright-card\b`，
// 不要求前导 `<`），版权页正文若原样写出 `class="copyright-card"`
// 这段示例文字会被误判为「结构已存在」，导致真正的 <section> 包裹被
// 跳过——漏做变换，text redline 抓不到（没有文本变化可比），只能靠人工
// 发现。改为只在 xhtml.ScanRegions 的真实 RegionTag（非闭合标签）属性内
// 查找。
//
// 第二个返回值是截断告警；截断后未扫到的区域保守视为「未找到」，交由
// 调用方按常规路径补齐包裹（顶多多包一层，不会丢正文/结构）。
func hasClassToken(content, want string) (bool, []string) {
	regions, stop := xhtml.ScanRegions(content)
	var warnings []string
	if stop != xhtml.ScanComplete {
		warnings = append(warnings, fmt.Sprintf(
			"markup scan stopped at byte offset %d (unterminated comment/CDATA/PI/declaration/tag or unclosed style/script); copyright-card detection stopped early",
			stop))
	}
	for _, r := range regions {
		if r.Kind != xhtml.RegionTag {
			continue
		}
		_, attrs, closing := xhtml.TagParts(content[r.Start:r.End])
		if closing {
			continue
		}
		if containsToken(classTokens(attrs), want) {
			return true, warnings
		}
	}
	return false, warnings
}

// refinePoster 对齐 refine_poster。BODY_RE.sub(count=1) 用区间拼接复刻。
func refinePoster(value string, volume int, imageHref, styleHref string) (string, []string, error) {
	loc := bodyRe.FindStringSubmatchIndex(value)
	if loc == nil {
		return "", nil, refinementErrf("poster page missing body")
	}
	attrs := value[loc[2]+len("<body") : loc[3]]
	attrs = strings.TrimSuffix(attrs, ">")
	attrs, _ = addClassToAttrs(attrs, "fullpage")
	attrs, _ = addClassToAttrs(attrs, "poster-bg")
	attrs, _ = addClassToAttrs(attrs, fmt.Sprintf("poster-bg-volume-%03d", volume))
	content := "\n  <section class=\"fullframe\" epub:type=\"chapter\">\n" +
		"    <img class=\"poster-fallback\" alt=\"\" src=\"" + imageHref + "\"/>\n" +
		"  </section>\n"
	updated := value[:loc[0]] + "<body" + attrs + ">" + content + "</body>" + value[loc[1]:]
	updated, _, warnings := ensureStylesheetLink(updated, styleHref)
	return updated, warnings, nil
}

// refineCopyright 对齐 refine_copyright。
func refineCopyright(value, styleHref string) (string, []string, error) {
	loc := bodyRe.FindStringSubmatchIndex(value)
	if loc == nil {
		return "", nil, refinementErrf("copyright page missing body")
	}
	attrs := value[loc[2]+len("<body") : loc[3]]
	attrs = strings.TrimSuffix(attrs, ">")
	attrs, _ = addClassToAttrs(attrs, "anthology-copyright-page")
	content := value[loc[4]:loc[5]]
	var warnings []string
	var w []string
	content, w = addClassToTag(content, "p", "copyright-heading", "cp")
	warnings = append(warnings, w...)
	content, w = addClassToTag(content, "ul", "copyright-meta", "list")
	warnings = append(warnings, w...)
	content, w = addClassToTag(content, "li", "copyright-meta-item", "i")
	warnings = append(warnings, w...)
	hasCard, w := hasClassToken(content, "copyright-card")
	warnings = append(warnings, w...)
	if !hasCard {
		content = "\n  <section class=\"copyright-card\" epub:type=\"frontmatter copyright-page\">" +
			content + "\n  </section>\n"
	}
	updated := value[:loc[0]] + "<body" + attrs + ">" + content + "</body>" + value[loc[1]:]
	updated, _, w = ensureStylesheetLink(updated, styleHref)
	warnings = append(warnings, w...)
	return updated, warnings, nil
}
