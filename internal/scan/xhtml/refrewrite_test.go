package xhtml

import (
	"strings"
	"testing"
)

func TestRewriteMarkupUsesRegionsAndOneTagAttributeParse(t *testing.T) {
	input := `<!-- <img src="ghost.png"> --><img src="a.png" srcset="a.png 1x, b.png 2x" style="background:url(a.png)"><style>p{background:url(a.png)}</style><script>"<img src='ghost.png'>"</script><?xml-stylesheet href="a.css"?>`
	rewriteURI := func(uri string) string {
		return strings.ReplaceAll(uri, "a.", "new.")
	}
	rewriteCSS := func(raw, _ string, _ byte, rewrite func(string) string) (string, error) {
		return strings.ReplaceAll(raw, "url(a.png)", "url("+rewrite("a.png")+")"), nil
	}
	got, err := RewriteMarkup(input, "Text/ch.xhtml", rewriteURI, rewriteCSS, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := `<!-- <img src="ghost.png"> --><img src="new.png" srcset="new.png 1x, b.png 2x" style="background:url(new.png)"><style>p{background:url(new.png)}</style><script>"<img src='ghost.png'>"</script><?xml-stylesheet href="new.css"?>`
	if got != want {
		t.Fatalf("RewriteMarkup() = %q, want %q", got, want)
	}
}
