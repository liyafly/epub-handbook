package csscleanup

import (
	"bytes"
	"errors"
	"testing"

	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/scan/css"
)

func TestSanitizeCSSLosslessByteRanges(t *testing.T) {
	data := []byte("\xef\xbb\xbf/* keep {; } */\r\n" +
		"————————————————标题————————————————\r\n" +
		"p {\r\n" + "  content: \"a;b{}\"; /* keep ; {} */\r\n" + "  background: url(data:text/css,a;b{});\r\n" + "  margin: 0\r\n" + "  padding: 0;\r\n" + "  font-family: \"SimHei\";\r\n" + "}\r\n")

	edits, rewrites, err := sanitizeCSSData("Styles/main.css", data)
	if err != nil {
		t.Fatalf("sanitizeCSSData: %v", err)
	}
	if rewrites != 1 {
		t.Fatalf("font rewrites=%d, want 1", rewrites)
	}
	if err := editset.Validate(edits); err != nil {
		t.Fatalf("edits overlap: %v", err)
	}
	got, err := editset.Apply("Styles/main.css", data, edits)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := []byte("\xef\xbb\xbf/* keep {; } */\r\n\r\n" +
		"p {\r\n" + "  content: \"a;b{}\"; /* keep ; {} */\r\n" + "  background: url(data:text/css,a;b{});\r\n" + "  margin: 0;\r\n" + "  padding: 0;\r\n" + "  font-family: " + heiChain + ";\r\n" + "}\r\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("lossless output mismatch:\n got %q\nwant %q", got, want)
	}
	if !bytes.Contains(got, []byte("/* keep {; } */\r\n\r\n")) ||
		!bytes.Contains(got, []byte(`content: "a;b{}"; /* keep ; {} */`)) ||
		!bytes.Contains(got, []byte("url(data:text/css,a;b{})")) {
		t.Fatal("untargeted comment/string/data URL bytes were not preserved")
	}
}

func TestSanitizeCSSParseErrorProducesNoEdits(t *testing.T) {
	tests := [][]byte{
		[]byte("p { font-family: \"SimHei\";"),
		[]byte{'p', '{', 'x', ':', 0xff, '}'},
	}
	for _, data := range tests {
		data := data
		t.Run(string(data), func(t *testing.T) {
			edits, rewrites, err := sanitizeCSSData("broken.css", data)
			if err == nil || len(edits) != 0 || rewrites != 0 {
				t.Fatalf("edits=%v rewrites=%d err=%v, want no edits and an error", edits, rewrites, err)
			}
			if got, count := sanitizeCSS(string(data)); got != string(data) || count != 0 {
				t.Fatalf("compatibility sanitize wrote on parse error: %q (%d)", got, count)
			}
		})
	}
	if _, err := css.Parse([]byte("p{color:red}")); err != nil {
		t.Fatalf("sanity parse: %v", err)
	}
	var parseErr *css.ParseError
	if _, err := css.Parse([]byte{'p', '{', 0xff}); !errors.As(err, &parseErr) {
		t.Fatalf("invalid UTF-8 error=%v, want ParseError", err)
	}
}

func TestSelectorListPartsKeepsNestedCommas(t *testing.T) {
	parts := selectorListParts(`.a, :is(.b,.c), [data-x=",,"], .d\,e`)
	if len(parts) != 4 {
		t.Fatalf("parts=%q, want four top-level selectors", parts)
	}
	if parts[1] != ` :is(.b,.c)` || parts[2] != ` [data-x=",,"]` || parts[3] != ` .d\,e` {
		t.Fatalf("nested selector commas were split: %q", parts)
	}
	if got := scopedSelector(`.a, :is(.b,.c)`, "scope"); got != "body.scope .a,\nbody.scope :is(.b,.c)" {
		t.Fatalf("scoped selector=%q", got)
	}
}

func TestXHTMLLinkEditsTargetHrefSpan(t *testing.T) {
	data := []byte("<!-- <link href=\"ignored.css\"> -->\r\n" +
		"<link data-x=\"a>b\" href='../Styles/old.css' type=\"text/css\"/>\r\n")
	mapping := map[string][]string{"Styles/old.css": {"Styles/new.css", "Styles/alt.css"}}
	edits, changed, err := rewriteCSSLinkEdits("Text/ch.xhtml", data, mapping)
	if err != nil || !changed {
		t.Fatalf("link edits changed=%v err=%v", changed, err)
	}
	if len(edits) != 2 || edits[0].Length != int64(len("../Styles/old.css")) {
		t.Fatalf("edits=%+v, want href replacement plus clone insertion", edits)
	}
	if err := editset.Validate(edits); err != nil {
		t.Fatalf("link edits overlap: %v", err)
	}
	got, err := editset.Apply("Text/ch.xhtml", data, edits)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !bytes.Contains(got, []byte("<!-- <link href=\"ignored.css\"> -->")) || !bytes.Contains(got, []byte("href='../Styles/new.css'")) ||
		!bytes.Contains(got, []byte("href='../Styles/alt.css'")) || !bytes.Contains(got, []byte("data-x=\"a>b\"")) {
		t.Fatalf("link rewrite touched wrong bytes: %q", got)
	}
	if _, _, err := rewriteCSSLinkEdits("Text/ch.xhtml", []byte{'<', 0xff, '>'}, mapping); err == nil {
		t.Fatal("invalid UTF-8 XHTML should be rejected before patching")
	}
}
