// Package structurenormalize 迁移 scripts/epub_structure_tool.py
// （capability id：epub.structure.normalize）：
//
//   - inspect：只读体检（manifest 资源数、encryption 审计）；
//   - format：目录归类（Text/Styles/Images/Fonts/Audio/Video/Misc，
//     ncx 留在 OPF 同级）并重写全部本地引用；
//   - deobfuscate：按 manifest id 生成可读文件名（deobfuscated_basename 规则）；
//   - normalize：两阶段 = 先 format 再 deobfuscate（与 Python 一致，
//     dry-run 只作用于最后一个阶段，阶段 1 始终执行）。
//
// 字节保真策略（parity 基准是 Python oracle 的最终输出字节）：
// XHTML/CSS 用与 Python 完全相同的正则语义做字符串级重写后按原编码回编；
// OPF / encryption.xml 在 Python 侧是 ElementTree 整体重写，这里用
// xmlmini.go 逐条复刻 ET 的解析与序列化规则，保证最终字节一致。
// 所有写入一律以 []editset.Edit 交给 book.Apply（SPEC §6.1 三段式），
// 未修改 entry 由 zipfs 原样透传（INV-1）。
package structurenormalize

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
)

// CapabilityID 是契约 id（contracts/capabilities/v1/epub.structure.normalize.json）。
const CapabilityID = "epub.structure.normalize"

// ErrStructureTool 对应 Python 的 StructureToolError（errors.Is 可判）。
var ErrStructureTool = errors.New("epub.structure.normalize: EPUB cannot be rewritten conservatively")

type toolError struct{ msg string }

func (e *toolError) Error() string   { return e.msg }
func (e *toolError) Is(t error) bool { return t == ErrStructureTool }

func toolErrf(format string, a ...any) error {
	return &toolError{msg: fmt.Sprintf(format, a...)}
}

// Mode 是运行模式，对应 Python 的四个子命令。
type Mode string

const (
	ModeInspect     Mode = "inspect"
	ModeFormat      Mode = "format"
	ModeDeobfuscate Mode = "deobfuscate"
	ModeNormalize   Mode = "normalize"
)

// pythonOperation 映射为 Python 报告里的 operation 字符串。
func (m Mode) pythonOperation() (string, bool) {
	switch m {
	case ModeFormat:
		return "format", true
	case ModeDeobfuscate:
		return "deobfuscate-filenames", true
	case ModeNormalize:
		return "normalize", true
	case ModeInspect:
		return "inspect", true
	}
	return "", false
}

// Params 是 capability 参数。
type Params struct {
	// Mode：inspect | format | deobfuscate | normalize。
	Mode Mode
	// DryRun 只做扫描不应用。normalize 与 Python 语义一致：
	// 阶段 1（format）始终执行（Python 版会写临时文件），dry-run
	// 只作用于阶段 2。
	DryRun bool
	// LegacyReport 为 true 时把 Python 形状的 JSON（RewriteReport /
	// WorkflowReport）放进 Result.Facts["legacyReport"]（json.RawMessage，
	// 由 report.MarshalLegacy 序列化），供 parity gate P2 使用。
	LegacyReport bool
	// Force 只是占位：输出文件冲突由 pipeline 层裁决，包内不处理。
	Force bool
	// Output 是输出路径（仅写入 legacy 报告字段；本包不落盘）。
	// 为空时按 Python 的 default_output 推导（*_formatted/_deobfuscated/
	// _normalized.epub）。
	Output string
}

// ---- legacy 报告形状（dataclass 字段序即 JSON 键序） ----

type legacyMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// legacyRewriteReport 对齐 RewriteReport。
type legacyRewriteReport struct {
	Operation                       string          `json:"operation"`
	Input                           string          `json:"input"`
	Output                          *string         `json:"output"`
	OPF                             string          `json:"opf"`
	ManifestResources               int             `json:"manifest_resources"`
	MovedResources                  int             `json:"moved_resources"`
	RenamedResources                int             `json:"renamed_resources"`
	RewrittenFiles                  int             `json:"rewritten_files"`
	FontObfuscationResources        int             `json:"font_obfuscation_resources"`
	RemovedStaleEncryptionResources int             `json:"removed_stale_encryption_resources"`
	DryRun                          bool            `json:"dry_run"`
	Mappings                        []legacyMapping `json:"mappings"`
	Warnings                        []string        `json:"warnings"`
}

// legacyWorkflowReport 对齐 WorkflowReport。
type legacyWorkflowReport struct {
	Operation string                `json:"operation"`
	Input     string                `json:"input"`
	Output    string                `json:"output"`
	DryRun    bool                  `json:"dry_run"`
	Stages    []legacyRewriteReport `json:"stages"`
}

// ---- 内部数据结构 ----

type manifestResource struct {
	itemID      string
	href        string
	mediaType   string
	archivePath string
}

type encryptionRecord struct {
	uri         string
	algorithm   string
	archivePath string
}

// refRewriter 复刻 rewrite_uri 的判定、重写与告警。
type refRewriter struct {
	pathMap  map[string]string
	files    map[string]bool
	warnings *[]string
}

func (rw *refRewriter) warn(format string, a ...any) {
	*rw.warnings = append(*rw.warnings, fmt.Sprintf(format, a...))
}

// rewriteURI 逐行复刻 rewrite_uri。
func (rw *refRewriter) rewriteURI(uri, oldDocument, newDocument string) string {
	if uri == "" || strings.HasPrefix(uri, "#") || pyIsExternalURI(uri) {
		return uri
	}
	parts := pyURLSplit(uri)
	if parts.path == "" {
		return uri
	}
	oldTarget, err := resolveRelativePath(oldDocument, parts.path)
	if err != nil {
		rw.warn("%s: unsafe local reference left unchanged: %s", oldDocument, uri)
		return uri
	}
	if !rw.files[oldTarget] {
		rw.warn("%s: missing local reference left unchanged: %s", oldDocument, uri)
		return uri
	}
	target := oldTarget
	if mapped, ok := rw.pathMap[oldTarget]; ok {
		target = mapped
	}
	return pyURLUnsplitPath(relativeURI(newDocument, target), parts.query, parts.fragment)
}

