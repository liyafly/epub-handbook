// xhtml.go 实现 EPUB3 迁移的 XHTML 页面处理：shell 属性与插入使用字节范围
// 编辑，弹注转换保留其能力专属结构逻辑；不对整页做 parse/serialize 往返。
package migrateepub3

import (
	"bytes"
	"fmt"
	"html"
	"strings"
	"unicode/utf8"

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
func updateXHTMLFiles(files *workFiles, root *xmlElem, opfPath, styleZip, noteZip string, rep *conversionReport, popupNotes, typography bool) (bool, error) {
	opfDir := pyDirname(opfPath)
	_, byZip := manifestMaps(root, opfDir)
	keys := make([]string, 0, len(byZip))
	for k := range byZip {
		keys = append(keys, k)
	}
	sortStrings(keys)
	defaultNoteIconUsed := false
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
			return false, convErrf("%s: lossless XHTML migration requires UTF-8 source bytes", zipPath)
		}
		text := utf8ReplaceDecode(original)
		text, changed, err := normalizeXHTMLShell(text, defaultLanguage)
		if err != nil {
			return false, convErrf("%s: cannot normalize XHTML shell: %v", zipPath, err)
		}
		if typography {
			styleHref := relHref(zipPath, styleZip)
			var linked bool
			text, linked, err = ensureStylesheetLink(text, styleHref)
			if err != nil {
				return false, convErrf("%s: cannot add stylesheet link: %v", zipPath, err)
			}
			if linked {
				rep.StylesheetLinksAdded++
				changed = true
			}
		}
		if popupNotes {
			noteHref := relHref(zipPath, noteZip)
			var notes, markerReplacements int
			var conversionErr error
			// 不能用 := —— 否则 text 成为块内新变量，转换结果丢失。
			text, notes, markerReplacements, conversionErr = convertPlainNotes(text, noteHref)
			if conversionErr != nil {
				return false, convErrf("%s: cannot convert plain notes: %v", zipPath, conversionErr)
			}
			if notes == 0 {
				text, notes, markerReplacements, conversionErr = convertSigilLegacyNotes(text, noteHref)
				if conversionErr != nil {
					return false, convErrf("%s: cannot convert Sigil notes: %v", zipPath, conversionErr)
				}
			}
			if notes > 0 {
				rep.PlainNotesConverted += notes
				changed = true
			}
			if markerReplacements > 0 {
				defaultNoteIconUsed = true
			}
			var normalized int
			var duokanWarnings []string
			text, normalized, duokanWarnings, conversionErr = normalizeDuokanNotes(text)
			if conversionErr != nil {
				return false, convErrf("%s: cannot normalize Duokan notes: %v", zipPath, conversionErr)
			}
			for _, w := range duokanWarnings {
				rep.Warnings = append(rep.Warnings, zipPath+": "+w)
			}
			if normalized > 0 {
				rep.DuokanNotesNormalized += normalized
				changed = true
			}
		}
		if pyPatterns["svgCheck"].hasMatch(text) && addProps(item, "svg") {
			rep.ManifestItemsUpdated++
		}
		if pyPatterns["mathCheck"].hasMatch(text) && addProps(item, "mathml") {
			rep.ManifestItemsUpdated++
		}
		if pyPatterns["scriptCheck"].hasMatch(text) && addProps(item, "scripted") {
			rep.ManifestItemsUpdated++
		}
		if changed {
			files.write(zipPath, []byte(text))
			rep.XHTMLFilesUpdated++
		}
	}
	return defaultNoteIconUsed, nil
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

// ---- 弹注迁移（scripts/epub3_conversion/notes.py → core） ----

