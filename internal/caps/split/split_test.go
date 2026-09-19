package split

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// ---- fixture：逐字节复刻 scripts/test_epub_package_tool.py 的 write_book ----

type zipEntry struct {
	name    string
	content []byte
}

func buildEpub(t *testing.T, path string, entries []zipEntry) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name}
		h.Method = zip.Deflate
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(e.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeBookEntries(title, marker string, coverBytes []byte) []zipEntry {
	f := func(s string) []byte { return []byte(s) }
	return []zipEntry{
		{name: "META-INF/container.xml", content: f(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`)},
		{name: "OEBPS/content.opf", content: f(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:uuid:` + marker + `</dc:identifier>
    <dc:title id="main-title">` + title + `</dc:title>
    <dc:creator>Author ` + marker + `</dc:creator>
    <dc:language>zh-CN</dc:language>
    <dc:publisher>Publisher ` + marker + `</dc:publisher>
    <dc:description>Description ` + marker + `</dc:description>
    <meta name="cover" content="cover-image"/>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chap" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="style" href="Styles/main.css" media-type="text/css"/>
    <item id="cover-image" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
  </manifest>
  <spine>
    <itemref idref="nav" linear="no"/>
    <itemref idref="chap"/>
  </spine>
</package>
`)},
		{name: "OEBPS/nav.xhtml", content: f(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <head><title>` + title + `</title></head>
  <body><nav epub:type="toc"><ol><li><a href="Text/chapter.xhtml#start">` + title + `</a></li></ol></nav></body>
</html>
`)},
		{name: "OEBPS/Text/chapter.xhtml", content: f(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>` + title + `</title><link rel="stylesheet" href="../Styles/main.css"/></head>
  <body>
    <h1 id="start">` + title + `</h1>
    <p>` + marker + ` 正文保留。<img src="../Images/cover.jpg" alt="cover"/></p>
  </body>
</html>
`)},
		{name: "OEBPS/Styles/main.css", content: f("body { background: url('../Images/cover.jpg'); }\n")},
		{name: "OEBPS/Images/cover.jpg", content: coverBytes},
		{name: ".DS_Store", content: f("macos-metadata")},
		{name: "mimetype", content: f("wrong-on-purpose")},
	}
}

// readZipEntries 解压出全部 entry（mimetype 除外）；srcset_test.go 也在用。
func readZipEntries(t *testing.T, path string) map[string][]byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	r, err := zip.NewReader(f, st.Size())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, zf := range r.File {
		if zf.Name == "mimetype" {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			t.Fatal(err)
		}
		rc.Close()
		out[zf.Name] = buf.Bytes()
	}
	return out
}

// assertOperationFacts 锁定 split 的正式 facts 键（旧 OperationReport 的全部
// 信息：operation / opf / outputs / segmentsCreated，另有 outputDir）。
func assertOperationFacts(t *testing.T, res report.Result, outputDir string, wantOutputs []string, wantSegments int) {
	t.Helper()
	want := map[string]any{
		"operation":       "split",
		"opf":             "OEBPS/content.opf",
		"outputDir":       outputDir,
		"segmentsCreated": wantSegments,
	}
	for k, v := range want {
		if got := res.Facts[k]; got != v {
			t.Errorf("facts[%q] = %#v, want %#v", k, got, v)
		}
	}
	outputs, _ := res.Facts["outputs"].([]string)
	if len(outputs) != len(wantOutputs) {
		t.Fatalf("facts[outputs] = %#v, want %#v", res.Facts["outputs"], wantOutputs)
	}
	for i := range wantOutputs {
		if outputs[i] != wantOutputs[i] {
			t.Errorf("facts[outputs][%d] = %q, want %q", i, outputs[i], wantOutputs[i])
		}
		if _, err := os.Stat(outputs[i]); err != nil {
			t.Errorf("segment %s missing: %v", outputs[i], err)
		}
	}
}