// ---- Run（SPEC §6.1 三段式：扫描 → 应用 → 报告） ----

// Run 执行本 capability。禁止修改 b 之外的任何状态；落盘由 pipeline 的
// b.WriteTo 负责（INV-3）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	op, ok := p.Mode.pythonOperation()
	if !ok {
		return report.Result{}, fmt.Errorf("%w: unsupported mode %q", ErrStructureTool, string(p.Mode))
	}
	if p.Mode == ModeInspect {
		return runInspect(b, p)
	}
	if p.Mode == ModeNormalize {
		return runNormalize(b, p)
	}
	return runSingleStage(b, p, op)
}

// scanRewriteStage 复刻 analyze_epub（+ 非 dry-run 的 transform_files）：
// 只读 b，产出报告与 []editset.Edit；不落盘。
func scanRewriteStage(b *book.Book, op string, dryRun bool, outputForReport *string) (stageResult, error) {
	names := b.Names()
	files := make(map[string]bool, len(names))
	for _, n := range names {
		files[n] = true
	}
	current := func(name string) ([]byte, error) { return b.Current(name) }

	rep := legacyRewriteReport{
		Operation: op,
		Input:     b.InputPath(),
		Output:    outputForReport,
		DryRun:    dryRun,
		Mappings:  []legacyMapping{},
		Warnings:  []string{},
	}

	// read_package。
	opfPath, opfRoot, resources, err := readPackage(files, current)
	if err != nil {
		return stageResult{}, err
	}
	rep.OPF = opfPath
	rep.ManifestResources = len(resources) // inspect 保持该值；format/deobfuscate 会被覆盖

	// encryption 审计（inspect 也执行，遇 DRM 直接拒绝）。
	encPath, records, err := inspectEncryption(names, files, current)
	if err != nil {
		return stageResult{}, err
	}
	if err := validateEncryption(records, resources, files, &rep); err != nil {
		return stageResult{}, err
	}

	if op == "inspect" {
		return stageResult{rep: rep}, nil
	}

	// build_path_map。
	pathMap, err := buildPathMap(resources, files, opfPath, op, &rep)
	if err != nil {
		return stageResult{}, err
	}
	if dryRun {
		// Python 的 dry-run 在 transform_files 之前返回：只有 plan，无 rewritten。
		return stageResult{rep: rep, pathMap: pathMap}, nil
	}

	// transform_files → editset.Edit。
	creates, deletes, replaces, err := transformContent(b, names, files, opfPath, opfRoot, encPath, pathMap, &rep)
	if err != nil {
		return stageResult{}, err
	}
	return stageResult{rep: rep, pathMap: pathMap, creates: creates, deletes: deletes, replaces: replaces}, nil
}

type stageResult struct {
	rep      legacyRewriteReport
	pathMap  map[string]string
	creates  []editset.Edit
	deletes  []editset.Edit
	replaces []editset.Edit
}

// applyStage 把扫描产出的编辑交给 book.Apply（唯一写点）。
// 改名 = 删除旧 entry + 新建 entry；当新建路径与被删路径重合
// （A 的原名是 B 的目标）时必须先删后建，故拆成两批 Apply。
func applyStage(b *book.Book, st stageResult) error {
	if len(st.deletes) > 0 {
		if err := b.Apply(st.deletes); err != nil {
			return fmt.Errorf("%s: %w", CapabilityID, err)
		}
	}
	rest := make([]editset.Edit, 0, len(st.creates)+len(st.replaces))
	rest = append(rest, st.creates...)
	rest = append(rest, st.replaces...)
	if len(rest) > 0 {
		if err := b.Apply(rest); err != nil {
			return fmt.Errorf("%s: %w", CapabilityID, err)
		}
	}
	return nil
}

func runSingleStage(b *book.Book, p Params, op string) (report.Result, error) {
	outputPath := p.Output
	if outputPath == "" {
		outputPath = defaultOutput(b.InputPath(), op)
	}
	st, err := scanRewriteStage(b, op, p.DryRun, strPtr(outputPath))
	if err != nil {
		return report.Result{}, err
	}
	if err := applyStage(b, st); err != nil {
		return report.Result{}, err
	}
	return buildResult(p, []legacyRewriteReport{st.rep}, renamesFromStages(st.rep), nil), nil
}

// runNormalize 复刻 normalize_epub：两阶段，阶段 1 始终执行
// （Python 版把 formatted 写进临时目录），dry-run 只作用于阶段 2。
func runNormalize(b *book.Book, p Params) (report.Result, error) {
	outputPath := p.Output
	if outputPath == "" {
		outputPath = defaultOutput(b.InputPath(), "normalize")
	}

	st1, err := scanRewriteStage(b, "format", false, nil)
	if err != nil {
		return report.Result{}, err
	}
	if err := applyStage(b, st1); err != nil {
		return report.Result{}, err
	}
	st1.rep.Output = nil // Python：format_report.output = None

	st2, err := scanRewriteStage(b, "deobfuscate-filenames", p.DryRun, strPtr(outputPath))
	if err != nil {
		return report.Result{}, err
	}
	if !p.DryRun {
		if err := applyStage(b, st2); err != nil {
			return report.Result{}, err
		}
	}
	st2.rep.Input = pyTempFormattedPath() // 不确定字段，保持 Python 语义

	workflow := legacyWorkflowReport{
		Operation: "normalize",
		Input:     b.InputPath(),
		Output:    outputPath,
		DryRun:    p.DryRun,
		Stages:    []legacyRewriteReport{st1.rep, st2.rep},
	}
	return buildResult(p, workflow.Stages, renamesFromStages(st1.rep, st2.rep), &workflow), nil
}

