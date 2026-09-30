// xhtml.go 实现 EPUB3 迁移的 XHTML 页面处理：shell 属性与插入使用字节范围
// 编辑；保留 plain/Sigil 旧尾注文本，只规范化已存在的 Duokan 标记。
package migrateepub3

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/scan/xhtml"
)

// xhtmlDefaultLanguage 逐行复刻 core.xhtml_default_language。
func xhtmlDefaultLanguage(root *xmlElem) string {
	meta := root.childByTag(opfURI, "metadata")
	if meta == nil {
		return ""
	}
	for _, child := range meta.children {
		if child.name != "language" {
			continue
		}
		if child.text != "" && pyStrip(child.text) != "" {
			return pyStrip(child.text)
		}
	}
	return ""
}

// updateXHTMLFiles 逐行复刻 core.update_xhtml_files。
func updateXHTMLFiles(files *workFiles, root *xmlElem, opfPath string, rep *conversionReport) error {
	opfDir := pypath.Dirname(opfPath)
	_, byZip := manifestMaps(root, opfDir)
	keys := make([]string, 0, len(byZip))
	for k := range byZip {
		keys = append(keys, k)
	}
	sortStrings(keys)
	defaultLanguage := xhtmlDefaultLanguage(root)
	for _, zipPath := range keys {
		item := byZip[zipPath]
		if item.attrOr("media-type", "") != "application/xhtml+xml" || !files.has(zipPath) {
			continue
		}
		original, err := files.read(zipPath)
		if err != nil {
			continue
		}
		if !xhtmlSourceIsUTF8(original) {
			return convErrf("structure.non-utf8-text: %s: text must be UTF-8 for lossless rewrite; 先人工转码为 UTF-8，再重新 S0 冻结", zipPath)
		}
		text := utf8ReplaceDecode(original)
		hasLegacyBig, err := xhtmlHasLegacyBigTag(text)
		if err != nil {
			return convErrf("%s: cannot scan legacy big elements: %v", zipPath, err)
		}
		if hasLegacyBig && rep.legacyBigTagPath == "" {
			rep.legacyBigTagPath = zipPath
		}
		text, changed, err := normalizeXHTMLShell(text, defaultLanguage)
		if err != nil {
			return convErrf("%s: cannot normalize XHTML shell: %v", zipPath, err)
		}
		var normalized int
		var duokanWarnings []string
		text, normalized, duokanWarnings, err = normalizeDuokanNotes(text)
		if err != nil {
			return convErrf("%s: cannot normalize Duokan notes: %v", zipPath, err)
		}
		for _, w := range duokanWarnings {
			rep.Warnings = append(rep.Warnings, zipPath+": "+w)
		}
		if normalized > 0 {
			rep.DuokanNotesNormalized += normalized
			changed = true
		}
		hasSVG, hasMathML, hasScript, err := xhtmlFeatureTags(text)
		if err != nil {
			return convErrf("%s: cannot scan XHTML feature tags: %v", zipPath, err)
		}
		if hasSVG && addProps(item, "svg") {
			rep.ManifestItemsUpdated++
		}
		if hasMathML && addProps(item, "mathml") {
			rep.ManifestItemsUpdated++
		}
		if hasScript && addProps(item, "scripted") {
			rep.ManifestItemsUpdated++
		}
		if changed {
			files.write(zipPath, []byte(text))
			rep.XHTMLFilesUpdated++
		}
	}
	return nil
}

func xhtmlFeatureTags(text string) (svg, mathml, scripted bool, err error) {
	regions, stop := xhtml.ScanRegions(text)
	if stop != xhtml.ScanComplete {
		return false, false, false, fmt.Errorf("markup scan stopped at byte offset %d", stop)
	}
	for _, region := range regions {
		name, closing, valid := regionTagNameAndClosing(text, region)
		if !valid || closing {
			continue
		}
		switch {
		case sameLocalName(name, "svg"):
			svg = true
		case sameLocalName(name, "math"):
			mathml = true
		case sameLocalName(name, "script"):
			scripted = true
		}
	}
	return svg, mathml, scripted, nil
}

func xhtmlSourceIsUTF8(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		data = data[3:]
	}
	i := 0
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\r' || data[i] == '\n') {
		i++
	}
	const xmlTarget = "<?xml"
	if len(data[i:]) < len(xmlTarget) || !strings.EqualFold(string(data[i:i+len(xmlTarget)]), xmlTarget) {
		return true
	}
	if i+len(xmlTarget) < len(data) {
		next := data[i+len(xmlTarget)]
		if next != ' ' && next != '\t' && next != '\r' && next != '\n' && next != '?' {
			return true // e.g. an xml-stylesheet processing instruction
		}
	}
	end := bytes.Index(data[i:], []byte("?>"))
	if end < 0 {
		return false
	}
	decl := data[i : i+end+2]
	encoding := xmlEncodingRe.FindSubmatch(decl)
	if encoding == nil {
		return true
	}
	return strings.EqualFold(string(encoding[1]), "utf-8") || strings.EqualFold(string(encoding[1]), "utf8")
}

