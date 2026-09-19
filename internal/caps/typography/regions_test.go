package typography

import (
	"strconv"
	"strings"
	"testing"
)

// regions_test.go 钉住 rewriteStylesheetLinks 从「对整页文本跑正则」改成
// 「只在 xhtml.ScanRegions 认定的真实标签字节内改写」之后的行为：
//   - 真实 </head> 前正确插入新 <link>，真实旧 stylesheet <link> 行被整行删除；
//   - HTML 注释里同形的 </head> 与示例 <link ...> 完全不受影响；
//   - <script> 字符串字面量里的同形文字完全不受影响；
//   - 扫描截断时给出带偏移的 warning，且截断点之后的内容不改。

// TestRewriteStylesheetLinksRealHead 覆盖最基本路径：真实 </head> 前插入、
// 真实旧 link 行被删除。
func TestRewriteStylesheetLinksRealHead(t *testing.T) {
	text := "<html>\n" +
		"  <head>\n" +
		"    <title>T</title>\n" +
		"    <link rel=\"stylesheet\" type=\"text/css\" href=\"../Styles/old.css\"/>\n" +
		"  </head>\n" +
		"  <body><p>正文</p></body>\n" +
		"</html>\n"
	got, warnings, err := rewriteStylesheetLinks(text, "OEBPS/Text/chapter.xhtml", []string{"OEBPS/Styles/base.css"})
	if err != nil {
		t.Fatalf("rewriteStylesheetLinks: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("不应产生截断告警: %v", warnings)
	}
	if strings.Contains(got, "old.css") {
		t.Fatalf("旧 link 行应被整行删除:\n%s", got)
	}
	want := `<link rel="stylesheet" type="text/css" href="../Styles/base.css"/>`
	if !strings.Contains(got, want) {
		t.Fatalf("缺少新插入的 link %q:\n%s", want, got)
	}
	if idx := strings.Index(got, want); idx < 0 || idx > strings.Index(got, "</head>") {
		t.Fatalf("新 link 必须插在 </head> 之前:\n%s", got)
	}
}

// TestRewriteStylesheetLinksIgnoresComment 是缺陷回归用例：一段「展示旧
// 写法」的 HTML 注释里原样写出 </head> 与 <link ...>，两者都不得被当成
// 真实标记。真实 </head> 在注释之后，新链接必须插在那里，而不是注释里。
func TestRewriteStylesheetLinksIgnoresComment(t *testing.T) {
	text := "<html>\n" +
		"  <head>\n" +
		"    <title>T</title>\n" +
		"    <!--\n" +
		"      旧写法示例（不要这样写）：\n" +
		"      <link rel=\"stylesheet\" type=\"text/css\" href=\"legacy.css\"/>\n" +
		"      别忘了在 body 前写 </head>。\n" +
		"    -->\n" +
		"  </head>\n" +
		"  <body><p>正文</p></body>\n" +
		"</html>\n"
	got, warnings, err := rewriteStylesheetLinks(text, "OEBPS/Text/chapter.xhtml", []string{"OEBPS/Styles/base.css"})
	if err != nil {
		t.Fatalf("rewriteStylesheetLinks: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("不应产生截断告警: %v", warnings)
	}
	// 注释内容必须逐字节保留：示例 <link> 行与示例 </head> 都还在原处。
	commentStart := strings.Index(text, "<!--")
	commentEnd := strings.Index(text, "-->") + len("-->")
	wantComment := text[commentStart:commentEnd]
	if !strings.Contains(got, wantComment) {
		t.Fatalf("注释内容必须原样保留:\n got:\n%s\n want substring:\n%s", got, wantComment)
	}
	// 新链接必须插在注释之后、真实 </head> 之前，不是注释里面。
	newLink := `<link rel="stylesheet" type="text/css" href="../Styles/base.css"/>`
	realHeadIdx := strings.LastIndex(got, "</head>")
	newLinkIdx := strings.Index(got, newLink)
	if newLinkIdx < 0 || newLinkIdx > realHeadIdx {
		t.Fatalf("新 link 必须插在真实 </head> 之前，不是注释里:\n%s", got)
	}
	commentIdxInGot := strings.Index(got, wantComment)
	if newLinkIdx < commentIdxInGot+len(wantComment) {
		t.Fatalf("新 link 不得插在注释内部:\n%s", got)
	}
	// 注释里的示例 legacy.css link 不得被删除。
	if !strings.Contains(got, "legacy.css") {
		t.Fatalf("注释里的示例 link 不得被删除:\n%s", got)
	}
}

// TestRewriteStylesheetLinksIgnoresScript 覆盖 <script> 字符串字面量里的
// 同形文字：不得被当成真实标记改写。
func TestRewriteStylesheetLinksIgnoresScript(t *testing.T) {
	text := "<html>\n" +
		"  <head>\n" +
		"    <title>T</title>\n" +
		"    <script>\n" +
		"      var demo = \"<link rel=\\\"stylesheet\\\" href=\\\"legacy.css\\\"/>\";\n" +
		"      var closeHead = \"</head>\";\n" +
		"    </script>\n" +
		"  </head>\n" +
		"  <body><p>正文</p></body>\n" +
		"</html>\n"
	got, warnings, err := rewriteStylesheetLinks(text, "OEBPS/Text/chapter.xhtml", []string{"OEBPS/Styles/base.css"})
	if err != nil {
		t.Fatalf("rewriteStylesheetLinks: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("不应产生截断告警: %v", warnings)
	}
	scriptStart := strings.Index(text, "<script>")
	scriptEnd := strings.Index(text, "</script>") + len("</script>")
	wantScript := text[scriptStart:scriptEnd]
	if !strings.Contains(got, wantScript) {
		t.Fatalf("<script> 内容必须原样保留:\n got:\n%s\n want substring:\n%s", got, wantScript)
	}
	newLink := `<link rel="stylesheet" type="text/css" href="../Styles/base.css"/>`
	realHeadIdx := strings.LastIndex(got, "</head>")
	newLinkIdx := strings.Index(got, newLink)
	if newLinkIdx < 0 || newLinkIdx > realHeadIdx {
		t.Fatalf("新 link 必须插在真实 </head> 之前，不是脚本里:\n%s", got)
	}
}

// TestRewriteStylesheetLinksTruncationWarns 覆盖截断路径：文档尾部有一段
// 无法闭合的注释，扫描器必须放弃其后内容并报告偏移；真实 </head>（在截断
// 点之前）照常生效，截断点之后的孤立 <link> 不得被删除。
func TestRewriteStylesheetLinksTruncationWarns(t *testing.T) {
	head := "<html>\n" +
		"  <head>\n" +
		"    <title>T</title>\n" +
		"  </head>\n" +
		"  <body><p>正文</p></body>\n" +
		"</html>\n"
	orphanTail := "<!-- unterminated comment never closes\n" +
		"  <link rel=\"stylesheet\" type=\"text/css\" href=\"../Styles/orphan.css\"/>\n"
	text := head + orphanTail
	truncOffset := strings.Index(text, "<!-- unterminated")

	got, warnings, err := rewriteStylesheetLinks(text, "OEBPS/Text/chapter.xhtml", []string{"OEBPS/Styles/base.css"})
	if err != nil {
		t.Fatalf("rewriteStylesheetLinks: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("应恰好一条截断告警: %v", warnings)
	}
	offsetStr := strconv.Itoa(truncOffset)
	if !strings.Contains(warnings[0], offsetStr) {
		t.Fatalf("告警必须带正确的字节偏移 %s: %q", offsetStr, warnings[0])
	}
	// 截断点之后的字节必须原样保留：孤立 link 未被删除，注释未被闭合修复。
	if !strings.HasSuffix(got, orphanTail) {
		t.Fatalf("截断点之后必须原样保留:\n got:\n%s\n want suffix:\n%s", got, orphanTail)
	}
	// 截断点之前的真实 </head> 仍正常插入新链接。
	newLink := `<link rel="stylesheet" type="text/css" href="../Styles/base.css"/>`
	if !strings.Contains(got[:len(got)-len(orphanTail)], newLink) {
		t.Fatalf("截断点之前的真实 </head> 应正常插入新链接:\n%s", got)
	}
}
