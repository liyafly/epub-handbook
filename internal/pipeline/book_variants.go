package pipeline

// 变体（阅读器版本与版本差异共用同一机制）。
//
// 变体 = 同一份源 + 一个覆盖层目录。覆盖层镜像 EPUB 树，只允许：
//   - 替换或新增 .css 文件；
//   - 替换 OPF（container.xml 指向的 rootfile）；
//   - 替换源树里已有的 XHTML（nav 文档除外，不能新增）；
//   - 替换或新增图片。
// 其余一切（nav、NCX、新章节、字体……）在构建前直接报错。替换的 XHTML 只能改
// 排版结构：构建对每个变体跑「正文与通用版逐字一致」的 text 红线，不一致即失败。
// 覆盖层替换了 OPF 时，新 manifest 没列的图片不进包，变体只带自己用的图。
// 不做任何阅读器识别 CSS（SPEC §5.6.1 禁止 amzn-kf8 等）；
// 只差样式时，在覆盖层里替换 Styles/reader.css 一类文件即可。

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/scan/opf"
	"github.com/liyafly/epub-handbook/internal/zipfs"
)

// legacyVariantsName 是旧布局的变体配置文件（位于制作区）。
const legacyVariantsName = "variants.json"

// variantConfig 是 book.json / variants.json 中的一个变体。
type variantConfig struct {
	Overlay string `json:"overlay"`
	Output  string `json:"output,omitempty"`
	// Fonts 非 nil 时整体替换字体任务列表（空数组表示本变体不嵌字体）。
	Fonts *[]json.RawMessage `json:"fonts,omitempty"`
}

type variantSpec struct {
	Name       string
	OverlayDir string
	Output     string
	Fonts      *[]json.RawMessage
}

// outputSpec 描述一个要构建的成品：通用版（Name 为空）或某个变体。
type outputSpec struct {
	Name    string
	File    string
	Receipt string
	Overlay string
	Fonts   *[]json.RawMessage
}

func (o outputSpec) label() string {
	if o.Name == "" {
		return "generic"
	}
	return o.Name
}

// suffix 给固定名报告加变体后缀。
func (o outputSpec) suffix() string {
	if o.Name == "" {
		return ""
	}
	return "." + o.Name
}

func validVariantName(name string) bool {
	return validOutputName(name) && !strings.HasPrefix(name, ".") && !strings.ContainsAny(name, `:*?"<>|`)
}

func buildVariantSpecs(configs map[string]variantConfig, baseDir, genericOutput string) ([]variantSpec, error) {
	names := slices.Sorted(maps.Keys(configs))
	seenOutputs := map[string]string{genericOutput: "generic"}
	specs := make([]variantSpec, 0, len(names))
	for _, name := range names {
		cfg := configs[name]
		if !validVariantName(name) {
			return nil, fmt.Errorf("variant name %q is not a safe file name", name)
		}
		if cfg.Overlay == "" || !filepath.IsLocal(filepath.FromSlash(cfg.Overlay)) {
			return nil, fmt.Errorf("variant %q: overlay must be a relative path inside the book", name)
		}
		output := cmp.Or(cfg.Output, strings.TrimSuffix(genericOutput, ".epub")+"-"+name+".epub")
		if !validOutputName(output) {
			return nil, fmt.Errorf("variant %q: output must be a file name", name)
		}
		if other, dup := seenOutputs[output]; dup {
			return nil, fmt.Errorf("variant %q: output %q is already used by %s", name, output, other)
		}
		seenOutputs[output] = name
		specs = append(specs, variantSpec{
			Name: name, OverlayDir: filepath.Join(baseDir, filepath.FromSlash(cfg.Overlay)),
			Output: output, Fonts: cfg.Fonts,
		})
	}
	return specs, nil
}

// outputs 返回通用版加全部变体（通用版在前）。
func (l bookLayout) outputs() []outputSpec {
	list := []outputSpec{{File: l.OutputName, Receipt: l.receiptPath()}}
	for _, v := range l.Variants {
		list = append(list, outputSpec{
			Name: v.Name, File: v.Output, Receipt: filepath.Join(l.StateDir, "dist-sha256."+v.Name),
			Overlay: v.OverlayDir, Fonts: v.Fonts,
		})
	}
	return list
}

func (l bookLayout) variant(name string) (outputSpec, bool) {
	for _, o := range l.outputs()[1:] {
		if o.Name == name {
			return o, true
		}
	}
	return outputSpec{}, false
}

// treeRootfile 读取源树 META-INF/container.xml 的第一个 rootfile 路径（斜杠形式）。
func treeRootfile(epubDir string) (string, error) {
	data, err := readSmallFile(filepath.Join(epubDir, "META-INF", "container.xml"))
	if err != nil {
		return "", err
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", errors.New("container.xml has no rootfile")
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "rootfile" {
			for _, attr := range start.Attr {
				if attr.Name.Local == "full-path" {
					return attr.Value, nil
				}
			}
			return "", errors.New("container.xml rootfile has no full-path")
		}
	}
}

