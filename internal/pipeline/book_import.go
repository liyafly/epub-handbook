package pipeline

// epub book import：冻结原件、DRM/加密停止、导航审计、解包、盘点内嵌字体。
//
// 由已删除的 templates/book-starter/import_book.py 移植而来。安全属性保持：原件只读并记录
// SHA-256；加密/rights 元数据、重名 ZIP 成员、不安全成员名一律停止；非 OEBPS
// 布局原样保留（不改名、不猜测），构建时按原布局打包；完整字体按 SHA 进入共享库且绝不覆盖不同版本；失败时冻结输入与
// 诊断保留在 .book-import-* 目录。

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
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

// BookImportOptions 是 epub book import 的输入。
type BookImportOptions struct {
	Original string
	Dir      string
	// Git 为 true 时在书根初始化 Git 并提交基线（可选）。
	Git bool
}

// BookImport 导入已有 EPUB 并建立新布局，返回退出码。
func BookImport(ctx context.Context, opts BookImportOptions, bio BookIO) int {
	if strings.TrimSpace(opts.Original) == "" || strings.TrimSpace(opts.Dir) == "" {
		return bookExit(bio, usageErrorf("usage: epub book import <original.epub> <book-directory> [--git]"))
	}
	r := &bookRunner{ctx: ctx, bio: bio, env: readBookEnv()}
	ensureLocalBin(r.env)
	dest, legacy, retained, err := r.importBook(opts)
	if err != nil {
		if retained != "" {
			err = fmt.Errorf("%w\nImport stopped; frozen input and diagnostics retained: %s", err, retained)
		}
		return bookExit(bio, err)
	}
	fmt.Fprintln(bio.out(), "Imported:", dest)
	if legacy {
		fmt.Fprintln(bio.out(), "Note: not an OEBPS/ layout; kept as-is and will be built as-is (presets and overlays assume OEBPS/).")
	}
	return ExitOK
}

func (r *bookRunner) importBook(opts BookImportOptions) (dest string, legacy bool, retained string, err error) {
	original, err := filepath.Abs(opts.Original)
	if err != nil {
		return "", false, "", err
	}
	dest, err = filepath.Abs(opts.Dir)
	if err != nil {
		return "", false, "", err
	}
	if !isFile(original) {
		return "", false, "", usageErrorf("Input must be a regular EPUB and destination must not exist")
	}
	if _, lerr := os.Lstat(dest); lerr == nil {
		return "", false, "", usageErrorf("Input must be a regular EPUB and destination must not exist")
	}
	parent := filepath.Dir(dest)
	if err := zipfs.MkdirAll(parent); err != nil {
		return "", false, "", err
	}
	retained, err = zipfs.MkdirTemp(parent, ".book-import-*")
	if err != nil {
		return "", false, "", err
	}
	_ = zipfs.Chmod(retained, 0o755)
	bookDir := filepath.Join(retained, "book")
	legacy, err = r.importInto(opts, original, dest, bookDir)
	if err != nil {
		return "", false, retained, err
	}
	if err := zipfs.RenameNoReplace(bookDir, dest); err != nil {
		return "", false, retained, fmt.Errorf("move imported book into place: %w", err)
	}
	_ = zipfs.Remove(retained)
	return dest, legacy, "", nil
}

