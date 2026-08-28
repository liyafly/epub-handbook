package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// modulePath 必须与 go.mod 的 module 行一致。
const modulePath = "github.com/liyafly/epub-handbook"

// repoRoot 从工作目录向上找 go.mod 所在目录。
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("向上未找到 go.mod")
		}
		dir = parent
	}
}

// goFile 是一个已解析的源文件。
type goFile struct {
	Rel  string // 相对仓库根的文件路径
	Pkg  string // 相对仓库根的包目录，如 internal/caps/foo
	AST  *ast.File
	Fset *token.FileSet
}

// pos 返回给定节点的可点击位置，形如 internal/caps/foo/foo.go:42。
func (g goFile) pos(n ast.Node) string {
	p := g.Fset.Position(n.Pos())
	return g.Rel + ":" + strconv.Itoa(p.Line)
}

// collect 解析 root/sub 子树下的全部 .go 文件。
// sub 不存在时返回 nil（bootstrap 阶段包尚未创建）。
func collect(t *testing.T, root, sub string, includeTests bool) []goFile {
	t.Helper()
	base := filepath.Join(root, sub)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil
	}
	fset := token.NewFileSet()
	var out []goFile
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// 不下钻到 testdata 与隐藏目录。
			if d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		if !includeTests && strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatalf("解析 %s 失败: %v", p, perr)
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		out = append(out, goFile{
			Rel:  rel,
			Pkg:  filepath.ToSlash(filepath.Dir(rel)),
			AST:  f,
			Fset: fset,
		})
		return nil
	})
	if err != nil {
		t.Fatalf("遍历 %s 失败: %v", base, err)
	}
	return out
}

// imports 返回该文件 import 的全部路径（已去引号）。
func imports(g goFile) []string {
	var out []string
	for _, spec := range g.AST.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		out = append(out, p)
	}
	return out
}

// internalPkg 把 import 路径转成相对仓库根的包目录；不是本模块的包则返回 ""。
func internalPkg(importPath string) string {
	if !strings.HasPrefix(importPath, modulePath+"/") {
		return ""
	}
	return strings.TrimPrefix(importPath, modulePath+"/")
}

// exportedFuncs 返回文件里的导出**函数**（不含方法）。
func exportedFuncs(g goFile) []*ast.FuncDecl {
	var out []*ast.FuncDecl
	for _, d := range g.AST.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		if fn.Name.IsExported() {
			out = append(out, fn)
		}
	}
	return out
}

// selectorCalls 遍历文件，对每个形如 pkg.Name(...) 的调用回调。
func selectorCalls(g goFile, fn func(pkg, name string, node ast.Node)) {
	ast.Inspect(g.AST, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		fn(ident.Name, sel.Sel.Name, call)
		return true
	})
}

// typeString 把类型表达式还原成源码文本（够用即可，不追求完全等价）。
func typeString(fset *token.FileSet, e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.ArrayType:
		if v.Len == nil {
			return "[]" + typeString(fset, v.Elt)
		}
		return "[N]" + typeString(fset, v.Elt)
	case *ast.StarExpr:
		return "*" + typeString(fset, v.X)
	case *ast.SelectorExpr:
		return typeString(fset, v.X) + "." + v.Sel.Name
	case *ast.Ellipsis:
		return "..." + typeString(fset, v.Elt)
	case *ast.MapType:
		return "map[" + typeString(fset, v.Key) + "]" + typeString(fset, v.Value)
	default:
		return "?"
	}
}

// results 返回函数签名的全部返回类型文本。
func results(g goFile, fn *ast.FuncDecl) []string {
	if fn.Type.Results == nil {
		return nil
	}
	var out []string
	for _, f := range fn.Type.Results.List {
		s := typeString(g.Fset, f.Type)
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, s)
		}
	}
	return out
}

// dirExists 用于 Tier B 的 bootstrap 让路判断。
func dirExists(root, sub string) bool {
	st, err := os.Stat(filepath.Join(root, sub))
	return err == nil && st.IsDir()
}

// unquote 去掉 Go 字符串字面量的引号。
func unquote(lit string) (string, error) {
	return strconv.Unquote(lit)
}
