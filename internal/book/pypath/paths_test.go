package pypath

import (
	"fmt"
	"strings"
	"testing"
)

func TestLexicalPathSemantics(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", "."}, {"a/../", "."}, {"../../a", "../../a"},
		{"/../../a", "/a"}, {"//server/../a", "//a"}, {"///a//b", "/a/b"},
		{"a%20b/../中", "中"}, {"a/..x", "a/..x"},
	} {
		if got := NormPath(tc.in); got != tc.want {
			t.Errorf("NormPath(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
	if got := Join("OPS/", "/Text/a"); got != "/Text/a" {
		t.Fatal(got)
	}
	if got := NormJoin("OPS/Text", "../a%20b.css?v=1#anchor"); got != "OPS/a%20b.css?v=1" {
		t.Fatal(got)
	}
	if got := NormJoin("OPS/Text", "/Images/a.png#cover"); got != "/Images/a.png" {
		t.Fatalf("rooted NormJoin = %q", got)
	}
	if got := Join("OPS", "Text", "chapter.xhtml"); got != "OPS/Text/chapter.xhtml" {
		t.Fatalf("Join = %q", got)
	}
	if got := RelPath("OPS/Text/a", "OPS/Styles"); got != "../Text/a" {
		t.Fatal(got)
	}
	if got := RelPath("OPS", "OPS"); got != "." {
		t.Fatal(got)
	}
	for _, tc := range []struct{ in, want string }{
		{"OPS/Styles/a.css", "a"}, {"OPS/.hidden", ".hidden"},
		{"OPS/.hidden.css", ".hidden"}, {"a.b.css", "a.b"},
	} {
		if got := BaseStem(tc.in); got != tc.want {
			t.Errorf("BaseStem(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestURLPathEscaping(t *testing.T) {
	if got := Unquote("a%20b%2F%E4%B8%AD%FF%ZZ"); got != "a b/中\uFFFD%ZZ" {
		t.Fatalf("Unquote = %q", got)
	}
	if got := QuotePath("a b/中"); got != "a%20b/%E4%B8%AD" {
		t.Fatalf("QuotePath = %q", got)
	}
	if got, err := ResolveRootPath("/Fonts/obf%20font.otf"); err != nil || got != "Fonts/obf font.otf" {
		t.Fatalf("ResolveRootPath = %q, %v", got, err)
	}
}

func TestPathAndURIEscapingRemainDistinct(t *testing.T) {
	from, to := "OPS/Text/a.xhtml", "OPS/Images/中 a.png"
	if got := RelativePath(from, to); got != "../Images/中 a.png" {
		t.Fatal(got)
	}
	if got := RelativeURI(from, to); got != "../Images/%E4%B8%AD%20a.png" {
		t.Fatal(got)
	}
	if got := RelativePath("a.xhtml", to); got != to {
		t.Fatal(got)
	}
}

func TestRewriteURIWarnsAndKeepsRootedHref(t *testing.T) {
	var warnings []string
	warn := func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}
	const href = "/Images/cover.png"
	got := RewriteURI(href, "OEBPS/Text/chapter.xhtml", "OEBPS/Text/chapter.xhtml", nil, map[string]bool{}, warn)
	if got != href {
		t.Fatalf("RewriteURI() = %q, want unchanged %q", got, href)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], href) {
		t.Fatalf("warnings = %q, want one warning naming the unsafe href", warnings)
	}
}

func TestRewriteURIMapsKnownLocalReference(t *testing.T) {
	known := map[string]bool{"OEBPS/Images/old.png": true}
	pathMap := map[string]string{"OEBPS/Images/old.png": "OEBPS/Images/new.png"}
	got := RewriteURI("../Images/old.png?download=1#cover", "OEBPS/Text/chapter.xhtml", "OEBPS/Text/chapter.xhtml", pathMap, known, nil)
	if got != "../Images/new.png?download=1#cover" {
		t.Fatalf("RewriteURI() = %q", got)
	}
}

func TestFixedQuoteAttributeEscaping(t *testing.T) {
	if got := EscapeAttribute("&<>\"'\r\n\t"); got != "&amp;&lt;&gt;&quot;'&#13;&#10;&#09;" {
		t.Fatal(got)
	}
}
