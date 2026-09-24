// Package kindlecheck reports deterministic static Kindle compatibility risks.
package kindlecheck

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/report"
	cssscan "github.com/liyafly/epub-handbook/internal/scan/css"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

const CapabilityID = "epub.kindle.compatibility.check"

// Params is intentionally empty: this validator has no tuning parameters.
type Params struct{}

type findingAt struct {
	finding report.Finding
	offset  int
}

type inspector struct {
	ctx      context.Context
	b        *book.Book
	opfPath  string
	pkg      *opf.Package
	findings []findingAt
}

// Run performs read-only checks against the OPF, manifest, spine XHTML, and
// manifest CSS. It does not interpret device or conversion logs.
func Run(ctx context.Context, b *book.Book, _ Params) (report.Result, error) {
	if err := ctx.Err(); err != nil {
		return report.Result{}, err
	}
	if b == nil {
		return report.Result{}, errors.New("kindle check requires an EPUB book")
	}
	container, err := b.CurrentContext(ctx, opf.ContainerPath)
	if err != nil {
		return report.Result{}, fmt.Errorf("kindle check read %s: %w", opf.ContainerPath, err)
	}
	opfPath, err := opf.FindOPFPath(container)
	if err != nil {
		return report.Result{}, fmt.Errorf("kindle check locate package: %w", err)
	}
	opfData, err := b.CurrentContext(ctx, opfPath)
	if err != nil {
		return report.Result{}, fmt.Errorf("kindle check read %s: %w", opfPath, err)
	}
	pkg, err := opf.Parse(opfPath, opfData)
	if err != nil {
		return report.Result{}, err
	}

	ins := inspector{ctx: ctx, b: b, opfPath: opfPath, pkg: pkg}
	ins.checkPackage()
	cssFilesScanned, err := ins.checkStylesheets()
	if err != nil {
		return report.Result{}, err
	}
	xhtmlFilesScanned, err := ins.checkSpineXHTML()
	if err != nil {
		return report.Result{}, err
	}

	checks := checkIDs()
	order := make(map[string]int, len(checks))
	counts := make(map[string]int, len(checks))
	for index, id := range checks {
		order[id] = index
		counts[id] = 0
	}
	slices.SortFunc(ins.findings, func(a, b findingAt) int {
		if n := cmp.Compare(order[a.finding.ID], order[b.finding.ID]); n != 0 {
			return n
		}
		if n := strings.Compare(a.finding.Location, b.finding.Location); n != 0 {
			return n
		}
		if n := cmp.Compare(a.offset, b.offset); n != 0 {
			return n
		}
		return strings.Compare(a.finding.Detail, b.finding.Detail)
	})

	findings := make([]report.Finding, 0, len(ins.findings))
	failed := false
	for _, item := range ins.findings {
		findings = append(findings, item.finding)
		counts[item.finding.ID]++
		failed = failed || item.finding.Level == "error"
	}
	status := report.StatusComplete
	if failed {
		status = report.StatusFailed
	}
	return report.Result{
		Capability: CapabilityID,
		Status:     status,
		Facts: map[string]any{
			"checks":            checks,
			"counts":            counts,
			"cssFilesScanned":   cssFilesScanned,
			"xhtmlFilesScanned": xhtmlFilesScanned,
			"staticOnly":        true,
		},
		Findings: findings,
	}, nil
}

func checkIDs() []string {
	return []string{
		"kindle.ncx-missing",
		"kindle.cover-image-missing",
		"kindle.cover-meta-missing",
		"kindle.cover-not-raster",
		"kindle.image-webp",
		"kindle.image-tiff",
		"kindle.image-gif",
		"kindle.image-svg",
		"kindle.mathml-properties-missing",
		"kindle.css-transform-rotate",
		"kindle.css-styled-underline",
		"kindle.css-amzn-media-query",
		"kindle.css-img-direct-float",
		"kindle.css-unicode-range",
		"kindle.css-parse-failed",
		"kindle.xhtml-parse-failed",
	}
}

func (i *inspector) add(level, id, title, detail, location string, offset int) {
	i.findings = append(i.findings, findingAt{
		finding: report.Finding{Level: level, ID: id, Title: title, Detail: detail, Location: location},
		offset:  offset,
	})
}

