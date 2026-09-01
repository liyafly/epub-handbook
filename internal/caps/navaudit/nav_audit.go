// Package navaudit 移植 epub.package.nav.audit（scripts/epub_preflight_harness.py
// 与 scripts/epub_ai/ 的检查家族）。它是只读 validator：不产生 edits。
//
// legacy-report 形状对齐 preflight harness 的 JSON（10 基键 + harness /
// preflight_status / next_gate）；新信封 findings 与其一一对应。
package navaudit

import (
	"context"
	"fmt"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/extern"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// legacyFinding 对齐 epub_ai finding() 的键序：level, message[, path[, kind]]。
type legacyFinding struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

// Params 是 nav.audit 的参数。
type Params struct {
	// LegacyReport 输出 preflight harness 的原始 JSON 形状。
	LegacyReport bool
	// Report 选报告族：preflight（默认）或 layout-audit（AI harness 形状，
	// 无 harness/preflight_status/next_gate 包装，也无 spine 特判）。
	Report string // "preflight" | "layout-audit"
}

type inspector struct {
	b           *book.Book
	pkg         *opf.Package
	opfPath     string
	mode        string
	summary     *orderedSummary
	findings    []legacyFinding
	skills      []string
	skillLv     map[string]string
	commands    []string
	tools       *orderedTools
	textChars   int
	imageRefs   int
	layoutAudit bool
}

// orderedSummary 保证 legacy JSON 的键序与 Python dict 插入序一致。
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

// Run 执行 nav.audit（只读）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	ins := &inspector{
		b:           b,
		mode:        "cleanup",
		layoutAudit: p.Report == "layout-audit",
		summary:     &orderedSummary{MediaCounts: map[string]int{"xhtml": 0, "css": 0, "images": 0, "fonts": 0, "other": 0}},
		skillLv:     map[string]string{},
		tools:       &orderedTools{Values: map[string]bool{}},
	}
	ins.inspect()

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
	if p.LegacyReport {
		if p.Report == "layout-audit" {
			res.Facts["legacyReport"] = ins.legacyLayoutAudit(status)
		} else {
			res.Facts["legacyReport"] = ins.legacyReport(status)
		}
	}
	res.NextCommands = ins.nextCommands()
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
	// commands 既用于 legacyReport 的 suggested_commands，也用于新信封的
	// nextCommands；两种报告都必须只暴露当前 Go CLI 的执行面。
	return append([]string(nil), ins.commands...)
}

func (ins *inspector) addFinding(level, message, path, kind string) {
	f := legacyFinding{Level: level, Message: message}
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
func (ins *inspector) inspect() {
	q := shlexQuote(ins.b.InputPath())
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
		ins.inspectOPF()
	}
	if len(ins.findings) == 0 {
		ins.addFinding("info", "No immediate structural issue detected by harness", "", "")
	}
	ins.addCommand("epub capabilities --json")
	// preflight 特有：epubcheck 可用性（经 extern；本机无 → 注释行占位）。
	ins.tools.Keys = append(ins.tools.Keys, "epubcheck")
	if ok, _ := extern.LookPath("epubcheck"); ok {
		ins.tools.Values["epubcheck"] = true
		ins.addCommand("epubcheck " + q)
	} else {
		ins.tools.Values["epubcheck"] = false
		ins.addCommand("# EPUBCheck runs in GitHub Actions; local preflight skips it when unavailable.")
	}
	ins.applyWorkflowMode()
}

