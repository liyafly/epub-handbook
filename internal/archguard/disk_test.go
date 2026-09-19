package archguard

import (
	"go/ast"
	"path"
	"strconv"
	"testing"
)

// diskWriteAllowed 是允许持有写句柄的包（INV-3：zipfs 是唯一磁盘边界，
// extern 需要给外部工具准备输入/接收输出）。
var diskWriteAllowed = map[string]bool{
	"internal/zipfs":  true,
	"internal/extern": true,
}

// osWriteAPIs 是 os 包里会产生写副作用的函数。
var osWriteAPIs = map[string]bool{
	"Create": true, "CreateTemp": true, "WriteFile": true, "OpenFile": true,
	"Truncate": true, "Rename": true, "Remove": true, "RemoveAll": true,
	"Mkdir": true, "MkdirAll": true, "MkdirTemp": true, "Symlink": true, "Link": true,
}

// ioutilWriteAPIs 是 io/ioutil 里会产生写副作用的函数（已废弃的包，但仍能编译）。
var ioutilWriteAPIs = map[string]bool{
	"WriteFile": true, "TempFile": true, "TempDir": true,
}

// stdImportAliases 返回本文件里「本地包名 → 标准库导入路径」的映射。
// importAliases 只覆盖仓库内包；这里要的是 os / io/ioutil 这类标准库。
func stdImportAliases(g goFile) map[string]string {
	out := map[string]string{}
	for _, spec := range g.AST.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil || internalPkg(p) != "" {
			continue
		}
		local := path.Base(p)
		if spec.Name != nil {
			if spec.Name.Name == "_" || spec.Name.Name == "." {
				continue // 空导入无调用点；点导入不产生选择器
			}
			local = spec.Name.Name
		}
		out[local] = p
	}
	return out
}

// TestSingleWrite 断言 INV-3：白名单外的包不得产生磁盘写副作用。
//
// 这条堵的是「把中间结果写临时文件再读回来」——也就是退回 Python 版
// subprocess-per-stage 架构的那条路。旧架构跑一本 49MB 的书要产生约 800MB 无谓 I/O。
func TestSingleWrite(t *testing.T) {
	root := repoRoot(t)
	files := append(collect(t, root, "cmd", false), collect(t, root, "internal", false)...)

	for _, g := range files {
		if diskWriteAllowed[g.Pkg] {
			continue
		}
		// 按本文件的导入表把「字面标识符」解析成真实导入路径。
		// 2026-09-07 收紧（经仓库所有者授权）：此前直接比较 pkg == "os"，于是
		// `import osx "os"` 后调用 osx.WriteFile 完全绕得过 —— 守卫拦的是变量名
		// 而规则说的是那个包。io/ioutil 的 WriteFile/TempFile 同理，此前也不在
		// 检查范围内。
		stdAliases := stdImportAliases(g)
		selectorCalls(g, func(pkg, name string, node ast.Node) {
			importPath, known := stdAliases[pkg]
			if !known {
				return
			}
			switch importPath {
			case "os":
				if !osWriteAPIs[name] {
					return
				}
			case "io/ioutil":
				if !ioutilWriteAPIs[name] {
					return
				}
			default:
				return
			}
			t.Errorf("%s: 包 %q 调用了 %s.%s（本文件里以 %q 引用）。\n"+
				"  INV-3：一次运行只写一次输出 EPUB，中间态一律留在内存。\n"+
				"  只有 internal/zipfs 和 internal/extern 允许持有写句柄。",
				g.pos(node), g.Pkg, importPath, name, pkg)
		})
	}
}
