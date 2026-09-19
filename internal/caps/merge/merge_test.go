package merge

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
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

// readZipEntries 解压出 name → content 映射（忽略目录项）。
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
		if zf.Name == "mimetype" || endsWithAny(zf.Name, "/") {
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

func endsWithAny(s, suf string) bool { return stringsHasSuffix(s, suf) }

// modifiedTSRe 匹配 OPF 里的 dcterms:modified meta 元素：这是合并产物中
// 唯一的不确定字段（当前 UTC 时间），Go-native 断言只验证它存在且格式
// 合法，不比较具体值。
var modifiedTSRe = regexp.MustCompile(`<meta property="dcterms:modified">[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z</meta>`)

func strPtr(s string) *string { return &s }

// assertOperationFacts 锁定 merge 的正式 facts 键（旧 OperationReport 的全部
// 信息：operation / opf / inputs / output / mergedItems / renamedResources /
// warnings；资源改名进入 Result.Renames）。
func assertOperationFacts(t *testing.T, res report.Result, inputs []string, output string, wantMerged, wantRenamed int) {
	t.Helper()
	want := map[string]any{
		"operation":        "merge",
		"opf":              "OEBPS/content.opf",
		"output":           output,
		"mergedItems":      wantMerged,
		"renamedResources": wantRenamed,
	}
	for k, v := range want {
		if got := res.Facts[k]; got != v {
			t.Errorf("facts[%q] = %#v, want %#v", k, got, v)
		}
	}
	gotInputs, _ := res.Facts["inputs"].([]string)
	if len(gotInputs) != len(inputs) {
		t.Fatalf("facts[inputs] = %#v, want %#v", res.Facts["inputs"], inputs)
	}
	for i := range inputs {
		if gotInputs[i] != inputs[i] {
			t.Errorf("facts[inputs][%d] = %q, want %q", i, gotInputs[i], inputs[i])
		}
	}
	if _, ok := res.Facts["warnings"].([]string); !ok {
		t.Errorf("facts[warnings] = %#v, want []string", res.Facts["warnings"])
	}
}

// TestMergeFacts 是不依赖 Python oracle 的正式 facts 断言：两卷同名资源
// 触发改名，改名数与 Result.Renames 一致。
func TestMergeFacts(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.epub")
	second := filepath.Join(dir, "second.epub")
	out := filepath.Join(dir, "merged.epub")
	buildEpub(t, first, writeBookEntries("第一册", "book-a", []byte("cover-a")))
	buildEpub(t, second, writeBookEntries("第二册", "book-b", []byte("cover-b")))
	b, err := book.Open(first)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{
		Inputs: []string{first, second},
		Title:  strPtr("合集"),
		Output: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	merged, _ := res.Facts["mergedItems"].(int)
	renamed, _ := res.Facts["renamedResources"].(int)
	if merged == 0 || renamed == 0 {
		t.Fatalf("mergedItems=%d renamedResources=%d, want both > 0", merged, renamed)
	}
	if len(res.Renames) != renamed {
		t.Errorf("len(Renames) = %d, want renamedResources %d", len(res.Renames), renamed)
	}
	assertOperationFacts(t, res, []string{first, second}, out, merged, renamed)
}

// TestMergeRewritesConflictingResourceReferences 原为 Python oracle 的 P3
// parity 用例（oracle 已于 2026-08-29 删除，`epub_package_merge_harness.py`
// 不复存在）。这里保留同一组「两卷同名资源冲突」fixture，改为按合并领域
// 语义手写的 Go-native 断言，而不是回填 Go 自身当前输出：
//
//   - 计数：fixture 里每卷 manifest 贡献 3 个非 nav 资源（chap/style/
//     cover-image），两卷共 mergedItems=6；vol2 的三个资源与 vol1 同名冲突，
//     全部改名，renamedResources=3（allocateArchivePath 只在 used[candidate]
//     已存在时才改名，vol1 先注册、不冲突，vol2 后到、全部冲突）。
//   - 冲突改名后引用确实被重写：vol2 的 chapter.xhtml / main.css 改名前
//     指向 "../Images/cover.jpg"，改名后必须指向改名结果
//     "../Images/vol2_cover.jpg"（rewriteURI 按 pathMap 把 oldTarget 换成
//     finalPath 再算相对路径）。
//   - 「dcterms:modified 之外的 entry 逐字节不变」：vol1 的资源全程未改名
//     （pathMap 是恒等映射），transformResource 应完全不产出编辑，因此
//     vol1 的 chapter.xhtml/main.css/cover.jpg 必须与源 fixture 逐字节
//     相同（INV-1 的透传语义），OPF 里只有 dcterms:modified 是不确定字段。
func TestMergeRewritesConflictingResourceReferences(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.epub")
	second := filepath.Join(dir, "second.epub")
	out := filepath.Join(dir, "merged.epub")
	buildEpub(t, first, writeBookEntries("第一册", "book-a", []byte("cover-a")))
	buildEpub(t, second, writeBookEntries("第二册", "book-b", []byte("cover-b")))

	b, err := book.Open(first)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{
		Inputs: []string{first, second},
		Title:  strPtr("合集"),
		Output: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	if err := b.WriteTo(out); err != nil {
		t.Fatal(err)
	}

	assertOperationFacts(t, res, []string{first, second}, out, 6, 3)
	entries := readZipEntries(t, out)

	// vol1 未与任何路径冲突：三个资源应逐字节保留（未改名 ⇒ transformResource
	// 无编辑），只有 OPF 里的 dcterms:modified 允许变化，而 vol1 的这三个
	// entry 根本不在 OPF 里。
	for _, e := range writeBookEntries("第一册", "book-a", []byte("cover-a")) {
		switch e.name {
		case "OEBPS/Text/chapter.xhtml", "OEBPS/Styles/main.css", "OEBPS/Images/cover.jpg":
			got, ok := entries[e.name]
			if !ok {
				t.Fatalf("vol1 资源 %s 应保留在合并产物里", e.name)
			}
			if !bytes.Equal(got, e.content) {
				t.Errorf("vol1 资源 %s 应逐字节不变:\n got  = %s\n want = %s", e.name, got, e.content)
			}
		}
	}

	// vol2 三个资源全部与 vol1 冲突改名，改名结果里的本地引用同步指向
	// 新路径：改前指向 "../Images/cover.jpg"，改后指向
	// "../Images/vol2_cover.jpg"。
	vol2Chapter, ok := entries["OEBPS/Text/vol2_chapter.xhtml"]
	if !ok {
		t.Fatal("vol2 的 chapter.xhtml 应改名为 vol2_chapter.xhtml")
	}
	if !bytes.Contains(vol2Chapter, []byte(`src="../Images/vol2_cover.jpg"`)) {
		t.Errorf("vol2_chapter.xhtml 的 img src 应指向改名后的封面: %s", vol2Chapter)
	}
	if bytes.Contains(vol2Chapter, []byte(`"../Images/cover.jpg"`)) {
		t.Errorf("vol2_chapter.xhtml 不应再引用改名前的路径: %s", vol2Chapter)
	}
	vol2CSS, ok := entries["OEBPS/Styles/vol2_main.css"]
	if !ok {
		t.Fatal("vol2 的 main.css 应改名为 vol2_main.css")
	}
	if !bytes.Contains(vol2CSS, []byte(`url('../Images/vol2_cover.jpg')`)) {
		t.Errorf("vol2_main.css 的 url() 应指向改名后的封面: %s", vol2CSS)
	}
	if bytes.Contains(vol2CSS, []byte(`'../Images/cover.jpg'`)) {
		t.Errorf("vol2_main.css 不应再引用改名前的路径: %s", vol2CSS)
	}
	vol2Cover, ok := entries["OEBPS/Images/vol2_cover.jpg"]
	if !ok {
		t.Fatal("vol2 的封面应改名为 vol2_cover.jpg")
	}
	if string(vol2Cover) != "cover-b" {
		t.Errorf("vol2_cover.jpg 内容应是 vol2 的原封面字节: %q", vol2Cover)
	}

	// Renames 精确等于这三条改名（TestMergeExposesRenamesAsPathMapFact 已
	// 覆盖 Renames 与 facts.mappings 的通用一致性，这里锁定具体值）。
	wantRenames := map[string]string{
		"OEBPS/Text/chapter.xhtml": "OEBPS/Text/vol2_chapter.xhtml",
		"OEBPS/Styles/main.css":    "OEBPS/Styles/vol2_main.css",
		"OEBPS/Images/cover.jpg":   "OEBPS/Images/vol2_cover.jpg",
	}
	if len(res.Renames) != len(wantRenames) {
		t.Fatalf("Renames = %v, want %v", res.Renames, wantRenames)
	}
	for from, to := range wantRenames {
		if res.Renames[from] != to {
			t.Errorf("Renames[%q] = %q, want %q", from, res.Renames[from], to)
		}
	}

	// container.xml 固定指向合并产物的 OPF；OPF 标题取自 --title，
	// dcterms:modified 是唯一允许变化的字段（见 opfbuild.go 头注：两者
	// 都是从零新建的 ET 输出，格式已在源码注释里逐字记录）。
	containerXML := string(entries["META-INF/container.xml"])
	if !strings.Contains(containerXML, `full-path="OEBPS/content.opf"`) {
		t.Errorf("container.xml 应指向 OEBPS/content.opf: %s", containerXML)
	}
	opf := string(entries["OEBPS/content.opf"])
	if !strings.Contains(opf, "<dc:title>合集</dc:title>") {
		t.Errorf("OPF 标题应为传入的 --title: %s", opf)
	}
	if !modifiedTSRe.MatchString(opf) {
		t.Errorf("OPF 应含格式合法的 dcterms:modified: %s", opf)
	}
	if !strings.Contains(opf, `href="Text/vol2_chapter.xhtml"`) ||
		!strings.Contains(opf, `href="Images/vol2_cover.jpg"`) {
		t.Errorf("OPF manifest 应引用改名后的 vol2 资源: %s", opf)
	}
}

func TestMergeRefusesEncryptedAndShortInput(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.epub")
	buildEpub(t, src, writeBookEntries("书", "m", []byte("c")))
	encrypted := append(writeBookEntries("书", "m", []byte("c")), zipEntry{
		name:    "META-INF/encryption.xml",
		content: []byte(`<?xml version="1.0"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"/>`),
	})
	encPath := filepath.Join(dir, "enc.epub")
	buildEpub(t, encPath, encrypted)

	b, err := book.Open(encPath)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{
		Inputs: []string{encPath, src}, Output: filepath.Join(dir, "out.epub"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed {
		t.Errorf("加密输入应 failed，got %s", res.Status)
	}
	if len(res.Findings) == 0 || res.Findings[0].Title !=
		"merge: encrypted EPUB resources detected; refusing package rewrite" {
		t.Errorf("拒绝措辞不一致: %+v", res.Findings)
	}

	b2, err := book.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()
	res2, err := Run(context.Background(), b2, Params{Inputs: []string{src}})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Status != report.StatusFailed || res2.Findings[0].Title != "merge requires at least two input EPUB files" {
		t.Errorf("单卷输入应拒绝: %+v", res2.Findings)
	}
}

func stringsHasSuffix(s, suf string) bool {
	return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
}
