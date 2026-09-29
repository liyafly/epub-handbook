// Package popupnotes 移植 epub.notes.popup.normalize 的执行面
// （scripts/validate_popup_notes.py 的弹注校验规则），只读。
//
// 错误措辞与触发顺序沿用原校验器：每条违反项是一条 error finding，
// title 以 zip 路径（OEBPS/Text/…）开头；有 error 时 status=failed（退出码 1）。
package popupnotes

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// CapabilityID 是本能力的契约 id。
const CapabilityID = "epub.notes.popup.normalize"

// Params 是本能力的参数。
type Params struct{}

type violation struct {
	msg string
}

type iconRef struct {
	source string // XHTML 的 zip 路径
	src    string // img@src 原文
}

// Run 执行弹注校验（只读）。
func Run(ctx context.Context, b *book.Book, _ Params) (report.Result, error) {
	res := report.Result{Capability: CapabilityID, Status: report.StatusComplete}
	var errs []violation

	textFiles, pkg, textFilesErr := textFiles(b)
	if textFilesErr != nil {
		errs = append(errs, violation{fmt.Sprintf("EPUB package XHTML scan failed: %v", textFilesErr)})
	}
	var iconRefs []iconRef
	foundNotes := false
	noterefCount := 0
	duokanViolations := 0

	for _, fp := range textFiles {
		raw, err := b.Current(fp)
		if err != nil {
			errs = append(errs, violation{fmt.Sprintf("XML parse failed: %s: %v", fp, err)})
			continue
		}
		doc, perr := parseXHTML(raw)
		if perr != nil {
			errs = append(errs, violation{fmt.Sprintf("XML parse failed: %s: %v", fp, perr)})
			continue
		}

		// collect_ids（文档序；Python ids[id] = elem 最后一次出现生效）。
		seenID := map[string]bool{}
		for _, el := range doc.idOrder {
			if seenID[el.id] {
				errs = append(errs, violation{fmt.Sprintf("%s: duplicate id: %s", fp, el.id)})
			}
			seenID[el.id] = true
		}

		// noterefs / footnote_asides 收集（role 为整串精确比较，epub:type 按分词）。
		var noterefs []*element
		var footnoteAsides []*element
		for _, el := range doc.elements {
			switch {
			case el.local == "a" && (typed(el, "noteref") || el.role == "doc-noteref"):
				noterefs = append(noterefs, el)
			case el.local == "aside" && (typed(el, "footnote") || el.role == "doc-footnote"):
				footnoteAsides = append(footnoteAsides, el)
			}
		}
		if len(noterefs) == 0 && len(footnoteAsides) == 0 {
			continue
		}
		foundNotes = true
		noterefCount += len(noterefs)

		if len(footnoteAsides) != 1 {
			errs = append(errs, violation{fmt.Sprintf("%s: files with notes must have exactly one grouped footnote aside", fp)})
		}
		var lists []*element // aside 内的 ol.footnote-list
		if len(footnoteAsides) > 0 {
			aside := footnoteAsides[0]
			if !typed(aside, "footnote") {
				errs = append(errs, violation{fmt.Sprintf("%s: footnote aside must have epub:type=footnote", fp)})
			}
			if aside.role != "doc-footnote" {
				errs = append(errs, violation{fmt.Sprintf("%s: footnote aside must have role=doc-footnote", fp)})
			}
			for _, o := range aside.descendants("ol") {
				if classTokens(o).contains("footnote-list") {
					lists = append(lists, o)
				}
			}
			if len(lists) != 1 {
				errs = append(errs, violation{fmt.Sprintf("%s: footnote aside must contain exactly one ol.footnote-list", fp)})
			}
		}

		targetIDs := map[string]bool{}
		noteIDs := map[string]bool{}
		for _, anchor := range noterefs {
			if anchor.id != "" {
				noteIDs[anchor.id] = true
			}
			targetID, iconSrc := validateNoteref(fp, doc, anchor, &errs)
			if targetID != "" {
				targetIDs[targetID] = true
			}
			if iconSrc != "" {
				iconRefs = append(iconRefs, iconRef{fp, iconSrc})
			}
		}

		if len(lists) > 0 {
			noteList := lists[0]
			var footnoteItems []*element
			for _, li := range noteList.descendants("li") {
				if classTokens(li).contains("footnote-item") {
					footnoteItems = append(footnoteItems, li)
				}
			}
			itemIDs := map[string]bool{}
			for _, li := range footnoteItems {
				if li.id != "" {
					itemIDs[li.id] = true
				}
			}
			if !subset(targetIDs, itemIDs) {
				errs = append(errs, violation{fmt.Sprintf("%s: every noteref target must be in ol.footnote-list", fp)})
			}
			var backlinks []*element
			for _, li := range footnoteItems {
				for _, a := range li.descendants("a") {
					if typed(a, "backlink") || a.role == "doc-backlink" {
						backlinks = append(backlinks, a)
					}
				}
			}
			if len(backlinks) < len(targetIDs) {
				errs = append(errs, violation{fmt.Sprintf("%s: each footnote item should contain a backlink", fp)})
			}
			for _, bl := range backlinks {
				validateBacklink(fp, bl, noteIDs, &errs)
			}
			duokanMode := classTokens(noteList).contains("duokan-footnote-content")
			for _, a := range noterefs {
				if classTokens(a).contains("duokan-footnote") {
					duokanMode = true
				}
			}
			for _, li := range footnoteItems {
				if classTokens(li).contains("duokan-footnote-item") {
					duokanMode = true
				}
			}
			duokanErrorsBefore := len(errs)
			if duokanMode {
				if !classTokens(noteList).contains("duokan-footnote-content") {
					errs = append(errs, violation{fmt.Sprintf("%s: Duokan fallback requires ol.duokan-footnote-content", fp)})
				}
				for _, a := range noterefs {
					if !classTokens(a).contains("duokan-footnote") {
						errs = append(errs, violation{fmt.Sprintf("%s: Duokan fallback noteref missing class=duokan-footnote", fp)})
					}
				}
			}
			for _, li := range footnoteItems {
				if duokanMode && !classTokens(li).contains("duokan-footnote-item") {
					errs = append(errs, violation{fmt.Sprintf("%s: Duokan fallback li missing class=duokan-footnote-item", fp)})
				}
				if classTokens(li).contains("duokan-footnote-content") {
					errs = append(errs, violation{fmt.Sprintf("%s: duokan-footnote-content must not be on li", fp)})
				}
			}
			duokanViolations += len(errs) - duokanErrorsBefore
		}
	}

	// manifest 图标校验（发现过弹注才执行，与 Python 一致）。
	if foundNotes {
		validateManifest(b, pkg, iconRefs, &errs)
	}

	if len(errs) > 0 {
		res.Status = report.StatusFailed
		for _, v := range errs {
			res.Findings = append(res.Findings, report.Finding{
				Level: "error", ID: "popupnotes", Title: v.msg,
			})
		}
	}
	res.Facts = map[string]any{
		"noterefs":           noterefCount,
		"violations":         len(errs),
		"standardViolations": len(errs) - duokanViolations,
		"text_files":         len(textFiles),
	}
	return res, nil
}