// isImagePath 判断覆盖层可替换或新增、以及变体按 manifest 裁剪的图片扩展名。
func isImagePath(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		return true
	}
	return false
}

func isXHTMLPath(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".xhtml", ".html", ".htm":
		return true
	}
	return false
}

// baseTree 是校验覆盖层用的源树事实。
type baseTree struct {
	dir     string
	opfPath string
	navPath string
}

func loadBaseTree(epubDir string) (baseTree, error) {
	opfPath, err := treeRootfile(epubDir)
	if err != nil {
		return baseTree{}, err
	}
	tree := baseTree{dir: epubDir, opfPath: opfPath}
	data, err := readSmallFile(filepath.Join(epubDir, filepath.FromSlash(opfPath)))
	if err != nil {
		return baseTree{}, err
	}
	pkg, err := opf.Parse(opfPath, data)
	if err != nil {
		return baseTree{}, fmt.Errorf("parse %s: %w", opfPath, err)
	}
	if nav, ok := pkg.NavItem(); ok {
		tree.navPath = nav.ArchivePath
	}
	return tree, nil
}

// overlayFiles 校验覆盖层并返回其中的文件（相对路径，斜杠形式）。
// 构建前调用：规则外的文件都是硬错误。
func overlayFiles(overlayDir string, base baseTree) ([]string, error) {
	info, err := os.Stat(overlayDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("overlay directory not found: %s", overlayDir)
	}
	var files, rejected []string
	err = filepath.WalkDir(overlayDir, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.Name() != "" && strings.HasPrefix(d.Name(), ".") && p != overlayDir {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil // .DS_Store 之类的杂物既不复制也不算违规
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(overlayDir, p)
		if rerr != nil {
			return rerr
		}
		slash := filepath.ToSlash(rel)
		if !d.Type().IsRegular() {
			rejected = append(rejected, slash+" (not a regular file)")
			return nil
		}
		switch {
		case strings.EqualFold(filepath.Ext(slash), ".css"), slash == base.opfPath, isImagePath(slash):
			files = append(files, slash)
		case isXHTMLPath(slash):
			if slash == base.navPath {
				rejected = append(rejected, slash+" (nav document)")
			} else if st, serr := os.Stat(filepath.Join(base.dir, rel)); serr != nil || !st.Mode().IsRegular() {
				rejected = append(rejected, slash+" (new XHTML; only existing ones can be replaced)")
			} else {
				files = append(files, slash)
			}
		default:
			rejected = append(rejected, slash)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(rejected) > 0 {
		slices.Sort(rejected)
		return nil, fmt.Errorf("overlay %s may only add or replace .css files and images, replace the OPF (%s) and replace existing XHTML other than nav; not allowed: %s",
			overlayDir, base.opfPath, strings.Join(rejected, ", "))
	}
	slices.Sort(files)
	return files, nil
}

// pruneUnlistedImages 删去源树副本里 OPF manifest 没列的图片，返回删去的路径。
// 只在覆盖层替换了 OPF 时调用：那份 OPF 决定变体带哪些图。
func pruneUnlistedImages(ctx context.Context, srcDir, opfPath string) ([]string, error) {
	data, err := readSmallFile(filepath.Join(srcDir, filepath.FromSlash(opfPath)))
	if err != nil {
		return nil, err
	}
	pkg, err := opf.Parse(opfPath, data)
	if err != nil {
		// 解析不了就不裁剪：导航审计会把这份 OPF 的问题逐条报出来并阻断构建。
		return nil, nil
	}
	listed := make(map[string]bool, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		if item.ArchivePath != "" {
			listed[item.ArchivePath] = true
		}
	}
	var removed []string
	err = filepath.WalkDir(srcDir, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() || !isImagePath(d.Name()) {
			return nil
		}
		rel, rerr := filepath.Rel(srcDir, p)
		if rerr != nil {
			return rerr
		}
		if slash := filepath.ToSlash(rel); !listed[slash] {
			removed = append(removed, slash)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, rel := range removed {
		if err := zipfs.Remove(filepath.Join(srcDir, filepath.FromSlash(rel))); err != nil {
			return nil, err
		}
	}
	return removed, nil
}

// applyOverlay 把覆盖层文件写进（已复制的）源树副本。
func applyOverlay(overlayDir string, files []string, srcDir string) error {
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(overlayDir, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		if err := zipfs.WriteFileAtomic(filepath.Join(srcDir, filepath.FromSlash(rel)), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// validateOverlays 在构建前校验给定输出的覆盖层。
func validateOverlays(layout bookLayout, targets []outputSpec) (map[string][]string, error) {
	result := map[string][]string{}
	var base baseTree
	for _, out := range targets {
		if out.Overlay == "" {
			continue
		}
		if base.opfPath == "" {
			var err error
			if base, err = loadBaseTree(layout.EpubDir); err != nil {
				return nil, err
			}
		}
		files, err := overlayFiles(out.Overlay, base)
		if err != nil {
			return nil, fmt.Errorf("variant %q: %w", out.Name, err)
		}
		result[out.Name] = files
	}
	return result, nil
}
