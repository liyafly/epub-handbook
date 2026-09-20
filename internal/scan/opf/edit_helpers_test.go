package opf

import (
	"testing"

	"github.com/liyafly/epub-handbook/internal/editset"
)

func TestManifestNodesPreserveNamespaceAndOrder(t *testing.T) {
	data := []byte(`<package xmlns="http://www.idpf.org/2007/opf" xmlns:f="urn:foreign"><manifest><item id="a"/><f:item id="foreign"/></manifest><f:manifest><item id="wrong-parent"/></f:manifest><manifest><item id="b"/></manifest></package>`)
	root, err := ScanSpanTree(data)
	if err != nil {
		t.Fatal(err)
	}
	items := ManifestNodes(root)
	if len(items) != 2 {
		t.Fatalf("items=%+v", items)
	}
	for i, want := range []string{"a", "b"} {
		if got, _ := items[i].AttrByLocal("", "id"); got != want {
			t.Fatalf("id=%s want %s", got, want)
		}
	}
}

func TestSharedOPFEditsPreserveNeighborBytes(t *testing.T) {
	data := []byte("<manifest><item id='keep'/><!-- retained -->\r\n<item id='drop'/> \r\n<?still here?><item id='last'/></manifest>")
	root, err := ScanSpanTree(data)
	if err != nil {
		t.Fatal(err)
	}
	edit := RemoveElementEdit("package.opf", data, root.Kids[1])
	got, err := editset.Apply("package.opf", data, []editset.Edit{edit})
	if err != nil {
		t.Fatal(err)
	}
	want := "<manifest><item id='keep'/><!-- retained -->\r\n<?still here?><item id='last'/></manifest>"
	if string(got) != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
	if got := BuildCSSItem("a&b", "Styles/a\n\".css"); got != `<item id="a&amp;b" href="Styles/a&#10;&quot;.css" media-type="text/css" />` {
		t.Fatal(got)
	}
}
