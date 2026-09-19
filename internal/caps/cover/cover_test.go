package cover

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
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

// svgCoverFixture 复刻 test_replace_cover_resizes_svg_cover_page 的
// 改造版输入：加 cover-page.xhtml（内联 SVG 包裹封面图）+ PNG 头封面。
func svgCoverFixture() []zipEntry {
	entries := writeBookEntries("封面书", "cover", []byte("old-cover"))
	var out []zipEntry
	for _, e := range entries {
		if e.name == "OEBPS/content.opf" {
			e.content = bytes.Replace(e.content,
				[]byte(`<item id="cover-image" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>`),
				[]byte(`<item id="cover-image" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>\n    <item id="cover-page" href="Text/cover.xhtml" media-type="application/xhtml+xml" properties="svg"/>`),
				1)
		}
		out = append(out, e)
	}
	out = append(out[:len(out)-1], // 保持 mimetype 仍在末尾前插入
		zipEntry{name: "OEBPS/Text/cover.xhtml", content: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <body><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1654 2362"><image width="1654" height="2362" href="../Images/cover.jpg"/></svg></body>
</html>
`)},
		zipEntry{name: "mimetype", content: []byte("wrong-on-purpose")})
	return out
}

// pngDimsHeader 构造 1024x1536 的最小 PNG 头（与 Python 测试一致）。
func pngDimsHeader() []byte {
	return []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x06, 0x00,
		0x08, 0x02, 0x00, 0x00, 0x00,
	}
}

// ---- 比较工具 ----

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

// assertOperationFacts 锁定 cover.replace 的正式 facts 键（旧 OperationReport 的
// 全部信息：operation / opf / output / coverPath；改名进入 Result.Renames）。
func assertOperationFacts(t *testing.T, res report.Result, wantOutput string) {
	t.Helper()
	want := map[string]any{
		"operation": "replace-cover",
		"opf":       "OEBPS/content.opf",
		"output":    wantOutput,
	}
	for k, v := range want {
		if got := res.Facts[k]; got != v {
			t.Errorf("facts[%q] = %#v, want %#v", k, got, v)
		}
	}
	coverPath, _ := res.Facts["coverPath"].(string)
	if coverPath == "" || !strings.HasPrefix(coverPath, "OEBPS/") {
		t.Errorf("facts[coverPath] = %#v, want an OEBPS/ archive path", res.Facts["coverPath"])
	}
}

// TestReplaceCoverFacts 是不依赖 Python oracle 的正式 facts 断言。
func TestReplaceCoverFacts(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("封面书", "cover", []byte("old-cover")))
	cover := filepath.Join(dir, "new-cover.png")
	if err := os.WriteFile(cover, []byte("new-cover"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "cover.epub")
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{Cover: cover, Output: out})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	assertOperationFacts(t, res, out)
	if got := res.Facts["coverPath"]; got != "OEBPS/Images/cover.png" {
		t.Errorf("facts[coverPath] = %#v", got)
	}
}

// TestReplaceCoverRewritesOldReferences 原为 Python oracle 的 P2/P3 parity
// 用例（oracle 已于 2026-08-29 删除，`epub_cover_replace_harness.py` 不复
// 存在）。这里保留同一组 fixture，改为按封面替换的领域语义手写的
// Go-native 断言：
//
//   - facts.coverPath 是新封面的归档路径；
//   - 正文里对旧封面的引用（chapter.xhtml 的 <img src>、main.css 的
//     url()）逐处改写到新路径，旧路径的字节不再出现；
//   - 与封面无关的 entry（nav.xhtml、.DS_Store）逐字节不变——证明区域感知
//     重写没有波及未命中的文件；
//   - 旧封面文件从包内删除，新封面文件字节与输入一致；
//   - Result.Renames 与 facts.mappings 一一对应（{old.jpg: new.png}）。
func TestReplaceCoverRewritesOldReferences(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	fixture := writeBookEntries("封面书", "cover", []byte("old-cover"))
	buildEpub(t, source, fixture)
	cover := filepath.Join(dir, "new-cover.png")
	if err := os.WriteFile(cover, []byte("new-cover"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "cover.epub")
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{Cover: cover, Output: out})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	if err := b.WriteTo(out); err != nil {
		t.Fatal(err)
	}

	assertOperationFacts(t, res, out)
	if got := res.Facts["coverPath"]; got != "OEBPS/Images/cover.png" {
		t.Fatalf("facts[coverPath] = %#v, want OEBPS/Images/cover.png", got)
	}

	entries := readZipEntries(t, out)

	chapter := string(entries["OEBPS/Text/chapter.xhtml"])
	if !strings.Contains(chapter, `src="../Images/cover.png"`) {
		t.Errorf("chapter.xhtml 的 img src 应重写到新封面: %s", chapter)
	}
	if strings.Contains(chapter, "cover.jpg") {
		t.Errorf("chapter.xhtml 不应再引用旧封面路径: %s", chapter)
	}
	css := string(entries["OEBPS/Styles/main.css"])
	if !strings.Contains(css, `url('../Images/cover.png')`) {
		t.Errorf("main.css 的 url() 应重写到新封面: %s", css)
	}
	if strings.Contains(css, "cover.jpg") {
		t.Errorf("main.css 不应再引用旧封面路径: %s", css)
	}

	// 与封面无关的 entry 必须逐字节不变：证明重写没有波及未命中的文件
	// （.DS_Store 不在此列——book.Open 从一开始就排除 macOS 元数据文件，
	// 与本次改动无关）。
	for _, e := range fixture {
		switch e.name {
		case "OEBPS/nav.xhtml":
			got, ok := entries[e.name]
			if !ok {
				t.Fatalf("entry %s 应保留在输出里", e.name)
			}
			if !bytes.Equal(got, e.content) {
				t.Errorf("entry %s 应逐字节不变:\n got  = %s\n want = %s", e.name, got, e.content)
			}
		}
	}

	if _, ok := entries["OEBPS/Images/cover.jpg"]; ok {
		t.Error("旧封面文件应被删除")
	}
	if got := string(entries["OEBPS/Images/cover.png"]); got != "new-cover" {
		t.Errorf("新封面文件内容 = %q, want %q", got, "new-cover")
	}

	if len(res.Renames) != 1 || res.Renames["OEBPS/Images/cover.jpg"] != "OEBPS/Images/cover.png" {
		t.Fatalf("Renames = %v, want {OEBPS/Images/cover.jpg: OEBPS/Images/cover.png}", res.Renames)
	}
	mappings, ok := res.Facts["mappings"].([]map[string]string)
	if !ok || len(mappings) != 1 || mappings[0]["from"] != "OEBPS/Images/cover.jpg" || mappings[0]["to"] != "OEBPS/Images/cover.png" {
		t.Errorf("facts[mappings] = %#v", res.Facts["mappings"])
	}
}

// TestReplaceCoverResizesSVGCoverPageAndRewritesReferences 原为 Python
// oracle 的 SVG 封面页 parity 用例（同上，oracle 已删除）。断言 SVG 封面页
// 的 viewBox/宽高改写数学：新封面（pngDimsHeader，1024x1536 像素）替换后，
// 内联 SVG 的 viewBox 与 <image> 的 width/height 必须精确对齐到新封面的
// 像素尺寸，而不是任意数值或旧尺寸的残留。
func TestReplaceCoverResizesSVGCoverPageAndRewritesReferences(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, svgCoverFixture())
	cover := filepath.Join(dir, "new-cover.png")
	dims := pngDimsHeader()
	if err := os.WriteFile(cover, dims, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "cover.epub")
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{Cover: cover, Output: out})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	if err := b.WriteTo(out); err != nil {
		t.Fatal(err)
	}

	assertOperationFacts(t, res, out)
	if got := res.Facts["coverPath"]; got != "OEBPS/Images/cover.png" {
		t.Fatalf("facts[coverPath] = %#v, want OEBPS/Images/cover.png", got)
	}

	entries := readZipEntries(t, out)

	// pngDimsHeader 编码的像素尺寸是 1024x1536（IHDR 的 width=0x0400,
	// height=0x0600）；SVG 封面页原尺寸是 1654x2362，替换后必须整体对齐
	// 到新封面的像素尺寸，旧尺寸不得残留。
	svgPage := string(entries["OEBPS/Text/cover.xhtml"])
	for _, want := range []string{
		`viewBox="0 0 1024 1536"`,
		`<image width="1024" height="1536" href="../Images/cover.png"/>`,
	} {
		if !strings.Contains(svgPage, want) {
			t.Errorf("cover.xhtml 缺少 %q:\n%s", want, svgPage)
		}
	}
	for _, stale := range []string{"1654", "2362", "cover.jpg"} {
		if strings.Contains(svgPage, stale) {
			t.Errorf("cover.xhtml 残留旧尺寸/旧路径 %q:\n%s", stale, svgPage)
		}
	}

	// 正文引用同样重写（与非 SVG 场景相同的 transformResource 路径）。
	chapter := string(entries["OEBPS/Text/chapter.xhtml"])
	if !strings.Contains(chapter, `src="../Images/cover.png"`) {
		t.Errorf("chapter.xhtml 的 img src 应重写到新封面: %s", chapter)
	}
	css := string(entries["OEBPS/Styles/main.css"])
	if !strings.Contains(css, `url('../Images/cover.png')`) {
		t.Errorf("main.css 的 url() 应重写到新封面: %s", css)
	}

	if _, ok := entries["OEBPS/Images/cover.jpg"]; ok {
		t.Error("旧封面文件应被删除")
	}
	if got := entries["OEBPS/Images/cover.png"]; !bytes.Equal(got, dims) {
		t.Errorf("新封面文件内容与输入不一致")
	}

	if len(res.Renames) != 1 || res.Renames["OEBPS/Images/cover.jpg"] != "OEBPS/Images/cover.png" {
		t.Fatalf("Renames = %v, want {OEBPS/Images/cover.jpg: OEBPS/Images/cover.png}", res.Renames)
	}
}

func TestCoverMissingFileRefused(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("书", "m", []byte("c")))
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{
		Cover:  filepath.Join(dir, "nope.png"),
		Output: filepath.Join(dir, "out.epub"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed ||
		res.Findings[0].Title != "cover image not found: "+filepath.Join(dir, "nope.png") {
		t.Errorf("缺封面文件应拒绝: %+v", res.Findings)
	}
}
