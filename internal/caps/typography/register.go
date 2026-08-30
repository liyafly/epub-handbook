// register.go 是本包的常量查找表（INV-7 白名单：包级表只允许住在
// register.go）。全部内容逐条对齐 scripts/epub_style_preset_tool.py。
package typography

import "regexp"

// coverageThreshold 对齐 COVERAGE_THRESHOLD。
const coverageThreshold = 0.3

// coverageWarningText 对齐 coverage_report 的中文 warning 文案。
const coverageWarningText = "该书尚未迁入本仓 class 体系，请先走 cleanup pipeline（oneclick 会注入 typography palette）"

// typoLinkRe 对齐 LINK_RE（re.I | re.M）：整行匹配 <link …>，供
// stylesheet link 的整行删除。
var typoLinkRe = regexp.MustCompile(`(?im)(^[ \t]*)<link\b([^>]*)/?>[ \t]*(?:\r?\n)?`)

// typoHeadEndRe 对齐 HEAD_END_RE（re.I | re.M，带缩进组）：
// rewrite_stylesheet_links 的 </head> 前插入点。
var typoHeadEndRe = regexp.MustCompile(`(?im)(^[ \t]*)</head\s*>`)

// idSanitizeRe 对齐 epub_lib.unique_id 的 re.sub(r"[^A-Za-z0-9_.-]+", "-", ...)。
var idSanitizeRe = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)
