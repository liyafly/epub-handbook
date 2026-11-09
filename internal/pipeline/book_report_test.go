package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFontWarningSummaryReportsWorstFontOnce(t *testing.T) {
	data := []byte(`{"facts":{"epub.font.subset.providerReport":{"fonts":[
	  {"target":"OEBPS/Fonts/a.ttf","notInMasterCount":30,"notInMasterBodyCount":20,"warnings":[
	    "OEBPS/Fonts/a.ttf: 30 required characters are not in the master font (20 in body text, 10 from CSS/config/generated text; fallback fonts must cover them)",
	    "OEBPS/Fonts/a.ttf: source coverage warning, first 20 of 30: …",
	    "shared deprecation notice"]},
	  {"target":"OEBPS/Fonts/b.ttf","notInMasterCount":5,"notInMasterBodyCount":5,"warnings":["shared deprecation notice"]},
	  {"target":"OEBPS/Fonts/c.ttf","notInMasterCount":0,"notInMasterBodyCount":0,"warnings":[]}
	]}}}`)
	lines := fontWarningSummary(data)
	if len(lines) != 2 {
		t.Fatalf("want summary + one deduplicated warning, got %q", lines)
	}
	if want := "fonts: 2 of 3 fonts lack body characters (most: OEBPS/Fonts/a.ttf, 20 in body + 10 from CSS/config/generated text)"; !strings.HasPrefix(lines[0], want) {
		t.Errorf("summary = %q, want prefix %q", lines[0], want)
	}
	if lines[1] != "shared deprecation notice" {
		t.Errorf("other warnings must be kept once: %q", lines[1])
	}
}

func TestFontWarningSummaryBodyCovered(t *testing.T) {
	data := []byte(`{"facts":{"epub.font.subset.providerReport":{"fonts":[
	  {"target":"OEBPS/Fonts/a.ttf","notInMasterCount":4,"notInMasterBodyCount":0,"warnings":[]}]}}}`)
	lines := fontWarningSummary(data)
	if len(lines) != 1 || !strings.Contains(lines[0], "body text fully covered") || strings.Contains(lines[0], "fallback") {
		t.Errorf("body-covered summary = %q", lines)
	}
}

func TestFontWarningSummaryOlderProvider(t *testing.T) {
	data := []byte(`{"facts":{"epub.font.subset.providerReport":{"fonts":[
	  {"target":"OEBPS/Fonts/a.ttf","notInMasterCount":7,"warnings":[]}]}}}`)
	lines := fontWarningSummary(data)
	if len(lines) != 1 || !strings.Contains(lines[0], "lack characters (most: OEBPS/Fonts/a.ttf, 7)") {
		t.Errorf("summary without body count = %q", lines)
	}
}

func TestSizeHint(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "small.epub")
	mustWrite(t, small, "x")
	if hint := sizeHint(small); hint != "" {
		t.Errorf("small file must not warn: %q", hint)
	}
	big := filepath.Join(dir, "big.epub")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(kindleSizeHint + 1); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if hint := sizeHint(big); !strings.Contains(hint, "50 MB") {
		t.Errorf("large file must warn: %q", hint)
	}
}

func TestBookBuildWritesSummaryReport(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("build = %d: %s", code, h.err)
	}
	page := mustRead(t, filepath.Join(dir, ".epub", reportHTMLName))
	for _, want := range []string{"Build passed", "epub.package.nav.audit", "redline"} {
		if !strings.Contains(page, want) {
			t.Errorf("report.html should mention %q", want)
		}
	}
	if !strings.Contains(h.out.String(), "Summary: "+filepath.Join(".epub", reportHTMLName)) {
		t.Errorf("build should point to the summary: %s", h.out)
	}
	// 第二次构建与上一版成品比对，只打一行。
	if code := h.run(dir, "build"); code != 0 {
		t.Fatalf("rebuild = %d: %s", code, h.err)
	}
	if !strings.Contains(h.out.String(), "INFO vs previous book.epub: no red-line differences") {
		t.Errorf("rebuild should summarise the previous-dist comparison: %s", h.out)
	}
	if strings.Contains(h.out.String(), "All requested red-line checks passed") {
		t.Errorf("previous-dist redline text must stay in its report: %s", h.out)
	}
}

func TestBookBuildFailureListsFindingsNotEnvelope(t *testing.T) {
	h := newBookHarness(t)
	dir := h.newBook("")
	mustWrite(t, filepath.Join(dir, "variants/bad/OEBPS/package.opf"), "<not-a-package>")
	setVariants(t, dir, map[string]variantConfig{"bad": {Overlay: "variants/bad"}})
	if code := h.run(dir, "build"); code == 0 {
		t.Fatal("a broken variant must fail the build")
	}
	if strings.Contains(h.err.String(), `"schemaVersion"`) {
		t.Errorf("failure output must not dump the JSON envelope: %s", h.err)
	}
	if !strings.Contains(h.err.String(), "FAIL ") {
		t.Errorf("failure must name the failing step: %s", h.err)
	}
	builds, _ := filepath.Glob(filepath.Join(dir, ".epub", "build.*", reportHTMLName))
	if len(builds) != 1 || !strings.Contains(mustRead(t, builds[0]), "Build failed") {
		t.Errorf("the retained failure directory must include report.html: %v", builds)
	}
}

func TestFixedRoleWarningLines(t *testing.T) {
	lines, err := fixedRoleWarningLines([]byte(`{"schema_version":"1.0","fixed_role_missing":[]}`))
	if err != nil || len(lines) != 0 {
		t.Fatalf("no missing characters means no warning: %q, %v", lines, err)
	}

	lines, err = fixedRoleWarningLines([]byte(`{"fixed_role_missing":[
	  {"char":"缺","codepoint":"U+7F3A","family":"tszt-letter","font":"OEBPS/Fonts/letter.ttf","count":2,"first_location":"a.xhtml"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"缺 U+7F3A", "tszt-letter", "used 2", "a.xhtml", "real readers"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warning lines lack %q: %q", want, lines)
		}
	}
}

func TestFixedRoleWarningLinesCapsList(t *testing.T) {
	var entries []string
	for range maxFixedRoleLines + 5 {
		entries = append(entries, `{"char":"缺","codepoint":"U+7F3A","family":"a","font":"f","count":1,"first_location":"x"}`)
	}
	lines, err := fixedRoleWarningLines([]byte(`{"fixed_role_missing":[` + strings.Join(entries, ",") + `]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "and 5 more") || len(lines) != maxFixedRoleLines+3 {
		t.Errorf("list not capped: %d lines: %q", len(lines), lines)
	}
}

// 旧版 provider 不认识结果字段时要说明检查没跑，不能当成没有缺字。
func TestFixedRoleWarningLinesRejectsOldOrBadProvider(t *testing.T) {
	for _, data := range []string{`{"schema_version":"1.0"}`, `not json`, ``} {
		if lines, err := fixedRoleWarningLines([]byte(data)); err == nil {
			t.Errorf("%q must be an error, got %q", data, lines)
		}
	}
}
