// Package csscleanup 迁移 scripts/epub_css_cleanup.py
// （capability id：epub.css.layering.optimize）：
//
//   - sanitize_css：删除装饰分隔行、补缺失分号、把旧系统字体链
//     （cnepub/SimSun/SimHei/STKaiti）规范化为三条标准链；
//   - 同构抽取：stylesheet_shape 相同且 ≥3 个文件、≥2 个变体才合并为
//     clean-shared-NN.css，差异声明落 clean-override-<stem>.css；
//   - sha256（归一化文本）重复去重；
//   - XHTML link 重写（srcset→URI→url()→@import 之外的 link 版本：
//     仅 <link href="….css">）；
//   - --merge-scoped-local-css：把互不重叠的本地样式表合并进
//     clean-scoped-local.css（css-local-NN scope class + body 前缀规则）；
//   - manifest 增删与 OPF 字节区间编辑（INV-2：不整文档重序列化）。
//
// 字节保真策略：CSS / XHTML 用与 Python 完全相同的正则语义做字符串级
// 重写；OPF 相对 Python oracle 的 ET 整体重写是**更少字节改动**的预期
// 差异（SPEC §5.2 P3），parity 测试对 OPF 做语义比对。
package csscleanup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// CapabilityID 是契约 id（contracts/capabilities/v1/epub.css.layering.optimize.json）。
const CapabilityID = "epub.css.layering.optimize"

// ErrCleanup 对应 Python 的 CleanupError（errors.Is 可判）。
var ErrCleanup = errors.New("epub.css.layering.optimize: the EPUB CSS cleanup cannot continue safely")

type cleanupError struct{ msg string }

func (e *cleanupError) Error() string   { return e.msg }
func (e *cleanupError) Is(t error) bool { return t == ErrCleanup }

func cleanupErrf(format string, a ...any) error {
	return &cleanupError{msg: fmt.Sprintf(format, a...)}
}

// Params 是 capability 参数。
type Params struct {
	// Output 是输出路径（仅写入 legacy 报告字段；本包不落盘，INV-3）。
	Output string
	// MergeScopedLocalCSS 对齐 Python --merge-scoped-local-css。
	MergeScopedLocalCSS bool
	// LegacyReport 为 true 时把 Python 形状的 JSON 报告放进
	// Result.Facts["legacyReport"]（json.RawMessage），供 parity gate P2。
	LegacyReport bool
}

// legacyCleanupReport 对齐 CleanupReport.as_dict（dataclass 字段序即 JSON 键序）。
type legacyCleanupReport struct {
	Harness                      string   `json:"harness"`
	Input                        string   `json:"input"`
	Output                       string   `json:"output"`
	OPF                          string   `json:"opf"`
	CSSFilesBefore               int      `json:"css_files_before"`
	CSSFilesAfter                int      `json:"css_files_after"`
	FactoredStylesheets          int      `json:"factored_stylesheets"`
	DuplicateStylesheetsRemoved  int      `json:"duplicate_stylesheets_removed"`
	OverridesCreated             int      `json:"overrides_created"`
	FontDeclarationsRewritten    int      `json:"font_declarations_rewritten"`
	XHTMLFilesUpdated            int      `json:"xhtml_files_updated"`
	CSSManifestItemsRemoved      int      `json:"css_manifest_items_removed"`
	CSSManifestItemsAdded        int      `json:"css_manifest_items_added"`
	ScopedLocalStylesheetsMerged int      `json:"scoped_local_stylesheets_merged"`
	ScopeClassesAdded            int      `json:"scope_classes_added"`
	Warnings                     []string `json:"warnings"`
}

// ---- Run（SPEC §6.1 三段式：扫描 → 应用 → 报告） ----

