package pipeline

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	typographycap "github.com/liyafly/epub-handbook/internal/caps/typography"
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

// UsageError 表示参数非法（SPEC §8.5：退出码 3 用法错误），而不是能力本身
// 失败。Runner 的参数校验用它包装错误；Run 见到就直接返回 ExitUsage，
// 不再记成 error finding + 退出码 1。
type UsageError struct{ Err error }

func (e *UsageError) Error() string { return e.Err.Error() }

func (e *UsageError) Unwrap() error { return e.Err }

// usageErrorf 构造 UsageError（register.go 的参数校验用）。
func usageErrorf(format string, a ...any) error {
	return &UsageError{Err: fmt.Errorf(format, a...)}
}

// usageEnvelope 是用法错误的最小信封：合 contracts/schemas/v2/envelope.schema.json
// 的必填三项（schemaVersion / capability / status），并把错误原文放进一条
// error finding，让只读 JSON 的 agent 也能拿到原因而不必解析 stderr。
func usageEnvelope(capabilityID string, err error) report.Envelope {
	return report.Envelope{
		SchemaVersion: "2",
		Capability:    capabilityID,
		Status:        report.StatusFailed,
		Findings: []report.Finding{{
			Level: "error", ID: "usage.invalid-argument",
			Title:  "Invalid command usage",
			Detail: err.Error(),
		}},
	}
}

// UsageOutcome exposes the canonical exit-3 envelope to the thin CLI layer for
// parse failures that happen before Run can be called.
func UsageOutcome(capabilityID string, err error) Outcome {
	return Outcome{Envelope: usageEnvelope(capabilityID, err), ExitCode: ExitUsage}
}