// TestSplitFacts 是不依赖 Python oracle 的正式 facts 断言。
func TestSplitFacts(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("拆分书", "split", []byte("cover")))
	outDir := filepath.Join(dir, "split")
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	assertOperationFacts(t, res, outDir, []string{filepath.Join(outDir, "source_01.epub")}, 1)
}

func TestSplitRefusesEncryptionAndBadPoints(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.epub")
	entries := append(writeBookEntries("书", "m", []byte("c")), zipEntry{
		name:    "META-INF/encryption.xml",
		content: []byte(`<?xml version="1.0"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"/>`),
	})
	encPath := filepath.Join(dir, "enc.epub")
	buildEpub(t, encPath, entries)
	buildEpub(t, src, writeBookEntries("书", "m", []byte("c")))
	outDir := filepath.Join(dir, "out")

	b, err := book.Open(encPath)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed || res.Findings[0].Title !=
		"split: encrypted EPUB resources detected; refusing package rewrite" {
		t.Errorf("加密输入应拒绝: %+v", res.Findings)
	}

	b2, err := book.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()
	res2, err := Run(t.Context(), b2, Params{SplitPoints: []int{5}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Status != report.StatusFailed || res2.Findings[0].Title != "split point out of range: 5" {
		t.Errorf("越界切分点应拒绝: %+v", res2.Findings)
	}
	res3, err := Run(t.Context(), b2, Params{OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res3.Status != report.StatusFailed || res3.Findings[0].Title != "split: at least one split point is required" {
		t.Errorf("缺切分点应拒绝: %+v", res3.Findings)
	}
}

func TestSplitProjectionValidationManual(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("拆分书", "split", []byte("cover")))
	outDir := filepath.Join(dir, "out")
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s findings=%+v", res.Status, res.Findings)
	}
	if _, err := os.Stat(filepath.Join(outDir, "source_01.epub")); err != nil {
		t.Fatalf("segment not committed: %v", err)
	}
}

func writeTwoChapterEntries(badSecond bool) []zipEntry {
	f := func(s string) []byte { return []byte(s) }
	second := `<html xmlns="http://www.w3.org/1999/xhtml"><body><h1 id="two">第二章</h1><p>第二段正文。</p></body></html>`
	if badSecond {
		second = `<html xmlns="http://www.w3.org/1999/xhtml"><body><h1 id="two">第二章`
	}
	return []zipEntry{
		{name: "META-INF/container.xml", content: f(`<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{name: "OEBPS/content.opf", content: f(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="book-id">urn:uuid:two</dc:identifier><dc:title>两章书</dc:title><dc:creator>作者</dc:creator><dc:language>zh-CN</dc:language><meta name="cover" content="cover-image"/></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="one" href="Text/one.xhtml" media-type="application/xhtml+xml"/><item id="two" href="Text/two.xhtml" media-type="application/xhtml+xml"/><item id="cover-image" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/></manifest><spine><itemref idref="nav" linear="no"/><itemref idref="one"/><itemref idref="two"/></spine></package>`)},
		{name: "OEBPS/nav.xhtml", content: f(`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="Text/one.xhtml#one">第一章</a></li><li><a href="Text/two.xhtml#two">第二章</a></li></ol></nav></body></html>`)},
		{name: "OEBPS/Text/one.xhtml", content: f(`<html xmlns="http://www.w3.org/1999/xhtml"><body><h1 id="one">第一章</h1><p>第一段正文。</p></body></html>`)},
		{name: "OEBPS/Text/two.xhtml", content: f(second)},
		{name: "OEBPS/Images/cover.jpg", content: []byte("cover bytes")},
		{name: "mimetype", content: f("wrong")},
	}
}

// writeThreeChapterEntries 提供三章书 fixture，用来验证 split_points 语义
// 在段边界不是简单"一点一段"时（一段吸收两章）依然按 targets 下标切分正确。
func writeThreeChapterEntries() []zipEntry {
	f := func(s string) []byte { return []byte(s) }
	return []zipEntry{
		{name: "META-INF/container.xml", content: f(`<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{name: "OEBPS/content.opf", content: f(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="book-id">urn:uuid:three</dc:identifier><dc:title>三章书</dc:title><dc:creator>作者</dc:creator><dc:language>zh-CN</dc:language><meta name="cover" content="cover-image"/></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="one" href="Text/one.xhtml" media-type="application/xhtml+xml"/><item id="two" href="Text/two.xhtml" media-type="application/xhtml+xml"/><item id="three" href="Text/three.xhtml" media-type="application/xhtml+xml"/><item id="cover-image" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/></manifest><spine><itemref idref="nav" linear="no"/><itemref idref="one"/><itemref idref="two"/><itemref idref="three"/></spine></package>`)},
		{name: "OEBPS/nav.xhtml", content: f(`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="Text/one.xhtml#one">第一章</a></li><li><a href="Text/two.xhtml#two">第二章</a></li><li><a href="Text/three.xhtml#three">第三章</a></li></ol></nav></body></html>`)},
		{name: "OEBPS/Text/one.xhtml", content: f(`<html xmlns="http://www.w3.org/1999/xhtml"><body><h1 id="one">第一章</h1><p>第一段正文。</p></body></html>`)},
		{name: "OEBPS/Text/two.xhtml", content: f(`<html xmlns="http://www.w3.org/1999/xhtml"><body><h1 id="two">第二章</h1><p>第二段正文。</p></body></html>`)},
		{name: "OEBPS/Text/three.xhtml", content: f(`<html xmlns="http://www.w3.org/1999/xhtml"><body><h1 id="three">第三章</h1><p>第三段正文。</p></body></html>`)},
		{name: "OEBPS/Images/cover.jpg", content: []byte("cover bytes")},
		{name: "mimetype", content: f("wrong")},
	}
}

var (
	hrefAttrRe = regexp.MustCompile(`href="([^"]*)"`)
	srcAttrRe  = regexp.MustCompile(`src="([^"]*)"`)
)

// assertLinksResolveInSegment 断言 docPath（nav.xhtml 或 toc.ncx）里全部
// attr="..." 目标——包括锚点——都能在 seg 内部解析。这是"分段是独立可用的
// EPUB"这条要求的直接体现：如果 nav / NCX 引用了没被选中、留在别的段里的
// 文件，读者在这一段书里点目录会打不开。
func assertLinksResolveInSegment(t *testing.T, seg *book.Book, docPath, attr string) {
	t.Helper()
	data, err := seg.Current(docPath)
	if err != nil {
		t.Fatalf("%s missing from segment: %v", docPath, err)
	}
	re := hrefAttrRe
	if attr == "src" {
		re = srcAttrRe
	}
	matches := re.FindAllStringSubmatch(string(data), -1)
	if len(matches) == 0 {
		t.Errorf("%s 没有任何 %s 目标，导航生成可能失败了", docPath, attr)
	}
	for _, m := range matches {
		raw := m[1]
		target, frag, _ := strings.Cut(raw, "#")
		resolved := path.Join(path.Dir(docPath), target)
		if !seg.Has(resolved) {
			t.Errorf("%s: %s=%q 指向段外文件，跨段死链: %s", docPath, attr, raw, resolved)
			continue
		}
		if frag == "" {
			continue
		}
		targetData, err := seg.Current(resolved)
		if err != nil {
			t.Fatalf("%s: 读取 %s 失败: %v", docPath, resolved, err)
		}
		if !bytes.Contains(targetData, []byte(`id="`+frag+`"`)) {
			t.Errorf("%s: %s=%q 的锚点 #%s 在 %s 里不存在", docPath, attr, raw, frag, resolved)
		}
	}
}

// TestSplitSegmentsAreIndependentEPUBs 是切分能力存在的核心承诺：每一段都
// 是能独立打开的完整 EPUB（container→OPF→manifest/spine 内部自洽），被选中
// 的正文逐字节保留，另一段的章节不会泄漏进来，且 nav / NCX 的全部目标都能
// 在段内部解析——不留跨段死链（否则读者打开这一段书点目录会打不开）。
func TestSplitSegmentsAreIndependentEPUBs(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeTwoChapterEntries(false))
	outDir := filepath.Join(dir, "out")
	srcBook, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer srcBook.Close()
	res, err := Run(t.Context(), srcBook, Params{SplitPoints: []int{0, 1}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("split status = %s findings=%+v", res.Status, res.Findings)
	}

	own := map[string]string{
		filepath.Join(outDir, "source_01.epub"): "OEBPS/Text/one.xhtml",
		filepath.Join(outDir, "source_02.epub"): "OEBPS/Text/two.xhtml",
	}
	other := map[string]string{
		filepath.Join(outDir, "source_01.epub"): "OEBPS/Text/two.xhtml",
		filepath.Join(outDir, "source_02.epub"): "OEBPS/Text/one.xhtml",
	}
	for outPath, chapterPath := range own {
		func() {
			seg, err := book.Open(outPath)
			if err != nil {
				t.Fatalf("segment %s does not open as EPUB: %v", outPath, err)
			}
			defer seg.Close()

			containerData, err := seg.Current("META-INF/container.xml")
			if err != nil {
				t.Fatalf("%s missing container.xml: %v", outPath, err)
			}
			opfPath, err := opf.FindOPFPath(containerData)
			if err != nil {
				t.Fatalf("%s container.xml does not resolve OPF: %v", outPath, err)
			}
			opfData, err := seg.Current(opfPath)
			if err != nil {
				t.Fatalf("%s missing resolved OPF %s: %v", outPath, opfPath, err)
			}
			pkg, err := opf.Parse(opfPath, opfData)
			if err != nil {
				t.Fatalf("%s OPF does not parse: %v", outPath, err)
			}

			// manifest 自洽：每个非外链 item 的目标都必须真的在段内。
			for _, item := range pkg.Manifest {
				if item.ArchivePath == "" {
					continue
				}
				if !seg.Has(item.ArchivePath) {
					t.Errorf("%s: manifest item %s -> %s missing from segment", outPath, item.ID, item.ArchivePath)
				}
			}
			// spine 自洽：每个 itemref 都能解析回一个 manifest item。
			for _, sp := range pkg.Spine {
				if _, ok := pkg.ItemByID(sp.IDRef); !ok {
					t.Errorf("%s: spine idref %q has no manifest item", outPath, sp.IDRef)
				}
			}

			// 被选中的正文逐字节保留：这是 INV-1 在 split 上的体现，一次
			// 切分绝不允许悄悄改写被选中的章节内容。
			srcXHTML, err := srcBook.Original(chapterPath)
			if err != nil {
				t.Fatal(err)
			}
			segXHTML, err := seg.Current(chapterPath)
			if err != nil {
				t.Fatalf("%s missing selected chapter %s: %v", outPath, chapterPath, err)
			}
			if !bytes.Equal(srcXHTML, segXHTML) {
				t.Errorf("%s: selected chapter %s bytes changed", outPath, chapterPath)
			}

			// 段间隔离：另一段的章节不该出现在这一段里，否则不算真正独立。
			if seg.Has(other[outPath]) {
				t.Errorf("%s: leaked chapter from the other segment: %s", outPath, other[outPath])
			}

			navItem, ok := pkg.NavItem()
			if !ok {
				t.Fatalf("%s: manifest has no nav item", outPath)
			}
			assertLinksResolveInSegment(t, seg, navItem.ArchivePath, "href")
			if ncxItem, ok := pkg.NCXItem(); ok {
				assertLinksResolveInSegment(t, seg, ncxItem.ArchivePath, "src")
			}
		}()
	}
}

// TestSplitPointsPartitionSpineContent 锁定 split_points 的边界语义：切分点
// 是 targets 下标而不是"每点一段"——[0,2] 在三章书上应该产出"第一、二章
// 一段 + 第三章一段"，而不是三段各一章。同时验证 segmentPlans / outputs /
// segmentsCreated 这些 facts 与实际落盘产物完全一致。
func TestSplitPointsPartitionSpineContent(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeThreeChapterEntries())
	outDir := filepath.Join(dir, "out")
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0, 2}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("split status = %s findings=%+v", res.Status, res.Findings)
	}

	if got := res.Facts["segmentsCreated"]; got != 2 {
		t.Fatalf("segmentsCreated = %v, want 2", got)
	}
	outputs, _ := res.Facts["outputs"].([]string)
	if len(outputs) != 2 {
		t.Fatalf("facts[outputs] = %#v, want 2 entries", res.Facts["outputs"])
	}
	for _, out := range outputs {
		if _, statErr := os.Stat(out); statErr != nil {
			t.Errorf("facts 声明的产物不存在: %s (%v)", out, statErr)
		}
	}

	plans, ok := res.Facts["segmentPlans"].([]map[string]any)
	if !ok || len(plans) != 2 {
		t.Fatalf("facts[segmentPlans] = %#v", res.Facts["segmentPlans"])
	}
	want := [][]string{
		{"OEBPS/Text/one.xhtml", "OEBPS/Text/two.xhtml"},
		{"OEBPS/Text/three.xhtml"},
	}
	for i, w := range want {
		got, _ := plans[i]["selectedSpine"].([]string)
		if !reflect.DeepEqual(got, w) {
			t.Errorf("segment %d selectedSpine = %#v, want %#v", i+1, got, w)
		}
		if plans[i]["output"] != outputs[i] {
			t.Errorf("segment %d facts.segmentPlans.output = %v, want %v", i+1, plans[i]["output"], outputs[i])
		}
	}

	// 落盘产物必须真的按上面的边界切分：第一段吸收了两章，OPF spine 应
	// 恰好是 nav + one + two（顺序不变）；第二段只有 nav + three。
	seg1, err := book.Open(outputs[0])
	if err != nil {
		t.Fatal(err)
	}
	defer seg1.Close()
	opfData1, err := seg1.Current("OEBPS/content.opf")
	if err != nil {
		t.Fatal(err)
	}
	pkg1, err := opf.Parse("OEBPS/content.opf", opfData1)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg1.Spine) != 3 {
		t.Fatalf("segment 1 spine length = %d, want 3 (nav + two chapters)", len(pkg1.Spine))
	}
	var seg1IDRefs []string
	for _, sp := range pkg1.Spine[1:] {
		seg1IDRefs = append(seg1IDRefs, sp.IDRef)
	}
	if !reflect.DeepEqual(seg1IDRefs, []string{"one", "two"}) {
		t.Errorf("segment 1 content spine idrefs = %v, want [one two]", seg1IDRefs)
	}

	seg2, err := book.Open(outputs[1])
	if err != nil {
		t.Fatal(err)
	}
	defer seg2.Close()
	opfData2, err := seg2.Current("OEBPS/content.opf")
	if err != nil {
		t.Fatal(err)
	}
	pkg2, err := opf.Parse("OEBPS/content.opf", opfData2)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg2.Spine) != 2 {
		t.Fatalf("segment 2 spine length = %d, want 2 (nav + one chapter)", len(pkg2.Spine))
	}
	if pkg2.Spine[1].IDRef != "three" {
		t.Errorf("segment 2 content spine idref = %q, want %q", pkg2.Spine[1].IDRef, "three")
	}
}

