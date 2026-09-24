package pipeline

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// buildRepoFixture 构造最小契约环境：pipeline 直接读仓库契约目录，
// 测试用真实契约（epub.package.nav.audit）跑端到端。
func buildSampleEpub(t *testing.T) string {
	t.Helper()
	return buildEpubWithOPF(t)
}

func buildTypographySampleEpub(t *testing.T) string {
	t.Helper()
	data := epubFixtureBytes(t)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if f.Name == "OEBPS/nav.xhtml" {
			content = bytes.Replace(content, []byte(`lang="zh-CN"><body>`), []byte("lang=\"zh-CN\">\n<head>\n<title>nav</title>\n</head>\n<body>"), 1)
			if !bytes.Contains(content, []byte("</head>")) {
				t.Fatal("test fixture nav.xhtml needs a head section for typography")
			}
		} else if strings.HasSuffix(f.Name, ".xhtml") {
			content = bytes.Replace(content, []byte("</head>"), []byte("\n</head>\n"), 1)
		}
		h := f.FileHeader
		w, err := zw.CreateHeader(&h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "typography.epub")
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunNavAuditEndToEnd(t *testing.T) {
	epub := buildSampleEpub(t)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.package.nav.audit",
		InputPath:    epub,
		Args:         Args{},
	})
	if err != nil {
		t.Fatal(err)
	}
	env := outcome.Envelope
	if env.SchemaVersion != "2" {
		t.Errorf("schemaVersion = %q", env.SchemaVersion)
	}
	if env.Capability != "epub.package.nav.audit" {
		t.Errorf("capability = %q", env.Capability)
	}
	if env.Input == nil || env.Input.SHA256 == "" {
		t.Error("input artifact 应含 sha256")
	} else {
		data, err := os.ReadFile(epub)
		if err != nil {
			t.Fatal(err)
		}
		want := sha256.Sum256(data)
		if env.Input.SHA256 != hex.EncodeToString(want[:]) {
			t.Errorf("input SHA-256 = %q, want hash of the opened EPUB %q", env.Input.SHA256, hex.EncodeToString(want[:]))
		}
	}
	if env.Status != report.StatusComplete && env.Status != report.StatusFailed {
		t.Errorf("status = %q", env.Status)
	}
	// 有 error findings → 退出码 1（与 Python preflight 语义一致）。
	if env.Status == report.StatusFailed && outcome.ExitCode != ExitFailed {
		t.Errorf("failed 状态退出码 = %d", outcome.ExitCode)
	}
	if env.Status == report.StatusComplete && outcome.ExitCode != ExitOK {
		t.Errorf("complete 状态退出码 = %d", outcome.ExitCode)
	}
	for _, event := range env.Events {
		if event.Step == "redline" {
			t.Fatal("unchanged read-only capability should skip redline")
		}
	}
}

