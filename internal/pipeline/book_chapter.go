package pipeline

// epub book add-chapter：给书目录里的解包源加一章，同步 OPF manifest、spine、
// nav 的 toc 列表与 NCX 的 navMap（playOrder 重排）。
//
// 只做字节区间插入/替换，不重序列化任何文档。全部编辑先在内存里算好，任何一处
// 不能安全编辑就一个文件都不动。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
	"github.com/liyafly/epub-handbook/internal/zipfs"
)

// BookAddChapterOptions 是 epub book add-chapter 的输入。
type BookAddChapterOptions struct {
	Dir   string
	Title string
	// File 是新文件名（不含目录）；为空时用 <NN>-<slug>.xhtml。
	File string
	// After 是已有章节文件；为空时排在 spine 末尾。
	After string
}

// BookAddChapter 加一章，返回退出码。
func BookAddChapter(ctx context.Context, opts BookAddChapterOptions, bio BookIO) int {
	layout, err := resolveBookLayout(opts.Dir, readBookEnv())
	if err != nil {
		return bookExit(bio, err)
	}
	if len(layout.Volumes) > 0 {
		return bookExit(bio, usageErrorf("multi-volume workspace: run add-chapter inside each volume directory"))
	}
	res, err := addChapter(ctx, layout.EpubDir, opts)
	if err != nil {
		return bookExit(bio, err)
	}
	synced := "OPF manifest, spine, nav"
	if res.ncx {
		synced += ", NCX"
	}
	fmt.Fprintf(bio.out(), "Added %s (%s).\n", layout.displayPath(filepath.Join(layout.EpubDir, filepath.FromSlash(res.path))), synced)
	return ExitOK
}

type addChapterResult struct {
	path string // 容器内路径
	ncx  bool
}

func addChapter(ctx context.Context, epubDir string, opts BookAddChapterOptions) (addChapterResult, error) {
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		return addChapterResult{}, usageErrorf("add-chapter needs a non-empty --title")
	}
	read := func(archivePath string) ([]byte, error) {
		return readSmallFile(filepath.Join(epubDir, filepath.FromSlash(archivePath)))
	}
	container, err := read(opf.ContainerPath)
	if err != nil {
		return addChapterResult{}, err
	}
	opfPath, err := opf.FindOPFPath(container)
	if err != nil {
		return addChapterResult{}, err
	}
	opfData, err := read(opfPath)
	if err != nil {
		return addChapterResult{}, err
	}
	pkg, err := opf.Parse(opfPath, opfData)
	if err != nil {
		return addChapterResult{}, err
	}
	navItem, ok := pkg.NavItem()
	if !ok || navItem.ArchivePath == "" {
		return addChapterResult{}, errors.New("add-chapter: the OPF has no nav document; add the chapter by hand")
	}
	var chapters []opf.ManifestItem
	for _, item := range opf.SpineXHTMLItems(pkg) {
		if item.ArchivePath != navItem.ArchivePath {
			chapters = append(chapters, item)
		}
	}
	if len(chapters) == 0 {
		return addChapterResult{}, errors.New("add-chapter: the spine has no XHTML chapter to copy from")
	}
	last := chapters[len(chapters)-1]
	dir := path.Dir(last.ArchivePath)

	anchor := last
	if opts.After != "" {
		anchor, err = resolveAfterChapter(pkg, chapters, opts.After)
		if err != nil {
			return addChapterResult{}, err
		}
	}

	newPath, err := chapterFilePath(epubDir, pkg, chapters, dir, opts.File, title)
	if err != nil {
		return addChapterResult{}, err
	}
	newID := uniqueManifestID(pkg, "chapter-"+strings.TrimSuffix(path.Base(newPath), ".xhtml"))

	lastData, err := read(last.ArchivePath)
	if err != nil {
		return addChapterResult{}, err
	}
	page, err := chapterSkeleton(ctx, last.ArchivePath, lastData, pkg, title)
	if err != nil {
		return addChapterResult{}, err
	}

	changed := map[string][]byte{}
	if changed[opfPath], err = chapterOPFEdits(ctx, opfPath, opfData, anchor, opts.After != "", newID, newPath); err != nil {
		return addChapterResult{}, err
	}
	navData, err := read(navItem.ArchivePath)
	if err != nil {
		return addChapterResult{}, err
	}
	if changed[navItem.ArchivePath], err = chapterNavEdit(ctx, navItem.ArchivePath, navData, anchor.ArchivePath, opts.After != "", title, newPath); err != nil {
		return addChapterResult{}, err
	}
	res := addChapterResult{path: newPath}
	if ncxItem, ok := pkg.NCXItem(); ok && ncxItem.ArchivePath != "" {
		ncxData, err := read(ncxItem.ArchivePath)
		if err != nil {
			return addChapterResult{}, err
		}
		if changed[ncxItem.ArchivePath], err = chapterNCXEdit(ctx, ncxItem.ArchivePath, ncxData, anchor.ArchivePath, opts.After != "", title, newPath); err != nil {
			return addChapterResult{}, err
		}
		res.ncx = true
	}
	if err := ctx.Err(); err != nil {
		return addChapterResult{}, err
	}

	target := filepath.Join(epubDir, filepath.FromSlash(newPath))
	if isFile(target) {
		return addChapterResult{}, usageErrorf("chapter file already exists: %s", newPath)
	}
	if err := zipfs.WriteFileAtomic(target, page, 0o644); err != nil {
		return addChapterResult{}, err
	}
	for archivePath, data := range changed {
		file := filepath.Join(epubDir, filepath.FromSlash(archivePath))
		mode := os.FileMode(0o644)
		if info, err := os.Stat(file); err == nil {
			mode = info.Mode().Perm()
		}
		if err := zipfs.WriteFileAtomic(file, data, mode); err != nil {
			return addChapterResult{}, err
		}
	}
	return res, nil
}

