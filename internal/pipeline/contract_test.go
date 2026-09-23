package pipeline

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
)

func TestResolveChainRequiresDependenciesFirst(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.chain.c", nil, false, nil)
	writeTestContract(t, root, "test.chain.b", []string{"test.chain.c"}, false, nil)
	writeTestContract(t, root, "test.chain.a", []string{"test.chain.b"}, false, nil)

	chain, err := ResolveChain(root, "test.chain.a")
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(chain))
	for i, c := range chain {
		got[i] = c.ID
	}
	want := []string{"test.chain.c", "test.chain.b", "test.chain.a"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("chain = %v, want %v", got, want)
	}
}

func TestResolveChainUnknownRootAndDependency(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveChain(root, "test.missing.root"); !errors.Is(err, ErrUnknownCapability) {
		t.Fatalf("unknown root error = %v, want ErrUnknownCapability", err)
	}

	writeTestContract(t, root, "test.missing.parent", []string{"test.missing.dep"}, false, nil)
	if _, err := ResolveChain(root, "test.missing.parent"); !errors.Is(err, ErrUnknownCapability) {
		t.Fatalf("unknown dependency error = %v, want ErrUnknownCapability", err)
	}
}

func TestResolveChainDetectsCycle(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.cycle.a", []string{"test.cycle.b"}, false, nil)
	writeTestContract(t, root, "test.cycle.b", []string{"test.cycle.a"}, false, nil)

	_, err := ResolveChain(root, "test.cycle.a")
	if err == nil || !strings.Contains(err.Error(), "requires 环") {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestRunExecutesFullChainAndExposesUpstream(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.run.c", nil, false, nil)
	writeTestContract(t, root, "test.run.b", []string{"test.run.c"}, false, nil)
	writeTestContract(t, root, "test.run.a", []string{"test.run.b"}, false, nil)

	var order []string
	installTestRunner(t, "test.run.c", func(_ context.Context, _ *book.Book, args Args, up Upstream) (report.Result, error) {
		order = append(order, "c")
		if len(up) != 0 {
			return report.Result{}, fmt.Errorf("c unexpectedly received upstream: %v", up)
		}
		if args.Get("input") == "" {
			return report.Result{}, errors.New("input was not forwarded")
		}
		return report.Result{Capability: "test.run.c", Status: report.StatusComplete,
			Facts: map[string]any{"value": "from-c"}}, nil
	})
	installTestRunner(t, "test.run.b", func(_ context.Context, _ *book.Book, _ Args, up Upstream) (report.Result, error) {
		order = append(order, "b")
		if got := up["test.run.c"].Facts["value"]; got != "from-c" {
			return report.Result{}, fmt.Errorf("upstream c value = %v", got)
		}
		return report.Result{Capability: "test.run.b", Status: report.StatusComplete,
			Facts: map[string]any{"value": "from-b"}}, nil
	})
	installTestRunner(t, "test.run.a", func(_ context.Context, _ *book.Book, _ Args, up Upstream) (report.Result, error) {
		order = append(order, "a")
		if got := up["test.run.b"].Facts["value"]; got != "from-b" {
			return report.Result{}, fmt.Errorf("upstream b value = %v", got)
		}
		return report.Result{Capability: "test.run.a", Status: report.StatusComplete,
			Facts: map[string]any{"value": "from-a"}}, nil
	})

	outcome, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: "test.run.a", InputPath: buildSampleEpub(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitOK || outcome.Envelope.Status != report.StatusComplete {
		t.Fatalf("outcome = %#v, want complete", outcome)
	}
	if strings.Join(order, "") != "cba" {
		t.Fatalf("execution order = %q, want cba", order)
	}
	if got := outcome.Envelope.Facts["test.run.c.value"]; got != "from-c" {
		t.Errorf("aggregated c fact = %v", got)
	}
	if got := outcome.Envelope.Facts["test.run.b.value"]; got != "from-b" {
		t.Errorf("aggregated b fact = %v", got)
	}
	if got := outcome.Envelope.Facts["test.run.a.value"]; got != "from-a" {
		t.Errorf("aggregated a fact = %v", got)
	}
	for _, event := range outcome.Envelope.Events {
		if event.Step == "test.run.a" && event.Status == "completed" {
			return
		}
	}
	t.Error("missing final stage event")
}

func TestRunRejectsReservedArgs(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.args", nil, false, nil)
	called := false
	installTestRunner(t, "test.args", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		called = true
		return report.Result{Capability: "test.args", Status: report.StatusComplete}, nil
	})

	input := buildSampleEpub(t)
	for _, key := range []string{"input", "output", "dry_run"} {
		t.Run(key, func(t *testing.T) {
			outcome, err := Run(t.Context(), Options{
				RepoRoot: root, CapabilityID: "test.args", InputPath: input,
				OutputPath: filepath.Join(t.TempDir(), "actual-out.epub"),
				Args:       Args{key: "forged"},
			})
			if err == nil || outcome.ExitCode != ExitUsage {
				t.Fatalf("outcome=%#v err=%v, want usage error", outcome, err)
			}
			if !strings.Contains(err.Error(), "flag") {
				t.Errorf("error = %q, want mention of global flag", err)
			}
			if called {
				t.Fatal("runner was called for a reserved KEY=VALUE argument")
			}
		})
	}
}

// TestRunUpstreamStatusFailedIsDiagnosticNotBlocking 锁定链语义：requires
// 上游 Status failed（如真书上的 nav.audit）不得阻断目标 stage 与落盘，
// 其 findings 落入 facts，信封只得到 info 摘要。
func TestRunUpstreamStatusFailedIsDiagnosticNotBlocking(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.fail.c", nil, false, nil)
	writeTestContract(t, root, "test.fail.b", []string{"test.fail.c"}, false, nil)
	writeTestContract(t, root, "test.fail.a", []string{"test.fail.b"}, true, nil)

	var calls []string
	installTestRunner(t, "test.fail.c", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		calls = append(calls, "c")
		return report.Result{Capability: "test.fail.c", Status: report.StatusFailed,
			Findings: []report.Finding{
				{Level: "error", ID: "test.failure", Title: "upstream failed"},
				{Level: "warn", ID: "test.warning", Title: "upstream warned"},
			}}, nil
	})
	installTestRunner(t, "test.fail.b", func(_ context.Context, _ *book.Book, _ Args, up Upstream) (report.Result, error) {
		calls = append(calls, "b")
		if up["test.fail.c"].Status != report.StatusFailed {
			return report.Result{}, errors.New("upstream c result not exposed")
		}
		return report.Result{Capability: "test.fail.b", Status: report.StatusComplete}, nil
	})
	installTestRunner(t, "test.fail.a", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		calls = append(calls, "a")
		return report.Result{Capability: "test.fail.a", Status: report.StatusComplete}, nil
	})

	out := filepath.Join(t.TempDir(), "out.epub")
	result, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: "test.fail.a", InputPath: buildSampleEpub(t), OutputPath: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Envelope.Status != report.StatusComplete || result.ExitCode != ExitOK {
		t.Fatalf("outcome = %#v, want complete/exit 0", result.Envelope)
	}
	if strings.Join(calls, "") != "cba" {
		t.Fatalf("runner calls = %q, want cba", calls)
	}
	if result.Envelope.Output == nil || result.Envelope.Output.SHA256 == "" {
		t.Fatalf("output artifact = %#v", result.Envelope.Output)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output missing: %v", err)
	}
	assertUpstreamDiagnostics(t, result.Envelope, "test.fail.c", report.StatusFailed, 1, 1)
	if got := result.Envelope.Facts["test.fail.b.status"]; got != report.StatusComplete {
		t.Errorf("facts[test.fail.b.status] = %v", got)
	}
}

