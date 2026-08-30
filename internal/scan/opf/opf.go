// Package opf 是 OPF / container / encryption.xml 的只读结构解析器（SPEC §1 第 4 层）。
//
// 只产出结构信息，不做任何写回 —— 修改 OPF 也必须走字节区间编辑（INV-2）。
package opf

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"slices"
	"strings"
)

// 与 scripts/epub_lib.py 保持一致的命名空间。
const (
	ContainerURI = "urn:oasis:names:tc:opendocument:xmlns:container"
	OPFURI       = "http://www.idpf.org/2007/opf"
	DCURI        = "http://purl.org/dc/elements/1.1/"
	NCXURI       = "http://www.daisy.org/z3986/2005/ncx/"
	XHTMLURI     = "http://www.w3.org/1999/xhtml"
	OPSURI       = "http://www.idpf.org/2007/ops"
	XMLURI       = "http://www.w3.org/XML/1998/namespace"
	IBOOKSPrefix = "http://vocabulary.itunes.apple.com/rdf/ibooks/vocabulary-extensions-1.0/"
)

// ContainerPath 是容器描述文件的固定路径。
const ContainerPath = "META-INF/container.xml"

// ManifestItem 是 OPF manifest 里的一个 <item>。
type ManifestItem struct {
	ID          string `json:"id"`
	Href        string `json:"href"`
	MediaType   string `json:"mediaType"`
	Properties  string `json:"properties,omitempty"`
	ArchivePath string `json:"archivePath,omitempty"` // href 解析到容器内的路径（外链为空）
	Fallback    string `json:"fallback,omitempty"`
}

// SpineItem 是 <spine> 里的一个 <itemref>。
type SpineItem struct {
	IDRef      string `json:"idref"`
	Linear     string `json:"linear,omitempty"`
	Properties string `json:"properties,omitempty"`
}

// MetaValue 是 metadata 里的 <meta> 节点。
type MetaValue struct {
	Name     string `json:"name,omitempty"`
	Property string `json:"property,omitempty"`
	Content  string `json:"content,omitempty"`
	Refines  string `json:"refines,omitempty"`
	Text     string `json:"text,omitempty"`
}

// Package 是 OPF 的只读投影。
type Package struct {
	Path           string              `json:"path"`
	Version        string              `json:"version,omitempty"`
	UniqueID       string              `json:"uniqueIdentifier,omitempty"`
	Prefix         string              `json:"prefix,omitempty"`
	Manifest       []ManifestItem      `json:"manifest"`
	Spine          []SpineItem         `json:"spine"`
	SpineToc       string              `json:"spineToc,omitempty"`
	MetadataTitles []string            `json:"metadataTitles"`
	Metadata       map[string][]string `json:"metadata"`
	Metas          []MetaValue         `json:"metas"`
}

// LocalName 返回限定名（{ns}local 或 prefix:local）的 local 部分。
func LocalName(tag string) string {
	if i := strings.LastIndexByte(tag, '}'); i >= 0 {
		return tag[i+1:]
	}
	if i := strings.LastIndexByte(tag, ':'); i >= 0 {
		return tag[i+1:]
	}
	return tag
}

// IsExternalURI 判断是否外部 URI（有 scheme 或以 / 开头）。
func IsExternalURI(uri string) bool {
	if u, err := url.Parse(uri); err == nil && u.Scheme != "" {
		return true
	}
	return strings.HasPrefix(uri, "/") || strings.HasPrefix(uri, "//")
}

// ResolveHref 把 manifest href 解析为容器内路径；外链返回 ok=false。
func ResolveHref(opfPath, href string) (string, bool) {
	if href == "" {
		return "", false
	}
	clean := href
	if i := strings.IndexByte(clean, '#'); i >= 0 {
		clean = clean[:i]
	}
	if clean == "" || IsExternalURI(clean) {
		return "", false
	}
	decoded, err := url.PathUnescape(clean)
	if err != nil {
		decoded = clean
	}
	return path.Join(path.Dir(opfPath), decoded), true
}

