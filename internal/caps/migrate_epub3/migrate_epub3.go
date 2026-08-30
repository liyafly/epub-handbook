// Package migrateepub3 迁移 scripts/epub3_conversion/（全部转发层合并）
// + scripts/epub3_oneclick_converter.py 门面（capability id：
// epub.package.migrate.epub3）。按 SPEC §7.2 的裁决，Go 侧只实现 B
// （epub3_conversion）的行为。
//
// convert_epub 的动作序列逐段复刻（顺序敏感）：
//
//  1. normalize_metadata（version=3.0、rendition prefix、dc:date event
//     处理——含 Python「裸 event 属性总是被删」的求值顺序、
//     dcterms:modified 当前 UTC、ibooks:specified-fonts 三态、ibooks prefix）
//  2. has_body_font_locked（body class 字节正则 + CSS body 规则的
//     split(";")[-1] 怪癖）
//  3. normalize_manifest_media / ensure_cover_properties / ensure_spine_toc /
//     fix_guide_hrefs
//  4. NCX→nav 生成（sanitize_ncx_text 坏引号修复 + nav.xhtml 模板逐字节 +
//     landmarks），无 NCX 时 spine_entries 兜底
//  5. CJK 排版覆盖样式注入（enhancement_css 常量照抄 + typography_roles）
//  6. note.png 图标（优先读 skills 资产；href 按 unique_href 规则）
//  7. update_xhtml_files 每页管线：normalize_xhtml_shell →
//     convert_plain_notes → convert_sigil_legacy_notes →
//     normalize_duokan_notes → svg/mathml/scripted 属性标记 →
//     format_xhtml_multiline（element-only 缩进、混合内容不缩进、
//     无效 XML 原样放行）
//  8. OPF 最终重序列化：字节区间编辑复刻 ET.tostring 输出（xmlmini），
//     前缀注册表对齐 epub_lib.py import 期之后的 _namespace_map（OPF 带
//     opf: 前缀，见 register.go 注释）。
//
// 三段式（SPEC §6.1）：扫描只读 b 并产出 []editset.Edit；b.Apply 是唯一
// 写点；报告不落盘。mimetype 重置与固定 mtime、字母序新 entry 由
// internal/book + internal/zipfs 的 write 语义承担。
package migrateepub3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
)

// CapabilityID 是契约 id（contracts/capabilities/v1/epub.package.migrate.epub3.json）。
const CapabilityID = "epub.package.migrate.epub3"

// ErrConversion 对齐 Python 的 ConversionError（EpubLibError）。
var ErrConversion = errors.New("epub.package.migrate.epub3: conversion failed")

type toolError struct{ msg string }

func (e *toolError) Error() string   { return e.msg }
func (e *toolError) Is(t error) bool { return t == ErrConversion }

func convErrf(format string, a ...any) error {
	return &toolError{msg: fmt.Sprintf(format, a...)}
}

const canonicalMimetype = "application/epub+zip"

// Params 是 capability 参数。PopupNotes / Typography 对齐 Python 的
// popup_notes / typography（默认开启，由注册闭包把 no_* 反转传入）。
type Params struct {
	PopupNotes   bool
	Typography   bool
	DryRun       bool
	LegacyReport bool
	// Output 仅为 legacy 报告的 output 字段（本包不落盘）。
	Output string
}

// conversionReport 对齐 epub3_conversion/models.py 的 ConversionReport
// as_dict 键序（dataclass 字段序即 JSON 键序）。
type conversionReport struct {
	Harness               string   `json:"harness"`
	InputSHA256           string   `json:"input_sha256"`
	Output                string   `json:"output"`
	OPF                   string   `json:"opf"`
	PackageVersionBefore  *string  `json:"package_version_before"`
	NavEntries            int      `json:"nav_entries"`
	XHTMLFilesUpdated     int      `json:"xhtml_files_updated"`
	StylesheetLinksAdded  int      `json:"stylesheet_links_added"`
	PlainNotesConverted   int      `json:"plain_notes_converted"`
	DuokanNotesNormalized int      `json:"duokan_notes_normalized"`
	ManifestItemsAdded    []string `json:"manifest_items_added"`
	ManifestItemsUpdated  int      `json:"manifest_items_updated"`
	MetadataUpdates       []string `json:"metadata_updates"`
	TypographyRoles       []string `json:"typography_roles"`
	Warnings              []string `json:"warnings"`
}

// workFiles 复刻 convert_epub 的 files dict：原 entry + 待写入覆盖。
type workFiles struct {
	b         *book.Book
	overrides map[string][]byte
}

func newWorkFiles(b *book.Book) *workFiles {
	return &workFiles{b: b, overrides: map[string][]byte{}}
}

func (w *workFiles) has(name string) bool {
	if _, ok := w.overrides[name]; ok {
		return true
	}
	return w.b.Has(name)
}

func (w *workFiles) read(name string) ([]byte, error) {
	if c, ok := w.overrides[name]; ok {
		return c, nil
	}
	return w.b.Current(name)
}

