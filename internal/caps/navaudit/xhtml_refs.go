package navaudit

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

type xhtmlDocument struct {
	item            opf.ManifestItem
	ids             map[string]struct{}
	references      []xhtmlReference
	unsupportedBase bool
	baseReference   string
}

type xhtmlReference struct {
	kind string
	raw  string
}

// scanXHTMLDocument verifies local references and keeps only IDs/reference
// strings for cross-document fragment checks. It releases the full span tree
// after each document instead of retaining every XHTML tree in memory.
func (ins *inspector) scanXHTMLDocument(ctx context.Context, item opf.ManifestItem, root *opf.SpanNode, manifestMedia map[string]map[string]struct{}) xhtmlDocument {
	doc := xhtmlDocument{item: item, ids: make(map[string]struct{})}
	walkXHTMLNodes(ctx, root, func(n *opf.SpanNode) {
		if !doc.unsupportedBase {
			if strings.EqualFold(n.Name.Local, "base") && (n.Name.Space == "" || n.Name.Space == opf.XHTMLURI) {
				if href, ok := n.AttrByLocal("", "href"); ok {
					doc.unsupportedBase, doc.baseReference = true, href
				} else {
					doc.unsupportedBase, doc.baseReference = true, "<base>"
				}
			}
			if base, ok := n.AttrByLocal(opf.XMLURI, "base"); ok {
				doc.unsupportedBase, doc.baseReference = true, base
			}
		}
		if id, ok := n.AttrByLocal("", "id"); ok && id != "" {
			doc.ids[id] = struct{}{}
		}
		if id, ok := n.AttrByLocal(opf.XMLURI, "id"); ok && id != "" {
			doc.ids[id] = struct{}{}
		}

		switch strings.ToLower(n.Name.Local) {
		case "img":
			if src, ok := n.AttrByLocal("", "src"); ok {
				doc.references = append(doc.references, xhtmlReference{kind: "image", raw: src})
			}
		case "link":
			rel, hasRel := n.AttrByLocal("", "rel")
			if hasRel && hasTokenFold(rel, "stylesheet") {
				if href, ok := n.AttrByLocal("", "href"); ok {
					doc.references = append(doc.references, xhtmlReference{kind: "stylesheet", raw: href})
				}
			}
		case "a":
			if href, ok := n.AttrByLocal("", "href"); ok {
				doc.references = append(doc.references, xhtmlReference{kind: "link", raw: href})
			}
		}
	})
	if ctx.Err() != nil {
		return doc
	}
	if doc.unsupportedBase {
		ins.addXHTMLReferenceFinding("error", "XHTML base URL semantics are not supported; local references cannot be safely resolved",
			"xhtml-base-unsupported", doc.item.ArchivePath, doc.baseReference, "<unresolved>")
		ins.addSkill("epub-audit", "error")
		return doc
	}
	for _, ref := range doc.references {
		if ctx.Err() != nil {
			return doc
		}
		ins.checkXHTMLLocalReference(ctx, doc.item.ArchivePath, ref.kind, ref.raw, manifestMedia)
	}
	return doc
}

func (ins *inspector) checkXHTMLFragments(ctx context.Context, documents []xhtmlDocument, manifestXHTML map[string]struct{}) {
	idsByPath := make(map[string]map[string]struct{}, len(documents))
	for _, doc := range documents {
		if ctx.Err() != nil {
			return
		}
		if _, done := idsByPath[doc.item.ArchivePath]; !done {
			idsByPath[doc.item.ArchivePath] = doc.ids
		}
	}

	seenDocuments := make(map[string]struct{}, len(documents))
	for _, doc := range documents {
		if ctx.Err() != nil {
			return
		}
		if doc.unsupportedBase {
			continue
		}
		if _, seen := seenDocuments[doc.item.ArchivePath]; seen {
			continue
		}
		seenDocuments[doc.item.ArchivePath] = struct{}{}
		for _, ref := range doc.references {
			if ctx.Err() != nil {
				return
			}
			if ref.kind == "link" {
				ins.checkXHTMLFragmentReference(ctx, doc.item.ArchivePath, ref.raw, idsByPath, manifestXHTML)
			}
		}
	}
}

