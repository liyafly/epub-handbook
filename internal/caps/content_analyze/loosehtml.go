// loosehtml.go 复刻 _extract_html_blocks 与 _LooseHTMLCollector：
// 面向不保证 XML 良构的普通 HTML 的容错分块。
//
// Python 用 html.parser.HTMLParser（convert_charrefs=True）。已知近似：
//   - 字符引用解码用本地表（namedHTMLRefs / legacyHTMLRefs）近似
//     html.unescape 的 HTML5 全集；
//   - <script>/<style> 进入 CDATA 模式（内容原样为文本、不解码实体）；
//   - 极端残缺标签（引号未闭合等）按 Python close() 的行为降级为文本。
package contentanalyze

import (
	"fmt"
	"strings"
)

// looseFrame 是 loose 收集器里的一个打开元素（对齐 (tag, values) 元组）。
type looseFrame struct {
	tag       string
	classAttr string
	epubType  string // 小写键 epub:type 的值
	lang      string
	xmlLang   string
}

// looseCurrent 是正在收集的块。
type looseCurrent struct {
	tag      string
	locator  string
	classes  []string
	ancTags  []string
	epubTyps []string
	language string
	hasLang  bool
	parts    []string
}

// looseCollector 对齐 _LooseHTMLCollector 的状态机。
type looseCollector struct {
	source string
	stack  []looseFrame
	counts map[string]int
	blocks []textBlock
	cur    *looseCurrent
}

// extractLooseHTMLBlocks 对齐 _extract_html_blocks。
func extractLooseHTMLBlocks(source, content string) ([]textBlock, error) {
	c := &looseCollector{source: source, counts: map[string]int{}}
	if err := feedLoose(c, content); err != nil {
		return nil, fmt.Errorf("%s: HTML parse failed: %v", source, err)
	}
	return withNeighbors(c.blocks), nil
}

// context 对齐 _context：栈序（外→内）聚合标签 / class / epub:type / 语言。
func (c *looseCollector) context() (tags []string, classes, epubTypes []string, language string, hasLang bool) {
	tags = make([]string, 0, len(c.stack))
	classes = []string{}
	seenClass := map[string]bool{}
	epubTypes = []string{}
	seenType := map[string]bool{}
	for _, fr := range c.stack {
		tags = append(tags, fr.tag)
		for _, cl := range splitPyFields(fr.classAttr) {
			if !seenClass[cl] {
				seenClass[cl] = true
				classes = append(classes, cl)
			}
		}
		for _, et := range splitPyFields(fr.epubType) {
			if !seenType[et] {
				seenType[et] = true
				epubTypes = append(epubTypes, et)
			}
		}
		if fr.lang != "" {
			language = fr.lang
			hasLang = true
		} else if fr.xmlLang != "" {
			language = fr.xmlLang
			hasLang = true
		}
	}
	return tags, classes, epubTypes, language, hasLang
}

// finish 对齐 _finish。
func (c *looseCollector) finish() {
	if c.cur == nil {
		return
	}
	text := pyTrimSpace(strings.Join(c.cur.parts, ""))
	if text != "" || c.cur.tag == "hr" {
		var language *string
		if c.cur.hasLang {
			v := c.cur.language
			language = &v
		}
		c.blocks = append(c.blocks, textBlock{
			source:       c.source,
			locator:      c.cur.locator,
			tag:          c.cur.tag,
			classes:      c.cur.classes,
			ancestorTags: c.cur.ancTags,
			epubTypes:    c.cur.epubTyps,
			language:     language,
			text:         text,
		})
	}
	c.cur = nil
}