// Run 执行本 capability。禁止修改 b 之外的任何状态；落盘由 pipeline 的
// b.WriteTo 负责（INV-3）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	names := b.Names()
	m := newFileModel(b, names)

	// 1. 扫描（只读）：解析容器与 OPF，产出全部编辑。
	opfPath, err := opfPathFromContainer(m)
	if err != nil {
		return report.Result{}, err
	}
	opfData, err := m.raw(opfPath)
	if err != nil {
		return report.Result{}, cleanupErrf("%v", err)
	}
	opfRoot, err := opf.ScanSpanTree(opfData)
	if err != nil {
		return report.Result{}, cleanupErrf("%s: XML parse failed: %v", opfPath, err)
	}
	opfDir := pyDirname(opfPath)

	rep := legacyCleanupReport{
		Harness:  "epub_css_cleanup",
		Input:    b.InputPath(),
		Output:   p.Output,
		OPF:      opfPath,
		Warnings: []string{},
	}

	// CSS manifest 条目（norm_join(opf_dir, href) → item）。
	cssItems := map[string]*opf.SpanNode{}
	for _, item := range opfManifestItems(opfRoot) {
		mediaType, _ := nodeAttr(item, "media-type")
		href, hasHref := nodeAttr(item, "href")
		if mediaType != "text/css" || !hasHref || href == "" {
			continue
		}
		cssItems[normJoin(opfDir, href)] = item
	}
	rep.CSSFilesBefore = len(cssItems)

	cssText := map[string]string{}
	parsed := map[string][]cssRule{}
	cssPaths := sortedKeys(cssItems)
	for _, cssPath := range cssPaths {
		if !m.has(cssPath) {
			rep.Warnings = append(rep.Warnings, "CSS manifest item does not resolve: "+cssPath)
			continue
		}
		data, err := m.raw(cssPath)
		if err != nil {
			return report.Result{}, cleanupErrf("%v", err)
		}
		cleaned, rewrites := sanitizeCSS(decodeUTF8Replace(data))
		cssText[cssPath] = cleaned
		rep.FontDeclarationsRewritten += rewrites
		if rules, ok := parseStylesheet(cleaned); ok {
			parsed[cssPath] = rules
		}
	}

	mapping := map[string][]string{}
	removed := map[string]bool{}
	generated := map[string][]byte{}

	// 同构抽取（shape 相同 + ≥3 文件 + ≥2 变体）。
	factorShapes(m, cssPaths, cssText, parsed, mapping, removed, generated, &rep)

	// sha256（归一化文本）重复去重。
	digestOwner := map[string]string{}
	for _, cssPath := range cssPaths {
		text, ok := cssText[cssPath]
		if !ok || removed[cssPath] {
			continue
		}
		digest := sha256Text(text)
		canonical, seen := digestOwner[digest]
		if !seen {
			digestOwner[digest] = cssPath
			continue
		}
		mapping[cssPath] = []string{canonical}
		removed[cssPath] = true
		rep.DuplicateStylesheetsRemoved++
	}

	// files 模型更新（对齐 Python 的 files dict 操作）。
	for _, cssPath := range cssPaths {
		if value, ok := cssText[cssPath]; ok && !removed[cssPath] {
			m.set(cssPath, []byte(value))
		}
	}
	for _, cssPath := range sortedKeys(removed) {
		m.drop(cssPath)
	}
	for _, path := range sortedKeys(generated) {
		m.set(path, generated[path])
	}

	// XHTML link 重写。
	xhtmlPaths := xhtmlZipPaths(opfRoot, opfDir)
	for _, xhtmlPath := range xhtmlPaths {
		data, ok := m.get(xhtmlPath)
		if !ok {
			continue
		}
		text, changed := rewriteCSSLinks(decodeUTF8Replace(data), xhtmlPath, mapping)
		if changed {
			m.set(xhtmlPath, []byte(text))
			rep.XHTMLFilesUpdated++
		}
	}

	// scoped-local 合并。
	if p.MergeScopedLocalCSS {
		consolidateScopedLocalCSS(m, xhtmlPaths, opfDir, removed, generated, &rep)
	}

	// css_files_after：最终 files 里以 .css 结尾（大小写不敏感）的数量。
	for name := range m.exists {
		if strings.HasSuffix(strings.ToLower(name), ".css") {
			rep.CSSFilesAfter++
		}
	}

	// 2. 应用（唯一写点）：OPF 字节区间编辑 + entry 增删改。
	opfEdits, err := opfEditsFor(opfPath, opfData, opfRoot, opfDir, removed, generated, &rep)
	if err != nil {
		return report.Result{}, err
	}
	edits, err := m.edits(opfPath, opfEdits)
	if err != nil {
		return report.Result{}, err
	}
	if err := b.Apply(edits); err != nil {
		return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
	}

	// 3. 报告（不落盘）。
	return buildResult(p, rep, mapping), nil
}

