// opfbuild.go 复刻 package_io.build_container 与 core.build_opf。
// 两者在 Python 侧都由 ElementTree 从零生成（整棵树是新建的，无原文可保），
// 这里按实测的 ET.tostring 输出逐字节复刻：
//   - 声明 `<?xml version='1.0' encoding='utf-8'?>\n`；
//   - container 命名空间未注册 → ns0 前缀；
//   - OPF 命名空间注册为空前缀、dc 注册为 dc:，xmlns 声明按前缀排序落在根上；
//   - 空元素 `<tag />`（斜杠前有空格）。
package merge

import (
	"strings"
	"time"
)

// buildContainer 复刻 package_io.build_container 的确切输出。
func buildContainer(opfPath string) []byte {
	var b strings.Builder
	b.WriteString("<?xml version='1.0' encoding='utf-8'?>\n")
	b.WriteString(`<ns0:container xmlns:ns0="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><ns0:rootfiles><ns0:rootfile full-path="`)
	b.WriteString(attribEscape(opfPath))
	b.WriteString(`" media-type="application/oebps-package+xml" /></ns0:rootfiles></ns0:container>`)
	return []byte(b.String())
}

// manifestTuple 对齐 build_opf 的 (item_id, href, media_type, properties)。
type manifestTuple struct {
	itemID    string
	href      string
	mediaType string
	props     string
}

type spineTuple struct {
	idref      string
	linear     string
	properties string
}

// metaExtract 是 build_opf 从第一卷 metadata 里读取的字段。
type metaExtract struct {
	creator     string
	publisher   string
	description string
	rights      string
}

// buildOPF 复刻 core.build_opf 的确切输出（dcterms:modified 用当前 UTC，
// 与 Python 的 datetime.now(timezone.utc) 同为不确定字段）。
func buildOPF(title string, meta *metaExtract, items []manifestTuple, spine []spineTuple) []byte {
	var b strings.Builder
	b.WriteString("<?xml version='1.0' encoding='utf-8'?>\n")
	b.WriteString(`<package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="book-id" prefix="dcterms: http://purl.org/dc/terms/">`)
	b.WriteString(`<metadata><dc:identifier id="book-id">urn:uuid:epub-package-tool</dc:identifier>`)
	b.WriteString("<dc:title>" + cdataEscape(title) + "</dc:title>")
	b.WriteString("<dc:language>zh-CN</dc:language>")
	if meta != nil {
		for _, field := range []struct{ tag, value string }{
			{"creator", meta.creator},
			{"publisher", meta.publisher},
			{"description", meta.description},
			{"rights", meta.rights},
		} {
			if field.value != "" {
				b.WriteString("<dc:" + field.tag + ">" + cdataEscape(field.value) + "</dc:" + field.tag + ">")
			}
		}
	}
	b.WriteString(`<meta property="dcterms:modified">` + time.Now().UTC().Format("2006-01-02T15:04:05Z") + "</meta>")
	coverID := ""
	for _, it := range items {
		if hasToken(it.props, "cover-image") {
			coverID = it.itemID
			break
		}
	}
	if coverID != "" {
		b.WriteString(`<meta name="cover" content="` + attribEscape(coverID) + `" />`)
	}
	b.WriteString("</metadata>")
	b.WriteString(`<manifest>`)
	b.WriteString(`<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav" />`)
	b.WriteString(`<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml" />`)
	for _, it := range items {
		b.WriteString(`<item id="` + attribEscape(it.itemID) + `" href="` + attribEscape(it.href) +
			`" media-type="` + attribEscape(it.mediaType) + `"`)
		if it.props != "" {
			b.WriteString(` properties="` + attribEscape(it.props) + `"`)
		}
		b.WriteString(" />")
	}
	b.WriteString("</manifest>")
	b.WriteString(`<spine toc="ncx">`)
	b.WriteString(`<itemref idref="nav" linear="no" />`)
	for _, sp := range spine {
		b.WriteString(`<itemref idref="` + attribEscape(sp.idref) + `"`)
		if sp.linear != "" {
			b.WriteString(` linear="` + attribEscape(sp.linear) + `"`)
		}
		if sp.properties != "" {
			b.WriteString(` properties="` + attribEscape(sp.properties) + `"`)
		}
		b.WriteString(" />")
	}
	b.WriteString("</spine></package>")
	return []byte(b.String())
}

func hasToken(value, token string) bool {
	for _, p := range splitProps(value) {
		if p == token {
			return true
		}
	}
	return false
}
