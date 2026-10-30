package typography

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

func generatedLayer(name string) (string, bool) {
	rest, ok := strings.CutPrefix(path.Base(name), "epub-preset-")
	if !ok || len(rest) < 66 || rest[64] != '-' {
		return "", false
	}
	for _, ch := range rest[:64] {
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f') {
			return "", false
		}
	}
	layer := rest[65:]
	return layer, layerNameRe.MatchString(layer)
}

func stylesheetTarget(document, href string) (string, bool) {
	if opf.IsExternalURI(href) {
		return "", false
	}
	return opf.ResolveHref(document, pypath.URLSplit(href).Path)
}

// Replace only owned, same-layer links. Source spans preserve comments, body
// bytes and unrelated links; unsafe link semantics fail before Book.Apply.
func replaceScopedLinks(text, document string, paths []string, root *opf.SpanNode) (string, []map[string]string, error) {
	head := root.ChildByAnyNS("head")
	if head == nil || head.SelfClose {
		return "", nil, presetErrf("XHTML has no explicit head: %s", document)
	}
	desired := map[string]string{}
	for _, name := range paths {
		layer, _ := generatedLayer(name)
		desired[layer] = name
	}
	seen := map[string]bool{}
	replaced := []map[string]string{}
	var edits []editset.Edit
	for _, node := range head.Kids {
		if node.Name.Local != "link" || node.Name.Space != head.Name.Space {
			continue
		}
		rel, _ := node.AttrByLocal("", "rel")
		if !slices.Contains(strings.Fields(strings.ToLower(rel)), "stylesheet") {
			continue
		}
		href, _ := node.AttrByLocal("", "href")
		old, ok := stylesheetTarget(document, href)
		if !ok {
			continue
		}
		layer, owned := generatedLayer(old)
		target, selected := desired[layer]
		if !owned || !selected {
			continue
		}
		if old == target && !seen[layer] {
			seen[layer] = true
			continue
		}
		for _, attr := range node.Attrs {
			if attr.Name.Space != "" || !slices.Contains([]string{"rel", "href", "type"}, attr.Name.Local) {
				return "", nil, presetErrf("%s: scoped link has additional semantics; review manually: %s", document, href)
			}
		}
		end := node.Open.End
		if !node.SelfClose {
			end = node.Close.End
		}
		edits = append(edits, editset.Replace(document, int64(node.Open.Start), int64(end-node.Open.Start), []byte{}))
		replaced = append(replaced, map[string]string{"document": document, "layer": layer, "previousPath": old, "newPath": target})
	}
	updated, err := editset.Apply(document, []byte(text), edits)
	if err != nil {
		return "", nil, err
	}
	missing := make([]string, 0, len(paths))
	for _, name := range paths {
		layer, _ := generatedLayer(name)
		if !seen[layer] {
			missing = append(missing, name)
		}
	}
	result, err := appendStylesheetLinks(string(updated), document, missing)
	return result, replaced, err
}

