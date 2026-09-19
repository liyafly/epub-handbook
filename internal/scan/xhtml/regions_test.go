package xhtml

import (
	"strings"
	"testing"
)

// describeRegions 把区域切分渲染成可读的 `kind:字节` 序列。
func describeRegions(text string, regions []Region) []string {
	parts := make([]string, 0, len(regions))
	for _, r := range regions {
		kind := "tag"
		switch r.Kind {
		case RegionStyle:
			kind = "style"
		case RegionStylesheetPI:
			kind = "pi"
		}
		parts = append(parts, kind+":"+text[r.Start:r.End])
	}
	return parts
}

func TestScanRegions(t *testing.T) {
	cases := []struct {
		name, text string
		want       []string
		stop       int
	}{
		{
			"tags style script pi",
			`<?pi?><!--c--><a href="x">t</a><style>s</style><script>j</script><br/></b><style type="text/css"/>`,
			[]string{
				`tag:<a href="x">`, `tag:</a>`, `tag:<style>`, `style:s`, `tag:</style>`,
				`tag:<script>`, `tag:</script>`, `tag:<br/>`, `tag:</b>`, `tag:<style type="text/css"/>`,
			},
			ScanComplete,
		},
		{
			// 含 href 的普通 PI 不产出区域；xml-stylesheet PI 产出专用区域。
			"pi with href vs xml-stylesheet pi",
			`<?other href="a.css"?><?xml-stylesheet href="a.css"?><?xml-stylesheet-alt href="a.css"?>`,
			[]string{`pi:<?xml-stylesheet href="a.css"?>`},
			ScanComplete,
		},
		{
			// DOCTYPE 内部子集整段跳过：其中的注释、'>' 与撇号都不影响扫描。
			"doctype internal subset",
			`<!DOCTYPE html [ <!-- don't --> <!ENTITY x "a > b"> ]><br/>`,
			[]string{`tag:<br/>`},
			ScanComplete,
		},
		{
			// HTML 空注释也是注释。
			"html empty comment",
			`<!--><br/>`,
			[]string{`tag:<br/>`},
			ScanComplete,
		},
		{
			// 标签里引号不配对：从该标签起全部放弃，并上报偏移。
			"unmatched quote in tag",
			`<br/><p class="a>x`,
			[]string{`tag:<br/>`},
			5,
		},
		{
			// <script> 内容整段跳过，其中的 </style> 不结束任何东西。
			"script containing style close tag",
			`<script>var s = "</style>";</script><br/>`,
			[]string{`tag:<script>`, `tag:</script>`, `tag:<br/>`},
			ScanComplete,
		},
		{
			// 记录既有行为：<style> 是 raw text 元素，CSS 字符串 / 注释里的
			// </style> 同样结束元素（HTML 语义），其后按普通字符数据处理。
			"style close tag inside css string",
			`<style>a{content:"</style>"}</style><br/>`,
			[]string{`tag:<style>`, `style:a{content:"`, `tag:</style>`, `tag:</style>`, `tag:<br/>`},
			ScanComplete,
		},
		{
			"style close tag inside css comment",
			`<style>/* </style> */</style><br/>`,
			[]string{`tag:<style>`, `style:/* `, `tag:</style>`, `tag:</style>`, `tag:<br/>`},
			ScanComplete,
		},
		{
			// 未闭合结构：剩余字节不可改写，但之前的标签仍然产出。
			"unterminated comment",
			`<img src="a"/><!-- open <img src="b">`,
			[]string{`tag:<img src="a"/>`},
			14,
		},
		{
			"unclosed style element",
			`<style>url(a)`,
			[]string{`tag:<style>`},
			7,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, stop := ScanRegions(tc.text)
			parts := describeRegions(tc.text, got)
			if strings.Join(parts, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("区域划分错误:\n got %q\nwant %q", parts, tc.want)
			}
			if stop != tc.stop {
				t.Fatalf("截断偏移 = %d, want %d", stop, tc.stop)
			}
		})
	}
}
