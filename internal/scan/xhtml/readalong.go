package xhtml

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// ---- 有声同步（Media Overlays）分段规划：只产出插入点，不构树 ----
//
// 规则（SPEC-实现约束 §5.14）：
//   - 句子只在 <p> 直属的字符数据里断开；行内元素整个落在某一句里。
//   - <h1>–<h6> 整体是一段，文字取自身文字，没有就取 <img alt>；缺 id 时补 id。
//   - <figure> 内有指向弹注的 noteref 时，整个 figure 是一段，匹配文字取该弹注
//     （去掉回链），SMIL 指向 figure；缺 id 时补 id。
//   - 脚注区（aside 的 epub:type 含 footnote / endnote / rearnote）、nav、figure、
//     script、style 里的段落不切。
//   - 新 id 一律用调用方给的前缀；文档里已有以该前缀开头的 id 视为冲突。

// ReadAlongSegment 是一个可高亮单元，按文档顺序排列。
type ReadAlongSegment struct {
	ID     string // SMIL text@src 的片段 id
	Text   string // 用于对齐的纯文字（已去标签、解实体）
	Figure bool   // true 表示文字来自 figure 链接的弹注
}

// Insertion 在 Offset 处插入 Text（文档绝对字节坐标）。同一偏移的多个插入按切片顺序拼接。
type Insertion struct {
	Offset int
	Text   string
}

// ReadAlongPlan 是一个 XHTML 的分段结果。
type ReadAlongPlan struct {
	Segments   []ReadAlongSegment
	Insertions []Insertion
}

// ErrReadAlongIDConflict 表示文档已有以前缀开头的 id。
type ErrReadAlongIDConflict struct{ IDs []string }

func (e *ErrReadAlongIDConflict) Error() string {
	return fmt.Sprintf("existing ids use the read-along prefix: %s", strings.Join(e.IDs, ", "))
}

var sentenceEnd = regexp.MustCompile(`[.!?]+[’”"')\]]*\s+`)

type openElem struct {
	name    string
	tag     Region
	skip    bool // 此元素及其内部不产出段落
	id      string
	noteRef string // figure：内部 noteref 的目标 id
}

// PlanReadAlong 规划一个 XHTML 的分段。idPrefix 例如 "mo-c3-"；paragraphs 为 true
// 时每个 <p> 整体一段，不再断句。
func PlanReadAlong(doc, idPrefix string, paragraphs bool) (ReadAlongPlan, error) {
	regions, trunc := ScanRegions(doc)
	if trunc != ScanComplete {
		return ReadAlongPlan{}, fmt.Errorf("markup cannot be scanned past byte %d", trunc)
	}
	var conflicts []string
	ids := map[string]int{} // id -> index of its start tag region
	for i, r := range regions {
		if r.Kind != RegionTag {
			continue
		}
		attrs, _ := TagAttrs(doc[r.Start:r.End])
		for _, a := range attrs {
			if strings.EqualFold(a.Raw, "id") {
				ids[a.Value] = i
				if strings.HasPrefix(a.Value, idPrefix) {
					conflicts = append(conflicts, a.Value)
				}
			}
		}
	}
	if len(conflicts) > 0 {
		return ReadAlongPlan{}, &ErrReadAlongIDConflict{IDs: conflicts}
	}

	var plan ReadAlongPlan
	n := 0
	newID := func(kind string) string {
		n++
		return fmt.Sprintf("%s%s%d", idPrefix, kind, n)
	}
	var stack []openElem
	inSkip := func() bool {
		for _, e := range stack {
			if e.skip {
				return true
			}
		}
		return false
	}
	for i := 0; i < len(regions); i++ {
		r := regions[i]
		if r.Kind != RegionTag {
			continue
		}
		tag := doc[r.Start:r.End]
		name, _, closing := TagParts(tag)
		name = strings.ToLower(localOf(name))
		if closing {
			for j := len(stack) - 1; j >= 0; j-- {
				if stack[j].name == name {
					if name == "figure" && stack[j].noteRef != "" && !insideSkipBelow(stack, j) {
						seg, err := figureSegment(doc, regions, ids, stack[j], newID, &plan)
						if err != nil {
							return ReadAlongPlan{}, err
						}
						if seg.Text != "" {
							plan.Segments = append(plan.Segments, seg)
						}
					}
					stack = stack[:j]
					break
				}
			}
			continue
		}
		if strings.HasSuffix(tag, "/>") {
			if name == "a" {
				markNoteRef(stack, tag)
			}
			continue
		}
		attrs, _ := TagAttrs(tag)
		e := openElem{name: name, tag: r}
		for _, a := range attrs {
			if strings.EqualFold(a.Raw, "id") {
				e.id = a.Value
			}
		}
		switch name {
		case "nav", "script", "style", "figure", "head":
			e.skip = true // figure 在闭合时按 noteref 整体成段，内部段落不切
		case "aside":
			if t := attrValue(attrs, "epub:type"); containsAny(t, "footnote", "footnotes", "endnote", "endnotes", "rearnote", "rearnotes") {
				e.skip = true
			}
		case "a":
			markNoteRef(stack, tag)
		}
		if name == "p" || isHeading(name) {
			end, ok := matchingClose(doc, regions, i, name)
			if !ok {
				return ReadAlongPlan{}, fmt.Errorf("<%s> at byte %d has no matching close tag", name, r.Start)
			}
			if !inSkip() {
				if isHeading(name) {
					text := plainText(doc[r.End:regions[end].Start])
					if strings.TrimSpace(text) == "" {
						text = firstImgAlt(doc, regions[i+1:end])
					}
					if strings.TrimSpace(text) != "" {
						id := e.id
						if id == "" {
							id = newID("h")
							plan.Insertions = append(plan.Insertions, Insertion{Offset: attrInsertOffset(doc, r), Text: ` id="` + id + `"`})
						}
						plan.Segments = append(plan.Segments, ReadAlongSegment{ID: id, Text: normSpace(text)})
					}
				} else {
					planParagraph(doc, regions, i, end, paragraphs, newID, &plan)
				}
			}
			i = end // 段内的标签不再进栈：p / 标题里没有需要识别的块
			continue
		}
		stack = append(stack, e)
	}
	return plan, nil
}