func runInspect(b *book.Book, p Params) (report.Result, error) {
	st, err := scanRewriteStage(b, "inspect", false, nil)
	if err != nil {
		return report.Result{}, err
	}
	return buildResult(p, []legacyRewriteReport{st.rep}, nil, nil), nil
}

// buildResult 装配统一信封的 Result 段（含 legacy-report 脚手架）。
func buildResult(p Params, stages []legacyRewriteReport, renames map[string]string, workflow *legacyWorkflowReport) report.Result {
	main := stages[len(stages)-1]
	facts := map[string]any{
		"mode": string(p.Mode),
	}
	if workflow == nil {
		facts["opf"] = main.OPF
		facts["manifestResources"] = main.ManifestResources
		facts["movedResources"] = main.MovedResources
		facts["renamedResources"] = main.RenamedResources
		facts["rewrittenFiles"] = main.RewrittenFiles
		facts["fontObfuscationResources"] = main.FontObfuscationResources
		facts["removedStaleEncryptionResources"] = main.RemovedStaleEncryptionResources
	} else {
		type stageSummary struct {
			Operation                       string          `json:"operation"`
			OPF                             string          `json:"opf"`
			ManifestResources               int             `json:"manifest_resources"`
			MovedResources                  int             `json:"moved_resources"`
			RenamedResources                int             `json:"renamed_resources"`
			RewrittenFiles                  int             `json:"rewritten_files"`
			FontObfuscationResources        int             `json:"font_obfuscation_resources"`
			RemovedStaleEncryptionResources int             `json:"removed_stale_encryption_resources"`
			DryRun                          bool            `json:"dry_run"`
			Mappings                        []legacyMapping `json:"mappings"`
			Warnings                        []string        `json:"warnings"`
		}
		summaries := make([]stageSummary, 0, len(stages))
		var allMappings []legacyMapping
		var allWarnings []string
		for _, st := range stages {
			summaries = append(summaries, stageSummary{
				Operation: st.Operation, OPF: st.OPF,
				ManifestResources: st.ManifestResources,
				MovedResources:    st.MovedResources, RenamedResources: st.RenamedResources,
				RewrittenFiles:                  st.RewrittenFiles,
				FontObfuscationResources:        st.FontObfuscationResources,
				RemovedStaleEncryptionResources: st.RemovedStaleEncryptionResources,
				DryRun:                          st.DryRun,
				Mappings:                        st.Mappings, Warnings: st.Warnings,
			})
			allMappings = append(allMappings, st.Mappings...)
			allWarnings = append(allWarnings, st.Warnings...)
		}
		facts["stages"] = summaries
		facts["movedResources"] = sumInt(stages, func(s legacyRewriteReport) int { return s.MovedResources })
		facts["renamedResources"] = sumInt(stages, func(s legacyRewriteReport) int { return s.RenamedResources })
		facts["rewrittenFiles"] = sumInt(stages, func(s legacyRewriteReport) int { return s.RewrittenFiles })
		facts["mappings"] = nonNilMappings(allMappings)
		facts["warnings"] = nonNilStrings(allWarnings)
	}
	facts["dryRun"] = p.DryRun

	var findings []report.Finding
	for _, st := range stages {
		for _, w := range st.Warnings {
			findings = append(findings, report.Finding{Level: "warn", ID: "structure.warning", Title: w})
		}
	}

	var events []report.Event
	for _, st := range stages {
		events = append(events, report.Event{
			Step:   st.Operation,
			Status: "completed",
			Message: fmt.Sprintf("moved=%d renamed=%d rewritten=%d dry_run=%t",
				st.MovedResources, st.RenamedResources, st.RewrittenFiles, st.DryRun),
		})
	}

	if p.LegacyReport {
		var v any = any(main)
		if workflow != nil {
			v = workflow
		}
		raw, err := report.MarshalLegacy(v)
		if err != nil {
			return report.Result{Capability: CapabilityID, Status: report.StatusFailed}
		}
		// 存 json.RawMessage，避免 []byte 被信封编码成 base64。
		facts["legacyReport"] = json.RawMessage(bytesTrimNewline(raw))
	}

	return report.Result{
		Capability: CapabilityID,
		Status:     report.StatusComplete,
		Facts:      facts,
		Findings:   findings,
		Events:     events,
		Renames:    renames,
	}
}

func sumInt(stages []legacyRewriteReport, get func(legacyRewriteReport) int) int {
	total := 0
	for _, st := range stages {
		total += get(st)
	}
	return total
}

func nonNilMappings(in []legacyMapping) []legacyMapping {
	if in == nil {
		return []legacyMapping{}
	}
	return in
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func strPtr(s string) *string { return &s }

// sha256Hex12 复刻 hashlib.sha256(seed).hexdigest()[:12]。
func sha256Hex12(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])[:12]
}

// renamesFromStages 把各阶段 mapping 链式展开成 Result.Renames
// （语义同 validate_text_invariance.add_path_mapping：先改既有映射中
// 目标为 source 的键，再登记 source→target；两阶段链式后 from 即原始名）。
func renamesFromStages(stages ...legacyRewriteReport) map[string]string {
	renames := map[string]string{}
	for _, st := range stages {
		for _, m := range st.Mappings {
			addPathMapping(renames, m.From, m.To)
		}
	}
	if len(renames) == 0 {
		return nil
	}
	return renames
}

func addPathMapping(m map[string]string, source, target string) {
	for k, v := range m {
		if v == source {
			m[k] = target
		}
	}
	m[source] = target
}

