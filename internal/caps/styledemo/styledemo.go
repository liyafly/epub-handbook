// Package styledemo 移植 scripts/validate_epub_style_demo.py（699 行）的
// epub-style-demo fixture 校验器为只读能力 epub.style.demo.maintain。
//
// 两种输入模式，与 Python 对齐：
//   - 源树模式（b == nil）：校验 demo 源树目录（默认
//     <repo>/templates/epub-style-demo，由 pipeline 以 args["demo_dir"] 提供）；
//   - 产物模式（b != nil）：先校验源树（Python main() 无条件先跑
//     validate_source），再校验 --input 指向的构建产物 EPUB。
//
// 错误措辞、触发顺序与退出码语义逐字对齐 Python oracle：有错误 → 逐条
// "ERROR: {msg}" 且退出码 1；通过 → stdout "epub-style-demo validation ok"。
// legacy_report 时这些行放进 Facts["legacyReport"]["lines"]。
//
// 与 Python 的已知分歧（均不在 parity 路径上）：
//   - run_epubcheck：Go 侧不执行外部 epubcheck（INV-4 禁止 caps 起
//     子进程；EPUBCheck 在 CI 作为独立 gate 运行）。artifact 模式以一条
//     warn 级 finding 说明；Python 侧的 "WARN: epubcheck skipped: ..." 不会
//     出现在 legacyReport 行里。
//   - XML 解析失败的 {exc} 文本来自 Go encoding/xml 而非 expat，措辞不同
//     （demo fixture 良构，parity 用例不触发）。
//   - 产物 zip 读取复用 internal/zipfs（层 2→6 方向合法）：artifact 校验
//     需要 entry 物理顺序（含目录项）与压缩方法（mimetype stored 检查），
//     book 模型不暴露这两者；zipfs 的 Names/Lookup/Read 与 Python
//     zipfile 的 namelist/infolist/read（重名后者覆盖）语义一致。
package styledemo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/zipfs"
)

// CapabilityID 是本能力的契约 id。
const CapabilityID = "epub.style.demo.maintain"

// Params 是本能力的参数。
type Params struct {
	// DemoDir 是 demo 源树根（templates/epub-style-demo 的绝对路径）。
	// 源树模式必填；产物模式必填（Python 的 validate_source 无条件先跑）。
	DemoDir string
	// LegacyReport 输出与 Python stdout/stderr 行一致的 findings 列表。
	LegacyReport bool
}

// Run 执行 demo fixture 校验（只读）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	res := report.Result{Capability: CapabilityID, Status: report.StatusComplete}
	var errs []string
	mode := "source-tree"

	if b != nil {
		mode = "artifact"
		if p.DemoDir == "" {
			return report.Result{}, errors.New("styledemo: demo_dir is required (Python oracle always validates the source fixture)")
		}
		if err := validateSource(newDiskSource(p.DemoDir), &errs); err != nil {
			return report.Result{}, err
		}
		if err := validateEpub(b.InputPath(), &errs); err != nil {
			return report.Result{}, err
		}
	} else {
		if p.DemoDir == "" {
			return report.Result{}, errors.New("styledemo: demo_dir is required in source-tree mode")
		}
		if err := validateSource(newDiskSource(p.DemoDir), &errs); err != nil {
			return report.Result{}, err
		}
	}

	if len(errs) > 0 {
		res.Status = report.StatusFailed
		for _, msg := range errs {
			res.Findings = append(res.Findings, report.Finding{
				Level: "error", ID: "styledemo", Title: msg,
			})
		}
	}
	if mode == "artifact" {
		res.Findings = append(res.Findings, report.Finding{
			Level: "warn", ID: "styledemo.epubcheck-skipped",
			Title: "epubcheck is not executed by the Go validator (external tool runs as the CI gate)",
		})
	}
	res.Facts = map[string]any{
		"errors": len(errs),
		"mode":   mode,
	}
	if p.LegacyReport {
		lines := make([]string, 0, len(errs)+1)
		for _, msg := range errs {
			lines = append(lines, "ERROR: "+msg)
		}
		if len(lines) == 0 {
			lines = append(lines, "epub-style-demo validation ok")
		}
		res.Facts["legacyReport"] = map[string]any{"lines": lines}
	}
	return res, nil
}

// ---- CSS 工具（对齐脚本顶层函数） ----

// stripCSSComments 对齐 strip_css_comments。
func stripCSSComments(css string) string {
	return reCSSComment.ReplaceAllString(css, "")
}

// selectorBlock 对齐 selector_block：首个 "{selector} {...}" 的 body
// （Python 按调用点用 re.escape(selector) 现场拼模式，这里同样现场编译）。
func selectorBlock(css, selector string) string {
	re, err := regexp.Compile(regexp.QuoteMeta(selector) + `\s*\{([^}]+)\}`)
	if err != nil {
		return ""
	}
	m := re.FindStringSubmatch(css)
	if m == nil {
		return ""
	}
	return m[1]
}

