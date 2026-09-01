package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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
	// ResolveChain 校验契约存在性与 requires 无环，并返回依赖在前的完整
	// 拓扑链。Run 必须执行整条链，不能把 chain 重置成最终 capability。
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
	needsWrite := chainNeedsWrite(chain)
	multiOutputCap := IsMultiOutput(contract.ID)
	if needsWrite && !opts.DryRun {
		if multiOutputCap {
			if opts.Args.Get("output_dir") == "" {
				return usage("output_dir is required for capability %s", contract.ID)
			}
		} else {
			if opts.OutputPath == "" {
				return usage("--output is required for capability %s", contract.ID)
			}
			if absIn, absOut, err := samePath(opts.InputPath, opts.OutputPath); err == nil && absIn == absOut {
				return usage("output must not overwrite the input EPUB")
			}
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
	// 先复制兼容性的 KEY=VALUE，再由正式全局 flag 最终覆盖保留键，避免
	// 用户参数伪造 pipeline 的输入、输出或事务模式。
	runArgs := make(Args, len(opts.Args)+4)
	for k, v := range opts.Args {
		runArgs[k] = v
	}
	runArgs["input"] = opts.InputPath
	runArgs["output"] = opts.OutputPath
	runArgs["dry_run"] = strconv.FormatBool(opts.DryRun)
	runArgs["legacy_report"] = strconv.FormatBool(opts.LegacyReport)
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

	redLines := chainRedLines(chain)
	if needsWrite && b != nil {
		preflight, preflightErr := redline.Check(
			redline.OriginalState(b), redline.OriginalState(b),
			[]string{redline.CheckDRM}, redline.Options{
				AllowFontObfuscation: runArgs.Bool("allow_font_obfuscation"),
			},
		)
		if preflightErr != nil {
			failed = true
			events = append(events, report.Event{Step: "drm-preflight", Status: "failed", Message: preflightErr.Error()})
			findings = append(findings, report.Finding{
				Level: "error", ID: "redline.drm-preflight-failed",
				Title: "DRM preflight failed", Detail: preflightErr.Error(), Location: "drm",
			})
		} else if len(preflight) > 0 {
			failed = true
			events = append(events, report.Event{Step: "drm-preflight", Status: "failed", Message: "DRM detected, refusing to process."})
			findings = appendRedlineFindings(findings, preflight)
		} else {
			events = append(events, report.Event{Step: "drm-preflight", Status: "completed"})
		}
	}

	if !failed {
		for _, c := range chain {
			step := c.ID
			runner, ok := registry[c.ID]
			if !ok {
				// 契约存在但无 Go 实现（B 类纯 AI/人工 skill 或待决策能力）：
				// 依赖失败必须阻断后续 capability，不能继续调用最终 stage。
				failed = true
				events = append(events, report.Event{Step: step, Status: "skipped",
					Message: "capability has no Go implementation"})
				findings = append(findings, report.Finding{
					Level: "error", ID: "capability.not-implemented",
					Title:  "Capability not implemented in Go",
					Detail: fmt.Sprintf("%s has no Go implementation; no check was executed. Follow the corresponding skill's manual/AI workflow, or list ready capabilities with `epub capabilities`", c.ID),
				})
				break
			}
			result, err := runner(ctx, b, runArgs, up)
			if err != nil {
				failed = true
				events = append(events, report.Event{Step: step, Status: "failed", Message: err.Error()})
				findings = append(findings, report.Finding{
					Level: "error", ID: "capability.run-failed",
					Title: "Capability failed", Detail: err.Error(), Location: step,
				})
				break
			}
			stageFailed := result.Status == report.StatusFailed || hasErrorFinding(result.Findings)
			stageStatus := "completed"
			if stageFailed {
				stageStatus = "failed"
			}
			events = append(events, report.Event{Step: step, Status: stageStatus})
			findings = append(findings, result.Findings...)
			events = append(events, result.Events...)
			for k, v := range result.Facts {
				facts[step+"."+k] = v
			}
			mergeRenames(renames, result.Renames)
			up[c.ID] = result
			if stageFailed {
				failed = true
				break
			}
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

	// 红线门禁：链上全部契约的 redLines 并集，对比书的原始态与当前态。
	// 普通 redline 必须在唯一的 b.WriteTo 之前通过；失败时不创建输出文件。
	// noBook 能力没有 Book 可比对（当前契约 redLines 均为空）；
	// 若未来声明红线，需要为无 Book 场景另行设计，不得静默跳过。
	if len(redLines) > 0 && !failed && b != nil && !multiOutputCap {
		redlineFindings, err := redline.Check(redline.OriginalState(b), redline.CurrentState(b), redLines, redline.Options{
			PathMap:              renames,
			AllowList:            []string{"*/nav.xhtml", "*/toc.ncx"},
			AllowFontObfuscation: runArgs.Bool("allow_font_obfuscation"),
		})
		if err != nil {
			failed = true
			env.Status = report.StatusFailed
			events = append(events, report.Event{Step: "redline", Status: "failed", Message: err.Error()})
			findings = append(findings, report.Finding{
				Level: "error", ID: "redline.check-failed",
				Title: "Redline validation failed", Detail: err.Error(), Location: "redline",
			})
		} else {
			findings = appendRedlineFindings(findings, redlineFindings)
			if len(redlineFindings) > 0 {
				failed = true
				env.Status = report.StatusFailed
				events = append(events, report.Event{Step: "redline", Status: "failed",
					Message: fmt.Sprintf("%d findings", len(redlineFindings))})
			} else {
				events = append(events, report.Event{Step: "redline", Status: "completed",
					Message: "0 findings"})
			}
		}
	} else if len(redLines) > 0 && !failed && multiOutputCap {
		// split 自己在组提交之前逐段执行红线与结构校验；pipeline 不应再拿
		// 未承载各段改动的原 Book 做虚假的单输出 post-check。
		events = append(events, report.Event{Step: "redline", Status: "completed",
			Message: "validated by multi-output capability before group commit"})
	}

	// 输出落盘：单输出链在全部 stage 与普通 redline 通过后只写一次。
	// dry-run 与能力失败不写；multi-output 的 split runner 自己管理段产物。
	if !failed && !opts.DryRun && needsWrite && !multiOutputCap {
		if b == nil {
			env.Status = report.StatusFailed
			failed = true
			findings = append(findings, report.Finding{
				Level: "error", ID: "output.no-book",
				Title: "Cannot write output without an EPUB input",
			})
			events = append(events, report.Event{Step: "write-output", Status: "failed", Message: "input book is nil"})
		} else if err := b.WriteToContext(ctx, opts.OutputPath); err != nil {
			env.Status = report.StatusFailed
			failed = true
			findings = append(findings, report.Finding{
				Level: "error", ID: "output.write-failed",
				Title: "Failed to write output EPUB", Detail: err.Error(),
			})
			events = append(events, report.Event{Step: "write-output", Status: "failed", Message: err.Error()})
		} else {
			outRef := &report.Artifact{Path: opts.OutputPath}
			if sum, err := fileSHA256(opts.OutputPath); err == nil {
				outRef.SHA256 = sum
			}
			env.Output = outRef
			events = append(events, report.Event{Step: "write-output", Status: "completed", Message: opts.OutputPath})
		}
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
		if IsMultiOutput(id) {
			return []string{"epub run " + id + " --input <reviewed-input> output_dir=<out-dir>"}
		}
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

func chainNeedsWrite(chain []Contract) bool {
	for _, c := range chain {
		if c.Permissions.RequiresWriteAccess && !IsReadOnly(c.ID) && !IsNoBook(c.ID) {
			return true
		}
	}
	return false
}

func chainRedLines(chain []Contract) []string {
	var out []string
	for _, c := range chain {
		out = append(out, c.RedLines...)
	}
	return dedupe(out)
}

func mergeRenames(dst, src map[string]string) {
	keys := make([]string, 0, len(src))
	for from := range src {
		keys = append(keys, from)
	}
	slices.Sort(keys)
	for _, from := range keys {
		redline.AddPathMapping(dst, from, src[from])
	}
	// A capability may report more than one rename in a single map. Resolve
	// those links before the next stage so redline sees the final path even
	// when the source map's insertion order is unavailable.
	for from := range dst {
		seen := map[string]bool{from: true}
		for {
			to := dst[from]
			next, ok := dst[to]
			if !ok || seen[to] {
				break
			}
			dst[from] = next
			seen[to] = true
		}
	}
}

func appendRedlineFindings(dst []report.Finding, src []redline.Finding) []report.Finding {
	for _, f := range src {
		dst = append(dst, report.Finding{
			Level: "error", ID: "redline." + f.Check,
			Title: f.Message, Location: f.Check,
		})
	}
	return dst
}

func hasErrorFinding(findings []report.Finding) bool {
	for _, f := range findings {
		if f.Level == "error" {
			return true
		}
	}
	return false
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
