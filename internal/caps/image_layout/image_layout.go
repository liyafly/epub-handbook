// Package imagelayout 移植 epub.image.layout.optimize
// （scripts/epub_image_layout_advisor.py）。
//
// 它是只读 planner（契约 kind=planner、requiresWriteAccess=false）：
// 只扫描 manifest / spine / nav / CSS / XHTML 产出六类排版 finding，
// 不产生 edits、不调用 b.Apply。
//
// legacy-report 形状与 Python `json.dumps(report, ensure_ascii=False, indent=2)`
// 逐字节一致（version/epub/findings/warnings 与 finding 键序按 :246-254），
// 供 SPEC §5.2 的 P2 parity 使用。
package imagelayout

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// CapabilityID 是契约里的 capability id。
const CapabilityID = "epub.image.layout.optimize"

// Params 是本 capability 的参数。
type Params struct {
	// LegacyReport 把 Python oracle 的原始 JSON 形状放进 Facts["legacyReport"]。
	LegacyReport bool
}

// legacyFinding 对齐 Python finding() 的键序（:246-254）。
type legacyFinding struct {
	Scene      string      `json:"scene"`
	Finding    string      `json:"finding"`
	File       string      `json:"file"`
	Selector   string      `json:"selector"`
	Image      string      `json:"image"`
	Candidates []candidate `json:"candidates"`
}

// legacyReport 对齐 analyze_epub 返回 dict 的键序。
type legacyReport struct {
	Version  string          `json:"version"`
	EPUB     string          `json:"epub"`
	Findings []legacyFinding `json:"findings"`
	Warnings []string        `json:"warnings"`
}

// Run 执行本 capability。只读：扫描 → 报告（无 apply 段）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	rep, err := analyzeEpub(b)
	if err != nil {
		return report.Result{}, err
	}

	res := report.Result{
		Capability: CapabilityID,
		Status:     report.StatusComplete,
		Facts: map[string]any{
			"findings": len(rep.Findings),
			"warnings": len(rep.Warnings),
		},
	}
	for _, f := range rep.Findings {
		res.Findings = append(res.Findings, report.Finding{
			Level:    "warn",
			ID:       f.Finding,
			Title:    findingTitle(f.Finding),
			Detail:   f.File + " · " + f.Image,
			Location: f.Selector,
		})
	}
	if p.LegacyReport {
		raw, err := report.MarshalLegacy(rep)
		if err != nil {
			return report.Result{}, err
		}
		res.Facts["legacyReport"] = json.RawMessage(raw)
	}
	return res, nil
}

// findingTitle 给六类 finding 的新信封标题。
func findingTitle(kind string) string {
	switch kind {
	case "lone-image-no-figure":
		return "Image is not wrapped in a figure"
	case "caption-detached":
		return "Likely caption sits outside the figure"
	case "float-width-risk":
		return "Float or percentage width on image is reader-fragile"
	case "missing-alt":
		return "Image has no alt attribute"
	case "chapter-head-image-candidate":
		return "Chapter-head image slot candidate"
	case "fullpage-image-alite-candidate":
		return "Full-page image is an A-lite candidate"
	default:
		return kind
	}
}