func (ins *inspector) inspectOPF() {
	pkg := ins.pkg
	q := shlexQuote(ins.b.InputPath())
	for _, item := range pkg.Manifest {
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
		ins.addSkill("epub-structure-normalizer", "warn")
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
		ins.addSkill("epub3-migrator", "warn")
		ins.addSkill("epub-package-nav-auditor", "warn")
		ins.addCommand("epub run epub.package.migrate.epub3 --input " + q + " --dry-run --json")
		ins.addCommand("epub run epub.package.migrate.epub3 --input " + q +
			" --output work/after/step-1-epub3.epub --json")
	}

	// 语言。
	if langs := pkg.Metadata["language"]; len(langs) > 0 && strings.TrimSpace(langs[0]) != "" {
		ins.summary.Language = collapseSpace(langs[0])
		if strings.HasPrefix(strings.ToLower(ins.summary.Language), "en") {
			ins.addSkill("epub-english-typography-optimizer", "info")
		}
	}

	// nav 恰好一个。
	navCount := 0
	for _, item := range pkg.Manifest {
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
		ins.addSkill("epub-package-nav-auditor", level)
	}

	// NCX。
	_, hasNCX := pkg.NCXItem()
	hasSpineToc := pkg.SpineToc != ""
	if !hasNCX || !hasSpineToc {
		ins.addFinding("warn", `Kindle/legacy delivery should keep toc.ncx and spine toc="ncx"`, "", "")
		ins.addSkill("epub-kindle-compatibility-checker", "warn")
		ins.addSkill("epub-package-nav-auditor", "warn")
	}

	// 封面。
	_, hasCoverProp := pkg.CoverItem()
	hasCoverMeta := false
	for _, m := range pkg.Metas {
		if m.Name == "cover" && m.Content != "" {
			hasCoverMeta = true
			break
		}
	}
	if !hasCoverProp || !hasCoverMeta {
		ins.addFinding("warn", `Cover should have properties="cover-image" and legacy meta name="cover"`, "", "")
		ins.addSkill("epub-image-layout-optimizer", "warn")
		ins.addSkill("epub-kindle-compatibility-checker", "warn")
	}

	// manifest href 解析。
	for _, item := range pkg.Manifest {
		if item.Href == "" {
			ins.addFinding("error", "Manifest item missing href", item.ID, "")
			ins.addSkill("epub-package-nav-auditor", "error")
			continue
		}
		if item.ArchivePath == "" {
			continue // 外链
		}
		if !hasEntry(ins.b, item.ArchivePath) {
			ins.addFinding("error", "Manifest href missing", item.ArchivePath, "")
			ins.addSkill("epub-package-nav-auditor", "error")
		}
	}

	// spine idref。
	for _, ref := range pkg.Spine {
		if ref.IDRef == "" {
			ins.addFinding("error", "Spine idref missing from manifest", "<missing>", "")
			ins.addSkill("epub-package-nav-auditor", "error")
			continue
		}
		if _, ok := pkg.ItemByID(ref.IDRef); !ok {
			ins.addFinding("error", "Spine idref missing from manifest", ref.IDRef, "")
			ins.addSkill("epub-package-nav-auditor", "error")
		}
	}

	ins.checkCSSURLs(pkg)
	ins.checkImages(pkg)
	ins.checkXHTML(pkg)
	ins.ocrHeuristic(pkg)
	ins.mediaDrivenSkills(pkg, q)
}