func (i *inspector) checkPackage() {
	if _, ok := i.pkg.NCXItem(); !ok || i.pkg.SpineToc == "" {
		i.add("warn", "kindle.ncx-missing", "NCX compatibility reference is missing",
			"manifest must include an NCX item and spine must reference it with toc", i.opfPath, 0)
	}

	cover, hasCover := i.pkg.CoverItem()
	if !hasCover {
		i.add("warn", "kindle.cover-image-missing", "Cover image declaration is missing",
			"no manifest item declares the cover-image property", i.opfPath, 0)
	}
	metaCoverFound, metaCoverMatches := false, false
	for _, meta := range i.pkg.Metas {
		if !strings.EqualFold(strings.TrimSpace(meta.Name), "cover") {
			continue
		}
		metaCoverFound = true
		if hasCover && meta.Content == cover.ID {
			metaCoverMatches = true
			break
		}
	}
	if !metaCoverFound || !metaCoverMatches {
		detail := "metadata must contain name=cover with content equal to the cover-image manifest id"
		if metaCoverFound {
			detail = "name=cover metadata does not point to the manifest item with cover-image"
		}
		i.add("warn", "kindle.cover-meta-missing", "Kindle cover metadata is missing or mismatched", detail, i.opfPath, 0)
	}
	if hasCover && cover.MediaType != "image/jpeg" && cover.MediaType != "image/png" {
		i.add("warn", "kindle.cover-not-raster", "Cover image is not a supported raster format",
			fmt.Sprintf("cover item %s has media-type %q; use image/jpeg or image/png for the Kindle main path", cover.ID, cover.MediaType), itemLocation(cover, i.opfPath), 0)
	}

	for _, item := range i.pkg.Manifest {
		mediaType := strings.ToLower(strings.TrimSpace(item.MediaType))
		ext := strings.ToLower(path.Ext(resourcePath(item)))
		location := itemLocation(item, i.opfPath)
		if mediaType == "image/webp" || ext == ".webp" {
			i.add("error", "kindle.image-webp", "WebP image is outside the Kindle main path",
				fmt.Sprintf("manifest item %s uses %s or a .webp resource", item.ID, item.MediaType), location, 0)
		}
		if mediaType == "image/tiff" || ext == ".tif" || ext == ".tiff" {
			i.add("warn", "kindle.image-tiff", "TIFF image may be incompatible",
				fmt.Sprintf("manifest item %s uses %s or a TIFF resource", item.ID, item.MediaType), location, 0)
		}
		if mediaType == "image/gif" {
			i.add("warn", "kindle.image-gif", "GIF image requires manual frame review",
				fmt.Sprintf("manifest item %s is image/gif; frame count cannot be determined statically", item.ID), location, 0)
		}
		if mediaType == "image/svg+xml" && !hasProperty(item.Properties, "cover-image") {
			i.add("info", "kindle.image-svg", "SVG image is present outside the cover role",
				fmt.Sprintf("manifest item %s is a non-cover SVG; verify its fallback on target Kindle formats", item.ID), location, 0)
		}
	}
}

func (i *inspector) checkStylesheets() (int, error) {
	seen := make(map[string]bool)
	filesScanned := 0
	for _, item := range i.pkg.Manifest {
		if item.MediaType != "text/css" || item.ArchivePath == "" || seen[item.ArchivePath] {
			continue
		}
		seen[item.ArchivePath] = true
		if err := i.ctx.Err(); err != nil {
			return filesScanned, err
		}
		filesScanned++
		data, err := i.b.CurrentContext(i.ctx, item.ArchivePath)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return filesScanned, err
			}
			if errors.Is(err, book.ErrMissingEntry) {
				i.add("warn", "kindle.css-parse-failed", "Stylesheet cannot be inspected",
					"manifest stylesheet entry is missing: "+err.Error(), item.ArchivePath, 0)
				continue
			}
			return filesScanned, fmt.Errorf("kindle check read %s: %w", item.ArchivePath, err)
		}
		sheet, err := cssscan.Parse(data)
		if err != nil {
			i.add("warn", "kindle.css-parse-failed", "Stylesheet cannot be inspected", err.Error(), item.ArchivePath, 0)
			continue
		}
		i.checkCSSRules(item.ArchivePath, data, sheet.Rules)
	}
	return filesScanned, nil
}

func (i *inspector) checkCSSRules(path string, data []byte, rules []cssscan.Rule) {
	for _, rule := range rules {
		if !rule.Span.Valid() || rule.Span.End > len(data) {
			continue
		}
		if rule.AtRule && strings.EqualFold(rule.AtRuleName, "media") {
			prelude := atRulePrelude(data, rule)
			lowerPrelude := strings.ToLower(cssscan.StripComments(prelude))
			if strings.Contains(lowerPrelude, "amzn-kf8") || strings.Contains(lowerPrelude, "amzn-mobi") {
				i.add("warn", "kindle.css-amzn-media-query", "Kindle-specific media query is present",
					fmt.Sprintf("@media prelude %q contains amzn-kf8 or amzn-mobi", strings.TrimSpace(prelude)), path, rule.Span.Start)
			}
		}
		if rule.AtRule && strings.EqualFold(rule.AtRuleName, "font-face") && hasDeclaration(rule.Declarations, "unicode-range") {
			i.add("info", "kindle.css-unicode-range", "Font face uses unicode-range",
				"Kindle handling of unicode-range is not stable; verify the target format", path, rule.Span.Start)
		}

		rotate := false
		styledUnderline := false
		styleSeen, underlineBeforeStyle := false, false
		floatSide := ""
		for _, decl := range rule.Declarations {
			name := strings.ToLower(strings.TrimSpace(decl.Name))
			value := strings.ToLower(strings.TrimSpace(cssscan.StripComments(decl.Value)))
			switch name {
			case "transform", "-webkit-transform":
				rotate = rotate || strings.Contains(value, "rotate")
			case "text-decoration":
				for _, token := range strings.Fields(value) {
					if token == "wavy" || token == "dotted" || token == "dashed" || token == "double" {
						styledUnderline = true
					}
				}
				if firstWord(value) == "underline" && !styleSeen {
					underlineBeforeStyle = true
				}
			case "text-decoration-line":
				if firstWord(value) == "underline" && !styleSeen {
					underlineBeforeStyle = true
				}
			case "text-decoration-style":
				styleSeen = true
				if !underlineBeforeStyle {
					styledUnderline = true
				}
			case "float":
				word := firstWord(value)
				if word == "left" || word == "right" {
					floatSide = word
				}
			}
		}
		if rotate {
			i.add("warn", "kindle.css-transform-rotate", "CSS rotation may break Kindle conversion",
				fmt.Sprintf("transform declaration contains rotate in selector %q", strings.TrimSpace(rule.Selector)), path, rule.Span.Start)
		}
		if styledUnderline {
			i.add("warn", "kindle.css-styled-underline", "Styled underline may not degrade to a basic underline",
				fmt.Sprintf("selector %q should declare a basic underline before text-decoration-style", strings.TrimSpace(rule.Selector)), path, rule.Span.Start)
		}
		if floatSide != "" && hasDirectImgSelector(rule.Selector) {
			i.add("warn", "kindle.css-img-direct-float", "An img selector carries float directly",
				fmt.Sprintf("selector %q applies float:%s directly to img; move float to a wrapping figure", strings.TrimSpace(rule.Selector), floatSide), path, rule.Span.Start)
		}
	}
}

