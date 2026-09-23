// Package typography 迁移 scripts/epub_style_preset_tool.py 的 apply 语义
// （capability id：epub.typography.optimize）：
//
//   - 读取 preset（preset.json + Styles/*.css，≤500 行硬校验）；
//   - coverage 统计（used/covered class、ratio round4、threshold 0.3，
//     低于阈值输出中文 warning）；
//   - 层文件拷贝到 OPF 同级的 Styles/ 目录（存在即替换，否则新增）；
//   - manifest ensure（unique_id style-{stem}）+ media-type 补齐；
//   - spine 页面 stylesheet link 整行重写（LINK_RE 多行删除 + </head>
//     前插入新链接）；
//   - OPF 字节区间编辑（INV-2：不整文档重序列化）；
//   - dry-run 同样生成内存候选，由 pipeline 跳过落盘。
//
// 报告键序对齐 Python dict：version, preset, input, coverage, stylesheets,
// xhtml_links, layers, notes, output, dry_run[, manifest_items_added,
// written_output]。
package typography

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
	"github.com/liyafly/epub-handbook/internal/scan/xhtml"
)

// CapabilityID 是契约 id（contracts/capabilities/v1/epub.typography.optimize.json）。
const CapabilityID = "epub.typography.optimize"

// ErrPreset 对应 Python 的 PresetError（errors.Is 可判）。
var ErrPreset = errors.New("epub.typography.optimize: a preset or EPUB cannot be processed safely")

type presetError struct{ msg string }

func (e *presetError) Error() string   { return e.msg }
func (e *presetError) Is(t error) bool { return t == ErrPreset }

func presetErrf(format string, a ...any) error {
	return &presetError{msg: fmt.Sprintf(format, a...)}
}

// DefaultPresetsDir 是 PresetDir 为空时的缺省值（相对工作目录，
// 与 Python PRESETS_ROOT = ROOT/templates/style-presets 对应）。
const DefaultPresetsDir = "templates/style-presets"

// Params 是 capability 参数。
type Params struct {
	// Preset 是 preset 目录名（templates/style-presets/<name>/）。
	Preset string
	// PresetDir 覆盖 preset 根目录；为空用 DefaultPresetsDir。
	PresetDir string
	// Output 是输出路径（报告字段 + 前置校验；本包不落盘，INV-3）。
	Output string
	// ScopePaths 非 nil 时仅向这些 spine XHTML 追加隔离样式，不替换共享 CSS。
	ScopePaths []string
	// DryRun 标记内存预览，不写输出。
	DryRun bool
}

// ---- 报告累加器（只进入 Result.Facts，不再有独立 JSON 形状） ----

// presetCoverage 是类覆盖度统计，序列化进 facts["coverage"]。
type presetCoverage struct {
	UsedClasses    []string
	CoveredClasses []string
	Ratio          float64
	Threshold      float64
	Warning        *string
}

// stylesheetAction 是逐层样式表动作，序列化进 facts["stylesheetActions"]。
type stylesheetAction struct {
	Path   string `json:"path"`
	Source string `json:"source"`
	Action string `json:"action"`
}

// presetReport 汇总 dry-run 与 apply 共有的报告字段。
type presetReport struct {
	Preset      string
	Coverage    presetCoverage
	Stylesheets []stylesheetAction
	XHTMLLinks  []string
	Layers      []string
	Notes       string
	DryRun      bool
}

// ---- preset 读取 ----

type presetConfig struct {
	CSS    map[string][]byte
	Name   string
	Layers []string
	Notes  string
}

