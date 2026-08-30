package zipfs

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// buildInputZip 构造一个多 entry 输入容器，覆盖 stored / deflate /
// 二进制 / 嵌套路径几种形态，供透传断言使用。
func buildInputZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	mustCreate := func(name string, method uint16, data []byte) {
		t.Helper()
		h := &zip.FileHeader{Name: name, Modified: fixedTime()}
		h.Method = method
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatal(err)
		}
	}

	mustCreate("mimetype", zip.Store, []byte("application/epub+zip"))
	mustCreate("a/chapter.xhtml", zip.Deflate, bytes.Repeat([]byte("<p>正文。</p>\n"), 64))
	mustCreate("img/cover.png", zip.Deflate, bytes.Repeat([]byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0xFF}, 500))
	mustCreate("legacy.txt", zip.Store, []byte("stored legacy content\n"))
	mustCreate("a/nav.xhtml", zip.Deflate, []byte("<nav>目录</nav>\n"))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeTempZip(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "in.epub")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRawPassthrough 是 INV-1 的行为验证：构造一个多 entry 的 zip，
// 改其中一个、删一个、加一个，逐 entry 断言未修改项的
// CRC32 / CompressedSize / Method 与输入完全一致。
func TestRawPassthrough(t *testing.T) {
	inPath := writeTempZip(t, buildInputZip(t))

	in, err := Open(inPath)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	// 改 a/chapter.xhtml，删 legacy.txt，新增 new.xhtml，其余透传。
	chapter, err := in.Read("a/chapter.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(chapter, []byte("正文。"), []byte("正文（改）。"), 1)
	plans := []Plan{
		{Name: "mimetype", Source: mustLookup(t, in, "mimetype")},
		{Name: "a/chapter.xhtml", Source: mustLookup(t, in, "a/chapter.xhtml"), Content: edited},
		{Name: "img/cover.png", Source: mustLookup(t, in, "img/cover.png")},
		{Name: "legacy.txt", Source: mustLookup(t, in, "legacy.txt"), Deleted: true},
		{Name: "a/nav.xhtml", Source: mustLookup(t, in, "a/nav.xhtml")},
		{Name: "new.xhtml", Content: []byte("<p>新增</p>\n")},
	}
	outPath := filepath.Join(t.TempDir(), "out.epub")
	if err := in.WriteTo(outPath, plans); err != nil {
		t.Fatal(err)
	}

	outData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	outReader, err := zip.NewReader(bytes.NewReader(outData), int64(len(outData)))
	if err != nil {
		t.Fatal(err)
	}
	outBy := make(map[string]*zip.File, len(outReader.File))
	for _, zf := range outReader.File {
		outBy[zf.Name] = zf
	}

	// 未修改的 entry：三个维度逐一比对，任何一个不等都说明透传被破坏。
	for _, name := range []string{"mimetype", "img/cover.png", "a/nav.xhtml"} {
		src, _ := in.Lookup(name)
		got, ok := outBy[name]
		if !ok {
			t.Fatalf("透传 entry %s 在输出中丢失", name)
		}
		if got.CRC32 != src.zf.CRC32 {
			t.Errorf("%s: CRC32 改变 %08x != %08x", name, got.CRC32, src.zf.CRC32)
		}
		if got.CompressedSize64 != src.zf.CompressedSize64 {
			t.Errorf("%s: CompressedSize 改变 %d != %d", name, got.CompressedSize64, src.zf.CompressedSize64)
		}
		if got.Method != src.zf.Method {
			t.Errorf("%s: Method 改变 %d != %d", name, got.Method, src.zf.Method)
		}
	}

	// 被修改的 entry：内容生效。
	if got, ok := outBy["a/chapter.xhtml"]; !ok {
		t.Fatal("被修改的 entry 丢失")
	} else {
		rc, _ := got.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		if !bytes.Equal(data, edited) {
			t.Error("被修改 entry 的内容与预期不一致")
		}
	}
	// 被删除的 entry：不存在。
	if _, ok := outBy["legacy.txt"]; ok {
		t.Error("被删除的 entry 仍出现在输出中")
	}
	// 新增的 entry：存在且内容正确。
	if got, ok := outBy["new.xhtml"]; !ok {
		t.Fatal("新增 entry 丢失")
	} else if got.UncompressedSize64 != uint64(len("<p>新增</p>\n")) {
		t.Errorf("新增 entry 大小不符: %d", got.UncompressedSize64)
	}
	// 输出顺序必须与 Plan 顺序一致。
	wantOrder := []string{"mimetype", "a/chapter.xhtml", "img/cover.png", "a/nav.xhtml", "new.xhtml"}
	gotOrder := make([]string, 0, len(outReader.File))
	for _, zf := range outReader.File {
		gotOrder = append(gotOrder, zf.Name)
	}
	if !slices.Equal(gotOrder, wantOrder) {
		t.Errorf("输出顺序不符: got %v want %v", gotOrder, wantOrder)
	}
}

// TestPassthroughOnSampleBook 用仓库里 49MB 的样本书做透传 I/O 实测：
// 只改一个小 XHTML，断言其余全部 entry 字节级一致，并报告搬运量。
// 这是 W0 完成判据「800MB → 几 MB」的实测凭据（go-rewrite-handoff.md §4）。
func TestPassthroughOnSampleBook(t *testing.T) {
	matches, _ := filepath.Glob(filepath.Join("..", "..", "references", "epubs", "*.epub"))
	if len(matches) == 0 {
		t.Skip("references/epubs 下没有样本书")
	}
	in, err := Open(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	// 找一个中等大小的 XHTML 只改一个字节级别的词。
	var target string
	for _, name := range in.Names() {
		if len(name) > 6 && name[len(name)-6:] == ".xhtml" {
			if e, _ := in.Lookup(name); e.Size() > 512 && e.Size() < 64*1024 {
				target = name
				break
			}
		}
	}
	if target == "" {
		t.Skip("样本书里没有合适大小的 XHTML")
	}
	content, err := in.Read(target)
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(content, []byte(">"), []byte(">\n"), 1)
	if bytes.Equal(edited, content) {
		t.Fatalf("样本书 %s 找不到可替换字节", target)
	}

	var plans []Plan
	var passthroughCompressed, totalCompressed int64
	for _, name := range in.Names() {
		e, _ := in.Lookup(name)
		totalCompressed += e.CompressedSize()
		switch name {
		case target:
			plans = append(plans, Plan{Name: name, Source: e, Content: edited})
		default:
			plans = append(plans, Plan{Name: name, Source: e})
			passthroughCompressed += e.CompressedSize()
		}
	}
	outPath := filepath.Join(t.TempDir(), "out.epub")
	if err := in.WriteTo(outPath, plans); err != nil {
		t.Fatal(err)
	}

	out, err := Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	for _, name := range in.Names() {
		if name == target {
			continue
		}
		src, _ := in.Lookup(name)
		got, ok := out.Lookup(name)
		if !ok {
			t.Fatalf("%s 在输出中丢失", name)
		}
		if got.CRC32() != src.CRC32() || got.CompressedSize() != src.CompressedSize() || got.MethodCode() != src.MethodCode() {
			t.Errorf("%s: 未修改 entry 的 CRC32/CompressedSize/Method 改变", name)
		}
	}
	inStat, _ := os.Stat(matches[0])
	outStat, _ := os.Stat(outPath)
	t.Logf("样本书 %d bytes：透传 %d bytes compressed（%.1f%%），重写 1 个 entry，输出 %d bytes",
		inStat.Size(), passthroughCompressed,
		100*float64(passthroughCompressed)/float64(max(totalCompressed, 1)), outStat.Size())
}

func mustLookup(t *testing.T, a *Archive, name string) *Entry {
	t.Helper()
	e, ok := a.Lookup(name)
	if !ok {
		t.Fatalf("input zip 缺少 %s", name)
	}
	return e
}
