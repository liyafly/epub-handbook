package migrateepub3

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// ---- fixture（逐字对齐 scripts/test_epub3_oneclick_converter.py 的
// write_legacy_epub 与 test_support.epub_fixture.write_epub） ----

type legacyOptions struct {
	bodyClass         string
	css               string
	extraMetadata     string
	chapterNoteMarkup string
	extraManifestItem string
	extraFiles        map[string]string
	missingLanguage   bool // 去掉两页的 xml:lang
	noPackageLanguage bool // 打包前剥掉 OPF 的 dc:language
	minifiedChapter   bool
	existingNav       bool // 追加 properties="nav" 的 EPUB3 nav item
}

const legacyNoteMarkup = `<p>正文<a id="w1"></a><a href="chapter.xhtml#m1"><sup>[1]</sup></a>继续。</p>` + "\n" +
	`    <hr/>` + "\n" +
	`    <p class="note"><a id="m1"></a><a href="chapter.xhtml#w1">[1]</a> 注释正文保留。</p>`

const legacyOPFTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:identifier id="book-id">urn:uuid:test-oneclick</dc:identifier>
    <dc:title>Oneclick Fixture</dc:title>
    <dc:creator>Test</dc:creator>
    <dc:language>zh-CN</dc:language>
    <dc:date opf:event="publication">2026-01-01</dc:date>
    <meta name="cover" content="cover-img"/>
{extra_metadata}  </metadata>
  <manifest>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="css" href="Styles/main.css" media-type="text/css"/>
    <item id="cover-page" href="Text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="cover-img" href="Images/cover.jpg" media-type="image/jpeg"/>
{extra_manifest_items}  </manifest>
  <spine toc="ncx">
    <itemref idref="cover-page"/>
    <itemref idref="chapter"/>
  </spine>
  <guide>
    <reference type="cover" title="Cover" href="../Text/cover.xhtml"/>
  </guide>
</package>
`

const legacyNCX = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE ncx PUBLIC "-//NISO//DTD ncx 2005-1//EN" "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd">
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <head><meta name="dtb:uid" content="urn:uuid:test-oneclick"/></head>
  <docTitle><text>Oneclick Fixture</text></docTitle>
  <navMap>
    <navPoint id="navPoint-1" playOrder="1">
      <navLabel><text>第一章</text></navLabel>
      <content src="Text/chapter.xhtml"#c1/>
    </navPoint>
  </navMap>
</ncx>
`

const legacyCover = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN"
  "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh-CN">
  <head><title>Cover</title></head>
  <body><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"></svg></body>
</html>
`

const legacyChapter = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN"
  "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh-CN">
  <head>
    <title>第一章</title>
    <meta http-equiv="Content-Type" content="application/xhtml+xml; charset=utf-8"/>
    <link rel="stylesheet" type="text/css" href="../Styles/main.css"/>
  </head>
  <body>
    <h1 id="c1">第一章</h1>
    {note_markup}
  </body>
</html>
`

