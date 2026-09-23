// Package structurenormalize 迁移 scripts/epub_structure_tool.py
// （capability id：epub.structure.normalize）：
//
//   - inspect：只读体检（manifest 资源数、encryption 审计）；
//   - format：目录归类（Text/Styles/Images/Fonts/Audio/Video/Misc，
//     ncx 留在 OPF 同级）并重写全部本地引用；
//   - deobfuscate：按 manifest id 生成可读文件名（deobfuscated_basename 规则）；
//   - normalize：两阶段 = 先 format 再 deobfuscate。
//     dry-run 同样生成完整内存候选供红线检查，只有 pipeline 决定是否落盘。
//
// 字节保真策略（parity 基准是 Python oracle 的最终输出字节）：
// XHTML 按真实标记区域重写；CSS 用 lossless token spans/editset 后按原编码回编；
// OPF / encryption.xml 在 Python 侧是 ElementTree 整体重写，这里用
// xmlmini.go 逐条复刻 ET 的解析与序列化规则，保证最终字节一致。
// 所有写入一律以 []editset.Edit 交给 book.Apply（SPEC §6.1 三段式），
// 未修改 entry 由 zipfs 原样透传（INV-1）。
package structurenormalize

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/redline"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/xhtml"
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
	// DryRun 标记预览；仍应用到内存 Book，使映射与红线候选一致。
	DryRun bool
	// Force 只是占位：输出文件冲突由 pipeline 层裁决，包内不处理。
	Force bool
}

// ---- 阶段报告（内部累加器；经 buildResult 映射为信封 facts） ----

// mapping 是一条改名映射；`facts.mappings` 与 `facts.stages[].mappings`
// 保持 {from,to} 形状，`epub redline --path-map` 直接读取。
type mapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// stageReport 是单个阶段（format / deobfuscate-filenames / inspect）的计数。
type stageReport struct {
	Operation                       string    `json:"operation"`
	OPF                             string    `json:"opf"`
	ManifestResources               int       `json:"manifest_resources"`
	MovedResources                  int       `json:"moved_resources"`
	RenamedResources                int       `json:"renamed_resources"`
	RewrittenFiles                  int       `json:"rewritten_files"`
	FontObfuscationResources        int       `json:"font_obfuscation_resources"`
	RemovedStaleEncryptionResources int       `json:"removed_stale_encryption_resources"`
	DryRun                          bool      `json:"dry_run"`
	Mappings                        []mapping `json:"mappings"`
	Warnings                        []string  `json:"warnings"`
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
	err      error
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
	if resolved, err := resolveRelativePath(newDocument, parts.path); err == nil && resolved == target {
		return uri
	}
	return pyURLUnsplitPath(relativeURI(newDocument, target), parts.query, parts.fragment)
}

// ---- Run（SPEC §6.1 三段式：扫描 → 应用 → 报告） ----

// Run 执行本 capability。禁止修改 b 之外的任何状态；落盘由 pipeline 的
// b.WriteTo 负责（INV-3）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	op, ok := p.Mode.pythonOperation()
	if !ok {
		return report.Result{}, fmt.Errorf("%w: unsupported mode %q", ErrStructureTool, string(p.Mode))
	}
	if p.Mode == ModeInspect {
		return runInspect(ctx, b, p)
	}
	if p.Mode == ModeNormalize {
		return runNormalize(ctx, b, p)
	}
	return runSingleStage(ctx, b, p, op)
}

