// Package csscleanup 迁移 scripts/epub_css_cleanup.py
// （capability id：epub.css.layering.optimize）：
//
//   - sanitize_css：删除装饰分隔行、补缺失分号、把旧系统字体链
//     （cnepub/SimSun/SimHei/STKaiti）规范化为三条标准链；
//   - 高风险同构抽取已禁用：在 token/span 保真方案完成前不重建既有 CSS；
//   - 重复去重只接受同目录、逐字节完全相同的样式表；
//   - XHTML link 重写（srcset→URI→url()→@import 之外的 link 版本：
//     仅 <link href="….css">）；
//   - --merge-scoped-local-css 当前安全拒绝并报告 warning，不改 link/body；
//   - manifest 增删与 OPF 字节区间编辑（INV-2：不整文档重序列化）。
//
// 字节保真策略：CSS / XHTML 只生成不重叠的原始 byte-range edits；未知
// 或非法 CSS 返回安全错误，不经 UTF-8 replacement 或整 entry 序列化。
package csscleanup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

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
	// MergeScopedLocalCSS 保留 CLI 兼容；当前因 lossless 约束安全禁用。
	MergeScopedLocalCSS bool
	// LegacyReport 为 true 时把 Python 形状的 JSON 报告放进
	// Result.Facts["legacyReport"]（json.RawMessage），供 parity gate P2。
	LegacyReport bool
}

// legacyCleanupReport 对齐 CleanupReport.as_dict（dataclass 字段序即 JSON 键序）。
type legacyCleanupReport struct {
	Harness                      string `json:"harness"`
	Input                        string `json:"input"`
	Output                       string `json:"output"`
	OPF                          string `json:"opf"`
	CSSFilesBefore               int    `json:"css_files_before"`
	CSSFilesAfter                int    `json:"css_files_after"`
	FactoredStylesheets          int    `json:"factored_stylesheets"`
	DuplicateStylesheetsRemoved  int    `json:"duplicate_stylesheets_removed"`
	OverridesCreated             int    `json:"overrides_created"`
	FontDeclarationsRewritten    int    `json:"font_declarations_rewritten"`
	XHTMLFilesUpdated            int    `json:"xhtml_files_updated"`
	CSSManifestItemsRemoved      int    `json:"css_manifest_items_removed"`
	CSSManifestItemsAdded        int    `json:"css_manifest_items_added"`
	ScopedLocalStylesheetsMerged int    `json:"scoped_local_stylesheets_merged"`
	ScopeClassesAdded            int    `json:"scope_classes_added"`
	// 下列安全策略只进入统一 facts，不进入必须保持旧 Python 形状的
	// legacyReport JSON。
	SemanticFactoringDisabled bool     `json:"-"`
	ScopedMergeDisabled       bool     `json:"-"`
	DuplicateDeduplication    string   `json:"-"`
	Warnings                  []string `json:"warnings"`
}

