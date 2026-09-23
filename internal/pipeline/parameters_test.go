package pipeline

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestParameterCatalogCoversExactlyTheCapabilities(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := loadParameterCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	infos, err := DescribeCapabilities(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != len(catalog.Capabilities) {
		t.Fatal("catalog contains orphaned/missing capabilities")
	}
	for _, info := range infos {
		if info.Description == "" || info.Execution.Input == "" || info.Execution.Output == "" || info.Parameters == nil {
			t.Fatalf("incomplete description: %+v", info)
		}
		for key, p := range info.Parameters {
			if p.Description == "" || !slices.Contains([]string{"string", "integer", "boolean", "integer-list", "json-string-array"}, p.Type) {
				t.Fatalf("invalid parameter %s.%s", info.ID, key)
			}
			if p.Default != nil {
				if err := validateParameter(p, *p.Default); err != nil {
					t.Fatalf("invalid default %s.%s: %v", info.ID, key, err)
				}
			}
		}
	}
	filtered, err := DescribeCapabilities(root, "epub.typography.optimize")
	if err != nil || len(filtered) != 1 || filtered[0].Parameters["scope_paths"].Type != "json-string-array" {
		t.Fatalf("filtered=%v err=%v", filtered, err)
	}
	if _, err := DescribeCapabilities(root, "no.such.capability"); err == nil {
		t.Fatal("unknown filter accepted")
	}
}

func TestInvalidParametersFailBeforeInputAccess(t *testing.T) {
	for _, args := range []Args{{"presett": "literary-cn"}, {"scope_paths": "[]"}, {"scope_paths": "null"}, {"scope_paths": "[1]"}, {"allow_font_obfuscation": "tru"}} {
		out, err := Run(t.Context(), Options{CapabilityID: "epub.typography.optimize", InputPath: "must-not-open.epub", DryRun: true, Args: args})
		if err == nil || out.ExitCode != ExitUsage || strings.Contains(err.Error(), "input not found") {
			t.Fatalf("args=%v exit=%d err=%v", args, out.ExitCode, err)
		}
	}
}

func TestPreviewNextCommandPreservesScopeAndQuotes(t *testing.T) {
	c := Contract{ID: "epub.typography.optimize"}
	c.Execution.Output = ExecOutputSingle
	opts := Options{DryRun: true, InputPath: "books/O'Brien $(ignored).epub", OutputPath: "my candidate.epub", Args: Args{"preset": "academic-cn", "scope_paths": `["OPS/Text/a.xhtml"]`}}
	got := nextCommands(c, opts, opts.Args, true)
	want := `epub run epub.typography.optimize --input 'books/O'"'"'Brien $(ignored).epub' --output 'my candidate.epub' --json preset=academic-cn 'scope_paths=["OPS/Text/a.xhtml"]'`
	if len(got) != 1 || got[0] != want {
		t.Fatalf("next command loses or expands scope: %q", got)
	}
}

func TestRegisterReadsOnlyCatalogParameters(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := loadParameterCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(root, "internal/pipeline/register.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	reserved := map[string]bool{
		"input": true, "output": true, "dry_run": true, "source_path": true,
		"demo_dir": true, "allow_font_obfuscation": true,
	}
	var failures []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		fn, ok := call.Fun.(*ast.Ident)
		if !ok || !strings.HasPrefix(fn.Name, "register") {
			return true
		}
		idLit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || idLit.Kind != token.STRING {
			return true
		}
		id, err := strconv.Unquote(idLit.Value)
		if err != nil {
			failures = append(failures, "invalid registration id literal: "+idLit.Value)
			return true
		}
		desc, ok := catalog.Capabilities[id]
		if !ok {
			failures = append(failures, id+": missing catalog capability")
			return true
		}
		ast.Inspect(call.Args[1], func(node ast.Node) bool {
			key := ""
			switch expr := node.(type) {
			case *ast.CallExpr:
				selector, ok := expr.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Get" && selector.Sel.Name != "Bool" || len(expr.Args) == 0 {
					return true
				}
				receiver, ok := selector.X.(*ast.Ident)
				if !ok || receiver.Name != "args" {
					return true
				}
				lit, ok := expr.Args[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				key, err = strconv.Unquote(lit.Value)
			case *ast.IndexExpr:
				receiver, ok := expr.X.(*ast.Ident)
				lit, litOK := expr.Index.(*ast.BasicLit)
				if !ok || receiver.Name != "args" || !litOK || lit.Kind != token.STRING {
					return true
				}
				key, err = strconv.Unquote(lit.Value)
			default:
				return true
			}
			if err != nil {
				failures = append(failures, id+": invalid parameter key literal")
				return true
			}
			if !reserved[key] {
				if _, ok := desc.Parameters[key]; !ok {
					if _, ok := catalog.CommonParameters[key]; !ok {
						failures = append(failures, id+" reads undeclared parameter "+key)
					}
				}
			}
			return true
		})
		return true
	})
	if len(failures) != 0 {
		t.Fatalf("registration reads outside cli.json:\n%s", strings.Join(failures, "\n"))
	}
}
