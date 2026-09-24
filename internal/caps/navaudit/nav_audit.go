// Package navaudit 移植 epub.package.nav.audit（scripts/epub_preflight_harness.py
// 与 scripts/epub_ai/ 的检查家族）。它是只读 validator：不产生 edits。
//
// 输出统一信封：findings 与 preflight harness 的检查项一一对应，附加的
// 结构化 facts（findingsByLevel / recommendedSkills / toolAvailability /
// actionableFindings）见 summary.go。
package navaudit

import (
	"context"
	"fmt"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/extern"
	"github.com/liyafly/epub-handbook/internal/report"
	cssscan "github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// auditFinding 是检查项的内部累积形态：level, message[, path[, kind]]。
type auditFinding struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

// Params 是 nav.audit 的参数。
type Params struct {
	// Report 选报告族：preflight（默认）或 layout-audit（AI harness 家族，
	// 无 spine 特判）。
	Report string // "preflight" | "layout-audit"
}

type inspector struct {
	b           *book.Book
	pkg         *opf.Package
	opfPath     string
	mode        string
	summary     *orderedSummary
	findings    []auditFinding
	skills      []string
	skillLv     map[string]string
	commands    []string
	tools       *orderedTools
	textChars   int
	imageRefs   int
	layoutAudit bool
	// lookPath 是外部工具探测器（默认 externToolProbe）。
	lookPath toolProbe
}

// orderedSummary 是 summary 的内部累积形态（键序固定）。
type orderedSummary struct {
	ZipEntries          int            `json:"zip_entries"`
	OPF                 string         `json:"opf,omitempty"`
	ManifestItems       int            `json:"manifest_items"`
	SpineItems          int            `json:"spine_items"`
	MediaCounts         map[string]int `json:"media_counts"`
	ObfuscatedFilenames int            `json:"obfuscated_filenames,omitempty"`
	PackageVersion      string         `json:"package_version,omitempty"`
	Language            string         `json:"language,omitempty"`
	HasOPF              bool           `json:"-"`
}

// orderedTools 是 tool_availability 的有序包装。
type orderedTools struct {
	Keys   []string
	Values map[string]bool
}

func (t *orderedTools) add(name string, ok bool) {
	if !t.Values[name] && ok {
		t.Values[name] = true
	}
}

// toolProbe 报告某个外部工具是否可用。默认实现走 internal/extern
// （INV-4：caps 不得 import os/exec）；测试注入桩，使 golden 与本机 PATH 无关。
type toolProbe func(name string) bool

// externToolProbe 是生产实现：extern.LookPath，工具缺失时返回 false。
func externToolProbe(name string) bool {
	ok, _ := extern.LookPath(name)
	return ok
}

// Run 执行 nav.audit（只读）。外部工具探测走 internal/extern。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	return run(ctx, b, p, externToolProbe)
}

// run 是 Run 的可注入内核：lookPath 固定外部工具探测结果，
// 供 golden 测试摆脱开发机 PATH（`brew install epubcheck` 不应让测试变红）。
func run(ctx context.Context, b *book.Book, p Params, lookPath toolProbe) (report.Result, error) {
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	if lookPath == nil {
		lookPath = externToolProbe
	}
	ins := &inspector{
		b:           b,
		mode:        "cleanup",
		layoutAudit: p.Report == "layout-audit",
		summary:     &orderedSummary{MediaCounts: map[string]int{"xhtml": 0, "css": 0, "images": 0, "fonts": 0, "other": 0}},
		skillLv:     map[string]string{},
		tools:       &orderedTools{Values: map[string]bool{}},
		lookPath:    lookPath,
	}
	ins.inspect(ctx)
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	actionableFindings := ins.detectActionable(ctx)
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	ins.addActionableFindings(actionableFindings)

	res := report.Result{
		Capability: "epub.package.nav.audit",
		Status:     report.StatusComplete,
		Facts: map[string]any{
			"input_kind": "existing-epub",
			"summary":    ins.summaryFields(),
		},
	}
	errorCount, warnCount := 0, 0
	for _, f := range ins.findings {
		switch f.Level {
		case "error":
			errorCount++
		case "warn":
			warnCount++
		}
		res.Findings = append(res.Findings, report.Finding{
			Level: f.Level, ID: "audit." + fmt.Sprint(len(res.Findings)),
			Title: f.Message, Detail: f.Kind, Location: f.Path,
		})
	}
	status := "pass"
	if errorCount > 0 {
		status = "fail"
		res.Status = report.StatusFailed
	} else if warnCount > 0 {
		status = "warn"
	}
	// spine 特判（仅 preflight 族）：spine 为空追加一条 error finding。
	if ins.summary.SpineItems == 0 && !ins.layoutAudit {
		res.Findings = append(res.Findings, report.Finding{
			Level: "error", ID: "audit." + fmt.Sprint(len(res.Findings)),
			Title: "OPF spine is missing or empty",
		})
		status = "fail"
		res.Status = report.StatusFailed
	}
	res.Facts["auditStatus"] = status
	res.Facts["findingsByLevel"] = countFindingsByLevel(res.Findings)
	res.Facts["recommendedSkills"] = ins.orderedSkills()
	res.Facts["toolAvailability"] = ins.toolAvailability()
	res.Facts["actionableFindings"] = actionableFindings
	res.NextCommands = ins.nextCommands()
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	return res, nil
}

