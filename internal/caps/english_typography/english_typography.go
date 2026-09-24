// Package englishtypography fills missing XHTML language declarations without
// making typographic choices or changing EPUB metadata.
package englishtypography

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

const (
	CapabilityID = "epub.typography.english.optimize"
	cjkSkipRatio = 0.2
)

// Params selects the preferred language tag and optional exact spine scope.
type Params struct {
	Lang       string
	ScopePaths []string
}

type spineFile struct {
	path string
}

type plannedEdit struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Value  string `json:"value"`
}

type skippedFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Run fills missing html lang/xml:lang declarations in the selected spine
// XHTML files. It never changes styles or package metadata.
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	if b == nil {
		return report.Result{}, errors.New("english typography requires an EPUB book")
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	lang := p.Lang
	if lang == "" {
		lang = "en"
	}
	if !ValidLang(lang) {
		return report.Result{}, fmt.Errorf("invalid language tag %q", lang)
	}

	edits, planned, skipped, findings, filesScanned, err := scanPhase(ctx, b, lang, p.ScopePaths)
	if err != nil {
		return report.Result{}, err
	}
	failed := hasErrorFinding(findings)
	if failed {
		edits = nil
		planned = []plannedEdit{}
	}
	if err := b.Apply(edits); err != nil {
		return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
	}
	if planned == nil {
		planned = []plannedEdit{}
	}
	if skipped == nil {
		skipped = []skippedFile{}
	}
	facts := map[string]any{
		"plannedEdits": planned,
		"skipped":      skipped,
		"filesScanned": filesScanned,
		"editCount":    len(edits),
	}
	status := report.StatusComplete
	if failed {
		status = report.StatusFailed
	}
	return report.Result{Capability: CapabilityID, Status: status, Facts: facts, Findings: findings}, nil
}

