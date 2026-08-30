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
	// LegacyReport 输出 RefinementReport 形状。
	LegacyReport bool
	// Output 仅为 legacy 报告字段（本包不落盘，INV-3）。
	Output string
}

// legacyReport 对齐 RefinementReport.as_dict 的键序（harness 为首键）。
type legacyReport struct {
	Harness               string   `json:"harness"`
	Input                 string   `json:"input"`
	Output                string   `json:"output"`
	OPF                   string   `json:"opf"`
	PosterPagesRefined    int      `json:"poster_pages_refined"`
	CopyrightPagesRefined int      `json:"copyright_pages_refined"`
	StylesheetsAdded      int      `json:"stylesheets_added"`
	PosterPages           []string `json:"poster_pages"`
	CopyrightPages        []string `json:"copyright_pages"`
	Warnings              []string `json:"warnings"`
}

// Run 执行精排（SPEC §6.1 三段式）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	res := report.Result{Capability: CapabilityID, Status: report.StatusComplete}
	rep := legacyReport{
		Harness:        "epub_anthology_refinement",
		Input:          b.InputPath(),
		Output:         p.Output,
		PosterPages:    []string{},
		CopyrightPages: []string{},
		Warnings:       []string{},
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
		refined, err := refinePoster(decodeUTF8Replace(raw), vol, cand.imageHref, styleHref)
		if err != nil {
			return report.Result{}, err
		}
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
			cRefined, err := refineCopyright(decodeUTF8Replace(cRaw), copyrightStyleHref)
			if err != nil {
				return report.Result{}, err
			}
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
		"poster_pages_refined":    rep.PosterPagesRefined,
		"copyright_pages_refined": rep.CopyrightPagesRefined,
		"stylesheets_added":       rep.StylesheetsAdded,
		"warnings":                len(rep.Warnings),
	}
	for _, w := range rep.Warnings {
		res.Findings = append(res.Findings, report.Finding{
			Level: "warn", ID: "alite.no-copyright", Title: w,
		})
	}
	if p.LegacyReport {
		raw, err := report.MarshalLegacy(rep)
		if err != nil {
			return report.Result{}, err
		}
		res.Facts["legacyReport"] = rawMessage(raw)
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
func addClassToTag(value, tag, className, requiredClass string) string {
	lower := strings.ToLower(value)
	var out strings.Builder
	pos := 0
	open := "<" + tag
	for {
		i := strings.Index(lower[pos:], open)
		if i < 0 {
			out.WriteString(value[pos:])
			return out.String()
		}
		i += pos
		after := i + len(open)
		if after < len(value) && isPyWordByte(value[after]) {
			// Python \b：tag 名后必须紧跟非词字符。
			out.WriteString(value[pos : i+len(open)])
			pos = i + len(open)
			continue
		}
		gt := strings.IndexByte(value[i:], '>')
		if gt < 0 {
			out.WriteString(value[pos:])
			return out.String()
		}
		gt += i
		attrs := value[i+len(open) : gt]
		if requiredClass != "" && !containsToken(classTokens(attrs), requiredClass) {
			out.WriteString(value[pos : gt+1])
			pos = gt + 1
			continue
		}
		newAttrs, _ := addClassToAttrs(attrs, className)
		out.WriteString(value[pos:i])
		out.WriteString("<" + tag + newAttrs + ">")
		pos = gt + 1
	}
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

// ensureStylesheetLink 复刻 epub_lib.ensure_stylesheet_link：幂等判据是
// href 子串已在文本中；HEAD_END_RE 命中的整个结束标签被替换为
// link + "</head>"（原标签内的空白等不保留）。
func ensureStylesheetLink(text, href string) (string, bool) {
	if strings.Contains(text, href) {
		return text, false
	}
	link := `  <link href="` + href + `" type="text/css" rel="stylesheet"/>` + "\n"
	loc := headEndRe.FindStringIndex(text)
	if loc == nil {
		return text, false
	}
	return text[:loc[0]] + link + "</head>" + text[loc[1]:], true
}

// refinePoster 对齐 refine_poster。BODY_RE.sub(count=1) 用区间拼接复刻。
func refinePoster(value string, volume int, imageHref, styleHref string) (string, error) {
	loc := bodyRe.FindStringSubmatchIndex(value)
	if loc == nil {
		return "", refinementErrf("poster page missing body")
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
	updated, _ = ensureStylesheetLink(updated, styleHref)
	return updated, nil
}

// refineCopyright 对齐 refine_copyright。
func refineCopyright(value, styleHref string) (string, error) {
	loc := bodyRe.FindStringSubmatchIndex(value)
	if loc == nil {
		return "", refinementErrf("copyright page missing body")
	}
	attrs := value[loc[2]+len("<body") : loc[3]]
	attrs = strings.TrimSuffix(attrs, ">")
	attrs, _ = addClassToAttrs(attrs, "anthology-copyright-page")
	content := value[loc[4]:loc[5]]
	content = addClassToTag(content, "p", "copyright-heading", "cp")
	content = addClassToTag(content, "ul", "copyright-meta", "list")
	content = addClassToTag(content, "li", "copyright-meta-item", "i")
	if !cardRe.MatchString(content) {
		content = "\n  <section class=\"copyright-card\" epub:type=\"frontmatter copyright-page\">" +
			content + "\n  </section>\n"
	}
	updated := value[:loc[0]] + "<body" + attrs + ">" + content + "</body>" + value[loc[1]:]
	updated, _ = ensureStylesheetLink(updated, styleHref)
	return updated, nil
}