// analyzeEpub 对齐 analyze_epub 主体。
func analyzeEpub(b *book.Book) (legacyReport, error) {
	container, err := b.Current(opf.ContainerPath)
	if err != nil {
		return legacyReport{}, fmt.Errorf("missing META-INF/container.xml")
	}
	opfPath, err := opf.FindOPFPath(container)
	if err != nil {
		return legacyReport{}, err
	}
	opfData, err := b.Current(opfPath)
	if err != nil {
		return legacyReport{}, err
	}
	pkg, err := opf.Parse(opfPath, opfData)
	if err != nil {
		return legacyReport{}, err
	}
	opfDir := dirName(pkg.Path)

	items := map[string]opf.ManifestItem{}
	for _, it := range pkg.Manifest {
		items[it.ID] = it
	}
	chapterPaths, coverPaths, err := navPaths(b, pkg)
	if err != nil {
		return legacyReport{}, err
	}
	cssRules := cssClassDeclarations(b)

	results := []legacyFinding{}
	warnings := []string{}

	for _, ref := range pkg.Spine {
		item, ok := items[ref.IDRef]
		if !ok || item.Href == "" {
			continue
		}
		if item.MediaType != "application/xhtml+xml" {
			continue
		}
		xhtmlPath := normJoin(opfDir, item.Href)
		if !b.Has(xhtmlPath) {
			warnings = append(warnings, "spine XHTML missing: "+xhtmlPath)
			continue
		}
		data, err := b.Current(xhtmlPath)
		if err != nil {
			warnings = append(warnings, "spine XHTML missing: "+xhtmlPath)
			continue
		}
		root, err := parseXMLTree(data)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: XML parse failed: %v", xhtmlPath, err))
			continue
		}
		body := findElement(root, "body")
		if body == nil {
			warnings = append(warnings, "body missing: "+xhtmlPath)
			continue
		}

		var images []*ixNode
		for _, elem := range body.subtree() {
			if elem.tag == "img" && !isNoterefIcon(elem) {
				images = append(images, elem)
			}
		}
		bodyChildren := body.children
		bodyClasses := splitPyFields(attrValue(body, "class"))
		fixedLayout := hasAnyProp(ref.Properties) || hasAnyProp(item.Properties)
		alitePage := false
		for _, cls := range bodyClasses {
			if aliteBodyClasses[cls] {
				alitePage = true
			}
		}
		coverPage := coverPaths[xhtmlPath]

		var firstChild *ixNode
		if len(bodyChildren) > 0 {
			firstChild = bodyChildren[0]
		}
		firstImages := map[*ixNode]bool{}
		if firstChild != nil && (firstChild.tag == "img" || firstChild.tag == "figure") {
			for _, elem := range firstChild.subtree() {
				if elem.tag == "img" {
					firstImages[elem] = true
				}
			}
		}
		fullpageCandidate := len(bodyChildren) == 1 && firstChild != nil &&
			(firstChild.tag == "img" || firstChild.tag == "figure") &&
			len(images) == 1 && runeLen(visibleText(body)) <= 20

		for _, imageElem := range images {
			parent := imageElem.parent
			imageSrc := attrValue(imageElem, "src")
			var imagePath string
			if imageSrc != "" {
				imagePath = normJoin(dirName(xhtmlPath), imageSrc)
			}
			imageSelector := selectorFor(imageElem, body)

			// lone-image-no-figure：图不在 figure 里（封面 / alite / 固定版式豁免）。
			if parent == nil || parent.tag != "figure" {
				if !(coverPage || alitePage || fixedLayout) {
					results = append(results, newFinding("lone-image-no-figure", xhtmlPath, imageSelector, imagePath))
				}
			}

			// caption-detached：图后紧跟短段落或图注类段落。
			if parent != nil {
				if index := imageElem.plainIdx - 1; index+1 < len(parent.children) {
					nextSibling := parent.children[index+1]
					if nextSibling.tag == "p" {
						siblingClasses := splitPyFields(attrValue(nextSibling, "class"))
						if runeLen(visibleText(nextSibling)) <= 30 || anyClass(siblingClasses, captionClasses) {
							results = append(results, newFinding("caption-detached", xhtmlPath, imageSelector, imagePath))
						}
					}
				}
			}

			// float-width-risk：img 浮动 / 直接百分比宽 / figure 浮动但宽度不当。
			imageStyle := elementStyle(imageElem, cssRules)
			imageFloat := imageStyle["float"] == "left" || imageStyle["float"] == "right"
			imageWidth := percentage(imageStyle["width"])
			var figureElem *ixNode
			if parent != nil && parent.tag == "figure" {
				figureElem = parent
			}
			figureStyle := map[string]string{}
			if figureElem != nil {
				figureStyle = elementStyle(figureElem, cssRules)
			}
			figureFloated := false
			if figureElem != nil {
				floated := figureStyle["float"] == "left" || figureStyle["float"] == "right"
				for _, cls := range splitPyFields(attrValue(figureElem, "class")) {
					if figureFloatClasses[cls] {
						floated = true
					}
				}
				figureFloated = floated
			}
			figureWidth := percentage(figureStyle["width"])
			directPercentage := imageWidth != nil && !(figureFloated && *imageWidth == 100.0)
			figureWidthRisk := figureFloated && (figureWidth == nil || *figureWidth < 25.0 || *figureWidth > 35.0)
			if imageFloat || directPercentage || figureWidthRisk {
				results = append(results, newFinding("float-width-risk", xhtmlPath, imageSelector, imagePath))
			}

			// missing-alt：Python 判据是键不存在（alt="" 视为已提供）。
			if !hasAttr(imageElem, "alt") {
				results = append(results, newFinding("missing-alt", xhtmlPath, imageSelector, imagePath))
			}

			if chapterPaths[xhtmlPath] && firstImages[imageElem] && !alitePage {
				results = append(results, newFinding("chapter-head-image-candidate", xhtmlPath, imageSelector, imagePath))
			}

			if fullpageCandidate {
				results = append(results, newFinding("fullpage-image-alite-candidate", xhtmlPath, imageSelector, imagePath))
			}
		}
	}

	return legacyReport{
		Version:  "1",
		EPUB:     b.InputPath(),
		Findings: results,
		Warnings: warnings,
	}, nil
}

