package pipeline

// 书级工作区：布局检测、book.json 与旧版 fonts.json 读取。
//
// 两种布局都由同一个构建器处理：
//   - 新布局：书目录含 book.json；epub/、out/、.epub/ 直接位于书根。
//   - 旧布局：01 源文件/、03 制作工作区/epub/、fonts.json、交付记录.tsv，
//     以及转发用的书根 build.sh（其中的 EPUB_OUTPUT_NAME / EPUB_LEGACY_LAYOUT
//     由本文件按行读取，不执行脚本）。

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// bookConfigName 是新布局的唯一配置文件。
const bookConfigName = "book.json"

const (
	layoutBook   = "book"
	layoutLegacy = "legacy"

	defaultOutputName = "book.epub"
)

// fontJob 是 fonts.json / book.json 中的一项字体任务。仅解析本层需要的键，
// 完整对象原样交给 epub-font provider。
type fontJob struct {
	Target string `json:"target"`
	From   string `json:"from"`
	SHA256 string `json:"sha256"`
}

type bookOriginal struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

// bookConfig 是 book.json。readers / variants 为后续阶段预留，本阶段只原样保留。
type bookConfig struct {
	Version      int               `json:"version"`
	Title        string            `json:"title,omitempty"`
	Output       string            `json:"output,omitempty"`
	LegacyLayout bool              `json:"legacyLayout,omitempty"`
	Usage        string            `json:"usage,omitempty"`
	Fonts        []json.RawMessage `json:"fonts"`
	Original     *bookOriginal     `json:"original,omitempty"`
	// Readers 只是文档性质的目标阅读器清单，不会据此自动生成任何东西。
	Readers  []string                 `json:"readers,omitempty"`
	Variants map[string]variantConfig `json:"variants,omitempty"`
}

func (c bookConfig) jobs() ([]fontJob, error) {
	return parseFontJobs(c.Fonts)
}

