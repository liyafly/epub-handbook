// Package split 迁移 epub.package.split（scripts/epub_package_split_harness.py
// + scripts/epub_package/{core,split}.py 的 split_epub）：按显式 TOC 序号把
// 一本书拆成若干独立段 EPUB。
//
// 与 INV-3 的关系：split 虽然产生多个产物，但所有段先在独立的
// book.Book 中完成内存投影与验证，最后经 book.CommitGroup 以目录事务
// 一次性提交；失败时不会把任何中间 EPUB 暴露给调用者。未选中的 entry
// 仍由 zipfs 原样透传（INV-1）。
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
	// DryRun 只规划和验证所有分段，不创建 output_dir 或其 sibling 临时目录。
	// 最终 status 由 pipeline 按全局 dry-run 规则提升为 approval-required。
	DryRun bool
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

type segRange struct{ start, end int }

type segmentProjection struct {
	name      string
	book      *book.Book
	selected  []string
	resources map[string]bool
	valid     segmentValidation
}

// Run 执行 split。所有段先建立为内存中的 Book 投影并完成分区红线，
// 最终通过 book.CommitGroup 一次性提交目录；任何失败都不会创建输出
// 目录或留下某一段产物。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	if ctx == nil {
		return failedResult("split: nil context")
	}
	if b == nil {
		return failedResult("split: input book is nil")
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
	}
	if p.OutputDir == "" {
		return failedResult("split: output directory is required")
	}
	if err := book.ValidateOutputDirAbsent(p.OutputDir); err != nil {
		return failedResult(fmt.Sprintf("split: output directory %q is not available: %v", p.OutputDir, err))
	}

	inputPath := b.InputPath()
	names := b.OriginalNames()
	read := b.Original
	namesSet := make(map[string]bool, len(names))
	for _, n := range names {
		if err := ctx.Err(); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
		namesSet[n] = true
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
		for _, archivePath := range spinePaths {
			if err := ctx.Err(); err != nil {
				return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
			}
			targets = append(targets, tocEntry{title: pyBasename(archivePath), href: archivePath, level: 1})
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
		if err := ctx.Err(); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
		if point < 0 || point >= len(targets) {
			return failedResult(fmt.Sprintf("split point out of range: %d", point))
		}
	}

	targetSpineIndices := make([]int, len(targets))
	for i, t := range targets {
		if err := ctx.Err(); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
		pathPart := t.href
		if i2 := strings.IndexByte(pathPart, '#'); i2 >= 0 {
			pathPart = pathPart[:i2]
		}
		targetSpineIndices[i] = slices.Index(spinePaths, pathPart)
	}
	var ranges []segRange
	for index, point := range sortedPoints {
		if err := ctx.Err(); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
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
	if len(ranges) == 0 {
		return failedResult("split: split points produced no splittable segments")
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
		return failedResult(fmt.Sprintf("%s: OPF missing manifest", pkg.opfPath))
	}
	metaNode := opfTree.ChildByLocal(opfURI, "metadata")
	if metaNode == nil {
		return failedResult("OPF missing metadata")
	}

	stem, err := safeOutputStem(inputPath)
	if err != nil {
		return failedResult(err.Error())
	}
	pathIdentity := make(map[string]string, len(names))
	for _, n := range names {
		pathIdentity[n] = n
	}
	outputNames := make([]string, len(ranges))
	for i := range ranges {
		if err := ctx.Err(); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
		outputNames[i] = fmt.Sprintf("%s_%02d.epub", stem, i+1)
		if !safeOutputBasename(outputNames[i]) {
			return failedResult(fmt.Sprintf("split: unsafe output filename %q", outputNames[i]))
		}
	}

	rep := legacyReport{
		Operation: "split",
		Input:     strPtr(inputPath),
		Inputs:    []string{},
		OPF:       pkg.opfPath,
		Warnings:  []string{},
	}
	var events []report.Event
	segments := make([]segmentProjection, 0, len(ranges))
	defer func() {
		for _, segment := range segments {
			_ = segment.book.Close()
		}
	}()

	opfDir := pyDirname(pkg.opfPath)
	navPath := joinPath(opfDir, "nav.xhtml")
	ncxPath := joinPath(opfDir, "toc.ncx")
	for segmentIndex, r := range ranges {
		if err := ctx.Err(); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
		segmentNumber := segmentIndex + 1
		selected := append([]string(nil), spinePaths[r.start:r.end]...)
		selectedSet := make(map[string]bool, len(selected))
		for _, archivePath := range selected {
			selectedSet[archivePath] = true
		}
		resources, err := collectReferencedResourcesContext(ctx, namesSet, read, pkg, selectedSet)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return report.Result{}, fmt.Errorf("%s: segment %d: %w", CapabilityID, segmentNumber, ctxErr)
			}
			return failedResult(fmt.Sprintf("%s: segment %d resource closure failed: %v", CapabilityID, segmentNumber, err))
		}
		var segmentToc []tocEntry
		for _, entry := range targets {
			if err := ctx.Err(); err != nil {
				return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
			}
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
		for _, archivePath := range ordered {
			if err := ctx.Err(); err != nil {
				return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
			}
			item, ok := pkg.byPath[archivePath]
			if !ok || addedIDs[item.itemID] {
				continue
			}
			items = append(items, manifestTuple{
				itemID:    item.itemID,
				href:      relativeURI(pkg.opfPath, archivePath),
				mediaType: item.mediaType,
				props:     removeProp(item.properties, "nav"),
			})
			addedIDs[item.itemID] = true
		}
		navID := uniqueID("nav", addedIDs)
		ncxID := uniqueID("ncx", addedIDs)
		var refs []spineRef
		refs = append(refs, spineRef{idref: navID, linear: "no"})
		var expectedSpine []string
		for _, sp := range pkg.spine {
			if item, ok := pkg.byID(sp.idref); ok && selectedSet[item.archivePath] {
				refs = append(refs, spineRef{idref: sp.idref, linear: sp.linear, properties: sp.properties})
				expectedSpine = append(expectedSpine, sp.idref)
			}
		}
		if len(expectedSpine) != len(selected) {
			return failedResult(fmt.Sprintf("split: segment %d selected spine projection is incomplete", segmentNumber))
		}

		title := pkg.title
		newManifest := buildManifestElement(items, navID, relativeURI(pkg.opfPath, navPath), ncxID, relativeURI(pkg.opfPath, ncxPath))
		newSpine := buildSpineElement(ncxID, refs)
		navBytes := buildNav(title, []tocGroup{{title: title, entries: segmentToc}}, navPath, pathIdentity)
		ncxBytes := buildNcx(title, []tocGroup{{title: title, entries: segmentToc}}, ncxPath, pathIdentity)

		segBook, err := book.Open(inputPath)
		if err != nil {
			return failedResult(err.Error())
		}
		segments = append(segments, segmentProjection{
			name: outputNames[segmentIndex], book: segBook,
			selected: selected, resources: resources,
		})

		overwritten := make(map[string]bool, len(selected)+len(resources))
		for archivePath := range selectedSet {
			overwritten[archivePath] = true
		}
		for archivePath := range resources {
			overwritten[archivePath] = true
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
			if overwritten[name] && namesSet[name] {
				keepSet[name] = true
			}
		}
		for _, name := range segBook.OriginalNames() {
			if !keepSet[name] {
				deletes = append(deletes, editset.Delete(name))
			}
		}

		var rest []editset.Edit
		if !overwritten[pkg.opfPath] {
			body := rootBodyBytes(opfData, metaNode, newManifest, newSpine)
			rest = append(rest, editset.Replace(pkg.opfPath,
				int64(opfTree.Open.End), int64(opfTree.Close.Start-opfTree.Open.End), body))
			rest = append(rest, rootAttrInsertEdits(pkg.opfPath, opfTree)...)
		}
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
			if err := ctx.Err(); err != nil {
				return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
			}
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
				return failedResult(fmt.Sprintf("%s: segment %d apply deletes: %v", CapabilityID, segmentNumber, err))
			}
		}
		if err := segBook.Apply(rest); err != nil {
			return failedResult(fmt.Sprintf("%s: segment %d apply edits: %v", CapabilityID, segmentNumber, err))
		}
		valid, err := validateSegment(ctx, b, segBook, pkg, selected, expectedSpine, navID, navPath, ncxPath)
		if err != nil {
			return failedResult(fmt.Sprintf("%s: segment %d validation failed: %v", CapabilityID, segmentNumber, err))
		}
		segments[segmentIndex].valid = valid
		events = append(events, report.Event{
			Step: "split", Status: "completed",
			Message: fmt.Sprintf("segment=%d selected=%d resources=%d validated", segmentNumber, len(selected), len(resources)),
		})
	}

	outputs := make([]string, 0, len(segments))
	group := make([]book.GroupOutput, 0, len(segments))
	redlineFacts := make([]map[string]any, 0, len(segments))
	partitionFacts := make([]map[string]any, 0, len(segments))
	modifiedEntries := make([]string, 0)
	seenModified := map[string]bool{}
	for _, segment := range segments {
		if err := ctx.Err(); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
		outputPath := filepath.Join(p.OutputDir, segment.name)
		outputs = append(outputs, outputPath)
		group = append(group, book.GroupOutput{Name: segment.name, Book: segment.book})
		redlineFacts = append(redlineFacts, segment.valid.redline)
		partitionFacts = append(partitionFacts, segment.valid.partition)
		for _, name := range segment.book.ModifiedNames() {
			if !seenModified[name] {
				seenModified[name] = true
				modifiedEntries = append(modifiedEntries, name)
			}
		}
	}
	slices.Sort(modifiedEntries)
	rep.Outputs = append([]string(nil), outputs...)
	if !p.DryRun {
		if err := book.CommitGroup(ctx, p.OutputDir, group); err != nil {
			return failedResult(fmt.Sprintf("%s: commit group: %v", CapabilityID, err))
		}
		rep.SegmentsCreated = len(segments)
	}

	segmentPlans := make([]map[string]any, 0, len(segments))
	for i, segment := range segments {
		segmentPlans = append(segmentPlans, map[string]any{
			"output":          outputs[i],
			"selectedSpine":   append([]string(nil), segment.selected...),
			"modifiedEntries": segment.book.ModifiedNames(),
			"redline":         segment.valid.redline,
			"partition":       segment.valid.partition,
		})
	}
	facts := map[string]any{
		"opf":             rep.OPF,
		"outputDir":       p.OutputDir,
		"outputs":         append([]string(nil), outputs...),
		"plannedOutputs":  append([]string(nil), outputs...),
		"segmentsCreated": rep.SegmentsCreated,
		"plannedSegments": len(segments),
		"modifiedEntries": modifiedEntries,
		"segmentPlans":    segmentPlans,
		"redlineFacts":    redlineFacts,
		"partitionFacts":  partitionFacts,
		"dryRun":          p.DryRun,
	}
	res := report.Result{Capability: CapabilityID, Status: report.StatusComplete, Facts: facts, Events: events}
	if p.LegacyReport {
		raw, err := report.MarshalLegacy(rep)
		if err != nil {
			return report.Result{}, err
		}
		res.Facts["legacyReport"] = json.RawMessage(raw)
	}
	return res, nil
}

func safeOutputStem(inputPath string) (string, error) {
	base := filepath.Base(filepath.Clean(inputPath))
	if !safeOutputBasename(base) || strings.ContainsRune(base, '\\') {
		return "", fmt.Errorf("split: input filename is not a safe basename: %q", base)
	}
	stem, _ := pySplitExt(base)
	if !safeOutputBasename(stem) || stem == "" {
		return "", fmt.Errorf("split: input filename has no safe output stem: %q", base)
	}
	return stem, nil
}

func safeOutputBasename(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.ContainsRune(name, 0) && !strings.ContainsAny(name, `/\\`) &&
		filepath.Base(name) == name && filepath.VolumeName(name) == ""
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
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