func (ins *inspector) summaryFields() map[string]any {
	out := map[string]any{
		"zip_entries":    ins.summary.ZipEntries,
		"manifest_items": ins.summary.ManifestItems,
		"spine_items":    ins.summary.SpineItems,
		"media_counts":   ins.summary.MediaCounts,
	}
	if ins.summary.HasOPF {
		out["opf"] = ins.summary.OPF
	}
	if ins.summary.ObfuscatedFilenames > 0 {
		out["obfuscated_filenames"] = ins.summary.ObfuscatedFilenames
	}
	if ins.summary.PackageVersion != "" {
		out["package_version"] = ins.summary.PackageVersion
	}
	if ins.summary.Language != "" {
		out["language"] = ins.summary.Language
	}
	return out
}

func (ins *inspector) nextCommands() []string {
	// commands 即新信封的 nextCommands；只暴露当前 Go CLI 的执行面。
	// 用 make(..., 0, n) 而非 append(nil, ...)：空列表必须序列化为 []，不是 null。
	out := make([]string, 0, len(ins.commands))
	return append(out, ins.commands...)
}

func (ins *inspector) addFinding(level, message, path, kind string) {
	f := auditFinding{Level: level, Message: message}
	if path != "" {
		f.Path = path
	}
	if kind != "" {
		f.Kind = kind
	}
	ins.findings = append(ins.findings, f)
}

// addSkill 记录推荐技能（$ 前缀，去重，级别只升不降）。
func (ins *inspector) addSkill(name, level string) {
	key := "$" + name
	if !slicesContains(ins.skills, key) {
		ins.skills = append(ins.skills, key)
		ins.skillLv[key] = level
		return
	}
	if severity(level) < severity(ins.skillLv[key]) {
		ins.skillLv[key] = level
	}
}

func (ins *inspector) addCommand(cmd string) {
	if !slicesContains(ins.commands, cmd) {
		ins.commands = append(ins.commands, cmd)
	}
}

func severity(level string) int {
	switch level {
	case "error":
		return 0
	case "warn":
		return 1
	default:
		return 2
	}
}

func slicesContains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// inspect 是 inspect_path(path, "cleanup") 对 EPUB 输入的主流程。
func (ins *inspector) inspect(ctx context.Context) {
	q := report.ShellQuote(ins.b.InputPath())
	// 旧 preflight / AI / refinement 入口已合并为 Go capability。保留原有
	// 推荐顺序，但让报告中的每一项都能由当前 `epub` CLI 直接执行。
	ins.addCommand("epub run epub.package.nav.audit --input " + q + " --json")
	ins.addCommand("epub run epub.layout.audit --input " + q + " --json")
	ins.addCommand("epub run epub.notes.popup.normalize --input " + q + " --dry-run --json")
	ins.addCommand("epub redline --check all <before.epub> <after.epub>")

	ins.summary.ZipEntries = len(ins.b.Names())

	// 容器与 OPF（book.Open 已保证 container/OPF 可解析）。
	containerRaw, err := ins.b.Current("META-INF/container.xml")
	if err == nil {
		if opfPath, err2 := opf.FindOPFPath(containerRaw); err2 == nil {
			if raw, err3 := ins.b.Current(opfPath); err3 == nil {
				if pkg, err4 := opf.Parse(opfPath, raw); err4 == nil {
					ins.opfPath = opfPath
					ins.pkg = pkg
				}
			}
		}
	}
	if hasEntry(ins.b, "META-INF/encryption.xml") {
		ins.addFinding("error",
			"EPUB has META-INF/encryption.xml; stop unless this is confirmed font obfuscation and explicitly allowed",
			"META-INF/encryption.xml", "drm")
	}
	if ins.pkg != nil {
		ins.summary.HasOPF = true
		ins.summary.OPF = ins.opfPath
		ins.inspectOPF(ctx)
	}
	ins.addCommand("epub capabilities --json")
	// preflight 特有：epubcheck 可用性（经 extern；本机无 → 注释行占位）。
	ins.tools.Keys = append(ins.tools.Keys, "epubcheck")
	if ins.lookPath("epubcheck") {
		ins.tools.Values["epubcheck"] = true
		ins.addCommand("epubcheck " + q)
	} else {
		ins.tools.Values["epubcheck"] = false
		ins.addCommand("# EPUBCheck runs in GitHub Actions; local preflight skips it when unavailable.")
	}
	ins.applyWorkflowMode()
}