const scopedMergeDisabledWarning = "MergeScopedLocalCSS requested but disabled for lossless safety; existing CSS entries, links, and body classes were left unchanged"

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

	cssBytes := map[string][]byte{}
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
		ornamentEdits, err := ornamentEditsFor(cssPath, data)
		if err != nil {
			return report.Result{}, cleanupErrf("%s: %v", cssPath, err)
		}
		if err := m.patch(cssPath, ornamentEdits); err != nil {
			return report.Result{}, cleanupErrf("%s: %v", cssPath, err)
		}
		cleaned, err := m.raw(cssPath)
		if err != nil {
			return report.Result{}, cleanupErrf("%v", err)
		}
		sheet, err := css.Parse(cleaned)
		if err != nil {
			return report.Result{}, cleanupErrf("%s: CSS parse failed: %v", cssPath, err)
		}
		missingEdits, err := missingSemicolonEdits(cssPath, cleaned, sheet)
		if err != nil {
			return report.Result{}, cleanupErrf("%s: %v", cssPath, err)
		}
		if err := m.patch(cssPath, missingEdits); err != nil {
			return report.Result{}, cleanupErrf("%s: %v", cssPath, err)
		}
		fixed, err := m.raw(cssPath)
		if err != nil {
			return report.Result{}, cleanupErrf("%v", err)
		}
		sheet, err = css.Parse(fixed)
		if err != nil {
			return report.Result{}, cleanupErrf("%s: CSS parse after semicolon repair failed: %v", cssPath, err)
		}
		fontEdits, rewrites, err := fontFamilyEdits(cssPath, fixed, sheet)
		if err != nil {
			return report.Result{}, cleanupErrf("%s: %v", cssPath, err)
		}
		if err := m.patch(cssPath, fontEdits); err != nil {
			return report.Result{}, cleanupErrf("%s: %v", cssPath, err)
		}
		finalData, err := m.raw(cssPath)
		if err != nil {
			return report.Result{}, cleanupErrf("%v", err)
		}
		cssBytes[cssPath] = bytes.Clone(finalData)
		rep.FontDeclarationsRewritten += rewrites
	}

	mapping := map[string][]string{}
	removed := map[string]bool{}
	generated := map[string][]byte{}

	// Semantic shape factoring and scoped merging are deliberately disabled.
	// Rebuilding a stylesheet from normalized selectors/declarations cannot be
	// proven lossless for CSS strings, custom properties, or case-sensitive
	// selectors. The only safe existing-entry deduplication is byte equality
	// after the explicitly authorized byte-range cleanup above.
	rep.SemanticFactoringDisabled = true
	rep.ScopedMergeDisabled = true
	rep.DuplicateDeduplication = "byte-exact"
	canonicalPaths := make([]string, 0, len(cssPaths))
	for _, cssPath := range cssPaths {
		data, ok := cssBytes[cssPath]
		if !ok || removed[cssPath] {
			continue
		}
		canonical := ""
		for _, candidate := range canonicalPaths {
			// CSS relative URLs and @import resolve against the stylesheet's
			// own directory. Byte equality across directories is therefore not
			// semantic equality and must not trigger link/path replacement.
			if pyDirname(candidate) == pyDirname(cssPath) && bytes.Equal(data, cssBytes[candidate]) {
				canonical = candidate
				break
			}
		}
		if canonical == "" {
			canonicalPaths = append(canonicalPaths, cssPath)
			continue
		}
		mapping[cssPath] = []string{canonical}
		removed[cssPath] = true
		rep.DuplicateStylesheetsRemoved++
	}

	// 既有 CSS entry 的清理已由 m.patch 保持原始坐标；这里只处理
	// entry 删除和新生成的完整 entry。
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
		edits, changed, err := rewriteCSSLinkEdits(xhtmlPath, data, mapping)
		if err != nil {
			return report.Result{}, cleanupErrf("%s: %v", xhtmlPath, err)
		}
		if changed {
			if err := m.patch(xhtmlPath, edits); err != nil {
				return report.Result{}, cleanupErrf("%s: %v", xhtmlPath, err)
			}
			rep.XHTMLFilesUpdated++
		}
	}

	// scoped-local 合并。
	if p.MergeScopedLocalCSS {
		rep.Warnings = append(rep.Warnings, scopedMergeDisabledWarning)
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
		"opf":                          rep.OPF,
		"cssFilesBefore":               rep.CSSFilesBefore,
		"cssFilesAfter":                rep.CSSFilesAfter,
		"factoredStylesheets":          rep.FactoredStylesheets,
		"duplicateStylesheetsRemoved":  rep.DuplicateStylesheetsRemoved,
		"overridesCreated":             rep.OverridesCreated,
		"fontDeclarationsRewritten":    rep.FontDeclarationsRewritten,
		"xhtmlFilesUpdated":            rep.XHTMLFilesUpdated,
		"cssManifestItemsRemoved":      rep.CSSManifestItemsRemoved,
		"cssManifestItemsAdded":        rep.CSSManifestItemsAdded,
		"scopedLocalStylesheetsMerged": rep.ScopedLocalStylesheetsMerged,
		"scopeClassesAdded":            rep.ScopeClassesAdded,
		"semanticFactoringDisabled":    rep.SemanticFactoringDisabled,
		"scopedMergeDisabled":          rep.ScopedMergeDisabled,
		"duplicateDeduplication":       rep.DuplicateDeduplication,
		"warnings":                     rep.Warnings,
		"mergeScopedLocalCss":          p.MergeScopedLocalCSS,
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

// ---- sanitize_css (lossless byte-range edits) ----

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

// sanitizeCSS retains the old test-facing convenience signature. The write
// path below uses the individual edit-producing helpers directly, so a parse
// error never results in a replacement string being written.
func sanitizeCSS(value string) (string, int) {
	data := []byte(value)
	edits, rewrites, err := sanitizeCSSData("<css>", data)
	if err != nil {
		return value, 0
	}
	cleaned, err := editset.Apply("<css>", data, edits)
	if err != nil {
		return value, 0
	}
	return string(cleaned), rewrites
}

func sanitizeCSSData(path string, data []byte) ([]editset.Edit, int, error) {
	ornament, err := ornamentEditsFor(path, data)
	if err != nil {
		return nil, 0, err
	}
	cleaned, err := editset.Apply(path, data, ornament)
	if err != nil {
		return nil, 0, err
	}
	sheet, err := css.Parse(cleaned)
	if err != nil {
		return nil, 0, err
	}
	missing, err := missingSemicolonEdits(path, cleaned, sheet)
	if err != nil {
		return nil, 0, err
	}
	fixed, err := editset.Apply(path, cleaned, missing)
	if err != nil {
		return nil, 0, err
	}
	sheet, err = css.Parse(fixed)
	if err != nil {
		return nil, 0, err
	}
	fontEdits, rewrites, err := fontFamilyEdits(path, fixed, sheet)
	if err != nil {
		return nil, 0, err
	}
	// Return edits in the original coordinate space. Each later pass was
	// rebased by composing the in-memory projection, just as fileModel.patch
	// does for a multi-stage cleanup.
	all := append([]editset.Edit(nil), ornament...)
	var missingOriginal []editset.Edit
	if len(missing) > 0 {
		// Translate the edits from the ornament projection to original offsets.
		missingOriginal, err = rebaseCSSStage(path, data, ornament, missing)
		if err != nil {
			return nil, 0, err
		}
		all = append(all, missingOriginal...)
	}
	if len(fontEdits) > 0 {
		// fontEdits are relative to the fixed projection. First map them
		// back through the missing-semicolon edits into the ornament
		// projection, then through ornament deletion into the original.
		translated, err := rebaseCSSStage(path, cleaned, missing, fontEdits)
		if err != nil {
			return nil, 0, err
		}
		translated, err = rebaseCSSStage(path, data, ornament, translated)
		if err != nil {
			return nil, 0, err
		}
		all = append(all, translated...)
	}
	return all, rewrites, editset.Validate(all)
}

// rebaseCSSStage is a small local adapter around fileModel's coordinate
// composition. It applies old edits to a synthetic path and maps new edits by
// searching the unchanged source boundaries. The cleanup Run path uses
// fileModel.patch; this helper only serves the compatibility sanitizeCSS API.
func rebaseCSSStage(path string, base []byte, old, next []editset.Edit) ([]editset.Edit, error) {
	if len(old) == 0 {
		return append([]editset.Edit(nil), next...), nil
	}
	projected, err := editset.Apply(path, base, old)
	if err != nil {
		return nil, err
	}
	var out []editset.Edit
	for _, e := range next {
		offset, length, err := rebaseRange(e.Offset, e.Length, old, len(projected))
		if err != nil {
			return nil, err
		}
		out = append(out, editset.Replace(path, offset, length, e.Replacement))
	}
	return out, nil
}

func ornamentEditsFor(path string, data []byte) ([]editset.Edit, error) {
	if !utf8.Valid(data) {
		return nil, css.ErrInvalidUTF8
	}
	var edits []editset.Edit
	comment, quote, url := false, byte(0), false
	for lineStart := 0; lineStart < len(data); {
		lineEnd := lineStart
		for lineEnd < len(data) && data[lineEnd] != '\n' {
			lineEnd++
		}
		contentEnd := lineEnd
		if contentEnd > lineStart && data[contentEnd-1] == '\r' {
			contentEnd--
		}
		protected := comment || quote != 0 || url
		for i := lineStart; i < contentEnd; i++ {
			if comment {
				if i+1 < contentEnd && data[i] == '*' && data[i+1] == '/' {
					comment = false
					i++
				}
				continue
			}
			if quote != 0 {
				if data[i] == '\\' && i+1 < contentEnd {
					i++
					continue
				}
				if data[i] == quote {
					quote = 0
				}
				continue
			}
			if url {
				if data[i] == '\\' && i+1 < contentEnd {
					i++
					continue
				}
				if data[i] == ')' {
					url = false
				}
				continue
			}
			if i+1 < contentEnd && data[i] == '/' && data[i+1] == '*' {
				comment = true
				protected = true
				i++
				continue
			}
			if data[i] == '\'' || data[i] == '"' {
				quote = data[i]
				protected = true
				continue
			}
			if hasURLPrefix(data, i) {
				url = true
				protected = true
			}
		}
		if !protected && ornamentLine(data[lineStart:contentEnd]) {
			edits = append(edits, editset.Replace(path, int64(lineStart), int64(contentEnd-lineStart), []byte{}))
		}
		if lineEnd == len(data) {
			break
		}
		lineStart = lineEnd + 1
	}
	return edits, editset.Validate(edits)
}

func ornamentLine(line []byte) bool {
	trimmed := strings.TrimSpace(string(line))
	if !strings.Contains(trimmed, "标题") {
		return false
	}
	return dashRunPrefix(trimmed) >= 3 && dashRunSuffix(trimmed) >= 3
}

func dashRunPrefix(s string) int {
	n := 0
	for len(s) > 0 {
		if strings.HasPrefix(s, "—") {
			n++
			s = s[len("—"):]
			continue
		}
		if s[0] == '-' {
			n++
			s = s[1:]
			continue
		}
		break
	}
	return n
}

func dashRunSuffix(s string) int {
	n := 0
	for len(s) > 0 {
		if strings.HasSuffix(s, "—") {
			n++
			s = s[:len(s)-len("—")]
			continue
		}
		if s[len(s)-1] == '-' {
			n++
			s = s[:len(s)-1]
			continue
		}
		break
	}
	return n
}

func hasURLPrefix(data []byte, pos int) bool {
	if pos+3 > len(data) || !strings.EqualFold(string(data[pos:pos+3]), "url") {
		return false
	}
	if pos > 0 && (isWordASCII(data[pos-1]) || data[pos-1] == '-') {
		return false
	}
	i := pos + 3
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\r' || data[i] == '\n' || data[i] == '\f') {
		i++
	}
	return i < len(data) && data[i] == '('
}