func buildLegacyFixture(opts legacyOptions) []fixtureEntry {
	noteMarkup := opts.chapterNoteMarkup
	if noteMarkup == "" {
		noteMarkup = legacyNoteMarkup
	}
	opf := strings.ReplaceAll(legacyOPFTemplate, "{extra_metadata}", opts.extraMetadata)
	opf = strings.ReplaceAll(opf, "{extra_manifest_items}", opts.extraManifestItem)
	if opts.noPackageLanguage {
		opf = strings.Replace(opf, "    <dc:language>zh-CN</dc:language>\n", "", 1)
	}
	chapter := strings.ReplaceAll(legacyChapter, "{note_markup}", noteMarkup)
	if opts.minifiedChapter {
		chapter = `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE html>` +
			`<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh-CN">` +
			`<head><title>第一章</title></head><body><h1 id="c1">第一章</h1>` +
			noteMarkup + `</body></html>`
	}
	if opts.bodyClass != "" {
		chapter = strings.Replace(chapter, "<body>", `<body class="`+opts.bodyClass+`">`, 1)
	}
	if opts.missingLanguage {
		chapter = strings.ReplaceAll(chapter, ` xml:lang="zh-CN"`, "")
	}
	css := opts.css
	if css == "" {
		css = "body { line-height: 1.4; }"
	}
	entries := []fixtureEntry{
		{name: "META-INF/container.xml", content: `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`},
		{name: "OEBPS/content.opf", content: opf},
		{name: "OEBPS/toc.ncx", content: legacyNCX},
		{name: "OEBPS/Text/cover.xhtml", content: legacyCover},
		{name: "OEBPS/Text/chapter.xhtml", content: chapter},
		{name: "OEBPS/Styles/main.css", content: css},
		{name: "OEBPS/Images/cover.jpg", content: "jpeg"},
	}
	if opts.existingNav {
		navXHTML := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN" lang="zh-CN">
  <head><title>目录</title></head>
  <body>
    <nav epub:type="toc" id="toc">
      <h1>Oneclick Fixture</h1>
      <ol>
        <li><a href="chapter.xhtml">第一章</a></li>
      </ol>
    </nav>
  </body>
</html>
`
		entries[1].content = strings.Replace(entries[1].content,
			`    <item id="cover-page" href="Text/cover.xhtml" media-type="application/xhtml+xml"/>`,
			`    <item id="cover-page" href="Text/cover.xhtml" media-type="application/xhtml+xml"/>`+"\n"+
				`    <item id="nav" href="Text/nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`, 1)
		entries = append(entries, fixtureEntry{name: "OEBPS/Text/nav.xhtml", content: navXHTML})
	}
	for name, content := range opts.extraFiles {
		entries = append(entries, fixtureEntry{name: name, content: content})
	}
	return entries
}

type fixtureEntry struct {
	name    string
	content string
}

// writeFixtureEpub 复刻 test_support.epub_fixture.write_epub：mimetype
// STORED 在前，其余 DEFLATED 按成员序。
func writeFixtureEpub(t *testing.T, path string, entries []fixtureEntry) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	fw, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("application/epub+zip")); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(e.content)); err != nil {
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

// ---- 测试工具 ----

// runGo 模拟 pipeline 的调用方式：Open → Run → WriteTo。
func runGo(t *testing.T, fixture, output string, p Params) (report.Result, error) {
	t.Helper()
	b, err := book.Open(fixture)
	if err != nil {
		t.Fatalf("book.Open: %v", err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, p)
	if err != nil {
		return res, err
	}
	if !p.DryRun {
		if err := b.WriteTo(output); err != nil {
			t.Fatalf("WriteTo: %v", err)
		}
	}
	return res, nil
}

func defaultParams(output string) Params {
	_ = output // 输出路径由 pipeline 落盘；本包 Params 不再携带。
	return Params{PopupNotes: true, Typography: true}
}

func openZip(t *testing.T, path string) *zip.ReadCloser {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("打开 %s: %v", path, err)
	}
	t.Cleanup(func() { zr.Close() })
	return zr
}

func zipRead(t *testing.T, zr *zip.ReadCloser, name string) []byte {
	t.Helper()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("打开 entry %s: %v", name, err)
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("读取 entry %s: %v", name, err)
			}
			return data
		}
	}
	t.Fatalf("entry %s 不存在", name)
	return nil
}

// resultFacts 是统一信封 facts 的测试视图（camelCase 键与 buildResult 一致）。
type resultFacts struct {
	OPF                   string   `json:"opf"`
	PackageVersionBefore  *string  `json:"packageVersionBefore"`
	NavEntries            int      `json:"navEntries"`
	XHTMLFilesUpdated     int      `json:"xhtmlFilesUpdated"`
	StylesheetLinksAdded  int      `json:"stylesheetLinksAdded"`
	PlainNotesConverted   int      `json:"plainNotesConverted"`
	DuokanNotesNormalized int      `json:"duokanNotesNormalized"`
	ManifestItemsAdded    []string `json:"manifestItemsAdded"`
	ManifestItemsUpdated  int      `json:"manifestItemsUpdated"`
	MetadataUpdates       []string `json:"metadataUpdates"`
	TypographyRoles       []string `json:"typographyRoles"`
	Warnings              []string `json:"warnings"`
	PopupNotes            bool     `json:"popupNotes"`
	Typography            bool     `json:"typography"`
}

