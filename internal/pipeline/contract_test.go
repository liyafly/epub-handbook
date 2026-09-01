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
		Args: Args{"input": "forged.epub", "output": "forged.epub", "dry_run": "true", "legacy_report": "true"},
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

func TestRunReservedArgsAreOverriddenByGlobalOptions(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.args", nil, false, nil)
	var got Args
	installTestRunner(t, "test.args", func(_ context.Context, _ *book.Book, args Args, _ Upstream) (report.Result, error) {
		got = make(Args, len(args))
		for k, v := range args {
			got[k] = v
		}
		return report.Result{Capability: "test.args", Status: report.StatusComplete}, nil
	})

	input := buildSampleEpub(t)
	outcome, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: "test.args", InputPath: input,
		OutputPath: "actual-out.epub", DryRun: false, LegacyReport: false,
		Args: Args{"input": "forged-in", "output": "forged-out", "dry_run": "true", "legacy_report": "true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitOK {
		t.Fatalf("exit = %d", outcome.ExitCode)
	}
	want := map[string]string{"input": input, "output": "actual-out.epub", "dry_run": "false", "legacy_report": "false"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("args[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestRunDependencyFailureBlocksDownstreamAndOutput(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.fail.c", nil, true, nil)
	writeTestContract(t, root, "test.fail.b", []string{"test.fail.c"}, true, nil)
	writeTestContract(t, root, "test.fail.a", []string{"test.fail.b"}, true, nil)

	var calls []string
	installTestRunner(t, "test.fail.c", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		calls = append(calls, "c")
		return report.Result{Capability: "test.fail.c", Status: report.StatusFailed,
			Findings: []report.Finding{{Level: "error", ID: "test.failure", Title: "upstream failed"}}}, nil
	})
	installTestRunner(t, "test.fail.b", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		calls = append(calls, "b")
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
	if result.Envelope.Status != report.StatusFailed || result.ExitCode != ExitFailed {
		t.Fatalf("outcome = %#v, want failed", result)
	}
	if strings.Join(calls, "") != "c" {
		t.Fatalf("runner calls = %q, want c", calls)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output stat error = %v, output should not exist", err)
	}
}

func TestRunErrorFindingBlocksDownstreamAndOutput(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.finding.c", nil, true, nil)
	writeTestContract(t, root, "test.finding.a", []string{"test.finding.c"}, true, nil)

	called := false
	installTestRunner(t, "test.finding.c", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		return report.Result{Capability: "test.finding.c", Status: report.StatusComplete,
			Findings: []report.Finding{{Level: "error", ID: "test.error-finding", Title: "blocked"}}}, nil
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
	if result.Envelope.Status != report.StatusFailed || result.ExitCode != ExitFailed {
		t.Fatalf("outcome = %#v, want failed", result)
	}
	if called {
		t.Fatal("downstream runner ran after error finding")
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output stat error = %v, output should not exist", err)
	}
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

func TestRunRedlineFailureBlocksOutput(t *testing.T) {
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
		t.Fatalf("outcome = %#v, want failed", result)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output stat error = %v, output should not exist", err)
	}
	if len(result.Envelope.Findings) == 0 || !strings.HasPrefix(result.Envelope.Findings[0].ID, "redline.") {
		t.Fatalf("redline finding missing: %#v", result.Envelope.Findings)
	}
}

func TestRunRedlineValidatorErrorBlocksOutput(t *testing.T) {
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
		t.Fatalf("outcome = %#v, want failed", result)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output stat error = %v, output should not exist", err)
	}
	found := false
	for _, f := range result.Envelope.Findings {
		if f.ID == "redline.check-failed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("validator-error finding missing: %#v", result.Envelope.Findings)
	}
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
	dir := filepath.Join(root, "contracts", "capabilities", "v1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	c := Contract{SchemaVersion: "1", ID: id, Version: "1", Kind: "transformer", Requires: requires, RedLines: redLines}
	c.Permissions.RequiresWriteAccess = write
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
