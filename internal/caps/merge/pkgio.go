// pkgio.go 复刻 package_io 的 read_package / ensure_no_encryption，以及
// core 的 package_title / metadata_node 读取投影。解析走 scan/opf 的
// 区间树（只读），命名空间语义与 ElementTree 一致。
package merge

import (
	"strings"

	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// OPF / DC 命名空间（与 scripts/epub_lib.py 一致）。
const (
	opfURI = opf.OPFURI
	dcURI  = opf.DCURI
)

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
	meta     *metaExtract
	manifest []manifestItem
	spine    []spineRef
	tocID    string
	byIDMap  map[string]manifestItem
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

// readPackage 复刻 package_io.read_package，并附带 core.package_title 与
// build_opf 需要的第一卷 metadata 投影。
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

	pkg := &pkgInfo{opfPath: opfPath, byIDMap: map[string]manifestItem{}}

	// metadata 投影（title 与 build_opf 需要的四个字段）。
	if metaNode := root.ChildByLocal(opfURI, "metadata"); metaNode != nil {
		if t := metaNode.ChildByLocal(dcURI, "title"); t != nil {
			pkg.title = strings.TrimSpace(t.IterText())
		} else {
			pkg.title = "Untitled"
		}
		m := &metaExtract{}
		for _, field := range []struct{ tag, target string }{
			{"creator", "creator"}, {"publisher", "publisher"},
			{"description", "description"}, {"rights", "rights"},
		} {
			src := metaNode.ChildByLocal(dcURI, field.tag)
			if src != nil {
				text := strings.TrimSpace(src.IterText())
				if text != "" {
					switch field.target {
					case "creator":
						m.creator = text
					case "publisher":
						m.publisher = text
					case "description":
						m.description = text
					case "rights":
						m.rights = text
					}
				}
			}
		}
		pkg.meta = m
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