func insideSkipBelow(stack []openElem, j int) bool {
	for k := 0; k < j; k++ {
		if stack[k].skip {
			return true
		}
	}
	return false
}

func markNoteRef(stack []openElem, tag string) {
	attrs, _ := TagAttrs(tag)
	if !containsAny(attrValue(attrs, "epub:type"), "noteref") {
		return
	}
	href := attrValue(attrs, "href")
	target, ok := strings.CutPrefix(href, "#")
	if !ok || target == "" {
		return
	}
	for j := len(stack) - 1; j >= 0; j-- {
		if stack[j].name == "figure" {
			if stack[j].noteRef == "" {
				stack[j].noteRef = target
			}
			return
		}
	}
}

func figureSegment(doc string, regions []Region, ids map[string]int, fig openElem, newID func(string) string, plan *ReadAlongPlan) (ReadAlongSegment, error) {
	start, ok := ids[fig.noteRef]
	if !ok {
		return ReadAlongSegment{}, nil
	}
	name, _, _ := TagParts(doc[regions[start].Start:regions[start].End])
	end, ok := matchingClose(doc, regions, start, strings.ToLower(localOf(name)))
	if !ok {
		return ReadAlongSegment{}, fmt.Errorf("note #%s has no matching close tag", fig.noteRef)
	}
	text := noteText(doc, regions[start+1:end])
	if strings.TrimSpace(text) == "" {
		return ReadAlongSegment{}, nil
	}
	id := fig.id
	if id == "" {
		id = newID("f")
		plan.Insertions = append(plan.Insertions, Insertion{Offset: attrInsertOffset(doc, fig.tag), Text: ` id="` + id + `"`})
	}
	return ReadAlongSegment{ID: id, Text: normSpace(text), Figure: true}, nil
}

// noteText 收集弹注文字：去掉回链 <a>，<br> 当空格。
func noteText(doc string, inner []Region) string {
	var b strings.Builder
	skipDepth := 0
	for _, r := range inner {
		s := doc[r.Start:r.End]
		if r.Kind == RegionTag {
			name, _, closing := TagParts(s)
			name = strings.ToLower(localOf(name))
			switch {
			case name == "a" && !closing && !strings.HasSuffix(s, "/>"):
				attrs, _ := TagAttrs(s)
				if containsAny(attrValue(attrs, "epub:type"), "backlink") || containsAny(attrValue(attrs, "role"), "doc-backlink") {
					skipDepth++
				} else if skipDepth > 0 {
					skipDepth++
				}
			case name == "a" && closing && skipDepth > 0:
				skipDepth--
			case name == "br" || name == "p" || name == "div":
				b.WriteByte(' ')
			}
			continue
		}
		if r.Kind == RegionText && skipDepth == 0 {
			b.WriteString(html.UnescapeString(s))
		}
	}
	return b.String()
}