func missingSemicolonEdits(path string, data []byte, sheet *css.Stylesheet) ([]editset.Edit, error) {
	var edits []editset.Edit
	for _, rule := range sheet.Rules {
		if rule.AtRule && len(rule.Declarations) == 0 {
			continue
		}
		span := rule.BodySpan
		if span.Start >= span.End {
			continue
		}
		for _, at := range missingSemicolonsInSpan(data, span) {
			edits = append(edits, editset.Insert(path, int64(at), []byte(";")))
		}
	}
	return edits, editset.Validate(edits)
}

func missingSemicolonsInSpan(data []byte, span css.Span) []int {
	var inserts []int
	lineStart := span.Start
	for lineStart < span.End {
		lineEnd := lineStart
		for lineEnd < span.End && data[lineEnd] != '\n' {
			lineEnd++
		}
		contentEnd := lineEnd
		if contentEnd > lineStart && data[contentEnd-1] == '\r' {
			contentEnd--
		}
		if simpleDeclarationLine(data, lineStart, contentEnd) &&
			!topLevelSemicolon(data, lineStart, contentEnd) {
			nextStart := lineEnd
			if nextStart < span.End && data[nextStart] == '\n' {
				nextStart++
			}
			if nextStart < span.End {
				nextEnd := nextStart
				for nextEnd < span.End && data[nextEnd] != '\n' {
					nextEnd++
				}
				nextContentEnd := nextEnd
				if nextContentEnd > nextStart && data[nextContentEnd-1] == '\r' {
					nextContentEnd--
				}
				if simpleDeclarationLine(data, nextStart, nextContentEnd) {
					inserts = append(inserts, contentEnd)
				}
			}
		}
		if lineEnd == span.End {
			break
		}
		lineStart = lineEnd + 1
	}
	return inserts
}