func (w *workFiles) write(name string, data []byte) {
	w.overrides[name] = data
}

func (w *workFiles) originalNames() []string { return w.b.OriginalNames() }

type scanResult struct {
	edits []editset.Edit
	rep   *conversionReport
}

// Run 执行本 capability。禁止修改 b 之外的任何状态；落盘由 pipeline 的
// b.WriteTo 负责（INV-3）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	scan, err := scanPhase(b, p)
	if err != nil {
		return report.Result{}, err
	}
	if !p.DryRun {
		if err := b.Apply(scan.edits); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
	}
	return buildResult(p, scan.rep), nil
}

// scanPhase 逐行复刻 converter.convert_epub（只读 b）。
func scanPhase(b *book.Book, p Params) (*scanResult, error) {
	rep := &conversionReport{
		Harness:              "epub3_oneclick_converter",
		Output:               p.Output,
		ManifestItemsAdded:   []string{},
		MetadataUpdates:      []string{},
		TypographyRoles:      []string{},
		Warnings:             []string{},
	}
	sum, err := fileSHA256Hex(b.InputPath())
	if err != nil {
		return nil, err
	}
	rep.InputSHA256 = sum

	files := newWorkFiles(b)
	// Python：files["mimetype"] = b"application/epub+zip"（无条件重置）。
	files.write("mimetype", []byte(canonicalMimetype))

	opfPath, err := opfPathFromContainer(files)
	if err != nil {
		return nil, err
	}
	rep.OPF = opfPath
	opfData, err := files.read(opfPath)
	if err != nil {
		return nil, convErrf("%v", err)
	}
	root, err := parseXMLTree(opfData)
	if err != nil {
		return nil, convErrf("%s: XML parse failed: %v", opfPath, err)
	}
	if v, ok := root.getAttr("version"); ok {
		rep.PackageVersionBefore = &v
	}
	opfDir := pyDirname(opfPath)

	bodyFontLocked := hasBodyFontLocked(files)
	if err := normalizeMetadata(root, rep, bodyFontLocked); err != nil {
		return nil, err
	}
	normalizeManifestMedia(root, rep)
	ensureCoverProperties(root, rep)
	if err := ensureSpineToc(root); err != nil {
		return nil, err
	}
	fixGuideHrefs(root, files, opfDir, rep)

	styleHref := uniqueHref(files, opfDir, "Styles/epub3-enhancements.css")
	styleZip := normJoin(opfDir, styleHref)
	noteHref := defaultNoteHref(files, root, opfDir)
	noteZip := normJoin(opfDir, noteHref)
	if p.Typography {
		rep.TypographyRoles = append([]string{}, typographyRoles...)
		files.write(styleZip, []byte(enhancementCSS))
		if _, err := addManifestItem(root, rep, "epub3-enhancements-css", styleHref, "text/css", ""); err != nil {
			return nil, err
		}
	}
	defaultNoteIconUsed, err := updateXHTMLFiles(files, root, opfPath, styleZip, noteZip, rep, p.PopupNotes, p.Typography)
	if err != nil {
		return nil, err
	}
	if p.PopupNotes && defaultNoteIconUsed {
		if !files.has(noteZip) {
			files.write(noteZip, notePNGBytes())
		}
		if _, err := addManifestItem(root, rep, "note-icon", noteHref, "image/png", ""); err != nil {
			return nil, err
		}
	}
	if err := ensureNav(files, root, opfPath, rep); err != nil {
		return nil, err
	}
	files.write(opfPath, serializeTree(root, namespacePrefixesOPF, true))

	edits, err := buildEdits(b, files)
	if err != nil {
		return nil, err
	}
	return &scanResult{edits: edits, rep: rep}, nil
}

// buildEdits 把 overrides 转成 []editset.Edit：
//   - mimetype 无条件整段重置（让 book.WriteTo 以 STORED 规范写出）；
//   - 既有 entry 整段替换；新 entry 以 Offset=0/Length=0 创建。
func buildEdits(b *book.Book, files *workFiles) ([]editset.Edit, error) {
	var edits []editset.Edit
	if b.Has("mimetype") {
		cur, err := b.Current("mimetype")
		if err != nil {
			return nil, convErrf("%v", err)
		}
		edits = append(edits, editset.Replace("mimetype", 0, int64(len(cur)), []byte(canonicalMimetype)))
	} else {
		edits = append(edits, editset.Replace("mimetype", 0, 0, []byte(canonicalMimetype)))
	}
	for _, name := range b.OriginalNames() {
		if name == "mimetype" {
			continue
		}
		data, ok := files.overrides[name]
		if !ok {
			continue
		}
		cur, err := b.Current(name)
		if err != nil {
			return nil, convErrf("%v", err)
		}
		edits = append(edits, editset.Replace(name, 0, int64(len(cur)), data))
	}
	var added []string
	for name := range files.overrides {
		if name != "mimetype" && !b.Has(name) {
			added = append(added, name)
		}
	}
	sort.Strings(added)
	for _, name := range added {
		edits = append(edits, editset.Replace(name, 0, 0, files.overrides[name]))
	}
	return edits, nil
}

