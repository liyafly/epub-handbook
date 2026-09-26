package migrateepub3

import (
	"fmt"
	"html"
	"strings"

	"github.com/liyafly/epub-handbook/internal/editset"
	xhtmlscan "github.com/liyafly/epub-handbook/internal/scan/xhtml"
)

func normalizeXHTMLShell(text, defaultLanguage string) (string, bool, error) {
	regions, truncated := xhtmlscan.ScanRegions(text)
	if truncated != xhtmlscan.ScanComplete {
		return "", false, fmt.Errorf("XHTML markup scan stopped at byte %d", truncated)
	}
	var edits []editset.Edit
	rootTag, headOpen, headClose := xhtmlShellTags(text, regions)
	if rootTag == nil {
		return text, false, nil
	}

	doctype, hasDoctype, err := xhtmlDoctypeSpan(text)
	if err != nil {
		return "", false, err
	}
	const htmlDoctype = "<!DOCTYPE html>"
	if hasDoctype {
		if string(text[doctype.Start:doctype.End]) != htmlDoctype {
			edits = append(edits, editset.Replace("xhtml", int64(doctype.Start), int64(doctype.End-doctype.Start), []byte(htmlDoctype)))
		}
	} else {
		edits = append(edits, editset.Insert("xhtml", int64(rootTag.Span.Start), []byte(htmlDoctype)))
	}

	rootAttrs, ok := xhtmlAttributes(text, *rootTag)
	if !ok {
		return "", false, fmt.Errorf("malformed XHTML root tag at byte %d", rootTag.Span.Start)
	}
	langAttrs := matchingXHTMLAttrs(rootAttrs, "lang")
	xmlLangAttrs := matchingXHTMLAttrs(rootAttrs, "xml:lang")
	if len(langAttrs) > 1 || len(xmlLangAttrs) > 1 {
		return "", false, fmt.Errorf("ambiguous language attributes on XHTML root at byte %d", rootTag.Span.Start)
	}
	var addRoot []string
	if len(matchingXHTMLAttrs(rootAttrs, "xmlns:epub")) == 0 {
		addRoot = append(addRoot, `xmlns:epub="`+escapeXHTMLAttribute(opsURI, '"')+`"`)
	}
	language := pyStrip(defaultLanguage)
	if len(langAttrs) == 0 && (len(xmlLangAttrs) > 0 || language != "") {
		value := language
		if len(xmlLangAttrs) > 0 {
			value = html.UnescapeString(xmlLangAttrs[0].Value)
		}
		addRoot = append(addRoot, `lang="`+escapeXHTMLAttribute(value, '"')+`"`)
	}
	if len(xmlLangAttrs) == 0 && (len(langAttrs) > 0 || language != "") {
		value := language
		if len(langAttrs) > 0 {
			value = html.UnescapeString(langAttrs[0].Value)
		}
		addRoot = append(addRoot, `xml:lang="`+escapeXHTMLAttribute(value, '"')+`"`)
	}
	if len(addRoot) > 0 {
		edits = append(edits, xhtmlInsertAttributes("xhtml", text, *rootTag, strings.Join(addRoot, " ")))
	}

	var headInsertions []string
	if headOpen != nil && headClose != nil {
		var firstHTTPMeta *xhtmlscan.Tag
		hasCharset := false
		for _, region := range regions {
			if region.Kind != xhtmlscan.RegionTag || region.Span.Start < headOpen.Span.End || region.Span.End > headClose.Span.Start {
				continue
			}
			tag, closing, valid := xhtmlRegionTag(text, region)
			if !valid || closing || !sameLocalName(tag.Name, "meta") {
				continue
			}
			attrs, valid := xhtmlAttributes(text, tag)
			if !valid {
				return "", false, fmt.Errorf("malformed XHTML meta tag at byte %d", tag.Span.Start)
			}
			charsetAttrs := matchingXHTMLAttrs(attrs, "charset")
			if len(charsetAttrs) > 1 {
				return "", false, fmt.Errorf("ambiguous charset attributes in XHTML meta at byte %d", tag.Span.Start)
			}
			if len(charsetAttrs) > 0 {
				hasCharset = true
			}
			httpAttrs := matchingXHTMLAttrs(attrs, "http-equiv")
			contentAttrs := matchingXHTMLAttrs(attrs, "content")
			if len(httpAttrs) > 1 || len(contentAttrs) > 1 {
				return "", false, fmt.Errorf("ambiguous Content-Type metadata in XHTML head at byte %d", tag.Span.Start)
			}
			if firstHTTPMeta == nil && len(httpAttrs) == 1 && len(contentAttrs) == 1 &&
				strings.EqualFold(html.UnescapeString(httpAttrs[0].Value), "Content-Type") &&
				strings.Contains(strings.ToLower(html.UnescapeString(contentAttrs[0].Value)), "charset=utf-8") {
				copy := tag
				firstHTTPMeta = &copy
			}
		}
		if firstHTTPMeta != nil {
			setAttrs := map[string]string{"charset": "utf-8"}
			removeAttrs := map[string]bool{"http-equiv": true, "content": true}
			metaEdits, err := xhtmlAttributeEdits("xhtml", text, *firstHTTPMeta, setAttrs, removeAttrs)
			if err != nil {
				return "", false, err
			}
			edits = append(edits, metaEdits...)
			hasCharset = true
		}
		if !hasCharset {
			headInsertions = append(headInsertions, `<meta charset="utf-8"/>`)
		}
	}
	if len(headInsertions) > 0 && headClose != nil {
		edits = append(edits, editset.Insert("xhtml", int64(headClose.Span.Start), []byte(strings.Join(headInsertions, ""))))
	}

	bigEdits, err := xhtmlBigTagEdits(text, regions)
	if err != nil {
		return "", false, err
	}
	edits = append(edits, bigEdits...)
	return applyXHTMLEdits(text, edits)
}

