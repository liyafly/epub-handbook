package pipeline

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/report"
)

// TestRunUsageErrorsCarryEnvelopeAndExit3 锁定用法错误的两件事：
//
//  1. 退出码是 3（SPEC §8.5），不是 1 —— 「参数写错了」与「书有问题」
//     必须能被调用方区分开；
//  2. 信封照常给出（SPEC §8.2「所有命令返回同一形状」）。否则 `--json`
//     的 agent 在退出码 3 上拿到空 stdout，必须另写一条非 JSON 解析分支，
//     而错误原因只能从 stderr 的自由文本里猜。
//
// KEY=VALUE 参数校验尤其容易退化：sourceintake 的 max_files 走了 UsageError
// 通道，而 split_points / expect_volumes 曾用裸 fmt.Errorf，于是同类输入错误
// 一个退 3 一个退 1。这里把三条路径一起钉住。
func TestRunUsageErrorsCarryEnvelopeAndExit3(t *testing.T) {
	// 不设 RepoRoot：Run 会自行 FindRepoRoot，用的是仓库真实契约目录
	// （这些用例故意跑真实 capability id，而不是合成契约）。
	sample := buildSampleEpub(t)
	outDir := t.TempDir()

	cases := []struct {
		name string
		opts Options
		want string // 期望出现在错误文本里的片段
	}{
		{
			name: "未知 capability",
			opts: Options{CapabilityID: "epub.nope.nope", InputPath: sample},
			want: "epub.nope.nope",
		},
		{
			name: "缺少 --input",
			opts: Options{CapabilityID: "epub.package.nav.audit"},
			want: "--input",
		},
		{
			name: "--input 不存在",
			opts: Options{CapabilityID: "epub.package.nav.audit",
				InputPath: filepath.Join(outDir, "missing.epub")},
			want: "input not found",
		},
		{
			name: "max_files 非正整数",
			opts: Options{CapabilityID: "epub.source.intake",
				InputPath: outDir, Args: Args{"max_files": "0"}},
			want: "max_files",
		},
		{
			name: "split_points 非整数",
			opts: Options{CapabilityID: "epub.package.split",
				InputPath: sample, Args: Args{"output_dir": outDir, "split_points": "abc"}},
			want: "split_points",
		},
		{
			name: "expect_volumes 非整数",
			opts: Options{CapabilityID: "epub.alite.convert",
				InputPath: sample, OutputPath: filepath.Join(outDir, "alite.epub"),
				Args: Args{"expect_volumes": "many"}},
			want: "expect_volumes",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome, err := Run(context.Background(), tc.opts)
			if err == nil {
				t.Fatalf("期望用法错误，实际 status=%s", outcome.Envelope.Status)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("错误文本 = %q, 期望包含 %q", err.Error(), tc.want)
			}
			if outcome.ExitCode != ExitUsage {
				t.Errorf("退出码 = %d, want %d (ExitUsage)", outcome.ExitCode, ExitUsage)
			}
			env := outcome.Envelope
			if env.SchemaVersion != "2" || env.Capability != tc.opts.CapabilityID {
				t.Errorf("信封头部 = %+v, want schemaVersion=2 capability=%s",
					env, tc.opts.CapabilityID)
			}
			if env.Status != report.StatusFailed {
				t.Errorf("status = %q, want %q", env.Status, report.StatusFailed)
			}
			if len(env.Findings) != 1 || env.Findings[0].ID != "usage.invalid-argument" ||
				env.Findings[0].Level != "error" {
				t.Fatalf("findings = %+v, want 单条 error usage.invalid-argument", env.Findings)
			}
			if !strings.Contains(env.Findings[0].Detail, tc.want) {
				t.Errorf("finding detail 未带上原因: %q", env.Findings[0].Detail)
			}
		})
	}
}

// TestUsageErrorUnwrapsToCause 保证 UsageError 是可判的包装而不是文本吞并：
// errors.As 拿得到它，errors.Is 拿得到被包的原因。
func TestUsageErrorUnwrapsToCause(t *testing.T) {
	cause := errors.New("bad flag")
	wrapped := error(&UsageError{Err: cause})

	var ue *UsageError
	if !errors.As(wrapped, &ue) {
		t.Fatalf("errors.As 未识别 *UsageError")
	}
	if !errors.Is(wrapped, cause) {
		t.Errorf("errors.Is 未穿透到原因")
	}
	if wrapped.Error() != "bad flag" {
		t.Errorf("Error() = %q, want %q", wrapped.Error(), "bad flag")
	}
}