// planParagraph 在 regions[open] 与 regions[close] 之间按句插入 span。
func planParagraph(doc string, regions []Region, open, close int, whole bool, newID func(string) string, plan *ReadAlongPlan) {
	innerStart, innerEnd := regions[open].End, regions[close].Start
	cuts := []int{innerStart}
	if !whole {
		depth := 0
		for _, r := range regions[open+1 : close] {
			switch r.Kind {
			case RegionTag:
				s := doc[r.Start:r.End]
				_, _, closing := TagParts(s)
				switch {
				case closing:
					depth--
				case !strings.HasSuffix(s, "/>"):
					depth++
				}
			case RegionText:
				if depth != 0 {
					continue
				}
				for _, m := range sentenceEnd.FindAllStringIndex(doc[r.Start:r.End], -1) {
					if c := r.Start + m[1]; c < innerEnd {
						cuts = append(cuts, c)
					}
				}
			}
		}
	}
	cuts = append(cuts, innerEnd)
	for k := 0; k+1 < len(cuts); k++ {
		a, b := cuts[k], cuts[k+1]
		for a < b && isSpaceByte(doc[a]) {
			a++
		}
		for b > a && isSpaceByte(doc[b-1]) {
			b--
		}
		text := plainText(doc[a:b])
		if strings.TrimSpace(text) == "" {
			continue
		}
		id := newID("s")
		plan.Insertions = append(plan.Insertions,
			Insertion{Offset: a, Text: `<span id="` + id + `">`},
			Insertion{Offset: b, Text: `</span>`})
		plan.Segments = append(plan.Segments, ReadAlongSegment{ID: id, Text: normSpace(text)})
	}
}

// matchingClose 返回与 regions[open] 同名的配对闭合标签下标。
func matchingClose(doc string, regions []Region, open int, name string) (int, bool) {
	depth := 0
	for k := open; k < len(regions); k++ {
		r := regions[k]
		if r.Kind != RegionTag {
			continue
		}
		s := doc[r.Start:r.End]
		n, _, closing := TagParts(s)
		if strings.ToLower(localOf(n)) != name || strings.HasSuffix(s, "/>") {
			continue
		}
		if closing {
			depth--
			if depth == 0 {
				return k, true
			}
		} else {
			depth++
		}
	}
	return 0, false
}

// attrInsertOffset 是开标签里插入新属性的位置（`>` 或 `/>` 之前）。
func attrInsertOffset(doc string, r Region) int {
	end := r.End - 1
	if doc[end-1] == '/' {
		end--
	}
	return end
}

func firstImgAlt(doc string, inner []Region) string {
	for _, r := range inner {
		if r.Kind != RegionTag {
			continue
		}
		s := doc[r.Start:r.End]
		if n, _, _ := TagParts(s); strings.EqualFold(localOf(n), "img") {
			attrs, _ := TagAttrs(s)
			return html.UnescapeString(attrValue(attrs, "alt"))
		}
	}
	return ""
}

var tagRE = regexp.MustCompile(`<[^>]*>`)

func plainText(markup string) string {
	return html.UnescapeString(tagRE.ReplaceAllString(markup, ""))
}

func normSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

func isHeading(name string) bool {
	return len(name) == 2 && name[0] == 'h' && name[1] >= '1' && name[1] <= '6'
}


func attrValue(attrs []Attr, name string) string {
	for _, a := range attrs {
		if strings.EqualFold(a.Raw, name) {
			return html.UnescapeString(a.Value)
		}
	}
	return ""
}

func containsAny(tokens string, want ...string) bool {
	for _, t := range strings.Fields(tokens) {
		for _, w := range want {
			if strings.EqualFold(t, w) || strings.EqualFold(strings.TrimPrefix(t, "doc-"), w) {
				return true
			}
		}
	}
	return false
}
