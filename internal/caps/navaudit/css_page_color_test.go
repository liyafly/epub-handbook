package navaudit

import (
	"path/filepath"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
)

func TestSelectorTargetsPageColor(t *testing.T) {
	cases := []struct {
		selector string
		want     bool
	}{
		{"html", true},
		{"HTML", true},
		{":root", true},
		{"*", true},
		{"body", true},
		{"body.colophon", true},
		{"html body", true},
		{"html, p", true},
		{"body.fullpage", true},
		{"body.poster-bg", false},
		{"body.fullpage.poster-bg", false},
		{"body.fullpage.poster-bg-contain", false},
		{"body:not(.poster-bg)", true},
		{"body p", false},
		{"html > body > p", false},
		{".html", false},
		{"bodytext", false},
		{"p:not(:root)", false},
		{"*::before", false},
		{"body *", false},
	}
	for _, tc := range cases {
		if got := selectorTargetsPageColor(tc.selector); got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.selector, got, tc.want)
		}
	}
}

func TestLocksPageColor(t *testing.T) {
	cases := map[string]bool{
		"#1a1a1a":                true,
		"black":                  true,
		"url(a.png) no-repeat":   true,
		"#fff !important":        true,
		"inherit":                false,
		"INHERIT":                false,
		"transparent":            false,
		"none":                   false,
		"initial":                false,
		"unset":                  false,
		"currentColor":           false,
		"inherit !important":     false,
		"":                       false,
		"transparent url(a.png)": true,
		"rgba(0, 0, 0, 0)":       true,
	}
	for value, want := range cases {
		if got := locksPageColor(value); got != want {
			t.Errorf("%q: got %v, want %v", value, got, want)
		}
	}
}

func TestPageColorLockFindings(t *testing.T) {
	path := writeNativeFixture(t)
	input := filepath.Join(t.TempDir(), "page-color.epub")
	rewriteZipEntry(t, path, input, "OEBPS/Styles/main.css", func([]byte) []byte {
		return []byte(`html { background: #fff; margin: 0; }
body { color: #1a1a1a; background-color: transparent; }
@media (prefers-color-scheme: dark) { :root { color: white; } }
body.fullpage { background: #000; }
body.poster-bg { background: url(../Images/bg.png) no-repeat; }
body.fullpage.poster-bg { background-color: #222; }
* { color: inherit; }
p { color: #333; background: #fafafa; }
h1 { color: red; }
.note-box { background: #fffaf0; }
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
	var got []string
	for _, f := range res.Findings {
		if f.Detail != "css-page-color-lock" {
			continue
		}
		if f.Level != "warn" {
			t.Errorf("page color finding level = %q", f.Level)
		}
		got = append(got, f.Location)
	}
	want := []string{
		"Styles/main.css -> html { background: #fff }",
		"Styles/main.css -> body { color: #1a1a1a }",
		"Styles/main.css -> :root { color: white }",
		"Styles/main.css -> body.fullpage { background: #000 }",
	}
	if diff := diffJSON(t, got, want); diff != "" {
		t.Errorf("page color findings:\n%s", diff)
	}
}

func TestPageColorLockCleanStylesheetHasNoFinding(t *testing.T) {
	path := writeNativeFixture(t)
	input := filepath.Join(t.TempDir(), "clean-page-color.epub")
	rewriteZipEntry(t, path, input, "OEBPS/Styles/main.css", func([]byte) []byte {
		return []byte(`html { margin: 0; }
body { line-height: 1.7; color: inherit; background: none; }
p { color: #333; }
`)
	})
	res := runNativeAudit(t, input)
	for _, f := range res.Findings {
		if f.Detail == "css-page-color-lock" {
			t.Fatalf("unexpected page color finding: %+v", f)
		}
	}
}
