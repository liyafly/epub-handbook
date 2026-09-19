// parity_test.go 原为 Python oracle 的 P2 parity 用例（oracle 已于
// 2026-08-29 删除）。这里保留同一组 fixture 场景，改为 Go-native 断言：
// 逐块明细（facts.blockList）、汇总（facts.blocks / review_required /
// fileErrors / analysisStatus）与信封 status 的映射。
package contentanalyze

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// runAnalyzer 打开 fixture 并执行 Run。
func runAnalyzer(t *testing.T, epubPath string) report.Result {
	t.Helper()
	b, err := book.Open(epubPath)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{})
	if err != nil {
		t.Fatalf("Go Run: %v", err)
	}
	return res
}

// rolesByLocator 把 blockList 折成 locator → primary_role。
func rolesByLocator(t *testing.T, res report.Result) map[string]analyzedBlock {
	t.Helper()
	out := map[string]analyzedBlock{}
	for _, bl := range blockListOf(t, res) {
		out[bl.Locator] = bl
	}
	return out
}

func TestAnalyzerHitMixedRoles(t *testing.T) {
	// 命中场景：标题 / 正文 / 待复核短句混合 → warn（complete + warn finding）。
	path := spineFixture(t, wrapXHTML(
		"<h1>第一章 风雪夜归人</h1><p>这是普通正文段落，长度足以稳定识别为正文。</p><p>春风又绿江南岸</p>", "", "zh-CN"), "", false)
	res := runAnalyzer(t, path)
	if res.Status != report.StatusComplete || res.Facts["analysisStatus"] != "warn" {
		t.Fatalf("status = %s analysisStatus = %v, want complete/warn", res.Status, res.Facts["analysisStatus"])
	}
	blocks := blockListOf(t, res)
	if len(blocks) != 3 || res.Facts["blocks"] != 3 {
		t.Fatalf("blocks = %d / %v, want 3", len(blocks), res.Facts["blocks"])
	}
	if blocks[0].Tag != "h1" || blocks[0].PrimaryRole != "heading" {
		t.Errorf("block[0] = %s/%s, want h1/heading", blocks[0].Tag, blocks[0].PrimaryRole)
	}
	if blocks[1].PrimaryRole != "body" || blocks[1].ReviewRequired {
		t.Errorf("block[1] = %s review=%t, want body/false", blocks[1].PrimaryRole, blocks[1].ReviewRequired)
	}
	if !blocks[2].ReviewRequired || len(blocks[2].CandidateRoles) == 0 || len(blocks[2].Evidence) == 0 {
		t.Errorf("block[2] 应为待复核并带候选角色与证据: %+v", blocks[2])
	}
	for _, bl := range blocks {
		if bl.Source != "OEBPS/Text/c1.xhtml" || bl.Locator == "" || bl.TextSHA256 == "" || bl.Confidence == "" {
			t.Errorf("block 缺少 source/locator/text_sha256/confidence: %+v", bl)
		}
		if bl.Snippet != "" {
			t.Errorf("未开启 include_snippets 不得输出 snippet: %q", bl.Snippet)
		}
		if bl.Typography.FontRole == "" {
			t.Errorf("block %s 缺少 typography.font_role", bl.Locator)
		}
	}
	if res.Facts["review_required"] != 1 {
		t.Errorf("review_required = %v, want 1", res.Facts["review_required"])
	}
	roles, _ := res.Facts["roles"].(map[string]int)
	if roles["heading"] != 1 || roles["body"] < 1 {
		t.Errorf("roles = %v", roles)
	}
	var reviewFinding bool
	for _, f := range res.Findings {
		if f.ID == "content.review-required" && f.Level == "warn" {
			reviewFinding = true
		}
	}
	if !reviewFinding {
		t.Errorf("缺少 warn content.review-required finding: %+v", res.Findings)
	}
}

