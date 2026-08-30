// Package cover 迁移 epub.cover.replace（scripts/epub_cover_replace_harness.py
// + scripts/epub_package/cover.py 的 replace_cover）：替换 EPUB 封面图片。
//
// 行为逐行复刻 Python：旧封面引用重写（manifest href / XHTML / CSS）、
// cover_raster_dimensions 的 PNG/JPEG 尺寸解析、resize_svg_cover_pages
// 的 SVG 封面页尺寸对齐、manifest properties 更新、meta name="cover"
// 重挂。字节策略：OPF 以 scan/opf 区间树做字节区间编辑（原格式与
// dcterms:modified 保留）；XHTML/CSS 引用重写用与 Python 相同的正则
// 语义扫描器（RE2 无反向引用，手工实现）；未变化 entry 原样透传。
package cover

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// CapabilityID 是契约 id（contracts/capabilities/v1/epub.cover.replace.json）。
const CapabilityID = "epub.cover.replace"

// ErrPackageTool 对应 Python 的 PackageToolError（errors.Is 可判）。
var ErrPackageTool = errors.New("epub.cover.replace: conservative package operation failed")

type toolError struct{ msg string }

func (e *toolError) Error() string   { return e.msg }
func (e *toolError) Is(t error) bool { return t == ErrPackageTool }

func toolErrf(format string, a ...any) error {
	return &toolError{msg: fmt.Sprintf(format, a...)}
}

const canonicalMimetype = "application/epub+zip"

// Params 是 cover.replace 的参数。
type Params struct {
	// Cover 是替换封面图片的文件路径（.jpg/.jpeg/.png/.svg/.webp/.gif）。
	Cover string
	// Output 是输出路径（仅进入 legacy 报告字段；本包不落盘）。
	Output string
	// LegacyReport 输出 Python OperationReport 形状的 JSON。
	LegacyReport bool
}

// legacyReport 对齐 models.OperationReport（键序 = dataclass 字段序）。
type legacyReport struct {
	Operation        string   `json:"operation"`
	Input            *string  `json:"input"`
	Inputs           []string `json:"inputs"`
	Output           *string  `json:"output"`
	Outputs          []string `json:"outputs"`
	OPF              string   `json:"opf"`
	MergedItems      int      `json:"merged_items"`
	RenamedResources int      `json:"renamed_resources"`
	SegmentsCreated  int      `json:"segments_created"`
	FieldsUpdated    int      `json:"fields_updated"`
	CoverPath        string   `json:"cover_path"`
	Warnings         []string `json:"warnings"`
}

// failedResult 复刻 Python harness 的失败语义：不产出报告 JSON，
// Status=failed，错误措辞原样进入 findings。
func failedResult(msg string) (report.Result, error) {
	return report.Result{
		Capability: CapabilityID,
		Status:     report.StatusFailed,
		Findings:   []report.Finding{{Level: "error", ID: "package.refused", Title: msg}},
	}, nil
}

// mediaTypeForCover 复刻 cover.media_type_for_cover。
func mediaTypeForCover(coverPath string) string {
	ext := strings.ToLower(pathExt(coverPath))
	if ext == ".jpeg" {
		return "image/jpeg"
	}
	if mt, ok := imageMediaByExt[ext]; ok {
		return mt
	}
	return "image/jpeg"
}

// coverRasterDimensions 复刻 core.cover_raster_dimensions：
// 不引入图片依赖的 PNG/JPEG 像素尺寸解析。
func coverRasterDimensions(data []byte) (int, int, bool) {
	if len(data) >= 24 && binary.BigEndian.Uint32(data[12:16]) == 0x49484452 &&
		string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return int(binary.BigEndian.Uint32(data[16:20])), int(binary.BigEndian.Uint32(data[20:24])), true
	}
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0, 0, false
	}
	sodMarkers := map[byte]bool{0xC0: true, 0xC1: true, 0xC2: true, 0xC3: true, 0xC5: true, 0xC6: true, 0xC7: true,
		0xC9: true, 0xCA: true, 0xCB: true, 0xCD: true, 0xCE: true, 0xCF: true}
	standalone := map[byte]bool{0xD8: true, 0xD9: true}
	index := 2
	for index+9 <= len(data) {
		if data[index] != 0xFF {
			index++
			continue
		}
		marker := data[index+1]
		if sodMarkers[marker] {
			return int(binary.BigEndian.Uint16(data[index+7 : index+9])),
				int(binary.BigEndian.Uint16(data[index+5 : index+7])), true
		}
		if standalone[marker] {
			index += 2
			continue
		}
		if index+4 > len(data) {
			return 0, 0, false
		}
		length := int(binary.BigEndian.Uint16(data[index+2 : index+4]))
		if length < 2 {
			return 0, 0, false
		}
		index += 2 + length
	}
	return 0, 0, false
}

