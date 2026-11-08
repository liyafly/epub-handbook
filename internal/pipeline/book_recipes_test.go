package pipeline

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBookRecipesListShowAndAdd(t *testing.T) {
	h := newBookHarness(t)
	if code := BookRecipes(BookRecipesOptions{}, h.io()); code != 0 {
		t.Fatalf("list = %d: %s", code, h.err)
	}
	for _, want := range []string{"shape-wrap", "drop-cap", "text-emphasis", "theme-adaptive-art", "parallel-pair", "vertical-title-art", "image-preview"} {
		if !strings.Contains(h.out.String(), want) {
			t.Errorf("list should include %s: %s", want, h.out)
		}
	}
	if strings.Contains(h.out.String(), "reader-evidence") || strings.Contains(h.out.String(), "dist ") {
		t.Errorf("non-recipe directories must not be listed: %s", h.out)
	}

	if code := BookRecipes(BookRecipesOptions{Name: "drop-cap"}, h.io()); code != 0 || !strings.Contains(h.out.String(), ".recipe-opening") {
		t.Fatalf("show = %d: %s %s", code, h.out, h.err)
	}
	if code := BookRecipes(BookRecipesOptions{Name: "no-such"}, h.io()); code != ExitUsage {
		t.Errorf("unknown recipe = %d, want usage", code)
	}
	if code := BookRecipes(BookRecipesOptions{Name: "../x"}, h.io()); code != ExitUsage {
		t.Errorf("path-like recipe name = %d, want usage", code)
	}

	dir := h.newBook("")
	design := filepath.Join(dir, "epub", "OEBPS", "Styles", "design.css")
	before := mustRead(t, design)
	if code := BookRecipes(BookRecipesOptions{Name: "drop-cap", Add: dir}, h.io()); code != 0 {
		t.Fatalf("add = %d: %s", code, h.err)
	}
	after := mustRead(t, design)
	if !strings.HasPrefix(after, before) || !strings.Contains(after, "/* recipe: drop-cap ") || !strings.Contains(after, ".recipe-opening") {
		t.Errorf("add must append a marked block and keep existing CSS:\n%s", after)
	}
	if code := BookRecipes(BookRecipesOptions{Name: "drop-cap", Add: dir}, h.io()); code != ExitUsage || mustRead(t, design) != after {
		t.Errorf("a second add must be refused without changes: %d %s", code, h.err)
	}
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build after add = %d: %s", code, h.err)
	}
}