func TestAnalyzerCleanHitExplicitRoles(t *testing.T) {
	// 全部显式角色 → pass：覆盖 epub:type 祖先、blockquote、pre/code、hr。
	path := spineFixture(t, wrapXHTML(
		`<h1>书名</h1><p class="subtitle">副标题</p>`+
			`<aside epub:type="footnote"><p>注释正文。</p></aside>`+
			`<blockquote>引用内容一行。</blockquote>`+
			`<pre><code>print(&quot;ok&quot;)</code></pre>`+
			`<hr/>`+
			`<p>这是普通正文段落，长度足以稳定识别为正文。</p>`, "", "zh-CN"), "", false)
	res := runAnalyzer(t, path)
	if res.Status != report.StatusComplete || res.Facts["analysisStatus"] != "pass" {
		t.Fatalf("status = %s analysisStatus = %v, want complete/pass", res.Status, res.Facts["analysisStatus"])
	}
	if res.Facts["review_required"] != 0 || res.Facts["fileErrors"] != 0 || len(res.Findings) != 0 {
		t.Errorf("pass 场景不应有待复核/错误/findings: facts=%v findings=%+v", res.Facts, res.Findings)
	}
	blocks := blockListOf(t, res)
	if len(blocks) == 0 {
		t.Fatal("blockList 为空")
	}
	wantRoles := map[string]bool{"heading": false, "subtitle": false, "note": false, "quotation": false, "code": false, "scene-break": false, "body": false}
	for _, bl := range blocks {
		if bl.ReviewRequired {
			t.Errorf("显式角色块不应待复核: %+v", bl)
		}
		if _, ok := wantRoles[bl.PrimaryRole]; ok {
			wantRoles[bl.PrimaryRole] = true
		}
	}
	for role, seen := range wantRoles {
		if !seen {
			t.Errorf("blockList 缺少角色 %s（实际 roles=%v）", role, res.Facts["roles"])
		}
	}
	if len(sourceErrorsOf(t, res)) != 0 {
		t.Errorf("sourceErrors 应为空")
	}
}

func TestAnalyzerMissNoBlocksFails(t *testing.T) {
	// 未命中场景：spine 文档没有块级标签 + 一份非法 UTF-8 文档 → fail。
	entries := []zipEntry{
		{name: "mimetype", content: []byte("application/epub+zip")},
		{name: "META-INF/container.xml", content: []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{name: "OEBPS/package.opf", content: []byte(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:language>zh-CN</dc:language></metadata><manifest><item id="c1" href="Text/c1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="Text/c2.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>`)},
		{name: "OEBPS/Text/c1.xhtml", content: []byte(wrapXHTML("<div>只有普通文字没有块级标签</div>", "", "zh-CN"))},
		{name: "OEBPS/Text/c2.xhtml", content: []byte{0xff, 0xfe, '<', 'p', '>', 0x62, '<', '/', 'p', '>'}}, // 非法 UTF-8
		{name: "OEBPS/nav.xhtml", content: []byte(wrapXHTML(`<nav epub:type="toc"><ol><li><a href="Text/c1.xhtml">目录</a></li></ol></nav>`, "", "zh-CN"))},
	}
	path := filepath.Join(t.TempDir(), "miss.epub")
	writeFixtureEpub(t, path, entries)
	res := runAnalyzer(t, path)
	if res.Status != report.StatusFailed || res.Facts["analysisStatus"] != "fail" {
		t.Fatalf("status = %s analysisStatus = %v, want failed/fail", res.Status, res.Facts["analysisStatus"])
	}
	if len(blockListOf(t, res)) != 0 || res.Facts["blocks"] != 0 {
		t.Errorf("fail 场景 blockList 应为空: %v", res.Facts["blocks"])
	}
	errs := sourceErrorsOf(t, res)
	if len(errs) != 1 || errs[0].Source != "OEBPS/Text/c2.xhtml" || errs[0].Message != "text is not valid UTF-8" {
		t.Errorf("sourceErrors = %+v, want c2.xhtml 非法 UTF-8", errs)
	}
	if res.Facts["fileErrors"] != 1 {
		t.Errorf("fileErrors = %v, want 1", res.Facts["fileErrors"])
	}
	var failFinding bool
	for _, f := range res.Findings {
		if f.ID == "content.analysis-failed" && f.Level == "error" && f.Detail == "text is not valid UTF-8" {
			failFinding = true
		}
	}
	if !failFinding {
		t.Errorf("缺少 error content.analysis-failed finding: %+v", res.Findings)
	}
}

func TestAnalyzerEncryptionRefused(t *testing.T) {
	// encryption.xml 拒绝：返回固定措辞的错误，不产出 Result。
	path := spineFixture(t, wrapXHTML("<p>正文</p>", "", "zh-CN"), "", true)
	b, err := book.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	_, goErr := Run(context.Background(), b, Params{})
	if goErr == nil || goErr.Error() != "encryption marker detected; content analysis stopped" {
		t.Errorf("Go 错误 = %v", goErr)
	}
}
