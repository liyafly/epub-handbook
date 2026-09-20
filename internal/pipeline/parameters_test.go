package pipeline

import (
	"slices"
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
	got := nextCommands(c, opts, true)
	want := `epub run epub.typography.optimize --input 'books/O'"'"'Brien $(ignored).epub' --output 'my candidate.epub' --json 'preset=academic-cn' 'scope_paths=["OPS/Text/a.xhtml"]'`
	if len(got) != 1 || got[0] != want {
		t.Fatalf("next command loses or expands scope: %q", got)
	}
}