// FindOPFPath 从 container.xml 内容解析 OPF 路径（第一个 rootfile）。
// 与 Python 侧一致：rootfile 必须属于 container 命名空间。
func FindOPFPath(container []byte) (string, error) {
	d := newDecoder(container)
	var opfPath string
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("container.xml: XML parse failed: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok && opfPath == "" &&
			se.Name.Space == ContainerURI && se.Name.Local == "rootfile" {
			opfPath = attrValue(se, "full-path")
		}
	}
	if opfPath == "" {
		return "", errors.New("container.xml rootfile does not resolve")
	}
	return opfPath, nil
}

// Parse 解析 OPF 内容为只读投影。
func Parse(opfPath string, data []byte) (*Package, error) {
	d := newDecoder(data)
	pkg := &Package{
		Path:           opfPath,
		Manifest:       []ManifestItem{},
		Spine:          []SpineItem{},
		MetadataTitles: []string{},
		Metadata:       map[string][]string{},
		Metas:          []MetaValue{},
	}
	var parents []string // 已打开元素的 local 名栈
	dcField := ""        // 正在收集的 dc:* 字段
	var dcText strings.Builder
	curMeta := -1 // pkg.Metas 中正在收集的下标
	var metaText strings.Builder

	parent := func() string {
		if len(parents) > 0 {
			return parents[len(parents)-1]
		}
		return ""
	}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: XML parse failed: %w", opfPath, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			local := t.Name.Local
			switch {
			case t.Name.Space == DCURI || strings.HasPrefix(t.Name.Space, "http://purl.org/dc/"):
				dcField = local
				dcText.Reset()
			case local == "package":
				pkg.Version = attrValue(t, "version")
				pkg.UniqueID = attrValue(t, "unique-identifier")
				pkg.Prefix = attrValue(t, "prefix")
			case local == "spine" && parent() == "package":
				pkg.SpineToc = attrValue(t, "toc")
			case local == "item" && parent() == "manifest":
				item := ManifestItem{
					ID:         attrValue(t, "id"),
					Href:       attrValue(t, "href"),
					MediaType:  attrValue(t, "media-type"),
					Properties: attrValue(t, "properties"),
					Fallback:   attrValue(t, "fallback"),
				}
				if ap, ok := ResolveHref(opfPath, item.Href); ok {
					item.ArchivePath = ap
				}
				pkg.Manifest = append(pkg.Manifest, item)
			case local == "itemref" && parent() == "spine":
				pkg.Spine = append(pkg.Spine, SpineItem{
					IDRef:      attrValue(t, "idref"),
					Linear:     attrValue(t, "linear"),
					Properties: attrValue(t, "properties"),
				})
			case local == "meta" && parent() == "metadata":
				pkg.Metas = append(pkg.Metas, MetaValue{
					Name:     attrValue(t, "name"),
					Property: attrValue(t, "property"),
					Content:  attrValue(t, "content"),
					Refines:  attrValue(t, "refines"),
				})
				curMeta = len(pkg.Metas) - 1
				metaText.Reset()
			}
			parents = append(parents, local)
		case xml.CharData:
			if dcField != "" {
				dcText.Write(t)
			}
			if curMeta >= 0 {
				metaText.Write(t)
			}
		case xml.EndElement:
			local := t.Name.Local
			if len(parents) > 0 && parents[len(parents)-1] == local {
				parents = parents[:len(parents)-1]
			}
			switch {
			case dcField != "" && (t.Name.Space == DCURI || strings.HasPrefix(t.Name.Space, "http://purl.org/dc/")):
				pkg.Metadata[dcField] = append(pkg.Metadata[dcField], strings.TrimSpace(dcText.String()))
				if local == "title" {
					pkg.MetadataTitles = append(pkg.MetadataTitles, strings.TrimSpace(dcText.String()))
				}
				dcField = ""
			case local == "meta" && curMeta >= 0:
				pkg.Metas[curMeta].Text = strings.TrimSpace(metaText.String())
				curMeta = -1
			}
		}
	}
	return pkg, nil
}

// ItemByID 返回 manifest 中第一个 id 匹配的项。
func (p *Package) ItemByID(id string) (ManifestItem, bool) {
	for _, it := range p.Manifest {
		if it.ID == id {
			return it, true
		}
	}
	return ManifestItem{}, false
}