func (r *bookRunner) importInto(opts BookImportOptions, original, dest, bookDir string) (legacy bool, err error) {
	ctx := r.ctx
	if err := zipfs.MkdirAll(bookDir); err != nil {
		return false, err
	}
	frozen := filepath.Join(bookDir, "original.epub")
	if err := zipfs.CopyFile(ctx, original, frozen, 0o444); err != nil {
		return false, err
	}
	reports := filepath.Join(bookDir, ".epub", "import")
	if err := zipfs.MkdirAll(reports); err != nil {
		return false, err
	}

	// 导航审计：DRM/损坏阻断；不解密。
	layout := bookLayout{BookDir: bookDir}
	if err := r.capabilityCheck(layout, filepath.Join(reports, "nav-audit.json"), "epub.package.nav.audit", Options{
		CapabilityID: "epub.package.nav.audit", InputPath: frozen,
	}); err != nil {
		if errors.Is(err, errBookStepFailed) {
			return false, errors.New("Navigation audit failed; inspect report and use cleanup-flow.md for repairs. DRM is not decrypted.")
		}
		return false, err
	}

	archive, err := zipfs.OpenContext(ctx, frozen)
	if err != nil {
		return false, err
	}
	names := archive.Names()
	if len(names) != len(slices.Compact(slices.Sorted(slices.Values(names)))) {
		_ = archive.Close()
		return false, errors.New("Duplicate ZIP entries are not supported")
	}
	rootfile, err := readRootfile(ctx, archive)
	if err != nil {
		_ = archive.Close()
		return false, err
	}
	// 非 OEBPS 布局（或 OEBPS/META-INF 之外有资源）原样保留：不改名、不猜测，
	// 在 book.json 标记后构建时按原布局打包。
	legacy = !strings.HasPrefix(rootfile, "OEBPS/") || slices.ContainsFunc(names, func(name string) bool {
		return name != "mimetype" && !strings.HasSuffix(name, "/") &&
			!strings.HasPrefix(name, "META-INF/") && !strings.HasPrefix(name, "OEBPS/")
	})
	if slices.Contains(names, "META-INF/encryption.xml") || slices.Contains(names, "META-INF/rights.xml") {
		_ = archive.Close()
		return false, errors.New("Encryption metadata present; stop for explicit audit, no decryption")
	}
	source := filepath.Join(bookDir, "epub")
	err = archive.ExtractTo(ctx, source)
	_ = archive.Close()
	if err != nil {
		return false, err
	}

	jobs, decisions, ignored, err := r.inventoryFonts(frozen, source, dest, reports)
	if err != nil {
		return false, err
	}

	digest, err := zipfs.FileSHA256Context(ctx, frozen)
	if err != nil {
		return false, err
	}
	cfg := bookConfig{
		Version: 1, Title: filepath.Base(dest), Output: defaultOutputName, LegacyLayout: legacy,
		Fonts:    []json.RawMessage{},
		Original: &bookOriginal{Name: filepath.Base(original), SHA256: digest},
	}
	for _, job := range jobs {
		raw, err := json.Marshal(job)
		if err != nil {
			return false, err
		}
		cfg.Fonts = append(cfg.Fonts, raw)
	}
	if err := writeBookConfig(filepath.Join(bookDir, bookConfigName), cfg); err != nil {
		return false, err
	}
	relocation, err := json.MarshalIndent(map[string]string{
		"auditedPath": frozen, "retainedPath": filepath.Join(dest, "original.epub"), "sha256": digest,
	}, "", "  ")
	if err != nil {
		return false, err
	}
	if err := zipfs.WriteFileAtomic(filepath.Join(reports, "relocation.json"), append(relocation, '\n'), 0o644); err != nil {
		return false, err
	}
	fontReport, err := json.MarshalIndent(decisions, "", "  ")
	if err != nil {
		return false, err
	}
	if err := zipfs.WriteFileAtomic(filepath.Join(reports, "fonts.json"), append(fontReport, '\n'), 0o644); err != nil {
		return false, err
	}

	if opts.Git {
		if err := initBookGit(ctx, bookDir, ignored, false); err != nil {
			return false, err
		}
		if err := gitCommitBaseline(ctx, bookDir, digest); err != nil {
			return false, err
		}
	}
	return legacy, nil
}

// readRootfile 返回 container.xml 中第一个 rootfile 的 full-path。
func readRootfile(ctx context.Context, archive *zipfs.Archive) (string, error) {
	data, err := archive.ReadContext(ctx, "META-INF/container.xml")
	if err != nil {
		return "", fmt.Errorf("read META-INF/container.xml: %w", err)
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", errors.New("Non-OEBPS layout: container.xml has no rootfile")
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "rootfile" {
			for _, attr := range start.Attr {
				if attr.Name.Local == "full-path" {
					return attr.Value, nil
				}
			}
			return "", nil
		}
	}
}

type fontDecision struct {
	Target          string `json:"target"`
	Glyphs          int    `json:"glyphs"`
	SubsetHeuristic bool   `json:"subsetHeuristic"`
	Action          string `json:"action"`
}

type importedFontJob struct {
	Target    string         `json:"target"`
	Variation map[string]any `json:"variation,omitempty"`
	From      string         `json:"from,omitempty"`
	SHA256    string         `json:"sha256,omitempty"`
}