// convertPlainNotes 逐行复刻 core.convert_plain_notes，同时只替换真实 <p>
// 注释区块与真实 noteref 标签序列。
func convertPlainNotes(text, noteHref string) (string, int, int, error) {
	regions, stop := xhtml.ScanRegions(text)
	if stop != xhtml.ScanComplete {
		return "", 0, 0, fmt.Errorf("markup scan stopped at byte offset %d", stop)
	}
	matches := pyPatterns["plainNote"].findAll(text)
	validNotes := make([]*pyMatch, 0, len(matches))
	for _, match := range matches {
		if matchHasRealElementBoundaries(text, regions, match.byteStart(0), match.byteEnd(0), "p", "p") {
			validNotes = append(validNotes, match)
		}
	}
	if len(validNotes) == 0 {
		return text, 0, 0, nil
	}
	for i := 0; i+1 < len(validNotes); i++ {
		gap := text[validNotes[i].byteEnd(0):validNotes[i+1].byteStart(0)]
		if strings.TrimSpace(gap) != "" {
			return text, 0, 0, nil
		}
	}
	noteIDs := map[string]bool{}
	for _, match := range validNotes {
		noteIDs[match.groupName("num")] = true
	}
	first, last := validNotes[0], validNotes[len(validNotes)-1]
	replaceStart, replaceEnd := first.byteStart(0), last.byteEnd(0)
	if prefix := text[:replaceStart]; len(prefix) > 0 {
		if hr, ok := pyPatterns["hrBeforeNotes"].search(prefix); ok &&
			matchHasRealSingleElement(text, regions, hr.byteStart(0), hr.byteEnd(0), "hr") {
			replaceStart = hr.byteStart(0)
		}
	}

	lines := []string{
		`  <aside epub:type="footnote" role="doc-footnote">`,
		`    <div><hr class="footnote-line xian"/></div>`,
		`    <ol class="footnote-list">`,
	}
	for _, match := range validNotes {
		num := match.groupName("num")
		body := pyStrip(match.groupName("body"))
		lines = append(lines,
			`      <li class="footnote-item" id="m`+num+`">`,
			`        <p class="footnote"><a class="footnote-back" epub:type="backlink" role="doc-backlink" href="#w`+num+`">◎</a>`+body+`</p>`,
			`      </li>`,
		)
	}
	lines = append(lines, `    </ol>`, `  </aside>`)
	edits := []editset.Edit{
		editset.Replace("xhtml", int64(replaceStart), int64(replaceEnd-replaceStart), []byte("\n"+strings.Join(lines, "\n"))),
	}
	markerReplacements := 0
	for _, match := range pyPatterns["plainNoteref"].findAll(text) {
		start, end := match.byteStart(0), match.byteEnd(0)
		if start < replaceEnd && end > replaceStart || !matchHasRealElementBoundaries(text, regions, start, end, "a", "a") {
			continue
		}
		num := match.groupName("num")
		if !noteIDs[num] {
			continue
		}
		replacement := `<sup class="note-marker"><a id="w` + num + `" class="noteref-icon" epub:type="noteref" ` +
			`role="doc-noteref" href="#m` + num + `"><img alt="注" src="` + escapeXHTMLAttribute(noteHref, '"') + `"/></a></sup>`
		edits = append(edits, editset.Replace("xhtml", int64(start), int64(end-start), []byte(replacement)))
		markerReplacements++
	}
	rebuilt, _, err := applyXHTMLEdits(text, edits)
	if err != nil {
		return "", 0, 0, err
	}
	rebuilt, _, err = markNoteMarkerSup(rebuilt)
	if err != nil {
		return "", 0, 0, err
	}
	return rebuilt, len(validNotes), markerReplacements, nil
}

