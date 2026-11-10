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
	// navTypes 是本文档里 nav 元素的 epub:type 词（toc、landmarks、page-list…）。
	navTypes map[string]bool
	// hasPagebreak 表示本文档有 epub:type="pagebreak" 或 role="doc-pagebreak" 的页码标记。
	hasPagebreak bool
}

type xhtmlReference struct {
	kind string
	raw  string
	// nav 非空时，链接位于该类型（landmarks / page-list）的 nav 里。
	nav string
}

// epubTypes 返回元素 epub:type 的分词。
func epubTypes(n *opf.SpanNode) []string {
	v, ok := n.AttrByLocal(opf.OPSURI, "type")
	if !ok {
		v, _ = n.AttrByLocal("", "epub:type")
	}
	return strings.Fields(v)
}

// enclosingNavType 返回链接所在的 landmarks / page-list nav 类型；不在其中时为空。
func enclosingNavType(n *opf.SpanNode) string {
	for p := n.Parent; p != nil; p = p.Parent {
		if !strings.EqualFold(p.Name.Local, "nav") {
			continue
		}
		for _, t := range epubTypes(p) {
			if t == "landmarks" || t == "page-list" {
				return t
			}
		}
		return ""
	}
	return ""
}

// scanXHTMLDocument verifies local references and keeps only IDs/reference
// strings for cross-document fragment checks. It releases the full span tree
// after each document instead of retaining every XHTML tree in memory.
func (ins *inspector) scanXHTMLDocument(ctx context.Context, item opf.ManifestItem, root *opf.SpanNode, manifestMedia map[string]map[string]struct{}) xhtmlDocument {
	doc := xhtmlDocument{item: item, ids: make(map[string]struct{}), navTypes: map[string]bool{}}
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

		if !doc.hasPagebreak {
			for _, t := range epubTypes(n) {
				if t == "pagebreak" {
					doc.hasPagebreak = true
				}
			}
			if role, ok := n.AttrByLocal("", "role"); ok && hasTokenFold(role, "doc-pagebreak") {
				doc.hasPagebreak = true
			}
		}
		switch strings.ToLower(n.Name.Local) {
		case "nav":
			for _, t := range epubTypes(n) {
				doc.navTypes[t] = true
			}
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
				doc.references = append(doc.references, xhtmlReference{kind: "link", raw: href, nav: enclosingNavType(n)})
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
		ins.checkXHTMLLocalReference(ctx, doc.item.ArchivePath, ref, manifestMedia)
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
				ins.checkXHTMLFragmentReference(ctx, doc.item.ArchivePath, ref, idsByPath, manifestXHTML)
			}
		}
	}
}

func (ins *inspector) checkXHTMLFragmentReference(ctx context.Context, source string, ref xhtmlReference, idsByPath map[string]map[string]struct{}, manifestXHTML map[string]struct{}) {
	raw := ref.raw
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
		if ref.nav != "" {
			ins.addXHTMLReferenceFinding("error", "Nav "+ref.nav+" link target ID is missing", "nav-"+ref.nav+"-target-missing", source, raw, target+"#"+fragment)
			ins.addSkill("epub-audit", "error")
			return
		}
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

func (ins *inspector) checkXHTMLLocalReference(ctx context.Context, source string, ref xhtmlReference, manifestMedia map[string]map[string]struct{}) {
	kind, raw := ref.kind, ref.raw
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
		if ref.nav != "" {
			ins.addXHTMLReferenceFinding("error", "Nav "+ref.nav+" link target is missing from the ZIP archive", "nav-"+ref.nav+"-target-missing", source, raw, target)
			ins.addSkill("epub-audit", "error")
			return
		}
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

// checkNavLandmarksAndPageList 依据已扫描的文档检查：
//   - EPUB 3 的 nav 文档没有 landmarks → info；
//   - 正文有页码标记但 nav 没有 page-list，或有 page-list 但正文没有页码标记 → warn。
func (ins *inspector) checkNavLandmarksAndPageList(pkg *opf.Package, documents []xhtmlDocument) {
	nav, ok := pkg.NavItem()
	if !ok || nav.ArchivePath == "" {
		return
	}
	var navDoc *xhtmlDocument
	pagebreaks := false
	for i := range documents {
		if documents[i].item.ArchivePath == nav.ArchivePath && navDoc == nil {
			navDoc = &documents[i]
		}
		if documents[i].hasPagebreak {
			pagebreaks = true
		}
	}
	if navDoc == nil {
		return // nav 读不到或解析失败，已有对应 finding。
	}
	if strings.HasPrefix(pkg.Version, "3") && !navDoc.navTypes["landmarks"] {
		ins.addFinding("info", "Nav document has no landmarks nav", nav.ArchivePath, "nav-landmarks-missing")
	}
	pageList := navDoc.navTypes["page-list"]
	switch {
	case pagebreaks && !pageList:
		ins.addFinding("warn", "XHTML has pagebreak markers but the nav document has no page-list", nav.ArchivePath, "nav-page-list-missing")
		ins.addSkill("epub-audit", "warn")
	case pageList && !pagebreaks && len(opf.FixedLayoutXHTML(pkg)) < len(opf.SpineXHTMLPaths(pkg)):
		// 整本固定版式的 page-list 直接指向页面文件，不需要页码标记（SPEC §5.13）。
		ins.addFinding("warn", "Nav page-list exists but no XHTML has pagebreak markers", nav.ArchivePath, "nav-page-list-without-pagebreaks")
		ins.addSkill("epub-audit", "warn")
	}
}
