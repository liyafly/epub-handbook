package navaudit

import (
	"context"
	"strings"

	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// checkNCXUID 核对 NCX 的 dtb:uid 与 OPF 的唯一标识符（unique-identifier 指向的
// dc:identifier）一致；不一致是 EPUBCheck NCX-001 error，常见于只改了 OPF 标识符。
func (ins *inspector) checkNCXUID(ctx context.Context, pkg *opf.Package) {
	ncx, ok := pkg.NCXItem()
	if !ok || pkg.UniqueID == "" {
		return
	}
	raw, err := ins.b.CurrentContext(ctx, ins.opfPath)
	if err != nil {
		return
	}
	tree, err := opf.ScanSpanTreeContext(ctx, raw)
	if err != nil {
		return
	}
	want, found := "", false
	for _, n := range tree.Walk() {
		if id, _ := n.AttrByLocal("", "id"); n.Name.Local == "identifier" && id == pkg.UniqueID {
			want, found = strings.TrimSpace(n.IterText()), true
			break
		}
	}
	if !found {
		return
	}
	data, err := ins.b.CurrentContext(ctx, ncx.ArchivePath)
	if err != nil {
		return
	}
	ncxTree, err := opf.ScanSpanTreeContext(ctx, data)
	if err != nil {
		return
	}
	for _, n := range ncxTree.Walk() {
		if name, _ := n.AttrByLocal("", "name"); n.Name.Local != "meta" || name != "dtb:uid" {
			continue
		}
		if got, _ := n.AttrByLocal("", "content"); strings.TrimSpace(got) != want {
			ins.addFinding("error", "NCX dtb:uid \""+got+"\" does not match the OPF identifier \""+want+"\" (EPUBCheck NCX-001)", ncx.ArchivePath, "ncx-uid-mismatch")
		}
		return
	}
}