// newFinding 对齐 finding()：scene 固定 image-layout，候选表深拷贝。
func newFinding(kind, file, selector, image string) legacyFinding {
	return legacyFinding{
		Scene:      "image-layout",
		Finding:    kind,
		File:       file,
		Selector:   selector,
		Image:      image,
		Candidates: append([]candidate(nil), candidates[kind]...),
	}
}

// findElement 以文档序找第一个指定 local 名的元素。
func findElement(root *ixNode, tag string) *ixNode {
	if root == nil {
		return nil
	}
	for _, n := range root.subtree() {
		if n.tag == tag {
			return n
		}
	}
	return nil
}

// hasAnyProp 对齐 properties 集合与 PREPAGINATED_PROPS 的交集判定。
func hasAnyProp(props string) bool {
	for _, tok := range strings.Fields(props) {
		if prepaginatedProps[tok] {
			return true
		}
	}
	return false
}

func anyClass(tokens []string, set map[string]bool) bool {
	for _, t := range tokens {
		if set[t] {
			return true
		}
	}
	return false
}

// hasAttr 判断元素是否存在指定无命名空间属性（"alt" 存在但可为空值）。
func hasAttr(n *ixNode, name string) bool {
	for _, a := range n.attrs {
		if a.Name.Space == "" && a.Name.Local == name {
			return true
		}
	}
	return false
}

