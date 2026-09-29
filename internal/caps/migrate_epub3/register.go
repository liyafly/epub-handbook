// register.go 是本包的常量查找表（INV-7 白名单：包级 var 只允许住在
// register.go，且仅 init() 期写入）。全部内容逐条对齐
// scripts/epub3_conversion/core.py 与 scripts/epub_lib.py。
package migrateepub3

import (
	"regexp"
)

// Python 侧 XML 命名空间 URI（epub_lib.py）。
const (
	containerURI = "urn:oasis:names:tc:opendocument:xmlns:container"
	opfURI       = "http://www.idpf.org/2007/opf"
	dcURI        = "http://purl.org/dc/elements/1.1/"
	dctermsURI   = "http://purl.org/dc/terms/"
	ncxURI       = "http://www.daisy.org/z3986/2005/ncx/"
	xhtmlURI     = "http://www.w3.org/1999/xhtml"
	opsURI       = "http://www.idpf.org/2007/ops"
	ibooksPrefix = "http://vocabulary.itunes.apple.com/rdf/ibooks/vocabulary-extensions-1.0/"
	renditionURI = "http://www.idpf.org/vocab/rendition/#"
)

// xmlEncodingRe 复刻 XML_ENCODING_RE：从字节前缀里提取声明的编码名。
var xmlEncodingRe = regexp.MustCompile(`(?i)encoding\s*=\s*["']([A-Za-z0-9._-]+)["']`)

// fontMediaTypes / imageMediaByExt 对齐 core.py 的 FONT_MEDIA_TYPES 与
// IMAGE_MEDIA_BY_EXT。
var fontMediaTypes = map[string]bool{
	"application/x-font-ttf":      true,
	"application/x-font-opentype": true,
	"application/font-sfnt":       true,
	"font/ttf":                    true,
	"font/otf":                    true,
}

var imageMediaByExt = map[string]string{
	".gif":  "image/gif",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".webp": "image/webp",
}

// guideTypeToEpub 对齐 GUIDE_TYPE_TO_EPUB。
var guideTypeToEpub = map[string]string{
	"cover":          "cover",
	"toc":            "toc",
	"text":           "bodymatter",
	"title-page":     "titlepage",
	"copyright-page": "copyright-page",
}

// The remaining core.py patterns are compatible with Go's RE2 engine.
var (
	ncxSrcFixRe      = regexp.MustCompile(`(?i)(<content\b[^>]*\bsrc=)(["'])([^"']+?)(["'])(#[^"'>\s/]+)`)
	doctypeRe        = regexp.MustCompile(`(?is)<!DOCTYPE[^>]*>`)
	fontFamilyDeclRe = regexp.MustCompile(`(?i)\bfont-family\s*:`)
	idCleanRe        = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)
)