func (ins *inspector) inspectOPF(ctx context.Context) {
	pkg := ins.pkg
	q := report.ShellQuote(ins.b.InputPath())
	for _, item := range pkg.Manifest {
		if ctx.Err() != nil {
			return
		}
		media := item.MediaType
		hrefLower := strings.ToLower(item.Href)
		switch {
		case media == "application/xhtml+xml" || strings.HasSuffix(hrefLower, ".xhtml"):
			ins.summary.MediaCounts["xhtml"]++
		case media == "text/css" || strings.HasSuffix(hrefLower, ".css"):
			ins.summary.MediaCounts["css"]++
		case strings.HasPrefix(media, "image/") || hasSuffixAny(hrefLower, ".jpg", ".jpeg", ".png", ".webp", ".svg", ".gif", ".tif", ".tiff"):
			ins.summary.MediaCounts["images"]++
		case isFontMedia(media) || hasSuffixAny(hrefLower, ".otf", ".ttf", ".woff", ".woff2"):
			ins.summary.MediaCounts["fonts"]++
		default:
			ins.summary.MediaCounts["other"]++
		}
	}
	ins.summary.ManifestItems = len(pkg.Manifest)
	ins.summary.SpineItems = len(pkg.Spine)

	// 文件名混淆。
	obfuscated := 0
	for _, item := range pkg.Manifest {
		if ctx.Err() != nil {
			return
		}
		if item.Href == "" {
			continue
		}
		base := item.Href
		if i := strings.LastIndexByte(base, '/'); i >= 0 {
			base = base[i+1:]
		}
		if base != "" && specialCharRe.MatchString(base) {
			obfuscated++
		}
	}
	if obfuscated > 0 {
		ins.summary.ObfuscatedFilenames = obfuscated
		ins.addFinding("warn",
			"Manifest filenames contain decoded special characters; run structure normalization before richer cleanup",
			"", "filename-obfuscation")
		ins.addSkill("epub-cleanup", "warn")
		ins.addCommand("epub run epub.structure.normalize --input " + q +
			" --output work/after/step-0-normalized.epub --dry-run --json")
	}

	// 版本与迁移。
	version := pkg.Version
	if version != "" {
		ins.summary.PackageVersion = version
	}
	if version != "" && !strings.HasPrefix(version, "3") {
		ins.addFinding("warn", "EPUB 2 package should be migrated to EPUB 3 before richer cleanup/features", "", "epub3-migration")
		ins.addSkill("epub-cleanup", "warn")
		ins.addSkill("epub-audit", "warn")
		ins.addCommand("epub run epub.package.migrate.epub3 --input " + q + " --dry-run --json")
		ins.addCommand("epub run epub.package.migrate.epub3 --input " + q +
			" --output work/after/step-1-epub3.epub --json")
	}

	// 语言。
	if langs := pkg.Metadata["language"]; len(langs) > 0 && strings.TrimSpace(langs[0]) != "" {
		ins.summary.Language = collapseSpace(langs[0])
		if strings.HasPrefix(strings.ToLower(ins.summary.Language), "en") {
			ins.addSkill("epub-special-layout", "info")
		}
	}

	// nav 恰好一个。
	navCount := 0
	for _, item := range pkg.Manifest {
		if ctx.Err() != nil {
			return
		}
		if opf.HasNavProps(item.Properties) {
			navCount++
		}
	}
	if navCount != 1 {
		level := "warn"
		if !strings.HasPrefix(version, "2") {
			level = "error"
		}
		ins.addFinding(level, "EPUB 3 package should contain exactly one nav item", "", "")
		ins.addSkill("epub-audit", level)
	}

	// NCX。
	_, hasNCX := pkg.NCXItem()
	hasSpineToc := pkg.SpineToc != ""
	if !hasNCX || !hasSpineToc {
		ins.addFinding("warn", `Kindle/legacy delivery should keep toc.ncx and spine toc="ncx"`, "", "")
		ins.addSkill("epub-reader-verify", "warn")
		ins.addSkill("epub-audit", "warn")
	}

	// 封面。
	_, hasCoverProp := pkg.CoverItem()
	hasCoverMeta := false
	for _, m := range pkg.Metas {
		if ctx.Err() != nil {
			return
		}
		if m.Name == "cover" && m.Content != "" {
			hasCoverMeta = true
			break
		}
	}
	if !hasCoverProp || !hasCoverMeta {
		ins.addFinding("warn", `Cover should have properties="cover-image" and legacy meta name="cover"`, "", "")
		ins.addSkill("epub-audit", "warn")
		ins.addSkill("epub-reader-verify", "warn")
	}

	// manifest href 解析。
	for _, item := range pkg.Manifest {
		if ctx.Err() != nil {
			return
		}
		if item.Href == "" {
			ins.addFinding("error", "Manifest item missing href", item.ID, "")
			ins.addSkill("epub-audit", "error")
			continue
		}
		if item.ArchivePath == "" {
			continue // 外链
		}
		if !hasEntry(ins.b, item.ArchivePath) {
			ins.addFinding("error", "Manifest href missing", item.ArchivePath, "")
			ins.addSkill("epub-audit", "error")
		}
	}

	// spine idref。
	for _, ref := range pkg.Spine {
		if ctx.Err() != nil {
			return
		}
		if ref.IDRef == "" {
			ins.addFinding("error", "Spine idref missing from manifest", "<missing>", "")
			ins.addSkill("epub-audit", "error")
			continue
		}
		if _, ok := pkg.ItemByID(ref.IDRef); !ok {
			ins.addFinding("error", "Spine idref missing from manifest", ref.IDRef, "")
			ins.addSkill("epub-audit", "error")
		}
	}

	manifestPaths := make(map[string]struct{}, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		if item.ArchivePath != "" {
			if _, exists := manifestPaths[item.ArchivePath]; !exists {
				manifestPaths[item.ArchivePath] = struct{}{}
			}
		}
	}
	ins.checkCSSURLs(ctx, pkg, manifestPaths)
	ins.checkImages(ctx, pkg)
	ins.checkXHTML(ctx, pkg)
	ins.ocrHeuristic(pkg)
	ins.mediaDrivenSkills(pkg, q)
}