// scanRewriteStage 复刻 analyze_epub（+ 非 dry-run 的 transform_files）：
// 只读 b，产出报告与 []editset.Edit；不落盘。
func scanRewriteStage(ctx context.Context, b *book.Book, op string, dryRun bool) (stageResult, error) {
	names := b.Names()
	files := make(map[string]bool, len(names))
	for _, n := range names {
		files[n] = true
	}
	current := func(name string) ([]byte, error) { return b.Current(name) }

	rep := stageReport{
		Operation: op,
		DryRun:    dryRun,
		Mappings:  []mapping{},
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
	// transform_files → editset.Edit。
	creates, deletes, replaces, err := transformContent(ctx, b, names, files, opfPath, opfRoot, encPath, pathMap, &rep)
	if err != nil {
		return stageResult{}, err
	}
	return stageResult{rep: rep, pathMap: pathMap, creates: creates, deletes: deletes, replaces: replaces}, nil
}

type stageResult struct {
	rep      stageReport
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

func runSingleStage(ctx context.Context, b *book.Book, p Params, op string) (report.Result, error) {
	st, err := scanRewriteStage(ctx, b, op, p.DryRun)
	if err != nil {
		return report.Result{}, err
	}
	if err := applyStage(b, st); err != nil {
		return report.Result{}, err
	}
	return buildResult(p, op, []stageReport{st.rep}, renamesFromStages(st.rep)), nil
}

// runNormalize 的两阶段始终在内存中完成，预览不产生部分执行状态。
func runNormalize(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	st1, err := scanRewriteStage(ctx, b, "format", p.DryRun)
	if err != nil {
		return report.Result{}, err
	}
	if err := applyStage(b, st1); err != nil {
		return report.Result{}, err
	}

	st2, err := scanRewriteStage(ctx, b, "deobfuscate-filenames", p.DryRun)
	if err != nil {
		return report.Result{}, err
	}
	if err := applyStage(b, st2); err != nil {
		return report.Result{}, err
	}
	return buildResult(p, "normalize", []stageReport{st1.rep, st2.rep}, renamesFromStages(st1.rep, st2.rep)), nil
}

func runInspect(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	st, err := scanRewriteStage(ctx, b, "inspect", false)
	if err != nil {
		return report.Result{}, err
	}
	return buildResult(p, "inspect", []stageReport{st.rep}, nil), nil
}

// buildResult 把阶段计数装配为统一信封的 Result 段。facts 键（camelCase）
// 对所有 mode 稳定：operation / mode / dryRun / opf / manifestResources /
// movedResources / renamedResources / rewrittenFiles /
// fontObfuscationResources / removedStaleEncryptionResources / mappings /
// warnings；多阶段（normalize）额外给出 stages[] 逐阶段明细。
// 输入/输出路径由 pipeline 信封的 input / output 段承担。
func buildResult(p Params, operation string, stages []stageReport, renames map[string]string) report.Result {
	main := stages[len(stages)-1]
	var allMappings []mapping
	var allWarnings []string
	for _, st := range stages {
		allMappings = append(allMappings, st.Mappings...)
		allWarnings = append(allWarnings, st.Warnings...)
	}
	facts := map[string]any{
		"operation":                       operation,
		"mode":                            string(p.Mode),
		"dryRun":                          p.DryRun,
		"opf":                             main.OPF,
		"manifestResources":               main.ManifestResources,
		"movedResources":                  sumInt(stages, func(s stageReport) int { return s.MovedResources }),
		"renamedResources":                sumInt(stages, func(s stageReport) int { return s.RenamedResources }),
		"rewrittenFiles":                  sumInt(stages, func(s stageReport) int { return s.RewrittenFiles }),
		"fontObfuscationResources":        sumInt(stages, func(s stageReport) int { return s.FontObfuscationResources }),
		"removedStaleEncryptionResources": sumInt(stages, func(s stageReport) int { return s.RemovedStaleEncryptionResources }),
		"mappings":                        nonNilMappings(allMappings),
		"warnings":                        nonNilStrings(allWarnings),
	}
	if len(stages) > 1 {
		facts["stages"] = stages
	}

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

	return report.Result{
		Capability: CapabilityID,
		Status:     report.StatusComplete,
		Facts:      facts,
		Findings:   findings,
		Events:     events,
		Renames:    renames,
	}
}

func sumInt(stages []stageReport, get func(stageReport) int) int {
	total := 0
	for _, st := range stages {
		total += get(st)
	}
	return total
}

func nonNilMappings(in []mapping) []mapping {
	if in == nil {
		return []mapping{}
	}
	return in
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// sha256Hex12 复刻 hashlib.sha256(seed).hexdigest()[:12]。
func sha256Hex12(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])[:12]
}

// renamesFromStages 把各阶段 mapping 链式展开成 Result.Renames
// （语义同 validate_text_invariance.add_path_mapping：先改既有映射中
// 目标为 source 的键，再登记 source→target；两阶段链式后 from 即原始名）。
func renamesFromStages(stages ...stageReport) map[string]string {
	renames := map[string]string{}
	for _, st := range stages {
		stageMap := make(map[string]string, len(st.Mappings))
		for _, m := range st.Mappings {
			stageMap[m.From] = m.To
		}
		renames = redline.ComposePathMaps(renames, stageMap)
	}
	if len(renames) == 0 {
		return nil
	}
	return renames
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
func validateEncryption(records []encryptionRecord, resources []manifestResource, files map[string]bool, rep *stageReport) error {
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
func buildPathMap(resources []manifestResource, files map[string]bool, opfPath, op string, rep *stageReport) (map[string]string, error) {
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
		rep.Mappings = append(rep.Mappings, mapping{From: source, To: target})
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
func transformContent(ctx context.Context, b *book.Book, names []string, files map[string]bool, opfPath string, opfRoot *xmlElem, encPath string, pathMap map[string]string, rep *stageReport) ([]editset.Edit, []editset.Edit, []editset.Edit, error) {
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
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
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
		if rw.err != nil {
			return nil, nil, nil, rw.err
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

// rewriteMarkupReferences 复刻 rewrite_markup_references 的三段流水
// （srcset → URI 属性 → CSS url()/@import），但只作用于真实标记区域
// （见 xhtml.ScanRegions）：属性重写与内联 style 的 url() 只发生在
// 标签内部，CSS 重写只发生在 <style> 元素内容，xml-stylesheet 处理指令
// 只改写 href 伪属性。字符数据里被实体转义的 `&lt;img src="…"/&gt;` 之类
// 正文、注释、CDATA、其它 PI 与 <script> 内容逐字节保留。Python 版对全文
// 做正则替换会改写作者正文，是已修复的缺陷。
//
// 扫描器遇到无法闭合的结构时会放弃文档剩余部分，此处必须转成告警：否则
// 改名后尾部引用会静默断链，而 anchors 红线只校验 id 存活、不校验 href
// 可解析，没有任何下游能兜住。
func rewriteMarkupReferences(text, oldDocument, newDocument string, rw *refRewriter) string {
	regions, stop := xhtml.ScanRegions(text)
	if stop != xhtml.ScanComplete {
		rw.warn("%s: markup scan stopped at byte offset %d (unterminated comment/CDATA/PI/declaration/tag or unclosed style/script); references after this offset left unchanged", oldDocument, stop)
	}
	if len(regions) == 0 {
		return text
	}
	var out strings.Builder
	out.Grow(len(text))
	last := 0
	for _, r := range regions {
		out.WriteString(text[last:r.Start])
		segment := text[r.Start:r.End]
		switch r.Kind {
		case xhtml.RegionTag:
			segment = rewriteTagReferences(segment, oldDocument, newDocument, rw)
		case xhtml.RegionStyle:
			if strings.Contains(segment, "&") {
				segment = rewriteCSSReferencesWithEntityMap(segment, oldDocument, newDocument, 0, rw)
			} else {
				segment = rewriteCSSReferences(segment, oldDocument, newDocument, rw)
			}
		case xhtml.RegionStylesheetPI:
			segment = rewriteStylesheetPIReference(segment, oldDocument, newDocument, rw)
		}
		out.WriteString(segment)
		last = r.End
	}
	out.WriteString(text[last:])
	return out.String()
}

// rewriteTagReferences 在单个标签的字节内重写 srcset、URI 属性与内联
// style 属性里的 url()/@import。
func rewriteTagReferences(tag, oldDocument, newDocument string, rw *refRewriter) string {
	tag = rewriteSrcsetURLs(tag, oldDocument, newDocument, rw)
	tag = rewriteQuotedTagAttrs(tag, uriAttrNames, func(_ string, quote byte, raw string) string {
		uri := raw
		if strings.Contains(raw, "&") {
			decoded, _, err := xhtml.DecodeAttrWithMap(raw)
			if err != nil {
				rw.err = toolErrf("%s: URI attribute entity decode: %v", oldDocument, err)
				return raw
			}
			uri = decoded
		}
		updated := rw.rewriteURI(uri, oldDocument, newDocument)
		if updated == uri {
			return raw
		}
		return attrEscapeFor(quote, updated)
	})
	return rewriteInlineStyleReferences(tag, oldDocument, newDocument, rw)
}

// rewriteInlineStyleReferences 只在 style="…" 属性值内部做 CSS url()/@import
// 重写。整段标签跑 CSS 重写会连 title=""、alt="" 这类读者可见文本一起改
// （`<div title="url(a.png)">`），那是正文损坏；而内联 style 的 url() 是真
// 标记，资源搬家后必须跟着改，不能整体放弃。
func rewriteInlineStyleReferences(tag, oldDocument, newDocument string, rw *refRewriter) string {
	return rewriteQuotedTagAttrs(tag, []string{"style"}, func(_ string, quote byte, value string) string {
		if strings.Contains(value, "&") {
			return rewriteCSSReferencesWithEntityMap(value, oldDocument, newDocument, quote, rw)
		}
		return rewriteCSSReferences(value, oldDocument, newDocument, rw)
	})
}

// rewriteStylesheetPIReference 只重写 <?xml-stylesheet …?> 的 href 伪属性。
// PI 不是标签：type/media/title 伪属性与 CSS url() 语法都不参与重写。
func rewriteStylesheetPIReference(pi, oldDocument, newDocument string, rw *refRewriter) string {
	return subNameQuoteURI(pi, []string{"href"}, func(prefix, quote, uri string) string {
		return prefix + quote + rw.rewriteURI(uri, oldDocument, newDocument) + quote
	})
}

// rewriteCSSReferences uses lossless token spans, excluding comments and
// non-resource strings. An uncertain local escape refuses the candidate.
func rewriteCSSReferences(text, oldDocument, newDocument string, rw *refRewriter) string {
	refs, err := css.ScanReferences([]byte(text))
	if err != nil {
		rw.err = toolErrf("%s: CSS reference scan: %v", oldDocument, err)
		return text
	}
	var edits []editset.Edit
	for _, ref := range refs {
		if ref.DataURL || pyIsExternalURI(ref.Value) {
			continue
		}
		if strings.Contains(ref.Value, `\`) {
			rw.err = toolErrf("%s: escaped local CSS URL requires explicit repair: %s", oldDocument, ref.Value)
			return text
		}
		updated := rw.rewriteURI(ref.Value, oldDocument, newDocument)
		if updated != ref.Value {
			edits = append(edits, editset.Replace(oldDocument, int64(ref.ValueSpan.Start), int64(ref.ValueSpan.Len()), []byte(updated)))
		}
	}
	updated, err := editset.Apply(oldDocument, []byte(text), edits)
	if err != nil {
		rw.err = err
		return text
	}
	return string(updated)
}

func rewriteCSSReferencesWithEntityMap(raw, oldDocument, newDocument string, quote byte, rw *refRewriter) string {
	decoded, rawOff, err := xhtml.DecodeAttrWithMap(raw)
	if err != nil {
		rw.err = toolErrf("%s: CSS entity decode: %v", oldDocument, err)
		return raw
	}
	edits, err := css.ReferenceEdits(oldDocument, []byte(decoded), func(uri string) string {
		return rw.rewriteURI(uri, oldDocument, newDocument)
	})
	if err != nil {
		rw.err = toolErrf("%s: CSS reference scan: %v", oldDocument, err)
		return raw
	}
	if len(edits) == 0 {
		return raw
	}
	mapped := make([]editset.Edit, 0, len(edits))
	for _, edit := range edits {
		start := int(edit.Offset)
		end := start + int(edit.Length)
		if start < 0 || end < start || end >= len(rawOff) {
			rw.err = fmt.Errorf("%s: CSS reference span cannot be mapped to source text", oldDocument)
			return raw
		}
		replacement := string(edit.Replacement)
		if quote == 0 {
			replacement = pypath.EscapeText(replacement)
		} else {
			replacement = attrEscapeFor(quote, replacement)
		}
		rawStart, rawEnd := rawOff[start], rawOff[end]
		mapped = append(mapped, editset.Replace(oldDocument, int64(rawStart), int64(rawEnd-rawStart), []byte(replacement)))
	}
	updated, err := editset.Apply(oldDocument, []byte(raw), mapped)
	if err != nil {
		rw.err = err
		return raw
	}
	return string(updated)
}

func attrEscapeFor(quote byte, value string) string {
	if quote == '\'' {
		return singleQuoteAttrEscaper.Replace(value)
	}
	return attribEscaper.Replace(value)
}

// rewriteSrcsetURLs 复刻 rewrite_srcset_urls。
func rewriteSrcsetURLs(text, oldDocument, newDocument string, rw *refRewriter) string {
	return rewriteQuotedTagAttrs(text, []string{"srcset"}, func(_ string, _ byte, uri string) string {
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
		return strings.Join(candidates, ", ")
	})
}

func rewriteQuotedTagAttrs(tag string, names []string, rewrite func(name string, quote byte, value string) string) string {
	_, _, closing := xhtml.TagParts(tag)
	if closing {
		return tag
	}
	attrs, ok := xhtml.TagAttrs(tag)
	if !ok {
		return tag
	}
	var out strings.Builder
	last := 0
	changed := false
	for _, attr := range attrs {
		if attr.Quote == 0 || !hasAttrName(names, attr.Name) {
			continue
		}
		value := tag[attr.ValueSpan.Start:attr.ValueSpan.End]
		updated := rewrite(attr.Name, attr.Quote, value)
		if updated == value {
			continue
		}
		out.WriteString(tag[last:attr.ValueSpan.Start])
		out.WriteString(updated)
		last = attr.ValueSpan.End
		changed = true
	}
	if !changed {
		return tag
	}
	out.WriteString(tag[last:])
	return out.String()
}

func hasAttrName(names []string, candidate string) bool {
	for _, name := range names {
		if strings.EqualFold(candidate, name) {
			return true
		}
	}
	return false
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
