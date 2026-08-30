// register.go 是本包的常量查找表（INV-7 白名单：包级表只允许住在
// register.go）。全部内容逐条对齐 scripts/epub_structure_tool.py。
package structurenormalize

import (
	"regexp"
	"strings"
)

// namespacePrefixes 复刻 Python 侧 ElementTree._namespace_map 在
// epub_lib.py 与 epub_structure_tool.py import 期 register_namespace
// 之后的最终状态（后注册者覆盖先注册者，OPF 命名空间最终绑定空前缀）。
// ET 序列化时不在表内的 URI 按 "ns%d"（当前已注册数量）生成前缀。
var namespacePrefixes = map[string]string{
	"http://www.w3.org/XML/1998/namespace":        "xml",
	"http://www.w3.org/1999/xhtml":                "html",
	"http://www.w3.org/1999/02/22-rdf-syntax-ns#": "rdf",
	"http://schemas.xmlsoap.org/wsdl/":            "wsdl",
	"http://www.w3.org/2001/XMLSchema":            "xs",
	"http://www.w3.org/2001/XMLSchema-instance":   "xsi",
	"http://www.idpf.org/2007/opf":                "",
	"http://purl.org/dc/elements/1.1/":            "dc",
	"http://purl.org/dc/terms/":                   "dcterms",
}

// fontObfuscationAlgorithms 是允许的标准 EPUB 字体混淆算法
// （FONT_OBFUSCATION_ALGORITHMS）。
var fontObfuscationAlgorithms = map[string]bool{
	"http://www.idpf.org/2008/embedding": true,
	"http://ns.adobe.com/pdf/enc#RC":     true,
}

// markupExtensions 是按扩展名参与引用重写的文本资源（MARKUP_EXTENSIONS）。
var markupExtensions = map[string]bool{
	".html": true, ".htm": true, ".xhtml": true,
	".xml": true, ".ncx": true, ".svg": true, ".smil": true,
}

var imageExtensions = map[string]bool{
	".bmp": true, ".gif": true, ".jpeg": true, ".jpg": true,
	".png": true, ".svg": true, ".webp": true,
}

var fontExtensions = map[string]bool{
	".otf": true, ".ttf": true, ".woff": true, ".woff2": true,
}

var audioExtensions = map[string]bool{".m4a": true, ".mp3": true, ".ogg": true}

var videoExtensions = map[string]bool{".m4v": true, ".mp4": true, ".webm": true}

// uriAttrNames 是 URI_ATTRIBUTE_RE 的名字交替表，按 Python 正则的
// 尝试顺序排列（href|src|poster|data|xlink:href|textref）。
var uriAttrNames = []string{"href", "src", "poster", "data", "xlink:href", "textref"}

// xmlEncodingRe 复刻 XML_ENCODING_RE：从字节前缀里提取声明的编码名。
var xmlEncodingRe = regexp.MustCompile(`(?i)encoding\s*=\s*["']([A-Za-z0-9._-]+)["']`)

// attribEscaper 复刻 ElementTree._escape_attrib：属性值转义
// & < > " \r \n \t（单趟替换，顺序与 CPython 一致）。
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

// invalidFilenameChars 复刻 INVALID_FILENAME_RE 的字符集：
// [\x00-\x1f\\/:*?"<>|] → 替换为 "-"。
func invalidFilenameChar(r rune) bool {
	if r < 0x20 {
		return true
	}
	switch r {
	case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
		return true
	}
	return false
}
