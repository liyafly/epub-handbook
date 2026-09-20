package split

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

func TestFontFallbackIsDeclarationScoped(t *testing.T) {
	cases := []struct {
		css      string
		optional bool
	}{
		{`@font-face{src:local("Songti SC"),url(missing.ttf)}`, true},
		{`@FONT-FACE{src:url(missing.ttf), LOCAL(Songti SC)}`, true},
		{`@font-face{src:local("Songti SC");src:url(missing.ttf)}`, false},
		{`@font-face{src:url(missing.ttf);font-family:local("Songti SC")}`, false},
		{`@font-face{src:url(missing.ttf),format(local("fake"))}`, false},
		{`@font-face{src:url(missing.ttf),local("")}`, false},
		{`@font-face{src:url(missing.ttf),local(" ")}`, false},
		{`@font-face{src:url(missing.ttf),local("Foo" Bar)}`, false},
		{`@font-face{src:url(missing.ttf),local ("Foo")}`, false},
		{`@font-face{src:url(missing.ttf) /* local("fake") */}`, false},
		{`@font-face{src:url(missing.ttf),"local('fake')"}`, false},
		{`p{background:url(missing.ttf);src:local("Songti SC")}`, false},
		{`@font-face{src:local("Songti SC"),url(missing.ttf)}p{background:url(missing.ttf)}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.css, func(t *testing.T) {
			refs, err := collectCSSReferences(tc.css)
			if err != nil || len(refs) == 0 {
				t.Fatalf("refs=%v err=%v", refs, err)
			}
			for _, ref := range refs {
				if ref.localFontFallback != tc.optional {
					t.Fatalf("%+v, want optional=%t", ref, tc.optional)
				}
			}
		})
	}
}

func TestSplitMissingFontWithLocalFallbackLogsWithoutWarning(t *testing.T) {
	for _, dryRun := range []bool{true, false} {
		t.Run(map[bool]string{true: "preview", false: "apply"}[dryRun], func(t *testing.T) {
			entries := buildSrcsetEntries(true, true)
			const fontCSS = `@font-face{font-family:Demo;src:local("Songti SC"),url(../Fonts/missing.ttf)}`
			for i := range entries {
				if entries[i].name == "OEBPS/Text/chapter.xhtml" {
					entries[i].content = bytes.Replace(entries[i].content, []byte("<body>"), []byte("<head><style>"+fontCSS+"</style></head><body>"), 1)
				}
			}
			dir := t.TempDir()
			input, output := filepath.Join(dir, "font.epub"), filepath.Join(dir, "out")
			buildEpub(t, input, entries)
			b, err := book.Open(input)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			res, err := Run(t.Context(), b, Params{SplitPoints: []int{0}, OutputDir: output, DryRun: dryRun})
			if err != nil || res.Status != report.StatusComplete || len(res.Findings) != 0 {
				t.Fatalf("result=%+v err=%v", res, err)
			}
			logged := false
			for _, event := range res.Events {
				if event.Step == "split.font-local-fallback" {
					logged = true
				}
			}
			if !logged {
				t.Fatal("fallback not recorded in event log")
			}
			if dryRun {
				if _, err := os.Stat(output); !os.IsNotExist(err) {
					t.Fatal("preview wrote output")
				}
			} else {
				written := readZipEntries(t, filepath.Join(output, "font_01.epub"))
				if !bytes.Contains(written["OEBPS/Text/chapter.xhtml"], []byte(fontCSS)) {
					t.Fatal("font declaration changed")
				}
			}
		})
	}
}
