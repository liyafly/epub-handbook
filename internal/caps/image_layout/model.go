// model.go 提供只读的轻量 XML 节点树：一次流式解码建成投影，
// 之后所有判定都跑在这棵只读树上；绝不序列化回字节（INV-2）。
package imagelayout

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ixNode 是一个元素节点的只读投影。
type ixNode struct {
	tag      string // local 名，保留原大小写（selector_for 不做 lower）
	attrs    []xml.Attr
	attrMap  map[string]string // Python attrib 键形状："src" 或 "{ns}local"
	children []*ixNode
	parent   *ixNode
	text     string // 本节点直接字符数据（不含后代）
	tail     string // 本节点结束标签后的字符数据（挂在父上下文）
	index    int    // 同 local tag 兄弟中的序号（1 起，大小写敏感）
	plainIdx int    // 全部元素兄弟中的位置（1 起）
}

// attrValue 按 Python attrib 键形状取属性值（不存在返回 ""）。
func attrValue(n *ixNode, key string) string {
	if n == nil {
		return ""
	}
	return n.attrMap[key]
}

// subtree 以文档序收集自身与全部后代。
func (n *ixNode) subtree() []*ixNode {
	out := []*ixNode{n}
	for _, c := range n.children {
		out = append(out, c.subtree()...)
	}
	return out
}

// iterText 对齐 ElementTree 的 itertext：text → 逐子树 → 子 tail。
func (n *ixNode) iterText(sb *strings.Builder) {
	sb.WriteString(n.text)
	for _, c := range n.children {
		c.iterText(sb)
		sb.WriteString(c.tail)
	}
}

// visibleText 对齐 visible_text：拼接 itertext 后剔除全部 \s 字符。
func visibleText(n *ixNode) string {
	var sb strings.Builder
	n.iterText(&sb)
	return stripPySpace(sb.String())
}

// parseXMLTree 流式解码 XML 为只读节点树（严格模式，语义对齐 ElementTree）。
func parseXMLTree(ctx context.Context, data []byte) (*ixNode, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = true
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil // 输入已是 UTF-8 文本
	}
	type frame struct {
		node   *ixNode
		counts map[string]int
	}
	var stack []frame
	var root *ixNode
	var lastClosed *ixNode // 结束标签后的 CharData 归入 tail

	appendText := func(s string) {
		if lastClosed != nil {
			lastClosed.tail += s
			return
		}
		if len(stack) > 0 {
			top := stack[len(stack)-1].node
			top.text += s
		}
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &ixNode{tag: t.Name.Local, index: 1, plainIdx: 1}
			n.attrs = append([]xml.Attr(nil), t.Attr...)
			n.attrMap = make(map[string]string, len(t.Attr))
			for _, a := range t.Attr {
				key := a.Name.Local
				if a.Name.Space != "" {
					key = "{" + a.Name.Space + "}" + a.Name.Local
				}
				n.attrMap[key] = a.Value
			}
			if len(stack) > 0 {
				top := &stack[len(stack)-1]
				n.parent = top.node
				top.node.children = append(top.node.children, n)
				n.plainIdx = len(top.node.children)
				if top.counts == nil {
					top.counts = map[string]int{}
				}
				top.counts[n.tag]++
				n.index = top.counts[n.tag]
			} else {
				root = n
			}
			stack = append(stack, frame{node: n})
			lastClosed = nil
		case xml.CharData:
			appendText(string(t))
		case xml.EndElement:
			if len(stack) > 0 {
				lastClosed = stack[len(stack)-1].node
				stack = stack[:len(stack)-1]
			}
		case xml.Comment:
			lastClosed = nil // ET 中注释后的文本属于注释的 tail，不入元素
		}
	}
	return root, nil
}

// ---- Python 语义工具（caps 之间禁止互相 import，本包自持一份） ----

// isPySpace 覆盖 Python str.isspace / 正则 \s 的全集。
func isPySpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// splitPyFields 复刻 str.split()（按 Unicode 空白切分，无空元素）。
func splitPyFields(value string) []string {
	fields := strings.FieldsFunc(value, isPySpace)
	if fields == nil {
		return []string{}
	}
	return fields
}

// stripPySpace 复刻 re.sub(r"\s+", "", text)：剔除全部空白字符。
func stripPySpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !isPySpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// runeLen 按码点计数（Python len(str) 语义）。
func runeLen(s string) int { return utf8.RuneCountInString(s) }
