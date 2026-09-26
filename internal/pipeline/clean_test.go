package pipeline

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/redline"
	"github.com/liyafly/epub-handbook/internal/report"
)

func TestCleanDryRunWritesOnlyPerBookSummary(t *testing.T) {
	input := buildEpubWithOPF(t)
	outputDir := filepath.Join(t.TempDir(), "out")
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: input, OutputDir: outputDir, Steps: []string{"normalize"}, Jobs: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitOK || len(result.Books) != 1 {
		t.Fatalf("result=%+v, want one successful book", result)
	}
	bookResult := result.Books[0]
	if bookResult.Envelope.Status != report.StatusPlanned || bookResult.OutputPath != "" {
		t.Fatalf("status=%q output=%q, want planned and no EPUB output", bookResult.Envelope.Status, bookResult.OutputPath)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "in.epub")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run EPUB output exists or stat failed: %v", err)
	}
	if _, err := os.Stat(bookResult.ReportPath); err != nil {
		t.Fatalf("summary report missing: %v", err)
	}
	var saved report.Envelope
	data, err := os.ReadFile(bookResult.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("summary is not a report envelope: %v", err)
	}
	if saved.Capability != cleanCapabilityID || saved.Status != report.StatusPlanned {
		t.Fatalf("saved summary=%+v", saved)
	}
	steps := bookResult.Envelope.Facts["epub.clean.steps"].([]cleanStepSummary)
	if len(steps) != 3 || steps[0].Name != "audit" || steps[1].Name != "normalize" || steps[2].Name != "audit-final" {
		t.Fatalf("steps=%+v, want audit, normalize, and final audit", steps)
	}
	if steps[1].InputSHA256 == "" || steps[1].OutputSHA256 == "" {
		t.Fatalf("normalize SHA chain=%+v, want input and output hashes", steps[1])
	}
	if saved.Facts["epub.clean.previewSHA256"] != steps[1].OutputSHA256 {
		t.Fatalf("preview SHA=%v step output=%s", saved.Facts["epub.clean.previewSHA256"], steps[1].OutputSHA256)
	}
	if _, err := redline.LoadPathMap(data); err != nil {
		t.Fatalf("clean summary cannot be reused as --path-map: %v", err)
	}
	redline := bookResult.Envelope.Facts["epub.clean.redline"].(cleanRedlineSummary)
	if redline.Status != report.StatusComplete {
		t.Fatalf("redline=%+v, want complete", redline)
	}
}

func TestCleanDefaultDryRunOnlyAudits(t *testing.T) {
	input := buildEpubWithOPF(t)
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: input, OutputDir: filepath.Join(t.TempDir(), "out"), Jobs: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitOK || len(result.Books) != 1 || result.Books[0].Envelope.Status != report.StatusPlanned {
		t.Fatalf("result=%+v, want a successful audit-only plan", result)
	}
	bookResult := result.Books[0]
	steps := bookResult.Envelope.Facts["epub.clean.steps"].([]cleanStepSummary)
	if len(steps) != 1 || steps[0].Name != "audit" {
		t.Fatalf("steps=%+v, want audit only", steps)
	}
	if _, exists := bookResult.Envelope.Facts["epub.clean.previewSHA256"]; exists {
		t.Fatalf("audit-only plan has a preview SHA: %+v", bookResult.Envelope.Facts)
	}
	if got := bookResult.Envelope.Facts["pipeline.artifactDisposition"]; got != "planned" {
		t.Fatalf("artifact disposition=%v, want planned", got)
	}
	if got := bookResult.Envelope.Facts["pipeline.selectedSteps"].([]string); len(got) != 0 {
		t.Fatalf("selected steps=%v, want none", got)
	}
	if result.Envelope.Capability != cleanCapabilityID || result.Envelope.Status != report.StatusPlanned {
		t.Fatalf("batch envelope=%+v, want planned epub.clean envelope", result.Envelope)
	}
}

func TestCleanStepSummaryPreservesFailedRedlineResult(t *testing.T) {
	env := report.Envelope{
		Events:   []report.Event{{Step: "redline", Status: "failed", Message: "1 finding"}},
		Findings: []report.Finding{{Level: "error", ID: "redline.text", Title: "Text changed"}},
	}
	summary := cleanStepSummaryFrom("normalize", "epub.structure.normalize", "before", "after", report.StatusFailed, env, nil)
	if summary.Redline == nil || summary.Redline.Status != report.StatusFailed || len(summary.Redline.Findings) != 1 {
		t.Fatalf("redline summary=%+v, want failed result and finding", summary.Redline)
	}
}

