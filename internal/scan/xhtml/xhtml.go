// Package xhtml 提供 XHTML 的只读扫描原语（SPEC §1 第 4 层）。
//
// 只产出区间信息，绝不序列化整文档（INV-2）。
// 标签匹配语义与 Python 侧的正则一致：<name\b 词边界命中；属性区
// 延伸到第一个 '>'（与 `[^>]*` 相同，不感知引号）。
package xhtml

import "strings"

// Span 是文档中的字节区间 [Start, End)。
type Span struct {
	Start int
	End   int
}

// Attr 是开标签里的一个属性，区间是文档绝对坐标。
type Attr struct {
	Name      string // local 名（去命名空间前缀）
	Raw       string // 原文名（含前缀）
	Value     string
	ValueSpan Span // 值内容（不含引号）
	Quote     byte // 0 表示无引号/无值
}

// Tag 是一个开标签，区间是文档绝对坐标。
type Tag struct {
	Span      Span // 完整开标签，含 < 与 >
	Name      string
	Attrs     []Attr
	SelfClose bool
}

// Attr 按名称查找属性：先精确匹配原文名（含前缀，大小写不敏感），
// 再退回 local 名。这样 Attr("lang") 不会误中 xml:lang。
func (t Tag) Attr(name string) (Attr, bool) {
	for _, a := range t.Attrs {
		if strings.EqualFold(a.Raw, name) {
			return a, true
		}
	}
	for _, a := range t.Attrs {
		if strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return Attr{}, false
}

// FindOpenTag 从 from 起找第一个名字匹配（大小写不敏感）的开标签。
func FindOpenTag(content, name string, from int) (Tag, bool) {
	lower := strings.ToLower(content)
	needle := "<" + strings.ToLower(name)
	for pos := from; pos < len(content); {
		i := strings.Index(lower[pos:], needle)
		if i < 0 {
			return Tag{}, false
		}
		i += pos
		after := i + len(needle)
		if after < len(content) && isWordByte(content[after]) {
			pos = i + len(needle)
			continue
		}
		end := strings.IndexByte(content[i:], '>')
		if end < 0 {
			return Tag{}, false
		}
		end += i
		rawName, attrs, selfClose := parseTagInner(content, i+1, end)
		if !strings.EqualFold(rawName, name) {
			pos = i + len(needle)
			continue
		}
		return Tag{Span: Span{Start: i, End: end + 1}, Name: rawName, Attrs: attrs, SelfClose: selfClose}, true
	}
	return Tag{}, false
}

// FindCloseTag 从 from 起找第一个 </name>（大小写不敏感），返回完整区间。
func FindCloseTag(content, name string, from int) (Span, bool) {
	needle := "</" + name
	for pos := from; pos < len(content); {
		i := strings.Index(strings.ToLower(content[pos:]), needle)
		if i < 0 {
			return Span{}, false
		}
		i += pos
		j := i + len(needle)
		for j < len(content) && isSpaceByte(content[j]) {
			j++
		}
		if j < len(content) && content[j] == '>' {
			return Span{Start: i, End: j + 1}, true
		}
		pos = i + len(needle)
	}
	return Span{}, false
}

// TagsIn 返回文档中全部同名开标签（按文档序）。
func TagsIn(content, name string) []Tag {
	var out []Tag
	from := 0
	for {
		tag, ok := FindOpenTag(content, name, from)
		if !ok {
			return out
		}
		out = append(out, tag)
		from = tag.Span.End
	}
}

// AttrEdit 计算把 tag 的 attr 设为 value 所需的字节区间替换。
// 属性已存在 → 替换值区间（保留原引号）；不存在 → 在开标签末尾追加。
// ok=false 表示属性已是目标值，无需改动。
func AttrEdit(content string, tag Tag, attr, value string) (span Span, replacement string, ok bool) {
	if a, found := tag.Attr(attr); found {
		if a.Value == value {
			return Span{}, "", false
		}
		if a.Quote != 0 {
			return a.ValueSpan, escapeAttr(value, a.Quote), true
		}
		// 无引号属性：把整个 name="value" 连带替换（借属性值回溯到 name 起点）。
		start := a.ValueSpan.Start
		for start > tag.Span.Start+1 && !isSpaceByte(content[start-1]) && content[start-1] != '=' {
			start--
		}
		for start > tag.Span.Start+1 && isSpaceByte(content[start-1]) {
			start--
		}
		eq := strings.IndexByte(content[start:a.ValueSpan.End], '=')
		_ = eq
		return Span{Start: start, End: a.ValueSpan.End}, attr + `="` + escapeAttr(value, '"') + `"`, true
	}
	insert := tag.Span.End - 1
	if tag.SelfClose && insert > tag.Span.Start && content[insert-1] == '/' {
		insert--
	}
	return Span{Start: insert, End: insert}, ` ` + attr + `="` + escapeAttr(value, '"') + `"`, true
}

func escapeAttr(v string, quote byte) string {
	v = strings.ReplaceAll(v, "&", "&amp;")
	v = strings.ReplaceAll(v, "<", "&lt;")
	if quote == '\'' {
		return strings.ReplaceAll(v, "'", "&#39;")
	}
	return strings.ReplaceAll(v, `"`, "&quot;")
}

func isWordByte(b byte) bool {
	return b == '-' || b == '_' || b == ':' || b == '.' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

// parseTagInner 解析 [lo, hi) 之间的标签内部（不含 < 与 >），区间为绝对坐标。
func parseTagInner(content string, lo, hi int) (name string, attrs []Attr, selfClose bool) {
	s := content[lo:hi]
	trimmed := strings.TrimRight(s, " \t\r\n")
	selfClose = strings.HasSuffix(trimmed, "/")
	bodyEnd := hi
	if selfClose {
		bodyEnd = lo + len(trimmed) - 1
	}
	i := lo
	for i < bodyEnd && !isSpaceByte(content[i]) {
		i++
	}
	name = strings.TrimSpace(content[lo:i])
	for i < bodyEnd {
		for i < bodyEnd && isSpaceByte(content[i]) {
			i++
		}
		if i >= bodyEnd {
			break
		}
		start := i
		for i < bodyEnd && content[i] != '=' && !isSpaceByte(content[i]) {
			i++
		}
		attrName := content[start:i]
		eq := i
		for eq < bodyEnd && isSpaceByte(content[eq]) {
			eq++
		}
		if eq >= bodyEnd || content[eq] != '=' {
			// 无值属性。
			attrs = append(attrs, Attr{Name: localOf(attrName), Raw: attrName})
			continue
		}
		vs := eq + 1
		for vs < bodyEnd && isSpaceByte(content[vs]) {
			vs++
		}
		attr := Attr{Name: localOf(attrName), Raw: attrName}
		if vs < bodyEnd && (content[vs] == '"' || content[vs] == '\'') {
			q := content[vs]
			end := strings.IndexByte(content[vs+1:bodyEnd], q)
			if end < 0 {
				attr.Quote = q
				attr.Value = content[vs+1 : bodyEnd]
				attr.ValueSpan = Span{Start: vs + 1, End: bodyEnd}
				i = bodyEnd
			} else {
				attr.Quote = q
				attr.Value = content[vs+1 : vs+1+end]
				attr.ValueSpan = Span{Start: vs + 1, End: vs + 1 + end}
				i = vs + 1 + end + 1
			}
		} else {
			ve := vs
			for ve < bodyEnd && !isSpaceByte(content[ve]) {
				ve++
			}
			attr.Value = content[vs:ve]
			attr.ValueSpan = Span{Start: vs, End: ve}
			i = ve
		}
		attrs = append(attrs, attr)
	}
	return name, attrs, selfClose
}

func localOf(name string) string {
	if i := strings.LastIndexByte(name, ':'); i >= 0 {
		return name[i+1:]
	}
	return name
}