// ItemByHref 返回 manifest 中第一个 href 匹配的项。
func (p *Package) ItemByHref(href string) (ManifestItem, bool) {
	for _, it := range p.Manifest {
		if it.Href == href {
			return it, true
		}
	}
	return ManifestItem{}, false
}

// HasNavProps 判断 properties 分词里是否含 nav。
func HasNavProps(props string) bool {
	return slices.Contains(strings.Fields(props), "nav")
}

// NavItem 返回 properties 含 nav 的第一个项。
func (p *Package) NavItem() (ManifestItem, bool) {
	for _, it := range p.Manifest {
		if HasNavProps(it.Properties) {
			return it, true
		}
	}
	return ManifestItem{}, false
}

// CoverItem 返回 properties 含 cover-image 的第一个项。
func (p *Package) CoverItem() (ManifestItem, bool) {
	for _, it := range p.Manifest {
		if slices.Contains(strings.Fields(it.Properties), "cover-image") {
			return it, true
		}
	}
	return ManifestItem{}, false
}

// NCXItem 返回第一个 NCX 项（media-type 或 id 判定）。
func (p *Package) NCXItem() (ManifestItem, bool) {
	for _, it := range p.Manifest {
		if it.MediaType == "application/x-dtbncx+xml" || it.ID == "ncx" {
			return it, true
		}
	}
	return ManifestItem{}, false
}

// OPFDir 返回 OPF 所在目录（带尾斜杠，根目录为空串）。
func (p *Package) OPFDir() string {
	if i := strings.LastIndexByte(p.Path, '/'); i >= 0 {
		return p.Path[:i+1]
	}
	return ""
}

// EncryptionRecord 是 encryption.xml 中一个 EncryptedData 的投影。
type EncryptionRecord struct {
	Algorithm  string   `json:"algorithm,omitempty"`
	Targets    []string `json:"targets"`    // CipherReference URI 归一后的容器路径
	RawTargets []string `json:"rawTargets"` // 原始 URI
}

// ParseEncryption 解析 META-INF/encryption.xml。
func ParseEncryption(data []byte) ([]EncryptionRecord, error) {
	d := newDecoder(data)
	var records []EncryptionRecord
	inEncrypted := false
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("META-INF/encryption.xml: XML parse failed: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch LocalName(t.Name.Local) {
			case "EncryptedData":
				inEncrypted = true
				records = append(records, EncryptionRecord{Targets: []string{}, RawTargets: []string{}})
			case "EncryptionMethod":
				if inEncrypted && len(records) > 0 && records[len(records)-1].Algorithm == "" {
					records[len(records)-1].Algorithm = attrValue(t, "Algorithm")
				}
			case "CipherReference":
				if inEncrypted && len(records) > 0 {
					uri := attrValue(t, "URI")
					rec := &records[len(records)-1]
					rec.RawTargets = append(rec.RawTargets, uri)
					rec.Targets = append(rec.Targets, EncryptionTargetPath(uri))
				}
			}
		case xml.EndElement:
			if LocalName(t.Name.Local) == "EncryptedData" {
				inEncrypted = false
			}
		}
	}
	return records, nil
}

// EncryptionTargetPath 复刻 Python urlsplit(uri).path → unquote → lstrip('/') → normpath。
func EncryptionTargetPath(uri string) string {
	clean := uri
	if i := strings.IndexByte(clean, '#'); i >= 0 {
		clean = clean[:i]
	}
	if i := strings.IndexByte(clean, '?'); i >= 0 {
		clean = clean[:i]
	}
	decoded, err := url.PathUnescape(clean)
	if err != nil {
		decoded = clean
	}
	return path.Clean(strings.TrimPrefix(decoded, "/"))
}

// ---- 内部工具 ----

func newDecoder(data []byte) *xml.Decoder {
	d := xml.NewDecoder(strings.NewReader(string(data)))
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil // 输入已按 UTF-8 宽容解码，忽略声明里的 charset
	}
	d.Strict = false // 扫描层只读宽容：未定义实体不致命
	return d
}

func attrValue(t xml.StartElement, name string) string {
	for _, a := range t.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
