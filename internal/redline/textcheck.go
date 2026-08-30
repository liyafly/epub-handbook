package redline

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// normalizeText 复刻 normalize_text：\u00a0→空格 → Python \s+ 折叠 → strip → NFC。
func normalizeText(value string) string {
	if strings.ContainsRune(value, '\u00a0') {
		value = strings.ReplaceAll(value, "\u00a0", " ")
	}
	value = collapsePySpace(value)
	value = strings.Trim(value, " ")
	return norm.NFC.String(value)
}

// isPySpace 覆盖 Python 正则 \s 的全集：ASCII 空白 + \x1c-\x1f + White_Space。
func isPySpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

func collapsePySpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if isPySpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
			}
			prevSpace = true
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

// isNoteControl 复刻 is_note_control：a 元素带 type 属性（任意命名空间），
// 值分词含 noteref/backlink。
func isNoteControl(name string, attrs []xml.Attr) bool {
	if name != "a" {
		return false
	}
	for _, a := range attrs {
		if a.Name.Local == "type" {
			for _, token := range strings.Fields(a.Value) {
				if controlTextEpubTypes[token] {
					return true
				}
			}
		}
	}
	return false
}

// textFrame 是流式提取中的一个打开元素。
type textFrame struct {
	name       string
	collecting bool // 是否处于可收集文本的上下文
	isBlock    bool
	blockChild bool // 打开期间出现过块级后代
	buf        strings.Builder
}

// ExtractTextBlocks 复刻 extract_text_blocks：
// 文档序产出最内层块级元素的归一化文本（rt/rp/script/style 与 noteref/backlink
// 锚点内的文本被剔除，但其尾部文本保留）。
func ExtractTextBlocks(content []byte, label string) ([]string, error) {
	cleaned := sanitizeXML(content)
	d := xml.NewDecoder(strings.NewReader(cleaned))
	d.Strict = true
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}

	var stack []*textFrame
	var blocks []string
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: XML parse failed: %w", label, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			parentCollecting := len(stack) == 0 || stack[len(stack)-1].collecting
			collecting := parentCollecting && !ignoredTextTags[name] && !isNoteControl(name, t.Attr)
			fr := &textFrame{
				name:       name,
				collecting: collecting,
				isBlock:    blockTags[name],
			}
			if fr.isBlock && len(stack) > 0 {
				// 所有尚在打开的祖先都获得了块级后代。
				for _, anc := range stack {
					anc.blockChild = true
				}
			}
			stack = append(stack, fr)
		case xml.CharData:
			if len(stack) > 0 && stack[len(stack)-1].collecting {
				stack[len(stack)-1].buf.Write(t)
			}
		case xml.EndElement:
			if len(stack) == 0 {
				continue
			}
			fr := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if fr.isBlock && !fr.blockChild {
				if text := normalizeText(fr.buf.String()); text != "" {
					blocks = append(blocks, text)
				}
			}
		}
	}
	return blocks, nil
}

// BlockHashes 复刻 block_hashes：每块 SHA-256 十六进制。
func BlockHashes(blocks []string) []string {
	out := make([]string, len(blocks))
	for i, b := range blocks {
		out[i] = sha256Hex([]byte(b))
	}
	return out
}

// ExtractAnchorIDs 复刻 extract_anchor_ids：全部元素的 id 属性集合。
func ExtractAnchorIDs(content []byte, label string) (map[string]bool, error) {
	cleaned := sanitizeXML(content)
	d := xml.NewDecoder(strings.NewReader(cleaned))
	d.Strict = true
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}
	ids := map[string]bool{}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: XML parse failed: %w", label, err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			for _, a := range se.Attr {
				if a.Name.Local == "id" && a.Value != "" {
					ids[a.Value] = true
				}
			}
		}
	}
	return ids, nil
}
