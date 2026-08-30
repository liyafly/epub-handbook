// register.go 是本包的常量查找表（INV-7 白名单：包级 var 只允许住在
// register.go，且仅 init() 期写入）。全部内容逐条对齐
// scripts/epub3_conversion/core.py 与 scripts/epub_lib.py。
package migrateepub3

import (
	"regexp"
	"strings"
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

// namespacePrefixesOPF 复刻 epub_lib.py import 期 register_namespace 之后的
// ElementTree._namespace_map 终态。关键点：register_namespace 的实现会先删掉
// 「同 URI 或同前缀」的旧条目，因此 epub_lib 末尾的
// register_namespace("opf", OPF_URI) 抹掉了最初的 "" → OPF 绑定；
// format_xhtml_multiline 的 finally 里虽然重注册 "" → OPF，但紧接着的
// register_namespace("opf", OPF_URI) 又再次抹掉。最终 OPF 序列化带
// opf: 前缀（<opf:package ...>）。xhtml 默认条目也被抹掉（html 前缀删除）。
var namespacePrefixesOPF = map[string]string{
	"http://www.w3.org/XML/1998/namespace":        "xml",
	"http://www.w3.org/1999/02/22-rdf-syntax-ns#": "rdf",
	"http://schemas.xmlsoap.org/wsdl/":            "wsdl",
	"http://www.w3.org/2001/XMLSchema":            "xs",
	"http://www.w3.org/2001/XMLSchema-instance":   "xsi",
	"http://purl.org/dc/elements/1.1/":            "dc",
	"http://purl.org/dc/terms/":                   "dcterms",
	"http://www.idpf.org/2007/opf":                "opf",
}

// namespacePrefixesXHTML 是 format_xhtml_multiline 序列化期间的注册表状态：
// register_namespace("", XHTML_URI) 抹掉 html 默认条目并绑定空前缀，
// register_namespace("epub", OPS_URI) 绑定 epub。
var namespacePrefixesXHTML = map[string]string{
	"http://www.w3.org/XML/1998/namespace":        "xml",
	"http://www.w3.org/1999/02/22-rdf-syntax-ns#": "rdf",
	"http://schemas.xmlsoap.org/wsdl/":            "wsdl",
	"http://www.w3.org/2001/XMLSchema":            "xs",
	"http://www.w3.org/2001/XMLSchema-instance":   "xsi",
	"http://purl.org/dc/elements/1.1/":            "dc",
	"http://purl.org/dc/terms/":                   "dcterms",
	"http://www.idpf.org/2007/opf":                "opf",
	"http://www.idpf.org/2007/ops":                "epub",
	"http://www.w3.org/1999/xhtml":                "",
}

// attribEscaper 复刻 ElementTree._escape_attrib：属性值转义
// & < > " \r \n \t。
var attribEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"\r", "&#13;",
	"\n", "&#10;",
	"\t", "&#09;",
)

// cdataEscaper 复刻 ElementTree._escape_cdata：文本与 tail 只转义 & < >。
var cdataEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

// xmlEncodingRe 复刻 XML_ENCODING_RE：从字节前缀里提取声明的编码名。
var xmlEncodingRe = regexp.MustCompile(`(?i)encoding\s*=\s*["']([A-Za-z0-9._-]+)["']`)

// fontMediaTypes / imageMediaByExt 对齐 core.py 的 FONT_MEDIA_TYPES 与
// IMAGE_MEDIA_BY_EXT。
var fontMediaTypes = map[string]bool{
	"application/x-font-ttf":       true,
	"application/x-font-opentype":  true,
	"application/font-sfnt":        true,
	"font/ttf":                     true,
	"font/otf":                     true,
}

var imageMediaByExt = map[string]string{
	".gif":  "image/gif",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".webp": "image/webp",
}

// typographyRoles 对齐 TYPOGRAPHY_ROLES（顺序敏感）。
var typographyRoles = []string{
	"type-body",
	"type-title",
	"type-subtitle",
	"type-quote",
	"type-note",
	"type-emphasis",
	"type-meta",
}

// inlineContentTags 对齐 INLINE_CONTENT_TAGS。
var inlineContentTags = map[string]bool{
	"a": true, "abbr": true, "b": true, "bdi": true, "bdo": true, "br": true,
	"cite": true, "code": true, "em": true, "i": true, "img": true,
	"kbd": true, "label": true, "mark": true, "q": true, "ruby": true,
	"s": true, "samp": true, "small": true, "span": true, "strong": true,
	"sub": true, "sup": true, "time": true, "u": true, "var": true, "wbr": true,
}

