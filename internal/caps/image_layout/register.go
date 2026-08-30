// register.go 收纳 imagelayout 的不可变扫描表（INV-7 白名单：注册表文件，
// 仅 init 期写入）。CANDIDATES 逐字照抄 scripts/epub_image_layout_advisor.py
// 的中文候选表（:37-125），键序与文案不得改动。
package imagelayout

import "regexp"

// pySpaceClass 复刻 Python 正则 \s（str 模式）的空白全集（供 re.split 等
// 使用；str.split() 的语义由运行时 splitPyFields 承担）。
const pySpaceClass = `\t\n\v\f\r \x{1c}-\x{1f}\x{85}\p{Z}`

// styleDeclRe 对齐 STYLE_DECL_RE `([-\w]+)\s*:\s*([^;]+)`；
// \w 的 Unicode 语义用 \p{L}\p{N} 加下划线近似，\s 用 pySpaceClass。
var styleDeclRe = regexp.MustCompile(`([-_\p{L}\p{N}]+)[` + pySpaceClass + `]*:[` + pySpaceClass + `]*([^;]+)`)

// cssRuleRe 对齐 CSS_RULE_RE `([^{}]+)\{([^{}]*)\}`（re.S）。
var cssRuleRe = regexp.MustCompile(`(?s)([^{}]+)\{([^{}]*)\}`)

// cssCommentRe 对齐 CSS_COMMENT_RE `/\*.*?\*/`（re.S）。
var cssCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)

// cssClassRe 对齐 CLASS_RE `\.([A-Za-z_][\w-]*)`（\w 用 Unicode 词字符近似）。
var cssClassRe = regexp.MustCompile(`\.([A-Za-z_][\p{L}\p{N}_-]*)`)

// percentRe 对齐 PERCENT_RE `^\s*(\d+(?:\.\d+)?)%`。
// 差异：Python \d 是全 Unicode 十进制数字且 float() 也能解析它们；
// 实际 CSS 宽度值都是 ASCII，这里用 [0-9]。
var percentRe = regexp.MustCompile(`^[` + pySpaceClass + `]*([0-9]+(?:\.[0-9]+)?)%`)

// aliteBodyClasses 对齐 ALITE_BODY_CLASSES。
var aliteBodyClasses = map[string]bool{
	"fullpage": true, "poster-bg": true,
}

// prepaginatedProps 对齐 PREPAGINATED_PROPS。
var prepaginatedProps = map[string]bool{
	"rendition:layout-pre-paginated": true,
}

// captionClasses 对齐 caption-detached 判定里的 {caption, tu-zhu}。
var captionClasses = map[string]bool{
	"caption": true, "tu-zhu": true,
}

// figureFloatClasses 对齐 figure 浮动类 {img-left, img-right}。
var figureFloatClasses = map[string]bool{
	"img-left": true, "img-right": true,
}

// candidate 是候选建议（键序 id / summary / risk）。
type candidate struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Risk    string `json:"risk"`
}

// candidates 逐字照抄 Python CANDIDATES（:37-125）。
var candidates = map[string][]candidate{
	"lone-image-no-figure": {
		{"figure.img-left", "左浮动 figure，宽度从 25%–35% 起步", "SPEC §5.6；短段落环绕会塌，见 demo 17-image-layout 反例。"},
		{"figure.img-right", "右浮动 figure，宽度从 25%–35% 起步", "SPEC §5.6；reader-matrix 17-image-layout 当前仍需大字号复测。"},
		{"figure-fullwidth", "通栏 figure，可附 figcaption", "SPEC §5.6；不使用 float，仍需目标阅读器人工确认。"},
	},
	"caption-detached": {
		{"figure.figcaption", "把图片与短图注并入同一 figure/figcaption", "SPEC §5.6；figure 与可选 figcaption 是通用路径。"},
		{"keep-separate-caption", "保留独立段落，但显式标记其非图注角色", "未实测，见 reader-matrix 待验证项；需人工确认该短段确为正文。"},
	},
	"float-width-risk": {
		{"figure.img-left", "float 与 25%–35% 宽度放到左浮动 figure", "SPEC §5.6；内层 img 使用 width:100%; height:auto。"},
		{"figure.img-right", "float 与 25%–35% 宽度放到右浮动 figure", "SPEC §5.6；reader-matrix 17-image-layout 仍为 warn。"},
		{"figure-fullwidth", "取消环绕，改为普通通栏 figure", "SPEC §5.6；正文过短或大字号时更保守。"},
	},
	"missing-alt": {
		{"add-alt-text", "人工填写表达图片信息的 alt", "未实测，见 reader-matrix 待验证项；工具不得猜测图片内容。"},
		{"decorative-empty-alt", "确认纯装饰后使用空 alt", "未实测，见 reader-matrix 待验证项；只有人工确认装饰角色后可选。"},
	},
	"chapter-head-image-candidate": {
		{"keep-current", "维持当前普通图片结构", "SPEC §5.11；仍需确认图片不会抢占章节标题语义。"},
		{"chapter-head-art", "转为 chapter-head-art 图片槽位", "SPEC §5.11；reader-matrix 20-chapter-head-image 当前为 warn。"},
	},
	"fullpage-image-alite-candidate": {
		{"alite-contain", "A-lite contain，保留整张图与留白", "SPEC §2；reader-matrix 03c-poster-contain 的 Kindle Previewer 3.104 转换为 0 errors，GUI 仍待复测。"},
		{"alite-fullbleed", "A-lite fullbleed，允许按视口裁切", "SPEC §2；03b-poster-fullbleed 已退役为历史对照，默认主路径优先 alite-contain。"},
		{"figure-fullwidth", "保留普通可重排整页 figure", "SPEC §5.6；不启用 A-lite，分页效果需目标阅读器确认。"},
	},
}
