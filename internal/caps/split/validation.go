package split

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/liyafly/epub-handbook/internal/book"
	"github.com/liyafly/epub-handbook/internal/redline"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// segmentValidation contains facts proved against one in-memory projection.
// The text/spine/anchors facts are deliberately partition-scoped: comparing a
// complete source book with one segment would report expected omissions as
// false failures (or require an unsafe allow-list).
type segmentValidation struct {
	redline   map[string]any
	partition map[string]any
}

func validateSegment(ctx context.Context, original, segment *book.Book, sourcePkg *pkgInfo, selected, expectedSpine []string, navID, navPath, ncxPath string) (segmentValidation, error) {
	if err := contextErr(ctx); err != nil {
		return segmentValidation{}, err
	}

	// These three checks retain the common redline implementations. Metadata
	// and cover are expected to survive every partition, while DRM is a hard
	// package-level refusal and is checked again at the segment boundary.
	findings, err := redline.Check(redline.OriginalState(original), redline.CurrentState(segment), []string{
		redline.CheckMetadata, redline.CheckCover, redline.CheckDRM,
	}, redline.Options{})
	if err != nil {
		return segmentValidation{}, err
	}
	if len(findings) > 0 {
		return segmentValidation{}, fmt.Errorf("%s", findings[0].Message)
	}

	currentOPF, err := segment.Current(sourcePkg.opfPath)
	if err != nil {
		return segmentValidation{}, fmt.Errorf("read projected OPF: %w", err)
	}
	if err := contextErr(ctx); err != nil {
		return segmentValidation{}, err
	}
	projectedPkg, err := opf.Parse(sourcePkg.opfPath, currentOPF)
	if err != nil {
		return segmentValidation{}, fmt.Errorf("parse projected OPF: %w", err)
	}
	if err := validateProjectedSpine(segment, projectedPkg, selected, expectedSpine, navID); err != nil {
		return segmentValidation{}, err
	}
	if err := validateProjectedManifest(segment, projectedPkg); err != nil {
		return segmentValidation{}, err
	}
	if err := validateRetainedReferences(ctx, segment, projectedPkg, sourcePkg.opfPath); err != nil {
		return segmentValidation{}, err
	}

	anchorCounts := make(map[string]int, len(selected))
	textBlocks := make(map[string]int, len(selected))
	selectedTextHashes := make([]map[string]any, 0, len(selected))
	for _, archivePath := range selected {
		if err := contextErr(ctx); err != nil {
			return segmentValidation{}, err
		}
		before, err := original.Original(archivePath)
		if err != nil {
			return segmentValidation{}, fmt.Errorf("read source spine %q: %w", archivePath, err)
		}
		after, err := segment.Current(archivePath)
		if err != nil {
			return segmentValidation{}, fmt.Errorf("projected spine missing %q: %w", archivePath, err)
		}
		if !bytes.Equal(before, after) {
			return segmentValidation{}, fmt.Errorf("selected spine XHTML bytes changed: %s", archivePath)
		}
		beforeBlocks, err := redline.ExtractTextBlocks(before, archivePath)
		if err != nil {
			return segmentValidation{}, fmt.Errorf("text partition %q: %w", archivePath, err)
		}
		afterBlocks, err := redline.ExtractTextBlocks(after, archivePath)
		if err != nil {
			return segmentValidation{}, fmt.Errorf("projected text partition %q: %w", archivePath, err)
		}
		beforeHashes := redline.BlockHashes(beforeBlocks)
		afterHashes := redline.BlockHashes(afterBlocks)
		if !slices.Equal(beforeHashes, afterHashes) {
			return segmentValidation{}, fmt.Errorf("text partition changed: %s", archivePath)
		}
		beforeIDs, err := redline.ExtractAnchorIDs(before, archivePath)
		if err != nil {
			return segmentValidation{}, fmt.Errorf("anchor partition %q: %w", archivePath, err)
		}
		afterIDs, err := redline.ExtractAnchorIDs(after, archivePath)
		if err != nil {
			return segmentValidation{}, fmt.Errorf("projected anchor partition %q: %w", archivePath, err)
		}
		if !sameStringSet(beforeIDs, afterIDs) {
			return segmentValidation{}, fmt.Errorf("anchor partition changed: %s", archivePath)
		}
		anchorCounts[archivePath] = len(afterIDs)
		textBlocks[archivePath] = len(afterBlocks)
		selectedTextHashes = append(selectedTextHashes, map[string]any{
			"path": archivePath, "blocks": beforeHashes,
		})
	}

	linkCounts, err := validateGeneratedReferences(ctx, segment, navPath, ncxPath)
	if err != nil {
		return segmentValidation{}, err
	}
	return segmentValidation{
		redline: map[string]any{
			"drm":      "pass",
			"metadata": "pass",
			"cover":    "pass",
		},
		partition: map[string]any{
			"text":          "pass",
			"spine":         "pass",
			"anchors":       "pass",
			"selectedSpine": append([]string(nil), selected...),
			"textBlocks":    textBlocks,
			"anchorCounts":  anchorCounts,
			"textHashes":    selectedTextHashes,
			"navTargets":    linkCounts[navPath],
			"ncxTargets":    linkCounts[ncxPath],
		},
	}, nil
}

