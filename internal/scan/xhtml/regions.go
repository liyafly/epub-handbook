package xhtml

import "strings"

// ---- 标记区域扫描（只定位可改写的区间，不构树、不重序列化） ----
//
// 引用重写只允许发生在真实标签内部（属性值）、<style> 元素的字符数据里，
// 以及 <?xml-stylesheet …?> 处理指令的 href 伪属性上。
// 字符数据中被实体转义的「看起来像属性」的文字（`&lt;img src="…"/&gt;`）、
// 注释、CDATA、其它处理指令、DOCTYPE 与 <script> 内容都必须原字节保留，
// 否则会改写作者正文（redline text 红线）。

type RegionKind int

const (
	// RegionTag 是一个完整的开始标签 / 空元素标签 / 结束标签（含首尾 <>）。
	RegionTag RegionKind = iota
	// RegionStyle 是 <style> 开始标签之后、</style> 之前的全部字符数据。
	RegionStyle
	// RegionStylesheetPI 是 <?xml-stylesheet …?> 处理指令的完整字节区间
	// （含首尾 <? ?>）。它不是标签：只有 href 伪属性可改写。
	RegionStylesheetPI
)

type Region struct {
	Kind RegionKind
	Span // 半开字节区间 [Start, End)
}

// ScanComplete 表示 ScanRegions 扫完了整份文档（没有截断）。
const ScanComplete = -1

// ScanRegions 前向单遍扫描，按出现顺序返回文档里的标签区域、
// <style> 内容区域与 xml-stylesheet 处理指令区域。其它字节（正文、注释、
// CDATA、其它 PI、DOCTYPE、<script> 内容）不产出区域，调用方对它们只做透传。
//
// 第二个返回值是截断偏移：扫描完整时为 ScanComplete（-1），遇到无法闭合的
// 结构时为「从该字节起不再扫描」的偏移。调用方必须把它转成告警，否则改名后
// 文件尾部的引用会静默断链。
//
// 判定规则：
//   - `<!--` … `-->` 整段跳过（兼容 HTML 空注释 `<!-->` / `<!--->`）；
//   - `<![CDATA[` … `]]>`、`<?` … `?>` 整段跳过，其中 `<?xml-stylesheet …?>`
//     另记为 RegionStylesheetPI；
//   - `<!` … `>`（DOCTYPE 等声明）整段跳过；其内部子集 `[` … `]` 里的 `>`
//     不结束声明，子集里的注释按注释跳过（注释内的引号不参与配对）；
//   - `<` 后紧跟名字起始字符（字母、`_`、`:`）或 `/` 才是标签，标签在
//     引号外的第一个 `>` 结束（属性值内的 `>` 不算）；
//   - 其它 `<`（后接空白、数字、`&` 等）按普通字符处理；
//   - 非自闭合的 <style>/<script> 开始标签之后，直到对应结束标签之前的
//     字节分别记为 RegionStyle / 跳过。
//   - 未闭合的结构（缺 `-->`、`]]>`、`?>`、`>`、`</style>`）一律把剩余字节
//     视为不可改写，并通过截断偏移上报。
func ScanRegions(text string) ([]Region, int) {
	var regions []Region
	i := 0
	for i < len(text) {
		lt := strings.IndexByte(text[i:], '<')
		if lt < 0 {
			break
		}
		lt += i
		rest := text[lt:]
		switch {
		case strings.HasPrefix(rest, "<!--"):
			end := regionCommentEnd(rest)
			if end < 0 {
				return regions, lt
			}
			i = lt + end
		case strings.HasPrefix(rest, "<![CDATA["):
			end := strings.Index(rest[9:], "]]>")
			if end < 0 {
				return regions, lt
			}
			i = lt + 9 + end + 3
		case strings.HasPrefix(rest, "<?"):
			end := strings.Index(rest[2:], "?>")
			if end < 0 {
				return regions, lt
			}
			piEnd := lt + 2 + end + 2
			if isXMLStylesheetPI(rest) {
				regions = append(regions, Region{Kind: RegionStylesheetPI, Span: Span{Start: lt, End: piEnd}})
			}
			i = piEnd
		case strings.HasPrefix(rest, "<!"):
			end := findRegionDeclClose(text, lt+2)
			if end < 0 {
				return regions, lt
			}
			i = end + 1
		case len(rest) > 1 && (isRegionNameStartByte(rest[1]) || rest[1] == '/'):
			end := findRegionTagClose(text, lt+1)
			if end < 0 {
				return regions, lt
			}
			regions = append(regions, Region{Kind: RegionTag, Span: Span{Start: lt, End: end + 1}})
			i = end + 1
			if rest[1] == '/' || text[end-1] == '/' {
				continue
			}
			name := regionTagName(text[lt+1 : end])
			switch {
			case strings.EqualFold(name, "style"):
				close := indexFoldCloseTag(text, i, "</style")
				if close < 0 {
					return regions, i
				}
				if close > i {
					regions = append(regions, Region{Kind: RegionStyle, Span: Span{Start: i, End: close}})
				}
				i = close
			case strings.EqualFold(name, "script"):
				close := indexFoldCloseTag(text, i, "</script")
				if close < 0 {
					return regions, i
				}
				i = close
			}
		default:
			i = lt + 1
		}
	}
	return regions, ScanComplete
}

