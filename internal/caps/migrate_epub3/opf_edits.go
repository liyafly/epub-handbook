package migrateepub3

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/editset"
	opfscan "github.com/liyafly/epub-handbook/internal/scan/opf"
)

// opfEditSnapshot keeps the source spans paired with the original semantic
// nodes while the capability computes its desired package tree.
type opfEditSnapshot struct {
	path           string
	original       []byte
	source         []byte
	baseOffset     int
	direct         bool
	nodes          []*xmlElem
	originalSet    map[*xmlElem]bool
	originalAttrs  map[*xmlElem][]xmlAttr
	originalChild  map[*xmlElem][]*xmlElem
	originalParent map[*xmlElem]*xmlElem
	originalText   map[*xmlElem]string
	originalTail   map[*xmlElem]string
	spans          map[*xmlElem]*opfscan.SpanNode
}

func captureOPFEditSnapshot(path string, data []byte, root *xmlElem) (*opfEditSnapshot, error) {
	decoded, err := xmlSourceToUTF8(data)
	if err != nil {
		return nil, convErrf("%s: XML parse failed: %v", path, err)
	}
	source := []byte(decoded)
	originalBody := data
	baseOffset := 0
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		originalBody = data[3:]
		baseOffset = 3
	}
	spanRoot, err := opfscan.ScanSpanTree(data)
	if err != nil {
		return nil, convErrf("%s: XML span scan failed: %v", path, err)
	}
	snapshot := &opfEditSnapshot{
		path: path, original: data, source: source, baseOffset: baseOffset,
		direct: bytes.Equal(source, originalBody), originalSet: map[*xmlElem]bool{},
		originalAttrs: map[*xmlElem][]xmlAttr{}, originalChild: map[*xmlElem][]*xmlElem{},
		originalParent: map[*xmlElem]*xmlElem{}, originalText: map[*xmlElem]string{},
		originalTail: map[*xmlElem]string{}, spans: map[*xmlElem]*opfscan.SpanNode{},
	}
	if err := pairOPFEditNodes(root, spanRoot, snapshot.spans); err != nil {
		return nil, convErrf("%s: OPF tree and source spans differ: %v", path, err)
	}
	snapshot.nodes = iterAll(root)
	for _, node := range snapshot.nodes {
		snapshot.originalSet[node] = true
		snapshot.originalAttrs[node] = slices.Clone(node.attrs)
		snapshot.originalChild[node] = slices.Clone(node.children)
		snapshot.originalText[node] = node.text
		snapshot.originalTail[node] = node.tail
		for _, child := range node.children {
			snapshot.originalParent[child] = node
		}
	}
	return snapshot, nil
}

func pairOPFEditNodes(tree *xmlElem, span *opfscan.SpanNode, paired map[*xmlElem]*opfscan.SpanNode) error {
	if tree == nil || span == nil || tree.ns != span.Name.Space || tree.name != span.Name.Local {
		return fmt.Errorf("element mismatch")
	}
	if len(tree.children) != len(span.Kids) {
		return fmt.Errorf("%s has %d parsed children and %d source children", tree.name, len(tree.children), len(span.Kids))
	}
	paired[tree] = span
	for i, child := range tree.children {
		if err := pairOPFEditNodes(child, span.Kids[i], paired); err != nil {
			return err
		}
	}
	return nil
}