// buildResult 装配统一信封（含 legacy-report 脚手架）。
func buildResult(p Params, rep *conversionReport) report.Result {
	var versionBefore any
	if rep.PackageVersionBefore != nil {
		versionBefore = *rep.PackageVersionBefore
	}
	facts := map[string]any{
		"opf":                    rep.OPF,
		"packageVersionBefore":   versionBefore,
		"navEntries":             rep.NavEntries,
		"xhtmlFilesUpdated":      rep.XHTMLFilesUpdated,
		"stylesheetLinksAdded":   rep.StylesheetLinksAdded,
		"plainNotesConverted":    rep.PlainNotesConverted,
		"duokanNotesNormalized":  rep.DuokanNotesNormalized,
		"manifestItemsAdded":     rep.ManifestItemsAdded,
		"manifestItemsUpdated":   rep.ManifestItemsUpdated,
		"metadataUpdates":        rep.MetadataUpdates,
		"typographyRoles":        rep.TypographyRoles,
		"warnings":               rep.Warnings,
		"popupNotes":             p.PopupNotes,
		"typography":             p.Typography,
	}
	var findings []report.Finding
	for _, w := range rep.Warnings {
		findings = append(findings, report.Finding{Level: "warn", ID: "migrate.warning", Title: w})
	}
	events := []report.Event{{
		Step:   "convert",
		Status: "completed",
		Message: fmt.Sprintf("nav_entries=%d xhtml_files_updated=%d plain_notes=%d duokan=%d",
			rep.NavEntries, rep.XHTMLFilesUpdated, rep.PlainNotesConverted, rep.DuokanNotesNormalized),
	}}
	if p.LegacyReport {
		raw, err := report.MarshalLegacy(rep)
		if err == nil {
			// 存 json.RawMessage，避免 []byte 被信封编码成 base64。
			facts["legacyReport"] = json.RawMessage(bytes.TrimSuffix(raw, []byte("\n")))
		}
	}
	return report.Result{
		Capability: CapabilityID,
		Status:     report.StatusComplete,
		Facts:      facts,
		Findings:   findings,
		Events:     events,
	}
}

// ---- 读包 ----

// opfPathFromContainer 逐行复刻 epub_lib.opf_path_from_container。
func opfPathFromContainer(files *workFiles) (string, error) {
	const containerPath = "META-INF/container.xml"
	if !files.has(containerPath) {
		return "", convErrf("missing META-INF/container.xml")
	}
	data, err := files.read(containerPath)
	if err != nil {
		return "", convErrf("%v", err)
	}
	container, err := parseXMLTree(data)
	if err != nil {
		return "", convErrf("%s: XML parse failed: %v", containerPath, err)
	}
	var rootfile *xmlElem
	for _, e := range iterAll(container) {
		if e != container && e.ns == containerURI && e.name == "rootfile" {
			rootfile = e
			break
		}
	}
	if rootfile == nil {
		return "", convErrf("container rootfile does not resolve: <missing>")
	}
	opfPath, _ := rootfile.getAttr("full-path")
	if opfPath == "" || !files.has(opfPath) {
		label := opfPath
		if label == "" {
			label = "<missing>"
		}
		return "", convErrf("container rootfile does not resolve: %s", label)
	}
	return opfPath, nil
}

func fileSHA256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", convErrf("%v", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", convErrf("%v", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ---- package passes（scripts/epub3_conversion/package.py） ----

// hasBodyFontLocked 逐行复刻 core.has_body_font_locked。
func hasBodyFontLocked(files *workFiles) bool {
	for _, name := range files.originalNames() {
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".xhtml") && !strings.HasSuffix(lower, ".html") && !strings.HasSuffix(lower, ".htm") {
			continue
		}
		data, err := files.read(name)
		if err != nil {
			continue
		}
		if bodyFontLockedInXHTML(data) {
			return true
		}
	}
	for _, name := range files.originalNames() {
		if !strings.HasSuffix(strings.ToLower(name), ".css") {
			continue
		}
		data, err := files.read(name)
		if err != nil {
			continue
		}
		if cssBodyFontLocked(stripCSSComments(utf8ReplaceDecode(data))) {
			return true
		}
	}
	return false
}

// cssBodyFontLocked 复刻 CSS 规则扫描：([^{}]+)\{([^{}]*)\}，
// 声明含 \bfont-family\s*: 且任一选择器 strip().split(";")[-1].strip().lower()
// == "body"。
func cssBodyFontLocked(css string) bool {
	runes := []rune(css)
	i := 0
	for i < len(runes) {
		if runes[i] == '{' || runes[i] == '}' {
			i++
			continue
		}
		j := i
		for j < len(runes) && runes[j] != '{' && runes[j] != '}' {
			j++
		}
		if j >= len(runes) {
			break
		}
		if runes[j] == '}' {
			i++
			continue
		}
		k := j + 1
		for k < len(runes) && runes[k] != '}' && runes[k] != '{' {
			k++
		}
		if k >= len(runes) {
			break
		}
		if runes[k] == '{' {
			i++
			continue
		}
		selectors := string(runes[i:j])
		declarations := string(runes[j+1 : k])
		i = k + 1
		if !pyPatterns["fontFamilyDecl"].hasMatch(declarations) {
			continue
		}
		for _, selector := range strings.Split(selectors, ",") {
			parts := strings.Split(pyStrip(selector), ";")
			last := pyStrip(parts[len(parts)-1])
			if strings.ToLower(last) == "body" {
				return true
			}
		}
	}
	return false
}

