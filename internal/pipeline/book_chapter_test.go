package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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
