// register.go 收纳 scan/css 的不可变扫描表（INV-7 白名单：注册表文件，
// 仅 init 期写入）。正则与 Python 侧语义一一对应，此处集中便于对账。
package css

import "regexp"

// commentRe 是 CSS 注释（re.S 语义由 (?s) 提供）。
var commentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)

// ruleRe 对齐 Python RULE_RE `([^{}]+)\{([^{}]*)\}`（re.S）。
var ruleRe = regexp.MustCompile(`(?s)([^{}]+)\{([^{}]*)\}`)

// fontDeclRe 对齐 Python FONT_FAMILY_RE `(font-family\s*:\s*)([^;}]+)`（re.I）。
var fontDeclRe = regexp.MustCompile(`(?i)font-family\s*:\s*([^;}]+)`)