// regionCommentEnd 返回以 `<!--` 开头的 rest 里注释的结束偏移（半开区间），
// 找不到结束返回 -1。除常规 `-->` 外，还接受 HTML 的空注释写法
// `<!-->` 与 `<!--->`（XML 里非法，但真实书里出现过，按注释跳过比
// 放弃扫描整份文档安全）。
func regionCommentEnd(rest string) int {
	body := rest[4:]
	switch {
	case strings.HasPrefix(body, ">"):
		return 5
	case strings.HasPrefix(body, "->"):
		return 6
	}
	end := strings.Index(body, "-->")
	if end < 0 {
		return -1
	}
	return 4 + end + 3
}

// isXMLStylesheetPI 判断 rest 是否是 `<?xml-stylesheet …?>` 处理指令：
// 目标名大小写不敏感，且其后必须是空白或 `?`（避免误判
// `<?xml-stylesheet-foo …?>` 这类别的目标）。
func isXMLStylesheetPI(rest string) bool {
	const target = "<?xml-stylesheet"
	if len(rest) <= len(target) || !strings.EqualFold(rest[:len(target)], target) {
		return false
	}
	switch rest[len(target)] {
	case ' ', '\t', '\n', '\r', '?':
		return true
	}
	return false
}

// findRegionTagClose 从 from 起找到引号外的第一个 '>'，返回其下标；找不到返回 -1。
func findRegionTagClose(text string, from int) int {
	var quote byte
	for j := from; j < len(text); j++ {
		c := text[j]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '>':
			return j
		}
	}
	return -1
}

// findRegionDeclClose 从 from 起找到 `<!` 声明（DOCTYPE 等）的结束 '>'，返回其
// 下标；找不到返回 -1。与 findRegionTagClose 的差别有两点，都是为了不被 DOCTYPE
// 内部子集带偏：
//   - 内部子集 `[` … `]` 里的 '>' 属于子集内的标记声明，不结束声明；
//   - 子集里可以出现注释，注释内的撇号（如 `<!-- don't -->`）不得被当成
//     引号起始，否则会一路扫到 EOF 并放弃整份文档。
func findRegionDeclClose(text string, from int) int {
	var quote byte
	depth := 0
	for j := from; j < len(text); {
		c := text[j]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
			j++
		case strings.HasPrefix(text[j:], "<!--"):
			end := regionCommentEnd(text[j:])
			if end < 0 {
				return -1
			}
			j += end
		case c == '"' || c == '\'':
			quote = c
			j++
		case c == '[':
			depth++
			j++
		case c == ']':
			if depth > 0 {
				depth--
			}
			j++
		case c == '>':
			if depth == 0 {
				return j
			}
			j++
		default:
			j++
		}
	}
	return -1
}