func TestCleanApprovedRunWritesOnlyFinalCandidateAndReport(t *testing.T) {
	input := buildEpubWithOPF(t)
	outputDir := filepath.Join(t.TempDir(), "out")
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: input, OutputDir: outputDir, Steps: []string{"normalize"}, Approve: true, Jobs: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitOK || len(result.Books) != 1 {
		t.Fatalf("result=%+v, want one successful book", result)
	}
	bookResult := result.Books[0]
	if bookResult.Envelope.Status != report.StatusComplete || bookResult.OutputPath != filepath.Join(outputDir, "in.epub") {
		t.Fatalf("status=%q output=%q", bookResult.Envelope.Status, bookResult.OutputPath)
	}
	steps := bookResult.Envelope.Facts["epub.clean.steps"].([]cleanStepSummary)
	if len(steps) != 3 || steps[0].Name != "audit" || steps[1].Name != "normalize" || steps[2].Name != "audit-final" {
		t.Fatalf("steps=%+v, want audit, normalize, and final audit", steps)
	}
	for _, path := range []string{bookResult.OutputPath, bookResult.ReportPath} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("approved artifact %s missing: %v", path, err)
		}
	}
	var files []string
	if err := filepath.WalkDir(outputDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			relative, err := filepath.Rel(outputDir, path)
			if err != nil {
				return err
			}
			files = append(files, relative)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	slices.Sort(files)
	wantFiles := []string{"in.clean.json", "in.epub"}
	if !slices.Equal(files, wantFiles) {
		t.Fatalf("approved files=%v, want only final EPUB and summary %v", files, wantFiles)
	}
	finalSHA, err := book.FileSHA256Context(t.Context(), bookResult.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if bookResult.Envelope.Output == nil || bookResult.Envelope.Output.SHA256 != finalSHA {
		t.Fatalf("output SHA=%s envelope=%+v", finalSHA, bookResult.Envelope.Output)
	}
}

func TestCleanFailedRunRetainsOnlyExplicitReviewCandidate(t *testing.T) {
	input := buildEpubWithOPF(t)
	outputDir := filepath.Join(t.TempDir(), "out")
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: input, OutputDir: outputDir,
		Steps: []string{"normalize", "typography"}, Preset: "missing-preset", Scope: []string{"all"},
		Approve: true, RetainReviewCandidate: true, Jobs: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitFailed || len(result.Books) != 1 {
		t.Fatalf("result=%+v, want one failed book", result)
	}
	bookResult := result.Books[0]
	wantReview := filepath.Join(outputDir, "in.review-only.epub")
	if bookResult.Envelope.Status != report.StatusFailed || bookResult.OutputPath != wantReview {
		t.Fatalf("status=%q output=%q, want failed review-only candidate", bookResult.Envelope.Status, bookResult.OutputPath)
	}
	if got := bookResult.Envelope.Facts["pipeline.artifactDisposition"]; got != "review-only" {
		t.Fatalf("artifact disposition=%v, want review-only", got)
	}
	if _, err := os.Stat(wantReview); err != nil {
		t.Fatalf("review-only candidate missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "in.epub")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed run wrote an approved EPUB: %v", err)
	}
}

func TestCleanFailedApprovedRunWithholdsCandidateByDefault(t *testing.T) {
	input := buildEpubWithOPF(t)
	outputDir := filepath.Join(t.TempDir(), "out")
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: input, OutputDir: outputDir,
		Steps: []string{"normalize", "typography"}, Preset: "missing-preset", Scope: []string{"all"},
		Approve: true, Jobs: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitFailed || len(result.Books) != 1 || result.Books[0].Envelope.Status != report.StatusFailed {
		t.Fatalf("result=%+v, want failed book", result)
	}
	if result.Books[0].OutputPath != "" || result.Books[0].Envelope.Output != nil {
		t.Fatalf("failed run published a candidate: %+v", result.Books[0])
	}
	if got := result.Books[0].Envelope.Facts["pipeline.artifactDisposition"]; got != "withheld" {
		t.Fatalf("artifact disposition=%v, want withheld", got)
	}
	for _, path := range []string{filepath.Join(outputDir, "in.epub"), filepath.Join(outputDir, "in.review-only.epub")} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed run created %s: %v", path, err)
		}
	}
}