// buildResult 装配统一信封的 Result 段（含 legacy-report 脚手架）。
func buildResult(p Params, rep legacyCleanupReport, mapping map[string][]string) report.Result {
	facts := map[string]any{
		"opf":                         rep.OPF,
		"cssFilesBefore":              rep.CSSFilesBefore,
		"cssFilesAfter":               rep.CSSFilesAfter,
		"factoredStylesheets":         rep.FactoredStylesheets,
		"duplicateStylesheetsRemoved": rep.DuplicateStylesheetsRemoved,
		"overridesCreated":            rep.OverridesCreated,
		"fontDeclarationsRewritten":   rep.FontDeclarationsRewritten,
		"xhtmlFilesUpdated":           rep.XHTMLFilesUpdated,
		"cssManifestItemsRemoved":     rep.CSSManifestItemsRemoved,
		"cssManifestItemsAdded":       rep.CSSManifestItemsAdded,
		"scopedLocalStylesheetsMerged": rep.ScopedLocalStylesheetsMerged,
		"scopeClassesAdded":           rep.ScopeClassesAdded,
		"warnings":                    rep.Warnings,
		"mergeScopedLocalCss":         p.MergeScopedLocalCSS,
	}
	findings := make([]report.Finding, 0, len(rep.Warnings))
	for _, w := range rep.Warnings {
		findings = append(findings, report.Finding{Level: "warn", ID: "css_cleanup.warning", Title: w})
	}
	if len(findings) == 0 {
		findings = nil
	}
	events := []report.Event{{
		Step: "css-cleanup", Status: "completed",
		Message: fmt.Sprintf("css %d -> %d factored=%d duplicates=%d scoped_merged=%d",
			rep.CSSFilesBefore, rep.CSSFilesAfter, rep.FactoredStylesheets,
			rep.DuplicateStylesheetsRemoved, rep.ScopedLocalStylesheetsMerged),
	}}

	// Renames：被移除样式表 → 其首个替代者（redline path map 用）。
	var renames map[string]string
	for from, targets := range mapping {
		if len(targets) == 0 || targets[0] == from {
			continue
		}
		if renames == nil {
			renames = map[string]string{}
		}
		renames[from] = targets[0]
	}

	if p.LegacyReport {
		raw, err := report.MarshalLegacy(rep)
		if err != nil {
			return report.Result{Capability: CapabilityID, Status: report.StatusFailed}
		}
		// 存 json.RawMessage，避免 []byte 被信封编码成 base64。
		facts["legacyReport"] = jsonRawMessage(raw)
	}

	return report.Result{
		Capability: CapabilityID,
		Status:     report.StatusComplete,
		Facts:      facts,
		Findings:   findings,
		Events:     events,
		Renames:    renames,
	}
}

// ---- sanitize_css ----

// systemFontFamily 复刻 system_font_family：压缩空白并小写后查表。
func systemFontFamily(value string) string {
	compact := removeAllSpace(strings.ToLower(value))
	switch compact {
	case `"cnepub",serif`, `"simsun"`:
		return songChain
	case `"simhei"`:
		return heiChain
	case `"stkaiti"`:
		return kaiChain
	}
	return ""
}

// sanitizeCSS 逐行复刻 sanitize_css：删装饰行 → 补缺失分号 → 字体链
// 规范化 → strip + 结尾补一个换行。
func sanitizeCSS(value string) (string, int) {
	value = ornamentRe.ReplaceAllString(value, "")
	value = fixMissingSemis(value)
	rewrites := 0
	var b strings.Builder
	last := 0
	for _, d := range css.FontFamilyDecls(value) {
		replacement := systemFontFamily(d.Value)
		if replacement == "" {
			continue
		}
		rewrites++
		b.WriteString(value[last:d.ValueSpan.Start])
		b.WriteString(replacement)
		last = d.ValueSpan.End
	}
	b.WriteString(value[last:])
	value = b.String()
	return pyStrip(value) + "\n", rewrites
}

// fixMissingSemis 复刻 MISSING_SEMICOLON_RE 的 re.sub
// （(?m)(^\s*[-\w]+\s*:\s*[^;{}\n]+)\n(?=\s*[-\w]+\s*:) → \1;\n）。
// RE2 无前瞻，按行扫描手工实现：行首匹配「属性: 值」且该行以换行结尾、
// 换行之后（允许跨行空白）出现下一个「属性:」时补分号。
func fixMissingSemis(text string) string {
	var b strings.Builder
	last := 0
	lineStart := 0
	appendMatch := func(end int) {
		b.WriteString(text[last:end])
		last = end
	}
	for lineStart < len(text) {
		if mEnd, ok := matchMissingSemiAt(text, lineStart); ok {
			// 命中：group(1) 保持原样 + ";" + "\n"。
			b.WriteString(text[last:mEnd-1])
			b.WriteString(";\n")
			last = mEnd
			lineStart = mEnd
			continue
		}
		// 推进到下一行行首。
		nl := strings.IndexByte(text[lineStart:], '\n')
		if nl < 0 {
			lineStart = len(text)
			break
		}
		lineStart += nl + 1
	}
	appendMatch(len(text))
	return b.String()
}

