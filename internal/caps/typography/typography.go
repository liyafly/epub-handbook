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
//   - dry-run 只出报告不应用。
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
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
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
	// DryRun 对齐 Python --dry-run：只出报告，不应用、不写 written_output。
	DryRun bool
	// LegacyReport 为 true 时把 Python 形状的 JSON 报告放进
	// Result.Facts["legacyReport"]（json.RawMessage），供 parity gate P2。
	LegacyReport bool
}

// ---- legacy 报告形状（dict 插入序 = 结构体字段序） ----

type legacyCoverage struct {
	UsedClasses    []string       `json:"used_classes"`
	CoveredClasses []string       `json:"covered_classes"`
	Ratio          report.PyFloat `json:"ratio"`
	Threshold      report.PyFloat `json:"threshold"`
	Warning        *string        `json:"warning"`
}

type legacyStylesheetAction struct {
	Path   string `json:"path"`
	Source string `json:"source"`
	Action string `json:"action"`
}

// legacyPresetReport 对齐 dry-run 与 apply 共有的键（version..dry_run）。
type legacyPresetReport struct {
	Version    string                   `json:"version"`
	Preset     string                   `json:"preset"`
	Input      string                   `json:"input"`
	Coverage   legacyCoverage           `json:"coverage"`
	Stylesheets []legacyStylesheetAction `json:"stylesheets"`
	XHTMLLinks []string                 `json:"xhtml_links"`
	Layers     []string                 `json:"layers"`
	Notes      string                   `json:"notes"`
	Output     string                   `json:"output"`
	DryRun     bool                     `json:"dry_run"`
}

// legacyPresetApplyReport 在 apply 时追加 manifest_items_added 与
// written_output（保持 Python 的插入序）。
type legacyPresetApplyReport struct {
	legacyPresetReport
	ManifestItemsAdded []string `json:"manifest_items_added"`
	WrittenOutput      string   `json:"written_output"`
}

// ---- preset 读取 ----

type presetConfig struct {
	Name   string
	Layers []string
	Notes  string
}

