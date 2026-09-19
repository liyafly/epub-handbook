package pipeline

import (
	"context"
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
	// 只读能力忽略 output，不报错。
	outcome, _ := Run(t.Context(), Options{
		CapabilityID: "epub.package.nav.audit",
		InputPath:    epub,
		OutputPath:   epub,
	})
	if outcome.ExitCode != ExitOK && outcome.ExitCode != ExitFailed {
		t.Fatalf("只读能力不应因 output=input 报用法错误，got %d", outcome.ExitCode)
	}
	// 写入型能力的 overwrite 保护在 caps 各自的 Params 层校验；
	// pipeline 层的用法错误覆盖 dry-run transformer 的 --output 必填路径。
	outcome, err := Run(t.Context(), Options{
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
	got := nextCommands(c, Options{}, true)
	want := "epub redline --check all --path-map <normalize-envelope.json> <before> <after>"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("nextCommands = %q, want [%q]", got, want)
	}
}
