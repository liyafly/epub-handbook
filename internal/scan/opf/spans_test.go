package opf

import (
	"strings"
	"testing"
)

// spanText 按区间切回原文，用于断言 INV-2 的无损字节区间契约。
func spanText(data []byte, s Span) string { return string(data[s.Start:s.End]) }

const spanXML = `<?xml version="1.0" encoding="UTF-8"?>
<!-- leading comment -->
<package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier='bookid'>
  <metadata>
    <dc:title id="t1">A &amp; B<!-- inline -->C</dc:title>
    <dc:creator>Someone</dc:creator>
    <meta property="dcterms:modified">2026-01-01T00:00:00Z</meta>
    <meta name="cover" content="cover-img" />
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
  </manifest>
</package>
trailing`

func mustScan(t *testing.T, src string) (*SpanNode, []byte) {
	t.Helper()
	data := []byte(src)
	root, err := ScanSpanTree(data)
	if err != nil {
		t.Fatalf("ScanSpanTree error = %v", err)
	}
	return root, data
}

func TestScanSpanTreeStructureAndWalk(t *testing.T) {
	root, data := mustScan(t, spanXML)
	if root.Name.Local != "package" || root.Name.Space != OPFURI {
		t.Fatalf("root = %+v", root.Name)
	}
	if root.Parent != nil {
		t.Error("root.Parent must be nil")
	}
	var locals []string
	for _, n := range root.Walk() {
		locals = append(locals, n.Name.Local)
	}
	want := "package metadata title creator meta meta manifest item"
	if got := strings.Join(locals, " "); got != want {
		t.Fatalf("Walk order = %q, want %q", got, want)
	}
	// 开标签 / 结束标签区间必须恰好切回原文。
	if got := spanText(data, root.Open); !strings.HasPrefix(got, "<package ") || !strings.HasSuffix(got, "unique-identifier='bookid'>") {
		t.Errorf("root.Open = %q", got)
	}
	if got := spanText(data, root.Close); got != "</package>" {
		t.Errorf("root.Close = %q", got)
	}
	if root.End() != root.Close.End {
		t.Errorf("End() = %d, want Close.End %d", root.End(), root.Close.End)
	}
	if root.SelfClose {
		t.Error("root.SelfClose = true")
	}
	// xmlns 声明不进入 Attrs。
	if len(root.Attrs) != 2 {
		t.Fatalf("root.Attrs = %+v, want 2 (xmlns skipped)", root.Attrs)
	}
	meta := root.ChildByLocal(OPFURI, "metadata")
	if meta == nil || meta.Parent != root {
		t.Fatal("metadata child missing or wrong parent")
	}
	if got := spanText(data, meta.Open); got != "<metadata>" {
		t.Errorf("metadata.Open = %q", got)
	}
}

func TestScanSpanTreeChildLookups(t *testing.T) {
	root, _ := mustScan(t, spanXML)
	meta := root.ChildByLocal(OPFURI, "metadata")
	if meta == nil {
		t.Fatal("metadata missing")
	}
	if root.ChildByLocal("", "metadata") != nil {
		t.Error("ChildByLocal must require exact namespace")
	}
	if meta.ChildByLocal(DCURI, "title") == nil {
		t.Error("ChildByLocal(DCURI, title) = nil")
	}
	if meta.ChildByLocal(OPFURI, "title") != nil {
		t.Error("ChildByLocal(OPFURI, title) should be nil (dc namespace)")
	}
	if meta.ChildByAnyNS("title") == nil || meta.ChildByAnyNS("nope") != nil {
		t.Error("ChildByAnyNS mismatch")
	}
	metas := meta.ChildrenByLocal(OPFURI, "meta")
	if len(metas) != 2 {
		t.Fatalf("ChildrenByLocal(meta) = %d, want 2", len(metas))
	}
	if v, ok := metas[0].AttrByLocal("", "property"); !ok || v != "dcterms:modified" {
		t.Errorf("metas[0].property = %q, %v", v, ok)
	}
	if v, ok := metas[1].AttrByLocal("", "name"); !ok || v != "cover" {
		t.Errorf("metas[1].name = %q, %v", v, ok)
	}
	if len(meta.ChildrenByLocal(OPFURI, "absent")) != 0 {
		t.Error("ChildrenByLocal(absent) should be empty")
	}
	if !metas[1].SelfClose || !metas[1].Close.IsZero() || metas[1].End() != metas[1].Open.End {
		t.Errorf("self-closing meta = Open %+v Close %+v SelfClose %v", metas[1].Open, metas[1].Close, metas[1].SelfClose)
	}
	if metas[0].SelfClose {
		t.Error("meta with text must not be SelfClose")
	}
}

