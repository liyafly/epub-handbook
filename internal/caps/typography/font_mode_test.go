package typography

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
)

const (
	fontModeCSSPath  = "OEBPS/Styles/fonts.css"
	fontModeOPFPath  = "OEBPS/content.opf"
	fontModeTextPath = "OEBPS/Text/chapter.xhtml"
	fontModePrefix   = `ibooks: http://vocabulary.itunes.apple.com/rdf/ibooks/vocabulary-extensions-1.0/`
)

func fontModeFixture(classes, fontsCSS string, specifiedFonts *bool) map[string]string {
	files := typographyFixture(classes)
	files[fontModeCSSPath] = fontsCSS

	opf := strings.Replace(files[fontModeOPFPath], "  </manifest>",
		"    <item id=\"old-fonts\" href=\"Styles/fonts.css\" media-type=\"text/css\"/>\n  </manifest>", 1)
	if specifiedFonts != nil {
		opf = strings.Replace(opf, `unique-identifier="book-id">`,
			`unique-identifier="book-id" prefix="`+fontModePrefix+`">`, 1)
		value := "false"
		if *specifiedFonts {
			value = "true"
		}
		opf = strings.Replace(opf, "  </metadata>",
			"    <meta property=\"ibooks:specified-fonts\">"+value+"</meta>\n  </metadata>", 1)
	}
	files[fontModeOPFPath] = opf

	chapter := files[fontModeTextPath]
	baseLink := `<link rel="stylesheet" type="text/css" href="../Styles/base.css"/>`
	fontsLink := `<link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/>`
	files[fontModeTextPath] = strings.Replace(chapter, baseLink, baseLink+"\n    "+fontsLink, 1)
	return files
}

func runFontModePreset(t *testing.T, input, output, presetDir, preset string, dryRun bool) (*book.Book, error) {
	t.Helper()
	b, err := book.Open(input)
	if err != nil {
		t.Fatalf("book.Open: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	_, err = Run(t.Context(), b, Params{
		Preset: preset, PresetDir: presetDir, Output: output, DryRun: dryRun,
	})
	return b, err
}

