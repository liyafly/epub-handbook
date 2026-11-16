package pipeline

// 变体换图片格式。
//
// 覆盖层里的图片 D/x.jpg 在源树没有同名文件、却恰有一个同目录同名不同扩展名的
// 图片 D/x.webp 时，视为「换格式」：变体树里删去 D/x.webp、放入 D/x.jpg，并把引用
// 改过去。引用只改扩展名那几个字节（同目录同名，相对路径的其余部分不变），用
// 字节区间编辑，不重序列化文档：
//   - XHTML 的 img@src，SVG image@href / xlink:href；
//   - OPF manifest item 的 href 与 media-type（覆盖层自带的 OPF 已改好时无事可做）。
// CSS 引用被换的图、同名候选不止一个、或改完后 XHTML 里仍有旧文件名，都报错，
// 不猜测。

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/liyafly/epub-handbook/internal/book/pypath"
	"github.com/liyafly/epub-handbook/internal/editset"
	"github.com/liyafly/epub-handbook/internal/scan/xhtml"
	"github.com/liyafly/epub-handbook/internal/zipfs"
)

// imageSwap 是一次换格式：From 是源树里的图，To 是覆盖层带来的图（斜杠相对路径）。
type imageSwap struct {
	From string
	To   string
}

func imageMediaType(p string) string {
	switch strings.ToLower(path.Ext(p)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	}
	return ""
}

// findImageSwaps 在套用覆盖层之前，对照源树副本找出换格式的图片。
func findImageSwaps(overlayFiles []string, srcDir string) ([]imageSwap, error) {
	var swaps []imageSwap
	for _, rel := range overlayFiles {
		if !isImagePath(rel) {
			continue
		}
		if _, err := os.Stat(filepath.Join(srcDir, filepath.FromSlash(rel))); err == nil {
			continue // 同名替换，沿用原有行为
		}
		dir, base := path.Split(rel)
		stem := strings.TrimSuffix(base, path.Ext(base))
		entries, err := os.ReadDir(filepath.Join(srcDir, filepath.FromSlash(dir)))
		if err != nil {
			continue // 新目录里的新图
		}
		var candidates []string
		for _, e := range entries {
			name := e.Name()
			if e.Type().IsRegular() && name != base && isImagePath(name) &&
				strings.TrimSuffix(name, path.Ext(name)) == stem {
				candidates = append(candidates, dir+name)
			}
		}
		switch len(candidates) {
		case 0:
		case 1:
			swaps = append(swaps, imageSwap{From: candidates[0], To: rel})
		default:
			slices.Sort(candidates)
			return nil, fmt.Errorf("overlay image %s could replace more than one source image (%s); keep one", rel, strings.Join(candidates, ", "))
		}
	}
	return swaps, nil
}

