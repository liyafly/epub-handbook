package pipeline

// 变体（阅读器版本与版本差异共用同一机制）。
//
// 变体 = 同一份源 + 一个覆盖层目录。覆盖层镜像 EPUB 树，只允许：
//   - 替换或新增 .css 文件；
//   - 替换 OPF（container.xml 指向的 rootfile）。
// 其余一切（XHTML、nav、NCX、图片、字体……）在构建前直接报错，因此各版本的
// 正文必然一致。不做任何阅读器识别 CSS（SPEC §5.6.1 禁止 amzn-kf8 等）；
// 想让某个阅读器看到不同样式，就在覆盖层里替换 Styles/reader.css 一类文件。

import (
	"bytes"
	"cmp"
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

// overlayFiles 校验覆盖层并返回其中的文件（相对路径，斜杠形式）。
// 构建前调用：任何非 CSS、非 OPF 的文件都是硬错误。
func overlayFiles(overlayDir, opfPath string) ([]string, error) {
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
		if strings.EqualFold(filepath.Ext(slash), ".css") || slash == opfPath {
			files = append(files, slash)
			return nil
		}
		rejected = append(rejected, slash)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(rejected) > 0 {
		slices.Sort(rejected)
		return nil, fmt.Errorf("overlay %s may only replace or add .css files and the OPF (%s); not allowed: %s",
			overlayDir, opfPath, strings.Join(rejected, ", "))
	}
	slices.Sort(files)
	return files, nil
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
	var opf string
	for _, out := range targets {
		if out.Overlay == "" {
			continue
		}
		if opf == "" {
			var err error
			if opf, err = treeRootfile(layout.EpubDir); err != nil {
				return nil, err
			}
		}
		files, err := overlayFiles(out.Overlay, opf)
		if err != nil {
			return nil, fmt.Errorf("variant %q: %w", out.Name, err)
		}
		result[out.Name] = files
	}
	return result, nil
}
