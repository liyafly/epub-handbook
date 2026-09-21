// Package merge 迁移 epub.package.merge（scripts/epub_package_merge_harness.py
// + scripts/epub_package/{core,merge}.py 的 merge_epubs）：
//
//   - 多卷输入：Params.Inputs[0] 是 pipeline 已打开为主 b 的第一卷，
//     其余卷在包内以只读 book.Open 打开；
//   - 第一卷资源保持原路径（未变化即透传，INV-1）；与固定产物名或
//     先前卷冲突的资源按 vol{N}_ 前缀改名，并在包内重写全部本地引用；
//   - container / OPF / nav / ncx 由本包按 Python ElementTree 的确切字节
//     规则重新生成（新建产物，非原文重写）；
//   - b.Apply 是唯一写入口，落盘由 pipeline 的 b.WriteTo 负责（INV-3）；
//   - encryption.xml 存在 → 拒绝（措辞与 Python 逐字对齐）。
//
// 报告以统一信封的 Result.Facts 表达（operation/opf/inputs/output/
// mergedItems/renamedResources/warnings）。
package merge

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// CapabilityID 是契约 id（contracts/capabilities/v1/epub.package.merge.json）。
const CapabilityID = "epub.package.merge"

// ErrPackageTool 对应 Python 的 PackageToolError（errors.Is 可判）。
var ErrPackageTool = errors.New("epub.package.merge: conservative package operation failed")

type toolError struct{ msg string }

func (e *toolError) Error() string   { return e.msg }
func (e *toolError) Is(t error) bool { return t == ErrPackageTool }

func toolErrf(format string, a ...any) error {
	return &toolError{msg: fmt.Sprintf(format, a...)}
}

const (
	fixedContainerPath = "META-INF/container.xml"
	fixedOPFPath       = "OEBPS/content.opf"
	fixedNavPath       = "OEBPS/nav.xhtml"
	fixedNCXPath       = "OEBPS/toc.ncx"
	canonicalMimetype  = "application/epub+zip"
)

// Params 是 merge 的参数。
type Params struct {
	// Inputs 是全部输入卷（按 Python harness 的 argv 顺序）。
	// Inputs[0] 必须就是 pipeline 已打开为 b 的那本书；其余卷只读打开。
	Inputs []string
	// Title 是可选的合并标题（nil = 取第一卷 dc:title）。
	Title *string
	// Output 是输出路径（只进入 facts.output；本包不落盘）。
	Output string
}

