// parity_test.go 原为 Python oracle 的 P2 parity 用例（oracle 已于
// 2026-08-29 删除）。这里保留同一组 fixture 场景，改为 Go-native 断言：
// 逐图明细（facts.imageFindings）、警告（facts.warningList）与信封 findings
// 的一致性。
package imagelayout

import (
	"sort"
	"strings"
	"testing"
)

// kindsByFile 把 imageFindings 折成 文件后缀 → 已排序 finding 种类列表。
func kindsByFile(findings []imageFinding) map[string][]string {
	out := map[string][]string{}
	for _, f := range findings {
		name := f.File[strings.LastIndex(f.File, "/")+1:]
		out[name] = append(out[name], f.Finding)
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

func assertKinds(t *testing.T, got map[string][]string, want map[string][]string) {
	t.Helper()
	for file, kinds := range want {
		if strings.Join(got[file], ",") != strings.Join(kinds, ",") {
			t.Errorf("%s findings = %v, want %v", file, got[file], kinds)
		}
	}
	for file := range got {
		if _, ok := want[file]; !ok {
			t.Errorf("%s 不应有 finding: %v", file, got[file])
		}
	}
}

// assertEnvelopeMirrorsFacts 锁定：信封 findings 与 facts.imageFindings 一一对应。
func assertEnvelopeMirrorsFacts(t *testing.T, path string) []imageFinding {
	t.Helper()
	res, findings := runAdvisor(t, path)
	if res.Status != "complete" {
		t.Errorf("advisor 恒为 complete，got %s", res.Status)
	}
	if len(res.Findings) != len(findings) || res.Facts["findings"] != len(findings) {
		t.Fatalf("findings 计数不一致: envelope=%d imageFindings=%d facts=%v", len(res.Findings), len(findings), res.Facts["findings"])
	}
	for i, f := range findings {
		env := res.Findings[i]
		if env.ID != f.Finding || env.Location != f.Selector || env.Detail != f.File+" · "+f.Image {
			t.Errorf("envelope finding[%d] = %+v 与 imageFindings %+v 不对应", i, env, f)
		}
		if f.Scene != "image-layout" || f.Image == "" || f.Selector == "" {
			t.Errorf("imageFindings[%d] 字段不完整: %+v", i, f)
		}
	}
	warnings, ok := res.Facts["warningList"].([]string)
	if !ok || len(warnings) != res.Facts["warnings"] {
		t.Errorf("warningList = %v (%T), facts.warnings = %v", res.Facts["warningList"], res.Facts["warningList"], res.Facts["warnings"])
	}
	return findings
}

func TestAdvisorHitBareImage(t *testing.T) {
	// 命中场景：裸图（lone / missing-alt / caption）；nav 收录且图在首位，
	// 同时命中章节头候选。
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"chapter.xhtml", `<img src="../Images/test.png"/><p>图注</p>`, ""},
	}, []string{"chapter.xhtml"})
	findings := assertEnvelopeMirrorsFacts(t, path)
	assertKinds(t, kindsByFile(findings), map[string][]string{
		"chapter.xhtml": {"caption-detached", "chapter-head-image-candidate", "lone-image-no-figure", "missing-alt"},
	})
}

func TestAdvisorMixedHitAcrossFiles(t *testing.T) {
	// 命中场景 2：float 风险 + 图注 + 整页候选，同一书内多文件。
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"bad.xhtml", `<img src="../Images/test.png" alt="test" style="float:left;width:50%"/><p>正文。</p>`, ""},
		{"fullpage.xhtml", `<figure><img src="../Images/test.png" alt="volume"/></figure>`, ""},
	}, []string{"bad.xhtml", "fullpage.xhtml"})
	findings := assertEnvelopeMirrorsFacts(t, path)
	assertKinds(t, kindsByFile(findings), map[string][]string{
		"bad.xhtml":      {"caption-detached", "chapter-head-image-candidate", "float-width-risk", "lone-image-no-figure"},
		"fullpage.xhtml": {"chapter-head-image-candidate", "fullpage-image-alite-candidate"},
	})
}

func TestAdvisorMissCanonicalFigure(t *testing.T) {
	// 未命中场景：规范 figure 且图不在首位，无任何 finding。
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"good.xhtml", `<h1>标题</h1><figure><img src="../Images/test.png" alt="test"/></figure><p>这是普通正文段落。</p>`, ""},
	}, []string{"good.xhtml"})
	findings := assertEnvelopeMirrorsFacts(t, path)
	if len(findings) != 0 {
		t.Errorf("规范 figure 不应有 finding: %+v", findings)
	}
}

func TestAdvisorChapterHeadAndNoterefExempt(t *testing.T) {
	// 命中 + 豁免混合：章节头候选（CSS 规范 figure 不触发 float 风险）、
	// noteref 图标豁免。
	path := advisorFixture(t, []struct{ name, body, bodyClass string }{
		{"head.xhtml", `<figure class="img-left" style="width:30%"><img src="../Images/test.png" alt="chapter" style="width:100%;height:auto"/></figure><h1>标题</h1><p>正文。</p>`, ""},
		{"notes.xhtml", `<p>正文<sup><a class="noteref-icon" href="#note"><img src="../Images/test.png" alt="注"/></a></sup>继续。</p>`, ""},
	}, []string{"head.xhtml"})
	findings := assertEnvelopeMirrorsFacts(t, path)
	assertKinds(t, kindsByFile(findings), map[string][]string{
		"head.xhtml": {"chapter-head-image-candidate"},
	})
}