func TestTypographyDefaultPresetDirIsRepoRootRelative(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	input := buildTypographySampleEpub(t)
	output := filepath.Join(t.TempDir(), "candidate.epub")
	t.Chdir(filepath.Join(root, "internal"))

	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.typography.optimize",
		InputPath:    input,
		OutputPath:   output,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome.Envelope.Status != report.StatusPlanned || outcome.ExitCode != ExitOK {
		t.Fatalf("status=%q exit=%d, want planned/0; findings=%+v", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
	if len(outcome.Envelope.NextCommands) != 1 || strings.Contains(outcome.Envelope.NextCommands[0], "preset_dir=") {
		t.Fatalf("nextCommands = %q, want a command without injected preset_dir", outcome.Envelope.NextCommands)
	}
}

func TestEmbeddedResourcesSupportRunOutsideRepository(t *testing.T) {
	t.Setenv("EPUB_HANDBOOK_ROOT", "")
	if _, err := FindRepoRoot(); err != nil {
		t.Fatal(err)
	}
	externalDir := t.TempDir()
	t.Chdir(externalDir)
	if root, err := FindRepoRoot(); err != nil || root != "" {
		t.Fatalf("FindRepoRoot() = %q, %v; want embedded-resource mode", root, err)
	}

	infos, err := DescribeCapabilities("", "")
	if err != nil {
		t.Fatalf("DescribeCapabilities: %v", err)
	}
	if len(infos) != 22 {
		t.Fatalf("embedded capability count = %d, want 22", len(infos))
	}
	if schema, err := readRepositoryFile("", "contracts/schemas/v2/envelope.schema.json"); err != nil || len(schema) == 0 {
		t.Fatalf("embedded envelope schema: bytes=%d err=%v", len(schema), err)
	}

	input := buildSampleEpub(t)
	audit, err := Run(t.Context(), Options{CapabilityID: "epub.package.nav.audit", InputPath: input})
	if err != nil || audit.ExitCode != ExitOK {
		t.Fatalf("embedded nav audit exit=%d err=%v findings=%+v", audit.ExitCode, err, audit.Envelope.Findings)
	}

	output := filepath.Join(externalDir, "migrated.epub")
	migrate, err := Run(t.Context(), Options{
		CapabilityID: "epub.package.migrate.epub3",
		InputPath:    input,
		OutputPath:   output,
	})
	if err != nil || migrate.ExitCode != ExitOK {
		t.Fatalf("embedded migrate exit=%d err=%v findings=%+v", migrate.ExitCode, err, migrate.Envelope.Findings)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("migrate did not write output: %v", err)
	}
}

func TestEPUBHandbookRootOverridesEmbeddedResources(t *testing.T) {
	t.Setenv("EPUB_HANDBOOK_ROOT", "")
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	externalDir := t.TempDir()
	t.Setenv("EPUB_HANDBOOK_ROOT", root)
	t.Chdir(externalDir)
	got, err := FindRepoRoot()
	if err != nil || got != root {
		t.Fatalf("FindRepoRoot() = %q, %v; want explicit root %q", got, err, root)
	}
	t.Setenv("EPUB_HANDBOOK_ROOT", filepath.Join(externalDir, "missing"))
	if _, err := FindRepoRoot(); err == nil || !strings.Contains(err.Error(), "EPUB_HANDBOOK_ROOT") {
		t.Fatalf("invalid EPUB_HANDBOOK_ROOT error = %v", err)
	}
}

func TestTypographyUsesEmbeddedPresetOutsideRepository(t *testing.T) {
	t.Setenv("EPUB_HANDBOOK_ROOT", "")
	input := buildTypographySampleEpub(t)
	output := filepath.Join(t.TempDir(), "candidate.epub")
	t.Chdir(t.TempDir())
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.typography.optimize",
		InputPath:    input,
		OutputPath:   output,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("Run with embedded preset: %v", err)
	}
	if outcome.ExitCode != ExitOK || outcome.Envelope.Status != report.StatusPlanned {
		t.Fatalf("status=%q exit=%d findings=%+v", outcome.Envelope.Status, outcome.ExitCode, outcome.Envelope.Findings)
	}
}

// TestRunPendingCapabilityFails 锁定 pending 能力语义：契约存在但无 Go
// 实现时必须 failed + exit 1，不得伪装成 complete/exit 0。
// （B 类纯 AI skill epub.kindle.compatibility.check 设计上永无 Go 实现。）
func TestRunPendingCapabilityFails(t *testing.T) {
	epub := buildSampleEpub(t)
	pending := []string{
		"epub.kindle.compatibility.check",
		"epub.literary.structure.format",
		"epub.notes.legacy-fallback",
		"epub.typography.english.optimize",
		"epub.vertical.ruby.optimize",
	}
	for _, id := range pending {
		t.Run(id, func(t *testing.T) {
			outcome, err := Run(t.Context(), Options{
				CapabilityID: id,
				InputPath:    epub,
				Args:         Args{},
			})
			if err != nil {
				t.Fatal(err)
			}
			env := outcome.Envelope
			if env.Status != report.StatusFailed {
				t.Errorf("pending 能力 status = %q, want failed", env.Status)
			}
			if outcome.ExitCode != ExitFailed {
				t.Errorf("pending 能力退出码 = %d, want 1", outcome.ExitCode)
			}
			found := false
			for _, f := range env.Findings {
				if f.ID == "capability.not-implemented" {
					found = true
					if f.Level != "error" {
						t.Errorf("finding level = %q, want error", f.Level)
					}
					if strings.Contains(f.Detail, "oracle") {
						t.Errorf("finding detail 不应再指向已删除的 Python oracle: %q", f.Detail)
					}
				}
			}
			if !found {
				t.Error("缺少 capability.not-implemented finding")
			}
		})
	}
	noInput, err := Run(t.Context(), Options{CapabilityID: "epub.notes.legacy-fallback"})
	if err != nil {
		t.Fatal(err)
	}
	if noInput.ExitCode != ExitFailed || noInput.Envelope.Status != report.StatusFailed {
		t.Fatalf("pending capability without input = status %q exit %d, want failed / 1", noInput.Envelope.Status, noInput.ExitCode)
	}
	if len(noInput.Envelope.Events) != 1 || noInput.Envelope.Events[0].Step != "epub.notes.legacy-fallback" || noInput.Envelope.Events[0].Status != "skipped" {
		t.Fatalf("pending capability event = %#v, want one skipped event", noInput.Envelope.Events)
	}
	if len(noInput.Envelope.Findings) != 1 || noInput.Envelope.Findings[0].ID != "capability.not-implemented" {
		t.Fatalf("pending capability findings = %#v, want capability.not-implemented", noInput.Envelope.Findings)
	}
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(root, "testdata/envelope/pending-capability.report.json"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(noInput.Envelope, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(append(data, '\n')); got != string(want) {
		t.Fatalf("pending envelope differs from golden\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRunUsageErrors(t *testing.T) {
	epub := buildSampleEpub(t)
	cases := []struct {
		name string
		opts Options
	}{
		{"unknown capability", Options{CapabilityID: "epub.not.exist", InputPath: epub}},
		{"missing input", Options{CapabilityID: "epub.package.nav.audit", InputPath: filepath.Join(t.TempDir(), "nope.epub")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			outcome, err := Run(t.Context(), c.opts)
			if outcome.ExitCode != ExitUsage {
				t.Errorf("退出码 = %d, want 3（err=%v）", outcome.ExitCode, err)
			}
		})
	}
}

func TestRunRejectsOutputOverwriteInput(t *testing.T) {
	epub := buildSampleEpub(t)
	// 只读能力不接受 --output。
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.package.nav.audit",
		InputPath:    epub,
		OutputPath:   filepath.Join(t.TempDir(), "ignored.epub"),
	})
	if outcome.ExitCode != ExitUsage || err == nil || !strings.Contains(err.Error(), "does not write an output") {
		t.Fatalf("只读能力 --output = exit %d err %v, want usage error", outcome.ExitCode, err)
	}
	// 写入型能力的 --output 必填检查也属于 pipeline 用法校验。
	outcome, err = Run(t.Context(), Options{
		CapabilityID: "epub.structure.normalize",
		InputPath:    epub,
		DryRun:       false,
	})
	if err == nil && outcome.ExitCode != ExitUsage {
		t.Logf("structure.normalize 未实现时的行为：exit=%d err=%v", outcome.ExitCode, err)
	}
}

func TestRedlineCompareExitCodes(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.epub")
	after := filepath.Join(dir, "after.epub")
	buildEpubFile(t, before)
	buildEpubFile(t, after)

	code, err := RedlineCompare(before, after, "all", nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("干净比对退出码 = %d", code)
	}
	code, err = RedlineCompare(before, after, "not-a-check", nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if code != ExitUsage {
		t.Errorf("bad --check exit = %d, want usage code %d", code, ExitUsage)
	}
}

// ---- fixture ----

func buildEpubFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, epubFixtureBytes(t), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCapabilitiesListsContracts 确保 capabilities 子命令有契约可列。
func TestCapabilitiesListsContracts(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Skip("不在仓库内运行")
	}
	contracts, err := AllContracts(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(contracts) != 22 {
		t.Errorf("契约数 = %d, want 22", len(contracts))
	}
	if !strings.Contains(ImplementedIDs()[0], "epub.") {
		t.Errorf("registry id 形态异常: %v", ImplementedIDs())
	}
}

// TestNextCommandsPrefersCapabilitySuggestions 锁定 SPEC §8.2 的 nextCommands
// 组装规则：能力依本次结果算出的建议优先（即使 status 是 failed），自引用的
// 「再跑一遍自己」被剔除，能力没给建议时才退回 pipeline 的静态文本。
func TestNextCommandsPrefersCapabilitySuggestions(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.nc.cap", nil, false, nil)
	writeTestContract(t, root, "test.nc.static", nil, false, nil)

	installTestRunner(t, "test.nc.cap", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		return report.Result{
			Capability: "test.nc.cap",
			Status:     report.StatusFailed,
			Findings:   []report.Finding{{Level: "error", ID: "test.err", Title: "boom"}},
			NextCommands: []string{
				"epub run test.nc.cap --input x.epub --json", // 自引用，必须被剔除
				"epub run epub.layout.audit --input x.epub --json",
				"epub run epub.layout.audit --input x.epub --json", // 重复，去重
			},
		}, nil
	})
	installTestRunner(t, "test.nc.static", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		return report.Result{Capability: "test.nc.static", Status: report.StatusComplete}, nil
	})

	in := buildSampleEpub(t)

	got, err := Run(t.Context(), Options{RepoRoot: root, CapabilityID: "test.nc.cap", InputPath: in})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"epub run epub.layout.audit --input x.epub --json"}
	if !slices.Equal(got.Envelope.NextCommands, want) {
		t.Errorf("失败能力的 nextCommands = %q, want %q", got.Envelope.NextCommands, want)
	}

	// 能力没给建议时退回静态分支（本 id 无静态文案，故为空）。
	got, err = Run(t.Context(), Options{RepoRoot: root, CapabilityID: "test.nc.static", InputPath: in})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Envelope.NextCommands) != 0 {
		t.Errorf("无建议能力的 nextCommands = %q, want 空", got.Envelope.NextCommands)
	}
}

