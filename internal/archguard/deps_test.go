package archguard

import (
	"strings"
	"testing"
)

// layer 定义 SPEC-go-architecture.md §1 的依赖方向。
// 数字越小越靠上层。规则：只能 import 层号**严格更大**的包。
//
// 同层互相 import 一律禁止 —— 这条同时封死了 caps↔caps 和 scan/xhtml↔scan/css。
var layer = map[string]int{
	"cmd/epub":          0,
	"internal/pipeline": 1,
	"internal/caps":     2,
	"internal/redline":  3,
	"internal/report":   3,
	"internal/extern":   3,
	"internal/scan":     4,
	"internal/book":     5,
	"internal/zipfs":    6,
	"internal/editset":  6,
}

// layerOf 用最长前缀匹配确定包所属层。未登记的包返回 -1。
func layerOf(pkg string) (int, string) {
	best, bestKey := -1, ""
	for prefix, n := range layer {
		if pkg == prefix || strings.HasPrefix(pkg, prefix+"/") {
			if len(prefix) > len(bestKey) {
				best, bestKey = n, prefix
			}
		}
	}
	return best, bestKey
}

// TestLayerDirection 断言 §1：依赖只能向下。
func TestLayerDirection(t *testing.T) {
	root := repoRoot(t)
	files := append(collect(t, root, "cmd", false), collect(t, root, "internal", false)...)

	for _, g := range files {
		if strings.HasPrefix(g.Pkg, "internal/archguard") {
			continue
		}
		from, fromKey := layerOf(g.Pkg)
		if from < 0 {
			t.Errorf("%s: 包 %q 未在 archguard 的 layer 表中登记。\n"+
				"  新增包必须先在 SPEC §1/§3 里定义它的层级和职责，再登记到这里。\n"+
				"  不要为了让测试通过而随手加一行 —— 先想清楚它属于哪一层。", g.Rel, g.Pkg)
			continue
		}
		for _, imp := range imports(g) {
			target := internalPkg(imp)
			if target == "" {
				continue // 标准库或第三方，依赖预算由 §8.3 管
			}
			to, toKey := layerOf(target)
			if to < 0 {
				t.Errorf("%s: import 了未登记的包 %q", g.Rel, target)
				continue
			}
			if to > from {
				continue // 向下，合法
			}
			if fromKey == toKey && from == to {
				t.Errorf("%s: 同层互相 import —— %q → %q（都在 %q 层）。\n"+
					"  §4 禁止清单第 3 条：caps 之间要依赖，走契约的 requires 字段，由 pipeline 注入。",
					g.Rel, g.Pkg, target, fromKey)
				continue
			}
			t.Errorf("%s: 违反依赖方向 —— %q(层%d) → %q(层%d)。\n"+
				"  只能 import 层号更大的包。要么调整设计，要么这条依赖本就不该存在。",
				g.Rel, g.Pkg, from, target, to)
		}
	}
}

// TestCapsAreIsolated 断言 §4 第 3 条：caps 之间零耦合。
// 与 TestLayerDirection 有重叠，但单独成测是为了让失败信息足够刺眼。
func TestCapsAreIsolated(t *testing.T) {
	root := repoRoot(t)
	for _, g := range collect(t, root, "internal/caps", false) {
		for _, imp := range imports(g) {
			target := internalPkg(imp)
			if !strings.HasPrefix(target, "internal/caps/") {
				continue
			}
			if target == g.Pkg {
				continue
			}
			t.Errorf("%s: capability 之间禁止直接 import（%q → %q）。\n"+
				"  正确做法：在 contracts/capabilities/v1/<id>.json 的 requires 里声明依赖，\n"+
				"  由 pipeline 排序并把上游结果作为参数传进来。", g.Rel, g.Pkg, target)
		}
	}
}

// TestCmdIsThin 断言 §3：cmd/epub 零业务逻辑。
// 判据：只允许 import pipeline 和标准库，不许直接碰 book/zipfs/scan 等。
func TestCmdIsThin(t *testing.T) {
	root := repoRoot(t)
	for _, g := range collect(t, root, "cmd", false) {
		for _, imp := range imports(g) {
			target := internalPkg(imp)
			if target == "" || target == "internal/pipeline" {
				continue
			}
			t.Errorf("%s: cmd 只能 import internal/pipeline，不能直接用 %q。\n"+
				"  §3：cmd 只做 flag 解析、退出码，不含任何 EPUB 知识。", g.Rel, target)
		}
	}
}