func ensureStylesheetLink(text, href string) (string, bool, error) {
	regions, truncated := xhtmlscan.ScanRegions(text)
	if truncated != xhtmlscan.ScanComplete {
		return "", false, fmt.Errorf("XHTML markup scan stopped at byte %d", truncated)
	}
	_, headOpen, headClose := xhtmlShellTags(text, regions)
	if headOpen == nil || headClose == nil {
		return text, false, nil
	}
	for _, region := range regions {
		if region.Kind != xhtmlscan.RegionTag || region.Span.Start < headOpen.Span.End || region.Span.End > headClose.Span.Start {
			continue
		}
		tag, closing, valid := xhtmlRegionTag(text, region)
		if !valid || closing || !sameLocalName(tag.Name, "link") {
			continue
		}
		attrs, valid := xhtmlAttributes(text, tag)
		if !valid {
			return "", false, fmt.Errorf("malformed XHTML link tag at byte %d", tag.Span.Start)
		}
		hrefAttrs := matchingXHTMLAttrs(attrs, "href")
		if len(hrefAttrs) > 1 {
			return "", false, fmt.Errorf("ambiguous href attributes in XHTML link at byte %d", tag.Span.Start)
		}
		for _, attr := range hrefAttrs {
			if html.UnescapeString(attr.Value) == href {
				return text, false, nil
			}
		}
	}
	link := `<link href="` + escapeXHTMLAttribute(href, '"') + `" type="text/css" rel="stylesheet"/>`
	updated, changed, err := applyXHTMLEdits(text, []editset.Edit{
		editset.Insert("xhtml", int64(headClose.Span.Start), []byte(link)),
	})
	return updated, changed, err
}

func xhtmlShellTags(text string, regions []xhtmlscan.Region) (root, headOpen, headClose *xhtmlscan.Tag) {
	for _, region := range regions {
		if region.Kind != xhtmlscan.RegionTag {
			continue
		}
		tag, closing, valid := xhtmlRegionTag(text, region)
		if !valid {
			continue
		}
		if root == nil && !closing && sameLocalName(tag.Name, "html") {
			copy := tag
			root = &copy
			continue
		}
		if root == nil {
			continue
		}
		if headOpen == nil && !closing && sameLocalName(tag.Name, "head") {
			copy := tag
			headOpen = &copy
			continue
		}
		if headOpen != nil && headClose == nil && closing && sameLocalName(tag.Name, "head") {
			copy := tag
			headClose = &copy
			return root, headOpen, headClose
		}
	}
	return root, headOpen, headClose
}

func xhtmlRegionTag(text string, region xhtmlscan.Region) (xhtmlscan.Tag, bool, bool) {
	if region.Span.Start < 0 || region.Span.End > len(text) || region.Span.Start >= region.Span.End {
		return xhtmlscan.Tag{}, false, false
	}
	raw := text[region.Span.Start:region.Span.End]
	name, _, closing := xhtmlscan.TagParts(raw)
	if name == "" {
		return xhtmlscan.Tag{}, false, false
	}
	attrs, valid := xhtmlscan.TagAttrs(raw)
	for i := range attrs {
		attrs[i].NameSpan.Start += region.Span.Start
		attrs[i].NameSpan.End += region.Span.Start
		attrs[i].ValueSpan.Start += region.Span.Start
		attrs[i].ValueSpan.End += region.Span.Start
	}
	selfClose := !closing && len(raw) >= 2 && raw[len(raw)-2] == '/'
	return xhtmlscan.Tag{Span: xhtmlscan.Span{Start: region.Span.Start, End: region.Span.End}, Name: name, Attrs: attrs, SelfClose: selfClose}, closing, valid
}