func validateProjectedSpine(segment *book.Book, projected *opf.Package, selected, expected []string, navID string) error {
	if len(projected.Spine) != len(expected)+1 {
		return fmt.Errorf("spine partition has %d itemrefs, want nav plus %d selected", len(projected.Spine), len(expected))
	}
	if len(projected.Spine) == 0 || projected.Spine[0].IDRef != navID {
		return fmt.Errorf("spine partition nav itemref is not %q", navID)
	}
	for i, idref := range expected {
		if projected.Spine[i+1].IDRef != idref {
			return fmt.Errorf("spine partition order changed at %d: got %q want %q", i, projected.Spine[i+1].IDRef, idref)
		}
		item, ok := projected.ItemByID(idref)
		if !ok || !slices.Contains(selected, item.ArchivePath) {
			return fmt.Errorf("spine partition item %q is not selected content", idref)
		}
	}
	return nil
}

func validateProjectedManifest(segment *book.Book, projected *opf.Package) error {
	if !segment.Has("mimetype") {
		return fmt.Errorf("projected segment has no mimetype")
	}
	mimetype, err := segment.Current("mimetype")
	if err != nil {
		return fmt.Errorf("read projected mimetype: %w", err)
	}
	if string(mimetype) != canonicalMimetype {
		return fmt.Errorf("projected mimetype is not canonical")
	}
	for _, item := range projected.Manifest {
		if item.ArchivePath == "" {
			continue
		}
		if !segment.Has(item.ArchivePath) {
			return fmt.Errorf("manifest target missing from segment: %s", item.ArchivePath)
		}
	}
	return nil
}

