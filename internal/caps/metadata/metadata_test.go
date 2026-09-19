package metadata

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/redline"
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

func writeBookEntries(title, marker string) []zipEntry {
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
    <meta property="dcterms:modified">2020-01-01T00:00:00Z</meta>
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
		{name: "OEBPS/Images/cover.jpg", content: []byte("cover")},
		{name: ".DS_Store", content: f("macos-metadata")},
		{name: "mimetype", content: f("wrong-on-purpose")},
	}
}

// assertOperationFacts 锁定 metadata.edit 的正式 facts 键（旧 OperationReport
// 的全部信息：operation / opf / output / fieldsUpdated）。
func assertOperationFacts(t *testing.T, res report.Result, output string, wantFields int) {
	t.Helper()
	want := map[string]any{
		"operation":     "metadata-write",
		"opf":           "OEBPS/content.opf",
		"output":        output,
		"fieldsUpdated": wantFields,
	}
	for k, v := range want {
		if got := res.Facts[k]; got != v {
			t.Errorf("facts[%q] = %#v, want %#v", k, got, v)
		}
	}
}

// TestMetadataWriteFacts 是不依赖 Python oracle 的正式 facts 断言：七个键
// 全部写入；title+subtitle 由 set_titles 合并计为一次更新，故 fieldsUpdated=6。
func TestMetadataWriteFacts(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("原题", "meta"))
	out := filepath.Join(dir, "metadata.epub")
	fieldsJSON := `{"title": "新题", "subtitle": "副题", "author": "新作者", "language": "zh-CN", "publisher": "新出版社", "description": "新简介", "rights": "版权声明"}`
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{MetadataJSON: fieldsJSON, Output: out})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	assertOperationFacts(t, res, out, 6)
}

