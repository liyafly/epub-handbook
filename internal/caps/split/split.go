// Package split 迁移 epub.package.split（scripts/epub_package_split_harness.py
// + scripts/epub_package/{core,split}.py 的 split_epub）：按显式 TOC 序号把
// 一本书拆成若干独立段 EPUB。
//
// 与 INV-3 的关系（特例说明）：pipeline 的单输出模型一次只落盘一本，
// 而 split 的本质就是「每段一个产物」—— 因此本包在 Run 内部对每一段
// 用独立 book.Book（从输入重新 Open）构建并 WriteTo，多次落盘是该
// capability 的定义而非中间态外泄；除此之外仍然只有 Apply 一个写入口，
// 未选中的 entry 由 zipfs 原样透传（INV-1）。
//
// 字节策略：OPF 的 metadata 与根属性保留源字节（优于 Python 的整树
// 重序列化），manifest 与 spine 按 Python build_split_package 的规则重建
// 并以整元素区间替换写回；nav/ncx/container 按字符串模板逐字节生成；
// 被选中资源原样透传（Python 也是原样复制）。
package split

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// CapabilityID 是契约 id（contracts/capabilities/v1/epub.package.split.json）。
const CapabilityID = "epub.package.split"

// ErrPackageTool 对应 Python 的 PackageToolError（errors.Is 可判）。
var ErrPackageTool = errors.New("epub.package.split: conservative package operation failed")

type toolError struct{ msg string }

func (e *toolError) Error() string   { return e.msg }
func (e *toolError) Is(t error) bool { return t == ErrPackageTool }

func toolErrf(format string, a ...any) error {
	return &toolError{msg: fmt.Sprintf(format, a...)}
}

const canonicalMimetype = "application/epub+zip"

// Params 是 split 的参数。
type Params struct {
	// SplitPoints 是切分点（targets 的下标，排序去重后生效）。
	SplitPoints []int
	// OutputDir 是段产物目录（每段一个 <stem>_<NN>.epub）。
	OutputDir string
	// LegacyReport 输出 Python OperationReport 形状的 JSON。
	LegacyReport bool
}

// legacyReport 对齐 models.OperationReport（键序 = dataclass 字段序）。
type legacyReport struct {
	Operation        string   `json:"operation"`
	Input            *string  `json:"input"`
	Inputs           []string `json:"inputs"`
	Output           *string  `json:"output"`
	Outputs          []string `json:"outputs"`
	OPF              string   `json:"opf"`
	MergedItems      int      `json:"merged_items"`
	RenamedResources int      `json:"renamed_resources"`
	SegmentsCreated  int      `json:"segments_created"`
	FieldsUpdated    int      `json:"fields_updated"`
	CoverPath        string   `json:"cover_path"`
	Warnings         []string `json:"warnings"`
}

// manifestTuple 是重建 manifest 的条目。
type manifestTuple struct {
	itemID    string
	href      string
	mediaType string
	props     string
}

// tocGroup 是 build_nav / build_ncx 的 (group_title, entries) 分组。
type tocGroup struct {
	title   string
	entries []tocEntry
}

// failedResult 复刻 Python harness 的失败语义：不产出报告 JSON，
// Status=failed，错误措辞原样进入 findings。
func failedResult(msg string) (report.Result, error) {
	return report.Result{
		Capability: CapabilityID,
		Status:     report.StatusFailed,
		Findings:   []report.Finding{{Level: "error", ID: "package.refused", Title: msg}},
	}, nil
}

