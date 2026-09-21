package typography

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// Whole-book presets preserve the existing font layer, including its font
// faces, roles and relative URLs. They never infer a new font chain or change
// ibooks metadata to make a candidate pass. Noncanonical body bindings require
// an explicit font repair before replacing the book's stylesheet links.
func preserveFontMode(ctx context.Context, b *book.Book, root *opf.SpanNode, paths []string, fontsPath string, layers map[string][]byte) (string, error) {
	meta := root.ChildByLocal(opf.OPFURI, "metadata")
	lockedMeta := false
	metaCount := 0
	if meta != nil {
		for _, node := range meta.ChildrenByLocal(opf.OPFURI, "meta") {
			property, _ := node.AttrByLocal("", "property")
			if property == "ibooks:specified-fonts" {
				metaCount++
				lockedMeta = strings.TrimSpace(node.IterText()) == "true"
			}
		}
	}
	if metaCount > 1 || (metaCount == 1 && !lockedMeta) {
		return "", presetErrf("ambiguous ibooks:specified-fonts metadata; repair font mode before applying a whole-book preset")
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
		d, l, err := bodyBindings(data)
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

	legacyPages := 0
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		data, err := b.CurrentContext(ctx, path)
		if err != nil {
			return "", err
		}
		doc, err := opf.ScanSpanTree(data)
		if err != nil {
			return "", presetErrf("%s: font-mode markup scan: %v", path, err)
		}
		for _, node := range doc.Walk() {
			if node.Name.Local == "style" {
				d, l, err := bodyBindings([]byte(node.IterText()))
				if err != nil || d || l {
					return "", presetErrf("%s: embedded body font cascade requires explicit review", path)
				}
			}
			if node.Name.Local == "body" || node.Name.Local == "html" {
				style, _ := node.AttrByLocal("", "style")
				d, l, err := bodyBindings([]byte("body {" + style + "}"))
				if err != nil || d || l {
					return "", presetErrf("%s: inline body font cascade requires explicit review", path)
				}
			}
		}
		body := doc.ChildByAnyNS("body")
		if body != nil {
			classes, _ := body.AttrByLocal("", "class")
			if slices.Contains(strings.Fields(classes), "body-font-locked") {
				legacyPages++
			}
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
		d, l, err := bodyBindings(data)
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
func bodyBindings(data []byte) (direct, legacy bool, err error) {
	sheet, err := css.Parse(data)
	if err != nil {
		return false, false, err
	}
	for _, ref := range sheet.References {
		if ref.Kind == css.ReferenceImport {
			return false, false, fmt.Errorf("imported font cascade requires explicit review")
		}
	}
	for _, rule := range sheet.Rules {
		if rule.AtRule {
			continue
		}
		for _, decl := range rule.Declarations {
			if !strings.EqualFold(decl.Name, "font-family") && !strings.EqualFold(decl.Name, "font") && !strings.EqualFold(decl.Name, "all") {
				continue
			}
			for selector := range strings.SplitSeq(css.StripComments(rule.Selector), ",") {
				switch strings.TrimSpace(strings.ToLower(selector)) {
				case "body":
					if err := checkStableBinding(sheet, rule, decl); err != nil {
						return false, false, err
					}
					direct = true
				case ".body-font-locked", "body.body-font-locked":
					if err := checkStableBinding(sheet, rule, decl); err != nil {
						return false, false, err
					}
					legacy = true
				case "p", "html", ":root", "*":
					return false, false, fmt.Errorf("font binding on %q requires explicit review", selector)
				}
			}
		}
	}
	return direct, legacy, nil
}

func checkStableBinding(sheet *css.Stylesheet, rule css.Rule, decl css.Declaration) error {
	if rule.Nested || strings.EqualFold(decl.Name, "all") {
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
