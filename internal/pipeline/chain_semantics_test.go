package pipeline

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
)

// TestRunNormalizeDryRunNotBlockedByNavAudit 是 ff30a1a 回归的合成 EPUB
// 复现：导航链接指向不存在的 fragment，nav.audit 会报告 error findings；
// 但作为诊断型上游时，不得阻止 normalize dry-run 继续执行。
func TestRunNormalizeDryRunNotBlockedByNavAudit(t *testing.T) {
	input := buildEpubWithNavHref(t, "c1.xhtml#missing-fragment")
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.structure.normalize",
		InputPath:    input,
		OutputPath:   filepath.Join(t.TempDir(), "norm.epub"),
		DryRun:       true,
		Args:         Args{},
	})
	if err != nil {
		t.Fatal(err)
	}
	env := outcome.Envelope
	if !hasEvent(env.Events, "epub.package.nav.audit", "completed") {
		t.Errorf("nav.audit upstream should complete as diagnostic: %#v", env.Events)
	}
	if !hasEvent(env.Events, "epub.structure.normalize", "completed") {
		t.Fatalf("target stage did not run to completion: %#v", env.Events)
	}
	upFindings, ok := env.Facts["epub.package.nav.audit.findingsByLevel"].(upstreamFindingsByLevel)
	if !ok {
		t.Fatalf("facts[epub.package.nav.audit.findingsByLevel] = %#v", env.Facts["epub.package.nav.audit.findingsByLevel"])
	}
	if upFindings.Error == 0 {
		t.Errorf("expected ≥1 error-level nav.audit finding on the generated EPUB, got %#v", upFindings)
	}
	if !hasFindingID(env.Findings, "upstream.diagnostics") {
		t.Errorf("upstream.diagnostics info finding missing: %#v", env.Findings)
	}
	// 信封里唯一允许的 error 来源是红线；上游诊断不得以 error 出现。
	redlineErrors := 0
	for _, f := range env.Findings {
		if f.Level != "error" {
			continue
		}
		if !strings.HasPrefix(f.ID, "redline.") {
			t.Errorf("envelope carries non-redline error-level finding: %#v", f)
			continue
		}
		redlineErrors++
	}
	if redlineErrors > 0 {
		if env.Status != report.StatusFailed || outcome.ExitCode != ExitFailed {
			t.Fatalf("status = %q exit = %d with %d redline errors, want failed / 1", env.Status, outcome.ExitCode, redlineErrors)
		}
		if !hasEvent(env.Events, "redline", "failed") {
			t.Errorf("redline:failed event missing: %#v", env.Events)
		}
	} else if env.Status != report.StatusPlanned || outcome.ExitCode != ExitOK {
		t.Fatalf("status = %q exit = %d without redline errors, want planned / 0", env.Status, outcome.ExitCode)
	}
	if env.Output != nil {
		t.Errorf("dry-run must not write output: %#v", env.Output)
	}
}

// TestRunMetadataRedlineWritesOutputButFails：目标能力声明 metadata 红线却
// 有意改写 OPF 标题（对齐 epub.metadata.edit 的契约形状）→ exit 1、failed，
// 输出仍写出。
func TestRunMetadataRedlineWritesOutputButFails(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.meta.edit", nil, true, []string{"metadata"})
	installTestRunner(t, "test.meta.edit", func(_ context.Context, b *book.Book, _ Args, _ Upstream) (report.Result, error) {
		data, err := b.Current("OEBPS/content.opf")
		if err != nil {
			return report.Result{}, err
		}
		old := []byte("<dc:title>书</dc:title>")
		i := bytes.Index(data, old)
		if i < 0 {
			return report.Result{}, errors.New("fixture title not found")
		}
		if err := b.Apply([]editset.Edit{{Path: "OEBPS/content.opf", Offset: int64(i), Length: int64(len(old)), Replacement: []byte("<dc:title>新书名</dc:title>")}}); err != nil {
			return report.Result{}, err
		}
		return report.Result{Capability: "test.meta.edit", Status: report.StatusComplete}, nil
	})

	out := filepath.Join(t.TempDir(), "out.epub")
	result, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: "test.meta.edit", InputPath: buildSampleEpub(t), OutputPath: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Envelope.Status != report.StatusFailed || result.ExitCode != ExitFailed {
		t.Fatalf("outcome = %#v, want failed / exit 1", result.Envelope)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output must exist for diff review: %v", err)
	}
	if result.Envelope.Output == nil || result.Envelope.Output.SHA256 == "" {
		t.Fatalf("output artifact = %#v", result.Envelope.Output)
	}
	if !hasFindingID(result.Envelope.Findings, "redline.metadata") {
		t.Fatalf("redline.metadata finding missing: %#v", result.Envelope.Findings)
	}
}

func TestDryRunMetadataRedlineSuggestsApplyingReviewedCandidate(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.meta.edit", nil, true, []string{"metadata"})
	installTestRunner(t, "test.meta.edit", func(_ context.Context, b *book.Book, _ Args, _ Upstream) (report.Result, error) {
		data, err := b.Current("OEBPS/content.opf")
		if err != nil {
			return report.Result{}, err
		}
		old := []byte("<dc:title>书</dc:title>")
		i := bytes.Index(data, old)
		if i < 0 {
			return report.Result{}, errors.New("fixture title not found")
		}
		if err := b.Apply([]editset.Edit{{Path: "OEBPS/content.opf", Offset: int64(i), Length: int64(len(old)), Replacement: []byte("<dc:title>新书名</dc:title>")}}); err != nil {
			return report.Result{}, err
		}
		return report.Result{Capability: "test.meta.edit", Status: report.StatusComplete}, nil
	})

	outcome, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: "test.meta.edit", InputPath: buildSampleEpub(t),
		OutputPath: filepath.Join(t.TempDir(), "candidate.epub"), DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Envelope.Status != report.StatusFailed || outcome.ExitCode != ExitFailed || !hasFindingID(outcome.Envelope.Findings, "redline.metadata") {
		t.Fatalf("dry-run redline outcome = status %q exit %d findings %#v", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	if len(outcome.Envelope.NextCommands) != 1 || strings.Contains(outcome.Envelope.NextCommands[0], "--dry-run") {
		t.Fatalf("dry-run redline nextCommands = %q, want one apply command", outcome.Envelope.NextCommands)
	}
}