// Run 执行 cover.replace（SPEC §6.1 三段式）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	inputPath := b.InputPath()
	names := b.OriginalNames()
	read := b.Original
	namesSet := make(map[string]bool, len(names))
	for _, n := range names {
		namesSet[n] = true
	}

	if err := ensureNoEncryption(names, "replace-cover"); err != nil {
		return failedResult(err.Error())
	}
	if _, err := os.Stat(p.Cover); err != nil {
		return failedResult(fmt.Sprintf("cover image not found: %s", p.Cover))
	}
	pkg, _, manifestNode, metaNode, err := readPackage(namesSet, read)
	if err != nil {
		return failedResult(err.Error())
	}
	opfData, err := read(pkg.opfPath)
	if err != nil {
		return failedResult(err.Error())
	}
	if metaNode == nil {
		return failedResult("OPF missing metadata")
	}
	if manifestNode == nil {
		return failedResult(fmt.Sprintf("%s: OPF missing manifest", pkg.opfPath))
	}

	coverID := coverItemID(manifestNode, metaNode)
	opfDir := pyDirname(pkg.opfPath)
	ext := strings.ToLower(pathExt(p.Cover))
	if ext == "" {
		ext = ".jpg"
	}
	if ext == ".jpeg" {
		ext = ".jpg"
	}
	newRelHref := "Images/cover" + ext
	newArchivePath, err := validateArchivePath(pathJoin(opfDir, newRelHref), "cover output")
	if err != nil {
		return failedResult(err.Error())
	}
	coverData, err := os.ReadFile(p.Cover)
	if err != nil {
		return failedResult(err.Error())
	}
	dimW, dimH, hasDims := coverRasterDimensions(coverData)

	rep := legacyReport{
		Operation: "replace-cover",
		Input:     strPtr(inputPath),
		Inputs:    []string{},
		Output:    strPtr(p.Output),
		Outputs:   []string{},
		OPF:       pkg.opfPath,
		Warnings:  []string{},
	}
	var edits []editset.Edit
	var deletes []editset.Edit
	inDeletes := map[string]bool{}

	// ---- manifest：定位旧封面、改写 cover item、移除其他 cover 项 ----
	oldCoverPaths := map[string]bool{}
	var coverItem *opf.SpanNode
	var manifestAppended []string
	for _, item := range manifestNode.Kids {
		if item.Name.Local != "item" {
			continue
		}
		itemID, _ := item.AttrByLocal("", "id")
		href, _ := item.AttrByLocal("", "href")
		props := splitProps(itemAttr(item, "properties"))
		if itemID == coverID {
			coverItem = item
			if href != "" && !pyIsExternalURI(href) {
				oldPath, rerr := resolveRelativePath(pkg.opfPath, pyURLSplit(href).path)
				if rerr != nil {
					return failedResult(rerr.Error())
				}
				oldCoverPaths[oldPath] = true
			}
			continue
		}
		lowerHref := strings.ToLower(href)
		if containsStr(props, "cover-image") ||
			(strings.Contains(lowerHref, "cover") && hasSuffixAny(lowerHref, ".jpg", ".jpeg", ".png", ".webp", ".gif")) {
			if href != "" && !pyIsExternalURI(href) {
				oldPath, rerr := resolveRelativePath(pkg.opfPath, pyURLSplit(href).path)
				if rerr != nil {
					return failedResult(rerr.Error())
				}
				oldCoverPaths[oldPath] = true
			}
			// manifest.remove(item)：元素连同 tail 一起消失。
			edits = append(edits, elementRemovalEdit(pkg.opfPath, opfData, item))
		}
	}

	newProps := "cover-image"
	if coverItem != nil {
		existing, _ := coverItem.AttrByLocal("", "properties")
		newProps = addProp(existing, "cover-image")
		if e, ok := setAttrEdit(pkg.opfPath, opfData, coverItem, "href", newRelHref); ok {
			edits = append(edits, e)
		}
		if e, ok := setAttrEdit(pkg.opfPath, opfData, coverItem, "media-type", mediaTypeForCover(p.Cover)); ok {
			edits = append(edits, e)
		}
		if e, ok := setAttrEdit(pkg.opfPath, opfData, coverItem, "properties", newProps); ok {
			edits = append(edits, e)
		}
	} else {
		manifestAppended = append(manifestAppended,
			`<item id="`+attribEscape(coverID)+`" href="`+attribEscape(newRelHref)+
				`" media-type="`+attribEscape(mediaTypeForCover(p.Cover))+`" properties="`+attribEscape(newProps)+`" />`)
	}

	// ---- entry 处置：删除旧封面、写入新封面 ----
	for oldPath := range oldCoverPaths {
		if oldPath == newArchivePath {
			continue
		}
		if namesSet[oldPath] && !inDeletes[oldPath] {
			deletes = append(deletes, editset.Delete(oldPath))
			inDeletes[oldPath] = true
		}
	}
	switch {
	case inDeletes[newArchivePath]:
		edits = append(edits, editset.Replace(newArchivePath, 0, 0, coverData))
	case namesSet[newArchivePath]:
		cur, cerr := read(newArchivePath)
		if cerr == nil {
			if !bytesEqual(cur, coverData) {
				edits = append(edits, editset.Replace(newArchivePath, 0, int64(len(cur)), coverData))
			}
		} else {
			edits = append(edits, editset.Replace(newArchivePath, 0, 0, coverData))
		}
	default:
		edits = append(edits, editset.Replace(newArchivePath, 0, 0, coverData))
	}

	// ---- 引用重写 + SVG 封面页对齐（仅存在旧封面引用时） ----
	if len(oldCoverPaths) > 0 {
		pathMap := map[string]string{}
		for oldPath := range oldCoverPaths {
			pathMap[oldPath] = newArchivePath
		}
		for _, name := range names {
			if name == pkg.opfPath || name == newArchivePath {
				continue
			}
			data, rerr := read(name)
			if rerr != nil {
				continue
			}
			transformed := transformResource(data, name, name, pathMap, namesSet)
			if hasDims {
				transformed = resizeSVGCoverPages(transformed, name, newArchivePath, dimW, dimH)
			}
			if !bytesEqual(transformed, data) {
				edits = append(edits, editset.Replace(name, 0, int64(len(data)), transformed))
			}
		}
	}

	// ---- metadata：移除全部 meta name="cover" 并重挂 ----
	var metaAppended []string
	for _, m := range metaNode.Kids {
		if m.Name.Space != opfURI || m.Name.Local != "meta" {
			continue
		}
		nameAttr, _ := m.AttrByLocal("", "name")
		if nameAttr == "cover" {
			edits = append(edits, elementRemovalEdit(pkg.opfPath, opfData, m))
		}
	}
	metaAppended = append(metaAppended, `<meta name="cover" content="`+attribEscape(coverID)+`" />`)
	if e, ok := appendBeforeCloseEdit(pkg.opfPath, opfData, metaNode, metaAppended); ok {
		edits = append(edits, e)
	}
	if e, ok := appendBeforeCloseEdit(pkg.opfPath, opfData, manifestNode, manifestAppended); ok {
		edits = append(edits, e)
	}

	// mimetype 规范化（Python write_epub 总是重写并 STORED）。
	if cur, err := b.Current("mimetype"); err == nil {
		if string(cur) != canonicalMimetype {
			edits = append(edits, editset.Replace("mimetype", 0, int64(len(cur)), []byte(canonicalMimetype)))
		}
	} else {
		edits = append(edits, editset.Replace("mimetype", 0, 0, []byte(canonicalMimetype)))
	}

	// 2. 应用（唯一写点）：先删后建，避免同名路径的删除与内容编辑冲突。
	if len(deletes) > 0 {
		if err := b.Apply(deletes); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
	}
	if len(edits) > 0 {
		if err := b.Apply(edits); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
	}

	// 3. 报告。
	rep.CoverPath = newArchivePath
	res := report.Result{
		Capability: CapabilityID,
		Status:     report.StatusComplete,
		Facts: map[string]any{
			"opf":       rep.OPF,
			"output":    p.Output,
			"coverPath": rep.CoverPath,
		},
		Events: []report.Event{{
			Step: "replace-cover", Status: "completed",
			Message: fmt.Sprintf("cover_path=%s dims=%dx%d", rep.CoverPath, dimW, dimH),
		}},
		Renames: nilIfEmpty(renamesFrom(oldCoverPaths, newArchivePath)),
	}
	if p.LegacyReport {
		raw, err := report.MarshalLegacy(rep)
		if err != nil {
			return report.Result{}, err
		}
		res.Facts["legacyReport"] = json.RawMessage(raw)
	}
	return res, nil
}

