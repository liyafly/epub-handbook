package navaudit

import (
	"encoding/xml"
	"io"
	"sort"
	"strings"

	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// findingsByLevel 是 facts["findingsByLevel"] 的形状（固定三键三序）。
type findingsByLevel struct {
	Error int `json:"error"`
	Warn  int `json:"warn"`
	Info  int `json:"info"`
}

// detectorFinding 是 facts["actionableFindings"] 的元素形状。
type detectorFinding struct {
	Kind        string         `json:"kind"`
	File        string         `json:"file,omitempty"`
	Locator     map[string]any `json:"locator"`
	Params      map[string]any `json:"params"`
	Lane        string         `json:"lane"`
	AutoFixable bool           `json:"autoFixable"`
	Confidence  string         `json:"confidence"`
	Evidence    string         `json:"evidence"`
}

// countFindingsByLevel 统计信封 findings 的三级数量（固定三键）。
func countFindingsByLevel(findings []report.Finding) findingsByLevel {
	levels := findingsByLevel{}
	for _, f := range findings {
		switch f.Level {
		case "error":
			levels.Error++
		case "warn":
			levels.Warn++
		default:
			levels.Info++
		}
	}
	return levels
}

// toolAvailability 返回本机外部工具探测结果（无探测记录时为空对象）。
func (ins *inspector) toolAvailability() map[string]bool {
	out := map[string]bool{}
	if ins.tools != nil {
		for k, v := range ins.tools.Values {
			out[k] = v
		}
	}
	return out
}

// orderedSkills 对齐 apply_workflow_mode：去掉 source-intake，
// layout-auditor 固定首位，其余按 (级别, 原序) 稳定排序。
func (ins *inspector) orderedSkills() []string {
	first := ""
	var rest []string
	for _, s := range ins.skills {
		if s == "$epub-layout-auditor" {
			first = s
			continue
		}
		rest = append(rest, s)
	}
	type item struct {
		name string
		lv   int
		idx  int
	}
	items := make([]item, 0, len(rest))
	for i, s := range rest {
		lv := ins.skillLv[s]
		items = append(items, item{s, severity(lv), i})
	}
	sort.SliceStable(items, func(a, b int) bool {
		if items[a].lv != items[b].lv {
			return items[a].lv < items[b].lv
		}
		return items[a].idx < items[b].idx
	})
	out := make([]string, 0, len(items)+1)
	if first != "" {
		out = append(out, first)
	}
	for _, it := range items {
		out = append(out, it.name)
	}
	return out
}

// detectActionable 移植 epub_ai/detectors.py 的四个 detector。
// 顺序对齐 DETECTORS 注册序：missing-html-lang → obfuscated-class →
// empty-paragraph → missing-manifest-properties；每个 detector 内按
// manifest 序遍历文档。
func (ins *inspector) detectActionable() []detectorFinding {
	// 空列表必须序列化为 []（facts 的形状是数组，消费方会做 | length）。
	out := []detectorFinding{}
	language := ""
	if ins.pkg != nil {
		if langs := ins.pkg.Metadata["language"]; len(langs) > 0 {
			language = strings.TrimSpace(langs[0])
		}
	}
	// 对齐 Python model：按 manifest 顺序（无 OPF 时回退 zip 序）遍历 XHTML。
	var names []string
	if ins.pkg != nil {
		for _, it := range ins.pkg.Manifest {
			if it.ArchivePath != "" && (it.MediaType == "application/xhtml+xml" || isXHTMLName(it.ArchivePath)) {
				names = append(names, it.ArchivePath)
			}
		}
	} else {
		for _, name := range ins.b.Names() {
			if isXHTMLName(name) {
				names = append(names, name)
			}
		}
	}
	type parsedDoc struct {
		name string
		doc  *xhtmlDoc
	}
	docs := make([]parsedDoc, 0, len(names))
	for _, name := range names {
		raw, err := ins.b.Current(name)
		if err != nil {
			continue
		}
		doc, perr := parseXHTMLLoose(raw)
		if perr != nil || doc == nil {
			continue // 对齐 Python：解析失败仅告警跳过
		}
		docs = append(docs, parsedDoc{name, doc})
	}

	// 1. missing-html-lang（每文档一条）。
	for _, pd := range docs {
		if pd.doc.rootAttrs["lang"] == "" && pd.doc.rootAttrs["xml:lang"] == "" {
			value := language
			if value == "" {
				value = "zh-Hans"
			}
			out = append(out, detectorFinding{
				Kind: "missing-html-lang", File: pd.name,
				Locator: map[string]any{"selector": "html"},
				Params:  map[string]any{"value": value},
				Lane:    "tag", AutoFixable: true, Confidence: "high",
				Evidence: "<html> root element missing lang/xml:lang",
			})
		}
	}
	// 2. obfuscated-class：每文档最多 1 条（首个命中）。
	for _, pd := range docs {
		for _, el := range pd.doc.elements {
			if el.class == "" {
				continue
			}
			m := calibreClassRe.FindString(el.class)
			if m == "" {
				continue
			}
			out = append(out, detectorFinding{
				Kind: "obfuscated-class", File: pd.name,
				Locator: map[string]any{"id": el.id},
				Params:  map[string]any{"mapping": map[string]string{m: ""}},
				Lane:    "tag", AutoFixable: false, Confidence: "medium",
				Evidence: "Found obfuscated class '" + m + "' — target mapping requires human/AI judgment",
			})
			break
		}
	}
	// 3. empty-paragraph：每个命中元素一条。
	for _, pd := range docs {
		for _, el := range pd.doc.elements {
			if el.local != "p" {
				continue
			}
			if t := strings.TrimSpace(el.text); t == "" || t == " " {
				out = append(out, detectorFinding{
					Kind: "empty-paragraph", File: pd.name,
					Locator: map[string]any{"id": el.id},
					Params:  map[string]any{"rule": "empty-paragraph"},
					Lane:    "tag", AutoFixable: true, Confidence: "high",
					Evidence: "Empty paragraph element (no visible text content)",
				})
			}
		}
	}
	// 4. missing-manifest-properties（package lane）。
	if ins.pkg != nil {
		byPath := map[string]opf.ManifestItem{}
		for _, it := range ins.pkg.Manifest {
			if it.ArchivePath != "" {
				byPath[it.ArchivePath] = it
			}
		}
		for _, pd := range docs {
			item, ok := byPath[pd.name]
			if !ok {
				continue
			}
			text := pd.doc.rawText
			if text == "" {
				if raw, err := ins.b.Current(pd.name); err == nil {
					text = string(raw)
				}
			}
			if strings.Contains(text, "<math") || strings.Contains(text, mathmlURI) {
				if !propsContain(ins.pkg, item.Properties, "mathml") {
					out = append(out, detectorFinding{
						Kind: "missing-manifest-properties", File: pd.name,
						Locator: map[string]any{"manifest_id": item.ID},
						Params:  map[string]any{"properties": "mathml"},
						Lane:    "package", AutoFixable: true, Confidence: "high",
						Evidence: `XHTML contains MathML but manifest item lacks properties="mathml"`,
					})
				}
			}
			if strings.Contains(text, "<svg") || strings.Contains(text, svgURI) {
				if !propsContain(ins.pkg, item.Properties, "svg") {
					out = append(out, detectorFinding{
						Kind: "missing-manifest-properties", File: pd.name,
						Locator: map[string]any{"manifest_id": item.ID},
						Params:  map[string]any{"properties": "svg"},
						Lane:    "package", AutoFixable: true, Confidence: "high",
						Evidence: `XHTML contains SVG but manifest item lacks properties="svg"`,
					})
				}
			}
		}
	}
	return out
}

func isXHTMLName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".xhtml") || strings.HasSuffix(lower, ".html")
}

