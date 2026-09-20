package pipeline

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/liyafly/epub-handbook/internal/report"
)

func buildSourceDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chapter.txt"), []byte("\xEF\xBB\xBF第一章\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scan.pdf"), []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestRunSourceIntakeDirectory：sourceInput 能力接受目录输入，不 book.Open、
// 不写输出、status complete / exit 0，facts 带能力前缀。
func TestRunSourceIntakeDirectory(t *testing.T) {
	dir := buildSourceDir(t)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.source.intake",
		InputPath:    dir,
		Args:         Args{},
	})
	if err != nil {
		t.Fatal(err)
	}
	env := outcome.Envelope
	if env.Status != report.StatusComplete || outcome.ExitCode != ExitOK {
		t.Fatalf("status=%q exit=%d findings=%+v", env.Status, outcome.ExitCode, env.Findings)
	}
	if env.Output != nil {
		t.Errorf("planner must not write output: %+v", env.Output)
	}
	if env.Input == nil || env.Input.Path != dir || env.Input.SHA256 != "" {
		t.Errorf("directory input should carry path only: %+v", env.Input)
	}
	if env.Facts["epub.source.intake.fileCount"] != 2 {
		t.Errorf("fileCount fact = %v", env.Facts["epub.source.intake.fileCount"])
	}
	for _, k := range []string{"sourcePath", "roleCounts", "files", "plan", "blockers", "workspacePlan"} {
		if _, ok := env.Facts["epub.source.intake."+k]; !ok {
			t.Errorf("missing fact %s", k)
		}
	}
	hasPDF := false
	for _, f := range env.Findings {
		if f.ID == "intake.pdf-out-of-scope" && f.Level == "warn" {
			hasPDF = true
		}
		if f.ID == "input.invalid-epub" || f.ID == "capability.not-implemented" {
			t.Errorf("unexpected finding %s", f.ID)
		}
	}
	if !hasPDF {
		t.Errorf("missing intake.pdf-out-of-scope warn: %+v", env.Findings)
	}
	if len(env.NextCommands) != 0 {
		t.Errorf("no epub in input → no nextCommands, got %v", env.NextCommands)
	}
}

// TestRunSourceIntakeSingleFile：单个非 EPUB 文件同样可作为输入，信封 input 带 sha256。
func TestRunSourceIntakeSingleFile(t *testing.T) {
	dir := buildSourceDir(t)
	file := filepath.Join(dir, "chapter.txt")
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.source.intake",
		InputPath:    file,
	})
	if err != nil {
		t.Fatal(err)
	}
	env := outcome.Envelope
	if env.Status != report.StatusComplete || outcome.ExitCode != ExitOK {
		t.Fatalf("status=%q exit=%d findings=%+v", env.Status, outcome.ExitCode, env.Findings)
	}
	if env.Input == nil || env.Input.SHA256 == "" {
		t.Errorf("file input should carry sha256: %+v", env.Input)
	}
	if env.Facts["epub.source.intake.inputKind"] != "file" {
		t.Errorf("inputKind = %v", env.Facts["epub.source.intake.inputKind"])
	}
}

// TestRunSourceIntakeDryRunStaysComplete：只读 planner 的 --dry-run 不得变成
// approval-required。
func TestRunSourceIntakeDryRunStaysComplete(t *testing.T) {
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.source.intake",
		InputPath:    buildSourceDir(t),
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Envelope.Status != report.StatusComplete || outcome.ExitCode != ExitOK {
		t.Errorf("dry-run status=%q exit=%d", outcome.Envelope.Status, outcome.ExitCode)
	}
}

// TestRunSourceIntakeUsageErrors：--input 缺失或不存在 → 退出码 3。
func TestRunSourceIntakeUsageErrors(t *testing.T) {
	for name, in := range map[string]string{
		"missing":     "",
		"nonexistent": filepath.Join(t.TempDir(), "nope"),
	} {
		t.Run(name, func(t *testing.T) {
			outcome, err := Run(t.Context(), Options{CapabilityID: "epub.source.intake", InputPath: in})
			if err == nil || outcome.ExitCode != ExitUsage {
				t.Errorf("exit=%d err=%v, want 3", outcome.ExitCode, err)
			}
		})
	}
}

// TestRunSourceIntakeIsReady：epub capabilities 必须把它列为 ready。
func TestRunSourceIntakeIsReady(t *testing.T) {
	if !Implemented("epub.source.intake") || !IsSourceInput("epub.source.intake") {
		t.Error("epub.source.intake must be registered as a sourceInput capability")
	}
	if IsSourceInput("epub.style.demo.maintain") || !IsNoBook("epub.style.demo.maintain") {
		t.Error("styledemo noBook semantics must be untouched")
	}
}

