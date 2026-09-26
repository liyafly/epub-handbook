package book

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/liyafly/epub-handbook/internal/editset"
)

// buildSampleEpub 构造一个最小 EPUB 输入。
func buildSampleEpub(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	mk := func(name string, method uint16, data []byte) {
		t.Helper()
		h := &zip.FileHeader{Name: name}
		h.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
		h.Method = method
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	mk("mimetype", zip.Store, []byte("application/epub+zip"))
	mk("META-INF/container.xml", zip.Deflate, []byte("<container/>"))
	mk("OEBPS/content.opf", zip.Deflate, []byte("<package/>"))
	mk("OEBPS/Text/c1.xhtml", zip.Deflate, []byte("<html><p>第一章</p></html>"))
	mk("OEBPS/Styles/main.css", zip.Deflate, []byte("p { margin: 0; }"))
	mk("OEBPS/Images/cover.png", zip.Deflate, bytes.Repeat([]byte{1, 2, 3}, 100))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func openSample(t *testing.T) (*Book, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "in.epub")
	if err := os.WriteFile(path, buildSampleEpub(t), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	return b, path
}

func TestOpenProjectsEntries(t *testing.T) {
	b, _ := openSample(t)
	want := []string{"mimetype", "META-INF/container.xml", "OEBPS/content.opf", "OEBPS/Text/c1.xhtml", "OEBPS/Styles/main.css", "OEBPS/Images/cover.png"}
	if got := b.OriginalNames(); !slices.Equal(got, want) {
		t.Fatalf("OriginalNames = %v", got)
	}
	if got := b.Names(); !slices.Equal(got, want) {
		t.Fatalf("Names = %v", got)
	}
	if !b.Has("mimetype") || b.Has("nope") {
		t.Fatal("Has 判定错误")
	}
}

func TestOpenBytesAndWriteBytes(t *testing.T) {
	input := buildSampleEpub(t)
	b, err := OpenBytesContext(t.Context(), "memory-input.epub", input)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	original, err := b.CurrentContext(t.Context(), "OEBPS/Text/c1.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(original, []byte("第一章"))
	if start < 0 {
		t.Fatal("fixture is missing the chapter heading")
	}
	if err := b.Apply([]editset.Edit{editset.Replace("OEBPS/Text/c1.xhtml", int64(start), int64(len("第一章")), []byte("第二章"))}); err != nil {
		t.Fatal(err)
	}
	output, err := b.WriteBytesContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(output) == 0 {
		t.Fatal("WriteBytesContext returned empty EPUB bytes")
	}
	opened, err := OpenBytesContext(t.Context(), "memory-output.epub", output)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	content, err := opened.CurrentContext(t.Context(), "OEBPS/Text/c1.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "第二章") {
		t.Fatalf("in-memory EPUB output does not contain applied edit: %s", content)
	}
}

func TestForkSharesArchiveAndIsolatesCurrentEdits(t *testing.T) {
	parent, _ := openSample(t)
	fork := parent.Fork()
	path := "OEBPS/Text/c1.xhtml"
	original, err := parent.Current(path)
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(original, []byte("第一章"))
	if start < 0 {
		t.Fatal("fixture is missing the chapter heading")
	}
	if err := fork.Apply([]editset.Edit{editset.Replace(path, int64(start), int64(len("第一章")), []byte("fork 修改"))}); err != nil {
		t.Fatal(err)
	}
	if err := fork.Close(); err != nil {
		t.Fatalf("closing a fork must not close the shared archive: %v", err)
	}
	parentContent, err := parent.Current(path)
	if err != nil {
		t.Fatalf("parent archive was closed with the fork: %v", err)
	}
	if !bytes.Equal(parentContent, original) {
		t.Fatalf("fork edit leaked into parent: got %q want %q", parentContent, original)
	}
	forkContent, err := fork.Current(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(forkContent, []byte("fork 修改")) {
		t.Fatalf("fork did not retain its isolated edit: %q", forkContent)
	}
}

func TestForkSharesOriginalCacheBudget(t *testing.T) {
	parent, _ := openSample(t)
	parent.maxRetainedBytes = 30
	fork := parent.Fork()
	if _, err := fork.Original("mimetype"); err != nil {
		t.Fatal(err)
	}
	if _, err := parent.Original("OEBPS/content.opf"); err != nil {
		t.Fatal(err)
	}
	if data, err := fork.Original("META-INF/container.xml"); !errors.Is(err, ErrMemoryLimit) || data != nil {
		t.Fatalf("fork cache exceeded the shared per-book budget: data=%q err=%v", data, err)
	}
}

func TestApplyModifyCreateDelete(t *testing.T) {
	b, _ := openSample(t)

	// 修改：字节区间替换。
	orig, err := b.Original("OEBPS/Text/c1.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(orig, []byte("一"))
	if i < 0 {
		t.Fatal("fixture 缺少目标字节")
	}
	if err := b.Apply([]editset.Edit{editset.Replace("OEBPS/Text/c1.xhtml", int64(i), 3, []byte("贰"))}); err != nil {
		t.Fatal(err)
	}
	// Original 必须保持原样（红线校验依赖这一点）。
	if !bytes.Equal(orig, []byte("<html><p>第一章</p></html>")) {
		t.Fatalf("Original 被污染: %q", orig)
	}
	cur, err := b.Current("OEBPS/Text/c1.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	if string(cur) != "<html><p>第贰章</p></html>" {
		t.Fatalf("Current = %q", cur)
	}
	if !b.IsModified("OEBPS/Text/c1.xhtml") {
		t.Error("IsModified 应为真")
	}

	// 新建 entry。
	if err := b.Apply([]editset.Edit{editset.Replace("OEBPS/Text/nav.xhtml", 0, 0, []byte("<nav/>"))}); err != nil {
		t.Fatal(err)
	}
	if !b.Has("OEBPS/Text/nav.xhtml") {
		t.Error("新建的 entry 应存在")
	}

	// 删除 entry。
	if err := b.Apply([]editset.Edit{editset.Delete("OEBPS/Styles/main.css")}); err != nil {
		t.Fatal(err)
	}
	if b.Has("OEBPS/Styles/main.css") {
		t.Error("被删除的 entry 不应存在")
	}
	// 重复删除必须报错。
	if err := b.Apply([]editset.Edit{editset.Delete("OEBPS/Styles/main.css")}); err == nil {
		t.Error("重复删除应报错")
	}
	// 删除后按原名重建。
	if err := b.Apply([]editset.Edit{editset.Replace("OEBPS/Styles/main.css", 0, 0, []byte("p{}"))}); err != nil {
		t.Fatal(err)
	}
	if !b.Has("OEBPS/Styles/main.css") {
		t.Error("重建后的 entry 应存在")
	}

	// 重建的 main.css 回到原位（原序），新增的 nav.xhtml 仍在末尾。
	want := []string{"mimetype", "META-INF/container.xml", "OEBPS/content.opf", "OEBPS/Text/c1.xhtml", "OEBPS/Styles/main.css", "OEBPS/Images/cover.png", "OEBPS/Text/nav.xhtml"}
	if got := b.Names(); !slices.Equal(got, want) {
		t.Fatalf("Names = %v", got)
	}
}

func TestApplyRejectsDeleteAfterContentEdit(t *testing.T) {
	b, _ := openSample(t)
	const name = "OEBPS/Text/c1.xhtml"
	original, err := b.Current(name)
	if err != nil {
		t.Fatal(err)
	}
	edits := []editset.Edit{
		editset.Replace(name, 0, 5, []byte("<main")),
		editset.Delete(name),
	}
	if err := b.Apply(edits); err == nil || !strings.Contains(err.Error(), "mixing entry deletion with content edits") {
		t.Fatalf("Apply error = %v, want mixed deletion/content rejection", err)
	}
	if b.IsModified(name) {
		t.Fatal("rejected edit group modified the book")
	}
	current, err := b.Current(name)
	if err != nil || !bytes.Equal(current, original) {
		t.Fatalf("rejected edit group changed content: %q (%v)", current, err)
	}
}

func TestWriteToProducesExpectedArchive(t *testing.T) {
	b, _ := openSample(t)
	if err := b.Apply([]editset.Edit{
		editset.Replace("OEBPS/Text/c1.xhtml", 9, 9, []byte("<p>改</p>")),
		editset.Replace("OEBPS/Text/nav.xhtml", 0, 0, []byte("<nav/>")),
		editset.Delete("OEBPS/Styles/main.css"),
	}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.epub")
	if err := b.WriteTo(out); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	stat, _ := f.Stat()
	r, err := zip.NewReader(f, stat.Size())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, zf := range r.File {
		names = append(names, zf.Name)
	}
	want := []string{"mimetype", "META-INF/container.xml", "OEBPS/content.opf", "OEBPS/Text/c1.xhtml", "OEBPS/Images/cover.png", "OEBPS/Text/nav.xhtml"}
	if !slices.Equal(names, want) {
		t.Fatalf("输出顺序 = %v, want %v", names, want)
	}
	if r.File[0].Method != zip.Store {
		t.Errorf("mimetype 应为 STORED，实际 %d", r.File[0].Method)
	}
	// 未修改的 entry 保持原压缩字节（透传）。
	for _, name := range []string{"META-INF/container.xml", "OEBPS/content.opf", "OEBPS/Images/cover.png"} {
		zf := r.File[slices.Index(names, name)]
		if zf.Method != zip.Deflate {
			t.Errorf("%s: Method = %d", name, zf.Method)
		}
	}
	// 新增 entry 排在最后且按字母序。
	if names[len(names)-1] != "OEBPS/Text/nav.xhtml" {
		t.Errorf("新增 entry 应在末尾，实际 %s", names[len(names)-1])
	}
}

func TestValidatePath(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"OEBPS/Text/a.xhtml", false},
		{"a.xhtml", false},
		{"", true},
		{"/abs.xhtml", true},
		{"../escape.xhtml", true},
		{"a/../../escape.xhtml", true},
		{"a/../b.xhtml", false},
	}
	for _, c := range cases {
		err := ValidatePath(c.name)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidatePath(%q) = %v, wantErr %v", c.name, err, c.wantErr)
		}
	}
}
