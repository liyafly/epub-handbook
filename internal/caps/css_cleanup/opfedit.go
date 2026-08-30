// opfedit.go 提供 OPF 的字节区间只读定位与元素注入工具。INV-2：只产出
// 区间信息与注入文本，不做整文档序列化 —— OPF 的修改以 []editset.Edit
// 交给 book.Apply（SPEC §6.1 三段式）。
package csscleanup

import (
	"strings"

	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// firstOPFChild 返回第一个 {opf}local 直接子元素（对齐 root.find("opf:xxx")）。
func firstOPFChild(root *opf.SpanNode, local string) *opf.SpanNode {
	for _, c := range root.Kids {
		if c.Name.Space == opf.OPFURI && c.Name.Local == local {
			return c
		}
	}
	return nil
}

// opfManifestItems 对齐 root.findall("opf:manifest/opf:item")：全部
// {opf}manifest 直接子元素下的 {opf}item（保持文档序）。
func opfManifestItems(root *opf.SpanNode) []*opf.SpanNode {
	var out []*opf.SpanNode
	for _, m := range root.Kids {
		if m.Name.Space != opf.OPFURI || m.Name.Local != "manifest" {
			continue
		}
		for _, c := range m.Kids {
			if c.Name.Space == opf.OPFURI && c.Name.Local == "item" {
				out = append(out, c)
			}
		}
	}
	return out
}

// nodeAttr 返回无命名空间属性的值（对齐 item.attrib.get(name)）。
func nodeAttr(n *opf.SpanNode, local string) (string, bool) {
	return n.AttrByLocal("", local)
}

// escapeAttrib 复刻 ElementTree._escape_attrib：属性值转义
// & < > " \r \n \t（单趟替换，顺序与 CPython 一致）。
func escapeAttrib(v string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"\r", "&#13;",
		"\n", "&#10;",
		"\t", "&#09;",
	)
	return r.Replace(v)
}

// opfItemElement 生成与 Python ET.SubElement + tostring 相同形状的
// manifest item 元素文本（空元素为 "<tag ... />"，属性序 = 插入序）。
func opfItemElement(id, href string) string {
	return `<item id="` + escapeAttrib(id) + `" href="` + escapeAttrib(href) + `" media-type="text/css" />`
}

// uniqueID 复刻 epub_lib.unique_id：候选名净化 → 数字开头加 x- →
// 与 idSeen 冲突时追加 -2/-3…（调用方需把新 id 写回 idSeen）。
func uniqueID(idSeen map[string]bool, base string) string {
	candidate := idSanitizeRe.ReplaceAllString(base, "-")
	candidate = strings.Trim(candidate, "-")
	if candidate == "" {
		candidate = "item"
	}
	if candidate[0] >= '0' && candidate[0] <= '9' {
		candidate = "x-" + candidate
	}
	index := 2
	result := candidate
	for idSeen[result] {
		result = candidate + "-" + itoa(index)
		index++
	}
	return result
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// tailEnd 计算元素在原文中的字节终点（含 tail，到下一个 '<' 或 EOF），
// 对齐 ET 删除元素时一并消失的 tail。
func tailEnd(data []byte, n *opf.SpanNode) int {
	span := n.TailAfter(data)
	return span.End
}

// removeElementEdit 生成删除整个元素（含 tail）的字节区间编辑。
func removeElementEdit(path string, data []byte, n *opf.SpanNode) editset.Edit {
	start := n.Open.Start
	end := tailEnd(data, n)
	return editset.Replace(path, int64(start), int64(end-start), []byte{})
}