func simpleDeclarationLine(data []byte, start, end int) bool {
	trimmed := trimCSSByteSpan(data, start, end)
	if trimmed.Start == trimmed.End {
		return false
	}
	colon := -1
	paren, bracket, brace := 0, 0, 0
	for i := trimmed.Start; i < trimmed.End; {
		if next, ok := skipCSSOpaque(data, i, trimmed.End); ok {
			i = next
			continue
		}
		switch data[i] {
		case '(':
			paren++
		case ')':
			if paren == 0 {
				return false
			}
			paren--
		case '[':
			bracket++
		case ']':
			if bracket == 0 {
				return false
			}
			bracket--
		case '{':
			brace++
		case '}':
			if brace == 0 {
				return false
			}
			brace--
		case ';':
			if paren == 0 && bracket == 0 && brace == 0 {
				// A terminal semicolon is accepted as a candidate for the
				// look-ahead line. The caller separately checks whether the
				// current line itself needs a repair.
				if trimCSSByteSpan(data, i+1, trimmed.End).Start != trimCSSByteSpan(data, i+1, trimmed.End).End {
					return false
				}
			}
		case ':':
			if paren == 0 && bracket == 0 && brace == 0 && colon < 0 {
				colon = i
			}
		}
		i++
	}
	if colon < 0 || paren != 0 || bracket != 0 || brace != 0 {
		return false
	}
	name := trimCSSByteSpan(data, trimmed.Start, colon)
	value := trimCSSByteSpan(data, colon+1, trimmed.End)
	if name.Start == name.End || value.Start == value.End {
		return false
	}
	for i := name.Start; i < name.End; i++ {
		if !isPropertyNameByte(data[i]) {
			return false
		}
	}
	return true
}

func topLevelSemicolon(data []byte, start, end int) bool {
	trimmed := trimCSSByteSpan(data, start, end)
	paren, bracket, brace := 0, 0, 0
	for i := trimmed.Start; i < trimmed.End; {
		if next, ok := skipCSSOpaque(data, i, trimmed.End); ok {
			i = next
			continue
		}
		switch data[i] {
		case '(':
			paren++
		case ')':
			if paren > 0 {
				paren--
			}
		case '[':
			bracket++
		case ']':
			if bracket > 0 {
				bracket--
			}
		case '{':
			brace++
		case '}':
			if brace > 0 {
				brace--
			}
		case ';':
			if paren == 0 && bracket == 0 && brace == 0 {
				return true
			}
		}
		i++
	}
	return false
}

func fontFamilyEdits(path string, data []byte, sheet *css.Stylesheet) ([]editset.Edit, int, error) {
	var edits []editset.Edit
	for _, decl := range sheet.Declarations {
		if !strings.EqualFold(strings.TrimSpace(decl.Name), "font-family") {
			continue
		}
		if decl.ValueSpan.Start < 0 || decl.ValueSpan.End > len(data) || decl.ValueSpan.Start > decl.ValueSpan.End {
			return nil, 0, fmt.Errorf("font-family span out of bounds: %+v", decl.ValueSpan)
		}
		replacement := systemFontFamily(string(data[decl.ValueSpan.Start:decl.ValueSpan.End]))
		if replacement == "" || replacement == string(data[decl.ValueSpan.Start:decl.ValueSpan.End]) {
			continue
		}
		edits = append(edits, editset.Replace(path, int64(decl.ValueSpan.Start), int64(decl.ValueSpan.Len()), []byte(replacement)))
	}
	return edits, len(edits), editset.Validate(edits)
}

