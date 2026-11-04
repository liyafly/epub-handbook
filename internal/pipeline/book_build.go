package pipeline

// epub book build / fonts / clean / accept-dist
//
// 逐条移植 templates/book-starter/build.sh 的安全属性：构建锁、dist 收据、
// 失败现场保留、子集化 + 独立覆盖检查、导航审计、全项 redline、原子替换、
// 交付记录与 accept-dist 的逐成员比对。所有磁盘写入经 internal/zipfs，
// 外部进程经 internal/extern。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/zipfs"
)

// 书级命令名。
const (
	BookCommandBuild      = "build"
	BookCommandFonts      = "fonts"
	BookCommandClean      = "clean"
	BookCommandAcceptDist = "accept-dist"
)

// BookRunOptions 是 build / fonts / clean / accept-dist 的输入。
type BookRunOptions struct {
	Dir     string // 书目录；空表示当前目录
	Command string
	// UpdateLock 仅用于 fonts：显式更新字体锁（须先审阅母版变化）。
	UpdateLock bool
}

type bookRunner struct {
	ctx        context.Context
	bio        BookIO
	env        bookEnv
	updateLock bool
}

// BookRun 执行一条书级命令并返回退出码（0 成功，1 失败，3 用法错误）。
func BookRun(ctx context.Context, opts BookRunOptions, bio BookIO) int {
	switch opts.Command {
	case BookCommandBuild, BookCommandFonts, BookCommandClean, BookCommandAcceptDist:
	default:
		return bookExit(bio, usageErrorf("unknown book command: %s", opts.Command))
	}
	r := &bookRunner{ctx: ctx, bio: bio, env: readBookEnv(), updateLock: opts.UpdateLock}
	ensureLocalBin(r.env)
	return bookExit(bio, r.run(opts.Dir, opts.Command))
}

func bookExit(bio BookIO, err error) int {
	if err == nil {
		return ExitOK
	}
	fmt.Fprintln(bio.err(), "epub book:", err)
	if _, ok := errors.AsType[*UsageError](err); ok {
		return ExitUsage
	}
	return ExitFailed
}

func (r *bookRunner) run(dir, command string) error {
	layout, err := resolveBookLayout(dir, r.env)
	if err != nil {
		return err
	}
	if !validOutputName(layout.OutputName) {
		return usageErrorf("EPUB_OUTPUT_NAME must be a filename")
	}
	if len(layout.Volumes) > 0 {
		for _, volume := range layout.Volumes {
			if err := r.run(volume, command); err != nil {
				return err
			}
		}
		return nil
	}
	switch command {
	case BookCommandFonts:
		return r.fonts(layout)
	case BookCommandClean:
		return r.clean(layout)
	default:
		return r.build(layout, command)
	}
}

// ---------------------------------------------------------------- fonts

// prepareFontConfig 返回交给 provider 的配置路径与 --library 参数。
// 旧布局直接使用 fonts.json；新布局从 book.json 派生只含 version/usage/fonts
// 的配置（provider 对未知键严格拒绝），并按 book.json 的位置确定共享字体库。
func prepareFontConfig(layout bookLayout, scratch string) (config string, libraryArgs []string, err error) {
	if layout.Kind != layoutBook {
		return layout.FontConfig, nil, nil
	}
	usage := layout.Config.Usage
	doc := struct {
		Version int               `json:"version"`
		Usage   string            `json:"usage,omitempty"`
		Fonts   []json.RawMessage `json:"fonts"`
	}{Version: 1, Usage: usage, Fonts: layout.Config.Fonts}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", nil, err
	}
	config = filepath.Join(scratch, "fonts.json")
	if err := zipfs.WriteFileAtomic(config, append(data, '\n'), 0o644); err != nil {
		return "", nil, err
	}
	if lib := findSharedFontLibrary(layout.FontConfig); lib != "" {
		libraryArgs = []string{"--library", lib}
	}
	return config, libraryArgs, nil
}

func (r *bookRunner) fonts(layout bookLayout) error {
	if !layout.hasFontConfig() {
		fmt.Fprintln(r.bio.out(), "No fonts.json; legacy source fonts stay in place.")
		return nil
	}
	scratch, err := r.scratchDir(layout, "fonts.")
	if err != nil {
		return err
	}
	defer func() { _ = zipfs.RemoveAll(scratch) }()
	config, libraryArgs, err := prepareFontConfig(layout, scratch)
	if err != nil {
		return err
	}
	argv := []string{r.env.fontBin(), "inject", "--config", config, "--tree", layout.EpubDir}
	argv = append(argv, libraryArgs...)
	if r.updateLock {
		argv = append(argv, "--update-lock")
	}
	res, err := runProvider(r.ctx, r.bio, argv)
	if err != nil {
		return err
	}
	_, _ = r.bio.out().Write(res.Stdout)
	if res.ExitCode != 0 {
		return fmt.Errorf("epub-font inject exited with status %d", res.ExitCode)
	}
	if r.updateLock && layout.Kind == layoutBook {
		return mergeUpdatedLock(layout, config)
	}
	return nil
}