// normalizeMetadata 逐行复刻 core.normalize_metadata。
func normalizeMetadata(root *xmlElem, rep *conversionReport, bodyFontLocked bool) error {
	meta := root.childByTag(opfURI, "metadata")
	if meta == nil {
		return convErrf("OPF missing metadata")
	}
	root.setAttr("", "version", "3.0")
	addPackagePrefix(root, "rendition", renditionURI)

	for _, child := range append([]*xmlElem(nil), meta.children...) {
		if child.ns != dcURI || child.name != "date" {
			continue
		}
		// Python：child.attrib.pop(q(OPF,"event"), child.attrib.pop("event",""))。
		// 求值顺序使两个 event 键总是被删除；取值优先 opf:event。
		namedVal, hadNamed := child.getAttrNS(opfURI, "event")
		child.delAttr(opfURI, "event")
		bareVal, hadBare := child.getAttr("event")
		child.delAttr("", "event")
		event := ""
		if hadNamed {
			event = namedVal
		} else if hadBare {
			event = bareVal
		}
		event = strings.ToLower(event)
		switch event {
		case "modification":
			meta.removeChild(child)
			rep.MetadataUpdates = append(rep.MetadataUpdates, "removed legacy modification dc:date")
		case "creation":
			created := newElem(opfURI, "meta", xmlAttr{name: "property", value: "dcterms:created"})
			created.text = child.text
			index := meta.indexOfChild(child)
			meta.removeChild(child)
			meta.insertChildAt(index, created)
			rep.MetadataUpdates = append(rep.MetadataUpdates, "mapped creation dc:date to dcterms:created")
		case "publication", "issued":
			child.clearAttrs()
		}
	}

	var modified, specifiedFonts *xmlElem
	for _, child := range meta.childrenByTag(opfURI, "meta") {
		prop, ok := child.getAttr("property")
		if !ok {
			continue
		}
		if prop == "dcterms:modified" {
			modified = child
		} else if prop == "ibooks:specified-fonts" {
			specifiedFonts = child
		}
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	if modified == nil {
		modified = newElem(opfURI, "meta", xmlAttr{name: "property", value: "dcterms:modified"})
		meta.appendChild(modified)
		rep.MetadataUpdates = append(rep.MetadataUpdates, "added dcterms:modified")
	} else {
		rep.MetadataUpdates = append(rep.MetadataUpdates, "updated dcterms:modified")
	}
	modified.text = now

	if specifiedFonts == nil {
		if bodyFontLocked {
			sf := newElem(opfURI, "meta", xmlAttr{name: "property", value: "ibooks:specified-fonts"})
			sf.text = "true"
			meta.appendChild(sf)
			rep.MetadataUpdates = append(rep.MetadataUpdates, "added ibooks:specified-fonts (locked body font detected)")
		}
	} else if !bodyFontLocked {
		rep.MetadataUpdates = append(rep.MetadataUpdates, "kept existing ibooks:specified-fonts (no locked body font detected; review manually)")
	}

	for _, child := range meta.childrenByTag(opfURI, "meta") {
		if prop, ok := child.getAttr("property"); ok && strings.HasPrefix(prop, "ibooks:") {
			addPackagePrefix(root, "ibooks", ibooksPrefix)
			break
		}
	}
	return nil
}

// addPackagePrefix 逐行复刻 core.add_package_prefix。
func addPackagePrefix(root *xmlElem, name, uri string) {
	prefix := root.attrOr("prefix", "")
	if prefixHasBinding(prefix, name) {
		return
	}
	addition := name + ": " + uri
	root.setAttr("", "prefix", strings.TrimSpace(strings.TrimSpace(prefix)+" "+addition))
}

// prefixHasBinding 复刻 re.search(rf"(?:^|\s){name}\s*:", prefix)。
func prefixHasBinding(prefix, name string) bool {
	runes := []rune(prefix)
	nameRunes := []rune(name)
	for i := 0; i+len(nameRunes) <= len(runes); i++ {
		if i != 0 && !pyIsSpace(runes[i-1]) {
			continue
		}
		match := true
		for k, nr := range nameRunes {
			if runes[i+k] != nr {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		j := i + len(nameRunes)
		for j < len(runes) && pyIsSpace(runes[j]) {
			j++
		}
		if j < len(runes) && runes[j] == ':' {
			return true
		}
	}
	return false
}

// manifestMaps 逐行复刻 core.manifest_maps。
func manifestMaps(root *xmlElem, opfDir string) (map[string]*xmlElem, map[string]*xmlElem) {
	byID := map[string]*xmlElem{}
	byZip := map[string]*xmlElem{}
	manifest := root.childByTag(opfURI, "manifest")
	if manifest == nil {
		return byID, byZip
	}
	for _, item := range manifest.childrenByTag(opfURI, "item") {
		itemID, okID := item.getAttr("id")
		href, okHref := item.getAttr("href")
		if okID && itemID != "" {
			byID[itemID] = item
		}
		if okHref && href != "" {
			byZip[normJoin(opfDir, href)] = item
		}
	}
	return byID, byZip
}

// findCoverID 复刻 core.find_cover_id。
func findCoverID(root *xmlElem) string {
	meta := root.childByTag(opfURI, "metadata")
	if meta == nil {
		return ""
	}
	for _, child := range meta.childrenByTag(opfURI, "meta") {
		if child.attrOr("name", "") == "cover" {
			return child.attrOr("content", "")
		}
	}
	return ""
}

// ensureCoverProperties 逐行复刻 core.ensure_cover_properties。
func ensureCoverProperties(root *xmlElem, rep *conversionReport) {
	coverID := findCoverID(root)
	if coverID == "" {
		return
	}
	manifest := root.childByTag(opfURI, "manifest")
	if manifest == nil {
		return
	}
	for _, item := range manifest.childrenByTag(opfURI, "item") {
		if id, ok := item.getAttr("id"); ok && id == coverID {
			if addProps(item, "cover-image") {
				rep.ManifestItemsUpdated++
			}
			return
		}
	}
}

// normalizeManifestMedia 逐行复刻 core.normalize_manifest_media。
func normalizeManifestMedia(root *xmlElem, rep *conversionReport) {
	manifest := root.childByTag(opfURI, "manifest")
	if manifest == nil {
		return
	}
	for _, item := range manifest.childrenByTag(opfURI, "item") {
		mediaType := item.attrOr("media-type", "")
		href := item.attrOr("href", "")
		suffix := strings.ToLower(pathSuffix(href))
		changed := false
		if fontMediaTypes[mediaType] || suffix == ".ttf" || suffix == ".otf" {
			if mediaType != "application/vnd.ms-opentype" {
				item.setAttr("", "media-type", "application/vnd.ms-opentype")
				changed = true
			}
		} else if want, ok := imageMediaByExt[suffix]; ok && mediaType != want {
			item.setAttr("", "media-type", want)
			changed = true
		}
		if changed {
			rep.ManifestItemsUpdated++
		}
	}
}

// ensureSpineToc 逐行复刻 core.ensure_spine_toc。
func ensureSpineToc(root *xmlElem) error {
	ncx := ncxItem(root)
	if ncx == nil {
		return nil
	}
	ncxID, ok := ncx.getAttr("id")
	if !ok || ncxID == "" {
		return nil
	}
	spine := root.childByTag(opfURI, "spine")
	if spine == nil {
		return convErrf("OPF missing spine")
	}
	spine.setAttr("", "toc", ncxID)
	return nil
}

// ncxItem 复刻 core.ncx_item。
func ncxItem(root *xmlElem) *xmlElem {
	manifest := root.childByTag(opfURI, "manifest")
	if manifest == nil {
		return nil
	}
	for _, item := range manifest.childrenByTag(opfURI, "item") {
		if item.attrOr("media-type", "") == "application/x-dtbncx+xml" {
			return item
		}
	}
	return nil
}

// fixGuideHrefs 逐行复刻 core.fix_guide_hrefs。
func fixGuideHrefs(root *xmlElem, files *workFiles, opfDir string, rep *conversionReport) {
	guide := root.childByTag(opfURI, "guide")
	if guide == nil {
		return
	}
	for _, ref := range guide.childrenByTag(opfURI, "reference") {
		href := ref.attrOr("href", "")
		if href == "" || files.has(normJoin(opfDir, href)) {
			continue
		}
		candidate := href
		for strings.HasPrefix(candidate, "../") {
			candidate = candidate[3:]
			if files.has(normJoin(opfDir, candidate)) {
				ref.setAttr("", "href", candidate)
				rep.ManifestItemsUpdated++
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("fixed guide href: %s -> %s", href, candidate))
				break
			}
		}
	}
}

// hrefExists 复刻 core.href_exists。
func hrefExists(root *xmlElem, href string) *xmlElem {
	manifest := root.childByTag(opfURI, "manifest")
	if manifest == nil {
		return nil
	}
	for _, item := range manifest.childrenByTag(opfURI, "item") {
		if v, ok := item.getAttr("href"); ok && v == href {
			return item
		}
	}
	return nil
}

// uniqueHref 逐行复刻 core.unique_href。
func uniqueHref(files *workFiles, opfDir, href string) string {
	stem, ext := pySplitExt(href)
	candidate := href
	index := 2
	for files.has(normJoin(opfDir, candidate)) {
		candidate = fmt.Sprintf("%s-%d%s", stem, index, ext)
		index++
	}
	return candidate
}

// itemIDExists 复刻 epub_lib.item_id_exists。
func itemIDExists(root *xmlElem, itemID string) bool {
	manifest := root.childByTag(opfURI, "manifest")
	if manifest == nil {
		return false
	}
	for _, item := range manifest.childrenByTag(opfURI, "item") {
		if v, ok := item.getAttr("id"); ok && v == itemID {
			return true
		}
	}
	return false
}

// uniqueID 逐行复刻 epub_lib.unique_id。
func uniqueID(root *xmlElem, base string) string {
	candidate, _ := pyPatterns["idClean"].subTemplate(base, "-", 0)
	candidate = strings.Trim(candidate, "-")
	if candidate == "" {
		candidate = "item"
	}
	if candidate[0] >= '0' && candidate[0] <= '9' {
		candidate = "x-" + candidate
	}
	index := 2
	result := candidate
	for itemIDExists(root, result) {
		result = fmt.Sprintf("%s-%d", candidate, index)
		index++
	}
	return result
}

// addProps 逐行复刻 core.add_props。
func addProps(e *xmlElem, props ...string) bool {
	current := pySplitWS(e.attrOr("properties", ""))
	changed := false
	for _, prop := range props {
		if prop == "" || containsString(current, prop) {
			continue
		}
		current = append(current, prop)
		changed = true
	}
	if changed {
		e.setAttr("", "properties", strings.Join(current, " "))
	}
	return changed
}

// removeProps 逐行复刻 core.remove_props。
func removeProps(e *xmlElem, props ...string) bool {
	current := pySplitWS(e.attrOr("properties", ""))
	updated := make([]string, 0, len(current))
	for _, p := range current {
		drop := false
		for _, rm := range props {
			if p == rm {
				drop = true
				break
			}
		}
		if !drop {
			updated = append(updated, p)
		}
	}
	if stringSliceEqual(updated, current) {
		return false
	}
	if len(updated) > 0 {
		e.setAttr("", "properties", strings.Join(updated, " "))
	} else {
		e.delAttr("", "properties")
	}
	return true
}

// addManifestItem 逐行复刻 core.add_manifest_item。
func addManifestItem(root *xmlElem, rep *conversionReport, itemIDBase, href, mediaType, properties string) (*xmlElem, error) {
	if existing := hrefExists(root, href); existing != nil {
		if properties != "" && addProps(existing, pySplitWS(properties)...) {
			rep.ManifestItemsUpdated++
		}
		return existing, nil
	}
	manifest := root.childByTag(opfURI, "manifest")
	if manifest == nil {
		return nil, convErrf("OPF missing manifest")
	}
	item := newElem(opfURI, "item",
		xmlAttr{name: "id", value: uniqueID(root, itemIDBase)},
		xmlAttr{name: "href", value: href},
		xmlAttr{name: "media-type", value: mediaType},
	)
	if properties != "" {
		item.setAttr("", "properties", properties)
	}
	manifest.appendChild(item)
	rep.ManifestItemsAdded = append(rep.ManifestItemsAdded, href)
	return item, nil
}

// defaultNoteHref 逐行复刻 core.default_note_href。
func defaultNoteHref(files *workFiles, root *xmlElem, opfDir string) string {
	const defaultHref = "Images/note.png"
	defaultZip := normJoin(opfDir, defaultHref)
	if hrefExists(root, defaultHref) != nil || files.has(defaultZip) {
		return defaultHref
	}
	return uniqueHref(files, opfDir, defaultHref)
}

// ---- 导航（scripts/epub3_conversion/navigation.py → core） ----

type navEntry struct {
	label    string
	href     string
	children []navEntry
}

// ensureNav 逐行复刻 core.ensure_nav。
func ensureNav(files *workFiles, root *xmlElem, opfPath string, rep *conversionReport) error {
	opfDir := pyDirname(opfPath)
	var navs []*xmlElem
	if manifest := root.childByTag(opfURI, "manifest"); manifest != nil {
		for _, item := range manifest.childrenByTag(opfURI, "item") {
			if containsString(pySplitWS(item.attrOr("properties", "")), "nav") {
				navs = append(navs, item)
			}
		}
	}
	if len(navs) > 1 {
		for _, extra := range navs[1:] {
			removeProps(extra, "nav")
			rep.ManifestItemsUpdated++
		}
		navs = navs[:1]
	}
	if len(navs) > 0 {
		navItem := navs[0]
		navID := navItem.attrOr("id", "")
		if navID == "" {
			navID = uniqueID(root, "nav")
		}
		navItem.setAttr("", "id", navID)
		return ensureNavInSpine(root, navID)
	}

	entries, err := ncxEntries(files, root, opfDir, rep)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		entries = spineEntries(root)
	}
	if len(entries) == 0 {
		return convErrf("cannot build nav.xhtml: no NCX navPoint or spine entries")
	}
	navHref := uniqueHref(files, opfDir, "nav.xhtml")
	navZip := normJoin(opfDir, navHref)
	files.write(navZip, buildNavXHTML(root, entries))
	navItem, err := addManifestItem(root, rep, "nav", navHref, "application/xhtml+xml", "nav")
	if err != nil {
		return err
	}
	if err := ensureNavInSpine(root, navItem.attrOr("id", "")); err != nil {
		return err
	}
	rep.NavEntries = navCount(entries)
	return nil
}

// ensureNavInSpine 逐行复刻 core.ensure_nav_in_spine。
func ensureNavInSpine(root *xmlElem, navID string) error {
	spine := root.childByTag(opfURI, "spine")
	if spine == nil {
		return convErrf("OPF missing spine")
	}
	for _, ref := range spine.childrenByTag(opfURI, "itemref") {
		if v, ok := ref.getAttr("idref"); ok && v == navID {
			return nil
		}
	}
	spine.appendChild(newElem(opfURI, "itemref",
		xmlAttr{name: "idref", value: navID},
		xmlAttr{name: "linear", value: "no"},
	))
	return nil
}

// sanitizeNCXText 逐行复刻 core.sanitize_ncx_text。
func sanitizeNCXText(data []byte, rep *conversionReport) string {
	text := utf8ReplaceDecode(data)
	text, _ = pyPatterns["doctype"].subTemplate(text, "", 1)
	text = strings.ReplaceAll(text, "&nbsp;", "&#160;")
	fixed, count := pyPatterns["ncxSrcFix"].subTemplate(text, `\1\2\3\5\4`, 0)
	if count > 0 {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("fixed malformed NCX content src fragment quoting: %d", count))
	}
	return fixed
}

