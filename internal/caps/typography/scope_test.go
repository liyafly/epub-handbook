package typography

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/redline"
)

func TestScopedPresetPreservesUnselectedAndIsIdempotent(t *testing.T) {
	files := typographyFixture("font-st chapter-head")
	const selected = "OEBPS/Text/chapter.xhtml"
	const untouched = "OEBPS/Text/other.xhtml"
	files[untouched] = files[selected]
	files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"], "</manifest>", `<item id="other" href="Text/other.xhtml" media-type="application/xhtml+xml"/></manifest>`, 1)
	files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"], "</spine>", `<itemref idref="other"/></spine>`, 1)
	input := filepath.Join(t.TempDir(), "scope.epub")
	buildFixtureEpub(t, input, files)
	b, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	p := Params{Preset: "literary-cn", PresetDir: filepath.Join(repoRootDir(t), "templates/style-presets"), DryRun: true, ScopePaths: []string{selected}}
	res, err := Run(t.Context(), b, p)
	if err != nil {
		t.Fatal(err)
	}
	if res.Facts["applicationMode"] != "scoped-additive" {
		t.Fatal(res.Facts)
	}
	for name, original := range files {
		if name == selected || name == "OEBPS/content.opf" {
			continue
		}
		got, err := b.Current(name)
		if err != nil || !bytes.Equal(got, []byte(original)) {
			t.Fatalf("unselected resource changed: %s (%v)", name, err)
		}
	}
	got, _ := b.Current(selected)
	if !bytes.Contains(got, []byte(`href="../Styles/base.css"`)) || !bytes.Contains(got, []byte("epub-preset-")) {
		t.Fatal("original or scoped links missing")
	}
	findings, err := redline.Check(redline.OriginalState(b), redline.CurrentState(b), []string{"text", "metadata", "spine", "cover", "drm", "anchors"}, redline.Options{})
	if err != nil || len(findings) != 0 {
		t.Fatalf("redlines: %v %v", findings, err)
	}
	names := slices.Clone(b.Names())
	opfBefore, _ := b.Current("OEBPS/content.opf")
	if _, err := Run(t.Context(), b, p); err != nil {
		t.Fatal(err)
	}
	again, _ := b.Current(selected)
	opfAfter, _ := b.Current("OEBPS/content.opf")
	if !slices.Equal(names, b.Names()) || !bytes.Equal(got, again) || !bytes.Equal(opfBefore, opfAfter) {
		t.Fatal("reapply is not idempotent")
	}
	// dry-run also leaves the actual input archive unchanged.
	check, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	unchanged, _ := check.Current(selected)
	if string(unchanged) != files[selected] {
		t.Fatal("dry-run wrote input")
	}
}

func TestScopedInputValidation(t *testing.T) {
	for _, scope := range [][]string{{}, {"../chapter.xhtml"}, {"missing.xhtml"}} {
		if _, err := selectScope([]string{"chapter.xhtml"}, scope); err == nil {
			t.Fatalf("accepted %q", scope)
		}
	}
	for _, text := range []string{`@import "a.css";`, `p{background:url(a.png)}`, `p{color:red`} {
		if err := validateScopedCSS([]byte(text)); err == nil {
			t.Fatalf("accepted CSS %q", text)
		}
	}
	text := `<html xmlns="http://www.w3.org/1999/xhtml"><head><!-- </head> --><title>A</title></head><body>正文</body></html>`
	got, err := appendStylesheetLinks(text, "Text/a.xhtml", []string{"Styles/new.css"})
	if err != nil || !strings.Contains(got, `<!-- </head> --><title>A</title><link`) {
		t.Fatalf("minified insertion: %s %v", got, err)
	}
}

func TestPresetHandlesPercentEncodedSpineHref(t *testing.T) {
	const raw = "OEBPS/Text/chapter one.xhtml"
	const encoded = "OEBPS/Text/chapter%20one.xhtml"
	files := typographyFixture("")
	files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"], "Text/chapter.xhtml", "Text/chapter%20one.xhtml", 1)
	files[raw] = files["OEBPS/Text/chapter.xhtml"]
	delete(files, "OEBPS/Text/chapter.xhtml")
	input := filepath.Join(t.TempDir(), "encoded.epub")
	buildFixtureEpub(t, input, files)
	presets := filepath.Join(repoRootDir(t), "templates/style-presets")

	for _, tc := range []struct {
		name  string
		scope []string
		want  bool
	}{
		{name: "whole-book", want: true},
		{name: "scope-raw-path", scope: []string{raw}, want: true},
		{name: "scope-uri-spelling", scope: []string{encoded}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := book.Open(input)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			_, err = Run(t.Context(), b, Params{Preset: "literary-cn", PresetDir: presets, DryRun: true, ScopePaths: tc.scope})
			if (err == nil) != tc.want {
				t.Fatalf("Run error = %v, want success=%v", err, tc.want)
			}
			if !tc.want {
				if len(b.ModifiedNames()) != 0 {
					t.Fatalf("rejected scope modified entries: %v", b.ModifiedNames())
				}
				return
			}
			chapter, err := b.Current(raw)
			wantLink := []byte("epub-preset-")
			if tc.name == "whole-book" {
				wantLink = []byte(`href="../Styles/literary.css"`)
			}
			if err != nil || !bytes.Contains(chapter, wantLink) {
				t.Fatalf("preset link missing from decoded spine path: %q (%v)", chapter, err)
			}
		})
	}
}

func TestScopedPresetRejectsBodyFontLock(t *testing.T) {
	presetRoot := t.TempDir()
	name := "scoped-body-lock"
	stylesDir := filepath.Join(presetRoot, name, "Styles")
	if err := os.MkdirAll(stylesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := `{"name":"scoped-body-lock","version":"1","layers":["fonts.css"],"notes":"test"}`
	if err := os.WriteFile(filepath.Join(presetRoot, name, "preset.json"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stylesDir, "fonts.css"), []byte(`body{font-family:"Locked Serif",serif}`), 0o644); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "input.epub")
	buildFixtureEpub(t, input, typographyFixture(""))
	b, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	_, err = Run(t.Context(), b, Params{Preset: name, PresetDir: presetRoot, DryRun: true, ScopePaths: []string{"OEBPS/Text/chapter.xhtml"}})
	if err == nil {
		t.Fatal("scoped preset accepted a book-level body font binding")
	}
	if names := b.ModifiedNames(); len(names) != 0 {
		t.Fatalf("rejected scoped preset modified entries: %v", names)
	}
}

func TestScopedCollisionDoesNotApplyPartialEdits(t *testing.T) {
	presets := filepath.Join(repoRootDir(t), "templates/style-presets")
	data, err := os.ReadFile(filepath.Join(presets, "literary-cn/Styles/base.css"))
	if err != nil {
		t.Fatal(err)
	}
	files := typographyFixture("font-st")
	files[scopedStylesheetPath("OEBPS/Styles", "base.css", data)] = "unrelated user data"
	input := filepath.Join(t.TempDir(), "collision.epub")
	buildFixtureEpub(t, input, files)
	b, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	_, err = Run(t.Context(), b, Params{Preset: "literary-cn", PresetDir: presets, DryRun: true, ScopePaths: []string{"OEBPS/Text/chapter.xhtml"}})
	if err == nil || len(b.ModifiedNames()) != 0 {
		t.Fatalf("collision: %v modified=%v", err, b.ModifiedNames())
	}
}