func (ins *inspector) checkCSSURLs(pkg *opf.Package) {
	fontExts := map[string]bool{".otf": true, ".ttf": true, ".woff": true, ".woff2": true}
	for _, item := range pkg.Manifest {
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
		text := string(raw)
		text = stripCSSComments(text)
		for _, target := range extractCSSURLs(text) {
			if isExternalURL(target) {
				continue
			}
			clean := target
			if i := strings.IndexByte(clean, '#'); i >= 0 {
				clean = clean[:i]
			}
			clean = unquotePct(clean)
			if clean == "" {
				continue
			}
			abs := joinArchivePath(parentDir(item.ArchivePath), clean)
			if hasEntry(ins.b, abs) {
				if _, ok := manifestItemByArchivePath(ins.pkg, abs); !ok {
					ins.addFinding("error", "CSS url() target missing from OPF manifest",
						item.Href+" -> "+target, "")
					ins.addSkill("epub-package-nav-auditor", "error")
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
				ins.addSkill("epub-css-layering-optimizer", "warn")
				ins.addSkill("epub-package-nav-auditor", "warn")
				ins.addSkill("epub-typography-optimizer", "warn")
			} else {
				ins.addFinding("error", "CSS url() target missing", item.Href+" -> "+target, "")
				ins.addSkill("epub-css-layering-optimizer", "error")
				ins.addSkill("epub-package-nav-auditor", "error")
			}
		}
	}
}

func (ins *inspector) checkImages(pkg *opf.Package) {
	for _, item := range pkg.Manifest {
		if item.ArchivePath == "" || !hasEntry(ins.b, item.ArchivePath) {
			continue
		}
		lower := strings.ToLower(item.Href)
		switch {
		case strings.HasSuffix(lower, ".webp"):
			ins.addFinding("warn", "WebP is not a Kindle main-path image format", item.Href, "")
			ins.addSkill("epub-image-layout-optimizer", "warn")
			ins.addSkill("epub-kindle-compatibility-checker", "warn")
		case strings.HasSuffix(lower, ".svg") && opf.HasNavProps(item.Properties) && strings.Contains(" "+item.Properties+" ", " cover-image "):
			ins.addFinding("warn", "SVG-only cover is risky for Kindle delivery", item.Href, "")
			ins.addSkill("epub-image-layout-optimizer", "warn")
			ins.addSkill("epub-kindle-compatibility-checker", "warn")
		case strings.HasSuffix(lower, ".tif") || strings.HasSuffix(lower, ".tiff") || strings.HasSuffix(lower, ".gif"):
			ins.addFinding("warn", "Convert this image to JPEG/PNG for EPUB delivery", item.Href, "")
			ins.addSkill("epub-image-layout-optimizer", "warn")
		}
	}
}

func (ins *inspector) checkXHTML(pkg *opf.Package) {
	textChars, imageRefs := 0, 0
	for _, item := range pkg.Manifest {
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
			if !isPySpaceRune(r) {
				textChars++
			}
		}
		imageRefs += len(imgRe.FindAllString(text, -1))

		if enLangRe.MatchString(text) {
			ins.addSkill("epub-english-typography-optimizer", "info")
		}
		// 对齐 Python：含 <math（或命名空间 URI 子串）而 manifest 缺 properties。
		if (strings.Contains(text, "<math") || strings.Contains(text, mathmlURI)) && !propsContain(pkg, item.Properties, "mathml") {
			ins.addFinding("error", `MathML XHTML item missing properties="mathml"`, item.Href, "")
			ins.addSkill("epub-package-nav-auditor", "error")
			ins.addSkill("epub-kindle-compatibility-checker", "error")
		}
		if (strings.Contains(text, "<svg") || strings.Contains(text, svgURI)) && !propsContain(pkg, item.Properties, "svg") {
			ins.addFinding("error", `Inline SVG XHTML item missing properties="svg"`, item.Href, "")
			ins.addSkill("epub-package-nav-auditor", "error")
		}
		if noterefRe.MatchString(text) {
			ins.addSkill("epub-popup-footnote-converter", "info")
			if !footnoteRe.MatchString(text) {
				ins.addFinding("warn", "noteref found without same-file footnote aside", item.Href, "")
				ins.addSkill("epub-popup-footnote-converter", "warn")
			}
		}
		if strings.Contains(text, "duokan-footnote") {
			ins.addSkill("epub-legacy-footnote-fallback", "info")
		}
		if strings.Contains(text, "writing-mode") || strings.Contains(text, "page-vrl") || strings.Contains(text, "<ruby") {
			ins.addSkill("epub-vertical-ruby-optimizer", "info")
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
		ins.addSkill("epub-css-layering-optimizer", "info")
	}
	if ins.summary.MediaCounts["xhtml"] > 0 {
		ins.addSkill("epub-content-analyzer", "info")
		ins.addCommand("epub run epub.text.content.analyze --input " + q + " --json")
	}
	if ins.summary.MediaCounts["images"] > 0 {
		ins.addSkill("epub-image-layout-optimizer", "info")
	}
	if ins.summary.MediaCounts["fonts"] > 0 {
		ins.addSkill("epub-font-coverage-analyzer", "info")
		ins.addCommand("epub run epub.font.coverage.analyze --input " + q + " --json")
	}
	lang := ins.summary.Language
	if ins.summary.MediaCounts["xhtml"] > 0 && !strings.HasPrefix(strings.ToLower(lang), "en") {
		ins.addSkill("epub-typography-optimizer", "info")
	}
	ins.addSkill("epub-layout-auditor", "info")
	ins.addSkill("epub-package-nav-auditor", "info")
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

func manifestItemByArchivePath(pkg *opf.Package, path string) (opf.ManifestItem, bool) {
	for _, it := range pkg.Manifest {
		if it.ArchivePath == path {
			return it, true
		}
	}
	return opf.ManifestItem{}, false
}

func stripCSSComments(text string) string {
	var b strings.Builder
	for {
		i := strings.Index(text, "/*")
		if i < 0 {
			b.WriteString(text)
			break
		}
		b.WriteString(text[:i])
		j := strings.Index(text[i+2:], "*/")
		if j < 0 {
			break
		}
		text = text[i+2+j+2:]
	}
	return b.String()
}

func extractCSSURLs(text string) []string {
	var out []string
	for _, m := range cssURLRe.FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}
	return out
}

// shlexQuote 复刻 Python shlex.quote：不含危险字符时原样返回，
// 否则用单引号包裹并按 '"'"' 方式转义内部单引号。
func shlexQuote(s string) string {
	for i := 0; i < len(s); i++ {
		b := s[i]
		unsafe := b < 'a' && b != '_' && b != '%' && b != '+' && b != '=' && b != '@' && b != ',' && b != '.' && b != '/' && b != '-' ||
			(b > 'z' && b != '~') || (b > 'Z' && b < 'a' && b != '^') || (b > '9' && b < 'A')
		if unsafe || b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
		}
	}
	return s
}