func (ins *inspector) checkCSSURLs(ctx context.Context, pkg *opf.Package, manifestPaths map[string]struct{}) {
	fontExts := map[string]bool{".otf": true, ".ttf": true, ".woff": true, ".woff2": true}
	for _, item := range pkg.Manifest {
		if ctx.Err() != nil {
			return
		}
		if item.MediaType != "text/css" || item.ArchivePath == "" {
			continue
		}
		if !hasEntry(ins.b, item.ArchivePath) {
			continue
		}
		raw, err := ins.b.Current(item.ArchivePath)
		if err != nil {
			continue
		}
		references, err := cssscan.ScanReferences(raw)
		if err != nil {
			ins.addFinding("error", "CSS reference scan failed: "+err.Error(), item.Href, "css-reference-scan")
			ins.addSkill("epub-audit", "error")
			continue
		}
		for _, ref := range references {
			if ctx.Err() != nil {
				return
			}
			if !isCSSURLFunction(ref) {
				continue
			}
			target := ref.Value
			if isExternalURL(target) {
				continue
			}
			parts := pypath.URLSplit(target)
			clean := parts.Path
			if strings.ContainsRune(clean, '\\') {
				ins.addFinding("warn", "CSS url() uses escapes; target not verified", item.Href+" -> "+target, "css-reference-escaped")
				continue
			}
			clean = unquotePct(clean)
			if clean == "" {
				continue
			}
			abs := joinArchivePath(parentDir(item.ArchivePath), clean)
			if hasEntry(ins.b, abs) {
				if _, ok := manifestPaths[abs]; !ok {
					ins.addFinding("error", "CSS url() target missing from OPF manifest",
						item.Href+" -> "+target, "")
					ins.addSkill("epub-audit", "error")
				}
				continue
			}
			ext := strings.ToLower(clean)
			if i := strings.LastIndexByte(ext, '.'); i >= 0 {
				ext = ext[i:]
			} else {
				ext = ""
			}
			if fontExts[ext] {
				ins.addFinding("warn",
					"CSS font url() target missing; preserve declaration for local() fallback and review manually",
					item.Href+" -> "+target, "missing-css-font-fallback")
				ins.addSkill("epub-cleanup", "warn")
				ins.addSkill("epub-audit", "warn")
				ins.addSkill("epub-cleanup", "warn")
			} else {
				ins.addFinding("error", "CSS url() target missing", item.Href+" -> "+target, "")
				ins.addSkill("epub-cleanup", "error")
				ins.addSkill("epub-audit", "error")
			}
		}
	}
}

