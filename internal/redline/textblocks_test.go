package redline

import (
	"slices"
	"strings"
	"testing"
)

// TestExtractTextBlocksIncludesInlineDescendants 锁定 ExtractTextBlocks 的取文
// 范围：块的文本 = 该块全部后代的文本，只在 rt / rp / script / style 与
// note-control 锚点处剪枝（保留其 tail）。
//
// 这条断言存在的理由：流式实现曾漏掉「子帧闭合时并回父帧」，于是任何被
// <em>/<a>/<span> 包裹的正文都不进块哈希 —— <p><em>整段</em></p> 甚至完全
// 不产出块，可以被整段删除而 text 红线零 findings。正文不变是本仓最高安全
// 属性，取文范围一旦收窄，gate 就在无声中失效，因此必须逐形状钉住。
func TestExtractTextBlocksIncludesInlineDescendants(t *testing.T) {
	cases := []struct {
		name string
		frag string
		want []string
	}{
		{"块内直接字符数据", `<p>第一段落。</p>`, []string{"第一段落。"}},
		{"整段被行内元素包裹", `<p><em>强调整段。</em></p>`, []string{"强调整段。"}},
		{"行内元素切断字符数据", `<p>前<em>中</em>后</p>`, []string{"前中后"}},
		{"锚点文字属于正文", `<p><a href="x.xhtml">链接文字</a></p>`, []string{"链接文字"}},
		{"span 包裹", `<p><span class="s">跨度文字</span></p>`, []string{"跨度文字"}},
		{"nav 目录标签", `<li><a href="x.xhtml">第一章</a></li>`, []string{"第一章"}},
		{"嵌套行内", `<p>a<em>b<strong>c</strong>d</em>e</p>`, []string{"abcde"}},
		{"只产出最内层块", `<div><p>段落</p></div>`, []string{"段落"}},
		// 剪枝形状：内部文本剔除，紧随其后的 tail 仍属于块。
		{"ruby 保留注音基字剔除 rt", `<p>汉字<ruby>字<rt>zì</rt></ruby>注音。</p>`, []string{"汉字字注音。"}},
		{"rt 剪枝后保留 tail", `<p><ruby>汉<rt>han</rt></ruby>字</p>`, []string{"汉字"}},
		{"script 剪枝后保留 tail", `<p>前<script>var x = 1;</script>后</p>`, []string{"前后"}},
		{"style 剪枝后保留 tail", `<p>前<style>p{color:red}</style>后</p>`, []string{"前后"}},
		{"noteref 锚点文字不属于正文", `<p>正文<a epub:type="noteref" href="#f1">[1]</a>尾巴</p>`, []string{"正文尾巴"}},
		{"backlink 锚点文字不属于正文", `<p><a epub:type="backlink" href="#r1">◎</a>注释正文</p>`, []string{"注释正文"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body>` +
				tc.frag + `</body></html>`
			got, err := ExtractTextBlocks([]byte(doc), tc.name)
			if err != nil {
				t.Fatalf("ExtractTextBlocks: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("blocks = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractTextBlocksAcceptsXHTML11DeclaredEntities(t *testing.T) {
	doc := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml"><body><p>这是&nbsp;旧书&mdash;正文。</p></body></html>`
	got, err := ExtractTextBlocks([]byte(doc), "legacy.xhtml")
	if err != nil {
		t.Fatalf("ExtractTextBlocks: %v", err)
	}
	want := []string{"这是 旧书—正文。"}
	if !slices.Equal(got, want) {
		t.Fatalf("blocks=%q, want %q", got, want)
	}
}

// TestCheckTextSeesInlineWrappedEdits 是上一条的端到端对照：取文范围收窄时，
// 下面每一种改动都会让 text 红线静默放行。
func TestCheckTextSeesInlineWrappedEdits(t *testing.T) {
	const inlineXHTML = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-CN">
  <head><title>行内</title></head>
  <body>
    <p><em>整段都被强调包裹。</em></p>
    <p>前<a href="other.xhtml">链接文字</a>后</p>
  </body>
</html>
`
	withInline := func(content string) []zipEntry {
		out := baseEntries()
		out = append(out, zipEntry{name: "OEBPS/Text/inline.xhtml", content: []byte(content)})
		return out
	}

	cases := []struct {
		name  string
		after string
	}{
		{"改写 em 内的整段", strings.Replace(inlineXHTML, "整段都被强调包裹。", "整段都被强调包裹！", 1)},
		{"改写锚点文字", strings.Replace(inlineXHTML, "链接文字", "链接文本", 1)},
		{"整段删除（该段全部文本在 em 内）",
			strings.Replace(inlineXHTML, "    <p><em>整段都被强调包裹。</em></p>\n", "", 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before, after := pair(t, withInline(inlineXHTML), withInline(tc.after))
			rep, text := compare(t, before, after, "text", Options{})
			wantCode(t, rep, text, 1)
			wantLine(t, rep, text, "text: modified OEBPS/Text/inline.xhtml")
		})
	}
}

func entriesWithTextBody(t *testing.T, body string) []zipEntry {
	t.Helper()
	entries := baseEntries()
	for i := range entries {
		if entries[i].name == "OEBPS/Text/c1.xhtml" {
			entries[i].content = []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>测试</title></head><body>` + body + `</body></html>`)
			return entries
		}
	}
	t.Fatal("baseEntries has no chapter XHTML")
	return nil
}