func applyFontModePreset(t *testing.T, input, presetDir, preset string) (*book.Book, map[string][]byte) {
	t.Helper()
	output := filepath.Join(t.TempDir(), "output.epub")
	b, err := runFontModePreset(t, input, output, presetDir, preset, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := b.WriteTo(output); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return b, readZipData(t, output)
}

func TestWholeBookPresetPreservesLockedFontsLayerAndMetadata(t *testing.T) {
	presets := filepath.Join(repoRootDir(t), "templates", "style-presets")
	dir := t.TempDir()
	input := filepath.Join(dir, "locked.epub")
	output := filepath.Join(dir, "output.epub")
	locked := true
	fonts := []byte(`@charset "utf-8";
body { font-family: "Book Serif", "Fallback Serif", serif; }
.font-quote { font-family: "Quote Serif", serif; }
`)
	buildFixtureEpub(t, input, fontModeFixture("chapter-head", string(fonts), &locked))

	b, err := runFontModePreset(t, input, output, presets, "literary-cn", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := b.WriteTo(output); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	files := readZipData(t, output)
	if !bytes.Equal(files[fontModeCSSPath], fonts) {
		t.Fatalf("fonts.css changed\n got: %q\nwant: %q", files[fontModeCSSPath], fonts)
	}
	opf := string(files[fontModeOPFPath])
	if count := strings.Count(opf, `<meta property="ibooks:specified-fonts">true</meta>`); count != 1 {
		t.Fatalf("specified-fonts meta count=%d, want one preserved true meta:\n%s", count, opf)
	}
	if !strings.Contains(opf, `prefix="`+fontModePrefix+`"`) {
		t.Fatalf("OPF lost the ibooks prefix declaration:\n%s", opf)
	}
	if !strings.Contains(string(files[fontModeTextPath]), `href="../Styles/fonts.css"`) {
		t.Fatalf("output chapter lost the fonts.css link:\n%s", files[fontModeTextPath])
	}
}

func TestWholeBookPresetKeepsFreeRoleFontsWithoutLockingBody(t *testing.T) {
	presets := filepath.Join(repoRootDir(t), "templates", "style-presets")
	dir := t.TempDir()
	input := filepath.Join(dir, "free.epub")
	fonts := []byte(`@charset "utf-8";
.font-st, .st { font-family: "Book Serif", serif; }
.font-quote { font-family: "Quote Serif", serif; }
`)
	buildFixtureEpub(t, input, fontModeFixture("chapter-head", string(fonts), nil))

	_, files := applyFontModePreset(t, input, presets, "literary-cn")
	if !bytes.Equal(files[fontModeCSSPath], fonts) {
		t.Fatalf("role-only fonts.css changed\n got: %q\nwant: %q", files[fontModeCSSPath], fonts)
	}
	opf := string(files[fontModeOPFPath])
	if strings.Contains(opf, "ibooks:specified-fonts") || strings.Contains(opf, "ibooks:") {
		t.Fatalf("free-mode output gained ibooks font-lock metadata:\n%s", opf)
	}
	for _, path := range []string{fontModeCSSPath, "OEBPS/Styles/base.css"} {
		direct, legacy, err := bodyBindings(files[path], nil)
		if err != nil {
			t.Fatalf("bodyBindings(%s): %v", path, err)
		}
		if direct || legacy {
			t.Fatalf("free-mode output locked body via %s (direct=%v legacy=%v)", path, direct, legacy)
		}
	}
}

func TestWholeBookPresetPreservesLegacyLockedBodyMode(t *testing.T) {
	presets := filepath.Join(repoRootDir(t), "templates", "style-presets")
	dir := t.TempDir()
	input := filepath.Join(dir, "legacy-locked.epub")
	output := filepath.Join(dir, "output.epub")
	locked := true
	fonts := []byte(`@charset "utf-8";
.body-font-locked { font-family: "Legacy Serif", serif; }
.font-st { font-family: "Role Serif", serif; }
`)
	buildFixtureEpub(t, input, fontModeFixture("body-font-locked", string(fonts), &locked))

	b, err := runFontModePreset(t, input, output, presets, "literary-cn", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := b.WriteTo(output); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	files := readZipData(t, output)
	if !bytes.Equal(files[fontModeCSSPath], fonts) {
		t.Fatalf("legacy fonts.css changed\n got: %q\nwant: %q", files[fontModeCSSPath], fonts)
	}
	if !strings.Contains(string(files[fontModeOPFPath]), `<meta property="ibooks:specified-fonts">true</meta>`) {
		t.Fatalf("legacy locked mode lost its OPF meta:\n%s", files[fontModeOPFPath])
	}
	if !strings.Contains(string(files[fontModeTextPath]), `body class="body-font-locked"`) {
		t.Fatalf("legacy body-font-locked class was not retained:\n%s", files[fontModeTextPath])
	}
}

func TestWholeBookPresetRejectsFontMetadataBodyModeMismatchWithoutEdits(t *testing.T) {
	presets := filepath.Join(repoRootDir(t), "templates", "style-presets")
	dir := t.TempDir()
	input := filepath.Join(dir, "mismatch.epub")
	output := filepath.Join(dir, "dry-run.epub")
	fonts := `body { font-family: "Book Serif", serif; }
.font-st { font-family: "Book Serif", serif; }
`
	buildFixtureEpub(t, input, fontModeFixture("", fonts, nil))

	b, err := runFontModePreset(t, input, output, presets, "literary-cn", true)
	if err == nil {
		t.Fatal("Run accepted locked body CSS without matching specified-fonts metadata")
	}
	if names := b.ModifiedNames(); len(names) != 0 {
		t.Fatalf("Run modified entries before rejecting mismatch: %v", names)
	}
}

func TestWholeBookPresetRejectsBodyFontOutsideFontsLayerWithoutEdits(t *testing.T) {
	presets := filepath.Join(repoRootDir(t), "templates", "style-presets")
	dir := t.TempDir()
	input := filepath.Join(dir, "mislayered.epub")
	output := filepath.Join(dir, "dry-run.epub")
	locked := true
	files := fontModeFixture("", `.font-st { font-family: "Book Serif", serif; }`, &locked)
	files["OEBPS/Styles/base.css"] += `body { font-family: "Book Serif", serif; }` + "\n"
	buildFixtureEpub(t, input, files)

	b, err := runFontModePreset(t, input, output, presets, "literary-cn", true)
	if err == nil {
		t.Fatal("Run accepted the direct body font binding in base.css")
	}
	if names := b.ModifiedNames(); len(names) != 0 {
		t.Fatalf("Run modified entries before rejecting misplaced body font: %v", names)
	}
}

func TestWholeBookPresetRejectsCustomBodyFontInBaseLayerWithoutEdits(t *testing.T) {
	root := repoRootDir(t)
	sourcePreset := filepath.Join(root, "templates", "style-presets", "literary-cn", "Styles")
	presetRoot := t.TempDir()
	presetName := "body-font-override"
	stylesDir := filepath.Join(presetRoot, presetName, "Styles")
	if err := os.MkdirAll(stylesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	layers := []string{"fonts.css", "base.css", "notes.css", "effects.css", "literary.css", "media.css"}
	metadata := fmt.Sprintf(`{"name":%q,"version":"1","layers":["fonts.css","base.css","notes.css","effects.css","literary.css","media.css"],"notes":"test"}`, presetName)
	if err := os.WriteFile(filepath.Join(presetRoot, presetName, "preset.json"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, layer := range layers {
		data, err := os.ReadFile(filepath.Join(sourcePreset, layer))
		if err != nil {
			t.Fatal(err)
		}
		if layer == "base.css" {
			data = append(data, []byte("\nbody { font-family: \"Override Serif\", serif; }\n")...)
		}
		if err := os.WriteFile(filepath.Join(stylesDir, layer), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	dir := t.TempDir()
	input := filepath.Join(dir, "free.epub")
	output := filepath.Join(dir, "dry-run.epub")
	fonts := `.font-st { font-family: "Book Serif", serif; }` + "\n"
	buildFixtureEpub(t, input, fontModeFixture("", fonts, nil))
	b, err := runFontModePreset(t, input, output, presetRoot, presetName, true)
	if err == nil {
		t.Fatal("Run accepted a custom base.css body font binding")
	}
	if names := b.ModifiedNames(); len(names) != 0 {
		t.Fatalf("Run modified entries before rejecting custom body font: %v", names)
	}
}

func TestWholeBookPresetRefusesUnsupportedFontCascade(t *testing.T) {
	presets := filepath.Join(repoRootDir(t), "templates", "style-presets")
	for _, tc := range []struct{ name, fonts, markup string }{
		{"variable", `body{font-family:var(--body-font)}`, ""},
		{"reset", `body{font-family:serif} body{all:initial}`, ""},
		{"inline", `.role{font-family:serif}`, `<body style="font-family:serif"`},
		{"embedded", `.role{font-family:serif}`, `<style>body{font-family:serif}</style></head>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := fontModeFixture("", tc.fonts, new(true))
			if tc.name == "inline" {
				files[fontModeTextPath] = strings.Replace(files[fontModeTextPath], "<body", tc.markup, 1)
			}
			if tc.name == "embedded" {
				files[fontModeTextPath] = strings.Replace(files[fontModeTextPath], "</head>", tc.markup, 1)
			}
			input := filepath.Join(t.TempDir(), "input.epub")
			buildFixtureEpub(t, input, files)
			b, err := runFontModePreset(t, input, filepath.Join(t.TempDir(), "output.epub"), presets, "literary-cn", true)
			if err == nil || len(b.ModifiedNames()) != 0 {
				t.Fatalf("unsupported font cascade changed book: %v, %v", b.ModifiedNames(), err)
			}
		})
	}
}

func twoChapterFontModeFixture(classes, fontsCSS string) map[string]string {
	files := fontModeFixture(classes, fontsCSS, nil)
	opf := files[fontModeOPFPath]
	opf = strings.Replace(opf,
		`    <item id="old-base"`,
		`    <item id="chapter2" href="Text/chapter2.xhtml" media-type="application/xhtml+xml"/>`+"\n"+`    <item id="old-base"`, 1)
	opf = strings.Replace(opf, `<spine><itemref idref="chapter"/></spine>`,
		`<spine><itemref idref="chapter"/><itemref idref="chapter2"/></spine>`, 1)
	files[fontModeOPFPath] = opf
	files["OEBPS/Text/chapter2.xhtml"] = files[fontModeTextPath]
	return files
}

func TestWholeBookPresetRefusesNoncanonicalBodyChains(t *testing.T) {
	presets := filepath.Join(repoRootDir(t), "templates", "style-presets")
	for _, tc := range []struct {
		name, classes, selector string
	}{
		{"book-class", "calibre", ".calibre"},
		{"html-body", "", "html body"},
		{"body-class", "chapter", "body.chapter"},
		{"body-paragraph", "", "body p"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := twoChapterFontModeFixture(tc.classes, `.font-st { font-family: serif; }`)
			files["OEBPS/Styles/base.css"] = tc.selector + ` { font-family: "Book Serif", serif; }` + "\n"
			input := filepath.Join(t.TempDir(), "input.epub")
			buildFixtureEpub(t, input, files)
			b, err := runFontModePreset(t, input, filepath.Join(t.TempDir(), "output.epub"), presets, "literary-cn", true)
			if err == nil {
				t.Fatal("Run accepted a noncanonical whole-book body font binding")
			}
			if names := b.ModifiedNames(); len(names) != 0 {
				t.Fatalf("Run modified entries before rejecting binding: %v", names)
			}
		})
	}
}

func TestWholeBookPresetAllowsPageRoleBodyClass(t *testing.T) {
	presets := filepath.Join(repoRootDir(t), "templates", "style-presets")
	files := twoChapterFontModeFixture("preface", `body.preface { font-family: serif; }`)
	files["OEBPS/Text/chapter2.xhtml"] = strings.Replace(files["OEBPS/Text/chapter2.xhtml"], `class="preface"`, `class="chapter"`, 1)
	input := filepath.Join(t.TempDir(), "input.epub")
	buildFixtureEpub(t, input, files)
	b, err := runFontModePreset(t, input, filepath.Join(t.TempDir(), "output.epub"), presets, "literary-cn", true)
	if err != nil {
		t.Fatalf("Run rejected a page-role body class: %v", err)
	}
	if len(b.ModifiedNames()) == 0 {
		t.Fatal("Run made no preset edits")
	}
}

func TestPresetAcceptsHTMLNamedEntities(t *testing.T) {
	presets := filepath.Join(repoRootDir(t), "templates", "style-presets")
	for _, scoped := range []bool{false, true} {
		name := "whole-book"
		if scoped {
			name = "scoped"
		}
		t.Run(name, func(t *testing.T) {
			files := fontModeFixture("", `.font-st { font-family: serif; }`, nil)
			files[fontModeTextPath] = strings.Replace(files[fontModeTextPath], "正文保持不变。", "正文&nbsp;保持不变。", 1)
			input := filepath.Join(t.TempDir(), "input.epub")
			buildFixtureEpub(t, input, files)
			var b *book.Book
			var err error
			if scoped {
				b, err = book.Open(input)
				if err == nil {
					t.Cleanup(func() { _ = b.Close() })
					_, err = Run(t.Context(), b, Params{Preset: "literary-cn", PresetDir: presets, DryRun: true, ScopePaths: []string{fontModeTextPath}})
				}
			} else {
				b, err = runFontModePreset(t, input, filepath.Join(t.TempDir(), "output.epub"), presets, "literary-cn", true)
			}
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			got, err := b.Current(fontModeTextPath)
			if err != nil || !bytes.Contains(got, []byte("&nbsp;")) {
				t.Fatalf("named entity bytes changed or disappeared: %q (%v)", got, err)
			}
		})
	}
}