// convertSigilLegacyNotes 逐行复刻 core.convert_sigil_legacy_notes，只修改
// 真实 section/aside 边界和对应的真实 noteref。
func convertSigilLegacyNotes(text, noteHref string) (string, int, int, error) {
	regions, stop := xhtml.ScanRegions(text)
	if stop != xhtml.ScanComplete {
		return "", 0, 0, fmt.Errorf("markup scan stopped at byte offset %d", stop)
	}
	var edits []editset.Edit
	convertedIDs := map[string]bool{}
	convertedCount := 0
	var convertedSections [][2]int
	for _, section := range pyPatterns["sigilSection"].findAll(text) {
		sectionStart, sectionEnd := section.byteStart(0), section.byteEnd(0)
		if !matchHasRealElementBoundaries(text, regions, sectionStart, sectionEnd, "section", "section") {
			continue
		}
		bodyStart, bodyEnd, ok := pyMatchGroupByteSpan(section, "body")
		if !ok || bodyStart < sectionStart || bodyEnd > sectionEnd {
			continue
		}
		body := text[bodyStart:bodyEnd]
		var notes []*pyMatch
		for _, note := range pyPatterns["sigilNote"].findAll(body) {
			if matchHasRealElementBoundaries(text, regions, bodyStart+note.byteStart(0), bodyStart+note.byteEnd(0), "aside", "aside") {
				notes = append(notes, note)
			}
		}
		if len(notes) == 0 {
			continue
		}
		var residual strings.Builder
		last := 0
		for _, note := range notes {
			start, end := note.byteStart(0), note.byteEnd(0)
			residual.WriteString(body[last:start])
			last = end
		}
		residual.WriteString(body[last:])
		if pyStrip(residual.String()) != "" {
			continue
		}

		lines := []string{
			`  <aside epub:type="footnote" role="doc-footnote">`,
			`    <div><hr class="footnote-line xian"/></div>`,
			`    <ol class="footnote-list">`,
		}
		for _, note := range notes {
			number := note.groupName("num")
			convertedIDs[number] = true
			convertedCount++
			noteBody := pyStrip(note.groupName("body"))
			lines = append(lines,
				`      <li class="footnote-item" id="footnote_`+number+`">`,
				`        <p class="footnote"><a class="footnote-back" epub:type="backlink" role="doc-backlink" href="#noteref_`+number+`">◎</a>`+noteBody+`</p>`,
				`      </li>`,
			)
		}
		lines = append(lines, `    </ol>`, `  </aside>`)
		edits = append(edits, editset.Replace("xhtml", int64(sectionStart), int64(sectionEnd-sectionStart), []byte(strings.Join(lines, "\n"))))
		convertedSections = append(convertedSections, [2]int{sectionStart, sectionEnd})
	}
	if convertedCount == 0 {
		return text, 0, 0, nil
	}

	markerReplacements := 0
	for _, match := range pyPatterns["sigilNoteref"].findAll(text) {
		start, end := match.byteStart(0), match.byteEnd(0)
		insideConvertedSection := false
		for _, section := range convertedSections {
			if start < section[1] && end > section[0] {
				insideConvertedSection = true
				break
			}
		}
		if insideConvertedSection || !matchHasRealElementBoundaries(text, regions, start, end, "a", "a") {
			continue
		}
		number := match.groupName("num")
		if !convertedIDs[number] {
			continue
		}
		replacement := `<a id="noteref_` + number + `" class="noteref-icon" epub:type="noteref" ` +
			`role="doc-noteref" href="#footnote_` + number + `"><img alt="注" src="` + escapeXHTMLAttribute(noteHref, '"') + `"/></a>`
		edits = append(edits, editset.Replace("xhtml", int64(start), int64(end-start), []byte(replacement)))
		markerReplacements++
	}
	rebuilt, _, err := applyXHTMLEdits(text, edits)
	if err != nil {
		return "", 0, 0, err
	}
	rebuilt, _, err = markNoteMarkerSup(rebuilt)
	if err != nil {
		return "", 0, 0, err
	}
	return rebuilt, convertedCount, markerReplacements, nil
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
// aside、class 与 marker glyph 都只在实际标签边界上编辑。第三个返回值是告警：
// 区域扫描在无法闭合的结构处截断时，其后的标签不再改写，不能静默半改。
func normalizeDuokanNotes(text string) (string, int, []string, error) {
	if !strings.Contains(text, "duokan-footnote") && !strings.Contains(text, `epub:type="footnote"`) {
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
	renamed, warnings, err := rewriteInTags(text, func(tag string) (string, int) {
		total := 0
		var n int
		tag, n = pyPatterns["duokanAside"].subTemplate(tag, `<aside epub:type="footnote" role="doc-footnote"`, 0)
		total += n
		for _, rw := range rewrites {
			out, k := subLiteral(tag, rw[0], rw[1])
			tag = out
			total += k
		}
		return tag, total
	})
	if err != nil {
		return "", 0, warnings, err
	}
	updated, glyphCount, warning, err := rewriteDuokanMarkerGlyph(renamed.text)
	if err != nil {
		return "", 0, warnings, err
	}
	if warning != "" && len(warnings) == 0 {
		warnings = append(warnings, warning)
	}
	return updated, renamed.count + glyphCount, warnings, nil
}

// tagRewriteResult 是 rewriteInTags 的产物：改写后的文本与命中计数。
type tagRewriteResult struct {
	text  string
	count int
}

// rewriteInTags 对文档里每个真实标签的字节区间调用 fn，其余字节（正文字符
// 数据、注释、CDATA、处理指令、DOCTYPE、<script>/<style> 内容）原样透传。
// 区域扫描截断时返回一条告警，截断点之后的标签保持不变。
func rewriteInTags(text string, fn func(tag string) (string, int)) (tagRewriteResult, []string, error) {
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
		segment, n := fn(text[r.Start:r.End])
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

// markNoteMarkerSup 复刻 core.mark_note_marker_sup，但只改真实 sup 开标签的 class。
func markNoteMarkerSup(text string) (string, bool, error) {
	regions, stop := xhtml.ScanRegions(text)
	if stop != xhtml.ScanComplete {
		return "", false, fmt.Errorf("markup scan stopped at byte offset %d", stop)
	}
	var edits []editset.Edit
	for _, match := range pyPatterns["noteMarkerSup"].findAll(text) {
		start, end := match.byteStart(0), match.byteEnd(0)
		if !matchHasRealElementBoundaries(text, regions, start, end, "sup", "sup") {
			continue
		}
		var openTag *xhtml.Tag
		for _, region := range regions {
			if region.Kind != xhtml.RegionTag || region.Span.Start != start {
				continue
			}
			tag, closing, ok := xhtmlRegionTag(text, region)
			if ok && !closing && sameLocalName(tag.Name, "sup") {
				copy := tag
				openTag = &copy
			}
			break
		}
		if openTag == nil {
			continue
		}
		attrs, ok := xhtmlAttributes(text, *openTag)
		if !ok {
			return "", false, fmt.Errorf("malformed note marker sup at byte %d", start)
		}
		classes := matchingXHTMLAttrs(attrs, "class")
		if len(classes) > 1 {
			return "", false, fmt.Errorf("ambiguous class attributes in note marker sup at byte %d", start)
		}
		var values []string
		if len(classes) == 1 {
			values = pySplitWS(html.UnescapeString(classes[0].Value))
		}
		if !containsString(values, "note-marker") {
			values = append(values, "note-marker")
		}
		attributeEdits, err := xhtmlAttributeEdits("xhtml", text, *openTag,
			map[string]string{"class": strings.Join(values, " ")}, nil)
		if err != nil {
			return "", false, err
		}
		edits = append(edits, attributeEdits...)
	}
	return applyXHTMLEdits(text, edits)
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