// mergeUpdatedLock 把 provider 更新过的锁（派生配置里的 fonts）写回 book.json，
// 保留其余键。
func mergeUpdatedLock(layout bookLayout, derived string) error {
	updated, err := readSmallFile(derived)
	if err != nil {
		return err
	}
	var doc struct {
		Fonts []json.RawMessage `json:"fonts"`
	}
	if err := json.Unmarshal(updated, &doc); err != nil {
		return err
	}
	original, err := readSmallFile(layout.FontConfig)
	if err != nil {
		return err
	}
	var whole map[string]json.RawMessage
	if err := json.Unmarshal(original, &whole); err != nil {
		return err
	}
	fonts, err := json.Marshal(doc.Fonts)
	if err != nil {
		return err
	}
	whole["fonts"] = fonts
	data, err := json.MarshalIndent(whole, "", "  ")
	if err != nil {
		return err
	}
	return zipfs.WriteFileAtomic(layout.FontConfig, append(data, '\n'), 0o644)
}

func (r *bookRunner) scratchDir(layout bookLayout, prefix string) (string, error) {
	if err := zipfs.MkdirAll(layout.StateDir); err != nil {
		return "", err
	}
	return zipfs.MkdirTemp(layout.StateDir, prefix+"*")
}

// ---------------------------------------------------------------- clean

func (r *bookRunner) clean(layout bookLayout) error {
	if _, err := os.Lstat(filepath.Join(layout.StateDir, "build.lock")); err == nil {
		return errors.New("Build lock exists; stop the active build before cleaning")
	}
	entries, err := os.ReadDir(layout.StateDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	type build struct {
		path string
		mod  int64
	}
	var builds []build
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "build.") || !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		builds = append(builds, build{filepath.Join(layout.StateDir, entry.Name()), info.ModTime().UnixNano()})
	}
	slices.SortStableFunc(builds, func(a, b build) int {
		switch {
		case a.mod > b.mod:
			return -1
		case a.mod < b.mod:
			return 1
		}
		return strings.Compare(a.path, b.path)
	})
	for _, stale := range builds[min(1, len(builds)):] {
		fmt.Fprintln(r.bio.out(), "Removing", stale.path)
		if err := zipfs.RemoveAll(stale.path); err != nil {
			return err
		}
	}
	if len(builds) > 0 {
		fmt.Fprintln(r.bio.out(), "Retained latest diagnostic build:", builds[0].path)
	}
	return nil
}

// ---------------------------------------------------------------- build

var (
	errBookStepFailed = errors.New("a build check failed; the last accepted dist was not changed")
)

func validateEpubTree(layout bookLayout) error {
	if !isFile(filepath.Join(layout.EpubDir, "mimetype")) || !isDir(filepath.Join(layout.EpubDir, "META-INF")) ||
		(!isDir(filepath.Join(layout.EpubDir, "OEBPS")) && !layout.LegacyLayout) {
		return fmt.Errorf("Expected mimetype, META-INF/, and OEBPS/ under %s", layout.EpubDir)
	}
	link, err := zipfs.FindSymlink(layout.EpubDir)
	if err != nil {
		return err
	}
	if link != "" {
		return errors.New("EPUB source tree contains a symlink; resolve it before building.")
	}
	return nil
}

var reportNames = []string{"font-subset.json", "nav-audit.json", "redline.txt", "previous-dist-redline.txt"}

