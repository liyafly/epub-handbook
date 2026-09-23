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

// pathMapShapes 是 --path-map 接受的形状说明（错误信息里复用）。
const pathMapShapes = `expected a v2 envelope with facts "*.mappings", ` +
	`{"stages":[{"mappings":[...]}]} or {"mappings":[...]}`

// LoadPathMap 从 JSON 报告载入改名映射，接受三种形状：
//
//   - `epub run epub.structure.normalize --json` 的 v2 统一信封：对象含
//     `schemaVersion` 与 `facts`，取 facts 中键名为 `mappings` 或以
//     `.mappings` 结尾（如 `epub.structure.normalize.mappings`）的数组；
//   - `{"stages":[{"mappings":[...]}]}`：多阶段报告；
//   - `{"mappings":[...]}`：单阶段报告。
//
// 每个 mapping 是 `{"from": ..., "to": ...}`，按出现顺序链式传递
// （AddPathMapping），信封的多个 facts 键按键名排序后依次处理。
//
// 认不出任何 mappings 数组时返回 ErrInput（退出码 3），**不返回空映射**：
// 用户显式传了 --path-map，静默给出零条映射只会让改名后的正文被误判成
// "文件缺失 / 新增文件"，噪声淹没真正的红线问题。空数组（未改名的成功
// normalize 会输出 `"mappings": []`）仍然合法，只是没有映射可用。
func LoadPathMap(data []byte) (map[string]string, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, inputErr("cannot read --path-map JSON: %v", err)
	}
	pathMap := map[string]string{}
	obj, ok := root.(map[string]any)
	if !ok {
		return nil, inputErr("--path-map JSON must be a JSON object; %s", pathMapShapes)
	}
	var lists [][]any
	if facts, isEnvelope := envelopeFacts(obj); isEnvelope {
		keys := make([]string, 0, len(facts))
		for k := range facts {
			if k == "mappings" || strings.HasSuffix(k, ".mappings") {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			list, ok := facts[k].([]any)
			if !ok {
				return nil, inputErr("--path-map envelope facts[%q] must be an array of {from,to} mappings", k)
			}
			lists = append(lists, list)
		}
	} else {
		sources := []any{obj}
		if stages, ok := obj["stages"].([]any); ok {
			sources = stages
		}
		for _, src := range sources {
			stage, ok := src.(map[string]any)
			if !ok {
				continue
			}
			raw, present := stage["mappings"]
			if !present {
				continue
			}
			list, ok := raw.([]any)
			if !ok {
				return nil, inputErr("--path-map \"mappings\" must be an array of {from,to} mappings")
			}
			lists = append(lists, list)
		}
	}
	if len(lists) == 0 {
		return nil, inputErr("--path-map JSON contains no mappings array; %s "+
			"(a failed epub.structure.normalize run emits no mappings — rerun it and pass the successful envelope)",
			pathMapShapes)
	}
	for _, list := range lists {
		for _, m := range list {
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

// envelopeFacts 识别 v2 统一信封（SPEC §8.2）：顶层同时含 schemaVersion
// 与 facts 对象时返回 facts。
//
// `facts` 缺失或为 null（失败信封的常见形态）时按 legacy 形状继续尝试，
// 最终由 LoadPathMap 的"零 mappings"判定统一报输入错误。
func envelopeFacts(obj map[string]any) (map[string]any, bool) {
	if _, ok := obj["schemaVersion"]; !ok {
		return nil, false
	}
	facts, ok := obj["facts"].(map[string]any)
	return facts, ok
}

func openState(path string) (*zipfs.Archive, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, inputErr("input not found: %s", path)
	}
	a, err := zipfs.Open(path)
	if err != nil {
		return nil, inputErr("not a valid zip/EPUB: %s", path)
	}
	seen := make(map[string]bool, len(a.Names()))
	for _, name := range a.Names() {
		if strings.HasSuffix(name, "/") {
			continue
		}
		if seen[name] {
			a.Close()
			return nil, inputErr("duplicate archive entry %q: %s", name, path)
		}
		seen[name] = true
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
