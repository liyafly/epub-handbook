// Package verticalruby applies narrowly scoped, lossless fixes for Ruby
// fallback text and CSS writing-mode prefixes.
package verticalruby

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
	"github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

const CapabilityID = "epub.vertical.ruby.optimize"

const (
	OpRubyRP            = "ruby-rp"
	OpWritingModePrefix = "writing-mode-prefix"
)

// Params selects one mechanical fix and an optional exact resource scope.
type Params struct {
	Op         string
	ScopePaths []string
	RPOpen     string
	RPClose    string
}

type plannedEdit struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Target any    `json:"target"`
}

type skippedEdit struct {
	Path   string `json:"path"`
	Target any    `json:"target"`
	Reason string `json:"reason"`
}

type sourceFile struct {
	path string
}

// Run performs exactly one of the two supported source-span transformations.
// All resources are scanned before edits are applied, and any error finding
// cancels the complete edit set.
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	if b == nil {
		return report.Result{}, errors.New("vertical Ruby optimizer requires an EPUB book")
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	if p.Op != OpRubyRP && p.Op != OpWritingModePrefix {
		return report.Result{}, fmt.Errorf("unsupported vertical Ruby operation %q", p.Op)
	}
	if p.RPOpen == "" {
		p.RPOpen = "（"
	}
	if p.RPClose == "" {
		p.RPClose = "）"
	}
	if !ValidRPToken(p.RPOpen) || !ValidRPToken(p.RPClose) {
		return report.Result{}, errors.New("rp_open and rp_close must each be one safe non-space character")
	}

	var edits []editset.Edit
	var planned []plannedEdit
	var skipped []skippedEdit
	var findings []report.Finding
	filesScanned := 0
	var err error
	switch p.Op {
	case OpRubyRP:
		edits, planned, skipped, findings, filesScanned, err = scanRuby(ctx, b, p)
	case OpWritingModePrefix:
		edits, planned, skipped, findings, filesScanned, err = scanWritingMode(ctx, b, p)
	}
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
		skipped = []skippedEdit{}
	}
	status := report.StatusComplete
	if failed {
		status = report.StatusFailed
	}
	return report.Result{
		Capability: CapabilityID,
		Status:     status,
		Facts: map[string]any{
			"op": p.Op, "plannedEdits": planned, "skipped": skipped,
			"filesScanned": filesScanned, "editCount": len(edits),
		},
		Findings: findings,
	}, nil
}

