package pipeline

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/redline"
	"github.com/liyafly/epub-handbook/internal/report"
)

func TestCleanSessionCommitsBookForkAndTracksEntryChanges(t *testing.T) {
	original, err := book.OpenBytesContext(t.Context(), "session.epub", epubFixtureBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	session := newCleanSession(original)
	candidate := session.BeginStep()
	path := "OEBPS/c1.xhtml"
	content, err := candidate.CurrentContext(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(content, []byte("段落。"))
	if start < 0 {
		t.Fatal("fixture is missing the paragraph text")
	}
	if err := candidate.Apply([]editset.Edit{editset.Replace(path, int64(start), int64(len("段落。")), []byte("已修改。"))}); err != nil {
		t.Fatal(err)
	}
	changed, err := session.ModifiedEntries(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(changed, []string{path}) {
		t.Fatalf("changed entries=%v, want only %s", changed, path)
	}
	session.CommitStep("normalize", candidate)
	if session.stateID != "step:normalize" || !session.hasCandidate {
		t.Fatalf("session state=%q candidate=%t", session.stateID, session.hasCandidate)
	}
	originalContent, err := original.CurrentContext(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(originalContent, []byte("段落。")) || bytes.Contains(originalContent, []byte("已修改。")) {
		t.Fatalf("committing a fork mutated the original Book: %q", originalContent)
	}
}

func TestCleanDryRunWritesOnlyPerBookSummary(t *testing.T) {
	input := buildEpubWithOPF(t)
	outputDir := filepath.Join(t.TempDir(), "out")
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: input, OutputDir: outputDir, Steps: []string{"normalize"},
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
	if steps[1].InputState != "input" || steps[1].OutputState != "step:normalize" || steps[1].ChangedEntries == nil {
		t.Fatalf("normalize state chain=%+v, want explicit state IDs and a changed-entry list", steps[1])
	}
	if saved.Facts["epub.clean.previewSHA256"] != nil || saved.Facts["epub.clean.previewState"] != "step:normalize" {
		t.Fatalf("preview state=%v preview SHA=%v", saved.Facts["epub.clean.previewState"], saved.Facts["epub.clean.previewSHA256"])
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	stepFacts := raw["facts"].(map[string]any)["epub.clean.steps"].([]any)[1].(map[string]any)
	if _, exists := stepFacts["inputSHA256"]; exists {
		t.Fatalf("intermediate stage must not claim an EPUB SHA: %+v", stepFacts)
	}
	if _, exists := stepFacts["outputSHA256"]; exists {
		t.Fatalf("intermediate stage must not claim an EPUB SHA: %+v", stepFacts)
	}
	if _, err := redline.LoadPathMap(data); err != nil {
		t.Fatalf("clean summary cannot be reused as --path-map: %v", err)
	}
	redline := bookResult.Envelope.Facts["epub.clean.redline"].(cleanRedlineSummary)
	if redline.Status != report.StatusComplete {
		t.Fatalf("redline=%+v, want complete", redline)
	}
}

func TestCleanApprovesEPUB2Migration(t *testing.T) {
	input := filepath.Join(t.TempDir(), "legacy.epub")
	if err := os.WriteFile(input, epub2NoNavFixtureBytes(t), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: input,
		OutputDir: filepath.Join(t.TempDir(), "out"),
		Steps:     []string{"migrate"},
		Approve:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitOK || len(result.Books) != 1 {
		t.Fatalf("result=%+v, want one approved successful book", result)
	}
	bookResult := result.Books[0]
	if got := bookResult.Envelope.Facts["pipeline.artifactDisposition"]; got != "approved" {
		t.Fatalf("artifact disposition=%v, want approved; findings=%+v", got, bookResult.Envelope.Findings)
	}
	if bookResult.OutputPath == "" {
		t.Fatal("approved migration did not publish its output path")
	}
	if _, err := os.Stat(bookResult.OutputPath); err != nil {
		t.Fatalf("approved migration output is unavailable: %v", err)
	}
}

func TestCleanSharesSessionAcrossSelectedSteps(t *testing.T) {
	input := buildEpubWithOPF(t)
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: input, OutputDir: filepath.Join(t.TempDir(), "out"),
		Steps: []string{"normalize", "migrate", "css"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitOK || len(result.Books) != 1 {
		t.Fatalf("result=%+v, want a successful planned book", result)
	}
	bookResult := result.Books[0]
	if bookResult.Envelope.Status != report.StatusPlanned {
		t.Fatalf("clean status=%s findings=%+v", bookResult.Envelope.Status, bookResult.Envelope.Findings)
	}
	steps := bookResult.Envelope.Facts["epub.clean.steps"].([]cleanStepSummary)
	wantNames := []string{"audit", "normalize", "migrate", "css", "audit-final"}
	if len(steps) != len(wantNames) {
		t.Fatalf("steps=%+v, want %v", steps, wantNames)
	}
	for index, name := range wantNames {
		if steps[index].Name != name {
			t.Fatalf("step %d=%q, want %q", index, steps[index].Name, name)
		}
	}
	for index, name := range []string{"normalize", "migrate", "css"} {
		step := steps[index+1]
		if step.InputState != "input" && index == 0 {
			t.Errorf("first transform input state=%q, want input", step.InputState)
		}
		if index > 0 && step.InputState != "step:"+[]string{"normalize", "migrate"}[index-1] {
			t.Errorf("%s input state=%q", name, step.InputState)
		}
		if step.OutputState != "step:"+name || step.ChangedEntries == nil {
			t.Errorf("%s result=%+v, want committed state and changed-entry list", name, step)
		}
	}
	if got := bookResult.Envelope.Facts["epub.clean.previewState"]; got != "step:css" {
		t.Fatalf("preview state=%v, want step:css", got)
	}
	redlineSummary := bookResult.Envelope.Facts["epub.clean.redline"].(cleanRedlineSummary)
	if redlineSummary.Status != report.StatusComplete {
		t.Fatalf("redline=%+v, want complete", redlineSummary)
	}
}

func TestCleanSkipsDiagnosticUpstreamAudits(t *testing.T) {
	input := buildEpubWithOPF(t)
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: input, OutputDir: filepath.Join(t.TempDir(), "out"),
		Steps: []string{"normalize", "migrate", "css"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitOK || len(result.Books) != 1 {
		t.Fatalf("result=%+v, want a successful planned book", result)
	}
	navAuditEvents := 0
	for _, event := range result.Books[0].Envelope.Events {
		if event.Step == "epub.package.nav.audit" {
			navAuditEvents++
		}
	}
	if navAuditEvents != 2 {
		t.Fatalf("nav.audit events=%d, want only clean's initial and final audits; events=%+v", navAuditEvents, result.Books[0].Envelope.Events)
	}
}

func TestCleanDefaultDryRunOnlyAudits(t *testing.T) {
	input := buildEpubWithOPF(t)
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: input, OutputDir: filepath.Join(t.TempDir(), "out"),
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

func TestCleanBatchPlannedOrCompleteHasZeroExitCode(t *testing.T) {
	for _, tc := range []struct {
		name       string
		approve    bool
		wantStatus string
	}{
		{name: "planned", wantStatus: report.StatusPlanned},
		{name: "complete", approve: true, wantStatus: report.StatusComplete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Clean(t.Context(), CleanOptions{
				InputPath: buildEpubWithOPF(t),
				OutputDir: filepath.Join(t.TempDir(), "out"),
				Steps:     []string{"normalize"},
				Approve:   tc.approve,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Envelope.Status != tc.wantStatus || result.ExitCode != ExitOK {
				t.Fatalf("batch status=%q exit=%d, want %q / 0", result.Envelope.Status, result.ExitCode, tc.wantStatus)
			}
		})
	}
}

func TestCleanStepSummaryPreservesFailedRedlineResult(t *testing.T) {
	env := report.Envelope{
		Events:   []report.Event{{Step: "redline", Status: "failed", Message: "1 finding"}},
		Findings: []report.Finding{{Level: "error", ID: "redline.text", Title: "Text changed"}},
	}
	summary := cleanStepSummaryFrom("normalize", "epub.structure.normalize", "input", "input", []string{}, report.StatusFailed, env, nil)
	if summary.Redline == nil || summary.Redline.Status != report.StatusFailed || len(summary.Redline.Findings) != 1 {
		t.Fatalf("redline summary=%+v, want failed result and finding", summary.Redline)
	}
}

func TestCleanApprovedRunWritesOnlyFinalCandidateAndReport(t *testing.T) {
	input := buildEpubWithOPF(t)
	outputDir := filepath.Join(t.TempDir(), "out")
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: input, OutputDir: outputDir, Steps: []string{"normalize"}, Approve: true,
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

func TestCleanApprovedCancellationAtAnyCheckpointNeverReportsCompleteWithoutOutput(t *testing.T) {
	input := buildEpubWithOPF(t)
	for checkpoint := int32(1); checkpoint <= 400; checkpoint++ {
		outputDir := filepath.Join(t.TempDir(), "out")
		ctx := cancelAtCheckpoint(t, checkpoint)
		result, err := Clean(ctx, CleanOptions{
			InputPath: input, OutputDir: outputDir, Steps: []string{"normalize"}, Approve: true,
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("checkpoint %d: Clean error = %v", checkpoint, err)
		}
		for _, bookResult := range result.Books {
			if bookResult.Envelope.Status != report.StatusComplete {
				continue
			}
			if bookResult.OutputPath == "" || bookResult.Envelope.Facts["pipeline.artifactDisposition"] != "approved" {
				t.Fatalf("checkpoint %d: complete result has no approved output: %+v", checkpoint, bookResult)
			}
			if _, statErr := os.Stat(bookResult.OutputPath); statErr != nil {
				t.Fatalf("checkpoint %d: complete result output is absent: %v", checkpoint, statErr)
			}
		}
		if ctx.Err() != nil {
			for _, bookResult := range result.Books {
				if bookResult.Envelope.Status == report.StatusComplete {
					t.Fatalf("checkpoint %d: cancelled run reported complete", checkpoint)
				}
			}
		}
	}
}

func TestCleanFailedApprovedRunWithholdsCandidate(t *testing.T) {
	input := buildEpubWithWrongDescendantNamespace(t)
	outputDir := filepath.Join(t.TempDir(), "out")
	result, err := Clean(t.Context(), CleanOptions{
		InputPath: input, OutputDir: outputDir, Steps: []string{"normalize", "migrate"}, Approve: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitFailed || len(result.Books) != 1 {
		t.Fatalf("result=%+v, want one failed book", result)
	}
	bookResult := result.Books[0]
	if bookResult.Envelope.Status != report.StatusFailed {
		t.Fatalf("book status=%q, want failed", bookResult.Envelope.Status)
	}
	if got := bookResult.Envelope.Facts["pipeline.artifactDisposition"]; got != "withheld" {
		t.Fatalf("artifact disposition=%v, want withheld", got)
	}
	if got := bookResult.Envelope.Facts["epub.clean.previewState"]; got != "step:normalize" {
		t.Fatalf("preview state=%v, want step:normalize", got)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "in.epub")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("withheld EPUB exists or stat failed: %v", err)
	}
}

func buildEpubWithWrongDescendantNamespace(t testing.TB) string {
	t.Helper()
	original := epubFixtureBytes(t)
	reader, err := zip.NewReader(bytes.NewReader(original), int64(len(original)))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range reader.File {
		input, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, readErr := io.ReadAll(input)
		closeErr := input.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if file.Name == "OEBPS/c1.xhtml" {
			content = bytes.Replace(content, []byte("<body>"), []byte(`<body xmlns:epub="urn:wrong">`), 1)
		}
		method := uint16(zip.Deflate)
		if file.Name == "mimetype" {
			method = zip.Store
		}
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: file.Name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "wrong-namespace.epub")
	if err := os.WriteFile(path, output.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCleanReportWriteFailureKeepsApprovedArtifactDisposition(t *testing.T) {
	input := buildEpubWithOPF(t)
	outputDir := filepath.Join(t.TempDir(), "out")
	outputPath := filepath.Join(outputDir, "in.epub")
	reportPath := filepath.Join(outputDir, "in.clean.json")
	result, err := cleanWithReportWriter(t.Context(), CleanOptions{
		InputPath: input, OutputDir: outputDir, Steps: []string{"normalize"}, Approve: true,
	}, func(_ context.Context, path string, _ []byte) error {
		if path != reportPath {
			t.Fatalf("report writer path=%q, want %q", path, reportPath)
		}
		if _, err := os.Stat(outputPath); err != nil {
			t.Fatalf("approved EPUB must exist when the later report write fails: %v", err)
		}
		return errors.New("forced report write failure")
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitFailed || len(result.Books) != 1 {
		t.Fatalf("result=%+v, want a failed book after report write error", result)
	}
	bookResult := result.Books[0]
	if bookResult.Envelope.Status != report.StatusFailed {
		t.Fatalf("book status=%q, want failed", bookResult.Envelope.Status)
	}
	if got := bookResult.Envelope.Facts["pipeline.artifactDisposition"]; got != "approved" {
		t.Fatalf("artifact disposition=%v, want approved", got)
	}
	if !hasFindingID(bookResult.Envelope.Findings, "clean.report-write-failed") {
		t.Fatalf("report write finding missing: %+v", bookResult.Envelope.Findings)
	}
	books := result.Envelope.Facts["epub.clean.books"].([]report.CleanBookSummary)
	if len(books) != 1 || books[0].ArtifactDisposition != "approved" {
		t.Fatalf("batch summary=%+v, want approved", books)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("approved EPUB was not preserved: %v", err)
	}
	if _, err := os.Stat(reportPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("report path should remain absent after simulated failure: %v", err)
	}
}

func TestCleanDirectoryUsesSortedInputsSerially(t *testing.T) {
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
		InputPath: inputDir, OutputDir: outputDir, Steps: []string{"normalize"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != ExitOK || len(result.Books) != 2 || len(result.NotStarted) != 0 {
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

func TestCleanCancellationDoesNotWriteReportsForUnstartedBooks(t *testing.T) {
	inputDir := filepath.Join(t.TempDir(), "books")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.epub", "b.epub", "c.epub"} {
		if err := os.WriteFile(filepath.Join(inputDir, name), epubFixtureBytes(t), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	outputDir := filepath.Join(t.TempDir(), "cleaned")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var written []string
	result, err := cleanWithReportWriter(ctx, CleanOptions{
		InputPath: inputDir, OutputDir: outputDir,
	}, func(_ context.Context, path string, data []byte) error {
		written = append(written, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		if len(written) == 1 {
			cancel()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Books) != 1 || filepath.Base(result.Books[0].InputPath) != "a.epub" {
		t.Fatalf("processed books=%v, want only the first sorted input", result.Books)
	}
	resolvedInputDir, err := filepath.EvalSymlinks(inputDir)
	if err != nil {
		t.Fatal(err)
	}
	wantNotStarted := []string{filepath.Join(resolvedInputDir, "b.epub"), filepath.Join(resolvedInputDir, "c.epub")}
	gotNotStarted, ok := result.Envelope.Facts["epub.clean.notStarted"].([]string)
	if !ok || !slices.Equal(gotNotStarted, wantNotStarted) {
		t.Fatalf("notStarted=%#v, want %v", result.Envelope.Facts["epub.clean.notStarted"], wantNotStarted)
	}
	if result.Envelope.Status != report.StatusCancelled {
		t.Fatalf("batch status=%q, want cancelled", result.Envelope.Status)
	}
	if result.ExitCode != ExitFailed {
		t.Fatalf("batch exit=%d, want failed exit code for cancellation", result.ExitCode)
	}
	if len(written) != 1 || filepath.Base(written[0]) != "a.clean.json" {
		t.Fatalf("written reports=%v, want only a.clean.json", written)
	}
	for _, name := range []string{"b.clean.json", "c.clean.json"} {
		if _, err := os.Stat(filepath.Join(outputDir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("unstarted report %s exists or stat failed: %v", name, err)
		}
	}
}

func TestCleanCancelledBatchEnvelopeGolden(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	cancelled := cleanBookFailure(CleanBookResult{
		InputPath:  filepath.Join(root, "testdata", "cancelled.epub"),
		ReportPath: filepath.Join(root, "out", "cancelled.clean.json"),
	}, report.Envelope{SchemaVersion: "2", Capability: cleanCapabilityID},
		"clean.book-cancelled", "EPUB clean was cancelled", context.Canceled, CleanOptions{}, nil)
	cancelledDisposition, ok := cancelled.Envelope.Facts["pipeline.artifactDisposition"].(string)
	if !ok || cancelledDisposition != "none" {
		t.Fatalf("cancelled artifactDisposition=%#v, want none", cancelled.Envelope.Facts["pipeline.artifactDisposition"])
	}
	if got := cancelled.Envelope.Facts["epub.clean.approved"]; got != false {
		t.Fatalf("cancelled approved fact=%v, want false", got)
	}
	if got, ok := cancelled.Envelope.Facts["pipeline.blockers"].([]string); !ok || !slices.Equal(got, []string{"clean.book-cancelled"}) {
		t.Fatalf("cancelled blockers=%#v, want [clean.book-cancelled]", cancelled.Envelope.Facts["pipeline.blockers"])
	}
	if len(cancelled.Envelope.Events) != 1 || cancelled.Envelope.Events[0].Status != "failed" || cancelled.Envelope.Events[0].Message != "cancelled: context canceled" {
		t.Fatalf("per-book cancellation event=%+v, want a schema-valid failed event with cancellation detail", cancelled.Envelope.Events)
	}
	planned := report.CleanBookSummary{
		InputPath:           filepath.Join(root, "testdata", "planned.epub"),
		ReportPath:          filepath.Join(root, "out", "planned.clean.json"),
		ArtifactDisposition: "planned",
		Status:              report.StatusPlanned,
		ExitCode:            ExitOK,
		Findings:            []report.Finding{},
	}
	batch := report.CleanBatchEnvelope([]report.CleanBookSummary{
		{
			InputPath: cancelled.InputPath, ReportPath: cancelled.ReportPath,
			ArtifactDisposition: cancelledDisposition, Status: cancelled.Envelope.Status,
			ExitCode: cancelled.ExitCode, Error: errorString(cancelled.Err),
			Findings: nonNilCleanFindings(cancelled.Envelope.Findings),
		},
		planned,
	}, nil)
	data, err := MarshalEnvelope(batch)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.ReplaceAll(string(data), root, "<repo>")
	goldenPath := filepath.Join(root, "testdata", "envelope", "clean-batch-cancelled.report.json")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("cancelled batch differs from golden %s\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, want)
	}
	for _, summary := range batch.Facts["epub.clean.books"].([]report.CleanBookSummary) {
		if summary.ArtifactDisposition == "" {
			t.Fatalf("cancelled batch has an empty artifact disposition: %+v", summary)
		}
	}
}

func TestCleanUnreadableInputReportsNoneDisposition(t *testing.T) {
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
	if got := bookResult.Envelope.Facts["pipeline.artifactDisposition"]; got != "none" {
		t.Fatalf("artifactDisposition=%v, want none", got)
	}
	if got := bookResult.Envelope.Facts["epub.clean.approved"]; got != false {
		t.Fatalf("approved fact=%v, want false", got)
	}
	if got, ok := bookResult.Envelope.Facts["pipeline.blockers"].([]string); !ok || !slices.Equal(got, []string{"clean.input-read-failed"}) {
		t.Fatalf("blockers=%#v, want [clean.input-read-failed]", bookResult.Envelope.Facts["pipeline.blockers"])
	}
	if got, ok := bookResult.Envelope.Facts["pipeline.selectedSteps"].([]string); !ok || !slices.Equal(got, []string{"normalize"}) {
		t.Fatalf("selectedSteps=%#v, want [normalize]", bookResult.Envelope.Facts["pipeline.selectedSteps"])
	}
	if _, err := os.Stat(bookResult.ReportPath); err != nil {
		t.Fatalf("failure report missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "broken.epub")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed input produced an EPUB: %v", err)
	}
}

func TestCleanCancelledBeforeStartReportsCancelled(t *testing.T) {
	input := buildEpubWithOPF(t)
	outputDir := filepath.Join(t.TempDir(), "out")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	defer cancel()
	steps := cleanStepDefinitions()
	bookResult := cleanOneBook(ctx, CleanOptions{Approve: true}, cleanInput{
		path: input, relative: filepath.Base(input),
	}, false, outputDir, steps)
	if bookResult.Envelope.Status != report.StatusCancelled {
		t.Fatalf("status=%q, want cancelled", bookResult.Envelope.Status)
	}
	if !hasFindingID(bookResult.Envelope.Findings, "clean.book-cancelled") || hasFindingID(bookResult.Envelope.Findings, "clean.input-read-failed") {
		t.Fatalf("findings=%+v, want clean.book-cancelled and no input-read failure", bookResult.Envelope.Findings)
	}
	if got := bookResult.Envelope.Facts["pipeline.artifactDisposition"]; got != "none" {
		t.Fatalf("artifactDisposition=%v, want none", got)
	}
	if got := bookResult.Envelope.Facts["epub.clean.approved"]; got != true {
		t.Fatalf("approved fact=%v, want true", got)
	}
	wantSteps := []string{"normalize", "migrate", "css"}
	if got, ok := bookResult.Envelope.Facts["pipeline.selectedSteps"].([]string); !ok || !slices.Equal(got, wantSteps) {
		t.Fatalf("selectedSteps=%#v, want %v", bookResult.Envelope.Facts["pipeline.selectedSteps"], wantSteps)
	}
	if got, ok := bookResult.Envelope.Facts["pipeline.blockers"].([]string); !ok || !slices.Equal(got, []string{"clean.book-cancelled"}) {
		t.Fatalf("blockers=%#v, want [clean.book-cancelled]", bookResult.Envelope.Facts["pipeline.blockers"])
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
		if _, err := normalizeCleanSteps(steps); err == nil {
			t.Errorf("normalizeCleanSteps(%v) succeeded, want error", steps)
		}
	}
	defaults, err := normalizeCleanSteps(nil)
	if err != nil || len(defaults) != 0 {
		t.Fatalf("default steps=%v err=%v, want audit-only plan", defaults, err)
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
