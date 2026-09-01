// register.go 是本包的常量查找表（INV-7 白名单：包级表只允许住在
// register.go）。全部内容逐条对齐 scripts/epub_css_cleanup.py。
package csscleanup

import "regexp"

// 三条系统字体链（SONG_CHAIN / HEI_CHAIN / KAI_CHAIN）。
const (
	songChain = `"Songti SC", "SimSun", "Noto Serif CJK SC", serif`
	heiChain  = `"Heiti SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif`
	kaiChain  = `"Kaiti SC", "STKaiti", "KaiTi", serif`
)

// idSanitizeRe 对齐 add_css_manifest_item 的
// re.sub(r"[^A-Za-z0-9_.-]+", "-", ...)。
var idSanitizeRe = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// scoped-local 合并阶段排除的文件名（excluded_names）。
var scopedExcludedNames = map[string]bool{
	"epub3-enhancements.css":   true,
	"anthology-refinement.css": true,
	"clean-scoped-local.css":   true,
}