func scanPhase(ctx context.Context, b *book.Book, lang string, scope []string) ([]editset.Edit, []plannedEdit, []skippedFile, []report.Finding, int, error) {
	container, err := b.CurrentContext(ctx, opf.ContainerPath)
	if err != nil {
		return nil, nil, nil, nil, 0, fmt.Errorf("read %s: %w", opf.ContainerPath, err)
	}
	opfPath, err := opf.FindOPFPath(container)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}
	opfData, err := b.CurrentContext(ctx, opfPath)
	if err != nil {
		return nil, nil, nil, nil, 0, fmt.Errorf("read %s: %w", opfPath, err)
	}
	pkg, err := opf.Parse(opfPath, opfData)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}
	spine := spineXHTML(pkg)
	byPath := make(map[string]spineFile, len(spine))
	for _, file := range spine {
		byPath[file.path] = file
	}
	selected, skipped := scopeFiles(spine, byPath, scope)
	findings := []report.Finding{}
	if scope != nil {
		for _, requested := range uniqueStrings(scope) {
			if _, ok := byPath[requested]; !ok {
				findings = append(findings, report.Finding{
					Level: "error", ID: "english.scope-not-in-spine",
					Title:    "Scope path is not a spine XHTML item",
					Detail:   fmt.Sprintf("scope_paths entry %q does not match a spine XHTML archive path", requested),
					Location: requested,
				})
			}
		}
	}
	if opfLanguageDiffers(pkg.Metadata["language"], lang) {
		findings = append(findings, report.Finding{
			Level: "info", ID: "english.opf-language-differs",
			Title:    "OPF language differs from the requested XHTML language",
			Detail:   fmt.Sprintf("dc:language=%q; requested primary language=%q", pkg.Metadata["language"], primaryLang(lang)),
			Location: opfPath,
		})
	}

	var edits []editset.Edit
	planned := []plannedEdit{}
	filesScanned := 0
	for _, file := range selected {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, nil, filesScanned, err
		}
		filesScanned++
		data, err := b.CurrentContext(ctx, file.path)
		if err != nil {
			return nil, nil, nil, nil, filesScanned, fmt.Errorf("read %s: %w", file.path, err)
		}
		if hasUTF8BOM(data) || hasNonUTF8Declaration(data) {
			findings = append(findings, report.Finding{
				Level: "error", ID: "english.unsupported-encoding",
				Title:    "XHTML encoding cannot be edited safely",
				Detail:   "BOM or XML declaration is not UTF-8; byte spans would not match the source",
				Location: file.path,
			})
			continue
		}
		root, err := opf.ScanXHTMLSpanTree(data)
		if err != nil {
			findings = append(findings, report.Finding{
				Level: "error", ID: "english.parse-failed",
				Title:    "XHTML cannot be parsed for language declarations",
				Detail:   err.Error(),
				Location: file.path,
			})
			continue
		}
		if root.Name.Local != "html" {
			findings = append(findings, report.Finding{
				Level: "error", ID: "english.parse-failed",
				Title:    "XHTML root is not html",
				Detail:   fmt.Sprintf("root element is %q", root.Name.Local),
				Location: file.path,
			})
			continue
		}
		if root.SelfClose {
			findings = append(findings, report.Finding{
				Level: "error", ID: "english.self-closing-html",
				Title:    "Self-closing html root cannot receive language attributes safely",
				Detail:   "the document root is a self-closing html element",
				Location: file.path,
			})
			continue
		}
		htmlLang, hasLang := root.AttrByLocal("", "lang")
		xmlLang, hasXMLLang := root.AttrByLocal(opf.XMLURI, "lang")
		if hasLang && hasXMLLang {
			if !strings.EqualFold(htmlLang, xmlLang) {
				findings = append(findings, report.Finding{
					Level: "warn", ID: "english.lang-mismatch",
					Title:    "html lang and xml:lang disagree",
					Detail:   fmt.Sprintf("lang=%q; xml:lang=%q; left unchanged", htmlLang, xmlLang),
					Location: file.path,
				})
				skipped = append(skipped, skippedFile{Path: file.path, Reason: "lang-mismatch"})
				continue
			}
			if primaryLang(htmlLang) != primaryLang(lang) {
				findings, skipped = appendLanguageConflict(findings, skipped, file.path, htmlLang, lang, scope != nil)
				continue
			}
			findings = append(findings, report.Finding{
				Level: "info", ID: "english.already-declared",
				Title:    "Language is already declared on html",
				Detail:   fmt.Sprintf("html lang and xml:lang both use primary language %q", primaryLang(lang)),
				Location: file.path,
			})
			skipped = append(skipped, skippedFile{Path: file.path, Reason: "already-declared"})
			continue
		}
		if hasLang || hasXMLLang {
			existing := htmlLang
			missingName := " xml:lang=\""
			if hasXMLLang {
				existing = xmlLang
				missingName = " lang=\""
			}
			if primaryLang(existing) != primaryLang(lang) {
				findings, skipped = appendLanguageConflict(findings, skipped, file.path, existing, lang, scope != nil)
				continue
			}
			insert := missingName + existing + `"`
			edits = append(edits, editset.Insert(file.path, int64(root.Open.End-1), []byte(insert)))
			planned = append(planned, plannedEdit{Path: file.path, Action: "mirror-lang", Value: existing})
			continue
		}
		body := bodyNode(root)
		if bodyHasLanguage(body) {
			findings = append(findings, report.Finding{
				Level: "info", ID: "english.declared-on-body",
				Title:    "Language is declared on body",
				Detail:   "html has no language declaration; body is left unchanged",
				Location: file.path,
			})
			skipped = append(skipped, skippedFile{Path: file.path, Reason: "declared-on-body"})
			continue
		}
		if scope == nil && body != nil && cjkLetterRatio(body.IterText()) >= cjkSkipRatio {
			findings = append(findings, report.Finding{
				Level: "info", ID: "english.skipped-cjk-text",
				Title:    "Undeclared-language page contains substantial CJK text",
				Detail:   fmt.Sprintf("CJK-to-letter ratio is at least %.1f; left unchanged", cjkSkipRatio),
				Location: file.path,
			})
			skipped = append(skipped, skippedFile{Path: file.path, Reason: "cjk-text"})
			continue
		}
		insert := ` lang="` + lang + `" xml:lang="` + lang + `"`
		edits = append(edits, editset.Insert(file.path, int64(root.Open.End-1), []byte(insert)))
		planned = append(planned, plannedEdit{Path: file.path, Action: "add-lang", Value: lang})
	}

	slices.SortFunc(findings, func(a, b report.Finding) int {
		if order := strings.Compare(a.ID, b.ID); order != 0 {
			return order
		}
		if location := strings.Compare(a.Location, b.Location); location != 0 {
			return location
		}
		return strings.Compare(a.Detail, b.Detail)
	})
	slices.SortFunc(skipped, func(a, b skippedFile) int {
		if order := strings.Compare(a.Path, b.Path); order != 0 {
			return order
		}
		return strings.Compare(a.Reason, b.Reason)
	})
	return edits, planned, skipped, findings, filesScanned, nil
}

