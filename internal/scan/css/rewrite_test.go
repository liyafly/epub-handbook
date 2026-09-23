package css

import (
	"bytes"
	"testing"

	"github.com/liyafly/epub-handbook/internal/editset"
)

func TestRewriteCSSKeepsUntouchedBytes(t *testing.T) {
	input := []byte("/*keep*/ .x { background: url(a.png); content: \"url(ghost.png)\" }\r\n")
	edits, err := RewriteCSS("Styles/main.css", input, func(uri string) string {
		if uri == "a.png" {
			return "b.png"
		}
		return uri
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := editset.Apply("Styles/main.css", input, edits)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("/*keep*/ .x { background: url(b.png); content: \"url(ghost.png)\" }\r\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("RewriteCSS() = %q, want %q", got, want)
	}
}