// validateNoteref 对齐 validate_noteref；返回 target 片段 id 与图标 src。
func validateNoteref(fp string, doc *doc, anchor *element, errs *[]violation) (string, string) {
	prefix := fp + ": noteref"
	require := func(cond bool, msg string) {
		if !cond {
			*errs = append(*errs, violation{msg})
		}
	}
	require(anchor.id != "", prefix+" missing id")
	targetID := hrefFragment(anchor.attrs["href"])
	require(targetID != "", prefix+" href must be same-file fragment")
	require(typed(anchor, "noteref"), prefix+" must have epub:type=noteref")
	require(anchor.role == "doc-noteref", prefix+" must have role=doc-noteref")
	require(classTokens(anchor).contains("noteref-icon"), prefix+" must include class=noteref-icon")
	images := anchor.descendants("img")
	require(len(images) == 1, prefix+" must contain exactly one img icon")
	iconSrc := ""
	if len(images) > 0 {
		require(images[0].hasAttr("alt"), prefix+" img icon must have alt")
		iconSrc = images[0].attrs["src"]
		require(iconSrc != "", prefix+" img icon must have src")
	}
	if targetID == "" {
		return "", iconSrc
	}
	target, found := doc.byID(targetID)
	require(found, prefix+" target missing: #"+targetID)
	if found {
		require(target.local == "li", prefix+" target must be li: #"+targetID)
		require(classTokens(target).contains("footnote-item"), prefix+" target li must have class=footnote-item")
	}
	return targetID, iconSrc
}