// factsOf 经 JSON 往返读取 Result.Facts，同时保证 facts 可序列化。
func factsOf(t *testing.T, res report.Result) resultFacts {
	t.Helper()
	raw, err := json.Marshal(res.Facts)
	if err != nil {
		t.Fatalf("facts 不可序列化: %v", err)
	}
	var out resultFacts
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("facts 解码失败: %v\n%s", err, raw)
	}
	return out
}

// ---- 语义测试（镜像 scripts/test_epub3_oneclick_converter.py） ----

func TestInlineOnlyParagraphFormatting(t *testing.T) {
	source := `<?xml version="1.0"?><!DOCTYPE html>` +
		`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>x</title></head>` +
		`<body><p><span>甲</span><span>乙</span></p></body></html>`
	formatted, changed := formatXHTMLMultiline(source)
	if !changed {
		t.Fatal("应报告 changed")
	}
	if !strings.Contains(formatted, "<p><span>甲</span><span>乙</span></p>") {
		t.Fatalf("行内内容不应缩进:\n%s", formatted)
	}
}

func TestOneclickDefaultFixture(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "legacy.epub")
	output := filepath.Join(dir, "converted.epub")
	writeFixtureEpub(t, fixture, buildLegacyFixture(legacyOptions{minifiedChapter: true}))

	res, err := runGo(t, fixture, output, defaultParams(output))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	rep := factsOf(t, res)
	if rep.PlainNotesConverted != 1 || rep.NavEntries != 1 || rep.StylesheetLinksAdded != 2 {
		t.Fatalf("报告计数错误: %+v", rep)
	}
	if want := []string{"type-body", "type-title", "type-subtitle", "type-quote", "type-note", "type-emphasis", "type-meta"}; !reflect.DeepEqual(rep.TypographyRoles, want) {
		t.Fatalf("typography_roles 错误: %v", rep.TypographyRoles)
	}

	zr := openZip(t, output)
	if len(zr.File) == 0 || zr.File[0].Name != "mimetype" || zr.File[0].Method != zip.Store {
		t.Fatal("第一个 entry 应为 STORED 的 mimetype")
	}
	if got := zipRead(t, zr, "mimetype"); string(got) != "application/epub+zip" {
		t.Fatalf("mimetype 内容错误: %q", got)
	}

	opf := string(zipRead(t, zr, "OEBPS/content.opf"))
	root, perr := parseXMLTree(zipRead(t, zr, "OEBPS/content.opf"))
	if perr != nil {
		t.Fatal(perr)
	}
	_ = opf
	if v, _ := root.getAttr("version"); v != "3.0" {
		t.Fatalf("version 应为 3.0，实际 %q", v)
	}
	meta := root.childByTag(opfURI, "metadata")
	for _, m := range meta.childrenByTag(opfURI, "meta") {
		if prop, _ := m.getAttr("property"); prop == "ibooks:specified-fonts" {
			t.Fatal("free-mode 书不应获得 ibooks:specified-fonts")
		}
	}
	if prefix, _ := root.getAttr("prefix"); strings.Contains(prefix, "ibooks:") {
		t.Fatal("free-mode 书不应获得 ibooks prefix")
	}
	manifest := root.childByTag(opfURI, "manifest")
	navCount := 0
	hasCSS, hasNote := false, false
	for _, item := range manifest.childrenByTag(opfURI, "item") {
		props := pySplitWS(item.attrOr("properties", ""))
		if containsString(props, "nav") {
			navCount++
		}
		switch item.attrOr("href", "") {
		case "Styles/epub3-enhancements.css":
			hasCSS = true
		case "Images/note.png":
			hasNote = true
		}
		if id, _ := item.getAttr("id"); id == "cover-img" {
			if !containsString(props, "cover-image") {
				t.Fatal("cover-img 应有 cover-image 属性")
			}
		}
		if id, _ := item.getAttr("id"); id == "cover-page" {
			if !containsString(props, "svg") {
				t.Fatal("cover-page 应有 svg 属性")
			}
		}
	}
	if navCount != 1 || !hasCSS || !hasNote {
		t.Fatalf("manifest 检查失败: nav=%d css=%t note=%t", navCount, hasCSS, hasNote)
	}
	if !strings.Contains(opf, `href="Text/cover.xhtml"`) {
		t.Fatal("guide href 应被修正为 Text/cover.xhtml")
	}

	if ncx := string(zipRead(t, zr, "OEBPS/toc.ncx")); !strings.Contains(ncx, `src="Text/chapter.xhtml#c1"`) {
		t.Fatalf("NCX 坏引号未修复:\n%s", ncx)
	}
	navFound := false
	for _, f := range zr.File {
		if f.Name == "OEBPS/nav.xhtml" {
			navFound = true
		}
	}
	if !navFound {
		t.Fatal("缺少 OEBPS/nav.xhtml")
	}
	nav := string(zipRead(t, zr, "OEBPS/nav.xhtml"))
	if !strings.Contains(nav, `href="Text/cover.xhtml"`) {
		t.Fatalf("nav 缺少 landmarks 引用:\n%s", nav)
	}

	chapter := string(zipRead(t, zr, "OEBPS/Text/chapter.xhtml"))
	for _, want := range []string{
		`xmlns:epub="http://www.idpf.org/2007/ops"`,
		`href="../Styles/epub3-enhancements.css"`,
		`<sup class="note-marker">`,
		`class="noteref-icon" epub:type="noteref" role="doc-noteref"`,
		`class="footnote-list"`,
		`role="doc-backlink"`,
		"注释正文保留。",
		"\n  <head>",
		"\n  <body>",
	} {
		if !strings.Contains(chapter, want) {
			t.Errorf("chapter 缺少 %q:\n%s", want, chapter)
		}
	}
	enhancement := string(zipRead(t, zr, "OEBPS/Styles/epub3-enhancements.css"))
	for _, want := range []string{".type-quote", `@namespace epub "http://www.idpf.org/2007/ops";`} {
		if !strings.Contains(enhancement, want) {
			t.Errorf("enhancement css 缺少 %q", want)
		}
	}
}

