package pipeline

// epub book new：从内置 book-starter 建立新布局的书级工作区。
//
// 由已删除的 templates/book-starter/new-book.sh 移植而来：preset 只是起点（把除 fonts.css /
// notes.css 以外的各层追加进 design.css；notes.css 单独成层并接入 OPF 与各章
// link），复制后归本书自由修改。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/zipfs"
)

// BookNewOptions 是 epub book new 的输入。
type BookNewOptions struct {
	Dir    string
	Preset string
	// Git 为 true 时在书根初始化独立 Git（可选，默认不建）。
	Git bool
}

// BookNew 建立新书工作区，返回退出码。
func BookNew(ctx context.Context, opts BookNewOptions, bio BookIO) int {
	notes, err := bookNew(ctx, opts, bio)
	if err != nil {
		return bookExit(bio, err)
	}
	dest, _ := filepath.Abs(opts.Dir)
	out := bio.out()
	fmt.Fprintf(out, "Created book workspace: %s\n", dest)
	fmt.Fprintf(out, "Edit: %s\n", filepath.Join(dest, "epub", "OEBPS")+string(filepath.Separator))
	fmt.Fprintf(out, "Build: epub book build \"%s\"\n", dest)
	if opts.Git {
		fmt.Fprintf(out, "Git:   git -C \"%s\" status --short\n", dest)
	}
	if opts.Preset != "" {
		fmt.Fprintf(out, "Preset %s copied into design.css (fonts.css skipped; configure fonts via book.json). Record it in your notes.\n", opts.Preset)
		if notes {
			fmt.Fprintln(out, "Preset notes.css copied as its own layer (Styles/notes.css, linked after base.css).")
		}
	}
	return ExitOK
}

func validPresetName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func bookNew(ctx context.Context, opts BookNewOptions, bio BookIO) (presetNotes bool, err error) {
	if strings.TrimSpace(opts.Dir) == "" {
		return false, usageErrorf("usage: epub book new <book-directory> [--from-preset <name>] [--git]")
	}
	root, err := FindRepoRoot()
	if err != nil {
		return false, err
	}
	var presetDir string
	if opts.Preset != "" {
		if !validPresetName(opts.Preset) {
			return false, usageErrorf("Invalid preset name: %s", opts.Preset)
		}
		presetDir = path.Join("templates", "style-presets", opts.Preset)
		if _, err := readRepositoryFile(root, path.Join(presetDir, "preset.json")); err != nil {
			return false, usageErrorf("Unknown preset: %s (see templates/style-presets/)", opts.Preset)
		}
	}
	dest, err := filepath.Abs(opts.Dir)
	if err != nil {
		return false, err
	}
	if _, err := os.Lstat(dest); err == nil {
		return false, fmt.Errorf("Book directory already exists: %s", dest)
	}
	name := filepath.Base(dest)
	if name == "" || name == "." || name == ".." || name == string(filepath.Separator) || dest == filepath.VolumeName(dest)+string(filepath.Separator) {
		return false, usageErrorf("Choose a new book directory, not its parent or the filesystem root.")
	}
	parent := filepath.Dir(dest)
	if err := zipfs.MkdirAll(parent); err != nil {
		return false, err
	}
	tmp, err := zipfs.MkdirTemp(parent, ".book-starter.*")
	if err != nil {
		return false, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = zipfs.RemoveAll(tmp)
		}
	}()
	_ = zipfs.Chmod(tmp, 0o755)

	epubDir := filepath.Join(tmp, "epub")
	if err := writeStarterTree(root, epubDir); err != nil {
		return false, err
	}
	if opts.Preset != "" {
		presetNotes, err = applyPreset(ctx, root, opts.Preset, presetDir, epubDir)
		if err != nil {
			return false, err
		}
	}
	if err := addReaderLayer(epubDir); err != nil {
		return false, err
	}
	cfg := bookConfig{Version: 1, Title: name, Output: defaultOutputName, Fonts: []json.RawMessage{}}
	if err := writeBookConfig(filepath.Join(tmp, bookConfigName), cfg); err != nil {
		return false, err
	}
	if opts.Git {
		if err := initBookGit(ctx, tmp, nil, true); err != nil {
			return false, err
		}
	}
	if err := zipfs.RenameNoReplace(tmp, dest); err != nil {
		return false, fmt.Errorf("Book directory already exists or cannot be created: %w", err)
	}
	committed = true
	return presetNotes, nil
}