func (ins *inspector) checkXHTMLFragmentReference(ctx context.Context, source, raw string, idsByPath map[string]map[string]struct{}, manifestXHTML map[string]struct{}) {
	if ctx.Err() != nil || pypath.IsExternalURI(raw) {
		return
	}
	parts := pypath.URLSplit(raw)
	if parts.Netloc != "" || strings.HasPrefix(parts.Path, "/") || parts.Fragment == "" {
		return
	}
	target := source
	if parts.Path != "" {
		resolved, err := pypath.ResolveRelativePath(source, parts.Path)
		if err != nil || !ins.b.Has(resolved) {
			// The resource check emits the more direct path/entry finding.
			return
		}
		target = resolved
	}
	fragment, err := url.PathUnescape(parts.Fragment)
	if err != nil {
		ins.addXHTMLReferenceFinding("error", "XHTML fragment has invalid percent-encoding", "xhtml-invalid-fragment", source, raw, target+"#<invalid-fragment>")
		ins.addSkill("epub-audit", "error")
		return
	}
	if strings.HasPrefix(strings.ToLower(fragment), "epubcfi(") {
		ins.addXHTMLReferenceFinding("warn", "EPUB CFI fragment is not resolved by this audit", "xhtml-cfi-unverified", source, raw, target+"#"+fragment)
		ins.addSkill("epub-audit", "warn")
		return
	}
	ids, parsedXHTML := idsByPath[target]
	if !parsedXHTML {
		if _, declared := manifestXHTML[target]; declared {
			// The target is an XHTML manifest item but could not be read or parsed;
			// its existing read/parse finding is sufficient to fail the audit.
			return
		}
		if strings.HasSuffix(strings.ToLower(target), ".xhtml") {
			ins.addXHTMLReferenceFinding("error", "XHTML fragment target exists but is not declared as a readable XHTML manifest item",
				"xhtml-fragment-target-unverified", source, raw, target+"#"+fragment)
			ins.addSkill("epub-audit", "error")
		}
		return
	}
	if _, ok := ids[fragment]; !ok {
		ins.addXHTMLReferenceFinding("error", "XHTML fragment target ID is missing", "xhtml-missing-fragment", source, raw, target+"#"+fragment)
		ins.addSkill("epub-audit", "error")
	}
}

func walkXHTMLNodes(ctx context.Context, root *opf.SpanNode, visit func(*opf.SpanNode)) bool {
	stack := []*opf.SpanNode{root}
	for len(stack) > 0 {
		if ctx.Err() != nil {
			return false
		}
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		visit(n)
		for i := len(n.Kids) - 1; i >= 0; i-- {
			stack = append(stack, n.Kids[i])
		}
	}
	return true
}

func (ins *inspector) checkXHTMLLocalReference(ctx context.Context, source, kind, raw string, manifestMedia map[string]map[string]struct{}) {
	if ctx.Err() != nil || pypath.IsExternalURI(raw) {
		return
	}
	parts := pypath.URLSplit(raw)
	if parts.Netloc != "" || strings.HasPrefix(parts.Path, "/") {
		return
	}
	// Fragment-only and query-only hyperlinks refer to the current document.
	// Fragment validation is handled separately from resource validation.
	if kind == "link" && parts.Path == "" {
		return
	}
	if parts.Path == "" {
		ins.addXHTMLReferenceFinding("error", "XHTML local "+kind+" reference has an empty target", "xhtml-empty-target", source, raw, "<empty>")
		ins.addSkill("epub-audit", "error")
		return
	}
	target, err := pypath.ResolveRelativePath(source, parts.Path)
	if err != nil {
		ins.addXHTMLReferenceFinding("error", "XHTML local "+kind+" target is invalid: "+err.Error(), "xhtml-invalid-target", source, raw, "<invalid>")
		ins.addSkill("epub-audit", "error")
		return
	}
	if !ins.b.Has(target) {
		ins.addXHTMLReferenceFinding("error", "XHTML local "+kind+" target is missing from the ZIP archive", "xhtml-missing-target", source, raw, target)
		ins.addSkill("epub-audit", "error")
		return
	}
	if kind == "link" {
		return
	}
	mediaTypes, declared := manifestMedia[target]
	if !declared {
		ins.addXHTMLReferenceFinding("error", "XHTML local "+kind+" target exists in the ZIP archive but is missing from the OPF manifest", "xhtml-missing-manifest-item", source, raw, target)
		ins.addSkill("epub-audit", "error")
		return
	}
	if kind == "stylesheet" {
		if _, ok := mediaTypes["text/css"]; !ok {
			ins.addXHTMLReferenceFinding("error", "XHTML stylesheet target has no text/css OPF manifest declaration", "xhtml-stylesheet-manifest-type", source, raw, target)
			ins.addSkill("epub-audit", "error")
		}
	}
}

func (ins *inspector) addXHTMLReferenceFinding(level, message, kind, source, raw, target string) {
	location := fmt.Sprintf("%s: %q -> %s", source, raw, target)
	ins.addFinding(level, message, location, kind)
}

func hasTokenFold(value, token string) bool {
	for _, part := range strings.Fields(value) {
		if strings.EqualFold(part, token) {
			return true
		}
	}
	return false
}
