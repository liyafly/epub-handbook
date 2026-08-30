// xhtml.go 复刻 scripts/epub3_conversion/core.py 的每页 XHTML 管线：
// normalize_xhtml_shell、format_xhtml_multiline（element-only 缩进）、
// ensure_stylesheet_link、三类弹注迁移（plain / Sigil / Duokan）与
// note-marker 标记。全部正则经 pyregex.go 按 Python re 语义执行。
package migrateepub3

import (
	"strings"
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

// normalizeXHTMLShell 逐行复刻 core.normalize_xhtml_shell。
func normalizeXHTMLShell(text, defaultLanguage string) (string, bool) {
	changed := false
	if _, ok := pyPatterns["doctype"].search(text); ok {
		text, _ = pyPatterns["doctype"].subTemplate(text, "", 1)
		changed = true
	}
	declaration := ""
	if m, ok := pyPatterns["xmlDecl"].search(text); ok {
		declaration = pyStrip(m.groupI(0)) + "\n"
		end := m.byteEnd(0)
		if end < 0 || end > len(text) {
			end = 0
		}
		text = pyStripLeft(text[end:])
	}
	if !strings.HasPrefix(strings.ToLower(pyStripLeft(text)), "<!doctype html>") {
		text = declaration + "<!DOCTYPE html>\n" + pyStripLeft(text)
		changed = true
	} else if declaration != "" {
		text = declaration + pyStripLeft(text)
	}

	if m, ok := pyPatterns["htmlTag"].search(text); ok {
		attrs := m.groupI(1)
		if !strings.Contains(attrs, "xmlns:epub") {
			attrs += ` xmlns:epub="` + opsURI + `"`
			changed = true
		}
		langMatch, hasLang := pyPatterns["langAttr"].search(attrs)
		xmlLangMatch, hasXMLLang := pyPatterns["xmlLangAttr"].search(attrs)
		language := ""
		if defaultLanguage != "" {
			language = pyStrip(defaultLanguage)
		}
		if !hasLang && (hasXMLLang || language != "") {
			value := language
			if hasXMLLang {
				value = xmlLangMatch.groupI(2)
			}
			attrs += ` lang="` + saxEscapeAttr(value) + `"`
			changed = true
		}
		if !hasXMLLang && (hasLang || language != "") {
			value := language
			if hasLang {
				value = langMatch.groupI(2)
			}
			attrs += ` xml:lang="` + saxEscapeAttr(value) + `"`
			changed = true
		}
		text = text[:m.byteStart(0)] + "<html" + attrs + ">" + text[m.byteEnd(0):]
	}

	if _, ok := pyPatterns["metaHTTP"].search(text); ok {
		text, _ = pyPatterns["metaHTTP"].subTemplate(text, `<meta charset="utf-8"/>`, 1)
		changed = true
	} else if !strings.Contains(strings.ToLower(text), "<meta charset=") {
		if _, ok := pyPatterns["headEnd"].search(text); ok {
			text, _ = pyPatterns["headEnd"].subTemplate(text, "  <meta charset=\"utf-8\"/>\n</head>", 1)
			changed = true
		}
	}
	if strings.Contains(strings.ToLower(text), "<big") {
		text, _ = pyPatterns["bigOpen"].subTemplate(text, `<span\1 class="big">`, 0)
		text, _ = pyPatterns["bigClose"].subTemplate(text, `</span>`, 0)
		changed = true
	}
	return text, changed
}

// formatXHTMLMultiline 逐行复刻 core.format_xhtml_multiline：
// 解析失败（无效 XML）时原样放行。
func formatXHTMLMultiline(text string) (string, bool) {
	stripped, _ := pyPatterns["xmlDecl"].subTemplate(text, "", 1)
	stripped, _ = pyPatterns["doctype"].subTemplate(stripped, "", 1)
	stripped = pyStrip(stripped)
	root, err := parseXMLTree([]byte(stripped))
	if err != nil {
		return text, false
	}
	indentElementOnly(root, 0)
	formatted := "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<!DOCTYPE html>\n" +
		string(serializeTree(root, namespacePrefixesXHTML, false)) + "\n"
	return formatted, formatted != text
}

// hasMixedTextContent 逐行复刻 core.has_mixed_text_content。
func hasMixedTextContent(e *xmlElem) bool {
	if pyStrip(e.text) != "" {
		return true
	}
	for _, child := range e.children {
		if pyStrip(child.tail) != "" || inlineContentTags[child.name] {
			return true
		}
	}
	return false
}

// indentElementOnly 逐行复刻 core.indent_element_only。
func indentElementOnly(e *xmlElem, level int) {
	children := e.children
	if len(children) == 0 || hasMixedTextContent(e) {
		return
	}
	childIndent := "\n" + strings.Repeat("  ", level+1)
	parentIndent := "\n" + strings.Repeat("  ", level)
	e.text = childIndent
	for _, child := range children {
		indentElementOnly(child, level+1)
		child.tail = childIndent
	}
	children[len(children)-1].tail = parentIndent
}

// ensureStylesheetLink 逐行复刻 epub_lib.ensure_stylesheet_link。
func ensureStylesheetLink(text, href string) (string, bool) {
	if strings.Contains(text, href) {
		return text, false
	}
	link := `  <link href="` + href + `" type="text/css" rel="stylesheet"/>` + "\n"
	updated, n := pyPatterns["headEnd"].subTemplate(text, link+"</head>", 1)
	return updated, n > 0
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
		text := utf8ReplaceDecode(original)
		text, changed := normalizeXHTMLShell(text, defaultLanguage)
		if typography {
			styleHref := relHref(zipPath, styleZip)
			var linked bool
			text, linked = ensureStylesheetLink(text, styleHref)
			if linked {
				rep.StylesheetLinksAdded++
				changed = true
			}
		}
		if popupNotes {
			noteHref := relHref(zipPath, noteZip)
			var notes, markerReplacements int
			// 不能用 := —— 否则 text 成为块内新变量，转换结果丢失。
			text, notes, markerReplacements = convertPlainNotes(text, noteHref)
			if notes == 0 {
				text, notes, markerReplacements = convertSigilLegacyNotes(text, noteHref)
			}
			if notes > 0 {
				rep.PlainNotesConverted += notes
				changed = true
			}
			if markerReplacements > 0 {
				defaultNoteIconUsed = true
			}
			var normalized int
			text, normalized = normalizeDuokanNotes(text)
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
		text, reformatted := formatXHTMLMultiline(text)
		changed = changed || reformatted
		if changed {
			files.write(zipPath, []byte(text))
			rep.XHTMLFilesUpdated++
		}
	}
	return defaultNoteIconUsed, nil
}

// ---- 弹注迁移（scripts/epub3_conversion/notes.py → core） ----

// convertPlainNotes 逐行复刻 core.convert_plain_notes。
func convertPlainNotes(text, noteHref string) (string, int, int) {
	matches := pyPatterns["plainNote"].findAll(text)
	if len(matches) == 0 {
		return text, 0, 0
	}
	noteIDs := map[string]bool{}
	for _, m := range matches {
		noteIDs[m.groupName("num")] = true
	}
	markerReplacements := 0

	first := matches[0]
	last := matches[len(matches)-1]
	prefix := text[:first.byteStart(0)]
	suffix := text[last.byteEnd(0):]
	if hr, ok := pyPatterns["hrBeforeNotes"].search(prefix); ok {
		prefix = prefix[:hr.byteStart(0)]
	}

	lines := []string{
		`  <aside epub:type="footnote" role="doc-footnote">`,
		`    <div><hr class="footnote-line xian"/></div>`,
		`    <ol class="footnote-list">`,
	}
	for _, m := range matches {
		num := m.groupName("num")
		body := pyStrip(m.groupName("body"))
		lines = append(lines,
			`      <li class="footnote-item" id="m`+num+`">`,
			`        <p class="footnote"><a class="footnote-back" epub:type="backlink" role="doc-backlink" href="#w`+num+`">◎</a>`+body+`</p>`,
			`      </li>`,
		)
	}
	lines = append(lines, `    </ol>`, `  </aside>`)
	rebuilt := prefix + "\n" + strings.Join(lines, "\n") + suffix
	rebuilt, _ = pyPatterns["plainNoteref"].subFunc(rebuilt, 0, func(m *pyMatch) string {
		num := m.groupName("num")
		if !noteIDs[num] {
			return m.groupI(0)
		}
		markerReplacements++
		return `<sup class="note-marker"><a id="w` + num + `" class="noteref-icon" epub:type="noteref" ` +
			`role="doc-noteref" href="#m` + num + `"><img alt="注" src="` + noteHref + `"/></a></sup>`
	})
	rebuilt = markNoteMarkerSup(rebuilt)
	return rebuilt, len(matches), markerReplacements
}

// convertSigilLegacyNotes 逐行复刻 core.convert_sigil_legacy_notes。
func convertSigilLegacyNotes(text, noteHref string) (string, int, int) {
	convertedIDs := map[string]bool{}
	convertedCount := 0

	rebuilt, _ := pyPatterns["sigilSection"].subFunc(text, 0, func(m *pyMatch) string {
		body := m.groupName("body")
		notes := pyPatterns["sigilNote"].findAll(body)
		if len(notes) == 0 {
			return m.groupI(0)
		}
		residual, _ := pyPatterns["sigilNote"].subTemplate(body, "", 0)
		if pyStrip(residual) != "" {
			return m.groupI(0)
		}
		convertedCount += len(notes)
		lines := []string{
			`  <aside epub:type="footnote" role="doc-footnote">`,
			`    <div><hr class="footnote-line xian"/></div>`,
			`    <ol class="footnote-list">`,
		}
		for _, note := range notes {
			number := note.groupName("num")
			convertedIDs[number] = true
			noteBody := pyStrip(note.groupName("body"))
			lines = append(lines,
				`      <li class="footnote-item" id="footnote_`+number+`">`,
				`        <p class="footnote"><a class="footnote-back" epub:type="backlink" role="doc-backlink" href="#noteref_`+number+`">◎</a>`+noteBody+`</p>`,
				`      </li>`,
			)
		}
		lines = append(lines, `    </ol>`, `  </aside>`)
		return strings.Join(lines, "\n")
	})
	if len(convertedIDs) == 0 {
		return text, 0, 0
	}

	markerReplacements := 0
	rebuilt, _ = pyPatterns["sigilNoteref"].subFunc(rebuilt, 0, func(m *pyMatch) string {
		number := m.groupName("num")
		if !convertedIDs[number] {
			return m.groupI(0)
		}
		markerReplacements++
		return `<a id="noteref_` + number + `" class="noteref-icon" epub:type="noteref" ` +
			`role="doc-noteref" href="#footnote_` + number + `"><img alt="注" src="` + noteHref + `"/></a>`
	})
	rebuilt = markNoteMarkerSup(rebuilt)
	return rebuilt, convertedCount, markerReplacements
}

// normalizeDuokanNotes 逐行复刻 core.normalize_duokan_notes。
func normalizeDuokanNotes(text string) (string, int) {
	if !strings.Contains(text, "duokan-footnote") && !strings.Contains(text, `epub:type="footnote"`) {
		return text, 0
	}
	count := 0
	updated := text
	var n int
	updated, n = pyPatterns["duokanAside"].subTemplate(updated, `<aside epub:type="footnote" role="doc-footnote"`, 0)
	count += n
	updated, n = subLiteral(updated, `class="duokan-footnote-content"`, `class="footnote-list"`)
	count += n
	updated, n = subLiteral(updated, `class="duokan-footnote-item"`, `class="footnote-item"`)
	count += n
	updated, n = subLiteral(updated, `class="duokan-footnote"`, `class="noteref-icon"`)
	count += n
	updated, n = subLiteral(updated, `>⊙</a>`, `>◎</a>`)
	count += n
	return updated, count
}

// markNoteMarkerSup 逐行复刻 core.mark_note_marker_sup。
func markNoteMarkerSup(text string) string {
	out, _ := pyPatterns["noteMarkerSup"].subFunc(text, 0, func(m *pyMatch) string {
		attrs := ""
		if m.hasGroup("attrs") {
			attrs = m.groupName("attrs")
		}
		if classMatch, ok := pyPatterns["classAttr"].search(attrs); ok {
			classes := pySplitWS(classMatch.groupName("value"))
			if !containsString(classes, "note-marker") {
				classes = append(classes, "note-marker")
			}
			quote := classMatch.groupName("quote")
			replacement := "class=" + quote + strings.Join(classes, " ") + quote
			attrs = attrs[:classMatch.byteStart(0)] + replacement + attrs[classMatch.byteEnd(0):]
		} else {
			attrs += ` class="note-marker"`
		}
		return "<sup" + attrs + ">" + m.groupName("content") + "</sup>"
	})
	return out
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
