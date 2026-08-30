// cmd/epub 是唯一公开命令（SPEC §1 第 0 层）：只做 flag 解析与退出码，
// 零业务逻辑、零 EPUB 知识。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/liyafly/epub-handbook/internal/pipeline"
)

func main() {
	os.Exit(run(os.Args[1:]))
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
            [--legacy-report] [KEY=VALUE...]
  epub capabilities [--json]          列出全部能力及其实现状态
  epub redline BEFORE AFTER [--check TEXT,...|all] [--allow-list GLOB]...
            [--path-map REPORT.JSON] [--allow-font-obfuscation] [--verbose]
                                      两文件红线比对（对齐 validate_text_invariance）
  epub help

退出码: 0 成功; 1 失败; 2 需要人工批准; 3 用法错误。
`)
}

// runCapability 处理 `epub run <id>`。
func runCapability(argv []string) int {
	if len(argv) == 0 || strings.HasPrefix(argv[0], "-") {
		fmt.Fprintln(os.Stderr, "epub run: 缺少 capability-id")
		return 3
	}
	id := argv[0]
	fs := flag.NewFlagSet("epub run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	input := fs.String("input", "", "输入 EPUB")
	output := fs.String("output", "", "输出 EPUB")
	dryRun := fs.Bool("dry-run", false, "只扫描并报告，不写输出")
	jsonOut := fs.Bool("json", false, "以统一信封 JSON 输出")
	legacy := fs.Bool("legacy-report", false, "迁移期脚手架：按 Python oracle 形状输出报告")
	if err := fs.Parse(argv[1:]); err != nil {
		return 3
	}
	args := pipeline.Args{}
	for _, kv := range fs.Args() {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			fmt.Fprintf(os.Stderr, "epub run: 参数必须是 KEY=VALUE 形式: %q\n", kv)
			return 3
		}
		args[k] = v
	}
	outcome, err := pipeline.Run(context.Background(), pipeline.Options{
		CapabilityID: id,
		InputPath:    *input,
		OutputPath:   *output,
		DryRun:       *dryRun,
		LegacyReport: *legacy,
		Args:         args,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "epub:", err)
		return outcome.ExitCode
	}
	env := outcome.Envelope
	if *jsonOut || *legacy {
		data, err := marshalEnvelope(env, *legacy)
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

// runCapabilities 处理 `epub capabilities`。
func runCapabilities(argv []string) int {
	fs := flag.NewFlagSet("epub capabilities", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "输出 JSON")
	if err := fs.Parse(argv); err != nil {
		return 3
	}
	root, err := pipeline.FindRepoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "epub:", err)
		return 3
	}
	contracts, err := pipeline.AllContracts(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "epub:", err)
		return 1
	}
	if *jsonOut {
		type capInfo struct {
			ID          string   `json:"id"`
			Kind        string   `json:"kind"`
			Implemented bool     `json:"implemented"`
			RedLines    []string `json:"redLines,omitempty"`
			Requires    []string `json:"requires,omitempty"`
		}
		out := make([]capInfo, 0, len(contracts))
		for _, c := range contracts {
			out = append(out, capInfo{
				ID:          c.ID,
				Kind:        c.Kind,
				Implemented: pipeline.Implemented(c.ID),
				RedLines:    c.RedLines,
				Requires:    c.Requires,
			})
		}
		data, err := json.MarshalIndent(out, "", "  ")
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
		if pipeline.Implemented(c.ID) {
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
	fs.Func("path-map", "structure normalize 报告 JSON（entry 改名映射）", func(v string) error {
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

func marshalEnvelope(env any, _ bool) ([]byte, error) {
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
