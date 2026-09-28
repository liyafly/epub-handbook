package pipeline

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/report"
)

func TestKindleCompatibilityCheckEndToEnd(t *testing.T) {
	// nav.audit records whether the EPUBCheck executable is available. Keep the
	// golden stable across developer machines by isolating this read-only probe.
	t.Setenv("PATH", t.TempDir())
	input := writeNewCapabilityEPUB(t)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.kindle.compatibility.check",
		InputPath:    input,
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitOK || outcome.Envelope.Status != report.StatusComplete {
		t.Fatalf("status=%q exit=%d findings=%+v, want complete / 0", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	var sawKindle, sawNavAudit, sawRedline bool
	for _, event := range outcome.Envelope.Events {
		switch event.Step {
		case "epub.kindle.compatibility.check":
			sawKindle = event.Status == "completed"
		case "epub.package.nav.audit":
			sawNavAudit = event.Status == "completed"
		case "redline":
			sawRedline = true
		}
	}
	if !sawKindle || !sawNavAudit || sawRedline {
		t.Fatalf("events=%+v, want nav audit + read-only Kindle check without redline", outcome.Envelope.Events)
	}
	if outcome.Envelope.Facts["epub.kindle.compatibility.check.staticOnly"] != true {
		t.Fatalf("Kindle facts=%#v, want staticOnly=true", outcome.Envelope.Facts)
	}
	if _, ok := outcome.Envelope.Facts["epub.kindle.compatibility.check.counts"]; !ok {
		t.Fatalf("missing Kindle counts fact: %#v", outcome.Envelope.Facts)
	}

	root := repoRootForTest(t)
	goldenPath := filepath.Join(root, "testdata", "kindle_check", "basic.report.json")
	if outcome.Envelope.Input == nil {
		t.Fatal("input artifact missing from E2E envelope")
	}
	outcome.Envelope.Input.Path = "<fixture.epub>"
	outcome.Envelope.Input.SHA256 = ""
	data, err := json.MarshalIndent(outcome.Envelope, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("Kindle E2E envelope differs from golden\n--- got ---\n%s\n--- want ---\n%s", data, want)
	}
}

func TestNotesFallbackEndToEnd(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	input := writeNotesFallbackEPUB(t, false, false)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.notes.legacy-fallback",
		InputPath:    input,
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitOK || outcome.Envelope.Status != report.StatusPlanned {
		t.Fatalf("status=%q exit=%d findings=%+v, want planned / 0", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	if got := outcome.Envelope.Facts["epub.notes.legacy-fallback.editCount"]; got != 3 {
		t.Fatalf("editCount=%#v, want 3", got)
	}
	if got := outcome.Envelope.Facts["epub.notes.popup.normalize.status"]; got != report.StatusComplete {
		t.Fatalf("popup status=%#v, want complete", got)
	}
	if got := outcome.Envelope.Facts["epub.notes.popup.normalize.findingsByLevel"]; got != (upstreamFindingsByLevel{}) {
		t.Fatalf("popup findingsByLevel=%#v, want zero counts", got)
	}
	if got := outcome.Envelope.Facts["pipeline.modifiedEntries"]; !slices.Equal(got.([]string), []string{"OEBPS/Text/chapter.xhtml"}) {
		t.Fatalf("pipeline.modifiedEntries=%#v", got)
	}
	var sawFallback, sawPopup, sawRedline bool
	for _, event := range outcome.Envelope.Events {
		switch event.Step {
		case "epub.notes.legacy-fallback":
			sawFallback = event.Status == "completed"
		case "epub.notes.popup.normalize":
			sawPopup = event.Status == "completed"
		case "redline":
			sawRedline = event.Status == "completed"
		}
	}
	if !sawFallback || !sawPopup || !sawRedline {
		t.Fatalf("events=%+v, want popup, fallback, and redline completion", outcome.Envelope.Events)
	}
	root := repoRootForTest(t)
	goldenPath := filepath.Join(root, "testdata", "notes_fallback", "basic.report.json")
	if outcome.Envelope.Input == nil {
		t.Fatal("input artifact missing from E2E envelope")
	}
	outcome.Envelope.Input.Path = "<fixture.epub>"
	outcome.Envelope.Input.SHA256 = ""
	for i, command := range outcome.Envelope.NextCommands {
		outcome.Envelope.NextCommands[i] = strings.ReplaceAll(command, input, "fixture.epub")
	}
	data, err := json.MarshalIndent(outcome.Envelope, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("notes fallback E2E envelope differs from golden\n--- got ---\n%s\n--- want ---\n%s", data, want)
	}
}

func TestNotesFallbackCompletesPartialDuokanClasses(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	input := writeNotesFallbackEPUB(t, false, true)
	dryRun, err := Run(t.Context(), Options{
		CapabilityID: "epub.notes.legacy-fallback",
		InputPath:    input,
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if dryRun.ExitCode != ExitOK || dryRun.Envelope.Status != report.StatusPlanned {
		t.Fatalf("dry-run status=%q exit=%d findings=%+v, want planned / 0", dryRun.Envelope.Status, dryRun.ExitCode, dryRun.Envelope.Findings)
	}
	if got := dryRun.Envelope.Facts["epub.notes.legacy-fallback.editCount"]; got != 2 {
		t.Fatalf("dry-run editCount=%#v, want only ol/li class additions", got)
	}
	if got := dryRun.Envelope.Facts["pipeline.modifiedEntries"]; !slices.Equal(got.([]string), []string{"OEBPS/Text/chapter.xhtml"}) {
		t.Fatalf("dry-run modifiedEntries=%#v", got)
	}

	captured, err := Run(t.Context(), Options{
		CapabilityID:  "epub.notes.legacy-fallback",
		InputPath:     input,
		CaptureOutput: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.ExitCode != ExitOK || captured.Envelope.Status != report.StatusComplete || len(captured.OutputBytes) == 0 {
		t.Fatalf("capture status=%q exit=%d outputBytes=%d findings=%+v", captured.Envelope.Status, captured.ExitCode, len(captured.OutputBytes), captured.Envelope.Findings)
	}
	validated, err := Run(t.Context(), Options{
		CapabilityID: "epub.notes.popup.normalize",
		InputPath:    "partial-duokan-output.epub",
		InputBytes:   captured.OutputBytes,
	})
	if err != nil {
		t.Fatal(err)
	}
	if validated.ExitCode != ExitOK || validated.Envelope.Status != report.StatusComplete || validated.Envelope.Facts["epub.notes.popup.normalize.violations"] != 0 {
		t.Fatalf("popup output validation status=%q exit=%d facts=%#v findings=%+v", validated.Envelope.Status, validated.ExitCode, validated.Envelope.Facts, validated.Envelope.Findings)
	}
}

func TestNotesFallbackRejectsInvalidPopupUpstream(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	input := writeNotesFallbackEPUB(t, true, false)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.notes.legacy-fallback",
		InputPath:    input,
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitFailed || outcome.Envelope.Status != report.StatusFailed {
		t.Fatalf("status=%q exit=%d facts=%#v findings=%+v events=%+v, want failed / 1", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Facts, outcome.Envelope.Findings, outcome.Envelope.Events)
	}
	if !slices.ContainsFunc(outcome.Envelope.Findings, func(finding report.Finding) bool {
		return finding.ID == "notes-fallback.upstream-not-clean"
	}) {
		t.Fatalf("missing upstream-not-clean finding: %+v", outcome.Envelope.Findings)
	}
	if got := outcome.Envelope.Facts["pipeline.modifiedEntries"]; len(got.([]string)) != 0 {
		t.Fatalf("pipeline.modifiedEntries=%#v, want []", got)
	}
}

func TestNotesFallbackRejectsUnscannedManifestXHTML(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	input := writeNotesFallbackOutsideTextEPUB(t)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.notes.legacy-fallback",
		InputPath:    input,
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitFailed || outcome.Envelope.Status != report.StatusFailed {
		t.Fatalf("status=%q exit=%d facts=%#v findings=%+v, want failed / 1", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Facts, outcome.Envelope.Findings)
	}
	if got := outcome.Envelope.Facts["epub.notes.popup.normalize.status"]; got != report.StatusFailed {
		t.Fatalf("popup status=%#v, want failed", got)
	}
	if got := outcome.Envelope.Facts["pipeline.modifiedEntries"]; len(got.([]string)) != 0 {
		t.Fatalf("pipeline.modifiedEntries=%#v, want []", got)
	}
	if !slices.ContainsFunc(outcome.Envelope.Findings, func(finding report.Finding) bool {
		return finding.ID == "notes-fallback.upstream-not-clean"
	}) {
		t.Fatalf("missing upstream-not-clean finding: %+v", outcome.Envelope.Findings)
	}
}

func TestEnglishTypographyEndToEnd(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	input := writeEnglishTypographyEPUB(t)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.typography.english.optimize",
		InputPath:    input,
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitOK || outcome.Envelope.Status != report.StatusPlanned {
		t.Fatalf("status=%q exit=%d findings=%+v, want planned / 0", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	if got := outcome.Envelope.Facts["epub.typography.english.optimize.editCount"]; got != 0 {
		t.Fatalf("editCount=%#v, want 0 until scope is explicit", got)
	}
	if got := outcome.Envelope.Facts["pipeline.modifiedEntries"]; len(got.([]string)) != 0 {
		t.Fatalf("pipeline.modifiedEntries=%#v, want []", got)
	}
	var sawScopeRequirement bool
	for _, finding := range outcome.Envelope.Findings {
		sawScopeRequirement = sawScopeRequirement || finding.ID == "english.opf-language-differs-requires-scope"
	}
	if !sawScopeRequirement {
		t.Fatalf("expected OPF language scope requirement finding, got %+v", outcome.Envelope.Findings)
	}
	if outcome.Envelope.Input == nil {
		t.Fatal("input artifact missing from E2E envelope")
	}
	outcome.Envelope.Input.Path = "<fixture.epub>"
	outcome.Envelope.Input.SHA256 = ""
	for i, command := range outcome.Envelope.NextCommands {
		outcome.Envelope.NextCommands[i] = strings.ReplaceAll(command, input, "fixture.epub")
	}
	data, err := json.MarshalIndent(outcome.Envelope, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	goldenPath := filepath.Join(repoRootForTest(t), "testdata", "english_typography", "basic.report.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("English typography E2E envelope differs from golden\n--- got ---\n%s\n--- want ---\n%s", data, want)
	}
}

func TestEnglishTypographyScopeOverridesOPFLanguageGate(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	input := writeEnglishTypographyEPUB(t)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.typography.english.optimize",
		InputPath:    input,
		DryRun:       true,
		Args: Args{
			"scope_paths": `["OEBPS/Text/english-1.xhtml","OEBPS/Text/english-2.xhtml","OEBPS/Text/english-3.xhtml"]`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitOK || outcome.Envelope.Status != report.StatusPlanned {
		t.Fatalf("status=%q exit=%d findings=%+v, want planned / 0", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	if got := outcome.Envelope.Facts["epub.typography.english.optimize.editCount"]; got != 3 {
		t.Fatalf("editCount=%#v, want 3", got)
	}
	if got := outcome.Envelope.Facts["pipeline.modifiedEntries"]; !slices.Equal(got.([]string), []string{
		"OEBPS/Text/english-1.xhtml", "OEBPS/Text/english-2.xhtml", "OEBPS/Text/english-3.xhtml",
	}) {
		t.Fatalf("pipeline.modifiedEntries=%#v", got)
	}
	if !slices.ContainsFunc(outcome.Envelope.Findings, func(finding report.Finding) bool {
		return finding.ID == "english.opf-language-differs"
	}) {
		t.Fatalf("missing scoped OPF-language diagnostic: %+v", outcome.Envelope.Findings)
	}
	if slices.ContainsFunc(outcome.Envelope.Findings, func(finding report.Finding) bool {
		return finding.ID == "english.opf-language-differs-requires-scope"
	}) {
		t.Fatalf("explicit scope should bypass requires-scope info: %+v", outcome.Envelope.Findings)
	}
}

func TestTypographyPresetMinifiedXHTMLEndToEnd(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	input := writeNewCapabilityEPUB(t)
	before := readPipelineEPUBEntry(t, input, "OEBPS/chapter.xhtml")
	bodyStart := bytes.Index(before, []byte("<body"))
	bodyClose := bytes.Index(before, []byte("</body>"))
	if bodyStart < 0 || bodyClose < bodyStart {
		t.Fatal("fixture body is missing")
	}
	body := bytes.Clone(before[bodyStart : bodyClose+len("</body>")])
	output := filepath.Join(t.TempDir(), "candidate.epub")
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.typography.optimize",
		InputPath:    input,
		OutputPath:   output,
		Args:         Args{"preset": "literary-cn"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitOK || outcome.Envelope.Status != report.StatusComplete {
		t.Fatalf("status=%q exit=%d findings=%+v", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	var sawTypography, sawAudit, sawRedline bool
	for _, event := range outcome.Envelope.Events {
		switch event.Step {
		case "epub.typography.optimize":
			sawTypography = event.Status == "completed"
		case "epub.package.nav.audit":
			sawAudit = event.Status == "completed"
		case "redline":
			sawRedline = event.Status == "completed" && event.Message == "0 findings"
		}
	}
	if !sawTypography || !sawAudit || !sawRedline {
		t.Fatalf("events=%+v, want typography, nav audit, and clean redline", outcome.Envelope.Events)
	}
	chapter := readPipelineEPUBEntry(t, output, "OEBPS/chapter.xhtml")
	gotBodyStart := bytes.Index(chapter, []byte("<body"))
	gotBodyClose := bytes.Index(chapter, []byte("</body>"))
	if gotBodyStart < 0 || gotBodyClose < gotBodyStart || !bytes.Equal(chapter[gotBodyStart:gotBodyClose+len("</body>")], body) {
		t.Fatalf("body bytes changed:\n%s", chapter)
	}
	for _, layer := range []string{"fonts.css", "base.css", "notes.css", "effects.css", "literary.css", "media.css"} {
		link := []byte(`href="Styles/` + layer + `"`)
		if count := bytes.Count(chapter, link); count != 1 {
			t.Fatalf("stylesheet %s link count = %d, want 1:\n%s", layer, count, chapter)
		}
	}

	if outcome.Envelope.Input == nil || outcome.Envelope.Output == nil {
		t.Fatal("input/output artifacts missing from E2E envelope")
	}
	outcome.Envelope.Input.Path, outcome.Envelope.Input.SHA256 = "<fixture.epub>", ""
	outcome.Envelope.Output.Path, outcome.Envelope.Output.SHA256 = "<candidate.epub>", ""
	for i, command := range outcome.Envelope.NextCommands {
		outcome.Envelope.NextCommands[i] = strings.ReplaceAll(command, input, "fixture.epub")
		outcome.Envelope.NextCommands[i] = strings.ReplaceAll(outcome.Envelope.NextCommands[i], output, "<candidate.epub>")
	}
	for i, event := range outcome.Envelope.Events {
		outcome.Envelope.Events[i].Message = strings.ReplaceAll(event.Message, output, "<candidate.epub>")
	}
	data, err := json.MarshalIndent(outcome.Envelope, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	goldenPath := filepath.Join(repoRootForTest(t), "testdata", "typography_preset", "minified.report.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("typography preset E2E envelope differs from golden\n--- got ---\n%s\n--- want ---\n%s", data, want)
	}
}

func readPipelineEPUBEntry(t *testing.T, epubPath, target string) []byte {
	t.Helper()
	archive, err := zip.OpenReader(epubPath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, entry := range archive.File {
		if entry.Name != target {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		return data
	}
	t.Fatalf("EPUB entry %q not found", target)
	return nil
}

func TestEnglishTypographyInvalidLanguageIsUsageError(t *testing.T) {
	input := writeEnglishTypographyEPUB(t)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.typography.english.optimize",
		InputPath:    input,
		DryRun:       true,
		Args:         Args{"lang": "en_US"},
	})
	if err == nil || outcome.ExitCode != ExitUsage || outcome.Envelope.Status != report.StatusFailed {
		t.Fatalf("outcome=%+v err=%v, want usage / exit 3", outcome, err)
	}
}

func TestVerticalRubyEndToEndAndRedline(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	input := writeVerticalRubyEPUB(t)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.vertical.ruby.optimize",
		InputPath:    input,
		DryRun:       true,
		Args:         Args{"op": "ruby-rp"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitOK || outcome.Envelope.Status != report.StatusPlanned {
		t.Fatalf("status=%q exit=%d findings=%+v, want planned / 0", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	if got := outcome.Envelope.Facts["epub.vertical.ruby.optimize.editCount"]; got != 2 {
		t.Fatalf("editCount=%#v, want two insertions for the ruby-rp pair", got)
	}
	if got := outcome.Envelope.Facts["pipeline.modifiedEntries"]; !slices.Equal(got.([]string), []string{"OEBPS/chapter.xhtml"}) {
		t.Fatalf("pipeline.modifiedEntries=%#v", got)
	}
	var redlineComplete bool
	for _, event := range outcome.Envelope.Events {
		if event.Step == "redline" {
			redlineComplete = event.Status == "completed"
		}
	}
	if !redlineComplete {
		t.Fatalf("redline did not complete: %+v", outcome.Envelope.Events)
	}
	if outcome.Envelope.Input == nil {
		t.Fatal("input artifact missing from E2E envelope")
	}
	outcome.Envelope.Input.Path = "<fixture.epub>"
	outcome.Envelope.Input.SHA256 = ""
	for i, command := range outcome.Envelope.NextCommands {
		outcome.Envelope.NextCommands[i] = strings.ReplaceAll(command, input, "fixture.epub")
	}
	data, err := json.MarshalIndent(outcome.Envelope, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	goldenPath := filepath.Join(repoRootForTest(t), "testdata", "vertical_ruby", "basic.report.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("vertical Ruby E2E envelope differs from golden\n--- got ---\n%s\n--- want ---\n%s", data, want)
	}
}

func TestVerticalRubyInvalidParametersAreUsageErrors(t *testing.T) {
	input := writeVerticalRubyEPUB(t)
	for _, args := range []Args{
		{},
		{"op": "unknown"},
		{"op": "ruby-rp", "rp_open": "<"},
		{"op": "ruby-rp", "rp_open": "\x01"},
	} {
		outcome, err := Run(t.Context(), Options{
			CapabilityID: "epub.vertical.ruby.optimize",
			InputPath:    input,
			DryRun:       true,
			Args:         args,
		})
		if err == nil || outcome.ExitCode != ExitUsage || outcome.Envelope.Status != report.StatusFailed {
			t.Fatalf("args=%#v outcome=%+v err=%v, want usage / exit 3", args, outcome, err)
		}
	}
}

func writeVerticalRubyEPUB(t *testing.T) string {
	t.Helper()
	entries := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":      `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid"><metadata><dc:title>Vertical Ruby fixture</dc:title><dc:identifier id="uid">urn:uuid:vertical-ruby</dc:identifier><dc:language>ja</dc:language><meta name="cover" content="cover"/></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/><item id="style" href="vertical.css" media-type="text/css"/></manifest><spine toc="ncx"><itemref idref="nav"/><itemref idref="chapter"/></spine></package>`,
		"OEBPS/nav.xhtml":        `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="ja" xml:lang="ja"><head><title>Navigation</title></head><body><nav epub:type="toc"><ol><li><a href="chapter.xhtml">Chapter</a></li></ol></nav></body></html>`,
		"OEBPS/chapter.xhtml":    `<html xmlns="http://www.w3.org/1999/xhtml" lang="ja" xml:lang="ja"><head><title>Chapter</title></head><body><h1>章</h1><p>本文<ruby>漢<rt>かん</rt></ruby>本文。</p></body></html>`,
		"OEBPS/toc.ncx":          `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head/><docTitle><text>Fixture</text></docTitle><navMap><navPoint id="n1" playOrder="1"><navLabel><text>Chapter</text></navLabel><content src="chapter.xhtml"/></navPoint></navMap></ncx>`,
		"OEBPS/cover.png":        "PNG fixture bytes",
		"OEBPS/vertical.css":     `.body { writing-mode: vertical-rl; }`,
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	mimetype, err := writer.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mimetype.Write([]byte("application/epub+zip")); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(entries))
	for name := range entries {
		paths = append(paths, name)
	}
	slices.Sort(paths)
	for _, name := range paths {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(entries[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "vertical-ruby.epub")
	if err := os.WriteFile(input, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return input
}

func writeNewCapabilityEPUB(t *testing.T) string {
	t.Helper()
	entries := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":      `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid"><metadata><dc:title>New capability fixture</dc:title><dc:identifier id="uid">urn:uuid:new-capability</dc:identifier><dc:language>en</dc:language><meta name="cover" content="cover"/></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/><item id="style" href="styles.css" media-type="text/css"/></manifest><spine toc="ncx"><itemref idref="nav"/><itemref idref="chapter"/></spine></package>`,
		"OEBPS/nav.xhtml":        `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="en" xml:lang="en"><head><title>Navigation</title></head><body><nav epub:type="toc"><ol><li><a href="chapter.xhtml">Chapter</a></li></ol></nav></body></html>`,
		"OEBPS/chapter.xhtml":    `<html xmlns="http://www.w3.org/1999/xhtml" lang="en" xml:lang="en"><head><title>Chapter</title></head><body><h1>Chapter</h1><p id="p1">English text</p></body></html>`,
		"OEBPS/toc.ncx":          `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head/><docTitle><text>Fixture</text></docTitle><navMap><navPoint id="n1" playOrder="1"><navLabel><text>Chapter</text></navLabel><content src="chapter.xhtml"/></navPoint></navMap></ncx>`,
		"OEBPS/cover.png":        "PNG fixture bytes",
		"OEBPS/styles.css":       `.chapter { color: black; }`,
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	mimetype, err := writer.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mimetype.Write([]byte("application/epub+zip")); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(entries))
	for name := range entries {
		paths = append(paths, name)
	}
	slices.Sort(paths)
	for _, name := range paths {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(entries[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "fixture.epub")
	if err := os.WriteFile(input, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return input
}

func writeNotesFallbackEPUB(t *testing.T, missingBacklink, partialDuokan bool) string {
	t.Helper()
	chapter := `<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-CN" xml:lang="zh-CN"><head><title>Chapter</title></head><body><p id="p1">正文<a id="nr1" epub:type="noteref" role="doc-noteref" class="noteref-icon" href="#n1"><img src="../Icons/note.png" alt="注"/></a>继续。</p><aside epub:type="footnote" role="doc-footnote"><ol class="footnote-list"><li class="footnote-item" id="n1">注<a epub:type="backlink" role="doc-backlink" href="#nr1">↩</a></li></ol></aside></body></html>`
	if missingBacklink {
		chapter = strings.Replace(chapter, `href="#nr1">↩`, `href="#missing">↩`, 1)
	}
	if partialDuokan {
		chapter = strings.Replace(chapter, `class="noteref-icon"`, `class="noteref-icon duokan-footnote"`, 1)
	}
	entries := map[string]string{
		"META-INF/container.xml":   `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":        `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid"><metadata><dc:title>Notes fallback fixture</dc:title><dc:identifier id="uid">urn:uuid:notes-fallback</dc:identifier><dc:language>zh-CN</dc:language><meta name="cover" content="cover"/></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/><item id="note-icon" href="Icons/note.png" media-type="image/png"/><item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/><item id="style" href="styles.css" media-type="text/css"/></manifest><spine toc="ncx"><itemref idref="nav"/><itemref idref="chapter"/></spine></package>`,
		"OEBPS/nav.xhtml":          `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-CN" xml:lang="zh-CN"><head><title>Navigation</title></head><body><nav epub:type="toc"><ol><li><a href="Text/chapter.xhtml">Chapter</a></li></ol></nav></body></html>`,
		"OEBPS/Text/chapter.xhtml": chapter,
		"OEBPS/toc.ncx":            `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head/><docTitle><text>Fixture</text></docTitle><navMap><navPoint id="n1" playOrder="1"><navLabel><text>Chapter</text></navLabel><content src="Text/chapter.xhtml"/></navPoint></navMap></ncx>`,
		"OEBPS/cover.png":          "PNG fixture bytes",
		"OEBPS/Icons/note.png":     "PNG fixture bytes",
		"OEBPS/styles.css":         `.footnote-list { border-top: 1px solid; }`,
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	mimetype, err := writer.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mimetype.Write([]byte("application/epub+zip")); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(entries))
	for name := range entries {
		paths = append(paths, name)
	}
	slices.Sort(paths)
	for _, name := range paths {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(entries[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "notes-fallback.epub")
	if err := os.WriteFile(input, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return input
}

func writeNotesFallbackOutsideTextEPUB(t *testing.T) string {
	t.Helper()
	entries := map[string]string{
		"META-INF/container.xml": `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="EPUB/package.opf"/></rootfiles></container>`,
		"EPUB/package.opf":       `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid"><metadata><dc:identifier id="uid">urn:uuid:outside-text</dc:identifier><dc:title>Outside text fixture</dc:title><dc:language>zh-CN</dc:language></metadata><manifest><item id="chapter" href="c1.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="chapter"/></spine></package>`,
		"EPUB/c1.xhtml":          `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Chapter</title></head><body><p><a epub:type="noteref" href="#missing" id="r1">1</a></p></body></html>`,
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	mimetype, err := writer.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mimetype.Write([]byte("application/epub+zip")); err != nil {
		t.Fatal(err)
	}
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "outside-text.epub")
	if err := os.WriteFile(input, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return input
}

func writeEnglishTypographyEPUB(t *testing.T) string {
	t.Helper()
	entries := map[string]string{
		"META-INF/container.xml":     `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":          `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid"><metadata><dc:title>English typography fixture</dc:title><dc:identifier id="uid">urn:uuid:english-typography</dc:identifier><dc:language>zh-CN</dc:language><meta name="cover" content="cover"/></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/><item id="style" href="styles.css" media-type="text/css"/><item id="en1" href="Text/english-1.xhtml" media-type="application/xhtml+xml"/><item id="en2" href="Text/english-2.xhtml" media-type="application/xhtml+xml"/><item id="en3" href="Text/english-3.xhtml" media-type="application/xhtml+xml"/><item id="copyright" href="Text/copyright.xhtml" media-type="application/xhtml+xml"/></manifest><spine toc="ncx"><itemref idref="en1"/><itemref idref="en2"/><itemref idref="en3"/><itemref idref="copyright"/></spine></package>`,
		"OEBPS/nav.xhtml":            `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-CN" xml:lang="zh-CN"><head><title>Navigation</title></head><body><nav epub:type="toc"><ol><li><a href="Text/english-1.xhtml">Chapter One</a></li><li><a href="Text/english-2.xhtml">Chapter Two</a></li><li><a href="Text/english-3.xhtml">Chapter Three</a></li><li><a href="Text/copyright.xhtml">版权页</a></li></ol></nav></body></html>`,
		"OEBPS/toc.ncx":              `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head/><docTitle><text>Fixture</text></docTitle><navMap><navPoint id="n1" playOrder="1"><navLabel><text>Chapter One</text></navLabel><content src="Text/english-1.xhtml"/></navPoint></navMap></ncx>`,
		"OEBPS/cover.png":            "PNG fixture bytes",
		"OEBPS/styles.css":           `.body { color: black; }`,
		"OEBPS/Text/english-1.xhtml": `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter One</title></head><body><p>The clock began to ring before the rain had stopped.</p></body></html>`,
		"OEBPS/Text/english-2.xhtml": `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter Two</title></head><body><p>Clara walked down the bright and narrow street.</p></body></html>`,
		"OEBPS/Text/english-3.xhtml": `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter Three</title></head><body><p>She returned home when the station clock struck once.</p></body></html>`,
		"OEBPS/Text/copyright.xhtml": `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>版权页</title></head><body><p>版权页 出版信息 版权所有 版次</p></body></html>`,
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	mimetype, err := writer.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mimetype.Write([]byte("application/epub+zip")); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(entries))
	for name := range entries {
		paths = append(paths, name)
	}
	slices.Sort(paths)
	for _, name := range paths {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(entries[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "english-typography.epub")
	if err := os.WriteFile(input, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return input
}

func TestLiteraryStructureEndToEndAndRedline(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	input := writeLiteraryStructureEPUB(t)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.literary.structure.format",
		InputPath:    input,
		DryRun:       true,
		Args: Args{
			"assignments": `[{"path":"OEBPS/Text/01-body.xhtml","tag":"blockquote","index":0,"class":"epigraph"}]`,
			"stylesheet":  "OEBPS/Styles/literary.css",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitOK || outcome.Envelope.Status != report.StatusPlanned {
		t.Fatalf("status=%q exit=%d findings=%+v, want planned / 0", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	if got := outcome.Envelope.Facts["epub.literary.structure.format.editCount"]; got != 2 {
		t.Fatalf("editCount=%#v, want class and stylesheet-link edits", got)
	}
	if got := outcome.Envelope.Facts["pipeline.modifiedEntries"]; !slices.Equal(got.([]string), []string{"OEBPS/Text/01-body.xhtml"}) {
		t.Fatalf("pipeline.modifiedEntries=%#v", got)
	}
	var redlinePassed bool
	for _, event := range outcome.Envelope.Events {
		if event.Step == "redline" {
			redlinePassed = event.Status == "completed" && event.Message == "0 findings"
		}
	}
	if !redlinePassed {
		t.Fatalf("events=%+v, want complete all-item redline with 0 findings", outcome.Envelope.Events)
	}
	if outcome.Envelope.Input == nil {
		t.Fatal("input artifact missing from E2E envelope")
	}
	outcome.Envelope.Input.Path = "<fixture.epub>"
	outcome.Envelope.Input.SHA256 = ""
	for i, command := range outcome.Envelope.NextCommands {
		outcome.Envelope.NextCommands[i] = strings.ReplaceAll(command, input, "fixture.epub")
	}
	data, err := json.MarshalIndent(outcome.Envelope, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	goldenPath := filepath.Join(repoRootForTest(t), "testdata", "literary_structure", "basic.report.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("literary structure E2E envelope differs from golden\n--- got ---\n%s\n--- want ---\n%s", data, want)
	}
}

func TestLiteraryStructureRejectsInvalidClassWithoutChanges(t *testing.T) {
	input := writeLiteraryStructureEPUB(t)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.literary.structure.format",
		InputPath:    input,
		DryRun:       true,
		Args: Args{
			"assignments": `[{"path":"OEBPS/Text/01-body.xhtml","tag":"blockquote","index":0,"class":"my-fancy"}]`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitFailed || outcome.Envelope.Status != report.StatusFailed || !hasFindingID(outcome.Envelope.Findings, "literary.class-not-allowed") {
		t.Fatalf("status=%q exit=%d findings=%+v, want class-not-allowed / exit 1", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	if got := outcome.Envelope.Facts["pipeline.modifiedEntries"]; len(got.([]string)) != 0 {
		t.Fatalf("pipeline.modifiedEntries=%#v, want []", got)
	}
	if got := outcome.Envelope.Facts["epub.literary.structure.format.editCount"]; got != 0 {
		t.Fatalf("editCount=%#v, want 0", got)
	}
}

func TestLiteraryStructureAssignmentsValidationUsesUsageExit(t *testing.T) {
	input := writeLiteraryStructureEPUB(t)
	for _, assignments := range []string{
		"{",
		"[]",
		`[{"path":"OEBPS/Text/01-body.xhtml","id":"target","tag":"blockquote","class":"epigraph"}]`,
		`[{"path":"OEBPS/Text/01-body.xhtml","class":"epigraph"}]`,
		`[{"path":"OEBPS/Text/01-body.xhtml","tag":"blockquote","class":"epigraph"}]`,
		`[{"path":"OEBPS/Text/01-body.xhtml","id":"target","index":0,"class":"epigraph"}]`,
	} {
		outcome, err := Run(t.Context(), Options{
			CapabilityID: "epub.literary.structure.format",
			InputPath:    input,
			DryRun:       true,
			Args:         Args{"assignments": assignments},
		})
		if err == nil || outcome.ExitCode != ExitUsage || outcome.Envelope.Status != report.StatusFailed {
			t.Fatalf("assignments=%q outcome=%+v err=%v, want usage / exit 3", assignments, outcome, err)
		}
	}
}

func writeLiteraryStructureEPUB(t *testing.T) string {
	t.Helper()
	entries := map[string]string{
		"META-INF/container.xml":    `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":         `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="uid"><metadata><dc:title>Literary structure fixture</dc:title><dc:identifier id="uid">urn:uuid:literary-structure</dc:identifier><dc:language>zh-CN</dc:language><meta name="cover" content="cover"/></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/><item id="base" href="Styles/base.css" media-type="text/css"/><item id="fonts" href="Styles/fonts.css" media-type="text/css"/><item id="literary" href="Styles/literary.css" media-type="text/css"/><item id="chapter" href="Text/01-body.xhtml" media-type="application/xhtml+xml"/></manifest><spine toc="ncx"><itemref idref="chapter"/></spine></package>`,
		"OEBPS/nav.xhtml":           `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-CN" xml:lang="zh-CN"><head><title>Navigation</title></head><body><nav epub:type="toc"><ol><li><a href="Text/01-body.xhtml">Body</a></li></ol></nav></body></html>`,
		"OEBPS/Text/01-body.xhtml":  `<html xmlns="http://www.w3.org/1999/xhtml" lang="zh-CN" xml:lang="zh-CN"><head><title>Body</title><link rel="stylesheet" type="text/css" href="../Styles/fonts.css"/><link rel="stylesheet" type="text/css" href="../Styles/base.css"/></head><body><h1>Body</h1><blockquote>引用文字</blockquote><p>正文继续。</p></body></html>`,
		"OEBPS/toc.ncx":             `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head/><docTitle><text>Fixture</text></docTitle><navMap><navPoint id="n1" playOrder="1"><navLabel><text>Body</text></navLabel><content src="Text/01-body.xhtml"/></navPoint></navMap></ncx>`,
		"OEBPS/cover.png":           "PNG fixture bytes",
		"OEBPS/Styles/base.css":     `.body { line-height: 1.6; }`,
		"OEBPS/Styles/fonts.css":    `.font-st { font-family: serif; }`,
		"OEBPS/Styles/literary.css": `.epigraph { font-style: italic; }`,
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	mimetype, err := writer.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mimetype.Write([]byte("application/epub+zip")); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(entries))
	for name := range entries {
		paths = append(paths, name)
	}
	slices.Sort(paths)
	for _, name := range paths {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(entries[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "literary-structure.epub")
	if err := os.WriteFile(input, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return input
}