func xhtmlAttributes(text string, tag xhtmlscan.Tag) ([]xhtmlscan.Attr, bool) {
	if tag.Span.Start < 0 || tag.Span.End > len(text) || tag.Span.Start >= tag.Span.End {
		return nil, false
	}
	attrs, ok := xhtmlscan.TagAttrs(text[tag.Span.Start:tag.Span.End])
	for i := range attrs {
		attrs[i].NameSpan.Start += tag.Span.Start
		attrs[i].NameSpan.End += tag.Span.Start
		attrs[i].ValueSpan.Start += tag.Span.Start
		attrs[i].ValueSpan.End += tag.Span.Start
	}
	return attrs, ok
}

func matchingXHTMLAttrs(attrs []xhtmlscan.Attr, name string) []xhtmlscan.Attr {
	var matches []xhtmlscan.Attr
	for _, attr := range attrs {
		if strings.EqualFold(attr.Raw, name) {
			matches = append(matches, attr)
		}
	}
	return matches
}

func xhtmlAttributeEdits(path, text string, tag xhtmlscan.Tag, set map[string]string, remove map[string]bool) ([]editset.Edit, error) {
	attrs, ok := xhtmlAttributes(text, tag)
	if !ok {
		return nil, fmt.Errorf("malformed XHTML tag at byte %d", tag.Span.Start)
	}
	var edits []editset.Edit
	for name, value := range set {
		matches := matchingXHTMLAttrs(attrs, name)
		if len(matches) > 1 {
			return nil, fmt.Errorf("ambiguous %s attribute at byte %d", name, tag.Span.Start)
		}
		if len(matches) == 0 {
			continue
		}
		attr := matches[0]
		if html.UnescapeString(attr.Value) == value {
			delete(set, name)
			continue
		}
		if attr.Quote != 0 {
			edits = append(edits, editset.Replace(path, int64(attr.ValueSpan.Start), int64(attr.ValueSpan.End-attr.ValueSpan.Start), []byte(escapeXHTMLAttribute(value, attr.Quote))))
		} else {
			end := attr.ValueSpan.End
			edits = append(edits, editset.Replace(path, int64(attr.NameSpan.Start), int64(end-attr.NameSpan.Start), []byte(attr.Raw+`="`+escapeXHTMLAttribute(value, '"')+`"`)))
		}
		delete(set, name)
	}
	for _, attr := range attrs {
		if !remove[strings.ToLower(attr.Raw)] {
			continue
		}
		start := attr.NameSpan.Start
		for start > tag.Span.Start+1 && isXHTMLSpace(text[start-1]) {
			start--
		}
		end := attr.NameSpan.End
		if attr.Quote != 0 {
			end = attr.ValueSpan.End + 1
		} else if attr.ValueSpan.End > attr.ValueSpan.Start {
			end = attr.ValueSpan.End
		}
		edits = append(edits, editset.Replace(path, int64(start), int64(end-start), []byte{}))
	}
	if len(set) > 0 {
		var additions []string
		for name, value := range set {
			additions = append(additions, name+`="`+escapeXHTMLAttribute(value, '"')+`"`)
		}
		// Map iteration order must not affect generated XHTML bytes.
		for i := 1; i < len(additions); i++ {
			for j := i; j > 0 && additions[j] < additions[j-1]; j-- {
				additions[j], additions[j-1] = additions[j-1], additions[j]
			}
		}
		edits = append(edits, xhtmlInsertAttributes(path, text, tag, strings.Join(additions, " ")))
	}
	return edits, nil
}

func xhtmlInsertAttributes(path, text string, tag xhtmlscan.Tag, attributes string) editset.Edit {
	at := tag.Span.End - 1
	if tag.SelfClose && at > tag.Span.Start && text[at-1] == '/' {
		at--
	}
	return editset.Insert(path, int64(at), []byte(" "+attributes))
}

