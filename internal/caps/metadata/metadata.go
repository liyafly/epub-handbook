// Package metadata 迁移 epub.metadata.edit（scripts/epub_metadata_edit_harness.py
// + scripts/epub_package/metadata.py 的 write_metadata）：对既有 EPUB 的
// OPF metadata 做保守字段更新。
//
// 字节策略（与 Python 的 ElementTree 整树重序列化不同）：全部修改以
// []editset.Edit 作用于源 OPF 字节 —— 文本替换、属性插入/值替换、
// 被删元素的 [元素+tail] 区间删除、新元素在 </metadata> 前的一次性
// 插入。语义与 Python 的树变换一致（set_titles / set_dc_text 逐行
// 复刻）；最终 OPF 字节语义等价但保留原有格式 —— 这同时保证
// dcterms:modified 一字不动（deepcopy 语义 = 不碰那段字节）。
package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// CapabilityID 是契约 id（contracts/capabilities/v1/epub.metadata.edit.json）。
const CapabilityID = "epub.metadata.edit"

// ErrPackageTool 对应 Python 的 PackageToolError（errors.Is 可判）。
var ErrPackageTool = errors.New("epub.metadata.edit: conservative package operation failed")

type toolError struct{ msg string }

func (e *toolError) Error() string   { return e.msg }
func (e *toolError) Is(t error) bool { return t == ErrPackageTool }

func toolErrf(format string, a ...any) error {
	return &toolError{msg: fmt.Sprintf(format, a...)}
}

const canonicalMimetype = "application/epub+zip"

// Params 是 metadata.edit 的参数。
type Params struct {
	// MetadataJSON 是 JSON 对象文本（键序保持 Python json.loads 的插入序），
	// 值均为字符串；非法形状报 "metadata JSON must be an object of string fields"。
	MetadataJSON string
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

// appendedElem 是逻辑上追加到 metadata 末尾的元素（序列化对齐 ET：
// 文本为空 → <tag attrs />；属性键序 = Python dict 插入序）。
type appendedElem struct {
	ns    string
	local string
	attrs [][2]string
	text  string
}

func (a appendedElem) serialize() string {
	prefix := ""
	if a.ns == dcURI {
		prefix = "dc:"
	}
	var b strings.Builder
	b.WriteString("<" + prefix + a.local)
	for _, kv := range a.attrs {
		b.WriteString(" " + kv[0] + `="` + attribEscape(kv[1]) + `"`)
	}
	if a.text == "" {
		b.WriteString(" />")
		return b.String()
	}
	b.WriteString(">" + cdataEscape(a.text) + "</" + prefix + a.local + ">")
	return b.String()
}

// liveChild 是 metadata 直接子元素的活视图成员：原始节点或追加元素。
type liveChild struct {
	node     *opf.SpanNode
	appended *appendedElem
}

func (c liveChild) tag() (string, string) {
	if c.appended != nil {
		return c.appended.ns, c.appended.local
	}
	return c.node.Name.Space, c.node.Name.Local
}

func (c liveChild) attr(name string) string {
	if c.appended != nil {
		for _, kv := range c.appended.attrs {
			if kv[0] == name {
				return kv[1]
			}
		}
		return ""
	}
	v, _ := c.node.AttrByLocal("", name)
	return v
}

func (c liveChild) text() string {
	if c.appended != nil {
		return c.appended.text
	}
	return c.node.Text
}

// childView 复刻 Python 在 metadata 元素上边遍历边增删的 find / findall
// 语义：活列表 = 原始子节点（去删除）+ 追加元素（保持追加序）。
type childView struct {
	path     string
	data     []byte
	original []*opf.SpanNode
	removed  map[*opf.SpanNode]bool
	appended []appendedElem
}

func newChildView(path string, data []byte, meta *opf.SpanNode) *childView {
	return &childView{path: path, data: data, original: meta.Kids, removed: map[*opf.SpanNode]bool{}}
}

func (v *childView) remove(n *opf.SpanNode) { v.removed[n] = true }

func (v *childView) live() []liveChild {
	var out []liveChild
	for _, n := range v.original {
		if !v.removed[n] {
			out = append(out, liveChild{node: n})
		}
	}
	for i := range v.appended {
		out = append(out, liveChild{appended: &v.appended[i]})
	}
	return out
}

// findMeta 复刻 meta.find('opf:meta[@refines="#X"][@property="Y"]')。
func (v *childView) findMeta(refines, property string) *liveChild {
	for i, c := range v.live() {
		ns, local := c.tag()
		if ns != opfURI || local != "meta" {
			continue
		}
		if c.attr("refines") == refines && c.attr("property") == property {
			return &v.live()[i]
		}
	}
	return nil
}

// findAllMeta 复刻 remove_title_type_meta 用的 findall（调用点都在追加
// main_meta 之前，只扫原始节点即可；调用前快照）。
func (v *childView) findAllMeta(refines, property string) []*opf.SpanNode {
	var out []*opf.SpanNode
	for _, c := range v.original {
		if v.removed[c] {
			continue
		}
		if c.Name.Space != opfURI || c.Name.Local != "meta" {
			continue
		}
		ref, _ := c.AttrByLocal("", "refines")
		prop, _ := c.AttrByLocal("", "property")
		if ref == refines && prop == property {
			out = append(out, c)
		}
	}
	return out
}

// findAllDcTitle 复刻 meta.findall("dc:title", OPF_NS)（快照语义）。
func (v *childView) findAllDcTitle() []*opf.SpanNode {
	var out []*opf.SpanNode
	for _, c := range v.original {
		if c.Name.Space == dcURI && c.Name.Local == "title" {
			out = append(out, c)
		}
	}
	return out
}

// findDc 复刻 meta.find("dc:tag", OPF_NS)（活列表语义）。
func (v *childView) findDc(tag string) *liveChild {
	live := v.live()
	for i := range live {
		ns, local := live[i].tag()
		if ns == dcURI && local == tag {
			return &live[i]
		}
	}
	return nil
}

// parseOrderedFields 复刻 json.loads + "object of string fields" 校验，
// 键序按插入序；重复键保持首现位置、值取后者（Python dict 语义）。
func parseOrderedFields(raw string) ([][2]string, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, toolErrf("metadata JSON must be an object of string fields")
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, toolErrf("metadata JSON must be an object of string fields")
	}
	var fields [][2]string
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, toolErrf("metadata JSON must be an object of string fields")
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, toolErrf("metadata JSON must be an object of string fields")
		}
		valTok, err := dec.Token()
		if err != nil {
			return nil, toolErrf("metadata JSON must be an object of string fields")
		}
		val, ok := valTok.(string)
		if !ok {
			return nil, toolErrf("metadata JSON must be an object of string fields")
		}
		found := false
		for i := range fields {
			if fields[i][0] == key {
				fields[i][1] = val
				found = true
				break
			}
		}
		if !found {
			fields = append(fields, [2]string{key, val})
		}
	}
	if _, err := dec.Token(); err != nil { // 收 '}' 或报 trailing junk
		return nil, toolErrf("metadata JSON must be an object of string fields")
	}
	if dec.More() {
		return nil, toolErrf("metadata JSON must be an object of string fields")
	}
	return fields, nil
}