// loadPreset 逐行复刻 load_preset（含 ≤500 行硬校验）。返回 (config,
// preset 自身目录)——与 Python 的 (config, preset_dir) 二元组对应。
// presetDir 是 preset 根目录（PRESETS_ROOT），具体 preset 在其 name 子目录。
func loadPreset(ctx context.Context, name, presetDir string) (presetConfig, string, error) {
	dir := filepath.Join(filepath.FromSlash(presetDir), name)
	configPath := filepath.Join(dir, "preset.json")
	if !isRegularFile(configPath) {
		return presetConfig{}, "", presetErrf("unknown preset: %s", name)
	}
	raw, err := book.ReadFileContext(ctx, configPath, 1<<20)
	if err != nil {
		return presetConfig{}, "", fmt.Errorf("invalid preset metadata: %s: %w", configPath, err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return presetConfig{}, "", fmt.Errorf("invalid preset metadata: %s: %w", configPath, err)
	}
	cfgName, _ := cfg["name"].(string)
	cfgVersion, _ := cfg["version"].(string)
	if cfg == nil || cfgName != name || cfgVersion != "1" {
		return presetConfig{}, "", presetErrf("invalid preset metadata: %s", configPath)
	}
	layersAny, ok := cfg["layers"].([]any)
	if !ok || len(layersAny) == 0 || len(layersAny) > 32 {
		return presetConfig{}, "", presetErrf("preset must have 1 to 32 layers: %s", name)
	}
	var presetBytes int
	layers := []string{}
	layerData := map[string][]byte{}
	for _, l := range layersAny {
		layer, ok := l.(string)
		if !ok || !layerNameRe.MatchString(layer) {
			return presetConfig{}, "", presetErrf("invalid stylesheet layer in preset %s: %s", name, pyRepr(layer))
		}
		cssPath := filepath.Join(dir, "Styles", layer)
		data, err := book.ReadFileContext(ctx, cssPath, 4<<20)
		if err != nil {
			return presetConfig{}, "", fmt.Errorf("preset stylesheet %s: %w", cssPath, err)
		}
		presetBytes += len(data)
		if presetBytes > 16<<20 {
			return presetConfig{}, "", presetErrf("preset stylesheet total exceeds 16 MiB: %s", name)
		}
		if pyLineCount(string(data)) > 500 {
			return presetConfig{}, "", presetErrf("preset stylesheet exceeds the 500-line hard limit: %s", cssPath)
		}
		if _, duplicate := layerData[layer]; duplicate {
			return presetConfig{}, "", presetErrf("duplicate stylesheet layer: %s", layer)
		}
		layerData[layer] = data
		layers = append(layers, layer)
	}
	notes, _ := cfg["notes"].(string)
	return presetConfig{Name: cfgName, Layers: layers, Notes: notes, CSS: layerData}, dir, nil
}

// ---- coverage ----

// usedClasses 复刻 used_classes（CLASS_ATTR_RE 的反向引用手工实现）。
func usedClasses(ctx context.Context, raw func(string) ([]byte, error), paths []string) (map[string]bool, error) {
	classes := map[string]bool{}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := raw(path)
		if err != nil {
			return nil, err
		}
		text := decodeUTF8Replace(data)
		for _, value := range scanClassAttrValues(text) {
			for _, tok := range strings.Fields(value) {
				if tok != "" {
					classes[tok] = true
				}
			}
		}
	}
	return classes, nil
}

type nameQuoteMatch struct {
	start, end int
	value      string
}

// findNameQuoteAttr 匹配 \bNAME\s*=\s*(["'])(.*?)\1（re.I | re.S；
// 反向引用 → 取首个同名引号）。Python CLASS_ATTR_RE / rel / href 搜索共用。
func findNameQuoteAttr(text, name string, from int) (nameQuoteMatch, bool) {
	for i := from; i < len(text); {
		p := indexFold(text, name, i)
		if p < 0 {
			return nameQuoteMatch{}, false
		}
		i = p + len(name)
		if !wordBoundaryAt(text, p) {
			continue
		}
		j := skipPySpace(text, i)
		if j >= len(text) || text[j] != '=' {
			continue
		}
		j = skipPySpace(text, j+1)
		if j >= len(text) || (text[j] != '"' && text[j] != '\'') {
			continue
		}
		q := text[j]
		vs := j + 1
		ve := strings.IndexByte(text[vs:], q)
		if ve < 0 {
			continue
		}
		return nameQuoteMatch{start: p, end: vs + ve + 1, value: text[vs : vs+ve]}, true
	}
	return nameQuoteMatch{}, false
}