func scanRuby(ctx context.Context, b *book.Book, p Params) ([]editset.Edit, []plannedEdit, []skippedEdit, []report.Finding, int, error) {
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
	byPath := make(map[string]sourceFile, len(spine))
	for _, file := range spine {
		byPath[file.path] = file
	}
	selected, skipped := selectScope(spine, byPath, p.ScopePaths)
	findings := []report.Finding{}
	if p.ScopePaths != nil {
		for _, requested := range uniqueStrings(p.ScopePaths) {
			if _, ok := byPath[requested]; !ok {
				findings = append(findings, report.Finding{
					Level: "error", ID: "vertical.scope-not-in-spine",
					Title:    "Scope path is not a spine XHTML item",
					Detail:   fmt.Sprintf("scope_paths entry %q does not match a spine XHTML archive path", requested),
					Location: requested,
				})
			}
		}
	}

	edits := []editset.Edit{}
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
		if hasBOM(data) || hasNonUTF8Declaration(data) {
			findings = append(findings, report.Finding{
				Level: "error", ID: "vertical.unsupported-encoding",
				Title:    "XHTML encoding cannot be edited safely",
				Detail:   "BOM or XML declaration is not UTF-8; byte spans would not match the source",
				Location: file.path,
			})
			continue
		}
		root, err := opf.ScanXHTMLSpanTree(data)
		if err != nil {
			findings = append(findings, report.Finding{
				Level: "error", ID: "vertical.xhtml-parse-failed",
				Title:  "XHTML cannot be parsed for Ruby fallback",
				Detail: err.Error(), Location: file.path,
			})
			continue
		}
		rubyNodes := make([]*opf.SpanNode, 0)
		for _, node := range root.Walk() {
			if node.Name.Local == "ruby" {
				rubyNodes = append(rubyNodes, node)
			}
		}
		blockedNested := map[*opf.SpanNode]bool{}
		for rubyIndex, ruby := range rubyNodes {
			if blockedNested[ruby] {
				continue
			}
			nested := descendant(ruby, "ruby")
			if nested != nil || descendant(ruby, "rtc") != nil {
				findings = append(findings, rubyFinding("warn", "vertical.ruby-complex", "Complex Ruby structure was skipped", "rtc and nested ruby structures need manual review", file.path))
				for _, child := range ruby.Walk() {
					if child != ruby && child.Name.Local == "ruby" {
						blockedNested[child] = true
					}
				}
				skipped = append(skipped, skippedEdit{Path: file.path, Target: rubyIndex + 1, Reason: "complex-ruby"})
				continue
			}
			if rawTagName(data, ruby) != "ruby" {
				findings = append(findings, rubyFinding("warn", "vertical.ruby-prefixed", "Prefixed Ruby element was skipped", "the source tag name is namespace-prefixed", file.path))
				skipped = append(skipped, skippedEdit{Path: file.path, Target: rubyIndex + 1, Reason: "prefixed-ruby"})
				continue
			}
			rts := []*opf.SpanNode{}
			rtIndexes := []int{}
			hasRP := false
			for i, child := range ruby.Kids {
				if child.Name.Local == "rp" {
					hasRP = true
				}
				if child.Name.Local == "rt" {
					rts = append(rts, child)
					rtIndexes = append(rtIndexes, i)
				}
			}
			if len(rts) == 0 {
				skipped = append(skipped, skippedEdit{Path: file.path, Target: rubyIndex + 1, Reason: "no-direct-rt"})
				continue
			}
			badRT := false
			for _, rt := range rts {
				if rawTagName(data, rt) != "rt" {
					findings = append(findings, rubyFinding("warn", "vertical.ruby-prefixed", "Prefixed Ruby text element was skipped", "the source rt tag name is namespace-prefixed", file.path))
					skipped = append(skipped, skippedEdit{Path: file.path, Target: rubyIndex + 1, Reason: "prefixed-rt"})
					badRT = true
					break
				}
				if rt.SelfClose || rt.Close.IsZero() {
					findings = append(findings, rubyFinding("warn", "vertical.ruby-empty-rt", "Empty Ruby text element was skipped", "self-closing rt has no insertion point for a closing rp", file.path))
					skipped = append(skipped, skippedEdit{Path: file.path, Target: rubyIndex + 1, Reason: "empty-rt"})
					badRT = true
					break
				}
			}
			if badRT {
				continue
			}
			if hasRP {
				complete := true
				for _, index := range rtIndexes {
					if index == 0 || index+1 >= len(ruby.Kids) || ruby.Kids[index-1].Name.Local != "rp" || ruby.Kids[index+1].Name.Local != "rp" {
						complete = false
						break
					}
				}
				if complete {
					findings = append(findings, rubyFinding("info", "vertical.ruby-has-rp", "Ruby already has fallback brackets", "all direct rt elements are flanked by rp elements", file.path))
					skipped = append(skipped, skippedEdit{Path: file.path, Target: rubyIndex + 1, Reason: "already-has-rp"})
				} else {
					findings = append(findings, rubyFinding("warn", "vertical.ruby-partial-rp", "Partial Ruby fallback was skipped", "existing rp elements do not flank every direct rt", file.path))
					skipped = append(skipped, skippedEdit{Path: file.path, Target: rubyIndex + 1, Reason: "partial-rp"})
				}
				continue
			}
			for _, rt := range rts {
				edits = append(edits,
					editset.Insert(file.path, int64(rt.Open.Start), []byte("<rp>"+p.RPOpen+"</rp>")),
					editset.Insert(file.path, int64(rt.Close.End), []byte("<rp>"+p.RPClose+"</rp>")),
				)
			}
			planned = append(planned, plannedEdit{Path: file.path, Action: "insert-rp", Target: rubyIndex + 1})
		}
	}
	return edits, planned, skipped, findings, filesScanned, nil
}

