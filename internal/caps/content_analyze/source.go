// source.go 对齐 analyze_source（:464-476）的分派与 _plain_blocks /
// _markdown_blocks 实现。EPUB 输入不走本文件，只走 spine XHTML 路径。
package contentanalyze

import (
	"fmt"
	"strings"
)

// AnalyzeSource 对齐 analyze_source：按文件名后缀分派。
func AnalyzeSource(source, content string, includeSnippets bool) ([]legacyBlock, error) {
	var blocks []textBlock
	switch suffix := sourceSuffix(source); suffix {
	case ".xhtml", ".xml":
		var err error
		if blocks, err = extractXHTMLBlocks(source, content); err != nil {
			return nil, err
		}
	case ".html", ".htm":
		var err error
		if blocks, err = extractLooseHTMLBlocks(source, content); err != nil {
			return nil, err
		}
	case ".md", ".markdown":
		blocks = markdownBlocks(source, content)
	case ".txt", "":
		blocks = plainBlocks(source, content)
	default:
		return nil, fmt.Errorf("unsupported source type: %s", suffix)
	}
	out := make([]legacyBlock, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, publicize(b, includeSnippets))
	}
	return out, nil
}

// plainBlocks 对齐 _plain_blocks：空行分段。
func plainBlocks(source, content string) []textBlock {
	var blocks []textBlock
	index := 0
	for _, part := range plainSplitRe.Split(content, -1) {
		text := pyTrimSpace(part)
		if text == "" {
			continue
		}
		index++
		blocks = append(blocks, textBlock{
			source:       source,
			locator:      fmt.Sprintf("/text/p[%d]", index),
			tag:          "p",
			classes:      []string{},
			ancestorTags: []string{"text", "p"},
			epubTypes:    []string{},
			text:         text,
		})
	}
	return withNeighbors(blocks)
}

// markdownBlocks 对齐 _markdown_blocks：标题 / 引用 / 列表 / 围栏代码 / 段落。
func markdownBlocks(source, content string) []textBlock {
	var blocks []textBlock
	var paragraph []string
	inCode := false
	var codeLines []string

	add := func(tag, text string) {
		index := 1
		for i := range blocks {
			if blocks[i].tag == tag {
				index++
			}
		}
		blocks = append(blocks, textBlock{
			source:       source,
			locator:      fmt.Sprintf("/markdown/%s[%d]", tag, index),
			tag:          tag,
			classes:      []string{},
			ancestorTags: []string{"markdown", tag},
			epubTypes:    []string{},
			text:         pyTrimSpace(text),
		})
	}
	flush := func() {
		if len(paragraph) > 0 {
			add("p", strings.Join(paragraph, " "))
			paragraph = paragraph[:0]
		}
	}

	lines := pySplitLines(content)
	lines = append(lines, "")
	for _, line := range lines {
		if strings.HasPrefix(pyTrimSpace(line), "```") {
			flush()
			if inCode {
				add("code", strings.Join(codeLines, "\n"))
				codeLines = codeLines[:0]
			}
			inCode = !inCode
			continue
		}
		if inCode {
			codeLines = append(codeLines, line)
			continue
		}
		if m := mdHeadingRe.FindStringSubmatch(line); m != nil {
			flush()
			add(fmt.Sprintf("h%d", len(m[1])), m[2])
			continue
		}
		if strings.HasPrefix(line, ">") {
			flush()
			add("blockquote", strings.TrimLeft(line, "> "))
			continue
		}
		if mdListRe.MatchString(line) {
			flush()
			add("li", stripListMarker(line))
			continue
		}
		if pyTrimSpace(line) == "" {
			flush()
			continue
		}
		paragraph = append(paragraph, pyTrimSpace(line))
	}
	return withNeighbors(blocks)
}

// stripListMarker 对齐 re.sub(r"^\s*(?:[-+*]|\d+[.)])\s+", "", line)：
// 模式锚定行首，至多命中一次。
func stripListMarker(line string) string {
	if loc := mdListRe.FindStringIndex(line); loc != nil && loc[0] == 0 {
		return line[loc[1]:]
	}
	return line
}