// ncxEntries 逐行复刻 core.ncx_entries。
func ncxEntries(files *workFiles, root *xmlElem, opfDir string, rep *conversionReport) ([]navEntry, error) {
	item := ncxItem(root)
	if item == nil {
		return nil, nil
	}
	ncxHref, ok := item.getAttr("href")
	if !ok || ncxHref == "" {
		return nil, nil
	}
	ncxZip := normJoin(opfDir, ncxHref)
	if !files.has(ncxZip) {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("NCX manifest item does not resolve: %s", ncxHref))
		return nil, nil
	}
	data, err := files.read(ncxZip)
	if err != nil {
		return nil, convErrf("%v", err)
	}
	sanitized := sanitizeNCXText(data, rep)
	ncxRoot, err := parseXMLTree([]byte(sanitized))
	if err != nil {
		return nil, convErrf("%s: XML parse failed: %v", ncxZip, err)
	}
	base := pyDirname(ncxHref)
	navMap := ncxRoot.childByTag(ncxURI, "navMap")
	var points []*xmlElem
	if navMap != nil {
		points = navMap.childrenByTag(ncxURI, "navPoint")
	}
	var entries []navEntry
	for _, point := range points {
		if e := parseNavPoints(point, base); e != nil {
			entries = append(entries, *e)
		}
	}
	files.write(ncxZip, []byte(sanitized))
	return entries, nil
}