// validateRetainedReferences is the reverse side of resource-closure
// collection. Every retained XHTML/XML/CSS local reference must resolve to a
// segment entry; content resources must also be represented by the projected
// manifest. Package-control files are the narrow exception because OCF's
// mimetype, container.xml, and the OPF itself are not manifest resources.
func validateRetainedReferences(ctx context.Context, segment *book.Book, projected *opf.Package, opfPath string) error {
	manifestPaths := make(map[string]bool, len(projected.Manifest))
	for _, item := range projected.Manifest {
		if item.ArchivePath != "" {
			manifestPaths[item.ArchivePath] = true
		}
	}
	for _, documentPath := range segment.Names() {
		if err := contextErr(ctx); err != nil {
			return err
		}
		if documentPath == "mimetype" || documentPath == "META-INF/container.xml" || documentPath == "META-INF/encryption.xml" {
			continue
		}
		ext := strings.ToLower(pathExt(documentPath))
		if ext == ".css" {
			data, err := segment.Current(documentPath)
			if err != nil {
				return fmt.Errorf("read retained CSS %s: %w", documentPath, err)
			}
			if !utf8.Valid(data) {
				return fmt.Errorf("retained CSS %s is not valid UTF-8", documentPath)
			}
			rawURIs, err := collectCSSURIsStrict(string(data))
			if err != nil {
				return fmt.Errorf("parse retained CSS %s: %w", documentPath, err)
			}
			for _, raw := range rawURIs {
				if err := validateRetainedReference(ctx, segment, documentPath, "CSS url", raw, manifestPaths, opfPath); err != nil {
					return err
				}
			}
			continue
		}
		if !markupExtensions[ext] {
			continue
		}
		data, err := segment.Current(documentPath)
		if err != nil {
			return fmt.Errorf("read retained XML %s: %w", documentPath, err)
		}
		if !utf8.Valid(data) {
			return fmt.Errorf("retained XML %s is not valid UTF-8", documentPath)
		}
		root, err := opf.ScanSpanTree(data)
		if err != nil {
			return fmt.Errorf("parse retained XML %s: %w", documentPath, err)
		}
		for _, node := range root.Walk() {
			if err := contextErr(ctx); err != nil {
				return err
			}
			for _, attr := range node.Attrs {
				if attr.Name.Space == "" && attr.Name.Local == "style" {
					if err := validateInlineCSSReferences(ctx, segment, documentPath, "style attribute", attr.Value, manifestPaths, opfPath); err != nil {
						return err
					}
					continue
				}
				kind, ok := resourceAttributeKind(attr.Name.Space, attr.Name.Local)
				if !ok {
					continue
				}
				if kind == "srcset" {
					candidates, err := parseSrcsetCandidates(attr.Value)
					if err != nil {
						return fmt.Errorf("parse srcset in %s: %w", documentPath, err)
					}
					for _, candidate := range candidates {
						if err := validateRetainedReference(ctx, segment, documentPath, "srcset", candidate.url, manifestPaths, opfPath); err != nil {
							return err
						}
					}
					continue
				}
				if err := validateRetainedReference(ctx, segment, documentPath, kind, attr.Value, manifestPaths, opfPath); err != nil {
					return err
				}
			}
			if node.Name.Local == "style" {
				if err := validateInlineCSSReferences(ctx, segment, documentPath, "style element", node.IterText(), manifestPaths, opfPath); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateInlineCSSReferences(ctx context.Context, segment *book.Book, documentPath, kind, text string, manifestPaths map[string]bool, opfPath string) error {
	rawURIs, err := collectCSSURIsStrict(text)
	if err != nil {
		return fmt.Errorf("parse %s in %s: %w", kind, documentPath, err)
	}
	for _, raw := range rawURIs {
		if err := validateRetainedReference(ctx, segment, documentPath, kind, raw, manifestPaths, opfPath); err != nil {
			return err
		}
	}
	return nil
}

func resourceAttributeKind(space, local string) (string, bool) {
	switch local {
	case "href":
		if space == "" || space == "http://www.w3.org/1999/xlink" {
			if space == "http://www.w3.org/1999/xlink" {
				return "xlink:href", true
			}
			return "href", true
		}
	case "src", "poster", "data", "textref", "srcset":
		return local, true
	}
	return "", false
}

func validateRetainedReference(ctx context.Context, segment *book.Book, basePath, kind, raw string, manifestPaths map[string]bool, opfPath string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	parts := pyURLSplit(raw)
	if raw == "" || pyIsExternalURI(raw) || parts.scheme != "" || parts.netloc != "" || strings.HasPrefix(parts.path, "/") {
		return nil
	}
	targetPath := basePath
	if parts.path != "" {
		resolved, err := resolveRelativePath(basePath, parts.path)
		if err != nil {
			return fmt.Errorf("resource reference %s=%q from %s: %w", kind, raw, basePath, err)
		}
		targetPath = resolved
	}
	if !segment.Has(targetPath) {
		return fmt.Errorf("resource reference target missing: %s (from %s %s=%q)", targetPath, basePath, kind, raw)
	}
	if requiresManifestEntry(targetPath, opfPath) && !manifestPaths[targetPath] {
		return fmt.Errorf("resource reference target is not in manifest: %s (from %s %s=%q)", targetPath, basePath, kind, raw)
	}
	return nil
}

func requiresManifestEntry(targetPath, opfPath string) bool {
	switch targetPath {
	case "mimetype", "META-INF/container.xml", "META-INF/encryption.xml", opfPath:
		return false
	default:
		return true
	}
}

func validateGeneratedReferences(ctx context.Context, segment *book.Book, navPath, ncxPath string) (map[string]int, error) {
	counts := map[string]int{navPath: 0, ncxPath: 0}
	for _, documentPath := range []string{navPath, ncxPath} {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		data, err := segment.Current(documentPath)
		if err != nil {
			return nil, fmt.Errorf("generated navigation missing %s: %w", documentPath, err)
		}
		root, err := opf.ScanSpanTree(data)
		if err != nil {
			return nil, fmt.Errorf("generated navigation %s is invalid: %w", documentPath, err)
		}
		for _, node := range root.Walk() {
			if err := contextErr(ctx); err != nil {
				return nil, err
			}
			for _, attr := range node.Attrs {
				isTarget := (documentPath == navPath && node.Name.Local == "a" && attr.Name.Local == "href") ||
					(documentPath == ncxPath && node.Name.Local == "content" && attr.Name.Local == "src")
				if !isTarget {
					continue
				}
				if err := validateLocalReference(ctx, segment, documentPath, attr.Value); err != nil {
					return nil, err
				}
				counts[documentPath]++
			}
		}
	}
	return counts, nil
}

func validateLocalReference(ctx context.Context, segment *book.Book, basePath, raw string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	parts := pyURLSplit(raw)
	if pyIsExternalURI(raw) || parts.scheme != "" || parts.netloc != "" {
		return nil
	}
	targetPath := basePath
	if parts.path != "" {
		resolved, err := resolveRelativePath(basePath, parts.path)
		if err != nil {
			return fmt.Errorf("navigation href %q from %s: %w", raw, basePath, err)
		}
		targetPath = resolved
	}
	if !segment.Has(targetPath) {
		return fmt.Errorf("navigation target missing: %s (from %s)", targetPath, basePath)
	}
	if parts.fragment == "" {
		return nil
	}
	data, err := segment.Current(targetPath)
	if err != nil {
		return fmt.Errorf("read navigation anchor target %s: %w", targetPath, err)
	}
	ids, err := redline.ExtractAnchorIDs(data, targetPath)
	if err != nil {
		return fmt.Errorf("parse navigation anchor target %s: %w", targetPath, err)
	}
	if !ids[parts.fragment] {
		return fmt.Errorf("navigation anchor target missing: %s#%s", targetPath, parts.fragment)
	}
	return nil
}

func sameStringSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for key := range a {
		if !b[key] {
			return false
		}
	}
	return true
}