// normalizeDuokanNotes 复刻 core.normalize_duokan_notes，但把 class 改名限制在
// **真实标签字节内**（xhtml.ScanRegions 的 RegionTag）。
//
// Python 版对全文做 re.subn，而 `class="duokan-footnote"` 这个针不含 `<` / `>`：
// 一本讲 EPUB 制作的书在正文里原样写出这段 class（`&lt;li class="duokan-…"&gt;`
// 只转义了尖括号，属性部分是裸字节）就会被当成标记改掉。样本书
// 《EPub指南》的 Chapter12-2 / Chapter8-6 正是如此，实跑会触发 8 条
// error redline.text —— 产物虽保留供人工 review，但这条能力在这本书上不可用。
// 交接文档 §10 把它记为"红线能拦住但尚未修"，这里按 structure_normalize 已经
// 验证过的区域化方案修掉。
//
// aside 与 class 只在实际标签边界上编辑。第三个返回值是告警：
// 区域扫描在无法闭合的结构处截断时，其后的标签不再改写，不能静默半改。
func normalizeDuokanNotes(text string) (string, int, []string, error) {
	if !strings.Contains(text, "duokan-footnote") &&
		!(strings.Contains(text, "epub:type") && strings.Contains(text, "footnote")) {
		return text, 0, nil, nil
	}
	// class 改名表按 Python 的先后顺序应用 —— `duokan-footnote-content` 与
	// `duokan-footnote-item` 必须先于 `duokan-footnote`，否则后者会先把前两个
	// 的前缀吃掉。所有替换仅在扫描到的真实标签字节内执行。
	rewrites := [...][2]string{
		{`class="duokan-footnote-content"`, `class="footnote-list"`},
		{`class="duokan-footnote-item"`, `class="footnote-item"`},
		{`class="duokan-footnote"`, `class="noteref-icon"`},
	}
	renamed, warnings, err := rewriteInTags(text, func(tag string) (string, int, error) {
		total := 0
		var n int
		var roleErr error
		tag, n, roleErr = normalizeDuokanAsideRole(tag)
		if roleErr != nil {
			return "", 0, roleErr
		}
		total += n
		for _, rw := range rewrites {
			out, k := subLiteral(tag, rw[0], rw[1])
			tag = out
			total += k
		}
		return tag, total, nil
	})
	if err != nil {
		return "", 0, warnings, err
	}
	return renamed.text, renamed.count, warnings, nil
}

func normalizeDuokanAsideRole(tag string) (string, int, error) {
	name, _, closing := xhtml.TagParts(tag)
	if closing || name != "aside" {
		return tag, 0, nil
	}
	attrs, ok := xhtml.TagAttrs(tag)
	if !ok {
		return tag, 0, nil
	}
	footnote, role := false, false
	for _, attr := range attrs {
		switch attr.Raw {
		case "epub:type":
			footnote = attr.Value == "footnote"
		case "role":
			role = true
		}
	}
	if !footnote || role {
		return tag, 0, nil
	}
	selfClose := len(tag) >= 2 && tag[len(tag)-2] == '/'
	tagInfo := xhtml.Tag{
		Span:      xhtml.Span{Start: 0, End: len(tag)},
		Name:      name,
		Attrs:     attrs,
		SelfClose: selfClose,
	}
	edit := xhtmlInsertAttributes("xhtml", tag, tagInfo, `role="doc-footnote"`)
	updated, _, err := applyXHTMLEdits(tag, []editset.Edit{edit})
	if err != nil {
		return "", 0, err
	}
	return updated, 1, nil
}

// tagRewriteResult 是 rewriteInTags 的产物：改写后的文本与命中计数。
type tagRewriteResult struct {
	text  string
	count int
}

// rewriteInTags 对文档里每个真实标签的字节区间调用 fn，其余字节（正文字符
// 数据、注释、CDATA、处理指令、DOCTYPE、<script>/<style> 内容）原样透传。
// 区域扫描截断时返回一条告警，截断点之后的标签保持不变。
func rewriteInTags(text string, fn func(tag string) (string, int, error)) (tagRewriteResult, []string, error) {
	regions, stop := xhtml.ScanRegions(text)
	var warnings []string
	if stop != xhtml.ScanComplete {
		warnings = append(warnings, fmt.Sprintf(
			"markup scan stopped at byte offset %d (unterminated comment/CDATA/PI/declaration/tag or unclosed style/script); Duokan note markup after this offset left unchanged", stop))
	}
	if len(regions) == 0 {
		return tagRewriteResult{text: text}, warnings, nil
	}
	var edits []editset.Edit
	total := 0
	for _, r := range regions {
		if r.Kind != xhtml.RegionTag {
			continue
		}
		segment, n, err := fn(text[r.Start:r.End])
		if err != nil {
			return tagRewriteResult{text: text}, warnings, err
		}
		if n > 0 {
			edits = append(edits, editset.Replace("xhtml", int64(r.Start), int64(r.End-r.Start), []byte(segment)))
		}
		total += n
	}
	updated, _, err := applyXHTMLEdits(text, edits)
	if err != nil {
		return tagRewriteResult{text: text}, warnings, err
	}
	return tagRewriteResult{text: updated, count: total}, warnings, nil
}

// subLiteral 复刻 re.subn 对无元字符字面量的替换计数语义。
func subLiteral(text, old, new string) (string, int) {
	n := strings.Count(text, old)
	return strings.ReplaceAll(text, old, new), n
}

func sortStrings(list []string) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j] < list[j-1]; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}
