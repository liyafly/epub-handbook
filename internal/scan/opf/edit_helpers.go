package opf

import (
	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
)

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