func parseFontJobs(raw []json.RawMessage) ([]fontJob, error) {
	jobs := make([]fontJob, 0, len(raw))
	for i, item := range raw {
		var job fontJob
		if err := json.Unmarshal(item, &job); err != nil {
			return nil, fmt.Errorf("fonts[%d]: %w", i, err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// bookEnv 汇集构建器读取的环境变量，便于测试隔离。
type bookEnv struct {
	OutputName   string
	LegacyLayout string
	FontBin      string
	LocalBin     string
	LocalBinSet  bool
}

func readBookEnv() bookEnv {
	localBin, set := os.LookupEnv("EPUB_LOCAL_BIN")
	return bookEnv{
		OutputName:   os.Getenv("EPUB_OUTPUT_NAME"),
		LegacyLayout: os.Getenv("EPUB_LEGACY_LAYOUT"),
		FontBin:      os.Getenv("EPUB_FONT_BIN"),
		LocalBin:     localBin,
		LocalBinSet:  set,
	}
}

// bookLayout 描述一个已解析的书级工作区。
type bookLayout struct {
	Kind         string
	BookDir      string
	WorkDir      string
	EpubDir      string
	DistDir      string
	StateDir     string
	DeliveryLog  string
	OutputName   string
	LegacyLayout bool // 非 OEBPS：原样打包已审计的原有布局
	// FontConfig 是旧版 fonts.json 路径（存在时）；新布局为 book.json 中 fonts 非空时的 book.json。
	FontConfig string
	Config     bookConfig // 新布局的 book.json
	// Variants 是已校验的变体（阅读器版本 / 版本差异），按名字排序。
	Variants []variantSpec
	Volumes  []string // 多册工作区：各册目录（含自己的 build.sh）
}

// displayPath 返回相对书根的路径，用于简洁输出。
func (l bookLayout) displayPath(p string) string {
	if rel, err := filepath.Rel(l.BookDir, p); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return p
}

func (l bookLayout) outputPath() string { return filepath.Join(l.DistDir, l.OutputName) }

func (l bookLayout) receiptPath() string { return filepath.Join(l.StateDir, "dist-sha256") }

func (l bookLayout) hasFontConfig() bool { return l.FontConfig != "" }

// validOutputName 复刻 sh 版的文件名校验，并补上 Windows 的反斜杠。
func validOutputName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.ContainsAny(name, "/\\\x00")
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

// resolveBookLayout 自动识别布局。dirArg 为空表示当前目录。
func resolveBookLayout(dirArg string, env bookEnv) (bookLayout, error) {
	if dirArg == "" {
		dirArg = "."
	}
	abs, err := filepath.Abs(dirArg)
	if err != nil {
		return bookLayout{}, fmt.Errorf("resolve book directory: %w", err)
	}
	if !isDir(abs) {
		return bookLayout{}, usageErrorf("book directory not found: %s", dirArg)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}

	if isFile(filepath.Join(abs, bookConfigName)) {
		return resolveNewLayout(abs, env)
	}
	return resolveLegacyLayout(abs, env)
}

func resolveNewLayout(bookDir string, env bookEnv) (bookLayout, error) {
	data, err := readSmallFile(filepath.Join(bookDir, bookConfigName))
	if err != nil {
		return bookLayout{}, err
	}
	var cfg bookConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return bookLayout{}, fmt.Errorf("%s is not valid JSON: %w", bookConfigName, err)
	}
	if cfg.Version != 1 {
		return bookLayout{}, fmt.Errorf("%s: version must be 1", bookConfigName)
	}
	if _, err := cfg.jobs(); err != nil {
		return bookLayout{}, fmt.Errorf("%s: %w", bookConfigName, err)
	}
	layout := bookLayout{
		Kind: layoutBook, BookDir: bookDir, WorkDir: bookDir,
		EpubDir:      filepath.Join(bookDir, "epub"),
		DistDir:      filepath.Join(bookDir, "out"),
		StateDir:     filepath.Join(bookDir, ".epub"),
		DeliveryLog:  filepath.Join(bookDir, ".epub", "delivery.tsv"),
		OutputName:   cmp.Or(cfg.Output, defaultOutputName),
		LegacyLayout: cfg.LegacyLayout,
		Config:       cfg,
	}
	if env.OutputName != "" {
		layout.OutputName = env.OutputName
	}
	if len(cfg.Fonts) > 0 {
		layout.FontConfig = filepath.Join(bookDir, bookConfigName)
	}
	layout.Variants, err = buildVariantSpecs(cfg.Variants, bookDir, layout.OutputName)
	if err != nil {
		return bookLayout{}, fmt.Errorf("%s: %w", bookConfigName, err)
	}
	return layout, nil
}

func resolveLegacyLayout(dir string, env bookEnv) (bookLayout, error) {
	layout := bookLayout{Kind: layoutLegacy, BookDir: dir}
	switch {
	case isFile(filepath.Join(dir, "mimetype")):
		// 直接指向源树（转发脚本放在 epub/ 内的旧用法，或孤立源树）。
		layout.EpubDir = dir
		if filepath.Base(filepath.Dir(dir)) == "03 制作工作区" {
			layout.WorkDir = filepath.Dir(dir)
			layout.BookDir = filepath.Dir(layout.WorkDir)
		} else {
			layout.WorkDir = dir
		}
	default:
		layout.WorkDir = filepath.Join(dir, "03 制作工作区")
		if isDir(filepath.Join(dir, "epub")) {
			layout.WorkDir = dir
		}
		layout.EpubDir = filepath.Join(layout.WorkDir, "epub")
	}
	layout.DistDir = filepath.Join(layout.WorkDir, "dist")
	layout.StateDir = filepath.Join(layout.WorkDir, ".pipeline")
	layout.DeliveryLog = filepath.Join(layout.WorkDir, "交付记录.tsv")

	// 转发脚本里的 export 优先于环境变量，等同于 sh 版里脚本先 export 再 exec。
	outputName, legacyFlag := env.OutputName, env.LegacyLayout
	for _, candidate := range []string{filepath.Join(dir, "build.sh"), filepath.Join(layout.EpubDir, "build.sh")} {
		settings, err := readForwardingSettings(candidate)
		if err != nil {
			return bookLayout{}, err
		}
		if settings.OutputName != "" {
			outputName = settings.OutputName
		}
		if settings.LegacyLayout != "" {
			legacyFlag = settings.LegacyLayout
		}
	}
	layout.OutputName = cmp.Or(outputName, defaultOutputName)
	layout.LegacyLayout = legacyFlag == "1"

	if !isFile(filepath.Join(layout.EpubDir, "mimetype")) && isDir(layout.WorkDir) {
		// 多册工作区：每册自带 build.sh 的子目录。
		entries, err := os.ReadDir(layout.WorkDir)
		if err != nil {
			return bookLayout{}, err
		}
		for _, entry := range entries {
			if entry.IsDir() && isFile(filepath.Join(layout.WorkDir, entry.Name(), "build.sh")) {
				layout.Volumes = append(layout.Volumes, filepath.Join(layout.WorkDir, entry.Name()))
			}
		}
		slices.Sort(layout.Volumes)
	}

	for _, candidate := range []string{filepath.Join(layout.WorkDir, "fonts.json"), filepath.Join(layout.BookDir, "fonts.json")} {
		if isFile(candidate) {
			layout.FontConfig = candidate
			break
		}
	}
	// 旧布局的变体配置放在制作区的 variants.json，不参与布局识别。
	if data, err := readSmallFile(filepath.Join(layout.WorkDir, legacyVariantsName)); err == nil {
		var doc struct {
			Variants map[string]variantConfig `json:"variants"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			return bookLayout{}, fmt.Errorf("%s is not valid JSON: %w", legacyVariantsName, err)
		}
		layout.Variants, err = buildVariantSpecs(doc.Variants, layout.WorkDir, layout.OutputName)
		if err != nil {
			return bookLayout{}, fmt.Errorf("%s: %w", legacyVariantsName, err)
		}
	}
	return layout, nil
}

type forwardingSettings struct {
	OutputName   string
	LegacyLayout string
}

// readForwardingSettings 按行读取转发脚本里 `[export] NAME=value` 的赋值。
// 只接受字面量（单引号、双引号、无引号）；含 shell 展开的值直接报错，
// 不去执行脚本。文件不存在时返回零值。LF 与 CRLF 皆可。
func readForwardingSettings(path string) (forwardingSettings, error) {
	data, err := readSmallFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return forwardingSettings{}, nil
		}
		return forwardingSettings{}, err
	}
	var settings forwardingSettings
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimSuffix(scanner.Text(), "\r"))
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		name, value, ok := strings.Cut(line, "=")
		if !ok || (name != "EPUB_OUTPUT_NAME" && name != "EPUB_LEGACY_LAYOUT") {
			continue
		}
		literal, err := shellLiteral(value)
		if err != nil {
			return forwardingSettings{}, fmt.Errorf("%s: %s: %w; set it in the environment or migrate to book.json", path, name, err)
		}
		if name == "EPUB_OUTPUT_NAME" {
			settings.OutputName = literal
		} else {
			settings.LegacyLayout = literal
		}
	}
	return settings, scanner.Err()
}

func shellLiteral(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch {
	case strings.HasPrefix(value, "'"):
		end := strings.IndexByte(value[1:], '\'')
		if end < 0 {
			return "", errors.New("unterminated quote")
		}
		return value[1 : 1+end], nil
	case strings.HasPrefix(value, `"`):
		end := strings.IndexByte(value[1:], '"')
		if end < 0 {
			return "", errors.New("unterminated quote")
		}
		inner := value[1 : 1+end]
		if strings.ContainsAny(inner, "$`\\") {
			return "", errors.New("shell expansion is not supported")
		}
		return inner, nil
	default:
		if i := strings.IndexAny(value, " \t#;"); i >= 0 {
			value = value[:i]
		}
		if strings.ContainsAny(value, "$`\\") {
			return "", errors.New("shell expansion is not supported")
		}
		return value, nil
	}
}

func readSmallFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 16<<20))
	if err != nil {
		return nil, err
	}
	return bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), nil
}

// legacyFontConfig 读取旧版 fonts.json 的任务列表。
func readFontConfigJobs(path string) ([]fontJob, error) {
	data, err := readSmallFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Fonts []json.RawMessage `json:"fonts"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	return parseFontJobs(doc.Fonts)
}