// TestRunUpstreamErrorFindingIsDiagnosticNotBlocking：上游 complete 但带
// error finding，同样不阻断；信封 findings 不得含 error 级条目。
func TestRunUpstreamErrorFindingIsDiagnosticNotBlocking(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.finding.c", nil, false, nil)
	writeTestContract(t, root, "test.finding.a", []string{"test.finding.c"}, true, nil)

	called := false
	installTestRunner(t, "test.finding.c", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		return report.Result{Capability: "test.finding.c", Status: report.StatusComplete,
			Findings: []report.Finding{{Level: "error", ID: "test.error-finding", Title: "diagnostic"}}}, nil
	})
	installTestRunner(t, "test.finding.a", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		called = true
		return report.Result{Capability: "test.finding.a", Status: report.StatusComplete}, nil
	})

	out := filepath.Join(t.TempDir(), "out.epub")
	result, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: "test.finding.a", InputPath: buildSampleEpub(t), OutputPath: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Envelope.Status != report.StatusComplete || result.ExitCode != ExitOK {
		t.Fatalf("outcome = %#v, want complete/exit 0", result.Envelope)
	}
	if !called {
		t.Fatal("target runner did not run after upstream error finding")
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output missing: %v", err)
	}
	assertUpstreamDiagnostics(t, result.Envelope, "test.finding.c", report.StatusComplete, 1, 0)
}

