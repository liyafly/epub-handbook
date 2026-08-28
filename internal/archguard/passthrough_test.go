package archguard

import (
	"strings"
	"testing"
)

// TestZipIsolated 断言 INV-1 的静态前提：archive/zip 只能出现在 zipfs 里。
//
// 只要容器读写集中在一个包，"未修改 entry 必须原样透传"才有唯一的落实点。
func TestZipIsolated(t *testing.T) {
	root := repoRoot(t)
	files := append(collect(t, root, "cmd", false), collect(t, root, "internal", false)...)

	for _, g := range files {
		if g.Pkg == "internal/zipfs" || strings.HasPrefix(g.Pkg, "internal/archguard") {
			continue
		}
		for _, imp := range imports(g) {
			if imp == "archive/zip" {
				t.Errorf("%s: 包 %q import 了 archive/zip。\n"+
					"  INV-1：容器读写必须收口到 internal/zipfs，\n"+
					"  否则「未修改 entry 用 zip.Writer.Copy 原样透传」无法被统一保证。",
					g.Rel, g.Pkg)
			}
		}
	}
}

// TestPassthroughTestExists 断言 INV-1 的行为测试没有被删掉。
//
// INV-1 的真实验证（逐 entry 比对 CRC32 / CompressedSize / Method）住在
// internal/zipfs/passthrough_test.go 里，因为只有那里能直接调容器 API。
// archguard 在这里断言那个测试**存在** —— 于是它删不掉。
func TestPassthroughTestExists(t *testing.T) {
	root := repoRoot(t)
	if !dirExists(root, "internal/zipfs") {
		t.Skip("bootstrap：internal/zipfs 尚未创建，暂缓。" +
			"  创建该包时必须同时写 TestRawPassthrough。")
	}

	const want = "TestRawPassthrough"
	for _, g := range collect(t, root, "internal/zipfs", true) {
		for _, fn := range exportedFuncs(g) {
			if fn.Name.Name == want {
				return
			}
		}
	}
	t.Errorf("internal/zipfs 里找不到 %s。\n"+
		"  INV-1 的行为验证必须存在：构造一个多 entry 的 zip，改其中一个，\n"+
		"  逐 entry 断言未修改项的 CRC32 / CompressedSize / Method 与输入完全一致。\n"+
		"  这是「49MB 的书只搬运改动部分」这一性能承诺的唯一凭据。", want)
}