func scanWritingMode(ctx context.Context, b *book.Book, p Params) ([]editset.Edit, []plannedEdit, []skippedEdit, []report.Finding, int, error) {
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
	cssFiles := manifestCSS(pkg)
	byPath := make(map[string]sourceFile, len(cssFiles))
	for _, file := range cssFiles {
		byPath[file.path] = file
	}
	selected, skipped := selectScope(cssFiles, byPath, p.ScopePaths)
	findings := []report.Finding{}
	if p.ScopePaths != nil {
		for _, requested := range uniqueStrings(p.ScopePaths) {
			if _, ok := byPath[requested]; !ok {
				findings = append(findings, report.Finding{
					Level: "error", ID: "vertical.scope-not-css-manifest-item",
					Title:    "Scope path is not a manifest CSS item",
					Detail:   fmt.Sprintf("scope_paths entry %q does not match a manifest text/css archive path", requested),
					Location: requested,
				})
			}
		}
	}

	edits := []editset.Edit{}
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
		sheet, err := css.Parse(data)
		if err != nil {
			findings = append(findings, report.Finding{
				Level: "error", ID: "vertical.css-parse-failed",
				Title:  "CSS cannot be parsed for writing-mode prefixes",
				Detail: err.Error(), Location: file.path,
			})
			continue
		}
		for _, rule := range sheet.Rules {
			if rule.AtRule {
				continue
			}
			standard := []css.Declaration{}
			prefixValues := map[string][]string{}
			unsupported := false
			for _, decl := range rule.Declarations {
				name := strings.ToLower(strings.TrimSpace(decl.Name))
				value := strings.ToLower(strings.TrimSpace(decl.Value))
				switch name {
				case "writing-mode":
					standard = append(standard, decl)
					if strings.Contains(value, "!important") || !supportedWritingMode(value) {
						unsupported = true
					}
				case "-webkit-writing-mode", "-epub-writing-mode":
					prefixValues[name] = append(prefixValues[name], value)
				}
			}
			if len(standard) == 0 {
				continue
			}
			target := strings.TrimSpace(rule.Selector)
			if unsupported {
				findings = append(findings, cssFinding("vertical.writing-mode-unsupported-value", "Unsupported writing-mode value was skipped", "only vertical-rl, vertical-lr, and horizontal-tb without !important are supported", file.path))
				skipped = append(skipped, skippedEdit{Path: file.path, Target: target, Reason: "unsupported-writing-mode"})
				continue
			}
			value := strings.ToLower(strings.TrimSpace(standard[0].Value))
			internalConflict := false
			for _, decl := range standard[1:] {
				if strings.ToLower(strings.TrimSpace(decl.Value)) != value {
					internalConflict = true
				}
			}
			conflict := internalConflict
			for _, name := range []string{"-webkit-writing-mode", "-epub-writing-mode"} {
				for _, existing := range prefixValues[name] {
					if existing != value {
						conflict = true
					}
				}
			}
			if conflict {
				findings = append(findings, cssFinding("vertical.prefix-conflict", "Writing-mode declarations conflict", "the rule contains a standard or prefixed writing-mode value that differs from the selected standard value", file.path))
				skipped = append(skipped, skippedEdit{Path: file.path, Target: target, Reason: "prefix-conflict"})
				continue
			}
			needWebkit := len(prefixValues["-webkit-writing-mode"]) == 0
			needEPUB := len(prefixValues["-epub-writing-mode"]) == 0
			if !needWebkit && !needEPUB {
				skipped = append(skipped, skippedEdit{Path: file.path, Target: target, Reason: "already-prefixed"})
				continue
			}
			decl := standard[0]
			indent := data[decl.Span.Start:decl.NameSpan.Start]
			if len(indent) == 0 {
				indent = []byte(" ")
			}
			var insertion strings.Builder
			if needWebkit {
				insertion.WriteString("-webkit-writing-mode: ")
				insertion.WriteString(value)
				insertion.WriteByte(';')
				insertion.Write(indent)
			}
			if needEPUB {
				insertion.WriteString("-epub-writing-mode: ")
				insertion.WriteString(value)
				insertion.WriteByte(';')
				insertion.Write(indent)
			}
			edits = append(edits, editset.Insert(file.path, int64(decl.NameSpan.Start), []byte(insertion.String())))
			planned = append(planned, plannedEdit{Path: file.path, Action: "insert-prefix", Target: target})
		}
	}
	return edits, planned, skipped, findings, filesScanned, nil
}