// validateBacklink 对齐 validate_backlink。
func validateBacklink(fp string, bl *element, noteIDs map[string]bool, errs *[]violation) {
	prefix := fp + ": backlink"
	require := func(cond bool, msg string) {
		if !cond {
			*errs = append(*errs, violation{msg})
		}
	}
	targetID := hrefFragment(bl.attrs["href"])
	require(targetID != "", prefix+" href must be same-file fragment")
	require(typed(bl, "backlink"), prefix+" must have epub:type=backlink")
	require(bl.role == "doc-backlink", prefix+" must have role=doc-backlink")
	if targetID != "" {
		require(noteIDs[targetID], prefix+" target must be a noteref id: #"+targetID)
	}
}

// validateManifest 对齐 validate_manifest：图标引用按收集顺序逐个解析。
func validateManifest(b *book.Book, pkg *opf.Package, iconRefs []iconRef, errs *[]violation) {
	fail := func(msg string) { *errs = append(*errs, violation{msg}) }
	if pkg == nil {
		fail("OEBPS: OPF package document not found")
		return
	}
	manifestItems := make(map[string]opf.ManifestItem, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		if item.ArchivePath != "" {
			manifestItems[item.ArchivePath] = item
		}
	}
	opfDir := pypath.Dirname(pkg.Path)
	for _, ir := range iconRefs {
		resolved, ok := resolveLocalIconHref(pkg.Path, ir.source, ir.src, errs)
		if !ok {
			continue
		}
		href, target := resolved.manifestHref, resolved.archivePath
		item, found := manifestItems[target]
		if !found {
			fail(fmt.Sprintf("%s: manifest must include noteref icon %s", pkg.Path, href))
		} else if !strings.HasPrefix(item.MediaType, "image/") {
			fail(fmt.Sprintf("%s: noteref icon %s must be image media-type", pkg.Path, href))
		}
		if !b.Has(target) {
			fail(fmt.Sprintf("%s: noteref icon missing on disk: %s", opfDir, href))
		}
	}
}

type resolvedIconHref struct {
	manifestHref string
	archivePath  string
}

// resolveLocalIconHref resolves relative to the source XHTML in container space.
func resolveLocalIconHref(opfPath, source, src string, errs *[]violation) (resolvedIconHref, bool) {
	fail := func(msg string) (resolvedIconHref, bool) {
		*errs = append(*errs, violation{msg})
		return resolvedIconHref{}, false
	}
	parts := pypath.URLSplit(src)
	prefix := source + ": noteref img"
	if parts.Scheme != "" || parts.Netloc != "" {
		return fail(fmt.Sprintf("%s src must be a local EPUB resource: %s", prefix, src))
	}
	if parts.Path == "" {
		return fail(fmt.Sprintf("%s src missing local path", prefix))
	}
	target, err := pypath.ResolveRelativePath(source, parts.Path)
	if err != nil {
		return fail(fmt.Sprintf("%s src escapes container root: %s", prefix, src))
	}
	return resolvedIconHref{
		manifestHref: pypath.RelativePath(opfPath, target),
		archivePath:  target,
	}, true
}

// ---- XHTML 解析投影 ----

type element struct {
	local  string
	id     string
	attrs  map[string]string
	epubT  string
	role   string
	class  string
	text   string
	kids   []*element
	parent *element
}

func (e *element) hasAttr(name string) bool {
	_, ok := e.attrs[name]
	return ok
}

