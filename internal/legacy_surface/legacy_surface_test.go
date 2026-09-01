package legacy_surface

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestProductionGoHasNoLegacyExecutionSurface keeps the deleted Python/shell
// execution surface out of user-visible Go strings. It deliberately parses
// only production .go files and string literals: test fixtures and historical
// oracle comments are not part of the runtime contract.
func TestProductionGoHasNoLegacyExecutionSurface(t *testing.T) {
	root := repositoryRoot(t)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bpython3?\s+scripts/[[:alnum:]_.-]+\.py\b`),
		regexp.MustCompile(`(?i)\bscripts/[[:alnum:]_.-]+\.sh\b`),
	}

	var violations []string
	for _, path := range productionGoFiles(t, root) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatalf("unquote %s: %v", path, err)
			}
			for _, pattern := range patterns {
				if pattern.MatchString(value) {
					pos := fset.Position(literal.Pos())
					violations = append(violations, pos.String()+": "+value)
					break
				}
			}
			return true
		})
	}

	if len(violations) > 0 {
		slices.Sort(violations)
		t.Fatalf("production Go contains deleted execution-surface command strings:\n%s", strings.Join(violations, "\n"))
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func productionGoFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			switch filepath.ToSlash(rel) {
			case ".git", "testdata", "internal/archguard":
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk production Go files: %v", err)
	}
	slices.Sort(files)
	return files
}