func (ins *inspector) checkImages(ctx context.Context, pkg *opf.Package) {
	for _, item := range pkg.Manifest {
		if ctx.Err() != nil {
			return
		}
		if item.ArchivePath == "" || !hasEntry(ins.b, item.ArchivePath) {
			continue
		}
		lower := strings.ToLower(item.Href)
		switch {
		case strings.HasSuffix(lower, ".webp"):
			ins.addFinding("warn", "WebP is not a Kindle main-path image format", item.Href, "")
			ins.addSkill("epub-audit", "warn")
			ins.addSkill("epub-reader-verify", "warn")
		case strings.HasSuffix(lower, ".svg") && opf.HasNavProps(item.Properties) && strings.Contains(" "+item.Properties+" ", " cover-image "):
			ins.addFinding("warn", "SVG-only cover is risky for Kindle delivery", item.Href, "")
			ins.addSkill("epub-audit", "warn")
			ins.addSkill("epub-reader-verify", "warn")
		case strings.HasSuffix(lower, ".tif") || strings.HasSuffix(lower, ".tiff") || strings.HasSuffix(lower, ".gif"):
			ins.addFinding("warn", "Convert this image to JPEG/PNG for EPUB delivery", item.Href, "")
			ins.addSkill("epub-audit", "warn")
		}
	}
}

func (ins *inspector) checkXHTML(ctx context.Context, pkg *opf.Package) {
	textChars, imageRefs := 0, 0
	for _, item := range pkg.Manifest {
		if ctx.Err() != nil {
			return
		}
		if item.MediaType != "application/xhtml+xml" || item.ArchivePath == "" || !hasEntry(ins.b, item.ArchivePath) {
			continue
		}
		raw, err := ins.b.Current(item.ArchivePath)
		if err != nil {
			continue
		}
		text := string(raw)
		stripped := tagStripRe.ReplaceAllString(text, "")
		for _, r := range stripped {
			if ctx.Err() != nil {
				return
			}
			if !isPySpaceRune(r) {
				textChars++
			}
		}
		imageRefs += len(imgRe.FindAllString(text, -1))

		if enLangRe.MatchString(text) {
			ins.addSkill("epub-special-layout", "info")
		}
		// 对齐 Python：含 <math（或命名空间 URI 子串）而 manifest 缺 properties。
		if (strings.Contains(text, "<math") || strings.Contains(text, mathmlURI)) && !propsContain(pkg, item.Properties, "mathml") {
			ins.addFinding("error", `MathML XHTML item missing properties="mathml"`, item.Href, "")
			ins.addSkill("epub-audit", "error")
			ins.addSkill("epub-reader-verify", "error")
		}
		if (strings.Contains(text, "<svg") || strings.Contains(text, svgURI)) && !propsContain(pkg, item.Properties, "svg") {
			ins.addFinding("error", `Inline SVG XHTML item missing properties="svg"`, item.Href, "")
			ins.addSkill("epub-audit", "error")
		}
		if noterefRe.MatchString(text) {
			ins.addSkill("epub-cleanup", "info")
			if !footnoteRe.MatchString(text) {
				ins.addFinding("warn", "noteref found without same-file footnote aside", item.Href, "")
				ins.addSkill("epub-cleanup", "warn")
			}
		}
		if strings.Contains(text, "duokan-footnote") {
			ins.addSkill("epub-special-layout", "info")
		}
		if strings.Contains(text, "writing-mode") || strings.Contains(text, "page-vrl") || strings.Contains(text, "<ruby") {
			ins.addSkill("epub-special-layout", "info")
		}
	}
	ins.summaryOCRCounters(textChars, imageRefs)
}

