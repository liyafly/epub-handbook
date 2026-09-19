package opf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testContainer = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/>
    <rootfile full-path="OEBPS/second.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

const testOPF = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid"
         prefix="rendition: http://www.idpf.org/vocab/rendition/#">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="bookid">urn:uuid:1234</dc:identifier>
    <dc:title> Main Title </dc:title>
    <dc:title>Subtitle</dc:title>
    <dc:creator>Author One</dc:creator>
    <dc:language>zh-CN</dc:language>
    <meta property="dcterms:modified">2026-01-02T03:04:05Z</meta>
    <meta name="cover" content="cover-img"/>
    <meta refines="#bookid" property="identifier-type">uuid</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="scripted nav"/>
    <item id="toc" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="cover-img" href="Images/cover%20art.png" media-type="image/png" properties="cover-image"/>
    <item id="ch1" href="Text/ch1.xhtml" media-type="application/xhtml+xml" fallback="ch1-fb"/>
    <item id="ch1-fb" href="../ch1.html" media-type="text/html"/>
    <item id="remote" href="https://example.com/x.css" media-type="text/css"/>
  </manifest>
  <spine toc="toc" page-progression-direction="ltr">
    <itemref idref="ch1" linear="yes" properties="rendition:layout-pre-paginated"/>
    <itemref idref="ch1-fb" linear="no"/>
  </spine>
