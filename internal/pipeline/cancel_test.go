package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
)

// TestRunCancelledContextBeforeStart 钉住取消/失败的分类回归：一个在调用前就
// 已经取消（或 deadline 已过期）的 ctx 传进 Run，必须被识别成"没跑完"而不是
// "工具坏了"。
func TestRunCancelledContextBeforeStart(t *testing.T) {
	cases := []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
	}{
		{"pre-cancelled", func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, cancel
		}},
		{"deadline already exceeded", func() (context.Context, context.CancelFunc) {
			return context.WithDeadline(context.Background(), time.Now().Add(-time.Minute))
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestContract(t, root, "test.cancel.pre", nil, true, nil)

			called := false
			installTestRunner(t, "test.cancel.pre", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
				called = true
				return report.Result{Capability: "test.cancel.pre", Status: report.StatusComplete}, nil
			})

			out := filepath.Join(t.TempDir(), "out.epub")
			ctx, cancel := tc.ctx()
			defer cancel()
			outcome, err := Run(ctx, Options{
				RepoRoot: root, CapabilityID: "test.cancel.pre",
				InputPath: buildSampleEpub(t), OutputPath: out,
			})
			if err != nil {
				t.Fatalf("Run 不应返回 Go error（取消已经被完整装进信封），got %v", err)
			}

			// 回归点 1：status 必须是 cancelled，不是 failed——否则 agent 没法
			// 把"没跑完"和"书/工具有问题"分开处理。
			if outcome.Envelope.Status != report.StatusCancelled {
				t.Fatalf("status = %q, want %q", outcome.Envelope.Status, report.StatusCancelled)
			}
			// 回归点 2：SPEC §8.5 没有专门的取消退出码档位，取消映射到失败档 1。
			if outcome.ExitCode != ExitFailed {
				t.Fatalf("exit code = %d, want %d(ExitFailed)", outcome.ExitCode, ExitFailed)
			}
			// 回归点 3：必须有一条说明取消的 error finding，且 id 不是
			// capability.run-failed——那个 id 专属"runner 返回了非取消错误"。
			if !hasFindingID(outcome.Envelope.Findings, "run.cancelled") {
				t.Fatalf("findings 缺少 run.cancelled: %#v", outcome.Envelope.Findings)
			}
			for _, f := range outcome.Envelope.Findings {
				if f.ID == "run.cancelled" && f.Level != "error" {
					t.Errorf("run.cancelled level = %q, want error", f.Level)
				}
				if f.ID == "capability.run-failed" {
					t.Errorf("取消被误报成 capability.run-failed: %#v", f)
				}
			}
			// 回归点 4：INV-3 的唯一一次落盘必须被取消阻断——输出文件不能存在，
			// 不能像红线失败那样"仍然写出来留给人工 diff review"。
			if _, statErr := os.Stat(out); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("输出文件不应存在，stat err = %v", statErr)
			}
			// 回归点 5：ctx 在进入 stage 循环前就已经取消，pipeline 应该在调用
			// runner 之前就识别出来，不依赖 runner 自己检查 ctx。
			if called {
				t.Fatal("ctx 已取消时不应该还去调用 runner")
			}
		})
	}
}

// TestRunCancelledContextViaRunnerError 钉住第二条取消检测路径：runner 自己
// 检查 ctx 并返回包了 context.Canceled / context.DeadlineExceeded 的
// error（split、sourceintake 与经 internal/extern 起子进程的 fontcoverage
// 都属于这种情况）。即使 pipeline 调用 runner 时 ctx 尚未过期（也就是本文件
// 上一个测试的"stage 边界检查"不会触发），pipeline 仍必须把这个 runner error
// 正确分类成 cancelled，而不是落进通用的 capability.run-failed 分支。
func TestRunCancelledContextViaRunnerError(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.cancel.runner-err", nil, true, nil)
	installTestRunner(t, "test.cancel.runner-err", func(ctx context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		// 模拟一个会检查 ctx 的 capability（例如 fontcoverage 经
		// internal/extern 起的 `uv run` 子进程被 ctx 杀死后返回的错误）：
		// ctx 本身此刻还没有过期，纯粹是 runner 自己判断该终止了。
		return report.Result{}, fmt.Errorf("coverage detector killed: %w", context.DeadlineExceeded)
	})

	out := filepath.Join(t.TempDir(), "out.epub")
	outcome, err := Run(t.Context(), Options{
		RepoRoot: root, CapabilityID: "test.cancel.runner-err",
		InputPath: buildSampleEpub(t), OutputPath: out,
	})
	if err != nil {
		t.Fatalf("Run 不应返回 Go error, got %v", err)
	}

	if outcome.Envelope.Status != report.StatusCancelled {
		t.Fatalf("status = %q, want %q", outcome.Envelope.Status, report.StatusCancelled)
	}
	if outcome.ExitCode != ExitFailed {
		t.Fatalf("exit code = %d, want %d(ExitFailed)", outcome.ExitCode, ExitFailed)
	}
	if !hasFindingID(outcome.Envelope.Findings, "run.cancelled") {
		t.Fatalf("findings 缺少 run.cancelled: %#v", outcome.Envelope.Findings)
	}
	// 核心回归点：runner 返回的 context.DeadlineExceeded 不能被通用错误分支
	// 吞成 capability.run-failed——那会让"取消"和"工具真的跑挂了"在信封里
	// 变得无法区分，agent 会把一次纯粹的超时误诊成书或工具坏了。
	if hasFindingID(outcome.Envelope.Findings, "capability.run-failed") {
		t.Fatalf("取消被误报成 capability.run-failed: %#v", outcome.Envelope.Findings)
	}
	if _, statErr := os.Stat(out); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("输出文件不应存在，stat err = %v", statErr)
	}
}

// TestRunCancellationObservedAfterRunner locks the second half of a stage
// boundary. Most migrated runners do not inspect ctx yet, so pipeline must
// observe a cancellation that happens while such a runner is executing.
func TestRunCancellationObservedAfterRunner(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, "test.cancel.after-runner", nil, false, nil)
	ctx, cancel := context.WithCancel(context.Background())
	installTestRunner(t, "test.cancel.after-runner", func(_ context.Context, _ *book.Book, _ Args, _ Upstream) (report.Result, error) {
		cancel()
		return report.Result{Capability: "test.cancel.after-runner", Status: report.StatusComplete}, nil
	})

	outcome, err := Run(ctx, Options{
		RepoRoot: root, CapabilityID: "test.cancel.after-runner",
		InputPath: buildSampleEpub(t),
	})
	if err != nil {
		t.Fatalf("Run returned Go error: %v", err)
	}
	if outcome.Envelope.Status != report.StatusCancelled {
		t.Fatalf("status = %q, want %q", outcome.Envelope.Status, report.StatusCancelled)
	}
	if outcome.ExitCode != ExitFailed || !hasFindingID(outcome.Envelope.Findings, "run.cancelled") {
		t.Fatalf("outcome = %#v, want cancelled/exit 1/run.cancelled", outcome)
	}
}