// startTag 对齐 handle_starttag（attrs 已是小写键 → 值字典）。
func (c *looseCollector) startTag(tag string, attrs map[string]string) {
	if tag == "br" {
		if c.cur != nil {
			c.cur.parts = append(c.cur.parts, "\n")
		}
		return
	}
	fr := looseFrame{tag: tag, classAttr: attrs["class"], epubType: attrs["epub:type"]}
	if v, ok := attrs["lang"]; ok {
		fr.lang = v
	}
	if v, ok := attrs["xml:lang"]; ok {
		fr.xmlLang = v
	}
	c.stack = append(c.stack, fr)
	if !blockTags[tag] {
		return
	}
	c.finish()
	c.counts[tag]++
	tags, classes, epubTypes, language, hasLang := c.context()
	c.cur = &looseCurrent{
		tag:      tag,
		locator:  fmt.Sprintf("/html/%s[%d]", tag, c.counts[tag]),
		classes:  classes,
		ancTags:  tags,
		epubTyps: epubTypes,
		language: language,
		hasLang:  hasLang,
	}
	if tag == "hr" {
		c.finish()
	}
}

// endTag 对齐 handle_endtag。
func (c *looseCollector) endTag(tag string) {
	if c.cur != nil && c.cur.tag == tag {
		c.finish()
	}
	for i := len(c.stack) - 1; i >= 0; i-- {
		if c.stack[i].tag == tag {
			c.stack = c.stack[:i]
			break
		}
	}
}

// data 对齐 handle_data。
func (c *looseCollector) data(s string) {
	if c.cur != nil {
		c.cur.parts = append(c.cur.parts, s)
	}
}

// feedLoose 是 HTML 容错分词器，产出 startTag / endTag / data 事件。
// dataDecode 跟踪缓冲数据是否需要实体解码：普通文本需要（convert_charrefs），
// <script>/<style> 的 CDATA 内容不需要。
func feedLoose(c *looseCollector, s string) error {
	i := 0
	var data strings.Builder
	dataDecode := true
	flush := func() {
		if data.Len() == 0 {
			return
		}
		text := data.String()
		if dataDecode {
			text = decodeHTMLText(text)
		}
		c.data(text)
		data.Reset()
	}
	put := func(text string, decode bool) {
		if data.Len() > 0 && decode != dataDecode {
			flush()
		}
		dataDecode = decode
		data.WriteString(text)
	}
	cdata := "" // 非空时处于 <script>/<style> 原文模式
	for i < len(s) {
		if cdata != "" {
			flush()
			end := indexCloseTag(s[i:], cdata)
			if end < 0 {
				put(s[i:], false)
				i = len(s)
				break
			}
			put(s[i:i+end], false)
			i += end
			cdata = ""
			continue
		}
		j := strings.IndexByte(s[i:], '<')
		if j < 0 {
			put(s[i:], true)
			break
		}
		put(s[i:i+j], true)
		i += j
		rest := s[i:] // rest[0] == '<'
		switch {
		case strings.HasPrefix(rest, "<!--"):
			flush()
			end := strings.Index(rest[4:], "-->")
			if end < 0 {
				i = len(s)
			} else {
				i += 4 + end + 3
			}
		case strings.HasPrefix(rest, "<!["):
			flush() // CDATA / marked section：HTMLParser 按 unknown_decl 丢弃
			end := strings.Index(rest, "]]>")
			if end < 0 {
				i = len(s)
			} else {
				i += end + 3
			}
		case strings.HasPrefix(rest, "<!"):
			flush() // DOCTYPE / declaration
			i += scanMarkupDecl(rest)
		case strings.HasPrefix(rest, "<?"):
			flush() // processing instruction
			end := strings.IndexByte(rest, '>')
			if end < 0 {
				i = len(s)
			} else {
				i += end + 1
			}
		case strings.HasPrefix(rest, "</") && len(rest) > 2 && isASCIIAlpha(rest[2]):
			flush()
			var next int
			next, cdata = scanEndTag(c, rest, cdata)
			i += next
		case len(rest) > 1 && isASCIIAlpha(rest[1]):
			flush()
			var next int
			next, cdata = scanStartTag(c, rest, cdata)
			i += next
		default:
			// '<' 后不是标记 → 按字面数据处理（HTMLParser 同为逐字吐出）
			put("<", true)
			i++
		}
	}
	flush()
	c.finish() // Python close() 的 _finish：EOF 处未闭合的最后一个块也要产出
	return nil
}