func lookupField(fields [][2]string, key string) (string, bool) {
	for _, kv := range fields {
		if kv[0] == key {
			return kv[1], true
		}
	}
	return "", false
}

// Run 执行 metadata.edit（SPEC §6.1 三段式）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	inputPath := b.InputPath()
	names := b.OriginalNames()
	read := b.Original
	namesSet := make(map[string]bool, len(names))
	for _, n := range names {
		namesSet[n] = true
	}

	fields, err := parseOrderedFields(p.MetadataJSON)
	if err != nil {
		return failedResult(err.Error())
	}

	if err := ensureNoEncryption(names, "metadata-write"); err != nil {
		return failedResult(err.Error())
	}
	pkg, err := readPackage(namesSet, read)
	if err != nil {
		return failedResult(err.Error())
	}
	opfData, err := read(pkg.opfPath)
	if err != nil {
		return failedResult(err.Error())
	}
	tree, err := opf.ScanSpanTree(opfData)
	if err != nil {
		return failedResult(fmt.Sprintf("%s: XML parse failed: %v", pkg.opfPath, err))
	}
	metaNode := tree.ChildByLocal(opfURI, "metadata")
	if metaNode == nil {
		return failedResult("OPF missing metadata")
	}
	view := newChildView(pkg.opfPath, opfData, metaNode)

	rep := legacyReport{
		Operation: "metadata-write",
		Input:     strPtr(inputPath),
		Inputs:    []string{},
		Output:    strPtr(p.Output),
		Outputs:   []string{},
		OPF:       pkg.opfPath,
		Warnings:  []string{},
	}

	var edits []editset.Edit
	appendElem := func(e appendedElem) { view.appended = append(view.appended, e) }

	// 1. 扫描/决策（set_titles → set_dc_text，顺序对齐 Python）。
	titleVal, hasTitle := lookupField(fields, "title")
	subtitleVal, hasSubtitle := lookupField(fields, "subtitle")
	if hasTitle || hasSubtitle {
		changed, titleEdits, err := setTitles(view, titleVal, hasTitle, subtitleVal, hasSubtitle, appendElem)
		if err != nil {
			return failedResult(err.Error())
		}
		rep.FieldsUpdated += changed
		edits = append(edits, titleEdits...)
	}
	for _, field := range fields {
		var tag string
		switch field[0] {
		case "author":
			tag = "creator"
		case "language", "publisher", "description", "identifier", "rights":
			tag = field[0]
		default:
			continue
		}
		changed, dcEdits := setDCText(view, tag, field[1])
		rep.FieldsUpdated += changed
		edits = append(edits, dcEdits...)
	}

	// 追加元素统一插在 </metadata> 之前（ET.SubElement 语义）。
	if len(view.appended) > 0 {
		var sb strings.Builder
		for _, e := range view.appended {
			sb.WriteString(e.serialize())
		}
		edits = append(edits, editset.Insert(pkg.opfPath, int64(metaNode.Close.Start), []byte(sb.String())))
	}

	// mimetype 规范化（Python write_epub 总是重写并 STORED）。
	if cur, err := b.Current("mimetype"); err == nil {
		if string(cur) != canonicalMimetype {
			edits = append(edits, editset.Replace("mimetype", 0, int64(len(cur)), []byte(canonicalMimetype)))
		}
	} else {
		edits = append(edits, editset.Replace("mimetype", 0, 0, []byte(canonicalMimetype)))
	}

	// 2. 应用（唯一写点）。
	if len(edits) > 0 {
		if err := b.Apply(edits); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
	}

	// 3. 报告。
	res := report.Result{
		Capability: CapabilityID,
		Status:     report.StatusComplete,
		Facts: map[string]any{
			"opf":           rep.OPF,
			"output":        p.Output,
			"fieldsUpdated": rep.FieldsUpdated,
		},
		Events: []report.Event{{
			Step: "metadata-write", Status: "completed",
			Message: fmt.Sprintf("fields_updated=%d", rep.FieldsUpdated),
		}},
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

// setTitles 逐行复刻 metadata.set_titles。
func setTitles(view *childView, titleVal string, hasTitle bool, subtitleVal string, hasSubtitle bool, appendElem func(appendedElem)) (int, []editset.Edit, error) {
	changed := 0
	var edits []editset.Edit
	titles := view.findAllDcTitle() // findall 快照

	var main *opf.SpanNode
	mainIsAppended := false
	if len(titles) > 0 {
		main = titles[0]
	} else {
		appendElem(appendedElem{ns: dcURI, local: "title"})
		mainIsAppended = true
	}
	mainID := ""
	mainText := ""
	if main != nil {
		mainID, _ = main.AttrByLocal("", "id")
		mainText = main.Text
	}

	// mergeIntoAppended 把 main 的 id/text 变更并入刚追加的 dc:title。
	mergeIntoAppended := func(setID, text string, textChanged bool) {
		for i := range view.appended {
			a := &view.appended[i]
			if a.ns == dcURI && a.local == "title" {
				if setID != "" {
					a.attrs = append(a.attrs, [2]string{"id", setID})
				}
				if textChanged {
					a.text = text
				}
				return
			}
		}
	}

	if mainID == "" {
		mainID = "main-title"
		if main != nil {
			if idx := main.AttrIndex("", "id"); idx >= 0 {
				// 已有 id 属性但值为空：值替换。
				if span, quote, ok := opf.RawAttrValueSpan(view.data, main, idx); ok {
					esc := attrEscapeFor(quote, "main-title")
					edits = append(edits, editset.Replace(view.path, int64(span.Start), int64(span.Len()), []byte(esc)))
				}
			} else {
				pos := main.Open.End - 1
				if main.SelfClose {
					pos = main.Open.End - 2
				}
				edits = append(edits, editset.Insert(view.path, int64(pos), []byte(` id="main-title"`)))
			}
		} else {
			mergeIntoAppended(mainID, "", false)
		}
	}

	if hasTitle && mainText != titleVal {
		if main != nil {
			if e, ok := textReplaceEdit(view, main, titleVal); ok {
				edits = append(edits, e)
			}
		} else {
			mergeIntoAppended("", titleVal, true)
		}
		changed++
	}
	_ = mainIsAppended

	// remove_title_type_meta(main id)：逐个删除（元素 + tail）。
	for _, m := range view.findAllMeta("#"+mainID, "title-type") {
		view.remove(m)
		edits = append(edits, elementRemovalEdit(view, m))
	}
	// main_meta 追加。
	appendElem(appendedElem{
		ns:    opfURI,
		local: "meta",
		attrs: [][2]string{{"refines", "#" + mainID}, {"property", "title-type"}},
		text:  "main",
	})

	if hasSubtitle {
		for _, titleElem := range titles[1:] {
			titleID, _ := titleElem.AttrByLocal("", "id")
			if titleID == "" {
				continue
			}
			refine := view.findMeta("#"+titleID, "title-type")
			if refine != nil && refine.node != nil && strings.TrimSpace(refine.node.IterText()) == "subtitle" {
				view.remove(titleElem)
				edits = append(edits, elementRemovalEdit(view, titleElem))
				view.remove(refine.node)
				edits = append(edits, elementRemovalEdit(view, refine.node))
			}
		}
		if subtitleVal != "" {
			appendElem(appendedElem{ns: dcURI, local: "title", attrs: [][2]string{{"id", "subtitle"}}, text: subtitleVal})
			appendElem(appendedElem{
				ns:    opfURI,
				local: "meta",
				attrs: [][2]string{{"refines", "#subtitle"}, {"property", "title-type"}},
				text:  "subtitle",
			})
			changed++
		}
	}
	return changed, edits, nil
}

// setDCText 逐行复刻 metadata.set_dc_text（首元素命中或末尾追加）。
func setDCText(view *childView, tag, value string) (int, []editset.Edit) {
	elem := view.findDc(tag)
	if elem == nil {
		// Python：追加后 before == "" != value（value 非空时计一次变化）。
		view.appended = append(view.appended, appendedElem{ns: dcURI, local: tag, text: value})
		if value != "" {
			return 1, nil
		}
		return 0, nil
	}
	before := elem.text()
	if before == value {
		return 0, nil
	}
	if elem.node != nil {
		if e, ok := textReplaceEdit(view, elem.node, value); ok {
			return 1, []editset.Edit{e}
		}
		return 0, nil
	}
	// 追加元素命中（值相同才可能，已在上方返回；防御）。
	for i := range view.appended {
		a := &view.appended[i]
		if a.ns == dcURI && a.local == tag {
			a.text = value
			break
		}
	}
	return 1, nil
}

// textReplaceEdit 生成把元素首段文本替换为 value 的编辑。
func textReplaceEdit(view *childView, n *opf.SpanNode, value string) (editset.Edit, bool) {
	if n.SelfClose {
		// <tag ... /> → <tag ...>text</tag>：整体替换开标签。
		raw := string(view.data[n.Open.Start:n.Open.End])
		inner := strings.TrimRight(raw[1:len(raw)-2], " \t\r\n") // 去掉 '<' 与 '/>'
		name := inner
		if i := strings.IndexAny(name, " \t\r\n"); i >= 0 {
			name = name[:i]
		}
		ser := "<" + inner + ">" + cdataEscape(value) + "</" + name + ">"
		return editset.Replace(view.path, int64(n.Open.Start), int64(n.Open.Len()), []byte(ser)), true
	}
	replacement := []byte(cdataEscape(value))
	if n.TextSpan.IsZero() {
		at := int64(n.Open.End)
		if len(n.Kids) == 0 && !n.Close.IsZero() {
			at = int64(n.Close.Start)
		}
		return editset.Insert(view.path, at, replacement), true
	}
	return editset.Replace(view.path, int64(n.TextSpan.Start), int64(n.TextSpan.Len()), replacement), true
}

// elementRemovalEdit 生成删除元素（连同 tail）的编辑 —— ET 的 remove
// 会一并丢弃被删元素的 tail 文本。
func elementRemovalEdit(view *childView, n *opf.SpanNode) editset.Edit {
	end := n.End()
	tail := n.TailAfter(view.data)
	if tail.End > end {
		end = tail.End
	}
	return editset.Replace(view.path, int64(n.Open.Start), int64(end-n.Open.Start), []byte{})
}

func attrEscapeFor(quote byte, value string) string {
	if quote == '\'' {
		return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(value,
			"&", "&amp;"), "<", "&lt;"), ">", "&gt;"), "'", "&#39;"), `"`, "&quot;")
	}
	return attribEscape(value)
}
