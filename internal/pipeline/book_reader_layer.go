package pipeline

// 阅读器覆盖层入口 reader.css：`epub book new` 的新书自带；导入的书由 import 自动加，
// 已有的书用 `epub book reader-layer` 补。变体覆盖层只需替换这一个文件，
// 不必整份复制书里已有的样式表。
//
// 只做字节区间插入：OPF 在最后一个 manifest item 后加一项，每个 spine XHTML 在 </head>
// 前加一个 link；不重序列化任何文档。任何一个文件无法安全编辑就整体放弃，不留半套改动。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/scan/opf"
	"github.com/liyafly/epub-handbook/internal/zipfs"
)

const readerLayerCSS = "@charset \"utf-8\";\n/* 阅读器覆盖层入口：默认为空。变体覆盖层（variants/<名称>/）只需替换本文件。\n * 不要在这里写阅读器识别查询（SPEC §5.6.1 禁止 amzn-kf8 / amzn-mobi）。 */\n"

// BookReaderLayerOptions 是 epub book reader-layer 的输入。
type BookReaderLayerOptions struct {
	Dir string
}

// BookReaderLayer 给已有的书补上 reader.css，返回退出码。
func BookReaderLayer(ctx context.Context, opts BookReaderLayerOptions, bio BookIO) int {
	layout, err := resolveBookLayout(opts.Dir, readBookEnv())
	if err != nil {
		return bookExit(bio, err)
	}
	if len(layout.Volumes) > 0 {
		return bookExit(bio, usageErrorf("multi-volume workspace: run reader-layer inside each volume directory"))
	}
	added, err := addSourceReaderLayer(ctx, layout.EpubDir)
	if errors.Is(err, errReaderLayerExists) {
		return bookExit(bio, usageErrorf("%v; nothing to add", err))
	}
	if err != nil {
		return bookExit(bio, err)
	}
	fmt.Fprintf(bio.out(), "Added %s (linked last in the OPF and %d XHTML pages). Overlays can now replace just this file.\n",
		layout.displayPath(filepath.Join(layout.EpubDir, filepath.FromSlash(added.cssPath))), added.pages)
	return ExitOK
}

// errReaderLayerExists 表示书里已有 reader.css；import 静默跳过，reader-layer 当用法错误报告。
var errReaderLayerExists = errors.New("reader layer already exists")

type readerLayerResult struct {
	cssPath string // 容器内路径，如 OEBPS/Styles/reader.css
	pages   int
}

// addSourceReaderLayer 在解包源 epubDir 里加 reader.css。全部编辑先在内存里算好，
// 都能安全应用才写盘。
func addSourceReaderLayer(ctx context.Context, epubDir string) (readerLayerResult, error) {
	read := func(archivePath string) ([]byte, error) {
		return readSmallFile(filepath.Join(epubDir, filepath.FromSlash(archivePath)))
	}
	container, err := read(opf.ContainerPath)
	if err != nil {
		return readerLayerResult{}, err
	}
	opfPath, err := opf.FindOPFPath(container)
	if err != nil {
		return readerLayerResult{}, err
	}
	opfData, err := read(opfPath)
	if err != nil {
		return readerLayerResult{}, err
	}
	pkg, err := opf.Parse(opfPath, opfData)
	if err != nil {
		return readerLayerResult{}, err
	}
	cssPath := readerLayerPath(pkg)
	inManifest := slices.ContainsFunc(pkg.Manifest, func(item opf.ManifestItem) bool { return item.ArchivePath == cssPath })
	if inManifest || isFile(filepath.Join(epubDir, filepath.FromSlash(cssPath))) {
		return readerLayerResult{}, fmt.Errorf("%w: %s", errReaderLayerExists, cssPath)
	}
	opfEdit, err := readerManifestEdit(ctx, opfPath, opfData, pkg, cssPath)
	if err != nil {
		return readerLayerResult{}, err
	}
	changed := map[string][]byte{}
	if changed[opfPath], err = editset.Apply(opfPath, opfData, []editset.Edit{opfEdit}); err != nil {
		return readerLayerResult{}, err
	}
	pages := opf.SpineXHTMLPaths(pkg)
	if len(pages) == 0 {
		return readerLayerResult{}, errors.New("reader layer: the spine has no XHTML pages")
	}
	for _, page := range pages {
		if err := ctx.Err(); err != nil {
			return readerLayerResult{}, err
		}
		data, err := read(page)
		if err != nil {
			return readerLayerResult{}, err
		}
		edit, err := readerLinkEdit(ctx, page, data, cssPath)
		if err != nil {
			return readerLayerResult{}, err
		}
		if changed[page], err = editset.Apply(page, data, []editset.Edit{edit}); err != nil {
			return readerLayerResult{}, err
		}
	}
	cssFile := filepath.Join(epubDir, filepath.FromSlash(cssPath))
	if err := zipfs.MkdirAll(filepath.Dir(cssFile)); err != nil {
		return readerLayerResult{}, err
	}
	if err := zipfs.WriteFileAtomic(cssFile, []byte(readerLayerCSS), 0o644); err != nil {
		return readerLayerResult{}, err
	}
	for archivePath, data := range changed {
		target := filepath.Join(epubDir, filepath.FromSlash(archivePath))
		mode := os.FileMode(0o644)
		if info, err := os.Stat(target); err == nil {
			mode = info.Mode().Perm()
		}
		if err := zipfs.WriteFileAtomic(target, data, mode); err != nil {
			return readerLayerResult{}, err
		}
	}
	return readerLayerResult{cssPath: cssPath, pages: len(pages)}, nil
}