func appendLanguageConflict(findings []report.Finding, skipped []skippedFile, path, existing, wanted string, scoped bool) ([]report.Finding, []skippedFile) {
	if scoped {
		findings = append(findings, report.Finding{
			Level: "error", ID: "english.lang-conflict",
			Title:    "Existing language declaration conflicts with the requested language",
			Detail:   fmt.Sprintf("existing=%q; requested=%q", existing, wanted),
			Location: path,
		})
		return findings, skipped
	}
	findings = append(findings, report.Finding{
		Level: "info", ID: "english.skipped-other-lang",
		Title:    "Page language differs from the requested language",
		Detail:   fmt.Sprintf("existing=%q; requested=%q; left unchanged", existing, wanted),
		Location: path,
	})
	skipped = append(skipped, skippedFile{Path: path, Reason: "other-language"})
	return findings, skipped
}

func opfLanguageDiffers(languages []string, wanted string) bool {
	for _, language := range languages {
		if strings.TrimSpace(language) != "" && primaryLang(language) != primaryLang(wanted) {
			return true
		}
	}
	return false
}

func primaryLang(value string) string {
	primary, _, _ := strings.Cut(strings.TrimSpace(value), "-")
	return strings.ToLower(primary)
}

func bodyNode(root *opf.SpanNode) *opf.SpanNode {
	for _, node := range root.Walk() {
		if node.Name.Local == "body" {
			return node
		}
	}
	return nil
}

func bodyHasLanguage(body *opf.SpanNode) bool {
	if body == nil {
		return false
	}
	_, hasLang := body.AttrByLocal("", "lang")
	_, hasXMLLang := body.AttrByLocal(opf.XMLURI, "lang")
	return hasLang || hasXMLLang
}

func cjkLetterRatio(text string) float64 {
	letters := 0
	cjk := 0
	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			cjk++
		}
	}
	if letters == 0 {
		return 0
	}
	return float64(cjk) / float64(letters)
}

func hasErrorFinding(findings []report.Finding) bool {
	return slices.ContainsFunc(findings, func(finding report.Finding) bool { return finding.Level == "error" })
}

func spineXHTML(pkg *opf.Package) []spineFile {
	byID := make(map[string]opf.ManifestItem, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		byID[item.ID] = item
	}
	seen := make(map[string]bool)
	var files []spineFile
	for _, ref := range pkg.Spine {
		item, ok := byID[ref.IDRef]
		if !ok || (item.MediaType != "application/xhtml+xml" && item.MediaType != "text/html") || item.ArchivePath == "" || seen[item.ArchivePath] {
			continue
		}
		seen[item.ArchivePath] = true
		files = append(files, spineFile{path: item.ArchivePath})
	}
	return files
}

func scopeFiles(spine []spineFile, byPath map[string]spineFile, scope []string) ([]spineFile, []skippedFile) {
	if scope == nil {
		return spine, []skippedFile{}
	}
	wanted := make(map[string]bool, len(scope))
	for _, path := range uniqueStrings(scope) {
		if _, ok := byPath[path]; ok {
			wanted[path] = true
		}
	}
	selected := make([]spineFile, 0, len(wanted))
	skipped := []skippedFile{}
	for _, file := range spine {
		if wanted[file.path] {
			selected = append(selected, file)
			continue
		}
		skipped = append(skipped, skippedFile{Path: file.path, Reason: "outside-scope"})
	}
	return selected, skipped
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func hasUTF8BOM(data []byte) bool {
	return bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

func hasNonUTF8Declaration(data []byte) bool {
	if !bytes.HasPrefix(data, []byte("<?xml")) {
		return false
	}
	end := bytes.Index(data, []byte("?>"))
	if end < 0 {
		return false
	}
	declaration := string(data[:end])
	lower := strings.ToLower(declaration)
	index := strings.Index(lower, "encoding")
	if index < 0 {
		return false
	}
	valueStart := index + len("encoding")
	for valueStart < len(declaration) && isXMLSpace(declaration[valueStart]) {
		valueStart++
	}
	if valueStart >= len(declaration) || declaration[valueStart] != '=' {
		return false
	}
	valueStart++
	for valueStart < len(declaration) && isXMLSpace(declaration[valueStart]) {
		valueStart++
	}
	if valueStart >= len(declaration) || (declaration[valueStart] != '\'' && declaration[valueStart] != '"') {
		return false
	}
	quote := declaration[valueStart]
	valueStart++
	valueEnd := strings.IndexByte(declaration[valueStart:], quote)
	if valueEnd < 0 {
		return false
	}
	encoding := strings.ToLower(strings.TrimSpace(declaration[valueStart : valueStart+valueEnd]))
	return encoding != "utf-8" && encoding != "utf8"
}

func isXMLSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}