// inventoryFonts 盘点解包源里的字体：判断为全量的进入共享库并从源树移除，
// 判断为子集的保留。返回 book.json 的字体任务、决策记录与应被 Git 忽略的路径。
func (r *bookRunner) inventoryFonts(frozen, source, dest, reports string) (jobs []importedFontJob, decisions []fontDecision, ignored []string, err error) {
	decisions = []fontDecision{}
	var fontFiles []string
	err = filepath.WalkDir(source, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.Type().IsRegular() {
			switch strings.ToLower(filepath.Ext(d.Name())) {
			case ".otf", ".ttf", ".woff", ".woff2":
				rel, rerr := filepath.Rel(source, p)
				if rerr != nil {
					return rerr
				}
				fontFiles = append(fontFiles, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil || len(fontFiles) == 0 {
		return nil, decisions, nil, err
	}
	slices.Sort(fontFiles)

	res, err := runProvider(r.ctx, r.bio, []string{r.env.fontBin(), "coverage", frozen, "--json"})
	if err != nil {
		return nil, nil, nil, err
	}
	if werr := zipfs.WriteFileAtomic(filepath.Join(reports, "coverage.json"), res.Stdout, 0o644); werr != nil {
		return nil, nil, nil, werr
	}
	if res.ExitCode != 0 && res.ExitCode != 1 {
		return nil, nil, nil, errors.New("Font inventory failed; source and report retained")
	}
	var coverage struct {
		Fonts struct {
			Embedded []struct {
				File          string         `json:"file"`
				VariationAxes map[string]any `json:"variation_axes"`
				IsSubset      bool           `json:"is_subset"`
				GlyphCount    int            `json:"glyph_count"`
			} `json:"embedded"`
		} `json:"fonts"`
	}
	if err := providerJSON(res.Stdout, &coverage); err != nil {
		return nil, nil, nil, fmt.Errorf("Font inventory failed: unreadable provider report: %w", err)
	}
	indexed := map[string]int{}
	for i, item := range coverage.Fonts.Embedded {
		indexed[item.File] = i
	}
	library, err := fontLibraryDir(filepath.Dir(dest))
	if err != nil {
		return nil, nil, nil, err
	}
	// 移动任何母版之前先确认每个字体都已分类。
	for _, target := range fontFiles {
		if _, ok := indexed[target]; !ok {
			return nil, nil, nil, fmt.Errorf("Unclassified font: %s", target)
		}
	}
	for _, target := range fontFiles {
		info := coverage.Fonts.Embedded[indexed[target]]
		job := importedFontJob{Target: target}
		if len(info.VariationAxes) > 0 {
			job.Variation = map[string]any{"mode": "instance", "axes": info.VariationAxes}
		}
		suspected := info.IsSubset || info.GlyphCount < 10000
		action := "retain-source-subset"
		if !suspected {
			font := filepath.Join(source, filepath.FromSlash(target))
			name, digest, rerr := registerFont(r.ctx, font, library, filepath.Base(dest))
			if rerr != nil {
				return nil, nil, nil, rerr
			}
			job.From, job.SHA256 = name, digest
			if rerr := zipfs.Remove(font); rerr != nil {
				return nil, nil, nil, rerr
			}
			ignored = append(ignored, target)
			action = "library"
		}
		jobs = append(jobs, job)
		decisions = append(decisions, fontDecision{Target: target, Glyphs: info.GlyphCount, SubsetHeuristic: suspected, Action: action})
	}
	return jobs, decisions, ignored, nil
}

func isFontLibraryFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".otf", ".ttf", ".woff", ".woff2":
		return true
	}
	return false
}

// registerFont 复用 SHA 相同的母版；绝不覆盖库内不同版本。
func registerFont(ctx context.Context, source, library, origin string) (name, digest string, err error) {
	if err := zipfs.MkdirAll(library); err != nil {
		return "", "", err
	}
	digest, err = zipfs.FileSHA256Context(ctx, source)
	if err != nil {
		return "", "", err
	}
	entries, err := os.ReadDir(library)
	if err != nil {
		return "", "", err
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !isFontLibraryFile(entry.Name()) {
			continue
		}
		existing, err := zipfs.FileSHA256Context(ctx, filepath.Join(library, entry.Name()))
		if err == nil && existing == digest {
			return entry.Name(), digest, nil
		}
	}
	name = filepath.Base(source)
	if _, err := os.Lstat(filepath.Join(library, name)); err == nil {
		ext := filepath.Ext(name)
		name = fmt.Sprintf("%s-%s%s", strings.TrimSuffix(name, ext), digest[:12], ext)
	}
	if _, err := os.Lstat(filepath.Join(library, name)); err == nil {
		return "", "", fmt.Errorf("Font library filename collision: %s", name)
	}
	if err := zipfs.CopyFile(ctx, source, filepath.Join(library, name), 0o644); err != nil {
		return "", "", err
	}
	inventory := filepath.Join(library, "字体清单.tsv")
	var rows bytes.Buffer
	existingInventory, readErr := readSmallFile(inventory)
	writer := csv.NewWriter(&rows)
	writer.Comma = '\t'
	writer.UseCRLF = true
	if readErr != nil {
		_ = writer.Write([]string{"文件名", "sha256", "来源", "许可", "备注"})
	}
	_ = writer.Write([]string{name, digest, origin, "待核", "按 SHA 去重；仅本地制作使用，发布前核验许可"})
	writer.Flush()
	combined := append(slices.Clone(existingInventory), rows.Bytes()...)
	if err := zipfs.WriteFileAtomic(inventory, combined, 0o644); err != nil {
		return "", "", err
	}
	readme := filepath.Join(library, "README.md")
	if !isFile(readme) {
		text := "# 共享字体库\n\n全量字体按 SHA-256 锁定；来源与许可见 `字体清单.tsv`。子集字体留在各书源树。\n字体版本变化须审阅后运行 `epub book fonts --update-lock`。\n"
		if err := zipfs.WriteFileAtomic(readme, []byte(text), 0o644); err != nil {
			return "", "", err
		}
	}
	return name, digest, nil
}

func gitCommitBaseline(ctx context.Context, dir, digest string) error {
	for _, argv := range [][]string{
		{"git", "-C", dir, "add", "-A"},
		{"git", "-C", dir, "commit", "-m", "import: freeze EPUB baseline sha256 " + digest},
	} {
		res, err := externRun(ctx, argv)
		if err != nil {
			return err
		}
		if res.ExitCode != 0 {
			return fmt.Errorf("%s failed: %s", path.Base(argv[3]), strings.TrimSpace(string(res.Stderr)+string(res.Stdout)))
		}
	}
	return nil
}
