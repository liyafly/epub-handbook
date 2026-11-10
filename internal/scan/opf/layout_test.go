package opf

import "testing"

func layoutPackage(t *testing.T, global, spine string) *Package {
	t.Helper()
	meta := ""
	if global != "" {
		meta = `<meta property="rendition:layout">` + global + `</meta>`
	}
	data := `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata>` + meta + `</metadata>` +
		`<manifest><item id="a" href="a.xhtml" media-type="application/xhtml+xml"/><item id="b" href="b.xhtml" media-type="application/xhtml+xml"/><item id="c" href="c.css" media-type="text/css"/></manifest>` +
		`<spine>` + spine + `</spine></package>`
	pkg, err := Parse("OEBPS/content.opf", []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}

func TestFixedLayoutDetection(t *testing.T) {
	cases := []struct {
		name, global, spine string
		want                map[string]bool
	}{
		{"reflowable default", "", `<itemref idref="a"/><itemref idref="b"/>`, map[string]bool{}},
		{"global fxl", "pre-paginated", `<itemref idref="a"/><itemref idref="b"/>`, map[string]bool{"OEBPS/a.xhtml": true, "OEBPS/b.xhtml": true}},
		{"itemref override to reflowable", "pre-paginated", `<itemref idref="a" properties="rendition:layout-reflowable"/><itemref idref="b"/>`, map[string]bool{"OEBPS/b.xhtml": true}},
		{"itemref override to fxl", "reflowable", `<itemref idref="a" properties="page-spread-left rendition:layout-pre-paginated"/><itemref idref="b"/>`, map[string]bool{"OEBPS/a.xhtml": true}},
	}
	for _, c := range cases {
		pkg := layoutPackage(t, c.global, c.spine)
		got := FixedLayoutXHTML(pkg)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
			continue
		}
		for k := range c.want {
			if !got[k] {
				t.Errorf("%s: missing %s in %v", c.name, k, got)
			}
		}
	}
	if !layoutPackage(t, " pre-paginated ", "").GlobalFixedLayout() {
		t.Error("global meta text should be trimmed")
	}
}