func TestScanSpanTreeAttrs(t *testing.T) {
	root, data := mustScan(t, spanXML)
	if i := root.AttrIndex("", "version"); i != 0 {
		t.Errorf("AttrIndex(version) = %d, want 0", i)
	}
	if i := root.AttrIndex("", "unique-identifier"); i != 1 {
		t.Errorf("AttrIndex(unique-identifier) = %d, want 1", i)
	}
	if i := root.AttrIndex("", "xmlns"); i != -1 {
		t.Errorf("AttrIndex(xmlns) = %d, want -1", i)
	}
	if i := root.AttrIndex(OPFURI, "version"); i != -1 {
		t.Errorf("AttrIndex with wrong namespace = %d, want -1", i)
	}
	if v, ok := root.AttrByLocal("", "version"); !ok || v != "3.0" {
		t.Errorf("AttrByLocal(version) = %q, %v", v, ok)
	}
	if _, ok := root.AttrByLocal("", "missing"); ok {
		t.Error("AttrByLocal(missing) found")
	}

	raws := RawAttrsIn(data, root.Open)
	if len(raws) != len(root.Attrs) {
		t.Fatalf("RawAttrsIn = %d entries, want %d (aligned with Attrs)", len(raws), len(root.Attrs))
	}
	if raws[0].RawName != "version" || raws[0].Quote != '"' || spanText(data, raws[0].ValueSpan) != "3.0" {
		t.Errorf("raws[0] = %+v -> %q", raws[0], spanText(data, raws[0].ValueSpan))
	}
	if raws[1].RawName != "unique-identifier" || raws[1].Quote != '\'' || spanText(data, raws[1].ValueSpan) != "bookid" {
		t.Errorf("raws[1] = %+v -> %q", raws[1], spanText(data, raws[1].ValueSpan))
	}

	s, q, ok := RawAttrValueSpan(data, root, 1)
	if !ok || q != '\'' || spanText(data, s) != "bookid" {
		t.Errorf("RawAttrValueSpan(1) = %+v %q %v", s, q, ok)
	}
	// 引号字节紧贴区间两侧。
	if data[s.Start-1] != q || data[s.End] != q {
		t.Errorf("quote bytes around span = %q %q, want %q", data[s.Start-1], data[s.End], q)
	}
	if _, _, ok := RawAttrValueSpan(data, root, -1); ok {
		t.Error("RawAttrValueSpan(-1) ok")
	}
	if _, _, ok := RawAttrValueSpan(data, root, len(root.Attrs)); ok {
		t.Error("RawAttrValueSpan(out of range) ok")
	}

	// 属性值含实体时：解码值 vs 原文区间。
	root2, data2 := mustScan(t, `<a t="x &amp; y" u = 'q'/>`)
	if v, _ := root2.AttrByLocal("", "t"); v != "x & y" {
		t.Errorf("decoded t = %q", v)
	}
	s2, q2, ok := RawAttrValueSpan(data2, root2, 0)
	if !ok || q2 != '"' || spanText(data2, s2) != "x &amp; y" {
		t.Errorf("raw t span = %q", spanText(data2, s2))
	}
	s3, q3, ok := RawAttrValueSpan(data2, root2, 1)
	if !ok || q3 != '\'' || spanText(data2, s3) != "q" {
		t.Errorf("raw u span (spaces around =) = %q", spanText(data2, s3))
	}
	if !root2.SelfClose {
		t.Error("root2 should be SelfClose")
	}
}

func TestRawAttrsInEdgeCases(t *testing.T) {
	data := []byte(`<x/>`)
	if got := RawAttrsIn(data, Span{Start: 0, End: len(data)}); got != nil {
		t.Errorf("RawAttrsIn(<x/>) = %+v, want nil", got)
	}
	if got := RawAttrsIn(data, Span{Start: 0, End: 2}); got != nil {
		t.Errorf("RawAttrsIn(too short) = %+v, want nil", got)
	}
	if got := RawAttrsIn(data, Span{Start: 0, End: 99}); got != nil {
		t.Errorf("RawAttrsIn(out of bounds) = %+v, want nil", got)
	}
	// 命名空间属性保留原始前缀名。
	root, d := mustScan(t, `<a xmlns:xml="http://www.w3.org/XML/1998/namespace" xml:lang="zh" id="1"></a>`)
	raws := RawAttrsIn(d, root.Open)
	if len(raws) != 2 || raws[0].RawName != "xml:lang" || spanText(d, raws[0].ValueSpan) != "zh" {
		t.Fatalf("raws = %+v", raws)
	}
	if v, ok := root.AttrByLocal(XMLURI, "lang"); !ok || v != "zh" {
		t.Errorf("AttrByLocal(xml:lang) = %q, %v", v, ok)
	}
}