func TestLockedModeCase(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "legacy-locked.epub")
	output := filepath.Join(dir, "converted-locked.epub")
	writeFixtureEpub(t, fixture, buildLegacyFixture(legacyOptions{bodyClass: "body-font-locked"}))

	res, err := runGo(t, fixture, output, defaultParams(output))
	if err != nil {
		t.Fatal(err)
	}
	rep := factsOf(t, res)
	found := false
	for _, u := range rep.MetadataUpdates {
		if u == "added ibooks:specified-fonts (locked body font detected)" {
			found = true
		}
	}
	if !found {
		t.Fatalf("缺少 specified-fonts 更新: %v", rep.MetadataUpdates)
	}
	zr := openZip(t, output)
	root, perr := parseXMLTree(zipRead(t, zr, "OEBPS/content.opf"))
	if perr != nil {
		t.Fatal(perr)
	}
	meta := root.childByTag(opfURI, "metadata")
	count := 0
	for _, m := range meta.childrenByTag(opfURI, "meta") {
		if prop, _ := m.getAttr("property"); prop == "ibooks:specified-fonts" {
			count++
			if m.text != "true" {
				t.Fatalf("specified-fonts 应为 true，实际 %q", m.text)
			}
		}
	}
	if count != 1 {
		t.Fatalf("specified-fonts 数量错误: %d", count)
	}
	if prefix, _ := root.getAttr("prefix"); !strings.Contains(prefix, "ibooks:") {
		t.Fatal("locked 书应有 ibooks prefix")
	}
}