func applyOPFTreeEdits(snapshot *opfEditSnapshot, root *xmlElem) ([]byte, error) {
	finalSet := map[*xmlElem]bool{}
	finalNodes := iterAll(root)
	for _, node := range finalNodes {
		finalSet[node] = true
	}
	groups, parentsWithChildren, err := collectOPFChildGroups(snapshot, finalNodes)
	if err != nil {
		return nil, err
	}
	var edits []editset.Edit
	removed := map[*xmlElem]bool{}
	for _, node := range snapshot.nodes {
		if !finalSet[node] {
			removed[node] = true
		}
	}
	for _, node := range snapshot.nodes {
		if !removed[node] || hasRemovedOPFAncestor(node, removed, snapshot.originalParent) {
			continue
		}
		span := snapshot.spans[node]
		edit := opfscan.RemoveElementEdit(snapshot.path, snapshot.source, span)
		edit.Offset += int64(snapshot.baseOffset)
		edits = append(edits, edit)
	}
	for _, node := range snapshot.nodes {
		if !finalSet[node] || hasRemovedOPFAncestor(node, removed, snapshot.originalParent) {
			continue
		}
		span := snapshot.spans[node]
		textChanged := node.text != snapshot.originalText[node]
		if node.tail != snapshot.originalTail[node] {
			return nil, convErrf("%s: cannot safely edit OPF tail text for %s", snapshot.path, node.name)
		}
		if span.SelfClose && (parentsWithChildren[node] || textChanged) {
			content := ""
			if textChanged {
				if parentsWithChildren[node] {
					return nil, convErrf("%s: cannot combine text and child insertion in self-closing %s", snapshot.path, node.name)
				}
				content = escapeOPFText(node.text)
			} else {
				fragments := make([]string, 0, len(node.children))
				for _, child := range node.children {
					fragment, err := serializeOPFChild(child, rawOPFPrefix(snapshot.source, span))
					if err != nil {
						return nil, err
					}
					fragments = append(fragments, fragment)
				}
				content = formatOPFInsertedChildren(snapshot, span, fragments)
			}
			replacement, err := expandOPFSelfClosingParent(snapshot, node, span, content)
			if err != nil {
				return nil, err
			}
			edits = append(edits, editset.Replace(snapshot.path,
				int64(snapshot.baseOffset+span.Open.Start), int64(span.Open.Len()), replacement))
			continue // 属性和内容合入单个 self-closing 扩展 edit。
		}
		attributeEdits, added, err := diffOPFAttributes(snapshot.path, snapshot.source, span, snapshot.originalAttrs[node], node.attrs)
		if err != nil {
			return nil, err
		}
		for _, edit := range attributeEdits {
			edit.Offset += int64(snapshot.baseOffset)
			edits = append(edits, edit)
		}
		if len(added) > 0 {
			at := span.Open.End - 1
			if span.SelfClose && at > span.Open.Start && snapshot.source[at-1] == '/' {
				at--
			}
			edits = append(edits, editset.Insert(snapshot.path, int64(snapshot.baseOffset+at), []byte(strings.Join(added, ""))))
		}
		if textChanged {
			textEdit, err := diffOPFText(snapshot, node, span)
			if err != nil {
				return nil, err
			}
			edits = append(edits, textEdit)
		}
	}
	for _, group := range groups {
		parentSpan := snapshot.spans[group.parent]
		prefix, err := opfElementPrefix(snapshot.source, parentSpan)
		if err != nil {
			return nil, err
		}
		fragments := make([]string, 0, len(group.children))
		for _, child := range group.children {
			fragment, err := serializeOPFChild(child, prefix)
			if err != nil {
				return nil, err
			}
			fragments = append(fragments, fragment)
		}
		if parentSpan.SelfClose {
			// Self-closing parents were expanded with their attributes above.
			continue
		}
		if group.next != nil {
			nextSpan := snapshot.spans[group.next]
			ending := nearbyLineEnding(snapshot.source, nextSpan.Open.Start)
			indent := lineIndent(snapshot.source, nextSpan.Open.Start)
			replacement := strings.Join(fragments, ending+indent) + ending + indent
			edits = append(edits, editset.Insert(snapshot.path,
				int64(snapshot.baseOffset+nextSpan.Open.Start), []byte(replacement)))
			continue
		}
		if parentSpan.Close.IsZero() {
			return nil, convErrf("%s: OPF insertion parent %s has no closing tag", snapshot.path, group.parent.name)
		}
		closeStart := parentSpan.Close.Start
		gapStart := closeStart
		for gapStart > parentSpan.Open.End && isXMLSpace(snapshot.source[gapStart-1]) {
			gapStart--
		}
		gap := snapshot.source[gapStart:closeStart]
		gapRemoved := false
		for _, child := range snapshot.originalChild[group.parent] {
			if removed[child] && snapshot.spans[child].TailAfter(snapshot.source).End == closeStart {
				gapRemoved = true
				break
			}
		}
		replacement := appendOPFChildrenBeforeClose(snapshot, group.parent, parentSpan, gap, fragments, gapRemoved)
		edits = append(edits, editset.Insert(snapshot.path, int64(snapshot.baseOffset+closeStart), []byte(replacement)))
	}
	if len(edits) == 0 {
		return snapshot.original, nil
	}
	if !snapshot.direct {
		return nil, convErrf("%s: lossless OPF edits require UTF-8 source bytes", snapshot.path)
	}
	if err := editset.Validate(edits); err != nil {
		return nil, convErrf("%s: conflicting OPF edits: %v", snapshot.path, err)
	}
	updated, err := editset.Apply(snapshot.path, snapshot.original, edits)
	if err != nil {
		return nil, convErrf("%s: cannot apply OPF edits: %v", snapshot.path, err)
	}
	return updated, nil
}

