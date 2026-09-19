package alite

import (
	"strconv"
	"strings"
	"testing"
)

// regions_test.go 钉住三处从「对整页/整段文本跑正则或裸字符串扫描」改成
// 「只在 xhtml.ScanRegions 认定的真实标签字节内匹配」之后的行为：
//   - ensureStylesheetLink：真实 </head> 前正确插入 <link>；
//     HTML 注释、<script> 字符串字面量里同形的 </head> 完全不受影响；
//   - hasClassToken（原 cardRe 裸正则）：正文字符数据里原样写出的
//     class="copyright-card" 不再被误判为「结构已存在」，包裹仍会发生；
//   - addClassToTag：注释里同形的 <li ...> 完全不受影响；
//   - 三者在扫描截断时都给出带偏移的 warning，且截断点之后的内容不改。

// TestEnsureStylesheetLinkRealHead 覆盖最基本路径：真实 </head> 前插入。
func TestEnsureStylesheetLinkRealHead(t *testing.T) {
	text := "<html>\n  <head>\n    <title>T</title>\n  </head>\n  <body></body>\n</html>\n"
	got, changed, warnings := ensureStylesheetLink(text, "../Styles/x.css")
	if !changed {
		t.Fatalf("应插入链接:\n%s", got)
	}
	if len(warnings) != 0 {
		t.Fatalf("不应产生截断告警: %v", warnings)
	}
	want := `<link href="../Styles/x.css" type="text/css" rel="stylesheet"/>`
	if !strings.Contains(got, want) {
		t.Fatalf("缺少新插入的 link:\n%s", got)
	}
	if idx := strings.Index(got, want); idx < 0 || idx > strings.Index(got, "</head>") {
		t.Fatalf("新 link 必须插在 </head> 之前:\n%s", got)
	}
}

// TestEnsureStylesheetLinkIgnoresCommentAndScript 是缺陷回归用例：注释与
// <script> 字符串字面量里同形的 </head> 不得被当成插入点或被改写。
func TestEnsureStylesheetLinkIgnoresCommentAndScript(t *testing.T) {
	text := "<html>\n  <head>\n    <title>T</title>\n" +
		"    <!-- old way: </head> shouldn't be here -->\n" +
		"    <script>var s = \"</head>\";</script>\n" +
		"  </head>\n  <body></body>\n</html>\n"
	got, changed, warnings := ensureStylesheetLink(text, "../Styles/x.css")
	if !changed {
		t.Fatalf("应插入链接:\n%s", got)
	}
	if len(warnings) != 0 {
		t.Fatalf("不应产生截断告警: %v", warnings)
	}
	if !strings.Contains(got, "<!-- old way: </head> shouldn't be here -->") {
		t.Fatalf("注释必须原样保留:\n%s", got)
	}
	if !strings.Contains(got, `<script>var s = "</head>";</script>`) {
		t.Fatalf("<script> 内容必须原样保留:\n%s", got)
	}
	want := `<link href="../Styles/x.css" type="text/css" rel="stylesheet"/>`
	realHeadIdx := strings.LastIndex(got, "</head>")
	linkIdx := strings.Index(got, want)
	if linkIdx < 0 || linkIdx > realHeadIdx {
		t.Fatalf("新 link 必须插在真实 </head> 之前:\n%s", got)
	}
	scriptEnd := strings.Index(got, "</script>") + len("</script>")
	if linkIdx < scriptEnd {
		t.Fatalf("插入点不得落在注释/脚本内部:\n%s", got)
	}
}

// TestEnsureStylesheetLinkTruncationWarns 覆盖截断路径：真实 </head> 在
// 截断点之前照常生效；截断点之后即便还有一个字面 </head>，也不得被使用
// 或改动，且必须报告偏移。
func TestEnsureStylesheetLinkTruncationWarns(t *testing.T) {
	head := "<html>\n  <head>\n    <title>T</title>\n  </head>\n  <body></body>\n</html>\n"
	orphanTail := "<!-- unterminated comment never closes\n  </head>\n"
	text := head + orphanTail
	truncOffset := strings.Index(text, "<!-- unterminated")

	got, changed, warnings := ensureStylesheetLink(text, "../Styles/x.css")
	if !changed {
		t.Fatalf("截断点之前的真实 </head> 应正常生效:\n%s", got)
	}
	if len(warnings) != 1 {
		t.Fatalf("应恰好一条截断告警: %v", warnings)
	}
	if !strings.Contains(warnings[0], strconv.Itoa(truncOffset)) {
		t.Fatalf("告警必须带正确的字节偏移 %d: %q", truncOffset, warnings[0])
	}
	if !strings.HasSuffix(got, orphanTail) {
		t.Fatalf("截断点之后必须原样保留:\n got:\n%s\n want suffix:\n%s", got, orphanTail)
	}
}