func TestDirectLockedModeCase(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "legacy-direct-locked.epub")
	output := filepath.Join(dir, "converted-direct-locked.epub")
	writeFixtureEpub(t, fixture, buildLegacyFixture(legacyOptions{
		css: `body { font-family: "cnepub", serif; line-height: 1.4; }`,
	}))

	res, err := runGo(t, fixture, output, defaultParams(output))
	if err != nil {
		t.Fatal(err)
	}
	rep := factsOf(t, res)
	found := false
	for _, u := range rep.MetadataUpdates {
		if u == "added ibooks:specified-fonts (locked body font detected)" {
			found = true
		}
	}
	if !found {
		t.Fatalf("直接 body 字体锁定应被识别: %v", rep.MetadataUpdates)
	}
}

func TestParagraphFontIsNotLockedCase(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "legacy-paragraph-font.epub")
	output := filepath.Join(dir, "converted-paragraph-font.epub")
	writeFixtureEpub(t, fixture, buildLegacyFixture(legacyOptions{
		css: `p { font-family: "cnepub", serif; line-height: 1.4; }`,
	}))

	res, err := runGo(t, fixture, output, defaultParams(output))
	if err != nil {
		t.Fatal(err)
	}
	rep := factsOf(t, res)
	for _, u := range rep.MetadataUpdates {
		if strings.Contains(u, "added ibooks:specified-fonts") {
			t.Fatalf("段落级字体规则不应视为全书锁定: %v", rep.MetadataUpdates)
		}
	}
	zr := openZip(t, output)
	root, perr := parseXMLTree(zipRead(t, zr, "OEBPS/content.opf"))
	if perr != nil {
		t.Fatal(perr)
	}
	meta := root.childByTag(opfURI, "metadata")
	for _, m := range meta.childrenByTag(opfURI, "meta") {
		if prop, _ := m.getAttr("property"); prop == "ibooks:specified-fonts" {
			t.Fatal("段落级字体规则不应写入 ibooks:specified-fonts")
		}
	}
}

func TestIbooksPrefixCase(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "legacy-ibooks-version.epub")
	output := filepath.Join(dir, "converted-ibooks-version.epub")
	writeFixtureEpub(t, fixture, buildLegacyFixture(legacyOptions{
		extraMetadata: `    <meta property="ibooks:version">1.0</meta>` + "\n",
	}))

	res, err := runGo(t, fixture, output, defaultParams(output))
	if err != nil {
		t.Fatal(err)
	}
	rep := factsOf(t, res)
	for _, u := range rep.MetadataUpdates {
		if u == "kept existing ibooks:specified-fonts" {
			t.Fatalf("free-mode 书不应报 kept existing: %v", rep.MetadataUpdates)
		}
	}
	zr := openZip(t, output)
	root, perr := parseXMLTree(zipRead(t, zr, "OEBPS/content.opf"))
	if perr != nil {
		t.Fatal(perr)
	}
	meta := root.childByTag(opfURI, "metadata")
	for _, m := range meta.childrenByTag(opfURI, "meta") {
		if prop, _ := m.getAttr("property"); prop == "ibooks:specified-fonts" {
			t.Fatal("free-mode 书不应获得 ibooks:specified-fonts")
		}
	}
	if prefix, _ := root.getAttr("prefix"); !strings.Contains(prefix, "ibooks:") {
		t.Fatal("其它 ibooks 属性应保留 ibooks prefix")
	}
}

