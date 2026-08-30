package typography

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/liyafly/epub-handbook/internal/book"
)

// ---- fixture（逐字对齐 scripts/test_epub_style_preset_tool.py 的 make_epub） ----

func typographyFixture(classes string) map[string]string {
	chapter := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
    <title>Fixture</title>
    <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
  </head>
  <body class="` + classes + `">
    <section>
      <h1>测试章</h1>
      <p>正文保持不变。</p>
    </section>
  </body>
</html>
`
	return map[string]string{
		"META-INF/container.xml": `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
`,
		"OEBPS/content.opf": `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="book-id">urn:test:preset</dc:identifier>
    <dc:title>Preset Fixture</dc:title>
    <dc:language>zh-CN</dc:language>
  </metadata>
  <manifest>
    <item id="chapter" href="Text/chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="old-base" href="Styles/base.css" media-type="text/css"/>
    <item id="cover-image" href="Images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
  </manifest>
  <spine><itemref idref="chapter"/></spine>
</package>
`,
		"OEBPS/Text/chapter.xhtml": chapter,
		"OEBPS/Styles/base.css":    "body { line-height: 1.2; }\n",
		"OEBPS/Images/cover.jpg":   "fixture-cover",
	}
}

func buildFixtureEpub(t *testing.T, path string, files map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	fw, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("application/epub+zip")); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(files[name])); err != nil {
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

func readZipData(t *testing.T, path string) map[string][]byte {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = data
	}
	return out
}

func repoRootDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func mustRun(t *testing.T, input, presetDir, preset, output string, dryRun bool) map[string]any {
	t.Helper()
	b, err := book.Open(input)
	if err != nil {
		t.Fatalf("book.Open: %v", err)
	}
	defer b.Close()
	res, err := Run(context.Background(), b, Params{
		Preset: preset, PresetDir: presetDir, Output: output, DryRun: dryRun, LegacyReport: true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !dryRun {
		if err := b.WriteTo(output); err != nil {
			t.Fatalf("WriteTo: %v", err)
		}
	}
	raw, ok := res.Facts["legacyReport"].(json.RawMessage)
	if !ok {
		t.Fatal("缺少 legacyReport")
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// ---- 单测（镜像 scripts/test_epub_style_preset_tool.py） ----

func TestTypographyDryRun(t *testing.T) {
	presets := filepath.Join(repoRootDir(t), "templates", "style-presets")
	dir := t.TempDir()
	source := filepath.Join(dir, "palette.epub")
	output := filepath.Join(dir, "output.epub")
	buildFixtureEpub(t, source, typographyFixture("font-st chapter-head note-box img-left"))
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}

	rep := mustRun(t, source, presets, "literary-cn", output, true)
	if rep["dry_run"] != true {
		t.Fatalf("dry_run 应为 true: %v", rep)
	}
	if _, has := rep["written_output"]; has {
		t.Fatal("dry-run 不应包含 written_output")
	}
	if _, has := rep["manifest_items_added"]; has {
		t.Fatal("dry-run 不应包含 manifest_items_added")
	}
	coverage := rep["coverage"].(map[string]any)
	ratio := coverage["ratio"].(float64)
	if ratio < 0.3 {
		t.Fatalf("coverage ratio 应 >= 0.3: %v", coverage)
	}
	if coverage["warning"] != nil {
		t.Fatalf("不应有 warning: %v", coverage)
	}
	actions := map[string]string{}
	for _, item := range rep["stylesheets"].([]any) {
		m := item.(map[string]any)
		actions[m["path"].(string)] = m["action"].(string)
	}
	if actions["OEBPS/Styles/base.css"] != "replace" || actions["OEBPS/Styles/fonts.css"] != "add" {
		t.Fatalf("actions 不符: %v", actions)
	}
	after, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("dry-run 改动了输入文件")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("dry-run 不应写出输出")
	}

	// 低覆盖率分支。
	randomSource := filepath.Join(dir, "random.epub")
	buildFixtureEpub(t, randomSource, typographyFixture("calibre99 raw-scene mystery-token"))
	randomRep := mustRun(t, randomSource, presets, "literary-cn", filepath.Join(dir, "random-output.epub"), true)
	randomCoverage := randomRep["coverage"].(map[string]any)
	if randomCoverage["ratio"].(float64) >= 0.3 {
		t.Fatalf("随机 class 的 ratio 应 < 0.3: %v", randomCoverage)
	}
	warning, _ := randomCoverage["warning"].(string)
	if !strings.Contains(warning, "先走 cleanup pipeline") {
		t.Fatalf("应输出低覆盖率 warning: %q", warning)
	}
}

func TestTypographyApply(t *testing.T) {
	presets := filepath.Join(repoRootDir(t), "templates", "style-presets")
	dir := t.TempDir()
	source := filepath.Join(dir, "source.epub")
	output := filepath.Join(dir, "output.epub")
	buildFixtureEpub(t, source, typographyFixture("font-st chapter-head note-box img-left"))

	rep := mustRun(t, source, presets, "literary-cn", output, false)
	layers := []string{"fonts.css", "base.css", "notes.css", "effects.css", "literary.css", "media.css"}
	if rep["written_output"] == "" {
		t.Fatalf("written_output 缺失: %v", rep)
	}
	added := rep["manifest_items_added"].([]any)
	if len(added) != 5 {
		t.Fatalf("manifest_items_added 应为 5（base.css 已存在）: %v", added)
	}

	files := readZipData(t, output)
	if _, ok := files["mimetype"]; !ok {
		t.Fatal("缺少 mimetype")
	}
	for _, layer := range layers {
		if _, ok := files["OEBPS/Styles/"+layer]; !ok {
			t.Fatalf("缺少层 %s", layer)
		}
	}
	if bytes.Equal(files["OEBPS/Styles/base.css"], []byte("body { line-height: 1.2; }\n")) {
		t.Fatal("base.css 应被替换为 preset 内容")
	}

	// chapter 链接 = 六层顺序一致。
	chapter := string(files["OEBPS/Text/chapter.xhtml"])
	for _, layer := range layers {
		want := `<link rel="stylesheet" type="text/css" href="../Styles/` + layer + `"/>`
		if !strings.Contains(chapter, want) {
			t.Fatalf("缺少链接 %q:\n%s", want, chapter)
		}
	}
	if strings.Contains(chapter, `href="../Styles/base.css" rel`) {
		t.Fatal("旧 link 应被整行移除")
	}

	// 输出已存在时拒绝重复应用（对齐 apply_preset）。
	b, err := book.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if _, err := Run(context.Background(), b, Params{Preset: "literary-cn", PresetDir: presets, Output: output}); err == nil {
		t.Fatal("输出已存在时应报错")
	}
}

func TestTypographyLoadPresetErrors(t *testing.T) {
	presets := filepath.Join(repoRootDir(t), "templates", "style-presets")
	if _, _, err := loadPreset("not-a-preset", presets); err == nil || !strings.Contains(err.Error(), "not-a-preset") {
		t.Fatalf("unknown preset 报错不符: %v", err)
	}
	if _, _, err := loadPreset("literary-cn", filepath.Join(presets, "nope")); err == nil ||
		!strings.Contains(err.Error(), "unknown preset") {
		t.Fatalf("缺失 preset.json 应报 unknown preset: %v", err)
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad")
	if err := os.MkdirAll(filepath.Join(bad, "Styles"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(bad, "preset.json"), []byte(`{"name": "bad", "version": "2", "layers": ["a.css"]}`), 0o644)
	if _, _, err := loadPreset("bad", dir); err == nil || !strings.Contains(err.Error(), "invalid preset metadata") {
		t.Fatalf("version 不符应报 invalid preset metadata: %v", err)
	}
	os.WriteFile(filepath.Join(bad, "preset.json"), []byte(`{"name": "bad", "version": "1", "layers": ["../evil.css"]}`), 0o644)
	if _, _, err := loadPreset("bad", dir); err == nil || !strings.Contains(err.Error(), "invalid stylesheet layer") {
		t.Fatalf("路径穿越层应报错: %v", err)
	}
}

func TestTypographyPresetLineLimits(t *testing.T) {
	// 镜像 test_preset_css_line_limits（仓库资产 ≤400 行）。
	glob := filepath.Join(repoRootDir(t), "templates", "style-presets", "*", "Styles", "*.css")
	matches, err := filepath.Glob(glob)
	if err != nil || len(matches) == 0 {
		t.Fatalf("preset 资产缺失: %v", err)
	}
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if pyLineCount(string(data)) > 400 {
			t.Fatalf("preset 样式表超 400 行: %s", path)
		}
	}
}

// ---- parity（同一 fixture 分别跑 Python oracle 与 Go 实现） ----

func chdir(t *testing.T, dir string) func() {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() { _ = os.Chdir(old) }
}

func pythonScriptPath(t *testing.T) string {
	t.Helper()
	script := filepath.Join(repoRootDir(t), "scripts", "epub_style_preset_tool.py")
	if _, err := os.Stat(script); err != nil {
		t.Skip("scripts/epub_style_preset_tool.py 不存在（oracle 已删除）")
	}
	if runtime.GOOS == "windows" {
		t.Skip("parity 用例需要 python3")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 不可用")
	}
	return script
}

func runPythonJSON(t *testing.T, dir, script string, args ...string) map[string]any {
	t.Helper()
	full := append([]string{script}, args...)
	cmd := exec.Command("python3", full...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("python oracle 运行失败: %v\nstderr: %s", err, stderr.String())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("python oracle 输出不是 JSON: %v\nstdout: %s", err, stdout.String())
	}
	return out
}

// normalizePaths 把不确定的绝对路径字段归一（Python resolve 与 Go Abs
// 在 macOS 符号链接目录上会得到不同字符串，parity 比对时置空）。
func normalizePaths(rep map[string]any) {
	for _, key := range []string{"input", "output", "written_output"} {
		if _, ok := rep[key]; ok {
			rep[key] = ""
		}
	}
}

func compareEpubEntries(t *testing.T, pyPath, goPath string) {
	t.Helper()
	pyFiles := readZipData(t, pyPath)
	goFiles := readZipData(t, goPath)
	if len(pyFiles) != len(goFiles) {
		t.Fatalf("entry 数不一致: python=%d go=%d", len(pyFiles), len(goFiles))
	}
	for name, pData := range pyFiles {
		gData, ok := goFiles[name]
		if !ok {
			t.Fatalf("Go 产物缺少 entry %s", name)
		}
		if name == "OEBPS/content.opf" {
			continue // OPF：字节区间编辑 vs ET 重写，P3 预期差异
		}
		if !bytes.Equal(pData, gData) {
			t.Fatalf("entry %s 内容不一致\npython=%q\ngo=%q", name, pData, gData)
		}
	}
}

func parityCaseTypography(t *testing.T, dryRun bool) {
	t.Helper()
	script := pythonScriptPath(t)
	presets := filepath.Join(repoRootDir(t), "templates", "style-presets")
	dir := t.TempDir()
	buildFixtureEpub(t, filepath.Join(dir, "fixture.epub"), typographyFixture("font-st chapter-head note-box img-left"))

	args := []string{"apply", "fixture.epub", "--preset", "literary-cn", "--output", "py-out.epub"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	pyReport := runPythonJSON(t, dir, script, args...)
	if !dryRun {
		if err := os.Rename(filepath.Join(dir, "py-out.epub"), filepath.Join(dir, "py.epub")); err != nil {
			t.Fatal(err)
		}
	}

	restore := chdir(t, dir)
	defer restore()
	b, err := book.Open("fixture.epub")
	if err != nil {
		t.Fatalf("book.Open: %v", err)
	}
	res, err := Run(context.Background(), b, Params{
		Preset: "literary-cn", PresetDir: presets, Output: "py-out.epub", DryRun: dryRun, LegacyReport: true,
	})
	if err != nil {
		t.Fatalf("Go Run: %v", err)
	}
	if !dryRun {
		if err := b.WriteTo("go-out.epub"); err != nil {
			t.Fatalf("WriteTo: %v", err)
		}
	}
	b.Close()
	restore()

	if !dryRun {
		compareEpubEntries(t, filepath.Join(dir, "py.epub"), filepath.Join(dir, "go-out.epub"))
	}

	raw, ok := res.Facts["legacyReport"].(json.RawMessage)
	if !ok {
		t.Fatal("缺少 legacyReport")
	}
	var goReport map[string]any
	if err := json.Unmarshal(raw, &goReport); err != nil {
		t.Fatal(err)
	}
	normalizePaths(goReport)
	normalizePaths(pyReport)
	if !reflect.DeepEqual(goReport, pyReport) {
		goJSON, _ := json.MarshalIndent(goReport, "", "  ")
		pyJSON, _ := json.MarshalIndent(pyReport, "", "  ")
		t.Fatalf("legacy 报告不一致:\n--- go ---\n%s\n--- python ---\n%s", goJSON, pyJSON)
	}
}

func TestParityTypographyApply(t *testing.T)  { parityCaseTypography(t, false) }
func TestParityTypographyDryRun(t *testing.T) { parityCaseTypography(t, true) }
