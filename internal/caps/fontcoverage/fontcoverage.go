// Package fontcoverage 移植 epub.font.coverage.analyze
// （scripts/epub_font_coverage_adapter.py）：经 internal/extern 调用
// tools-font/coverage-detector（Python + fonttools，明确不迁，SPEC §9.4）。
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

// ErrAdapter 对齐 FontCoverageAdapterError：detector 未能产出合法报告。
var ErrAdapter = errors.New("fontcoverage: adapter error")

// Params 是本能力的参数。
type Params struct {
	// Profile 是检测档案：ideal-browser | kindle-pessimistic。
	Profile string
	// LegacyReport 输出 adapter 的原始 JSON 形状（保持 detector 键序）。
	LegacyReport bool
	// ToolRoot 覆盖 tools-font/coverage-detector 的位置（测试用）。
	ToolRoot string
}

// detectorReport 保持 detector JSON 的键插入序（Python dict 语义）。
type detectorReport struct {
	keys []string
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
		rep.keys = append(rep.keys, key)
		rep.vals[key] = v
	}
	if _, err := dec.Token(); err != nil { // closing }
		return nil, err
	}
	return rep, nil
}

// set 追加（或覆盖）一个键；新键排在已有键之后，对齐 Python dict 追加语义。
func (r *detectorReport) set(key string, v any) {
	if _, exists := r.vals[key]; !exists {
		r.keys = append(r.keys, key)
	}
	r.vals[key] = v
}

// MarshalJSON 保持键插入序输出。
func (r *detectorReport) MarshalJSON() ([]byte, error) {
	var buf []byte
	buf = append(buf, '{')
	for i, k := range r.keys {
		if i > 0 {
			buf = append(buf, ',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf = append(buf, kb...)
		buf = append(buf, ':')
		vb, err := json.Marshal(r.vals[k])
		if err != nil {
			return nil, err
		}
		buf = append(buf, vb...)
	}
	return append(buf, '}'), nil
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
	if ok, _ := extern.LookPath("uv"); !ok {
		return adapterFailure(&res, "uv is required for tools-font/coverage-detector")
	}
	toolRoot := p.ToolRoot
	if toolRoot == "" {
		root, err := findRepoRoot()
		if err != nil {
			return adapterFailure(&res, err.Error())
		}
		toolRoot = filepath.Join(root, "tools-font", "coverage-detector")
	}
	run, _ := extern.Run(toolRoot, []string{
		"uv", "run", "python", "-m", "src.cli", input,
		"--profile", p.Profile, "--json", "--quiet",
	})
	det, perr := parseOrdered(run.Stdout)
	if perr != nil {
		detail := trimmed(run.Stderr)
		if detail == "" {
			detail = trimmed(run.Stdout)
		}
		if detail == "" {
			detail = fmt.Sprintf("exit code %d", run.ExitCode)
		}
		return adapterFailure(&res, fmt.Sprintf("coverage detector did not return JSON: %s", detail))
	}
	if v, _ := det.vals["schema_version"].(string); v != "1.0" {
		return adapterFailure(&res, "coverage detector returned an unsupported report schema")
	}
	status := statusFor(det, p.Profile)
	det.set("capability", CapabilityID)
	det.set("status", status)
	det.set("profile", p.Profile)
	det.set("detector_exit_code", run.ExitCode)
	if s := trimmed(run.Stderr); s != "" {
		det.set("detector_stderr", s)
	}

	res.Facts = map[string]any{
		"profile": p.Profile,
		"status":  status,
	}
	if summary, ok := det.vals["summary"]; ok {
		res.Facts["summary"] = summary
	}
	switch status {
	case "fail":
		res.Status = report.StatusFailed
		res.Findings = append(res.Findings, report.Finding{
			Level: "error", ID: "fontcoverage.fail",
			Title: "Font coverage detector reported fail for profile " + p.Profile,
		})
	case "warn":
		res.Findings = append(res.Findings, report.Finding{
			Level: "warn", ID: "fontcoverage.risk",
			Title: "Font coverage detector reported risk for profile " + p.Profile,
		})
	}
	if p.LegacyReport {
		raw, err := json.Marshal(det)
		if err != nil {
			return report.Result{}, err
		}
		res.Facts["legacyReport"] = json.RawMessage(raw)
	}
	return res, nil
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
	return *res, fmt.Errorf("%w: %s", ErrAdapter, msg)
}

// findRepoRoot 向上找含 tools-font 的目录。
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if st, err := os.Stat(filepath.Join(dir, "tools-font", "coverage-detector")); err == nil && st.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("fontcoverage: 未找到 tools-font/coverage-detector")
		}
		dir = parent
	}
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
