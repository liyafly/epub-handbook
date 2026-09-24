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

func TestClassTokenEdit(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		token   string
		want    string
		present bool
		wantErr bool
	}{
		{name: "missing class", input: `<p id="x">text</p>`, token: "alpha", want: `<p id="x" class="alpha">text</p>`},
		{name: "append existing class", input: `<p class='x'>text</p>`, token: "y", want: `<p class='x y'>text</p>`},
		{name: "token already present", input: `<p class="x y">text</p>`, token: "x", want: `<p class="x y">text</p>`, present: true},
		{name: "empty class", input: `<p class="">text</p>`, token: "x", want: `<p class="x">text</p>`},
		{name: "self closing", input: `<img src="x"/>`, token: "icon", want: `<img src="x" class="icon"/>`},
		{name: "invalid token", input: `<p>text</p>`, token: "a b", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(tt.input)
			root, err := ScanSpanTree(data)
			if err != nil {
				t.Fatal(err)
			}
			edit, present, err := ClassTokenEdit("chapter.xhtml", data, root, tt.token)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ClassTokenEdit error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if present != tt.present {
				t.Fatalf("present = %v, want %v", present, tt.present)
			}
			if present {
				return
			}
			got, err := editset.Apply("chapter.xhtml", data, []editset.Edit{edit})
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassTokenEditRejectsMismatchedSpan(t *testing.T) {
	data := []byte(`<p class="x">text</p>`)
	root, err := ScanSpanTree(data)
	if err != nil {
		t.Fatal(err)
	}
	root.Open.End--
	if _, _, err := ClassTokenEdit("chapter.xhtml", data, root, "y"); err == nil {
		t.Fatal("ClassTokenEdit accepted a mismatched open-tag span")
	}
}
