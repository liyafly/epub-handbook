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
	"slices"
	"strconv"
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
	case "clean":
		return runClean(argv[1:])
	case "capabilities":
		return runCapabilities(argv[1:])
	case "version":
		return runVersion(argv[1:])
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
  epub clean <in.epub | 目录> --out DIR [--steps normalize,migrate,css] [--approve] [--json]
            （默认只审计；目录按路径顺序串行处理）
  epub capabilities [--id ID] [--json] 列出能力、参数、执行形态及实现状态
  epub version [--json]          显示版本、commit、构建时间与目标平台
  epub redline [--check TEXT,...|all]
            [--path-map ENVELOPE.JSON] [--allow-font-obfuscation] [--verbose] [--json]
            BEFORE AFTER
                                      两文件红线比对（对齐 validate_text_invariance）
  epub help

退出码: 0 成功; 1 失败; 2 需要人工批准; 3 用法错误。
`)
}

// runClean 只解析批量命令参数并回传 pipeline 的结果。
func runClean(argv []string) int {
	jsonRequested := wantsJSON(argv)
	input := ""
	flagArgs := argv
	if len(argv) > 0 && !strings.HasPrefix(argv[0], "-") {
		input = argv[0]
		flagArgs = argv[1:]
	}
	if err := rejectDuplicateCleanFlags(flagArgs); err != nil {
		return cleanUsageError(jsonRequested, err)
	}
	fs := flag.NewFlagSet("epub clean", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	outputDir := fs.String("out", "", "输出目录")
	stepsValue := fs.String("steps", "", "步骤：normalize,migrate,css")
	approve := fs.Bool("approve", false, "仅在全部步骤、复检和红线通过后写出候选 EPUB")
	jsonOutput := fs.Bool("json", false, "将批次信封 JSON 写到 stdout")
	if err := fs.Parse(flagArgs); err != nil {
		if jsonRequested {
			return cleanUsageError(true, err)
		}
		return pipeline.ExitUsage
	}
	jsonRequested = jsonRequested || *jsonOutput
	if input == "" {
		if fs.NArg() != 1 {
			return cleanUsageError(jsonRequested, errors.New("provide one input EPUB or directory"))
		}
		input = fs.Arg(0)
	} else if fs.NArg() != 0 {
		return cleanUsageError(jsonRequested, errors.New("unexpected positional arguments"))
	}
	steps := []string(nil)
	stepsSpecified := false
	fs.Visit(func(item *flag.Flag) {
		stepsSpecified = stepsSpecified || item.Name == "steps"
	})
	if stepsSpecified {
		steps = []string{}
		for step := range strings.SplitSeq(*stepsValue, ",") {
			steps = append(steps, strings.TrimSpace(step))
		}
	}
	ctx, stop := runCtx()
	defer stop()
	result, err := pipeline.Clean(ctx, pipeline.CleanOptions{
		InputPath: input, OutputDir: *outputDir, Steps: steps, Approve: *approve,
	})
	if err != nil {
		if jsonRequested {
			if _, ok := errors.AsType[*pipeline.UsageError](err); ok {
				return cleanUsageError(true, err)
			}
			fmt.Fprintln(os.Stderr, "epub clean:", err)
			data, marshalErr := marshalEnvelope(pipeline.CleanFailureEnvelope(err))
			if marshalErr == nil {
				_, _ = os.Stdout.Write(data)
			} else {
				fmt.Fprintln(os.Stderr, "epub clean:", marshalErr)
			}
			return pipeline.ExitFailed
		}
		fmt.Fprintln(os.Stderr, "epub clean:", err)
		if _, ok := errors.AsType[*pipeline.UsageError](err); ok {
			return pipeline.ExitUsage
		}
		return pipeline.ExitFailed
	}
	if jsonRequested {
		data, marshalErr := marshalEnvelope(result.Envelope)
		if marshalErr != nil {
			fmt.Fprintln(os.Stderr, "epub clean:", marshalErr)
			return pipeline.ExitFailed
		}
		_, _ = os.Stdout.Write(data)
		return result.ExitCode
	}
	for _, book := range result.Books {
		fmt.Printf("%s: %s (report %s)", book.InputPath, book.Envelope.Status, book.ReportPath)
		if book.OutputPath != "" {
			fmt.Printf("; output %s", book.OutputPath)
		}
		fmt.Println()
		if book.Err != nil {
			fmt.Fprintln(os.Stderr, "epub clean:", book.Err)
		}
	}
	fmt.Println(formatCleanBatchSummary(result))
	return result.ExitCode
}

func formatCleanBatchSummary(result pipeline.CleanBatchResult) string {
	counts := map[string]int{"planned": 0, "complete": 0, "failed": 0, "cancelled": 0}
	outputs := 0
	for _, book := range result.Books {
		if _, ok := counts[book.Envelope.Status]; ok {
			counts[book.Envelope.Status]++
		}
		if book.OutputPath != "" {
			outputs++
		}
	}
	summary := fmt.Sprintf("batch: planned=%d complete=%d failed=%d cancelled=%d outputs=%d/%d",
		counts["planned"], counts["complete"], counts["failed"], counts["cancelled"], outputs, len(result.Books))
	if counts["planned"] > 0 {
		summary += "; planned books have no written EPUB"
	}
	if len(result.NotStarted) > 0 {
		summary += fmt.Sprintf("; notStarted=%d", len(result.NotStarted))
	}
	return summary
}

func rejectDuplicateCleanFlags(argv []string) error {
	names := []string{"out", "steps", "approve", "json"}
	counts := map[string]int{
		"out": 0, "steps": 0, "approve": 0, "json": 0,
	}
	for index := 0; index < len(argv); index++ {
		arg := argv[index]
		for _, name := range names {
			if arg == "-"+name || arg == "--"+name {
				counts[name]++
				if name != "approve" && name != "json" && index+1 < len(argv) {
					index++
				}
				break
			}
			if strings.HasPrefix(arg, "-"+name+"=") || strings.HasPrefix(arg, "--"+name+"=") {
				counts[name]++
				break
			}
		}
	}
	for _, name := range names {
		if counts[name] > 1 {
			return fmt.Errorf("duplicate flag: --%s", name)
		}
	}
	return nil
}

func cleanUsageError(jsonOutput bool, err error) int {
	fmt.Fprintln(os.Stderr, "epub clean:", err)
	if jsonOutput {
		outcome := pipeline.UsageOutcome("epub.clean", err)
		if data, marshalErr := marshalEnvelope(outcome.Envelope); marshalErr == nil {
			_, _ = os.Stdout.Write(data)
		} else {
			fmt.Fprintln(os.Stderr, "epub clean:", marshalErr)
		}
	}
	return pipeline.ExitUsage
}

// runCapability 处理 `epub run <id>`。
func runCapability(argv []string) int {
	jsonRequested := wantsJSON(argv)
	if len(argv) == 0 || strings.HasPrefix(argv[0], "-") {
		return runUsageError("", jsonRequested, errors.New("缺少 capability-id"))
	}
	id := argv[0]
	if err := rejectDuplicatePathFlags(argv[1:]); err != nil {
		return runUsageError(id, jsonRequested, err)
	}
	fs := flag.NewFlagSet("epub run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	input := fs.String("input", "", "输入 EPUB")
	output := fs.String("output", "", "输出 EPUB")
	dryRun := fs.Bool("dry-run", false, "生成内存候选并检查，不写输出")
	jsonOut := fs.Bool("json", false, "以统一信封 JSON 输出")
	if err := fs.Parse(argv[1:]); err != nil {
		return runUsageError(id, jsonRequested, err)
	}
	wantJSON := jsonRequested || *jsonOut
	args := pipeline.Args{}
	for _, kv := range fs.Args() {
		if strings.HasPrefix(kv, "-") {
			return runUsageError(id, wantJSON, fmt.Errorf("flag %q must appear before KEY=VALUE arguments", kv))
		}
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return runUsageError(id, wantJSON, fmt.Errorf("参数必须是 KEY=VALUE 形式: %q", kv))
		}
		if _, exists := args[k]; exists {
			return runUsageError(id, wantJSON, fmt.Errorf("重复参数: %s", k))
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
		printCapabilitySummary(outcome)
	}
	return outcome.ExitCode
}

func printCapabilitySummary(outcome pipeline.Outcome) {
	env := outcome.Envelope
	fmt.Printf("capability: %s\nstatus:     %s\n", env.Capability, env.Status)
	if env.Output != nil {
		fmt.Printf("output:     %s\n", env.Output.Path)
	}
	if env.Capability == "epub.style.demo.maintain" && factString(env.Facts, "epub.style.demo.maintain.mode") == "catalog" {
		printStyleCatalog(env.Facts)
	}
	if env.Capability == "epub.typography.optimize" {
		printTypographySummary(env.Facts)
	}
	for _, f := range env.Findings {
		fmt.Printf("[%s] %s\n", f.Level, f.Title)
	}
	for _, c := range env.NextCommands {
		fmt.Printf("next: %s\n", c)
	}
}

func printStyleCatalog(facts map[string]any) {
	prefix := "epub.style.demo.maintain."
	collection := factString(facts, prefix+"collection")
	if collection == "presets" {
		var presets []struct {
			ID          string   `json:"id"`
			Description string   `json:"description"`
			Layers      []string `json:"layers"`
			Source      string   `json:"source"`
			SourcePath  string   `json:"sourcePath"`
			Reader      string   `json:"readerStatus"`
		}
		if decodeFact(facts[prefix+"presets"], &presets) {
			fmt.Printf("presets: %d\n", len(presets))
			for _, preset := range presets {
				fmt.Printf("- %s — %s\n  layers: %s\n  source: %s (%s); reader: %s\n", preset.ID, preset.Description, strings.Join(preset.Layers, ", "), preset.Source, preset.SourcePath, preset.Reader)
			}
			if len(presets) == 0 {
				fmt.Println("0 presets matched; try a broader query.")
			}
		}
		return
	}
	var scenes []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Path  string `json:"path"`
	}
	if !decodeFact(facts[prefix+"scenes"], &scenes) {
		return
	}
	fmt.Printf("scenes: %d (%s; %s)\n", len(scenes), factString(facts, prefix+"resourceSource"), factString(facts, prefix+"sourcePathBase"))
	for _, scene := range scenes {
		fmt.Printf("- %s — %s\n  %s\n", scene.ID, scene.Title, scene.Path)
	}
	if len(scenes) == 0 {
		fmt.Println("0 scenes matched; try a shorter query such as chapter, poetry, or note.")
	}
}

func printTypographySummary(facts map[string]any) {
	prefix := "epub.typography.optimize."
	if preset := factString(facts, prefix+"preset"); preset != "" {
		fmt.Printf("preset:     %s\n", preset)
	}
	mode := factString(facts, prefix+"applicationMode")
	if mode == "" {
		mode = "whole-book"
	}
	scopeFileCount := factInt(facts, prefix+"scopeFileCount")
	if scopeFileCount == 0 {
		scopeFileCount = factInt(facts, prefix+"xhtmlLinks")
	}
	fileWord := "files"
	if scopeFileCount == 1 {
		fileWord = "file"
	}
	fmt.Printf("scope:      %s (%d XHTML %s)\n", mode, scopeFileCount, fileWord)
	var actions []struct {
		Action string `json:"action"`
	}
	if decodeFact(facts[prefix+"stylesheetActions"], &actions) {
		counts := map[string]int{}
		for _, action := range actions {
			counts[action.Action]++
		}
		fmt.Printf("stylesheets: add=%d replace=%d keep=%d\n", counts["add"], counts["replace"], counts["keep"])
	}
	var coverage struct {
		Ratio        float64  `json:"ratio"`
		Uncovered    []string `json:"uncoveredClasses"`
		Used         []string `json:"usedClasses"`
		Insufficient bool     `json:"insufficientToDetermineApplicability"`
		Warning      string   `json:"warning"`
	}
	if decodeFact(facts[prefix+"coverage"], &coverage) {
		basis := factString(facts, prefix+"coverageBasis")
		if basis == "" {
			basis = "class-token"
		}
		if coverage.Insufficient || len(coverage.Used) == 0 {
			fmt.Printf("%s coverage: insufficient class tokens to determine applicability\n", basis)
		} else {
			fmt.Printf("%s coverage: %.0f%%", basis, coverage.Ratio*100)
			if len(coverage.Uncovered) > 0 {
				shown := coverage.Uncovered
				if len(shown) > 20 {
					shown = shown[:20]
				}
				fmt.Printf("; uncovered: %s", strings.Join(shown, ", "))
				if len(coverage.Uncovered) > len(shown) {
					fmt.Printf(" (+%d more)", len(coverage.Uncovered)-len(shown))
				}
			}
			fmt.Println()
		}
		if coverage.Warning != "" {
			fmt.Printf("warning: %s\n", coverage.Warning)
		}
		fmt.Printf("coverage basis: %s; this counts class attributes and does not measure CSS cascade or visual compatibility.\n", basis)
	}
	if factString(facts, prefix+"fontModeAction") == "preserve" {
		fmt.Printf("font mode:  %s (preserved)\n", factString(facts, prefix+"fontMode"))
	}
}

func decodeFact(value any, target any) bool {
	if value == nil {
		return false
	}
	data, err := json.Marshal(value)
	return err == nil && json.Unmarshal(data, target) == nil
}

func factString(facts map[string]any, key string) string {
	value, _ := facts[key].(string)
	return value
}

func factInt(facts map[string]any, key string) int {
	switch value := facts[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

// rejectDuplicatePathFlags catches ambiguous input/output choices before the
// standard flag parser silently keeps the last value.
func rejectDuplicatePathFlags(argv []string) error {
	counts := map[string]int{"input": 0, "output": 0}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		for _, name := range []string{"input", "output"} {
			if arg == "-"+name || arg == "--"+name {
				counts[name]++
				// The next token is the value for this flag, even if it begins
				// with a dash; flag.Parse will report a missing value if absent.
				if i+1 < len(argv) {
					i++
				}
				break
			}
			if strings.HasPrefix(arg, "-"+name+"=") || strings.HasPrefix(arg, "--"+name+"=") {
				counts[name]++
				break
			}
		}
	}
	for _, name := range []string{"input", "output"} {
		if counts[name] > 1 {
			return fmt.Errorf("重复全局 flag: --%s", name)
		}
	}
	return nil
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
		default:
			for _, prefix := range []string{"-json=", "--json="} {
				if value, ok := strings.CutPrefix(arg, prefix); ok {
					if parsed, err := strconv.ParseBool(value); err == nil {
						want = parsed
					}
					break
				}
			}
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
		if errors.Is(err, pipeline.ErrUnknownCapability) {
			return pipeline.ExitUsage
		}
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
	if *id != "" {
		for _, c := range contracts {
			printCapabilityDetails(c)
		}
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

func printCapabilityDetails(capability pipeline.CapabilityInfo) {
	status := "pending"
	if capability.Implemented {
		status = "ready"
	}
	fmt.Printf("capability: %s\nkind:       %s\nimplemented:%s\ndescription: %s\n",
		capability.ID, capability.Kind, " "+status, capability.Description)
	fmt.Printf("input:      %s\noutput:     %s\nwrite:      %t\nnetwork:    %s\n",
		capability.Execution.Input, capability.Execution.Output,
		capability.Permissions.RequiresWriteAccess, capability.Permissions.Network)
	if len(capability.Parameters) == 0 {
		fmt.Println("parameters: none")
		return
	}
	keys := make([]string, 0, len(capability.Parameters))
	for key := range capability.Parameters {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	fmt.Println("parameters:")
	for _, key := range keys {
		parameter := capability.Parameters[key]
		fmt.Printf("  %s (%s)", key, parameter.Type)
		if parameter.Required {
			fmt.Print(" required")
		}
		if parameter.Default != nil {
			fmt.Printf(" default=%q", *parameter.Default)
		}
		if len(parameter.Enum) > 0 {
			fmt.Printf(" enum=%s", strings.Join(parameter.Enum, "|"))
		}
		if parameter.Minimum != nil {
			fmt.Printf(" min=%d", *parameter.Minimum)
		}
		fmt.Printf("\n    %s\n", parameter.Description)
	}
}

// runRedline 处理 `epub redline`（legacy 两文件比对）。
func runRedline(argv []string) int {
	wantJSON := wantsJSON(argv)
	fs := flag.NewFlagSet("epub redline", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	check := fs.String("check", "all", "text,metadata,spine,cover,drm,anchors,all 或逗号列表")
	jsonOut := fs.Bool("json", false, "以统一信封 JSON 输出")
	allowFont := fs.Bool("allow-font-obfuscation", false, "允许标准 EPUB 字体混淆")
	verbose := fs.Bool("verbose", false, "输出 verbose 行")
	var pathMapFiles []string
	fs.Func("path-map", "structure normalize 的 --json 信封（或含 mappings 的报告 JSON），提供 entry 改名映射", func(v string) error {
		pathMapFiles = append(pathMapFiles, v)
		return nil
	})
	if err := fs.Parse(argv); err != nil {
		if wantJSON {
			return runRedlineUsageError(true, err)
		}
		return 3
	}
	wantJSON = wantJSON || *jsonOut
	if fs.NArg() != 2 {
		return runRedlineUsageError(wantJSON, errors.New("需要 BEFORE 与 AFTER 两个 EPUB 路径"))
	}
	if wantJSON {
		env, code, err := pipeline.RedlineCompareEnvelopeWith(fs.Arg(0), fs.Arg(1), *check, pathMapFiles, *allowFont, *verbose)
		if err != nil {
			fmt.Fprintln(os.Stderr, "epub:", err)
		}
		data, marshalErr := marshalEnvelope(env)
		if marshalErr != nil {
			fmt.Fprintln(os.Stderr, "epub:", marshalErr)
			return pipeline.ExitFailed
		}
		_, _ = os.Stdout.Write(data)
		return code
	}
	code, err := pipeline.RedlineCompareWith(fs.Arg(0), fs.Arg(1), *check, pathMapFiles, *allowFont, *verbose)
	if err != nil {
		fmt.Fprintln(os.Stderr, "epub:", err)
	}
	return code
}

func runRedlineUsageError(jsonOut bool, err error) int {
	fmt.Fprintln(os.Stderr, "epub redline:", err)
	if jsonOut {
		outcome := pipeline.UsageOutcome("epub.redline", err)
		if data, marshalErr := marshalEnvelope(outcome.Envelope); marshalErr == nil {
			_, _ = os.Stdout.Write(data)
		} else {
			fmt.Fprintln(os.Stderr, "epub:", marshalErr)
		}
	}
	return pipeline.ExitUsage
}

func marshalEnvelope(env any) ([]byte, error) {
	return pipeline.MarshalEnvelope(env)
}
