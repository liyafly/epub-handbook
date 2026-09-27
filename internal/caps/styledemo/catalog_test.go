package styledemo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

func TestCatalogMatchesSourceAndArtifactWithoutClaimingAcceptance(t *testing.T) {
	repo := repoRoot(t)
	p := Params{DemoDir: demoDirOf(repo), Catalog: true}
	source, err := Run(t.Context(), nil, p)
	if err != nil {
		t.Fatal(err)
	}
	scenes := source.Facts["scenes"].([]report.StyleScene)
	if len(scenes) < 24 {
		t.Fatalf("missing active scenes: %d", len(scenes))
	}
	for _, scene := range scenes {
		if scene.Title == "" || len(scene.SHA256) != 64 || len(scene.Stylesheets) == 0 || len(scene.StylesheetSHA256) != len(scene.Stylesheets) {
			t.Fatalf("incomplete scene %+v", scene)
		}
		for _, stylesheet := range scene.Stylesheets {
			if len(scene.StylesheetSHA256[stylesheet]) != 64 {
				t.Fatalf("stylesheet hash missing for %s: %+v", stylesheet, scene.StylesheetSHA256)
			}
		}
	}
	artifact := filepath.Join(t.TempDir(), "demo.epub")
	buildDemoEpub(t, repo, artifact)
	b, err := book.Open(artifact)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	result, err := Run(t.Context(), b, p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scenes, result.Facts["scenes"]) {
		t.Fatal("source and artifact catalogs differ")
	}
	if result.Facts["readerStatus"] != "not-verified" || result.Facts["previewStatus"] != "not-rendered" {
		t.Fatal("catalog claims acceptance")
	}
	p.Query = "28-chapter-opening"
	filtered, err := Run(t.Context(), b, p)
	if err != nil || filtered.Facts["sceneCount"] != 1 {
		t.Fatalf("filter=%v %v", filtered.Facts, err)
	}
	p.Query = "definitely-no-such-scene"
	empty, err := Run(t.Context(), b, p)
	if err != nil || empty.Facts["sceneCount"] != 0 || empty.Facts["scenes"] == nil {
		t.Fatalf("no match=%v %v", empty.Facts, err)
	}
}

