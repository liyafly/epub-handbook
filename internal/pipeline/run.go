package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/redline"
	"github.com/liyafly/epub-handbook/internal/report"
)

// Options 是一次 run 的全部输入。
type Options struct {
	RepoRoot     string
	CapabilityID string
	InputPath    string
	OutputPath   string
	DryRun       bool
	// LegacyReport 输出 --legacy-report 脚手架（SPEC §5.2，唯一被批准的
	// 临时脚手架）：Result.Facts["legacyReport"] 以 Python 形状序列化。
	LegacyReport bool
	Args         Args
}

// Outcome 是 pipeline 的完整产出：信封 + 退出码。
type Outcome struct {
	Envelope report.Envelope
	ExitCode int
}

// 退出码语义（SPEC §8.5）。
const (
	ExitOK       = 0
	ExitFailed   = 1
	ExitApproval = 2
	ExitUsage    = 3
)

// Run 执行一个 capability 及其依赖链。
func Run(ctx context.Context, opts Options) (Outcome, error) {
	env := report.Envelope{
		SchemaVersion: "2",
		Capability:    opts.CapabilityID,
	}
	usage := func(format string, args ...any) (Outcome, error) {
		return Outcome{ExitCode: ExitUsage}, fmt.Errorf(format, args...)
	}

	root := opts.RepoRoot
	if root == "" {
		found, err := FindRepoRoot()
		if err != nil {
			return usage("%v", err)
		}
		root = found
	}
	// ResolveChain 校验契约存在性与 requires 无环；`epub run` 本身
	// 只执行请求的能力（与 Python oracle 的单能力路由一致），链编排
	// 属于完整清洗流水线的范畴。
	chain, err := ResolveChain(root, opts.CapabilityID)
	if err != nil {
		return Outcome{ExitCode: ExitUsage}, err
	}
	if len(chain) == 0 {
		return usage("unknown capability: %s", opts.CapabilityID)
	}
	contract := chain[len(chain)-1]
	if contract.ID != opts.CapabilityID {
		return usage("unknown capability: %s", opts.CapabilityID)
	}
	chain = []Contract{contract}
	noBookCap := IsNoBook(contract.ID)
	inputIsDir := false
	if opts.InputPath != "" {
		st, statErr := os.Stat(opts.InputPath)
		if statErr != nil {
			return usage("input not found: %s", opts.InputPath)
		}
		inputIsDir = noBookCap && st.IsDir()
	} else if !noBookCap {
		return usage("--input is required")
	}
	needsWrite := contract.Permissions.RequiresWriteAccess && !IsReadOnly(contract.ID) && !noBookCap
	if needsWrite && !opts.DryRun {
		if opts.OutputPath == "" {
			return usage("--output is required for capability %s", contract.ID)
		}
		if absIn, absOut, err := samePath(opts.InputPath, opts.OutputPath); err == nil && absIn == absOut {
			return usage("output must not overwrite the input EPUB")
		}
	}

	if opts.InputPath != "" && !inputIsDir {
		inputRef := &report.Artifact{Path: opts.InputPath}
		if sum, err := fileSHA256(opts.InputPath); err == nil {
			inputRef.SHA256 = sum
		}
		env.Input = inputRef
	}

	var b *book.Book
	if opts.InputPath != "" && !inputIsDir {
		var err error
		b, err = book.Open(opts.InputPath)
		if err != nil {
			env.Status = report.StatusFailed
			env.Findings = append(env.Findings, report.Finding{
				Level: "error", ID: "input.invalid-epub",
				Title: "Input is not a valid EPUB", Detail: err.Error(),
				Location: opts.InputPath,
			})
			return Outcome{Envelope: env, ExitCode: ExitFailed}, nil
		}
		defer b.Close()
	}

	// 全局 flag 经 Args 透传给 capability：--output / --input / --dry-run /
	// --legacy-report（迁移期脚手架，cap 侧读取 legacy_report 键）。
	runArgs := Args{"input": opts.InputPath, "output": opts.OutputPath}
	if opts.DryRun {
		runArgs["dry_run"] = "true"
	}
	if opts.LegacyReport {
		runArgs["legacy_report"] = "true"
	}
	for k, v := range opts.Args {
		runArgs[k] = v
	}
	if noBookCap {
		// noBook 能力的输入语义：--input 指向目录 → 该目录即 demo 源树；
		// 为空或指向文件 → 缺省为仓库内 demo 源树（用户 KEY=VALUE 可覆盖）。
		if inputIsDir {
			if abs, err := filepath.Abs(opts.InputPath); err == nil {
				runArgs["demo_dir"] = abs
			} else {
				runArgs["demo_dir"] = opts.InputPath
			}
		} else if runArgs["demo_dir"] == "" {
			runArgs["demo_dir"] = filepath.Join(root, "templates", "epub-style-demo")
		}
	}
	opts.Args = runArgs

	up := Upstream{}
	renames := map[string]string{}
	var events []report.Event
	var findings []report.Finding
	var facts = map[string]any{}
	failed := false

	for _, c := range chain {
		step := c.ID
		runner, ok := registry[c.ID]
		if !ok {
			// 契约存在但无 Go 实现（B 类纯 AI/人工 skill 或待决策能力）：
			// 必须 failed + error finding。返回 complete/exit 0 会让只看
			// status 与退出码的调用方把未执行当成功。
			failed = true
			events = append(events, report.Event{Step: step, Status: "skipped",
				Message: "capability has no Go implementation"})
			findings = append(findings, report.Finding{
				Level: "error", ID: "capability.not-implemented",
				Title:  "Capability not implemented in Go",
				Detail: fmt.Sprintf("%s has no Go implementation; no check was executed. Follow the corresponding skill's manual/AI workflow, or list ready capabilities with `epub capabilities`", c.ID),
			})
			continue
		}
		result, err := runner(ctx, b, opts.Args, up)
		if err != nil {
			failed = true
			events = append(events, report.Event{Step: step, Status: "failed", Message: err.Error()})
			findings = append(findings, report.Finding{
				Level: "error", ID: "capability.run-failed",
				Title: "Capability failed", Detail: err.Error(), Location: step,
			})
			break
		}
		events = append(events, report.Event{Step: step, Status: "completed"})
		findings = append(findings, result.Findings...)
		events = append(events, result.Events...)
		for k, v := range result.Facts {
			facts[step+"."+k] = v
		}
		for from, to := range result.Renames {
			redline.AddPathMapping(renames, from, to)
		}
		up[c.ID] = result
		if result.Status == report.StatusFailed {
			failed = true
			break
		}
	}

	switch {
	case failed:
		env.Status = report.StatusFailed
	case opts.DryRun && needsWrite:
		env.Status = report.StatusApprovalRequired
	default:
		env.Status = report.StatusComplete
	}

	// 输出落盘：唯一一次（INV-3），在红线校验之前 —— 与 Python 流程一致：
	// 先产出 after 文件，红线结论供人工 diff review，输出不销毁。
	// dry-run 与能力失败不写。
	if !failed && !opts.DryRun && needsWrite && !IsMultiOutput(contract.ID) {
		if opts.OutputPath == "" {
			return usage("--output is required for capability %s", contract.ID)
		}
		if err := b.WriteTo(opts.OutputPath); err != nil {
			env.Status = report.StatusFailed
			failed = true
			findings = append(findings, report.Finding{
				Level: "error", ID: "output.write-failed",
				Title: "Failed to write output EPUB", Detail: err.Error(),
			})
		} else {
			outRef := &report.Artifact{Path: opts.OutputPath}
			if sum, err := fileSHA256(opts.OutputPath); err == nil {
				outRef.SHA256 = sum
			}
			env.Output = outRef
			events = append(events, report.Event{Step: "write-output", Status: "completed", Message: opts.OutputPath})
		}
	}

	// 红线门禁：链上全部契约的 redLines 并集，对比书的原始态与当前态。
	// 在写盘之后执行（Python 先写 after 再跑 gate）：error 级发现把状态
	// 降为 failed、退出码 1，但输出保留供人工 diff review。
	var redLines []string
	for _, c := range chain {
		redLines = append(redLines, c.RedLines...)
	}
	redLines = dedupe(redLines)
	// noBook 能力没有 Book 可比对（当前契约 redLines 均为空）；
	// 若未来声明红线，需要为无 Book 场景另行设计，不得静默跳过。
	if len(redLines) > 0 && !failed && b != nil {
		redlineFindings, err := redline.Check(redline.OriginalState(b), redline.CurrentState(b), redLines, redline.Options{
			PathMap:   renames,
			AllowList: []string{"*/nav.xhtml", "*/toc.ncx"},
		})
		if err != nil {
			return Outcome{Envelope: env, ExitCode: ExitFailed}, fmt.Errorf("redline: %w", err)
		}
		for _, f := range redlineFindings {
			findings = append(findings, report.Finding{
				Level:    "error",
				ID:       "redline." + f.Check,
				Title:    f.Message,
				Location: f.Check,
			})
		}
		if len(redlineFindings) > 0 {
			failed = true
			env.Status = report.StatusFailed
		}
		events = append(events, report.Event{Step: "redline", Status: "completed",
			Message: fmt.Sprintf("%d findings", len(redlineFindings))})
	}

	if opts.DryRun {
		facts["dry_run"] = true
		if b != nil { // noBook 源树模式没有 Book，无修改 entry 可报
			facts["modified_entries"] = b.ModifiedNames()
		}
	}

	env.Events = events
	env.Findings = findings
	env.Facts = facts
	env.NextCommands = nextCommands(chain[len(chain)-1].ID, opts)

	exit := ExitOK
	switch env.Status {
	case report.StatusFailed:
		exit = ExitFailed
	case report.StatusApprovalRequired:
		exit = ExitApproval
	}
	return Outcome{Envelope: env, ExitCode: exit}, nil
}