// parseNavPoints 逐行复刻 core.parse_nav_points。
func parseNavPoints(point *xmlElem, base string) *navEntry {
	label := ""
	if nl := point.childByTag(ncxURI, "navLabel"); nl != nil {
		label = pyTextContent(nl.childByTag(ncxURI, "text"))
	}
	src := ""
	if content := point.childByTag(ncxURI, "content"); content != nil {
		src = content.attrOr("src", "")
	}
	var children []navEntry
	for _, node := range point.childrenByTag(ncxURI, "navPoint") {
		if c := parseNavPoints(node, base); c != nil {
			children = append(children, *c)
		}
	}
	if src == "" && len(children) == 0 {
		return nil
	}
	entryLabel := label
	if entryLabel == "" {
		entryLabel = src
	}
	if entryLabel == "" {
		entryLabel = "Untitled"
	}
	href := ""
	if src != "" {
		href = hrefWithFragment(base, src)
	}
	return &navEntry{label: entryLabel, href: href, children: children}
}

// hrefWithFragment 逐行复刻 core.href_with_fragment。
func hrefWithFragment(base, href string) string {
	clean := href
	fragment := ""
	sep := false
	if i := strings.IndexByte(href, '#'); i >= 0 {
		clean = href[:i]
		fragment = href[i+1:]
		sep = true
	}
	p := ""
	if clean != "" {
		p = pyNormPath(pyJoin(base, clean))
	}
	if sep {
		return p + "#" + fragment
	}
	return p
}

