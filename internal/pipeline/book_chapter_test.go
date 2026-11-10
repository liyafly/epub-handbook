package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

func (h *bookHarness) addChapter(opts BookAddChapterOptions) int {
	h.t.Helper()
	return BookAddChapter(context.Background(), opts, h.io())
}

func playOrders(t *testing.T, ncx string) []string {
	t.Helper()
	var out []string
	for _, m := range regexp.MustCompile(`playOrder="(\d+)"`).FindAllStringSubmatch(ncx, -1) {
		out = append(out, m[1])
	}
	return out
}

func TestBookAddChapterAppend(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	oebps := filepath.Join(dir, "epub", "OEBPS")
	for _, title := range []string{"第二章", "Chapter Three", "第四章"} {
		if code := h.addChapter(BookAddChapterOptions{Dir: dir, Title: title}); code != 0 {
			t.Fatalf("add %s = %d: %s", title, code, h.err)
		}
	}
	for _, rel := range []string{"Text/02-chapter.xhtml", "Text/03-chapter-three.xhtml", "Text/04-chapter.xhtml"} {
		page := mustRead(t, filepath.Join(oebps, filepath.FromSlash(rel)))
		if !strings.Contains(page, `href="../Styles/design.css"/>`+"\n"+`  <link rel="stylesheet" type="text/css" href="../Styles/reader.css"/>`+"\n</head>") {
			t.Errorf("%s: stylesheet links not copied with reader.css last:\n%s", rel, page)
		}
	}
	opf := mustRead(t, filepath.Join(oebps, "package.opf"))
	spine := opf[strings.Index(opf, "<spine"):]
	if !regexp.MustCompile(`(?s)idref="chapter-1"/>\s*<itemref idref="chapter-02-chapter"/>\s*<itemref idref="chapter-03-chapter-three"/>\s*<itemref idref="chapter-04-chapter"/>\s*</spine>`).MatchString(spine) {
		t.Errorf("spine order wrong:\n%s", spine)
	}
	if !strings.Contains(opf, `<item id="chapter-04-chapter" href="Text/04-chapter.xhtml" media-type="application/xhtml+xml"/>`) {
		t.Errorf("manifest item missing:\n%s", opf)
	}
	nav := mustRead(t, filepath.Join(oebps, "nav.xhtml"))
	if !strings.Contains(nav, `<li><a href="Text/01-chapter.xhtml">第一章</a></li>`+"\n"+`      <li><a href="Text/02-chapter.xhtml">第二章</a></li>`) {
		t.Errorf("nav entry not appended:\n%s", nav)
	}
	ncx := mustRead(t, filepath.Join(oebps, "toc.ncx"))
	if got := strings.Join(playOrders(t, ncx), ","); got != "1,2,3,4,5,6" {
		t.Errorf("playOrder = %s", got)
	}
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s %s", code, h.out, h.err)
	}
	if !strings.Contains(h.out.String(), "PASS epub.package.nav.audit") {
		t.Errorf("nav audit did not pass: %s", h.out)
	}
}

