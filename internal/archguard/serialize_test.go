package archguard

import (
	"strings"
	"testing"
)

// bannedSerializerPrefix 是「整文档序列化」的典型命名。
// scan/* 只该产出 []editset.Edit，任何看起来像"把文档吐回去"的导出函数都是信号。
var bannedSerializerPrefix = []string{
	"Marshal", "Serialize", "Render", "Encode", "Format",
	"ToXML", "ToHTML", "ToXHTML", "ToCSS", "Dump", "Emit",
	// 2026-09-07 补充（经仓库所有者授权，只扩覆盖面不放宽规则）：
	// 硬判据只拦 []byte 返回值，而本包的文档注释又明确允许「读取小片段返回
	// string」，于是一个返回 string 的整文档函数只要不叫上面那些名字就能同时
	// 绕过两条判据。下面这批是「把整份文档交出去」最常见的命名。
	"String", "Bytes", "Whole", "Document", "Unparse", "Reserialize",
}

// 未列入的判断（记下来，避免被当成遗漏）：
//   - "Build" 不禁。它的形状是「从结构化条目生成一份新文档」（如从 TOC 条目
//     生成 nav / NCX），输入侧没有原文档，不存在 INV-2 要防的解析→重序列化
//     往返破坏。这是一个在受守卫不变式上的judgement call，值得人类复核。
//   - 「首参是文档、单一 string 返回值」这种结构判据没有采用：scan/css 的
//     StripComments 正是这个形状且是正当用途，加了就要开白名单，而开白名单
//     本身被规则 0 禁止。这条缺口靠 AST 关不掉，如实留在这里。

// TestNoWholeDocSerializer 断言 INV-2：scan/* 不得导出整文档序列化能力。
//
// Go 的 encoding/xml 往返会静默改写命名空间前缀、自闭合标签、实体和 DOCTYPE，
// 这对本项目的正文不变 gate 是致命的。只要不整篇重序列化，风险归零 ——
// 这也是"Go 的 XML 短板被架构消解"这一判断的落地点。
func TestNoWholeDocSerializer(t *testing.T) {
	root := repoRoot(t)
	files := collect(t, root, "internal/scan", false)

	for _, g := range files {
		for _, fn := range exportedFuncs(g) {
			name := fn.Name.Name

			for _, p := range bannedSerializerPrefix {
				if strings.HasPrefix(name, p) {
					t.Errorf("%s: scan 包导出了 %s()，命中禁用前缀 %q。\n"+
						"  INV-2：scan/* 只能产出 []editset.Edit{Offset,Length,Replacement}，\n"+
						"  不得提供任何整文档序列化路径。",
						g.pos(fn), name, p)
				}
			}

			// 返回整份文档字节 —— 比命名更硬的判据。
			for _, r := range results(g, fn) {
				if r == "[]byte" {
					t.Errorf("%s: scan 包的导出函数 %s() 返回 []byte。\n"+
						"  INV-2：禁止把文档字节整体交出去。需要表达改动，请返回 []editset.Edit。\n"+
						"  （读取小片段请返回 string 或显式的区间类型。）",
						g.pos(fn), name)
					break
				}
			}
		}
	}
}
