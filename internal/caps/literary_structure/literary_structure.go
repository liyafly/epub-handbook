// Package literarystructure applies explicit, reviewable class assignments to
// spine XHTML without inferring roles or serializing whole documents.
package literarystructure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

const CapabilityID = "epub.literary.structure.format"

// Assignment names one XHTML element and one allowed class token.
type Assignment struct {
	Path  string `json:"path"`
	ID    string `json:"id,omitempty"`
	Tag   string `json:"tag,omitempty"`
	Index *int   `json:"index,omitempty"`
	Class string `json:"class"`
}

// Params contains the explicit assignment list and an optional manifest CSS
// path to link from each XHTML file that receives a new class.
type Params struct {
	Assignments []Assignment
	Stylesheet  string
}

type plannedEdit struct {
	Path   string `json:"path"`
	Target string `json:"target"`
	Action string `json:"action"`
	Value  string `json:"value"`
}

type skippedEdit struct {
	Path   string `json:"path"`
	Target string `json:"target"`
	Reason string `json:"reason"`
}

type sourceFile struct {
	path string
}

type parsedXHTML struct {
	data []byte
	root *opf.SpanNode
}

type targetPlan struct {
	path      string
	node      *opf.SpanNode
	target    string
	classes   []string
	seenClass map[string]bool
}