func (r *bookRunner) build(layout bookLayout, command string) (err error) {
	accept := command == BookCommandAcceptDist
	stdout, stderr := r.bio.out(), r.bio.err()
	if err := validateEpubTree(layout); err != nil {
		return err
	}
	if err := zipfs.MkdirAll(layout.DistDir); err != nil {
		return err
	}
	if err := zipfs.MkdirAll(layout.StateDir); err != nil {
		return err
	}
	lockDir := filepath.Join(layout.StateDir, "build.lock")
	if lockErr := zipfs.MkdirExclusive(lockDir); lockErr != nil {
		if errors.Is(lockErr, zipfs.ErrOutputExists) {
			return fmt.Errorf("Another build is active (or a stale lock exists): %s", lockDir)
		}
		return lockErr
	}
	defer func() { _ = zipfs.Remove(lockDir) }()

	buildTmp := ""
	succeeded := false
	defer func() {
		if buildTmp == "" {
			return
		}
		if succeeded {
			_ = zipfs.RemoveAll(buildTmp)
			return
		}
		// 保留本次失败的输入/候选与报告；之后的构建会替换固定名报告。
		for _, name := range reportNames {
			src := filepath.Join(layout.StateDir, name)
			if isFile(src) {
				if copyErr := zipfs.CopyFile(context.WithoutCancel(r.ctx), src, filepath.Join(buildTmp, name), 0o644); copyErr != nil {
					fmt.Fprintf(stderr, "Could not snapshot report; inspect original: %s\n", src)
				}
			}
		}
		fmt.Fprintf(stderr, "Build failed; retained diagnostic files: %s\n", buildTmp)
	}()

	output := layout.outputPath()
	receipt := layout.receiptPath()
	distExists, distShaAtStart := false, ""
	if info, lerr := os.Lstat(output); lerr == nil {
		distExists = true
		if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("Refusing to replace a non-regular dist artifact: %s", output)
		}
		if !accept && !isFile(receipt) {
			return fmt.Errorf("Refusing to overwrite existing dist without its accepted SHA-256 receipt: %s\n"+
				"Review/import any dist edits into %s, then record the reviewed artifact SHA in %s.", output, layout.EpubDir, receipt)
		}
		distShaAtStart, err = zipfs.FileSHA256Context(r.ctx, output)
		if err != nil {
			return err
		}
		recorded := readReceipt(receipt)
		if !accept && distShaAtStart != recorded {
			return fmt.Errorf("Refusing to overwrite dist: its SHA-256 differs from the last accepted build.\n"+
				"Artifact: %s\nRecorded: %s\nCurrent:  %s\n"+
				"Review the EPUB changes and merge intended edits into %s before rebuilding.", output, recorded, distShaAtStart, layout.EpubDir)
		}
	}
	if accept && !distExists {
		return errors.New("accept-dist requires an existing dist artifact.")
	}

	// 现有 dist 通过收据预检之后才清掉上一轮报告；失败现场目录互相独立，不在此清理。
	for _, name := range reportNames {
		if err := zipfs.Remove(filepath.Join(layout.StateDir, name)); err != nil {
			return err
		}
	}
	buildTmp, err = zipfs.MkdirTemp(layout.StateDir, "build.*")
	if err != nil {
		return err
	}
	fullEpub := filepath.Join(buildTmp, "full-font.epub")
	finalEpub := filepath.Join(buildTmp, "final.epub")
	srcDir := filepath.Join(buildTmp, "src")

	keep := func(rel string, top bool) bool {
		if !top {
			return true
		}
		if layout.LegacyLayout {
			// 已审计的非 OEBPS 布局：原样打包，不改名。
			return !strings.HasPrefix(rel, ".") && rel != "build.sh"
		}
		return rel == "mimetype" || rel == "META-INF" || rel == "OEBPS"
	}
	if err := zipfs.CopyTree(r.ctx, layout.EpubDir, srcDir, keep); err != nil {
		return err
	}

	// 字体配置：旧布局沿用 fonts.json，新布局从 book.json 派生。
	fontConfig := ""
	var libraryArgs []string
	var jobs []fontJob
	if layout.hasFontConfig() {
		fontConfig, libraryArgs, err = prepareFontConfig(layout, buildTmp)
		if err != nil {
			return err
		}
		if layout.Kind == layoutBook {
			jobs, err = layout.Config.jobs()
		} else {
			jobs, err = readFontConfigJobs(layout.FontConfig)
		}
		if err != nil {
			return err
		}
		if slices.ContainsFunc(jobs, func(j fontJob) bool { return j.From != "" }) {
			argv := []string{r.env.fontBin(), "inject", "--config", fontConfig, "--tree", srcDir}
			argv = append(argv, libraryArgs...)
			res, runErr := runProvider(r.ctx, r.bio, argv)
			if runErr != nil {
				return runErr
			}
			if werr := zipfs.WriteFileAtomic(filepath.Join(buildTmp, "font-inject.json"), res.Stdout, 0o644); werr != nil {
				return werr
			}
			if res.ExitCode != 0 {
				return fmt.Errorf("epub-font inject exited with status %d", res.ExitCode)
			}
		}
	}
	if err := zipfs.PackTree(r.ctx, srcDir, fullEpub, "mimetype"); err != nil {
		return err
	}

	hasFonts := layout.hasFontConfig() || treeHasFontFiles(srcDir)
	subsetReport := filepath.Join(layout.StateDir, "font-subset.json")
	if hasFonts {
		args := Args{}
		if fontConfig != "" {
			args["font_config"] = fontConfig
		}
		if err := r.capabilityCheck(layout, subsetReport, "epub.font.subset", Options{
			CapabilityID: "epub.font.subset", InputPath: fullEpub, OutputPath: finalEpub, Args: args,
		}); err != nil {
			return err
		}
	} else if err := zipfs.CopyFile(r.ctx, fullEpub, finalEpub, 0o644); err != nil {
		return err
	}
	if isFile(subsetReport) {
		r.printProviderWarnings(subsetReport)
	}

	if err := r.capabilityCheck(layout, filepath.Join(layout.StateDir, "nav-audit.json"), "epub.package.nav.audit", Options{
		CapabilityID: "epub.package.nav.audit", InputPath: finalEpub,
	}); err != nil {
		return err
	}
	if err := r.redlineCheck(layout, filepath.Join(layout.StateDir, "redline.txt"), fullEpub, finalEpub, true); err != nil {
		return err
	}
	if isFile(output) {
		fmt.Fprintf(stdout, "INFO comparing candidate with previous dist (review only; does not block): %s\n", output)
		prevPath := filepath.Join(layout.StateDir, "previous-dist-redline.txt")
		_ = r.redlineCheck(layout, prevPath, output, finalEpub, false)
		if data, readErr := readSmallFile(prevPath); readErr == nil {
			_, _ = stdout.Write(data)
		}
	}

	// 不覆盖构建期间被改动或新出现的成品。
	if distExists {
		if !isFile(output) {
			return errors.New("Refusing to replace dist because the previous artifact disappeared during this build.")
		}
		if info, lerr := os.Lstat(output); lerr != nil || info.Mode()&fs.ModeSymlink != 0 {
			return errors.New("Refusing to replace dist because the previous artifact disappeared during this build.")
		}
		now, shaErr := zipfs.FileSHA256Context(r.ctx, output)
		if shaErr != nil {
			return shaErr
		}
		if now != distShaAtStart {
			return errors.New("Refusing to replace dist because it changed while this build was running.")
		}
	} else if _, lerr := os.Lstat(output); lerr == nil {
		return errors.New("Refusing to replace dist because an artifact appeared while this build was running.")
	}

	var finalSHA string
	if accept {
		// 包括 CSS 与图片在内的所有成员字节都须与源树重建的候选一致；红线本身
		// 查不出遗漏的样式表修改。
		changed, cmpErr := compareArchiveMembers(r.ctx, output, finalEpub)
		if cmpErr != nil {
			return cmpErr
		}
		if len(changed) > 0 {
			return fmt.Errorf("Dist still differs from rebuilt source; merge/review first: %s", strings.Join(changed, ", "))
		}
		finalSHA, err = zipfs.FileSHA256Context(r.ctx, output)
	} else {
		finalSHA, err = zipfs.FileSHA256Context(r.ctx, finalEpub)
	}
	if err != nil {
		return err
	}

	// 先在内存里备好交付记录，再替换成品。
	logBytes, err := buildDeliveryLog(r.ctx, layout, jobs, finalSHA)
	if err != nil {
		return err
	}
	if !accept {
		if err := zipfs.ReplaceFile(finalEpub, output); err != nil {
			return err
		}
	}
	if err := zipfs.WriteFileAtomic(receipt, []byte(finalSHA+"\n"), 0o644); err != nil {
		return err
	}
	if err := zipfs.WriteFileAtomic(layout.DeliveryLog, logBytes, 0o644); err != nil {
		return err
	}
	succeeded = true
	fmt.Fprintf(stdout, "Built %s\n", output)
	return nil
}