// guideTypeToEpub 对齐 GUIDE_TYPE_TO_EPUB。
var guideTypeToEpub = map[string]string{
	"cover":          "cover",
	"toc":            "toc",
	"text":           "bodymatter",
	"title-page":     "titlepage",
	"copyright-page": "copyright-page",
}

// pyPatterns 预编译 core.py 的全部正则（fold = re.I，dotAll = re.S）。
var pyPatterns = buildPatterns()

func buildPatterns() map[string]*pyRegexp {
	m := map[string]*pyRegexp{}
	def := func(name, pattern string, fold, dotAll bool) {
		m[name] = mustCompilePy(pattern, fold, dotAll)
	}
	// sanitize_ncx_text 的坏引号修复（re.I）。
	def("ncxSrcFix", `(<content\b[^>]*\bsrc=)(["'])([^"']+?)(["'])(#[^"'>\s/]+)`, true, false)
	// normalize_xhtml_shell（re.I；DOCTYPE 另有 re.S）。
	def("xmlDecl", `^\s*<\?xml[^>]*\?>`, true, false)
	def("doctype", `<!DOCTYPE[^>]*>`, true, true)
	def("htmlTag", `<html\b([^>]*)>`, true, false)
	def("headEnd", `</head\s*>`, true, false)
	def("metaHTTP", `<meta\b(?=[^>]*http-equiv=["']Content-Type["'])(?=[^>]*charset=utf-8)[^>]*/?>`, true, false)
	// html_repl 的 lang 补齐（无 re.I）。
	def("langAttr", `(?<![:\w-])lang\s*=\s*(["'])(.*?)\1`, false, false)
	def("xmlLangAttr", `xml:lang\s*=\s*(["'])(.*?)\1`, false, false)
	// big → span。
	def("bigOpen", `<big\b([^>]*)>`, true, false)
	def("bigClose", `</big\s*>`, true, false)
	// 本地纯文本弹注（re.S）。
	def("plainNoteref", `<a\s+id="w(?P<num>\d+)"></a>\s*<a\s+href="(?P<href>[^"]*#m(?P=num))">\s*<sup>\[(?P=num)\]</sup>\s*</a>`, false, true)
	def("plainNote", `\s*<p\s+class="note"\s*>\s*<a\s+id="m(?P<num>\d+)"></a>\s*<a\s+href="[^"]*#w(?P=num)">\[(?P=num)\]</a>\s*(?P<body>.*?)</p>`, false, true)
	// Sigil 遗留弹注（re.I | re.S）。
	def("sigilSection", `<section\b(?=[^>]*\bepub:type\s*=\s*["']footnotes["'])[^>]*>(?P<body>.*?)</section>`, true, true)
	def("sigilNote", `<aside\b(?=[^>]*\bid\s*=\s*["']footnote_(?P<num>\d+)["'])[^>]*>\s*<p\b[^>]*>\s*<a\b(?=[^>]*\bhref\s*=\s*["']#noteref_(?P=num)["'])[^>]*>\s*\[(?P=num)\]\s*</a>(?P<body>.*?)</p>\s*</aside>`, true, true)
	def("sigilNoteref", `<a\b(?=[^>]*\bid\s*=\s*["']noteref_(?P<num>\d+)["'])[^>]*>\s*\[(?P=num)\]\s*</a>`, true, true)
	def("noteMarkerSup", `<sup(?P<attrs>\s[^>]*)?>(?P<content>\s*<a\b(?=[^>]*\bclass\s*=\s*["'][^"']*\bnoteref-icon\b)[^>]*>.*?</a>\s*)</sup>`, true, true)
	def("classAttr", `\bclass\s*=\s*(?P<quote>["'])(?P<value>[^"']*)(?P=quote)`, true, false)
	def("hrBeforeNotes", `\s*<hr\b[^>]*/?>\s*$`, true, true)
	// 属性标记检查（re.I）。
	def("svgCheck", `<(?:svg|svg:svg)\b`, true, false)
	def("mathCheck", `<(?:math|m:math)\b`, true, false)
	def("scriptCheck", `<script\b`, true, false)
	// duokan 归一（无 re.I）。
	def("duokanAside", `<aside\s+epub:type="footnote"(?![^>]*\brole=)`, false, false)
	// has_body_font_locked 的声明检查（re.I）。
	def("fontFamilyDecl", `\bfont-family\s*:`, true, false)
	// unique_id 的 id 清洗。
	def("idClean", `[^A-Za-z0-9_.-]+`, false, false)
	return m
}