func spineXHTML(pkg *opf.Package) []sourceFile {
	byID := make(map[string]opf.ManifestItem, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		byID[item.ID] = item
	}
	seen := map[string]bool{}
	files := []sourceFile{}
	for _, ref := range pkg.Spine {
		item, ok := byID[ref.IDRef]
		if !ok || (item.MediaType != "application/xhtml+xml" && item.MediaType != "text/html") || item.ArchivePath == "" || seen[item.ArchivePath] {
			continue
		}
		seen[item.ArchivePath] = true
		files = append(files, sourceFile{path: item.ArchivePath})
	}
	return files
}

func manifestCSS(pkg *opf.Package) []sourceFile {
	seen := map[string]bool{}
	files := []sourceFile{}
	for _, item := range pkg.Manifest {
		if item.MediaType != "text/css" || item.ArchivePath == "" || seen[item.ArchivePath] {
			continue
		}
		seen[item.ArchivePath] = true
		files = append(files, sourceFile{path: item.ArchivePath})
	}
	return files
}

func selectScope(all []sourceFile, byPath map[string]sourceFile, scope []string) ([]sourceFile, []skippedEdit) {
	if scope == nil {
		return all, []skippedEdit{}
	}
	wanted := map[string]bool{}
	for _, path := range uniqueStrings(scope) {
		if _, ok := byPath[path]; ok {
			wanted[path] = true
		}
	}
	selected := []sourceFile{}
	skipped := []skippedEdit{}
	for _, file := range all {
		if wanted[file.path] {
			selected = append(selected, file)
		} else {
			skipped = append(skipped, skippedEdit{Path: file.path, Target: "resource", Reason: "outside-scope"})
		}
	}
	return selected, skipped
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			unique = append(unique, value)
		}
	}
	return unique
}

func rawTagName(data []byte, node *opf.SpanNode) string {
	start, end := node.Open.Start, node.Open.End
	if start < 0 || end > len(data) || start >= end || data[start] != '<' {
		return ""
	}
	start++
	if start < end && data[start] == '/' {
		start++
	}
	nameEnd := start
	for nameEnd < end && data[nameEnd] != '>' && data[nameEnd] != '/' && !isXMLSpace(data[nameEnd]) {
		nameEnd++
	}
	return string(data[start:nameEnd])
}

func isXMLSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\r' || b == '\n' }

func descendant(node *opf.SpanNode, local string) *opf.SpanNode {
	for _, child := range node.Walk() {
		if child != node && child.Name.Local == local {
			return child
		}
	}
	return nil
}

// ValidRPToken reports whether value is one safe, non-space rune suitable for
// literal text inside an rp element.
func ValidRPToken(value string) bool {
	runes := []rune(value)
	if len(runes) != 1 || unicode.IsSpace(runes[0]) {
		return false
	}
	switch runes[0] {
	case '<', '>', '&', '"', '\'':
		return false
	default:
		return true
	}
}

func supportedWritingMode(value string) bool {
	return slices.Contains([]string{"vertical-rl", "vertical-lr", "horizontal-tb"}, value)
}

func hasBOM(data []byte) bool {
	return bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) ||
		bytes.HasPrefix(data, []byte{0xFE, 0xFF}) ||
		bytes.HasPrefix(data, []byte{0xFF, 0xFE}) ||
		bytes.HasPrefix(data, []byte{0x00, 0x00, 0xFE, 0xFF}) ||
		bytes.HasPrefix(data, []byte{0xFF, 0xFE, 0x00, 0x00})
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

func hasErrorFinding(findings []report.Finding) bool {
	return slices.ContainsFunc(findings, func(finding report.Finding) bool { return finding.Level == "error" })
}

func rubyFinding(level, id, title, detail, path string) report.Finding {
	return report.Finding{Level: level, ID: id, Title: title, Detail: detail, Location: path}
}

func cssFinding(id, title, detail, path string) report.Finding {
	return report.Finding{Level: "warn", ID: id, Title: title, Detail: detail, Location: path}
}