// resolveAfterChapter 把 --after 解析成 spine 里的章节：容器内路径、相对 OPF 的路径
// 或（唯一的）文件名都可以。
func resolveAfterChapter(pkg *opf.Package, chapters []opf.ManifestItem, after string) (opf.ManifestItem, error) {
	after = strings.TrimPrefix(filepath.ToSlash(after), "./")
	var byBase []opf.ManifestItem
	for _, item := range chapters {
		if item.ArchivePath == after || item.ArchivePath == pkg.OPFDir()+after {
			return item, nil
		}
		if path.Base(item.ArchivePath) == after {
			byBase = append(byBase, item)
		}
	}
	switch len(byBase) {
	case 1:
		return byBase[0], nil
	case 0:
		return opf.ManifestItem{}, usageErrorf("--after %s is not a chapter in the spine", after)
	default:
		return opf.ManifestItem{}, usageErrorf("--after %s matches several spine files; give its path relative to the OPF", after)
	}
}

// chapterFilePath 选新文件的容器内路径，放在最后一章所在目录。
func chapterFilePath(epubDir string, pkg *opf.Package, chapters []opf.ManifestItem, dir, file, title string) (string, error) {
	taken := func(p string) bool {
		for _, item := range pkg.Manifest {
			if item.ArchivePath == p {
				return true
			}
		}
		return isFile(filepath.Join(epubDir, filepath.FromSlash(p))) || dirExists(filepath.Join(epubDir, filepath.FromSlash(p)))
	}
	if file != "" {
		if strings.ContainsAny(file, `/\`) || file == "." || file == ".." || !strings.HasSuffix(file, ".xhtml") {
			return "", usageErrorf("--file must be a plain file name ending in .xhtml: %s", file)
		}
		p := path.Join(dir, file)
		if taken(p) {
			return "", usageErrorf("chapter file already exists: %s", p)
		}
		return p, nil
	}
	next, width := 0, 2
	for _, item := range chapters {
		if path.Dir(item.ArchivePath) != dir {
			continue
		}
		digits, _, found := strings.Cut(path.Base(item.ArchivePath), "-")
		n, err := strconv.Atoi(digits)
		if !found || err != nil || digits == "" {
			continue
		}
		next = max(next, n+1)
		width = max(width, len(digits))
	}
	slug := chapterSlug(title)
	for {
		p := path.Join(dir, fmt.Sprintf("%0*d-%s.xhtml", width, next, slug))
		if !taken(p) {
			return p, nil
		}
		next++
	}
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// chapterSlug 只保留 ASCII 字母数字；没有可用字符时用 chapter。
func chapterSlug(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if b.Len() > 0 && !dash {
			b.WriteByte('-')
			dash = true
		}
		if b.Len() >= 40 {
			break
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "chapter"
	}
	return slug
}

func uniqueManifestID(pkg *opf.Package, base string) string {
	used := make(map[string]bool, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		used[item.ID] = true
	}
	return pypath.UniqueID(base, used)
}

// chapterSkeleton 生成新章 XHTML：骨架同起始书的 01-chapter.xhtml，样式表 link
// 原样照抄最后一章（同目录，href 不变），语言照抄其 html 元素。
func chapterSkeleton(ctx context.Context, lastPath string, lastData []byte, pkg *opf.Package, title string) ([]byte, error) {
	root, err := opf.ScanXHTMLSpanTreeContext(ctx, lastData)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", lastPath, err)
	}
	var links []string
	if head := root.ChildByAnyNS("head"); head != nil {
		for _, kid := range head.Kids {
			if kid.Name.Local != "link" {
				continue
			}
			rel, _ := kid.AttrByLocal("", "rel")
			if !strings.Contains(" "+strings.ToLower(rel)+" ", " stylesheet ") {
				continue
			}
			links = append(links, string(lastData[kid.Open.Start:kid.Open.End]))
		}
	}
	lang := ""
	if v, ok := root.AttrByLocal(opf.XMLURI, "lang"); ok {
		lang += ` xml:lang="` + pypath.EscapeAttribute(v) + `"`
	}
	if v, ok := root.AttrByLocal("", "lang"); ok {
		lang += ` lang="` + pypath.EscapeAttribute(v) + `"`
	}
	if lang == "" {
		if langs := pkg.Metadata["language"]; len(langs) > 0 && langs[0] != "" {
			lang = ` xml:lang="` + pypath.EscapeAttribute(langs[0]) + `"`
		}
	}
	t := xmlText(title)
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE html>\n")
	b.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"` + lang + ">\n")
	b.WriteString("<head>\n  <title>" + t + "</title>\n")
	for _, link := range links {
		b.WriteString("  " + link + "\n")
	}
	b.WriteString("</head>\n<body>\n  <section epub:type=\"chapter\">\n")
	b.WriteString("    <h1>" + t + "</h1>\n    <p>正文从这里开始。</p>\n  </section>\n</body>\n</html>\n")
	return []byte(b.String()), nil
}

// chapterOPFEdits 在锚点章节的 manifest item 之后插入新 item，在其 itemref 之后
// （默认：spine 最后一个 itemref 之后）插入新 itemref。
func chapterOPFEdits(ctx context.Context, opfPath string, data []byte, anchor opf.ManifestItem, after bool, id, newPath string) ([]byte, error) {
	if err := opf.EditableUTF8(data); err != nil {
		return nil, fmt.Errorf("%s: %w", opfPath, err)
	}
	root, err := opf.ScanSpanTreeContext(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", opfPath, err)
	}
	var itemNode *opf.SpanNode
	for _, n := range opf.ManifestNodes(root) {
		if v, _ := n.AttrByLocal("", "id"); v == anchor.ID {
			itemNode = n
		}
	}
	if itemNode == nil {
		return nil, fmt.Errorf("%s: manifest item %s not found", opfPath, anchor.ID)
	}
	spine := root.ChildByLocal(opf.OPFURI, "spine")
	if spine == nil {
		return nil, fmt.Errorf("%s: no spine", opfPath)
	}
	refs := spine.ChildrenByLocal(opf.OPFURI, "itemref")
	if len(refs) == 0 {
		return nil, fmt.Errorf("%s: spine has no itemref", opfPath)
	}
	refNode := refs[len(refs)-1]
	if after {
		refNode = nil
		for _, n := range refs {
			if v, _ := n.AttrByLocal("", "idref"); v == anchor.ID {
				refNode = n
			}
		}
		if refNode == nil {
			return nil, fmt.Errorf("%s: spine itemref for %s not found", opfPath, anchor.ID)
		}
	}
	href := pypath.RelativeURI(opfPath, newPath)
	item := "\n" + lineIndent(data, itemNode.Open.Start) + `<item id="` + pypath.EscapeAttribute(id) + `" href="` + pypath.EscapeAttribute(href) + `" media-type="application/xhtml+xml"/>`
	ref := "\n" + lineIndent(data, refNode.Open.Start) + `<itemref idref="` + pypath.EscapeAttribute(id) + `"/>`
	return editset.Apply(opfPath, data, []editset.Edit{
		editset.Insert(opfPath, int64(itemNode.End()), []byte(item)),
		editset.Insert(opfPath, int64(refNode.End()), []byte(ref)),
	})
}

// hrefTarget 把文档里的 href/src 解析成不带片段的容器内路径。
func hrefTarget(docPath, href string) string {
	if href == "" || pypath.IsExternalURI(href) {
		return ""
	}
	p, _, _ := strings.Cut(href, "#")
	if p == "" {
		return docPath
	}
	resolved, err := pypath.ResolveRelativePath(docPath, pypath.URLSplit(p).Path)
	if err != nil {
		return ""
	}
	return resolved
}

func findTOCNav(root *opf.SpanNode) *opf.SpanNode {
	for _, n := range root.Walk() {
		if n.Name.Local != "nav" {
			continue
		}
		t, _ := n.AttrByLocal(opf.OPSURI, "type")
		if t == "" {
			t, _ = n.AttrByLocal("", "epub:type")
		}
		for tok := range strings.FieldsSeq(t) {
			if tok == "toc" {
				return n
			}
		}
	}
	return nil
}

// chapterNavEdit 在 nav 的 toc 里加一条：默认追加到顶层 ol 末尾；after 时插在指向
// 锚点文件的最外层目录项（含其小节）之后，作为它的同级。
func chapterNavEdit(ctx context.Context, navPath string, data []byte, anchorPath string, after bool, title, newPath string) ([]byte, error) {
	if err := opf.EditableUTF8(data); err != nil {
		return nil, fmt.Errorf("%s: %w", navPath, err)
	}
	root, err := opf.ScanXHTMLSpanTreeContext(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", navPath, err)
	}
	toc := findTOCNav(root)
	if toc == nil {
		return nil, fmt.Errorf("%s: no nav epub:type=\"toc\"; add the chapter by hand", navPath)
	}
	var anchorLI *opf.SpanNode
	if after {
		// 小节链接（file#s1）去掉片段后也指向同一文件：取嵌套最浅的 li，
		// 深度相同取文档序最后一个。
		best := -1
		for _, n := range toc.Walk() {
			if n.Name.Local != "a" || n.Parent == nil || n.Parent.Name.Local != "li" {
				continue
			}
			if href, _ := n.AttrByLocal("", "href"); hrefTarget(navPath, href) == anchorPath {
				if d := ancestorDepth(n.Parent, toc, "li"); best < 0 || d <= best {
					anchorLI, best = n.Parent, d
				}
			}
		}
		if anchorLI == nil {
			return nil, usageErrorf("--after %s has no entry in the nav toc; retry without --after", path.Base(anchorPath))
		}
	} else {
		ol := toc.ChildByAnyNS("ol")
		if ol == nil {
			return nil, fmt.Errorf("%s: toc nav has no <ol>", navPath)
		}
		for _, li := range ol.Kids {
			if li.Name.Local == "li" {
				anchorLI = li
			}
		}
		if anchorLI == nil {
			return nil, fmt.Errorf("%s: toc list is empty; add the chapter by hand", navPath)
		}
	}
	href := pypath.RelativeURI(navPath, newPath)
	li := "\n" + lineIndent(data, anchorLI.Open.Start) + `<li><a href="` + pypath.EscapeAttribute(href) + `">` + xmlText(title) + `</a></li>`
	return editset.Apply(navPath, data, []editset.Edit{editset.Insert(navPath, int64(anchorLI.End()), []byte(li))})
}

// chapterNCXEdit 在 navMap 里加 navPoint，并把全部 navPoint 的 playOrder 按文档序重排为 1..n。
// after 时插在指向锚点文件的最外层 navPoint（含其子项）之后，作为它的同级。
func chapterNCXEdit(ctx context.Context, ncxPath string, data []byte, anchorPath string, after bool, title, newPath string) ([]byte, error) {
	if err := opf.EditableUTF8(data); err != nil {
		return nil, fmt.Errorf("%s: %w", ncxPath, err)
	}
	root, err := opf.ScanSpanTreeContext(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ncxPath, err)
	}
	navMap := root.ChildByAnyNS("navMap")
	if navMap == nil {
		return nil, fmt.Errorf("%s: no navMap", ncxPath)
	}
	for _, n := range root.Walk() {
		if n.Name.Local == "pageTarget" || n.Name.Local == "navTarget" {
			return nil, fmt.Errorf("%s: has pageList/navList playOrder entries; add the chapter by hand", ncxPath)
		}
	}
	var points []*opf.SpanNode
	for _, n := range navMap.Walk() {
		if n.Name.Local == "navPoint" {
			points = append(points, n)
		}
	}
	var anchorPoint *opf.SpanNode
	if after {
		best := -1
		for _, p := range points {
			content := p.ChildByAnyNS("content")
			if content == nil {
				continue
			}
			if src, _ := content.AttrByLocal("", "src"); hrefTarget(ncxPath, src) == anchorPath {
				if d := ancestorDepth(p, navMap, "navPoint"); best < 0 || d <= best {
					anchorPoint, best = p, d
				}
			}
		}
		if anchorPoint == nil {
			return nil, usageErrorf("--after %s has no navPoint in the NCX; retry without --after", path.Base(anchorPath))
		}
	} else {
		for _, p := range navMap.Kids {
			if p.Name.Local == "navPoint" {
				anchorPoint = p
			}
		}
		if anchorPoint == nil {
			return nil, fmt.Errorf("%s: navMap is empty; add the chapter by hand", ncxPath)
		}
	}
	insertAt := anchorPoint.End()
	ids := map[string]bool{}
	var edits []editset.Edit
	newOrder := 1
	for _, p := range points {
		if id, ok := p.AttrByLocal("", "id"); ok {
			ids[id] = true
		}
		if p.Open.Start < insertAt {
			newOrder++
		}
	}
	for i, p := range points {
		want := i + 1
		if p.Open.Start > insertAt {
			want++
		}
		idx := p.AttrIndex("", "playOrder")
		if idx < 0 {
			return nil, fmt.Errorf("%s: navPoint without playOrder; add the chapter by hand", ncxPath)
		}
		if p.Attrs[idx].Value == strconv.Itoa(want) {
			continue
		}
		span, _, ok := opf.RawAttrValueSpan(data, p, idx)
		if !ok {
			return nil, fmt.Errorf("%s: cannot locate a playOrder value; add the chapter by hand", ncxPath)
		}
		edits = append(edits, editset.Replace(ncxPath, int64(span.Start), int64(span.Len()), []byte(strconv.Itoa(want))))
	}
	id := pypath.UniqueID("np-"+strconv.Itoa(len(points)+1), ids)
	indent := lineIndent(data, anchorPoint.Open.Start)
	src := pypath.RelativeURI(ncxPath, newPath)
	point := "\n" + indent + `<navPoint id="` + pypath.EscapeAttribute(id) + `" playOrder="` + strconv.Itoa(newOrder) + `">` +
		"\n" + indent + "  <navLabel><text>" + xmlText(title) + "</text></navLabel>" +
		"\n" + indent + `  <content src="` + pypath.EscapeAttribute(src) + `"/>` +
		"\n" + indent + "</navPoint>"
	edits = append(edits, editset.Insert(ncxPath, int64(insertAt), []byte(point)))
	return editset.Apply(ncxPath, data, edits)
}

// ancestorDepth 数 n 与 stop 之间名为 local 的祖先个数（不含 n 和 stop 本身）。
func ancestorDepth(n, stop *opf.SpanNode, local string) int {
	depth := 0
	for p := n.Parent; p != nil && p != stop; p = p.Parent {
		if p.Name.Local == local {
			depth++
		}
	}
	return depth
}
