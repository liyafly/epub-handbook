// Package contentanalyze 移植 epub.text.content.analyze
// （scripts/epub_content_analysis.py + scripts/epub_content_analyzer.py 门面）。
//
// 它是只读 detector（契约 kind=detector、requiresWriteAccess=false）：
// 不产生 edits、不调用 b.Apply，直接返回分析报告。
//
// legacy-report 形状与 Python `json.dumps(report, ensure_ascii=False, indent=2)`
// 逐字节一致（键序 = Python dict 插入序，浮点经 report.PyFloat 保持
// 1.0 → "1.0" 的 repr 语义），供 SPEC §5.2 的 P2 parity 使用。
package contentanalyze

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// CapabilityID 是契约里的 capability id。
const CapabilityID = "epub.text.content.analyze"

// encryptionPath 是 Python 侧硬编码拒绝的加密标记文件。
const encryptionPath = "META-INF/encryption.xml"

// Params 是本 capability 的参数。
type Params struct {
	// IncludeSnippets 在 legacy 报告块里附带头 160 码点本地文本预览
	// （Python --include-snippets；隐私默认关闭，报告不得直接入库）。
	IncludeSnippets bool
	// LegacyReport 把 Python oracle 的原始 JSON 形状放进 Facts["legacyReport"]。
	LegacyReport bool
	// SourceName / SourceContent 非空时直接分析该文本源（非 EPUB），
	// 按文件名后缀分派 xhtml / loose-html / markdown / plain。
	// 输入是 EPUB 时保持两者为零值，只走 spine XHTML 路径。
	SourceName    string
	SourceContent string
}

// legacyError 对齐 report["errors"] 元素键序：source, message。
type legacyError struct {
	Source  string `json:"source"`
	Message string `json:"message"`
}

// legacyFeatures 对齐 _features 的键序（:335-345）。
// 比率字段必须是 PyFloat：Python round(x,4) 的结果 repr 出来 0 → "0.0"。
type legacyFeatures struct {
	VisibleChars     int            `json:"visible_chars"`
	CJKCount         int            `json:"cjk_count"`
	LatinCount       int            `json:"latin_count"`
	DigitCount       int            `json:"digit_count"`
	PunctuationCount int            `json:"punctuation_count"`
	QuoteCount       int            `json:"quote_count"`
	LineCount        int            `json:"line_count"`
	CJKRatio         report.PyFloat `json:"cjk_ratio"`
	LatinRatio       report.PyFloat `json:"latin_ratio"`
}

// legacyBlock 对齐 _public_block 的键序（:438-457）。
// language / previous_tag / next_tag 可为 null（指针）；snippet 仅 opt-in。
type legacyBlock struct {
	Source         string         `json:"source"`
	Locator        string         `json:"locator"`
	Tag            string         `json:"tag"`
	Classes        []string       `json:"classes"`
	Language       *string        `json:"language"`
	PreviousTag    *string        `json:"previous_tag"`
	NextTag        *string        `json:"next_tag"`
	TextSHA256     string         `json:"text_sha256"`
	Features       legacyFeatures `json:"features"`
	PrimaryRole    string         `json:"primary_role"`
	CandidateRoles []string       `json:"candidate_roles"`
	Confidence     string         `json:"confidence"`
	ReviewRequired bool           `json:"review_required"`
	Evidence       []string       `json:"evidence"`
	Typography     typographyRow  `json:"typography"`
	Snippet        string         `json:"snippet,omitempty"`
}

// legacySummary 对齐 report["summary"] 键序；roles 是 Counter → sorted dict，
// Go map 的键序序列化即为字典序，与 Python sorted 一致。
type legacySummary struct {
	Blocks         int            `json:"blocks"`
	ReviewRequired int            `json:"review_required"`
	FileErrors     int            `json:"file_errors"`
	Roles          map[string]int `json:"roles"`
}

// legacyReport 对齐 _report 的顶层键序（:489-502）。
type legacyReport struct {
	SchemaVersion string        `json:"schema_version"`
	Capability    string        `json:"capability"`
	Input         string        `json:"input"`
	Status        string        `json:"status"`
	Summary       legacySummary `json:"summary"`
	Errors        []legacyError `json:"errors"`
	Blocks        []legacyBlock `json:"blocks"`
}

// Run 执行本 capability。只读：扫描 → 报告（无 apply 段）。
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	if p.SourceName != "" {
		return runSource(p)
	}
	return runEpub(b, p)
}

// runSource 走 analyze_source 分派（markdown / plain / loose-html / xhtml）。
func runSource(p Params) (report.Result, error) {
	blocks, err := AnalyzeSource(p.SourceName, p.SourceContent, p.IncludeSnippets)
	if err != nil {
		return report.Result{}, err
	}
	rep := buildReport(p.SourceName, blocks, nil)
	return assemble(rep, p), nil
}

// runEpub 走 EPUB spine XHTML 路径（含 encryption.xml 拒绝）。
func runEpub(b *book.Book, p Params) (report.Result, error) {
	docs, fileErrs, err := spineDocuments(b)
	if err != nil {
		return report.Result{}, err
	}
	var blocks []legacyBlock
	for _, doc := range docs {
		pbs, err := AnalyzeXHTML(doc.name, doc.content, p.IncludeSnippets)
		if err != nil {
			fileErrs = append(fileErrs, legacyError{Source: doc.name, Message: err.Error()})
			continue
		}
		blocks = append(blocks, pbs...)
	}
	rep := buildReport(resolvedInputPath(b), blocks, fileErrs)
	return assemble(rep, p), nil
}

