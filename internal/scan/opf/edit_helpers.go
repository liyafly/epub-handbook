package opf

import (
	"fmt"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
)

// ClassTokenEdit returns a lossless edit that adds token to n's class
// attribute. It appends inside the existing quoted value, or inserts a new
// class attribute before the end of the open tag. present is true when the
// token already exists. Unsafe source spans or values return an error.
func ClassTokenEdit(path string, data []byte, n *SpanNode, token string) (edit editset.Edit, present bool, err error) {
	if !validClassToken(token) {
		return editset.Edit{}, false, fmt.Errorf("class token %q: only [A-Za-z0-9_-] allowed", token)
	}
	if n == nil || n.Open.Start < 0 || n.Open.End > len(data) || n.Open.End-n.Open.Start < 3 ||
		data[n.Open.Start] != '<' || data[n.Open.End-1] != '>' {
		return editset.Edit{}, false, fmt.Errorf("%s: open-tag span does not match source bytes", path)
	}
	if idx := n.AttrIndex("", "class"); idx >= 0 {
		if slices.Contains(strings.Fields(n.Attrs[idx].Value), token) {
			return editset.Edit{}, true, nil
		}
		span, _, ok := RawAttrValueSpan(data, n, idx)
		if !ok || span.Start < n.Open.Start || span.End > n.Open.End || span.Start > span.End {
			return editset.Edit{}, false, fmt.Errorf("%s: class attribute value is not quoted or span does not match source bytes", path)
		}
		if strings.TrimSpace(string(data[span.Start:span.End])) == "" {
			return editset.Replace(path, int64(span.Start), int64(span.Len()), []byte(token)), false, nil
		}
		return editset.Insert(path, int64(span.End), []byte(" "+token)), false, nil
	}
	at := n.Open.End - 1
	if n.SelfClose && data[at-1] == '/' {
		at--
	}
	return editset.Insert(path, int64(at), []byte(` class="`+token+`"`)), false, nil
}

func validClassToken(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// ManifestNodes returns all direct OPF manifest/item nodes in document order.
// Foreign namespaces and nested lookalikes do not count.
func ManifestNodes(root *SpanNode) []*SpanNode {
	var out []*SpanNode
	for _, manifest := range root.ChildrenByLocal(OPFURI, "manifest") {
		out = append(out, manifest.ChildrenByLocal(OPFURI, "item")...)
	}
	return out
}

// BuildCSSItem creates one new manifest fragment, never serializing a source
// document. Attribute ordering and escaping preserve the existing wire bytes.
func BuildCSSItem(id, href string) string {
	return `<item id="` + pypath.EscapeAttribute(id) + `" href="` + pypath.EscapeAttribute(href) + `" media-type="text/css" />`
}

// RemoveElementEdit removes one node including its tail, without reserializing
// neighboring elements, comments or processing instructions.
func RemoveElementEdit(path string, data []byte, node *SpanNode) editset.Edit {
	start := node.Open.Start
	end := node.TailAfter(data).End
	return editset.Replace(path, int64(start), int64(end-start), []byte{})
}

// RemoveAttributeEdit removes one attribute and its immediately preceding
// whitespace, while preserving the neighboring attribute bytes.
func RemoveAttributeEdit(path string, data []byte, node *SpanNode, index int) (editset.Edit, error) {
	if node == nil || index < 0 || index >= len(node.Attrs) {
		return editset.Edit{}, fmt.Errorf("%s: attribute index %d is out of range", path, index)
	}
	raw := RawAttrsIn(data, node.Open)
	if index >= len(raw) || raw[index].Span.Start < node.Open.Start || raw[index].Span.End > node.Open.End || raw[index].Span.Start > raw[index].Span.End {
		return editset.Edit{}, fmt.Errorf("%s: attribute %d has no safe source span", path, index)
	}
	start := raw[index].Span.Start
	for start > node.Open.Start+1 && isASCIISpaceByte(data[start-1]) {
		start--
	}
	return editset.Replace(path, int64(start), int64(raw[index].Span.End-start), []byte{}), nil
}
