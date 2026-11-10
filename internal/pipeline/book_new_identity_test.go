package pipeline

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var starterUUIDPattern = regexp.MustCompile(`^urn:uuid:[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func starterIDs(t *testing.T, dir string) (opfID, ncxID string) {
	t.Helper()
	opf := mustRead(t, filepath.Join(dir, "epub/OEBPS/package.opf"))
	ncx := mustRead(t, filepath.Join(dir, "epub/OEBPS/toc.ncx"))
	m := regexp.MustCompile(`<dc:identifier id="bookid">([^<]*)</dc:identifier>`).FindStringSubmatch(opf)
	n := regexp.MustCompile(`<meta name="dtb:uid" content="([^"]*)"/>`).FindStringSubmatch(ncx)
	if m == nil || n == nil {
		t.Fatalf("identifier not found: opf=%v ncx=%v", m, n)
	}
	return m[1], n[1]
}

func TestBookNewUniqueIdentifier(t *testing.T) {
	h := newBookHarness(t)
	a, b := h.newBook(""), h.newBook("")
	aOPF, aNCX := starterIDs(t, a)
	bOPF, bNCX := starterIDs(t, b)
	if aOPF != aNCX || bOPF != bNCX {
		t.Errorf("OPF/NCX identifiers differ: %s/%s, %s/%s", aOPF, aNCX, bOPF, bNCX)
	}
	if aOPF == bOPF {
		t.Errorf("two new books share identifier %s", aOPF)
	}
	for _, id := range []string{aOPF, bOPF} {
		if !starterUUIDPattern.MatchString(id) || id == starterIdentifier {
			t.Errorf("identifier %q is not a fresh urn:uuid v4", id)
		}
	}
	opf := mustRead(t, filepath.Join(a, "epub/OEBPS/package.opf"))
	if strings.Contains(opf, starterModified) || !regexp.MustCompile(`<meta property="dcterms:modified">\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ</meta>`).MatchString(opf) {
		t.Errorf("dcterms:modified not refreshed")
	}
	if !strings.Contains(opf, starterTitle) || !strings.Contains(opf, starterAuthor) {
		t.Error("without --title/--author the placeholders must stay")
	}
}

func TestBookNewTitleAuthorEscaped(t *testing.T) {
	h := newBookHarness(t)
	dir := filepath.Join(t.TempDir(), "book")
	opts := BookNewOptions{Dir: dir, Title: "猫 & <狗>", Author: "某 & 某"}
	if code := BookNew(context.Background(), opts, h.io()); code != 0 {
		t.Fatalf("book new = %d: %s", code, h.err)
	}
	for _, rel := range []string{"epub/OEBPS/package.opf", "epub/OEBPS/toc.ncx", "epub/OEBPS/Text/00-title.xhtml"} {
		data := mustRead(t, filepath.Join(dir, rel))
		if strings.Contains(data, starterTitle) {
			t.Errorf("%s still has the title placeholder", rel)
		}
		if !strings.Contains(data, "猫 &amp; &lt;狗&gt;") {
			t.Errorf("%s lacks escaped title", rel)
		}
		dec := xml.NewDecoder(strings.NewReader(data))
		dec.Strict = true
		dec.Entity = xml.HTMLEntity
		for {
			if _, err := dec.Token(); err != nil {
				if err.Error() != "EOF" {
					t.Errorf("%s is not well-formed: %v", rel, err)
				}
				break
			}
		}
	}
	opf := mustRead(t, filepath.Join(dir, "epub/OEBPS/package.opf"))
	if !strings.Contains(opf, "<dc:creator>某 &amp; 某</dc:creator>") {
		t.Error("author not written to OPF")
	}
	var cfg bookConfig
	if err := json.Unmarshal([]byte(mustRead(t, filepath.Join(dir, "book.json"))), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Title != "猫 & <狗>" {
		t.Errorf("book.json title = %q", cfg.Title)
	}
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s %s", code, h.out, h.err)
	}
}

func TestPersonalizeStarterRequiresSingleMatch(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "x x")
	mustWrite(t, filepath.Join(dir, "b.txt"), "y")
	err := personalizeStarter(dir, []starterReplacement{{"b.txt", "y", "z"}, {"a.txt", "x", "z"}})
	if err == nil {
		t.Fatal("double match must fail")
	}
	if got := mustRead(t, filepath.Join(dir, "b.txt")); got != "y" {
		t.Errorf("failed personalization wrote b.txt = %q", got)
	}
	if err := personalizeStarter(dir, []starterReplacement{{"b.txt", "missing", "z"}}); err == nil {
		t.Fatal("zero match must fail")
	}
}
