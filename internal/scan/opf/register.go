// register.go 收纳 scan/opf 的包级只读表（INV-7 白名单：注册表文件）。
package opf

import "regexp"

// xmlDeclarationEncoding 匹配 XML 声明中的 encoding 属性。
var xmlDeclarationEncoding = regexp.MustCompile(`(?i)\bencoding\s*=\s*["']([^"']+)["']`)