// nextCommands 按 SPEC §8.2 给 agent 提示下一步（`epub run` 形态）。
func nextCommands(id string, opts Options) []string {
	var out []string
	if opts.DryRun {
		out = append(out, "epub run "+id+" --input <reviewed-input> --output <out.epub>")
		return out
	}
	switch id {
	case "epub.package.nav.audit":
		out = append(out,
			"epub run epub.layout.audit --input "+placeholder(opts.OutputPath, opts.InputPath))
	case "epub.structure.normalize":
		out = append(out, "epub redline <before> <after> --check all --path-map <normalize-report.json>")
	}
	return out
}

func placeholder(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return "<artifact>"
}

// RedlineCompare 是 legacy 两文件比对的人类入口（pre-commit hook / parity 用）。
// 输出与退出码语义逐字对齐 scripts/validate_text_invariance.py；
// 报告写到 stderr，由调用方自行重定向到文件。
func RedlineCompare(before, after, check string, allowList []string, allowFontObfuscation, verbose bool) (int, error) {
	return RedlineCompareWith(before, after, check, allowList, nil, allowFontObfuscation, verbose)
}

// RedlineCompareWith 在 RedlineCompare 基础上接受 structure normalize 报告
// 文件路径（对齐 validate_text_invariance.py 的 --path-map），由本层载入并
// 链式展开改名映射（cmd 层保持零 EPUB/redline 知识）。
func RedlineCompareWith(before, after, check string, allowList []string, pathMapFiles []string, allowFontObfuscation, verbose bool) (int, error) {
	pathMap := map[string]string{}
	for _, p := range pathMapFiles {
		raw, err := os.ReadFile(p)
		if err != nil {
			return ExitUsage, fmt.Errorf("读取 --path-map 失败: %w", err)
		}
		m, err := redline.LoadPathMap(raw)
		if err != nil {
			return ExitUsage, err
		}
		for from, to := range m {
			redline.AddPathMapping(pathMap, from, to)
		}
	}
	rep, err := redline.CompareFiles(before, after, check, redline.Options{
		AllowList:            allowList,
		PathMap:              pathMap,
		AllowFontObfuscation: allowFontObfuscation,
		Verbose:              verbose,
	})
	if err != nil {
		return ExitFailed, err
	}
	text := strings.Join(rep.Lines, "\n")
	if text != "" {
		text += "\n"
	}
	if text != "" {
		fmt.Fprint(os.Stderr, text)
	}
	return rep.Code, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func samePath(a, b string) (string, string, error) {
	absA, err := filepath.Abs(a)
	if err != nil {
		return "", "", err
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		return "", "", err
	}
	return absA, absB, nil
}
