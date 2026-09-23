package structurenormalize

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/redline"
	"github.com/liyafly/epub-handbook/internal/report"
)

// proseRewriter 让 old/ 下的 a.png、a.css、doc.xhtml 搬到 new/，从
// old/doc.xhtml 改写到 new/doc.xhtml 时相对引用 a.png → img/a.png、
// a.css → css/a.css。
func proseRewriter(t *testing.T) (*refRewriter, *[]string) {
	t.Helper()
	warnings := []string{}
	return &refRewriter{
		pathMap: map[string]string{
			"old/a.png":     "new/img/a.png",
			"old/a.css":     "new/css/a.css",
			"old/doc.xhtml": "new/doc.xhtml",
		},
		files: map[string]bool{
			"old/a.png":     true,
			"old/a.css":     true,
			"old/doc.xhtml": true,
		},
		warnings: &warnings,
	}, &warnings
}

// rewriteCases 跑一组「输入 → 期望输出」并断言零告警。
func rewriteCases(t *testing.T, cases []struct{ name, in, want string }) {
	t.Helper()
	rw, warnings := proseRewriter(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rewriteMarkupReferences(tc.in, "old/doc.xhtml", "new/doc.xhtml", rw)
			if got != tc.want {
				t.Fatalf("重写结果错误:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
	if len(*warnings) != 0 {
		t.Fatalf("不应有告警: %v", *warnings)
	}
}

func TestMarkupRewriteLeavesEscapedProseUntouched(t *testing.T) {
	rw, warnings := proseRewriter(t)
	cases := []struct{ name, in, want string }{
		{
			// (a) 实体转义的「标签」是正文；同一文档里真实 <img> 必须改写。
			"escaped prose vs real tag",
			`<p>替换：&lt;img src="a.png"/&gt;</p><img src="a.png"/>`,
			`<p>替换：&lt;img src="a.png"/&gt;</p><img src="img/a.png"/>`,
		},
		{
			// (a') 真书 Chapter11-2 第 139 行形态：转义的 SMIL 片段。
			"escaped smil prose",
			`<p>替换：&lt;par id="\1"&gt;&lt;text src="doc.xhtml#\2"/&gt;&lt;/par&gt;\n</p>`,
			`<p>替换：&lt;par id="\1"&gt;&lt;text src="doc.xhtml#\2"/&gt;&lt;/par&gt;\n</p>`,
		},
		{
			// (b) 属性名、等号与带引号的值被拆进相邻 <span>：全部是字符数据。
			"attribute split across spans",
			`<li>&#160; &lt;<span class="tag-color">text</span>&#160;<span class="selector-color">src</span>=<span class="class-color">"a.png"</span>/&gt;</li>`,
			`<li>&#160; &lt;<span class="tag-color">text</span>&#160;<span class="selector-color">src</span>=<span class="class-color">"a.png"</span>/&gt;</li>`,
		},
		{
			// 真书 Chapter12-2 形态：正文里引号后带空格的 src=" ../x"。
			"escaped prose with leading space in value",
			`<p>替换：&lt;img alt="" src=" a.png"/&gt;</p>`,
			`<p>替换：&lt;img alt="" src=" a.png"/&gt;</p>`,
		},
		{
			// (c) 注释、CDATA、PI、DOCTYPE 与 <script> 内容不参与改写。
			"comment cdata pi script",
			`<?xml version="1.0"?><!DOCTYPE html SYSTEM "a.png"><!-- <img src="a.png"> --><script>var s = '<img src="a.png">';</script><![CDATA[<img src="a.png">]]><img src="a.png"/>`,
			`<?xml version="1.0"?><!DOCTYPE html SYSTEM "a.png"><!-- <img src="a.png"> --><script>var s = '<img src="a.png">';</script><![CDATA[<img src="a.png">]]><img src="img/a.png"/>`,
		},
		{
			// (d) <style> 内容里的 url() 改写；<p> 正文里的 url(...) 不改。
			"style url vs prose url",
			`<style>body{background:url(a.png)}</style><p>写 url(a.png) 即可</p><p>@import "a.png"</p>`,
			`<style>body{background:url(img/a.png)}</style><p>写 url(a.png) 即可</p><p>@import "a.png"</p>`,
		},
		{
			// 内联 style 属性属于真实标签，url() 仍然改写（保持既有行为）。
			"inline style attribute",
			`<div style="background:url('a.png')">url(a.png)</div>`,
			`<div style="background:url('img/a.png')">url(a.png)</div>`,
		},
		{
			// 属性值里的 '>' 不结束标签；标签跨行；大小写不敏感。
			"quoted gt and multiline tag",
			"<a title=\"x > y\"\n  HREF='doc.xhtml#p'>x</a>",
			"<a title=\"x > y\"\n  HREF='doc.xhtml#p'>x</a>",
		},
		{
			// 正文里的裸 '<'（后接空白/数字）按字符处理，不吞掉后续真实标签。
			"stray lt in prose",
			`<p>1 < 2 和 a<3</p><img src="a.png"/>`,
			`<p>1 < 2 和 a<3</p><img src="img/a.png"/>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rewriteMarkupReferences(tc.in, "old/doc.xhtml", "new/doc.xhtml", rw)
			if got != tc.want {
				t.Fatalf("重写结果错误:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
	if len(*warnings) != 0 {
		t.Fatalf("不应有告警: %v", *warnings)
	}
}

func TestRewriteMarkupIgnoresAttributeLookalikesInsideValues(t *testing.T) {
	warnings := []string{}
	rw := &refRewriter{
		pathMap:  map[string]string{"old/Images/old.png": "new/Images/new.png"},
		files:    map[string]bool{"old/Images/old.png": true},
		warnings: &warnings,
	}
	const input = `<img alt='src="../Images/old.png"' title='style="background:url(../Images/old.png)"' data-src="../Images/old.png" src="../Images/old.png"/>`
	const want = `<img alt='src="../Images/old.png"' title='style="background:url(../Images/old.png)"' data-src="../Images/old.png" src="../Images/new.png"/>`
	got := rewriteMarkupReferences(input, "old/Text/chapter.xhtml", "new/Text/chapter.xhtml", rw)
	if got != want {
		t.Fatalf("attribute lookalike rewrite = %q, want %q", got, want)
	}
	if len(warnings) != 0 || rw.err != nil {
		t.Fatalf("rewrite should be clean: warnings=%v err=%v", warnings, rw.err)
	}
}

func TestRewriteURIKeepsSpellingWhenTargetUnchanged(t *testing.T) {
	rw, warnings := proseRewriter(t)
	rw.files["old/Images/插图.jpg"] = true
	rw.files["old/Text/ch2.xhtml"] = true
	rw.files["old/Fonts/My Font.ttf"] = true
	rw.files["old/Images/old.png"] = true
	rw.pathMap["old/Images/old.png"] = "new/Images/new.png"
	const oldDoc = "old/Text/doc.xhtml"
	for _, uri := range []string{"../Images/插图.jpg", "./ch2.xhtml#n1", "../Fonts/My Font.ttf"} {
		if got := rw.rewriteURI(uri, oldDoc, oldDoc); got != uri {
			t.Errorf("rewriteURI(%q) = %q, want original spelling", uri, got)
		}
	}
	if got := rw.rewriteURI("../Images/old.png", oldDoc, "new/Text/doc.xhtml"); got != "../Images/new.png" {
		t.Errorf("moved target rewrite = %q, want ../Images/new.png", got)
	}
	if len(*warnings) != 0 {
		t.Errorf("unexpected warnings: %v", *warnings)
	}
}

func TestRewriteDecodesEntityEscapedAttributeValues(t *testing.T) {
	warnings := []string{}
	rw := &refRewriter{
		pathMap: map[string]string{
			"old/Text/a&b.xhtml":      "old/Text/ab.xhtml",
			"old/Text/a.xhtml":        "old/Text/b.xhtml",
			"old/Images/old.png":      "old/Images/new.png",
			"old/Images/old&name.png": "old/Images/new&name.png",
		},
		files: map[string]bool{
			"old/Text/a&b.xhtml":      true,
			"old/Text/a.xhtml":        true,
			"old/Images/old.png":      true,
			"old/Images/old&name.png": true,
		},
		warnings: &warnings,
	}
	input := `<a href="a&amp;b.xhtml">link</a><a href="a.xhtml?x=1&amp;y=2">query</a>` +
		`<div style="background-image:url(&quot;../Images/old.png&quot;)"></div>` +
		`<div style="background:url(../Images/old.png?x=1&amp;y=2)"></div>` +
		`<div style="background:url(../Images/old&amp;name.png)"></div>` +
		`<style>.cover{background:url(&quot;../Images/old.png&quot;);mask:url(../Images/old&amp;name.png);background:url(../Images/old.png?x=1&amp;y=2)}</style>`
	want := `<a href="ab.xhtml">link</a><a href="b.xhtml?x=1&amp;y=2">query</a>` +
		`<div style="background-image:url(&quot;../Images/new.png&quot;)"></div>` +
		`<div style="background:url(../Images/new.png?x=1&amp;y=2)"></div>` +
		`<div style="background:url(../Images/new%26name.png)"></div>` +
		`<style>.cover{background:url(&quot;../Images/new.png&quot;);mask:url(../Images/new%26name.png);background:url(../Images/new.png?x=1&amp;y=2)}</style>`
	got := rewriteMarkupReferences(input, "old/Text/chapter.xhtml", "old/Text/chapter.xhtml", rw)
	if rw.err != nil {
		t.Fatal(rw.err)
	}
	if got != want {
		t.Fatalf("entity-escaped references = %q, want %q", got, want)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	bad := &refRewriter{warnings: &[]string{}}
	_ = rewriteMarkupReferences(`<div style="x:&bogus;"></div>`, "old/Text/chapter.xhtml", "old/Text/chapter.xhtml", bad)
	if bad.err == nil {
		t.Fatal("unsupported entity in style attribute should set the rewrite error")
	}
	bad = &refRewriter{warnings: &[]string{}}
	_ = rewriteMarkupReferences(`<a href="a&bogus;b.xhtml"></a>`, "old/Text/chapter.xhtml", "old/Text/chapter.xhtml", bad)
	if bad.err == nil {
		t.Fatal("unsupported entity in URI attribute should set the rewrite error")
	}
}

// TestMarkupRewriteXMLStylesheetPI 覆盖 <?xml-stylesheet …?>：它是 SVG 与
// XHTML 里合法的样式表引用，改名后必须跟着改（区域化扫描初版整段跳过全部
// PI，导致引用静默断链，且没有任何下游红线能发现）。其余 PI 仍原样保留。
func TestMarkupRewriteXMLStylesheetPI(t *testing.T) {
	rewriteCases(t, []struct{ name, in, want string }{
		{
			// 典型形态：XML 声明 + xml-stylesheet PI + SVG 根。
			"xml declaration then stylesheet pi",
			`<?xml version="1.0"?><?xml-stylesheet type="text/css" href="a.css"?><svg/>`,
			`<?xml version="1.0"?><?xml-stylesheet type="text/css" href="css/a.css"?><svg/>`,
		},
		{
			// 单引号、伪属性顺序不同、`?>` 前有空白。
			"single quoted href",
			`<?xml-stylesheet href='a.css' type="text/css" ?>`,
			`<?xml-stylesheet href='css/a.css' type="text/css" ?>`,
		},
		{
			// 目标名大小写不敏感。
			"uppercase target",
			`<?XML-STYLESHEET HREF="a.css"?>`,
			`<?XML-STYLESHEET HREF="css/a.css"?>`,
		},
		{
			// 含 href 但不是 xml-stylesheet 的 PI：原样保留。
			"other pi with href",
			`<?other href="a.css"?><?php echo 'href="a.css"'; ?>`,
			`<?other href="a.css"?><?php echo 'href="a.css"'; ?>`,
		},
		{
			// 目标名只是以 xml-stylesheet 开头：不是同一个 PI。
			"target with suffix",
			`<?xml-stylesheet-alt href="a.css"?>`,
			`<?xml-stylesheet-alt href="a.css"?>`,
		},
		{
			// PI 是 PI，不是标签：src / srcset / CSS url() 都不参与改写。
			"pi is not a tag",
			`<?xml-stylesheet href="a.css" title="url(a.png)" src="a.png"?>`,
			`<?xml-stylesheet href="css/a.css" title="url(a.png)" src="a.png"?>`,
		},
		{
			// 注释里的 PI 属于注释，正文里被转义的 PI 属于字符数据。
			"pi inside comment and prose",
			`<!-- <?xml-stylesheet href="a.css"?> --><p>写 &lt;?xml-stylesheet href="a.css"?&gt; 即可</p><?xml-stylesheet href="a.css"?>`,
			`<!-- <?xml-stylesheet href="a.css"?> --><p>写 &lt;?xml-stylesheet href="a.css"?&gt; 即可</p><?xml-stylesheet href="css/a.css"?>`,
		},
	})
}

// TestMarkupRewriteDeclarations 覆盖 `<!` 声明分支：DOCTYPE 内部子集与
// HTML 空注释都不得让扫描器放弃文档剩余部分。撇号是最典型的踩雷点：
// `<!-- don't -->` 里的 `'` 曾被当成属性引号，一路吞到 EOF。
func TestMarkupRewriteDeclarations(t *testing.T) {
	rewriteCases(t, []struct{ name, in, want string }{
		{
			// 内部子集里的注释含撇号。
			"doctype internal subset with apostrophe comment",
			`<!DOCTYPE html [ <!-- don't --> ]><img src="a.png"/>`,
			`<!DOCTYPE html [ <!-- don't --> ]><img src="img/a.png"/>`,
		},
		{
			// 内部子集里的实体声明含 '>'：不结束声明。
			"doctype internal subset with entity",
			`<!DOCTYPE html [<!ENTITY nbsp "&#160;"><!ENTITY gt2 "a > b">]><img src="a.png"/>`,
			`<!DOCTYPE html [<!ENTITY nbsp "&#160;"><!ENTITY gt2 "a > b">]><img src="img/a.png"/>`,
		},
		{
			// 引号里的 ']' 不结束内部子集。
			"bracket inside quoted entity value",
			`<!DOCTYPE html [<!ENTITY x "]>">]><img src="a.png"/>`,
			`<!DOCTYPE html [<!ENTITY x "]>">]><img src="img/a.png"/>`,
		},
		{
			// 常规外部子集声明（无内部子集）行为不变。
			"doctype public identifiers",
			`<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd"><img src="a.png"/>`,
			`<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd"><img src="img/a.png"/>`,
		},
		{
			// HTML 空注释 <!--> / <!--->：整段是注释，其后仍要扫描。
			"html empty comments",
			`<!--><!---><!----><img src="a.png"/>`,
			`<!--><!---><!----><img src="img/a.png"/>`,
		},
	})
}

// TestMarkupRewriteTruncationWarns 断言扫描器放弃文档剩余部分时会给出
// 带文件名与字节偏移的告警（进入 facts.warnings）。没有它，改名后书里
// 会留下断链而信封一片安静。
func TestMarkupRewriteTruncationWarns(t *testing.T) {
	cases := []struct {
		name, in, want string
		offset         int
	}{
		{
			// 标签里引号不配对：整份文档都不可改写。
			"unmatched quote in tag",
			`<p class="a><img src="a.png"/>`,
			`<p class="a><img src="a.png"/>`,
			0,
		},
		{
			// 未闭合注释：注释之前的标签仍然改写。
			"unterminated comment",
			`<img src="a.png"/><!-- open <img src="a.png">`,
			`<img src="img/a.png"/><!-- open <img src="a.png">`,
			18,
		},
		{
			"unterminated cdata",
			`<img src="a.png"/><![CDATA[ open`,
			`<img src="img/a.png"/><![CDATA[ open`,
			18,
		},
		{
			"unterminated pi",
			`<img src="a.png"/><?xml-stylesheet href="a.css"`,
			`<img src="img/a.png"/><?xml-stylesheet href="a.css"`,
			18,
		},
		{
			"unterminated doctype internal subset",
			`<!DOCTYPE html [<img src="a.png"/>`,
			`<!DOCTYPE html [<img src="a.png"/>`,
			0,
		},
		{
			// 未闭合 <style>：开始标签已产出区域，其后内容不可改写。
			"unclosed style element",
			`<style>body{background:url(a.png)}`,
			`<style>body{background:url(a.png)}`,
			7,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rw, warnings := proseRewriter(t)
			got := rewriteMarkupReferences(tc.in, "old/doc.xhtml", "new/doc.xhtml", rw)
			if got != tc.want {
				t.Fatalf("重写结果错误:\n got %q\nwant %q", got, tc.want)
			}
			if len(*warnings) != 1 {
				t.Fatalf("应恰好有一条截断告警，实际 %v", *warnings)
			}
			w := (*warnings)[0]
			if !strings.Contains(w, "old/doc.xhtml") {
				t.Errorf("告警应指名文件: %q", w)
			}
			if !strings.Contains(w, fmt.Sprintf("byte offset %d", tc.offset)) {
				t.Errorf("告警应给出截断偏移 %d: %q", tc.offset, w)
			}
			if !strings.Contains(w, "left unchanged") {
				t.Errorf("告警应说明其后引用未改写: %q", w)
			}
		})
	}
}

// TestMarkupRewriteCSSOnlyInStyleAttr 断言 url()/@import 只在 style="…"
// 属性值里改写。title / alt 是读者可见文本，整段标签跑 CSS 重写会把它们
// 一起改掉，那是正文损坏。
func TestMarkupRewriteCSSOnlyInStyleAttr(t *testing.T) {
	rewriteCases(t, []struct{ name, in, want string }{
		{
			"title and alt untouched",
			`<div title="url(a.png)"><img alt="url(a.png)" src="a.png"/></div>`,
			`<div title="url(a.png)"><img alt="url(a.png)" src="img/a.png"/></div>`,
		},
		{
			"import in title untouched",
			`<div title='@import "a.css"'>x</div>`,
			`<div title='@import "a.css"'>x</div>`,
		},
		{
			// 资源搬家后内联背景图必须跟着改。
			"inline style still rewritten",
			`<div style="background:url(a.png)">x</div>`,
			`<div style="background:url(img/a.png)">x</div>`,
		},
		{
			"inline style with quoted url and other attrs",
			`<div class="c" style='background:url("a.png")' title="url(a.png)">x</div>`,
			`<div class="c" style='background:url("img/a.png")' title="url(a.png)">x</div>`,
		},
		{
			// 大小写不敏感的属性名与 `=` 两侧空白保持 Python 语义。
			"uppercase style attr with spaces",
			`<div STYLE = "background:url(a.png)">x</div>`,
			`<div STYLE = "background:url(img/a.png)">x</div>`,
		},
	})
}

// TestRealBookNormalizeKeepsProse 用仓库样本书跑完整 normalize（两阶段，
// 内存中应用），再用 redline text 红线比对原始态与当前态：必须零发现。
// 样本书的 Chapter11-2 / Chapter12-2 / Chapter8-6 正文含转义的
// `&lt;text src="…"/&gt;` 代码示例，曾被引用重写器当作属性改写。
func TestRealBookNormalizeKeepsProse(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(repo, "references", "epubs", "*.epub"))
	if len(matches) == 0 {
		// 样本书随仓库入 git（internal/pipeline/chain_semantics_test.go 同样
		// 硬失败）。这是真实缺陷的唯一回归，缺书必须报错而不是静默跳过。
		t.Fatal("references/epubs/ 下没有样本书，无法跑正文不变回归")
	}
	source := matches[0]

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{Mode: ModeNormalize})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	facts := factsOf(t, res)
	if facts.RewrittenFiles == 0 {
		t.Fatal("样本书应有被重写的文件，否则本回归无效")
	}

	findings, err := redline.Check(redline.OriginalState(b), redline.CurrentState(b),
		// 不给 allow-list：真书上零发现，加了反而会掩盖 nav / NCX 的正文损坏。
		[]string{redline.CheckText}, redline.Options{PathMap: res.Renames})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		t.Errorf("redline %s: %s", f.Check, f.Message)
	}

	// 逐字核对被误改过的三行正文仍在。
	const chapter = "OEBPS/Text/Chapter11-2.xhtml"
	target := chapter
	if mapped, ok := res.Renames[chapter]; ok {
		target = mapped
	}
	cur, err := b.Current(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, prose := range []string{
		`<p>替换：&lt;par id="\1"&gt;&lt;text src="../Text/Chapter2-2.xhtml#\2"/&gt;`,
		`<span class="selector-color">src</span>=<span class="class-color">"../Text/Chapter2-2.xhtml#xj01"</span>/&gt;</li>`,
	} {
		if !strings.Contains(string(cur), prose) {
			t.Errorf("%s 正文被改写，缺少 %q", target, prose)
		}
	}
}
