package archguard

import "testing"

// execAllowed 是允许起外部进程的包（INV-4：extern 是唯一进程边界）。
var execAllowed = map[string]bool{
	"internal/extern": true,
}

// TestNoExecInCaps 断言 INV-4：caps 内禁止 os/exec。
func TestNoExecInCaps(t *testing.T) {
	root := repoRoot(t)
	files := append(collect(t, root, "cmd", false), collect(t, root, "internal", false)...)

	for _, g := range files {
		if execAllowed[g.Pkg] {
			continue
		}
		for _, imp := range imports(g) {
			if imp != "os/exec" {
				continue
			}
			t.Errorf("%s: 包 %q 禁止 import os/exec。\n"+
				"  INV-4：所有外部进程调用必须经 internal/extern，\n"+
				"  那里统一处理工具缺失的降级（magick / oxipng 在很多机器上不存在）。",
				g.Rel, g.Pkg)
		}
	}
}
