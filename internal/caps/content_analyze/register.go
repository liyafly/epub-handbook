// register.go 收纳 contentanalyze 的不可变扫描表（INV-7 白名单：注册表文件，
// 仅 init 期写入）。全部表项逐字对齐 scripts/epub_content_analysis.py，
// 集中一处便于与 Python oracle 对账。
package contentanalyze

import "regexp"

// pySpaceClass 复刻 Python 正则 \s（str 模式）的空白全集：
// ASCII 空白 + \x1c-\x1f + NEL(U+0085) + Unicode Zs/Zl/Zp。
// Go 的 \s 只有 [\t\n\f\r ]，因此所有含 \s 的 Python 模式都用它拼接。
const pySpaceClass = `\t\n\v\f\r \x{1c}-\x{1f}\x{85}\p{Z}`

// blockTags 对齐 Python BLOCK_TAGS（epub_content_analysis.py:24-27）。
var blockTags = map[string]bool{
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"p": true, "blockquote": true, "figcaption": true, "caption": true,
	"li": true, "dt": true, "dd": true, "pre": true, "code": true,
	"td": true, "th": true, "address": true, "hr": true,
}

// typographyRow 是单条排版建议（键序 = Python TYPOGRAPHY dict 插入序）。
type typographyRow struct {
	FontRole   string `json:"font_role"`
	LineHeight string `json:"line_height"`
	TextIndent string `json:"text_indent"`
	TextAlign  string `json:"text_align"`
	Spacing    string `json:"spacing"`
	Pagination string `json:"pagination"`
}

// typographyTable 逐字照抄 Python TYPOGRAPHY 表（:414-432）。
var typographyTable = map[string]typographyRow{
	"title":              {"ht", "1.2", "0", "center", "display", "avoid-after"},
	"heading":            {"ht", "1.3", "0", "inherit", "heading", "avoid-after"},
	"subtitle":           {"kt", "1.4", "0", "center", "compact", "avoid-after"},
	"body":               {"inherit", "1.7", "2em", "justify", "body", "auto"},
	"dialogue":           {"inherit", "inherit", "2em", "inherit", "body", "auto"},
	"quotation":          {"kt", "1.7", "0", "inherit", "extract", "auto"},
	"epigraph":           {"kt", "1.6", "0", "inherit", "epigraph", "avoid-inside"},
	"verse":              {"kt", "1.7", "0", "left", "verse", "auto"},
	"letter":             {"fs", "1.7", "0", "left", "letter", "auto"},
	"list":               {"inherit", "1.6", "0", "left", "list", "auto"},
	"caption":            {"inherit", "1.5", "0", "center", "caption", "avoid-before"},
	"note":               {"inherit", "1.5", "0", "left", "note", "auto"},
	"code":               {"mono", "1.45", "0", "left", "code", "auto"},
	"classical":          {"st", "1.8", "2em", "justify", "classical", "auto"},
	"modern-translation": {"kt", "1.7", "2em", "justify", "translation", "auto"},
	"scene-break":        {"inherit", "normal", "0", "center", "scene-break", "avoid-inside"},
	"unknown":            {"inherit", "preserve", "preserve", "preserve", "preserve", "preserve"},
}

// chapterRe / sceneRe 对齐 Python CHAPTER_RE / SCENE_RE。
// 差异说明：Python \d 是 Unicode Nd（含全角数字等），此处用 \p{Nd} 等价复刻；
// Python \s 用 pySpaceClass 复刻。
var (
	chapterRe = regexp.MustCompile(`(?i)^(?:第[〇零一二三四五六七八九十百千万两\p{Nd}]+[章节卷回部篇]|chapter[` + pySpaceClass + `]+\p{Nd}+)`)
	sceneRe   = regexp.MustCompile(`^(?:[*＊※·•—―\-][` + pySpaceClass + `]*){2,}$`)
)

// quoteChars / sentenceEnd 对齐 Python QUOTE_CHARS / SENTENCE_END 集合。
const (
	quoteChars   = "“”‘’「」『』《》\"'"
	sentenceEnd  = "。！？!?；;：:"
	dashEm       = "—"
	dashEmEm     = "——"
	verseLineMax = 24
	visibleMax   = 200
)

// markdown / plain 源的正则（对齐 analyze 源码里的字面模式）。
var (
	// `^(#{1,6})\s+(.+)$`
	mdHeadingRe = regexp.MustCompile(`^(#{1,6})[` + pySpaceClass + `](.+)$`)
	// `^\s*(?:[-+*]|\d+[.)])\s+`；\d 用 \p{Nd} 复刻 Python 的 Unicode 语义。
	mdListRe = regexp.MustCompile(`^[` + pySpaceClass + `]*(?:[-+*]|\p{Nd}+[.)])[` + pySpaceClass + `]+`)
	// `\n\s*\n`
	plainSplitRe = regexp.MustCompile(`\n[` + pySpaceClass + `]*\n`)
)

// namedHTMLRefs 是 loose-HTML 字符引用解码表（Python 用 html.unescape 的
// 2500 项 HTML5 全集；此处收录中文电子书常见的子集，属已知近似）。
var namedHTMLRefs = map[string]string{
	"amp": "&", "lt": "<", "gt": ">", "quot": "\"", "apos": "'",
	"nbsp": " ", "ensp": " ", "emsp": " ", "thinsp": " ",
	"copy": "©", "reg": "®", "trade": "™", "sect": "§", "para": "¶",
	"deg": "°", "plusmn": "±", "times": "×", "divide": "÷", "middot": "·",
	"bull": "•", "dagger": "†", "Dagger": "‡", "permil": "‰",
	"ldquo": "“", "rdquo": "”", "lsquo": "‘", "rsquo": "’",
	"sbquo": "‚", "bdquo": "„", "laquo": "«", "raquo": "»",
	"mdash": "—", "ndash": "–", "hellip": "…",
	"frac12": "½", "frac14": "¼", "frac34": "¾", "sup2": "²", "sup3": "³",
	"euro": "€", "pound": "£", "yen": "¥", "cent": "¢",
	"lrm": "‎", "rlm": "‏", "zwnj": "‌", "zwj": "‍", "shy": "­",
}

// legacyHTMLRefs 是 HTML5 允许省略分号的 legacy 命名引用子集。
var legacyHTMLRefs = map[string]bool{
	"AMP": true, "LT": true, "GT": true, "QUOT": true,
	"amp": true, "lt": true, "gt": true, "quot": true,
	"nbsp": true, "copy": true, "reg": true,
}