// defaultOutput 复刻 default_output。
func defaultOutput(inputPath, operation string) string {
	suffix := map[string]string{
		"format":                "_formatted.epub",
		"deobfuscate-filenames": "_deobfuscated.epub",
		"normalize":             "_normalized.epub",
	}[operation]
	name := pyBasename(filepath.ToSlash(inputPath))
	stem := name
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		stem = name[:i]
	}
	return filepath.Join(filepath.Dir(filepath.FromSlash(inputPath)), stem+suffix)
}

// pyTempFormattedPath 复刻 normalize 的 stage[1].input：
// tempfile.TemporaryDirectory(prefix="epub-structure-tool-")/formatted.epub。
// 这是每次运行都变化的路径（与 Python 同为不确定字段），仅保形状。
func pyTempFormattedPath() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789_"
	raw := make([]byte, 8)
	if _, err := cryptorand.Read(raw); err != nil {
		for i := range raw {
			raw[i] = 'a'
		}
	}
	for i := range raw {
		raw[i] = chars[int(raw[i])%len(chars)]
	}
	return filepath.Join(os.TempDir(), "epub-structure-tool-"+string(raw), "formatted.epub")
}

// ---- read_package ----

func readPackage(files map[string]bool, current func(string) ([]byte, error)) (string, *xmlElem, []manifestResource, error) {
	const containerPath = "META-INF/container.xml"
	if !files[containerPath] {
		return "", nil, nil, toolErrf("missing META-INF/container.xml")
	}
	containerData, err := current(containerPath)
	if err != nil {
		return "", nil, nil, toolErrf("%v", err)
	}
	container, err := parseXMLTree(containerData)
	if err != nil {
		return "", nil, nil, toolErrf("%s: XML parse failed: %v", containerPath, err)
	}
	opfPath := ""
	for _, e := range iterAll(container) {
		if e.name == "rootfile" {
			opfPath, _ = e.getAttr("full-path")
			break
		}
	}
	if opfPath == "" {
		return "", nil, nil, toolErrf("container.xml has no rootfile full-path")
	}
	opfPath, err = validateArchivePath(opfPath, "container.xml rootfile")
	if err != nil {
		return "", nil, nil, err
	}
	if !files[opfPath] {
		return "", nil, nil, toolErrf("container.xml rootfile does not resolve: %s", opfPath)
	}
	opfData, err := current(opfPath)
	if err != nil {
		return "", nil, nil, toolErrf("%v", err)
	}
	opfRoot, err := parseXMLTree(opfData)
	if err != nil {
		return "", nil, nil, toolErrf("%s: XML parse failed: %v", opfPath, err)
	}
	manifest := opfRoot.findChild("manifest")
	if manifest == nil {
		return "", nil, nil, toolErrf("%s: OPF missing manifest", opfPath)
	}
	var resources []manifestResource
	itemIDs := map[string]bool{}
	for _, item := range manifest.children {
		if item.name != "item" {
			continue
		}
		itemID, _ := item.getAttr("id")
		href, _ := item.getAttr("href")
		mediaType, ok := item.getAttr("media-type")
		if !ok {
			mediaType = "application/octet-stream"
		}
		if itemID == "" || href == "" {
			return "", nil, nil, toolErrf("%s: manifest item missing id or href", opfPath)
		}
		if itemIDs[itemID] {
			return "", nil, nil, toolErrf("%s: duplicate manifest id: %s", opfPath, itemID)
		}
		itemIDs[itemID] = true
		if pyIsExternalURI(href) {
			continue
		}
		archivePath, err := resolveRelativePath(opfPath, pyURLSplit(href).path)
		if err != nil {
			return "", nil, nil, err
		}
		resources = append(resources, manifestResource{itemID: itemID, href: href, mediaType: mediaType, archivePath: archivePath})
	}
	return opfPath, opfRoot, resources, nil
}

// ---- encryption ----

// inspectEncryption 复刻 encryption_path + inspect_encryption。
func inspectEncryption(names []string, files map[string]bool, current func(string) ([]byte, error)) (string, []encryptionRecord, error) {
	encPath := ""
	for _, name := range names {
		if strings.EqualFold(name, "meta-inf/encryption.xml") {
			encPath = name
			break
		}
	}
	if encPath == "" {
		return "", nil, nil
	}
	data, err := current(encPath)
	if err != nil {
		return "", nil, toolErrf("%v", err)
	}
	root, err := parseXMLTree(data)
	if err != nil {
		return "", nil, toolErrf("%s: XML parse failed: %v", encPath, err)
	}
	var records []encryptionRecord
	for _, elem := range iterAll(root) {
		if elem.name != "EncryptedData" {
			continue
		}
		algorithm := ""
		for _, d := range iterAll(elem) {
			if d.name == "EncryptionMethod" {
				algorithm, _ = d.getAttr("Algorithm")
				break
			}
		}
		for _, d := range iterAll(elem) {
			if d.name != "CipherReference" {
				continue
			}
			uri, _ := d.getAttr("URI")
			if uri == "" || pyIsExternalURI(uri) {
				return "", nil, toolErrf("%s: unsupported encryption URI: %s", encPath, pyRepr(uri))
			}
			parts := pyURLSplit(uri)
			target, err := resolveRootPath(parts.path)
			if err != nil {
				return "", nil, err
			}
			records = append(records, encryptionRecord{uri: uri, algorithm: algorithm, archivePath: target})
		}
	}
	return encPath, records, nil
}