// isRegionNameStartByte 判断 XML 名字起始字符（ASCII 字母、'_'、':'，或非 ASCII 首字节）。
func isRegionNameStartByte(c byte) bool {
	return c == '_' || c == ':' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

// TagParts 从一个完整标签的字节（含首尾 `<>`，即 ScanRegions 返回的
// RegionTag 区间）切出标签名、属性子串与「是否闭合标签」。
//
// 闭合标签（`</name>`）的 name 不含前导 `/`；attrs 是标签名之后的原样字节
// （可能带自闭合的 `/`）。调用方拿到 attrs 后自行按需匹配属性。
//
// 放在这里而不是各 caps 包里：`caps` 之间禁止互相 import（SPEC §1），
// 于是「每个包自己复制一份小工具」是最容易走上的路 —— 本仓已经因此积累了
// 数千行逐字节副本。区域扫描的配套工具属于层 4，两个方向都合法。
func TagParts(tag string) (name, attrs string, closing bool) {
	if len(tag) < 2 || tag[0] != '<' || tag[len(tag)-1] != '>' {
		return "", "", false
	}
	inner := tag[1 : len(tag)-1]
	if strings.HasPrefix(inner, "/") {
		closing = true
		inner = inner[1:]
	}
	name = regionTagName(inner)
	return name, inner[len(name):], closing
}

// TagAttrs parses attributes from one complete tag. It preserves the source
// bytes through spans and returns ok=false when the tag's attribute syntax is
// malformed. Name and spans refer to the original tag bytes; ValueSpan excludes
// any surrounding quotes. Closing tags have no attributes.
func TagAttrs(tag string) ([]Attr, bool) {
	if len(tag) < 3 || tag[0] != '<' || tag[len(tag)-1] != '>' {
		return nil, false
	}
	name, _, closing := TagParts(tag)
	if name == "" {
		return nil, false
	}
	if closing {
		return nil, true
	}
	end := len(tag) - 1
	i := 1 + len(name)
	var attrs []Attr
	for i < end {
		for i < end && isTagSpace(tag[i]) {
			i++
		}
		if i >= end {
			return attrs, true
		}
		if tag[i] == '/' {
			if i == end-1 {
				return attrs, true
			}
			return nil, false
		}
		nameStart := i
		for i < end && !isTagSpace(tag[i]) && tag[i] != '=' && tag[i] != '/' && tag[i] != '>' {
			i++
		}
		if i == nameStart {
			return nil, false
		}
		nameEnd := i
		for i < end && isTagSpace(tag[i]) {
			i++
		}
		name := tag[nameStart:nameEnd]
		attr := Attr{Name: name, Raw: name, NameSpan: Span{Start: nameStart, End: nameEnd}}
		if i >= end || tag[i] != '=' {
			attrs = append(attrs, attr)
			continue
		}
		i++
		for i < end && isTagSpace(tag[i]) {
			i++
		}
		if i >= end {
			return nil, false
		}
		if tag[i] == '"' || tag[i] == '\'' {
			attr.Quote = tag[i]
			valueStart := i + 1
			valueEnd := strings.IndexByte(tag[valueStart:end], attr.Quote)
			if valueEnd < 0 {
				return nil, false
			}
			valueEnd += valueStart
			attr.ValueSpan = Span{Start: valueStart, End: valueEnd}
			attr.Value = tag[valueStart:valueEnd]
			i = valueEnd + 1
			if i < end && !isTagSpace(tag[i]) {
				if tag[i] != '/' || i != end-1 {
					return nil, false
				}
			}
		} else {
			valueStart := i
			for i < end && !isTagSpace(tag[i]) {
				i++
			}
			if i == valueStart {
				return nil, false
			}
			attr.ValueSpan = Span{Start: valueStart, End: i}
			attr.Value = tag[valueStart:i]
		}
		attrs = append(attrs, attr)
	}
	return attrs, true
}

func isTagSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// regionTagName 取标签内容（不含 '<' 与 '>'）开头的名字。
func regionTagName(inner string) string {
	for j := 0; j < len(inner); j++ {
		c := inner[j]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '/' || c == '>' {
			return inner[:j]
		}
	}
	return inner
}

// indexFoldCloseTag 从 from 起大小写不敏感地查找结束标签前缀（如 "</style"），
// 且要求前缀之后紧跟 '>' 或空白；返回其下标，找不到返回 -1。
func indexFoldCloseTag(text string, from int, prefix string) int {
	n := len(prefix)
	for j := from; j+n <= len(text); j++ {
		if text[j] != '<' || !strings.EqualFold(text[j:j+n], prefix) {
			continue
		}
		if j+n == len(text) {
			return j
		}
		switch text[j+n] {
		case '>', ' ', '\t', '\n', '\r':
			return j
		}
	}
	return -1
}
