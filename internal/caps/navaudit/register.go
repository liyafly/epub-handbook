// register.go 收纳 navaudit 的不可变扫描表（INV-7 白名单：注册表文件）。
package navaudit

import "regexp"

// 文件名混淆判定（对齐 Python [\x00-\x1f\\:*?<>|]）。
var specialCharRe = regexp.MustCompile(`[\x{0}-\x{1f}\\\:*?<>|]`)

var tagStripRe = regexp.MustCompile(`<[^>]+>`)
var imgRe = regexp.MustCompile(`(?i)<img\b`)
var enLangRe = regexp.MustCompile(`\b(?:xml:)?lang=["']en(?:[-_][A-Za-z0-9]+)?["']`)
var mathmlRe = regexp.MustCompile(`(?i)<(?:math|m:math)\b`)
var svgRe = regexp.MustCompile(`(?i)<(?:svg|svg:svg)\b`)
var noterefRe = regexp.MustCompile(`epub:type=["']noteref["']`)
var footnoteRe = regexp.MustCompile(`epub:type=["']footnote["']`)

// CSS url() 提取（对齐 epub_ai/core.py:59-66，忽略注释后的文本）。
var cssURLRe = regexp.MustCompile(`url\(\s*["']?([^"')]+)["']?\s*\)`)

// actionable detectors 用。
var calibreClassRe = regexp.MustCompile(`\bcalibre\d*\b`)

// Python 侧用朴素子串匹配命名空间 URI（epub_ai/core.py），即使 URI 出现在
// 代码示例文本里也会命中 —— 为保 parity 原样复刻这一行为。
const mathmlURI = "http://www.w3.org/1998/Math/MathML"
const svgURI = "http://www.w3.org/2000/svg"