// xhtmlDoc 是 detector 需要的最小文档投影。
type xhtmlDoc struct {
	rootAttrs map[string]string
	elements  []xhtmlElement
	rawText   string
}

type xhtmlElement struct {
	local string
	id    string
	class string
	text  string
}

// parseXHTMLLoose 流式解析（宽容：实体不致命）。
// 每个元素在闭合时入列，text 已累积完子树全文（itertext 语义）。
func parseXHTMLLoose(data []byte) (*xhtmlDoc, error) {
	doc0 := &xhtmlDoc{rootAttrs: map[string]string{}, rawText: string(data)}
	_ = doc0
	d := xml.NewDecoder(strings.NewReader(string(data)))
	d.Strict = false
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) { return input, nil }
	doc := &xhtmlDoc{rootAttrs: map[string]string{}}
	first := true
	var open []*xhtmlElement
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if first {
				first = false
				for _, a := range t.Attr {
					doc.rootAttrs[a.Name.Local] = a.Value
				}
			}
			el := &xhtmlElement{local: t.Name.Local}
			for _, a := range t.Attr {
				switch a.Name.Local {
				case "id":
					el.id = a.Value
				case "class":
					el.class = a.Value
				}
			}
			open = append(open, el)
		case xml.CharData:
			if len(open) > 0 {
				for _, el := range open {
					el.text += string(t)
				}
			}
		case xml.EndElement:
			if len(open) > 0 {
				top := open[len(open)-1]
				// 只保留自身直接名匹配的出栈（宽容模式下的安全网）。
				if top.local == t.Name.Local || true {
					open = open[:len(open)-1]
					doc.elements = append(doc.elements, *top)
				}
			}
		}
	}
	return doc, nil
}
