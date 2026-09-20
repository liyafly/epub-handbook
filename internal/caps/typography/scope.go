package typography

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

func selectScope(spine, requested []string) ([]string, error) {
	if len(requested) == 0 {
		return nil, presetErrf("scope_paths must contain at least one spine XHTML path")
	}
	for _, name := range requested {
		if !slices.Contains(spine, name) {
			return nil, presetErrf("scope_paths target is not a spine XHTML: %q", name)
		}
	}
	// Keep reading order, independent of caller order, and deduplicate.
	selected := make([]string, 0, len(requested))
	for _, name := range spine {
		if slices.Contains(requested, name) && !slices.Contains(selected, name) {
			selected = append(selected, name)
		}
	}
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
	if len(sheet.References) != 0 {
		return fmt.Errorf("scoped preset must be self-contained (no url() or @import)")
	}
	return nil
}

// Append by XML source span: preserve all original links, comments, inline
// styles and body bytes, including minified pages. Reapplying is idempotent.
func appendStylesheetLinks(text, document string, paths []string) (string, error) {
	root, err := opf.ScanSpanTree([]byte(text))
	if err != nil {
		return "", presetErrf("%s: %v", document, err)
	}
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
		href := pypath.RelativePath(document, cssPath)
		if !seen[href] {
			insert.WriteString(`<link rel="stylesheet" type="text/css" href="` + pypath.EscapeAttribute(href) + `"/>` + "\n")
		}
	}
	return text[:head.Close.Start] + insert.String() + text[head.Close.Start:], nil
}
