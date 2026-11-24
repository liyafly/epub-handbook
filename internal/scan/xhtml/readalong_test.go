package xhtml

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const readAlongDoc = `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>One</title></head><body>
<section epub:type="chapter">
<h1 class="chapter-head"><img src="../Images/h.png" alt="Chapter One"/></h1>
<p class="first"><span class="dropcap">M</span><span class="smallcaps">r Jones</span> locked the hen-houses. <span epub:type="pagebreak" id="p2"></span>He was &amp; drunk! ‘Comrades,’ he said. ‘Now?’ No.</p>
<!-- a comment. Not a sentence. -->
<p>One <em>sentence. Inside</em> emphasis stays whole.</p>
<figure id="fig-1"><img src="a.png" alt=""/><figcaption><a epub:type="noteref" href="#note-1">Transcription</a></figcaption></figure>
<figure><img src="b.png" alt="plain"/><p>Caption text.</p></figure>
<p><span epub:type="pagebreak" id="p3"></span></p>
<aside epub:type="footnotes"><aside id="note-1" epub:type="footnote"><div class="verse"><p><a epub:type="backlink" href="#fig-1">◎</a>Beasts of England,<br/>beasts of Ireland</p></div></aside></aside>
</section></body></html>`

func applyInsertions(doc string, ins []Insertion) string {
	sorted := slices.Clone(ins)
	slices.SortStableFunc(sorted, func(a, b Insertion) int { return a.Offset - b.Offset })
	var out strings.Builder
	last := 0
	for _, in := range sorted {
		out.WriteString(doc[last:in.Offset])
		out.WriteString(in.Text)
		last = in.Offset
	}
	out.WriteString(doc[last:])
	return out.String()
}

func TestPlanReadAlongSentences(t *testing.T) {
	plan, err := PlanReadAlong(readAlongDoc, "mo-c1-", false)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range plan.Segments {
		got = append(got, s.ID+"|"+s.Text)
	}
	want := []string{
		"mo-c1-h1|Chapter One",
		"mo-c1-s2|Mr Jones locked the hen-houses.",
		"mo-c1-s3|He was & drunk!",
		"mo-c1-s4|‘Comrades,’ he said.",
		"mo-c1-s5|‘Now?’",
		"mo-c1-s6|No.",
		"mo-c1-s7|One sentence. Inside emphasis stays whole.",
		"fig-1|Beasts of England, beasts of Ireland",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("segments:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !plan.Segments[7].Figure {
		t.Fatal("figure segment not flagged")
	}
	out := applyInsertions(readAlongDoc, plan.Insertions)
	for _, frag := range []string{
		`<h1 class="chapter-head" id="mo-c1-h1">`,
		`<p class="first"><span id="mo-c1-s2"><span class="dropcap">M</span><span class="smallcaps">r Jones</span> locked the hen-houses.</span> <span id="mo-c1-s3"><span epub:type="pagebreak" id="p2"></span>He was &amp; drunk!</span>`,
		`<p><span id="mo-c1-s7">One <em>sentence. Inside</em> emphasis stays whole.</span></p>`,
		`<p><span epub:type="pagebreak" id="p3"></span></p>`,
		`<!-- a comment. Not a sentence. -->`,
	} {
		if !strings.Contains(out, frag) {
			t.Errorf("output missing %q", frag)
		}
	}
	if plainText(out) != plainText(readAlongDoc) {
		t.Fatal("insertions changed the text")
	}
}

func TestPlanReadAlongParagraphs(t *testing.T) {
	plan, err := PlanReadAlong(readAlongDoc, "mo-c1-", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Segments) != 4 || plan.Segments[1].Text != "Mr Jones locked the hen-houses. He was & drunk! ‘Comrades,’ he said. ‘Now?’ No." {
		t.Fatalf("paragraph mode segments: %+v", plan.Segments)
	}
}

func TestPlanReadAlongIDConflict(t *testing.T) {
	doc := strings.Replace(readAlongDoc, `id="p2"`, `id="mo-c1-x"`, 1)
	_, err := PlanReadAlong(doc, "mo-c1-", false)
	var conflict *ErrReadAlongIDConflict
	if !errors.As(err, &conflict) || !slices.Equal(conflict.IDs, []string{"mo-c1-x"}) {
		t.Fatalf("conflict error = %v", err)
	}
}

func TestPlanReadAlongTruncated(t *testing.T) {
	if _, err := PlanReadAlong(`<html><body><p>open <!-- never closed`, "mo-", false); err == nil {
		t.Fatal("truncated markup must fail")
	}
}