// TestRunSourceIntakeMaxFilesUsage 回归 MEDIUM-2：非法 max_files 是用法错误
// （SPEC §8.5 退出码 3），不是 capability.run-failed（退出码 1）；<=0 必须
// 显式拒绝，不能静默回落到默认 5000（会被读成"不限"）。
func TestRunSourceIntakeMaxFilesUsage(t *testing.T) {
	dir := buildSourceDir(t)
	for _, v := range []string{"abc", "0", "-1", "1.5", " 2"} {
		t.Run(v, func(t *testing.T) {
			outcome, err := Run(t.Context(), Options{
				CapabilityID: "epub.source.intake",
				InputPath:    dir,
				Args:         Args{"max_files": v},
			})
			if err == nil {
				t.Fatalf("max_files=%q accepted; envelope=%+v", v, outcome.Envelope)
			}
			if outcome.ExitCode != ExitUsage {
				t.Errorf("max_files=%q exit=%d, want %d (usage)", v, outcome.ExitCode, ExitUsage)
			}
			var usageErr *UsageError
			if !errors.As(err, &usageErr) {
				t.Errorf("err = %T %v, want *UsageError", err, err)
			}
		})
	}
	// 合法值仍照常生效（2 个文件、上限 1 → 截断并 failed）。
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.source.intake", InputPath: dir, Args: Args{"max_files": "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitFailed || outcome.Envelope.Facts["epub.source.intake.fileCount"] != 1 {
		t.Errorf("max_files=1 exit=%d fileCount=%v", outcome.ExitCode, outcome.Envelope.Facts["epub.source.intake.fileCount"])
	}
}

// TestRunSourceIntakeNonRegularInput 回归 HIGH-3 的 pipeline 侧：信封的
// input.sha256 会先于 capability 读输入，非普通文件必须在那之前就被拒绝。
func TestRunSourceIntakeNonRegularInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("没有 /dev/null 这类字符设备")
	}
	done := make(chan int, 1)
	go func() {
		outcome, err := Run(t.Context(), Options{CapabilityID: "epub.source.intake", InputPath: "/dev/null"})
		if err == nil {
			t.Error("/dev/null accepted as source input")
		}
		done <- outcome.ExitCode
	}()
	select {
	case exit := <-done:
		if exit != ExitUsage {
			t.Errorf("exit=%d, want %d (usage)", exit, ExitUsage)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run blocked on a character device input")
	}
}

// TestRunSourceIntakeDryRunHasNoNextCommands 回归 MEDIUM-3：只读 planner 没有
// --output，dry-run 不得建议一条它根本不接受的 `--output <out.epub>`。
// SKILL.md 也写明纯源材料输入时 nextCommands 为空。
func TestRunSourceIntakeDryRunHasNoNextCommands(t *testing.T) {
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.source.intake",
		InputPath:    buildSourceDir(t),
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := outcome.Envelope.NextCommands; len(got) != 0 {
		t.Errorf("dry-run nextCommands = %v, want none", got)
	}
	// 写出型能力的 dry-run 建议不受影响。形态现在来自契约的 execution 字段
	// （而不是 Go 侧的 id 白名单），所以这里直接构造契约。
	single := Contract{ID: "epub.structure.normalize"}
	single.Execution.Input, single.Execution.Output = ExecInputEpub, ExecOutputSingle
	if got := nextCommands(single, Options{DryRun: true}, true); len(got) != 1 ||
		!strings.Contains(got[0], "--output '<out.epub>'") {
		t.Errorf("write capability dry-run nextCommands = %v", got)
	}
	multi := Contract{ID: "epub.package.split"}
	multi.Execution.Input, multi.Execution.Output = ExecInputEpub, ExecOutputMulti
	if got := nextCommands(multi, Options{DryRun: true}, true); len(got) != 1 ||
		!strings.Contains(got[0], "output_dir=<out-dir>") {
		t.Errorf("multi-output dry-run nextCommands = %v", got)
	}
}

// TestSourceIntakeEnvelopeGolden 回归 MEDIUM-4：
// testdata/envelope/source-intake-directory.report.json 此前只被
// archguard.TestReportSchema 做形状校验，内容可以静默漂移。这里逐字节对账：
// 与 cmd/epub 相同的序列化，仓库根按 golden 的约定归一成 <repo>。
func TestSourceIntakeEnvelopeGolden(t *testing.T) {
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join(root, "testdata", "envelope", "source-intake-directory.report.json")
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	// golden 的 input.path 是用户敲进去的相对路径，必须从仓库根运行。
	t.Chdir(root)
	outcome, err := Run(t.Context(), Options{
		CapabilityID: "epub.source.intake",
		InputPath:    filepath.ToSlash(filepath.Join("testdata", "sourceintake")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ExitCode != ExitOK {
		t.Fatalf("exit=%d findings=%+v", outcome.ExitCode, outcome.Envelope.Findings)
	}
	// cmd/epub 的 marshalEnvelope：MarshalIndent(env, "", "  ") + 换行。
	data, err := json.MarshalIndent(outcome.Envelope, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.ReplaceAll(string(append(data, '\n')), root, "<repo>")
	if got != string(want) {
		t.Errorf("信封与 golden 不一致。\n"+
			"  golden: %s\n"+
			"  行为变了就更新 golden，golden 错了就修实现 —— 但不要让两者继续分叉。\n"+
			"--- got ---\n%s\n--- want ---\n%s", golden, got, want)
	}
}