func diffOPFText(snapshot *opfEditSnapshot, node *xmlElem, span *opfscan.SpanNode) (editset.Edit, error) {
	if len(snapshot.originalChild[node]) != 0 || len(node.children) != 0 || span.Close.IsZero() {
		return editset.Edit{}, convErrf("%s: cannot safely edit mixed-content OPF text in %s", snapshot.path, node.name)
	}
	start, end := span.Open.End, span.Close.Start
	if start < 0 || end < start || end > len(snapshot.source) {
		return editset.Edit{}, convErrf("%s: OPF text span for %s is invalid", snapshot.path, node.name)
	}
	if bytes.Contains(snapshot.source[start:end], []byte("<")) {
		return editset.Edit{}, convErrf("%s: cannot safely edit OPF text containing comments, processing instructions, or CDATA in %s", snapshot.path, node.name)
	}
	return editset.Replace(snapshot.path, int64(snapshot.baseOffset+start), int64(end-start), []byte(escapeOPFText(node.text))), nil
}

type opfChildGroup struct {
	parent   *xmlElem
	children []*xmlElem
	next     *xmlElem
}

func collectOPFChildGroups(snapshot *opfEditSnapshot, finalNodes []*xmlElem) ([]opfChildGroup, map[*xmlElem]bool, error) {
	var groups []opfChildGroup
	parentsWithChildren := map[*xmlElem]bool{}
	for _, parent := range finalNodes {
		if !snapshot.originalSet[parent] {
			if len(parent.children) > 0 {
				return nil, nil, convErrf("new OPF element %s unexpectedly has children", parent.name)
			}
			continue
		}
		children := parent.children
		for i := 0; i < len(children); {
			child := children[i]
			if snapshot.originalSet[child] {
				if snapshot.originalParent[child] != parent {
					return nil, nil, convErrf("%s: OPF tree moved existing element %s", snapshot.path, child.name)
				}
				i++
				continue
			}
			group := opfChildGroup{parent: parent}
			for i < len(children) && !snapshot.originalSet[children[i]] {
				group.children = append(group.children, children[i])
				i++
			}
			if i < len(children) {
				group.next = children[i]
			}
			groups = append(groups, group)
			parentsWithChildren[parent] = true
		}
	}
	return groups, parentsWithChildren, nil
}

func diffOPFAttributes(path string, source []byte, node *opfscan.SpanNode, original, current []xmlAttr) ([]editset.Edit, []string, error) {
	currentByKey := make(map[string]xmlAttr, len(current))
	originalByKey := make(map[string]xmlAttr, len(original))
	for _, attr := range current {
		currentByKey[opfAttrKey(attr)] = attr
	}
	for _, attr := range original {
		originalByKey[opfAttrKey(attr)] = attr
	}
	var edits []editset.Edit
	for _, old := range original {
		updated, exists := currentByKey[opfAttrKey(old)]
		index := node.AttrIndex(old.ns, old.name)
		if !exists {
			edit, err := opfscan.RemoveAttributeEdit(path, source, node, index)
			if err != nil {
				return nil, nil, convErrf("%s: cannot remove %s attribute: %v", path, old.name, err)
			}
			edits = append(edits, edit)
			continue
		}
		if updated.value == old.value {
			continue
		}
		span, quote, ok := opfscan.RawAttrValueSpan(source, node, index)
		if !ok || span.Start < node.Open.Start || span.End > node.Open.End {
			return nil, nil, convErrf("%s: %s attribute has no safe source span", path, old.name)
		}
		edits = append(edits, editset.Replace(path, int64(span.Start), int64(span.Len()), []byte(escapeOPFAttribute(updated.value, quote))))
	}
	var added []string
	for _, attr := range current {
		if _, existed := originalByKey[opfAttrKey(attr)]; existed {
			continue
		}
		name, err := opfAttributeQName(attr)
		if err != nil {
			return nil, nil, err
		}
		added = append(added, ` `+name+`="`+escapeOPFAttribute(attr.value, '"')+`"`)
	}
	return edits, added, nil
}