func TestBookAddChapterAfter(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	oebps := filepath.Join(dir, "epub", "OEBPS")
	if code := h.addChapter(BookAddChapterOptions{Dir: dir, Title: "尾章", File: "09-end.xhtml"}); code != 0 {
		t.Fatalf("add end = %d: %s", code, h.err)
	}
	if code := h.addChapter(BookAddChapterOptions{Dir: dir, Title: "插 & 章", After: "01-chapter.xhtml"}); code != 0 {
		t.Fatalf("add after = %d: %s", code, h.err)
	}
	opf := mustRead(t, filepath.Join(oebps, "package.opf"))
	if !regexp.MustCompile(`(?s)idref="chapter-1"/>\s*<itemref idref="chapter-10-chapter"/>\s*<itemref idref="chapter-09-end"/>`).MatchString(opf) {
		t.Errorf("--after spine order wrong:\n%s", opf)
	}
	nav := mustRead(t, filepath.Join(oebps, "nav.xhtml"))
	i1, i2, i3 := strings.Index(nav, "01-chapter.xhtml"), strings.Index(nav, "插 &amp; 章"), strings.Index(nav, "09-end.xhtml")
	if !(i1 < i2 && i2 < i3) {
		t.Errorf("--after nav order wrong:\n%s", nav)
	}
	ncx := mustRead(t, filepath.Join(oebps, "toc.ncx"))
	if got := strings.Join(playOrders(t, ncx), ","); got != "1,2,3,4,5" {
		t.Errorf("playOrder = %s", got)
	}
	n1, n2, n3 := strings.Index(ncx, "01-chapter.xhtml"), strings.Index(ncx, "10-chapter.xhtml"), strings.Index(ncx, "09-end.xhtml")
	if !(n1 < n2 && n2 < n3) {
		t.Errorf("--after NCX order wrong:\n%s", ncx)
	}
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s %s", code, h.out, h.err)
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		out[p] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestBookAddChapterFailsWithoutChanges(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	epubDir := filepath.Join(dir, "epub")
	before := snapshotTree(t, epubDir)
	cases := []struct {
		name string
		opts BookAddChapterOptions
		code int
	}{
		{"empty title", BookAddChapterOptions{Dir: dir, Title: "  "}, ExitUsage},
		{"existing file", BookAddChapterOptions{Dir: dir, Title: "x", File: "01-chapter.xhtml"}, ExitUsage},
		{"bad file", BookAddChapterOptions{Dir: dir, Title: "x", File: "../x.xhtml"}, ExitUsage},
		{"after not in spine", BookAddChapterOptions{Dir: dir, Title: "x", After: "nope.xhtml"}, ExitUsage},
	}
	for _, c := range cases {
		if code := h.addChapter(c.opts); code != c.code {
			t.Errorf("%s = %d, want %d: %s", c.name, code, c.code, h.err)
		}
	}
	// 书名页在 nav 里有条目，版权页也有；去掉版权页的 nav 条目后 --after 它必须拒绝。
	navPath := filepath.Join(epubDir, "OEBPS", "nav.xhtml")
	nav := mustRead(t, navPath)
	mustWrite(t, navPath, strings.Replace(nav, `      <li><a href="Text/00-copyright.xhtml">版权信息</a></li>`+"\n", "", 1))
	before[navPath] = mustRead(t, navPath)
	if code := h.addChapter(BookAddChapterOptions{Dir: dir, Title: "x", After: "00-copyright.xhtml"}); code != ExitUsage {
		t.Errorf("after without toc entry = %d: %s", code, h.err)
	} else if !strings.Contains(h.err.String(), "retry without --after") {
		t.Errorf("missing retry hint: %s", h.err)
	}
	// NCX 坏掉时，OPF 与 nav 也不能被改。
	ncxPath := filepath.Join(epubDir, "OEBPS", "toc.ncx")
	mustWrite(t, ncxPath, strings.Replace(mustRead(t, ncxPath), `playOrder="3"`, "", 1))
	before[ncxPath] = mustRead(t, ncxPath)
	if code := h.addChapter(BookAddChapterOptions{Dir: dir, Title: "x"}); code == 0 {
		t.Error("NCX without playOrder must fail")
	}
	after := snapshotTree(t, epubDir)
	if len(after) != len(before) {
		t.Errorf("failed add-chapter changed file count: %d -> %d", len(before), len(after))
	}
	for p, data := range before {
		if after[p] != data {
			t.Errorf("failed add-chapter modified %s", p)
		}
	}
}

func TestBookAddChapterLegacyLayoutAndNoNCX(t *testing.T) {
	h := newBookHarness(t)
	seed := h.newBook("")
	legacy := filepath.Join(t.TempDir(), "legacy")
	work := filepath.Join(legacy, "03 制作工作区")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(seed, "epub"), filepath.Join(work, "epub")); err != nil {
		t.Fatal(err)
	}
	oebps := filepath.Join(work, "epub", "OEBPS")
	opfPath := filepath.Join(oebps, "package.opf")
	opf := mustRead(t, opfPath)
	opf = strings.Replace(opf, `    <item id="ncx"         href="toc.ncx"               media-type="application/x-dtbncx+xml"/>`+"\n", "", 1)
	opf = strings.Replace(opf, `<spine toc="ncx">`, `<spine>`, 1)
	mustWrite(t, opfPath, opf)
	if err := os.Remove(filepath.Join(oebps, "toc.ncx")); err != nil {
		t.Fatal(err)
	}
	if code := h.addChapter(BookAddChapterOptions{Dir: legacy, Title: "第二章"}); code != 0 {
		t.Fatalf("legacy add = %d: %s", code, h.err)
	}
	if !strings.Contains(h.out.String(), "nav)") {
		t.Errorf("output should not mention NCX: %s", h.out)
	}
	if _, err := os.Stat(filepath.Join(oebps, "Text", "02-chapter.xhtml")); err != nil {
		t.Error(err)
	}
}

// tocChildren 返回 toc nav 里某个 ol 的直接 li 子项的 href（ol 为 nil 时取顶层 ol）。
func tocTopHrefs(t *testing.T, nav string) []string {
	t.Helper()
	root, err := opf.ScanXHTMLSpanTree([]byte(nav))
	if err != nil {
		t.Fatal(err)
	}
	toc := findTOCNav(root)
	if toc == nil {
		t.Fatal("no toc nav")
	}
	var out []string
	for _, li := range toc.ChildByAnyNS("ol").Kids {
		if a := li.ChildByAnyNS("a"); li.Name.Local == "li" && a != nil {
			href, _ := a.AttrByLocal("", "href")
			out = append(out, href)
		}
	}
	return out
}

func navMapTopSrcs(t *testing.T, ncx string) []string {
	t.Helper()
	root, err := opf.ScanSpanTree([]byte(ncx))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range root.ChildByAnyNS("navMap").Kids {
		if c := p.ChildByAnyNS("content"); p.Name.Local == "navPoint" && c != nil {
			src, _ := c.AttrByLocal("", "src")
			out = append(out, src)
		}
	}
	return out
}

