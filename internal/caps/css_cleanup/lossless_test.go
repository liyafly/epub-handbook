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
