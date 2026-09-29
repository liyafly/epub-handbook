package migrateepub3

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestNormalizeXHTMLShellEditsOnlyTargetRegions(t *testing.T) {
	source := `<?xml version='1.0' encoding='UTF-8'?>
<!-- keep prolog comment -->
<!DOCTYPE html PUBLIC "old" "old.dtd" [<!ELEMENT html ANY><!-- > stays in subset -->]>
<?review keep?>
<h:html xmlns:h="http://www.w3.org/1999/xhtml" xml:lang='zh&#45;CN' data-x = 'a &amp; b'>
<h:head><h:title>Keep <b>mixed</b> text</h:title>
<!-- <meta http-equiv="Content-Type" content="text/html; charset=utf-8"/> -->
<h:meta id='keep' http-equiv='Content-Type' content='application/xhtml+xml; charset=utf-8'/>
<![CDATA[<meta charset="utf-8"/>]]></h:head>
<h:body><h:p><big id='large'>中文</big><!-- <big>keep fake tag</big> --></h:p></h:body>
</h:html>`
	want := `<?xml version='1.0' encoding='UTF-8'?>
<!-- keep prolog comment -->
<!DOCTYPE html>
<?review keep?>
<h:html xmlns:h="http://www.w3.org/1999/xhtml" xml:lang='zh&#45;CN' data-x = 'a &amp; b' xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-CN">
<h:head><h:title>Keep <b>mixed</b> text</h:title>
<!-- <meta http-equiv="Content-Type" content="text/html; charset=utf-8"/> -->
<h:meta id='keep' charset="utf-8"/>
<![CDATA[<meta charset="utf-8"/>]]></h:head>
<h:body><h:p><span id='large' class="big">中文</span><!-- <big>keep fake tag</big> --></h:p></h:body>
</h:html>`
	got, changed, err := normalizeXHTMLShell(source, "en")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected shell edits")
	}
	if got != want {
		t.Fatalf("XHTML 非目标字节发生变化:\n got %q\nwant %q", got, want)
	}
	if _, changed, err := normalizeXHTMLShell(got, "en"); err != nil || changed {
		t.Fatalf("second shell pass should be a no-op: changed=%t err=%v", changed, err)
	}
}

func TestNormalizeXHTMLShellAndStylesheetInsertionsAreIdempotent(t *testing.T) {
	source := `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><!-- <link href="../Styles/main.css"/> --><title>x</title></head><body>x</body></html>`
	got, changed, err := normalizeXHTMLShell(source, "en")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected shell edits")
	}
	got, linked, err := ensureStylesheetLink(got, "../Styles/main.css")
	if err != nil {
		t.Fatal(err)
	}
	if !linked {
		t.Fatal("expected stylesheet link insertion despite matching text in a comment")
	}
	if strings.Count(got, `href="../Styles/main.css"`) != 2 {
		t.Fatalf("expected one comment occurrence and one real link, got %q", got)
	}
	again, linked, err := ensureStylesheetLink(got, "../Styles/main.css")
	if err != nil {
		t.Fatal(err)
	}
	if linked || again != got {
		t.Fatalf("second stylesheet pass should be a no-op: linked=%t", linked)
	}
	again, changed, err = normalizeXHTMLShell(got, "en")
	if err != nil || changed || again != got {
		t.Fatalf("second shell pass should be a no-op: changed=%t err=%v", changed, err)
	}
}

func TestNormalizeXHTMLShellRejectsTruncatedMarkup(t *testing.T) {
	if _, _, err := normalizeXHTMLShell(`<html><head><!-- unfinished`, "en"); err == nil {
		t.Fatal("expected truncated XHTML to be rejected")
	}
}

