package archguard

import (
	"go/ast"
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
		selectorCalls(g, func(pkg, name string, node ast.Node) {
			if pkg == "os" && osWriteAPIs[name] {
				t.Errorf("%s: 包 %q 调用了 os.%s。\n"+
					"  INV-3：一次运行只写一次输出 EPUB，中间态一律留在内存。\n"+
					"  只有 internal/zipfs 和 internal/extern 允许持有写句柄。",
					g.pos(node), g.Pkg, name)
			}
		})
	}
}