// TestValidateSegmentRejectsCoverDrift 直接调用 validateSegment（分区红线的
// 唯一入口），证明 validation.go 的 metadata/cover/drm 红线真的会开火，而
// 不是形同虚设：把段投影里的封面字节篡改掉、OPF 不动，模拟"资源闭包或提交
// 阶段悄悄换了封面却没人发现"的回归。断言必须拿到带 "cover:" 的明确失败。
func TestValidateSegmentRejectsCoverDrift(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeTwoChapterEntries(false))

	original, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	names := original.OriginalNames()
	namesSet := make(map[string]bool, len(names))
	for _, n := range names {
		namesSet[n] = true
	}
	pkg, err := readPackage(namesSet, original.Original)
	if err != nil {
		t.Fatal(err)
	}

	segment, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer segment.Close()
	tampered := []byte("tampered-cover-bytes")
	if err := segment.Apply([]editset.Edit{
		editset.Replace("OEBPS/Images/cover.jpg", 0, int64(len("cover bytes")), tampered),
	}); err != nil {
		t.Fatal(err)
	}

	_, err = validateSegment(t.Context(), original, segment, pkg,
		[]string{"OEBPS/Text/one.xhtml"}, []string{"one"}, "nav", "OEBPS/nav.xhtml", "OEBPS/toc.ncx")
	if err == nil {
		t.Fatal("封面被篡改的分段投影应被红线拒绝，实际静默通过了")
	}
	if !strings.Contains(err.Error(), "cover:") {
		t.Errorf("拒绝原因应指向 cover 红线，实际: %v", err)
	}
}