func TestPresetCatalogReadsMetadataFiltersAndReportsUntestedStatus(t *testing.T) {
	root := t.TempDir()
	presetDir := filepath.Join(root, "fiction-en")
	if err := os.Mkdir(presetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := `{"name":"fiction-en","version":"1","description":"English novel layout","layers":["fonts.css","base.css","literary.css"],"notes":"Reader testing required"}`
	if err := os.WriteFile(filepath.Join(presetDir, "preset.json"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Run(t.Context(), nil, Params{Catalog: true, Collection: "presets", PresetDir: root})
	if err != nil {
		t.Fatal(err)
	}
	presets := result.Facts["presets"].([]report.StylePreset)
	if len(presets) != 1 || presets[0].ID != "fiction-en" || presets[0].Source != "filesystem" || presets[0].ReaderStatus != "not-verified" {
		t.Fatalf("preset catalog = %+v", presets)
	}
	if !reflect.DeepEqual(presets[0].Layers, []string{"fonts.css", "base.css", "literary.css"}) || presets[0].Notes != "Reader testing required" {
		t.Fatalf("preset metadata = %+v", presets[0])
	}
	result, err = Run(t.Context(), nil, Params{Catalog: true, Collection: "presets", PresetDir: root, Query: "no-match"})
	if err != nil || result.Facts["presetCount"] != 0 || result.Facts["presets"] == nil {
		t.Fatalf("empty preset query facts=%v err=%v", result.Facts, err)
	}
}

func TestCatalogRejectsCollectionWithoutCatalog(t *testing.T) {
	if _, err := Run(t.Context(), nil, Params{Collection: "presets"}); err == nil {
		t.Fatal("collection without catalog=true was accepted")
	}
}

func TestCatalogRejectsSourceTreeEscapeAndCancellation(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "META-INF"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "META-INF/container.xml"), []byte(`<container><rootfiles><rootfile full-path="../outside.opf"/></rootfiles></container>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(t.Context(), nil, Params{DemoDir: dir, Catalog: true}); err == nil {
		t.Fatal("path traversal accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Run(ctx, nil, Params{DemoDir: dir, Catalog: true}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
}

func TestCatalogAcceptsHTMLNamedEntities(t *testing.T) {
	files := map[string][]byte{
		"META-INF/container.xml":   []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/book.opf"/></rootfiles></container>`),
		"OEBPS/book.opf":           []byte(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata/><manifest><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/><item id="style" href="Styles/base.css" media-type="text/css"/></manifest><spine><itemref idref="chapter"/></spine></package>`),
		"OEBPS/Text/chapter.xhtml": []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter</title><link rel="stylesheet" href="../Styles/base.css"/><link rel="stylesheet" href="../Styles/base.css"/></head><body>正文&nbsp;保持</body></html>`),
		"OEBPS/Styles/base.css":    []byte(`body { color: black; }`),
	}
	reads := map[string]int{}
	result, findings, err := scanScenes(t.Context(), func(name string) ([]byte, error) {
		reads[name]++
		data, ok := files[name]
		if !ok {
			return nil, os.ErrNotExist
		}
		return data, nil
	}, "")
	if err != nil {
		t.Fatalf("scanScenes: %v", err)
	}
	if len(result) != 1 || result[0].Title != "Chapter" || len(result[0].Stylesheets) != 1 || len(findings) != 0 {
		t.Fatalf("catalog result = %+v, findings=%+v", result, findings)
	}
	if reads["OEBPS/Styles/base.css"] != 1 {
		t.Fatalf("duplicate stylesheet was read %d times", reads["OEBPS/Styles/base.css"])
	}
	originalHash := result[0].StylesheetSHA256["OEBPS/Styles/base.css"]
	files["OEBPS/Styles/base.css"] = []byte(`body { color: navy; }`)
	updated, _, err := scanScenes(t.Context(), func(name string) ([]byte, error) {
		data, ok := files[name]
		if !ok {
			return nil, os.ErrNotExist
		}
		return data, nil
	}, "")
	if err != nil || len(updated) != 1 || updated[0].StylesheetSHA256["OEBPS/Styles/base.css"] == originalHash {
		t.Fatalf("stylesheet source change did not update provenance hash: scenes=%+v err=%v", updated, err)
	}
}

func TestCatalogReportsDanglingStylesheet(t *testing.T) {
	files := map[string][]byte{
		"META-INF/container.xml":   []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/book.opf"/></rootfiles></container>`),
		"OEBPS/book.opf":           []byte(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata/><manifest><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="chapter"/><itemref idref="missing"/></spine></package>`),
		"OEBPS/Text/chapter.xhtml": []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter</title><link rel="stylesheet" href="../Styles/missing.css"/></head><body>正文</body></html>`),
	}
	dir := t.TempDir()
	for name, data := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Run(t.Context(), nil, Params{DemoDir: dir, Catalog: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	scenes := result.Facts["scenes"].([]report.StyleScene)
	findings := result.Findings
	if len(scenes) != 1 || len(scenes[0].Stylesheets) != 0 {
		t.Fatalf("catalog scenes = %+v", scenes)
	}
	var stylesheet, spine bool
	for _, finding := range findings {
		switch finding.ID {
		case "styledemo.catalog-missing-stylesheet":
			stylesheet = finding.Level == "warn" && finding.Title == "OEBPS/Styles/missing.css" && finding.Location == "OEBPS/Text/chapter.xhtml"
		case "styledemo.catalog-missing-spine-item":
			spine = finding.Level == "warn" && finding.Title == "missing" && finding.Location == "OEBPS/book.opf"
		}
	}
	if !stylesheet || !spine {
		t.Fatalf("missing dangling-reference findings: %+v", findings)
	}
}