// TestMetadataWriteUpdatesFieldValues 按字段语义手写期望值（不是"跑一遍拿
// 现在的输出当 golden"）：写入 dc:title / dc:creator / dc:language /
// dc:identifier / dc:publisher / dc:description / dc:rights 后，OPF 里
// 对应元素的值必须确实变成新值、旧值必须确实消失——防止任何一个字段被
// 漏接、接错标签，或者"看起来变了但其实值没换"。
func TestMetadataWriteUpdatesFieldValues(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("原题", "meta"))
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	fieldsJSON := `{"title": "新题", "subtitle": "副题", "author": "新作者", "language": "en", ` +
		`"identifier": "urn:uuid:updated-1234", "publisher": "新出版社", "description": "新简介", "rights": "版权声明"}`
	res, err := Run(context.Background(), b, Params{MetadataJSON: fieldsJSON})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	// 8 = 标题+副标题(setTitles 记 2) + creator/language/publisher/description/
	// identifier/rights 各 1（rights 原本不存在，追加计一次）。
	if got := res.Facts["fieldsUpdated"]; got != 8 {
		t.Fatalf("fieldsUpdated = %v, want 8", got)
	}

	opfData, err := b.Current("OEBPS/content.opf")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<dc:title id="main-title">新题</dc:title>`,
		`<dc:title id="subtitle">副题</dc:title>`,
		`<dc:creator>新作者</dc:creator>`,
		`<dc:language>en</dc:language>`,
		`<dc:identifier id="book-id">urn:uuid:updated-1234</dc:identifier>`,
		`<dc:publisher>新出版社</dc:publisher>`,
		`<dc:description>新简介</dc:description>`,
		`<dc:rights>版权声明</dc:rights>`,
	} {
		if !bytes.Contains(opfData, []byte(want)) {
			t.Errorf("OPF 缺少期望的字段值 %q\n完整 OPF:\n%s", want, opfData)
		}
	}
	// 旧值必须真的被替换掉，不是新旧并存（例如误插而不是替换）。
	for _, stale := range []string{">原题<", ">Author meta<", ">zh-CN<", ">urn:uuid:meta<", ">Publisher meta<", ">Description meta<"} {
		if bytes.Contains(opfData, []byte(stale)) {
			t.Errorf("OPF 仍残留旧值 %q，字段替换未生效", stale)
		}
	}
}

// TestMetadataWriteDoesNotTouchDctermsModified 锁定 metadata.go 文件头注释
// 声明的行为：字段写入全部走字节区间编辑，从不重算或改写
// dcterms:modified（这与 Python 版 ElementTree 整树重序列化的关键差异——
// deepcopy 语义下这段字节从未被触碰）。如果未来有人"顺手"给写入加上自动
// 打时间戳的逻辑，这条测试会先炸，而不是让下游误以为文件在写入后才修改过。
func TestMetadataWriteDoesNotTouchDctermsModified(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("原题", "meta"))
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{MetadataJSON: `{"title": "新题", "author": "新作者"}`})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	opfData, err := b.Current("OEBPS/content.opf")
	if err != nil {
		t.Fatal(err)
	}
	const modifiedElem = `<meta property="dcterms:modified">2020-01-01T00:00:00Z</meta>`
	if !bytes.Contains(opfData, []byte(modifiedElem)) {
		t.Errorf("dcterms:modified 被改写，OPF 中缺少原始字节:\n%s", opfData)
	}
}

// TestMetadataWritePreservesNonOPFBytes 锁定 INV-1 字节透传在 metadata.edit
// 上的体现：字段写入只应该碰 OPF，以及需要规范化的 mimetype；其余 entry
// （nav、正文、CSS、封面、甚至无关的 .DS_Store）必须与输入逐字节相同——
// 否则一次"改书名"的操作就可能悄悄改坏排版或图片。同时锁定 mimetype 规范
// 化：fixture 故意写了错误值，metadata.edit 应把它纠正为标准 MIME 值。
func TestMetadataWritePreservesNonOPFBytes(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("原题", "meta"))
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{MetadataJSON: `{"title": "新题"}`})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}

	opfPath, _ := res.Facts["opf"].(string)
	wantModified := map[string]bool{opfPath: true, "mimetype": true}
	modified := b.ModifiedNames()
	if len(modified) != len(wantModified) {
		t.Fatalf("modified entries = %v, want exactly %v", modified, wantModified)
	}
	for _, name := range modified {
		if !wantModified[name] {
			t.Errorf("metadata.edit 意外改动了 %s", name)
		}
	}

	for _, name := range b.OriginalNames() {
		if name == opfPath || name == "mimetype" {
			continue
		}
		orig, err := b.Original(name)
		if err != nil {
			t.Fatal(err)
		}
		cur, err := b.Current(name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(orig, cur) {
			t.Errorf("entry %s 字节被改动，违反 INV-1", name)
		}
	}

	mt, err := b.Current("mimetype")
	if err != nil {
		t.Fatal(err)
	}
	if string(mt) != canonicalMimetype {
		t.Errorf("mimetype = %q, want %q", mt, canonicalMimetype)
	}
}

// TestMetadataWriteOnlyTouchesMetadataRedline 用红线做门禁：只改 metadata
// 时，正文、spine 顺序与锚点必须纹丝不动。CheckMetadata 本来就会被这次写入
// 触发（这正是本能力的作用），故意不放进这一组检查，否则会掩盖真正的回归。
func TestMetadataWriteOnlyTouchesMetadataRedline(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("原题", "meta"))
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	fieldsJSON := `{"title": "新题", "author": "新作者", "language": "en", "publisher": "新出版社", "description": "新简介"}`
	res, err := Run(context.Background(), b, Params{MetadataJSON: fieldsJSON})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}

	findings, err := redline.Check(redline.OriginalState(b), redline.CurrentState(b),
		[]string{redline.CheckText, redline.CheckSpine, redline.CheckAnchors}, redline.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		t.Errorf("redline %s: %s", f.Check, f.Message)
	}
}

// TestMetadataWriteChainedEditsPreservePriorFields 验证写入→落盘→重新打开
// →再写入的链路：第二次只改 author 时，第一次追加的 subtitle / rights /
// title-type meta 必须原样保留，且第二次不该再碰任何第一次已经写定的字节。
// 这是 Python 版"整树重序列化"与 Go 版"字节区间编辑"两条路径最容易分叉的
// 地方：Go 侧第二次解析的是自己第一次输出的、已经包含追加元素的 OPF。
func TestMetadataWriteChainedEditsPreservePriorFields(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("原题", "meta"))

	out1 := filepath.Join(dir, "pass1.epub")
	b1, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b1.Close()
	firstJSON := `{"title": "新题", "subtitle": "副题", "rights": "版权声明"}`
	res1, err := Run(context.Background(), b1, Params{MetadataJSON: firstJSON, Output: out1})
	if err != nil {
		t.Fatal(err)
	}
	if res1.Status != report.StatusComplete {
		t.Fatalf("pass1 status = %s: %+v", res1.Status, res1.Findings)
	}
	if err := b1.WriteTo(out1); err != nil {
		t.Fatal(err)
	}

	b2, err := book.Open(out1)
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()
	res2, err := Run(context.Background(), b2, Params{MetadataJSON: `{"author": "再作者"}`})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Status != report.StatusComplete {
		t.Fatalf("pass2 status = %s: %+v", res2.Status, res2.Findings)
	}
	if got := res2.Facts["fieldsUpdated"]; got != 1 {
		t.Errorf("pass2 fieldsUpdated = %v, want 1（只改了 author）", got)
	}

	opfData, err := b2.Current("OEBPS/content.opf")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<dc:title id="main-title">新题</dc:title>`,
		`<dc:title id="subtitle">副题</dc:title>`,
		`<dc:rights>版权声明</dc:rights>`,
		`<dc:creator>再作者</dc:creator>`,
	} {
		if !bytes.Contains(opfData, []byte(want)) {
			t.Errorf("链式写入后缺少 %q，第一次的字段被第二次覆盖或丢失", want)
		}
	}

	// 第二次的输入（out1 的 mimetype）已经在第一次被规范化过，故第二次
	// 只应该再碰 OPF；mimetype 不该被判定为"又变了一次"。
	modified := b2.ModifiedNames()
	wantModified := map[string]bool{"OEBPS/content.opf": true}
	if len(modified) != len(wantModified) {
		t.Fatalf("pass2 modified entries = %v, want exactly %v", modified, wantModified)
	}
	for _, name := range modified {
		if !wantModified[name] {
			t.Errorf("pass2 意外改动了 %s", name)
		}
	}
}

func TestMetadataBadJSONRefused(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	buildEpub(t, source, writeBookEntries("书", "m"))
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{MetadataJSON: `{"title": 1}`})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusFailed ||
		res.Findings[0].Title != "metadata JSON must be an object of string fields" {
		t.Errorf("非法 metadata JSON 应拒绝: %+v", res.Findings)
	}
}
