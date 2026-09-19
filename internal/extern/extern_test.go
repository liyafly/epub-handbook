package extern

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// presentTool 返回测试宿主机上必然存在的外部工具名。
func presentTool() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "sh"
}

const missingTool = "epub-handbook-definitely-missing-tool-7f3a9c"

func requireSh(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("sh-based case: POSIX shell is not available on windows")
	}
}

func TestLookPath(t *testing.T) {
	tests := []struct {
		name string
		tool string
		want bool
	}{
		{name: "present", tool: presentTool(), want: true},
		{name: "missing", tool: missingTool, want: false},
		{name: "empty", tool: "", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := LookPath(tc.tool)
			if err != nil {
				t.Fatalf("LookPath(%q) error = %v, want nil", tc.tool, err)
			}
			if got != tc.want {
				t.Fatalf("LookPath(%q) = %v, want %v", tc.tool, got, tc.want)
			}
		})
	}
}

func TestRequire(t *testing.T) {
	if err := Require(presentTool()); err != nil {
		t.Fatalf("Require(%q) = %v, want nil", presentTool(), err)
	}
	err := Require(missingTool)
	if !errors.Is(err, ErrToolMissing) {
		t.Fatalf("Require(missing) = %v, want ErrToolMissing", err)
	}
}

func TestRunEmptyArgv(t *testing.T) {
	_, err := Run(t.Context(), t.TempDir(), nil)
	if err == nil {
		t.Fatal("Run(empty argv) error = nil, want error")
	}
	if errors.Is(err, ErrToolMissing) {
		t.Fatalf("Run(empty argv) = %v, must not be ErrToolMissing", err)
	}
}

func TestRunMissingTool(t *testing.T) {
	res, err := Run(t.Context(), t.TempDir(), []string{missingTool, "--version"})
	if !errors.Is(err, ErrToolMissing) {
		t.Fatalf("Run(missing) error = %v, want ErrToolMissing", err)
	}
	if res.ExitCode != 0 || res.Stdout != nil || res.Stderr != nil {
		t.Fatalf("Run(missing) result = %+v, want zero CmdResult", res)
	}
}

func TestRunCapturesExitCodeAndStreams(t *testing.T) {
	requireSh(t)
	res, err := Run(t.Context(), t.TempDir(), []string{"sh", "-c", "echo out; echo err 1>&2; exit 3"})
	if err != nil {
		t.Fatalf("Run error = %v, want nil (non-zero exit is reported via ExitCode)", err)
	}
	if res.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", res.ExitCode)
	}
	if got := string(res.Stdout); got != "out\n" {
		t.Errorf("Stdout = %q, want %q", got, "out\n")
	}
	if got := string(res.Stderr); got != "err\n" {
		t.Errorf("Stderr = %q, want %q", got, "err\n")
	}
}

func TestRunSuccessExitCodeZero(t *testing.T) {
	requireSh(t)
	res, err := Run(t.Context(), t.TempDir(), []string{"sh", "-c", "printf ok"})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if res.ExitCode != 0 || string(res.Stdout) != "ok" || len(res.Stderr) != 0 {
		t.Fatalf("Run result = %+v, want ExitCode 0, Stdout \"ok\", empty Stderr", res)
	}
}

func TestCappedBufferConsumesAllAndRetainsPrefix(t *testing.T) {
	w := cappedBuffer{limit: 4}
	n, err := w.Write([]byte("abcdef"))
	if err != nil || n != 6 {
		t.Fatalf("Write = (%d, %v), want (6, nil)", n, err)
	}
	if got := string(w.Bytes()); got != "abcd" {
		t.Fatalf("stored prefix = %q, want %q", got, "abcd")
	}
	if !w.Exceeded() || w.received != 6 {
		t.Fatalf("Exceeded=%v received=%d, want true/6", w.Exceeded(), w.received)
	}
}

func TestRunHonorsDir(t *testing.T) {
	requireSh(t)
	dir := t.TempDir()
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(t.Context(), dir, []string{"sh", "-c", "pwd"})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	got, err := filepath.EvalSymlinks(strings.TrimSpace(string(res.Stdout)))
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", res.Stdout, err)
	}
	if got != want {
		t.Fatalf("pwd inside Run = %q, want %q", got, want)
	}
}

// TestRunAlreadyCancelledContext 锁定回归点：ctx 在调用前已取消时，Run 必须
// 立刻返回 context.Canceled，绝不能把子进程启动起来跑完（哪怕命令本身秒退）。
// ErrToolMissing 与「取消」是两种不同的失败原因，不能被混在一起判定。
func TestRunAlreadyCancelledContext(t *testing.T) {
	requireSh(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := Run(ctx, t.TempDir(), []string{"sh", "-c", "echo should-not-run"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run(cancelled ctx) error = %v, want wrapping context.Canceled", err)
	}
	if errors.Is(err, ErrToolMissing) {
		t.Fatalf("Run(cancelled ctx) error = %v, must not read as ErrToolMissing", err)
	}
	if len(res.Stdout) != 0 {
		t.Fatalf("Run(cancelled ctx) Stdout = %q, want empty: 子进程本不该被启动执行", res.Stdout)
	}
}

// TestRunContextCancelledMidFlight 锁定回归点：一个正在运行、原本会跑很久的
// 子进程，在 ctx 超时后必须被限时杀掉（而不是等它自己跑完），且返回的 error
// 要能用 errors.Is(err, context.DeadlineExceeded) 判定——否则调用链上层
// （fontcoverage → pipeline）没法把这次「取消」和「工具真的跑挂了」区分开，
// 只会误报成 capability.run-failed。
//
// 用 `sh -c "sleep N; ..."`（而不是直接 `sleep N`）刻意构造「孙进程持有
// stdout/stderr 管道写端」的场景：ctx 取消后 Cancel 只杀 sh 这一个直接子
// 进程，被兜住的 sleep 孙进程会被重新挂到 init 下继续跑，只要它不退出，
// 管道写端就不关闭——如果 Run 没设 WaitDelay，Wait 会一直卡到 sleep 自然
// 结束为止，取消形同虚设。这正是 epub.font.coverage.analyze 经 extern 起
// `uv run python -m src.cli …` 的真实进程形状（python 是 uv 的子进程）。
func TestRunContextCancelledMidFlight(t *testing.T) {
	requireSh(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// sleep 时长必须显著超过 waitDelay，这样"没设上限、傻等孙进程退出"的
	// 回归会让本测试的等待时间明显超出下面的断言（而不是恰好卡在边界上）。
	longSleep := waitDelay + 10*time.Second

	start := time.Now()
	res, err := Run(ctx, t.TempDir(), []string{"sh", "-c",
		fmt.Sprintf("sleep %d; echo full-sleep-completed", int(longSleep.Seconds()))})
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run(mid-flight cancel) error = %v, want wrapping context.DeadlineExceeded", err)
	}
	// elapsed 必须被 waitDelay 封顶（留出调度余量），而不是等那个被孤儿化的
	// sleep 孙进程自己跑完（longSleep，比 waitDelay 长 10s）。
	if elapsed >= waitDelay+5*time.Second {
		t.Fatalf("elapsed = %v, want bounded by waitDelay(%v)：子进程应在 WaitDelay 到期后被强制解除阻塞，不能傻等孙进程退出", elapsed, waitDelay)
	}
	if strings.Contains(string(res.Stdout), "full-sleep-completed") {
		t.Fatalf("Stdout = %q, 子进程不应该被放任跑完", res.Stdout)
	}
}
