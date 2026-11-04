package pipeline

// 书级工作流用到的外部工具：epub-font provider 与 git，全部经 internal/extern。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/liyafly/epub-handbook/internal/extern"
)

// BookIO 是书级命令的输出句柄。
type BookIO struct {
	Out io.Writer
	Err io.Writer
}

func (b BookIO) out() io.Writer {
	if b.Out == nil {
		return io.Discard
	}
	return b.Out
}

func (b BookIO) err() io.Writer {
	if b.Err == nil {
		return io.Discard
	}
	return b.Err
}

// ensureLocalBin 把 uv tool 的默认安装目录追加到 PATH 末尾（调用者 PATH 优先），
// 复刻 sh 版行为：默认 $HOME/.local/bin（Windows 为 %USERPROFILE%\.local\bin）；
// EPUB_LOCAL_BIN=<目录> 指定位置，显式设空则保留原始 PATH。
func ensureLocalBin(env bookEnv) {
	dir := env.LocalBin
	if !env.LocalBinSet {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return
		}
		dir = filepath.Join(home, ".local", "bin")
	}
	if dir == "" || !isDir(dir) {
		return
	}
	current := os.Getenv("PATH")
	for _, part := range filepath.SplitList(current) {
		if filepath.Clean(part) == filepath.Clean(dir) {
			return
		}
	}
	if current == "" {
		_ = os.Setenv("PATH", dir)
		return
	}
	_ = os.Setenv("PATH", current+string(os.PathListSeparator)+dir)
}

func (e bookEnv) fontBin() string {
	if e.FontBin != "" {
		return e.FontBin
	}
	return "epub-font"
}

// errProviderMissing 给出面向用户的 provider 缺失说明。
func providerMissingError(tool string) error {
	return fmt.Errorf("font provider not found: %s (install it with `uv tool install --editable tools-font/epub-font`, or set EPUB_FONT_BIN to its executable path)", tool)
}

// runProvider 运行 epub-font provider，provider 自己的 stderr 原样转给用户。
func runProvider(ctx context.Context, bio BookIO, argv []string) (extern.CmdResult, error) {
	res, err := extern.Run(ctx, "", argv)
	if err != nil {
		if errors.Is(err, extern.ErrToolMissing) {
			return res, providerMissingError(argv[0])
		}
		return res, err
	}
	if len(res.Stderr) > 0 {
		_, _ = bio.err().Write(res.Stderr)
	}
	return res, nil
}

// findSharedFontLibrary 复刻 epub-font 的向上查找：对配置文件路径的每一级祖先，
// 取其父目录下的 fonts/。新布局以 book.json 的位置为起点，让共享库与书同级即可。
// EPUB_FONT_LIBRARY 由 provider 自己处理，此处返回空串。
func findSharedFontLibrary(configPath string) string {
	if os.Getenv("EPUB_FONT_LIBRARY") != "" {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(configPath)
	if err != nil {
		resolved = configPath
	}
	dir := filepath.Dir(resolved)
	for {
		candidate := filepath.Join(filepath.Dir(dir), "fonts")
		if isDir(candidate) {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// fontLibraryDir 返回导入时登记字体的共享库目录。
func fontLibraryDir(destParent string) (string, error) {
	if selected := os.Getenv("EPUB_FONT_LIBRARY"); selected != "" {
		if strings.HasPrefix(selected, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				selected = filepath.Join(home, strings.TrimLeft(selected[1:], `/\`))
			}
		}
		return filepath.Abs(selected)
	}
	return filepath.Abs(filepath.Join(destParent, "fonts"))
}

// gitRevision 复刻交付记录里“书源 commit / 手册 commit”的取值：
// 无 HEAD → uncommitted-dirty；有未提交修改（交付记录.tsv 自身除外）→ 加 -dirty。
func gitRevision(ctx context.Context, dir string) string {
	head, err := extern.Run(ctx, "", []string{"git", "-C", dir, "rev-parse", "--verify", "HEAD"})
	if err != nil {
		return "git-unavailable"
	}
	if head.ExitCode != 0 {
		return "uncommitted-dirty"
	}
	status, err := extern.Run(ctx, "", []string{"git", "-C", dir, "status", "--porcelain=v1", "-z", "--untracked-files=normal"})
	if err != nil || status.ExitCode != 0 {
		return strings.TrimSpace(string(head.Stdout)) + "-status-unavailable"
	}
	// NUL 分隔的路径是字面值；重命名/复制条目带有第二个路径。
	entries := strings.Split(string(status.Stdout), "\x00")
	var changed []string
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if entry == "" {
			continue
		}
		if len(entry) > 3 {
			changed = append(changed, entry[3:])
		}
		if len(entry) >= 2 && strings.ContainsAny(entry[:2], "RC") && i+1 < len(entries) {
			i++
			changed = append(changed, entries[i])
		}
	}
	dirty := slices.ContainsFunc(changed, func(name string) bool { return path.Base(name) != "交付记录.tsv" })
	revision := strings.TrimSpace(string(head.Stdout))
	if dirty {
		revision += "-dirty"
	}
	return revision
}

// bookSourceRevision 取书源的修订：新布局只有书根自带 .git 才算；
// 旧布局沿用 git -C 的原语义。
func bookSourceRevision(ctx context.Context, layout bookLayout) string {
	if layout.Kind == layoutBook {
		if _, err := os.Stat(filepath.Join(layout.BookDir, ".git")); err != nil {
			return "no-git"
		}
	}
	return gitRevision(ctx, layout.BookDir)
}

// handbookRevision 取手册（工具）的修订：优先用检出目录的 git，
// 单文件发行二进制则回落到构建信息。
func handbookRevision(ctx context.Context) string {
	if root, err := FindRepoRoot(); err == nil && root != "" {
		if rev := gitRevision(ctx, root); !strings.HasPrefix(rev, "git-unavailable") {
			return rev
		}
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		revision, modified := "", false
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.modified":
				modified = setting.Value == "true"
			}
		}
		if revision != "" {
			if modified {
				revision += "-dirty"
			}
			return revision
		}
	}
	return "unknown"
}

// handbookShortRevision 给 preset 注释用。
func handbookShortRevision(ctx context.Context, root string) string {
	if root != "" {
		res, err := extern.Run(ctx, "", []string{"git", "-C", root, "rev-parse", "--short", "HEAD"})
		if err == nil && res.ExitCode == 0 {
			if rev := strings.TrimSpace(string(res.Stdout)); rev != "" {
				return rev
			}
		}
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && len(setting.Value) >= 7 {
				return setting.Value[:7]
			}
		}
	}
	return "unknown"
}

// deliveryRow 生成交付记录一行（Python isoformat 风格的 UTC 时间戳）。
func deliveryRow(sourceRev, handbookRev, sha, fonts string) string {
	stamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000-07:00")
	return strings.Join([]string{stamp, sourceRev, handbookRev, sha, fonts}, "\t") + "\n"
}

const deliveryHeader = "日期\t书源commit\t手册commit\tdist sha256\t字体库版本\n"

// providerJSON 解码 provider stdout 中的 JSON 文档。
func providerJSON(stdout []byte, target any) error {
	return json.Unmarshal(bytes.TrimSpace(stdout), target)
}

// externRun runs an external command and explains a missing executable.
func externRun(ctx context.Context, argv []string) (extern.CmdResult, error) {
	res, err := extern.Run(ctx, "", argv)
	if errors.Is(err, extern.ErrToolMissing) {
		return res, fmt.Errorf("%s was not found on PATH; install it or omit the option that needs it", argv[0])
	}
	return res, err
}
