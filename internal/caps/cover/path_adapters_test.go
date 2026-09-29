package cover

import (
	"errors"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
)

func TestSharedPathAdaptersPreserveBoundarySemantics(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"../Images/a%20b.png", "OPS/Images/a b.png"},
		{"../Images/%E4%B8%AD.png", "OPS/Images/中.png"},
		{"../Images/%zz.png", "OPS/Images/%zz.png"},
	} {
		got, err := resolveRelativePath("OPS/Text/ch.xhtml", tc.raw)
		if err != nil || got != tc.want {
			t.Fatalf("resolve %q=%q %v", tc.raw, got, err)
		}
	}
	if _, err := resolveRelativePath("OPS/Text/a.xhtml", "../../../escape"); !errors.Is(err, ErrPackageTool) {
		t.Fatalf("lost capability sentinel: %v", err)
	}
	parts := pypath.URLSplit(" \tHTTPS://example.test/a\nb?q=1#note\r ")
	if parts.Scheme != "https" || parts.Netloc != "example.test" || parts.Path != "/ab" || parts.Query != "q=1" || parts.Fragment != "note" {
		t.Fatalf("URL projection: %+v", parts)
	}
}