// scanClassAttrValues 返回全部 class 属性值（CLASS_ATTR_RE 语义）。
func scanClassAttrValues(text string) []string {
	var out []string
	for i := 0; ; {
		m, ok := findNameQuoteAttr(text, "class", i)
		if !ok {
			break
		}
		out = append(out, m.value)
		i = m.end
	}
	return out
}

// presetClasses 复刻 preset_classes：注释剥离后按 CSS_CLASS_RE 收集
// `(?<![\w-])\.([A-Za-z_][\w-]*)`（负向后顾手工实现）。
func presetClasses(layers map[string][]byte) map[string]bool {
	classes := map[string]bool{}
	for _, data := range layers {
		scanCSSClasses(css.StripComments(string(data)), classes)
	}
	return classes
}

func scanCSSClasses(text string, out map[string]bool) {
	for i := 0; i < len(text); {
		if text[i] != '.' {
			i++
			continue
		}
		if i > 0 {
			r, _ := decodeLastRune(text[:i])
			if isWordRune(r) || r == '-' {
				i++
				continue
			}
		}
		j := i + 1
		if j >= len(text) {
			break
		}
		r, size := decodeRune(text[j:])
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_') {
			i++
			continue
		}
		j += size
		for j < len(text) {
			r2, s2 := decodeRune(text[j:])
			if !isWordRune(r2) && r2 != '-' {
				break
			}
			j += s2
		}
		out[text[i+1:j]] = true
		i = j
	}
}

// pyRound4 复刻 Python round(x, 4)（四位小数无精确二进制 tie，
// 与正确的十进制舍入等价）。
func pyRound4(v float64) float64 {
	out, err := strconv.ParseFloat(strconv.FormatFloat(v, 'f', 4, 64), 64)
	if err != nil {
		return v
	}
	return out
}

// coverageReport 复刻 coverage_report（threshold 比较用四舍五入后的 ratio）。
func coverageReport(used, styled map[string]bool) presetCoverage {
	covered := map[string]bool{}
	for c := range used {
		if styled[c] {
			covered[c] = true
		}
	}
	ratio := 0.0
	if len(used) > 0 {
		ratio = float64(len(covered)) / float64(len(used))
	}
	rounded := pyRound4(ratio)
	warning := (*string)(nil)
	if rounded < coverageThreshold {
		w := coverageWarningText
		warning = &w
	}
	return presetCoverage{
		UsedClasses:    sortedSet(used),
		CoveredClasses: sortedSet(covered),
		Ratio:          rounded,
		Threshold:      coverageThreshold,
		Warning:        warning,
	}
}

func sortedSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---- Run（SPEC §6.1 三段式：扫描 → 应用 → 报告） ----