// matchMissingSemiAt 在行首 lineStart 尝试匹配
// ^\s*[-\w]+\s*:\s*[^;{}\n]+\n(?=\s*[-\w]+\s*:)，返回完整匹配终点。
func matchMissingSemiAt(text string, lineStart int) (int, bool) {
	i := lineStart
	for i < len(text) {
		r, size := utf8DecodeRune(text[i:])
		if r == '\n' || !isSpaceRune(r) {
			break
		}
		i += size
	}
	nameStart := i
	for i < len(text) {
		r, size := utf8DecodeRune(text[i:])
		if r == '\n' || !(isWordRune(r) || r == '-') {
			break
		}
		i += size
	}
	if i == nameStart {
		return 0, false
	}
	for i < len(text) {
		r, size := utf8DecodeRune(text[i:])
		if r == '\n' || !isSpaceRune(r) {
			break
		}
		i += size
	}
	if i >= len(text) || text[i] != ':' {
		return 0, false
	}
	i++
	for i < len(text) {
		r, size := utf8DecodeRune(text[i:])
		if r == '\n' || !isSpaceRune(r) {
			break
		}
		i += size
	}
	valStart := i
	for i < len(text) {
		r, _ := utf8DecodeRune(text[i:])
		if r == '\n' || r == ';' || r == '{' || r == '}' {
			break
		}
		i++
	}
	if i == valStart || i >= len(text) || text[i] != '\n' {
		return 0, false
	}
	if !lookaheadNextDecl(text, i+1) {
		return 0, false
	}
	return i + 1, true
}

// lookaheadNextDecl 复刻前瞻 (?=\s*[-\w]+\s*:)：允许跨行空白。
func lookaheadNextDecl(text string, pos int) bool {
	i := skipPySpace(text, pos)
	start := i
	for i < len(text) {
		r, size := utf8DecodeRune(text[i:])
		if r == '\n' || !(isWordRune(r) || r == '-') {
			break
		}
		i += size
	}
	if i == start {
		return false
	}
	i = skipPySpace(text, i)
	return i < len(text) && text[i] == ':'
}

func utf8DecodeRune(s string) (rune, int) {
	for _, r := range s {
		return r, len(string(r))
	}
	return 0, 0
}

// ---- parse_stylesheet / shape ----

// cssRule 对齐 Python 的 Rule（selector + (name, value) 声明二元组）。
type cssRule struct {
	selector     string
	declarations [][2]string
}

// parseStylesheet 复刻 parse_stylesheet：注释剥离后按 RULE_RE 迭代；
// 出现 @ 规则、空选择器或无冒号声明时整体判为不可解析（返回 false）。
func parseStylesheet(value string) ([]cssRule, bool) {
	stripped := css.StripComments(value)
	var rules []cssRule
	for _, r := range css.Rules(stripped) {
		selector := normalizeSpace(r.Selector)
		if selector == "" || strings.HasPrefix(selector, "@") {
			return nil, false
		}
		var decls [][2]string
		for _, raw := range strings.Split(r.Body, ";") {
			raw = pyStrip(raw)
			if raw == "" {
				continue
			}
			i := strings.IndexByte(raw, ':')
			if i < 0 {
				return nil, false
			}
			decls = append(decls, [2]string{pyStrip(raw[:i]), normalizeSpace(raw[i+1:])})
		}
		rules = append(rules, cssRule{selector: selector, declarations: decls})
	}
	if len(rules) == 0 {
		return nil, false
	}
	return rules, true
}

// stylesheetShape 复刻 stylesheet_shape：(selector, 小写声明名序列) 元组。
// 返回可哈希的签名字符串。
func stylesheetShape(rules []cssRule) string {
	var b strings.Builder
	for _, rule := range rules {
		b.WriteString(rule.selector)
		b.WriteByte(0)
		for _, d := range rule.declarations {
			b.WriteString(strings.ToLower(d[0]))
			b.WriteByte(0)
		}
		b.WriteByte(1)
	}
	return b.String()
}