func TestCustomImageNoterefCase(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "legacy-image-note.epub")
	output := filepath.Join(dir, "converted-image-note.epub")
	writeFixtureEpub(t, fixture, buildLegacyFixture(legacyOptions{
		chapterNoteMarkup: `<p>正文<a id="w1" class="noteref-icon" epub:type="noteref" role="doc-noteref" href="#m1">` +
			`<img alt="注" src="../Images/custom-note.png"/></a>继续。</p>` + "\n" +
			`    <hr/>` + "\n" +
			`    <p class="note"><a id="m1"></a><a href="chapter.xhtml#w1">[1]</a> 注释正文保留。</p>`,
		extraManifestItem: `    <item id="custom-note" href="Images/custom-note.png" media-type="image/png"/>` + "\n",
		extraFiles:        map[string]string{"OEBPS/Images/custom-note.png": "png"},
	}))

	res, err := runGo(t, fixture, output, defaultParams(output))
	if err != nil {
		t.Fatal(err)
	}
	rep := factsOf(t, res)
	if rep.PlainNotesConverted != 1 {
		t.Fatalf("plain_notes_converted 应为 1: %+v", rep)
	}
	zr := openZip(t, output)
	chapter := string(zipRead(t, zr, "OEBPS/Text/chapter.xhtml"))
	if !strings.Contains(chapter, `src="../Images/custom-note.png"`) {
		t.Fatal("应保留自定义图标引用")
	}
	if strings.Contains(chapter, `src="../Images/note.png"`) {
		t.Fatal("不应注入默认图标")
	}
	hasNotePNG, hasCustomPNG := false, false
	for _, f := range zr.File {
		if f.Name == "OEBPS/Images/note.png" {
			hasNotePNG = true
		}
		if f.Name == "OEBPS/Images/custom-note.png" {
			hasCustomPNG = true
		}
	}
	if hasNotePNG || !hasCustomPNG {
		t.Fatalf("产物 entry 错误: note.png=%t custom=%t", hasNotePNG, hasCustomPNG)
	}
	root, perr := parseXMLTree(zipRead(t, zr, "OEBPS/content.opf"))
	if perr != nil {
		t.Fatal(perr)
	}
	manifest := root.childByTag(opfURI, "manifest")
	for _, item := range manifest.childrenByTag(opfURI, "item") {
		if item.attrOr("href", "") == "Images/note.png" {
			t.Fatal("manifest 不应包含默认 note.png")
		}
	}
}

func TestSigilLegacyNotesCase(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "sigil-legacy-notes.epub")
	output := filepath.Join(dir, "converted-sigil-legacy-notes.epub")
	writeFixtureEpub(t, fixture, buildLegacyFixture(legacyOptions{
		chapterNoteMarkup: `<p>正文<sup><a id="noteref_1" href="#footnote_1" epub:type="noteref">[1]</a></sup>` +
			`继续<sup><a id="noteref_2" href="#footnote_2" epub:type="noteref">[2]</a></sup>。</p>` + "\n" +
			`    <section class="fnote" epub:type="footnotes">` + "\n" +
			`      <aside id="footnote_1" epub:type="footnote"><p>` +
			`<a href="#noteref_1" epub:type="noteref">[1]</a> 第一条注释正文保留。</p></aside>` + "\n" +
			`      <aside id="footnote_2" epub:type="footnote"><p>` +
			`<a href="#noteref_2" epub:type="noteref">[2]</a> 第二条注释正文保留。</p></aside>` + "\n" +
			`    </section>`,
	}))

	res, err := runGo(t, fixture, output, defaultParams(output))
	if err != nil {
		t.Fatal(err)
	}
	rep := factsOf(t, res)
	if rep.PlainNotesConverted != 2 {
		t.Fatalf("sigil 弹注应转换 2 条: %+v", rep)
	}
	zr := openZip(t, output)
	chapter := string(zipRead(t, zr, "OEBPS/Text/chapter.xhtml"))
	for _, check := range []struct {
		want      string
		wantCount int
	}{
		{`<aside epub:type="footnote" role="doc-footnote">`, 1},
		{`class="footnote-item"`, 2},
		{`<sup class="note-marker">`, 2},
	} {
		if got := strings.Count(chapter, check.want); got != check.wantCount {
			t.Errorf("%q 出现 %d 次，应为 %d", check.want, got, check.wantCount)
		}
	}
	for _, banned := range []string{`<sup><a id="noteref_1"`} {
		if strings.Contains(chapter, banned) {
			t.Errorf("不应残留 %q", banned)
		}
	}
	for _, want := range []string{
		`id="noteref_1" class="noteref-icon"`,
		`id="noteref_2" class="noteref-icon"`,
		`id="footnote_1"`,
		`id="footnote_2"`,
		`href="#noteref_1">◎</a>第一条注释正文保留。`,
		`href="#noteref_2">◎</a>第二条注释正文保留。`,
	} {
		if !strings.Contains(chapter, want) {
			t.Errorf("chapter 缺少 %q:\n%s", want, chapter)
		}
	}
}