func xhtmlBigTagEdits(text string, regions []xhtmlscan.Region) ([]editset.Edit, error) {
	var edits []editset.Edit
	for _, region := range regions {
		if region.Kind != xhtmlscan.RegionTag {
			continue
		}
		tag, closing, valid := xhtmlRegionTag(text, region)
		if !valid || !sameLocalName(tag.Name, "big") {
			continue
		}
		nameStart := region.Span.Start + 1
		if closing {
			nameStart++
		}
		nameEnd := nameStart + len(tag.Name)
		localStart := nameStart
		if colon := strings.LastIndexByte(tag.Name, ':'); colon >= 0 {
			localStart += colon + 1
		}
		if localStart < nameStart || nameEnd > region.Span.End {
			return nil, fmt.Errorf("invalid big tag span at byte %d", region.Span.Start)
		}
		edits = append(edits, editset.Replace("xhtml", int64(localStart), int64(nameEnd-localStart), []byte("span")))
		if closing {
			continue
		}
		attrs, ok := xhtmlAttributes(text, tag)
		if !ok {
			return nil, fmt.Errorf("malformed XHTML big tag at byte %d", tag.Span.Start)
		}
		classes := matchingXHTMLAttrs(attrs, "class")
		if len(classes) > 1 {
			return nil, fmt.Errorf("ambiguous class attributes in XHTML big tag at byte %d", tag.Span.Start)
		}
		if len(classes) == 0 {
			edits = append(edits, xhtmlInsertAttributes("xhtml", text, tag, `class="big"`))
			continue
		}
		classValue := html.UnescapeString(classes[0].Value)
		if containsString(pySplitWS(classValue), "big") {
			continue
		}
		updated := classValue + " big"
		attr := classes[0]
		if attr.Quote == 0 {
			edits = append(edits, editset.Replace("xhtml", int64(attr.NameSpan.Start), int64(attr.ValueSpan.End-attr.NameSpan.Start), []byte(attr.Raw+`="`+escapeXHTMLAttribute(updated, '"')+`"`)))
		} else {
			edits = append(edits, editset.Replace("xhtml", int64(attr.ValueSpan.Start), int64(attr.ValueSpan.End-attr.ValueSpan.Start), []byte(escapeXHTMLAttribute(updated, attr.Quote))))
		}
	}
	return edits, nil
}

func applyXHTMLEdits(text string, edits []editset.Edit) (string, bool, error) {
	if len(edits) == 0 {
		return text, false, nil
	}
	if err := editset.Validate(edits); err != nil {
		return "", false, err
	}
	updated, err := editset.Apply("xhtml", []byte(text), edits)
	if err != nil {
		return "", false, err
	}
	return string(updated), true, nil
}

func xhtmlDoctypeSpan(text string) (xhtmlscan.Span, bool, error) {
	i := 0
	if strings.HasPrefix(text, "\uFEFF") {
		i = len("\uFEFF")
	}
	for i < len(text) {
		for i < len(text) && isXHTMLSpace(text[i]) {
			i++
		}
		switch {
		case strings.HasPrefix(text[i:], "<!--"):
			end := strings.Index(text[i+4:], "-->")
			if end < 0 {
				return xhtmlscan.Span{}, false, fmt.Errorf("unterminated XHTML prolog comment at byte %d", i)
			}
			i += 4 + end + 3
		case strings.HasPrefix(text[i:], "<?"):
			end := strings.Index(text[i+2:], "?>")
			if end < 0 {
				return xhtmlscan.Span{}, false, fmt.Errorf("unterminated XHTML processing instruction at byte %d", i)
			}
			i += 2 + end + 2
		case hasXHTMLDoctypePrefix(text[i:]):
			end := xhtmlDeclarationEnd(text, i+2)
			if end < 0 {
				return xhtmlscan.Span{}, false, fmt.Errorf("unterminated XHTML doctype at byte %d", i)
			}
			return xhtmlscan.Span{Start: i, End: end + 1}, true, nil
		default:
			return xhtmlscan.Span{}, false, nil
		}
	}
	return xhtmlscan.Span{}, false, nil
}

func hasXHTMLDoctypePrefix(text string) bool {
	const prefix = "<!DOCTYPE"
	if len(text) < len(prefix) || !strings.EqualFold(text[:len(prefix)], prefix) {
		return false
	}
	return len(text) == len(prefix) || isXHTMLSpace(text[len(prefix)])
}

func xhtmlDeclarationEnd(text string, start int) int {
	var quote byte
	depth := 0
	for i := start; i < len(text); {
		c := text[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
			i++
		case strings.HasPrefix(text[i:], "<!--"):
			end := strings.Index(text[i+4:], "-->")
			if end < 0 {
				return -1
			}
			i += 4 + end + 3
		case c == '\'' || c == '"':
			quote = c
			i++
		case c == '[':
			depth++
			i++
		case c == ']':
			if depth > 0 {
				depth--
			}
			i++
		case c == '>' && depth == 0:
			return i
		default:
			i++
		}
	}
	return -1
}