func declsEqual(a, b [][2]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// isCleanupGeneratedCSS 复刻 is_cleanup_generated_css。
func isCleanupGeneratedCSS(cssPath string) bool {
	name := pyBasename(cssPath)
	return strings.HasPrefix(name, "clean-shared-") ||
		strings.HasPrefix(name, "clean-override-") ||
		strings.HasPrefix(name, "clean-scoped-local")
}

// formatRules 复刻 format_rules（固定两空格缩进格式）。
func formatRules(rules []cssRule) []byte {
	var chunks []string
	for _, rule := range rules {
		chunks = append(chunks, rule.selector+" {")
		for _, d := range rule.declarations {
			chunks = append(chunks, "  "+d[0]+": "+d[1]+";")
		}
		chunks = append(chunks, "}", "")
	}
	return []byte(pyRStrip(strings.Join(chunks, "\n")) + "\n")
}

// uniqueZipPath 复刻 unique_zip_path（stem-2、stem-3 … 探测）。
func uniqueZipPath(has func(string) bool, base string) string {
	stem, ext := pySplitExt(base)
	candidate := base
	index := 2
	for has(candidate) {
		candidate = fmt.Sprintf("%s-%d%s", stem, index, ext)
		index++
	}
	return candidate
}

// sha256Text 复刻 sha256_text：sha256(normalize_css(text))。
func sha256Text(value string) string {
	normalized := strings.ToLower(removeAllSpace(css.StripComments(value)))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// factorShapes 复刻同构抽取主循环。
func factorShapes(m *fileModel, cssPaths []string, cssText map[string]string, parsed map[string][]cssRule,
	mapping map[string][]string, removed map[string]bool, generated map[string][]byte, rep *legacyCleanupReport) {

	groups := map[string][]string{}
	for _, cssPath := range cssPaths {
		rules, ok := parsed[cssPath]
		if !ok || isCleanupGeneratedCSS(cssPath) {
			continue
		}
		sig := stylesheetShape(rules)
		groups[sig] = append(groups[sig], cssPath)
	}
	var groupLists [][]string
	for _, paths := range groups {
		groupLists = append(groupLists, paths)
	}
	sort.Slice(groupLists, func(i, j int) bool { return groupLists[i][0] < groupLists[j][0] })

	sharedIndex := 1
	for _, paths := range groupLists {
		if len(paths) < 3 || distinctDigestCount(paths, cssText) < 2 {
			continue
		}
		sortedPaths := append([]string(nil), paths...)
		sort.Strings(sortedPaths)
		canonical := sortedPaths[0]
		cssDir := pyDirname(canonical)
		sharedPath := uniqueZipPath(m.unionHas, pyJoinPath(cssDir, fmt.Sprintf("clean-shared-%02d.css", sharedIndex)))
		sharedIndex++
		canonicalRules := parsed[canonical]
		generated[sharedPath] = formatRules(canonicalRules)
		for _, cssPath := range sortedPaths {
			rules := parsed[cssPath]
			var changed []cssRule
			for i, rule := range rules {
				if i < len(canonicalRules) && !declsEqual(rule.declarations, canonicalRules[i].declarations) {
					changed = append(changed, rule)
				}
			}
			replacement := []string{sharedPath}
			if len(changed) > 0 {
				overrideBase := pyJoinPath(cssDir, "clean-override-"+pyPathStem(cssPath)+".css")
				overridePath := uniqueZipPath(m.unionHas, overrideBase)
				generated[overridePath] = formatRules(changed)
				replacement = append(replacement, overridePath)
				rep.OverridesCreated++
			}
			mapping[cssPath] = replacement
			removed[cssPath] = true
		}
		rep.FactoredStylesheets += len(sortedPaths)
	}
}

func distinctDigestCount(paths []string, cssText map[string]string) int {
	seen := map[string]bool{}
	for _, p := range paths {
		seen[sha256Text(cssText[p])] = true
	}
	return len(seen)
}

// ---- XHTML link 重写（LINK_RE 的手工实现，含引号反向引用） ----

type linkMatch struct {
	start, end int
	href       string
}

// findLinkMatch 复刻 LINK_RE
// `<link\b[^>]*\bhref=(["'])([^"']+\.css)(\1)[^>]*/?>`（re.I）的 finditer：
// [^>]* 贪心回溯取最右的 \bhref= 候选。
func findLinkMatch(text string, from int) (linkMatch, bool) {
	for i := from; i < len(text); {
		idx := indexFold(text, "<link", i)
		if idx < 0 {
			return linkMatch{}, false
		}
		nameEnd := idx + len("<link")
		// <link\b：link 后必须不是 \w 字符（或已到串尾）。
		if nameEnd < len(text) {
			r, size := utf8DecodeRune(text[nameEnd:])
			if isWordRune(r) {
				i = idx + size
				continue
			}
		}
		tagClose := strings.IndexByte(text[nameEnd:], '>')
		if tagClose < 0 {
			return linkMatch{}, false // [^>]* 无法越过 '>'，后面也不会再有完整标签
		}
		tagClose += nameEnd
		// 收集区间内的 \bhref= 候选（贪心 → 从右往左尝试）。
		var cands []int
		for p := indexFold(text, "href=", nameEnd); p >= 0 && p < tagClose; {
			if wordBoundaryAt(text, p) {
				cands = append(cands, p)
			}
			next := indexFold(text, "href=", p+len("href="))
			if next < 0 || next >= tagClose {
				break
			}
			p = next
		}
		for k := len(cands) - 1; k >= 0; k-- {
			p := cands[k]
			vp := p + len("href=")
			if vp >= len(text) {
				continue
			}
			q := text[vp]
			if q != '"' && q != '\'' {
				continue
			}
			ve := strings.IndexByte(text[vp+1:], q)
			if ve < 0 {
				continue
			}
			valStart := vp + 1
			valEnd := valStart + ve
			href := text[valStart:valEnd]
			if len(href) < 4 || !strings.EqualFold(href[len(href)-4:], ".css") {
				continue
			}
			return linkMatch{start: idx, end: tagClose + 1, href: href}, true
		}
		i = idx + 1
	}
	return linkMatch{}, false
}

// rewriteCSSLinks 复刻 rewrite_css_links。
func rewriteCSSLinks(text, xhtmlPath string, mapping map[string][]string) (string, bool) {
	changed := false
	var b strings.Builder
	last := 0
	dir := pyDirname(xhtmlPath)
	for pos := 0; ; {
		m, ok := findLinkMatch(text, pos)
		if !ok {
			break
		}
		pos = m.end
		cssPath := pyNormPath(pyJoinPath(dir, m.href))
		targets, hit := mapping[cssPath]
		if !hit {
			continue
		}
		if !changed {
			changed = true
		}
		b.WriteString(text[last:m.start])
		b.WriteString(replacementLinks(text[m.start:m.end], xhtmlPath, targets))
		last = m.end
	}
	if !changed {
		return text, false
	}
	b.WriteString(text[last:])
	return b.String(), true
}

// linkedCSSPaths 复刻 linked_css_paths。
func linkedCSSPaths(text, xhtmlPath string) []string {
	var out []string
	dir := pyDirname(xhtmlPath)
	for pos := 0; ; {
		m, ok := findLinkMatch(text, pos)
		if !ok {
			break
		}
		out = append(out, pyNormPath(pyJoinPath(dir, m.href)))
		pos = m.end
	}
	return out
}

// replacementLinks 复刻 replacement_links：每个目标生成一个 link，按
// "\n" 连接。Python 侧的 indent 探测在「link 自身内部」搜索，恒得 ""
// （group(0) 以 '<' 开头，字面量不可能在其内部再现），故直接以 "\n" 连接。
func replacementLinks(link, xhtmlPath string, cssPaths []string) string {
	links := make([]string, 0, len(cssPaths))
	for _, cssPath := range cssPaths {
		href := relHref(xhtmlPath, cssPath)
		links = append(links, replaceFirstHref(link, href))
	}
	return strings.Join(links, "\n")
}

// replaceFirstHref 复刻 re.sub(r'href=(["\'])[^"\']+\1', f'href="{href}"', link, count=1)。
func replaceFirstHref(link, newHref string) string {
	for i := 0; ; {
		p := indexFold(link, "href=", i)
		if p < 0 {
			return link
		}
		vp := p + len("href=")
		if vp < len(link) {
			q := link[vp]
			if q == '"' || q == '\'' {
				ve := strings.IndexByte(link[vp+1:], q)
				// [^"']+ 至少一个字符。
				if ve > 0 {
					return link[:p] + `href="` + newHref + `"` + link[vp+1+ve+1:]
				}
			}
		}
		i = p + len("href=")
	}
}

// ---- scoped-local 合并 ----

// consolidateScopedLocalCSS 逐行复刻 consolidate_scoped_local_css。
func consolidateScopedLocalCSS(m *fileModel, xhtmlPaths []string, opfDir string,
	removed map[string]bool, generated map[string][]byte, rep *legacyCleanupReport) {

	refs := map[string]map[string]bool{}
	for _, xhtmlPath := range xhtmlPaths {
		data, ok := m.get(xhtmlPath)
		if !ok {
			continue
		}
		for _, cssPath := range linkedCSSPaths(decodeUTF8Replace(data), xhtmlPath) {
			if refs[cssPath] == nil {
				refs[cssPath] = map[string]bool{}
			}
			refs[cssPath][xhtmlPath] = true
		}
	}

	candidates := map[string][]cssRule{}
	for cssPath, pages := range refs {
		name := pyBasename(cssPath)
		if len(pages) == 0 || !m.has(cssPath) {
			continue
		}
		if scopedExcludedNames[name] || strings.HasPrefix(name, "clean-shared-") || len(pages)*2 > len(xhtmlPaths) {
			continue
		}
		data, _ := m.get(cssPath)
		if rules, ok := parseStylesheet(decodeUTF8Replace(data)); ok {
			candidates[cssPath] = rules
		}
	}

	overlapping := map[string]bool{}
	paths := sortedKeys(candidates)
	for i, cssPath := range paths {
		for _, other := range paths[i+1:] {
			if setsIntersect(refs[cssPath], refs[other]) {
				overlapping[cssPath] = true
				overlapping[other] = true
			}
		}
	}
	if len(overlapping) > 0 {
		rep.Warnings = append(rep.Warnings,
			"skipped overlapping local stylesheets: "+strings.Join(sortedKeys(overlapping), ", "))
	}

	var mergePaths []string
	for _, p := range paths {
		if !overlapping[p] {
			mergePaths = append(mergePaths, p)
		}
	}
	if len(mergePaths) == 0 {
		return
	}

	scopedPath := uniqueZipPath(m.unionHas, normJoin(opfDir, "Styles/clean-scoped-local.css"))
	var chunks []string
	scopeByPath := map[string]string{}
	for index, cssPath := range mergePaths {
		scopeClass := fmt.Sprintf("css-local-%02d", index+1)
		scopeByPath[cssPath] = scopeClass
		chunks = append(chunks, formatScopedRules(scopeClass, cssPath, candidates[cssPath])...)
	}
	scopedBytes := []byte(pyRStrip(strings.Join(chunks, "\n")) + "\n")
	generated[scopedPath] = scopedBytes
	m.set(scopedPath, scopedBytes)

	mapping := map[string][]string{}
	for _, cssPath := range mergePaths {
		mapping[cssPath] = []string{scopedPath}
	}
	affectedPages := map[string]bool{}
	for _, cssPath := range mergePaths {
		for page := range refs[cssPath] {
			affectedPages[page] = true
		}
	}
	for _, xhtmlPath := range sortedKeys(affectedPages) {
		data, _ := m.get(xhtmlPath)
		text := decodeUTF8Replace(data)
		for _, cssPath := range mergePaths {
			if refs[cssPath][xhtmlPath] {
				updated, added := addBodyClass(text, scopeByPath[cssPath])
				text = updated
				if added {
					rep.ScopeClassesAdded++
				}
			}
		}
		text, _ = rewriteCSSLinks(text, xhtmlPath, mapping)
		m.set(xhtmlPath, []byte(text))
	}

	for _, cssPath := range mergePaths {
		m.drop(cssPath)
		delete(generated, cssPath)
		removed[cssPath] = true
	}
	rep.ScopedLocalStylesheetsMerged += len(mergePaths)
}

// formatScopedRules 复刻 format_scoped_rules。
func formatScopedRules(scopeClass, cssPath string, rules []cssRule) []string {
	chunks := []string{"/* Scoped from " + cssPath + ". */"}
	for _, rule := range rules {
		chunks = append(chunks, scopedSelector(rule.selector, scopeClass)+" {")
		for _, d := range rule.declarations {
			chunks = append(chunks, "  "+d[0]+": "+d[1]+";")
		}
		chunks = append(chunks, "}", "")
	}
	return chunks
}

// scopedSelector 复刻 scoped_selector。
func scopedSelector(selector, scopeClass string) string {
	var scoped []string
	for _, part := range strings.Split(selector, ",") {
		part = pyStrip(part)
		if bodyPrefixLen(part) > 0 {
			// re.sub(r"^body", f"body.{scope}", part, count=1, flags=re.I)
			scoped = append(scoped, "body."+scopeClass+part[bodyPrefixLen(part):])
		} else {
			scoped = append(scoped, "body."+scopeClass+" "+part)
		}
	}
	return strings.Join(scoped, ",\n")
}

// bodyPrefixLen 复刻 re.match(r"^body(?:\b|[.#:[ ])", part, re.I)：
// part 以 "body"（大小写不敏感）开头且后随非 \w 字符（`.#:[ ` 均非 \w，
// 故该字符类分支被 \b 覆盖）。命中返回 4，否则 0。
func bodyPrefixLen(part string) int {
	if len(part) < 4 || !strings.EqualFold(part[:4], "body") {
		return 0
	}
	if len(part) == 4 {
		return 4
	}
	r, _ := utf8DecodeRune(part[4:])
	if isWordRune(r) {
		return 0
	}
	return 4
}

// addBodyClass 复刻 add_body_class。
func addBodyClass(text, className string) (string, bool) {
	loc := bodyTagRe.FindStringSubmatchIndex(text)
	if loc == nil {
		return text, false
	}
	attrs := text[loc[2]:loc[3]]
	newAttrs, added := addClassToAttrs(attrs, className)
	if !added {
		return text, false
	}
	updated := text[:loc[0]] + "<body" + newAttrs + ">" + text[loc[1]:]
	return updated, updated != text
}

type classAttrMatch struct {
	start, end int
	classes    string
}

// findClassAttr 匹配 \bclass=(["'])([^"']*)(\1)（re.I，反向引用手工实现）。
func findClassAttr(text string, from int) (classAttrMatch, bool) {
	for i := from; i < len(text); {
		p := indexFold(text, "class=", i)
		if p < 0 {
			return classAttrMatch{}, false
		}
		i = p + len("class=")
		if !wordBoundaryAt(text, p) {
			continue
		}
		if i >= len(text) {
			return classAttrMatch{}, false
		}
		q := text[i]
		if q != '"' && q != '\'' {
			continue
		}
		ve := strings.IndexByte(text[i+1:], q)
		if ve < 0 {
			continue
		}
		return classAttrMatch{
			start:   p,
			end:     i + 1 + ve + 1,
			classes: text[i+1 : i+1+ve],
		}, true
	}
	return classAttrMatch{}, false
}

// classTokens 复刻 class_tokens（首个 class 属性的空白切分）。
func classTokens(attrs string) []string {
	m, ok := findClassAttr(attrs, 0)
	if !ok {
		return nil
	}
	return strings.Fields(m.classes)
}

// addClassToAttrs 复刻 add_class_to_attrs。
func addClassToAttrs(attrs, className string) (string, bool) {
	classes := classTokens(attrs)
	if contains(classes, className) {
		return attrs, false
	}
	classes = append(classes, className)
	joined := strings.Join(classes, " ")
	if m, ok := findClassAttr(attrs, 0); ok {
		return attrs[:m.start] + `class="` + joined + `"` + attrs[m.end:], true
	}
	return attrs + ` class="` + className + `"`, true
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func setsIntersect(a, b map[string]bool) bool {
	for k := range a {
		if b[k] {
			return true
		}
	}
	return false
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---- OPF 编辑 ----

// opfEditsFor 生成 manifest item 的删除与新增字节区间编辑。
func opfEditsFor(opfPath string, opfData []byte, opfRoot *opf.SpanNode, opfDir string,
	removed map[string]bool, generated map[string][]byte, rep *legacyCleanupReport) ([]editset.Edit, error) {

	var edits []editset.Edit
	removedItems := map[*opf.SpanNode]bool{}
	for _, item := range opfManifestItems(opfRoot) {
		mediaType, _ := nodeAttr(item, "media-type")
		href, hasHref := nodeAttr(item, "href")
		if mediaType != "text/css" || !hasHref || href == "" {
			continue
		}
		if removed[normJoin(opfDir, href)] {
			edits = append(edits, removeElementEdit(opfPath, opfData, item))
			removedItems[item] = true
			rep.CSSManifestItemsRemoved++
		}
	}

	if len(generated) == 0 {
		return edits, nil
	}
	manifestNode := firstOPFChild(opfRoot, "manifest")
	if manifestNode == nil {
		return nil, cleanupErrf("OPF missing manifest")
	}
	// Python 在移除之后读取当前 root 的 href/id 集合。
	hrefSeen := map[string]bool{}
	idSeen := map[string]bool{}
	for _, item := range opfManifestItems(opfRoot) {
		if removedItems[item] {
			continue
		}
		if h, ok := nodeAttr(item, "href"); ok {
			hrefSeen[h] = true
		}
		if id, ok := nodeAttr(item, "id"); ok {
			idSeen[id] = true
		}
	}
	var insert strings.Builder
	for _, cssPath := range sortedKeys(generated) {
		href := cssPath
		if opfDir != "" {
			href = pyRelPath(cssPath, opfDir)
		}
		if hrefSeen[href] {
			continue
		}
		hrefSeen[href] = true
		itemID := cssManifestItemID(href, idSeen)
		idSeen[itemID] = true
		insert.WriteString(opfItemElement(itemID, href))
		rep.CSSManifestItemsAdded++
	}
	if insert.Len() > 0 {
		edits = append(edits, editset.Insert(opfPath, int64(manifestNode.Close.Start), []byte(insert.String())))
	}
	return edits, nil
}

// cssManifestItemID 复刻 add_css_manifest_item 的 id 生成。
func cssManifestItemID(href string, idSeen map[string]bool) string {
	itemID := strings.Trim(idSanitizeRe.ReplaceAllString("css-"+pyPathStem(href), "-"), "-")
	base := itemID
	index := 2
	for idSeen[itemID] {
		itemID = fmt.Sprintf("%s-%d", base, index)
		index++
	}
	return itemID
}

// xhtmlZipPaths 复刻 xhtml_zip_paths（manifest 文档序，允许重复）。
func xhtmlZipPaths(opfRoot *opf.SpanNode, opfDir string) []string {
	var out []string
	for _, item := range opfManifestItems(opfRoot) {
		mediaType, _ := nodeAttr(item, "media-type")
		href, hasHref := nodeAttr(item, "href")
		if mediaType != "application/xhtml+xml" || !hasHref || href == "" {
			continue
		}
		out = append(out, normJoin(opfDir, href))
	}
	return out
}

// opfPathFromContainer 复刻 epub_lib.opf_path_from_container。
func opfPathFromContainer(m *fileModel) (string, error) {
	if !m.has(opf.ContainerPath) {
		return "", cleanupErrf("missing META-INF/container.xml")
	}
	data, err := m.raw(opf.ContainerPath)
	if err != nil {
		return "", cleanupErrf("%v", err)
	}
	root, err := opf.ScanSpanTree(data)
	if err != nil {
		return "", cleanupErrf("META-INF/container.xml: XML parse failed: %v", err)
	}
	opfPath := ""
	for _, e := range root.Walk() {
		if e.Name.Space == opf.ContainerURI && e.Name.Local == "rootfile" {
			opfPath, _ = e.AttrByLocal("", "full-path")
			break
		}
	}
	if opfPath == "" || !m.has(opfPath) {
		display := opfPath
		if display == "" {
			display = "<missing>"
		}
		return "", cleanupErrf("container rootfile does not resolve: %s", display)
	}
	return opfPath, nil
}
