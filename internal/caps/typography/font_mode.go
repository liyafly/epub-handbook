package typography

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// Whole-book presets preserve the existing font layer, including its font
// faces, roles and relative URLs. They never infer a new font chain or change
// ibooks metadata to make a candidate pass. Noncanonical body bindings require
// an explicit font repair before replacing the book's stylesheet links.
func preserveFontMode(ctx context.Context, b *book.Book, pages *typographyPageCache, root *opf.SpanNode, paths []string, fontsPath string, layers map[string][]byte) (string, error) {
	meta := root.ChildByLocal(opf.OPFURI, "metadata")
	var fontModeValues []string
	metaCount := 0
	if meta != nil {
		for _, node := range meta.ChildrenByLocal(opf.OPFURI, "meta") {
			property, _ := node.AttrByLocal("", "property")
			if strings.TrimSpace(property) == "ibooks:specified-fonts" {
				metaCount++
				fontModeValues = append(fontModeValues, strings.TrimSpace(node.IterText()))
			}
			name, _ := node.AttrByLocal("", "name")
			if strings.TrimSpace(name) == "ibooks:specified-fonts" {
				content, _ := node.AttrByLocal("", "content")
				fontModeValues = append(fontModeValues, strings.TrimSpace(content))
			}
		}
	}
	if metaCount > 1 {
		return "", presetErrf("ambiguous ibooks:specified-fonts metadata; repair font mode before applying a whole-book preset")
	}
	const displayOptionsPath = "META-INF/com.apple.ibooks.display-options.xml"
	if b.Has(displayOptionsPath) {
		data, err := b.CurrentContext(ctx, displayOptionsPath)
		if err != nil {
			return "", err
		}
		displayOptions, err := opf.ScanSpanTree(data)
		if err != nil {
			return "", presetErrf("ambiguous ibooks:specified-fonts metadata; repair font mode before applying a whole-book preset")
		}
		for _, node := range displayOptions.Walk() {
			if node.Name.Local != "option" {
				continue
			}
			name, _ := node.AttrByLocal("", "name")
			if strings.TrimSpace(name) == "specified-fonts" {
				fontModeValues = append(fontModeValues, strings.TrimSpace(node.IterText()))
			}
		}
	}
	for _, value := range fontModeValues {
		if value != "true" {
			return "", presetErrf("ambiguous ibooks:specified-fonts metadata; repair font mode before applying a whole-book preset")
		}
	}
	lockedMeta := len(fontModeValues) > 0

	// Read the spine first so selector classification can distinguish book-wide
	// body classes from page-role classes before inspecting any stylesheet.
	bookClassPages := map[string]int{}
	type pageStyles struct {
		path   string
		styles [][]byte
	}
	pageCSS := make([]pageStyles, 0, len(paths))
	legacyPages := 0
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if _, err := pages.data(path); err != nil {
			return "", err
		}
		doc, err := pages.tree(path)
		if err != nil {
			return "", presetErrf("%s: font-mode markup scan: %v", path, err)
		}
		pageClasses := map[string]bool{}
		var styles [][]byte
		for _, node := range doc.Walk() {
			if node.Name.Local == "style" {
				styles = append(styles, []byte(node.IterText()))
			}
			if node.Name.Local == "body" || (node == doc && node.Name.Local == "html") {
				class, _ := node.AttrByLocal("", "class")
				for _, name := range strings.Fields(class) {
					pageClasses[name] = true
				}
				style, _ := node.AttrByLocal("", "style")
				if style != "" {
					selector := "body"
					if node.Name.Local == "html" {
						selector = "html"
					}
					styles = append(styles, []byte(selector+" {"+style+"}"))
				}
			}
		}
		for class := range pageClasses {
			bookClassPages[class]++
		}
		pageCSS = append(pageCSS, pageStyles{path: path, styles: styles})
		body := doc.ChildByAnyNS("body")
		if body != nil {
			classes, _ := body.AttrByLocal("", "class")
			if slices.Contains(strings.Fields(classes), "body-font-locked") {
				legacyPages++
			}
		}
	}
	bookClasses := map[string]bool{}
	for class, count := range bookClassPages {
		if len(paths) > 0 && count == len(paths) {
			bookClasses[class] = true
		}
	}
	for _, page := range pageCSS {
		for _, data := range page.styles {
			d, l, err := bodyBindings(data, bookClasses)
			if err != nil || d || l {
				return "", presetErrf("%s: embedded body font cascade requires explicit review", page.path)
			}
		}
	}

	var direct, legacy bool
	for _, path := range b.Names() {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if !strings.EqualFold(pypath.PathExt(path), ".css") {
			continue
		}
		data, err := b.CurrentContext(ctx, path)
		if err != nil {
			return "", err
		}
		d, l, err := bodyBindings(data, bookClasses)
		if err != nil {
			return "", presetErrf("%s: cannot preserve font mode: %v", path, err)
		}
		if path != fontsPath && (d || l) {
			return "", presetErrf("%s: body font binding is outside the preserved fonts.css layer; repair font layering before applying a whole-book preset", path)
		}
		if path == fontsPath {
			direct, legacy = d, l
			if _, installsFonts := layers[path]; !installsFonts {
				return "", presetErrf("preset must include fonts.css to preserve the existing font layer")
			}
			layers[path] = data
		}
	}
	if !direct && legacyPages > 0 && (!legacy || legacyPages != len(paths)) {
		return "", presetErrf("mixed or unresolved body-font-locked mode; repair font mode before applying a whole-book preset")
	}
	locked := direct || (legacy && legacyPages > 0)
	if locked != lockedMeta {
		return "", presetErrf("body font binding and ibooks:specified-fonts disagree; repair font mode before applying a whole-book preset")
	}
	for _, path := range slices.Sorted(maps.Keys(layers)) {
		data := layers[path]
		if path == fontsPath && b.Has(fontsPath) {
			continue // Exact original bytes, already checked above.
		}
		d, l, err := bodyBindings(data, bookClasses)
		if err != nil {
			return "", presetErrf("%s: cannot preserve font mode: %v", path, err)
		}
		if d || l {
			return "", presetErrf("%s: preset must not introduce or replace a body font binding", path)
		}
	}
	if locked {
		return "locked", nil
	}
	return "free", nil
}

