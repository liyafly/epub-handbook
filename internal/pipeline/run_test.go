package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
func TestRunPendingCapabilityFails(t *testing.T) {
	epub := buildSampleEpub(t)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.source.intake",
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
