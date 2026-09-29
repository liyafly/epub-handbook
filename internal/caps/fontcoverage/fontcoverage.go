// Package fontcoverage 适配 epub.font.coverage.analyze，并经 internal/extern
// 调用 PATH 中的 epub-font provider。
package fontcoverage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/extern"
	"github.com/liyafly/epub-handbook/internal/report"
)

// CapabilityID 是本能力的契约 id。
const CapabilityID = "epub.font.coverage.analyze"

// Params 是本能力的参数。
type Params struct {
	// Profile 是检测档案：ideal-browser | kindle-pessimistic。
	Profile string
}

// detectorReport 是 detector 顶层 JSON 对象（键 → 值）。
type detectorReport struct {
	vals map[string]any
}

func parseOrdered(data []byte) (*detectorReport, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errors.New("not a JSON object")
	}
	rep := &detectorReport{vals: map[string]any{}}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, errors.New("non-string object key")
		}
		var v any
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		rep.vals[key] = v
	}
	if _, err := dec.Token(); err != nil { // closing }
		return nil, err
	}
	return rep, nil
}

// Run 执行字体覆盖检测（只读）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	if p.Profile == "" {
		p.Profile = "kindle-pessimistic"
	}
	res := report.Result{Capability: CapabilityID, Status: report.StatusComplete}

	input := b.InputPath()
	if abs, err := filepath.Abs(input); err == nil {
		input = abs
	}
	if _, err := os.Stat(input); err != nil {
		return adapterFailure(&res, fmt.Sprintf("input EPUB does not exist: %s", input))
	}
	if err := extern.Require("epub-font"); err != nil {
		return adapterFailure(&res, "install the epub-font provider and retry")
	}
	run, runErr := extern.Run(ctx, "", []string{
		"epub-font", "coverage", input,
		"--profile", p.Profile, "--json", "--quiet",
	})
	if runErr != nil {
		// ctx 取消/超时（大书 provider 跑很久时被上层 Ctrl-C 或 deadline 打断）：
		// extern.Run 已经把 ctx 的错误联结进 runErr。这里必须原样透传，让
		// pipeline 区分取消与 detector 故障；其他 adapter 故障则作为结构化结果返回。
		if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
			return res, runErr
		}
		// 进程根本没起来（uv 在 LookPath 之后消失、权限不足、工作目录缺失…）。
		// 丢掉这个 error 会让下面的 parseOrdered 拿着零值 CmdResult 走失败分支，
		// 报出 "exit code 0" —— 暗示工具跑完了且干净退出，恰好把真正的原因
		// （工具没装 / 起不来）藏起来。extern.ErrToolMissing 也在这里。
		return adapterFailure(&res, fmt.Sprintf("epub-font coverage failed: %v", runErr))
	}
	det, perr := parseOrdered(run.Stdout)
	if perr != nil {
		detail := trimmed(run.Stderr)
		if detail == "" {
			detail = trimmed(run.Stdout)
		}
		if detail == "" {
			detail = fmt.Sprintf("exit code %d", run.ExitCode)
		}
		return adapterFailure(&res, fmt.Sprintf("epub-font coverage did not return JSON: %s", detail))
	}
	if v, _ := det.vals["schema_version"].(string); v != "1.0" {
		return adapterFailure(&res, "epub-font coverage returned an unsupported report schema")
	}
	status := statusFor(det, p.Profile)

	// facts：`profile` / `status` 是本次结论；detector 报告的各段以 camelCase
	// 键原样透出（段内字段沿用 detector 的 snake_case 键名）：`summary`、
	// `charInventory`（问题字与出现位置）、`unresolved`（未解析 CSS run）、
	// `chainHealth`（字体链健康）、`textRuns`；`detectorExitCode` /
	// `detectorStderr` 便于排查 provider 本身的问题。
	res.Facts = map[string]any{
		"profile":          p.Profile,
		"status":           status,
		"detectorExitCode": run.ExitCode,
	}
	for key, fact := range detectorFactKeys() {
		if v, ok := det.vals[key]; ok {
			res.Facts[fact] = v
		}
	}
	if s := trimmed(run.Stderr); s != "" {
		res.Facts["detectorStderr"] = s
	}
	switch status {
	case "fail":
		res.Status = report.StatusFailed
		res.Findings = append(res.Findings, report.Finding{
			Level: "error", ID: "fontcoverage.fail",
			Title: "epub-font coverage reported fail for profile " + p.Profile,
		})
	case "warn":
		res.Findings = append(res.Findings, report.Finding{
			Level: "warn", ID: "fontcoverage.risk",
			Title: "epub-font coverage reported risk for profile " + p.Profile,
		})
	}
	return res, nil
}

// detectorFactKeys 把 detector JSON 顶层段映射到正式 facts 键
// （函数而非包级 var：INV-7 禁止包级可变状态）。
func detectorFactKeys() map[string]string {
	return map[string]string{
		"summary":        "summary",
		"char_inventory": "charInventory",
		"unresolved":     "unresolved",
		"chain_health":   "chainHealth",
		"text_runs":      "textRuns",
	}
}

// statusFor 复刻 status_for，含 Python 的短路优先级：
// (counts 是 dict 且 risk>0) OR unresolved>0 —— unresolved 检查不受
// counts 类型影响。
func statusFor(det *detectorReport, profile string) string {
	summary, _ := det.vals["summary"].(map[string]any)
	if summary == nil {
		return "fail"
	}
	profiles, _ := summary["by_profile_risk"].(map[string]any)
	counts, _ := profiles[profile].(map[string]any)
	if counts != nil {
		if failCount, _ := counts["fail"].(float64); int(failCount) > 0 {
			return "fail"
		}
	}
	riskPos := false
	if counts != nil {
		if risk, _ := counts["risk"].(float64); int(risk) > 0 {
			riskPos = true
		}
	}
	unresolved, _ := summary["unresolved_runs"].(float64)
	if riskPos || int(unresolved) > 0 {
		return "warn"
	}
	return "pass"
}

func adapterFailure(res *report.Result, msg string) (report.Result, error) {
	res.Status = report.StatusFailed
	res.Findings = append(res.Findings, report.Finding{
		Level: "error", ID: "fontcoverage.adapter", Title: msg,
	})
	return *res, nil
}

func trimmed(b []byte) string {
	s := string(b)
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\n' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\n' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
