package migrateepub3

import (
	"strings"
	"testing"
)

// TestNormalizeDuokanNotesSkipsEscapedProse 锁定区域化改写：`class="duokan-…"`
// 只在真实标签字节内改名，正文里被转义写出的同一段文字必须原字节保留。
//
// 样本书《EPub指南》的 Chapter12-2 / Chapter8-6 正是「讲解多看注释写法」的
// 章节，改前实跑会产生 8 条 error redline.text —— 那不是红线误报，是这条
// 能力真的改了作者正文。
func TestNormalizeDuokanNotesSkipsEscapedProse(t *testing.T) {
	cases := []struct {
		name, in, want string
		wantCount      int
	}{
		{
			name:      "真实标签改名",
			in:        `<ol class="duokan-footnote-content"><li class="duokan-footnote-item" id="f1"><p>注文</p></li></ol>`,
			want:      `<ol class="footnote-list"><li class="footnote-item" id="f1"><p>注文</p></li></ol>`,
			wantCount: 2,
		},
		{
			name:      "转义正文不动",
			in:        `<p>替换：&lt;li class="duokan-footnote-item" id="footnote-\1"&gt;&lt;p&gt;\2&lt;/p&gt;&lt;/li&gt;</p>`,
			want:      `<p>替换：&lt;li class="duokan-footnote-item" id="footnote-\1"&gt;&lt;p&gt;\2&lt;/p&gt;&lt;/li&gt;</p>`,
			wantCount: 0,
		},
		{
			name:      "noteref 图标 class 在标签内仍改名",
			in:        `<p>正文<a class="duokan-footnote" href="#f1"><img alt="" src="../Images/note.png"/></a></p>`,
			want:      `<p>正文<a class="noteref-icon" href="#f1"><img alt="" src="../Images/note.png"/></a></p>`,
			wantCount: 1,
		},
		{
			name:      "同一文件里标签改名与转义正文并存",
			in:        `<a class="duokan-footnote" href="#f1">⊙</a><p>写作 &lt;a class="duokan-footnote"&gt; 即可。</p>`,
			want:      `<a class="noteref-icon" href="#f1">◎</a><p>写作 &lt;a class="duokan-footnote"&gt; 即可。</p>`,
			wantCount: 2,
		},
		{
			name:      "注释与 CDATA 内不改",
			in:        `<!-- <li class="duokan-footnote-item"> --><![CDATA[class="duokan-footnote"]]><br/>`,
			want:      `<!-- <li class="duokan-footnote-item"> --><![CDATA[class="duokan-footnote"]]><br/>`,
			wantCount: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, count, warnings := normalizeDuokanNotes(tc.in)
			if got != tc.want {
				t.Errorf("改写结果不符:\n got %s\nwant %s", got, tc.want)
			}
			if count != tc.wantCount {
				t.Errorf("计数 = %d, want %d", count, tc.wantCount)
			}
			if len(warnings) != 0 {
				t.Errorf("不应有告警: %q", warnings)
			}
		})
	}
}

// TestNormalizeDuokanNotesWarnsOnTruncatedScan 断言区域扫描截断时不静默半改：
// 截断点之后的标签保持原样，并给出带偏移的告警。
func TestNormalizeDuokanNotesWarnsOnTruncatedScan(t *testing.T) {
	in := `<li class="duokan-footnote-item">a</li><!-- 未闭合注释 <li class="duokan-footnote-item">`
	got, count, warnings := normalizeDuokanNotes(in)
	if count != 1 {
		t.Errorf("截断前的标签应改名一次, count = %d", count)
	}
	if !strings.Contains(got, `<!-- 未闭合注释 <li class="duokan-footnote-item">`) {
		t.Errorf("截断点之后不应被改写: %s", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "markup scan stopped at byte offset") {
		t.Fatalf("缺少截断告警: %q", warnings)
	}
}