func renamesFrom(oldCoverPaths map[string]bool, newArchivePath string) map[string]string {
	out := map[string]string{}
	for oldPath := range oldCoverPaths {
		if oldPath != newArchivePath {
			out[oldPath] = newArchivePath
		}
	}
	return out
}

// coverItemID 复刻 cover.cover_item_id。
func coverItemID(manifestNode, metaNode *opf.SpanNode) string {
	if coverMeta := findCoverMeta(metaNode); coverMeta != nil {
		if content, _ := coverMeta.AttrByLocal("", "content"); content != "" {
			return content
		}
	}
	for _, item := range manifestNode.Kids {
		if item.Name.Local != "item" {
			continue
		}
		props := splitProps(itemAttr(item, "properties"))
		if containsStr(props, "cover-image") {
			if id, ok := item.AttrByLocal("", "id"); ok && id != "" {
				return id
			}
			return "cover-image"
		}
	}
	return "cover-image"
}

// findCoverMeta 复刻 meta.find('opf:meta[@name="cover"]')（直接子元素）。
func findCoverMeta(metaNode *opf.SpanNode) *opf.SpanNode {
	for _, m := range metaNode.Kids {
		if m.Name.Space != opfURI || m.Name.Local != "meta" {
			continue
		}
		if nameAttr, _ := m.AttrByLocal("", "name"); nameAttr == "cover" {
			return m
		}
	}
	return nil
}

