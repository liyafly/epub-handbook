// opfgen.go 提供 split 段内固定产物的字节生成：
//   - build_container（与 merge 包同源，ns0 前缀为实测 ET 输出）；
//   - manifest / spine 的重建元素（build_split_package 用 ET 新建这两个
//     子树；这里按 ET.tostring 规则生成确切字节，再以整元素区间替换的
//     编辑写回源 OPF —— metadata 与根属性保留原字节）。
package split

import "strings"

// buildContainer 复刻 package_io.build_container 的确切输出。
func buildContainer(opfPath string) []byte {
	var b strings.Builder
	b.WriteString("<?xml version='1.0' encoding='utf-8'?>\n")
	b.WriteString(`<ns0:container xmlns:ns0="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><ns0:rootfiles><ns0:rootfile full-path="`)
	b.WriteString(attribEscape(opfPath))
	b.WriteString(`" media-type="application/oebps-package+xml" /></ns0:rootfiles></ns0:container>`)
	return []byte(b.String())
}

// buildManifestElement 生成 build_split_package 里重建的 manifest 元素
// 字节（不含 tail）。
func buildManifestElement(items []manifestTuple, navID, navHref, ncxID, ncxHref string) string {
	var b strings.Builder
	b.WriteString("<manifest>")
	for _, it := range items {
		b.WriteString(`<item id="` + attribEscape(it.itemID) + `" href="` + attribEscape(it.href) +
			`" media-type="` + attribEscape(it.mediaType) + `"`)
		if it.props != "" {
			b.WriteString(` properties="` + attribEscape(it.props) + `"`)
		}
		b.WriteString(" />")
	}
	b.WriteString(`<item id="` + attribEscape(navID) + `" href="` + attribEscape(navHref) +
		`" media-type="application/xhtml+xml" properties="nav" />`)
	b.WriteString(`<item id="` + attribEscape(ncxID) + `" href="` + attribEscape(ncxHref) +
		`" media-type="application/x-dtbncx+xml" />`)
	b.WriteString("</manifest>")
	return b.String()
}

// buildSpineElement 生成 build_split_package 里重建的 spine 元素字节。
func buildSpineElement(ncxID string, refs []spineRef) string {
	var b strings.Builder
	b.WriteString(`<spine toc="` + attribEscape(ncxID) + `">`)
	b.WriteString(`<itemref idref="` + attribEscape(refs[0].idref) + `" linear="no" />`)
	for _, sp := range refs[1:] {
		b.WriteString(`<itemref idref="` + attribEscape(sp.idref) + `"`)
		if sp.linear != "" {
			b.WriteString(` linear="` + attribEscape(sp.linear) + `"`)
		}
		if sp.properties != "" {
			b.WriteString(` properties="` + attribEscape(sp.properties) + `"`)
		}
		b.WriteString(" />")
	}
	b.WriteString("</spine>")
	return b.String()
}