// spineEntries 逐行复刻 core.spine_entries。
func spineEntries(root *xmlElem) []navEntry {
	// Python dict 允许 None 键（manifest item 无 id）；用不可能出现在
	// 真实 id 里的哨兵键复刻。
	const noneKey = "\x00"
	byID := map[string]string{}
	if manifest := root.childByTag(opfURI, "manifest"); manifest != nil {
		for _, item := range manifest.childrenByTag(opfURI, "item") {
			id, ok := item.getAttr("id")
			if !ok {
				id = noneKey
			}
			byID[id] = item.attrOr("href", "")
		}
	}
	var entries []navEntry
	spine := root.childByTag(opfURI, "spine")
	if spine == nil {
		return entries
	}
	for _, ref := range spine.childrenByTag(opfURI, "itemref") {
		idref, ok := ref.getAttr("idref")
		if !ok {
			idref = noneKey
		}
		href := byID[idref]
		if href == "" {
			continue
		}
		label := pyBasename(href)
		if i := strings.LastIndexByte(label, '.'); i >= 0 {
			label = label[:i]
		}
		if label == "" {
			label = href
		}
		entries = append(entries, navEntry{label: label, href: href})
	}
	return entries
}

// navCount 逐行复刻 core.nav_count。
func navCount(entries []navEntry) int {
	total := 0
	for _, entry := range entries {
		total += 1 + navCount(entry.children)
	}
	return total
}