func escapeXHTMLAttribute(value string, quote byte) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\r", "&#13;", "\n", "&#10;", "\t", "&#09;")
	value = replacer.Replace(value)
	if quote == '\'' {
		return strings.ReplaceAll(value, "'", "&#39;")
	}
	return strings.ReplaceAll(value, `"`, "&quot;")
}

func sameLocalName(name, want string) bool {
	if colon := strings.LastIndexByte(name, ':'); colon >= 0 {
		name = name[colon+1:]
	}
	return strings.EqualFold(name, want)
}

func isXHTMLSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r'
}

func pyMatchGroupByteSpan(match *pyMatch, name string) (int, int, bool) {
	index, ok := match.re.named[name]
	if !ok || !match.hasGroupI(index) {
		return 0, 0, false
	}
	return match.byteStart(index), match.byteEnd(index), true
}

func matchHasRealElementBoundaries(text string, regions []xhtmlscan.Region, start, end int, firstName, lastName string) bool {
	if start < 0 || end < start || end > len(text) {
		return false
	}
	for start < end && isXHTMLSpace(text[start]) {
		start++
	}
	for end > start && isXHTMLSpace(text[end-1]) {
		end--
	}
	var first, last *xhtmlscan.Region
	for i := range regions {
		region := &regions[i]
		if region.Kind != xhtmlscan.RegionTag || region.Span.Start < start || region.Span.End > end {
			continue
		}
		if first == nil {
			first = region
		}
		last = region
	}
	if first == nil || last == nil || first.Span.Start != start || last.Span.End != end {
		return false
	}
	firstTag, firstClosing, firstOK := xhtmlRegionTag(text, *first)
	lastTag, lastClosing, lastOK := xhtmlRegionTag(text, *last)
	return firstOK && lastOK && !firstClosing && lastClosing &&
		sameLocalName(firstTag.Name, firstName) && sameLocalName(lastTag.Name, lastName)
}

func matchHasRealSingleElement(text string, regions []xhtmlscan.Region, start, end int, name string) bool {
	if start < 0 || end < start || end > len(text) {
		return false
	}
	for start < end && isXHTMLSpace(text[start]) {
		start++
	}
	for end > start && isXHTMLSpace(text[end-1]) {
		end--
	}
	for _, region := range regions {
		if region.Kind != xhtmlscan.RegionTag || region.Span.Start != start || region.Span.End != end {
			continue
		}
		tag, closing, valid := xhtmlRegionTag(text, region)
		return valid && !closing && sameLocalName(tag.Name, name)
	}
	return false
}

func regionTagNameAndClosing(text string, region xhtmlscan.Region) (string, bool, bool) {
	if region.Kind != xhtmlscan.RegionTag || region.Span.Start < 0 || region.Span.End > len(text) || region.Span.Start >= region.Span.End {
		return "", false, false
	}
	name, _, closing := xhtmlscan.TagParts(text[region.Span.Start:region.Span.End])
	return name, closing, name != ""
}

func rewriteDuokanMarkerGlyph(text string) (string, int, string, error) {
	regions, stop := xhtmlscan.ScanRegions(text)
	warning := ""
	if stop != xhtmlscan.ScanComplete {
		warning = fmt.Sprintf("markup scan stopped at byte offset %d (unterminated comment/CDATA/PI/declaration/tag or unclosed style/script); Duokan note markup after this offset left unchanged", stop)
	}
	const oldGlyph, newGlyph = "⊙", "◎"
	var edits []editset.Edit
	for i, region := range regions {
		if region.Kind != xhtmlscan.RegionTag {
			continue
		}
		name, closing, valid := regionTagNameAndClosing(text, region)
		if !valid || !closing || !sameLocalName(name, "a") {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			previous := regions[j]
			if previous.Kind != xhtmlscan.RegionTag || previous.Span.End+len(oldGlyph) != region.Span.Start {
				continue
			}
			if string(text[previous.Span.End:region.Span.Start]) == oldGlyph {
				edits = append(edits, editset.Replace("xhtml", int64(previous.Span.End), int64(len(oldGlyph)), []byte(newGlyph)))
			}
			break
		}
	}
	updated, changed, err := applyXHTMLEdits(text, edits)
	if err != nil {
		return "", 0, warning, err
	}
	if !changed {
		updated = text
	}
	return updated, len(edits), warning, nil
}