func expectTextRedlineFailure(t *testing.T, beforeBody, afterBody string) {
	t.Helper()
	before, after := pair(t, entriesWithTextBody(t, beforeBody), entriesWithTextBody(t, afterBody))
	rep, text := compare(t, before, after, "text", Options{})
	wantCode(t, rep, text, 1)
	wantLine(t, rep, text, "text: modified OEBPS/Text/c1.xhtml")
}

func TestRedlineDetectsTableHeaderTextChange(t *testing.T) {
	before := `<p>正文</p><table><tr><th>表头文字</th><td>单元格</td></tr></table>`
	after := strings.Replace(before, "表头文字", "表头改写", 1)
	expectTextRedlineFailure(t, before, after)
}

func TestRedlineDetectsDefinitionListTextChange(t *testing.T) {
	before := `<p>正文</p><dl><dt>术语</dt><dd>定义文字</dd></dl>`
	after := strings.Replace(before, "定义文字", "定义改写", 1)
	expectTextRedlineFailure(t, before, after)
}

func TestRedlineDetectsFigcaptionDeletion(t *testing.T) {
	before := `<p>正文</p><figure><img src="image.png"/><figcaption>图注文字</figcaption></figure>`
	after := strings.Replace(before, `<figcaption>图注文字</figcaption>`, "", 1)
	expectTextRedlineFailure(t, before, after)
}

func TestRedlineDetectsLooseSectionText(t *testing.T) {
	before := `<p>正文</p><section>节内裸文字 <em>强调内容</em></section>`
	after := strings.Replace(before, "节内裸文字", "节内改写文字", 1)
	expectTextRedlineFailure(t, before, after)
}

func TestRedlineWrappingLooseTextInParagraphKeepsHashes(t *testing.T) {
	beforeBody := `<section>节内裸文字 <em>强调内容</em></section>`
	afterBody := `<section><p>节内裸文字 <em>强调内容</em></p></section>`
	beforeBlocks, err := ExtractTextBlocks([]byte(`<html><body>`+beforeBody+`</body></html>`), "before.xhtml")
	if err != nil {
		t.Fatalf("ExtractTextBlocks before: %v", err)
	}
	afterBlocks, err := ExtractTextBlocks([]byte(`<html><body>`+afterBody+`</body></html>`), "after.xhtml")
	if err != nil {
		t.Fatalf("ExtractTextBlocks after: %v", err)
	}
	if got, want := BlockHashes(beforeBlocks), BlockHashes(afterBlocks); !slices.Equal(got, want) {
		t.Fatalf("wrapping loose text changed block hashes: before=%q after=%q", got, want)
	}
	before, after := pair(t, entriesWithTextBody(t, beforeBody), entriesWithTextBody(t, afterBody))
	rep, text := compare(t, before, after, "text", Options{})
	wantCode(t, rep, text, 0)
}

func TestExtractTextBlocksIncludesAddedBlockTags(t *testing.T) {
	doc := `<html><body><table><caption>表题</caption><tr><th>表头</th><td>单元格</td></tr></table>` +
		`<dl><dt>术语</dt><dd>定义</dd></dl><figure><figcaption>图注</figcaption></figure>` +
		`<details><summary>摘要</summary></details><address>地址</address></body></html>`
	got, err := ExtractTextBlocks([]byte(doc), "structural.xhtml")
	if err != nil {
		t.Fatalf("ExtractTextBlocks: %v", err)
	}
	want := []string{"表题", "表头", "单元格", "术语", "定义", "图注", "摘要", "地址"}
	if !slices.Equal(got, want) {
		t.Fatalf("blocks=%q, want %q", got, want)
	}
}

func TestExtractTextBlocksCapturesLooseTextInContainers(t *testing.T) {
	for _, name := range []string{"body", "section", "article", "aside", "header", "footer", "main", "figure"} {
		t.Run(name, func(t *testing.T) {
			var doc string
			if name == "body" {
				doc = `<html><body>容器零散文字</body></html>`
			} else {
				doc = `<html><body><` + name + `>容器零散文字</` + name + `></body></html>`
			}
			got, err := ExtractTextBlocks([]byte(doc), "container.xhtml")
			if err != nil {
				t.Fatalf("ExtractTextBlocks: %v", err)
			}
			if want := []string{"容器零散文字"}; !slices.Equal(got, want) {
				t.Fatalf("blocks=%q, want %q", got, want)
			}
		})
	}
}

func TestExtractTextBlocksKeepsLooseTextInDocumentOrder(t *testing.T) {
	doc := `<html><body>body before<section>section before<div>div before<p>段落</p>div after</div>` +
		`section after<figure>figure before<figcaption>图注</figcaption>figure after</figure></section>` +
		`body after</body></html>`
	got, err := ExtractTextBlocks([]byte(doc), "loose.xhtml")
	if err != nil {
		t.Fatalf("ExtractTextBlocks: %v", err)
	}
	want := []string{
		"body before", "section before", "div before", "段落", "div after",
		"section after", "figure before", "图注", "figure after", "body after",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("blocks=%q, want %q", got, want)
	}
}
