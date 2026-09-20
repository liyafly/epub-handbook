// cmd/epub 是唯一公开命令（SPEC §1 第 0 层）：只做 flag 解析与退出码，
// 零业务逻辑、零 EPUB 知识。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/liyafly/epub-handbook/internal/pipeline"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// runCtx 建一个会响应 SIGINT/SIGTERM（Ctrl-C、`kill`）的 ctx，贯穿到
// pipeline.Run。这是取消链路唯一的入口：cmd 层只负责建 ctx 和把
// ExitCode 传回去（§3：cmd 保持薄），取消与失败的区分、退出码语义都在
// pipeline 内部完成（见 internal/pipeline/run.go 里 cancelled 相关注释）。
func runCtx() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func run(argv []string) int {
	if len(argv) == 0 {
		usage(os.Stderr)
		return 3
	}
	switch argv[0] {
	case "run":
		return runCapability(argv[1:])
	case "capabilities":
		return runCapabilities(argv[1:])
	case "redline":
		return runRedline(argv[1:])
	case "help", "-h", "--help":
		usage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "epub: unknown command %q\n\n", argv[0])
		usage(os.Stderr)
		return 3
	}
}

func usage(w *os.File) {
	fmt.Fprint(w, `epub — EPUB 手册 CLI（SPEC-go-architecture §8）

用法:
  epub run <capability-id> [--input PATH] [--output PATH] [--dry-run] [--json]
            [KEY=VALUE...]
  epub capabilities [--id ID] [--json] 列出能力、参数、执行形态及实现状态
  epub redline [--check TEXT,...|all] [--allow-list GLOB]...
            [--path-map ENVELOPE.JSON] [--allow-font-obfuscation] [--verbose]
            BEFORE AFTER
                                      两文件红线比对（对齐 validate_text_invariance）
  epub help

退出码: 0 成功; 1 失败; 2 需要人工批准; 3 用法错误。
`)
}

// runCapability 处理 `epub run <id>`。
func runCapability(argv []string) int {
	jsonRequested := wantsJSON(argv)
	if len(argv) == 0 || strings.HasPrefix(argv[0], "-") {
		return runUsageError("", jsonRequested, errors.New("缺少 capability-id"))
	}
	id := argv[0]
	fs := flag.NewFlagSet("epub run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	input := fs.String("input", "", "输入 EPUB")
	output := fs.String("output", "", "输出 EPUB")
	dryRun := fs.Bool("dry-run", false, "生成内存候选并检查，不写输出")
	jsonOut := fs.Bool("json", false, "以统一信封 JSON 输出")
	if err := fs.Parse(argv[1:]); err != nil {
		return runUsageError(id, jsonRequested, err)
	}
	args := pipeline.Args{}
	for _, kv := range fs.Args() {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return runUsageError(id, *jsonOut, fmt.Errorf("参数必须是 KEY=VALUE 形式: %q", kv))
		}
		if _, exists := args[k]; exists {
			return runUsageError(id, *jsonOut, fmt.Errorf("重复参数: %s", k))
		}
		args[k] = v
	}
	ctx, stop := runCtx()
	defer stop()
	outcome, err := pipeline.Run(ctx, pipeline.Options{
		CapabilityID: id,
		InputPath:    *input,
		OutputPath:   *output,
		DryRun:       *dryRun,
		Args:         args,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "epub:", err)
		// 用法错误也给信封（SPEC §8.2 单一形状）：--json 的调用方不必为
		// 退出码 3 另写一条「stdout 不是 JSON」的分支。
		if *jsonOut {
			if data, mErr := marshalEnvelope(outcome.Envelope); mErr == nil {
				os.Stdout.Write(data)
			}
		}
		return outcome.ExitCode
	}
	env := outcome.Envelope
	if *jsonOut {
		data, err := marshalEnvelope(env)
		if err != nil {
			fmt.Fprintln(os.Stderr, "epub:", err)
			return 1
		}
		os.Stdout.Write(data)
	} else {
		fmt.Printf("capability: %s\nstatus:     %s\n", env.Capability, env.Status)
		if env.Output != nil {
			fmt.Printf("output:     %s\n", env.Output.Path)
		}
		for _, f := range env.Findings {
			fmt.Printf("[%s] %s\n", f.Level, f.Title)
		}
		for _, c := range env.NextCommands {
			fmt.Printf("next: %s\n", c)
		}
	}
	return outcome.ExitCode
}

