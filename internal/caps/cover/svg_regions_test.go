package cover

import (
	"fmt"
	"strings"
	"testing"
)

// TestResizeSVGCoverPagesSkipsNonMarkup 锁定内联 SVG 封面缩放的区域感知：
// 只有真实标签里的 <svg>/<image> 参与缩放。裸 strings.Index 扫描要求字面
// 尖括号，所以转义正文（`&lt;svg&gt;`）本来就命中不了；真正的漏洞是注释、
// CDATA 与 <script> 里**未转义**的示例 SVG 片段——那是作者正文，改了就是
// 损坏正文，而 redline text 对注释内容不设块，抓不到。
func TestResizeSVGCoverPagesSkipsNonMarkup(t *testing.T) {
	const (
		doc  = "OEBPS/Text/cover.xhtml"
		newC = "OEBPS/Images/new-cover.png"
	)
	// 真实的内联 SVG 封面：viewBox 与 <image> 尺寸都要被改成 800x1200。
	real := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 200">` +
		`<image width="100" height="200" xlink:href="../Images/new-cover.png"/></svg>`

	cases := []struct {
		name string
		in   string
		want func(out string) error
	}{
		{
			name: "真实标签照常缩放",
			in:   `<html><body>` + real + `</body></html>`,
			want: func(out string) error {
				if !strings.Contains(out, `viewBox="0 0 800 1200"`) {
					return fmt.Errorf("viewBox 未被改写: %s", out)
				}
				if !strings.Contains(out, `width="800"`) || !strings.Contains(out, `height="1200"`) {
					return fmt.Errorf("<image> 尺寸未被改写: %s", out)
				}
				return nil
			},
		},
		{
			name: "注释里的示例 SVG 原字节保留",
			in:   `<html><body><!-- 旧写法：` + real + ` --></body></html>`,
			want: func(out string) error {
				if !strings.Contains(out, `viewBox="0 0 100 200"`) ||
					strings.Contains(out, `viewBox="0 0 800 1200"`) {
					return fmt.Errorf("注释内容被改写了: %s", out)
				}
				return nil
			},
		},
		{
			name: "CDATA 里的示例 SVG 原字节保留",
			in:   `<html><body><![CDATA[` + real + `]]></body></html>`,
			want: func(out string) error {
				if !strings.Contains(out, `viewBox="0 0 100 200"`) ||
					strings.Contains(out, `viewBox="0 0 800 1200"`) {
					return fmt.Errorf("CDATA 内容被改写了: %s", out)
				}
				return nil
			},
		},
		{
			name: "script 字符串字面量里的 SVG 原字节保留",
			in:   `<html><head><script>var t = '` + real + `';</script></head><body/></html>`,
			want: func(out string) error {
				if !strings.Contains(out, `viewBox="0 0 100 200"`) ||
					strings.Contains(out, `viewBox="0 0 800 1200"`) {
					return fmt.Errorf("script 内容被改写了: %s", out)
				}
				return nil
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := string(resizeSVGCoverPages([]byte(tc.in), doc, newC, 800, 1200, nil))
			if err := tc.want(out); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestResizeSVGCoverAfterNonLengthPreservingLowercase(t *testing.T) {
	const (
		doc       = "OEBPS/Text/cover.xhtml"
		coverPath = "OEBPS/Images/new-cover.png"
	)
	for _, prefix := range []string{"<p>K</p>", "<p>İİ</p>"} {
		input := prefix + `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><image xlink:href="../Images/new-cover.png"/></svg>`
		got := string(resizeSVGCoverPages([]byte(input), doc, coverPath, 100, 200, nil))
		if !strings.Contains(got, `viewBox="0 0 100 200"`) ||
			!strings.Contains(got, `width="100" height="200"`) {
			t.Errorf("SVG after prefix %q was not resized: %s", prefix, got)
		}
	}
}

// TestResizeSVGCoverPagesWarnsOnTruncatedScan 断言扫描截断不静默半改：
// 截断点之后的 SVG 保持原样，并给出带文件名与字节偏移的告警。
func TestResizeSVGCoverPagesWarnsOnTruncatedScan(t *testing.T) {
	in := `<html><body><!-- 未闭合注释 <svg viewBox="0 0 100 200">` +
		`<image width="100" height="200" xlink:href="../Images/new-cover.png"/></svg>`
	var warnings []string
	out := string(resizeSVGCoverPages([]byte(in), "OEBPS/Text/cover.xhtml",
		"OEBPS/Images/new-cover.png", 800, 1200, func(format string, a ...any) {
			warnings = append(warnings, fmt.Sprintf(format, a...))
		}))
	if out != in {
		t.Errorf("截断点之后不应被改写:\n got %s\nwant %s", out, in)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "markup scan stopped at byte offset") {
		t.Fatalf("缺少截断告警: %q", warnings)
	}
	if !strings.Contains(warnings[0], "OEBPS/Text/cover.xhtml") {
		t.Errorf("告警未带文件名: %q", warnings[0])
	}
}