func (e *element) descendants(name string) []*element {
	var out []*element
	var walk func(*element)
	walk = func(el *element) {
		for _, k := range el.kids {
			if name == "" || k.local == name {
				out = append(out, k)
			}
			walk(k)
		}
	}
	walk(e)
	return out
}

type doc struct {
	root     *element
	elements []*element
	idOrder  []*element          // 有 id 的元素，文档序
	idElems  map[string]*element // id → 最后出现的元素（Python ids[id] = elem）
}

func (d *doc) byID(id string) (*element, bool) {
	el, ok := d.idElems[id]
	return el, ok
}

func parseXHTML(data []byte) (*doc, error) {
	d := xml.NewDecoder(strings.NewReader(string(data)))
	d.Strict = true
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) { return input, nil }
	out := &doc{idElems: map[string]*element{}}
	var stack []*element
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			el := &element{
				local: t.Name.Local,
				attrs: map[string]string{},
			}
			for _, a := range t.Attr {
				el.attrs[a.Name.Local] = a.Value
				switch {
				case a.Name.Space == opsURI && a.Name.Local == "type":
					if el.epubT == "" {
						el.epubT = a.Value
					}
				case a.Name.Local == "epub:type" && el.epubT == "":
					el.epubT = a.Value
				case a.Name.Local == "role":
					el.role = a.Value
				case a.Name.Local == "class":
					el.class = a.Value
				case a.Name.Local == "id":
					el.id = a.Value
				}
			}
			if el.id != "" {
				out.idOrder = append(out.idOrder, el)
				out.idElems[el.id] = el
			}
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.kids = append(p.kids, el)
				el.parent = p
			} else if out.root == nil {
				out.root = el
			}
			stack = append(stack, el)
			out.elements = append(out.elements, el)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(t)
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if out.root == nil {
		return nil, fmt.Errorf("no root element")
	}
	return out, nil
}

const opsURI = "http://www.idpf.org/2007/ops"

// ---- 小工具 ----

type tokenSet map[string]bool

func (t tokenSet) contains(v string) bool { return t[v] }

func tokenSetOf(s string) tokenSet {
	out := tokenSet{}
	for _, tok := range strings.Fields(s) {
		out[tok] = true
	}
	return out
}

// typed 对齐 Python typed()：token ∈ epub:type 分词。
func typed(el *element, token string) bool {
	return tokenSetOf(el.epubT).contains(token)
}

func classTokens(el *element) tokenSet {
	return tokenSetOf(el.class)
}

// hrefFragment 对齐 href_fragment：#x → x；其余 → 空串。
func hrefFragment(href string) string {
	if href == "" || !strings.HasPrefix(href, "#") || len(href) == 1 {
		return ""
	}
	return href[1:]
}

func subset(small, big map[string]bool) bool {
	for k := range small {
		if !big[k] {
			return false
		}
	}
	return true
}

// textFiles returns unique, existing XHTML paths in the OPF spine.
func textFiles(b *book.Book) ([]string, *opf.Package, error) {
	pkg, err := readPackage(b)
	if err != nil {
		return nil, nil, err
	}
	out := opf.SpineXHTMLPaths(pkg)
	selected := out[:0]
	for _, path := range out {
		if path != "" && b.Has(path) {
			selected = append(selected, path)
		}
	}
	sort.Strings(selected)
	return selected, pkg, nil
}

func readPackage(b *book.Book) (*opf.Package, error) {
	container, err := b.Current(opf.ContainerPath)
	if err != nil {
		return nil, fmt.Errorf("package document path was not found")
	}
	opfPath, err := opf.FindOPFPath(container)
	if err != nil || opfPath == "" {
		return nil, fmt.Errorf("package document path was not found")
	}
	opfData, err := b.Current(opfPath)
	if err != nil {
		return nil, fmt.Errorf("read package document %s: %w", opfPath, err)
	}
	pkg, err := opf.Parse(opfPath, opfData)
	if err != nil {
		return nil, fmt.Errorf("parse package document %s: %w", opfPath, err)
	}
	return pkg, nil
}