// Run 执行一个 capability 及其依赖链。
func Run(ctx context.Context, opts Options) (Outcome, error) {
	env := report.Envelope{
		SchemaVersion: "2",
		Capability:    opts.CapabilityID,
	}
	// usage 组装用法错误（SPEC §8.5 退出码 3）。信封照常给出：SPEC §8.2 说
	// 「所有命令返回同一形状」，`--json` 的调用方不该在退出码 3 上突然拿到
	// 一段非 JSON 的 stderr 而必须另写一条解析分支。状态用 failed（枚举里
	// 没有 usage 档），退出码仍是 3，二者一起才说清「是参数错，不是书错」。
	usage := func(format string, args ...any) (Outcome, error) {
		err := fmt.Errorf(format, args...)
		return Outcome{Envelope: usageEnvelope(opts.CapabilityID, err), ExitCode: ExitUsage}, err
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
		return Outcome{Envelope: usageEnvelope(opts.CapabilityID, err), ExitCode: ExitUsage}, err
	}
	if len(chain) == 0 {
		return usage("unknown capability: %s", opts.CapabilityID)
	}
	contract := chain[len(chain)-1]
	if contract.ID != opts.CapabilityID {
		return usage("unknown capability: %s", opts.CapabilityID)
	}
	if err := validateArguments(root, chain, opts.Args); err != nil {
		usageErr := &UsageError{Err: err}
		return UsageOutcome(opts.CapabilityID, usageErr), usageErr
	}
	noBookCap := contract.Execution.Input == ExecInputEpubOrTree
	// sourceInput 能力（planner，如 epub.source.intake）：--input 必填，可以是
	// 目录或任意文件；永不 book.Open，b 恒为 nil。
	sourceInputCap := contract.Execution.Input == ExecInputSourcePath
	inputIsDir := false
	if opts.InputPath != "" {
		st, statErr := os.Stat(opts.InputPath)
		if statErr != nil {
			return usage("input not found: %s", opts.InputPath)
		}
		if !st.IsDir() && !st.Mode().IsRegular() {
			// FIFO / 字符设备 / socket：下面的信封 sha256 与 capability 的流式
			// 读取都会永久阻塞（--input /dev/zero 只能靠 kill 结束）。
			return usage("input must be a regular file or a directory: %s", opts.InputPath)
		}
		inputIsDir = (noBookCap || sourceInputCap) && st.IsDir()
	} else if !noBookCap {
		return usage("--input is required")
	}
	needsWrite := chainNeedsWrite(chain)
	chainReady := chainImplemented(chain)
	multiOutputCap := contract.Execution.Output == ExecOutputMulti
	// Pending manual/AI capabilities must report capability.not-implemented
	// consistently. Their eventual output shape must not force callers to invent
	// an output path before the registry can explain that no Go runner exists.
	if chainReady && needsWrite && !opts.DryRun {
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

	if opts.InputPath != "" && (!inputIsDir || sourceInputCap) {
		// 信封 input：文件带 sha256；目录（sourceInput）只记 path
		// （envelope.schema.json 的 input.sha256 可选）。
		inputRef := &report.Artifact{Path: opts.InputPath}
		env.Input = inputRef
	}

	cancelInput := func(err error) (Outcome, error) {
		env.Status = report.StatusCancelled
		env.Findings = append(env.Findings, report.Finding{Level: "error", ID: "run.cancelled", Title: "Run cancelled before completion", Detail: err.Error()})
		return Outcome{Envelope: env, ExitCode: ExitFailed}, nil
	}
	if err := ctx.Err(); err != nil {
		return cancelInput(err)
	}
	var b *book.Book
	if opts.InputPath != "" && !inputIsDir && !sourceInputCap {
		var err error
		b, err = book.OpenContext(ctx, opts.InputPath)
		if err != nil {
			if env.Input != nil {
				if sum, hashErr := book.FileSHA256Context(ctx, opts.InputPath); hashErr == nil {
					env.Input.SHA256 = sum
				}
			}
			if ctx.Err() != nil {
				return cancelInput(ctx.Err())
			}
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
	if env.Input != nil && !inputIsDir {
		if b != nil {
			if sum, err := b.InputSHA256Context(ctx); err == nil {
				env.Input.SHA256 = sum
			}
		} else if sum, err := book.FileSHA256Context(ctx, opts.InputPath); err == nil {
			env.Input.SHA256 = sum
		}
	}

	// 全局 flag 经 Args 透传给 capability：--output / --input / --dry-run。
	// 先复制兼容性的 KEY=VALUE，再由正式全局 flag 最终覆盖保留键，避免
	// 用户参数伪造 pipeline 的输入、输出或事务模式。
	userArgs := maps.Clone(opts.Args)
	runArgs := make(Args, len(opts.Args)+3)
	for k, v := range opts.Args {
		runArgs[k] = v
	}
	runArgs["input"] = opts.InputPath
	runArgs["output"] = opts.OutputPath
	runArgs["dry_run"] = strconv.FormatBool(opts.DryRun)
	if sourceInputCap {
		// sourceInput 能力：解析后的绝对输入路径（目录或文件）。
		if abs, err := filepath.Abs(opts.InputPath); err == nil {
			runArgs["source_path"] = abs
		} else {
			runArgs["source_path"] = opts.InputPath
		}
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
	if runArgs["preset_dir"] == "" && slices.ContainsFunc(chain, func(c Contract) bool {
		return c.ID == typographycap.CapabilityID
	}) {
		runArgs["preset_dir"] = filepath.Join(root, typographycap.DefaultPresetsDir)
	}
	opts.Args = runArgs

	up := Upstream{}
	renames := map[string]string{}
	var events []report.Event
	var findings []report.Finding
	var facts = map[string]any{}
	failed := false
	// cancelled 与 failed 是两回事：failed 说的是"工具坏了 / 书有问题"，
	// cancelled 说的是"没跑完"（ctx 超时或 Ctrl-C）。cancelled 恒会同时把
	// failed 置真——这样才能复用下面既有的"failed 时不落盘、不做红线校验"
	// 分支——但 env.Status / finding 必须能让人（和 agent）分清两者，不能都
	// 归成 capability.run-failed。
	cancelled := false

	redLines := chainRedLines(chain)
	if chainReady && needsWrite && b != nil {
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

	// 链语义：chain 的最后一个元素是目标 capability，其余是 requires 上游。
	// 上游只是诊断（当前全部为只读审计），其 Status / error findings 不得阻断
	// 目标 stage，否则任何不完美的真书都无法 normalize / migrate。上游诊断
	// 结果落入 facts["<id>.status"] / facts["<id>.findings"]，信封 findings 只
	// 追加一条 info 摘要（SPEC §8.5：信封中的 error finding 会强制退出码 1）。
	// 仍然阻断的情形：runner 返回 Go error（工具坏了 ≠ 书有问题）、链上任一
	// 能力没有 Go 实现。
	if !failed {
		last := len(chain) - 1
		for i, c := range chain {
			step := c.ID
			// pipeline 是唯一贯穿整条链的调度者：E 项盘点显示 16 个 capability
			// 里 14 个的 Run(ctx, ...) 完全不看 ctx（split 与 sourceintake 除
			// 外）。如果只指望 runner 自己检查 ctx，一个已取消/已超时的 ctx
			// 传进来时，大多数 capability 会照常跑完、pipeline 也就永远不会
			// 观测到"取消"这件事。所以在每个 stage 边界都显式查一次
			// ctx.Err()，不依赖被调用方是否配合。
			if cErr := ctx.Err(); cErr != nil {
				cancelled = true
				failed = true
				events = append(events, report.Event{Step: step, Status: "failed",
					Message: "run cancelled before stage started: " + cErr.Error()})
				break
			}
			runner, ok := registry[c.ID]
			if !ok {
				// 契约存在但无 Go 实现（B 类纯 AI/人工 skill 或待决策能力）：
				// 无论处于链上哪个位置都必须阻断，不能继续调用最终 stage。
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
			var result report.Result
			var err error
			if b != nil {
				err = b.ReadError()
			}
			if err == nil {
				result, err = runner(ctx, b, runArgs, up)
			}
			if err == nil && b != nil {
				// Older capabilities may ignore individual read errors. A failed
				// input read must never become a partial successful report/output.
				err = b.ReadError()
			}
			if err != nil {
				var usageErr *UsageError
				if errors.As(err, &usageErr) {
					// KEY=VALUE 参数非法：用法错误（退出码 3），不是
					// capability.run-failed（退出码 1）。
					return Outcome{Envelope: usageEnvelope(opts.CapabilityID, err), ExitCode: ExitUsage}, err
				}
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					// 少数确实检查 ctx 的 runner（split、sourceintake，以及
					// fontcoverage 经 internal/extern 起的 `uv run` 子进程被
					// ctx 杀死时）会把这两个哨兵之一包进返回的 error。这是
					// "没跑完"，不是"工具坏了"——不能落进 capability.run-failed。
					cancelled = true
					failed = true
					events = append(events, report.Event{Step: step, Status: "failed",
						Message: "run cancelled: " + err.Error()})
					break
				}
				failed = true
				events = append(events, report.Event{Step: step, Status: "failed", Message: err.Error()})
				findings = append(findings, report.Finding{
					Level: "error", ID: "capability.run-failed",
					Title: "Capability failed", Detail: err.Error(), Location: step,
				})
				break
			}
			// A runner may not consume ctx yet. Re-check at the other side of the
			// stage boundary so cancellation during the final/read-only stage cannot
			// be reported as complete (or as approval-required for a dry-run).
			if cErr := ctx.Err(); cErr != nil {
				cancelled = true
				failed = true
				events = append(events, report.Event{Step: step, Status: "failed",
					Message: "run cancelled before stage completed: " + cErr.Error()})
				break
			}
			if i < last {
				events, findings = appendUpstreamDiagnostics(events, findings, facts, step, result)
				events = append(events, result.Events...)
				for k, v := range result.Facts {
					facts[step+"."+k] = v
				}
				mergeRenames(renames, result.Renames)
				up[c.ID] = result
				continue
			}
			// 目标 stage：findings 进信封，Status failed / error finding → 信封
			// failed、退出码 1，且不写输出。
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
			}
		}
	}

	switch {
	case cancelled:
		// 必须排在 case failed 之前：cancelled 恒会同时置 failed=true
		// （复用它的"不落盘、不做红线校验"分支），但信封状态要说清是取消，
		// 不是书或工具本身出了问题。
		env.Status = report.StatusCancelled
		findings = append(findings, report.Finding{
			Level: "error", ID: "run.cancelled",
			Title:  "Run cancelled before completion",
			Detail: "the run's context was cancelled or its deadline was exceeded before the capability chain finished; no output was written",
		})
	case failed:
		env.Status = report.StatusFailed
	case opts.DryRun && needsWrite:
		env.Status = report.StatusApprovalRequired
	default:
		env.Status = report.StatusComplete
	}

	// 红线门禁：链上全部契约的 redLines 并集，对比书的原始态与当前态。
	// 红线在内存态上校验，error finding / 校验器错误把状态降为 failed、退出码
	// 1，但**不阻止**下面唯一的一次落盘：metadata.edit / merge / cover.replace
	// / migrate.epub3 的契约红线（metadata / spine / cover）本就会被自身的预期
	// 变更触发，输出必须保留供人工 diff review（handoff §0 决策 2、
	// cleanup-flow.md「写出型能力自带内置红线 gate」）。
	// 落盘只被 DRM preflight、runner 错误、未实现能力、目标 stage 失败阻断。
	// noBook 能力没有 Book 可比对（当前契约 redLines 均为空）；
	// 若未来声明红线，需要为无 Book 场景另行设计，不得静默跳过。
	redlineFailed := false
	if len(redLines) > 0 && !failed && b != nil && !multiOutputCap {
		redlineFindings, err := redline.Check(redline.OriginalState(b), redline.CurrentState(b), redLines, redline.Options{
			PathMap:              renames,
			AllowList:            []string{"*/nav.xhtml", "*/toc.ncx"},
			AllowFontObfuscation: runArgs.Bool("allow_font_obfuscation"),
		})
		if readErr := b.ReadError(); readErr != nil {
			// Expected content differences retain a candidate for review; an
			// unreadable input cannot safely produce that candidate.
			failed = true
			if err == nil {
				err = readErr
			}
		}
		if err != nil {
			redlineFailed = true
			env.Status = report.StatusFailed
			events = append(events, report.Event{Step: "redline", Status: "failed", Message: err.Error()})
			findings = append(findings, report.Finding{
				Level: "error", ID: "redline.check-failed",
				Title: "Redline validation failed", Detail: err.Error(), Location: "redline",
			})
		} else {
			findings = appendRedlineFindings(findings, redlineFindings)
			if len(redlineFindings) > 0 {
				redlineFailed = true
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
	if !cancelled {
		if cErr := ctx.Err(); cErr != nil {
			cancelled, failed = true, true
			env.Status = report.StatusCancelled
			events = append(events, report.Event{Step: "redline", Status: "failed", Message: "run cancelled: " + cErr.Error()})
			findings = append(findings, report.Finding{Level: "error", ID: "run.cancelled", Title: "Run cancelled before completion",
				Detail: "the run's context was cancelled during validation; no output was written"})
		}
	}

	// 输出落盘（INV-3 单次写）：单输出链在全部 stage 通过后只写一次。
	// dry-run、DRM/runner/未实现/目标 stage 失败不写；红线失败仍写（见上）；
	// multi-output 的 split runner 自己管理段产物。
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
			failed = true
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				// ctx 恰好在全部 stage 通过之后、落盘这一步中间才被取消：
				// zipfs.WriteToContext 会清理掉未提交的临时文件、不 rename，
				// 所以 INV-3 的唯一一次落盘同样没有发生；这里只是把信封状态
				// 从默认的 output.write-failed 纠正成 cancelled，别再混进
				// capability.run-failed 的语义里。
				cancelled = true
				env.Status = report.StatusCancelled
				findings = append(findings, report.Finding{
					Level: "error", ID: "run.cancelled",
					Title:  "Run cancelled before completion",
					Detail: "the run's context was cancelled or its deadline was exceeded while writing the output EPUB; no output was written",
				})
				events = append(events, report.Event{Step: "write-output", Status: "failed", Message: "run cancelled: " + err.Error()})
			} else {
				env.Status = report.StatusFailed
				findings = append(findings, report.Finding{
					Level: "error", ID: "output.write-failed",
					Title: "Failed to write output EPUB", Detail: err.Error(),
				})
				events = append(events, report.Event{Step: "write-output", Status: "failed", Message: err.Error()})
			}
		} else {
			outRef := &report.Artifact{Path: opts.OutputPath}
			if sum, err := book.FileSHA256ContextLimit(ctx, opts.OutputPath, 4<<30); err == nil {
				outRef.SHA256 = sum
			} else {
				findings = append(findings, report.Finding{
					Level: "warn", ID: "output.sha256-unavailable", Title: "Output SHA-256 unavailable", Detail: err.Error(), Location: opts.OutputPath,
				})
			}
			env.Output = outRef
			msg := opts.OutputPath
			if redlineFailed {
				msg += " (retained for human diff review despite redline findings)"
			}
			events = append(events, report.Event{Step: "write-output", Status: "completed", Message: msg})
		}
	}

	if opts.DryRun {
		facts["dry_run"] = true
		if b != nil { // noBook 源树模式没有 Book，无修改 entry 可报
			// ModifiedNames 无改动时返回 nil，直接放进 facts 就序列化成 null；
			// 这是个数组形状的 fact，空时必须是 []。
			names := b.ModifiedNames()
			if names == nil {
				names = []string{}
			}
			facts["modified_entries"] = names
		}
	}

	env.Events = events
	env.Findings = findings
	env.Facts = facts
	// nextCommands（SPEC §8.2）：能力依本次结果算出的建议优先——navaudit 依
	// findings、sourceintake 依盘点，都比 pipeline 的静态文本更具体，而且能力
	// 即使 status 是 failed 也照样给（那正是"该跑什么来修"）。能力没给建议时才
	// 退回 pipeline 的静态建议。未执行的能力（DRM 拦截 / 未实现 / 上游 runner
	// 报错）在 up 里没有条目，直接走静态分支。
	env.NextCommands = dropSelfReruns(dedupe(up[contract.ID].NextCommands), contract.ID)
	if len(env.NextCommands) == 0 && !(opts.DryRun && needsWrite && env.Status != report.StatusApprovalRequired) {
		env.NextCommands = nextCommands(contract, opts, userArgs, needsWrite)
	}

	exit := ExitOK
	switch env.Status {
	case report.StatusFailed:
		exit = ExitFailed
	case report.StatusApprovalRequired:
		exit = ExitApproval
	case report.StatusCancelled:
		// SPEC §8.5 只定义 0/1/2/3，没有专门的"取消"退出码档位。取消属于
		// 「没跑完」，与 failed 一样映射到 1；靠 status/finding（而不是退出码）
		// 说清是取消，不是书或工具本身出了问题。不要为取消发明第 4 个退出码，
		// 那会破坏 pre-commit hook / epub_text_gate.py 依赖的既有退出码语义。
		exit = ExitFailed
	}
	return Outcome{Envelope: env, ExitCode: exit}, nil
}

// nextCommands 按 SPEC §8.2 给 agent 提示下一步（`epub run` 形态）。
//
// needsWrite 为假时链上没有待批准的写出（只读 planner / noBook / readOnly
// 能力），dry-run 与正常运行等价：不得建议一个该能力根本不接受的 --output。
func nextCommands(contract Contract, opts Options, userArgs Args, needsWrite bool) []string {
	id := contract.ID
	var out []string
	if opts.DryRun {
		if !needsWrite {
			return nil
		}
		command := "epub run " + id + " --input " + shellQuote(placeholder(opts.InputPath, "<reviewed-input>"))
		if contract.Execution.Output != ExecOutputMulti {
			command += " --output " + shellQuote(placeholder(opts.OutputPath, "<out.epub>"))
		}
		command += " --json"
		args := maps.Clone(userArgs)
		if args == nil {
			args = Args{}
		}
		if contract.Execution.Output == ExecOutputMulti && args["output_dir"] == "" {
			args["output_dir"] = "<out-dir>"
		}
		for _, key := range slices.Sorted(maps.Keys(args)) {
			if key == "input" || key == "output" || key == "dry_run" {
				continue
			}
			command += " " + shellQuote(key+"="+args[key])
		}
		return []string{command}
	}
	switch id {
	case "epub.package.nav.audit":
		out = append(out,
			"epub run epub.layout.audit --input "+shellQuote(placeholder(opts.OutputPath, opts.InputPath)))
	case "epub.structure.normalize":
		out = append(out, "epub redline --check all --path-map <normalize-envelope.json> <before> <after>")
	}
	return out
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

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
		raw, err := book.ReadFileContext(nil, p, 16<<20)
		if err != nil {
			return ExitUsage, fmt.Errorf("读取 --path-map 失败: %w", err)
		}
		m, err := redline.LoadPathMap(raw)
		if err != nil {
			return ExitUsage, err
		}
		pathMap = redline.ComposePathMaps(pathMap, m)
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

// dropSelfReruns 去掉「再跑一遍刚跑完的能力」这类建议。能力自己的命令表
// 可能是一份完整工作流清单（navaudit 就沿用了这个形状，首条是审核自身），
// 但信封里的 nextCommands 是给 agent 的下一步；把自身留在里面会让照单执行的
// agent 原地打转。只按 `epub run <本能力 id>` 前缀判断，不猜其余参数。
func dropSelfReruns(in []string, id string) []string {
	prefix := "epub run " + id
	out := make([]string, 0, len(in))
	for _, cmd := range in {
		trimmed := strings.TrimSpace(cmd)
		if trimmed == prefix || strings.HasPrefix(trimmed, prefix+" ") {
			continue
		}
		out = append(out, cmd)
	}
	return out
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
		if c.Execution.Output == ExecOutputSingle || c.Execution.Output == ExecOutputMulti {
			return true
		}
	}
	return false
}

func chainImplemented(chain []Contract) bool {
	for _, c := range chain {
		if !Implemented(c.ID) {
			return false
		}
	}
	return true
}

func chainRedLines(chain []Contract) []string {
	var out []string
	for _, c := range chain {
		out = append(out, c.RedLines...)
	}
	return dedupe(out)
}

func mergeRenames(dst, src map[string]string) {
	composed := redline.ComposePathMaps(dst, src)
	clear(dst)
	maps.Copy(dst, composed)
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

// appendUpstreamDiagnostics 把上游（requires）stage 的结果记为非阻断诊断：
// Status 与完整 findings 进 facts，信封只得到一条 info 摘要与 completed 事件。
func appendUpstreamDiagnostics(events []report.Event, findings []report.Finding, facts map[string]any, step string, result report.Result) ([]report.Event, []report.Finding) {
	errN, warnN := 0, 0
	for _, f := range result.Findings {
		switch f.Level {
		case "error":
			errN++
		case "warn":
			warnN++
		}
	}
	upstreamFindings := result.Findings
	if upstreamFindings == nil {
		upstreamFindings = []report.Finding{}
	}
	facts[step+".status"] = result.Status
	facts[step+".findings"] = upstreamFindings
	events = append(events, report.Event{Step: step, Status: "completed",
		Message: fmt.Sprintf("diagnostic: %d error, %d warn", errN, warnN)})
	if errN+warnN > 0 {
		findings = append(findings, report.Finding{
			Level: "info", ID: "upstream.diagnostics",
			Title:    fmt.Sprintf("%s reported %d error / %d warn findings (diagnostic, not blocking)", step, errN, warnN),
			Detail:   fmt.Sprintf("see facts[%q]", step+".findings"),
			Location: step,
		})
	}
	return events, findings
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
