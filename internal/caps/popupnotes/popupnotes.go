// Package popupnotes 移植 epub.notes.popup.normalize 的执行面
// （scripts/validate_popup_notes.py 的弹注校验规则），只读。
//
// 错误措辞、触发顺序与退出码语义逐字对齐 Python oracle：有错误 → 逐条
// "ERROR: {msg}" 且退出码 1；通过 → stdout "popup note validation ok"。
// Python 侧 --epub 模式把容器解到临时目录后只扫 OEBPS/Text/*.xhtml，
// 错误消息里的路径前缀是临时目录；Go 侧直接用 zip 路径（OEBPS/Text/…），
// parity 比对时按路径前缀归一。
package popupnotes

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// CapabilityID 是本能力的契约 id。
const CapabilityID = "epub.notes.popup.normalize"

// Params 是本能力的参数。
type Params struct {
	// LegacyReport 输出与 Python stderr 行一致的 findings 列表。
	LegacyReport bool
}

type violation struct {
	msg string
}

type iconRef struct {
	source string // XHTML 的 zip 路径
	src    string // img@src 原文
}

// Run 执行弹注校验（只读）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	res := report.Result{Capability: CapabilityID, Status: report.StatusComplete}
	var errs []violation

	textFiles := textFiles(b)
	var iconRefs []iconRef
	foundNotes := false
	noterefCount := 0

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
			if duokanMode {
				if !classTokens(noteList).contains("duokan-footnote-content") {
					errs = append(errs, violation{fmt.Sprintf("%s: Duokan fallback requires ol.duokan-footnote-content", fp)})
				}
				for _, a := range noterefs {
					if !classTokens(a).contains("duokan-footnote") {
						errs = append(errs, violation{fmt.Sprintf("%s: Duokan fallback noteref missing class=duokan-footnote", fp)})
					}
				}
				for _, li := range footnoteItems {
					if !classTokens(li).contains("duokan-footnote-item") {
						errs = append(errs, violation{fmt.Sprintf("%s: Duokan fallback li missing class=duokan-footnote-item", fp)})
					}
					if classTokens(li).contains("duokan-footnote-content") {
						errs = append(errs, violation{fmt.Sprintf("%s: duokan-footnote-content must not be on li", fp)})
					}
				}
			}
		}
	}

	// manifest 图标校验（发现过弹注才执行，与 Python 一致）。
	if foundNotes {
		validateManifest(b, textFiles, iconRefs, &errs)
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
		"noterefs":   noterefCount,
		"violations": len(errs),
		"text_files": len(textFiles),
	}
	if p.LegacyReport {
		lines := make([]string, 0, len(errs))
		for _, v := range errs {
			lines = append(lines, "ERROR: "+v.msg)
		}
		if len(lines) == 0 {
			lines = append(lines, "popup note validation ok")
		}
		res.Facts["legacyReport"] = map[string]any{"lines": lines}
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
func validateManifest(b *book.Book, textFiles []string, iconRefs []iconRef, errs *[]violation) {
	fail := func(msg string) { *errs = append(*errs, violation{msg}) }
	opfPath := findOPFPath(b)
	if opfPath == "" {
		fail("OEBPS: OPF package document not found")
		return
	}
	raw, err := b.Current(opfPath)
	if err != nil {
		fail(fmt.Sprintf("XML parse failed: %s: %v", opfPath, err))
		return
	}
	pkgDoc, perr := parseXHTML(raw)
	if perr != nil {
		fail(fmt.Sprintf("XML parse failed: %s: %v", opfPath, perr))
		return
	}
	var manifest *element
	for _, el := range pkgDoc.elements {
		if el.local == "manifest" {
			manifest = el
			break
		}
	}
	manifestItems := map[string]*element{}
	if manifest != nil {
		for _, it := range manifest.kids {
			if it.local != "item" || it.attrs["href"] == "" {
				continue
			}
			manifestItems[pyNormPath(pyUnquote(it.attrs["href"]))] = it
		}
	}
	opfDir := pyDirname(opfPath)
	for _, ir := range iconRefs {
		href, ok := resolveLocalIconHref(opfDir, ir.source, ir.src, errs)
		if !ok {
			continue
		}
		item, found := manifestItems[href]
		if !found {
			fail(fmt.Sprintf("%s: manifest must include noteref icon %s", opfPath, href))
		} else if !strings.HasPrefix(item.attrs["media-type"], "image/") {
			fail(fmt.Sprintf("%s: noteref icon %s must be image media-type", opfPath, href))
		}
		if !b.Has(normJoin(opfDir, href)) {
			fail(fmt.Sprintf("%s: noteref icon missing on disk: %s", opfDir, href))
		}
	}
}

// resolveLocalIconHref 对齐 resolve_local_icon_href；成功返回相对 OPF 目录的
// 目标路径。
func resolveLocalIconHref(opfDir, source, src string, errs *[]violation) (string, bool) {
	fail := func(msg string) (string, bool) {
		*errs = append(*errs, violation{msg})
		return "", false
	}
	scheme, netloc, p := pyURLSplit(src)
	prefix := source + ": noteref img"
	if scheme != "" || netloc != "" {
		return fail(fmt.Sprintf("%s src must be a local EPUB resource: %s", prefix, src))
	}
	if p == "" {
		return fail(fmt.Sprintf("%s src missing local path", prefix))
	}
	if opfDir == "" || !strings.HasPrefix(source, opfDir+"/") {
		return fail(fmt.Sprintf("%s source XHTML is outside OPF directory", prefix))
	}
	sourceRel := strings.TrimPrefix(source, opfDir+"/")
	target := pyNormPath(pyJoin(pyDirname(sourceRel), pyUnquote(p)))
	if target == "." || strings.HasPrefix(target, "../") || strings.HasPrefix(target, "/") {
		return fail(fmt.Sprintf("%s src escapes OPF directory: %s", prefix, src))
	}
	return target, true
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

// textFiles 对齐 --epub 模式的扫描面：仅 OEBPS/Text/*.xhtml（排序）。
func textFiles(b *book.Book) []string {
	var out []string
	for _, name := range b.Names() {
		if strings.HasPrefix(name, "OEBPS/Text/") && strings.HasSuffix(name, ".xhtml") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// findOPFPath 对齐 find_opf：container rootfile（需存在）→ OEBPS/package.opf
// → OEBPS 下字典序第一个 *.opf。
func findOPFPath(b *book.Book) string {
	if raw, err := b.Current(opfContainerPath); err == nil {
		if p, err := findRootfile(raw); err == nil && p != "" && b.Has(p) {
			return p
		}
	}
	if b.Has("OEBPS/package.opf") {
		return "OEBPS/package.opf"
	}
	var cands []string
	for _, name := range b.Names() {
		rest := strings.TrimPrefix(name, "OEBPS/")
		if strings.HasPrefix(name, "OEBPS/") && !strings.Contains(rest, "/") && strings.HasSuffix(name, ".opf") {
			cands = append(cands, name)
		}
	}
	sort.Strings(cands)
	if len(cands) > 0 {
		return cands[0]
	}
	return ""
}

const opfContainerPath = "META-INF/container.xml"

// findRootfile 从 container.xml 取 rootfile@full-path。
func findRootfile(container []byte) (string, error) {
	doc, err := parseXHTML(container)
	if err != nil {
		return "", err
	}
	var rootfile *element
	for _, el := range doc.elements {
		if el.local == "rootfile" {
			rootfile = el
			break
		}
	}
	if rootfile == nil {
		return "", fmt.Errorf("no rootfile")
	}
	return rootfile.attrs["full-path"], nil
}

// pyURLSplit 对齐 urllib.parse.urlsplit 的相关投影（C0+空格首尾剥离、
// \t\r\n 全文移除、scheme/netloc/path 切分）。
func pyURLSplit(raw string) (scheme, netloc, pathPart string) {
	const c0OrSpace = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f "
	u := strings.Trim(raw, c0OrSpace)
	u = strings.ReplaceAll(u, "\t", "")
	u = strings.ReplaceAll(u, "\r", "")
	u = strings.ReplaceAll(u, "\n", "")
	rest := u
	if i := strings.IndexByte(rest, ':'); i > 0 && isASCIILetter(rest[0]) {
		ok := true
		for j := 0; j < i; j++ {
			c := rest[j]
			if !isASCIILetter(c) && !(c >= '0' && c <= '9') && c != '+' && c != '-' && c != '.' {
				ok = false
				break
			}
		}
		if ok {
			scheme = strings.ToLower(rest[:i])
			rest = rest[i+1:]
		}
	}
	if strings.HasPrefix(rest, "//") {
		j := 2
		for j < len(rest) && rest[j] != '/' && rest[j] != '?' && rest[j] != '#' {
			j++
		}
		netloc = rest[2:j]
		rest = rest[j:]
	}
	if k := strings.IndexAny(rest, "?#"); k >= 0 {
		rest = rest[:k]
	}
	return scheme, netloc, rest
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// pyUnquote 对齐 urllib.parse.unquote（UTF-8、errors=replace）。
func pyUnquote(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] == '%' && i+2 < len(s)+1 && i+2 <= len(s) && isHexByte(s[i+1]) && isHexByte(s[i+2]) {
			out = append(out, hexVal(s[i+1])<<4|hexVal(s[i+2]))
			i += 3
			continue
		}
		out = append(out, s[i])
		i++
	}
	return string(bytesToValidUTF8(out))
}

func isHexByte(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func hexVal(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	default:
		return b - 'A' + 10
	}
}

func bytesToValidUTF8(b []byte) []byte {
	return []byte(strings.ToValidUTF8(string(b), "\uFFFD"))
}

// ---- 路径工具（posixpath 对齐） ----

func pyDirname(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

func pyJoin(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "/" + b
}

// pyNormPath 对齐 posixpath.normpath（相对路径形态）。
func pyNormPath(p string) string {
	parts := strings.Split(p, "/")
	var out []string
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
				continue
			}
			out = append(out, part)
		default:
			out = append(out, part)
		}
	}
	joined := strings.Join(out, "/")
	if strings.HasPrefix(p, "/") && !strings.HasPrefix(joined, "/") {
		return "/" + joined
	}
	if joined == "" {
		return "."
	}
	return joined
}

// normJoin 对齐 OPF 目录与相对 href 的容器路径拼接。
func normJoin(base, href string) string {
	clean := href
	if i := strings.IndexByte(clean, '#'); i >= 0 {
		clean = clean[:i]
	}
	if base == "" {
		return pyNormPath(clean)
	}
	return pyNormPath(base + "/" + clean)
}
