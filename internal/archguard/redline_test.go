package archguard

import (
	"encoding/json"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type capabilityContract struct {
	ID       string   `json:"id"`
	RedLines []string `json:"redLines"`
}

// TestRedlineClosure 断言 INV-5：契约声明过的每条红线都必须有已注册的校验器。
//
// redLines 是本项目的安全底线（text / metadata / spine / anchors / cover / drm）。
// 契约声明了却没实现 = 静默失去保护。
// 新增契约时若用了新红线，本测试会立刻红 —— 这是免费拿到的闭包检查。
func TestRedlineClosure(t *testing.T) {
	root := repoRoot(t)

	// 1. 从契约收集红线并集。
	declared := map[string][]string{} // redline -> 声明它的 capability id
	pattern := filepath.Join(root, "contracts", "capabilities", "v1", "*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob 契约失败: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("未找到任何 capability 契约（%s）。\n"+
			"  契约是架构的输入，不是可选项。", pattern)
	}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", p, err)
		}
		var c capabilityContract
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatalf("解析 %s 失败: %v", p, err)
		}
		for _, r := range c.RedLines {
			declared[r] = append(declared[r], c.ID)
		}
	}

	// 2. bootstrap 让路：internal/redline 尚未创建时放行一次。
	//    目录一旦存在，本测试就再也无法跳过。
	if !dirExists(root, "internal/redline") {
		t.Skipf("bootstrap：internal/redline 尚未创建，暂缓。\n"+
			"  契约当前声明了 %d 条红线，创建该包后必须全部注册。", len(declared))
	}

	// 3. 从 internal/redline 收集已注册的红线名。
	registered := map[string]bool{}
	for _, g := range collect(t, root, "internal/redline", false) {
		ast.Inspect(g.AST, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			name := ""
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				name = fn.Name
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			}
			if name != "Register" {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				return true
			}
			if s, err := unquote(lit.Value); err == nil {
				registered[s] = true
			}
			return true
		})
	}

	// 4. 比对。
	var missing []string
	for r := range declared {
		if !registered[r] {
			missing = append(missing, r)
		}
	}
	sort.Strings(missing)
	for _, r := range missing {
		t.Errorf("红线 %q 被契约声明但没有注册校验器（声明者：%v）。\n"+
			"  INV-5 修法见 SPEC §6.3，三处必须同时到位：\n"+
			"    1. internal/redline/%s.go        实现 Validator\n"+
			"    2. internal/redline/register.go  注册\n"+
			"    3. contracts 里的 redLines 数组\n"+
			"  不要通过从契约里删掉这条红线来让测试变绿。", r, declared[r], r)
	}
}