// TestRunUpstreamRunnerErrorStillBlocks：上游 runner 返回 Go error 是工具
// 故障而非书的问题，仍然阻断目标 stage 与落盘。
func TestRunUpstreamRunnerErrorStillBlocks(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.runerr.c", nil, false, nil)
	writeTestContract(t, root, "test.runerr.a", []string{"test.runerr.c"}, true, nil)

	called := false
	installTestRunner(t, "test.runerr.c", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		return report.Result{}, errors.New("tool exploded")
	})
	installTestRunner(t, "test.runerr.a", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		called = true
		return report.Result{Capability: "test.runerr.a", Status: report.StatusComplete}, nil
	})

	out := filepath.Join(t.TempDir(), "out.epub")
	result, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: "test.runerr.a", InputPath: buildSampleEpub(t), OutputPath: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Envelope.Status != report.StatusFailed || result.ExitCode != ExitFailed {
		t.Fatalf("outcome = %#v, want failed", result.Envelope)
	}
	if called {
		t.Fatal("target runner ran after upstream runner error")
	}
	if result.Envelope.Output != nil {
		t.Fatalf("output artifact should be nil: %#v", result.Envelope.Output)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output stat error = %v, output should not exist", err)
	}
	if !hasFindingID(result.Envelope.Findings, "capability.run-failed") {
		t.Fatalf("capability.run-failed finding missing: %#v", result.Envelope.Findings)
	}
}

// assertUpstreamDiagnostics 断言上游 stage 的非阻断诊断形状。
func assertUpstreamDiagnostics(t *testing.T, env report.Envelope, id, wantStatus string, wantErr, wantWarn int) {
	t.Helper()
	if got := env.Facts[id+".status"]; got != wantStatus {
		t.Errorf("facts[%s.status] = %v, want %s", id, got, wantStatus)
	}
	upFindings, ok := env.Facts[id+".findings"].([]report.Finding)
	if !ok {
		t.Fatalf("facts[%s.findings] = %#v, want []report.Finding", id, env.Facts[id+".findings"])
	}
	gotErr, gotWarn := 0, 0
	for _, f := range upFindings {
		switch f.Level {
		case "error":
			gotErr++
		case "warn":
			gotWarn++
		}
	}
	if gotErr != wantErr || gotWarn != wantWarn {
		t.Errorf("facts[%s.findings] = %d error / %d warn, want %d / %d", id, gotErr, gotWarn, wantErr, wantWarn)
	}
	for _, f := range env.Findings {
		if f.Level == "error" {
			t.Errorf("envelope has error-level finding from upstream: %#v", f)
		}
	}
	info := false
	for _, f := range env.Findings {
		if f.ID == "upstream.diagnostics" && f.Level == "info" && f.Location == id {
			info = true
		}
	}
	if !info {
		t.Errorf("upstream.diagnostics info finding missing: %#v", env.Findings)
	}
	stageEvent := false
	for _, e := range env.Events {
		if e.Step == id && e.Status == "completed" && strings.HasPrefix(e.Message, "diagnostic:") {
			stageEvent = true
		}
	}
	if !stageEvent {
		t.Errorf("upstream diagnostic event missing: %#v", env.Events)
	}
}

func hasFindingID(findings []report.Finding, id string) bool {
	for _, f := range findings {
		if f.ID == id {
			return true
		}
	}
	return false
}

