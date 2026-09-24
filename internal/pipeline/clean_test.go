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
	if len(steps) != 2 || steps[0].Name != "audit" || steps[1].Name != "normalize" {
		t.Fatalf("steps=%+v, want audit then normalize", steps)
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

func TestCleanDefaultDryRunChainsAllDocumentedSteps(t *testing.T) {
	input := buildEpubWithOPF(t)
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: input, OutputDir: filepath.Join(t.TempDir(), "out"), Jobs: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitOK || len(result.Books) != 1 || result.Books[0].Envelope.Status != report.StatusPlanned {
		t.Fatalf("result=%+v, want a successful full-chain preview", result)
	}
	steps := result.Books[0].Envelope.Facts["epub.clean.steps"].([]cleanStepSummary)
	want := []string{"audit", "normalize", "migrate", "css", "typography"}
	if len(steps) != len(want) {
		t.Fatalf("steps=%+v, want %v", steps, want)
	}
	for index, name := range want {
		if steps[index].Name != name {
			t.Fatalf("step %d=%q, want %q", index, steps[index].Name, name)
		}
		if index > 1 && steps[index].InputSHA256 != steps[index-1].OutputSHA256 {
			t.Fatalf("step %s input SHA=%s, previous output SHA=%s", name, steps[index].InputSHA256, steps[index-1].OutputSHA256)
		}
		if index > 0 && (steps[index].Redline == nil || steps[index].Redline.Status != report.StatusComplete || hasCleanErrorFinding(steps[index].Redline.Findings)) {
			t.Fatalf("step %s redline=%+v, want explicit successful redline result", name, steps[index].Redline)
		}
	}
	if redline := result.Books[0].Envelope.Facts["epub.clean.redline"].(cleanRedlineSummary); redline.Status != report.StatusComplete {
		t.Fatalf("final redline=%+v, want complete", redline)
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
		InputPath: input, OutputDir: outputDir, Approve: true, Jobs: 1,
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
	if len(steps) != 5 || steps[0].Name != "audit" || steps[4].Name != "typography" {
		t.Fatalf("steps=%+v, want audit plus all four steps", steps)
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

func hasCleanErrorFinding(findings []report.Finding) bool {
	for _, finding := range findings {
		if finding.Level == "error" {
			return true
		}
	}
	return false
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
	for _, steps := range [][]string{{}, {"typo"}, {"css", "normalize"}, {"normalize", "normalize"}} {
		if _, err := normalizeCleanSteps(steps); err == nil {
			t.Errorf("normalizeCleanSteps(%v) succeeded, want error", steps)
		}
	}
	all, err := normalizeCleanSteps(nil)
	if err != nil || len(all) != 4 {
		t.Fatalf("default steps=%v err=%v, want four steps", all, err)
	}
}
