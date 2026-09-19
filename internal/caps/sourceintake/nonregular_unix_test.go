//go:build unix

package sourceintake

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/liyafly/epub-handbook/internal/report"
)

// TestRunRejectsNonRegularFileInput 回归 HIGH-3：单文件入口没有 IsRegular 检查时，
// FIFO 会让 sha256 的 io.Copy 在 open/read 上永久阻塞（--input /dev/zero 更是
// 只能靠 kill 结束）。目录内的同类条目本来就被跳过，单文件必须同样被拒。
//
// 用 FIFO 而不是 /dev/zero 做 fixture：即使断言失败也只是阻塞在 open 上，
// 不会烧 CPU 写满测试机。
func TestRunRejectsNonRegularFileInput(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("mkfifo unsupported: %v", err)
	}

	type outcome struct {
		res report.Result
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := Run(t.Context(), nil, Params{SourcePath: fifo})
		done <- outcome{res, err}
	}()

	select {
	case got := <-done:
		if !errors.Is(got.err, ErrNotRegularFile) {
			t.Fatalf("err = %v, want ErrNotRegularFile (pipeline 把它映射为退出码 3)", got.err)
		}
	case <-time.After(10 * time.Second):
		// goroutine 卡在 open/read 上，无法回收；让测试立刻失败并退出进程。
		t.Fatal("Run blocked on a FIFO input: the IsRegular guard regressed")
	}
}