</package>`

func TestFindOPFPath(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "first rootfile wins", in: testContainer, want: "OEBPS/package.opf"},
		{
			name: "rootfile outside container namespace is ignored",
			in: `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
			  <rootfiles xmlns="http://example.com/other"><rootfile full-path="x.opf"/></rootfiles></container>`,
			wantErr: true,
		},
		{
			name:    "missing rootfile",
			in:      `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles/></container>`,
			wantErr: true,
		},
		{
			name:    "empty full-path",
			in:      `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path=""/></rootfiles></container>`,
			wantErr: true,
		},
		{name: "malformed xml", in: `<container><rootfiles>`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FindOPFPath([]byte(tc.in))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("FindOPFPath = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("FindOPFPath error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("FindOPFPath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	pkg, err := Parse("OEBPS/package.opf", []byte(testOPF))
	if err != nil {
		t.Fatalf("Parse error = %v", err)
	}
	if pkg.Path != "OEBPS/package.opf" || pkg.Version != "3.0" || pkg.UniqueID != "bookid" {
		t.Errorf("package attrs = %q/%q/%q", pkg.Path, pkg.Version, pkg.UniqueID)
	}
	if !strings.HasPrefix(pkg.Prefix, "rendition:") {
		t.Errorf("Prefix = %q", pkg.Prefix)
	}
	if pkg.OPFDir() != "OEBPS/" {
		t.Errorf("OPFDir = %q, want %q", pkg.OPFDir(), "OEBPS/")
	}
	if pkg.SpineToc != "toc" {
		t.Errorf("SpineToc = %q, want toc", pkg.SpineToc)
	}

	// metadata：dc:* 文本 TrimSpace，标题按文档序。
	if got := strings.Join(pkg.MetadataTitles, "|"); got != "Main Title|Subtitle" {
		t.Errorf("MetadataTitles = %q", got)
	}
	if got := pkg.Metadata["creator"]; len(got) != 1 || got[0] != "Author One" {
		t.Errorf("Metadata[creator] = %q", got)
	}
	if got := pkg.Metadata["identifier"]; len(got) != 1 || got[0] != "urn:uuid:1234" {
		t.Errorf("Metadata[identifier] = %q", got)
	}
	if len(pkg.Metas) != 3 {
		t.Fatalf("len(Metas) = %d, want 3", len(pkg.Metas))
	}
	if m := pkg.Metas[0]; m.Property != "dcterms:modified" || m.Text != "2026-01-02T03:04:05Z" {
		t.Errorf("Metas[0] = %+v", m)
	}
	if m := pkg.Metas[1]; m.Name != "cover" || m.Content != "cover-img" || m.Text != "" {
		t.Errorf("Metas[1] = %+v", m)
	}
	if m := pkg.Metas[2]; m.Refines != "#bookid" || m.Property != "identifier-type" || m.Text != "uuid" {
		t.Errorf("Metas[2] = %+v", m)
	}

	// manifest。
	if len(pkg.Manifest) != 6 {
		t.Fatalf("len(Manifest) = %d, want 6", len(pkg.Manifest))
	}
	nav, ok := pkg.NavItem()
	if !ok || nav.ID != "nav" || nav.ArchivePath != "OEBPS/nav.xhtml" {
		t.Errorf("NavItem = %+v, %v", nav, ok)
	}
	cover, ok := pkg.CoverItem()
	if !ok || cover.ID != "cover-img" || cover.ArchivePath != "OEBPS/Images/cover art.png" {
		t.Errorf("CoverItem = %+v, %v", cover, ok)
	}
	ncx, ok := pkg.NCXItem()
	if !ok || ncx.ID != "toc" || ncx.Href != "toc.ncx" {
		t.Errorf("NCXItem = %+v, %v", ncx, ok)
	}
	ch1, ok := pkg.ItemByID("ch1")
	if !ok || ch1.Fallback != "ch1-fb" || ch1.MediaType != "application/xhtml+xml" {
		t.Errorf("ItemByID(ch1) = %+v, %v", ch1, ok)
	}
	fb, ok := pkg.ItemByHref("../ch1.html")
	if !ok || fb.ID != "ch1-fb" || fb.ArchivePath != "ch1.html" {
		t.Errorf("ItemByHref(../ch1.html) = %+v, %v", fb, ok)
	}
	remote, ok := pkg.ItemByID("remote")
	if !ok || remote.ArchivePath != "" {
		t.Errorf("remote item ArchivePath = %q, want empty for external href", remote.ArchivePath)
	}
	if _, ok := pkg.ItemByID("nope"); ok {
		t.Error("ItemByID(nope) found unexpected item")
	}
	if _, ok := pkg.ItemByHref("nope.xhtml"); ok {
		t.Error("ItemByHref(nope.xhtml) found unexpected item")
	}

	// spine。
	if len(pkg.Spine) != 2 {
		t.Fatalf("len(Spine) = %d, want 2", len(pkg.Spine))
	}
	if s := pkg.Spine[0]; s.IDRef != "ch1" || s.Linear != "yes" || s.Properties != "rendition:layout-pre-paginated" {
		t.Errorf("Spine[0] = %+v", s)
	}
	if s := pkg.Spine[1]; s.IDRef != "ch1-fb" || s.Linear != "no" {
		t.Errorf("Spine[1] = %+v", s)
	}
}

func TestParseEmptyPackageHasNonNilCollections(t *testing.T) {
	pkg, err := Parse("package.opf", []byte(`<package xmlns="http://www.idpf.org/2007/opf" version="2.0"/>`))
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Manifest == nil || pkg.Spine == nil || pkg.MetadataTitles == nil || pkg.Metadata == nil || pkg.Metas == nil {
		t.Fatalf("collections must be non-nil for stable JSON: %+v", pkg)
	}
	if pkg.OPFDir() != "" {
		t.Errorf("OPFDir for root OPF = %q, want empty", pkg.OPFDir())
	}
	if _, ok := pkg.NavItem(); ok {
		t.Error("NavItem on empty manifest should be absent")
	}
	if _, ok := pkg.CoverItem(); ok {
		t.Error("CoverItem on empty manifest should be absent")
	}
	if _, ok := pkg.NCXItem(); ok {
		t.Error("NCXItem on empty manifest should be absent")
	}
}

func TestParseMalformed(t *testing.T) {
	if _, err := Parse("package.opf", []byte(`<package><manifest>`)); err == nil {
		t.Fatal("Parse(malformed) error = nil, want error")
	}
}

func TestParseDemoOPF(t *testing.T) {
	p := filepath.Join("..", "..", "..", "templates", "epub-style-demo", "OEBPS", "package.opf")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("demo OPF unavailable: %v", err)
	}
	pkg, err := Parse("OEBPS/package.opf", data)
	if err != nil {
		t.Fatalf("Parse(demo) error = %v", err)
	}
	nav, ok := pkg.NavItem()
	if !ok || nav.Href != "nav.xhtml" {
		t.Errorf("demo NavItem = %+v, %v", nav, ok)
	}
	if len(pkg.Spine) == 0 || len(pkg.Manifest) == 0 {
		t.Errorf("demo spine/manifest empty: %d/%d", len(pkg.Spine), len(pkg.Manifest))
	}
	if _, ok := pkg.CoverItem(); !ok {
		t.Error("demo CoverItem missing")
	}
	if ncx, ok := pkg.NCXItem(); !ok || ncx.ID != pkg.SpineToc {
		t.Errorf("demo NCXItem = %+v, SpineToc = %q", ncx, pkg.SpineToc)
	}
	if len(pkg.MetadataTitles) != 1 {
		t.Errorf("demo MetadataTitles = %q", pkg.MetadataTitles)
	}
	for _, s := range pkg.Spine {
		if _, ok := pkg.ItemByID(s.IDRef); !ok {
			t.Errorf("spine idref %q has no manifest item", s.IDRef)
		}
	}
}

func TestLocalName(t *testing.T) {
	tests := map[string]string{
		"{http://www.idpf.org/2007/opf}item": "item",
		"dc:title":                           "title",
		"plain":                              "plain",
		"":                                   "",
	}
	for in, want := range tests {
		if got := LocalName(in); got != want {
			t.Errorf("LocalName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsExternalURI(t *testing.T) {
	tests := map[string]bool{
		"Text/ch1.xhtml":             false,
		"../Styles/a.css":            false,
		"a%20b.png":                  false,
		"":                           false,
		"http://example.com/a.css":   true,
		"https://example.com/a.css":  true,
		"mailto:someone@example.com": true,
		"/absolute/path.xhtml":       true,
		"//host/path.xhtml":          true,
	}
	for in, want := range tests {
		if got := IsExternalURI(in); got != want {
			t.Errorf("IsExternalURI(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestResolveHref(t *testing.T) {
	tests := []struct {
		name    string
		opfPath string
		href    string
		want    string
		wantOK  bool
	}{
		{name: "relative", opfPath: "OEBPS/package.opf", href: "Text/ch1.xhtml", want: "OEBPS/Text/ch1.xhtml", wantOK: true},
		{name: "fragment stripped", opfPath: "OEBPS/package.opf", href: "Text/ch1.xhtml#sec", want: "OEBPS/Text/ch1.xhtml", wantOK: true},
		{name: "parent dir", opfPath: "OEBPS/package.opf", href: "../shared/a.css", want: "shared/a.css", wantOK: true},
		{name: "percent-encoded", opfPath: "OEBPS/package.opf", href: "Images/cover%20art.png", want: "OEBPS/Images/cover art.png", wantOK: true},
		{name: "root opf", opfPath: "package.opf", href: "ch1.xhtml", want: "ch1.xhtml", wantOK: true},
		{name: "empty href", opfPath: "OEBPS/package.opf", href: ""},
		{name: "fragment only", opfPath: "OEBPS/package.opf", href: "#top"},
		{name: "absolute", opfPath: "OEBPS/package.opf", href: "/etc/passwd"},
		{name: "external", opfPath: "OEBPS/package.opf", href: "https://example.com/x.css"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ResolveHref(tc.opfPath, tc.href)
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("ResolveHref(%q, %q) = (%q, %v), want (%q, %v)", tc.opfPath, tc.href, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestHasNavProps(t *testing.T) {
	tests := map[string]bool{
		"nav":           true,
		"scripted nav":  true,
		"nav  scripted": true,
		"":              false,
		"navigation":    false,
		"cover-image":   false,
	}
	for in, want := range tests {
		if got := HasNavProps(in); got != want {
			t.Errorf("HasNavProps(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseEncryption(t *testing.T) {
	const enc = `<?xml version="1.0" encoding="UTF-8"?>