func assertPlayOrderSequential(t *testing.T, ncx string) {
	t.Helper()
	got := playOrders(t, ncx)
	for i, v := range got {
		if v != strconv.Itoa(i+1) {
			t.Fatalf("playOrder = %v, want 1..%d", got, len(got))
		}
	}
}

func TestBookAddChapterAfterChapterWithSections(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	oebps := filepath.Join(dir, "epub", "OEBPS")
	chapter := filepath.Join(oebps, "Text", "01-chapter.xhtml")
	mustWrite(t, chapter, strings.Replace(mustRead(t, chapter), "</section>", `  <h2 id="s1">第一节</h2>
  </section>`, 1))
	navPath := filepath.Join(oebps, "nav.xhtml")
	mustWrite(t, navPath, strings.Replace(mustRead(t, navPath),
		`<li><a href="Text/01-chapter.xhtml">第一章</a></li>`,
		`<li><a href="Text/01-chapter.xhtml">第一章</a><ol><li><a href="Text/01-chapter.xhtml#s1">第一节</a></li></ol></li>`, 1))
	ncxPath := filepath.Join(oebps, "toc.ncx")
	mustWrite(t, ncxPath, strings.Replace(mustRead(t, ncxPath),
		`<content src="Text/01-chapter.xhtml"/>`,
		`<content src="Text/01-chapter.xhtml"/>
      <navPoint id="np-s1" playOrder="4"><navLabel><text>第一节</text></navLabel><content src="Text/01-chapter.xhtml#s1"/></navPoint>`, 1))

	if code := h.addChapter(BookAddChapterOptions{Dir: dir, Title: "第二章", After: "01-chapter.xhtml"}); code != 0 {
		t.Fatalf("add = %d: %s", code, h.err)
	}
	nav := mustRead(t, navPath)
	top := tocTopHrefs(t, nav)
	if n := len(top); n < 2 || top[n-2] != "Text/01-chapter.xhtml" || top[n-1] != "Text/02-chapter.xhtml" {
		t.Errorf("new li must be a top-level sibling right after chapter 1: %v\n%s", top, nav)
	}
	ncx := mustRead(t, ncxPath)
	srcs := navMapTopSrcs(t, ncx)
	if n := len(srcs); n < 2 || srcs[n-2] != "Text/01-chapter.xhtml" || srcs[n-1] != "Text/02-chapter.xhtml" {
		t.Errorf("new navPoint must be a direct navMap child after chapter 1: %v\n%s", srcs, ncx)
	}
	assertPlayOrderSequential(t, ncx)
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s %s", code, h.out, h.err)
	}
}

func TestBookAddChapterAfterNestedOnlyStaysAtThatLevel(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	oebps := filepath.Join(dir, "epub", "OEBPS")
	navPath := filepath.Join(oebps, "nav.xhtml")
	nav := mustRead(t, navPath)
	nav = strings.Replace(nav, `<li><a href="Text/00-title.xhtml">书名页</a></li>
      <li><a href="Text/00-copyright.xhtml">版权信息</a></li>`,
		`<li><a href="Text/00-title.xhtml">书名页</a><ol><li><a href="Text/00-copyright.xhtml">版权信息</a></li></ol></li>`, 1)
	mustWrite(t, navPath, nav)
	ncxPath := filepath.Join(oebps, "toc.ncx")
	ncx := mustRead(t, ncxPath)
	copyright := `
    <navPoint id="np-2" playOrder="2">
      <navLabel><text>版权信息</text></navLabel>
      <content src="Text/00-copyright.xhtml"/>
    </navPoint>`
	if !strings.Contains(ncx, copyright) {
		t.Fatal("starter NCX changed; update the fixture")
	}
	ncx = strings.Replace(ncx, copyright, "", 1)
	ncx = strings.Replace(ncx, `<content src="Text/00-title.xhtml"/>`, `<content src="Text/00-title.xhtml"/>`+strings.ReplaceAll(copyright, "\n    ", "\n      "), 1)
	mustWrite(t, ncxPath, ncx)

	if code := h.addChapter(BookAddChapterOptions{Dir: dir, Title: "献词", After: "00-copyright.xhtml"}); code != 0 {
		t.Fatalf("add = %d: %s", code, h.err)
	}
	nav = mustRead(t, navPath)
	if !strings.Contains(nav, `<ol><li><a href="Text/00-copyright.xhtml">版权信息</a></li>`+"\n"+`<li><a href="Text/02-chapter.xhtml">献词</a></li></ol></li>`) {
		t.Errorf("new li must be a sibling of the nested copyright entry:\n%s", nav)
	}
	if top := tocTopHrefs(t, nav); slices.Contains(top, "Text/02-chapter.xhtml") {
		t.Errorf("new li must not be top level: %v", top)
	}
	ncx = mustRead(t, ncxPath)
	if slices.Contains(navMapTopSrcs(t, ncx), "Text/02-chapter.xhtml") {
		t.Errorf("new navPoint must stay nested under the title page:\n%s", ncx)
	}
	assertPlayOrderSequential(t, ncx)
}
