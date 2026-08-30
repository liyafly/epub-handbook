// register.go 收纳 cover 包的只读表（INV-7 白名单：注册表文件）。
package cover

import "strings"

// markupExtensions 对齐 core.MARKUP_EXTENSIONS。
var markupExtensions = map[string]bool{
	".html": true, ".htm": true, ".xhtml": true,
	".xml": true, ".ncx": true, ".svg": true, ".smil": true,
}

// uriAttrNames 是 URI_ATTRIBUTE_RE 的名字交替表（按 Python 正则的尝试序）。
var uriAttrNames = []string{"href", "src", "poster", "data", "xlink:href", "textref"}

// imageMediaByExt 对齐 core.IMAGE_MEDIA_BY_EXT。
var imageMediaByExt = map[string]string{
	".gif":  "image/gif",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".webp": "image/webp",
}

// attribEscape 复刻 ElementTree._escape_attrib 的属性值转义。
var attribEscape = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"\r", "&#13;",
	"\n", "&#10;",
	"\t", "&#09;",
).Replace

// singleQuoteEscaper 单引号属性值的转义。
var singleQuoteEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"'", "&#39;",
	`"`, "&quot;",
)
