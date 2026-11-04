package pipeline

// epub book preview：快速打包用于在阅读器里看效果。
//
// 复制源 → 套用变体覆盖层 → 注入库字体（不子集化）→ 打包到状态目录。
// 不跑任何检查、不写收据和交付记录，也从不触碰 dist / out。

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/liyafly/epub-handbook/internal/zipfs"
)

func (r *bookRunner) preview(layout bookLayout) error {
	out := layout.outputs()[0]
	fileName := "preview.epub"
	if r.variant != "" {
		v, ok := layout.variant(r.variant)
		if !ok {
			return usageErrorf("unknown variant %q (defined: %s)", r.variant, variantNames(layout))
		}
		out, fileName = v, "preview-"+v.Name+".epub"
	}
	if err := validateEpubTree(layout); err != nil {
		return err
	}
	overlays, err := validateOverlays(layout, []outputSpec{out})
	if err != nil {
		return err
	}
	if err := zipfs.MkdirAll(layout.StateDir); err != nil {
		return err
	}
	tmp, err := zipfs.MkdirTemp(layout.StateDir, "preview-tmp.*")
	if err != nil {
		return err
	}
	defer func() { _ = zipfs.RemoveAll(tmp) }()

	srcDir, _, err := r.prepareTree(layout, out, overlays[out.Name], tmp)
	if err != nil {
		return err
	}
	candidate := filepath.Join(tmp, "preview.epub")
	if err := zipfs.PackTree(r.ctx, srcDir, candidate, "mimetype"); err != nil {
		return err
	}
	_ = zipfs.Chmod(candidate, 0o644)
	dest := filepath.Join(layout.StateDir, fileName)
	if err := zipfs.ReplaceFile(candidate, dest); err != nil {
		return err
	}
	fmt.Fprintf(r.bio.out(), "Preview written (UNCHECKED: no font subsetting, audit, redline or receipt; dist is untouched): %s\n", dest)
	fmt.Fprintln(r.bio.out(), "Run `epub book build` before delivery.")
	if r.open {
		if err := openWithSystem(r.ctx, dest, r.openApp); err != nil {
			fmt.Fprintf(r.bio.err(), "Could not open the preview (the file is still at %s): %v\n", dest, err)
		}
	}
	return nil
}

// openWithSystem 用系统默认程序或指定应用打开文件；进程经 internal/extern 启动。
func openWithSystem(ctx context.Context, path, app string) error {
	var argv []string
	switch runtime.GOOS {
	case "darwin":
		argv = []string{"open"}
		if app != "" {
			argv = append(argv, "-a", app)
		}
		argv = append(argv, path)
	case "windows":
		argv = []string{"cmd", "/c", "start", ""}
		if app != "" {
			argv = append(argv, app)
		}
		argv = append(argv, path)
	default:
		if app != "" {
			argv = []string{app, path}
		} else {
			argv = []string{"xdg-open", path}
		}
	}
	res, err := externRun(ctx, argv)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("%s exited with status %d: %s", argv[0], res.ExitCode, string(res.Stderr))
	}
	return nil
}
