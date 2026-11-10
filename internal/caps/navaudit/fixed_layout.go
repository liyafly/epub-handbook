package navaudit

// 固定版式（SPEC §5.13）相关检查：viewport、宽高比、page-spread-* 用法，以及
// 字号检查用的“只被固定版式页面引用的样式表”集合。只读。

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// fixedLayoutOnlyStylesheets 返回只被固定版式 spine 页面链接的样式表。
// 只看 spine 里的页面：不在 spine 里的 nav 文档由阅读器自己呈现目录，不算可重排页面。
// 书里没有固定版式页面时返回空集合，不做额外扫描。
func (ins *inspector) fixedLayoutOnlyStylesheets(ctx context.Context, pkg *opf.Package) map[string]bool {
	fxl := opf.FixedLayoutXHTML(pkg)
	if len(fxl) == 0 {
		return nil
	}
	fxlRefs, otherRefs := map[string]bool{}, map[string]bool{}
	for _, item := range opf.SpineXHTMLItems(pkg) {
		if ctx.Err() != nil {
			return nil
		}
		if !hasEntry(ins.b, item.ArchivePath) {
			continue
		}
		raw, err := ins.b.CurrentContext(ctx, item.ArchivePath)
		if err != nil {
			continue
		}
		root, err := opf.ScanXHTMLSpanTreeContext(ctx, raw)
		if err != nil {
			continue
		}
		for _, n := range root.Walk() {
			if !strings.EqualFold(n.Name.Local, "link") {
				continue
			}
			rel, _ := n.AttrByLocal("", "rel")
			href, ok := n.AttrByLocal("", "href")
			if !ok || !hasTokenFold(rel, "stylesheet") || pypath.IsExternalURI(href) {
				continue
			}
			target, err := pypath.ResolveRelativePath(item.ArchivePath, pypath.URLSplit(href).Path)
			if err != nil {
				continue
			}
			if fxl[item.ArchivePath] {
				fxlRefs[target] = true
			} else {
				otherRefs[target] = true
			}
		}
	}
	out := map[string]bool{}
	for p := range fxlRefs {
		if !otherRefs[p] {
			out[p] = true
		}
	}
	return out
}

// parseViewport 解析 viewport meta 的 width / height（CSS 像素）。
func parseViewport(content string) (w, h float64, ok bool) {
	hasW, hasH := false, false
	for part := range strings.FieldsFuncSeq(content, func(r rune) bool { return r == ',' || r == ';' }) {
		key, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		n, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(value), "px"), 64)
		if err != nil || n <= 0 {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "width":
			w, hasW = n, true
		case "height":
			h, hasH = n, true
		}
	}
	return w, h, hasW && hasH
}

// checkFixedLayout 检查：
//   - 固定版式页面缺 viewport meta，或缺 width/height → error；
//   - 各页宽高比不一致 → warn；
//   - page-spread-* 用在非固定版式 spine 项上 → warn。
func (ins *inspector) checkFixedLayout(ctx context.Context, pkg *opf.Package) {
	byID := make(map[string]opf.ManifestItem, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		byID[item.ID] = item
	}
	ratios := map[string]int{}
	var ratioOrder []string
	for _, ref := range pkg.Spine {
		if ctx.Err() != nil {
			return
		}
		item, ok := byID[ref.IDRef]
		if !ok || item.ArchivePath == "" {
			continue
		}
		fixed := pkg.IsFixedLayout(ref)
		if !fixed {
			props := strings.Fields(ref.Properties)
			if slices.Contains(props, "page-spread-left") || slices.Contains(props, "page-spread-right") || slices.Contains(props, "rendition:page-spread-center") {
				ins.addFinding("warn", "page-spread-* is used on a spine item that is not fixed layout", item.ArchivePath, "page-spread-on-reflowable")
				ins.addSkill("epub-audit", "warn")
			}
			continue
		}
		if !strings.EqualFold(item.MediaType, "application/xhtml+xml") || !hasEntry(ins.b, item.ArchivePath) {
			continue
		}
		raw, err := ins.b.CurrentContext(ctx, item.ArchivePath)
		if err != nil {
			continue
		}
		root, err := opf.ScanXHTMLSpanTreeContext(ctx, raw)
		if err != nil {
			continue // 解析失败已由 XHTML 检查报告。
		}
		content, found := "", false
		if head := root.ChildByAnyNS("head"); head != nil {
			for _, n := range head.Kids {
				if name, _ := n.AttrByLocal("", "name"); strings.EqualFold(n.Name.Local, "meta") && strings.EqualFold(name, "viewport") {
					content, found = n.AttrByLocal("", "content")
					break
				}
			}
		}
		if !found {
			ins.addFinding("error", "Fixed-layout page has no viewport meta", item.ArchivePath, "fxl-viewport-missing")
			ins.addSkill("epub-audit", "error")
			continue
		}
		w, h, ok := parseViewport(content)
		if !ok {
			ins.addFinding("error", "Fixed-layout viewport meta lacks a numeric width and height: "+content, item.ArchivePath, "fxl-viewport-size-missing")
			ins.addSkill("epub-audit", "error")
			continue
		}
		key := strconv.FormatFloat(math.Round(w/h*1000)/1000, 'f', 3, 64)
		if ratios[key] == 0 {
			ratioOrder = append(ratioOrder, key)
		}
		ratios[key]++
	}
	if len(ratioOrder) > 1 {
		parts := make([]string, 0, len(ratioOrder))
		for _, k := range ratioOrder {
			parts = append(parts, fmt.Sprintf("%s x%d", k, ratios[k]))
		}
		ins.addFinding("warn", "Fixed-layout pages use different viewport aspect ratios (width/height): "+strings.Join(parts, ", "), ins.opfPath, "fxl-aspect-ratio-mixed")
		ins.addSkill("epub-audit", "warn")
	}
}
