package split

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	cssscan "github.com/liyafly/epub-handbook/internal/scan/css"
)

// collectCSSURIsStrict is a test-only projection of the strict scanner contract.
func collectCSSURIsStrict(text string) ([]string, error) {
	references, err := cssscan.ScanReferences([]byte(text))
	if err != nil {
		return nil, fmt.Errorf("invalid CSS: %w", err)
	}
	var out []string
	for _, ref := range references {
		if strings.ContainsRune(ref.Value, '\\') {
			return nil, fmt.Errorf("invalid CSS: escaped URL at byte %d", ref.ValueSpan.Start)
		}
		if ref.Value != "" {
			out = append(out, ref.Value)
		}
	}
	return out, nil
}

func TestParseSrcsetCandidates(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantURL []string
		wantErr bool
	}{
		{
			name:    "density candidates",
			value:   "../Images/a.webp 1x, ../Images/b.webp 2x",
			wantURL: []string{"../Images/a.webp", "../Images/b.webp"},
		},
		{
			name:    "data URI comma belongs to URL",
			value:   "data:image/svg+xml,%3Csvg%3E 1x, ../Images/b.webp 2x",
			wantURL: []string{"data:image/svg+xml,%3Csvg%3E", "../Images/b.webp"},
		},
		{
			name:    "data URI multiple commas",
			value:   "data:text/plain,a,b,c 1x, ../Images/b.webp 2x",
			wantURL: []string{"data:text/plain,a,b,c", "../Images/b.webp"},
		},
		{
			name:    "malformed trailing comma",
			value:   "../Images/a.webp 1x,",
			wantErr: true,
		},
		{
			name:    "quoted candidate",
			value:   "\"../Images/a.webp\" 1x",
			wantErr: true,
		},
		{
			name:    "unknown descriptor",
			value:   "../Images/a.webp 1q",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSrcsetCandidates(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseSrcsetCandidates() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			urls := make([]string, len(got))
			for i, candidate := range got {
				urls[i] = candidate.url
			}
			if !reflect.DeepEqual(urls, tt.wantURL) {
				t.Fatalf("candidate URLs = %#v, want %#v", urls, tt.wantURL)
			}
		})
	}
}

func TestCollectMarkupURIsIncludesSrcsetCandidates(t *testing.T) {
	text := `<source srcset="data:image/svg+xml,%3Csvg%3E 1x, ../Images/a.webp 2x" src="../Images/fallback.webp"/>`
	got, err := collectMarkupURIsStrict([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"data:image/svg+xml,%3Csvg%3E", "../Images/a.webp", "../Images/fallback.webp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectMarkupURIsStrict() = %#v, want %#v", got, want)
	}
}

// TestCollectMarkupURIsIgnoresCharacterData 钉住资源闭合收集的区域感知。
//
// 回归来源是仓库自己的样书：它是一本讲 EPUB 的书，正文里的示例代码只转义
// 了尖括号，于是 `src="../Audio/XinJing.mp3"` 以**字符数据**形式出现在 <p>
// 里，而那个音频文件并不在书中。旧的裸文本扫描把它当成真引用，
// epub.package.split 于是对一本能正常打开的书报
// 「resource closure: referenced target missing from source」并硬拒。
//
// 负向控制：同一份文档里的真属性必须照常被收集到 —— 只忽略字符数据，
// 不是整份放弃。
func TestCollectMarkupURIsIgnoresCharacterData(t *testing.T) {
	doc := []byte(`<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
<style>.hero { background: url("../Images/real-style.webp"); }</style>
</head>
<body>
  <img src="../Images/real-attr.webp" alt="真引用"/>
  <p>替换：&lt;audio src="../Audio/ghost.mp3"/&gt;</p>
  <p>查找：&lt;img src="../Images/Picture(\d+)\.jpg"/&gt;</p>
  <!-- <img src="../Images/ghost-comment.webp"/> -->
  <script><![CDATA[ var s = '<img src="../Images/ghost-cdata.webp"/>'; ]]></script>
  <div style="background: url('../Images/real-inline.webp')">真内联样式</div>
</body>
</html>`)
	got, err := collectMarkupURIsStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"../Images/real-style.webp",
		"../Images/real-attr.webp",
		"../Images/real-inline.webp",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectMarkupURIsStrict() = %#v\nwant %#v", got, want)
	}
	for _, ghost := range []string{"../Audio/ghost.mp3", "../Images/ghost-comment.webp",
		"../Images/ghost-cdata.webp"} {
		for _, g := range got {
			if g == ghost {
				t.Errorf("字符数据/注释/CDATA 里的 %q 被当成了真引用", ghost)
			}
		}
	}
}

func TestCollectCSSURIsStrict(t *testing.T) {
	text := `/* url("ignored.png") */ .hero { background: url("../Images/hero.webp"); } @import "../Styles/more.css";`
	got, err := collectCSSURIsStrict(text)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"../Images/hero.webp", "../Styles/more.css"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectCSSURIsStrict() = %#v, want %#v", got, want)
	}
}

func TestCollectCSSURIsStrictImageSetAndEscapedFunction(t *testing.T) {
	got, err := collectCSSURIsStrict(`a{background:image-set("a.webp" 1x, "b.webp" 2x);content:"image-set(\"ghost.webp\")"}`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.webp", "b.webp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectCSSURIsStrict() = %#v, want %#v", got, want)
	}
	if _, err := collectCSSURIsStrict(`a{background:u\72l(image.webp)}`); err == nil {
		t.Fatal("escaped URL function was silently skipped")
	}
	if got, err := collectCSSURIsStrict(`a{background:url (image.webp)}`); err != nil || len(got) != 0 {
		t.Fatalf("whitespace-separated url() = %#v, %v; want no reference", got, err)
	}
}