// assemble 把 legacy 报告映射进统一信封：
// fail → StatusFailed；warn → complete + warn findings；pass → complete。
func assemble(rep legacyReport, p Params) report.Result {
	res := report.Result{
		Capability: CapabilityID,
		Status:     report.StatusComplete,
		Facts: map[string]any{
			"blocks":          rep.Summary.Blocks,
			"review_required": rep.Summary.ReviewRequired,
			"roles":           rep.Summary.Roles,
		},
	}
	switch rep.Status {
	case "fail":
		res.Status = report.StatusFailed
		res.Findings = append(res.Findings, report.Finding{
			Level:    "error",
			ID:       "content.analysis-failed",
			Title:    "No analyzable content in input",
			Detail:   strings.Join(errorMessages(rep.Errors), "; "),
			Location: rep.Input,
		})
	case "warn":
		for _, fe := range rep.Errors {
			res.Findings = append(res.Findings, report.Finding{
				Level:    "warn",
				ID:       "content.source-error",
				Title:    "Source document could not be analyzed",
				Detail:   fe.Message,
				Location: fe.Source,
			})
		}
		if rep.Summary.ReviewRequired > 0 {
			res.Findings = append(res.Findings, report.Finding{
				Level:  "warn",
				ID:     "content.review-required",
				Title:  "Some blocks need manual role review",
				Detail: fmt.Sprintf("%d of %d blocks are structurally ambiguous", rep.Summary.ReviewRequired, rep.Summary.Blocks),
			})
		}
	}
	if p.LegacyReport {
		raw, err := report.MarshalLegacy(rep)
		if err != nil {
			res.Status = report.StatusFailed
			return res
		}
		res.Facts["legacyReport"] = json.RawMessage(raw)
	}
	return res
}

func errorMessages(errs []legacyError) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Message)
	}
	return out
}

// buildReport 对齐 Python _report：状态判定与 summary 统计。
func buildReport(input string, blocks []legacyBlock, fileErrs []legacyError) legacyReport {
	roles := map[string]int{}
	review := 0
	for _, bl := range blocks {
		roles[bl.PrimaryRole]++
		if bl.ReviewRequired {
			review++
		}
	}
	status := "pass"
	switch {
	case len(fileErrs) > 0 && len(blocks) == 0:
		status = "fail"
	case review > 0 || len(fileErrs) > 0:
		status = "warn"
	}
	if blocks == nil {
		blocks = []legacyBlock{}
	}
	if fileErrs == nil {
		fileErrs = []legacyError{}
	}
	return legacyReport{
		SchemaVersion: "1.0",
		Capability:    CapabilityID,
		Input:         input,
		Status:        status,
		Summary: legacySummary{
			Blocks:         len(blocks),
			ReviewRequired: review,
			FileErrors:     len(fileErrs),
			Roles:          roles,
		},
		Errors: fileErrs,
		Blocks: blocks,
	}
}

// resolvedInputPath 复刻 Python analyze_path 的 path.resolve()：
// 绝对化 + 解析符号链接（无法解析时退回绝对路径）。
func resolvedInputPath(b *book.Book) string {
	p := b.InputPath()
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return p
}

// spineDoc 是 spine 里的一个可分析 XHTML 文档。
type spineDoc struct {
	name    string
	content string
}

// spineDocuments 对齐 _epub_spine_documents：encryption 拒绝 → container/OPF
// → manifest id → 归一化路径 → 严格 UTF-8 解码（失败记 error 继续）。
func spineDocuments(b *book.Book) ([]spineDoc, []legacyError, error) {
	if b.Has(encryptionPath) {
		return nil, nil, fmt.Errorf("encryption marker detected; content analysis stopped")
	}
	container, err := b.Current(opf.ContainerPath)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read EPUB package: %w", err)
	}
	opfPath, err := opf.FindOPFPath(container)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read EPUB package: %w", err)
	}
	opfData, err := b.Current(opfPath)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read EPUB package: %w", err)
	}
	pkg, err := opf.Parse(opfPath, opfData)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read EPUB package: %w", err)
	}
	opfDir := dirName(opfPath)
	byID := map[string]string{}
	for _, item := range pkg.Manifest {
		if item.ID != "" && item.Href != "" {
			byID[item.ID] = normJoin(opfDir, item.Href)
		}
	}
	var docs []spineDoc
	var errs []legacyError
	for _, ref := range pkg.Spine {
		target, ok := byID[ref.IDRef]
		if !ok || !b.Has(target) {
			continue
		}
		data, err := b.Current(target)
		if err != nil {
			continue
		}
		if !utf8.Valid(data) {
			errs = append(errs, legacyError{Source: target, Message: "text is not valid UTF-8"})
			continue
		}
		docs = append(docs, spineDoc{name: target, content: string(data)})
	}
	if len(docs) == 0 && len(errs) == 0 {
		return nil, nil, fmt.Errorf("EPUB spine contains no readable XHTML documents")
	}
	return docs, errs, nil
}
