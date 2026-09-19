package cover

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/redline"
	"github.com/liyafly/epub-handbook/internal/report"
)

// TestRealBookReplaceCoverKeepsProse 原为 Python oracle 的真书 parity 用例
// （oracle 已于 2026-08-29 删除，`epub_cover_replace_harness.py` 不复存在）。
// 样本书含大量 href 带 "cover" 的图片 manifest 项——旧封面规则会把它们全部
// 移出 manifest、删除文件、把引用重写到新封面路径，命中面很大，正是
// refs.go 头注描述的「区域感知重写」最该在真书上验证的场景。
//
// 不再有 oracle 可比对，改用两条 Go-native 断言，思路与
// internal/caps/structure_normalize 的 TestRealBookNormalizeKeepsProse 一致：
//   - facts / Renames 证明确实发生了旧封面引用重写（不是空跑）；
//   - redline text 红线在替换前后零发现——这是本仓最高安全属性「正文
//     不变」，也是区域感知重写要修的缺陷（全文裸匹配会把转义写出的示例
//     文本当成标记误改）最终必须守住的门禁。
func TestRealBookReplaceCoverKeepsProse(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(repo, "references", "epubs", "*.epub"))
	if len(matches) == 0 {
		// 保留「没有样本书就跳过」这一层：样本书随仓库入 git 但体积较大，
		// 缺失是环境问题而非缺陷；不允许因为 oracle 缺失而跳过（oracle
		// 已经不在此测试的依赖路径上）。
		t.Skip("没有样本书")
	}
	source := matches[0]

	dir := t.TempDir()
	cover := filepath.Join(dir, "new-cover.png")
	dims := pngDimsHeader()
	if err := os.WriteFile(cover, dims, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "cover.epub")

	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{Cover: cover, Output: out})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != report.StatusComplete {
		t.Fatalf("status = %s: %+v", res.Status, res.Findings)
	}
	if err := b.WriteTo(out); err != nil {
		t.Fatal(err)
	}

	assertOperationFacts(t, res, out)
	if len(res.Renames) == 0 {
		t.Fatal("样本书应触发旧封面引用改名，否则本回归对区域感知重写无效")
	}
	mappings, ok := res.Facts["mappings"].([]map[string]string)
	if !ok || len(mappings) != len(res.Renames) {
		t.Fatalf("facts[mappings] 应与 Renames 一一对应: mappings=%v renames=%v", res.Facts["mappings"], res.Renames)
	}
	for _, m := range mappings {
		if to, exists := res.Renames[m["from"]]; !exists || to != m["to"] {
			t.Errorf("mappings 条目 %v 与 Renames 不一致（Renames[%q]=%q）", m, m["from"], to)
		}
	}

	// 正文不变：不给 allow-list，真书上零发现，加了反而会掩盖区域感知
	// 重写没做对时对转义示例文本的误改。
	findings, err := redline.Check(redline.OriginalState(b), redline.CurrentState(b),
		[]string{redline.CheckText}, redline.Options{PathMap: res.Renames})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		t.Errorf("redline %s: %s", f.Check, f.Message)
	}
}