func TestScanSpanTreeTextTailAndIterText(t *testing.T) {
	root, data := mustScan(t, spanXML)
	meta := root.ChildByLocal(OPFURI, "metadata")
	title := meta.ChildByLocal(DCURI, "title")
	if title == nil {
		t.Fatal("title missing")
	}
	// 实体解码 + 注释不切断文本；TextSpan 覆盖原文（含实体与注释）。
	if title.Text != "A & BC" {
		t.Errorf("title.Text = %q", title.Text)
	}
	if got := spanText(data, title.TextSpan); got != "A &amp; B<!-- inline -->C" {
		t.Errorf("title.TextSpan = %q", got)
	}
	if got := spanText(data, title.Close); got != "</dc:title>" {
		t.Errorf("title.Close = %q", got)
	}
	// Tail：结束标签后到下一个 '<'。
	if title.Tail != "\n    " {
		t.Errorf("title.Tail = %q", title.Tail)
	}
	if got := spanText(data, title.TailSpan); got != "\n    " {
		t.Errorf("title.TailSpan = %q", got)
	}
	ta := title.TailAfter(data)
	if ta != title.TailSpan {
		t.Errorf("TailAfter = %+v, TailSpan = %+v", ta, title.TailSpan)
	}
	if ta.Start != title.End() || data[ta.End] != '<' {
		t.Errorf("TailAfter bounds = %+v (End() %d)", ta, title.End())
	}
	// 根元素的 tail：到 EOF。
	rt := root.TailAfter(data)
	if spanText(data, rt) != "\ntrailing" || rt.End != len(data) {
		t.Errorf("root.TailAfter = %q", spanText(data, rt))
	}
	// metadata 的首段文本是开标签后的缩进。
	if meta.Text != "\n    " || spanText(data, meta.TextSpan) != "\n    " {
		t.Errorf("metadata.Text = %q / %q", meta.Text, spanText(data, meta.TextSpan))
	}
	// IterText 对齐 ET itertext()：自身 Text + 后代 Text 与后代 Tail，
	// 不含被调用元素自身的 Tail。
	want := "\n    A & BC\n    Someone\n    2026-01-01T00:00:00Z\n    \n  "
	if got := meta.IterText(); got != want {
		t.Errorf("metadata.IterText = %q, want %q", got, want)
	}
	if got := title.IterText(); got != "A & BC" {
		t.Errorf("title.IterText = %q, want %q", got, "A & BC")
	}
	// 无文本的自闭合元素：Text/TextSpan 为零。
	item := root.ChildByLocal(OPFURI, "manifest").ChildByLocal(OPFURI, "item")
	if item == nil || item.Text != "" || !item.TextSpan.IsZero() {
		t.Errorf("item text = %+v", item)
	}
	if !(Span{}).IsZero() || (Span{Start: 2, End: 5}).Len() != 3 {
		t.Error("Span helpers mismatch")
	}
}

func TestScanSpanTreeErrors(t *testing.T) {
	tests := map[string]string{
		"empty":            ``,
		"prolog only":      `<?xml version="1.0"?>`,
		"unclosed":         `<a><b></a>`,
		"multiple roots":   `<a/><b/>`,
		"utf16 bom":        "\xFF\xFE<a/>",
		"unknown encoding": `<?xml version="1.0" encoding="x-bogus-enc"?><a/>`,
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			if root, err := ScanSpanTree([]byte(in)); err == nil {
				t.Fatalf("ScanSpanTree = %+v, want error", root)
			}
		})
	}
}

func TestScanSpanTreeUTF8BOMAndDeclaredEncoding(t *testing.T) {
	// UTF-8 BOM 被剥离；区间相对于剥离后的文本。
	src := "\xEF\xBB\xBF<a>x</a>"
	root, err := ScanSpanTree([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if root.Text != "x" || root.Open != (Span{Start: 0, End: 3}) {
		t.Errorf("BOM root = %+v", root)
	}
	// 声明为 ISO-8859-1 的输入被转码后解析。
	latin := []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><a>caf\xe9</a>")
	root, err = ScanSpanTree(latin)
	if err != nil {
		t.Fatal(err)
	}
	if root.Text != "café" {
		t.Errorf("latin1 text = %q", root.Text)
	}
}
