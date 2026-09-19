// parity_test.go 原为 Python oracle（scripts/validate_text_invariance.py）的
// P2 parity 用例。oracle 已于 2026-08-29 随 scripts/ 删除，此后 runPythonOracle
// 的 os.Stat 检查让全部 9 个用例**无条件 t.Skip** —— 六条红线里
// anchors / metadata / spine / cover / drm 的**命中路径**因此完全没有可执行
// 断言（inprocess_test.go 只覆盖 text 命中，以及"干净的书零 findings"的放行
// 路径）。INV-5 只保证校验器被注册，不保证它还会开火。
//
// 现改为 Go-native 断言：期望值按 checks.go 各校验器的语义手写（哪条红线
// 开火、退出码、以及能标识问题的关键片段），不回填实现输出，避免 golden
// 自我实现。凡带"反向对照"的用例都同时断言"不给选项时必须报错"，证明是
// 选项而不是巧合让它通过。
package redline

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type zipEntry struct {
	name    string
	content []byte
	method  uint16
}

func buildEpub(t *testing.T, path string, entries []zipEntry) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name}
		h.Method = e.method
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(e.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

const opfXML = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="id">
  <metadata>
    <dc:title>测试书</dc:title>
    <dc:creator>作者</dc:creator>
    <dc:identifier id="id">urn:uuid:1234</dc:identifier>
    <dc:language>zh-CN</dc:language>
    <meta name="cover" content="cover-image"/>
  </metadata>
  <manifest>
    <item id="c1" href="Text/c1.xhtml" media-type="application/xhtml+xml"/>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="css" href="Styles/main.css" media-type="text/css"/>
    <item id="cover-image" href="Images/cover.png" media-type="image/png" properties="cover-image"/>
  </manifest>
  <spine>
    <itemref idref="nav" linear="no"/>
    <itemref idref="c1"/>
  </spine>
</package>
`

const c1XHTML = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-CN">
  <head><title>第一章</title></head>
  <body>
    <p id="p1">第一段落。</p>
    <p>第二段落　全角空格。</p>
    <p>汉字<ruby>字<rt>zì</rt></ruby>注音。</p>
  </body>
</html>
`

func baseEntries() []zipEntry {
	return []zipEntry{
		{name: "mimetype", content: []byte("application/epub+zip"), method: 0},
		{name: "META-INF/container.xml", content: []byte(`<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{name: "OEBPS/content.opf", content: []byte(opfXML)},
		{name: "OEBPS/Text/c1.xhtml", content: []byte(c1XHTML)},
		{name: "OEBPS/nav.xhtml", content: []byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="Text/c1.xhtml">第一章</a></li></ol></nav></body></html>`)},
		{name: "OEBPS/Styles/main.css", content: []byte("p { margin: 0; }\n")},
		{name: "OEBPS/Images/cover.png", content: bytes.Repeat([]byte{0x89, 'P', 'N', 'G'}, 32)},
	}
}

// editEntry 返回一份 baseEntries 副本，其中 name 对应的 entry 内容被 fn 改写。
// 找不到 name 时 t.Fatal —— fixture 改了名而用例悄悄什么都没测是最坏结果。
func editEntry(t *testing.T, name string, fn func([]byte) []byte) []zipEntry {
	t.Helper()
	out := baseEntries()
	for i := range out {
		if out[i].name == name {
			out[i].content = fn(out[i].content)
			return out
		}
	}
	t.Fatalf("fixture 里没有 entry %q", name)
	return nil
}

// renameEntry 返回一份 baseEntries 副本，其中 from 被改名为 to。
func renameEntry(t *testing.T, from, to string) []zipEntry {
	t.Helper()
	out := baseEntries()
	for i := range out {
		if out[i].name == from {
			out[i].name = to
			return out
		}
	}
	t.Fatalf("fixture 里没有 entry %q", from)
	return nil
}

// pair 建一对 before/after EPUB 并返回它们的路径。
func pair(t *testing.T, before, after []zipEntry) (string, string) {
	t.Helper()
	dir := t.TempDir()
	bp := filepath.Join(dir, "before.epub")
	ap := filepath.Join(dir, "after.epub")
	buildEpub(t, bp, before)
	buildEpub(t, ap, after)
	return bp, ap
}

// compare 跑 CompareFiles 并把报告行拼成一段便于断言的文本。
func compare(t *testing.T, before, after, check string, o Options) (Report, string) {
	t.Helper()
	rep, err := CompareFiles(before, after, check, o)
	if err != nil {
		t.Fatalf("CompareFiles(%s): %v", check, err)
	}
	return rep, strings.Join(rep.Lines, "\n")
}

// wantCode 断言退出码，失败时把整份报告打出来。
func wantCode(t *testing.T, rep Report, text string, code int) {
	t.Helper()
	if rep.Code != code {
		t.Errorf("退出码 = %d, want %d\n报告:\n%s", rep.Code, code, text)
	}
}

// wantLine 断言报告里有一行以 prefix 开头，并返回该行（便于继续断言细节）。
func wantLine(t *testing.T, rep Report, text, prefix string) string {
	t.Helper()
	for _, line := range rep.Lines {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	t.Errorf("报告缺少以 %q 开头的行\n报告:\n%s", prefix, text)
	return ""
}

// wantNoLine 断言报告里没有以 prefix 开头的行（用于"这条红线不该开火"）。
func wantNoLine(t *testing.T, rep Report, text, prefix string) {
	t.Helper()
	for _, line := range rep.Lines {
		if strings.HasPrefix(line, prefix) {
			t.Errorf("报告出现了不该有的 %q 行: %q\n报告:\n%s", prefix, line, text)
			return
		}
	}
}

const passLine = "All requested red-line checks passed."

func TestRedlineIdenticalPasses(t *testing.T) {
	before, after := pair(t, baseEntries(), baseEntries())
	rep, text := compare(t, before, after, "all", Options{})
	wantCode(t, rep, text, 0)
	if len(rep.Lines) != 1 || rep.Lines[0] != passLine {
		t.Errorf("报告 = %q, want [%q]", rep.Lines, passLine)
	}
}

func TestRedlineTextChangeDetected(t *testing.T) {
	before, after := pair(t, baseEntries(), editEntry(t, "OEBPS/Text/c1.xhtml", func(b []byte) []byte {
		return bytes.Replace(b, []byte("第一段落"), []byte("第一段落!"), 1)
	}))
	rep, text := compare(t, before, after, "text", Options{})
	wantCode(t, rep, text, 1)
	wantLine(t, rep, text, "text: modified OEBPS/Text/c1.xhtml -> OEBPS/Text/c1.xhtml: 3 blocks before, 3 after")
	// 明细必须点到具体块与改后文本，否则人工 diff review 无从下手。
	wantLine(t, rep, text, "  block 0:")
	if after := wantLine(t, rep, text, "    after:"); !strings.Contains(after, "第一段落!") {
		t.Errorf("after 明细未包含改后正文: %q", after)
	}
}

func TestRedlineDeletedXHTMLDetected(t *testing.T) {
	dropped := baseEntries()
	out := dropped[:0]
	for _, e := range dropped {
		if e.name != "OEBPS/Text/c1.xhtml" {
			out = append(out, e)
		}
	}
	before, after := pair(t, baseEntries(), out)
	rep, text := compare(t, before, after, "text", Options{})
	wantCode(t, rep, text, 1)
	wantLine(t, rep, text, "text: deleted XHTML file: OEBPS/Text/c1.xhtml")
}

func TestRedlineDeletedAnchorDetected(t *testing.T) {
	before, after := pair(t, baseEntries(), editEntry(t, "OEBPS/Text/c1.xhtml", func(b []byte) []byte {
		return bytes.Replace(b, []byte(` id="p1"`), nil, 1)
	}))
	rep, text := compare(t, before, after, "anchors", Options{})
	wantCode(t, rep, text, 1)
	line := wantLine(t, rep, text, "anchors: deleted id in OEBPS/Text/c1.xhtml:")
	if !strings.Contains(line, "p1") {
		t.Errorf("anchors 行未点名被删的 id: %q", line)
	}
}

func TestRedlineMetadataChangeDetected(t *testing.T) {
	before, after := pair(t, baseEntries(), editEntry(t, "OEBPS/content.opf", func(b []byte) []byte {
		return bytes.Replace(b, []byte("测试书"), []byte("新书名"), 1)
	}))
	rep, text := compare(t, before, after, "metadata,spine", Options{})
	wantCode(t, rep, text, 1)
	line := wantLine(t, rep, text, "metadata: dc:title changed:")
	if !strings.Contains(line, "测试书") || !strings.Contains(line, "新书名") {
		t.Errorf("metadata 行未给出改前/改后书名: %q", line)
	}
	// 只改了标题，spine 必须保持沉默 —— 否则红线区分不出改了什么。
	wantNoLine(t, rep, text, "spine:")
}

func TestRedlineSpineChangeDetected(t *testing.T) {
	before, after := pair(t, baseEntries(), editEntry(t, "OEBPS/content.opf", func(b []byte) []byte {
		return bytes.Replace(b,
			[]byte(`<itemref idref="nav" linear="no"/>`+"\n    "+`<itemref idref="c1"/>`),
			[]byte(`<itemref idref="c1"/>`+"\n    "+`<itemref idref="nav" linear="no"/>`), 1)
	}))
	rep, text := compare(t, before, after, "metadata,spine", Options{})
	wantCode(t, rep, text, 1)
	line := wantLine(t, rep, text, "spine: itemref sequence changed:")
	if !strings.Contains(line, "nav") || !strings.Contains(line, "c1") {
		t.Errorf("spine 行未给出 idref 序列: %q", line)
	}
	wantNoLine(t, rep, text, "metadata:")
}

func TestRedlineCoverBytesChangeDetected(t *testing.T) {
	before, after := pair(t, baseEntries(), editEntry(t, "OEBPS/Images/cover.png", func(b []byte) []byte {
		return append(append([]byte(nil), b...), 0xFF)
	}))
	rep, text := compare(t, before, after, "cover", Options{})
	wantCode(t, rep, text, 1)
	line := wantLine(t, rep, text, "cover: cover-image bytes changed:")
	if !strings.Contains(line, "cover.png") {
		t.Errorf("cover 行未点名封面路径: %q", line)
	}
}

func TestRedlineCoverMissingFromZipDetected(t *testing.T) {
	kept := baseEntries()
	out := kept[:0]
	for _, e := range kept {
		if e.name != "OEBPS/Images/cover.png" {
			out = append(out, e)
		}
	}
	before, after := pair(t, baseEntries(), out)
	rep, text := compare(t, before, after, "cover", Options{})
	wantCode(t, rep, text, 1)
	wantLine(t, rep, text, "cover: cover-image missing from zip:")
}

// TestRedlineCoverPathMessageUsesMappedPath 钉住封面红线消息里的**左值**。
//
// 判据用的是映射后的 before 路径（MappedPath），消息此前却打印未映射的
// 原路径，于是带 path-map 的场景会给出自相矛盾的 finding：
// 「cover-image path changed: 'OEBPS/Images/cover.png' -> 'OEBPS/Images/cover.png'」。
// 2026-09-07 用真书实测出来：merge 两卷同源书时，802 条重命名映射里包含
// 封面，redline 就报了这么一行。
//
// 这里前后两本书逐字节相同，只靠 path-map 让判据不成立 —— 消息必须点名
// 映射后的目标路径，否则读者无从知道守卫在拿什么跟什么比。
func TestRedlineCoverPathMessageUsesMappedPath(t *testing.T) {
	before, after := pair(t, baseEntries(), baseEntries())
	o := Options{PathMap: map[string]string{
		"OEBPS/Images/cover.png": "OEBPS/Images/vol2_cover.png",
	}}
	rep, text := compare(t, before, after, "cover", o)
	wantCode(t, rep, text, 1)
	line := wantLine(t, rep, text, "cover: cover-image path changed:")
	if !strings.Contains(line, "vol2_cover.png") {
		t.Errorf("消息未点名映射后的路径（左值仍是未映射值）: %q", line)
	}
	if strings.Count(line, "'OEBPS/Images/cover.png'") == 2 {
		t.Errorf("消息两侧同值，等于说「变了但没变」: %q", line)
	}
}

// TestRedlineCoverPathMapClosesLoop 负向控制：path-map 与实际改名一致时，
// 封面红线必须放行 —— 上面那条不是把判据改松了。
func TestRedlineCoverPathMapClosesLoop(t *testing.T) {
	renamed := renameEntry(t, "OEBPS/Images/cover.png", "OEBPS/Images/vol2_cover.png")
	for i := range renamed {
		if renamed[i].name == "OEBPS/content.opf" {
			renamed[i].content = []byte(strings.ReplaceAll(string(renamed[i].content),
				`href="Images/cover.png"`, `href="Images/vol2_cover.png"`))
		}
	}
	before, after := pair(t, baseEntries(), renamed)
	o := Options{PathMap: map[string]string{
		"OEBPS/Images/cover.png": "OEBPS/Images/vol2_cover.png",
	}}
	rep, text := compare(t, before, after, "cover", o)
	wantCode(t, rep, text, 0)
	wantNoLine(t, rep, text, "cover:")
}

func TestRedlineDRMRefused(t *testing.T) {
	// 未知算法的 encryption.xml：DRM 是硬拒绝（退出码 2），不是普通 finding。
	drm := append(baseEntries(), zipEntry{
		name:    "META-INF/encryption.xml",
		content: []byte(`<?xml version="1.0"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><EncryptedData><EncryptionMethod Algorithm="http://www.example.com/unknown"/><CipherReference URI="OEBPS/Text/c1.xhtml"/></EncryptedData></encryption>`),
	})
	before, after := pair(t, baseEntries(), drm)
	rep, text := compare(t, before, after, "all", Options{})
	wantCode(t, rep, text, 2)
	if len(rep.Lines) != 1 || rep.Lines[0] != "DRM detected, refusing to process." {
		t.Errorf("报告 = %q, want 单行 DRM 拒绝", rep.Lines)
	}
}

func TestRedlineDRMStaleAllowed(t *testing.T) {
	// 声明目标在 ZIP 中不存在：stale 引用，两侧一致时放行。
	stale := append(baseEntries(), zipEntry{
		name:    "META-INF/encryption.xml",
		content: []byte(`<?xml version="1.0"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><EncryptedData><EncryptionMethod Algorithm="http://www.idpf.org/2008/embedding"/><CipherReference URI="Fonts/gone.ttf"/></EncryptedData></encryption>`),
	})
	before, after := pair(t, stale, stale)
	rep, text := compare(t, before, after, "drm", Options{})
	wantCode(t, rep, text, 0)
}

func TestRedlineFontObfuscationNeedsExplicitFlag(t *testing.T) {
	// 真实存在的字体 + 标准混淆算法：默认仍然拒绝，只有显式授权才放行。
	// 这是 AllowFontObfuscation 唯一的行为差异，必须两支都测。
	fontEntries := func() []zipEntry {
		out := baseEntries()
		for i := range out {
			if out[i].name == "OEBPS/content.opf" {
				out[i].content = bytes.Replace(out[i].content,
					[]byte(`<item id="css"`),
					[]byte(`<item id="font" href="Fonts/embedded.ttf" media-type="font/ttf"/>`+"\n    "+`<item id="css"`), 1)
			}
		}
		out = append(out,
			zipEntry{name: "OEBPS/Fonts/embedded.ttf", content: bytes.Repeat([]byte{0x00, 0x01, 0x00, 0x00}, 16)},
			zipEntry{
				name:    "META-INF/encryption.xml",
				content: []byte(`<?xml version="1.0"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><EncryptedData><EncryptionMethod Algorithm="http://www.idpf.org/2008/embedding"/><CipherReference URI="OEBPS/Fonts/embedded.ttf"/></EncryptedData></encryption>`),
			})
		return out
	}
	before, after := pair(t, fontEntries(), fontEntries())

	rep, text := compare(t, before, after, "drm", Options{})
	wantCode(t, rep, text, 2)

	rep, text = compare(t, before, after, "drm", Options{AllowFontObfuscation: true})
	wantCode(t, rep, text, 0)
}

func TestRedlinePathMapMakesRenameInvisible(t *testing.T) {
	renamed := renameEntry(t, "OEBPS/Text/c1.xhtml", "OEBPS/Text/chapter1.xhtml")
	before, after := pair(t, baseEntries(), renamed)

	// 反向对照：不给 --path-map 时，改名必须被报成删除 + 新增，而不是静默通过。
	rep, text := compare(t, before, after, "text", Options{})
	wantCode(t, rep, text, 1)
	wantLine(t, rep, text, "text: deleted XHTML file: OEBPS/Text/c1.xhtml")
	wantLine(t, rep, text, "text: added XHTML file: OEBPS/Text/chapter1.xhtml")

	pm, err := LoadPathMap([]byte(`{"stages":[{"mappings":[{"from":"OEBPS/Text/c1.xhtml","to":"OEBPS/Text/chapter1.xhtml"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	rep, text = compare(t, before, after, "text", Options{PathMap: pm})
	wantCode(t, rep, text, 0)
}

func TestRedlineAllowListExemptsOnlyListedPaths(t *testing.T) {
	touched := editEntry(t, "OEBPS/Text/c1.xhtml", func(b []byte) []byte {
		return bytes.Replace(b, []byte("第二段落"), []byte("第贰段落"), 1)
	})
	before, after := pair(t, baseEntries(), touched)

	// 反向对照：没有豁免时必须开火。
	rep, text := compare(t, before, after, "text", Options{})
	wantCode(t, rep, text, 1)
	wantLine(t, rep, text, "text: modified OEBPS/Text/c1.xhtml")

	rep, text = compare(t, before, after, "text", Options{AllowList: []string{"*/Text/c1.xhtml"}})
	wantCode(t, rep, text, 0)

	// 豁免必须是精确的：不相关的 glob 不得让问题消失。
	rep, text = compare(t, before, after, "text", Options{AllowList: []string{"*/nav.xhtml"}})
	wantCode(t, rep, text, 1)
	wantLine(t, rep, text, "text: modified OEBPS/Text/c1.xhtml")
}

func TestRedlineInvalidCheckIsInputError(t *testing.T) {
	before, after := pair(t, baseEntries(), baseEntries())
	rep, text := compare(t, before, after, "text,bogus", Options{})
	wantCode(t, rep, text, 2)
	wantLine(t, rep, text, "input error:")
}

func TestRedlineMissingInputIsInputError(t *testing.T) {
	before, _ := pair(t, baseEntries(), baseEntries())
	rep, text := compare(t, before, filepath.Join(t.TempDir(), "nope.epub"), "all", Options{})
	wantCode(t, rep, text, 2)
	wantLine(t, rep, text, "input error:")
}