func TestSigilPartialSectionCase(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "sigil-partial-notes.epub")
	output := filepath.Join(dir, "converted-sigil-partial-notes.epub")
	writeFixtureEpub(t, fixture, buildLegacyFixture(legacyOptions{
		chapterNoteMarkup: `<p>正文<sup><a id="noteref_1" href="#footnote_1" epub:type="noteref">[1]</a></sup>继续。</p>` + "\n" +
			`    <section epub:type="footnotes">` + "\n" +
			`      <aside id="footnote_1" epub:type="footnote"><p>` +
			`<a href="#noteref_1" epub:type="noteref">[1]</a> 注释正文保留。</p></aside>` + "\n" +
			`      <p>不能自动识别的残余内容。</p>` + "\n" +
			`    </section>`,
	}))

	res, err := runGo(t, fixture, output, defaultParams(output))
	if err != nil {
		t.Fatal(err)
	}
	rep := factsOf(t, res)
	if rep.PlainNotesConverted != 0 {
		t.Fatalf("残余内容应阻止自动转换: %+v", rep)
	}
	zr := openZip(t, output)
	chapter := string(zipRead(t, zr, "OEBPS/Text/chapter.xhtml"))
	for _, want := range []string{
		`<section epub:type="footnotes">`,
		`id="noteref_1" href="#footnote_1" epub:type="noteref">[1]</a>`,
		"不能自动识别的残余内容。",
	} {
		if !strings.Contains(chapter, want) {
			t.Errorf("chapter 缺少 %q", want)
		}
	}
}

func TestMissingHTMLLanguageCase(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "legacy-missing-language.epub")
	output := filepath.Join(dir, "converted-missing-language.epub")
	writeFixtureEpub(t, fixture, buildLegacyFixture(legacyOptions{missingLanguage: true}))

	if _, err := runGo(t, fixture, output, defaultParams(output)); err != nil {
		t.Fatal(err)
	}
	zr := openZip(t, output)
	for _, name := range []string{"OEBPS/Text/chapter.xhtml", "OEBPS/Text/cover.xhtml"} {
		page := string(zipRead(t, zr, name))
		if !strings.Contains(page, `lang="zh-CN"`) || !strings.Contains(page, `xml:lang="zh-CN"`) {
			t.Errorf("%s 缺少语言补齐:\n%s", name, page)
		}
	}
}

func TestNonNoteSupCase(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "legacy-non-note-sup.epub")
	output := filepath.Join(dir, "converted-non-note-sup.epub")
	writeFixtureEpub(t, fixture, buildLegacyFixture(legacyOptions{
		chapterNoteMarkup: `<p>水的式子是 H<sup>2</sup>O。<a id="w1"></a><a href="chapter.xhtml#m1"><sup>[1]</sup></a></p>` + "\n" +
			`    <hr/>` + "\n" +
			`    <p class="note"><a id="m1"></a><a href="chapter.xhtml#w1">[1]</a> 注释正文保留。</p>`,
	}))

	if _, err := runGo(t, fixture, output, defaultParams(output)); err != nil {
		t.Fatal(err)
	}
	zr := openZip(t, output)
	chapter := string(zipRead(t, zr, "OEBPS/Text/chapter.xhtml"))
	if !strings.Contains(chapter, "H<sup>2</sup>O") {
		t.Fatalf("普通 sup 不应被标记:\n%s", chapter)
	}
	if strings.Contains(chapter, `H<sup class="note-marker">2</sup>O`) {
		t.Fatal("普通 sup 不应获得 note-marker")
	}
}

