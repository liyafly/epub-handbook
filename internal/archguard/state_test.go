package archguard

import (
	"go/ast"
	"go/token"
	"path"
	"strings"
	"testing"
)

// TestNoPackageState 断言 INV-7：internal/** 禁止包级可变 var。
//
// 这条专门堵弱模型最常见的跑偏方式：「加个全局变量把状态传过去」。
// 禁掉之后数据流被迫走参数和返回值，stage 保持可并行、可单测。
//
// 白名单只有两类：
//  1. error 哨兵（var ErrXxx = ...）
//  2. register.go 里的注册表（仅在 init() 期写入）
func TestNoPackageState(t *testing.T) {
	root := repoRoot(t)

	for _, g := range collect(t, root, "internal", false) {
		if strings.HasPrefix(g.Pkg, "internal/archguard") {
			continue
		}
		isRegistry := path.Base(g.Rel) == "register.go"

		for _, d := range g.AST.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue // const / type / import 不受限
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					n := name.Name
					if n == "_" {
						continue
					}
					if strings.HasPrefix(n, "Err") || strings.HasPrefix(n, "err") {
						continue // error 哨兵
					}
					if isRegistry {
						continue // 注册表
					}
					t.Errorf("%s: 包级变量 %q。\n"+
						"  INV-7：internal/** 禁止包级可变状态。\n"+
						"  改法：把它变成 Run() 的参数或返回值，或提升为显式传递的 struct 字段。\n"+
						"  白名单仅限 error 哨兵（Err 前缀）与 register.go 里的注册表。",
						g.pos(name), n)
				}
			}
		}
	}
}
