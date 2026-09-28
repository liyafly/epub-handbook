// Package englishtypography fills missing XHTML language declarations without
// making typographic choices or changing EPUB metadata.
package englishtypography

import (
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
	if scope == nil && opfLanguageDiffers(pkg.Metadata["language"], lang) {
		findings = append(findings, report.Finding{
			Level: "info", ID: "english.opf-language-differs-requires-scope",
			Title:    "OPF language differs from the requested XHTML language; explicit scope is required",
			Detail:   fmt.Sprintf("dc:language=%q; requested primary language=%q; provide scope_paths to select pages", pkg.Metadata["language"], primaryLang(lang)),
			Location: opfPath,
		})
		return nil, []plannedEdit{}, skipped, findings, 0, nil
	}
	if scope != nil {
		for _, requested := range opf.UniqueStrings(scope) {
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
		if err := opf.EditableUTF8(data); err != nil {
			findings = append(findings, report.Finding{
				Level: "error", ID: "english.unsupported-encoding",
				Title:    "XHTML encoding cannot be edited safely",
				Detail:   err.Error(),
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
			if !ValidLang(existing) {
				findings = append(findings, report.Finding{
					Level: "warn", ID: "english.invalid-existing-lang",
					Title:    "Existing language declaration cannot be mirrored safely",
					Detail:   fmt.Sprintf("existing language %q is invalid; left unchanged", existing),
					Location: file.path,
				})
				skipped = append(skipped, skippedFile{Path: file.path, Reason: "invalid-existing-lang"})
				continue
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
		if scope == nil && body != nil {
			cjkRatio, letters := cjkLetterRatio(body.IterText())
			if letters == 0 {
				findings = append(findings, report.Finding{
					Level: "info", ID: "english.skipped-no-text",
					Title:    "Undeclared-language page contains no letters",
					Detail:   "body has no Unicode letters; left unchanged",
					Location: file.path,
				})
				skipped = append(skipped, skippedFile{Path: file.path, Reason: "no-letter-text"})
				continue
			}
			if cjkRatio >= cjkSkipRatio {
				findings = append(findings, report.Finding{
					Level: "info", ID: "english.skipped-cjk-text",
					Title:    "Undeclared-language page contains substantial CJK text",
					Detail:   fmt.Sprintf("CJK-to-letter ratio is at least %.1f; left unchanged", cjkSkipRatio),
					Location: file.path,
				})
				skipped = append(skipped, skippedFile{Path: file.path, Reason: "cjk-text"})
				continue
			}
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

func cjkLetterRatio(text string) (float64, int) {
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
		return 0, 0
	}
	return float64(cjk) / float64(letters), letters
}

func hasErrorFinding(findings []report.Finding) bool {
	return slices.ContainsFunc(findings, func(finding report.Finding) bool { return finding.Level == "error" })
}

func spineXHTML(pkg *opf.Package) []spineFile {
	paths := opf.SpineXHTMLPaths(pkg)
	files := make([]spineFile, 0, len(paths))
	for _, path := range paths {
		files = append(files, spineFile{path: path})
	}
	return files
}

func scopeFiles(spine []spineFile, byPath map[string]spineFile, scope []string) ([]spineFile, []skippedFile) {
	paths := make([]string, 0, len(spine))
	for _, file := range spine {
		paths = append(paths, file.path)
	}
	selectedPaths, skippedPaths := opf.SelectScopePaths(paths, scope)
	selected := make([]spineFile, 0, len(selectedPaths))
	for _, path := range selectedPaths {
		selected = append(selected, byPath[path])
	}
	skipped := make([]skippedFile, 0, len(skippedPaths))
	for _, path := range skippedPaths {
		skipped = append(skipped, skippedFile{Path: path, Reason: "outside-scope"})
	}
	return selected, skipped
}