func TestMissingPackageLanguageCase(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "legacy-missing-package-language.epub")
	writeFixtureEpub(t, fixture, buildLegacyFixture(legacyOptions{
		missingLanguage:   true,
		noPackageLanguage: true,
	}))
	output := filepath.Join(dir, "converted-missing-package-language.epub")

	if _, err := runGo(t, fixture, output, defaultParams(output)); err != nil {
		t.Fatal(err)
	}
	zr := openZip(t, output)
	chapter := string(zipRead(t, zr, "OEBPS/Text/chapter.xhtml"))
	nav := string(zipRead(t, zr, "OEBPS/nav.xhtml"))
	if strings.Contains(chapter, `lang="zh-CN"`) || strings.Contains(chapter, `xml:lang="zh-CN"`) {
		t.Fatalf("无包级语言时不应补齐页面语言:\n%s", chapter)
	}
	if !strings.Contains(nav, `lang="und"`) {
		t.Fatalf("nav 应回退到 und:\n%s", nav)
	}
}

// ---- parity（Python oracle 对照，见 parity_test.go） ----

var _ = flate.BestSpeed

// ---- 信封回归（原 Python oracle parity 的 Go 原生替代） ----

// TestConversionFactsAreFormal 锁定 facts 键集合与形状：conversion report 的
// 全部字段都以 camelCase 出现在 Result.Facts，输入/输出 SHA-256 由 pipeline
// 信封的 input / output 段承担，不在本包重复。
func TestConversionFactsAreFormal(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "legacy.epub")
	output := filepath.Join(dir, "converted.epub")
	writeFixtureEpub(t, fixture, buildLegacyFixture(legacyOptions{minifiedChapter: true}))

	res, err := runGo(t, fixture, output, defaultParams(output))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	wantKeys := []string{
		"opf", "packageVersionBefore", "navEntries", "xhtmlFilesUpdated", "stylesheetLinksAdded",
		"plainNotesConverted", "duokanNotesNormalized", "manifestItemsAdded", "manifestItemsUpdated",
		"metadataUpdates", "typographyRoles", "warnings", "popupNotes", "typography",
	}
	for _, k := range wantKeys {
		if _, ok := res.Facts[k]; !ok {
			t.Errorf("facts 缺少 %q", k)
		}
	}
	if len(res.Facts) != len(wantKeys) {
		t.Errorf("facts 键数 = %d, want %d: %v", len(res.Facts), len(wantKeys), res.Facts)
	}
	rep := factsOf(t, res)
	if rep.OPF != "OEBPS/content.opf" || rep.PackageVersionBefore == nil || *rep.PackageVersionBefore != "2.0" {
		t.Errorf("opf/packageVersionBefore 错误: %+v", rep)
	}
	if !rep.PopupNotes || !rep.Typography {
		t.Errorf("开关回显错误: %+v", rep)
	}
	if rep.ManifestItemsAdded == nil || rep.MetadataUpdates == nil || rep.TypographyRoles == nil || rep.Warnings == nil {
		t.Errorf("列表 facts 必须是数组而非 null: %+v", rep)
	}
	for _, w := range rep.Warnings {
		found := false
		for _, f := range res.Findings {
			if f.ID == "migrate.warning" && f.Title == w {
				found = true
			}
		}
		if !found {
			t.Errorf("warning %q 未映射为 finding", w)
		}
	}
	if len(res.Events) != 1 || res.Events[0].Step != "convert" || res.Events[0].Status != "completed" {
		t.Errorf("events 错误: %+v", res.Events)
	}
}

// TestNoTypographyDisablesRoles 锁定 no_typography 分支：typographyRoles 为空
// 数组、开关回显为 false，其余转换计数不受影响。
func TestNoTypographyDisablesRoles(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "legacy.epub")
	output := filepath.Join(dir, "converted-no-typography.epub")
	writeFixtureEpub(t, fixture, buildLegacyFixture(legacyOptions{minifiedChapter: true}))

	params := defaultParams(output)
	params.Typography = false
	res, err := runGo(t, fixture, output, params)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	rep := factsOf(t, res)
	if rep.Typography || len(rep.TypographyRoles) != 0 {
		t.Fatalf("no_typography 下不应有 typography roles: %+v", rep)
	}
	if rep.PlainNotesConverted != 1 || rep.NavEntries != 1 {
		t.Fatalf("no_typography 不应影响弹注/nav 计数: %+v", rep)
	}
}