// validateEncryption 逐行复刻 validate_encryption。
func validateEncryption(records []encryptionRecord, resources []manifestResource, files map[string]bool, rep *legacyRewriteReport) error {
	if len(records) == 0 {
		return nil
	}
	resourceByPath := map[string]manifestResource{}
	for _, r := range resources {
		resourceByPath[r.archivePath] = r // Python dict comprehension：后者覆盖
	}
	var unsupported []encryptionRecord
	for _, rec := range records {
		if !files[rec.archivePath] {
			rep.RemovedStaleEncryptionResources++
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("remove stale encryption reference with missing target: %s", rec.uri))
			continue
		}
		resource, ok := resourceByPath[rec.archivePath]
		if !ok || classifyResource(resource) != "Fonts" {
			unsupported = append(unsupported, rec)
			continue
		}
		if !fontObfuscationAlgorithms[rec.algorithm] {
			unsupported = append(unsupported, rec)
		}
	}
	if len(unsupported) > 0 {
		uris := make([]string, 0, 3)
		for i, rec := range unsupported {
			if i == 3 {
				break
			}
			uris = append(uris, rec.uri)
		}
		return toolErrf("DRM or unsupported encrypted resources detected; this tool only deobfuscates filenames "+
			"and allows standard EPUB font obfuscation. Refusing to rewrite: %s", strings.Join(uris, ", "))
	}
	rep.FontObfuscationResources = len(records) - rep.RemovedStaleEncryptionResources
	return nil
}

// ---- 分类与命名 ----

// classifyResource 逐行复刻 classify_resource。
func classifyResource(resource manifestResource) string {
	mediaType := strings.ToLower(resource.mediaType)
	ext := strings.ToLower(pathExt(resource.archivePath))
	switch {
	case mediaType == "application/xhtml+xml" || ext == ".html" || ext == ".htm" || ext == ".xhtml":
		return "Text"
	case mediaType == "text/css" || ext == ".css":
		return "Styles"
	case strings.HasPrefix(mediaType, "image/") || imageExtensions[ext]:
		return "Images"
	case strings.Contains(mediaType, "font") || fontExtensions[ext]:
		return "Fonts"
	case strings.HasPrefix(mediaType, "audio/") || audioExtensions[ext]:
		return "Audio"
	case strings.HasPrefix(mediaType, "video/") || videoExtensions[ext]:
		return "Video"
	case mediaType == "application/x-dtbncx+xml" || ext == ".ncx":
		return ""
	}
	return "Misc"
}

// deobfuscatedBasename 逐行复刻 deobfuscated_basename。
func deobfuscatedBasename(resource manifestResource) string {
	sourceName := pyBasename(resource.archivePath)
	_, sourceExt := pySplitExt(sourceName)
	itemName := resource.itemID
	itemStem, itemExt := pySplitExt(itemName)
	if strings.EqualFold(itemExt, sourceExt) {
		itemName = itemStem
	}

	slim := false
	if stem, ok := cutSlimSuffix(itemName); ok {
		slim = true
		itemName = stem
	} else if _, ok := cutSlimSuffix(pathStem(sourceName)); ok {
		slim = true
	}

	stem := sanitizeFilenameComponent(itemName, resource.itemID)
	suffix := ""
	if slim {
		suffix = "~slim"
	}
	return stem + suffix + strings.ToLower(sourceExt)
}

// cutSlimSuffix 复刻正则 (?:[~_-]?slim)$（大小写不敏感）的匹配与剥离。
func cutSlimSuffix(name string) (string, bool) {
	if len(name) < 4 || !strings.EqualFold(name[len(name)-4:], "slim") {
		return name, false
	}
	rest := name[:len(name)-4]
	if len(rest) > 0 {
		switch rest[len(rest)-1] {
		case '~', '-', '_':
			rest = rest[:len(rest)-1]
		}
	}
	return rest, true
}

// sanitizeFilenameComponent 逐行复刻 sanitize_filename_component。
func sanitizeFilenameComponent(value, fallbackSeed string) string {
	decoded := pyUnquote(value)
	var b strings.Builder
	prevInvalid := false
	for _, r := range decoded {
		if invalidFilenameChar(r) {
			if !prevInvalid {
				b.WriteByte('-')
				prevInvalid = true
			}
			continue
		}
		prevInvalid = false
		b.WriteRune(r)
	}
	sanitized := strings.Trim(b.String(), " .")
	sanitized = collapseHyphens(sanitized)
	if sanitized != "" {
		return sanitized
	}
	sum := sha256Hex12(fallbackSeed)
	return "resource-" + sum
}