// loadPreset 逐行复刻 load_preset（含 ≤500 行硬校验）。返回 (config,
// preset 自身目录)——与 Python 的 (config, preset_dir) 二元组对应。
// presetDir 是 preset 根目录（PRESETS_ROOT），具体 preset 在其 name 子目录。
func loadPreset(name, presetDir string) (presetConfig, string, error) {
	dir := filepath.Join(filepath.FromSlash(presetDir), name)
	configPath := filepath.Join(dir, "preset.json")
	if !isRegularFile(configPath) {
		return presetConfig{}, "", presetErrf("unknown preset: %s", name)
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return presetConfig{}, "", presetErrf("invalid preset metadata: %s: %v", configPath, err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return presetConfig{}, "", presetErrf("invalid preset metadata: %s: %v", configPath, err)
	}
	cfgName, _ := cfg["name"].(string)
	cfgVersion, _ := cfg["version"].(string)
	if cfg == nil || cfgName != name || cfgVersion != "1" {
		return presetConfig{}, "", presetErrf("invalid preset metadata: %s", configPath)
	}
	layersAny, ok := cfg["layers"].([]any)
	if !ok || len(layersAny) == 0 {
		return presetConfig{}, "", presetErrf("preset has no layers: %s", name)
	}
	layers := []string{}
	for _, l := range layersAny {
		layer, ok := l.(string)
		if !ok || pyBasename(layer) != layer || !strings.HasSuffix(layer, ".css") {
			return presetConfig{}, "", presetErrf("invalid stylesheet layer in preset %s: %s", name, pyRepr(layer))
		}
		cssPath := filepath.Join(dir, "Styles", layer)
		data, err := os.ReadFile(cssPath)
		if err != nil {
			return presetConfig{}, "", presetErrf("preset stylesheet is missing: %s", cssPath)
		}
		if pyLineCount(string(data)) > 500 {
			return presetConfig{}, "", presetErrf("preset stylesheet exceeds the 500-line hard limit: %s", cssPath)
		}
		layers = append(layers, layer)
	}
	notes, _ := cfg["notes"].(string)
	return presetConfig{Name: cfgName, Layers: layers, Notes: notes}, dir, nil
}

// ---- coverage ----

// usedClasses 复刻 used_classes（CLASS_ATTR_RE 的反向引用手工实现）。
func usedClasses(raw func(string) ([]byte, bool), paths []string) (map[string]bool, error) {
	classes := map[string]bool{}
	for _, path := range paths {
		data, ok := raw(path)
		if !ok {
			return nil, presetErrf("spine item is missing from EPUB: %s", path)
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
func presetClasses(presetDir, presetName string, layers []string) (map[string]bool, error) {
	classes := map[string]bool{}
	dir := filepath.Join(filepath.FromSlash(presetDir), presetName)
	for _, layer := range layers {
		data, err := os.ReadFile(filepath.Join(dir, "Styles", layer))
		if err != nil {
			return nil, presetErrf("preset stylesheet is missing: %s", filepath.Join(dir, "Styles", layer))
		}
		scanCSSClasses(css.StripComments(string(data)), classes)
	}
	return classes, nil
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
func coverageReport(used, styled map[string]bool) legacyCoverage {
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
	return legacyCoverage{
		UsedClasses:    sortedSet(used),
		CoveredClasses: sortedSet(covered),
		Ratio:          report.PyFloat(rounded),
		Threshold:      report.PyFloat(coverageThreshold),
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
	presetDir := p.PresetDir
	if presetDir == "" {
		presetDir = DefaultPresetsDir
	}
	config, _, err := loadPreset(p.Preset, presetDir)
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
	opfData, err := b.Current(opfPath)
	if err != nil {
		return report.Result{}, presetErrf("%v", err)
	}
	opfRoot, err := opf.ScanSpanTree(opfData)
	if err != nil {
		return report.Result{}, presetErrf("%s: XML parse failed: %v", opfPath, err)
	}
	opfDir := pyDirname(opfPath)

	xhtmlPaths, err := spineXHTMLPaths(opfRoot, opfPath)
	if err != nil {
		return report.Result{}, err
	}
	raw := func(name string) ([]byte, bool) {
		data, err := b.Current(name)
		if err != nil {
			return nil, false
		}
		return data, true
	}
	used, err := usedClasses(raw, xhtmlPaths)
	if err != nil {
		return report.Result{}, err
	}
	styled, err := presetClasses(presetDir, p.Preset, config.Layers)
	if err != nil {
		return report.Result{}, err
	}
	exists := func(name string) bool { return b.Has(name) }
	actions := stylesheetActions(exists, opfPath, filepath.Join(presetDir, p.Preset), config.Layers)

	reportBase := legacyPresetReport{
		Version:     "1",
		Preset:      p.Preset,
		Input:       b.InputPath(),
		Coverage:    coverageReport(used, styled),
		Stylesheets: actions,
		XHTMLLinks:  xhtmlPaths,
		Layers:      append([]string(nil), config.Layers...),
		Notes:       config.Notes,
		Output:      p.Output,
		DryRun:      p.DryRun,
	}

	facts := map[string]any{
		"preset":       p.Preset,
		"coverage":     map[string]any{},
		"stylesheets":  len(actions),
		"xhtmlLinks":   len(xhtmlPaths),
		"layers":       append([]string(nil), config.Layers...),
		"notes":        config.Notes,
		"dryRun":       p.DryRun,
	}
	findings := []report.Finding{}
	if reportBase.Coverage.Warning != nil {
		findings = append(findings, report.Finding{Level: "warn", ID: "typography.low-coverage", Title: *reportBase.Coverage.Warning})
	}

	if p.DryRun {
		facts["coverage"] = coverageFacts(reportBase.Coverage)
		if p.LegacyReport {
			rawJSON, err := report.MarshalLegacy(reportBase)
			if err != nil {
				return report.Result{Capability: CapabilityID, Status: report.StatusFailed}, err
			}
			facts["legacyReport"] = jsonRawMessage(rawJSON)
		}
		return report.Result{
			Capability: CapabilityID,
			Status:     report.StatusComplete,
			Facts:      facts,
			Findings:   nonNilFindings(findings),
			Events:     []report.Event{{Step: "style-preset-apply", Status: "completed", Message: "dry-run: " + p.Preset}},
		}, nil
	}

	// 2. 应用（唯一写点）。
	stylesDir := pyJoinPath(opfDir, "Styles")
	cssPaths := make([]string, 0, len(config.Layers))
	for _, layer := range config.Layers {
		cssPaths = append(cssPaths, pyJoinPath(stylesDir, layer))
	}

	var edits []editset.Edit
	for i, layer := range config.Layers {
		data, err := os.ReadFile(filepath.Join(filepath.FromSlash(presetDir), p.Preset, "Styles", layer))
		if err != nil {
			return report.Result{}, presetErrf("preset stylesheet is missing: %s",
				filepath.Join(filepath.FromSlash(presetDir), p.Preset, "Styles", layer))
		}
		cssPath := cssPaths[i]
		if b.Has(cssPath) {
			cur, err := b.Current(cssPath)
			if err != nil {
				return report.Result{}, presetErrf("%v", err)
			}
			if !bytes.Equal(data, cur) {
				edits = append(edits, editset.Replace(cssPath, 0, int64(len(cur)), data))
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
		data, err := b.Current(path)
		if err != nil {
			return report.Result{}, presetErrf("%v", err)
		}
		text, ok := utf8Strict(data)
		if !ok {
			return report.Result{}, presetErrf("'utf-8' codec can't decode text resource: %s", path)
		}
		updated, err := rewriteStylesheetLinks(text, path, cssPaths)
		if err != nil {
			return report.Result{}, err
		}
		if updated != text {
			edits = append(edits, editset.Replace(path, 0, int64(len(data)), []byte(updated)))
		}
	}

	if err := b.Apply(edits); err != nil {
		return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
	}

	// 3. 报告（不落盘）。
	applyRep := legacyPresetApplyReport{
		legacyPresetReport: reportBase,
		ManifestItemsAdded: added,
		WrittenOutput:      p.Output,
	}
	facts["coverage"] = coverageFacts(reportBase.Coverage)
	facts["manifestItemsAdded"] = len(added)
	if p.LegacyReport {
		rawJSON, err := report.MarshalLegacy(applyRep)
		if err != nil {
			return report.Result{Capability: CapabilityID, Status: report.StatusFailed}, err
		}
		facts["legacyReport"] = jsonRawMessage(rawJSON)
	}
	return report.Result{
		Capability: CapabilityID,
		Status:     report.StatusComplete,
		Facts:      facts,
		Findings:   nonNilFindings(findings),
		Events:     []report.Event{{Step: "style-preset-apply", Status: "completed",
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
	manifestNode := firstOPFChild(opfRoot, "manifest")
	if manifestNode == nil {
		return nil, presetErrf("OPF missing manifest")
	}
	items := map[string]*opf.SpanNode{}
	for _, it := range manifestNode.Kids {
		if it.Name.Space != opf.OPFURI || it.Name.Local != "item" {
			continue
		}
		id, _ := nodeAttr(it, "id")
		items[id] = it
	}
	spineNode := firstOPFChild(opfRoot, "spine")
	if spineNode == nil {
		return nil, presetErrf("OPF missing spine")
	}
	var paths []string
	for _, ref := range spineNode.Kids {
		if ref.Name.Space != opf.OPFURI || ref.Name.Local != "itemref" {
			continue
		}
		idref, _ := nodeAttr(ref, "idref")
		item := items[idref]
		if item == nil {
			continue
		}
		mediaType, _ := nodeAttr(item, "media-type")
		if mediaType != "application/xhtml+xml" && mediaType != "text/html" {
			continue
		}
		href, ok := nodeAttr(item, "href")
		if !ok || href == "" {
			continue
		}
		paths = append(paths, normJoin(pyDirname(opfPath), href))
	}
	return paths, nil
}

// stylesheetActions 复刻 stylesheet_actions。
func stylesheetActions(exists func(string) bool, opfPath, presetDir string, layers []string) []legacyStylesheetAction {
	stylesDir := pyJoinPath(pyDirname(opfPath), "Styles")
	// presetDir = <repo>/templates/style-presets/<name>；Python 的
	// relative_to(ROOT) 需要 repoRoot = presetDir 上三层。
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(filepath.FromSlash(presetDir))))
	actions := make([]legacyStylesheetAction, 0, len(layers))
	for _, layer := range layers {
		path := pyJoinPath(stylesDir, layer)
		source := filepath.Join(filepath.FromSlash(presetDir), "Styles", layer)
		rel, err := filepath.Rel(repoRoot, source)
		if err != nil {
			rel = source // Python relative_to 越界会抛错；这里退化为原路径
		}
		action := "add"
		if exists(path) {
			action = "replace"
		}
		actions = append(actions, legacyStylesheetAction{Path: path, Source: rel, Action: action})
	}
	return actions
}

// ensureManifestStylesheets 复刻 ensure_manifest_stylesheets：
// 已存在的条目把 media-type 收敛为 text/css（字节区间编辑），
// 缺失的以 unique_id style-{stem} 追加到 manifest 尾部。
func ensureManifestStylesheets(opfPath string, opfData []byte, opfRoot *opf.SpanNode, cssPaths []string) ([]string, []editset.Edit, error) {
	manifestNode := firstOPFChild(opfRoot, "manifest")
	if manifestNode == nil {
		return nil, nil, presetErrf("OPF missing manifest")
	}
	opfDir := pyDirname(opfPath)
	existing := map[string]*opf.SpanNode{}
	idSeen := map[string]bool{}
	for _, it := range manifestNode.Kids {
		if it.Name.Space != opf.OPFURI || it.Name.Local != "item" {
			continue
		}
		if href, ok := nodeAttr(it, "href"); ok && href != "" {
			existing[normJoin(opfDir, href)] = it
		}
		if id, ok := nodeAttr(it, "id"); ok {
			idSeen[id] = true
		}
	}
	added := []string{}
	var edits []editset.Edit
	var insert strings.Builder
	for _, cssPath := range cssPaths {
		href := relHref(opfPath, cssPath)
		item := existing[cssPath]
		if item == nil {
			id := uniqueID(idSeen, "style-"+pyPathStem(cssPath))
			idSeen[id] = true
			insert.WriteString(opfItemElement(id, href))
			added = append(added, href)
			continue
		}
		if mediaType, _ := nodeAttr(item, "media-type"); mediaType != "text/css" {
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
func rewriteStylesheetLinks(text, xhtmlPath string, cssPaths []string) (string, error) {
	var b strings.Builder
	last := 0
	for _, loc := range typoLinkRe.FindAllStringSubmatchIndex(text, -1) {
		attrs := text[loc[4]:loc[5]]
		if !isStylesheetLinkAttrs(attrs) {
			continue
		}
		b.WriteString(text[last:loc[0]])
		last = loc[1]
	}
	b.WriteString(text[last:])
	without := b.String()

	head := typoHeadEndRe.FindStringSubmatchIndex(without)
	if head == nil {
		return "", presetErrf("XHTML has no </head>: %s", xhtmlPath)
	}
	indent := without[head[2]:head[3]] + "  "
	var links strings.Builder
	for _, cssPath := range cssPaths {
		links.WriteString(indent + `<link rel="stylesheet" type="text/css" href="` + relHref(xhtmlPath, cssPath) + `"/>` + "\n")
	}
	return without[:head[0]] + links.String() + without[head[0]:], nil
}

// ---- 报告助手 ----

func coverageFacts(c legacyCoverage) map[string]any {
	facts := map[string]any{
		"usedClasses":    c.UsedClasses,
		"coveredClasses": c.CoveredClasses,
		"ratio":          float64(c.Ratio),
		"threshold":      float64(c.Threshold),
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

// jsonRawMessage 把 MarshalLegacy 的输出作为 RawMessage 存入 Facts，
// 避免 []byte 被信封编码成 base64。
func jsonRawMessage(raw []byte) json.RawMessage {
	return json.RawMessage(bytes.TrimSuffix(raw, []byte("\n")))
}

func isRegularFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}
