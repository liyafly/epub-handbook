package typography

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/redline"
)

func TestScopedReplacementPrunesOnlyUnreferencedLayers(t *testing.T) {
	for _, reference := range []string{"orphan", "other-page-reference", "css-import"} {
		t.Run(reference, func(t *testing.T) {
			shared := reference != "orphan"
			files := typographyFixture("")
			const chapter = "OEBPS/Text/chapter.xhtml"
			const other = "OEBPS/Text/other.xhtml"
			files[other] = files[chapter]
			files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"], "</manifest>", `<item id="other" href="Text/other.xhtml" media-type="application/xhtml+xml"/></manifest>`, 1)
			if reference == "css-import" {
				files["OEBPS/Styles/importer.css"] = "/* importer */"
				files["OEBPS/content.opf"] = strings.Replace(files["OEBPS/content.opf"], "</manifest>", `<item id="importer" href="Styles/importer.css" media-type="text/css"/></manifest>`, 1)
			}
			input := filepath.Join(t.TempDir(), "input.epub")
			buildFixtureEpub(t, input, files)
			b, err := book.Open(input)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			presets := t.TempDir()
			styles := filepath.Join(presets, "sample", "Styles")
			if err := os.MkdirAll(styles, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(presets, "sample", "preset.json"), []byte(`{"name":"sample","version":"1","layers":["media.css"]}`), 0644); err != nil {
				t.Fatal(err)
			}
			cssFile := filepath.Join(styles, "media.css")
			if err := os.WriteFile(cssFile, []byte(`p { margin: 0; }`), 0644); err != nil {
				t.Fatal(err)
			}
			p := Params{Preset: "sample", PresetDir: presets, ScopePaths: []string{chapter}, DryRun: true}
			if _, err := Run(t.Context(), b, p); err != nil {
				t.Fatal(err)
			}
			old := ""
			for _, name := range b.Names() {
				if strings.Contains(name, "epub-preset-") {
					old = name
				}
			}
			if old == "" {
				t.Fatal("no first layer")
			}
			if reference == "other-page-reference" {
				text := strings.Replace(files[other], "</head>", `<link rel="stylesheet" href="../Styles/`+filepath.Base(old)+`"/></head>`, 1)
				if err := b.Apply([]editset.Edit{editset.Replace(other, 0, int64(len(files[other])), []byte(text))}); err != nil {
					t.Fatal(err)
				}
			}
			if reference == "css-import" {
				if err := b.Apply([]editset.Edit{editset.Replace("OEBPS/Styles/importer.css", 0, int64(len("/* importer */")), []byte(`@import "`+filepath.Base(old)+`";`))}); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(cssFile, []byte(`p { margin: 1em; }`), 0644); err != nil {
				t.Fatal(err)
			}
			res, err := Run(t.Context(), b, p)
			if err != nil {
				t.Fatal(err)
			}
			page, _ := b.Current(chapter)
			if strings.Count(string(page), "epub-preset-") != 1 {
				t.Fatalf("accumulated links: %s", page)
			}
			if b.Has(old) != shared {
				t.Fatalf("old layer retained=%v, shared=%v", b.Has(old), shared)
			}
			removed := res.Facts["removedFiles"].([]string)
			if slices.Contains(removed, old) == shared {
				t.Fatalf("removedFiles=%v", removed)
			}
			if findings, err := redline.Check(redline.OriginalState(b), redline.CurrentState(b), []string{"text", "metadata", "spine", "anchors", "cover", "drm"}, redline.Options{}); err != nil || len(findings) != 0 {
				t.Fatalf("redlines=%v err=%v", findings, err)
			}
		})
	}
}
