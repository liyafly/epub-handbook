// register.go 是本包的常量查找表（INV-7 白名单：包级表只允许住在
// register.go）。全部内容逐条对齐 scripts/epub_structure_tool.py。
package structurenormalize

import (
	"regexp"
	"strings"
)

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

// xmlEncodingRe 复刻 XML_ENCODING_RE：从字节前缀里提取声明的编码名。
var xmlEncodingRe = regexp.MustCompile(`(?i)encoding\s*=\s*["']([A-Za-z0-9._-]+)["']`)

// attribEscaper 保留标记属性原引号时，用相应 XML 实体转义替换值。
var attribEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"\r", "&#13;",
	"\n", "&#10;",
	"\t", "&#09;",
)

var singleQuoteAttrEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"'", "&#39;",
	`"`, "&quot;",
	"\r", "&#13;",
	"\n", "&#10;",
	"\t", "&#09;",
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
