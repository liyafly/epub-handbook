package pipeline

// epub book build / fonts / clean / accept-dist
//
// 逐条移植旧 sh 构建器的安全属性（templates/book-starter/build.sh 现只转发到这里）：构建锁、dist 收据、
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

	"github.com/liyafly/epub-handbook/internal/report"
	"github.com/liyafly/epub-handbook/internal/zipfs"
)

// 书级命令名。
const (
	BookCommandBuild      = "build"
	BookCommandFonts      = "fonts"
	BookCommandClean      = "clean"
	BookCommandAcceptDist = "accept-dist"
	BookCommandPreview    = "preview"
)

// BookRunOptions 是 build / fonts / clean / accept-dist 的输入。
type BookRunOptions struct {
	Dir     string // 书目录；空表示当前目录
	Command string
	// UpdateLock 仅用于 fonts：显式更新字体锁（须先审阅母版变化）。
	UpdateLock bool
	// Variant 非空时 build / accept-dist / preview 只处理该变体（不碰通用版成品与收据）。
	Variant string
	// Open 仅用于 preview：构建后用系统默认程序（或 OpenApp 指定的应用）打开。
	Open    bool
	OpenApp string
}

type bookRunner struct {
	ctx        context.Context
	bio        BookIO
	env        bookEnv
	updateLock bool
	variant    string
	open       bool
	openApp    string
	// steps 记录本次构建各项检查，供 report.html 汇总。
	steps []bookStep
}

