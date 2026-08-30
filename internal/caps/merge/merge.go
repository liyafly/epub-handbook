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
// legacy-report 形状对齐 models.OperationReport 的 asdict()（键序 = dataclass
// 字段序），经 Params.LegacyReport 放进 Result.Facts["legacyReport"]。
package merge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
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
	// Output 是输出路径（仅进入 legacy 报告字段；本包不落盘）。
	Output string
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

// Run 执行 merge（SPEC §6.1 三段式：扫描 → 应用 → 报告）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	inputs := p.Inputs
	if len(inputs) == 0 {
		inputs = []string{b.InputPath()}
	}
	if len(inputs) < 2 {
		return failedResult("merge requires at least two input EPUB files")
	}

	rep := legacyReport{
		Operation: "merge",
		Inputs:    append([]string(nil), inputs...),
		Output:    strPtr(p.Output),
		Outputs:   []string{},
		OPF:       fixedOPFPath,
		Warnings:  []string{},
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
		creates     []editset.Edit
		replaces    []editset.Edit
		deletes     []editset.Edit
		inDeletes   = map[string]bool{}
		renames     = map[string]string{}
		mergedMeta  []manifestTuple
		mergedSp    []spineTuple
		groups      []tocGroup
		firstMeta   *metaExtract
		mergedTitle = p.Title // Python：--title 给定时永不回退到卷标题
	)

	for vi, inputPath := range inputs {
		var names []string
		var read func(string) ([]byte, error)
		if vi == 0 {
			names = b.OriginalNames()
			read = b.Original
		} else {
			vb, err := book.Open(inputPath)
			if err != nil {
				return failedResult(err.Error())
			}
			defer vb.Close()
			names = vb.OriginalNames()
			read = vb.Original
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
			if !namesSet[item.archivePath] {
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: manifest href does not resolve: %s", inputPath, item.href))
				continue
			}
			if hasNavProp(item.properties) || item.mediaType == "application/x-dtbncx+xml" {
				continue
			}
			finalPath, renamedFlag := allocateArchivePath(item.archivePath, usedPaths, prefix)
			pathMap[item.archivePath] = finalPath
			if renamedFlag {
				rep.RenamedResources++
				renames[item.archivePath] = finalPath
			}
			baseID := item.itemID
			if usedIDs[item.itemID] {
				baseID = fmt.Sprintf("vol%d_%s", vi+1, item.itemID)
			}
			newID := uniqueID(baseID, usedIDs)
			idMap[item.itemID] = newID
			props := removeProp(item.properties, "nav")
			mergedMeta = append(mergedMeta, manifestTuple{
				itemID:    newID,
				href:      relativeURI(fixedOPFPath, finalPath),
				mediaType: item.mediaType,
				props:     props,
			})
			rep.MergedItems++
		}

		for _, item := range pkg.manifest {
			finalPath, ok := pathMap[item.archivePath]
			if !ok {
				continue
			}
			data, err := read(item.archivePath)
			if err != nil {
				return failedResult(err.Error())
			}
			transformed := transformResource(data, item.archivePath, finalPath, pathMap, namesSet)
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
			if !ok || hasNavProp(src.properties) {
				continue
			}
			if newID, ok2 := idMap[src.itemID]; ok2 {
				mergedSp = append(mergedSp, spineTuple{idref: newID, linear: sp.linear, properties: sp.properties})
			}
		}

		entries := []tocEntry{}
		toc, err := parseToc(namesSet, read, pkg)
		if err != nil {
			return failedResult(err.Error())
		}
		for _, entry := range toc {
			if entry.href == "" {
				entries = append(entries, entry)
				continue
			}
			href := entry.href
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
				entries = append(entries, tocEntry{title: entry.title, href: target, level: entry.level})
			}
		}
		if len(entries) == 0 {
			for _, sp := range pkg.spine {
				src, ok := pkg.byID(sp.idref)
				if !ok {
					continue
				}
				if final, ok2 := pathMap[src.archivePath]; ok2 {
					entries = append(entries, tocEntry{title: pyBasename(src.href), href: final, level: 1})
				}
			}
		}
		groups = append(groups, tocGroup{title: pkg.title, entries: entries})
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
	navBytes := buildNav(title, groups, fixedNavPath, identity)
	ncxBytes := buildNcx(title, groups, fixedNCXPath, identity)

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
			cur, err := b.Current(path)
			if err != nil {
				return failedResult(err.Error())
			}
			replaces = append(replaces, editset.Replace(path, 0, int64(len(cur)), content))
		default:
			creates = append(creates, editset.Replace(path, 0, 0, content))
		}
	}
	if cur, err := b.Current("mimetype"); err == nil {
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
			"opf":              rep.OPF,
			"inputs":           append([]string(nil), rep.Inputs...),
			"output":           p.Output,
			"mergedItems":      rep.MergedItems,
			"renamedResources": rep.RenamedResources,
			"warnings":         append([]string(nil), rep.Warnings...),
		},
		Findings: findings,
		Events: []report.Event{{
			Step: "merge", Status: "completed",
			Message: fmt.Sprintf("volumes=%d merged_items=%d renamed_resources=%d", len(inputs), rep.MergedItems, rep.RenamedResources),
		}},
		Renames: nilIfEmpty(renames),
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

func strPtr(s string) *string { return &s }

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
