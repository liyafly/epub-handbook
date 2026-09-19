// regions_test.go 锁定引用重写的区域感知边界（见 refs.go 的
// rewriteMarkupReferences 头注）：真实标记内改写照旧；字符数据、注释、
// CDATA、<script> 内容原样保留；title=""/alt="" 里的 url(...) 不被当成
// CSS；独立 .css 文件仍走全文重写；扫描截断时上报带偏移的告警且截断点
// 之后不改。与 cover 包同名测试同源（transformResource 签名一致）。
package merge

import (
	"fmt"
	"strings"
	"testing"
)

// regionsFixturePathMap 构造一个统一的改名映射：chapter.xhtml 里对
// Images/old.png 的引用应改写到 Images/new.png。
func regionsFixturePathMap() (map[string]string, map[string]bool) {
	pathMap := map[string]string{"OEBPS/Images/old.png": "OEBPS/Images/new.png"}
	knownFiles := map[string]bool{"OEBPS/Images/old.png": true}
	return pathMap, knownFiles
}

func TestTransformResourceRegionAwareRewritesRealMarkup(t *testing.T) {
	pathMap, knownFiles := regionsFixturePathMap()
	const doc = "OEBPS/Text/chapter.xhtml"
	text := `<html><head>` +
		`<style>body { background: url('../Images/old.png'); } @import "../Images/old.png";</style>` +
		`</head><body>` +
		`<img src="../Images/old.png"/>` +
		`<div style="background: url('../Images/old.png');">styled</div>` +
		`</body></html>`
	got := string(transformResource([]byte(text), doc, doc, pathMap, knownFiles, nil))

	for _, want := range []string{
		`url('../Images/new.png')`,                      // <style> 元素内容
		`@import "../Images/new.png";`,                  // <style> 元素内容
		`<img src="../Images/new.png"/>`,                // 标签属性
		`style="background: url('../Images/new.png');"`, // 内联 style 属性
	} {
		if !strings.Contains(got, want) {
			t.Errorf("输出缺少 %q\n完整输出: %s", want, got)
		}
	}
	if strings.Contains(got, "old.png") {
		t.Errorf("旧路径未被完全替换: %s", got)
	}
}

func TestTransformResourceKeepsEscapedProseVerbatim(t *testing.T) {
	pathMap, knownFiles := regionsFixturePathMap()
	const doc = "OEBPS/Text/chapter.xhtml"
	const prose = `<p>写作 &lt;img src="../Images/old.png"/&gt; 即可。</p>`
	got := string(transformResource([]byte(prose), doc, doc, pathMap, knownFiles, nil))
	if got != prose {
		t.Errorf("转义正文被改写:\n got  = %q\n want = %q", got, prose)
	}
}

func TestTransformResourceKeepsCommentsCDATAAndScriptVerbatim(t *testing.T) {
	pathMap, knownFiles := regionsFixturePathMap()
	const doc = "OEBPS/Text/chapter.xhtml"
	text := `<!-- <img src="../Images/old.png"/> -->` +
		`<![CDATA[<img src="../Images/old.png"/>]]>` +
		`<script>var s = "../Images/old.png";</script>`
	got := string(transformResource([]byte(text), doc, doc, pathMap, knownFiles, nil))
	if got != text {
		t.Errorf("注释/CDATA/<script> 内容被改写:\n got  = %q\n want = %q", got, text)
	}
}

func TestTransformResourceDoesNotTreatTitleAltAsCSS(t *testing.T) {
	pathMap, knownFiles := regionsFixturePathMap()
	const doc = "OEBPS/Text/chapter.xhtml"
	text := `<img src="../Images/old.png" title="url(../Images/old.png)" alt="url(../Images/old.png)"/>`
	got := string(transformResource([]byte(text), doc, doc, pathMap, knownFiles, nil))
	if !strings.Contains(got, `src="../Images/new.png"`) {
		t.Errorf("src 应被改写: %s", got)
	}
	if !strings.Contains(got, `title="url(../Images/old.png)"`) || !strings.Contains(got, `alt="url(../Images/old.png)"`) {
		t.Errorf("title/alt 里的 url(...) 不应被当成 CSS 改写: %s", got)
	}
}

func TestTransformResourceStandaloneCSSStillRewritesWholeFile(t *testing.T) {
	pathMap, knownFiles := regionsFixturePathMap()
	const doc = "OEBPS/Styles/main.css"
	text := "@import \"../Images/old.png\";\nbody { background: url(../Images/old.png); }\n"
	got := string(transformResource([]byte(text), doc, doc, pathMap, knownFiles, nil))
	if strings.Contains(got, "old.png") {
		t.Errorf("独立 .css 文件应整份替换: %s", got)
	}
	if !strings.Contains(got, `@import "../Images/new.png";`) || !strings.Contains(got, "url(../Images/new.png)") {
		t.Errorf("独立 .css 文件替换结果不对: %s", got)
	}
}

// TestTransformResourceReportsScanTruncation 覆盖截断上报：截断前的真实
// 标签正常改写，截断点（含它自身与其后本会命中 URI 正则的字节）原样
// 保留，且必须产生一条带文件名与字节偏移的告警。
func TestTransformResourceReportsScanTruncation(t *testing.T) {
	pathMap, knownFiles := regionsFixturePathMap()
	const doc = "OEBPS/Text/chapter.xhtml"
	const head = `<img src="../Images/old.png"/>`
	// 引号数为奇数（title 属性值本身未闭合），扫描器找不到标签结束的
	// '>'，从这个字节起放弃：tail 里的 src="…" 必须原样保留。
	const tail = `<p title="unterminated src="../Images/old.png">tail`
	text := head + tail
	var warnings []string
	warn := func(format string, a ...any) { warnings = append(warnings, fmt.Sprintf(format, a...)) }
	got := string(transformResource([]byte(text), doc, doc, pathMap, knownFiles, warn))

	if !strings.HasPrefix(got, `<img src="../Images/new.png"/>`) {
		t.Errorf("截断前的真实标签应正常改写: %s", got)
	}
	if !strings.HasSuffix(got, tail) {
		t.Errorf("截断点之后必须原字节保留: got=%q want suffix=%q", got, tail)
	}
	if len(warnings) != 1 {
		t.Fatalf("应产生且只产生 1 条截断告警: %v", warnings)
	}
	wantOffset := len(head)
	if !strings.Contains(warnings[0], doc) || !strings.Contains(warnings[0], fmt.Sprintf("byte offset %d", wantOffset)) {
		t.Errorf("告警应带文件名与字节偏移: %q (want offset %d)", warnings[0], wantOffset)
	}
}
