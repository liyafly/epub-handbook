package xhtml

import (
	"slices"
	"testing"

	"github.com/liyafly/epub-handbook/internal/editset"
)

func TestNormalizeXHTML11EntitiesForXMLPreservesOpaqueContent(t *testing.T) {
	input := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd"><html><body><p title="&nbsp;">&nbsp;&mdash; &amp;</p><!-- &nbsp; --><script><![CDATA[&mdash;]]></script></body></html>`
	want := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd"><html><body><p title="&#160;">&#160;&#8212; &amp;</p><!-- &nbsp; --><script><![CDATA[&mdash;]]></script></body></html>`
	edits := XHTML11EntityEdits("legacy.xhtml", input)
	gotBytes, err := editset.Apply("legacy.xhtml", []byte(input), edits)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(gotBytes); got != want {
		t.Fatalf("normalized XHTML = %q, want %q", got, want)
	}
}

func TestNormalizeXHTML11EntitiesForXMLRequiresActualDoctype(t *testing.T) {
	for _, input := range []string{
		`<html><body><p>&nbsp;</p></body></html>`,
		`<!-- <!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd"> --><html><body><p>&nbsp;</p></body></html>`,
	} {
		edits := XHTML11EntityEdits("legacy.xhtml", input)
		got, err := editset.Apply("legacy.xhtml", []byte(input), edits)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != input {
			t.Errorf("input without a real XHTML 1.1 doctype changed: %q -> %q", input, got)
		}
	}
}

func TestXHTML11EntityEditsAreOrderedAndDisjoint(t *testing.T) {
	text := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd"><html title="&nbsp;"><body>&mdash;</body></html>`
	edits := XHTML11EntityEdits("legacy.xhtml", text)
	if len(edits) != 2 {
		t.Fatalf("edits=%+v, want attribute and text replacements", edits)
	}
	if !slices.IsSortedFunc(edits, func(a, b editset.Edit) int {
		if a.Offset < b.Offset {
			return -1
		}
		if a.Offset > b.Offset {
			return 1
		}
		return 0
	}) {
		t.Fatalf("edits are not ordered by source offset: %+v", edits)
	}
}