// BookRun 执行一条书级命令并返回退出码（0 成功，1 失败，3 用法错误）。
func BookRun(ctx context.Context, opts BookRunOptions, bio BookIO) int {
	switch opts.Command {
	case BookCommandBuild, BookCommandFonts, BookCommandClean, BookCommandAcceptDist, BookCommandPreview:
	default:
		return bookExit(bio, usageErrorf("unknown book command: %s", opts.Command))
	}
	r := &bookRunner{ctx: ctx, bio: bio, env: readBookEnv(), updateLock: opts.UpdateLock, variant: opts.Variant, open: opts.Open, openApp: opts.OpenApp}
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
	case BookCommandPreview:
		return r.preview(layout)
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
	if r.buildLocked(layout) {
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

var reportPrefixes = []string{"font-subset", "font-roles", "nav-audit", "kindle-check", "redline", "previous-dist-redline", "report"}

// isReportFile 判断状态目录里的文件是否是上一轮留下的（含变体后缀的）报告。
func isReportFile(name string) bool {
	for _, prefix := range reportPrefixes {
		if strings.HasPrefix(name, prefix+".") || strings.HasPrefix(name, prefix+"-") {
			return true
		}
	}
	return false
}

func reportFiles(stateDir string) []string {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && isReportFile(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	return names
}

// fontPlan 是某个输出使用的字体配置。
type fontPlan struct {
	Config      string
	LibraryArgs []string
	Jobs        []fontJob
	Has         bool
}

func baseFontUsage(layout bookLayout) string {
	if layout.Kind == layoutBook {
		return layout.Config.Usage
	}
	if layout.FontConfig == "" {
		return ""
	}
	data, err := readSmallFile(layout.FontConfig)
	if err != nil {
		return ""
	}
	var doc struct {
		Usage string `json:"usage"`
	}
	_ = json.Unmarshal(data, &doc)
	return doc.Usage
}

// fontPlanFor 决定输出使用的字体配置：变体可整体覆盖字体任务列表。
func fontPlanFor(layout bookLayout, out outputSpec, scratch string) (fontPlan, error) {
	if out.Fonts == nil {
		if !layout.hasFontConfig() {
			return fontPlan{}, nil
		}
		config, lib, err := prepareFontConfig(layout, scratch)
		if err != nil {
			return fontPlan{}, err
		}
		var jobs []fontJob
		if layout.Kind == layoutBook {
			jobs, err = layout.Config.jobs()
		} else {
			jobs, err = readFontConfigJobs(layout.FontConfig)
		}
		return fontPlan{Config: config, LibraryArgs: lib, Jobs: jobs, Has: true}, err
	}
	fonts := *out.Fonts
	if len(fonts) == 0 {
		return fontPlan{}, nil
	}
	jobs, err := parseFontJobs(fonts)
	if err != nil {
		return fontPlan{}, err
	}
	data, err := json.MarshalIndent(struct {
		Version int               `json:"version"`
		Usage   string            `json:"usage,omitempty"`
		Fonts   []json.RawMessage `json:"fonts"`
	}{1, baseFontUsage(layout), fonts}, "", "  ")
	if err != nil {
		return fontPlan{}, err
	}
	config := filepath.Join(scratch, "fonts.json")
	if err := zipfs.WriteFileAtomic(config, append(data, '\n'), 0o644); err != nil {
		return fontPlan{}, err
	}
	anchor := layout.FontConfig
	if anchor == "" {
		anchor = filepath.Join(layout.BookDir, bookConfigName)
		if layout.Kind == layoutLegacy {
			anchor = filepath.Join(layout.WorkDir, legacyVariantsName)
		}
	}
	var lib []string
	if dir := findSharedFontLibrary(anchor); dir != "" {
		lib = []string{"--library", dir}
	}
	return fontPlan{Config: config, LibraryArgs: lib, Jobs: jobs, Has: true}, nil
}

// prepareTree 复制源树 → 套用覆盖层 → 按字体锁注入库字体，返回待打包的树。
// build 与 preview 共用。
func (r *bookRunner) prepareTree(layout bookLayout, out outputSpec, overlayFiles []string, dir string) (string, fontPlan, error) {
	srcDir := filepath.Join(dir, "src")
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
		return "", fontPlan{}, err
	}
	if out.Overlay != "" {
		if err := applyOverlay(out.Overlay, overlayFiles, srcDir); err != nil {
			return "", fontPlan{}, err
		}
	}
	plan, err := fontPlanFor(layout, out, dir)
	if err != nil {
		return "", fontPlan{}, err
	}
	if slices.ContainsFunc(plan.Jobs, func(j fontJob) bool { return j.From != "" }) {
		argv := []string{r.env.fontBin(), "inject", "--config", plan.Config, "--tree", srcDir}
		argv = append(argv, plan.LibraryArgs...)
		res, err := runProvider(r.ctx, r.bio, argv)
		if err != nil {
			return "", fontPlan{}, err
		}
		if err := zipfs.WriteFileAtomic(filepath.Join(dir, "font-inject.json"), res.Stdout, 0o644); err != nil {
			return "", fontPlan{}, err
		}
		if res.ExitCode != 0 {
			return "", fontPlan{}, fmt.Errorf("epub-font inject exited with status %d", res.ExitCode)
		}
	}
	return srcDir, plan, nil
}

// candidate 是一个输出在构建临时目录里的候选。
type candidate struct {
	out   outputSpec
	dir   string
	full  string
	final string
	plan  fontPlan
}

// buildCandidate 生成候选；gates 为 false 时只打包完整字体版（用作通用版文本基线）。
func (r *bookRunner) buildCandidate(layout bookLayout, c *candidate, overlayFiles []string, gates bool, genericFull string) error {
	if err := zipfs.MkdirAll(c.dir); err != nil {
		return err
	}
	c.full = filepath.Join(c.dir, "full-font.epub")
	c.final = filepath.Join(c.dir, "final.epub")
	srcDir, plan, err := r.prepareTree(layout, c.out, overlayFiles, c.dir)
	if err != nil {
		return err
	}
	c.plan = plan
	if err := zipfs.PackTree(r.ctx, srcDir, c.full, "mimetype"); err != nil {
		return err
	}
	if !gates {
		return nil
	}
	tag := ""
	if c.out.Name != "" {
		tag = " [" + c.out.Name + "]"
	}
	sfx := c.out.suffix()
	subsetReport := filepath.Join(layout.StateDir, "font-subset"+sfx+".json")
	if plan.Has || treeHasFontFiles(srcDir) {
		args := Args{}
		if plan.Config != "" {
			args["font_config"] = plan.Config
		}
		if err := r.capabilityCheck(layout, subsetReport, "epub.font.subset"+tag, Options{
			CapabilityID: "epub.font.subset", InputPath: c.full, OutputPath: c.final, Args: args,
		}); err != nil {
			return err
		}
		r.printProviderWarnings(layout, subsetReport, tag)
		if err := r.fixedRoleFontCheck(layout, filepath.Join(layout.StateDir, "font-roles"+sfx+".json"), tag, c.final); err != nil {
			return err
		}
	} else if err := zipfs.CopyFile(r.ctx, c.full, c.final, 0o644); err != nil {
		return err
	}
	if err := r.capabilityCheck(layout, filepath.Join(layout.StateDir, "nav-audit"+sfx+".json"), "epub.package.nav.audit"+tag, Options{
		CapabilityID: "epub.package.nav.audit", InputPath: c.final,
	}); err != nil {
		return err
	}
	if err := r.redlineCheck(layout, filepath.Join(layout.StateDir, "redline"+sfx+".txt"), "redline"+tag, "all", c.full, c.final, true); err != nil {
		return err
	}
	if c.out.Name != "" && genericFull != "" {
		// 变体正文必须与通用版逐字一致。
		if err := r.redlineCheck(layout, filepath.Join(layout.StateDir, "redline-vs-generic"+sfx+".txt"), "redline text vs generic"+tag, "text", genericFull, c.final, true); err != nil {
			return err
		}
	}
	if err := r.kindleCheck(layout, filepath.Join(layout.StateDir, "kindle-check"+sfx+".json"), tag, c.final, c.out.Name == kindleVariantName); err != nil {
		return err
	}
	output := filepath.Join(layout.DistDir, c.out.File)
	if isFile(output) {
		r.comparePreviousDist(layout, filepath.Join(layout.StateDir, "previous-dist-redline"+sfx+".txt"), tag, output, c.final)
	}
	return nil
}

type distState struct {
	exists bool
	sha    string
}

// preflightOutput 复刻 sh 版对现有 dist 的收据预检（每个输出各用自己的收据）。
func (r *bookRunner) preflightOutput(layout bookLayout, out outputSpec, accept bool) (distState, error) {
	output := filepath.Join(layout.DistDir, out.File)
	info, err := os.Lstat(output)
	if err != nil {
		if accept {
			return distState{}, fmt.Errorf("accept-dist requires an existing dist artifact: %s", output)
		}
		return distState{}, nil
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return distState{}, fmt.Errorf("Refusing to replace a non-regular dist artifact: %s", output)
	}
	if !accept && !isFile(out.Receipt) {
		return distState{}, fmt.Errorf("Refusing to overwrite existing dist without its accepted SHA-256 receipt: %s\n"+
			"Review/import any dist edits into %s, then record the reviewed artifact SHA in %s.", output, layout.EpubDir, out.Receipt)
	}
	sum, err := zipfs.FileSHA256Context(r.ctx, output)
	if err != nil {
		return distState{}, err
	}
	if recorded := readReceipt(out.Receipt); !accept && sum != recorded {
		return distState{}, fmt.Errorf("Refusing to overwrite dist: its SHA-256 differs from the last accepted build.\n"+
			"Artifact: %s\nRecorded: %s\nCurrent:  %s\n"+
			"Review the EPUB changes and merge intended edits into %s before rebuilding.", output, recorded, sum, layout.EpubDir)
	}
	return distState{exists: true, sha: sum}, nil
}

func (r *bookRunner) build(layout bookLayout, command string) (err error) {
	accept := command == BookCommandAcceptDist
	stdout, stderr := r.bio.out(), r.bio.err()
	all := layout.outputs()
	targets := all
	if r.variant != "" {
		v, ok := layout.variant(r.variant)
		if !ok {
			return usageErrorf("unknown variant %q (defined: %s)", r.variant, variantNames(layout))
		}
		targets = []outputSpec{v}
	}
	if err := validateEpubTree(layout); err != nil {
		return err
	}
	overlays, err := validateOverlays(layout, targets)
	if err != nil {
		return err
	}
	if err := zipfs.MkdirAll(layout.DistDir); err != nil {
		return err
	}
	if err := zipfs.MkdirAll(layout.StateDir); err != nil {
		return err
	}
	release, err := r.acquireBuildLock(layout)
	if err != nil {
		return err
	}
	defer release()

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
		for _, name := range reportFiles(layout.StateDir) {
			src := filepath.Join(layout.StateDir, name)
			if copyErr := zipfs.CopyFile(context.WithoutCancel(r.ctx), src, filepath.Join(buildTmp, name), 0o644); copyErr != nil {
				fmt.Fprintf(stderr, "Could not snapshot report; inspect original: %s\n", src)
			}
		}
		fmt.Fprintf(stderr, "Build failed; retained diagnostic files: %s\n", buildTmp)
	}()
	// 先于上面的失败快照执行，让 report.html 也进入失败现场。
	defer func() {
		if buildTmp == "" {
			return
		}
		if path := r.writeReportHTML(layout, succeeded); path != "" {
			fmt.Fprintf(stdout, "Summary: %s\n", layout.displayPath(path))
		}
	}()

	states := make(map[string]distState, len(targets))
	for _, out := range targets {
		st, err := r.preflightOutput(layout, out, accept)
		if err != nil {
			return err
		}
		states[out.Name] = st
	}

	// 现有 dist 通过收据预检之后才清掉上一轮报告；失败现场目录互相独立，不在此清理。
	for _, name := range reportFiles(layout.StateDir) {
		if err := zipfs.Remove(filepath.Join(layout.StateDir, name)); err != nil {
			return err
		}
	}
	buildTmp, err = zipfs.MkdirTemp(layout.StateDir, "build.*")
	if err != nil {
		return err
	}

	r.noteStyleLayers(layout)

	// 阶段一：生成并检查全部候选，此阶段不写 dist。
	generic := &candidate{out: all[0], dir: buildTmp}
	genericIsTarget := targets[0].Name == ""
	var cands []*candidate
	var failures []error
	fail := func(out outputSpec, err error) {
		if ctxErr := r.ctx.Err(); ctxErr != nil {
			failures = append(failures, ctxErr)
			return
		}
		failures = append(failures, fmt.Errorf("%s: %w", out.label(), err))
	}
	if err := r.buildCandidate(layout, generic, overlays[""], genericIsTarget, ""); err != nil {
		fail(generic.out, err)
	} else if genericIsTarget {
		cands = append(cands, generic)
	}
	if isFile(generic.full) {
		for _, out := range targets {
			if out.Name == "" {
				continue
			}
			c := &candidate{out: out, dir: filepath.Join(buildTmp, "variants", out.Name)}
			if err := r.buildCandidate(layout, c, overlays[out.Name], true, generic.full); err != nil {
				fail(out, err)
				continue
			}
			cands = append(cands, c)
		}
	} else if len(targets) > 1 || !genericIsTarget {
		failures = append(failures, errors.New("variants not built: the generic candidate is unavailable"))
	}
	if len(failures) > 0 {
		return fmt.Errorf("%w\n%d of %d outputs failed; nothing was written to %s: %w",
			errBookStepFailed, len(failures), len(targets), layout.DistDir, errors.Join(failures...))
	}

	// 阶段二：全部候选通过后，先逐个做并发修改与 accept 比对，再统一替换。
	finalSHA := make(map[string]string, len(cands))
	for _, c := range cands {
		output := filepath.Join(layout.DistDir, c.out.File)
		st := states[c.out.Name]
		if st.exists {
			if info, lerr := os.Lstat(output); lerr != nil || info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return errors.New("Refusing to replace dist because the previous artifact disappeared during this build.")
			}
			now, shaErr := zipfs.FileSHA256Context(r.ctx, output)
			if shaErr != nil {
				return shaErr
			}
			if now != st.sha {
				return errors.New("Refusing to replace dist because it changed while this build was running.")
			}
		} else if _, lerr := os.Lstat(output); lerr == nil {
			return errors.New("Refusing to replace dist because an artifact appeared while this build was running.")
		}
		if accept {
			// 包括 CSS 与图片在内的所有成员字节都须与源树重建的候选一致；红线本身
			// 查不出遗漏的样式表修改。
			changed, cmpErr := compareArchiveMembers(r.ctx, output, c.final)
			if cmpErr != nil {
				return cmpErr
			}
			if len(changed) > 0 {
				return fmt.Errorf("Dist still differs from rebuilt source (%s); merge/review first: %s", c.out.label(), strings.Join(changed, ", "))
			}
			finalSHA[c.out.Name], err = zipfs.FileSHA256Context(r.ctx, output)
		} else {
			finalSHA[c.out.Name], err = zipfs.FileSHA256Context(r.ctx, c.final)
		}
		if err != nil {
			return err
		}
	}

	// 先在内存里备好交付记录，再替换成品。
	logBytes, err := buildDeliveryLog(r.ctx, layout, cands, finalSHA)
	if err != nil {
		return err
	}
	for _, c := range cands {
		output := filepath.Join(layout.DistDir, c.out.File)
		if !accept {
			if err := zipfs.ReplaceFile(c.final, output); err != nil {
				return err
			}
		}
		if err := zipfs.WriteFileAtomic(c.out.Receipt, []byte(finalSHA[c.out.Name]+"\n"), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Built %s\n", output)
		if hint := sizeHint(output); hint != "" {
			fmt.Fprintln(stdout, "WARN", hint)
			r.addStep(bookStep{Status: "WARN", Label: "size", Lines: []string{hint}})
		}
	}
	if err := zipfs.WriteFileAtomic(layout.DeliveryLog, logBytes, 0o644); err != nil {
		return err
	}
	succeeded = true
	return nil
}

func variantNames(layout bookLayout) string {
	names := make([]string, 0, len(layout.Variants))
	for _, v := range layout.Variants {
		names = append(names, v.Name)
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
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

func buildDeliveryLog(ctx context.Context, layout bookLayout, cands []*candidate, shas map[string]string) ([]byte, error) {
	sourceRev, handbookRev := bookSourceRevision(ctx, layout), handbookRevision(ctx)
	var rows strings.Builder
	for _, c := range cands {
		fonts := make([]string, 0, len(c.plan.Jobs))
		for _, job := range c.plan.Jobs {
			if job.From != "" {
				fonts = append(fonts, job.Target+"="+job.SHA256)
			}
		}
		fontLock := "source-tree"
		if len(fonts) > 0 {
			fontLock = strings.Join(fonts, ";")
		}
		row := deliveryRow(sourceRev, handbookRev, shas[c.out.Name], fontLock)
		if c.out.Name != "" {
			// 变体行在末尾多一列 variant:<名字>；通用版行保持原有五列。
			row = strings.TrimSuffix(row, "\n") + "\tvariant:" + c.out.Name + "\n"
		}
		rows.WriteString(row)
	}
	existing := []byte(deliveryHeader)
	if layout.Kind == layoutLegacy {
		if data, err := readSmallFile(layout.DeliveryLog); err == nil {
			existing = data
		}
		if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
			existing = append(existing, '\n')
		}
	}
	return append(existing, rows.String()...), nil
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
		lines := findingLines(outcome.Envelope.Findings, "error", 0)
		if runErr != nil {
			lines = append(lines, runErr.Error())
		}
		printIndented(r.bio.err(), lines)
		r.addStep(bookStep{Status: "FAIL", Label: label, Report: layout.displayPath(reportPath), Lines: lines})
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
	var lines []string
	if opts.CapabilityID != "epub.font.subset" { // 字体警告另由 printProviderWarnings 汇总成一行
		lines = warnStepLines(outcome.Envelope)
	}
	printIndented(r.bio.out(), lines)
	r.addStep(bookStep{Status: "PASS", Label: label, Report: layout.displayPath(reportPath), Lines: lines})
	return nil
}

// kindleVariantName 是唯一一个让 Kindle 检查的 error 阻断构建的变体名。
const kindleVariantName = "kindle"

// kindleCheck 对一个产物跑 epub.kindle.compatibility.check：终端只打一行各级别数量，
// 明细进 report.html。blocking 为 true（名为 kindle 的变体）时 error 级 finding 使构建失败；
// 其他产物只提示。book.json 的 readers 仍只是备注，不影响这里。
func (r *bookRunner) kindleCheck(layout bookLayout, reportPath, tag, epubPath string, blocking bool) error {
	label := "kindle check" + tag
	outcome, runErr := Run(r.ctx, Options{CapabilityID: "epub.kindle.compatibility.check", InputPath: epubPath})
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	display := layout.displayPath(reportPath)
	if runErr != nil {
		lines := []string{"check did not run: " + runErr.Error()}
		status := "WARN"
		if blocking {
			status = "FAIL"
		}
		fmt.Fprintf(r.bio.out(), "%s %s (report: %s)\n", status, label, display)
		printIndented(r.bio.out(), lines)
		r.addStep(bookStep{Status: status, Label: label, Report: display, Lines: lines})
		if blocking {
			return errBookStepFailed
		}
		return nil
	}
	data, err := MarshalEnvelope(outcome.Envelope)
	if err != nil {
		return err
	}
	if err := zipfs.WriteFileAtomic(reportPath, data, 0o644); err != nil {
		return err
	}
	var own []report.Finding
	counts := map[string]int{}
	for _, f := range outcome.Envelope.Findings {
		if strings.HasPrefix(f.ID, "upstream.") {
			continue // nav.audit 的转述，已由导航审计一步报告。
		}
		own = append(own, f)
		counts[f.Level]++
	}
	var lines []string
	for _, level := range []string{"error", "warn", "info"} {
		lines = append(lines, findingLines(own, level, maxFindingLines)...)
	}
	summary := fmt.Sprintf("error: %d, warn: %d, info: %d", counts["error"], counts["warn"], counts["info"])
	status := "PASS"
	switch {
	case blocking && counts["error"] > 0:
		status = "FAIL"
	case counts["error"] > 0 || counts["warn"] > 0:
		status = "WARN"
		if !blocking && counts["error"] > 0 {
			summary += "; blocks only the kindle variant"
		}
	}
	out := r.bio.out()
	if status == "FAIL" {
		out = r.bio.err()
	}
	fmt.Fprintf(out, "%s %s (%s; report: %s)\n", status, label, summary, display)
	if status == "FAIL" {
		printIndented(out, findingLines(own, "error", maxFindingLines))
	}
	r.addStep(bookStep{Status: status, Label: label, Report: display, Lines: lines})
	if status == "FAIL" {
		return errBookStepFailed
	}
	return nil
}

// fixedRoleFontCheck 列出指定了字体的文字里、这些字体都没有的字（SPEC §8）。
// 只警告不阻断：阅读器的回退字体可能显示正常，要在真实阅读器里看。
func (r *bookRunner) fixedRoleFontCheck(layout bookLayout, reportPath, tag, epubPath string) error {
	label := "fixed-role fonts" + tag
	warn := func(lines []string) error {
		fmt.Fprintf(r.bio.out(), "WARN %s (report: %s)\n", label, layout.displayPath(reportPath))
		printIndented(r.bio.out(), lines)
		r.addStep(bookStep{Status: "WARN", Label: label, Report: layout.displayPath(reportPath), Lines: lines})
		return nil
	}
	res, err := runProvider(r.ctx, r.bio, []string{r.env.fontBin(), "coverage", epubPath, "--fixed-role-check"})
	if err != nil {
		if ctxErr := r.ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return warn([]string{"check did not run: " + err.Error()})
	}
	if res.ExitCode != 0 {
		return warn([]string{fmt.Sprintf("check did not run: epub-font coverage exited with status %d", res.ExitCode)})
	}
	if err := zipfs.WriteFileAtomic(reportPath, res.Stdout, 0o644); err != nil {
		return err
	}
	lines, err := fixedRoleWarningLines(res.Stdout)
	if err != nil {
		return warn([]string{"check did not run: " + err.Error()})
	}
	if len(lines) > 0 {
		return warn(lines)
	}
	fmt.Fprintf(r.bio.out(), "PASS %s (report: %s)\n", label, layout.displayPath(reportPath))
	r.addStep(bookStep{Status: "PASS", Label: label, Report: layout.displayPath(reportPath)})
	return nil
}

// redlineCheck 对两个 EPUB 跑 redline（check 为检查项），文本报告写入 reportPath。
// required=false 时只做审阅用途，失败不阻断。
func (r *bookRunner) redlineCheck(layout bookLayout, reportPath, label, check, before, after string, required bool) error {
	outcome, runErr := compareRedline(before, after, check, nil, false, false)
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
	if !required {
		return nil
	}
	if runErr != nil || outcome.ExitCode != ExitOK {
		fmt.Fprintf(r.bio.err(), "FAIL %s (report: %s)\n", label, layout.displayPath(reportPath))
		lines := outcome.lines
		if runErr != nil {
			lines = append(slices.Clone(lines), runErr.Error())
		}
		shown := lines
		if len(shown) > maxRedlineLines {
			shown = append(slices.Clone(shown[:maxRedlineLines]), fmt.Sprintf("… %d more lines (see report)", len(lines)-maxRedlineLines))
		}
		printIndented(r.bio.err(), shown)
		r.addStep(bookStep{Status: "FAIL", Label: label, Report: layout.displayPath(reportPath), Lines: lines})
		return errBookStepFailed
	}
	fmt.Fprintf(r.bio.out(), "PASS %s (warnings: 0; report: %s)\n", label, layout.displayPath(reportPath))
	r.addStep(bookStep{Status: "PASS", Label: label, Report: layout.displayPath(reportPath)})
	return nil
}

// maxRedlineLines 是红线失败时终端最多显示的行数；完整内容在报告里。
const maxRedlineLines = 20

// comparePreviousDist 把候选与上一版成品比对（只供审阅，不阻断），终端只打一行。
func (r *bookRunner) comparePreviousDist(layout bookLayout, reportPath, tag, previous, candidate string) {
	outcome, runErr := compareRedline(previous, candidate, "all", nil, false, false)
	text := strings.Join(outcome.lines, "\n")
	if text != "" {
		text += "\n"
	}
	_ = zipfs.WriteFileAtomic(reportPath, []byte(text), 0o644)
	label := "vs previous " + filepath.Base(previous) + tag
	step := bookStep{Status: "INFO", Label: label, Report: layout.displayPath(reportPath)}
	if runErr != nil || outcome.ExitCode != ExitOK {
		step.Lines = []string{"text, structure or metadata changed since the last build (review only; does not block)"}
	} else {
		step.Lines = []string{"no red-line differences from the last build"}
	}
	fmt.Fprintf(r.bio.out(), "INFO %s: %s (report: %s)\n", label, step.Lines[0], step.Report)
	r.addStep(step)
}

// standardStyleLayers 是 SPEC-实现约束 §7 的 CSS 分层文件名，加上 reader.css 与 decoration.css。
func standardStyleLayers() []string {
	return []string{"fonts.css", "base.css", "design.css", "notes.css", "effects.css", "decoration.css",
		"literary.css", "media.css", "vertical.css", "poster.css", "reader.css"}
}

// noteStyleLayers 对不在分层表里的样式文件给一条 INFO（不阻断）。导入书原有的样式表不必改名；
// 这条提示是为了让新加的样式落到对应分层，而不是另起一个文件。
func (r *bookRunner) noteStyleLayers(layout bookLayout) {
	var others []string
	_ = filepath.WalkDir(layout.EpubDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".css") {
			return nil
		}
		if !slices.Contains(standardStyleLayers(), strings.ToLower(d.Name())) {
			if rel, rerr := filepath.Rel(layout.EpubDir, p); rerr == nil {
				others = append(others, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if len(others) == 0 {
		return
	}
	shown := others
	if len(shown) > maxFindingLines {
		shown = shown[:maxFindingLines]
	}
	text := strings.Join(shown, ", ")
	if extra := len(others) - len(shown); extra > 0 {
		text += fmt.Sprintf(" and %d more", extra)
	}
	step := bookStep{Status: "INFO", Label: "style layers", Lines: []string{
		"outside the standard layers (SPEC-实现约束 §7): " + text,
		"imported style sheets can stay; put new rules in the matching layer instead of a new file",
	}}
	fmt.Fprintf(r.bio.out(), "INFO %s: %s\n", step.Label, step.Lines[0])
	r.addStep(step)
}

// printProviderWarnings 汇总 provider 报告里各字体的 warning（不阻断）。
func (r *bookRunner) printProviderWarnings(layout bookLayout, reportPath, tag string) {
	data, err := readSmallFile(reportPath)
	if err != nil {
		return
	}
	lines := fontWarningSummary(data)
	if len(lines) == 0 {
		return
	}
	for _, line := range lines {
		fmt.Fprintln(r.bio.out(), "WARN", line)
	}
	r.addStep(bookStep{Status: "WARN", Label: "fonts" + tag, Report: layout.displayPath(reportPath), Lines: lines})
}
