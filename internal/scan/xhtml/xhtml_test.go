package xhtml

import "testing"

func TestFindOpenTagAndAttrs(t *testing.T) {
	doc := `<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="zh-CN" lang="zh-CN">
<body class="a b">
<img src='x.png' alt=cover />
</body>
</html>`
	html, ok := FindOpenTag(doc, "html", 0)
	if !ok {
		t.Fatal("html 未找到")
	}
	if html.Name != "html" || html.Span.Start != 0 {
		t.Errorf("html tag = %+v", html)
	}
	if a, ok := html.Attr("xml:lang"); !ok || a.Value != "zh-CN" || a.Quote != '"' {
		t.Errorf("xml:lang = %+v", a)
	}
	if a, ok := html.Attr("lang"); !ok || a.Value != "zh-CN" {
		t.Errorf("lang = %+v", a)
	}

	img, ok := FindOpenTag(doc, "img", 0)
	if !ok {
		t.Fatal("img 未找到")
	}
	if !img.SelfClose {
		t.Error("img 应识别为自闭合")
	}
	if a, ok := img.Attr("src"); !ok || a.Value != "x.png" || a.Quote != '\'' {
		t.Errorf("src = %+v", a)
	}
	if a, ok := img.Attr("alt"); !ok || a.Value != "cover" || a.Quote != 0 {
		t.Errorf("alt = %+v", a)
	}
}

func TestTagWordBoundary(t *testing.T) {
	doc := `<spanx>1</spanx><span>2</span>`
	tag, ok := FindOpenTag(doc, "span", 0)
	if !ok || tag.Span.Start != 16 {
		t.Fatalf("span 起点 = %+v, ok=%v", tag, ok)
	}
}

func TestAttrEditReplaceAndAppend(t *testing.T) {
	doc := `<html lang="en" xmlns="x"><body/></html>`
	tag, _ := FindOpenTag(doc, "html", 0)
	span, repl, ok := AttrEdit(doc, tag, "lang", "zh-CN")
	if !ok {
		t.Fatal("应产生替换")
	}
	out := doc[:span.Start] + repl + doc[span.End:]
	if out != `<html lang="zh-CN" xmlns="x"><body/></html>` {
		t.Errorf("替换结果 = %s", out)
	}
	span, repl, ok = AttrEdit(doc, tag, "epub:type", "toc")
	if !ok {
		t.Fatal("应产生追加")
	}
	out = doc[:span.Start] + repl + doc[span.End:]
	want := `<html lang="en" xmlns="x" epub:type="toc"><body/></html>`
	if out != want {
		t.Errorf("追加结果 = %s, want %s", out, want)
	}
	// 自闭合标签追加在 / 之前。
	btag, _ := FindOpenTag(doc, "body", 0)
	span, repl, _ = AttrEdit(doc, btag, "class", "c")
	out = doc[:span.Start] + repl + doc[span.End:]
	if out != `<html lang="en" xmlns="x"><body class="c"/></html>` {
		t.Errorf("自闭合追加 = %s", out)
	}
	// 已是目标值 → 不动。
	if _, _, ok := AttrEdit(doc, tag, "lang", "en"); ok {
		t.Error("值未变不应产生编辑")
	}
}

func TestFindCloseTag(t *testing.T) {
	doc := "<div>\n  <p>x</p>\n</DIV>"
	span, ok := FindCloseTag(doc, "div", 0)
	if !ok || span.Start != 17 || doc[span.Start:span.End] != "</DIV>" {
		t.Errorf("close span = %+v", span)
	}
}
