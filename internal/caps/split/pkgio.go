// pkgio.go 复刻 package_io 的 read_package / ensure_no_encryption 与
// split.content_spine_paths。解析走 scan/opf 的区间树（只读）。
package split

import (
	"context"
	"fmt"
	"strings"

	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// OPF 命名空间（与 scripts/epub_lib.py 一致）。
const opfURI = opf.OPFURI

type manifestItem struct {
	itemID      string
	href        string
	mediaType   string
	properties  string
	archivePath string
}

type spineRef struct {
	idref      string
	linear     string
	properties string
}

type pkgInfo struct {
	opfPath  string
	title    string
	manifest []manifestItem
	spine    []spineRef
	tocID    string
	byIDMap  map[string]manifestItem
	byPath   map[string]manifestItem
}

func (p *pkgInfo) byID(id string) (manifestItem, bool) {
	it, ok := p.byIDMap[id]
	return it, ok
}

// ensureNoEncryption 复刻 package_io.ensure_no_encryption（措辞逐字对齐）。
func ensureNoEncryption(names []string, action string) error {
	for _, name := range names {
		if strings.EqualFold(name, "meta-inf/encryption.xml") {
			return toolErrf("%s: encrypted EPUB resources detected; refusing package rewrite", action)
		}
	}
	return nil
}

// readPackage 复刻 package_io.read_package（含 core.package_title 投影）。
func readPackage(names map[string]bool, read func(string) ([]byte, error)) (*pkgInfo, error) {
	const containerPath = "META-INF/container.xml"
	if !names[containerPath] {
		return nil, toolErrf("missing META-INF/container.xml")
	}
	containerData, err := read(containerPath)
	if err != nil {
		return nil, toolErrf("%v", err)
	}
	container, err := opf.ScanSpanTree(containerData)
	if err != nil {
		return nil, toolErrf("%s: XML parse failed: %v", containerPath, err)
	}
	opfPath := ""
	for _, e := range container.Walk() {
		if e.Name.Local == "rootfile" {
			opfPath, _ = e.AttrByLocal("", "full-path")
			break
		}
	}
	if opfPath == "" {
		return nil, toolErrf("container.xml has no rootfile full-path")
	}
	opfPath, err = validateArchivePath(opfPath, "container.xml rootfile")
	if err != nil {
		return nil, err
	}
	if !names[opfPath] {
		return nil, toolErrf("container.xml rootfile does not resolve: %s", opfPath)
	}
	opfData, err := read(opfPath)
	if err != nil {
		return nil, toolErrf("%v", err)
	}
	root, err := opf.ScanSpanTree(opfData)
	if err != nil {
		return nil, toolErrf("%s: XML parse failed: %v", opfPath, err)
	}
	manifestNode := root.ChildByAnyNS("manifest")
	if manifestNode == nil {
		return nil, toolErrf("%s: OPF missing manifest", opfPath)
	}
	spineNode := root.ChildByAnyNS("spine")
	if spineNode == nil {
		return nil, toolErrf("%s: OPF missing spine", opfPath)
	}

	pkg := &pkgInfo{
		opfPath: opfPath,
		byIDMap: map[string]manifestItem{},
		byPath:  map[string]manifestItem{},
	}
	if titleNode := root.ChildByLocal(opfURI, "metadata"); titleNode != nil {
		if t := titleNode.ChildByLocal(opf.DCURI, "title"); t != nil {
			pkg.title = strings.TrimSpace(t.IterText())
		} else {
			pkg.title = "Untitled"
		}
	}
	for _, item := range manifestNode.Kids {
		if item.Name.Local != "item" {
			continue
		}
		itemID, _ := item.AttrByLocal("", "id")
		href, _ := item.AttrByLocal("", "href")
		mediaType, hasMedia := item.AttrByLocal("", "media-type")
		if !hasMedia {
			mediaType = "application/octet-stream"
		}
		properties, _ := item.AttrByLocal("", "properties")
		if itemID == "" || href == "" {
			return nil, toolErrf("%s: manifest item missing id or href", opfPath)
		}
		if pyIsExternalURI(href) {
			continue
		}
		archivePath, err := resolveRelativePath(opfPath, pyURLSplit(href).path)
		if err != nil {
			return nil, err
		}
		mi := manifestItem{itemID: itemID, href: href, mediaType: mediaType, properties: properties, archivePath: archivePath}
		pkg.manifest = append(pkg.manifest, mi)
		pkg.byIDMap[itemID] = mi
		pkg.byPath[archivePath] = mi // Python dict comprehension：后者覆盖
	}
	for _, ref := range spineNode.Kids {
		if ref.Name.Local != "itemref" {
			continue
		}
		idref, _ := ref.AttrByLocal("", "idref")
		if idref == "" {
			continue
		}
		linear, _ := ref.AttrByLocal("", "linear")
		properties, _ := ref.AttrByLocal("", "properties")
		pkg.spine = append(pkg.spine, spineRef{idref: idref, linear: linear, properties: properties})
	}
	pkg.tocID, _ = spineNode.AttrByLocal("", "toc")
	return pkg, nil
}

// contentSpinePaths 复刻 split.content_spine_paths。
func contentSpinePaths(pkg *pkgInfo) []string {
	var paths []string
	for _, sp := range pkg.spine {
		item, ok := pkg.byID(sp.idref)
		if !ok || hasNavProp(item.properties) {
			continue
		}
		lower := strings.ToLower(item.archivePath)
		if item.mediaType == "application/xhtml+xml" ||
			strings.HasSuffix(lower, ".xhtml") || strings.HasSuffix(lower, ".html") {
			paths = append(paths, item.archivePath)
		}
	}
	return paths
}

// collectReferencedResources 复刻 split.collect_referenced_resources 的
// BFS（栈式 pop；与 Python 的 set 语义一致，结果与遍历序无关）。
func collectReferencedResources(names map[string]bool, read func(string) ([]byte, error), pkg *pkgInfo, contentPaths map[string]bool) map[string]bool {
	resources, _ := collectReferencedResourcesContext(nil, names, read, pkg, contentPaths)
	return resources
}

func collectReferencedResourcesContext(ctx context.Context, names map[string]bool, read func(string) ([]byte, error), pkg *pkgInfo, contentPaths map[string]bool) (map[string]bool, error) {
	if read == nil {
		return nil, fmt.Errorf("resource closure: nil reader")
	}
	if pkg == nil {
		return nil, fmt.Errorf("resource closure: nil package")
	}
	referenced := map[string]bool{}
	scanned := map[string]bool{}
	var queue []string
	// The cover image is package-level metadata, not necessarily referenced by
	// every selected XHTML. Keep it in every segment so cover redline semantics
	// remain valid after partitioning.
	for _, item := range pkg.manifest {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if !hasProp(item.properties, "cover-image") || contentPaths[item.archivePath] {
			continue
		}
		if !names[item.archivePath] {
			return nil, fmt.Errorf("resource closure: manifest target missing from source: %s", item.archivePath)
		}
		if names[item.archivePath] {
			referenced[item.archivePath] = true
			queue = append(queue, item.archivePath)
		}
	}
	for p := range contentPaths {
		if !names[p] {
			return nil, fmt.Errorf("resource closure: selected content missing from source: %s", p)
		}
		queue = append(queue, p)
	}
	for len(queue) > 0 {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		current := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if scanned[current] || !names[current] {
			if !names[current] {
				return nil, fmt.Errorf("resource closure: queued resource missing from source: %s", current)
			}
			continue
		}
		scanned[current] = true
		ext := strings.ToLower(pathExt(current))
		if ext != ".css" && !markupExtensions[ext] {
			continue
		}
		data, err := read(current)
		if err != nil {
			return nil, fmt.Errorf("resource closure: read %s: %w", current, err)
		}
		if !utf8Valid(data) {
			return nil, fmt.Errorf("resource closure: %s is not valid UTF-8", current)
		}
		var rawURIs []string
		if ext == ".css" {
			rawURIs, err = collectCSSURIsStrict(string(data))
		} else {
			rawURIs, err = collectRawURIsStrict(string(data))
		}
		if err != nil {
			return nil, fmt.Errorf("resource closure: parse references in %s: %w", current, err)
		}
		for _, raw := range rawURIs {
			if ctx != nil {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			if raw == "" || strings.HasPrefix(raw, "#") || pyIsExternalURI(raw) {
				continue
			}
			parts := pyURLSplit(raw)
			if parts.path == "" {
				continue
			}
			target, terr := resolveRelativePath(current, parts.path)
			if terr != nil {
				return nil, fmt.Errorf("resource closure: resolve %q from %s: %w", raw, current, terr)
			}
			if !names[target] {
				return nil, fmt.Errorf("resource closure: referenced target missing from source: %s (from %s)", target, current)
			}
			if !contentPaths[target] && !referenced[target] {
				referenced[target] = true
				queue = append(queue, target)
			}
		}
	}
	return referenced, nil
}

func hasProp(value, wanted string) bool {
	return strings.Contains(" "+strings.Join(strings.Fields(value), " ")+" ", " "+wanted+" ")
}
