package navaudit

import (
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
)

func TestSelectorTargetsReaderScale(t *testing.T) {
	cases := []struct {
		selector   string
		root, body bool
	}{
		{"html", true, false},
		{"HTML", true, false},
		{":root", true, false},
		{"html:root", true, false},
		{"html, body, div, p", true, true},
		{"/* reset */ body", false, true},
		{"body.colophon", false, true},
		{"html body", false, true},
		{"body p", false, false},
		{"html > body > p", false, false},
		{".html", false, false},
		{"htmlish", false, false},
		{"p:not(:root)", false, false},
		{"div[data-x=\"html, body\"]", false, false},
		{"bodytext", false, false},
		{"body.fullpage", false, false},
		{"body.fullpage.poster-bg", false, false},
		{"body.fullpaged", false, true},
		{"*", true, false},
		{"*, p", true, false},
		{"body *", false, false},
		{"*::before", false, false},
	}
	for _, tc := range cases {
		root, body := selectorTargetsReaderScale(tc.selector)
		if root != tc.root || body != tc.body {
			t.Errorf("%q: got root=%v body=%v, want root=%v body=%v", tc.selector, root, body, tc.root, tc.body)
		}
	}
}

func TestReaderFontScalingFindings(t *testing.T) {
	path := writeNativeFixture(t)
	input := filepath.Join(t.TempDir(), "root-font-size.epub")
	rewriteZipEntry(t, path, input, "OEBPS/Styles/main.css", func([]byte) []byte {
		return []byte(`html { font-size: 100%; }
@media (min-width: 40em) { :root { font: 1em/1.6 serif; } }
body { margin: 0; font-size: 1em; }
body p { font-size: .9em; }
html { line-height: 1.6; }
.visually-hidden { font-size: 1px; }
`)
	})
	b, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := run(t.Context(), b, Params{}, stubProbe(false))
	if err != nil {
		t.Fatal(err)
	}
	var rootErrors, bodyWarns []string
	for _, f := range res.Findings {
		switch f.Detail {
		case "css-root-font-size":
			if f.Level != "error" {
				t.Errorf("root finding level = %q", f.Level)
			}
			rootErrors = append(rootErrors, f.Location)
		case "css-body-font-size":
			if f.Level != "info" {
				t.Errorf("body finding level = %q", f.Level)
			}
			bodyWarns = append(bodyWarns, f.Location)
		}
	}
	wantRoot := []string{
		"Styles/main.css -> html { font-size: 100% }",
		"Styles/main.css -> :root { font: 1em/1.6 serif }",
	}
	wantBody := []string{"Styles/main.css -> body { font-size: 1em }"}
	if diff := diffJSON(t, rootErrors, wantRoot); diff != "" {
		t.Errorf("root findings:\n%s", diff)
	}
	if diff := diffJSON(t, bodyWarns, wantBody); diff != "" {
		t.Errorf("body findings:\n%s", diff)
	}
}

func TestReaderFontScalingCleanStylesheetHasNoFinding(t *testing.T) {
	path := writeNativeFixture(t)
	input := filepath.Join(t.TempDir(), "clean-font-size.epub")
	rewriteZipEntry(t, path, input, "OEBPS/Styles/main.css", func([]byte) []byte {
		return []byte(`html { margin: 0; }
body { line-height: 1.7; }
p { font-size: 1em; }
h1 { font-size: 1.6em; }
`)
	})
	res := runNativeAudit(t, input)
	for _, f := range res.Findings {
		if f.Detail == "css-root-font-size" || f.Detail == "css-body-font-size" {
			t.Fatalf("unexpected font-scaling finding: %+v", f)
		}
	}
}

func TestReaderFontScalingFixedSizes(t *testing.T) {
	path := writeNativeFixture(t)
	input := filepath.Join(t.TempDir(), "fixed-font-size.epub")
	rewriteZipEntry(t, path, input, "OEBPS/Styles/main.css", func([]byte) []byte {
		return []byte(`p { font-size: 16px; }
.caption { font-size: small; }
h1 { font: bold 2vw/1.2 serif; }
.note { font-size: 0.9em; }
.big { font: 1.2em/1.5 serif; }
.visually-hidden { font-size: 1px; }
.zero { font-size: 0; }
body.fullpage .poster-title { font-size: 40px; }
* { font-size: 100%; }
`)
	})
	b, err := book.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	res, err := run(t.Context(), b, Params{}, stubProbe(false))
	if err != nil {
		t.Fatal(err)
	}
	var fixed, root []string
	for _, f := range res.Findings {
		switch f.Detail {
		case "css-fixed-font-size":
			if f.Level != "warn" {
				t.Errorf("fixed finding level = %q", f.Level)
			}
			fixed = append(fixed, f.Location)
		case "css-root-font-size":
			root = append(root, f.Location)
		}
	}
	wantFixed := []string{
		"Styles/main.css -> p { font-size: 16px }",
		"Styles/main.css -> .caption { font-size: small }",
		"Styles/main.css -> h1 { font: bold 2vw/1.2 serif }",
	}
	if diff := diffJSON(t, fixed, wantFixed); diff != "" {
		t.Errorf("fixed findings:\n%s", diff)
	}
	if diff := diffJSON(t, root, []string{"Styles/main.css -> * { font-size: 100% }"}); diff != "" {
		t.Errorf("universal selector findings:\n%s", diff)
	}
}
