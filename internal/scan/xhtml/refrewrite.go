package xhtml

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CSSRewriter adapts the CSS scanner at the caller's layer. The scan/xhtml
// package intentionally does not import scan/css because both belong to the
// same architecture layer.
type CSSRewriter func(raw, document string, quote byte, rewriteURI func(string) string) (string, error)

// AttrEscaper preserves the owning capability's established attribute
// escaping policy when a URI changes.
type AttrEscaper func(quote byte, value string) string

// RewriteMarkup rewrites resource references only inside real markup regions.
// CSS tokenization is supplied by the caller so the scanner packages remain
// independent at the same architecture layer.
func RewriteMarkup(text, document string, rewriteURI func(string) string, rewriteCSS CSSRewriter, escapeAttr AttrEscaper, warn func(string, ...any)) (string, error) {
	if rewriteURI == nil {
		rewriteURI = func(uri string) string { return uri }
	}
	regions, stop := ScanRegions(text)
	if stop != ScanComplete && warn != nil {
		warn("%s: markup scan stopped at byte offset %d (unterminated comment/CDATA/PI/declaration/tag or unclosed style/script); references after this offset left unchanged", document, stop)
	}
	if len(regions) == 0 {
		return text, nil
	}
	var out strings.Builder
	out.Grow(len(text))
	last := 0
	for _, region := range regions {
		out.WriteString(text[last:region.Start])
		segment := text[region.Start:region.End]
		var err error
		switch region.Kind {
		case RegionTag:
			segment, err = rewriteTag(segment, document, rewriteURI, rewriteCSS, escapeAttr)
		case RegionStyle:
			if rewriteCSS != nil {
				segment, err = rewriteCSS(segment, document, 0, rewriteURI)
			}
		case RegionStylesheetPI:
			segment = rewriteStylesheetPI(segment, rewriteURI)
		}
		if err != nil {
			return "", err
		}
		out.WriteString(segment)
		last = region.End
	}
	out.WriteString(text[last:])
	return out.String(), nil
}

func rewriteTag(tag, document string, rewriteURI func(string) string, rewriteCSS CSSRewriter, escapeAttr AttrEscaper) (string, error) {
	attrs, ok := TagAttrs(tag)
	if !ok || len(attrs) == 0 {
		return tag, nil
	}
	var out strings.Builder
	last := 0
	changed := false
	for _, attr := range attrs {
		if attr.Quote == 0 {
			continue
		}
		raw := tag[attr.ValueSpan.Start:attr.ValueSpan.End]
		updated := raw
		switch {
		case strings.EqualFold(attr.Name, "srcset"):
			updated = rewriteSrcset(raw, rewriteURI)
		case isURIAttr(attr.Name):
			uri := raw
			if strings.Contains(raw, "&") {
				decoded, _, err := DecodeAttrWithMap(raw)
				if err != nil {
					return "", fmt.Errorf("%s: URI attribute entity decode: %w", document, err)
				}
				uri = decoded
			}
			updatedURI := rewriteURI(uri)
			if updatedURI != uri {
				if escapeAttr == nil {
					escapeAttr = defaultAttrEscape
				}
				updated = escapeAttr(attr.Quote, updatedURI)
			}
		case strings.EqualFold(attr.Name, "style") && rewriteCSS != nil:
			var err error
			updated, err = rewriteCSS(raw, document, attr.Quote, rewriteURI)
			if err != nil {
				return "", err
			}
		}
		if updated == raw {
			continue
		}
		out.WriteString(tag[last:attr.ValueSpan.Start])
		out.WriteString(updated)
		last = attr.ValueSpan.End
		changed = true
	}
	if !changed {
		return tag, nil
	}
	out.WriteString(tag[last:])
	return out.String(), nil
}

func isURIAttr(name string) bool {
	switch {
	case strings.EqualFold(name, "href"), strings.EqualFold(name, "src"), strings.EqualFold(name, "poster"),
		strings.EqualFold(name, "data"), strings.EqualFold(name, "xlink:href"), strings.EqualFold(name, "textref"):
		return true
	default:
		return false
	}
}

func rewriteSrcset(value string, rewriteURI func(string) string) string {
	var candidates []string
	start := 0
	inURL := true
	for index := 0; index < len(value); {
		r, size := utf8.DecodeRuneInString(value[index:])
		if unicode.IsSpace(r) && strings.TrimSpace(value[start:index]) != "" {
			inURL = false
		} else if r == ',' {
			current := strings.TrimSpace(value[start:index])
			currentURL := ""
			if parts := strings.Fields(current); len(parts) > 0 {
				currentURL = parts[0]
			}
			if inURL && strings.HasPrefix(strings.ToLower(currentURL), "data:") {
				index += size
				continue
			}
			if candidate := rewriteSrcsetCandidate(value[start:index], rewriteURI); candidate != "" {
				candidates = append(candidates, candidate)
			}
			start = index + size
			inURL = true
		}
		index += size
	}
	if candidate := rewriteSrcsetCandidate(value[start:], rewriteURI); candidate != "" {
		candidates = append(candidates, candidate)
	}
	return strings.Join(candidates, ", ")
}

func rewriteSrcsetCandidate(candidate string, rewriteURI func(string) string) string {
	parts := strings.Fields(strings.TrimSpace(candidate))
	if len(parts) == 0 {
		return ""
	}
	url := parts[0]
	if rewriteURI != nil {
		url = rewriteURI(url)
	}
	return strings.TrimSpace(url + " " + strings.Join(parts[1:], " "))
}

func rewriteStylesheetPI(pi string, rewriteURI func(string) string) string {
	if rewriteURI == nil {
		return pi
	}
	var out strings.Builder
	last := 0
	for i := 0; i < len(pi); {
		if !wordBoundary(pi, i) || i+4 > len(pi) || !strings.EqualFold(pi[i:i+4], "href") {
			r, size := utf8.DecodeRuneInString(pi[i:])
			if r == utf8.RuneError && size == 0 {
				break
			}
			i += size
			continue
		}
		j := skipSpace(pi, i+4)
		if j >= len(pi) || pi[j] != '=' {
			i += 4
			continue
		}
		j = skipSpace(pi, j+1)
		if j >= len(pi) || pi[j] != '\'' && pi[j] != '"' {
			i += 4
			continue
		}
		quote := pi[j]
		start := j + 1
		endRel := strings.IndexByte(pi[start:], quote)
		if endRel < 0 {
			i += 4
			continue
		}
		end := start + endRel
		uri := pi[start:end]
		updated := rewriteURI(uri)
		if updated != uri {
			out.WriteString(pi[last:start])
			out.WriteString(updated)
			last = end
		}
		i = end + 1
	}
	if last == 0 {
		return pi
	}
	out.WriteString(pi[last:])
	return out.String()
}

func wordBoundary(text string, offset int) bool {
	before := false
	if offset > 0 {
		r, _ := utf8.DecodeLastRuneInString(text[:offset])
		before = r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
	}
	after := false
	if offset < len(text) {
		r, _ := utf8.DecodeRuneInString(text[offset:])
		after = r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
	}
	return before != after
}

func skipSpace(text string, offset int) int {
	for offset < len(text) {
		r, size := utf8.DecodeRuneInString(text[offset:])
		if !unicode.IsSpace(r) {
			break
		}
		offset += size
	}
	return offset
}

func defaultAttrEscape(quote byte, value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	if quote == '\'' {
		return strings.ReplaceAll(value, "'", "&#39;")
	}
	return strings.ReplaceAll(value, `"`, "&quot;")
}