func TestCleanDirectoryUsesSortedInputsAndParallelJobs(t *testing.T) {
	inputDir := filepath.Join(t.TempDir(), "books")
	if err := os.MkdirAll(filepath.Join(inputDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(inputDir, "b.epub"), filepath.Join(inputDir, "nested", "a.epub")} {
		if err := os.WriteFile(path, epubFixtureBytes(t), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(inputDir, "ignore.txt"), []byte("not an EPUB"), 0o644); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(t.TempDir(), "cleaned")
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: inputDir, OutputDir: outputDir, Steps: []string{"normalize"}, Jobs: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitOK || len(result.Books) != 2 {
		t.Fatalf("result=%+v, want two successful books", result)
	}
	if !slices.IsSortedFunc(result.Books, func(a, b CleanBookResult) int { return strings.Compare(a.InputPath, b.InputPath) }) {
		t.Fatalf("book results are not sorted: %+v", result.Books)
	}
	for _, bookResult := range result.Books {
		if bookResult.Envelope.Status != report.StatusPlanned {
			t.Errorf("%s status=%s, want planned", bookResult.InputPath, bookResult.Envelope.Status)
		}
		if _, err := os.Stat(bookResult.ReportPath); err != nil {
			t.Errorf("summary %s missing: %v", bookResult.ReportPath, err)
		}
		if _, err := os.Stat(bookResult.OutputPath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("dry-run EPUB %s unexpectedly exists", bookResult.OutputPath)
		}
	}
}

func TestCleanInvalidEPUBStillWritesFailureSummary(t *testing.T) {
	input := filepath.Join(t.TempDir(), "broken.epub")
	if err := os.WriteFile(input, []byte("not a zip file"), 0o644); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(t.TempDir(), "out")
	result, err := Clean(t.Context(), CleanOptions{InputPath: input, OutputDir: outputDir, Steps: []string{"normalize"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitFailed || len(result.Books) != 1 {
		t.Fatalf("result=%+v, want failed per-book result", result)
	}
	bookResult := result.Books[0]
	if bookResult.Envelope.Status != report.StatusFailed || bookResult.ReportPath == "" {
		t.Fatalf("book result=%+v", bookResult)
	}
	if _, err := os.Stat(bookResult.ReportPath); err != nil {
		t.Fatalf("failure report missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "broken.epub")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed input produced an EPUB: %v", err)
	}
}

func TestCleanRejectsInputOutputOverlapAndInvalidSteps(t *testing.T) {
	inputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(inputDir, "book.epub"), epubFixtureBytes(t), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Clean(t.Context(), CleanOptions{InputPath: inputDir, OutputDir: filepath.Join(inputDir, "out")})
	if _, ok := errors.AsType[*UsageError](err); err == nil || !ok {
		t.Fatalf("nested output error=%v, want UsageError", err)
	}
	if _, err := Clean(t.Context(), CleanOptions{InputPath: filepath.Join(inputDir, "book.epub"), OutputDir: filepath.Join(t.TempDir(), "out"), Approve: true}); err == nil {
		t.Fatal("--approve without a transform step succeeded")
	}
	for _, steps := range [][]string{{}, {"typo"}, {"css", "normalize"}, {"normalize", "normalize"}} {
		if _, err := normalizeCleanSteps(steps, "", nil); err == nil {
			t.Errorf("normalizeCleanSteps(%v) succeeded, want error", steps)
		}
	}
	defaults, err := normalizeCleanSteps(nil, "", nil)
	if err != nil || len(defaults) != 0 {
		t.Fatalf("default steps=%v err=%v, want audit-only plan", defaults, err)
	}
	if _, err := normalizeCleanSteps([]string{"typography"}, "", []string{"all"}); err == nil {
		t.Fatal("typography without an explicit preset succeeded")
	}
	if _, err := normalizeCleanSteps([]string{"typography"}, "literary-cn", nil); err == nil {
		t.Fatal("typography without an explicit scope succeeded")
	}
	steps, err := normalizeCleanSteps([]string{"typography"}, "literary-cn", []string{"OEBPS/Text/a.xhtml", "OEBPS/Text/b.xhtml"})
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].args["preset"] != "literary-cn" || steps[0].args["scope_paths"] != `["OEBPS/Text/a.xhtml","OEBPS/Text/b.xhtml"]` {
		t.Fatalf("typography args=%v, want explicit preset and exact scope", steps[0].args)
	}
}

func TestCleanAuditOnlyAllowsOnlyKnownMigrateRepairs(t *testing.T) {
	findings := []report.Finding{
		{Level: "error", ID: "mathml", Title: `MathML XHTML item missing properties="mathml"`},
		{Level: "error", ID: "svg", Title: `Inline SVG XHTML item missing properties="svg"`},
		{Level: "error", ID: "drm", Title: "EPUB has META-INF/encryption.xml"},
	}
	if got := cleanAuditBlockers(findings, nil); len(got) != len(findings) {
		t.Fatalf("audit-only blockers=%v, want all errors", got)
	}
	got := cleanAuditBlockers(findings, []cleanStepDefinition{{name: "migrate"}})
	if len(got) != 1 || got[0].ID != "drm" {
		t.Fatalf("migration blockers=%v, want only DRM", got)
	}
}