func (ins *inspector) summaryOCRCounters(textChars, imageRefs int) {
	ins.textChars = textChars
	ins.imageRefs = imageRefs
}

func (ins *inspector) ocrHeuristic(pkg *opf.Package) {
	if ins.imageRefs != 0 && ins.imageRefs >= ins.summary.MediaCounts["xhtml"] &&
		ins.textChars < max2(300, ins.imageRefs*120) {
		ins.addFinding("warn",
			"This EPUB appears to be OCR-derived or scan-heavy; cleanup is unlikely to help until source intake/OCR is revisited",
			"", "ocr-residual")
		ins.addSkill("epub-source-intake", "warn")
	}
}

func (ins *inspector) mediaDrivenSkills(pkg *opf.Package, q string) {
	if ins.summary.MediaCounts["css"] > 0 {
		ins.addSkill("epub-cleanup", "info")
	}
	if ins.summary.MediaCounts["xhtml"] > 0 {
		ins.addSkill("epub-audit", "info")
		ins.addCommand("epub run epub.text.content.analyze --input " + q + " --json")
	}
	if ins.summary.MediaCounts["images"] > 0 {
		ins.addSkill("epub-audit", "info")
	}
	if ins.summary.MediaCounts["fonts"] > 0 {
		ins.addSkill("epub-audit", "info")
		ins.addCommand("epub run epub.font.coverage.analyze --input " + q + " --json")
	}
	lang := ins.summary.Language
	if ins.summary.MediaCounts["xhtml"] > 0 && !strings.HasPrefix(strings.ToLower(lang), "en") {
		ins.addSkill("epub-cleanup", "info")
	}
	ins.addSkill("epub-audit", "info")
	ins.addSkill("epub-audit", "info")
}

// applyWorkflowMode 对齐 apply_workflow_mode：cleanup 模式重排技能。
func (ins *inspector) applyWorkflowMode() {
	var kept []string
	for _, s := range ins.skills {
		if s != "$epub-source-intake" {
			kept = append(kept, s)
		}
	}
	ins.skills = kept
	ins.skillLv["$epub-source-intake"] = ""
}

func hasEntry(b *book.Book, name string) bool {
	return b.Has(name)
}

func isFontMedia(media string) bool {
	m := strings.ToLower(media)
	switch m {
	case "application/x-font-ttf", "application/x-font-opentype", "application/font-sfnt", "font/ttf", "font/otf":
		return true
	}
	return strings.Contains(m, "font")
}

func hasSuffixAny(s string, suffixes ...string) bool {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

func propsContain(pkg *opf.Package, props, want string) bool {
	for _, p := range strings.Fields(props) {
		if p == want {
			return true
		}
	}
	return false
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func isPySpaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func parentDir(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

func joinArchivePath(base, rel string) string {
	return normJoin(base, rel)
}

// normJoin 对齐 epub_lib.norm_join（去 fragment 后 posixpath.normpath(join)）。
func normJoin(base, href string) string {
	clean := href
	if i := strings.IndexByte(clean, '#'); i >= 0 {
		clean = clean[:i]
	}
	p := base
	if p == "" {
		p = "."
	}
	joined := p + "/" + clean
	return normalizeArchivePath(joined)
}

func normalizeArchivePath(p string) string {
	parts := strings.Split(p, "/")
	var out []string
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		default:
			out = append(out, part)
		}
	}
	return strings.Join(out, "/")
}

func unquotePct(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '%' && i+2 < len(s) {
			h, l := unhexByte(s[i+1]), unhexByte(s[i+2])
			if h >= 0 && l >= 0 {
				b.WriteByte(byte(h<<4 | l))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func unhexByte(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func isExternalURL(uri string) bool {
	return opf.IsExternalURI(uri) || strings.HasPrefix(uri, "#")
}

func isCSSURLFunction(ref cssscan.Reference) bool {
	if ref.Kind == cssscan.ReferenceURL {
		return true
	}
	// Preserve the legacy url() check for @import url(...), but do not add new
	// checks for quoted @import values that the old extractor did not inspect.
	return ref.Kind == cssscan.ReferenceImport && ref.ValueSpan.Start-ref.Span.Start > 1
}