func trimCSSByteSpan(data []byte, start, end int) css.Span {
	for start < end && isCSSSpaceByte(data[start]) {
		start++
	}
	for end > start && isCSSSpaceByte(data[end-1]) {
		end--
	}
	return css.Span{Start: start, End: end}
}

func isCSSSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

func isPropertyNameByte(b byte) bool {
	return b == '-' || b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

func isWordASCII(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

func skipCSSOpaque(data []byte, pos, end int) (int, bool) {
	if pos+1 < end && data[pos] == '/' && data[pos+1] == '*' {
		for i := pos + 2; i+1 < end; i++ {
			if data[i] == '*' && data[i+1] == '/' {
				return i + 2, true
			}
		}
		return end, true
	}
	if data[pos] == '\'' || data[pos] == '"' {
		q := data[pos]
		for i := pos + 1; i < end; i++ {
			if data[i] == '\\' {
				i++
				continue
			}
			if data[i] == q {
				return i + 1, true
			}
		}
		return end, true
	}
	if hasURLPrefix(data, pos) {
		for i := pos + 3; i < end; i++ {
			if data[i] == '\\' {
				i++
				continue
			}
			if data[i] == ')' {
				return i + 1, true
			}
		}
		return end, true
	}
	return pos, false
}

// ---- parse_stylesheet (read-only compatibility view) ----

// cssRule 对齐 Python 的 Rule（selector + (name, value) 声明二元组）。
type cssRule struct {
	selector     string
	declarations [][2]string
}

// ErrUnsupportedCSSShape distinguishes valid CSS that the old semantic
// factoring algorithm cannot represent from malformed CSS. It is retained
// for read-only callers; Run does not need to parse stylesheet shape because
// semantic factoring is disabled until a token/span implementation exists.
var ErrUnsupportedCSSShape = errors.New("css cleanup: unsupported stylesheet shape")

type unsupportedCSSShapeError struct {
	detail string
}

func (e *unsupportedCSSShapeError) Error() string { return e.detail }
func (e *unsupportedCSSShapeError) Unwrap() error { return ErrUnsupportedCSSShape }

// parseStylesheet is retained as a compatibility view for old in-package
// tests. The write path uses parseStylesheetSafe so syntax errors become a
// hard, diagnosable cleanup error rather than a silent fallback.
func parseStylesheet(value string) ([]cssRule, bool) {
	rules, err := parseStylesheetSafe([]byte(value))
	return rules, err == nil && len(rules) > 0
}

func parseStylesheetSafe(value []byte) ([]cssRule, error) {
	sheet, err := css.Parse(value)
	if err != nil {
		return nil, err
	}
	var rules []cssRule
	for _, rule := range sheet.Rules {
		if rule.AtRule {
			// Flattening a conditional or declaration at-rule into a generated
			// top-level stylesheet changes cascade semantics. This is a valid
			// stylesheet, but an unsupported shape for the old factoring view.
			return nil, &unsupportedCSSShapeError{
				detail: fmt.Sprintf("unsupported at-rule @%s", rule.AtRuleName),
			}
		}
		// Keep the source projection exact. These values are read-only and
		// must never become a normalized serialization input.
		selector := rule.Selector
		if selector == "" || strings.HasPrefix(selector, "@") {
			return nil, &unsupportedCSSShapeError{detail: "empty or at-rule selector"}
		}
		var decls [][2]string
		for _, decl := range rule.Declarations {
			name := decl.Name
			value := decl.Value
			if name == "" {
				return nil, &unsupportedCSSShapeError{detail: "empty declaration name"}
			}
			decls = append(decls, [2]string{name, value})
		}
		rules = append(rules, cssRule{selector: selector, declarations: decls})
	}
	if len(rules) == 0 {
		return nil, &unsupportedCSSShapeError{detail: "stylesheet contains no qualified rules"}
	}
	return rules, nil
}

// uniqueZipPath is retained for compatibility with the removed factoring
// helpers. The active Run path never creates generated CSS entries.
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

// ---- XHTML link 重写（LINK_RE 的手工实现，含引号反向引用） ----

type linkMatch struct {
	start, end int
	valueStart int
	valueEnd   int
	quote      byte
	href       string
}

// findLinkMatch is a quote-aware XHTML link scanner. It returns only the
// href value span, so callers can patch that value without serializing the
// surrounding tag or document.
func findLinkMatch(text string, from int) (linkMatch, bool) {
	for i := max(from, 0); i < len(text); {
		if text[i] != '<' {
			i++
			continue
		}
		if strings.HasPrefix(text[i:], "<!--") {
			end := strings.Index(text[i+4:], "-->")
			if end < 0 {
				return linkMatch{}, false
			}
			i += end + 7
			continue
		}
		if strings.HasPrefix(text[i:], "<![CDATA[") {
			end := strings.Index(text[i+9:], "]]>")
			if end < 0 {
				return linkMatch{}, false
			}
			i += end + 12
			continue
		}
		if i+len("<link") > len(text) || !strings.EqualFold(text[i+1:i+len("<link")], "link") {
			if end, ok := htmlTagEnd(text, i+1); ok {
				i = end + 1
			} else {
				i++
			}
			continue
		}
		idx := i
		nameEnd := idx + len("<link")
		if nameEnd < len(text) && !isHTMLTagBoundary(text[nameEnd]) {
			i = idx + 1
			continue
		}
		tagClose, ok := htmlTagEnd(text, nameEnd)
		if !ok {
			return linkMatch{}, false
		}
		if valueStart, valueEnd, quote, href, ok := htmlHrefAttr(text, nameEnd, tagClose); ok {
			return linkMatch{start: idx, end: tagClose + 1,
				valueStart: valueStart, valueEnd: valueEnd,
				quote: quote, href: href}, true
		}
		i = tagClose + 1
	}
	return linkMatch{}, false
}

func htmlTagEnd(text string, from int) (int, bool) {
	quote := byte(0)
	for i := from; i < len(text); i++ {
		if quote != 0 {
			if text[i] == '\\' && i+1 < len(text) {
				i++
				continue
			}
			if text[i] == quote {
				quote = 0
			}
			continue
		}
		if text[i] == '\'' || text[i] == '"' {
			quote = text[i]
			continue
		}
		if text[i] == '>' {
			return i, true
		}
	}
	return 0, false
}

func htmlHrefAttr(text string, from, end int) (valueStart, valueEnd int, quote byte, href string, ok bool) {
	var found bool
	for i := from; i < end; {
		for i < end && isHTMLSpace(text[i]) {
			i++
		}
		if i >= end || text[i] == '/' {
			break
		}
		nameStart := i
		for i < end && isHTMLAttrNameByte(text[i]) {
			i++
		}
		if nameStart == i {
			i++
			continue
		}
		name := text[nameStart:i]
		for i < end && isHTMLSpace(text[i]) {
			i++
		}
		if i >= end || text[i] != '=' {
			continue
		}
		i++
		for i < end && isHTMLSpace(text[i]) {
			i++
		}
		if i >= end || (text[i] != '\'' && text[i] != '"') {
			continue
		}
		q := text[i]
		start := i + 1
		i = start
		for i < end && text[i] != q {
			if text[i] == '\\' && i+1 < end {
				i += 2
				continue
			}
			i++
		}
		if i >= end {
			return 0, 0, 0, "", false
		}
		if strings.EqualFold(name, "href") && i > start {
			candidate := text[start:i]
			if len(candidate) >= 4 && strings.EqualFold(candidate[len(candidate)-4:], ".css") {
				valueStart, valueEnd, quote, href, found = start, i, q, candidate, true
			}
		}
		i++
	}
	return valueStart, valueEnd, quote, href, found
}

// rewriteCSSLinkEdits returns edits against the supplied XHTML bytes. An
// existing link's href is the only original range replaced. If one stylesheet
// expands to multiple targets, additional link tags are inserted at a stable
// boundary; the existing tag is never rewritten as a whole.
func rewriteCSSLinkEdits(xhtmlPath string, data []byte, mapping map[string][]string) ([]editset.Edit, bool, error) {
	if !utf8.Valid(data) {
		return nil, false, fmt.Errorf("XHTML is not valid UTF-8: %w", css.ErrInvalidUTF8)
	}
	text := string(data)
	dir := pyDirname(xhtmlPath)
	var edits []editset.Edit
	changed := false
	for pos := 0; ; {
		m, ok := findLinkMatch(text, pos)
		if !ok {
			break
		}
		pos = m.end
		cssPath := pyNormPath(pyJoinPath(dir, m.href))
		targets, hit := mapping[cssPath]
		if !hit || len(targets) == 0 {
			continue
		}
		changed = true
		firstHref := relHref(xhtmlPath, targets[0])
		edits = append(edits, editset.Replace(xhtmlPath, int64(m.valueStart), int64(m.valueEnd-m.valueStart), []byte(escapeXHTMLValue(firstHref, m.quote))))
		if len(targets) == 1 {
			continue
		}
		for _, target := range targets[1:] {
			clone := append([]byte(nil), data[m.start:m.end]...)
			localStart := m.valueStart - m.start
			localEnd := m.valueEnd - m.start
			href := []byte(escapeXHTMLValue(relHref(xhtmlPath, target), m.quote))
			clone = append(append(append([]byte(nil), clone[:localStart]...), href...), clone[localEnd:]...)
			repl := append([]byte{'\n'}, clone...)
			edits = append(edits, editset.Insert(xhtmlPath, int64(m.end), repl))
		}
	}
	if err := editset.Validate(edits); err != nil {
		return nil, false, err
	}
	return edits, changed, nil
}

func escapeXHTMLValue(value string, quote byte) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	if quote == '\'' {
		return strings.ReplaceAll(value, "'", "&#39;")
	}
	return strings.ReplaceAll(value, `"`, "&quot;")
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

// linkedCSSPaths is retained only for compatibility with the old
// read-only helper surface. Run never calls it: scoped-local CSS merging is
// disabled until it can be implemented with a lossless token/span edit plan.
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
		for _, cssPath := range linkedCSSPaths(string(data), xhtmlPath) {
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
		if rules, err := parseStylesheetSafe(data); err == nil {
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
		for _, cssPath := range mergePaths {
			if refs[cssPath][xhtmlPath] {
				edits, added, err := addBodyClassEdits(xhtmlPath, data, scopeByPath[cssPath])
				if err != nil {
					rep.Warnings = append(rep.Warnings, "skipped body scope for "+xhtmlPath+": "+err.Error())
					continue
				}
				if err := m.patch(xhtmlPath, edits); err != nil {
					rep.Warnings = append(rep.Warnings, "skipped body scope for "+xhtmlPath+": "+err.Error())
					continue
				}
				data, _ = m.get(xhtmlPath)
				if added {
					rep.ScopeClassesAdded++
				}
			}
		}
		edits, changed, err := rewriteCSSLinkEdits(xhtmlPath, data, mapping)
		if err != nil {
			rep.Warnings = append(rep.Warnings, "skipped scoped links for "+xhtmlPath+": "+err.Error())
			continue
		}
		if changed {
			if err := m.patch(xhtmlPath, edits); err != nil {
				rep.Warnings = append(rep.Warnings, "skipped scoped links for "+xhtmlPath+": "+err.Error())
			}
		}
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
	for _, part := range selectorListParts(selector) {
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

// selectorListParts splits only commas at the top level of a selector list.
// Commas in :is(), attribute strings, comments, and escaped sequences remain
// part of the selector. An unbalanced selector is kept opaque rather than
// guessed into multiple selectors.
func selectorListParts(selector string) []string {
	data := []byte(selector)
	var parts []string
	start := 0
	paren, bracket, brace := 0, 0, 0
	for i := 0; i < len(data); {
		if i+1 < len(data) && data[i] == '/' && data[i+1] == '*' {
			next, err := skipHTMLCSSComment(data, i)
			if err != nil {
				return []string{selector}
			}
			i = next
			continue
		}
		if data[i] == '\'' || data[i] == '"' {
			next, err := skipHTMLCSSString(data, i)
			if err != nil {
				return []string{selector}
			}
			i = next
			continue
		}
		if data[i] == '\\' {
			if i+1 >= len(data) {
				return []string{selector}
			}
			i += 2
			continue
		}
		switch data[i] {
		case '(':
			paren++
		case ')':
			if paren == 0 {
				return []string{selector}
			}
			paren--
		case '[':
			bracket++
		case ']':
			if bracket == 0 {
				return []string{selector}
			}
			bracket--
		case '{':
			brace++
		case '}':
			if brace == 0 {
				return []string{selector}
			}
			brace--
		case ',':
			if paren == 0 && bracket == 0 && brace == 0 {
				parts = append(parts, selector[start:i])
				start = i + 1
			}
		}
		i++
	}
	if paren != 0 || bracket != 0 || brace != 0 {
		return []string{selector}
	}
	return append(parts, selector[start:])
}

func skipHTMLCSSComment(data []byte, start int) (int, error) {
	for i := start + 2; i+1 < len(data); i++ {
		if data[i] == '*' && data[i+1] == '/' {
			return i + 2, nil
		}
	}
	return len(data), css.ErrUnterminated
}

func skipHTMLCSSString(data []byte, start int) (int, error) {
	q := data[start]
	for i := start + 1; i < len(data); i++ {
		if data[i] == '\\' {
			if i+1 >= len(data) {
				return len(data), css.ErrUnterminated
			}
			i++
			continue
		}
		if data[i] == q {
			return i + 1, nil
		}
	}
	return len(data), css.ErrUnterminated
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
type bodyTagSpan struct {
	start, end           int
	attrsStart, attrsEnd int
}

// addBodyClassEdits scans the opening body tag and changes only its class
// value (or inserts a new class attribute). It never serializes the XHTML
// document or replaces the tag as a whole.
func addBodyClassEdits(path string, data []byte, className string) ([]editset.Edit, bool, error) {
	if className == "" || !isHTMLClassName(className) {
		return nil, false, fmt.Errorf("unsafe body class %q", className)
	}
	tag, ok := findBodyOpenTag(data)
	if !ok {
		return nil, false, nil
	}
	attrs := data[tag.attrsStart:tag.attrsEnd]
	if classAttr, found := findClassAttrBytes(attrs, 0); found {
		if contains(strings.Fields(classAttr.classes), className) {
			return nil, false, nil
		}
		value := append([]byte(nil), attrs[classAttr.valueStart:classAttr.valueEnd]...)
		if len(value) > 0 && !isHTMLSpace(value[len(value)-1]) {
			value = append(value, ' ')
		}
		value = append(value, className...)
		edits := []editset.Edit{editset.Replace(path,
			int64(tag.attrsStart+classAttr.valueStart),
			int64(classAttr.valueEnd-classAttr.valueStart), value)}
		return edits, true, editset.Validate(edits)
	}
	edits := []editset.Edit{editset.Insert(path, int64(tag.end-1),
		[]byte(` class="`+className+`"`))}
	return edits, true, editset.Validate(edits)
}

// addBodyClass is the old test-facing string helper and delegates to the
// byte-range implementation used by Run.
func addBodyClass(text, className string) (string, bool) {
	edits, added, err := addBodyClassEdits("<xhtml>", []byte(text), className)
	if err != nil || !added {
		return text, false
	}
	updated, err := editset.Apply("<xhtml>", []byte(text), edits)
	if err != nil {
		return text, false
	}
	return string(updated), true
}

func findBodyOpenTag(data []byte) (bodyTagSpan, bool) {
	for i := 0; i < len(data); i++ {
		if i+3 < len(data) && data[i] == '<' && data[i+1] == '!' &&
			data[i+2] == '-' && data[i+3] == '-' {
			end := indexBytes(data, []byte("-->"), i+4)
			if end < 0 {
				return bodyTagSpan{}, false
			}
			i = end + 2
			continue
		}
		if i+5 >= len(data) || data[i] != '<' || data[i+1] == '/' ||
			!equalFoldBytes(data[i+1:i+5], []byte("body")) {
			continue
		}
		nameEnd := i + len("<body")
		if nameEnd < len(data) && !isHTMLTagBoundary(data[nameEnd]) {
			continue
		}
		quote := byte(0)
		for j := nameEnd; j < len(data); j++ {
			if quote != 0 {
				if data[j] == '\\' && j+1 < len(data) {
					j++
					continue
				}
				if data[j] == quote {
					quote = 0
				}
				continue
			}
			switch data[j] {
			case '\'', '"':
				quote = data[j]
			case '>':
				if j > nameEnd && data[j-1] == '/' {
					return bodyTagSpan{}, false
				}
				return bodyTagSpan{start: i, end: j + 1,
					attrsStart: nameEnd, attrsEnd: j}, true
			}
		}
		return bodyTagSpan{}, false
	}
	return bodyTagSpan{}, false
}

func indexBytes(data, needle []byte, from int) int {
	if from < 0 || from > len(data) {
		return -1
	}
	if len(needle) == 0 {
		return from
	}
	if at := bytes.Index(data[from:], needle); at >= 0 {
		return from + at
	}
	return -1
}

func equalFoldBytes(a, b []byte) bool {
	return bytes.EqualFold(a, b)
}

func isHTMLSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '\f':
		return true
	default:
		return false
	}
}

func isHTMLTagBoundary(b byte) bool {
	return isHTMLSpace(b) || b == '>' || b == '/'
}

func isHTMLAttrNameByte(b byte) bool {
	return b == ':' || b == '_' || b == '-' || b >= 'a' && b <= 'z' ||
		b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

func isHTMLClassName(name string) bool {
	for i := 0; i < len(name); i++ {
		b := name[i]
		if !(b == '_' || b == '-' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9') {
			return false
		}
	}
	return true
}

func findClassAttrBytes(data []byte, from int) (classAttrMatch, bool) {
	for i := from; i < len(data); {
		for i < len(data) && isHTMLSpace(data[i]) {
			i++
		}
		if i >= len(data) || data[i] == '/' {
			return classAttrMatch{}, false
		}
		start := i
		for i < len(data) && isHTMLAttrNameByte(data[i]) {
			i++
		}
		if start == i {
			i++
			continue
		}
		name := data[start:i]
		for i < len(data) && isHTMLSpace(data[i]) {
			i++
		}
		if i >= len(data) || data[i] != '=' {
			for i < len(data) && !isHTMLSpace(data[i]) {
				i++
			}
			continue
		}
		i++
		for i < len(data) && isHTMLSpace(data[i]) {
			i++
		}
		if i >= len(data) {
			return classAttrMatch{}, false
		}
		q := data[i]
		if q != '\'' && q != '"' {
			for i < len(data) && !isHTMLSpace(data[i]) {
				i++
			}
			continue
		}
		valueStart := i + 1
		i = valueStart
		for i < len(data) {
			if data[i] == '\\' && i+1 < len(data) {
				i += 2
				continue
			}
			if data[i] == q {
				if bytes.EqualFold(name, []byte("class")) {
					return classAttrMatch{start: start, end: i + 1,
						valueStart: valueStart, valueEnd: i, quote: q,
						classes: string(data[valueStart:i])}, true
				}
				i++
				break
			}
			i++
		}
		if i >= len(data) {
			return classAttrMatch{}, false
		}
	}
	return classAttrMatch{}, false
}

type classAttrMatch struct {
	start, end           int
	valueStart, valueEnd int
	quote                byte
	classes              string
}

// findClassAttr 匹配 \bclass=(["'])([^"']*)(\1)（re.I，反向引用手工实现）。
func findClassAttr(text string, from int) (classAttrMatch, bool) {
	return findClassAttrBytes([]byte(text), from)
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