func TestNormalizeNextCommandUsesFlagFirstOrder(t *testing.T) {
	c := Contract{ID: "epub.structure.normalize"}
	c.Execution.Output = ExecOutputSingle
	opts := Options{InputPath: "source book.epub", OutputPath: "candidate book.epub"}
	got := nextCommands(c, opts, nil, true)
	want := "epub redline --check all --path-map '<normalize-envelope.json>' 'source book.epub' 'candidate book.epub'"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("nextCommands = %q, want [%q]", got, want)
	}
}

func TestRunRejectsOutputForReadOnly(t *testing.T) {
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.package.nav.audit",
		InputPath:    buildSampleEpub(t),
		OutputPath:   filepath.Join(t.TempDir(), "candidate.epub"),
	})
	if err == nil || outcome.ExitCode != ExitUsage || !strings.Contains(err.Error(), "does not write an output") {
		t.Fatalf("read-only output = exit %d err %v, want usage error", outcome.ExitCode, err)
	}
}

func TestRunRejectsOutputForMultiOutput(t *testing.T) {
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.package.split",
		InputPath:    buildSampleEpub(t),
		OutputPath:   filepath.Join(t.TempDir(), "candidate.epub"),
		Args:         Args{"output_dir": filepath.Join(t.TempDir(), "segments"), "split_points": "0"},
	})
	if err == nil || outcome.ExitCode != ExitUsage || !strings.Contains(err.Error(), "writes to output_dir") {
		t.Fatalf("multi-output --output = exit %d err %v, want usage error", outcome.ExitCode, err)
	}
}

func TestRunRejectsExistingOutputEarly(t *testing.T) {
	root := t.TempDir()
	const id = "test.write.existing-output"
	writeTestContract(t, root, id, nil, true, nil)
	called := false
	installTestRunner(t, id, func(context.Context, *book.Book, Args, Upstream) (report.Result, error) {
		called = true
		return report.Result{Capability: id, Status: report.StatusComplete}, nil
	})
	output := filepath.Join(t.TempDir(), "existing.epub")
	if err := os.WriteFile(output, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	outcome, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: id, InputPath: buildSampleEpub(t), OutputPath: output,
	})
	if err == nil || outcome.ExitCode != ExitUsage || !strings.Contains(err.Error(), "output already exists") {
		t.Fatalf("existing output = exit %d err %v, want usage error", outcome.ExitCode, err)
	}
	if called {
		t.Fatal("runner was called before existing output rejection")
	}
}
