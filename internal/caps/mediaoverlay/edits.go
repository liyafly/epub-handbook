package mediaoverlay

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
	"github.com/liyafly/epub-handbook/internal/scan/xhtml"
)

type editFailure struct{ msg, loc string }

func (e *editFailure) Error() string { return e.loc + ": " + e.msg }

// highlightRule 追加到章节最后一个样式表末尾。半透明黄底在浅色和深色主题下都可见。
const highlightRule = "\n/* read-along highlight (media overlay) */\n." + ActiveClass +
	" { background-color: rgba(255, 200, 0, .35); border-radius: .15em; }\n"

// buildEdits 产出全部字节编辑，以及要从磁盘流式写入的音频 entry（容器路径, 磁盘路径）。
func buildEdits(ctx context.Context, b *book.Book, opfPath string, opfData []byte, tree *opf.SpanNode,
	plans []chapterPlan, narrator string) ([]editset.Edit, [][2]string, error) {
	var edits []editset.Edit
	var files [][2]string
	opfDir := path.Dir(opfPath)

	// 1. XHTML：句子 span 与补充的 id
	for _, cp := range plans {
		edits = append(edits, mergeInsertions(cp.item.ArchivePath, cp.plan.Insertions)...)
	}

	// 2. SMIL 新文档与音频
	var total float64
	durations := make([]float64, len(plans))
	for i, cp := range plans {
		smil, dur := buildSMIL(cp)
		durations[i] = dur
		total += dur
		edits = append(edits, editset.Insert(cp.smilName, 0, []byte(smil)))
		files = append(files, [2]string{cp.audioName, cp.audioDisk})
	}

	// 3. OPF：manifest 新 item、章节 item 的 media-overlay、metadata
	manifest := tree.ChildByLocal(opf.OPFURI, "manifest")
	metadata := tree.ChildByLocal(opf.OPFURI, "metadata")
	if manifest == nil || metadata == nil || manifest.SelfClose || metadata.SelfClose {
		return nil, nil, &editFailure{"OPF needs non-empty <metadata> and <manifest>", opfPath}
	}
	itemNodes := map[string]*opf.SpanNode{}
	for _, n := range opf.ManifestNodes(tree) {
		if id, _ := n.AttrByLocal("", "id"); id != "" {
			itemNodes[id] = n
		}
	}
	indent := lineIndent(opfData[manifest.Open.End:manifest.Close.Start])
	var items strings.Builder
	for _, cp := range plans {
		node := itemNodes[cp.item.ID]
		if node == nil {
			return nil, nil, &editFailure{"manifest item not found: " + cp.item.ID, opfPath}
		}
		at := node.Open.End - 1
		if opfData[at-1] == '/' {
			at--
			for at > node.Open.Start && opfData[at-1] == ' ' {
				at--
			}
		}
		edits = append(edits, editset.Insert(opfPath, int64(at), []byte(` media-overlay="`+pypath.EscapeAttribute(cp.smilID)+`"`)))
		fmt.Fprintf(&items, "%s<item id=\"%s\" href=\"%s\" media-type=\"application/smil+xml\"/>\n",
			indent, pypath.EscapeAttribute(cp.smilID), pypath.EscapeAttribute(relHref(opfDir, cp.smilName)))
		fmt.Fprintf(&items, "%s<item id=\"%s\" href=\"%s\" media-type=\"%s\"/>\n",
			indent, pypath.EscapeAttribute(cp.audioID), pypath.EscapeAttribute(relHref(opfDir, cp.audioName)), audioMediaType(cp.audioName))
	}
	edits = append(edits, insertBeforeClose(opfPath, opfData, manifest, items.String()))

	mIndent := lineIndent(opfData[metadata.Open.End:metadata.Close.Start])
	var metas strings.Builder
	fmt.Fprintf(&metas, "%s<meta property=\"media:active-class\">%s</meta>\n", mIndent, ActiveClass)
	if narrator != "" {
		fmt.Fprintf(&metas, "%s<meta property=\"media:narrator\">%s</meta>\n", mIndent, pypath.EscapeAttribute(narrator))
	}
	fmt.Fprintf(&metas, "%s<meta property=\"media:duration\">%s</meta>\n", mIndent, clock(total))
	for i, cp := range plans {
		fmt.Fprintf(&metas, "%s<meta property=\"media:duration\" refines=\"#%s\">%s</meta>\n", mIndent, pypath.EscapeAttribute(cp.smilID), clock(durations[i]))
	}
	edits = append(edits, insertBeforeClose(opfPath, opfData, metadata, metas.String()))

	// 4. CSS：第一章最后一个样式表末尾追加高亮规则
	if css := lastStylesheet(plans[0]); css != "" && b.Has(css) {
		data, err := b.CurrentContext(ctx, css)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", css, err)
		}
		if !strings.Contains(string(data), "."+ActiveClass) {
			rule := highlightRule
			if len(data) > 0 && data[len(data)-1] != '\n' {
				rule = "\n" + rule
			}
			edits = append(edits, editset.Insert(css, int64(len(data)), []byte(rule)))
		}
	} else {
		return nil, nil, &editFailure{"first chapter links no stylesheet in the book; cannot define ." + ActiveClass, plans[0].item.ArchivePath}
	}
	return edits, files, nil
}