// scanStartTag 解析开始标签，调用 startTag（自闭合再补 endTag）。
// 返回新的扫描偏移与（可能进入的）CDATA 标签名。
func scanStartTag(c *looseCollector, s string, cdata string) (int, string) {
	start := 0 // '<' 的位置，用于未闭合时整体降级为文本
	i := 1
	for i < len(s) && !isWSByte(s[i]) && s[i] != '>' && s[i] != '/' {
		i++
	}
	name := strings.ToLower(s[1:i])
	attrs := map[string]string{}
	selfClose := false
	for i < len(s) {
		j := i
		for j < len(s) && isWSByte(s[j]) {
			j++
		}
		if j >= len(s) {
			break
		}
		if s[j] == '>' {
			i = j + 1
			break
		}
		if s[j] == '/' {
			k := j + 1
			for k < len(s) && isWSByte(s[k]) {
				k++
			}
			if k < len(s) && s[k] == '>' {
				selfClose = true
				i = k + 1
				break
			}
			i = j + 1 // 非自闭合的 '/' 是属性分隔符
			continue
		}
		// 属性名：首字符不能是空白 / '/' / '>'，其余到 空白/'='/'/'/'>' 为止
		nameStart := j
		i = j
		for i < len(s) && !isWSByte(s[i]) && s[i] != '=' && s[i] != '/' && s[i] != '>' {
			i++
		}
		attrName := strings.ToLower(s[nameStart:i])
		attrVal := ""
		p := i
		for p < len(s) && isWSByte(s[p]) {
			p++
		}
		if p < len(s) && s[p] == '=' {
			p++
			for p < len(s) && isWSByte(s[p]) {
				p++
			}
			if p < len(s) && (s[p] == '"' || s[p] == '\'') {
				q := s[p]
				k := strings.IndexByte(s[p+1:], q)
				if k < 0 {
					// 引号未闭合到 EOF：整体按文本降级（HTMLParser close 语义）
					c.data(decodeHTMLText(s[start:]))
					return len(s), cdata
				}
				attrVal = s[p+1 : p+1+k]
				p = p + 1 + k + 1
			} else {
				k := p
				for k < len(s) && !isWSByte(s[k]) && s[k] != '>' {
					k++
				}
				attrVal = s[p:k]
				p = k
			}
			i = p
		}
		if attrName != "" {
			attrs[attrName] = decodeHTMLText(attrVal) // 属性值恒定反转义
		}
	}
	if selfClose {
		c.startTag(name, attrs)
		c.endTag(name)
		return i, cdata
	}
	if i > len(s) {
		i = len(s)
	}
	if strings.HasSuffix(s[:i], ">") {
		c.startTag(name, attrs)
		if name == "script" || name == "style" {
			return i, name
		}
		return i, cdata
	}
	// EOF 处标签未闭合：残余原文降级为文本
	c.data(decodeHTMLText(s[start:]))
	return len(s), cdata
}

// scanEndTag 解析结束标签并调用 endTag。返回新偏移（CDATA 模式不变，
// 结束标签本身即退出 CDATA，由调用方在 startTag 里设置，这里保持传入值）。
func scanEndTag(c *looseCollector, s string, cdata string) (int, string) {
	i := 2
	nameEnd := i
	for nameEnd < len(s) && !isWSByte(s[nameEnd]) && s[nameEnd] != '/' && s[nameEnd] != '>' {
		nameEnd++
	}
	name := strings.ToLower(s[2:nameEnd])
	j := nameEnd
	for j < len(s) && s[j] != '>' {
		if s[j] == '"' || s[j] == '\'' {
			k := strings.IndexByte(s[j+1:], s[j])
			if k < 0 {
				j = len(s)
				break
			}
			j += k + 2
			continue
		}
		j++
	}
	if j < len(s) {
		c.endTag(name)
		return j + 1, cdata
	}
	return len(s), cdata
}