func TestSplitOutputDirectoryPreflightAndDryRun(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeTwoChapterEntries(false))
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: ""})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed || len(res.Findings) == 0 {
		t.Fatalf("empty output dir should fail: %+v", res)
	}
	if _, statErr := os.Stat("source_01.epub"); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("empty output dir may not write cwd: %v", statErr)
	}

	existingDir := filepath.Join(dir, "existing-dir")
	if err := os.Mkdir(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(existingDir, "marker")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: existingDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed {
		t.Fatalf("existing output dir should fail: %+v", res)
	}
	if got, readErr := os.ReadFile(marker); readErr != nil || string(got) != "keep" {
		t.Fatalf("existing output dir changed: %q (%v)", got, readErr)
	}

	existingFile := filepath.Join(dir, "existing-file")
	if err := os.WriteFile(existingFile, []byte("keep file"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: existingFile})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed {
		t.Fatalf("existing output file should fail: %+v", res)
	}
	if got, readErr := os.ReadFile(existingFile); readErr != nil || string(got) != "keep file" {
		t.Fatalf("existing output file changed: %q (%v)", got, readErr)
	}

	dryDir := filepath.Join(dir, "dry-run")
	res, err = Run(t.Context(), b, Params{SplitPoints: []int{0, 1}, OutputDir: dryDir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("dry-run capability result should remain complete for pipeline: %s", res.Status)
	}
	if _, statErr := os.Stat(dryDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("dry-run created output directory: %v", statErr)
	}
	planned, ok := res.Facts["plannedOutputs"].([]string)
	if !ok || len(planned) != 2 {
		t.Fatalf("dry-run planned outputs = %#v", res.Facts["plannedOutputs"])
	}
}

// TestSplitValidationFailureLeavesNoArtifacts 用一段格式损坏的 XHTML（第二
// 章标签未闭合）证明 validateSegment 内的结构校验（validateRetainedReferences
// 的 XML 解析）真的会开火：拿到的必须是明确的 failed 结果而不是静默产出坏
// 分段；失败时不许留下任何输出目录或部分产物。
func TestSplitValidationFailureLeavesNoArtifacts(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeTwoChapterEntries(true))
	outDir := filepath.Join(dir, "segments")
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0, 1}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed || len(res.Findings) == 0 {
		t.Fatalf("malformed second segment should fail with finding: %+v", res)
	}
	if _, statErr := os.Stat(outDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed split left output directory: %v", statErr)
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if entry.Name() != filepath.Base(source) {
			t.Errorf("failed split left sibling artifact: %s", entry.Name())
		}
	}
}

func TestSplitCommitsAllSegmentsTogether(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeTwoChapterEntries(false))
	outDir := filepath.Join(dir, "segments")
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(t.Context(), b, Params{SplitPoints: []int{0, 1}, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("split status = %s findings=%+v", res.Status, res.Findings)
	}
	for _, name := range []string{"source_01.epub", "source_02.epub"} {
		if _, statErr := os.Stat(filepath.Join(outDir, name)); statErr != nil {
			t.Fatalf("missing committed segment %s: %v", name, statErr)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("unexpected transaction siblings: %+v", entries)
	}
}