// packageTitle 复刻 core.package_title。
func packageTitle(root *xmlElem) string {
	if titles := root.descendantsByTag(dcURI, "title"); len(titles) > 0 {
		if s := pyTextContent(titles[0]); s != "" {
			return s
		}
	}
	return "目录"
}

// packageLanguage 复刻 core.package_language。
func packageLanguage(root *xmlElem) string {
	if langs := root.descendantsByTag(dcURI, "language"); len(langs) > 0 {
		if s := pyTextContent(langs[0]); s != "" {
			return s
		}
	}
	return "und"
}

type landmark struct {
	epubType string
	label    string
	href     string
}

// guideLandmarks 逐行复刻 core.guide_landmarks。
func guideLandmarks(root *xmlElem) []landmark {
	guide := root.childByTag(opfURI, "guide")
	if guide == nil {
		return nil
	}
	var out []landmark
	for _, ref := range guide.childrenByTag(opfURI, "reference") {
		href := ref.attrOr("href", "")
		guideType := ref.attrOr("type", "")
		epubType, ok := guideTypeToEpub[guideType]
		if href == "" || !ok {
			continue
		}
		label := guideType
		if v, ok2 := ref.getAttr("title"); ok2 {
			label = v
		}
		out = append(out, landmark{epubType: epubType, label: label, href: href})
	}
	return out
}

// renderNavItems 逐行复刻 core.render_nav_items。
func renderNavItems(entries []navEntry, indent string) string {
	var lines []string
	for _, entry := range entries {
		href := saxEscapeAttr(entry.href)
		label := saxEscape(entry.label)
		if entry.href != "" {
			lines = append(lines, indent+`<li><a href="`+href+`">`+label+`</a>`)
		} else {
			lines = append(lines, indent+`<li><span>`+label+`</span>`)
		}
		if len(entry.children) > 0 {
			lines = append(lines, indent+`  <ol>`)
			lines = append(lines, renderNavItems(entry.children, indent+"    "))
			lines = append(lines, indent+`  </ol>`)
		}
		lines = append(lines, indent+`</li>`)
	}
	return strings.Join(lines, "\n")
}

// buildNavXHTML 逐字节复刻 core.build_nav_xhtml 的模板。
func buildNavXHTML(root *xmlElem, entries []navEntry) []byte {
	lang := saxEscapeAttr(packageLanguage(root))
	title := saxEscape(packageTitle(root))
	items := renderNavItems(entries, "        ")
	landmarkBlock := ""
	if landmarks := guideLandmarks(root); len(landmarks) > 0 {
		var lis []string
		for _, lm := range landmarks {
			lis = append(lis, `        <li><a epub:type="`+saxEscapeAttr(lm.epubType)+
				`" href="`+saxEscapeAttr(lm.href)+`">`+saxEscape(lm.label)+`</a></li>`)
		}
		landmarkBlock = "\n" +
			`    <nav epub:type="landmarks" hidden="hidden" id="landmarks">` + "\n" +
			"      <h2>Landmarks</h2>\n" +
			"      <ol>\n" +
			strings.Join(lis, "\n") + "\n" +
			"      </ol>\n" +
			"    </nav>"
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE html>` + "\n")
	b.WriteString(`<html xmlns="` + xhtmlURI + `" xmlns:epub="` + opsURI + `" xml:lang="` + lang + `" lang="` + lang + `">` + "\n")
	b.WriteString("  <head>\n")
	b.WriteString("    <title>" + title + "目录</title>\n")
	b.WriteString("  </head>\n")
	b.WriteString("  <body>\n")
	b.WriteString(`    <nav epub:type="toc" id="toc">` + "\n")
	b.WriteString("      <h1>" + title + "</h1>\n")
	b.WriteString("      <ol>\n")
	b.WriteString(items)
	b.WriteString("\n      </ol>\n")
	b.WriteString("    </nav>" + landmarkBlock + "\n")
	b.WriteString("  </body>\n")
	b.WriteString("</html>\n")
	return []byte(b.String())
}

// containsString 是小工具，避免为只读检查引入 slices 依赖噪音。
func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