// wantsJSON recognizes the boolean flag before flag.Parse runs, including
// parse-error paths where FlagSet may stop before reaching --json.
func wantsJSON(argv []string) bool {
	want := false
	for _, arg := range argv {
		switch arg {
		case "-json", "--json", "-json=true", "--json=true":
			want = true
		case "-json=false", "--json=false":
			want = false
		}
	}
	return want
}

func runUsageError(capabilityID string, jsonOut bool, err error) int {
	fmt.Fprintln(os.Stderr, "epub run:", err)
	if jsonOut {
		outcome := pipeline.UsageOutcome(capabilityID, err)
		if data, marshalErr := marshalEnvelope(outcome.Envelope); marshalErr == nil {
			_, _ = os.Stdout.Write(data)
		} else {
			fmt.Fprintln(os.Stderr, "epub:", marshalErr)
		}
	}
	return pipeline.ExitUsage
}

// runCapabilities 处理 `epub capabilities`。
func runCapabilities(argv []string) int {
	fs := flag.NewFlagSet("epub capabilities", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "输出 JSON")
	id := fs.String("id", "", "只显示指定能力及其参数")
	if err := fs.Parse(argv); err != nil {
		return 3
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "epub capabilities: unexpected positional arguments")
		return 3
	}
	root, err := pipeline.FindRepoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "epub:", err)
		return 3
	}
	contracts, err := pipeline.DescribeCapabilities(root, *id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "epub:", err)
		return 1
	}
	if *jsonOut {
		data, err := json.MarshalIndent(contracts, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "epub:", err)
			return 1
		}
		fmt.Println(string(data))
		return 0
	}
	fmt.Printf("%-42s %-10s %s\n", "CAPABILITY", "KIND", "GO")
	for _, c := range contracts {
		status := "pending"
		if c.Implemented {
			status = "ready"
		}
		fmt.Printf("%-42s %-10s %s\n", c.ID, c.Kind, status)
	}
	return 0
}

// runRedline 处理 `epub redline`（legacy 两文件比对）。
func runRedline(argv []string) int {
	fs := flag.NewFlagSet("epub redline", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	check := fs.String("check", "all", "text,metadata,spine,cover,drm,anchors,all 或逗号列表")
	allowFont := fs.Bool("allow-font-obfuscation", false, "允许标准 EPUB 字体混淆")
	verbose := fs.Bool("verbose", false, "输出 verbose 行")
	var allowList []string
	fs.Func("allow-list", "XHTML 路径 fnmatch 豁免", func(v string) error {
		allowList = append(allowList, v)
		return nil
	})
	var pathMapFiles []string
	fs.Func("path-map", "structure normalize 的 --json 信封（或含 mappings 的报告 JSON），提供 entry 改名映射", func(v string) error {
		pathMapFiles = append(pathMapFiles, v)
		return nil
	})
	if err := fs.Parse(argv); err != nil {
		return 3
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "epub redline: 需要 BEFORE 与 AFTER 两个 EPUB 路径")
		return 3
	}
	code, err := pipeline.RedlineCompareWith(fs.Arg(0), fs.Arg(1), *check, allowList, pathMapFiles, *allowFont, *verbose)
	if err != nil {
		fmt.Fprintln(os.Stderr, "epub:", err)
	}
	return code
}

func marshalEnvelope(env any) ([]byte, error) {
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
