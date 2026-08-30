package contentanalyze

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
)

// wrapXHTML 组装带 lang 的最小 XHTML（对齐 Python 测试的 xhtml() 助手）。
func wrapXHTML(body, bodyClass, language string) string {
	cls := ""
	if bodyClass != "" {
		cls = ` class="` + bodyClass + `"`
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="` + language + `" xml:lang="` + language + `">` + "\n")
	b.WriteString("  <head><title>Test</title></head>\n")
	b.WriteString("  <body" + cls + ">" + body + "</body>\n")
	b.WriteString("</html>")
	return b.String()
}

func roles(blocks []legacyBlock) []string {
	out := make([]string, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, b.PrimaryRole)
	}
	return out
}

func join(ss []string) string { return strings.Join(ss, ",") }

func TestStructureWinsOverMisleadingText(t *testing.T) {
	blocks, err := AnalyzeXHTML("Text/ch01.xhtml",
		wrapXHTML("<h1>正文一样长也仍是标题</h1><figcaption>第一章</figcaption><p>这是普通正文段落，长度足以稳定识别为正文。</p>", "", "zh-CN"), false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := join(roles(blocks)), "heading,caption,body"; got != want {
		t.Fatalf("roles = %s, want %s", got, want)
	}
	if blocks[0].Typography.FontRole != "ht" {
		t.Errorf("h1 font_role = %q", blocks[0].Typography.FontRole)
	}
	if blocks[1].Locator != "/html[1]/body[1]/figcaption[1]" {
		t.Errorf("locator = %q", blocks[1].Locator)
	}
	if blocks[2].PreviousTag == nil || *blocks[2].PreviousTag != "figcaption" {
		t.Errorf("previous_tag = %v", blocks[2].PreviousTag)
	}
}

func TestTitlePageAndSubtitleOverrideHeading(t *testing.T) {
	blocks, err := AnalyzeXHTML("Text/title.xhtml",
		wrapXHTML(`<section class="title-page" epub:type="titlepage"><h1>书名</h1><h2 class="subtitle">副标题</h2></section>`, "", "zh-CN"), false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := join(roles(blocks)), "title,subtitle"; got != want {
		t.Fatalf("roles = %s, want %s", got, want)
	}
	if blocks[0].Typography.TextAlign != "center" || blocks[0].Typography.LineHeight != "1.2" {
		t.Errorf("title typography = %+v", blocks[0].Typography)
	}
	if blocks[1].Typography.FontRole != "kt" {
		t.Errorf("subtitle font_role = %q", blocks[1].Typography.FontRole)
	}
}

func TestExplicitChineseRolesAndFontAdvice(t *testing.T) {
	body := `<p class="subtitle">副标题</p>` +
		`<blockquote>引用内容</blockquote>` +
		`<p class="epigraph">题记内容</p>` +
		`<div class="poem"><p>床前明月光</p><p>疑是地上霜</p></div>` +
		`<section class="letter"><p>亲爱的朋友：</p></section>` +
		`<aside epub:type="footnote"><p>注释正文</p></aside>` +
		`<pre><code>print(&quot;ok&quot;)</code></pre>` +
		`<p class="classical-text">学而时习之，不亦说乎。</p>` +
		`<p class="modern-text">学习并经常温习，是令人愉快的。</p>` +
		`<hr class="scene-break"/>`
	blocks, err := AnalyzeXHTML("Text/roles.xhtml", wrapXHTML(body, "", "zh-CN"), false)
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, r := range roles(blocks) {
		set[r] = true
	}
	for _, want := range []string{"subtitle", "quotation", "epigraph", "verse", "letter", "note", "code", "classical", "modern-translation", "scene-break"} {
		if !set[want] {
			t.Errorf("缺少角色 %s；got %v", want, roles(blocks))
		}
	}
	byRole := map[string]legacyBlock{}
	for _, b := range blocks {
		byRole[b.PrimaryRole] = b
	}
	if byRole["epigraph"].Typography.FontRole != "kt" {
		t.Errorf("epigraph font_role = %q", byRole["epigraph"].Typography.FontRole)
	}
	if byRole["code"].Typography.FontRole != "mono" {
		t.Errorf("code font_role = %q", byRole["code"].Typography.FontRole)
	}
	if byRole["scene-break"].Typography.TextAlign != "center" {
		t.Errorf("scene-break text_align = %q", byRole["scene-break"].Typography.TextAlign)
	}
}

func TestDialogueAndAmbiguousShortChinese(t *testing.T) {
	blocks, err := AnalyzeXHTML("Text/ambiguous.xhtml",
		wrapXHTML(`<p>“你明天还来吗？”她问。</p><p>春风又绿江南岸</p>`, "", "zh-CN"), false)
	if err != nil {
		t.Fatal(err)
	}
	if blocks[0].PrimaryRole != "dialogue" || !blocks[0].ReviewRequired {
		t.Errorf("block0 = %s review=%v", blocks[0].PrimaryRole, blocks[0].ReviewRequired)
	}
	if blocks[1].PrimaryRole != "unknown" || !blocks[1].ReviewRequired {
		t.Errorf("block1 = %s review=%v", blocks[1].PrimaryRole, blocks[1].ReviewRequired)
	}
	if !contains(blocks[1].CandidateRoles, "body") || !contains(blocks[1].CandidateRoles, "verse") {
		t.Errorf("candidates = %v", blocks[1].CandidateRoles)
	}
}

func TestFeaturesMixedChineseLatinPunctuation(t *testing.T) {
	blocks, _ := AnalyzeXHTML("Text/mixed.xhtml",
		wrapXHTML("<p>EPUB 3.3 与中文混排：Hello，世界！2026。</p>", "", "zh-CN"), false)
	f := blocks[0].Features
	if f.CJKCount != 7 || f.LatinCount != 9 || f.DigitCount != 6 || f.PunctuationCount != 5 {
		t.Errorf("features = %+v", f)
	}
	if f.VisibleChars != 27 || f.CJKRatio != 0.2593 || f.LineCount != 1 {
		t.Errorf("visible=%d cjk_ratio=%v lines=%d", f.VisibleChars, f.CJKRatio, f.LineCount)
	}
}

func TestUnpunctuatedClassicalNotForcedToBody(t *testing.T) {
	blocks, _ := AnalyzeXHTML("Text/classical.xhtml",
		wrapXHTML("<p>天地玄黄宇宙洪荒日月盈昃辰宿列张寒来暑往秋收冬藏</p>", "", "zh-CN"), false)
	if blocks[0].PrimaryRole != "unknown" || !blocks[0].ReviewRequired {
		t.Errorf("role = %s", blocks[0].PrimaryRole)
	}
	if !contains(blocks[0].CandidateRoles, "classical") {
		t.Errorf("candidates = %v", blocks[0].CandidateRoles)
	}
}

func TestDashDialogueAndBrSeparatedVerse(t *testing.T) {
	blocks, _ := AnalyzeXHTML("Text/heuristics.xhtml",
		wrapXHTML("<p>——你终于来了？</p><p>床前明月光<br/>疑是地上霜<br/>举头望明月</p>", "", "zh-CN"), false)
	if blocks[0].PrimaryRole != "dialogue" {
		t.Errorf("block0 = %s", blocks[0].PrimaryRole)
	}
	if blocks[1].PrimaryRole != "verse" || !blocks[1].ReviewRequired {
		t.Errorf("block1 = %s review=%v", blocks[1].PrimaryRole, blocks[1].ReviewRequired)
	}
	if blocks[1].Features.LineCount != 3 {
		t.Errorf("line_count = %d", blocks[1].Features.LineCount)
	}
}

func TestLooseHTMLAndLanguage(t *testing.T) {
	blocks, err := AnalyzeSource("chapter.html",
		`<html lang="zh-Hant"><body><h1>第一章<p>這是一段沒有閉合標籤的繁體中文正文內容。`, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := join(roles(blocks)), "heading,body"; got != want {
		t.Fatalf("roles = %s, want %s", got, want)
	}
	for _, b := range blocks {
		if b.Language == nil || *b.Language != "zh-Hant" {
			t.Errorf("language = %v", b.Language)
		}
	}
}

func TestMarkdownAndPlainTextInputs(t *testing.T) {
	markdown, err := AnalyzeSource("chapter.md", "# 第一章\n\n> 引用内容\n\n这是普通正文段落，长度足以稳定识别。", false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := join(roles(markdown[:2])), "heading,quotation"; got != want {
		t.Fatalf("markdown roles = %s, want %s", got, want)
	}
	plain, err := AnalyzeSource("chapter.txt", "第一段普通正文，长度足以识别。\n\n第二段普通正文，继续叙述内容。", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) != 2 {
		t.Fatalf("plain blocks = %d", len(plain))
	}
	for _, b := range plain {
		if b.Source != "chapter.txt" {
			t.Errorf("source = %q", b.Source)
		}
	}
}

func TestMarkdownListAndCodeRoles(t *testing.T) {
	blocks, err := AnalyzeSource("notes.md", "- 第一项\n\n```python\nprint('ok')\n```\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := join(roles(blocks)), "list,code"; got != want {
		t.Fatalf("roles = %s, want %s", got, want)
	}
}

func TestDefaultReportPrivateSnippetsOptIn(t *testing.T) {
	source := "这是完整私有正文，默认报告不得直接保存这一段文本。"
	private, err := AnalyzeSource("private.txt", source, false)
	if err != nil {
		t.Fatal(err)
	}
	if private[0].Snippet != "" {
		t.Error("默认报告不得包含 snippet")
	}
	if private[0].TextSHA256 == "" {
		t.Error("缺少 text_sha256")
	}
	local, err := AnalyzeSource("private.txt", source, true)
	if err != nil {
		t.Fatal(err)
	}
	if local[0].Snippet != source {
		t.Errorf("snippet = %q", local[0].Snippet)
	}
}

func TestUnsupportedSourceType(t *testing.T) {
	_, err := AnalyzeSource("data.rst", "内容", false)
	if err == nil || err.Error() != "unsupported source type: .rst" {
		t.Fatalf("err = %v", err)
	}
	_, err = AnalyzeSource("noext", "内容", false)
	if err != nil {
		t.Fatalf("无后缀应按 plain 处理: %v", err)
	}
}

// ---- EPUB spine 路径 ----

type zipEntry struct {
	name    string
	content []byte
}

// writeFixtureEpub 把 entries 打包为 EPUB（测试专用；对齐 redline 的 buildEpub）。
func writeFixtureEpub(t *testing.T, path string, entries []zipEntry) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.name == "mimetype" {
			h.Method = zip.Store
		}
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

func spineFixture(t *testing.T, chapter, extraChapter string, encrypted bool) string {
	t.Helper()
	extraManifest := ""
	extraSpine := ""
	entries := []zipEntry{
		{name: "mimetype", content: []byte("application/epub+zip")},
		{name: "META-INF/container.xml", content: []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{name: "OEBPS/package.opf", content: []byte(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:language>zh-CN</dc:language></metadata><manifest><item id="c1" href="Text/c1.xhtml" media-type="application/xhtml+xml"/>` + extraManifest + `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="c1"/>` + extraSpine + `</spine></package>`)},
		{name: "OEBPS/Text/c1.xhtml", content: []byte(chapter)},
		{name: "OEBPS/nav.xhtml", content: []byte(wrapXHTML(`<nav epub:type="toc"><ol><li><a href="Text/c1.xhtml">目录标题</a></li></ol></nav>`, "", "zh-CN"))},
	}
	if extraChapter != "" {
		entries[2].content = []byte(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:language>zh-CN</dc:language></metadata><manifest><item id="c1" href="Text/c1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="Text/c2.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>`)
		entries = append(entries, zipEntry{name: "OEBPS/Text/c2.xhtml", content: []byte(extraChapter)})
	}
	if encrypted {
		entries = append(entries, zipEntry{name: "META-INF/encryption.xml", content: []byte("<encryption/>")})
	}
	path := filepath.Join(t.TempDir(), "book.epub")
	writeFixtureEpub(t, path, entries)
	return path
}

func TestEpubUsesSpineContentAndStopsOnEncryption(t *testing.T) {
	path := spineFixture(t, wrapXHTML("<h1>第一章</h1><p>这是正文内容，长度足以识别。</p>", "", "zh-CN"), "", false)
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{LegacyReport: true})
	if err != nil {
		t.Fatal(err)
	}
	raw := string(res.Facts["legacyReport"].(json.RawMessage))
	if !strings.Contains(raw, `"blocks": 2`) {
		t.Errorf("legacyReport 缺少 blocks=2:\n%s", raw)
	}
	if strings.Contains(raw, "目录标题") {
		t.Error("nav.xhtml 不在 spine，其内容不得出现")
	}
	if res.Status != "complete" || res.Facts["blocks"] != 2 {
		t.Errorf("status=%s blocks=%v", res.Status, res.Facts["blocks"])
	}
	if len(b.ModifiedNames()) != 0 {
		t.Errorf("只读 capability 不得修改 book: %v", b.ModifiedNames())
	}

	encrypted := spineFixture(t, wrapXHTML("<p>正文</p>", "", "zh-CN"), "", true)
	eb, err := book.Open(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	defer eb.Close()
	if _, err := Run(context.Background(), eb, Params{}); err == nil || !strings.Contains(err.Error(), "encryption") {
		t.Fatalf("encryption 拒绝失败: %v", err)
	}
}

func TestEpubRecordsBadXHTMLAndContinues(t *testing.T) {
	path := spineFixture(t,
		`<html xmlns="http://www.w3.org/1999/xhtml"><body><p>没有闭合的段落</body></html>`,
		wrapXHTML("<p>这是仍然可以分析的正文段落，长度足以识别。</p>", "", "zh-CN"), false)
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{LegacyReport: true})
	if err != nil {
		t.Fatal(err)
	}
	raw := string(res.Facts["legacyReport"].(json.RawMessage))
	for _, want := range []string{`"status": "warn"`, `"blocks": 1`, `"file_errors": 1`, `"source": "OEBPS/Text/c1.xhtml"`} {
		if !strings.Contains(raw, want) {
			t.Errorf("legacyReport 缺少 %s:\n%s", want, raw)
		}
	}
	if res.Status != "complete" {
		t.Errorf("warn 应映射为 complete，got %s", res.Status)
	}
	var warnFound bool
	for _, f := range res.Findings {
		if f.Level == "warn" {
			warnFound = true
		}
	}
	if !warnFound {
		t.Error("warn 状态应产生 warn finding")
	}
}

func TestFailWhenNoSpineDocumentAnalyzable(t *testing.T) {
	path := spineFixture(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>未闭合</body></html>`, "", false)
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{LegacyReport: true})
	if err != nil {
		t.Fatal(err)
	}
	raw := string(res.Facts["legacyReport"].(json.RawMessage))
	if !strings.Contains(raw, `"status": "fail"`) {
		t.Errorf("legacyReport 缺少 fail:\n%s", raw)
	}
	if res.Status != "failed" {
		t.Errorf("fail 应映射为 failed，got %s", res.Status)
	}
}

func TestRunIsReadOnlyOnDisk(t *testing.T) {
	path := spineFixture(t, wrapXHTML("<p>这是普通正文段落，长度足以稳定识别为正文。</p>", "", "zh-CN"), "", false)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if _, err := Run(context.Background(), b, Params{}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("输入 EPUB 字节被改动")
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