// Run 执行本 capability。禁止修改 b 之外的任何状态；落盘由 pipeline 的
// b.WriteTo 负责（INV-3）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	presetDir := p.PresetDir
	if presetDir == "" {
		presetDir = DefaultPresetsDir
	}
	config, _, err := loadPreset(ctx, p.Preset, presetDir)
	if err != nil {
		return report.Result{}, err
	}
	if err := validateOutputPaths(b.InputPath(), p.Output, p.DryRun); err != nil {
		return report.Result{}, err
	}

	// 1. 扫描（只读）。
	opfPath, err := opfPathFromContainer(b)
	if err != nil {
		return report.Result{}, err
	}
	opfData, err := b.CurrentContext(ctx, opfPath)
	if err != nil {
		return report.Result{}, presetErrf("%v", err)
	}
	opfRoot, err := opf.ScanSpanTree(opfData)
	if err != nil {
		return report.Result{}, presetErrf("%s: XML parse failed: %v", opfPath, err)
	}
	opfDir := pypath.Dirname(opfPath)

	xhtmlPaths, err := spineXHTMLPaths(opfRoot, opfPath)
	if err != nil {
		return report.Result{}, err
	}
	if p.ScopePaths != nil {
		xhtmlPaths, err = selectScope(xhtmlPaths, p.ScopePaths)
		if err != nil {
			return report.Result{}, err
		}
	}
	stylesDir := pypath.Join(opfDir, "Styles")
	cssPaths := make([]string, 0, len(config.Layers))
	layerData := make(map[string][]byte, len(config.Layers))
	for _, layer := range config.Layers {
		path := pypath.Join(stylesDir, layer)
		cssPaths = append(cssPaths, path)
		layerData[path] = config.CSS[layer]
	}
	fontMode := ""
	if p.ScopePaths == nil {
		fontMode, err = preserveFontMode(ctx, b, opfRoot, xhtmlPaths, pypath.Join(stylesDir, "fonts.css"), layerData)
		if err != nil {
			return report.Result{}, err
		}
	}
	raw := func(name string) ([]byte, error) { return b.CurrentContext(ctx, name) }
	used, err := usedClasses(ctx, raw, xhtmlPaths)
	if err != nil {
		return report.Result{}, err
	}
	styled := presetClasses(layerData)
	exists := func(name string) bool { return b.Has(name) }
	actions := stylesheetActions(exists, opfPath, filepath.Join(presetDir, p.Preset), config.Layers)

	reportBase := presetReport{
		Preset:      p.Preset,
		Coverage:    coverageReport(used, styled),
		Stylesheets: actions,
		XHTMLLinks:  xhtmlPaths,
		Layers:      append([]string(nil), config.Layers...),
		Notes:       config.Notes,
		DryRun:      p.DryRun,
	}

	facts := map[string]any{
		"preset":            p.Preset,
		"coverage":          coverageFacts(reportBase.Coverage),
		"stylesheets":       len(actions),
		"stylesheetActions": actions,
		"xhtmlLinks":        len(xhtmlPaths),
		"xhtmlLinkFiles":    append([]string{}, xhtmlPaths...),
		"layers":            append([]string(nil), config.Layers...),
		"notes":             config.Notes,
		"dryRun":            p.DryRun,
	}
	findings := []report.Finding{}
	if reportBase.Coverage.Warning != nil {
		findings = append(findings, report.Finding{Level: "warn", ID: "typography.low-coverage", Title: *reportBase.Coverage.Warning})
	}

	// 2. 应用（唯一写点）。
	var edits []editset.Edit
	for i, layer := range config.Layers {
		if err := ctx.Err(); err != nil {
			return report.Result{}, err
		}
		data := layerData[cssPaths[i]]
		if p.ScopePaths == nil && layer == "fonts.css" && b.Has(cssPaths[i]) {
			actions[i].Action = "keep"
			actions[i].Source = cssPaths[i]
		}
		if p.ScopePaths != nil {
			if err := validateScopedCSS(data); err != nil {
				return report.Result{}, presetErrf("%s: %v", layer, err)
			}
			d, l, err := bodyBindings(data, nil)
			if err != nil || d || l {
				return report.Result{}, presetErrf("%s: scoped preset must not bind the body font (book-level mode, SPEC §8)", layer)
			}
			cssPaths[i] = scopedStylesheetPath(stylesDir, layer, data)
			actions[i].Path = cssPaths[i]
			actions[i].Action = "add"
		}
		cssPath := cssPaths[i]
		if b.Has(cssPath) {
			cur, err := b.CurrentContext(ctx, cssPath)
			if err != nil {
				return report.Result{}, presetErrf("%v", err)
			}
			if !bytes.Equal(data, cur) {
				if p.ScopePaths != nil {
					return report.Result{}, presetErrf("isolated stylesheet collision: %s", cssPath)
				}
				edits = append(edits, editset.Replace(cssPath, 0, int64(len(cur)), data))
			} else if p.ScopePaths != nil {
				actions[i].Action = "keep"
			}
		} else {
			edits = append(edits, editset.Replace(cssPath, 0, 0, data))
		}
	}

	added, manifestEdits, err := ensureManifestStylesheets(opfPath, opfData, opfRoot, cssPaths)
	if err != nil {
		return report.Result{}, err
	}
	edits = append(edits, manifestEdits...)

	// spine 页面 stylesheet link 整行重写。
	for _, path := range xhtmlPaths {
		if err := ctx.Err(); err != nil {
			return report.Result{}, err
		}
		data, err := b.CurrentContext(ctx, path)
		if err != nil {
			return report.Result{}, presetErrf("%v", err)
		}
		text, ok := utf8Strict(data)
		if !ok {
			return report.Result{}, presetErrf("'utf-8' codec can't decode text resource: %s", path)
		}
		var updated string
		var warnings []string
		if p.ScopePaths != nil {
			updated, err = appendStylesheetLinks(text, path, cssPaths)
		} else {
			updated, warnings, err = rewriteStylesheetLinks(text, path, cssPaths)
		}
		if err != nil {
			return report.Result{}, err
		}
		for _, w := range warnings {
			findings = append(findings, report.Finding{
				Level: "warn", ID: "typography.markup-scan-truncated", Title: w, Location: path,
			})
		}
		if updated != text {
			// 整文件替换（不是最小区间 Edit）：与本仓其余 caps（structure_normalize、
			// alite、cover、merge、metadata、migrate_epub3、split）对已改动 entry
			// 的处理方式一致 —— 它们全部是 `editset.Replace(path, 0, len(old), new)`。
			// INV-1 只要求「未被编辑命中的 entry 原样透传」，不要求已改动的 entry
			// 用最小区间；改成基于 wholeLineIndent/RegionTag 偏移拼最小 Edit 需要把
			// 删除/插入拆成若干条不重叠区间编辑，边际复杂度不小，却拿不到任何额外
			// 正确性收益（不影响 INV-1/INV-2，也不影响正文不变红线），且会让这一个
			// 文件的编辑策略偏离本仓其它 caps 的统一约定。评估结论：维持整文件替换。
			edits = append(edits, editset.Replace(path, 0, int64(len(data)), []byte(updated)))
		}
	}

	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	if err := b.Apply(edits); err != nil {
		return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
	}

	// 3. 报告（不落盘）。
	if !p.DryRun {
		facts["manifestItemsAdded"] = len(added)
		facts["manifestItemsAddedHrefs"] = added
	}
	if fontMode != "" {
		facts["fontMode"] = fontMode
		facts["fontModeAction"] = "preserve"
	}
	if p.ScopePaths != nil {
		facts["applicationMode"] = "scoped-additive"
		facts["scopePaths"] = xhtmlPaths
	}
	return report.Result{
		Capability: CapabilityID,
		Status:     report.StatusComplete,
		Facts:      facts,
		Findings:   nonNilFindings(findings),
		Events: []report.Event{{Step: "style-preset-apply", Status: "completed",
			Message: fmt.Sprintf("preset=%s layers=%d manifest_added=%d", p.Preset, len(config.Layers), len(added))}},
	}, nil
}