func (i *inspector) checkSpineXHTML() (int, error) {
	filesScanned := 0
	for _, item := range spineXHTMLItems(i.pkg) {
		if err := i.ctx.Err(); err != nil {
			return filesScanned, err
		}
		filesScanned++
		data, err := i.b.CurrentContext(i.ctx, item.ArchivePath)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return filesScanned, err
			}
			if errors.Is(err, book.ErrMissingEntry) {
				i.add("warn", "kindle.xhtml-parse-failed", "Spine XHTML cannot be inspected",
					"manifest XHTML entry is missing: "+err.Error(), item.ArchivePath, 0)
				continue
			}
			return filesScanned, fmt.Errorf("kindle check read %s: %w", item.ArchivePath, err)
		}
		root, err := opf.ScanXHTMLSpanTree(data)
		if err != nil {
			i.add("warn", "kindle.xhtml-parse-failed", "Spine XHTML cannot be inspected", err.Error(), item.ArchivePath, 0)
			continue
		}
		mathCount := 0
		for _, node := range root.Walk() {
			if node.Name.Local == "math" {
				mathCount++
			}
		}
		if mathCount > 0 && !hasProperty(item.Properties, "mathml") {
			i.add("error", "kindle.mathml-properties-missing", "MathML manifest property is missing",
				fmt.Sprintf("spine item %s contains %d math element(s) but lacks properties token mathml", item.ID, mathCount), item.ArchivePath, 0)
		}
	}
	return filesScanned, nil
}

func spineXHTMLItems(pkg *opf.Package) []opf.ManifestItem {
	byID := make(map[string]opf.ManifestItem, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		byID[item.ID] = item
	}
	seen := make(map[string]bool)
	var out []opf.ManifestItem
	for _, ref := range pkg.Spine {
		item, ok := byID[ref.IDRef]
		if !ok || item.MediaType != "application/xhtml+xml" || item.ArchivePath == "" || seen[item.ArchivePath] {
			continue
		}
		seen[item.ArchivePath] = true
		out = append(out, item)
	}
	return out
}

func hasProperty(properties, token string) bool {
	return slices.Contains(strings.Fields(properties), token)
}

func itemLocation(item opf.ManifestItem, fallback string) string {
	if item.ArchivePath != "" {
		return item.ArchivePath
	}
	if item.Href != "" {
		return item.Href
	}
	return fallback
}

func resourcePath(item opf.ManifestItem) string {
	href := item.ArchivePath
	if href == "" {
		href = item.Href
	}
	if before, _, ok := strings.Cut(href, "?"); ok {
		href = before
	}
	if before, _, ok := strings.Cut(href, "#"); ok {
		href = before
	}
	return href
}

func hasDeclaration(declarations []cssscan.Declaration, name string) bool {
	for _, declaration := range declarations {
		if strings.EqualFold(strings.TrimSpace(declaration.Name), name) {
			return true
		}
	}
	return false
}

func atRulePrelude(data []byte, rule cssscan.Rule) string {
	if rule.Span.Start < 0 || rule.Span.Start >= len(data) || rule.Span.End > len(data) {
		return ""
	}
	span := data[rule.Span.Start:rule.Span.End]
	brace := bytes.IndexByte(span, '{')
	if brace < 0 {
		return ""
	}
	return string(span[:brace])
}

func firstWord(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func hasDirectImgSelector(selector string) bool {
	for _, part := range strings.Split(cssscan.StripComments(selector), ",") {
		if directImgSelectorRE.MatchString(strings.ToLower(strings.TrimSpace(part))) {
			return true
		}
	}
	return false
}