func collapseHyphens(s string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		if r == '-' {
			if prevHyphen {
				continue
			}
			prevHyphen = true
		} else {
			prevHyphen = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

func suffixPath(p string, index int) string {
	stem, ext := pySplitExt(p)
	return fmt.Sprintf("%s-%d%s", stem, index, ext)
}

// allocatePath 逐行复刻 allocate_path。
func allocatePath(preferred string, used map[string]bool) (string, error) {
	candidate, err := validateArchivePath(preferred, "output resource")
	if err != nil {
		return "", err
	}
	index := 2
	for used[candidate] {
		candidate = suffixPath(preferred, index)
		index++
	}
	used[candidate] = true
	return candidate, nil
}

// buildPathMap 逐行复刻 build_path_map。
func buildPathMap(resources []manifestResource, files map[string]bool, opfPath, op string, rep *legacyRewriteReport) (map[string]string, error) {
	var order []string
	sourceResources := map[string]manifestResource{}
	for _, r := range resources {
		if !files[r.archivePath] {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("manifest href does not resolve: %s", r.href))
			continue
		}
		if _, ok := sourceResources[r.archivePath]; !ok {
			order = append(order, r.archivePath)
			sourceResources[r.archivePath] = r
		}
	}
	rep.ManifestResources = len(order)

	managed := map[string]bool{}
	for k := range sourceResources {
		managed[k] = true
	}
	used := map[string]bool{}
	for name := range files {
		if managed[name] || name == "mimetype" {
			continue
		}
		used[name] = true
	}
	opfDir := pyDirname(opfPath)
	pathMap := map[string]string{}
	for _, source := range order {
		resource := sourceResources[source]
		folder := classifyResource(resource)
		basename := pyBasename(source)
		if op == "deobfuscate-filenames" {
			basename = deobfuscatedBasename(resource)
		}
		preferred := pyJoin(opfDir, basename)
		if folder != "" {
			preferred = pyJoin(opfDir, folder, basename)
		}
		target, err := allocatePath(preferred, used)
		if err != nil {
			return nil, err
		}
		pathMap[source] = target
		if target == source {
			continue
		}
		rep.MovedResources++
		if pyBasename(target) != pyBasename(source) {
			rep.RenamedResources++
		}
		rep.Mappings = append(rep.Mappings, legacyMapping{From: source, To: target})
	}
	return pathMap, nil
}

// ---- transform_files ----

// transformContent 逐行复刻 transform_files，产出 editset.Edit：
//   - OPF / encryption.xml：ET 兼容重写（xmlmini）；
//   - CSS / 标记类：decode_text → 正则语义重写 → 原编码回编；
//   - 其余字节透传（不产生编辑，zipfs 原样搬运）；
//   - 改名 = 新建 entry（携带重写后的完整内容）+ 删除旧 entry；
//   - mimetype：Python 总是重写为规范内容并以 STORED 写出。
func transformContent(b *book.Book, names []string, files map[string]bool, opfPath string, opfRoot *xmlElem, encPath string, pathMap map[string]string, rep *legacyRewriteReport) ([]editset.Edit, []editset.Edit, []editset.Edit, error) {
	rw := &refRewriter{pathMap: pathMap, files: files, warnings: &rep.Warnings}
	transformed := map[string]bool{}
	var creates, deletes, replaces []editset.Edit

	const canonicalMimetype = "application/epub+zip"
	if files["mimetype"] {
		// Python write_epub 总是重写 mimetype 为规范内容并 STORED：
		// 用整段替换编辑强制 book 把它标记为已改（WriteTo 随之走 STORED）。
		cur, cerr := b.Current("mimetype")
		if cerr != nil {
			return nil, nil, nil, toolErrf("%v", cerr)
		}
		replaces = append(replaces, editset.Replace("mimetype", 0, int64(len(cur)), []byte(canonicalMimetype)))
	} else {
		creates = append(creates, editset.Replace("mimetype", 0, 0, []byte(canonicalMimetype)))
	}

	for _, oldPath := range names {
		if oldPath == "mimetype" {
			continue
		}
		newPath := oldPath
		if mapped, ok := pathMap[oldPath]; ok {
			newPath = mapped
		}
		currentBytes, err := b.Current(oldPath)
		if err != nil {
			return nil, nil, nil, toolErrf("%v", err)
		}
		updated := currentBytes
		drop := false
		switch {
		case oldPath == opfPath:
			updated, err = rewriteOPF(opfRoot, opfPath, rw)
			if err != nil {
				return nil, nil, nil, err
			}
		case encPath != "" && oldPath == encPath:
			out, keep, rerr := rewriteEncryptionXML(currentBytes, encPath, files, pathMap)
			if rerr != nil {
				return nil, nil, nil, rerr
			}
			if !keep {
				drop = true
			} else {
				updated = out
			}
		default:
			ext := strings.ToLower(pathExt(oldPath))
			if ext == ".css" || markupExtensions[ext] {
				text, enc, derr := decodeText(currentBytes, oldPath)
				if derr != nil {
					return nil, nil, nil, derr
				}
				var rewritten string
				if ext == ".css" {
					rewritten = rewriteCSSReferences(text, oldPath, newPath, rw)
				} else {
					rewritten = rewriteMarkupReferences(text, oldPath, newPath, rw)
				}
				updated, err = encodeText(rewritten, enc)
				if err != nil {
					return nil, nil, nil, err
				}
			}
		}
		if !bytes.Equal(updated, currentBytes) {
			rep.RewrittenFiles++
		}
		if transformed[newPath] {
			return nil, nil, nil, toolErrf("output path collision: %s", newPath)
		}
		transformed[newPath] = true
		if drop {
			deletes = append(deletes, editset.Delete(oldPath))
			continue
		}
		if newPath != oldPath {
			creates = append(creates, editset.Replace(newPath, 0, 0, updated))
			deletes = append(deletes, editset.Delete(oldPath))
		} else if !bytes.Equal(updated, currentBytes) {
			replaces = append(replaces, editset.Replace(oldPath, 0, int64(len(currentBytes)), updated))
		}
	}
	return creates, deletes, replaces, nil
}

// rewriteOPF 逐行复刻 rewrite_opf：manifest item 的 href 直接按 path_map
// 改写；其余元素的 href/src 走 rewrite_uri。最后按 ET 规则序列化。
func rewriteOPF(opfRoot *xmlElem, opfPath string, rw *refRewriter) ([]byte, error) {
	manifest := opfRoot.findChild("manifest")
	if manifest == nil {
		return nil, toolErrf("%s: OPF missing manifest", opfPath)
	}
	for _, item := range manifest.children {
		if item.name != "item" {
			continue
		}
		href, _ := item.getAttr("href")
		if href == "" || pyIsExternalURI(href) {
			continue
		}
		parts := pyURLSplit(href)
		oldTarget, err := resolveRelativePath(opfPath, parts.path)
		if err != nil {
			return nil, err
		}
		if target, ok := rw.pathMap[oldTarget]; ok && target != "" {
			item.setAttr("", "href", pyURLUnsplitPath(relativeURI(opfPath, target), parts.query, parts.fragment))
		}
	}
	for _, elem := range iterAll(opfRoot) {
		if elem.name == "item" {
			continue
		}
		for _, attrName := range []string{"href", "src"} {
			if uri, ok := elem.getAttr(attrName); ok && uri != "" {
				elem.setAttr("", attrName, rw.rewriteURI(uri, opfPath, opfPath))
			}
		}
	}
	return etreeToBytes(opfRoot), nil
}

// rewriteEncryptionXML 逐行复刻 rewrite_encryption_xml：更新存活
// CipherReference 的 URI、删除指向缺失目标的引用、清掉空 EncryptedData；
// 全部清空时返回 keep=false（entry 删除）。
func rewriteEncryptionXML(data []byte, path string, files map[string]bool, pathMap map[string]string) ([]byte, bool, error) {
	root, err := parseXMLTree(data)
	if err != nil {
		return nil, false, toolErrf("%s: XML parse failed: %v", path, err)
	}
	parents := map[*xmlElem]*xmlElem{}
	for _, e := range iterAll(root) {
		for _, c := range e.children {
			parents[c] = e
		}
	}
	for _, elem := range iterAll(root) {
		if elem.name != "CipherReference" {
			continue
		}
		uri, _ := elem.getAttr("URI")
		if uri == "" {
			continue
		}
		parts := pyURLSplit(uri)
		oldTarget, err := resolveRootPath(parts.path)
		if err != nil {
			return nil, false, err
		}
		if !files[oldTarget] {
			if p := parents[elem]; p != nil {
				p.removeChild(elem)
			}
			continue
		}
		target := oldTarget
		if mapped, ok := pathMap[oldTarget]; ok {
			target = mapped
		}
		elem.setAttr("", "URI", pyURLUnsplitPath(pyQuote(target), parts.query, parts.fragment))
	}
	parents = map[*xmlElem]*xmlElem{}
	for _, e := range iterAll(root) {
		for _, c := range e.children {
			parents[c] = e
		}
	}
	for _, elem := range iterAll(root) {
		if elem.name != "EncryptedData" {
			continue
		}
		hasRef := false
		for _, d := range iterAll(elem) {
			if d.name == "CipherReference" {
				hasRef = true
				break
			}
		}
		if !hasRef {
			if p := parents[elem]; p != nil {
				p.removeChild(elem)
			}
		}
	}
	found := false
	for _, e := range iterAll(root) {
		if e.name == "EncryptedData" {
			found = true
			break
		}
	}
	if !found {
		return nil, false, nil
	}
	return etreeToBytes(root), true, nil
}

// ---- 引用重写（正则语义的扫描器实现） ----

// rewriteMarkupReferences 复刻 rewrite_markup_references 的三段流水：
// srcset → URI 属性 → CSS url()/@import。
func rewriteMarkupReferences(text, oldDocument, newDocument string, rw *refRewriter) string {
	text = rewriteSrcsetURLs(text, oldDocument, newDocument, rw)
	text = subNameQuoteURI(text, uriAttrNames, func(prefix, quote, uri string) string {
		return prefix + quote + rw.rewriteURI(uri, oldDocument, newDocument) + quote
	})
	return rewriteCSSReferences(text, oldDocument, newDocument, rw)
}

// rewriteCSSReferences 复刻 rewrite_css_references。
func rewriteCSSReferences(text, oldDocument, newDocument string, rw *refRewriter) string {
	text = subCSSURL(text, func(prefix, quote, uri, suffix string) string {
		return prefix + quote + rw.rewriteURI(uri, oldDocument, newDocument) + quote + suffix
	})
	return subCSSImport(text, func(prefix, quote, uri string) string {
		return prefix + quote + rw.rewriteURI(uri, oldDocument, newDocument) + quote
	})
}

// rewriteSrcsetURLs 复刻 rewrite_srcset_urls。
func rewriteSrcsetURLs(text, oldDocument, newDocument string, rw *refRewriter) string {
	return subNameQuoteURI(text, []string{"srcset"}, func(prefix, quote, uri string) string {
		var candidates []string
		for _, candidate := range splitSrcsetCandidates(uri) {
			parts := splitPyWhitespace(strings.TrimSpace(candidate))
			if len(parts) == 0 {
				continue
			}
			url := rw.rewriteURI(parts[0], oldDocument, newDocument)
			descriptor := strings.Join(parts[1:], " ")
			candidates = append(candidates, strings.TrimSpace(url+" "+descriptor))
		}
		return prefix + quote + strings.Join(candidates, ", ") + quote
	})
}

// splitSrcsetCandidates 逐行复刻 split_srcset_candidates。
func splitSrcsetCandidates(value string) []string {
	var candidates []string
	start := 0
	inURL := true
	for index, char := range value {
		if unicode.IsSpace(char) && strings.TrimSpace(value[start:index]) != "" {
			inURL = false
		} else if char == ',' {
			seg := strings.TrimSpace(value[start:index])
			currentURL := ""
			if seg != "" {
				if parts := splitPyWhitespace(seg); len(parts) > 0 {
					currentURL = parts[0]
				}
			}
			if inURL && strings.HasPrefix(strings.ToLower(currentURL), "data:") {
				continue
			}
			candidates = append(candidates, value[start:index])
			start = index + 1
			inURL = true
		}
	}
	candidates = append(candidates, value[start:])
	return candidates
}

// splitPyWhitespace 复刻 str.split()（按 Unicode 空白切分、去空段）。
func splitPyWhitespace(s string) []string {
	var out []string
	start := -1
	for i, r := range s {
		if unicode.IsSpace(r) {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

// ---- 正则扫描器（Python re 语义的手工实现；RE2 无反向引用） ----

type uriMatch struct {
	start, end int    // 完整匹配的字节区间
	prefix     string // prefix 组（到引号前）
	quote      byte   // 0 表示无引号
	uri        string
	suffix     string // 仅 CSS url() 使用（\s*\)）
}

// isWordRune 对齐 Python \w（字母、数字、下划线，Unicode 感知）。
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// wordBoundary 对齐 Python \b。
func wordBoundary(text string, i int) bool {
	before := false
	if i > 0 {
		r, _ := utf8.DecodeLastRuneInString(text[:i])
		before = isWordRune(r)
	}
	after := false
	if i < len(text) {
		r, _ := utf8.DecodeRuneInString(text[i:])
		after = isWordRune(r)
	}
	return before != after
}

func skipPySpace(text string, i int) int {
	for i < len(text) {
		r, size := utf8.DecodeRuneInString(text[i:])
		if !unicode.IsSpace(r) {
			break
		}
		i += size
	}
	return i
}

// subNameQuoteURI 复刻 `\b(?:name|…)\s*=\s*(["'])(.*?)\1` 的 re.sub 语义。
func subNameQuoteURI(text string, names []string, repl func(prefix, quote, uri string) string) string {
	var out strings.Builder
	last := 0
	for {
		m, ok := findNameQuoteMatch(text, last, names)
		if !ok {
			break
		}
		out.WriteString(text[last:m.start])
		out.WriteString(repl(m.prefix, string(m.quote), m.uri))
		last = m.end
	}
	out.WriteString(text[last:])
	return out.String()
}

func findNameQuoteMatch(text string, from int, names []string) (uriMatch, bool) {
	for i := from; i < len(text); {
		if wordBoundary(text, i) {
			for _, name := range names {
				if i+len(name) > len(text) || !strings.EqualFold(text[i:i+len(name)], name) {
					continue
				}
				j := skipPySpace(text, i+len(name))
				if j >= len(text) || text[j] != '=' {
					continue
				}
				j = skipPySpace(text, j+1)
				if j >= len(text) || (text[j] != '"' && text[j] != '\'') {
					continue
				}
				quote := text[j]
				uriStart := j + 1
				idx := strings.IndexByte(text[uriStart:], quote)
				if idx < 0 {
					continue
				}
				uriEnd := uriStart + idx
				return uriMatch{
					start: i, end: uriEnd + 1,
					prefix: text[i:j],
					quote:  quote,
					uri:    text[uriStart:uriEnd],
				}, true
			}
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		if size == 0 {
			break
		}
		i += size
	}
	return uriMatch{}, false
}

// subCSSURL 复刻 `\burl\(\s*(["']?)(.*?)\1\s*\)` 的 re.sub 语义
// （含引号分支失败后回退为无引号的回溯行为）。
func subCSSURL(text string, repl func(prefix, quote, uri, suffix string) string) string {
	var out strings.Builder
	last := 0
	for {
		m, ok := findURLMatch(text, last)
		if !ok {
			break
		}
		out.WriteString(text[last:m.start])
		q := ""
		if m.quote != 0 {
			q = string(m.quote)
		}
		out.WriteString(repl(m.prefix, q, m.uri, m.suffix))
		last = m.end
	}
	out.WriteString(text[last:])
	return out.String()
}

func findURLMatch(text string, from int) (uriMatch, bool) {
	for i := from; i < len(text); {
		if wordBoundary(text, i) && i+4 <= len(text) && strings.EqualFold(text[i:i+4], "url(") {
			j := skipPySpace(text, i+4)
			quote := byte(0)
			quoteStart := -1
			if j < len(text) && (text[j] == '"' || text[j] == '\'') {
				quote = text[j]
				quoteStart = j
				j++
			}
			uriStart := j
			if quote != 0 {
				p := j
				for p < len(text) {
					idx := strings.IndexByte(text[p:], quote)
					if idx < 0 {
						break
					}
					qPos := p + idx
					k := skipPySpace(text, qPos+1)
					if k < len(text) && text[k] == ')' {
						return uriMatch{
							start: i, end: k + 1,
							prefix: text[i:quoteStart],
							quote:  quote,
							uri:    text[uriStart:qPos],
							suffix: text[qPos+1 : k+1],
						}, true
					}
					p = qPos + 1
				}
				// 回溯：引号组视为空，uri 从引号字符处开始。
				uriStart = quoteStart
			}
			idx := strings.IndexByte(text[uriStart:], ')')
			if idx >= 0 {
				closePos := uriStart + idx
				wsStart := closePos
				for wsStart > uriStart {
					r, size := utf8.DecodeLastRuneInString(text[uriStart:wsStart])
					if !unicode.IsSpace(r) {
						break
					}
					wsStart -= size
				}
				return uriMatch{
					start: i, end: closePos + 1,
					prefix: text[i:uriStart],
					quote:  0,
					uri:    text[uriStart:wsStart],
					suffix: text[wsStart : closePos+1],
				}, true
			}
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		if size == 0 {
			break
		}
		i += size
	}
	return uriMatch{}, false
}

// subCSSImport 复刻 `@import\s+(["'])(.*?)\1` 的 re.sub 语义。
func subCSSImport(text string, repl func(prefix, quote, uri string) string) string {
	var out strings.Builder
	last := 0
	for {
		m, ok := findImportMatch(text, last)
		if !ok {
			break
		}
		out.WriteString(text[last:m.start])
		out.WriteString(repl(m.prefix, string(m.quote), m.uri))
		last = m.end
	}
	out.WriteString(text[last:])
	return out.String()
}

func findImportMatch(text string, from int) (uriMatch, bool) {
	for i := from; i+7 <= len(text); i++ {
		if strings.EqualFold(text[i:i+7], "@import") {
			j := skipPySpace(text, i+7)
			if j > i+7 && j < len(text) && (text[j] == '"' || text[j] == '\'') {
				quote := text[j]
				uriStart := j + 1
				idx := strings.IndexByte(text[uriStart:], quote)
				if idx >= 0 {
					uriEnd := uriStart + idx
					return uriMatch{
						start: i, end: uriEnd + 1,
						prefix: text[i:j],
						quote:  quote,
						uri:    text[uriStart:uriEnd],
					}, true
				}
			}
		}
	}
	return uriMatch{}, false
}

// bytesTrimNewline 去掉尾部单个换行（MarshalLegacy 为对齐 Python print 加的）。
func bytesTrimNewline(b []byte) []byte {
	return bytes.TrimSuffix(b, []byte("\n"))
}