// bodyBindings reads CSS tokens/rules; comments and string contents cannot
// invent bindings. Bare paragraph/root/universal font rules and imports are
// deliberately refused: their effective cascade is not a font-mode decision
// this preset installer can safely make.
func bodyBindings(data []byte, bookClasses map[string]bool) (direct, legacy bool, err error) {
	sheet, err := css.Parse(data)
	if err != nil {
		return false, false, err
	}
	return bodyBindingsSheet(sheet, bookClasses)
}

func bodyBindingsSheet(sheet *css.Stylesheet, bookClasses map[string]bool) (direct, legacy bool, err error) {
	for _, ref := range sheet.References {
		if ref.Kind == css.ReferenceImport {
			return false, false, fmt.Errorf("imported font cascade requires explicit review")
		}
	}
	for _, rule := range sheet.Rules {
		if rule.AtRule {
			if rule.HasBlock && !rule.Nested && len(rule.Declarations) == 0 &&
				strings.Contains(strings.ToLower(css.StripComments(rule.Body)), "font") {
				return false, false, fmt.Errorf("opaque @%s block with font declarations requires explicit review", rule.AtRuleName)
			}
			continue
		}
		for _, decl := range rule.Declarations {
			if !strings.EqualFold(decl.Name, "font-family") && !strings.EqualFold(decl.Name, "font") && !strings.EqualFold(decl.Name, "all") {
				continue
			}
			for selector := range strings.SplitSeq(css.StripComments(rule.Selector), ",") {
				switch classifySelector(selector, bookClasses) {
				case "direct":
					if err := checkStableBinding(sheet, rule, decl); err != nil {
						return false, false, err
					}
					direct = true
				case "legacy":
					if err := checkStableBinding(sheet, rule, decl); err != nil {
						return false, false, err
					}
					legacy = true
				case "review":
					return false, false, fmt.Errorf("font binding on %q requires explicit review", strings.ToLower(strings.TrimSpace(css.StripComments(selector))))
				}
			}
		}
	}
	return direct, legacy, nil
}

// classifySelector identifies selectors that can carry a book-wide body font
// binding. Page-role selectors remain available when their classes do not occur
// on every spine page.
func classifySelector(selector string, bookClasses map[string]bool) string {
	s := strings.ToLower(strings.TrimSpace(css.StripComments(selector)))
	switch s {
	case "body":
		return "direct"
	case ".body-font-locked", "body.body-font-locked":
		return "legacy"
	case "p", "html", ":root", "*":
		return "review"
	}
	parts := strings.Fields(strings.NewReplacer(">", " ", "+", " ", "~", " ").Replace(s))
	if len(parts) == 0 {
		return ""
	}
	subject := parts[len(parts)-1]
	qualifier := strings.IndexAny(subject, ".#:[")
	typeName := subject
	if qualifier >= 0 {
		typeName = subject[:qualifier]
	}
	classes := selectorClasses(subject)
	qualified := qualifier >= 0
	if (typeName == "p" || typeName == "*") && !qualified {
		ancestorsOK := true
		for _, ancestor := range parts[:len(parts)-1] {
			ancestorType := ancestor
			if i := strings.IndexAny(ancestor, ".#:["); i >= 0 {
				ancestorType = ancestor[:i]
			}
			if ancestorType != "body" && ancestorType != "html" && ancestor != ":root" {
				ancestorsOK = false
				break
			}
		}
		if ancestorsOK && len(parts) > 1 {
			return "review"
		}
	}
	allBookClasses := func() bool {
		for _, class := range classes {
			if !bookClasses[class] {
				return false
			}
		}
		return true
	}
	if (typeName == "body" || typeName == "html" || strings.HasPrefix(subject, ":root")) && allBookClasses() {
		return "review"
	}
	if typeName == "" && len(classes) > 0 && !strings.ContainsAny(subject, "#:[") && allBookClasses() {
		return "review"
	}
	return ""
}

func selectorClasses(s string) []string {
	var classes []string
	for i := 0; i < len(s); {
		if s[i] != '.' {
			i++
			continue
		}
		start := i + 1
		j := start
		for j < len(s) {
			r, size := utf8.DecodeRuneInString(s[j:])
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
				break
			}
			j += size
		}
		if j > start {
			classes = append(classes, s[start:j])
		}
		i = j
	}
	return classes
}

func checkStableBinding(sheet *css.Stylesheet, rule css.Rule, decl css.Declaration) error {
	if rule.InAtRule || strings.EqualFold(decl.Name, "all") {
		return fmt.Errorf("conditional or reset body font binding requires explicit review")
	}
	for _, token := range sheet.Tokens {
		if token.Span.Start < decl.ValueSpan.Start || token.Span.End > decl.ValueSpan.End || token.Kind != css.TokenIdent {
			continue
		}
		switch strings.ToLower(string(token.Data)) {
		case "var", "env", "inherit", "initial", "unset", "revert", "revert-layer":
			return fmt.Errorf("inherited or dynamic body font binding requires explicit review")
		}
	}
	return nil
}