// setAttrEdit 生成把元素属性设为 value 的字节编辑（已相等 → 无编辑；
// 缺失 → 开标签末尾插入，对齐 ET 的 set 追加语义）。
func setAttrEdit(path string, data []byte, n *opf.SpanNode, name, value string) (editset.Edit, bool) {
	if idx := n.AttrIndex("", name); idx >= 0 {
		if n.Attrs[idx].Value == value {
			return editset.Edit{}, false
		}
		span, quote, ok := opf.RawAttrValueSpan(data, n, idx)
		if !ok {
			return editset.Edit{}, false
		}
		return editset.Replace(path, int64(span.Start), int64(span.Len()), []byte(attrEscapeFor(quote, value))), true
	}
	pos := n.Open.End - 1
	if n.SelfClose {
		pos = n.Open.End - 2
	}
	return editset.Insert(path, int64(pos), []byte(` `+name+`="`+attribEscape(value)+`"`)), true
}

// appendBeforeCloseEdit 在元素的结束标签前追加序列化内容；自闭合元素
// 改写为 open + content + close。
func appendBeforeCloseEdit(path string, data []byte, n *opf.SpanNode, pieces []string) (editset.Edit, bool) {
	if len(pieces) == 0 {
		return editset.Edit{}, false
	}
	content := strings.Join(pieces, "")
	if n.SelfClose || n.Close.IsZero() {
		// <tag ... /> → <tag ...>content</tag>
		raw := string(data[n.Open.Start:n.Open.End])
		inner := strings.TrimRight(raw[1:len(raw)-2], " \t\r\n") // 去 '<' 与 '/>'
		name := inner
		if i := strings.IndexAny(name, " \t\r\n"); i >= 0 {
			name = name[:i]
		}
		ser := "<" + inner + ">" + content + "</" + name + ">"
		return editset.Replace(path, int64(n.Open.Start), int64(n.Open.Len()), []byte(ser)), true
	}
	return editset.Insert(path, int64(n.Close.Start), []byte(content)), true
}

// elementRemovalEdit 删除元素（连同 tail，对齐 ET remove 语义）。
func elementRemovalEdit(path string, data []byte, n *opf.SpanNode) editset.Edit {
	end := n.End()
	tail := n.TailAfter(data)
	if tail.End > end {
		end = tail.End
	}
	return editset.Replace(path, int64(n.Open.Start), int64(end-n.Open.Start), []byte{})
}

func bytesEqual(a, b []byte) bool {
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

func hasSuffixAny(s string, suffixes ...string) bool {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

func containsStr(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func nilIfEmpty(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	return m
}

func strPtr(s string) *string { return &s }
