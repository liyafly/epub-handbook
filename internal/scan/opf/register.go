// register.go 收纳 scan/opf 的包级只读表（INV-7 白名单：注册表文件）。
package opf

import "regexp"

// xmlEncodingRe 从字节前缀提取 XML 声明的编码名。
var xmlEncodingRe = regexp.MustCompile(`(?i)encoding\s*=\s*["']([A-Za-z0-9._-]+)["']`)