// isNoterefIcon 对齐 is_noteref_icon：祖先带 noteref-icon 类，或祖先 <a>
// 的任意 type 属性分词含 noteref。
func isNoterefIcon(elem *ixNode) bool {
	for cur := elem.parent; cur != nil; cur = cur.parent {
		for _, cls := range splitPyFields(attrValue(cur, "class")) {
			if cls == "noteref-icon" {
				return true
			}
		}
		if cur.tag == "a" {
			for _, a := range cur.attrs {
				if a.Name.Local == "type" {
					for _, tok := range strings.Fields(a.Value) {
						if tok == "noteref" {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// selectorFor 对齐 selector_for：body > tag:nth-of-type(i) 链。
func selectorFor(elem, body *ixNode) string {
	var segments []string
	current := elem
	for current != body {
		parent := current.parent
		if parent == nil {
			break
		}
		segments = append(segments, fmt.Sprintf("%s:nth-of-type(%d)", current.tag, current.index))
		current = parent
	}
	out := "body"
	for i := len(segments) - 1; i >= 0; i-- {
		out += " > " + segments[i]
	}
	return out
}

// declarations 对齐 declarations()：name 小写，value strip→lower→去 !important→strip。
func declarations(value string) map[string]string {
	out := map[string]string{}
	for _, m := range styleDeclRe.FindAllStringSubmatch(value, -1) {
		v := strings.TrimSpace(m[2])
		v = strings.ToLower(v)
		v = strings.ReplaceAll(v, "!important", "")
		v = strings.TrimSpace(v)
		out[strings.ToLower(m[1])] = v
	}
	return out
}

// cssClassDeclarations 对齐 css_class_declarations：仅单类选择器规则。
// 文件迭代顺序 = 容器序（Python files dict 的插入序）。
func cssClassDeclarations(b *book.Book) map[[2]string]map[string]string {
	rules := map[[2]string]map[string]string{}
	for _, path := range b.OriginalNames() {
		if !strings.HasSuffix(strings.ToLower(path), ".css") {
			continue
		}
		data, err := b.Current(path)
		if err != nil {
			continue
		}
		css := cssCommentRe.ReplaceAllString(toPythonIgnoredUTF8(data), "")
		for _, m := range cssRuleRe.FindAllStringSubmatch(css, -1) {
			props := declarations(m[2])
			for _, sel := range strings.Split(m[1], ",") {
				sel = strings.TrimSpace(sel)
				if strings.ContainsAny(sel, " >+~") {
					continue
				}
				matches := cssClassRe.FindAllStringSubmatch(sel, -1)
				if len(matches) != 1 {
					continue
				}
				tag := strings.ToLower(strings.TrimSpace(strings.SplitN(sel, ".", 2)[0]))
				if tag == "" {
					tag = "*"
				}
				key := [2]string{tag, matches[0][1]}
				if rules[key] == nil {
					rules[key] = map[string]string{}
				}
				for k, v := range props {
					rules[key][k] = v
				}
			}
		}
	}
	return rules
}

// toPythonIgnoredUTF8 复刻 bytes.decode("utf-8", errors="ignore")：丢弃非法字节。
func toPythonIgnoredUTF8(data []byte) string {
	var b strings.Builder
	b.Grow(len(data))
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size <= 1 {
			i++
			continue
		}
		b.Write(data[i : i+size])
		i += size
	}
	return b.String()
}

// elementStyle 对齐 element_style：(*, class) 与 (tag, class) 规则 + style 属性。
// 已知差异：Python class_tokens 是 set（多类同属性时胜者不确定）；
// 这里按 class 出现顺序取值（后者覆盖前者），更确定。
func elementStyle(elem *ixNode, cssRules map[[2]string]map[string]string) map[string]string {
	tag := strings.ToLower(elem.tag)
	style := map[string]string{}
	for _, className := range splitPyFields(attrValue(elem, "class")) {
		for _, m := range []map[string]string{cssRules[[2]string{"*", className}], cssRules[[2]string{tag, className}]} {
			for k, v := range m {
				style[k] = v
			}
		}
	}
	for k, v := range declarations(attrValue(elem, "style")) {
		style[k] = v
	}
	return style
}

// percentage 对齐 percentage()：^\s*(\d+(?:\.\d+)?)% → float，否则 nil。
func percentage(value string) *float64 {
	if value == "" {
		return nil
	}
	m := percentRe.FindStringSubmatch(value)
	if m == nil {
		return nil
	}
	f, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return nil
	}
	return &f
}

// navPaths 对齐 nav_paths：nav 项 → toc 链接为章节，landmarks cover 链接为封面。
func navPaths(b *book.Book, pkg *opf.Package) (map[string]bool, map[string]bool, error) {
	chapters := map[string]bool{}
	covers := map[string]bool{}
	navItem, ok := pkg.NavItem()
	if !ok || navItem.Href == "" {
		return chapters, covers, nil
	}
	navPath := normJoin(dirName(pkg.Path), navItem.Href)
	if !b.Has(navPath) {
		return chapters, covers, nil
	}
	data, err := b.Current(navPath)
	if err != nil {
		return chapters, covers, nil
	}
	root, err := parseXMLTree(data)
	if err != nil {
		return nil, nil, err
	}
	navDir := dirName(navPath)
	epubTypeKey := "{" + opf.OPSURI + "}type"

	var walk func(n *ixNode)
	walk = func(n *ixNode) {
		if n.tag == "nav" {
			types := map[string]bool{}
			for _, tok := range strings.Fields(attrValue(n, epubTypeKey)) {
				types[tok] = true
			}
			for _, link := range n.subtree() {
				if link.tag != "a" {
					continue
				}
				href := attrValue(link, "href")
				if href == "" {
					continue
				}
				target := normJoin(navDir, href)
				if types["toc"] {
					chapters[target] = true
				}
				if types["landmarks"] {
					for _, tok := range strings.Fields(attrValue(link, epubTypeKey)) {
						if tok == "cover" {
							covers[target] = true
						}
					}
				}
			}
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	if root != nil {
		walk(root)
	}
	return chapters, covers, nil
}
