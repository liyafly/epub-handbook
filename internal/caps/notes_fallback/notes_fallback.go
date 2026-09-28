// Package notesfallback adds Duokan legacy hooks to validated standard notes.
package notesfallback

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

const (
	CapabilityID = "epub.notes.legacy-fallback"
	UpstreamID   = "epub.notes.popup.normalize"
)

// Params contains upstream standard-note counts and an optional exact
// spine-XHTML scope. A negative upstream count means the required fact is missing.
type Params struct {
	UpstreamViolations int
	UpstreamNoterefs   int
	ScopePaths         []string
}

type spineFile struct {
	path string
}

type plannedEdit struct {
	Path  string `json:"path"`
	Tag   string `json:"tag"`
	ID    string `json:"id,omitempty"`
	Class string `json:"addClass"`
}

type skippedEdit struct {
	Path   string `json:"path"`
	Target string `json:"target"`
	Reason string `json:"reason"`
}

// Run adds only the legacy Duokan classes to an already-valid standard note
// structure. All edits are byte-range insertions made after a read-only scan.
func Run(ctx context.Context, b *book.Book, p Params) (report.Result, error) {
	if b == nil {
		return report.Result{}, errors.New("notes fallback requires an EPUB book")
	}
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	if p.UpstreamViolations != 0 {
		finding := report.Finding{
			Level: "error", ID: "notes-fallback.upstream-not-clean",
			Title:  "Standard popup notes must validate before adding legacy hooks",
			Detail: fmt.Sprintf("%s violations=%d", UpstreamID, p.UpstreamViolations),
		}
		if err := b.Apply(nil); err != nil {
			return report.Result{}, err
		}
		return report.Result{
			Capability: CapabilityID,
			Status:     report.StatusFailed,
			Facts:      map[string]any{"plannedEdits": []plannedEdit{}, "editCount": 0, "filesScanned": 0},
			Findings:   []report.Finding{finding},
		}, nil
	}

	edits, planned, skipped, findings, filesScanned, err := scanPhase(ctx, b, p)
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
	status := report.StatusComplete
	if failed {
		status = report.StatusFailed
	}
	facts := map[string]any{
		"plannedEdits": planned,
		"editCount":    len(edits),
		"filesScanned": filesScanned,
	}
	if skipped == nil {
		skipped = []skippedEdit{}
	}
	if len(skipped) > 0 {
		facts["skipped"] = skipped
	}
	return report.Result{Capability: CapabilityID, Status: status, Facts: facts, Findings: findings}, nil
}

func scanPhase(ctx context.Context, b *book.Book, p Params) ([]editset.Edit, []plannedEdit, []skippedEdit, []report.Finding, int, error) {
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
	selected, skipped := scopeFiles(spine, byPath, p.ScopePaths)
	selectedByPath := make(map[string]bool, len(selected))
	for _, file := range selected {
		selectedByPath[file.path] = true
	}
	var findings []report.Finding
	if p.ScopePaths != nil {
		for _, requested := range opf.UniqueStrings(p.ScopePaths) {
			if _, ok := byPath[requested]; !ok {
				findings = append(findings, report.Finding{
					Level: "error", ID: "notes-fallback.scope-not-in-spine",
					Title:    "Scope path is not a spine XHTML item",
					Detail:   fmt.Sprintf("scope_paths entry %q does not match a spine XHTML archive path", requested),
					Location: requested,
				})
			}
		}
	}

	var edits []editset.Edit
	planned := []plannedEdit{}
	filesScanned := 0
	noterefCount := 0
	for _, file := range spine {
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
				Level: "error", ID: "notes-fallback.unsupported-encoding",
				Title:    "XHTML encoding cannot be edited safely",
				Detail:   err.Error(),
				Location: file.path,
			})
			continue
		}
		root, err := opf.ScanXHTMLSpanTree(data)
		if err != nil {
			findings = append(findings, report.Finding{
				Level: "error", ID: "notes-fallback.parse-failed",
				Title:    "XHTML cannot be parsed for legacy note hooks",
				Detail:   err.Error(),
				Location: file.path,
			})
			continue
		}
		nodes := root.Walk()
		var noterefs, lists []*opf.SpanNode
		for _, node := range nodes {
			class, _ := node.AttrByLocal("", "class")
			switch {
			case node.Name.Local == "a" && hasOPSType(node, "noteref"):
				noterefs = append(noterefs, node)
			case node.Name.Local == "ol" && hasClass(class, "footnote-list"):
				lists = append(lists, node)
			}
		}
		noterefCount += len(noterefs)
		if len(noterefs) == 0 {
			if !selectedByPath[file.path] {
				continue
			}
			if hasFileSkip(skipped, file.path) {
				continue
			}
			skipped = append(skipped, skippedEdit{Path: file.path, Target: "notes", Reason: "no-noteref"})
			continue
		}
		if !selectedByPath[file.path] {
			continue
		}
		if len(lists) > 1 {
			findings = append(findings, report.Finding{
				Level: "error", ID: "notes-fallback.multiple-lists",
				Title:    "A spine XHTML file contains multiple footnote lists",
				Detail:   fmt.Sprintf("found %d ol.footnote-list elements", len(lists)),
				Location: file.path,
			})
		}
		for _, anchor := range noterefs {
			if !hasDescendant(anchor, "img") {
				findings = append(findings, report.Finding{
					Level: "error", ID: "notes-fallback.noteref-without-icon",
					Title:    "Noteref anchor has no image icon",
					Detail:   "SPEC §1 requires an img descendant inside each Duokan fallback noteref anchor",
					Location: file.path,
				})
				continue
			}
			if err := addClass(file.path, data, anchor, "duokan-footnote", &edits, &planned, &skipped); err != nil {
				findings = append(findings, unsafeAttributeFinding(file.path, anchor, err))
			}
		}
		for _, list := range lists {
			if err := addClass(file.path, data, list, "duokan-footnote-content", &edits, &planned, &skipped); err != nil {
				findings = append(findings, unsafeAttributeFinding(file.path, list, err))
			}
		}
		for _, node := range nodes {
			if node.Name.Local != "li" {
				continue
			}
			class, _ := node.AttrByLocal("", "class")
			if hasClass(class, "duokan-footnote-content") {
				findings = append(findings, report.Finding{
					Level: "error", ID: "notes-fallback.content-class-on-li",
					Title:    "Duokan note content class is on an li element",
					Detail:   "duokan-footnote-content belongs on ol.footnote-list, not li",
					Location: file.path,
				})
				continue
			}
			if hasClass(class, "footnote-item") {
				if err := addClass(file.path, data, node, "duokan-footnote-item", &edits, &planned, &skipped); err != nil {
					findings = append(findings, unsafeAttributeFinding(file.path, node, err))
				}
			}
		}
	}
	if p.UpstreamNoterefs != noterefCount {
		findings = append(findings, report.Finding{
			Level: "error", ID: "notes-fallback.upstream-coverage-mismatch",
			Title:    "Popup validator and fallback disagree on spine noteref coverage",
			Detail:   fmt.Sprintf("%s reported %d noterefs; fallback counted %d", UpstreamID, p.UpstreamNoterefs, noterefCount),
			Location: opfPath,
		})
	}
	if noterefCount == 0 && !hasErrorFinding(findings) {
		location := opfPath
		if len(selected) > 0 {
			location = selected[0].path
		}
		findings = append(findings, report.Finding{
			Level: "info", ID: "notes-fallback.no-notes",
			Title:    "No standard noteref anchors found in scope",
			Detail:   "no legacy note classes were needed",
			Location: location,
		})
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
	slices.SortFunc(skipped, func(a, b skippedEdit) int {
		if order := strings.Compare(a.Path, b.Path); order != 0 {
			return order
		}
		if order := strings.Compare(a.Target, b.Target); order != 0 {
			return order
		}
		return strings.Compare(a.Reason, b.Reason)
	})
	return edits, planned, skipped, findings, filesScanned, nil
}