// Read every possible static reference before deleting generated CSS. A file
// used by another page, CSS import, SVG or OPF relation remains in the book.
func pruneScopedCSS(ctx context.Context, b *book.Book, updates map[string][]byte, desired []string, opfPath string, opfRoot *opf.SpanNode) ([]editset.Edit, []string, error) {
	used := map[string]bool{}
	for _, name := range desired {
		used[name] = true
	}
	reference := func(document, href string) {
		if name, ok := stylesheetTarget(document, href); ok {
			used[name] = true
		}
	}
	read := func(name string) ([]byte, error) {
		if data, ok := updates[name]; ok {
			return data, nil
		}
		return b.CurrentContext(ctx, name)
	}
	for _, name := range b.Names() {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		switch strings.ToLower(path.Ext(name)) {
		case ".css":
			data, err := read(name)
			if err != nil {
				return nil, nil, err
			}
			sheet, err := css.Parse(data)
			if err != nil {
				return nil, nil, presetErrf("cannot prove stylesheet references in %s: %v", name, err)
			}
			for _, ref := range sheet.References {
				reference(name, ref.Value)
			}
		case ".xhtml", ".html", ".htm", ".svg":
			data, err := read(name)
			if err != nil {
				return nil, nil, err
			}
			if bytes.Contains(data, []byte("<?xml-stylesheet")) {
				return nil, nil, presetErrf("cannot prove XML stylesheet references: %s", name)
			}
			root, err := opf.ScanXHTMLSpanTree(data)
			if err != nil {
				return nil, nil, presetErrf("cannot prove references in %s: %v", name, err)
			}
			for _, node := range root.Walk() {
				if node.Name.Local == "base" {
					return nil, nil, presetErrf("cannot prune scoped CSS with an HTML base element: %s", name)
				}
				for _, attr := range node.Attrs {
					if strings.HasPrefix(strings.ToLower(attr.Name.Local), "on") {
						return nil, nil, presetErrf("cannot prove dynamic event-handler CSS references: %s", name)
					}
					if attr.Name.Local == "base" {
						return nil, nil, presetErrf("cannot prune scoped CSS with xml:base: %s", name)
					}
					if attr.Name.Local == "href" || attr.Name.Local == "src" {
						reference(name, attr.Value)
					}
					if attr.Name.Local == "style" {
						sheet, err := css.Parse([]byte(".inline{" + attr.Value + "}"))
						if err != nil {
							return nil, nil, presetErrf("cannot prove style attribute references: %s", name)
						}
						for _, ref := range sheet.References {
							reference(name, ref.Value)
						}
					}
				}
				if node.Name.Local == "script" {
					return nil, nil, presetErrf("cannot prove dynamic CSS references: %s", name)
				}
				if node.Name.Local == "style" {
					sheet, err := css.Parse([]byte(node.Text))
					if err != nil {
						return nil, nil, presetErrf("cannot prove inline CSS references: %s", name)
					}
					for _, ref := range sheet.References {
						reference(name, ref.Value)
					}
				}
			}
		}
	}
	protectedIDs := map[string]bool{}
	for _, node := range opfRoot.Walk() {
		for _, attr := range node.Attrs {
			if attr.Name.Local == "fallback" || attr.Name.Local == "media-overlay" || attr.Name.Local == "refines" {
				protectedIDs[strings.TrimPrefix(attr.Value, "#")] = true
			}
		}
	}
	items := map[string][]*opf.SpanNode{}
	for _, node := range opf.ManifestNodes(opfRoot) {
		href, _ := node.AttrByLocal("", "href")
		name, ok := stylesheetTarget(opfPath, href)
		if !ok {
			continue
		}
		items[name] = append(items[name], node)
		id, _ := node.AttrByLocal("", "id")
		if protectedIDs[id] || len(node.Attrs) != 3 {
			used[name] = true
		}
	}
	removed := []string{}
	for _, name := range b.Names() {
		if _, owned := generatedLayer(name); owned && !used[name] {
			removed = append(removed, name)
		}
	}
	slices.Sort(removed)
	var edits []editset.Edit
	for _, name := range removed {
		edits = append(edits, editset.Delete(name))
		for _, node := range items[name] {
			end := node.Open.End
			if !node.SelfClose {
				end = node.Close.End
			}
			edits = append(edits, editset.Replace(opfPath, int64(node.Open.Start), int64(end-node.Open.Start), []byte{}))
		}
	}
	return edits, removed, nil
}

func selectScope(spine, requested []string) ([]string, error) {
	if len(requested) == 0 {
		return nil, presetErrf("scope_paths must contain at least one spine XHTML path")
	}
	selected, _ := opf.SelectScopePaths(spine, requested)
	selectedSet := make(map[string]struct{}, len(selected))
	for _, name := range selected {
		selectedSet[name] = struct{}{}
	}
	for _, name := range requested {
		if _, ok := selectedSet[name]; !ok {
			return nil, presetErrf("scope_paths target is not a spine XHTML: %q", name)
		}
	}
	// SelectScopePaths keeps reading order and deduplicates caller input.
	return selected, nil
}

func scopedStylesheetPath(dir, layer string, data []byte) string {
	return pypath.Join(dir, fmt.Sprintf("epub-preset-%x-%s", sha256.Sum256(data), layer))
}

// Local mode only accepts self-contained layers. Copying CSS with relative
// imports/fonts/images would silently bind it to unrelated book assets. Asset
// installation requires an explicit future contract, not a best-effort guess.
func validateScopedCSS(data []byte) error {
	sheet, err := css.Parse(data)
	if err != nil {
		return err
	}
	return validateScopedCSSSheet(sheet)
}

func validateScopedCSSSheet(sheet *css.Stylesheet) error {
	if len(sheet.References) != 0 {
		return fmt.Errorf("scoped preset must be self-contained (no url() or @import)")
	}
	return nil
}

// Append by XML source span: preserve all original links, comments, inline
// styles and body bytes, including minified pages. Reapplying is idempotent.
func appendStylesheetLinks(text, document string, paths []string) (string, error) {
	root, err := opf.ScanXHTMLSpanTree([]byte(text))
	if err != nil {
		return "", presetErrf("%s: %v", document, err)
	}
	return appendStylesheetLinksTree(text, document, paths, root)
}

func appendStylesheetLinksTree(text, document string, paths []string, root *opf.SpanNode) (string, error) {
	head := root.ChildByAnyNS("head")
	if head == nil || head.SelfClose {
		return "", presetErrf("XHTML has no explicit head closing tag: %s", document)
	}
	seen := map[string]bool{}
	for _, node := range head.Kids {
		if node.Name.Local != "link" {
			continue
		}
		rel, _ := node.AttrByLocal("", "rel")
		if !slices.Contains(strings.Fields(strings.ToLower(rel)), "stylesheet") {
			continue
		}
		href, _ := node.AttrByLocal("", "href")
		seen[href] = true
	}
	var insert strings.Builder
	for _, cssPath := range paths {
		href := pypath.RelativeURI(document, cssPath)
		if !seen[href] {
			insert.WriteString(`<link rel="stylesheet" type="text/css" href="` + pypath.EscapeAttribute(href) + `"/>` + "\n")
		}
	}
	return text[:head.Close.Start] + insert.String() + text[head.Close.Start:], nil
}
