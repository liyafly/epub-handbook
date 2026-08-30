// register.go 收纳 metadata 包的只读表（INV-7 白名单：注册表文件）。
package metadata

import "strings"

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

// cdataEscape 复刻 ElementTree._escape_cdata 的文本转义。
var cdataEscape = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
).Replace