// applyImageSwaps 在已套用覆盖层的树里删去被换的图并改引用，返回改过的文件（斜杠相对路径）。
func applyImageSwaps(ctx context.Context, srcDir, opfPath string, swaps []imageSwap) ([]string, error) {
	if len(swaps) == 0 {
		return nil, nil
	}
	byFrom := make(map[string]imageSwap, len(swaps))
	for _, s := range swaps {
		byFrom[s.From] = s
	}
	var docs, sheets []string
	err := filepath.WalkDir(srcDir, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(srcDir, p)
		if rerr != nil {
			return rerr
		}
		switch slash := filepath.ToSlash(rel); {
		case isXHTMLPath(slash):
			docs = append(docs, slash)
		case strings.EqualFold(path.Ext(slash), ".css"):
			sheets = append(sheets, slash)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(docs)
	slices.Sort(sheets)

	for _, sheet := range sheets {
		data, err := readSmallFile(filepath.Join(srcDir, filepath.FromSlash(sheet)))
		if err != nil {
			return nil, err
		}
		for _, s := range swaps {
			if strings.Contains(string(data), path.Base(s.From)) {
				return nil, fmt.Errorf("%s mentions %s; image format swaps only rewrite XHTML and the OPF", sheet, s.From)
			}
		}
	}

	var changed []string
	for _, doc := range docs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		full := filepath.Join(srcDir, filepath.FromSlash(doc))
		data, err := readSmallFile(full)
		if err != nil {
			return nil, err
		}
		edits, err := swapDocEdits(doc, string(data), byFrom)
		if err != nil {
			return nil, err
		}
		updated := data
		if len(edits) > 0 {
			if updated, err = editset.Apply(doc, data, edits); err != nil {
				return nil, err
			}
		}
		for _, s := range swaps {
			if strings.Contains(string(updated), path.Base(s.From)) {
				return nil, fmt.Errorf("%s still mentions %s outside img/image references; not rewritten", doc, s.From)
			}
		}
		if len(edits) > 0 {
			if err := zipfs.WriteFileAtomic(full, updated, 0o644); err != nil {
				return nil, err
			}
			changed = append(changed, doc)
		}
	}

	opfFull := filepath.Join(srcDir, filepath.FromSlash(opfPath))
	data, err := readSmallFile(opfFull)
	if err != nil {
		return nil, err
	}
	edits, err := swapManifestEdits(opfPath, string(data), byFrom)
	if err != nil {
		return nil, err
	}
	if len(edits) > 0 {
		updated, err := editset.Apply(opfPath, data, edits)
		if err != nil {
			return nil, err
		}
		if err := zipfs.WriteFileAtomic(opfFull, updated, 0o644); err != nil {
			return nil, err
		}
		changed = append(changed, opfPath)
	}

	for _, s := range swaps {
		if err := zipfs.Remove(filepath.Join(srcDir, filepath.FromSlash(s.From))); err != nil {
			return nil, err
		}
	}
	return changed, nil
}

// swapDocEdits 改 XHTML 里指向被换图片的 img/image 引用。
func swapDocEdits(doc, content string, byFrom map[string]imageSwap) ([]editset.Edit, error) {
	var edits []editset.Edit
	for _, ref := range []struct{ tag, attr string }{{"img", "src"}, {"image", "href"}, {"image", "xlink:href"}} {
		for _, tag := range xhtml.TagsIn(content, ref.tag) {
			a, ok := tag.Attr(ref.attr)
			if !ok || a.Quote == 0 || (ref.attr == "href" && !strings.EqualFold(a.Raw, "href")) {
				continue
			}
			edit, hit, err := extEdit(doc, content, a, byFrom)
			if err != nil {
				return nil, err
			}
			if hit {
				edits = append(edits, edit)
			}
		}
	}
	return edits, nil
}

// swapManifestEdits 改 OPF manifest 里指向被换图片的 item 的 href 与 media-type。
func swapManifestEdits(opfPath, content string, byFrom map[string]imageSwap) ([]editset.Edit, error) {
	var edits []editset.Edit
	for _, tag := range xhtml.TagsIn(content, "item") {
		a, ok := tag.Attr("href")
		if !ok || a.Quote == 0 {
			continue
		}
		edit, hit, err := extEdit(opfPath, content, a, byFrom)
		if err != nil {
			return nil, err
		}
		if !hit {
			continue
		}
		edits = append(edits, edit)
		target, _ := pypath.ResolveRelativePath(opfPath, pypath.URLSplit(a.Value).Path)
		want := imageMediaType(byFrom[target].To)
		mt, ok := tag.Attr("media-type")
		if !ok || mt.Quote == 0 || want == "" {
			return nil, fmt.Errorf("%s: manifest item for %s has no quoted media-type to update", opfPath, target)
		}
		if mt.Value != want {
			edits = append(edits, editset.Replace(opfPath, int64(mt.ValueSpan.Start), int64(mt.ValueSpan.End-mt.ValueSpan.Start), []byte(want)))
		}
	}
	return edits, nil
}

// extEdit 判断属性是否指向被换的图；是则返回把值里的旧扩展名换成新扩展名的编辑。
func extEdit(doc, content string, a xhtml.Attr, byFrom map[string]imageSwap) (editset.Edit, bool, error) {
	parts := pypath.URLSplit(a.Value)
	if parts.Scheme != "" || parts.Netloc != "" || parts.Path == "" {
		return editset.Edit{}, false, nil
	}
	target, err := pypath.ResolveRelativePath(doc, parts.Path)
	if err != nil {
		return editset.Edit{}, false, nil // 坏路径交给导航审计
	}
	s, ok := byFrom[target]
	if !ok {
		return editset.Edit{}, false, nil
	}
	oldExt, newExt := path.Ext(s.From), path.Ext(s.To)
	raw := content[a.ValueSpan.Start:a.ValueSpan.End]
	if parts.Query != "" || parts.Fragment != "" || !strings.HasSuffix(raw, oldExt) {
		return editset.Edit{}, false, fmt.Errorf("%s: reference %q to %s is not a plain path ending in %s; not rewritten", doc, raw, s.From, oldExt)
	}
	at := a.ValueSpan.End - len(oldExt)
	return editset.Replace(doc, int64(at), int64(len(oldExt)), []byte(newExt)), true, nil
}