func TestRunDRMPreflightBlocksRunnerAndOutput(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.drm", nil, true, []string{"drm"})
	called := false
	installTestRunner(t, "test.drm", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		called = true
		return report.Result{Capability: "test.drm", Status: report.StatusComplete}, nil
	})

	out := filepath.Join(t.TempDir(), "out.epub")
	result, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: "test.drm", InputPath: buildEncryptedEpub(t), OutputPath: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Envelope.Status != report.StatusFailed || result.ExitCode != ExitFailed {
		t.Fatalf("outcome = %#v, want failed", result)
	}
	if called {
		t.Fatal("runner ran despite DRM preflight failure")
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output stat error = %v, output should not exist", err)
	}
	found := false
	for _, f := range result.Envelope.Findings {
		if f.ID == "redline.drm" {
			found = true
		}
	}
	if !found {
		t.Fatalf("DRM finding missing: %#v", result.Envelope.Findings)
	}
}

func TestRunDRMPreflightAllowsStaleOnlyEncryption(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.drm.stale", nil, true, []string{"drm"})
	called := false
	installTestRunner(t, "test.drm.stale", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		called = true
		return report.Result{Capability: "test.drm.stale", Status: report.StatusComplete}, nil
	})

	out := filepath.Join(t.TempDir(), "out.epub")
	result, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: "test.drm.stale", InputPath: buildStaleEncryptedEpub(t), OutputPath: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Envelope.Status != report.StatusComplete || result.ExitCode != ExitOK {
		t.Fatalf("outcome = %#v, want complete", result)
	}
	if !called {
		t.Fatal("runner did not run for stale-only encryption")
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output missing: %v", err)
	}
}

// TestRunRedlineFailureWritesOutputButFails：红线 error 把状态降为 failed /
// 退出码 1，但输出仍然写出并在信封 output 中报告，供人工 diff review。
func TestRunRedlineFailureWritesOutputButFails(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.redline", nil, true, []string{"text"})
	installTestRunner(t, "test.redline", func(_ context.Context, b *book.Book, _ Args, _ Upstream) (report.Result, error) {
		data, err := b.Current("OEBPS/c1.xhtml")
		if err != nil {
			return report.Result{}, err
		}
		old := []byte("段落。")
		i := bytes.Index(data, old)
		if i < 0 {
			return report.Result{}, errors.New("fixture text not found")
		}
		if err := b.Apply([]editset.Edit{{Path: "OEBPS/c1.xhtml", Offset: int64(i), Length: int64(len(old)), Replacement: []byte("改写。")}}); err != nil {
			return report.Result{}, err
		}
		return report.Result{Capability: "test.redline", Status: report.StatusComplete}, nil
	})

	out := filepath.Join(t.TempDir(), "out.epub")
	result, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: "test.redline", InputPath: buildSampleEpub(t), OutputPath: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Envelope.Status != report.StatusFailed || result.ExitCode != ExitFailed {
		t.Fatalf("outcome = %#v, want failed", result.Envelope)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output must be retained for diff review: %v", err)
	}
	if result.Envelope.Output == nil || result.Envelope.Output.Path != out || result.Envelope.Output.SHA256 == "" {
		t.Fatalf("output artifact = %#v, want path+sha256", result.Envelope.Output)
	}
	if len(result.Envelope.Findings) == 0 || !strings.HasPrefix(result.Envelope.Findings[0].ID, "redline.") {
		t.Fatalf("redline finding missing: %#v", result.Envelope.Findings)
	}
	if !hasEvent(result.Envelope.Events, "redline", "failed") || !hasEvent(result.Envelope.Events, "write-output", "completed") {
		t.Fatalf("events = %#v, want redline:failed and write-output:completed", result.Envelope.Events)
	}
}

// TestRunRedlineValidatorErrorWritesOutputButFails：红线校验器本身出错同样
// failed / 退出码 1，输出仍保留。
func TestRunRedlineValidatorErrorWritesOutputButFails(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.redline.error", nil, true, []string{"unknown-check"})
	installTestRunner(t, "test.redline.error", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		return report.Result{Capability: "test.redline.error", Status: report.StatusComplete}, nil
	})

	out := filepath.Join(t.TempDir(), "out.epub")
	result, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: "test.redline.error", InputPath: buildSampleEpub(t), OutputPath: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Envelope.Status != report.StatusFailed || result.ExitCode != ExitFailed {
		t.Fatalf("outcome = %#v, want failed", result.Envelope)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output must be retained for diff review: %v", err)
	}
	if result.Envelope.Output == nil || result.Envelope.Output.SHA256 == "" {
		t.Fatalf("output artifact = %#v, want sha256", result.Envelope.Output)
	}
	if !hasFindingID(result.Envelope.Findings, "redline.check-failed") {
		t.Fatalf("validator-error finding missing: %#v", result.Envelope.Findings)
	}
}