<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"
            xmlns:enc="http://www.w3.org/2001/04/xmlenc#">
  <enc:EncryptedData>
    <enc:EncryptionMethod Algorithm="http://www.idpf.org/2008/embedding"/>
    <enc:CipherData>
      <enc:CipherReference URI="/OEBPS/Fonts/Source%20Han.otf"/>
    </enc:CipherData>
  </enc:EncryptedData>
  <enc:EncryptedData>
    <enc:EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#aes128-cbc"/>
    <enc:CipherData>
      <enc:CipherReference URI="OEBPS/Text/ch1.xhtml"/>
      <enc:CipherReference URI="OEBPS/Text/ch2.xhtml#frag"/>
    </enc:CipherData>
  </enc:EncryptedData>
</encryption>`
	recs, err := ParseEncryption([]byte(enc))
	if err != nil {
		t.Fatalf("ParseEncryption error = %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("len(recs) = %d, want 2", len(recs))
	}
	r0 := recs[0]
	if r0.Algorithm != "http://www.idpf.org/2008/embedding" {
		t.Errorf("recs[0].Algorithm = %q", r0.Algorithm)
	}
	if len(r0.RawTargets) != 1 || r0.RawTargets[0] != "/OEBPS/Fonts/Source%20Han.otf" {
		t.Errorf("recs[0].RawTargets = %q", r0.RawTargets)
	}
	if len(r0.Targets) != 1 || r0.Targets[0] != "OEBPS/Fonts/Source Han.otf" {
		t.Errorf("recs[0].Targets = %q", r0.Targets)
	}
	r1 := recs[1]
	if r1.Algorithm != "http://www.w3.org/2001/04/xmlenc#aes128-cbc" {
		t.Errorf("recs[1].Algorithm = %q", r1.Algorithm)
	}
	if got := strings.Join(r1.Targets, "|"); got != "OEBPS/Text/ch1.xhtml|OEBPS/Text/ch2.xhtml" {
		t.Errorf("recs[1].Targets = %q", got)
	}

	// 没有 EncryptedData 时返回空（nil）记录且无错误。
	none, err := ParseEncryption([]byte(`<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"/>`))
	if err != nil || len(none) != 0 {
		t.Errorf("ParseEncryption(empty) = %v, %v", none, err)
	}
	if _, err := ParseEncryption([]byte(`<encryption><EncryptedData>`)); err == nil {
		t.Error("ParseEncryption(malformed) error = nil, want error")
	}
}

func TestEncryptionTargetPath(t *testing.T) {
	tests := map[string]string{
		"/OEBPS/Fonts/a.otf":           "OEBPS/Fonts/a.otf",
		"OEBPS/Fonts/a.otf":            "OEBPS/Fonts/a.otf",
		"OEBPS/Fonts/a%20b.otf":        "OEBPS/Fonts/a b.otf",
		"OEBPS/Fonts/a.otf?x=1#frag":   "OEBPS/Fonts/a.otf",
		"OEBPS/./Fonts/../Fonts/a.otf": "OEBPS/Fonts/a.otf",
		"OEBPS//Fonts/a.otf":           "OEBPS/Fonts/a.otf",
	}
	for in, want := range tests {
		if got := EncryptionTargetPath(in); got != want {
			t.Errorf("EncryptionTargetPath(%q) = %q, want %q", in, got, want)
		}
	}
}