// operationReport 是本能力的包内统计累加器，最终展开为 Result.Facts。
type operationReport struct {
	Operation        string
	Inputs           []string
	OPF              string
	MergedItems      int
	RenamedResources int
	Warnings         []string
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

// spineTocEntries 复刻 core.spine_toc_entries。
//
// 留在本包（而不是 internal/scan/opf）：pkg 的类型 *pkgInfo 是本包私有的
// 包投影（见 pkgio.go），scan/opf 是层 4、不能反向 import caps（层 2）。
func spineTocEntries(pkg *pkgInfo) []opf.TocEntry {
	var entries []opf.TocEntry
	for _, sp := range pkg.spine {
		item, ok := pkg.byID(sp.idref)
		if !ok || pypath.HasNavProp(item.properties) {
			continue
		}
		lower := strings.ToLower(item.archivePath)
		if item.mediaType == "application/xhtml+xml" ||
			strings.HasSuffix(lower, ".xhtml") || strings.HasSuffix(lower, ".html") {
			entries = append(entries, opf.TocEntry{Title: pypath.Basename(item.href), Href: item.archivePath, Level: 1})
		}
	}
	return entries
}

// parseToc 复刻 core.parse_toc：nav → ncx → spine 回退。理由同
// spineTocEntries：pkg 是本包私有类型，纯 XML 解析部分已经在
// internal/scan/opf.ParseTocNav / ParseTocNcx。
func parseToc(names map[string]bool, read func(string) ([]byte, error), pkg *pkgInfo) ([]opf.TocEntry, error) {
	for _, item := range pkg.manifest {
		if !pypath.HasNavProp(item.properties) {
			continue
		}
		if !names[item.archivePath] {
			continue // parse_toc_nav 对缺失文件返回 []
		}
		data, err := read(item.archivePath)
		if err != nil {
			data = nil
		}
		entries, perr := opf.ParseTocNav(item.archivePath, data)
		if perr != nil {
			return nil, perr
		}
		if len(entries) > 0 {
			return entries, nil
		}
	}
	if pkg.tocID != "" {
		if item, ok := pkg.byID(pkg.tocID); ok {
			if names[item.archivePath] {
				data, err := read(item.archivePath)
				if err == nil {
					entries, perr := opf.ParseTocNcx(item.archivePath, data)
					if perr != nil {
						return nil, perr
					}
					if len(entries) > 0 {
						return entries, nil
					}
				}
			}
		}
	}
	return spineTocEntries(pkg), nil
}

// Run 执行 merge（SPEC §6.1 三段式：扫描 → 应用 → 报告）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	inputs := p.Inputs
	if len(inputs) == 0 {
		inputs = []string{b.InputPath()}
	}
	if len(inputs) < 2 {
		return failedResult("merge requires at least two input EPUB files")
	}

	rep := operationReport{
		Operation: "merge",
		Inputs:    append([]string(nil), inputs...),
		OPF:       fixedOPFPath,
		Warnings:  []string{},
	}
	// warnf 喂给 transformResource：区域扫描截断时上报文件名与字节偏移，
	// 走既有的 rep.Warnings → findings（level=warn）通道，不静默半改。
	warnf := func(format string, a ...any) {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf(format, a...))
	}

	usedPaths := map[string]bool{
		fixedContainerPath: true,
		fixedOPFPath:       true,
		fixedNavPath:       true,
		fixedNCXPath:       true,
	}
	usedIDs := map[string]bool{"nav": true, "ncx": true}
	expected := map[string]bool{
		"mimetype":         true,
		fixedContainerPath: true,
		fixedOPFPath:       true,
		fixedNavPath:       true,
		fixedNCXPath:       true,
	}

	var (
		creates        []editset.Edit
		replaces       []editset.Edit
		deletes        []editset.Edit
		inDeletes      = map[string]bool{}
		renames        = map[string]string{}
		sourceMappings []map[string]any
		mergedMeta     []manifestTuple
		mergedSp       []spineTuple
		groups         []opf.TocGroup
		firstMeta      *metaExtract
		mergedTitle    = p.Title // Python：--title 给定时永不回退到卷标题
	)

	for vi, inputPath := range inputs {
		if err := ctx.Err(); err != nil {
			return report.Result{}, err
		}
		var names []string
		var read func(string) ([]byte, error)
		if vi == 0 {
			names = b.OriginalNames()
			read = func(path string) ([]byte, error) { return b.OriginalContext(ctx, path) }
		} else {
			vb, err := book.OpenContext(ctx, inputPath)
			if err != nil {
				return failedResult(err.Error())
			}
			defer vb.Close()
			names = vb.OriginalNames()
			read = func(path string) ([]byte, error) { return vb.OriginalContext(ctx, path) }
		}
		namesSet := make(map[string]bool, len(names))
		for _, n := range names {
			namesSet[n] = true
		}

		if err := ensureNoEncryption(names, "merge"); err != nil {
			return failedResult(err.Error())
		}
		pkg, err := readPackage(namesSet, read)
		if err != nil {
			return failedResult(err.Error())
		}
		if firstMeta == nil {
			if pkg.meta == nil {
				return failedResult("OPF missing metadata")
			}
			firstMeta = pkg.meta
		}
		if mergedTitle == nil {
			t := pkg.title
			mergedTitle = &t
		}
		prefix := fmt.Sprintf("vol%d_", vi+1)
		pathMap := map[string]string{}
		idMap := map[string]string{}

		for _, item := range pkg.manifest {
			if err := ctx.Err(); err != nil {
				return report.Result{}, err
			}
			if !namesSet[item.archivePath] {
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: manifest href does not resolve: %s", inputPath, item.href))
				continue
			}
			if pypath.HasNavProp(item.properties) || item.mediaType == "application/x-dtbncx+xml" {
				continue
			}
			finalPath, renamedFlag := pypath.AllocateArchivePath(item.archivePath, usedPaths, prefix)
			pathMap[item.archivePath] = finalPath
			if renamedFlag {
				rep.RenamedResources++
				// Pipeline's before-state is only the first book. Later volumes
				// must not redirect a first-volume resource with the same path.
				if vi == 0 {
					renames[item.archivePath] = finalPath
				}
			}
			baseID := item.itemID
			if usedIDs[item.itemID] {
				baseID = fmt.Sprintf("vol%d_%s", vi+1, item.itemID)
			}
			newID := pypath.UniqueID(baseID, usedIDs)
			idMap[item.itemID] = newID
			props := pypath.RemoveProp(item.properties, "nav")
			mergedMeta = append(mergedMeta, manifestTuple{
				itemID:    newID,
				href:      pypath.RelativeURI(fixedOPFPath, finalPath),
				mediaType: item.mediaType,
				props:     props,
			})
			rep.MergedItems++
		}

		sourceMappings = append(sourceMappings, map[string]any{
			"inputIndex": vi, "input": inputPath, "mappings": mappingList(pathMap),
		})
		for _, item := range pkg.manifest {
			if err := ctx.Err(); err != nil {
				return report.Result{}, err
			}
			finalPath, ok := pathMap[item.archivePath]
			if !ok {
				continue
			}
			data, err := read(item.archivePath)
			if err != nil {
				return failedResult(err.Error())
			}
			transformed, transformErr := transformResource(data, item.archivePath, finalPath, pathMap, namesSet, warnf)
			if transformErr != nil {
				return failedResult(transformErr.Error())
			}
			expected[finalPath] = true
			if vi == 0 {
				switch {
				case finalPath != item.archivePath:
					creates = append(creates, editset.Replace(finalPath, 0, 0, transformed))
					deletes = append(deletes, editset.Delete(item.archivePath))
					inDeletes[item.archivePath] = true
				case !bytesEqual(transformed, data):
					replaces = append(replaces, editset.Replace(item.archivePath, 0, int64(len(data)), transformed))
				}
				continue
			}
			creates = append(creates, editset.Replace(finalPath, 0, 0, transformed))
		}

		for _, sp := range pkg.spine {
			src, ok := pkg.byID(sp.idref)
			if !ok || pypath.HasNavProp(src.properties) {
				continue
			}
			if newID, ok2 := idMap[src.itemID]; ok2 {
				mergedSp = append(mergedSp, spineTuple{idref: newID, linear: sp.linear, properties: sp.properties})
			}
		}

		entries := []opf.TocEntry{}
		toc, err := parseToc(namesSet, read, pkg)
		if err != nil {
			return failedResult(err.Error())
		}
		for _, entry := range toc {
			if entry.Href == "" {
				entries = append(entries, entry)
				continue
			}
			href := entry.Href
			fragment := ""
			sep := false
			if i := indexOfByte(href, '#'); i >= 0 {
				href, fragment, sep = href[:i], href[i+1:], true
			}
			if final, ok := pathMap[href]; ok {
				target := final
				if sep {
					target += "#" + fragment
				}
				entries = append(entries, opf.TocEntry{Title: entry.Title, Href: target, Level: entry.Level})
			}
		}
		if len(entries) == 0 {
			for _, sp := range pkg.spine {
				src, ok := pkg.byID(sp.idref)
				if !ok {
					continue
				}
				if final, ok2 := pathMap[src.archivePath]; ok2 {
					entries = append(entries, opf.TocEntry{Title: pypath.Basename(src.href), Href: final, Level: 1})
				}
			}
		}
		groups = append(groups, opf.TocGroup{Title: pkg.title, Entries: entries})
	}

	title := "Merged EPUB"
	if mergedTitle != nil && *mergedTitle != "" {
		title = *mergedTitle
	}
	containerBytes := buildContainer(fixedOPFPath)
	opfBytes := buildOPF(title, firstMeta, mergedMeta, mergedSp)
	identity := map[string]string{}
	for p := range expected {
		identity[p] = p
	}
	navBytes := []byte(opf.BuildNav(title, groups, fixedNavPath, identity))
	ncxBytes := []byte(opf.BuildNCX(title, groups, fixedNCXPath, identity))

	// 删除：主卷里不在最终产物名集合中的 entry（旧 OPF / nav / ncx /
	// 非签名内文件），对齐 Python 输出容器只含 manifest 资源的语义。
	for _, name := range b.OriginalNames() {
		if !expected[name] && !inDeletes[name] {
			deletes = append(deletes, editset.Delete(name))
			inDeletes[name] = true
		}
	}
	_ = inDeletes

	// 固定产物：删除后重建的路径用创建型编辑，其余整段替换。
	writeContent := map[string][]byte{
		fixedContainerPath: containerBytes,
		fixedOPFPath:       opfBytes,
		fixedNavPath:       navBytes,
		fixedNCXPath:       ncxBytes,
	}
	for _, path := range []string{fixedContainerPath, fixedOPFPath, fixedNavPath, fixedNCXPath} {
		content := writeContent[path]
		switch {
		case inDeletes[path]:
			creates = append(creates, editset.Replace(path, 0, 0, content))
		case b.Has(path):
			cur, err := b.CurrentContext(ctx, path)
			if err != nil {
				return failedResult(err.Error())
			}
			replaces = append(replaces, editset.Replace(path, 0, int64(len(cur)), content))
		default:
			creates = append(creates, editset.Replace(path, 0, 0, content))
		}
	}
	if cur, err := b.CurrentContext(ctx, "mimetype"); err == nil {
		if string(cur) != canonicalMimetype {
			replaces = append(replaces, editset.Replace("mimetype", 0, int64(len(cur)), []byte(canonicalMimetype)))
		}
	} else {
		creates = append(creates, editset.Replace("mimetype", 0, 0, []byte(canonicalMimetype)))
	}

	// 2. 应用（唯一写点）：先删后建，避免同名路径的删除与内容编辑冲突。
	if len(deletes) > 0 {
		if err := b.Apply(deletes); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
	}
	rest := make([]editset.Edit, 0, len(creates)+len(replaces))
	rest = append(rest, creates...)
	rest = append(rest, replaces...)
	if len(rest) > 0 {
		if err := b.Apply(rest); err != nil {
			return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
		}
	}

	// 3. 报告。
	var findings []report.Finding
	for _, w := range rep.Warnings {
		findings = append(findings, report.Finding{Level: "warn", ID: "merge.warning", Title: w})
	}
	res := report.Result{
		Capability: CapabilityID,
		Status:     report.StatusComplete,
		Facts: map[string]any{
			"operation":        rep.Operation,
			"opf":              rep.OPF,
			"inputs":           nonNilStrings(rep.Inputs),
			"output":           p.Output,
			"mergedItems":      rep.MergedItems,
			"renamedResources": rep.RenamedResources,
			// Flat mappings refer only to --input (the redline before-state).
			// Full provenance includes identities and is keyed by source input.
			"mappings":       mappingList(renames),
			"sourceMappings": sourceMappings,
			// nonNilStrings 而不是 append([]string(nil), …)：后者在源切片为空时
			// 返回 nil，JSON 里就是 null，而 SKILL.md 声明的是数组（`| length`
			// 会炸）。无告警的合并是最常见路径。
			"warnings": nonNilStrings(rep.Warnings),
		},
		Findings: findings,
		Events: []report.Event{{
			Step: "merge", Status: "completed",
			Message: fmt.Sprintf("volumes=%d merged_items=%d renamed_resources=%d", len(inputs), rep.MergedItems, rep.RenamedResources),
		}},
		Renames: nilIfEmpty(renames),
	}
	return res, nil
}

// nonNilStrings 复制字符串切片，空切片仍是空切片（不退化成 nil/null）。
func nonNilStrings(in []string) []string {
	out := make([]string, 0, len(in))
	return append(out, in...)
}

// mappingList 把 from→to 映射摊平成 {from,to} 数组，按 from 排序保证输出
// 稳定；空映射输出空数组而不是 null（SKILL.md 声明的是数组形状）。
func mappingList(renames map[string]string) []map[string]string {
	froms := make([]string, 0, len(renames))
	for from := range renames {
		froms = append(froms, from)
	}
	sort.Strings(froms)
	out := make([]map[string]string, 0, len(renames))
	for _, from := range froms {
		out = append(out, map[string]string{"from": from, "to": renames[from]})
	}
	return out
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func indexOfByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func nilIfEmpty(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	return m
}
