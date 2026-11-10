package opf

import (
	"slices"
	"strings"
)

// GlobalFixedLayout 报告 OPF 全局 rendition:layout 是否为 pre-paginated。
func (p *Package) GlobalFixedLayout() bool {
	for _, m := range p.Metas {
		if m.Property == "rendition:layout" && m.Refines == "" {
			return strings.TrimSpace(m.Text) == "pre-paginated"
		}
	}
	return false
}

// IsFixedLayout 报告一个 spine 项是否按固定版式排版：itemref 的
// rendition:layout-pre-paginated / rendition:layout-reflowable 覆盖全局 meta。
func (p *Package) IsFixedLayout(ref SpineItem) bool {
	props := strings.Fields(ref.Properties)
	switch {
	case slices.Contains(props, "rendition:layout-pre-paginated"):
		return true
	case slices.Contains(props, "rendition:layout-reflowable"):
		return false
	}
	return p.GlobalFixedLayout()
}

// FixedLayoutXHTML 返回按固定版式排版的 spine XHTML 容器内路径集合。
func FixedLayoutXHTML(p *Package) map[string]bool {
	byID := make(map[string]ManifestItem, len(p.Manifest))
	for _, item := range p.Manifest {
		byID[item.ID] = item
	}
	out := map[string]bool{}
	for _, ref := range p.Spine {
		item, ok := byID[ref.IDRef]
		if !ok || item.ArchivePath == "" || (item.MediaType != "application/xhtml+xml" && item.MediaType != "text/html") {
			continue
		}
		if p.IsFixedLayout(ref) {
			out[item.ArchivePath] = true
		}
	}
	return out
}
