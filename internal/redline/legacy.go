package redline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/liyafly/epub-handbook/internal/zipfs"
)

// init 把六条校验器全部注册（INV-5 闭包的事实来源）。
func init() {
	Register("text", textCheck{})
	Register("anchors", anchorsCheck{})
	Register("metadata", metadataCheck{})
	Register("spine", spineCheck{})
	Register("cover", coverCheck{})
	Register("drm", drmCheck{})
}

// Report 是 legacy 比对协议的结果（对齐 validate_text_invariance.py 的退出码语义）。
type Report struct {
	// Code: 0 成功；1 存在问题；2 DRM 拒绝或输入错误。
	Code int
	// Lines 是按 legacy 顺序排好的输出行（verbose 行已按位置插入）。
	Lines []string
}

// Check 对选中的红线做一次 in-process 比对，返回问题 findings（不含 verbose 行）。
// checks 为空表示全部六条。
func Check(before, after State, checks []string, o Options) ([]Finding, error) {
	selected, err := resolveChecks(checks)
	if err != nil {
		return nil, err
	}
	rep, err := runChecks(before, after, selected, o)
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, f := range rep.findings {
		if !f.Verbose {
			out = append(out, f)
		}
	}
	return out, nil
}

// resolveChecks 复刻 parse_checks：空 / "all" → 全部；否则逗号分词并校验。
func resolveChecks(checks []string) ([]string, error) {
	if len(checks) == 0 {
		return CheckOrder, nil
	}
	if len(checks) == 1 && checks[0] == "all" {
		return CheckOrder, nil
	}
	var out []string
	var invalid []string
	for _, c := range checks {
		c = strings.TrimSpace(c)
		if !contains(CheckOrder, c) {
			invalid = append(invalid, c)
			continue
		}
		if !contains(out, c) {
			out = append(out, c)
		}
	}
	if len(invalid) > 0 {
		sort.Strings(invalid)
		return nil, inputErr("invalid --check value: %s", strings.Join(invalid, ", "))
	}
	return out, nil
}

type runReport struct {
	code     int
	findings []Finding
}

// runChecks 执行比对核心，语义与 validate() 的主体一致。
func runChecks(before, after State, checks []string, o Options) (runReport, error) {
	if contains(checks, CheckDRM) && (hasDRM(before) || hasDRM(after)) {
		staleAllowed := isStaleOnly(before) && isStaleOnly(after)
		fontAllowed := o.AllowFontObfuscation && isFontObfOnly(before) && isFontObfOnly(after)
		if !staleAllowed && !fontAllowed {
			return runReport{code: 2, findings: []Finding{{CheckDRM, "DRM detected, refusing to process.", false}}}, nil
		}
	}
	var findings []Finding
	for _, name := range checkExecOrder {
		if !contains(checks, name) {
			continue
		}
		v, ok := registry[name]
		if !ok {
			return runReport{}, fmt.Errorf("redline %q declared but no validator registered (INV-5)", name)
		}
		fs, err := v.Check(before, after, o)
		if err != nil {
			return runReport{}, err
		}
		findings = append(findings, fs...)
	}
	code := 0
	for _, f := range findings {
		if !f.Verbose {
			code = 1
			break
		}
	}
	return runReport{code: code, findings: findings}, nil
}

// renderReport 把运行结果装配成 legacy 输出行（verbose 行在问题行之前）。
func renderReport(rep runReport, o Options) []string {
	var verbose, problems []string
	for _, f := range rep.findings {
		if f.Verbose {
			verbose = append(verbose, f.Message)
		} else {
			problems = append(problems, f.Message)
		}
	}
	switch {
	case rep.code == 1:
		if o.Verbose {
			return append(verbose, problems...)
		}
		return problems
	case rep.code == 2:
		return problems
	case o.Verbose:
		return append(verbose, "All requested red-line checks passed.")
	default:
		return []string{"All requested red-line checks passed."}
	}
}

// CompareFiles 是 legacy 两文件比对入口，逐字对齐
// `validate_text_invariance.py before after --check ...` 的行为与输出。
// checkArg 传 --check 的原始值（空或 "all" 表示全部）。
func CompareFiles(beforePath, afterPath string, checkArg string, o Options) (Report, error) {
	checks, err := resolveChecks(splitCheckArg(checkArg))
	if err != nil {
		return Report{Code: 2, Lines: []string{"input error: " + err.Error()}}, nil
	}
	before, err := openState(beforePath)
	if err != nil {
		return Report{Code: 2, Lines: []string{"input error: " + err.Error()}}, nil
	}
	defer before.Close()
	after, err := openState(afterPath)
	if err != nil {
		return Report{Code: 2, Lines: []string{"input error: " + err.Error()}}, nil
	}
	defer after.Close()

	rep, err := runChecks(before, after, checks, o)
	if err != nil {
		if isErrInput(err) {
			return Report{Code: 2, Lines: []string{"input error: " + inputErrorText(err)}}, nil
		}
		return Report{}, err
	}
	return Report{Code: rep.code, Lines: renderReport(rep, o)}, nil
}

// LoadPathMap 从 legacy JSON 报告载入改名映射，复刻 load_path_map：
// 顶层 stages 列表或单个对象，逐项取 mappings，链式传递。
func LoadPathMap(data []byte) (map[string]string, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, inputErr("cannot read --path-map JSON: %v", err)
	}
	pathMap := map[string]string{}
	var sources []any
	if obj, ok := root.(map[string]any); ok {
		if stages, ok := obj["stages"].([]any); ok {
			sources = stages
		} else {
			sources = []any{obj}
		}
	}
	for _, src := range sources {
		obj, ok := src.(map[string]any)
		if !ok {
			continue
		}
		mappings, ok := obj["mappings"].([]any)
		if !ok {
			continue
		}
		for _, m := range mappings {
			item, ok := m.(map[string]any)
			if !ok {
				return nil, inputErr("each mapping must contain string from/to paths")
			}
			from, okF := item["from"].(string)
			to, okT := item["to"].(string)
			if !okF || !okT {
				return nil, inputErr("each mapping must contain string from/to paths")
			}
			AddPathMapping(pathMap, from, to)
		}
	}
	return pathMap, nil
}

func openState(path string) (*zipfs.Archive, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, inputErr("input not found: %s", path)
	}
	a, err := zipfs.Open(path)
	if err != nil {
		return nil, inputErr("not a valid zip/EPUB: %s", path)
	}
	return a, nil
}

func splitCheckArg(arg string) []string {
	if strings.TrimSpace(arg) == "" {
		return nil
	}
	return strings.Split(arg, ",")
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func isErrInput(err error) bool { return errors.Is(err, ErrInput) }

func inputErrorText(err error) string {
	msg := err.Error()
	const prefix = "redline: input error: "
	return strings.TrimPrefix(msg, prefix)
}
