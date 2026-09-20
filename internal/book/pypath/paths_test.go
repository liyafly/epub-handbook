package pypath

import "testing"

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

func TestFixedQuoteAttributeEscaping(t *testing.T) {
	if got := EscapeAttribute("&<>\"'\r\n\t"); got != "&amp;&lt;&gt;&quot;'&#13;&#10;&#09;" {
		t.Fatal(got)
	}
}
