// register.go 是本包的常量查找表（INV-7 白名单：包级表只允许住在
// register.go）。全部内容逐条对齐 scripts/epub_style_preset_tool.py。
package typography

import "regexp"

// coverageThreshold 对齐 COVERAGE_THRESHOLD。
const coverageThreshold = 0.3

// coverageWarningText 对齐 coverage_report 的中文 warning 文案。
const coverageWarningText = "该书尚未迁入本仓 class 体系，请先走 cleanup pipeline（oneclick 会注入 typography palette）"

// 注：曾经住在这里的 typoLinkRe / typoHeadEndRe（对齐 Python LINK_RE /
// HEAD_END_RE）已改为 xhtml.ScanRegions 驱动的区域化实现（见
// typography.go 的 rewriteStylesheetLinks），不再需要整文本正则 —— 那两条
// 正则在注释/CDATA/<script> 里同形文字上也会命中，是已修复的缺陷。

// idSanitizeRe 对齐 epub_lib.unique_id 的 re.sub(r"[^A-Za-z0-9_.-]+", "-", ...)。
var idSanitizeRe = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)
