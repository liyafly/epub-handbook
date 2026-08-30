// Package css 提供 CSS 的只读扫描原语（SPEC §1 第 4 层）。
//
// 只产出区间信息，绝不序列化整文档（INV-2）。
// 规则迭代语义对齐 Python 侧的 `([^{}]+)\{([^{}]*)\}`：
// 注释剥离后迭代；@media/@supports 头本身不构成规则，其内层规则会被捕获。
package css

import (
	"strings"
)

// Span 是文本中的字节区间 [Start, End)。
type Span struct {
	Start int
	End   int
}

// CommentSpan 是原文中的一段注释。
type CommentSpan struct {
	Span
	Text string
}

// Comments 返回原文中的全部注释区间（含 /* */ 定界符）。
func Comments(text string) []CommentSpan {
	var out []CommentSpan
	for _, loc := range commentRe.FindAllStringIndex(text, -1) {
		out = append(out, CommentSpan{Span{loc[0], loc[1]}, text[loc[0]:loc[1]]})
	}
	return out
}

// StripComments 移除全部注释，返回新文本与原文→新文本的坐标映射
// （map[len(原文)区间起点] = 新文本坐标起点；仅对注释之后的锚点有意义）。
func StripComments(text string) string {
	return commentRe.ReplaceAllString(text, "")
}

// Rule 是一条 CSS 规则（选择器 + 声明体），区间相对传入文本。
type Rule struct {
	Selector     string
	SelectorSpan Span
	Body         string
	BodySpan     Span
	Span         Span // 完整规则（含大括号）
}

// Rules 对齐 Python iter_css_rules：迭代 `([^{}]+){([^{}]*)}`，
// 选择器做 trim。@media 头会因正则语义被自然跳过，内层规则被捕获。
func Rules(text string) []Rule {
	var out []Rule
	for _, m := range ruleRe.FindAllStringSubmatchIndex(text, -1) {
		selStart, selEnd := m[2], m[3]
		bodyStart, bodyEnd := m[4], m[5]
		lead := len(text[selStart:selEnd]) - len(strings.TrimLeft(text[selStart:selEnd], " \t\r\n"))
		out = append(out, Rule{
			Selector:     strings.TrimSpace(text[selStart:selEnd]),
			SelectorSpan: Span{Start: selStart + lead, End: selEnd},
			Body:         text[bodyStart:bodyEnd],
			BodySpan:     Span{Start: bodyStart, End: bodyEnd},
			Span:         Span{Start: m[0], End: m[1]},
		})
	}
	return out
}

// Decl 是声明体内的一条声明，区间相对声明体文本。
type Decl struct {
	Name      string
	Value     string
	Span      Span // 完整声明（不含分号）
	ValueSpan Span
}

// Declarations 解析声明体（不含大括号）中的声明列表，区间相对 body。
// 切分对齐 Python 的 split(";")。
func Declarations(body string) []Decl {
	var out []Decl
	pos := 0
	for _, part := range strings.Split(body, ";") {
		start := pos
		pos += len(part) + 1
		i := strings.IndexByte(part, ':')
		if i < 0 || strings.TrimSpace(part[:i]) == "" {
			continue
		}
		valueTail := part[i+1:]
		lead := len(valueTail) - len(strings.TrimLeft(valueTail, " \t\r\n"))
		trail := len(part) - len(strings.TrimRight(part, " \t\r\n"))
		out = append(out, Decl{
			Name:  strings.TrimSpace(part[:i]),
			Value: strings.TrimSpace(valueTail),
			Span:  Span{Start: start, End: start + len(part)},
			ValueSpan: Span{
				Start: start + i + 1 + lead,
				End:   start + len(part) - trail,
			},
		})
	}
	return out
}

// FontFamilyDecls 返回声明体中全部 font-family 声明（含冒号后的值区间）。
// 对齐 Python FONT_FAMILY_RE `(font-family\s*:\s*)([^;}]+)`（re.I）。
func FontFamilyDecls(body string) []FontFamilyDecl {
	var out []FontFamilyDecl
	for _, m := range fontDeclRe.FindAllStringSubmatchIndex(body, -1) {
		out = append(out, FontFamilyDecl{
			WholeSpan: Span{Start: m[0], End: m[1]},
			PrefixEnd: m[3],
			ValueSpan: Span{Start: m[2], End: m[3]},
			Value:     body[m[2]:m[3]],
		})
	}
	return out
}

// FontFamilyDecl 是 font-family 声明的定位信息。
type FontFamilyDecl struct {
	WholeSpan Span // 含 `font-family: ` 前缀
	ValueSpan Span // 值区间
	PrefixEnd int  // 值区间的起点（即前缀结束）
	Value     string
}