func readReceipt(path string) string {
	data, err := readSmallFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), "\r\n \t")
}

func treeHasFontFiles(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		switch strings.ToLower(filepath.Ext(d.Name())) {
		case ".ttf", ".otf", ".woff", ".woff2", ".ttc", ".otc":
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func buildDeliveryLog(ctx context.Context, layout bookLayout, jobs []fontJob, sha string) ([]byte, error) {
	fonts := make([]string, 0, len(jobs))
	for _, job := range jobs {
		if job.From != "" {
			fonts = append(fonts, job.Target+"="+job.SHA256)
		}
	}
	fontLock := "source-tree"
	if len(fonts) > 0 {
		fontLock = strings.Join(fonts, ";")
	}
	row := deliveryRow(bookSourceRevision(ctx, layout), handbookRevision(ctx), sha, fontLock)
	existing := []byte(deliveryHeader)
	if layout.Kind == layoutLegacy {
		if data, err := readSmallFile(layout.DeliveryLog); err == nil {
			existing = data
		}
		if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
			existing = append(existing, '\n')
		}
	}
	return append(existing, row...), nil
}

// compareArchiveMembers 返回两个 EPUB 之间成员集合或字节不同的名字（升序）。
func compareArchiveMembers(ctx context.Context, acceptedPath, candidatePath string) ([]string, error) {
	accepted, err := zipfs.OpenContext(ctx, acceptedPath)
	if err != nil {
		return nil, err
	}
	defer accepted.Close()
	candidate, err := zipfs.OpenContext(ctx, candidatePath)
	if err != nil {
		return nil, err
	}
	defer candidate.Close()
	a, b := map[string]bool{}, map[string]bool{}
	for _, name := range accepted.Names() {
		a[name] = true
	}
	for _, name := range candidate.Names() {
		b[name] = true
	}
	changedSet := map[string]bool{}
	for name := range a {
		if !b[name] {
			changedSet[name] = true
			continue
		}
		left, err := accepted.ReadContext(ctx, name)
		if err != nil {
			return nil, err
		}
		right, err := candidate.ReadContext(ctx, name)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(left, right) {
			changedSet[name] = true
		}
	}
	for name := range b {
		if !a[name] {
			changedSet[name] = true
		}
	}
	changed := make([]string, 0, len(changedSet))
	for name := range changedSet {
		changed = append(changed, name)
	}
	slices.Sort(changed)
	return changed, nil
}

// capabilityCheck 运行一个 capability，把信封写入报告文件，并打印 PASS/FAIL 行。
func (r *bookRunner) capabilityCheck(layout bookLayout, reportPath, label string, opts Options) error {
	outcome, runErr := Run(r.ctx, opts)
	data, marshalErr := MarshalEnvelope(outcome.Envelope)
	if marshalErr == nil {
		marshalErr = zipfs.WriteFileAtomic(reportPath, data, 0o644)
	}
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if runErr != nil || outcome.ExitCode != ExitOK {
		fmt.Fprintf(r.bio.err(), "FAIL %s (report: %s)\n", label, layout.displayPath(reportPath))
		if marshalErr == nil {
			_, _ = r.bio.err().Write(data)
		}
		if runErr != nil {
			fmt.Fprintln(r.bio.err(), runErr)
		}
		return errBookStepFailed
	}
	if marshalErr != nil {
		return marshalErr
	}
	warnings := 0
	for _, f := range outcome.Envelope.Findings {
		if f.Level == "warn" {
			warnings++
		}
	}
	fmt.Fprintf(r.bio.out(), "PASS %s (warnings: %d; report: %s)\n", label, warnings, layout.displayPath(reportPath))
	return nil
}

// redlineCheck 对两个 EPUB 跑全项 redline，文本报告写入 reportPath。
// required=false 时只做审阅用途，失败不阻断。
func (r *bookRunner) redlineCheck(layout bookLayout, reportPath, before, after string, required bool) error {
	outcome, runErr := compareRedline(before, after, "all", nil, false, false)
	text := strings.Join(outcome.lines, "\n")
	if text != "" {
		text += "\n"
	}
	if werr := zipfs.WriteFileAtomic(reportPath, []byte(text), 0o644); werr != nil && required {
		return werr
	}
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	failed := runErr != nil || outcome.ExitCode != ExitOK
	if !required {
		return nil
	}
	if failed {
		fmt.Fprintf(r.bio.err(), "FAIL redline (report: %s)\n", layout.displayPath(reportPath))
		_, _ = r.bio.err().Write([]byte(text))
		return errBookStepFailed
	}
	fmt.Fprintf(r.bio.out(), "PASS redline (warnings: 0; report: %s)\n", layout.displayPath(reportPath))
	return nil
}

// printProviderWarnings 打印 provider 报告里各字体的 warning（不阻断）。
func (r *bookRunner) printProviderWarnings(reportPath string) {
	data, err := readSmallFile(reportPath)
	if err != nil {
		return
	}
	var doc struct {
		Facts map[string]json.RawMessage `json:"facts"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return
	}
	var provider struct {
		Fonts []struct {
			Warnings []string `json:"warnings"`
		} `json:"fonts"`
	}
	if json.Unmarshal(doc.Facts["epub.font.subset.providerReport"], &provider) != nil {
		return
	}
	for _, font := range provider.Fonts {
		for _, warning := range font.Warnings {
			fmt.Fprintln(r.bio.out(), "WARNING", warning)
		}
	}
}
