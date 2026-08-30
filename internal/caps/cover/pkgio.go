// pkgio.go 复刻 package_io 的 read_package / ensure_no_encryption，
// 并返回区间树上的 manifest / metadata 节点（字节编辑定位用）。
package cover

import (
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
	manifest []manifestItem
	spine    []spineRef
	tocID    string
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

// readPackage 复刻 package_io.read_package，同时给出区间树定位。
func readPackage(names map[string]bool, read func(string) ([]byte, error)) (*pkgInfo, *opf.SpanNode, *opf.SpanNode, *opf.SpanNode, error) {
	const containerPath = "META-INF/container.xml"
	if !names[containerPath] {
		return nil, nil, nil, nil, toolErrf("missing META-INF/container.xml")
	}
	containerData, err := read(containerPath)
	if err != nil {
		return nil, nil, nil, nil, toolErrf("%v", err)
	}
	container, err := opf.ScanSpanTree(containerData)
	if err != nil {
		return nil, nil, nil, nil, toolErrf("%s: XML parse failed: %v", containerPath, err)
	}
	opfPath := ""
	for _, e := range container.Walk() {
		if e.Name.Local == "rootfile" {
			opfPath, _ = e.AttrByLocal("", "full-path")
			break
		}
	}
	if opfPath == "" {
		return nil, nil, nil, nil, toolErrf("container.xml has no rootfile full-path")
	}
	opfPath, err = validateArchivePath(opfPath, "container.xml rootfile")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if !names[opfPath] {
		return nil, nil, nil, nil, toolErrf("container.xml rootfile does not resolve: %s", opfPath)
	}
	opfData, err := read(opfPath)
	if err != nil {
		return nil, nil, nil, nil, toolErrf("%v", err)
	}
	root, err := opf.ScanSpanTree(opfData)
	if err != nil {
		return nil, nil, nil, nil, toolErrf("%s: XML parse failed: %v", opfPath, err)
	}
	manifestNode := root.ChildByAnyNS("manifest")
	if manifestNode == nil {
		return nil, nil, nil, nil, toolErrf("%s: OPF missing manifest", opfPath)
	}
	spineNode := root.ChildByAnyNS("spine")
	if spineNode == nil {
		return nil, nil, nil, nil, toolErrf("%s: OPF missing spine", opfPath)
	}

	pkg := &pkgInfo{opfPath: opfPath}
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
			return nil, nil, nil, nil, toolErrf("%s: manifest item missing id or href", opfPath)
		}
		if pyIsExternalURI(href) {
			continue
		}
		archivePath, err := resolveRelativePath(opfPath, pyURLSplit(href).path)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		pkg.manifest = append(pkg.manifest, manifestItem{
			itemID: itemID, href: href, mediaType: mediaType, properties: properties, archivePath: archivePath,
		})
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
	return pkg, root, manifestNode, root.ChildByLocal(opfURI, "metadata"), nil
}