func addClass(path string, data []byte, node *opf.SpanNode, class string, edits *[]editset.Edit, planned *[]plannedEdit, skipped *[]skippedEdit) error {
	edit, present, err := opf.ClassTokenEdit(path, data, node, class)
	if err != nil {
		return err
	}
	if present {
		*skipped = append(*skipped, skippedEdit{Path: path, Target: nodeTarget(node), Reason: "class-present"})
		return nil
	}
	*edits = append(*edits, edit)
	item := plannedEdit{Path: path, Tag: node.Name.Local, Class: class}
	item.ID, _ = node.AttrByLocal("", "id")
	*planned = append(*planned, item)
	return nil
}

func unsafeAttributeFinding(path string, node *opf.SpanNode, err error) report.Finding {
	return report.Finding{
		Level: "error", ID: "notes-fallback.unsafe-attribute",
		Title:    "Legacy class attribute cannot be edited safely",
		Detail:   fmt.Sprintf("%s: %v", nodeTarget(node), err),
		Location: path,
	}
}

func hasErrorFinding(findings []report.Finding) bool {
	return slices.ContainsFunc(findings, func(f report.Finding) bool { return f.Level == "error" })
}

func spineXHTML(pkg *opf.Package) []spineFile {
	paths := opf.SpineXHTMLPaths(pkg)
	files := make([]spineFile, 0, len(paths))
	for _, path := range paths {
		files = append(files, spineFile{path: path})
	}
	return files
}

func scopeFiles(spine []spineFile, byPath map[string]spineFile, scope []string) ([]spineFile, []skippedEdit) {
	paths := make([]string, 0, len(spine))
	for _, file := range spine {
		paths = append(paths, file.path)
	}
	selectedPaths, skippedPaths := opf.SelectScopePaths(paths, scope)
	selected := make([]spineFile, 0, len(selectedPaths))
	for _, path := range selectedPaths {
		selected = append(selected, byPath[path])
	}
	skipped := make([]skippedEdit, 0, len(skippedPaths))
	for _, path := range skippedPaths {
		skipped = append(skipped, skippedEdit{Path: path, Target: "spine-xhtml", Reason: "outside-scope"})
	}
	return selected, skipped
}

func hasFileSkip(skipped []skippedEdit, path string) bool {
	return slices.ContainsFunc(skipped, func(item skippedEdit) bool { return item.Path == path && item.Reason == "outside-scope" })
}

func hasOPSType(node *opf.SpanNode, token string) bool {
	typeValue, _ := node.AttrByLocal(opf.OPSURI, "type")
	return hasClass(typeValue, token)
}

func hasClass(classValue, token string) bool {
	return slices.Contains(strings.Fields(classValue), token)
}

func hasDescendant(node *opf.SpanNode, local string) bool {
	for _, descendant := range node.Walk() {
		if descendant != node && descendant.Name.Local == local {
			return true
		}
	}
	return false
}

func nodeTarget(node *opf.SpanNode) string {
	if id, ok := node.AttrByLocal("", "id"); ok && id != "" {
		return node.Name.Local + "#" + id
	}
	return node.Name.Local
}