// validateOutputPaths 逐行复刻 apply_preset 的前置校验（输入存在性由
// book.Open 保证，这里校验输入/输出冲突与输出已存在）。
func validateOutputPaths(inputPath, outputPath string, dryRun bool) error {
	inputPath = pyAbs(inputPath)
	outputPath = pyAbs(outputPath)
	if st, err := os.Stat(inputPath); err != nil || !st.Mode().IsRegular() {
		return presetErrf("input EPUB does not exist: %s", inputPath)
	}
	if inputPath == outputPath {
		return presetErrf("output must not overwrite the input EPUB")
	}
	if !dryRun {
		if _, err := os.Stat(outputPath); err == nil {
			return presetErrf("output already exists: %s", outputPath)
		}
	}
	return nil
}

// pyAbs 复刻 Path.resolve() 的常规用途：绝对化（不解析符号链接；
// resolve 的链接消解差异只影响极端部署，报告路径以 pipeline 输入为准）。
func pyAbs(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

// ---- spine / manifest / link 重写 ----

// spineXHTMLPaths 复刻 spine_xhtml_paths（media-type ∈ {xhtml, html}）。
func spineXHTMLPaths(opfRoot *opf.SpanNode, opfPath string) ([]string, error) {
	manifestNode := opfRoot.ChildByLocal(opf.OPFURI, "manifest")
	if manifestNode == nil {
		return nil, presetErrf("OPF missing manifest")
	}
	items := map[string]*opf.SpanNode{}
	for _, it := range manifestNode.Kids {
		if it.Name.Space != opf.OPFURI || it.Name.Local != "item" {
			continue
		}
		id, _ := it.AttrByLocal("", "id")
		items[id] = it
	}
	spineNode := opfRoot.ChildByLocal(opf.OPFURI, "spine")
	if spineNode == nil {
		return nil, presetErrf("OPF missing spine")
	}
	var paths []string
	for _, ref := range spineNode.Kids {
		if ref.Name.Space != opf.OPFURI || ref.Name.Local != "itemref" {
			continue
		}
		idref, _ := ref.AttrByLocal("", "idref")
		item := items[idref]
		if item == nil {
			continue
		}
		mediaType, _ := item.AttrByLocal("", "media-type")
		if mediaType != "application/xhtml+xml" && mediaType != "text/html" {
			continue
		}
		href, ok := item.AttrByLocal("", "href")
		if !ok || href == "" {
			continue
		}
		path, err := pypath.ResolveRelativePath(opfPath, pypath.URLSplit(href).Path)
		if err != nil {
			return nil, presetErrf("spine href %q: %v", href, err)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// stylesheetActions 复刻 stylesheet_actions。
func stylesheetActions(exists func(string) bool, opfPath, presetDir string, layers []string) []stylesheetAction {
	stylesDir := pypath.Join(pypath.Dirname(opfPath), "Styles")
	// presetDir = <repo>/templates/style-presets/<name>；Python 的
	// relative_to(ROOT) 需要 repoRoot = presetDir 上三层。
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(filepath.FromSlash(presetDir))))
	actions := make([]stylesheetAction, 0, len(layers))
	for _, layer := range layers {
		path := pypath.Join(stylesDir, layer)
		source := filepath.Join(filepath.FromSlash(presetDir), "Styles", layer)
		rel, err := filepath.Rel(repoRoot, source)
		if err != nil {
			rel = source // Python relative_to 越界会抛错；这里退化为原路径
		}
		action := "add"
		if exists(path) {
			action = "replace"
		}
		actions = append(actions, stylesheetAction{Path: path, Source: rel, Action: action})
	}
	return actions
}

// ensureManifestStylesheets 复刻 ensure_manifest_stylesheets：
// 已存在的条目把 media-type 收敛为 text/css（字节区间编辑），
// 缺失的以 unique_id style-{stem} 追加到 manifest 尾部。
func ensureManifestStylesheets(opfPath string, opfData []byte, opfRoot *opf.SpanNode, cssPaths []string) ([]string, []editset.Edit, error) {
	manifestNode := opfRoot.ChildByLocal(opf.OPFURI, "manifest")
	if manifestNode == nil {
		return nil, nil, presetErrf("OPF missing manifest")
	}
	existing := map[string]*opf.SpanNode{}
	idSeen := map[string]bool{}
	for _, node := range opfRoot.Walk() {
		if id, ok := node.AttrByLocal("", "id"); ok {
			idSeen[id] = true
		}
	}
	for _, it := range manifestNode.Kids {
		if it.Name.Space != opf.OPFURI || it.Name.Local != "item" {
			continue
		}
		if href, ok := it.AttrByLocal("", "href"); ok && href != "" {
			if resolved, err := pypath.ResolveRelativePath(opfPath, pypath.URLSplit(href).Path); err == nil {
				existing[resolved] = it
			}
		}
	}
	added := []string{}
	var edits []editset.Edit
	var insert strings.Builder
	for _, cssPath := range cssPaths {
		href := pypath.RelativeURI(opfPath, cssPath)
		item := existing[cssPath]
		if item == nil {
			id := uniqueID(idSeen, "style-"+pypath.BaseStem(cssPath))
			idSeen[id] = true
			insert.WriteString(opf.BuildCSSItem(id, href))
			added = append(added, href)
			continue
		}
		if mediaType, _ := item.AttrByLocal("", "media-type"); mediaType != "text/css" {
			edits = append(edits, setMediaTypeEdit(opfPath, opfData, item))
		}
	}
	if insert.Len() > 0 {
		edits = append(edits, editset.Insert(opfPath, int64(manifestNode.Close.Start), []byte(insert.String())))
	}
	return added, edits, nil
}

// setMediaTypeEdit 生成 media-type 属性的字节区间编辑（缺失则插入属性）。
func setMediaTypeEdit(opfPath string, opfData []byte, item *opf.SpanNode) editset.Edit {
	if idx := item.AttrIndex("", "media-type"); idx >= 0 {
		if span, _, ok := opf.RawAttrValueSpan(opfData, item, idx); ok {
			return editset.Replace(opfPath, int64(span.Start), int64(span.End-span.Start), []byte("text/css"))
		}
	}
	pos := item.Open.End
	if item.SelfClose {
		pos -= 2 // "/>"
	} else {
		pos-- // ">"
	}
	return editset.Insert(opfPath, int64(pos), []byte(` media-type="text/css"`))
}

// isStylesheetLinkAttrs 复刻 is_stylesheet_link。
func isStylesheetLinkAttrs(attrs string) bool {
	if m, ok := findNameQuoteAttr(attrs, "rel", 0); ok {
		for _, tok := range strings.Fields(m.value) {
			if strings.EqualFold(tok, "stylesheet") {
				return true
			}
		}
	}
	if m, ok := findNameQuoteAttr(attrs, "href", 0); ok {
		path := m.value
		if i := strings.IndexByte(path, '#'); i >= 0 {
			path = path[:i]
		}
		if strings.HasSuffix(strings.ToLower(path), ".css") {
			return true
		}
	}
	return false
}

// rewriteStylesheetLinks 复刻 rewrite_stylesheet_links：删除全部
// stylesheet link 行，再在 </head> 所在行的行首前插入新链接。
//
// 定位与增删只发生在 xhtml.ScanRegions 认定的真实标签字节内（RegionTag）：
// 原实现对整页文本跑正则，一段「展示旧写法」的 HTML 注释里若原样写出
// `</head>` 或 `<link ...>`，新链接会被插进注释里、注释里的示例 <link>
// 行也会被一起删掉 —— 那是正文/注释被误当标记，已修复。<script> 字符串
// 字面量、CDATA 同理不参与匹配。
//
// 与原正则保留的行为一致：`<link>` / `</head>` 必须是所在行第一个非空白
// 字符（对齐原 `(?m)^[ \t]*` 锚点语义），否则该次出现不参与删除/定位——
// 这不是本次要修的缺陷，只是照抄原语义。
//
// 第二个返回值是告警：区域扫描遇到无法闭合的结构而截断时，截断点之后的
// `<link>`/`</head>` 不再改写，调用方必须转成 finding，不能静默半改。
func rewriteStylesheetLinks(text, xhtmlPath string, cssPaths []string) (string, []string, error) {
	regions, stop := xhtml.ScanRegions(text)
	var warnings []string
	if stop != xhtml.ScanComplete {
		warnings = append(warnings, fmt.Sprintf(
			"%s: markup scan stopped at byte offset %d (unterminated comment/CDATA/PI/declaration/tag or unclosed style/script); stylesheet links after this offset left unchanged",
			xhtmlPath, stop))
	}

	type deletion struct{ start, end int }
	var deletes []deletion
	headStart := -1
	headIndent := ""
	headFound := false

	for _, r := range regions {
		if r.Kind != xhtml.RegionTag {
			continue
		}
		tag := text[r.Start:r.End]
		name, attrs, closing := xhtml.TagParts(tag)
		switch {
		case !closing && strings.EqualFold(name, "link"):
			if !isStylesheetLinkAttrs(attrs) {
				continue
			}
			lineStart, _, ok := wholeLineIndent(text, r.Start)
			if !ok {
				continue
			}
			end := r.End
			for end < len(text) && (text[end] == ' ' || text[end] == '\t') {
				end++
			}
			switch {
			case strings.HasPrefix(text[end:], "\r\n"):
				end += 2
			case end < len(text) && text[end] == '\n':
				end++
			}
			deletes = append(deletes, deletion{start: lineStart, end: end})
		case closing && !headFound && strings.EqualFold(name, "head"):
			lineStart, indent, ok := wholeLineIndent(text, r.Start)
			if !ok {
				continue
			}
			headStart, headIndent, headFound = lineStart, indent, true
		}
	}

	if !headFound {
		return "", warnings, presetErrf("XHTML has no </head>: %s", xhtmlPath)
	}

	var out strings.Builder
	out.Grow(len(text) + 64)
	last := 0
	di := 0
	for di < len(deletes) && deletes[di].start < headStart {
		out.WriteString(text[last:deletes[di].start])
		last = deletes[di].end
		di++
	}
	out.WriteString(text[last:headStart])
	indent := headIndent + "  "
	for _, cssPath := range cssPaths {
		out.WriteString(indent + `<link rel="stylesheet" type="text/css" href="` + pypath.RelativeURI(xhtmlPath, cssPath) + `"/>` + "\n")
	}
	last = headStart
	for ; di < len(deletes); di++ {
		out.WriteString(text[last:deletes[di].start])
		last = deletes[di].end
	}
	out.WriteString(text[last:])
	return out.String(), warnings, nil
}

// wholeLineIndent 判断 pos 在 text 中是否是其所在行第一个非空白字符（对齐
// 原正则 `(?m)^[ \t]*` 的锚点语义：从上一个换行符或文本开头到 pos 之间
// 必须全部是空格/制表符）。返回该行起点与那段缩进文本。
func wholeLineIndent(text string, pos int) (lineStart int, indent string, ok bool) {
	lineStart = strings.LastIndexByte(text[:pos], '\n') + 1
	indent = text[lineStart:pos]
	for i := 0; i < len(indent); i++ {
		if indent[i] != ' ' && indent[i] != '\t' {
			return lineStart, indent, false
		}
	}
	return lineStart, indent, true
}

// ---- 报告助手 ----

func coverageFacts(c presetCoverage) map[string]any {
	facts := map[string]any{
		"usedClasses":    c.UsedClasses,
		"coveredClasses": c.CoveredClasses,
		"ratio":          c.Ratio,
		"threshold":      c.Threshold,
	}
	if c.Warning != nil {
		facts["warning"] = *c.Warning
	}
	return facts
}

func nonNilFindings(findings []report.Finding) []report.Finding {
	if len(findings) == 0 {
		return nil
	}
	return findings
}

func utf8Strict(data []byte) (string, bool) {
	if !isUTF8(data) {
		return "", false
	}
	return string(data), true
}

func isUTF8(data []byte) bool { return utf8Valid(data) }

// opfPathFromContainer 复刻 epub_lib.opf_path_from_container。
func opfPathFromContainer(b *book.Book) (string, error) {
	if !b.Has(opf.ContainerPath) {
		return "", presetErrf("missing META-INF/container.xml")
	}
	data, err := b.Current(opf.ContainerPath)
	if err != nil {
		return "", presetErrf("%v", err)
	}
	root, err := opf.ScanSpanTree(data)
	if err != nil {
		return "", presetErrf("META-INF/container.xml: XML parse failed: %v", err)
	}
	opfPath := ""
	for _, e := range root.Walk() {
		if e.Name.Space == opf.ContainerURI && e.Name.Local == "rootfile" {
			opfPath, _ = e.AttrByLocal("", "full-path")
			break
		}
	}
	if opfPath == "" || !b.Has(opfPath) {
		display := opfPath
		if display == "" {
			display = "<missing>"
		}
		return "", presetErrf("container rootfile does not resolve: %s", display)
	}
	return opfPath, nil
}

func isRegularFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}