func hasEvent(events []report.Event, step, status string) bool {
	for _, e := range events {
		if e.Step == step && e.Status == status {
			return true
		}
	}
	return false
}

func TestRunSuccessfulSingleOutputWritesOnce(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.write", nil, true, nil)
	installTestRunner(t, "test.write", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		return report.Result{Capability: "test.write", Status: report.StatusComplete}, nil
	})

	out := filepath.Join(t.TempDir(), "out.epub")
	result, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: "test.write", InputPath: buildSampleEpub(t), OutputPath: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Envelope.Status != report.StatusComplete || result.ExitCode != ExitOK {
		t.Fatalf("outcome = %#v, want complete", result)
	}
	if result.Envelope.Output == nil || result.Envelope.Output.Path != out {
		t.Fatalf("output artifact = %#v", result.Envelope.Output)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output missing: %v", err)
	}
	if _, err := os.Stat(out + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary output stat error = %v", err)
	}
}

func writeTestContract(t *testing.T, root, id string, requires []string, write bool, redLines []string) {
	t.Helper()
	parameterPath := filepath.Join(root, "contracts/parameters/v2/cli.json")
	catalog := report.ParameterCatalog{SchemaVersion: "2", Capabilities: map[string]report.CapabilityDescription{}}
	if raw, err := os.ReadFile(parameterPath); err == nil {
		if err := json.Unmarshal(raw, &catalog); err != nil {
			t.Fatal(err)
		}
	}
	catalog.Capabilities[id] = report.CapabilityDescription{Description: "test capability", Parameters: map[string]report.Parameter{}}
	if err := os.MkdirAll(filepath.Dir(parameterPath), 0o755); err != nil {
		t.Fatal(err)
	}
	parameterJSON, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parameterPath, parameterJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "contracts", "capabilities", "v1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	c := Contract{SchemaVersion: "1", ID: id, Version: "1", Kind: "transformer", Requires: requires, RedLines: redLines}
	c.Permissions.RequiresWriteAccess = write
	// execution 是必填契约字段（capability-manifest.schema.json），pipeline 的
	// noBookCap / multiOutputCap / chainNeedsWrite 都读它。合成契约漏了它就等于
	// 声明「不落盘」，落盘相关的用例会全部走空路径。
	c.Execution.Input = ExecInputEpub
	c.Execution.Output = ExecOutputNone
	if write {
		c.Execution.Output = ExecOutputSingle
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func installTestRunner(t *testing.T, id string, runner Runner) {
	t.Helper()
	old, existed := registry[id]
	registry[id] = runner
	t.Cleanup(func() {
		if existed {
			registry[id] = old
		} else {
			delete(registry, id)
		}
	})
}

func buildEncryptedEpub(t *testing.T) string {
	return buildEncryptedEpubWith(t, "urn:test:unknown", "OEBPS/c1.xhtml")
}

func buildStaleEncryptedEpub(t *testing.T) string {
	return buildEncryptedEpubWith(t, "urn:test:unknown", "OEBPS/missing.xhtml")
}

func buildEncryptedEpubWith(t *testing.T, algorithm, target string) string {
	t.Helper()
	base := epubFixtureBytes(t)
	src, err := zip.NewReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range src.File {
		if err := w.Copy(f); err != nil {
			t.Fatal(err)
		}
	}
	encryption := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><encryption xmlns="http://www.w3.org/2001/04/xmlenc#"><EncryptedData><EncryptionMethod Algorithm=%q/><CipherData><CipherReference URI=%q/></CipherData></EncryptedData></encryption>`, algorithm, target)
	fw, err := w.Create("META-INF/encryption.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(encryption)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "encrypted.epub")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