// selectorBlocks 对齐 selector_blocks：按逗号分词后整段精确匹配选择器。
func selectorBlocks(css, selector string) []string {
	css = stripCSSComments(css)
	var blocks []string
	for _, m := range reCSSRule.FindAllStringSubmatch(css, -1) {
		selectors, body := m[1], m[2]
		for _, part := range strings.Split(selectors, ",") {
			if strings.TrimSpace(part) == selector {
				blocks = append(blocks, body)
				break
			}
		}
	}
	return blocks
}

// percentageWidth 对齐 percentage_width。
func percentageWidth(css, selector string) (float64, bool) {
	block := selectorBlock(css, selector)
	if block == "" {
		return 0, false
	}
	m := rePercentageWidth.FindStringSubmatch(block)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// hasBodyFontLockedMarkup 对齐 has_body_font_locked_markup。
func hasBodyFontLockedMarkup(xhtml string) bool {
	return reBodyFontLockedDQ.MatchString(xhtml) || reBodyFontLockedSQ.MatchString(xhtml)
}

// hasDirectBodyFontFamily 对齐 has_direct_body_font_family。
func hasDirectBodyFontFamily(css string) bool {
	for _, m := range reCSSRule.FindAllStringSubmatch(css, -1) {
		selectors, body := m[1], m[2]
		if !reFontFamilyColon.MatchString(body) {
			continue
		}
		for _, selector := range strings.Split(selectors, ",") {
			parts := strings.Split(strings.TrimSpace(selector), ";")
			last := strings.TrimSpace(parts[len(parts)-1])
			if strings.ToLower(last) == "body" {
				return true
			}
		}
	}
	return false
}

// hasIbooksSpecifiedFonts 对齐 has_ibooks_specified_fonts。
func hasIbooksSpecifiedFonts(pkgRoot *element) bool {
	for _, md := range findallPath(pkgRoot, [2]string{opfNS, "metadata"}) {
		for _, meta := range findKidsNS(md, opfNS, "meta") {
			if prop, ok := meta.attr("property"); ok && prop == "ibooks:specified-fonts" {
				return strings.TrimSpace(strings.ToLower(meta.text)) == "true"
			}
		}
	}
	return false
}

// hasNamespacedMarkup 对齐 has_namespaced_markup（源树路径）。
// 解析失败按 Python 追加 "XML parse failed: {path}: {exc}" 并返回 False；
// 读取失败（Python 的 ET.parse 会崩溃）返回 error。
func hasNamespacedMarkup(src diskSource, href, uri string, errs *[]string) (bool, error) {
	data, err := os.ReadFile(src.hrefPath(href))
	if err != nil {
		return false, err
	}
	root, perr := parseXMLDoc(data)
	if perr != nil {
		*errs = append(*errs, fmt.Sprintf("XML parse failed: %s: %v", src.hrefPath(href), perr))
		return false, nil
	}
	for _, el := range iterAll(root) {
		if el.space == uri {
			return true, nil
		}
	}
	return false, nil
}

// ---- body font 模式契约（源树与产物共用） ----

func validateBodyFontModeContract(pkgRoot *element, baseCSS, fontsCSS string, xhtmlTexts map[string]string, errs *[]string, context string) {
	require := func(cond bool, msg string) {
		if !cond {
			*errs = append(*errs, msg)
		}
	}
	activeBaseCSS := stripCSSComments(baseCSS)
	bodyBlock := selectorBlock(activeBaseCSS, "body")
	require(bodyBlock != "", context+": base.css must define a body block")
	if bodyBlock != "" {
		require(!reFontFamilyColon.MatchString(bodyBlock),
			context+": base.css body block must not set font-family; put locked-mode role binding in fonts.css")
	}

	activeFontsCSS := stripCSSComments(fontsCSS)
	hasClassRule := reBodyFontClassRule.MatchString(activeFontsCSS)
	hasDirectRule := hasDirectBodyFontFamily(activeFontsCSS)
	require(hasClassRule || hasDirectRule,
		context+": fonts.css must define a direct body or legacy .body-font-locked font-family chain")

	var lockedHrefs []string
	for href, text := range xhtmlTexts {
		if hasBodyFontLockedMarkup(text) {
			lockedHrefs = append(lockedHrefs, href)
		}
	}
	sort.Strings(lockedHrefs)
	hasLockedMode := len(lockedHrefs) > 0 || hasDirectRule
	hasMeta := hasIbooksSpecifiedFonts(pkgRoot)
	require(hasLockedMode == hasMeta,
		fmt.Sprintf("%s: locked body font mode and OPF ibooks:specified-fonts meta must match; "+
			"direct_body=%s, locked_pages=%s, meta=%s",
			context, pyBool(hasDirectRule), pyListOrNone(lockedHrefs), pyBool(hasMeta)))
}

// validateChapterOpeningContract 校验 scene 28 的 poster.css/XHTML 联动契约。
// 该 helper 同时用于源树和 EPUB 产物，且只读 CSS/XHTML 文本，不做任何
// 序列化或重写。
func validateChapterOpeningContract(posterCSS, chapterOpeningText string, errs *[]string) {
	require := func(cond bool, msg string) {
		if !cond {
			*errs = append(*errs, msg)
		}
	}
	activePosterCSS := stripCSSComments(posterCSS)
	require(strings.Count(activePosterCSS, `url("../Images/chapter-banner.png")`) == 1,
		"chapter-opening block must use exactly one poster.css chapter-banner background layer")
	for _, token := range chapterOpeningTokens {
		require(strings.Contains(chapterOpeningText, token),
			"28-chapter-opening-block.xhtml missing marker: "+token)
	}
	require(!strings.Contains(chapterOpeningText, "style="),
		"28-chapter-opening-block.xhtml must keep the background out of inline style")
	require(!strings.Contains(chapterOpeningText, "<img"),
		"28-chapter-opening-block.xhtml ornament must not enter normal flow as img")
	require(reChapterOpeningBackground.MatchString(activePosterCSS),
		"chapter-opening block background must be shared CSS at left bottom / 5.5em auto")
	require(reChapterOpeningMain.MatchString(activePosterCSS),
		"chapter-opening block title group must retain its production-derived top/right spacing")
	require(reChapterOpeningNumberTitle.MatchString(activePosterCSS),
		"chapter-opening ordinal and title must remain block spans")
	require(reChapterOpeningTitlePadding.MatchString(activePosterCSS),
		"chapter-opening title must retain its block-level gap below the ordinal")
}

// validateChapterOpeningNavigation proves that scene 28 is not merely present
// in the ZIP: it must be a unique manifest item and remain reachable through
// the reading order and both EPUB navigation documents.
func validateChapterOpeningNavigation(pkgRoot, navRoot, ncxRoot *element, errs *[]string) {
	require := func(cond bool, msg string) {
		if !cond {
			*errs = append(*errs, msg)
		}
	}
	if pkgRoot != nil {
		manifestHrefCount := 0
		manifestIDCount := 0
		matchingID := ""
		for _, item := range findallPath(pkgRoot, [2]string{opfNS, "manifest"}, [2]string{opfNS, "item"}) {
			if item.attrOr("id") == chapterOpeningID {
				manifestIDCount++
			}
			if item.attrOr("href") == chapterOpeningHref {
				manifestHrefCount++
				matchingID = item.attrOr("id")
			}
		}
		require(manifestHrefCount == 1,
			"28-chapter-opening-block.xhtml must appear exactly once in manifest")
		require(manifestIDCount == 1 && matchingID == chapterOpeningID,
			"28-chapter-opening-block.xhtml must use unique manifest id=chapter-opening-block")

		spineCount := 0
		for _, itemref := range findallPath(pkgRoot, [2]string{opfNS, "spine"}, [2]string{opfNS, "itemref"}) {
			if itemref.attrOr("idref") == chapterOpeningID {
				spineCount++
			}
		}
		require(spineCount == 1,
			"28-chapter-opening-block.xhtml must appear exactly once in spine")
	}
	if navRoot != nil {
		navCount := 0
		for _, link := range findAllDesc(navRoot, xhtmlNS, "a") {
			if link.attrOr("href") == chapterOpeningHref {
				navCount++
			}
		}
		require(navCount == 1,
			"28-chapter-opening-block.xhtml must appear exactly once in nav.xhtml")
	}
	if ncxRoot != nil {
		ncxCount := 0
		for _, content := range findAllDesc(ncxRoot, ncxNS, "content") {
			if content.attrOr("src") == chapterOpeningHref {
				ncxCount++
			}
		}
		require(ncxCount == 1,
			"28-chapter-opening-block.xhtml must appear exactly once in toc.ncx")
	}
}

// ---- 源树模式 ----

// validateSource 逐行对齐 validate_source。
// 返回 error 对齐 Python 的未捕获异常（必需文件缺失/解码失败等会带
// traceback 崩溃退出；这里转为 Go error，已累积的错误行随之丢弃，与
// Python 行为一致）。
func validateSource(src diskSource, errs *[]string) error {
	fail := func(msg string) { *errs = append(*errs, msg) }
	require := func(cond bool, msg string) {
		if !cond {
			fail(msg)
		}
	}

	// package_root = parse_xml(PACKAGE, check)
	pkgData, err := src.read(relPackage)
	if err != nil {
		return err
	}
	packageRoot, perr := parseXMLDoc(pkgData)
	if perr != nil {
		fail(fmt.Sprintf("XML parse failed: %s: %v", src.abs(relPackage), perr))
		return nil
	}

	manifest := buildManifestMap(findallPath(packageRoot, [2]string{opfNS, "manifest"}, [2]string{opfNS, "item"}))
	hrefToItem := buildHrefMap(manifest.values())

	navItems := 0
	for _, item := range manifest.values() {
		if tokenSetOf(item.attrOr("properties"))["nav"] {
			navItems++
		}
	}
	require(navItems == 1, "OPF manifest must contain exactly one nav item")
	require(manifest.has("ncx"), "OPF manifest must contain toc.ncx item id=ncx")

	for _, item := range manifest.values() {
		href := item.attrOr("href")
		if href == "" {
			id, ok := item.attr("id")
			if !ok {
				id = "<missing id>"
			}
			fail(fmt.Sprintf("Manifest item %s has no href", id))
			continue
		}
		require(src.hrefExists(href), "Manifest href missing on disk: "+href)
	}

	for _, itemref := range findallPath(packageRoot, [2]string{opfNS, "spine"}, [2]string{opfNS, "itemref"}) {
		idref, ok := itemref.attr("idref")
		if !ok {
			idref = "None"
		}
		require(manifest.has(idref), "Spine idref missing from manifest: "+idref)
	}

	for _, href := range hrefToItem.order {
		if href == "" || !strings.HasSuffix(href, ".xhtml") {
			continue
		}
		if !src.hrefExists(href) {
			continue
		}
		item, _ := hrefToItem.get(href)
		props := tokenSetOf(item.attrOr("properties"))
		mathml, err := hasNamespacedMarkup(src, href, mathmlURI, errs)
		if err != nil {
			return err
		}
		if mathml {
			require(props["mathml"], "MathML content missing OPF properties=mathml: "+href)
		}
		svg, err := hasNamespacedMarkup(src, href, svgURI, errs)
		if err != nil {
			return err
		}
		if svg {
			require(props["svg"], "Inline SVG content missing OPF properties=svg: "+href)
		}
	}

	require(hrefToItem.has("Text/16-math.xhtml"), "16-math.xhtml must be in manifest")
	if mathItem, ok := hrefToItem.get("Text/16-math.xhtml"); ok {
		require(tokenSetOf(mathItem.attrOr("properties"))["mathml"],
			"16-math.xhtml manifest item must declare properties=mathml")
	}
	noteItem, noteFound := hrefToItem.get("Text/19-border-shadow-notes.xhtml")
	require(noteFound, "19-border-shadow-notes.xhtml must be in manifest")
	if noteFound {
		require(tokenSetOf(noteItem.attrOr("properties"))["svg"],
			"19-border-shadow-notes.xhtml manifest item must declare properties=svg")
	}
	require(hrefToItem.has("Text/20-chapter-head-image.xhtml"), "20-chapter-head-image.xhtml must be in manifest")
	require(hrefToItem.has("Text/21-classical-modern.xhtml"), "21-classical-modern.xhtml must be in manifest")
	require(hrefToItem.has("Text/03c-poster-contain.xhtml"), "03c-poster-contain.xhtml must be in manifest")

	navData, err := src.read(relNav)
	if err != nil {
		return err
	}
	navRoot, perr := parseXMLDoc(navData)
	if perr != nil {
		fail(fmt.Sprintf("XML parse failed: %s: %v", src.abs(relNav), perr))
	} else {
		for _, link := range findAllDesc(navRoot, xhtmlNS, "a") {
			href, ok := link.attr("href")
			if ok && href != "" && !strings.HasPrefix(href, "#") {
				require(src.hrefExists(href), "nav.xhtml link missing: "+href)
			}
		}
	}

	ncxData, err := src.read(relNCX)
	if err != nil {
		return err
	}
	ncxRoot, perr := parseXMLDoc(ncxData)
	if perr != nil {
		fail(fmt.Sprintf("XML parse failed: %s: %v", src.abs(relNCX), perr))
	} else {
		for _, content := range findAllDesc(ncxRoot, ncxNS, "content") {
			srcAttr, ok := content.attr("src")
			if ok && srcAttr != "" {
				require(src.hrefExists(srcAttr), "toc.ncx content missing: "+srcAttr)
			}
		}
	}
	validateChapterOpeningNavigation(packageRoot, navRoot, ncxRoot, errs)

	baseCSS, err := src.readUTF8(relBaseCSS)
	if err != nil {
		return err
	}
	fontsCSS, err := src.readUTF8(relFontsCSS)
	if err != nil {
		return err
	}
	require(!strings.Contains(stripCSSComments(fontsCSS), "../Fonts/"),
		"fonts.css default @font-face skeleton leaked an active missing font URL")

	xhtmlTexts := map[string]string{}
	for _, href := range hrefToItem.order {
		if href == "" || !strings.HasSuffix(href, ".xhtml") || !src.hrefExists(href) {
			continue
		}
		text, err := src.readHrefUTF8(href)
		if err != nil {
			return err
		}
		xhtmlTexts[href] = text
	}
	validateBodyFontModeContract(packageRoot, baseCSS, fontsCSS, xhtmlTexts, errs, "source fixture")

	posterCSS, err := src.readUTF8(relPosterCSS)
	if err != nil {
		return err
	}
	activePosterCSS := stripCSSComments(posterCSS)
	posterContainText, err := src.readUTF8(relPosterPage)
	if err != nil {
		return err
	}
	chapterOpeningText, err := src.readUTF8(relChapterOpening)
	if err != nil {
		return err
	}
	for _, token := range posterCSSTokens {
		require(strings.Contains(posterCSS, token), "poster.css missing single-image contain fallback style: "+token)
	}
	for _, token := range posterPageTokens {
		require(strings.Contains(posterContainText, token), "03c-poster-contain.xhtml missing marker: "+token)
	}
	validateChapterOpeningContract(posterCSS, chapterOpeningText, errs)
	require(!strings.Contains(activePosterCSS, "position: absolute"), "poster.css must not use position:absolute")
	require(!reVhVw.MatchString(activePosterCSS), "poster.css must not use vh/vw units")

	noteCSS, err := src.readUTF8(relNotesCSS)
	if err != nil {
		return err
	}
	rubyNoteText, err := src.readUTF8(relRubyPage)
	if err != nil {
		return err
	}
	require(strings.Contains(rubyNoteText, `class="note-marker"`),
		"02-ruby-note.xhtml must scope image noteref in note-marker")
	for _, token := range noteCSSTokens {
		require(strings.Contains(noteCSS, token), "notes.css missing scoped note-marker baseline rule: "+token)
	}
	require(!strings.Contains(noteCSS, "sup img"), "notes.css must not use a global sup img rule")

	mediaCSS, err := src.readUTF8(relMediaCSS)
	if err != nil {
		return err
	}
	imageLayout, err := src.readUTF8(relImagePage)
	if err != nil {
		return err
	}
	require(!strings.Contains(mediaCSS, "kindle-img"),
		"media.css must not define direct img kindle-* float classes")
	require(!strings.Contains(imageLayout, "kindle-img"),
		"17-image-layout must not use direct img kindle-* float classes")
	for _, selector := range []string{".img-left", ".img-right"} {
		width, found := percentageWidth(mediaCSS, selector)
		require(found, selector+" must define percentage width")
		if found {
			require(25 <= width && width <= 35,
				fmt.Sprintf("%s width must stay in the 25%%-35%% range, found %s%%", selector, pyFormatG(width)))
		}
	}
	require(!strings.Contains(mediaCSS, "aspect-ratio"),
		"media.css must not depend on aspect-ratio for image wrapping")
	require(strings.Contains(imageLayout, `class="img-left"`), "17-image-layout must include figure.img-left")
	require(strings.Contains(imageLayout, `class="img-right"`), "17-image-layout must include figure.img-right")
	require(strings.Contains(imageLayout, "短段反例"), "17-image-layout must include a short-text threshold counterexample")
	require(strings.Contains(imageLayout, "大字号 figure 回归"), "17-image-layout must include large-font figure regression")
	for _, token := range imageLayoutSizeTokens {
		require(strings.Contains(imageLayout, token), "17-image-layout.xhtml missing image sizing marker: "+token)
	}
	for _, token := range mediaCSSTokens {
		require(strings.Contains(mediaCSS, token), "media.css missing scoped image sizing style: "+token)
	}
	narrowBlocks := selectorBlocks(mediaCSS, ".figure-pair .image-pair-narrow")
	wideBlocks := selectorBlocks(mediaCSS, ".figure-pair .image-pair-wide")
	require(len(narrowBlocks) > 0, "media.css must define a wide-screen narrow figure-pair rule")
	require(len(wideBlocks) > 0, "media.css must define a wide-screen wide figure-pair rule")
	for _, block := range append(append([]string{}, narrowBlocks...), wideBlocks...) {
		require(!strings.Contains(block, "float:"), "non-equal figure pair must not use float")
	}
	require(!reGenericImgWidth.MatchString(mediaCSS),
		"media.css must not introduce a generic img width:100% rule for image sizing")

	mathText, err := src.readUTF8(relMathPage)
	if err != nil {
		return err
	}
	for _, token := range mathmlTokens {
		require(strings.Contains(mathText, token), "16-math.xhtml missing MathML sample: "+token)
	}
	for _, token := range eqLayoutPageTokens {
		require(strings.Contains(mathText, token), "16-math.xhtml missing equation layout marker: "+token)
	}
	require(!strings.Contains(mathText, "<mlabeledtr"),
		"16-math.xhtml must not use mlabeledtr as the equation-numbering path")
	for _, token := range dataTablePageTokens {
		require(strings.Contains(mathText, token), "16-math.xhtml missing semantic data-table marker: "+token)
	}
	for _, token := range dataTableCSSTokens {
		require(strings.Contains(mediaCSS, token), "media.css missing scoped MathML data-table style: "+token)
	}
	dataTableBlocks := append(append(append(append([]string{},
		selectorBlocks(mediaCSS, ".math-data-table")...),
		selectorBlocks(mediaCSS, ".math-data-table th")...),
		selectorBlocks(mediaCSS, ".math-data-table td")...),
		selectorBlocks(mediaCSS, ".math-data-table math")...)
	mathBlocks := selectorBlocks(mediaCSS, ".math-data-table math")
	anyEmFontSize := false
	for _, block := range mathBlocks {
		if reFontSizeEm.MatchString(block) {
			anyEmFontSize = true
			break
		}
	}
	require(anyEmFontSize,
		"media.css must give .math-data-table math a scoped relative em font-size candidate")
	dataTableCSS := strings.Join(dataTableBlocks, "\n")
	for _, forbidden := range []string{"overflow: hidden", "max-content"} {
		require(!strings.Contains(dataTableCSS, forbidden), "math-data-table styles must not use "+forbidden)
	}
	dataTableMatch := reMathDataTable.FindStringSubmatch(mathText)
	require(dataTableMatch != nil, "16-math.xhtml must contain the math-data-table fixture")
	if dataTableMatch != nil {
		markup := dataTableMatch[1]
		require(strings.Count(markup, "<math ") == 4, "math-data-table must contain four MathML formulas")
		require(strings.Count(markup, "<semantics>") == 4, "math-data-table formulas must all use semantics")
		annotations := reTeXAnnotation.FindAllStringSubmatch(markup, -1)
		annotationsOK := len(annotations) == 4
		if annotationsOK {
			for _, a := range annotations {
				if strings.TrimSpace(a[1]) == "" {
					annotationsOK = false
					break
				}
			}
		}
		require(annotationsOK, "math-data-table formulas must all have non-empty TeX annotations")
	}
	for _, token := range eqLayoutCSSTokens {
		require(strings.Contains(mediaCSS, token), "media.css missing equation layout style: "+token)
	}

	englishText, err := src.readUTF8(relEnglish)
	if err != nil {
		return err
	}
	for _, token := range englishPageTokens {
		require(strings.Contains(englishText, token), "18-english-fiction.xhtml missing English fiction marker: "+token)
	}

	literaryCSS, err := src.readUTF8(relLiterary)
	if err != nil {
		return err
	}
	for _, token := range literaryEnglishTokens {
		require(strings.Contains(literaryCSS, token), "literary.css missing English fiction style: "+token)
	}

	frontmatterText, err := src.readUTF8(relFrontPage)
	if err != nil {
		return err
	}
	for _, token := range frontPageTokens {
		require(strings.Contains(frontmatterText, token), "15-frontmatter.xhtml missing copyright marker: "+token)
	}
	for _, token := range frontCSSTokens {
		require(strings.Contains(literaryCSS, token), "literary.css missing copyright page style: "+token)
	}

	effectsCSS, err := src.readUTF8(relEffects)
	if err != nil {
		return err
	}
	activeEffectsCSS := stripCSSComments(effectsCSS)
	noteText, err := src.readUTF8(relNoteBoxes)
	if err != nil {
		return err
	}
	// SPEC §5.10 bans rotated note boxes after Kindle Previewer 3.104 KFX failures.
	require(!reCSSTransformRotate.MatchString(activeEffectsCSS),
		"effects.css note fixtures must not use transform: rotate(); see docs/final/SPEC-实现约束.md §5.10")
	for _, token := range effectsCSSTokens {
		require(strings.Contains(effectsCSS, token), "effects.css missing note box style: "+token)
	}
	for _, token := range noteBoxPageTokens {
		require(strings.Contains(noteText, token), "19-border-shadow-notes.xhtml missing note box sample: "+token)
	}

	chapterHeadText, err := src.readUTF8(relChapterHead)
	if err != nil {
		return err
	}
	for _, token := range chapterHeadCSSTokens {
		require(strings.Contains(literaryCSS, token), "literary.css missing chapter head image style: "+token)
	}
	for _, token := range chapterHeadPageTokens {
		require(strings.Contains(chapterHeadText, token), "20-chapter-head-image.xhtml missing chapter head marker: "+token)
	}

	classicalModernText, err := src.readUTF8(relClassical)
	if err != nil {
		return err
	}
	for _, token := range classicalPageTokens {
		require(strings.Contains(classicalModernText, token), "21-classical-modern.xhtml missing marker: "+token)
	}
	for _, token := range classicalCSSTokens {
		require(strings.Contains(literaryCSS, token), "literary.css missing classical-modern style: "+token)
	}
	classicalBlock := selectorBlock(literaryCSS, ".parallel-float-pair .classical-text")
	modernBlock := selectorBlock(literaryCSS, ".parallel-float-pair .modern-text")
	pairBlock := selectorBlock(literaryCSS, ".parallel-pair")
	floatPairBlock := selectorBlock(literaryCSS, ".parallel-float-pair")
	stackPairBlock := selectorBlock(literaryCSS, ".parallel-stack-pair")
	classicalWidth, classicalOK := percentageWidth(literaryCSS, ".parallel-float-pair .classical-text")
	modernWidth, modernOK := percentageWidth(literaryCSS, ".parallel-float-pair .modern-text")
	balancedClassicalWidth, _ := percentageWidth(literaryCSS, ".parallel-float-pair.parallel-ratio-balanced .classical-text")
	balancedModernWidth, _ := percentageWidth(literaryCSS, ".parallel-float-pair.parallel-ratio-balanced .modern-text")
	sourceWideClassicalWidth, _ := percentageWidth(literaryCSS, ".parallel-float-pair.parallel-ratio-source-wide .classical-text")
	sourceWideModernWidth, _ := percentageWidth(literaryCSS, ".parallel-float-pair.parallel-ratio-source-wide .modern-text")
	require(!strings.Contains(pairBlock, "display: flex"), "parallel-pair must not depend on display:flex")
	require(!strings.Contains(pairBlock, "page-break-inside: avoid"),
		"parallel-pair default must allow long stacked pairs to paginate")
	require(strings.Contains(floatPairBlock, "page-break-inside: avoid"),
		"parallel-float-pair must protect short source/translation pairs from page splits")
	require(strings.Contains(stackPairBlock, "page-break-inside: auto"),
		"parallel-stack-pair must explicitly allow long stacked pairs to paginate")
	require(strings.Contains(literaryCSS, "@media (min-width: 40em)"),
		"parallel float layout must be a wide-screen progressive enhancement")
	require(strings.Contains(classicalBlock, "float: left"), "classical-text must float left in the wide enhancement")
	require(strings.Contains(modernBlock, "float: right"), "modern-text must float right in the wide enhancement")
	require(!strings.Contains(literaryCSS, "display: flex"), "classical-modern layout must not depend on flex")
	require(classicalOK, "classical-text must define percentage width in the enhancement")
	require(modernOK, "modern-text must define percentage width in the enhancement")
	if classicalOK {
		require(36 <= classicalWidth && classicalWidth <= 40,
			fmt.Sprintf("classical-text width must stay near the sample 38/58 split, found %s%%", pyFormatG(classicalWidth)))
	}
	if modernOK {
		require(56 <= modernWidth && modernWidth <= 60,
			fmt.Sprintf("modern-text width must stay near the sample 38/58 split, found %s%%", pyFormatG(modernWidth)))
	}
	require(widthEq(balancedClassicalWidth, 48) && widthEq(balancedModernWidth, 48),
		"parallel-ratio-balanced must define a 48/48 split")
	require(widthEq(sourceWideClassicalWidth, 58) && widthEq(sourceWideModernWidth, 38),
		"parallel-ratio-source-wide must define a 58/38 split")

	return nil
}

// widthEq 对齐 Python 的 `width == v`（None 比较为 False；percentageWidth
// 未命中时返回 0/false，0 != 48 等价于 None == 48）。
func widthEq(w float64, v float64) bool { return w == v }

// ---- 产物模式 ----

// validateEpub 对齐 validate_epub + run_epubcheck（Go 不执行 epubcheck）。
func validateEpub(epubPath string, errs *[]string) error {
	if _, err := os.Stat(epubPath); err != nil {
		*errs = append(*errs, "EPUB does not exist: "+epubPath)
		return nil
	}
	arch, err := zipfs.Open(epubPath)
	if err != nil {
		return err
	}
	defer arch.Close()

	if zerr := validateEpubZip(arch, errs); zerr != nil {
		// 对齐 Python 的 try/except (BadZipFile, ParseError, KeyError,
		// UnicodeDecodeError)：整个 zip 块与 epubcheck 一起中止。
		*errs = append(*errs, fmt.Sprintf("EPUB validation failed: %s: %s", epubPath, zerr.Error()))
	}
	// run_epubcheck：Go 侧不执行（见包注释），CI 以 EPUBCheck 作为独立 gate。
	return nil
}

// validateEpubZip 对齐 validate_epub 的 try 块；返回非 nil error 模拟
// Python 异常（KeyError 用 repr 字符串形状，如 'mimetype'）。
func validateEpubZip(arch *zipfs.Archive, errs *[]string) error {
	require := func(cond bool, msg string) {
		if !cond {
			*errs = append(*errs, msg)
		}
	}
	names := arch.Names()
	require(len(names) > 0, "EPUB zip is empty")
	if len(names) > 0 {
		first, _ := arch.Lookup(names[0])
		require(first.Name() == "mimetype", "EPUB mimetype must be first zip entry")
		require(first.MethodCode() == methodStore, "EPUB mimetype must be stored")
		content, err := arch.Read("mimetype")
		if err != nil {
			// KeyError 的 str 形状（实测 CPython 3.14 zipfile）。
			return errors.New(`"There is no item named 'mimetype' in the archive"`)
		}
		require(string(content) == "application/epub+zip", "EPUB mimetype content is invalid")
	}

	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	require(nameSet[zipPackage], "EPUB missing OEBPS/package.opf")
	var packageRoot *element
	if nameSet[zipPackage] {
		pkgData, err := arch.Read(zipPackage)
		if err != nil {
			return err
		}
		root, perr := parseXMLDoc(pkgData)
		if perr != nil {
			return perr
		}
		packageRoot = root
		xhtmlTexts := map[string]string{}
		for _, item := range findallPath(root, [2]string{opfNS, "manifest"}, [2]string{opfNS, "item"}) {
			href := item.attrOr("href")
			if href == "" {
				continue
			}
			full := pyNormPath(pyJoin("OEBPS", stripFragment(href)))
			require(nameSet[full], "EPUB manifest href missing in zip: "+href)
			if strings.HasSuffix(href, ".xhtml") && nameSet[full] {
				data, err := arch.Read(full)
				if err != nil {
					return err
				}
				text, derr := pyDecodeUTF8(data)
				if derr != nil {
					return derr
				}
				xhtmlTexts[href] = text
			}
		}
		if nameSet[zipBaseCSS] && nameSet[zipFontsCSS] {
			baseData, err := arch.Read(zipBaseCSS)
			if err != nil {
				return err
			}
			fontsData, err := arch.Read(zipFontsCSS)
			if err != nil {
				return err
			}
			base, derr := pyDecodeUTF8(baseData)
			if derr != nil {
				return derr
			}
			fonts, derr := pyDecodeUTF8(fontsData)
			if derr != nil {
				return derr
			}
			validateBodyFontModeContract(root, base, fonts, xhtmlTexts, errs, "EPUB artifact")
		} else {
			*errs = append(*errs, "EPUB artifact missing Styles/base.css or Styles/fonts.css")
		}
	}

	var navRoot, ncxRoot *element
	for _, document := range []struct {
		path string
		dst  **element
	}{
		{path: zipNav, dst: &navRoot},
		{path: zipNCX, dst: &ncxRoot},
	} {
		if !nameSet[document.path] {
			*errs = append(*errs, "EPUB artifact missing "+document.path)
			continue
		}
		data, err := arch.Read(document.path)
		if err != nil {
			return err
		}
		root, err := parseXMLDoc(data)
		if err != nil {
			return err
		}
		*document.dst = root
	}
	validateChapterOpeningNavigation(packageRoot, navRoot, ncxRoot, errs)

	var posterCSS, chapterOpeningText string
	posterFound := nameSet[zipPosterCSS]
	chapterOpeningFound := nameSet[zipChapterOpening]
	if !posterFound {
		*errs = append(*errs, "EPUB artifact missing "+zipPosterCSS)
	} else {
		data, err := arch.Read(zipPosterCSS)
		if err != nil {
			return err
		}
		posterCSS, err = pyDecodeUTF8(data)
		if err != nil {
			return err
		}
	}
	if !chapterOpeningFound {
		*errs = append(*errs, "EPUB artifact missing "+zipChapterOpening)
	} else {
		data, err := arch.Read(zipChapterOpening)
		if err != nil {
			return err
		}
		chapterOpeningText, err = pyDecodeUTF8(data)
		if err != nil {
			return err
		}
	}
	if posterFound && chapterOpeningFound {
		validateChapterOpeningContract(posterCSS, chapterOpeningText, errs)
	}
	return nil
}

// tokenSetOf 对齐 split_props：按空白分词。
func tokenSetOf(s string) map[string]bool {
	out := map[string]bool{}
	for _, tok := range strings.Fields(s) {
		out[tok] = true
	}
	return out
}
