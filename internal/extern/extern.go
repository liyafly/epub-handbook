// Package extern 是外部进程边界的唯一入口（SPEC §1 第 3 层，INV-4）。
//
// caps 不得 import os/exec；工具缺失必须显式降级（返回 ErrToolMissing），
// 由调用方决定跳过还是失败，不许静默忽略。
package extern

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// ErrToolMissing 表示外部工具在 PATH 中不存在。
var ErrToolMissing = errors.New("extern: tool missing")

// ErrOutputLimit 表示外部 provider 的 stdout 或 stderr 超过捕获上限。
var ErrOutputLimit = errors.New("extern: output limit exceeded")

// waitDelay 是 ctx 取消/超时后，Wait 等待子进程 I/O 管道关闭的上限
// （exec.Cmd.WaitDelay 语义）。
//
// 不设它（零值）意味着无限等：Cancel 只杀掉 argv[0] 这一个直接子进程；如果
// 它自己又 fork 了孙进程且孙进程继承了 stdout/stderr 管道写端（典型例子正是
// fontcoverage 调的 `uv run python -m src.cli …`——python 是 uv 的子进程），
// 孙进程只要不退出，管道写端就不关闭，Wait 会一直卡到孙进程自己跑完为止，
// 取消形同虚设——这正是「子进程无法被取消」的根因，而不只是没传 ctx。
// 5 秒足够让正常进程把最后一点缓冲输出写完，同时严格封顶了取消后的等待。
const waitDelay = 5 * time.Second

// maxRunDuration prevents a caller without its own deadline from leaving a
// provider running forever. streamOutputLimit is per stream; the prefix is
// retained for diagnostics while the writer keeps consuming bytes so the
// child process cannot deadlock on a full pipe.
const (
	maxRunDuration    = 30 * time.Minute
	streamOutputLimit = 16 << 20
)

type cappedBuffer struct {
	buf      bytes.Buffer
	limit    int
	received int64
	onLimit  context.CancelFunc
}

func (w *cappedBuffer) Write(p []byte) (int, error) {
	wasExceeded := w.Exceeded()
	if remaining := w.limit - w.buf.Len(); remaining > 0 {
		keep := len(p)
		if keep > remaining {
			keep = remaining
		}
		_, _ = w.buf.Write(p[:keep])
	}
	w.received += int64(len(p))
	if !wasExceeded && w.Exceeded() && w.onLimit != nil {
		w.onLimit()
	}
	return len(p), nil
}

func (w *cappedBuffer) Bytes() []byte { return w.buf.Bytes() }

func (w *cappedBuffer) Exceeded() bool { return w.received > int64(w.limit) }

// LookPath 报告外部工具是否可用。absent 时返回 (false, nil)。
//
// 不接收 ctx：LookPath 只在 PATH 目录里做文件系统查找，不起子进程，
// 没有可取消的长耗时操作。
func LookPath(name string) (bool, error) {
	path, err := exec.LookPath(name)
	if err != nil || path == "" {
		return false, nil
	}
	return true, nil
}

// Require 在工具缺失时返回 ErrToolMissing。
func Require(name string) error {
	ok, err := LookPath(name)
	if err != nil {
		return err
	}
	if !ok {
		return ErrToolMissing
	}
	return nil
}

// CmdResult 是一次外部进程运行的产出。
type CmdResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// Run 在 dir 工作目录下执行 argv[0]（带完整参数 argv[1:]），
// 捕获 stdout/stderr。工具不存在时返回 ErrToolMissing。
// 允许外部进程自行落盘（例如字体子集化工具）——这是 INV-3 中
// extern 作为磁盘边界的另一半职责。
//
// ctx 取消或超时时，子进程会被杀死（exec.CommandContext 默认的 Cancel
// 行为是 Process.Kill），不会再无限跑下去——这是全仓唯一的外部进程边界，
// 也是唯一能真正终止一个失控子进程（例如 epub.font.coverage.analyze 起的
// `uv run` Python 字体工具）的地方。
func Run(ctx context.Context, dir string, argv []string) (CmdResult, error) {
	if ctx == nil {
		// 防御性兜底，与 internal/zipfs 的 contextErr 同一约定：nil ctx 降级为
		// 「不取消」而不是 panic。exec.CommandContext 本身对 nil ctx 会 panic。
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, maxRunDuration)
		defer cancel()
	}
	if len(argv) == 0 {
		return CmdResult{}, errors.New("extern: empty argv")
	}
	if err := Require(argv[0]); err != nil {
		return CmdResult{}, err
	}
	processCtx, cancelProcess := context.WithCancel(ctx)
	defer cancelProcess()
	cmd := exec.CommandContext(processCtx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.WaitDelay = waitDelay
	out := cappedBuffer{limit: streamOutputLimit, onLimit: cancelProcess}
	errBuf := cappedBuffer{limit: streamOutputLimit, onLimit: cancelProcess}
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	res := CmdResult{Stdout: out.Bytes(), Stderr: errBuf.Bytes()}
	if exit, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exit.ExitCode()
	}

	// exec.CommandContext 取消/超时时只是把子进程 Kill 掉；Wait 把结果包成
	// 一个普通 *exec.ExitError（比如 "signal: killed"，ExitCode() == -1），
	// 并不会自动带上 context.Canceled / context.DeadlineExceeded。如果这里
	// 什么都不做，下面的 *exec.ExitError 分支会把它当成「进程正常跑完、只是
	// 退出码是 -1」直接吞掉——调用方（pipeline）就永远看不出这是一次取消，
	// 只会误判成 capability.run-failed。所以先检查 ctx 是否已经出错，并把
	// 它显式联结进返回的 error，让上层 errors.Is(err, context.Canceled) /
	// errors.Is(err, context.DeadlineExceeded) 能穿透到这里。
	if cerr := ctx.Err(); cerr != nil && !errors.Is(err, cerr) {
		if err != nil {
			return res, fmt.Errorf("extern: %w: %w", cerr, err)
		}
		return res, cerr
	}
	if out.Exceeded() || errBuf.Exceeded() {
		return res, fmt.Errorf("%w: stdout=%d bytes stderr=%d bytes limit=%d bytes per stream",
			ErrOutputLimit, out.received, errBuf.received, streamOutputLimit)
	}

	if _, ok := err.(*exec.ExitError); ok {
		return res, nil
	}
	if err != nil {
		return res, err
	}
	return res, nil
}