// Run validates the explicit target list, prepares class/link byte-range
// edits, and applies the complete set only when no error finding exists.
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	if b == nil {
		return report.Result{}, errors.New("literary structure formatter requires an EPUB book")
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	if len(p.Assignments) == 0 {
		return report.Result{}, errors.New("literary structure formatter requires at least one assignment")
	}

	container, err := b.CurrentContext(ctx, opf.ContainerPath)
	if err != nil {
		return report.Result{}, fmt.Errorf("read %s: %w", opf.ContainerPath, err)
	}
	opfPath, err := opf.FindOPFPath(container)
	if err != nil {
		return report.Result{}, err
	}
	opfData, err := b.CurrentContext(ctx, opfPath)
	if err != nil {
		return report.Result{}, fmt.Errorf("read %s: %w", opfPath, err)
	}
	pkg, err := opf.Parse(opfPath, opfData)
	if err != nil {
		return report.Result{}, err
	}

	spine := spineXHTML(pkg)
	byPath := make(map[string]sourceFile, len(spine))
	for _, file := range spine {
		byPath[file.path] = file
	}
	manifestCSS := make(map[string]bool)
	for _, item := range pkg.Manifest {
		if item.MediaType == "text/css" && item.ArchivePath != "" {
			manifestCSS[item.ArchivePath] = true
		}
	}

	findings := []report.Finding{}
	skipped := []skippedEdit{}
	if p.Stylesheet != "" && !manifestCSS[p.Stylesheet] {
		findings = append(findings, report.Finding{
			Level: "error", ID: "literary.stylesheet-not-in-manifest",
			Title:    "Stylesheet is not a manifest CSS item",
			Detail:   fmt.Sprintf("stylesheet %q does not match a manifest text/css archive path", p.Stylesheet),
			Location: p.Stylesheet,
		})
	}

	vocabulary := classVocabulary()
	loaded := map[string]bool{}
	files := map[string]*parsedXHTML{}
	plans := []targetPlan{}
	planByNode := map[*opf.SpanNode]int{}

	for _, assignment := range p.Assignments {
		if err := ctx.Err(); err != nil {
			return report.Result{}, err
		}
		if !vocabulary[assignment.Class] {
			findings = append(findings, assignmentFinding(
				"literary.class-not-allowed", "Class token is outside the literary vocabulary",
				fmt.Sprintf("class %q is not listed by SPEC §7 or the classical-modern guide", assignment.Class), assignment.Path,
			))
		}
		if _, ok := byPath[assignment.Path]; !ok {
			findings = append(findings, assignmentFinding(
				"literary.path-not-in-spine", "Assignment path is not a spine XHTML item",
				fmt.Sprintf("path %q does not match a spine XHTML archive path", assignment.Path), assignment.Path,
			))
			continue
		}
		if !vocabulary[assignment.Class] {
			continue
		}
		if !loaded[assignment.Path] {
			loaded[assignment.Path] = true
			data, readErr := b.CurrentContext(ctx, assignment.Path)
			if readErr != nil {
				return report.Result{}, fmt.Errorf("read %s: %w", assignment.Path, readErr)
			}
			if hasBOM(data) || hasNonUTF8Declaration(data) {
				findings = append(findings, assignmentFinding(
					"literary.unsupported-encoding", "XHTML encoding cannot be edited safely",
					"BOM or XML declaration is not UTF-8; byte spans would not match the source", assignment.Path,
				))
				continue
			}
			root, parseErr := opf.ScanXHTMLSpanTree(data)
			if parseErr != nil {
				findings = append(findings, assignmentFinding(
					"literary.parse-failed", "XHTML cannot be parsed for class assignments", parseErr.Error(), assignment.Path,
				))
				continue
			}
			if root.Name.Local != "html" {
				findings = append(findings, assignmentFinding(
					"literary.parse-failed", "Spine item root is not html",
					fmt.Sprintf("root element is %q", root.Name.Local), assignment.Path,
				))
				continue
			}
			files[assignment.Path] = &parsedXHTML{data: data, root: root}
		}
		file := files[assignment.Path]
		if file == nil {
			continue
		}

		node, idMatches, found := resolveTarget(file.root, assignment)
		target := targetName(assignment)
		if !found {
			findings = append(findings, assignmentFinding(
				"literary.target-not-found", "Assignment target was not found",
				fmt.Sprintf("no element matches target %q", target), assignment.Path,
			))
			continue
		}
		if idMatches > 1 {
			findings = append(findings, assignmentFinding(
				"literary.target-ambiguous", "Assignment id matches multiple elements",
				fmt.Sprintf("id %q appears %d times in this XHTML", assignment.ID, idMatches), assignment.Path,
			))
			continue
		}
		if forbiddenTarget(node) {
			findings = append(findings, assignmentFinding(
				"literary.target-forbidden", "Assignment target is in document head or is a document root",
				fmt.Sprintf("target %q cannot receive literary structure classes", target), assignment.Path,
			))
			continue
		}

		planIndex, exists := planByNode[node]
		if !exists {
			planIndex = len(plans)
			planByNode[node] = planIndex
			plans = append(plans, targetPlan{
				path: assignment.Path, node: node, target: target,
				classes: []string{}, seenClass: map[string]bool{},
			})
		}
		plan := &plans[planIndex]
		if !plan.seenClass[assignment.Class] {
			plan.seenClass[assignment.Class] = true
			plan.classes = append(plan.classes, assignment.Class)
		}
	}

	edits := []editset.Edit{}
	planned := []plannedEdit{}
	modifiedPaths := []string{}
	modifiedSeen := map[string]bool{}
	for _, plan := range plans {
		if err := ctx.Err(); err != nil {
			return report.Result{}, err
		}
		file := files[plan.path]
		classValue, _ := plan.node.AttrByLocal("", "class")
		newClasses := []string{}
		for _, class := range plan.classes {
			if containsClassToken(classValue, class) {
				findings = append(findings, report.Finding{
					Level: "info", ID: "literary.already-has-class",
					Title:    "Target already has the requested class",
					Detail:   fmt.Sprintf("%s already contains class %q", plan.target, class),
					Location: plan.path,
				})
				skipped = append(skipped, skippedEdit{Path: plan.path, Target: plan.target, Reason: "class-present"})
				continue
			}
			newClasses = append(newClasses, class)
		}
		if len(newClasses) == 0 {
			continue
		}
		edit, present, editErr := opf.ClassTokenEdit(plan.path, file.data, plan.node, newClasses[0])
		if editErr != nil {
			findings = append(findings, assignmentFinding(
				"literary.unsafe-attribute", "Class attribute cannot be edited safely", editErr.Error(), plan.path,
			))
			continue
		}
		if present {
			findings = append(findings, report.Finding{
				Level: "info", ID: "literary.already-has-class",
				Title:    "Target already has the requested class",
				Detail:   fmt.Sprintf("%s already contains class %q", plan.target, newClasses[0]),
				Location: plan.path,
			})
			continue
		}
		if len(newClasses) > 1 {
			edit.Replacement = append(edit.Replacement, []byte(" "+strings.Join(newClasses[1:], " "))...)
		}
		edits = append(edits, edit)
		planned = append(planned, plannedEdit{
			Path: plan.path, Target: plan.target, Action: "add-class", Value: strings.Join(newClasses, " "),
		})
		if !modifiedSeen[plan.path] {
			modifiedSeen[plan.path] = true
			modifiedPaths = append(modifiedPaths, plan.path)
		}
	}

	if p.Stylesheet != "" && manifestCSS[p.Stylesheet] {
		for _, path := range modifiedPaths {
			if err := ctx.Err(); err != nil {
				return report.Result{}, err
			}
			file := files[path]
			if hasStylesheetLink(file.root, path, p.Stylesheet) {
				findings = append(findings, report.Finding{
					Level: "info", ID: "literary.stylesheet-already-linked",
					Title:    "Stylesheet is already linked",
					Detail:   fmt.Sprintf("a link in %s already resolves to %s", path, p.Stylesheet),
					Location: path,
				})
				skipped = append(skipped, skippedEdit{Path: path, Target: "link", Reason: "stylesheet-already-linked"})
				continue
			}
			head := directHead(file.root)
			if head == nil || head.Close.IsZero() {
				findings = append(findings, assignmentFinding(
					"literary.no-head", "XHTML has no closed head element",
					"stylesheet link requires a head element with a closing tag", path,
				))
				continue
			}
			href := pypath.RelativeURI(path, p.Stylesheet)
			link := `<link rel="stylesheet" type="text/css" href="` + href + `"/>` + "\n"
			edits = append(edits, editset.Insert(path, int64(head.Close.Start), []byte(link)))
			planned = append(planned, plannedEdit{Path: path, Target: "head", Action: "add-link", Value: href})
		}
	}

	failed := hasErrorFinding(findings)
	if failed {
		edits = nil
		planned = []plannedEdit{}
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	if err := b.Apply(edits); err != nil {
		return report.Result{}, fmt.Errorf("%s: %w", CapabilityID, err)
	}
	status := report.StatusComplete
	if failed {
		status = report.StatusFailed
	}
	return report.Result{
		Capability: CapabilityID,
		Status:     status,
		Facts: map[string]any{
			"plannedEdits":     planned,
			"skipped":          skipped,
			"editCount":        len(edits),
			"assignmentsTotal": len(p.Assignments),
		},
		Findings: findings,
	}, nil
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

func resolveTarget(root *opf.SpanNode, assignment Assignment) (*opf.SpanNode, int, bool) {
	if assignment.ID != "" {
		var match *opf.SpanNode
		count := 0
		for _, node := range root.Walk() {
			if id, ok := node.AttrByLocal("", "id"); ok && id == assignment.ID {
				match = node
				count++
			}
		}
		return match, count, count > 0
	}
	if assignment.Tag == "" || assignment.Index == nil || *assignment.Index < 0 {
		return nil, 0, false
	}
	index := 0
	for _, node := range root.Walk() {
		if node.Name.Local != assignment.Tag {
			continue
		}
		if index == *assignment.Index {
			return node, 0, true
		}
		index++
	}
	return nil, 0, false
}

func targetName(assignment Assignment) string {
	if assignment.ID != "" {
		return "#" + assignment.ID
	}
	if assignment.Index == nil {
		return assignment.Tag
	}
	return fmt.Sprintf("%s[%d]", assignment.Tag, *assignment.Index)
}

func forbiddenTarget(node *opf.SpanNode) bool {
	if node.Name.Local == "html" || node.Name.Local == "head" {
		return true
	}
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if parent.Name.Local == "head" {
			return true
		}
	}
	return false
}

func directHead(root *opf.SpanNode) *opf.SpanNode {
	for _, child := range root.Kids {
		if child.Name.Local == "head" {
			return child
		}
	}
	return nil
}

func hasStylesheetLink(root *opf.SpanNode, xhtmlPath, stylesheet string) bool {
	for _, node := range root.Walk() {
		if node.Name.Local != "link" {
			continue
		}
		href, ok := node.AttrByLocal("", "href")
		if !ok {
			continue
		}
		parts := pypath.URLSplit(href)
		if parts.Scheme != "" || parts.Netloc != "" {
			continue
		}
		resolved, err := pypath.ResolveRelativePath(xhtmlPath, parts.Path)
		if err == nil && resolved == stylesheet {
			return true
		}
	}
	return false
}

func containsClassToken(value, token string) bool {
	for existing := range strings.FieldsSeq(value) {
		if existing == token {
			return true
		}
	}
	return false
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

func isXMLSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\r' || b == '\n' }

func assignmentFinding(id, title, detail, path string) report.Finding {
	return report.Finding{Level: "error", ID: id, Title: title, Detail: detail, Location: path}
}

func hasErrorFinding(findings []report.Finding) bool {
	return slices.ContainsFunc(findings, func(finding report.Finding) bool { return finding.Level == "error" })
}
