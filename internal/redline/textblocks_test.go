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