func TestMigrateConvertsNamedEntitiesWhenDroppingDTD(t *testing.T) {
	source := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "xhtml11.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en">
<head><title>Keep &mdash;</title><style>p::after{content:'&nbsp;'}</style></head><body title="A&nbsp;B &mdash;">Fish &nbsp;chips &mdash;&amp;&lt;&gt;&quot;&apos;<!-- &bogus; &nbsp; --><![CDATA[&bogus; &nbsp;]]></body>
</html>`
	want := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en" xmlns:epub="http://www.idpf.org/2007/ops" lang="en">
<head><title>Keep &#8212;</title><style>p::after{content:'&#160;'}</style><meta charset="utf-8"/></head><body title="A&#160;B &#8212;">Fish &#160;chips &#8212;&amp;&lt;&gt;&quot;&apos;<!-- &bogus; &nbsp; --><![CDATA[&bogus; &nbsp;]]></body>
</html>`
	got, changed, err := normalizeXHTMLShell(source, "en")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected shell and entity edits")
	}
	if got != want {
		t.Fatalf("migrate output changed unexpected bytes:\n got %q\nwant %q", got, want)
	}
	decoder := xml.NewDecoder(strings.NewReader(got))
	decoder.Strict = true
	for {
		if _, err := decoder.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("migrated XHTML is not well-formed XML: %v", err)
		}
	}
}

func TestNormalizeXHTMLShellRejectsUnknownNamedEntity(t *testing.T) {
	source := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>x</title></head><body>&notAnXHTMLEntity;</body></html>`
	if _, _, err := normalizeXHTMLShell(source, "en"); err == nil {
		t.Fatal("expected unknown named entity to be rejected")
	}
}

func TestNormalizeXHTMLShellRejectsEntitiesOutsideXMLHTMLTable(t *testing.T) {
	for _, entity := range []string{"&check;", "&notit;"} {
		t.Run(entity, func(t *testing.T) {
			source := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>x</title></head><body>` + entity + `</body></html>`
			if _, _, err := normalizeXHTMLShell(source, "en"); err == nil {
				t.Fatalf("expected unsupported entity %s to be rejected", entity)
			}
		})
	}
}

func TestNormalizeXHTMLShellConvertsStyleEntitiesAndPreservesOpaqueText(t *testing.T) {
	source := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd"><html xmlns="http://www.w3.org/1999/xhtml"><head><title>&mdash;</title><style>p::after{content:"&mdash;"}</style></head><body><p>&mdash; &amp;nbsp;</p><!-- &mdash; --><![CDATA[&mdash;]]></body></html>`
	got, _, err := normalizeXHTMLShell(source, "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<title>&#8212;</title>`,
		`<style>p::after{content:"&#8212;"}</style>`,
		`<p>&#8212; &amp;nbsp;</p>`,
		`<!-- &mdash; -->`,
		`<![CDATA[&mdash;]]>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("normalized XHTML lacks %q:\n%s", want, got)
		}
	}
}

func TestMigrateRejectsInternalSubsetEntities(t *testing.T) {
	source := `<!DOCTYPE html [<!ENTITY mdash "&#8212;">]><html xmlns="http://www.w3.org/1999/xhtml"><head><title>x</title></head><body>&mdash;</body></html>`
	if _, _, err := normalizeXHTMLShell(source, "en"); err == nil {
		t.Fatal("expected internal entity declarations to be rejected")
	}
}

func TestXHTMLSourceEncodingGate(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		want bool
	}{
		{name: "utf8 declaration", data: []byte(`<?xml version="1.0" encoding="UTF-8"?><html/>`), want: true},
		{name: "utf8 bom", data: append([]byte{0xEF, 0xBB, 0xBF}, []byte(`<html/>`)...), want: true},
		{name: "legacy declaration", data: []byte(`<?xml version="1.0" encoding="ISO-8859-1"?><html/>`), want: false},
		{name: "invalid bytes", data: []byte{'<', 'p', '>', 0xff, '<', '/', 'p', '>'}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := xhtmlSourceIsUTF8(tc.data); got != tc.want {
				t.Fatalf("xhtmlSourceIsUTF8()=%t want %t", got, tc.want)
			}
		})
	}
}