// mergeInsertions 把同一偏移的插入按顺序拼成一个编辑，避免依赖排序稳定性。
func mergeInsertions(name string, ins []xhtml.Insertion) []editset.Edit {
	sorted := slices.Clone(ins)
	slices.SortStableFunc(sorted, func(a, b xhtml.Insertion) int { return a.Offset - b.Offset })
	var out []editset.Edit
	for i := 0; i < len(sorted); {
		j := i
		var text strings.Builder
		for j < len(sorted) && sorted[j].Offset == sorted[i].Offset {
			text.WriteString(sorted[j].Text)
			j++
		}
		out = append(out, editset.Insert(name, int64(sorted[i].Offset), []byte(text.String())))
		i = j
	}
	return out
}

func buildSMIL(cp chapterPlan) (string, float64) {
	smilDir := path.Dir(cp.smilName)
	textHref := relHref(smilDir, cp.item.ArchivePath)
	audioHref := relHref(smilDir, cp.audioName)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<smil xmlns="http://www.w3.org/ns/SMIL" xmlns:epub="http://www.idpf.org/2007/ops" version="3.0">` + "\n")
	b.WriteString("<body>\n")
	fmt.Fprintf(&b, "<seq id=\"seq1\" epub:textref=\"%s\" epub:type=\"chapter\">\n", pypath.EscapeAttribute(textHref))
	var dur float64
	for k, seg := range cp.plan.Segments {
		c := cp.align.Clips[k]
		fmt.Fprintf(&b, "<par id=\"par%d\"><text src=\"%s#%s\"/><audio src=\"%s\" clipBegin=\"%.3fs\" clipEnd=\"%.3fs\"/></par>\n",
			k+1, pypath.EscapeAttribute(textHref), pypath.EscapeAttribute(seg.ID), pypath.EscapeAttribute(audioHref), c.Begin, c.End)
		dur += round3(c.End) - round3(c.Begin)
	}
	b.WriteString("</seq>\n</body>\n</smil>\n")
	return b.String(), dur
}

func clock(sec float64) string {
	ms := int64(sec*1000 + 0.5)
	h := ms / 3_600_000
	m := ms / 60_000 % 60
	s := ms / 1000 % 60
	return fmt.Sprintf("%d:%02d:%02d.%03d", h, m, s, ms%1000)
}

func audioMediaType(name string) string {
	if strings.EqualFold(path.Ext(name), ".m4a") {
		return "audio/mp4"
	}
	return "audio/mpeg"
}

// lastStylesheet 返回章节里最后一个 <link rel="stylesheet"> 的容器路径。
func lastStylesheet(cp chapterPlan) string {
	found := ""
	for _, tag := range xhtml.TagsIn(cp.doc, "link") {
		rel, _ := tag.Attr("rel")
		href, ok := tag.Attr("href")
		if !ok || !slices.Contains(strings.Fields(strings.ToLower(rel.Value)), "stylesheet") {
			continue
		}
		if p, ok := opf.ResolveHref(cp.item.ArchivePath, href.Value); ok {
			found = p
		}
	}
	return found
}

// insertBeforeClose 在闭合标签所在行之前插入多行内容；闭合标签前不是换行时先补一个。
func insertBeforeClose(name string, data []byte, n *opf.SpanNode, lines string) editset.Edit {
	at := n.Close.Start
	for at > n.Open.End && (data[at-1] == ' ' || data[at-1] == '\t') {
		at--
	}
	if at > 0 && data[at-1] != '\n' {
		lines = "\n" + lines
		at = n.Close.Start
	}
	return editset.Insert(name, int64(at), []byte(lines))
}

// lineIndent 取容器里第一个子元素所在行的缩进；没有缩进时返回空串。
func lineIndent(inner []byte) string {
	s := string(inner)
	i := strings.Index(s, "<")
	if i < 0 {
		return ""
	}
	lineStart := strings.LastIndexByte(s[:i], '\n') + 1
	indent := s[lineStart:i]
	if strings.Trim(indent, " \t") != "" {
		return ""
	}
	return indent
}