func opfAttrKey(attr xmlAttr) string { return attr.ns + "\x00" + attr.name }

func opfAttributeQName(attr xmlAttr) (string, error) {
	if attr.ns == "" {
		return attr.name, nil
	}
	if attr.ns == xmlNamespaceURI {
		return "xml:" + attr.name, nil
	}
	return "", convErrf("cannot add namespaced OPF attribute {%s}%s without a source prefix", attr.ns, attr.name)
}

func escapeOPFAttribute(value string, quote byte) string {
	replacer := strings.NewReplacer(
		"&", "&amp;", "<", "&lt;", ">", "&gt;",
		"\r", "&#13;", "\n", "&#10;", "\t", "&#09;",
	)
	value = replacer.Replace(value)
	if quote == '\'' {
		return strings.ReplaceAll(value, "'", "&#39;")
	}
	return strings.ReplaceAll(value, `"`, "&quot;")
}

func serializeOPFChild(child *xmlElem, parentPrefix string) (string, error) {
	if child.ns != opfURI || len(child.children) > 0 {
		return "", convErrf("cannot safely insert OPF fragment {%s}%s", child.ns, child.name)
	}
	name := child.name
	if parentPrefix != "" {
		name = parentPrefix + ":" + name
	}
	var b strings.Builder
	b.WriteByte('<')
	b.WriteString(name)
	for _, attr := range child.attrs {
		attrName, err := opfAttributeQName(attr)
		if err != nil {
			return "", err
		}
		b.WriteByte(' ')
		b.WriteString(attrName)
		b.WriteString(`="`)
		b.WriteString(escapeOPFAttribute(attr.value, '"'))
		b.WriteByte('"')
	}
	if child.text == "" {
		b.WriteString(" />")
		return b.String(), nil
	}
	b.WriteByte('>')
	b.WriteString(escapeOPFText(child.text))
	b.WriteString("</")
	b.WriteString(name)
	b.WriteByte('>')
	return b.String(), nil
}

func escapeOPFText(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\r", "&#13;").Replace(value)
}

func opfElementPrefix(source []byte, node *opfscan.SpanNode) (string, error) {
	name := rawOPFElementName(source, node)
	if name == "" {
		return "", fmt.Errorf("element has no safe raw QName")
	}
	if colon := strings.IndexByte(name, ':'); colon >= 0 {
		return name[:colon], nil
	}
	return "", nil
}

func rawOPFElementName(source []byte, node *opfscan.SpanNode) string {
	if node == nil || node.Open.Start < 0 || node.Open.End > len(source) || node.Open.Len() < 3 {
		return ""
	}
	start := node.Open.Start + 1
	end := start
	for end < node.Open.End && !isXMLSpace(source[end]) && source[end] != '/' && source[end] != '>' {
		end++
	}
	if end == start {
		return ""
	}
	return string(source[start:end])
}