// readerLayerPath 把 reader.css 放在书里第一份样式表的目录；没有样式表时放 OPF 旁的 Styles/。
func readerLayerPath(pkg *opf.Package) string {
	for _, item := range pkg.Manifest {
		if item.MediaType == "text/css" && item.ArchivePath != "" {
			return path.Join(path.Dir(item.ArchivePath), "reader.css")
		}
	}
	return path.Join(pkg.OPFDir(), "Styles", "reader.css")
}

// readerManifestEdit 在最后一个 manifest item 之后插入 reader.css，沿用该 item 的缩进。
func readerManifestEdit(ctx context.Context, opfPath string, data []byte, pkg *opf.Package, cssPath string) (editset.Edit, error) {
	if err := opf.EditableUTF8(data); err != nil {
		return editset.Edit{}, fmt.Errorf("%s: %w", opfPath, err)
	}
	root, err := opf.ScanSpanTreeContext(ctx, data)
	if err != nil {
		return editset.Edit{}, fmt.Errorf("%s: %w", opfPath, err)
	}
	items := opf.ManifestNodes(root)
	if len(items) == 0 {
		return editset.Edit{}, fmt.Errorf("%s: manifest has no items", opfPath)
	}
	id := "css-reader"
	for n := 2; ; n++ {
		if _, taken := pkg.ItemByID(id); !taken {
			break
		}
		id = fmt.Sprintf("css-reader-%d", n)
	}
	last := items[len(items)-1]
	indent := lineIndent(data, last.Open.Start)
	fragment := "\n" + indent + opf.BuildCSSItem(id, pypath.RelativeURI(opfPath, cssPath))
	return editset.Insert(opfPath, int64(last.End()), []byte(fragment)), nil
}

// readerLinkEdit 在 </head> 前插入 reader.css 的 link，使它排在本页所有样式之后。
func readerLinkEdit(ctx context.Context, page string, data []byte, cssPath string) (editset.Edit, error) {
	if err := opf.EditableUTF8(data); err != nil {
		return editset.Edit{}, fmt.Errorf("%s: %w", page, err)
	}
	root, err := opf.ScanXHTMLSpanTreeContext(ctx, data)
	if err != nil {
		return editset.Edit{}, fmt.Errorf("%s: %w", page, err)
	}
	var head *opf.SpanNode
	for _, child := range root.Kids {
		if child.Name.Local == "head" {
			head = child
			break
		}
	}
	if head == nil || head.Close.IsZero() {
		return editset.Edit{}, fmt.Errorf("%s: no closed <head>; reader layer not added", page)
	}
	indent := "  "
	for _, kid := range head.Kids {
		indent = lineIndent(data, kid.Open.Start)
	}
	link := indent + `<link rel="stylesheet" type="text/css" href="` + pypath.EscapeAttribute(pypath.RelativeURI(page, cssPath)) + `"/>` + "\n"
	insertAt := head.Close.Start
	// </head> 独占一行时插在该行行首，保持原有缩进。
	if lineStart := strings.LastIndexByte(string(data[:insertAt]), '\n') + 1; strings.TrimSpace(string(data[lineStart:insertAt])) == "" {
		insertAt = lineStart
	} else {
		link = "\n" + strings.TrimSuffix(link, "\n")
	}
	return editset.Insert(page, int64(insertAt), []byte(link)), nil
}

// lineIndent 返回 offset 所在行行首的空白。
func lineIndent(data []byte, offset int) string {
	start := strings.LastIndexByte(string(data[:offset]), '\n') + 1
	prefix := string(data[start:offset])
	if strings.TrimLeft(prefix, " \t") != "" {
		return ""
	}
	return prefix
}
