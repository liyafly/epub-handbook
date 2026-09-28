package xhtml

import (
	"html"
	"strconv"
	"strings"

	"github.com/liyafly/epub-handbook/internal/editset"
)

// XHTML11EntityEdits returns byte-range edits that replace known named
// references from the XHTML 1.1 external entity set with numeric references
// for XML readers that do not load external DTDs. Unknown references remain
// untouched and still fail strict XML validation.
func XHTML11EntityEdits(path, text string) []editset.Edit {
	if !hasXHTML11Doctype(text) {
		return nil
	}
	regions, stop := ScanRegions(text)
	if stop != ScanComplete {
		return nil
	}
	var edits []editset.Edit
	for _, region := range regions {
		switch region.Kind {
		case RegionText:
			appendNamedEntityReplacements(text, region.Start, region.End, path, true, &edits)
		case RegionTag:
			tag := text[region.Start:region.End]
			attrs, ok := TagAttrs(tag)
			if !ok {
				continue
			}
			for _, attr := range attrs {
				appendNamedEntityReplacements(
					text,
					region.Start+attr.ValueSpan.Start,
					region.Start+attr.ValueSpan.End,
					path,
					false,
					&edits,
				)
			}
		}
	}
	return edits
}

func hasXHTML11Doctype(text string) bool {
	for index := 0; index < len(text); {
		relativeTag := strings.IndexByte(text[index:], '<')
		if relativeTag < 0 {
			return false
		}
		start := index + relativeTag
		rest := text[start:]
		switch {
		case strings.HasPrefix(rest, "<!--"):
			end := regionCommentEnd(rest)
			if end < 0 {
				return false
			}
			index = start + end
		case strings.HasPrefix(rest, "<![CDATA["):
			end := strings.Index(rest[9:], "]]>")
			if end < 0 {
				return false
			}
			index = start + 9 + end + 3
		case strings.HasPrefix(rest, "<?"):
			end := strings.Index(rest[2:], "?>")
			if end < 0 {
				return false
			}
			index = start + 2 + end + 2
		case strings.HasPrefix(rest, "<!"):
			end := findRegionDeclClose(text, start+2)
			if end < 0 {
				return false
			}
			declaration := strings.ToLower(text[start : end+1])
			if strings.Contains(declaration, "-//w3c//dtd xhtml 1.1//en") ||
				strings.Contains(declaration, "/xhtml11.dtd") {
				return true
			}
			index = end + 1
		default:
			index = start + 1
		}
	}
	return false
}

func appendNamedEntityReplacements(text string, start, end int, path string, skipOpaqueMarkup bool, edits *[]editset.Edit) {
	for start < end {
		if skipOpaqueMarkup {
			rest := text[start:end]
			switch {
			case strings.HasPrefix(rest, "<!--"):
				opaqueEnd := regionCommentEnd(rest)
				if opaqueEnd < 0 {
					return
				}
				start += opaqueEnd
				continue
			case strings.HasPrefix(rest, "<![CDATA["):
				opaqueEnd := strings.Index(rest[9:], "]]>")
				if opaqueEnd < 0 {
					return
				}
				start += 9 + opaqueEnd + 3
				continue
			case strings.HasPrefix(rest, "<?"):
				opaqueEnd := strings.Index(rest[2:], "?>")
				if opaqueEnd < 0 {
					return
				}
				start += 2 + opaqueEnd + 2
				continue
			case strings.HasPrefix(rest, "<!"):
				opaqueEnd := findRegionDeclClose(text, start+2)
				if opaqueEnd < 0 || opaqueEnd >= end {
					return
				}
				start = opaqueEnd + 1
				continue
			}
		}
		relativeAmp := strings.IndexByte(text[start:end], '&')
		if relativeAmp < 0 {
			return
		}
		amp := start + relativeAmp
		relativeSemicolon := strings.IndexByte(text[amp+1:end], ';')
		if relativeSemicolon < 0 {
			return
		}
		entityEnd := amp + 1 + relativeSemicolon + 1
		name := text[amp+1 : entityEnd-1]
		if name == "" || name[0] == '#' || isPredefinedXMLEntity(name) {
			start = entityEnd
			continue
		}
		entity := text[amp:entityEnd]
		decoded := html.UnescapeString(entity)
		if decoded == entity {
			start = entityEnd
			continue
		}
		var value strings.Builder
		for _, character := range decoded {
			value.WriteString("&#")
			value.WriteString(strconv.Itoa(int(character)))
			value.WriteByte(';')
		}
		*edits = append(*edits, editset.Replace(path, int64(amp), int64(entityEnd-amp), []byte(value.String())))
		start = entityEnd
	}
}

func isPredefinedXMLEntity(name string) bool {
	switch name {
	case "amp", "lt", "gt", "quot", "apos":
		return true
	default:
		return false
	}
}