func writeBookConfig(p string, cfg bookConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return zipfs.WriteFileAtomic(p, append(data, '\n'), 0o644)
}

// writeStarterTree 把内置 book-starter 的 mimetype / META-INF / OEBPS 写入 epubDir。
func writeStarterTree(root, epubDir string) error {
	starter, err := repositoryFS(root, "templates/book-starter")
	if err != nil {
		return err
	}
	for _, top := range []string{"mimetype", "META-INF", "OEBPS"} {
		err := fs.WalkDir(starter, top, func(name string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return zipfs.MkdirAll(filepath.Join(epubDir, filepath.FromSlash(name)))
			}
			data, err := fs.ReadFile(starter, name)
			if err != nil {
				return err
			}
			return zipfs.WriteFileAtomic(filepath.Join(epubDir, filepath.FromSlash(name)), data, 0o644)
		})
		if err != nil {
			return fmt.Errorf("copy starter %s: %w", top, err)
		}
	}
	return nil
}

// dropCharsetLines 等同于 `grep -v '^@charset'`：丢弃以 @charset 开头的行，
// 其余行各以 LF 结尾。
func dropCharsetLines(data []byte) []byte {
	var out bytes.Buffer
	for line := range strings.SplitSeq(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, "@charset") {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	if len(data) == 0 {
		return nil
	}
	return out.Bytes()
}

// insertAfterLine 等同于 awk '{ print } /needle/ { print extra }'。
func insertAfterLine(data []byte, needle, extra string) []byte {
	var out bytes.Buffer
	text := strings.TrimSuffix(string(data), "\n")
	if len(data) == 0 {
		return nil
	}
	for line := range strings.SplitSeq(text, "\n") {
		out.WriteString(line)
		out.WriteByte('\n')
		if strings.Contains(line, needle) {
			out.WriteString(extra)
			out.WriteByte('\n')
		}
	}
	return out.Bytes()
}

func applyPreset(ctx context.Context, root, preset, presetDir, epubDir string) (notes bool, err error) {
	raw, err := readRepositoryFile(root, path.Join(presetDir, "preset.json"))
	if err != nil {
		return false, err
	}
	var manifest struct {
		Layers []string `json:"layers"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return false, fmt.Errorf("preset %s: %w", preset, err)
	}
	commit := handbookShortRevision(ctx, root)
	design := filepath.Join(epubDir, "OEBPS", "Styles", "design.css")
	current, err := os.ReadFile(design)
	if err != nil {
		return false, err
	}
	var appended bytes.Buffer
	appended.Write(current)
	fmt.Fprintf(&appended, "\n/* 起点：preset %s @ %s，可自由修改；不再与 preset 同步。\n * 已略过 fonts.css（字体走 book.json 流程）与 notes.css（单独成层）。 */\n", preset, commit)
	for _, layer := range manifest.Layers {
		if layer == "" || layer == "fonts.css" || layer == "notes.css" {
			continue
		}
		css, err := readRepositoryFile(root, path.Join(presetDir, "Styles", layer))
		if err != nil {
			return false, fmt.Errorf("Preset layer missing: %s", layer)
		}
		fmt.Fprintf(&appended, "\n/* preset %s: %s */\n", preset, layer)
		appended.Write(dropCharsetLines(css))
	}
	if err := zipfs.WriteFileAtomic(design, appended.Bytes(), 0o644); err != nil {
		return false, err
	}

	// 标准弹注保留自己的层：base.css -> notes.css -> design.css。
	if !slices.Contains(manifest.Layers, "notes.css") {
		return false, nil
	}
	css, err := readRepositoryFile(root, path.Join(presetDir, "Styles", "notes.css"))
	if err != nil {
		return false, errors.New("Preset layer missing: notes.css")
	}
	var notesCSS bytes.Buffer
	fmt.Fprintf(&notesCSS, "@charset \"utf-8\";\n/* 起点：preset %s @ %s 的 notes.css，可自由修改。 */\n", preset, commit)
	notesCSS.Write(dropCharsetLines(css))
	if err := zipfs.WriteFileAtomic(filepath.Join(epubDir, "OEBPS", "Styles", "notes.css"), notesCSS.Bytes(), 0o644); err != nil {
		return false, err
	}
	opf := filepath.Join(epubDir, "OEBPS", "package.opf")
	opfData, err := os.ReadFile(opf)
	if err != nil {
		return false, err
	}
	opfData = insertAfterLine(opfData, `id="css-base"`, `    <item id="css-notes"   href="Styles/notes.css"       media-type="text/css"/>`)
	if err := zipfs.WriteFileAtomic(opf, opfData, 0o644); err != nil {
		return false, err
	}
	pages, err := filepath.Glob(filepath.Join(epubDir, "OEBPS", "Text", "*.xhtml"))
	if err != nil {
		return false, err
	}
	slices.Sort(pages)
	for _, page := range pages {
		data, err := os.ReadFile(page)
		if err != nil {
			return false, err
		}
		data = insertAfterLine(data, `href="../Styles/base.css"`, `  <link rel="stylesheet" type="text/css" href="../Styles/notes.css"/>`)
		if err := zipfs.WriteFileAtomic(page, data, 0o644); err != nil {
			return false, err
		}
	}
	return true, nil
}

// bookGitignore 是 --git 时写入的忽略规则；extra 为库字体在源树内的路径。
func bookGitignore(extra []string, blanketFonts bool) []byte {
	var b strings.Builder
	b.WriteString("# Temporary build state and the single replaceable delivery artifact.\n/.epub/\n/out/\n")
	b.WriteString("# Preview-only injected masters; source subset fonts are ignored individually\n# when a book mixes locked library fonts and retained embedded subsets.\n")
	if blanketFonts {
		b.WriteString("/epub/OEBPS/Fonts/\n")
	}
	for _, target := range extra {
		b.WriteString("/epub/" + target + "\n")
	}
	b.WriteString("\n# Local editor noise.\n.DS_Store\nThumbs.db\n")
	return []byte(b.String())
}

// initBookGit 在 dir 初始化独立 Git 并写 .gitignore（可选功能）。
func initBookGit(ctx context.Context, dir string, ignoredFonts []string, blanketFonts bool) error {
	if err := zipfs.WriteFileAtomic(filepath.Join(dir, ".gitignore"), bookGitignore(ignoredFonts, blanketFonts), 0o644); err != nil {
		return err
	}
	res, err := externRun(ctx, []string{"git", "-C", dir, "init", "-b", "main"})
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("git init failed: %s", strings.TrimSpace(string(res.Stderr)))
	}
	return nil
}

// addReaderLayer 给新书加一个空的 Styles/reader.css，并在 OPF 与每个 XHTML 里
// 把它链接在最后。阅读器变体只需用覆盖层替换这一个文件。
func addReaderLayer(epubDir string) error {
	styles := filepath.Join(epubDir, "OEBPS", "Styles")
	css := "@charset \"utf-8\";\n/* 阅读器覆盖层入口：默认为空。变体覆盖层（variants/<名称>/）只需替换本文件。\n * 不要在这里写阅读器识别查询（SPEC §5.6.1 禁止 amzn-kf8 / amzn-mobi）。 */\n"
	if err := zipfs.WriteFileAtomic(filepath.Join(styles, "reader.css"), []byte(css), 0o644); err != nil {
		return err
	}
	opf := filepath.Join(epubDir, "OEBPS", "package.opf")
	opfData, err := os.ReadFile(opf)
	if err != nil {
		return err
	}
	opfData = insertAfterLine(opfData, `id="css-design"`, `    <item id="css-reader"  href="Styles/reader.css"     media-type="text/css"/>`)
	if err := zipfs.WriteFileAtomic(opf, opfData, 0o644); err != nil {
		return err
	}
	pages, err := filepath.Glob(filepath.Join(epubDir, "OEBPS", "Text", "*.xhtml"))
	if err != nil {
		return err
	}
	for _, page := range pages {
		data, err := os.ReadFile(page)
		if err != nil {
			return err
		}
		data = insertAfterLine(data, `href="../Styles/design.css"`, `  <link rel="stylesheet" type="text/css" href="../Styles/reader.css"/>`)
		if err := zipfs.WriteFileAtomic(page, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