// Run 执行 split。多段落盘是上述包文档声明的 INV-3 特例。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	inputPath := b.InputPath()
	names := b.OriginalNames()
	read := b.Original
	namesSet := make(map[string]bool, len(names))
	for _, n := range names {
		namesSet[n] = true
	}

	// harness 的 ensure_empty_output_dir 守卫（措辞逐字对齐）。
	if entries, err := os.ReadDir(p.OutputDir); err == nil && len(entries) > 0 {
		return failedResult("refusing to write into non-empty output directory: " + p.OutputDir)
	}

	if err := ensureNoEncryption(names, "split"); err != nil {
		return failedResult(err.Error())
	}
	pkg, err := readPackage(namesSet, read)
	if err != nil {
		return failedResult(err.Error())
	}
	targets, err := parseToc(namesSet, read, pkg)
	if err != nil {
		return failedResult(err.Error())
	}
	spinePaths := contentSpinePaths(pkg)
	if len(targets) == 0 {
		for _, path := range spinePaths {
			targets = append(targets, tocEntry{title: pyBasename(path), href: path, level: 1})
		}
	}
	if len(targets) == 0 || len(spinePaths) == 0 {
		return failedResult("split: EPUB has no splittable spine content")
	}
	if len(p.SplitPoints) == 0 {
		return failedResult("split: at least one split point is required")
	}
	sortedPoints := append([]int(nil), p.SplitPoints...)
	slices.Sort(sortedPoints)
	sortedPoints = slices.Compact(sortedPoints)
	for _, point := range sortedPoints {
		if point < 0 || point >= len(targets) {
			return failedResult(fmt.Sprintf("split point out of range: %d", point))
		}
	}

	targetSpineIndices := make([]int, len(targets))
	for i, t := range targets {
		pathPart := t.href
		if i2 := strings.IndexByte(pathPart, '#'); i2 >= 0 {
			pathPart = pathPart[:i2]
		}
		idx := -1
		for j, sp := range spinePaths {
			if sp == pathPart {
				idx = j
				break
			}
		}
		targetSpineIndices[i] = idx
	}
	type segRange struct{ start, end int }
	var ranges []segRange
	for index, point := range sortedPoints {
		start := targetSpineIndices[point]
		if start < 0 {
			start = 0
			for i := point; i < len(targets); i++ {
				if targetSpineIndices[i] >= 0 {
					start = targetSpineIndices[i]
					break
				}
			}
		}
		var end int
		if index+1 < len(sortedPoints) {
			nextPoint := sortedPoints[index+1]
			end = targetSpineIndices[nextPoint]
			if end < 0 {
				end = len(spinePaths)
				for i := nextPoint; i < len(targets); i++ {
					if targetSpineIndices[i] >= 0 {
						end = targetSpineIndices[i]
						break
					}
				}
			}
		} else {
			end = len(spinePaths)
		}
		if start < end {
			ranges = append(ranges, segRange{start, end})
		}
	}

	// OPF 区间定位（只读）：根内容整段替换（metadata 原字节 + 重建的
	// manifest / spine），根属性保留源字节。
	opfData, err := read(pkg.opfPath)
	if err != nil {
		return failedResult(err.Error())
	}
	opfTree, err := opf.ScanSpanTree(opfData)
	if err != nil {
		return failedResult(fmt.Sprintf("%s: XML parse failed: %v", pkg.opfPath, err))
	}
	manifestNode := opfTree.ChildByAnyNS("manifest")
	spineNode := opfTree.ChildByAnyNS("spine")
	if manifestNode == nil || spineNode == nil {
		// readPackage 已保证两者存在；防御性检查。
		return failedResult(fmt.Sprintf("%s: OPF missing manifest", pkg.opfPath))
	}
	metaNode := opfTree.ChildByLocal(opfURI, "metadata")
	if metaNode == nil {
		// build_split_package 经 metadata_node 读取元数据，缺失即失败。
		return failedResult("OPF missing metadata")
	}

	rep := legacyReport{
		Operation: "split",
		Input:     strPtr(inputPath),
		Inputs:    []string{},
		OPF:       pkg.opfPath,
		Warnings:  []string{},
	}
	var events []report.Event
	var outputs []string

	stem, _ := pySplitExt(pyBasename(inputPath))
	pathIdentity := map[string]string{}
	for _, n := range names {
		pathIdentity[n] = n
	}

	for segmentIndex, r := range ranges {
		segmentNumber := segmentIndex + 1
		selected := append([]string(nil), spinePaths[r.start:r.end]...)
		selectedSet := map[string]bool{}
		for _, p := range selected {
			selectedSet[p] = true
		}
		resources := collectReferencedResources(namesSet, read, pkg, selectedSet)
		opfDir := pyDirname(pkg.opfPath)
		navPath := joinPath(opfDir, "nav.xhtml")
		ncxPath := joinPath(opfDir, "toc.ncx")
		var segmentToc []tocEntry
		for _, entry := range targets {
			pathPart := entry.href
			if i := strings.IndexByte(pathPart, '#'); i >= 0 {
				pathPart = pathPart[:i]
			}
			if entry.href == "" || selectedSet[pathPart] {
				segmentToc = append(segmentToc, entry)
			}
		}

		// 重建 manifest / spine（build_split_package 规则）。
		addedIDs := map[string]bool{}
		var items []manifestTuple
		ordered := append(append([]string(nil), selected...), sortedKeys(resources)...)
		for _, path := range ordered {
			item, ok := pkg.byPath[path]
			if !ok || addedIDs[item.itemID] {
				continue
			}
			items = append(items, manifestTuple{
				itemID:    item.itemID,
				href:      relativeURI(pkg.opfPath, path),
				mediaType: item.mediaType,
				props:     removeProp(item.properties, "nav"),
			})
			addedIDs[item.itemID] = true
		}
		navID := uniqueID("nav", addedIDs)
		ncxID := uniqueID("ncx", addedIDs)
		var refs []spineRef
		refs = append(refs, spineRef{idref: navID, linear: "no"})
		for _, sp := range pkg.spine {
			if item, ok := pkg.byID(sp.idref); ok && selectedSet[item.archivePath] {
				refs = append(refs, spineRef{idref: sp.idref, linear: sp.linear, properties: sp.properties})
			}
		}

		title := pkg.title
		newManifest := buildManifestElement(items, navID, relativeURI(pkg.opfPath, navPath), ncxID, relativeURI(pkg.opfPath, ncxPath))
		newSpine := buildSpineElement(ncxID, refs)
		navBytes := buildNav(title, []tocGroup{{title: title, entries: segmentToc}}, navPath, pathIdentity)
		ncxBytes := buildNcx(title, []tocGroup{{title: title, entries: segmentToc}}, ncxPath, pathIdentity)

		// 段 book：从输入重新打开（只读源），Apply 后 WriteTo 落盘一段。
		segBook, err := book.Open(inputPath)
		if err != nil {
			return failedResult(err.Error())
		}
		overwritten := map[string]bool{}
		for path := range selectedSet {
			overwritten[path] = true
		}
		for path := range resources {
			overwritten[path] = true
		}

		keepSet := map[string]bool{
			"mimetype":               true,
			"META-INF/container.xml": true,
			pkg.opfPath:              true,
			navPath:                  true,
			ncxPath:                  true,
		}
		var deletes []editset.Edit
		for _, name := range segBook.OriginalNames() {
			if _, kept := overwritten[name]; kept && namesSet[name] {
				keepSet[name] = true
			}
		}
		for _, name := range segBook.OriginalNames() {
			if !keepSet[name] {
				deletes = append(deletes, editset.Delete(name))
			}
		}

		var rest []editset.Edit
		// OPF：对齐 build_split_package —— 新建 package 根只含
		// [metadata(deepcopy), manifest, spine]，其余子元素（如 guide）与
		// 根内其余空白一并丢弃；metadata 以原字节保留（含其 tail），
		// 根属性缺失时按 Python 默认值补齐。
		// opfPath 同时是选中资源时保持原字节（对齐 Python dict 后写覆盖语义）。
		if !overwritten[pkg.opfPath] {
			body := rootBodyBytes(opfData, metaNode, newManifest, newSpine)
			rest = append(rest, editset.Replace(pkg.opfPath,
				int64(opfTree.Open.End), int64(opfTree.Close.Start-opfTree.Open.End), body))
			rest = append(rest, rootAttrInsertEdits(pkg.opfPath, opfTree)...)
		}
		// container / nav / ncx：未被选中资源覆盖时写入新生成字节。
		fixedWrites := []struct {
			path    string
			content []byte
		}{
			{"META-INF/container.xml", buildContainer(pkg.opfPath)},
		}
		if !overwritten[navPath] {
			fixedWrites = append(fixedWrites, struct {
				path    string
				content []byte
			}{navPath, navBytes})
		}
		if !overwritten[ncxPath] {
			fixedWrites = append(fixedWrites, struct {
				path    string
				content []byte
			}{ncxPath, ncxBytes})
		}
		for _, fw := range fixedWrites {
			cur, cerr := segBook.Current(fw.path)
			if cerr == nil {
				rest = append(rest, editset.Replace(fw.path, 0, int64(len(cur)), fw.content))
			} else {
				rest = append(rest, editset.Replace(fw.path, 0, 0, fw.content))
			}
		}
		if cur, cerr := segBook.Current("mimetype"); cerr == nil {
			if string(cur) != canonicalMimetype {
				rest = append(rest, editset.Replace("mimetype", 0, int64(len(cur)), []byte(canonicalMimetype)))
			}
		} else {
			rest = append(rest, editset.Replace("mimetype", 0, 0, []byte(canonicalMimetype)))
		}

		if len(deletes) > 0 {
			if err := segBook.Apply(deletes); err != nil {
				segBook.Close()
				return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
			}
		}
		if err := segBook.Apply(rest); err != nil {
			segBook.Close()
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
		outputPath := filepath.Join(p.OutputDir, fmt.Sprintf("%s_%02d.epub", stem, segmentNumber))
		// INV-3 特例：每段一个产物（见包文档）。
		if err := segBook.WriteTo(outputPath); err != nil {
			segBook.Close()
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
		segBook.Close()

		outputs = append(outputs, outputPath)
		rep.Outputs = append(rep.Outputs, outputPath)
		rep.SegmentsCreated++
		events = append(events, report.Event{
			Step: "split", Status: "completed",
			Message: fmt.Sprintf("segment=%d selected=%d resources=%d", segmentNumber, len(selected), len(resources)),
		})
	}
	rep.Outputs = outputs

	res := report.Result{
		Capability: CapabilityID,
		Status:     report.StatusComplete,
		Facts: map[string]any{
			"opf":             rep.OPF,
			"outputDir":       p.OutputDir,
			"outputs":         append([]string(nil), outputs...),
			"segmentsCreated": rep.SegmentsCreated,
		},
		Events: events,
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

// rootBodyBytes 组装根内容替换体：metadata 原字节（含 tail）+ 重建的
// manifest + spine，对应 Python 新建 package 的子节点序列。
func rootBodyBytes(data []byte, meta *opf.SpanNode, manifest, spine string) []byte {
	end := meta.End()
	if tail := meta.TailAfter(data); tail.End > end {
		end = tail.End
	}
	out := make([]byte, 0, int(end-meta.Open.Start)+len(manifest)+len(spine))
	out = append(out, data[meta.Open.Start:end]...)
	out = append(out, manifest...)
	out = append(out, spine...)
	return out
}

// rootAttrInsertEdits 对齐 build_split_package 的根属性默认值：
// version 缺省 "3.0"、unique-identifier 缺省 "book-id"。
// 同一插入点上先插入的编辑落在最前（editset 稳定序），
// 因此先插 version 再插 unique-identifier，最终属性顺序与
// Python 新建 dict 的 {version, unique-identifier} 一致。
func rootAttrInsertEdits(path string, root *opf.SpanNode) []editset.Edit {
	var out []editset.Edit
	insertPos := root.Open.End - 1
	if root.SelfClose {
		insertPos = root.Open.End - 2
	}
	if _, ok := root.AttrByLocal("", "version"); !ok {
		out = append(out, editset.Insert(path, int64(insertPos), []byte(` version="3.0"`)))
	}
	if _, ok := root.AttrByLocal("", "unique-identifier"); !ok {
		out = append(out, editset.Insert(path, int64(insertPos), []byte(` unique-identifier="book-id"`)))
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// joinPath 复刻 posixpath.join(a, b)（b 为相对段）。
func joinPath(a, b string) string {
	if a == "" {
		return b
	}
	return a + "/" + b
}

func strPtr(s string) *string { return &s }