// scanMarkupDecl 跳过 <!...>，尊重引号与 [ 内部子集 ]。
func scanMarkupDecl(s string) int {
	i := 2
	for i < len(s) {
		switch s[i] {
		case '"', '\'':
			k := strings.IndexByte(s[i+1:], s[i])
			if k < 0 {
				return len(s)
			}
			i += k + 2
		case '[':
			k := strings.Index(s[i:], "]>")
			if k < 0 {
				return len(s)
			}
			return i + k + 2
		case '>':
			return i + 1
		default:
			i++
		}
	}
	return len(s)
}

// indexCloseTag 在 CDATA 模式里找大小写不敏感的 "</name"（HTMLParser 的
// interesting 正则 `</\s*name`，允许 </ 与名字间有空白）。
func indexCloseTag(s, name string) int {
	for i := 0; i+1 < len(s); i++ {
		if s[i] != '<' || s[i+1] != '/' {
			continue
		}
		j := i + 2
		for j < len(s) && isWSByte(s[j]) {
			j++
		}
		if j+len(name) <= len(s) && strings.EqualFold(s[j:j+len(name)], name) {
			return i
		}
	}
	return -1
}

func isASCIIAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isWSByte(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\v', '\f', 0x00:
		return true
	}
	return false
}

// decodeHTMLText 近似 html.unescape：数字引用（分号可选）+ 本地命名表。
// 超界数字引用按 HTML5 规则替换为 U+FFFD。
func decodeHTMLText(s string) string {
	if !strings.ContainsRune(s, '&') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '&' {
			b.WriteByte(s[i])
			i++
			continue
		}
		// 数字引用：&#123; / &#x1F;（分号可选）
		if n, r, ok := decodeNumericRef(s[i:]); ok {
			b.WriteRune(r)
			i += n
			continue
		}
		// 命名引用
		k := i + 1
		for k < len(s) && isAlphaNumByte(s[k]) {
			k++
		}
		name := s[i+1 : k]
		consumed := 0
		repl := ""
		if k < len(s) && s[k] == ';' {
			if v, ok := namedHTMLRefs[name]; ok {
				repl = v
				consumed = k + 1 - i
			}
		} else if legacyHTMLRefs[name] {
			if v, ok := namedHTMLRefs[name]; ok {
				repl = v
				consumed = k - i
			}
		}
		if consumed > 0 {
			b.WriteString(repl)
			i += consumed
			continue
		}
		b.WriteByte('&')
		i++
	}
	return b.String()
}

// decodeNumericRef 解析数字引用。ok 时返回消耗字节数与替换 rune；
// 0 / 超出 Unicode 范围 / 代理区按 HTML5 规则返回 U+FFFD。
func decodeNumericRef(t string) (int, rune, bool) {
	if len(t) < 3 || t[1] != '#' {
		return 0, 0, false
	}
	base := 10
	digits := 2
	if t[2] == 'x' || t[2] == 'X' {
		base = 16
		digits = 3
	}
	val := 0
	ok := false
	for digits < len(t) {
		c := t[digits]
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case base == 16 && c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case base == 16 && c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		default:
			d = -1
		}
		if d < 0 {
			break
		}
		val = val*base + d
		if val > 0x10FFFF {
			val = 0x110000 // 标记超界，最终映射 U+FFFD
			break
		}
		digits++
		ok = true
	}
	if !ok {
		return 0, 0, false
	}
	consumed := digits
	if consumed < len(t) && t[consumed] == ';' {
		consumed++
	}
	r := rune(val)
	if val == 0 || val > 0x10FFFF || (r >= 0xD800 && r <= 0xDFFF) {
		r = 0xFFFD
	}
	return consumed, r, true
}

func isAlphaNumByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
