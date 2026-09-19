package archguard

import (
	"go/ast"
	"go/token"
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
//  2. 注册表 —— **仅在 init() 期写入**（SPEC §2 INV-7 的原文）
//
// 第 2 类的判定是行为性的，不是文件名性的（2026-09-07 收紧，经仓库所有者授权）：
// 此前只要文件名叫 `register.go` 就整份豁免，于是「把一个运行期才写入的缓存
// map 放进任何一个叫 register.go 的文件」即可完全绕过这条不变式 —— 豁免的是
// 文件名，而规则说的是写入时机。现在改为：
//
//   - 声明处初始化（`var t = map[...]{...}`）永远允许 —— 那是 init 期；
//   - 声明之后的写入必须发生在 `init()` 函数体内，或发生在一个**只被 init()
//     调用**的函数里（`redline.Register` 就是这个形状：由 legacy.go 的 init()
//     逐条调用）。间接层数不限：中间函数自己的调用点同样要求全在 init 里，
//     递归检查。
//
// 只扫生产文件（不含 _test.go）。
//
// 已知边界（不隐瞒）：这是纯 AST 分析，没有类型信息，以下路径检不出来 ——
//   - 通过接口值、函数值或反射间接写入；
//   - 把包级 map / slice 当参数传给一个会写它的函数（写入发生在形参上）。
//
// `&x` 一律按「可写」保守处理。检出的写入形式：`x = …`、`x op= …`、
// `x[k] = …`、`x.f = …`、`x++` / `x--`、`&x`。
func TestNoPackageState(t *testing.T) {
	root := repoRoot(t)
	files := collect(t, root, "internal", false)
	files = append(files, collect(t, root, "cmd", false)...)

	// 1. 收集每个包的包级 var（排除 error 哨兵与 `_`）。
	type varDecl struct {
		file goFile
		name *ast.Ident
	}
	pkgVars := map[string]map[string]varDecl{} // pkg → 变量名 → 声明
	for _, g := range files {
		if strings.HasPrefix(g.Pkg, "internal/archguard") {
			continue
		}
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
					if n == "_" || strings.HasPrefix(n, "Err") || strings.HasPrefix(n, "err") {
						continue
					}
					if pkgVars[g.Pkg] == nil {
						pkgVars[g.Pkg] = map[string]varDecl{}
					}
					pkgVars[g.Pkg][n] = varDecl{file: g, name: name}
				}
			}
		}
	}
	if len(pkgVars) == 0 {
		return
	}

	// 2. 建立「函数 → 它调用了哪些同包函数」与「函数 → 它写了哪些包级 var」。
	//    键是 pkg + "." + funcName；init 用其位置区分（同包可有多个 init）。
	type funcKey struct{ pkg, name string }
	writesBy := map[funcKey]map[string]ast.Node{} // 函数 → 写到的包级 var
	calledBy := map[funcKey]map[funcKey]bool{}    // 被调函数 → 调用它的函数集合
	for _, g := range files {
		vars := pkgVars[g.Pkg]
		for _, d := range g.AST.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			self := funcKey{pkg: g.Pkg, name: fd.Name.Name}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				// 写入：赋值 / 复合赋值的左值根标识符。
				if as, ok := n.(*ast.AssignStmt); ok {
					for _, lhs := range as.Lhs {
						if name := rootIdent(lhs); name != "" && vars != nil {
							if _, isPkgVar := vars[name]; isPkgVar {
								if writesBy[self] == nil {
									writesBy[self] = map[string]ast.Node{}
								}
								writesBy[self][name] = as
							}
						}
					}
				}
				// 自增/自减：`x++` / `x--` 是 IncDecStmt，不是 AssignStmt。
				if inc, ok := n.(*ast.IncDecStmt); ok {
					if name := rootIdent(inc.X); name != "" && vars != nil {
						if _, isPkgVar := vars[name]; isPkgVar {
							if writesBy[self] == nil {
								writesBy[self] = map[string]ast.Node{}
							}
							writesBy[self][name] = inc
						}
					}
				}
				// 取地址：把写权交出去，保守当作写。
				if ue, ok := n.(*ast.UnaryExpr); ok && ue.Op == token.AND {
					if name := rootIdent(ue.X); name != "" && vars != nil {
						if _, isPkgVar := vars[name]; isPkgVar {
							if writesBy[self] == nil {
								writesBy[self] = map[string]ast.Node{}
							}
							writesBy[self][name] = ue
						}
					}
				}
				// 同包函数调用（无选择器的裸调用）。
				if call, ok := n.(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok {
						callee := funcKey{pkg: g.Pkg, name: id.Name}
						if calledBy[callee] == nil {
							calledBy[callee] = map[funcKey]bool{}
						}
						calledBy[callee][self] = true
					}
				}
				return true
			})
		}
	}
	// 跨包调用：被调函数可能是导出的（如 redline.Register）。补上选择器调用。
	for _, g := range files {
		for _, d := range g.AST.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			self := funcKey{pkg: g.Pkg, name: fd.Name.Name}
			imports := importAliases(g)
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				base, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				target, known := imports[base.Name]
				if !known {
					return true
				}
				callee := funcKey{pkg: target, name: sel.Sel.Name}
				if calledBy[callee] == nil {
					calledBy[callee] = map[funcKey]bool{}
				}
				calledBy[callee][self] = true
				return true
			})
		}
	}

	// 3. onlyInit 判定一个函数是否只在 init 期被执行（递归，带环保护）。
	var onlyInit func(fn funcKey, seen map[funcKey]bool) bool
	onlyInit = func(fn funcKey, seen map[funcKey]bool) bool {
		if fn.name == "init" {
			return true
		}
		if seen[fn] {
			return true // 环：不把它当成新的违规来源，由环上其它节点裁定
		}
		seen[fn] = true
		callers := calledBy[fn]
		if len(callers) == 0 {
			return false // 没有 init 调它却写了包级状态 —— 不在白名单里
		}
		for c := range callers {
			if !onlyInit(c, seen) {
				return false
			}
		}
		return true
	}

	// 4. 逐条裁定：写入点要么在 init，要么在只被 init 调用的函数里。
	offending := map[string]map[string]bool{} // pkg → var → 已报过
	for fn, written := range writesBy {
		if onlyInit(fn, map[funcKey]bool{}) {
			continue
		}
		for name, node := range written {
			if offending[fn.pkg] == nil {
				offending[fn.pkg] = map[string]bool{}
			}
			if offending[fn.pkg][name] {
				continue
			}
			offending[fn.pkg][name] = true
			decl := pkgVars[fn.pkg][name]
			t.Errorf("%s: 包级变量 %q 在 %s() 里被写入，而 %s() 不是只在 init 期执行。\n"+
				"  （声明处：%s）\n"+
				"  INV-7：internal/** 禁止包级可变状态。\n"+
				"  改法：把它变成 Run() 的参数或返回值，或提升为显式传递的 struct 字段。\n"+
				"  白名单仅限 error 哨兵（Err 前缀）与仅在 init() 期写入的注册表 ——\n"+
				"  注意豁免的是「写入时机」，不是文件名。",
				decl.file.pos(node), name, fn.name, fn.name, decl.file.pos(decl.name))
		}
	}
}

// rootIdent 取左值表达式的根标识符：`x`、`x[k]`、`x.f`、`x[i].f` 都返回 "x"。
// 不是以标识符打头（如函数调用结果）时返回空串。
func rootIdent(e ast.Expr) string {
	for {
		switch v := e.(type) {
		case *ast.Ident:
			return v.Name
		case *ast.IndexExpr:
			e = v.X
		case *ast.SelectorExpr:
			e = v.X
		case *ast.StarExpr:
			e = v.X
		case *ast.ParenExpr:
			e = v.X
		default:
			return ""
		}
	}
}