// TestHasClassTokenIgnoresCharacterData 是缺陷回归用例：原 cardRe 是裸
// 正则（不要求前导 `<`），版权页正文里原样写出的 class="copyright-card"
// 会被误判为「结构已存在」而漏做包裹。改为只在真实标签的属性内查找后，
// 字符数据里的同形文字不再算数；真实标签的 class 仍能被正确识别。
func TestHasClassTokenIgnoresCharacterData(t *testing.T) {
	content := `<p class="cp">示例写法：class="copyright-card"</p>`
	got, warnings := hasClassToken(content, "copyright-card")
	if got {
		t.Fatal("字符数据里的同形文字不得被判定为已存在结构")
	}
	if len(warnings) != 0 {
		t.Fatalf("不应产生截断告警: %v", warnings)
	}

	real := `<section class="copyright-card"><p>x</p></section>`
	got2, warnings2 := hasClassToken(real, "copyright-card")
	if !got2 {
		t.Fatal("真实标签的 class 应被识别")
	}
	if len(warnings2) != 0 {
		t.Fatalf("不应产生截断告警: %v", warnings2)
	}
}

// TestRefineCopyrightWrapsDespiteLiteralClassText 端到端覆盖同一缺陷：
// 版权页正文里原样写出 class="copyright-card" 示例文字时，refineCopyright
// 仍必须完成真正的 <section class="copyright-card"> 结构包裹（断言包裹
// 确实发生），而不是把示例文字误判为「已包裹」而跳过。
func TestRefineCopyrightWrapsDespiteLiteralClassText(t *testing.T) {
	value := aliteXHTML("版权信息", `<p class="cp">示例写法：class="copyright-card"</p>
    <ul class="list">
      <li class="i">书名：测试</li>
    </ul>`)
	updated, warnings, err := refineCopyright(value, "../Styles/anthology-refinement.css")
	if err != nil {
		t.Fatalf("refineCopyright: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("不应产生截断告警: %v", warnings)
	}
	if !strings.Contains(updated, `<section class="copyright-card" epub:type="frontmatter copyright-page">`) {
		t.Fatalf("结构包裹应发生，即使正文里原样写出 class=\"copyright-card\":\n%s", updated)
	}
}

// TestAddClassToTagIgnoresComment 覆盖 addClassToTag 从手写 strings.Index
// 扫描改成 xhtml.ScanRegions 之后的行为：注释里同形的 <li ...> 不受影响。
func TestAddClassToTagIgnoresComment(t *testing.T) {
	content := `<!-- 旧写法: <li class="i">x</li> --><li class="i">y</li>`
	got, warnings := addClassToTag(content, "li", "copyright-meta-item", "i")
	if len(warnings) != 0 {
		t.Fatalf("不应产生截断告警: %v", warnings)
	}
	if !strings.Contains(got, `<!-- 旧写法: <li class="i">x</li> -->`) {
		t.Fatalf("注释必须原样保留:\n%s", got)
	}
	if !strings.Contains(got, `<li class="i copyright-meta-item">y</li>`) {
		t.Fatalf("真实标签应被改写:\n%s", got)
	}
}

// TestAddClassToTagTruncationWarns 覆盖截断路径：截断点之前的真实标签
// 正常加 class；截断点之后（含另一个满足条件的 <li>）保持原样，并报告
// 带偏移的告警。
func TestAddClassToTagTruncationWarns(t *testing.T) {
	head := `<li class="i">x</li>`
	orphanTail := "<!-- unterminated\n<li class=\"i\">y</li>\n"
	content := head + orphanTail
	truncOffset := strings.Index(content, "<!-- unterminated")

	got, warnings := addClassToTag(content, "li", "copyright-meta-item", "i")
	if len(warnings) != 1 {
		t.Fatalf("应恰好一条截断告警: %v", warnings)
	}
	if !strings.Contains(warnings[0], strconv.Itoa(truncOffset)) {
		t.Fatalf("告警必须带正确的字节偏移 %d: %q", truncOffset, warnings[0])
	}
	if !strings.HasSuffix(got, orphanTail) {
		t.Fatalf("截断点之后必须原样保留:\n got:\n%s\n want suffix:\n%s", got, orphanTail)
	}
	if !strings.Contains(got, `<li class="i copyright-meta-item">x</li>`) {
		t.Fatalf("截断点之前的真实标签应正常改写:\n%s", got)
	}
}
