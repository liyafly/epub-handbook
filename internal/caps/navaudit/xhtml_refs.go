package navaudit

import (
	"context"
	"fmt"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

type xhtmlDocument struct {
	item opf.ManifestItem
	root *opf.SpanNode
}

// checkXHTMLResources verifies local references in real XHTML elements. The
// span scanner ignores markup-like text in comments, CDATA and script bodies;
// this audit only reads source and never serializes or edits it.
func (ins *inspector) checkXHTMLResources(ctx context.Context, doc xhtmlDocument, manifestMedia map[string]map[string]struct{}) {
	if raw, ok := unsupportedXHTMLBase(ctx, doc.root); ok {
		ins.addXHTMLReferenceFinding("error", "XHTML base URL semantics are not supported; local references cannot be safely resolved",
			"xhtml-base-unsupported", doc.item.ArchivePath, raw, "<unresolved>")
		ins.addSkill("epub-audit", "error")
		return
	}
	stack := []*opf.SpanNode{doc.root}
	for len(stack) > 0 {
		if ctx.Err() != nil {
			return
		}
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for i := len(n.Kids) - 1; i >= 0; i-- {
			stack = append(stack, n.Kids[i])
		}

		switch strings.ToLower(n.Name.Local) {
		case "img":
			if src, ok := n.AttrByLocal("", "src"); ok {
				ins.checkXHTMLLocalReference(ctx, doc.item.ArchivePath, "image", src, manifestMedia)
			}
		case "link":
			rel, hasRel := n.AttrByLocal("", "rel")
			if hasRel && hasTokenFold(rel, "stylesheet") {
				if href, ok := n.AttrByLocal("", "href"); ok {
					ins.checkXHTMLLocalReference(ctx, doc.item.ArchivePath, "stylesheet", href, manifestMedia)
				}
			}
		case "a":
			if href, ok := n.AttrByLocal("", "href"); ok {
				ins.checkXHTMLLocalReference(ctx, doc.item.ArchivePath, "link", href, manifestMedia)
			}
		}
	}
}

func unsupportedXHTMLBase(ctx context.Context, root *opf.SpanNode) (string, bool) {
	stack := []*opf.SpanNode{root}
	for len(stack) > 0 {
		if ctx.Err() != nil {
			return "", false
		}
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for i := len(n.Kids) - 1; i >= 0; i-- {
			stack = append(stack, n.Kids[i])
		}
		if strings.EqualFold(n.Name.Local, "base") {
			if href, ok := n.AttrByLocal("", "href"); ok {
				return href, true
			}
			return "<base>", true
		}
		if base, ok := n.AttrByLocal(opf.XMLURI, "base"); ok {
			return base, true
		}
	}
	return "", false
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