func expandOPFSelfClosingParent(snapshot *opfEditSnapshot, parent *xmlElem, span *opfscan.SpanNode, content string) ([]byte, error) {
	open := snapshot.source[span.Open.Start:span.Open.End]
	attributeEdits, added, err := diffOPFAttributes(snapshot.path, snapshot.source, span, snapshot.originalAttrs[parent], parent.attrs)
	if err != nil {
		return nil, err
	}
	for _, addition := range added {
		at := span.Open.End - 1
		if at > span.Open.Start && snapshot.source[at-1] == '/' {
			at--
		}
		attributeEdits = append(attributeEdits, editset.Insert(snapshot.path, int64(at), []byte(addition)))
	}
	localEdits := slices.Clone(attributeEdits)
	for i := range localEdits {
		localEdits[i].Path = "opf-open-tag"
		localEdits[i].Offset -= int64(span.Open.Start)
	}
	updatedOpen, err := editset.Apply("opf-open-tag", open, localEdits)
	if err != nil {
		return nil, convErrf("%s: cannot update self-closing %s attributes: %v", snapshot.path, parent.name, err)
	}
	if !bytes.HasSuffix(updatedOpen, []byte("/>")) {
		return nil, convErrf("%s: self-closing %s tag has no slash delimiter", snapshot.path, parent.name)
	}
	qname := rawOPFElementName(snapshot.source, span)
	if qname == "" {
		return nil, convErrf("%s: self-closing %s tag has no raw QName", snapshot.path, parent.name)
	}
	result := append([]byte(nil), updatedOpen[:len(updatedOpen)-2]...)
	result = append(result, '>')
	result = append(result, content...)
	result = append(result, []byte("</"+qname+">")...)
	return result, nil
}

func rawOPFPrefix(source []byte, span *opfscan.SpanNode) string {
	name := rawOPFElementName(source, span)
	if colon := strings.IndexByte(name, ':'); colon >= 0 {
		return name[:colon]
	}
	return ""
}

func formatOPFInsertedChildren(snapshot *opfEditSnapshot, span *opfscan.SpanNode, children []string) string {
	body := strings.Join(children, "")
	indent := lineIndent(snapshot.source, span.Open.Start)
	ending := nearbyLineEnding(snapshot.source, span.Open.Start)
	if ending != "" {
		childIndent := indent + "  "
		body = ending + childIndent + strings.Join(children, ending+childIndent) + ending + indent
	}
	return body
}

func appendOPFChildrenBeforeClose(snapshot *opfEditSnapshot, parent *xmlElem, span *opfscan.SpanNode, gap []byte, children []string, gapRemoved bool) []byte {
	if ending := lastLineEnding(gap); ending != "" {
		parentIndent := lineIndent(snapshot.source, span.Close.Start)
		childIndent := opfChildIndent(snapshot, parent, span, parentIndent)
		prefix := childIndent
		if strings.HasPrefix(childIndent, parentIndent) {
			prefix = childIndent[len(parentIndent):]
		}
		if gapRemoved {
			prefix = ""
		}
		return []byte(prefix + strings.Join(children, ending+childIndent) + ending + parentIndent)
	}
	spaces := string(gap)
	return []byte(strings.Join(children, "") + spaces)
}

func opfChildIndent(snapshot *opfEditSnapshot, parent *xmlElem, span *opfscan.SpanNode, fallback string) string {
	for _, child := range snapshot.originalChild[parent] {
		childSpan := snapshot.spans[child]
		if indent := lineIndent(snapshot.source, childSpan.Open.Start); indent != "" {
			return indent
		}
	}
	if span.TextSpan.IsZero() && len(span.Kids) > 0 {
		if indent := lineIndent(snapshot.source, span.Kids[0].Open.Start); indent != "" {
			return indent
		}
	}
	return fallback + "  "
}

func nearbyLineEnding(source []byte, offset int) string {
	if offset > len(source) {
		offset = len(source)
	}
	if offset < 0 {
		offset = 0
	}
	if ending := lastLineEnding(source[:offset]); ending != "" {
		return ending
	}
	return lastLineEnding(source)
}

func lastLineEnding(source []byte) string {
	index := bytes.LastIndexAny(source, "\r\n")
	if index < 0 {
		return ""
	}
	if source[index] == '\n' && index > 0 && source[index-1] == '\r' {
		return "\r\n"
	}
	return string(source[index])
}

func lineIndent(source []byte, offset int) string {
	if offset < 0 || offset > len(source) {
		return ""
	}
	start := bytes.LastIndexAny(source[:offset], "\r\n") + 1
	indent := source[start:offset]
	for _, value := range indent {
		if value != ' ' && value != '\t' {
			return ""
		}
	}
	return string(indent)
}

func isXMLSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r'
}

func hasRemovedOPFAncestor(node *xmlElem, removed map[*xmlElem]bool, parents map[*xmlElem]*xmlElem) bool {
	for parent := parents[node]; parent != nil; parent = parents[parent] {
		if removed[parent] {
			return true
		}
	}
	return false
}
